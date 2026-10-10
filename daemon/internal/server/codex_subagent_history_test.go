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
	"os"
	"path/filepath"
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
// 返回 false。这些分支在 provider 能力判定之前 —— 所以测试环境无需真 codex。
// 【F9 后口径】普通 codex 线程 0 消息不再是早退分支(要进自愈懒拉),
// 早退的是"普通线程但有本地消息"(有历史 = 不需要自愈)。
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
		{"普通 codex 线程但已有消息", &store.Thread{ID: "x3", AgentID: "codex", CodexThreadID: "cx-3",
			Messages: []store.StoredMessage{{Role: "user", Blocks: textBlocks("已有的一轮")}}}},
	}
	for _, c := range cases {
		if s.maybePullCodexSubagentHistory(ctx, c.th) {
			t.Fatalf("%s 应返回 false", c.name)
		}
	}
}

// ---------- F9 存量自愈:0 消息普通 codex 线程的懒拉(2026-10-09)----------

// fakeCodexReadFull 实现 agent.Provider + subagentReadFuller 的假 codex:
// 记录 ReadThreadFull 调用次数,按预置脚本返回快照或错误(不真调桌面 codex)。
type fakeCodexReadFull struct {
	snap  *agent.CodexThreadSnapshot
	err   error
	calls int
}

func (f *fakeCodexReadFull) ID() string          { return "codex" }
func (f *fakeCodexReadFull) DisplayName() string { return "Codex(测试)" }
func (f *fakeCodexReadFull) Models() []pb.ModelInfo {
	return nil
}

func (f *fakeCodexReadFull) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, nil
}

func (f *fakeCodexReadFull) ReadThreadFull(_ context.Context, _ string) (*agent.CodexThreadSnapshot, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.snap, nil
}

// healSnapshot 一份 thread/read 全量快照:一轮 user/assistant 对话。
func healSnapshot() *agent.CodexThreadSnapshot {
	return &agent.CodexThreadSnapshot{
		ID: "cx-heal",
		Turns: []agent.CodexThreadTurn{{
			ID: "turn-1",
			Items: []agent.CodexTurnItem{
				{Type: "userMessage", Text: "对接一下这个文件夹中的项目"},
				{Type: "agentMessage", Text: "好的,先看目录结构。"},
			},
		}},
	}
}

// newZeroMessageCodexThread 建一条"旧解析器落下的 0 消息普通 codex 线程",
// 并在注入的主目录里放一份 rollout 文件(否则 GetThreadHistory 的 stale 守卫
// 会按"电脑上的记录已找不到"先行报错,轮不到懒拉 —— 生产上能自愈的正是
// rollout 还在的这类线程)。
func newZeroMessageCodexThread(t *testing.T, s *store.FileStore) *store.Thread {
	t.Helper()
	home := t.TempDir()
	t.Setenv("USERPROFILE", home) // os.UserHomeDir:Windows 读 USERPROFILE、unix 读 HOME
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".codex", "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建 sessions 目录失败: %v", err)
	}
	rollout := filepath.Join(dir, "rollout-2026-10-04T11-26-19-cx-heal.jsonl")
	body := newFormatRolloutLine("2026-10-04T03:27:00.000Z", "user", `{"type":"input_text","text":"对接一下这个文件夹中的项目"}`) + "\n" +
		newFormatRolloutLine("2026-10-04T03:27:05.000Z", "assistant", `{"type":"output_text","text":"好的。"}`) + "\n"
	if err := os.WriteFile(rollout, []byte(body), 0o644); err != nil {
		t.Fatalf("写 rollout 失败: %v", err)
	}
	th := &store.Thread{
		ID: "th-heal", Title: "对接一下这个文件夹中的项目", Workspace: "w",
		ModelID: "codex/default", AgentID: "codex", CodexThreadID: "cx-heal",
	}
	if err := s.Create(th); err != nil {
		t.Fatalf("建 0 消息线程失败: %v", err)
	}
	return th
}

// newFormatRolloutLine 与 agent 包 fixture 同款的新格式 rollout 行(server 侧
// 只用于让 stale 守卫放行,不参与断言)。
func newFormatRolloutLine(ts, role, contentJSON string) string {
	return `{"timestamp":"` + ts + `","type":"response_item","payload":{"type":"message","id":"msg_x","role":"` +
		role + `","content":[` + contentJSON + `]}}`
}

// TestGetThreadHistoryCodexZeroMessageFirstPullHeals 核心治愈路径:0 消息普通
// codex 线程首次 GetThreadHistory 触发懒拉,拉回即落库,第二次走缓存不再在线。
func TestGetThreadHistoryCodexZeroMessageFirstPullHeals(t *testing.T) {
	fake := &fakeCodexReadFull{snap: healSnapshot()}
	st := mustStore(t)
	th := newZeroMessageCodexThread(t, st)
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"codex": fake}})

	out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID})
	if err != nil {
		t.Fatalf("首次打开不应报错: %v", err)
	}
	if len(out.Messages) != 2 || fake.calls != 1 {
		t.Fatalf("首次应回 2 条且懒拉 1 次: msgs=%d calls=%d", len(out.Messages), fake.calls)
	}
	if out.Messages[0].Role != "user" || out.Messages[1].Role != "assistant" {
		t.Fatalf("历史形状不对: %+v / %+v", out.Messages[0], out.Messages[1])
	}
	// 落库缓存:store 里现在应真有这两条(重启 daemon 也不丢)。
	cached, ok := st.Get(th.ID)
	if !ok || len(cached.Messages) != 2 {
		t.Fatalf("懒拉结果应已落库: ok=%v msgs=%d", ok, len(cached.Messages))
	}
	// 第二次打开:纯本地,不再调 ReadThreadFull。
	out2, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID})
	if err != nil || len(out2.Messages) != 2 || fake.calls != 1 {
		t.Fatalf("第二次应走缓存: err=%v msgs=%d calls=%d", err, len(out2.Messages), fake.calls)
	}
}

// TestGetThreadHistoryCodexZeroMessagePullFailureKeepsEmpty 失败降级:拉不到
// 不落库、不报错(下次打开再试),历史页不被打挂。
func TestGetThreadHistoryCodexZeroMessagePullFailureKeepsEmpty(t *testing.T) {
	fake := &fakeCodexReadFull{err: context.DeadlineExceeded}
	st := mustStore(t)
	th := newZeroMessageCodexThread(t, st)
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"codex": fake}})

	out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID})
	if err != nil {
		t.Fatalf("拉取失败必须降级而不是抛错: %v", err)
	}
	if len(out.Messages) != 0 || fake.calls != 1 {
		t.Fatalf("降级应返回已有内容(空)且尝试过 1 次: msgs=%d calls=%d", len(out.Messages), fake.calls)
	}
	if cached, _ := st.Get(th.ID); len(cached.Messages) != 0 {
		t.Fatalf("空结果不得缓存, got %d 条", len(cached.Messages))
	}
}

// TestGetThreadHistoryCodexWithMessagesSkipsPull 有本地消息的普通 codex 线程
// (导入后聊过/旧版正常落库)不触发懒拉;非 codex agent 亦不触发。
func TestGetThreadHistoryCodexWithMessagesSkipsPull(t *testing.T) {
	fake := &fakeCodexReadFull{snap: healSnapshot()}
	st := mustStore(t)
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	chatted := &store.Thread{ID: "th-cx-chat", Title: "聊过的 codex", ModelID: "codex/default",
		AgentID: "codex", CodexThreadID: "cx-chat"}
	chatted.Messages = []store.StoredMessage{{Role: "user", Blocks: textBlocks("已有的一轮"), CreatedAt: 1}}
	pi := &store.Thread{ID: "th-pi-heal", Title: "pi 会话", ModelID: "pi/default", AgentID: "pi"}
	pi.CodexThreadID = "" // pi 没有 codex id
	for _, th := range []*store.Thread{chatted, pi} {
		if err := st.Create(th); err != nil {
			t.Fatalf("建线程失败: %v", err)
		}
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"codex": fake}})

	for _, id := range []string{chatted.ID, pi.ID} {
		if _, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: id}); err != nil {
			t.Fatalf("%s 历史不应报错: %v", id, err)
		}
	}
	if fake.calls != 0 {
		t.Fatalf("有消息的 codex 线程与非 codex 线程都不得触发懒拉: calls=%d", fake.calls)
	}
}
