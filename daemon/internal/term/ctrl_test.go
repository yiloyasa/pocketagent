package term

import (
	"bytes"
	"testing"
)

// 翻译层用例:只动 0x03/0x1c,其余字节必须原样透传。
func TestTranslateInputPassthrough(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"普通命令", "ping -t 127.0.0.1\r\n"},
		{"方向键 VT 序列", "\x1b[A\x1b[B\x1b[C\x1b[D"},
		{"win32-input-mode 序列", "\x1b[67;46;3;1;8;1_"},
		{"粘贴多行文本", "echo a\r\necho b\r\n"},
		{"空输入", ""},
		{"DECSET 序列", "\x1b[?9001h\x1b[?2004h"},
	}
	for _, c := range cases {
		acts := translateInput([]byte(c.in))
		var joined bytes.Buffer
		for _, a := range acts {
			if a.ctrl {
				t.Fatalf("%s: 普通输入被误判为中断", c.name)
			}
			joined.Write(a.data)
		}
		if joined.String() != c.in {
			t.Fatalf("%s: 透传字节被改动: %q != %q", c.name, joined.String(), c.in)
		}
	}
}

// 单独与混合的 0x03/0x1c 切分。
func TestTranslateInputControlBytes(t *testing.T) {
	acts := translateInput([]byte("\x03"))
	if len(acts) != 1 || !acts[0].ctrl || acts[0].isBreak {
		t.Fatalf("\\x03 应为一个 Ctrl+C 动作: %+v", acts)
	}

	acts = translateInput([]byte("\x1c"))
	if len(acts) != 1 || !acts[0].ctrl || !acts[0].isBreak {
		t.Fatalf("\\x1c 应为一个 Ctrl+Break 动作: %+v", acts)
	}

	// 前缀 + Ctrl+C + 尾段:三段
	acts = translateInput([]byte("ping -t 127.0.0.1\r\n\x03ls\r\n"))
	if len(acts) != 3 {
		t.Fatalf("应切为 3 段: %+v", acts)
	}
	if string(acts[0].data) != "ping -t 127.0.0.1\r\n" || !acts[1].ctrl || acts[1].isBreak || string(acts[2].data) != "ls\r\n" {
		t.Fatalf("切分内容不符: %+v", acts)
	}
}

// win32KeySeq 输出必须符合 CSI Vk;Sc;Uc;Kd;Cs;Rc _ 六参格式。
func TestWin32KeySeq(t *testing.T) {
	got := string(win32KeySeq(3, 70, 0, 1, 8, 1))
	want := "\x1b[3;70;0;1;8;1_"
	if got != want {
		t.Fatalf("win32KeySeq = %q, want %q", got, want)
	}
}
