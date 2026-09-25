// controlplaneshow —— 把 daemon 的 GetControlPlane 控制面总览打成人话报表。
//
// 【它是谁】一次性演示/运维工具:连上 daemon,把八个功能域的控制面
// 排版输出到终端,给用户直接看。不是生产链组件,只读不写。
//
// 用法: controlplaneshow -addr 127.0.0.1:7246 [-token <设备token>](自签 TLS,跳过校验)
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7246", "daemon gRPC 地址")
	token := flag.String("token", "", "设备 token(鉴权模式下必填)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// daemon 默认启用 TLS(自签,首启生成);本机演示直接跳过证书校验。
	tlsCreds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
	opts := []grpc.DialOption{grpc.WithTransportCredentials(tlsCreds)}
	if *token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(tokenCreds(*token)))
	}
	conn, err := grpc.NewClient(*addr, opts...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "连接失败:", err)
		os.Exit(1)
	}
	defer conn.Close()

	cp, err := pb.NewRuntimeServiceClient(conn).GetControlPlane(ctx, &pb.GetControlPlaneRequest{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "GetControlPlane 失败(旧 daemon 没有此 RPC?):", err)
		os.Exit(1)
	}

	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║        PocketAgent daemon · 控制面总览        ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Printf("  主机: %s   版本: %s   系统: %s   数据目录: %s\n",
		cp.HostName, cp.DaemonVersion, cp.Os, cp.DataDirLabel)
	if cp.UptimeUnixMs > 0 {
		fmt.Printf("  已运行: %s\n", humanUptime(cp.UptimeUnixMs))
	}
	fmt.Println()
	for _, d := range cp.Domains {
		status := "●"
		switch d.StatusCode {
		case 1:
			status = "◐" // 注意
		case 2:
			status = "○" // 受限
		}
		w := ""
		if !d.Writable {
			w = "(只读)"
		}
		fmt.Printf("%s %s %s — %s\n", status, d.Title, w, d.Summary)
		for _, it := range d.Items {
			line := fmt.Sprintf("     · %s: %s", it.Label, it.Current)
			if len(it.Options) > 0 {
				line += fmt.Sprintf("  [候选: %s]", strings.Join(it.Options, "/"))
			}
			fmt.Println(line)
			if it.Hint != "" {
				fmt.Printf("       └ %s\n", it.Hint)
			}
		}
		fmt.Println()
	}
	fmt.Println("(● 正常  ◐ 有注意项  ○ 受限;切换请在手机「设置 → 运行时控制面 → 控制中心」操作)")
}

func humanUptime(startedUnixMs int64) string {
	d := time.Since(time.UnixMilli(startedUnixMs))
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d 秒", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时 %d 分钟", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%d 天 %d 小时", int(d.Hours())/24, int(d.Hours())%24)
	}
}

// tokenCreds 按 daemon 鉴权链路的 metadata 约定携带设备 token
// (auth_p0p2:authorization = "Bearer <token>")。
type tokenCreds string

func (t tokenCreds) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + string(t)}, nil
}

func (t tokenCreds) RequireTransportSecurity() bool { return false }
