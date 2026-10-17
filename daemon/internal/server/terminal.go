package server

import (
	"context"
	"fmt"
	"io"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/term"
)

// Attach 终端双向流:开 PTY → 输入/输出/resize 双向搬运 → 退出。
//
// 【讲解】手机上的 xterm.js 渲染字符画面,但真正跑 shell 的是你电脑上的
// PTY(伪终端)。PTY 输出的是带 ANSI 转义码的原始字节流(颜色、光标移动),
// xterm.js 负责把它"画"成你看到的终端样子——所以手机终端和电脑终端
// 画面一模一样,因为字节流一模一样。
func (s *Server) Attach(stream pb.TerminalService_AttachServer) error {
	var session *term.Session
	defer func() {
		if session != nil {
			// 断开不杀会话(支持重连找回);无 reader 空闲回收由
			// term.Manager 的后台回收器按 -term-idle 处理,此处无需动作。
			_ = session
		}
	}()

	for {
		msg, err := stream.Recv()
		if err != nil {
			// 客户端主动结束双向流（例如切换目录/离开终端页）时，
			// gRPC 以 io.EOF 通知服务端。它是正常的生命周期结束，
			// 不能把 EOF 作为 UNKNOWN 错误再回传给客户端。
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch m := msg.Msg.(type) {
		case *pb.TerminalClientMessage_Open:
			if session != nil {
				continue
			}
			// 复用检测:同 session_id 的存活会话已在表 → 先发 Resumed 提示帧。
			// PTY 无回放,复用静默 shell 时不会有任何首字节输出,该帧让客户端
			// 得以区分"恢复旧会话"与"卡死";旧客户端忽略未知 oneof 字段,兼容。
			if prev, ok := s.cfg.Terminals.GetAlive(m.Open.SessionId); ok {
				resumed := &pb.Resumed{
					SessionId:  prev.ID,
					AttachedMs: time.Now().UnixMilli() - prev.OpenedAt,
				}
				if err := stream.Send(&pb.TerminalServerMessage{Msg: &pb.TerminalServerMessage_Resumed{Resumed: resumed}}); err != nil {
					return err
				}
			}
			env := map[string]string{}
			sess, err := s.cfg.Terminals.Open(
				m.Open.SessionId, m.Open.Cwd, m.Open.Shell,
				int(m.Open.Cols), int(m.Open.Rows), env,
			)
			if err != nil {
				_ = stream.Send(&pb.TerminalServerMessage{Msg: &pb.TerminalServerMessage_Error{Error: &pb.TerminalError{
					Message: err.Error(),
				}}})
				return err
			}
			session = sess
			if err := stream.Send(&pb.TerminalServerMessage{Msg: &pb.TerminalServerMessage_Opened{Opened: &pb.TerminalOpened{
				SessionId: sess.ID, Shell: sess.Shell, Cwd: sess.CWD,
			}}}); err != nil {
				return err
			}
			go pumpTerminalOutput(stream, sess)

		case *pb.TerminalClientMessage_Input:
			if session != nil {
				if err := session.Write(m.Input.Data); err != nil {
					return failTerminalStream(stream, session.ID,
						fmt.Sprintf("终端输入写入失败(会话 %s): %v", session.ID, err), err)
				}
			}

		case *pb.TerminalClientMessage_Resize:
			if session != nil {
				cols, rows := int(m.Resize.Cols), int(m.Resize.Rows)
				if err := session.Resize(cols, rows); err != nil {
					return failTerminalStream(stream, session.ID,
						fmt.Sprintf("终端改窗失败(会话 %s,%dx%d): %v", session.ID, cols, rows, err), err)
				}
			}
		}
	}
}

// failTerminalStream 向客户端补发一帧 TerminalError 并终止本条 Attach 流;
// 返回值是给 gRPC 的(cause 原样上抛,状态码文本里带得下底层原因)。
//
// 【为什么发帧之后必须 return,而不是"发帧后继续"】error 帧在本协议里就是
// 终态信号:手机端收到即置 error 态并关流(android TerminalScreen.kt 的
// ERROR 分支)。所以"先发帧再续跑"并不存在续跑——服务端只会守一条客户端已经
// 放弃的流,下一个 Recv 错误反而把真正的原因盖掉。纪律:error 帧 ⟺ 本流结束。
//
// 【为什么必须让客户端看见】PTY 写/改窗失败意味着这一轮按键或改窗没落地,而
// 此前这两个调用是 `_ =` 丢弃:daemon.log 零线索、手机零信号,现场只剩用户报障
// 的"终端不响应了"。这与本仓自己写下的留痕纪律冲突(store_observability.go:
// "静默失效……且日志里没有任何线索"= 不可接受)。error 帧是本文件现成的错误面,
// 与 Open 失败同款;顺带补上 proto 里本就为"哪个会话坏了"准备的 session_id
// (Open 失败那条没有会话可填,保持原样)。
//
// 【为什么两条失败都按终态处理,而不是只让写失败终态】
//   - 写失败:term/ctrl.go 的 ptyWriteWithBudget 已把语义写死——超时"按会话已
//     坏对待……由调用方终止/废弃该会话,不再复用";吞掉它等于让此后每一次按键
//     都静默丢失。
//   - 改窗失败:Session.Resize 唯一会失败的形态是伪控制台已死(go-pty 的
//     Resize 只包 windows.ResizePseudoConsole;cols/rows<=0 已在 term/manager.go
//     里被当成 no-op 提前返回)。伪控制台死了,这条流就再也给不出正确尺寸的画面,
//     而复用路径(term/manager.go Open 的活会话快返)不会重放 cols/rows —— 继续
//     跑等于让用户盯着一块永久错位、永不解释的屏幕。终止后重连会拿到新会话,
//     尺寸由 OpenTerminal 重新给准。注意这与 spawn 期 manager.go:314 那次
//     "首次 resize 失败不影响会话"不矛盾:那里还没有客户端可通知。
func failTerminalStream(stream pb.TerminalService_AttachServer, sessionID, message string, cause error) error {
	if err := stream.Send(&pb.TerminalServerMessage{Msg: &pb.TerminalServerMessage_Error{
		Error: &pb.TerminalError{SessionId: sessionID, Message: message},
	}}); err != nil {
		return err // 连错误帧都送不出去:以传输错误为准,别把 cause 顶掉
	}
	return cause
}

// pumpTerminalOutput 把 PTY 输出搬运到 gRPC 流。
func pumpTerminalOutput(stream pb.TerminalService_AttachServer, sess *term.Session) {
	ch, err := sess.Read()
	if err != nil {
		return
	}
	// 无论因何种原因退出(手机断开/Send 失败/会话关闭),都必须反注册:
	// 否则死 channel 常驻 readers 切片,且空闲回收器无法知道会话已无 reader。
	defer sess.Unsubscribe(ch)
	ctx := stream.Context()
	for data := range ch {
		// 手机已断(keepalive 检出/对端 RST):别再往死流里塞,
		// 否则流控窗口一满,Send 会把整个 PTY 泵一起堵死。
		if ctx.Err() != nil {
			return
		}
		d := data
		if err := stream.Send(&pb.TerminalServerMessage{Msg: &pb.TerminalServerMessage_Output{Output: &pb.TerminalOutput{Data: d}}}); err != nil {
			return
		}
	}
	// PTY 结束:通知退出码
	<-sess.Done()
	_ = stream.Send(&pb.TerminalServerMessage{Msg: &pb.TerminalServerMessage_Exit{Exit: &pb.TerminalExit{
		SessionId: sess.ID, Code: int32(sess.ExitCode()),
	}}})
}

func (s *Server) ListTerminals(ctx context.Context, _ *pb.ListTerminalsRequest) (*pb.ListTerminalsResponse, error) {
	out := &pb.ListTerminalsResponse{}
	for _, sess := range s.cfg.Terminals.List() {
		out.Sessions = append(out.Sessions, &pb.TerminalSessionInfo{
			SessionId: sess.ID, Shell: sess.Shell, Cwd: sess.CWD,
			OpenedAtUnixMs: sess.OpenedAt, Alive: sess.Alive(),
		})
	}
	return out, nil
}

func (s *Server) CloseTerminal(ctx context.Context, req *pb.CloseTerminalRequest) (*pb.Empty, error) {
	s.cfg.Terminals.Close(req.SessionId)
	return &pb.Empty{}, nil
}
