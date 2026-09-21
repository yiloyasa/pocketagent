package collabmcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeEngine 可注入的 DelegationEngine 假实现：真实接线（CreateThread +
// StartTurn）属后续任务，本包全部行为测试都由它驱动。
//
// 并发断言靠 per-agent active/maxAct 记账（信号量语义的直接观测），不靠
// 裸计时 —— 计时在慢机器上会抖。
type fakeEngine struct {
	delegate func(ctx context.Context, agentID, task string, opts DelegateOptions) (string, string, error)
	status   func(threadID string) (ThreadStatus, error)

	mu     sync.Mutex
	active map[string]int
	maxAct map[string]int
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{active: map[string]int{}, maxAct: map[string]int{}}
}

func (f *fakeEngine) Delegate(ctx context.Context, agentID, task string, opts DelegateOptions) (string, string, error) {
	f.mu.Lock()
	f.active[agentID]++
	if f.active[agentID] > f.maxAct[agentID] {
		f.maxAct[agentID] = f.active[agentID]
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.active[agentID]--
		f.mu.Unlock()
	}()
	return f.delegate(ctx, agentID, task, opts)
}

func (f *fakeEngine) StatusOf(threadID string) (ThreadStatus, error) {
	if f.status != nil {
		return f.status(threadID)
	}
	return ThreadStatus{ThreadID: threadID, AgentID: "fake", State: "idle"}, nil
}

func (f *fakeEngine) maxActive(agentID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxAct[agentID]
}

func TestDelegateSuccessSummaryBounded(t *testing.T) {
	eng := newFakeEngine()
	var gotOpts DelegateOptions
	eng.delegate = func(ctx context.Context, agentID, task string, opts DelegateOptions) (string, string, error) {
		if agentID != "codex" || task != "有界子任务" {
			t.Errorf("引擎收到的 agentID/task 不对: %q/%q", agentID, task)
		}
		gotOpts = opts
		return "th-child-1", strings.Repeat("汉", 100), nil
	}
	s := New(eng, "test")

	text, isErr := s.runDelegate(context.Background(), delegateArgs{
		AgentID: "codex", Task: "有界子任务",
		Workspace: "G:/ws", Model: "codex/gpt-x", Effort: "high",
	}, nil)
	if isErr {
		t.Fatalf("成功路径不应是错误: %s", text)
	}
	if !strings.Contains(text, "th-child-1") || !strings.Contains(text, strings.Repeat("汉", 100)) {
		t.Fatalf("终稿摘要未完整返回: %s", text)
	}
	if strings.Contains(text, "已截断") {
		t.Fatalf("未超限不应标注截断: %s", text)
	}
	// 可选参数必须原样透传给引擎，工具层不解释、不补默认值。
	if gotOpts.Workspace != "G:/ws" || gotOpts.Model != "codex/gpt-x" || gotOpts.Effort != "high" {
		t.Fatalf("DelegateOptions 透传不符: %+v", gotOpts)
	}
}

func TestDelegateSummaryTruncatedAt8K(t *testing.T) {
	eng := newFakeEngine()
	eng.delegate = func(ctx context.Context, agentID, task string, opts DelegateOptions) (string, string, error) {
		return "th-big", strings.Repeat("汉", maxSummaryRunes+500), nil
	}
	s := New(eng, "test")

	text, isErr := s.runDelegate(context.Background(), delegateArgs{AgentID: "codex", Task: "t"}, nil)
	if isErr {
		t.Fatalf("截断不是错误: %s", text)
	}
	if !strings.Contains(text, "已截断") {
		t.Fatalf("超限摘要必须标注截断: %s", text)
	}
	// 有界：整体（含标注）不超过 上限+标注 的合理余量。
	if n := len([]rune(text)); n > maxSummaryRunes+120 {
		t.Fatalf("摘要未真正截断: %d 字符", n)
	}
	if !strings.Contains(text, strings.Repeat("汉", 64)) {
		t.Fatalf("截断后应保留前缀内容: %s", text[:80])
	}
}

func TestDelegateMissingArgsIsToolError(t *testing.T) {
	s := New(newFakeEngine(), "test")
	for _, in := range []delegateArgs{
		{AgentID: "", Task: "t"},
		{AgentID: "codex", Task: " "},
	} {
		text, isErr := s.runDelegate(context.Background(), in, nil)
		if !isErr {
			t.Fatalf("空参应报工具错误: %+v -> %s", in, text)
		}
	}
}

func TestDelegateEngineErrorPassthroughVerbatim(t *testing.T) {
	// 准则 6：错误如实透传 —— 文本一字不改出现在工具结果里，daemon 不代拟。
	want := "电脑上没有可用的 ghost agent(未安装或未配置)。当前可用: codex, echo"
	eng := newFakeEngine()
	eng.delegate = func(ctx context.Context, agentID, task string, opts DelegateOptions) (string, string, error) {
		return "", "", errors.New(want)
	}
	s := New(eng, "test")

	text, isErr := s.runDelegate(context.Background(), delegateArgs{AgentID: "ghost", Task: "t"}, nil)
	if !isErr {
		t.Fatalf("引擎报错必须如实回传为工具错误: %s", text)
	}
	if text != "委派失败（agent=ghost）："+want {
		t.Fatalf("错误文本被改写（应逐字透传）:\n got=%s\nwant=委派失败（agent=ghost）：%s", text, want)
	}
	if strings.Contains(text, "完成") {
		t.Fatalf("失败绝不能伪装成完成: %s", text)
	}
}

func TestDelegateSameAgentSerializes(t *testing.T) {
	// 准则 7：同 agent(=同 provider)串行是事实约束。
	eng := newFakeEngine()
	eng.delegate = func(ctx context.Context, agentID, task string, opts DelegateOptions) (string, string, error) {
		time.Sleep(60 * time.Millisecond)
		return "th-" + agentID, "ok " + task, nil
	}
	s := New(eng, "test")

	const n = 3
	var wg sync.WaitGroup
	texts := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tx, isErr := s.runDelegate(context.Background(), delegateArgs{AgentID: "codex", Task: fmt.Sprintf("任务%d", i)}, nil)
			if isErr {
				t.Errorf("并发委派不应失败: %s", tx)
			}
			texts[i] = tx
		}(i)
	}
	wg.Wait()

	if got := eng.maxActive("codex"); got != 1 {
		t.Fatalf("同 agent 委派出现并行（maxActive=%d），违反串行事实约束", got)
	}
	for i, tx := range texts {
		if !strings.Contains(tx, "ok 任务") || strings.Contains(tx, "失败") {
			t.Fatalf("第 %d 个委派结果异常: %s", i, tx)
		}
	}
}

func TestDelegateCrossAgentParallel(t *testing.T) {
	// 跨 agent 必须并行：两把独立锁。证明方式不用计时 —— 用栅栏：
	// 两个引擎调用都先报"已进入"再等栅栏；若串行化，第二个永远进不来，
	// 收不满两个进入信号就会超时失败。
	eng := newFakeEngine()
	gate := make(chan struct{})
	entered := make(chan string, 4)
	eng.delegate = func(ctx context.Context, agentID, task string, opts DelegateOptions) (string, string, error) {
		entered <- agentID
		<-gate
		return "th-" + agentID, "ok", nil
	}
	s := New(eng, "test")

	results := make(chan string, 2)
	for _, id := range []string{"codex", "zcode"} {
		go func(id string) {
			tx, isErr := s.runDelegate(context.Background(), delegateArgs{AgentID: id, Task: "t"}, nil)
			if isErr {
				t.Errorf("跨 agent 并行委派不应失败: %s", tx)
			}
			results <- tx
		}(id)
	}

	for got := 0; got < 2; got++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("两个 agent 未能并行进入引擎（被串行化）")
		}
	}
	close(gate)
	for got := 0; got < 2; got++ {
		select {
		case tx := <-results:
			if !strings.Contains(tx, "子任务完成") {
				t.Fatalf("结果异常: %s", tx)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("并行委派未返回")
		}
	}
	if a, b := eng.maxActive("codex"), eng.maxActive("zcode"); a != 1 || b != 1 {
		t.Fatalf("各自 agent 内部应串行: codex=%d zcode=%d", a, b)
	}
}

func TestDelegateQueuedProgressOnlyWhenContended(t *testing.T) {
	// 两级进度的第一级：真排队才发"排队中"（准则 7，不伪造并行）。
	eng := newFakeEngine()
	hold := make(chan struct{})
	entered := make(chan struct{}, 1)
	eng.delegate = func(ctx context.Context, agentID, task string, opts DelegateOptions) (string, string, error) {
		if task == "独跑" {
			// 无争用路径：立即完成（"没排队就不发 queued"是主断言之一）。
			return "th-free", "ok 独跑", nil
		}
		entered <- struct{}{}
		<-hold
		return "th-1", "ok " + task, nil
	}
	s := New(eng, "test")

	// 无争用时：不得谎报排队。
	var freeStages []string
	tx, isErr := s.runDelegate(context.Background(), delegateArgs{AgentID: "pi", Task: "独跑"}, func(stage, tid, msg string) {
		freeStages = append(freeStages, stage)
	})
	if isErr || len(freeStages) != 0 {
		t.Fatalf("无争用不应发排队进度: isErr=%v stages=%v tx=%s", isErr, freeStages, tx)
	}

	// 有争用时：第二个委派先收到 queued，再拿到执行权完成。次序由回调
	// 驱动：queued 进度发出（说明已真排队）→ 才放行第一个 —— 不靠 sleep，
	// 也不会互相等死。
	done1 := make(chan string, 1)
	go func() {
		tx, _ := s.runDelegate(context.Background(), delegateArgs{AgentID: "pi", Task: "第一个"}, nil)
		done1 <- tx
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("第一个委派未进入引擎")
	}

	var (
		mu         sync.Mutex
		queuedMsgs []string
		once       sync.Once
	)
	tx2, isErr := s.runDelegate(context.Background(), delegateArgs{AgentID: "pi", Task: "第二个"}, func(stage, tid, msg string) {
		if stage == StageQueued {
			mu.Lock()
			queuedMsgs = append(queuedMsgs, msg)
			mu.Unlock()
			once.Do(func() { close(hold) })
		}
	})
	<-done1
	if isErr {
		t.Fatalf("排队后应正常完成: %s", tx2)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(queuedMsgs) != 1 || !strings.Contains(queuedMsgs[0], "排队中") {
		t.Fatalf("争用时必须发一级排队进度: %v", queuedMsgs)
	}
}

func TestDelegateCancelledWhileQueued(t *testing.T) {
	eng := newFakeEngine()
	hold := make(chan struct{})
	entered := make(chan struct{}, 1)
	eng.delegate = func(ctx context.Context, agentID, task string, opts DelegateOptions) (string, string, error) {
		entered <- struct{}{}
		<-hold
		return "th-1", "ok", nil
	}
	s := New(eng, "test")

	go func() {
		s.runDelegate(context.Background(), delegateArgs{AgentID: "pi", Task: "占位"}, nil)
	}()
	<-entered

	ctx, cancel := context.WithCancel(context.Background())
	queued := make(chan struct{}, 1)
	done := make(chan string, 1)
	go func() {
		tx, _ := s.runDelegate(ctx, delegateArgs{AgentID: "pi", Task: "等待者"}, func(stage, tid, msg string) {
			if stage == StageQueued {
				queued <- struct{}{}
			}
		})
		done <- tx
	}()
	<-queued // 已在排队
	cancel() // coordinator 撤了
	select {
	case tx := <-done:
		if !strings.Contains(tx, "已取消") {
			t.Fatalf("排队中取消应如实报取消: %s", tx)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("取消后未返回")
	}
	close(hold)
}

func TestAgentStatusSingleAndUnknown(t *testing.T) {
	eng := newFakeEngine()
	eng.status = func(threadID string) (ThreadStatus, error) {
		if threadID == "th-9" {
			return ThreadStatus{ThreadID: "th-9", AgentID: "dsh", State: "waiting_approval", Detail: "等待用户审批"}, nil
		}
		return ThreadStatus{}, fmt.Errorf("没有找到线程 %s", threadID)
	}
	s := New(eng, "test")

	text, isErr := s.runStatus(statusArgs{ThreadID: "th-9"})
	if isErr || !strings.Contains(text, "state=waiting_approval") ||
		!strings.Contains(text, "agent=dsh") || !strings.Contains(text, "等待用户审批") {
		t.Fatalf("状态查询结果不符: isErr=%v %s", isErr, text)
	}

	text, isErr = s.runStatus(statusArgs{ThreadID: "th-nope"})
	if !isErr || !strings.Contains(text, "没有找到线程 th-nope") {
		t.Fatalf("未知线程必须如实报错，不得编造状态: isErr=%v %s", isErr, text)
	}
}

func TestAgentStatusList(t *testing.T) {
	eng := newFakeEngine()
	eng.status = func(threadID string) (ThreadStatus, error) {
		return ThreadStatus{}, fmt.Errorf("线程 %s 已随 daemon 重启丢失", threadID)
	}
	s := New(eng, "test")

	text, isErr := s.runStatus(statusArgs{})
	if isErr || !strings.Contains(text, "尚无委派记录") {
		t.Fatalf("空记录应给人话提示: isErr=%v %s", isErr, text)
	}

	s.record("th-1", "codex", strings.Repeat("长", 200))
	text, isErr = s.runStatus(statusArgs{})
	if isErr || !strings.Contains(text, "th-1") || !strings.Contains(text, "agent=codex") {
		t.Fatalf("记录列表缺失: isErr=%v %s", isErr, text)
	}
	// 记录里的 task 预览要截断；查不到状态要如实标注（重启丢失），不编造。
	if !strings.Contains(text, "已截断") || !strings.Contains(text, "已随 daemon 重启丢失") {
		t.Fatalf("列表应截断 task 且如实标注状态不可得: %s", text)
	}
}

// TestStdioEndToEnd SDK 服务器对 stdio 的端到端最小自测：stdio 传输 =
// 换行分隔 JSON 过字节流（SDK 的 StdioTransport 对 os.Stdin/os.Stdout 与
// IOTransport 对管道走的是同一套 ioConn 帧），测试用 io.Pipe 顶替进程
// stdio，起 goroutine 跑服务器端，SDK 客户端模拟接入方做
// tools/list + tools/call + progress 通知。
func TestStdioEndToEnd(t *testing.T) {
	eng := newFakeEngine()
	eng.delegate = func(ctx context.Context, agentID, task string, opts DelegateOptions) (string, string, error) {
		if agentID != "codex" {
			// 模拟 server 侧真实实现：agent_id 必须在注册表里，不在就人话报错。
			return "", "", fmt.Errorf("电脑上没有可用的 %s agent(未安装或未配置)。当前可用: codex, echo", agentID)
		}
		child := "th-e2e-1"
		if opts.Progress != nil {
			opts.Progress(StageChildCreated, child, "已创建子线程 "+child)
		}
		return child, "子任务终稿：全部测试通过", nil
	}
	eng.status = func(threadID string) (ThreadStatus, error) {
		if threadID == "th-e2e-1" {
			return ThreadStatus{ThreadID: threadID, AgentID: "codex", State: "idle", Detail: "已完成"}, nil
		}
		return ThreadStatus{}, fmt.Errorf("没有找到线程 %s", threadID)
	}
	srv := New(eng, "test")

	progressCh := make(chan *mcp.ProgressNotificationParams, 8)
	toServerR, toServerW := io.Pipe() // 客户端 → 服务器
	toClientR, toClientW := io.Pipe() // 服务器 → 客户端

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- srv.MCPServer().Run(ctx, &mcp.IOTransport{Reader: toServerR, Writer: toClientW})
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "collab-e2e-client", Version: "0"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
			select {
			case progressCh <- r.Params:
			default:
			}
		},
	})
	cs, err := client.Connect(ctx, &mcp.IOTransport{Reader: toClientR, Writer: toServerW}, nil)
	if err != nil {
		t.Fatalf("客户端连接失败: %v", err)
	}
	defer cs.Close()

	// ---- tools/list ----
	list, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("tools/list 失败: %v", err)
	}
	names := map[string]string{}
	for _, tl := range list.Tools {
		names[tl.Name] = tl.Description
	}
	desc, ok := names["agent_delegate"]
	if !ok {
		t.Fatalf("缺 agent_delegate: %v", names)
	}
	if !strings.Contains(desc, "已注册") {
		t.Fatalf("agent_delegate 描述必须写清 agent_id 取已注册 agent 的 id: %s", desc)
	}
	if _, ok := names["agent_status"]; !ok {
		t.Fatalf("缺 agent_status: %v", names)
	}

	// ---- tools/call 成功路径 ----
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "agent_delegate",
		Arguments: map[string]any{"agent_id": "codex", "task": "跑一遍回归"},
	})
	if err != nil {
		t.Fatalf("tools/call 协议错误: %v", err)
	}
	if res.IsError {
		t.Fatalf("成功路径不应 IsError: %s", textOf(t, res))
	}
	if tx := textOf(t, res); !strings.Contains(tx, "th-e2e-1") || !strings.Contains(tx, "子任务终稿：全部测试通过") {
		t.Fatalf("终稿文本不符: %s", tx)
	}

	// ---- tools/call 未知 agent：错误文本如实回传（IsError=true，非协议错）----
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "agent_delegate",
		Arguments: map[string]any{"agent_id": "ghost", "task": "x"},
	})
	if err != nil {
		t.Fatalf("工具级错误不应表现为协议错误: %v", err)
	}
	if tx := textOf(t, res); !res.IsError || !strings.Contains(tx, "电脑上没有可用的 ghost") {
		t.Fatalf("未知 agent 应 IsError + 错误文本: isErr=%v %s", res.IsError, tx)
	}

	// ---- agent_status：单查 / 未知 / 列表 ----
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "agent_status",
		Arguments: map[string]any{"thread_id": "th-e2e-1"},
	})
	if err != nil || res.IsError {
		t.Fatalf("agent_status 单查失败: %v %s", err, textOf(t, res))
	}
	if tx := textOf(t, res); !strings.Contains(tx, "state=idle") || !strings.Contains(tx, "已完成") {
		t.Fatalf("状态文本不符: %s", tx)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "agent_status",
		Arguments: map[string]any{"thread_id": "th-nope"},
	})
	if err != nil || !res.IsError {
		t.Fatalf("未知线程应工具级报错: %v isErr=%v", err, res.IsError)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "agent_status"})
	if err != nil || res.IsError {
		t.Fatalf("agent_status 列表失败: %v %s", err, textOf(t, res))
	}
	if tx := textOf(t, res); !strings.Contains(tx, "th-e2e-1") {
		t.Fatalf("列表应含刚完成的委派记录: %s", tx)
	}

	// ---- progress：带 token 的委派应收到 child_created 进度通知 ----
	p := &mcp.CallToolParams{
		Name:      "agent_delegate",
		Arguments: map[string]any{"agent_id": "codex", "task": "再跑一遍"},
	}
	p.SetProgressToken("tok-e2e")
	if _, err := cs.CallTool(ctx, p); err != nil {
		t.Fatalf("带 token 的委派失败: %v", err)
	}
	var got *mcp.ProgressNotificationParams
	deadline := time.Now().Add(5 * time.Second)
	for got == nil && time.Now().Before(deadline) {
		select {
		case pn := <-progressCh:
			got = pn
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if got == nil {
		t.Fatal("未收到 progress 通知")
	}
	if !strings.Contains(got.Message, "已创建子线程") {
		t.Fatalf("进度通知文本不符: %+v", got)
	}

	// ---- 收尾：ctx 取消 → 服务器退出 ----
	cancel()
	select {
	case <-serverDone:
	case <-time.After(10 * time.Second):
		t.Fatal("服务器未随 ctx 退出")
	}
}

func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		return ""
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] 不是 TextContent: %T", res.Content[0])
	}
	return tc.Text
}
