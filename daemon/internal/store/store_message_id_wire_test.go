// store_message_id_wire_test.go —— 消息稳定身份字段的 wire 兼容钉子
// (2026-10-04 会话选择与历史体验计划 Task 5,字段号分配见 proto/FIELD_ALLOCATIONS.md)。
//
// 【本文件钉什么、不钉什么】ThreadMessage 追加的 message_id(4)/turn_id(5) 是
// wire 层的 additive 字段,号位/类型一旦被挪动,客户端的稳定定位与 store 序列化
// (protojson 落盘)会同时破 —— 这里钉死往返。**不钉**迁移算法本身:旧数据按
// thread + index + created_at + 内容 hash 生成迁移 id、幂等重跑不变形、fork/
// rollback 对 id 的保持/重生成语义,都是纯 Go 层行为,归 store_message_id_test.go
// (Task 5 的 store 接线),不属 protocol steward 的 wire 契约面。
package store

import (
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// TestThreadMessageIdentityFieldsWireRoundTrip 身份字段全量往返:
// 旧字段 1~3(role/blocks/created_at,语义一字未动)+ 新字段 4~5(message_id/
// turn_id)。store 落盘走 protojson、wire 走 proto 二进制,两条编码路径共用同一
// 字段号 —— 这里钉二进制路径,任何改号/改型在此变红。
func TestThreadMessageIdentityFieldsWireRoundTrip(t *testing.T) {
	in := &pb.ThreadMessage{
		// 旧字段 1~3
		Role: "assistant",
		Blocks: []*pb.ContentBlock{
			{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "已修复:超时重试改为指数退避。"}}},
			{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "共重试 3 次,最终 200。"}}},
		},
		CreatedAtUnixMs: 1728000000000,
		// 新字段 4~5(稳定身份)
		MessageId: "msg-0f3e2a",
		TurnId:    "turn-9a1c",
	}
	wire, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got pb.ThreadMessage
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if !proto.Equal(in, &got) {
		t.Fatalf("消息往返不一致:\n want %+v\n got  %+v", in, &got)
	}
	if got.GetRole() != "assistant" || len(got.GetBlocks()) != 2 ||
		got.GetCreatedAtUnixMs() != 1728000000000 {
		t.Fatalf("旧字段(1~3)往返丢失: %+v", &got)
	}
	if got.GetMessageId() != "msg-0f3e2a" || got.GetTurnId() != "turn-9a1c" {
		t.Fatalf("身份字段(4~5)往返丢失: %+v", &got)
	}

	// user 消息 + 空 turn_id(轮次归属未知是合法状态,不是错误)也要保真:
	// message_id 在、turn_id 空的形状是迁移期/非轮内消息的真实形态。
	userMsg := &pb.ThreadMessage{
		Role:            "user",
		Blocks:          []*pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "看看 502 的原因"}}}},
		CreatedAtUnixMs: 1728000001000,
		MessageId:       "msg-b01d",
		// TurnId 刻意留空:空 = 未知,必须原样往返,不得被填成哨兵值
	}
	wire2, err := proto.Marshal(userMsg)
	if err != nil {
		t.Fatalf("user 消息 marshal 失败: %v", err)
	}
	var got2 pb.ThreadMessage
	if err := proto.Unmarshal(wire2, &got2); err != nil {
		t.Fatalf("user 消息 unmarshal 失败: %v", err)
	}
	if !proto.Equal(userMsg, &got2) {
		t.Fatalf("user 消息往返不一致:\n want %+v\n got  %+v", userMsg, &got2)
	}
	if got2.GetMessageId() != "msg-b01d" || got2.GetTurnId() != "" {
		t.Fatalf("空 turn_id 必须原样往返(空=未知,不是错误): %+v", &got2)
	}
}

// TestThreadMessageOldWireWithoutIdentity "旧客户端/旧 daemon 忽略身份字段"模拟:
// 旧数据(只有 1~3,且 blocks 可缺席)在同一条 wire 规则下解析 —— 在场字段原样、
// 缺席字段零值、身份字段整体缺席不报错。迁移器读取旧落盘数据靠的正是这条性质:
// 身份字段缺席 = 未迁移,补齐后重写;在场 = 已迁移,幂等跳过。
func TestThreadMessageOldWireWithoutIdentity(t *testing.T) {
	oldMsg := &pb.ThreadMessage{
		Role: "user",
		// Blocks(2) 缺席:旧数据缺项必须不炸解析(失败轮可能只有 role+时间戳)
		CreatedAtUnixMs: 1727990000000,
		// MessageId/TurnId 缺席 = 未迁移的旧数据
	}
	wire, err := proto.Marshal(oldMsg)
	if err != nil {
		t.Fatalf("旧数据 marshal 失败: %v", err)
	}
	var got pb.ThreadMessage
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("旧数据 unmarshal 失败: %v", err)
	}
	if got.GetRole() != "user" || got.GetCreatedAtUnixMs() != 1727990000000 {
		t.Fatalf("在场旧字段往返丢失: %+v", &got)
	}
	if len(got.GetBlocks()) != 0 {
		t.Fatalf("缺席旧字段(2)应保持零值: %+v", &got)
	}
	if got.GetMessageId() != "" || got.GetTurnId() != "" {
		t.Fatalf("未迁移旧数据不应出现身份字段: %+v", &got)
	}

	// 混合形态:部分旧字段(1、3)+ 身份字段全带 —— 已迁移数据被旧读取方
	// (按没有 4~5 的旧 schema)解析时,proto3 未知字段语义整体跳过 4~5,
	// 旧字段一个不少;本测试对"缺席 blocks"的跳过是同一条规则的两面。
	migrated := &pb.ThreadMessage{
		Role:            "assistant",
		CreatedAtUnixMs: 1728000002000,
		MessageId:       "msg-c2d2",
		TurnId:          "turn-31f0",
	}
	wire2, err := proto.Marshal(migrated)
	if err != nil {
		t.Fatalf("已迁移数据 marshal 失败: %v", err)
	}
	var got2 pb.ThreadMessage
	if err := proto.Unmarshal(wire2, &got2); err != nil {
		t.Fatalf("已迁移数据 unmarshal 失败: %v", err)
	}
	if got2.GetRole() != "assistant" || got2.GetCreatedAtUnixMs() != 1728000002000 {
		t.Fatalf("已迁移数据旧字段往返丢失: %+v", &got2)
	}
	if got2.GetMessageId() != "msg-c2d2" || got2.GetTurnId() != "turn-31f0" {
		t.Fatalf("已迁移数据身份字段往返丢失: %+v", &got2)
	}
}
