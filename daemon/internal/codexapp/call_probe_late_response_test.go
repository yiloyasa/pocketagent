// call_probe_late_response_test.go —— CallProbeCtx「探测迟到应答静默丢弃」的回归钉。
//
// 【背景】fetchUsageUncached 的两个 account/* 探测超时放弃后,app-server 补发的
// 迟到 error 帧曾每个探测窗在未知事件台账积 24 条(先收编为 KNOWN_PASSTHROUGH,
// 2026-10-01 起源头治理:CallProbeCtx + dispatch 按 id 丢弃)。本文件钉死三条:
//   ① 预算内正常应答仍原路生效,且不误入丢弃通道(能力位语义不变);
//   ② 超时放弃后,同 id 的迟到应答被静默丢弃 —— UnknownResponseHandler 不触发;
//      而**从未被请求**的孤儿帧照旧走兜底留痕 —— 一帧流里完成 A/B 对照;
//   ③ CallCtx(其他探测/普通调用)的迟到应答语义一字不变:放弃后到达仍走兜底。
package codexapp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// newProbeTestClient 造一个不走子进程的最小 Client:stdin 是 io.Pipe(写出的每条
// 请求被解码、id 回传给测试),pending/abandoned 就绪。返回 id 通道由后台 goroutine
// 持续喂数,测试用它拿到"刚写出的请求"的调用代号再 dispatch 应答帧。
func newProbeTestClient(t *testing.T) (*Client, <-chan int64) {
	t.Helper()
	pr, pw := io.Pipe()
	c := &Client{
		stdin:     pw,
		pending:   map[int64]chan rpcResp{},
		abandoned: map[int64]abandonedProbe{},
	}
	ids := make(chan int64, 4)
	go func() {
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			var m struct {
				ID int64 `json:"id"`
			}
			if json.Unmarshal(sc.Bytes(), &m) == nil {
				ids <- m.ID
			}
		}
	}()
	t.Cleanup(func() { _ = pw.Close() })
	return c, ids
}

// ① 预算内正常应答:CallProbeCtx 原路拿到结果,且该 id 绝不进丢弃通道。
func TestCallProbeCtxDeliversResponseWithinBudget(t *testing.T) {
	c, ids := newProbeTestClient(t)

	type out struct {
		raw json.RawMessage
		err error
	}
	done := make(chan out, 1)
	go func() {
		raw, err := c.CallProbeCtx(context.Background(), "account/rateLimits/read", map[string]interface{}{})
		done <- out{raw, err}
	}()

	id := <-ids
	idCopy := id
	c.dispatch(&wireMsg{ID: int64ID(idCopy), Result: json.RawMessage(`{"rateLimits":{"planType":"pro"}}`)})

	select {
	case o := <-done:
		if o.err != nil {
			t.Fatalf("预算内应答必须生效: %v", o.err)
		}
		if !strings.Contains(string(o.raw), "planType") {
			t.Fatalf("应答原文应原样返回: %s", o.raw)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CallProbeCtx 未在预算内收到应答")
	}
	if _, still := c.takeAbandoned(id); still {
		t.Fatal("预算内完成的调用不得被记入丢弃通道")
	}
}

// ② 超时放弃后:同 id 迟到应答静默丢弃(兜底不触发);从未被请求的孤儿帧照旧
// 走兜底(对照,证明丢弃是按调用代号精确命中,不是全局关闸)。
func TestCallProbeCtxDropsLateResponseAfterGiveUp(t *testing.T) {
	c, ids := newProbeTestClient(t)
	fired := make(chan unknownProbe, 4)
	c.UnknownResponseHandler = func(id int64, raw json.RawMessage) {
		fired <- unknownProbe{id: id, raw: string(raw)}
	}
	c.syncHandlers()

	errCh := make(chan error, 1)
	go func() {
		_, err := c.CallProbeCtx(fastTimeoutCtx(t), "account/usage/read", map[string]interface{}{})
		errCh <- err
	}()
	id := <-ids

	// 调用方按预算放弃(50ms),迟到应答 150ms 后才到。
	select {
	case err := <-errCh:
		if !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Fatalf("应按 ctx 预算放弃: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CallProbeCtx 未按预算返回")
	}

	// a) 迟到的真 id 应答:静默丢弃,UnknownResponseHandler 不得触发。
	idCopy := id
	c.dispatch(&wireMsg{ID: int64ID(idCopy), Result: json.RawMessage(`{"late":"upstream error"}`)})
	select {
	case p := <-fired:
		t.Fatalf("已放弃探测的迟到应答不得触发兜底: %+v", p)
	case <-time.After(100 * time.Millisecond):
	}

	// b) 对照:从未被请求的孤儿帧仍走兜底留痕(台账管道对真未知保持睁眼)。
	bogus := int64(424242)
	c.dispatch(&wireMsg{ID: int64ID(bogus), Result: json.RawMessage(`{"orphan":true}`)})
	select {
	case p := <-fired:
		if p.id != bogus {
			t.Fatalf("对照孤儿帧应带原 id %d,得到 %d", bogus, p.id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("孤儿帧未走兜底(对照失败:丢弃通道不该挡住真未知)")
	}
}

// ③ 普通调用(CallCtx,其他探测全走这条)语义不变:超时放弃后迟到应答仍进兜底。
func TestCallCtxLateResponseStillFallsBackToHandler(t *testing.T) {
	c, ids := newProbeTestClient(t)
	fired := make(chan unknownProbe, 4)
	c.UnknownResponseHandler = func(id int64, raw json.RawMessage) {
		fired <- unknownProbe{id: id, raw: string(raw)}
	}
	c.syncHandlers()

	errCh := make(chan error, 1)
	go func() {
		_, err := c.CallCtx(fastTimeoutCtx(t), "mcpServerStatus/list", map[string]interface{}{})
		errCh <- err
	}()
	id := <-ids

	select {
	case err := <-errCh:
		if !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Fatalf("应按 ctx 预算放弃: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CallCtx 未按预算返回")
	}

	idCopy := id
	c.dispatch(&wireMsg{ID: int64ID(idCopy), Result: json.RawMessage(`{"late":true}`)})
	select {
	case p := <-fired:
		if p.id != id {
			t.Fatalf("CallCtx 迟到应答应带原 id 入兜底: want %d got %d", id, p.id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CallCtx 的迟到应答必须仍走兜底留痕(其他探测语义不得被改)")
	}
}

// fastTimeoutCtx 毫秒级探测预算(测试只钉"放弃 → 迟到"时序,不测预算数值)。
func fastTimeoutCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	t.Cleanup(cancel)
	return ctx
}
