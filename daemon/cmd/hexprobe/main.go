// hexprobe:诊断探针——抓 PTY 开屏原始字节,检查 ConPTY 是否启用了
// win32-input-mode(DECSET ?9001)。若启用,输入解析器只认编码键事件,
// 裸 \x03 会被丢弃——这正是 Ctrl+C 失效的候选根因。
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	conn, err := grpc.NewClient("127.0.0.1:7246",
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	stream, err := pb.NewTerminalServiceClient(conn).Attach(context.Background())
	if err != nil {
		panic(err)
	}
	stream.Send(&pb.TerminalClientMessage{Msg: &pb.TerminalClientMessage_Open{
		Open: &pb.OpenTerminal{SessionId: "hexprobe", Cols: 80, Rows: 24}}})
	deadline := time.Now().Add(4 * time.Second)
	var all []byte
	for time.Now().Before(deadline) {
		ev, err := stream.Recv()
		if err == io.EOF || err != nil {
			break
		}
		if m, ok := ev.Msg.(*pb.TerminalServerMessage_Output); ok {
			all = append(all, m.Output.Data...)
		}
	}
	fmt.Printf("total=%d bytes\n", len(all))
	for _, pat := range []string{"9001", "?1004", "?25", "[6n"} {
		idx := strings.Index(string(all), pat)
		fmt.Printf("pattern %q at %d\n", pat, idx)
	}
	if len(all) > 400 {
		all = all[:400]
	}
	fmt.Printf("%q\n", string(all))
}
