// string_id_dispatch_test.go —— wireMsg.ID 双形态(RawMessage 化,2026-10-04)的回归钉。
//
// 【钉什么】JSON-RPC 2.0 允许 number|string id,旧 *int64 一遇字符串 id 整帧
// 解码失败、readLoop 静默丢帧。三条路各钉一条:
//   - 字符串 id 的应答帧 → 不可能命中 pending,但必须进 unknownResponse 台账
//     (绝不静默丢),且 id 原样透传(RawMessage 签名后不再 0 占位);
//   - 字符串 id 的反向请求 → handler 拿到原文 id,Respond 应答时原样回显字符串 id
//     (2026-10-04 起直透:负数句柄桥已随签名 RawMessage 化整体删除);
//   - 数字 id 的反向请求/应答 → 行为与旧版逐字节一致(id 原样回显数字)。
//
// 另钉写预算(rpcwire.Wire 收编后):stdin 卡死时写按预算超时返回,不再无限期阻塞调用方。
package codexapp

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// newStringIDTestClient 造一个不走子进程的最小 Client:stdin 是 io.Pipe,写出的
// 每一行(完整协议帧)被送进 lines 通道供测试断言。
func newStringIDTestClient(t *testing.T) (*Client, <-chan string) {
	t.Helper()
	pr, pw := io.Pipe()
	c := &Client{
		stdin:     pw,
		pending:   map[int64]chan rpcResp{},
		abandoned: map[int64]abandonedProbe{},
	}
	lines := make(chan string, 4)
	go func() {
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	t.Cleanup(func() { _ = pw.Close() })
	return c, lines
}

// 字符串 id 的应答帧:进 unknownResponse 台账,id 原样透传(完整原始帧同在,
// 双保险可还原)—— 绝不静默丢。
func TestDispatchStringIDResponseGoesToLedger(t *testing.T) {
	c := &Client{pending: map[int64]chan rpcResp{}}
	got := make(chan unknownProbe, 4)
	c.UnknownResponseHandler = func(id json.RawMessage, raw json.RawMessage) {
		got <- unknownProbe{id: id, raw: string(raw)}
	}
	c.syncHandlers()

	c.dispatch(&wireMsg{ID: json.RawMessage(`"req-1"`), Result: json.RawMessage(`{"late":true}`)})
	select {
	case p := <-got:
		if string(p.id) != `"req-1"` {
			t.Fatalf("非数字应答 id 应原样透传(RawMessage 签名),得到 %s", p.id)
		}
		if !strings.Contains(p.raw, `"id":"req-1"`) || !strings.Contains(p.raw, "late") {
			t.Fatalf("台账必须保留完整原始帧(真实 id 在里面): %s", p.raw)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("字符串 id 应答被静默丢帧(双形态修复失效)")
	}
}

// 字符串 id 的反向请求:handler 拿到的 id 与线上原文逐字节一致;Respond 原样
// 回显字符串 id —— 中间无句柄、无登记,不查表、不出错。
func TestDispatchStringIDServerRequestEchoesOriginalID(t *testing.T) {
	c, lines := newStringIDTestClient(t)
	got := make(chan json.RawMessage, 1)
	c.ReqHandler = func(id json.RawMessage, method string, _ json.RawMessage) {
		got <- id
	}
	c.syncHandlers()

	c.dispatch(&wireMsg{Method: "applyPatch/approval", ID: json.RawMessage(`"srv-9"`)})
	var h json.RawMessage
	select {
	case h = <-got:
		if string(h) != `"srv-9"` {
			t.Fatalf("handler 应拿到线上原样 id,得到 %s", h)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("字符串 id 反向请求未送达 ReqHandler")
	}

	if err := c.Respond(h, map[string]string{"decision": "accept"}); err != nil {
		t.Fatalf("原样 id 应答不应报错: %v", err)
	}
	select {
	case line := <-lines:
		if !strings.Contains(line, `"id":"srv-9"`) || !strings.Contains(line, "accept") {
			t.Fatalf("应答必须原样回显字符串 id: %s", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未读到应答帧")
	}

	// RespondError 同样原样回显(直透对两种应答面一致)。
	if err := c.RespondError(h, -32601, "不支持"); err != nil {
		t.Fatalf("RespondError 原样 id 应答不应报错: %v", err)
	}
	select {
	case line := <-lines:
		if !strings.Contains(line, `"id":"srv-9"`) || !strings.Contains(line, "-32601") {
			t.Fatalf("RespondError 必须原样回显字符串 id: %s", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未读到 RespondError 应答帧")
	}
}

// 数字 id 的反向请求/应答:id 原样回显数字 —— 与旧版(*int64 时代)逐字节一致。
func TestRespondNumericIDEchoesVerbatim(t *testing.T) {
	c, lines := newStringIDTestClient(t)
	if err := c.Respond(json.RawMessage("7"), map[string]string{"decision": "accept"}); err != nil {
		t.Fatalf("数字 id 应答不应报错: %v", err)
	}
	select {
	case line := <-lines:
		if !strings.Contains(line, `"id":7`) {
			t.Fatalf("数字 id 应答形状必须与旧版一致(回数字): %s", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未读到应答帧")
	}
}

// 缺失/null 的 id 不可回应:Respond 拒绝回写,绝不向服务器发出 "id":null 的
// 协议残帧(id:null 按 JSON-RPC 2.0 等价于无 id,应答无从配对)。
func TestRespondRejectsAbsentID(t *testing.T) {
	c, lines := newStringIDTestClient(t)
	if err := c.Respond(nil, map[string]string{"decision": "accept"}); err == nil {
		t.Fatal("nil id 应拒绝回写")
	}
	if err := c.RespondError(json.RawMessage("null"), -32601, "不支持"); err == nil {
		t.Fatal("null id 应拒绝回写")
	}
	select {
	case line := <-lines:
		t.Fatalf("拒绝路径不得有任何回写: %s", line)
	case <-time.After(100 * time.Millisecond):
	}
}

// stdin 卡死(没人读管道):写预算机制(rpcwire.Wire 收编后)按预算超时返回,
// 调用方不再无限期阻塞。
func TestWriteWithBudgetTimesOut(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pw.Close(); _ = pr.Close() }) // 关管道,放行卡在 Write 上的 worker
	c := &Client{stdin: pw}

	start := time.Now()
	err := c.wire.Write(c.stdin, []byte("{\"jsonrpc\":\"2.0\"}\n"), 100*time.Millisecond)
	if err == nil {
		t.Fatal("无人读管道,写必须超时报错")
	}
	if !strings.Contains(err.Error(), "100ms") {
		t.Fatalf("错误应说明预算: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("超时返回拖了 %v,调用方又被无限期阻塞了", elapsed)
	}
}

// 正常路径:预算内写完返回 nil,内容逐字节落管(含换行)。
func TestWriteWithBudgetSuccess(t *testing.T) {
	c, lines := newStringIDTestClient(t)
	if err := c.wire.Write(c.stdin, []byte("{\"ping\":1}\n"), writeBudget); err != nil {
		t.Fatalf("预算内写不应报错: %v", err)
	}
	select {
	case line := <-lines:
		if line != `{"ping":1}` {
			t.Fatalf("写入内容应逐字节落管: %s", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未读到写入帧")
	}
}
