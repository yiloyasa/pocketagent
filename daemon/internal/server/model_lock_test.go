// model_lock_test.go —— 七项设置**可写且真实生效**的回归测试(回退锁定改动后的反向钉死)。
//
// 【用户已否定"七项锁定"】审批策略/沙箱模式/默认模型/思考强度/隐藏推理过程/
// 回答详略/思考摘要这七项必须能从手机端修改并真实生效:
//  1. 手机发来的 presence 写入被 daemon **照常合并落盘**(store 与 provider 内存态都变);
//  2. setting_status 对这七项按真实状态回报(applied / follow_desktop),绝不回 locked;
//  3. "跟随桌面"(显式空串)也必须能写回去;
//  4. disable_paste_burst 照常按 presence 语义写入。
package server

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// sevenSettingKeys 七项设置的状态 key(与 codexSettingSpecs 对齐,双侧核对)。
var sevenSettingKeys = []string{
	"approval_policy", "sandbox", "model", "effort",
	"hide_agent_reasoning", "model_verbosity", "model_reasoning_summary",
}

// TestSetCodexSettingsAppliesSevenItems 端到端:七项 presence 写入**真的生效**
// —— store、provider 内存态、回显值、逐项状态四处一致,无任何锁定痕迹。
func TestSetCodexSettingsAppliesSevenItems(t *testing.T) {
	srv, p := newProbeTestServer(t, errorProbe{nil}) // ReadyCtx 返回 nil = codex 可达

	// 预置基线:store 与 provider 内存态都是"旧值",等待被写入覆盖。
	base := store.CodexPrefs{
		ApprovalPolicy:    "never",
		Sandbox:           "workspace-write",
		HideReasoning:     true,
		Verbosity:         "high",
		ReasoningSummary:  "detailed",
		DisablePasteBurst: false,
	}
	if err := srv.cfg.Store.SetCodexPrefs(base); err != nil {
		t.Fatalf("预置 CodexPrefs: %v", err)
	}
	if err := srv.cfg.Store.SetDefaultModelID("codex/gpt-5"); err != nil {
		t.Fatalf("预置 model: %v", err)
	}
	if err := srv.cfg.Store.SetDefaultEffort("high"); err != nil {
		t.Fatalf("预置 effort: %v", err)
	}
	p.ApplyPrefs(agent.DesktopPrefs{
		ApprovalPolicy: base.ApprovalPolicy, Sandbox: base.Sandbox,
		Model: "codex/gpt-5", Effort: "high",
		HideReasoning: base.HideReasoning, Verbosity: base.Verbosity,
		ReasoningSummary: base.ReasoningSummary, DisablePasteBurst: base.DisablePasteBurst,
	})

	// 手机发来七项 presence 写入(全部换成不同的值)+ disable_paste_burst。
	ap, sb, md, ef := "untrusted", "read-only", "codex/other", "low"
	hid, vb, sm, pbOn := false, "medium", "concise", true
	got, err := srv.SetCodexSettings(context.Background(), &pb.SetCodexSettingsRequest{
		ApprovalPolicy:        &ap,
		Sandbox:               &sb,
		Model:                 &md,
		Effort:                &ef,
		HideAgentReasoning:    &hid,
		ModelVerbosity:        &vb,
		ModelReasoningSummary: &sm,
		DisablePasteBurst:     &pbOn,
	})
	if err != nil {
		t.Fatalf("SetCodexSettings: %v", err)
	}

	// 1) store:七项全部变成请求值(真的落盘,不是被忽略)。
	st := srv.cfg.Store.CodexPrefs()
	if st.ApprovalPolicy != "untrusted" || st.Sandbox != "read-only" ||
		st.HideReasoning || st.Verbosity != "medium" || st.ReasoningSummary != "concise" {
		t.Fatalf("七项未落盘为请求值: %+v", st)
	}
	if !st.DisablePasteBurst {
		t.Fatalf("disable_paste_burst 未落盘")
	}
	if srv.cfg.Store.DefaultModelID() != "codex/other" || srv.cfg.Store.DefaultEffort() != "low" {
		t.Fatalf("model/effort 未落盘: %q/%q", srv.cfg.Store.DefaultModelID(), srv.cfg.Store.DefaultEffort())
	}

	// 2) provider 内存态同步更新(下一轮 turn/start 将据此携带)。
	pf := p.Prefs()
	if pf.ApprovalPolicy != "untrusted" || pf.Sandbox != "read-only" ||
		pf.Model != "codex/other" || pf.Effort != "low" ||
		pf.HideReasoning || pf.Verbosity != "medium" || pf.ReasoningSummary != "concise" {
		t.Fatalf("provider 内存态未更新: %+v", pf)
	}

	// 3) 回显值 = 请求值。
	if got.ApprovalPolicy != "untrusted" || got.Sandbox != "read-only" ||
		got.Model != "codex/other" || got.Effort != "low" ||
		got.ModelVerbosity != "medium" || got.ModelReasoningSummary != "concise" || got.HideAgentReasoning {
		t.Fatalf("回显值不等于请求值: %+v", got)
	}

	// 4) setting_status:七项按真实状态回报,绝不回 locked。
	states := map[string]string{}
	for _, s := range got.SettingStatus {
		states[s.Key] = s.State
	}
	for _, key := range sevenSettingKeys {
		if states[key] == "" {
			t.Fatalf("%s 缺少状态", key)
		}
		if states[key] == "locked" {
			t.Fatalf("%s 仍回 locked —— 锁定改动未回退干净", key)
		}
	}
	for _, key := range []string{"approval_policy", "sandbox", "model", "effort", "model_verbosity", "model_reasoning_summary"} {
		if states[key] != "applied" {
			t.Fatalf("%s 有值应为 applied, got=%q", key, states[key])
		}
	}
	if states["hide_agent_reasoning"] != "applied" {
		t.Fatalf("hide_agent_reasoning 应为 applied, got=%q", states["hide_agent_reasoning"])
	}
}

// TestSetCodexSettingsBackToFollowDesktop "跟随桌面"必须能写回去:显式空串覆盖旧值。
func TestSetCodexSettingsBackToFollowDesktop(t *testing.T) {
	srv, p := newProbeTestServer(t, errorProbe{nil})

	if err := srv.cfg.Store.SetDefaultModelID("codex/gpt-5"); err != nil {
		t.Fatalf("预置 model: %v", err)
	}
	if err := srv.cfg.Store.SetDefaultEffort("high"); err != nil {
		t.Fatalf("预置 effort: %v", err)
	}
	if err := srv.cfg.Store.SetCodexPrefs(store.CodexPrefs{ApprovalPolicy: "never"}); err != nil {
		t.Fatalf("预置 prefs: %v", err)
	}

	emptyModel, emptyEffort, emptySandbox := "", "", ""
	got, err := srv.SetCodexSettings(context.Background(), &pb.SetCodexSettingsRequest{
		Model:   &emptyModel,
		Effort:  &emptyEffort,
		Sandbox: &emptySandbox,
	})
	if err != nil {
		t.Fatalf("SetCodexSettings: %v", err)
	}
	if srv.cfg.Store.DefaultModelID() != "" || srv.cfg.Store.DefaultEffort() != "" {
		t.Fatalf("显式空串未写回(跟随桌面): %q/%q", srv.cfg.Store.DefaultModelID(), srv.cfg.Store.DefaultEffort())
	}
	if srv.cfg.Store.CodexPrefs().Sandbox != "" {
		t.Fatalf("显式空串 sandbox 未写回: %q", srv.cfg.Store.CodexPrefs().Sandbox)
	}
	if p.Prefs().Model != "" || p.Prefs().Effort != "" || p.Prefs().Sandbox != "" {
		t.Fatalf("provider 内存态未回到跟随桌面: %+v", p.Prefs())
	}
	if got.Model != "" || got.Effort != "" || got.Sandbox != "" {
		t.Fatalf("回显未回到跟随桌面: %+v", got)
	}
}

// TestGetCodexSettingsNeverReportsLocked 只读路径同样不得出现 locked 状态。
func TestGetCodexSettingsNeverReportsLocked(t *testing.T) {
	srv, _ := newProbeTestServer(t, errorProbe{nil})
	got, err := srv.GetCodexSettings(context.Background(), &pb.GetCodexSettingsRequest{})
	if err != nil {
		t.Fatalf("GetCodexSettings: %v", err)
	}
	for _, s := range got.SettingStatus {
		if s.State == "locked" {
			t.Fatalf("%s 仍回 locked —— 锁定改动未回退干净", s.Key)
		}
	}
}
