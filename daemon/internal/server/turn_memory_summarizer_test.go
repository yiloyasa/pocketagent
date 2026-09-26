// turn_memory_summarizer_test.go —— P2「Summarizer 经 server.Config 注入」的钉子
// （memory 分层设计稿 §4 P2：server 层注入实现，缺省 nil = P1 行为零分叉）。
//
// 【要钉什么】注入要真正走到 Compact：daemon 家跑一轮触发轮末收敛后，
//  1. 注入成功的 fake → summary 槽摘要行带语义正文 + summary-v1 审计首行；
//  2. 注入报错的 fake → 摘要行回退结构摘要，且不伪造审计首行。
//
// 走真实轮次而不是直呼 Compact，是因为注入发生在 New 的装配期——只有真实链路
// 才能证明"Config 字段 → Backend → Compact"这三跳都接上了。
package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// summarizerCall 记录一次 Summarize 调用的槽标识与批大小（验证注入点传参）。
type summarizerCall struct {
	scope    string
	items    int
	sequence []int64
}

// scriptedSummarizer 可编程替身：text/err 决定行为，calls 记录调用。
type scriptedSummarizer struct {
	text  string
	err   error
	calls []summarizerCall
}

func (s *scriptedSummarizer) Summarize(_ context.Context, conversationScope string, items []runtime.MemoryItem) (string, error) {
	c := summarizerCall{scope: conversationScope, items: len(items)}
	for i := range items {
		c.sequence = append(c.sequence, items[i].Sequence)
	}
	s.calls = append(s.calls, c)
	if s.err != nil {
		return "", s.err
	}
	return s.text, nil
}

// newSummarizerServer 同 newTurnMemoryServerWithPolicy，但注入语义摘要器。
func newSummarizerServer(t *testing.T, p agent.Provider, sum runtime.Summarizer) (*Server, *store.Thread) {
	t.Helper()
	dir := t.TempDir()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{
		Store:            fs,
		Providers:        map[string]agent.Provider{p.ID(): p},
		RuntimeBackend:   runtime.NewFileBackend(dir),
		MemorySummarizer: sum,
	})
	t.Cleanup(func() { _ = srv.events.Close() })
	thID := memoryTestThreadID(p)
	if err := fs.Create(&store.Thread{ID: thID, Title: "语义摘要注入", Workspace: t.TempDir()}); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	th, ok := fs.Get(thID)
	if !ok {
		t.Fatal("线程丢了")
	}
	return srv, th
}

// mustSummaryRows 取 summary 槽全部条目（轮末收敛摘要行的落点）。
func mustSummaryRows(t *testing.T, srv *Server, agentID, threadID string) []runtime.MemoryItem {
	t.Helper()
	snap := mustLoadMemory(t, srv, runtime.MemoryKey{AgentID: agentID, ThreadID: threadID, Kind: runtime.MemorySummary})
	if len(snap.Items) == 0 {
		t.Fatal("前置条件不成立：summary 槽没有摘要行（轮末收敛没跑？）")
	}
	return snap.Items
}

// TestServerInjectedSummarizerProducesSemanticDigest：钉 1——注入成功走语义摘要。
func TestServerInjectedSummarizerProducesSemanticDigest(t *testing.T) {
	p := &memShapeProvider{id: "sum1", model: runtime.MemoryDaemon, reply: "回答正文"}
	fake := &scriptedSummarizer{text: "用户在连续追问记忆分层的设计"}
	srv, th := newSummarizerServer(t, p, fake)
	key := runtime.MemoryKey{AgentID: p.ID(), ThreadID: th.ID, Kind: runtime.MemoryConversation}
	seedConversation(t, srv, key, 201) // 越阈值，逼轮末收敛真的发生

	stream := runSimpleTurn(t, srv, th.ID)
	assertTurnCompleted(t, stream)

	if len(fake.calls) == 0 {
		t.Fatal("Config 注入的 Summarizer 没被 Compact 调用（注入链断了）")
	}
	// 201 条垫床 + 本轮 2 条 = 203，KeepRecentItems=50 → 被压缩批 = seq 1..153。
	if got := fake.calls[0]; got.scope != key.String() || got.items != 153 ||
		got.sequence[0] != 1 || got.sequence[len(got.sequence)-1] != 153 {
		t.Fatalf("Summarize 应收到 conversation 槽与被压缩批 seq 1..153，实得 scope=%s items=%d 首=%v 末=%v",
			got.scope, got.items, got.sequence[:1], got.sequence[len(got.sequence)-1:])
	}
	rows := mustSummaryRows(t, srv, p.ID(), th.ID)
	last := rows[len(rows)-1]
	if !strings.Contains(string(last.Payload), "用户在连续追问记忆分层的设计") {
		t.Fatalf("摘要行应登记语义摘要正文，实得 %q", last.Payload)
	}
	if !strings.HasPrefix(string(last.Payload), "summary-v1 seq=") {
		t.Fatalf("语义摘要行必须带可审计首行，实得 %q", last.Payload)
	}
}

// TestServerSummarizerFailureFallsBackToStructural：钉 2——失败回退结构摘要，
// 且不给摘要行伪造 summary-v1 首行（审计首行 = "语义摘要真的生成过"的凭证）。
func TestServerSummarizerFailureFallsBackToStructural(t *testing.T) {
	p := &memShapeProvider{id: "sum2", model: runtime.MemoryDaemon, reply: "回答正文"}
	fake := &scriptedSummarizer{err: errors.New("模型通道不可用")}
	srv, th := newSummarizerServer(t, p, fake)
	key := runtime.MemoryKey{AgentID: p.ID(), ThreadID: th.ID, Kind: runtime.MemoryConversation}
	seedConversation(t, srv, key, 201)

	stream := runSimpleTurn(t, srv, th.ID)
	assertTurnCompleted(t, stream)

	rows := mustSummaryRows(t, srv, p.ID(), th.ID)
	last := rows[len(rows)-1]
	payload := string(last.Payload)
	if !strings.Contains(payload, "已压缩") {
		t.Fatalf("失败时应回退结构摘要，实得 %q", payload)
	}
	if strings.Contains(payload, "summary-v1") {
		t.Fatalf("结构摘要行不得伪造审计首行，实得 %q", payload)
	}
}
