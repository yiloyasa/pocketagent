package server

// slash_commands_ledger_test.go —— 真 *agent.ZcodeProvider 接入后的切换行为钉子。
//
// 【钉什么】zcode.go 实现 SlashCommandsFor 后,server 的 zcodeSlashSource 断言
// 必然成立(生产注册表里就是 *agent.ZcodeProvider),此时行为必须等价:
//   - 账本未记账(daemon 刚启动/线程没跑过轮次)→ 返回 nil → 回退到一次性
//     子进程 resume 兜底路径(fake 拉取被调用,命令照常透出);
//   - 兜底路径仍带 10 分钟缓存(与切账本之前同款,二次调用不再拉取)。
//
// 账本命中通路已由 slash_commands_test.go 的 TestListSlashCommandsProviderLedgerWins
// 用 fake 钉住,这里不重复;本文件专注"真 provider + 未记账"的回退等价性。

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func TestListSlashCommandsRealProviderLedgerMissFallsBackToResume(t *testing.T) {
	slashCacheFlush()
	calls := 0
	restore := zcodeSlashFetch
	zcodeSlashFetch = func(ctx context.Context, sessionID string) ([]*pb.SlashCommand, error) {
		calls++
		if sessionID != "sess_zc5" {
			t.Fatalf("应按线程绑定的会话 id 兜底拉取,得到 %q", sessionID)
		}
		return []*pb.SlashCommand{{Name: "goal", Description: "常驻目标", InputHint: "/goal [pause|resume|…]"}}, nil
	}
	defer func() { zcodeSlashFetch = restore }()
	// 生产同款:注册表里放真 *agent.ZcodeProvider(已实现 zcodeSlashSource),
	// 但它从未记账(没跑过任何轮次)→ 必须回退 resume 兜底,而不是回空列表。
	srv := newSlashServer(t, map[string]agent.Provider{"zcode": agent.NewZcodeProvider()},
		&store.Thread{ID: "th-zc5", Title: "zcode 会话", AgentID: "zcode",
			ExternalSession: store.ExternalSessionRef{SessionID: "sess_zc5"}})
	req := &pb.ListSlashCommandsRequest{ThreadId: "th-zc5"}
	resp, err := srv.ListSlashCommands(context.Background(), req)
	if err != nil {
		t.Fatalf("回退兜底不应报错: %v", err)
	}
	if calls != 1 {
		t.Fatalf("未记账应回退到 resume 兜底(fake 被调 %d 次)", calls)
	}
	// 兜底命令字段映射不变;聚合层的 fork/rewind 漏报补充照样追加(共 3 条)。
	if len(resp.Commands) != 3 || resp.Commands[0].Name != "goal" ||
		resp.Commands[0].InputHint != "/goal [pause|resume|…]" {
		t.Fatalf("兜底命令字段映射不符: %+v", resp.Commands)
	}
	if resp.Commands[1].Name != "fork" || resp.Commands[2].Name != "rewind" {
		t.Fatalf("兜底路径也应带漏报补充: %+v", resp.Commands)
	}
	// 兜底路径的 10 分钟缓存仍在:第二次调用不再拉取(与切账本前等价)。
	if _, err := srv.ListSlashCommands(context.Background(), req); err != nil {
		t.Fatalf("二次调用不应报错: %v", err)
	}
	if calls != 1 {
		t.Fatalf("兜底缓存应生效(fake 被调 %d 次)", calls)
	}
}
