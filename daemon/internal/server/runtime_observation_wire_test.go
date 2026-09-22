// runtime_observation_wire_test.go —— §9.5「每家都能显示 memory/cache/resource/
// checkpoint/recovery 状态」的 RPC 面回归用例。
//
// 【为什么与 runtime_state_test.go 分开】那个文件钉的是"形态申报与核验"（静态的、
// 启动时定的）；本文件钉的是"运行时观测有没有真的经 RPC 出去"（动态的、随使用变化）。
// 两件事的失效方式不同：前者漏了 → 面板少一块；后者漏了 → 面板在，但永远是 0。
package server

import (
	"context"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

func TestGetAgentRuntimeExposesObservations(t *testing.T) {
	srv := newRuntimeStateTestServer(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	ctx := context.Background()
	b := srv.runtimeBackend

	// 造真数据：echo 名下的记忆 + checkpoint + 缓存（各一条，另造一家的用于验证隔离）
	if _, err := b.Memory().Append(ctx,
		runtime.MemoryKey{AgentID: "echo", ThreadID: "th-x", Kind: runtime.MemoryConversation},
		runtime.MemoryItem{Role: "user", Payload: []byte("hello")}); err != nil {
		t.Fatalf("Append 失败: %v", err)
	}
	if _, err := b.Memory().Append(ctx,
		runtime.MemoryKey{AgentID: "codex", ThreadID: "th-y", Kind: runtime.MemoryConversation},
		runtime.MemoryItem{Role: "user", Payload: []byte("other")}); err != nil {
		t.Fatalf("Append 失败: %v", err)
	}
	if err := b.Checkpoints().Save(ctx,
		runtime.CheckpointKey{AgentID: "echo", ThreadID: "th-x", Name: "conversation"},
		[]byte("cp")); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	if err := b.Cache().Put(ctx,
		runtime.CacheKey{Namespace: runtime.Namespace("echo", runtime.NsModels), Key: "k"},
		runtime.CacheValue{Payload: []byte("{}")},
		runtime.CachePolicy{TTL: time.Minute, MaxEntries: 5}); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}

	resp, err := srv.GetAgentRuntime(ctx, &pb.GetAgentRuntimeRequest{AgentId: "echo"})
	if err != nil {
		t.Fatalf("GetAgentRuntime 失败: %v", err)
	}
	// memory：条目/分层/字节都要透出，且只算 echo 自己的
	if got := resp.GetMemory().GetItems(); got != 1 {
		t.Fatalf("echo 记忆条目应为 1（隔离口径），实得 %d", got)
	}
	if got := resp.GetMemory().GetKinds()["conversation"]; got != 1 {
		t.Fatalf("分层计数应含 conversation=1，实得 %v", resp.GetMemory().GetKinds())
	}
	if resp.GetMemory().GetBytes() <= 0 {
		t.Fatal("应带记忆字节数")
	}
	if got := resp.GetMemory().GetThreads(); got != 1 {
		t.Fatalf("应带涉及线程数 1，实得 %d", got)
	}
	// checkpoint / cache
	if got := resp.GetCheckpoint().GetCount(); got != 1 {
		t.Fatalf("echo checkpoint 应为 1，实得 %d", got)
	}
	if got := resp.GetCache().GetEntries(); got != 1 {
		t.Fatalf("echo 缓存条目应为 1，实得 %d", got)
	}

	// recovery：目前没有产出方 —— 必须如实报"未上报"，
	// 不得显示成"恢复失败"，也不得凭空造一个成功。
	rec := resp.GetRecovery()
	if rec.GetReported() {
		t.Fatal("没有产出方时不得声称上报过恢复状态")
	}
	if rec.GetFrom() != string(runtime.RecoveredNone) {
		t.Fatalf("未上报时来源应为 none，实得 %q", rec.GetFrom())
	}
}

// TestGetAgentRuntimeObservationIsZeroForUnusedAgent：完全没用过公共 Runtime 的
// agent 应显示全零，而不是报错或缺字段 —— 诊断页要能区分"没用上"与"查不到"。
func TestGetAgentRuntimeObservationIsZeroForUnusedAgent(t *testing.T) {
	srv := newRuntimeStateTestServer(t, map[string]agent.Provider{"echo": agent.NewEcho(), "codex": nil})
	resp, err := srv.GetAgentRuntime(context.Background(), &pb.GetAgentRuntimeRequest{AgentId: "codex"})
	if err != nil {
		t.Fatalf("GetAgentRuntime 失败: %v", err)
	}
	if resp.GetMemory().GetItems() != 0 || resp.GetCheckpoint().GetCount() != 0 || resp.GetCache().GetEntries() != 0 {
		t.Fatalf("未使用公共 Runtime 的 agent 应显示全零，实得 memory=%+v checkpoint=%+v cache=%+v",
			resp.GetMemory(), resp.GetCheckpoint(), resp.GetCache())
	}
	if resp.GetMemory() == nil || resp.GetCheckpoint() == nil || resp.GetRecovery() == nil {
		t.Fatal("字段必须存在（值为0），缺失会被客户端读成'该 agent 没这一项'")
	}
}
