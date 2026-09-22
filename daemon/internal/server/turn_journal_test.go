package server

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/store"
)

// 记录后可读回,且 preview 截断到 120 个字符(按 rune,中文按字)。
func TestTurnJournalRecordAndReadBack(t *testing.T) {
	j := newTurnJournal("")
	long := strings.Repeat("字", journalPreviewLimit+10) // 超出上限 10 字
	j.Record("q1", "thread-1", long)

	got := j.Pending()
	if len(got) != 1 {
		t.Fatalf("应读回 1 条,得到 %d", len(got))
	}
	if got[0].ID != "q1" || got[0].ThreadID != "thread-1" {
		t.Fatalf("id/线程不匹配: %+v", got[0])
	}
	if n := len([]rune(got[0].Preview)); n != journalPreviewLimit {
		t.Fatalf("preview 应截断到 %d 字符,得到 %d", journalPreviewLimit, n)
	}
	if got[0].CreatedAt == 0 {
		t.Fatalf("created_at 应被写入")
	}
}

// 换实例(模拟 daemon 重启)能从同一文件读回未完成条目。
func TestTurnJournalSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	j1 := newTurnJournal(dir)
	j1.Record("q1", "thread-1", "第一句")
	j1.Record("q2", "thread-2", "第二句")

	// 模拟重启:新实例重读同一 turn-journal.json
	j2 := newTurnJournal(dir)
	got := j2.Pending()
	if len(got) != 2 {
		t.Fatalf("重启后应读回 2 条,得到 %d", len(got))
	}
	if got[0].ID != "q1" || got[1].ID != "q2" {
		t.Fatalf("应保持入账顺序: %+v", got)
	}

	// 落盘内容是合法 JSON(不许写成半截)。
	raw, err := os.ReadFile(filepath.Join(dir, "turn-journal.json"))
	if err != nil {
		t.Fatalf("读台账文件: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("turn-journal.json 应是合法 JSON: %q", raw)
	}
}

// 出队(Remove)后条目被移除、不残留,且移除落盘。
func TestTurnJournalRemoveClearsEntry(t *testing.T) {
	dir := t.TempDir()
	j := newTurnJournal(dir)
	j.Record("q1", "thread-1", "a")
	j.Record("q2", "thread-1", "b")
	j.Remove("q1")

	if got := j.Pending(); len(got) != 1 || got[0].ID != "q2" {
		t.Fatalf("q1 应被移除、只剩 q2: %+v", got)
	}
	// 换实例:移除也必须落盘,重启后不复活。
	if got := newTurnJournal(dir).Pending(); len(got) != 1 || got[0].ID != "q2" {
		t.Fatalf("移除应落盘,重启后只剩 q2: %+v", got)
	}
}

// 空 dataDir 退化为纯内存:能读写,但不落盘、不跨实例共享。
func TestTurnJournalMemoryOnlyWithoutDataDir(t *testing.T) {
	j := newTurnJournal("")
	if j.path != "" {
		t.Fatalf("空 dataDir 不应有落盘路径,得到 %q", j.path)
	}
	j.Record("q1", "thread-1", "x")
	if got := j.Pending(); len(got) != 1 {
		t.Fatalf("内存模式也应能读回,得到 %+v", got)
	}
	// 另一个空实例读不到任何东西 —— 证明没有落盘共享。
	if got := newTurnJournal("").Pending(); len(got) != 0 {
		t.Fatalf("空 dataDir 不应跨实例共享(未落盘),得到 %+v", got)
	}
}

// 损坏的台账文件 fail-soft:不 panic、不阻塞启动,告警后按空台账继续,
// 且后续仍可正常记账。(与身份/设备表的 fail-closed 形成对照,理由见文件头。)
func TestTurnJournalCorruptFileFailSoft(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "turn-journal.json")
	if err := os.WriteFile(badPath, []byte("{broken"), 0o600); err != nil {
		t.Fatalf("写坏文件: %v", err)
	}

	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	j := newTurnJournal(dir) // 不得 panic
	log.SetOutput(prev)

	if got := j.Pending(); len(got) != 0 {
		t.Fatalf("损坏台账应按空处理,得到 %+v", got)
	}
	if !strings.Contains(buf.String(), "[turnjournal]") {
		t.Fatalf("损坏台账应告警,日志: %q", buf.String())
	}
	// 坏文件不阻塞后续使用。
	j.Record("q1", "thread-1", "still works")
	if got := j.Pending(); len(got) != 1 {
		t.Fatalf("损坏后仍应能记入,得到 %+v", got)
	}
}

// 【关键一条】模拟重启:队列本身是空的(证明没有自动重放),但台账在启动报告里
// 被读出(日志含汇总 + 逐条明细),报告后台账被清空(不留僵尸条目)。
func TestTurnJournalStartupReportNoReplay(t *testing.T) {
	dir := t.TempDir()
	// 模拟"重启前"留下的未完成台账。
	seed := newTurnJournal(dir)
	seed.Record("q1", "thread-1", "重启前排队的第一句")
	seed.Record("q2", "thread-2", "重启前排队的第二句")

	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}

	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	srv := New(Config{Store: st, DataDir: dir})
	log.SetOutput(prev)
	// New 会打开 events.jsonl,Windows 上不关文件句柄会让 TempDir 清理失败。
	t.Cleanup(func() { _ = srv.events.Close() })

	// 证明没有自动重放:两个线程的队列都是空的。
	if n := srv.turnQueue.Len("thread-1"); n != 0 {
		t.Fatalf("重启不应自动重放,thread-1 队列应为空,得到 %d", n)
	}
	if n := srv.turnQueue.Len("thread-2"); n != 0 {
		t.Fatalf("重启不应自动重放,thread-2 队列应为空,得到 %d", n)
	}

	// 证明启动报告读出了台账:汇总 + 两条明细都在日志里。
	out := buf.String()
	for _, want := range []string{
		"[turnjournal]", "2 条排队输入", "thread-1", "thread-2", "重启前排队的第一句",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("启动报告应含 %q,实际日志:\n%s", want, out)
		}
	}

	// 报告后台账被清空(不保留成僵尸条目)。
	if got := srv.turnJournal.Pending(); len(got) != 0 {
		t.Fatalf("启动报告后台账应清空,得到 %+v", got)
	}
}

// 出队接线:turningRuntime.next 出队时把对应台账条目划掉(不残留)。
func TestTurnJournalNextRemovesEntry(t *testing.T) {
	j := newTurnJournal("")
	q := newTurnInputQueue(j)
	// 模拟 handleMidTurnInput:先记账,再入队(队列与台账共用同一 id)。
	j.Record("q1", "thread-1", "x")
	if pos, _ := q.Enqueue("thread-1", queuedInput{id: "q1"}); pos == 0 {
		t.Fatalf("入队失败")
	}

	rt := &turnRuntime{}
	if _, ok := rt.next(q, "thread-1"); !ok {
		t.Fatalf("应能出队")
	}
	if got := j.Pending(); len(got) != 0 {
		t.Fatalf("出队开始执行后,台账条目应被移除,得到 %+v", got)
	}
	if n := q.Len("thread-1"); n != 0 {
		t.Fatalf("队列应空,得到 %d", n)
	}
}

// 取消/异常收尾清队时,被丢弃的排队输入要一并划账 —— 它们是"被明确丢弃"的,
// 不该在下次重启被误报成"随重启丢失"。
func TestTurnJournalStopForgetsClearedEntries(t *testing.T) {
	j := newTurnJournal("")
	q := newTurnInputQueue(j)
	j.Record("q1", "thread-1", "a")
	j.Record("q2", "thread-1", "b")
	if pos, _ := q.Enqueue("thread-1", queuedInput{id: "q1"}); pos == 0 {
		t.Fatalf("入队 q1 失败")
	}
	if pos, _ := q.Enqueue("thread-1", queuedInput{id: "q2"}); pos == 0 {
		t.Fatalf("入队 q2 失败")
	}

	rt := &turnRuntime{}
	rt.stop(q, "thread-1")
	if got := j.Pending(); len(got) != 0 {
		t.Fatalf("清队后被丢弃的条目应一并划账,得到 %+v", got)
	}
	if n := q.Len("thread-1"); n != 0 {
		t.Fatalf("队列应空,得到 %d", n)
	}
}
