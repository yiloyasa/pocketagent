// string_id_dispatch_test.go —— wireMsg.ID 双形态(RawMessage 化,2026-10-04)的回归钉。
//
// 【钉什么】JSON-RPC 2.0 允许 number|string id,旧 *int64 一遇字符串 id 整帧
// 解码失败、readLoop 静默丢帧。三条路各钉一条:
//   - 字符串 id 的应答帧 → 不可能命中 pending,但必须进 unknownResponse 台账
//     (绝不静默丢);
//   - 字符串 id 的反向请求 → handler 拿负数句柄,Respond 应答时原样回显字符串 id;
//   - 数字 id 的反向请求 → 行为与旧版逐字节一致(句柄即原值)。
// 另钉 writeWithBudget:stdin 卡死时写按预算超时返回,不再无限期阻塞调用方。
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
		serverIDs: map[int64]serverIDEntry{},
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

// 字符串 id 的应答帧:进 unknownResponse 台账,id 参数按 int64 签名传 0 占位,
// 完整原始帧里保留真实 id —— 绝不静默丢。
func TestDispatchStringIDResponseGoesToLedger(t *testing.T) {
	c := &Client{pending: map[int64]chan rpcResp{}}
	got := make(chan unknownProbe, 4)
	c.UnknownResponseHandler = func(id int64, raw json.RawMessage) {
		got <- unknownProbe{id: id, raw: string(raw)}
	}
	c.syncHandlers()

	c.dispatch(&wireMsg{ID: json.RawMessage(`"req-1"`), Result: json.RawMessage(`{"late":true}`)})
	select {
	case p := <-got:
		if p.id != 0 {
			t.Fatalf("非数字应答 id 应以 0 占位(int64 签名限制),得到 %d", p.id)
		}
		if !strings.Contains(p.raw, `"id":"req-1"`) || !strings.Contains(p.raw, "late") {
			t.Fatalf("台账必须保留完整原始帧(真实 id 在里面): %s", p.raw)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("字符串 id 应答被静默丢帧(双形态修复失效)")
	}
}

// 字符串 id 的反向请求:handler 拿负数句柄;Respond 查表原样回显字符串 id;
// 句柄一次应答即除名,重复应答得到错误、不重复回写。
func TestDispatchStringIDServerRequestEchoesOriginalID(t *testing.T) {
	c, lines := newStringIDTestClient(t)
	handles := make(chan int64, 1)
	c.ReqHandler = func(id int64, method string, _ json.RawMessage) {
		handles <- id
	}
	c.syncHandlers()

	c.dispatch(&wireMsg{Method: "applyPatch/approval", ID: json.RawMessage(`"srv-9"`)})
	var h int64
	select {
	case h = <-handles:
		if h >= 0 {
			t.Fatalf("非数字 id 应换负数句柄(与正数 id 空间隔离),得到 %d", h)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("字符串 id 反向请求未送达 ReqHandler")
	}

	if err := c.Respond(h, map[string]string{"decision": "accept"}); err != nil {
		t.Fatalf("句柄应答不应报错: %v", err)
	}
	select {
	case line := <-lines:
		if !strings.Contains(line, `"id":"srv-9"`) || !strings.Contains(line, "accept") {
			t.Fatalf("应答必须原样回显字符串 id: %s", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未读到应答帧")
	}

	// 同句柄第二次应答:查表落空,报错且不再回写(与上层 resolved 守卫双保险)。
	if err := c.Respond(h, map[string]string{"decision": "accept"}); err == nil {
		t.Fatal("句柄已除名,重复应答应报错")
	}
	select {
	case line := <-lines:
		t.Fatalf("重复应答不得回写: %s", line)
	case <-time.After(100 * time.Millisecond):
	}
}

// 数字 id 的反向请求:句柄即原值,应答回显数字 —— 与旧版(*int64 时代)逐字节一致。
func TestRespondNumericIDEchoesVerbatim(t *testing.T) {
	c, lines := newStringIDTestClient(t)
	if err := c.Respond(7, map[string]string{"decision": "accept"}); err != nil {
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

// stdin 卡死(没人读管道):writeWithBudget 按预算返回错误,调用方不再无限期阻塞。
func TestWriteWithBudgetTimesOut(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pw.Close(); _ = pr.Close() }) // 关管道,放行卡在 Write 上的 worker
	c := &Client{stdin: pw}

	start := time.Now()
	err := c.writeWithBudget([]byte("{\"jsonrpc\":\"2.0\"}\n"), 100*time.Millisecond)
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
	if err := c.writeWithBudget([]byte("{\"ping\":1}\n"), writeBudget); err != nil {
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
