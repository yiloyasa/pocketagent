// collab_session_wire_test.go —— 协同协议增量的 wire 兼容钉子
// (2026-10-04 协同闭环计划 Task 1,字段号分配见 proto/FIELD_ALLOCATIONS.md)。
//
// 【为什么只钉 proto 层】本文件不构造协同会话的任何 daemon 侧实现 —— 那是
// collab_session_test.go 们的职责。这里钉的是"协议形状"本身:
//  1. 新消息全字段 marshal→unmarshal 保真 —— 任何字段在序列化往返后变值/
//     丢失,都说明字段号或类型被破坏;协同状态要靠快照+事件流恢复,保真是底线。
//  2. 全零值消息合法 —— 旧 daemon 返回的空快照必须能被新客户端安静地解析,
//     不能炸(与"旧字段解码不丢失,新字段被旧客户端忽略"互为镜像)。
//  3. kind 的字符串词表 —— WatchCollabSession 事件种类按 string 口径透传。
//
// 【说明】daemon/internal/server 目前没有 ACCEPTED/RUNNING/... 的 Go 常量
// (实现归后续任务),故本文件只在 proto 层钉词表:测试里的字面量即协议约定,
// daemon 侧实现落地时应引用/复归此处词表,不得另造一套。
package server

import (
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// TestCollabSessionSnapshotWireRoundTrip 覆盖会话快照 + 成员卡片全字段保真,
// 包括成员卡上内嵌的 pending_approvals(复用既有 ApprovalRequest,审批通道
// 不新增 —— 这是协议层面就冻结的决定,字段被生成器改名/改号在这里就红)。
func TestCollabSessionSnapshotWireRoundTrip(t *testing.T) {
	snap := &pb.CollabSessionSnapshot{
		SessionId:       "sess-1",
		Instruction:     "修复登录页的超时问题",
		Status:          "running",
		Workspace:       `G:\repo`,
		CreatedAtUnixMs: 1728000000000,
		UpdatedAtUnixMs: 1728000001000,
		Error:           "",
		Members: []*pb.CollabMemberSnapshot{
			{
				AgentId:  "pi",
				RoleId:   "reviewer",
				ThreadId: "th-member-1",
				Status:   "WAITING_APPROVAL",
				ModelId:  "pi/test-model",
				Effort:   "high",
				Progress: "等待审批:要写入 src/login.go",
				Summary:  "", // 等待中的成员没有终稿,不允许伪造
				Error:    "",
				PendingApprovals: []*pb.ApprovalRequest{
					{RequestId: "req-1", Reason: "需要写入文件"},
				},
				Workspace: `G:\repo\.worktrees\th-member-1`,
			},
			{
				AgentId:  "codex",
				RoleId:   "", // 未用角色也是合法态
				ThreadId: "th-member-2",
				Status:   "COMPLETED",
				Summary:  "已完成修复并附测试证据",
			},
		},
	}
	wire, err := proto.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var got pb.CollabSessionSnapshot
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if !proto.Equal(snap, &got) {
		t.Fatalf("全字段往返后不一致:\n want %+v\n got  %+v", snap, &got)
	}
	// 关键字段逐一点名:proto.Equal 挡住改号/改型,这里挡住"字段被静默挪进
	//别的消息"这类 Equal 仍绿的形变 —— 恢复逻辑按这些路径取值。
	m := got.GetMembers()[0]
	if m.GetThreadId() != "th-member-1" || m.GetStatus() != "WAITING_APPROVAL" ||
		len(m.GetPendingApprovals()) != 1 || m.GetPendingApprovals()[0].GetRequestId() != "req-1" {
		t.Fatalf("成员卡关键字段保真失败: %+v", m)
	}
}

// TestCollabSessionSnapshotZeroValueIsLegal 旧 daemon 不认识新 RPC 时,客户端
// 可能拿到的是全零快照;字段缺失必须能安静解析,不得要求任何字段非空。
func TestCollabSessionSnapshotZeroValueIsLegal(t *testing.T) {
	wire, err := proto.Marshal(&pb.CollabSessionSnapshot{})
	if err != nil {
		t.Fatalf("空快照 marshal 失败: %v", err)
	}
	var got pb.CollabSessionSnapshot
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("空快照 unmarshal 失败: %v", err)
	}
	if !proto.Equal(&pb.CollabSessionSnapshot{}, &got) {
		t.Fatalf("空快照往返后应仍是零值,得到 %+v", &got)
	}
	// 旧 daemon 可能只填了旧语义里就有的骨架字段,新字段缺席 = 零值,不报错。
	wireOld, err := proto.Marshal(&pb.CollabSessionSnapshot{SessionId: "sess-old", Instruction: "老指令"})
	if err != nil {
		t.Fatalf("骨架快照 marshal 失败: %v", err)
	}
	var gotOld pb.CollabSessionSnapshot
	if err := proto.Unmarshal(wireOld, &gotOld); err != nil {
		t.Fatalf("骨架快照 unmarshal 失败: %v", err)
	}
	if gotOld.GetSessionId() != "sess-old" || gotOld.GetInstruction() != "老指令" {
		t.Fatalf("骨架字段往返丢失: %+v", &gotOld)
	}
}

// TestCollabSessionEventKindWireVocabulary 钉 WatchCollabSession 事件流的 kind
// 字符串词表(ACCEPTED/RUNNING/COMPLETED/FAILED/CANCELLED/WAITING_APPROVAL)。
// daemon 侧常量落地前,这里就是词表的唯一出处;客户端遇到词表之外的取值必须
// 按 snapshot 兜底刷新而不是报错 —— 那是加新状态时不破坏旧客户端的逃生门。
func TestCollabSessionEventKindWireVocabulary(t *testing.T) {
	kinds := []string{"ACCEPTED", "RUNNING", "COMPLETED", "FAILED", "CANCELLED", "WAITING_APPROVAL"}
	for _, kind := range kinds {
		ev := &pb.CollabSessionEvent{
			Sequence:       7,
			SessionId:      "sess-1",
			MemberThreadId: "th-member-1",
			Kind:           kind, // kind 是 string 字段:本行能编译即钉住"不是 enum"
		}
		wire, err := proto.Marshal(ev)
		if err != nil {
			t.Fatalf("kind=%s marshal 失败: %v", kind, err)
		}
		var got pb.CollabSessionEvent
		if err := proto.Unmarshal(wire, &got); err != nil {
			t.Fatalf("kind=%s unmarshal 失败: %v", kind, err)
		}
		if got.GetKind() != kind || got.GetSequence() != 7 || got.GetSessionId() != "sess-1" {
			t.Fatalf("kind=%s 往返丢失: %+v", kind, &got)
		}
	}
	// 快照兜底帧:同一消息里 snapshot 与 member 都可缺席/在场,解析必须无副作用。
	ev := &pb.CollabSessionEvent{Kind: "SNAPSHOT", Snapshot: &pb.CollabSessionSnapshot{SessionId: "sess-1"}}
	wire, err := proto.Marshal(ev)
	if err != nil {
		t.Fatalf("SNAPSHOT 帧 marshal 失败: %v", err)
	}
	var got pb.CollabSessionEvent
	if err := proto.Unmarshal(wire, &got); err != nil {
		t.Fatalf("SNAPSHOT 帧 unmarshal 失败: %v", err)
	}
	if got.GetSnapshot().GetSessionId() != "sess-1" {
		t.Fatalf("SNAPSHOT 帧快照丢失: %+v", &got)
	}
}
