package stderrdrain

// stderrdrain_test.go —— 统一 stderr 收口的回归钉子。
//
// 钉两件事:①**凭据脱敏**(api_key / token / JWT / Bearer 不许原样落进日志);
// ②**限流**(单行截断 + 累计上限,刷屏不撑爆 daemon.log)。
// 另钉一条**反过度脱敏**:普通诊断文本(hash / 路径 / 错误原文)必须原样保留 ——
// 把死因也打码掉,等于把 stderr 捕获本身变成无用功。

import (
	"strings"
	"testing"
)

func TestRedactMasksKeyValueSecrets(t *testing.T) {
	cases := []struct{ in, secret string }{
		{`api_key=sk-abcdefghijklmnopqrstuvwxyz012345`, "sk-abcdefghijklmnopqrstuvwxyz012345"},
		{`Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.abcdefghij`, "eyJhbGciOiJIUzI1NiJ9"},
		{`DEEPSEEK_API_KEY="dsk-1234567890abcdefg"`, "dsk-1234567890abcdefg"},
		{`token: 'glm-ABCDEFGHIJKLMNOPQRSTUVWXYZ'`, "glm-ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
	}
	for _, c := range cases {
		got := Redact(c.in)
		if strings.Contains(got, c.secret) {
			t.Fatalf("凭据未脱敏:\n in =%s\n out=%s", c.in, got)
		}
		if !strings.Contains(got, "[redacted") {
			t.Fatalf("应留下 redacted 标记, got=%s", got)
		}
	}
}

func TestRedactKeepsDiagnosticText(t *testing.T) {
	for _, s := range []string{
		`MCP client for idalib failed to start: program not found`,
		`panic at G:\Claude Code\node_modules\dsh\lib\bin.js:12`,
		`cwd=C:\Users\yanzi\.pocketagentd\dsh-home`,
		`request_user_input is unavailable in Default mode`,
	} {
		if got := Redact(s); got != s {
			t.Fatalf("诊断文本被误改(过度脱敏会把死因也抹掉):\n in =%s\n out=%s", s, got)
		}
	}
}

func TestDrainEmitsLinesAndPassesEmpty(t *testing.T) {
	var got []string
	Drain(strings.NewReader("line1\n\nline2\n"), Options{}, func(l string) { got = append(got, l) })
	want := []string{"line1", "", "line2"}
	if len(got) != len(want) {
		t.Fatalf("行数不符: got=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 行不符: got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestDrainTruncatesOverlongLine(t *testing.T) {
	long := strings.Repeat("x", 5000)
	var got []string
	Drain(strings.NewReader(long+"\n"), Options{MaxLineLog: 100}, func(l string) { got = append(got, l) })
	if len(got) != 1 {
		t.Fatalf("应只有一行, got=%d 行", len(got))
	}
	if !strings.Contains(got[0], "行过长已截断") {
		t.Fatalf("超长行应带截断标记, got 前缀=%q", got[0][:60])
	}
	if len(got[0]) > 100+len(" …[行过长已截断]") {
		t.Fatalf("截断后仍过长: %d", len(got[0]))
	}
}

func TestDrainStopsAtTotalLimit(t *testing.T) {
	// 每行 60 字节、上限 100 → 落 1 行后就该触发限流并补说明。
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, strings.Repeat("a", 60))
	}
	var got []string
	Drain(strings.NewReader(strings.Join(lines, "\n")+"\n"),
		Options{MaxTotal: 100}, func(l string) { got = append(got, l) })
	if len(got) == 0 || !strings.Contains(got[len(got)-1], "已限流") {
		t.Fatalf("应补一行限流说明, got=%v", got)
	}
	if len(got) > 3 {
		t.Fatalf("限流后不该继续输出, got=%d 行", len(got))
	}
}

func TestDrainRedactsEveryEmittedLine(t *testing.T) {
	var got []string
	Drain(strings.NewReader("api_key=secret-value-1234567890\n"), Options{},
		func(l string) { got = append(got, l) })
	if len(got) != 1 || strings.Contains(got[0], "secret-value-1234567890") {
		t.Fatalf("Drain 吐出的每行都必须已脱敏, got=%v", got)
	}
	if !lineHasSecret("api_key=secret-value-1234567890") {
		t.Fatal("lineHasSecret 应识别出含凭据的行")
	}
	if lineHasSecret("program not found") {
		t.Fatal("lineHasSecret 不该把普通诊断行当凭据")
	}
}
