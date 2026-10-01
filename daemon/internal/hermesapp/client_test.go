// client_test.go —— hermesapp.Client 行为契约(dshapp/codexapp 同款夹具纪律):
//   - 假子进程端到端(helper process 说 hermes ACP 方言,形状逐字对 acp_adapter
//     实测):握手能力面 → new → load(应答前流出历史回放)/load 查无(null)→
//     list 游标 → fork → prompt(审批反向请求双向 + session/update 流);
//   - 反向请求路由:已装钩子交钩子,未装/不认识回 -32601(不伪造 decision);
//   - 管道直控(io.Pipe)单测 Respond/RespondError 帧形状。
package hermesapp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// ---- 假子进程:说 hermes ACP 方言的最小 JSON-RPC 服务 ----

// fakeHandshakeJSON 形状对 server.py initialize(0.9.0 实测:loadSession 布尔 +
// sessionCapabilities fork/list/resume 空对象 + promptCapabilities.image)。
const fakeHandshakeJSON = `{"protocolVersion":1,
 "agentInfo":{"name":"hermes-agent","version":"9.9.9-fake"},
 "agentCapabilities":{"loadSession":true,"promptCapabilities":{"image":true},
  "sessionCapabilities":{"fork":{},"list":{},"resume":{}}},
 "authMethods":[{"id":"terminal-setup","name":"Terminal setup"}]}`

// fakePermissionRequestJSON 形状对 permissions.py _build_permission_options +
// _build_permission_tool_call(危险命令审批,五选项)。【单行】协议是换行分帧,
// 夹具 JSON 绝不能带字面换行(dshapp 同坑:must Marshal 压缩)。
const fakePermissionRequestJSON = `{"sessionId":"fake-sess-1","toolCall":{"toolCallId":"perm-check-1","title":"运行命令: mkdir x","kind":"execute","status":"pending","content":[{"type":"content","content":{"type":"text","text":"运行命令: mkdir x\n$ mkdir x"}}],"rawInput":{"command":"mkdir x","description":"运行命令"}},"options":[{"optionId":"allow_once","kind":"allow_once","name":"Allow once"},{"optionId":"allow_session","kind":"allow_always","name":"Allow for session"},{"optionId":"allow_always","kind":"allow_always","name":"Allow always"},{"optionId":"deny","kind":"reject_once","name":"Deny"},{"optionId":"deny_always","kind":"reject_always","name":"Deny always"}]}`

// TestFakeHermesHelperProcess 是 go test helper 子进程(main 测试经 os.Args[0]
// 拉起,piapp/codexapp/dshapp 同款模式)。绝不能带 -test.v 拉起 —— 测试框架会先
// 往 stdout 打 "=== RUN",污染协议流。
func TestFakeHermesHelperProcess(t *testing.T) {
	if os.Getenv("HERMESAPP_FAKE_HERMES") != "1" {
		return
	}
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	notify := func(method string, params string) {
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": json.RawMessage(params)})
		out.Write(b)
		out.WriteByte('\n')
		out.Flush()
	}
	respond := func(id int64, result any) {
		b, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
		if err != nil {
			return
		}
		out.Write(b)
		out.WriteByte('\n')
		out.Flush()
	}
	respondRaw := func(id int64, raw string) {
		fmt.Fprintf(out, "{\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":%s}\n", id, raw)
		out.Flush()
	}
	for sc.Scan() {
		var m struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		switch m.Method {
		case "initialize":
			respond(*m.ID, json.RawMessage(fakeHandshakeJSON))
		case "session/new":
			respond(*m.ID, map[string]any{"sessionId": "fake-sess-1",
				"models": map[string]any{"currentModelId": "deepseek:deepseek-flash", "availableModels": []any{}},
				"modes":  map[string]any{"currentModeId": "default", "availableModes": []any{}},
			})
		case "session/load":
			var p struct {
				SessionID string `json:"sessionId"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if p.SessionID != "known-sess" {
				respondRaw(*m.ID, "null") // server.py load_session 查无 → None → null
				continue
			}
			// ACP 规范(实证 server.py _session_response_fields):历史回放必须在
			// 应答前以 session/update 流出。
			notify("session/update", `{"sessionId":"known-sess","update":{"sessionUpdate":"user_message_chunk","content":{"type":"text","text":"旧问题"}}}`)
			notify("session/update", `{"sessionId":"known-sess","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"旧回答"}}}`)
			notify("session/update", `{"sessionId":"known-sess","update":{"sessionUpdate":"tool_call","toolCallId":"tc-old","title":"read_file","kind":"read","status":"pending"}}`)
			respond(*m.ID, map[string]any{
				"models": map[string]any{"currentModelId": "deepseek:deepseek-flash", "availableModels": []any{}},
				"modes":  map[string]any{"currentModeId": "default", "availableModes": []any{}},
			})
		case "session/resume":
			notify("session/update", `{"sessionId":"known-sess","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"resume 回放"}}}`)
			respond(*m.ID, map[string]any{
				"models": map[string]any{"currentModelId": "deepseek:deepseek-flash", "availableModels": []any{}},
				"modes":  map[string]any{"currentModeId": "default", "availableModes": []any{}},
			})
		case "session/list":
			respond(*m.ID, map[string]any{"sessions": []any{
				map[string]any{"sessionId": "known-sess", "cwd": `C:\ws`, "title": "旧会话", "updatedAt": "2026-10-01T00:00:00Z"},
				map[string]any{"sessionId": "fake-sess-1", "cwd": `C:\ws`},
			}, "nextCursor": nil})
		case "session/fork":
			respond(*m.ID, map[string]any{"sessionId": "fork-sess-1",
				"models": map[string]any{"currentModelId": "deepseek:deepseek-flash", "availableModels": []any{}}})
		case "session/set_session_model":
			respond(*m.ID, map[string]any{})
		case "session/prompt":
			var p struct {
				Prompt []struct {
					Text string `json:"text"`
				} `json:"prompt"`
			}
			_ = json.Unmarshal(m.Params, &p)
			text := ""
			if len(p.Prompt) > 0 {
				text = p.Prompt[0].Text
			}
			if strings.Contains(text, "need-approval") {
				// 反向请求:等客户端的 outcome 应答,再把它回显进正文(端到端对账)。
				fmt.Fprintf(out, "{\"jsonrpc\":\"2.0\",\"id\":9001,\"method\":\"session/request_permission\",\"params\":%s}\n", fakePermissionRequestJSON)
				out.Flush()
				if !sc.Scan() {
					return
				}
				var resp struct {
					ID     int64 `json:"id"`
					Result struct {
						Outcome struct {
							Outcome  string `json:"outcome"`
							OptionID string `json:"optionId"`
						} `json:"outcome"`
					} `json:"result"`
				}
				if json.Unmarshal(sc.Bytes(), &resp) == nil && resp.ID == 9001 {
					notify("session/update", fmt.Sprintf(
						`{"sessionId":"fake-sess-1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"outcome=%s:%s"}}}`,
						resp.Result.Outcome.Outcome, resp.Result.Outcome.OptionID))
				}
			}
			notify("session/update", `{"sessionId":"fake-sess-1","update":{"sessionUpdate":"tool_call","toolCallId":"tc-1","title":"write_file","kind":"edit","status":"pending","content":[{"type":"diff","path":"a.txt","newText":"hi"}]}}`)
			notify("session/update", `{"sessionId":"fake-sess-1","update":{"sessionUpdate":"usage_update","size":1000,"used":42}}`)
			notify("session/update", `{"sessionId":"fake-sess-1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"完成"}}}`)
			respond(*m.ID, map[string]any{"stopReason": "end_turn"})
		default:
			respond(*m.ID, map[string]any{})
		}
	}
	os.Exit(0)
}

// startFake 拉起假子进程客户端。
func startFake(t *testing.T) *Client {
	t.Helper()
	t.Setenv("HERMESAPP_FAKE_HERMES", "1")
	c, err := startProcess(os.Args[0], []string{"-test.run=^TestFakeHermesHelperProcess$"}, "")
	if err != nil {
		t.Fatalf("拉起假子进程失败: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

// collectUpdates 装当轮通知收集器,返回收集通道(装卸随 t.Cleanup)。
func collectUpdates(t *testing.T, c *Client) chan string {
	t.Helper()
	ch := make(chan string, 64)
	c.SetTurnHandlers(func(raw json.RawMessage) {
		var u struct {
			Update struct {
				SessionUpdate string `json:"sessionUpdate"`
			} `json:"update"`
		}
		if json.Unmarshal(raw, &u) == nil {
			select {
			case ch <- u.Update.SessionUpdate:
			default:
			}
		}
	}, nil)
	t.Cleanup(func() { c.SetTurnHandlers(nil, nil) })
	return ch
}

func waitUpdate(t *testing.T, ch chan string, want string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case got := <-ch:
			if got == want {
				return
			}
		case <-deadline:
			t.Fatalf("5s 内没等到 %s 通知", want)
		}
	}
}

// TestFakeProcessLifecycle 端到端:握手能力面 → new → 模型切换 → list →
// load(先回放后应答)→ load 查无(null→ErrSessionNotFound)→ fork。
func TestFakeProcessLifecycle(t *testing.T) {
	c := startFake(t)

	init, err := c.Initialize(nil)
	if err != nil {
		t.Fatalf("initialize 应成功: %v", err)
	}
	if !init.AgentCapabilities.CanLoadSession() || !init.AgentCapabilities.CanList() ||
		!init.AgentCapabilities.CanResume() || !init.AgentCapabilities.CanFork() {
		t.Fatalf("能力快照不符: %+v", init.AgentCapabilities)
	}

	st, err := c.SessionNew(nil, "C:\\ws")
	if err != nil || st.SessionID != "fake-sess-1" {
		t.Fatalf("session/new 不符: %+v err=%v", st, err)
	}
	if st.Models.CurrentModelID != "deepseek:deepseek-flash" {
		t.Fatalf("模型面不符: %+v", st.Models)
	}
	if err := c.SetSessionModel(nil, st.SessionID, "deepseek:deepseek-flash"); err != nil {
		t.Fatalf("set_session_model 应成功: %v", err)
	}

	list, err := c.ListSessions(nil, "")
	if err != nil || len(list.Sessions) != 2 {
		t.Fatalf("session/list 不符: %+v err=%v", list, err)
	}
	if list.Sessions[0].SessionID != "known-sess" || list.Sessions[0].Title != "旧会话" || list.Sessions[0].Cwd != `C:\ws` {
		t.Fatalf("list 条目不符: %+v", list.Sessions[0])
	}

	// load:回放通知先于应答到达(通知通道在调用前装好)。
	ch := collectUpdates(t, c)
	loaded := make(chan error, 1)
	go func() {
		_, err := c.LoadSession(nil, `C:\ws`, "known-sess")
		loaded <- err
	}()
	waitUpdate(t, ch, "user_message_chunk")
	waitUpdate(t, ch, "agent_message_chunk")
	waitUpdate(t, ch, "tool_call")
	if err := <-loaded; err != nil {
		t.Fatalf("load 已知会话应成功: %v", err)
	}

	if _, err := c.LoadSession(nil, `C:\ws`, "ghost-sess"); err != ErrSessionNotFound {
		t.Fatalf("load 幽灵会话应 ErrSessionNotFound, got %v", err)
	}

	fk, err := c.ForkSession(nil, `C:\ws`, "known-sess")
	if err != nil || fk.SessionID != "fork-sess-1" {
		t.Fatalf("fork 不符: %+v err=%v", fk, err)
	}
}

// TestFakeProcessResumeSession resume 也先回放后应答(server.py 同一
// _session_response_fields 通路)。
func TestFakeProcessResumeSession(t *testing.T) {
	c := startFake(t)
	ch := collectUpdates(t, c)
	done := make(chan error, 1)
	go func() {
		_, err := c.ResumeSession(nil, `C:\ws`, "known-sess")
		done <- err
	}()
	waitUpdate(t, ch, "agent_message_chunk")
	if err := <-done; err != nil {
		t.Fatalf("resume 应成功: %v", err)
	}
}

// TestFakeProcessApprovalRound 端到端审批:prompt → 适配器发
// session/request_permission → 钩子解析并以 selected 应答 → 适配器把收到的
// optionId 回显进正文(对账),随后 tool_call(diff)/usage/正文流式到达。
func TestFakeProcessApprovalRound(t *testing.T) {
	for _, tc := range []struct {
		name       string
		allow      bool
		wantOption string
	}{
		{"允许", true, "allow_once"},
		{"拒绝", false, "deny"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := startFake(t)
			st, err := c.SessionNew(nil, "C:\\ws")
			if err != nil {
				t.Fatalf("session/new: %v", err)
			}
			texts := make(chan string, 64)
			events := make(chan StreamEvent, 64)
			c.SetTurnHandlers(func(raw json.RawMessage) {
				ev := ParseSessionUpdate(raw)
				select {
				case events <- ev:
				default:
				}
				if ev.MessageChunk != "" {
					select {
					case texts <- ev.MessageChunk:
					default:
					}
				}
			}, func(id int64, method string, params json.RawMessage) {
				if method != "session/request_permission" {
					_ = c.RespondError(id, -32601, "method not found: "+method)
					return
				}
				perm, err := ParseRequestPermission(params)
				if err != nil {
					_ = c.RespondError(id, -32602, err.Error())
					return
				}
				option := perm.RejectOptionID()
				if tc.allow {
					option = perm.AllowOptionID()
				}
				_ = c.Respond(id, PermissionOutcomeSelected(option))
			})
			defer c.SetTurnHandlers(nil, nil)

			stop, err := c.Prompt(nil, st.SessionID, "need-approval 创建文件")
			if err != nil {
				t.Fatalf("prompt 应成功: %v", err)
			}
			if stop != "end_turn" {
				t.Fatalf("stopReason = %q", stop)
			}
			// 对账:适配器把收到的 outcome 回显进正文;diff 工具帧与 usage 帧照常到。
			var sawOutcome, sawTool, sawUsage bool
			deadline := time.After(5 * time.Second)
			for !(sawOutcome && sawTool && sawUsage) {
				select {
				case got := <-texts:
					if strings.Contains(got, "outcome=selected:"+tc.wantOption) {
						sawOutcome = true
					}
				case ev := <-events:
					if ev.ToolCall != nil && ev.ToolCall.Diff != nil && ev.ToolCall.Diff.Path == "a.txt" {
						sawTool = true
					}
					if ev.Usage != nil && ev.Usage.Size == 1000 {
						sawUsage = true
					}
				case <-deadline:
					t.Fatalf("5s 内对账未齐(outcome=%v tool=%v usage=%v)", sawOutcome, sawTool, sawUsage)
				}
			}
		})
	}
}

// TestReverseRequestNoHandlerFallsBack 未装钩子时反向请求必须回 -32601
// (不让适配器悬挂,也不伪造 decision)。
func TestReverseRequestNoHandlerFallsBack(t *testing.T) {
	_, sOut, sIn := pipeClient(t)
	fmt.Fprintf(sOut, "{\"jsonrpc\":\"2.0\",\"id\":77,\"method\":\"session/request_permission\",\"params\":{\"options\":[]}}\n")
	line, err := bufio.NewReader(sIn).ReadBytes('\n')
	if err != nil {
		t.Fatalf("读应答失败: %v", err)
	}
	var resp struct {
		ID    int64 `json:"id"`
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(line, &resp) != nil || resp.ID != 77 || resp.Error.Code != -32601 {
		t.Fatalf("应回 -32601 错误帧: %s", line)
	}
}

// TestRespondAndRespondErrorFrames Respond/RespondError 的出帧形状。
func TestRespondAndRespondErrorFrames(t *testing.T) {
	c, _, sIn := pipeClient(t)
	// io.Pipe 是同步管道:写阻塞到有人读 —— 读面常驻 goroutine,先读后断言。
	lines := make(chan []byte, 8)
	go func() {
		defer close(lines)
		rd := bufio.NewReader(sIn)
		for {
			line, err := rd.ReadBytes('\n')
			if err != nil {
				return
			}
			lines <- line
		}
	}()
	if err := c.Respond(5, PermissionOutcomeSelected("allow_once")); err != nil {
		t.Fatalf("Respond 应成功: %v", err)
	}
	if err := c.RespondError(6, -32000, "审批通道不可用"); err != nil {
		t.Fatalf("RespondError 应成功: %v", err)
	}
	l1 := <-lines
	l2 := <-lines
	var r1 struct {
		ID     int64 `json:"id"`
		Result struct {
			Outcome struct {
				Outcome  string `json:"outcome"`
				OptionID string `json:"optionId"`
			} `json:"outcome"`
		} `json:"result"`
	}
	var r2 struct {
		ID    int64 `json:"id"`
		Error struct {
			Code int    `json:"code"`
			Text string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(l1, &r1) != nil || r1.ID != 5 || r1.Result.Outcome.OptionID != "allow_once" {
		t.Fatalf("Respond 帧不符: %s", l1)
	}
	if json.Unmarshal(l2, &r2) != nil || r2.ID != 6 || r2.Error.Code != -32000 {
		t.Fatalf("RespondError 帧不符: %s", l2)
	}
}

// TestHandlerSwapAtomic 装卸钩子后 readLoop 分发随之切换(原子快照)。
func TestHandlerSwapAtomic(t *testing.T) {
	c, sOut, _ := pipeClient(t)
	ch := make(chan string, 8)
	c.SetTurnHandlers(func(raw json.RawMessage) { ch <- "first" }, nil)
	fmt.Fprint(sOut, "{\"jsonrpc\":\"2.0\",\"method\":\"session/update\",\"params\":{\"update\":{\"sessionUpdate\":\"agent_message_chunk\"}}}\n")
	select {
	case got := <-ch:
		if got != "first" {
			t.Fatalf("首钩子未收到: %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("2s 内通知未达")
	}
	c.SetTurnHandlers(nil, nil)
	fmt.Fprint(sOut, "{\"jsonrpc\":\"2.0\",\"method\":\"session/update\",\"params\":{}}\n")
	select {
	case got := <-ch:
		t.Fatalf("卸钩后不应再分发: %q", got)
	case <-time.After(200 * time.Millisecond):
	}
}

// pipeClient 构造管道驱动的裸 Client(stdout 读面 + stdin 写面),供直控帧序。
func pipeClient(t *testing.T) (*Client, *io.PipeWriter, *io.PipeReader) {
	t.Helper()
	sOutR, sOutW := io.Pipe()
	sInR, sInW := io.Pipe()
	c := &Client{stdout: bufio.NewReader(sOutR), stdin: sInW,
		pending: map[int64]chan rpcResp{}, exited: make(chan struct{})}
	go c.readLoop()
	t.Cleanup(func() {
		_ = sOutW.Close() // 先断数据源让 readLoop 退出,reap 才能收尸
		<-c.exited
		c.Close()
		_ = sInR.Close()
	})
	return c, sOutW, sInR
}
