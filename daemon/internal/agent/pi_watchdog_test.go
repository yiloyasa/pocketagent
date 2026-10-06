// pi_watchdog_test.go —— pi 轮次看门狗判定的回归钉(P1-7)。
//
// 【为什么只钉判定函数】Run 主体的三闸接线(事件打点/审批计数/runCancel 传播)
// 依赖真子进程 harness,由既有 pi 契约套件覆盖挂载路径;这里钉的是看门狗的
// **决策面**——阈值比较、审批豁免、单轮上限优先级,三个分支一个都不能错:
// 错一个就是"误杀正常轮"或"停摆轮永挂"(缺陷即测试,AUDIT §4)。
package agent

import (
	"testing"
	"time"
)

func TestPiWatchdogVerdict(t *testing.T) {
	origSilence, origCap := piSilenceWatchdog, piTurnCap
	defer func() { piSilenceWatchdog, piTurnCap = origSilence, origCap }()
	// 收紧阈值:静默 150ms、单轮 300ms(判定函数只做时差比较,不必真睡)。
	piSilenceWatchdog = 150 * time.Millisecond
	piTurnCap = 300 * time.Millisecond

	cases := []struct {
		name       string
		sinceEvent time.Duration
		sinceStart time.Duration
		uiInFlight int64
		want       int
	}{
		{"事件新鲜_继续等", 10 * time.Millisecond, 50 * time.Millisecond, 0, 0},
		{"静默超限_杀", 200 * time.Millisecond, 250 * time.Millisecond, 0, 1},
		{"恰好达静默阈值_杀", 150 * time.Millisecond, 160 * time.Millisecond, 0, 1},
		{"审批在飞_豁免静默", 10 * time.Second, 250 * time.Millisecond, 1, 0},
		{"单轮超限_即便审批在飞也杀", 10 * time.Second, 400 * time.Millisecond, 1, 2},
		{"单轮超限优先于静默", 50 * time.Millisecond, 400 * time.Millisecond, 0, 2},
	}
	for _, tc := range cases {
		if got := piWatchdogVerdict(tc.sinceEvent, tc.sinceStart, tc.uiInFlight); got != tc.want {
			t.Fatalf("%s: piWatchdogVerdict(%v,%v,%d)=%d, want %d",
				tc.name, tc.sinceEvent, tc.sinceStart, tc.uiInFlight, got, tc.want)
		}
	}
}

// TestPiWatchdogThresholdsAlignWithOtherProviders 钉生产默认值不漂移:
// 150s 静默 + 60min 单轮 —— 与 codex/zcode/dsh/hermes 的同口径三闸一致
// (改阈值必须有意识和其余四家一起改,不许 pi 单家漂移)。
func TestPiWatchdogThresholdsAlignWithOtherProviders(t *testing.T) {
	if piSilenceWatchdog != 150*time.Second {
		t.Fatalf("piSilenceWatchdog 默认应 150s(对齐四家),得到 %s", piSilenceWatchdog)
	}
	if piTurnCap != 60*time.Minute {
		t.Fatalf("piTurnCap 默认应 60min(对齐四家),得到 %s", piTurnCap)
	}
}
