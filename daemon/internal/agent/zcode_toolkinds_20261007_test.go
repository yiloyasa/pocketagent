// zcode_toolkinds_20261007_test.go —— tool.updated fork kind 族的回归钉
// (2026-10-07 字段级全适配审计)。
//
// 证据来源(只读静态提取,未运行任何程序):本机安装 zcode fork
// (%LOCALAPPDATA%/Programs/ZCode/resources/glm/zcode.cjs,构建 2026-09-29)
// 内嵌 zod discriminatedUnion:tool.updated 的 payload kind ∈
// {scheduled, started, progress, result, error, batch, raw} —— 0.16.5 探针时代
// 的 "tool_call"/"tool_error" 已不在 union 内。旧映射只认旧键,fork 上工具
// 起始卡与失败卡整帧静默丢失,progress/batch/raw 连留痕都没有。
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

// ---- kind="scheduled":带 input 的计划态(bundle Zpe+scheduled 变体)----

func TestZcodeToolScheduledMapsToStarted(t *testing.T) {
	raw := json.RawMessage(`{"kind":"scheduled","toolCallId":"call_s1","toolName":"Bash",
		"input":{"command":"go test ./..."},"inputByteLength":18,"inputOmitted":false,
		"parallelGroupIndex":0,"canRunParallel":true,
		"source":"subagent","childSessionId":"sess_child"}`)
	upd, ok := mapZcodeToolCall(raw)
	if !ok {
		t.Fatal("scheduled 应映射 STARTED(旧映射只认 tool_call,fork 上整帧丢失)")
	}
	if upd.Phase != 1 /* STARTED */ {
		t.Fatalf("phase 应为 STARTED,得到 %v", upd.Phase)
	}
	if upd.Call.Id != "call_s1" || upd.Call.Name != "Bash" {
		t.Fatalf("工具标识错误: %+v", upd.Call)
	}
	if !strings.Contains(upd.Call.JsonArgs, "go test") {
		t.Fatalf("scheduled 的 input 应完整透出: %s", upd.Call.JsonArgs)
	}
}

// ---- kind="started":仅 startedAt,toolName 可选(bundle Zpe+started 变体)----

func TestZcodeToolStartedMapsToStartedWithNameFallback(t *testing.T) {
	raw := json.RawMessage(`{"kind":"started","toolCallId":"call_t1","toolName":"Write","startedAt":1760000000000}`)
	upd, ok := mapZcodeToolCall(raw)
	if !ok || upd.Phase != 1 {
		t.Fatalf("started 应映射 STARTED: ok=%v upd=%+v", ok, &upd)
	}
	if upd.Call.Name != "Write" {
		t.Fatalf("工具名应取 toolName: %q", upd.Call.Name)
	}
	// fork 的 started 变体 toolName 是可选字段:空名兜底通用名,不出空名卡。
	rawNoName := json.RawMessage(`{"kind":"started","toolCallId":"call_t2","startedAt":1760000000000}`)
	upd2, ok2 := mapZcodeToolCall(rawNoName)
	if !ok2 || upd2.Call.Name != "tool" {
		t.Fatalf("toolName 缺失应兜底 tool: ok=%v name=%q", ok2, upd2.Call.Name)
	}
}

// ---- kind="progress":执行进度(bundle:elapsedMs/pid/outputPreview/stdoutTail/stderrTail)----

func TestZcodeToolProgressMapsToUpdate(t *testing.T) {
	raw := json.RawMessage(`{"kind":"progress","toolCallId":"call_p1","toolName":"Bash",
		"elapsedMs":1200,"pid":4242,
		"outputPreview":{"text":"ok 3 tests","fullText":"ok 3 tests\n","totalLines":1,"totalBytes":11,"linesEstimated":false},
		"stdoutBytes":11,"stderrBytes":0,"outputBytes":11,
		"stdoutTail":"ok 3 tests","stderrTail":""}`)
	upd, ok := mapZcodeToolProgress(raw)
	if !ok {
		t.Fatal("progress 应映射 UPDATE 帧(此前整个 kind 静默丢弃)")
	}
	if upd.Phase != 3 /* UPDATE */ {
		t.Fatalf("phase 应为 UPDATE,得到 %v", upd.Phase)
	}
	if upd.Call == nil || upd.Call.Id != "call_p1" {
		t.Fatalf("UPDATE 帧应带 callId: %+v", upd.Call)
	}
	if upd.Result == nil || !upd.Result.Ok {
		t.Fatal("进行中没有失败语义,Ok 应为 true(pi 同款)")
	}
	if !strings.Contains(upd.Result.Output, "ok 3 tests") {
		t.Fatalf("outputPreview 应进入输出: %q", upd.Result.Output)
	}
	if upd.Result.ElapsedMs != 1200 {
		t.Fatalf("elapsedMs 应透到 ElapsedMs: %v", upd.Result.ElapsedMs)
	}
	// 全空进度也要有可读占位,不允许空帧
	rawBare := json.RawMessage(`{"kind":"progress","toolCallId":"call_p2"}`)
	if upd2, ok2 := mapZcodeToolProgress(rawBare); !ok2 || upd2.Result.Output == "" {
		t.Fatal("无预览的 progress 应有兜底文案")
	}
}

// ---- kind="error":fork 改名后的失败帧(旧名 tool_error 仍在兼容)----

func TestZcodeToolErrorForkKindMaps(t *testing.T) {
	raw := json.RawMessage(`{"kind":"error","toolCallId":"call_e1","toolName":"Bash",
		"error":{"type":"exec","message":"exit code 1"}}`)
	upd, ok := mapZcodeToolError(raw)
	if !ok {
		t.Fatal("fork kind=error 应映射失败帧(旧映射只认 tool_error)")
	}
	if upd.Phase != 2 /* FINISHED */ || upd.Result.Ok {
		t.Fatalf("error 必须 FINISHED+Ok=false: %+v", &upd)
	}
	if !strings.Contains(upd.Result.Output, "exit code 1") {
		t.Fatalf("错误信息应进入输出: %q", upd.Result.Output)
	}
}

// ---- kind="result":content 非字符串形状(bundle result 段是 m.unknown())----

func TestZcodeToolResultNonStringContentKept(t *testing.T) {
	// content 为对象数组(bundle k5i=m.unknown(),不保证字符串):
	// 旧实现反序列化失败 → 整帧静默丢失;现在按原文透出。
	raw := json.RawMessage(`{"kind":"result","toolCallId":"call_r1","toolName":"Edit",
		"result":{"success":true,"content":[{"path":"a.go","lines":3}]},"duration":42.5}`)
	upd, ok := mapZcodeToolResultClipped(raw, 4000)
	if !ok {
		t.Fatal("非字符串 content 不应让结果帧丢失")
	}
	if !upd.Result.Ok || !strings.Contains(upd.Result.Output, "a.go") {
		t.Fatalf("content 应按原文透出: %+v", upd.Result)
	}
	if upd.Result.ElapsedMs != 42.5 {
		t.Fatalf("duration 应透到 ElapsedMs: %v", upd.Result.ElapsedMs)
	}
}

// ---- kind="batch"/"raw":刻意不投影,但必须走诊断留痕(不再无声消失)----

func TestZcodeToolRestNoticeKindClassification(t *testing.T) {
	if got := zcodeToolRestNoticeKind("batch"); got != runtime.KindKnownPassthrough {
		t.Fatalf("batch 是 fork union 已知成员,应 PASSTHROUGH: %q", got)
	}
	if got := zcodeToolRestNoticeKind("raw"); got != runtime.KindKnownPassthrough {
		t.Fatalf("raw 是 fork union 已知成员,应 PASSTHROUGH: %q", got)
	}
	if got := zcodeToolRestNoticeKind("never_seen"); got != runtime.KindUnknownNotification {
		t.Fatalf("没见过的 kind 应如实 UNKNOWN: %q", got)
	}
	if got := zcodeToolRestNoticeKind(""); got != runtime.KindUnknownNotification {
		t.Fatalf("kind 缺失应如实 UNKNOWN: %q", got)
	}
	if got := zcodeToolPayloadKind(json.RawMessage(`{"kind":"batch","toolCallIds":["a"]}`)); got != "batch" {
		t.Fatalf("kind 判别字段读取失败: %q", got)
	}
}
