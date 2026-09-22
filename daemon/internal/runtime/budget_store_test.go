// budget_store_test.go —— 预算策略的落盘、校验与用量明细(规格 §6.5 的控制面底座)。
//
// 【为什么与 budget_test.go 分开】那个文件钉的是"逐级覆盖与溯源怎么算"(纯函数);
// 本文件钉的是"算出来的东西能不能安全地存下来、能不能被观察" —— 读写路径与
// 覆盖面完全不同,混在一起会让"落盘坏了"被"算错了"掩盖。
package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestSaveBudgetPolicyRoundTrip：写出去、读回来一致（含 revision）。
func TestSaveBudgetPolicyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := BudgetPolicy{
		Revision: 3,
		Global:   Limits{RAMWorkingBytes: i64p(1234), MaxParallelTurns: intp(2)},
		Agents:   map[string]Limits{"echo": {DiskCacheBytes: i64p(99)}},
	}
	if err := SaveBudgetPolicy(dir, p); err != nil {
		t.Fatalf("落盘失败: %v", err)
	}
	got, err := LoadBudgetPolicy(dir)
	if err != nil {
		t.Fatalf("读回失败: %v", err)
	}
	if got.Revision != 3 {
		t.Fatalf("revision 未随文件落盘(CAS 基准必须跨重启成立): %d", got.Revision)
	}
	lim := got.Effective("echo", "", "", "")
	if lim.RAMWorkingBytes == nil || *lim.RAMWorkingBytes != 1234 {
		t.Fatalf("global 未读回: %v", lim.RAMWorkingBytes)
	}
	if lim.DiskCacheBytes == nil || *lim.DiskCacheBytes != 99 {
		t.Fatalf("agent 级未读回: %v", lim.DiskCacheBytes)
	}
	// 原子写：不该把临时文件留在目录里（残留会让"目录里有什么"不再可信）。
	if _, err := os.Stat(filepath.Join(dir, budgetFileName+".tmp")); err == nil {
		t.Fatal("落盘后不该残留 .tmp")
	}
}

// TestSaveBudgetPolicyRejectsNegative：负额度必须被拒且不落盘。
func TestSaveBudgetPolicyRejectsNegative(t *testing.T) {
	dir := t.TempDir()
	err := SaveBudgetPolicy(dir, BudgetPolicy{Global: Limits{RAMWorkingBytes: i64p(-5)}})
	if err == nil {
		t.Fatal("负额度应被拒（下游会把它当成'未设置'，表现为设了不生效）")
	}
	if _, serr := os.Stat(filepath.Join(dir, budgetFileName)); serr == nil {
		t.Fatal("被拒的写入不该落盘")
	}
}

// TestSaveBudgetPolicyNoDataDirFailsLoudly：无数据目录 → 如实失败。
//
// 【为什么不能静默成功】这是用户在控制面上显式提交的一次修改：存不下来却回"成功"，
// 用户会以为策略生效了，而进程一重启就变回旧值。
func TestSaveBudgetPolicyNoDataDirFailsLoudly(t *testing.T) {
	if err := SaveBudgetPolicy("", BudgetPolicy{Global: Limits{RAMWorkingBytes: i64p(1)}}); err == nil {
		t.Fatal("无数据目录时必须如实失败")
	}
}

// TestValidateCoversNestedScopes：每一级的负值都能被拒（不是只查 global）。
func TestValidateCoversNestedScopes(t *testing.T) {
	cases := map[string]BudgetPolicy{
		"global":    {Global: Limits{MaxParallelTurns: intp(-1)}},
		"agent":     {Agents: map[string]Limits{"pi": {RAMWorkingBytes: i64p(-1)}}},
		"workspace": {Workspaces: map[string]Limits{`C:\w`: {DiskCacheBytes: i64p(-1)}}},
		"thread":    {Threads: map[string]Limits{"th": {RAMSessionIndexBytes: i64p(-1)}}},
		"collab":    {Collabs: map[string]Limits{"c": {DiskArtifactBytes: i64p(-1)}}},
	}
	for name, p := range cases {
		if err := p.Validate(); err == nil {
			t.Fatalf("%s 级的负额度未被拒", name)
		}
	}
	// 零是合法的（明令封禁），不能被误当成非法值。
	if err := (BudgetPolicy{Global: Limits{MaxParallelTurns: intp(0)}}).Validate(); err != nil {
		t.Fatalf("显式 0 是合法取值（封禁），不该被拒: %v", err)
	}
}

// TestUsageReportCarriesItems：用量汇总必须带明细（额度 + 用量 + 压力档）。
func TestUsageReportCarriesItems(t *testing.T) {
	b := NewBackend()
	ctx := context.Background()
	if _, err := b.Resources().Acquire(ctx, LeaseRequest{
		LeaseID: "l1", OwnerScope: ScopeThread("th"), Kind: ResEventBuffer,
		ReservedBytes: 10, SoftLimit: 100, HardLimit: 200, Optional: false,
	}); err != nil {
		t.Fatalf("申请租约失败: %v", err)
	}
	if _, err := b.Resources().Charge(ctx, "l1", 90); err != nil {
		t.Fatalf("记账失败: %v", err)
	}
	rep := b.Resources().Usage(ctx, ScopeThread("th"))
	if len(rep.Items) != 1 {
		t.Fatalf("应回 1 条明细,得到 %+v", rep.Items)
	}
	it := rep.Items[0]
	if it.UsedBytes != 100 {
		t.Fatalf("用量 = 预占 10 + 记账 90 应为 100,得到 %d", it.UsedBytes)
	}
	if it.SoftLimit != 100 || it.HardLimit != 200 {
		t.Fatalf("明细必须带上额度(否则'离上限多远'无从判断): %d/%d", it.SoftLimit, it.HardLimit)
	}
	if it.Pressure() != PressureSoft {
		t.Fatalf("到软上限应报 soft,得到 %q", it.Pressure())
	}
	// 汇总与明细同源：不能出现"总数与明细对不上"。
	if rep.ByKind[ResEventBuffer] != 100 {
		t.Fatalf("汇总与明细不一致: %d", rep.ByKind[ResEventBuffer])
	}
}
