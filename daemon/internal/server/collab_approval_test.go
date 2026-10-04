package server

// collab_approval_test.go —— 协同成员的审批/部分终局/取消收敛(P1 协同闭环 Task 4)。
//
// 【为什么钉这些】协同的审批不新增通道:成员轮次把真实 request_id 挂在成员卡上,
// 手机端仍用既有 SubmitApproval(thread_id=成员子线程, request_id=卡上的那个)
// 应答。任何一条松了,界面上就会出现"永远应答不了的审批"或"等待成员把全会话
// 拖成假完成" —— 后者是本计划反复强调的红线:等待不是终局。
//
// 【无法跑测试的说明】本文件与既有 server 测试同构(echo/concurrency 同款注入
// 方式、有界轮询等待),可编译性由与现有用例同一套依赖保证;风险点见交付报告。

import (
	"context"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// askOnceProvider 请求一次工具审批,把应答结果作为终稿如实返回。
func askOnceProvider(id string) *collabTestProvider {
	return &collabTestProvider{id: id, run: func(ctx context.Context, _ agent.TurnRequest, _ agent.Sink, ask agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
		ok, note, err := ask(ctx, &pb.ToolCallBlock{Id: "call-1", Name: "write_file", JsonArgs: `{"path":"x.txt"}`})
		if err != nil {
			return nil, &pb.Usage{}, err
		}
		if ok {
			return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "已获批准,写入完成"}}}}, &pb.Usage{}, nil
		}
		return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "审批被拒:" + note}}}}, &pb.Usage{}, nil
	}}
}

// blockingProvider 等待 ctx 取消(模拟一个可被取消的长任务)。
func blockingProvider(id string) *collabTestProvider {
	return &collabTestProvider{id: id, run: func(ctx context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
		<-ctx.Done()
		return nil, &pb.Usage{}, ctx.Err()
	}}
}

// collabMemberByThread 从快照里取指定线程的成员卡。
func collabMemberByThread(snap *pb.CollabSessionSnapshot, threadID string) *pb.CollabMemberSnapshot {
	for _, m := range snap.GetMembers() {
		if m.GetThreadId() == threadID {
			return m
		}
	}
	return nil
}

// waitCollabMembers 轮询等会话快照满足条件(有界),返回最后的快照。
func waitCollabMembers(t *testing.T, s *Server, sessionID string, cond func(*pb.CollabSessionSnapshot) bool) *pb.CollabSessionSnapshot {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		snap, err := s.collabCoordinatorFor().Snapshot(context.Background(), sessionID)
		if err == nil && cond(snap) {
			return snap
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待协同快照条件超时,最后快照: %s", collabSnapDebug(s, sessionID))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// ① 审批映射与应答:成员卡出现带真实 request_id 的挂起审批(thread_id 就是
// 成员子线程);既有 SubmitApproval 应答后成员回 RUNNING 并真实完成;
// 重复提交与跨线程提交被拒。
func TestCollabMemberApprovalMapsToCardAndResolves(t *testing.T) {
	s := newCollabProvServer(t, askOnceProvider("waiter"))
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "写入文件",
		Members:     []*pb.CollabMember{{AgentId: "waiter"}},
	})
	if err != nil {
		t.Fatalf("派发失败: %v", err)
	}
	sid, th := resp.GetSessionId(), resp.GetItems()[0].GetThreadId()

	// 等成员卡挂上审批:WAITING_APPROVAL + 一条带真实 request_id 的挂起审批。
	var reqID string
	waitCollabMembers(t, s, sid, func(snap *pb.CollabSessionSnapshot) bool {
		m := collabMemberByThread(snap, th)
		if m == nil || m.GetStatus() != string(store.CollabStatusWaitingApproval) || len(m.GetPendingApprovals()) != 1 {
			return false
		}
		reqID = m.GetPendingApprovals()[0].GetRequestId()
		return reqID != ""
	})
	if reqID == "" {
		t.Fatal("成员卡上的审批必须携带真实 request_id")
	}

	// 复用既有 SubmitApproval 应答:成员回 RUNNING 并最终 COMPLETED。
	if _, err := s.SubmitApproval(context.Background(), &pb.SubmitApprovalRequest{
		ThreadId: th,
		Decision: &pb.ApprovalDecision{RequestId: reqID, Approved: true},
	}); err != nil {
		t.Fatalf("既有 SubmitApproval 应能应答协同成员审批: %v", err)
	}
	waitCollabMemberStatus(t, s, sid, th, string(store.CollabStatusCompleted))

	// 重复提交同一 request_id:审批已结束,被拒。
	if _, err := s.SubmitApproval(context.Background(), &pb.SubmitApprovalRequest{
		ThreadId: th,
		Decision: &pb.ApprovalDecision{RequestId: reqID, Approved: true},
	}); err == nil {
		t.Fatal("重复提交应被拒")
	}
	// 跨线程提交:线程上没有这个 request_id,被拒。
	if _, err := s.SubmitApproval(context.Background(), &pb.SubmitApprovalRequest{
		ThreadId: "th-别的会话",
		Decision: &pb.ApprovalDecision{RequestId: reqID, Approved: true},
	}); err == nil {
		t.Fatal("跨线程提交应被拒")
	}
}

// ② 部分终局:一个成员等审批、另一个成员完成 —— 等待的成员保持
// WAITING_APPROVAL,完成的成员如实 COMPLETED,会话整体必须是
// WAITING_APPROVAL(等待不是终局,绝不被误报成完成);审批通过后整体收敛。
func TestCollabWaitingApprovalPartialTerminal(t *testing.T) {
	s := newCollabProvServer(t, askOnceProvider("waiter"), &collabTestProvider{
		id: "echo2",
		run: func(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
			return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "B 的结论"}}}}, &pb.Usage{}, nil
		},
	})
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "一个等审批,一个先跑完",
		Members:     []*pb.CollabMember{{AgentId: "waiter"}, {AgentId: "echo2"}},
	})
	if err != nil {
		t.Fatalf("派发失败: %v", err)
	}
	sid, thA, thB := resp.GetSessionId(), resp.GetItems()[0].GetThreadId(), resp.GetItems()[1].GetThreadId()

	snap := waitCollabMembers(t, s, sid, func(snap *pb.CollabSessionSnapshot) bool {
		a, b := collabMemberByThread(snap, thA), collabMemberByThread(snap, thB)
		return a != nil && b != nil &&
			a.GetStatus() == string(store.CollabStatusWaitingApproval) && len(a.GetPendingApprovals()) == 1 &&
			b.GetStatus() == string(store.CollabStatusCompleted)
	})
	if snap.GetStatus() != string(store.CollabStatusWaitingApproval) {
		t.Fatalf("一个成员等审批、其他成员已完成时,会话应如实显示 WAITING_APPROVAL,实际 %s", snap.GetStatus())
	}
	if b := collabMemberByThread(snap, thB); b.GetSummary() == "" {
		t.Fatal("完成成员的结果不应被等待成员掩盖")
	}

	reqID := collabMemberByThread(snap, thA).GetPendingApprovals()[0].GetRequestId()
	if _, err := s.SubmitApproval(context.Background(), &pb.SubmitApprovalRequest{
		ThreadId: thA,
		Decision: &pb.ApprovalDecision{RequestId: reqID, Approved: true},
	}); err != nil {
		t.Fatalf("审批应答失败: %v", err)
	}
	waitCollabMemberStatus(t, s, sid, thA, string(store.CollabStatusCompleted))
	final := waitCollabMembers(t, s, sid, func(snap *pb.CollabSessionSnapshot) bool {
		return snap.GetStatus() == string(store.CollabStatusCompleted)
	})
	if final.GetError() != "" {
		t.Fatalf("全部完成后会话级错误应为空,实际 %q", final.GetError())
	}
}

// ③ 审批过期:内存里的审批已不在时提交被拒,且成员卡上遗留的挂起审批被
// 摘掉(卡片与真相收敛,不留"永远应答不了的审批")。
func TestCollabApprovalExpiredClearsMemberCard(t *testing.T) {
	s := newCollabProvServer(t, askOnceProvider("waiter"))
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "写入文件",
		Members:     []*pb.CollabMember{{AgentId: "waiter"}},
	})
	if err != nil {
		t.Fatalf("派发失败: %v", err)
	}
	sid, th := resp.GetSessionId(), resp.GetItems()[0].GetThreadId()
	var reqID string
	waitCollabMembers(t, s, sid, func(snap *pb.CollabSessionSnapshot) bool {
		m := collabMemberByThread(snap, th)
		if m == nil || len(m.GetPendingApprovals()) != 1 {
			return false
		}
		reqID = m.GetPendingApprovals()[0].GetRequestId()
		return reqID != ""
	})

	// 模拟"内存里的审批已经不在了"(轮次收尾/重启后的过期窗口)。
	s.approvalSessions.Delete(th)
	if _, err := s.SubmitApproval(context.Background(), &pb.SubmitApprovalRequest{
		ThreadId: th,
		Decision: &pb.ApprovalDecision{RequestId: reqID, Approved: true},
	}); err == nil {
		t.Fatal("过期审批应被拒")
	}
	// 被拒的同时,成员卡上的遗留审批被摘掉。
	waitCollabMembers(t, s, sid, func(snap *pb.CollabSessionSnapshot) bool {
		m := collabMemberByThread(snap, th)
		return m != nil && len(m.GetPendingApprovals()) == 0
	})

	// 收尾:取消仍在等审批的成员轮次,别让 2 分钟审批等待拖住测试清理。
	if _, err := s.CancelCollabMember(context.Background(), &pb.CancelCollabMemberRequest{SessionId: sid, ThreadId: th}); err != nil {
		t.Fatalf("取消成员失败: %v", err)
	}
	waitCollabMemberStatus(t, s, sid, th, string(store.CollabStatusCancelled))
}

// ④ 会话级取消含等待审批的成员:WAITING_APPROVAL 的成员随取消指令退出审批
// 等待并收敛为 CANCELLED,在跑成员同步收敛,会话整体 CANCELLED —— 取消
// 不因"有成员停在审批上"而卡死。
func TestCollabCancelSessionWithWaitingApprovalMember(t *testing.T) {
	s := newCollabProvServer(t, askOnceProvider("waiter"), blockingProvider("blocker"))
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "都停下",
		Members:     []*pb.CollabMember{{AgentId: "waiter"}, {AgentId: "blocker"}},
	})
	if err != nil {
		t.Fatalf("派发失败: %v", err)
	}
	sid, thA, thB := resp.GetSessionId(), resp.GetItems()[0].GetThreadId(), resp.GetItems()[1].GetThreadId()
	waitCollabMembers(t, s, sid, func(snap *pb.CollabSessionSnapshot) bool {
		a, b := collabMemberByThread(snap, thA), collabMemberByThread(snap, thB)
		return a != nil && b != nil &&
			a.GetStatus() == string(store.CollabStatusWaitingApproval) &&
			b.GetStatus() == string(store.CollabStatusRunning)
	})

	snap, err := s.CancelCollabSession(context.Background(), &pb.CancelCollabSessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("会话取消应在有界等待内收敛(含等待审批的成员): %v", err)
	}
	if snap.GetStatus() != string(store.CollabStatusCancelled) {
		t.Fatalf("取消后会话应为 CANCELLED,实际 %s", snap.GetStatus())
	}
	for _, m := range snap.GetMembers() {
		if m.GetStatus() != string(store.CollabStatusCancelled) {
			t.Fatalf("取消后成员 %s 应为 CANCELLED,实际 %s", m.GetThreadId(), m.GetStatus())
		}
		if len(m.GetPendingApprovals()) != 0 {
			t.Fatalf("已取消成员 %s 不应还挂着审批", m.GetThreadId())
		}
	}
}
