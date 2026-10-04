// codex_usage_probe_late_response_test.go —— 用量探测(account/rateLimits/read +
// account/usage/read)「迟到应答治理」的端到端回归钉(fake codex app-server)。
//
// 【背景】daemon.log 与未知事件台账实证:app-server 对这两个探测要先等自己的上游
// (chatgpt.com backend),慢路径远超 daemon 的 10s 预算;调用方超时降级后,上游
// 失败才补发迟到 error 帧 —— 每个探测窗在未知事件台账积 24 条。2026-10-01 起两个
// 探测改走 CallProbeCtx,迟到应答在 codexapp dispatch 处按调用代号静默丢弃。
//
// 【钉什么】两条场景,全部走生产链路(真 codexapp.Client + attachGlobalHandler +
// 共享台账),不 mock 分派层:
//
//	① 预算内正常应答仍生效:结果原样解析进 CodexUsage,Error 为空(能力位语义
//	   不变,换传输入口不得丢能力);
//	② 超时放弃后,迟到应答不再入未知台账:同 id 的迟到 error 帧被静默丢弃;同一
//	   帧流里**从未被请求**的孤儿错误帧(对照)必须照常入账 —— 证明台账管道活
//	   着、丢弃是按调用代号精确命中,而不是全局关闸。
package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/runtime"
)

// ledgerResponseCount 统计台账里所有 "response*" 登记名(含裸 response 与
// response:前缀家族)的条数,作基线/对照计数。
func ledgerResponseCount() int {
	n := 0
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{}, 0) {
		if strings.HasPrefix(e.SourceMethod, "response") {
			n++
		}
	}
	return n
}

// ① 预算内正常应答仍生效:fake 立即回真实形状的 payload,fetchUsageUncached 必须
// 原样解析(限额 + 累计用量),Error 为空 —— CallProbeCtx 不得影响正常路径。
func TestCodexUsageProbeWithinBudgetStillWorks(t *testing.T) {
	t.Setenv("PA_FAKE_CODEX_ACCOUNT_RESULT",
		`{"rateLimits":{"planType":"pro","primary":{"usedPercent":42,"resetsAt":1700000000,"windowDurationMins":5}},"summary":{"lifetimeTokens":777}}`)
	p, _ := newFakeCodexProvider(t, false)

	out := p.fetchUsageUncached()
	if out.Error != "" {
		t.Fatalf("预算内应答应成功,得到错误: %q", out.Error)
	}
	if out.PlanType != "pro" {
		t.Errorf("planType 应原样解析,得到 %q", out.PlanType)
	}
	if out.Primary == nil || out.Primary.GetUsedPercent() != 42 {
		t.Errorf("primary 窗口应解析 usedPercent=42,得到 %+v", out.Primary)
	}
	if out.LifetimeTokens != 777 {
		t.Errorf("lifetimeTokens 应解析 777,得到 %d", out.LifetimeTokens)
	}
}

// ② 超时放弃后,迟到应答不再入未知台账:
//   - 注入:fake 对两个 account/* 先睡 500ms 再回家族错误原文(daemon 侧预算
//     150ms,必先放弃);应答前先补一条从未被请求的孤儿错误帧(消息避开已知
//     家族 → 归裸 "response")作台账管道的活体对照;
//   - 断言:孤儿对照入账(管道活着),而已放弃探测的迟到应答("response:account/*"
//     家族,若未治理必然按消息原文归入此名)一条都不许出现。
func TestCodexAbandonedProbeLateResponseSkipsLedger(t *testing.T) {
	t.Setenv("PA_FAKE_CODEX_ACCOUNT_DELAY_MS", "500")
	t.Setenv("PA_FAKE_CODEX_ACCOUNT_ORPHAN", "1")
	p, _ := newFakeCodexProvider(t, false)
	p.usageProbeBudget = 150 * time.Millisecond // 生产恒为零值(= statusBudget 10s),此处仅钉时序

	before := ledgerResponseCount()
	out := p.fetchUsageUncached()
	if !strings.Contains(out.Error, "context deadline exceeded") {
		t.Fatalf("探测应按预算放弃(而非收到上游错误),得到: %q", out.Error)
	}

	// 等孤儿对照入账(证明台账管道与 fake 帧都被真实处理,断言才有对照意义)。
	deadline := time.Now().Add(5 * time.Second)
	for ledgerResponseCount() <= before {
		if time.Now().After(deadline) {
			t.Fatal("孤儿对照帧未入账:台账管道未走通,本测试失去对照意义")
		}
		time.Sleep(25 * time.Millisecond)
	}

	// 等两条迟到应答都到齐再断言:fake 单线程逐条睡 500ms,第二条 error 帧在
	// fetch 返回后约 700ms 才写出;留 2.5s 覆盖调度抖动。若丢弃失效,迟到帧会按
	// 消息原文归入 "response:account/*" 并在此窗口内现形。
	time.Sleep(2500 * time.Millisecond)

	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{}, 0) {
		if strings.HasPrefix(e.SourceMethod, "response:account/") {
			t.Errorf("已放弃探测的迟到应答不得入账: %s (%s) corr=%s",
				e.SourceMethod, e.NormalizedKind, e.CorrelationID)
		}
	}
	if got := ledgerResponseCount(); got != before+1 {
		t.Errorf("台账应恰好新增 1 条(孤儿对照),得到 before=%d after=%d", before, got)
	}
}
