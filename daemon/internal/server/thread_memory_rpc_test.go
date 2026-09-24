package server

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// thread_memory_rpc_test.go —— ListThreadMemories / ClearThreadMemory 两条 RPC 的
// 契约回归(内存管理页面 [手机端] 的服务端)。
//
// 验收点:
//
//	① 清单:两个会话的记忆按字节降序回,合计逐项对得上;
//	② 未注册的 agent → InvalidArgument,文案里动态列出已注册名单(不写死);
//	③ 缺 agent_id → InvalidArgument;
//	④ 清除:成功回剩余合计;会话不存在回 NotFound;未注册 agent 回 InvalidArgument;
//	⑤ 清除后 GetAgentRuntime 的 memory 合计同步变小(观测口径与清单同源)。

func newThreadMemoryTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{
		Store:     st,
		Providers: map[string]agent.Provider{"echo": agent.NewEcho()},
		// 生产 daemon 用文件后端（internal/agent/runtime_descriptors.go:
		// NewFileBackend(dir)），清单的"最后 checkpoint 时刻"定义在文件 mtime 上
		// —— 测试装配必须同构,否则那条路径永远测不到。
		RuntimeBackend: runtime.NewFileBackend(t.TempDir()),
	})
	t.Cleanup(func() { _ = srv.events.Close() })
	return srv
}

// seedServerMemory 往后端写两个会话的记忆(th-a 4B、th-b 8B)并落盘。
func seedServerMemory(t *testing.T, srv *Server) {
	t.Helper()
	ctx := context.Background()
	for _, tc := range []struct {
		thread  string
		payload string
	}{
		{"th-a", "aa"},
		{"th-a", "bb"},
		{"th-b", "cccccccc"},
	} {
		key := runtime.MemoryKey{AgentID: "echo", ThreadID: tc.thread, Kind: runtime.MemoryConversation}
		if _, err := srv.runtimeBackend.Memory().Append(ctx, key, runtime.MemoryItem{Role: "user", Payload: []byte(tc.payload)}); err != nil {
			t.Fatalf("Append(%s/%s) 失败: %v", tc.thread, tc.payload, err)
		}
		if err := srv.runtimeBackend.Memory().Checkpoint(ctx, key); err != nil {
			t.Fatalf("Checkpoint(%s) 失败: %v", tc.thread, err)
		}
	}
}

func TestListThreadMemoriesRPC(t *testing.T) {
	srv := newThreadMemoryTestServer(t)
	seedServerMemory(t, srv)
	ctx := context.Background()

	resp, err := srv.ListThreadMemories(ctx, &pb.ListThreadMemoriesRequest{AgentId: "echo"})
	if err != nil {
		t.Fatalf("ListThreadMemories 失败: %v", err)
	}
	if len(resp.GetRows()) != 2 {
		t.Fatalf("应回 2 行,实得 %d: %+v", len(resp.GetRows()), resp.GetRows())
	}
	// 按字节降序:th-b(8B) 在前。
	if resp.GetRows()[0].GetThreadId() != "th-b" || resp.GetRows()[0].GetBytes() != 8 {
		t.Fatalf("第一行应是 th-b(8B),实得 %+v", resp.GetRows()[0])
	}
	if resp.GetRows()[0].GetEntries() != 1 || resp.GetRows()[0].GetSeq() != 1 {
		t.Fatalf("th-b 行应 1 条/seq=1,实得 %+v", resp.GetRows()[0])
	}
	if !resp.GetRows()[0].GetLoaded() {
		t.Fatal("本进程写过的会话应标 loaded=true")
	}
	if resp.GetRows()[0].GetLastCheckpointUnixMs() == 0 {
		t.Fatal("落过盘的会话应带最后 checkpoint 时刻")
	}
	if resp.GetTotalThreads() != 2 || resp.GetTotalEntries() != 3 || resp.GetTotalBytes() != 12 {
		t.Fatalf("合计应 2/3/12B,实得 %+v", resp)
	}
	if resp.GetSampledAtUnixMs() == 0 {
		t.Fatal("应带采样时刻")
	}
}

func TestListThreadMemoriesUnregisteredAgent(t *testing.T) {
	srv := newThreadMemoryTestServer(t)
	_, err := srv.ListThreadMemories(context.Background(), &pb.ListThreadMemoriesRequest{AgentId: "nosuch"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("未注册 agent 应 InvalidArgument,实得 %v", err)
	}
	// 文案动态列出已注册名单:注册表有谁就列谁,不写死。
	if !strings.Contains(err.Error(), "nosuch") || !strings.Contains(err.Error(), "echo") {
		t.Fatalf("错误文案应含请求的 id 与已注册名单,实得: %v", err)
	}

	// 缺 agent_id 同样回 InvalidArgument。
	_, err = srv.ListThreadMemories(context.Background(), &pb.ListThreadMemoriesRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("缺 agent_id 应 InvalidArgument,实得 %v", err)
	}
}

func TestClearThreadMemoryRPC(t *testing.T) {
	srv := newThreadMemoryTestServer(t)
	seedServerMemory(t, srv)
	ctx := context.Background()

	resp, err := srv.ClearThreadMemory(ctx, &pb.ClearThreadMemoryRequest{AgentId: "echo", ThreadId: "th-a"})
	if err != nil {
		t.Fatalf("ClearThreadMemory 失败: %v", err)
	}
	// 剩余合计:只剩 th-b。
	if resp.GetTotalThreads() != 1 || resp.GetTotalEntries() != 1 || resp.GetTotalBytes() != 8 {
		t.Fatalf("清除后剩余合计应 1/1/8B,实得 %+v", resp)
	}
	// 清除后再列:th-a 不在清单里。
	list, err := srv.ListThreadMemories(ctx, &pb.ListThreadMemoriesRequest{AgentId: "echo"})
	if err != nil {
		t.Fatalf("清除后 ListThreadMemories 失败: %v", err)
	}
	for _, r := range list.GetRows() {
		if r.GetThreadId() == "th-a" {
			t.Fatalf("清除后 th-a 不应还在清单里: %+v", list.GetRows())
		}
	}
	// 观测口径(GetAgentRuntime.memory)与清单同源地变小 —— 诊断页与管理页不能各说各话。
	obs, err := srv.GetAgentRuntime(ctx, &pb.GetAgentRuntimeRequest{AgentId: "echo"})
	if err != nil {
		t.Fatalf("GetAgentRuntime 失败: %v", err)
	}
	if obs.GetMemory().GetItems() != 1 || obs.GetMemory().GetThreads() != 1 {
		t.Fatalf("观测 memory 应同步为 1 会话/1 条,实得 %+v", obs.GetMemory())
	}

	// 不存在的会话 → NotFound。
	_, err = srv.ClearThreadMemory(ctx, &pb.ClearThreadMemoryRequest{AgentId: "echo", ThreadId: "th-absent"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("不存在的会话应 NotFound,实得 %v", err)
	}
	// 未注册的 agent → InvalidArgument。
	_, err = srv.ClearThreadMemory(ctx, &pb.ClearThreadMemoryRequest{AgentId: "nosuch", ThreadId: "th-a"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("未注册 agent 应 InvalidArgument,实得 %v", err)
	}
	// 缺 thread_id → InvalidArgument。
	_, err = srv.ClearThreadMemory(ctx, &pb.ClearThreadMemoryRequest{AgentId: "echo"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("缺 thread_id 应 InvalidArgument,实得 %v", err)
	}
}
