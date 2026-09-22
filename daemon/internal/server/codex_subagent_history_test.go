package server

// codex_subagent_history_test.go —— T-SUBVIS 回归钉子:子代理对话回填
// (codex_subagent_history.go)。
//
// 【钉的是什么】该文件是"存根只有标题没有内容"这个缺口(用户报障:看不到子
// 智能体对话)的补全实现。这里钉三件事:
//  1. clipRunesOr 的 rune 安全截断(n<=0 不设限;超限补省略号;中文不劈半个字);
//  2. subagentSnapshotMessages 的落库规则 —— 有正文的 item 才落(role 文本 /
//     命令执行 / 协作调用),reasoning 与纯活动刻度不落(诚实边界:造不出对话);
//  3. maybePullCodexSubagentHistory 的早退面 —— 非 codex / 非存根 / 无父链时
//     零副作用返回(这些分支在 provider 能力判定之前,测试环境无需真 codex)。

import (
	"context"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func TestClipRunesOr(t *testing.T) {
	// 不超限:原样返回(不补省略号 —— 截断标注只在真截断时出现)。
	if got := clipRunesOr("abc", 5); got != "abc" {
		t.Fatalf("未超限应原样返回, got %q", got)
	}
	if got := clipRunesOr("abc", 3); got != "abc" {
		t.Fatalf("恰好等于上限应原样返回, got %q", got)
	}
	// 超限:截到 n 个 rune 再补省略号。
	if got := clipRunesOr("abcdef", 3); got != "abc…" {
		t.Fatalf("超限应截断并补省略号, got %q", got)
	}
	// n<=0 视为不设限(调用方可传 0 关闭截断)。
	if got := clipRunesOr("abcdef", 0); got != "abcdef" {
		t.Fatalf("n=0 应不截断, got %q", got)
	}
	if got := clipRunesOr("abcdef", -1); got != "abcdef" {
		t.Fatalf("n<0 应不截断, got %q", got)
	}
	// rune 边界:中文按字符数截,不按字节 —— 截 2 个应得完整两个汉字。
	if got := clipRunesOr("中文测试", 2); got != "中文…" {
		t.Fatalf("中文应按 rune 截断, got %q", got)
	}
}

// storedText 取一条落库消息的首个文本块(测试断言用)。
func storedText(m store.StoredMessage) string {
	for i := range m.Blocks {
		if t, ok := m.Blocks[i].Block.(*pb.ContentBlock_Text); ok {
			return t.Text.Text
		}
	}
	return ""
}

func TestSubagentSnapshotMessagesFallsBackToTextOnly(t *testing.T) {
	exit := 0
	snap := &agent.CodexThreadSnapshot{
		Turns: []agent.CodexThreadTurn{{
			ID: "turn-1",
			Items: []agent.CodexTurnItem{
				{Type: "userMessage", Text: "  写个调研  "},  // 首尾空白 trim 后落库
				{Type: "agentMessage", Text: "好的"},       // assistant 文本
				{Type: "reasoning", Text: "内部思考"},        // 无正文可读:不落
				{Type: "subAgentActivity", Text: "它还活着"}, // 纯心跳:不落
				{Type: "userMessage", Text: "   "},       // 空白:跳过
				{Type: "commandExecution", Command: "go test ./...", Output: "PASS", ExitCode: &exit},
				{Type: "commandExecution", Command: "noop"}, // 无输出/无退出码:只落命令
				{Type: "collabAgentToolCall", CollabTool: "spawnAgent",
					CollabStatus: "completed", ReceiverThreadIDs: []string{"c1", "c2"}},
				{Type: "collabAgentToolCall", CollabTool: "spawnAgent",
					CollabStatus: "completed"}, // 无 receiver:不落(没有可透传的归属信息)
			},
		}},
	}

	msgs := subagentSnapshotMessages(snap)
	if len(msgs) != 5 {
		t.Fatalf("应落 5 条(user/assistant/命令×2/协作), got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || storedText(msgs[0]) != "写个调研" {
		t.Fatalf("第 1 条应是 trim 后的 user 文本, got role=%q text=%q", msgs[0].Role, storedText(msgs[0]))
	}
	if msgs[1].Role != "assistant" || storedText(msgs[1]) != "好的" {
		t.Fatalf("第 2 条应是 assistant 文本, got role=%q text=%q", msgs[1].Role, storedText(msgs[1]))
	}
	// 命令条目:带 [命令] 前缀 + 退出码 + 输出(展示标注不是改写内容)。
	cmd := storedText(msgs[2])
	if !strings.Contains(cmd, "[命令] go test ./...") ||
		!strings.Contains(cmd, "退出码 0") || !strings.Contains(cmd, "PASS") {
		t.Fatalf("命令条目应含命令/退出码/输出, got %q", cmd)
	}
	// 无输出的命令:不应出现空输出行。
	if got := storedText(msgs[3]); strings.Contains(got, "\n\n") || !strings.Contains(got, "[命令] noop") {
		t.Fatalf("无输出命令不应追加空行, got %q", got)
	}
	// 协作调用:工具名 + 状态 + 线程 id 都透传。
	collab := storedText(msgs[4])
	if !strings.Contains(collab, "spawnAgent") || !strings.Contains(collab, "completed") ||
		!strings.Contains(collab, "c1, c2") {
		t.Fatalf("协作调用条目应含工具/状态/接收线程, got %q", collab)
	}
}

func TestSubagentSnapshotMessagesEmptySnapshot(t *testing.T) {
	if got := subagentSnapshotMessages(nil); got != nil {
		t.Fatalf("nil 快照应返回 nil(不造空消息), got %+v", got)
	}
	if got := subagentSnapshotMessages(&agent.CodexThreadSnapshot{}); len(got) != 0 {
		t.Fatalf("无 turn 的快照应返回空, got %+v", got)
	}
}

// TestMaybePullCodexSubagentHistoryEarlyReturns 钉早退面:非目标线程一律零副作用
// 返回 false。这些分支在 provider 能力判定之前 —— 所以测试环境不需要真 codex。
func TestMaybePullCodexSubagentHistoryEarlyReturns(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	if s.maybePullCodexSubagentHistory(ctx, nil) {
		t.Fatal("nil 线程应返回 false")
	}
	cases := []struct {
		name string
		th   *store.Thread
	}{
		{"非 codex 线程", &store.Thread{ID: "x1", AgentID: "pi", CodexThreadID: "cx-1", ParentThreadID: "p"}},
		{"codex 但无 CodexThreadID", &store.Thread{ID: "x2", AgentID: "codex", ParentThreadID: "p"}},
		{"codex 但无父链(非子代理)", &store.Thread{ID: "x3", AgentID: "codex", CodexThreadID: "cx-3"}},
	}
	for _, c := range cases {
		if s.maybePullCodexSubagentHistory(ctx, c.th) {
			t.Fatalf("%s 应返回 false", c.name)
		}
	}
}
