package server

import (
	"context"
	"errors"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func TestTurnTransportPersistsAfterDisconnect(t *testing.T) {
	for _, deadContext := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		var recorded []string
		tr := newTurnTransport(ctx, func(f *pb.TurnServerMessage) {
			recorded = append(recorded, turnFrameType(f))
		}, func(*pb.TurnServerMessage) error { return errors.New("phone disconnected") })
		if deadContext {
			cancel()
		}
		tr.emit(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: "recovered"}}})
		tr.emit(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Completed{Completed: &pb.TurnCompleted{TurnId: "turn"}}})
		tr.finish()
		cancel()
		if len(recorded) != 2 || recorded[1] != "completed" {
			t.Fatalf("disconnect lost journal: %v", recorded)
		}
	}
}

func TestSlowTurnTransportDoesNotBlockProducer(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	count := 0
	tr := newTurnTransport(context.Background(), func(*pb.TurnServerMessage) { count++ },
		func(*pb.TurnServerMessage) error { <-release; return nil })
	for i := 0; i < 1000; i++ {
		tr.emit(&pb.TurnServerMessage{})
	}
	if count != 1000 || !tr.dead.Load() {
		t.Fatalf("journal=%d dead=%v", count, tr.dead.Load())
	}
	tr.mu.Lock()
	tr.closed = true
	close(tr.pending)
	tr.mu.Unlock()
}
