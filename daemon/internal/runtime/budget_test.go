package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func i64p(v int64) *int64 { return &v }
func intp(v int) *int     { return &v }

// TestEffectiveOverridesBySpecificity 钉住逐级覆盖的顺序：
// global → agent → workspace → thread → collab，越具体越优先（规格 §6.5）。
func TestEffectiveOverridesBySpecificity(t *testing.T) {
	b := BudgetPolicy{
		Global:     Limits{RAMWorkingBytes: i64p(100), MaxParallelTurns: intp(3)},
		Agents:     map[string]Limits{"echo": {RAMWorkingBytes: i64p(200)}},
		Workspaces: map[string]Limits{"ws1": {RAMWorkingBytes: i64p(300)}},
		Threads:    map[string]Limits{"th1": {RAMWorkingBytes: i64p(400)}},
		Collabs:    map[string]Limits{"c1": {RAMWorkingBytes: i64p(500)}},
	}
	cases := []struct {
		agent, ws, th, collab string
		want                  int64
	}{
		{"", "", "", "", 100},
		{"echo", "", "", "", 200},
		{"echo", "ws1", "", "", 300},
		{"echo", "ws1", "th1", "", 400},
		{"echo", "ws1", "th1", "c1", 500},
		{"other", "ws-unknown", "th-unknown", "", 100}, // 未匹配 → global
	}
	for _, c := range cases {
		got := b.Effective(c.agent, c.ws, c.th, c.collab)
		if got.RAMWorkingBytes == nil || *got.RAMWorkingBytes != c.want {
			t.Fatalf("Effective(%q,%q,%q,%q) RAMWorking=%v want %d",
				c.agent, c.ws, c.th, c.collab, got.RAMWorkingBytes, c.want)
		}
		// 只配在 global 的字段必须逐级继承下来（覆盖是逐字段的，不是整块替换）。
		if got.MaxParallelTurns == nil || *got.MaxParallelTurns != 3 {
			t.Fatalf("未覆盖字段应继承 global，实得 %v", got.MaxParallelTurns)
		}
	}
}

// TestEffectiveDistinguishesUnsetFromExplicitZero 钉住 nil 与显式 0 的区别：
// nil = 该级未设置（继承上一级）；0 = 明令封禁该类资源。
//
// 【为什么这条是硬要求】用零值当哨兵会让 `max_parallel_turns: 0` 被读成
// 「未设置」而继承全局的 3 —— 用户明明关了并发，系统还在并行。
func TestEffectiveDistinguishesUnsetFromExplicitZero(t *testing.T) {
	b := BudgetPolicy{
		Global: Limits{MaxParallelTurns: intp(3)},
		Agents: map[string]Limits{"frozen": {MaxParallelTurns: intp(0)}},
	}
	got := b.Effective("frozen", "", "", "")
	if got.MaxParallelTurns == nil {
		t.Fatal("显式 0 被当成未设置（nil）—— 用户关了并发却会继承全局值")
	}
	if *got.MaxParallelTurns != 0 {
		t.Fatalf("显式 0 应生效，实得 %d", *got.MaxParallelTurns)
	}
}

// TestBudgetJSONShape 钉住规格 §6.5 的 JSON 形状可直接读写（外部可编辑的配置面）。
func TestBudgetJSONShape(t *testing.T) {
	raw := `{
	  "global": {"ram_working_bytes": 8388608, "max_parallel_turns": 3},
	  "agents": {"pi": {"ram_working_bytes": 4194304, "max_parallel_turns": 1}}
	}`
	var b BudgetPolicy
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	got := b.Effective("pi", "", "", "")
	if got.RAMWorkingBytes == nil || *got.RAMWorkingBytes != 4194304 {
		t.Fatalf("agent 覆盖未生效: %v", got.RAMWorkingBytes)
	}
	if got.MaxParallelTurns == nil || *got.MaxParallelTurns != 1 {
		t.Fatalf("agent 的 max_parallel_turns 未生效: %v", got.MaxParallelTurns)
	}
}

// TestDefaultLimitsAreCompleteAndPositive 内置基线必须齐备且为正数：
// 缺一项会让该资源无上限（回到"无界缓冲"），配成 0 会静默封禁一切。
func TestDefaultLimitsAreCompleteAndPositive(t *testing.T) {
	d := DefaultLimits()
	for name, v := range map[string]*int64{
		"RAMWorkingBytes":       d.RAMWorkingBytes,
		"RAMEventBufferBytes":   d.RAMEventBufferBytes,
		"RAMSessionIndexBytes":  d.RAMSessionIndexBytes,
		"DiskConversationBytes": d.DiskConversationBytes,
		"DiskCacheBytes":        d.DiskCacheBytes,
		"DiskArtifactBytes":     d.DiskArtifactBytes,
	} {
		if v == nil {
			t.Fatalf("内置基线缺字段 %s（缺项 = 该类资源无上限）", name)
		}
		if *v <= 0 {
			t.Fatalf("内置基线 %s=%d 应为正数", name, *v)
		}
	}
	if d.MaxParallelTurns == nil || *d.MaxParallelTurns <= 0 {
		t.Fatalf("内置基线 MaxParallelTurns 应为正数: %v", d.MaxParallelTurns)
	}
}

// TestLoadBudgetMissingFileUsesDefault：没有配置文件是绝大多数机器的正常状态，
// 必须安静地用内置基线（而不是报错、也不是给一份零值配置）。
func TestLoadBudgetMissingFileUsesDefault(t *testing.T) {
	b, err := LoadBudgetPolicy(t.TempDir())
	if err != nil {
		t.Fatalf("缺配置文件不应报错: %v", err)
	}
	got := b.Effective("whatever", "", "", "")
	def := DefaultLimits()
	if got.RAMWorkingBytes == nil || *got.RAMWorkingBytes != *def.RAMWorkingBytes {
		t.Fatalf("缺文件时应等于内置基线: %v vs %v", got.RAMWorkingBytes, def.RAMWorkingBytes)
	}
}

// TestLoadBudgetCorruptIsFailSoftButLoud：配置坏掉时不能让 daemon 起不来
// （预算不是安全边界），但也**绝不能静默给零值** —— 零值等于封禁一切资源。
func TestLoadBudgetCorruptIsFailSoftButLoud(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "runtime-policy.json"), []byte("{ 坏掉的配置"), 0o600); err != nil {
		t.Fatalf("造文件失败: %v", err)
	}
	b, err := LoadBudgetPolicy(dir)
	if err == nil {
		t.Fatal("损坏配置应回报错误（供调用方出声），而不是装作没发生")
	}
	// 但仍要给出可用的预算 —— 且是默认值，不是零值。
	got := b.Effective("any", "", "", "")
	if got.RAMWorkingBytes == nil || *got.RAMWorkingBytes <= 0 {
		t.Fatalf("损坏时应回退内置基线，实得 %v（零值会静默封禁一切资源）", got.RAMWorkingBytes)
	}
	// 原文件不得被覆盖（保留证据供人工修）。
	raw, rerr := os.ReadFile(filepath.Join(dir, "runtime-policy.json"))
	if rerr != nil || string(raw) != "{ 坏掉的配置" {
		t.Fatalf("损坏的配置文件被改写了: %q (err=%v)", raw, rerr)
	}
}

// TestLoadBudgetReadsFile：正常文件按内容生效。
func TestLoadBudgetReadsFile(t *testing.T) {
	dir := t.TempDir()
	body := `{"global":{"ram_working_bytes":123456},"agents":{"echo":{"max_parallel_turns":7}}}`
	if err := os.WriteFile(filepath.Join(dir, "runtime-policy.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
	b, err := LoadBudgetPolicy(dir)
	if err != nil {
		t.Fatalf("正常文件不应报错: %v", err)
	}
	got := b.Effective("echo", "", "", "")
	if got.RAMWorkingBytes == nil || *got.RAMWorkingBytes != 123456 {
		t.Fatalf("global 未生效: %v", got.RAMWorkingBytes)
	}
	if got.MaxParallelTurns == nil || *got.MaxParallelTurns != 7 {
		t.Fatalf("agent 未生效: %v", got.MaxParallelTurns)
	}
	// 文件未覆盖的字段应补齐内置基线（否则缺项 = 无上限）。
	if got.DiskArtifactBytes == nil || *got.DiskArtifactBytes <= 0 {
		t.Fatalf("文件未提的字段应补内置基线: %v", got.DiskArtifactBytes)
	}
}

// TestBudgetSourceTracksOrigin：能回答「这个数字是哪一级给的」——
// 预算争议（为什么是 3？）只能靠溯源自证，不能靠猜。
func TestBudgetSourceTracksOrigin(t *testing.T) {
	b := BudgetPolicy{
		Global: Limits{MaxParallelTurns: intp(3)},
		Agents: map[string]Limits{"echo": {RAMWorkingBytes: i64p(1)}},
	}
	_, origins := b.EffectiveWithSource("echo", "", "", "")
	if origins["MaxParallelTurns"] != ScopeGlobal() {
		t.Fatalf("MaxParallelTurns 应溯源到 global，实得 %q", origins["MaxParallelTurns"])
	}
	if origins["RAMWorkingBytes"] != ScopeAgent("echo") {
		t.Fatalf("RAMWorkingBytes 应溯源到 agent:echo，实得 %q", origins["RAMWorkingBytes"])
	}
}
