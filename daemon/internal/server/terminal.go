package server

import (
	"context"
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
				_ = session.Write(m.Input.Data)
			}

		case *pb.TerminalClientMessage_Resize:
			if session != nil {
				_ = session.Resize(int(m.Resize.Cols), int(m.Resize.Rows))
			}
		}
	}
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
