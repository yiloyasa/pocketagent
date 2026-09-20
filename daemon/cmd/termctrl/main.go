// termctrl:诊断/验证探针——在真 daemon 的 PTY 里跑 ping -t,再发 \x03,
// 验证 Ctrl+C 经 daemon→ConPTY 链路能否中断子进程。
//
// 三种模式:
//
//	go run ./cmd/termctrl            诊断模式:打印各阶段输出(历史行为)
//	go run ./cmd/termctrl -verify    验证模式:3 秒内断言 ping 被中断,
//	                                 退出码 0=通过,1=失败(部署后回归用)
//	go run ./cmd/termctrl -reclaim   回收探针:开 N 个终端会话再全部断开,
//	                                 轮询 ListTerminals 断言它们被空闲回收,
//	                                 且回收后重连同 id 按新会话开(无 Resumed)。
//	                                 需 daemon 以短 -term-idle 运行(如 0.02m)。
//
// 用法:go run ./cmd/termctrl [-addr 127.0.0.1:7246](TLS 跳过校验,仅本机诊断用)
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7246", "daemon 地址")
	verify := flag.Bool("verify", false, "验证模式:断言 Ctrl+C 能在 3 秒内中断 ping,退出码 0/1")
	reclaim := flag.Bool("reclaim", false, "回收探针:开 N 个会话全部断开,断言被空闲回收(需 daemon 短 -term-idle)")
	probeN := flag.Int("n", 3, "-reclaim 模式:会话数")
	probeTimeout := flag.Duration("probe-timeout", 30*time.Second, "-reclaim 模式:等待回收的总超时")
	flag.Parse()

	conn, err := grpc.NewClient(*addr,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if *reclaim {
		os.Exit(runReclaim(conn, *probeN, *probeTimeout))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	stream, err := pb.NewTerminalServiceClient(conn).Attach(ctx)
	if err != nil {
		log.Fatalf("attach: %v", err)
	}

	out := make(chan string, 512)
	go func() {
		for {
			ev, err := stream.Recv()
			if err == io.EOF || ctx.Err() != nil {
				return
			}
			if err != nil {
				log.Printf("recv: %v", err)
				return
			}
			if m, ok := ev.Msg.(*pb.TerminalServerMessage_Output); ok {
				out <- string(m.Output.Data)
			}
		}
	}()

	send := func(data string) {
		if err := stream.Send(&pb.TerminalClientMessage{Msg: &pb.TerminalClientMessage_Input{
			Input: &pb.TerminalInput{Data: []byte(data)},
		}}); err != nil {
			log.Fatalf("send: %v", err)
		}
	}

	// 会话 id 带时间戳:不复用旧会话,避免上次诊断的残留状态干扰
	sessionID := fmt.Sprintf("ctrltest-%d", time.Now().UnixNano())
	must(stream.Send(&pb.TerminalClientMessage{Msg: &pb.TerminalClientMessage_Open{
		Open: &pb.OpenTerminal{SessionId: sessionID, Cols: 100, Rows: 40},
	}}))

	collect := func(d time.Duration) string {
		var b strings.Builder
		deadline := time.Now().Add(d)
		for time.Now().Before(deadline) {
			select {
			case s := <-out:
				b.WriteString(s)
			case <-time.After(150 * time.Millisecond):
			}
		}
		return b.String()
	}

	// 等 shell 提示符就绪
	warm := collect(3 * time.Second)
	fmt.Printf("[warm] %d bytes(末尾 %q)\n", len(warm), tail(warm, 40))

	// 跑 ping
	send("ping -t 127.0.0.1\r\n")
	p1 := collect(4 * time.Second)
	replies1 := strings.Count(p1, "TTL=")
	fmt.Printf("[ping 启动] TTL 行数=%d(末尾 %q)\n", replies1, tail(p1, 40))
	if replies1 == 0 {
		fmt.Println("[fail] ping 没有产出 TTL 行,链路异常")
		os.Exit(1)
	}

	// 发 Ctrl+C,观察 3 秒
	send("\x03")
	p2 := collect(3 * time.Second)
	replies2 := strings.Count(p2, "TTL=")
	interrupted := pingInterrupted(p2)
	fmt.Printf("[发送 0x03 后] 新增 TTL 行=%d,中断迹象=%v\n", replies2, interrupted)
	fmt.Printf("[发送 0x03 后末尾] %q\n", tail(p2, 120))

	// 收尾:退出 shell
	send("exit\r\n")
	time.Sleep(500 * time.Millisecond)

	if *verify {
		if interrupted {
			fmt.Println("[PASS] Ctrl+C 在 3 秒内中断了 ping")
			os.Exit(0)
		}
		fmt.Println("[FAIL] Ctrl+C 未能中断 ping")
		os.Exit(1)
	}
}

// pingInterrupted 判断这段输出是否意味着 ping 已被中断:
// 出现统计行(中/英)或 shell 提示符。
func pingInterrupted(newOut string) bool {
	for _, kw := range []string{"统计", "statistics", "Sent =", "已发送", "Lost =", "已丢失", "Minimum", "最短"} {
		if strings.Contains(newOut, kw) {
			return true
		}
	}
	return strings.Contains(newOut, "PS ") || strings.Contains(newOut, "PS>") ||
		strings.Contains(newOut, "C:\\>")
}

func tail(s string, n int) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\u0000", "")
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// ────────────────── -reclaim 回收探针 ──────────────────

// runReclaim:①开 N 个独立 Attach 流各开一个会话,收到 Opened 即断流
// (detach);②轮询 ListTerminals,断言 N 个会话都被空闲回收出表;
// ③重连第一个 id,断言没有 Resumed 帧(已回收会话=当新会话开)。
// 退出码 0=PASS,1=FAIL。
func runReclaim(conn *grpc.ClientConn, n int, timeout time.Duration) int {
	client := pb.NewTerminalServiceClient(conn)
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("reclaimprobe-%d-%d", stamp, i)
	}

	// ① 开会话并全部断开
	start := time.Now()
	for i, sid := range ids {
		if err := openAndDetach(ctx, client, sid); err != nil {
			fmt.Printf("[fail] 会话 %s(%d/%d): %v\n", sid, i+1, n, err)
			return 1
		}
		fmt.Printf("[open+detach] %s\n", sid)
	}
	fmt.Printf("[info] %d 个会话已开并全部断开,等待空闲回收(总超时 %s)\n", n, timeout)

	// ② 轮询直到全部从表里消失
	deadline := time.Now().Add(timeout)
	for {
		remaining, err := listRemaining(ctx, client, ids)
		if err != nil {
			fmt.Printf("[fail] ListTerminals: %v\n", err)
			return 1
		}
		if remaining == 0 {
			fmt.Printf("[ok] %d 个会话在开始后 %s 内全部被回收\n", n, time.Since(start).Round(time.Millisecond))
			break
		}
		if time.Now().After(deadline) {
			fmt.Printf("[fail] %d/%d 个会话在 %s 内未被回收(daemon 是否带短 -term-idle 启动?)\n", remaining, n, timeout)
			return 1
		}
		time.Sleep(300 * time.Millisecond)
	}

	// ③ 已回收会话重连 = 新会话,不应有 Resumed 帧
	sid := ids[0]
	resumed, err := reopenShowsResumed(ctx, client, sid)
	if err != nil {
		fmt.Printf("[fail] 重连 %s: %v\n", sid, err)
		return 1
	}
	if resumed {
		fmt.Printf("[fail] %s 已被回收,重连却收到 Resumed 帧\n", sid)
		return 1
	}
	fmt.Printf("[ok] 回收后重连 %s 按新会话开(无 Resumed 帧)\n", sid)
	fmt.Println("[PASS] 终端会话空闲回收生效")
	return 0
}

// openAndDetach 开一条 Attach 流开指定会话,收到 Opened(或异常的 Resumed)
// 后断流。ctx 超时兜底回收流资源。
func openAndDetach(ctx context.Context, client pb.TerminalServiceClient, sid string) error {
	sctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	stream, err := client.Attach(sctx)
	if err != nil {
		return err
	}
	if err := stream.Send(&pb.TerminalClientMessage{Msg: &pb.TerminalClientMessage_Open{
		Open: &pb.OpenTerminal{SessionId: sid, Cols: 80, Rows: 24},
	}}); err != nil {
		return err
	}
	for {
		ev, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("等 Opened: %w", err)
		}
		switch m := ev.Msg.(type) {
		case *pb.TerminalServerMessage_Opened:
			return stream.CloseSend() // 断流 = detach,回收交给 daemon
		case *pb.TerminalServerMessage_Resumed:
			return fmt.Errorf("新会话却收到 Resumed 帧 session=%s", m.Resumed.GetSessionId())
		case *pb.TerminalServerMessage_Error:
			return fmt.Errorf("服务端错误: %s", m.Error.GetMessage())
		}
	}
}

// listRemaining 统计目标会话 id 仍在 ListTerminals 表里的数量。
func listRemaining(ctx context.Context, client pb.TerminalServiceClient, ids []string) (int, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := client.ListTerminals(cctx, &pb.ListTerminalsRequest{})
	if err != nil {
		return 0, err
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	n := 0
	for _, s := range resp.Sessions {
		if want[s.SessionId] {
			n++
		}
	}
	return n, nil
}

// reopenShowsResumed 重新 Attach 指定 id,返回是否出现 Resumed 帧
// (读到 Opened 即返回)。
func reopenShowsResumed(ctx context.Context, client pb.TerminalServiceClient, sid string) (bool, error) {
	sctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	stream, err := client.Attach(sctx)
	if err != nil {
		return false, err
	}
	if err := stream.Send(&pb.TerminalClientMessage{Msg: &pb.TerminalClientMessage_Open{
		Open: &pb.OpenTerminal{SessionId: sid, Cols: 80, Rows: 24},
	}}); err != nil {
		return false, err
	}
	resumed := false
	for {
		ev, err := stream.Recv()
		if err != nil {
			return false, fmt.Errorf("等 Opened: %w", err)
		}
		switch m := ev.Msg.(type) {
		case *pb.TerminalServerMessage_Resumed:
			resumed = true
		case *pb.TerminalServerMessage_Opened:
			_ = stream.CloseSend()
			return resumed, nil
		case *pb.TerminalServerMessage_Error:
			return false, fmt.Errorf("服务端错误: %s", m.Error.GetMessage())
		}
	}
}
