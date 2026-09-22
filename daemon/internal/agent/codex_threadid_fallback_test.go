package agent

// codex_threadid_fallback_test.go —— T-S1P 回归钉子:codexThreadIDOf 对
// thread/started(顶层无 threadId,线程 id 在 thread.id)的回退解析。
//
// 【背景】"会话级事实同步"管线以 codexThreadIDOf 的返回值作为 server 层
// SyncThreadEvent 的线程键;回退缺失时 thread/started 永远拿到空 id 被整帧
// 丢弃,子代理线程的存根登记无从谈起(S0 实测帧形状见
// docs/verify/evidence/codex-subagent-s0.md)。

import (
	"encoding/json"
	"testing"
)

func TestCodexThreadIDOfFallsBackToThreadObject(t *testing.T) {
	// thread/started(S0 实测):只有 thread.id。
	params := json.RawMessage(`{"thread":{"id":"01a0c00d-e928","parentThreadId":null,"cwd":"C:\\w"}}`)
	if got := codexThreadIDOf(params); got != "01a0c00d-e928" {
		t.Fatalf("thread/started 应回退取 thread.id, got %q", got)
	}
}

func TestCodexThreadIDOfPrefersTopLevelThreadID(t *testing.T) {
	// 其余通知(thread/status/changed、item/*、turn/* 等)顶层带 threadId:
	// 优先取顶层,行为与回退引入前一字不变。
	params := json.RawMessage(`{"threadId":"cx-1","status":{"type":"idle"}}`)
	if got := codexThreadIDOf(params); got != "cx-1" {
		t.Fatalf("顶层 threadId 优先, got %q", got)
	}
	// 双双在场也以顶层为准(不存在的形状,钉住优先级防回归)。
	params = json.RawMessage(`{"threadId":"cx-1","thread":{"id":"cx-2"}}`)
	if got := codexThreadIDOf(params); got != "cx-1" {
		t.Fatalf("双字段在场时顶层优先, got %q", got)
	}
}

func TestCodexThreadIDOfEmptyForUnrelatedNotifications(t *testing.T) {
	// 与线程无关的通知(skills/changed、warning 等)两处皆无 → 空串,
	// server 层按既有路径忽略。
	if got := codexThreadIDOf(json.RawMessage(`{"status":"changed"}`)); got != "" {
		t.Fatalf("无关通知应返回空串, got %q", got)
	}
	if got := codexThreadIDOf(json.RawMessage(`not-json`)); got != "" {
		t.Fatalf("坏 JSON 应返回空串, got %q", got)
	}
}
