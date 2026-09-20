package codexapp

// stderr_drain_test.go —— T-12 回归钉子(AUDIT §4 缺陷即测试):codex 子进程的
// stderr 必须逐行落 daemon.log,不许透传 os.Stderr(生产 daemon 无控制台,
// 透传 = 诊断信息全丢,piapp/zcodeapp 当年同款病)。进程级管道接线难以单测,
// 验证口径参照 piapp 先例:这里钉核心 drainStderr 的行为契约 —— 逐行回调、
// 空行/截断行不丢、非 EOF 读错误记一行且不 panic、读完即终止;生产接线
// (StderrPipe + log.Printf("[codex-stderr] …") 注入)由代码审查确认。

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// drainCollect 注入收集器跑一遍 drainStderr,返回回调收到的全部行。
func drainCollect(t *testing.T, r io.Reader) []string {
	t.Helper()
	var got []string
	drainStderr(r, func(line string) { got = append(got, line) })
	return got
}

// TestDrainStderrLineByLine 多行输入逐行回调,一行不丢、内容不改。
// bytes.Buffer 读尽即 EOF:同时钉住"EOF 自然终止"这一条(函数同步返回,
// 不挂起、不再需要任何外部关闭动作)。
func TestDrainStderrLineByLine(t *testing.T) {
	got := drainCollect(t, bytes.NewBufferString("alpha\nbeta\ngamma\n"))
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("回调行数 = %d, want %d (got=%q)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 行 = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDrainStderrEmptyAndTruncatedLines 空行原样透传(诊断输出里空行也可能是
// 排版线索,不过滤 —— 与 piapp/zcodeapp 行为一致);无结尾换行的截断行(进程
// 死于半行输出,恰是最需要落日志的死因)同样必须回调,不能被 Scanner 静默吞掉。
func TestDrainStderrEmptyAndTruncatedLines(t *testing.T) {
	t.Run("空行透传", func(t *testing.T) {
		got := drainCollect(t, bytes.NewBufferString("a\n\nb\n"))
		want := []string{"a", "", "b"}
		if len(got) != len(want) {
			t.Fatalf("回调行数 = %d, want %d (got=%q)", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("第 %d 行 = %q, want %q", i, got[i], want[i])
			}
		}
	})
	t.Run("截断行不丢", func(t *testing.T) {
		got := drainCollect(t, bytes.NewBufferString("a\nhalf-dead"))
		want := []string{"a", "half-dead"}
		if len(got) != len(want) {
			t.Fatalf("回调行数 = %d, want %d (got=%q)", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("第 %d 行 = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

// errReader 先吐一段数据,此后每次 Read 固定返回非 EOF 错误(模拟收尸方
// Wait 先一步关管道等异常),用于验证错误路径:记一行、不 panic、终止。
type errReader struct {
	data string
	err  error
	done bool
}

func (r *errReader) Read(p []byte) (int, error) {
	if !r.done {
		r.done = true
		return copy(p, r.data), nil
	}
	return 0, r.err
}

// TestDrainStderrReaderErrorNoPanic reader 报非 EOF 错误:已读到的半行照常
// 回调,再补一行"读取异常"说明读不下去了;函数同步返回即证明终止(不终止
// 测试会整体超时),不 panic 即通过。同步调用,race 下也不许 data race。
func TestDrainStderrReaderErrorNoPanic(t *testing.T) {
	boom := errors.New("pipe closed by reap")
	got := drainCollect(t, &errReader{data: "partial-line\n", err: boom})
	if len(got) != 2 {
		t.Fatalf("回调行数 = %d, want 2 (got=%q)", len(got), got)
	}
	if got[0] != "partial-line" {
		t.Errorf("错误前的行 = %q, want %q", got[0], "partial-line")
	}
	if !strings.HasPrefix(got[1], "读取异常: ") || !strings.Contains(got[1], "pipe closed by reap") {
		t.Errorf("错误行 = %q, want 前缀 %q 且含原始错误 %q", got[1], "读取异常: ", "pipe closed by reap")
	}
}

// TestDrainStderrEmptyInput 空输入(子进程一个字都没说就死了):不回调任何行、
// 不报错,函数立即返回。
func TestDrainStderrEmptyInput(t *testing.T) {
	got := drainCollect(t, strings.NewReader(""))
	if len(got) != 0 {
		t.Fatalf("空输入不应回调, got=%q", got)
	}
}
