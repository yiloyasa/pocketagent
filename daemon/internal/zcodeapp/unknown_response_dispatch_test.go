// unknown_response_dispatch_test.go —— AUDIT §3.3「未知 response」分发语义回归钉
// (codexapp 同款;另钉一条 zcode 特有约束:id 必须按原始字节透传给兜底回调,
// 首启/重启两条接线路径都靠它把 correlation 原样交给 recordUnknownResponse)。
package zcodeapp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDispatchUnknownResponseFallsBackToHandler(t *testing.T) {
	c, _ := newTestClient()
	got := make(chan unknownProbe, 4)
	c.UnknownResponseHandler = func(rawID json.RawMessage, raw json.RawMessage) {
		got <- unknownProbe{id: string(rawID), raw: string(raw)}
	}
	c.SyncHandlers()

	// ① 未匹配:回调带原始 id 字节(不重排版)+ 完整帧。
	c.dispatch(&wireMsg{ID: json.RawMessage(`77`), Result: json.RawMessage(`{"echo":"zcode-orphan"}`)})
	select {
	case p := <-got:
		if p.id != `77` {
			t.Fatalf("id 应按原始字节透传,得到 %s", p.id)
		}
		if !strings.Contains(p.raw, "zcode-orphan") || !strings.Contains(p.raw, `"id":77`) {
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
	c.dispatch(&wireMsg{ID: json.RawMessage(`5`), Result: json.RawMessage(`{"ok":true}`)})
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
	c2 := &Client{stdin: &syncBuffer{}, pending: map[int64]chan rpcResp{}}
	c2.dispatch(&wireMsg{ID: json.RawMessage(`7`), Result: json.RawMessage(`{}`)})
}

// unknownProbe 兜底回调的捕获样本(避免共享变量跨 goroutine 读写,-race 必报)。
type unknownProbe struct {
	id  string
	raw string
}
