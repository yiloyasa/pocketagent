// codex_commands.go —— codex 会话的斜杠命令面板:skills 即命令(只增)。
//
// 【查证结论(2026-09-19,codex-schema/v2 全 252 个 schema)】codex app-server
// 协议里没有任何 commands/slash/prompts 清单 RPC —— grep "slash|command|prompt"
// 只命中 CommandExec*(shell 执行,与命令面板无关)。codex 与 zcode 不同:
// zcode 的 session 快照自带 slashCommands[],codex 的"命令面"只有技能
// skills/list(SkillsListParams/SkillsListResponse)。
//
// 【产品决策】把 codex 的 skills 以斜杠面板呈现:用户输入 "/" 时列出该会话
// 工作区可用的技能(名称+描述),点选后由手机端以自然语言填充输入框
// ("请使用 xxx 技能:"),发送后 codex 依据技能的 description 自行触发 ——
// 会话内真实可用,不要求 codex 理解 "/xxx" 语法(它也没有这个语法)。
//
// 【数据通路】零新增 codex 调用:直接复用 CapabilitiesDetailed(内部
// capabilities TTL 缓存 30 分钟、重拉失败回旧值,见 capabilities_cache.go;
// skills/list 的字段映射见 codex_capabilities.go 的 fetchCapabilitiesDetailed)。
// 缓存 key 是 cwd —— 技能列表按目录算,会话工作区不同列表就不同,与
// "该会话可用的技能"语义天然对齐。
package agent

import (
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// SlashCommands 返回 cwd 工作区可用的技能,转成斜杠面板的命令形状。
//
// 只保留 enabled 的技能:被禁用的技能发出去也触发不了,列出来只会让用户
// 点一个没反应的命令("为什么被禁用"的解释放在设置页的能力列表里,那里
// 才有 disabled_reason 的展示空间)。
//
// inputHint 有意留空:codex 技能没有 "/name 参数模板"这回事(zcode 的
// inputHint 是 "/goal [pause|resume|…]" 这类参数提示,skills 没有)。手机端
// 点选后据此按自然语言填充("请使用 <name> 技能:"),两源共用一个判断。
//
// 读取失败且无旧缓存时返回 nil:面板是锦上添花,与 server 侧 ListSlashCommands
// 的失败哲学一致 —— 空列表 = 不显示候选条,绝不把错误抛给输入框。
func (p *CodexProvider) SlashCommands(cwd string) []*pb.SlashCommand {
	caps, err := p.CapabilitiesDetailed(cwd)
	if err != nil && len(caps) == 0 {
		return nil
	}
	out := make([]*pb.SlashCommand, 0, len(caps))
	for _, c := range caps {
		// 加载错误占位项(Kind=="skill" 但 Name=="加载错误"、Enabled=false)
		// 被 Enabled 过滤挡住;Name 空串再兜一道,避免出现点不动的空命令。
		if c.Kind != "skill" || !c.Enabled || c.Name == "" {
			continue
		}
		out = append(out, &pb.SlashCommand{
			Name:        c.Name,
			Description: c.Description,
		})
	}
	return out
}
