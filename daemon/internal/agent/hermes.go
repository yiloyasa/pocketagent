package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// HermesProvider 桥接 Hermes Agent(ACP stdio;Nous Research,经 npm bridge)。
// 与 zcode/dsh 同构:懒启动共享适配器进程;会话跨轮续接(同进程内缓存 →
// ExternalSession 重挂载(session/load,历史回放)→ 新建),不再"一轮一会话"。
//
// 【投影映射(ACP session/update → TurnServerMessage)】
//
//	agent_message_chunk → TextDelta(流式正文)
//	agent_thought_chunk → ThinkingDelta(HideReasoning 时丢弃)
//	tool_call / tool_call_update → ToolCallUpdate(STARTED / UPDATE / FINISHED;
//	  diff 内容块并入 JsonArgs,write_file/patch 的编辑提案实证形状)
//	plan → 计划步骤("status|text",status 用四词口径)
//	usage_update(size,used) → TurnServerMessage_Usage(ModelContextWindow=size,
//	  Usage.InputTokens=used —— 手机 ContextUsageCard 的数据源,zcode/dsh 同通道)
//
// 【轮次取消(2026-09-30 接通)】prompt 阻塞到轮次终局,Run 用看门狗三闸收场:
// ctx 取消(手机停止帧/CancelTurn/重入打断)、150s 无任何事件(审批等待豁免)、
// 60 分钟单轮上限 —— 三路都先向适配器发 session/cancel 通知(ACP:无应答,
// server.py cancel → cancel_event.set + hard interrupt)再返回,daemon 侧放弃
// 不再等于"适配器还在跑"(旧实现只断 daemon 侧,下一轮 prompt 被挂起的实害)。
//
// 【审批链(2026-10-02 实证接通)】适配器在危险命令/文件编辑前发反向请求
// session/request_permission(permissions.py/edit_approval.py),provider 装当轮
// 钩子承接 → 手机审批卡(ApprovalRequest)→ ApprovalHook 裁决 → 以
// {"outcome":{"outcome":"selected","optionId":…}} 应答。超时/通道故障 → 拒绝侧
// (失败安全,绝不伪造"同意")。
//
// 【终局语义(ACP 实测)】适配器把 provider 失败也包成 stopReason=end_turn+错误
// 正文(turn_failure_copy 文案);正文为空 = 真失败,如实报错,不伪造空成功。
type HermesProvider struct {
	mu     sync.Mutex // prefs 锁
	lifeMu sync.Mutex // 客户端生命周期锁
	turnMu sync.Mutex // 同一时刻只跑一轮(共享适配器 + 当轮钩子,与 dsh 同款)

	client       hermesClient
	newClient    func() (hermesClient, error) // 拉起内核(测试注入假客户端)
	pyExe        string
	acpDir       string
	caps         *hermesapp.AgentCapabilities // initialize 应答快照(重挂载门控)
	agentVersion string                       // initialize 应答的 agentInfo.version(诊断文案用)
	prefs        DesktopPrefs
	turnActive   bool

	threadMu  sync.Mutex
	threadMap map[string]hermesThreadRef // threadID → ACP 会话引用(内存缓存)
}

// hermesThreadRef 是 hermes 会话的内存引用(同代缓存;失效回落 ExternalSession
// 重挂载,hermes 持久库(HERMES_HOME 内 DB)是跨重启的事实来源)。
type hermesThreadRef struct {
	sessionID string
}

// hermesClient 是 provider 对 ACP 客户端的依赖面(与 dshClient 同款手法:接口
// 钉死,测试注入假实现;*hermesapp.Client 编译期满足)。
type hermesClient interface {
	Alive() bool
	Initialize(ctx context.Context) (*hermesapp.InitializeResult, error)
	SessionNew(ctx context.Context, cwd string) (*hermesapp.SessionNewResult, error)
	LoadSession(ctx context.Context, cwd, sessionID string) (*hermesapp.SessionLoadResult, error)
	ListSessions(ctx context.Context, cursor string) (*hermesapp.SessionListResult, error)
	SetSessionModel(ctx context.Context, sessionID, modelID string) error
	Prompt(ctx context.Context, sessionID, text string) (string, error)
	CancelSession(ctx context.Context, sessionID string) error
	SetTurnHandlers(notif func(json.RawMessage), req func(id int64, method string, params json.RawMessage))
	Respond(id int64, result any) error
	RespondError(id int64, code int, message string) error
	Close()
}

// 编译期钉死:真实客户端满足依赖面(接口漂移在编译期暴露)。
var _ hermesClient = (*hermesapp.Client)(nil)

func NewHermesProvider() *HermesProvider {
	return &HermesProvider{newClient: defaultNewHermesClient, threadMap: map[string]hermesThreadRef{}}
}

func defaultNewHermesClient() (hermesClient, error) {
	py, dir, err := hermesapp.LocateHermes()
	if err != nil {
		return nil, err
	}
	return hermesapp.Start(py, dir)
}

func (p *HermesProvider) ID() string          { return "hermes" }
func (p *HermesProvider) DisplayName() string { return "Hermes" }

// Models 静态可见集:真实目录需 Start+session/new(启动红线:探测不起子进程)。
// daemon 在用户首轮时经会话内 SetSessionModel 对上 hermes 自己的模型表。
// AvailableEfforts = hermes 的 VALID_REASONING_EFFORTS(hermes_constants.py:
// 1300)—— 手机端强度胶囊据此渲染,写入前 daemon 亦按同表校验(IsValidReasoningEffort)。
func (p *HermesProvider) Models() []pb.ModelInfo {
	efforts := []string{"minimal", "low", "medium", "high", "xhigh", "max"}
	return []pb.ModelInfo{
		{Id: "hermes/deepseek-flash", Provider: "hermes", DisplayName: "Hermes · DeepSeek Flash",
			Available: true, ContextWindow: 1_000_000, SupportsThinking: true, SupportsImages: true,
			AvailableEfforts: efforts},
		{Id: "hermes/default", Provider: "hermes", DisplayName: "Hermes · 默认模型(config.yaml)",
			Available: true, AvailableEfforts: efforts},
	}
}

// ApplyPrefs PrefsAware:HideReasoning 过滤思考流;审批策略/沙箱不映射——
// ACP 的审批由适配器的 request_permission 反向请求驱动(手机审批卡裁决),
// 编辑策略由 hermes 自己的会话模式管,daemon 不越权改写。
func (p *HermesProvider) ApplyPrefs(prefs DesktopPrefs) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.prefs = prefs
}

// ---- 生命周期预算(R2:懒启动两道闸都对齐四家,消灭无界挂起/无界排队)----

// hermesInitBudget ACP initialize 握手的专属上界(zcodeInitBudget=15s 同口径,
// 取宽档;codex clientInitBudget=8s 为紧档)。为什么握手要专门给上界:适配器
// "进程起来了但 initialize 永不回"是最坏情况,它发生在 ensureClient 内且全程
// 持有 lifeMu —— 无上界 = 该轮无界挂起 + 其它调用方在锁上无界排队(codex
// clientInitBudget 注释同款教训)。必须用新建的预算 ctx:调用方(Run)的 ctx
// 常无截止时间,复用它等于没有上界。
const hermesInitBudget = 15 * time.Second

// errHermesBusy 是 hermes 自己的忙哨兵(zcode ErrZcodeBusy 同款载体 busySentinelError
// 与形状:如实文案 + Unwrap 到包级 ErrBusy,调用方按 errors.Is(err, ErrBusy) 统一
// 识别忙态;文案不窜用别家)。
var errHermesBusy = &busySentinelError{msg: "hermes 正忙(另一个操作正在初始化/探测),请稍后重试"}

// lockLife 有界排队获取生命周期锁(zcode/dsh 同款):先 TryLock(无争用零开销),
// 失败后在 wait 窗口内 25ms 间隔重试 —— 排队而非立即拒绝,把"低频用户路径
// 刚好撞上初始化"的瞬时争用吸收成先后等待;窗口耗尽(持有方真卡死)才返回
// false → 调用方回忙哨兵,保留 TryLock 模式"一个卡死的操作不拖住全线"的底线。
// wait <= 0 退化为纯 TryLock 语义(与 zcode 同款,给测试留"立即忙"断言口)。
func (p *HermesProvider) lockLife(wait time.Duration) bool {
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

// ensureClient 懒启动:定位运行时 → 拉适配器 → ACP initialize(能力快照)。
//
// 【有界两闸(R2,对齐 zcode withClientCtx / codex clientInitBudget)】
//  1. lifeMu 有界排队(lockLife):旧实现裸 Lock 无上界,前一轮初始化挂死持锁时
//     后续调用方全部无界排队。排队窗口 = min(hermesLockWait, 调用方自身剩余
//     截止时间),绝不超过调用方本来就愿意等待的时长,ctx 传导语义不受影响;
//     窗口耗尽回忙哨兵(人话文案,errors.Is→ErrBusy)。
//  2. initialize 握手专属预算(hermesInitBudget):新建预算 ctx,不复用调用方
//     ctx(常无截止时间);预算 ctx 的 cancel 经 defer 全路径释放;初始化失败时
//     已拉起的适配器进程一并 Close,不留半初始化客户端,lifeMu 随 defer 释放。
func (p *HermesProvider) ensureClient(ctx context.Context) (hermesClient, error) {
	// 排队窗口 = min(hermesLockWait, 调用方自己的剩余截止时间)(zcode 同款)。
	wait := hermesLockWait
	if d, hasDeadline := ctx.Deadline(); hasDeadline {
		if rem := time.Until(d); rem < wait {
			wait = rem
		}
	}
	if !p.lockLife(wait) {
		return nil, errHermesBusy
	}
	defer p.lifeMu.Unlock()

	if p.client != nil && p.client.Alive() {
		return p.client, nil
	}
	if p.client != nil {
		p.client.Close()
		p.client = nil
	}
	if p.newClient == nil {
		p.newClient = defaultNewHermesClient
	}
	// 握手专属预算:从 context.Background() 打底新建,与调用方 ctx 的取消/截止
	// 解耦 —— Run 的 ctx 常无 deadline,复用它 = 握手挂死该轮无界悬着。
	ictx, icancel := context.WithTimeout(context.Background(), hermesInitBudget)
	defer icancel()
	c, err := p.newClient()
	if err != nil {
		return nil, err
	}
	init, err := c.Initialize(ictx)
	if err != nil {
		c.Close() // 释放已拉起的适配器进程:初始化失败不占资源、不留半初始化引用
		return nil, fmt.Errorf("hermes ACP initialize: %w", err)
	}
	if init != nil {
		p.caps = init.AgentCapabilities
		p.agentVersion = init.AgentInfo.Version
	}
	p.client = c
	log.Printf("[hermes] ACP 适配器已就绪 caps(loadSession=%v list=%v resume=%v fork=%v)",
		p.caps.CanLoadSession(), p.caps.CanList(), p.caps.CanResume(), p.caps.CanFork())
	return c, nil
}

// Run 执行一轮。
func (p *HermesProvider) Run(ctx context.Context, req TurnRequest, sink Sink, approve ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	turnStart := time.Now()
	text := textInputText(req.InputBlocks)
	if strings.TrimSpace(text) == "" {
		return nil, nil, fmt.Errorf("hermes: 本轮没有文本输入")
	}
	p.mu.Lock()
	p.turnActive = true
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.turnActive = false
		p.mu.Unlock()
	}()

	// 【取消要通(H1)】turnMu 的获取必须感知 ctx:前一轮挂死持锁时,排队轮次
	// 的"停止"按钮会被这把锁吞掉(点了没反应 —— 真机实害)。拿不到就随 ctx
	// 退出;让位后旧轮若恰好释放,接锁即还(reaper),绝不带着锁空跑。
	// 解锁权单一所有者:两分支各自负责,绝不双解锁(select 两分支同时就绪时
	// 随机择一,若 defer 与 reaper 并存会对已解锁的 mutex 二次 Unlock,进程
	// 整体 fatal)。
	acquired := make(chan struct{})
	go func() { p.turnMu.Lock(); close(acquired) }()
	select {
	case <-acquired:
		// 本轮拿到锁,唯一解锁权在本函数
		defer p.turnMu.Unlock()
	case <-ctx.Done():
		// 让位:解锁权完全移交给 reaper,本函数绝不碰锁
		go func() { <-acquired; p.turnMu.Unlock() }()
		return nil, nil, ctx.Err()
	}

	c, err := p.ensureClient(ctx)
	if err != nil {
		return nil, nil, err
	}

	workspace := req.Workspace
	if workspace == "" {
		workspace, _ = os.Getwd()
	}

	// 【思考强度真实化(2026-10-02,时机优化:写入挪到 ensureSession 之前)】
	// hermes 的强度没有运行时 RPC,唯一入口是家目录 config.yaml 的
	// agent.reasoning_effort(会话创建那一刻读取)。写入提前到会话就绪前:
	//   - 新建会话:创建那一刻即读到新值 → "本轮已生效";
	//   - 重挂载会话(内存缓存命中 / ExternalSession 重挂载,历史已按旧强度
	//     落库):创建时刻已过 → 仍"下轮会话生效"。
	// 重挂载判定与 ensureSession 的分路同源(缓存命中且适配器活着,或
	// ExternalSession 在场)。用户在手机上显式选了强度 → 写该键(与手动编辑
	// config.yaml 等价,hermes 官方配置面)。空串 = 用户选"默认" = 删键回
	// hermes 默认。白名单校验(VALID_REASONING_EFFORTS)在写入前做,非法档
	// 宁可不写。
	if req.Thread != nil {
		if e := req.Thread.Effort; e != "" {
			resumed := false // 重挂载判定(与 ensureSession 三分支同源)
			if _, ok := p.threadRefFor(req.Thread.ID); ok && c.Alive() {
				resumed = true
			} else if req.Thread.ExternalSession.SessionID != "" {
				resumed = true
			}
			if hermesapp.IsValidReasoningEffort(e) {
				if werr := hermesapp.SetReasoningEffort(e); werr != nil {
					log.Printf("[hermes] 写强度 %s 失败(沿用 config 现值): %v", e, werr)
				} else {
					effortTiming := "下轮会话生效"
					if !resumed {
						effortTiming = "本轮已生效" // 新建会话在创建那一刻读 config
					}
					log.Printf("[hermes] 思考强度已设为 %s(%s)", e, effortTiming)
					// 【用户可见反馈(H3)】强度没有运行时 RPC;手机此刻看不到
					// config.yaml,静默会让"我明明改了强度"像没生效。写成功即发
					// 一条 STATUS 事件(手机端 codex 事件面板照常渲染,hermes ·
					// 执行计划 同通道),按新建/重挂载如实说清生效时机。
					sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
						Kind:    pb.CodexEvent_STATUS,
						Code:    "reasoning_effort_applied",
						Title:   "hermes · 思考强度",
						Message: fmt.Sprintf("已设为 %s,%s", e, effortTiming),
					}}})
				}
			} else {
				log.Printf("[hermes] 强度 %s 不在 hermes 档位表(minimal..ultra),跳过写入", e)
			}
		}
	}

	// 会话就绪:同代内存缓存 → ExternalSession 重挂载(session/load + 历史回放)
	// → 新建。手机重开历史会话续聊 = daemon 重启/适配器重启后的重挂载路径。
	// (强度写入已在此前完成:新建会话在 session/new 那一刻即读到新值。)
	sid, err := p.ensureSession(ctx, c, req, workspace)
	if err != nil {
		return nil, nil, err
	}

	// 模型选择:thread.ModelID("hermes/deepseek-flash" → "deepseek:deepseek-flash";
	// 会话内显式选择的 "hermes/deepseek:deepseek-flash" 剥前缀后带冒号原样透传)。
	// 默认档不发 set(config.yaml model.default 兜底),守卫口径对齐 codex:按
	// "/default" 后缀拦 —— 手机选"默认模型"落库的是带前缀完整 ID "hermes/default",
	// 只比 "default" 拦不住,会对默认档发出注定多余的 session/set_model 并误报
	// 切模型失败;裸 "default"(历史哨兵形)一并拦。
	if m := threadModelID(req.Thread); m != "" && !strings.HasSuffix(m, "/default") && m != "default" {
		acpModel := strings.TrimPrefix(m, "hermes/")
		if !strings.Contains(acpModel, ":") {
			acpModel = "deepseek:" + acpModel
		}
		if serr := c.SetSessionModel(ctx, sid, acpModel); serr != nil {
			log.Printf("[hermes] 切模型失败(按默认跑): %v", serr)
		}
	}

	// 当轮钩子 + 事件泵(codex/zcode/dsh/pi 同款"4096 缓冲队列 + 单消费者保序"):
	// readLoop 里同步调 notif 钩子,慢 sink(手机网络背压)/审批等待会堵死整条
	// 通知管道。分工:
	//   生产者(notif 闭包,readLoop goroutine):lastNotif 看门狗刻度(readLoop
	//     收到帧即算活,含别会话的帧)→ 解析 → 本会话过滤 → 入队。4096 满宁可
	//     阻塞保序不丢帧(四家同语义);notifStop 是收尸期逃生口。绝不触碰
	//     final/sink/toolIDs —— 它们只归消费者所有。
	//   消费者(单 goroutine):逐条 projectSessionUpdate。final/toolIDs/sink 只
	//     被这一个 goroutine 读写(旧实现 notif 在 readLoop 同步写 final、Run
	//     goroutine 并发读 —— 无锁数据竞争的 P0,由此根治)。
	//   收尸:先 close(notifStop) 再 <-notifDone,消费者退出后 Run 才读 final
	//     —— 这是消灭竞争的关键顺序(见 stopPump 与各 return 路径)。
	var final strings.Builder // 仅消费者 goroutine 写;收尸(stopPump)后 Run 才读
	toolIDs := map[string]int{} // ACP toolCallId → 序号(帧 Id 稳定;同 id start/update 配对;仅消费者访问)
	var toolMu sync.Mutex
	lastNotif := &atomic.Int64{} // 看门狗活跃刻度:readLoop 收到任何一帧 = 适配器还活着
	approvalWaiting := &atomic.Bool{}
	type hermesNotifMsg struct {
		ev  hermesapp.StreamEvent // 已解析的 session/update(生产者侧解析)
		raw json.RawMessage       // 原始帧(载荷对齐四家惯例,留诊断扩展)
	}
	notifCh := make(chan hermesNotifMsg, 4096)
	notifStop := make(chan struct{})
	notifDone := make(chan struct{})
	// enqueued/processed 组成排空栅栏(dsh 同款):update 通知与 prompt 应答分路
	// 竞速,终局先到时先把已入队帧消费完再读终稿,否则尾部正文可能丢。
	var enqueued, processed atomic.Int64
	notif := func(raw json.RawMessage) {
		lastNotif.Store(time.Since(turnStart).Milliseconds()) // 单调毫秒(距 turnStart),免疫墙钟跳变
		ev := hermesapp.ParseSessionUpdate(raw)
		if ev.SessionID != sid {
			return // 非本轮会话的帧(共享适配器上还有别的会话)不进本轮
		}
		enqueued.Add(1)
		select {
		case notifCh <- hermesNotifMsg{ev: ev, raw: raw}:
		case <-notifStop:
			enqueued.Add(-1) // 逃生未入队:回退计数,保排空栅栏账目平衡
		}
	}
	go func() {
		defer close(notifDone)
		for {
			select {
			case nm := <-notifCh:
				p.projectSessionUpdate(nm.ev, sink, &final, toolIDs, &toolMu)
				processed.Add(1)
			case <-notifStop:
				return
			}
		}
	}()
	reqHandler := func(id int64, method string, params json.RawMessage) {
		if method == "session/request_permission" {
			// 审批等待是合法静默(用户在手机上想多久都行),看门狗豁免。
			// req 钩子在 readLoop 里同步执行,等待期间通知流不动 —— 不豁免会把
			// "用户在审批" 误判成 "任务挂起" 而误杀轮次。
			approvalWaiting.Store(true)
			defer approvalWaiting.Store(false)
		}
		// req 路径的 sink 直接在 readLoop goroutine 上调用,不经队列 —— 与四家
		// 同款(codex handleCodexApproval / dsh handleDshRequest 亦如此):审批卡
		// 必须即时弹出,排进事件队列反会被慢消费冻结;它与消费者 goroutine 的
		// sink 并发由 sink 自身的并发安全约定承担(server 层同一通道),final/
		// toolIDs 在 req 路径不被触碰,不引入新的竞争面。
		p.handleRequestPermission(c, id, method, params, ctx, sink, approve)
	}
	// 【泵安装】从这里起 readLoop 的帧改走 notifCh 队列;卸载统一由 stopPump 负责。
	c.SetTurnHandlers(notif, reqHandler)
	// stopPump 收尸(幂等,sync.Once 防双 close):先卸钩子(断新帧入队)→
	// close(notifStop)(生产者逃生、消费者退出)→ <-notifDone(消费者真正退出)。
	// 此后 final 归 Run 独占,读点无竞争。挂在 defer 上兜底覆盖 Run 的每一条
	// return;凡要读 final 的路径,另在读取前显式调 stopPump()(return 语句的
	// 表达式先于 defer 求值,靠 defer 读不到"收尸后"的 final)。
	stopPump := sync.OnceFunc(func() {
		c.SetTurnHandlers(nil, nil)
		close(notifStop)
		<-notifDone
	})
	defer stopPump()

	// 【轮次取消看门狗(H1 接通 session/cancel)】prompt 阻塞到轮次终局,不能
	// 干等(dsh/codex 同款三闸;此前 ctx 取消只会让 daemon 侧放弃,适配器还在
	// 继续跑,下一轮 prompt 被挂起的真机实害由此而来):
	//   1. 手机取消帧 / CancelTurn / 重入打断(ctx 取消)→ 先发 session/cancel
	//      让适配器真停,再收 prompt 随 ctx 的返回;
	//   2. hermesSilenceWatchdog 无任何事件(审批等待豁免)→ 判挂起,发取消收场;
	//   3. hermesTurnCap 单轮上限 → 发取消收场。
	outCh := make(chan hermesPromptOutcome, 1)
	lastNotif.Store(time.Since(turnStart).Milliseconds()) // 初喂:单调毫秒,免疫墙钟跳变
	log.Printf("[hermes] session/prompt session=%s 输入=%d字", sid, len([]rune(text)))
	go func() {
		stop, err := c.Prompt(ctx, sid, text)
		outCh <- hermesPromptOutcome{stop: stop, err: err}
	}()

	tick := time.NewTicker(hermesWatchdogTick)
	defer tick.Stop()
	var res hermesPromptOutcome
	waitDone := false
	for !waitDone {
		select {
		case res = <-outCh:
			waitDone = true
		case <-ctx.Done():
			p.cancelTurn(c, sid)
			<-outCh // prompt 随 ctx 取消返回(带缓冲 channel,必达;等 goroutine 退出)
			// 收尸后再读 final(消费者已退出,独占读无竞争)。取消路径不排空
			// (dsh 同款:用户要的是"停",已落定的正文照交,未消费尾帧随泵弃)。
			stopPump()
			// 手机断连/取消也要交回已流出的内容:server 层落库后,重连才接得上看。
			return hermesPartialBlocks(&final), nil, ctx.Err()
		case <-tick.C:
			if approvalWaiting.Load() {
				continue // 审批等待是合法静默(用户在手机上想多久都行)
			}
			// 单调口径:ms 与判定两侧共用 turnStart(Run 开头,单调),差值即
			// 静默时长,免疫墙钟 NTP 跳变。ms>=0 同 codex:初喂在等待循环前
			// 必然发生,旧 ms>0 防的"未喂值"态已不可达,判定恒生效。
			if ms := lastNotif.Load(); ms >= 0 && time.Since(turnStart).Milliseconds()-ms > hermesSilenceWatchdog.Milliseconds() {
				p.cancelTurn(c, sid)
				return nil, nil, fmt.Errorf("hermes %v 无任何事件(任务可能未被接收,session=%s)", hermesSilenceWatchdog, sid)
			}
			if time.Since(turnStart) > hermesTurnCap {
				p.cancelTurn(c, sid)
				return nil, nil, fmt.Errorf("hermes 轮次超时(%v),已发取消", hermesTurnCap)
			}
		}
	}

	if res.err != nil {
		stopPump() // 收尸后再读 final(仅 ctx.Err 分支读);纯 prompt 错误分支靠 defer 收尸
		if ctx.Err() != nil {
			// 与应答竞速获胜的用户停:已流出的内容照常交回(不排空,dsh 同款)。
			return hermesPartialBlocks(&final), nil, ctx.Err()
		}
		return nil, nil, fmt.Errorf("hermes prompt: %w", res.err)
	}
	if hermesapp.StopReasonIsError(res.stop) {
		return nil, nil, fmt.Errorf("hermes 轮次被拒/被取消(stopReason=%s)", res.stop)
	}

	// 排空栅栏(dsh drainEvents 同款):先把已入队帧消费完(尾部正文不丢),
	// 再收尸 —— 消费者退出后 final 归 Run 独占,以下读点无竞争。
	drainEvents(&enqueued, &processed)
	stopPump()
	out := strings.TrimSpace(final.String())
	if out == "" {
		return nil, nil, fmt.Errorf("hermes 轮次没有产出正文(stopReason=%s);常见原因:hermes 安装目录的 config.yaml 未配置 model.default,或凭据无效", res.stop)
	}
	log.Printf("[hermes] 一轮完成 session=%s 用时 %v 回复 %d 字", sid,
		time.Since(turnStart).Round(time.Millisecond), len([]rune(out)))
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: out}}}},
		nil, nil
}

// hermesPromptOutcome 是 prompt goroutine 的收场(形状同构 dsh 的 promptOutcome)。
type hermesPromptOutcome struct {
	stop string
	err  error
}

// ---- 时长预算(与 dsh/codex 对表;包级变量仅为测试注入短值)----
var (
	hermesTurnCap         = 60 * time.Minute  // 单轮上限(对齐 codex/dsh)
	hermesSilenceWatchdog = 150 * time.Second // 无任何事件的看门狗(对齐 codex/dsh)
	hermesWatchdogTick    = 10 * time.Second  // 看门狗检查节奏
	// hermesCancelBudget 发 session/cancel 的写入上界(通知帧只是一次管道写;
	// 适配器挂死不读管道时宁可放弃,不陪葬轮次收尾)。
	hermesCancelBudget = 5 * time.Second
	// hermesLockWait lifeMu 的有界排队窗口(zcodeLockWait/dshLockWait 同档 15s;
	// 排队而非立即拒绝的依据见 lockLife 注释)。包级变量仅为测试注入短值,
	// 与 zcodeLockWait 同惯例。
	hermesLockWait = 15 * time.Second
)

// hermesPartialBlocks 已流出的正文(codex/dsh "手机断连也要交回已流出的内容"
// 同款);空返回 nil。
func hermesPartialBlocks(final *strings.Builder) []pb.ContentBlock {
	partial := strings.TrimSpace(final.String())
	if partial == "" {
		return nil
	}
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: partial}}}}
}

// ---- 轮次取消(session/cancel 的接线)----

// cancelTurn 向适配器发 session/cancel(有界预算,fire-and-forget)。【必须用
// 新鲜的有界 ctx】三个闸口里 ctx 取消那路的 ctx 已经死了 —— 拿它发取消帧永远
// 发不出去,适配器继续跑,轮次"停了又没停"。
func (p *HermesProvider) cancelTurn(c hermesClient, sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), hermesCancelBudget)
	defer cancel()
	if err := c.CancelSession(ctx, sessionID); err != nil {
		log.Printf("[hermes] session/cancel 发送失败(session=%s): %v", sessionID, err)
		return
	}
	log.Printf("[hermes] 已发 session/cancel(session=%s)", sessionID)
}

// Steer 实现 agent.Steerer。hermes 的 ACP 面没有运行中插话(server.py 只有
// /steer slash 文本重写,属适配器会话内命令,不是协议方法,daemon 不做文本级
// 模拟),一律返回 steered=false,server 层按既有排队语义把输入留到当前轮
// 终局后续跑(dsh 同款)。真正的"停止"不走这里:Run 的取消看门狗把 ctx 取消/
// 静默超时翻译成 session/cancel(见 Run)。
func (p *HermesProvider) Steer(context.Context, string, []pb.ContentBlock) (bool, string, error) {
	return false, "", nil
}

// projectSessionUpdate 把一条已解析的 session/update 投影到 sink(只从 Run 的
// 事件泵单消费者 goroutine 里调;只处理本轮会话的帧。final/toolIDs/sink 仅被
// 这个 goroutine 读写,是消灭 P0 数据竞争的所有权边界)。
func (p *HermesProvider) projectSessionUpdate(ev hermesapp.StreamEvent, sink Sink, final *strings.Builder, toolIDs map[string]int, toolMu *sync.Mutex) {
	switch {
	case ev.MessageChunk != "":
		final.WriteString(ev.MessageChunk)
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{
			TextDelta: &pb.TextDelta{Delta: ev.MessageChunk}}})
	case ev.ThoughtChunk != "":
		if !p.hideReasoning() {
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{
				ThinkingDelta: &pb.ThinkingDelta{Delta: ev.ThoughtChunk}}})
		}
	case ev.ToolCall != nil:
		tc := ev.ToolCall
		toolMu.Lock()
		id, known := toolIDs[tc.ToolCallID]
		if !known {
			id = len(toolIDs) + 1
			toolIDs[tc.ToolCallID] = id
		}
		toolMu.Unlock()
		phase := pb.ToolCallUpdate_UPDATE
		switch {
		case tc.Kind == "tool_call":
			phase = pb.ToolCallUpdate_STARTED
		case tc.Status == "completed" || tc.Status == "failed":
			phase = pb.ToolCallUpdate_FINISHED
		}
		args := map[string]any{"output": clipString(tc.Output, 2000)}
		if tc.Diff != nil {
			args["diff"] = map[string]string{"path": tc.Diff.Path, "oldText": tc.Diff.OldText, "newText": tc.Diff.NewText}
		}
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
			Phase: phase,
			Call: &pb.ToolCallBlock{
				Id:       fmt.Sprintf("hermes-%s-%d", clipString(ev.SessionID, 8), id),
				Name:     tc.Title,
				JsonArgs: mustJSON(args),
			},
		}}})
	case len(ev.Plan) > 0:
		steps := make([]string, 0, len(ev.Plan))
		for _, e := range ev.Plan {
			steps = append(steps, planStatusWord(e.Status)+"|"+e.Content)
		}
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
			Kind:      pb.CodexEvent_STATUS,
			Code:      "native_notification",
			Title:     "hermes · 执行计划",
			PlanSteps: steps,
		}}})
	case ev.Usage != nil:
		// 【usage_update 投影(H1)】size=上下文窗,used=窗口占用 → 手机
		// ContextUsageCard 的数据源(zcode/dsh 同一条 TurnServerMessage_Usage
		// 通道;InputTokens 承载"当前窗口占用",codex last/dsh used 同口径,
		// ModelContextWindow 对齐 zcode 的窗口位 —— 手机端 usage+窗口算占用环)。
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Usage{Usage: &pb.UsageUpdate{
			Usage:              &pb.Usage{InputTokens: ev.Usage.Used},
			ModelContextWindow: ev.Usage.Size,
		}}})
	}
	// user_message_chunk(排队回显/历史回放)/ current_mode_update /
	// available_commands_update / session_info_update:解析层已具名(daemon
	// 暂无 sink 消费方),不透传、不误当未知。
}

// ---- 会话就绪(重挂载握手纪律:校验不能省,dsh 同款)----

// ensureSession 优先级:内存缓存(适配器同进程)→ ExternalSession 重挂载
// (session/load,历史回放)→ 新建。
func (p *HermesProvider) ensureSession(ctx context.Context, c hermesClient, req TurnRequest, workspace string) (string, error) {
	if req.Thread != nil {
		if ref, ok := p.threadRefFor(req.Thread.ID); ok && c.Alive() {
			return ref.sessionID, nil
		}
		if ext := req.Thread.ExternalSession; ext.SessionID != "" {
			return p.remountSession(ctx, c, req.Thread, ext, workspace)
		}
		return p.createSession(ctx, c, req.Thread, workspace)
	}
	// 无 thread 的一次性轮次:维持既有行为,新建即弃。
	return p.createSession(ctx, c, nil, workspace)
}

// createSession session/new 新建并登记:threadMap 写入 + ExternalSession 写回
// (server 层轮次后落库;zcode/dsh 同款机制)。th 为 nil(一次性轮次)不登记。
func (p *HermesProvider) createSession(ctx context.Context, c hermesClient, th *store.Thread, workspace string) (string, error) {
	st, err := c.SessionNew(ctx, workspace)
	if err != nil {
		return "", fmt.Errorf("hermes session/new: %w", err)
	}
	if th != nil {
		p.setThreadRef(th.ID, hermesThreadRef{sessionID: st.SessionID})
		th.ExternalSession = store.ExternalSessionRef{SessionID: st.SessionID, Cwd: workspace}
	}
	log.Printf("[hermes] 新建会话 %s cwd=%s", st.SessionID, workspace)
	return st.SessionID, nil
}

// remountSession daemon/适配器重启后的重挂载:
//  1. 能力门控:initialize 的 loadSession 不在场 → 显式报错(§5.2,不猜协议);
//  2. session/load 严格校验:查无此会话(应答 null)→ 显式报错,不静默新开
//     ("不 resume 就静默吞请求"的三方共同教训;hermes 的 session/resume 会
//     自动新建,掩盖数据错位,daemon 不用它做重挂载);
//  3. 历史回放经通知钩子流出 —— 换弃帧收集器承接(手机端消息早已落库,回放
//     只为对账/日志计数,重投影会在手机端重复出气泡)。
func (p *HermesProvider) remountSession(ctx context.Context, c hermesClient, th *store.Thread, ext store.ExternalSessionRef, workspace string) (string, error) {
	if !p.caps.CanLoadSession() {
		return "", fmt.Errorf("hermes %s 不支持会话重挂载(agentCapabilities.loadSession 不在场),无法恢复会话 %s;请新建会话",
			p.agentVersionLabel(), ext.SessionID)
	}
	cwd := ext.Cwd
	if cwd == "" {
		cwd = workspace
	}
	// 弃帧收集器:load 的历史回放(user/assistant/thought chunk + tool_call 对)
	// 在应答前流出,只计数不投影。
	var replayed int
	c.SetTurnHandlers(func(json.RawMessage) { replayed++ }, nil)
	st, err := c.LoadSession(ctx, cwd, ext.SessionID)
	c.SetTurnHandlers(nil, nil)
	if err != nil {
		if errors.Is(err, hermesapp.ErrSessionNotFound) {
			return "", fmt.Errorf("hermes 会话 %s 不在持久库中(session/load 查无),不静默新开会话;如需继续请新建会话", ext.SessionID)
		}
		return "", fmt.Errorf("hermes session/load %s: %w", ext.SessionID, err)
	}
	p.setThreadRef(th.ID, hermesThreadRef{sessionID: ext.SessionID})
	if ext.Cwd == "" {
		th.ExternalSession.Cwd = workspace // 补齐落库引用的 cwd(轮次后由 server 落库)
	}
	log.Printf("[hermes] 已重挂载会话 %s(历史回放 %d 帧,当前模型 %q)", ext.SessionID, replayed, st.Models.CurrentModelID)
	return ext.SessionID, nil
}

func (p *HermesProvider) agentVersionLabel() string {
	if p.agentVersion == "" {
		return "适配器"
	}
	return "适配器 " + p.agentVersion
}

// ---- 审批链(session/request_permission 反向请求 → 手机审批卡)----

// hermesApprovalTimeout 审批卡等待手机裁决的窗口(与 dshApprovalTimeout 同款:
// 超时按拒绝处理 —— 失败安全方向;适配器侧自己的 approvals.timeout 默认 300s,
// daemon 先答"拒绝"也不会悬挂)。
var hermesApprovalTimeout = 2 * time.Minute

// handleRequestPermission 反向请求分派,三条规则与 dsh/codex 同款:
//  1. 不认识的方法回 JSON-RPC error,绝不伪造 decision;
//  2. 审批卡按请求真实内容构造,不猜不编造;
//  3. 超时/断线/通道故障 → 拒绝侧,绝不伪造"同意"。
func (p *HermesProvider) handleRequestPermission(c hermesClient, id int64, method string, params json.RawMessage, ctx context.Context, sink Sink, approve ApprovalHook) {
	if method != "session/request_permission" {
		log.Printf("[hermes] %s → 回 error(未实现,不伪造 decision)", method)
		_ = c.RespondError(id, -32601, "PocketAgent 未实现该反向请求: "+method)
		return
	}
	perm, err := hermesapp.ParseRequestPermission(params)
	if err != nil {
		log.Printf("[hermes] %v → 回 error", err)
		_ = c.RespondError(id, -32602, err.Error())
		return
	}
	allowID, rejectID := perm.AllowOptionID(), perm.RejectOptionID()
	if allowID == "" || rejectID == "" {
		log.Printf("[hermes] 审批选项表无法识别(allow=%q reject=%q)→ 回 error", allowID, rejectID)
		_ = c.RespondError(id, -32602, "hermes 审批选项表无法识别 allow/reject 侧,拒绝代答")
		return
	}
	if approve == nil {
		// 审批通道缺失(诊断 RPC 等路径):基础设施问题不伪装成用户意志
		log.Printf("[hermes] 审批通道不可用(approve==nil),回 error")
		_ = c.RespondError(id, -32000, "审批通道不可用")
		return
	}

	call := buildHermesPermissionCall(perm)
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ApprovalRequest{ApprovalRequest: &pb.ApprovalRequest{
		RequestId: fmt.Sprintf("hermes-%d", id),
		Action:    call,
		Reason:    "hermes 请求授权(一次性 allow/deny;allow_session/allow_always 由桌面端 Hermes 自己管)",
	}}})

	askCtx, cancel := context.WithTimeout(ctx, hermesApprovalTimeout)
	defer cancel()
	allow, _, askErr := approve(askCtx, call)
	switch {
	case askErr != nil:
		if errors.Is(askErr, context.DeadlineExceeded) {
			log.Printf("[hermes] 审批 %v 无应答,按拒绝处理(request=%s)", hermesApprovalTimeout, call.GetId())
		} else {
			log.Printf("[hermes] 审批通道失败,按拒绝处理: %v", askErr)
		}
		_ = c.Respond(id, hermesapp.PermissionOutcomeSelected(rejectID))
	case allow:
		_ = c.Respond(id, hermesapp.PermissionOutcomeSelected(allowID))
	default:
		_ = c.Respond(id, hermesapp.PermissionOutcomeSelected(rejectID))
	}
}

// buildHermesPermissionCall 构造审批卡:动作名用 hermes 原生 title(缺失时用
// 通用名),rawInput/编辑提案原样透出,绝不编造具体命令。
func buildHermesPermissionCall(perm *hermesapp.RequestPermissionParams) *pb.ToolCallBlock {
	id, name := "hermes-permission", "hermes_permission"
	args := map[string]any{}
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
			var input any
			_ = json.Unmarshal(perm.ToolCall.RawInput, &input)
			args["input"] = input
		}
	}
	return &pb.ToolCallBlock{Id: id, Name: name, JsonArgs: mustJSON(args)}
}

func (p *HermesProvider) hideReasoning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.prefs.HideReasoning
}

func (p *HermesProvider) setThreadRef(id string, ref hermesThreadRef) {
	p.threadMu.Lock()
	defer p.threadMu.Unlock()
	if p.threadMap == nil {
		p.threadMap = map[string]hermesThreadRef{}
	}
	p.threadMap[id] = ref
}

func (p *HermesProvider) threadRefFor(id string) (hermesThreadRef, bool) {
	p.threadMu.Lock()
	defer p.threadMu.Unlock()
	ref, ok := p.threadMap[id]
	return ref, ok
}

func planStatusWord(acpStatus string) string {
	switch strings.ToLower(acpStatus) {
	case "completed":
		return "已完成"
	case "in_progress":
		return "正在执行"
	default:
		return "未完成"
	}
}

func textInputText(blocks []pb.ContentBlock) string {
	var b strings.Builder
	for i := range blocks {
		if t := blocks[i].GetText(); t != nil && t.Text != "" {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

func threadModelID(t *store.Thread) string {
	if t == nil {
		return ""
	}
	return t.ModelID
}

func clipString(s string, n int) string {
	// 统一走 runtime.ClipRunes,收敛见审查报告 util 节。
	return runtime.ClipRunes(s, n)
}
