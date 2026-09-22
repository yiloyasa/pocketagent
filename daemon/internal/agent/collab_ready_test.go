package agent

// collab_ready_test.go —— 「协同能力」判定的回归钉子(README §7 能力接口动态发现)。
//
// 【钉的是什么】daemon 要向手机报告"哪些 agent 已接入跨 agent 委派"(collab:<id>),
// 手机端据此点亮"多 agent 协同"入口 —— 而不是继续摆一个恒假的"开发中"开关。
// 判据必须是**可核验的接线事实**,不能是"配置开关的意愿":开关开着但目录没建、
// 或 MCP 根本没注入,对外宣称具备协同能力就是**假可用**。
//
// 【不钉什么】pi 那条走 piapp.CollabEnvInjection(依赖真实的 daemon 自有目录是否
// 存在),属集成事实,由 L2 真机与 GetHostInfo 端到端覆盖;这里只钉可单测可控制的
// dsh 会话级接线。

import (
	"testing"

	"github.com/pocketagent/daemon/internal/dshapp"
)

func TestDshCollabReadyFollowsWiring(t *testing.T) {
	t.Cleanup(func() { dshapp.SetSessionMCPServers(nil) })
	p := NewDshProvider()

	dshapp.SetSessionMCPServers(nil)
	if p.CollabReady() {
		t.Fatal("未注入会话级 MCP 时不得宣称具备协同能力(否则手机端点亮的是假入口)")
	}

	dshapp.SetSessionMCPServers([]map[string]interface{}{{
		"name":    "pocketagent-collab",
		"command": "x",
	}})
	if !p.CollabReady() {
		t.Fatal("注入会话级 MCP 后应报告具备协同能力")
	}

	dshapp.SetSessionMCPServers(nil)
	if p.CollabReady() {
		t.Fatal("撤回接线后应重新报告不具备")
	}
}

// 编译期钉死:两家 provider 都满足 CollabAware(接口漂移在编译期暴露)。
var (
	_ CollabAware = (*DshProvider)(nil)
	_ CollabAware = (*PiProvider)(nil)
)
