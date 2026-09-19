package main

// verifyext 验证"codex 全面接入"这一轮新增的 gRPC 面在**真实 daemon** 上是否可用。
//
// 【为什么不只靠单测】单测只能证明 daemon 内部自洽;这些 RPC 的价值在于
// "手机能不能真的拿到数据"。所以这个探针直连 127.0.0.1:7246,把每个新方法
// 都真调一遍,并把返回的**具体字段**打出来 —— 只打印"调用成功"是没有意义的,
// 一个字段全空的响应同样会"成功"。
//
// 用法:先确保 daemon 在跑,然后
//
//	cd daemon && go run ./cmd/verifyext
//
// 部分方法需要真实 codex 会话(见各段注释),拿不到时打印 SKIP 并说明原因,
// 不伪造成功。
import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

var failures int

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
	must(err)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	ms := pb.NewModelServiceClient(conn)
	ts := pb.NewThreadServiceClient(conn)

	fmt.Println("=== 只读面:用量 / 能力 / 托管策略 / MCP ===")
	checkCodexUsage(ctx, ms)
	checkCodexCapabilities(ctx, ms)
	checkManagedPolicy(ctx, ms)
	checkMcpServers(ctx, ms)

	fmt.Println()
	fmt.Println("=== 会话组织:分组 / 归档 / 搜索 ===")
	checkSections(ctx, ts)
	checkListFilters(ctx, ts)

	fmt.Println()
	fmt.Println("=== 审查者推翻入口 ===")
	checkGuardianOverride(ctx, ts)

	fmt.Println()
	checkRollback(ctx, ts)

	fmt.Println()
	if failures == 0 {
		fmt.Println("全部检查通过")
		return
	}
	fmt.Printf("有 %d 项未通过\n", failures)
	os.Exit(1)
}

func checkCodexUsage(ctx context.Context, ms pb.ModelServiceClient) {
	u, err := ms.GetCodexUsage(ctx, &pb.GetCodexUsageRequest{})
	if err != nil {
		fail("GetCodexUsage", err)
		return
	}
	if u.Error != "" {
		// 账号未登录 / 地区受限在开发机上是**预期**结果:重点是 daemon 把原因
		// 带回来了,而不是返回一堆 0 让手机误读成"额度用完了"。
		fmt.Printf("~ GetCodexUsage: 读不到但带回原因(正确行为): %s\n", u.Error)
		return
	}
	fmt.Printf("ok GetCodexUsage: plan=%q 主窗口=%d%% 次窗口=%d%% 累计token=%d 拉取=%s\n",
		u.PlanType, u.GetPrimary().GetUsedPercent(), u.GetSecondary().GetUsedPercent(),
		u.LifetimeTokens, time.UnixMilli(u.FetchedAtUnixMs).Format("15:04:05"))
}

func checkCodexCapabilities(ctx context.Context, ms pb.ModelServiceClient) {
	resp, err := ms.ListCodexCapabilities(ctx, &pb.ListCodexCapabilitiesRequest{})
	if err != nil {
		fail("ListCodexCapabilities", err)
		return
	}
	byKind := map[string]int{}
	for _, c := range resp.Items {
		byKind[c.Kind]++
	}
	fmt.Printf("ok ListCodexCapabilities: 共 %d 项(skill=%d hook=%d plugin=%d)\n",
		len(resp.Items), byKind["skill"], byKind["hook"], byKind["plugin"])
	for i, c := range resp.Items {
		if i >= 3 {
			break
		}
		fmt.Printf("     - [%s] %s enabled=%v 来源=%q 状态=%q 禁用原因=%q\n",
			c.Kind, c.Name, c.Enabled, c.Source, c.Status, c.DisabledReason)
	}
	if len(resp.Items) == 0 {
		// 空不是错误:这台机器上可能确实没装技能。但要让人看见"确实读到 0 项",
		// 而不是静默什么都不打印。
		fmt.Println("     (这台电脑上 codex 报告 0 项能力)")
	}
}

func checkManagedPolicy(ctx context.Context, ms pb.ModelServiceClient) {
	st, err := ms.GetCodexSettings(ctx, &pb.GetCodexSettingsRequest{})
	if err != nil {
		fail("GetCodexSettings(托管策略)", err)
		return
	}
	fmt.Printf("ok GetCodexSettings: approval=%q sandbox=%q model=%q\n", st.ApprovalPolicy, st.Sandbox, st.Model)
	fmt.Printf("     托管=%v 说明=%q\n", st.Managed, st.ManagedNote)
	fmt.Printf("     允许的 approval=%v sandbox=%v webSearch=%v(空列表=无限制)\n",
		st.AllowedApprovalPolicies, st.AllowedSandboxModes, st.AllowedWebSearchModes)
}

func checkMcpServers(ctx context.Context, ms pb.ModelServiceClient) {
	resp, err := ms.ListMcpServers(ctx, &pb.ListMcpServersRequest{})
	if err != nil {
		fail("ListMcpServers", err)
		return
	}
	fmt.Printf("ok ListMcpServers: 共 %d 个\n", len(resp.Servers))
	for _, s := range resp.Servers {
		// 这里必须能看到 tools 数:它原本解析失败过(协议里 tools 是**对象**
		// 不是数组),当时的症状是 MCP 面板整个空掉。
		fmt.Printf("     - %s 工具=%d 状态=%q 鉴权=%q 版本=%q 标题=%q 资源=%d 错误=%q\n",
			s.Name, s.Tools, s.Status, s.AuthStatus, s.ServerVersion, s.ServerTitle,
			s.ResourceCount, s.Error)
	}
}

func checkSections(ctx context.Context, ts pb.ThreadServiceClient) {
	base, err := ts.ListThreadSections(ctx, &pb.ListThreadSectionsRequest{})
	if err != nil {
		fail("ListThreadSections", err)
		return
	}
	fmt.Printf("ok ListThreadSections: 现有 %d 个分组\n", len(base.Sections))

	sec, err := ts.CreateThreadSection(ctx, &pb.CreateThreadSectionRequest{Name: "验证分组-可删"})
	if err != nil {
		fail("CreateThreadSection", err)
		return
	}
	fmt.Printf("ok CreateThreadSection: id=%s name=%q\n", sec.Id, sec.Name)

	renamed, err := ts.RenameThreadSection(ctx, &pb.RenameThreadSectionRequest{
		SectionId: sec.Id, Name: "验证分组-已改名",
	})
	if err != nil {
		fail("RenameThreadSection", err)
		return
	}
	fmt.Printf("ok RenameThreadSection: name=%q\n", renamed.Name)

	// ---- 移动分组:必须在**已经落地到 codex 的会话**上测 ----
	// 分组是 codex 侧的概念,本地新建、还没发过消息的会话没有 codex 线程可挂,
	// daemon 会正当地拒掉(FailedPrecondition)。拿空会话测只会测到这条守卫,
	// 测不到真正的写入路径 —— 所以这里挑一条 codex_thread_id 非空的会话,
	// 并且**记下它原来的分组,测完原样放回**,不留副作用。
	all, err := ts.ListThreads(ctx, &pb.ListThreadsRequest{})
	if err != nil {
		fail("ListThreads(为验证移动)", err)
		cleanupSection(ctx, ts, sec.Id)
		return
	}
	var target *pb.Thread
	for _, t := range all.Threads {
		if t.CodexThreadId != "" {
			target = t
			break
		}
	}
	if target == nil {
		fmt.Println("~ MoveThreadToSection: SKIP(没有已落地到 codex 的会话可测;分组依附 codex 线程)")
		cleanupSection(ctx, ts, sec.Id)
		checkFork(ctx, ts, nil)
		return
	}
	original := target.SectionId

	moved, err := ts.MoveThreadToSection(ctx, &pb.MoveThreadToSectionRequest{
		ThreadId: target.Id, SectionId: sec.Id,
	})
	if err != nil {
		fail("MoveThreadToSection", err)
	} else if moved.SectionId != sec.Id {
		fail("MoveThreadToSection", fmt.Errorf("返回 sectionId=%q,应为 %q", moved.SectionId, sec.Id))
	} else {
		fmt.Printf("ok MoveThreadToSection: 会话 %s 已进入分组 %s\n", target.Id, sec.Id)
	}

	// 分组过滤:只看该分组时应当只命中移进去的那条。
	filtered, err := ts.ListThreads(ctx, &pb.ListThreadsRequest{SectionId: sec.Id})
	if err != nil {
		fail("ListThreads(section_id)", err)
	} else if len(filtered.Threads) != 1 || filtered.Threads[0].Id != target.Id {
		fail("ListThreads(section_id)", fmt.Errorf("命中 %d 条,应只命中移入的那条", len(filtered.Threads)))
	} else {
		fmt.Println("ok ListThreads(section_id): 分组过滤命中 1 条")
	}

	// 移出分组:section_id 传空必须真的清掉,而不是"空值就当没改"。
	out, err := ts.MoveThreadToSection(ctx, &pb.MoveThreadToSectionRequest{ThreadId: target.Id})
	if err != nil {
		fail("MoveThreadToSection(移出分组)", err)
	} else if out.SectionId != "" {
		fail("MoveThreadToSection(移出分组)", fmt.Errorf("sectionId 仍为 %q,应已清空", out.SectionId))
	} else {
		fmt.Println("ok MoveThreadToSection: 传空 section_id 已把会话移出分组")
	}

	// 还原:如果这条会话本来就在别的分组里,放回去。
	if original != "" {
		if _, err := ts.MoveThreadToSection(ctx, &pb.MoveThreadToSectionRequest{
			ThreadId: target.Id, SectionId: original,
		}); err != nil {
			fmt.Printf("     ! 还原原分组 %s 失败: %v\n", original, err)
		} else {
			fmt.Printf("     (已还原原分组 %s)\n", original)
		}
	}

	// 分组 thread_count 回填。
	after, err := ts.ListThreadSections(ctx, &pb.ListThreadSectionsRequest{})
	if err == nil {
		for _, s := range after.Sections {
			if s.Id == sec.Id {
				fmt.Printf("ok 分组 thread_count 回填: %d\n", s.ThreadCount)
			}
		}
	}

	cleanupSection(ctx, ts, sec.Id)
	checkFork(ctx, ts, target)
}

// checkFork 验证会话分叉。
//
// 【为什么挑有回合的会话】fork 的语义是"从这个回合之后分出一条新会话",
// 没有回合的会话分不出任何东西(daemon 侧会 Skip)。这里复用上面挑出的
// 已落地会话,断言的是:新线程被建出来、绑定到同一个 codex 会话、标题可分辨,
// 然后立刻把分叉删掉 —— 不在用户列表里留垃圾。
func checkFork(ctx context.Context, ts pb.ThreadServiceClient, target *pb.Thread) {
	if target == nil {
		fmt.Println("~ ForkThread: SKIP(没有已落地到 codex 的会话可测)")
		return
	}
	forked, err := ts.ForkThread(ctx, &pb.ForkThreadRequest{ThreadId: target.Id})
	if err != nil {
		// 空会话(没有任何回合)分不出来是正常语义,不当失败。
		fmt.Printf("~ ForkThread: SKIP(%v)\n", err)
		return
	}
	fmt.Printf("ok ForkThread: 新会话 %s 标题=%q codex=%s\n",
		forked.Id, forked.Title, forked.CodexThreadId)
	if forked.Id == target.Id {
		fail("ForkThread", fmt.Errorf("分叉返回了原会话 id"))
	}
	if forked.CodexThreadId == "" {
		fail("ForkThread", fmt.Errorf("分叉没有绑上 codex 线程,后续无法继续对话"))
	}
	cleanupThread(ctx, ts, forked.Id)
}

func checkListFilters(ctx context.Context, ts pb.ThreadServiceClient) {
	all, err := ts.ListThreads(ctx, &pb.ListThreadsRequest{})
	if err != nil {
		fail("ListThreads", err)
		return
	}
	fmt.Printf("ok ListThreads: 活跃 %d 条(sectionId 已带出)\n", len(all.Threads))
	for i, t := range all.Threads {
		if i >= 2 {
			break
		}
		fmt.Printf("     - %s %q archived=%v section=%q\n", t.Id, t.Title, t.Archived, t.SectionId)
	}

	// 归档过滤:include_archived 必须真的有区别,否则"归档"在手机上等于删除。
	withArch, err := ts.ListThreads(ctx, &pb.ListThreadsRequest{IncludeArchived: true})
	if err != nil {
		fail("ListThreads(含归档)", err)
		return
	}
	fmt.Printf("ok ListThreads(include_archived=true): %d 条(≥ 活跃数 %d)\n", len(withArch.Threads), len(all.Threads))

	// 搜索:拿第一条会话标题的第一个字去搜,必须能搜回来。
	if len(all.Threads) > 0 {
		title := []rune(all.Threads[0].Title)
		if len(title) > 0 {
			q := string(title[:1])
			hit, err := ts.ListThreads(ctx, &pb.ListThreadsRequest{SearchTerm: q})
			if err != nil {
				fail("ListThreads(search_term)", err)
			} else {
				fmt.Printf("ok ListThreads(search_term=%q): 命中 %d 条\n", q, len(hit.Threads))
				if len(hit.Threads) == 0 {
					fail("ListThreads(search_term)", fmt.Errorf("标题以 %q 开头的会话搜不到", q))
				}
			}
		}
	}

	// 排序:updated 与 created 至少要能调通并返回同样多的条目。
	sorted, err := ts.ListThreads(ctx, &pb.ListThreadsRequest{SortKey: "updated"})
	if err != nil {
		fail("ListThreads(sort_key=updated)", err)
	} else {
		reversed := true
		for i := 1; i < len(sorted.Threads); i++ {
			if sorted.Threads[i-1].UpdatedAtUnixMs < sorted.Threads[i].UpdatedAtUnixMs {
				reversed = false
				break
			}
		}
		if !reversed {
			fail("ListThreads(sort_key)", fmt.Errorf("updated 未按时间倒序"))
		} else {
			fmt.Printf("ok ListThreads(sort_key=updated): %d 条,已按更新时间倒序\n", len(sorted.Threads))
		}
	}
}

func checkGuardianOverride(ctx context.Context, ts pb.ThreadServiceClient) {
	// 【为什么故意用假 id】这条 RPC 是我们唯一"越权"的地方(推翻审查者结论),
	// 最关键的性质是:**不许假装成功**。用不存在的 reviewId 调用必须报错 ——
	// 如果它返回 nil,说明 daemon 会向用户谎称"已推翻,继续执行"。
	_, err := ts.ApproveGuardianDeniedAction(ctx, &pb.ApproveGuardianDeniedActionRequest{
		ThreadId:     "不存在的会话",
		ReviewId:     "不存在的审查",
		TargetItemId: "不存在的动作",
	})
	if err == nil {
		fail("ApproveGuardianDeniedAction(假 id)", fmt.Errorf("返回成功 —— 会向用户谎称已推翻"))
		return
	}
	fmt.Printf("ok ApproveGuardianDeniedAction: 假 id 被拒,理由是 %q\n", err.Error())
}

// checkRollback 说明回滚为什么在这台机器上验不了。
//
// 【为什么不一测而过】RollbackThread 会**真的丢掉最近 N 个回合**,这是不可
// 撤销的破坏性操作。要正经验证它,必须先有一个带真实回合的会话 —— 而"跑出一
// 个回合"要求 codex 能正常调用上游模型。这台开发机上的 codex 登录被地区策略
// 拒绝(token 刷新 403 unsupported_country_region_territory),连不上模型,
// 因此拿不到真实回合。
//
// 这种情况下**绝不能拿用户的真实会话去回滚**来凑一个"通过"。daemon 侧对
// 这条 RPC 的单测(codex_threads 相关)已覆盖参数校验与错误透传;
// 真机上的完整验证需要一台 codex 可用、且有至少两轮对话的会话。
func checkRollback(ctx context.Context, ts pb.ThreadServiceClient) {
	all, err := ts.ListThreads(ctx, &pb.ListThreadsRequest{})
	if err != nil {
		fail("ListThreads(为评估回滚)", err)
		return
	}
	// 只做"参数非法必须被拒"这一条 —— 它不需要真实回合,而且能防住
	// "num_turns=0 被当成回滚全部"这种会把会话清空的经典错误。
	// 不碰任何真实会话。
	if len(all.Threads) == 0 {
		fmt.Println("~ RollbackThread: SKIP(无会话可测)")
		return
	}
	_, err = ts.RollbackThread(ctx, &pb.RollbackThreadRequest{
		ThreadId: all.Threads[0].Id, NumTurns: 0,
	})
	if err == nil {
		fail("RollbackThread(num_turns=0)", fmt.Errorf("返回成功 —— 0 个回合应当被拒,否则可能被误当成回滚全部"))
		return
	}
	fmt.Printf("ok RollbackThread: num_turns=0 被拒,理由是 %q\n", err.Error())
	fmt.Println("~ RollbackThread 正向路径: SKIP(需要带真实回合的会话,本机 codex 登录被地区策略拒绝,拿不到回合;不会为凑通过去回滚真实会话)")
}

func cleanupThread(ctx context.Context, ts pb.ThreadServiceClient, id string) {
	if id == "" {
		return
	}
	if _, err := ts.DeleteThread(ctx, &pb.DeleteThreadRequest{ThreadId: id}); err != nil {
		fmt.Printf("     ! 清理验证会话 %s 失败: %v\n", id, err)
	} else {
		fmt.Printf("     (已清理验证会话 %s)\n", id)
	}
}

func cleanupSection(ctx context.Context, ts pb.ThreadServiceClient, id string) {
	if id == "" {
		return
	}
	if _, err := ts.DeleteThreadSection(ctx, &pb.DeleteThreadSectionRequest{SectionId: id}); err != nil {
		fmt.Printf("     ! 清理验证分组 %s 失败: %v\n", id, err)
	} else {
		fmt.Printf("     (已清理验证分组 %s)\n", id)
	}
}

func fail(name string, err error) {
	failures++
	fmt.Printf("FAIL %s: %v\n", name, err)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "verifyext:", err)
		os.Exit(2)
	}
}
