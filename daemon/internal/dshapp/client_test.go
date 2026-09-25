// client_test.go —— dshapp.Client 行为契约(对标 codexapp 测试文件族):
//   - 分帧/dispatch(应答路由、通知双钩子、反向请求、错误保真);
//   - prompt 不受 callTimeout 管(会话级调用,轮次超时归上层看门狗);
//   - 假子进程端到端(helper process 说探针夹具方言):握手→建会话→prompt;
//   - 子进程死亡叫醒所有 pending(重启路径的前提);收尸 waitOnce;
//   - handler 快照装卸与 dispatch 并发(-race 审批并发基础);
//   - DSH_HOME 隔离默认值与 .cmd launch 归一。
package dshapp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// ---- 假子进程:说 dsh ACP 方言的最小 JSON-RPC 服务 ----

// TestFakeDshHelperProcess 是 go test helper 子进程(main 测试经 os.Args[0]
// 拉起,piapp/codexapp 同款模式)。应答形状逐字用探针夹具。
// 注意:绝不能带 -test.v 拉起 —— 测试框架会先往 stdout 打 "=== RUN",污染协议流。
func TestFakeDshHelperProcess(t *testing.T) {
	if os.Getenv("DSHAPP_FAKE_DSH") != "1" {
		return
	}
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	// respond 单行分帧:夹具 JSON 含换行,必须经 Marshal 压缩(协议是换行分隔)。
	respond := func(id int64, result interface{}) {
		b, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": id, "result": result})
		if err != nil {
			return
		}
		out.Write(b)
		out.WriteByte('\n')
		out.Flush()
	}
	respondErr := func(id int64, code int, msg string) {
		b, _ := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0", "id": id,
			"error": map[string]interface{}{"code": code, "message": msg},
		})
		out.Write(b)
		out.WriteByte('\n')
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
			respond(*m.ID, json.RawMessage(probeHandshakeJSON))
		case "session/new":
			respond(*m.ID, map[string]interface{}{"sessionId": "fake-sess-1", "configOptions": json.RawMessage(probeConfigOptionsJSON)})
		case "session/resume":
			// rc.3 契约(升级兼容探针 docs/verify/evidence/dsh-resume-compat-probe.md
			// 实测,与 set_config_option 的 rc.3 钉子同口径):params 必须带 cwd,
			// 缺失按 zod 行为回 -32602(data 指明 cwd required —— A1 帧);应答
			// 不回显 sessionId,只有 configOptions(A2 帧全文 2073 字节无该键)。
			var p struct {
				SessionID string `json:"sessionId"`
				Cwd       string `json:"cwd"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if p.Cwd == "" {
				respondErr(*m.ID, -32602, "Invalid params")
				continue
			}
			respond(*m.ID, map[string]interface{}{"configOptions": json.RawMessage(probeConfigOptionsJSON)})
		case "session/prompt":
			// 先发一条 session/update 通知,再回终局(验证通知+应答混流分帧)。
			notif, _ := json.Marshal(map[string]interface{}{
				"jsonrpc": "2.0", "method": "session/update",
				"params": map[string]interface{}{
					"sessionId": "fake-sess-1",
					"update": map[string]interface{}{
						"sessionUpdate": "agent_message_chunk",
						"content":       map[string]interface{}{"type": "text", "text": "2"},
					},
				},
			})
			out.Write(notif)
			out.WriteByte('\n')
			respond(*m.ID, map[string]interface{}{"stopReason": "end_turn"})
		case "session/set_config_option":
			// rc.3 参数名契约(探针实测,configoptions_test.go):必须带 configId,
			// 缺失按 zod 行为回 -32602。value 形状按 configId 分家(探针原帧):
			// model 必须原样是双层编码串(裸模型名回错);reasoning_effort 是
			// off/low/high/max 纯串 —— 对 effort 校验编码串是假子进程越权,会误红。
			var p struct {
				ConfigID string `json:"configId"`
				Value    string `json:"value"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if p.ConfigID == "" {
				respondErr(*m.ID, -32602, "Invalid params")
				continue
			}
			if p.ConfigID == "model" {
				if _, _, ok := DecodeModelTuple(p.Value); !ok {
					respondErr(*m.ID, -32602, "bare model value")
					continue
				}
			}
			respond(*m.ID, map[string]interface{}{})
		case "boom":
			respondErr(*m.ID, -32603, "no API key for provider route")
		default:
			respond(*m.ID, map[string]interface{}{})
		}
	}
	os.Exit(0)
}

// startFake 拉起假子进程客户端。
func startFake(t *testing.T) *Client {
	t.Helper()
	t.Setenv("DSHAPP_FAKE_DSH", "1")
	c, err := startProcess(os.Args[0], []string{"-test.run=^TestFakeDshHelperProcess$"}, StartOptions{})
	if err != nil {
		t.Fatalf("拉起假子进程失败: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

// pipeClient 构造管道驱动的裸 Client(stdout 读面 + stdin 写面),供分帧/错误
// 保真单测直控帧序。返回 (client, stdout写端[模拟子进程输出], stdin读端[读出请求])。
func pipeClient(t *testing.T) (*Client, *io.PipeWriter, *io.PipeReader) {
	t.Helper()
	sOutR, sOutW := io.Pipe()
	sInR, sInW := io.Pipe()
	c := &Client{stdout: bufio.NewReader(sOutR), stdin: sInW, pending: map[int64]chan rpcResp{}}
	go c.readLoop()
	t.Cleanup(func() {
		c.Close()
		_ = sOutW.Close()
		_ = sInR.Close()
	})
	return c, sOutW, sInR
}

// readRequest 从 stdin 面读出客户端发出的请求行(取 id)。
func readRequest(t *testing.T, r *io.PipeReader) int64 {
	t.Helper()
	line, err := bufio.NewReader(r).ReadBytes('\n')
	if err != nil {
		t.Fatalf("读请求失败: %v", err)
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if json.Unmarshal(line, &req) != nil {
		t.Fatalf("请求行无法解析: %s", line)
	}
	return req.ID
}

// TestFakeProcessEndToEnd 端到端:initialize 握手 → session/new(探针夹具目录)
// → set_config_option(编码串)→ prompt(通知+终局)。
func TestFakeProcessEndToEnd(t *testing.T) {
	c := startFake(t)

	init, err := c.Initialize(nil)
	if err != nil {
		t.Fatalf("initialize 应成功: %v", err)
	}
	if !init.CanResume() || init.SupportsImageInput() {
		t.Fatalf("能力快照不符: %+v", init.AgentCapabilities)
	}

	st, err := c.SessionNew(nil, t.TempDir())
	if err != nil {
		t.Fatalf("session/new 应成功: %v", err)
	}
	if st.SessionID != "fake-sess-1" {
		t.Fatalf("sessionId 不符: %q", st.SessionID)
	}
	cat := ParseModelCatalog(st.ConfigOptions)
	if len(cat.ModelOptions) != 4 || cat.EffortValue != "high" {
		t.Fatalf("探针夹具目录解析不符: %+v", cat)
	}

	// set_config_option:只允许原样编码串。
	if err := c.SetConfigOption(nil, st.SessionID, "model", cat.ModelOptions[1].Value); err != nil {
		t.Fatalf("编码串应被接受: %v", err)
	}
	if err := c.SetConfigOption(nil, st.SessionID, "model", "deepseek-v4-pro"); err == nil {
		t.Fatal("裸模型名必须被拒绝(红线:禁止裸模型名)")
	}

	// prompt:通知到达当轮钩子,应答=终局。
	got := make(chan string, 4)
	c.SetTurnHandlers(func(method string, params json.RawMessage) {
		if method == "session/update" {
			var u struct {
				Update struct {
					SessionUpdate string `json:"sessionUpdate"`
					Content       struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"update"`
			}
			if json.Unmarshal(params, &u) == nil {
				got <- u.Update.SessionUpdate + ":" + u.Update.Content.Text
			}
		}
	}, nil)
	defer c.SetTurnHandlers(nil, nil)
	raw, err := c.Prompt(nil, st.SessionID, []map[string]interface{}{{"type": "text", "text": "1+1"}})
	if err != nil {
		t.Fatalf("prompt 应成功: %v", err)
	}
	var res struct {
		StopReason string `json:"stopReason"`
	}
	if json.Unmarshal(raw, &res) != nil || res.StopReason != "end_turn" {
		t.Fatalf("prompt 终局不符: %s", raw)
	}
	select {
	case u := <-got:
		if u != "agent_message_chunk:2" {
			t.Fatalf("update 通知不符: %q", u)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未收到 session/update 通知")
	}
}

// TestPromptNotBoundByCallTimeout prompt 的应答可能晚于 120s(PromptCtx timeout=0
// 不设兜底,只受 ctx 管)。直接钉 callTimeout 的两条路径:0=无限等,>0=到期超时。
func TestPromptNotBoundByCallTimeout(t *testing.T) {
	c, sOutW, sInR := pipeClient(t)

	done := make(chan error, 1)
	go func() {
		_, err := c.callTimeout(context.Background(), "session/prompt", nil, 0) // 0=不设兜底
		done <- err
	}()
	time.Sleep(150 * time.Millisecond) // 远超常见短超时,验证没有隐藏 deadline
	select {
	case err := <-done:
		t.Fatalf("timeout=0 不应超时返回: %v", err)
	default:
	}
	id := readRequest(t, sInR)
	fmt.Fprintf(sOutW, "{\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":{\"stopReason\":\"end_turn\"}}\n", id)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("timeout=0 应等到应答: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("应答未投递")
	}

	// 对照:短超时会超时(callTimeout>0 路径;stdin 可写但永无应答)。
	c2 := &Client{stdout: bufio.NewReader(strings.NewReader("")), stdin: nopWriteCloser{}, pending: map[int64]chan rpcResp{}}
	if _, err := c2.callTimeout(context.Background(), "m", nil, 30*time.Millisecond); err == nil {
		t.Fatal("短超时应返回超时错误")
	}
}

// nopWriteCloser 可写不可关的 stdin 替身(超时路径单测用)。
type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (nopWriteCloser) Close() error                { return nil }

// TestChildDeathWakesPending 子进程退出(stdout 关闭)必须叫醒所有等待者
// (errClientClosed),上层据此走"崩溃自动重启"路径 —— 重启契约的传输层前提。
func TestChildDeathWakesPending(t *testing.T) {
	c, sOutW, sInR := pipeClient(t)
	go func() { _, _ = io.Copy(io.Discard, sInR) }() // 排空写面,别让请求方卡在管道写入
	errCh := make(chan error, 1)
	go func() {
		_, err := c.Call("session/prompt", nil)
		errCh <- err
	}()
	time.Sleep(50 * time.Millisecond)
	_ = sOutW.Close() // 模拟子进程死亡
	select {
	case err := <-errCh:
		if err == nil || !strings.Contains(err.Error(), "dsh 子进程连接已关闭") {
			t.Fatalf("应报 errClientClosed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("子进程死亡未叫醒 pending")
	}
}

// TestReapWithLogWritesExitCode 死因日志:退出码落标准 log(与 codexapp 同款)。
func TestReapWithLogWritesExitCode(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "exit", "7")
	c := &Client{cmd: cmd, pending: map[int64]chan rpcResp{}}
	if err := cmd.Start(); err != nil {
		t.Skipf("无法拉起测试子进程: %v", err)
	}
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)
	c.reapWithLog("测试收尸")
	if !strings.Contains(buf.String(), "退出码=7") || !strings.Contains(buf.String(), "测试收尸") {
		t.Fatalf("收尸日志必须带退出码与 why: %q", buf.String())
	}
	c.reapWithLog("第二次收尸") // waitOnce:不重复 Wait、不 panic
}

// TestDispatchSnapshotRaceFree handler 快照装卸与 dispatch 并发(-race 回归,
// 审批桥的并发基础:轮内装卸 reqHandler 与 readLoop 分发反向请求同款竞态面)。
func TestDispatchSnapshotRaceFree(t *testing.T) {
	c := &Client{pending: map[int64]chan rpcResp{}}
	c.syncHandlers()
	stop := make(chan struct{})
	done := make(chan struct{})
	id := int64(1)
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			c.dispatch(&wireMsg{Method: "session/request_permission", ID: &id})
			c.dispatch(&wireMsg{Method: "session/update"})
		}
	}()
	for i := 0; i < 1000; i++ {
		c.SetTurnHandlers(func(string, json.RawMessage) {}, func(int64, string, json.RawMessage) {})
		c.SetTurnHandlers(nil, nil)
	}
	close(stop)
	<-done
}

// TestApprovalRespondConcurrent 审批并发:多个反向请求 handler 同时 Respond/
// RespondError,写面必须互斥 —— 每帧都是完整单行 JSON,绝不允许并发交错出坏帧。
func TestApprovalRespondConcurrent(t *testing.T) {
	sInR, sInW := io.Pipe()
	cw := &Client{stdin: sInW, pending: map[int64]chan rpcResp{}}
	t.Cleanup(func() { _ = sInW.Close() })
	collect := make(chan string, 64)
	done := make(chan struct{})
	go func() { // 消费写出的帧
		defer close(done)
		sc := bufio.NewScanner(sInR)
		for sc.Scan() {
			collect <- sc.Text()
		}
	}()
	for i := 0; i < 20; i++ {
		go func(i int) {
			if i%2 == 0 {
				_ = cw.Respond(int64(i), PermissionOutcomeSelected("ok"))
				return
			}
			_ = cw.RespondError(int64(i), -32601, "不支持")
		}(i)
	}
	deadline := time.After(5 * time.Second)
	for n := 0; n < 20; n++ {
		select {
		case line := <-collect:
			var m map[string]interface{}
			if json.Unmarshal([]byte(line), &m) != nil {
				t.Fatalf("帧不是合法 JSON(写面被并发交错破坏): %s", line)
			}
		case <-deadline:
			t.Fatalf("20 个应答只收到 %d", n)
		}
	}
}

// ---- 纯函数/归一辅助 ----

func TestLaunchSpecCmdScript(t *testing.T) {
	exe, args := launchSpec(`C:\node\node.exe`, `C:\npm\dsh.cmd`)
	if exe != "cmd.exe" || strings.Join(args, " ") != "/c C:\\npm\\dsh.cmd --profile acp" {
		t.Fatalf(".cmd 应经 cmd /c: exe=%s args=%v", exe, args)
	}
	exe, args = launchSpec(`C:\node.exe`, `G:\x\bin.js`)
	if exe != `C:\node.exe` || strings.Join(args, " ") != `G:\x\bin.js --profile acp` {
		t.Fatalf("bin.js 应直拉 node: exe=%s args=%v", exe, args)
	}
}

func TestDefaultDshHomeIsolated(t *testing.T) {
	home := defaultDshHome()
	if !strings.Contains(home, ".pocketagentd") || !strings.Contains(home, "dsh-home") {
		t.Fatalf("隔离 HOME 默认值应在 ~/.pocketagentd/dsh-home: %q", home)
	}
}

func TestDrainStderrBasics(t *testing.T) {
	var got []string
	drainStderr(bytes.NewBufferString("line1\n\nhalf-dead"), func(l string) { got = append(got, l) })
	if strings.Join(got, "|") != "line1||half-dead" {
		t.Fatalf("逐行含空行/截断行: %q", got)
	}
}

// TestRPCErrorRoundTrip 应答错误帧 → *RPCError 保真(code/消息不丢,并命中
// MISSING_CREDENTIAL 归一)。
func TestRPCErrorRoundTrip(t *testing.T) {
	c, sOutW, sInR := pipeClient(t)
	errCh := make(chan error, 1)
	go func() {
		_, err := c.Call("boom", nil)
		errCh <- err
	}()
	id := readRequest(t, sInR)
	fmt.Fprintf(sOutW, "{\"jsonrpc\":\"2.0\",\"id\":%d,\"error\":{\"code\":-32603,\"message\":\"no API key for provider route\"}}\n", id)
	select {
	case err := <-errCh:
		var re *RPCError
		if !errors.As(err, &re) {
			t.Fatalf("错误必须保真为 *RPCError: %v (%T)", err, err)
		}
		if re.Code != -32603 || !strings.Contains(re.Message, "no API key") || re.Method != "boom" {
			t.Fatalf("RPCError 字段不符: %+v", re)
		}
		if !IsMissingCredential(err) {
			t.Fatal("该形状应命中 MISSING_CREDENTIAL")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("错误应答未投递")
	}
}
