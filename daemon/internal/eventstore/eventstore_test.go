// eventstore_test.go —— P1-5 事件持久化的行为钉子。
//
// 覆盖:追加与 seq 单调、重启恢复 max、轮转(保留一代+minSeq 记账)、
// 损坏行跳过(读写两侧)、按线程回放、after 过滤、超窗(ResyncRequired)判定、
// 空目录/内存模式降级。
package eventstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// mustOpen 打开测试目录的存储,失败即终止。
func mustOpen(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(%s): %v", dir, err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestAppendSeqMonotonicAcrossThreads 全局 seq 跨线程/事件类型严格单调 +1。
func TestAppendSeqMonotonicAcrossThreads(t *testing.T) {
	s := mustOpen(t, t.TempDir())
	got := []int64{
		s.Append("th-a", "turn-1", TypePrefixTurn+"started", []byte(`{}`)),
		s.Append("th-b", "turn-2", TypePrefixTurn+"text_delta", []byte(`{}`)),
		s.Append("", "", TypePrefixHost+"turn_finished", []byte(`{}`)),
	}
	if got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("seq 序列 = %v, want [1 2 3](全局单调,不分线程)", got)
	}
	if s.MaxSeq() != 3 {
		t.Fatalf("MaxSeq = %d, want 3", s.MaxSeq())
	}
}

// TestReopenRestoresMaxSeq 重启后从文件尾恢复计数器,seq 绝不回退/重号。
func TestReopenRestoresMaxSeq(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir)
	last := int64(0)
	for i := 0; i < 5; i++ {
		last = s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"delta":"x"}`))
	}
	_ = s.Close()

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("重开: %v", err)
	}
	defer s2.Close()
	if s2.MaxSeq() != last {
		t.Fatalf("重启后 MaxSeq = %d, want %d(从尾行恢复)", s2.MaxSeq(), last)
	}
	if got := s2.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{}`)); got != last+1 {
		t.Fatalf("重启后首条 seq = %d, want %d", got, last+1)
	}
}

// TestRotateKeepsOneGeneration 超限轮转:保留一代(events-1.jsonl),
// 更老的删;minSeq 记账到新最老行,丢过的区间由 BeyondRetention 暴露。
func TestRotateKeepsOneGeneration(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir)
	// 阈值 200B:每行约 90B,写 3 条必触发一次轮转
	s.SetMaxBytes(200)
	for i := 0; i < 3; i++ {
		s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"delta":"line"}`))
	}
	if s.MaxSeq() != 3 {
		t.Fatalf("MaxSeq = %d, want 3", s.MaxSeq())
	}
	if _, err := os.Stat(filepath.Join(dir, RotatedName)); err != nil {
		t.Fatalf("轮转文件应存在: %v", err)
	}
	// 当前文件仍在写(轮转后继续追加)
	if got := s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{}`)); got != 4 {
		t.Fatalf("轮转后 seq = %d, want 4", got)
	}
	minSeq := s.MinSeq()
	if minSeq < 1 || minSeq > 4 {
		t.Fatalf("MinSeq = %d, 越界", minSeq)
	}
	// 手机停在轮转丢弃的区间(早于最老保留行)→ 必须要求 resync
	if !s.BeyondRetention(minSeq - 2) {
		t.Fatalf("after=%d 落在丢弃区间,应判定超窗(最老保留=%d)", minSeq-2, minSeq)
	}
	// 手机已收到最老保留行的前一帧 → 无缺口,可增量
	if s.BeyondRetention(minSeq - 1) {
		t.Fatalf("after=%d 恰好接上最老保留行,不应判定超窗", minSeq-1)
	}
	_ = s.Close()

	// 再开一次:恢复逻辑应同样认账(min/max 从两个文件恢复)
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("重开: %v", err)
	}
	defer s2.Close()
	if s2.MaxSeq() != 4 || s2.MinSeq() != minSeq {
		t.Fatalf("重开后 max/min = %d/%d, want 4/%d", s2.MaxSeq(), s2.MinSeq(), minSeq)
	}
}

// TestRotateDeletesOldGeneration 二次轮转:只留一代 —— events-1.jsonl 被覆盖,
// 不无限堆积。
func TestRotateDeletesOldGeneration(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir)
	s.SetMaxBytes(150)
	for i := 0; i < 8; i++ {
		s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"delta":"padding-line"}`))
	}
	if n := countTurnEvents(t, dir, "th-a"); n == 0 {
		t.Fatal("轮转后至少应保留最新一代的事件")
	}
	// 目录里只有当前文件 + 至多一代轮转件
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var jsonl int
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".jsonl" {
			jsonl++
		}
	}
	if jsonl > 2 {
		t.Fatalf("事件文件数 = %d, want <= 2(只保留一代)", jsonl)
	}
}

// countTurnEvents 直接读盘统计某线程的 turn 帧行数(绕过 Store,验证真实落盘)。
func countTurnEvents(t *testing.T, dir, threadID string) int {
	t.Helper()
	total := 0
	for _, name := range []string{RotatedName, FileName} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, line := range splitLines(raw) {
			var e Event
			if jsonOk(line, &e) && e.ThreadID == threadID &&
				len(e.EventType) > 5 && e.EventType[:5] == TypePrefixTurn {
				total++
			}
		}
	}
	return total
}

func splitLines(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i := 0; i < len(b); i++ {
		if b[i] == '\n' {
			if i > start {
				out = append(out, b[start:i])
			}
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}

func jsonOk(line []byte, out *Event) bool {
	return json.Unmarshal(line, out) == nil
}

// TestReplayByThreadAndAfterFilter 回放按线程隔离、after 过滤、升序。
func TestReplayByThreadAndAfterFilter(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir)
	// 交错写两个线程 + 一条主机事件
	s.Append("th-a", "turn-1", TypePrefixTurn+"started", []byte(`{"t":1}`))
	s.Append("th-b", "turn-2", TypePrefixTurn+"started", []byte(`{"t":2}`))
	s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"t":3}`))
	s.Append("", "", TypePrefixHost+"turn_finished", []byte(`{"t":4}`))
	s.Append("th-a", "turn-1", TypePrefixTurn+"completed", []byte(`{"t":5}`))

	// 只回放 th-a 的 turn 帧
	frames := s.ReplayTurnFrames("th-a", 0)
	if len(frames) != 3 {
		t.Fatalf("th-a 帧数 = %d, want 3", len(frames))
	}
	for i, e := range frames {
		if want := int64(i*2 + 1); e.Seq != want { // 1,3,5
			t.Fatalf("帧序 = %d, want %d(升序)", e.Seq, want)
		}
	}
	// after=3:只剩 seq 5
	frames = s.ReplayTurnFrames("th-a", 3)
	if len(frames) != 1 || frames[0].Seq != 5 {
		t.Fatalf("after=3 回放 = %+v, want 仅 seq 5", frames)
	}
	// 主机事件与 turn 帧互不串门
	if hs := s.ReplayHostEvents(0); len(hs) != 1 || hs[0].Seq != 4 {
		t.Fatalf("主机事件回放 = %+v, want 仅 seq 4", hs)
	}
	if s.ReplayTurnFrames("th-不存在", 0) != nil {
		t.Fatal("未知线程应回放为空")
	}
}

// TestCorruptLinesSkipped 损坏行读写两侧都跳过:恢复 max 不被半行骗到,
// 回放不因坏行中断。
func TestCorruptLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir)
	s.Append("th-a", "turn-1", TypePrefixTurn+"started", []byte(`{}`))
	s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{}`))
	_ = s.Close()
	// 追加两行垃圾:一行残缺 JSON,一行合法 JSON 但 seq 非法
	f, err := os.OpenFile(filepath.Join(dir, FileName), os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"seq":3,"thread_id":"th-a","paylo`) // 崩溃半行
	_, _ = f.WriteString("\n")
	_, _ = f.WriteString(`{"seq":-7,"thread_id":"th-a"}` + "\n") // 非法 seq
	_ = f.Close()

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("重开(含坏行): %v", err)
	}
	defer s2.Close()
	if s2.MaxSeq() != 2 {
		t.Fatalf("重启后 MaxSeq = %d, want 2(坏行不计)", s2.MaxSeq())
	}
	if got := s2.Append("th-a", "turn-1", TypePrefixTurn+"completed", []byte(`{}`)); got != 3 {
		t.Fatalf("坏行后新 seq = %d, want 3", got)
	}
	frames := s2.ReplayTurnFrames("th-a", 0)
	if len(frames) != 3 {
		t.Fatalf("回放帧数 = %d, want 3(坏行跳过、好行全回)", len(frames))
	}
}

// TestBeyondRetentionSemantics 超窗判定:从未轮转(minSeq=1)永不超窗;
// 手机有序号但库是空的(文件被删)必须 resync;nil 存储保守 resync。
func TestBeyondRetentionSemantics(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir)
	s.Append("th-a", "turn-1", TypePrefixTurn+"started", []byte(`{}`))
	if s.BeyondRetention(0) {
		t.Fatal("after=0(手机还没记过 seq)谈不上超窗")
	}
	if s.BeyondRetention(1) || s.BeyondRetention(99) {
		t.Fatal("从未轮转(minSeq=1)时任何 after 都不该判超窗")
	}
	_ = s.Close()

	// 清空事件库:手机拿着 seq=1 来续传,daemon 无法证明连续 → resync
	if err := os.Remove(filepath.Join(dir, FileName)); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if !s2.BeyondRetention(1) {
		t.Fatal("库为空而手机已持序号:应要求 resync")
	}
	if s2.BeyondRetention(0) {
		t.Fatal("after=0 始终不算超窗")
	}

	// nil 存储:无法续传(除 after=0 外一律 resync)
	var nilStore *Store
	if !nilStore.BeyondRetention(5) || nilStore.BeyondRetention(0) {
		t.Fatal("nil 存储的保守判定不对")
	}
	if got := nilStore.Append("th", "turn", "x", nil); got != 0 {
		t.Fatal("nil 存储应返回 0 且不崩")
	}
}

// TestMemoryModeWhenDirEmpty 未配置数据目录(dir="")= 纯内存模式:
// 计数器照常单调,不产生任何文件,回放为空。
func TestMemoryModeWhenDirEmpty(t *testing.T) {
	s := mustOpen(t, "")
	if got := s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{}`)); got != 1 {
		t.Fatalf("内存模式首条 seq = %d, want 1", got)
	}
	if s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{}`)) != 2 {
		t.Fatal("内存模式 seq 应继续单调")
	}
	if s.ReplayTurnFrames("th-a", 0) != nil {
		t.Fatal("内存模式没有回放数据")
	}
}

// TestAppendSurvivesUnwritableDir 目标目录不可写:Open 降级为内存模式,
// daemon 不崩(后续 Append 仍返回单调 seq)。
func TestAppendSurvivesUnwritableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 无视目录权限,跳过") // CI 若以 root 跑,该用例无意义
	}
	dir := filepath.Join(t.TempDir(), "sub")
	if err := os.MkdirAll(dir, 0o500); err != nil { // 只读目录
		t.Skip("无法构造只读目录")
	}
	s, err := Open(dir)
	if err != nil {
		t.Skipf("该文件系统未拦住只读目录: %v", err)
	}
	defer s.Close()
	for i := 0; i < 3; i++ {
		if got := s.Append("th-a", "turn-1", TypePrefixTurn+"text_delta", []byte(`{}`)); got != int64(i+1) {
			t.Fatalf("降级模式下 seq = %d, want %d(不崩、不回退)", got, i+1)
		}
	}
}
