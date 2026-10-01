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
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
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

// ensureClient 懒启动:定位运行时 → 拉适配器 → ACP initialize(能力快照)。
func (p *HermesProvider) ensureClient(ctx context.Context) (hermesClient, error) {
	p.lifeMu.Lock()
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
	c, err := p.newClient()
	if err != nil {
		return nil, err
	}
	init, err := c.Initialize(ctx)
	if err != nil {
		c.Close()
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
	p.turnMu.Lock()
	defer p.turnMu.Unlock()
	p.mu.Lock()
	p.turnActive = true
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.turnActive = false
		p.mu.Unlock()
	}()

	c, err := p.ensureClient(ctx)
	if err != nil {
		return nil, nil, err
	}

	workspace := req.Workspace
	if workspace == "" {
		workspace, _ = os.Getwd()
	}

	// 会话就绪:同代内存缓存 → ExternalSession 重挂载(session/load + 历史回放)
	// → 新建。手机重开历史会话续聊 = daemon 重启/适配器重启后的重挂载路径。
	sid, err := p.ensureSession(ctx, c, req, workspace)
	if err != nil {
		return nil, nil, err
	}

	// 模型选择:thread.ModelID("hermes/deepseek-flash" → "deepseek:deepseek-flash");
	// 默认/未知不发 set(config.yaml model.default 兜底)。
	if m := threadModelID(req.Thread); m != "" && m != "default" {
		acpModel := strings.TrimPrefix(m, "hermes/")
		if !strings.Contains(acpModel, ":") {
			acpModel = "deepseek:" + acpModel
		}
		if serr := c.SetSessionModel(ctx, sid, acpModel); serr != nil {
			log.Printf("[hermes] 切模型失败(按默认跑): %v", serr)
		}
	}

	// 【思考强度真实化(2026-10-02)】hermes 的强度没有运行时 RPC,唯一入口是
	// 家目录 config.yaml 的 agent.reasoning_effort(会话创建那一刻读取)。用户在
	// 手机上显式选了强度 → 写该键(与手动编辑 config.yaml 等价,hermes 官方配置
	// 面;下一轮生效,本轮已按旧值创建)。空串 = 用户选"默认" = 删键回 hermes 默认。
	// 白名单校验(VALID_REASONING_EFFORTS)在写入前做,非法档宁可不写。
	if e := req.Thread.Effort; e != "" {
		if hermesapp.IsValidReasoningEffort(e) {
			if werr := hermesapp.SetReasoningEffort(e); werr != nil {
				log.Printf("[hermes] 写强度 %s 失败(沿用 config 现值): %v", e, werr)
			} else {
				log.Printf("[hermes] 思考强度已设为 %s(下轮会话生效)", e)
			}
		} else {
			log.Printf("[hermes] 强度 %s 不在 hermes 档位表(minimal..ultra),跳过写入", e)
		}
	}

	// 当轮钩子:通知(readLoop goroutine 同步回调,必须快进快出)+ 反向请求
	// (session/request_permission 审批桥)。别的会话的帧按 sessionId 分流丢弃。
	var final strings.Builder
	toolIDs := map[string]int{} // ACP toolCallId → 序号(帧 Id 稳定;同 id start/update 配对)
	var toolMu sync.Mutex
	var streamErr onceError
	notif := func(raw json.RawMessage) {
		if streamErr.get() != nil {
			return
		}
		ev := hermesapp.ParseSessionUpdate(raw)
		if ev.SessionID != sid {
			return // 非本轮会话的帧(共享适配器上还有别的会话)不进本轮
		}
		p.projectSessionUpdate(ev, sink, &final, toolIDs, &toolMu)
	}
	reqHandler := func(id int64, method string, params json.RawMessage) {
		p.handleRequestPermission(c, id, method, params, ctx, sink, approve)
	}
	c.SetTurnHandlers(notif, reqHandler)
	defer c.SetTurnHandlers(nil, nil)

	stop, err := c.Prompt(ctx, sid, text)
	if err != nil {
		if e := streamErr.get(); e != nil {
			return nil, nil, e
		}
		return nil, nil, fmt.Errorf("hermes prompt: %w", err)
	}
	if hermesapp.StopReasonIsError(stop) {
		return nil, nil, fmt.Errorf("hermes 轮次被拒/被取消(stopReason=%s)", stop)
	}

	out := strings.TrimSpace(final.String())
	if out == "" {
		return nil, nil, fmt.Errorf("hermes 轮次没有产出正文(stopReason=%s);常见原因:G:\\Hermes 未配置模型(config.yaml 的 model.default)或凭据无效", stop)
	}
	log.Printf("[hermes] 一轮完成 session=%s 用时 %v 回复 %d 字", sid,
		time.Since(turnStart).Round(time.Millisecond), len([]rune(out)))
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: out}}}},
		nil, nil
}

// projectSessionUpdate 把一条已解析的 session/update 投影到 sink(从 Run 的通知
// 钩子里调;只处理本轮会话的帧)。
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
	}
	// user_message_chunk(排队回显/历史回放)/ current_mode_update /
	// available_commands_update / usage_update / session_info_update:解析层已
	// 具名(daemon 暂无 sink 消费方),不透传、不误当未知。
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
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// onceError 只记第一个错误(投影路径错误不中断轮次)。
type onceError struct {
	mu sync.Mutex
	e  error
}

func (o *onceError) set(err error) {
	o.mu.Lock()
	if o.e == nil {
		o.e = err
	}
	o.mu.Unlock()
}

func (o *onceError) get() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.e
}
