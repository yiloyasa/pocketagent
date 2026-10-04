// codexliveturnprobe 探针:0.159.2 活体单轮实证(协议适配轮)。
//
// 【目的】两件 daemon 关键回路在升级后的 A 级验证,一次最小 LLM 轮次拿到:
//  1. 审批回路:approvalPolicy=untrusted 下,echo 类命令应触发
//     item/commandExecution/requestApproval(或旧版 execCommandApproval)反向请求;
//     探针按 daemon 同款应答 {"decision":"accept"},观察命令是否真被执行、
//     轮次是否正常收口(turn/completed)。同时留样 params 全文,核对 0.159.2
//     新增的 kind(command/writeStdin)字段是否出现在真实帧里。
//  2. turn/steer:turn/started 到达后,按 daemon 同款(threadId+input+
//     expectedTurnId)插入一条文本,记录应答 turnId 与后续是否被吸收。
//
// 【红线】只在 %TEMP% 下建 cwd;提示词要求只跑 echo,零文件写入;结束后
// thread/delete 清理探针线程。默认一轮回合,不做多轮。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()

	if _, err := c.CallCtx(ctx, "initialize", map[string]interface{}{
		"clientInfo": map[string]string{"name": "codexliveturnprobe", "version": "0.1"},
	}); err != nil {
		log.Fatal("initialize:", err)
	}
	fmt.Println("=== initialize OK ===")

	var mu sync.Mutex
	var notifMethods []string
	var approvals []string
	steerDone := make(chan string, 1) // steer 应答 turnId(或错误文案)
	turnStartedID := make(chan string, 1)
	completed := make(chan string, 1) // turn/completed 或 turn/failed 的留样

	var threadID string
	c.SetTurnHandlers(
		func(method string, params json.RawMessage) {
			mu.Lock()
			notifMethods = append(notifMethods, method)
			mu.Unlock()
			switch method {
			case "turn/started":
				var n struct {
					ThreadId string `json:"threadId"`
					Turn     struct {
						Id string `json:"id"`
					} `json:"turn"`
				}
				_ = json.Unmarshal(params, &n)
				select {
				case turnStartedID <- n.Turn.Id:
				default:
				}
			case "turn/completed", "turn/failed":
				s := string(params)
				if len([]rune(s)) > 800 {
					s = string([]rune(s)[:800]) + "…(截断)"
				}
				select {
				case completed <- method + " " + s:
				default:
				}
			}
		},
		func(id json.RawMessage, method string, params json.RawMessage) {
			full := fmt.Sprintf("method=%s id=%s params=%s", method, string(id), string(params))
			mu.Lock()
			approvals = append(approvals, full)
			mu.Unlock()
			fmt.Printf("<<< 反向请求 %s: %.700s\n", method, string(params))
			// daemon 同款:命令类审批回 {"decision":"accept"}(v2 枚举)。
			_ = c.Respond(id, map[string]string{"decision": "accept"})
		},
	)

	// ---- cwd:%TEMP% 下独立目录,零污染 ----
	cwd := filepath.Join(os.TempDir(), "pa_liveturn_cwd")
	_ = os.MkdirAll(cwd, 0o755)

	// ---- thread/start(untrusted 审批策略)----
	var startRes struct {
		Thread struct {
			Id string `json:"id"`
		} `json:"thread"`
	}
	rawStart, err := c.CallCtx(ctx, "thread/start", map[string]interface{}{
		"cwd": cwd, "approvalPolicy": "untrusted",
	})
	if err != nil {
		log.Fatal("thread/start:", err)
	}
	if err := json.Unmarshal(rawStart, &startRes); err != nil || startRes.Thread.Id == "" {
		// 三级回退第一级:顶层 threadId 形状。
		var alt struct {
			ThreadId string `json:"threadId"`
		}
		_ = json.Unmarshal(rawStart, &alt)
		threadID = alt.ThreadId
		if threadID == "" {
			log.Fatalf("thread/start 解析不出 threadId: %.400s", string(rawStart))
		}
	} else {
		threadID = startRes.Thread.Id
	}
	fmt.Printf("=== thread/start OK threadId=%s cwd=%s\n", threadID, cwd)
	defer func() {
		dctx, dcancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dcancel()
		if _, err := c.CallCtx(dctx, "thread/delete", map[string]interface{}{"threadId": threadID}); err != nil {
			fmt.Printf("=== 清理 thread/delete 失败(留痕不阻塞): %v\n", err)
		} else {
			fmt.Printf("=== 清理 thread/delete OK %s\n", threadID)
		}
	}()

	// ---- turn/start(最小 echo 指令)----
	if _, err := c.CallCtx(ctx, "turn/start", map[string]interface{}{
		"threadId": threadID,
		"input":    []map[string]string{{"type": "text", "text": "请原样运行命令 echo PA_01592_OK,完成后用一句话告诉我命令输出。不要做任何其他事。"}},
	}); err != nil {
		log.Fatal("turn/start:", err)
	}
	fmt.Println("=== turn/start 已发出,等待 turn/started ===")

	// ---- steer:turn/started 到达后插一条(daemon 同款形状)----
	go func() {
		select {
		case tid := <-turnStartedID:
			if tid == "" {
				return
			}
			time.Sleep(500 * time.Millisecond) // 等轮次真正 active
			sctx, scancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer scancel()
			raw, err := c.CallCtx(sctx, "turn/steer", map[string]interface{}{
				"threadId":       threadID,
				"input":          []map[string]string{{"type": "text", "text": "(探针插话)结束后请额外单独输出一行:STEER_OK"}},
				"expectedTurnId": tid,
			})
			if err != nil {
				select {
				case steerDone <- "ERROR " + err.Error():
				default:
				}
				return
			}
			select {
			case steerDone <- "OK " + string(raw):
			default:
			}
		case <-ctx.Done():
		}
	}()

	// ---- 等轮次收口 ----
	select {
	case done := <-completed:
		fmt.Printf("=== 收口: %s\n", done)
	case <-ctx.Done():
		fmt.Println("=== 240s 未收口(如实留样)===")
	}
	select {
	case s := <-steerDone:
		fmt.Printf("=== steer 应答: %.400s\n", s)
	default:
		fmt.Println("=== steer 应答: (未发生/未返回)===")
	}

	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("=== 通知序列(%d): %s\n", len(notifMethods), strings.Join(notifMethods, ", "))
	fmt.Printf("=== 反向请求(%d):\n", len(approvals))
	for _, a := range approvals {
		fmt.Printf("    %s\n", a)
	}
	fmt.Println("=== 探针结束 ===")
}
