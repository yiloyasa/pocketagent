// cancel_test.go —— session/cancel 通知的行为契约(H1 看门狗的数据面):
//   - 帧形:{"jsonrpc":"2.0","method":"session/cancel","params":{"sessionId":…}}
//     —— **通知**(无 id、无应答帧,ACP router.py route_notification 实证);
//   - 超时安全:适配器挂死不读管道时,取消写入按 ctx 放弃,不悬挂轮次收尾;
//   - 端到端:假子进程收到 cancel 后回显一帧 current_mode_update(活的证据)。
package hermesapp

import (
	"bufio"
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestCancelSessionFrameShape 出帧形状:无 id 的通知帧,params 只带 sessionId。
func TestCancelSessionFrameShape(t *testing.T) {
	c, _, sIn := pipeClient(t)
	lines := make(chan []byte, 4)
	go func() {
		defer close(lines)
		rd := bufio.NewReader(sIn)
		for {
			line, err := rd.ReadBytes('\n')
			if err != nil {
				return
			}
			lines <- line
		}
	}()
	if err := c.CancelSession(context.Background(), "sess-9"); err != nil {
		t.Fatalf("CancelSession 应成功: %v", err)
	}
	select {
	case line := <-lines:
		var raw map[string]any
		if json.Unmarshal(line, &raw) != nil {
			t.Fatalf("不是合法 JSON 帧: %s", line)
		}
		if _, hasID := raw["id"]; hasID {
			t.Fatalf("session/cancel 是通知,不得带 id: %s", line)
		}
		if raw["method"] != "session/cancel" {
			t.Fatalf("method 不符: %s", line)
		}
		params, _ := raw["params"].(map[string]any)
		if params == nil || params["sessionId"] != "sess-9" {
			t.Fatalf("params 不符: %s", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("2s 内取消帧未写出")
	}
}

// TestCancelSessionTimeoutSafe 适配器不读管道(io.Pipe 写阻塞)时,取消按 ctx
// 放弃返回,绝不悬挂 —— "点停止"不能反过来被取消帧卡死。
func TestCancelSessionTimeoutSafe(t *testing.T) {
	c, _, sIn := pipeClient(t)
	_ = sIn // 故意不读:写面阻塞
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := c.CancelSession(ctx, "sess-9")
	if err == nil {
		t.Fatal("写不出去应报错(放弃)")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("取消写入未被 ctx 及时打断: %v", elapsed)
	}
}

// TestFakeProcessCancelRoundtrip 端到端:cancel 通知送达假适配器 → 适配器回显
// current_mode_update(cancelled-echo)= 帧真的发到了对方手里。
func TestFakeProcessCancelRoundtrip(t *testing.T) {
	c := startFake(t)
	st, err := c.SessionNew(nil, `C:\ws`)
	if err != nil {
		t.Fatalf("session/new: %v", err)
	}
	ch := collectUpdates(t, c)
	if err := c.CancelSession(nil, st.SessionID); err != nil {
		t.Fatalf("CancelSession 应成功: %v", err)
	}
	waitUpdate(t, ch, "current_mode_update")
}

// TestCancelSessionNilCtx nil ctx 合法(CancelSession 自身无内部超时,退化为
// 无超时;与传输层 CallCtx 的 120s 兜底是两回事 —— 那只罩短控制调用)。
func TestCancelSessionNilCtx(t *testing.T) {
	c, _, sIn := pipeClient(t)
	lines := make(chan []byte, 4)
	go func() {
		defer close(lines)
		rd := bufio.NewReader(sIn)
		for {
			line, err := rd.ReadBytes('\n')
			if err != nil {
				return
			}
			lines <- line
		}
	}()
	if err := c.CancelSession(nil, "s"); err != nil {
		t.Fatalf("nil ctx 应可用: %v", err)
	}
	select {
	case <-lines:
	case <-time.After(2 * time.Second):
		t.Fatal("2s 内取消帧未写出")
	}
}
