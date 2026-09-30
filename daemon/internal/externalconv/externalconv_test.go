package externalconv

// externalconv_test.go —— 只读解析器的合成夹具单测。
//
// 【为什么全是合成样例】2026-09-30 本机实测(见包注释):三家外部 agent 均无
// 已持久化会话,没有真实文件可读。夹具按 Claude Code 公开且长期稳定的 JSONL
// 行格式自造(与 server 层 pi 夹具同款做法),绝不伪造"真实数据"。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// seedClaudeSession 造一个覆盖全部行型的合成会话文件,返回路径。
func seedClaudeSession(t *testing.T, dir, name string) string {
	t.Helper()
	body := strings.Join([]string{
		`{"type":"summary","summary":"修复登录页","leafUuid":"u9"}`,
		`{"type":"user","message":{"role":"user","content":"帮我看下登录超时"},"uuid":"u1","timestamp":"2026-09-30T10:00:00.000Z"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"先看日志。"},{"type":"tool_use","name":"Read","input":{"path":"a.go"}}]},"uuid":"u2","timestamp":"2026-09-30T10:00:05.000Z"}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"..."}]},"uuid":"u3"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"定位到了:"}]},"uuid":"u4"}`,
		`{"type":"user","isMeta":true,"message":{"role":"user","content":"meta 行不进时间线"},"uuid":"u5"}`,
		`{"type":"assistant","isSidechain":true,"message":{"role":"assistant","content":[{"type":"text","text":"子代理行不进主时间线"}]},"uuid":"u6"}`,
		`{"type":"system","content":"系统行","uuid":"u7"}`,
		`{corrupt-json`,
		``,
	}, "\n") + "\n"
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestClaudeRoot(t *testing.T) {
	if got, want := ClaudeRoot(`C:\Users\u`), filepath.Join(`C:\Users\u`, ".claude", "projects"); got != want {
		t.Errorf("ClaudeRoot = %q, want %q", got, want)
	}
}

func TestScanClaudeDiscoveryOrderAndMissingRoot(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "-Users-u-projA")
	dirB := filepath.Join(root, "-Users-u-projB")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatal(err)
	}
	old := seedClaudeSession(t, dirA, "11111111-aaaa-4bbb-bccc-000000000001.jsonl")
	newer := seedClaudeSession(t, dirB, "22222222-aaaa-4bbb-bccc-000000000002.jsonl")
	future := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(newer, future, future); err != nil {
		t.Fatal(err)
	}
	// 非 jsonl 文件不收。
	if err := os.WriteFile(filepath.Join(dirA, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	metas, err := ScanClaude(root)
	if err != nil {
		t.Fatalf("ScanClaude: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("应发现 2 个会话,得到 %d: %+v", len(metas), metas)
	}
	if metas[0].ID != "22222222-aaaa-4bbb-bccc-000000000002" || metas[0].File != newer {
		t.Errorf("应按修改时间倒序,最新的在前: %+v", metas[0])
	}
	if metas[1].File != old || metas[1].Agent != AgentClaude {
		t.Errorf("第二项应是旧会话且 agent=claude: %+v", metas[1])
	}
	if got := metas[0].Title; got != "修复登录页" {
		t.Errorf("列表项标题应取文件头 8KB 内第一条 summary 行,得到 %q", got)
	}

	// 根不存在 = 无会话,不报错(导入页语义)。
	empty, err := ScanClaude(filepath.Join(root, "not-exist"))
	if err != nil || len(empty) != 0 {
		t.Errorf("缺失根应返回空列表 + nil,得到 (%v, %v)", empty, err)
	}
}

func TestReadClaudeSessionExtractionAndSkipAccounting(t *testing.T) {
	root := t.TempDir()
	p := seedClaudeSession(t, root, "33333333-aaaa-4bbb-bccc-000000000003.jsonl")

	s, err := ReadClaudeSession(p)
	if err != nil {
		t.Fatalf("ReadClaudeSession: %v", err)
	}
	if s.Title != "修复登录页" {
		t.Errorf("标题应取 summary 行,得到 %q", s.Title)
	}
	if s.ID != "33333333-aaaa-4bbb-bccc-000000000003" {
		t.Errorf("ID 应取文件名去扩展名,得到 %q", s.ID)
	}
	// 期望时间线:user 文本 → assistant 文本(拼 text 块)→ assistant 文本。
	// user 的纯 tool_result 行、meta/sidechain/system/损坏行都不进时间线。
	type want struct{ role, text string }
	wants := []want{
		{"user", "帮我看下登录超时"},
		{"assistant", "先看日志。"},
		{"assistant", "定位到了:"},
	}
	if len(s.Messages) != len(wants) {
		t.Fatalf("应抽出 %d 条消息,得到 %d: %+v", len(wants), len(s.Messages), s.Messages)
	}
	for i, w := range wants {
		if s.Messages[i].Role != w.role || s.Messages[i].Text != w.text {
			t.Errorf("消息[%d] = (%q, %q), want (%q, %q)", i, s.Messages[i].Role, s.Messages[i].Text, w.role, w.text)
		}
	}
	// 如实计数:summary 标题行 / 纯 tool_result 行 / meta / sidechain / system
	// / 损坏行 = 6(空行不计)。
	if s.SkippedLines != 6 {
		t.Errorf("SkippedLines = %d, want 6", s.SkippedLines)
	}
	// 行级时间戳落在消息上(RFC3339)。
	if want, err := time.Parse(time.RFC3339, "2026-09-30T10:00:00.000Z"); err != nil || !s.Messages[0].At.Equal(want) {
		t.Errorf("消息[0].At = %v, want %v", s.Messages[0].At, want)
	}
	if s.Messages[2].At != (time.Time{}) {
		t.Errorf("缺 timestamp 的行 At 应为零值,得到 %v", s.Messages[2].At)
	}
}

func TestReadClaudeSessionMissingFile(t *testing.T) {
	if _, err := ReadClaudeSession(filepath.Join(t.TempDir(), "nope.jsonl")); err == nil {
		t.Fatal("读不存在的会话文件必须报错")
	}
}

// 形状认不出的 content 不猜:返回空 → 行按"无正文"跳过计数。
func TestClaudeTextUnknownShapeStaysEmpty(t *testing.T) {
	if got := claudeText(json.RawMessage(`{"weird":true}`)); got != "" {
		t.Errorf("认不出的 content 形状必须返回空,得到 %q", got)
	}
	if got := claudeText(json.RawMessage(`[{"type":"image","source":{}}]`)); got != "" {
		t.Errorf("无 text 块的数组必须返回空,得到 %q", got)
	}
	if got := claudeText(json.RawMessage(`[{"type":"text","text":"a"},{"type":"text","text":"b"}]`)); got != "a\nb" {
		t.Errorf("text 块应以换行相接,得到 %q", got)
	}
}
