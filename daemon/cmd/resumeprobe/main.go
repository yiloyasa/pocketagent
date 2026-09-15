package main

// 验证 thread/resume 的参数形状与语义,以及 turn/start 打到未知 threadId 时的行为
// (挂死 vs 报错)—— 这是修复"daemon 重启后旧会话续聊卡死"的最后证据。
import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
)

var mu sync.Mutex
var events []string

func main() {
	c, err := codexapp.Start()
	if err != nil {
		log.Fatal("spawn app-server:", err)
	}
	defer c.Close()

	c.NotifHandler = func(method string, params json.RawMessage) {
		mu.Lock()
		events = append(events, method)
		mu.Unlock()
		if strings.Contains(method, "delta") {
			return // delta 太吵,只计数
		}
		fmt.Printf("[%s] NOTIF %s %.200s\n", time.Now().Format("15:04:05.000"), method, string(params))
	}
	c.ReqHandler = func(id int64, method string, params json.RawMessage) {
		fmt.Printf("[%s] REQ(id=%d) %s %.200s\n", time.Now().Format("15:04:05.000"), id, method, string(params))
		_ = c.Respond(id, map[string]string{"decision": "accept"})
	}

	if _, err := c.Call("initialize", map[string]interface{}{"clientInfo": map[string]string{"name": "resumeprobe", "version": "0.1"}}); err != nil {
		log.Fatal("initialize:", err)
	}

	// ── 1) 建一个真线程并跑完一轮(拿到有效 threadId + 已落盘会话)──
	raw, err := c.Call("thread/start", map[string]interface{}{"cwd": `C:\dev\pocketagent`})
	if err != nil {
		log.Fatal("thread/start:", err)
	}
	var tr struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(raw, &tr)
	tid := tr.Thread.ID
	fmt.Println("== thread/start ->", tid)

	waitCompleted(c, tid, "Reply with exactly: PONG", 120*time.Second)
	fmt.Printf("== 基线轮完成,事件数=%d ==\n\n", count())

	// ── 2) thread/resume 真 id:验证参数形状与应答 ──
	resumeRaw, err := c.Call("thread/resume", map[string]interface{}{"threadId": tid})
	fmt.Printf("== thread/resume(真id) err=%v raw=%.300s\n\n", err, string(resumeRaw))

	// ── 3) turn/start 打到【不存在的 threadId】:看是报错还是无声吞掉 ──
	errCh := make(chan error, 1)
	go func() {
		_, err := c.Call("turn/start", map[string]interface{}{
			"threadId": "does-not-exist-xyz",
			"input":    []map[string]string{{"type": "text", "text": "Reply with exactly: PONG"}},
		})
		errCh <- err
	}()
	select {
	case err := <-errCh:
		fmt.Printf("== turn/start(假id) 返回 err=%v(会走 FAILED 路径)\n\n", err)
	case <-time.After(25 * time.Second):
		fmt.Printf("== turn/start(假id) 25 秒无应答且无任何通知 → 静默吞掉,确认卡死根因 ==\n\n")
	}

	// ── 4) resume 后再跑一轮:daemon 修复后将走的路径 ──
	waitCompleted(c, tid, "Reply with exactly: PONG2", 120*time.Second)
	fmt.Printf("== resume 后第二轮完成,事件数=%d ==\n", count())
}

func waitCompleted(c *codexapp.Client, tid, prompt string, d time.Duration) {
	mu.Lock()
	base := len(events)
	mu.Unlock()
	if _, err := c.Call("turn/start", map[string]interface{}{
		"threadId": tid,
		"input":    []map[string]string{{"type": "text", "text": prompt}},
	}); err != nil {
		log.Fatalf("turn/start(%s): %v", prompt, err)
	}
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		mu.Lock()
		n := len(events)
		mu.Unlock()
		for i := base; i < n; i++ {
			mu.Lock()
			m := events[i]
			mu.Unlock()
			if m == "turn/completed" {
				return
			}
		}
	}
	log.Fatalf("waitCompleted(%s): %v 内未见 turn/completed", prompt, d)
}

func count() int {
	mu.Lock()
	defer mu.Unlock()
	return len(events)
}
