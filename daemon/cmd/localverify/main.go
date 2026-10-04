package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7246", "daemon 地址(ip:port)")
	plaintext := flag.Bool("plaintext", false, "连明文 gRPC(对付 -plaintext 起的测试 daemon);默认 TLS")
	flag.Parse()

	// daemon 生产默认 TLS(自签证书)。诊断用 InsecureSkipVerify 跳过自签校验;
	// 指纹核对走 tlsprobe。-plaintext=true 回明文。
	var creds credentials.TransportCredentials
	if *plaintext {
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // 探针仅本机诊断用
	}
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	host, err := pb.NewHostServiceClient(conn).GetHostInfo(ctx, &pb.GetHostInfoRequest{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("HOST workspaces=%v shell=%s\n", host.Workspaces, host.DefaultShell)

	// 工作区用系统临时目录(运行时派生):本探针验证的是 daemon 的 workspace/
	// 上传/终端链路本身,不依赖任何特定磁盘位置;曾写死开发机仓库所在盘符,
	// 字符串常量会随探针二进制分发(规格 §2 禁止)。退出时清理。
	ws, err := os.MkdirTemp("", "pa-localverify")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(ws)
	threads := pb.NewThreadServiceClient(conn)
	th, err := threads.CreateThread(ctx, &pb.CreateThreadRequest{Workspace: ws})
	if err != nil {
		panic(err)
	}
	fmt.Printf("THREAD id=%s workspace=%s\n", th.Id, th.Workspace)

	upload := fmt.Sprintf(`%s\.pocketagent-uploads\local-verify-%d.txt`, ws, time.Now().UnixNano())
	_, err = pb.NewFileSystemServiceClient(conn).WriteFile(ctx, &pb.WriteFileRequest{
		Path: upload, Data: []byte("workspace-ok"), Create: true, Truncate: true,
	})
	if err != nil {
		panic(err)
	}
	data, err := os.ReadFile(upload)
	if err != nil {
		panic(err)
	}
	fmt.Printf("UPLOAD path=%s data=%s\n", upload, data)
	_ = os.Remove(upload)

	term, err := pb.NewTerminalServiceClient(conn).Attach(ctx)
	if err != nil {
		panic(err)
	}
	sid := fmt.Sprintf("local-verify-%d", time.Now().UnixNano())
	if err := term.Send(&pb.TerminalClientMessage{
		Msg: &pb.TerminalClientMessage_Open{
			Open: &pb.OpenTerminal{SessionId: sid, Cwd: ws, Cols: 120, Rows: 30},
		},
	}); err != nil {
		panic(err)
	}

	var output strings.Builder
	sent := false
	deadline := time.After(8 * time.Second)
	for {
		select {
		case <-deadline:
			panic("terminal timeout")
		default:
		}
		recv, err := term.Recv()
		if err == io.EOF {
			panic("terminal EOF")
		}
		if err != nil {
			panic(err)
		}
		switch m := recv.Msg.(type) {
		case *pb.TerminalServerMessage_Opened:
			fmt.Printf("TERMINAL opened=%s\n", m.Opened.Cwd)
			if err := term.Send(&pb.TerminalClientMessage{
				Msg: &pb.TerminalClientMessage_Input{
					Input: &pb.TerminalInput{Data: []byte("Get-Location\r\n")},
				},
			}); err != nil {
				panic(err)
			}
			sent = true
		case *pb.TerminalServerMessage_Output:
			output.Write(m.Output.Data)
			if sent && strings.Contains(output.String(), ws) {
				fmt.Println("TERMINAL output_contains_workspace=true")
				_, _ = pb.NewTerminalServiceClient(conn).CloseTerminal(
					ctx, &pb.CloseTerminalRequest{SessionId: sid},
				)
				return
			}
		case *pb.TerminalServerMessage_Error:
			panic("terminal error: " + m.Error.Message)
		case *pb.TerminalServerMessage_Exit:
			panic(fmt.Sprintf("terminal exited code=%d", m.Exit.Code))
		}
	}
}
