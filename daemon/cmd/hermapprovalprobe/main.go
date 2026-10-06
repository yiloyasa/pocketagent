// hermapprovalprobe —— hermes ACP 审批面探针(真进程 + 真工具任务实测,HM 深接)。
//
// 实证面:跑一轮"创建文件"任务,抓 session/request_permission 反向请求原始帧
// (permissions.py 的危险命令审批 + edit_approval.py 的文件编辑提案两条路),
// 按手机审批卡同款逻辑裁决(默认自动允许,-deny 演练拒绝),并逐变体对表
// session/update 流(流式完整度)。
//
// 红线:工作区用临时目录(不在 hermes 家目录(见 HERMES_HOME)与上游 checkout 写任何文件);
// hermes 家目录(见 HERMES_HOME)用户配置只读。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/hermesapp"
)

func main() {
	deny := flag.Bool("deny", false, "审批一律拒绝(演练 deny 路;默认允许)")
	model := flag.String("model", "", "模型 ID(如 deepseek:deepseek-flash);空=跟 config.yaml 默认")
	timeout := flag.Duration("timeout", 300*time.Second, "整轮预算")
	flag.Parse()

	py, dir, err := hermesapp.LocateHermes()
	must("locate", err)
	fmt.Printf("[1] locate python=%s acp=%s\n", py, dir)

	c, err := hermesapp.Start(py, dir)
	must("start", err)
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	init, err := c.Initialize(ctx)
	must("initialize", err)
	fmt.Printf("[2] initialize agent=%s v=%s\n", init.AgentInfo.Name, init.AgentInfo.Version)

	ws, err := os.MkdirTemp("", "hermes-approval-probe-*")
	must("tempdir", err)
	defer os.RemoveAll(ws)
	target := filepath.Join(ws, "hermes-approval-probe.txt")

	st, err := c.SessionNew(ctx, ws)
	must("session/new", err)
	fmt.Printf("[3] session/new id=%s cwd=%s\n", st.SessionID, ws)
	if *model != "" {
		if err := c.SetSessionModel(ctx, st.SessionID, *model); err != nil {
			fmt.Printf("[3b] set_session_model 失败(继续): %v\n", err)
		}
	}

	// 逐变体台账(update 流完整度)+ 审批反向请求裁决。
	var mu sync.Mutex
	variants := map[string]int{}
	var permFrames []string
	var outcomes []string
	c.SetTurnHandlers(func(raw json.RawMessage) {
		ev := hermesapp.ParseSessionUpdate(raw)
		mu.Lock()
		variants[updateTag(raw)]++
		mu.Unlock()
		switch {
		case ev.ToolCall != nil:
			mu.Lock()
			fmt.Printf("[u] %s id=%s title=%q status=%s kind=%s diff=%v out=%.120s\n",
				ev.ToolCall.Kind, ev.ToolCall.ToolCallID, ev.ToolCall.Title, ev.ToolCall.Status,
				ev.ToolCall.ToolKind, ev.ToolCall.Diff != nil, ev.ToolCall.Output)
			mu.Unlock()
		case ev.MessageChunk != "":
			fmt.Printf("[u] agent_message_chunk %.120s\n", ev.MessageChunk)
		case ev.Usage != nil:
			fmt.Printf("[u] usage_update size=%d used=%d\n", ev.Usage.Size, ev.Usage.Used)
		case ev.SessionInfo != nil:
			fmt.Printf("[u] session_info_update title=%q\n", ev.SessionInfo.Title)
		}
	}, func(id int64, method string, params json.RawMessage) {
		if method != "session/request_permission" {
			fmt.Printf("[?] 反向请求 %s → -32601\n", method)
			_ = c.RespondError(id, -32601, "method not found: "+method)
			return
		}
		perm, err := hermesapp.ParseRequestPermission(params)
		if err != nil {
			fmt.Printf("[!] request_permission 解析失败 → -32602: %v\n", err)
			_ = c.RespondError(id, -32602, err.Error())
			return
		}
		option := perm.RejectOptionID()
		verb := "DENY"
		if !*deny {
			option = perm.AllowOptionID()
			verb = "ALLOW"
		}
		mu.Lock()
		permFrames = append(permFrames, string(params))
		outcomes = append(outcomes, verb+":"+option)
		mu.Unlock()
		fmt.Printf("[perm] 原始帧: %s\n", string(params))
		fmt.Printf("[perm] 裁决=%s optionId=%s(手机审批卡同款 selected 应答)\n", verb, option)
		_ = c.Respond(id, hermesapp.PermissionOutcomeSelected(option))
	})
	defer c.SetTurnHandlers(nil, nil)

	task := fmt.Sprintf("请在工作区创建文件 %s,内容恰好一行:hermes approval probe ok。只允许写到工作区内。完成后只回复两个字母:OK。", target)
	out, err := c.Prompt(ctx, st.SessionID, task)
	if err != nil {
		fmt.Printf("[4] prompt 失败: %v\n", err)
	} else {
		stop := ""
		if out != nil {
			stop = out.StopReason
		}
		fmt.Printf("[4] prompt stop=%s\n", stop)
	}

	created := false
	if b, err := os.ReadFile(target); err == nil {
		created = strings.Contains(string(b), "hermes approval probe ok")
		fmt.Printf("[5] 文件已创建内容正确=%v\n", created)
	} else {
		fmt.Printf("[5] 文件未创建(%v)——审批链未走到工具执行\n", err)
	}

	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("[6] update 变体台账: %s\n", sortedCounts(variants))
	fmt.Printf("[7] 审批帧 %d 个,裁决: %s\n", len(permFrames), strings.Join(outcomes, " | "))
	if *deny {
		fmt.Printf("[8] deny 演练:文件应当不存在(存在=%v)\n", created)
	} else {
		fmt.Printf("[8] allow 路:文件应存在=%v\n", created)
	}
	fmt.Println("== hermapprovalprobe 完成 ==")
}

// updateTag 提取 update.sessionUpdate 原始标签(台账以线形状为准)。
func updateTag(raw json.RawMessage) string {
	var u struct {
		Update struct {
			SessionUpdate string `json:"sessionUpdate"`
		} `json:"update"`
	}
	if json.Unmarshal(raw, &u) != nil || u.Update.SessionUpdate == "" {
		return "<unparsed>"
	}
	return u.Update.SessionUpdate
}

func must(stage string, err error) {
	if err != nil {
		fmt.Printf("!! %s: %v\n", stage, err)
		os.Exit(1)
	}
}

func sortedCounts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, fmt.Sprintf("%s=%d", k, m[k]))
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}
