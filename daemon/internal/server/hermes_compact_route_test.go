// hermes_compact_route_test.go —— CompactThread RPC 按 AgentID 路由到 hermes 的回归。
//
// 【为什么必须钉死】2026-10-14 P1 接线前,hermes 线程点「压缩上下文」会掉进
// codex 兜底:codexProvider 断言不上时报"没有可用的 codex"(hermes 用户一脸懵),
// 断言上了也只会撞"先发一条消息"的误导提示(hermes 线程没有 CodexThreadID)。
// 路由补 hermes 显式分支(与 pi 同形状的 *store.Thread 形,host.go 的
// piShapedCompactor 同款纪律)后,本文件钉死三件事:分发正确、provider 闸门
// 失败如实透传、绝不静默落 codex。
package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// hermesCompactStub 在 PiProviderStub(通用最小 provider 基座,四个方法与 pi
// 语义无关,只求满足 agent.Provider 接口)之上补 CompactThread —— 记录调用并按
// 注入值返回,不真拉 hermes 进程。路由按 store.Thread.AgentID 查 Providers 表,
// 不看 provider.ID(),基座继承的 ID()="pi" 不影响分发。
type hermesCompactStub struct {
	PiProviderStub
	calls int
	err   error
}

func (s *hermesCompactStub) CompactThread(*store.Thread) error {
	s.calls++
	return s.err
}

func hermesCompactThread(id string) *store.Thread {
	return &store.Thread{
		ID: id, AgentID: "hermes",
		ExternalSession: store.ExternalSessionRef{SessionID: "hermes-sess-" + id},
	}
}

// 路由分发:hermes 线程恰好命中 hermes 的 CompactThread 一次(与 pi 同形状断言)。
func TestCompactThreadRoutesToHermes(t *testing.T) {
	st := mustStore(t)
	th := hermesCompactThread("th-hermes-c1")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	comp := &hermesCompactStub{}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"hermes": comp}})
	if _, err := srv.CompactThread(context.Background(), &pb.CompactThreadRequest{ThreadId: th.ID}); err != nil {
		t.Fatalf("hermes 路径应成功: %v", err)
	}
	if comp.calls != 1 {
		t.Fatalf("CompactThread 应恰好被调一次,实际 %d", comp.calls)
	}
}

// provider 侧闸门拒绝(轮次在跑 TryLock 快拒 / 会话未落地 / 预算超时)必须如实
// 变成 gRPC Internal 且带 hermes 原因,不能假成功 —— 包装口径与 pi 路径逐字一致。
func TestCompactThreadPropagatesHermesFailure(t *testing.T) {
	st := mustStore(t)
	th := hermesCompactThread("th-hermes-c2")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	comp := &hermesCompactStub{err: errors.New("hermes: 轮次进行中,稍后再压")}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"hermes": comp}})
	_, err := srv.CompactThread(context.Background(), &pb.CompactThreadRequest{ThreadId: th.ID})
	if err == nil || !strings.Contains(err.Error(), "轮次进行中") {
		t.Fatalf("应透传 hermes 的失败原因: %v", err)
	}
}

// 病态形状(注册表里的 hermes 不具备压缩能力,如 stub/旧实现):显式报
// "没有可用的 hermes",绝不静默落 codex 兜底 —— codexCompactProbe 零调用是
// 本用例的判决点(掉兜底时会撞"先发一条消息"的误导提示,不含 "hermes")。
func TestCompactThreadHermesNeverFallsBackToCodex(t *testing.T) {
	st := mustStore(t)
	th := hermesCompactThread("th-hermes-c3")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	probe := &codexCompactProbe{}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{
		"codex":  probe,
		"hermes": &PiProviderStub{},
	}})
	_, err := srv.CompactThread(context.Background(), &pb.CompactThreadRequest{ThreadId: th.ID})
	if err == nil || !strings.Contains(err.Error(), "hermes") {
		t.Fatalf("应报 hermes 不可用: %v", err)
	}
	if len(probe.calls) != 0 {
		t.Fatalf("hermes 会话不得误路由到 codex 压缩通道, 实际调用 %v", probe.calls)
	}
}
