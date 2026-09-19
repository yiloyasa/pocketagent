package piapp

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestCallCtxConcurrentHandlerPublication(t *testing.T) {
	c := &Client{stdin: errWriter{}, pending: map[string]chan piResp{}, deathCh: make(chan struct{})}
	install := func(on bool) {
		if on {
			c.SetGlobalHandler(func(Event) {})
			c.SetEventHandler(func(Event) {})
			c.SetUIRequestHandler(func(*UIRequest) (map[string]any, bool) { return nil, false })
		} else {
			c.SetGlobalHandler(nil)
			c.SetEventHandler(nil)
			c.SetUIRequestHandler(nil)
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
					if _, err := c.CallCtx(context.Background(), "get_state", nil, time.Second); err == nil {
						t.Error("expected failed test pipe")
					}
				case 2:
					c.emit(Event{Type: "future_model_notification"})
				}
			}
		}(worker)
	}
	close(start)
	wg.Wait()
}

func TestHandlerSettersPublishImmediatelyAndPreserveGlobal(t *testing.T) {
	c := &Client{}
	raw := []byte(`{"type":"future_custom_model","unknown":{"value":7}}`)
	var global, turn int
	c.SetGlobalHandler(func(ev Event) {
		global++
		if ev.Type != "future_custom_model" || string(ev.Raw) != string(raw) {
			t.Error("unknown event was changed")
		}
	})
	c.SetUIRequestHandler(func(*UIRequest) (map[string]any, bool) { return nil, false })
	c.SetEventHandler(func(Event) {
		turn++
		// Callback invocation must be outside handlerMu.
		c.SetEventHandler(nil)
	})
	c.emit(Event{Type: "future_custom_model", Raw: raw})
	c.emit(Event{Type: "future_custom_model", Raw: raw})
	if global != 2 || turn != 1 {
		t.Fatalf("immediate publication/uninstall: global=%d turn=%d", global, turn)
	}
	if hs := c.hs.Load(); hs.ev != nil || hs.global == nil || hs.ui == nil {
		t.Fatal("event uninstall changed independent handlers")
	}
	c.SetUIRequestHandler(nil)
	if c.hs.Load().ui != nil {
		t.Fatal("UI handler uninstall needs another CallCtx")
	}
}
