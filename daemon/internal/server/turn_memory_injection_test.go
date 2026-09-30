// turn_memory_injection_test.go —— P2「四层上下文注入」的接线回归
// （memory 分层设计稿 §4 P2/D4；thread.go 轮次心脏 × turn_memory_context.go 组装
// × openai_compat.go 消费,三段接起来的那根线）。
//
// 【要钉什么】
//  1. 注入钉：daemon 家第二轮起,openai_compat 实际发出的 /chat/completions 里
//     必须带有第一轮的记忆上下文前缀（system 补充段,且在首条 user 消息之前）——
//     只测 buildTurnMemoryContext 返回值挡不住"字段填了但 provider 不读"；
//  2. 预算钉：前缀长度以 effectiveLimits 的 DiskConversationBytes 为硬顶,
//     超限截断并在尾部如实标注"[上下文摘要已截断]"（谎称完整比超限更糟）；
//  3. 空发钉：空 memory 时前缀为空串,请求里不出现记忆前缀段（不发空 system）；
//     非 daemon 家（hybrid 等）一律不注入。
//
// 【怎么 mock openai_compat】包内此前没有 OpenAICompatible.Run 的先例,这里用
// net/http/httptest 起一个本地 /chat/completions 假端点:捕获请求体、回一段最小
// SSE 流（一条正文增量 + stop 收尾）,让真 provider 走完整轮次路径 —— 与
// zcodeapp/accounts_test.go 的假端点同款取舍。
package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// oaCapture 捕获 openai_compat 发来的请求体,并回一段最小 SSE 应答。
type oaCapture struct {
	mu     sync.Mutex
	bodies []string
}

func (c *oaCapture) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.bodies = append(c.bodies, string(body))
		c.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"好的\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}
}

// snapshot 按角色取请求消息里的 content 字符串（仅适配本文件的最小形状）。
type oaReqSnapshot struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func (s oaReqSnapshot) systemContents() []string {
	var out []string
	for _, m := range s.Messages {
		if m.Role == "system" {
			out = append(out, m.Content)
		}
	}
	return out
}

// firstUserIndex 首条 user 消息的下标;没有返回 -1。
func (s oaReqSnapshot) firstUserIndex() int {
	for i, m := range s.Messages {
		if m.Role == "user" {
			return i
		}
	}
	return -1
}

// newInjectionTestServer 造一个"真 openai_compat + 假端点 + 文件后端"的测试 server,
// 并落一条归属该 provider 的线程。
func newInjectionTestServer(t *testing.T, h http.Handler) (*Server, *store.Thread) {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	p := agent.NewOpenAICompatible("oatest", "注入测试兼容", ts.URL, "test-key",
		[]agent.ModelSpec{{ID: "m1", DisplayName: "测试模型"}})
	srv := New(Config{
		Store:          fs,
		Providers:      map[string]agent.Provider{"oatest": p},
		Workspace:      t.TempDir(),
		RuntimeBackend: runtime.NewFileBackend(t.TempDir()),
	})
	// 跑过轮次的 server 会惰性启动闲时整理循环（P3）,测试收尾一并停掉。
	t.Cleanup(func() { _ = srv.events.Close(); srv.stopIdleMemorySweep() })
	th := &store.Thread{ID: "th-inject", AgentID: "oatest", Title: "注入回归", Workspace: t.TempDir()}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	return srv, th
}

// runInjectTurn 用 fakeTurnStream 跑一轮指定文本的输入（runSimpleTurn 的带参版）。
func runInjectTurn(t *testing.T, srv *Server, threadID, text string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &fakeTurnStream{
		ctx: ctx,
		in: []*pb.TurnClientMessage{{Msg: &pb.TurnClientMessage_Input{Input: &pb.TurnInput{
			ThreadId: threadID,
			Blocks: []*pb.ContentBlock{
				{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}}},
			},
		}}}},
	}
	if err := srv.StartTurn(stream); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
}

// TestMemoryPrefixReachesOpenAICompatRequest：注入钉 + 空发钉（端到端）。
// 第一轮（记忆为空）请求里不得有记忆前缀段;第二轮请求必须带上第一轮的
// user/assistant 原文,作为 system 补充段插在首条 user 消息之前,且本轮输入
// 不在前缀里重复出现。
func TestMemoryPrefixReachesOpenAICompatRequest(t *testing.T) {
	captured := &oaCapture{}
	srv, th := newInjectionTestServer(t, captured.handler())

	runInjectTurn(t, srv, th.ID, "第一轮的问题")
	runInjectTurn(t, srv, th.ID, "第二轮的问题")

	captured.mu.Lock()
	defer captured.mu.Unlock()
	if len(captured.bodies) != 2 {
		t.Fatalf("两轮应各发一次 /chat/completions,实得 %d 次", len(captured.bodies))
	}
	var first, second oaReqSnapshot
	if err := json.Unmarshal([]byte(captured.bodies[0]), &first); err != nil {
		t.Fatalf("解析第一轮请求失败: %v", err)
	}
	if err := json.Unmarshal([]byte(captured.bodies[1]), &second); err != nil {
		t.Fatalf("解析第二轮请求失败: %v", err)
	}

	// ③ 空 memory:第一轮只有基座人设一条 system,无记忆前缀段。
	sysFirst := first.systemContents()
	if len(sysFirst) != 1 {
		t.Fatalf("空记忆第一轮应只有 1 条 system,实得 %d 条", len(sysFirst))
	}
	if strings.Contains(captured.bodies[0], "记忆上下文") {
		t.Fatalf("空记忆不应发记忆前缀段,实得 %s", captured.bodies[0])
	}

	// ① 第二轮:基座 system + 记忆前缀 system,共两条;前缀含第一轮原文。
	sysSecond := second.systemContents()
	if len(sysSecond) != 2 {
		t.Fatalf("第二轮应有基座+前缀两条 system,实得 %d 条", len(sysSecond))
	}
	base, prefix := sysSecond[0], sysSecond[1]
	if strings.Contains(base, "第一轮的问题") {
		t.Fatalf("基座 system 不该携带会话内容")
	}
	for _, want := range []string{"[会话元数据]", "agent=oatest", "user: 第一轮的问题", "assistant: 好的"} {
		if !strings.Contains(prefix, want) {
			t.Fatalf("记忆前缀段应含 %q,实得 %q", want, prefix)
		}
	}
	// 本轮输入走 InputBlocks,前缀里不得重复出现同一句。
	if strings.Contains(prefix, "第二轮的问题") {
		t.Fatalf("前缀不应重复携带本轮输入,实得 %q", prefix)
	}
	// 前缀段必须插在首条 user 消息之前(system 补充段的定案位置)。
	prefixAt, userAt := -1, second.firstUserIndex()
	for i, m := range second.Messages {
		if m.Role == "system" && strings.Contains(m.Content, "记忆上下文") {
			prefixAt = i
			break
		}
	}
	if prefixAt < 0 || userAt < 0 || prefixAt > userAt {
		t.Fatalf("记忆前缀段应位于首条 user 消息之前,prefixAt=%d userAt=%d", prefixAt, userAt)
	}
}

// newPrefixUnitTestServer 造一个只用于直接调 buildTurnMemoryContext 的 server;
// diskConversation 非 nil 时以它覆盖全局 DiskConversationBytes 预算。
func newPrefixUnitTestServer(t *testing.T, diskConversation *int64) *Server {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{
		Store:          fs,
		Providers:      map[string]agent.Provider{},
		Workspace:      t.TempDir(),
		RuntimeBackend: runtime.NewFileBackend(t.TempDir()),
		BudgetPolicy:   runtime.BudgetPolicy{Global: runtime.Limits{DiskConversationBytes: diskConversation}},
	})
	t.Cleanup(func() { _ = srv.events.Close(); srv.stopIdleMemorySweep() })
	return srv
}

// seedConversationMemory 往某家某会话的 conversation 槽种 n 条可辨识的原文。
func seedConversationMemory(t *testing.T, srv *Server, agentID, threadID string, n int) {
	t.Helper()
	key := runtime.MemoryKey{AgentID: agentID, ThreadID: threadID, Kind: runtime.MemoryConversation}
	for i := 0; i < n; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		payload := "第" + string(rune('0'+i)) + "条记忆内容" + strings.Repeat("x", 60)
		if _, err := srv.runtimeBackend.Memory().Append(context.Background(), key, runtime.MemoryItem{
			Kind: runtime.MemoryConversation, Role: role, Payload: []byte(payload),
		}); err != nil {
			t.Fatalf("种记忆失败: %v", err)
		}
	}
}

// TestMemoryPrefixBudgetTruncation：预算钉。
// 额度内不截断、不带标注;额度收紧后前缀长度不超过 DiskConversationBytes,
// 且尾部带"[上下文摘要已截断]"如实标注。
func TestMemoryPrefixBudgetTruncation(t *testing.T) {
	i64p := func(v int64) *int64 { return &v }
	// 160 字节:身份行(随临时目录路径浮动,百字节级)加保底消息必然顶破,
	// 稳定触发出口处的兜底截断;额度再小就只剩标注本身,钉不住层序。
	const budget = int64(160)

	loose := newPrefixUnitTestServer(t, nil) // 零值策略 = 内置基线(32MB)
	tight := newPrefixUnitTestServer(t, i64p(budget))

	p := &memShapeProvider{id: "dmp", model: runtime.MemoryDaemon, reply: "x"}
	thLoose := &store.Thread{ID: "th-loose", Workspace: t.TempDir()}
	thTight := &store.Thread{ID: "th-tight", Workspace: t.TempDir()}
	seedConversationMemory(t, loose, p.ID(), thLoose.ID, 10)
	seedConversationMemory(t, tight, p.ID(), thTight.ID, 10)

	gotLoose := loose.buildTurnMemoryContext(p, thLoose)
	gotTight := tight.buildTurnMemoryContext(p, thTight)

	// 额度内:记忆进得来,不截断、无标注。
	if gotLoose == "" || !strings.Contains(gotLoose, "第9条记忆内容") {
		t.Fatalf("额度内前缀应携带最近记忆,实得 %q", gotLoose)
	}
	if strings.Contains(gotLoose, turnContextTruncatedMark) {
		t.Fatalf("额度内不应出现截断标注,实得 %q", gotLoose)
	}
	// 超预算:硬顶生效 + 尾部如实标注。
	if int64(len(gotTight)) > budget {
		t.Fatalf("前缀应截到 %d 字节内,实得 %d", budget, len(gotTight))
	}
	if !strings.HasSuffix(gotTight, turnContextTruncatedMark) {
		t.Fatalf("截断后尾部应带标注,实得 %q", gotTight)
	}
	if !strings.HasPrefix(gotTight, "[会话元数据]") {
		t.Fatalf("截断不应破坏层序(元数据仍在最前),实得 %q", gotTight)
	}
	if len(gotTight) >= len(gotLoose) {
		t.Fatalf("收紧预算应显著缩短前缀,tight=%d loose=%d", len(gotTight), len(gotLoose))
	}
}

// TestMemoryPrefixEmptyAndFamilyGated：空发钉(直调侧)。
// 空 memory → 空串不发;hybrid 家(非 daemon 家)即使槽里有东西也不注入。
func TestMemoryPrefixEmptyAndFamilyGated(t *testing.T) {
	srv := newPrefixUnitTestServer(t, nil)

	daemon := &memShapeProvider{id: "dme", model: runtime.MemoryDaemon, reply: "x"}
	th := &store.Thread{ID: "th-empty", Workspace: t.TempDir()}
	if got := srv.buildTurnMemoryContext(daemon, th); got != "" {
		t.Fatalf("空 memory 应返回空串(不发),实得 %q", got)
	}

	// hybrid 家:daemon 侧槽里只有索引账目,拼进提示词是让模型读账——不注入。
	hybrid := &memShapeProvider{id: "hm", model: runtime.MemoryHybrid, reply: "x"}
	seedConversationMemory(t, srv, hybrid.ID(), th.ID, 3)
	if got := srv.buildTurnMemoryContext(hybrid, th); got != "" {
		t.Fatalf("非 daemon 家不注入,实得 %q", got)
	}
}
