package main

// 直连 codex app-server 的最小对话探针:thread/new → turn/start,
// 打印 120 秒内的每一个原始通知/请求 —— 拿"codex 是否真的收到任务"的证据。
import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
)

func main() {
	c, err := codexapp.Start()
	if err != nil {
		log.Fatal("spawn app-server:", err)
	}
	defer c.Close()

	var events []string
	c.NotifHandler = func(method string, params json.RawMessage) {
		line := fmt.Sprintf("[%s] NOTIF %s %.260s", time.Now().Format("15:04:05.000"), method, string(params))
		fmt.Println(line)
		events = append(events, line)
	}
	c.ReqHandler = func(id json.RawMessage, method string, params json.RawMessage) {
		line := fmt.Sprintf("[%s] REQ(id=%s) %s %.260s", time.Now().Format("15:04:05.000"), string(id), method, string(params))
		fmt.Println(line)
		// 审批类请求一律允许,保证流程能走通
		_ = c.Respond(id, map[string]string{"decision": "accept"})
	}

	if _, err := c.Call("initialize", map[string]interface{}{"clientInfo": map[string]string{"name": "turnprobe", "version": "0.1"}}); err != nil {
		log.Fatal("initialize:", err)
	}

	raw, err := c.Call("thread/start", map[string]interface{}{"cwd": `C:\dev\pocketagent`})
	if err != nil {
		log.Fatal("thread/start:", err)
	}
	var tr struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	fmt.Println("RAW thread/new:", string(raw))
	_ = json.Unmarshal(raw, &tr)
	fmt.Println("thread id:", tr.Thread.ID)

	start := time.Now()
	tp := map[string]interface{}{
		"threadId": tr.Thread.ID,
		"input":    []map[string]string{{"type": "text", "text": "Reply with exactly: PONG"}},
	}
	// 命令行传了 "clean" 就不带任何策略参数(纯复现)
	if len(os.Args) > 1 && os.Args[1] == "clean" {
		fmt.Println("(clean 模式:不带 approvalPolicy/sandboxPolicy/model/effort)")
	} else {
		tp["approvalPolicy"] = "never"
		tp["sandboxPolicy"] = map[string]interface{}{"type": "dangerFullAccess"}
		fmt.Println("(带 approvalPolicy=never + sandboxPolicy=dangerFullAccess)")
	}
	if _, err := c.Call("turn/start", tp); err != nil {
		log.Fatal("turn/start:", err)
	}

	deadline := time.Now().Add(180 * time.Second)
	done := false
	for time.Now().Before(deadline) && !done {
		time.Sleep(500 * time.Millisecond)
		for _, e := range events {
			if contains(e, "NOTIF turn/completed") {
				done = true
			}
		}
	}
	fmt.Printf("== 探测结束,共 %d 条事件,用时 %v ==\n", len(events), time.Since(start).Round(time.Millisecond))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
