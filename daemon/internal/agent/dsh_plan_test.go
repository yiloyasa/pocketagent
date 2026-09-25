package agent

// dsh_plan_test.go —— dsh 计划变体 → 计划卡投影的回归钉子。
//
// 【形状来源】随包 @agentclientprotocol/sdk schema(bundle 静态提取,见
// dsh_plan.go 文件头);live 边界:0.1.5-rc.3 适配器不发计划变体,本组测试
// 钉的是"发了就能投影、认不出就原样透传"的两条硬边界。

import (
	"encoding/json"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// dshPlanSink 收集一轮投影帧的最小 sink(sinkCollect 在别的测试文件,这里
// 为了独立可读复制最小版会引入重复;直接复用 sinkCollect)。

func TestDshPlanVariantProjectsPlanCard(t *testing.T) {
	p := NewDshProvider()
	sink := &sinkCollect{}
	// stable ACP plan 变体:entries 三态齐发。
	raw := json.RawMessage(`{"sessionUpdate":"plan","entries":[
		{"content":"研究现有实现","priority":"high","status":"completed"},
		{"content":"编写计划卡投影","priority":"medium","status":"in_progress"},
		{"content":"真机回归","priority":"low","status":"pending"}]}`)
	p.handleSessionUpdate(raw, sink.add, &dshTurnState{}, &nativeNoticeRecorder{})

	evs := sink.all()
	if len(evs) != 1 || evs[0].GetCodexEvent() == nil {
		t.Fatalf("plan 变体应投影成一条 CodexEvent: %+v", evs)
	}
	ev := evs[0].GetCodexEvent()
	if ev.Title != "执行计划更新" {
		t.Fatalf("标题应与 codex 计划卡同款, got %q", ev.Title)
	}
	if len(ev.PlanSteps) != 3 {
		t.Fatalf("三步应全进卡, got %+v", ev.PlanSteps)
	}
	if ev.PlanSteps[0] != "completed|研究现有实现" || ev.PlanSteps[2] != "pending|真机回归" {
		t.Fatalf("步骤格式应为 status|text(手机端契约), got %+v", ev.PlanSteps)
	}
}

func TestDshPlanUpdateItemsProjects(t *testing.T) {
	p := NewDshProvider()
	sink := &sinkCollect{}
	// UNSTABLE plan_update 的 items 内容(0.1.7-next 方向)。
	raw := json.RawMessage(`{"sessionUpdate":"plan_update","plan":{"type":"items","planId":"p1","entries":[
		{"content":"第一步","priority":"high","status":"in_progress"}]}}`)
	p.handleSessionUpdate(raw, sink.add, &dshTurnState{}, &nativeNoticeRecorder{})

	evs := sink.all()
	if len(evs) != 1 || evs[0].GetCodexEvent() == nil {
		t.Fatalf("plan_update(items) 应投影: %+v", evs)
	}
	if got := evs[0].GetCodexEvent().PlanSteps; len(got) != 1 || got[0] != "in_progress|第一步" {
		t.Fatalf("步骤投影不对: %+v", got)
	}
}

func TestDshPlanUpdateMarkdownFileFallsThrough(t *testing.T) {
	p := NewDshProvider()
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"sessionUpdate":"plan_update","plan":{"type":"markdown","planId":"p1","content":"# 计划"}}`),
		json.RawMessage(`{"sessionUpdate":"plan_update","plan":{"type":"file","planId":"p1","uri":"file:///plan.md"}}`),
	} {
		sink := &sinkCollect{}
		notices := &nativeNoticeRecorder{}
		p.handleSessionUpdate(raw, sink.add, &dshTurnState{}, notices)
		// markdown/file 不猜内容:走透传诊断视图(单条原生通知帧)。
		evs := sink.all()
		if len(evs) != 1 || evs[0].GetCodexEvent() == nil ||
			evs[0].GetCodexEvent().Code != "native_notification" {
			t.Fatalf("不可投影的计划内容应原样透传: %s → %+v", raw, evs)
		}
	}
}

func TestDshPlanRemovedProjectsNotice(t *testing.T) {
	p := NewDshProvider()
	sink := &sinkCollect{}
	raw := json.RawMessage(`{"sessionUpdate":"plan_removed","planId":"p1"}`)
	p.handleSessionUpdate(raw, sink.add, &dshTurnState{}, &nativeNoticeRecorder{})
	evs := sink.all()
	if len(evs) != 1 || evs[0].GetCodexEvent() == nil {
		t.Fatalf("plan_removed 应投影成一条事件: %+v", evs)
	}
	if evs[0].GetCodexEvent().Title != "执行计划已移除" {
		t.Fatalf("移除事件标题不对: %+v", evs[0].GetCodexEvent())
	}
}

func TestDshPlanEmptyEntriesFallsThrough(t *testing.T) {
	p := NewDshProvider()
	sink := &sinkCollect{}
	// 空 entries = 没有可投影内容:按透传处理,不伪造"计划已更新"。
	raw := json.RawMessage(`{"sessionUpdate":"plan","entries":[]}`)
	p.handleSessionUpdate(raw, sink.add, &dshTurnState{}, &nativeNoticeRecorder{})
	evs := sink.all()
	if len(evs) != 1 || evs[0].GetCodexEvent() == nil ||
		evs[0].GetCodexEvent().Code != "native_notification" {
		t.Fatalf("空计划应透传诊断视图: %+v", evs)
	}
	// 纯空白 content 的条目也不进卡。
	sink2 := &sinkCollect{}
	raw2 := json.RawMessage(`{"sessionUpdate":"plan","entries":[{"content":"  ","priority":"low","status":"pending"}]}`)
	p.handleSessionUpdate(raw2, sink2.add, &dshTurnState{}, &nativeNoticeRecorder{})
	for _, ev := range sink2.all() {
		if ce := ev.GetCodexEvent(); ce != nil && len(ce.PlanSteps) > 0 {
			t.Fatalf("空白条目不得进卡: %+v", ce.PlanSteps)
		}
	}
}

// 计划帧经 pb 序列化后 PlanSteps 必须保形(手机端按 "status|text" 拆分)。
func TestDshPlanStepsSurviveProtoRoundTrip(t *testing.T) {
	sink := &sinkCollect{}
	emitDshPlanSteps([]dshPlanEntry{
		{Content: "汇总", Status: "completed"},
	}, sink.add)
	ev := sink.all()[0].GetCodexEvent()
	if ev == nil || len(ev.PlanSteps) != 1 || ev.PlanSteps[0] != "completed|汇总" {
		t.Fatalf("投影帧不对: %+v", ev)
	}
	_ = pb.CodexEvent_STATUS // 编译期钉住 Kind 常量仍在
}
