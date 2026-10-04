// unknown_events_wire_test.go —— 未知事件用户处理中心增量的 wire 兼容钉子
// (2026-10-04 未知事件用户处理中心计划 Task 1,字段号分配见 proto/FIELD_ALLOCATIONS.md)。
//
// 【为什么只钉 proto 层】人话文案怎么生成(PresentUnknownEvent)归 runtime 侧
// 后续任务;这里钉的是"只增不改"的兼容语义本身:
//   1. 新字段(12~17 / 7~8)全字段往返保真 —— 改号/改型在这里就红;
//   2. 旧字段(1~11 / 1~6)零改动 —— 旧字段在新消息里往返不丢;
//   3. "旧客户端忽略新字段"模拟 —— 只填部分旧字段 + 新字段的wire,
//      解析回来旧字段一个不少(真正的旧 APK 按 proto3 未知字段语义丢弃
//      新字段,同一性质:字段缺席/多余都不影响其余字段解析)。
package server

import (
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// TestUserActionDescriptorWireRoundTrip 动作描述全字段保真。safe/requires_
// confirmation/destructive 三个 bool 承载安全语义,任何一位在往返后翻转都是
// 安全回归 —— 客户端按它们决定渲染成普通按钮还是危险按钮。
func TestUserActionDescriptorWireRoundTrip(t *testing.T) {
	in := &pb.UserActionDescriptor{
		ActionId:             "inspect",
		Label:                "查看技术详情",
		Description:          "打开本次事件的原文与字段透析",
		Safe:                 true,
		RequiresConfirmation: false,
		Destructive:          false,
	}
	wire, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got pb.UserActionDescriptor
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if !proto.Equal(in, &got) {
		t.Fatalf("动作描述往返不一致:\n want %+v\n got  %+v", in, &got)
	}
	// 危险动作位同样要保真( destructive/RequiresConfirmation 反向取值)。
	danger := &pb.UserActionDescriptor{
		ActionId: "retry-later", Safe: false, RequiresConfirmation: true, Destructive: true,
	}
	wire2, err := proto.Marshal(danger)
	if err != nil {
		t.Fatalf("危险动作 marshal 失败: %v", err)
	}
	var got2 pb.UserActionDescriptor
	if err := proto.Unmarshal(wire2, &got2); err != nil {
		t.Fatalf("危险动作 unmarshal 失败: %v", err)
	}
	if !got2.GetRequiresConfirmation() || !got2.GetDestructive() || got2.GetSafe() {
		t.Fatalf("危险动作安全位往返失真: %+v", &got2)
	}
}

// TestUnknownEventSummaryWireRoundTrip 17 个字段(旧 1~11 + 新 12~17)全量往返。
// 旧字段一个不能少 —— 列表页、详情页、台账审计都按它们取值;新字段一个不能丢。
func TestUnknownEventSummaryWireRoundTrip(t *testing.T) {
	in := &pb.UnknownEventSummary{
		// 旧字段 1~11(语义一字未动)
		EventId:          "ue-1",
		Sequence:         42,
		ReceivedAtUnixMs: 1728000000000,
		SourceAgent:      "pi",
		SourceMethod:     "native/experimental_widget",
		ThreadId:         "th-1",
		NormalizedKind:   "UNKNOWN_NOTIFICATION",
		SafetyClass:      "UNKNOWN",
		ProcessingStatus: "NEW",
		Revision:         3,
		Summary:          "电脑端传来一条新消息,当前版本还没有对应的卡片显示方式。",
		// 新字段 12~17(用户处理中心)
		PlainTitle:       "电脑端的一条新消息",
		PlainExplanation: "电脑端传来一条新消息,当前版本还没有对应的卡片显示方式,原文已保留。",
		Impact:           "不影响正在进行的轮次。",
		RequiresUser:     false,
		CanOpenNative:    true,
		Actions: []*pb.UserActionDescriptor{
			{ActionId: "inspect", Label: "查看技术详情", Safe: true},
			{ActionId: "acknowledge", Label: "标记已查看", Safe: true, RequiresConfirmation: false},
		},
	}
	wire, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got pb.UnknownEventSummary
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if !proto.Equal(in, &got) {
		t.Fatalf("摘要往返不一致:\n want %+v\n got  %+v", in, &got)
	}
	if got.GetEventId() != "ue-1" || got.GetSequence() != 42 || got.GetRevision() != 3 ||
		got.GetSummary() == "" {
		t.Fatalf("旧字段(1~11)往返丢失: %+v", &got)
	}
	if got.GetPlainTitle() == "" || got.GetImpact() == "" || got.GetRequiresUser() ||
		!got.GetCanOpenNative() || len(got.GetActions()) != 2 {
		t.Fatalf("新字段(12~17)往返丢失: %+v", &got)
	}
}

// TestUnknownEventResponseWireRoundTrip 详情响应 8 字段(旧 1~6 + 新 7~8)全量往返。
func TestUnknownEventResponseWireRoundTrip(t *testing.T) {
	in := &pb.UnknownEventResponse{
		// 旧字段 1~6(语义一字未动)
		Summary:           &pb.UnknownEventSummary{EventId: "ue-1", SourceMethod: "native/x"},
		RawJson:           `{"x":1}`,
		DecodedFieldsJson: `{"x":"number"}`,
		Note:              "用户留的一句话",
		Digest:            []*pb.UnknownEventField{{Key: "x", Kind: "number", Preview: "1"}},
		Translation:       "字段 x 像是一个数字参数。",
		// 新字段 7~8(用户处理中心)
		RecommendedNextStep: "可在电脑端查看原文;待上游支持后再处理。",
		SupportBundleHint:   "导出支持包时会包含本事件的脱敏原文。",
	}
	wire, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got pb.UnknownEventResponse
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if !proto.Equal(in, &got) {
		t.Fatalf("详情响应往返不一致:\n want %+v\n got  %+v", in, &got)
	}
	if got.GetSummary().GetEventId() != "ue-1" || got.GetRawJson() == "" ||
		len(got.GetDigest()) != 1 || got.GetTranslation() == "" {
		t.Fatalf("旧字段(1~6)往返丢失: %+v", &got)
	}
	if got.GetRecommendedNextStep() == "" || got.GetSupportBundleHint() == "" {
		t.Fatalf("新字段(7~8)往返丢失: %+v", &got)
	}
}

// TestUnknownEventSummaryPartialWireKeepsOldFields "旧客户端忽略新字段"模拟:
// 只填部分旧字段 + 全部新字段的 wire,解析回来旧字段一个不少、新字段各就各位。
// 【为什么这就够了】真正的旧 APK 是按"没有 12~17 的旧 schema"解析 —— proto3
// 对未知字段(新号)一律跳过,与本测试对"缺席旧字段(4~11)"的跳过是同一条
// wire 规则的两面;除非 wire 层被破坏,否则不会出现"新字段解析弄丢旧字段"。
func TestUnknownEventSummaryPartialWireKeepsOldFields(t *testing.T) {
	in := &pb.UnknownEventSummary{
		// 旧字段只填 1、5、11(刻意跳过 2~4、6~10:缺席必须不炸解析)
		EventId:      "ue-2",
		SourceMethod: "native/unknown_request",
		Summary:      "电脑端请求了一个当前版本无法安全判断的操作,因此没有自动执行。",
		// 新字段全填
		PlainTitle:   "一个未被执行的请求",
		Impact:       "该请求未自动执行,任务未被改动。",
		RequiresUser: true,
		Actions:      []*pb.UserActionDescriptor{{ActionId: "inspect", Label: "查看技术详情", Safe: true}},
	}
	wire, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got pb.UnknownEventSummary
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	// 旧字段保真(缺席的保持零值,在场的原样回来)。
	if got.GetEventId() != "ue-2" || got.GetSourceMethod() != "native/unknown_request" ||
		got.GetSummary() == "" {
		t.Fatalf("部分旧字段往返丢失: %+v", &got)
	}
	if got.GetSequence() != 0 || got.GetSafetyClass() != "" || got.GetRevision() != 0 {
		t.Fatalf("缺席旧字段应保持零值: %+v", &got)
	}
	// 新字段保真(旧客户端视角:这些号会被整体跳过,不影响上面旧字段的读取)。
	if got.GetPlainTitle() == "" || got.GetImpact() == "" || !got.GetRequiresUser() ||
		len(got.GetActions()) != 1 {
		t.Fatalf("新字段往返丢失: %+v", &got)
	}
}
