package server

// WorkflowB 单测:codex 设置合并语义 + 逐项状态生成。
//
// 【为什么这几条必须钉死】"设置页所有开关点了都没用"这一轮的核心修复,
// 成败全在两点:① absent 字段不能被清空;② present 的空串/ false 必须能写进去。
// 这两个方向都能把用户坑惨,所以用表驱动测试锁住。

import (
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func pstr(s string) *string { return &s }
func pbool(b bool) *bool    { return &b }

// TestWorkflowBMergeCodexPrefsPresence 验证 presence 合并的两个方向。
func TestWorkflowBMergeCodexPrefsPresence(t *testing.T) {
	base := store.CodexPrefs{
		ApprovalPolicy:    "never",
		Sandbox:           "workspace-write",
		HideReasoning:     true,
		Verbosity:         "high",
		ReasoningSummary:  "detailed",
		DisablePasteBurst: false,
	}
	const baseModel = "codex/gpt-5"
	const baseEffort = "high"

	t.Run("absent 字段一律原样保留", func(t *testing.T) {
		// 只改思考强度,其余全是 absent → 一个字都不能被动。
		got, model, effort := mergeCodexPrefs(base, baseModel, baseEffort, &pb.SetCodexSettingsRequest{
			Effort: pstr("low"),
		})
		if got != base {
			t.Fatalf("absent 字段被改动: got=%+v want=%+v", got, base)
		}
		if model != baseModel {
			t.Fatalf("模型被清空: got=%q want=%q", model, baseModel)
		}
		if effort != "low" {
			t.Fatalf("思考强度未更新: got=%q", effort)
		}
	})

	t.Run("present 的空串是「跟随桌面」真值,必须能写回", func(t *testing.T) {
		got, model, _ := mergeCodexPrefs(base, baseModel, baseEffort, &pb.SetCodexSettingsRequest{
			Model:   pstr(""), // 显式 set 空串 = 回到"跟随桌面"
			Sandbox: pstr(""),
		})
		if model != "" {
			t.Fatalf("setModel(\"\") 未生效: got=%q", model)
		}
		if got.Sandbox != "" {
			t.Fatalf("setSandbox(\"\") 未生效: got=%q", got.Sandbox)
		}
		if got.ApprovalPolicy != base.ApprovalPolicy {
			t.Fatalf("改动 model 时误清了 approval: got=%q", got.ApprovalPolicy)
		}
	})

	t.Run("present 的 false 必须能把开关关回去", func(t *testing.T) {
		got, _, _ := mergeCodexPrefs(base, baseModel, baseEffort, &pb.SetCodexSettingsRequest{
			HideAgentReasoning: pbool(false),
		})
		if got.HideReasoning {
			t.Fatalf("hide_agent_reasoning=false 未能关闭开关")
		}
	})

	t.Run("present 的 true 能打开开关", func(t *testing.T) {
		got, _, _ := mergeCodexPrefs(store.CodexPrefs{}, baseModel, baseEffort, &pb.SetCodexSettingsRequest{
			HideAgentReasoning: pbool(true),
		})
		if !got.HideReasoning {
			t.Fatalf("hide_agent_reasoning=true 未生效")
		}
	})
}

// TestWorkflowBBuildSettingStatus 验证四态(含不支持)如实回报。
func TestWorkflowBBuildSettingStatus(t *testing.T) {
	c := &pb.CodexSettings{
		ApprovalPolicy:        "on-request",
		Model:                 "codex/gpt-5",
		Effort:                "",
		ModelVerbosity:        "medium",
		ModelReasoningSummary: "concise",
	}

	t.Run("codex 可达:有值=applied,空值=follow_desktop,不支持=unsupported", func(t *testing.T) {
		got := buildSettingStatus(c, true, "")
		byKey := map[string]*pb.CodexSettingStatus{}
		for _, s := range got {
			byKey[s.Key] = s
		}
		if byKey["approval_policy"].State != "applied" {
			t.Fatalf("approval_policy 应为 applied, got=%q", byKey["approval_policy"].State)
		}
		if byKey["effort"].State != "follow_desktop" {
			t.Fatalf("effort 空值应为 follow_desktop, got=%q", byKey["effort"].State)
		}
		if byKey["disable_paste_burst"].State != "unsupported" {
			t.Fatalf("disable_paste_burst 应为 unsupported, got=%q", byKey["disable_paste_burst"].State)
		}
		if byKey["hide_agent_reasoning"].State != "applied" {
			t.Fatalf("hide_agent_reasoning(false) 应为 applied, got=%q", byKey["hide_agent_reasoning"].State)
		}
	})

	t.Run("codex 不可达:支持项全部报 failed 且带原因", func(t *testing.T) {
		got := buildSettingStatus(c, false, "boom")
		for _, s := range got {
			if s.Key == "disable_paste_burst" {
				if s.State != "unsupported" {
					t.Fatalf("不可达时 unsupported 仍应为 unsupported, got=%q", s.State)
				}
				continue
			}
			if s.State != "failed" {
				t.Fatalf("%s 应为 failed, got=%q", s.Key, s.State)
			}
			if s.Detail == "" {
				t.Fatalf("%s failed 但没给原因", s.Key)
			}
		}
	})

	t.Run("每项都必须有状态,不漏项", func(t *testing.T) {
		got := buildSettingStatus(c, true, "")
		if len(got) != len(codexSettingSpecs) {
			t.Fatalf("状态条数 %d != 设置项数 %d", len(got), len(codexSettingSpecs))
		}
	})
}
