package agent

// collab_inject_test.go —— codex / zcode / hermes 协同 MCP 注入的回归钉子
// (X2 协同面收口)。钉四件事:
//
//  1. 注入要素的开关两态:数据目录未注入 = 一律不注入、CollabReady 如实报暗
//     (绝不摆"开关开着就算接线"的假可用);注入后三家判据齐亮。
//  2. codex 的 argv 覆盖形状:`-c` 键值成对、TOML 字面量合法(反斜杠转义,
//     Windows 路径不破形)。
//  3. zcode 会话请求真的带上 mcpServers 条目(session/create 与 session/resume
//     两个唯一实现调用点);未接线时不带该键(旧版 zcode strict schema 兼容)。
//  4. hermes 会话请求的 mcpServers 来源接线(agent 包 init → hermesapp)。
//
// 【夹具】全部走注入式假客户端/临时数据目录,不拉真子进程;结束时恢复全局态
// (runtimeDataDir / hermesapp.SessionMCPServersSource),不污染别的用例。

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/store"
)

// withRuntimeDataDir 临时注入数据目录,返回恢复函数(测试内 defer)。
func withRuntimeDataDir(t *testing.T, dir string) {
	t.Helper()
	prev := RuntimeDataDir()
	SetRuntimeDataDir(dir)
	t.Cleanup(func() { SetRuntimeDataDir(prev) })
}

// TestCollabInjectOffWithoutDataDir 注入要素不齐(数据目录未注入)= 三家
// CollabReady 一律报暗、会话参数不带键 —— CollabAware 合同的"不摆假可用"。
func TestCollabInjectOffWithoutDataDir(t *testing.T) {
	withRuntimeDataDir(t, "") // 显式清空,防其他用例经全局态串入

	if e := CollabSessionMCPEntry(); e != nil {
		t.Fatalf("数据目录未注入时不得产出 MCP 条目: %v", e)
	}
	if collabCodexExtraArgs() != nil {
		t.Fatal("数据目录未注入时 codex argv 不得追加协同覆盖")
	}
	if (NewCodexProvider()).CollabReady() {
		t.Fatal("codex 未接线时 CollabReady 不得报真(假可用)")
	}
	if (NewZcodeProvider()).CollabReady() {
		t.Fatal("zcode 未接线时 CollabReady 不得报真(假可用)")
	}
	if (NewHermesProvider()).CollabReady() {
		t.Fatal("hermes 未接线时 CollabReady 不得报真(假可用)")
	}
}

// TestCollabInjectOnWithWiring 注入要素齐备 = 三家判据齐亮,条目形状对齐
// zcode/ACP 双方的公共子集(name/command/args/env 四键,env 空数组)。
func TestCollabInjectOnWithWiring(t *testing.T) {
	withRuntimeDataDir(t, t.TempDir())

	entry := CollabSessionMCPEntry()
	if entry == nil {
		t.Fatal("接线成立时应产出 MCP 条目")
	}
	if entry["name"] != "pocketagent-collab" {
		t.Fatalf("条目 name 应为共享服务器名,实得 %v", entry["name"])
	}
	args, _ := entry["args"].([]string)
	if len(args) != 3 || args[0] != "-collab-mcp" || args[1] != "-data" || args[2] == "" {
		t.Fatalf("条目 args 应为 [-collab-mcp -data <数据目录>],实得 %v", args)
	}
	if _, ok := entry["env"]; !ok {
		t.Fatal("zcode strict schema 要求 env 键在场(空数组)")
	}

	if !(NewCodexProvider()).CollabReady() {
		t.Fatal("接线成立后 codex CollabReady 应报真")
	}
	if !(NewZcodeProvider()).CollabReady() {
		t.Fatal("接线成立后 zcode CollabReady 应报真")
	}
	if !(NewHermesProvider()).CollabReady() {
		t.Fatal("接线成立后 hermes CollabReady 应报真")
	}
}

// TestCollabCodexExtraArgsShape codex argv 覆盖的形状:`-c` 键值成对追加,
// 值是合法 TOML 字面量(反斜杠转义成 \\,数组元素逐个带引号)。
func TestCollabCodexExtraArgsShape(t *testing.T) {
	withRuntimeDataDir(t, `C:\tmp\pa-data`) // 含反斜杠:钉 Windows 路径不破 TOML

	got := collabCodexExtraArgs()
	// 形状逐项断言(可执行文件路径进程而异,不能整串比对)。
	if len(got) != 4 {
		t.Fatalf("应恰好两对 -c 覆盖(4 个 argv 元素),实得 %v", got)
	}
	if got[0] != "-c" || got[2] != "-c" {
		t.Fatalf("-c 键必须与值成对,实得 %v", got)
	}
	cmdOverride := got[1]
	if !strings.HasPrefix(cmdOverride, "mcp_servers.pocketagent-collab.command=") {
		t.Fatalf("第一覆盖应指向 command 键,实得 %q", cmdOverride)
	}
	// TOML 字面量合法性:命令路径里每个真实反斜杠都被转义(不含裸 \)。
	valuePart := strings.TrimPrefix(cmdOverride, "mcp_servers.pocketagent-collab.command=")
	if !strings.HasPrefix(valuePart, `"`) || !strings.HasSuffix(valuePart, `"`) {
		t.Fatalf("command 值必须是 TOML basic string,实得 %q", valuePart)
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(valuePart, `"`), `"`)
	if strings.Count(inner, `\\`)*2 != strings.Count(inner, `\`) {
		t.Fatalf("反斜杠必须成对转义(值内不应出现裸 \\),实得 %q", inner)
	}
	argsOverride := got[3]
	if !strings.HasPrefix(argsOverride, "mcp_servers.pocketagent-collab.args=[") ||
		!strings.HasSuffix(argsOverride, `"]`) {
		t.Fatalf("第二覆盖应指向 args 键且为字符串数组,实得 %q", argsOverride)
	}
	if !strings.Contains(argsOverride, `"-collab-mcp"`) || !strings.Contains(argsOverride, `"-data"`) {
		t.Fatalf("args 数组应含 -collab-mcp 与 -data,实得 %q", argsOverride)
	}
}

// TestTOMLStringEscapes tomlString 的转义表(基本面 + 控制字符)。
func TestTOMLStringEscapes(t *testing.T) {
	cases := []struct{ in, want string }{
		{`C:\x`, `"C:\\x"`},
		{`say "hi"`, `"say \"hi\""`},
		{"a\nb", `"a\nb"`},
		{"tab\there", `"tab\there"`},
		{"中文", `"中文"`}, // 非 ASCII 原样保留
		{"ctl\x01", `"ctl\u0001"`},
	}
	for _, c := range cases {
		if got := tomlString(c.in); got != c.want {
			t.Errorf("tomlString(%q) = %s, want %s", c.in, got, c.want)
		}
	}
	if got := tomlStringArray([]string{"a", `b\c`}); got != `["a","b\\c"]` {
		t.Errorf("tomlStringArray = %s", got)
	}
}

// fakeMCPZcodeClient 捕获会话请求参数的 zcodeSessionClient 打桩。
type fakeMCPZcodeClient struct {
	paramsByMethod map[string]map[string]interface{}
}

func (f *fakeMCPZcodeClient) CallCtx(_ context.Context, method string, params interface{}) (json.RawMessage, error) {
	if m, ok := params.(map[string]interface{}); ok {
		if f.paramsByMethod == nil {
			f.paramsByMethod = map[string]map[string]interface{}{}
		}
		f.paramsByMethod[method] = m
	}
	return json.RawMessage(`{"session":{"sessionId":"sess-mcp"}}`), nil
}

// TestZcodeSessionCarriesCollabMCP 接线成立时,zcode 的 session/create 与
// session/resume 两个唯一实现调用点都要带上 pocketagent-collab 条目。
func TestZcodeSessionCarriesCollabMCP(t *testing.T) {
	withRuntimeDataDir(t, t.TempDir())
	p := NewZcodeProvider()
	fake := &fakeMCPZcodeClient{}
	p.sessionSource = func(context.Context) (zcodeSessionClient, error) { return fake, nil }
	th := &store.Thread{ID: "t1", Workspace: `C:\ws`}

	if _, _, _, err := p.createZcodeSession(context.Background(), fake, th, `C:\ws`, 1); err != nil {
		t.Fatalf("createZcodeSession: %v", err)
	}
	if _, err := p.resumeZcodeSession(context.Background(), fake, th.ID, "sess-old", 1); err != nil {
		t.Fatalf("resumeZcodeSession: %v", err)
	}
	for _, method := range []string{"session/create", "session/resume"} {
		m := fake.paramsByMethod[method]
		if m == nil {
			t.Fatalf("%s 应被发出", method)
		}
		servers, ok := m["mcpServers"].([]any)
		if !ok || len(servers) != 1 {
			t.Fatalf("%s 应带恰好一条 mcpServers,实得 %v", method, m["mcpServers"])
		}
		entry, _ := servers[0].(map[string]any)
		if entry["name"] != "pocketagent-collab" {
			t.Fatalf("%s 的条目 name 不对: %v", method, entry["name"])
		}
	}
}

// TestZcodeSessionNoMCPKeyWhenUnwired 未接线时请求参数不得出现 mcpServers 键
// (旧版 zcode strict schema 不认未知键;不带 = 与改造前零差异)。
func TestZcodeSessionNoMCPKeyWhenUnwired(t *testing.T) {
	withRuntimeDataDir(t, "")
	p := NewZcodeProvider()
	fake := &fakeMCPZcodeClient{}
	p.sessionSource = func(context.Context) (zcodeSessionClient, error) { return fake, nil }

	if _, _, _, err := p.createZcodeSession(context.Background(), fake, &store.Thread{ID: "t1", Workspace: `C:\ws`}, "C:\\ws", 1); err != nil {
		t.Fatalf("createZcodeSession: %v", err)
	}
	if m := fake.paramsByMethod["session/create"]; m == nil {
		t.Fatal("session/create 应被发出")
	} else if _, exists := m["mcpServers"]; exists {
		t.Fatalf("未接线时不得携带 mcpServers 键: %v", m["mcpServers"])
	}
}

// TestHermesSessionMCPSourceWiring agent 包 init 已把 hermesapp 的
// mcpServers 来源接上:接线成立返回恰好一条条目,未接线返回 nil(适配器侧
// 原样发空数组,零行为变化)。
func TestHermesSessionMCPSourceWiring(t *testing.T) {
	if hermesapp.SessionMCPServersSource == nil {
		t.Fatal("agent 包应已把 hermesapp.SessionMCPServersSource 接上(init)")
	}
	withRuntimeDataDir(t, t.TempDir())
	entries := hermesapp.SessionMCPServersSource()
	if len(entries) != 1 {
		t.Fatalf("接线成立应返回恰好一条条目,实得 %d", len(entries))
	}
	if entries[0]["name"] != "pocketagent-collab" {
		t.Fatalf("条目 name 不对: %v", entries[0]["name"])
	}
	wantKeys := []string{"name", "command", "args", "env"}
	for _, k := range wantKeys {
		if _, ok := entries[0][k]; !ok {
			t.Fatalf("条目缺键 %s(ACP/zcode 公共形状四键): %v", k, entries[0])
		}
	}

	withRuntimeDataDir(t, "")
	if got := hermesapp.SessionMCPServersSource(); got != nil {
		t.Fatalf("未接线应返回 nil,实得 %v", got)
	}
}

