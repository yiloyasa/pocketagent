// codex_desktop_echo_test.go —— 桌面只读回显"归一/填充"纯函数的钉子。
//
// 【钉住什么】四个只读口子(config/read、thread/goal/get、provider capabilities、
// sandbox readiness)透出到手机的最后一段:daemon 把内部快照翻成 proto 的规则。
// 真实桌面读会被子进程 spawn 卡住测试,所以这里只钉纯函数;
// RPC 接线(fillCodexDesktopEcho)在 probeOverride 生效时必须让路,也一并钉住。
package server

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// TestDesktopConfigSummaryFrom config/read 快照 → proto 的逐字段映射。
func TestDesktopConfigSummaryFrom(t *testing.T) {
	snap := &agent.CodexConfigSnapshot{
		// 实测形状:账号组限定长 id,必须取末段与模型列表对齐(见 desktopModelSlug)。
		Model:          "dexacct.QKbase64==.https%3Aendpoint.gpt-5.4-turbo",
		ModelProvider:  "dex_ai",
		Effort:         "high",
		Summary:        "detailed",
		Verbosity:      "medium",
		SandboxMode:    "workspace-write",
		ApprovalPolicy: "on-request",
		ContextWindow:  400000,
		Raw:            `{"config":{"model":"…原始大对象…"}}`,
	}
	got := desktopConfigSummaryFrom(snap)
	if got == nil {
		t.Fatal("快照非空却返回 nil")
	}
	if got.Model != "gpt-5.4-turbo" {
		t.Fatalf("模型未取末段: %q", got.Model)
	}
	if got.Provider != "dex_ai" || got.Effort != "high" || got.Summary != "detailed" ||
		got.Verbosity != "medium" || got.SandboxMode != "workspace-write" || got.ApprovalPolicy != "on-request" {
		t.Fatalf("字段映射不符: %+v", got)
	}
	// Raw 绝不透传:proto 消息里只有七个对照字段,这里按字段名集合兜底核对。
	wantFields := map[string]bool{
		"model": true, "provider": true, "effort": true, "summary": true,
		"verbosity": true, "sandbox_mode": true, "approval_policy": true,
	}
	fields := got.ProtoReflect().Descriptor().Fields()
	if fields.Len() != len(wantFields) {
		t.Fatalf("CodexDesktopConfigSummary 字段数变了(当前 %d),确认没有把 Raw 透传进来", fields.Len())
	}
	for i := 0; i < fields.Len(); i++ {
		name := string(fields.Get(i).Name())
		if !wantFields[name] {
			t.Fatalf("出现计划外字段 %q(疑似 Raw 透传)", name)
		}
	}
}

// TestDesktopConfigSummaryFromNil nil 快照返回 nil(读取失败路径不造空消息)。
func TestDesktopConfigSummaryFromNil(t *testing.T) {
	if got := desktopConfigSummaryFrom(nil); got != nil {
		t.Fatalf("nil 快照应返回 nil, got %+v", got)
	}
}

// TestDesktopModelSlug 模型 slug 归一规则:只动 ≥4 段的 dexacct. 长 id,
// 普通 slug(含自带版本点的)与形状不符的短 id 一字不动。
func TestDesktopModelSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"dexacct.QKbase64==.https%3Aendpoint.gpt-5.4", "gpt-5.4"},
		{"dexacct.QKbase64==.https%3Aendpoint.gpt-5.4-turbo", "gpt-5.4-turbo"},
		// 少于 4 段:不是实测的基建段形状,原样返回(不瞎猜)。
		{"dexacct.a.gpt-5", "dexacct.a.gpt-5"},
		// 自带版本点的裸 slug 不能被砍尾 —— 一刀切取末段会把 "gpt-5.1-codex" 砍成 "1-codex"。
		{"gpt-5.1-codex", "gpt-5.1-codex"},
		{"o4-mini", "o4-mini"},
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range cases {
		if got := desktopModelSlug(tc.in); got != tc.want {
			t.Fatalf("desktopModelSlug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestProviderCapsFrom 能力声明映射(nil 安全 + 三个布尔逐位一致)。
func TestProviderCapsFrom(t *testing.T) {
	if got := providerCapsFrom(nil); got != nil {
		t.Fatalf("nil 应返回 nil, got %+v", got)
	}
	got := providerCapsFrom(&agent.CodexProviderCaps{ImageGeneration: true, NamespaceTools: true})
	if !got.ImageGeneration || got.WebSearch || !got.NamespaceTools {
		t.Fatalf("能力位映射不符: %+v", got)
	}
}

// TestGoalDetailFrom codex 目标态 + 本地兜底 → proto 的四象限。
func TestGoalDetailFrom(t *testing.T) {
	// ① 电脑端没目标 + 本地也没有 → nil(手机端不占位)。
	if got := goalDetailFrom(nil, ""); got != nil {
		t.Fatalf("双空应返回 nil, got %+v", got)
	}
	// ② 电脑端没目标(读取失败/还没补发)+ 本地有 → objective 兜底本地值,status 留空。
	got := goalDetailFrom(nil, "把测试全跑绿")
	if got == nil || got.Objective != "把测试全跑绿" || got.Status != "" {
		t.Fatalf("本地兜底不符: %+v", got)
	}
	// ③ 电脑端有目标 → 全字段照搬(含 token 预算/消耗)。
	got = goalDetailFrom(&agent.CodexGoalState{
		Objective: "重构 daemon", Status: "active",
		TokensUsed: 1234, TokenBudget: 5000, TimeUsedSeconds: 90,
	}, "")
	if got.Objective != "重构 daemon" || got.Status != "active" ||
		got.TokensUsed != 1234 || got.TokenBudget != 5000 || got.TimeUsedSeconds != 90 {
		t.Fatalf("codex 目标态映射不符: %+v", got)
	}
	// ④ 电脑端有状态但 objective 为空 + 本地有 → objective 补本地,其余保留。
	got = goalDetailFrom(&agent.CodexGoalState{Status: "paused"}, "本地目标")
	if got.Objective != "本地目标" || got.Status != "paused" {
		t.Fatalf("objective 兜底不符: %+v", got)
	}
}

// TestFillCodexDesktopEchoSkipsUnderProbeOverride 测试注入点生效时,
// 桌面回读必须让路(否则 newProbeTestServer 一族会真的拉起 codex 子进程)。
func TestFillCodexDesktopEchoSkipsUnderProbeOverride(t *testing.T) {
	srv, _ := newProbeTestServer(t, errorProbe{nil})
	out := &pb.CodexSettings{}
	srv.fillCodexDesktopEcho(context.Background(), out)
	if out.DesktopEchoError != "" || out.DesktopConfigSummary != nil ||
		out.ProviderCaps != nil || out.SandboxReadiness != "" {
		t.Fatalf("probeOverride 生效时不应做任何桌面回读: %+v", out)
	}
}
