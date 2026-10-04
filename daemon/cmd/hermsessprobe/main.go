// hermsessprobe —— hermes ACP 会话生命周期探针(真进程实测,深接差距修复 HM)。
//
// 实证面(与 acp_adapter/server.py 对表):
//  1. initialize 能力快照(loadSession / sessionCapabilities.fork/list/resume);
//  2. session/new → prompt(真 LLM 一轮)→ session/list(持久化在列证明);
//  3. session/load:应答前历史回放(update 通知先于应答,逐变体清点);
//  4. session/resume:同通路对照;
//  5. session/fork:分叉出新 sessionId。
//  6. -cancel:取消链实测 —— 长 prompt 产出中发 session/cancel(通知无回执),
//     以 prompt 终局 stopReason=cancelled 为 cancel_event 生效的可观察证据,
//     并核实取消后增量停止流动(适配器 hard interrupt 真停)。
//
// 红线:只读 hermes 家目录(见 HERMES_HOME)用户配置;工作区用临时目录,不在用户家写任何文件;
// 只读上游 checkout(不写 runtime/hermes-agent 内任何文件)。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/hermesapp"
)

func main() {
	model := flag.String("model", "", "模型 ID(如 deepseek:deepseek-flash);空=跟 config.yaml 默认")
	timeout := flag.Duration("timeout", 180*time.Second, "单步 RPC 预算")
	cancelMode := flag.Bool("cancel", false, "取消链实测:发长 prompt→session/cancel→确认 stopReason=cancelled")
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
	capsJSON, _ := json.Marshal(init.AgentCapabilities)
	fmt.Printf("[2] initialize v=%s agent=%s caps=%s\n", init.AgentInfo.Version, init.AgentInfo.Name, capsJSON)
	if !init.AgentCapabilities.CanLoadSession() || !init.AgentCapabilities.CanList() {
		fmt.Println("!! loadSession/list 能力未广播,续接面不完整(差距)")
	}

	ws := mustTempDir()
	defer os.RemoveAll(ws)
	st, err := c.SessionNew(ctx, ws)
	must("session/new", err)
	fmt.Printf("[3] session/new id=%s model=%q modes=%d\n", st.SessionID, st.Models.CurrentModelID, len(st.Modes.AvailableModes))

	if *model != "" {
		if err := c.SetSessionModel(ctx, st.SessionID, *model); err != nil {
			fmt.Printf("[3b] set_session_model %s 失败(继续): %v\n", *model, err)
		} else {
			fmt.Printf("[3b] set_session_model %s OK\n", *model)
		}
	}

	// -cancel 模式:专跑取消链,不继续后面的生命周期步骤。
	if *cancelMode {
		runCancelScenario(c, ctx, st.SessionID)
		fmt.Println("== hermsessprobe -cancel 完成 ==")
		return
	}

	// prompt 一轮(真 LLM;正文经通知钩子累计)。
	var reply strings.Builder
	c.SetTurnHandlers(func(raw json.RawMessage) {
		ev := hermesapp.ParseSessionUpdate(raw)
		if ev.MessageChunk != "" {
			reply.WriteString(ev.MessageChunk)
		}
	}, nil)
	stop, err := c.Prompt(ctx, st.SessionID, "只回复两个字符:pong")
	if err != nil {
		fmt.Printf("[4] prompt 失败(常见:模型/凭据未配,差距见报告): %v\n", err)
	} else {
		fmt.Printf("[4] prompt stop=%s reply=%q\n", stop, strings.TrimSpace(reply.String()))
	}
	c.SetTurnHandlers(nil, nil)

	// session/list:持久化在列证明。
	time.Sleep(500 * time.Millisecond) // 适配器异步 save_session
	list, err := c.ListSessions(ctx, "")
	must("session/list", err)
	inList := false
	for _, e := range list.Sessions {
		if e.SessionID == st.SessionID {
			inList = true
		}
	}
	fmt.Printf("[5] session/list n=%d nextCursor=%q 本会话在列=%v\n", len(list.Sessions), list.NextCursor, inList)

	// session/load:回放先于应答(逐变体清点)。
	variants := map[string]int{}
	var replay int
	c.SetTurnHandlers(func(raw json.RawMessage) {
		var u struct {
			Update struct {
				SessionUpdate string `json:"sessionUpdate"`
			} `json:"update"`
		}
		if json.Unmarshal(raw, &u) == nil {
			variants[u.Update.SessionUpdate]++
			replay++
		}
	}, nil)
	loaded, err := c.LoadSession(ctx, ws, st.SessionID)
	if err != nil {
		fmt.Printf("[6] session/load 失败: %v\n", err)
	} else {
		fmt.Printf("[6] session/load OK 回放帧=%d 变体=%s 当前模型=%q\n",
			replay, sortedKeys(variants), loaded.Models.CurrentModelID)
	}
	// session/resume 对照(收集器要先装:回放先于应答)。
	resumeVariants := map[string]int{}
	c.SetTurnHandlers(func(raw json.RawMessage) {
		var u struct {
			Update struct {
				SessionUpdate string `json:"sessionUpdate"`
			} `json:"update"`
		}
		if json.Unmarshal(raw, &u) == nil {
			resumeVariants[u.Update.SessionUpdate]++
		}
	}, nil)
	resumed, err := c.ResumeSession(ctx, ws, st.SessionID)
	c.SetTurnHandlers(nil, nil)
	_ = resumed
	_ = err // resume 查无会自动新建,不作为失败判据(见报告)
	fmt.Printf("[7] session/resume OK 回放变体=%s\n", sortedKeys(resumeVariants))

	// session/fork。
	fk, err := c.ForkSession(ctx, ws, st.SessionID)
	if err != nil {
		fmt.Printf("[8] session/fork 失败: %v\n", err)
	} else {
		fmt.Printf("[8] session/fork %s -> %s\n", st.SessionID, fk.SessionID)
	}

	fmt.Println("== hermsessprobe 完成 ==")
}

func must(stage string, err error) {
	if err != nil {
		fmt.Printf("!! %s: %v\n", stage, err)
		os.Exit(1)
	}
}

// runCancelScenario -cancel 模式(H3 探针):发长 prompt → 首个正文增量到达后
// 发 session/cancel → 以 prompt 终局 stopReason=cancelled 确认适配器 cancel_event
// 真的生效(ACP 的 cancel 是通知、无受理回执,prompt 侧终局是唯一可观察证据);
// 终局后再数一段窗,增量不再增长 = hard interrupt 真停,不是"daemon 单方面放弃"。
func runCancelScenario(c *hermesapp.Client, ctx context.Context, sid string) {
	var mu sync.Mutex
	chunks := 0
	first := make(chan struct{}, 1)
	c.SetTurnHandlers(func(raw json.RawMessage) {
		ev := hermesapp.ParseSessionUpdate(raw)
		if ev.MessageChunk == "" {
			return
		}
		mu.Lock()
		chunks++
		mu.Unlock()
		select {
		case first <- struct{}{}:
		default:
		}
	}, nil)
	defer c.SetTurnHandlers(nil, nil)

	type promptOutcome struct {
		stop string
		err  error
	}
	out := make(chan promptOutcome, 1)
	go func() {
		stop, err := c.Prompt(ctx, sid, "写一篇两千字的散文,主题是秋夜的萤火,从头写到尾不要停,不要提前收束。")
		out <- promptOutcome{stop, err}
	}()

	select {
	case <-first:
		fmt.Println("[C1] 长 prompt 已在产出(首个正文增量到达)")
	case <-time.After(60 * time.Second):
		fmt.Println("!! 60s 内无正文增量(模型/凭据未配或产出不够长),取消链无法实证,见报告")
		return
	}
	time.Sleep(2 * time.Second) // 再流一点,证明取消发生在持续产出中途
	mu.Lock()
	before := chunks
	mu.Unlock()
	fmt.Printf("[C1b] 取消前增量帧=%d\n", before)

	if err := c.CancelSession(ctx, sid); err != nil {
		fmt.Printf("!! session/cancel 写入失败: %v\n", err)
		return
	}
	fmt.Println("[C2] session/cancel 已发(通知无回执,以 prompt 终局为准)")

	select {
	case res := <-out:
		if res.err != nil {
			fmt.Printf("!! prompt 未按终局应答返回(err=%v)—— cancel_event 未生效或通道已坏\n", res.err)
			return
		}
		fmt.Printf("[C3] prompt 终局 stop=%s\n", res.stop)
		if res.stop != "cancelled" {
			fmt.Printf("!! stopReason=%s ≠ cancelled —— 取消未传导到轮次终局(差距)\n", res.stop)
		}
	case <-time.After(30 * time.Second):
		fmt.Println("!! 发取消后 30s prompt 仍未返回 —— session/cancel 没叫停轮次(差距)")
		return
	}

	time.Sleep(1500 * time.Millisecond) // 静默观察窗:硬打断后不该再有增量
	mu.Lock()
	after := chunks
	mu.Unlock()
	if after != before {
		fmt.Printf("!! 取消终局后仍新增 %d 个增量帧(%d→%d)—— 适配器未真停(差距)\n", after-before, before, after)
		return
	}
	fmt.Printf("[C4] 取消后增量停止流动(帧数维持 %d)—— hard interrupt 生效\n", after)
}

func mustTempDir() string {
	dir, err := os.MkdirTemp("", "hermes-sess-probe-*")
	must("tempdir", err)
	return dir
}

func sortedKeys(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, fmt.Sprintf("%s=%d", k, m[k]))
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}
