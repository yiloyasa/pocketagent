package main

// wbfix:验证"模型服务不再无限期阻塞"这一修复**是否真的热部署到了运行中的 daemon**。
//
// 【为什么必须单独有这么一个工具】源码改对了不等于跑着的进程改对了 —— 本项目已经
// 反复踩过"装的是不是新包/新 exe"这个坑。这个探针只回答一个问题:
//   现在这台机器上正在跑的 pocketagentd,一次设置读写要多久返回?
//
// 三个测量,顺序固定:
//  1. GetHostInfo        —— 不碰 codex 的 RPC,基线。它慢说明是 gRPC 层的问题。
//  2. GetCodexSettings   —— 修复前:它会走进 codex 探测,codex 拉不起来时**永不返回**;
//                           修复后:受探测短上界约束,必然在有界时间内返回,
//                           并把逐项状态降级为 unreachable(而不是死等)。
//  3. SetCodexSettings   —— 写**当前值本身**(幂等,不改任何配置),
//                           修复前同样卡在第 4 步的回读上。
//
// 判定:两个设置调用都在 SET_BUDGET 内返回即 PASS;否则 FAIL 并打印实测耗时。
import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// SET_BUDGET 是"还算是人能用"的上界。修复后 daemon 的探测上界是 8s,
// 这里留一倍余量;超过它就意味着无界阻塞还在。
//
// 【删除说明】本文件曾有一个写死开发机绝对路径的 storePath 常量
// (家目录 .pocketagentd 下的 threads.json),但全文无任何引用 —— 属纯死代码,
// 且字符串常量会随探针二进制分发(规格 §2 禁止),直接删除,不留个人路径痕迹。
const SET_BUDGET = 20 * time.Second

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
		log.Fatal(err)
	}
	defer conn.Close()

	// ---- 1) 基线 ----
	func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		t := time.Now()
		_, err := pb.NewHostServiceClient(conn).GetHostInfo(ctx, &pb.GetHostInfoRequest{})
		fmt.Printf("[1] GetHostInfo        %6dms  err=%v\n", time.Since(t).Milliseconds(), err)
	}()

	// ---- 2) 读设置(修复前会永不返回) ----
	var cur string
	{
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		t := time.Now()
		st, err := pb.NewModelServiceClient(conn).GetCodexSettings(ctx, &pb.GetCodexSettingsRequest{})
		el := time.Since(t)
		if err != nil {
			fmt.Printf("[2] GetCodexSettings  %6dms  err=%v\n", el.Milliseconds(), err)
		} else {
			cur = st.ApprovalPolicy
			fmt.Printf("[2] GetCodexSettings  %6dms  approval=%q\n", el.Milliseconds(), st.ApprovalPolicy)
			for _, s := range st.SettingStatus {
				fmt.Printf("      %-24s state=%-14s %s\n", s.Key, s.State, s.Detail)
			}
		}
		if el > SET_BUDGET {
			fmt.Printf("    ❌ 读设置超过上界 %s —— 无界阻塞可能还在\n", SET_BUDGET)
		}
	}

	// ---- 3) 写回"当前值本身"(幂等) ----
	if cur != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		v := cur
		t := time.Now()
		st, err := pb.NewModelServiceClient(conn).SetCodexSettings(ctx, &pb.SetCodexSettingsRequest{ApprovalPolicy: &v})
		el := time.Since(t)
		if err != nil {
			fmt.Printf("[3] SetCodexSettings  %6dms  err=%v\n", el.Milliseconds(), err)
		} else {
			fmt.Printf("[3] SetCodexSettings  %6dms  approval=%q (与写入值一致=%v)\n",
				el.Milliseconds(), st.ApprovalPolicy, st.ApprovalPolicy == v)
		}
		if el > SET_BUDGET {
			fmt.Printf("    ❌ 写设置超过上界 %s —— 无界阻塞可能还在\n", SET_BUDGET)
			os.Exit(1)
		}
		fmt.Println("✅ PASS:设置读写都在有界时间内返回(不再无限期阻塞)")
	} else {
		fmt.Println("⚠️ 读设置没拿到值(可能超时),跳过写回")
		os.Exit(1)
	}
}
