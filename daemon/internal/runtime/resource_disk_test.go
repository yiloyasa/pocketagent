// resource_disk_test.go —— 资源账本的落盘与重启装载（D7 收尾 B）。
//
// 【口径与 memory_persistence_test.go 相同】账本此前是纯内存（backend.go 的
// memLedger），重启归零；只有"新 Backend 实例 + 同 dataDir"读回租约，才算
// ResourceLease 观测连续。账本是记账性质：丢账可接受，但**不丢时必须连得上**。
package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestResourceLedgerSurvivesRestart：Acquire/Charge 的账跨重启读回，
// Release 的释放状态同样连续（观测不复活已释放的租约）。
func TestResourceLedgerSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	b1 := NewFileBackend(dir)
	lease, err := b1.Resources().Acquire(ctx, LeaseRequest{
		LeaseID:       "lease-1",
		OwnerScope:    ScopeAgent("echo"),
		Kind:          ResConversation,
		ReservedBytes: 100,
		SoftLimit:     1000,
		HardLimit:     2000,
		Optional:      true,
	})
	if err != nil {
		t.Fatalf("Acquire 失败: %v", err)
	}
	if _, err := b1.Resources().Charge(ctx, lease.LeaseID, 350); err != nil {
		t.Fatalf("Charge 失败: %v", err)
	}
	// 再放一条已释放的租约：释放状态也要随账本走。
	lease2, err := b1.Resources().Acquire(ctx, LeaseRequest{
		LeaseID: "lease-2", OwnerScope: ScopeAgent("echo"), Kind: ResCacheDisk, ReservedBytes: 10,
	})
	if err != nil {
		t.Fatalf("Acquire(lease-2) 失败: %v", err)
	}
	if err := b1.Resources().Release(ctx, lease2.LeaseID); err != nil {
		t.Fatalf("Release 失败: %v", err)
	}
	if err := b1.FlushResourceLedger(); err != nil {
		t.Fatalf("FlushResourceLedger 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "resource-ledger.json")); err != nil {
		t.Fatalf("账本文件应已落盘: %v", err)
	}

	// —— 新 Backend 实例 + 同 dataDir（模拟 daemon 重启）——
	b2 := NewFileBackend(dir)
	if err := b2.SyncResourceLedger(); err != nil {
		t.Fatalf("重启装载账本失败: %v", err)
	}
	rep := b2.Resources().Usage(ctx, ScopeAgent("echo"))
	if rep.Leases != 1 {
		t.Fatalf("重启后活跃租约应为 1（已释放的不计入活跃），实得 %d", rep.Leases)
	}
	if rep.ByKind[ResConversation] != 450 {
		t.Fatalf("重启后用量应连续（100 预留 + 350 记账 = 450），实得 %d", rep.ByKind[ResConversation])
	}
	if len(rep.Items) != 1 || rep.Items[0].LeaseID != "lease-1" {
		t.Fatalf("重启后应能看见 lease-1 的额度与用量，实得 %+v", rep.Items)
	}
	// 装载后的账本可继续记账：额度约束（硬上限）在重启后仍然生效。
	if _, err := b2.Resources().Charge(ctx, "lease-1", 2000); err == nil {
		t.Fatal("重启后硬上限约束应仍然生效（450+2000 越界须拒绝）")
	}
	// 已释放状态随账本连续：装载后对已释放租约重复释放应幂等成功而非"不存在"。
	if err := b2.Resources().Release(ctx, "lease-2"); err != nil {
		t.Fatalf("已释放租约的释放状态应随账本保留（幂等释放应成功）: %v", err)
	}
}

// TestResourceLedgerSyncsOnObserve：诊断读路径（Observe）顺带刷账 ——
// 这是"定期刷盘"的落地形式：以真实读流量为周期，而不是空转定时器。
func TestResourceLedgerSyncsOnObserve(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	b := NewFileBackend(dir)
	if _, err := b.Resources().Acquire(ctx, LeaseRequest{
		LeaseID: "lease-obs", OwnerScope: ScopeAgent("echo"), Kind: ResConversation, ReservedBytes: 5,
	}); err != nil {
		t.Fatalf("Acquire 失败: %v", err)
	}
	b.Observe("echo") // 读路径触发同步
	if _, err := os.Stat(filepath.Join(dir, "resource-ledger.json")); err != nil {
		t.Fatalf("Observe 后账本应已落盘: %v", err)
	}
	// 新实例不显式 Flush，只走同样的读路径也应装载回来。
	b2 := NewFileBackend(dir)
	b2.Observe("echo")
	if got := b2.Resources().Usage(ctx, ScopeAgent("echo")); got.Leases != 1 {
		t.Fatalf("读路径同步应让重启后的观测连续，实得 %+v", got)
	}
}

// TestResourceLedgerCorruptFileFailsOpen：账本是记账性质，损坏时丢弃重建
// （不阻塞运行），后续写入用新账覆盖旧文件。
func TestResourceLedgerCorruptFileFailsOpen(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(dir, "resource-ledger.json"),
		[]byte("{not-json"), 0o600); err != nil {
		t.Fatalf("制造损坏账本失败: %v", err)
	}
	b := NewFileBackend(dir)
	if err := b.SyncResourceLedger(); err != nil {
		t.Fatalf("损坏账本必须 fail-open（丢弃重建），不得报错阻断: %v", err)
	}
	lease, err := b.Resources().Acquire(ctx, LeaseRequest{
		LeaseID: "lease-new", OwnerScope: ScopeAgent("echo"), Kind: ResConversation, ReservedBytes: 1,
	})
	if err != nil {
		t.Fatalf("损坏后 Acquire 应照常可用: %v", err)
	}
	if err := b.FlushResourceLedger(); err != nil {
		t.Fatalf("损坏后刷盘失败: %v", err)
	}
	b2 := NewFileBackend(dir)
	if err := b2.SyncResourceLedger(); err != nil {
		t.Fatalf("重载失败: %v", err)
	}
	rep := b2.Resources().Usage(ctx, ScopeAgent("echo"))
	if rep.Leases != 1 || rep.Items[0].LeaseID != lease.LeaseID {
		t.Fatalf("新账应完整覆盖旧文件，实得 %+v", rep)
	}
}

// TestResourceLedgerMemoryBackendStaysInMemory：dataDir 为空 = 纯内存，
// 历史语义零分叉（不落任何文件）。
func TestResourceLedgerMemoryBackendStaysInMemory(t *testing.T) {
	ctx := context.Background()
	b := NewBackend()
	if _, err := b.Resources().Acquire(ctx, LeaseRequest{
		LeaseID: "l", OwnerScope: ScopeGlobal(), Kind: ResConversation, ReservedBytes: 1,
	}); err != nil {
		t.Fatalf("Acquire 失败: %v", err)
	}
	if err := b.SyncResourceLedger(); err != nil {
		t.Fatalf("内存后端 Sync 应为无害空操作: %v", err)
	}
	if err := b.FlushResourceLedger(); err != nil {
		t.Fatalf("内存后端 Flush 应为无害空操作: %v", err)
	}
	rep := b.Resources().Usage(ctx, ScopeGlobal())
	if rep.Leases != 1 {
		t.Fatalf("内存后端记账语义不变，实得 %+v", rep)
	}
}

// TestResourceLedgerReleasedCapBoundsFile：账本文件对已释放租约设上限，
// 长跑进程的账本文件不得无限增长。
func TestResourceLedgerReleasedCapBoundsFile(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	b := NewFileBackend(dir)
	for i := 0; i < 150; i++ {
		id := fmt.Sprintf("released-%03d", i)
		lease, err := b.Resources().Acquire(ctx, LeaseRequest{
			LeaseID: id, OwnerScope: ScopeAgent("echo"), Kind: ResConversation, ReservedBytes: 1,
		})
		if err != nil {
			t.Fatalf("Acquire(%s) 失败: %v", id, err)
		}
		if err := b.Resources().Release(ctx, lease.LeaseID); err != nil {
			t.Fatalf("Release(%s) 失败: %v", id, err)
		}
	}
	if err := b.FlushResourceLedger(); err != nil {
		t.Fatalf("Flush 失败: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "resource-ledger.json"))
	if err != nil {
		t.Fatalf("读账本失败: %v", err)
	}
	// 上限 = 活跃(0) + 最多 100 条已释放。粗验：文件里不应有 150 条租约。
	if got := bytes.Count(raw, []byte(`"lease_id"`)); got > 120 {
		t.Fatalf("已释放租约应被截断到上限附近，实得 %d 条", got)
	}
}
