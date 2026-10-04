// model_selection_wire_test.go —— 有效模型选择增量的 wire 兼容钉子
// (2026-10-04 会话选择与历史体验计划 Task 1,字段号分配见 proto/FIELD_ALLOCATIONS.md)。
//
// 【为什么只钉 proto 层】选择怎么解析(SelectionResolver)、快照怎么落
// (TurnSnapshot)归 server 侧 Task 2;这里钉的是"只增不改"的兼容语义本身:
//  1. EffectiveModelSelection 全字段(1~8)往返保真 —— 改号/改型在这里就红;
//  2. 零值语义钉子 —— 空 model/effort 是"跟随默认"的合法状态,不是错误;
//     wire 层必须保证空串原样往返,任何"把空串序列化成哨兵值"的改动在这里红;
//  3. ListModelsResponse(旧 1~4 + 新 5)与 Thread(旧 1~23 + 新 24)的 additive
//     往返 —— 新字段不得挤占/改动旧字段;
//  4. "旧客户端忽略新字段"模拟 —— 只填部分旧字段 + 新字段的 wire,解析回来
//     旧字段一个不少(真正的旧 APK 按 proto3 未知字段语义整体跳过新号,同一性质)。
package server

import (
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// TestEffectiveModelSelectionWireRoundTrip 选择真值 8 字段全量往返。
// source 承载"这个结论从哪一级来",applies_to_next_turn 承载"运行中切换不改写
// 正在跑的 Turn"的口径 —— 任何一位往返后翻转都是兼容性回归。
func TestEffectiveModelSelectionWireRoundTrip(t *testing.T) {
	in := &pb.EffectiveModelSelection{
		AgentId:                "pi",
		ModelId:                "pi/glm-5.3",
		Effort:                 "high",
		Source:                 "thread_override",
		AppliesToNextTurn:      true,
		ConfigurationConfirmed: true,
		Explanation:            "本会话由手机端显式切换,当前轮已开跑,将从下一轮生效。",
		AvailableEfforts:       []string{"low", "high", "max"}, // pi 动态档位实测口径
	}
	wire, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got pb.EffectiveModelSelection
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if !proto.Equal(in, &got) {
		t.Fatalf("选择真值往返不一致:\n want %+v\n got  %+v", in, &got)
	}
	if got.GetSource() != "thread_override" || !got.GetAppliesToNextTurn() ||
		!got.GetConfigurationConfirmed() || len(got.GetAvailableEfforts()) != 3 {
		t.Fatalf("关键字段往返丢失: %+v", &got)
	}

	// 反向形态:来源来自桌面默认、本轮即生效(两个 bool 反向取值也要保真)。
	desktop := &pb.EffectiveModelSelection{
		AgentId:          "codex",
		ModelId:          "codex/gpt-5.4",
		Effort:           "medium",
		Source:           "desktop_default",
		Explanation:      "桌面默认 gpt-5.4,本会话未覆盖。",
		AvailableEfforts: []string{"low", "medium", "high", "xhigh"},
	}
	wire2, err := proto.Marshal(desktop)
	if err != nil {
		t.Fatalf("桌面默认形态 marshal 失败: %v", err)
	}
	var got2 pb.EffectiveModelSelection
	if err := proto.Unmarshal(wire2, &got2); err != nil {
		t.Fatalf("桌面默认形态 unmarshal 失败: %v", err)
	}
	if !proto.Equal(desktop, &got2) {
		t.Fatalf("桌面默认形态往返不一致:\n want %+v\n got  %+v", desktop, &got2)
	}
	if got2.GetAppliesToNextTurn() || got2.GetConfigurationConfirmed() {
		t.Fatalf("bool 反向取值往返失真: %+v", &got2)
	}
}

// TestEffectiveModelSelectionEmptyMeansFollowDefault 零值语义钉子:
// 空 model_id / effort = "跟随默认",是**合法状态**不是错误 —— wire 层必须让
// 空串原样往返(绝不冒出哨兵值/默认值填充),完全空的消息也要能无损序列化。
// 客户端一旦把空串误当异常改写成具体值,用户就永远回不到"跟随默认"
// (与 SetCodexSettings.model="" 的教训同源)。
func TestEffectiveModelSelectionEmptyMeansFollowDefault(t *testing.T) {
	follow := &pb.EffectiveModelSelection{
		Source:      "app_default",
		Explanation: "跟随 App 默认模型,未单独指定思考强度。",
		// ModelId/Effort 刻意留空:这就是"跟随默认"的正确表达
	}
	wire, err := proto.Marshal(follow)
	if err != nil {
		t.Fatalf("跟随默认形态 marshal 失败(空串必须是合法取值): %v", err)
	}
	var got pb.EffectiveModelSelection
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("跟随默认形态 unmarshal 失败: %v", err)
	}
	if got.GetModelId() != "" || got.GetEffort() != "" {
		t.Fatalf("空 model/effort 必须原样往返(空=跟随默认,不是错误): got model=%q effort=%q",
			got.GetModelId(), got.GetEffort())
	}
	if got.GetSource() != "app_default" || got.GetExplanation() == "" {
		t.Fatalf("空串不得影响其余字段: %+v", &got)
	}

	// 完全零值的消息也要无损往返(0 字节 wire 是合法编码,不是错误)。
	var empty pb.EffectiveModelSelection
	wire2, err := proto.Marshal(&empty)
	if err != nil {
		t.Fatalf("零值消息 marshal 失败: %v", err)
	}
	var got2 pb.EffectiveModelSelection
	if err := proto.Unmarshal(wire2, &got2); err != nil {
		t.Fatalf("零值消息 unmarshal 失败: %v", err)
	}
	if !proto.Equal(&empty, &got2) {
		t.Fatalf("零值消息往返不一致: %+v vs %+v", &empty, &got2)
	}
}

// TestListModelsResponseEffectiveSelectionWireRoundTrip ListModelsResponse
// 旧字段 1~4 + 新字段 5(effective_selection)全量往返:新字段不得挤占旧字段。
func TestListModelsResponseEffectiveSelectionWireRoundTrip(t *testing.T) {
	in := &pb.ListModelsResponse{
		// 旧字段 1~4(语义一字未动)
		Models: []*pb.ModelInfo{
			{Id: "pi/glm-5.3", Provider: "pi", DisplayName: "GLM-5.3",
				SupportsThinking: true, Available: true, AvailableEfforts: []string{"low", "high", "max"}},
		},
		CurrentModelId: "pi/glm-5.3",
		CurrentEffort:  proto.String("high"),
		StateError:     "",
		// 新字段 5(有效模型选择)
		EffectiveSelection: &pb.EffectiveModelSelection{
			AgentId: "pi", ModelId: "pi/glm-5.3", Effort: "high",
			Source: "thread_override", ConfigurationConfirmed: true,
		},
	}
	wire, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got pb.ListModelsResponse
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if !proto.Equal(in, &got) {
		t.Fatalf("ListModelsResponse 往返不一致:\n want %+v\n got  %+v", in, &got)
	}
	if len(got.GetModels()) != 1 || got.GetCurrentModelId() != "pi/glm-5.3" ||
		got.GetCurrentEffort() != "high" {
		t.Fatalf("旧字段(1~4)往返丢失: %+v", &got)
	}
	if got.GetEffectiveSelection().GetSource() != "thread_override" {
		t.Fatalf("新字段(5)往返丢失: %+v", &got)
	}
}

// TestListModelsResponsePartialWireKeepsOldFields "旧客户端忽略新字段"模拟:
// 只填部分旧字段(1、4,刻意跳过 2~3)+ 全部新字段语义的 wire,解析回来
// 在场旧字段原样、缺席旧字段零值、新消息各就各位 —— 旧 APK 按没有字段 5 的
// 旧 schema 解析同一份 wire 时,proto3 未知字段语义会整体跳过它,与本测试
// 对"缺席旧字段"的跳过是同一条 wire 规则的两面。
func TestListModelsResponsePartialWireKeepsOldFields(t *testing.T) {
	in := &pb.ListModelsResponse{
		Models:     []*pb.ModelInfo{{Id: "codex/gpt-5.4", Available: false}}, // CurrentModelId/CurrentEffort 缺席
		StateError: "metadata unavailable",
		EffectiveSelection: &pb.EffectiveModelSelection{
			AgentId: "codex", ModelId: "codex/gpt-5.4", Effort: "medium",
			Source: "desktop_default", AppliesToNextTurn: true,
		},
	}
	wire, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got pb.ListModelsResponse
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if len(got.GetModels()) != 1 || got.GetModels()[0].GetId() != "codex/gpt-5.4" ||
		got.GetStateError() == "" {
		t.Fatalf("在场旧字段往返丢失: %+v", &got)
	}
	if got.GetCurrentModelId() != "" || got.GetCurrentEffort() != "" {
		t.Fatalf("缺席旧字段应保持零值: %+v", &got)
	}
	sel := got.GetEffectiveSelection()
	if sel == nil || sel.GetModelId() != "codex/gpt-5.4" || !sel.GetAppliesToNextTurn() {
		t.Fatalf("新字段(5)往返丢失: %+v", &got)
	}
}

// TestThreadEffectiveSelectionWireRoundTrip Thread 旧字段(刻意只填一部分,
// 跳过 3~4、13~22)+ 新字段 24 往返:会话级选择真值挂在 Thread 上时,
// 旧字段一个不能少、缺席的保持零值,消息缺席(旧 daemon)时客户端按
// model_id(5)/effort(19) 兜底 —— 该兜底依赖 5/19 号位永不被挪动。
func TestThreadEffectiveSelectionWireRoundTrip(t *testing.T) {
	started := int64(1728000000000)
	in := &pb.Thread{
		// 在场旧字段:1、2、5、6、7、8、9、11、12、16、19、23
		Id:                "th-1",
		Title:             "修复登录超时",
		ModelId:           "pi/glm-5.3",
		Status:            pb.ThreadStatus_THREAD_STATUS_RUNNING,
		CreatedAtUnixMs:   1727990000000,
		UpdatedAtUnixMs:   1728000000000,
		MessageCount:      12,
		SectionId:         "sec-work",
		Archived:          false,
		AgentId:           "pi",
		Effort:            "high",
		TurnStartedUnixMs: &started,
		// 缺席旧字段(3~4、10、13~15、17~18、20~22)必须零值往返
		// 新字段 24(有效模型选择)
		EffectiveSelection: &pb.EffectiveModelSelection{
			AgentId: "pi", ModelId: "pi/glm-5.3", Effort: "high",
			Source: "thread_override", AppliesToNextTurn: true,
			Explanation:            "运行中切换,将从下一轮生效。",
			ConfigurationConfirmed: true,
			AvailableEfforts:       []string{"low", "high", "max"},
		},
	}
	wire, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got pb.Thread
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	// 在场旧字段保真(1~23 各就各位)
	if got.GetId() != "th-1" || got.GetModelId() != "pi/glm-5.3" ||
		got.GetStatus() != pb.ThreadStatus_THREAD_STATUS_RUNNING ||
		got.GetMessageCount() != 12 || got.GetAgentId() != "pi" ||
		got.GetEffort() != "high" || got.GetTurnStartedUnixMs() != started {
		t.Fatalf("在场旧字段往返丢失: %+v", &got)
	}
	// 缺席旧字段零值(兜底口径依赖 5/19 号位稳定)
	if got.GetWorkspace() != "" || got.GetGoal() != "" || got.GetCodexThreadId() != "" ||
		got.GetGuardianDeniedReviewId() != "" || got.GetLastTurnError() != "" ||
		got.GetParentThreadId() != "" || len(got.GetPendingApprovals()) != 0 ||
		got.GetExternalSession() != nil || got.GetGoalDetail() != nil {
		t.Fatalf("缺席旧字段应保持零值: %+v", &got)
	}
	// 新字段 24 保真
	sel := got.GetEffectiveSelection()
	if sel == nil || sel.GetSource() != "thread_override" || !sel.GetAppliesToNextTurn() ||
		sel.GetExplanation() == "" || len(sel.GetAvailableEfforts()) != 3 {
		t.Fatalf("新字段(24)往返丢失: %+v", &got)
	}

	// 旧 daemon 形态:effective_selection 缺席,旧字段照常往返
	// (客户端按 model_id/effort 兜底显示,不报错)。
	oldDaemon, err := proto.Marshal(&pb.Thread{Id: "th-1", ModelId: "pi/glm-5.3", Effort: "high"})
	if err != nil {
		t.Fatalf("旧 daemon 形态 marshal 失败: %v", err)
	}
	var gotOld pb.Thread
	if err := proto.Unmarshal(oldDaemon, &gotOld); err != nil {
		t.Fatalf("旧 daemon 形态 unmarshal 失败: %v", err)
	}
	if gotOld.GetEffectiveSelection() != nil {
		t.Fatalf("旧 daemon 形态不应出现新字段: %+v", &gotOld)
	}
	if gotOld.GetModelId() != "pi/glm-5.3" || gotOld.GetEffort() != "high" {
		t.Fatalf("旧 daemon 形态兜底字段往返丢失: %+v", &gotOld)
	}
}
