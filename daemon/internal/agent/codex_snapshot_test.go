// codex_snapshot_test.go —— "设置页更改只对新建对话生效"的 turn 装配语义钉子。
//
// 【三级优先级】线程显式覆盖(Thread.ModelID/Effort,会话内 ModelEffortCard)>
// 线程设置快照(Thread.SettingsSnapshot,新建对话那一刻的值)> 全局偏好
// (ApplyPrefs 后的内存态,旧线程 nil 快照兜底)。用 fake codex app-server
// (见 codex_effect_test.go 的 TestHelperProcess 惯用法)核验真实发出的
// turn/start 参数、thread/start|resume 的 config 覆盖与 ThinkingDelta 过滤:
//
//	① 新建对话携带创建时快照:全局设置改了,老对话的 turn 参数不变;
//	③ 旧线程(快照 nil)回退全局行为(升级前语义原样);
//	④ ModelEffortCard 的会话内覆盖仍然最优先(压过快照与全局);
//	⑤ hideReasoning 全局优先:无论有无快照,一律按当前全局开关判定
//	  (与 pi/zcode 的 hideReasoningFor 三家一致;快照 HideReasoning 字段不参与判定,
//	  见 codex.go HideReasoningFor 的口径注释)。
package agent

import (
	"testing"

	"github.com/pocketagent/daemon/internal/store"
)

// ① 有快照的会话按快照装配:全局(ApplyPrefs)随后被改成完全不同的值,
// 该会话 turn/start 与 thread/start config 仍必须是快照那一刻的旧值。
func TestSnapshotThreadKeepsCreationTimeSettings(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	ws := t.TempDir()
	th := &store.Thread{ID: "daemon-snap-old", SettingsSnapshot: &store.TurnSettings{
		ApprovalPolicy:   "on-request",
		Sandbox:          "read-only",
		Model:            "codex/gpt-5.1",
		Effort:           "medium",
		Verbosity:        "low",
		ReasoningSummary: "concise",
	}}
	// "设置页后来改的新值" —— 不得影响上面的会话。
	p.ApplyPrefs(DesktopPrefs{
		ApprovalPolicy:   "never",
		Sandbox:          "danger-full-access",
		Model:            "codex/gpt-9",
		Effort:           "xhigh",
		Verbosity:        "high",
		ReasoningSummary: "detailed",
	})

	runOneTurn(t, p, th, ws)
	reqs := fakeCodexRequests(t, logPath)
	turn := findStartParams(t, reqs, "turn/start")
	if turn["approvalPolicy"] != "on-request" {
		t.Fatalf("有快照的会话审批策略应取快照 on-request, got=%v(全局已改成 never)", turn["approvalPolicy"])
	}
	sp, ok := turn["sandboxPolicy"].(map[string]interface{})
	if !ok || sp["type"] != "readOnly" {
		t.Fatalf("有快照的会话沙箱应取快照 readOnly, got=%v", turn["sandboxPolicy"])
	}
	if turn["model"] != "gpt-5.1" {
		t.Fatalf("有快照的会话模型应取快照 gpt-5.1, got=%v", turn["model"])
	}
	if turn["effort"] != "medium" {
		t.Fatalf("有快照的会话思考强度应取快照 medium, got=%v", turn["effort"])
	}
	if turn["summary"] != "concise" {
		t.Fatalf("有快照的会话思考摘要应取快照 concise, got=%v", turn["summary"])
	}
	start := findStartParams(t, reqs, "thread/start")
	cfg, ok := start["config"].(map[string]interface{})
	if !ok || cfg["model_verbosity"] != "low" || cfg["model_reasoning_summary"] != "concise" {
		t.Fatalf("有快照的会话 config 覆盖应取快照(verbosity=low/summary=concise), got=%v", start["config"])
	}
}

// ③ 旧线程(SettingsSnapshot=nil,快照机制上线前创建)回退全局行为:
// turn/start 与 config 覆盖照旧携带当前全局偏好,一行不变。
func TestLegacyThreadWithoutSnapshotFallsBackToGlobal(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	ws := t.TempDir()
	p.ApplyPrefs(DesktopPrefs{
		ApprovalPolicy:   "never",
		Sandbox:          "workspace-write",
		Model:            "codex/gpt-9",
		Effort:           "xhigh",
		Verbosity:        "high",
		ReasoningSummary: "detailed",
	})
	runOneTurn(t, p, &store.Thread{ID: "daemon-legacy-nil"}, ws)

	turn := findStartParams(t, fakeCodexRequests(t, logPath), "turn/start")
	if turn["approvalPolicy"] != "never" {
		t.Fatalf("旧线程审批策略应回全局 never, got=%v", turn["approvalPolicy"])
	}
	if sp, ok := turn["sandboxPolicy"].(map[string]interface{}); !ok || sp["type"] != "workspaceWrite" {
		t.Fatalf("旧线程沙箱应回全局 workspaceWrite, got=%v", turn["sandboxPolicy"])
	}
	if turn["model"] != "gpt-9" || turn["effort"] != "xhigh" || turn["summary"] != "detailed" {
		t.Fatalf("旧线程模型/强度/摘要应回全局, got model=%v effort=%v summary=%v",
			turn["model"], turn["effort"], turn["summary"])
	}
	start := findStartParams(t, fakeCodexRequests(t, logPath), "thread/start")
	cfg, ok := start["config"].(map[string]interface{})
	if !ok || cfg["model_verbosity"] != "high" || cfg["model_reasoning_summary"] != "detailed" {
		t.Fatalf("旧线程 config 覆盖应回全局, got=%v", start["config"])
	}
}

// ④ 会话内 ModelEffortCard 的模型/强度覆盖仍最优先:同时压过快照与全局。
func TestThreadModelEffortOverrideBeatsSnapshot(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	ws := t.TempDir()
	p.ApplyPrefs(DesktopPrefs{Model: "codex/gpt-9", Effort: "xhigh"})
	runOneTurn(t, p, &store.Thread{
		ID: "daemon-snap-override", ModelID: "codex/gpt-5.5-turbo", Effort: "high",
		SettingsSnapshot: &store.TurnSettings{Model: "codex/gpt-5.1", Effort: "medium"},
	}, ws)
	turn := findStartParams(t, fakeCodexRequests(t, logPath), "turn/start")
	if turn["model"] != "gpt-5.5-turbo" {
		t.Fatalf("会话内模型覆盖应压过快照与全局, got=%v", turn["model"])
	}
	if turn["effort"] != "high" {
		t.Fatalf("会话内强度覆盖应压过快照与全局, got=%v", turn["effort"])
	}
}

// ⑤ hideReasoning 线程化:有快照的会话按快照判定(与全局开关无关);
// 旧线程(nil)回退全局。
func TestHideReasoningIsGlobalAcrossSnapshots(t *testing.T) {
	p := NewCodexProvider()
	for _, hidden := range []bool{true, false} {
		p.ApplyPrefs(DesktopPrefs{HideReasoning: hidden})
		for _, thread := range []*store.Thread{nil, {}, {SettingsSnapshot: &store.TurnSettings{HideReasoning: !hidden}}} {
			if got := p.HideReasoningFor(thread); got != hidden {
				t.Fatalf("global filter=%v, got %v", hidden, got)
			}
		}
	}
}

// 补充钉子:快照里的空串 = "快照那一刻的跟随桌面",按原值生效 —— 不发参数、
// 不带 config,绝不中途改抓新的全局值(否则"保持原设置"会被全局变更击穿)。
func TestSnapshotEmptyFieldsStayEmpty(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	ws := t.TempDir()
	p.ApplyPrefs(DesktopPrefs{Model: "codex/gpt-9", Effort: "xhigh", Verbosity: "high", ReasoningSummary: "detailed"})
	runOneTurn(t, p, &store.Thread{ID: "daemon-snap-empty",
		SettingsSnapshot: &store.TurnSettings{Model: "codex/gpt-5.1"},
	}, ws)

	turn := findStartParams(t, fakeCodexRequests(t, logPath), "turn/start")
	if _, ok := turn["effort"]; ok {
		t.Fatalf("快照 effort 为空 = 不发该参数(跟随快照时的桌面), 不应回抓全局, got=%v", turn["effort"])
	}
	if _, ok := turn["summary"]; ok {
		t.Fatalf("快照摘要为空 = 不发该参数, 不应回抓全局, got=%v", turn["summary"])
	}
	start := findStartParams(t, fakeCodexRequests(t, logPath), "thread/start")
	if _, ok := start["config"]; ok {
		t.Fatalf("快照 verbosity/summary 为空 = 不带 config 覆盖, got=%v", start["config"])
	}
	if turn["model"] != "gpt-5.1" {
		t.Fatalf("模型仍应取快照, got=%v", turn["model"])
	}
}
