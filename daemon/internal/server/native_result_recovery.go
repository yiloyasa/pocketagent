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
