// cmd/pisteerprobe —— 补充探针:验证 Pi 的 steer 命令(运行中插话)。
//
// PiProvider.Steer 的协议依据(docs/rpc.md 的 steer 命令),P0 主探针 11 项里
// 没覆盖它,而 P1"聊天输入运行中可用(steer/followUp)"直接依赖 —— 单独实测:
//  1. 长任务运行中发 steer → 消息进入队列(queue_update)→ 被执行;
//  2. 空闲时发 steer → success:false(server 层据此转排队兜底)。
//
// 结论写进 docs/verify/evidence/pi-p0-probe.md 的补充节。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/hidewin"
	"github.com/pocketagent/daemon/internal/hostenv"
)

// ---- 定位(与 piprobe 同款:env 覆盖 → 通用派生 → 报错退出)----
//
// 【为什么没有写死的默认路径】本探针曾把开发机的 node.exe 与 pi 入口写成常量:
// 字符串常量会随探针二进制分发(规格 §2 明令禁止),且换一台机器必然不命中。
// 开发机自测路径经 PI_NODE / PI_CLI 注入,不进代码。

// piCLIRel 是 pi 在 npm 包内的入口相对路径(直接 node 调 cli.js)。
const piCLIRel = "@earendil-works/pi-coding-agent/dist/bundle/cli.js"

func locateNode() string {
	if p := strings.TrimSpace(os.Getenv("PI_NODE")); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return hostenv.FindNode()
}

func locateCLI() string {
	if p := strings.TrimSpace(os.Getenv("PI_CLI")); p != "" {
		return p
	}
	return hostenv.FirstFile(hostenv.PackageCandidates(piCLIRel))
}

func main() {
	node, cli := locateNode(), locateCLI()
	if node == "" || cli == "" {
		fmt.Printf("未找到 node 或 pi 入口(node=%q cli=%q)。请设 PI_NODE / PI_CLI 后重试。\n", node, cli)
		os.Exit(1)
	}
	base, _ := os.MkdirTemp("", "pisteer-*")
	defer os.RemoveAll(base)

	cmd := exec.Command(node, cli, "--mode", "rpc", "--session-dir", base)
	cmd.Dir = base
	hidewin.Apply(cmd)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		fmt.Println("START FAIL:", err)
		return
	}
	defer cmd.Process.Kill()

	var mu sync.Mutex
	nextID := 0
	pending := map[string]chan map[string]any{}
	events := make(chan map[string]any, 4096)
	go func() {
		r := bufio.NewReaderSize(stdout, 8*1024*1024)
		for {
			line, err := r.ReadBytes('\n')
			if len(line) > 0 {
				var m map[string]any
				if json.Unmarshal(line, &m) == nil {
					if t, _ := m["type"].(string); t == "response" {
						if id, _ := m["id"].(string); id != "" {
							mu.Lock()
							ch, ok := pending[id]
							delete(pending, id)
							mu.Unlock()
							if ok {
								ch <- m
							}
							continue
						}
					}
					select {
					case events <- m:
					default:
					}
				}
			}
			if err != nil {
				return
			}
		}
	}()

	call := func(typ string, extra map[string]any, to time.Duration) map[string]any {
		mu.Lock()
		nextID++
		id := fmt.Sprintf("sp-%d", nextID)
		ch := make(chan map[string]any, 1)
		pending[id] = ch
		mu.Unlock()
		obj := map[string]any{"id": id, "type": typ}
		for k, v := range extra {
			obj[k] = v
		}
		raw, _ := json.Marshal(obj)
		stdin.Write(append(raw, '\n'))
		select {
		case r := <-ch:
			return r
		case <-time.After(to):
			return map[string]any{"success": false, "error": "timeout"}
		}
	}

	time.Sleep(2 * time.Second)

	// A. 空闲时 steer:预期 success:false(排队兜底的协议依据)
	resp := call("steer", map[string]any{"message": "hello"}, 10*time.Second)
	fmt.Printf("[idle-steer] success=%v error=%v\n", resp["success"], resp["error"])

	// B. 长任务运行中 steer:预期受理 + 内容被执行
	r2 := call("prompt", map[string]any{"message": "用 bash 工具原样执行这条命令,别的什么都不要做:for i in $(seq 1 20); do echo tick-$i; sleep 0.4; done"}, 15*time.Second)
	fmt.Printf("[prompt] success=%v\n", r2["success"])
	// 等工具开跑
	deadline := time.After(30 * time.Second)
	toolStarted := false
	for !toolStarted {
		select {
		case m := <-events:
			if t, _ := m["type"].(string); t == "tool_execution_start" {
				toolStarted = true
			}
		case <-deadline:
		}
		if toolStarted || deadline == nil {
			break
		}
		if !toolStarted {
			select {
			case <-time.After(100 * time.Millisecond):
			case <-deadline:
			}
		}
	}
	time.Sleep(1 * time.Second)
	r3 := call("steer", map[string]any{"message": "追加:再原样执行一次 echo STEER_OK 然后结束"}, 15*time.Second)
	fmt.Printf("[running-steer] success=%v error=%v\n", r3["success"], r3["error"])

	// 等 settled,收集 tool_execution_end 输出
	settled := false
	steerOK := false
	deadline2 := time.After(120 * time.Second)
	for !settled {
		select {
		case m := <-events:
			t, _ := m["type"].(string)
			if t == "tool_execution_end" {
				raw, _ := json.Marshal(m)
				if strings.Contains(string(raw), "STEER_OK") {
					steerOK = true
				}
			}
			if t == "agent_settled" {
				settled = true
			}
		case <-deadline2:
			settled = true
		}
	}
	fmt.Printf("[result] toolStarted=%v steerAccepted=%v steerExecuted=%v\n", toolStarted, r3["success"], steerOK)
}
