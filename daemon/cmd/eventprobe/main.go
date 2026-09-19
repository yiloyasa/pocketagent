// eventprobe —— P1-5 真机验证探针:连真 daemon,打印某线程 since N 的回放帧。
//
// 用法:
//
//	eventprobe -addr 127.0.0.1:7246 -token <token> -thread th-xxxx -after 0
//
// 输出每帧的 seq / 事件类型 / 单行摘要,末尾统计。用于在真机轮验证
// GetThreadEventsSince 的回放序、after 过滤与 ResyncRequired 行为。
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// tlsSkipVerify 探针专用:TLS 但不校验证书链/指纹(诊断工具,连接的是自己的电脑)。
func tlsSkipVerify() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true} //nolint:gosec // 探针仅本机诊断用
}

func main() {
	addr := flag.String("addr", "127.0.0.1:7246", "daemon 地址(ip:port)")
	token := flag.String("token", "", "访问 token(未启用鉴权可留空)")
	thread := flag.String("thread", "", "线程 id(必填)")
	after := flag.Int64("after", 0, "after_sequence(手机已收到的最大 seq)")
	flag.Parse()
	if *thread == "" {
		log.Fatal("缺少 -thread")
	}

	// daemon 一律 TLS(自签);探针是诊断工具,跳过证书校验即可(不校验指纹)。
	conn, err := grpc.NewClient(*addr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsSkipVerify())))
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if *token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+*token)
	}

	stream, err := pb.NewThreadServiceClient(conn).GetThreadEventsSince(ctx,
		&pb.GetThreadEventsSinceRequest{ThreadId: *thread, AfterSequence: *after})
	if err != nil {
		log.Fatalf("GetThreadEventsSince: %v", err)
	}

	var n int
	var lastSeq int64
	for {
		msg, err := stream.Recv()
		if err != nil {
			break // EOF 或错误都结束(回放完服务端即关流)
		}
		n++
		if s := msg.GetSequence(); s > lastSeq {
			lastSeq = s
		}
		fmt.Printf("#%03d seq=%d %s %s\n", n, msg.GetSequence(), frameKind(msg), frameSummary(msg))
	}
	fmt.Printf("共 %d 帧,max seq=%d(after=%d)\n", n, lastSeq, *after)
}

// frameKind 帧的 oneof 分支名(人话)。
func frameKind(m *pb.TurnServerMessage) string {
	switch m.Msg.(type) {
	case *pb.TurnServerMessage_Started:
		return "Started"
	case *pb.TurnServerMessage_TextDelta:
		return "TextDelta"
	case *pb.TurnServerMessage_ThinkingDelta:
		return "ThinkingDelta"
	case *pb.TurnServerMessage_ToolCall:
		return "ToolCall"
	case *pb.TurnServerMessage_ApprovalRequest:
		return "ApprovalRequest"
	case *pb.TurnServerMessage_Usage:
		return "Usage"
	case *pb.TurnServerMessage_Completed:
		return "Completed"
	case *pb.TurnServerMessage_Failed:
		return "Failed"
	case *pb.TurnServerMessage_CodexEvent:
		return "CodexEvent"
	case *pb.TurnServerMessage_ApprovalResolved:
		return "ApprovalResolved"
	case *pb.TurnServerMessage_Steered:
		return "Steered"
	case *pb.TurnServerMessage_Queued:
		return "Queued"
	case *pb.TurnServerMessage_QueueFlushed:
		return "QueueFlushed"
	case *pb.TurnServerMessage_ResyncRequired:
		return "ResyncRequired"
	default:
		return "?"
	}
}

// frameSummary 单行摘要(delta 文本 / 错误信息等),截到 60 字符。
func frameSummary(m *pb.TurnServerMessage) string {
	s := ""
	switch {
	case m.GetTextDelta() != nil:
		s = m.GetTextDelta().GetDelta()
	case m.GetThinkingDelta() != nil:
		s = m.GetThinkingDelta().GetDelta()
	case m.GetCompleted() != nil:
		s = fmt.Sprintf("turn=%s blocks=%d", m.GetCompleted().GetTurnId(), len(m.GetCompleted().GetFinalBlocks()))
	case m.GetFailed() != nil:
		s = "err=" + m.GetFailed().GetError().GetMessage()
	case m.GetStarted() != nil:
		s = "turn=" + m.GetStarted().GetTurnId()
	case m.GetResyncRequired() != nil:
		s = m.GetResyncRequired().GetReason()
	case m.GetToolCall() != nil && m.GetToolCall().GetCall() != nil:
		s = m.GetToolCall().GetCall().GetName()
	case m.GetCodexEvent() != nil:
		s = m.GetCodexEvent().GetTitle()
	}
	r := []rune(s)
	if len(r) > 60 {
		r = r[:60]
	}
	return string(r)
}
