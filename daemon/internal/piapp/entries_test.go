// entries_test.go —— get_entries 解析、游标失效分类、回放形状、假 pi 子进程端到端。
//
// 【端到端怎么做】不起真 Pi(要 node + cli.js):用 Go 测试二进制自举模式 ——
// 把自己以 PA_FAKE_PI_CHILD=1 再跑一遍,TestFakePiProcess 说最小 JSONL 协议,
// 父进程走真实 CallCtx/dispatch/解析管线(GetEntries 全链路,含 since 增量与
// success:false → ErrCursorInvalid)。
package piapp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---- 解析(rpc.md「get_entries」应答形状)----

func TestParseEntriesPage(t *testing.T) {
	raw := json.RawMessage(`{
		"entries": [
			{"type":"message","id":"entryaa1","parentId":null,"timestamp":"t0","message":{"role":"user","content":"hi"}},
			{"type":"message","id":"entrybb2","parentId":"entryaa1","message":{"role":"assistant","content":[{"type":"text","text":"hello"}]}},
			{"type":"message","id":"","message":{"role":"user","content":"no id"}}
		],
		"leafId": "entrybb2"
	}`)
	page, err := parseEntriesPage(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if page.LeafID != "entrybb2" {
		t.Fatalf("leafId 解析错误: %q", page.LeafID)
	}
	if len(page.Entries) != 2 { // 无 id 条目必须被剔除(当不了游标)
		t.Fatalf("应保留 2 条带 id 条目, 实得 %d", len(page.Entries))
	}
	if page.LastID() != "entrybb2" {
		t.Fatalf("LastID 应为 entrybb2, 实得 %q", page.LastID())
	}
	// 空页:LastID 必须返回空串(调用方保持原游标不动)
	if (&EntriesPage{}).LastID() != "" || (*EntriesPage)(nil).LastID() != "" {
		t.Fatal("空页/nil 页的 LastID 应为空串")
	}
	if _, err := parseEntriesPage(json.RawMessage(`{`)); err == nil {
		t.Fatal("坏 JSON 应报错")
	}
}

// content 归一:字符串 content(user 简写)→ 单 text 块;数组逐块;坏数组 → nil。
func TestEntryContentBlocksNormalization(t *testing.T) {
	str := &EntryMessage{Content: json.RawMessage(`"你好"`)}
	if bs := str.ContentBlocks(); len(bs) != 1 || bs[0].Type != "text" || bs[0].Text != "你好" {
		t.Fatalf("字符串 content 应归一成单 text 块: %+v", bs)
	}
	arr := &EntryMessage{Content: json.RawMessage(`[{"type":"text","text":"a"},{"type":"thinking","thinking":"b"}]`)}
	if bs := arr.ContentBlocks(); len(bs) != 2 || bs[1].Thinking != "b" {
		t.Fatalf("数组 content 应逐块解析: %+v", bs)
	}
	bad := &EntryMessage{Content: json.RawMessage(`[1,2`)}
	if bs := bad.ContentBlocks(); bs != nil {
		t.Fatalf("坏数组应返回 nil: %+v", bs)
	}
}

// ---- 回放形状:WireEvents 合成的线上形状必须能被原样解析(与实时流同构)----

func TestWireEventsShapes(t *testing.T) {
	e := &Entry{Type: "message", ID: "e2", Message: &EntryMessage{Role: "assistant",
		Content: json.RawMessage(`[
			{"type":"thinking","thinking":"想一想"},
			{"type":"text","text":"答"},
			{"type":"toolCall","id":"call_1","name":"bash","arguments":{"command":"ls"}}
		]`)}}
	evs := e.WireEvents()
	if len(evs) != 3 {
		t.Fatalf("应有 3 条回放事件, 实得 %d", len(evs))
	}
	// 【P1-3】thinking 块回放发终局全文形状(thinking_end+content,不伪装增量),
	// 经 agent 包 MapThinkingSnapshot 映射成 snapshot 帧。
	if evs[0].Type != "message_update" || !strings.Contains(string(evs[0].Raw), `"thinking_end"`) || !strings.Contains(string(evs[0].Raw), `"content":"想一想"`) {
		t.Fatalf("thinking 回放应为 thinking_end 终局形状: %s", evs[0].Raw)
	}
	if evs[1].Type != "message_update" || !strings.Contains(string(evs[1].Raw), `"text_delta"`) || !strings.Contains(string(evs[1].Raw), "答") {
		t.Fatalf("text 形状不符: %s", evs[1].Raw)
	}
	// JSON 键序不保证(map 序列化按字典序),断言只看字段在场
	if evs[2].Type != "message_update" || !strings.Contains(string(evs[2].Raw), `"toolcall_end"`) ||
		!strings.Contains(string(evs[2].Raw), `"name":"bash"`) ||
		!strings.Contains(string(evs[2].Raw), `"command":"ls"`) {
		t.Fatalf("toolcall_end 形状不符: %s", evs[2].Raw)
	}

	tr := &Entry{Type: "message", ID: "e3", Message: &EntryMessage{Role: "toolResult",
		ToolCallID: "call_1", ToolName: "bash", IsError: true,
		Content: json.RawMessage(`[{"type":"text","text":"total 0"}]`)}}
	tev := tr.WireEvents()
	if len(tev) != 1 || tev[0].Type != "tool_execution_end" {
		t.Fatalf("toolResult 应合成一条 tool_execution_end: %+v", tev)
	}
	for _, want := range []string{`"toolCallId":"call_1"`, `"toolName":"bash"`, `"text":"total 0"`, `"isError":true`} {
		if !strings.Contains(string(tev[0].Raw), want) {
			t.Fatalf("tool_execution_end 缺 %s: %s", want, tev[0].Raw)
		}
	}

	// user 没有流式事件对应面;message 为 nil(非 message 条目)同样为空。
	user := &Entry{ID: "u1", Message: &EntryMessage{Role: "user", Content: json.RawMessage(`"hi"`)}}
	if u := user.WireEvents(); len(u) != 0 {
		t.Fatalf("user 条目不应产生回放事件: %+v", u)
	}
	if m := (&Entry{ID: "m1"}).WireEvents(); m != nil {
		t.Fatalf("无 message 条目应返回 nil: %+v", m)
	}
}

// 【P1-3 snapshot 契约】thinking 块回放必须发终局全文(thinking_end 形状),
// 不得拆成 thinking_delta 假增量 —— 客户端对 snapshot 整体替换、对 delta 追加,
// 伪装增量会让断点续传场景的思考文本语义失真(完整块本就一次到齐)。
func TestWireEventsThinkingIsSnapshotNotDelta(t *testing.T) {
	e := &Entry{Type: "message", ID: "e9", Message: &EntryMessage{Role: "assistant",
		Content: json.RawMessage(`[{"type":"thinking","thinking":"第一段思考"}]`)}}
	evs := e.WireEvents()
	if len(evs) != 1 || evs[0].Type != "message_update" {
		t.Fatalf("应合成 1 条 message_update: %+v", evs)
	}
	raw := string(evs[0].Raw)
	if !strings.Contains(raw, `"thinking_end"`) || !strings.Contains(raw, `"content":"第一段思考"`) {
		t.Fatalf("thinking 回放应为 thinking_end+content 终局形状: %s", raw)
	}
	if strings.Contains(raw, `"thinking_delta"`) || strings.Contains(raw, `"delta"`) {
		t.Fatalf("回放不得伪装增量(thinking_delta/delta 不应出现): %s", raw)
	}
}

// ---- 游标失效分类 ----

func TestIsCursorError(t *testing.T) {
	for _, msg := range []string{"Entry not found: abc", "no such entry", "Unknown entry id", "invalid cursor"} {
		if !isCursorError(errors.New(msg)) {
			t.Fatalf("%q 应判为游标失效", msg)
		}
	}
	if isCursorError(errors.New("pi 子进程已退出")) {
		t.Fatal("传输层错误不应判为游标失效")
	}
}

// ---- 端到端:假 pi 子进程 ----

// TestFakePiProcess 仅作为子进程运行(env 门禁):对 get_entries 给固定应答 ——
// 全量 2 条;since=entrybb2/entrycc3 增量 1 条;其他 since 一律 success:false。
// 另支持(供 client_test 的就绪重试/set_model 用例):
//   - PA_FAKE_PI_DELAY=<秒>:扫描 stdin 前先睡 N 秒 —— 模拟 pi 1.0.2 的启动
//     门控(扩展绑定完成前不读 stdin,先发的命令在管道里排队);
//   - get_state/switch_session:固定应答(1.0.2 bundle 实证形状);
//   - set_model:按 1.0.2 语义校验 provider+modelId 二元组(缺任一即
//     "Model not found");PA_FAKE_PI_EXPECT_MODEL 设 "<provider>/<modelId>" 时
//     进一步要求二元组恰为该值 —— 父进程据此断言线上帧"字段在场且正确"。
func TestFakePiProcess(t *testing.T) {
	if os.Getenv("PA_FAKE_PI_CHILD") != "1" {
		t.Skip("仅作为 TestGetEntriesAgainstFakePi 的假 pi 子进程运行")
	}
	if d := os.Getenv("PA_FAKE_PI_DELAY"); d != "" {
		var secs int
		if _, err := fmt.Sscanf(d, "%d", &secs); err == nil && secs > 0 {
			time.Sleep(time.Duration(secs) * time.Second)
		}
	}
	expectModel := os.Getenv("PA_FAKE_PI_EXPECT_MODEL")
	if os.Getenv("PA_FAKE_PI_SILENT") == "1" {
		// 静默模式:不回应答(供"ctx 到期必须报错"类测试用;父进程会杀掉我们)
		time.Sleep(10 * time.Minute)
		os.Exit(0)
	}
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 64*1024), 1024*1024)
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for in.Scan() {
		var req struct {
			Type        string `json:"type"`
			ID          string `json:"id"`
			Since       string `json:"since"`
			SessionPath string `json:"sessionPath"`
			Provider    string `json:"provider"`
			ModelID     string `json:"modelId"`
		}
		if json.Unmarshal(in.Bytes(), &req) != nil {
			continue
		}
		resp := map[string]any{"type": "response", "id": req.ID}
		switch req.Type {
		case "get_state":
			resp["success"] = true
			resp["data"] = map[string]any{
				"sessionFile": `C:\fake\pi-session.jsonl`,
				"sessionId":   "fake-session-id",
				"isStreaming": false,
			}
		case "switch_session":
			resp["success"] = true
			resp["data"] = map[string]any{"cancelled": false}
		case "set_model":
			// 1.0.2 语义:provider/modelId 任一缺省即拒(真机回退根因的镜像断言)
			if req.Provider == "" || req.ModelID == "" ||
				(expectModel != "" && req.Provider+"/"+req.ModelID != expectModel) {
				resp["success"] = false
				resp["error"] = fmt.Sprintf("Model not found: %s/%s", req.Provider, req.ModelID)
			} else {
				resp["success"] = true
				resp["data"] = map[string]any{"provider": req.Provider, "modelId": req.ModelID}
			}
		case "get_available_models":
			// 1.0.2 应答形状(bundle 实证:getAvailableSnapshot() → {models}):
			// 两模型固定表,供 client_test 的就绪重试+真表命中用例使用;字段名
			// (id/name/provider/reasoning/input/contextWindow/thinkingLevelMap)
			// 与 agent 层 probePiModels 的解析结构逐一对齐。
			resp["success"] = true
			resp["data"] = map[string]any{
				"models": []map[string]any{
					{
						"id": "glm-5.3", "name": "GLM-5.3", "provider": "zai-coding-cn",
						"reasoning": true, "input": []string{"text"}, "contextWindow": 1000000,
						"thinkingLevelMap": map[string]any{"off": nil, "low": "low", "high": "high", "max": "max"},
					},
					{
						"id": "deepseek-flash", "name": "DeepSeek V4.1 Flash", "provider": "deepseek",
						"reasoning": false, "input": []string{"text", "image"}, "contextWindow": 128000,
					},
				},
			}
		case "get_entries":
			switch req.Since {
			case "":
				resp["success"] = true
				resp["data"] = map[string]any{
					"entries": []map[string]any{
						{"type": "message", "id": "entryaa1", "message": map[string]any{"role": "user", "content": "hi"}},
						{"type": "message", "id": "entrybb2", "parentId": "entryaa1",
							"message": map[string]any{"role": "assistant", "content": []map[string]any{{"type": "text", "text": "hello"}}}},
					},
					"leafId": "entrybb2",
				}
			case "entrybb2", "entrycc3":
				resp["success"] = true
				resp["data"] = map[string]any{
					"entries": []map[string]any{
						{"type": "message", "id": "entrycc3", "parentId": "entrybb2",
							"message": map[string]any{"role": "toolResult", "toolCallId": "call_1", "toolName": "bash",
								"content": []map[string]any{{"type": "text", "text": "ok"}}, "isError": false}},
					},
					"leafId": "entrycc3",
				}
			default:
				resp["success"] = false
				resp["error"] = "Entry not found: " + req.Since
			}
		default:
			continue // 与原行为一致:无关命令不回(父进程按超时/死亡处理)
		}
		line, _ := json.Marshal(resp)
		w.WriteString(string(line) + "\n")
		w.Flush()
	}
	os.Exit(0) // 别让测试框架往协议管道写 PASS
}

// 自举拉起假 pi 子进程的客户端(走真实 readLoop/dispatch 管线)。
func newFakePiClient(t *testing.T) *Client {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestFakePiProcess$")
	cmd.Env = append(os.Environ(), "PA_FAKE_PI_CHILD=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("拉起假 pi 子进程失败: %v", err)
	}
	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReaderSize(stdout, 64*1024),
		pending: map[string]chan piResp{},
		deathCh: make(chan struct{}),
		reaped:  make(chan struct{}),
	}
	c.syncHandlers()
	go c.readLoop()
	t.Cleanup(c.Close)
	return c
}

func TestGetEntriesAgainstFakePi(t *testing.T) {
	c := newFakePiClient(t)
	ctx := context.Background()

	// 全量:entries + leafId + LastID
	page, err := c.GetEntries(ctx, "", "")
	if err != nil {
		t.Fatalf("全量 get_entries 失败: %v", err)
	}
	if page.LeafID != "entrybb2" || len(page.Entries) != 2 || page.LastID() != "entrybb2" {
		t.Fatalf("全量形状不符: %+v", page)
	}

	// since 增量:严格之后的追加序条目(piprobe 第 8 项的形状)
	inc, err := c.GetEntries(ctx, "", "entrybb2")
	if err != nil {
		t.Fatalf("增量 get_entries 失败: %v", err)
	}
	if len(inc.Entries) != 1 || inc.Entries[0].ID != "entrycc3" || inc.Entries[0].Message.Role != "toolResult" {
		t.Fatalf("增量形状不符: %+v", inc)
	}

	// 错误游标:success:false 必须变成 ErrCursorInvalid(调用方据此转全量兜底)
	if _, err := c.GetEntries(ctx, "", "piprobe-nonexistent-id"); !errors.Is(err, ErrCursorInvalid) {
		t.Fatalf("错误游标应包成 ErrCursorInvalid, 实得 %v", err)
	}

	// 会话校验:子进程挂载的会话与目标不符时拒绝续传(防给错误会话回放)
	c.updateSession(filepath.Join("C:", "other.jsonl"), "") // P2-4:会话缓存走原子快照写入
	if _, err := c.GetEntries(ctx, filepath.Join("C:", "target.jsonl"), "entrybb2"); err == nil || !strings.Contains(err.Error(), "会话不符") {
		t.Fatalf("会话不符应被拒绝, 实得 %v", err)
	}
	// 挂载一致时不拦截(继续走协议)
	if _, err := c.GetEntries(ctx, c.SessionFile(), "entrybb2"); err != nil {
		t.Fatalf("会话一致不应拦截: %v", err)
	}
}

// CallCtx 应答超时的兜底参数也在 GetEntries 生效:构造一个不回话的假 pi,
// 把预算压到极小不现实(常量不可调),改为验证"子进程死了必须报错而不是挂死"。
func TestGetEntriesOnDeadProcess(t *testing.T) {
	c := &Client{pending: map[string]chan piResp{}, deathCh: make(chan struct{})}
	c.mu.Lock()
	c.dead = true
	c.deathErr = errors.New("simulated exit")
	c.mu.Unlock()
	if _, err := c.GetEntries(context.Background(), "", ""); err == nil {
		t.Fatal("死子进程的 get_entries 必须报错")
	}
}

// 超时上界保险:GetEntries 挂在无回话的活子进程上时,30s 预算必须兜住。
// 用假子进程(不回应答)验证 ctx 短路 —— 不等满 30s,外层 ctx 先取消。
func TestGetEntriesRespectsCallerContext(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestFakePiProcess$")
	cmd.Env = append(os.Environ(), "PA_FAKE_PI_CHILD=1", "PA_FAKE_PI_SILENT=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("拉起假 pi 失败: %v", err)
	}
	defer func() {
		stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	c := &Client{
		stdin:   stdin,
		stdout:  bufio.NewReaderSize(stdout, 64*1024),
		pending: map[string]chan piResp{},
		deathCh: make(chan struct{}),
		reaped:  make(chan struct{}),
	}
	c.syncHandlers()
	go c.readLoop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := c.GetEntries(ctx, "", ""); err == nil {
		t.Fatal("无应答 + ctx 到期必须报错")
	}
}
