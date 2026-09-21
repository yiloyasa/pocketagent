// pi_collab_wire_test.go —— pi 侧协作 MCP 接线(T-FU)开关两态的单测。
//
// 【注入方式】piapp.CollabEnabled 的开关经导出的 piapp.SetCollabEnabled 注入
// (cmd 层 -pi-collab flag → SetCollabEnabled 的同款入口);生产默认关。
// 【隔离方式】PI_CODING_AGENT_DIR 指向临时目录(t.Setenv 自动还原),并预置
// "adapter 已装"痕迹(settings.json 的 packages 列表含 npm:pi-mcp-adapter,
// AdapterInstalled 的验收证据之一)—— SetupCollabAgentDir 走幂等跳过路径,
// 不触网、不起真 pi。测试全程不碰真实 ~/.pi 与 ~/.pocketagentd/pi-agent。
package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pocketagent/daemon/internal/piapp"
)

// collabTestDir 隔离的 agent 目录 + 已装 adapter 痕迹,返回目录路径。
func collabTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	st := `{"packages":["npm:pi-mcp-adapter"]}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(st), 0o644); err != nil {
		t.Fatalf("预置 settings.json 失败: %v", err)
	}
	return dir
}

func TestPiCollabWireEnabledWritesMcpConfig(t *testing.T) {
	dir := collabTestDir(t)
	piapp.SetCollabEnabled(true)
	t.Cleanup(func() { piapp.SetCollabEnabled(false) })

	p := NewPiProvider()
	p.ensureCollabMcp()

	raw, err := os.ReadFile(filepath.Join(dir, "mcp.json"))
	if err != nil {
		t.Fatalf("开关开启时应写出 mcp.json: %v", err)
	}
	var doc struct {
		Servers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("mcp.json 应为合法 JSON: %v\n%s", err, raw)
	}
	entry, ok := doc.Servers[piapp.CollabMcpServerName]
	if !ok {
		t.Fatalf("应含 %s 条目: %s", piapp.CollabMcpServerName, raw)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable 失败: %v", err)
	}
	if want := filepath.Clean(exe); filepath.Clean(entry.Command) != want {
		t.Fatalf("command 应为 daemon 自身(test 二进制)绝对路径: got %s want %s", entry.Command, want)
	}
	if len(entry.Args) != 1 || entry.Args[0] != "-collab-mcp" {
		t.Fatalf(`args 应为 ["-collab-mcp"](指向前端 -collab-mcp 子模式): %v`, entry.Args)
	}
}

func TestPiCollabWireDisabledIsNoop(t *testing.T) {
	dir := collabTestDir(t)
	piapp.SetCollabEnabled(false)
	t.Cleanup(func() { piapp.SetCollabEnabled(false) })

	p := NewPiProvider()
	p.ensureCollabMcp()

	if _, err := os.Stat(filepath.Join(dir, "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("开关关闭时不得写 mcp.json(零行为变化): %v", err)
	}
}

func TestPiCollabWireSetupFailureIsLoggedNotFatal(t *testing.T) {
	// PI_CODING_AGENT_DIR 指向普通文件:MkdirAll 必败 → ensureCollabMcp 仅日志
	// 返回(不 panic、不写 mcp.json),pi 主功能不受影响的容错纪律。
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("准备失败路径: %v", err)
	}
	t.Setenv("PI_CODING_AGENT_DIR", file)
	piapp.SetCollabEnabled(true)
	t.Cleanup(func() { piapp.SetCollabEnabled(false) })

	p := NewPiProvider()
	p.ensureCollabMcp() // 不得 panic

	if _, err := os.Stat(filepath.Join(file, "mcp.json")); err == nil {
		t.Fatal("初始化失败时不得写出 mcp.json")
	}
}
