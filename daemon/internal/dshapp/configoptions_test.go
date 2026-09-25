// configoptions_test.go —— 原生配置项目录解析与 set_config_option 参数名契约。
//
// 【红灯先行】rc.3 实测(docs/verify/evidence/dsh-config-persist-probe.md):
// session/set_config_option 的参数名是 configId,旧 optionId 被 -32602 拒绝 ——
// 假子进程按该 zod 行为回错,SetConfigOption 不改就会在这里红。
package dshapp

import (
	"encoding/json"
	"strings"
	"testing"
)

// 探针原帧逐字夹具(rc.3,docs/verify/evidence/dsh-p0-probe.md §2)。
const (
	flatOptionsJSON = `[
		{"value":"off","name":"Off","description":"Use for simple tasks that do not need reasoning."},
		{"value":"low","name":"Low","description":"Prefer for routine or latency-sensitive tasks."},
		{"value":"high","name":"High","description":"The default balance for most tasks."},
		{"value":"max","name":"Max","description":"Reserve for the hardest quality-first tasks."}]`
	groupedOptionsJSON = `[
		{"group":"deepseek-official","name":"DeepSeek","options":[
			{"value":"[\"deepseek-official\",\"deepseek-flash\"]","name":"DeepSeek-V41-Flash"},
			{"value":"[\"deepseek-official\",\"deepseek-v4-flash\"]","name":"DeepSeek-V4-Flash",
			 "description":"Fast, efficient, and economical; suited to focused, routine, or parallel tasks."}
		]}]`
)

func TestFlattenOptionLeaves(t *testing.T) {
	leaves := FlattenOptionLeaves(json.RawMessage(flatOptionsJSON))
	if len(leaves) != 4 {
		t.Fatalf("扁平候选应得 4 叶子,得 %d: %+v", len(leaves), leaves)
	}
	if leaves[0].Value != "off" || leaves[0].Name != "Off" || leaves[0].Group != "" {
		t.Fatalf("扁平叶子字段不符: %+v", leaves[0])
	}
	if leaves[3].Description == "" {
		t.Fatalf("叶子描述应原样保留: %+v", leaves[3])
	}

	g := FlattenOptionLeaves(json.RawMessage(groupedOptionsJSON))
	if len(g) != 2 {
		t.Fatalf("分组候选应展平 2 叶子,得 %d: %+v", len(g), g)
	}
	if g[0].Group != "deepseek-official" {
		t.Fatalf("分组名应记进叶子: %+v", g[0])
	}
	// 双层编码串必须逐字节原样(红线:任何加工都是污染)。
	if !strings.Contains(g[1].Value, "deepseek-v4-flash") {
		t.Fatalf("编码串被改写: %q", g[1].Value)
	}
	if json.Unmarshal([]byte(g[1].Value), new([]string)) != nil {
		t.Fatalf("value 应仍是合法双层编码串: %q", g[1].Value)
	}

	// 脏报文兜底:未知形状/空 → 空切片,不猜不炸。
	for _, dirty := range []json.RawMessage{nil, json.RawMessage(`{}`), json.RawMessage(`[1,2,3]`)} {
		if got := FlattenOptionLeaves(dirty); len(got) != 0 {
			t.Fatalf("脏报文应得空候选 %s,得 %+v", dirty, got)
		}
	}
}

// TestSetConfigOptionSendsConfigId 红灯:参数名必须是 rc.3 实测的 configId;
// 假子进程在缺 configId 时按 zod 行为回 -32602。
func TestSetConfigOptionSendsConfigId(t *testing.T) {
	c := startFake(t)
	if _, err := c.Initialize(t.Context()); err != nil {
		t.Fatalf("initialize 应成功: %v", err)
	}
	st, err := c.SessionNew(t.Context(), t.TempDir())
	if err != nil {
		t.Fatalf("session/new 应成功: %v", err)
	}
	if err := c.SetConfigOption(t.Context(), st.SessionID, "reasoning_effort", "low"); err != nil {
		t.Fatalf("configId 参数应被接受: %v", err)
	}
}
