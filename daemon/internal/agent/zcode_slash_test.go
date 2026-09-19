package agent

// zcode_slash_test.go —— 斜杠命令账本通路的单测(零改动升级口的落地钉子)。
//
// 【钉什么】zcode.go 把 slashCommands 随 create/resume/subscribe 三路快照记进
// 线程账本后,server 的 zcodeSlashSource 断言即接管面板查询:
//   - 记账后 SlashCommandsFor 命中,且 name/description/inputHint 三字段完整
//     (与 server 侧 fetchViaZcode 的映射一一对应);
//   - 未记账返回 nil(server 自然回退到一次性子进程 resume 兜底);
//   - 快照不带 slashCommands 字段(如 subscribe 的快照段)不冲掉已记的账;
//   - 显式空数组是"确认为空"的真记账,冲掉旧值(会话命令真的没了)。
//
// 快照形状与 zcodeapp/snapshot.go 的解析层共用地面真值(探针 raw.log)。

import (
	"encoding/json"
	"testing"
)

// 带斜杠命令与 projection 的快照应答(形状 = ParseSessionSnapshot 消费的 wire 格式)。
var zcodeSlashSnapshot = json.RawMessage(`{
	"session": {"sessionId": "sess_slash1"},
	"projection": {"contextWindow": 200000, "totalTokenCount": 19850},
	"slashCommands": [
		{"name": "goal", "description": "常驻目标", "inputHint": "/goal [pause|resume|…]"},
		{"name": "compact", "description": "历史压缩", "inputHint": ""}
	]
}`)

func TestZcodeSlashLedgerHitAfterRecord(t *testing.T) {
	p := NewZcodeProvider()
	p.recordProjection("th1", zcodeSlashSnapshot)
	got := p.SlashCommandsFor("th1")
	if len(got) != 2 {
		t.Fatalf("记账后应命中 2 条命令,得到 %d 条", len(got))
	}
	// 字段完整钉子:name/description/inputHint 一一透传,空 inputHint 保住空串。
	if got[0].Name != "goal" || got[0].Description != "常驻目标" || got[0].InputHint != "/goal [pause|resume|…]" {
		t.Fatalf("第一条字段映射不符: %+v", got[0])
	}
	if got[1].Name != "compact" || got[1].Description != "历史压缩" || got[1].InputHint != "" {
		t.Fatalf("第二条字段映射不符: %+v", got[1])
	}
	// 账本扩展不得破坏原 projection 记账。
	if w := p.contextWindowFor("th1"); w != 200000 {
		t.Fatalf("projection 记账被破坏: contextWindow=%d", w)
	}
}

func TestZcodeSlashLedgerMissReturnsNil(t *testing.T) {
	p := NewZcodeProvider()
	// 未记账:返回 nil,server 端自然回退到一次性子进程 resume 兜底路径。
	if got := p.SlashCommandsFor("th-never-seen"); got != nil {
		t.Fatalf("未记账应返回 nil,得到 %+v", got)
	}
	// 坏 JSON 与无命令字段的快照都不记账(字段缺失是常态,静默跳过)。
	p.recordProjection("th2", json.RawMessage(`{broken`))
	p.recordProjection("th3", json.RawMessage(`{"projection":{"contextWindow":200000}}`))
	if got := p.SlashCommandsFor("th2"); got != nil {
		t.Fatalf("坏快照不应记账,得到 %+v", got)
	}
	if got := p.SlashCommandsFor("th3"); got != nil {
		t.Fatalf("无命令字段不应记账,得到 %+v", got)
	}
}

func TestZcodeSlashLedgerNotWipedBySnapshotWithoutField(t *testing.T) {
	p := NewZcodeProvider()
	p.recordProjection("th1", zcodeSlashSnapshot)
	// subscribe 应答的快照段可能不带 slashCommands 字段:不得冲掉已记的账。
	p.recordProjection("th1", json.RawMessage(`{"projection":{"contextWindow":200001,"totalTokenCount":20000}}`))
	got := p.SlashCommandsFor("th1")
	if len(got) != 2 || got[0].Name != "goal" {
		t.Fatalf("无字段快照不应冲账: %+v", got)
	}
	// 显式空数组 = 确认会话已无命令:覆盖旧值,且命中(空列表,不再回退 resume)。
	p.recordProjection("th1", json.RawMessage(`{"slashCommands":[]}`))
	got = p.SlashCommandsFor("th1")
	if got == nil || len(got) != 0 {
		t.Fatalf("显式空数组应记为空账(非 nil),得到 %+v", got)
	}
}
