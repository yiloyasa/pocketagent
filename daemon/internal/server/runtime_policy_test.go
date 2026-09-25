// runtime_policy_test.go —— 预算策略控制面(规格 §6.5 / §14)的回归用例。
//
// 验收点:
//
//	① 提交策略 → 落盘 + **立即生效**(下一次 Effective 就是新值,不必重启 daemon);
//	② revision 递增并跨重启成立(CAS 的基准);
//	③ 只改一项不清零其它项(逐字段覆盖,不是整块清空);
//	④ 过期 revision 提交 → 冲突,不静默覆盖;
//	⑤ 负额度 → 拒绝,且不落盘;
//	⑥ 无数据目录 → 如实失败,不改内存态(不让用户以为生效了);
//	⑦ 轮次强制点:租约的 soft/hard 取自 Effective,且**改策略后下一轮立刻用新额度**。
package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

func newPolicyTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{
		Store:          st,
		DataDir:        dir,
		BudgetPolicy:   runtime.BudgetPolicy{Global: runtime.DefaultLimits()},
		RuntimeBackend: runtime.NewBackend(),
	})
	t.Cleanup(func() { _ = srv.events.Close() })
	return srv, dir
}

// TestSetRuntimePolicyPersistsAndTakesEffect 覆盖验收点 ①②③。
func TestSetRuntimePolicyPersistsAndTakesEffect(t *testing.T) {
	srv, dir := newPolicyTestServer(t)
	ctx := context.Background()

	quota := int64(123456)
	resp, err := srv.SetRuntimePolicy(ctx, &pb.SetRuntimePolicyRequest{
		Policy: &pb.RuntimePolicy{Global: &pb.RuntimeLimits{RamEventBufferBytes: &quota}},
	})
	if err != nil {
		t.Fatalf("SetRuntimePolicy 失败: %v", err)
	}
	if resp.GetRevision() != 1 {
		t.Fatalf("首次写入 revision 应为 1,得到 %d", resp.GetRevision())
	}
	if resp.GetPolicyPath() == "" {
		t.Fatal("应回报落盘位置(用户要手改时得知道改哪儿)")
	}
	// 【实测踩到过的坑】提交的策略只含一项时,`policy` 里其余字段是缺席的(线上为 0),
	// 客户端拿它当"生效值"就会把未设置的项显示成"封禁"。生效值必须单独给出,且打底内置基线。
	if eff := resp.GetEffectiveGlobal(); eff.GetMaxParallelTurns() != 3 || eff.GetRamWorkingBytes() <= 0 {
		t.Fatalf("实际生效值未打底内置基线(客户端会误读成封禁): %+v", eff)
	}

	// ① 立即生效:同一进程内的下一次读取就是新值。
	got := srv.effectiveLimits("", "", "", "")
	if got.RAMEventBufferBytes == nil || *got.RAMEventBufferBytes != quota {
		t.Fatalf("策略未立即生效: %v", got.RAMEventBufferBytes)
	}

	// ② 落盘:重启(重新装载)后读回同一个值与同一个 revision。
	reloaded, err := runtime.LoadBudgetPolicy(dir)
	if err != nil {
		t.Fatalf("读回策略失败: %v", err)
	}
	if v := reloaded.Effective("", "", "", "").RAMEventBufferBytes; v == nil || *v != quota {
		t.Fatalf("落盘值不对: %v", v)
	}
	if reloaded.Revision != 1 {
		t.Fatalf("revision 应随文件一起落盘(CAS 基准要跨重启成立),得到 %d", reloaded.Revision)
	}

	// ③ 只改一项不该清零其它项 —— 未显式设置的字段沿用内置基线。
	if v := reloaded.Effective("", "", "", "").MaxParallelTurns; v == nil || *v != 3 {
		t.Fatalf("只改一项却把其它项弄丢了: max_parallel_turns=%v", v)
	}

	if _, err := os.Stat(filepath.Join(dir, "runtime-policy.json")); err != nil {
		t.Fatalf("策略文件未落盘: %v", err)
	}
}

// TestSetRuntimePolicyRevisionConflict 覆盖验收点 ④。
func TestSetRuntimePolicyRevisionConflict(t *testing.T) {
	srv, _ := newPolicyTestServer(t)
	ctx := context.Background()
	q := int64(111)
	if _, err := srv.SetRuntimePolicy(ctx, &pb.SetRuntimePolicyRequest{
		Policy: &pb.RuntimePolicy{Global: &pb.RuntimeLimits{RamEventBufferBytes: &q}},
	}); err != nil {
		t.Fatalf("首次提交失败: %v", err)
	}

	_, err := srv.SetRuntimePolicy(ctx, &pb.SetRuntimePolicyRequest{
		Policy:           &pb.RuntimePolicy{Global: &pb.RuntimeLimits{RamEventBufferBytes: &q}},
		ExpectedRevision: 7, // 过期
	})
	if err == nil {
		t.Fatal("过期 revision 必须冲突,不能静默覆盖别人的修改")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("应为 FailedPrecondition,得到 %v", err)
	}

	// 带上正确的 revision 就能提交(revision=1 → 新值 2)。
	if _, err := srv.SetRuntimePolicy(ctx, &pb.SetRuntimePolicyRequest{
		Policy:           &pb.RuntimePolicy{Global: &pb.RuntimeLimits{RamEventBufferBytes: &q}},
		ExpectedRevision: 1,
	}); err != nil {
		t.Fatalf("正确 revision 应放行: %v", err)
	}
}

// TestSetRuntimePolicyRejectsNegativeQuota 覆盖验收点 ⑤。
func TestSetRuntimePolicyRejectsNegativeQuota(t *testing.T) {
	srv, dir := newPolicyTestServer(t)
	neg := int64(-1)
	_, err := srv.SetRuntimePolicy(context.Background(), &pb.SetRuntimePolicyRequest{
		Policy: &pb.RuntimePolicy{Global: &pb.RuntimeLimits{RamEventBufferBytes: &neg}},
	})
	if err == nil {
		t.Fatal("负额度必须被拒 —— 它在下游会被当成'未设置',表现为设了不生效")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("应为 InvalidArgument,得到 %v", err)
	}
	if _, serr := os.Stat(filepath.Join(dir, "runtime-policy.json")); serr == nil {
		t.Fatal("被拒的提交不该落盘")
	}
}

// TestSetRuntimePolicyWithoutDataDirFailsLoudly 覆盖验收点 ⑥。
func TestSetRuntimePolicyWithoutDataDirFailsLoudly(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{Store: st}) // 无 DataDir(内存模式)
	t.Cleanup(func() { _ = srv.events.Close() })

	q := int64(1024)
	_, err = srv.SetRuntimePolicy(context.Background(), &pb.SetRuntimePolicyRequest{
		Policy: &pb.RuntimePolicy{Global: &pb.RuntimeLimits{RamEventBufferBytes: &q}},
	})
	if err == nil {
		t.Fatal("存不下来时必须如实失败 —— 回'成功'会让用户以为策略生效了")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("应为 Internal,得到 %v", err)
	}
	if v := srv.effectiveLimits("", "", "", "").RAMEventBufferBytes; v != nil && *v == q {
		t.Fatal("失败时不该改动内存态(否则重启后凭空变回去)")
	}
}

// TestGetRuntimePolicyReturnsCurrent 读回的就是当前生效值(手机端据此显示)。
func TestGetRuntimePolicyReturnsCurrent(t *testing.T) {
	srv, _ := newPolicyTestServer(t)
	ctx := context.Background()

	before, err := srv.GetRuntimePolicy(ctx, &pb.GetRuntimePolicyRequest{})
	if err != nil {
		t.Fatalf("GetRuntimePolicy 失败: %v", err)
	}
	if before.GetRevision() != 0 {
		t.Fatalf("未写过时 revision 应为 0,得到 %d", before.GetRevision())
	}
	if v := before.GetPolicy().GetGlobal().GetRamEventBufferBytes(); v <= 0 {
		t.Fatalf("应回内置基线额度,得到 %d", v)
	}

	q := int64(2048)
	agentQuota := int64(512)
	if _, err := srv.SetRuntimePolicy(ctx, &pb.SetRuntimePolicyRequest{
		Policy: &pb.RuntimePolicy{
			Global: &pb.RuntimeLimits{RamEventBufferBytes: &q},
			Agents: map[string]*pb.RuntimeLimits{"echo": {RamEventBufferBytes: &agentQuota}},
		},
	}); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	after, err := srv.GetRuntimePolicy(ctx, &pb.GetRuntimePolicyRequest{})
	if err != nil {
		t.Fatalf("GetRuntimePolicy 失败: %v", err)
	}
	if got := after.GetPolicy().GetAgents()["echo"].GetRamEventBufferBytes(); got != agentQuota {
		t.Fatalf("agent 级覆盖未回读: %d", got)
	}
	// 读回路径同样要给生效值(客户端显示的是它,不是 policy 的缺席字段)。
	if eff := after.GetEffectiveGlobal(); eff.GetMaxParallelTurns() != 3 {
		t.Fatalf("读回缺生效值: 并发应为内置基线 3,得到 %d", eff.GetMaxParallelTurns())
	}
}

// TestTurnLeaseUsesEffectiveQuotaAndFollowsPolicyChange 覆盖验收点 ⑦(强制点)。
//
// 【为什么这条是 W7 的核心】"策略可改"若没有消费方,就只是往文件里写了一个数字。
// 本条钉住:额度真的进了租约的 soft/hard,而且改完策略后**下一轮立刻**按新额度记账。
func TestTurnLeaseUsesEffectiveQuotaAndFollowsPolicyChange(t *testing.T) {
	srv, _ := newPolicyTestServer(t)
	ctx := context.Background()
	th := &store.Thread{ID: "th-1", AgentID: "echo", Workspace: `C:\ws`}

	// ① 默认策略下的租约额度 = 内置基线的事件缓冲额度。
	lease := srv.openTurnLease(ctx, th, "turn-1")
	if lease == nil {
		t.Fatal("应申请到租约(后端已配置)")
	}
	want := *runtime.DefaultLimits().RAMEventBufferBytes
	if lease.SoftLimit != want || lease.HardLimit != want {
		t.Fatalf("租约额度未取自 Effective: soft=%d hard=%d want=%d", lease.SoftLimit, lease.HardLimit, want)
	}

	// ② 记账后用量可见,且带上了额度(否则"离上限多远"无从判断)。
	srv.chargeTurnLease(lease, 4096)
	rep := srv.runtimeBackend.Resources().Usage(ctx, runtime.ScopeThread("th-1"))
	if len(rep.Items) != 1 {
		t.Fatalf("应有 1 条活跃租约,得到 %+v", rep.Items)
	}
	if rep.Items[0].UsedBytes != 4096 {
		t.Fatalf("用量未记账: %d", rep.Items[0].UsedBytes)
	}
	if rep.Items[0].SoftLimit != want {
		t.Fatalf("租约没带上额度: %d", rep.Items[0].SoftLimit)
	}

	// ③ 改策略 → 下一轮的租约立刻用新额度(不重启、不重新读文件)。
	q := int64(4096)
	if _, err := srv.SetRuntimePolicy(ctx, &pb.SetRuntimePolicyRequest{
		Policy: &pb.RuntimePolicy{Global: &pb.RuntimeLimits{RamEventBufferBytes: &q}},
	}); err != nil {
		t.Fatalf("改策略失败: %v", err)
	}
	lease2 := srv.openTurnLease(ctx, th, "turn-2")
	if lease2 == nil {
		t.Fatal("第二轮应申请到租约")
	}
	if lease2.SoftLimit != q || lease2.HardLimit != q {
		t.Fatalf("改策略后新租约未用新额度: soft=%d hard=%d want=%d", lease2.SoftLimit, lease2.HardLimit, q)
	}

	// ④ agent 级覆盖要能压过 global(逐级覆盖在强制点上真的生效)。
	perAgent := int64(777)
	if _, err := srv.SetRuntimePolicy(ctx, &pb.SetRuntimePolicyRequest{
		Policy: &pb.RuntimePolicy{
			Global: &pb.RuntimeLimits{RamEventBufferBytes: &q},
			Agents: map[string]*pb.RuntimeLimits{"echo": {RamEventBufferBytes: &perAgent}},
		},
	}); err != nil {
		t.Fatalf("提交 agent 级策略失败: %v", err)
	}
	lease3 := srv.openTurnLease(ctx, th, "turn-3")
	if lease3 == nil {
		t.Fatal("第三轮应申请到租约")
	}
	if lease3.SoftLimit != perAgent {
		t.Fatalf("agent 级覆盖未在强制点生效: %d want %d", lease3.SoftLimit, perAgent)
	}

	// ⑤ 释放后退出活跃集合 —— 账单不会随轮次结束而残留。
	srv.closeTurnLease(lease)
	srv.closeTurnLease(lease2)
	srv.closeTurnLease(lease3)
	after := srv.runtimeBackend.Resources().Usage(ctx, runtime.ScopeThread("th-1"))
	if len(after.Items) != 0 {
		t.Fatalf("释放后不应残留活跃租约: %+v", after.Items)
	}
}

// TestGetResourceUsageExposesLeaseLimits 用量 RPC 带上额度与压力档(手机端要显示"离上限多远")。
func TestGetResourceUsageExposesLeaseLimits(t *testing.T) {
	srv, _ := newPolicyTestServer(t)
	ctx := context.Background()
	th := &store.Thread{ID: "th-2", AgentID: "echo"}
	lease := srv.openTurnLease(ctx, th, "turn-x")
	if lease == nil {
		t.Fatal("应申请到租约")
	}
	defer srv.closeTurnLease(lease)
	srv.chargeTurnLease(lease, 1024)

	resp, err := srv.GetResourceUsage(ctx, &pb.GetResourceUsageRequest{Scope: runtime.ScopeThread("th-2")})
	if err != nil {
		t.Fatalf("GetResourceUsage 失败: %v", err)
	}
	if len(resp.GetMetrics()) != 1 {
		t.Fatalf("应回 1 条租约,得到 %+v", resp.GetMetrics())
	}
	m := resp.GetMetrics()[0]
	if m.GetKind() != string(runtime.ResEventBuffer) {
		t.Fatalf("资源类别不对: %q", m.GetKind())
	}
	if m.GetSoftLimitBytes() <= 0 || m.GetHardLimitBytes() <= 0 {
		t.Fatalf("必须带上额度(否则'离上限多远'无从判断): soft=%d hard=%d", m.GetSoftLimitBytes(), m.GetHardLimitBytes())
	}
	if m.GetUsedBytes() != 1024 {
		t.Fatalf("用量不对: %d", m.GetUsedBytes())
	}
	if m.GetState() != string(runtime.PressureNormal) {
		t.Fatalf("未触上限时应报 normal,得到 %q", m.GetState())
	}
	if resp.GetSampledAtUnixMs() == 0 {
		t.Fatal("应带采样时刻(手机端要显示'多久前')")
	}
}

// TestRuntimePolicyCarriesUnknownRequestPassthrough 辅线 B:策略字段经
// SetRuntimePolicy / GetRuntimePolicy 自然携带(字段追加,RPC 数不变),且
// optional 的缺席语义保真 —— 缺席 ≠ false:客户端设置页要靠这个区别呈现
// "默认态"与"用户显式选过关"。
func TestRuntimePolicyCarriesUnknownRequestPassthrough(t *testing.T) {
	srv, dir := newPolicyTestServer(t)
	ctx := context.Background()

	// 从未设置:Get 回缺席(不是 false)。
	before, err := srv.GetRuntimePolicy(ctx, &pb.GetRuntimePolicyRequest{})
	if err != nil {
		t.Fatalf("GetRuntimePolicy 失败: %v", err)
	}
	if before.GetPolicy().UnknownRequestPassthrough != nil {
		t.Fatal("从未设置时字段应缺席(缺席 ≠ false,设置页靠它显示默认态)")
	}

	// 显式 true:Set → 落盘 → Get 回 true;重启(重载)后仍为 true。
	on := true
	if _, err := srv.SetRuntimePolicy(ctx, &pb.SetRuntimePolicyRequest{
		Policy: &pb.RuntimePolicy{UnknownRequestPassthrough: &on},
	}); err != nil {
		t.Fatalf("SetRuntimePolicy 失败: %v", err)
	}
	after, err := srv.GetRuntimePolicy(ctx, &pb.GetRuntimePolicyRequest{})
	if err != nil {
		t.Fatalf("GetRuntimePolicy 失败: %v", err)
	}
	if !after.GetPolicy().GetUnknownRequestPassthrough() {
		t.Fatalf("显式 true 未回读: %+v", after.GetPolicy())
	}
	reloaded, err := runtime.LoadBudgetPolicy(dir)
	if err != nil {
		t.Fatalf("重载策略失败: %v", err)
	}
	if reloaded.UnknownRequestPassthrough == nil || !*reloaded.UnknownRequestPassthrough {
		t.Fatalf("落盘丢失开关: %+v", reloaded.UnknownRequestPassthrough)
	}

	// 显式 false:保留为"设了 false",不得归一成缺席(整块替换语义下,缺席会被
	// 当成"未设置",用户显式关掉的事实必须留得住)。
	off := false
	if _, err := srv.SetRuntimePolicy(ctx, &pb.SetRuntimePolicyRequest{
		Policy:        &pb.RuntimePolicy{UnknownRequestPassthrough: &off},
		ExpectedRevision: after.GetRevision(),
	}); err != nil {
		t.Fatalf("SetRuntimePolicy(false) 失败: %v", err)
	}
	final, err := srv.GetRuntimePolicy(ctx, &pb.GetRuntimePolicyRequest{})
	if err != nil {
		t.Fatalf("GetRuntimePolicy 失败: %v", err)
	}
	if final.GetPolicy().UnknownRequestPassthrough == nil || final.GetPolicy().GetUnknownRequestPassthrough() {
		t.Fatalf("显式 false 应保留为已设置的 false: %+v", final.GetPolicy().UnknownRequestPassthrough)
	}
}
