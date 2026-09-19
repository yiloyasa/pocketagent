package zcodeapp

// snapshot.go 的单测:fixture 形状照抄实机探针证据
// daemon/docs/verify/evidence/zcode-p0-probe.md §4.2 与 zcode-p0-probe-raw.log
// (2026-09-18 实测),保证"解析器对着地面真值写",不是对着想象写。

import (
	"encoding/json"
	"math"
	"testing"
)

// snapshotFixture 按 §4.2 探针摘录拼的 session/create 应答快照:
// 两个模型(一个收图一个不收)、三个斜杠命令(raw.log 实测 goal/compact/init)、
// thoughtLevel 两档、projection 有用量。
const snapshotFixture = `{
  "messages": [],
  "projection": {"contextWindow": 200000, "mode": "build", "sessionId": "unknown", "status": "idle", "totalTokenCount": 50000},
  "session": {"createdAt": 1789753949180, "mode": "build", "sessionId": "sess_2188a268-test", "status": "idle", "title": ""},
  "settings": {
    "mode": {"current": "build"},
    "model": {"available": [
      {"ref": {"providerId": "p1", "modelId": "glm-5.2"}, "label": "GLM-5.2", "providerLabel": "zai",
       "contextWindow": 200000,
       "reasoning": {"levels": ["low", "medium", "high"], "defaultLevel": "medium"},
       "properties": {"inputFormat": {"supportsText": true, "supportsImage": true, "supportsVideo": false, "supportsAudio": false, "supportsPdf": false}}},
      {"ref": {"providerId": "p2", "modelId": "deepseek-v4-flash"}, "label": "DeepSeek V4 Flash", "providerLabel": "deepseek",
       "contextWindow": 1000000,
       "reasoning": {"levels": [], "defaultLevel": "max"},
       "properties": {"inputFormat": {"supportsText": true, "supportsImage": false, "supportsVideo": false, "supportsAudio": false, "supportsPdf": false}}}
    ], "current": {"ref": {"providerId": "p1", "modelId": "glm-5.2"}}},
    "permission": {"mode": "build"},
    "thoughtLevel": {"available": [{"label": "disabled", "value": "disabled"}, {"label": "enabled", "value": "enabled"}], "current": "enabled", "enabled": true}
  },
  "slashCommands": [
    {"description": "Show or set the current session goal.", "inputHint": "/goal [pause|resume|clear|replace <objective>|<objective>]", "name": "goal"},
    {"description": "Compact the session.", "inputHint": "/compact [instructions]", "name": "compact"},
    {"description": "Init.", "inputHint": "/init [notes]", "name": "init"}
  ]
}`

func mustParseFixture(t *testing.T) *SessionSnapshot {
	t.Helper()
	snap, err := ParseSessionSnapshot(json.RawMessage(snapshotFixture))
	if err != nil {
		t.Fatalf("解析 fixture 失败: %v", err)
	}
	return snap
}

func TestParseSessionSnapshotFull(t *testing.T) {
	snap := mustParseFixture(t)
	if snap.SessionID != "sess_2188a268-test" || snap.Title != "" {
		t.Fatalf("session 段解析错误: id=%q title=%q", snap.SessionID, snap.Title)
	}
	if snap.Projection.ContextWindow != 200000 || snap.Projection.TotalTokenCount != 50000 ||
		snap.Projection.Mode != "build" || snap.Projection.Status != "idle" {
		t.Fatalf("projection 解析错误: %+v", snap.Projection)
	}
	if len(snap.Models) != 2 {
		t.Fatalf("模型应有 2 项,得到 %d", len(snap.Models))
	}
	glm, ok := snap.ModelByID("glm-5.2")
	if !ok {
		t.Fatal("glm-5.2 应在快照模型表里")
	}
	// 审计 S-1 之 supportsImage:收图模型 true,不收图 false。
	if !glm.InputFormat.SupportsImage || !glm.InputFormat.SupportsText || glm.InputFormat.SupportsAudio {
		t.Fatalf("glm-5.2 inputFormat 解析错误: %+v", glm.InputFormat)
	}
	if glm.DefaultReasoningLevel != "medium" || glm.ProviderID != "p1" || glm.ContextWindow != 200000 {
		t.Fatalf("glm-5.2 展平字段错误: %+v", glm)
	}
	ds, _ := snap.ModelByID("deepseek-v4-flash")
	if ds.InputFormat.SupportsImage {
		t.Fatalf("deepseek-v4-flash 实测不收图,supportsImage 应为 false: %+v", ds.InputFormat)
	}
	// thoughtLevel:两档 + 当前档。
	if len(snap.ThoughtLevels) != 2 || snap.ThoughtLevels[0].Value != "disabled" ||
		snap.ThoughtLevels[1].Label != "enabled" {
		t.Fatalf("thoughtLevel.available 解析错误: %+v", snap.ThoughtLevels)
	}
	if snap.ThoughtLevelCurrent != "enabled" || !snap.ThoughtLevelEnabled {
		t.Fatalf("thoughtLevel current/enabled 解析错误: %q %v", snap.ThoughtLevelCurrent, snap.ThoughtLevelEnabled)
	}
	// slashCommands:三条,字段与 raw.log 实测一致。
	if len(snap.SlashCommands) != 3 {
		t.Fatalf("slashCommands 应有 3 条,得到 %d", len(snap.SlashCommands))
	}
	goal := snap.SlashCommands[0]
	if goal.Name != "goal" || goal.InputHint == "" || goal.Description == "" {
		t.Fatalf("slashCommands[0] 解析错误: %+v", goal)
	}
}

func TestContextUsagePercent(t *testing.T) {
	cases := []struct {
		name          string
		window, total int64
		want          float64
	}{
		{"窗口未知返回 0", 0, 50000, 0},
		{"无用量返回 0", 200000, 0, 0},
		{"普通比例", 200000, 50000, 25},
		{"负值按无用量处理", 200000, -1, 0},
		{"超出窗口封顶 100", 200000, 260000, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := &SessionSnapshot{}
			snap.Projection.ContextWindow = tc.window
			snap.Projection.TotalTokenCount = tc.total
			got := snap.ContextUsagePercent()
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("ContextUsagePercent()=%v, 期望 %v", got, tc.want)
			}
		})
	}
	// nil 接收者不炸(消费接线未完成前,调用方可能拿到空指针路径)。
	var nilSnap *SessionSnapshot
	if got := nilSnap.ContextUsagePercent(); got != 0 {
		t.Fatalf("nil 接收者应返回 0,得到 %v", got)
	}
}

func TestParseSessionSnapshotTolerant(t *testing.T) {
	// 全空对象:不报错,全部零值,占用率 0。
	snap, err := ParseSessionSnapshot(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("空对象不应报错: %v", err)
	}
	if snap.SessionID != "" || len(snap.Models) != 0 || len(snap.SlashCommands) != 0 ||
		snap.ContextUsagePercent() != 0 {
		t.Fatalf("空对象应全零值: %+v", snap)
	}
	// 缺 properties(老模型条目可能没有):零值放过,其余字段照常。
	partial, err := ParseSessionSnapshot(json.RawMessage(`{
		"session": {"sessionId": "sess_x"},
		"settings": {"model": {"available": [{"ref": {"providerId": "p", "modelId": "m"}}]}}
	}`))
	if err != nil {
		t.Fatalf("缺字段不应报错: %v", err)
	}
	if len(partial.Models) != 1 || partial.Models[0].ModelID != "m" || partial.Models[0].InputFormat != (InputFormat{}) {
		t.Fatalf("缺 properties 的模型条目应零值放过: %+v", partial.Models)
	}
	// 非法 JSON:必须报错(不能让坏帧装成空快照混过去)。
	if _, err := ParseSessionSnapshot(json.RawMessage(`{broken`)); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
}

func TestSlashCommandByName(t *testing.T) {
	snap := mustParseFixture(t)
	// 带斜杠、不带斜杠、大小写不敏感都要能查到,且查到的是对的那条。
	for _, tc := range []struct{ query, want string }{
		{"goal", "goal"}, {"/goal", "goal"}, {"GOAL", "goal"}, {"/Compact", "compact"},
	} {
		c, ok := snap.SlashCommandByName(tc.query)
		if !ok {
			t.Fatalf("%q 应能查到命令", tc.query)
		}
		if c.Name != tc.want {
			t.Fatalf("%q 应查到 %q,得到 %+v", tc.query, tc.want, c)
		}
	}
	if _, ok := snap.SlashCommandByName("nonexistent"); ok {
		t.Fatal("不存在的命令不应命中")
	}
	// nil 接收者不炸。
	var nilSnap *SessionSnapshot
	if _, ok := nilSnap.SlashCommandByName("goal"); ok {
		t.Fatal("nil 接收者不应命中")
	}
}
