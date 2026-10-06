package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// 空闲看门狗:流开始后服务端装死,须在阈值+tick 量级内干净报"停摆",
// 而不是无限等(旧实现 Client.Timeout=180s 只能兜墙钟,长轮反而被误杀)。
func TestOpenAICompatIdleWatchdogAbortsSilentStream(t *testing.T) {
	oldIdle, oldTick := oaIdleTimeout, oaWatchdogTick
	oaIdleTimeout, oaWatchdogTick = 300*time.Millisecond, 50*time.Millisecond
	defer func() { oaIdleTimeout, oaWatchdogTick = oldIdle, oldTick }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		// 先给一行合法 SSE 证明"流开始了",然后装死等客户端取消。
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
		fl.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	p := NewOpenAICompatible("glm", "GLM", srv.URL, "sk-test", nil)
	start := time.Now()
	_, _, _, _, err := p.streamOnce(context.Background(), "glm-5",
		[]oaMessage{{Role: "user", Content: "hi"}}, nil,
		func(*pb.TurnServerMessage) {})
	if err == nil {
		t.Fatalf("静默流应报错,实际 nil")
	}
	if !strings.Contains(err.Error(), "停摆") {
		t.Fatalf("应为停摆错误,实际: %v", err)
	}
	if el := time.Since(start); el > 5*time.Second {
		t.Fatalf("看门狗应秒级触发,实际耗时 %v", el)
	}
}

// 慢但活着:每个字节间隔都在阈值内,持续时长超过阈值也不得误杀。
func TestOpenAICompatIdleWatchdogToleratesSlowStream(t *testing.T) {
	oldIdle, oldTick := oaIdleTimeout, oaWatchdogTick
	oaIdleTimeout, oaWatchdogTick = 400*time.Millisecond, 50*time.Millisecond
	defer func() { oaIdleTimeout, oaWatchdogTick = oldIdle, oldTick }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		for i := 0; i < 8; i++ { // 8×100ms=800ms 总时长 > 阈值,靠逐行续命
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n"))
			fl.Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}))
	defer srv.Close()

	var deltas int
	p := NewOpenAICompatible("glm", "GLM", srv.URL, "sk-test", nil)
	_, _, _, _, err := p.streamOnce(context.Background(), "glm-5",
		[]oaMessage{{Role: "user", Content: "hi"}}, nil,
		func(m *pb.TurnServerMessage) {
			if m.GetTextDelta() != nil {
				deltas++
			}
		})
	if err != nil {
		t.Fatalf("慢流不应被误杀: %v", err)
	}
	if deltas != 8 {
		t.Fatalf("应收到 8 个增量,实际 %d", deltas)
	}
}
