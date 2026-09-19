// event_sequence_test.go —— P1-5 server 侧接线的回归钉子。
//
// 【测什么】纯 eventstore 单测钉不住"调用点被人删掉":
//  1. StartTurn 发出的每一帧都带单调 seq 且已落盘(帧序 = seq 序 = 回放序);
//  2. GetThreadEventsSince 按线程回放、after 过滤、回放帧携带原 seq;
//  3. after 落入轮转丢弃区间 / 事件库为空而手机持序号 → 只回 ResyncRequired;
//  4. 轮次终局产生带 seq 的主机事件(WatchEvents 补发的数据源)。
package server

import (
	"context"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// newEventTestServer 带 DataDir(事件真实落盘)的测试 server。
func newEventTestServer(t *testing.T, p *seqProvider) (*Server, *store.Thread) {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	srv := New(Config{Store: fs, Providers: map[string]agent.Provider{"echo": p}, DataDir: t.TempDir()})
	// Windows 上打开中的 events.jsonl 会锁住临时目录,测试收尾必须关句柄。
	t.Cleanup(func() { _ = srv.events.Close() })
	th := &store.Thread{
		ID: "th-ev", Title: "续传测试", Workspace: t.TempDir(),
		ModelID: "echo/m", Status: pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	return srv, th
}

// runEchoTurn 跑完一轮 echo 轮次(进入 → 放行 → 终局),返回手机收到的帧。
func runEchoTurn(t *testing.T, srv *Server, th *store.Thread, text string) []*pb.TurnServerMessage {
	t.Helper()
	p := srv.cfg.Providers["echo"].(*seqProvider)
	stream := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 8)}
	stream.push(t, textInput(th.ID, text))
	done := make(chan error, 1)
	go func() { done <- srv.StartTurn(stream) }()
	<-p.entered
	p.release <- struct{}{}
	close(stream.in)
	if err := <-done; err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	return stream.snapshot()
}

// TestStartTurnFramesCarrySequenceAndPersist 每帧带单调 seq 且已落盘:
// 重连回放能一字不差地补出手机断线期间错过的帧。
func TestStartTurnFramesCarrySequenceAndPersist(t *testing.T) {
	p := &seqProvider{id: "echo", entered: make(chan struct{}, 8), release: make(chan struct{}, 8)}
	srv, th := newEventTestServer(t, p)
	frames := runEchoTurn(t, srv, th, "你好")

	if len(frames) < 3 {
		t.Fatalf("帧数 = %d, 至少应有 Started/TextDelta/Completed", len(frames))
	}
	// 帧上 seq 严格单调(从 1 起,不跳号 —— 测试里没有并发发送方)
	prev := int64(0)
	for i, f := range frames {
		if f.Sequence != prev+1 {
			t.Fatalf("帧 %d 的 seq = %d, want %d(单调连续)", i, f.Sequence, prev+1)
		}
		prev = f.Sequence
	}
	if last := frames[len(frames)-1]; last.GetCompleted() == nil {
		t.Fatalf("末帧应为 Completed, got %s", frameNames(frames))
	}
	// 落盘回放与手机收到的帧完全对齐(seq 与 oneof 分支都一致)
	replayed := srv.events.ReplayTurnFrames(th.ID, 0)
	if len(replayed) != len(frames) {
		t.Fatalf("落盘帧数 = %d, 手机收到 %d", len(replayed), len(frames))
	}
	for i := range frames {
		if replayed[i].Seq != frames[i].Sequence {
			t.Fatalf("第 %d 帧: 落盘 seq=%d, 帧 seq=%d", i, replayed[i].Seq, frames[i].Sequence)
		}
	}
}

// TestGetThreadEventsSinceReplayAndAfter 增量回放:after=0 全量;after=断点
// 只补后面的帧;回放帧携带原 seq;他线程帧不串门。
func TestGetThreadEventsSinceReplayAndAfter(t *testing.T) {
	p := &seqProvider{id: "echo", entered: make(chan struct{}, 8), release: make(chan struct{}, 8)}
	srv, th := newEventTestServer(t, p)
	frames := runEchoTurn(t, srv, th, "你好")
	mid := frames[len(frames)/2].Sequence // 断线时手机已收到的最大 seq

	// 模拟"另一个线程"也在产生事件(全局 seq 空间里有交错)
	srv.recordTurnFrame("th-other", "turn-x", &pb.TurnServerMessage{
		Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: "别串台"}},
	})

	// after=0:回放该线程全部帧
	rs := &replayStream{ctx: context.Background()}
	if err := srv.GetThreadEventsSince(&pb.GetThreadEventsSinceRequest{ThreadId: th.ID}, rs); err != nil {
		t.Fatalf("GetThreadEventsSince(after=0): %v", err)
	}
	if len(rs.snapshot()) != len(frames) {
		t.Fatalf("after=0 回放 %d 帧, want %d", rs.sentCount(), len(frames))
	}
	for i, f := range rs.snapshot() {
		if f.Sequence != frames[i].Sequence {
			t.Fatalf("回放第 %d 帧 seq=%d, want %d(升序一致)", i, f.Sequence, frames[i].Sequence)
		}
	}
	if rs.snapshot()[0].GetStarted() == nil {
		t.Fatal("回放首帧应为 Started(帧序保持)")
	}

	// after=mid:只补 mid 之后的帧
	rs2 := &replayStream{ctx: context.Background()}
	if err := srv.GetThreadEventsSince(&pb.GetThreadEventsSinceRequest{ThreadId: th.ID, AfterSequence: mid}, rs2); err != nil {
		t.Fatalf("GetThreadEventsSince(after=%d): %v", mid, err)
	}
	got := rs2.snapshot()
	if len(got) == 0 {
		t.Fatal("断点之后应有帧可补")
	}
	for _, f := range got {
		if f.Sequence <= mid {
			t.Fatalf("回放出 seq=%d <= 断点 %d", f.Sequence, mid)
		}
	}
	// 他线程的帧绝不出现
	for _, f := range append(rs.snapshot(), got...) {
		if d := f.GetTextDelta(); d != nil && strings.Contains(d.Delta, "别串台") {
			t.Fatal("他线程的帧串进了本线程回放")
		}
	}
}

// TestGetThreadEventsSinceResyncRequired 超窗/空库两种情况都只回一帧
// ResyncRequired,手机端据此走全量回退。
func TestGetThreadEventsSinceResyncRequired(t *testing.T) {
	p := &seqProvider{id: "echo", entered: make(chan struct{}, 8), release: make(chan struct{}, 8)}
	srv, th := newEventTestServer(t, p)

	// 空库 + 手机持序号(典型:events.jsonl 被清掉/换机重装)→ resync
	rs := &replayStream{ctx: context.Background()}
	if err := srv.GetThreadEventsSince(&pb.GetThreadEventsSinceRequest{ThreadId: th.ID, AfterSequence: 42}, rs); err != nil {
		t.Fatalf("空库请求: %v", err)
	}
	if sent := rs.snapshot(); len(sent) != 1 || sent[0].GetResyncRequired() == nil {
		t.Fatalf("空库应只回 ResyncRequired, got %d 帧", rs.sentCount())
	}

	// 轮转产生丢弃区间:after 落在区间里 → resync
	srv.events.SetMaxBytes(120)
	for i := 0; i < 6; i++ {
		srv.recordTurnFrame(th.ID, "turn-r", &pb.TurnServerMessage{
			Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: "padding-line"}},
		})
	}
	stale := srv.events.MinSeq() - 2 // 比最老保留行更早 = 已被轮转丢弃
	rs2 := &replayStream{ctx: context.Background()}
	if err := srv.GetThreadEventsSince(&pb.GetThreadEventsSinceRequest{ThreadId: th.ID, AfterSequence: stale}, rs2); err != nil {
		t.Fatalf("超窗请求: %v", err)
	}
	if sent := rs2.snapshot(); len(sent) != 1 || sent[0].GetResyncRequired() == nil {
		t.Fatalf("超窗应只回 ResyncRequired, got %d 帧", rs2.sentCount())
	}
	// 恰好接上最老保留行:正常回放,不给 resync
	fresh := srv.events.MinSeq() - 1
	rs3 := &replayStream{ctx: context.Background()}
	if err := srv.GetThreadEventsSince(&pb.GetThreadEventsSinceRequest{ThreadId: th.ID, AfterSequence: fresh}, rs3); err != nil {
		t.Fatalf("接续请求: %v", err)
	}
	if sent := rs3.snapshot(); len(sent) == 0 || sent[0].GetResyncRequired() != nil {
		t.Fatal("断点恰好接上保留区间时应正常回放")
	}
}

// TestTurnFinishedHostEventRecorded 轮次终局产生带 seq 的主机事件
// (WatchEvents after_sequence 补发的数据源)。
func TestTurnFinishedHostEventRecorded(t *testing.T) {
	p := &seqProvider{id: "echo", entered: make(chan struct{}, 8), release: make(chan struct{}, 8)}
	srv, th := newEventTestServer(t, p)
	runEchoTurn(t, srv, th, "你好")

	hosts := srv.events.ReplayHostEvents(0)
	if len(hosts) != 1 {
		t.Fatalf("主机事件数 = %d, want 1(轮次终局)", len(hosts))
	}
	if hosts[0].ThreadID != th.ID || hosts[0].Seq == 0 {
		t.Fatalf("主机事件归属错误: %+v", hosts[0])
	}
	if !strings.Contains(hosts[0].EventType, "turn_finished") {
		t.Fatalf("事件类型 = %s, want host.turn_finished", hosts[0].EventType)
	}
}

// ---------- 测试基建 ----------

// replayStream GetThreadEventsSince 的服务端流假体:只记录 Send 进来的帧。
type replayStream struct {
	grpc.ServerStream
	ctx  context.Context
	mu   sync.Mutex
	sent []*pb.TurnServerMessage
}

func (r *replayStream) Send(m *pb.TurnServerMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, m)
	return nil
}

func (r *replayStream) Context() context.Context { return r.ctx }

func (r *replayStream) snapshot() []*pb.TurnServerMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*pb.TurnServerMessage(nil), r.sent...)
}

func (r *replayStream) sentCount() int { return len(r.snapshot()) }
