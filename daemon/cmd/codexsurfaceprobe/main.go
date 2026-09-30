// codexsurfaceprobe 探针:0.159.2 协议面实证(协议适配轮)。
//
// 【目的】升级 codex 0.156.1 → 0.159.2 后,拿到三块零 LLM 成本的 A 级证据:
//  1. 运行时自报请求面:对杜撰方法发请求,-32600 "unknown variant … expected
//     one of …" 的错误文案会**全量列出**服务端接受的请求方法(0156 校准基线,
//     docs/verify/evidence/codex-0156-probe.log 第三节),与 0.156.1 的 167 个对比。
//  2. 关键方法逐点复核(daemon 在用/在观察的):thread/read includeTurns=true 在
//     paginated 线程上是否仍全量回水(turns 是否还在)、turn/steer、thread/revert、
//     thread/compact/start、thread/attachment/list(新面)的方法存在性 ——
//     判据:报错不是 -32600 unknown variant 即方法存在(参数/资源错误也算存在)。
//  3. 握手窗口通知留样(连接即到的 configWarning / remoteControl 等)。
//
// 【红线】零 LLM 轮次:不 thread/start、不 turn/start;线程取自 thread/list 存量;
// 不写任何配置;steer/revert/compact/attachment 全部用杜撰或存量 threadId 打参数
// 校验层,不产生真实副作用(steer 对不存在线程只会报"找不到线程")。
package main

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

	initCtx, initCancel := context.WithTimeout(context.Background(), 60*time.Second)
	_, err = c.CallCtx(initCtx, "initialize", map[string]interface{}{
		"clientInfo": map[string]string{"name": "codexsurfaceprobe", "version": "0.1"},
	})
	initCancel()
	if err != nil {
		log.Fatal("initialize:", err)
	}
	fmt.Println("=== initialize OK ===")

	var notifLog []string
	c.SetGlobalHandler(func(method string, params json.RawMessage) {
		notifLog = append(notifLog, method)
	})

	call := func(tag, method string, params interface{}) json.RawMessage {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		raw, err := c.CallCtx(ctx, method, params)
		if err != nil {
			fmt.Printf("=== %s ERROR: %v\n", tag, err)
			return nil
		}
		s := string(raw)
		if len([]rune(s)) > 600 {
			s = string([]rune(s)[:600]) + "…(截断)"
		}
		fmt.Printf("=== %s OK: %s\n", tag, s)
		return raw
	}
	// callFull 同 call 但不截断:thread/read 全量导出要用(人工核对回水形状)。
	callFull := func(tag, method string, params interface{}) json.RawMessage {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		raw, err := c.CallCtx(ctx, method, params)
		if err != nil {
			fmt.Printf("=== %s ERROR: %v\n", tag, err)
			return nil
		}
		fmt.Printf("=== %s FULL-OK: %s\n", tag, string(raw))
		return raw
	}

	// ---- 1) 运行时自报请求面:杜撰方法触发 -32600 全量枚举 ----
	call("杜撰方法(拿全量方法面)", "thread/__pa_probe_nonexistent_01592", map[string]interface{}{})

	// ---- 2) 存量线程(paginated historyMode 实测于 thread/list)----
	rawList := call("thread/list", "thread/list", map[string]interface{}{"limit": 3})
	var tl struct {
		Data []struct {
			Id          string `json:"id"`
			HistoryMode string `json:"historyMode"`
		} `json:"data"`
	}
	var tid, mode string
	if err := json.Unmarshal(rawList, &tl); err == nil && len(tl.Data) > 0 {
		tid, mode = tl.Data[0].Id, tl.Data[0].HistoryMode
	}
	fmt.Printf("=== 选定线程 tid=%s historyMode=%q\n", tid, mode)

	// ---- 3) thread/read includeTurns=true:paginated 线程是否仍回水 turns ----
	// daemon 的子代理历史拉取与 rollback 锚点定位都吃这个形状。
	// 【形状注意】turns 在应答的 thread 对象内(parseCodexThreadRead 同款),
	// 这里有全量导出,人工核对回水是否还在。
	rawRead := callFull("thread/read(includeTurns=true)", "thread/read", map[string]interface{}{
		"threadId": tid, "includeTurns": true,
	})
	var tr struct {
		Thread struct {
			Turns []struct {
				Id     string `json:"id"`
				Status string `json:"status"`
			} `json:"turns"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(rawRead, &tr)
	first := ""
	if len(tr.Thread.Turns) > 0 {
		first = tr.Thread.Turns[0].Id
	}
	fmt.Printf("=== thread/read 解析: thread.turns=%d turn[0].id=%q\n", len(tr.Thread.Turns), first)

	// ---- 4) 关键方法存在性(报错但不是 unknown variant = 存在)----
	if tid != "" {
		call("thread/turns/list(存量线程)", "thread/turns/list", map[string]interface{}{"threadId": tid})
		call("thread/items/list(存量线程)", "thread/items/list", map[string]interface{}{"threadId": tid})
		call("thread/attachment/list(新面)", "thread/attachment/list", map[string]interface{}{"threadId": tid})
	}
	// steer/revert/compact 用杜撰 threadId 打参数校验层,零副作用。
	call("turn/steer(杜撰线程)", "turn/steer", map[string]interface{}{
		"threadId": "pa-probe-nonexistent", "input": []map[string]string{{"type": "text", "text": "x"}}, "expectedTurnId": "pa-probe",
	})
	call("thread/revert(杜撰线程)", "thread/revert", map[string]interface{}{"threadId": "pa-probe-nonexistent", "beforeTurnId": "pa-probe"})
	call("thread/compact/start(杜撰线程)", "thread/compact/start", map[string]interface{}{"threadId": "pa-probe-nonexistent"})

	// ---- 5) 握手窗口通知留样 ----
	time.Sleep(2 * time.Second)
	fmt.Printf("=== 握手窗口通知: %v\n", notifLog)

	fmt.Println("=== 探针结束(零 LLM 轮次,零配置写入)===")
}
