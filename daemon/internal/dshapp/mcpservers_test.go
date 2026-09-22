package dshapp

// mcpservers_test.go —— dsh「会话级 MCP 接线」的回归钉子。
//
// 【钉的是什么】dsh 的 ACP 面允许**按会话**传 MCP 服务器(session/new 的
// mcpServers 参数)—— 这是"dsh 会话里的模型能派发子智能体"的唯一入口。
// 改造前恒传空数组,模型看不见 agent_delegate。本文件钉三件事:
//  1. 未接线时仍是**空数组**(不改变既有行为,旧路径零差异);
//  2. 接线后请求体确实带上(名/命令/参数/token 走 ACP 的 env 数组);
//  3. 读接口返回**快照**,调用方就地改写不会污染全局状态。

import "testing"

func TestSessionNewParamsEmptyByDefault(t *testing.T) {
	SetSessionMCPServers(nil)
	p := sessionNewParams("/tmp/work")
	if p["cwd"] != "/tmp/work" {
		t.Fatalf("cwd 应原样带上, got %v", p["cwd"])
	}
	servers, ok := p["mcpServers"].([]map[string]interface{})
	if !ok {
		t.Fatalf("mcpServers 必须是数组(ACP 要求), got %T", p["mcpServers"])
	}
	if len(servers) != 0 {
		t.Fatalf("未接线时应是空数组, got %v", servers)
	}
}

func TestSessionNewParamsCarriesWiredServers(t *testing.T) {
	t.Cleanup(func() { SetSessionMCPServers(nil) })
	SetSessionMCPServers([]map[string]interface{}{{
		"name":    "pocketagent-collab",
		"command": `C:\pa\pocketagentd.exe`,
		"args":    []string{"-collab-mcp", "-data", `C:\pa\data`},
		// env 必须是**对象 map** —— cmd/dshmcpcheck 实测:{name,value} 数组形状
		// 会被 dsh 以 -32603 Internal error 拒绝,session/new 根本建不起会话。
		"env": map[string]string{"POCKETAGENT_TOKEN": "t0k"},
	}})

	p := sessionNewParams("/tmp/work")
	servers, ok := p["mcpServers"].([]map[string]interface{})
	if !ok || len(servers) != 1 {
		t.Fatalf("接线后应带 1 条 MCP 服务器, got %v", p["mcpServers"])
	}
	if servers[0]["name"] != "pocketagent-collab" {
		t.Fatalf("服务器名不符, got %v", servers[0]["name"])
	}
	env, ok := servers[0]["env"].(map[string]string)
	if !ok || env["POCKETAGENT_TOKEN"] != "t0k" {
		t.Fatalf("token 应经 ACP 的 env 对象 map 下发(数组形状会被拒), got %v", servers[0]["env"])
	}
}

func TestSessionMCPServersReturnsSnapshot(t *testing.T) {
	t.Cleanup(func() { SetSessionMCPServers(nil) })
	SetSessionMCPServers([]map[string]interface{}{{"name": "x"}})

	got := SessionMCPServers()
	if len(got) != 1 || got[0]["name"] != "x" {
		t.Fatalf("应原样往返, got %v", got)
	}
	// 就地改写返回的快照,不得影响内部状态(否则会静默污染后续所有会话)。
	got[0]["name"] = "mutated"
	if SessionMCPServers()[0]["name"] != "x" {
		t.Fatal("读接口必须是快照语义,外部改写不得影响全局接线值")
	}
}
