// pi_prefs_effect_test.go —— Pi 侧消费桌面设置(七项偏好)的单测,不依赖 Pi 子进程。
//
// 【钉什么】
//  1. HideReasoning 开关:emitThinking 在开关打开时 ThinkingDelta 一帧不出,
//     关闭(显式 false 是真值)后恢复透出 —— 与 codex.go 的 daemon 侧过滤同语义;
//  2. ApplyPrefs 完整快照覆盖语义 + 内存态读回(desktopDefaults);
//  3. threadModelExplicit 的"跟随 vs 显式"判定(桌面默认模型只认 pi/ 前缀的依据)。
package agent

import (
	"reflect"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// recordingSink 收集 ThinkingDelta 帧的 delta 文本(其余帧忽略)。
// 【为什么用 reflect.MakeFunc】历史上 Sink 的签名含 pb 值类型,任何具名函数/方法/
// 字面量定义都会触发 vet copylocks("func passes lock by value"),测试侧用动态
// 构造绕开。现在 Sink 已改指针签名,具名闭包本身不再触发 vet,但本构造照常工作
// (args[0] 现为 *pb.TurnServerMessage),保持不动以减少改动面。
func recordingSink(out *[]string) Sink {
	return reflect.MakeFunc(reflect.TypeOf((*Sink)(nil)).Elem(), func(args []reflect.Value) []reflect.Value {
		if w, ok := args[0].Interface().(*pb.TurnServerMessage); ok {
			if td, ok2 := w.Msg.(*pb.TurnServerMessage_ThinkingDelta); ok2 && td.ThinkingDelta != nil {
				*out = append(*out, td.ThinkingDelta.Delta)
			}
		}
		return nil
	}).Interface().(Sink)
}

func TestPiHideReasoningFilter(t *testing.T) {
	p := NewPiProvider()
	var got []string
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

	// 显式 false 是真值(改回"显示推理"):恢复透出。
	p.ApplyPrefs(DesktopPrefs{HideReasoning: false})
	p.emitThinking("第三段", sink)
	if len(got) != 2 || got[1] != "第三段" {
		t.Fatalf("关掉隐藏后应恢复透出, got=%v", got)
	}
}

func TestPiApplyPrefsSnapshotAndReadback(t *testing.T) {
	p := NewPiProvider()
	p.ApplyPrefs(DesktopPrefs{
		Model:         "pi/zai-coding-cn/glm-5.3",
		Effort:        "high",
		HideReasoning: true,
	})
	model, effort := p.desktopDefaults()
	if model != "pi/zai-coding-cn/glm-5.3" || effort != "high" {
		t.Fatalf("桌面默认读回不符: model=%q effort=%q", model, effort)
	}
	if !p.hideReasoning() {
		t.Fatal("HideReasoning 读回不符")
	}

	// 完整快照语义:只带部分字段的快照会覆盖掉其余字段(照单全收,
	// 与 codex 的 ApplyPrefs 同纪律) —— 否则"改回跟随桌面"永远回不来。
	p.ApplyPrefs(DesktopPrefs{Effort: "low"})
	if model, effort := p.desktopDefaults(); model != "" || effort != "low" {
		t.Fatalf("ApplyPrefs 必须整快照覆盖: model=%q effort=%q", model, effort)
	}
	if p.hideReasoning() {
		t.Fatal("上一份快照的 HideReasoning 应被覆盖回 false")
	}
}

func TestPiThreadModelExplicit(t *testing.T) {
	cases := []struct {
		modelID string
		want    bool
		reason  string
	}{
		{"", false, "未指定 = 跟随"},
		{"default", false, "显式 default = 跟随"},
		{"pi/default", false, "pi 前缀 + default = 跟随"},
		{"glm/glm-4.7", false, "非 pi 前缀不属于 Pi 的选择 = 跟随"},
		{"pi/zai-coding-cn/glm-5.3", true, "显式 pi 模型"},
	}
	for _, tc := range cases {
		if got := threadModelExplicit(&store.Thread{ModelID: tc.modelID}); got != tc.want {
			t.Fatalf("ModelID=%q: got=%v want=%v(%s)", tc.modelID, got, tc.want, tc.reason)
		}
	}
}
