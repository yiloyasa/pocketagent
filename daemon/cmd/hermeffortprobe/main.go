// hermeffortprobe —— hermes 强度写入时机优化的真进程三件套实证探针。
//
// 完全走 daemon 代码路径(agent.HermesProvider.Run,非探针自拼 RPC):
//  1. 完整一轮:新建会话 + 真 LLM prompt,正文回流(网络面恢复证明);
//  2. 强度写入:thread.Effort=high —— 新建会话当轮 STATUS"本轮已生效";
//     同线程第二轮(内存缓存命中 = 重挂载情形)STATUS"下轮会话生效";
//  3. 切模型:thread.ModelID=hermes/deepseek-flash → session 内 SetSessionModel,
//     以独立适配器 session/load 回读该会话的 CurrentModelID 作正向证据。
//
// 红线:HERMES_HOME 用 -home 指到沙箱(强度写入落沙箱 config.yaml,绝不碰
// hermes 家目录(见 HERMES_HOME));工作区用临时目录;凭据文件由调用方预先拷入沙箱(本探针只读)。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/store"
)

func main() {
	home := flag.String("home", "", "HERMES_HOME 沙箱(必填;强度写入落这里)")
	effort := flag.String("effort", "high", "强度档位")
	model := flag.String("model", "hermes/deepseek-flash", "thread.ModelID(切模型面)")
	flag.Parse()
	if *home == "" {
		fmt.Println("!! -home 必填(HERMES_HOME 沙箱)")
		os.Exit(1)
	}
	os.Setenv("HERMES_HOME", *home) // SetReasoningEffort + 适配器同读此环境

	p := agent.NewHermesProvider()
	ws := mustTempDir()
	defer os.RemoveAll(ws)

	runTurn := func(tag, text string, th *store.Thread) (string, []*pb.TurnServerMessage) {
		var events []*pb.TurnServerMessage
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		blocks, _, err := p.Run(ctx, agent.TurnRequest{
			Thread:      th,
			InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}}}},
			Workspace:   ws,
		}, func(ev *pb.TurnServerMessage) {
			events = append(events, ev)
			if ce := ev.GetCodexEvent(); ce != nil && ce.Title == "hermes · 思考强度" {
				fmt.Printf("[%s] STATUS %s | %s\n", tag, ce.Code, ce.Message)
			}
		}, nil)
		if err != nil {
			fmt.Printf("!! [%s] run 失败: %v\n", tag, err)
			os.Exit(1)
		}
		var reply strings.Builder
		for i := range blocks {
			if t := blocks[i].GetText(); t != nil {
				reply.WriteString(t.Text)
			}
		}
		fmt.Printf("[%s] 轮次完成 reply=%q external=%q\n", tag,
			strings.TrimSpace(reply.String()), th.ExternalSession.SessionID)
		return strings.TrimSpace(reply.String()), events
	}

	// 【1+2】新建会话:完整一轮 + 强度写入(当轮生效情形)。
	th := &store.Thread{ID: "probe-t1", Effort: *effort, ModelID: *model}
	reply1, _ := runTurn("T1", "只回复两个字符:pong", th)
	if reply1 == "" {
		fmt.Println("!! T1 正文为空(终局语义 = 真失败)")
		os.Exit(1)
	}
	fmt.Printf("[T1] 沙箱 config.yaml:\n%s\n", mustRead(filepath.Join(*home, "config.yaml")))

	// 【2 续】同线程第二轮:内存缓存命中(重挂载情形)→ 文案应转"下轮会话生效"。
	reply2, _ := runTurn("T2", "再回复两个字符:pong", th)
	if reply2 == "" {
		fmt.Println("!! T2 正文为空(终局语义 = 真失败)")
		os.Exit(1)
	}

	// 【3】切模型正向证据:独立适配器 session/load 回读该会话的当前模型。
	py, dir, err := hermesapp.LocateHermes()
	must("locate", err)
	c, err := hermesapp.Start(py, dir)
	must("start", err)
	defer c.Close()
	initCtx, initCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer initCancel()
	if _, err := c.Initialize(initCtx); err != nil {
		must("initialize", err)
	}
	loaded, err := c.LoadSession(initCtx, ws, th.ExternalSession.SessionID)
	if err != nil {
		fmt.Printf("!! session/load 回读失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[M] session/load CurrentModelID=%q(期望 deepseek:deepseek-flash)\n", loaded.Models.CurrentModelID)
	if loaded.Models.CurrentModelID != "deepseek:deepseek-flash" {
		fmt.Println("!! 切模型未落到会话(差距)")
		os.Exit(1)
	}

	fmt.Println("== hermeffortprobe 三件套完成 ==")
}

func must(stage string, err error) {
	if err != nil {
		fmt.Printf("!! %s: %v\n", stage, err)
		os.Exit(1)
	}
}

func mustTempDir() string {
	dir, err := os.MkdirTemp("", "hermeffortprobe-ws")
	must("tempdir", err)
	return dir
}

func mustRead(path string) string {
	raw, err := os.ReadFile(path)
	must("read "+path, err)
	return string(raw)
}
