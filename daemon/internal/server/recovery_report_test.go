// recovery_report_test.go —— 启动恢复报告(recovery_report.go)的回归用例(规格 §8.2)。
//
// 验收点:
//
//	① 干净启动 → ok,store 来源为 clean,且能看到"已有 .bak";
//	② 会话库被隔离留档 → corrupted,摘要里给 .bak 恢复提示与留档路径;
//	③ 设备表降级 → degraded,报告如实反映 fail-closed;
//	④ 排队输入台账有未完成条目 → 报告里看得见丢了哪些会话,判为 degraded;
//	⑤ 空 dataDir(测试/内存模式)→ 不 panic,报告仍可读。
package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/store"
)

const recoveryGoodThreads = `{"threads":{},"default_workspace":"C:\\dev"}`

func writeRecoveryFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("写 %s 失败: %v", name, err)
	}
}

// TestRecoveryReportCleanStartupIsOK 覆盖验收点 ①:一份可解析的会话库 = 干净启动。
func TestRecoveryReportCleanStartupIsOK(t *testing.T) {
	dir := t.TempDir()
	writeRecoveryFile(t, dir, "threads.json", recoveryGoodThreads)

	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	if got := st.LoadOutcome(); got != store.LoadOutcomeClean {
		t.Fatalf("正常文件应记为 clean,得到 %q", got)
	}

	buf := captureStdlog(t)
	srv := New(Config{Store: st, DataDir: dir})
	t.Cleanup(func() { _ = srv.events.Close() })

	rep := srv.RecoveryReport()
	if rep.Status != RecoveryStatusOK || !rep.IsOK() {
		t.Fatalf("干净启动应判 ok,得到 status=%q summary=%q", rep.Status, rep.Summary)
	}
	if rep.Store.Outcome != store.LoadOutcomeClean {
		t.Fatalf("store.Outcome 应为 clean,得到 %q", rep.Store.Outcome)
	}
	if !rep.Store.HasBackup {
		t.Fatalf("成功加载后应有 threads.json.bak,报告的 has_backup 应为 true")
	}
	if rep.TurnJournal.PendingCount != 0 {
		t.Fatalf("无排队输入时 pending 应为 0,得到 %d", rep.TurnJournal.PendingCount)
	}
	if rep.Devices.Degraded || rep.Identity.Degraded {
		t.Fatalf("干净启动不应有降级: devices=%+v identity=%+v", rep.Devices, rep.Identity)
	}
	// 接线验证:启动路径确实打了一条醒目汇总日志。
	if !strings.Contains(buf.String(), "[recovery]") {
		t.Fatalf("启动路径应打一条 [recovery] 汇总日志,实际:\n%s", buf.String())
	}
}

// TestRecoveryReportStoreQuarantinedIsCorrupted 覆盖验收点 ②:
// 解析失败被隔离留档 → corrupted,并带 .bak 恢复提示。
func TestRecoveryReportStoreQuarantinedIsCorrupted(t *testing.T) {
	dir := t.TempDir()
	// 先有一次好版本,生成 threads.json.bak。
	writeRecoveryFile(t, dir, "threads.json", recoveryGoodThreads)
	if _, err := store.NewFileStore(dir); err != nil {
		t.Fatalf("好版本首启不应失败: %v", err)
	}
	// 把主文件弄坏,再启动 → 隔离自愈。
	writeRecoveryFile(t, dir, "threads.json", `{"threads": {`)

	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("损坏文件应自愈启动而非报错: %v", err)
	}
	if got := st.LoadOutcome(); got != store.LoadOutcomeQuarantined {
		t.Fatalf("解析失败应记为 quarantined,得到 %q", got)
	}

	srv := New(Config{Store: st, DataDir: dir})
	t.Cleanup(func() { _ = srv.events.Close() })

	rep := srv.RecoveryReport()
	if rep.Status != RecoveryStatusCorrupted {
		t.Fatalf("会话库隔离留档应判 corrupted,得到 status=%q summary=%q", rep.Status, rep.Summary)
	}
	if rep.Store.Outcome != store.LoadOutcomeQuarantined {
		t.Fatalf("store.Outcome 应为 quarantined,得到 %q", rep.Store.Outcome)
	}
	if !rep.Store.HasBackup {
		t.Fatalf("隔离场景应有 .bak 可用,报告的 has_backup 应为 true")
	}
	if !strings.Contains(rep.Summary, ".bak") {
		t.Fatalf("摘要应给 .bak 恢复提示,得到 %q", rep.Summary)
	}
	if !strings.Contains(rep.Summary, "隔离留档") {
		t.Fatalf("摘要应说明已隔离留档,得到 %q", rep.Summary)
	}
	if rep.Store.QuarantinePath == "" {
		t.Fatalf("quarantine_path 应指向留档文件")
	}
	if _, err := os.Stat(rep.Store.QuarantinePath); err != nil {
		t.Fatalf("报告的留档路径应真实存在: %v", err)
	}
}

// TestRecoveryReportDevicesDegraded 覆盖验收点 ③:设备表损坏 → fail-closed 降级。
func TestRecoveryReportDevicesDegraded(t *testing.T) {
	dir := t.TempDir()
	writeRecoveryFile(t, dir, "devices.json", "{broken json")

	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}

	srv := New(Config{Store: st, PairCode: "123456", DataDir: dir})
	t.Cleanup(func() { _ = srv.events.Close() })

	rep := srv.RecoveryReport()
	if !rep.Devices.Degraded {
		t.Fatalf("损坏设备表应反映为 degraded,得到 %+v", rep.Devices)
	}
	if rep.Devices.Reason == "" {
		t.Fatalf("降级应记录原因,得到空")
	}
	if rep.Status != RecoveryStatusDegraded {
		t.Fatalf("设备表降级应使总体判为 degraded,得到 status=%q summary=%q", rep.Status, rep.Summary)
	}
	if !strings.Contains(rep.Summary, "设备表") {
		t.Fatalf("摘要应点名设备表降级,得到 %q", rep.Summary)
	}
	// 会话库本身没坏,不应被误判成 corrupted。
	if rep.Store.Outcome == store.LoadOutcomeQuarantined {
		t.Fatalf("设备表损坏不应把会话库误报为 quarantined")
	}
}

// TestRecoveryReportTurnJournalPending 覆盖验收点 ④:有未完成排队输入时报告能体现。
func TestRecoveryReportTurnJournalPending(t *testing.T) {
	dir := t.TempDir()
	// 模拟"重启前"留下的未完成台账。
	seed := newTurnJournal(dir)
	seed.Record("q1", "thread-1", "重启前排队的第一句")
	seed.Record("q2", "thread-2", "重启前排队的第二句")

	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}

	srv := New(Config{Store: st, DataDir: dir})
	t.Cleanup(func() { _ = srv.events.Close() })

	rep := srv.RecoveryReport()
	if rep.TurnJournal.PendingCount != 2 {
		t.Fatalf("应报告 2 条未完成排队输入,得到 %d", rep.TurnJournal.PendingCount)
	}
	if len(rep.TurnJournal.ThreadIDs) != 2 {
		t.Fatalf("应报告涉及的 2 个会话 id,得到 %+v", rep.TurnJournal.ThreadIDs)
	}
	joined := strings.Join(rep.TurnJournal.ThreadIDs, ",")
	if !strings.Contains(joined, "thread-1") || !strings.Contains(joined, "thread-2") {
		t.Fatalf("涉及会话应含 thread-1/thread-2,得到 %q", joined)
	}
	if rep.Status != RecoveryStatusDegraded {
		t.Fatalf("有丢失的排队输入应判 degraded,得到 status=%q", rep.Status)
	}
	// 报告读出后台账仍被清空(与既有 reportPendingQueueInputs 语义一致)。
	if got := srv.turnJournal.Pending(); len(got) != 0 {
		t.Fatalf("报告后台账应清空,得到 %+v", got)
	}
}

// TestRecoveryReportEmptyDataDirNoPanic 覆盖验收点 ⑤:空 dataDir 不 panic。
func TestRecoveryReportEmptyDataDirNoPanic(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}

	// 空 DataDir = 测试/内存模式:不得 panic,报告仍可读。
	srv := New(Config{Store: st})
	rep := srv.RecoveryReport()

	if rep.Status != RecoveryStatusOK {
		t.Fatalf("内存模式无降级应判 ok,得到 status=%q", rep.Status)
	}
	if rep.Store.Outcome != store.LoadOutcomeFirstRun {
		t.Fatalf("空目录首启应记为 first-run,得到 %q", rep.Store.Outcome)
	}
	if rep.Devices.Degraded || rep.Identity.Degraded {
		t.Fatalf("内存模式不应有降级: devices=%+v identity=%+v", rep.Devices, rep.Identity)
	}
}
