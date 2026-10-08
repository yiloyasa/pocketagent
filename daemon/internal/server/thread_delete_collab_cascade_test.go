package server

// thread_delete_collab_cascade_test.go —— 删除线程级联清理协同会话的回归钉子。
//
// 【背景】CollabSessionStore 此前没有任何删除入口:成员线程被用户删掉后,
// sessions.json 里的会话带着悬空的 member.ThreadID 永久残留,并经 ListSessions
// 一路透出到手机端。现由 DeleteThread → deleteCollabSessionsForThread 级联删除
// (会话元数据 + events/<id>.jsonl)。

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func TestDeleteThreadCascadesCollabSession(t *testing.T) {
	dir := t.TempDir()
	fs, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(Config{
		Store: fs,
		Providers: map[string]agent.Provider{"echo": &concurrencyProvider{run: func(context.Context, agent.TurnRequest, agent.Sink, agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
			return nil, &pb.Usage{}, nil
		}}},
		DataDir: dir,
	})
	t.Cleanup(func() { _ = srv.events.Close() })

	th := &store.Thread{ID: "th-member", Title: "成员线程", Status: pb.ThreadStatus_THREAD_STATUS_IDLE}
	if err := fs.Create(th); err != nil {
		t.Fatal(err)
	}

	// 经由独立实例向 collab/sessions.json 与事件文件落真值(DataDir 相同,
	// Server 侧协调器在删除时刻才惰性打开,能看到这里写入的内容)。
	csWriter, err := store.NewFileCollabSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := csWriter.CreateSession(ctx, store.CollabSession{ID: "cs-del", Status: store.CollabStatusRunning,
		Members: []store.CollabMember{{AgentID: "echo", ThreadID: "th-member", Status: store.CollabStatusRunning}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := csWriter.AppendEvent(ctx, store.CollabEvent{SessionID: "cs-del", Kind: "session.accepted"}); err != nil {
		t.Fatal(err)
	}
	// 无关会话:成员指向别的线程,不得误伤(同样落一条事件,让事件文件真实存在)。
	if err := csWriter.CreateSession(ctx, store.CollabSession{ID: "cs-keep", Status: store.CollabStatusRunning,
		Members: []store.CollabMember{{AgentID: "echo", ThreadID: "th-other", Status: store.CollabStatusRunning}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := csWriter.AppendEvent(ctx, store.CollabEvent{SessionID: "cs-keep", Kind: "session.accepted"}); err != nil {
		t.Fatal(err)
	}

	if _, err := srv.DeleteThread(ctx, &pb.DeleteThreadRequest{ThreadId: "th-member"}); err != nil {
		t.Fatalf("DeleteThread: %v", err)
	}

	// 重开一只有文件真值的实例断言(写侧实例的内存态不回读)。
	csReader, err := store.NewFileCollabSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := csReader.GetSession(ctx, "cs-del"); ok {
		t.Fatal("成员线程被删后,协同会话应被级联删除")
	}
	if _, ok := csReader.GetSession(ctx, "cs-keep"); !ok {
		t.Fatal("无关协同会话不得误伤")
	}
	if _, err := os.Stat(filepath.Join(dir, "collab", "events", "cs-del.jsonl")); !os.IsNotExist(err) {
		t.Fatal("被级联删除会话的事件文件应一并清除")
	}
	if _, err := os.Stat(filepath.Join(dir, "collab", "events", "cs-keep.jsonl")); err != nil {
		t.Fatalf("无关会话的事件文件应保留: %v", err)
	}
}

// TestDeleteSessionNotFoundSentinel FileCollabSessionStore.DeleteSession 对
// 不存在的会话必须报哨兵错误(调用方按 errors.Is 分型,不刮字符串)。
func TestDeleteSessionNotFoundSentinel(t *testing.T) {
	cs, err := store.NewFileCollabSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = cs.DeleteSession(context.Background(), "no-such")
	if err == nil {
		t.Fatal("删除不存在的会话应报错")
	}
	if !errors.Is(err, store.ErrCollabSessionNotFound) {
		t.Fatalf("应包 ErrCollabSessionNotFound,得到 %v", err)
	}
}
