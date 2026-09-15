package server

import (
	"context"
	"io"

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
			// 断开不杀会话(支持重连找回);P1 加空闲超时回收
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
	for data := range ch {
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

var _ = io.EOF
