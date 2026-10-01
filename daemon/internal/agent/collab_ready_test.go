package agent

// collab_ready_test.go —— 「协同能力」判定的回归钉子(README §7 能力接口动态发现)。
//
// 【钉的是什么】daemon 要向手机报告"哪些 agent 已接入跨 agent 委派"(collab:<id>),
// 手机端据此点亮"多 agent 协同"入口 —— 而不是继续摆一个恒假的"开发中"开关。
// 判据必须是**可核验的接线事实**,不能是"配置开关的意愿":开关开着但目录没建、
// 或 MCP 根本没注入,对外宣称具备协同能力就是**假可用**。
//
// 【不钉什么】pi 那条走 piapp.CollabEnvInjection(依赖真实的 daemon 自有目录是否
// 存在),属集成事实,由 L2 真机与 GetHostInfo 端到端覆盖;codex/zcode/hermes 的
// 判据同构(注入要素 = exe + 数据目录,由 collab_inject_test.go 钉形状与开关两态)。
// 这里钉 dsh 的部署级补丁层判据 —— 可单测可控制的文件事实。

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDshCollabReadyFollowsPatchWiring(t *testing.T) {
	// 补丁层文件不存在 = 未接线(生产判据 dshCollabWired 走默认隔离 HOME 的
	// 补丁层路径,hermetic 起见这里钉路径参数化的事实函数,默认入口只是薄转发)。
	missing := filepath.Join(t.TempDir(), "cordis.patch.yml")
	if dshCollabPatchWired(missing) {
		t.Fatal("补丁层文件不存在时不得宣称具备协同能力")
	}

	// 别人的补丁层(无我们的条目)= 未接线:启动侧刻意不覆盖,协同如实报暗。
	dir := t.TempDir()
	foreign := filepath.Join(dir, "cordis.patch.yml")
	if err := os.WriteFile(foreign, []byte("- id: someone-else-plugin\n  name: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if dshCollabPatchWired(foreign) {
		t.Fatal("补丁层无 pocketagent-collab 条目时不得宣称具备协同能力")
	}

	// 已接线(含我们的条目)= 具备。条目 id 含共享服务器名子串,与
	// dshapp.renderCollabPatch 的产物形状一致(collabpatch_test.go 钉死生成侧)。
	wired := filepath.Join(dir, "wired.patch.yml")
	if err := os.WriteFile(wired, []byte("- id: \"pocketagent-collab-mcp\"\n  name: \"@deepseek-ai/dsh-mcp-client\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !dshCollabPatchWired(wired) {
		t.Fatal("补丁层含 pocketagent-collab 条目时应报告具备协同能力")
	}
}

// 编译期钉死:五家 provider 都满足 CollabAware(接口漂移在编译期暴露)。
// codex/zcode/hermes 为 X2 收口新增(注入通道实证见 collab_inject.go 文件头)。
var (
	_ CollabAware = (*CodexProvider)(nil)
	_ CollabAware = (*PiProvider)(nil)
	_ CollabAware = (*ZcodeProvider)(nil)
	_ CollabAware = (*DshProvider)(nil)
	_ CollabAware = (*HermesProvider)(nil)
)
