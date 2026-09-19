// zcode_prefs_effect_test.go —— ZCode 侧消费桌面设置(七项偏好)的单测,不依赖子进程。
//
// 【钉什么】
//  1. HideReasoning 开关:emitThinking 打开时 ThinkingDelta 一帧不出(与
//     codex.go / pi.go 的 daemon 侧过滤同语义);
//  2. effort 默认值→options.reasoningLevel 的取值优先级:
//     线程显式 > 桌面默认 > 模型自带 defaultLevel,全空才省略 options
//     (GLM 系不带 reasoningLevel 会 -32603,不能轻易不带);
//  3. ApplyPrefs 完整快照覆盖语义;审批策略/沙箱收下但无消费点(权限模式固定 build)。
package agent

import (
	"encoding/json"
	"testing"
)

func TestZcodeHideReasoningFilter(t *testing.T) {
	p := NewZcodeProvider()
	var got []string
	// recordingSink 见 pi_prefs_effect_test.go(同包共享,reflect 构造避免新增 vet 告警)。
	sink := recordingSink(&got)

	// 未灌偏好(零值 = 不隐藏):思考增量照常透出。
	p.emitThinking("第一段", sink)
	if len(got) != 1 || got[0] != "第一段" {
		t.Fatalf("默认应透出思考增量, got=%v", got)
	}

	// 开关打开:daemon 侧强制过滤,一帧不出。
	p.ApplyPrefs(DesktopPrefs{HideReasoning: true})
	p.emitThinking("第二段", sink)
	if len(got) != 1 {
		t.Fatalf("HideReasoning=true 时 ThinkingDelta 不应透出, got=%v", got)
	}
	if !p.hideReasoning() {
		t.Fatal("hideReasoning() 应读回 true")
	}

	// 显式 false 是真值:恢复透出。
	p.ApplyPrefs(DesktopPrefs{HideReasoning: false})
	p.emitThinking("第三段", sink)
	if len(got) != 2 || got[1] != "第三段" {
		t.Fatalf("关掉隐藏后应恢复透出, got=%v", got)
	}
}

func TestZcodeReasoningLevelPriority(t *testing.T) {
	cases := []struct {
		name                                string
		threadEffort, desktopEff, defaultLv string
		want                                string
	}{
		{"线程显式优先", "high", "medium", "low", "high"},
		{"线程空则桌面默认", "", "medium", "low", "medium"},
		{"都空用模型 defaultLevel", "", "", "low", "low"},
		{"全空省略 options", "", "", "", ""},
	}
	for _, tc := range cases {
		if got := zcodeReasoningLevel(tc.threadEffort, tc.desktopEff, tc.defaultLv); got != tc.want {
			t.Fatalf("[%s] got=%q want=%q", tc.name, got, tc.want)
		}
	}
}

func TestZcodeModelSelectionCarriesReasoningLevel(t *testing.T) {
	// effort(线程或桌面默认)必须落进 options.reasoningLevel(GLM 系缺它会 -32603)。
	sel := zcodeModelSelection("zai-coding-cn", "glm-5.3", "high")
	if sel["providerId"] != "zai-coding-cn" || sel["modelId"] != "glm-5.3" {
		t.Fatalf("基础定位不符: %v", sel)
	}
	opts, ok := sel["options"].(map[string]string)
	if !ok || opts["reasoningLevel"] != "high" {
		t.Fatalf("reasoningLevel 应为 high: %v", sel)
	}

	// 全空:省略 options(空 = 不覆盖,沿用桌面当前会话状态)。
	sel = zcodeModelSelection("p", "m", "")
	if _, exists := sel["options"]; exists {
		t.Fatalf("level 为空时不应带 options: %v", sel)
	}

	// 整体可 JSON 序列化(协议参数以 JSON-RPC 直传,形状必须稳定)。
	raw, err := json.Marshal(sel)
	if err != nil || len(raw) == 0 {
		t.Fatalf("选择对象应可序列化: %v %v", string(raw), err)
	}
}

func TestZcodeApplyPrefsSnapshotSemantics(t *testing.T) {
	p := NewZcodeProvider()
	p.ApplyPrefs(DesktopPrefs{Effort: "high", HideReasoning: true})
	if p.desktopEffort() != "high" || !p.hideReasoning() {
		t.Fatalf("内存态读回不符: effort=%q hide=%v", p.desktopEffort(), p.hideReasoning())
	}

	// 完整快照覆盖:只带审批策略/沙箱的新快照把其余字段清回零值。
	// 审批策略/沙箱在 zcode 侧没有消费点(权限模式固定 mode:"build",红线),
	// 收下不炸、也不影响 effort/hide 之外的可观察状态。
	p.ApplyPrefs(DesktopPrefs{ApprovalPolicy: "never", Sandbox: "workspace-write"})
	if p.desktopEffort() != "" || p.hideReasoning() {
		t.Fatalf("ApplyPrefs 必须整快照覆盖: effort=%q hide=%v", p.desktopEffort(), p.hideReasoning())
	}
}
