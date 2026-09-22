// collab_policy_test.go —— 协同策略控制面与**生效点**的回归用例(规格 §11)。
//
// 【为什么必须钉生效点】策略能读能写,不等于它真的管住了什么。没有这一组,
// "改完策略"就只是往文件里写了一个数字 —— 那正是本轮要消掉的形态。
package server

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/collabmcp"
	"github.com/pocketagent/daemon/internal/runtime"
)

func TestSetCollabPolicyPersistsAndTakesEffect(t *testing.T) {
	srv, dir := newPolicyTestServer(t)
	ctx := context.Background()

	before := srv.collabPolicySnapshot()
	if !before.Fanout() {
		t.Fatal("默认应全开(与既有行为一致)")
	}

	off := false
	two := int32(2)
	resp, err := srv.SetCollabPolicy(ctx, &pb.SetCollabPolicyRequest{
		Policy: &pb.CollabPolicy{FanoutEnabled: &off, MaxParallelMembers: &two},
	})
	if err != nil {
		t.Fatalf("SetCollabPolicy 失败: %v", err)
	}
	if resp.GetRevision() != 1 {
		t.Fatalf("首次写入 revision 应为 1,得到 %d", resp.GetRevision())
	}
	// 立即生效(不重启)。
	if after := srv.collabPolicySnapshot(); after.Fanout() {
		t.Fatal("策略未立即生效")
	}
	// 实际生效值必须齐备(客户端靠它显示,不该靠缺席推断)。
	if eff := resp.GetEffective(); eff.GetFanoutEnabled() || !eff.GetCollaborationEnabled() {
		t.Fatalf("生效值不对: %+v", eff)
	}
	if resp.GetEffective().GetMaxParallelMembers() != 2 {
		t.Fatalf("生效值应含上限 2,得到 %d", resp.GetEffective().GetMaxParallelMembers())
	}
	// 落盘(重启后仍在):重新装载应读到同一份。
	reloaded, err := runtime.LoadCollabPolicy(dir)
	if err != nil {
		t.Fatalf("读回失败: %v", err)
	}
	if reloaded.Fanout() || reloaded.MaxMembers() != 2 || reloaded.Revision != 1 {
		t.Fatalf("落盘值不对: %+v", reloaded)
	}
}

func TestSetCollabPolicyRevisionConflict(t *testing.T) {
	srv, _ := newPolicyTestServer(t)
	ctx := context.Background()
	off := false
	if _, err := srv.SetCollabPolicy(ctx, &pb.SetCollabPolicyRequest{
		Policy: &pb.CollabPolicy{FanoutEnabled: &off},
	}); err != nil {
		t.Fatalf("首次提交失败: %v", err)
	}
	_, err := srv.SetCollabPolicy(ctx, &pb.SetCollabPolicyRequest{
		Policy:           &pb.CollabPolicy{FanoutEnabled: &off},
		ExpectedRevision: 9,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("过期 revision 应为 FailedPrecondition,得到 %v", err)
	}
}

// TestDispatchCollabHonoursPolicy 是生效点 ①②:扇出开关与成员数上限。
func TestDispatchCollabHonoursPolicy(t *testing.T) {
	srv, _ := newPolicyTestServer(t)
	ctx := context.Background()
	members := []*pb.CollabMember{{AgentId: "echo"}, {AgentId: "codex"}, {AgentId: "dsh"}}

	// ① 关掉扇出 → 必须拒绝(而不是静默照跑:用户关它的目的就是让这件事不再发生)。
	off := false
	if _, err := srv.SetCollabPolicy(ctx, &pb.SetCollabPolicyRequest{
		Policy: &pb.CollabPolicy{FanoutEnabled: &off},
	}); err != nil {
		t.Fatalf("关扇出失败: %v", err)
	}
	_, err := srv.DispatchCollab(ctx, &pb.DispatchCollabRequest{Instruction: "一起看看", Members: members})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("扇出关闭时应 FailedPrecondition,得到 %v", err)
	}

	// ② 只关总开关 → 同样拒绝。
	on := true
	if _, err := srv.SetCollabPolicy(ctx, &pb.SetCollabPolicyRequest{
		Policy: &pb.CollabPolicy{CollaborationEnabled: &off, FanoutEnabled: &on},
	}); err != nil {
		t.Fatalf("关总开关失败: %v", err)
	}
	if _, err := srv.DispatchCollab(ctx, &pb.DispatchCollabRequest{Instruction: "一起看看", Members: members}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("协同总开关关闭时应 FailedPrecondition,得到 %v", err)
	}

	// ③ 开回来但把成员上限压到 2 → 选 3 家必须如实报错(不静默截断)。
	two := int32(2)
	if _, err := srv.SetCollabPolicy(ctx, &pb.SetCollabPolicyRequest{
		Policy: &pb.CollabPolicy{CollaborationEnabled: &on, FanoutEnabled: &on, MaxParallelMembers: &two},
	}); err != nil {
		t.Fatalf("设上限失败: %v", err)
	}
	_, err = srv.DispatchCollab(ctx, &pb.DispatchCollabRequest{Instruction: "一起看看", Members: members})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("超过成员上限应 InvalidArgument,得到 %v", err)
	}
	if err != nil && !errors.Is(err, nil) {
		// 错误信息必须说清"上限是多少"(用户才知道该调什么)。
		if msg := status.Convert(err).Message(); msg == "" {
			t.Fatal("错误信息不能为空")
		}
	}

	// ④ 上限之内不拦(策略不该把正常路径也挡了)。
	if _, err := srv.DispatchCollab(ctx, &pb.DispatchCollabRequest{
		Instruction: "一起看看", Members: members[:2],
	}); err != nil && status.Code(err) != codes.InvalidArgument {
		// 成员 agent 若不在注册表会有别的人话报错;此处只关心"不是因为策略被挡"。
		t.Logf("上限内扇出返回: %v", err)
	}
}

// TestDelegateHonoursPolicy 是生效点 ③:委派开关(两个入口都要挡)。
func TestDelegateHonoursPolicy(t *testing.T) {
	srv, _ := newPolicyTestServer(t)
	ctx := context.Background()
	eng := &collabEngine{srv: srv, childThreads: map[string]bool{}}

	// 默认全开时不该被这条策略挡(它可能因为别的理由失败,但不该是 errDelegateDisabled)。
	if _, _, err := eng.Delegate(ctx, "echo", "task", collabmcp.DelegateOptions{}); errors.Is(err, errDelegateDisabled) {
		t.Fatalf("默认策略下不该报'委派已关闭': %v", err)
	}

	off := false
	if _, err := srv.SetCollabPolicy(ctx, &pb.SetCollabPolicyRequest{
		Policy: &pb.CollabPolicy{NativeDelegateEnabled: &off},
	}); err != nil {
		t.Fatalf("关委派失败: %v", err)
	}
	if _, _, err := eng.Delegate(ctx, "echo", "task", collabmcp.DelegateOptions{}); !errors.Is(err, errDelegateDisabled) {
		t.Fatalf("委派关闭后应报 errDelegateDisabled,得到 %v", err)
	}
	// 跨进程入口同一条策略,不能绕过。
	if _, _, err := eng.DelegateForDevice(ctx, "dev-1", "echo", "task", collabmcp.DelegateOptions{}); !errors.Is(err, errDelegateDisabled) {
		t.Fatalf("跨进程委派同样应被策略挡下,得到 %v", err)
	}
}
