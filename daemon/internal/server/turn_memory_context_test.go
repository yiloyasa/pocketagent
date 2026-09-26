// turn_memory_context_test.go —— P2「四层上下文组装」的三根钉
// （memory 分层设计稿 §4 P2 / D4、spec §7:598-610：daemon 家请求前组装的第一个读者）。
//
// 【要钉什么】
//  1. 顺序钉：四层按 元数据 → 已保存记忆 → 早期摘要 → 近期消息 固定顺序组装，
//     消息旧→新呈现、带角色前缀——读者（模型）看到的层序必须稳定可测；
//  2. 预算裁剪钉：总额度内从新到旧保留、超限截断最旧——saved 行从最旧丢起、
//     摘要整段让位（半段摘要是谎报）、消息丢最旧但**最新一条永远保底**
//     （与 runtime ReadWindow"单条自身超预算仍返回"同一纪律）；
//  3. 空槽降级钉：任何一层空缺，整层（含层头）消失，绝不留空标题——
//     空标题既浪费预算又让模型误以为内容丢了。
package server

import (
	"strings"
	"testing"
)

// ctx 是组装钉的输入速记。字段序即层序。
type ctxFixture struct {
	meta    string
	saved   []string
	digest  string
	msgs    []turnContextMessage
	maxXita int64
}

// TestAssembleTurnContextOrder：钉 1——四层顺序与呈现方向。
func TestAssembleTurnContextOrder(t *testing.T) {
	out := assembleTurnContext(
		"host=h1 agent=echo thread=th1 workspace=/w",
		[]string{"早先的决策行", "后来的决策行"},
		"已压缩 4 条（seq 1..4）",
		[]turnContextMessage{{Role: "user", Text: "第一问"}, {Role: "assistant", Text: "第一答"}, {Role: "user", Text: "第二问"}},
		0, // 0 = 不限额度
	)
	metaAt := strings.Index(out, "[会话元数据]")
	savedAt := strings.Index(out, "[已保存的记忆]")
	digestAt := strings.Index(out, "[早期对话摘要]")
	msgsAt := strings.Index(out, "[近期消息]")
	if metaAt < 0 || savedAt < 0 || digestAt < 0 || msgsAt < 0 {
		t.Fatalf("四层的层头都应出现，实得 %q", out)
	}
	if !(metaAt < savedAt && savedAt < digestAt && digestAt < msgsAt) {
		t.Fatalf("层序必须为 元数据→已保存→摘要→消息，实得 %q", out)
	}
	// 消息旧→新、带角色。
	iOld := strings.Index(out, "user: 第一问")
	iAsg := strings.Index(out, "assistant: 第一答")
	iNew := strings.Index(out, "user: 第二问")
	if !(iOld >= 0 && iOld < iAsg && iAsg < iNew) {
		t.Fatalf("消息应旧→新呈现且带角色前缀，实得 %q", out)
	}
	// saved 行保序（旧→新）。
	if strings.Index(out, "早先的决策行") > strings.Index(out, "后来的决策行") {
		t.Fatalf("saved 行应保持装入顺序，实得 %q", out)
	}
}

// TestAssembleTurnContextBudget：钉 2——预算裁剪。
func TestAssembleTurnContextBudget(t *testing.T) {
	// 布置：身份行、saved 3 行（等长）、digest 40 字节、消息 5 条（长短交替）。
	const meta = "host=h1"
	var saved = []string{"saved-row-01", "saved-row-02", "saved-row-03"}
	const digest = "digest-0123456789012345678901234567890123456789"
	var msgs = []turnContextMessage{
		{Role: "user", Text: "msg-01"}, {Role: "assistant", Text: "msg-02"},
		{Role: "user", Text: "msg-03"}, {Role: "assistant", Text: "msg-04"},
		{Role: "user", Text: "msg-05"},
	}
	// 额度按"逐行 len+1"精确核算（与实现的记账口径一致，见 ctxLineSink）。
	cost := func(lines ...string) int64 {
		var n int64
		for _, l := range lines {
			n += int64(len(l)) + 1
		}
		return n
	}

	// 2a. 中等额度：saved 与 digest 全保住，消息只留最近两条（更旧的截掉）。
	budget := cost(ctxLayerMeta, meta) + cost(ctxLayerSaved, saved[0], saved[1], saved[2]) +
		cost(ctxLayerDigest, digest) + cost(ctxLayerMsgs, "assistant: msg-04", "user: msg-05")
	out := assembleTurnContext(meta, saved, digest, msgs, budget)
	if !strings.Contains(out, "saved-row-01") || !strings.Contains(out, "saved-row-03") || !strings.Contains(out, digest) {
		t.Fatalf("该额度下 saved 与 digest 都应保住，实得 %q", out)
	}
	if !strings.Contains(out, "user: msg-05") || !strings.Contains(out, "assistant: msg-04") {
		t.Fatalf("消息应从最新往回保留，实得 %q", out)
	}
	if strings.Contains(out, "msg-01") || strings.Contains(out, "msg-02") || strings.Contains(out, "msg-03") {
		t.Fatalf("超限的最旧消息应被截掉，实得 %q", out)
	}

	// 2a'. saved 超限：从最旧丢起——row-01 让位，row-02/03 保住；随后额度耗尽，
	// digest 整段让位、消息只剩保底的最新一条。
	budget = cost(ctxLayerMeta, meta) + cost(ctxLayerSaved, saved[1], saved[2])
	out = assembleTurnContext(meta, saved, digest, msgs, budget)
	if strings.Contains(out, "saved-row-01") {
		t.Fatalf("saved 超限应从最旧丢起，实得 %q", out)
	}
	if !strings.Contains(out, "saved-row-02") || !strings.Contains(out, "saved-row-03") {
		t.Fatalf("放得下的较新 saved 行应保住，实得 %q", out)
	}
	if strings.Contains(out, digest) || strings.Contains(out, ctxLayerDigest) {
		t.Fatalf("额度耗尽时 digest 应整段（含层头）让位，实得 %q", out)
	}
	if !strings.Contains(out, "user: msg-05") {
		t.Fatalf("最新消息必须有保底，实得 %q", out)
	}

	// 2b. digest 差一字节（记账口径）：摘要整段让位（绝不半段），省下的额度归
	// 更新的层——saved 全保，消息保住 03..05，更旧的 02/01 出局。
	budget = cost(ctxLayerMeta, meta) + cost(ctxLayerSaved, saved[0], saved[1], saved[2]) +
		cost(ctxLayerDigest, digest) - 1
	out = assembleTurnContext(meta, saved, digest, msgs, budget)
	if strings.Contains(out, digest) || strings.Contains(out, ctxLayerDigest) {
		t.Fatalf("差一字节也放不下时 digest 应整段（含层头）让位，实得 %q", out)
	}
	if !strings.Contains(out, "saved-row-01") || !strings.Contains(out, "saved-row-03") {
		t.Fatalf("该额度下 saved 三行都放得下，应全保，实得 %q", out)
	}
	for _, m := range []string{"msg-03", "msg-04", "msg-05"} {
		if !strings.Contains(out, m) {
			t.Fatalf("digest 让位后额度应归消息层，%s 应保住，实得 %q", m, out)
		}
	}
	if strings.Contains(out, "msg-01") || strings.Contains(out, "msg-02") {
		t.Fatalf("更旧的消息应出局，实得 %q", out)
	}

	// 2c. 极端额度：身份行截断不丢弃，最新一条消息永远保底（哪怕超预算）。
	budget = int64(len(ctxLayerMeta)) + 1 + 6
	out = assembleTurnContext("host=very-long-identity-line", nil, "", msgs[len(msgs)-1:], budget)
	if !strings.HasPrefix(out, ctxLayerMeta) {
		t.Fatalf("身份层应最先出现（截断而非丢弃），实得 %q", out)
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("身份行截断应带省略号（rune 安全），实得 %q", out)
	}
	if !strings.Contains(out, "user: msg-05") {
		t.Fatalf("最新消息必须有保底（ReadWindow 同款纪律），实得 %q", out)
	}
}

// TestAssembleTurnContextEmptySlots：钉 3——空槽降级。
func TestAssembleTurnContextEmptySlots(t *testing.T) {
	// 全空 → 空串（不产出一堆空标题）。
	if got := assembleTurnContext("", nil, "", nil, 0); got != "" {
		t.Fatalf("全空输入应组装出空串，实得 %q", got)
	}
	// 只有消息 → 只有消息层。
	out := assembleTurnContext("", nil, "", []turnContextMessage{{Role: "user", Text: "只有这句"}}, 0)
	if !strings.Contains(out, "user: 只有这句") || strings.Contains(out, "[会话元数据]") ||
		strings.Contains(out, "[已保存的记忆]") || strings.Contains(out, "[早期对话摘要]") {
		t.Fatalf("空槽应整层消失（含层头），实得 %q", out)
	}
}

// TestDaemonTurnContextMeta：身份行装配（installation 标识 + 会话归属）。
func TestDaemonTurnContextMeta(t *testing.T) {
	got := daemonTurnContextMeta("host-xyz", "echo", "th-1", "/tmp/ws")
	for _, want := range []string{"host-xyz", "agent=echo", "thread=th-1", "workspace=/tmp/ws"} {
		if !strings.Contains(got, want) {
			t.Fatalf("身份行应含 %q，实得 %q", want, got)
		}
	}
}
