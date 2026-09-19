// entries.go —— Pi 会话条目(get_entries)与断点续传游标(调研 §8.2)。
//
// 【为什么是它】Pi 会话文件是追加树,条目 id 是**跨客户端重启有效**的持久游标:
// get_entries(since=lastId) 只返回该条目之后的追加序条目(含压缩前历史与废弃
// 分支)。断线/子进程重挂载后凭它做权威增量,不必全量重拉 —— 0.85.1 探针实证:
// 全量→记录 lastId→新增条目→since 增量 1 条;错误游标 success:false(docs/verify/
// evidence/pi-p0-probe.md 第 8 项)。rpc.md「get_entries」「Message Types」。
//
// 【分层】本文件只做"传输 + 解析 + 形状合成",不 import pb:piapp 是传输层,
// 条目 → pb 帧的映射归 agent 包(复用 pi_events.go 的现有映射函数,单一映射真相)。
package piapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ErrCursorInvalid since 游标失效(条目不存在/会话已重建):调用方应转全量兜底。
// 用哨兵包裹(errors.Is 可判),原始错误保留在链上供日志定位。
var ErrCursorInvalid = errors.New("pi 断点续传游标失效")

// piEntriesBudget 单次 get_entries 的等待上界:大会话文件解析可能秒级,30s 对齐
// piCallBudget 的量级(entries.go 不依赖 agent 包的常量,这里独立声明)。
const piEntriesBudget = 30 * time.Second

// Entry get_entries 返回的会话条目(rpc.md:{"type":"message","id":"def456",
// "parentId":"abc123","timestamp":"...","message":{...}})。id 是 8 位短 id
// (piprobe 实证),即断点续传游标本体。
type Entry struct {
	Type      string          `json:"type"` // 目前实测均为 "message";其他类型原样保留
	ID        string          `json:"id"`   // 持久游标
	ParentID  string          `json:"parentId"`
	Timestamp string          `json:"timestamp"`
	Message   *EntryMessage   `json:"message"` // type=="message" 时携带(AgentMessage)
	Raw       json.RawMessage `json:"-"`       // 原始条目(诊断/回放摘要)
}

// EntryMessage AgentMessage(rpc.md「Message Types」,只解析用得到的字段)。
type EntryMessage struct {
	Role       string          `json:"role"`       // user / assistant / toolResult / bashExecution…
	Content    json.RawMessage `json:"content"`    // user 可能是纯字符串;assistant/toolResult 是块数组
	ToolCallID string          `json:"toolCallId"` // toolResult 携带
	ToolName   string          `json:"toolName"`   // toolResult 携带
	IsError    bool            `json:"isError"`    // toolResult 携带
}

// EntryContentBlock content 块(text/thinking/toolCall/image…,只取用得到的字段)。
type EntryContentBlock struct {
	Type      string          `json:"type"` // "text" / "thinking" / "toolCall" / "image"
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"` // toolCall
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ContentBlocks 把 content 归一成块数组:字符串 content(user 简写)视为单个
// text 块;数组逐块解析;解析失败返回 nil(宁可少回放,不可错回放)。
func (m *EntryMessage) ContentBlocks() []EntryContentBlock {
	if m == nil || len(m.Content) == 0 {
		return nil
	}
	var s string
	if json.Unmarshal(m.Content, &s) == nil {
		return []EntryContentBlock{{Type: "text", Text: s}}
	}
	var blocks []EntryContentBlock
	if json.Unmarshal(m.Content, &blocks) != nil {
		return nil
	}
	return blocks
}

// EntriesPage get_entries 应答:data = {"entries":[…],"leafId":"…"}。
type EntriesPage struct {
	Entries []Entry
	LeafID  string // 当前活动分支的叶条目 id(空会话为空;废弃分支不计入)
}

// LastID 页内最后一条条目的 id —— "最近一次从 Pi 拿到的条目 id"(游标推进用)。
// get_entries 是追加序(含废弃分支),字面取最后一条即"见过的最大位置";
// 空页返回空串(调用方保持原游标不动)。
func (p *EntriesPage) LastID() string {
	if p == nil || len(p.Entries) == 0 {
		return ""
	}
	return p.Entries[len(p.Entries)-1].ID
}

// parseEntriesPage 解析 get_entries 应答(纯函数,单测钉形状)。
func parseEntriesPage(raw json.RawMessage) (*EntriesPage, error) {
	var data struct {
		Entries []Entry `json:"entries"`
		LeafID  string  `json:"leafId"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("get_entries 应答无法解析: %w", err)
	}
	out := &EntriesPage{LeafID: data.LeafID}
	for i := range data.Entries {
		e := &data.Entries[i]
		if e.ID == "" {
			continue // 无 id 的条目当不了游标,跳过(诊断可看 Raw 的路径留待需要时加)
		}
		out.Entries = append(out.Entries, *e)
	}
	return out, nil
}

// GetEntries 拉会话条目 —— 断点续传的权威数据源(调研 §8.2)。
//
//   - sinceEntryID == "":全量(应答带 leafId,可作全量兜底的重锚点);
//   - sinceEntryID != "":严格增量(仅该条目之后的追加序条目,探针实证);
//   - 游标不存在等失败:返回 error;疑似游标失效时以 ErrCursorInvalid 包装,
//     调用方据此转全量兜底(任何 since 调用失败本身就该兜底,哨兵只为日志分流)。
//
// sessionFile 非空时校验子进程当前挂载会话,防"给错误会话续传"(重挂载竞态兜底)。
func (c *Client) GetEntries(ctx context.Context, sessionFile, sinceEntryID string) (*EntriesPage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx2, cancel := context.WithTimeout(ctx, piEntriesBudget)
	defer cancel()
	params := map[string]any{}
	if sinceEntryID != "" {
		if err := c.checkMountedSession(sessionFile); err != nil {
			return nil, err
		}
		params["since"] = sinceEntryID
	}
	raw, err := c.CallCtx(ctx2, "get_entries", params, piEntriesBudget)
	if err != nil {
		if sinceEntryID != "" && isCursorError(err) {
			return nil, fmt.Errorf("%w: %v", ErrCursorInvalid, err)
		}
		return nil, err
	}
	return parseEntriesPage(raw)
}

// checkMountedSession 校验子进程当前挂载的就是目标会话(会话缓存由
// GetState/SwitchSession 经原子快照维护;缓存未知时跳过 —— 校验是兜底不是门禁)。
func (c *Client) checkMountedSession(sessionFile string) error {
	mounted := c.SessionFile()
	if sessionFile == "" || mounted == "" {
		return nil
	}
	if !strings.EqualFold(filepath.Clean(mounted), filepath.Clean(sessionFile)) {
		return fmt.Errorf("会话不符: 子进程挂载 %s,目标 %s", filepath.Base(mounted), filepath.Base(sessionFile))
	}
	return nil
}

// isCursorError 疑似"since 不匹配任何条目"(rpc.md:此时 success:false)。
// Pi 的错误文案未在文档定死,按常见措辞宽匹配;匹配不上也只会走"普适兜底",
// 不影响正确性。
func isCursorError(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "not found") ||
		strings.Contains(s, "no such entry") ||
		strings.Contains(s, "unknown entry") ||
		strings.Contains(s, "invalid")
}

// WireEvents 把条目回放成 pi **线上事件形状**(message_update/tool_execution_end)。
// 目的:断线期间的会话条目能走与实时流**同一条映射管线**(agent 包的
// MapTextDelta/MapThinkingDelta/MapToolcallEnd/MapToolExecEnd)—— 回放与实时
// 同构,映射规则只有一份。user 条目在实时流里本就没有对应事件,跳过。
func (e *Entry) WireEvents() []Event {
	if e == nil || e.Message == nil {
		return nil
	}
	m := e.Message
	switch m.Role {
	case "assistant":
		var out []Event
		for _, b := range m.ContentBlocks() {
			var raw json.RawMessage
			switch b.Type {
			case "text":
				raw = mustJSON(map[string]any{"type": "message_update", "assistantMessageEvent": map[string]any{
					"type": "text_delta", "delta": b.Text}})
			case "thinking":
				raw = mustJSON(map[string]any{"type": "message_update", "assistantMessageEvent": map[string]any{
					"type": "thinking_delta", "delta": b.Thinking}})
			case "toolCall":
				args := b.Arguments
				if len(args) == 0 {
					args = json.RawMessage("{}")
				}
				raw = mustJSON(map[string]any{"type": "message_update", "assistantMessageEvent": map[string]any{
					"type": "toolcall_end", "id": b.ID, "toolName": b.Name,
					"toolCall": map[string]any{"id": b.ID, "name": b.Name, "arguments": args}}})
			default:
				continue // image 等块无流式事件对应面
			}
			out = append(out, Event{Type: rawTypeOf(raw), Raw: raw})
		}
		return out
	case "toolResult":
		// 工具结果 → tool_execution_end 形状(result.content 取全部 text 拼接,
		// 与实时帧同构;isError 直译)。
		texts := make([]map[string]any, 0, 4)
		for _, b := range m.ContentBlocks() {
			if b.Type == "text" {
				texts = append(texts, map[string]any{"type": "text", "text": b.Text})
			}
		}
		raw := mustJSON(map[string]any{
			"type": "tool_execution_end", "toolCallId": m.ToolCallID, "toolName": m.ToolName,
			"result": map[string]any{"content": texts}, "isError": m.IsError})
		return []Event{{Type: rawTypeOf(raw), Raw: raw}}
	default:
		return nil // user / bashExecution 等没有流式事件对应面
	}
}

// rawTypeOf 从合成 JSON 里回读 type 字段(合成即自证,Event.Type 不许与 Raw 脱节)。
func rawTypeOf(raw json.RawMessage) string {
	var m struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	return m.Type
}

// mustJSON 测试与合成路径的兜底序列化:map[string]any 不可能失败,失败即 bug。
func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}
