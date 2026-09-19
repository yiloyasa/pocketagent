package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	// 特例:termtest 打的是手工起的测试 daemon(7262,通常 -plaintext 明文),
	// 默认保持明文兼容旧行为;要连 TLS 端口时显式加 -tls。
	addr := flag.String("addr", "127.0.0.1:7262", "daemon 地址(ip:port)")
	tlsMode := flag.Bool("tls", false, "连 TLS(自签证书,诊断用跳过校验;指纹核对走 tlsprobe)")
	flag.Parse()

	var creds credentials.TransportCredentials
	if *tlsMode {
		creds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // 探针仅本机诊断用
	} else {
		creds = insecure.NewCredentials()
	}
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(creds))
	must(err)
	defer conn.Close()

	stream, err := pb.NewTerminalServiceClient(conn).Attach(context.Background())
	must(err)
	must(stream.Send(&pb.TerminalClientMessage{Msg: &pb.TerminalClientMessage_Open{
		Open: &pb.OpenTerminal{SessionId: "t1", Cols: 80, Rows: 24},
	}}))

	got := make(chan string, 64)
	go func() {
		for {
			ev, err := stream.Recv()
			if err == io.EOF {
				return
			}
			must(err)
			switch m := ev.Msg.(type) {
			case *pb.TerminalServerMessage_Opened:
				fmt.Printf("OPENED shell=%s cwd=%s\n", m.Opened.Shell, m.Opened.Cwd)
			case *pb.TerminalServerMessage_Output:
				got <- string(m.Output.Data)
			case *pb.TerminalServerMessage_Error:
				fmt.Printf("ERROR: %s\n", m.Error.Message)
				return
			case *pb.TerminalServerMessage_Exit:
				fmt.Printf("EXIT code=%d\n", m.Exit.Code)
				return
			}
		}
	}()

	time.Sleep(2 * time.Second)
	must(stream.Send(&pb.TerminalClientMessage{Msg: &pb.TerminalClientMessage_Input{
		Input: &pb.TerminalInput{Data: []byte("echo hello_pocketagent\r\n")},
	}}))

	deadline := time.After(8 * time.Second)
	buf := ""
	for {
		select {
		case b := <-got:
			buf += b
			if len(buf) > 4000 || time.Now().UnixMilli()%97 == 0 {
			}
		case <-deadline:
			fmt.Printf("TOTAL %d bytes, contains hello_pocketagent: %v\n", len(buf), contains(buf, "hello_pocketagent"))
			return
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
