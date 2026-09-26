package server

import (
	"context"
	"log"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func (s *Server) recoverNativeResult(ctx context.Context, thread *store.Thread) *store.Thread {
	if thread == nil || thread.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
		return thread
	}
	reader, ok := s.cfg.Providers[threadAgentID(thread)].(agent.NativeTurnReader)
	if !ok {
		return thread
	}
	// Reserve against StartTurn before consulting the native journal. A running
	// local provider owns its own finalization and must never be overwritten.
	s.activeTurns.mu.Lock()
	defer s.activeTurns.mu.Unlock()
	if s.activeTurns.m[thread.ID] != nil {
		return thread
	}
	current, exists := s.cfg.Store.Get(thread.ID)
	if !exists || current.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
		if exists {
			return current
		}
		return thread
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	result, err := reader.ReadNativeTurn(ctx, current)
	if err != nil || result == nil {
		return current // Unknown is not idle, failed, or complete.
	}
	var blocks []*pb.ContentBlock
	if result.Text != "" {
		blocks = []*pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: result.Text}}}}
		current.Messages = append(current.Messages, store.StoredMessage{
			Role: "assistant", CreatedAt: result.At, Blocks: derefBlocks(blocks),
		})
	}
	current.Status = pb.ThreadStatus_THREAD_STATUS_IDLE
	current.LastTurnError = ""
	if result.Error != nil {
		current.LastTurnError = result.Error.Message
	}
	if err := s.cfg.Store.Update(current); err != nil {
		log.Printf("[recovery] 原生终局落库失败 thread=%s: %v", current.ID, err)
		return thread
	}
	turnID := result.TurnID
	if turnID == "" {
		turnID = randomID("recovered")
	}
	var frame *pb.TurnServerMessage
	if result.Error != nil {
		frame = &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Failed{Failed: &pb.TurnFailed{
			TurnId: turnID, Error: result.Error, PartialText: result.Text,
		}}}
	} else {
		frame = &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Completed{Completed: &pb.TurnCompleted{
			TurnId: turnID, FinalBlocks: blocks,
		}}}
	}
	s.recordTurnFrame(current.ID, turnID, frame)
	s.emitTurnFinished(current.ID, "已从原生 Agent 恢复任务终局")
	log.Printf("[recovery] 已恢复原生终局 thread=%s failed=%t", current.ID, result.Error != nil)
	return current
}

// backfillRestartedRunningStamp 重启后仍处于 RUNNING 的会话,回填轮次起点。
//
// 【为什么记重启时刻而不是留 0/留旧值】真实开始时刻已不可追溯(上一进程的
// 轮次随进程终止),手机端"运行 N 分钟"需要锚点 —— 本次进程的启动时刻是
// 唯一诚实的近似。无条件覆盖(而非仅在 0 时)是刻意的:上一进程留下的时刻
// 描述的是一段已死亡的运行,对"当前还挂着 RUNNING"的展示没有意义。
// 恢复出终局的会话(recoverNativeResult 落为 IDLE)不在此列,不会误盖。
func (s *Server) backfillRestartedRunningStamp(now time.Time) {
	for _, thread := range s.cfg.Store.List(true) {
		if thread.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
			continue
		}
		current := s.recoverNativeResult(context.Background(), thread)
		if current.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
			continue // 恢复出终局的会话:时刻已无展示意义,不动
		}
		current.TurnStartedUnixMs = now.UnixMilli()
		if err := s.cfg.Store.Update(current); err != nil {
			log.Printf("[recovery] 重启时刻回填失败 thread=%s: %v", current.ID, err)
		}
	}
}
