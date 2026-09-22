package server

// thread_delete_children_test.go —— 删除主会话时"孤儿子代理存根"的连带清理。
//
// 【为什么钉】删掉主会话后,它派出过的子代理存根(pi-sub-* / codex 存根)会变成
// 孤儿:列表里已无父会话可归属,条目却还赖着 —— 用户视角是"删了会话,子代理还在"。
// 同时必须钉住**不误伤**:判定依据只能是 ParentThreadID 这个结构性事实,
// 不能按 id 前缀猜(否则将来换命名就会漏,或误删用户自己建的会话)。

import (
	"context"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func TestDeleteThreadRemovesChildStubs(t *testing.T) {
	s := newTestServer(t)
	mk := func(id, parent, agent string) {
		t.Helper()
		if err := s.cfg.Store.Create(&store.Thread{
			ID: id, Title: id, AgentID: agent, ParentThreadID: parent,
			Status: pb.ThreadStatus_THREAD_STATUS_IDLE,
		}); err != nil {
			t.Fatal(err)
		}
	}
	mk("th-p", "", "pi")             // 主会话
	mk("pi-sub-a", "th-p", "pi")     // 它的子代理存根
	mk("pi-sub-b", "th-p", "pi")     // 同上
	mk("th-other", "", "codex")      // 无关的主会话
	mk("pi-sub-c", "th-other", "pi") // 挂在别人下面的存根

	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th-p"}); err != nil {
		t.Fatalf("删除主会话失败: %v", err)
	}

	for _, gone := range []string{"th-p", "pi-sub-a", "pi-sub-b"} {
		if _, ok := s.cfg.Store.Get(gone); ok {
			t.Fatalf("%s 应已被删除(主会话或其子代理存根)", gone)
		}
	}
	for _, keep := range []string{"th-other", "pi-sub-c"} {
		if _, ok := s.cfg.Store.Get(keep); !ok {
			t.Fatalf("%s 不该被误删(父链不指向被删会话)", keep)
		}
	}
}

// 没有子存根时删除照常成功(零副作用路径)。
func TestDeleteThreadWithoutChildren(t *testing.T) {
	s := newTestServer(t)
	if err := s.cfg.Store.Create(&store.Thread{
		ID: "th-solo", Title: "solo", AgentID: "pi",
		Status: pb.ThreadStatus_THREAD_STATUS_IDLE,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th-solo"}); err != nil {
		t.Fatalf("无子存根时删除应成功: %v", err)
	}
	if _, ok := s.cfg.Store.Get("th-solo"); ok {
		t.Fatal("会话应已删除")
	}
}
