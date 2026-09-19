// codex_effect_test.go —— 七项设置"真实生效链路"的端到端单测(fake codex app-server)。
//
// 【测的是什么】手机改设置 → SetCodexSettings/ApplyPrefs → CodexProvider.Run 每轮
// 真实发给 codex 的 JSON-RPC 参数。用"假 codex 子进程"钉死,不依赖本机装有 codex:
//
//	审批策略/沙箱/模型/强度/思考摘要 → turn/start 参数(approvalPolicy/sandboxPolicy/
//	                                  model/effort/summary,见 codex.go 的 startParams);
//	回答详略/思考摘要(线程级)        → thread/start、thread/resume 的 config 覆盖
//	                                  (model_verbosity / model_reasoning_summary);
//	隐藏推理过程                      → Run() 的事件过滤:HideReasoning=true 时不向手机
//	                                  推 ThinkingDelta(codex 本版本无原生开关)。
//
// 【fake 怎么接进去】codexapp.Start() 尊重 CODEX_BIN 环境变量;这里把它指向
// **测试二进制自己**(TestHelperProcess 惯用法):子进程以 `app-server` 为第一个
// 参数被拉起时,TestMain 分流到 runFakeCodexAppServer,按 JSON-RPC 2.0 over stdio
// 应答,并把收到的每个请求原样记进 PA_FAKE_CODEX_LOG 文件,供父进程断言。
package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// TestMain 既是常规测试入口,也是 fake codex 子进程入口:
// codexapp.Start() 会以 `CODEX_BIN app-server` 拉起子进程,于是子进程的
// os.Args[1] == "app-server" —— 此时走 fake 协议循环,不跑任何测试。
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "app-server" {
		os.Exit(runFakeCodexAppServer())
	}
	os.Exit(m.Run())
}

// fakeReq 记进日志的一条 JSON-RPC 请求(method + 原始 params)。
type fakeReq struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// fakeCodexRequests 读回父进程断言所需的全部已记录请求。
func fakeCodexRequests(t *testing.T, logPath string) []fakeReq {
	t.Helper()
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读 fake codex 日志: %v", err)
	}
	var out []fakeReq
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var r fakeReq
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("日志行无法解析: %v(%s)", err, line)
		}
		out = append(out, r)
	}
	return out
}

// runFakeCodexAppServer 是 fake codex app-server 的主循环:
// stdin 读 JSON-RPC,stdout 写应答/通知,请求按行追加进 PA_FAKE_CODEX_LOG。
func runFakeCodexAppServer() int {
	logPath := os.Getenv("PA_FAKE_CODEX_LOG")
	if logPath == "" {
		return 2
	}
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 2
	}
	defer logf.Close()

	var (
		mu      sync.Mutex // 保护 logf 与 stdout 的逐行写
		counter int        // fake threadId 计数
	)
	writeLine := func(v interface{}) error {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		if _, err := logf.Write(append(raw, '\n')); err != nil {
			return err
		}
		_, err = os.Stdout.Write(append(raw, '\n'))
		return err
	}
	logOnly := func(method string, params json.RawMessage) {
		raw, _ := json.Marshal(fakeReq{Method: method, Params: params})
		mu.Lock()
		defer mu.Unlock()
		_, _ = logf.Write(append(raw, '\n'))
	}
	respond := func(id int64, result interface{}) {
		raw, _ := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0", "id": id, "result": result,
		})
		mu.Lock()
		defer mu.Unlock()
		_, _ = os.Stdout.Write(append(raw, '\n'))
	}
	notify := func(method string, params interface{}) {
		_ = writeLine(map[string]interface{}{
			"jsonrpc": "2.0", "method": method, "params": params,
		})
	}

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1024*1024), 1024*1024)
	for in.Scan() {
		var m struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(in.Bytes(), &m) != nil || m.Method == "" {
			continue
		}
		if m.ID == nil {
			// 通知(客户端 → 服务端):记下来即可
			logOnly(m.Method, m.Params)
			continue
		}
		id := *m.ID
		switch m.Method {
		case "initialize":
			respond(id, map[string]interface{}{})
		case "model/list":
			// 默认与旧行为一致(空对象应答);PA_FAKE_CODEX_MODELS 设置时按其
			// 原样 JSON 应答,供图片能力门控(codexSupportsImageInput)的降级单测用。
			logOnly(m.Method, m.Params)
			if models := os.Getenv("PA_FAKE_CODEX_MODELS"); models != "" {
				respond(id, json.RawMessage(models))
			} else {
				respond(id, map[string]interface{}{})
			}
		case "thread/start":
			// 记录后应答一个 threadId;cwd/config 覆盖都在 params 里,供父进程断言。
			logOnly(m.Method, m.Params)
			counter++
			respond(id, map[string]interface{}{"threadId": fmt.Sprintf("fake-thread-%d", counter)})
		case "thread/resume":
			logOnly(m.Method, m.Params)
			counter++
			respond(id, map[string]interface{}{"threadId": fmt.Sprintf("fake-thread-%d", counter)})
		case "turn/start":
			logOnly(m.Method, m.Params)
			respond(id, map[string]interface{}{})
			// 先应答再发事件:client.dispatch 按到达顺序处理,父进程的
			// NotifHandler 在 turn/start 发出前就已装好(codex.go),不会丢。
			var p struct {
				ThreadID string `json:"threadId"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if os.Getenv("PA_FAKE_CODEX_REASONING") == "1" {
				notify("item/reasoning/textDelta", map[string]string{
					"threadId": p.ThreadID, "delta": "思考片段一",
				})
				notify("item/reasoning/summaryTextDelta", map[string]string{
					"threadId": p.ThreadID, "delta": "思考片段二",
				})
			}
			notify("turn/completed", map[string]interface{}{
				"threadId": p.ThreadID,
				"turn": map[string]interface{}{
					"items": []map[string]interface{}{
						{"type": "agentMessage", "text": "fake-ok"},
					},
				},
			})
		default:
			logOnly(m.Method, m.Params)
			respond(id, map[string]interface{}{})
		}
	}
	return 0
}

// newFakeCodexProvider 造一个"codex"实际是本测试二进制的 provider,
// 并返回记录请求用的日志路径。
func newFakeCodexProvider(t *testing.T, reasoning bool) (*CodexProvider, string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "rpc.log")
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatalf("建日志文件: %v", err)
	}
	t.Setenv("PA_FAKE_CODEX_LOG", logPath)
	if reasoning {
		t.Setenv("PA_FAKE_CODEX_REASONING", "1")
	}
	// LocateCodex 认 CODEX_BIN:指向测试二进制自己(TestMain 顶部的分流配合)。
	t.Setenv("CODEX_BIN", os.Args[0])
	p := NewCodexProvider()
	// 收尾:杀掉 fake 子进程。子进程退出才会释放日志句柄(Windows 上句柄被占时
	// t.TempDir 的 RemoveAll 会失败),所以这里等到"日志能被删掉"再返回。
	// 本测试文件与 codex.go 同包,可直接触达未导出的 client 字段。
	t.Cleanup(func() {
		if p.client != nil {
			p.client.Close()
		}
		for i := 0; i < 100; i++ {
			if err := os.Remove(logPath); err == nil || os.IsNotExist(err) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
	return p, logPath
}

// runOneTurn 跑一轮并收集推给手机的全部消息。
func runOneTurn(t *testing.T, p *CodexProvider, thread *store.Thread, ws string) []*pb.TurnServerMessage {
	t.Helper()
	var mu sync.Mutex
	var got []*pb.TurnServerMessage
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, _, err := p.Run(ctx, agentTurnRequest(thread, ws), func(m *pb.TurnServerMessage) {
		// Sink 已改指针签名:直接收集指针,无 pb 值拷贝(内嵌 MessageState 含锁)。
		mu.Lock()
		got = append(got, m)
		mu.Unlock()
	}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return got
}

func agentTurnRequest(thread *store.Thread, ws string) TurnRequest {
	return TurnRequest{
		Thread:    thread,
		Workspace: ws,
		InputBlocks: []pb.ContentBlock{
			{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "你好"}}},
		},
	}
}

// findStartParams 取第 method 条请求的 params(JSON 对象)。
func findStartParams(t *testing.T, reqs []fakeReq, method string) map[string]interface{} {
	t.Helper()
	for _, r := range reqs {
		if r.Method != method {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal(r.Params, &m); err != nil {
			t.Fatalf("%s params 解析: %v", method, err)
		}
		return m
	}
	t.Fatalf("fake codex 没有收到 %s 请求", method)
	return nil
}

// TestCodexSettingsCarriedIntoTurnStart 设置页七项中"每轮参数"的真实生效链路:
// ApplyPrefs → Run → turn/start 参数与 thread/start、thread/resume 的 config 覆盖。
func TestCodexSettingsCarriedIntoTurnStart(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	ws := t.TempDir()
	p.ApplyPrefs(DesktopPrefs{
		ApprovalPolicy:   "on-request",
		Sandbox:          "workspace-write",
		Model:            "codex/gpt-5.1",
		Effort:           "high",
		Verbosity:        "low",
		ReasoningSummary: "detailed",
	})

	runOneTurn(t, p, &store.Thread{ID: "daemon-t1"}, ws)

	reqs := fakeCodexRequests(t, logPath)
	turn := findStartParams(t, reqs, "turn/start")
	if turn["approvalPolicy"] != "on-request" {
		t.Fatalf("turn/start 应携带审批策略 on-request, got=%v", turn["approvalPolicy"])
	}
	sp, ok := turn["sandboxPolicy"].(map[string]interface{})
	if !ok || sp["type"] != "workspaceWrite" {
		t.Fatalf("turn/start 应携带 sandboxPolicy {type: workspaceWrite}, got=%v", turn["sandboxPolicy"])
	}
	if turn["model"] != "gpt-5.1" {
		t.Fatalf("turn/start 应携带模型 slug gpt-5.1, got=%v", turn["model"])
	}
	if turn["effort"] != "high" {
		t.Fatalf("turn/start 应携带思考强度 high, got=%v", turn["effort"])
	}
	if turn["summary"] != "detailed" {
		t.Fatalf("turn/start 应携带思考摘要 summary=detailed, got=%v", turn["summary"])
	}
	start := findStartParams(t, reqs, "thread/start")
	cfg, ok := start["config"].(map[string]interface{})
	if !ok || cfg["model_verbosity"] != "low" || cfg["model_reasoning_summary"] != "detailed" {
		t.Fatalf("thread/start 应携带 config 覆盖(verbosity/summary), got=%v", start["config"])
	}

	// 既有会话(重启后 resume)同样带 config 覆盖:老会话改设置也要立刻生效。
	p2, logPath2 := newFakeCodexProvider(t, false)
	p2.ApplyPrefs(DesktopPrefs{Verbosity: "medium", ReasoningSummary: "concise"})
	runOneTurn(t, p2, &store.Thread{ID: "daemon-t2", CodexThreadID: "survived-thread"}, ws)
	reqs2 := fakeCodexRequests(t, logPath2)
	resume := findStartParams(t, reqs2, "thread/resume")
	cfg2, ok := resume["config"].(map[string]interface{})
	if !ok || cfg2["model_verbosity"] != "medium" || cfg2["model_reasoning_summary"] != "concise" {
		t.Fatalf("thread/resume 应携带 config 覆盖(verbosity/summary), got=%v", resume["config"])
	}
}

// TestCodexThreadModelEffortOverrideWins 会话内切换的模型/强度**每轮覆盖**桌面默认:
// 手机 ModelEffortCard → setThreadModel/setThreadEffort → TurnInput.modelId/effort →
// daemon 侧 Thread.ModelID/Effort(applyTurnOverrides)→ turn/start 显式参数。
func TestCodexThreadModelEffortOverrideWins(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	ws := t.TempDir()
	p.ApplyPrefs(DesktopPrefs{Model: "codex/gpt-5.1", Effort: "high"})
	runOneTurn(t, p, &store.Thread{ID: "daemon-t3", ModelID: "codex/gpt-5.5-turbo", Effort: "xhigh"}, ws)

	turn := findStartParams(t, fakeCodexRequests(t, logPath), "turn/start")
	if turn["model"] != "gpt-5.5-turbo" {
		t.Fatalf("会话内模型应覆盖桌面默认, got=%v", turn["model"])
	}
	if turn["effort"] != "xhigh" {
		t.Fatalf("会话内强度应覆盖桌面默认, got=%v", turn["effort"])
	}
}

// TestCodexHideReasoningFiltersThinkingDelta 隐藏推理过程开关的真实行为:
// HideReasoning=true → 思考 delta 被过滤(手机看不到思考流);
// HideReasoning=false → 思考 delta 原样推送(思考卡恢复显示)。
func TestCodexHideReasoningFiltersThinkingDelta(t *testing.T) {
	// 开关打开:过滤。
	p, _ := newFakeCodexProvider(t, true)
	ws := t.TempDir()
	p.ApplyPrefs(DesktopPrefs{HideReasoning: true})
	msgs := runOneTurn(t, p, &store.Thread{ID: "daemon-hide"}, ws)
	for _, m := range msgs {
		if m.GetThinkingDelta() != nil {
			t.Fatalf("HideReasoning=true 时不应推送 ThinkingDelta, got=%v", m.GetThinkingDelta())
		}
	}

	// 开关关闭:思考 delta 必须真实到达手机(这是用户否掉"锁定/过滤"后的核心诉求)。
	p.ApplyPrefs(DesktopPrefs{HideReasoning: false})
	msgs = runOneTurn(t, p, &store.Thread{ID: "daemon-show"}, ws)
	deltas := 0
	for _, m := range msgs {
		if td := m.GetThinkingDelta(); td != nil && td.Delta != "" {
			deltas++
		}
	}
	if deltas < 2 {
		t.Fatalf("HideReasoning=false 时思考流应原样推送, 只收到 %d 条 ThinkingDelta", deltas)
	}
}
