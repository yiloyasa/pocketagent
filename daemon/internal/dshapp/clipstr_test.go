// clipstr_test.go —— clipStr CJK 截断语义的回归钉(2026-10-07 历史遗留项收口)。
//
// 旧实现 s[:n] 按字节硬截,会把多字节汉字劈成非法 UTF-8(parseSessionState 的
// "应答缺 sessionId" 报错带 CJK 原文时成乱码)。新语义:字节预算 n 不变,回退到
// 最后一个完整 rune 的边界再截(与 openai_compat.go 的 clip 同款修法)。
package dshapp

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestClipStrNeverSplitsRune(t *testing.T) {
	// 300 字节边界恰落在 rune 起点(3 字节字 × 100):回退量为 0,不多砍。
	s := strings.Repeat("汉", 200) // 600 字节
	got := clipStr(s, 300)
	wantBody := strings.Repeat("汉", 100)
	if got != wantBody+"...[截断]" {
		t.Fatalf("3 字节字整除边界应恰好截在 100 字: got=%q", got[:min(len(got), 40)])
	}
	if !utf8.ValidString(got) {
		t.Fatal("截断结果必须是合法 UTF-8")
	}

	// 边界落在字中间:回退到最近 rune 起点(最多回退 2 字节),绝不劈字。
	got = clipStr("汉字", 4) // 字节 6 > 4;s[4] 是"字"的续字节 → 回退到 3
	if got != "汉...[截断]" {
		t.Fatalf("CJK 中间边界应回退到 rune 起点: got=%q", got)
	}
	if !utf8.ValidString(strings.TrimSuffix(got, "...[截断]")) {
		t.Fatalf("截断体不是合法 UTF-8: %q", got)
	}

	// ASCII 语义不变:仍在字节 n 处截断。
	if got := clipStr("hello world", 5); got != "hello...[截断]" {
		t.Fatalf("ASCII 截断语义应不变: got=%q", got)
	}

	// 未超限(含短 CJK):原样返回,不截。
	if got := clipStr("汉字", 10); got != "汉字" {
		t.Fatalf("未超限应原样返回: got=%q", got)
	}
	if got := clipStr("", 10); got != "" {
		t.Fatalf("空串应原样返回: got=%q", got)
	}
}
