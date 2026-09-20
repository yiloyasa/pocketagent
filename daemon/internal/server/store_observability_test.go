// store_observability_test.go —— P2-4 / T-13 落盘失败埋点的回归用例。
//
// 钉住两件事:
//  1. 助手函数契约:格式固定(排障可 grep)、err == nil 完全静默(正常路径零噪音);
//  2. 调用点行为不变(只加观测):落盘失败时只记日志,流程不 panic、循环继续
//     处理后续条目 —— 以 mergeCodexEntries 为代表调用点,注入真实磁盘故障验证。
package server

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	"github.com/pocketagent/daemon/internal/store"
)

// captureStdlog 接管标准 log 输出,返回缓冲区;测试结束后自动还原。
func captureStdlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	oldWriter, oldFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
	})
	return &buf
}

func TestStoreUpdateErrMsgFormat(t *testing.T) {
	got := storeUpdateErrMsg("turn-start", "th_abc", errors.New("disk full"))
	want := "[store] 更新落盘失败 op=turn-start thread=th_abc err=disk full"
	if got != want {
		t.Fatalf("格式不符:\n got: %q\nwant: %q", got, want)
	}
}

func TestLogStoreUpdateErrSilentOnNil(t *testing.T) {
	buf := captureStdlog(t)
	logStoreUpdateErr("turn-start", "th_abc", nil)
	if buf.Len() != 0 {
		t.Fatalf("err == nil 不应打日志, got %q", buf.String())
	}
}

func TestLogStoreUpdateErrLogsOnErr(t *testing.T) {
	buf := captureStdlog(t)
	logStoreUpdateErr("codex-goal-sync", "th_xyz", errors.New("rename: 目标被占用"))
	out := buf.String()
	for _, want := range []string{"[store]", "op=codex-goal-sync", "thread=th_xyz", "rename: 目标被占用"} {
		if !strings.Contains(out, want) {
			t.Fatalf("日志缺 %q, got %q", want, out)
		}
	}
}

// TestMergeCodexEntriesContinuesOnStoreFailure 调用点回归:注入真实磁盘故障
// (store 数据目录被移除 → saveLocked 的 WriteFile 必然失败),断言:
//   - 不 panic,两条条目的循环都走完(第一条失败后继续处理第二条);
//   - 每条失败都留下带 op 与 thread ID 的埋点日志。
//
// 注:FileStore.Update 是先改内存索引、后落盘,所以失败后内存态可见新值 ——
// 这正是 P2-4 描述的"内存态已变而磁盘没写",埋点让它至少可诊断。
func TestMergeCodexEntriesContinuesOnStoreFailure(t *testing.T) {
	dir := t.TempDir()
	fs, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	s := &Server{cfg: Config{Store: fs}}
	for _, id := range []string{"th1", "th2"} {
		th := &store.Thread{ID: id, CodexThreadID: "cx-" + id, Title: "旧"}
		if err := fs.Create(th); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("注入磁盘故障失败: %v", err)
	}

	buf := captureStdlog(t)

	s.mergeCodexEntries([]agent.CodexThreadEntry{
		{ID: "cx-th1", Title: "新1"},
		{ID: "cx-th2", Title: "新2"},
	})

	out := buf.String()
	if !strings.Contains(out, "op=codex-list-sync") || !strings.Contains(out, "thread=th1") {
		t.Fatalf("第一条落盘失败应被埋点记录, got:\n%s", out)
	}
	if !strings.Contains(out, "thread=th2") {
		t.Fatalf("第一条失败后循环应继续处理第二条(th2 也应有埋点日志), got:\n%s", out)
	}
	// 不 panic 且流程走完:内存态照常推进(行为与埋点前一致)。
	got, ok := fs.Get("th2")
	if !ok || got.Title != "新2" {
		t.Fatalf("失败后循环应继续推进: ok=%v title=%q", ok, got.Title)
	}
}
