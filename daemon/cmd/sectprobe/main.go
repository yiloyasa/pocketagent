package main

// sectprobe —— 临时探针:分组读写 + 会话列表 ground truth(验证与清理用)。
//
// 【为什么要看 ground truth】真机 UI 上"看不到某个分组/chip 被裁掉没显示"
// 与"分组真的没建出来"在 dump 里长得很像(被完全裁掉的节点会直接不出现在
// 视图树里)。所以涉及"到底存没存下来"的判断一律回到 daemon 直接问。
//
// 用法:
//
//	cd daemon && go run ./cmd/sectprobe -mode list
//	cd daemon && go run ./cmd/sectprobe -mode probe    # 验证 create→list→delete 往返
//	cd daemon && go run ./cmd/sectprobe -mode del -id th-xxxx
//	cd daemon && go run ./cmd/sectprobe -mode cleanforks   # 删掉消息数为 0 的分叉残留
import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	mode := flag.String("mode", "list", "list | probe | del | cleanforks")
	id := flag.String("id", "", "del 模式要删的 threadId")
	flag.Parse()

	conn, err := grpc.NewClient("127.0.0.1:7246", grpc.WithTransportCredentials(insecure.NewCredentials()))
	must(err)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	ts := pb.NewThreadServiceClient(conn)

	switch *mode {
	case "list":
		listSections(ctx, ts)
		listThreads(ctx, ts)
	case "probe":
		probe(ctx, ts)
	case "del":
		if *id == "" {
			fmt.Println("del 模式需要 -id")
			os.Exit(2)
		}
		del(ctx, ts, *id)
	case "delsection":
		if *id == "" {
			fmt.Println("delsection 模式需要 -id")
			os.Exit(2)
		}
		if _, err := ts.DeleteThreadSection(ctx, &pb.DeleteThreadSectionRequest{SectionId: *id}); err != nil {
			fmt.Printf("删除分组 %s 失败: %v\n", *id, err)
			os.Exit(1)
		}
		fmt.Println("已删除分组", *id)
		listSections(ctx, ts)
	case "cleanforks":
		cleanForkLeftovers(ctx, ts)
	default:
		fmt.Println("未知 mode:", *mode)
		os.Exit(2)
	}
}

func listSections(ctx context.Context, ts pb.ThreadServiceClient) {
	r, err := ts.ListThreadSections(ctx, &pb.ListThreadSectionsRequest{})
	if err != nil {
		fmt.Println("ListThreadSections 失败:", err)
		return
	}
	fmt.Printf("分组 %d 个:\n", len(r.Sections))
	for _, s := range r.Sections {
		fmt.Printf("  %s  %-16q n=%d\n", s.Id, s.Name, s.ThreadCount)
	}
}

func listThreads(ctx context.Context, ts pb.ThreadServiceClient) {
	r, err := ts.ListThreads(ctx, &pb.ListThreadsRequest{IncludeArchived: true})
	if err != nil {
		fmt.Println("ListThreads 失败:", err)
		return
	}
	fmt.Printf("会话 %d 个:\n", len(r.Threads))
	for _, t := range r.Threads {
		fmt.Printf("  %-18s msgs=%-3d sec=%-6s codex=%-14s %q\n",
			t.Id, t.MessageCount, orDash(t.SectionId), short(t.CodexThreadId), clip(t.Title, 28))
	}
}

func probe(ctx context.Context, ts pb.ThreadServiceClient) {
	before, err := ts.ListThreadSections(ctx, &pb.ListThreadSectionsRequest{})
	if err != nil {
		fmt.Println("ListThreadSections 失败:", err)
		os.Exit(1)
	}
	fmt.Printf("BEFORE 分组 %d 个\n", len(before.Sections))

	created, err := ts.CreateThreadSection(ctx, &pb.CreateThreadSectionRequest{Name: "probe-alpha"})
	if err != nil {
		fmt.Println("CreateThreadSection 失败:", err)
		os.Exit(1)
	}
	fmt.Printf("CREATE 返回 id=%s name=%q\n", created.Id, created.Name)

	after, err := ts.ListThreadSections(ctx, &pb.ListThreadSectionsRequest{})
	if err != nil {
		fmt.Println("ListThreadSections 失败:", err)
		os.Exit(1)
	}
	persisted := false
	for _, s := range after.Sections {
		if s.Id == created.Id {
			persisted = true
		}
	}
	fmt.Printf("==> 写入后能被列出来: %v(BEFORE %d → AFTER %d)\n",
		persisted, len(before.Sections), len(after.Sections))

	if _, err := ts.DeleteThreadSection(ctx, &pb.DeleteThreadSectionRequest{SectionId: created.Id}); err != nil {
		fmt.Println("清理失败(需手动删):", err)
	} else {
		fmt.Println("已清理探针分组")
	}
}

func del(ctx context.Context, ts pb.ThreadServiceClient, id string) {
	if _, err := ts.DeleteThread(ctx, &pb.DeleteThreadRequest{ThreadId: id}); err != nil {
		fmt.Printf("删除 %s 失败: %v\n", id, err)
		os.Exit(1)
	}
	fmt.Println("已删除", id)
}

// cleanForkLeftovers 只删"消息数为 0 且绑定了 codex 线程"的会话 ——
// 那正是分叉在修好历史之前留下的残留(新分叉现在会带上历史)。
func cleanForkLeftovers(ctx context.Context, ts pb.ThreadServiceClient) {
	r, err := ts.ListThreads(ctx, &pb.ListThreadsRequest{IncludeArchived: true})
	if err != nil {
		fmt.Println("ListThreads 失败:", err)
		os.Exit(1)
	}
	for _, t := range r.Threads {
		if t.MessageCount == 0 && t.CodexThreadId != "" {
			if _, err := ts.DeleteThread(ctx, &pb.DeleteThreadRequest{ThreadId: t.Id}); err != nil {
				fmt.Printf("  删除 %s 失败: %v\n", t.Id, err)
			} else {
				fmt.Printf("  已删除空会话残留 %s %q\n", t.Id, clip(t.Title, 24))
			}
		}
	}
	listThreads(ctx, ts)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s[:min(6, len(s))]
}

func short(s string) string {
	if len(s) > 13 {
		return s[:13]
	}
	return s
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
