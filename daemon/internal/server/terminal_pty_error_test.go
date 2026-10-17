package server

// terminal_pty_error_test.go —— Attach 流里 PTY 侧失败的传播钉子
// (clean-daemon-server 审查 P0-1:terminal.go 的 session.Write/Resize 曾是
// `_ =` 丢弃,PTY 写失败时客户端收不到任何信号,daemon.log 也没有线索)。
//
// 【钉的不是日志,是判决】
//   1. PTY 写失败 → 客户端必须收到 TerminalError 帧(带 session_id),且 Attach
//      以错误收场;"发帧后继续"不算修好(error 帧在手机端就是终态信号)。
//   2. PTY 改窗失败 → 同上。
//   3. PTY 正常时不得凭空发 error 帧(防过度 signaling:好端端的会话被误伤)。
//
// 【时序怎么做到确定】不靠 sleep 等 PTY 自己死:fake 流在"交付第 n 帧"这个
// 精确时刻执行回调,回调里先 Terminals.Close(伪控制台随之关闭)再等到探针写
// 失败(证明 pty.Close 已跑完 —— 它先关伪控制台、再关管道),于是下一帧的
// Write/Resize 必然失败。Recv 只由 Attach 的调用 goroutine 驱动(本文件同步
// 调用 Attach),故回调里的 t.Fatalf 合法,且整条时序无竞态。

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/term"
)

// ptyErrStream 与 fakeAttachStream 同形,多一个 before 回调:Recv 交付第 idx
// 帧(从 0 计)之前先跑它。Send 会被 Attach 的循环与 pumpTerminalOutput 两个
// goroutine 并发调用,所以 sent 用锁保护(与 fakeAttachStream 同款)。
type ptyErrStream struct {
	grpc.ServerStream
	ctx    context.Context
	msgs   []*pb.TerminalClientMessage
	idx    int
	before func(idx int)
	mu     sync.Mutex
	sent   []*pb.TerminalServerMessage
}

func (f *ptyErrStream) Context() context.Context { return f.ctx }

func (f *ptyErrStream) Recv() (*pb.TerminalClientMessage, error) {
	if f.idx >= len(f.msgs) {
		return nil, io.EOF // 帧发完 = 客户端正常结束双向流(Attach 应返回 nil)
	}
	if f.before != nil {
		f.before(f.idx)
	}
	m := f.msgs[f.idx]
	f.idx++
	return m, nil
}

func (f *ptyErrStream) Send(m *pb.TerminalServerMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, m)
	return nil
}

func (f *ptyErrStream) sentFrames() []*pb.TerminalServerMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*pb.TerminalServerMessage(nil), f.sent...)
}

func inputMsg(data string) *pb.TerminalClientMessage {
	return &pb.TerminalClientMessage{Msg: &pb.TerminalClientMessage_Input{Input: &pb.TerminalInput{Data: []byte(data)}}}
}

func resizeMsg(cols, rows int32) *pb.TerminalClientMessage {
	return &pb.TerminalClientMessage{Msg: &pb.TerminalClientMessage_Resize{Resize: &pb.TerminalResize{Cols: cols, Rows: rows}}}
}

// errorFrame 取流里第一帧 TerminalError(没有则 nil)。
func errorFrame(frames []*pb.TerminalServerMessage) *pb.TerminalError {
	for _, fr := range frames {
		if e := fr.GetError(); e != nil {
			return e
		}
	}
	return nil
}

// killPTYNow 在"会话已建好、下一帧还没交付"的窗口里把 PTY 关掉,并等到关闭
// 真正完成。返回该会话以便调用方复查。
func killPTYNow(t *testing.T, m *term.Manager, sid string) *term.Session {
	t.Helper()
	sess, ok := m.Get(sid)
	if !ok {
		t.Fatalf("会话 %s 应已建好(Attach 的 Open 已处理)", sid)
	}
	m.Close(sid) // 关会话 → Session.close 关 reader、杀 shell、关伪控制台
	// 以"探针写失败"为 pty.Close 完成的信号(Close 内部先 ClosePseudoConsole
	// 再关管道,故管道写失败时伪控制台必已关闭)。上限 5s 是 term 侧同款写预算。
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := sess.Write([]byte("x")); err != nil {
			return sess
		}
		if time.Now().After(deadline) {
			t.Fatalf("会话 %s 关闭后 PTY 写入仍未失败,无法构造失败现场", sid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// PTY 写失败:客户端必须收到 error 帧(swallow 修复的判决点)。
func TestAttachSendsErrorFrameWhenPTYWriteFails(t *testing.T) {
	srv := newAttachTestServer(t)
	const sid = "pty-write-fail-1"
	dir := t.TempDir()

	fs := &ptyErrStream{ctx: context.Background(), msgs: []*pb.TerminalClientMessage{
		openMsg(sid, dir),
		inputMsg("echo hi\r\n"),
	}}
	fs.before = func(idx int) {
		if idx == 1 { // Open 已处理完、Input 还没交付
			killPTYNow(t, srv.cfg.Terminals, sid)
		}
	}
	err := srv.Attach(fs)
	if err == nil {
		t.Fatalf("PTY 写失败后 Attach 必须以错误收场(不能装作一切正常)")
	}
	terr := errorFrame(fs.sentFrames())
	if terr == nil {
		t.Fatalf("PTY 写失败时客户端必须收到 TerminalError 帧,实际帧: %+v", fs.sentFrames())
	}
	if terr.GetSessionId() != sid {
		t.Fatalf("error 帧应带 session_id=%q,得到 %q", sid, terr.GetSessionId())
	}
	if !strings.Contains(terr.GetMessage(), "写入失败") {
		t.Fatalf("error 帧文案要说明是写入失败: %q", terr.GetMessage())
	}
}

// PTY 改窗失败:同样必须让客户端看见(此前与写失败一起被 `_ =` 吞掉)。
func TestAttachSendsErrorFrameWhenPTYResizeFails(t *testing.T) {
	srv := newAttachTestServer(t)
	const sid = "pty-resize-fail-1"
	dir := t.TempDir()

	fs := &ptyErrStream{ctx: context.Background(), msgs: []*pb.TerminalClientMessage{
		openMsg(sid, dir),
		resizeMsg(120, 40),
	}}
	fs.before = func(idx int) {
		if idx == 1 {
			killPTYNow(t, srv.cfg.Terminals, sid)
		}
	}
	err := srv.Attach(fs)
	if err == nil {
		t.Fatalf("PTY 改窗失败后 Attach 必须以错误收场")
	}
	terr := errorFrame(fs.sentFrames())
	if terr == nil {
		t.Fatalf("PTY 改窗失败时客户端必须收到 TerminalError 帧,实际帧: %+v", fs.sentFrames())
	}
	if terr.GetSessionId() != sid {
		t.Fatalf("error 帧应带 session_id=%q,得到 %q", sid, terr.GetSessionId())
	}
	if !strings.Contains(terr.GetMessage(), "改窗失败") || !strings.Contains(terr.GetMessage(), "120x40") {
		t.Fatalf("error 帧文案要说明改窗失败并带上尺寸: %q", terr.GetMessage())
	}
}

// 反向钉子:PTY 活着时不得发 error 帧(修复不能变成"无条件报错的终端")。
// 正常输入 + 正常改窗走完后 Attach 因 io.EOF 返回 nil。
func TestAttachNoErrorFrameWhenPTYIsHealthy(t *testing.T) {
	srv := newAttachTestServer(t)
	const sid = "pty-healthy-1"
	dir := t.TempDir()

	fs := &ptyErrStream{ctx: context.Background(), msgs: []*pb.TerminalClientMessage{
		openMsg(sid, dir),
		inputMsg("echo hi\r\n"),
		resizeMsg(100, 30),
	}}
	fs.before = func(idx int) {
		if idx == 1 {
			if !waitReader(mustGetSession(t, srv.cfg.Terminals, sid), 1, 5*time.Second) {
				t.Fatalf("pump 未订阅会话输出")
			}
		}
	}
	if err := srv.Attach(fs); err != nil {
		t.Fatalf("PTY 正常时 Attach 应因流结束返回 nil,得到 %v", err)
	}
	if e := errorFrame(fs.sentFrames()); e != nil {
		t.Fatalf("PTY 正常时不得发 error 帧: %+v", e)
	}
	srv.cfg.Terminals.Close(sid)
}

func mustGetSession(t *testing.T, m *term.Manager, sid string) *term.Session {
	t.Helper()
	sess, ok := m.Get(sid)
	if !ok {
		t.Fatalf("会话 %s 应在表里", sid)
	}
	return sess
}
