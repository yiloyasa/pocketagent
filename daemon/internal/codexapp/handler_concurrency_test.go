package codexapp

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

// Run installs/uninstalls callbacks while settings/goal requests call CallCtx.
// An atomic dispatch snapshot alone does not protect its publication reads.
func TestCallCtxConcurrentHandlerPublication(t *testing.T) {
	c := &Client{stdin: errWriter{}, pending: map[int64]chan rpcResp{}}
	install := func(on bool) {
		if on {
			c.SetGlobalHandler(func(string, json.RawMessage) {})
			c.SetTurnHandlers(func(string, json.RawMessage) {}, func(json.RawMessage, string, json.RawMessage) {})
		} else {
			c.SetGlobalHandler(nil)
			c.SetTurnHandlers(nil, nil)
		}
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for worker := 0; worker < 3; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for i := 0; i < 1000; i++ {
				switch worker {
				case 0:
					install(i%2 == 0)
				case 1:
					if _, err := c.CallCtx(context.Background(), "thread/goal/get", nil); err == nil {
						t.Error("expected failed test pipe")
					}
				case 2:
					c.dispatch(&wireMsg{Method: "future/model/notification", Params: json.RawMessage(`{"custom":true}`)})
				}
			}
		}(worker)
	}
	close(start)
	wg.Wait()
}

func TestHandlerSettersPublishImmediatelyAndPreserveGlobal(t *testing.T) {
	c := &Client{}
	const method = "future/custom-model/notification"
	raw := json.RawMessage(`{"model":"custom","unknown":{"value":7}}`)
	var global, turn int
	c.SetGlobalHandler(func(m string, p json.RawMessage) {
		global++
		if m != method || string(p) != string(raw) {
			t.Error("unknown notification was changed")
		}
	})
	c.SetTurnHandlers(func(string, json.RawMessage) {
		turn++
		// Callback invocation must be outside handlerMu: uninstall can reenter.
		c.SetTurnHandlers(nil, nil)
	}, func(json.RawMessage, string, json.RawMessage) {})
	c.dispatch(&wireMsg{Method: method, Params: raw})
	c.dispatch(&wireMsg{Method: method, Params: raw})
	if global != 2 || turn != 1 {
		t.Fatalf("immediate publication/uninstall: global=%d turn=%d", global, turn)
	}
	if hs := c.hs.Load(); hs.req != nil || hs.notif != nil || hs.global == nil {
		t.Fatal("turn uninstall did not preserve global handler")
	}
}
