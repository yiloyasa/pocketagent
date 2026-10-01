package server

// approval_options_test.go —— C1 审批三按钮增量的 server 侧回归钉子。
//
// 钉两件事:
//  1. options/details 从审批卡 JSON 提升为 ApprovalRequest 结构化字段;
//  2. 旧端兼容:Decision.option 为空时 note 一个字节不动;非空时信封可逆。

import (
	"encoding/json"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func testApprovalCard(options, details string) *pb.ToolCallBlock {
	card := `{"kind":"command","command":"git push","cwd":"/w","reason":"推送"`
	if options != "" {
		card += `,"options":` + options
	}
	if details != "" {
		card += `,"details":` + details
	}
	card += `}`
	return &pb.ToolCallBlock{Id: "i1", Name: "shell", JsonArgs: card}
}

// TestApprovalCardExtrasLift 卡 JSON 里的 options/details 应提升为结构化字段。
func TestApprovalCardExtrasLift(t *testing.T) {
	opts, details := approvalCardExtras(testApprovalCard(
		`[{"id":"accept","label":"允许","description":"仅本次执行"},
		  {"id":"acceptForSession","label":"本次及本会话允许","description":"本会话内同类请求不再询问"},
		  {"id":"decline","label":"拒绝","description":"告知 agent 换其他做法"}]`,
		`"上游提议把 2 条命令规则加入本会话免审批清单"`))
	if len(opts) != 3 {
		t.Fatalf("应提升 3 个选项,得到 %d", len(opts))
	}
	if opts[1].Id != "acceptForSession" || opts[1].Label != "本次及本会话允许" {
		t.Errorf("第三按钮提升错误: %+v", opts[1])
	}
	if details == "" {
		t.Errorf("details 应一并提升")
	}

	// 无附加信息:零值,不得凭空造
	if o, d := approvalCardExtras(testApprovalCard("", "")); o != nil || d != "" {
		t.Errorf("无 options/details 的卡应返回零值,得到 (%v, %q)", o, d)
	}
	// 坏 JSON:照常零值,不影响审批卡本体送达
	broken := &pb.ToolCallBlock{Id: "i2", Name: "shell", JsonArgs: `{"kind":`}
	if o, d := approvalCardExtras(broken); o != nil || d != "" {
		t.Errorf("坏 JSON 应按零值处理,得到 (%v, %q)", o, d)
	}
	// 无 id 的选项不提升(没有 id 就无法应答)
	opts2, _ := approvalCardExtras(testApprovalCard(`[{"label":"无名"}]`, ""))
	if len(opts2) != 0 {
		t.Errorf("无 id 选项不应提升,得到 %v", opts2)
	}
}

// TestApprovalNoteWithOptionCompat 旧端兼容是字节级的:option 为空时 note
// 原样返回;新端 option 非空时信封可逆(option/note 都能解回来)。
func TestApprovalNoteWithOptionCompat(t *testing.T) {
	// 旧端:两按钮应答,option 空 —— note 一个字节不动
	const raw = `{"q1":{"answers":["B"]}}`
	if got := approvalNoteWithOption(&pb.ApprovalDecision{Note: raw}); got != raw {
		t.Fatalf("旧端 note 必须字节级原样,得到 %q", got)
	}
	if got := approvalNoteWithOption(nil); got != "" {
		t.Fatalf("nil decision 应回空串,得到 %q", got)
	}

	// 新端:三按钮应答,信封可逆
	env := approvalNoteWithOption(&pb.ApprovalDecision{Approved: true, Option: "acceptForSession", Note: "ok"})
	var parsed struct {
		Option string `json:"option"`
		Note   string `json:"note"`
	}
	if err := json.Unmarshal([]byte(env), &parsed); err != nil {
		t.Fatalf("信封应是合法 JSON: %v (%q)", err, env)
	}
	if parsed.Option != "acceptForSession" || parsed.Note != "ok" {
		t.Fatalf("信封内容错误: %q", env)
	}
}
