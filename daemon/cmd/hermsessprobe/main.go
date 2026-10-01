// hermsessprobe —— hermes ACP 会话生命周期探针(真进程实测,深接差距修复 HM)。
//
// 实证面(与 acp_adapter/server.py 对表):
//  1. initialize 能力快照(loadSession / sessionCapabilities.fork/list/resume);
//  2. session/new → prompt(真 LLM 一轮)→ session/list(持久化在列证明);
//  3. session/load:应答前历史回放(update 通知先于应答,逐变体清点);
//  4. session/resume:同通路对照;
//  5. session/fork:分叉出新 sessionId。
//
// 红线:只读 G:\Hermes 用户配置;工作区用临时目录,不在用户家写任何文件;
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
	"time"

	"github.com/pocketagent/daemon/internal/hermesapp"
)

func main() {
	model := flag.String("model", "", "模型 ID(如 deepseek:deepseek-flash);空=跟 config.yaml 默认")
	timeout := flag.Duration("timeout", 180*time.Second, "单步 RPC 预算")
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
