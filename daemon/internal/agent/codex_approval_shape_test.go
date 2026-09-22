package agent

// codex_approval_shape_test.go —— 审批应答枚举判定的回归钉子。
//
// 【为什么钉】审批应答的枚举新旧不通用:旧版 approved/denied,新版 accept/decline,
// 发错值对方判为无效应答 —— 用户看到的是"点了允许却没反应",极难归因。
// 原实现**枚举两个旧方法名**,将来 codex 新增第三种旧式方法时会掉进新版分支。
// 现改为按**协议形态**判定:新版挂 item/ 命名空间,旧版是顶层 camelCase。
// 本文件钉住"形态判定",且专门断言**未来新增的方法也能自动归类**。

import "testing"

func TestIsLegacyApprovalByShape(t *testing.T) {
	// 旧版:顶层 camelCase(含一个假想的"未来的旧式方法")
	for _, m := range []string{
		"execCommandApproval",
		"applyPatchApproval",
		"someFutureTopLevelApproval", // 未来新增,也应自动落进旧版分支
	} {
		if !isLegacyApproval(m) {
			t.Fatalf("%s 是顶层方法名,应判为旧版应答枚举", m)
		}
	}

	// 新版:item/ 命名空间(含一个假想的"未来的 item 方法")
	for _, m := range []string{
		"item/commandExecution/requestApproval",
		"item/fileChange/requestApproval",
		"item/permissions/requestApproval",
		"item/someFutureThing/requestApproval", // 未来新增,也应自动落进新版分支
	} {
		if isLegacyApproval(m) {
			t.Fatalf("%s 在 item/ 命名空间下,应判为新版应答枚举", m)
		}
	}
}

func TestApprovalDecisionEnums(t *testing.T) {
	cases := []struct {
		method string
		allow  bool
		want   string
	}{
		{"execCommandApproval", true, "approved"},
		{"execCommandApproval", false, "denied"},
		{"applyPatchApproval", true, "approved"},
		{"item/commandExecution/requestApproval", true, "accept"},
		{"item/commandExecution/requestApproval", false, "decline"},
		{"item/fileChange/requestApproval", false, "decline"},
	}
	for _, c := range cases {
		if got := approvalDecision(c.method, c.allow); got != c.want {
			t.Fatalf("approvalDecision(%q, %v) = %q, want %q", c.method, c.allow, got, c.want)
		}
	}
}
