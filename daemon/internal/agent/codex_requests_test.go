package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

// 这组测试钉死「反向请求分派」这张表。
//
// 【为什么值得钉】这三条曾经全是 bug,而且都是**静默**的:
//   - 所有审批坍缩成 "shell",用户在手机上看到的动作类型是错的;
//   - 非审批请求被回 {"decision":"decline"},token 刷新被当成"用户拒绝刷新";
//   - 旧版审批用 accept/decline,而它只认 approved/denied,应答被判无效。
//
// 任何一条回归,测试会在这里先炸,而不是等到真机上"某个功能莫名失效"。
func TestClassifyCodexRequest(t *testing.T) {
	cases := map[string]codexRequestKind{
		"item/commandExecution/requestApproval": reqApprovalCommand,
		"execCommandApproval":                   reqApprovalCommand,
		"item/fileChange/requestApproval":       reqApprovalFileChange,
		"applyPatchApproval":                    reqApprovalPatch,
		"item/permissions/requestApproval":      reqApprovalPermissions,
		"item/tool/requestUserInput":            reqAskUser,
		"mcpServer/elicitation/request":         reqAskMcp,
		"item/tool/call":                        reqUnsupported,
		"account/chatgptAuthTokens/refresh":     reqUnsupported,
		"attestation/generate":                  reqUnsupported,
		"some/brand/new/method":                 reqUnknown,
		// 子串匹配时代的老陷阱:名字里带 approval 但不是审批。
		"item/approvalReview/statusReport": reqUnknown,
	}
	for method, want := range cases {
		if got := classifyCodexRequest(method); got != want {
			t.Errorf("classifyCodexRequest(%q) = %d, want %d", method, got, want)
		}
	}
}

func TestApprovalDecisionEnumMatchesProtocolVersion(t *testing.T) {
	// v2:accept / decline
	if got := approvalDecision("item/commandExecution/requestApproval", true); got != "accept" {
		t.Errorf("v2 允许应为 accept,得到 %q", got)
	}
	if got := approvalDecision("item/commandExecution/requestApproval", false); got != "decline" {
		t.Errorf("v2 拒绝应为 decline,得到 %q", got)
	}
	// 旧版:approved / denied —— 发成 accept 会被判为无效应答
	if got := approvalDecision("execCommandApproval", true); got != "approved" {
		t.Errorf("旧版允许应为 approved,得到 %q", got)
	}
	if got := approvalDecision("applyPatchApproval", false); got != "denied" {
		t.Errorf("旧版拒绝应为 denied,得到 %q", got)
	}
}

// 审批卡必须带真实动作类型,而不是一律 "shell"。
func TestBuildApprovalCallUsesRealActionType(t *testing.T) {
	// 命令:字符串形式的 command
	cmd := buildApprovalCall(reqApprovalCommand, "item/commandExecution/requestApproval",
		json.RawMessage(`{"itemId":"i1","command":"rm -rf build","cwd":"/w","reason":"清理"}`), nil)
	if cmd.Name != toolNameShell {
		t.Errorf("命令审批 name = %q, want %q", cmd.Name, toolNameShell)
	}
	var cmdArgs map[string]interface{}
	_ = json.Unmarshal([]byte(cmd.JsonArgs), &cmdArgs)
	if cmdArgs["command"] != "rm -rf build" {
		t.Errorf("命令未正确归一: %v", cmdArgs["command"])
	}
	if cmd.Id != "i1" {
		t.Errorf("命令审批应取 itemId,得到 %q", cmd.Id)
	}

	// 旧版命令:数组形式的 command
	legacy := buildApprovalCall(reqApprovalCommand, "execCommandApproval",
		json.RawMessage(`{"callId":"c9","command":["go","test","./..."]}`), nil)
	if legacy.Name != toolNameShell || legacy.Id != "c9" {
		t.Errorf("旧版命令审批 = (%q,%q)", legacy.Name, legacy.Id)
	}
	var legacyArgs map[string]interface{}
	_ = json.Unmarshal([]byte(legacy.JsonArgs), &legacyArgs)
	if legacyArgs["command"] != "go test ./..." {
		t.Errorf("数组 command 未拼成一行: %v", legacyArgs["command"])
	}

	// 文件变更:参数里没有文件列表,必须用同回合累积的 changeFiles 补上
	fc := buildApprovalCall(reqApprovalFileChange, "item/fileChange/requestApproval",
		json.RawMessage(`{"itemId":"i2","reason":"写入配置"}`),
		[]string{"a.go", "b.go"})
	if fc.Name != toolNameFileChange {
		t.Errorf("文件变更审批 name = %q, want %q", fc.Name, toolNameFileChange)
	}
	var fcArgs map[string]interface{}
	_ = json.Unmarshal([]byte(fc.JsonArgs), &fcArgs)
	if fcArgs["count"] != float64(2) {
		t.Errorf("文件变更审批应带上变更文件数,得到 %v", fcArgs["count"])
	}

	// 旧版补丁:自带 fileChanges,取 key 且排序稳定
	patch := buildApprovalCall(reqApprovalPatch, "applyPatchApproval",
		json.RawMessage(`{"callId":"c3","fileChanges":{"z.go":{},"a.go":{}}}`), nil)
	if patch.Name != toolNamePatch {
		t.Errorf("补丁审批 name = %q, want %q", patch.Name, toolNamePatch)
	}
	var patchArgs struct {
		Files []string `json:"files"`
	}
	_ = json.Unmarshal([]byte(patch.JsonArgs), &patchArgs)
	if len(patchArgs.Files) != 2 || patchArgs.Files[0] != "a.go" || patchArgs.Files[1] != "z.go" {
		t.Errorf("补丁文件列表应去重并排序,得到 %v", patchArgs.Files)
	}

	// 权限提升:permissions 原样透传,不能丢
	perms := buildApprovalCall(reqApprovalPermissions, "item/permissions/requestApproval",
		json.RawMessage(`{"itemId":"i4","permissions":{"network":{"enabled":true}},"reason":"要联网"}`), nil)
	if perms.Name != toolNamePerms {
		t.Errorf("权限审批 name = %q, want %q", perms.Name, toolNamePerms)
	}
	if !strings.Contains(perms.JsonArgs, `"enabled":true`) {
		t.Errorf("权限申请内容未透传: %s", perms.JsonArgs)
	}

	// agent 反问
	ask := buildApprovalCall(reqAskUser, "item/tool/requestUserInput",
		json.RawMessage(`{"itemId":"i5","questions":[{"id":"q1","header":"选择","question":"用哪个?","options":[{"label":"A","description":"a"}]}]}`), nil)
	if ask.Name != toolNameAskUser {
		t.Errorf("反问 name = %q, want %q", ask.Name, toolNameAskUser)
	}
	if !strings.Contains(ask.JsonArgs, `"q1"`) {
		t.Errorf("反响应带上问题列表: %s", ask.JsonArgs)
	}

	// MCP 询问走自己的名字,别和 agent 反问混成一个
	mcp := buildApprovalCall(reqAskMcp, "mcpServer/elicitation/request",
		json.RawMessage(`{"serverName":"github","message":"授权?"}`), nil)
	if mcp.Name != toolNameAskMcp {
		t.Errorf("MCP 询问 name = %q, want %q", mcp.Name, toolNameAskMcp)
	}
}

func TestSplitAnswerNote(t *testing.T) {
	qs := []askUserQuestion{{Id: "q1"}, {Id: "q2"}}

	// 正常情况:Android 提交的是 JSON 映射,原样透传
	got := splitAnswerNote(`{"q1":{"answers":["A"]}}`, qs)
	if m, ok := got["q1"].(map[string]interface{}); !ok || m["answers"].([]interface{})[0] != "A" {
		t.Errorf("JSON 答案未透传: %v", got)
	}

	// 退路:用户只打了一句话 → 用来回答第一个问题,别让 agent 收到空答案去猜
	got = splitAnswerNote("就用 A 方案", qs)
	if _, ok := got["q1"]; !ok {
		t.Errorf("自由文本应落到第一个问题上: %v", got)
	}

	// 空答案:返回空映射(而不是 nil,避免序列化成 null)
	got = splitAnswerNote("   ", qs)
	if got == nil || len(got) != 0 {
		t.Errorf("空答案应为空映射,得到 %v", got)
	}
}

func TestMergePathsDedupesAndKeepsOrder(t *testing.T) {
	got := mergePaths([]string{"a.go"}, []string{"a.go", "", "b.go", "c.go", "b.go"})
	want := []string{"a.go", "b.go", "c.go"}
	if len(got) != len(want) {
		t.Fatalf("mergePaths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mergePaths = %v, want %v", got, want)
		}
	}
}

func TestUnsupportedRequestsAreHonest(t *testing.T) {
	for _, m := range []string{
		"account/chatgptAuthTokens/refresh",
		"item/tool/call",
		"attestation/generate",
	} {
		if msg := unsupportedRequestMessage(m); msg == "" || strings.Contains(msg, "decline") {
			t.Errorf("%s 的说明不应暗示「拒绝」: %q", m, msg)
		}
	}
}
