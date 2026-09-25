// dsh.go —— DeepSeek Harness(dsh)桌面 agent 桥接(甲级变更 dsh-P1)。
//
// 架构完全复制 codexapp:一个长驻 `dsh --profile acp` 子进程承载任意多会话,
// JSON-RPC 2.0 换行分帧,审批走反向请求(session/request_permission)。事实来源:
//   - 设计蓝本:2026-09-20-dsh-integration-plan.md §4/§5(写码时逐项对标 §10.A
//     参考实现对照表);
//   - 实测契约:docs/verify/evidence/dsh-p0-probe.md(P0 探针,8 项实测结论与
//     6 条设计修正,本文件逐条落实)。
//
// 【本 Provider 明确不做的事(蓝本 §4.3,如实文档化)】
//   - Steerer 不实现:ACP 一会话一 prompt,运行中插话由 server 层 turn_queue
//     排队、当前轮次终局后自动续跑(pi/zcode 同款);
//   - 审批策略/沙箱不映射:dsh ACP 固定一次性审批(allow/reject,无"始终允许"),
//     与 zcode 固定 mode:"build" 同款边界;
//   - AgentSessionImporter 不做:隔离 HOME 下没有"用户桌面 dsh 会话"可导;
//   - TextGenerator 不实现:会话标题由 dsh 自带 session-title;
//   - 流式粒度按"块级"预期实现:agent_message_chunk 是已提交消息的文本块,
//     非逐字 delta(上游 discussion #594;探针受缺 key 阻塞未量化,跟踪上游);
//   - 图片输入按能力协商:promptCapabilities.image=false 时如实报错,不静默丢。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pocketagent/daemon/internal/dshapp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// ---- 时长预算 ----

const (
	dshModelsTTL       = 5 * time.Minute   // Models() 缓存
	dshModelsBudget    = 10 * time.Second  // 一次性取样的整段预算(对齐 ListModels 10s 有界)
	dshTurnCap         = 60 * time.Minute  // 单轮上限(对齐 codex)
	dshSilenceWatchdog = 150 * time.Second // 无任何事件的看门狗(对齐 codex)
	// dshInitBudget initialize 握手的默认上界(探针实测冷启动 1.415s,8s 给足余量;
	// 与 dshapp 包内同名常量同值 —— 两侧独立声明,避免跨包私有耦合)。
	dshInitBudget = 8 * time.Second
)

// ---- 生命周期锁的有界排队与启动预热(T-DSH 问题 B)----
//
// 真机实证(2026-09-21 daemon.log):dsh 首跑(DSH_HOME 为空)的 initialize 握手
// 实际耗时可达十位数分钟(16:19:36 拉起子进程 → 16:38:22 握手完成),远超探针
// 实测的 1.4s。期间所有用户路径调用(ListModels 模型取样/轮次就绪)与慢握手
// 撞锁,TryLock 立即 busy-reject → 手机端反复收到"dsh 正忙"。两步收敛:
//  1. 有界排队:TryLock 失败不再立即拒绝,先在 dshLockWait 窗口内等待锁
//     (pi/zcode 排队语义;窗口受调用方自身截止时间收窄),窗口耗尽仍拿不到
//     才回 dshBusy —— 保留"一个卡死的操作不拖住全线"的底线;
//  2. 启动预热:daemon 启动后台完成一次握手(Warmup),把首跑慢启动的开销
//     吸收到启动期,用户路径不再与慢握手竞争。
var (
	dshLockWait      = 15 * time.Second // TryLock 失败后的排队窗口(包级变量仅为测试注入短值)
	dshWarmupBudget  = 2 * time.Minute  // 单次预热握手预算(吸收分钟级首跑启动)
	dshWarmupRetry   = 30 * time.Second // 预热失败后的重试间隔
	dshWarmupRetries = 12               // 预热重试次数上限(2min+30s × 12 ≈ 29 分钟覆盖)
)

// dshApprovalTimeout 审批等待上限:超时走拒绝路径,绝不伪造"同意"(红线 1)。
// 包级变量仅为测试注入短值;生产语义 = 2 分钟,请勿改动。
var dshApprovalTimeout = 2 * time.Minute

// dshClient 是 DshProvider 对 dshapp.Client 的依赖面(最小接口,单测注 fake,
// 不依赖真子进程;codexApprovalResponder/zcodeResponder 同款模式)。
type dshClient interface {
	Initialize(ctx context.Context) (*dshapp.InitializeResult, error)
	SessionNew(ctx context.Context, cwd string) (*dshapp.SessionState, error)
	SessionList(ctx context.Context) ([]dshapp.SessionListEntry, error)
	// SessionResume 的 cwd:rc.3 zod 必填(dsh-resume-compat-probe.md 实测),
	// 缺它旧会话一律 -32602 "Invalid params" —— 升级后"会话发不出消息"的真因。
	SessionResume(ctx context.Context, sessionID, cwd string) (*dshapp.SessionState, error)
	SessionClose(ctx context.Context, sessionID string) error
	SetConfigOption(ctx context.Context, sessionID, optionID, value string) error
	Prompt(ctx context.Context, sessionID string, blocks []map[string]interface{}) (json.RawMessage, error)
	Cancel(sessionID string)
	SetTurnHandlers(notif func(string, json.RawMessage), req func(int64, string, json.RawMessage))
	Respond(id int64, result interface{}) error
	RespondError(id int64, code int, message string) error
	SyncHandlers()
	Alive() bool
	PID() int
	Close()
}

// 编译期钉死:*dshapp.Client 满足依赖面(接口漂移在编译期暴露);
// dshRunError 满足 ErrorInfoCarrier(终局 TurnFailed 保真)。
var (
	_ dshClient        = (*dshapp.Client)(nil)
	_ ErrorInfoCarrier = (*dshRunError)(nil)
)

// DshProvider 把我们的 gRPC 协议桥接到 dsh 的 ACP 面。
//
// 事件对照表(手机上看到的 ← dsh session/update 变体):
//
//	助手消息(块级)  agent_message_chunk → TextDelta
//	思考过程         agent_thought_chunk → ThinkingDelta(HideReasoning 时过滤)
//	工具卡片状态机   tool_call / tool_call_update → ToolCallUpdate
//	上下文用量       context_usage → UsageUpdate
//	审批弹卡         session/request_permission(反向请求)→ ApprovalHook
//	一轮终局         session/prompt 应答 stopReason end_turn/cancelled
type DshProvider struct {
	// turnMu 串行化轮次;lifeMu 保护子进程生命周期(有界排队获取→dshBusy,查模型不阻塞轮次)
	turnMu     sync.Mutex
	lifeMu     sync.Mutex
	turnActive atomic.Bool
	client     dshClient
	inited     bool
	caps       *dshapp.InitializeResult // initialize 能力快照(§5.2 能力门控的事实来源)
	gen        atomic.Int32             // 子进程代号,每次(重)启 +1

	// 我们的 threadID → dsh sessionId(threadRef 复用 codex 的 gen 语义):
	// 内存缓存只在同一子进程代内有效,重启后走 ExternalSession 重挂载。
	threadMap map[string]threadRef
	threadMu  sync.Mutex

	// 模型面缓存(无静态名单:全部来自 session/new|resume 应答的 configOptions)
	modelsMu  sync.Mutex
	models    []pb.ModelInfo
	modelsAt  time.Time
	catMu     sync.Mutex
	catalog   dshapp.ModelCatalog   // 最近一次目录(切换模型时取原样编码串)
	modelVals map[string]string     // "dsh/<slug>" → 原样双层编码串(探索目录时顺手登记)
	rawOpts   []dshapp.ConfigOption // 最近一次完整原生目录(原生设置透出用;非仅模型面)
	// nativeDefaults 原生设置的"新会话初始值"(设置页无线程上下文写入的目标;
	// 探针实证 set_config_option 仅会话内生效不持久,新会话由 ensureSession 灌入。
	// 进程内存态:daemon 重启后由手机设置页当前值重新写入 —— 与"仅新建对话生效"
	// 的设置页语义一致,如实文档化)。
	nativeDefaults map[string]string

	// 桌面偏好快照(PrefsAware 子集:Model/Effort/HideReasoning)
	prefsMu sync.Mutex
	prefs   DesktopPrefs

	// newClient 是测试注入点:非空时 withClientCtx 用它造 fake(生产恒 nil)。
	newClient func() (dshClient, error)

	// dataDir daemon 数据目录(凭据文件 dsh-credentials.env 的所在地)。生产上
	// 由启动接线传入真实值;当前约束下不触碰 main/server,故默认空 → 回落
	// dshapp 默认值 ~/.pocketagentd(见 resolvedDataDir)。字段只为"尊重显式
	// 数据目录"留口,不直接读全局 flag,保持本包自洽。
	dataDir string
}

func NewDshProvider() *DshProvider {
	return &DshProvider{
		threadMap: map[string]threadRef{},
		modelVals: map[string]string{},
	}
}

// CollabReady 见 CollabAware。dsh 的协同走 ACP 的**会话级** mcpServers
// (session/new 参数),所以判据是"cmd 层是否已把 collab 服务器注入进来" ——
// 由 dshapp.SessionMCPServers 报告,空即未接线。
func (p *DshProvider) CollabReady() bool { return len(dshapp.SessionMCPServers()) > 0 }

func (p *DshProvider) ID() string          { return "dsh" }
func (p *DshProvider) DisplayName() string { return "DeepSeek Harness(桌面 agent)" }

// ---- 子进程生命周期(对标 codex.withClientCtx:TryLock/超时上界/重启一次)----

var (
	dshBusy        = errors.New("dsh 正忙(另一个操作正在初始化/探测),请稍后重试")
	dshErrNoClient = errors.New("dsh 子进程未启动")
)

func (p *DshProvider) withClient(fn func(context.Context, dshClient) error) error {
	return p.withClientCtx(context.Background(), dshInitBudget, fn)
}

// lockLife 有界排队获取生命周期锁:先 TryLock(无争用零开销),失败后在 wait
// 窗口内短间隔重试 —— 排队而非立即拒绝(pi/zcode 排队语义,不 busy-reject),
// 把"模型取样/握手刚好人挤人"的瞬时争用吸收成先后等待;窗口耗尽仍拿不到
// (持有方真卡死)才返回 false → dshBusy,保留 TryLock 模式"一个卡死的操作
// 不拖住全线"的底线(codex/zcode 同款三点纪律的第 1 点由此弱化为"有界拖")。
func (p *DshProvider) lockLife(wait time.Duration) bool {
	if p.lifeMu.TryLock() {
		return true
	}
	if wait <= 0 {
		return false
	}
	deadline := time.Now().Add(wait)
	for {
		time.Sleep(25 * time.Millisecond)
		if p.lifeMu.TryLock() {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
	}
}

// withClientCtx 在保证子进程活着且已完成 initialize 的前提下执行 fn;
// 失败自动重启一次。ctx 无截止时间时套默认上界(8s,探针实测冷启动 1.4s)。
//
// 【T-DSH 问题 B】锁获取从"TryLock 立即 busy-reject"改为"有界排队":
// 排队窗口 = min(dshLockWait, 调用方自身剩余截止时间),绝不超过调用方
// 本来就愿意等待的时长 —— ctx 传导不受影响(dshapp.callTimeout 逐请求
// select ctx.Done,超时即返回并 forget pending,无僵尸持锁,已核)。
func (p *DshProvider) withClientCtx(ctx context.Context, budget time.Duration, fn func(context.Context, dshClient) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	wait := dshLockWait
	if d, hasDeadline := ctx.Deadline(); hasDeadline {
		if rem := time.Until(d); rem < wait {
			wait = rem
		}
	}
	if !p.lockLife(wait) {
		return dshBusy
	}
	defer p.lifeMu.Unlock()
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		if budget <= 0 {
			budget = dshInitBudget
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, budget)
		defer cancel()
	}

	try := func() error {
		if p.client == nil {
			return dshErrNoClient
		}
		if !p.inited {
			caps, err := p.client.Initialize(ctx)
			if err != nil {
				return err
			}
			p.caps = caps
			p.inited = true
			log.Printf("[dsh] initialize 握手完成 protocol=%d agent=%s@%s image=%t list/resume/close=%t/%t/%t",
				caps.ProtocolVersion, caps.AgentInfo.Name, caps.AgentInfo.Version,
				caps.SupportsImageInput(), caps.CanList(), caps.CanResume(), caps.CanClose())
		}
		return fn(ctx, p.client)
	}
	err := try()
	// 超时/取消不是"子进程挂了,重启能救"的情况:如实返回(codex 同款)。
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	// 轮次执行中状态查询失败不重启共享子进程(会切断正在跑的流)。
	if err != nil && !errors.Is(err, dshErrNoClient) && p.turnActive.Load() {
		return err
	}
	if err == nil || errors.Is(err, dshErrNoClient) {
		if errors.Is(err, dshErrNoClient) {
			// 首次拉起
			if _, serr := p.startClientLocked(); serr != nil {
				return serr
			}
			err = try()
		}
		return err
	}
	// 子进程可能挂了:杀掉重来一次(会话缓存随 gen 作废,重挂载走 ExternalSession)
	p.dropClientLocked()
	if _, serr := p.startClientLocked(); serr != nil {
		return serr
	}
	log.Printf("[dsh] 子进程已重启(gen=%d),会话缓存作废,后续走 session/list+resume 对账", p.gen.Load())
	return try()
}

// startClientLocked 拉起新子进程并登记(调用方须持 lifeMu)。
func (p *DshProvider) startClientLocked() (dshClient, error) {
	var c dshClient
	var err error
	if p.newClient != nil {
		c, err = p.newClient()
	} else {
		// 凭据注入:优先用进程 env(daemon 启动时已 export 的),缺失时以
		// dsh-credentials.env 文件补充(绝不强压进程 env,文件只作 fallback)。
		// 文件只在 daemon 数据目录内(默认 ~/.pocketagentd),不碰用户 ~/.dsh。
		extra := dshapp.LoadCredentialEnv(p.dataDir)
		c, err = dshapp.Start(dshapp.StartOptions{ExtraEnv: extra, DataDir: p.dataDir})
	}
	if err != nil {
		return nil, err
	}
	p.client, p.inited, p.caps = c, false, nil
	if h, ok := c.(interface {
		SetUnknownResponseHandler(func(int64, json.RawMessage))
	}); ok {
		h.SetUnknownResponseHandler(func(id int64, raw json.RawMessage) {
			recordUnknownResponse("dsh", fmt.Sprintf("%d", id), raw)
		})
	}
	if h, ok := c.(interface {
		SetGlobalHandler(func(string, json.RawMessage))
	}); ok {
		h.SetGlobalHandler(func(method string, params json.RawMessage) {
			recordProviderEvent("dsh", method, "", dshNoticeKind(method), params)
		})
	}
	p.gen.Add(1)
	return c, nil
}

// resolvedDataDir 返回用于凭据文件定位的数据目录:provider 持有显式目录时优先,
// 否则回落 dshapp 默认值(与 StartOptions.DataDir 同源,见 dshapp.DefaultDataDir)。
func (p *DshProvider) resolvedDataDir() string {
	if p.dataDir != "" {
		return p.dataDir
	}
	return dshapp.DefaultDataDir()
}

// dropClientLocked 关闭并清空当前子进程(调用方须持 lifeMu)。
func (p *DshProvider) dropClientLocked() {
	if p.client != nil {
		p.client.Close()
	}
	p.client, p.inited, p.caps = nil, false, nil
}

// Warmup daemon 启动后台预热:拉起子进程并完成 initialize 握手(对标
// codex.Warmup 在 cmd/pocketagentd/main.go 的启动接线)。
//
// 【为什么 dsh 的预热不能像 codex 一样一次性】codex 首包 ~1s;dsh 首跑
// (DSH_HOME 为空)实测握手可拖到十位数分钟(2026-09-21 真机:16:19:36 拉起
// → 16:38:22 完成),期间用户路径与慢握手撞锁就是"dsh 正忙"的根因。预热
// 把这次慢启动吸收到启动期:握手成功一次后 inited 恒真,用户路径就绪检查
// 变成瞬时通过。单次预算 dshWarmupBudget(2min,持有 lifeMu 期间用户调用
// 走 lockLife 有界排队);失败按 dshWarmupRetry 间隔重试至多
// dshWarmupRetries 次,窗口耗尽如实落日志并退出 —— 不阻塞启动、不影响
// 之后的懒启动路径(用户随时可抢先完成握手,两条路殊途同归)。
// dshWarmup* 为包级变量仅为测试注入短值,生产语义见上方注释。
func (p *DshProvider) Warmup() {
	for i := 0; i < dshWarmupRetries; i++ {
		err := p.withClientCtx(context.Background(), dshWarmupBudget, func(context.Context, dshClient) error {
			return nil
		})
		switch {
		case err == nil:
			if i > 0 {
				log.Printf("[dsh] 预热握手完成(第 %d 次尝试)", i+1)
			}
			return
		case errors.Is(err, dshBusy):
			// 已有别的调用在持锁(大概率在做同一件事):不空转,等下个周期再看。
			log.Printf("[dsh] 预热遇忙(其他调用持锁),%v 后复查", dshWarmupRetry)
		default:
			log.Printf("[dsh] 预热第 %d/%d 次未完成握手: %v", i+1, dshWarmupRetries, err)
		}
		time.Sleep(dshWarmupRetry)
	}
	log.Printf("[dsh] 预热窗口耗尽仍未完成握手(不阻塞启动;后续请求仍可懒启动)")
}

// ---- Provider 接口 ----

// Models 手机端模型列表。
//
// 【数据源唯一且无静态名单】session/new 应答携带的 configOptions(探针第 2 项:
// model 是双层编码串二元组、effort 档位 off/low/high/max)。无活动会话时用
// 一次性瞬态会话取样后 session/close。缓存 + TTL;失败时至少保住 default 条目
// (绝不返回空列表冒充成功,zcode 同款)。
func (p *DshProvider) Models() []pb.ModelInfo {
	p.modelsMu.Lock()
	if len(p.models) > 0 && time.Since(p.modelsAt) < dshModelsTTL {
		out := p.models
		p.modelsMu.Unlock()
		return out
	}
	p.modelsMu.Unlock()

	out := []pb.ModelInfo{{
		Id: "default", Provider: "dsh", DisplayName: "DeepSeek(跟随桌面配置)", Available: true,
	}}
	fetched, cat, err := p.fetchModels()
	if err != nil {
		log.Printf("[dsh] 模型列表探测失败(返回 default 兜底): %v", err)
		return out
	}
	out = append(out, fetched...)
	p.modelsMu.Lock()
	p.models, p.modelsAt = out, time.Now()
	p.modelsMu.Unlock()
	p.setCatalog(cat)
	return out
}

// InvalidateModels 清缓存(ListModels refresh=true 路径自动生效)。
func (p *DshProvider) InvalidateModels() {
	p.modelsMu.Lock()
	p.models, p.modelsAt = nil, time.Time{}
	p.modelsMu.Unlock()
}

// fetchModels 用一次性瞬态会话取样模型目录。
func (p *DshProvider) fetchModels() ([]pb.ModelInfo, dshapp.ModelCatalog, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dshModelsBudget)
	defer cancel()
	var out []pb.ModelInfo
	var cat dshapp.ModelCatalog
	err := p.withClientCtx(ctx, dshInitBudget, func(ctx context.Context, c dshClient) error {
		st, err := c.SessionNew(ctx, os.TempDir()) // 取样会话用系统临时目录,不作业
		if err != nil {
			return err
		}
		defer func() {
			closeCtx, cc := context.WithTimeout(context.Background(), dshInitBudget)
			defer cc()
			_ = c.SessionClose(closeCtx, st.SessionID) // 用完即毁
		}()
		cat = dshapp.ParseModelCatalog(st.ConfigOptions)
		out = dshModelsFromCatalog(cat)
		// 完整原生目录顺手登记(原生设置透出的缓存与模型面同源刷新)。
		if len(st.ConfigOptions) > 0 {
			p.catMu.Lock()
			p.setRawCatalog(st.ConfigOptions)
			p.catMu.Unlock()
		}
		return nil
	})
	if err != nil {
		return nil, cat, err
	}
	return out, cat, nil
}

// dshModelsFromCatalog 目录 → 模型列表(纯映射;非法叶子跳过,不猜)。
func dshModelsFromCatalog(cat dshapp.ModelCatalog) []pb.ModelInfo {
	out := make([]pb.ModelInfo, 0, len(cat.ModelOptions))
	seen := map[string]bool{}
	for _, m := range cat.ModelOptions {
		_, model, ok := dshapp.DecodeModelTuple(m.Value)
		if !ok {
			continue
		}
		id := "dsh/" + model
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, pb.ModelInfo{})
		info := &out[len(out)-1]
		info.Id, info.Provider, info.Available = id, "dsh", true
		info.DisplayName = m.Name
		if info.DisplayName == "" {
			info.DisplayName = model
		}
		// 档位是 agent 级目录(实测 4 档),随每个模型透出;有档位即声明思考能力。
		if len(cat.Efforts) > 0 {
			info.SupportsThinking = true
			info.AvailableEfforts = append([]string(nil), cat.Efforts...)
		}
	}
	return out
}

// setCatalog 登记最近一次目录(切换模型时取原样编码串的唯一来源)。
func (p *DshProvider) setCatalog(cat dshapp.ModelCatalog) {
	vals := map[string]string{}
	for _, m := range cat.ModelOptions {
		if _, model, ok := dshapp.DecodeModelTuple(m.Value); ok {
			vals["dsh/"+model] = m.Value
		}
	}
	p.catMu.Lock()
	p.catalog = cat
	p.modelVals = vals
	p.catMu.Unlock()
}

// refreshCatalogFromResume 探针修正②:session/resume 应答带 configOptions,
// 重挂载后据此刷新模型缓存(桌面端目录可能在 daemon 停机期间变了)。
func (p *DshProvider) refreshCatalogFromResume(opts []dshapp.ConfigOption) {
	if len(opts) == 0 {
		return
	}
	p.catMu.Lock()
	p.setRawCatalog(opts)
	p.catMu.Unlock()
	p.setCatalog(dshapp.ParseModelCatalog(opts))
}

// ---- Run:一轮 ----

func (p *DshProvider) Run(ctx context.Context, req TurnRequest, sink Sink, approve ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	p.turnMu.Lock() // 同一时刻只跑一轮(手机端本来也是单会话交互)
	defer p.turnMu.Unlock()
	p.turnActive.Store(true)
	defer p.turnActive.Store(false)

	// 0) 子进程就绪(懒启动 + initialize 握手;失败如实上抛)
	if err := p.withClient(func(context.Context, dshClient) error { return nil }); err != nil {
		return nil, nil, err
	}

	// 1) 输入收集:文本零裁剪(硬约定);图片按能力协商如实报错(R4)。
	var userText strings.Builder
	hasImage := false
	for i := range req.InputBlocks {
		switch t := req.InputBlocks[i].Block.(type) {
		case *pb.ContentBlock_Text:
			userText.WriteString(t.Text.Text)
		case *pb.ContentBlock_Image:
			hasImage = true
		}
	}
	if hasImage && (p.caps == nil || !p.caps.SupportsImageInput()) {
		return nil, nil, errors.New("dsh 当前路由不支持图片输入(promptCapabilities.image=false),请改用文本输入")
	}

	// 2) 会话就绪:同代内存缓存 → ExternalSession 重挂载(list 校验+resume)→ 新建
	sid, err := p.ensureSession(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	// 3) 偏好落配置(PrefsAware 子集:model/effort → set_config_option)
	if err := p.applyConfigOptions(ctx, sid, req); err != nil {
		return nil, nil, err
	}

	// 4) 事件路由:readLoop 里同步调 notif 钩子,慢 sink 会堵整条管道 —— 只入队,
	// 单消费者 goroutine 保序分发(codex 同款)。enqueued/processed 组成排空栅栏:
	// prompt 应答(update 与应答分路竞速)到达后,先把已入队帧消费完再读终局。
	st := &dshTurnState{}
	var notices nativeNoticeRecorder
	var enqueued, processed atomic.Int64
	type notifMsg struct {
		method string
		params json.RawMessage
	}
	notifCh := make(chan notifMsg, 4096)
	notifStop := make(chan struct{})
	notifDone := make(chan struct{})
	notifHandler := func(method string, params json.RawMessage) {
		st.lastNotif.Store(time.Now().UnixMilli())
		enqueued.Add(1)
		select {
		case notifCh <- notifMsg{method: method, params: params}:
		case <-notifStop:
			enqueued.Add(-1)
		}
	}
	go func() {
		defer close(notifDone)
		for {
			select {
			case nm := <-notifCh:
				if nm.method == "session/update" {
					var env dshUpdateEnvelope
					if json.Unmarshal(nm.params, &env) != nil || env.SessionID != sid {
						processed.Add(1)
						continue // 坏帧/别的会话的帧不进本轮(诊断可开 PA_DSH_DEBUG)
					}
					p.handleSessionUpdate(env.Update, sink, st, &notices)
					processed.Add(1)
					continue
				}
				// 非 session/update 的通知:识别不了≠出错,原样包装进诊断视图(主线 A)
				notices.recordAs(dshNoticeKind(nm.method), "dsh", nm.method, nm.params, sink)
				processed.Add(1)
			case <-notifStop:
				return
			}
		}
	}()
	reqHandler := func(id int64, method string, params json.RawMessage) {
		log.Printf("[dsh] 反向请求 %s 原始参数: %.400s", method, string(params))
		p.handleDshRequest(p.client, id, method, params, ctx, sink, approve, &st.approvalWaiting)
	}
	p.client.SetTurnHandlers(notifHandler, reqHandler)
	defer func() {
		p.client.SetTurnHandlers(nil, nil)
		p.client.SyncHandlers()
		close(notifStop)
		<-notifDone
	}()

	// 5) 发起本轮。prompt 的应答在轮次终局才回来(stopReason),过程走 update 通知,
	// 传输层不设 120s 兜底(见 dshapp.PromptCtx);超时归下面的看门狗。
	blocks := []map[string]interface{}{{"type": "text", "text": userText.String()}}
	type promptOutcome struct {
		raw json.RawMessage
		err error
	}
	outCh := make(chan promptOutcome, 1)
	log.Printf("[dsh] session/prompt session=%s 输入=%d字", sid, userText.Len())
	st.lastNotif.Store(time.Now().UnixMilli())
	turnStart := time.Now()
	go func() {
		raw, err := p.client.Prompt(ctx, sid, blocks)
		outCh <- promptOutcome{raw: raw, err: err}
	}()

	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	var res promptOutcome
	waitDone := false
	for !waitDone {
		select {
		case res = <-outCh:
			waitDone = true
		case <-ctx.Done():
			// 手机断连/取消:先通知 dsh 终止本轮(prompt 调用会随 ctx 取消返回)。
			p.client.Cancel(sid)
			partial := st.snapshotText()
			<-outCh // 等 goroutine 退出(channel 带缓冲,必达)
			if partial != "" {
				return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: partial}}}}, &pb.Usage{}, ctx.Err()
			}
			return nil, nil, ctx.Err()
		case <-tick.C:
			if st.approvalWaiting.Load() {
				continue // 审批等待是合法静默(用户在手机上想多久都行)
			}
			if ms := st.lastNotif.Load(); ms > 0 && time.Now().UnixMilli()-ms > dshSilenceWatchdog.Milliseconds() {
				p.client.Cancel(sid)
				return nil, nil, fmt.Errorf("dsh %v 无任何事件(任务可能未被接收,session=%s)", dshSilenceWatchdog, sid)
			}
			if time.Since(turnStart) > dshTurnCap {
				p.client.Cancel(sid)
				return nil, nil, fmt.Errorf("dsh 轮次超时(%v),已发取消", dshTurnCap)
			}
		}
	}

	if res.err != nil {
		if ctx.Err() != nil {
			partial := st.snapshotText()
			if partial != "" {
				return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: partial}}}}, &pb.Usage{}, ctx.Err()
			}
			return nil, nil, ctx.Err()
		}
		return nil, nil, p.dshRunErrorFrom(res.err)
	}
	drainEvents(&enqueued, &processed) // 排空栅栏:update 帧先于终局被消费,不丢尾部事件
	// 终局 stopReason 映射(蓝本 §4.3:end_turn → 轮次完成;cancelled → TurnFailed)
	stop := dshStopReason(res.raw)
	switch stop {
	case "", "end_turn":
		text := st.snapshotText()
		log.Printf("[dsh] 一轮完成 session=%s 用时 %v 回复 %d 字", sid, time.Since(turnStart).Round(time.Millisecond), len([]rune(text)))
		// 【空就是空】见 codex.go 同处注释:占位文本会把"真的没话说""只有产物"
		// "适配器丢正文"压成同一条普通 assistant 文本,用户无从判断该不该重试。
		// 如实返回空块,OutputState 由 server 层按 blocks 判定。
		if text == "" {
			return nil, &pb.Usage{}, nil
		}
		return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}}}}, &pb.Usage{}, nil
	case "cancelled":
		partial := st.snapshotText()
		return nil, nil, &dshRunError{info: &pb.Error{
			Code: "CANCELLED", Message: "dsh 轮次已取消", Details: "stopReason=cancelled", Retryable: true,
		}, partial: partial}
	default:
		// 其余 stopReason(max_tokens/refusal/...)如实终局报错,不加工
		return nil, nil, &dshRunError{info: &pb.Error{
			Code: "STOP_" + strings.ToUpper(stop), Message: "dsh 轮次终止: " + stop, Details: "stopReason=" + stop,
		}, partial: st.snapshotText()}
	}
}

// dshTurnState 是一轮内的可变事件状态(跨 update 帧累积)。
type dshTurnState struct {
	mu              sync.Mutex
	streamText      strings.Builder
	lastNotif       atomic.Int64
	approvalWaiting atomic.Bool
}

func (s *dshTurnState) appendText(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streamText.WriteString(text)
}

func (s *dshTurnState) snapshotText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streamText.String()
}

// drainEvents 等"已入队的事件全部被消费"(上限 5s):update 流与 prompt 应答
// 分路竞速,终局先到时必须先消费完已入队帧再读终稿,否则尾部文本可能丢。
func drainEvents(enqueued, processed *atomic.Int64) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if processed.Load() >= enqueued.Load() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	log.Printf("[dsh] 排空事件栅栏超时(5s): enqueued=%d processed=%d", enqueued.Load(), processed.Load())
}

// dshStopReason 从 prompt 应答取 stopReason(缺省按 end_turn,防御性)。
func dshStopReason(raw json.RawMessage) string {
	var r struct {
		StopReason string `json:"stopReason"`
	}
	if json.Unmarshal(raw, &r) != nil {
		return ""
	}
	return strings.TrimSpace(r.StopReason)
}

// ---- 会话就绪(重挂载握手纪律:校验不能省,pi.go 同款)----

// ensureSession 优先级:同代内存缓存 → ExternalSession 重挂载 → 新建。
func (p *DshProvider) ensureSession(ctx context.Context, req TurnRequest) (string, error) {
	gen := int(p.gen.Load())
	if ref, hit := p.dshThreadRefFor(req.Thread.ID); hit && ref.gen == gen {
		return ref.id, nil
	}
	if ext := req.Thread.ExternalSession; ext.SessionID != "" {
		sid, catOpts, err := p.remountSession(ctx, ext)
		if err != nil {
			// 一次性降级(唯一出口):会话确实在列、resume 仍被 agent 拒绝(-32602)
			// = "agent 升级导致旧会话不可恢复"而非网络抖动/数据错位。此时若只是
			// 如实报错,手机端重试永远重放同一路径,这个会话从此发不出任何消息、
			// 用户没有出路(真机实锤,l2-20260925/RESULT.md 适配债 1)。
			var re *dshRemountError
			if errors.As(err, &re) && re.downgradeable {
				return p.rebuildSessionAfterUpgrade(ctx, req, ext, gen, err)
			}
			// 显式报错,不静默新开 —— "不 resume 就静默吞请求"的三方共同教训,
			// 以及蓝本 §4.3:不在列 → 显式报错。
			return "", err
		}
		p.setDshThreadRef(req.Thread.ID, threadRef{id: sid, gen: gen})
		p.refreshCatalogFromResume(catOpts) // 探针修正②
		return sid, nil
	}
	return p.createSession(ctx, req, gen)
}

// createSession session/new 新建并登记:threadMap 写入 + ExternalSession 写回
// (server 层轮次后落库,手机端据此展示外部会话引用;zcode 同款机制)+ 灌"新
// 会话初始值"。ensureSession 的"无 ExternalSession"常规新建分支与升级降级
// 重建共用,保证两条路的登记语义完全一致。
func (p *DshProvider) createSession(ctx context.Context, req TurnRequest, gen int) (string, error) {
	st, err := p.client.SessionNew(ctx, req.Workspace)
	if err != nil {
		return "", fmt.Errorf("session/new: %w", err)
	}
	p.setDshThreadRef(req.Thread.ID, threadRef{id: st.SessionID, gen: gen})
	req.Thread.ExternalSession = store.ExternalSessionRef{SessionID: st.SessionID, Cwd: req.Workspace}
	log.Printf("[dsh] 新建会话 %s cwd=%s", st.SessionID, req.Workspace)
	// 灌"新会话初始值"(设置页原生设置;探针实证 set_config_option 仅会话内生效
	// 不持久,新会话在这里灌入 —— 与"更改默认对新建对话生效"的设置页语义对齐。
	// 尽力而为:单项失败只落日志,不阻塞本轮;applyConfigOptions 随后照常覆盖
	// model/effort 的显式偏好)。
	p.applyNativeDefaults(ctx, st.SessionID)
	return st.SessionID, nil
}

// rebuildSessionAfterUpgrade 一次性降级:原会话因 agent 升级不可恢复(resume
// 被拒且 list 确认在列)→ 自动 session/new 新开会话,ExternalSessionRef 写回
// 新 sessionId(store 落库由 server 层轮次后完成,与常规新建同机制),后续轮次
// 稳定在新会话上。三条纪律:
//   - 只在用户主动发消息的路径发生(唯一调用链 ensureSession ← Run):发消息 =
//     明确要继续用这个会话,降级才是用户意志;后台/探测路径绝不自动触发;
//   - 前提是 remountSession 已判定 downgradeable(list 在列 + resume -32602,
//     dsh-resume-compat-probe.md 实测形态):数据错位(list 里根本没有)永远
//     显式拒绝,绝不降级 —— 那不是升级问题,静默新开会把错位掩盖掉;
//   - 绝不伪造旧上下文:新会话就是新会话,日志写明原委与原 sessionId(用户仍可
//     在桌面端找回旧记录),手机端不再看到"resume 失败且重试必败"的死路。
func (p *DshProvider) rebuildSessionAfterUpgrade(ctx context.Context, req TurnRequest, old store.ExternalSessionRef, gen int, cause error) (string, error) {
	sid, err := p.createSession(ctx, req, gen)
	if err != nil {
		// 降级也失败:两个失败都如实说出,不吞任何一个。
		return "", fmt.Errorf("dsh 原会话 %s 不可恢复(%v),自动重建新会话又失败: %w", old.SessionID, cause, err)
	}
	log.Printf("[dsh] 原会话 %s 因 agent 升级不可恢复(resume 被拒:%v),已自动重建新会话 %s 替换;旧上下文不再延续,本轮起在新会话继续对话", old.SessionID, cause, sid)
	return sid, nil
}

// applyNativeDefaults 把设置页记下的"新会话初始值"灌进一个刚建立的会话。
// 稳定顺序写入(map 迭代无序);单项失败不中断(下一项照灌),失败如实落日志。
func (p *DshProvider) applyNativeDefaults(ctx context.Context, sid string) {
	defaults := p.nativeDefaultsSnapshot()
	for _, id := range dshSortedDefaultIDs(defaults) {
		if err := p.client.SetConfigOption(ctx, sid, id, defaults[id]); err != nil {
			log.Printf("[dsh] 灌新会话初始值 %s=%s 失败(跳过): %v", id, defaults[id], err)
		}
	}
}

// remountSession daemon 重启后的重挂载:
//  1. 能力门控:sessionCapabilities.resume/list 不在场 → 显式报错(§5.2,不猜);
//  2. session/list 校验在列(探针修正④:按 sessionId+cwd 双因子;无 title 可比);
//  3. 在列才 session/resume(log 恢复不重放旧 update,正好避免手机端重复帧)。
//
// 【cwd 传递(rc.3 升级兼容,dsh-resume-compat-probe.md 实测)】rc.3 的 resume
// zod 必填 cwd:优先用落库的 ext.Cwd,缺失时用 list 校验命中的那条目里的 cwd
// (同一次校验顺手取得,不再多猜一个目录)。
//
// 【降级分类】返回 *dshRemountError:downgradeable=true 仅当"list 校验确认会话
// 在列、resume 仍报 -32602"—— 列表里有它说明数据没错位,是 agent 侧不认这份数据
// (升级格式断裂);此时由调用方(ensureSession,用户发消息路径)决定一次性重建。
// list 不在列(数据错位)、list 调用失败、能力缺席等一律 false:维持显式拒绝,
// 不静默新开("不 resume 就静默吞请求"的三方共同教训)。
func (p *DshProvider) remountSession(ctx context.Context, ext store.ExternalSessionRef) (string, []dshapp.ConfigOption, error) {
	if p.caps != nil && !p.caps.CanResume() {
		return "", nil, &dshRemountError{fmt.Errorf("dsh %s 不支持会话恢复(sessionCapabilities.resume 不在场),无法重挂载会话 %s", p.caps.AgentInfo.Version, ext.SessionID), false}
	}
	resumeCwd := ext.Cwd
	if p.caps != nil && p.caps.CanList() {
		entries, err := p.client.SessionList(ctx)
		if err != nil {
			return "", nil, &dshRemountError{fmt.Errorf("重挂载前 session/list 校验失败: %w", err), false}
		}
		entry, listed := dshSessionEntry(entries, ext)
		if !listed {
			return "", nil, &dshRemountError{fmt.Errorf("dsh 会话 %s 不在可恢复列表中(session/list 校验未通过),不静默新开会话;如需继续请删除该会话后重建", ext.SessionID), false}
		}
		if resumeCwd == "" {
			resumeCwd = entry.Cwd
		}
	}
	st, err := p.client.SessionResume(ctx, ext.SessionID, resumeCwd)
	if err != nil {
		// 降级条件(唯一形态):会话在列 + resume 报 -32602。实测依据:rc.3 对
		// "参数不合规"与"会话不可恢复"都报 -32602(dsh-resume-compat-probe.md
		// A1/B 两帧)——参数问题已由 cwd 补参治本,走到这里的 -32602 就是
		// "agent 拒绝恢复这份数据"(如未来日志格式再断裂),属升级兼容问题。
		var re *dshapp.RPCError
		downgradeable := errors.As(err, &re) && re.Code == -32602
		return "", nil, &dshRemountError{fmt.Errorf("session/resume %s 失败: %w", ext.SessionID, err), downgradeable}
	}
	log.Printf("[dsh] 已重挂载会话 %s", ext.SessionID)
	return st.SessionID, st.ConfigOptions, nil
}

// dshRemountError 重挂载失败的分类载体:downgradeable 报告是否满足一次性降级
// 条件(见 remountSession 注释),ensureSession 据此决定重建;错误原文原样携带,
// 不降级时手机端看到的仍是显式报错原文。
type dshRemountError struct {
	err          error
	downgradeable bool
}

func (e *dshRemountError) Error() string { return e.err.Error() }
func (e *dshRemountError) Unwrap() error { return e.err }

// dshSessionEntry 按 sessionId+cwd 双因子找 list 条目(探针修正④:list 无
// title);一侧 cwd 缺失退化为单因子;Windows 路径大小写不敏感。找到返回该条目
// (resume 的 cwd 兜底来源),找不到 ok=false。
func dshSessionEntry(entries []dshapp.SessionListEntry, ext store.ExternalSessionRef) (dshapp.SessionListEntry, bool) {
	for _, e := range entries {
		if e.SessionID != ext.SessionID {
			continue
		}
		if ext.Cwd == "" || e.Cwd == "" {
			return e, true
		}
		if strings.EqualFold(filepath.Clean(e.Cwd), filepath.Clean(ext.Cwd)) {
			return e, true
		}
	}
	return dshapp.SessionListEntry{}, false
}

// dshSessionListed 双因子在列判定(dshSessionEntry 的谓词形态;既有回归钉用)。
func dshSessionListed(entries []dshapp.SessionListEntry, ext store.ExternalSessionRef) bool {
	_, ok := dshSessionEntry(entries, ext)
	return ok
}

func (p *DshProvider) setDshThreadRef(id string, ref threadRef) {
	p.threadMu.Lock()
	defer p.threadMu.Unlock()
	p.threadMap[id] = ref
}

func (p *DshProvider) dshThreadRefFor(id string) (threadRef, bool) {
	p.threadMu.Lock()
	defer p.threadMu.Unlock()
	ref, ok := p.threadMap[id]
	return ref, ok
}

// ---- PrefsAware 子集(编译期断言见 provider.go)----

// ApplyPrefs 桌面偏好快照(完整覆盖,server 层已按 presence 合并)。
// 【只消费语义上说得通的子集】Model/Effort → 每轮 set_config_option;
// HideReasoning → 事件路径过滤 ThinkingDelta。审批策略/沙箱不映射
// (dsh ACP 固定一次性审批,无策略面可配 —— 蓝本 §4.3 如实边界)。
func (p *DshProvider) ApplyPrefs(pref DesktopPrefs) {
	p.prefsMu.Lock()
	defer p.prefsMu.Unlock()
	p.prefs = pref
}

func (p *DshProvider) hideReasoning() bool {
	p.prefsMu.Lock()
	defer p.prefsMu.Unlock()
	return p.prefs.HideReasoning
}

// applyConfigOptions 把"线程胶囊显式 > 线程设置快照 > 桌面默认"的模型/effort
// 映射进 session/set_config_option。
//
// 【红线(探针修正①)】model 的 value 只允许原样回传目录里的双层编码串,
// 禁止发送裸模型名:目录里找不到编码串时跳过并落日志,绝不现拼。
func (p *DshProvider) applyConfigOptions(ctx context.Context, sid string, req TurnRequest) error {
	p.prefsMu.Lock()
	modelID, effort := p.prefs.Model, p.prefs.Effort
	p.prefsMu.Unlock()
	if snap := req.Thread.SettingsSnapshot; snap != nil { // 快照语义:新建对话那一刻的值
		modelID, effort = snap.Model, snap.Effort
	}
	if m := req.Thread.ModelID; m != "" {
		modelID = m
	}
	if e := req.Thread.Effort; e != "" {
		effort = e
	}
	if spec := dshModelSpec(modelID); spec != "" {
		v, ok := p.encodedModelValue(spec)
		if !ok {
			// 目录缺失:刷新一次再找(桌面端目录可能刚变)
			p.InvalidateModels()
			p.Models()
			v, ok = p.encodedModelValue(spec)
		}
		if ok {
			if err := p.client.SetConfigOption(ctx, sid, "model", v); err != nil {
				return fmt.Errorf("切换模型失败: %w", err)
			}
		} else {
			log.Printf("[dsh] 模型 %s 在目录中找不到原样编码串,跳过切换(禁止发送裸模型名)", modelID)
		}
	}
	if effort != "" {
		if err := p.client.SetConfigOption(ctx, sid, "reasoning_effort", effort); err != nil {
			return fmt.Errorf("切换思考强度失败: %w", err)
		}
	}
	return nil
}

// dshModelSpec 从模型 id 提取 dsh 侧 spec:"dsh/<spec>" → "<spec>";
// "default"(跟随桌面)与其他 agent 前缀 → ""(不覆盖)。
func dshModelSpec(modelID string) string {
	s := strings.TrimPrefix(strings.TrimSpace(modelID), "dsh/")
	if s == "" || s == "default" || s == modelID {
		return ""
	}
	return s
}

// encodedModelValue 取 spec 对应的原样编码串(不重编码 —— "原样回传"原则)。
func (p *DshProvider) encodedModelValue(spec string) (string, bool) {
	p.catMu.Lock()
	defer p.catMu.Unlock()
	if v, ok := p.modelVals["dsh/"+spec]; ok {
		return v, true
	}
	return "", false
}

// ---- 事件映射:session/update → TurnServerMessage ----

// dshUpdateEnvelope 是 session/update 通知的信封(探针修正⑤:update 体只透传
// RawMessage,本层不猜形状;变体识别见 handleSessionUpdate)。
type dshUpdateEnvelope struct {
	SessionID string          `json:"sessionId"`
	Update    json.RawMessage `json:"update"`
}

// handleSessionUpdate 把一条 update 体映射成手机帧(纯分发,单测直测)。
func (p *DshProvider) handleSessionUpdate(raw json.RawMessage, sink Sink, st *dshTurnState, notices *nativeNoticeRecorder) {
	kind := dshUpdateKind(raw)
	switch kind {
	case "agent_message_chunk":
		// 块级文本(上游 #594 落地前不是逐字 delta,见包注释)
		if text := dshUpdateText(raw); text != "" {
			st.appendText(text)
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: text}}})
		}
	case "agent_thought_chunk":
		// HideReasoning 过滤(daemon 侧行为改变,不依赖 dsh 原生开关)
		if p.hideReasoning() {
			return
		}
		if text := dshUpdateText(raw); text != "" {
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Delta: text}}})
		}
	case "tool_call", "tool_call_update":
		p.handleDshToolUpdate(kind, raw, sink, st, notices)
	case "user_message_chunk":
		// 自己的输入回显:手机端已展示,不转发
	case "context_usage":
		// 用量帧:识别出任一 token 字段才发 UsageUpdate;认不出就走诊断透传,不伪造
		if u := dshUsageUpdate(raw); u != nil {
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Usage{Usage: u}})
			return
		}
		notices.recordAs(dshNoticeKind("session/update:context_usage"), "dsh", "session/update:context_usage", raw, sink)
	case "usage_update":
		// 0.1.5-rc.3 探针实测(dsh-p0-probe.md §3):每轮至少 1 帧
		// {"sessionUpdate":"usage_update","used":N,"size":M} —— used/size 是
		// 上下文窗口占用(≈codex last 口径的 total/窗口),与 context_usage 的
		// token 计数是两种形状。此前它落 default 变 UNKNOWN,每轮必来必脏台账。
		// used+size 都在场才投影(字段缺失不猜);与 context_usage 同一 UsageUpdate 通道。
		if u := dshUsageUpdateFromUsedSize(raw); u != nil {
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Usage{Usage: u}})
			return
		}
		notices.recordAs(dshNoticeKind("session/update:usage_update"), "dsh", "session/update:usage_update", raw, sink)
	case "plan", "available_commands_update":
		// 透传进诊断视图(蓝本 §4.3:plan / available_commands 透传/忽略分支)
		notices.recordAs(dshNoticeKind("session/update:"+kind), "dsh", "session/update:"+kind, raw, sink)
	default:
		// 探针修正⑤:未知变体原样透传进诊断视图,不猜(README §7 主线 A:
		// 识别不了≠出错;原文+变体名进诊断视图,App 显示"原生发来了一个新事件")
		notices.recordAs(dshNoticeKind("session/update:"+kind), "dsh", "session/update:"+kind, raw, sink)
	}
}

// dshUpdateKind 取变体判别字段。防御性读两个候选名:ACP v1 规范用
// "sessionUpdate";部分实现习惯 "type"。这只是"读判别名",变体内容仍原样
// 透传不猜;真实形状待配 key 真机验证(验证清单有专项)。
func dshUpdateKind(raw json.RawMessage) string {
	var probe struct {
		SessionUpdate string `json:"sessionUpdate"`
		Type          string `json:"type"`
	}
	if json.Unmarshal(raw, &probe) != nil {
		return ""
	}
	if probe.SessionUpdate != "" {
		return probe.SessionUpdate
	}
	return probe.Type
}

// dshUpdateText 提取文本块的 text(ACP ContentBlock:{type:"text",text:...})。
func dshUpdateText(raw json.RawMessage) string {
	var u struct {
		Content struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &u) != nil {
		return ""
	}
	if u.Content.Type != "" && u.Content.Type != "text" {
		return ""
	}
	return u.Content.Text
}

// dshToolCallUpdate 是 tool_call/tool_call_update 体(只声明用得到的字段;
// 状态枚举按蓝本 pending/running/in_progress/finished 与 ACP
// pending/in_progress/completed/failed 的并集做归一,未知状态进诊断不猜)。
type dshToolCallUpdate struct {
	ToolCallID string          `json:"toolCallId"`
	Title      string          `json:"title"`
	Kind       string          `json:"kind"`
	Status     string          `json:"status"`
	RawInput   json.RawMessage `json:"rawInput"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// dshToolPhase 状态归一:pending→STARTED,进行中→UPDATE,终态→FINISHED。
// 第二个返回值报告 status 是否被认识(未知状态必须进诊断,不得瞎猜阶段)。
func dshToolPhase(status string) (pb.ToolCallUpdate_Phase, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending", "":
		return pb.ToolCallUpdate_STARTED, true
	case "running", "in_progress", "executing":
		return pb.ToolCallUpdate_UPDATE, true
	case "completed", "finished":
		return pb.ToolCallUpdate_FINISHED, true
	case "failed", "error":
		return pb.ToolCallUpdate_FINISHED, true
	default:
		return pb.ToolCallUpdate_UNSPECIFIED, false
	}
}

func (p *DshProvider) handleDshToolUpdate(kind string, raw json.RawMessage, sink Sink, st *dshTurnState, notices *nativeNoticeRecorder) {
	var u dshToolCallUpdate
	if json.Unmarshal(raw, &u) != nil {
		notices.recordAs(dshNoticeKind("session/update:"+kind), "dsh", "session/update:"+kind, raw, sink)
		return
	}
	phase, known := dshToolPhase(u.Status)
	if !known {
		// 未知状态:如实进诊断,不猜阶段(与未知变体同口径)
		notices.recordAs(dshNoticeKind("session/update:"+kind+":"+u.Status), "dsh", "session/update:"+kind+":"+u.Status, raw, sink)
		return
	}
	tu := &pb.ToolCallUpdate{Phase: phase}
	name := u.Kind
	if name == "" {
		name = "tool"
	}
	args := map[string]interface{}{"title": u.Title, "kind": u.Kind, "status": u.Status}
	if len(u.RawInput) > 0 {
		var input interface{}
		_ = json.Unmarshal(u.RawInput, &input)
		args["input"] = input
	}
	tu.Call = &pb.ToolCallBlock{Id: u.ToolCallID, Name: name, JsonArgs: mustJSON(args)}
	if phase == pb.ToolCallUpdate_FINISHED {
		var sb strings.Builder
		for _, c := range u.Content {
			if c.Type == "" || c.Type == "text" {
				sb.WriteString(c.Text)
			}
		}
		ok := strings.EqualFold(u.Status, "completed") || strings.EqualFold(u.Status, "finished")
		tu.Result = &pb.ToolResultBlock{CallId: u.ToolCallID, Ok: ok, Output: clip(sb.String(), 4000)}
	}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: tu}})
}

// dshUsageUpdate 防御性解析用量帧:识别常见 token 字段(实测形状待配 key 真机
// 补验 —— 探针受缺 key 阻塞;一个数字都认不出就不伪造 UsageUpdate,由调用方
// 走诊断透传)。
func dshUsageUpdate(raw json.RawMessage) *pb.UsageUpdate {
	var u struct {
		InputTokens  *float64 `json:"inputTokens"`
		OutputTokens *float64 `json:"outputTokens"`
		TotalTokens  *float64 `json:"totalTokens"`
		ContextUsage *float64 `json:"contextUsage"`
	}
	if json.Unmarshal(raw, &u) != nil {
		return nil
	}
	if u.InputTokens == nil && u.OutputTokens == nil && u.TotalTokens == nil && u.ContextUsage == nil {
		return nil
	}
	us := &pb.Usage{}
	if u.InputTokens != nil {
		us.InputTokens = int64(*u.InputTokens)
	}
	if u.OutputTokens != nil {
		us.OutputTokens = int64(*u.OutputTokens)
	}
	if u.TotalTokens != nil {
		us.InputTokens = int64(*u.TotalTokens) // total ≈ 当前窗口占用(codex last 同口径)
	}
	return &pb.UsageUpdate{Usage: us}
}

// dshUsageUpdateFromUsedSize 解 0.1.5-rc.3 实测的 usage_update 形状
// {"used":N,"size":M}:上下文窗口占用,InputTokens=used(≈codex last 口径:
// InputTokens 承载"当前窗口占用",ContextUsageCard 按此渲染)。
// used/size 任一缺失返回 nil —— 字段不全不猜。
func dshUsageUpdateFromUsedSize(raw json.RawMessage) *pb.UsageUpdate {
	var u struct {
		Used *int64 `json:"used"`
		Size *int64 `json:"size"`
	}
	if json.Unmarshal(raw, &u) != nil || u.Used == nil || u.Size == nil {
		return nil
	}
	return &pb.UsageUpdate{Usage: &pb.Usage{InputTokens: *u.Used}}
}

// ---- 审批桥(反向请求 → 手机 → outcome;对标 codex_requests.go 纪律)----

// handleDshRequest 反向请求分派,三条规则与 codex_requests.go 同款:
//  1. 不认识的方法回 JSON-RPC error,绝不伪造 decision(红线 2);
//  2. 审批卡按请求真实内容构造,不猜不编造;
//  3. 超时(2min)/断线/通道故障 → 拒绝路径,绝不伪造"同意"(红线 1)。
//
// 【已实证 2026-09-25】key 配齐后 dsh-p0 探针 8/8 通过:第 4 项实测
// session/request_permission 的请求/应答原始帧与 allow/reject 双路行为、
// 第 5 项实测 cancel 中断语义(stopReason=cancelled),形状与本实现一致;
// 证据 docs/verify/evidence/dsh-p0-probe.md。
func (p *DshProvider) handleDshRequest(rsp dshClient, id int64, method string, params json.RawMessage, ctx context.Context, sink Sink, approve ApprovalHook, waiting *atomic.Bool) {
	if method != "session/request_permission" {
		// 精确匹配分派:未登记的一律诚实回 error(codex 第一条规则)
		log.Printf("[dsh] %s → 回 error(未实现,不伪造 decision)", method)
		// 登记进诊断台账(规格 §9):回 error 只让对端知道"这次没做成",而"收到过一个
		// 不认识的请求"需要能事后查。thread_id 留空 —— 本函数签名里没有会话上下文,
		// 且它的调用点之一是握手路径(那里确实没有会话),拿别的 id 冒充会更糟。
		sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
			SourceAgent:    "dsh",
			SourceMethod:   method,
			NormalizedKind: runtime.KindUnknownRequest,
			RawPayload:     redactNativePayload(params),
			Note:           "已回协议错误 -32601(安全拒绝;未替用户做任何审批决定)",
		})
		_ = rsp.RespondError(id, -32601, "PocketAgent 未实现该反向请求: "+method)
		return
	}
	perm, err := dshapp.ParseRequestPermission(params)
	if err != nil {
		log.Printf("[dsh] %v → 回 error", err)
		_ = rsp.RespondError(id, -32602, err.Error())
		return
	}
	allowID, rejectID := perm.AllowOptionID(), perm.RejectOptionID()
	if allowID == "" || rejectID == "" {
		// 选项表认不出 allow/reject 两侧:显式报错,不猜不伪造
		log.Printf("[dsh] 审批选项表无法识别(allow=%q reject=%q)→ 回 error", allowID, rejectID)
		_ = rsp.RespondError(id, -32602, "dsh 审批选项表无法识别 allow/reject 侧,拒绝代答")
		return
	}
	if approve == nil {
		// 审批通道缺失(TestModel 诊断 RPC 等路径):基础设施问题不伪装成用户意志
		log.Printf("[dsh] 审批通道不可用(approve==nil),回 error")
		_ = rsp.RespondError(id, -32000, "审批通道不可用")
		return
	}

	waiting.Store(true)
	defer waiting.Store(false)
	// 【vet】call 为指针:pb 消息含锁,按值传 hook 会被 copylocks 拒绝。
	call := buildDshPermissionCall(perm)
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ApprovalRequest{ApprovalRequest: &pb.ApprovalRequest{
		RequestId: fmt.Sprintf("dsh-%d", id),
		Action:    call,
		Reason:    "dsh 请求授权(一次性 allow/reject,无\"始终允许\")",
	}}})

	askCtx, cancel := context.WithTimeout(ctx, dshApprovalTimeout)
	defer cancel()
	allow, _, askErr := approve(askCtx, call)
	switch {
	case askErr != nil:
		// 超时(2min)/断线/通道故障 → 拒绝路径(蓝本 §4.3:绝不伪造"同意")。
		// 拒绝是失败安全方向:最坏损失是"用户本想允许但没来得及点",而不是
		// "用户没答应却被当成答应了"。
		if errors.Is(askErr, context.DeadlineExceeded) {
			log.Printf("[dsh] 审批 %v 无应答,按拒绝处理(request=%s)", dshApprovalTimeout, call.GetId())
		} else {
			log.Printf("[dsh] 审批通道失败,按拒绝处理: %v", askErr)
		}
		_ = rsp.Respond(id, dshapp.PermissionOutcomeSelected(rejectID))
	case allow:
		_ = rsp.Respond(id, dshapp.PermissionOutcomeSelected(allowID))
	default:
		_ = rsp.Respond(id, dshapp.PermissionOutcomeSelected(rejectID))
	}
}

// buildDshPermissionCall 构造审批卡:动作名用 dsh 原生 title(缺失时用通用名),
// 参数原样透出,绝不编造具体命令。
func buildDshPermissionCall(perm *dshapp.RequestPermissionParams) *pb.ToolCallBlock {
	id, name := "dsh-permission", "dsh_permission"
	args := map[string]interface{}{"kind": perm.Kind}
	if perm.ToolCall != nil {
		if perm.ToolCall.ToolCallID != "" {
			id = perm.ToolCall.ToolCallID
		}
		if perm.ToolCall.Title != "" {
			name = perm.ToolCall.Title
		}
		if perm.ToolCall.Kind != "" {
			args["kind"] = perm.ToolCall.Kind
		}
		if len(perm.ToolCall.RawInput) > 0 {
			var input interface{}
			_ = json.Unmarshal(perm.ToolCall.RawInput, &input)
			args["input"] = input
		}
	}
	return &pb.ToolCallBlock{Id: id, Name: name, JsonArgs: mustJSON(args)}
}

// ---- 错误保真(ErrorInfoCarrier)----

// dshRunError 携带结构化 Error:终局 TurnFailed 帧保真(codexRunError 同款)。
type dshRunError struct {
	info    *pb.Error
	partial string
}

func (e *dshRunError) Error() string {
	if e == nil || e.info == nil {
		return "dsh: 未知错误"
	}
	return "dsh: " + e.info.Message
}

func (e *dshRunError) ErrorInfo() *pb.Error {
	if e == nil {
		return nil
	}
	return e.info
}

// dshRunErrorFrom 传输层错误 → 结构化错误(探针修正③ + 本次增强):缺 key 归一为
// MISSING_CREDENTIAL,且"人话在前、上游原文在后"——直接告诉用户该干什么,同时
// 不丢弃 dsh 原生报错(铁律:不认识的输入要原样透传,不能替换成"未知/错误")。
func (p *DshProvider) dshRunErrorFrom(err error) *dshRunError {
	info := &pb.Error{Retryable: false}
	if dshapp.IsMissingCredential(err) {
		info.Code = "MISSING_CREDENTIAL"
		// 人话在前:指向用户可操作的修复(创建 dsh-credentials.env 或给 daemon 设 env)。
		info.Message = "dsh 缺少 DeepSeek 凭据：无法调用模型。请在电脑上创建 " +
			filepath.Join(p.resolvedDataDir(), "dsh-credentials.env") +
			" 并写入 DEEPSEEK_API_KEY=你的密钥，或给 daemon 进程设置同名环境变量；改完重启 daemon 即可。"
		// 上游原文原样保留进 Details(保真,不加工):用户可据此向 dsh 社区贴原报错。
		if msg, ok := dshapp.RPCErrorMessage(err); ok {
			info.Details = msg
		}
	} else {
		info.Code = "DSH_ERROR"
		if msg, ok := dshapp.RPCErrorMessage(err); ok {
			info.Message = msg
		} else {
			info.Message = err.Error()
		}
	}
	return &dshRunError{info: info}
}

// ---- DeleteThread 收尾 ----

// CloseSession 远端删除:session/close(静默取消+排空+落盘+销毁)。
// 失败返回明确错误,由上层决定降级(zcode 同款口径)。
func (p *DshProvider) CloseSession(sessionID string) error {
	return p.withClient(func(ctx context.Context, c dshClient) error {
		return c.SessionClose(ctx, sessionID)
	})
}
