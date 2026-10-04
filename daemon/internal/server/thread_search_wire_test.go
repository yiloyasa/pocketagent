// thread_search_wire_test.go —— 会话搜索增强的 wire 兼容钉子
// (2026-10-04 会话选择与历史体验计划 Task 6,字段号分配见 proto/FIELD_ALLOCATIONS.md)。
//
// 【为什么只钉 proto 层】扫描/范围计算/降级口径归 thread_search.go 的 Task 6 接线;
// 这里钉的是"只增不改"的兼容语义本身:
//  1. ThreadMessageHit 旧字段(1~4)零改动 + 新字段(6~9)往返保真 —— 搜索定位
//     主键从 sequence 换到 message_id 的前提是 6~9 号位稳定;
//  2. 一条消息多个 MatchRange —— 多处命中全量保真,高亮靠它;
//  3. SearchThreadMessagesResponse 信封(has_more/searched_messages/scope)往返,
//     scope 两种口径(full_thread/local_snapshot)都必须能独立表达;
//  4. "旧客户端忽略新字段"模拟 —— 旧 daemon 回包(只有 1~4)与新客户端解析
//     互不破坏:字段缺席/多余都不影响其余字段。
package server

import (
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// TestThreadMessageHitSearchIdentityWireRoundTrip 命中卡全字段往返:
// 旧字段 1~4(语义一字未动)+ 新字段 6~9(message_id/turn_id/ranges/
// from_loaded_history)。message_id 是定位主键,任何往返丢失都会让
// "加载更早/fork/rollback 之后跳错消息"。
func TestThreadMessageHitSearchIdentityWireRoundTrip(t *testing.T) {
	in := &pb.ThreadMessageHit{
		// 旧字段 1~4(兼容定位,语义一字未动)
		Sequence:        17,
		Role:            "assistant",
		Snippet:         "…重试第 2 次后仍是 502,建议检查网关配置…",
		CreatedAtUnixMs: 1728000000000,
		// 新字段 6~9(命中身份与范围)
		MessageId: "msg-0f3e",
		TurnId:    "turn-9a1",
		Ranges: []*pb.MatchRange{
			{Start: 120, End: 125}, // 同一消息内的第一处命中
			{Start: 310, End: 315}, // 第二处命中(多次出现全量给)
			{Start: 480, End: 485}, // 第三处
		},
		FromLoadedHistory: true,
	}
	wire, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got pb.ThreadMessageHit
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if !proto.Equal(in, &got) {
		t.Fatalf("命中卡往返不一致:\n want %+v\n got  %+v", in, &got)
	}
	if got.GetSequence() != 17 || got.GetSnippet() == "" || got.GetCreatedAtUnixMs() == 0 {
		t.Fatalf("旧字段(1~4)往返丢失: %+v", &got)
	}
	if got.GetMessageId() != "msg-0f3e" || got.GetTurnId() != "turn-9a1" ||
		!got.GetFromLoadedHistory() {
		t.Fatalf("新字段(6~9)往返丢失: %+v", &got)
	}
	if len(got.GetRanges()) != 3 ||
		got.GetRanges()[0].GetStart() != 120 || got.GetRanges()[0].GetEnd() != 125 ||
		got.GetRanges()[2].GetStart() != 480 || got.GetRanges()[2].GetEnd() != 485 {
		t.Fatalf("多 MatchRange 往返丢失: %+v", got.GetRanges())
	}

	// 反向形态:命中来自未下载历史(from_loaded_history=false 也要保真,
	// 客户端据此先请求目标片段再跳,不能误显示"就在眼前")。
	remote := &pb.ThreadMessageHit{
		Sequence: 4, Role: "user", Snippet: "…tls 握手失败…",
		MessageId: "msg-77c1", TurnId: "turn-2b",
		Ranges: []*pb.MatchRange{{Start: 0, End: 3}}, // 首字符起命中(边界)
	}
	wire2, err := proto.Marshal(remote)
	if err != nil {
		t.Fatalf("未下载历史形态 marshal 失败: %v", err)
	}
	var got2 pb.ThreadMessageHit
	if err := proto.Unmarshal(wire2, &got2); err != nil {
		t.Fatalf("未下载历史形态 unmarshal 失败: %v", err)
	}
	if !proto.Equal(remote, &got2) {
		t.Fatalf("未下载历史形态往返不一致:\n want %+v\n got  %+v", remote, &got2)
	}
	if got2.GetFromLoadedHistory() {
		t.Fatalf("from_loaded_history=false 往返失真: %+v", &got2)
	}
}

// TestThreadMessageHitPartialWireKeepsOldFields "旧客户端忽略新字段"模拟:
// 旧 daemon 回包只有旧字段且可能缺项(刻意跳过 2、4)+ 新客户端在同一份 wire
// 上读取 —— 在场字段原样、缺席字段零值、新字段(6~9)整体缺席不得报错。
// 真正的旧 APK 解析新回包时按 proto3 未知字段语义跳过 6~9,与本测试同一条
// wire 规则的另一面:兼容是双向的,钉住单面即可。
func TestThreadMessageHitPartialWireKeepsOldFields(t *testing.T) {
	oldDaemonHit := &pb.ThreadMessageHit{
		Sequence: 3,
		Snippet:  "…flutter build 出现 ndk 版本冲突…",
		// Role(2)/CreatedAtUnixMs(4) 缺席:旧 daemon 缺项必须不炸解析
	}
	wire, err := proto.Marshal(oldDaemonHit)
	if err != nil {
		t.Fatalf("旧 daemon 回包 marshal 失败: %v", err)
	}
	var got pb.ThreadMessageHit
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("旧 daemon 回包 unmarshal 失败: %v", err)
	}
	if got.GetSequence() != 3 || got.GetSnippet() == "" {
		t.Fatalf("在场旧字段往返丢失: %+v", &got)
	}
	if got.GetRole() != "" || got.GetCreatedAtUnixMs() != 0 {
		t.Fatalf("缺席旧字段应保持零值: %+v", &got)
	}
	if got.GetMessageId() != "" || got.GetTurnId() != "" ||
		len(got.GetRanges()) != 0 || got.GetFromLoadedHistory() {
		t.Fatalf("旧 daemon 回包不应出现新字段: %+v", &got)
	}
	// 旧形态退路成立:sequence 兼容定位仍然可用(客户端退回下标兜底的依据)。
	if got.GetSequence() < 0 {
		t.Fatalf("兼容定位字段失真: %+v", &got)
	}
}

// TestSearchThreadMessagesResponseEnvelopeWireRoundTrip 回包信封往返:
// hits(带多 MatchRange)+ has_more + searched_messages + scope 两种口径
// (full_thread / local_snapshot)各自独立表达 —— "远端搜索失败、本地未下载、
// 确实无结果"三态可区分的前提是 scope/has_more 不被吞掉。
func TestSearchThreadMessagesResponseEnvelopeWireRoundTrip(t *testing.T) {
	fullThread := &pb.SearchThreadMessagesResponse{
		Hits: []*pb.ThreadMessageHit{
			{
				Sequence: 17, Role: "assistant",
				Snippet:   "…重试第 2 次后仍是 502…",
				MessageId: "msg-0f3e", TurnId: "turn-9a1",
				Ranges:            []*pb.MatchRange{{Start: 120, End: 125}, {Start: 310, End: 315}},
				FromLoadedHistory: true,
			},
			{
				Sequence: 41, Role: "user",
				Snippet:   "…502 又出现了…",
				MessageId: "msg-88aa", TurnId: "turn-22f",
				Ranges: []*pb.MatchRange{{Start: 2, End: 5}},
			},
		},
		HasMore:          true, // 还有未返回命中(真分页位,客户端不得假设恒 false)
		SearchedMessages: 87,   // 本次实扫条数(与命中数无关)
		Scope:            "full_thread",
	}
	wire, err := proto.Marshal(fullThread)
	if err != nil {
		t.Fatalf("full_thread 形态 marshal 失败: %v", err)
	}
	var got pb.SearchThreadMessagesResponse
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("full_thread 形态 unmarshal 失败: %v", err)
	}
	if !proto.Equal(fullThread, &got) {
		t.Fatalf("full_thread 形态往返不一致:\n want %+v\n got  %+v", fullThread, &got)
	}
	if !got.GetHasMore() || got.GetSearchedMessages() != 87 || got.GetScope() != "full_thread" {
		t.Fatalf("信封字段(full_thread)往返丢失: %+v", &got)
	}
	if len(got.GetHits()) != 2 || len(got.GetHits()[0].GetRanges()) != 2 ||
		len(got.GetHits()[1].GetRanges()) != 1 {
		t.Fatalf("hits/多 MatchRange 往返丢失: %+v", &got)
	}

	// local_snapshot 口径:命中都在本地快照里,has_more=false 也要如实保真。
	localSnapshot := &pb.SearchThreadMessagesResponse{
		Hits: []*pb.ThreadMessageHit{{
			Sequence: 5, Role: "user",
			Snippet:   "…本地快照里的旧记录…",
			MessageId: "msg-l1", TurnId: "turn-l1",
			Ranges: []*pb.MatchRange{{Start: 6, End: 10}},
		}},
		HasMore:          false,
		SearchedMessages: 5,
		Scope:            "local_snapshot",
	}
	wire2, err := proto.Marshal(localSnapshot)
	if err != nil {
		t.Fatalf("local_snapshot 形态 marshal 失败: %v", err)
	}
	var got2 pb.SearchThreadMessagesResponse
	if err := proto.Unmarshal(wire2, &got2); err != nil {
		t.Fatalf("local_snapshot 形态 unmarshal 失败: %v", err)
	}
	if !proto.Equal(localSnapshot, &got2) {
		t.Fatalf("local_snapshot 形态往返不一致:\n want %+v\n got  %+v", localSnapshot, &got2)
	}
	if got2.GetHasMore() || got2.GetScope() != "local_snapshot" || got2.GetSearchedMessages() != 5 {
		t.Fatalf("信封字段(local_snapshot)往返丢失: %+v", &got2)
	}

	// 旧 daemon 形态:只有 hits(1),信封字段整体缺席 = 旧 daemon,
	// 新客户端按"未知处理",不得把缺 scope 臆断成 full_thread。
	oldDaemon, err := proto.Marshal(&pb.SearchThreadMessagesResponse{
		Hits: []*pb.ThreadMessageHit{{Sequence: 0, Role: "user", Snippet: "…旧回包…"}},
	})
	if err != nil {
		t.Fatalf("旧 daemon 形态 marshal 失败: %v", err)
	}
	var gotOld pb.SearchThreadMessagesResponse
	if err := proto.Unmarshal(oldDaemon, &gotOld); err != nil {
		t.Fatalf("旧 daemon 形态 unmarshal 失败: %v", err)
	}
	if len(gotOld.GetHits()) != 1 || gotOld.GetHasMore() ||
		gotOld.GetSearchedMessages() != 0 || gotOld.GetScope() != "" {
		t.Fatalf("旧 daemon 形态失真(信封字段应整体缺席): %+v", &gotOld)
	}
}
