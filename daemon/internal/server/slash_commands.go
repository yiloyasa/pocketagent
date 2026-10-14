package server

// slash_commands.go —— ThreadService.ListSlashCommands:zcode 会话的斜杠命令面板(只增)。
//
// 【链路】手机输入栏以 "/" 开头且会话是 zcode → 本 RPC → daemon 把该线程绑定的
// zcode 会话快照里的 slashCommands[](name/description/inputHint,解析层在
// zcodeapp/snapshot.go,审计 S-1)透给手机 → 输入框上方弹命令候选条。
//
// 【数据通路,三条按优先级】
//  1. provider 账本:注册表里的 zcode provider 若实现了 zcodeSlashSource(持有
//     "线程 → 会话快照"账本),直接内存取,零进程开销。*agent.ZcodeProvider 已
//     实现(快照在 create/resume/subscribe 三路记账,见 agent/zcode.go
//     recordProjection):命中(非 nil)透出即返回;未记账返回 nil,自然落到
//     底下两条路径兜底,行为与切账本之前等价。
//  2. 会话重挂:线程已绑定外部会话(ExternalSession.SessionID)时,拉一次性
//     app-server 子进程 session/resume 该会话,从应答快照取 slashCommands。
//     与 OptimizePrompt(prompt_opt.go)同款"一次性子进程用完即关"模式:列表是
//     低频只读查询,不挂在 provider 的共享 client 与轮次生命周期上。结果按
//     sessionId 缓存 10 分钟(zcodeModelsTTL 同口径),兜住"每敲一个 / 就冷启
//     一个 node"的频率。
//  3. 无快照:线程还没在电脑上落地(一条消息都没发过)→ 空列表。不为"看一眼
//     命令列表"就在用户真实工作区里凭空创建 zcode 会话(桌面端会话列表会多出
//     垃圾条目);用户发过第一条消息后,下次进会话就能看到面板。
//
// 【失败哲学】非 zcode 会话 / 无快照 / 拉取失败,一律回**空列表**而不是 gRPC
// 错误:命令面板是锦上添花的入口,拉不到就让手机端安静地不显示,绝不 Toast
// 报错打断输入(与 ExportThreadHtml"可降级"的家训一致,只是这里连错误码都省)。
//
// 【四家面板现状(2026-09-30 全量实证)】用户反馈"面板内容不全"后逐家核验:
//   - codex 0.159.2:app-server 协议**没有**斜杠/prompt 命令概念(2026-09-30
//     surface 探针拿到的全量方法面无 commands/slash RPC;rust-v0.159.2 源码里
//     SlashCommand 只存在于 tui/,core/app-server/protocol 零引用、斜杠文本
//     无拦截)——TUI 的 60+ 条斜杠命令是 TUI 本地转换的,协议客户端发 "/xxx"
//     只是一段普通文本。协议层真实可用的"命令"只有 skills(skills/list),
//     面板即全集,不补(补了就是伪造能力位)。
//   - pi 0.99.1:get_commands(extension/prompt/skill 三源)就是"prompt 以
//     /名 开头即可触发"的全集;随包官方文档 docs/rpc-commands.md 明文:内建
//     TUI 命令(/model /theme /compact 等 24 条,bundle 实证)只在交互模式
//     处理,"would not execute if sent via prompt" —— 不列,面板即全集。
//   - zcode 0.16.9:快照 slashCommands[](协议清单 listProtocolSlashCommands)
//     是权威源,但它**漏报**两条:bundle 实证 parseCompactCommand/parseRewind
//     Command 在 app.submitPrompt 轮次启动路径内联解析 "/compact /fork
//     /rewind"(session/send → app.sendInput → submitPrompt 同一解析,TUI 的
//     rewind/fork 分支也是把原文转发 submitPrompt),而协议清单只列
//     [goal,workflow,compact,init,plan]+custom —— fork/rewind 由本文件在
//     聚合层补齐(zcodeWithPromptCommandSupplement),其余各家语义不变。
//   - dsh 0.2.0-rc.2:daemon 唯一通路是 `dsh --profile acp`(ACP 1.4.0),实测
//     bundle:agentCapabilities 无 load 能力、bridge 从不发
//     available_commands_update、prompt 文本无斜杠展开(产品内的 /compact
//     /goal /feedback 命令注册面 dsh-commands 只服务自家 tui/web,不经 ACP)
//     —— 该家无可透传命令,维持空列表。

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/piapp"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// zcodeSlashSource provider 侧"线程 → 会话快照 slashCommands"账本的可选能力
// (与 zcodeGoalPusher / zcodeHistorySource 同款类型断言探测:抽最小接口,
// 单测可注 fake,不依赖真子进程)。生产由 *agent.ZcodeProvider 在补上快照账本后实现。
type zcodeSlashSource interface {
	// SlashCommandsFor 返回线程当前已知的斜杠命令;未知/无快照返回 nil。
	SlashCommandsFor(threadID string) []*pb.SlashCommand
}

// zcodeSlashBudget 一次性子进程冷启 + session/resume 的整段上界。
// 实测 resume 毫秒级、node 冷启 ~3s(见 zcode-p0-probe.md),20s 与
// zcodeStatusBudget 同口径,余量充足。
const zcodeSlashBudget = 20 * time.Second

// ListSlashCommands 返回 zcode 会话可用的斜杠命令(空列表 = 没有面板,不是错误)。
func (s *Server) ListSlashCommands(ctx context.Context, req *pb.ListSlashCommandsRequest) (*pb.ListSlashCommandsResponse, error) {
	t, err := s.requireThread(req.GetThreadId())
	if err != nil {
		return nil, err
	}
	resp := &pb.ListSlashCommandsResponse{}
	// 非 zcode 会话:codex 走"skills 即命令"(agent_commands.go,与 zcode 共用
	// 本 RPC 与同一候选条);pi 走活客户端 get_commands(piapp/commands.go,
	// 0.85.1 实测 56 条真命令,点选回填 "/名 " 原生触发);其余空列表。
	if threadAgentID(t) != "zcode" {
		if threadAgentID(t) == "codex" {
			resp.Commands = append(resp.Commands, s.codexSlashCommands(t)...)
		} else if threadAgentID(t) == "pi" {
			if p, ok := s.cfg.Providers["pi"].(interface {
				SlashCommandsFor(string) []*pb.SlashCommand
			}); ok {
				resp.Commands = append(resp.Commands, p.SlashCommandsFor(t.ID)...)
			}
			if len(resp.Commands) == 0 && req.Refresh && t.Workspace != "" {
				// Read native extension/template/skill commands in this workspace
				// without creating a persistent session or sending a prompt.
				c, err := piapp.Start(piapp.StartOptions{Cwd: t.Workspace, ExtraArgs: []string{"--no-session"}})
				if err == nil {
					defer c.Close()
					if commands, err := c.GetCommands(ctx); err == nil {
						for _, cmd := range commands {
							resp.Commands = append(resp.Commands, &pb.SlashCommand{Name: cmd.Name, Description: cmd.Description})
						}
					}
				}
			}
		}
		return resp, nil
	}
	// 路径 1:provider 账本(命中 = 非 nil,纯内存直接透出;未记账返回 nil,
	// 落到下面的重挂路径兜底,一次性 resume 分支保留不删)。
	if p, ok := s.cfg.Providers["zcode"].(zcodeSlashSource); ok && !req.Refresh {
		if cmds := p.SlashCommandsFor(t.ID); cmds != nil {
			resp.Commands = append(resp.Commands, zcodeWithPromptCommandSupplement(cmds)...)
			return resp, nil
		}
	}
	// 路径 3:线程还没在电脑上落地,没有可重挂的会话 → 空列表(见文件头说明)。
	sid := t.ExternalSession.SessionID
	if sid == "" {
		if discover, ok := s.cfg.Providers["zcode"].(interface {
			DiscoverCommands(context.Context, string) ([]*pb.SlashCommand, error)
		}); ok && req.Refresh {
			commands, err := discover.DiscoverCommands(ctx, t.Workspace)
			if err != nil {
				return nil, errUnavailable("原生命令列表暂不可用: " + err.Error())
			}
			resp.Commands = zcodeWithPromptCommandSupplement(commands)
		}
		return resp, nil
	}
	// 缓存命中:同一条 zcode 会话 10 分钟内只冷启一次子进程(缓存里存的已是
	// 补齐后的完整面板清单,命中即返回,不再二次补充)。
	if cmds, hit := slashCacheGet(sid); hit && !req.Refresh {
		resp.Commands = append(resp.Commands, cmds...)
		return resp, nil
	}
	// 路径 2:一次性子进程重挂该线程绑定的会话,从应答快照取 slashCommands。
	cmds, err := zcodeSlashFetch(ctx, sid)
	if err != nil {
		log.Printf("[slash] 拉取 zcode 斜杠命令失败(回空列表)session=%s: %v", sid, err)
		return resp, nil
	}
	cmds = zcodeWithPromptCommandSupplement(cmds)
	if len(cmds) > 0 {
		slashCachePut(sid, cmds)
	}
	resp.Commands = append(resp.Commands, cmds...)
	return resp, nil
}

// zcodePromptCommandSupplement zcode 快照协议清单的漏报补充(bundle 实证可执行
// 但原生 slashCommands 不列的命令)。字段逐条照抄本机 zcode.cjs 0.16.9 的内建
// 命令注册面 $R(name/summary/usage,与 goal/compact 等原生条目同源):
//
//   - fork:  parseRewindCommand(bundle 函数 vko)在 app.submitPrompt 轮次启动
//     路径解析 "/fork [latest|checkpointId]";session/send → app.sendInput →
//     submitPrompt 走同一解析,TUI 的 fork 分支也是把原文转发 submitPrompt。
//   - rewind: 同一 parseRewindCommand 解析 "/rewind [latest|checkpointId]"
//     (无参 = status 回读),session/send 文本直接可触发。
//
// 【为什么由 daemon 补而不是改原生】协议清单(listProtocolSlashCommands)只列
// WVo[goal,workflow,compact,init]+plan+custom,fork/rewind 是 app 层文本命令
// 而非 protocol 声明面 —— 客户端侧补齐是唯一改动点,版本漂移风险由"只补不删"
// 约束住:原生清单将来若把这两条列进来,去重逻辑让本补充自动失效。
// workflow 不在此列:dynamicWorkflowEnabled=false 时 bundle 明确不展开
// /workflow(resolveZCodeBuiltinPromptCommand 返回 void),原生不列就是真不可用,
// 补了反而伪造能力位。
var zcodePromptCommandSupplement = []struct {
	name        string
	description string
	inputHint   string
}{
	{"fork", "Fork a new session from a workspace checkpoint.", "/fork [latest|checkpointId]"},
	{"rewind", "Inspect or restore workspace checkpoints.", "/rewind [latest|checkpointId]"},
}

// zcodeWithPromptCommandSupplement 把漏报补充并入原生清单:原生条目全序在前,
// 补充条目按 name(大小写不敏感)去重后追加;入参 nil(拉取失败/无快照)原样
// 返回 nil —— 失败路径绝不凭空发面板。入参切片不被本函数改动(账本里那份
// 是 provider 的共享底层数组,只读)。
func zcodeWithPromptCommandSupplement(cmds []*pb.SlashCommand) []*pb.SlashCommand {
	if cmds == nil {
		return nil
	}
	have := make(map[string]struct{}, len(cmds)+len(zcodePromptCommandSupplement))
	for _, c := range cmds {
		if c != nil {
			have[strings.ToLower(c.Name)] = struct{}{}
		}
	}
	out := make([]*pb.SlashCommand, len(cmds), len(cmds)+len(zcodePromptCommandSupplement))
	copy(out, cmds)
	for _, s := range zcodePromptCommandSupplement {
		if _, ok := have[s.name]; ok {
			continue
		}
		have[s.name] = struct{}{}
		out = append(out, &pb.SlashCommand{Name: s.name, Description: s.description, InputHint: s.inputHint})
	}
	return out
}

// zcodeSlashFetch 拉取实现的注入点:生产走一次性 ZCode 子进程(见下方
// fetchViaZcode),单测换成 fake 以钉住 RPC 层行为(不依赖本机装没装 ZCode)。
var zcodeSlashFetch = fetchViaZcode

// fetchViaZcode 拉一次性 app-server,session/resume 指定会话,解析应答快照里的
// slashCommands[](解析统一走 zcodeapp.ParseSessionSnapshot,快照形状的单一事实来源)。
func fetchViaZcode(ctx context.Context, sessionID string) ([]*pb.SlashCommand, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c, err := zcodeapp.Start(zcodeapp.StartOptions{})
	if err != nil {
		return nil, err
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(ctx, zcodeSlashBudget)
	defer cancel()
	raw, err := c.CallCtx(ctx, "session/resume", map[string]interface{}{"sessionId": sessionID})
	if err != nil {
		return nil, err
	}
	snap, err := zcodeapp.ParseSessionSnapshot(raw)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.SlashCommand, 0, len(snap.SlashCommands))
	for _, sc := range snap.SlashCommands {
		out = append(out, &pb.SlashCommand{
			Name:        sc.Name,
			Description: sc.Description,
			InputHint:   sc.InputHint,
		})
	}
	return out, nil
}

// ---- 按 sessionId 的命令列表缓存(TTL)----
// 【为什么是包级而不是 Server 字段】server.go 归并行智能体占用,本文件只增不改;
// 缓存键是 zcode sessionId(全局唯一),包级 map 语义等价,单测用 slashCacheFlush 清场。

var (
	slashMu       sync.Mutex
	slashCache    = map[string]slashCacheEntry{}
	slashCacheTTL = 10 * time.Minute // 与 agent.zcodeModelsTTL 同口径
)

type slashCacheEntry struct {
	commands []*pb.SlashCommand
	at       time.Time
}

func slashCacheGet(sessionID string) ([]*pb.SlashCommand, bool) {
	slashMu.Lock()
	defer slashMu.Unlock()
	e, ok := slashCache[sessionID]
	if !ok || time.Since(e.at) > slashCacheTTL {
		return nil, false
	}
	return e.commands, true
}

func slashCachePut(sessionID string, cmds []*pb.SlashCommand) {
	slashMu.Lock()
	defer slashMu.Unlock()
	slashCache[sessionID] = slashCacheEntry{commands: cmds, at: time.Now()}
}

// slashCacheFlush 清空缓存(单测隔离用)。
func slashCacheFlush() {
	slashMu.Lock()
	defer slashMu.Unlock()
	slashCache = map[string]slashCacheEntry{}
}
