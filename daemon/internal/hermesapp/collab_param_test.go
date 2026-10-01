package hermesapp

// collab_param_test.go —— 协同 MCP 会话参数组装的回归钉子(X2 协同面收口):
// 来源未接(nil)时四个会话方法原样发空数组(与改造前逐字一致);来源接线后
// 发注入条目。适配器侧对非空 mcpServers 的语义(register_mcp_servers 真注册)
// 由 acp_adapter/server.py 实证背书,不在本包重复钉。

import (
	"testing"
)

func TestSessionMCPServersParamNilSourceIsEmptyArray(t *testing.T) {
	prev := SessionMCPServersSource
	SessionMCPServersSource = nil
	t.Cleanup(func() { SessionMCPServersSource = prev })

	got := sessionMCPServersParam()
	if len(got) != 0 {
		t.Fatalf("来源未接时应发空数组(零行为变化),实得 %v", got)
	}
}

func TestSessionMCPServersParamCarriesEntriesWhenWired(t *testing.T) {
	prev := SessionMCPServersSource
	SessionMCPServersSource = func() []map[string]any {
		return []map[string]any{{
			"name": "pocketagent-collab", "command": "daemon.exe",
			"args": []string{"-collab-mcp"}, "env": []any{},
		}}
	}
	t.Cleanup(func() { SessionMCPServersSource = prev })

	got := sessionMCPServersParam()
	if len(got) != 1 {
		t.Fatalf("接线后应带恰好一条条目,实得 %d", len(got))
	}
	entry, ok := got[0].(map[string]any)
	if !ok || entry["name"] != "pocketagent-collab" {
		t.Fatalf("条目形状不对: %v", got[0])
	}
}

func TestSessionMCPServersParamEmptySourceFallsBack(t *testing.T) {
	prev := SessionMCPServersSource
	SessionMCPServersSource = func() []map[string]any { return nil }
	t.Cleanup(func() { SessionMCPServersSource = prev })

	if got := sessionMCPServersParam(); len(got) != 0 {
		t.Fatalf("来源返回空时也应发空数组,实得 %v", got)
	}
}
