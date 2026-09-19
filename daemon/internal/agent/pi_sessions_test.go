package agent

// Pi 会话发现与历史读取的单测:构造临时会话树,逐条钉死
// "文件头 cwd 不可从目录名反推"、"排序截断"、"历史只取 user/assistant 文本"。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// piHead 造一行 type=="session" 的文件头。
func piHead(id, cwd string) string {
	return fmt.Sprintf(`{"type":"session","version":3,"id":%q,"timestamp":"2026-09-13T15:17:43.587Z","cwd":%q}`, id, cwd)
}

// writePiSession 造一个最小可解析的 Pi 会话文件,返回其路径。
func writePiSession(t *testing.T, dir, name, head string, msgs ...string) string {
	t.Helper()
	lines := append([]string{head}, msgs...)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("写测试会话文件失败: %v", err)
	}
	return path
}

func TestScanPiSessionsHeaderParse(t *testing.T) {
	root := t.TempDir()
	// 目录名故意与 cwd 无关(真实形态是 --C--dev-…,但"反推不成立"是关键假设):
	// cwd 只能来自文件头,不能从目录名拼出来。
	wsDir := filepath.Join(root, "--G--x--")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const sid = "01a09b58-8022-7602-a540-ce81845a394a"
	writePiSession(t, wsDir, "2026-09-13T15-17-43-587Z_"+sid+".jsonl",
		piHead(sid, `G:\项目1`),
		`{"type":"model_change","id":"2c8a3a97","timestamp":"2026-09-13T15:17:43.635Z"}`,
		`{"type":"message","id":"m1","timestamp":"2026-09-13T15:17:53.688Z","message":{"role":"user","content":[{"type":"text","text":"帮我修一下会话列表的排序"}],"timestamp":1789312673685}}`,
		`{"type":"message","id":"m2","timestamp":"2026-09-13T15:17:56.274Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"想一下"},{"type":"text","text":"好的"}],"timestamp":1789312673904}}`,
	)

	got, err := scanPiSessionsFrom(root, 0)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("应扫出 1 条会话,得到 %d", len(got))
	}
	s := got[0]
	if s.SessionID != sid {
		t.Fatalf("sessionId 应来自文件头,得到 %q", s.SessionID)
	}
	if s.Cwd != `G:\项目1` {
		t.Fatalf("cwd 必须来自文件头(目录名不可反推),得到 %q", s.Cwd)
	}
	if s.Title != "帮我修一下会话列表的排序" {
		t.Fatalf("标题应为首条用户消息,得到 %q", s.Title)
	}
	if !strings.HasSuffix(s.SessionFile, ".jsonl") || !filepath.IsAbs(s.SessionFile) {
		t.Fatalf("SessionFile 应为绝对路径: %q", s.SessionFile)
	}
	if s.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt 应取文件 mtime")
	}
}

func TestScanPiSessionsHeadlessFileFallsBackToName(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	// 没有文件头的损坏文件:id 从文件名回退;标题兜底"未命名会话"。
	writePiSession(t, root, "2026-09-13T15-17-43-587Z_deadbeef-0000-7000-8000-000000000000.jsonl",
		`{"type":"message","id":"m1","message":{"role":"assistant","content":[{"type":"text","text":"只有助手消息"}]}}`,
	)
	got, err := scanPiSessionsFrom(root, 0)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	if len(got) != 1 || got[0].SessionID != "deadbeef-0000-7000-8000-000000000000" {
		t.Fatalf("应按文件名回退解析 id,得到 %+v", got)
	}
	if got[0].Title != "未命名会话" {
		t.Fatalf("无用户消息应兜底标题,得到 %q", got[0].Title)
	}
}

func TestScanPiSessionsSortAndTruncate(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	old := now.Add(-2 * time.Hour)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	// piSessionsLimit + 2 个文件:应截断到上限,且被截掉的是"最老"的那两个。
	for i := 0; i < piSessionsLimit+2; i++ {
		name := fmt.Sprintf("2026-01-01T00-00-00-000Z_s%03d.jsonl", i)
		p := writePiSession(t, root, name,
			piHead(fmt.Sprintf("sid-%03d", i), `G:\x`),
			fmt.Sprintf(`{"type":"message","message":{"role":"user","content":[{"type":"text","text":"第%d条"}]}}`, i),
		)
		stamp := old
		if i >= 2 {
			stamp = now
		}
		if err := os.Chtimes(p, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	got, err := scanPiSessionsFrom(root, 0)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	if len(got) != piSessionsLimit {
		t.Fatalf("应截断到 %d 条,得到 %d", piSessionsLimit, len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].UpdatedAt.Before(got[i].UpdatedAt) {
			t.Fatalf("应按修改时间倒序:%v 排在了 %v 前面", got[i-1].UpdatedAt, got[i].UpdatedAt)
		}
	}
	for _, s := range got {
		if s.SessionID == "sid-000" || s.SessionID == "sid-001" {
			t.Fatalf("最老的会话 %s 应被截断丢弃", s.SessionID)
		}
	}
}

// TestScanPiSessionsLimitTruncatesToMostRecent 显式 limit 的超限截断:只留
// 最近 n 条,最老的被裁掉(对齐 server 侧 TestLimitZcodeSessionsTruncatesToMostRecent)。
func TestScanPiSessionsLimitTruncatesToMostRecent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	// 5 个文件 mtime 依次更新(i 越大越新),limit=3:应只留最近 3 条。
	for i := 0; i < 5; i++ {
		p := writePiSession(t, root, fmt.Sprintf("2026-01-01T00-00-0%d-000Z_s%02d.jsonl", i, i),
			piHead(fmt.Sprintf("sid-%02d", i), `G:\x`),
			fmt.Sprintf(`{"type":"message","message":{"role":"user","content":[{"type":"text","text":"第%d条"}]}}`, i),
		)
		stamp := now.Add(time.Duration(i-5) * time.Minute)
		if err := os.Chtimes(p, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	got, err := scanPiSessionsFrom(root, 3)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("应截断到 3 条,得到 %d", len(got))
	}
	if got[0].SessionID != "sid-04" || got[2].SessionID != "sid-02" {
		t.Fatalf("截断后应保留最新 3 条(04/03/02),得到 %s…%s", got[0].SessionID, got[2].SessionID)
	}
	for _, s := range got {
		if s.SessionID == "sid-00" || s.SessionID == "sid-01" {
			t.Fatalf("最老的会话 %s 应被截断丢弃", s.SessionID)
		}
	}
}

// TestScanPiSessionsLimitPassthrough 不足上限:一条不少,仍按 mtime 倒序;
// 空目录不炸(对齐 server 侧 TestLimitZcodeSessionsPassthrough 的"原样"语义 ——
// 扫描的契约本身就是按修改时间倒序,这里钉的是"不截、不丢")。
func TestScanPiSessionsLimitPassthrough(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i := 0; i < 3; i++ {
		p := writePiSession(t, root, fmt.Sprintf("2026-01-01T00-00-0%d-000Z_s%02d.jsonl", i, i),
			piHead(fmt.Sprintf("sid-%02d", i), `G:\x`),
			fmt.Sprintf(`{"type":"message","message":{"role":"user","content":[{"type":"text","text":"第%d条"}]}}`, i),
		)
		stamp := now.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(p, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	got, err := scanPiSessionsFrom(root, 10)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("不足上限应一条不少,得到 %d 条", len(got))
	}
	// 最新(序号 02)在前,倒序不因"没截断"而丢失。
	for i, want := range []string{"sid-02", "sid-01", "sid-00"} {
		if got[i].SessionID != want {
			t.Fatalf("第 %d 条应为 %s,得到 %s", i, want, got[i].SessionID)
		}
	}
	// 空目录 + 大上限:空列表,不报错。
	empty, err := scanPiSessionsFrom(t.TempDir(), 100)
	if err != nil || len(empty) != 0 {
		t.Fatalf("空目录应返回空列表: %v %+v", err, empty)
	}
}

// TestScanPiSessionsLimitStableOnTies 同一修改时刻的文件:保持扫描(文件名字典)
// 原序截前 n 条 —— 这正是 sort.Slice 换成 SliceStable 要钉死的行为
// (对齐 server 侧 TestLimitZcodeSessionsStableOnTies)。
func TestScanPiSessionsLimitStableOnTies(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now()
	for i := 0; i < 4; i++ {
		p := writePiSession(t, root, fmt.Sprintf("2026-01-01T00-00-00-000Z_s%02d.jsonl", i),
			piHead(fmt.Sprintf("sid-%02d", i), `G:\x`),
			fmt.Sprintf(`{"type":"message","message":{"role":"user","content":[{"type":"text","text":"第%d条"}]}}`, i),
		)
		// 四条同一个 mtime:模拟"同一批会话"毫秒级同刻。
		if err := os.Chtimes(p, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	got, err := scanPiSessionsFrom(root, 2)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	if len(got) != 2 || got[0].SessionID != "sid-00" || got[1].SessionID != "sid-01" {
		t.Fatalf("同刻条目应按文件名字典序保留前两条,得到 %+v", got)
	}
}

func TestScanPiSessionsTitleClip(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("长", 100)
	writePiSession(t, root, "s.jsonl", piHead("sid-clip", `G:\x`),
		fmt.Sprintf(`{"type":"message","message":{"role":"user","content":[{"type":"text","text":%q}]}}`, long),
	)
	got, err := scanPiSessionsFrom(root, 0)
	if err != nil || len(got) != 1 {
		t.Fatalf("扫描失败: %v %+v", err, got)
	}
	if got[0].Title != strings.Repeat("长", 40) {
		t.Fatalf("标题应按 rune 截到 40,得到 %d 字", len([]rune(got[0].Title)))
	}
}

func TestReadPiSessionHistory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writePiSession(t, root, "s.jsonl",
		piHead("sid-1", `G:\项目1`),
		`{"type":"message","timestamp":"2026-09-13T15:17:53.688Z","message":{"role":"user","content":[{"type":"text","text":"第一问"}],"timestamp":1789312673685}}`,
		`{"type":"message","timestamp":"2026-09-13T15:17:56.274Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"推理过程不该出现在时间线"},{"type":"text","text":"第一答"}]}}`,
		`{"type":"message","timestamp":"2026-09-13T15:18:03.026Z","message":{"role":"toolResult","toolCallId":"c1","toolName":"bash","content":[{"type":"text","text":"工具输出不进时间线"}]}}`,
		`{"type":"message","timestamp":"2026-09-13T15:18:10.000Z","message":{"role":"user","content":[{"type":"text","text":"第二问"}]}}`,
		`这行不是 JSON,应被跳过`,
	)
	msgs, err := ReadPiSessionHistory(p)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("应抽出 3 条(user/assistant/user),得到 %d:%+v", len(msgs), msgs)
	}
	want := []struct{ role, text string }{
		{"user", "第一问"}, {"assistant", "第一答"}, {"user", "第二问"},
	}
	for i, w := range want {
		if msgs[i].Role != w.role || msgs[i].Text != w.text {
			t.Fatalf("第 %d 条应为 %s/%q,得到 %s/%q", i, w.role, w.text, msgs[i].Role, msgs[i].Text)
		}
	}
	if msgs[0].At.IsZero() {
		t.Fatal("消息时间应来自 entry 的 timestamp 字段")
	}
}

func TestParsePiSessionHeadLineSkipsBadLines(t *testing.T) {
	head := &piSessionHead{}
	have := false
	parsePiSessionHeadLine([]byte(`{broken`), head, &have)
	if head.ID != "" {
		t.Fatal("坏行不得产生 id")
	}
	parsePiSessionHeadLine([]byte(piHead("01a09b58", `G:\项目1`)), head, &have)
	if head.ID != "01a09b58" || head.Cwd != `G:\项目1` {
		t.Fatalf("文件头解析失败: %+v", head)
	}
}
