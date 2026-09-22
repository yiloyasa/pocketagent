package agent

// codex_output_contract_test.go —— 一轮「最终正文」各层来源的边界(方案 §4.4 的 7 类输出)。
//
// 【为什么要钉这些边界】用户报"任务完成却没有输出"时,第一件事是判断正文到底**在不在**、
// **在哪一层**:① 增量 delta 拿到了?② item/completed 拿到了?③ turn/completed 自带的
// items 有?④ 原生历史(rollout)里有?四层全空才是"真的没话说"。
//
// ① 与 ④ 由别的用例覆盖(delta 在 Run 的 streamText 层,原生历史在
// internal/server 的 output_recovery_test.go 经 NativeTurnReader 覆盖)。本文件补的是
// ② ③ 两层,以及"结构变了会怎样"的边界 —— 这些一旦被后来者改动(比如 turn 的 items
// 只取第一条),图二就会以另一种形式回来。
//
// 【样本来源如实说明】下面是**按协议形状构造**的样本(对应方案 §4.4 要区分的 7 类);
// 真机抓包样本尚未落盘 —— 已在交付报告里明示,不当作已完成的 fixture。

import (
	"encoding/json"
	"testing"
)

// 第 ② 层:item/completed 的 agentMessage。
func TestLayer2ItemCompletedCarriesText(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"t1","item":{"type":"agentMessage","id":"m1","text":"第二层拿到的正文"}}`)
	if got := agentMessageTextFromNotification(raw); got != "第二层拿到的正文" {
		t.Fatalf("第二层提取错: got %q", got)
	}
}

// 结构变化的容错:agentMessage 正文也可能在 content[] 里(旧版/变体形状)。
// 【为什么这条重要】上游换过正文承载字段 —— 只认 text 的话会静默变空,正是图二的成因之一。
func TestLayer2ToleratesContentArrayShape(t *testing.T) {
	raw := json.RawMessage(`{"item":{"type":"agentMessage","id":"m1","content":[{"type":"text","text":"旧版形状的正文"}]}}`)
	if got := agentMessageTextFromNotification(raw); got != "旧版形状的正文" {
		t.Fatalf("content[] 形状没被容错: got %q", got)
	}
}

// 第 ③ 层:turn/completed 自带的 turn.items。
func TestLayer3TurnCompletedItemsCarryText(t *testing.T) {
	raw := json.RawMessage(`{"turn":{"id":"turn-1","items":[{"type":"agentMessage","text":"第三层拿到的正文"}]}}`)
	if got := agentMessageTextsFromTurn(raw); got != "第三层拿到的正文" {
		t.Fatalf("第三层提取错: got %q", got)
	}
}

// 第 ③ 层拼接多条 agentMessage(一层里可能有多条)。
func TestLayer3ConcatenatesMultipleAgentMessages(t *testing.T) {
	raw := json.RawMessage(`{"turn":{"id":"t","items":[
		{"type":"agentMessage","text":"第一段"},
		{"type":"userMessage","text":"用户的话"},
		{"type":"agentMessage","text":"第二段"}
	]}}`)
	if got := agentMessageTextsFromTurn(raw); got != "第一段第二段" {
		t.Fatalf("应只拼 agentMessage、按序拼接, got %q", got)
	}
}

// 边界:turn/completed 的结构变了(没有 turn.items)—— 必须安全返回空,而不是 panic。
// 返回空之后,server 层会走原生历史补偿那条路(recoverEmptyTurnOutput)。
func TestLayer3ToleratesMissingTurnItems(t *testing.T) {
	for _, raw := range []string{
		`{"turn":{"id":"t"}}`,
		`{"turn":null}`,
		`{}`,
		`not json at all`,
	} {
		if got := agentMessageTextsFromTurn(json.RawMessage(raw)); got != "" {
			t.Fatalf("结构缺失/坏 JSON 应安全返回空, raw=%s got=%q", raw, got)
		}
	}
}

// 第 ⑥ 类:只有非文本产物(图片/工具)—— 提取不到正文,但不该崩、也不该编出文本。
// 这类在 server 层会被判 ARTIFACT_ONLY,而不是 EMPTY。
func TestNonTextArtifactsYieldNoText(t *testing.T) {
	for _, raw := range []string{
		`{"item":{"type":"image","id":"i1","url":"x.png"}}`,
		`{"item":{"type":"commandExecution","id":"c1"}}`,
		`{"item":{"type":"contextCompaction","id":"k1"}}`,
	} {
		if got := agentMessageTextFromNotification(json.RawMessage(raw)); got != "" {
			t.Fatalf("非文本 item 不该产出正文, raw=%s got=%q", raw, got)
		}
	}
}

// 第 ⑦ 类:真正无文本(agentMessage 存在但正文为空)—— 提取结果为空,
// 于是 server 层判 EMPTY,手机端会显示"本轮没有正文"。
func TestEmptyAgentMessageYieldsEmptyText(t *testing.T) {
	raw := json.RawMessage(`{"item":{"type":"agentMessage","id":"m1","text":""}}`)
	if got := agentMessageTextFromNotification(raw); got != "" {
		t.Fatalf("空正文应如实返回空, got %q", got)
	}
}
