package agent

// zcode.go —— ZcodeProvider:把 PocketAgent 的 gRPC 协议桥接到 ZCode 桌面端的
// agent 内核(node zcode.cjs app-server --stdio,"ZCode Protocol")。
//
// 【讲解】事件对照表(手机上看到的 ← ZCode 发来的,0.16.5 探针实测,
// 证据见 docs/verify/evidence/zcode-p0-probe.md):
//
//	文字逐字蹦出      model.streaming + payload.kind="text_delta"
//	思考过程          model.streaming + payload.kind="reasoning_delta"
//	"⚙ Write"卡片    tool.updated + payload.kind="tool_call"(0.16.5)/
//	                  "scheduled"|"started"(本机 fork union,2026-10-07 审计)
//	卡片进度          tool.updated + payload.kind="progress"(预览/尾随输出)
//	卡片出结果        tool.updated + payload.kind="result"(result.success/content)
//	工具出错          tool.updated + payload.kind="tool_error"(0.16.5)/
//	                  "error"(fork) → FINISHED(ok=false)
//	审批弹卡          interaction/requestPermission(服务器反向请求)→ approve hook
//	用量              turn.completed payload.usage(input/output/cache/reasoning tokens)
//	一轮真正结束      turn.completed / turn.failed
//
// 【进程模型】与 codex app-server 相同:一个子进程承载多个会话(session),
// Thread ↔ zcode sessionId 经 threadMap(同子进程代内)+ Thread.ExternalSession.
// SessionID(落库,daemon 重启后重挂载,session/resume 实测通过)两级映射。
//
// 【运行中新输入】zcode 协议侧的 steer 队列(turn.steerQueued/steerDrained)只服务
// 桌面端内部通道:2026-09-19 实测(docs/verify/evidence/zcode-steer-probe.log,
// cmd/zcprobe -steer),对 running session 发 session/send 被服务器直接拒绝
// (-32010 "A prompt is already running for this session"),事件流无任何 steer
// 类事件,原轮次照常跑完 —— stdio 通道没有可用插话入口,故本 provider【不实现
// Steerer】;server 层按既有排队语义处理,当前轮次终局后输入作为新轮续跑,不丢。
//
// 新会话默认 mode:"build"；仅用户显式 NativeMode 才调用原生 setMode。
// setMode 会持久化该项目的权限偏好，不能宣称为仅会话或 OS sandbox。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// zcode 常量预算(与 codex 同款分级)。
const (
	// zcodeInitBudget 拉起子进程 + 存储就绪的默认上界(本机实测 <3s)。
	zcodeInitBudget = 15 * time.Second
	// zcodeStatusBudget 只读状态查询(模型列表等)的整段上界。
	zcodeStatusBudget = 20 * time.Second
	// zcodeModelsTTL 模型列表缓存:拉列表要建一个一次性会话,不能每次都来。
	zcodeModelsTTL = 10 * time.Minute
	// zcodeWatchdogSilence 轮次看门狗:连续这么久没有任何事件且不在等审批,
	// 判定任务未被接收/上游卡死(与 codex 的 150s 同口径)。
	zcodeWatchdogSilence = 150 * time.Second
	// zcodeUIBudget 审批等手机应答的上界(P2-6,对齐 pi 的 piUIBudget=120s):
	// 手机端停在审批卡但流不断时,无上界的等待会把整轮一直挂着;若桌面端已按
	// 自身超时判 deny,daemon 仍在等,之后的应答发的是过期 requestId。
	zcodeUIBudget = 120 * time.Second
	// zcodeToolClip 工具输出单帧限长(rune)。
	zcodeToolClip = 4000
	// zcodeSubagentToolClip 子代理委派工具结果的限长(rune)。
	// 【为什么比 zcodeToolClip 宽】情况 B:ZCode 不暴露子代理内部对话,一次
	// "Agent"/"Task" 工具调用就是用户能看到的"子代理"全部载体 —— 它的结果
	// 同时是子代理的"产出"(返回的文本/写出的文件路径)。普通工具(如 Write
	// 回"文件已创建")结果短,4000 够用;子代理结果可能带回整段方案正文,
	// 沿用 4000 会截断用户最想看的"产出"。放宽到 16000 仍是有界(绝不无限),
	// gRPC 单消息体量无压力。识别依据见 zcodeapp.ToolInputIsSubagent(纯结构)。
	zcodeSubagentToolClip = 16000
)

// ZcodeProvider 桥接 ZCode 桌面端 agent。
type ZcodeProvider struct {
	// turnMu 串行化轮次;lifeMu 保护子进程生命周期(两者独立,查模型不阻塞轮次)。
	turnMu     sync.Mutex
	lifeMu     sync.Mutex
	client     *zcodeapp.Client
	inited     bool
	turnActive atomic.Bool

	// 我们的 threadID → zcode sessionId + 子进程代号。真正的事实来源是
	// Thread.ExternalSession.SessionID(server 层轮次后落库);内存缓存只在
	// 同一子进程生命周期内有效,子进程重启后必须 session/resume(实证通过)。
	// 【P1-2】threadMap 配专属锁:写方 ensureSession 在轮次 goroutine(持
	// turnMu),读方 loadedZcodeSessionID 会被 SetGoal 在 server RPC goroutine
	// 调用 —— 两边没有共同锁,裸 map 并发读写会触发 Go runtime fatal
	// ("concurrent map read and map write"),整个 daemon 崩。读写全走 threadMu。
	threadMap map[string]threadRef
	threadMu  sync.Mutex
	gen       atomic.Int32

	// 模型列表缓存:available 快照来自一次性会话的 settings.model.available。
	// refByModel 记录 modelId → {providerId, defaultReasoningLevel},供 setModel
	// 使用(zcode 的 setModel 必须带 providerId,GLM 系还必须带 reasoningLevel)。
	mu         sync.Mutex
	models     []pb.ModelInfo
	modelsAt   time.Time
	refByModel map[string]zcodeModelRef

	// projMu/projByThread/slashByThread 每个线程的快照账本(session/create、
	// resume、subscribe 三路应答快照,recordProjection 同点记账):
	//   - projByThread:projection 段两数,窗口随 UsageUpdate.ModelContextWindow
	//     透出,手机端据此算上下文占用环(审计 S-1"上下文占用接入 UsageUpdate")。
	//     【探针实证】turn 结束后 projection.totalTokenCount == turn usage.totalTokens
	//     (zcode-ctrl-probe2.log:两处同为 19850),所以窗口 + 每轮 usage 就是真实占用。
	//   - slashByThread:快照 slashCommands[](转 pb),供 SlashCommandsFor 走纯
	//     内存通路回应 server 的斜杠命令面板,免去"每次起一次性子进程 resume"。
	projMu        sync.Mutex
	projByThread  map[string]zcodeProjection
	slashByThread map[string][]*pb.SlashCommand

	// goalMu/pendingGoals 线程还没挂载会话时的待下发常驻目标(与 codex_goal
	// 同款:SetGoal 时线程未挂载 → 暂存,Run 拿到 sessionId 后补发)。
	goalMu       sync.Mutex
	pendingGoals map[string]string

	// sessionSource 原生会话适配器(zcode_session.go)的客户端源:生产为 nil,
	// 走 withClientCtx 的真子进程;单测与契约套件注入假客户端,不拉真子进程
	// (与 pi 的 sessionFor 钩子同模式,014db16 先例)。
	sessionSource func(ctx context.Context) (zcodeSessionClient, error)

	// prefsMu 保护桌面偏好内存态(ApplyPrefs 与事件/轮次路径并发读写)。
	// 【红线】只存内存,绝不写用户桌面端配置。
	prefsMu sync.Mutex
	prefs   DesktopPrefs

	// TitleHandler 标题同步钩子:zcode 自动命名会话时回调(server 层落库后
	// 双端可见)。server.New 已接线(applyZcodeTitle 落库,server.go),与 codex
	// 同名同语义;此注释原写"P0 阶段尚未接线"已过时,更正以免误导后续排查。
	TitleHandler func(threadID, title string)

	// SubagentsHandler 原生子代理清单钩子(0.16.9 session/subagents):轮次终局
	// 拉到清单后回调,server 层据此登记/更新子代理存根线程(接线见
	// server/zcode_subagent_sync.go)。nil = 未接线,轮次后不拉(探针/测试路径)。
	SubagentsHandler func(threadID string, res zcodeapp.SubagentsResult)
}

// zcodeModelRef 记录一次模型选择所需的完整定位。
type zcodeModelRef struct {
	NativeID           string
	ProviderID         string
	DefaultReasoningLv string
	// SupportsImages 快照声明的图片输入能力(inputFormat.supportsImage,审计 S-1
	// 同源)。threadMap 命中(本轮无新鲜快照)时,图片门控用这一位兜底判定。
	SupportsImages bool
	Levels         []string
}

// NewZcodeProvider 构造。
func NewZcodeProvider() *ZcodeProvider {
	return &ZcodeProvider{
		threadMap:     map[string]threadRef{},
		refByModel:    map[string]zcodeModelRef{},
		projByThread:  map[string]zcodeProjection{},
		slashByThread: map[string][]*pb.SlashCommand{},
		pendingGoals:  map[string]string{},
	}
}

func (p *ZcodeProvider) ID() string          { return "zcode" }
func (p *ZcodeProvider) DisplayName() string { return "ZCode(桌面 agent)" }

// RegisterZcode 把 ZCode provider 显式注册进 providers 表(装了 node + zcode.cjs 才注册)。
//
// 【为什么是独立函数而不是塞进 DetectProviders】provider.go 正在被 Pi 改造(多
// agent 注册路径),本阶段红线是不碰它 —— 主模型合并时在 DetectProviders 末尾加一行:
//
//	RegisterZcode(m)
//
// 即可点亮,与 codex 的注册并列、互不影响。
func RegisterZcode(m map[string]Provider) {
	if _, _, err := zcodeapp.LocateZCode(); err == nil {
		m["zcode"] = NewZcodeProvider()
	}
}

// ---- 模型列表 ----

// Models 手机端"模型·强度"胶囊的列表。
//
// 【必须以 ZCode 实际返回为准,不能写死】模型表来自 session/create 应答快照的
// settings.model.available(runtime/capabilities 只回 {independentPlanState:true},
// 实证)。拉取要建一个一次性会话(工作区用系统临时目录,ZCode 只记录不作业),
// TTL 缓存兜住频率;失败时至少保住 "zcode/default"(跟随桌面当前模型)。
func (p *ZcodeProvider) Models() []pb.ModelInfo {
	p.mu.Lock()
	if len(p.models) > 0 && time.Since(p.modelsAt) < zcodeModelsTTL {
		out := p.models
		p.mu.Unlock()
		return out
	}
	p.mu.Unlock()

	out := []pb.ModelInfo{{
		Id: "default", Provider: "zcode", DisplayName: "ZCode(跟随桌面配置)",
		Available: true,
	}}
	fetched, refs := p.fetchModels()
	if len(fetched) > 0 {
		out = append(out, fetched...)
		p.mu.Lock()
		p.models = out
		p.modelsAt = time.Now()
		p.refByModel = refs
		p.mu.Unlock()
	}
	return out
}

// fetchModels 用一次性会话拉模型表。返回 (模型列表, modelId→引用映射)。
func (p *ZcodeProvider) fetchModels() ([]pb.ModelInfo, map[string]zcodeModelRef) {
	ctx, cancel := context.WithTimeout(context.Background(), zcodeStatusBudget)
	defer cancel()
	var out []pb.ModelInfo
	refs := map[string]zcodeModelRef{}
	err := p.withClientCtx(ctx, zcodeStatusBudget, func(ctx context.Context, c *zcodeapp.Client) error {
		raw, err := c.CallCtx(ctx, "session/create", map[string]interface{}{
			"workspace": zcodeWorkspace(os.TempDir()),
			"mode":      "build", // 红线:显式 build,绝不 yolo
		})
		if err != nil {
			return err
		}
		out, refs = parseZcodeModels(raw)
		snapshot, err := zcodeapp.ParseSessionSnapshot(raw)
		if err != nil {
			return err
		}
		if snapshot.SessionID == "" {
			return errors.New("ZCode 模型快照没有 sessionId")
		}
		closeCtx, closeCancel := context.WithTimeout(context.Background(), zcodeInitBudget)
		defer closeCancel()
		return c.CloseSession(closeCtx, snapshot.SessionID)
	})
	if err != nil {
		log.Printf("[zcode] 模型列表探测失败: %v", err)
		return nil, refs
	}
	return out, refs
}

// parseZcodeModels 从 session/create 快照解析模型表(纯函数,单测覆盖)。
// 解析统一走 zcodeapp.ParseSessionSnapshot(快照形状的单一事实来源,与 zcodeapp
// 包的 fixture 单测共用),本函数只做"快照 → pb.ModelInfo"映射:
//   - supportsImage 透出到 SupportsImages(审计 S-1;与 pi.go 的模型能力位同模式);
//   - ref/defaultLevel 记进引用映射,供 setModel 定位(GLM 系必带 reasoningLevel)。
func parseZcodeModels(snapshotRaw json.RawMessage) ([]pb.ModelInfo, map[string]zcodeModelRef) {
	snap, err := zcodeapp.ParseSessionSnapshot(snapshotRaw)
	if err != nil {
		return nil, map[string]zcodeModelRef{}
	}
	refs := map[string]zcodeModelRef{}
	var out []pb.ModelInfo
	seen := map[string]bool{}
	counts := map[string]int{}
	for _, m := range snap.Models {
		counts[m.ModelID]++
	}
	for _, m := range snap.Models {
		key := zcodeapp.ModelKey(m.ProviderID, m.ModelID)
		if m.ModelID == "" || seen[key] {
			continue
		}
		seen[key] = true
		if counts[m.ModelID] == 1 {
			key = m.ModelID
		}
		refs[key] = zcodeModelRef{
			NativeID:           m.ModelID,
			ProviderID:         m.ProviderID,
			DefaultReasoningLv: m.DefaultReasoningLevel,
			SupportsImages:     m.InputFormat.SupportsImage,
			Levels:             m.ReasoningLevels,
		}
		name := m.Label
		if name == "" {
			name = m.ModelID
		}
		if counts[m.ModelID] > 1 {
			name += " · " + m.ProviderLabel
		}
		out = append(out, pb.ModelInfo{
			Id:               "zcode/" + key,
			Provider:         "zcode",
			DisplayName:      name,
			ContextWindow:    int32(m.ContextWindow),
			SupportsImages:   m.InputFormat.SupportsImage,
			SupportsThinking: len(m.ReasoningLevels) > 0,
			AvailableEfforts: m.ReasoningLevels,
			Available:        true,
		})
	}
	return out, refs
}

// zcodeWorkspace 构造 workspace 对象。
// 【实测】schema Wi={workspacePath, workspaceKey}(strict),且 workspaceKey 必须
// 等于 workspaceIdentity?.trim() || workspacePath —— 不传 identity 时就是路径本身。
func zcodeWorkspace(path string) map[string]string {
	return map[string]string{"workspacePath": path, "workspaceKey": path}
}

// ---- 子进程生命周期 ----

var errNoZcodeClient = fmt.Errorf("ZCode 子进程未启动")

// ErrZcodeBusy 表示另一个调用正持有 zcode 生命周期锁(正在初始化/探测)。
//
// 【PA-REAL-0003 文案窜用修复】此前这里直接复用包级 ErrBusy(定义在 codex.go,
// 文案写死"codex 正忙"),zcode 自己忙却被标成"codex 正忙",把排障引向完全无关
// 的 provider(AUDIT §3.3:错误文案不得误导)。现在文案如实指明是 zcode 在忙;
// 同时以 Unwrap 挂在 ErrBusy 哨兵之下 —— errors.Is(err, ErrBusy) 仍为真,
// codex.modelsListErr(T-14,PA-REAL-0002)与 server 层依 ErrBusy 做的
// "可重试/正忙"分级归类链不回归。
var ErrZcodeBusy = &busySentinelError{msg: "zcode 正忙(另一个操作正在初始化/探测),请稍后重试"}

// busySentinelError 是"本 provider 正忙"哨兵的载体:Error() 只含自身如实文案
// (不拼底层哨兵文本,避免把"codex 正忙"又带回来),Unwrap 到包级 ErrBusy
// 维持既有 errors.Is 分级兼容。
type busySentinelError struct {
	msg string
}

func (e *busySentinelError) Error() string { return e.msg }
func (e *busySentinelError) Unwrap() error { return ErrBusy }

// zcodeLockWait TryLock 失败后的**有界排队**窗口。
//
// 【为什么要排队而不是立即忙】zcode 以前是 TryLock 失败即回"正忙",而用户路径
// (打开会话拉历史 / 拉斜杠命令 / 切模型)与"初始化·探测"撞锁的概率并不低 ——
// 实测 daemon.log 就有「拉取导入历史失败(降级为已有内容): zcode 正忙」,用户
// 看到的是**会话历史一片空白**(即"原生会话加载不出来")。dsh 早已用有界排队
// 解决同类问题(internal/agent/dsh.go 的 lockLife),这里把同一模式搬到 zcode:
// 先在窗口内等一等,拿到就正常做完;只有持有方真卡死(窗口耗尽)才如实回忙哨兵。
// 包级变量仅为测试注入短值;生产语义 = 15 秒,请勿改动。
var zcodeLockWait = 15 * time.Second

// lockLife 有界排队获取生命周期锁:先 TryLock(无争用零开销),失败后在 wait
// 窗口内短间隔重试 —— 排队而非立即拒绝,把"低频用户路径刚好撞上初始化"的瞬时
// 争用吸收成先后等待;窗口耗尽(持有方真卡死)才返回 false → 调用方回忙哨兵,
// 保留 TryLock 模式"一个卡死的操作不拖住全线"的底线。
//
// wait <= 0 时退化为纯 TryLock 语义 —— 既有单测靠它保住"立即忙"的断言,
// 不必为了一个生产优化去改测试的语义。
func (p *ZcodeProvider) lockLife(wait time.Duration) bool {
	return lockLifeMu(&p.lifeMu, wait)
}

// withClientCtx 在保证子进程活着的前提下执行 fn;失败自动重启一次。
//
// 【TryLock/忙哨兵模式,与 codex.withClientCtx 同款且三点缺一不可】
//  1. TryLock 替代 Lock:拿不到锁立即返回忙哨兵(ErrZcodeBusy,文案如实指明
//     zcode 自己忙,不窜用 codex 的 ErrBusy 文案),一个卡死的操作不拖住全线;
//  2. 调用方没给截止时间时套默认上界(zcodeInitBudget),无论如何有界;
//  3. 全程同步,绝不"超时后 go 继续等" —— 那会留下还持着 lifeMu 的僵尸 goroutine。
//
// zcode 没有 codex 那样的 initialize 握手:进程起来、存储初始化以 startup/storageState
// 通知自行推进到 ready,请求即可直接发(实证:启动后立刻 runtime/capabilities 正常应答)。
func (p *ZcodeProvider) withClientCtx(ctx context.Context, budget time.Duration, fn func(context.Context, *zcodeapp.Client) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	// 排队窗口 = min(zcodeLockWait, 调用方自己的剩余截止时间):绝不超过调用方
	// 本来就愿意等待的时长,ctx 传导语义不受影响。
	wait := zcodeLockWait
	if d, hasDeadline := ctx.Deadline(); hasDeadline {
		if rem := time.Until(d); rem < wait {
			wait = rem
		}
	}
	if !p.lockLife(wait) {
		return ErrZcodeBusy
	}
	defer p.lifeMu.Unlock()

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		if budget <= 0 {
			budget = zcodeInitBudget
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, budget)
		defer cancel()
	}

	try := func() error {
		if p.client == nil {
			return errNoZcodeClient
		}
		return fn(ctx, p.client)
	}
	err := try()
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	var rpcErr *zcodeapp.RPCError
	var accountErr *zcodeapp.AccountSyncError
	if errors.As(err, &rpcErr) || errors.As(err, &accountErr) {
		return err // Native/configuration errors do not mean the worker crashed.
	}
	// 正在执行轮次时,失败不能重启共享子进程:会把跑着的会话强行切断,
	// 手机只能收到半截流(与 codex 同款守卫)。
	if err != nil && err != errNoZcodeClient && p.turnActive.Load() {
		return err
	}
	if err == nil || err == errNoZcodeClient {
		if err == errNoZcodeClient {
			c, serr := zcodeapp.Start(zcodeapp.StartOptions{})
			if serr != nil {
				return serr
			}
			p.client, p.inited = c, true
			p.attachUnknownResponseHandler(c)
			p.attachGlobalHandler(c)
			p.gen.Add(1)
			log.Printf("[zcode] app-server 子进程已启动(gen=%d)", p.gen.Load())
			err = try()
			// 【启动闪退即重试一次(2026-10-01)】真机日志实证:偶发"拉起 ~1s 即
			// exit 1 且零 stderr"(手动同参复现不出来,疑与 daemon 进程环境瞬时
			// 状态相关),且一次失败会把这个 provider 在能力面/选择面整个判死。
			// 死亡必在启动早期(errNoZcodeClient 路径 = 刚 Start 完),立即换一个
			// 新进程重试代价 ~1s,能把偶发闪退从"provider 不可用"降为"慢一秒"。
			// 重试仍死 → 原样上抛(真配置错误不该被重试掩盖)。
			if err != nil && p.client != nil && !p.client.Alive() {
				log.Printf("[zcode] 启动闪退(零 stderr 偶发),重试一次: err=%v", err)
				p.client.Close()
				p.client, p.inited = nil, false
				c2, serr2 := zcodeapp.Start(zcodeapp.StartOptions{})
				if serr2 != nil {
					return serr2
				}
				p.client, p.inited = c2, true
				p.attachUnknownResponseHandler(c2)
				p.attachGlobalHandler(c2)
				p.gen.Add(1)
				log.Printf("[zcode] app-server 子进程已启动(重试 gen=%d)", p.gen.Load())
				err = try()
			}
		}
		return err
	}
	// 子进程可能挂了:杀掉重来一次
	if p.client != nil {
		p.client.Close()
	}
	p.client, p.inited = nil, false
	c, serr := zcodeapp.Start(zcodeapp.StartOptions{})
	if serr != nil {
		return serr
	}
	p.client, p.inited = c, true
	p.attachUnknownResponseHandler(c)
	p.attachGlobalHandler(c)
	p.gen.Add(1)
	log.Printf("[zcode] app-server 子进程已重启(gen=%d),所有会话缓存作废", p.gen.Load())
	return try()
}

func (p *ZcodeProvider) attachUnknownResponseHandler(c *zcodeapp.Client) {
	c.SetUnknownResponseHandler(func(rawID json.RawMessage, raw json.RawMessage) {
		recordUnknownResponse("zcode", string(rawID), raw)
	})
}

func (p *ZcodeProvider) attachGlobalHandler(c *zcodeapp.Client) {
	c.SetGlobalHandler(func(method string, params json.RawMessage) {
		ledgerMethod, kind := zcodeLedgerMethod(method, params)
		recordProviderEvent("zcode", ledgerMethod, "", kind, params)
	})
}

// zcodeLedgerMethod 计算一条 zcode 通知进诊断台账时的 method 名与分类。
//
// 【session/event 必须拆壳再分类】session/event 通知的 params 外壳是
// {sessionId,type,payload,…}(zcodeEventEnvelope),分类表(zcodeNoticeKind)
// 认的键是**内部 type**。全局钩子直接拿外层 method="session/event" 记账的话,
// 分类器永远查空 —— 轮次期间 model.streaming/tool.updated/turn.completed
// 每条都被灌成 UNKNOWN,把真未知挤出滚动窗。
// 这里在记账前把 method 换成 "session/event:<内部type>"(与 dsh 的
// "session/update:<kind>" 同款拼法):SourceMethod 同时保留原始方法名与内部
// type,既诚实又可检索。壳解析失败/type 为空:按原样 "session/event" 记
// UNKNOWN —— 拆不开的壳最该让人看见,不替它编一个已知身份。
func zcodeLedgerMethod(method string, params json.RawMessage) (string, string) {
	if method != "session/event" {
		return method, zcodeNoticeKind(method)
	}
	var ev zcodeEventEnvelope
	if json.Unmarshal(params, &ev) != nil || ev.Type == "" {
		return method, runtime.KindUnknownNotification
	}
	return "session/event:" + ev.Type, zcodeNoticeKind(ev.Type)
}

// withClient 无 ctx 的便捷入口。
func (p *ZcodeProvider) withClient(fn func(*zcodeapp.Client) error) error {
	return p.withClientCtx(context.Background(), zcodeInitBudget, func(ctx context.Context, c *zcodeapp.Client) error {
		return fn(c)
	})
}

// ---- 会话准备 ----

// zcodeProjection 快照 projection 段里"手机占用环要用"的两个数。
type zcodeProjection struct {
	ContextWindow   int64
	TotalTokenCount int64
}

// zcodeSlashCommands 快照 slashCommands[] → pb 命令列表(纯函数,单测覆盖)。
// 映射与 server/slash_commands.go 的 fetchViaZcode 完全同款:name/description/
// inputHint 三字段逐一透传。入参为 nil(快照没带该字段)时返回 nil,由调用方
// 决定"跳过记账"—— 防止形状缺失的快照把已记账的命令抹掉。
func zcodeSlashCommands(cmds []zcodeapp.SlashCommand) []*pb.SlashCommand {
	if cmds == nil {
		return nil
	}
	out := make([]*pb.SlashCommand, 0, len(cmds))
	for _, sc := range cmds {
		out = append(out, &pb.SlashCommand{
			Name:        sc.Name,
			Description: sc.Description,
			InputHint:   sc.InputHint,
		})
	}
	return out
}

// recordProjection 解析一份快照应答,把 projection 与斜杠命令一起记到线程名下。
// create/resume/subscribe 的应答都可能喂进来;解析失败或没有 projection 段
// (字段缺失是常态)静默跳过,绝不让快照形状异常打断会话准备。
// 【斜杠命令记账】快照带 slashCommands 字段(非 nil,显式空数组也算)才覆盖
// 账本 —— subscribe 应答的快照段可能不带命令列表,不能把 create/resume 记下的
// 好数据冲掉;记了账,server 的面板查询就走纯内存(SlashCommandsFor)。
func (p *ZcodeProvider) recordProjection(threadID string, raw json.RawMessage) {
	snap, err := zcodeapp.ParseSessionSnapshot(raw)
	if err != nil {
		return
	}
	_, refs := parseZcodeModels(raw)
	p.mu.Lock()
	if len(refs) > 0 {
		p.refByModel = refs
	}
	p.mu.Unlock()
	p.projMu.Lock()
	if snap.Projection.ContextWindow > 0 {
		p.projByThread[threadID] = zcodeProjection{
			ContextWindow:   snap.Projection.ContextWindow,
			TotalTokenCount: snap.Projection.TotalTokenCount,
		}
	}
	if cmds := zcodeSlashCommands(snap.SlashCommands); cmds != nil {
		p.slashByThread[threadID] = cmds
	}
	p.projMu.Unlock()
}

// SlashCommandsFor 实现 server 侧 zcodeSlashSource 接口(internal/server/
// slash_commands.go 类型断言探测):返回线程账本里最近一份快照的斜杠命令,
// 纯内存、零进程开销。未记账(daemon 刚启动/线程还没跑过轮次/快照从没带过
// 命令字段)返回 nil,server 自然回退到"一次性子进程 session/resume"兜底路径,
// 行为与切换前等价。
func (p *ZcodeProvider) SlashCommandsFor(threadID string) []*pb.SlashCommand {
	p.projMu.Lock()
	defer p.projMu.Unlock()
	return p.slashByThread[threadID]
}

// contextWindowFor 取线程已知的上下文窗口;未知返回 0(UsageUpdate 里 0=未知,
// 与 codex tokenUsage.modelContextWindow 同口径)。
func (p *ZcodeProvider) contextWindowFor(threadID string) int64 {
	p.projMu.Lock()
	defer p.projMu.Unlock()
	return p.projByThread[threadID].ContextWindow
}

// ensureSession 找到/创建 zcode 会话,并订阅事件流。
//
// 优先级:同子进程代内的内存缓存 → Thread.ExternalSession.SessionID 重挂载
// (session/resume,daemon 重启后的常规路径)→ 新建 session/create。
// 返回 (sessionId, 是否新建, 本次应答快照原文, 错误)。快照原文供图片输入的
// 能力门控读取 inputFormat.supportsImage(含 current 模型定位);threadMap 命中
// (无新鲜快照)时返回 nil,门控退回 refByModel 缓存位判定。
//
// 【P1-7 bindSession 上收】协议路径(重挂载/新建)改经原生会话适配器
// (zcodeSession.resumeForRun/openForRun,内部复用 resumeZcodeSession/
// createZcodeSession 同一实现);本函数保留的是 Run 链自己的决策:同代短路、
// "resume 失败改开新会话"的降级、账号同步错误直通、subscribe(轮次内状态机
// 的事,设计稿 §4-P3)。"过渡期双路径"就此退役,Run 链与契约面
// (openSharedRuntime 注入)走同一个适配器入口。
func (p *ZcodeProvider) ensureSession(ctx context.Context, req TurnRequest) (string, bool, json.RawMessage, error) {
	gen := int(p.gen.Load())
	if ref, hit := p.zcodeThreadRefFor(req.Thread.ID); hit && ref.gen == gen {
		return ref.id, false, nil, nil
	}
	ns, err := p.NativeSessionFor(req.Thread)
	if err != nil {
		return "", false, nil, err
	}
	zs, ok := ns.(*zcodeSession)
	if !ok {
		return "", false, nil, fmt.Errorf("zcode: 原生会话适配器类型异常: %T", ns)
	}
	if sid := req.Thread.ExternalSession.SessionID; sid != "" {
		raw, rerr := zs.resumeForRun(ctx, req.Thread)
		if rerr != nil {
			var accountErr *zcodeapp.AccountSyncError
			if errors.As(rerr, &accountErr) {
				return "", false, nil, rerr
			}
			log.Printf("[zcode] session/resume %s 失败(改开新会话): %v", sid, rerr)
		} else {
			return sid, false, raw, zs.subscribeForRun(ctx, req.Thread.ID, sid)
		}
	}
	sid, created, raw, err := zs.openForRun(ctx, req.Thread, req.Workspace)
	if err != nil {
		return "", false, nil, err
	}
	return sid, created, raw, zs.subscribeForRun(ctx, req.Thread.ID, sid)
}

// applyModel 把 Thread.ModelID("zcode/<modelId>")映射成 session/setModel。
// 【失败语义】结构上不可解析(模型不在快照/不属于 zcode/resume 失败)与
// setModel RPC 失败(登录/配额/上游拒收)都返回错误,由 Run 交整轮失败,
// 错误文案直接指导用户动作;档位与模型自声明 levels 的不一致走容错,不报错
// (见下方 T-02 注释)。
// 【实测】setModel 的 model={providerId,modelId,options?{reasoningLevel}};
// GLM 系不带 reasoningLevel 会报 -32603,所以引用缓存里存了 defaultLevel。
// 【effort 取值(P2-8 快照语义)】threadEffort = Thread.Effort(会话胶囊显式
// 指定)第一级;desktopEffort 由调用方经 desktopEffortFor 传入(线程快照 >
// 全局桌面默认)第二级;两者都空才用模型自带 defaultLevel,全空才省略
// options(空=不覆盖),见 zcodeReasoningLevel。
func (p *ZcodeProvider) applyModel(ctx context.Context, sid, modelID, threadEffort, desktopEffort string) error {
	spec := strings.TrimPrefix(modelID, "zcode/")
	if spec == "" || spec == "default" {
		if threadEffort == "" && desktopEffort == "" {
			return nil
		}
		raw, err := p.client.CallCtx(ctx, "session/resume", map[string]string{"sessionId": sid})
		if err != nil {
			return fmt.Errorf("读取 ZCode 当前模型失败: %w", err)
		}
		var snapshot struct {
			Settings struct {
				Model struct {
					Current struct {
						ModelID string `json:"modelId"`
					} `json:"current"`
				} `json:"model"`
			} `json:"settings"`
		}
		if json.Unmarshal(raw, &snapshot) != nil || snapshot.Settings.Model.Current.ModelID == "" {
			return fmt.Errorf("ZCode 未返回当前模型，无法设置思考强度")
		}
		spec = snapshot.Settings.Model.Current.ModelID
		_, refs := parseZcodeModels(raw)
		p.mu.Lock()
		for id, ref := range refs {
			p.refByModel[id] = ref
		}
		p.mu.Unlock()
	} else if spec == modelID {
		return fmt.Errorf("模型不属于 ZCode: %s", modelID)
	}
	p.mu.Lock()
	ref, ok := p.refByModel[spec]
	p.mu.Unlock()
	if !ok {
		// Metadata can change in the desktop app between turns. Refresh the actual
		// session before declaring an opaque/custom model unresolvable.
		raw, err := p.client.CallCtx(ctx, "session/resume", map[string]string{"sessionId": sid})
		if err != nil {
			return fmt.Errorf("ZCode 模型资料暂不可用（不是模型不支持）: %w", err)
		}
		_, refs := parseZcodeModels(raw)
		p.mu.Lock()
		for id, r := range refs {
			p.refByModel[id] = r
		}
		ref, ok = p.refByModel[spec]
		p.mu.Unlock()
		if !ok {
			return fmt.Errorf("ZCode 最新快照未报告模型 %s 的原生引用，请刷新模型列表；不会替换成其他模型", spec)
		}
	}
	level := zcodeReasoningLevel(threadEffort, desktopEffort, ref.DefaultReasoningLv)
	// 【T-02 容错,替代原"档位不在 levels 即整轮失败"的硬校验】快照可能自相矛盾:
	// deepseek-v4-flash 实测 default_level="max" 但 levels=[low,medium,high,xhigh,none],
	// 上游也确实拒收 max("ReasoningEffort invalid … low, medium, high, xhigh, none",
	// 证据 docs/verify/evidence/2026-09-19-device-build/02-routing-models/
	// probe-zcmodelprobe.txt)。按档位来源分两级,不新增任何写死枚举:
	//   - 档位来自模型自带 defaultLevel(用户线程/桌面都没选)且不在其自声明
	//     levels 里 → 省略 options(协议语义"不覆盖"),沿用桌面端对当前模型的
	//     默认档位 —— 不伪造档位、不每轮白打一次注定被拒的 setModel;GLM 系
	//     (defaultLevel 均在 levels 内)照旧显式下发,不受影响。
	//   - 用户显式选择(线程 effort / 桌面默认)不在 levels 里 → 原样透传,
	//     provider 是唯一裁决者,拒绝时由 setModel 失败分支把可用档位报给用户。
	if threadEffort == "" && desktopEffort == "" && level != "" && len(ref.Levels) > 0 && !slices.Contains(ref.Levels, level) {
		log.Printf("[zcode] 模型 %s 自带 defaultLevel %q 不在其声明档位 %v 内,省略 reasoningLevel(沿用桌面端当前档位)", spec, level, ref.Levels)
		level = ""
	}
	_, err := p.client.CallCtx(ctx, "session/setModel", map[string]interface{}{"sessionId": sid, "model": zcodeModelSelection(ref.ProviderID, firstNonEmpty(ref.NativeID, spec), level)})
	if err != nil {
		return fmt.Errorf("ZCode 模型/思考强度切换失败: %w", err)
	}
	return nil
}

// ---- 控制面:压缩 / 常驻目标 / 删除 ----
//
// 【地面真值】2026-09-19 控制面探针(cmd/zcprobe -ctrl,证据
// docs/verify/evidence/zcode-ctrl-probe.log 与 -probe2.log):
//   - session/compact {sessionId} → {"response":…,"snapshot":{…}}(全快照);
//     轮次进行中报 -32010 "Cannot compact while a prompt is running";
//   - session/goal 参数是 {sessionId, action, objective?},action ∈
//     "show"|"set"|"replace"|"pause"|"resume"|"clear"(zod -32602 枚举实测);
//     objective 字段名探针实证(set 成功且回显 "Objective: …")。注意 goal 的
//     set/clear 会在桌面端驱动一轮目标确认(prompt),期间其它 goal 操作与
//     compact 都被 -32010 拒 —— 所以目标补发必须放在轮次终局之后,不能抢在
//     session/send 之前;
//   - session/close {sessionId} → {"closed":true};close 后已不在 session/list,
//     resume 报 -32004 "Session not found" —— 语义=真删除,对齐手机"删会话=
//     双端一致"(审计 §7-5"三家远端删除")。

// CompactSession 手动压缩入口(对齐 codex CompactThread → thread/compact/start):
// sessionID 是 zcode 侧会话 id(server 层从 Thread.ExternalSession.SessionID 取)。
// 压缩在桌面端异步执行,应答里的 response/snapshot 我们不消费;会话在跑轮次时
// zcode 会以 -32010 拒绝,错误原样返回由 server 层转 gRPC 错误码。
func (p *ZcodeProvider) CompactSession(sessionID string) error {
	return p.withClient(func(c *zcodeapp.Client) error {
		if _, err := c.Call("session/compact", map[string]interface{}{"sessionId": sessionID}); err != nil {
			return fmt.Errorf("session/compact: %w", err)
		}
		return nil
	})
}

// CloseSession 远端删除:手机删会话时同步删掉桌面端会话(见上节探针实证的
// 真删除语义)。失败返回明确错误,由 server 层记日志但不让本地删除失败
// (会话可能已被桌面端删过,与 codex rollout 清理同款降级)。
func (p *ZcodeProvider) CloseSession(sessionID string) error {
	return p.withControlClient(context.Background(), func(ctx context.Context, c *zcodeapp.Client) error {
		return c.CloseSession(ctx, sessionID)
	})
}

// setZcodeThreadRef / zcodeThreadRefFor threadMap 的唯一读写入口(P1-2:全走
// threadMu。ensureSession 的命中查询与记账、loadedZcodeSessionID 的读都必须
// 经过这两个助手,不允许任何路径直接摸 p.threadMap)。
func (p *ZcodeProvider) setZcodeThreadRef(threadID string, ref threadRef) {
	p.threadMu.Lock()
	defer p.threadMu.Unlock()
	p.threadMap[threadID] = ref
}

func (p *ZcodeProvider) zcodeThreadRefFor(threadID string) (threadRef, bool) {
	p.threadMu.Lock()
	defer p.threadMu.Unlock()
	ref, hit := p.threadMap[threadID]
	return ref, hit
}

// loadedZcodeSessionID 取该线程在当前子进程里已挂载的 zcode sessionId(带 gen
// 校验,与 codex 的 loadedCodexThreadID 同款:子进程重启后旧 id 无效);没有则空。
func (p *ZcodeProvider) loadedZcodeSessionID(threadID string) string {
	gen := int(p.gen.Load())
	if ref, hit := p.zcodeThreadRefFor(threadID); hit && ref.gen == gen {
		return ref.id
	}
	return ""
}

// zcodeGoalParams 构造 session/goal 的参数(纯函数,单测覆盖):
// 空目标 = action:"clear";非空 = action:"set" + objective(字段名探针实证)。
func zcodeGoalParams(sessionID, goal string) map[string]interface{} {
	if strings.TrimSpace(goal) == "" {
		return map[string]interface{}{"sessionId": sessionID, "action": "clear"}
	}
	return map[string]interface{}{"sessionId": sessionID, "action": "set", "objective": goal}
}

// pushGoal 真正调用 session/goal。set 失败时退 replace 再试一次(目标已存在时
// zcode 的语义分叉;slash 提示 "/goal [pause|resume|clear|replace <objective>|…]"
// 实证 replace 存在,探针未走通 replace 本体——当时撞上 -32010 busy)。
// 【P2-3】client 由调用方经 withClient* 传入:不再裸读 p.client(与 RestartGuard
// 对齐 —— 轮次未激活时 HistoryFor 等路径可能正在"杀旧起新",窗口内 p.client
// 为 nil,裸读会空指针 panic)。
func (p *ZcodeProvider) pushGoal(ctx context.Context, c *zcodeapp.Client, sessionID, goal string) error {
	params := zcodeGoalParams(sessionID, goal)
	raw, err := c.CallCtx(ctx, "session/goal", params)
	if err != nil && strings.TrimSpace(goal) != "" {
		params["action"] = "replace"
		if raw2, err2 := c.CallCtx(ctx, "session/goal", params); err2 == nil {
			raw, err = raw2, nil
		}
	}
	if err != nil {
		log.Printf("[zcode] goal 下发失败 session=%s: %v", sessionID, err)
		return err
	}
	log.Printf("[zcode] goal 已下发 session=%s goal=%.80q resp=%.160s", sessionID, goal, string(raw))
	return nil
}

// stashGoal 记录/清除待下发目标(空串=清除,与 codex stashGoal 同语义)。
func (p *ZcodeProvider) stashGoal(threadID, goal string) {
	p.goalMu.Lock()
	defer p.goalMu.Unlock()
	p.pendingGoals[threadID] = goal
}

// takePendingGoal 取出并清除某线程的待下发目标。
func (p *ZcodeProvider) takePendingGoal(threadID string) (string, bool) {
	p.goalMu.Lock()
	defer p.goalMu.Unlock()
	goal, ok := p.pendingGoals[threadID]
	if ok {
		delete(p.pendingGoals, threadID)
	}
	return goal, ok
}

// SetGoal 把常驻目标下发到 zcode。threadID 是我们自己的会话 id;goal 为空串
// 表示清除目标。下发不了(线程未挂载会话/子进程起不来/正在跑轮次被 -32010 拒)
// 不算失败:暂存起来等 Run 终局补发,调用方照常落库即可(codex_goal 同款纪律;
// 清除失败时暂存为空 = 不重试,桌面端目标保持原状,用户可再清一次)。
// 【P2-3】pushGoal 一并收进 withClient:整段持 lifeMu,杜绝与"杀旧起新"窗口的
// 竞态;goal 下发失败在 fn 内消化(回 nil)—— goal 被 -32010 拒是桌面端忙,
// 不是进程死了,不能让 withClient 把它当成"子进程挂了"去重启。
func (p *ZcodeProvider) SetGoal(threadID, goal string) error {
	var (
		mounted bool
		pushErr error
	)
	err := p.withClient(func(c *zcodeapp.Client) error {
		sid := p.loadedZcodeSessionID(threadID)
		if sid == "" {
			return nil // 线程还没挂载会话:暂存等 Run 补发(下方统一处理)
		}
		mounted = true
		pushErr = p.pushGoal(context.Background(), c, sid, goal)
		return nil
	})
	if err == nil && mounted && pushErr == nil {
		return nil
	}
	p.stashGoal(threadID, goal)
	if err != nil {
		return err
	}
	return pushErr
}

// applyPendingGoal 在 Run 正常终局后补发暂存目标。带短超时,失败放回暂存
// (下一轮再试)。【为什么不能在 send 之前发】goal 的 set/clear 会在桌面端
// 驱动一轮目标确认 prompt(探针实证),期间 session/send 会被 -32010
// "A prompt is already running" 拒掉,把用户的正式输入打死。
// 【P2-3】走 withClientCtx(与 SetGoal 同款:push 失败在 fn 内消化,不触发
// withClient 的"杀旧起新"重启逻辑 —— 桌面端忙不是进程死了)。
func (p *ZcodeProvider) applyPendingGoal(threadID, sessionID string) {
	goal, ok := p.takePendingGoal(threadID)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var pushErr error
	err := p.withClientCtx(ctx, 30*time.Second, func(fctx context.Context, c *zcodeapp.Client) error {
		pushErr = p.pushGoal(fctx, c, sessionID, goal)
		return nil
	})
	if err != nil || pushErr != nil {
		p.stashGoal(threadID, goal)
		return
	}
	log.Printf("[zcode] 补发暂存目标 thread=%s", threadID)
}

// ---- 桌面偏好(PrefsAware)----

// ApplyPrefs 实现 agent.PrefsAware:接收桌面设置页七项偏好的完整快照。
//
// 【消费哪些】HideReasoning → 事件路径过滤 ThinkingDelta(emitThinking);
// Effort → setModel 的 options.reasoningLevel 桌面默认值(zcodeReasoningLevel)。
// NativeMode → 原生 session/setMode（仅显式值；空值跟随原生）。
// 【不映射哪些、为什么】Codex 审批策略/沙箱不等价于 ZCode permission mode；
// 这两项仅对 codex 生效;Verbosity/
// ReasoningSummary/DisablePasteBurst 协议同样无对应面。快照照单全收(完整覆盖),
// 与 codex 的 ApplyPrefs 同纪律。
func (p *ZcodeProvider) ApplyPrefs(pref DesktopPrefs) {
	applyPrefsSnapshot(&p.prefsMu, &p.prefs, pref)
}

// hideReasoning 读"隐藏推理过程"开关。
func (p *ZcodeProvider) hideReasoning() bool {
	return hideReasoningFrom(&p.prefsMu, &p.prefs)
}

// hideReasoningFor 是 hideReasoning 的线程感知版(P2-8,与 codex 的
// HideReasoningFor 同语义)。
//
// 【当前口径:全局优先(2026-10-09 注释与实现对齐)】快照字段
// TurnSettings.HideReasoning 虽然随快照落库,但**不参与判定** —— 无论有无
// 快照,一律以当前全局开关为准。这是有测试钉住的既定行为
// (TestHideReasoningIsGlobalAcrossSnapshots,本文件族 codex_snapshot_test.go;
// codex/pi/zcode 三家同款实现、同款全局优先 —— "切换快照语义"必须三家一起动,
// 此处不单边改道)。
func (p *ZcodeProvider) hideReasoningFor(t *store.Thread) bool {
	return p.hideReasoning()
}

// desktopEffort 读桌面默认思考强度(空=未设置,不覆盖模型自带 defaultLevel)。
func (p *ZcodeProvider) desktopEffort() string {
	p.prefsMu.Lock()
	defer p.prefsMu.Unlock()
	return p.prefs.Effort
}

// desktopEffortFor 是 desktopEffort 的线程感知版(P2-8,与 codex 的三级优先级
// 对齐):线程设置快照(SettingsSnapshot,新建对话那一刻的值)> 全局偏好
// (旧线程兜底)。线程显式 Effort 的第一级由调用方(zcodeReasoningLevel 的
// threadEffort 参数)处理;快照非 nil 时快照里的空串就是"当时未设置",保持
// 原值生效、不中途改抓新全局值。
func (p *ZcodeProvider) desktopEffortFor(t *store.Thread) string {
	if t != nil && t.SettingsSnapshot != nil {
		return t.SettingsSnapshot.Effort
	}
	return p.desktopEffort()
}

// emitThinking 在"隐藏推理"关闭时把思考增量推给 sink(与 codex.go/pi.go 同款
// daemon 侧强制过滤;ZCode 协议没有原生开关)。抽成方法便于单测钉死开关行为。
func (p *ZcodeProvider) emitThinking(delta string, sink Sink) {
	emitThinkingDelta(p.hideReasoning(), delta, sink)
}

// emitThinkingFor 是 emitThinking 的线程感知版(P2-8):按 hideReasoningFor 的
// 快照语义判定(有快照按快照,旧线程回全局),Run 的事件泵走这里;
// emitThinking 保留给无线程上下文的调用与既有单测。
func (p *ZcodeProvider) emitThinkingFor(t *store.Thread, delta string, sink Sink) {
	emitThinkingDelta(p.hideReasoningFor(t), delta, sink)
}

// zcodeReasoningLevel 计算 setModel 应携带的 reasoningLevel(纯函数,单测覆盖):
// 线程显式 effort > 桌面默认 effort > 模型引用自带 defaultLevel。
// 【为什么必须带值】GLM 系 setModel 不带 reasoningLevel 会报 -32603(实证),
// 所以三者都空时保留 defaultLevel,defaultLevel 也为空才省略 options。
// 用户 effort 值原样透传,provider 不认由 applyModel 的 setModel 失败分支
// 上抛报错;模型自带 defaultLevel 与其自声明 levels 自相矛盾时的容错见
// applyModel(T-02:省略 options,不整轮失败)。
func zcodeReasoningLevel(threadEffort, desktopEffort, defaultLevel string) string {
	if threadEffort != "" {
		return threadEffort
	}
	if desktopEffort != "" {
		return desktopEffort
	}
	return defaultLevel
}

// zcodeModelSelection 构造 setModel 的 model 选择对象(纯函数,单测覆盖):
// level 为空时省略 options(空=不覆盖),否则带 options.reasoningLevel。
func zcodeModelSelection(providerID, modelID, level string) map[string]interface{} {
	sel := map[string]interface{}{"providerId": providerID, "modelId": modelID}
	if level != "" {
		sel["options"] = map[string]string{"reasoningLevel": level}
	}
	return sel
}

// ---- 事件 → pb 映射(纯函数,单测覆盖)----

// zcodeEventEnvelope session/event 通知参数外壳。
type zcodeEventEnvelope struct {
	SessionID    string          `json:"sessionId"`
	TurnID       string          `json:"turnId"`
	EventID      string          `json:"eventId"`
	Seq          int64           `json:"seq"`
	DeliveryKind string          `json:"deliveryKind"`
	Type         string          `json:"type"`
	Payload      json.RawMessage `json:"payload"`
}

// mapZcodeStreamDelta 把 model.streaming 事件映射成文本/思考增量。
// 返回 (kind, delta, ok);kind = "text"|"reasoning"。
func mapZcodeStreamDelta(payload json.RawMessage) (string, string, bool) {
	var pl struct {
		Kind  string `json:"kind"`
		Delta string `json:"delta"`
		Done  bool   `json:"done"`
	}
	if json.Unmarshal(payload, &pl) != nil || pl.Delta == "" {
		return "", "", false
	}
	switch pl.Kind {
	case "text_delta":
		return "text", pl.Delta, true
	case "reasoning_delta":
		return "reasoning", pl.Delta, true
	}
	return "", "", false
}

// subagentResultClip 子代理委派工具结果用更宽的限长(zcodeSubagentToolClip),
// 普通工具沿用 zcodeToolClip。ids 是本轮已识别的子代理工具调用集合(情况 B)。
func subagentResultClip(ids map[string]bool, toolCallID string) int {
	if ids[toolCallID] {
		return zcodeSubagentToolClip
	}
	return zcodeToolClip
}

// mapZcodeToolCall 从 tool.updated 提取工具调用开始帧(STARTED)。
//
// 【kind 双代兼容(2026-10-07 字段级审计,证据=bundle 静态 zod union)】
//   - 0.16.5 探针实测(cmd/zcprobe):kind="tool_call"(含完整 input);
//   - 本机 fork(bundle eGt discriminatedUnion,2026-09-29 构建)已无 "tool_call":
//     工具开始拆成 kind="scheduled"(带 input/inputByteLength/inputOmitted 等)
//     与 kind="started"(仅 startedAt)。旧映射只认 "tool_call",fork 上工具
//     起始卡整帧静默丢失(用户只看到结果、看不到"正在跑什么")。
// 三种 kind 都映射 STARTED:同 id 的重复卡由手机端按 callId 原地替换,
// scheduled→started 连发等价于一次刷新。
func mapZcodeToolCall(payload json.RawMessage) (pb.ToolCallUpdate, bool) {
	var pl struct {
		Kind       string          `json:"kind"`
		ToolCallID string          `json:"toolCallId"`
		ToolName   string          `json:"toolName"`
		Input      json.RawMessage `json:"input"`
	}
	if json.Unmarshal(payload, &pl) != nil || pl.ToolCallID == "" {
		return pb.ToolCallUpdate{}, false
	}
	switch pl.Kind {
	case "tool_call", "started", "scheduled":
	default:
		return pb.ToolCallUpdate{}, false
	}
	// fork 的 kind="started" 变体 toolName 是可选(base Zpe.optional)——空名卡
	// 没有可读标识,兜底通用名(与 buildZcodeApprovalCall 同口径)。
	if pl.ToolName == "" {
		pl.ToolName = "tool"
	}
	return pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_STARTED,
		Call: &pb.ToolCallBlock{
			Id:       pl.ToolCallID,
			Name:     pl.ToolName,
			JsonArgs: mustJSON(rawOrNull(pl.Input)),
		},
	}, true
}

// mapZcodeToolResult 从 tool.updated(payload.kind="result")提取工具结果。
// 默认限长 zcodeToolClip;子代理委派结果要完整透出"产出",用
// mapZcodeToolResultClipped 传 zcodeSubagentToolClip。
func mapZcodeToolResult(payload json.RawMessage) (*pb.ToolCallUpdate, bool) {
	return mapZcodeToolResultClipped(payload, zcodeToolClip)
}

// mapZcodeToolResultClipped 同 mapZcodeToolResult,限长可由调用方指定
// (子代理委派放宽到 zcodeSubagentToolClip,普通工具沿用 zcodeToolClip)。
//
// 【2026-10-07 字段级审计补齐】
//   - content 改按 RawMessage 接:bundle 里 result 段是 m.unknown()(k5i),
//     content 不保证是字符串;非字符串时按原 JSON 文本透出,不再因反序列化
//     失败把整个结果帧静默丢掉;
//   - duration(毫秒,fork union 实测字段)透到 ToolResultBlock.ElapsedMs,
//     此前静默丢弃。
func mapZcodeToolResultClipped(payload json.RawMessage, clipN int) (*pb.ToolCallUpdate, bool) {
	var pl struct {
		Kind       string `json:"kind"`
		ToolCallID string `json:"toolCallId"`
		ToolName   string `json:"toolName"`
		Result     struct {
			Success bool            `json:"success"`
			Content json.RawMessage `json:"content"`
		} `json:"result"`
		Duration float64 `json:"duration"`
	}
	if json.Unmarshal(payload, &pl) != nil || pl.Kind != "result" || pl.ToolCallID == "" {
		return nil, false
	}
	upd := pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_FINISHED,
		Call: &pb.ToolCallBlock{
			Id:       pl.ToolCallID,
			Name:     pl.ToolName,
			JsonArgs: "{}",
		},
		Result: &pb.ToolResultBlock{
			CallId:    pl.ToolCallID,
			Ok:        pl.Result.Success,
			Output:    clip(zcodeResultContentText(pl.Result.Content), clipN),
			ElapsedMs: pl.Duration,
		},
	}
	return &upd, true
}

// zcodeResultContentText 把 tool.updated(kind="result")的 result.content 归一成
// 展示文本:字符串原样;其它 JSON 形状(数组/对象/数字)按原文透出 —— 结果内容
// 不允许因为"形状不是字符串"就被静默丢掉;空值给空串。
func zcodeResultContentText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

// mapZcodeToolProgress 从 tool.updated(payload.kind="progress")提取工具执行
// 进度帧(UPDATE)。fork union 实测字段(bundle):elapsedMs/pid/stdoutBytes/
// stderrBytes/outputBytes/outputPreview{text,fullText?,totalLines,totalBytes,
// linesEstimated}/stdoutTail/stderrTail —— 此前整个 kind 落在三个旧映射之外,
// Run 链连诊断留痕都没有,是纯静默丢弃。映射与 pi 的 tool_execution_update
// 同款:UPDATE 帧 + Result.Ok=true(进行中没有失败语义),输出取可读预览。
func mapZcodeToolProgress(payload json.RawMessage) (pb.ToolCallUpdate, bool) {
	var pl struct {
		Kind          string  `json:"kind"`
		ToolCallID    string  `json:"toolCallId"`
		ToolName      string  `json:"toolName"`
		ElapsedMs     float64 `json:"elapsedMs"`
		Pid           int     `json:"pid"`
		OutputPreview *struct {
			Text     string `json:"text"`
			FullText string `json:"fullText"`
		} `json:"outputPreview"`
		StdoutTail string `json:"stdoutTail"`
		StderrTail string `json:"stderrTail"`
	}
	if json.Unmarshal(payload, &pl) != nil || pl.Kind != "progress" || pl.ToolCallID == "" {
		return pb.ToolCallUpdate{}, false
	}
	var parts []string
	if pl.OutputPreview != nil {
		if t := firstNonEmpty(pl.OutputPreview.Text, pl.OutputPreview.FullText); t != "" {
			parts = append(parts, t)
		}
	}
	if pl.StdoutTail != "" {
		parts = append(parts, "stdout: "+pl.StdoutTail)
	}
	if pl.StderrTail != "" {
		parts = append(parts, "stderr: "+pl.StderrTail)
	}
	output := strings.Join(parts, "\n")
	if output == "" {
		output = "工具执行中…"
	}
	return pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_UPDATE,
		Call: &pb.ToolCallBlock{
			Id:       pl.ToolCallID,
			Name:     pl.ToolName,
			JsonArgs: "{}",
		},
		Result: &pb.ToolResultBlock{
			CallId:    pl.ToolCallID,
			Ok:        true, // 进行中:没有失败语义(pi 同款)
			Output:    clip(output, zcodeToolClip),
			ElapsedMs: pl.ElapsedMs,
		},
	}, true
}

// zcodeToolPayloadKind 取 tool.updated 载荷的 kind 判别字段(纯读,不猜内容)。
func zcodeToolPayloadKind(payload json.RawMessage) string {
	var pl struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(payload, &pl) != nil {
		return ""
	}
	return pl.Kind
}

// zcodeToolRestNoticeKind 给"映射不了的 tool.updated kind"分档:batch(并行组
// 完成汇总,单卡无 toolCallId)/ raw(桌面桥原始透传)是 fork union 的已知成员
// —— 刻意不投影但必须留痕,标 PASSTHROUGH 与"只透传"口径一致;其余(含空)
// 如实 UNKNOWN,不替没见过的 kind 编身份。
func zcodeToolRestNoticeKind(kind string) string {
	switch kind {
	case "batch", "raw":
		return runtime.KindKnownPassthrough
	}
	return runtime.KindUnknownNotification
}

// mapZcodeToolError 从 tool.updated 提取工具失败帧(FINISHED,Ok=false)。
// 【kind 双代兼容】0.16.5 探针实测 kind="tool_error"(样本曾被截断,按
// best-effort 解析);本机 fork union 已改名 kind="error",负载同为
// {toolCallId, error:{message,type}, …}(bundle 静态)。两者都认,失败卡
// 不再因改名整帧静默丢失。
func mapZcodeToolError(payload json.RawMessage) (pb.ToolCallUpdate, bool) {
	var pl struct {
		Kind       string `json:"kind"`
		ToolCallID string `json:"toolCallId"`
		ToolName   string `json:"toolName"`
		Error      *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(payload, &pl) != nil || pl.ToolCallID == "" {
		return pb.ToolCallUpdate{}, false
	}
	switch pl.Kind {
	case "tool_error", "error":
	default:
		return pb.ToolCallUpdate{}, false
	}
	msg := "工具执行出错"
	if pl.Error != nil && pl.Error.Message != "" {
		msg = pl.Error.Message
	} else if pl.Message != "" {
		msg = pl.Message
	}
	return pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_FINISHED,
		Call: &pb.ToolCallBlock{
			Id:       pl.ToolCallID,
			Name:     pl.ToolName,
			JsonArgs: "{}",
		},
		Result: &pb.ToolResultBlock{
			CallId: pl.ToolCallID,
			Ok:     false,
			Output: clip(msg, zcodeToolClip),
		},
	}, true
}

// mapZcodeTurnCompleted 解析 turn.completed:返回 (最终正文兜底, 用量, ok)。
// usage 形状为事件的地面真值(turn.completed payload,探针实测)。
func mapZcodeTurnCompleted(payload json.RawMessage) (string, *pb.Usage, bool) {
	var pl struct {
		Response   string `json:"response"`
		ResultType string `json:"resultType"`
		Usage      struct {
			InputTokens      int64 `json:"inputTokens"`
			OutputTokens     int64 `json:"outputTokens"`
			TotalTokens      int64 `json:"totalTokens"`
			CacheReadTokens  int64 `json:"cacheReadTokens"`
			CacheWriteTokens int64 `json:"cacheWriteTokens"`
			ReasoningTokens  int64 `json:"reasoningTokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(payload, &pl) != nil {
		return "", nil, false
	}
	usage := &pb.Usage{
		InputTokens:      pl.Usage.InputTokens,
		OutputTokens:     pl.Usage.OutputTokens,
		CacheReadTokens:  pl.Usage.CacheReadTokens,
		CacheWriteTokens: pl.Usage.CacheWriteTokens,
		ReasoningTokens:  pl.Usage.ReasoningTokens,
	}
	return pl.Response, usage, true
}

// zcodeTurnCompletedResultType returns the provider's terminal classification.
// ZCode emits turn.completed for cancelled/aborted turns as well as successful
// turns; treating every such frame as success made a cancelled task appear
// completed with "(完成,无文本输出)".
func zcodeTurnCompletedResultType(payload json.RawMessage) string {
	var pl struct {
		ResultType string `json:"resultType"`
	}
	if json.Unmarshal(payload, &pl) != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(pl.ResultType))
}

func zcodeTurnCompletedIsSuccess(payload json.RawMessage) bool {
	switch zcodeTurnCompletedResultType(payload) {
	case "", "success", "completed", "complete":
		return true
	default:
		return false
	}
}

// mapZcodeTurnFailed 解析 turn.failed 的错误信息。
func mapZcodeTurnFailed(payload json.RawMessage) (string, bool) {
	var pl struct {
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
		TurnPhase string `json:"turnPhase"`
	}
	if json.Unmarshal(payload, &pl) != nil {
		return "", false
	}
	if pl.Error != nil && pl.Error.Message != "" {
		return pl.Error.Message, true
	}
	if pl.TurnPhase != "" {
		return "轮次失败(turnPhase=" + pl.TurnPhase + ")", true
	}
	return "轮次失败", true
}

// zcodePermissionRequest interaction/requestPermission 的参数(审批的地面真值)。
type zcodePermissionRequest struct {
	RequestID  string          `json:"requestId"`
	ToolCallID string          `json:"toolCallId"`
	ToolName   string          `json:"toolName"`
	RiskLevel  string          `json:"riskLevel"`
	Reason     string          `json:"reason"`
	Input      json.RawMessage `json:"input"`
	Options    []struct {
		OptionID string `json:"optionId"`
		Kind     string `json:"kind"`
	} `json:"options"`
}

// buildZcodeApprovalCall 把权限请求翻译成手机审批卡。
// 【vet】返回指针:pb 消息含锁,调用方(zcode.go:945)按调用结果直传 approve,
// ApprovalHook 已改指针参数,这里必须回传指针(内容不变)。
func buildZcodeApprovalCall(pr zcodePermissionRequest) *pb.ToolCallBlock {
	name := pr.ToolName
	if name == "" {
		name = "tool"
	}
	return &pb.ToolCallBlock{
		Id:       pr.ToolCallID,
		Name:     name,
		JsonArgs: mustJSON(rawOrNull(pr.Input)),
	}
}

// zcodeApprovalAnswer 构造审批应答(纯函数,单测覆盖)。
//
// 【实测】服务器按 zod schema Jb 严格校验应答:{decision, reason?, modifiedInput?,
// permissionUpdates?} —— 只允许这四个键。带 optionId 的回答是桌面 UI 内部路径的
// 形状,走 stdio 回会被判非法 → permission.resolved{decision:"deny"}。
func zcodeApprovalAnswer(allow bool, note string) map[string]interface{} {
	if allow {
		return map[string]interface{}{"decision": "allow", "reason": "Approved once"}
	}
	if note == "" {
		note = "用户在手机上拒绝了该操作"
	}
	return map[string]interface{}{"decision": "deny", "reason": note}
}

// zcodeResponder zcodeapp.Client 的应答面(最小接口,单测可注 fake,不依赖真子进程)。
type zcodeResponder interface {
	Respond(rawID json.RawMessage, result interface{}) error
	RespondError(rawID json.RawMessage, code int, message string) error
}

// zcodeApprovalTimeoutNote 审批等待超时自动拒绝的人话原因(deny reason,桌面端
// 会把它当成用户侧拒绝原因展示给 agent —— 说清是超时兜底,不是人的意志)。
const zcodeApprovalTimeoutNote = "手机端审批等待超时(2 分钟),已自动拒绝该操作"

// handleZcodePermission 处理 interaction/requestPermission(从 Run 的 ReqHandler
// 抽出,便于单测钉死超时语义;budget 常态传 zcodeUIBudget,测试注入短超时)。
// threadID 是我们自己的会话 id,供应答写回失败的日志定位到具体会话(与 codex/dsh
// 的 B4 留痕同口径)。
func (p *ZcodeProvider) handleZcodePermission(rawID, params json.RawMessage, ctx context.Context, rsp zcodeResponder, approve ApprovalHook, waiting *atomic.Bool, budget time.Duration, threadID string) bool {
	var pr zcodePermissionRequest
	if json.Unmarshal(params, &pr) != nil {
		_ = rsp.RespondError(rawID, -32602, "审批参数无法解析")
		return true
	}
	if approve == nil {
		// 【P1-2】审批通道缺失:不能回 {"decision":"deny","reason":"用户在手机上
		// 拒绝了该操作"} —— 手机从未被询问,却向 zcode 桌面端假称"用户拒绝",
		// 虚假归因会写进 agent 对话上下文(红线 1)。回 error 让 zcode 按通道
		// 故障处理(askViaPhone 同款口径:基础设施问题不伪装成用户意志)。
		log.Printf("[zcode] 审批通道不可用(approve==nil),回 error")
		_ = rsp.RespondError(rawID, -32000, "审批通道不可用")
		return true
	}
	// 【B4 应答必达性留痕】decision 已定,写回失败(子进程恰好退出/管道断裂)
	// 不再 _ = 丢弃 —— 落一行日志带线程 id 与方法名,与"绝不伪造 decision"红线
	// 配套:痕迹不改变应答语义,只保证"答案没送到"可追查。
	respond := func(result interface{}) {
		if err := rsp.Respond(rawID, result); err != nil {
			log.Printf("[zcode] 审批应答写回失败 thread=%s method=interaction/requestPermission: %v", threadID, err)
		}
	}
	waiting.Store(true)
	defer waiting.Store(false)
	allow, note := false, ""
	askErr := error(nil)
	if approve != nil {
		// 【P2-6】等待必须有上界(对齐 pi 的 piUIBudget):手机端停在审批卡但
		// 流不断时,无上界会一直等;超时按 deny 回人话,让桌面端流程继续走
		// (与 pi 的"未取得手机决定回 cancelled"同口径),不把整轮拖死。
		actx, acancel := context.WithTimeout(ctx, budget)
		defer acancel()
		// 【vet】pb 消息按值存变量再传接口会报 copies lock;直接传复合调用
		// 结果则不报(pi.go 同款注释)。approve 拿到的卡与 call 完全同值。
		allow, note, askErr = approve(actx, buildZcodeApprovalCall(pr))
	}
	if askErr != nil {
		if errors.Is(askErr, context.DeadlineExceeded) {
			log.Printf("[zcode] 审批等待超时(%s),已自动拒绝", budget)
			respond(zcodeApprovalAnswer(false, zcodeApprovalTimeoutNote))
			return true
		}
		// 手机端断流:这不是"用户拒绝"。回 error,让 zcode 按通道故障处理,
		// 别把基础设施问题伪装成用户意志(与 codex 同款)。
		log.Printf("[zcode] 审批通道失败: %v", askErr)
		_ = rsp.RespondError(rawID, -32000, "审批通道失败: "+askErr.Error())
		return true
	}
	if allow {
		// 【实测】应答必须严格匹配 schema Jb(decision/reason,strict):
		// 多余键(如 optionId)会让服务器抛异常 → deny"Permission request failed"。
		respond(zcodeApprovalAnswer(true, ""))
	} else {
		respond(zcodeApprovalAnswer(false, note))
	}
	return true
}

// rawOrNull 空参数归一成 "{}"(JsonArgs 不允许空串)。
func rawOrNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("{}")
	}
	return raw
}

// ---- 反问类反向请求(requestUserInput / elicitation)----
//
// 【地面真值】zcode.cjs 内嵌 zod schema(2026-09-18 提取):
//   - interaction/requestUserInput 参数 DXn(strict):
//     {requestId*, sessionId*, turnId?, toolCallId?, toolName?, prompt?,
//      questions?:[{question*, header*, options*:[{value*,label*,description?,preview?}].min(1),
//      multiSelect?}].min(1), input?, origin?, schema?};
//     应答 jge(strict):{action:"accept"|"decline"|"cancel", content?:object, reason?:string}。
//   - elicitation/create = MCP 标准 ElicitRequest:form 模式
//     {mode?:"form", message*, requestedSchema:{type:"object", properties, required?}},
//     url 模式 {mode:"url", message*, elicitationId*, url*};应答 {action, content?}。
//
// 【呈现】两类统一复用 Android 反问表单(工具名 request_user_input,与 codex
// 同款形状,见 buildAskUserCall);-32601 只留给真正无法呈现的(url 模式 /
// 连一个可呈现的问题都拼不出来)。

// zcodeUserInputQuestion interaction/requestUserInput 的一条问题(schema AJt)。
type zcodeUserInputQuestion struct {
	Question    string `json:"question"`
	Header      string `json:"header"`
	MultiSelect bool   `json:"multiSelect"`
	Options     []struct {
		Value       string `json:"value"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Preview     string `json:"preview"`
	} `json:"options"`
}

// zcodeUserInputRequest interaction/requestUserInput 的参数(只声明消费的字段)。
type zcodeUserInputRequest struct {
	RequestID  string                   `json:"requestId"`
	SessionID  string                   `json:"sessionId"`
	TurnID     string                   `json:"turnId"`
	ToolCallID string                   `json:"toolCallId"`
	ToolName   string                   `json:"toolName"`
	Prompt     string                   `json:"prompt"`
	Questions  []zcodeUserInputQuestion `json:"questions"`
}

// buildZcodeUserInputCall 把 requestUserInput 翻译成手机反问卡。
// 卡上问题 id 固定 q0..qN;应答侧按服务器的收集规则(BCs 以 question 原文为键)
// 把答案映射回问题原文。ok=false 表示连一个可呈现的问题都拼不出来。
func buildZcodeUserInputCall(req zcodeUserInputRequest, cardID string) (pb.ToolCallBlock, bool) {
	qs := make([]uiAskQuestion, 0, len(req.Questions))
	for i, q := range req.Questions {
		// 选项三字段全透出:description/preview 是 zcode 原生 schema(AJt)的一部分,
		// 以前只上屏 label 等于丢信息;label 仍是答案回传值(服务器按 label=value
		// 收集,jCs:value:t.label),value 不上屏。
		opts := make([]uiAskOption, 0, len(q.Options))
		for _, o := range q.Options {
			opts = append(opts, uiAskOption{Label: o.Label, Description: o.Description, Preview: o.Preview})
		}
		qs = append(qs, uiAskQuestion{
			ID:          fmt.Sprintf("q%d", i),
			Question:    firstNonEmpty(q.Question, q.Header),
			Options:     opts,
			MultiSelect: q.MultiSelect, // 多选题在卡 JSON 标注 multiSelect:true,手机渲染多选态
		})
	}
	message := req.Prompt
	if len(qs) == 0 {
		if message == "" {
			return pb.ToolCallBlock{}, false
		}
		// 纯 prompt 无 questions:也给一个输入框,让用户有地方作答。
		qs = append(qs, uiAskQuestion{ID: "q0", Question: message})
		message = "agent 需要你的输入"
	}
	return buildAskUserCall(cardID, message, qs), true
}

// zcodeUserInputAnswer 构造 requestUserInput 的应答(schema jge,strict:只允许
// action/content/reason 三键)。accept 的 content 按"问题原文→答案"收集
// (服务器 BCs 按 WCs=question 原文取 content.answers);一个答案都没有时回
// content:{}(与桌面端 optionId 放行路径 MCs 的 {action:"accept",content:{}} 同款)。
func zcodeUserInputAnswer(allow bool, note string, req zcodeUserInputRequest) map[string]interface{} {
	if !allow {
		return map[string]interface{}{"action": "decline", "reason": askDenyReason(note)}
	}
	answers := answerNoteMap(note)
	content := map[string]interface{}{}
	for i, q := range req.Questions {
		a := answers[fmt.Sprintf("q%d", i)]
		if a == "" {
			continue
		}
		if key := firstNonEmpty(q.Question, q.Header); key != "" {
			content[key] = a
		}
	}
	return map[string]interface{}{"action": "accept", "content": content}
}

// zcodeElicitationProperty requestedSchema.properties 的条目(MCP 原始类型定义,
// 只声明渲染要用的字段;enum 有值时渲染成点选项)。
type zcodeElicitationProperty struct {
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Enum        []string `json:"enum"`
}

// zcodeElicitationRequest elicitation/create 的参数(MCP 标准两形态)。
type zcodeElicitationRequest struct {
	Mode            string `json:"mode"`
	Message         string `json:"message"`
	ElicitationID   string `json:"elicitationId"`
	URL             string `json:"url"`
	RequestedSchema struct {
		Type       string                              `json:"type"`
		Properties map[string]zcodeElicitationProperty `json:"properties"`
		Required   []string                            `json:"required"`
	} `json:"requestedSchema"`
}

// buildZcodeElicitationCall 把 form 模式 elicitation 翻译成手机反问卡。
// 返回 (卡, 属性名顺序, ok):属性名顺序供应答侧把 qN 映射回 content 键
// (MCP 要求 content 键 = requestedSchema.properties 键);属性按名字排序,
// 保证卡片顺序稳定可复现(map 遍历无序)。ok=false = url 模式,手机呈现不了。
func buildZcodeElicitationCall(req zcodeElicitationRequest, cardID string) (pb.ToolCallBlock, []string, bool) {
	if req.Mode == "url" {
		return pb.ToolCallBlock{}, nil, false
	}
	names := make([]string, 0, len(req.RequestedSchema.Properties))
	for n := range req.RequestedSchema.Properties {
		names = append(names, n)
	}
	sort.Strings(names)
	qs := make([]uiAskQuestion, 0, len(names))
	for i, n := range names {
		prop := req.RequestedSchema.Properties[n]
		qs = append(qs, uiAskQuestion{
			ID:       fmt.Sprintf("q%d", i),
			Question: firstNonEmpty(prop.Title, n),
			Options:  plainOptions(prop.Enum),
		})
	}
	message := firstNonEmpty(req.Message, "MCP 服务需要你填写内容")
	if len(qs) == 0 {
		// 空 form(无属性):没有可填项,给一个说明性条目让用户能"确认/取消"。
		qs = append(qs, uiAskQuestion{ID: "q0", Question: message})
		message = "MCP 服务请求确认"
	}
	return buildAskUserCall(cardID, message, qs), names, true
}

// zcodeElicitationAnswer 构造 elicitation/create 的应答(MCP ElicitResult:
// {action:"accept"|"decline"|"cancel", content?},content 键必须是 properties 键)。
func zcodeElicitationAnswer(allow bool, note string, req zcodeElicitationRequest, names []string) map[string]interface{} {
	if !allow {
		return map[string]interface{}{"action": "decline"}
	}
	answers := answerNoteMap(note)
	content := map[string]interface{}{}
	for i, n := range names {
		if a := answers[fmt.Sprintf("q%d", i)]; a != "" {
			content[n] = a
		}
	}
	return map[string]interface{}{"action": "accept", "content": content}
}

// askDenyReason 拒绝反问时的人话原因(服务器会把它转成 deny reason 给 agent 看)。
func askDenyReason(note string) string {
	if r := strings.TrimSpace(note); r != "" {
		return r
	}
	return "用户在手机上拒绝回答"
}

// ---- 反问类反向请求的接线(Run 的 ReqHandler 分发到这里)----
//
// 【与审批分支的分工】呈现复用手机反问表单(approve hook 通道,ApprovalReason 对
// request_user_input 有专属文案);应答按各自 schema 组装;-32601 只留给"真的
// 呈现不了"的形态(url 模式 elicitation / 拼不出任何问题)。通道故障回 -32000,
// 不把基础设施问题伪装成用户意志(与审批分支同款)。

// rawIDString 服务器反向请求 id 是 JSON 字符串("server-6"),解出裸文本做卡片 id。
func rawIDString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

// askViaPhone 把反问卡交给手机审批通道等应答;等待期间看门狗放行(用户想清楚
// 再答是合法静默,与审批分支同款)。approve 为 nil 视为通道不可用。
func (p *ZcodeProvider) askViaPhone(ctx context.Context, call *pb.ToolCallBlock, approve ApprovalHook, waiting *atomic.Bool) (bool, string, error) {
	waiting.Store(true)
	defer waiting.Store(false)
	if approve == nil {
		return false, "", errors.New("审批通道不可用")
	}
	return approve(ctx, call)
}

// handleZcodeUserInput 处理 interaction/requestUserInput(agent 反问卡):
// questions 渲染成点选/输入表单,答案按"问题原文→答案"收进 content(schema jge)。
func (p *ZcodeProvider) handleZcodeUserInput(rawID, params json.RawMessage, ctx context.Context, rsp zcodeResponder, approve ApprovalHook, waiting *atomic.Bool) bool {
	var req zcodeUserInputRequest
	if json.Unmarshal(params, &req) != nil {
		_ = rsp.RespondError(rawID, -32602, "反问参数无法解析")
		return true
	}
	// 卡片 id:requestId 是服务器侧的关联键(schema 必填),缺失退服务器请求 id。
	cardID := firstNonEmpty(req.RequestID, rawIDString(rawID))
	call, ok := buildZcodeUserInputCall(req, cardID)
	if !ok {
		// 连一个可呈现的问题都拼不出来:回"客户端不支持",让服务器按自己的
		// 兜底路径走(绝不伪造一个空 accept 让 agent 拿着空气继续跑)。
		_ = rsp.RespondError(rawID, -32601, "PocketAgent 无法呈现该反问(无问题且无提示语)")
		return true
	}
	allow, note, err := p.askViaPhone(ctx, &call, approve, waiting)
	if err != nil {
		log.Printf("[zcode] 反问通道失败(requestId=%s): %v", req.RequestID, err)
		_ = rsp.RespondError(rawID, -32000, "反问通道失败: "+err.Error())
		return true
	}
	_ = rsp.Respond(rawID, zcodeUserInputAnswer(allow, note, req))
	return true
}

// handleZcodeElicitation 处理 elicitation/create(MCP 标准反问):form 模式按
// requestedSchema 渲染表单(enum 属性渲染成点选项),应答 content 键 = 属性名。
// -32601 只留给 url 模式 —— 手机没有"打开外部 URL 等回调"的呈现形态。
func (p *ZcodeProvider) handleZcodeElicitation(rawID, params json.RawMessage, ctx context.Context, rsp zcodeResponder, approve ApprovalHook, waiting *atomic.Bool) bool {
	var req zcodeElicitationRequest
	if json.Unmarshal(params, &req) != nil {
		_ = rsp.RespondError(rawID, -32602, "elicitation 参数无法解析")
		return true
	}
	call, names, ok := buildZcodeElicitationCall(req, rawIDString(rawID))
	if !ok {
		_ = rsp.RespondError(rawID, -32601, "PocketAgent 无法呈现 url 模式的 elicitation")
		return true
	}
	allow, note, err := p.askViaPhone(ctx, &call, approve, waiting)
	if err != nil {
		log.Printf("[zcode] elicitation 通道失败: %v", err)
		_ = rsp.RespondError(rawID, -32000, "elicitation 通道失败: "+err.Error())
		return true
	}
	_ = rsp.Respond(rawID, zcodeElicitationAnswer(allow, note, req, names))
	return true
}

// ---- 导入会话的历史惰性拉取 ----
//
// 【背景】从桌面端导入的 ZCode 会话(agents_sessions.go)只绑元信息
// (Thread.ExternalSession.SessionID),首条消息之前的历史不在我们的库里,
// 手机时间线从导入时刻起算、看不到导入前的对话。
//
// 【地面真值】session/resume 的应答自带完整 messages[](探针实证,同进程与
// 新进程一致,见 docs/verify/evidence/zcode-p0-probe.md §4.6):
//
//	{"messages":[{"info":{"role":"user"|"assistant","semantics":{"kind":…},
//	  "time":{"created":<unixMs>}},"parts":[{"type":"text","text":…}]}]}
//
// 本节就是把这个应答映射成 {role,text} 序列,server 层(GetThreadHistory)
// 首次打开导入线程时调一次并落库缓存。与 ensureSession 的重挂载走同一个
// session/resume RPC,但不碰 threadMap、不 subscribe(纯读,不订阅事件流)。

// HistoryMsg 导入历史的一条消息(供 server 层落库/上屏的最小形状)。
type HistoryMsg struct {
	Role            string // "user" | "assistant"(其余角色在解析时被丢弃)
	Text            string // 该消息全部 text 部件按原顺序拼接(多段用 \n 连接)
	CreatedAtUnixMs int64  // zcode info.time.created(毫秒);0 = 应答里没带
}

// HistoryFor 拉取一个 zcode 会话的完整历史。
//
// 内部 withClientCtx(子进程没起会自动拉起)+ session/resume 读取当前消息;
// 失败(配额/进程起不来/会话已被桌面端删除)返回错误,由调用方降级 ——
// 绝不崩,也不伪造空历史冒充成功。
//
// 【与轮次的互不干扰】withClientCtx 带 TryLock/turnActive 守卫:轮次跑着时
// 若子进程死亡不会中途重启(避免切断在跑的会话),直接报错降级。
func (p *ZcodeProvider) HistoryFor(ctx context.Context, sessionID string) ([]HistoryMsg, error) {
	var out []HistoryMsg
	err := p.withClientCtx(ctx, zcodeStatusBudget, func(ctx context.Context, c *zcodeapp.Client) error {
		raw, err := c.CallCtx(ctx, "session/resume", map[string]interface{}{"sessionId": sessionID})
		if err != nil {
			return fmt.Errorf("session/resume: %w", err)
		}
		out = parseZcodeHistory(raw)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// zcodeResumeReply session/resume 应答里我们消费的字段(其余快照字段忽略)。
type zcodeResumeReply struct {
	Messages []zcodeHistoryMessage `json:"messages"`
}

// zcodeHistoryMessage resume messages[] 的一条(info + parts 两级)。
type zcodeHistoryMessage struct {
	Info struct {
		Role      string `json:"role"`
		Semantics struct {
			// "timeline_event" = 换模型等系统时间线(探针实测),不是对话内容
			Kind string `json:"kind"`
		} `json:"semantics"`
		Time struct {
			Created int64 `json:"created"` // unix 毫秒
		} `json:"time"`
	} `json:"info"`
	Parts []struct {
		Type string `json:"type"` // text / reasoning / step-start / step-finish / timeline / …
		Text string `json:"text"`
	} `json:"parts"`
}

// parseZcodeHistory 把 session/resume 应答映射成 {role,text} 时间线
// (纯函数,单测覆盖)。规则:
//   - timeline_event(info.semantics.kind)整条丢弃 —— 换模型分隔条不是对话;
//   - role 只认 user/assistant,其余(system 等)丢弃;
//   - parts 只取 type=="text" 的文本;reasoning(思考)/step-start/step-finish/
//     工具类部件不进手机时间线(与 server 落库的 user/assistant 文本同口径);
//   - 拼完没有任何文本的空消息丢弃。
//
// 应答无法解析时返回 nil(调用方按"空历史"处理,不报错 —— 快照形状异常
// 不该把导入线程的历史页打挂)。
func parseZcodeHistory(raw json.RawMessage) []HistoryMsg {
	var reply zcodeResumeReply
	if json.Unmarshal(raw, &reply) != nil {
		return nil
	}
	out := make([]HistoryMsg, 0, len(reply.Messages))
	for i := range reply.Messages {
		m := &reply.Messages[i]
		if m.Info.Semantics.Kind == "timeline_event" {
			continue
		}
		if m.Info.Role != "user" && m.Info.Role != "assistant" {
			continue
		}
		var b strings.Builder
		for _, part := range m.Parts {
			if part.Type != "text" || part.Text == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(part.Text)
		}
		if b.Len() == 0 {
			continue
		}
		out = append(out, HistoryMsg{
			Role:            m.Info.Role,
			Text:            b.String(),
			CreatedAtUnixMs: m.Info.Time.Created,
		})
	}
	return out
}

// ---- 一轮 ----

// Run 执行完整一轮:session/send → 消费 session/event → turn.completed/failed 收尾。
func (p *ZcodeProvider) Run(ctx context.Context, req TurnRequest, sink Sink, approve ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	p.turnMu.Lock() // 同一时刻只跑一轮(与 codex 同款;手机端本来也是单会话交互)
	defer p.turnMu.Unlock()
	p.turnActive.Store(true)
	defer p.turnActive.Store(false)

	if err := p.withClient(func(*zcodeapp.Client) error { return nil }); err != nil {
		return nil, nil, err
	}

	// 【输入收集】文本拼接(零裁剪约定不变,与 codex 同款);图片字节暂存,
	// 等下面会话快照确定、做完能力门控后再映射成 session/send attachments
	// (见 zcodeSendParams;实测形状 2026-09-19 cmd/zcprobe -image)。
	var userText strings.Builder
	var userImgs []turnImage
	for i := range req.InputBlocks {
		switch t := req.InputBlocks[i].Block.(type) {
		case *pb.ContentBlock_Text:
			userText.WriteString(t.Text.Text)
		case *pb.ContentBlock_Image:
			if t.Image != nil && len(t.Image.Data) > 0 {
				userImgs = append(userImgs, turnImage{data: t.Image.Data, mediaType: t.Image.MediaType})
			}
		}
	}

	// 1) 准备会话 + 模型(失败语义:建不上 = 整轮失败;选不上模型 = 降级继续)
	// created 不再分流(P1-3 修复后新建/resume 一律下发模型)。
	sid, _, snapRaw, err := p.ensureSession(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	if mode := p.nativeModeFor(req.Thread); mode != "" {
		effective, err := applyZcodeNativeMode(ctx, p.client, sid, mode)
		if err != nil {
			return nil, nil, err
		}
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
			Kind: pb.CodexEvent_STATUS, Code: "native_notification", Title: "zcode · mode", Message: "原生权限模式: " + effective,
		}}})
	}
	// 【P1-3】每轮无条件 applyModel(原来只在 created==true 时下发):threadMap
	// 命中(同进程第 2 轮起)、daemon 重启后的 resume、导入线程的 resume 全部
	// created=false,修复前手机端"模型·强度"卡选的值静默失效 —— 与 server 层
	// applyTurnOverrides 的契约("会话内切换必须生效",thread.go:632)直接矛盾。
	// applyModel 对 default/空直通、setModel 幂等,重复下发无副作用;模型不可用
	// 记日志降级继续,不整轮失败。
	// 【P2-8】桌面默认 effort 走线程快照语义(desktopEffortFor):有快照的会话
	// 强度按快照装配,老会话不再随设置页全局漂移;与 codex 三级优先级一致。
	if err := p.applyModel(ctx, sid, req.Thread.ModelID, req.Thread.Effort, p.desktopEffortFor(req.Thread)); err != nil {
		return nil, nil, err
	}

	// 2) 事件路由:readLoop → 带缓冲 channel → 单消费者保序分发(慢 sink 不背压管道)
	type notifMsg struct {
		method string
		params json.RawMessage
	}
	notifCh := make(chan notifMsg, 4096)
	notifStop := make(chan struct{})
	notifDone := make(chan struct{})

	var (
		mu              sync.Mutex
		streamText      strings.Builder // text_delta 实时拼接(主来源,和手机看到的逐字一致)
		finalText       string          // turn.completed.response 兜底
		lastUsage       *pb.Usage
		lastErr         string
		lastNotif       atomic.Int64 // 最近一次收到通知的时间(看门狗用)
		approvalWaiting atomic.Bool  // 正在等手机审批:合法静默,看门狗跳过
		// subagentCalls 本轮识别出的子代理委派工具调用 id 集合(Run 的 notifCh
		// 单消费者 goroutine 内访问,无并发;情况 B:据此把结果限长放宽,让用户
		// 看到子代理产出)。识别依据 zcodeapp.ToolInputIsSubagent(纯结构、不写死名)。
		subagentCalls = map[string]bool{}
	)
	done := make(chan struct{})
	var doneOnce sync.Once
	finish := func() { doneOnce.Do(func() { close(done) }) }

	c := p.client
	// turnStartRef 是看门狗的单调参照:喂值点存"距参照的单调毫秒数",判定式
	// 用同一参照求差,差值即静默时长 —— 免疫墙钟 NTP 跳变(旧实现两侧各取
	// UnixMilli,跳变可误杀/漏杀)。不复用下方 turnStart:它声明在后,闭包
	// 作用域不可达;60 分钟上限仍用 turnStart(time.Since 本就单调,维持原状)。
	turnStartRef := time.Now()
	notifHandler := func(method string, params json.RawMessage) {
		lastNotif.Store(time.Since(turnStartRef).Milliseconds()) // 单调毫秒,免疫墙钟跳变
		select {
		case notifCh <- notifMsg{method: method, params: params}:
		case <-notifStop:
		}
	}
	reqHandler := func(rawID json.RawMessage, method string, params json.RawMessage) bool {
		switch method {
		case "interaction/requestPermission":
			log.Printf("[zcode] 审批请求 原始参数: %.400s", string(params))
			// 审批分支抽到 handleZcodePermission(P2-6:120s 上界 + 超时 deny
			// 人话 + 通道故障 -32000;单测可注 fake responder/短预算)。
			return p.handleZcodePermission(rawID, params, ctx, c, approve, &approvalWaiting, zcodeUIBudget, req.Thread.ID)
		case "interaction/requestUserInput":
			return p.handleZcodeUserInput(rawID, params, ctx, c, approve, &approvalWaiting)
		case "elicitation/create":
			return p.handleZcodeElicitation(rawID, params, ctx, c, approve, &approvalWaiting)
		}
		// 【辅线 B】开关 ON 时,未知反向请求先弹手机通用确认卡:允许 → 回空对象 {}
		// + 台账 KNOWN_PASSTHROUGH(用户看过脱敏原文后授权这次试探);拒绝/超时/
		// 通道缺失 → 未消费,落回下面的既有安全拒绝路径,与 OFF 态逐字节一致。
		if unknownPassthroughQuery() && p.handleZcodeUnknownRequest(rawID, method, req.Thread.ID, params, ctx, c, approve, &approvalWaiting, zcodeUIBudget) {
			return true
		}
		// 交回客户端层统一回 -32601 —— 不伪造应答(与 codex 的第一条规则同款)。
		// 回 error 只让对端知道"这次没做成",而"我们收到过一个不认识的请求"要能
		// 事后查(规格 §9:未知 request 保留原始 payload、关联会话)。与 codex/dsh
		// 同款登记;粒度差异如实声明:zcode 侧尚无 reqUnsupported/reqUnknown 分类器,
		// auth headers 等**已知拒答**也会进台账 —— 宁可台账有噪声,不可漏掉真未知
		// (上游发来全新 method 时,这里是唯一的痕迹)。
		recordUnknownRequestRejection(req.Thread.ID, "zcode", method, params)
		return false
	}
	c.SetTurnHandlers(notifHandler, reqHandler)
	defer func() {
		// Setter 立即发布卸载快照，不需要等下一次 CallCtx。
		c.SetTurnHandlers(nil, nil)
		close(notifStop)
		<-notifDone
	}()

	var notices nativeNoticeRecorder
	go func() {
		defer close(notifDone)
		for {
			select {
			case nm := <-notifCh:
				if nm.method == "state.updated" {
					if raw := zcodeStateUpdateSnapshot(nm.params, sid); len(raw) > 0 {
						p.InvalidateModels()
						emitRuntimeSelection(p.confirmedZcodeSelection(raw), sink)
						notices.recordAs(zcodeNoticeKind(nm.method), "zcode", nm.method, nm.params, sink)
					}
					continue
				}
				if nm.method != "session/event" {
					notices.recordAs(zcodeNoticeKind(nm.method), "zcode", nm.method, nm.params, sink)
					continue
				}
				var ev zcodeEventEnvelope
				if json.Unmarshal(nm.params, &ev) != nil {
					// 【Z-3 坏帧留痕 → B4 共享件(2026-10-09)】解包失败与异会话过滤
					// 此前共用一条 continue,拆开后坏帧走共享 recordBadFrame:方法名
					// 带 <unparseable> 标记,与全局钩子(zcodeLedgerMethod 对拆不开的
					// 壳记的裸 "session/event")可区分、可检索 —— 同一帧两条台账各说
					// 各的事实(全局=壳,泵=消费失败),台账是索引,记全真实路径。
					// 【裁量:只台账,不出限频 STATUS 卡】这不是坏帧的唯一留痕:
					// 常驻全局钩子对每条通知(含本帧)已先落 UNKNOWN 台账;传输级
					// 异常帧也没有用户可动手脚的处置路径,诊断卡只会添噪。异会话
					// 过滤维持下方静默 —— 全家同款刻意取舍,不写台账(会被并发
					// 会话流量刷爆 200 条滚动窗,见 unknown-bypass-audit §二"全家")。
					notices.recordBadFrame("zcode", "session/event", nm.params, nil)
					continue
				}
				if ev.SessionID != sid {
					continue // 别的会话的帧不进本轮(刻意过滤,与四家同款)
				}
				switch ev.Type {
				case "session.updated", "settings.updated", "session.modelUpdated":
					// 【0.16.9 实证(2026-09-24,zcode-0169-probe.log)】这三个
					// event.type 已从 0.16.9 事件流消失(setModel/setThoughtLevel
					// 只发顶层 state.updated,bundle 静态检索 0 次)——本分支在
					// 0.16.9 上是死代码,保留只为旧 bundle(≤0.16.5)兼容;现行
					// 载体的投影走上方 nm.method=="state.updated" 分支。
					p.recordProjection(req.Thread.ID, ev.Payload)
					p.InvalidateModels()
					emitRuntimeSelection(p.confirmedZcodeSelection(ev.Payload), sink)
					notices.recordAs(zcodeNoticeKind(ev.Type), "zcode", ev.Type, ev.Payload, sink)
				case "turn.started":
					log.Printf("[zcode] zcode 已接收任务 session=%s turn=%s", sid, ev.TurnID)
				case "model.streaming":
					kind, delta, ok := mapZcodeStreamDelta(ev.Payload)
					if !ok {
						continue
					}
					switch kind {
					case "text":
						mu.Lock()
						streamText.WriteString(delta)
						mu.Unlock()
						sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: delta}}})
					case "reasoning":
						// 隐藏推理过程(daemon 侧强制过滤,与 codex.go 同款):
						// 开关打开时 ThinkingDelta 一帧不出,见 emitThinkingFor。
						p.emitThinkingFor(req.Thread, delta, sink)
					}
				case "tool.updated":
					// 子代理委派识别(情况 B):ZCode 不暴露子代理内部对话,只透出
					// tool.updated 的入参/结果。tool_call/scheduled 阶段先记下 id,
					// result 阶段据此放宽限长,让手机端能看到子代理"承担了什么/产出了
					// 什么"(详见 zcodeapp.ToolInputIsSubagent 的识别依据,纯结构不写死名)。
					// 【2026-10-07】fork 把带 input 的起始帧改名 kind="scheduled",
					// 与旧 bundle 的 "tool_call" 并列识别。
					var toolProbe struct {
						Kind       string          `json:"kind"`
						ToolCallID string          `json:"toolCallId"`
						Input      json.RawMessage `json:"input"`
					}
					if json.Unmarshal(ev.Payload, &toolProbe) == nil {
						switch toolProbe.Kind {
						case "tool_call", "scheduled":
							if zcodeapp.ToolInputIsSubagent(toolProbe.Input) {
								subagentCalls[toolProbe.ToolCallID] = true
							}
						}
					}
					if upd, ok := mapZcodeToolCall(ev.Payload); ok {
						sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &upd}})
						continue
					}
					if upd, ok := mapZcodeToolResultClipped(ev.Payload, subagentResultClip(subagentCalls, toolProbe.ToolCallID)); ok {
						sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: upd}})
						continue
					}
					if upd, ok := mapZcodeToolError(ev.Payload); ok {
						sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &upd}})
						continue
					}
					if upd, ok := mapZcodeToolProgress(ev.Payload); ok {
						sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &upd}})
						continue
					}
					// fork union 的其余成员(batch=并行组完成汇总、raw=桌面桥原始透传,
					// bundle 静态在案)刻意不投影,但必须走诊断留痕 —— 此前这条路径
					// 连留痕都没有,是真正的静默丢弃;从未见过的 kind 如实 UNKNOWN。
					notices.recordAs(zcodeToolRestNoticeKind(zcodeToolPayloadKind(ev.Payload)),
						"zcode", "session/event:tool.updated:"+zcodeToolPayloadKind(ev.Payload), ev.Payload, sink)
				case "session.titleUpdated":
					var pl struct {
						Title string `json:"title"`
					}
					if json.Unmarshal(ev.Payload, &pl) == nil && pl.Title != "" && p.TitleHandler != nil {
						go p.TitleHandler(req.Thread.ID, pl.Title)
					}
				case "turn.completed":
					resp, usage, ok := mapZcodeTurnCompleted(ev.Payload)
					if ok {
						mu.Lock()
						finalText = resp
						lastUsage = usage
						if zcodeTurnCompletedIsSuccess(ev.Payload) {
							lastErr = ""
						} else {
							lastErr = "轮次未成功结束(resultType=" + zcodeTurnCompletedResultType(ev.Payload) + ")"
						}
						mu.Unlock()
						// 窗口来自会话快照的 projection(recordProjection 记账),
						// 手机端用它 + usage 算上下文占用环(审计 S-1;探针实证
						// projection.totalTokenCount 与 usage.totalTokens 同值)。
						sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Usage{Usage: &pb.UsageUpdate{
							Usage:              usage,
							ModelContextWindow: p.contextWindowFor(req.Thread.ID),
						}}})
					}
					finish()
				case "turn.failed":
					msg, ok := mapZcodeTurnFailed(ev.Payload)
					if ok {
						mu.Lock()
						lastErr = msg
						mu.Unlock()
					}
					finish()
				default:
					notices.recordAs(zcodeNoticeKind(ev.Type), "zcode", ev.Type, ev.Payload, sink)
				}
			case <-notifStop:
				return
			}
		}
	}()

	// 3) 发起本轮(STARTED 由 server 层统一发,这里不重复)
	// 图片输入:有图才做能力门控(supportsImage 判定不了放行,与 codex 同语义);
	// cleanup 在 Run 返回(轮次终局)后删除临时图片 —— live 实测 zcode 在轮次
	// 开始阶段就把 localPath 读走编码进模型请求(turn.started 事件的 attachments
	// 回显 + 用户消息 parts 出现 data URL),轮次结束后不再依赖原文件。
	imageOK := true
	if len(userImgs) > 0 {
		imageOK = p.zcodeSupportsImage(req.Thread.ModelID, snapRaw)
	}
	sendParams, imgCleanup := zcodeSendParams(sid, userText.String(), userImgs, imageOK)
	defer imgCleanup()
	if len(userImgs) > 0 {
		log.Printf("[zcode] session/send session=%s 输入=%d字 图片=%d张 imageOK=%v", sid, userText.Len(), len(userImgs), imageOK)
	}
	if _, err := p.client.CallCtx(ctx, "session/send", sendParams); err != nil {
		return nil, nil, fmt.Errorf("session/send: %w", err)
	}

	// 4) 等结束 + 看门狗(与 codex 同口径:150s 无事件且不在等审批 = 失败)
	lastNotif.Store(time.Since(turnStartRef).Milliseconds()) // 初喂:单调毫秒,免疫墙钟跳变
	turnStart := time.Now()
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	waitDone := false
	for !waitDone {
		select {
		case <-done:
			waitDone = true
		case <-ctx.Done():
			// 中断 = session/stop(尽力而为);手机断连也要交回已流出的内容
			_, _ = p.client.Call("session/stop", map[string]string{"sessionId": sid})
			mu.Lock()
			partial := streamText.String()
			mu.Unlock()
			if partial != "" {
				return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: partial}}}}, &pb.Usage{}, ctx.Err()
			}
			return nil, nil, ctx.Err()
		case <-tick.C:
			if !approvalWaiting.Load() {
				// 单调口径:ms 与判定两侧共用 turnStartRef,差值即静默时长,
				// 免疫墙钟 NTP 跳变。ms>=0 同 codex:初喂在等待循环前必然发生,
				// 旧 ms>0 防的"未喂值"态已不可达,判定恒生效。
				if ms := lastNotif.Load(); ms >= 0 && time.Since(turnStartRef).Milliseconds()-ms > zcodeWatchdogSilence.Milliseconds() {
					_, _ = p.client.Call("session/stop", map[string]string{"sessionId": sid})
					mu.Lock()
					partial := streamText.String()
					mu.Unlock()
					err := fmt.Errorf("zcode %s 无任何事件(任务可能未被接收,session=%s)", zcodeWatchdogSilence, sid)
					if partial != "" {
						return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: partial}}}}, &pb.Usage{}, err
					}
					return nil, nil, err
				}
			}
			if time.Since(turnStart) > 60*time.Minute {
				_, _ = p.client.Call("session/stop", map[string]string{"sessionId": sid})
				log.Printf("[zcode] 轮次超时(60 分钟),已打断 session=%s", sid)
				return nil, nil, fmt.Errorf("zcode 超时(60 分钟)")
			}
		}
	}
	log.Printf("[zcode] 一轮完成 session=%s 用时 %v", sid, time.Since(turnStart).Round(time.Millisecond))

	// 正常终局后补发暂存的常驻目标(取消/看门狗路径不补,避免与 session/stop
	// 竞争;goal 操作本身会驱动桌面端一轮确认 prompt,放在轮次外才安全)。
	// 异步:不延迟本轮结果返回手机。
	go p.applyPendingGoal(req.Thread.ID, sid)
	// 【子代理原生事实】轮次终局拉一次原生子代理清单(0.16.9 session/subagents,
	// 见 zcode_subagents.go):本轮委派的子代理此刻已有事实,server 层据此登记
	// 存根,手机子代理卡从"tool.updated 结构推测"升级为原生清单。异步,失败
	// 只留痕,绝不拖住本轮结果返回。
	go p.fetchSubagentsForTurn(req.Thread.ID, sid)

	mu.Lock()
	defer mu.Unlock()
	if lastErr != "" {
		return nil, nil, fmt.Errorf("zcode: %s", lastErr)
	}
	text := streamText.String()
	if text == "" {
		text = finalText
	}
	// 【空就是空】见 codex.go 同处注释:占位文本会把"真的没话说""只有产物"
	// "适配器丢正文"压成同一条普通 assistant 文本,用户无从判断该不该重试。
	// 如实返回空块,OutputState 由 server 层按 blocks 判定。
	if text == "" {
		return nil, lastUsage, nil
	}
	blocks := []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}}}}
	return blocks, lastUsage, nil
}

func (p *ZcodeProvider) InvalidateModels() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.modelsAt = time.Time{}
}
