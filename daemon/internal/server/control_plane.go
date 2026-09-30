// control_plane.go —— GetControlPlane:daemon 全部可控域的总览(聚合读,规格 §14)。
//
// 【它解决什么】预算、协同、agent 设置、记忆、未知事件、审批……每一条控制通道
// 都已存在且可用,但散落在七八个页面里:手机上一个页面看不全,也就谈不上
// "一盘棋的控制"。本文件把各域的**当前状态**聚合成一次 RPC 回答 —— 一个页面
// 看全,就地跳到各自的写通道去改。
//
// 【它刻意不做的事:不新增任何写路径、不落任何新配置文件】ControlItem 只是呈现;
// 切换动作继续走既有的 SetRuntimePolicy / SetCollabPolicy / SetCodexSettings /
// AcknowledgeUnknownEvent / ClearThreadMemory。给总览再配一套写通道,等于给同一份
// 状态造第二份真值 —— 那正是"两边显示不一致"这类事故的标准来源。
//
// 【同源纪律】每个域的数值都取自既有 RPC 同一把锁、同一个函数的结果:budget 用
// GetRuntimePolicy 同款的 budgetMu + Effective("",...),collab 用 GetCollabPolicy
// 同款的 collabPolicySnapshot().Effective()。两个界面看到同一个数,靠的是共用同一
// 份读法,不是约定。
//
// 【单域降级】总览页是聚合读:任何一个域的组装故障(panic/数据源缺失)只降级它
// 自己(status_code=2 + 一句如实说明),其余域照常返回 —— 一片乌云不该遮住整张天。
//
// 【人话纪律】summary/label/hint 全部 daemon 给中文,与 event_translate 同款红线:
// 不编造,状态取不到就写"未检测到/未报告";词认不出就照抄原文。客户端只按
// id 匹配与排版,不做任何术语翻译。
package server

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hostenv"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/version"
)

// 控制面域的稳定 id(客户端按它匹配,契约注释里已钉死取值)。
const (
	domainBudget     = "budget"
	domainCollab     = "collab"
	domainAgents     = "agents"
	domainMemory     = "memory"
	domainDialysis   = "dialysis" // 未知事件透析(字段级拆读 + 五处置)
	domainApprovals  = "approvals"
	domainTransports = "transports"
	domainResources  = "resources"
)

// GetControlPlane 一次返回全部可控域的当前状态。
func (s *Server) GetControlPlane(_ context.Context, _ *pb.GetControlPlaneRequest) (*pb.ControlPlane, error) {
	osName, _, _ := platformInfo()
	out := &pb.ControlPlane{
		DaemonVersion: version.App,
		HostName:      hostDisplayName(),
		Os:            osName,
		UptimeUnixMs:  s.startedAt.UnixMilli(),
		DataDirLabel:  dataDirLabel(s.cfg.DataDir),
	}
	// 域清单钉死次序:客户端总览页按这份次序排版,次序抖动 = 前后两次刷新
	// 版面换来换去。新增域在表尾追加,不改既有项。
	for _, d := range []struct {
		id, title string
		build     func() *pb.ControlDomain
	}{
		{domainBudget, "资源预算", s.controlDomainBudget},
		{domainCollab, "多 Agent 协同", s.controlDomainCollab},
		{domainAgents, "Agent 接入", s.controlDomainAgents},
		{domainMemory, "会话记忆", s.controlDomainMemory},
		{domainDialysis, "未知事件", s.controlDomainDialysis},
		{domainApprovals, "审批请求", s.controlDomainApprovals},
		{domainTransports, "连接通道", s.controlDomainTransports},
		{domainResources, "资源用量", s.controlDomainResources},
	} {
		out.Domains = append(out.Domains, buildControlDomain(d.id, d.title, d.build))
	}
	return out, nil
}

// buildControlDomain 组装一个域,panic 就地降级。
//
// 【为什么用 recover 而不是让错误冒出去】总览页的价值在"全":某个域的数据源
// 出了没料到的状况,把整条 RPC 掀翻,用户连其它七个域也看不到了 —— 那比不修还糟。
// 降级域保留 id/title(客户端才知道哪里坏了),summary 如实说明,故障进 daemon 日志。
func buildControlDomain(id, title string, build func() *pb.ControlDomain) (d *pb.ControlDomain) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[controlplane] 域 %s 组装失败,已单独降级: %v", id, r)
			d = &pb.ControlDomain{
				Id: id, Title: title, StatusCode: 2,
				Summary: "这一域的状态暂时取不到(内部错误),其余域不受影响",
			}
		}
	}()
	return build()
}

// degradedControlDomain 数据源缺失时的如实降级(与 panic 降级同一呈现语义)。
func degradedControlDomain(id, title, reason string) *pb.ControlDomain {
	return &pb.ControlDomain{Id: id, Title: title, StatusCode: 2, Summary: reason}
}

// ---- budget:资源预算 ---------------------------------------------------------

// controlDomainBudget 与 GetRuntimePolicy 同源:同一把读锁、同一次 Effective("",
// "", "", "")。两个界面看到的数永远一致,靠的是共用同一份读法。
func (s *Server) controlDomainBudget() *pb.ControlDomain {
	s.budgetMu.RLock()
	eff := s.budgetPolicy.Effective("", "", "", "")
	s.budgetMu.RUnlock()
	d := &pb.ControlDomain{
		Id: domainBudget, Title: "资源预算", Writable: true, StatusCode: 0,
		Summary: fmt.Sprintf("全局预算已生效:并行轮次上限 %d、事件缓冲 %s;改动立即生效,不必重启",
			derefInt(eff.MaxParallelTurns), formatBytes(derefInt64(eff.RAMEventBufferBytes))),
	}
	// 数值域的写通道是 SetRuntimePolicy(提交完整策略,CAS 保护);这里 current
	// 展示的是 effective_global —— 用户配置可能缺席,生效值必须由 daemon 算好。
	d.Items = []*pb.ControlItem{
		{
			Id: "budget.global.ram_working_bytes", Label: "工作内存上限",
			Current: formatBytes(derefInt64(eff.RAMWorkingBytes)), Kind: "number",
			Hint: "单个轮次在内存里干活的额度;到线触发压缩,不丢会话内容。设为 0 表示封禁",
		},
		{
			Id: "budget.global.ram_event_buffer_bytes", Label: "事件缓冲上限",
			Current: formatBytes(derefInt64(eff.RAMEventBufferBytes)), Kind: "number",
			Hint: "每个轮次事件的内存缓冲;到线把事件落盘,消息一条不丢。设为 0 表示封禁",
		},
		{
			Id: "budget.global.ram_session_index_bytes", Label: "会话索引上限",
			Current: formatBytes(derefInt64(eff.RAMSessionIndexBytes)), Kind: "number",
			Hint: "会话索引在内存里的占用上限。设为 0 表示封禁",
		},
		{
			Id: "budget.global.disk_conversation_bytes", Label: "会话存档空间",
			Current: formatBytes(derefInt64(eff.DiskConversationBytes)), Kind: "number",
			Hint: "会话内容在磁盘上的空间上限;到线会触发压缩。设为 0 表示封禁",
		},
		{
			Id: "budget.global.disk_cache_bytes", Label: "缓存空间",
			Current: formatBytes(derefInt64(eff.DiskCacheBytes)), Kind: "number",
			Hint: "可再生缓存的磁盘上限;触顶优先清缓存,随时可重建。设为 0 表示封禁",
		},
		{
			Id: "budget.global.disk_artifact_bytes", Label: "产物文件空间",
			Current: formatBytes(derefInt64(eff.DiskArtifactBytes)), Kind: "number",
			Hint: "导出文件等运行产物的磁盘上限。设为 0 表示封禁",
		},
		{
			Id: "budget.global.max_parallel_turns", Label: "并行轮次上限",
			Current: fmt.Sprintf("%d", derefInt(eff.MaxParallelTurns)), Kind: "number",
			Hint: "同时进行的轮次数量;调大更并行,也更快耗尽其它额度",
		},
	}
	return d
}

// ---- collab:多 agent 协同 ----------------------------------------------------

// controlDomainCollab 与 GetCollabPolicy 同源(collabPolicySnapshot + Effective)。
func (s *Server) controlDomainCollab() *pb.ControlDomain {
	eff := s.collabPolicySnapshot().Effective()
	toggle := func(on bool) string {
		if on {
			return "开"
		}
		return "关"
	}
	d := &pb.ControlDomain{Id: domainCollab, Title: "多 Agent 协同", Writable: true, StatusCode: 0}
	switch {
	case !eff.CollabEnabled():
		// 总开关关了,子开关再开也没用 —— summary 如实说总闸的状态。
		d.Summary = "协同已关闭:Agent 之间互不派活"
		d.StatusCode = 1
	default:
		d.Summary = fmt.Sprintf("协同已开启:委派与扇出可用,单次扇出上限 %s", membersText(eff.MaxMembers()))
	}
	d.Items = []*pb.ControlItem{
		{
			Id: "collab.collaboration_enabled", Label: "协同总开关",
			Current: toggle(eff.CollabEnabled()), Kind: "toggle",
			Hint: "关掉后 Agent 之间不能再互相派活,各自会话不受影响",
		},
		{
			Id: "collab.native_delegate_enabled", Label: "委派子任务",
			Current: toggle(eff.DelegateEnabled()), Kind: "toggle",
			Hint: "允许 Agent 把子任务派给别的 Agent 执行",
		},
		{
			Id: "collab.fanout_enabled", Label: "并行扇出",
			Current: toggle(eff.Fanout()), Kind: "toggle",
			Hint: "一条指令同时发给多家 Agent 并行执行",
		},
		{
			Id: "collab.max_parallel_members", Label: "扇出成员上限",
			// number 型条目给纯数值字符串,客户端可直接预填编辑框;"0 = 不限"
			// 的语义放 hint 与 summary 的人话里。
			Current: fmt.Sprintf("%d", eff.MaxMembers()), Kind: "number",
			Hint: "单次扇出最多同时给几家;0 表示不限",
		},
	}
	return d
}

// membersText 扇出成员数的人话:<=0 是"不限",不是"零家"。
func membersText(n int) string {
	if n <= 0 {
		return "不限"
	}
	return fmt.Sprintf("%d 家", n)
}

// ---- agents:Agent 接入形态 ---------------------------------------------------

// controlDomainAgents 各 agent 的在线形态、能力位数,以及 codex 的桌面偏好
// select 项。形态数据源与 GetRuntimeState 同一份 contractReports(启动时生成、
// 之后只读 —— 它反映接入形态,不随运行变化)。
func (s *Server) controlDomainAgents() *pb.ControlDomain {
	reports := s.contractReportsSnapshot()
	d := &pb.ControlDomain{Id: domainAgents, Title: "Agent 接入", Writable: true}
	// 写通道:每家 agent 的桌面偏好与原生设置(SetCodexSettings / ListAgentConfigOptions)
	// 都在;没有 agent 时无所谓可写,如实按"有通道"声明。
	bad := 0
	for _, rep := range reports {
		if !rep.OK() {
			bad++
		}
		name := rep.Descriptor.DisplayName
		if name == "" {
			// 连名字都没申报的 agent:照抄注册 id,不编一个好看的名字。
			name = rep.AgentID
		}
		d.Items = append(d.Items,
			&pb.ControlItem{
				Id: "agents." + rep.AgentID + ".session", Label: name + " · 会话形态",
				Current: sessionModelText(rep.Descriptor.SessionModel), Kind: "info",
				Hint: "会话与恢复数据存在谁那边;申报不合格会在运行时状态页列明细",
			},
			&pb.ControlItem{
				Id: "agents." + rep.AgentID + ".capabilities", Label: name + " · 能力申报",
				Current: fmt.Sprintf("%d 项", len(rep.Descriptor.SortedCapabilities())), Kind: "info",
				Hint: "该 Agent 声明支持的入口(终端/文件/审批等);缺报的必需入口会被核验标出",
			},
		)
		// 桌面偏好 select 只给 codex:六键的写通道是 SetCodexSettings 的同名
		// 字段,而非 codex agent 走 setAgentSettings(AgentDefaults:只认
		// model/effort/native_mode),approval/sandbox/verbosity/summary 会被
		// 静默忽略 —— 给它们摆入口就是造一份写了不生效的假账。
		if rep.AgentID == "codex" {
			d.Items = append(d.Items, s.codexDesktopPrefItems(name)...)
		}
	}
	switch {
	case len(reports) == 0:
		d.Summary = "还没有 Agent 接入"
		d.StatusCode = 2
	case bad > 0:
		d.Summary = fmt.Sprintf("已接入 %d 家 Agent,其中 %d 家的形态申报不合格(详见运行时状态页)", len(reports), bad)
		d.StatusCode = 1
	default:
		d.Summary = fmt.Sprintf("已接入 %d 家 Agent,形态申报全部合格", len(reports))
		d.StatusCode = 0
	}
	return d
}

// sessionModelText 会话形态的人话。认不出的词照抄原文 —— 形态词表会扩,
// 猜一个"看起来合理"的翻译比不翻更糟(与 event_translate 的不编造红线同源)。
func sessionModelText(m runtime.SessionSource) string {
	switch m {
	case runtime.SourceNative:
		return "原生自管"
	case runtime.SourceDaemon:
		return "daemon 代管"
	case runtime.SourceHybrid:
		return "混合(原生为主)"
	case runtime.SourceNone:
		return "未接入会话"
	default:
		return string(m)
	}
}

// agentsPrefsBudget 是 agents 域为 codex 桌面偏好拉候选(模型列表 + 托管策略)
// 的整段上界。总览页是聚合读:预算内拿不到候选就跳过对应 select 项(客户端对
// 无候选的项按只读呈现),绝不能让一个设置目录把整页八个域拖住。
const agentsPrefsBudget = 3 * time.Second

// codexPrefsSource 是 agents 域透出 codex 桌面偏好所需的**动态候选**来源
// (生产由 *agent.CodexProvider 满足;抽成接口,回归测试塞假件确定性钉行为,
// 与 codexProbe 同款取舍)。
type codexPrefsSource interface {
	// ModelsStatusCtx 模型列表(model/list;候选连同各模型的思考档位),错误如实返回。
	ModelsStatusCtx(ctx context.Context) ([]pb.ModelInfo, error)
	// RequirementsCtx 托管策略白名单(configRequirements/read),错误如实返回。
	RequirementsCtx(ctx context.Context) (agent.CodexRequirements, error)
}

// codexDesktopPrefItems 组装 codex 的桌面偏好 select 项(id 形如 agents.codex.<键>)。
//
// 【候选来源全部动态,宁缺毋假】每键的 options 都取自 codex app-server 的既有
// 查询,daemon 不写死任何名单:
//   - approval_policy / sandbox ← configRequirements/read 的托管白名单;
//     未托管或读不到 = 白名单为空 → 该键整项不透出(拿不到真候选就绝不用
//     猜的名单顶上,否则"选了被桌面拒绝"的账算谁的);
//   - model ← model/list 全部条目(含 codex/default = 跟随桌面),带 codex/
//     前缀,与 ListModels、store.DefaultModelID 同一口径;
//   - effort ← model/list 的 supportedReasoningEfforts(当前默认模型申报了
//     档位就用它的,否则取全部模型的并集);无人申报 → 不透出;
//   - model_verbosity / model_reasoning_summary ← codex app-server 没有任何
//     枚举查询(config/read 只回当前值,model/list 只有档位没有它们),候选
//     无法动态取得 → 这两键**恒不透出**,等协议给出枚举源再点亮。
//
// 【current 与 GetCodexSettings 同源】同一组 store 读数(CodexPrefs /
// DefaultModelID / DefaultEffort,即 codexSettingsSnapshot 的取数);写通道
// SetCodexSettings 落盘的正是这几处 —— 写后回读即见新值,两边界面永远一致。
// 电脑端此刻的实况由 GetCodexSettings 的 desktop echo 字段另行携带,不混进 current。
func (s *Server) codexDesktopPrefItems(name string) []*pb.ControlItem {
	src, ok := s.cfg.Providers["codex"].(codexPrefsSource)
	if !ok {
		return nil // codex 未注册或不是动态候选来源:一项不摆,不做假入口
	}
	// 不吃 RPC ctx:GetControlPlane 的域组装函数是纯 () *ControlDomain 形状,
	// 这里与 agent 层 withStatusClientBudget 同款 —— 用 Background 起自己的
	// 短上界,手机取消请求也最多多跑这 3 秒,不影响正确性。
	ctx, cancel := context.WithTimeout(context.Background(), agentsPrefsBudget)
	defer cancel()
	prefs := s.cfg.Store.CodexPrefs()
	currentModel := s.cfg.Store.DefaultModelID()
	currentEffort := s.cfg.Store.DefaultEffort()
	// 两个候选源共用同一个预算:模型列表若把预算耗尽,托管策略读取会立刻随
	// 同一个已过期的 ctx 返回,不会各自再等一遍。
	models, modelsErr := src.ModelsStatusCtx(ctx)
	reqs, reqsErr := src.RequirementsCtx(ctx)
	mk := func(key, label, current, hint string, options []string) *pb.ControlItem {
		return &pb.ControlItem{
			Id: "agents.codex." + key, Label: name + " · " + label,
			Current: current, Kind: "select", Options: options, Hint: hint,
		}
	}
	// 键序钉死成词表序(approval/sandbox/model/effort),缺候选的键整项跳过;
	// 次序稳定,客户端总览页前后两次刷新排版不抖。
	var items []*pb.ControlItem
	if reqsErr == nil && len(reqs.AllowedApprovalPolicies) > 0 {
		items = append(items, mk("approval_policy", "审批策略", prefs.ApprovalPolicy,
			"codex 执行命令前要不要先征得同意;电脑端被托管策略限制时,这里只列允许的档",
			reqs.AllowedApprovalPolicies))
	}
	if reqsErr == nil && len(reqs.AllowedSandboxModes) > 0 {
		items = append(items, mk("sandbox", "沙箱模式", prefs.Sandbox,
			"限定 codex 能动哪里:只读 / 限工作区 / 全盘;越宽越省事也越危险",
			reqs.AllowedSandboxModes))
	}
	if modelsErr == nil && len(models) > 0 {
		if options := codexModelOptions(models); len(options) > 0 {
			items = append(items, mk("model", "默认模型", currentModel,
				"新建对话默认用的模型;codex/default 表示跟随电脑端 codex 自己的配置", options))
		}
	}
	if options := codexEffortOptions(models, modelsErr, currentModel); len(options) > 0 {
		items = append(items, mk("effort", "思考强度", currentEffort,
			"推理的深浅档;档位更高想得更周到,也更耗时耗额度", options))
	}
	return items
}

// codexModelOptions 模型候选:列表 id 补上 codex/ 前缀(ListModels 与
// store.DefaultModelID 的全局唯一口径),去重保序 —— codex 给什么就摆什么,
// 顺序也照它,前端排版不重排。
func codexModelOptions(models []pb.ModelInfo) []string {
	seen := make(map[string]bool, len(models))
	out := make([]string, 0, len(models))
	// 【vet】按下标取址遍历,避免 range 复制 pb 消息(内嵌 MessageState 含锁)。
	for i := range models {
		id := "codex/" + strings.TrimPrefix(models[i].GetId(), "codex/")
		if id == "codex/" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// codexEffortOptions 思考强度候选:当前默认模型申报了档位就用它的(档位随模型
// 走,摆别的模型的档位就是骗人);否则取全部模型档位的并集兜底(默认模型常是
// codex/default=跟随桌面,此刻还不知道具体是哪家)。模型列表没拿到或无人申报
// → 返回 nil,effort 整项跳过。
func codexEffortOptions(models []pb.ModelInfo, modelsErr error, currentModel string) []string {
	if modelsErr != nil || len(models) == 0 {
		return nil
	}
	cur := strings.TrimPrefix(currentModel, "codex/")
	// 【vet】按下标取址遍历,避免 range 复制 pb 消息(内嵌 MessageState 含锁)。
	for i := range models {
		if models[i].GetId() == cur && len(models[i].GetAvailableEfforts()) > 0 {
			return append([]string(nil), models[i].GetAvailableEfforts()...)
		}
	}
	seen := make(map[string]bool)
	var out []string
	for i := range models {
		for _, e := range models[i].GetAvailableEfforts() {
			if !seen[e] {
				seen[e] = true
				out = append(out, e)
			}
		}
	}
	return out
}

// ---- memory:会话记忆 ---------------------------------------------------------

// controlDomainMemory 记忆合计(全部 + 按 agent)。数据源与 GetAgentRuntime 同一份
// Observe;写通道是 ClearThreadMemory(按会话清,不伤及无辜)。
func (s *Server) controlDomainMemory() *pb.ControlDomain {
	if s.runtimeBackend == nil {
		return degradedControlDomain(domainMemory, "会话记忆", "运行时后端未装配,记忆状态未报告")
	}
	names := s.agentDisplayNames()
	var threads, items int
	var bytes int64
	d := &pb.ControlDomain{Id: domainMemory, Title: "会话记忆", Writable: true, StatusCode: 0}
	for _, id := range s.sortedProviderIDs() {
		obs := s.runtimeBackend.Observe(id)
		threads += obs.Memory.Threads
		items += obs.Memory.Items
		bytes += obs.Memory.Bytes
		name := id
		if n := names[id]; n != "" {
			name = n
		}
		d.Items = append(d.Items, &pb.ControlItem{
			Id: "memory." + id, Label: name + " 名下记忆",
			Current: memoryText(obs.Memory.Threads, obs.Memory.Items, obs.Memory.Bytes), Kind: "info",
			Hint: "可在记忆管理页按会话清除,重新开始",
		})
	}
	d.Items = append(d.Items, &pb.ControlItem{
		Id: "memory.total", Label: "全部 Agent 合计",
		Current: memoryText(threads, items, bytes), Kind: "info",
		Hint: "记忆只存 PocketAgent 自己的数据目录;清除不影响原生侧会话",
	})
	d.Summary = "记忆合计:" + memoryText(threads, items, bytes)
	return d
}

// memoryText "N 个会话 / M 条 / X KB" 的人话合计。
func memoryText(threads, items int, bytes int64) string {
	return fmt.Sprintf("%d 个会话 / %d 条 / %s", threads, items, formatBytes(bytes))
}

// ---- dialysis:未知事件 -------------------------------------------------------

// controlDomainDialysis 未知事件台账的事实。写通道是 AcknowledgeUnknownEvent
// (五处置:已看/忽略/稍后/升级/需原生支持)。
func (s *Server) controlDomainDialysis() *pb.ControlDomain {
	lg := s.unknownLog()
	if lg == nil {
		return degradedControlDomain(domainDialysis, "未知事件", "诊断台账未启用(运行时后端未装配)")
	}
	// 未读计数与 ListUnknownEvents(status=NEW)同一份台账真值;台账本身只保留
	// 最近 200 条(超限丢最旧并计数),所以这个数天然有界。
	unread := len(lg.List(runtime.ListFilter{Status: runtime.StatusNew}, 0))
	total, dropped := lg.Stats()
	d := &pb.ControlDomain{Id: domainDialysis, Title: "未知事件", Writable: true}
	if unread > 0 {
		d.Summary = fmt.Sprintf("有 %d 条没见过的原生事件等你查看", unread)
		d.StatusCode = 1
	} else {
		d.Summary = fmt.Sprintf("没有待查看的未知事件(累计登记 %d 条)", total)
		d.StatusCode = 0
	}
	d.Items = []*pb.ControlItem{
		{
			Id: "dialysis.unread", Label: "待查看",
			Current: fmt.Sprintf("%d 条", unread), Kind: "info",
			Hint: "在未知事件页逐条查看原文并选择处置",
		},
		{
			Id: "dialysis.total", Label: "累计登记",
			Current: fmt.Sprintf("%d 条", total), Kind: "info",
			Hint: "台账只保留最近 200 条,更早的会被挤出",
		},
		{
			Id: "dialysis.dropped", Label: "超限丢弃",
			Current: fmt.Sprintf("%d 条", dropped), Kind: "info",
			Hint: "因超上限丢弃的条数;不为 0 说明最近流量不小",
		},
	}
	return d
}

// ---- approvals:审批请求 ------------------------------------------------------

// controlDomainApprovals 挂起的审批请求。决策写通道沿用既有审批应答
// (ApprovalDecision),这里只把"有几条在等你"摆出来。
func (s *Server) controlDomainApprovals() *pb.ControlDomain {
	n := s.pendingApprovalCount()
	d := &pb.ControlDomain{
		Id: domainApprovals, Title: "审批请求", Writable: true,
		Items: []*pb.ControlItem{{
			Id: "approvals.pending", Label: "等待处理的审批请求",
			Current: fmt.Sprintf("%d 条", n), Kind: "info",
			Hint: "Agent 要执行敏感操作时会先问你;在会话里即可处理",
		}},
	}
	if n > 0 {
		d.Summary = fmt.Sprintf("有 %d 条审批请求等待你的决定", n)
		d.StatusCode = 1
	} else {
		d.Summary = "没有等待处理的审批请求"
		d.StatusCode = 0
	}
	return d
}

// pendingApprovalCount 汇总全部会话挂起的审批请求数。与 GetThreadState 的
// PendingApprovals 同一份内存真值(snapshot),不另立口径。
func (s *Server) pendingApprovalCount() int {
	n := 0
	s.approvalSessions.Range(func(_, v any) bool {
		if a, ok := v.(*turnApprovals); ok {
			n += len(a.snapshot())
		}
		return true
	})
	return n
}

// ---- transports:连接通道 -----------------------------------------------------

// controlDomainTransports 手机连这台电脑的可用路径。纯只读事实(改不了,也不该在
// 手机上改)—— 地址探测与 GetHostInfo 同源,鉴权模式如实反映。
func (s *Server) controlDomainTransports() *pb.ControlDomain {
	lan := detectLanIP()
	ts := detectTailscaleIP()
	d := &pb.ControlDomain{
		Id: domainTransports, Title: "连接通道", Writable: false,
		Items: []*pb.ControlItem{
			{
				Id: "transports.lan", Label: "局域网直连",
				Current: transportText(lan), Kind: "info",
				Hint: "同一 WiFi 下手机直连这个地址最快",
			},
			{
				Id: "transports.tailscale", Label: "Tailscale 远程",
				Current: transportText(ts), Kind: "info",
				Hint: "装了 Tailscale 且在线时,出门在外也能连回家",
			},
			{
				Id: "transports.auth", Label: "鉴权模式",
				Current: authModeText(s.authRequired()), Kind: "info",
				Hint: "鉴权模式只能在电脑上重启 daemon 时更换",
			},
		},
	}
	switch {
	case lan != "" && ts != "":
		d.Summary = "局域网与 Tailscale 地址都在线,随时可连"
		d.StatusCode = 0
	case lan != "":
		d.Summary = fmt.Sprintf("局域网地址可用(%s);Tailscale 未检测到", lan)
		d.StatusCode = 0
	case ts != "":
		d.Summary = fmt.Sprintf("Tailscale 地址在线(%s);局域网地址未检测到", ts)
		d.StatusCode = 0
	default:
		// 两个地址都没有 = 手机此刻根本连不上 —— 这是"受限",不是"正常"。
		d.Summary = "未检测到任何可用网络地址,手机暂时连不上这台电脑"
		d.StatusCode = 2
	}
	return d
}

// transportText 地址在,给地址;不在,如实说"未检测到" —— 不编一个占位地址。
func transportText(addr string) string {
	if addr == "" {
		return "未检测到"
	}
	return addr
}

// authModeText 鉴权事实的人话(与 GetHostInfo.auth_required 同一判据)。
func authModeText(required bool) string {
	if required {
		return "配对码鉴权(已启用)"
	}
	return "无鉴权(仅限本机)"
}

// ---- resources:资源用量 ------------------------------------------------------

// controlDomainResources 资源账本合计。与 GetResourceUsage(scope="")同一份 Usage;
// 写通道是预算(SetRuntimePolicy),所以这里只读。
func (s *Server) controlDomainResources() *pb.ControlDomain {
	if s.runtimeBackend == nil {
		return degradedControlDomain(domainResources, "资源用量", "未记账(运行时后端未装配)")
	}
	rep := s.runtimeBackend.Resources().Usage(context.Background(), "")
	d := &pb.ControlDomain{Id: domainResources, Title: "资源用量", Writable: false}
	d.Items = []*pb.ControlItem{
		{
			Id: "resources.active_leases", Label: "活跃资源租约",
			Current: fmt.Sprintf("%d 份", rep.Leases), Kind: "info",
			Hint: "租约是轮次对内存/磁盘/并发的实时占用凭据",
		},
		{
			Id: "resources.over_soft", Label: "超过软上限",
			Current: fmt.Sprintf("%d 份", len(rep.OverSoft)), Kind: "info",
			Hint: "超过软上限 = 该压缩/落盘了",
		},
		{
			Id: "resources.over_hard", Label: "超过硬上限",
			Current: fmt.Sprintf("%d 份", len(rep.OverHard)), Kind: "info",
			Hint: "超硬上限的可选资源会被拒;必需资源照常放行并如实记账",
		},
	}
	if len(rep.OverSoft) > 0 || len(rep.OverHard) > 0 {
		d.Summary = fmt.Sprintf("活跃租约 %d 份,其中 %d 份进入压力区(该压缩/落盘了)",
			rep.Leases, len(rep.OverSoft)+len(rep.OverHard))
		d.StatusCode = 1
	} else {
		d.Summary = fmt.Sprintf("活跃租约 %d 份,均未触上限", rep.Leases)
		d.StatusCode = 0
	}
	return d
}

// ---- 共用小工具 ---------------------------------------------------------------

// formatBytes 字节额度的人话:整 MB/KB 带单位,其余如实给字节 —— 不把 123456
// 舍入成"0.1 GB"让用户核不了账。
func formatBytes(v int64) string {
	switch {
	case v >= 1<<20 && v%(1<<20) == 0:
		return fmt.Sprintf("%d MB", v>>20)
	case v >= 1<<10 && v%(1<<10) == 0:
		return fmt.Sprintf("%d KB", v>>10)
	default:
		return fmt.Sprintf("%d B", v)
	}
}

// derefInt 取 *int 值;nil → 0(预算语义里 0 是"未设上限/不限",不是"封禁",
// 呈现层交给各类 hint 去解释)。
func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// sortedProviderIDs 注册表 id 排序副本:map 遍历无序,不排序总览页每次刷新
// 条目次序都会换位(与 GetHostInfo caps 排序同款理由)。
func (s *Server) sortedProviderIDs() []string {
	ids := make([]string, 0, len(s.cfg.Providers))
	for id := range s.cfg.Providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// agentDisplayNames agent id → 展示名(取自启动时的形态申报;没申报的回空,
// 调用方按注册 id 兜底 —— 名字可以缺,不能编)。
func (s *Server) agentDisplayNames() map[string]string {
	out := make(map[string]string, len(s.contractReports))
	for _, rep := range s.contractReports {
		out[rep.AgentID] = rep.Descriptor.DisplayName
	}
	return out
}

// dataDirLabel 数据目录落在哪个标准位置,只给标签不给路径 —— 绝对路径里的
// 用户名属于敏感信息,诊断事实用不着它(要看落盘位置,各策略响应里的
// policy_path 已有完整路径)。
func dataDirLabel(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		return "内存模式"
	}
	if underDir(dataDir, hostenv.UserDataDir()) {
		return "LOCALAPPDATA"
	}
	if underDir(dataDir, hostenv.LegacyDataDir()) {
		return "家目录/.pocketagentd"
	}
	return "自定义位置"
}

// underDir 判断 dir 是否位于 base 之下(Windows 路径大小写不敏感)。
func underDir(dir, base string) bool {
	base = strings.TrimSpace(base)
	if base == "" {
		return false
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absDir)
	if err != nil {
		return false
	}
	// rel 以 ".." 开头 = dir 在 base 之外;等于 "." = dir 就是 base 本身(也算)。
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}
