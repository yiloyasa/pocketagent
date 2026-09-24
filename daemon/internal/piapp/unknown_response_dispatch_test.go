// unknown_response_dispatch_test.go —— AUDIT §3.3「未知 response」分发语义回归钉
// (codexapp 同款;pi 特有:官方应答 id 支持任意 JSON 值而本客户端只发 string,
// 兜底回调必须把原始 id 字节带出去,provider 侧 recordUnknownResponse 才留得住
// correlation —— id 解析失败被静默忽略的历史行为不受影响,一并钉住防误改)。
package piapp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDispatchUnknownResponseFallsBackToHandler(t *testing.T) {
	c := &Client{pending: map[string]chan piResp{}, deathCh: make(chan struct{})}
	got := make(chan unknownProbe, 4)
	c.UnknownResponseHandler = func(rawID json.RawMessage, raw json.RawMessage) {
		got <- unknownProbe{id: string(rawID), raw: string(raw)}
	}
	c.syncHandlers()

	// ① 未匹配(string id):回调带原始 id 字节 + 完整原始行。
	line := `{"type":"response","id":"weird-9","success":true,"data":{"echo":"pi-orphan"}}`
	c.dispatch([]byte(line))
	select {
	case p := <-got:
		if p.id != `"weird-9"` {
			t.Fatalf("id 应按原始字节透传(含引号),得到 %s", p.id)
		}
		if !strings.Contains(p.raw, "pi-orphan") {
			t.Fatalf("应带完整原始行: %s", p.raw)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未匹配的 response 未触发 UnknownResponseHandler(兜底断)")
	}

	// ② 已匹配:原路送达 pending,兜底不得截胡。
	pending := make(chan piResp, 1)
	c.mu.Lock()
	c.pending["ok-1"] = pending
	c.mu.Unlock()
	c.dispatch([]byte(`{"type":"response","id":"ok-1","success":true,"data":{"x":1}}`))
	select {
	case r := <-pending:
		if r.err != nil || string(r.data) != `{"x":1}` {
			t.Fatalf("已匹配 response 应原路送达: data=%s err=%v", r.data, r.err)
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
	//    附带钉住:非 string id 的应答(本客户端只发 string id)继续静默忽略。
	c2 := &Client{pending: map[string]chan piResp{}, deathCh: make(chan struct{})}
	c2.dispatch([]byte(`{"type":"response","id":"lonely","success":true,"data":{}}`))
	c2.dispatch([]byte(`{"type":"response","id":3,"success":true,"data":{}}`))
}

// unknownProbe 兜底回调的捕获样本(避免共享变量跨 goroutine 读写,-race 必报)。
type unknownProbe struct {
	id  string
	raw string
}
