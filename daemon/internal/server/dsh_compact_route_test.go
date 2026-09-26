// dsh_compact_route_test.go —— CompactThread 对 dsh 会话的显式拒绝(EB 门控审计修复)。
//
// 【为什么必须钉死】CompactThread 路由是「pi 显式分支 + zcode 显式分支 + codex
// 兜底」,dsh 会话此前掉进 codex 兜底:codexProvider 断言不上时报"电脑上没有
// 可用的 codex"(dsh 用户一脸懵);断言上了也只会撞"先发一条消息"的误导提示
// (dsh 线程永远不会有 CodexThreadID)。dsh provider 没有任何压缩实现
// (dsh_*.go 仅 autoCompactionEnabled 配置项,与"压缩上下文"RPC 无关),
// 按 ClearQueuedInputs / SetThreadGoal 的先例显式 FailedPrecondition,不静默换人。

package server

import (
	"context"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// codexCompactProbe codex 形状(thread/compact/start 的 string 形)的探针替身:
// 记录是否被误调 —— dsh 会话绝不允许把压缩指令发到 codex 通道。
type codexCompactProbe struct {
	calls []string
}

func (p *codexCompactProbe) ID() string          { return "codex" }
func (p *codexCompactProbe) DisplayName() string { return "codex 压缩探针" }
func (p *codexCompactProbe) Models() []pb.ModelInfo {
	return nil
}
func (p *codexCompactProbe) Run(ctx context.Context, req agent.TurnRequest, sink agent.Sink, approve agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, nil
}
func (p *codexCompactProbe) CompactThread(codexTID string) error {
	p.calls = append(p.calls, codexTID)
	return nil
}

func dshCompactThread(id, strayCodexTID string) *store.Thread {
	return &store.Thread{
		ID: id, AgentID: "dsh", CodexThreadID: strayCodexTID,
		ExternalSession: store.ExternalSessionRef{SessionID: "dsh-sess-" + id},
	}
}

// 真实形状:dsh 会话(CodexThreadID 恒空)点「压缩上下文」必须显式报"不支持",
// 而不是掉进 codex 兜底给出误导性错误。
func TestCompactThreadRejectsDshThread(t *testing.T) {
	st := mustStore(t)
	th := dshCompactThread("th-dsh-c1", "")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	probe := &codexCompactProbe{}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"codex": probe}})
	_, err := srv.CompactThread(context.Background(), &pb.CompactThreadRequest{ThreadId: th.ID})
	if status.Code(err) != codes.FailedPrecondition || !strings.Contains(err.Error(), "不支持会话压缩") {
		t.Fatalf("dsh 会话应显式 FailedPrecondition「不支持会话压缩」,得到 %v", err)
	}
	if len(probe.calls) != 0 {
		t.Fatalf("dsh 会话不得误路由到 codex 压缩通道, 实际调用 %v", probe.calls)
	}
}

// 反证(病态形状):万一旧库/未来改动让 dsh 线程带上 codexTID,也不得把压缩
// 指令发给 codex —— 归属判断看 AgentID,不看有没有顺手可用的通道。
func TestCompactThreadRejectsDshThreadEvenWithStrayCodexTID(t *testing.T) {
	st := mustStore(t)
	th := dshCompactThread("th-dsh-c2", "codex-tid-xyz")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	probe := &codexCompactProbe{}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"codex": probe}})
	_, err := srv.CompactThread(context.Background(), &pb.CompactThreadRequest{ThreadId: th.ID})
	if status.Code(err) != codes.FailedPrecondition || !strings.Contains(err.Error(), "不支持会话压缩") {
		t.Fatalf("带杂散 codexTID 的 dsh 会话同样应显式拒绝,得到 %v", err)
	}
	if len(probe.calls) != 0 {
		t.Fatalf("杂散 codexTID 不得把 dsh 压缩误路由到 codex, 实际调用 %v", probe.calls)
	}
}
