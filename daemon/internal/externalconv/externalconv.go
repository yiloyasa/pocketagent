// Package externalconv 只读解析 PA 尚未接入的桌面 agent 留在本机的既有会话
// (首批:Claude Code 的 ~/.claude/projects/**/*.jsonl)。
//
// 【红线】本包对用户的 agent 目录**只读**:全部文件访问走 os.ReadFile /
// os.ReadDir / filepath.WalkDir,没有任何写入路径。绝不碰用户配置,绝不迁移、
// 改写或删除外部会话文件。
//
// 【为什么独立成包】daemon 已接入的四家(pi/zcode/codex/dsh)各有进程内协议;
// 未接入 agent 的会话是纯文件,解析器不依赖 provider 与网络,独立包 + 单测
// (合成夹具)即可验证。真正接到手机走 server 层既有的
// AgentSessionImporter / SessionHistoryReader 能力接口(server/external_sessions.go),
// proto 与既有生产文件零改动。
//
// 【证据边界(2026-09-30 本机实测)】本机三家均**无任何已持久化会话**:
//   - Claude Code:%HOME%\.claude 存在但无 projects/ 目录;
//   - OpenCode:%HOME%\.local\share\opencode 只有空的 log/ 与 repos/,
//     无 storage/(无任何会话落盘);
//   - Hermes:全盘常见位置均未发现任何 agent 会话存储
//     (.hermes-apple-design-research 只是技能调研工作区,非 agent 存储)。
//
// 因此本包按 Claude Code 公开且长期稳定的 JSONL 行格式实现,单测全部使用
// **合成夹具**(t.TempDir 自造样例),本机真实数据零依赖 —— 有真实文件时,
// 解析失败的行如实计入 SkippedLines,绝不伪造内容。
package externalconv

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Agent 是外部 agent 家族标识(与 server 层导入能力的 agentId 同名)。
type Agent string

// AgentClaude 是 Claude Code 桌面 CLI。
const AgentClaude Agent = "claude"

// ClaudeRoot 返回 Claude Code 会话存储根:<home>/.claude/projects。
// 会话文件按项目目录转义名分目录存放:<root>/<escaped-cwd>/<session-uuid>.jsonl。
func ClaudeRoot(home string) string {
	return filepath.Join(home, ".claude", "projects")
}

// SessionMeta 是一条外部会话的列表项(不含消息正文)。
type SessionMeta struct {
	Agent     Agent     // 所属 agent 家族
	ID        string    // 会话 id(取会话文件名去扩展名;与文件内 sessionId 一致)
	Title     string    // 可得则填;拿不到留空 —— 不猜
	File      string    // 会话文件绝对路径(导入与解析的唯一句柄)
	Modified  time.Time // 文件修改时间(列表排序用)
	SizeBytes int64
}

// Message 是从会话文件里读出的一条消息(只抽正文,不抽工具块)。
type Message struct {
	Role string    // "user" / "assistant"(按文件内 message.role 原样;拿不到按行 type)
	Text string    // 文本块的拼接(详见 claudeLine 的抽取规则)
	At   time.Time // 行级 timestamp;缺失为零值
}

// Session 是一条外部会话的完整只读视图。
type Session struct {
	SessionMeta
	// Messages 是按文件顺序排好的消息。空文件/无消息行 → 空切片,不是错误。
	Messages []Message
	// SkippedLines 是解析中**如实跳过**的行数:非消息行(summary/system 等
	// 元行不计入消息,空行、损坏行、sidechain/子代理行均计入本计数)。
	// "绝不伪造":跳过的行只计数,不补内容。
	SkippedLines int
}

// ScanClaude 列出 <root> 下的 Claude Code 会话(只读目录走查,按修改时间倒序)。
//
// root 不存在 = 电脑上没有 Claude Code 会话存储:返回空列表 + nil,调用方
// (导入页)据此显示"无会话"而不是报错 —— "没装"与"空"在这个能力上无区分
// 必要(与 pi 的纯文件扫描同口径)。走查中单个不可读文件跳过不挡整体。
//
// 【标题是 best-effort】列表页需要标题,但 summary 行可能出现在文件任意位置;
// 为列表全量读每个文件太重。这里只读每个文件**开头 8KB**找第一条 summary 行:
// 找到就如实采用,找不到 Title 留空(不猜、不造)。导入时的全量解析
// (ReadClaudeSession)仍会拿到完整标题 —— summary 行在文件尾部的会话在
// 列表页显示为无标题,可接受且诚实。
func ScanClaude(root string) ([]SessionMeta, error) {
	if root == "" {
		return nil, nil // 连根都拿不到(如 home 目录解析失败)= 无会话,不报错
	}
	if _, err := os.Stat(root); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []SessionMeta
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrPermission) || errors.Is(err, fs.ErrNotExist) {
				return nil // 单个不可达目录跳过,不挡整体
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil // 拿不到元信息的文件不进列表,也不挡整体
		}
		out = append(out, SessionMeta{
			Agent:     AgentClaude,
			ID:        strings.TrimSuffix(d.Name(), ".jsonl"),
			Title:     claudeTitlePreview(path),
			File:      path,
			Modified:  info.ModTime(),
			SizeBytes: info.Size(),
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	return out, nil
}

// claudeTitlePreview 读文件开头 8KB,返回其中第一条 summary 行的标题;
// 读不到(无 summary 行/文件不可读)返回空 —— 只取文件里真实存在的字段。
func claudeTitlePreview(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, 8*1024)
	n, _ := f.Read(buf)
	for _, line := range strings.Split(string(buf[:n]), "\n") {
		var head struct {
			Type    string `json:"type"`
			Summary string `json:"summary"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &head) == nil && head.Type == "summary" {
			return head.Summary
		}
	}
	return ""
}

// ReadClaudeSession 解析一个 Claude Code 会话文件,抽出可展示的消息序列。
//
// 【行格式(公开且长期稳定的口径)】每行一个 JSON 对象:
//   - {"type":"user"|"assistant","message":{...},...} 是消息行;
//   - message.content 是字符串,或内容块数组({type:"text",text:...} 等);
//   - {"type":"summary","summary":"标题",...} 是会话标题行(取第一条命中);
//   - 其余类型(system 等)与 isMeta/isSidechain 行不是主时间线内容,跳过并计数。
//
// 【文本抽取规则】只拼 type=="text" 的内容块(tool_use/tool_result/图片等不是
// 正文);块间以换行相接。字符串形态的 content 整体作为正文。
// 单行损坏(非 JSON):计一条跳过并继续 —— 一个坏行不废整个会话。
func ReadClaudeSession(path string) (*Session, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	s := &Session{SessionMeta: SessionMeta{Agent: AgentClaude, ID: id, File: path}}
	if info, err := os.Stat(path); err == nil {
		s.Modified = info.ModTime()
		s.SizeBytes = info.Size()
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		title, msg, ok := claudeLine(line)
		if title != "" && s.Title == "" {
			s.Title = title
		}
		if !ok {
			s.SkippedLines++
			continue
		}
		s.Messages = append(s.Messages, *msg)
	}
	return s, nil
}

// claudeLine 解析一行:返回 (会话标题, 消息, 是否为可展示消息)。
// 标题只在 summary 行非空;ok=false 的行仍可能带回标题(如 summary 行)。
func claudeLine(line string) (string, *Message, bool) {
	var head struct {
		Type        string `json:"type"`
		IsMeta      bool   `json:"isMeta"`
		IsSidechain bool   `json:"isSidechain"`
		Timestamp   string `json:"timestamp"`
		Summary     string `json:"summary"`
		Message     *struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(line), &head); err != nil {
		return "", nil, false // 损坏行:如实跳过
	}
	if head.Type == "summary" {
		return head.Summary, nil, false
	}
	if head.Message == nil || head.IsMeta || head.IsSidechain {
		return "", nil, false
	}
	msg := &Message{Role: head.Message.Role}
	if msg.Role == "" {
		msg.Role = head.Type // role 缺失按行 type 如实记
	}
	if head.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, head.Timestamp); err == nil {
			msg.At = t
		}
	}
	msg.Text = claudeText(head.Message.Content)
	if msg.Text == "" {
		return "", nil, false // 无正文的行(纯 tool_use 等)不进时间线,按跳过计数
	}
	return "", msg, true
}

// claudeText 从 content 抽正文:字符串原样;数组只拼 text 块(块间换行)。
func claudeText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asText string
	if err := json.Unmarshal(raw, &asText); err == nil {
		return asText
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "" // 形状认不出:不猜,返回空由调用方按"无正文"处理
	}
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}
