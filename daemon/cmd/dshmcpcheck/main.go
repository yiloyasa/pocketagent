// dshmcpcheck —— 验证 dsh 的 ACP 面是否接受 session/new 的 mcpServers 参数,
// 以及它认的 McpServer 字段形状。
//
// 【为什么必须实测】daemon 要往每个 dsh 会话里挂一条本地 MCP 服务器
// (pocketagent-collab,让模型看得见 agent_delegate 从而能派发子智能体),而
// "ACP 的 McpServer 结构到底长什么样"只能从上游实现/实测确认 —— 猜错的话
// session/new 会直接失败,dsh 整条链路都建不起会话。本探针不猜:把几种候选
// 形状逐个真发一遍,如实记录应答。
//
// 用法:
//
//	go run ./cmd/dshmcpcheck
//
// 红线:只在系统临时目录建会话、只用本进程自己的 exe 充当 MCP 服务器(连接必然
// 失败也无妨 —— 本探针验证的是**参数形状是否被接受**,不是 MCP 是否连得上),
// 不碰用户 ~/.dsh,不写任何用户文件。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pocketagent/daemon/internal/dshapp"
)

func main() {
	log.SetFlags(0)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := dshapp.Start(dshapp.StartOptions{})
	if err != nil {
		log.Fatalf("拉起 dsh 失败: %v", err)
	}
	defer c.Close()

	if _, err := c.Initialize(ctx); err != nil {
		log.Fatalf("initialize 失败: %v", err)
	}
	fmt.Println("initialize 成功")

	work, err := os.MkdirTemp("", "dshmcpcheck-")
	if err != nil {
		log.Fatalf("建临时工作区失败: %v", err)
	}
	defer os.RemoveAll(work)
	exe, _ := os.Executable()

	// 候选形状(逐个试,记录哪一个被接受)。
	cases := []struct {
		name    string
		servers []map[string]interface{}
	}{
		{
			name: "A: env 为 {name,value} 数组(ACP 常见形状)",
			servers: []map[string]interface{}{{
				"name":    "pocketagent-collab",
				"command": exe,
				"args":    []string{"-collab-mcp", "-data", filepath.Dir(exe)},
				"env":     []map[string]string{{"name": "POCKETAGENT_TOKEN", "value": "probe"}},
			}},
		},
		{
			name: "B: env 为对象 map",
			servers: []map[string]interface{}{{
				"name":    "pocketagent-collab",
				"command": exe,
				"args":    []string{"-collab-mcp", "-data", filepath.Dir(exe)},
				"env":     map[string]string{"POCKETAGENT_TOKEN": "probe"},
			}},
		},
		{
			name: "C: 仅 name+command(最简)",
			servers: []map[string]interface{}{{
				"name":    "pocketagent-collab",
				"command": exe,
			}},
		},
	}

	for _, tc := range cases {
		dshapp.SetSessionMCPServers(tc.servers)
		st, err := c.SessionNew(ctx, work)
		if err != nil {
			fmt.Printf("  [%s] 拒绝: %v\n", tc.name, err)
			continue
		}
		out, _ := json.Marshal(map[string]interface{}{
			"sessionId": st.SessionID,
		})
		fmt.Printf("  [%s] 接受 -> %s\n", tc.name, out)
		if st.SessionID != "" {
			if cerr := c.SessionClose(ctx, st.SessionID); cerr != nil {
				fmt.Printf("       (清理会话失败,不影响结论: %v)\n", cerr)
			}
		}
	}
	dshapp.SetSessionMCPServers(nil)
	fmt.Println("完成:被接受的形状即为 daemon 应采用的那一种")
}
