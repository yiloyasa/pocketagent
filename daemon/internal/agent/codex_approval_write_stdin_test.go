// codex_approval_write_stdin_test.go —— 0.159.2 审批面新字段 kind 的适配回归。
//
// 【协议依据】codex-schema(0.159.2 快照)CommandExecutionApprovalKind:
// item/commandExecution/requestApproval 的 params 带 kind ∈ {command, writeStdin},
// default "command"(老服务端不带此字段);活体探针实帧见 0.159.2 适配轮
// (kind="command" 的 echo 审批,accept 应答后命令真执行、turn/completed 收口)。
// Response schema 未变:两种 kind 都用 {"decision":"accept"|"decline"} 应答。
//
// 【钉什么】
//  1. kind=writeStdin 的审批卡必须用 write_stdin 动作类型(不能冒充 shell 命令),
//     卡内 JSON 如实带 kind;ApprovalReason 给出对应人话;
//  2. kind 缺省(老形状/纯 command)走既有 shell 卡路径,行为一字不变;
//  3. 应答枚举两种 kind 同路:approvalDecision 不分叉。
package agent

import (
	"encoding/json"
	"testing"
)

func TestWriteStdinApprovalCardIsNotShell(t *testing.T) {
	call := buildApprovalCall(reqApprovalCommand, "item/commandExecution/requestApproval",
		json.RawMessage(`{"itemId":"it1","approvalId":"ap1","kind":"writeStdin","reason":"向交互进程写确认"}`), nil)
	if call.Name != toolNameWriteStdin {
		t.Fatalf("writeStdin 审批卡 name = %q, want %q(不得冒充 shell)", call.Name, toolNameWriteStdin)
	}
	// id 取值链不变:callId → itemId → approvalId;此帧只有 itemId/approvalId。
	if call.Id != "it1" {
		t.Errorf("writeStdin 审批卡 id = %q, want itemId it1", call.Id)
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.JsonArgs), &args); err != nil {
		t.Fatalf("卡内 JSON 无法解析: %v", err)
	}
	if args["kind"] != "writeStdin" {
		t.Errorf("卡内 kind = %v, want writeStdin(手机端与透析都要能看到真实动作类型)", args["kind"])
	}
	if _, has := args["command"]; has {
		t.Errorf("writeStdin 卡不得带 command 键(它不是命令执行): %v", args["command"])
	}
	if got := ApprovalReason(call); got != "agent 请求向运行中的命令写入输入" {
		t.Errorf("writeStdin 审批原因 = %q, want 与动作类型一致的人话", got)
	}
}

func TestCommandKindApprovalCardKeepsLegacyShape(t *testing.T) {
	// 老服务端/纯命令审批:kind 缺省或 "command" → 既有 shell 卡,键位只增不改。
	for _, raw := range []string{
		`{"itemId":"i1","command":"echo hi","cwd":"/w"}`,
		`{"itemId":"i2","kind":"command","command":"echo hi","cwd":"/w"}`,
	} {
		call := buildApprovalCall(reqApprovalCommand, "item/commandExecution/requestApproval",
			json.RawMessage(raw), nil)
		if call.Name != toolNameShell {
			t.Fatalf("输入 %s: name = %q, want shell", raw, call.Name)
		}
		var args map[string]interface{}
		_ = json.Unmarshal([]byte(call.JsonArgs), &args)
		if args["command"] != "echo hi" {
			t.Errorf("输入 %s: command = %v, want 原样保留", raw, args["command"])
		}
		if got := ApprovalReason(call); got != "agent 请求执行命令" {
			t.Errorf("输入 %s: 审批原因 = %q", raw, got)
		}
	}
}

func TestWriteStdinDecisionUsesSameV2Enum(t *testing.T) {
	// schema Response 未变:writeStdin 与 command 共用 accept/decline 枚举。
	if got := approvalDecision("item/commandExecution/requestApproval", true); got != "accept" {
		t.Errorf("writeStdin 允许应答 = %q, want accept", got)
	}
	if got := approvalDecision("item/commandExecution/requestApproval", false); got != "decline" {
		t.Errorf("writeStdin 拒绝应答 = %q, want decline(availableDecisions 另有 cancel=拒并打断,daemon 不擅自升级语义)", got)
	}
}
