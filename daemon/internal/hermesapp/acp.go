package hermesapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// InitializeResult 是 initialize 应答(agentCapabilities 按 0.9.0 实测形状:
// loadSession 布尔在场,sessionCapabilities 的 fork/list/resume 以对象在场即能力)。
type InitializeResult struct {
	ProtocolVersion int `json:"protocolVersion"`
	AgentInfo       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"agentInfo"`
	AgentCapabilities *AgentCapabilities `json:"agentCapabilities,omitempty"`
	AuthMethods       []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"authMethods"`
}

// AgentCapabilities 是 ACP 0.9.0 的能力面(实证 2026-10-02,server.py initialize):
// loadSession=true + sessionCapabilities{fork:{},list:{},resume:{}}。指针在场即能力,
// 缺席即不支持(与 dshapp 能力门控同纪律:一切扩展方法的使用以 initialize 应答为准)。
type AgentCapabilities struct {
	LoadSession         bool                         `json:"loadSession"`
	PromptCapabilities  *PromptCapabilitiesSnapshot  `json:"promptCapabilities,omitempty"`
	SessionCapabilities *SessionCapabilitiesSnapshot `json:"sessionCapabilities,omitempty"`
}

// PromptCapabilitiesSnapshot 目前只关心 image(实测 image=true)。
type PromptCapabilitiesSnapshot struct {
	Image bool `json:"image"`
}

// SessionCapabilitiesSnapshot 的 fork/list/resume 是空对象,指针在场即支持。
type SessionCapabilitiesSnapshot struct {
	Fork   *struct{} `json:"fork,omitempty"`
	List   *struct{} `json:"list,omitempty"`
	Resume *struct{} `json:"resume,omitempty"`
}

// CanLoadSession / CanFork / CanList / CanResume 能力谓词(重挂载/列举前的门控)。
func (a *AgentCapabilities) CanLoadSession() bool { return a != nil && a.LoadSession }
func (a *AgentCapabilities) CanFork() bool {
	return a != nil && a.SessionCapabilities != nil && a.SessionCapabilities.Fork != nil
}
func (a *AgentCapabilities) CanList() bool {
	return a != nil && a.SessionCapabilities != nil && a.SessionCapabilities.List != nil
}
func (a *AgentCapabilities) CanResume() bool {
	return a != nil && a.SessionCapabilities != nil && a.SessionCapabilities.Resume != nil
}

func (c *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	raw, err := c.callCtx(ctx, "initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs": map[string]any{"readTextFile": false, "writeTextFile": false},
		},
	})
	if err != nil {
		return nil, err
	}
	out := &InitializeResult{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("hermes initialize 应答解析: %w", err)
	}
	return out, nil
}

// SessionNewResult 建一个 ACP 会话的应答(sessionId + 模型/模式目录快照)。
type SessionNewResult struct {
	SessionID string `json:"sessionId"`
	sessionSettings
}

func (c *Client) SessionNew(ctx context.Context, cwd string) (*SessionNewResult, error) {
	raw, err := c.callCtx(ctx, "session/new", map[string]any{
		"cwd": cwd, "mcpServers": []any{},
	})
	if err != nil {
		return nil, err
	}
	out := &SessionNewResult{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("hermes session/new 应答解析: %w", err)
	}
	if out.SessionID == "" {
		return nil, fmt.Errorf("hermes session/new 没回 sessionId")
	}
	return out, nil
}

// SetSessionModel 切会话模型(modelId 形如 "deepseek:deepseek-flash")。
func (c *Client) SetSessionModel(ctx context.Context, sessionID, modelID string) error {
	_, err := c.callCtx(ctx, "session/set_model", map[string]any{
		"sessionId": sessionID, "modelId": modelID,
	})
	return err
}

// sessionSettings 是 load/resume/new 应答共有的 models/modes 面(ACP 0.9.0:
// SessionModelState / SessionModeState)。
type sessionSettings struct {
	Models struct {
		CurrentModelID  string `json:"currentModelId"`
		AvailableModels []struct {
			ModelID     string `json:"modelId"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"availableModels"`
	} `json:"models"`
	Modes struct {
		CurrentModeID  string `json:"currentModeId"`
		AvailableModes []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"availableModes"`
	} `json:"modes"`
}

// SessionLoadResult 是 session/load | session/resume 的应答面。
// 【关键口径(实证 server.py,ACP 规范同)】应答**不带 sessionId**(会话沿用
// 请求的 id);且 load/resume 必须在应答前把持久历史以 session/update 通知流出
// (user/assistant/thought chunk + tool_call start/complete 对)—— 客户端要在
// 调用前装好通知钩子承接回放,否则回放丢失(规范:客户端在 load RPC 生存期内
// 同步度量通知)。
type SessionLoadResult struct {
	sessionSettings
}

// ErrSessionNotFound session/load 的应答是 null(适配器 update_cwd/get 查无此
// 会话,server.py load_session 返回 None)。调用方据此显式报错,不静默新开。
var ErrSessionNotFound = fmt.Errorf("hermes 会话不存在(session/load 应答 null)")

// LoadSession 重挂载一个已持久化的会话(手机重开历史会话→hermes resume 的
// 数据面)。cwd 传会话工作区;历史回放经通知钩子流出(**调用前装好钩子**)。
// 查无此会话返回 (nil, ErrSessionNotFound)。
func (c *Client) LoadSession(ctx context.Context, cwd, sessionID string) (*SessionLoadResult, error) {
	raw, err := c.callCtx(ctx, "session/load", map[string]any{
		"cwd": cwd, "sessionId": sessionID, "mcpServers": []any{},
	})
	if err != nil {
		return nil, err
	}
	if isNullJSON(raw) {
		return nil, ErrSessionNotFound
	}
	out := &SessionLoadResult{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("hermes session/load 应答解析: %w", err)
	}
	return out, nil
}

// ResumeSession session/resume(协议面收编:与 load 同形、同样先流出历史回放;
// 差异:查无此会话时适配器**自动新建**,与 daemon"不静默新开"纪律相悖,故
// daemon 重挂载走 LoadSession,此方法供协议完整性/探针对表)。
func (c *Client) ResumeSession(ctx context.Context, cwd, sessionID string) (*SessionLoadResult, error) {
	raw, err := c.callCtx(ctx, "session/resume", map[string]any{
		"cwd": cwd, "sessionId": sessionID, "mcpServers": []any{},
	})
	if err != nil {
		return nil, err
	}
	if isNullJSON(raw) {
		return nil, ErrSessionNotFound
	}
	out := &SessionLoadResult{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("hermes session/resume 应答解析: %w", err)
	}
	return out, nil
}

// ForkSessionResult 是 session/fork 的应答(sessionId = 分叉出的新会话;适配器
// 查无源会话时回 sessionId="" —— 空串按失败处理,不假装分叉成功)。
type ForkSessionResult struct {
	SessionID string `json:"sessionId"`
	sessionSettings
}

// ForkSession 分叉一个已有会话(fork/list/resume 能力实证在 initialize 应答)。
func (c *Client) ForkSession(ctx context.Context, cwd, sessionID string) (*ForkSessionResult, error) {
	raw, err := c.callCtx(ctx, "session/fork", map[string]any{
		"cwd": cwd, "sessionId": sessionID, "mcpServers": []any{},
	})
	if err != nil {
		return nil, err
	}
	out := &ForkSessionResult{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("hermes session/fork 应答解析: %w", err)
	}
	if out.SessionID == "" {
		return nil, fmt.Errorf("hermes session/fork 没回新 sessionId(源会话 %s 可能不存在)", sessionID)
	}
	return out, nil
}

// SessionListEntry 是 session/list 的一条(SessionInfo;title/updatedAt 可缺席)。
type SessionListEntry struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	Title     string `json:"title,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// SessionListResult 是 session/list 应答(游标分页:nextCursor 非空即还有页,
// 下一页传它作 cursor;未知 cursor → 空页,适配器绝不回全量)。
type SessionListResult struct {
	Sessions   []SessionListEntry `json:"sessions"`
	NextCursor string             `json:"nextCursor,omitempty"`
}

// ListSessions 列出适配器持久库的会话(内存 + DB,cwd 过滤可选)。cursor 为
// 上一页的 nextCursor。
func (c *Client) ListSessions(ctx context.Context, cursor string) (*SessionListResult, error) {
	params := map[string]any{}
	if cursor != "" {
		params["cursor"] = cursor
	}
	raw, err := c.callCtx(ctx, "session/list", params)
	if err != nil {
		return nil, err
	}
	out := &SessionListResult{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("hermes session/list 应答解析: %w", err)
	}
	return out, nil
}

// isNullJSON 判应答体是否为字面 null(适配器查无此会话时 load_session 返回
// None → JSON-RPC result=null)。
func isNullJSON(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}

// StreamEvent 一条 session/update 投影(已解析的最小面,供 Provider 映射)。
// 【变体对表(2026-10-02 server.py/events.py 静读 + 真机探针实测)】
//
//	agent_message_chunk / agent_thought_chunk / tool_call / tool_call_update /
//	plan —— 主投影五件套(Provider 消费);
//	user_message_chunk(排队 prompt 回显 + load 历史回放)/ current_mode_update /
//	available_commands_update / usage_update(上下文占用)/ session_info_update
//	(标题/压缩轮转)—— 已知协议成员,逐变体具名解析(不再笼统静默),daemon
//	暂无消费方的留字段待接;未知变体如实带 Raw 上抛(未知事件同一纪律)。
type StreamEvent struct {
	SessionID string
	// 以下按 Update 分类:
	MessageChunk string // agent_message_chunk 正文增量
	ThoughtChunk string // agent_thought_chunk 思考增量
	ToolCall     *ToolCallEvent
	Plan         []PlanEntry
	UserChunk    string            // user_message_chunk(回放/排队回显)
	CurrentMode  string            // current_mode_update
	Usage        *UsageSnapshot    // usage_update(context 占用)
	SessionInfo  *SessionInfoPatch // session_info_update(标题/出处)
	CommandsSeen int               // available_commands_update 条数(0=无此变体)
	Raw          json.RawMessage
}

// UsageSnapshot 是 usage_update(ACP UsageUpdate:size=上下文窗,used=估算占用)。
type UsageSnapshot struct {
	Size int64
	Used int64
}

// SessionInfoPatch 是 session_info_update(title 轮转/自动起名;updatedAt ISO8601)。
type SessionInfoPatch struct {
	Title     string
	UpdatedAt string
}

type ToolCallEvent struct {
	Kind       string // tool_call(新开)/ tool_call_update(进展/收口)
	ToolCallID string
	Title      string
	Status     string // pending / in_progress / completed / failed
	ToolKind   string // ACP ToolKind:read/edit/execute/fetch/think/other…
	Output     string // content 里 text 块拼接
	// Diff 是 content 里 type=diff 的编辑提案(write_file/patch 实测:
	// FileEditToolCallContent{path, oldText, newText}),nil = 无。
	Diff     *ToolDiff
	RawInput json.RawMessage // rawInput 原样透传
}

// ToolDiff 是编辑类工具调用的 diff 内容块。
type ToolDiff struct {
	Path    string
	OldText string
	NewText string
}

type PlanEntry struct {
	Content string
	Status  string
}

// updateEnvelope / updateBody 对 session/update 外层与 update 内层的解析结构。
type updateEnvelope struct {
	SessionID string     `json:"sessionId"`
	Update    updateBody `json:"update"`
}

type updateBody struct {
	SessionUpdate string `json:"sessionUpdate"`
	// ContentJSON 是 update.content 原样:文本类变体是单个 ContentBlock 对象,
	// 工具类变体是 ContentBlock 数组 —— 按变体二解(json 标签不可在同级重复)。
	ContentJSON json.RawMessage `json:"content"`
	Title       string          `json:"title"` // tool_call 标题 / session_info 标题
	Status      string          `json:"status"`
	ToolKind    string          `json:"kind"`
	ToolCallID  string          `json:"toolCallId"`
	RawInput    json.RawMessage `json:"rawInput"`
	Entries     []struct {
		Content string `json:"content"`
		Status  string `json:"status"`
	} `json:"entries"`
	CurrentModeID string `json:"currentModeId"`
	// usage_update 的 size/used 是 update 顶层字段(ACP UsageUpdate 实测形状)。
	Size          int64             `json:"size"`
	Used          int64             `json:"used"`
	Commands      []json.RawMessage `json:"availableCommands"`
	InfoUpdatedAt string            `json:"updatedAt"`
}

// textContentBlock 是文本类变体的 content 形状。
type textContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// toolContentBlock 按 type 分派(helpers.py 实测形状):
//   - text 块经 ContentToolCallContent 包一层:{type:"content",content:{type:"text",text}}
//     —— text 在嵌套的 content 里;
//   - diff 块是扁平的 {type:"diff",path,oldText,newText}。
type toolContentBlock struct {
	Type    string            `json:"type"`
	Content *textContentBlock `json:"content"`
	Path    string            `json:"path"`
	OldText string            `json:"oldText"`
	NewText string            `json:"newText"`
}

// ParseSessionUpdate 把一条 session/update params 解析成 StreamEvent。
// 已知变体逐一对表(见 StreamEvent 注);未知 sessionUpdate 变体如实带 Raw
// 上抛(不静默吞,与未知事件同一纪律)。
func ParseSessionUpdate(params json.RawMessage) StreamEvent {
	var p updateEnvelope
	ev := StreamEvent{Raw: params}
	if json.Unmarshal(params, &p) != nil {
		return ev
	}
	ev.SessionID = p.SessionID
	u := p.Update
	// content 二解:文本类变体按单对象,工具类变体按数组(解析失败 = 空处理)。
	var textBlk textContentBlock
	var toolBlks []toolContentBlock
	if len(u.ContentJSON) > 0 {
		if u.ContentJSON[0] == '[' {
			_ = json.Unmarshal(u.ContentJSON, &toolBlks)
		} else {
			_ = json.Unmarshal(u.ContentJSON, &textBlk)
		}
	}
	switch u.SessionUpdate {
	case "agent_message_chunk":
		ev.MessageChunk = textBlk.Text
	case "agent_thought_chunk":
		ev.ThoughtChunk = textBlk.Text
	case "user_message_chunk":
		ev.UserChunk = textBlk.Text
	case "tool_call", "tool_call_update":
		tc := &ToolCallEvent{
			Kind:       u.SessionUpdate,
			ToolCallID: u.ToolCallID,
			Title:      u.Title,
			Status:     u.Status,
			ToolKind:   u.ToolKind,
			RawInput:   u.RawInput,
		}
		for _, c := range toolBlks {
			switch c.Type {
			case "diff":
				if tc.Diff == nil {
					tc.Diff = &ToolDiff{Path: c.Path, OldText: c.OldText, NewText: c.NewText}
				}
			default: // content(text;及 terminal:daemon 不消费 terminal_id)
				if c.Content != nil {
					tc.Output += c.Content.Text
				}
			}
		}
		// rawInput 字面 null 归一为空(缺席语义,不应答与真实输入区分开)。
		if string(tc.RawInput) == "null" {
			tc.RawInput = nil
		}
		ev.ToolCall = tc
	case "plan":
		for _, e := range u.Entries {
			ev.Plan = append(ev.Plan, PlanEntry{Content: e.Content, Status: e.Status})
		}
	case "current_mode_update":
		ev.CurrentMode = u.CurrentModeID
	case "usage_update":
		if u.Size > 0 || u.Used > 0 {
			ev.Usage = &UsageSnapshot{Size: u.Size, Used: u.Used}
		}
	case "available_commands_update":
		ev.CommandsSeen = len(u.Commands)
	case "session_info_update":
		ev.SessionInfo = &SessionInfoPatch{Title: u.Title, UpdatedAt: u.InfoUpdatedAt}
	default:
		// 未知变体:留 Raw 上抛,不算崩溃(daemon 无消费方时静默留痕)。
	}
	return ev
}

// Prompt 发一轮输入并阻塞到轮次终局。流式增量经 OnSessionUpdate 钩子实时回调
// (挂钩要在调用本方法前完成);返回值 = stopReason(end_turn / refusal / cancelled…)。
func (c *Client) Prompt(ctx context.Context, sessionID, text string) (string, error) {
	raw, err := c.callCtx(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": text}},
	})
	if err != nil {
		return "", err
	}
	var out struct {
		StopReason string `json:"stopReason"`
	}
	if json.Unmarshal(raw, &out) != nil {
		return "", nil
	}
	return out.StopReason, nil
}

// ---- 反向请求(session/request_permission,审批桥的数据面)----
// 【形状实证 2026-10-02 permissions.py/edit_approval.py + 真机帧】server 发
// params={sessionId, toolCall:{toolCallId,title,kind,status,content,rawInput},
// options:[{optionId,kind,name,…}]};应答 result={"outcome":{"outcome":
// "selected","optionId":…}}。optionId 语义稳定:allow_once/allow_session/
// allow_always(危险命令审批)+ deny/deny_always;编辑审批只有
// allow_once/deny 两个选项(edit_approval.py make_acp_edit_approval_requester)。

// RequestPermissionParams 是 session/request_permission 的请求参数。
type RequestPermissionParams struct {
	SessionID string              `json:"sessionId"`
	ToolCall  *PermissionToolCall `json:"toolCall,omitempty"`
	Options   []PermissionOption  `json:"options"`
	Raw       json.RawMessage     `json:"-"`
}

// PermissionToolCall 是请求附带的工具上下文(危险命令审批是 perm-check-N 的
// 合成 bubble;编辑审批是 edit-approval-N 带 diff 内容)。
type PermissionToolCall struct {
	ToolCallID string          `json:"toolCallId"`
	Title      string          `json:"title"`
	Kind       string          `json:"kind"` // execute(命令)/ edit(文件编辑)
	Status     string          `json:"status"`
	RawInput   json.RawMessage `json:"rawInput"`
}

// PermissionOption 一个授权选项(kind 前缀 allow/reject 决定归属,不猜名字)。
type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // allow_once/allow_always/reject_once/reject_always
}

// ParseRequestPermission 从反向请求 params 解析(容错:坏参返回错误,由上层
// RespondError,绝不伪造 decision)。
func ParseRequestPermission(params json.RawMessage) (*RequestPermissionParams, error) {
	p := &RequestPermissionParams{Raw: params}
	if err := json.Unmarshal(params, p); err != nil {
		return nil, fmt.Errorf("session/request_permission 参数无法解析: %w", err)
	}
	if len(p.Options) == 0 {
		return nil, fmt.Errorf("session/request_permission 缺 options 数组")
	}
	return p, nil
}

// AllowOptionID 按 kind 前缀挑第一个 allow 侧选项;没有返回空串(上层显式报错)。
func (p *RequestPermissionParams) AllowOptionID() string {
	for _, o := range p.Options {
		if strings.HasPrefix(strings.ToLower(o.Kind), "allow") {
			return o.OptionID
		}
	}
	return ""
}

// RejectOptionID 按 kind 前缀挑第一个 reject 侧选项;没有显式 reject kind 时按
// ACP 惯例兜底最后一个非 allow 选项(拒绝是失败安全方向,与 dshapp 同口径)。
func (p *RequestPermissionParams) RejectOptionID() string {
	for _, o := range p.Options {
		if strings.HasPrefix(strings.ToLower(o.Kind), "reject") {
			return o.OptionID
		}
	}
	last := p.Options[len(p.Options)-1]
	if strings.HasPrefix(strings.ToLower(last.Kind), "allow") {
		return ""
	}
	return last.OptionID
}

// PermissionOutcomeSelected 构造应答 result:用户做出了选择。
// 形状(ACP 0.9.0 + permissions.py AllowedOutcome):{"outcome":{"outcome":
// "selected","optionId":…}}。
func PermissionOutcomeSelected(optionID string) map[string]any {
	return map[string]any{
		"outcome": map[string]any{"outcome": "selected", "optionId": optionID},
	}
}

func (c *Client) callCtx(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background() // nil ctx 合法(无超时语义;旧路径实测如此调用)
	}
	type result struct {
		raw json.RawMessage
		err error
	}
	done := make(chan result, 1)
	go func() {
		raw, err := c.Call(method, params)
		done <- result{raw, err}
	}()
	select {
	case r := <-done:
		return r.raw, r.err
	case <-ctx.Done():
		// 放弃:迟到应答经 readLoop 静默丢(pending 已随 reap/清理);调用方按
		// 超时降级。适配器进程由调用方 Close 收尸。
		return nil, ctx.Err()
	}
}

// StopReasonIsError 桌面适配器把 provider 失败也包装成 end_turn + 错误正文
// (实测:DeepSeek 400 时 stopReason=end_turn,正文是 turn_failure_copy 文案),
// 判定"这轮真的成了"只能看正文内容。给调用方一个保守启发:refusal/cancelled
// 视为失败;end_turn 交由正文判定(正文由调用方累计)。
func StopReasonIsError(stop string) bool {
	return strings.EqualFold(stop, "refusal") || strings.EqualFold(stop, "cancelled")
}

var _ = time.Second // 预留(超时预算由调用方定)
