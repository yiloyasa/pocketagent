// unknown_response_dispatch_test.go —— AUDIT §3.3「未知 response」分发语义回归钉
// (codexapp 同款三断言;dsh 的接口断言容错挂接依赖这里的兜底存在,断了它,
// provider 侧 recordUnknownResponse 就收不到任何东西)。
package dshapp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDispatchUnknownResponseFallsBackToHandler(t *testing.T) {
	c := &Client{pending: map[int64]chan rpcResp{}}
	got := make(chan unknownProbe, 4)
	c.UnknownResponseHandler = func(id int64, raw json.RawMessage) {
		got <- unknownProbe{id: id, raw: string(raw)}
	}
	c.SyncHandlers()

	// ① 未匹配:回调带原 id + 完整帧。
	c.dispatch(&wireMsg{ID: dshInt64Ptr(99), Result: json.RawMessage(`{"answer":"orphan"}`)})
	select {
	case p := <-got:
		if p.id != 99 {
			t.Fatalf("应带原 id,得到 %d", p.id)
		}
		if !strings.Contains(p.raw, "orphan") || !strings.Contains(p.raw, `"id":99`) {
			t.Fatalf("应带完整原始帧: %s", p.raw)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未匹配的 response 未触发 UnknownResponseHandler(兜底断)")
	}

	// ② 已匹配:原路送达 pending,兜底不得截胡。
	pending := make(chan rpcResp, 1)
	c.mu.Lock()
	c.pending[5] = pending
	c.mu.Unlock()
	c.dispatch(&wireMsg{ID: dshInt64Ptr(5), Result: json.RawMessage(`{"ok":true}`)})
	select {
	case r := <-pending:
		if r.err != nil || string(r.raw) != `{"ok":true}` {
			t.Fatalf("已匹配 response 应原路送达: raw=%s err=%v", r.raw, r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("已匹配 response 未送达 pending")
	}
	select {
	case p := <-got:
		t.Fatalf("已匹配 response 不得触发兜底: %+v", p)
	case <-time.After(100 * time.Millisecond):
	}

	// ③ 没装 handler:静默忽略,不得 panic。
	c2 := &Client{pending: map[int64]chan rpcResp{}}
	c2.dispatch(&wireMsg{ID: dshInt64Ptr(7), Result: json.RawMessage(`{}`)})
}

func dshInt64Ptr(v int64) *int64 { return &v }

// unknownProbe 兜底回调的捕获样本(避免共享变量跨 goroutine 读写,-race 必报)。
type unknownProbe struct {
	id  int64
	raw string
}
