// pi.go —— PiProvider:把 PocketAgent 的 gRPC 协议桥接到 pi --mode rpc 子进程。
//
// 【讲解】事件对照表(手机上看到的 ← Pi 发来的,0.85.1 实测):
//
//	文字逐字蹦出      message_update/text_delta
//	思考过程          message_update/thinking_delta
//	"⚙ bash"卡片     toolcall_end → ToolCallUpdate(STARTED)
//	卡片流式进度      tool_execution_update → ToolCallUpdate(UPDATE,累积值原地替换)
//	卡片出结果        tool_execution_end → ToolCallUpdate(FINISHED)
//	审批弹卡          extension_ui_request(confirm/select)→ 经 approve hook
//	压缩/重试/排队    compaction_*/auto_retry_*/queue_update → CodexEvent
//	一轮真正结束      agent_settled(【不是】agent_end —— 后面可能还有重试/压缩/排队)
//	用量              message_update 终帧(0.87.1+,探针实证)→ 事件驱动推送;
//	                  get_session_stats 收尾定稿 + 旧版(≤0.85.x 流式恒 0)轮询兜底
//
// 【进程模型】Pi 一个子进程 = 一个会话(与 codex app-server 不同),所以
// Thread ↔ 子进程 1:1:clients[threadID] 各自独立、可并行;daemon 重启后按
// Thread.ExternalSession.SessionFile 重挂载(switch_session + get_state 校验,
// 调研 §8.3,握手不能省 —— codex 那边"不 resume 就静默吞请求"是同款教训)。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/piapp"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// Pi 会话级参数(对齐 codex 的同名纪律):
const (
	// piCallBudget 单条 RPC 命令的等待上界(prompt 受理、get_state 等都该秒回;
	// switch_session 加载大会话文件可能慢,单独放宽)。
	piCallBudget = 30 * time.Second
	// piSwitchBudget switch_session 的等待上界(MB 级会话文件解析)。
	piSwitchBudget = 60 * time.Second
	// piSettleWait abort 后等 agent_settled 的上限(abort 应答本身就在 settled 之后)。
	piSettleWait = 30 * time.Second
	// piUIBudget 扩展 dialog 审批等手机应答的上界:没有它的请求会永久挂起整轮
	// (实测:无 timeout 的 confirm 一直等 —— 调研 §4.6)。
	piUIBudget = 120 * time.Second
	// piToolClip 工具输出单帧限长(rune),UPDATE 帧高频到达,限制防流拥堵。
	piToolClip = 8000
	// modelsTTL 模型列表缓存(Pi 模型列表要拉一个子进程,不能每次都冷启)。
	modelsTTL = 10 * time.Minute
	// piUsagePushInterval 轮次进行中用量**轮询兜底**的间隔。pi ≤0.85.x 的流式
	// usage 恒 0、get_session_stats 是唯一真相:不定期推,轮次进行中(尤其长任务)
	// 手机端上下文窗口卡就恒为 0%(真机实锤,2026-09-19)。5s 足够"会话中可见",
	// 又不至于让 get_session_stats 的 RPC 频率成为负担。
	// 【0.87.1 起(探针实证,docs/verify/evidence/pi-087-usage-probe.log)】
	// message_update 的最后一个增量帧自带本条消息的真实 usage,Run 的事件泵据此
	// 即时推送(piUsageTracker):一旦见过流式 usage,本轮询自动歇脚 —— 它只服务
	// 旧版 pi(流式恒 0,observed 恒假),保留为旧版兜底。
	piUsagePushInterval = 5 * time.Second
)

// Pi 轮次看门狗阈值(P1-7 补齐四家同口径三闸:codex/zcode/dsh/hermes 均为
// 150s 静默 + 60min 单轮上限 + 审批豁免,pi 此前是唯一缺失的一家 —— 子进程
// 活着但 LLM/事件流停摆时 Run 永久挂起且占住 running[t.ID],该会话后续轮次全被
// PI_BUSY 拒,只能重启 daemon)。var(非 const)仅为让测试可收紧阈值,生产勿改。
var (
	// piSilenceWatchdog 无任何 pi 事件的静默上界。审批等待豁免:手机 dialog
	// (piUIBudget=120s)与未知请求确认在飞时不计静默 —— 120s < 150s,豁免位
	// 是双保险,口径与 codex/dsh/zcode/hermes 一致。
	piSilenceWatchdog = 150 * time.Second
	// piTurnCap 单轮总时长上限(静默豁免救不了"慢而不断"的轮)。
	piTurnCap = 60 * time.Minute
)

// piWatchdogTick 看门狗巡检周期(检查只是取时差,便宜;5s 足够)。
const piWatchdogTick = 5 * time.Second

// PiProvider 桥接 pi --mode rpc。
type PiProvider struct {
	textOnly bool
	// mu 保护 clients/running/runningCtx/modelsCache。
	mu         sync.Mutex
	clients    map[string]*piapp.Client   // threadID → 子进程(1 会话 1 进程)
	running    map[string]bool            // threadID → 有轮次在跑(同线程防重入)
	runningCtx map[string]context.Context // threadID → 当前轮的 Run ctx(P2-2:重入守卫判断"旧轮已被打断"用)

	// steerMu 保护 active/settled/fuSeen(可插话目标、终局标记与原生队列水位;
	// Run 登记/收尾清除,投递与对账路径并发读写,与 codex_steer 同模式)。
	steerMu sync.Mutex
	active  map[string]bool
	// settled[threadID] 本轮是否已见过 agent_settled(FollowUp 的守卫:settled 后
	// pi 已空闲,prompt 会立即执行成新轮次而非排队 —— 那不是 followUp 语义)。
	settled map[string]bool
	// fuSeen[threadID] 原生 followUp 队列的上一水位(queue_update 事件对账用,
	// 见 pi_followup.go reconcileNativeQueue;跨 Run 存续,队列本体在 pi 进程内存)。
	fuSeen map[string]int

	modelsCache []pb.ModelInfo
	modelsAt    time.Time

	// fetchMu/fetch 模型探测的单飞状态(见 fetchModelsSingleFlight):并发调用
	// 共享同一次一次性子进程探测,绝不重复拉起。
	fetchMu sync.Mutex
	fetch   *piModelsFetch

	// guardPath 守护扩展文件(危险命令强制审批,见 piapp/guard.go)。
	guardPath string

	// cursors 每线程断点续传游标(threadID → 最近一次从 Pi 拿到的条目 id,
	// 调研 §8.2),与 clients/running 同受 mu 保护。内存版:daemon 重启后游标
	// 归零 → 首次重挂载跳过回放、轮次收尾重新建立(随 store 落库可后置)。
	cursors map[string]string

	// prefsMu 保护桌面偏好内存态(ApplyPrefs 与事件/轮次路径并发读写)。
	// 【与 codex 的 setMu 同纪律】只存内存,绝不写 Pi 侧用户配置。
	prefsMu sync.Mutex
	prefs   DesktopPrefs

	// remount 只读路径(设置目录)的按需重挂载钩子(单测注入假读取器,不拉真
	// 子进程;nil = 生产路径走 ensureMounted。与 dsh 的 newClient 注入同模式)。
	remount func(t *store.Thread) (stateReader, error)

	// sessionFor 原生会话适配器(pi_session.go)与 ForkThread(forkMount)共用的
	// 挂载客户端源:生产为 nil,走 clientFor(冷启/复用真子进程);单测与契约套件
	// 注入假客户端,不拉真子进程(与上方 remount 钩子、clearQueueOn 的窄接口打桩
	// 同模式)。
	sessionFor func(t *store.Thread) (piMountClient, error)

	// TitleHandler 标题同步钩子:Pi 会话名变化时回调(server 落库,双端一致)。
	// P1 只在挂载时把 thread.Title 写给 Pi(set_session_name),回调预留。
	TitleHandler func(threadID, title string)

	// uiInFlight 在飞的"等手机应答"计数(dialog 审批 + 未知请求确认,两处都在
	// Run 生命周期内)。看门狗的静默闸据此豁免:审批等待期间 pi 合法地不出事件,
	// 不能当停摆杀掉(与 codex/dsh/zcode/hermes 的审批豁免同口径)。原子量:
	// 生产侧(事件泵)与看门狗 tick 跨 goroutine 读写。
	uiInFlight atomic.Int64
}

// NewPiProvider 构造;守护扩展写到临时目录(固定名,覆盖写,daemon 启动即生效)。
func NewPiProvider() *PiProvider {
	p := &PiProvider{
		clients:    map[string]*piapp.Client{},
		running:    map[string]bool{},
		runningCtx: map[string]context.Context{},
		active:     map[string]bool{},
		settled:    map[string]bool{},
		fuSeen:     map[string]int{},
		cursors:    map[string]string{},
	}
	if dir := os.TempDir(); dir != "" {
		path := filepath.Join(dir, "pocketagent-pi-guard.ts")
		if err := os.WriteFile(path, []byte(piapp.RenderGuardExtension(piapp.GuardRules)), 0o644); err == nil {
			p.guardPath = path
		} else {
			log.Printf("[pi] 守护扩展写入失败(危险命令强制审批不可用): %v", err)
		}
	}
	return p
}

// CollabReady 见 CollabAware。判据用 CollabEnvInjection 而非裸开关:它已含
// "自建 pi agent 目录存在"这道硬前提 —— 目录不存在时 env 不会注入,pi 子进程
// 仍走用户自己的目录、看不到 collab MCP,此时不该对外宣称具备协同能力。
func (p *PiProvider) CollabReady() bool { return piapp.CollabEnvInjection() != "" }

func (p *PiProvider) ID() string          { return "pi" }
func (p *PiProvider) DisplayName() string { return "Pi(桌面 agent)" }

// ensureCollabMcp 会话创建前的协作 MCP 接线(pi 侧半步,T-FU)。
//
// 总开关开启时:确保 daemon 自有 pi agent 目录就绪(SetupCollabAgentDir,幂等,
// 未装 adapter 才真跑 pi install),再把本 daemon 自身注册为其 pocketagent-collab
// MCP 服务器 —— 命令 = os.Executable()(daemon 自己),参数 ["-collab-mcp"] 指向
// cmd/pocketagentd 的 -collab-mcp 子模式(pi 拉起的就是本 daemon)。
//
// 【与 cmd 侧的分工】cmd/pocketagentd 的 -pi-collab flag → SetCollabEnabled 归
// T-L2wire(cmd/ 本任务不可改);这里只认 piapp.CollabEnabled() 的最终状态,补上
// "env PA_PI_COLLAB 等价路径(不经 flag)"与"pi 会话子进程拉起前就绪"两个缺口。
// 【容错纪律】失败仅日志,绝不阻塞 pi 主功能 —— SetupCollabAgentDir/WriteCollabMcpConfig
// 的 error 一律不向上传播;下次会话创建时自然重试(均幂等)。
func (p *PiProvider) ensureCollabMcp() {
	if !piapp.CollabEnabled() {
		return
	}
	dir := piapp.ResolveCollabAgentDir()
	if err := piapp.SetupCollabAgentDir(dir); err != nil {
		log.Printf("[pi-collab] WARNING: 协作目录初始化失败(pi 主功能不受影响,下次重试): %v", err)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		log.Printf("[pi-collab] WARNING: 无法定位当前可执行文件,跳过协作 MCP 配置写入: %v", err)
		return
	}
	if err := piapp.WriteCollabMcpConfig(dir, exe, []string{"-collab-mcp"}); err != nil {
		log.Printf("[pi-collab] WARNING: 写入协作 MCP 配置失败(pi 主功能不受影响): %v", err)
		return
	}
	log.Printf("[pi-collab] 协作 MCP 已就绪: agent 目录 %s | 服务器 %s -collab-mcp", dir, exe)
}

// ---- 模型列表 ----

// Models 手机端"模型·强度"胶囊的列表。
//
// 【必须以 Pi 实际返回为准,不能写死】本机只登录了 zai-coding-cn(调研 §2)。
// 拉列表要起一个 --no-session 的一次性子进程(几秒),TTL 缓存兜住频率;
// 失败时至少保住 "pi/default"(跟随 Pi 当前模型,与 codex 的 default 同语义)。
func (p *PiProvider) Models() []pb.ModelInfo {
	p.mu.Lock()
	if len(p.modelsCache) > 0 && time.Since(p.modelsAt) < modelsTTL {
		out := p.modelsCache
		p.mu.Unlock()
		return out
	}
	p.mu.Unlock()

	out := []pb.ModelInfo{{
		Id: "default", Provider: "pi", DisplayName: "Pi(跟随 Pi 当前配置)",
		Available: true,
	}}
	if cached := p.fetchModels(); len(cached) > 0 {
		out = append(out, cached...)
	}

	p.mu.Lock()
	if len(out) > 1 {
		p.modelsCache = out
		p.modelsAt = time.Now()
	}
	p.mu.Unlock()
	return out
}

// fetchModels 用一次性 --no-session 子进程拉 get_available_models。
//
// 【并发去重(2026-09-25 弹窗治理专项)】Models() 的 TTL 缓存挡不住"同一瞬间
// 并发进来的一批调用"—— 手机端连发 ListModels 时,缓存判空与写回之间无互斥,
// 实测一秒内拉起过 3 个一次性 pi 子进程(daemon.log 18:17:52-53)。单飞包裹:
// 在飞期间的所有调用共享同一次探测结果,一个探测周期最多一个子进程。
func (p *PiProvider) fetchModels() []pb.ModelInfo {
	return p.fetchModelsSingleFlight(p.fetchModelsUncached)
}

// piModelsFetch 是一次模型探测的在飞状态(结果先写后关 done,等待方看到
// done 关闭时结果必然可见)。
type piModelsFetch struct {
	done   chan struct{}
	models []pb.ModelInfo
}

// fetchModelsSingleFlight 把 do 包装成并发去重:第一个调用者执行 do,在飞期间
// 的其余调用者等待并共享其结果。测试注入 do 以钉住"N 个并发调用只跑 1 次"。
func (p *PiProvider) fetchModelsSingleFlight(do func() []pb.ModelInfo) []pb.ModelInfo {
	p.fetchMu.Lock()
	if cur := p.fetch; cur != nil {
		p.fetchMu.Unlock()
		<-cur.done
		return cur.models
	}
	f := &piModelsFetch{done: make(chan struct{})}
	p.fetch = f
	p.fetchMu.Unlock()

	f.models = do()
	close(f.done)

	// 只清自己名下的在飞槽:若已有新调用换上了新的 f,不要误删别人的。
	p.fetchMu.Lock()
	if p.fetch == f {
		p.fetch = nil
	}
	p.fetchMu.Unlock()
	return f.models
}

// fetchModelsUncached 是模型探测的真实实现(无去重,经 fetchModelsSingleFlight
// 调用;名字里 uncached 对齐 fetchUsageUncached 的既有命名)。
func (p *PiProvider) fetchModelsUncached() []pb.ModelInfo {
	c, err := piapp.Start(piapp.StartOptions{ExtraArgs: []string{"--no-session"}})
	if err != nil {
		log.Printf("[pi] 模型列表探测失败: %v", err)
		return nil
	}
	defer c.Close()
	out := probePiModels(c, PiModelsStoreFallbackModels)
	// 【死因日志去冤案(2026-09-25 弹窗治理专项)】探测结束后 defer Close 会 Kill
	// 子进程,daemon.log 随之出现"退出(Close 主动关闭) 退出码=1"—— 曾被误读成
	// "0.87.1 rpc 拉起即退"挂进 AUDIT 适配债。这里留一行人话收尾,与退出行相邻,
	// 后人不再重复排查(退出码 1 = TerminateProcess 的固定码,与崩溃无关)。
	// 真表与 models-store 回退两种来源都走这一行收尾,措辞用"候选"兼容二者。
	if len(out) > 0 {
		log.Printf("[pi] 模型列表探测收尾(%d 个候选):一次性子进程 %d 即将按设计关闭,其后的退出码=1 是 Kill 收尸码非崩溃", len(out), c.PID())
	}
	return out
}

// 模型探测的传输预算(分相):单次尝试上界 + 总上界交给就绪重试。
const (
	// piModelsProbeAttempt 单次尝试的等待上界,与 piapp 的 piReadyAttempt 同值
	// 同义(未应答换 id 重试,直到总预算耗尽/拿到定局;两处本地声明,注释互指
	// —— 不为读一个常量把 piapp 的内部预算升成公共面)。
	piModelsProbeAttempt = 12 * time.Second
	// piModelsProbeBudget 探测总上界(ctx deadline)。【8s→30s(2026-10-07,pi
	// 1.0.2 对齐)】1.0.2 的 runRpcMode 在扩展加载完成后才开始读 stdin(启动
	// 门控,见 piapp.callWithReadyRetry):spawn 后首条命令可能躺在管道里数十秒
	// (本机重扩展实测 >15s),旧 8s 单次预算大概率空超时、白白落 models-store
	// 回退 —— 而 1.0.2 的 get_available_models handler 实际存在(bundle 实证),
	// 拿不到真表纯属传输时机。30s 给"重扩展绑定完成后拿到真表"留足机会;
	// 拿不到时回退链照旧,不劣化。
	piModelsProbeBudget = 30 * time.Second
)

// piModelsProbeCaller 模型探测对 piapp.Client 的窄接口(单测打桩,不拉真子进程;
// piapp.Client 的 CallWithReadyRetry 天然满足,生产零适配 —— 与 piMountClient
// 的抽接口同模式)。
type piModelsProbeCaller interface {
	CallWithReadyRetry(ctx context.Context, typ string, params map[string]any, attemptBudget time.Duration) (json.RawMessage, error)
}

// 生产零适配的编译期保证:真客户端的方法集必须天然满足窄接口。
var _ piModelsProbeCaller = (*piapp.Client)(nil)

// probePiModels 就绪重试发 get_available_models 并按既有语义折叠成最终候选:
//   - 拿到真表 → 解析返回;
//   - RPC 失败(1.0.2 门控下总预算耗尽仍未就绪 / success:false / 子进程死亡)
//     → fallback()(models-store 只读回退,不劣化 —— 0.99.1 的 --no-session 下
//     这条命令永不应答(cmd/pimodelsprobe 探针溯源),回退链正是为它而生);
//   - 应答不可解析 → 返回 nil 且**不**回退(原生形状漂移,宁缺毋假,与
//     2026-10-01 以来"解析失败不走回退"的既有分支一致)。
//
// fallback 抽成参数是注入缝:单测用临时 models-store 文件钉"超时必回退"
// (生产传 PiModelsStoreFallbackModels,与探针 cmd/pimodelsprobe 同源)。
func probePiModels(c piModelsProbeCaller, fallback func() []pb.ModelInfo) []pb.ModelInfo {
	ctx, cancel := context.WithTimeout(context.Background(), piModelsProbeBudget)
	defer cancel()
	raw, err := c.CallWithReadyRetry(ctx, "get_available_models", nil, piModelsProbeAttempt)
	if err != nil {
		log.Printf("[pi] get_available_models 未取得真表(未就绪/超时),回退实证候选: %v", err)
		return fallback()
	}
	var res struct {
		Models []struct {
			ID               string                     `json:"id"`
			Name             string                     `json:"name"`
			Provider         string                     `json:"provider"`
			Reasoning        bool                       `json:"reasoning"`
			Input            []string                   `json:"input"`
			ContextWindow    int64                      `json:"contextWindow"`
			ThinkingLevelMap map[string]json.RawMessage `json:"thinkingLevelMap"`
		} `json:"models"`
	}
	if json.Unmarshal(raw, &res) != nil {
		return nil
	}
	var out []pb.ModelInfo
	for _, m := range res.Models {
		// 直接 append 复合字面量:pb 消息含互斥量,append 已有值会被 vet 拒绝
		out = append(out, pb.ModelInfo{
			Id:               m.Provider + "/" + m.ID, // Thread.ModelID 形如 "pi/zai-coding-cn/glm-5.3"
			Provider:         "pi",
			DisplayName:      m.Name,
			Available:        true,
			ContextWindow:    int32(m.ContextWindow),
			SupportsThinking: m.Reasoning,
			AvailableEfforts: piModelThinkingLevels(m.Reasoning, m.ThinkingLevelMap),
			SupportsImages: func() bool {
				for _, in := range m.Input {
					if in == "image" {
						return true // 实测 glm-5.3 input=["text"]:App 据此关图片入口
					}
				}
				return false
			}(),
		})
		if m.Name == "" {
			out[len(out)-1].DisplayName = m.ID
		}
	}
	return out
}

// piModelsStorePath 返回 pi 自维护模型目录文件的标准路径(只读用途)。
func piModelsStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".pi", "agent", "models-store.json")
}

// ---- 会话挂载 ----

// clientFor 取(或建)该线程的子进程,并完成重挂载握手。
//
// 【握手不能省】ExternalSession.SessionFile 与子进程当前会话不符时必须
// switch_session + get_state 校验;新会话则从 get_state 回填 SessionFile/SessionID
// (server 层轮次结束后落库,daemon 重启全靠它)。
func (p *PiProvider) clientFor(t *store.Thread, workspace string) (*piapp.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.clients[t.ID]
	if c != nil && c.Alive() {
		return c, nil
	}
	if c != nil {
		c.Close()
		delete(p.clients, t.ID)
	}
	opts := piapp.StartOptions{
		Cwd:            workspace,
		Name:           t.Title,
		GuardExtension: p.guardPath,
	}
	if p.textOnly {
		opts.GuardExtension = ""
		opts.ExtraArgs = []string{"--no-session", "--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates"}
	}
	// 协作 MCP 接线(pi 侧半步,T-FU):开关关闭(默认)时零行为变化。必须排在
	// Start 之前 —— Start 内的 PI_CODING_AGENT_DIR 注入(piapp.CollabEnvInjection)
	// 以"目录已初始化"为硬前提,先就绪目录与 mcp.json,首个子进程才能带上。
	// 首次开启时的 pi install 可能分钟级且 clientFor 持锁串行:一次性成本,
	// 之后幂等跳过(stat 级)。
	p.ensureCollabMcp()
	nc, err := piapp.Start(opts)
	if err != nil {
		return nil, err
	}
	nc.SetUIRequestHandler(func(req *piapp.UIRequest) (map[string]any, bool) {
		// 无轮次上下文的 dialog:失败安全,拒绝(正常路径的审批在 Run 内接线)。
		return map[string]any{"cancelled": true}, true
	})
	nc.SetUnknownResponseHandler(func(rawID json.RawMessage, raw json.RawMessage) {
		recordUnknownResponse("pi", string(rawID), raw)
	})
	// 标题回调:Pi 的 set_session_name(我们挂载时下发,或电脑端 pi -r 同进程改名)
	// 都会发 session_info_changed 事件(0.85.1 实测:{"name":"...","type":
	// "session_info_changed"})。用常驻 GlobalHandler 接住它 → TitleHandler →
	// server 落库,双端标题一致(与 codex/zcode 的 TitleHandler 同语义)。
	// 【并发纪律】GlobalHandler 跑在 readLoop 里:只做解析 + go 出去,不阻塞;
	// Setter 在锁内立即发布快照，不依赖后续 CallCtx 才让 readLoop 看见它。
	// 挂载时回显的 set_session_name 会带 t.Title 原值回来,server 侧有
	// "标题相同就不写库"守卫,不会形成回调环。
	threadID := t.ID
	// 【B4 坏帧留痕(2026-10-09)】pi 事件边界对信封二解失败(GlobalHandler 的
	// frame 解包,此前失败时降级成"空 method + 空 payload"的台账条目,不可检索)
	// 统一走共享 recordBadFrame,只台账不出卡:此处是常驻边界、无轮次 sink 可用,
	// 且 piapp 对非 JSON 行在上游直接丢弃(不入任何钩子),能到达本边界的帧外层
	// 必为合法 JSON,解包失败属罕见畸形;限频记账随该子进程生命周期(口径见
	// native_notifications.go 的 recordBadFrame 注释)。
	var badFrames nativeNoticeRecorder
	nc.SetGlobalHandler(func(ev piapp.Event) {
		var method string
		var payload json.RawMessage
		var frame struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(ev.Raw, &frame) != nil {
			badFrames.recordBadFrame("pi", "global", ev.Raw, nil)
			return
		}
		method, payload = frame.Type, frame.Data
		recordProviderEvent("pi", method, threadID, piEventNoticeKind(method), payload)
		if name, ok := MapSessionNameChange(ev.Raw); ok && p.TitleHandler != nil {
			go p.TitleHandler(threadID, name)
		}
	})
	p.clients[t.ID] = nc
	return nc, nil
}

// DropThread 删除线程时回收该线程的 pi 子进程与断点游标(P2-1:pi 一个子进程
// 就是一个会话,手机删掉 pi 会话后若 piapp.Client 留在 clients,进程会一直空转
// 到 daemon 重启 —— 进程+游标泄漏)。Close 幂等;线程从没挂载过子进程时静默
// 返回。server 层 DeleteThread 经最小接口 piThreadDropper 调用(同
// zcodeSessionCloser 的抽接口模式,单测可注 fake)。
func (p *PiProvider) DropThread(threadID string) {
	p.mu.Lock()
	c := p.clients[threadID]
	delete(p.clients, threadID)
	delete(p.cursors, threadID)
	p.mu.Unlock()
	p.forgetNativeQueue(threadID) // 水位/终局标记随子进程一起回收(原生队列随进程消亡)
	if c != nil {
		c.Close()
		log.Printf("[pi] 已回收线程 %s 的子进程与游标", threadID)
	}
}

// piMountClient 是 mount 握手与 fork 序列对 piapp.Client 的窄接口(单测打桩,
// 不依赖子进程;piapp.Client 的方法集天然满足,生产零适配 —— 与 sendSessionName
// 的 sessionNameSetter、advanceEntryCursor 的 entriesFetcher 同款模式)。抽它出来
// 是为了让原生会话适配器(pi_session.go)复用 mount 时可用假客户端钉握手行为,
// 而 ForkThread(forkMount)复用它钉"clone → 取新身份 → Close 回收"的唯一序列,
// 而不是在两条路径里各写第二实现。
type piMountClient interface {
	SessionFile() string
	SessionID() string
	GetState(ctx context.Context) (map[string]any, error)
	SwitchSession(ctx context.Context, sessionFile string) error
	Call(typ string, params map[string]any, timeout time.Duration) (json.RawMessage, error)
	// Close 子进程回收(DropThread/fork 的关闭回收用;mount 不调)。
	Close()
}

// 生产零适配的编译期保证:真客户端的方法集必须天然满足窄接口。
var _ piMountClient = (*piapp.Client)(nil)

// mount 完成会话挂载:重挂载(switch+校验)或新建(get_state 回填)。
func (p *PiProvider) mount(ctx context.Context, t *store.Thread, c piMountClient) error {
	want := t.ExternalSession.SessionFile
	if want != "" {
		if cur := c.SessionFile(); cur != "" && strings.EqualFold(filepath.Clean(cur), filepath.Clean(want)) {
			return nil // 当前子进程已挂载目标会话
		}
		ctx2, cancel := context.WithTimeout(ctx, piSwitchBudget)
		defer cancel()
		if err := c.SwitchSession(ctx2, want); err != nil {
			return fmt.Errorf("重挂载 Pi 会话失败(%s): %w", filepath.Base(want), err)
		}
		return nil
	}
	// 新会话:取 sessionFile/sessionId 回填(落库由 server 层做)。
	ctx2, cancel := context.WithTimeout(ctx, piCallBudget)
	defer cancel()
	if _, err := c.GetState(ctx2); err != nil {
		return fmt.Errorf("获取 Pi 会话状态失败: %w", err)
	}
	t.ExternalSession = store.ExternalSessionRef{
		SessionID:   c.SessionID(),
		SessionFile: c.SessionFile(),
		Cwd:         t.Workspace,
	}
	// 会话名同步(双端一致:电脑端 pi -r 也看到这个名字 —— 调研 §6.4)。
	if t.Title != "" {
		_, _ = c.Call("set_session_name", map[string]any{"name": t.Title}, piCallBudget)
	}
	return nil
}

// applyModel 把 Thread.ModelID("pi/<provider>/<model>")与 effort 映射成
// set_model / set_thinking_level。失败只记日志:模型不可用是"降级继续",
// 不是"整轮失败"(Pi 会话本身有自己的当前模型)。
//
// 【1.0.2 set_model 必须 provider+modelId 二元组】pi handler 按
// find(m => m.provider === command.provider && m.id === command.modelId) 定位,
// 缺 provider 的旧形状({"modelId": spec})恒 "Model not found" → 每轮 applyModel
// 都失败 → 跨轮次静默回退旧模型(真机 15d/15e 实证)。改走 piapp.SetModel:
// 拆解与 pi_selection.go 先例同形;无 "/" 的遗留规格本地被拒,降级语义不变。
func (p *PiProvider) applyModel(ctx context.Context, c *piapp.Client, t *store.Thread) {
	spec := strings.TrimPrefix(t.ModelID, "pi/")
	if spec == "" || spec == t.ModelID || spec == "default" {
		// 不是 pi/ 前缀或 default:跟随 Pi 当前配置(探针确认 get_state.model 即当前)。
		return
	}
	if err := c.SetModel(ctx, spec); err != nil {
		log.Printf("[pi] set_model %s 失败(沿用当前模型): %v", spec, err)
	}
	if t.Effort != "" {
		if _, err := c.CallCtx(ctx, "set_thinking_level", map[string]any{"level": t.Effort}, piCallBudget); err != nil {
			log.Printf("[pi] set_thinking_level %s 失败: %v", t.Effort, err)
		}
	}
}

// ---- 桌面偏好(PrefsAware)----

// ApplyPrefs 实现 agent.PrefsAware:接收桌面设置页七项偏好的完整快照。
//
// 【消费哪些】HideReasoning → 事件路径过滤 ThinkingDelta(emitThinking);
// Model/Effort → 桌面默认模型/思考强度(applyDesktopDefaults,线程未显式指定
// 时每轮 Run 跟随)。【不映射哪些、为什么】审批策略/沙箱:Pi 的审批走守护扩展
// (guard.ts)与 dialog 确认,没有 approvalPolicy/sandboxPolicy 对应面 —— 这两项
// 仅对 codex 生效;Verbosity/ReasoningSummary/DisablePasteBurst 协议无对应面,
// 同样收下不消费。完整快照覆盖(照单全收),与 codex 的 ApplyPrefs 同纪律。
func (p *PiProvider) ApplyPrefs(pref DesktopPrefs) {
	p.prefsMu.Lock()
	defer p.prefsMu.Unlock()
	p.prefs = pref
}

// hideReasoning 读"隐藏推理过程"开关。
func (p *PiProvider) hideReasoning() bool {
	p.prefsMu.Lock()
	defer p.prefsMu.Unlock()
	return p.prefs.HideReasoning
}

// desktopDefaults 读桌面默认(模型, 思考强度);空串 = 未设置 = 跟随 Pi 当前配置。
func (p *PiProvider) desktopDefaults() (model, effort string) {
	p.prefsMu.Lock()
	defer p.prefsMu.Unlock()
	return p.prefs.Model, p.prefs.Effort
}

// emitThinking 在"隐藏推理"关闭时把思考增量推给 sink(与 codex.go 的
// HideReasoning() 分支同款 daemon 侧强制过滤;Pi 协议没有全局开关)。
// 【为什么抽成方法】Run 的事件泵闭包不便直接单测,把开关判断收敛到这里,
// 单测钉死"HideReasoning=true 时 ThinkingDelta 一帧不出"。
func (p *PiProvider) emitThinking(delta string, sink Sink) {
	if p.hideReasoning() {
		return
	}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Delta: delta}}})
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
func (p *PiProvider) hideReasoningFor(t *store.Thread) bool {
	return p.hideReasoning()
}

// desktopEffortFor 是 desktopDefaults 的线程感知面(P2-8,与 codex 的三级
// 优先级对齐):线程设置快照(SettingsSnapshot,新建对话那一刻的值)> 全局
// 偏好(旧线程兜底)。线程显式 Effort 的第一级由 applyDesktopDefaults 的
// level := t.Effort 处理;快照非 nil 时快照里的空串就是"当时未设置",保持
// 原值生效、不中途改抓新全局值。桌面默认模型不在此列(pi 消费的七项只有
// effort/hideReasoning,模型走 Thread.ModelID)。
func (p *PiProvider) desktopEffortFor(t *store.Thread) string {
	if t != nil && t.SettingsSnapshot != nil {
		return t.SettingsSnapshot.Effort
	}
	_, effort := p.desktopDefaults()
	return effort
}

// emitThinkingFor 是 emitThinking 的线程感知版(P2-8):按 hideReasoningFor 的
// 当前口径判定(全局优先,不读快照 —— 见上方 hideReasoningFor 注释),
// Run 的事件泵走这里;emitThinking 保留给无线程上下文的调用与既有单测。
func (p *PiProvider) emitThinkingFor(t *store.Thread, delta string, sink Sink) {
	if p.hideReasoningFor(t) {
		return
	}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Delta: delta}}})
}

// emitThinkingSnapshotFor 是 snapshot 帧(权威全文)的线程感知出口(P1-3):
// 与 emitThinkingFor 同一 hideReasoning 闸门 —— 隐藏推理时终局全文同样不出。
func (p *PiProvider) emitThinkingSnapshotFor(t *store.Thread, snapshot string, sink Sink) {
	if p.hideReasoningFor(t) {
		return
	}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Snapshot: snapshot}}})
}

// threadModelExplicit 线程是否显式指定了 pi 模型("pi/<provider>/<model>")。
// 空 / 无 pi/ 前缀 / "pi/default" 都算"跟随"(与 applyModel 的判定一致)。
func threadModelExplicit(t *store.Thread) bool {
	spec := strings.TrimPrefix(t.ModelID, "pi/")
	return spec != "" && spec != t.ModelID && spec != "default"
}

// applyDesktopDefaults 把桌面设置页的默认模型/思考强度下发到 Pi 会话。
// 由 Run 在 applyModel 之后调用(每轮开始,会话级状态随之刷新)。
//
// 【最小方案取舍】codex 的桌面默认是"每轮 turn/start 都带参数"自然覆盖;Pi 的
// set_model/set_thinking_level 是会话级状态、没有 per-turn 参数,所以选"每轮
// Run 开始时下发"这条最短路径(不用在会话创建处再加挂载钩子)。优先级:
// 线程显式指定 > 桌面默认 > 跟随 Pi 当前配置(不下发)。桌面默认模型只认
// "pi/..." 前缀 —— 它是全局默认,可能指向其他 provider(glm/codex…),指向
// 别家时 Pi 侧不下发、沿用 Pi 自己的当前模型。失败只记日志降级继续,与
// applyModel 同语义。
func (p *PiProvider) applyDesktopDefaults(ctx context.Context, c *piapp.Client, t *store.Thread) {
	model, _ := p.desktopDefaults()
	// 【P2-8】思考强度的"桌面默认"走线程快照语义:有快照的会话按快照装配,
	// 老会话不再随设置页全局漂移;线程显式 Effort 的第一级优先保持不变。
	effort := p.desktopEffortFor(t)
	if threadModelExplicit(t) {
		// 模型已被 applyModel 下发;线程没钉思考强度时由桌面默认兜底。
		if t.Effort == "" && effort != "" {
			p.setThinkingLevel(ctx, c, effort)
		}
		return
	}
	// 模型跟随:桌面默认是 pi 模型才下发,否则沿用 Pi 当前配置。
	// 【1.0.2】走 SetModel(provider+modelId 二元组),与 applyModel 同理。
	if ms := strings.TrimPrefix(model, "pi/"); ms != "" && ms != model && ms != "default" {
		if err := c.SetModel(ctx, ms); err != nil {
			log.Printf("[pi] 桌面默认 set_model %s 失败(沿用当前模型): %v", ms, err)
		}
	}
	// 思考强度:线程显式 > 桌面默认。【顺带补漏】applyModel 只在线程显式指定
	// pi 模型时才下发 t.Effort("跟随"分支提前 return),这里统一补上。
	level := t.Effort
	if level == "" {
		level = effort
	}
	if level != "" {
		p.setThinkingLevel(ctx, c, level)
	}
}

// setThinkingLevel 下发 set_thinking_level(失败只记日志,降级继续)。
func (p *PiProvider) setThinkingLevel(ctx context.Context, c *piapp.Client, level string) {
	if _, err := c.CallCtx(ctx, "set_thinking_level", map[string]any{"level": level}, piCallBudget); err != nil {
		log.Printf("[pi] set_thinking_level %s 失败: %v", level, err)
	}
}

// piEventNoticeKind 是 pi 事件分档入口:0.86/0.87 新增事件在此补归
// KNOWN_PASSTHROUGH,其余回落 native_notifications.go 的 piNoticeKind。
//
// 【为什么这里再包一层,而不直接扩 piNoticeKind】piNoticeKind 驻
// native_notifications.go,该文件本轮归 zcode 适配线所有(约定互不踩文件);
// 于是两份词表分工:**0.85.1 及以前的老事件**分档以那边为准,**0.86/0.87 新增
// 事件**以这里为准。判据与那边同款:事件在 pi 真实存在(0.87.1 探针抓到
// turn_start/turn_end/message_start/message_end,其余在 bundle 里有 emit 点,
// 证据 docs/verify/evidence/pi-087-usage-probe.log),daemon 只是没把它投影成
// TurnServerMessage —— 是取舍,不是"没见过",不能记成 UNKNOWN 淹没诊断页。
// 【extension_error 为什么也归已知】它带 extensionPath/event/error,留痕本身
// 就是诊断价值;Run 路径另有 MapStatusEvent 把它投影成 ERROR 卡,双路不冲突
// (本函数只服务 GlobalHandler/兜底留痕这两条记录路径)。
// 【0.99.1 增补(2026-09-30 探针实证 pi-0991-*.md + 随包 docs/json.md)】
//   - bash_execution_update:RPC bash 命令(留观未接)的流式输出块,id 关联命令;
//     daemon 不发 bash RPC,真出现也只可能来自桌面端同进程操作 —— 留痕归已知。
//   - thinking_level_changed:0.87.1 起就有(遗漏收编),Run 路径已消费
//     (InvalidateModels + Started 投影),此前 GlobalHandler/兜底路径却落 UNKNOWN;
//     0.99.1 事件全集复核后补归。
func piEventNoticeKind(eventType string) string {
	switch eventType {
	case "session_info_changed":
		// 2026-10-01 通宵采样批:标题同步是真实投影(GlobalHandler 里
		// MapSessionNameChange → TitleHandler → server 落库双端同步,与 zcode 的
		// session.titleUpdated → PROJECTED 同语义),词表 knowledge_pi.go 也早有
		// 词条 —— 但分类两表(piEventNoticeKind / piNoticeKind)都没档,活跃台账
		// 实测落了一条 UNKNOWN(unknown-events.json seq 1291)。补归 PROJECTED,
		// 别让"已知且已消费"的事件灌诊断页。
		return runtime.KindKnownProjected
	case "turn_start", "turn_end", "message_start", "message_end",
		"entry_appended", "summarization_retry_scheduled",
		"summarization_retry_attempt_start", "summarization_retry_finished",
		"extension_error", "bash_execution_update", "thinking_level_changed":
		return runtime.KindKnownPassthrough
	case "model_changed", "model_select", "state_changed":
		// 【1.0.2 审计 G6】三类不在 pi 1.0.2 事件全集里(AgentSessionEvent 并集
		// 无此三类,0.85.1 时代防御残留),Run 的 consume 却有真实投影分支
		// (InvalidateModels + emitRuntimeSelection → Started 帧)—— 旧分档漏收,
		// 一旦旧版 pi 回连,已投影的事件会被灌成 UNKNOWN 淹诊断页。分类依据是
		// "daemon 做了什么":有投影 → PROJECTED。
		return runtime.KindKnownProjected
	}
	return piNoticeKind(eventType)
}

// piLedgerOnlyEvent 判定"只落台账、不推 sink"的轮次骨架事件(0.87.1 探针实证,
// docs/verify/evidence/pi-087-usage-probe.log)。
//
// 【为什么 turn_start/message_start/message_end 三类不推 sink】
//  1. 载荷不可推:message_start/end 携带完整 messages 数组,其中 system prompt
//     全量 ~70KB —— 即便台账侧有 8KiB 截断,推送侧也是一帧巨大的 STATUS 卡,
//     手机端时间线被无意义的"系统提示词快照"顶开。
//  2. 轮次噪音:这三类每条消息/每轮必然各来一次,0.87.1 起经 consume 的兜底
//     分支变成"原生状态通知"卡(每方法限 2 条也只是把刷屏压成 6 卡/轮)——
//     真正的过程感已由 message_update/tool_execution_* 的正式投影承担,
//     骨架事件对手机端零增量信息,只淹内容。
//
// 【台账为什么保留】诊断索引必须如实反映"上游每轮都发这些",上游哪天改形状、
// 改频次,台账是第一个看见的地方(KNOWN_PASSTHROUGH 分类不变)。
func piLedgerOnlyEvent(eventType string) bool {
	switch eventType {
	case "turn_start", "message_start", "message_end":
		return true
	}
	return false
}

// ---- 一轮 ----

// acquireRunning 登记本轮为该线程的 running 轮(P2-2 重入闭环)。
//
// 【背景】server 层重入打断会取消旧轮的 Run ctx,但旧 Run 还要发 abort 并等
// agent_settled 收尾(最长 piSettleWait=30s)才清 p.running —— 期间新轮直接进
// 会吃 PI_BUSY,手机端表现为"打断后第一轮必失败"("返回后不工作"修复对 pi
// 不闭环)。现在:旧轮真在跑(runningCtx 未取消)→ 立即 PI_BUSY;旧轮已被打断
// (runningCtx 已取消)→ 有界轮询等 running 释放(上限 30s,与 abort 收尾的
// piSettleWait 同口径),等到即登记成功。
func (p *PiProvider) acquireRunning(threadID string, runCtx context.Context) error {
	p.mu.Lock()
	if !p.running[threadID] {
		p.running[threadID] = true
		p.runningCtx[threadID] = runCtx
		p.mu.Unlock()
		return nil
	}
	oldCtx := p.runningCtx[threadID]
	p.mu.Unlock()
	if oldCtx == nil || oldCtx.Err() == nil {
		return &piError{code: "PI_BUSY", msg: "该 Pi 会话已有一轮在执行"}
	}
	deadline := time.Now().Add(piSettleWait)
	for {
		p.mu.Lock()
		if !p.running[threadID] {
			p.running[threadID] = true
			p.runningCtx[threadID] = runCtx
			p.mu.Unlock()
			return nil
		}
		p.mu.Unlock()
		if time.Now().After(deadline) {
			return &piError{code: "PI_BUSY", msg: "上一轮打断收尾中,请稍后重试"}
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// piUnknownUIInFlightLimit 未知 extension_ui_request 透传确认的在飞上限:每条在飞
// 确认占一条 goroutine、各自以 piUIBudget(120s)为界,上游扩展异常刷屏时若无上界,
// goroutine 与手机端确认卡会在窗口内同步堆积。占坑失败(超限)的请求不排队,直接
// 回 pi 协议原生兜底拒绝(piRejectUnknownUIBusy)。上限盖过任何真实场景的并发未知
// 请求(正常交互是"一次一张卡"),只拦异常洪水。var 仅为缩时单测留缝,生产勿改。
var piUnknownUIInFlightLimit = int64(16)

// acquireUnknownUIInFlight 未知请求确认的在飞占坑:占坑成功(未超限)返回 true,
// 调用方负责在结束时 uiInFlight.Add(-1) 释放;超限返回 false 且**不占坑** ——
// Add 探到超限立即退坑,窗口内不留残坑(否则洪水过后上限永久缩水一个)。判定与
// 占坑是同一次 Add,无 check-then-act 竞态,第 limit+1 个必被拒。计数器 uiInFlight
// 本就存在(dialog 审批 + 未知请求确认共用,看门狗静默豁免位),这里只补上此前
// 缺失的上限判定。
func (p *PiProvider) acquireUnknownUIInFlight() bool {
	if p.uiInFlight.Add(1) > int64(piUnknownUIInFlightLimit) {
		p.uiInFlight.Add(-1) // 超限:立刻退坑,不占窗口
		return false
	}
	return true
}

// piRejectUnknownUIBusy 未知请求确认在飞超限的"忙"拒绝:回 pi 协议原生兜底拒绝
// {"cancelled":true}(pi 的 extension_ui_response 没有 -32601 错误码通道,cancelled
// 是协议内唯一的安全拒绝形状,与 consumePiUnknownUIRequest 拒绝路同款)+ 台账
// UNKNOWN_REQUEST 留痕 —— 备注写明是 daemon 侧容量拒绝(忙),不是用户决定,事后
// 排查不误判。id 缺失无法回写,只落台账(与 consumePiUnknownUIRequest 的 id 缺失
// 静默同口径,但"收到过洪水"要能事后查)。同步执行、不起 goroutine:调用点在
// 事件泵上,RespondUI 与其他协议写回一样是便宜的管道写。
func piRejectUnknownUIBusy(c piUIClient, raw json.RawMessage, threadID string) {
	var busy struct {
		ID     string `json:"id"`
		Method string `json:"method"`
	}
	_ = json.Unmarshal(raw, &busy)
	if busy.ID != "" {
		_ = c.RespondUI(busy.ID, map[string]any{"cancelled": true})
	}
	sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
		ThreadID:       threadID,
		SourceAgent:    "pi",
		SourceMethod:   busy.Method,
		NormalizedKind: runtime.KindUnknownRequest,
		RawPayload:     redactNativePayload(raw),
		Note:           "未知请求确认在飞超限,已回协议原生兜底拒绝 cancelled:true(daemon 容量拒绝,非用户决定;未替用户做任何审批决定)",
	})
}

// Run 执行完整一轮:发 prompt → 消费事件流 → agent_settled 收尾。
func (p *PiProvider) Run(ctx context.Context, req TurnRequest, sink Sink, approve ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	t := req.Thread

	// 同线程防重入(server 已有重入打断,这里是 provider 侧防御)。
	// 【P2-2】runCtx 提前构造:守卫要拿它登记本轮,并据此判断旧轮是否已被打断
	// (打断后等 abort 收尾释放,见 acquireRunning)。
	runCtx, runCancel := context.WithCancelCause(ctx)
	defer runCancel(nil)
	if err := p.acquireRunning(t.ID, runCtx); err != nil {
		return nil, nil, err
	}
	defer func() {
		p.mu.Lock()
		p.running[t.ID] = false
		delete(p.runningCtx, t.ID)
		p.mu.Unlock()
	}()

	// 登记"可插话"目标(Steer 用)并清"已终局"标记(FollowUp 的守卫位:每个新
	// Run 都是新一轮,上一轮的 settled 不再适用);轮次收尾统一清除 —— 迟到的
	// steer/followUp 不能投进早已结束的轮次(与 codex_steer 的 turnActive 守卫
	// 同语义)。
	p.steerMu.Lock()
	p.active[t.ID] = true
	p.settled[t.ID] = false
	p.steerMu.Unlock()
	defer func() {
		p.steerMu.Lock()
		delete(p.active, t.ID)
		delete(p.settled, t.ID)
		p.steerMu.Unlock()
	}()

	// 断线恢复点检测:clientFor 对死亡子进程做"关旧起新"(pi 侧唯一的重挂载
	// 路径)。prev 与 c 不同(或原先不存在)= 刚重启过,才可能有断线期间的事件
	// 落在会话文件里;沿用存活子进程时无断点可续。
	p.mu.Lock()
	prev := p.clients[t.ID]
	p.mu.Unlock()
	// 【P1-7 bindSession 上收】会话解析入口经原生会话适配器:客户端获取走
	// piSession.mountFor(与契约面 Open/Resume 同一来源 —— 生产 clientFor、
	// 测试 sessionFor 钩子),握手走 mount 唯一实现(两条链共用,TestPiResolveMount*
	// 钉住三分支);"过渡期双路径"就此退役。workspace 显式传 req 值,保留旧链取值
	// 语义(轮次工作区随 TurnRequest 给出,与 thread 落库值生产同源)。
	ns, nerr := p.NativeSessionFor(t)
	if nerr != nil {
		return nil, nil, &piError{code: "PI_UNAVAILABLE", msg: nerr.Error()}
	}
	ps, psok := ns.(*piSession)
	if !psok {
		return nil, nil, &piError{code: "PI_UNAVAILABLE", msg: fmt.Sprintf("pi: 原生会话适配器类型异常: %T", ns)}
	}
	mc, err := ps.mountFor(t, req.Workspace)
	if err != nil {
		return nil, nil, &piError{code: "PI_UNAVAILABLE", msg: err.Error()}
	}
	c, cok := mc.(*piapp.Client)
	if !cok {
		// sessionFor 钩子只服务于契约/单测打桩(窄接口),生产恒为真客户端;
		// 轮次内状态机(回放/用量/prompt)需要具体类型,类型不符如实报不可用。
		return nil, nil, &piError{code: "PI_UNAVAILABLE", msg: fmt.Sprintf("pi: 会话客户端不是生产子进程客户端: %T", mc)}
	}
	if err := p.mount(ctx, t, c); err != nil {
		return nil, nil, &piError{code: "PI_MOUNT_FAILED", msg: err.Error()}
	}
	// 断点续传(调研 §8.2):仅"子进程刚重启 + 已有会话文件"执行三态续传
	// (增量回放 / 游标失效全量兜底 / 游标推进)。回放只发 CodexEvent 摘要帧,
	// 不注入正文流 —— textBuf 会拼进本轮最终答案,不能把上一轮尾部混进来。
	if prev != c && t.ExternalSession.SessionFile != "" {
		p.replayMissed(ctx, c, t, sink)
	}
	if err := p.configureSelection(ctx, c, t, sink); err != nil {
		return nil, nil, err
	}

	// 用量基线(本轮增量 = stats - 基线;0.87.1 起流式帧另有 per-message usage
	// 事件驱动推送,基线仍承担"会话累计的减数"与窗口快照,收尾定稿依旧靠 stats)。
	baseline := p.statsQuiet(c)

	// ---- 审批接线:dialog 类 extension_ui_request → approve hook → 手机审批卡 ----
	//
	// 【断线/超时回 cancelled】approve 返回 err(server 断线 readerDone / ctx 超时
	// 120s)或拒绝,一律 cancelled:true,绝不默认放行(调研 §4.6);select 等
	// 用户真选(选项渲染进反问卡,答案经 note 通道回传),input/editor 带回输入文本。
	c.SetUIRequestHandler(func(uireq *piapp.UIRequest) (map[string]any, bool) {
		if !uireq.Dialog() {
			return nil, false // fire-and-forget(notify/setStatus/setWidget):不回写
		}
		actx, acancel := context.WithTimeout(runCtx, piUIBudget)
		defer acancel()
		// 内联构造传参:pb 消息按值传给接口会被 vet 拒绝,复合调用结果可以
		// 【看门狗豁免】等手机的这段时间 pi 合法地不出事件,登记在飞数让
		// 静默闸绕行(与 codex/dsh/zcode/hermes 的审批豁免同口径)。
		p.uiInFlight.Add(1)
		approved, note, aerr := approve(actx, ApprovalCallFromUI(uireq.ID, uireq.Method, uireq.Title, uireq.Message, uireq.Options, uireq.Placeholder, uireq.Prefill))
		p.uiInFlight.Add(-1)
		if aerr != nil {
			log.Printf("[pi] 审批 %s(%s)未取得手机决定,回 cancelled: %v", uireq.ID, uireq.Method, aerr)
			return map[string]any{"cancelled": true}, true
		}
		return UIResponseFromDecision(uireq.Method, approved, note), true
	})

	// ---- 事件泵 ----
	// readLoop 不能被慢 sink 阻塞:事件先入带缓冲 channel,单消费者保序处理
	// (与 codex.go 的 notifCh 同模式)。消费 goroutine 里完成"Pi 事件 → pb"映射。
	type done struct {
		settled bool
		aborted bool
	}
	notifCh := make(chan piapp.Event, 4096)
	notifStop := make(chan struct{})
	notifDone := make(chan struct{})
	// 看门狗时钟:turnStartRef 单调起点,lastEvent 最近一次 pi 事件到达时刻。
	// 打点在生产侧(readLoop 回调)—— 事件一到就算"活着",与 codex lastNotif
	// 同语义;wdMu 只护一个时间值的读写,纳秒级临界区(与消费侧 mu 分开,避免
	// 在 readLoop 热路径上碰文本缓冲的锁)。
	turnStartRef := time.Now()
	var wdMu sync.Mutex
	lastEvent := turnStartRef
	c.SetEventHandler(func(ev piapp.Event) {
		wdMu.Lock()
		lastEvent = time.Now()
		wdMu.Unlock()
		select {
		case notifCh <- ev:
		case <-notifStop:
		}
	})
	defer func() {
		close(notifStop)
		<-notifDone
		c.SetEventHandler(nil)
	}()

	var (
		mu      sync.Mutex
		textBuf strings.Builder // 最终正文(text_delta 拼接)
		started = map[string]bool{}
		// toolStart 各 callId 的执行开始时刻(daemon 实测):UPDATE 帧据此带
		// "已运行 X.Xs" 脚注、END 帧在 structuredContent 缺 wall_time 时兜底,
		// 对齐 pi TUI 工具卡的 Elapsed/Took 展示(见 pi_events.go piToolFooter)。
		toolStart = map[string]time.Time{}
		lastUsage *pb.Usage // pb 消息存指针:含互斥量,按值复制会被 vet 拒绝
		// compacting 压缩进行中(compaction_start/end 维护):中程用量推送暂停。
		// 手机端已由 CONTEXT_COMPACTED 事件进入"压缩中"状态并清空 turnUsage,
		// 若中程推送继续顶新数字进去,会把"压缩中"状态闪没(见下方 ticker)。
		compacting bool
	)

	var notices nativeNoticeRecorder
	// usageTrack 流式用量累计(0.87.1+ message_update 终帧;事件驱动推送的数据源,
	// 同时是"轮询兜底是否歇脚"与收尾兜底链的一环,见各使用点注释)。
	usageTrack := &piUsageTracker{}
	consume := func(ev piapp.Event) {
		if ev.Type == "model_changed" || ev.Type == "model_select" || ev.Type == "thinking_level_changed" || ev.Type == "state_changed" {
			p.InvalidateModels()
			emitRuntimeSelection(decodeRuntimeSelection("pi", ev.Type, ev.Raw), sink)
		}
		switch ev.Type {
		case "message_update":
			// 流式用量(0.87.1+,探针实证):每条 assistant 消息的最后一个增量帧
			// 自带真实 usage —— 入账并立即推 UsageUpdate(事件驱动,取代旧版
			// 5 秒轮询的"会话中可见";压缩闸门与 ticker 同款,压缩中不入屏)。
			if usageTrack.observe(ev.Raw) {
				mu.Lock()
				busy := compacting
				mu.Unlock()
				if !busy {
					sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Usage{
						Usage: usageUpdateFromStream(usageTrack.snapshot(), baseline),
					}})
				}
			}
			if d, ok := MapTextDelta(ev.Raw); ok {
				mu.Lock()
				textBuf.WriteString(d.Delta)
				mu.Unlock()
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &d}})
				return
			}
			if d, ok := MapThinkingDelta(ev.Raw); ok {
				// 隐藏推理过程(daemon 侧强制过滤,与 codex.go 同款):
				// 开关打开时 ThinkingDelta 一帧不出,见 emitThinkingFor
				// (P2-8:有快照的线程按快照判定,老会话不随全局漂移)。
				p.emitThinkingFor(t, d.Delta, sink)
				return
			}
			if snap, ok := MapThinkingSnapshot(ev.Raw); ok {
				// thinking_end 终局权威全文(P1-3):客户端整体替换累积文本,
				// 丢帧/断连后的兜底真相。与增量帧互斥(thinking_end 不带 delta)。
				p.emitThinkingSnapshotFor(t, snap.Snapshot, sink)
				return
			}
			if upd, ok := MapToolcallEnd(ev.Raw); ok {
				mu.Lock()
				started[upd.Call.Id] = true
				mu.Unlock()
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &upd}})
				return
			}
			// toolcall_start/toolcall_delta:参数拼装过程,STARTED 等 end(见 pi_events.go)
		case "tool_execution_start":
			mu.Lock()
			seen := started[toolCallIDOf(ev.Raw)]
			mu.Unlock()
			if seen {
				return
			}
			if upd, ok := MapToolExecStart(ev.Raw); ok {
				mu.Lock()
				started[upd.Call.Id] = true
				toolStart[upd.Call.Id] = time.Now()
				mu.Unlock()
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &upd}})
			}
		case "tool_execution_update":
			if upd, ok := MapToolExecUpdate(ev.Raw, piToolClip, piElapsedOf(&mu, toolStart, ev.Raw)); ok {
				// partialResult 是累积值:App 端按 callId 原地替换(UPDATE 相位)
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &upd}})
			}
		case "tool_execution_end":
			if upd, ok := MapToolExecEnd(ev.Raw, piToolClip, piElapsedOf(&mu, toolStart, ev.Raw)); ok {
				mu.Lock()
				delete(toolStart, upd.Call.Id)
				mu.Unlock()
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &upd}})
			}
		case "agent_end":
			// 每低级 run 结束(后面可能还有重试/压缩/排队续跑),不算终局;
			// 顺带取最新 usage(完整消息里才有真实用量)。
			if u := usageFromAgentEnd(ev.Raw); u != nil {
				mu.Lock()
				lastUsage = u
				mu.Unlock()
			}
		case "agent_settled":
			// 真终局:settled 之后不再有自动续跑(实测 §11#5)。
			// 【顺带钉终局标记】FollowUp 据此拒投:settled 后 pi 已空闲,再投
			// prompt 会立即执行成新轮次,不是 followUp 语义(见 FollowUp 守卫)。
			p.steerMu.Lock()
			p.settled[t.ID] = true
			p.steerMu.Unlock()
			runCancel(errSettled)
		case "queue_update":
			// 原生队列对账(入队 → Queued 出队 → QueueFlushed,手机排队计数
			// 继续正确);随后的状态卡照旧投影 —— 两条信息面各司其职,回执链
			// 管计数、状态卡管"插队 N 条 · 排队 M 条"的现场感。
			for _, frame := range p.reconcileNativeQueue(t.ID, ev.Raw) {
				sink(frame)
			}
			if cev, ok := MapStatusEvent(ev.Type, ev.Raw); ok {
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: cev}})
			}
		case "compaction_start", "compaction_end":
			// 维护压缩闸门(中程用量推送据此暂停,见下方 ticker),再按原路径
			// 映射成 CONTEXT_COMPACTED 状态事件上屏。
			mu.Lock()
			compacting = ev.Type == "compaction_start"
			mu.Unlock()
			if cev, ok := MapStatusEvent(ev.Type, ev.Raw); ok {
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: cev}})
			}
		default:
			// 【abort 为什么无专案分支】pi 0.99.1 事件全集已无 abort 事件
			// (2026-09-30 piprobe 实证),原"自身 abort 回执,无需处理"的
			// no-op 分支已删;若回滚到会发 abort 帧的旧 pi,落此处经
			// piEventNoticeKind 仍归 KNOWN_PASSTHROUGH,只落台账、不推屏、
			// 不淹诊断页(词表兜底见 runtime/knowledge_pi.go)。
			if ev.Type == "extension_ui_request" {
				// 【辅线 B】未知 method 的 UI 反向请求:开关 ON 时弹通用确认卡征求
				// 用户(允许→回写 {};拒绝/超时→回写原生兜底拒绝 cancelled)。
				// 必须异步:consume 跑在事件泵上,同步等手机会把整条事件管道堵住
				// (与 piapp dispatch 对 dialog 的 go 出去同理);已知 dialog 由上方
				// UIRequestHandler 专线处理、已知 fire-and-forget 无应答通道,两者
				// 在 consumePiUnknownUIRequest 里原样放行,维持静默忽略
				// (实测 26 次 setStatus 噪声)。
				// 【notify 例外(1.0.2 审计 G3)】notify 是扩展发给用户的即时通知
				// (info/warning/error),不属于"机器交互",静默会吞掉用户可见内容
				// —— 先映射成状态/警告卡上屏(不回写,fire-and-forget 无应答通道),
				// 其余 fire-and-forget(setStatus/setWidget/setTitle/set_editor_text
				// 是桌面 TUI 专属面)与未知 method 照旧走下方管道。
				if cev, ok := MapUINotify(ev.Raw); ok {
					sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: cev}})
					return
				}
				frame := append(json.RawMessage(nil), ev.Raw...)
				// 【看门狗豁免】等手机确认的这段时间同样登记在飞数(与 dialog
				// 审批同口径,预算同为 piUIBudget)。
				// 【in-flight 闸(2026-10-09)】此前这里无条件 go,上游扩展异常刷屏时
				// 确认 goroutine 无上界(每条各自 piUIBudget=120s 为界,窗口内照样
				// 堆积)。复用 uiInFlight 计数器补一道上限:同步先占坑(判定与占坑
				// 是同一次 Add,无 check-then-act 竞态),超限直接回"忙"拒绝 ——
				// pi 协议原生兜底 {cancelled:true}(失败安全形状,非用户决定)+ 台账
				// 留痕,不起 goroutine、不问手机,见 piRejectUnknownUIBusy。
				if !p.acquireUnknownUIInFlight() {
					piRejectUnknownUIBusy(c, frame, t.ID)
					return
				}
				go func() {
					defer p.uiInFlight.Add(-1)
					p.consumePiUnknownUIRequest(c, frame, t.ID, runCtx, approve)
				}()
				return
			}
			if piLedgerOnlyEvent(ev.Type) {
				// 0.87.1 噪音治理:轮次骨架事件只落台账、不推 sink(理由见
				// piLedgerOnlyEvent)。分类照旧走 piEventNoticeKind → KNOWN_PASSTHROUGH,
				// 治的是推送,不动台账。
				// 【例外:助手消息异常收尾】message_end 携带 stopReason=length/error
				// 时补一张 WARNING 卡上屏(pi TUI 会渲染 "Response was truncated
				// before completion."/"Error: msg",手机端此前对截断完全不可见);
				// 正常 stop/aborted 不发卡,台账照落。
				if cev, ok := MapAssistantStop(ev.Raw); ok {
					sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: cev}})
				}
				notices.recordLedgerOnly(piEventNoticeKind(ev.Type), "pi", ev.Type, ev.Raw)
				return
			}
			if cev, ok := MapStatusEvent(ev.Type, ev.Raw); ok {
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: cev}})
			} else {
				notices.recordAs(piEventNoticeKind(ev.Type), "pi", ev.Type, ev.Raw, sink)
			}
		}
	}
	go func() {
		defer close(notifDone)
		for {
			select {
			case ev := <-notifCh:
				consume(ev)
			case <-notifStop:
				return
			}
		}
	}()

	// ---- 发 prompt ----
	// 【走 Prompt 封装】行为与原内联 params 零分叉(behavior 为空 = 不带
	// streamingBehavior 字段);封装是 followUp 原生投递(pi_followup.go)与
	// 首轮 prompt 的同一传输口,字段形状只有一份真相。
	message, images := blocksToPrompt(req.InputBlocks)
	if err := c.Prompt(runCtx, message, images, ""); err != nil {
		// success:false = prompt 在受理前被拒(流式中未带 streamingBehavior、模型不可用…)
		// 受理后的失败走事件流(不会以第二条 response 出现 —— 文档明说)。
		return partialText(&mu, &textBuf), nil, &piError{code: "PI_PROMPT_REJECTED", msg: err.Error()}
	}

	// ---- 轮次中定期用量推送(轮询兜底) ----
	// 【图10 实锤】过去 UsageUpdate 只在收尾发一次:pi ≤0.85.x 的流式 usage 恒 0、
	// get_session_stats 是唯一真相,轮次进行中(尤其长任务)手机端上下文窗口卡
	// 恒为 "0% · 0/窗口"。现在用量主路径是事件驱动的(message_update 终帧 →
	// usageTrack 即时推送,见上方 consume);本 ticker 降级为**旧版兜底**:
	// 0.87.1+ 会话(observed=true)轮询纯冗余,自动歇脚;旧版(恒假)按原频率
	// 轮询,行为与 0.85.1 时代完全一致。usage=本轮增量(stats - baseline),
	// total=会话累计,窗口=contextUsage.contextWindow —— 与收尾帧同一套口径
	// (UsageUpdateFromStats)。
	// 【压缩期间暂停】compaction_start 后 stats 会先缩后涨,手机端正显示
	// "压缩中/已压缩"(turnUsage 已清):中程推送会把旧状态顶掉,故跳过;
	// 收尾推送不受影响(压缩后 stats 已定稿,即"新一轮用量上报")。
	// 【并发】sink 由 server 层串行化(thread.go 的 sendMu);get_session_stats
	// 走 piapp 的 pending 表,与事件流互不干扰(Steer 同款轮中调用)。
	usageStop := make(chan struct{})
	usageDone := make(chan struct{})
	var usageStopOnce sync.Once
	stopUsagePush := func() {
		usageStopOnce.Do(func() { close(usageStop) })
	}
	go func() {
		defer close(usageDone)
		ticker := time.NewTicker(piUsagePushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-usageStop:
				return
			case <-ticker.C:
				mu.Lock()
				busy := compacting
				mu.Unlock()
				if busy {
					continue
				}
				// 0.87.1+:流式 usage 已事件驱动推送,轮询纯冗余;旧版兜底照旧。
				if usageTrack.observed() {
					continue
				}
				cur := p.statsQuiet(c)
				if cur == nil {
					// stats 取不到(超时/子进程临终):保持沉默,绝不推全 0 冒充真实值。
					continue
				}
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Usage{Usage: UsageUpdateFromStats(cur, baseline)}})
			}
		}
	}()
	defer func() {
		stopUsagePush()
		<-usageDone
	}()

	// ---- 等 settled / 取消 / 子进程死亡(带看门狗,P1-7 补齐四家同口径)----
	// 旧实现裸 select:pi 子进程活着但 LLM/事件流停摆时永久挂起并占住
	// running[t.ID],该会话后续轮次全被 PI_BUSY 拒。现在 150s 无事件(审批豁免)
	// 或单轮超 60min 即判失败退出,与 codex/zcode/dsh/hermes 同口径。
	wdTick := time.NewTicker(piWatchdogTick)
	defer wdTick.Stop()
wdLoop:
	for {
		select {
		case <-runCtx.Done():
			break wdLoop
		case <-c.DeathCh():
			break wdLoop
		case <-wdTick.C:
			wdMu.Lock()
			sinceEvent := time.Since(lastEvent)
			wdMu.Unlock()
			switch piWatchdogVerdict(sinceEvent, time.Since(turnStartRef), p.uiInFlight.Load()) {
			case 1:
				runCancel(errPiSilence)
				break wdLoop
			case 2:
				runCancel(errPiTurnCap)
				break wdLoop
			}
		}
	}
	cause := context.Cause(runCtx)
	userStop := ctx.Err() != nil && cause != errSettled // 上层取消(手机取消帧 / CancelTurn / 重入打断)
	if cause == errSettled {
		// 正常终局
	} else if cause == errPiSilence || cause == errPiTurnCap {
		// 看门狗触发:如实终止本轮。先尽力走与用户中止同款的清理序列
		// (clear_queue+abort)—— 静默≠进程死,子进程多半还活着,不清理会话级
		// 运行态会残留;清理失败不阻塞返回,下轮 clientFor 对异常子进程重挂载兜底。
		actx, acancel := context.WithTimeout(context.Background(), piSettleWait)
		if _, _, cerr := stopTurnOn(actx, c); cerr != nil {
			log.Printf("[pi] 看门狗终止序列失败(下轮重挂载兜底) thread=%s: %v", t.ID, cerr)
		}
		acancel()
		if cause == errPiSilence {
			log.Printf("[pi] 看门狗: %s 内无任何事件且不在等审批 thread=%s", piSilenceWatchdog, t.ID)
			return partialText(&mu, &textBuf), nil, &piError{code: "PI_STALLED",
				msg: fmt.Sprintf("Pi %s 内无任何事件且不在等审批(子进程可能僵死),本轮终止;重发消息会自动重挂载会话", piSilenceWatchdog)}
		}
		log.Printf("[pi] 看门狗: 单轮超过 %s 仍未终局 thread=%s", piTurnCap, t.ID)
		return partialText(&mu, &textBuf), nil, &piError{code: "PI_TURN_TIMEOUT",
			msg: fmt.Sprintf("Pi 单轮超过 %s 仍未终局,本轮终止;重发消息会自动重挂载会话", piTurnCap)}
	} else if userStop {
		// 发终止序列并等它干净收尾。【顺序是 0.99.1 文档配方(rpc-commands.md
		// clear_queue/abort 两节)】"abort continues queued messages when they
		// remain in the session" —— abort 不清原生队列,反而让队列续跑;且
		// 0.99.1 的 abort 应答语义是"当前操作停了就回"(isIdle 只看运行态,不
		// 看队列)。旧实现先 abort 后 clear_queue:预投递进原生队列的 followUp
		// 会在 abort 之后被 pi 继续执行(收尾还排队跑,与"终止任务"语义相反),
		// clear_queue 到达时队首早已出队。改为先 clear_queue(摘掉插队+排队全
		// 部)再 abort(停当前操作,abort 内部连重试/压缩一并取消 —— bundle:
		// abortRetry()+abortCompaction()+agent.abort()+waitForIdle),队列空了
		// abort 之后无事可续,settled 干净到来。
		abortStart := time.Now()
		actx, acancel := context.WithTimeout(context.Background(), piSettleWait)
		steered, followUp, cerr := stopTurnOn(actx, c)
		// 清理成功 = 队列必空:水位直接归零,不依赖随后那条 queue_update
		// 是否赶在事件泵收尾前到达(赶到了也只是零增量对账)。失败只记
		// 日志:残留条目由 ClearQueuedInputs(用户显式撤回)或下轮清空兜底。
		if cerr != nil {
			log.Printf("[pi] 中止后清理原生排队失败(残留由显式撤回兜底) thread=%s: %v", t.ID, cerr)
		} else {
			p.steerMu.Lock()
			p.fuSeen[t.ID] = 0
			p.steerMu.Unlock()
		}
		acancel()
		log.Printf("[pi] 用户中止轮次 thread=%s(撤销插队 %d/排队 %d,abort 收尾 %s)",
			t.ID, steered, followUp, time.Since(abortStart).Round(time.Millisecond))
	} else {
		// 子进程死亡 / 协议外错误
		if !c.Alive() {
			return partialText(&mu, &textBuf), nil, &piError{code: "PI_CRASHED", msg: "Pi 子进程已退出,本轮终止(重发消息会自动重挂载会话)"}
		}
		return partialText(&mu, &textBuf), nil, &piError{code: "PI_STREAM_BROKEN", msg: "Pi 事件流中断"}
	}

	// ---- 收尾:停中程推送 + 推进续传游标 + 用量 + UsageUpdate ----
	// 先停 ticker 并等它退出,保证收尾的最终 UsageUpdate 是最后一帧
	// (中程帧与最终帧乱序会让手机端 turnUsage 回跳)。
	stopUsagePush()
	<-usageDone
	// 正常终局/用户中止(abort 收尾后)会话条目已定稿,是每轮一次的最省推进点
	// (RPC 流式事件本身不带条目 id,逐帧推进不成立 —— 实测)。崩溃路径已在
	// 上方提前 return:游标故意留在原地,下次重挂载的增量回放才覆盖崩溃前尾部。
	p.advanceEntryCursor(c, t.ID)
	final := p.statsQuiet(c)
	usage := UsageUpdateFromStats(final, baseline)
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Usage{Usage: usage}})

	mu.Lock()
	finalText := textBuf.String()
	mu.Unlock()
	if finalText == "" {
		// 全程无 text_delta(比如整轮只有工具调用被中止):兜底取 Pi 的最后回复
		if raw, err := c.Call("get_last_assistant_text", nil, piCallBudget); err == nil {
			var d struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(raw, &d) == nil {
				finalText = d.Text
			}
		}
	}
	blocks := []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: finalText}}}}
	if userStop {
		return blocks, nil, ctx.Err()
	}
	// 优先用 session stats 的本轮增量(settle 时最权威:流式帧没赶上的都在会话
	// 文件里);stats 不可用时按 0.87.1 流式累计兜底,再退 agent_end 携带的 usage。
	outUsage := usage.Usage
	if outUsage == nil || (outUsage.GetInputTokens() == 0 && outUsage.GetOutputTokens() == 0) {
		if su := usageTrack.snapshot(); su != nil && (su.GetInputTokens() != 0 || su.GetOutputTokens() != 0) {
			outUsage = su
		} else if lastUsage != nil {
			outUsage = lastUsage
		}
	}
	return blocks, outUsage, nil
}

// errSettled 用 context.Cause 传递"正常终局"信号(agent_settled → runCancel)。
var errSettled = errors.New("agent_settled")

// 看门狗两闸的 cause 信号(与 errSettled 同机制:runCancel(err) → context.Cause)。
var (
	errPiSilence = errors.New("pi silence watchdog")
	errPiTurnCap = errors.New("pi turn cap")
)

// piWatchdogVerdict 看门狗判定(纯函数,回归钉用):0=继续等,1=静默超限,2=单轮超限。
// uiInFlight>0(手机 dialog 审批/未知请求确认在飞)豁免静默闸 —— 审批等待期间
// pi 合法地不出事件;单轮上限不豁免(预算远超单次审批 120s,60min 仍超即真异常)。
func piWatchdogVerdict(sinceEvent, sinceStart time.Duration, uiInFlight int64) int {
	if sinceStart >= piTurnCap {
		return 2
	}
	if uiInFlight == 0 && sinceEvent >= piSilenceWatchdog {
		return 1
	}
	return 0
}

// statsQuiet 静默取 get_session_stats(失败返回 nil,不阻塞轮次)。
func (p *PiProvider) statsQuiet(c *piapp.Client) *piSessionStats {
	raw, err := c.Call("get_session_stats", nil, piCallBudget)
	if err != nil {
		return nil
	}
	return parseSessionStats(raw)
}

// usageFromAgentEnd 从 agent_end 的 messages 里取最后一条 assistant 的 usage。
//
// 【cost 为什么按对象解析】0.85.1/0.87.1 实测 message.usage.cost 都是对象
// {"input":…,"output":…,"total":…}(探针报文),曾按 float64 解析导致整条
// json.Unmarshal 失败、本兜底从未生效 —— 现取 cost.total。
func usageFromAgentEnd(raw json.RawMessage) *pb.Usage {
	var e struct {
		Messages []struct {
			Role  string `json:"role"`
			Usage struct {
				Input      int64 `json:"input"`
				Output     int64 `json:"output"`
				CacheRead  int64 `json:"cacheRead"`
				CacheWrite int64 `json:"cacheWrite"`
				Cost       struct {
					Total float64 `json:"total"`
				} `json:"cost"`
			} `json:"usage"`
		} `json:"messages"`
	}
	if json.Unmarshal(raw, &e) != nil {
		return nil
	}
	for i := len(e.Messages) - 1; i >= 0; i-- {
		m := e.Messages[i]
		if m.Role == "assistant" {
			return &pb.Usage{
				InputTokens:      m.Usage.Input,
				OutputTokens:     m.Usage.Output,
				CacheReadTokens:  m.Usage.CacheRead,
				CacheWriteTokens: m.Usage.CacheWrite,
				CostUsd:          m.Usage.Cost.Total,
			}
		}
	}
	return nil
}

// ---- 流式用量(0.87.1+,探针实证)----

// usageFromMessageUpdate 从 message_update 帧提取流式用量;没有可用的非零
// usage 返回 nil。
//
// 【0.87.1 实测(docs/verify/evidence/pi-087-usage-probe.log)】RPC 层
// toJsonEvent 把帧上的 usage 定为"当前 assistant 消息自己的 usage"(bundle:
// `usage:event.message.usage`):流式增量帧(text_start/text_delta/thinking_delta)
// 期间恒为 0,**本条消息的最后一个增量帧(实测 text_end)才带上真实值**。
// 与 0.85.1"全程恒 0"不同,也和文档宣称的"累计"不符 —— 它是 **per-message**
// 口径:多步轮次有多条 assistant 消息,各带各的,轮次总量必须求和,不能拿
// 最后一帧冒充总量(求和口径与 get_session_stats 的轮次增量核对一致)。
func usageFromMessageUpdate(raw json.RawMessage) *pb.Usage {
	var e struct {
		Usage *struct {
			Input      int64 `json:"input"`
			Output     int64 `json:"output"`
			CacheRead  int64 `json:"cacheRead"`
			CacheWrite int64 `json:"cacheWrite"`
			Reasoning  int64 `json:"reasoning"`
			Total      int64 `json:"totalTokens"`
			Cost       struct {
				Total float64 `json:"total"`
			} `json:"cost"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &e) != nil || e.Usage == nil {
		return nil
	}
	u := e.Usage
	// 过程帧(0/全零)不入账:这是"非零才算真用量"的唯一判据,也是新旧版本
	// 自动分流的关键 —— 旧版(≤0.85.x)全帧为零,observed 恒假,轮询兜底照旧。
	if u.Total == 0 && u.Input == 0 && u.Output == 0 && u.CacheRead == 0 && u.CacheWrite == 0 && u.Cost.Total == 0 {
		return nil
	}
	return &pb.Usage{
		InputTokens:      u.Input,
		OutputTokens:     u.Output,
		CacheReadTokens:  u.CacheRead,
		CacheWriteTokens: u.CacheWrite,
		ReasoningTokens:  u.Reasoning,
		CostUsd:          u.Cost.Total,
	}
}

// piUsageTracker 累计本轮从 message_update 帧见到的流式用量。
//
// 【为什么按消息求和】流式 usage 是 per-message 口径(见 usageFromMessageUpdate),
// 每条 assistant 消息只在最后一个增量帧报一次非零值,天然不重复;求和恰等于
// 本轮增量(与 stats 差值同口径)。并发:observe 在事件泵消费 goroutine 调用,
// observed/snapshot 在轮询与收尾路径调用 —— 全走 mu;snapshot 逐字段拷贝
// (pb 消息含互斥量,按值复制会被 vet 拒绝,也不能共享指针被并发累加)。
type piUsageTracker struct {
	mu   sync.Mutex
	acc  *pb.Usage
	seen bool
}

// observe 解析一帧 message_update;带非零 usage 时入账并返回 true,调用方据此
// 立即推一帧 UsageUpdate(事件驱动,取代旧版 5 秒轮询的"会话中可见")。
func (t *piUsageTracker) observe(raw json.RawMessage) bool {
	u := usageFromMessageUpdate(raw)
	if u == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.acc == nil {
		t.acc = u
	} else {
		t.acc.InputTokens += u.InputTokens
		t.acc.OutputTokens += u.OutputTokens
		t.acc.CacheReadTokens += u.CacheReadTokens
		t.acc.CacheWriteTokens += u.CacheWriteTokens
		t.acc.ReasoningTokens += u.ReasoningTokens
		t.acc.CostUsd += u.CostUsd
	}
	t.seen = true
	return true
}

// observed 报告本轮是否见过至少一帧非零流式 usage。真(0.87.1+)→ 轮询兜底
// 歇脚;假(旧版 pi)→ 轮询照旧,行为与 0.85.1 时代完全一致。
func (t *piUsageTracker) observed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.seen
}

// snapshot 返回累计快照(逐字段拷贝);没见过非零帧返回 nil。
func (t *piUsageTracker) snapshot() *pb.Usage {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.acc == nil {
		return nil
	}
	return &pb.Usage{
		InputTokens:      t.acc.InputTokens,
		OutputTokens:     t.acc.OutputTokens,
		CacheReadTokens:  t.acc.CacheReadTokens,
		CacheWriteTokens: t.acc.CacheWriteTokens,
		ReasoningTokens:  t.acc.ReasoningTokens,
		CostUsd:          t.acc.CostUsd,
	}
}

// usageUpdateFromStream 用流式累计构造中程 UsageUpdate 帧,口径与收尾的
// UsageUpdateFromStats 对齐:usage=本轮增量(已累计部分),total_usage=会话累计
// (基线+本轮),窗口沿用基线快照(同一会话窗口不变;基线取不到时如实留 0,
// App 端忽略空窗口)。返回指针:pb 消息含互斥量,禁止按值返回(vet)。
func usageUpdateFromStream(acc *pb.Usage, baseline *piSessionStats) *pb.UsageUpdate {
	if acc == nil {
		acc = &pb.Usage{}
	}
	total := &pb.Usage{
		InputTokens:      acc.InputTokens,
		OutputTokens:     acc.OutputTokens,
		CacheReadTokens:  acc.CacheReadTokens,
		CacheWriteTokens: acc.CacheWriteTokens,
		ReasoningTokens:  acc.ReasoningTokens,
		CostUsd:          acc.CostUsd,
	}
	if baseline != nil {
		total.InputTokens += baseline.Tokens.Input
		total.OutputTokens += baseline.Tokens.Output
		total.CacheReadTokens += baseline.Tokens.CacheRead
		total.CacheWriteTokens += baseline.Tokens.CacheWrite
		total.CostUsd += baseline.Cost.Total
	}
	upd := &pb.UsageUpdate{Usage: acc, TotalUsage: total}
	if baseline != nil && baseline.ContextUsage != nil {
		upd.ModelContextWindow = baseline.ContextUsage.ContextWindow
	}
	return upd
}

func toolCallIDOf(raw json.RawMessage) string {
	var e struct {
		ToolCallID string `json:"toolCallId"`
	}
	if json.Unmarshal(raw, &e) == nil {
		return e.ToolCallID
	}
	return ""
}

// piElapsedOf 取该 callId 自 tool_execution_start 起的实测时长(daemon 自己的钟,
// 真测量非编造);没有开始时刻(断点回放/乱序帧)返回 0 = 无脚注。
// mu 保护 starts map:与事件泵消费 goroutine 并发。
func piElapsedOf(mu *sync.Mutex, starts map[string]time.Time, raw json.RawMessage) time.Duration {
	id := toolCallIDOf(raw)
	if id == "" {
		return 0
	}
	mu.Lock()
	defer mu.Unlock()
	if st, ok := starts[id]; ok {
		return time.Since(st)
	}
	return 0
}

// blocksToPrompt 把输入块转成 Pi 的 prompt 参数(文本拼接 + base64 图片)。
func blocksToPrompt(blocks []pb.ContentBlock) (string, []any) {
	var sb strings.Builder
	var images []any
	for i := range blocks {
		switch b := blocks[i].Block.(type) {
		case *pb.ContentBlock_Text:
			sb.WriteString(b.Text.Text)
		case *pb.ContentBlock_Image:
			// 图片走 base64 进单行 JSON(实测 1MB 图单行 1.34MB,8MB 读缓冲扛住 —— 探针第 9 项)
			images = append(images, map[string]any{
				"type":     "image",
				"data":     base64Of(b.Image.Data),
				"mimeType": b.Image.MediaType,
			})
		}
	}
	return sb.String(), images
}

// partialText 失败路径返回已有文本(TurnFailed.partialText 用)。
func partialText(mu *sync.Mutex, buf *strings.Builder) []pb.ContentBlock {
	mu.Lock()
	defer mu.Unlock()
	txt := buf.String()
	if txt == "" {
		return nil
	}
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: txt}}}}
}

// ---- Steerer:运行中插话(对接 server 的 TurnSupplement/TurnInput 体系) ----

// Steer 实现 agent.Steerer:向正在执行的 Pi 轮次插入输入(steer 语义)。
//
// 【对接】server.handleMidTurnInput 对实现 Steerer 的 provider 先试插话,
// 失败自动转排队(Queued 帧 + 终局后续跑)—— 排队路径完全复用,输入绝不丢。
// Pi 侧用专用 steer 命令(等价于 prompt + streamingBehavior:"steer",
// 在当前助手轮的工具调用结束后、下一次 LLM 调用前注入)。
func (p *PiProvider) Steer(ctx context.Context, threadID string, blocks []pb.ContentBlock) (bool, string, error) {
	p.steerMu.Lock()
	streaming := p.active[threadID]
	p.steerMu.Unlock()
	if !streaming {
		return false, "", errors.New("当前没有正在运行的轮次")
	}
	p.mu.Lock()
	c := p.clients[threadID]
	p.mu.Unlock()
	if c == nil || !c.Alive() {
		return false, "", errors.New("Pi 子进程不可用")
	}
	message, images := blocksToPrompt(blocks)
	params := map[string]any{"message": message}
	if len(images) > 0 {
		params["images"] = images
	}
	sctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := c.CallCtx(sctx, "steer", params, 15*time.Second); err != nil {
		// 轮次刚好结束/Pi 拒绝:交回 server 排队兜底,输入不丢
		return false, "", err
	}
	return true, "", nil
}

// ---- QueueClearer:清空上游原生排队输入(0.87.1 clear_queue)----

// QueueClearer 是「撤销上游原生队列里排队中的输入」的可选扩展能力(与 Steerer
// 同款探测模式:server 层类型断言,想支持的 provider 自行实现,不想就不用管)。
//
// 【与 daemon 自己的 turnInputQueue 是两码事】pi 的 steer/follow_up 队列活在
// pi 子进程内存里(0.87.1 探针 get_state 实证 pendingMessageCount 计数),用户
// 在手机端撤回排队消息时经它原生命中 pi 的 clear_queue —— 等价 pi TUI 里按
// Esc;daemon 侧排队的撤销不归它管。
type QueueClearer interface {
	// ClearQueue 清空 threadID 在上游排队中的全部输入,返回 (插队条数, 排队条数)。
	ClearQueue(ctx context.Context, threadID string) (steered, followUp int, err error)
}

// 编译期钉死能力声明(与 PrefsAware 的 var _ 同款)。
var _ QueueClearer = (*PiProvider)(nil)

// ClearQueue 实现 QueueClearer。
//
// 【没有活子进程 = 按空队列成功】原生队列在 pi 进程内存里,进程没了队列必然
// 已空(重挂载是新会话),按 (0,0,nil) 返回,绝不为"撤回一条可能不存在的消息"
// 冷启一个 node 子进程(与 SlashCommandsFor 的"不冷启"同哲学)。
func (p *PiProvider) ClearQueue(ctx context.Context, threadID string) (int, int, error) {
	c := p.aliveClient(threadID)
	if c == nil {
		return 0, 0, nil
	}
	return clearQueueOn(ctx, c)
}

// queueClearerRPC 撤销排队对 piapp.Client 的窄接口(单测打桩,不依赖子进程;
// piapp.Client 的 ClearQueue 签名天然满足,生产零适配)。
type queueClearerRPC interface {
	ClearQueue(ctx context.Context) (*piapp.ClearedQueue, error)
}

// clearQueueOn 下发 clear_queue 并折算被撤销的条数(抽成函数便于注入桩,与
// sendSessionName 同模式);失败包结构化 piError 交调用方上报。
func clearQueueOn(ctx context.Context, c queueClearerRPC) (int, int, error) {
	q, err := c.ClearQueue(ctx)
	if err != nil {
		return 0, 0, &piError{code: "PI_QUEUE_CLEAR_FAILED", msg: "撤销排队输入失败: " + err.Error()}
	}
	return len(q.Steering), len(q.FollowUp), nil
}

// piStopClient 用户中止序列对 piapp.Client 的窄接口(单测用记录调用顺序的桩
// 钉下发顺序,不依赖子进程;piapp.Client 的方法集天然满足,生产零适配)。
type piStopClient interface {
	queueClearerRPC
	CallCtx(ctx context.Context, typ string, params map[string]any, timeout time.Duration) (json.RawMessage, error)
}

// stopTurnOn 按文档配方终止一轮:先 clear_queue 摘掉原生队列(插队+排队全撤),
// 再 abort 停当前操作(0.99.1 bundle:abort 内部连重试/压缩/分支摘要一并取消)。
// 【顺序即语义】"abort continues queued messages when they remain in the
// session"(rpc-commands.md)—— 队列不清,abort 之后 pi 继续把排队跑完;
// 先清后停,abort 之后无事可续。抽成函数让单测钉死顺序(先 abort 后清 = 排队
// 消息在停止后仍被执行的回归)。返回撤销的 (插队, 排队) 条数与 clear_queue 的
// 错误;abort 的错误不向上传播(abort 无应答多半是子进程临终,收尾照走,与
// 原行为一致)。
func stopTurnOn(ctx context.Context, c piStopClient) (steered, followUp int, err error) {
	steered, followUp, err = clearQueueOn(ctx, c)
	_, _ = c.CallCtx(ctx, "abort", nil, piSettleWait)
	return steered, followUp, err
}

// ---- S 级增量能力:压缩 / 离线导出 / 动态思考档位 / 斜杠命令清单 ----
//
// 这组方法供 server 层接线(CompactThread RPC 路由 / ListSlashCommands 面板)与
// 后续 UI 消费;全部带自有超时,失败返回 piError(结构化进 gRPC 错误),不碰
// Run 的轮次路径。

// S 项专属超时(compact 要等一次真实 LLM 摘要,export 要渲染整棵会话树)。
const (
	piCompactBudget = 180 * time.Second
	piExportBudget  = 60 * time.Second
	// piSlashBudget 斜杠面板拉 get_commands 的上界:本地注册面毫秒级应答,
	// 上界只兜子进程临终一刻(用户每敲一个 "/" 都可能拉一次,不能给长预算)。
	piSlashBudget = 5 * time.Second
)

// piDefaultThinkingLevels Pi 档位全集(bundle THINKING_LEVEL_OPTIONS)。
// 【只做兜底】权威值必须问会话:get_available_thinking_levels 按当前模型的
// reasoning 能力 + thinkingLevelMap 过滤(实测 glm-5.3 只有 ["low","high","max"])。
var piDefaultThinkingLevels = []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"}

// aliveClient 取该线程还活着的子进程;没有(或已死)返回 nil,由调用方决定
// 是否冷启重挂载。与 Steer 同款只读快照,不与 Run 抢 clientFor 的重建路径。
func (p *PiProvider) aliveClient(threadID string) *piapp.Client {
	p.mu.Lock()
	c := p.clients[threadID]
	p.mu.Unlock()
	if c != nil && !c.Alive() {
		return nil
	}
	return c
}

// ensureMounted 冷启(或重挂)线程的子进程并完成会话握手。仅在外部能力方法
// (压缩/导出)发现没有活客户端时调用;与 Run 的 mount 共用,重挂载后
// ExternalSession 回填/校验语义一致。
func (p *PiProvider) ensureMounted(t *store.Thread) (*piapp.Client, error) {
	c, err := p.clientFor(t, t.Workspace)
	if err != nil {
		return nil, &piError{code: "PI_UNAVAILABLE", msg: err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), piSwitchBudget)
	defer cancel()
	if err := p.mount(ctx, t, c); err != nil {
		return nil, &piError{code: "PI_MOUNT_FAILED", msg: err.Error()}
	}
	return c, nil
}

// CompactThread 实现 pi 侧手动压缩(对齐 codex 的 CompactThread 语义,server
// CompactThread RPC 按 AgentID 路由到它)。同步等真实压缩完成后返回。
//
// 【两道闸,顺序不能换】
//  1. 轮次在跑 → 拒绝:pi 的 compact 第一步就是 abort 当前轮,放进来等于
//     悄悄杀掉用户的任务(0.85.1 实测语义,bundle: `await this.abort()`);
//  2. 会话没落地(SessionFile 空)→ 拒绝:没东西可压,与 codex 路径同文案口径。
func (p *PiProvider) CompactThread(t *store.Thread) error {
	p.mu.Lock()
	busy := p.running[t.ID]
	p.mu.Unlock()
	if busy {
		return &piError{code: "PI_BUSY", msg: "这个会话正在执行任务,等它跑完再压缩"}
	}
	if t.ExternalSession.SessionFile == "" {
		return &piError{code: "PI_NOT_MOUNTED", msg: "这个会话还没有在电脑上落地(先发一条消息),暂时无法压缩"}
	}
	c := p.aliveClient(t.ID)
	if c == nil {
		nc, err := p.ensureMounted(t)
		if err != nil {
			return err
		}
		c = nc
	}
	ctx, cancel := context.WithTimeout(context.Background(), piCompactBudget)
	defer cancel()
	res, err := c.Compact(ctx, "")
	if err != nil {
		return &piError{code: "PI_COMPACT_FAILED", msg: err.Error()}
	}
	log.Printf("[pi] 手动压缩完成 thread=%s tokens %d → 约 %d", t.ID, res.TokensBefore, res.EstimatedTokensAfter)
	return nil
}

// ForkThread 把当前 pi 会话在"当前叶"分叉成一个新会话(pi 0.99.1 clone RPC),
// 返回新会话的挂载引用;原会话与其会话文件不受影响(pi_fork_plan.md §0 的语义
// 裁决:pi 的 clone 才是 codex thread/fork 的对等物 —— fork {entryId} 是"切换
// 当前进程"的半对等语义,不采用)。落库由 server 层做(forkPiThread),本方法只
// 回引用 —— 与 mount"回填由 server 落库"的分工一致(pi.go:464 注释)。
//
// 【子进程归属裁决(v1:关闭回收,活体证据 pi-fork-live-probe.md F7/F8)】
// clone 后这个子进程已切到新分支会话,而新线程 id 由 server 生成、provider 拿
// 不到;把"挂在新会话上的子进程"留给任何一条线程都会把对方的事件记错账
// (GlobalHandler 按 threadID 闭包记账)。这里按 DropThread 的内存回收三连关闭
// 回收:活体实测分支文件立即落盘,源线程下次 Run 冷启动后按 SessionFile switch
// 回源文件、新线程首次 Run 冷启动后 switch 上分支文件都稳 —— 各多一次 spawn,
// 换零事件错账。
//
// 【三道闸,前两道不发 RPC】轮次运行中/未落地(SessionFile 空)→ 结构化拒绝
// (server 侧 forkPiThread 同款前置,双保险);会话未落盘(文件不在磁盘上)→
// 对齐 bundle 闸门(agent-session-runtime.js:217 的 existsSync,活体 F2:pi 原生
// 报英文 "This session has not been saved yet…")提前转结构化中文错误。
func (p *PiProvider) ForkThread(t *store.Thread) (store.ExternalSessionRef, error) {
	p.mu.Lock()
	busy := p.running[t.ID]
	p.mu.Unlock()
	if busy {
		return store.ExternalSessionRef{}, &piError{code: "PI_BUSY", msg: "这个会话正在执行任务,等它跑完再分叉"}
	}
	if t.ExternalSession.SessionFile == "" {
		return store.ExternalSessionRef{}, &piError{code: "PI_NOT_MOUNTED", msg: "这个会话还没有在电脑上落地(先发一条消息),暂时无法分叉"}
	}
	c, err := p.forkMount(t)
	if err != nil {
		return store.ExternalSessionRef{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), piCallBudget)
	defer cancel()
	// 前置校验:文件必须在磁盘上(bundle existsSync 闸门的结构化前移)+ 树上
	// 必须有叶(get_state 只保证"落过盘",leafId=null 的空树 clone 会被 pi 以
	// "no current entry selected" 拒 —— 活体 F6:正常分支 leafId 非空)。
	st, err := c.GetState(ctx)
	if err != nil {
		return store.ExternalSessionRef{}, &piError{code: "PI_FORK_FAILED", msg: "获取 Pi 会话状态失败: " + err.Error()}
	}
	current, _ := st["sessionFile"].(string)
	if current == "" {
		current = t.ExternalSession.SessionFile
	}
	if _, err := os.Stat(current); err != nil {
		return store.ExternalSessionRef{}, &piError{code: "PI_NOT_SAVED", msg: "先发一条消息,会话落盘后才能分叉"}
	}
	if raw, err := c.Call("get_tree", nil, piCallBudget); err == nil {
		var tree struct {
			LeafID any `json:"leafId"`
		}
		if json.Unmarshal(raw, &tree) == nil && tree.LeafID == nil {
			return store.ExternalSessionRef{}, &piError{code: "PI_NOT_SAVED", msg: "先发一条消息,会话有内容后才能分叉"}
		}
	}
	// clone:pi 侧 createBranchedSession(复制活跃分支全量)→ 进程切到新会话。
	// 应答 data={cancelled}(不回新会话 id),新身份靠下一步 get_state(活体 F4/F5)。
	raw, err := c.Call("clone", nil, piCallBudget)
	if err != nil {
		return store.ExternalSessionRef{}, &piError{code: "PI_FORK_FAILED", msg: "Pi 分叉失败: " + err.Error()}
	}
	var data struct {
		Cancelled bool `json:"cancelled"`
	}
	if json.Unmarshal(raw, &data) == nil && data.Cancelled {
		return store.ExternalSessionRef{}, &piError{code: "PI_FORK_CANCELLED", msg: "Pi 分叉被扩展取消"}
	}
	// 取新引用:clone 后进程已切到新分支会话,GetState 刷新 SessionFile/SessionID
	// 缓存(updateSession)后组装 ref(与 mount 回填 pi.go:470-474 同式)。
	if _, err := c.GetState(ctx); err != nil {
		return store.ExternalSessionRef{}, &piError{code: "PI_FORK_FAILED", msg: "分叉后获取新会话状态失败: " + err.Error()}
	}
	ref := store.ExternalSessionRef{
		SessionID:   c.SessionID(),
		SessionFile: c.SessionFile(),
		Cwd:         t.Workspace,
	}
	if ref.SessionFile == "" || ref.SessionID == "" {
		return store.ExternalSessionRef{}, &piError{code: "PI_FORK_FAILED", msg: "分叉后未取得新会话身份(sessionFile/sessionId 为空)"}
	}
	// 子进程回收(DropThread 三连 + Close):子进程已属于"新会话"但新线程 id
	// 归 server 管,关闭回收换零事件错账(见方法头裁决)。
	p.mu.Lock()
	delete(p.clients, t.ID)
	delete(p.cursors, t.ID)
	p.mu.Unlock()
	p.forgetNativeQueue(t.ID)
	c.Close()
	log.Printf("[pi] 分叉完成 thread=%s → 新会话 %s(子进程已回收,两条线程下次 Run 各自冷启动重挂载)", t.ID, filepath.Base(ref.SessionFile))
	return ref, nil
}

// forkMount 分叉前的挂载入口:生产走 ensureMounted(复用活客户端或冷启重挂载,
// 握手在 mount 内);测试经 sessionFor 钩子注入假客户端(与原生会话适配器的
// mountFor 同一注入源,不拉真子进程)。
func (p *PiProvider) forkMount(t *store.Thread) (piMountClient, error) {
	if hook := p.sessionFor; hook != nil {
		return hook(t)
	}
	return p.ensureMounted(t)
}

// ExportHTML 把会话导出成离线 HTML 存档,落到会话工作区下的
// .pocketagent-exports/<threadId>.html,返回绝对路径 —— 手机端经现有
// ReadFile(FileChunk 流)下载链路取文件,手机浏览器直接打开,离线可看。
//
// 【为什么落工作区】daemon 不额外引入"导出目录"配置;放会话自己的工作区
// (隐藏目录),跟随现有文件服务的可见范围与鉴权,不新开攻击面。
// 【轮次中可导出】export 是只读渲染,不碰运行态,不设 running 闸。
func (p *PiProvider) ExportHTML(t *store.Thread) (string, error) {
	if t.ExternalSession.SessionFile == "" {
		return "", &piError{code: "PI_NOT_MOUNTED", msg: "这个会话还没有在电脑上落地(先发一条消息),暂时无法导出"}
	}
	c := p.aliveClient(t.ID)
	if c == nil {
		nc, err := p.ensureMounted(t)
		if err != nil {
			return "", err
		}
		c = nc
	}
	out, err := piExportPath(t)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), piExportBudget)
	defer cancel()
	got, err := c.ExportHTML(ctx, out)
	if err != nil {
		return "", &piError{code: "PI_EXPORT_FAILED", msg: err.Error()}
	}
	return got, nil
}

// piExportPath 导出文件的落盘路径:<工作区>/.pocketagent-exports/<threadId>.html。
// 抽成纯函数单测钉死布局(手机端按同一约定拼下载路径)。
func piExportPath(t *store.Thread) (string, error) {
	ws := strings.TrimSpace(t.Workspace)
	if ws == "" {
		return "", &piError{code: "PI_EXPORT_FAILED", msg: "会话没有工作区,无法确定导出位置"}
	}
	dir := filepath.Join(ws, ".pocketagent-exports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", &piError{code: "PI_EXPORT_FAILED", msg: "创建导出目录失败: " + err.Error()}
	}
	return filepath.Join(dir, t.ID+".html"), nil
}

// SlashCommandsFor 返回该线程当前可用的 pi 斜杠命令(ListSlashCommands 面板的
// pi 数据源,与 zcode 的 zcodeSlashSource.SlashCommandsFor 同签名同语义)。
//
// 【数据来源(探针实证)】pi RPC get_commands(0.85.1,cmd/piprobe -mode cmds,
// 证据 docs/verify/evidence/pi-cmds-probe.md):一次应答带全 extension 命令 /
// prompt 模板 / 技能("skill:<名>" 前缀形态)。三源都是"prompt 以 /<名> 开头
// 即可触发"的真命令 —— 名字原样透传,手机点选回填 "/<名> " 后走 prompt RPC,
// pi 侧原生展开(命令执行 / 技能内容注入),不需要任何自然语言转译。
//
// 【只用活客户端,不冷启】没有活子进程(会话没在电脑上落地 / daemon 刚重启)
// 直接返回 nil:为一个"/"按键冷启一个 node 子进程不划算,与 zcode 路径 3 的
// "不为看一眼列表凭空建会话"同哲学;会话跑起来后(或重挂载后)自然有数据。
// 失败记日志回 nil,面板安静收起,绝不把错误抛给输入框。
func (p *PiProvider) SlashCommandsFor(threadID string) []*pb.SlashCommand {
	c := p.aliveClient(threadID)
	if c == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), piSlashBudget)
	defer cancel()
	got, err := c.GetCommands(ctx)
	if err != nil {
		log.Printf("[pi] 拉取斜杠命令失败(面板收起) thread=%s: %v", threadID, err)
		return nil
	}
	out := make([]*pb.SlashCommand, 0, len(got))
	for _, sc := range got {
		out = append(out, &pb.SlashCommand{
			Name:        sc.Name,
			Description: sc.Description,
			// inputHint 有意留空:pi 的 RpcSlashCommand 没有参数提示字段;手机端
			// 对空 inputHint 的非 codex 命令走 "/命令 " 回填,与 pi 的原生展开语义对齐。
		})
	}
	return out
}

// ---- 标题同步(手机端重命名 → Pi 会话,与 TitleHandler 互为反向)----

// sessionNameSetter 重命名同步对 piapp.Client 的窄接口(单测打桩,不依赖子进程;
// piapp.Client 的 CallCtx 签名天然满足,生产零适配)。
type sessionNameSetter interface {
	CallCtx(ctx context.Context, typ string, params map[string]any, timeout time.Duration) (json.RawMessage, error)
}

// Rename 把手机端的重命名下发到 Pi(set_session_name),电脑端 pi -r 立即可见。
// 与 TitleHandler 互为反向:那边是"Pi 改名 → 落库",这边是"落库 → Pi 改名"。
//
// 【两条短路,都不碰子进程】
//   - 空标题:与 mount 新会话分支同口径(空名不下发);
//   - 会话还没在电脑上落地(SessionFile 空):下次 Run 的 mount 新会话分支会用
//     落库后的 t.Title 下发 set_session_name,天然覆盖,无需此刻起进程。
//
// 【其余路径】有活客户端直接下发;没有则冷启重挂载后下发(与 CompactThread
// 同款"按需起子进程" —— 重命名是低频动作,秒级开销可接受)。不在 mount 的
// 重挂载分支补下发:电脑端在 daemon 无客户端期间改的名会被库里的旧标题顶掉,
// "最后一次改名为准"的正确语义由本方法直接下发保证。
//
// 【失败语义】返回错误交调用方记日志,不让本地重命名失败 —— 与 codex 的
// SetCodexThreadName 同款纪律(库内已改名是既成事实)。名字下发后 Pi 回发
// session_info_changed → TitleHandler → 落库,库内标题已相同,"标题相同不写库"
// 守卫保证不会形成回调环(见 server.wirePiTitleHandler)。
func (p *PiProvider) Rename(t *store.Thread, title string) error {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return nil
	}
	if t.ExternalSession.SessionFile == "" {
		return nil
	}
	c := p.aliveClient(t.ID)
	if c == nil {
		nc, err := p.ensureMounted(t)
		if err != nil {
			return err
		}
		c = nc
	}
	return p.sendSessionName(c, t.ID, trimmed)
}

// sendSessionName 下发 set_session_name(抽成方法便于单测注入 [sessionNameSetter] 桩)。
func (p *PiProvider) sendSessionName(c sessionNameSetter, threadID, title string) error {
	ctx, cancel := context.WithTimeout(context.Background(), piCallBudget)
	defer cancel()
	if _, err := c.CallCtx(ctx, "set_session_name", map[string]any{"name": title}, piCallBudget); err != nil {
		log.Printf("[pi] 同步新名字失败(本地已改名) thread=%s: %v", threadID, err)
		return &piError{code: "PI_RENAME_FAILED", msg: "同步新名字到 Pi 失败(本地已改名): " + err.Error()}
	}
	log.Printf("[pi] 重命名已同步 thread=%s title=%.60q", threadID, title)
	return nil
}

// ---- 错误类型(实现 ErrorInfoCarrier,结构化进 TurnFailed)----

type piError struct {
	code string
	msg  string
}

func (e *piError) Error() string { return e.msg }

func (e *piError) ErrorInfo() *pb.Error {
	return &pb.Error{Code: e.code, Message: e.msg}
}

// ---- 断点续传(调研 §8.2:事件缓冲 + 游标续传的 daemon 侧落地)----
//
// 三态:
//  1. 推进:每轮收尾 get_entries 一次,游标推到"最近一次从 Pi 拿到的条目 id"
//     (RPC 流式事件不带条目 id,逐帧推进不成立;每轮一次 RPC 最省,页内最后
//     一条即见过的最大位置)。
//  2. 增量:重挂载后游标有效 → get_entries(since=游标) 权威增量;条目经
//     piapp.Entry.WireEvents 合成**线上事件形状**,复用现有映射函数(MapTextDelta/
//     MapToolcallEnd/…)做摘要 —— 回放与实时共用同一映射真相;一条 CodexEvent
//     (STATUS)上屏,不注入正文流(textBuf 会拼进本轮最终答案,不能混)。
//  3. 兜底:游标失效(ErrCursorInvalid,Entry not found 等)→ 走现有
//     switch_session 路径(mount 已完成,历史以 store 会话记录为准),并以全量
//     get_entries 的 leafId 重锚游标;全量也失败则清空游标,下轮收尾重建。
//     (rpc.md:get_messages 的 AgentMessage 不带 entryId,重锚必须用全量
//     get_entries,而不是 get_messages —— 后者只在 get_fork_messages 里带 id。)

// entriesFetcher 断点续传对 piapp.Client 的窄接口(单测打桩,不依赖子进程)。
type entriesFetcher interface {
	GetEntries(ctx context.Context, sessionFile, sinceEntryID string) (*piapp.EntriesPage, error)
}

// cursorOf 读线程游标(与 clients/running 同锁)。
func (p *PiProvider) cursorOf(threadID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cursors[threadID]
}

// setCursor 写线程游标;id 为空表示清空。
func (p *PiProvider) setCursor(threadID, id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if id == "" {
		delete(p.cursors, threadID)
		return
	}
	p.cursors[threadID] = id
}

// advanceEntryCursor 推进游标(Run 收尾调用):用脱离轮次生命周期的独立超时
// (收尾时 runCtx 已取消,但 Pi 会话文件是磁盘真相,照拉不误)。失败只记日志:
// 游标落后只会让下次回放多补几条,不会丢 —— 续传语义天生幂等。
func (p *PiProvider) advanceEntryCursor(f entriesFetcher, threadID string) {
	ctx, cancel := context.WithTimeout(context.Background(), piCallBudget)
	defer cancel()
	page, err := f.GetEntries(ctx, p.sessionFileOf(f), p.cursorOf(threadID))
	if err != nil {
		log.Printf("[pi] 断点续传游标推进失败(下轮再试) thread=%s: %v", threadID, err)
		return
	}
	if id := page.LastID(); id != "" {
		p.setCursor(threadID, id)
	}
}

// sessionFileOf 取 fetcher 当前挂载的会话文件(用于 GetEntries 的挂载校验)。
// 真客户端读 SessionFile 缓存;测试桩返回空串(校验自动跳过)。
func (p *PiProvider) sessionFileOf(f entriesFetcher) string {
	if c, ok := f.(*piapp.Client); ok {
		return c.SessionFile()
	}
	return ""
}

// replayMissed 断线重连(子进程重启 → mount 重挂载成功)后的三态续传。
// 返回是否回放了条目(诊断用)。
func (p *PiProvider) replayMissed(ctx context.Context, f entriesFetcher, t *store.Thread, sink Sink) bool {
	sessionFile := t.ExternalSession.SessionFile
	cur := p.cursorOf(t.ID)
	if cur == "" {
		return false // 无游标基线(daemon 重启后内存归零/全新会话):跳过,收尾会重建
	}
	page, err := f.GetEntries(ctx, sessionFile, cur)
	if err != nil {
		// ---- 兜底:增量不可用 → 现有路径 + 全量重锚 ----
		log.Printf("[pi] 断点续传:增量拉取失败,转全量兜底 thread=%s: %v", t.ID, err)
		newCur := ""
		if full, ferr := f.GetEntries(ctx, sessionFile, ""); ferr != nil {
			log.Printf("[pi] 断点续传:全量重锚也失败,清空游标(下轮收尾重建) thread=%s: %v", t.ID, ferr)
		} else {
			newCur = full.LeafID // 重锚到活动分支叶(废弃分支历史不回放,以会话记录为准)
		}
		p.setCursor(t.ID, newCur)
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
			Kind:    pb.CodexEvent_STATUS,
			Code:    "pi_resume_fallback",
			Title:   "断线续传",
			Message: "断线期间的增量不可用,已回退全量:历史以会话记录为准,本轮继续执行。",
		}}})
		return false
	}
	if len(page.Entries) == 0 {
		return false // 断线期间没有新条目:游标原位即最新,无需回放
	}
	// ---- 增量回放 ----
	parts := make([]string, 0, len(page.Entries))
	for i := range page.Entries {
		parts = append(parts, replayEntryDigest(&page.Entries[i]))
	}
	summary := fmt.Sprintf("断线期间补到 %d 条事件:%s", len(page.Entries), strings.Join(parts, ";"))
	log.Printf("[pi] 断点续传:增量回放 thread=%s since=%s → %s", t.ID, cur, summary)
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
		Kind:    pb.CodexEvent_STATUS,
		Code:    "pi_resume_replay",
		Title:   "断线续传",
		Message: clipRunes(summary, 8000),
	}}})
	if id := page.LastID(); id != "" {
		p.setCursor(t.ID, id) // 推进到见过的最大位置(含废弃分支,字面"最后一条")
	}
	return true
}

// replayEntryDigest 单条回放摘要:条目先经 WireEvents 合成线上形状,再喂给
// 现有映射函数(MapTextDelta/MapThinkingDelta/MapToolcallEnd/MapToolExecEnd)——
// 回放与实时共用同一映射管线,格式漂移会被映射函数的既有单测拦住。
func replayEntryDigest(e *piapp.Entry) string {
	if e == nil {
		return "?"
	}
	var texts, thinks, calls, ends, failedEnds int
	toolName := ""
	for _, ev := range e.WireEvents() {
		switch ev.Type {
		case "message_update":
			if d, ok := MapTextDelta(ev.Raw); ok {
				texts += len([]rune(d.Delta))
				continue
			}
			if d, ok := MapThinkingDelta(ev.Raw); ok {
				thinks += len([]rune(d.Delta))
				continue
			}
			if snap, ok := MapThinkingSnapshot(ev.Raw); ok {
				// 历史回放/thinking_end 终局全文同样计入摘要字数(P1-3 接线配平)。
				thinks += len([]rune(snap.Snapshot))
				continue
			}
			if upd, ok := MapToolcallEnd(ev.Raw); ok {
				calls++
				toolName = upd.Call.Name
			}
		case "tool_execution_end":
			if upd, ok := MapToolExecEnd(ev.Raw, piToolClip, 0); ok {
				ends++
				toolName = upd.Call.Name
				if !upd.Result.Ok {
					failedEnds++
				}
			}
		}
	}
	if texts > 0 || thinks > 0 || calls > 0 {
		d := make([]string, 0, 3)
		if texts > 0 {
			d = append(d, fmt.Sprintf("正文 %d 字", texts))
		}
		if thinks > 0 {
			d = append(d, fmt.Sprintf("思考 %d 字", thinks))
		}
		if calls > 0 {
			d = append(d, "调用 "+nonEmpty(toolName, "工具"))
		}
		return "助手消息(" + strings.Join(d, ", ") + ")"
	}
	if ends > 0 {
		state := "完成"
		if failedEnds > 0 {
			state = "失败"
		}
		return nonEmpty(toolName, "工具") + " 结果(" + state + ")"
	}
	return "条目 " + e.ID // 非 message 条目(会话元数据等):只报 id
}

// nonEmpty 空串兜底。
func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// Only advertise levels explicitly supported by this model. Pi uses null to disable a level.
// Mirrors Pi getSupportedThinkingLevels: null disables; missing base levels are supported.
func piModelThinkingLevels(reasoning bool, mapping map[string]json.RawMessage) []string {
	if !reasoning {
		return []string{"off"}
	}
	var out []string
	for _, level := range piDefaultThinkingLevels {
		raw, exists := mapping[level]
		if exists && string(raw) == "null" {
			continue
		}
		if (level == "xhigh" || level == "max") && !exists {
			continue
		}
		out = append(out, level)
	}
	// Preserve future capability keys instead of imposing our legacy seven-level enum.
	seen := map[string]bool{}
	for _, v := range piDefaultThinkingLevels {
		seen[v] = true
	}
	var additional []string
	for key, value := range mapping {
		if !seen[key] && string(value) != "null" {
			additional = append(additional, key)
		}
	}
	sort.Strings(additional)
	return append(out, additional...)
}

func (p *PiProvider) InvalidateModels() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.modelsAt = time.Time{}
}
