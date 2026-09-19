package main

// codexgoalprobe 探针:对真实 codex app-server 做 thread/goal set→get→clear 的
// 双形状对照实测,钉死 set 参数名的证据链:
//
//	旧行 {"threadId","goal"}     → 预期被 -32600 "no goal exists" 拒(P1 缺陷实证);
//	新行 {"threadId","objective"} → 预期成功,get 回显 objective;
//	clear                        → 预期 {"cleared":true}。
//
// 先 thread/start 建临时线程,全程 app-server 内存态,不落盘。参考 cmd/codexreadprobe。
import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

	initCtx, initCancel := context.WithTimeout(context.Background(), 60*time.Second)
	_, err = c.CallCtx(initCtx, "initialize", map[string]interface{}{
		"clientInfo": map[string]string{"name": "codexgoalprobe", "version": "0.1"},
	})
	initCancel()
	if err != nil {
		log.Fatal("initialize:", err)
	}

	// call 带短超时的一次调用;结果原样打印(截断 600 字节)。
	call := func(tag, method string, params interface{}) json.RawMessage {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		raw, err := c.CallCtx(ctx, method, params)
		if err != nil {
			fmt.Printf("=== %s ERROR: %v\n", tag, err)
			return nil
		}
		s := string(raw)
		if len(s) > 600 {
			s = s[:600] + "…(截断)"
		}
		fmt.Printf("=== %s OK: %s\n", tag, s)
		return raw
	}

	rawTS := call("thread/start", "thread/start", map[string]interface{}{"cwd": "."})
	var ts struct {
		ThreadId string `json:"threadId"`
		Thread   struct {
			Id string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(rawTS, &ts); err != nil {
		log.Fatalf("thread/start 应答解析失败: %v raw=%.200s", err, string(rawTS))
	}
	tid := ts.ThreadId
	if tid == "" {
		tid = ts.Thread.Id
	}
	fmt.Printf("=== thread/start tid=%s\n", tid)

	// 阴性对照:旧形状(goal 键)——缺陷实锤
	call("set[旧行:goal键]", "thread/goal/set", map[string]interface{}{"threadId": tid, "goal": "旧形状目标"})
	// 阳性:新形状(objective 键)——修复实证
	call("set[新行:objective键]", "thread/goal/set", map[string]interface{}{"threadId": tid, "objective": "新形状目标"})
	call("get[set后]", "thread/goal/get", map[string]interface{}{"threadId": tid})
	call("clear", "thread/goal/clear", map[string]interface{}{"threadId": tid})
	call("get[clear后]", "thread/goal/get", map[string]interface{}{"threadId": tid})
}
