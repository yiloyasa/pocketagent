package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	"github.com/pocketagent/daemon/internal/eventstore"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

type disruptedTurnStream struct {
	*feedTurnStream
	send func(*pb.TurnServerMessage) error
}

func (s *disruptedTurnStream) Send(frame *pb.TurnServerMessage) error {
	return s.send(frame)
}

// Exercise StartTurn -> real events.jsonl -> reopen -> replay RPC, not merely
// an in-memory recorder. The blocked sender remains blocked until the RPC has
// returned, proving both provider execution and finalization are independent.
func TestTurnRecoveryFromRealEventFile(t *testing.T) {
	for _, tc := range []struct {
		name, wire string
		fail       bool
	}{
		{"disconnect_completed", "disconnect", false},
		{"disconnect_failed", "disconnect", true},
		{"send_error_completed", "error", false},
		{"blocked_overflow_completed", "blocked", false},
		{"blocked_overflow_failed", "blocked", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const deltas = 300 // greater than the live sender's 256-frame buffer
			dataDir := t.TempDir()
			st, err := store.NewFileStore(dataDir)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.Create(&store.Thread{ID: "t", Title: "recovery", ModelID: "echo/m"}); err != nil {
				t.Fatal(err)
			}
			firstSend := make(chan int64, 1)
			sendEntered, sendReleased, sendReturned := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(sendReleased) }) }
			defer release()
			ctx, disconnect := context.WithCancel(context.Background())
			defer disconnect()
			runReturned := make(chan struct{})
			p := &concurrencyProvider{run: func(ctx context.Context, _ agent.TurnRequest, sink agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
				defer close(runReturned)
				select {
				case <-sendEntered:
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				}
				for i := 0; i < deltas; i++ {
					sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: fmt.Sprint(i)}}})
				}
				blocks := []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "recoverable result"}}}}
				if tc.fail {
					return blocks, nil, errors.New("native failure after partial output")
				}
				return blocks, &pb.Usage{InputTokens: 3, OutputTokens: 7}, nil
			}}
			s := New(Config{Store: st, Providers: map[string]agent.Provider{"echo": p}, DataDir: dataDir})
			defer s.events.Close()
			f := &disruptedTurnStream{feedTurnStream: concurrencyStream("recover")}
			f.ctx = ctx
			// Receive EOF must not cancel execution.
			close(f.in)
			f.send = func(frame *pb.TurnServerMessage) error {
				defer close(sendReturned)
				firstSend <- frame.Sequence
				if tc.wire == "disconnect" {
					disconnect()
				}
				close(sendEntered)
				if tc.wire == "blocked" {
					<-sendReleased
				}
				return errors.New("phone unavailable")
			}
			done := make(chan error, 1)
			go func() { done <- s.StartTurn(f) }()
			defer func() {
				if cancel := s.activeTurns.Cancel("t"); cancel != nil {
					cancel()
				}
			}()
			select {
			case <-runReturned:
			case <-time.After(5 * time.Second):
				t.Fatal("wire failure blocked provider execution")
			}
			awaitTurn(t, done)
			if tc.wire == "blocked" {
				select {
				case <-sendReturned:
					t.Fatal("test sender was not actually blocked")
				default:
				}
			}
			release()
			select {
			case <-sendReturned:
			case <-time.After(time.Second):
				t.Fatal("sender did not exit after release")
			}
			after := <-firstSend
			if after != 1 {
				t.Fatalf("initial delivered sequence = %d", after)
			}
			info, err := os.Stat(filepath.Join(dataDir, eventstore.FileName))
			if err != nil || info.Size() == 0 {
				t.Fatalf("real event file missing/empty: %v", err)
			}
			if err := s.events.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := eventstore.Open(dataDir)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			recovery := &Server{events: reopened}
			replay := &replayStream{ctx: context.Background()}
			if err := recovery.GetThreadEventsSince(&pb.GetThreadEventsSinceRequest{ThreadId: "t", AfterSequence: after}, replay); err != nil {
				t.Fatal(err)
			}
			frames := replay.snapshot()
			if len(frames) != deltas+1 {
				t.Fatalf("recovered %d frames, want %d", len(frames), deltas+1)
			}
			for i, frame := range frames {
				if frame.Sequence != after+int64(i)+1 {
					t.Fatalf("replay sequence gap at %d: %d", i, frame.Sequence)
				}
				if i < deltas && frame.GetTextDelta().GetDelta() != fmt.Sprint(i) {
					t.Fatalf("replay content/order changed at %d", i)
				}
			}
			last := frames[len(frames)-1]
			if tc.fail {
				if last.GetFailed().GetPartialText() != "recoverable result" {
					t.Fatalf("lost failed terminal/partial output: %v", last)
				}
			} else if last.GetCompleted() == nil || last.GetCompleted().GetFinalBlocks()[0].GetText().GetText() != "recoverable result" {
				t.Fatalf("lost completed terminal/result: %v", last)
			}
			all := reopened.ReplayTurnFrames("t", 0)
			for _, e := range all {
				if e.TurnID == "" || e.TurnID != all[0].TurnID {
					t.Fatal("replay crossed turn identity")
				}
			}
			if hosts := reopened.ReplayHostEvents(0); len(hosts) != 1 || hosts[0].Seq != last.Sequence+1 {
				t.Fatalf("lost terminal host notification: %v", hosts)
			}
			restored, err := store.NewFileStore(dataDir)
			if err != nil {
				t.Fatal(err)
			}
			thread, _ := restored.Get("t")
			if thread.Status != pb.ThreadStatus_THREAD_STATUS_IDLE || len(thread.Messages) != 2 ||
				thread.Messages[1].Blocks[0].GetText().GetText() != "recoverable result" {
				t.Fatal("terminal history/status did not survive reopen")
			}
		})
	}
}

type concurrencyProvider struct {
	run func(context.Context, agent.TurnRequest, agent.Sink, agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error)
}

func (*concurrencyProvider) ID() string             { return "echo" }
func (*concurrencyProvider) DisplayName() string    { return "concurrency" }
func (*concurrencyProvider) Models() []pb.ModelInfo { return nil }
func (p *concurrencyProvider) Run(c context.Context, r agent.TurnRequest, s agent.Sink, a agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return p.run(c, r, s, a)
}
func concurrencyServer(t *testing.T, p agent.Provider) *Server {
	t.Helper()
	st := mustStore(t)
	if err := st.Create(&store.Thread{ID: "t", Title: "thread", ModelID: "echo/m"}); err != nil {
		t.Fatal(err)
	}
	return New(Config{Store: st, Providers: map[string]agent.Provider{"echo": p}})
}
func concurrencyStream(text string) *feedTurnStream {
	f := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 16)}
	f.in <- textInput("t", text)
	return f
}
func awaitTurn(t *testing.T, ch <-chan error) {
	t.Helper()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("turn stuck")
	}
}

func TestReentryWaitsForOldTurnFinalization(t *testing.T) {
	oldEntered, oldCanceled, releaseOld := make(chan struct{}), make(chan struct{}), make(chan struct{})
	newEntered, releaseNew := make(chan struct{}), make(chan struct{})
	p := &concurrencyProvider{run: func(ctx context.Context, req agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
		if firstUserText(req.InputBlocks) == "old" {
			close(oldEntered)
			<-ctx.Done()
			close(oldCanceled)
			<-releaseOld // native provider needs longer than the old 300ms sleep
			return nil, nil, ctx.Err()
		}
		close(newEntered)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-releaseNew:
			return nil, &pb.Usage{}, nil
		}
	}}
	s := concurrencyServer(t, p)
	old, next := concurrencyStream("old"), concurrencyStream("new")
	defer close(old.in)
	defer close(next.in)
	d1, d2 := make(chan error, 1), make(chan error, 1)
	go func() { d1 <- s.StartTurn(old) }()
	<-oldEntered
	go func() { d2 <- s.StartTurn(next) }()
	<-oldCanceled
	overlap := false
	select {
	case <-newEntered:
		overlap = true
	case <-time.After(450 * time.Millisecond):
	}
	close(releaseOld)
	awaitTurn(t, d1)
	select {
	case <-newEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("replacement never started")
	}
	registered := s.activeTurns.Has("t")
	close(releaseNew)
	awaitTurn(t, d2)
	if overlap {
		t.Error("replacement ran before old turn finished")
	}
	if !registered {
		t.Error("old cleanup erased replacement registration")
	}
	got, _ := s.cfg.Store.Get("t")
	if len(got.Messages) != 3 {
		t.Fatalf("lost turn history: %d", len(got.Messages))
	}
}

func TestApprovalDecisionsDoNotCrossRequests(t *testing.T) {
	type result struct {
		id, note string
		approved bool
		err      error
	}
	results := make(chan result, 2)
	p := &concurrencyProvider{run: func(ctx context.Context, _ agent.TurnRequest, _ agent.Sink, approve agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
		var wg sync.WaitGroup
		for _, id := range []string{"first", "second"} {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				a, n, e := approve(ctx, &pb.ToolCallBlock{Id: id, Name: "shell"})
				results <- result{id, n, a, e}
			}(id)
		}
		wg.Wait()
		return nil, &pb.Usage{}, nil
	}}
	s := concurrencyServer(t, p)
	f := concurrencyStream("approval")
	defer close(f.in)
	done := make(chan error, 1)
	go func() { done <- s.StartTurn(f) }()
	waitFor(t, "two approvals", func() bool {
		return countFrame(f.snapshot(), func(m *pb.TurnServerMessage) bool { return m.GetApprovalRequest() != nil }) == 2
	})
	requests := []*pb.ApprovalRequest{}
	for _, m := range f.snapshot() {
		if r := m.GetApprovalRequest(); r != nil {
			requests = append(requests, r)
		}
	}
	// A stale reply must not satisfy either pending request.
	f.in <- &pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Approval{Approval: &pb.ApprovalDecision{RequestId: "expired", Approved: true, Note: "stale"}}}
	for i := len(requests) - 1; i >= 0; i-- {
		r := requests[i]
		f.in <- &pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Approval{Approval: &pb.ApprovalDecision{RequestId: r.RequestId, Approved: r.Action.Id == "first", Note: r.Action.Id}}}
	}
	awaitTurn(t, done)
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil || r.note != r.id || r.approved != (r.id == "first") {
			t.Error(fmt.Sprintf("approval crossed requests: %+v", r))
		}
	}
}
