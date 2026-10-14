// hermes_slash.go —— hermes 斜杠命令账本与 SlashCommandsFor(2026-10-14 slash 位化)。
//
// 【为什么是账本,不是主动拉取】ACP v0.11.2 的 agent 方法全集(venv acp
// meta.py:AGENT_METHODS 13 个:authenticate/initialize/session_cancel/
// session_close/session_fork/session_list/session_load/session_new/
// session_prompt/session_resume/session_set_config_option/session_set_mode/
// session_set_model)里**没有任何"取命令清单"的请求方法** —— 命令面是适配器经
// session/update 的 available_commands_update 变体**纯推送**广告(commands.py:
// 79-95 `_available_commands` + :85 `_send_available_commands_update`)。所以
// daemon 没有第二数据源可问:账本空就是空面板,绝不冷启适配器"问一遍清单"
// (pi 有 get_commands 可问、zcode 有快照可拉,hermes 连问的接口都不存在)。
//
// 【广告到达时机(live 实测,探针输出原文见 docs/reviews/2026-10-14/
// d1-hermes-slash-wiring.md)】适配器 server.py:596/668 在 session/new|load|
// resume|fork 的应答之后经 loop.call_soon 调度 _send_available_commands_update
// ("right after the current response is queued")。2026-10-14 真机抓帧次序:
//
//	13.169 resp  {"id":2, "result":{"sessionId":…}}   ← session/new 应答
//	13.170 notif {"update":{"availableCommands":[9 条],"sessionUpdate":
//	              "available_commands_update"}}        ← 应答后 ~1ms 到达
//
// 即**第一条广告早于 Run 的事件泵安装**(泵装在 ensureSession 之后,中间还隔着
// 一次 set_model 往返)。旧接线里这段窗口没有钩子 → 帧在 hermesapp readLoop 的
// `h != nil && h.Notif != nil` 判空处被静默丢,谁也接不住。因此账本写入点有两处,
// 缺一不可:
//
//  1. 会话建立窗口(hermes.go 的 createSession/remountSession):装只认命令帧的
//     捕获器,应答后**保留安装**直到 Run 的泵整面替换它 —— 接住"应答后 1ms"
//     那条(成功路径没有空窗:泵安装在 ensureSession 与全部可能 return 之后,
//     见 Run 的代码顺序;失败路径立刻闭窗卸载);
//  2. 轮次事件泵(hermes.go Run 消费者 goroutine):命令帧若在轮内到达(未来
//     适配器改时机 / fork 会话等)照样入账;projectSessionUpdate 里的
//     KNOWN_PASSTHROUGH 台账留痕照旧保留,二者不互斥。
//
// 【条目形状(ACP v0.11.2 schema.py:2138 AvailableCommand,live 实测逐字段核)】
// name 必有;description 必有(适配器恒填);input 是 UnstructuredCommandInput
// {hint},可缺席 —— live 帧里 hint 只在 model/steer/queue 三条在场,其余
// (help/tools/context/reset/compress/version)无 input。映射到
// pb.SlashCommand{Name, Description, InputHint},与 zcode 三字段同形;手机端对
// 带 inputHint 的条目走"/命令 "回填(命令名原样触发,适配器 slash 分派实证)。
//
// 【红线:不硬编码】本文件不含任何命令名单字面量 —— 面板内容只能来自适配器
// 实际广告的帧。账本没数据就返回 nil(空面板),绝不拿探针报告里的 9 条名单
// 充数:那是取证事实,不是协议数据。
package agent

import (
	"encoding/json"
	"strings"
	"sync"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/store"
)

// hermesAvailableCommand 是 available_commands_update.availableCommands[] 的一项
// (ACP AvailableCommand;json 字段对 schema alias:name/description/input.hint)。
type hermesAvailableCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Input       *struct {
		Hint string `json:"hint"`
	} `json:"input"`
}

// parseHermesAvailableCommands 从一条 session/update 原始帧里解出命令广告。
// 返回 (会话 id, 命令清单, 是否命令帧):ok=false 表示帧不是
// available_commands_update(其它变体/坏帧),调用方原样跳过,不误记也不误计数。
//
// 【为什么直接解 ev.Raw 而不是用 hermesapp 已解析的字段】hermesapp 的
// StreamEvent 只把条数暴露为 CommandsSeen(acp.go:293/465),原始条目数组不在
// 导出面上;本批次改动面只允许 internal/agent(slash 位化契约),不动 hermesapp
// 解析层 —— 同一份字节(ev.Raw 就是 updateBody 的原文)在 agent 侧解一次,
// 语义与 hermesapp 的 CommandsSeen 完全同源(同一帧、同一变体判别)。
//
// 显式空数组(availableCommands:[])会得到空清单而非 nil:那是适配器"没有命令"
// 的显式事实(与 zcode"空数组也算覆盖"同纪律);nil 只留给"从未收到广告"。
func parseHermesAvailableCommands(raw json.RawMessage) (sessionID string, cmds []*pb.SlashCommand, ok bool) {
	var envelope struct {
		SessionID string `json:"sessionId"`
		Update    struct {
			SessionUpdate string                   `json:"sessionUpdate"`
			Commands      []hermesAvailableCommand `json:"availableCommands"`
		} `json:"update"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &envelope) != nil {
		return "", nil, false
	}
	if envelope.Update.SessionUpdate != "available_commands_update" {
		return "", nil, false
	}
	out := make([]*pb.SlashCommand, 0, len(envelope.Update.Commands))
	for _, c := range envelope.Update.Commands {
		if strings.TrimSpace(c.Name) == "" {
			continue // 空名条目点不动,不入面板(与 codex_commands.go 空名过滤同纪律)
		}
		sc := &pb.SlashCommand{Name: c.Name, Description: c.Description}
		if c.Input != nil {
			sc.InputHint = c.Input.Hint
		}
		out = append(out, sc)
	}
	return envelope.SessionID, out, true
}

// hermesSlashCapture 是"会话建立窗口"的命令捕获器(见文件头写入点 1)。
// 生命周期:begin → 装进 client 通知钩子 → bind(会话号)→ commit(建立成功,
// 缓冲帧入账;此后直达记账)→ 被 Run 的泵整面替换;失败路径 close(丢弃缓冲,
// 迟到帧不再入账 —— 半挂载的会话不得把命令面留在账本里)。
type hermesSlashCapture struct {
	p        *HermesProvider
	threadID string

	mu        sync.Mutex
	sid       string             // 建立应答返回后回填(绑定前不知道会话 id)
	buffered  []hermesSlashFrame // 建立应答返回前到达的命令帧(提交时按 sid 对表)
	committed bool               // 建立成功:此后同会话帧直达账本
	done      bool               // 失败/收场:闭窗,迟到帧一律丢弃
	replayed  int                // 非命令帧计数(重挂载"历史回放 N 帧"日志的原口径)
}

// hermesSlashFrame 是缓冲的一条命令帧(带会话 id,提交时对表)。
type hermesSlashFrame struct {
	sid  string
	cmds []*pb.SlashCommand
}

// beginSlashCapture 为一次会话建立开捕获窗口;th == nil(一次性/无归属轮次)
// 不记账也不开窗,原样维持"无钩子"行为。
func (p *HermesProvider) beginSlashCapture(th *store.Thread) *hermesSlashCapture {
	if th == nil || th.ID == "" {
		return nil
	}
	return &hermesSlashCapture{p: p, threadID: th.ID}
}

// handler 是装进 client 的通知钩子(形状对 SetTurnHandlers 的 notif 参数):
// 已提交的情况下同会话命令帧直达账本;未提交(建立 RPC 在途)先缓冲,提交时
// 对表入账;其余帧只计数(重挂载历史回放的"回放 N 帧"日志沿用计数,命令帧不
// 再混进回放数 —— 它是控制广告,不是历史)。
//
// 【锁序】账本写在 cap.mu 内进行(锁序 cap.mu → threadMu,单向;没有任何路径
// 反向取锁),为的是让"缓冲帧提交"与"提交后新帧直达"互斥 —— 否则并发下旧缓冲
// 可能覆盖新广告(丢更新)。开销可忽略:账本写入是纯内存 map 赋值。
func (c *hermesSlashCapture) handler(raw json.RawMessage) {
	sid, cmds, ok := parseHermesAvailableCommands(raw)
	if !ok {
		c.mu.Lock()
		c.replayed++
		c.mu.Unlock()
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done {
		return
	}
	if !c.committed {
		// 建立应答还没回来(或刚回来还没提交):先缓冲,提交后按 sessionId
		// 对表入账 —— 共享适配器上别家会话的帧不得混进本线程的面板,建立失败
		// 的帧也不得留下(半挂载账本)。
		c.buffered = append(c.buffered, hermesSlashFrame{sid: sid, cmds: cmds})
		return
	}
	if sid == c.sid {
		c.p.setSlashCommands(c.threadID, cmds)
	}
}

// bind 用建立应答的 sessionId 绑定窗口(提交前不影响缓冲语义)。
func (c *hermesSlashCapture) bind(sid string) {
	c.mu.Lock()
	c.sid = sid
	c.mu.Unlock()
}

// commit 建立成功:缓冲帧按绑定 sid 对表入账,窗口转"直达"模式(此后同会话
// 命令帧即时入账;安装面留给 Run 的泵整面替换 —— 应答后 1ms 的广告正好落
// 在这个窗口里)。只 commit 一次,重复调用为幂等空操作。锁序同 handler。
func (c *hermesSlashCapture) commit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.committed || c.done {
		return
	}
	c.committed = true
	for _, f := range c.buffered {
		if f.sid == c.sid {
			c.p.setSlashCommands(c.threadID, f.cmds)
		}
	}
	c.buffered = nil
}

// close 失败收场闭窗:丢弃缓冲,此后迟到的帧不再入账(安装面由调用方卸载)。
func (c *hermesSlashCapture) close() {
	c.mu.Lock()
	c.done = true
	c.buffered = nil
	c.mu.Unlock()
}

// replayedFrames 窗口内收到的非命令帧数(重挂载日志用;互斥读,与 readLoop
// 的写入无竞争)。
func (c *hermesSlashCapture) replayedFrames() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.replayed
}

// setSlashCommands 覆盖登记一条线程的命令账本(纯内存,threadMu 与 threadMap
// 同锁纪律;空 threadID 不记账,防把无归属的帧挂到空键上)。
func (p *HermesProvider) setSlashCommands(threadID string, cmds []*pb.SlashCommand) {
	if threadID == "" {
		return
	}
	p.threadMu.Lock()
	defer p.threadMu.Unlock()
	if p.slashByThread == nil {
		p.slashByThread = map[string][]*pb.SlashCommand{}
	}
	p.slashByThread[threadID] = cmds
}

// SlashCommandsFor 返回该线程账本里的斜杠命令(server 侧 ListSlashCommands 的
// hermes 数据源,与 pi/zcode 的 SlashCommandsFor 同签名同语义)。
//
// 未记账(会话还没建立过 / daemon 刚重启且未重挂载 / 适配器从未广告该会话)
// 返回 nil —— "无会话无清单,绝不为拉清单冷启进程"(与 pi 的 aliveClient 语义
// 同纪律;本家连可拉的协议方法都没有,见文件头)。server 侧把 nil 与空都能安静
// 收成空面板。
func (p *HermesProvider) SlashCommandsFor(threadID string) []*pb.SlashCommand {
	p.threadMu.Lock()
	defer p.threadMu.Unlock()
	return p.slashByThread[threadID]
}

// noteCommandsFromEvent 事件泵消费侧的命令帧补记(文件头写入点 2)。命令帧在
// 轮内到达时(含显式空广告)照记;变体判据用 ev.Variant 而不是 CommandsSeen,
// 显式空数组(CommandsSeen==0)也是"没有命令"的显式事实,与从未广告(nil)区分。
// expectSID 是本次会话号:共享适配器上别家会话的广告不得挂进本线程账本(与
// 泵生产者的 sessionId 过滤同判据,这里是消费侧的二道防线 —— 压缩轮等自装钩子
// 的路径没有泵过滤器兜底)。
// 台账留痕(projectSessionUpdate 的 recordLedgerOnly)不在此处、也不受影响。
func (p *HermesProvider) noteCommandsFromEvent(threadID, expectSID string, ev hermesapp.StreamEvent) {
	if threadID == "" || ev.Variant != "available_commands_update" {
		return
	}
	sid, cmds, ok := parseHermesAvailableCommands(ev.Raw)
	if !ok || sid != expectSID {
		return
	}
	p.setSlashCommands(threadID, cmds)
}

// 编译期钉死:hermes 命令面与 pi/zcode 同形状(host.go 的 piShapedSlashSource
// 位断言与 server ListSlashCommands 路由都按形状走,不按 agent 名分支)。
var _ interface {
	SlashCommandsFor(string) []*pb.SlashCommand
} = (*HermesProvider)(nil)
