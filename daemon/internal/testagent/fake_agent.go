// Package testagent 提供 deterministic fake agent:按 Scenario 脚本回放的
// agent.Provider 实现,供发布验证(Task 2/3 的 daemon E2E)把任意 agent id
// 顶替成行为完全可控的假 agent。
//
// 【与 echo provider 的分工】internal/agent 的 EchoProvider 是"产品内置的演示
// agent"(固定剧情、真调一次 list_dir、面向真实用户);本包的 FakeAgent 是
// **测试装置**(剧情逐字脚本化、绝不做真 IO、面向 E2E 断言)。两者并存:
// 生产二进制不 import 本包(internal/ 各包不得反向依赖 testagent),测试按需
// 经 server.Config.Providers 灌入 —— provider 表是唯一灌入点,无旁路。
//
// 【同构保证】FakeAgent 实现的面与真 provider 一致:Provider 基础四方法之外,
// 还实现 server 层会类型断言探测的 Steerer(运行中插话)与 CollabAware(协同
// 委派能力)—— 编译期断言见文件底部。server 层新增可选能力探测时,这里必须
// 并列补断言与语义(internal/agent/provider.go 的 seam 注释与本块互指)。
//
// 【审批语义红线】provider 侧从不伪造审批结果:FakeAgent 只负责"发起审批并
// 等待",决定(允许/拒绝/超时)永远来自**注入的 ApprovalHook** —— server E2E
// 里是真手机链路(2min 预算在 server 的 approve 钩子里),fake 单测里是测试
// 脚本化的 hook,"超时"脚本 = ApprovalScenario.Budget>0 时 fake 给 ask 套短
// 预算 ctx,让阻塞型 hook 按DeadlineExceeded 返回。绝不自动替用户做危险审批
// (计划全局约束第 4 条)。
//
// 【确定性】无随机;Factory.Now 可注入时钟,但只用于观测打点(审批请求/应答
// 时刻),回放节奏由真实 timer 驱动(测试用毫秒级);每段等待都可被 ctx 取消
// 打断;并发安全(观察面全部走互斥锁),后台 goroutine 有界退出(WaitIdle 可等)。
package testagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// ---- Scenario:剧情脚本(spec 接口,字段冻结)----

// Scenario 描述一个 fake agent 的完整剧情。
type Scenario struct {
	// Name 是剧情的人读名(测试报告/证据文档引用)。
	Name string
	// Events 是按序回放的事件清单(构造器见下方 AgentEvent)。回放在终局事件
	// (Completed/Failed)处停止;其后的列表项不再消费。
	Events []AgentEvent
	// Delay 是**事件间**的间隔(首个事件前不等待;显式节拍用 Pause)。
	// 每段等待都可被 ctx 取消打断,也会被 ExitAfter 竞速截断。
	Delay time.Duration
	// Approval 非空时,Run 一开始就发起一次审批(卡面与等待节奏取自脚本;
	// 允许/拒绝/超时的**决定**由注入的 ApprovalHook 给出,见包注释红线)。
	// 未知反向请求事件(UnknownRequest)复用同一份等待节奏。
	Approval *ApprovalScenario
	// ExitAfter 非零时,模拟"轮中子进程退出":Run 开始后经过这么久,回放在
	// 最近一个可中断点停止,以 provider 错误(ExitError,EOF/exit status 语义)
	// 返回 —— server 层据此发 Failed 帧,与真 agent 崩溃同构。
	ExitAfter time.Duration
	// IgnoreStdin 模拟"进程僵死、stdin 写入被吞":Steer 照常 ack(真僵死的
	// codex 也会收下 stdin)但**不产生任何效果** —— 不发事件、不改回放,
	// 上层只能靠静默看门狗/取消收场。false 时 Steer 正常消费:立即回一句
	// 可断言的文本增量。
	IgnoreStdin bool
}

// ApprovalScenario 描述一次审批的卡面与等待节奏。
//
// 【决定从哪来】允许/拒绝/超时三种结局不是本结构体"执行"出来的,而是
// ApprovalHook 的三种可能返回:hook 返回 (true,…) = 批准;(false,…) = 拒绝;
// Budget>0 时 fake 给 ask 套短预算 ctx,阻塞型 hook 到点按 DeadlineExceeded
// 返回 = 超时(把 server 侧 2min 审批预算压缩成毫秒级的手段;Budget=0 表示
// 不套预算 —— server E2E 用真实 2min 链路时必须留零)。
type ApprovalScenario struct {
	// CallName 是审批卡动作名(空 = "shell")。动作名决定手机端卡片渲染与
	// ApprovalReason 文案,与 toolNameShell 等既有名字同一空间。
	CallName string
	// JSONArgs 是卡参数原文(空 = {"command":"echo fake-approval"})。
	JSONArgs string
	// DecideAfter 是发起审批后、真正调用 hook 前的挂起时长(0 = 立即)。
	// 用于"成员等待审批,其他成员继续"这类时序编排的可观测锚点。
	DecideAfter time.Duration
	// Budget>0 = 超时路脚本:给 ask 套这么长的预算 ctx(测试用毫秒级);
	// 0 = 不套预算,决定何时给就等到何时。
	Budget time.Duration
}

// ---- AgentEvent:回放步 ----

// AgentEvent 是一个回放步。
//
// 【为什么在本包定义而不是"复用"】规格写"复用 internal/agent 的 AgentEvent",
// 但该类型在代码库中不存在;provider 边界的真实事件面 = pb.TurnServerMessage
// 各帧 + Run 的返回三元组(终稿块/用量/错误)。本类型即按这个真实面对齐的
// 脚本词汇表:每类事件要么 sink 一帧、要么记一条台账、要么终结回放。
type AgentEvent interface{ isAgentEvent() }

func (textEvent) isAgentEvent()                {}
func (thinkingEvent) isAgentEvent()            {}
func (toolCallEvent) isAgentEvent()            {}
func (toolResultEvent) isAgentEvent()          {}
func (usageEvent) isAgentEvent()               {}
func (codexEventFrame) isAgentEvent()          {}
func (unknownNotificationEvent) isAgentEvent() {}
func (unknownRequestEvent) isAgentEvent()      {}
func (unknownResponseEvent) isAgentEvent()     {}
func (pauseEvent) isAgentEvent()               {}
func (lateEvent) isAgentEvent()                {}
func (completedEvent) isAgentEvent()           {}
func (failedEvent) isAgentEvent()              {}

// ---- 过程帧事件 ----

type textEvent struct{ delta string }

// Text 回放一帧文本增量(TextDelta)。
func Text(delta string) AgentEvent { return textEvent{delta: delta} }

type thinkingEvent struct{ delta string }

// Thinking 回放一帧思考增量(ThinkingDelta)。
func Thinking(delta string) AgentEvent { return thinkingEvent{delta: delta} }

type toolCallEvent struct{ card *pb.ToolCallBlock }

// ToolCall 回放一帧工具调用开始(ToolCallUpdate_STARTED)。
func ToolCall(id, name, jsonArgs string) AgentEvent {
	return toolCallEvent{card: &pb.ToolCallBlock{Id: id, Name: name, JsonArgs: jsonArgs}}
}

// ToolCallCard 用现成卡回放工具调用开始(需要精细卡面时用)。
func ToolCallCard(card *pb.ToolCallBlock) AgentEvent { return toolCallEvent{card: card} }

type toolResultEvent struct {
	callID string
	ok     bool
	output string
}

// ToolResult 回放一帧工具结果(ToolCallUpdate_FINISHED)。卡面复用本轮里同
// callID 的 STARTED 卡(与 echo provider 的帧序同构);找不到就只带 Id。
func ToolResult(callID string, ok bool, output string) AgentEvent {
	return toolResultEvent{callID: callID, ok: ok, output: output}
}

type usageEvent struct{ usage *pb.Usage }

// Usage 回放一帧用量(UsageUpdate),并作为 Run 返回的用量。
func Usage(u *pb.Usage) AgentEvent { return usageEvent{usage: u} }

// UsageOf 是 Usage 的便捷构造(inputTokens/outputTokens 两项)。
func UsageOf(input, output int64) AgentEvent {
	return Usage(&pb.Usage{InputTokens: input, OutputTokens: output})
}

type codexEventFrame struct{ evt *pb.CodexEvent }

// CodexEventFrame 直通一帧原生事件(CodexEvent:重试/降级/diff 等呈现用)。
func CodexEventFrame(evt *pb.CodexEvent) AgentEvent { return codexEventFrame{evt: evt} }

// ---- 未知事件(台账路,不伪造帧)----

type unknownNotificationEvent struct {
	method  string
	payload string
}

// UnknownNotification 记一条未知通知进运行时台账(UNKNOWN_NOTIFICATION)。
// 【与真 agent 同构】未知通知不产生协议帧、不结束轮次:回放继续走后续事件,
// 诊断信息只落台账(server 层经未知事件 RPC 透出)。
func UnknownNotification(method, rawPayload string) AgentEvent {
	return unknownNotificationEvent{method: method, payload: rawPayload}
}

type unknownRequestEvent struct {
	method string
	params string
}

// UnknownRequest 模拟"agent 发来一个 daemon 不认识的反向请求":按
// unknown_request 通用确认卡的既有形状(internal/agent 的
// buildUnknownRequestCall:reason/method/payload)向注入的 ApprovalHook 发卡
// 等待;结局按真实语义分流台账 —— 允许 = KNOWN_PASSTHROUGH(尽力应答空对象),
// 拒绝/超时/通道缺失 = UNKNOWN_REQUEST(安全拒绝,绝不伪造批准)。
func UnknownRequest(method, rawParams string) AgentEvent {
	return unknownRequestEvent{method: method, params: rawParams}
}

type unknownResponseEvent struct {
	correlationID string
	raw           string
}

// UnknownResponse 记一条无法关联到在途请求的响应(UNKNOWN_RESPONSE)。
func UnknownResponse(correlationID, raw string) AgentEvent {
	return unknownResponseEvent{correlationID: correlationID, raw: raw}
}

// ---- 节拍与迟到 ----

type pauseEvent struct{ d time.Duration }

// Pause 在回放中插入一段显式静默(与 Scenario.Delay 无关的额外节拍)。
// 静默期间无任何帧 —— 这就是"静默看门狗"场景的注入手段;等待可被 ctx 取消
// 打断,也会被 Scenario.ExitAfter 竞速截断。
func Pause(d time.Duration) AgentEvent { return pauseEvent{d: d} }

type lateEvent struct {
	after time.Duration
	inner AgentEvent
}

// Late 声明一个"迟到补发":从回放推进到该声明处起,再等 after 才试图发出
// inner。到点时若轮次已终局(Completed/Failed 已处理)或已取消,补发**被拒**:
// 不发出、迟到计数 +1 —— 这是"调用方超时放弃后上游才补发响应"的 provider
// 侧同构(真实治理先例:codex 迟到应答按调用代号静默丢弃)。
// 判定以"到点瞬间终局闸是否已闭"为准;赶上了就正常发出。
func Late(after time.Duration, ev AgentEvent) AgentEvent { return lateEvent{after: after, inner: ev} }

// ---- 终局事件 ----

type completedEvent struct{ blocks []pb.ContentBlock }

// Completed 终局:Run 以给定的文本块返回(nil 错误)。回放到此停止。
func Completed(texts ...string) AgentEvent {
	blocks := make([]pb.ContentBlock, 0, len(texts))
	for _, t := range texts {
		blocks = append(blocks, pb.ContentBlock{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: t}}})
	}
	return completedEvent{blocks: blocks}
}

// CompletedBlocks 终局:Run 以现成块返回(图片等非文本终稿用)。
func CompletedBlocks(blocks []pb.ContentBlock) AgentEvent { return completedEvent{blocks: blocks} }

type failedEvent struct{ err error }

// Failed 终局:Run 返回该错误(server 层据此发 Failed 帧 —— 与真 provider
// 失败同构;fake 绝不自发 Failed 帧,那不是 provider 的职责)。
func Failed(err error) AgentEvent { return failedEvent{err: err} }

// FailedText 是 Failed 的便捷构造。
func FailedText(msg string) AgentEvent { return Failed(errors.New(msg)) }

// ---- ExitError:进程退出的 provider 错误 ----

// ExitError 模拟"子进程退出"的 provider 错误:Error() 是 EOF/exit status 的
// 人话形态,ErrorInfo() 让 server 把结构化诊断(Code=AGENT_EXITED)保留进
// TurnFailed 帧(agent.ErrorInfoCarrier 的既有通道)。
type ExitError struct {
	AgentID string
	Code    int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("fake agent %s: 子进程意外退出(EOF): exit status %d", e.AgentID, e.Code)
}

// ErrorInfo 实现 agent.ErrorInfoCarrier。
func (e *ExitError) ErrorInfo() *pb.Error {
	return &pb.Error{
		Code:      "AGENT_EXITED",
		Message:   "桌面 agent 进程已退出",
		Details:   e.Error(),
		Retryable: true, // 进程退出可通过重新发起轮次恢复,手机端允许重试
	}
}

// ---- Factory:spec 的 FakeAgentFactory ----

// FakeAgentFactory 按 agent id + 剧情构造 fake provider(spec 接口,签名冻结)。
type FakeAgentFactory interface {
	Provider(id string, scenario Scenario) agent.Provider
}

// Factory 是 FakeAgentFactory 的唯一实现。
type Factory struct {
	// Now 是可注入时钟(nil = time.Now),只用于观测打点(审批请求/应答时刻),
	// 不改变回放节奏 —— 节奏由 Scenario 的真实 timer 驱动。
	Now func() time.Time
	// Ledger 是未知事件台账的后端(nil = agent.SharedRuntimeBackend(),即与
	// 生产/server E2E 同一本账)。fake 自身的单测注入 runtime.NewBackend()
	// 做隔离断言。
	Ledger *runtime.Backend
	// CollabReadyDefault 是本工厂产出的 fake 对 agent.CollabAware 的回答
	// (协同 E2E 用它把成员标记为"已具备委派接线";与"接线事实"的判定分工见
	// provider.go 的 CollabAware 注释 —— fake 的接线事实就是这条配置)。
	CollabReadyDefault bool
}

var _ FakeAgentFactory = (*Factory)(nil)

// Provider 实现 FakeAgentFactory。
func (f *Factory) Provider(id string, sc Scenario) agent.Provider {
	return &FakeAgent{id: id, sc: sc, factory: f, enteredCh: make(chan struct{}, 16)}
}

// ---- FakeAgent ----

// ApprovalObs 是一次审批等待的观察记录(测试断言用)。
type ApprovalObs struct {
	Call        *pb.ToolCallBlock // 发出的卡
	Allow       bool              // hook 给出的决定
	Note        string            // hook 附带备注
	Err         error             // hook 错误(nil = 通道正常;DeadlineExceeded = 超时路)
	RequestedAt time.Time         // 发卡时刻(Factory.Now 打点)
	ResolvedAt  time.Time         // 收到决定的时刻
}

// RecordedFrame 是一帧已发出帧及其所属轮次(断言"重试后帧序"用;Steer 发出的
// 帧记在当轮,无轮可挂时 run=-1)。
type RecordedFrame struct {
	Run int
	Msg *pb.TurnServerMessage
}

// FakeAgent 是按 Scenario 回放的脚本化 provider。
//
// 同一个实例可被多次 Run(server 重试语义):每次回放逐字一致,观察面按轮次
// 累积 —— 确定性回放 + 可累积观测,正是"取消和重试"场景需要的组合。
type FakeAgent struct {
	id        string
	sc        Scenario
	factory   *Factory
	enteredCh chan struct{} // 每次 Run 启动的信号(带缓冲,非阻塞发)

	mu          sync.Mutex
	runs        int                 // Run 调用次数
	frames      []RecordedFrame     // 已发出的全部帧(按轮次)
	steered     [][]pb.ContentBlock // Steer 收到的输入
	approvals   []*ApprovalObs      // 审批观察
	lateDropped int                 // 迟到补发被拒的次数
	wg          sync.WaitGroup      // 后台 goroutine(迟到补发计时器)
}

// 编译期钉死:fake 与真 provider 的能力面同构(server 层新增可选能力探测时,
// 这里与 internal/agent/provider.go 的 TestFakeSurface 契约必须同步补)。
var (
	_ agent.Provider         = (*FakeAgent)(nil)
	_ agent.Steerer          = (*FakeAgent)(nil)
	_ agent.CollabAware      = (*FakeAgent)(nil)
	_ agent.ErrorInfoCarrier = (*ExitError)(nil)
	// 镜像断言:provider.go 的测试工厂 seam 契约 —— fake 必须能整体顶替真
	// provider 的完整探测面。
	_ agent.TestFakeSurface = (*FakeAgent)(nil)
)

// ID 实现 agent.Provider。
func (a *FakeAgent) ID() string { return a.id }

// DisplayName 实现 agent.Provider。
func (a *FakeAgent) DisplayName() string { return "Fake(" + a.id + ")" }

// Models 实现 agent.Provider:最小目录(id=m1),足以让 server 的默认模型解析
// 出结果(collabTestProvider 同款先例)。
func (a *FakeAgent) Models() []pb.ModelInfo {
	return []pb.ModelInfo{{
		Id: "m1", Provider: a.id, DisplayName: "Fake " + a.id,
		ContextWindow: 128_000, Available: true,
	}}
}

// CollabReady 实现 agent.CollabAware。
func (a *FakeAgent) CollabReady() bool {
	return a.factory != nil && a.factory.CollabReadyDefault
}

// Steer 实现 agent.Steerer(运行中插话)。
//
// IgnoreStdin=false:ack 并立即回一帧可断言的文本增量("[steered] …"),证明
// 输入被"消费";true:照常 ack(真僵死的 codex 也会收下 stdin 并回执)但吞掉
// 输入、不产生任何效果 —— 上层只见 Steered 帧却永远等不到后续,只能靠静默
// 看门狗/取消收场。两种形态都记录输入供断言。
func (a *FakeAgent) Steer(_ context.Context, _ string, blocks []pb.ContentBlock) (bool, string, error) {
	cp := append([]pb.ContentBlock(nil), blocks...)
	a.mu.Lock()
	a.steered = append(a.steered, cp)
	run := a.runs - 1 // 当轮(尚未开轮时为 -1,与 RecordedFrame 的约定一致)
	a.mu.Unlock()
	if a.sc.IgnoreStdin {
		return true, a.turnID(run), nil // 吞掉:无帧、无效果
	}
	a.emitWith(nil, run, &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{
		Delta: "[steered] 已插入运行中的轮次",
	}}})
	return true, a.turnID(run), nil
}

// ---- 观察面(并发安全)----

// EnterCh 返回 Run 启动信号通道(带缓冲,不阻塞生产者)。
func (a *FakeAgent) EnterCh() <-chan struct{} { return a.enteredCh }

// Runs 返回 Run 被调用的次数。
func (a *FakeAgent) Runs() int { a.mu.Lock(); defer a.mu.Unlock(); return a.runs }

// Frames 迄今发出的全部帧(按发出顺序,含轮次标号)。
func (a *FakeAgent) Frames() []RecordedFrame {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]RecordedFrame(nil), a.frames...)
}

// RunFrames 返回第 i 次 Run(从 0 起)发出的帧。
func (a *FakeAgent) RunFrames(i int) []*pb.TurnServerMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []*pb.TurnServerMessage
	for _, f := range a.frames {
		if f.Run == i {
			out = append(out, f.Msg)
		}
	}
	return out
}

// SteeredInputs 返回 Steer 收到的全部输入(按序)。
func (a *FakeAgent) SteeredInputs() [][]pb.ContentBlock {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([][]pb.ContentBlock(nil), a.steered...)
}

// Approvals 返回全部审批观察记录。
func (a *FakeAgent) Approvals() []*ApprovalObs {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]*ApprovalObs(nil), a.approvals...)
}

// LateDropped 返回迟到补发被拒的次数。
func (a *FakeAgent) LateDropped() int { a.mu.Lock(); defer a.mu.Unlock(); return a.lateDropped }

// WaitIdle 等待本 fake 的全部后台 goroutine(迟到补发计时器)退出。
// 返回 false = 超时仍有存活(即泄漏,测试应当失败)。
func (a *FakeAgent) WaitIdle(timeout time.Duration) bool {
	ch := make(chan struct{})
	go func() { a.wg.Wait(); close(ch) }()
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

// ---- Run:回放主循环 ----

// stopReason 是一次可中断等待的结束原因。
type stopReason int

const (
	stopContinue stopReason = iota
	stopCanceled            // ctx 取消
	stopExited              // ExitAfter 到点(进程退出)
)

// Run 实现 agent.Provider:按脚本回放,返回终稿三元组。
func (a *FakeAgent) Run(ctx context.Context, _ agent.TurnRequest, sink agent.Sink, approve agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	a.mu.Lock()
	run := a.runs
	a.runs++
	a.mu.Unlock()
	select {
	case a.enteredCh <- struct{}{}:
	default:
	}

	// 记录并转发的 sink:帧先入观察面再交调用方(server),顺序与真链路一致。
	emit := func(ev *pb.TurnServerMessage) { a.emitWith(sink, run, ev) }
	start := time.Now() // 节奏参照(真实时钟;Factory.Now 只管观测打点)
	exitAt := start.Add(a.sc.ExitAfter)
	var exitAfter = a.sc.ExitAfter // 0 = 退出路不参与竞速

	// 终局闸:本 Run 出口必然关闭一次(迟到补发 goroutine 据此立即判拒退出)。
	done := make(chan struct{})
	var closeOnce sync.Once
	closeDone := func() { closeOnce.Do(func() { close(done) }) }
	defer closeDone()

	// 场景级审批:Run 一开始就发起(剧情编排见 ApprovalScenario 注释);
	// 挂起期间被取消则本轮立即按取消收场。
	if a.sc.Approval != nil {
		a.ask(ctx, a.sc.Approval, a.approvalCard(), approve)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, nil, ctxErr
		}
	}

	var (
		final      []pb.ContentBlock
		usage      *pb.Usage
		err        error
		terminated bool // 终局事件已处理(与"终稿是否为空"无关)
	)
	terminal := func(b []pb.ContentBlock, u *pb.Usage, e error) {
		final, usage, err, terminated = b, u, e, true
		closeDone()
	}

loop:
	for i, ev := range a.sc.Events {
		// 事件间默认间隔(首事件前不等待;显式节拍用 Pause)。
		if i > 0 && a.sc.Delay > 0 {
			if r := a.wait(ctx, a.sc.Delay, exitAt, exitAfter); r != stopContinue {
				return a.interrupted(ctx, r, usage)
			}
		}
		switch e := ev.(type) {
		case textEvent:
			emit(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: e.delta}}})
		case thinkingEvent:
			emit(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Delta: e.delta}}})
		case toolCallEvent:
			emit(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
				Phase: pb.ToolCallUpdate_STARTED, Call: e.card,
			}}})
		case toolResultEvent:
			emit(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
				Phase:  pb.ToolCallUpdate_FINISHED,
				Call:   a.startedCard(run, e.callID),
				Result: &pb.ToolResultBlock{CallId: e.callID, Ok: e.ok, Output: e.output},
			}}})
		case usageEvent:
			usage = e.usage
			emit(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Usage{Usage: &pb.UsageUpdate{Usage: e.usage}}})
		case codexEventFrame:
			emit(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: e.evt}})
		case unknownNotificationEvent:
			a.recordLedger(runtime.KindUnknownNotification, e.method, "", e.payload, "fake 脚本注入的未知通知")
		case unknownRequestEvent:
			a.consumeUnknownRequest(ctx, e, approve)
		case unknownResponseEvent:
			a.recordLedger(runtime.KindUnknownResponse, "response", e.correlationID, e.raw, "fake 脚本注入的迟到/对不上号响应")
		case pauseEvent:
			if r := a.wait(ctx, e.d, exitAt, exitAfter); r != stopContinue {
				return a.interrupted(ctx, r, usage)
			}
		case lateEvent:
			a.scheduleLate(ctx, done, emit, e)
		case completedEvent:
			terminal(e.blocks, usage, nil)
			break loop
		case failedEvent:
			terminal(nil, usage, e.err)
			break loop
		default:
			// 剧本含未知事件类型是测试代码错误,绝不能静默吞掉伪装成功。
			terminal(nil, usage, fmt.Errorf("fake agent %s: 剧本含未知事件类型 %T", a.id, ev))
			break loop
		}
	}
	// 剧本走完仍无终局事件:按失败收场(缺终局不该伪装成成功)。
	if !terminated {
		err = fmt.Errorf("fake agent %s: 剧本无终局事件(Completed/Failed)", a.id)
	}
	return final, usage, err
}

// interrupted 把可中断等待的结束原因翻译成 Run 的返回:ctx 取消 = 上层取消
// 语义;进程退出 = ExitError(EOF/exit status,含结构化 ErrorInfo)。
func (a *FakeAgent) interrupted(ctx context.Context, r stopReason, usage *pb.Usage) ([]pb.ContentBlock, *pb.Usage, error) {
	if r == stopCanceled {
		return nil, usage, ctx.Err()
	}
	return nil, usage, &ExitError{AgentID: a.id, Code: 1}
}

// wait 是回放中唯一的等待点:计时到点 / ctx 取消 二路竞速;ExitAfter>0 时把
// 计时时长向"距进程退出的剩余时间"取短,到点即按退出收场(轮中崩溃可打断
// 任意长度的静默)。exitAfter<=0 时退出路不参与。
func (a *FakeAgent) wait(ctx context.Context, d time.Duration, exitAt time.Time, exitAfter time.Duration) stopReason {
	if exitAfter > 0 {
		if remain := time.Until(exitAt); remain < d {
			d = remain
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		if exitAfter > 0 && !time.Now().Before(exitAt) {
			return stopExited
		}
		return stopContinue
	case <-ctx.Done():
		return stopCanceled
	}
}

// scheduleLate 把"迟到补发"挂到后台计时器:到点时轮次已终局/已取消 → 计入
// 迟到被拒(不发出);仍在进行 → 正常发出。goroutine 生命期被 done/ctx 有界
// 封顶(Run 出口必然闭闸),WaitIdle 可等它退出(无泄漏)。
func (a *FakeAgent) scheduleLate(ctx context.Context, done <-chan struct{}, emit func(*pb.TurnServerMessage), e lateEvent) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		t := time.NewTimer(e.after)
		defer t.Stop()
		select {
		case <-t.C:
		case <-done: // 本轮已终局:必然被拒,立即结算
			a.dropLate()
			return
		case <-ctx.Done(): // 上层已取消:同样拒收
			a.dropLate()
			return
		}
		select {
		case <-done:
			a.dropLate() // 迟到:补发被拒
		case <-ctx.Done():
			a.dropLate()
		default:
			emit(lateFrame(e.inner)) // 赶上了:正常补发
		}
	}()
}

// lateFrame 把迟到事件的载荷翻成帧(仅过程帧类;终局类迟到无意义,翻成一条
// 诊断帧让剧本错误可见)。
func lateFrame(ev AgentEvent) *pb.TurnServerMessage {
	switch e := ev.(type) {
	case textEvent:
		return &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: e.delta}}}
	case thinkingEvent:
		return &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Delta: e.delta}}}
	case usageEvent:
		return &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Usage{Usage: &pb.UsageUpdate{Usage: e.usage}}}
	default:
		return &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
			Code: "FAKE_LATE_UNSUPPORTED", Title: "迟到事件类型不可作帧", Message: fmt.Sprintf("%T", ev),
		}}}
	}
}

func (a *FakeAgent) dropLate() { a.mu.Lock(); a.lateDropped++; a.mu.Unlock() }

// ---- 审批 ----

// approvalCard 场景级审批的卡面。
func (a *FakeAgent) approvalCard() *pb.ToolCallBlock {
	sc := a.sc.Approval
	name := sc.CallName
	if name == "" {
		name = "shell"
	}
	args := sc.JSONArgs
	if args == "" {
		args = `{"command":"echo fake-approval"}`
	}
	return &pb.ToolCallBlock{Id: a.id + "-approval", Name: name, JsonArgs: args}
}

// ask 发起一次审批等待并记录观察。决定完全来自注入的 hook(红线见包注释):
// Budget>0 时给 ask 套短预算 ctx(超时路脚本);帧不在这里发 —— 审批卡的
// ApprovalRequest/ApprovalResolved 帧由 server 的 approve 钩子发出,与真链路
// 同构,fake 不重复发。
func (a *FakeAgent) ask(ctx context.Context, sc *ApprovalScenario, call *pb.ToolCallBlock, approve agent.ApprovalHook) {
	obs := &ApprovalObs{Call: call, RequestedAt: a.clock()}
	a.mu.Lock()
	a.approvals = append(a.approvals, obs)
	a.mu.Unlock()
	if approve == nil {
		obs.Err = errors.New("fake agent: 审批通道缺失(不伪装成用户决定)")
		return
	}
	if sc.DecideAfter > 0 {
		// 挂起等待期(可被取消打断;不参与进程退出竞速 —— 审批等待是合法静默)。
		t := time.NewTimer(sc.DecideAfter)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			obs.Err = ctx.Err()
			return
		}
	}
	askCtx := ctx
	if sc.Budget > 0 {
		var cancel context.CancelFunc
		askCtx, cancel = context.WithTimeout(ctx, sc.Budget)
		defer cancel()
	}
	allow, note, err := approve(askCtx, call)
	obs.Allow, obs.Note, obs.Err = allow, note, err
	obs.ResolvedAt = a.clock()
}

// consumeUnknownRequest 处理未知反向请求事件:发 unknown_request 通用确认卡
// (形状与 internal/agent 的 buildUnknownRequestCall 一致),按结局分流台账。
func (a *FakeAgent) consumeUnknownRequest(ctx context.Context, e unknownRequestEvent, approve agent.ApprovalHook) {
	sc := a.sc.Approval
	if sc == nil {
		sc = &ApprovalScenario{}
	}
	call := &pb.ToolCallBlock{
		Id:   a.id + "-unknown-request",
		Name: "unknown_request",
		JsonArgs: mustJSON(map[string]any{
			"reason":  "agent 请求执行一个未识别的操作",
			"method":  e.method,
			"payload": jsonPayload(e.params),
		}),
	}
	obs := &ApprovalObs{Call: call, RequestedAt: a.clock()}
	a.mu.Lock()
	a.approvals = append(a.approvals, obs)
	a.mu.Unlock()
	if approve == nil {
		obs.Err = errors.New("审批通道缺失")
		a.recordLedger(runtime.KindUnknownRequest, e.method, "", e.params, "审批通道缺失,按安全拒绝处理(未伪装用户意志)")
		return
	}
	askCtx := ctx
	if sc.Budget > 0 {
		var cancel context.CancelFunc
		askCtx, cancel = context.WithTimeout(ctx, sc.Budget)
		defer cancel()
	}
	allow, _, err := approve(askCtx, call)
	obs.Allow, obs.Err = allow, err
	obs.ResolvedAt = a.clock()
	switch {
	case err != nil:
		a.recordLedger(runtime.KindUnknownRequest, e.method, "", e.params, "确认超时/通道失败,按拒绝处理(失败安全方向)")
	case !allow:
		a.recordLedger(runtime.KindUnknownRequest, e.method, "", e.params, "用户拒绝未知请求(未替用户做任何审批决定)")
	default:
		a.recordLedger(runtime.KindKnownPassthrough, e.method, "", e.params, "用户已在手机查看脱敏原文并允许;已回空对象 {}(尽力应答,非业务结果)")
	}
}

// ---- 基础设施 ----

// emitWith 记录一帧(带轮次标号)并转发给调用方 sink。
func (a *FakeAgent) emitWith(sink agent.Sink, run int, ev *pb.TurnServerMessage) {
	a.mu.Lock()
	a.frames = append(a.frames, RecordedFrame{Run: run, Msg: ev})
	a.mu.Unlock()
	if sink != nil {
		sink(ev)
	}
}

// startedCard 在本轮已发出的 STARTED 帧里找同 callID 的卡(ToolResult 复用,
// 与 echo 的帧序同构);找不到退化为只带 Id 的最小卡。
func (a *FakeAgent) startedCard(run int, callID string) *pb.ToolCallBlock {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := len(a.frames) - 1; i >= 0; i-- {
		f := a.frames[i]
		if f.Run != run {
			continue
		}
		if tc := f.Msg.GetToolCall(); tc.GetPhase() == pb.ToolCallUpdate_STARTED && tc.GetCall().GetId() == callID {
			return tc.GetCall()
		}
	}
	return &pb.ToolCallBlock{Id: callID}
}

// ledger 台账目标:工厂注入优先,缺省与生产同一本账(见 Factory.Ledger 注释)。
func (a *FakeAgent) ledger() *runtime.Backend {
	if a.factory != nil && a.factory.Ledger != nil {
		return a.factory.Ledger
	}
	return agent.SharedRuntimeBackend()
}

// recordLedger 记一条未知事件(载荷裁剪与真实通道同宽:8KiB 上限同源)。
func (a *FakeAgent) recordLedger(kind, method, correlationID, raw, note string) {
	l := a.ledger()
	if l == nil {
		return
	}
	l.UnknownEvents().Record(runtime.EventEnvelope{
		SourceAgent:      a.id,
		SourceMethod:     method,
		CorrelationID:    correlationID,
		NormalizedKind:   kind,
		SafetyClass:      runtime.SafetyClassUnknown,
		RawPayload:       runtime.ClipRunesEllipsis(raw, 8192),
		ProcessingStatus: runtime.StatusNew,
		Note:             note,
	})
}

func (a *FakeAgent) clock() time.Time {
	if a.factory != nil && a.factory.Now != nil {
		return a.factory.Now()
	}
	return time.Now()
}

func (a *FakeAgent) turnID(run int) string { return fmt.Sprintf("fake-turn-%s-%d", a.id, run) }

// jsonPayload 尽力把参数原文解析成结构化 JSON(解析失败按字符串保留),
// 与 buildUnknownRequestCall 的"保持结构化原文"同语义。
func jsonPayload(raw string) any {
	var v any
	if raw != "" && json.Unmarshal([]byte(raw), &v) == nil {
		return v
	}
	return raw
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
