// thread_history_zcode_test.go —— GetThreadHistory 对 zcode 导入线程的
// 惰性历史拉取三态(首次拉取落库 / 缓存复用 / 失败降级),以及
// codex(含旧库空 AgentID)/ pi 现有历史路径零改动的回归。
package server

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// fakeZcodeHistory 实现 agent.Provider + zcodeHistorySource 的假 zcode:
// 记录 HistoryFor 调用次数,按预置脚本返回历史或错误。
type fakeZcodeHistory struct {
	history []agent.HistoryMsg
	err     error
	calls   int
}

func (f *fakeZcodeHistory) ID() string          { return "zcode" }
func (f *fakeZcodeHistory) DisplayName() string { return "ZCode(测试)" }
func (f *fakeZcodeHistory) Models() []pb.ModelInfo {
	return nil
}

func (f *fakeZcodeHistory) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, nil
}

func (f *fakeZcodeHistory) HistoryFor(_ context.Context, _ string) ([]agent.HistoryMsg, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.history, nil
}

// newZcodeHistoryThread 建一条"刚导入、还没有任何落库消息"的 zcode 线程。
func newZcodeHistoryThread(t *testing.T, s *store.FileStore) *store.Thread {
	t.Helper()
	th := &store.Thread{
		ID: "th-zimp", Title: "导入的会话", Workspace: "w", ModelID: "zcode/default",
		AgentID:         "zcode",
		ExternalSession: store.ExternalSessionRef{SessionID: "sess_imported", Cwd: "w"},
	}
	if err := s.Create(th); err != nil {
		t.Fatalf("建导入线程失败: %v", err)
	}
	return th
}

// 三态之一 + 之二:首次打开拉取并落库,第二次走缓存不再在线。
func TestGetThreadHistoryZcodeImportedFirstPullThenCache(t *testing.T) {
	fake := &fakeZcodeHistory{history: []agent.HistoryMsg{
		{Role: "user", Text: "导入前的问题", CreatedAtUnixMs: 1000},
		{Role: "assistant", Text: "导入前的回答", CreatedAtUnixMs: 2000},
	}}
	st := mustStore(t)
	th := newZcodeHistoryThread(t, st)
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"zcode": fake}})

	out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID})
	if err != nil {
		t.Fatalf("首次拉取不应报错: %v", err)
	}
	if len(out.Messages) != 2 || fake.calls != 1 {
		t.Fatalf("首次应返回 2 条且在线拉 1 次: msgs=%d calls=%d", len(out.Messages), fake.calls)
	}
	if out.Messages[0].Role != "user" || out.Messages[0].CreatedAtUnixMs != 1000 {
		t.Fatalf("历史形状不对: %+v", out.Messages[0])
	}
	// 落库缓存:store 里现在应该真的有这两条(重启 daemon 也不丢)。
	cached, ok := st.Get(th.ID)
	if !ok || len(cached.Messages) != 2 {
		t.Fatalf("历史应已落库缓存: ok=%v msgs=%d", ok, len(cached.Messages))
	}
	// 第二次打开:纯本地,不再调 HistoryFor。
	out2, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID})
	if err != nil || len(out2.Messages) != 2 || fake.calls != 1 {
		t.Fatalf("第二次应走缓存: err=%v msgs=%d calls=%d", err, len(out2.Messages), fake.calls)
	}
	// Limit 依旧作用于(缓存后的)全量时间线。
	out3, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Limit: 1})
	if err != nil || len(out3.Messages) != 1 || out3.Messages[0].Role != "assistant" {
		t.Fatalf("limit=1 应只返回最新 1 条: err=%v msgs=%+v", err, out3.Messages)
	}
}

// 三态之三:provider 拉取失败(配额/进程起不来/会话已删)→ 降级返回已有内容。
func TestGetThreadHistoryZcodeImportedFailureDegrades(t *testing.T) {
	fake := &fakeZcodeHistory{err: context.DeadlineExceeded}
	st := mustStore(t)
	th := newZcodeHistoryThread(t, st)
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"zcode": fake}})

	out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID})
	if err != nil {
		t.Fatalf("失败必须降级而不是把错误抛给历史页: %v", err)
	}
	if len(out.Messages) != 0 {
		t.Fatalf("降级应返回已有内容(空),得到 %d 条", len(out.Messages))
	}
	if fake.calls != 1 {
		t.Fatalf("应在线尝试过 1 次: calls=%d", fake.calls)
	}
}

// 已有落库消息(首次缓存或导入后已聊过)的 zcode 线程不再在线拉。
func TestGetThreadHistoryZcodeWithLocalMessagesSkipsPull(t *testing.T) {
	fake := &fakeZcodeHistory{}
	st := mustStore(t)
	th := newZcodeHistoryThread(t, st)
	th.Messages = []store.StoredMessage{{Role: "user", Blocks: textBlocks("导入后的一轮"), CreatedAt: 5000}}
	if err := st.Update(th); err != nil {
		t.Fatalf("落库失败: %v", err)
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"zcode": fake}})

	out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID})
	if err != nil || len(out.Messages) != 1 || fake.calls != 0 {
		t.Fatalf("有本地消息不应在线拉: err=%v msgs=%d calls=%d", err, len(out.Messages), fake.calls)
	}
}

// codex(旧库空 AgentID / 显式 codex)/ pi 的历史路径零改动:不触发 zcode 拉取。
func TestGetThreadHistoryCodexAndPiUntouched(t *testing.T) {
	fake := &fakeZcodeHistory{}
	st := mustStore(t)
	legacy := &store.Thread{ID: "th-codex-old", Title: "旧库 codex", ModelID: "codex/gpt", AgentID: ""}
	legacy.Messages = []store.StoredMessage{
		{Role: "user", Blocks: textBlocks("问题"), CreatedAt: 1},
		{Role: "assistant", Blocks: textBlocks("回答"), CreatedAt: 2},
	}
	pi := &store.Thread{ID: "th-pi", Title: "pi 会话", ModelID: "pi/default", AgentID: "pi"}
	pi.Messages = []store.StoredMessage{{Role: "user", Blocks: textBlocks("pi 问题"), CreatedAt: 3}}
	for _, th := range []*store.Thread{legacy, pi} {
		if err := st.Create(th); err != nil {
			t.Fatalf("建线程失败: %v", err)
		}
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"zcode": fake}})

	for _, id := range []string{legacy.ID, pi.ID} {
		out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: id})
		if err != nil {
			t.Fatalf("%s 历史不应报错: %v", id, err)
		}
		if len(out.Messages) == 0 {
			t.Fatalf("%s 应原样返回落库消息", id)
		}
	}
	if fake.calls != 0 {
		t.Fatalf("codex/pi 路径不得触发 zcode 拉取: calls=%d", fake.calls)
	}
}

// textBlocks 单条文本块的最简构造(测试专用)。
func textBlocks(s string) []pb.ContentBlock {
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: s}}}}
}
