package main

// WorkflowB 探针:对**活着的** codex app-server 实测
//
//  1. thread/start 的 config 覆盖是否被接受(model_verbosity / model_reasoning_summary);
//  2. turn/start 的 summary / effort / model / approvalPolicy / sandboxPolicy 是否被接受;
//  3. config/value/write 对本 codex 版本而言,哪些 keyPath 是**真实有效**的
//     —— 关键:**必须带 filePath 指向临时文件**,否则默认写用户 ~/.codex/config.toml(红线)。
//
// 目的:为"哪些设置项真实支持、哪些不支持"提供可复现证据,而不是靠猜。
import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
)

func main() {
	c, err := codexapp.Start()
	if err != nil {
		log.Fatal("spawn app-server:", err)
	}
	defer c.Close()

	c.NotifHandler = func(method string, params json.RawMessage) {}
	c.ReqHandler = func(id int64, method string, params json.RawMessage) {
		_ = c.Respond(id, map[string]string{"decision": "accept"})
	}

	if _, err := c.Call("initialize", map[string]interface{}{
		"clientInfo": map[string]string{"name": "wbprobe", "version": "0.1"},
	}); err != nil {
		log.Fatal("initialize:", err)
	}
	fmt.Println("=== initialize OK ===")

	// ---- 2) turn/start 支持的字段(用独立线程) ----
	rawTS, err := c.Call("thread/start", map[string]interface{}{
		"cwd": `C:\dev\pocketagent`,
		// thread 级 config 覆盖:验证 model_verbosity / model_reasoning_summary 是否被接受
		"config": map[string]interface{}{
			"model_verbosity":         "high",
			"model_reasoning_summary": "detailed",
		},
	})
	if err != nil {
		fmt.Println(">>> thread/start(带 config 覆盖) 失败:", err)
	} else {
		fmt.Println(">>> thread/start(带 config 覆盖) OK:", trunc(string(rawTS)))
	}
	var tr struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(rawTS, &tr)

	// turn/start:逐项验证支持的字段(只发不发消息的 turn 会真的跑模型,这里只发一条极短指令)
	if tr.Thread.ID != "" {
		tp := map[string]interface{}{
			"threadId": tr.Thread.ID,
			"input":    []map[string]string{{"type": "text", "text": "Reply with exactly: PONG"}},
			// 逐项:summary(=思考摘要)、effort、model、approvalPolicy、sandboxPolicy
			"summary":        "concise",
			"effort":         "low",
			"approvalPolicy": "never",
			"sandboxPolicy":  map[string]interface{}{"type": "dangerFullAccess"},
		}
		if _, err := c.Call("turn/start", tp); err != nil {
			fmt.Println(">>> turn/start(带 summary/effort/approval/sandbox) 失败:", err)
		} else {
			fmt.Println(">>> turn/start(带 summary/effort/approval/sandbox) 接受(done 事件稍后到)")
		}
	}

	// ---- 3) config/value/write 逐 key 实测(带 filePath 指向临时文件,绝不碰用户 config.toml) ----
	tmp := filepath.Join(os.TempDir(), "wbprobe_config.toml")
	_ = os.Remove(tmp)
	fmt.Println("\n=== config/value/write 逐 key 实测(filePath =", tmp, ") ===")
	keys := []struct{ k, v string }{
		{"approval_policy", "on-request"},
		{"sandbox_mode", "workspace-write"},
		{"model_reasoning_effort", "high"},
		{"model_verbosity", "medium"},
		{"model_reasoning_summary", "concise"},
		{"hide_agent_reasoning", "true"}, // 预期:不支持
		{"disable_paste_burst", "true"},  // 预期:不支持
	}
	for _, kv := range keys {
		// 正确形状:必须带 mergeStrategy(required)
		_, err := c.Call("config/value/write", map[string]interface{}{
			"keyPath":         kv.k,
			"value":           kv.v,
			"mergeStrategy":   "upsert",
			"filePath":        tmp,
			"expectedVersion": nil,
		})
		if err != nil {
			fmt.Printf("  [不支持] %-26s -> %v\n", kv.k, err)
		} else {
			fmt.Printf("  [ 支持 ] %-26s -> ok\n", kv.k)
		}
	}

	// 对照:不带 mergeStrategy 是否能成功(验证"缺 mergeStrategy 会失败"这一根因)
	_, err = c.Call("config/value/write", map[string]interface{}{
		"keyPath":         "model_verbosity",
		"value":           "medium",
		"filePath":        tmp,
		"expectedVersion": nil,
	})
	fmt.Printf("\n对照(缺 mergeStrategy): err=%v\n", err)

	time.Sleep(1500 * time.Millisecond)
	fmt.Println("\n=== 探针结束 ===")
}

func trunc(s string) string {
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}
