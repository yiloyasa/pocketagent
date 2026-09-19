package server

// QA 独立复核补充测试(项 3:presence 合并语义 + 逐项状态回报)。
//
// 【为什么补这些】现有 workflow_b_test.go 覆盖了"只改一项(absent 保留)"与
// "present 空串/false 能写回"。这里补的是**没被覆盖的三个方向**:
//  1. 8 项**同时** present 时彼此不得串味(改一项顺手清另一项是这一轮的历史病根);
//  2. model / effort 是与 CodexPrefs 分离存放的两个值,合并时不能被整体结构体带走;
//  3. 逐项状态里的 failed 必须把**真实错误文本**带进 detail(否则用户只看到
//     "连接失败"四个字,无从排查)。
//
// 注意:本文件不得重定义 workflow_b_test.go 里已有的 pstr / pbool 辅助函数。

import (
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// 8 项全部显式 set 时,合并结果必须与请求一一对应,且不残留基线里的旧值。
func TestQAReviewMergeAllPresentOverwritesEverything(t *testing.T) {
	base := store.CodexPrefs{
		ApprovalPolicy:    "never",
		Sandbox:           "danger-full-access",
		HideReasoning:     false,
		Verbosity:         "low",
		ReasoningSummary:  "auto",
		DisablePasteBurst: true,
	}
	ap := "on-request"
	sb := "workspace-write"
	md := "codex/gpt-5-mini"
	ef := "medium"
	hr := true
	vb := "high"
	rs := "detailed"
	pb2 := false

	got, model, effort := mergeCodexPrefs(base, "codex/gpt-5", "high", &pb.SetCodexSettingsRequest{
		ApprovalPolicy:        &ap,
		Sandbox:               &sb,
		Model:                 &md,
		Effort:                &ef,
		HideAgentReasoning:    &hr,
		ModelVerbosity:        &vb,
		ModelReasoningSummary: &rs,
		DisablePasteBurst:     &pb2,
	})

	want := store.CodexPrefs{
		ApprovalPolicy:    ap,
		Sandbox:           sb,
		HideReasoning:     hr,
		Verbosity:         vb,
		ReasoningSummary:  rs,
		DisablePasteBurst: pb2,
	}
	if got != want {
		t.Fatalf("全量覆盖结果不符:\n got=%+v\nwant=%+v", got, want)
	}
	if model != md {
		t.Fatalf("model 未覆盖: got=%q want=%q", model, md)
	}
	if effort != ef {
		t.Fatalf("effort 未覆盖: got=%q want=%q", effort, ef)
	}
}

// 只改一个布尔项:其余 7 项(含 model / effort)必须一字不动。
func TestQAReviewMergeSingleBoolLeavesOthersUntouched(t *testing.T) {
	base := store.CodexPrefs{
		ApprovalPolicy:    "on-request",
		Sandbox:           "read-only",
		HideReasoning:     false,
		Verbosity:         "medium",
		ReasoningSummary:  "concise",
		DisablePasteBurst: true,
	}
	tru := true
	got, model, effort := mergeCodexPrefs(base, "codex/gpt-5", "xhigh", &pb.SetCodexSettingsRequest{
		HideAgentReasoning: &tru,
	})
	if !got.HideReasoning {
		t.Fatalf("布尔项未生效")
	}
	// 除 hide_reasoning 外全部保持基线
	got.HideReasoning = base.HideReasoning
	if got != base {
		t.Fatalf("只改布尔项却动了别的字段:\n got=%+v\nwant=%+v", got, base)
	}
	if model != "codex/gpt-5" || effort != "xhigh" {
		t.Fatalf("model/effort 被误改: model=%q effort=%q", model, effort)
	}
}

// failed 状态的 detail 必须包含真实错误文本,便于用户排查。
func TestQAReviewFailedStatusCarriesRealErrorText(t *testing.T) {
	c := &pb.CodexSettings{ApprovalPolicy: "on-request"}
	const boom = "dial tcp 127.0.0.1:7246: connection refused"
	got := buildSettingStatus(c, false, boom)
	for _, s := range got {
		if s.State != "failed" {
			continue
		}
		if !strings.Contains(s.Detail, boom) {
			t.Fatalf("%s 的 detail 未带真实错误文本: %q", s.Key, s.Detail)
		}
	}
}

// 设置项清单必须正好 8 项、key 唯一且覆盖 8 个键(防止漏报/重复报)。
func TestQAReviewSettingSpecsCoverExactlyEightUniqueKeys(t *testing.T) {
	want := []string{
		"approval_policy", "sandbox", "model", "effort",
		"model_reasoning_summary", "model_verbosity",
		"hide_agent_reasoning", "disable_paste_burst",
	}
	if len(codexSettingSpecs) != len(want) {
		t.Fatalf("设置项数 %d != %d", len(codexSettingSpecs), len(want))
	}
	seen := map[string]bool{}
	for _, sp := range codexSettingSpecs {
		if seen[sp.key] {
			t.Fatalf("重复的 key: %s", sp.key)
		}
		seen[sp.key] = true
	}
	for _, k := range want {
		if !seen[k] {
			t.Fatalf("缺少设置项 key: %s", k)
		}
	}
}
