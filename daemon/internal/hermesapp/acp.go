package hermesapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
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

// SessionMCPServersSource 由上层(internal/agent 协同接线,agent/collab_inject.go)
// 注入:非 nil 且返回非空时,四个会话建立/重挂请求把条目带进 mcpServers。适配器
// 侧实证(acp_adapter/server.py:_register_session_mcp_servers):session/new·
// load·resume·fork 的 mcpServers 非空即 register_mcp_servers 注册进会话工具面
// —— 真注册,不是 dsh 式的"非空拒绝"。nil/空 = 原样发空数组,零行为变化。
var SessionMCPServersSource func() []map[string]any

// sessionMCPServersParam 组装 mcpServers 参数值:接线生效用注入条目,否则空数组。
func sessionMCPServersParam() []any {
	if SessionMCPServersSource != nil {
		if entries := SessionMCPServersSource(); len(entries) > 0 {
			out := make([]any, 0, len(entries))
			for _, e := range entries {
				out = append(out, e)
			}
			return out
		}
	}
	return []any{}
}

// SessionNewResult 建一个 ACP 会话的应答(sessionId + 模型/模式目录快照)。
type SessionNewResult struct {
	SessionID string `json:"sessionId"`
	sessionSettings
}

func (c *Client) SessionNew(ctx context.Context, cwd string) (*SessionNewResult, error) {
	raw, err := c.callCtx(ctx, "session/new", map[string]any{
		"cwd": cwd, "mcpServers": sessionMCPServersParam(),
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
		"cwd": cwd, "sessionId": sessionID, "mcpServers": sessionMCPServersParam(),
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
		"cwd": cwd, "sessionId": sessionID, "mcpServers": sessionMCPServersParam(),
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
		"cwd": cwd, "sessionId": sessionID, "mcpServers": sessionMCPServersParam(),
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
// 【变体对表(2026-10-02 server.py/events.py 静读 + 真机探针实测;2026-10-07
// 复核 acp_adapter 全量静读,证据 docs/reviews/2026-10-07/field-coverage-hermes.md)】
//
//	agent_message_chunk / agent_thought_chunk / tool_call / tool_call_update /
//	plan —— 主投影五件套(Provider 消费);
//	user_message_chunk(排队 prompt 回显 + load 历史回放)/ current_mode_update /
//	available_commands_update / usage_update(上下文占用→ provider 投影为
//	TurnServerMessage_Usage 发手机)/ session_info_update(标题/压缩轮转)
//	—— 已知协议成员,逐变体具名解析(不再笼统静默),无投影消费方的由 provider
//	落未知事件台账留痕(KNOWN_PASSTHROUGH 档,不再静默);
//	未知变体如实带 Raw 上抛(未知事件同一纪律)。Variant 记变体原名,供台账
//	登记名对表(dsh "session/update:<kind>" 同款后缀惯例)。
type StreamEvent struct {
	SessionID string
	// Variant 是 update.sessionUpdate 原文(未知变体也有;坏帧为空)。
	Variant string
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
	// Locations 是工具涉及的文件位置(ACP ToolCallLocation{path,line}:read_file/
	// write_file/patch 的 args.path 实测经 tools.py extract_locations 上报;line
	// 缺席为 0)。此前解析层不收、工具卡上无文件位置 —— 2026-10-07 补收。
	Locations []ToolLocation
	RawInput  json.RawMessage // rawInput 原样透传
}

// ToolLocation 是 ACP ToolCallLocation(path 必有;line 可缺席,0 = 未上报)。
type ToolLocation struct {
	Path string
	Line int64
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
	Locations   []struct {
		Path string `json:"path"`
		Line int64  `json:"line"`
	} `json:"locations"`
	Entries []struct {
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
	ev.Variant = u.SessionUpdate
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
		for _, loc := range u.Locations {
			if loc.Path != "" {
				tc.Locations = append(tc.Locations, ToolLocation{Path: loc.Path, Line: loc.Line})
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

// PromptOutcome 是 session/prompt 的终局应答(stopReason + 轮次用量)。
// 【usage 形状实证 server.py _finish_turn】Usage(input_tokens, output_tokens,
// total_tokens, thought_tokens=result.get("reasoning_tokens"),
// cached_read_tokens=…),且仅当三者任一非 None 时在场 —— 缺席 = 适配器没拿到
// 用量,不伪造。此前 Prompt 只回 stopReason,轮次用量整块静默丢(手机端轮次
// 用量/线程累计两处都吃不到),2026-10-07 补收。
type PromptOutcome struct {
	StopReason string     `json:"stopReason"`
	Usage      *TurnUsage `json:"usage,omitempty"`
}

// TurnUsage 是 prompt 应答的 usage 对象(server.py 实测形状;0 = 字段缺席)。
type TurnUsage struct {
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
	ThoughtTokens    int64 `json:"thought_tokens"`
	CachedReadTokens int64 `json:"cached_read_tokens"`
}

// Prompt 发一轮输入并阻塞到轮次终局。流式增量经 OnSessionUpdate 钩子实时回调
// (挂钩要在调用本方法前完成);返回值 = 终局应答(stopReason 取 end_turn /
// refusal / cancelled…,Usage 为轮次 token 用量,缺席为 nil)。
// 【长调用,无 120s 兜底】本方法阻塞到整轮结束(长流式轮次轻松超 2 分钟),走
// CallTurnCtx(两路 select,无 callTimeout)—— 兜底会把长轮次误杀成假死超时;
// 存活由 hermes 看门狗三闸(150s 静默 + 60min 上限 + 审批豁免)与 ctx 取消治理。
func (c *Client) Prompt(ctx context.Context, sessionID, text string) (*PromptOutcome, error) {
	raw, err := c.CallTurnCtx(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": text}},
	})
	if err != nil {
		return nil, err
	}
	out := &PromptOutcome{}
	if json.Unmarshal(raw, out) != nil {
		// 应答体解析失败:不伪造 stopReason,给空终局留痕(prompt 侧按空串走
		// "正文判据",与旧口径一致,只是多一条日志可查)。
		log.Printf("[hermes-acp] session/prompt 应答体无法解析(原样留日志): %s", truncateForLog(string(raw), 300))
		return out, nil
	}
	return out, nil
}

// ---- 轮次取消(session/cancel,ACP 通知)----
// 【形状实证 2026-09-30】venv acp 包 agent/router.py:112 把 "session/cancel"
// 路由为**通知**(无 id、无应答帧)到 server.py cancel:cancel_event.set() +
// request_hard_interrupt(连同该会话的后台委派一并打断)。

// CancelSession 发 session/cancel 通知,叫停一个正在跑的轮次。协议上没有受理
// 回执 —— 返回错误仅代表写入失败/超时,"是否真停了"以 prompt 侧的终局应答
// (stopReason=cancelled)或调用方自己的 ctx 返回为准。
// 【超时安全】stdin 写在 writeFrame 里有 5s 预算(writeWithBudget):适配器挂死
// 不读管道时,取消帧最迟 5s 返回错误;即便预算内没写完,后台写也可能仍在进行 ——
// 那时连接已无救,调用方会 Close 收尸;轮次收尾绝不能陪葬在取消帧上。
func (c *Client) CancelSession(ctx context.Context, sessionID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	done := make(chan error, 1)
	go func() {
		done <- c.writeFrame(map[string]any{
			"jsonrpc": "2.0", "method": "session/cancel",
			"params": map[string]any{"sessionId": sessionID},
		})
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
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
	// 直接走传输层 CallCtx(ctx.Done/ch 两路 select + 放弃时从 pending 除名)。
	// 曾经的 goroutine 包装(`go func(){ c.Call; done<-r }()` + ctx.Done 即弃)在
	// 放弃后会留下一个永挂 <-ch 的僵尸 goroutine,pending 表项要等 reap 才清 ——
	// 子进程僵死即无界累积(codexapp client.go goroutine 包装僵尸坑点名的同款)。
	return c.CallCtx(ctx, method, params)
}

// StopReasonIsError 桌面适配器把 provider 失败也包装成 end_turn + 错误正文
// (实测:DeepSeek 400 时 stopReason=end_turn,正文是 turn_failure_copy 文案),
// 判定"这轮真的成了"只能看正文内容。给调用方一个保守启发:refusal/cancelled
// 视为失败;end_turn 交由正文判定(正文由调用方累计)。
func StopReasonIsError(stop string) bool {
	return strings.EqualFold(stop, "refusal") || strings.EqualFold(stop, "cancelled")
}
