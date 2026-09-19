package agent

import (
	"strings"
	"testing"
)

// turnInput 必须**一字不改**地把用户输入封装进 turn/start。
//
// 【为什么值得单独钉一个测试】"粘贴长文本不应中断/被截断"是用户明确要求的真实能力。
// 这类回归最阴的地方是**静默**:加了上限后不报错、不失败,只是内容少了一半,
// 用户看到的是"agent 好像没读到我后半段"。普通功能测试根本抓不到。
// 所以这里用远超任何"合理上限"的输入(20 万字)正面钉住"零裁剪"。
func TestTurnInputKeepsLongPasteIntact(t *testing.T) {
	long := strings.Repeat("粘贴的长文本 with ascii & 符号 %d\n", 12000) // 约 24 万字符
	in := turnInput(long)

	if len(in) != 1 {
		t.Fatalf("input 应当只有 1 个元素, got=%d", len(in))
	}
	if in[0]["type"] != "text" {
		t.Fatalf("input[0].type 应为 text, got=%q", in[0]["type"])
	}
	got, _ := in[0]["text"].(string)
	if got != long {
		t.Fatalf("输入被改动:原 %d 字符, 实 %d 字符", len([]rune(long)), len([]rune(got)))
	}
	if len(got) != len(long) {
		t.Fatalf("字节数不一致:原 %d, 实 %d", len(long), len(got))
	}
}

// 空输入也要如实封装成空串(不能省略字段、不能替换成占位文案)。
func TestTurnInputKeepsEmptyInputVerbatim(t *testing.T) {
	in := turnInput("")
	if len(in) != 1 || in[0]["text"] != "" {
		t.Fatalf("空输入应原样封装, got=%#v", in)
	}
}

// 多行 / 首尾空白 / NUL 等易被"顺手清理"的形态同样必须原样保留。
func TestTurnInputDoesNotTrimOrNormalize(t *testing.T) {
	raw := "\n\n  前导空白\n\t制表符\r\n尾随空白  \n"
	in := turnInput(raw)
	if in[0]["text"] != raw {
		t.Fatalf("不得 trim/规范化换行:原 %q, 实 %q", raw, in[0]["text"])
	}
}
