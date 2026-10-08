// eventstore_degrade_test.go —— 降级自愈回归(2026-10-09)。
//
// 旧缺陷:①Append 写错误后降级 mem-only,直到重启才恢复;②轮转 Remove/Rename
// 失败(Windows 共享冲突即可触发)后 s.f 已关但 memOnly 未置,后续帧静默不落盘
// 且永不自愈。两条路径现在都会:真正降级 → WARNING("事件落盘已暂停")→ 按
// recoverInterval 节拍重开文件,成功即恢复落盘(INFO)。
//
// 【时间注入】recoverInterval 是白盒字段,测试直接调小(缩时),不真等 30s。
// 【写错误注入】白盒关掉底层句柄,再写一行超过 32KB 行缓冲的大行强制冲缓冲
// 触盘写 —— 对 closed *os.File 的写必失败,跨平台确定性。
// 【轮转失败注入】把 events-1.jsonl 占成非空目录,os.Remove 必败(ENOTEMPTY,
// 跨平台),命中 rotateLocked 的 Remove 失败分支 —— 与 Windows 共享冲突同路。
package eventstore

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// captureLog 截获标准 log 输出,返回读取函数与恢复函数。
func captureLog() (read func() string, restore func()) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	return buf.String, func() { log.SetOutput(os.Stderr) }
}

// diskSeqs 直接读盘(两代文件)收集全部可解析行的 seq,验证真实落盘内容。
func diskSeqs(t *testing.T, dir string) []int64 {
	t.Helper()
	var out []int64
	for _, name := range []string{RotatedName, FileName} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, line := range splitLines(raw) {
			var e Event
			if jsonOk(line, &e) {
				out = append(out, e.Seq)
			}
		}
	}
	return out
}

func seqsEqual(got []int64, want ...int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestDegradeRecoversAfterWriteError 写错误 → 降级(seq 照常单调;未刷缓冲随
// 降级丢弃,降级窗口不再落盘)→ 过自愈节拍 → 重开成功恢复落盘,窗口内缺失
// 的 seq 如实留缺(续传断点语义兜底)。
func TestDegradeRecoversAfterWriteError(t *testing.T) {
	readLog, restore := captureLog()
	defer restore()

	dir := t.TempDir()
	s := mustOpen(t, dir)
	s.recoverInterval = 50 * time.Millisecond // 缩时:降级 50ms 后即可自愈

	s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"n":1}`))
	s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"n":2}`)) // 仍在 32KB 用户态缓冲

	// 注入写失败:底层句柄失效(等价磁盘 IO 错误)
	s.mu.Lock()
	if err := s.f.Close(); err != nil {
		t.Fatalf("注入关句柄: %v", err)
	}
	s.mu.Unlock()

	// >32KB 的大行强制冲缓冲触盘写 → 失败 → 降级
	big := bytes.Repeat([]byte("a"), appendBufSize+1024)
	if got := s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", big); got != 3 {
		t.Fatalf("降级帧 seq = %d, want 3", got)
	}
	if !s.memOnly {
		t.Fatal("写错误后应降级 mem-only")
	}
	// 降级窗口内:seq 照常单调、不落盘(时间门控未到,不白打重开)
	if got := s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"n":4}`)); got != 4 {
		t.Fatalf("降级期 seq = %d, want 4(计数不受影响)", got)
	}
	if !s.memOnly {
		t.Fatal("自愈节拍未到不应尝试恢复")
	}
	_ = s.Close() // 冲干净能冲的,把盘面定格
	// seq1/2 在用户态缓冲里、随降级一并丢弃(见 degradeLocked 的权衡注释);
	// seq3/4 降级窗口不落盘 → 盘上空。
	if got := diskSeqs(t, dir); len(got) != 0 {
		t.Fatalf("降级窗口内不应有任何落盘,磁盘 seq = %v", got)
	}

	// 过一个自愈节拍 → 下一帧触发重开成功,恢复落盘(同一 Store 实例,无重启)
	time.Sleep(100 * time.Millisecond)
	if got := s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"n":5}`)); got != 5 {
		t.Fatalf("自愈帧 seq = %d, want 5", got)
	}
	if s.memOnly {
		t.Fatal("自愈节拍到期后仍未恢复落盘")
	}
	_ = s.Close()
	if got := diskSeqs(t, dir); !seqsEqual(got, 5) {
		t.Fatalf("自愈后磁盘 seq = %v, want [5](1-4 的缺口如实保留,由续传断点语义兜底)", got)
	}
	if !strings.Contains(readLog(), "恢复落盘") {
		t.Fatalf("缺少自愈成功 INFO 日志: %q", readLog())
	}
}

// TestRotateFailureDegradesWithWarningAndHeals 轮转 Remove 失败 → 真正降级 +
// WARNING 醒目日志(含"事件落盘已暂停")→ 解除占用并过节拍 → 自愈重开,
// 下一帧顺带补上此前失败的轮转(文件仍超限,轮转判断照常生效),落盘恢复。
func TestRotateFailureDegradesWithWarningAndHeals(t *testing.T) {
	readLog, restore := captureLog()
	defer restore()

	dir := t.TempDir()
	s := mustOpen(t, dir)
	s.recoverInterval = 50 * time.Millisecond
	s.SetMaxBytes(150) // 单行约 140B:第 2 条必触发轮转

	// 阻断轮转:目标代次名被非空目录占用 → os.Remove 必败(跨平台)
	blocked := filepath.Join(dir, RotatedName)
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"n":1}`))
	if got := s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"n":2}`)); got != 2 {
		t.Fatalf("轮转失败帧 seq = %d, want 2(计数照常)", got)
	}
	if !s.memOnly {
		t.Fatal("轮转失败(句柄已关)应真正降级 mem-only,否则后续帧静默不落盘")
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); err != nil {
		t.Fatalf("轮转失败不应动当前文件: %v", err)
	}
	logged := readLog()
	if !strings.Contains(logged, "[WARNING]") || !strings.Contains(logged, "事件落盘已暂停") {
		t.Fatalf("缺 WARNING 醒目日志(应含'事件落盘已暂停'): %q", logged)
	}
	_ = s.Close()
	// 轮转前缓冲已冲刷:seq1 在盘上;seq2 在失败帧上,未写盘。
	if got := diskSeqs(t, dir); !seqsEqual(got, 1) {
		t.Fatalf("磁盘 seq = %v, want [1](轮转冲刷保住了尾前帧,失败帧不落盘)", got)
	}

	// 解除占用 → 过自愈节拍 → 重开恢复;恢复出的 size 仍超限,
	// 本帧的轮转判断会补试轮转(Remove 这次成功),落盘链路完全恢复。
	if err := os.RemoveAll(blocked); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"n":3}`)); got != 3 {
		t.Fatalf("自愈帧 seq = %d, want 3", got)
	}
	if s.memOnly {
		t.Fatal("解除阻断并过节拍后应自愈恢复落盘")
	}
	_ = s.Close()
	if got := diskSeqs(t, dir); !seqsEqual(got, 1, 3) {
		t.Fatalf("自愈后磁盘 seq = %v, want [1 3](seq2 缺口如实保留)", got)
	}
	if _, err := os.Stat(filepath.Join(dir, RotatedName)); err != nil {
		t.Fatalf("自愈后应补上轮转(events-1.jsonl 应存在): %v", err)
	}
	if !strings.Contains(readLog(), "恢复落盘") {
		t.Fatalf("缺少自愈成功 INFO 日志: %q", readLog())
	}
}

// TestRecoverGatingThrottlesRetries 自愈门控:节拍未到时多次 Append 只尝试
// 重开一次 —— 磁盘持续坏时,热路径不能被打成周期性 OpenFile 热点。
// 注入:自愈目标路径指向不存在的子目录,OpenFile 必败(跨平台确定性)。
func TestRecoverGatingThrottlesRetries(t *testing.T) {
	readLog, restore := captureLog()
	defer restore()

	s := &Store{dir: t.TempDir(), maxBytes: DefaultMaxBytes, recoverInterval: time.Hour}
	s.path = filepath.Join(s.dir, "no-such-sub", FileName) // 父目录不存在,重开必败
	s.memOnly = true

	for i := 0; i < 5; i++ {
		s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{}`))
	}
	if !s.memOnly {
		t.Fatal("节拍(1h)未到不应离开降级态")
	}
	if n := strings.Count(readLog(), "自愈重开事件文件失败"); n != 1 {
		t.Fatalf("1 小时节拍内 5 次 Append 只应尝试重开 1 次,实际 %d 次", n)
	}
}
