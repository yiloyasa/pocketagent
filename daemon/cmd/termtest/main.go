package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	conn, err := grpc.NewClient("127.0.0.1:7262", grpc.WithTransportCredentials(insecure.NewCredentials()))
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
