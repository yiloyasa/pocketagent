package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/version"
)

// CodexProvider 把我们的 gRPC 协议桥接到 codex app-server。
//
// 【讲解】事件对照表(手机上看到的 ← codex 发来的):
//
//	文字逐字蹦出   agentMessage/delta
//	思考过程       reasoning/…/delta
//	"⚙ shell"卡片 item/started (commandExecution)
//	卡片出结果     item/completed
//	审批弹卡       execCommandApproval(反向请求)
//	一轮结束       turn/completed
type CodexProvider struct {
	// turnMu 串行化轮次;lifeMu 保护子进程生命周期(两者独立,查模型不阻塞轮次)
	turnMu     sync.Mutex
	lifeMu     sync.Mutex
	client     *codexapp.Client
	inited     bool
	turnActive atomic.Bool
	// 桌面端设置(手机"设置"页写入,thread/start 时带上)
	setMu          sync.Mutex
	approvalPolicy string
	sandbox        string
	desktopModel   string
	desktopEffort  string
	// 行为与降噪(第 27 轮新增)。与上面 4 项同样只作用于本 daemon 拉起的
	// app-server 子进程内存态 —— 不写 config.toml,不影响用户桌面端。
	hideAgentReasoning    bool
	modelVerbosity        string
	modelReasoningSummary string
	disablePasteBurst     bool
	// ProbeHook 是测试注入点:非空时 ReadyCtx 直接调用它,用来确定性地模拟
	// "codex 不可达(超时/报错)"。生产环境恒为 nil。见 server 的无界阻塞回归测试。
	ProbeHook func(ctx context.Context) error
	// 标题同步:codex 自动命名线程时回调(server 层落库,双端可见)
	TitleHandler func(codexThreadID, title string)
	// 目标同步:codex 侧 goal 状态机变化时回调(server 层落库)。
	// goal 为空表示被清除;status 是 active/paused/blocked/usageLimited/budgetLimited/complete。
	GoalHandler func(codexThreadID, goal, status string, tokensUsed, timeUsedSeconds int64)
	// 我们的 threadID → codex threadId + 加载它的子进程代号。真正的事实来源是
	// store.Thread.CodexThreadID;内存缓存只在同一子进程生命周期内有效。
	// 子进程重启(gen 变化)后必须先 thread/resume,否则 turn/start 打到
	// 子进程不认识的 threadId 会被 codex 静默吞掉(实测:无报错、无事件、
	// 永不结束——手机端表现为一直转圈)。
	threadMap map[string]threadRef
	threadMu  sync.Mutex
	gen       atomic.Int32 // codex 子进程代号,每次(重)启 +1
	// sessionCall 原生会话适配器(codex_session.go)的 JSON-RPC 调用源:生产为
	// nil,走 p.client.Call(withClient 先保证子进程就绪);单测与契约套件注入
	// 假调用,不拉真子进程(与 pi 的 sessionFor 钩子同模式,014db16 先例)。
	sessionCall func(method string, params interface{}) (json.RawMessage, error)
	// 常驻目标(codex 的 thread/goal/set)。codex 只认"已被 app-server 加载"的线程,
	// 所以线程还没跑过第一轮(或子进程刚重启还没 resume)时先暂存,等 Run() 拿到
	// codex threadId 后补发。见 codex_goal.go。
	goalMu       sync.Mutex
	pendingGoals map[string]string
	// ---- 会话级事实(由常驻通知钩子维护,见 codexapp.Client.GlobalHandler)----
	// 审查(guardian / 自动审查)结论。用户点"仍然执行"时,我们必须把 codex 发来的
	// 原始结论**原样**还原成它要的 GuardianAssessmentEvent —— 见 codex_guardian.go。
	gMu             sync.Mutex
	guardianReviews map[string]*GuardianReview // reviewId → 结论
	guardianLatest  map[string]string          // threadID → 最近一条"未放行"的 reviewId
	guardianOrder   []string                   // 淘汰顺序(只保留最近若干条)
	// 会话生命周期变化(归档/删除/回滚/改名/队列)。以前这些通知只在任务运行期间
	// 才有人接,任务外到达的会被直接丢掉 —— 于是"电脑上归档了、手机上还是活跃"。
	// 这里回调给 server 层,由它同步本地库。
	ThreadEventHandler func(codexThreadID, method string, params json.RawMessage)

	// 自动压缩状态机(按 codex 线程 id 索引):compactArmed/compactFired 防抖。
	// 仅由 maybeAutoCompact / resetCompactState 在 compactMu 保护下访问。
	compactMu    sync.Mutex
	compactState map[string]compactState

	// 正在等手机应答的反向请求(codex 侧 JSON-RPC id 的归一键 → 解决信号
	// channel,见 codexRequestIDKey)。codex 通过 serverRequest/resolved 通知
	// "该请求已在桌面端被解决"时,我们据此提前结束等待、不再 Respond,并让
	// 手机端撤掉审批卡(幽灵卡治理)。
	pendingReqMu    sync.Mutex
	pendingResolved map[string]chan struct{}

	// ---- P1-4:运行中插话(turn/steer)----
	// 我们的 threadID → 当前运行中轮次的 codex threadId + turnId。
	// 由 turn/started 通知登记(见 Run 的 handleNotif),Run 收尾时清除;
	// steer 时作为 expectedTurnId 传给 codex(schema 的前置条件校验)。
	steerMu     sync.Mutex
	activeSteer map[string]steerTarget

	// ---- 反问卡能力宣告(只读发现,绝不注入) ----
	// 旧版这里曾把实验特性 default_mode_request_user_input 注入 thread/start、
	// thread/resume 的 config 覆盖来"解锁"Default 模式的 request_user_input 工具。
	// 但该特性在 codex 0.155.1 仍是 underDevelopment,注入即触发 codex 的
	// "Under-development features enabled"警告(真机实测每次会话都弹),而官方抑制
	// 途径只有改写用户 ~/.codex/config.toml —— 本项目红线永久封死。
	// 因此现已**不再注入**:能力是否存在改为运行时宣告(askUserInputSupported),
	// 由手机端据其决定是否显示反问入口。这组字段只缓存"codex 是否宣告该特性"
	// 的运行时判定结果(按 gen),不参与任何写入。
	askFeatMu  sync.Mutex
	askFeatGen int
	askFeatVal bool

	// ---- 第 28 轮:三类只读查询的 TTL 缓存(见 capabilities_cache.go)----
	// capabilities(skills/hooks/plugins,30 分钟)/ mcpServers(5 分钟)/
	// usage(60 秒)。命中零 codex 调用;重拉失败回退旧值;InvalidateAll() 全清。
	// fetch 闭包在 NewCodexProvider 里接线;零值结构体(&CodexProvider{})的
	// fetch 为 nil,read 会如实报错而不是 panic。
	capCache   statusCache[[]*pb.CodexCapability]
	mcpCache   statusCache[[]*pb.McpServerStatus]
	usageCache statusCache[*pb.CodexUsage]

	// modelsBudget 是 ModelsStatusCtx(ListModels 的 codex 拉取,PA-REAL-0002)
	// 的整段预算;零值回落 statusBudget(10s)。仅测试注入更短值以钉"预算内返回
	// 可重试错误"的行为,生产恒为零值。
	modelsBudget time.Duration

	// usageProbeBudget 是 account/rateLimits/read + account/usage/read 两个用量
	// 探测(fetchUsageUncached,本轮唯一调用方)的整段预算;零值回落
	// statusBudget(10s)。仅测试注入更短值以钉"探测超时放弃 → 迟到应答静默
	// 丢弃"的行为(与 modelsBudget 同款注入模式),生产恒为零值。
	usageProbeBudget time.Duration
}

// guardianKeep 最多记住多少条审查结论。审查是高频事件(每个被执行的动作一次),
// 只保留最近的足够用:能点"仍然执行"的必然是刚刚发生、还在屏幕上的那一条。
const guardianKeep = 200

type threadRef struct {
	id  string
	gen int
}

func NewCodexProvider() *CodexProvider {
	p := &CodexProvider{
		threadMap:       map[string]threadRef{},
		pendingGoals:    map[string]string{},
		guardianReviews: map[string]*GuardianReview{},
		guardianLatest:  map[string]string{},
		compactState:    map[string]compactState{},
		pendingResolved: map[string]chan struct{}{},
		activeSteer:     map[string]steerTarget{},
	}
	// ---- 第 28 轮:三类只读查询的缓存接线 ----
	// fetch 闭包里 key 对 capabilities 是 cwd(技能按目录算),其余两类恒空串。
	// 预算分级:capabilities(含 plugin/installed)与 mcpServerStatus/list 属重查询,
	// 重拉放宽到 heavyQueryBudget(45s);usage 本就要新鲜,维持 10s 默认预算。
	p.capCache = statusCache[[]*pb.CodexCapability]{
		ttl: capabilitiesTTL,
		fetch: func(cwd string) ([]*pb.CodexCapability, error) {
			return p.fetchCapabilitiesDetailed(cwd, heavyQueryBudget)
		},
	}
	p.mcpCache = statusCache[[]*pb.McpServerStatus]{
		ttl: mcpServersTTL,
		fetch: func(string) ([]*pb.McpServerStatus, error) {
			return p.fetchMcpStatusDetailed(heavyQueryBudget)
		},
	}
	p.usageCache = statusCache[*pb.CodexUsage]{
		ttl: usageTTL,
		fetch: func(string) (*pb.CodexUsage, error) {
			out := p.fetchUsageUncached()
			if out.Error != "" {
				// 拉取失败:把错误同时走 error 通道(缓存据此回退旧值/透传),
				// out 本身仍带 Error 字段返回给无缓存时的调用方。
				return out, fmt.Errorf("%s", out.Error)
			}
			return out, nil
		},
	}
	return p
}

// codexRequestIDKey 把反向请求 id(json.RawMessage)归一成登记/台账用的字符串键:
// 数字形态转十进制字符串(与旧 int64 时代的台账口径逐字节一致)、字符串形态去
// 引号原样、其余形态用原文。同一 id 的两条来路 —— 反向请求帧的 id 与
// serverRequest/resolved 通知的 requestId —— 在任一形态下都归到同一个键,
// resolved 竞速守卫与未知事件台账因此对双形态 id 都成立。
func codexRequestIDKey(raw json.RawMessage) string {
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return strconv.FormatInt(n, 10)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

// registerPendingServerRequest 登记"正在等手机应答"的反向请求,
// 返回一个在该请求被 codex 侧解决时关闭的 channel(见 serverRequest/resolved)。
func (p *CodexProvider) registerPendingServerRequest(id json.RawMessage) <-chan struct{} {
	ch := make(chan struct{})
	p.pendingReqMu.Lock()
	p.pendingResolved[codexRequestIDKey(id)] = ch
	p.pendingReqMu.Unlock()
	return ch
}

// unregisterPendingServerRequest 正常结束(拿到手机应答)时清理登记,防 map 泄漏。
func (p *CodexProvider) unregisterPendingServerRequest(id json.RawMessage) {
	p.pendingReqMu.Lock()
	delete(p.pendingResolved, codexRequestIDKey(id))
	p.pendingReqMu.Unlock()
}

// resolvePendingServerRequest codex 通知某反向请求已被解决:关闭对应信号。
// 通知里的 requestId 与当初反向请求帧的 id 同形,经同一归一(见
// codexRequestIDKey)必然命中同一登记项。
func (p *CodexProvider) resolvePendingServerRequest(rawID json.RawMessage) {
	key := codexRequestIDKey(rawID)
	if key == "" {
		return
	}
	p.pendingReqMu.Lock()
	if ch, ok := p.pendingResolved[key]; ok {
		delete(p.pendingResolved, key)
		close(ch)
	}
	p.pendingReqMu.Unlock()
}

func (p *CodexProvider) ID() string          { return "codex" }
func (p *CodexProvider) DisplayName() string { return "OpenAI Codex(桌面 agent)" }

func (p *CodexProvider) Models() []pb.ModelInfo {
	ms, _ := p.ModelsStatusCtx(context.Background())
	return ms
}

// ModelsCtx 是带 ctx 的模型列表:失败时仍返回含 default 的列表、不向上传错误
// (设置页账号读取 AccountCtx 等既有调用方靠这个语义工作)。内部改走
// ModelsStatusCtx 的状态预算通道(PA-REAL-0002:整段有界,不再随调用方截止
// 时间无限等 initialize —— 那正是"首次 ListModels 吃满 30s 客户端超时"的形态)。
func (p *CodexProvider) ModelsCtx(ctx context.Context) []pb.ModelInfo {
	ms, _ := p.ModelsStatusCtx(ctx)
	return ms
}

// ErrModelsInitializing 表示"模型列表这一刻取不到:codex 正在初始化/探测,
// 或本条查询的预算用完了"。它是**可重试态**而不是终态失败 —— 几秒后重试通常
// 就能拿到完整列表。文案即人话(AUDIT §3.3):手机端对错误文本透传显示,
// 所以这句话直接面向用户,不再是裸 DeadlineExceeded。
var ErrModelsInitializing = errors.New("模型列表初始化中,请稍后重试")

// ModelsStatusCtx 是模型列表的"状态预算通道"入口(ListModels 专用,PA-REAL-0002):
//
//   - 整段受预算约束:默认 statusBudget(10s);调用方给过更紧的截止时间时取
//     更早者(探测路径/测试不被拖慢)。预算内拿不到就返回 ErrModelsInitializing。
//   - 走 withStatusClientBudget:lifeMu 只保护"确保已初始化"那一小段,
//     model/list 本身与其它只读查询一样在锁外并发执行,不再把别的 codex 调用
//     挡成 ErrBusy。
//   - 返回值 (列表, err):err 非空时列表仍含真实的 "default"(跟随桌面)条目
//     —— 绝不返回空列表冒充成功(空列表会让手机端模型抽屉"看起来正常但选不了"),
//     也不把失败伪装成"只有 default、一切正常";server 层把 err 透传给手机。
func (p *CodexProvider) ModelsStatusCtx(ctx context.Context) ([]pb.ModelInfo, error) {
	out := []pb.ModelInfo{{
		Id: "default", Provider: "codex", DisplayName: "Codex(跟随桌面配置)",
		// Codex app-server 的真实窗口只在 thread/tokenUsage/updated 中上报。
		// 模型列表没有可靠的窗口字段时保持 0，客户端必须显示“未知”，
		// 不能把历史经验值伪装成当前模型窗口。
		ContextWindow: 0, Available: true,
	}}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return out, modelsListErr(err)
	}
	budget := p.modelsBudget
	if budget <= 0 {
		budget = statusBudget
	}
	// withStatusClientBudget 从 Background 起算自己的超时,这里先把预算压到
	// 不晚于调用方的截止时间;调用方截止已过 → 立即按超时处理,绝不反向放大。
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d <= 0 {
			return out, modelsListErr(context.DeadlineExceeded)
		} else if d < budget {
			budget = d
		}
	}
	err := p.withStatusClientBudget(budget, func(ctx context.Context, c *codexapp.Client) error {
		raw, err := c.CallCtx(ctx, "model/list", map[string]interface{}{})
		if err != nil {
			return err
		}
		var res struct {
			Data []struct {
				Id                        string   `json:"id"`
				DisplayName               string   `json:"displayName"`
				Description               string   `json:"description"`
				InputModalities           []string `json:"inputModalities"`
				SupportedReasoningEfforts []struct {
					ReasoningEffort string `json:"reasoningEffort"`
				} `json:"supportedReasoningEfforts"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &res) != nil {
			return nil
		}
		for _, m := range res.Data {
			// 直接往切片槽位填字段,避免 protobuf 消息按值 append 拷贝锁(内嵌 MessageState)。
			out = append(out, pb.ModelInfo{})
			info := &out[len(out)-1]
			info.Id, info.Provider, info.Available = m.Id, "codex", true
			for _, effort := range m.SupportedReasoningEfforts {
				info.AvailableEfforts = append(info.AvailableEfforts, effort.ReasoningEffort)
			}
			info.SupportsThinking = len(info.AvailableEfforts) > 0
			info.DisplayName = m.DisplayName
			if m.DisplayName == "" {
				info.DisplayName = m.Id
			}
			for _, mod := range m.InputModalities {
				if mod == "image" {
					info.SupportsImages = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return out, modelsListErr(err)
	}
	return out, nil
}

// modelsListErr 把模型列表拉取失败归类成手机端可直接显示的形态(PA-REAL-0002):
// 初始化/探测占用(ErrBusy)或预算耗尽(DeadlineExceeded/Canceled)→ 可重试态
// ErrModelsInitializing;两个错误都进错误链(多重 %w)—— 上层用 errors.Is 对
// ErrBusy/超时做的"可重试"分级不受影响,原始原因也可归因。其余错误**原样透传**
// —— 那是 codex 的原生原文(如登录/token 问题),按 README §7 如实显示,不加工。
func modelsListErr(err error) error {
	if errors.Is(err, ErrBusy) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		return fmt.Errorf("%w(底层原因: %w)", ErrModelsInitializing, err)
	}
	return err
}

// clientInitBudget 是 initialize 握手的默认上界。
//
// 【为什么专门给它一个上界】子进程"起来了但握手不完成"是最坏情况:进程在、
// initialize 却永远不回(本机实测:token 刷新被 403 地区限制,刷屏
// "Failed to refresh token")。它发生在本函数内、**全程持有 lifeMu**,
// 于是所有模型服务调用(设置页读写、模型列表、MCP 详情)一起排队等它 ——
// 这就是"该界面所有设置项都无法实际生效"的真凶:保存请求永远不返回。
const clientInitBudget = 8 * time.Second

// withClient 在保证子进程活着的前提下执行 fn;失败自动重启一次。
//
// 【无 ctx 的便捷入口】fn 内部的 codex 调用用 c.Call(受 callTimeout=120s 兜底)。
// 只想把它当"通用的、有界的"执行器时用它。需要把整段操作压进更短上界(探测路径)
// 时用 withClientCtx。
//
// 【重要:只读状态查询请用 withStatusClient】withClient 的 fn 走 120s 兜底,
// codex 卡死(如 token 刷新 403 死循环)时会**全程持有 lifeMu 长达 120 秒**,
// 期间所有其它调用(设置页、MCP、同步、会话列表)全部 ErrBusy —— 实测这就是
// "界面显示连接成功却莫名各种失败"的根源(2026-09-18 daemon.log 连环正忙)。
func (p *CodexProvider) withClient(fn func(*codexapp.Client) error) error {
	return p.withClientCtx(context.Background(), clientInitBudget, func(_ context.Context, c *codexapp.Client) error {
		return fn(c)
	})
}

// statusBudget 是只读状态查询(用量/能力/MCP/线程列表等)的整段上界。
// 10s 足够正常应答,又能在 codex 卡死时快速释放 lifeMu,避免连环 ErrBusy。
const statusBudget = 10 * time.Second

// withStatusClient 是**只读状态查询**专用入口。生命周期锁只保护“确保
// app-server 已初始化”这一小段;初始化完成后 fn 不再持有 lifeMu,因此 MCP、
// skills、hooks、plugins、用量等查询可以在同一客户端上并发执行。每个调用仍
// 共用自己的 statusBudget,不会因某个查询卡住而把其它只读通道锁住。
//
// 【第 28 轮】它只是 withStatusClientBudget(statusBudget, fn) 的默认上界便捷入口;
// 重查询(如 plugin/installed、mcpServerStatus/list)在缓存重拉时用更宽的预算,
// 见 withStatusClientBudget 与 capabilities_cache.go。
func (p *CodexProvider) withStatusClient(fn func(context.Context, *codexapp.Client) error) error {
	return p.withStatusClientBudget(statusBudget, fn)
}

// withStatusClientBudget 是带**自定义预算**的只读状态查询入口,行为与
// withStatusClient 完全一致,只是整段上界由调用方给定(d<=0 时回落到 10s 默认)。
//
// 【为什么需要它】"暂时读不到能力列表"的根因是:所有只读查询统一 10s 预算,
// 而 codex 子进程繁忙时 plugin/installed、mcpServerStatus/list 经常 >10s ——
// 手机端整页红字。配合 TTL 缓存(capabilities_cache.go)后,重拉只发生在
// 冷启动/过期瞬间,慢一次可接受,之后 30 分钟内秒回;所以重查询的重拉预算
// 放宽到 45s,普通查询维持 10s 快速失败。
func (p *CodexProvider) withStatusClientBudget(d time.Duration, fn func(context.Context, *codexapp.Client) error) error {
	if d <= 0 {
		d = statusBudget
	}
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	// 页面初始化可能同时发起多个只读请求。只让第一个请求负责初始化，
	// 其余请求短暂等待并复用已初始化客户端，避免把正常并发误报成失败。
	for {
		err := p.withClientCtx(ctx, clientInitBudget, func(context.Context, *codexapp.Client) error {
			return nil
		})
		if err == nil {
			break
		}
		if !errors.Is(err, ErrBusy) {
			return err
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	// withClientCtx 已经确保 client 存在且完成 initialize;后续只读调用
	// 不会触发重启,所以不会在 fn 执行期间关闭这个客户端。
	return fn(ctx, p.client)
}

// withClientCtx 是带 ctx 的版本:fn 里的 codex 调用请用 c.CallCtx(ctx, ...),
// 这样"initialize 握手 + fn 整段"都受同一个 ctx 截止时间约束。
//
// 【第 3 轮:给无界阻塞上界 —— 这三点缺一不可】
//  1. TryLock 替代 Lock:拿不到锁说明另一个调用正在启动/探测 codex,
//     立即返回 ErrBusy,而不是排队 —— 一个卡死的探测不该把后续所有请求一起拖住;
//  2. initialize 用 CallCtx 加超时:它是唯一会无限阻塞的点;
//  3. 调用方没给截止时间时,套一个默认上界(budget),无论如何都有界。
//
// 【绝不"超时后用 go 继续等"】那会留下一个还持着 lifeMu 的僵尸 goroutine,
// 正是本 bug 的形态。这里全程同步:CallCtx 到期即返回,锁随之释放。
func (p *CodexProvider) withClientCtx(ctx context.Context, budget time.Duration, fn func(context.Context, *codexapp.Client) error) error {
	if !p.lifeMu.TryLock() {
		return ErrBusy
	}
	defer p.lifeMu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	// 调用方已用 WithTimeout 给了截止时间就沿用;否则套一层默认上界保底。
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		if budget <= 0 {
			budget = clientInitBudget
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, budget)
		defer cancel()
	}

	try := func() error {
		if p.client == nil {
			return errNoClient
		}
		if !p.inited {
			if _, err := p.client.CallCtx(ctx, "initialize", map[string]interface{}{
				"clientInfo": map[string]string{"name": "pocketagent", "title": "PocketAgent", "version": version.App},
			}); err != nil {
				return err
			}
			p.inited = true
		}
		return fn(ctx, p.client)
	}
	err := try()
	// 超时/取消不是"子进程挂了,重启能救"的情况:重启只会再超时并刷屏。
	// 直接如实返回,交给调用方降级(设置页会把它翻译成 unreachable)。
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	// 正在执行 turn 时，状态查询失败不能重启共享 app-server。
	// Run() 持有的是 turnMu，不会持有 lifeMu；如果这里重启，正在跑的
	// turn 会被强行切断，手机只能收到半截流。
	if err != nil && err != errNoClient && p.turnActive.Load() {
		return err
	}
	if err == nil || err == errNoClient {
		if err == errNoClient {
			// 首次拉起
			c, serr := codexapp.Start()
			if serr != nil {
				return serr
			}
			p.client, p.inited = c, false
			p.attachGlobalHandler(c)
			p.gen.Add(1)
			err = try()
		}
		return err
	}
	// 子进程可能挂了:杀掉重来一次
	if p.client != nil {
		p.client.Close()
	}
	p.client, p.inited = nil, false
	c, serr := codexapp.Start()
	if serr != nil {
		return serr
	}
	p.client, p.inited = c, false
	p.attachGlobalHandler(c)
	p.gen.Add(1)
	log.Printf("[codex] app-server 子进程已重启(gen=%d),所有线程缓存作废", p.gen.Load())
	return try()
}

// attachGlobalHandler 给新拉起的子进程装上**常驻**通知钩子。
//
// 【为什么单独一层】codexapp.Client.NotifHandler 是给"当轮任务"用的、随 Run 装拆;
// 而审查结论、会话归档/删除/回滚、技能变化这些事实在任务之外也会到达。
// 不挂常驻钩子的话,这些通知就是发给了空气 —— 表现是手机上永远看不到"审查者驳回了",
// 以及电脑上删掉的会话在手机上还活着。
//
// 这里只做**廉价记录**(加锁写 map / 回调),绝不碰 sink 与手机网络:
// 它在 readLoop 里被同步调用,一旦阻塞,整条事件管道都会卡住。
func (p *CodexProvider) attachGlobalHandler(c *codexapp.Client) {
	c.SetUnknownResponseHandler(func(id json.RawMessage, raw json.RawMessage) {
		method, kind := codexLateResponseLedger(raw)
		// id 双形态直透后的台账口径:数字转十进制串(与旧 int64 时代逐字节
		// 一致),字符串形态原样 —— 台账里首次能看到字符串 id 的真实值
		// (旧实现此处恒记 0 占位)。
		recordUnknownResponseAs(kind, "codex", method, codexRequestIDKey(id), raw)
	})
	c.SetGlobalHandler(func(method string, params json.RawMessage) {
		recordProviderEvent("codex", method, codexThreadIDOf(params), codexGlobalNoticeKind(method), params)
		switch {
		case strings.HasPrefix(method, "item/autoApprovalReview/"),
			method == "autoApprovalReview/strictReviewRequired", method == "guardianWarning":
			// 审查结论:记下来,供"仍然执行"还原 + 供手机端展示。
			p.recordGuardianNotification(method, params)
		case method == "thread/archived", method == "thread/unarchived",
			method == "thread/deleted", method == "thread/reverted", method == "thread/closed",
			method == "thread/name/updated", method == "thread/queue/changed",
			method == "thread/status/changed", method == "thread/started",
			method == "skills/changed":
			// 会话级事实的变化:交给 server 层同步本地库(异步,别拖住 readLoop)。
			if h := p.ThreadEventHandler; h != nil {
				p.ThreadEventHandlerAsync(h, method, params)
			}
		case method == "thread/goal/updated", method == "thread/goal/cleared":
			// 目标状态机的轮外变化(桌面端设/清目标、codex 自己推进状态):与 Run 的
			// 轮内分支走同一条 GoalHandler 通路。以前全局路径不认这两条 —— 任务之外
			// 到达时只进台账(PROJECTED),本地库永远停在设下去那一刻的旧状态。
			// 异步同上:GoalHandler 会落库,绝不拖住 readLoop。
			if p.GoalHandler != nil {
				tid := codexThreadIDOf(params)
				if tid != "" {
					goalText, status, tokensUsed, timeUsed := goalStatusFromNotification(params)
					if method == "thread/goal/cleared" {
						goalText = ""
					}
					go p.GoalHandler(tid, goalText, status, tokensUsed, timeUsed)
				}
			}
		}
	})
}

// codexGlobalNoticeKinds 是 codex 通知方法的**全局登记表**(唯一一张:全局钩子
// 路径与轮内兜底路径共用,见 codexMethodNoticeKind)。表驱动是为了让 schema 对表
// 测试(codex_notice_schema_20261002_test.go)能遍历全部登记项:每个登记名都必须
// 在 vendored 0.159.2 schema 的 ServerNotification oneOf 里真实存在 —— 幻影方法
// (如审计时误写的 thread/plan/updated,codexplanprobe 实测定案)当场被反例拦下;
// schema 里出现而表里没有的方法,测试也要求逐个落在"明确不收清单"里,两向都钉。
//
// 分类依据 = "Run 路径/事件翻译层对它做了什么"(PROJECTED=确有投影帧,
// PASSTHROUGH=已知但只留痕),与 knowledge_codex.go 词表一一对应。
var codexGlobalNoticeKinds = map[string]string{
	// ---- 会话级事实族(server 层经 ThreadEventHandler 同步本地库)----
	// turn/plan/updated 是真实通知名(vendored schema + 0.156.1 二进制命中;
	// 审计时误写的 thread/plan/updated 已由 codexplanprobe 实测定案为幻影 ——
	// 证据 docs/verify/evidence/codex-plan-probe.log)。事件流投影在
	// codex_events.go 的 turn/plan/updated 分支,带 plan_steps。
	"thread/archived":       runtime.KindKnownProjected,
	"thread/unarchived":     runtime.KindKnownProjected,
	"thread/deleted":        runtime.KindKnownProjected,
	"thread/reverted":       runtime.KindKnownProjected,
	"thread/closed":         runtime.KindKnownProjected,
	"thread/name/updated":   runtime.KindKnownProjected,
	"thread/queue/changed":  runtime.KindKnownProjected,
	"thread/status/changed": runtime.KindKnownProjected,
	"thread/started":        runtime.KindKnownProjected,
	"skills/changed":        runtime.KindKnownProjected,
	"turn/plan/updated":     runtime.KindKnownProjected,
	"thread/goal/updated":   runtime.KindKnownProjected,
	"thread/goal/cleared":   runtime.KindKnownProjected,
	// 0.159.2 补登:命令在等终端输入 —— 事件流有投影(codex_events.go 的
	// terminalInteraction 分支,带指引文案),此前漏登记被当 UNKNOWN 计数;
	// 0.159.2 起它还常有配套的 writeStdin 审批卡(codex_requests.go)。
	"item/commandExecution/terminalInteraction": runtime.KindKnownProjected,
	// 2026-10-01 通宵采样批(codexliveturnprobe 活体单轮实证,0.159.2):
	// 轮次骨架通知族。codexapp 对每条通知都是"先 global 后 notif"两路分发,
	// 本表管的是全局那一路 —— 轮次内 Run 路径明明有投影分支,全局路径却不
	// 认识这些方法,一次最小轮(46 条通知)就有 ~39 条灌成 UNKNOWN
	// (zcode session/event 收编批同一病根)。逐条判定依据 = Run 路径对它做了什么:
	//   - warning → WARNING 状态卡(codex_events.go warning 分支);
	//   - item/started|completed → 工具卡 STARTED/FINISHED、压缩卡、终稿聚合
	//     (codex.go Run 的 item/started|completed 分支);
	//   - item/agentMessage/delta → 流式正文主来源(→ AssistantDelta);
	//   - item/reasoning/summaryTextDelta → 思考流(→ ThinkingDelta,随
	//     HideReasoning 过滤);
	//   - item/commandExecution/outputDelta → 工具卡 UPDATE(codex_events.go);
	//   - thread/tokenUsage/updated → 用量帧 + Token 用量卡;
	//   - account/rateLimits/updated → RATE_LIMIT 用量卡;
	//   - mcpServer/startupStatus/updated → MCP 服务器状态卡;
	//   - turn/completed → 轮次终局驱动(Run 收口 + 终稿冲洗)。
	"warning":                           runtime.KindKnownProjected,
	"turn/completed":                    runtime.KindKnownProjected,
	"item/started":                      runtime.KindKnownProjected,
	"item/completed":                    runtime.KindKnownProjected,
	"item/agentMessage/delta":           runtime.KindKnownProjected,
	"item/reasoning/summaryTextDelta":   runtime.KindKnownProjected,
	"item/commandExecution/outputDelta": runtime.KindKnownProjected,
	"thread/tokenUsage/updated":         runtime.KindKnownProjected,
	"account/rateLimits/updated":        runtime.KindKnownProjected,
	"mcpServer/startupStatus/updated":   runtime.KindKnownProjected,

	// ---- 2026-10-02 深接入差距修复批(证据:vendored ServerNotification.json
	// 逐名 diff + daemon 事件翻译层逐名核对,活体轮探针日志
	// docs/verify/evidence/codex-01592-liveturn-20261002.log)----
	// 这 16 个方法在 0.159.2 schema 里真实存在、daemon 事件翻译层早有投影分支,
	// 但本表此前漏登记:轮次外到达(全局钩子)一律灌 UNKNOWN,与"投影明明在"的
	// 事实不符。逐条投影落点:
	//   - error → ERROR/RETRY_STARTED 事件 + lastErr(codex.go Run error 分支,
	//     codex_events.go parseCodexErrorNotification);
	//   - guardianWarning → WARNING/GUARDIAN_REVIEW 卡(codex_events.go 与
	//     codex_events_ext.go 双分支;全局钩子还记 guardian 结论);
	//   - item/autoApprovalReview/started|completed、
	//     autoApprovalReview/strictReviewRequired → GUARDIAN_REVIEW 卡
	//     (codex_events_ext.go;全局钩子 recordGuardianNotification);
	//   - item/plan/delta → "执行计划生成中"状态卡(codex_events.go);
	//   - command/exec/outputDelta、process/outputDelta → 命令输出卡(base64
	//     解码,codex_events.go;两者是 0.155 旧协议名,保留登记兼容旧二进制);
	//   - item/fileChange/outputDelta → "命令执行过程"卡;
	//   - item/fileChange/patchUpdated → "文件变更预览"卡 + 审批卡文件列表
	//     累积(Run 的 changeFiles);
	//   - item/mcpToolCall/progress → "MCP 工具进度"卡;
	//   - model/rerouted → MODEL_REROUTED 切换卡;
	//   - model/safetyBuffering/updated → MODEL_SAFETY 卡(codex_events_ext.go);
	//   - thread/compacted → CONTEXT_COMPACTED 卡 + 压缩状态机重置
	//     (codex_events.go + codex_compact.go);
	//   - item/reasoning/textDelta → ThinkingDelta(isReasoningDeltaMethod 的
	//     兼容名;summaryTextDelta 之外的旧式思考流载体);
	//   - windows/worldWritableWarning → SANDBOX_WARNING 卡(codex_events_ext.go)。
	//     【与词表"明确不收清单"的矛盾就此修正】该清单此前把它列为不收,但
	//     codex_events_ext.go 自始就有投影分支 —— 代码在先、分类随后,按
	//     "判定依据是我们做了什么"的口径收编为 PROJECTED(词表同批补词条)。
	"error":                                   runtime.KindKnownProjected,
	"guardianWarning":                         runtime.KindKnownProjected,
	"item/autoApprovalReview/started":         runtime.KindKnownProjected,
	"item/autoApprovalReview/completed":       runtime.KindKnownProjected,
	"autoApprovalReview/strictReviewRequired": runtime.KindKnownProjected,
	"item/plan/delta":                         runtime.KindKnownProjected,
	"command/exec/outputDelta":                runtime.KindKnownProjected,
	"process/outputDelta":                     runtime.KindKnownProjected,
	"item/fileChange/outputDelta":             runtime.KindKnownProjected,
	"item/fileChange/patchUpdated":            runtime.KindKnownProjected,
	"item/mcpToolCall/progress":               runtime.KindKnownProjected,
	"model/rerouted":                          runtime.KindKnownProjected,
	"model/safetyBuffering/updated":           runtime.KindKnownProjected,
	"thread/compacted":                        runtime.KindKnownProjected,
	"item/reasoning/textDelta":                runtime.KindKnownProjected,
	"windows/worldWritableWarning":            runtime.KindKnownProjected,

	// ---- 已知但无投影,只留痕(PASSTHROUGH)----
	"turn/diff/updated":  runtime.KindKnownPassthrough,
	"model/verification": runtime.KindKnownPassthrough,
	// 0.156.1 收编批(docs/verify/evidence/codex-0156-probe.log;词表在
	// runtime/knowledge_codex.go):environment 接通/掉线、会话设置被
	// 桌面端改动、配置警告/弃用提示、账号与 MCP 授权变更、后台进程退出、
	// 0.156 生命周期钩子,以及思考摘要分段(思考流投影已有
	// item/reasoning/summaryTextDelta,本条只补"新一段开始"的留痕)。
	"thread/environment/connected":    runtime.KindKnownPassthrough,
	"thread/environment/disconnected": runtime.KindKnownPassthrough,
	// 2026-10-02 收编(unknown-events.json 滚动窗 200 条复盘):Codex 桌面端远程
	// 控制开关启停通知,载荷 {status: enabled|disabled, serverName}。与
	// environment/connected|disconnected 同性质(状态变化+人话可译),留痕+词表。
	// 其余 remoteControl/* 变体仍属明确不收清单,出现即如实 UNKNOWN 留观。
	"remoteControl/status/changed":    runtime.KindKnownPassthrough,
	"thread/settings/updated":         runtime.KindKnownPassthrough,
	"configWarning":                   runtime.KindKnownPassthrough,
	"deprecationNotice":               runtime.KindKnownPassthrough,
	"item/reasoning/summaryPartAdded": runtime.KindKnownPassthrough,
	"turn/moderationMetadata":         runtime.KindKnownPassthrough,
	"account/updated":                 runtime.KindKnownPassthrough,
	"account/login/completed":         runtime.KindKnownPassthrough,
	"mcpServer/oauthLogin/completed":  runtime.KindKnownPassthrough,
	"process/exited":                  runtime.KindKnownPassthrough,
	"hook/started":                    runtime.KindKnownPassthrough,
	"hook/completed":                  runtime.KindKnownPassthrough,
	// 0.159.2 收编批(generate-json-schema diff + 活体探针,2026-09-30):
	// 模型服务登录态自动恢复两连 —— 它解释"轮次莫名失败",属关键通知。
	// 【2026-10-07 字段级覆盖审计升级】从 PASSTHROUGH 升为 PROJECTED:载荷
	// {message, provider, threadId, turnId} 正是"错误"维度的人话载体,
	// codex_events_ext.go 自本批起投影成 WARNING 卡,不再只留痕。
	"modelProvider/authRecoveryStarted":   runtime.KindKnownProjected,
	"modelProvider/authRecoveryCompleted": runtime.KindKnownProjected,
	// 2026-10-01 通宵采样批(同上活体单轮):已知但无投影,只留痕 ——
	//   - turn/started → Run 路径只做日志 + steer 目标登记,不发协议帧;
	//   - serverRequest/resolved → 桌面端处理完反向请求的回执,手机端无消费方。
	"turn/started":           runtime.KindKnownPassthrough,
	"serverRequest/resolved": runtime.KindKnownPassthrough,
}

// codexGlobalNoticeKind 查全局登记表;表外一律 UNKNOWN_NOTIFICATION(不猜)。
func codexGlobalNoticeKind(method string) string {
	if kind, ok := codexGlobalNoticeKinds[method]; ok {
		return kind
	}
	return runtime.KindUnknownNotification
}

// codexLateResponseLedger 判定一条"无法关联到在途请求"的 codex 响应如何入账,
// 返回(台账 SourceMethod, normalized_kind)。
//
// 【积压从哪来】daemon 的能力探测/用量面板会发 account/rateLimits/read 与
// account/usage/read(codex_capabilities.go,本轮唯一调用方);CallCtx 的 ctx 或
// 120s 兜底先到期时 pending 已删,app-server 随后把上游失败补发成 error 响应,
// 无人认领 → UnknownResponseHandler。调用方早已按超时降级(用量面板如实显示
// "读不到"),这条迟到应答是纯冗余证据 —— 台账 200 条滚动窗实测积压 24 条
// UNKNOWN_RESPONSE,全部落在两个可实证的消息家族上。
//
// 【2026-10-01 源头治理后的定位】这两个探测已改走 CallProbeCtx(按调用代号在
// codexapp dispatch 处静默丢弃迟到应答),本家族不再新增;本分类器保留作兜底 ——
// 极端陈旧(超过 abandonedProbeTTL)才到达的应答、以及非探测调用产生的孤儿帧,
// 仍按既有口径留痕,不静默隐身。
//
// 【判定标准是消息原文,不是 id 猜测】响应体里只有 error.code/-32603 与
// error.message,按 message 精确短语归入两个已知家族;登记名带上对应的请求方法
// (dsh "session/update:<kind>" / zcode "session/event:<type>" 的同款后缀先例),
// 词表(knowledge_codex.go)据此给"已知迟到应答"人话,真未知的仍走裸
// "response" + UNKNOWN_RESPONSE,两类在诊断页一眼可分。
func codexLateResponseLedger(raw json.RawMessage) (string, string) {
	var resp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &resp) != nil || resp.Error == nil {
		return "response", runtime.KindUnknownResponse
	}
	switch {
	case strings.Contains(resp.Error.Message, "failed to fetch codex rate limits"):
		return "response:account/rateLimits/read", runtime.KindKnownPassthrough
	case strings.Contains(resp.Error.Message, "token usage profile fetch timed out"):
		return "response:account/usage/read", runtime.KindKnownPassthrough
	}
	return "response", runtime.KindUnknownResponse
}

// ThreadEventHandlerAsync 在独立 goroutine 里回调 server 层。
//
// 【为什么必须异步】这段代码跑在 codexapp 的 readLoop 里 —— 那是**唯一**一条
// 读子进程输出的 goroutine。回调里只要做任何可能阻塞的事(写文件、打日志、
// 访问手机),整条事件管道就停了:表现是"手机上半天不出字,然后一次性全出来"。
// params 在这里立即排空成独立副本 —— readLoop 之后不会再动它,但复制一份
// 成本极低(几百字节),换来"回调方想留多久留多久"的自由。
func (p *CodexProvider) ThreadEventHandlerAsync(h func(codexThreadID, method string, params json.RawMessage), method string, params json.RawMessage) {
	cp := make([]byte, len(params))
	copy(cp, params)
	tid := codexThreadIDOf(params)
	go h(tid, method, cp)
}

// codexThreadIDOf 从通知参数里取 threadId(取不到就返回空,由 server 层忽略)。
// 【T-S1P】绝大多数通知的 threadId 在顶层;唯一例外是 thread/started —— 它带
// 完整 ThreadInfo 对象,线程 id 在 thread.id(S0 实测帧,
// docs/verify/evidence/codex-subagent-s0.md)。顶层缺 threadId 时回退读
// thread.id,否则"会话级事实同步"管线对 thread/started 永远拿到空 id 而整帧
// 丢弃,子代理线程的存根登记(server 层)也就无从谈起。只做回退:顶层有
// threadId 的既有通知行为一字不变。
func codexThreadIDOf(params json.RawMessage) string {
	var n struct {
		ThreadId string `json:"threadId"`
		Thread   struct {
			Id string `json:"id"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(params, &n)
	if n.ThreadId != "" {
		return n.ThreadId
	}
	return n.Thread.Id
}

var errNoClient = fmt.Errorf("codex 子进程未启动")

// DeleteCodexThread 走 app-server 协议把一条线程从 codex 侧真删(thread/delete)。
//
// 【为什么有了 DeleteCodexSession(直接删 rollout 文件)还要它】文件删除只抽掉
// 落盘痕迹;app-server/桌面端还握着线程索引时,thread/list 仍可能把它当活会话
// 报回来。thread/delete 是协议层的权威删除,codex 删完还会广播 thread/deleted,
// 双端事实自然一致。0.156.1 实测仍支持 thread/delete(运行时自报请求面含此
// 方法,docs/verify/evidence/codex-0156-probe.log 第四节)。
//
// 【失败语义】调用方(server 层删除路径)必须把它当"尽力而为":失败只留日志,
// 不阻塞本地删除与墓碑 —— 墓碑才是防复活的权威(见 server.DeleteThread)。
func (p *CodexProvider) DeleteCodexThread(codexThreadID string) error {
	if codexThreadID == "" {
		return fmt.Errorf("缺少 codex 线程 id")
	}
	return p.withClient(func(c *codexapp.Client) error {
		_, err := c.Call("thread/delete", map[string]interface{}{"threadId": codexThreadID})
		return err
	})
}

// ErrBusy 表示另一个调用正持有 codex 生命周期锁(正在初始化/探测)。
//
// 【为什么单列而不是排队等锁】排队等锁会把"一次卡死"放大成"全线卡死":
// 设置页保存、模型列表、MCP 详情会一个接一个地排在同一个锁上。拿到 ErrBusy 的
// 调用方应降级为"暂时不可用,请稍后重试",这是诚实的 —— 比默默等几十秒强。
// 导出它,以便 server 层用 errors.Is 区分"正忙"与"真失败"。
var ErrBusy = fmt.Errorf("codex 正忙(另一个操作正在初始化/探测),请稍后重试")

// ---- 通知/请求的 JSON 形状(只解析用得到的字段)----

type deltaNotif struct {
	ThreadId string `json:"threadId"`
	Delta    string `json:"delta"`
}
type itemNotif struct {
	ThreadId string `json:"threadId"`
	Item     struct {
		Type    string   `json:"type"`
		Id      string   `json:"id"`
		Text    string   `json:"text"` // 新版 agentMessage 直接带 text
		Command []string `json:"command"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		AggregatedOutput string `json:"aggregatedOutput"`
		Status           string `json:"status"`
		// ExitCode 是 CommandExecutionThreadItem 的退出码(可缺失);非零时补进
		// 工具卡结果,用户不用点开原始输出也能看出命令没跑成。
		ExitCode *int64 `json:"exitCode"`
	} `json:"item"`
}

// isOpaqueItemType 判断是否"没有专案处理分支"的 item 类型(item/started 与
// item/completed 共用):已知五类各有专门翻译/提取分支,其余(userMessage/
// reasoning 等 schema 可能新增的类型)按不透明数据走主线 A 透传。
func isOpaqueItemType(t string) bool {
	switch t {
	case "contextCompaction", "commandExecution", "agentMessage", "subAgentActivity", "collabAgentToolCall":
		return false
	}
	return true
}

// maybeRecordOpaqueItem 把 item/started|item/completed 内的未知 item 类型接进
// 主线 A 的 notices.record"原样包装"透传(native_notifications.go:限频与脱敏
// 自动生效),命中并透传后返回 true。仅透传原始 params,不伪造、不改写;
// 已知类型(各有专案分支)、异线程、坏 JSON 一律返回 false,保持既有行为。
// 限频键 = 方法 + item 类型,不同类型互不挤占。
// 分类结果经 codexItemNoticeKind 落进台账的 normalized_kind —— reasoning /
// userMessage / summary 是已知类型,不得冒充 UNKNOWN_NOTIFICATION(方案 §9.4)。
func maybeRecordOpaqueItem(method string, params json.RawMessage, codexTID string, notices *nativeNoticeRecorder, sink Sink) bool {
	var n itemNotif
	if json.Unmarshal(params, &n) != nil || n.ThreadId != codexTID || !isOpaqueItemType(n.Item.Type) {
		return false
	}
	notices.recordAs(codexItemNoticeKind(n.Item.Type), "codex", method+":"+n.Item.Type, params, sink)
	return true
}

// agentMessageTextFromItem 从 Codex v2 ThreadItem 提取最终正文。
// agentMessage 的稳定字段是 type/id/text；保留 content 兼容旧版响应。
func agentMessageTextFromItem(raw json.RawMessage) string {
	var item struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &item) != nil || item.Type != "agentMessage" {
		return ""
	}
	var text strings.Builder
	text.WriteString(item.Text)
	for _, part := range item.Content {
		if part.Type == "text" {
			text.WriteString(part.Text)
		}
	}
	return text.String()
}

func agentMessageTextFromNotification(raw json.RawMessage) string {
	var n struct {
		Item json.RawMessage `json:"item"`
	}
	if json.Unmarshal(raw, &n) != nil {
		return ""
	}
	return agentMessageTextFromItem(n.Item)
}

func agentMessageTextsFromTurn(raw json.RawMessage) string {
	var n struct {
		Turn struct {
			Items []json.RawMessage `json:"items"`
		} `json:"turn"`
	}
	if json.Unmarshal(raw, &n) != nil {
		return ""
	}
	var text strings.Builder
	for _, item := range n.Turn.Items {
		text.WriteString(agentMessageTextFromItem(item))
	}
	return text.String()
}

// approvalCommand 把 command 字段归一成一行命令字符串。
func approvalCommand(raw json.RawMessage) string {
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return strings.Join(arr, " ")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// codexApprovalWaitBudget 审批等待预算:用户可能久不点(去开会、过夜再看),
// 等待本身是合法静默(看门狗豁免);但"手机在线、就是不点"没有任何天然上界,
// 反向请求会永远占着 codex 侧的未决请求槽与手机端一张点不动的死卡 —— 通道不能
// 无限挂,给 15 分钟预算(与一次深度任务轮的合理等待同量级;手机断流另有无处
// 不在的 ctx 超时兜底,这里防的是"流还在、人不点")。超预算按"通道失败"处置
// (照下方 askErr 分支口径:回 -32000 让 codex 按基础设施故障自行恢复,绝不
// 伪装用户拒绝)。var 仅为缩时单测留缝,生产不要改它。
var codexApprovalWaitBudget = 15 * time.Minute

// codexApprovalResponder codexapp.Client 的应答面(最小接口,单测可注 fake,
// 不依赖真子进程;zcode 的 zcodeResponder 同款模式)。id 用 RawMessage 与
// codexapp 的签名同形:数字原值、字符串原文,应答原样回显。
type codexApprovalResponder interface {
	Respond(id json.RawMessage, result interface{}) error
	RespondError(id json.RawMessage, code int, message string) error
}

// handleCodexApproval 处理一个已识别为"问人"的反向请求:构造审批卡 → 问手机 →
// 把结果翻译成 codex 协议应答。所有路径要么已应答、要么因桌面端已解决而不应答,
// 调用方拿到返回即可结束。
// 【P1-1】approve==nil 属审批通道缺失(TestModel 诊断 RPC 等路径传 nil):
// 回 JSON-RPC error 让 codex 按通道故障处理,绝不替用户伪造 denied 决定
// (与下方 askErr 分支同口径:基础设施问题不伪装成用户意志)。
// threadID 是我们自己的会话 id,供超时/写回失败的日志定位到具体会话(台账口径
// 与全家一致:日志必须能回答"哪个会话的审批卡出的事")。
func (p *CodexProvider) handleCodexApproval(rsp codexApprovalResponder, id json.RawMessage, kind codexRequestKind, method string, params json.RawMessage, files []string, ctx context.Context, sink Sink, approve ApprovalHook, waiting *atomic.Bool, threadID string) {
	// 【第二条规则】按类别构造真实的审批卡,再问手机。
	call := buildApprovalCall(kind, method, params, files)
	if approve == nil {
		log.Printf("[codex] 审批通道不可用(approve==nil),%s 回 error", method)
		_ = rsp.RespondError(id, -32000, "审批通道不可用")
		return
	}
	// 【B4 应答必达性留痕】decision 已由用户定出,写回失败(子进程恰好退出/
	// 管道断裂)时不再 _ = 丢弃 —— 落一行日志带线程 id 与方法名,与"绝不伪造
	// decision"红线配套:痕迹不改变应答语义,只保证"答案没送到"可追查。
	respond := func(result interface{}) {
		if err := rsp.Respond(id, result); err != nil {
			log.Printf("[codex] 审批应答写回失败 thread=%s method=%s: %v", threadID, method, err)
		}
	}
	waiting.Store(true)
	// 【serverRequest/resolved 竞速】我们等手机应答期间,codex 可能通知
	// "该请求已被解决"(用户在桌面端点了,或被 codex 自动处理)。
	// 此时不能再等手机、也绝不能再 Respond(重复应答);同时要让手机端
	// 把挂着的审批卡撤下去 —— 否则就是一张点不动也没意义的"幽灵审批卡"。
	resolved := p.registerPendingServerRequest(id)
	type approveOut struct {
		allow  bool
		choice approvalChoice
		err    error
	}
	outCh := make(chan approveOut, 1)
	go func() {
		a, n, e := approve(ctx, call)
		outCh <- approveOut{allow: a, choice: parseApprovalChoice(n), err: e}
	}()
	allow := false
	choice := approvalChoice{}
	askErr := error(nil)
	select {
	case out := <-outCh:
		allow, choice, askErr = out.allow, out.choice, out.err
	case <-resolved:
		// 迟到的手机应答会落进 outCh(带缓冲),由 GC 回收,无泄漏。
		log.Printf("[codex] 反向请求 %s(id=%s)已在桌面端被解决,不再等待手机审批", method, codexRequestIDKey(id))
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ApprovalResolved{ApprovalResolved: &pb.ApprovalResolved{
			Note: "这条审批已在电脑端被处理,手机上的审批卡已同步关闭。",
		}}})
		waiting.Store(false)
		p.unregisterPendingServerRequest(id)
		return
	case <-time.After(codexApprovalWaitBudget):
		// 【B3 等待预算】用户久不点也不能无限挂(预算取舍见常量注释)。超时
		// ≠ 用户拒绝:照"通道失败不是用户拒绝"口径回 -32000,让 codex 按基础
		// 设施故障自行恢复,绝不回 denied 伪装用户意志;副作用收干净 —— waiting
		// 复位(看门狗豁免解除)+ 注销未决请求(防 map 泄漏),日志带线程 id
		// (台账口径:出事必须能定位到会话)。迟到的手机应答落 outCh 由 GC 回收。
		log.Printf("[codex] 审批等待超预算(%s)未取得用户决定,thread=%s method=%s 回 error(非用户拒绝)",
			codexApprovalWaitBudget, threadID, method)
		waiting.Store(false)
		p.unregisterPendingServerRequest(id)
		_ = rsp.RespondError(id, -32000, fmt.Sprintf("审批等待超时(%s),未取得用户决定", codexApprovalWaitBudget))
		return
	}
	waiting.Store(false)
	p.unregisterPendingServerRequest(id)
	if askErr != nil {
		// 手机端断流/超时:这不是"用户拒绝"。回 error,让 codex 按通道故障处理,
		// 别把基础设施问题伪装成用户意志。
		log.Printf("[codex] 审批通道失败 %s: %v", method, askErr)
		_ = rsp.RespondError(id, -32000, "审批通道失败: "+askErr.Error())
		return
	}

	// 【第三条规则】应答形状按方法名走,尤其旧版审批用 approved/denied 而不是 accept/decline。
	// 【0.159.2 三按钮】choice.Option 非空时按选项翻译:acceptForSession =
	// 允许且本会话内同类请求不再询问(旧版 approved_for_session);cancel =
	// 拒绝且**立即中止本轮** —— codex 收到该枚举后自己打断当前轮次
	// (schema: "The turn will also be immediately interrupted."),
	// daemon 不再另发停止指令,避免双重中止。旧手机不带选项,行为一字不变。
	switch kind {
	case reqApprovalCommand, reqApprovalFileChange, reqApprovalPatch:
		decision := codexApprovalDecision(method, choice.Option, allow)
		if choice.Option == optCancel || choice.Option == optAbort {
			log.Printf("[codex] 用户拒绝并中止本轮, %s 回 decision=%q(上游将打断当前轮次)", method, decision)
		} else if choice.Option == optAcceptSession || choice.Option == optApprovedSession {
			log.Printf("[codex] 用户允许且本会话内同类请求不再询问, %s 回 decision=%q", method, decision)
		}
		respond(map[string]string{"decision": decision})

	case reqApprovalPermissions:
		if allow {
			var a permissionsApprovalParams
			_ = json.Unmarshal(params, &a)
			var perms interface{}
			_ = json.Unmarshal(a.Permissions, &perms)
			// 原样授予申请的那份 profile:不做裁剪,否则"少给一个路径"会让命令
			// 换一种方式失败,故障更难定位。
			respond(map[string]interface{}{"permissions": perms, "scope": "turn"})
		} else {
			// 这个应答没有 decision 字段:不同意的表达方式就是"什么都不授予"。
			respond(map[string]interface{}{"permissions": map[string]interface{}{}})
		}

	case reqAskUser:
		var a askUserParams
		_ = json.Unmarshal(params, &a)
		// 手机端的答案放在 ApprovalDecision.note 里(JSON),这里原样转成 answers。
		// 用 choice.Note:若 note 恰是选项信封(不该发生),剥完再解析。
		respond(map[string]interface{}{"answers": splitAnswerNote(choice.Note, a.Questions)})

	case reqAskMcp:
		action := "decline"
		if allow {
			action = "accept"
		}
		respond(map[string]interface{}{"action": action})
	}
}

func (p *CodexProvider) Run(ctx context.Context, req TurnRequest, sink Sink, approve ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	p.turnMu.Lock() // 同一时刻只跑一轮(手机端本来也是单会话交互)
	defer p.turnMu.Unlock()
	p.turnActive.Store(true)
	defer p.turnActive.Store(false)

	if err := p.withClient(func(*codexapp.Client) error { return nil }); err != nil {
		return nil, nil, err
	}

	// 1) 找到/创建 codex 线程(子进程重启后必须 resume,见 threadRef 注释)
	// 【输入收集】文本拼接(零裁剪约定不变);图片字节暂存,等下面模型 slug 确定、
	// 做完能力门控后再映射成 localImage 块(见 turnInputWithImages)。
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
	gen := int(p.gen.Load())
	codexTID := ""
	// 【P1-7 bindSession 上收】会话解析统一经原生会话适配器(cs)单一入口:
	// 重挂载走 cs.Resume、新建走 cs.openForRun,内部复用 callThreadResume/
	// startCodexThread 唯一实现;"过渡期双路径"就此退役。Run 链保留的只有
	// 同代短路、"resume 失败改开新线程"的降级(D4)与目标暂存补发。
	ns, nerr := p.NativeSessionFor(req.Thread)
	if nerr != nil {
		return nil, nil, nerr
	}
	cs, csok := ns.(*codexSession)
	if !csok {
		return nil, nil, fmt.Errorf("codex: 原生会话适配器类型异常: %T", ns)
	}
	// 线程级 config 覆盖(model_verbosity / model_reasoning_summary)。
	// 放在这里算一次:resume 与 start 都要带上,保证"回答详略/思考摘要"对
	// 既有会话和新会话都生效(只在新建线程时带,会让老会话永远拿旧偏好)。
	// 【线程化】有设置快照的会话按快照生成(新建对话语义:之后改设置不影响它),
	// 旧线程(快照 nil)回退全局。
	// 【反问卡:已主动停用实验特性注入 —— 见下方 AskUserInputSupported 注释】
	// 这里只算"回答详略/思考摘要"的覆盖,绝不再并上
	// features.default_mode_request_user_input。开启该 under-development 特性
	// 必然触发 codex 的"Under-development features enabled"警告(真机实测每次都弹),
	// 而抑制它的唯一官方途径是改写用户 ~/.codex/config.toml,本项目红线永久封死。
	// 因此 daemon 不再注入该特性;反问能力是否存在改为由 askUserInputSupported()
	// 运行时宣告,交给手机端决定是否显示反问入口(见 codex_requests.go / android)。
	cfgOverride := p.ConfigOverrideFor(req.Thread)
	if ref, hit := p.codexThreadRef(req.Thread.ID); hit && ref.gen == gen {
		codexTID = ref.id // 当前子进程仍加载着该线程
	} else if req.Thread.CodexThreadID != "" {
		// daemon/子进程重启后内存缓存作废:先把旧线程 resume 进当前子进程。
		// resume 也带 config,保证"改了立刻生效"对老会话成立,且可核验。
		// 【P1-7 上收】resume 统一经适配器 Resume(callThreadResume 唯一实现,
		// config 透传与同代缓存登记都在里面);Run 保留的只有降级决策(D4):
		// 失败清引用改开新线程 —— 缺这条护栏时表现为"AI 失忆"(用户老会话
		// 历史全丢),适配器绝不擅自新建。
		log.Printf("[codex] thread/resume %s config=%v", req.Thread.CodexThreadID, cfgOverride)
		if rerr := cs.Resume(ctx, req.Thread); rerr != nil {
			log.Printf("[codex] thread/resume %s 失败: %v(改开新线程)", req.Thread.CodexThreadID, rerr)
			req.Thread.CodexThreadID = ""
		} else {
			codexTID = req.Thread.CodexThreadID
			log.Printf("[codex] 已 resume 旧线程 %s", codexTID)
		}
	}
	if codexTID == "" {
		// 【P3 拆分 + P1-7 上收】新建统一经适配器入口 openForRun(startCodexThread
		// 是 thread/start 调用与 threadId 三级回退解析的唯一实现,同代缓存登记与
		// 引用写回都在里面);本函数保留的是 Run 链自己的编排:同代短路、resume
		// 失败"改开新线程"的降级、目标暂存补发。ephemeral/config 覆盖显式传入,
		// cfgOverride 不重算第二遍。
		tid, terr := cs.openForRun(ctx, req.Thread, req.Ephemeral, cfgOverride)
		if terr != nil {
			return nil, nil, terr
		}
		codexTID = tid
		if req.Thread.Goal != "" {
			p.stashGoal(req.Thread.ID, req.Thread.Goal)
		}
		log.Printf("[codex] 新建线程 %s", codexTID)
	}

	// 2) 事件路由
	// 线程此时已确实加载在 codex 侧:把之前暂存的常驻目标补发下去
	// (用户可能是"先设目标、再开始第一轮",那时还没有 codex threadId)。
	p.applyPendingGoal(req.Thread.ID, codexTID)

	var finalText strings.Builder  // item/completed 兜底来源
	var streamText strings.Builder // delta 实时拼接(主来源,和手机看到的逐字一致)
	var mu sync.Mutex
	var lastNotif atomic.Int64      // 最近一次收到通知的时间(看门狗用)
	var approvalWaiting atomic.Bool // 正在等手机审批:这是合法静默,看门狗跳过
	done := make(chan struct{})
	var doneOnce sync.Once
	finish := func() { doneOnce.Do(func() { close(done) }) }
	var lastErr *pb.Error
	// 本回合 codex 报过变更的文件。审批卡上"这次会改哪些文件"的唯一来源 ——
	// v2 的 item/fileChange/requestApproval 参数里**没有**文件列表,只有同一回合
	// item/fileChange/patchUpdated 通知带过来的 changes。写(readLoop 分发 goroutine)
	// 与读(反向请求 handler)不在同一 goroutine,统一用 mu 保护。
	var changeFiles []string

	c := p.client
	// 通知分发走单独 goroutine:readLoop 若被慢 sink(手机网络背压)阻塞,
	// 管道里后续事件会全部积压甚至卡死。单消费者保序。
	type notifMsg struct {
		method string
		params json.RawMessage
	}
	notifCh := make(chan notifMsg, 4096)
	notifStop := make(chan struct{})
	notifDone := make(chan struct{})
	// 绑定会话 id:未知通知/请求登记进诊断台账时要能关联到具体会话(见下方 defer 里
	// 的 clearSteerTarget(req.Thread.ID),同一个 req 就在本作用域内)。
	notices := nativeNoticeRecorder{threadID: req.Thread.ID}
	var subagentSeen subagentNoticeGate // 子代理 item 事件限频(语义同 nativeNoticeRecorder)
	// 【C-1 坏帧留痕 → B4 共享件(2026-10-09)】事件泵 item/turn 各 case 的参数
	// 解包失败统一走 notices.recordBadFrame("codex", …):台账 UNKNOWN(method 带
	// <unparseable> 标记)+ 限频 STATUS 卡;异线程过滤维持静默。裁量的完整说明
	// (codex 为何出卡而 zcode/dsh 只台账)见 native_notifications.go 的
	// recordBadFrame 注释。
	handleNotif := func(method string, params json.RawMessage) {
		switch {
		case method == "turn/started":
			// 任务真正进入 codex 的凭据:turn/started 已到达
			log.Printf("[codex] codex 已接收任务 thread=%s %.140s", codexTID, string(params))
			// P1-4:登记「当前运行中轮次」的 codex id,供 turn/steer 校验
			// expectedTurnId(见 codex_steer.go)。turnActive 守卫过滤迟到的旧通知。
			// 【C-1】解包失败(parseTurnStarted nil:解不开或缺 turn.id,都是
			// "没解出可用的轮次标识")单独留痕;异线程静默维持(同款过滤)。
			if n := parseTurnStarted(params); n != nil {
				if n.ThreadId == codexTID {
					p.setSteerTarget(req.Thread.ID, codexTID, n.Turn.Id)
				}
			} else {
				notices.recordBadFrame("codex", method, params, sink)
			}
		case method == "item/fileChange/patchUpdated":
			// 累积"本回合被改动的文件",供文件变更审批卡展示(见 changeFiles 注释)。
			var n struct {
				ThreadId string `json:"threadId"`
				Changes  []struct {
					Path string `json:"path"`
				} `json:"changes"`
			}
			// 【C-1】解包失败此前整帧静默(审批卡将拿不到文件列表且无任何痕迹)。
			if err := json.Unmarshal(params, &n); err != nil {
				notices.recordBadFrame("codex", method, params, sink)
			} else if n.ThreadId == codexTID {
				paths := make([]string, 0, len(n.Changes))
				for _, ch := range n.Changes {
					if ch.Path != "" {
						paths = append(paths, ch.Path)
					}
				}
				if len(paths) > 0 {
					mu.Lock()
					changeFiles = mergePaths(changeFiles, paths)
					mu.Unlock()
				}
			}
		case method == "thread/goal/updated" || method == "thread/goal/cleared":
			// codex 是 goal 的事实来源:它会规范化文本、也会自己改状态
			// (active→complete / blocked / usageLimited)。把变化同步回我们库里,
			// 手机端就能看到"目标已完成"这类信息,而不是永远停在设下去的那一刻。
			goalText, status, tokensUsed, timeUsed := goalStatusFromNotification(params)
			if method == "thread/goal/cleared" {
				goalText = ""
			}
			log.Printf("[codex] goal 变化 %s status=%s tokens=%d time=%ds", method, status, tokensUsed, timeUsed)
			if p.GoalHandler != nil {
				go p.GoalHandler(codexTID, goalText, status, tokensUsed, timeUsed)
			}
		case method == "turn/completed":
			var n struct {
				ThreadId string `json:"threadId"`
				Turn     struct {
					Items []struct {
						Type    string `json:"type"`
						Text    string `json:"text"` // 新版:agentMessage 直接带 text
						Content []struct {
							Type string `json:"type"`
							Text string `json:"text"`
						} `json:"content"` // 旧版兼容
					} `json:"items"`
					// Status 是轮次结束原因(completed/interrupted/failed/inProgress,
					// schema TurnStatus,活体轮实证 docs/verify/evidence/
					// codex-01592-liveturn-20261002.log 的收口帧带 "status":"completed")。
					// 旧实现只看 error 对象:failed 而没附 error、或 interrupted 的轮次
					// 都被当成"正常完成",用户看到的终局与实际不符。
					Status string `json:"status"`
					Error  *struct {
						Message        string          `json:"message"`
						Additional     string          `json:"additionalDetails"`
						CodexErrorInfo json.RawMessage `json:"codexErrorInfo"`
					} `json:"error"` // turn 级终局错误
				} `json:"turn"`
			}
			// 【C-1】终局帧解包失败此前整段静默(连 finish() 都不触发,轮次只能
			// 等看门狗收口且无任何痕迹)。留痕不改变收口语义:finish 仍只在
			// "解包成功且同线程"时调用,坏帧的处置路径维持原样,只是现在查得到。
			if err := json.Unmarshal(params, &n); err != nil {
				notices.recordBadFrame("codex", method, params, sink)
			} else if n.ThreadId == codexTID {
				if n.Turn.Error != nil && n.Turn.Error.Message != "" {
					mu.Lock()
					lastErr = &pb.Error{
						Code:    codexErrorCode(n.Turn.Error.CodexErrorInfo),
						Message: n.Turn.Error.Message,
						Details: n.Turn.Error.Additional,
						Url:     extractUpstreamURL(n.Turn.Error.Message),
					}
					lastErr.HttpStatus = int32(statusFromMessage(n.Turn.Error.Message))
					lastErr.Retryable = false
					mu.Unlock()
				} else if n.Turn.Status == "failed" {
					// failed 却没附 error 对象:如实按失败收口,不冒充成功。
					mu.Lock()
					lastErr = &pb.Error{
						Code:      "turnFailed",
						Message:   "Codex 报告本轮未能正常完成,且没有附上具体原因",
						Details:   "turn.status=failed",
						Retryable: false,
					}
					mu.Unlock()
				} else {
					// 正常终局:清掉此前瞬时 error 通知留下的痕迹
					mu.Lock()
					lastErr = nil
					mu.Unlock()
					if n.Turn.Status == "interrupted" {
						// 被打断的轮次(电脑端停止/另一端 interrupt):已流出的部分正文
						// 照常交回,但必须告诉用户"为什么话说到一半停了",否则看起来
						// 就是"答完了"。不打成 lastErr:中断不是失败,部分输出仍有效。
						sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
							Kind:     pb.CodexEvent_WARNING,
							Code:     "turnInterrupted",
							Title:    "本轮已中断",
							Message:  "任务在完成前被中断(可能是在电脑端停止了执行)。以上是中断前已输出的内容,可继续追问让它接着做。",
							ThreadId: codexTID,
							RawJson:  clip(string(params), 8000),
						}}})
					}
				}
				// turn.completed 自带最终 items,兜底提取(防 item/completed 竞态漏掉)。
				var sb strings.Builder
				sb.WriteString(agentMessageTextsFromTurn(params))
				mu.Lock()
				if sb.Len() > 0 && finalText.Len() == 0 {
					// delta 拼接是主来源;items 只在 delta 全丢时兜底
					finalText.WriteString(sb.String())
				}
				mu.Unlock()
				finish()
			}
		case strings.Contains(method, "agentMessage") && strings.Contains(method, "delta"):
			var n deltaNotif
			// 【C-1】解包失败单独留痕;空 delta 是合法空转,维持静默。
			if err := json.Unmarshal(params, &n); err != nil {
				notices.recordBadFrame("codex", method, params, sink)
			} else if n.ThreadId == codexTID && n.Delta != "" {
				mu.Lock()
				streamText.WriteString(n.Delta)
				mu.Unlock()
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: n.Delta}}})
			}
		case isReasoningDeltaMethod(method):
			var n deltaNotif
			// 【C-1】解包失败单独留痕;空 delta 是合法空转,维持静默。
			if err := json.Unmarshal(params, &n); err != nil {
				notices.recordBadFrame("codex", method, params, sink)
			} else if n.ThreadId == codexTID && n.Delta != "" {
				// 隐藏推理过程(daemon 侧强制过滤):不把思考事件推给手机。
				// 这是真实的行为改变,不依赖 codex 是否有原生开关 —— 本版本 codex
				// 的 schema 里根本没有 hide_agent_reasoning 这个配置键。
				// 【线程化】按本线程的设置快照判定(有快照的会话与全局开关无关);
				// 旧线程(快照 nil)回退全局。
				if p.HideReasoningFor(req.Thread) {
					break
				}
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Delta: n.Delta}}})
			}
		case method == "item/started":
			var n itemNotif
			// 【C-1】解包一次,失败态与异线程分开判定(此前两者都静默落进末尾
			// 兜底,坏形状与"别的会话的帧"不可区分)。
			unpacked := json.Unmarshal(params, &n) == nil
			if unpacked && n.ThreadId == codexTID && n.Item.Type == "contextCompaction" {
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
					Kind:     pb.CodexEvent_CONTEXT_COMPACTED,
					Code:     "started",
					Title:    "上下文压缩中",
					Message:  "正在压缩上下文,期间占用率暂不显示。",
					ThreadId: codexTID,
					RawJson:  clip(string(params), 8000),
				}}})
			} else if unpacked && n.ThreadId == codexTID && n.Item.Type == "commandExecution" {
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
					Phase: pb.ToolCallUpdate_STARTED,
					Call: &pb.ToolCallBlock{
						Id: n.Item.Id, Name: "shell",
						JsonArgs: mustJSON(map[string]interface{}{"command": strings.Join(n.Item.Command, " ")}),
					},
				}}})
			} else if ev, key := subagentEvent(params, codexTID); ev != nil {
				// 子代理活动/协作工具调用(codex_subagents.go):透传 + 限频防刷屏。
				// 注意:其余 item 类型既不进这里、也不会走外层 default —— 本 case
				// 已命中,方法不会再落到 default(旧注释"走外层兜底"失实,已修正);
				// 它们由下方 else 分支接主线 A 透传,不再是静默丢弃。
				if subagentSeen.allow(key) {
					sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: ev}})
				}
			} else if msg := codexItemToolCard(method, params, codexTID); msg != nil {
				// 工具类 item(fileChange/mcpToolCall/webSearch)的开始卡:
				// 独立工具卡投影(见 codex_items.go),不再降级成不透明透传。
				sink(msg)
			} else if !unpacked {
				// 【C-1】坏帧:解不开的 params 在这里单独留痕(走到这说明前面
				// 各专案分支都不可能命中 —— 它们各自的解析同样解不开)。
				notices.recordBadFrame("codex", method, params, sink)
			} else {
				// 其余 item 类型(userMessage/reasoning 等):此前是静默丢弃,现接进
				// 主线 A notices.record 原样包装透传(限频+脱敏自动生效)。仅透传,
				// 不伪造;已知类型与异线程在 maybeRecordOpaqueItem 内被排除。
				maybeRecordOpaqueItem(method, params, codexTID, &notices, sink)
			}
		case method == "item/completed":
			var n itemNotif
			// 【C-1】解包失败与异线程此前共用一条 return,坏形状与"别的会话的帧"
			// 不可区分;现在坏帧单独留痕,异线程过滤维持静默(同款口径)。
			if err := json.Unmarshal(params, &n); err != nil {
				notices.recordBadFrame("codex", method, params, sink)
				return
			}
			if n.ThreadId != codexTID {
				return
			}
			switch n.Item.Type {
			case "agentMessage":
				mu.Lock()
				finalText.WriteString(agentMessageTextFromNotification(params))
				mu.Unlock()
			case "contextCompaction":
				p.resetCompactState(codexTID)
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
					Kind:     pb.CodexEvent_CONTEXT_COMPACTED,
					Code:     "completed",
					Title:    "上下文已压缩",
					Message:  "上下文已压缩,正在等待新的用量上报。",
					ThreadId: codexTID,
					RawJson:  clip(string(params), 8000),
				}}})
			case "commandExecution":
				output := clip(n.Item.AggregatedOutput, 4000)
				if n.Item.ExitCode != nil && *n.Item.ExitCode != 0 {
					// 非零退出码是"命令其实没跑成"的第一信号:拼在结果末尾,
					// 输出为空时它就是唯一线索(此前该字段被静默丢弃)。
					output = strings.TrimSpace(output + "\n(退出码 " + strconv.FormatInt(*n.Item.ExitCode, 10) + ")")
				}
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
					Phase: pb.ToolCallUpdate_FINISHED,
					Call: &pb.ToolCallBlock{
						Id: n.Item.Id, Name: "shell",
						JsonArgs: mustJSON(map[string]interface{}{"command": strings.Join(n.Item.Command, " ")}),
					},
					Result: &pb.ToolResultBlock{
						CallId: n.Item.Id,
						Ok:     n.Item.Status == "completed" || n.Item.Status == "",
						Output: output,
					},
				}}})
			case "subAgentActivity", "collabAgentToolCall":
				// 子代理活动/协作工具调用(codex_subagents.go):与 item/started
				// 同一套翻译与限频 —— item 自身的 kind/status 已区分阶段。
				if ev, key := subagentEvent(params, codexTID); ev != nil && subagentSeen.allow(key) {
					sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: ev}})
				}
			case "fileChange", "mcpToolCall", "webSearch":
				// 工具类 item 的收尾卡(codex_items.go):与开始卡同一条 ToolCall
				// 通路,阶段/结果随 item 自身的 status 走;解析不出形状时落回
				// 下面的不透明透传,不伪造成功。
				if msg := codexItemToolCard(method, params, codexTID); msg != nil {
					sink(msg)
				} else {
					maybeRecordOpaqueItem(method, params, codexTID, &notices, sink)
				}
			default:
				// 其余 item 类型(userMessage/reasoning 等):此前静默丢弃,接主线 A
				// notices.record 原样包装透传(限频+脱敏自动生效)。已知类型各有
				// case 不会进这里;仅透传,不伪造。
				maybeRecordOpaqueItem(method, params, codexTID, &notices, sink)
			}
		case method == "thread/name/updated":
			var n struct {
				ThreadId   string  `json:"threadId"`
				ThreadName *string `json:"threadName"`
			}
			if json.Unmarshal(params, &n) == nil && n.ThreadId == codexTID && n.ThreadName != nil && *n.ThreadName != "" {
				if p.TitleHandler != nil {
					go p.TitleHandler(codexTID, *n.ThreadName)
				}
			}
		case method == "error":
			event, errInfo := parseCodexErrorNotification(params, codexTID, "")
			if event == nil || errInfo == nil {
				return
			}
			log.Printf("[codex] error code=%s status=%d retry=%t: %.240s",
				event.Code, event.HttpStatus, event.WillRetry, event.Message)
			sink(&pb.TurnServerMessage{
				Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: event},
			})
			mu.Lock()
			lastErr = errInfo
			mu.Unlock()
		default:
			if method == "thread/tokenUsage/updated" {
				if usage := codexTokenUsageUpdate(params, codexTID, ""); usage != nil {
					sink(&pb.TurnServerMessage{
						Msg: &pb.TurnServerMessage_Usage{Usage: usage},
					})
					// 真实自动压缩:占用率越阈自动调 thread/compact/start,状态机防抖。
					// 【口径与桌面端一致】占用率 = 本 turn 用量(last) / 窗口:每轮输入都
					// 重发全部上下文,last.totalTokens 就是"当前窗口被占了多少"。
					// 绝不能用 total_usage(thread 累计)—— 压缩后累计不回落,状态机永远无法重新武装。
					// 窗口未知(0)时 compactDecide 不触发,不会误压。
					p.maybeAutoCompact(codexTID, usage.GetUsage(), usage.GetModelContextWindow())
				}
			}
			if method == "context/compacted" || method == "thread/compacted" {
				// 压缩已发生:重新武装状态机,后续再次吃满会再次触发。
				p.resetCompactState(codexTID)
			}
			if method == "serverRequest/resolved" {
				// 该反向请求已在桌面端被解决:若我们正等手机审批,提前结束等待
				// 并让手机撤卡(幽灵审批卡治理,见 ReqHandler 的竞速分支)。
				var n struct {
					RequestId json.RawMessage `json:"requestId"`
					ThreadId  string          `json:"threadId"`
				}
				if json.Unmarshal(params, &n) == nil {
					p.resolvePendingServerRequest(n.RequestId)
				}
			}
			if event := codexNotificationEvent(method, params, codexTID, ""); event != nil {
				log.Printf("[codex] event method=%s: %.240s", method, event.Message)
				sink(&pb.TurnServerMessage{
					Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: event},
				})
			} else {
				notices.recordAs(codexMethodNoticeKind(method), "codex", method, params, sink)
			}
		}
	}
	// readLoop 里同步调 NotifHandler:慢 sink(手机网络背压)会阻塞整条管道,
	// 后续事件全部积压。这里只入队,由单消费者 goroutine 保序分发。
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
	go func() {
		defer close(notifDone)
		for {
			select {
			case nm := <-notifCh:
				handleNotif(nm.method, nm.params)
			case <-notifStop:
				return
			}
		}
	}()
	reqHandler := func(id json.RawMessage, method string, params json.RawMessage) {
		log.Printf("[codex] 反向请求 %s 原始参数: %.400s", method, string(params))
		kind := classifyCodexRequest(method)

		// 【第一条规则】不认识的、以及明确不属于我们职责的,回 JSON-RPC error。
		// 以前这里统一回 {"decision":"decline"} —— 那不是"拒绝执行",而是拿一种请求的
		// 报文形状去回答另一种请求,对方只能按协议错误处理(token 刷新就是这么被打死的)。
		switch kind {
		case reqCurrentTime:
			// 0.160.0 新增反向请求 currentTime/read:桌面把 clock_source 配成 external 时,
			// app-server 把连接着的客户端(我们)当外部时钟源,在注入"时间提醒"前问当前
			// 时间。协议是 fail-closed:不回、回错形状或超时,本轮在模型请求前直接停 ——
			// 所以这里必须如实应答,不能落进 reqUnknown 的 -32601 兜底。
			// 应答形状 {"currentTimeAt": <unix 秒>}(字段名在 0.160.0 二进制请求面
			// "currentTimeAt" 与公开协议文档双侧实证;params 只带 threadId,无需回读)。
			now := time.Now().Unix()
			log.Printf("[codex] currentTime/read → 应答 currentTimeAt=%d", now)
			_ = c.Respond(id, map[string]int64{"currentTimeAt": now})
			return
		case reqUnsupported, reqUnknown:
			// 【辅线 B】开关 ON 时,未知反向请求先弹手机通用确认卡(复用 requestApproval
			// 的 approve/deny 回路,确认卡经既有审批卡通道渲染):允许 → 已回空对象 {}
			// + 台账 KNOWN_PASSTHROUGH,true=请求已终结;等待中桌面端已解决 → 双方都
			// 不再应答。拒绝/超时/通道缺失/开关关 → false,落到下面的既有安全拒绝
			// 路径,-32601 + 台账与 OFF 态走同一段代码(台账口径逐字节一致)。
			if kind == reqUnknown && p.consumeCodexUnknownRequest(c, id, method, req.Thread.ID, params, ctx, sink, approve, &approvalWaiting) {
				return
			}
			msg := unsupportedRequestMessage(method)
			if kind == reqUnknown {
				msg = "PocketAgent 未实现该反向请求: " + method
			}
			log.Printf("[codex] %s → 回 error(%s)", method, msg)
			// 登记进诊断台账:回 error 只让对端知道"这次没做成",而"我们收到过一个
			// 不认识的请求"需要能事后查(规格 §9:未知 request 保留原始 payload、
			// 关联会话)。只登记**真正未知**的 reqUnknown —— reqUnsupported 是我们
			// 明确不承担的类别(如 token 刷新),它的"没做"是已知的设计选择,不是未知。
			if kind == reqUnknown {
				sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
					ThreadID:       req.Thread.ID,
					SourceAgent:    "codex",
					SourceMethod:   method,
					NormalizedKind: runtime.KindUnknownRequest,
					RawPayload:     redactNativePayload(params),
					Note:           "已回协议错误 -32601(安全拒绝;未替用户做任何审批决定)",
				})
			}
			_ = c.RespondError(id, -32601, msg)
			return
		}

		mu.Lock()
		files := append([]string(nil), changeFiles...)
		mu.Unlock()

		// 【第二条规则】按类别构造真实的审批卡,再问手机(抽到 handleCodexApproval,
		// zcode 的 handleZcodePermission 同款注入式单测模式)。
		p.handleCodexApproval(c, id, kind, method, params, files, ctx, sink, approve, &approvalWaiting, req.Thread.ID)
	}
	c.SetTurnHandlers(notifHandler, reqHandler)
	defer func() {
		c.SetTurnHandlers(nil, nil)
		// P1-4:本轮结束,插话目标随之失效(下一个 turn/started 会重新登记)
		p.clearSteerTarget(req.Thread.ID)
		close(notifStop)
		<-notifDone
	}()

	// 3) 发起本轮(STARTED 由 server 层统一发,这里不再重复)
	// input 先按纯文本缺省封装;有图片时在下面做完能力门控后重写(见 turnInputWithImages)。
	startParams := map[string]interface{}{
		"threadId": codexTID,
		"input":    turnInput(userText.String()),
	}
	// 手机设置页配置的桌面端策略,每轮显式携带(codex 会话内真实生效)。
	// 【三级优先级】线程显式覆盖(Thread.ModelID/Effort,会话内 ModelEffortCard)>
	// 线程设置快照(SettingsSnapshot,新建对话那一刻的值)> 全局偏好(旧线程兜底):
	// 有快照的会话一律按快照装配审批策略/沙箱/模型/思考强度/思考摘要 —— 设置页
	// 之后的更改只影响新建对话,不再牵动已有对话的后续轮次;快照里的空串是
	// "快照那一刻的跟随桌面",同样按原值生效(不会中途改抓新全局值)。
	p.setMu.Lock()
	approvalPolicy, sandbox := p.approvalPolicy, p.sandbox
	desktopModel, desktopEffort := p.desktopModel, p.desktopEffort
	desktopSummary := p.modelReasoningSummary
	if snap := req.Thread.SettingsSnapshot; snap != nil {
		approvalPolicy = snap.ApprovalPolicy
		sandbox = snap.Sandbox
		desktopModel = snap.Model
		desktopEffort = snap.Effort
		desktopSummary = snap.ReasoningSummary
	}
	p.setMu.Unlock()
	if approvalPolicy != "" {
		startParams["approvalPolicy"] = approvalPolicy
	}
	if sandbox != "" {
		// v2 协议:sandboxPolicy 是对象;旧键名 "sandbox" 会被拒
		mode := map[string]string{
			"read-only":          "readOnly",
			"workspace-write":    "workspaceWrite",
			"danger-full-access": "dangerFullAccess",
		}[sandbox]
		if mode == "" {
			mode = sandbox
		}
		startParams["sandboxPolicy"] = map[string]interface{}{"type": mode}
	}
	// 模型:线程级指定优先,否则用设置页的桌面默认;codex/default 视为跟随桌面。
	// 注意:模型 ID 里可能带 "/"(账号组),取最后一段才是模型 slug。
	// slug 同时供图片能力门控使用(codexSupportsImageInput)。
	modelSlug := ""
	if m := req.Thread.ModelID; strings.HasPrefix(m, "codex/") && !strings.HasSuffix(m, "/default") {
		modelSlug = strings.TrimPrefix(m, "codex/")
	} else if seg := lastModelSeg(desktopModel); seg != "" {
		modelSlug = seg
	}
	if modelSlug != "" {
		startParams["model"] = modelSlug
	}
	// 思考强度(手机端会话内可切):low/medium/high/xhigh
	if e := req.Thread.Effort; e != "" {
		startParams["effort"] = e
	} else if desktopEffort != "" {
		startParams["effort"] = desktopEffort
	}
	// 思考摘要详略(auto/concise/detailed):空 = 跟随桌面,不带该参数
	if desktopSummary != "" {
		startParams["summary"] = desktopSummary
	}
	// 图片输入:有图才查一次 model/list 做能力门控;无图零开销,纯文本路径与原先完全一致。
	// cleanup 在 Run 返回(turn 终局)后删除临时图片 —— 这对 resume 也是安全的,
	// 无需把清理延迟到线程关闭:live 实测(cmd/imgprobe resume,2026-09-19)发图
	// 一轮后立即删掉临时文件,同会话追问与"杀掉 app-server 重开 + thread/resume
	// 后追问"都能正确答出图片内容(背景色/黑点数,且该信息从未出现在任何文字
	// 轮次里);rollout 文件里该图记录为 input_image 的 dataURL 编码(原路径只是
	// 随附的元文本),即 resume 重放历史用的是编码内容,不按路径重读原文件。
	if len(userImgs) > 0 {
		imageOK := p.codexSupportsImageInput(modelSlug)
		items, imgCleanup := turnInputWithImages(userText.String(), userImgs, imageOK)
		startParams["input"] = items
		defer imgCleanup()
	}
	logParams := map[string]interface{}{}
	for k, v := range startParams {
		if k != "input" {
			logParams[k] = v
		}
	}
	log.Printf("[codex] turn/start thread=%s %v 输入=%d字 图片=%d张", codexTID, logParams, userText.Len(), len(userImgs))

	lastNotif.Store(time.Since(turnStartRef).Milliseconds()) // 初喂:单调毫秒,免疫墙钟跳变
	turnStart := time.Now()
	if _, err := p.client.Call("turn/start", startParams); err != nil {
		return nil, nil, fmt.Errorf("turn/start: %w", err)
	}

	// 4) 等结束。不能只干等:codex 对未知 threadId 会静默吞掉 turn/start
	// (无报错、无事件),90 秒收不到任何通知就判失败,让手机端看到明确报错。
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	waitDone := false
	for !waitDone {
		select {
		case <-done:
			waitDone = true
		case <-ctx.Done():
			_, _ = p.client.Call("turn/interrupt", map[string]string{"threadId": codexTID})
			// 手机断连也要交回已流出的内容:server 层落库后,重连才能接上看
			mu.Lock()
			partial := streamText.String()
			mu.Unlock()
			if partial != "" {
				return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: partial}}}}, &pb.Usage{}, ctx.Err()
			}
			return nil, nil, ctx.Err()
		case <-tick.C:
			// 审批等待是合法静默(用户在手机上想多久都行),看门狗跳过
			if !approvalWaiting.Load() {
				// 单调口径:ms 与判定两侧共用 turnStartRef,差值即静默时长,
				// 免疫墙钟 NTP 跳变。ms>=0:初喂在等待循环前必然发生,旧 ms>0
				// 防的"未喂值"态已不可达;且喂值恰落在参照同一毫秒(0 值)时
				// 不再被误判成未喂 —— 判定恒生效,与旧实现行为一致。
				if ms := lastNotif.Load(); ms >= 0 && time.Since(turnStartRef).Milliseconds()-ms > 150_000 {
					_, _ = p.client.Call("turn/interrupt", map[string]string{"threadId": codexTID})
					mu.Lock()
					partial := streamText.String()
					mu.Unlock()
					if partial != "" {
						return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: partial}}}}, &pb.Usage{}, fmt.Errorf("codex 150 秒无任何事件(任务可能未被接收,thread=%s)", codexTID)
					}
					return nil, nil, fmt.Errorf("codex 150 秒无任何事件(任务可能未被接收,thread=%s)", codexTID)
				}
			}
			if time.Since(turnStart) > 60*time.Minute {
				// 超时必须真的打断 codex,否则子进程继续跑、turnMu 之外的算力被白白占用
				_, _ = p.client.Call("turn/interrupt", map[string]string{"threadId": codexTID})
				log.Printf("[codex] 轮次超时(60 分钟),已打断 thread=%s", codexTID)
				mu.Lock()
				partial := streamText.String()
				mu.Unlock()
				if partial != "" {
					return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: partial}}}}, &pb.Usage{}, fmt.Errorf("codex 超时(60 分钟)")
				}
				return nil, nil, fmt.Errorf("codex 超时(60 分钟)")
			}
		}
	}
	log.Printf("[codex] 一轮完成 thread=%s 用时 %v 回复 %d 字",
		codexTID, time.Since(turnStart).Round(time.Millisecond), streamText.Len()+finalText.Len())

	mu.Lock()
	defer mu.Unlock()
	if lastErr != nil {
		return nil, nil, &codexRunError{info: lastErr}
	}
	text := streamText.String()
	if text == "" {
		text = finalText.String()
	}
	// 【为什么空就是空】此前这里塞一句 "(完成,无文本输出)" 当**正文**发出去,把
	// 三种完全不同的情况压成同一条普通 assistant 文本:
	//   1. 真的没话说;2. 只产出了非文本产物;3. 适配器把正文弄丢了。
	// 用户看到的都是"任务完成 + 一句占位",无从判断该不该重试(图二)。
	// 现在如实返回空块,由 server 层按 blocks 派 OutputState;补正文的责任落在
	// "确有正文却没拿到"那一档,走原生历史补偿。
	if text == "" {
		return nil, &pb.Usage{}, nil
	}
	blocks := []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}}}}
	return blocks, &pb.Usage{}, nil
}

// isReasoningDeltaMethod 兼容 Codex 的 camelCase 通知名。
//
// Codex 当前会发 item/reasoning/summaryTextDelta 与
// item/reasoning/textDelta。原实现直接查找小写 "delta"，而这两个
// 方法名的结尾是大写 "Delta"，导致思考事件被静默丢弃。
func isReasoningDeltaMethod(method string) bool {
	m := strings.ToLower(method)
	return strings.Contains(m, "reasoning") && strings.Contains(m, "delta")
}

// ListCodexThreadsViaAppServer 用 app-server 的 thread/list 拿真实标题
// (codex 自动命名的 name 字段;rollout 文件里没有,只有首条消息 preview)。
func (p *CodexProvider) ListCodexThreadsViaAppServer() ([]CodexSessionInfo, error) {
	if err := p.withClient(func(*codexapp.Client) error { return nil }); err != nil {
		return nil, err
	}
	var out []CodexSessionInfo
	cursor := ""
	for page := 0; page < 5; page++ {
		params := map[string]interface{}{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := p.client.Call("thread/list", params)
		if err != nil {
			return out, err
		}
		// 【实测纠正】result.data 是数组(不是 {items:[...]} —— "items" 是
		// JSON Schema 里数组元素的声明,不是字段名)。这个写法曾经解析失败,
		// 而失败被下面的 rollout 扫描兜底吃掉,所以"加载电脑上已有会话"一直
		// 用的是文件扫描的标题,而不是 codex 自动命名的真实标题。
		var res struct {
			Data []struct {
				ID        string  `json:"id"`
				Name      *string `json:"name"`
				Preview   string  `json:"preview"`
				Cwd       string  `json:"cwd"`
				UpdatedAt int64   `json:"updatedAt"`
			} `json:"data"`
			NextCursor *string `json:"nextCursor"`
		}
		if json.Unmarshal(raw, &res) != nil {
			return out, fmt.Errorf("thread/list 应答无法解析: %s", clip(string(raw), 200))
		}
		for _, it := range res.Data {
			title := ""
			if it.Name != nil && *it.Name != "" {
				title = *it.Name
			} else if it.Preview != "" {
				r := []rune(it.Preview)
				if len(r) > 40 {
					r = r[:40]
				}
				title = string(r)
			}
			out = append(out, CodexSessionInfo{
				ID: it.ID, Title: title, Cwd: it.Cwd,
				UpdatedAt: time.Unix(it.UpdatedAt, 0),
			})
		}
		if res.NextCursor == nil || *res.NextCursor == "" || len(res.Data) == 0 {
			break
		}
		cursor = *res.NextCursor
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Warmup 启动即拉起 codex 子进程并完成 initialize(消除首条消息/MCP 的冷启动等待)。
func (p *CodexProvider) Warmup() {
	_ = p.withClient(func(*codexapp.Client) error { return nil })
}

// DesktopPrefs 是手机设置页各项偏好的**最终生效值快照**。
//
// 【为什么用一个结构体而不是一堆散参数】
//  1. presence 合并由 server 层完成(只有客户端显式 set 的字段才写进来),
//     所以这里空串是**有意义的取值**(跟随桌面 / 默认),provider 侧一律照单全收,
//     绝不做"非空才覆盖" —— 否则"改回跟随桌面"永远回不来(实测过的真实 bug)。
//  2. 以后加设置项:这个结构体 + server 的合并 + 手机端 UI 表各加一处,
//     不必写 8 段重复的 set/read 代码。
type DesktopPrefs struct {
	NativeMode        string
	ApprovalPolicy    string
	Sandbox           string
	Model             string
	Effort            string
	HideReasoning     bool
	Verbosity         string
	ReasoningSummary  string
	DisablePasteBurst bool
}

// ApplyPrefs 用一份**完整快照**覆盖内存态(server 层已按 presence 合并完毕)。
//
// 【红线】这里只改内存,绝不写 ~/.codex/config.toml。
//
// 【为什么没有 config/value/write —— 别再把它加回来】
// 第 2 轮实测(见 daemon/cmd/wbprobe):codex app-server 的 config/value/write
// 有两个硬约束,合起来等于"此路不通":
//  1. `mergeStrategy` 是 schema 必填字段,不传直接报 missing field mergeStrategy;
//  2. 传了 mergeStrategy 之后,只要 `filePath` 不是用户 config.toml,
//     就报 "Only writes to the user config are allowed"。
//
// 也就是说:**它唯一能写的就是 ~/.codex/config.toml** —— 正是本项目的红线。
// 以前那段代码"看起来没出事",仅仅因为缺 mergeStrategy、调用永远失败;
// 一旦有人"顺手补全参数",就会立刻违反红线。所以整条路径删除,而不是修好它。
//
// 真正让设置在 codex 侧生效的路径(全部不落盘、不碰用户配置):
//   - approvalPolicy / sandboxPolicy / model / effort / summary → 每轮 turn/start;
//   - model_verbosity / model_reasoning_summary → thread/start、thread/resume 的
//     `config` 覆盖对象(实测被 app-server 接受)。
func (p *CodexProvider) ApplyPrefs(pref DesktopPrefs) {
	p.setMu.Lock()
	defer p.setMu.Unlock()
	p.approvalPolicy = pref.ApprovalPolicy
	p.sandbox = pref.Sandbox
	p.desktopModel = pref.Model
	p.desktopEffort = pref.Effort
	p.hideAgentReasoning = pref.HideReasoning
	p.modelVerbosity = pref.Verbosity
	p.modelReasoningSummary = pref.ReasoningSummary
	p.disablePasteBurst = pref.DisablePasteBurst
}

// Prefs 读回当前内存态(仅供测试与调试回显;生产链路七项设置经 PrefsAware
// 广播(ApplyPrefs 灌入、轮次参数透传),不经此结构)。
func (p *CodexProvider) Prefs() DesktopPrefs {
	p.setMu.Lock()
	defer p.setMu.Unlock()
	return DesktopPrefs{
		ApprovalPolicy:    p.approvalPolicy,
		Sandbox:           p.sandbox,
		Model:             p.desktopModel,
		Effort:            p.desktopEffort,
		HideReasoning:     p.hideAgentReasoning,
		Verbosity:         p.modelVerbosity,
		ReasoningSummary:  p.modelReasoningSummary,
		DisablePasteBurst: p.disablePasteBurst,
	}
}

// HideReasoning 是否隐藏推理过程。
//
// 【实现层次说清楚】这里返回 true 时,Run() 的 reasoning/delta 分支**不再向手机
// 推送** ThinkingDelta —— 这是 daemon 侧的行为改变,不依赖 codex 是否原生支持。
// 客户端另有一个纯显示开关 AgentConnection.showThinking(ChatScreen 用它决定
// 要不要渲染思考卡);两者叠加时以本开关为准,客户端在写入后会把 showThinking
// 同步为一致值,避免出现"两种开关打架"。
func (p *CodexProvider) HideReasoning() bool {
	p.setMu.Lock()
	defer p.setMu.Unlock()
	return p.hideAgentReasoning
}

// HideReasoningFor 是 HideReasoning 的线程感知版:返回"这个线程这一轮该不该
// 隐藏思考流"。
//
// 【实现层次说清楚】返回 true 时,Run() 的 reasoning/delta 分支**不再向手机
// 推送** ThinkingDelta —— 这是 daemon 侧的行为改变,不依赖 codex 是否原生支持。
// 客户端另有一个纯显示开关 AgentConnection.showThinking(ChatScreen 用它决定
// 要不要渲染思考卡);两者叠加时以本开关为准,客户端在写入后会把 showThinking
// 同步为一致值,避免出现"两种开关打架"。
//
// 【当前口径:全局优先(2026-10-02 核实钉死)】快照字段 TurnSettings.HideReasoning
// 虽然随快照落库,但**不参与判定** —— 无论有无快照,一律以当前全局开关为准。
// 这是有测试钉住的既定行为(TestHideReasoningIsGlobalAcrossSnapshots,本文件族
// codex_snapshot_test.go),且 pi/zcode 的 hideReasoningFor 同款实现、同款全局优先
// (三家一致;"切换快照语义"必须三家一起动,此处不单边改道)。
// 快照真正生效的七项见 Run 的装配段(approvalPolicy/sandbox/model/effort/summary)
// 与 ConfigOverrideFor(verbosity/reasoning_summary)。
func (p *CodexProvider) HideReasoningFor(t *store.Thread) bool {
	return p.HideReasoning()
}

// ConfigOverrideFor 是 ConfigOverride 的线程感知版:有快照的线程按快照里的
// verbosity / reasoning_summary 生成 thread/start、thread/resume 的 config 覆盖,
// 旧线程(快照 nil)回退全局值。键集与 ConfigOverride 相同(只放 codex 确实
// 支持的两个键),空串不放。
func (p *CodexProvider) ConfigOverrideFor(t *store.Thread) map[string]interface{} {
	if t != nil && t.SettingsSnapshot != nil {
		out := map[string]interface{}{}
		if v := t.SettingsSnapshot.Verbosity; v != "" {
			out["model_verbosity"] = v
		}
		if v := t.SettingsSnapshot.ReasoningSummary; v != "" {
			out["model_reasoning_summary"] = v
		}
		return out
	}
	return p.ConfigOverride()
}

// ConfigOverride 返回 thread/start、thread/resume 的 `config` 覆盖对象。
//
// 【只放 codex 确实支持的键】schema 里只有 model_verbosity / model_reasoning_summary
// 是有效配置键;hide_agent_reasoning / disable_paste_burst 在**本版本 codex 的
// schema 里根本不存在**,塞进去只会被拒。空值一律不放,避免给 codex 传空串。
func (p *CodexProvider) ConfigOverride() map[string]interface{} {
	p.setMu.Lock()
	defer p.setMu.Unlock()
	out := map[string]interface{}{}
	if p.modelVerbosity != "" {
		out["model_verbosity"] = p.modelVerbosity
	}
	if p.modelReasoningSummary != "" {
		out["model_reasoning_summary"] = p.modelReasoningSummary
	}
	return out
}

// ---- 反问卡能力宣告(只发现、不启用) ----
//
// 【背景】codex 在 Default 协作模式下按模式门控 request_user_input 工具。早期
// 为了"解锁"它,daemon 曾在 thread/start、thread/resume 的 config 覆盖里注入
// features.default_mode_request_user_input=true。该特性在 codex 0.155.1 仍是
// underDevelopment(默认关),注入即触发 codex 的"Under-development features
// enabled"警告 —— 真机实测每次 codex 会话都弹,体验极差,正是本次报障的根因。
//
// 【为什么不再注入 · 红线】该警告的官方抑制途径只有改写用户桌面
// ~/.codex/config.toml(把 suppress_unstable_features_warning 设为 true)。
// 改写用户文件是本项目永久封死的副作用,绝不可为。而 schema 的 Config 对象
// (codex_app_server_protocol.v2.schemas.json → ConfigReadResponse.Config)并
// 未把 suppress_unstable_features_warning 列为可在运行时 config 覆盖里使用的
// 合法键(只有 additionalProperties:true 的宽松约束,且 codex.exe 的警告文案明确
// 指向"写进 config 文件"),即在"config 覆盖"通道内没有无副作用的抑制手段。
// 因此勾选该实验特性必然弹警告、且无法在协议内抑制 —— 必须选择"诚实地不提供",
// 而不是"提供了但每次弹警告"(后者被本项目红线明确禁止)。
//
// 【代价与配合】停用后,Default 模式下反问卡(request_user_input)不再可用,手机端
// 若仍摆着反问入口、用户一点就踩回最初那个"unavailable in default mode"的拒绝。
// 所以能力是否可用改为**运行时宣告**:AskUserInputSupported() 仅据
// experimentalFeature/list 探测 codex 是否宣告该特性(按 gen 缓存、绝不写死版本),
// 手机端/android 据此决定显不显示反问入口 —— 宣告为可用(未来 codex 把它从
// underDevelopment 毕业、或换非实验路径)时再恢复入口,现状则干脆不显示。
const codexAskUserFeature = "default_mode_request_user_input"

// AskUserInputSupported 只做一件事:报告"当前 codex 子进程是否在 experimentalFeature/list
// 里宣告了反问卡特性"。它是**纯能力发现**,绝不写入任何 config、绝不启用特性。
// 返回值供手机端决定是否显示反问入口(见 android 侧与 codex_requests.go)。
//
// 判定按 experimentalFeature/list 的运行时宣告、并按子进程代号(gen)缓存:旧版本
// codex 没有这个方法、或注册表不再宣告该特性时返回 false。这里只做动态发现,
// 不做版本号二分 —— 与项目"按运行时宣告判定、不写死版本"的约定一致。
func (p *CodexProvider) AskUserInputSupported() bool {
	gen := int(p.gen.Load())
	p.askFeatMu.Lock()
	defer p.askFeatMu.Unlock()
	if p.askFeatGen == gen {
		return p.askFeatVal
	}
	val := false
	if p.client != nil {
		raw, err := p.client.Call("experimentalFeature/list", map[string]interface{}{})
		if err == nil {
			val = askUserFeatureAdvertised(raw)
			log.Printf("[codex] experimentalFeature/list: %s 宣告=%v(仅用于能力宣告,不注入)", codexAskUserFeature, val)
		} else {
			// 旧版 codex 无此方法:如实记录,按"未宣告"处理。
			log.Printf("[codex] experimentalFeature/list 失败(%v),反问卡按不可用处理", err)
		}
	}
	p.askFeatGen = gen
	p.askFeatVal = val
	return val
}

// askUserFeatureAdvertised 解析 experimentalFeature/list 应答,判定 codex 是否
// 仍在注册表里宣告反问卡特性。已移除(removed)/弃用(deprecated)的阶段不算
// 宣告 —— 注册表会保留已移除特性的条目(实测 unavailable_dummy_tools 即为
// stage=removed 的墓碑),照注入就是对着墓碑开开关。
func askUserFeatureAdvertised(raw json.RawMessage) bool {
	var resp struct {
		Data []struct {
			Name  string `json:"name"`
			Stage string `json:"stage"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &resp) != nil {
		return false
	}
	for _, f := range resp.Data {
		if f.Name == codexAskUserFeature {
			return f.Stage != "removed" && f.Stage != "deprecated"
		}
	}
	return false
}

// callThreadResume 封装 thread/resume 调用。
//
// 【历史】这里曾内含"反问卡特性覆盖"降级护栏(带 features 覆盖失败 → 摘掉 features
// 重试),那是专为"注入 under-development 特性"设的逃生舱。现已不再注入任何特性
// (见 AskUserInputSupported 注释),无 features 键可摘,故该护栏整段删除,只保留
// 单纯的 resume + config 透传。
//
// 【为什么仍单独成函数】Run 的 resume 路径与退化到 thread/start 共用同一段"带 config
// 的 resume"逻辑,抽出来避免两处各写一遍、形状漂移。cfgOverride 全程只读。
//
// call 形同 p.client.Call(method, params),用函数参数注入便于单测(无需真实子进程)。
func (p *CodexProvider) callThreadResume(threadID string, cfgOverride map[string]interface{}, call func(method string, params interface{}) (json.RawMessage, error)) error {
	resumeParams := map[string]interface{}{"threadId": threadID}
	if len(cfgOverride) > 0 {
		resumeParams["config"] = cfgOverride
	}
	if _, err := call("thread/resume", resumeParams); err != nil {
		return err
	}
	return nil
}

// Ready 探测 app-server 是否真的可用(会按需拉起子进程并 initialize)。
//
// 【为什么要它】设置写入后必须能回答"到底打没打到 codex"。以前所有失败都只
// log.Printf,App 侧一无所知。这里把"能不能连上 codex"变成一个可判断的错误,
// 由 server 层转成 setting_status 里的 failed 状态回报给手机。
func (p *CodexProvider) Ready() error {
	return p.ReadyCtx(context.Background())
}

// ReadyCtx 是带 ctx 的探测。ProbeHook 非空时用它(测试注入"永远超时/立即报错"),
// 否则走真实探测。
//
// 【为什么要有 ctx 版本】设置页探测整段(Ready + Account + Requirements)必须
// 共用一个短上界;没有 ctx 出入口的话,每一小步各自等满默认上界,合计就失控。
func (p *CodexProvider) ReadyCtx(ctx context.Context) error {
	if p.ProbeHook != nil {
		return p.ProbeHook(ctx)
	}
	return p.withClientCtx(ctx, clientInitBudget, func(context.Context, *codexapp.Client) error { return nil })
}

// 【原 McpStatus() 已删除】它返回 []map[string]interface{},只带 name/tools,
// 而 authStatus / serverInfo / 错误原因全被丢掉了 —— 于是"某个 MCP 用不了"在手机上
// 完全看不出来。它还被 McpStatusDetailed 完全覆盖(后者按 MCP 状态详情建模)。
// 留着一份行为不同的同名旧实现,只会让下一个人改错那一个。

// Account 从模型列表里取登录账号(形如 "Codex · xxx@qq.com" 的账号组条目)。
func (p *CodexProvider) Account() string {
	return p.AccountCtx(context.Background())
}

// AccountCtx 是带 ctx 的账号读取(设置页探测与它共用同一个短上界)。
func (p *CodexProvider) AccountCtx(ctx context.Context) string {
	// 按下标取址遍历,避免 range 复制 protobuf 消息(内嵌 MessageState 含锁)。
	ms := p.ModelsCtx(ctx)
	for i := range ms {
		m := &ms[i]
		if strings.Contains(m.DisplayName, "·") {
			return m.DisplayName
		}
	}
	return ""
}

func mustJSON(v interface{}) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// lastModelSeg 取模型 slug:剥掉 "codex/" 前缀与账号组分段;default 表示跟随桌面,返回空。
func lastModelSeg(m string) string {
	m = strings.TrimPrefix(strings.TrimSpace(m), "codex/")
	if m == "default" {
		return ""
	}
	return m
}

// turnImage 是待发给 codex 的一张本地图片。
//
// 【为什么不直接传 .pocketagent-uploads 的路径】协议层(ImageBlock)只携带图片
// 字节与 MIME,不带落盘路径;但 codex app-server 与 daemon 同机,把字节落成本机
// 临时文件后直传绝对路径即可(v2 schema 的 UserInput::LocalImage;live 实测
// codex 接受该形状,且回复内容与图片内容一致),无需 base64。
type turnImage struct {
	data      []byte
	mediaType string
}

// turnInput 构造 turn/start 的 input 数组。
//
// 【为什么单独抽出来,且里面**什么都不做**】
// "粘贴长文本时不应中断、不应被截断"是用户明确要求的真实能力:手机端可能一次
// 粘贴几万字的代码、日志或报错。任何形式的裁剪(len 上限、按行截断、"太长就丢弃")
// 都会静默丢内容 —— 用户看到的现象是"agent 好像没读到我后半段",极难定位。
//
// 所以这里只做**原样封装**:完整送达是硬约定,不是"尽力而为"。
// 单测 TestTurnInputKeepsLongPasteIntact 用 20 万字输入钉住它,防止以后有人
// 顺手加个上限。
//
// (顺带说明:codex 侧的"粘贴爆破检测"属于它的终端输入层,我们走 stdio JSON-RPC,
// 不经过那一层,因此也不存在"需要先关掉某个开关才能粘贴"的前置条件。)
func turnInput(text string) []map[string]interface{} {
	items, _ := turnInputWithImages(text, nil, true) // 无图:不会有临时文件,cleanup 为空操作
	return items
}

// turnInputWithImages 构造带图片的 turn/start input 数组。
//
// 【形状】文本 1 块(原样封装,零裁剪约定同上)+ 每张图 1 个
// {"type":"localImage","path":<本机绝对路径>} 块(schema v2:UserInput::LocalImage)。
//
// 【能力门控】imageOK=false(模型 inputModalities 明确不含 image,如 glm-5.3 类
// 纯文本模型)时不发图片块,降级为文本占位 "[图片:<文件名>]" + 日志 —— codex 至少
// 知道"用户发了张图",也不会收到自己处理不了、进而在 turn/start 报错的输入。
//
// 【临时文件生命周期】图片字节在发送前落成本机临时文件(codex 处理 turn/start 时
// 才真正读文件),返回的 cleanup 删除它们;调用方须在 turn 结束后调用。turn 终局
// 即删对后续 resume 也安全:rollout 里图片以 input_image 的 dataURL 编码留档,
// resume 重放历史不按路径重读原文件(实测依据见 Run 内图片输入分支的注释)。
func turnInputWithImages(text string, imgs []turnImage, imageOK bool) ([]map[string]interface{}, func()) {
	items := []map[string]interface{}{{"type": "text", "text": text}}
	paths := make([]string, 0, len(imgs))
	for i := range imgs {
		// 门控不过:连临时文件都不落,直接占位(省一次 IO)。
		if !imageOK {
			name := fmt.Sprintf("image-%d%s", i+1, imageExt(imageMediaType(&imgs[i])))
			log.Printf("[codex] 模型不支持图片输入,图片 %s 降级为文本占位", name)
			items = append(items, map[string]interface{}{"type": "text", "text": "[图片:" + name + "]"})
			continue
		}
		path, name, err := materializeImage(&imgs[i], i)
		if err != nil {
			// 单张图落盘失败不能卡死整轮:降级为占位并留日志。
			log.Printf("[codex] 图片落盘失败(降级为文本占位): %v", err)
			items = append(items, map[string]interface{}{"type": "text", "text": "[图片:" + name + "]"})
			continue
		}
		items = append(items, map[string]interface{}{"type": "localImage", "path": path})
		paths = append(paths, path)
	}
	cleanup := func() {
		for _, p := range paths {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				log.Printf("[codex] 清理图片临时文件失败 %s: %v", p, err)
			}
		}
	}
	return items, cleanup
}

// materializeImage 把一张图片字节落成本机临时文件,返回(绝对路径, 文件名)。
// 文件名形如 pocketagent-img-<序号>-<随机串><扩展名>,扩展名按 MIME 推断
// (codex 侧靠扩展名识别图片类型)。
func materializeImage(img *turnImage, idx int) (string, string, error) {
	f, err := os.CreateTemp("", fmt.Sprintf("pocketagent-img-%d-*%s", idx+1, imageExt(imageMediaType(img))))
	if err != nil {
		return "", fmt.Sprintf("image-%d.png", idx+1), err
	}
	name := filepath.Base(f.Name())
	if _, err := f.Write(img.data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", name, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", name, err
	}
	return f.Name(), name, nil
}

// SweepStaleImageTemps 启动清扫:materializeImage 的临时文件正常由 Run 结束时
// 的 imgCleanup 收尾,但 daemon 被 taskkill /F 或崩溃时收不到 defer,残片会在
// %TEMP% 无限积累(Windows 不自动清)。daemon 启动时对"早于本次启动"的残留
// 做一次幂等清扫 —— 在役文件必然创建于本次进程启动之后,不会误删。
func SweepStaleImageTemps() {
	if n := sweepImageTemps(os.TempDir(), time.Now()); n > 0 {
		log.Printf("[codex] 启动清扫:移除历史残留图片临时文件 %d 个", n)
	}
}

func sweepImageTemps(dir string, started time.Time) int {
	paths, err := filepath.Glob(filepath.Join(dir, "pocketagent-img-*"))
	if err != nil {
		return 0
	}
	removed := 0
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil || !fi.ModTime().Before(started) {
			continue
		}
		if rmErr := os.Remove(p); rmErr == nil {
			removed++
		} else if !os.IsNotExist(rmErr) {
			log.Printf("[codex] 清理历史图片临时文件失败 %s: %v", p, rmErr)
		}
	}
	return removed
}

// imageMediaType 兜底补全 MIME:协议层没带时按内容嗅探(前 512 字节,png/jpeg/
// gif/webp 都认得);实在认不出按 image/png 处理,成败交由 codex 侧读字节决定。
func imageMediaType(img *turnImage) string {
	if img.mediaType != "" {
		return img.mediaType
	}
	if mt := http.DetectContentType(img.data); strings.HasPrefix(mt, "image/") {
		return mt
	}
	return "image/png"
}

// imageExt 由 MIME 推断扩展名。
func imageExt(mediaType string) string {
	switch mediaType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

// codexSupportsImageInput 查 model/list,判断本轮实际使用的模型是否声明支持图片输入。
//
// 【数据源】v2 schema Model.inputModalities("text"/"image"/"audio";schema 默认
// ["text","image"])。live 实测(2026-09-19):model/list 各模型均报 [text image]。
//
// 【判定不了的都放行】列表拉不到(model/list 失败/超时)、找不到 slug 对应条目、
// 条目没报 inputModalities —— 一律放行:此时发图最坏是 codex 报错(用户可见),
// 而静默降级会让"明明支持图的模型永远收不到图"且无感知。只有**明确声明不含
// image** 才降级为占位。slug 为空 = 跟随桌面默认,按 isDefault 条目判定。
//
// 【为何独立短上界、不随 runCtx】门控发生在 turn/start 之前,5s 拉不到列表就放行,
// 不让"查询能力"拖延或卡死正常轮次。
func (p *CodexProvider) codexSupportsImageInput(slug string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := p.client.CallCtx(ctx, "model/list", map[string]interface{}{})
	if err != nil {
		log.Printf("[codex] model/list 失败,图片输入按放行处理: %v", err)
		return true
	}
	var res struct {
		Data []struct {
			Id              string   `json:"id"`
			IsDefault       bool     `json:"isDefault"`
			InputModalities []string `json:"inputModalities"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &res) != nil || len(res.Data) == 0 {
		return true
	}
	for i := range res.Data {
		m := &res.Data[i]
		match := (slug == "" && m.IsDefault) || (slug != "" && m.Id == slug)
		if !match {
			continue
		}
		if len(m.InputModalities) == 0 {
			return true
		}
		for _, mod := range m.InputModalities {
			if mod == "image" {
				return true
			}
		}
		log.Printf("[codex] 模型 %q inputModalities=%v 不含 image,图片将降级为文本占位", slug, m.InputModalities)
		return false
	}
	return true // 列表里没有该模型:放行
}

func (p *CodexProvider) codexThreadRef(id string) (threadRef, bool) {
	p.threadMu.Lock()
	defer p.threadMu.Unlock()
	ref, ok := p.threadMap[id]
	return ref, ok
}
func (p *CodexProvider) setCodexThreadRef(id string, ref threadRef) {
	p.threadMu.Lock()
	defer p.threadMu.Unlock()
	p.threadMap[id] = ref
}
