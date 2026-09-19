package server

import (
	"context"
	"io"
	"os/exec"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/term"
)

// P1 终端会话生命周期(server 侧)回归:
//   - Resumed 提示帧只在"复用已存在会话"时发,新会话不发;
//   - pumpTerminalOutput 在客户端断开/会话关闭时退出且反注册 reader,
//     不留 goroutine(goroutine 泄漏检查用 done channel 确定性 join)。

// fakeAttachStream 只实现 Attach/pump 触达的三个方法(Recv/Send/Context);
// 其余 grpc.ServerStream 方法嵌入接口占位,不会被调用。
type fakeAttachStream struct {
	grpc.ServerStream
	ctx   context.Context
	msgs  []*pb.TerminalClientMessage
	idx   int
	block bool // Recv 队列耗尽后阻塞到 ctx 取消(模拟长连流)
	mu    sync.Mutex
	sent  []*pb.TerminalServerMessage
}

func (f *fakeAttachStream) Context() context.Context { return f.ctx }

func (f *fakeAttachStream) Recv() (*pb.TerminalClientMessage, error) {
	if f.idx < len(f.msgs) {
		m := f.msgs[f.idx]
		f.idx++
		return m, nil
	}
	if f.block {
		<-f.ctx.Done()
		return nil, f.ctx.Err()
	}
	return nil, io.EOF
}

func (f *fakeAttachStream) Send(m *pb.TerminalServerMessage) error {
	f.mu.Lock()
	f.sent = append(f.sent, m)
	f.mu.Unlock()
	return nil
}

func (f *fakeAttachStream) sentFrames() []*pb.TerminalServerMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*pb.TerminalServerMessage(nil), f.sent...)
}

// waitReader 等 ReaderCount 达到 want;pump 订阅是异步的,需轮询。
func waitReader(s *term.Session, want int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.ReaderCount() == want {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return s.ReaderCount() == want
}

func openMsg(sid, dir string) *pb.TerminalClientMessage {
	return &pb.TerminalClientMessage{Msg: &pb.TerminalClientMessage_Open{
		Open: &pb.OpenTerminal{SessionId: sid, Cwd: dir, Shell: testShell()},
	}}
}

// testShell 返回 cmd.exe 绝对路径:go-pty 的 Cmd.Path 不走 LookPath,
// 传裸 "cmd.exe" 会被当作相对 cwd 的路径解析。
func testShell() string {
	p, err := exec.LookPath("cmd.exe")
	if err != nil {
		return "cmd.exe"
	}
	return p
}

// newAttachTestServer 造一个带真实临时存储的 Server(server.New 内部要读 Store)。
func newAttachTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return New(Config{Store: st, Terminals: term.NewManager()})
}

// Resumed 帧只在复用时发:新会话首帧是 Opened(无 Resumed);
// 同 session_id 第二次 Attach 先收 Resumed(会话已存活时长)再收 Opened。
func TestResumedFrameOnlyOnReuse(t *testing.T) {
	srv := newAttachTestServer(t)
	const sid = "resume-probe-1"
	dir := t.TempDir()

	ctx1, cancel1 := context.WithCancel(context.Background())
	st1 := &fakeAttachStream{ctx: ctx1, msgs: []*pb.TerminalClientMessage{openMsg(sid, dir)}}
	if err := srv.Attach(st1); err != nil {
		t.Fatalf("首次 Attach: %v", err)
	}
	cancel1() // 断开首连:pump 应退出并反注册

	for _, fr := range st1.sentFrames() {
		if fr.GetResumed() != nil {
			t.Fatalf("新会话不应发 Resumed 帧: %+v", fr)
		}
	}
	if n := len(st1.sentFrames()); n == 0 || st1.sentFrames()[0].GetOpened() == nil {
		t.Fatalf("新会话首帧应为 Opened: %+v", st1.sentFrames())
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	st2 := &fakeAttachStream{ctx: ctx2, msgs: []*pb.TerminalClientMessage{openMsg(sid, dir)}}
	if err := srv.Attach(st2); err != nil {
		t.Fatalf("复用 Attach: %v", err)
	}
	frames := st2.sentFrames()
	if len(frames) < 2 || frames[0].GetResumed() == nil || frames[1].GetOpened() == nil {
		t.Fatalf("复用应先 Resumed 后 Opened: %+v", frames)
	}
	r := frames[0].GetResumed()
	if r.SessionId != sid {
		t.Fatalf("Resumed.SessionId = %q, want %q", r.SessionId, sid)
	}
	if r.AttachedMs < 0 {
		t.Fatalf("Resumed.AttachedMs = %d, 应 >= 0", r.AttachedMs)
	}
	srv.cfg.Terminals.Close(sid)
}

// 客户端断开(ctx 取消):pump 退出并反注册,reader 数归零,goroutine 不泄漏。
func TestPumpExitsAndUnsubscribesOnClientDisconnect(t *testing.T) {
	m := term.NewManager()
	sess, err := m.Open("pump-leak-1", t.TempDir(), testShell(), 80, 24, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close("pump-leak-1")

	ctx, cancel := context.WithCancel(context.Background())
	fs := &fakeAttachStream{ctx: ctx, block: true}
	done := make(chan struct{})
	go func() {
		pumpTerminalOutput(fs, sess)
		close(done)
	}()
	if !waitReader(sess, 1, 5*time.Second) {
		t.Fatal("pump 未订阅会话输出")
	}

	cancel() // 模拟手机断开
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pump 未随客户端断开退出(goroutine 泄漏)")
	}
	if n := sess.ReaderCount(); n != 0 {
		t.Fatalf("pump 退出后应反注册 reader, 仍剩 %d", n)
	}
}

// 会话关闭(空闲回收/显式 Close):reader channel 被关闭,pump 自然退出
// 并向流补发 TerminalExit,goroutine 不泄漏。
func TestPumpExitsOnSessionCloseAndSendsExitFrame(t *testing.T) {
	m := term.NewManager()
	const sid = "pump-close-1"
	sess, err := m.Open(sid, t.TempDir(), testShell(), 80, 24, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	fs := &fakeAttachStream{ctx: context.Background()}
	done := make(chan struct{})
	go func() {
		pumpTerminalOutput(fs, sess)
		close(done)
	}()
	if !waitReader(sess, 1, 5*time.Second) {
		t.Fatal("pump 未订阅会话输出")
	}

	m.Close(sid) // 回收路径:close 会关掉所有 reader channel
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pump 未随会话关闭退出(goroutine 泄漏)")
	}
	var exit *pb.TerminalExit
	for _, fr := range fs.sentFrames() {
		if e := fr.GetExit(); e != nil {
			exit = e
		}
	}
	if exit == nil || exit.SessionId != sid {
		t.Fatalf("会话关闭后 pump 应补发 Exit 帧: %+v", fs.sentFrames())
	}
}
