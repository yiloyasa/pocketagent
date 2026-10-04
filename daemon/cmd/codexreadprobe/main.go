package main

// codexreadprobe 探针:对**活着的** codex app-server 一次性实测四个只读方法的
// 请求/应答形状,为 daemon 侧只读桥接(internal/agent/codex_read.go)提供可复现证据:
//
//	1) config/read                       —— 设置页回显"桌面当前真实值"
//	2) thread/goal/get                   —— 重连后回读目标态
//	3) modelProvider/capabilities/read   —— 模型页解释"为什么某模型不可用"
//	4) windowsSandbox/readiness          —— Windows 沙箱就绪度诊断
//	5) review/start                      —— "/计划"发起计划审查(§8 待实测项)
//
// 只读,不写任何配置;goal 探测先 thread/start 建临时线程再 set/get/clear,
// 全程在 app-server 内存态,不落盘。review/start 会真的发起一轮审查 turn
// (消耗模型 token),形状确认后即 CancelTurn 不等它跑完。
import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
)

func main() {
	c, err := codexapp.Start()
	if err != nil {
		log.Fatal("spawn app-server:", err)
	}
	defer c.Close()

	// 收集通知(method 名 + 截断参数),review/start 之后打印一段,
	// 为"审查结果以事件形式流回"提供证据。
	var (
		notifMu   sync.Mutex
		notifLog  []string
		startTime = time.Now()
	)
	collectNotif := func(method string, params json.RawMessage) {
		notifMu.Lock()
		defer notifMu.Unlock()
		s := string(params)
		if len(s) > 160 {
			s = s[:160] + "…"
		}
		notifLog = append(notifLog, fmt.Sprintf("[%6.1fs] %s %s", time.Since(startTime).Seconds(), method, s))
	}
	c.NotifHandler = collectNotif
	c.ReqHandler = func(id json.RawMessage, method string, params json.RawMessage) {
		_ = c.Respond(id, map[string]string{"decision": "accept"})
	}

	initCtx, initCancel := context.WithTimeout(context.Background(), 60*time.Second)
	_, err = c.CallCtx(initCtx, "initialize", map[string]interface{}{
		"clientInfo": map[string]string{"name": "codexreadprobe", "version": "0.1"},
	})
	initCancel()
	if err != nil {
		log.Fatal("initialize:", err)
	}
	fmt.Println("=== initialize OK ===")

	// call 带短超时的一次调用;失败时打印错误并返回 nil,不中断整个探针。
	call := func(tag, method string, params interface{}) json.RawMessage {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		raw, err := c.CallCtx(ctx, method, params)
		if err != nil {
			fmt.Printf("=== %s ERROR: %v\n", tag, err)
			return nil
		}
		// 只打印形状:截断到 1200 字节,避免把整份 config 刷屏
		s := string(raw)
		if len(s) > 1200 {
			s = s[:1200] + "…(截断)"
		}
		fmt.Printf("=== %s OK: %s\n", tag, s)
		return raw
	}

	// ---- 1) config/read ----
	call("config/read", "config/read", map[string]interface{}{"includeLayers": true})

	// ---- 2) thread/goal/get(先建临时线程)----
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

	call("thread/goal/get(未设目标)", "thread/goal/get", map[string]interface{}{"threadId": tid})
	// set 一次再看 get 的实心形状(codex 会规范化文本)。
	// 【实测 0.149.1】参数名是 objective(v2 schema ThreadGoalSetParams),旧形状
	// {"goal":...} 会被 -32600 "no goal exists" 拒绝。
	if _, err := c.Call("thread/goal/set", map[string]interface{}{"threadId": tid, "objective": "探针临时目标"}); err != nil {
		fmt.Printf("=== thread/goal/set ERROR: %v\n", err)
	}
	call("thread/goal/get(已设目标)", "thread/goal/get", map[string]interface{}{"threadId": tid})
	_, _ = c.Call("thread/goal/clear", map[string]interface{}{"threadId": tid})

	// ---- 3) modelProvider/capabilities/read ----
	call("modelProvider/capabilities/read", "modelProvider/capabilities/read", map[string]interface{}{})

	// ---- 4) windowsSandbox/readiness(schema:params 为 null)----
	call("windowsSandbox/readiness", "windowsSandbox/readiness", nil)

	// ---- 5) review/start:"/计划"的底层方法(§8 待实测项)----
	//
	// v2 schema 形状(ReviewStartParams,待 live 确认):
	//   required = threadId + target;target 四选一:
	//     {type:"uncommittedChanges"} / {type:"baseBranch",branch} /
	//     {type:"commit",sha,title?} / {type:"custom",instructions}
	//   delivery 可省(缺省 inline = 审查跑在原线程,结果以事件流回)。
	// 应答(ReviewStartResponse):required = reviewThreadId + turn。
	fmt.Println("=== review/start(缺 threadId,期望 -32602 形状错误)===")
	call("review/start(无参数)", "review/start", map[string]interface{}{})

	reviewParams := map[string]interface{}{
		"threadId": tid,
		"target": map[string]interface{}{
			"type":         "custom",
			"instructions": "探针:请用一句话确认你收到了这次计划审查请求(不要执行任何工具)。",
		},
	}
	reviewStart := time.Now()
	rawRev := call("review/start(custom,threadId="+tid+")", "review/start", reviewParams)
	fmt.Printf("=== review/start 耗时 %.1fs\n", time.Since(reviewStart).Seconds())
	// 打印应答里 turn 的关键字段(id/status),看"发起即回"还是"等跑完才回"。
	if len(rawRev) > 0 {
		var rev struct {
			ReviewThreadId string `json:"reviewThreadId"`
			Turn           struct {
				Id     string `json:"id"`
				Status string `json:"status"`
			} `json:"turn"`
		}
		if err := json.Unmarshal(rawRev, &rev); err == nil {
			fmt.Printf("=== review/start 解析:reviewThreadId=%q turnId=%q turnStatus=%q\n",
				rev.ReviewThreadId, rev.Turn.Id, rev.Turn.Status)
		}
	}
	// 错误形状:不存在的 threadId(对应 daemon 侧"线程未加载"的处理依据)。
	call("review/start(不存在的 threadId)", "review/start", map[string]interface{}{
		"threadId": "00000000-0000-0000-0000-000000000000",
		"target":   map[string]interface{}{"type": "custom", "instructions": "x"},
	})
	// 留 12 秒收事件流(审查 turn 的通知形状),然后尽最大努力取消,
	// 不让探针空等一轮审查跑完。
	time.Sleep(12 * time.Second)
	fmt.Println("=== review 后 12s 内的通知流 ===")
	notifMu.Lock()
	for _, n := range notifLog {
		fmt.Println("  ", n)
	}
	notifMu.Unlock()
	fmt.Println("=== turn/interrupt(清理探针发起的审查轮)===")
	var revTid string
	if len(rawRev) > 0 {
		var rev struct {
			ReviewThreadId string `json:"reviewThreadId"`
			Turn           struct {
				Id string `json:"id"`
			} `json:"turn"`
		}
		if json.Unmarshal(rawRev, &rev) == nil {
			revTid = rev.ReviewThreadId
		}
	}
	if revTid == "" {
		revTid = tid
	}
	if _, err := c.Call("turn/interrupt", map[string]interface{}{"threadId": revTid}); err != nil {
		fmt.Printf("=== turn/interrupt ERROR(不影响结论): %v\n", err)
	}
}
