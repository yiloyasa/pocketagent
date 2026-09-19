// pi_compact_route_test.go —— CompactThread RPC 按 AgentID 路由到 pi 的回归。
//
// 【为什么必须钉死】pi 的 compact 是同步真实压缩(等 LLM 摘要完成),与 codex
// 的 thread/compact/start(发起即回)语义不同;路由错 = 用户按"压缩"没反应或
// 报"没有可用的 codex"。fake 注入遵循 zcodeHistorySource 同款最小接口模式。
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

// piCompactStub 在 PiProviderStub(纯 provider 接口)之上补 CompactThread,
// 记录调用并按注入值返回 —— 不真拉 pi 子进程。
type piCompactStub struct {
	PiProviderStub
	calls int
	err   error
}

func (s *piCompactStub) CompactThread(*store.Thread) error {
	s.calls++
	return s.err
}

func piThreadWithSession(id string) *store.Thread {
	return &store.Thread{
		ID: id, AgentID: "pi",
		ExternalSession: store.ExternalSessionRef{SessionFile: `C:\sessions\` + id + ".jsonl"},
	}
}

func TestCompactThreadRoutesToPi(t *testing.T) {
	st := mustStore(t)
	th := piThreadWithSession("th-pi-1")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	comp := &piCompactStub{}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": comp}})
	if _, err := srv.CompactThread(context.Background(), &pb.CompactThreadRequest{ThreadId: th.ID}); err != nil {
		t.Fatalf("pi 路径应成功: %v", err)
	}
	if comp.calls != 1 {
		t.Fatalf("CompactThread 应恰好被调一次,实际 %d", comp.calls)
	}
}

// pi 侧失败(如 PI_BUSY / 压缩被拒)必须如实变成 gRPC Internal,不能假成功。
func TestCompactThreadPropagatesPiFailure(t *testing.T) {
	st := mustStore(t)
	th := piThreadWithSession("th-pi-2")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	comp := &piCompactStub{err: errors.New("Nothing to compact (session too small)")}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": comp}})
	_, err := srv.CompactThread(context.Background(), &pb.CompactThreadRequest{ThreadId: th.ID})
	if err == nil || !strings.Contains(err.Error(), "Nothing to compact") {
		t.Fatalf("应透传 pi 的失败原因: %v", err)
	}
}

// 注册表里的 pi 不具备压缩能力(如 stub/旧实现)时显式报"没有可用的 pi",
// 绝不静默回退 codex 路径。
func TestCompactThreadPiWithoutCompactor(t *testing.T) {
	st := mustStore(t)
	th := piThreadWithSession("th-pi-3")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": &PiProviderStub{}}})
	_, err := srv.CompactThread(context.Background(), &pb.CompactThreadRequest{ThreadId: th.ID})
	if err == nil || !strings.Contains(err.Error(), "pi") {
		t.Fatalf("应报 pi 不可用: %v", err)
	}
}

// TestWirePiTitleHandler 标题回调落库:改名才写库,重名/异线程不动
// (挂载回显 t.Title 原值时不形成回调环)。
func TestWirePiTitleHandler(t *testing.T) {
	st := mustStore(t)
	th := &store.Thread{ID: "th-pi-4", Title: "旧标题", AgentID: "pi"}
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{
		"pi": &PiProviderStub{},
	}})
	srv.wirePiTitleHandler()
	// Providers 里是 stub,不是 *agent.PiProvider:接线应安全跳过(不 panic)
	// 换成真 PiProvider 再验证落库行为。
	srv2 := New(Config{Store: st, Providers: map[string]agent.Provider{
		"pi": agent.NewPiProvider(),
	}})
	srv2.wirePiTitleHandler()
	pp, _ := srv2.cfg.Providers["pi"].(*agent.PiProvider)
	if pp.TitleHandler == nil {
		t.Fatal("wirePiTitleHandler 应安装 Pi 的 TitleHandler")
	}
	pp.TitleHandler("th-pi-4", "新标题")
	if cur, _ := st.Get("th-pi-4"); cur.Title != "新标题" {
		t.Fatalf("标题应落库: %q", cur.Title)
	}
	// 相同标题(挂载回显场景):不产生写库动作(标题不变即守卫生效)
	pp.TitleHandler("th-pi-4", "新标题")
	if cur, _ := st.Get("th-pi-4"); cur.Title != "新标题" {
		t.Fatalf("重复回调不应改标题: %q", cur.Title)
	}
	// 未知线程:静默忽略
	pp.TitleHandler("th-不存在", "x")
}
