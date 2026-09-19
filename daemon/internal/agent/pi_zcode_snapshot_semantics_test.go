// pi_zcode_snapshot_semantics_test.go —— P2-8 回归钉子:pi/zcode 的 Run 消费
// effort/hideReasoning 时按 Thread.SettingsSnapshot 快照语义(与 codex 三级
// 优先级对齐):
//
//	线程显式 Effort(Thread.Effort)> 线程设置快照(SettingsSnapshot)> 全局偏好。
//
// 【背景】设置页"仅新建生效"的快照语义上线后,codex 每轮按快照装配,而
// pi/zcode 的 Run 仍读全局 —— 老会话的思考强度/隐藏推理随设置页漂移。
// 不依赖子进程,纯内存态断言。
package agent

import (
	"testing"

	"github.com/pocketagent/daemon/internal/store"
)

// snapOf 造一个带快照的线程(七项里 pi/zcode 只消费 effort/hideReasoning)。
func snapOf(effort string, hide bool) *store.Thread {
	return &store.Thread{
		SettingsSnapshot: &store.TurnSettings{Effort: effort, HideReasoning: hide},
	}
}

func TestPiHideReasoningForSnapshotSemantics(t *testing.T) {
	p := NewPiProvider()
	p.ApplyPrefs(DesktopPrefs{HideReasoning: true}) // 全局:隐藏

	// 旧线程(快照 nil):回全局,保持升级前行为。
	if !p.hideReasoningFor(&store.Thread{}) {
		t.Fatal("快照 nil 应回退全局 HideReasoning=true")
	}
	// 有快照:一律按快照 —— 快照 false(当时未勾)不随全局 true 漂移。
	if !p.hideReasoningFor(snapOf("", false)) {
		t.Fatal("全局隐藏必须覆盖旧快照 false")
	}
	// 快照 true(当时勾了)在全局后来关掉时仍隐藏。
	p.ApplyPrefs(DesktopPrefs{HideReasoning: false})
	if p.hideReasoningFor(snapOf("", true)) {
		t.Fatal("全局显示必须覆盖旧快照 true")
	}
}

func TestPiDesktopEffortForSnapshotSemantics(t *testing.T) {
	p := NewPiProvider()
	p.ApplyPrefs(DesktopPrefs{Effort: "high"}) // 全局:high

	// 旧线程(快照 nil):回全局。
	if got := p.desktopEffortFor(&store.Thread{}); got != "high" {
		t.Fatalf("快照 nil 应回退全局 effort=high,得到 %q", got)
	}
	// 有快照:快照里的空串就是"当时未设置",保持原值生效、不中途改抓新全局值
	// (与 codex 的"快照空串按原值生效"同语义)。
	if got := p.desktopEffortFor(snapOf("", false)); got != "" {
		t.Fatalf("快照 effort 空串应保持空串(不回抓全局),得到 %q", got)
	}
	// 快照有值:按快照,全局漂移不影响。
	p.ApplyPrefs(DesktopPrefs{Effort: "low"})
	if got := p.desktopEffortFor(snapOf("medium", false)); got != "medium" {
		t.Fatalf("快照 effort=medium 应按快照,得到 %q", got)
	}
}

func TestZcodeHideReasoningForSnapshotSemantics(t *testing.T) {
	p := NewZcodeProvider()
	p.ApplyPrefs(DesktopPrefs{HideReasoning: true})

	if !p.hideReasoningFor(&store.Thread{}) {
		t.Fatal("快照 nil 应回退全局 HideReasoning=true")
	}
	if !p.hideReasoningFor(snapOf("", false)) {
		t.Fatal("全局隐藏必须覆盖旧快照 false")
	}
}

func TestZcodeDesktopEffortForSnapshotSemantics(t *testing.T) {
	p := NewZcodeProvider()
	p.ApplyPrefs(DesktopPrefs{Effort: "high"})

	if got := p.desktopEffortFor(&store.Thread{}); got != "high" {
		t.Fatalf("快照 nil 应回退全局 effort=high,得到 %q", got)
	}
	if got := p.desktopEffortFor(snapOf("", false)); got != "" {
		t.Fatalf("快照 effort 空串应保持空串(不回抓全局),得到 %q", got)
	}
	if got := p.desktopEffortFor(snapOf("medium", false)); got != "medium" {
		t.Fatalf("快照 effort=medium 应按快照,得到 %q", got)
	}
}

// 三级优先级端到端(纯函数组合):线程显式 > 快照 effort > 模型 defaultLevel。
func TestZcodeEffortThreeLevelPriorityWithSnapshot(t *testing.T) {
	p := NewZcodeProvider()
	p.ApplyPrefs(DesktopPrefs{Effort: "high"}) // 全局(仅旧线程可见)

	th := &store.Thread{Effort: "xhigh"} // 第一级:线程显式
	if got := zcodeReasoningLevel(th.Effort, p.desktopEffortFor(th), "low"); got != "xhigh" {
		t.Fatalf("线程显式应最高优先,得到 %q", got)
	}

	th = &store.Thread{SettingsSnapshot: &store.TurnSettings{Effort: "medium"}} // 第二级:快照
	if got := zcodeReasoningLevel(th.Effort, p.desktopEffortFor(th), "low"); got != "medium" {
		t.Fatalf("快照 effort 应次优先,得到 %q", got)
	}

	th = &store.Thread{} // 第三级:模型自带 defaultLevel(旧线程无快照无显式)
	// 旧线程回全局 —— 全局此刻是 high;把全局清零才落到 defaultLevel。
	p.ApplyPrefs(DesktopPrefs{})
	if got := zcodeReasoningLevel(th.Effort, p.desktopEffortFor(th), "low"); got != "low" {
		t.Fatalf("全空应用模型 defaultLevel,得到 %q", got)
	}
}
