package codexapp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"
)

func TestReadLoopReportsClientClosedToPendingCall(t *testing.T) {
	reader, writer := io.Pipe()
	c := &Client{
		stdout:  bufio.NewReader(reader),
		pending: map[int64]chan rpcResp{},
	}
	pending := make(chan rpcResp, 1)
	c.pending[7] = pending

	done := make(chan struct{})
	go func() {
		c.readLoop()
		close(done)
	}()

	_ = writer.Close()

	select {
	case resp := <-pending:
		if resp.err != errClientClosed {
			t.Fatalf("expected errClientClosed, got %v", resp.err)
		}
	case <-time.After(time.Second):
		t.Fatal("readLoop did not wake pending call after stdout closed")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("readLoop did not exit after stdout closed")
	}
}

// ---- 2026-09-19 质量巡逻 P1 回归:handler 原子快照(与 piapp.Client 同款)----

// dispatch 的读面是原子快照:装上并发布则收到;卸下并重新发布后不再收到。
// 未发布快照(hs==nil)时按"无 handler"处理 —— Start 必然先发布,正常路径不会出现。
func TestDispatchSnapshotFunnel(t *testing.T) {
	c := &Client{pending: map[int64]chan rpcResp{}}

	var global, notif int
	c.GlobalHandler = func(string, json.RawMessage) { global++ }
	c.NotifHandler = func(string, json.RawMessage) { notif++ }
	c.syncHandlers()
	c.dispatch(&wireMsg{Method: "thread/status/changed"})
	if global != 1 || notif != 1 {
		t.Fatalf("装上后应各收 1 条: global=%d notif=%d", global, notif)
	}
	c.GlobalHandler, c.NotifHandler = nil, nil
	c.syncHandlers()
	c.dispatch(&wireMsg{Method: "thread/status/changed"})
	if global != 1 || notif != 1 {
		t.Fatalf("卸下并重发快照后不应再收: global=%d notif=%d", global, notif)
	}
}

// CallCtx 的同步点:哪怕命令发送失败,快照也必须已发布 —— dispatch 能看到新装 handler。
func TestCallCtxRefreshesHandlerSnapshot(t *testing.T) {
	c := &Client{stdin: errWriter{}, pending: map[int64]chan rpcResp{}}
	got := make(chan string, 1)
	c.GlobalHandler = func(method string, _ json.RawMessage) { got <- method }
	// 不手动 syncHandlers:Call 内部走「同步点 → writeLine(败)」
	if _, err := c.Call("probe", nil); err == nil {
		t.Fatal("errWriter 应使命令发送失败")
	}
	c.dispatch(&wireMsg{Method: "thread/archived"})
	select {
	case m := <-got:
		if m != "thread/archived" {
			t.Fatalf("方法不符: %s", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CallCtx 同步点未发布快照:dispatch 未看到新 handler")
	}
}

// dispatch 与装卸 handler 并发(-race 回归):一个 goroutine 连续 dispatch(读快照),
// 本 goroutine 反复装卸裸字段 + 同步点发布。快照发布后不可变,必须全程无 data race。
func TestDispatchSnapshotRaceFree(t *testing.T) {
	c := &Client{pending: map[int64]chan rpcResp{}}
	c.syncHandlers()

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() { // 充当 readLoop:持续 dispatch(读快照)
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			c.dispatch(&wireMsg{Method: "thread/status/changed"})
		}
	}()
	for i := 0; i < 2000; i++ { // 充当 provider:装卸裸字段 + CallCtx 同步点
		c.NotifHandler = func(string, json.RawMessage) {}
		c.syncHandlers()
		c.NotifHandler = nil
		c.syncHandlers()
	}
	close(stop)
	<-done
}

// 反向请求同样走快照(ReqHandler 在轮内装卸,与 readLoop 并发)。
func TestDispatchReverseRequestViaSnapshot(t *testing.T) {
	c := &Client{pending: map[int64]chan rpcResp{}}
	ids := make(chan json.RawMessage, 1)
	c.ReqHandler = func(id json.RawMessage, _ string, _ json.RawMessage) { ids <- id }
	c.syncHandlers()
	c.dispatch(&wireMsg{Method: "serverRequest", ID: int64ID(7)})
	select {
	case id := <-ids:
		if string(id) != "7" {
			t.Fatalf("id 不符: %s", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("反向请求未送达快照中的 ReqHandler")
	}
	// 未发布快照(hs==nil):静默忽略,不得 panic
	c2 := &Client{pending: map[int64]chan rpcResp{}}
	c2.dispatch(&wireMsg{Method: "serverRequest", ID: int64ID(8)})
	c2.dispatch(&wireMsg{Method: "thread/archived"})
}

// int64ID 测试造"数字 id"帧:wireMsg.ID 是 RawMessage(见 wireMsg 注释),
// 数字 id 以十进制字面形态放入,json.Marshal 回原始帧时仍是 `"id":7` 形状。
func int64ID(v int64) json.RawMessage { return json.RawMessage(strconv.FormatInt(v, 10)) }

// errWriter 模拟已断的 stdin。
type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, fmt.Errorf("管道已断") }
func (errWriter) Close() error                { return nil }
