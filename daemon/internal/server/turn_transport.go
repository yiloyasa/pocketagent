package server

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// A slow or half-open phone must not stall the native Agent's event loop.
// Persist synchronously in order, then enqueue for the single gRPC writer.
// Overflow ends live delivery; the complete journal is available for replay.
type turnTransport struct {
	mu      sync.Mutex
	dead    atomic.Bool
	closed  bool
	pending chan *pb.TurnServerMessage
	done    chan struct{}
	record  func(*pb.TurnServerMessage)
	ctx     context.Context
}

func newTurnTransport(ctx context.Context, record func(*pb.TurnServerMessage), send func(*pb.TurnServerMessage) error) *turnTransport {
	t := &turnTransport{ctx: ctx, record: record, pending: make(chan *pb.TurnServerMessage, 256), done: make(chan struct{})}
	go func() {
		defer close(t.done)
		for frame := range t.pending {
			if t.dead.Load() || ctx.Err() != nil {
				continue
			}
			if err := send(frame); err != nil {
				t.dead.Store(true)
			}
		}
	}()
	return t
}

func (t *turnTransport) emit(frame *pb.TurnServerMessage) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return // Late callbacks belong to a finished native turn.
	}
	t.record(frame)
	if t.dead.Load() || t.ctx.Err() != nil {
		return
	}
	select {
	case t.pending <- frame:
	default:
		t.dead.Store(true)
	}
}

func (t *turnTransport) finish() {
	t.mu.Lock()
	if !t.closed {
		t.closed = true
		close(t.pending)
	}
	t.mu.Unlock()
	// Returning the RPC unblocks a stalled gRPC Send; no Agent is held hostage.
	select {
	case <-t.done:
	case <-t.ctx.Done():
	case <-time.After(2 * time.Second):
		t.dead.Store(true)
	}
}

// A reply only belongs to the pending request with that exact ID. Channels are
// never closed: a concurrent late/duplicate reply is simply ignored.
type turnApprovals struct {
	mu       sync.Mutex
	pending  map[string]chan *pb.ApprovalDecision
	requests map[string]*pb.ApprovalRequest
}

func newTurnApprovals() *turnApprovals {
	return &turnApprovals{pending: make(map[string]chan *pb.ApprovalDecision), requests: make(map[string]*pb.ApprovalRequest)}
}

func (a *turnApprovals) add(id string) chan *pb.ApprovalDecision {
	a.mu.Lock()
	defer a.mu.Unlock()
	ch := make(chan *pb.ApprovalDecision, 1)
	a.pending[id] = ch
	return ch
}

func (a *turnApprovals) remove(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.pending, id)
	delete(a.requests, id)
}

func (a *turnApprovals) deliver(d *pb.ApprovalDecision) bool {
	if d == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if ch := a.pending[d.RequestId]; ch != nil {
		select {
		case ch <- d:
		default:
		}
		delete(a.pending, d.RequestId)
		delete(a.requests, d.RequestId)
		return true
	}
	return false
}

func (a *turnApprovals) attach(req *pb.ApprovalRequest) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests[req.RequestId] = req
}
func (a *turnApprovals) snapshot() []*pb.ApprovalRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]*pb.ApprovalRequest, 0, len(a.requests))
	for _, req := range a.requests {
		out = append(out, req)
	}
	return out
}
