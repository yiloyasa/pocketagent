package main

// wbempty:设置/回读单个 codex 偏好字段的定案工具(直连 daemon,不经手机)。
//
// 【用途一 · 定案】真机复核出现"UI 显示跟随桌面、daemon 回读仍是具体值、store 未被写过"。
// 本工具把这团矛盾拆成两个可独立回答的问题:
//   Q1 daemon 能不能存空串?  —— 盯 store 落盘文件即可看到真实字节(不靠任何一层的回显)
//   Q2 daemon 的 gRPC 层还活着吗? —— 先调一个**不碰 codex**的 RPC(GetHostInfo) 探活
//
// 【用途二 · 回滚】探针会把 store 改掉,必须能原样还回去。所以从 argv 取值:
//
//	go run ./cmd/wbempty ""       # 写成"跟随桌面"
//	go run ./cmd/wbempty never    # 还回具体值
//
// 【为什么用短 ctx】SetCodexSettings 的落盘发生在"探测 codex 可达性"之前,
// 毫秒级就完成;之后的回读环节可能长时间阻塞(见下),所以给一个短超时,
// 超时不影响已落盘的结果 —— 这本身也是"落盘先于阻塞"的实验证据。
import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pocketagent/daemon/internal/hostenv"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// storePath 定位 daemon 的 threads.json:优先迁移后的数据目录
// (<LOCALAPPDATA>\PocketAgent\data),回退历史位置(家目录 .pocketagentd)。
//
// 【为什么不再写死绝对路径】原实现把开发机的家目录写成常量,会随探针二进制
// 分发(规格 §2 禁止),换机必失效。数据目录的真实解析规则由 internal/hostenv
// 统一持有,这里只做**只读**定位(现存的优先,都不存在时回退历史位置,
// 让读失败的报错仍然指向一个说得通的位置);不调 hostenv.ResolveDataDir ——
// 那会触发一次性迁移副作用,探针不该替 daemon 做迁移。
func storePath() string {
	for _, dir := range []string{hostenv.UserDataDir(), hostenv.LegacyDataDir()} {
		p := filepath.Join(dir, "threads.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(hostenv.LegacyDataDir(), "threads.json")
}

func ps(s string) *string { return &s }

func codexSection() string {
	b, err := os.ReadFile(storePath())
	if err != nil {
		return "读失败:" + err.Error()
	}
	var v struct {
		Codex json.RawMessage `json:"codex"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return "解析失败:" + err.Error()
	}
	return string(v.Codex)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:7246", "daemon 地址(ip:port)")
	plaintext := flag.Bool("plaintext", false, "连明文 gRPC(对付 -plaintext 起的测试 daemon);默认 TLS")
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatal("用法: wbempty <approval_policy 目标值;空串表示跟随桌面>")
	}
	target := flag.Arg(0)

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
		log.Fatal(err)
	}
	defer conn.Close()

	fmt.Printf("改前 codex 段: %s\n", codexSection())

	// 探活:不碰 codex 的 RPC。用来把"模型服务卡住"与"daemon 整体卡住"区分开。
	func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		start := time.Now()
		_, err := pb.NewHostServiceClient(conn).GetHostInfo(ctx, &pb.GetHostInfoRequest{})
		fmt.Printf("探活 GetHostInfo → %v 耗时 %dms\n", err, time.Since(start).Milliseconds())
	}()

	// 盯盘:SetCodexSettings 可能不返回,但落盘一定会发生 —— 靠它拿证据。
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		last := codexSection()
		for {
			select {
			case <-done:
				return
			case <-time.After(300 * time.Millisecond):
				if cur := codexSection(); cur != last {
					fmt.Printf("  [盯盘] 已落盘 → %s\n", cur)
					last = cur
				}
			}
		}
	}()

	start := time.Now()
	st, err := pb.NewModelServiceClient(conn).SetCodexSettings(ctx, &pb.SetCodexSettingsRequest{ApprovalPolicy: ps(target)})
	el := time.Since(start).Milliseconds()
	if err != nil {
		fmt.Printf("SetCodexSettings 返回错误(%dms): %v\n", el, err)
	} else {
		fmt.Printf("SetCodexSettings 返回(%dms): approval=%q\n", el, st.ApprovalPolicy)
	}
	time.Sleep(700 * time.Millisecond)
	close(done)
	fmt.Printf("改后 codex 段: %s\n", codexSection())
}
