// unknown_response_dispatch_test.go —— AUDIT §3.3「未知 response」分发语义回归钉。
//
// 【钉什么】W8(d82d915)给 dispatch 加的兜底:response 的 id 对不上任何 pending
// 请求时,交给 UnknownResponseHandler 留痕(provider 侧接 recordUnknownResponse),
// 而不是静默丢弃。三种情形各有失效方式,分开断言:
//   - 未匹配 → 必须回调,带原 id 与完整原始帧(丢了就查不了"上游答了什么");
//   - 已匹配 → 绝不误触兜底(响应必须原路送达调用方,不得被兜底截胡);
//   - 没装 handler → 静默忽略,不得 panic(readLoop 里 panic 会带走整条管道)。
package codexapp

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
	c.syncHandlers()

	// ① 未匹配:回调带原 id + 完整帧。
	c.dispatch(&wireMsg{ID: int64Ptr(99), Result: json.RawMessage(`{"answer":"orphan"}`)})
	select {
	case p := <-got:
		if p.id != 99 {
			t.Fatalf("应带原 id,得到 %d", p.id)
		}
		if !strings.Contains(p.raw, "orphan") || !strings.Contains(p.raw, `"id":99`) {
			t.Fatalf("应带完整原始帧(留痕要能还原上游答了什么): %s", p.raw)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未匹配的 response 未触发 UnknownResponseHandler(兜底断)")
	}

	// ② 已匹配:原路送达 pending,兜底不得截胡。
	pending := make(chan rpcResp, 1)
	c.mu.Lock()
	c.pending[5] = pending
	c.mu.Unlock()
	c.dispatch(&wireMsg{ID: int64Ptr(5), Result: json.RawMessage(`{"ok":true}`)})
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

	// ③ 没装 handler:静默忽略,不得 panic(readLoop 里的 panic 带走整条管道)。
	c2 := &Client{pending: map[int64]chan rpcResp{}}
	c2.dispatch(&wireMsg{ID: int64Ptr(7), Result: json.RawMessage(`{}`)})
}

// unknownProbe 兜底回调的捕获样本(避免直接共享变量跨 goroutine 读写,-race 必报)。
type unknownProbe struct {
	id  int64
	raw string
}
