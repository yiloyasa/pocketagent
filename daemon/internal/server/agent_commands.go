// agent_commands.go —— ListSlashCommands 的 codex 分支:skills 即命令(只增)。
//
// 【与 slash_commands.go 的分工】那边是 RPC 入口 + zcode 快照通路(zcode 面板
// 智能体落库,本文件零改动它);这边只补 codex 路由:把该会话工作区可用的
// codex skills 映射成同一形状的 SlashCommand(name/description,inputHint 留空),
// 手机端候选条双源(zcode 快照 / codex skills)共用同一个 RPC、同一套 UI。
// pi 等其它 agent 没有命令面,维持空列表不动。
//
// 【失败哲学】provider 不在场 / 不支持 / 读取失败,一律安静返回 nil(空面板),
// 与 slash_commands.go 文件头的约定一致:拉不到就让手机端安静地不显示,
// 绝不报错打断输入。
package server

import (
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// codexSlashSource codex provider 的斜杠命令源。
//
// 【为什么走类型断言而不是直接断言 *agent.CodexProvider】与 zcodeSlashSource /
// zcodeGoalPusher 同款最小接口:单测可注入 fake(不必构造真 codex 子进程),
// 将来实现方变化也不牵连 server。生产由 *agent.CodexProvider 实现
// (agent/codex_commands.go)。
type codexSlashSource interface {
	// SlashCommands 返回 cwd 工作区可用的技能命令;无可用技能/读取失败返回 nil。
	SlashCommands(cwd string) []*pb.SlashCommand
}

// codexSlashCommands 取该 codex 会话工作区的技能命令。
//
// cwd 用线程绑定的 Workspace:codex 的 skills/list 按 cwd 返回"该目录可用"的
// 技能(repo 级技能只在其仓库里出现),这正是"该会话可用"的语义;
// Workspace 为空时 agent 层回退当前目录(codex 的默认行为,与设置页一致)。
func (s *Server) codexSlashCommands(t *store.Thread) []*pb.SlashCommand {
	if t == nil {
		return nil
	}
	p, ok := s.cfg.Providers["codex"]
	if !ok {
		return nil
	}
	src, ok := p.(codexSlashSource)
	if !ok {
		return nil
	}
	return src.SlashCommands(t.Workspace)
}
