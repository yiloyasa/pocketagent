package main

// modelprobe:回答一个具体问题 —— 手机端那个"上下文占用环"为什么显示"—"(窗口未知)?
//
// 环显示"—"的条件是 daemon 侧窗口不可知:
//   ChatScreen.kt:848  reportedWindow = max(模型声明窗口, daemon 随用量上报的窗口)
//                      reportedWindow<=0 → windowIsFallback=true → 环内画"—"
// 所以要么"模型声明窗口"取不到,要么"上报窗口"取不到,或者两者都取不到。
//
// 本探针把这两条路各自的地面真值打出来:
//   [1] ListModels            —— 每个模型 id / provider / context_window(模型声明窗口这条路的源头)
//   [2] GetThread(目标会话)     —— 该会话的 model_id(手机端拿它去 ListModels 里查窗口)
//   [3] ListCodexCapabilities —— daemon 自报的 codex 能力(含它认为的窗口能力)
//
// 用法: modelprobe [threadId]   (默认 th-1d4359da55bf2e5e)
import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
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
	tid := "th-1d4359da55bf2e5e"
	if flag.NArg() > 0 {
		tid = flag.Arg(0)
	}
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

	bg := context.Background()

	// ---- 1) 模型列表:这是"模型声明窗口"的唯一来源 ----
	fmt.Println("=== [1] ListModels ===")
	{
		ctx, cancel := context.WithTimeout(bg, 30*time.Second)
		defer cancel()
		t := time.Now()
		res, err := pb.NewModelServiceClient(conn).ListModels(ctx, &pb.ListModelsRequest{})
		fmt.Printf("    (%dms, err=%v, n=%d)\n", time.Since(t).Milliseconds(), err, len(res.GetModels()))
		if res != nil {
			for _, m := range res.GetModels() {
				flag := ""
				if m.GetContextWindow() <= 0 {
					flag = "   <-- 窗口未知,选它会显示 —"
				}
				fmt.Printf("    id=%-34s prov=%-18s cw=%-8d avail=%-5v %s%s\n",
					m.GetId(), m.GetProvider(), m.GetContextWindow(), m.GetAvailable(),
					m.GetDisplayName(), flag)
			}
		}
	}

	// ---- 2) 目标会话的 model_id ----
	fmt.Printf("\n=== [2] GetThread(%s) ===\n", tid)
	var modelID string
	{
		ctx, cancel := context.WithTimeout(bg, 15*time.Second)
		defer cancel()
		th, err := pb.NewThreadServiceClient(conn).GetThread(ctx, &pb.GetThreadRequest{ThreadId: tid})
		if err != nil {
			fmt.Printf("    err=%v\n", err)
		} else {
			modelID = th.GetModelId()
			fmt.Printf("    title=%q model_id=%q workspace=%q status=%v\n",
				th.GetTitle(), th.GetModelId(), th.GetWorkspace(), th.GetStatus())
		}
	}

	// ---- 3) 该 model_id 在列表里能不能查到、窗口是多少(手机端就是这么查的) ----
	fmt.Println("\n=== [3] 手机端解析结果(模拟 ChatScreen 的 effectiveModel 查找)===")
	{
		ctx, cancel := context.WithTimeout(bg, 30*time.Second)
		defer cancel()
		res, _ := pb.NewModelServiceClient(conn).ListModels(ctx, &pb.ListModelsRequest{})
		var hit *pb.ModelInfo
		for _, m := range res.GetModels() {
			if m.GetId() == modelID {
				hit = m
				break
			}
		}
		if hit == nil {
			fmt.Printf("    ❌ 会话 model_id=%q 在模型列表里**找不到**\n", modelID)
			fmt.Printf("       → ChatScreen.kt:840 的 models.find{it.id==effectiveModelId} 落空,\n")
			fmt.Printf("         会退到 runningModel / \"codex/default\" / firstOrNull()\n")
		} else {
			fmt.Printf("    ✅ 命中: id=%s cw=%d → contextWindow=%d, windowIsFallback=%v\n",
				hit.GetId(), hit.GetContextWindow(), hit.GetContextWindow(), hit.GetContextWindow() <= 0)
		}
	}

	// ---- 4) daemon 自报能力(含窗口能力) ----
	fmt.Println("\n=== [4] ListCodexCapabilities ===")
	{
		ctx, cancel := context.WithTimeout(bg, 30*time.Second)
		defer cancel()
		cap, err := pb.NewModelServiceClient(conn).ListCodexCapabilities(ctx,
			&pb.ListCodexCapabilitiesRequest{})
		if err != nil {
			fmt.Printf("    err=%v\n", err)
		} else {
			fmt.Printf("    %v\n", cap.String())
		}
	}
}
