// host_recovery_wire_test.go —— W6b:启动恢复报告经 RPC 送达手机的回归用例。
//
// 【为什么与 recovery_report_test.go 分开】那个文件钉的是"报告本身算得对不对"
// (进程内结构),本文件钉的是"它有没有真的走到手机"(GetHostInfo 返回里带没带)。
// 两者的失效原因不同:判定算错 → 前者红;判定算对但没人填进 RPC → 只有后者红。
// 合成一个文件会让后一种失效被前一种掩盖 —— 而"只写进日志、手机端看不到"
// 正是这一轮被判为"未达成"的那类缺陷。
package server

import (
	"context"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// TestGetHostInfoCarriesTurnJournalLoss 覆盖 W6b 的核心判据:
// 排队台账里丢失的输入,必须逐条出现在 GetHostInfo 的返回里(带"丢的是哪句")。
func TestGetHostInfoCarriesTurnJournalLoss(t *testing.T) {
	dir := t.TempDir()
	// 模拟"重启前"留下的未完成台账(与 recovery_report_test.go 同款手法)。
	seed := newTurnJournal(dir)
	seed.Record("q1", "thread-1", "重启前排队的第一句")
	seed.Record("q2", "thread-2", "重启前排队的第二句")

	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{Store: st, DataDir: dir})
	t.Cleanup(func() { _ = srv.events.Close() })

	info, err := srv.GetHostInfo(context.Background(), &pb.GetHostInfoRequest{})
	if err != nil {
		t.Fatalf("GetHostInfo 失败: %v", err)
	}
	rec := info.GetRecovery()
	if rec == nil {
		t.Fatal("GetHostInfo 必须带恢复报告:丢了排队输入却只有 daemon 日志知道 = 手机端看不到 = 未达成")
	}
	if rec.GetStatus() != RecoveryStatusDegraded {
		t.Fatalf("有丢失的排队输入应判 degraded,得到 %q", rec.GetStatus())
	}
	tj := rec.GetTurnJournal()
	if tj.GetPendingCount() != 2 {
		t.Fatalf("pending_count 应为 2,得到 %d", tj.GetPendingCount())
	}
	// 逐字核对预览:手机端要靠它让用户认出"丢的是哪句"。
	got := map[string]string{}
	for _, li := range tj.GetLostInputs() {
		got[li.GetThreadId()] = li.GetPreview()
	}
	if got["thread-1"] != "重启前排队的第一句" || got["thread-2"] != "重启前排队的第二句" {
		t.Fatalf("丢的是哪句必须逐字送到手机,得到 %+v", got)
	}
	if len(tj.GetThreadIds()) != 2 {
		t.Fatalf("涉及会话应报告 2 个,得到 %+v", tj.GetThreadIds())
	}
	if !strings.Contains(rec.GetSummary(), "排队输入") {
		t.Fatalf("摘要应点名排队输入丢失,得到 %q", rec.GetSummary())
	}
}

// TestGetHostInfoCarriesCleanRecovery 干净启动也必须带报告(空台账)。
//
// 【为什么"没问题"也要有字段】若只有异常时才填,手机端就只能靠"字段缺失"推断
// 一切正常 —— 而字段缺失同样意味着"这是台旧 daemon"。分不清这两件事,UI 要么
// 误报异常,要么显示"未知"(AUDIT §3.3 禁止措辞)。
func TestGetHostInfoCarriesCleanRecovery(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{Store: st, DataDir: dir})
	t.Cleanup(func() { _ = srv.events.Close() })

	info, err := srv.GetHostInfo(context.Background(), &pb.GetHostInfoRequest{})
	if err != nil {
		t.Fatalf("GetHostInfo 失败: %v", err)
	}
	rec := info.GetRecovery()
	if rec == nil {
		t.Fatal("干净启动也必须带恢复报告(不能靠字段缺失表达 ok)")
	}
	if rec.GetStatus() != RecoveryStatusOK {
		t.Fatalf("干净启动应判 ok,得到 %q(摘要 %q)", rec.GetStatus(), rec.GetSummary())
	}
	if rec.GetTurnJournal().GetPendingCount() != 0 || len(rec.GetTurnJournal().GetLostInputs()) != 0 {
		t.Fatalf("干净启动不应报丢失条目,得到 %+v", rec.GetTurnJournal())
	}
	// 运行时控制面能力必须声明(手机端据此点亮入口;缺了就只能靠版本号猜,
	// 而版本号二分正是 AUDIT 点名要消掉的形态)。
	if joined := strings.Join(info.GetCapabilities(), ","); !strings.Contains(joined, "runtime.v1") {
		t.Fatalf("GetHostInfo 应声明 runtime.v1 能力,得到 %q", joined)
	}
}
