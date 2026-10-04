// turn_source_selection_wire_test.go —— P3-T2 模型选择来源接线的 wire 兼容钉子
// (TurnStarted.source=5 与 HostEvent.selection_changed=7,字段号分配见
// proto/FIELD_ALLOCATIONS.md)。
//
// 【为什么只钉 proto 层】daemon 侧的来源快照真值与 startedFrame/推送接线归
// 其他执行者;这里钉的是"协议形状":来源词表按 string 口径透传(四值 + 旧
// daemon 空串全部合法)、通知是 oneof 分支且整包(非 diff)。
// EffectiveModelSelection 本身的往返由 model_selection_wire_test.go 覆盖,
// 本文件不重复 —— 两处失效原因不同:词表/分支形状坏 → 本文件红;选择消息
// 坏 → 那边红。
package server

import (
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// TestTurnStartedSourceWireRoundTrip 覆盖 source 五值往返:四级来源词表
// (thread_override / app_default / desktop_default / native_current)+
// 空串 —— 空串是"旧 daemon 未带该字段"的真实形态,必须与四级取值一样
// 能安静往返;任何一值往返变值都说明字段号/类型被破坏。
func TestTurnStartedSourceWireRoundTrip(t *testing.T) {
	sources := []string{"", "thread_override", "app_default", "desktop_default", "native_current"}
	for _, src := range sources {
		in := &pb.TurnStarted{
			TurnId:                 "turn-1",
			ModelId:                "pi/test-model",
			ConfigurationConfirmed: true,
			Source:                 src, // string 口径:本行能编译即钉住"不是 enum"
		}
		wire, err := proto.Marshal(in)
		if err != nil {
			t.Fatalf("source=%q marshal 失败: %v", src, err)
		}
		var got pb.TurnStarted
		if err := proto.Unmarshal(wire, &got); err != nil {
			t.Fatalf("source=%q unmarshal 失败: %v", src, err)
		}
		if !proto.Equal(in, &got) {
			t.Fatalf("source=%q 往返不一致:\n want %+v\n got  %+v", src, in, &got)
		}
		if got.GetSource() != src || got.GetTurnId() != "turn-1" {
			t.Fatalf("source=%q 往返丢失: %+v", src, &got)
		}
	}
}

// TestHostEventSelectionChangedWireRoundTrip 钉 selection_changed 分支在
// HostEvent oneof 中的形状:分支往返保真 + oneof 判别正确(塞了通知就不能
// 同时还是 turn_finished)+ 与既有 turn_finished 分支互不串扰 —— 客户端
// 按 oneof 取分支,判别坏了会把"选择变了"渲染成"轮次完成"。
func TestHostEventSelectionChangedWireRoundTrip(t *testing.T) {
	in := &pb.HostEvent{
		TimestampUnixMs: 1728000000000,
		Sequence:        15,
		Event: &pb.HostEvent_SelectionChanged{
			SelectionChanged: &pb.SelectionChangedNotification{
				ThreadId: "th-1",
				// 整包真值(非 diff):来源/下一轮生效/确认态原样随行。
				Selection: &pb.EffectiveModelSelection{
					AgentId:                "pi",
					ModelId:                "", // 空串 = 跟随默认,合法状态(零值语义)
					Effort:                 "high",
					Source:                 "thread_override",
					AppliesToNextTurn:      true,
					ConfigurationConfirmed: false,
				},
			},
		},
	}
	wire, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got pb.HostEvent
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if !proto.Equal(in, &got) {
		t.Fatalf("selection_changed 往返不一致:\n want %+v\n got  %+v", in, &got)
	}
	// 分支判别:解析后必须还是 selection_changed,且绝不是 turn_finished。
	sel := got.GetSelectionChanged()
	if sel == nil {
		t.Fatalf("oneof 判别失败: 应为 selection_changed 分支,实得 %T", got.GetEvent())
	}
	if got.GetTurnFinished() != nil {
		t.Fatal("selection_changed 与 turn_finished 分支串扰: 客户端会把选择变化误报成轮次完成")
	}
	if sel.GetThreadId() != "th-1" || sel.GetSelection().GetSource() != "thread_override" ||
		!sel.GetSelection().GetAppliesToNextTurn() {
		t.Fatalf("通知整包字段往返丢失: %+v", sel)
	}
	if sel.GetSelection().GetModelId() != "" {
		t.Fatalf("空串=跟随默认是合法状态,往返后必须保持空串: %+v", sel.GetSelection())
	}

	// 对照组:既有 turn_finished 分支在新增分支后仍原样往返(只增不改的另一面)。
	finish := &pb.HostEvent{
		Sequence: 16,
		Event: &pb.HostEvent_TurnFinished{
			TurnFinished: &pb.TurnFinishedNotification{ThreadId: "th-1", Summary: "已完成"},
		},
	}
	wire2, err := proto.Marshal(finish)
	if err != nil {
		t.Fatalf("turn_finished marshal 失败: %v", err)
	}
	var gotFinish pb.HostEvent
	if err := proto.Unmarshal(wire2, &gotFinish); err != nil {
		t.Fatalf("turn_finished unmarshal 失败: %v", err)
	}
	if gotFinish.GetTurnFinished() == nil || gotFinish.GetTurnFinished().GetSummary() != "已完成" {
		t.Fatalf("既有分支被波及: %+v", &gotFinish)
	}
	if gotFinish.GetSelectionChanged() != nil {
		t.Fatal("turn_finished 帧不得带出 selection_changed 分支")
	}
}
