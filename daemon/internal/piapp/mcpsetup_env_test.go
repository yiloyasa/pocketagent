// mcpsetup_env_test.go —— WriteCollabMcpConfigWithEnv 的回归用例(T-L2fix)。
//
// 钉三件事:
//  1. env 非空 → 条目落盘含 "env" 对象(键 POCKETAGENT_TOKEN 原样保留),
//     command/args 语义不变,他人条目仍原样保留;
//  2. env 为空 → 与既有 WriteCollabMcpConfig 逐字段等价(无 "env" 键,
//     行为零变化 —— T-L2pi-prod 已交付格式不漂移);
//  3. 隔离纪律:全程 t.TempDir(),不碰真实 ~/.pi 与 ~/.pocketagentd/pi-agent。
package piapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// readCollabEntry 读 mcp.json 里 pocketagent-collab 条目的原始 JSON。
func readCollabEntry(t *testing.T, dir string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "mcp.json"))
	if err != nil {
		t.Fatalf("读取 mcp.json 失败: %v", err)
	}
	var doc struct {
		Servers map[string]map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("mcp.json 不是合法 JSON: %v\n%s", err, raw)
	}
	entry, ok := doc.Servers[CollabMcpServerName]
	if !ok {
		t.Fatalf("缺 %s 条目: %s", CollabMcpServerName, raw)
	}
	return entry
}

func TestWriteCollabMcpConfigWithEnvWritesEnvObject(t *testing.T) {
	dir := t.TempDir()
	exe := `C:\fake\pocketagentd.exe`
	args := []string{"-collab-mcp", "-data", `C:\fake\data`}
	env := map[string]string{"POCKETAGENT_TOKEN": "deadbeef"}
	if err := WriteCollabMcpConfigWithEnv(dir, exe, args, env); err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	entry := readCollabEntry(t, dir)
	var gotEnv map[string]string
	if err := json.Unmarshal(entry["env"], &gotEnv); err != nil {
		t.Fatalf("env 不是对象: %v\n%s", err, entry["env"])
	}
	if gotEnv["POCKETAGENT_TOKEN"] != "deadbeef" {
		t.Fatalf("POCKETAGENT_TOKEN 应原样落盘: %v", gotEnv)
	}
	var gotArgs []string
	if err := json.Unmarshal(entry["args"], &gotArgs); err != nil || !reflect.DeepEqual(gotArgs, args) {
		t.Fatalf("args 应原样保留: %v err=%v", entry["args"], err)
	}
	var gotCmd string
	if err := json.Unmarshal(entry["command"], &gotCmd); err != nil || gotCmd != exe {
		t.Fatalf("command 应原样落盘: %s err=%v", entry["command"], err)
	}
	// 原子写不留残片。
	if leftovers := globTmp(t, dir); len(leftovers) != 0 {
		t.Fatalf("原子写后残留临时文件: %v", leftovers)
	}
}

func TestWriteCollabMcpConfigWithEnvEmptyEqualsLegacy(t *testing.T) {
	dir := t.TempDir()
	exe := `C:\fake\pocketagentd.exe`
	args := []string{"-collab-mcp"}
	// env 为空的 WithEnv 与既有函数:输出逐字段等价(无 env 键)。
	if err := WriteCollabMcpConfig(dir, exe, args); err != nil {
		t.Fatalf("既有函数写入失败: %v", err)
	}
	legacy := readCollabEntry(t, dir)

	dir2 := t.TempDir()
	if err := WriteCollabMcpConfigWithEnv(dir2, exe, args, nil); err != nil {
		t.Fatalf("空 env 写入失败: %v", err)
	}
	emptyEnv := readCollabEntry(t, dir2)

	if _, ok := emptyEnv["env"]; ok {
		t.Fatalf("空 env 不得写出 env 键(行为零变化): %v", emptyEnv)
	}
	for _, k := range []string{"command", "args"} {
		if string(legacy[k]) != string(emptyEnv[k]) {
			t.Fatalf("字段 %s 与既有函数输出不一致: %s vs %s", k, legacy[k], emptyEnv[k])
		}
	}
}
