// model_selection.go —— 会话有效模型选择解析 + 轮次选择快照
// (2026-10-04「会话真值、模型选择与历史体验」计划 Task 2;协议载体见
// model.proto 的 EffectiveModelSelection,Task 1 已入库)。
//
// 【这个文件回答什么问题】手机端原来有 selectedModel/defaultModel/threadModels/
// runtimeModel 好几个可变全局状态互相覆盖,谁也说不清最后生效的是哪个、为什么。
// daemon 在这里按四级优先级把来源收敛成一个结论(EffectiveModelSelection),
// 并在轮次边界把"本轮实际跑的配置"冻结成 TurnSnapshot —— 运行中的轮次绝不被
// 后续选择变化改写(全局约束红线)。
//
// 【source 判定决策树(优先级从上到下,首个命中即结论;source 用 spec 四值)】
//
//  1. thread_override  thread.ModelID 非空、不是 "default" 哨兵,且(严格模式)
//     通过 ValidateSelection —— 手机端/委派对会话的显式覆盖。
//     校验失败 = 报错,绝不静默改成默认(计划规则原文)。
//  2. desktop_default  thread.ModelID 恰为 "<agent>/default" 哨兵 —— 该哨兵在
//     模型列表里的语义就是"跟随桌面端当前配置"(见 model.go
//     ListModels 注释),它本身就是桌面默认这一级的表达。
//  3. app_default / desktop_default(复用 defaultModelIDFor 先例,thread.go)
//     thread.ModelID 为空(用户选了"跟随默认"/老库记录):
//     AgentDefaults → default 哨兵 → 模型列表首项。解析出
//     具体模型 = app_default;解析出哨兵 = desktop_default。
//  4. native_current   以上全落空(该 agent 无可用模型)且 provider 实现
//     agent.CurrentSelectionReader —— 会话实挂的原生当前值。
//     仍拿不到 = 显式报错,绝不编造 ID(defaultModelIDFor 同纪律)。
//
// 【不写死任何名单】模型归属、effort 档位全部来自 provider.Models() 的动态数据;
// agent 是否存在只查 s.cfg.Providers 注册表 —— 与 defaultModelIDFor /
// availableAgentIDs 同一红线,新增 provider 自动进入判定,无需改这里。
package server

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"google.golang.org/protobuf/proto"
)

// ---- source 四值(spec 钉死的稳定字符串,新增来源不改 proto)----

const (
	SelectionSourceThreadOverride = "thread_override" // 本会话显式覆盖
	SelectionSourceAppDefault     = "app_default"     // App 侧默认(daemon 落库默认)
	SelectionSourceDesktopDefault = "desktop_default" // 桌面端默认(含 default 哨兵)
	SelectionSourceNativeCurrent  = "native_current"  // agent 原生会话当前值
)

// selectionNativeBudget native_current 一级的原生读取预算(L4 兜底才发生 IO,
// 任何选择解析都不该拖住轮次启动/模型列表)。
const selectionNativeBudget = 3 * time.Second

// SelectionResolver 把一个会话(或 agent 视角)的选择解析成"实际生效"的结论,
// 并对"用户想写入的选择"做归属校验。
//
// 【实现形态】挂在 *Server 的方法(照 server 包注入风格:能力来自
// s.cfg.Providers / s.cfg.Store,无需独立构造参数);接口留作测试替身与
// 未来非文件实现(如 SQLite 元数据)的替换缝。
type SelectionResolver interface {
	// ResolveThreadSelection 解析会话当前实际生效的选择。空 model/effort 是
	// "跟随默认"的合法结论(零值语义),error 只表示"四级来源全部落空"。
	ResolveThreadSelection(ctx context.Context, t *store.Thread) (pb.EffectiveModelSelection, error)
	// ValidateSelection 校验一份待写入的选择:模型必须属于该 agent 的动态
	// 模型列表;effort 必须在该模型声明的档位集合内(列表为空 = 该模型无动态
	// 档位,不设限)。空 model/effort = 跟随默认,合法,直接放行。
	ValidateSelection(agentID, modelID, effort string) error
}

// 编译期钉:*Server 就是本包的 SelectionResolver 实现。
var _ SelectionResolver = (*Server)(nil)

// ---- TurnSnapshot:轮次启动那一刻的选择冻结 ----

// TurnSnapshot 一轮启动后不再变化的配置快照(spec 接口,immutable 值快照:
// 捕获后任何代码都不得改写它的字段 —— 改"下一轮的选择"请产生新快照)。
//
// 【冻结时序】StartTurn 在 applyTurnOverrides 之后、任何帧发出之前捕获;
// 排队续跑(P1-4)在每次出队应用覆盖后重捕。TurnStarted 帧与 selection_changed
// 判定一律读快照,绝不回读线程实时字段。
type TurnSnapshot struct {
	ThreadID         string
	AgentID          string
	ModelID          string // 本轮实际生效的模型 id(空 = 跟随默认,合法)
	Effort           string // 本轮实际生效的思考强度(空 = 跟随默认,合法)
	Workspace        string
	Source           string // SelectionSource* 四值之一;"" = 四级全落空(如实未知)
	CapturedAtUnixMs int64
}

// ---- 基础工具(与既有实现同一口径,不另起炉灶)----

// defaultSentinelID 各 provider 模型列表里"跟随桌面当前配置"的哨兵全局 id
// (appendProviderModels 的全局唯一化规则:p.ID()+"/"+原始 id)。
func defaultSentinelID(agentID string) string { return agentID + "/default" }

// isDefaultSentinel 判断 modelID 是否为该 agent 的 default 哨兵。
func isDefaultSentinel(agentID, modelID string) bool {
	return modelID != "" && modelID == defaultSentinelID(agentID)
}

// providerGlobalModelID 把 provider 的原始模型 id 归一成全局唯一 id
// (与 appendProviderModels 逐字节同一规则:双前缀防重;缓存切片只读)。
func providerGlobalModelID(p agent.Provider, raw string) string {
	return p.ID() + "/" + strings.TrimPrefix(raw, p.ID()+"/")
}

// agentModelInfos 按 appendProviderModels 同一归一规则取某 agent 的全局模型表
// (provider 返回的可能是 TTL 缓存切片,这里只读不写)。
func (s *Server) agentModelInfos(agentID string) []pb.ModelInfo {
	p, ok := s.cfg.Providers[agentID]
	if !ok {
		return nil
	}
	raw := p.Models()
	out := make([]pb.ModelInfo, 0, len(raw))
	// 【vet】按下标取址遍历,避免 range 复制 pb 消息(内嵌 MessageState 含锁)。
	for i := range raw {
		m := &raw[i]
		out = append(out, pb.ModelInfo{
			Id:               providerGlobalModelID(p, m.Id),
			Provider:         m.Provider,
			DisplayName:      m.DisplayName,
			ContextWindow:    m.ContextWindow,
			SupportsThinking: m.SupportsThinking,
			SupportsImages:   m.SupportsImages,
			Available:        m.Available,
			AvailableEfforts: append([]string(nil), m.AvailableEfforts...),
		})
	}
	return out
}

// findModelInfo 在 agent 的全局模型表里找一份模型(找不到返回 nil)。
func (s *Server) findModelInfo(agentID, modelID string) *pb.ModelInfo {
	ms := s.agentModelInfos(agentID)
	// 【vet】按下标取址遍历,避免 range 复制 pb 消息。
	for i := range ms {
		if ms[i].Id == modelID {
			return &ms[i]
		}
	}
	return nil
}

// effortsFor 某模型的动态思考档位(与 ModelInfo.available_efforts /
// EffectiveModelSelection.available_efforts 同源口径);模型不在列表 = 空。
func (s *Server) effortsFor(agentID, modelID string) []string {
	if m := s.findModelInfo(agentID, modelID); m != nil {
		return append([]string(nil), m.AvailableEfforts...)
	}
	return nil
}

// agentDefaultEffort 该 agent 的默认思考强度(codex 维持全局默认语义,
// 与 defaultModelIDFor 对 AgentDefaults 的不对称处理逐行对齐)。
func (s *Server) agentDefaultEffort(agentID string) string {
	if agentID != "codex" {
		return s.cfg.Store.AgentDefaults(agentID).Effort
	}
	return s.cfg.Store.DefaultEffort()
}

// ---- ValidateSelection:待写入选择的归属校验 ----

// ValidateSelection 计划规则:"创建/更新线程时校验 model 是否属于 agent;
// 未知模型不能静默改成默认"、"effort 不在当前模型能力集合时返回清楚的人话,
// 并建议可用值"。返回人话 error(RPC 层决定映射成哪个 gRPC code),nil = 放行。
func (s *Server) ValidateSelection(agentID, modelID, effort string) error {
	if agentID == "" {
		agentID = "codex" // threadAgentID 同款兜底:空 = codex(老库语义)
	}
	_, ok := s.cfg.Providers[agentID] // provider 值本身不用:模型归属校验走 findModelInfo/agentModelInfos 动态数据
	if !ok {
		// 动态列举当前可用 agent(availableAgentIDs 红线:不写死名单)
		return fmt.Errorf("电脑上没有可用的 %s agent(当前可用: %s),无法应用该模型选择",
			agentID, strings.Join(s.availableAgentIDs(), ", "))
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil // 空 = "跟随默认",合法取值,无对象可校验(零值语义)
	}
	m := s.findModelInfo(agentID, modelID)
	if m == nil {
		ms := s.agentModelInfos(agentID)
		if len(ms) == 0 {
			return fmt.Errorf("模型 %s 不在 %s 的可用模型列表中(列表当前为空,该 agent 可能还在初始化,请稍后重试);daemon 不会把它静默替换成默认模型",
				modelID, agentID)
		}
		ids := make([]string, 0, len(ms))
		for i := range ms {
			ids = append(ids, ms[i].Id)
		}
		const preview = 8
		if len(ids) > preview {
			return fmt.Errorf("模型 %s 不在 %s 的可用模型列表中,daemon 不会静默替换成默认模型;可用模型(%d 个): %s 等",
				modelID, agentID, len(ids), strings.Join(ids[:preview], ", "))
		}
		return fmt.Errorf("模型 %s 不在 %s 的可用模型列表中,daemon 不会静默替换成默认模型;可用模型: %s",
			modelID, agentID, strings.Join(ids, ", "))
	}
	effort = strings.TrimSpace(effort)
	if effort == "" {
		return nil // 空 = 跟随默认强度,合法
	}
	// 档位集合只来自 provider 动态数据:列表为空 = 该模型未声明动态档位
	// (codex/zcode 无动态档位,手机端用内置默认档)→ 不设限,绝不拿内置名单拦人。
	if len(m.AvailableEfforts) == 0 {
		return nil
	}
	for _, v := range m.AvailableEfforts {
		if v == effort {
			return nil
		}
	}
	return fmt.Errorf("思考强度 %q 不被模型 %s 支持;该模型可用档位: %s",
		effort, modelID, strings.Join(m.AvailableEfforts, ", "))
}

// ---- ResolveThreadSelection:四级来源收敛 ----

// resolvedSelection 分类器的中间结论(纯内部结构,公开口径见
// EffectiveModelSelection 各字段注释)。
type resolvedSelection struct {
	source      string
	modelID     string
	effort      string
	confirmed   bool // 用户是否显式确认过(仅会话显式覆盖为 true)
	available   []string
	explanation string
}

// classifySelection 四级来源判定(决策树见文件头)。strict=true 用于"读真相"
// 入口(ListModels/将来 RPC):会话覆盖校验失败 → 返回 error,绝不静默降级;
// strict=false 用于轮次快照:来源判定尽力而为,失败只让 source 留空 —— 轮次
// 该怎么跑还怎么跑,provider 自会用真实错误说话。
func (s *Server) classifySelection(ctx context.Context, t *store.Thread, strict bool) (resolvedSelection, error) {
	agentID := threadAgentID(t)
	running := t != nil && t.Status == pb.ThreadStatus_THREAD_STATUS_RUNNING

	// ---- 1) 会话显式覆盖(非哨兵、非空)----
	if t != nil && t.ModelID != "" && !isDefaultSentinel(agentID, t.ModelID) {
		if strict {
			if err := s.ValidateSelection(agentID, t.ModelID, t.Effort); err != nil {
				return resolvedSelection{}, err // 绝不静默改成默认
			}
		}
		out := resolvedSelection{
			source:    SelectionSourceThreadOverride,
			modelID:   t.ModelID,
			effort:    t.Effort,
			confirmed: true,
			available: s.effortsFor(agentID, t.ModelID),
		}
		out.explanation = "本会话显式选择了 " + t.ModelID
		if t.Effort != "" {
			out.explanation += "(思考强度 " + t.Effort + ")"
		}
		if running {
			out.explanation += ";已有轮次正在运行,此选择将从下一轮生效,运行中的轮次不受影响"
		} else {
			out.explanation += ",将在下一轮启动时按此装配"
		}
		return out, nil
	}

	// ---- 2) default 哨兵 = 显式"跟随桌面端当前配置" ----
	if t != nil && isDefaultSentinel(agentID, t.ModelID) {
		if strict && t.Effort != "" {
			if err := s.ValidateSelection(agentID, t.ModelID, t.Effort); err != nil {
				return resolvedSelection{}, err
			}
		}
		out := resolvedSelection{
			source:    SelectionSourceDesktopDefault,
			modelID:   t.ModelID,
			effort:    t.Effort,
			available: s.effortsFor(agentID, t.ModelID),
		}
		out.explanation = "本会话跟随桌面端当前配置(daemon 不钉死具体模型)"
		if t.Effort != "" {
			out.explanation += ";思考强度为本会话指定(" + t.Effort + ")"
		}
		return out, nil
	}

	// ---- 3) App 默认 / 桌面默认(defaultModelIDFor 先例:AgentDefaults →
	//         default 哨兵 → 列表首项;解析不出不编造)----
	if def, err := s.defaultModelIDFor(agentID); err == nil {
		effort := s.agentDefaultEffort(agentID)
		threadEffort := ""
		if t != nil {
			threadEffort = t.Effort
		}
		if threadEffort != "" {
			effort = threadEffort // 会话显式指定过强度:强度跟着会话走
		}
		// 严格模式只在"默认模型确实在动态列表里"时校验档位:AgentDefaults 落盘的
		// 默认不经过列表检查(defaultModelIDFor 先例),列表此刻可能为空/过期 ——
		// 没有能力数据就不设否决权(与 ValidateSelection 的档位原则同一红线),
		// 值不真,provider 会用真实错误说话。
		if strict && effort != "" && s.findModelInfo(agentID, def) != nil {
			if err := s.ValidateSelection(agentID, def, effort); err != nil {
				return resolvedSelection{}, err
			}
		}
		out := resolvedSelection{
			modelID:   def,
			effort:    effort,
			available: s.effortsFor(agentID, def),
		}
		if isDefaultSentinel(agentID, def) {
			out.source = SelectionSourceDesktopDefault
			out.explanation = "本会话未单独选择模型,跟随桌面端默认(default 哨兵)"
		} else {
			out.source = SelectionSourceAppDefault
			out.explanation = "本会话未单独选择模型,按 " + agentID + " 的 App 默认 " + def + " 生效"
		}
		if threadEffort != "" {
			out.explanation += ";思考强度为本会话指定(" + threadEffort + ")"
		}
		return out, nil
	}

	// ---- 4) 原生当前值(唯一会发生原生 IO 的一级,预算有界)----
	if t != nil && t.ID != "" {
		if p, ok := s.cfg.Providers[agentID]; ok {
			if native, isNative := p.(agent.CurrentSelectionReader); isNative {
				nctx, cancel := context.WithTimeout(ctx, selectionNativeBudget)
				model, effort, err := native.CurrentSelection(nctx, t)
				cancel()
				if err == nil && model != "" {
					out := resolvedSelection{
						source:    SelectionSourceNativeCurrent,
						modelID:   model,
						available: s.effortsFor(agentID, model),
					}
					if effort != nil {
						out.effort = *effort
					}
					out.explanation = "会话未显式选择且 App/桌面默认不可用,取 agent 原生会话当前挂载的模型"
					return out, nil
				}
			}
		}
	}

	return resolvedSelection{}, fmt.Errorf("无法确定会话当前生效的模型选择: %s 的模型列表不可用,原生当前值也读不到", agentID)
}

// ResolveThreadSelection 实现 SelectionResolver(接口语义见定义处)。
func (s *Server) ResolveThreadSelection(ctx context.Context, t *store.Thread) (pb.EffectiveModelSelection, error) {
	r, err := s.classifySelection(ctx, t, true)
	if err != nil {
		// 部分填充的结论一并带回(agent_id 可用),调用方按需记录;语义以 err 为准。
		return pb.EffectiveModelSelection{AgentId: threadAgentID(t)}, err
	}
	return pb.EffectiveModelSelection{
		AgentId:                threadAgentID(t),
		ModelId:                r.modelID,
		Effort:                 r.effort,
		Source:                 r.source,
		AppliesToNextTurn:      t != nil && t.Status == pb.ThreadStatus_THREAD_STATUS_RUNNING,
		ConfigurationConfirmed: r.confirmed,
		Explanation:            r.explanation,
		AvailableEfforts:       r.available,
	}, nil
}

// ---- TurnSnapshot 捕获 + selection_changed 变化判定 ----

// captureTurnSnapshot 在轮次上下文取值完成后立即冻结本轮配置。快照里的
// ModelID/Effort 是线程此刻的**实际取值**(provider 将按 TurnRequest.Thread
// 原样消费的就是它们),Source 是"为什么是它"的尽力判定(strict=false:
// 校验失败不拦轮次,轮次失败由 provider 的真实错误呈现)。本函数不返回 error:
// 轮次启动不因"来源说不清"而失败,只有 provider 选不到才会显式失败。
func (s *Server) captureTurnSnapshot(t *store.Thread) *TurnSnapshot {
	snap := &TurnSnapshot{
		ThreadID:         t.ID,
		AgentID:          threadAgentID(t),
		ModelID:          t.ModelID,
		Effort:           t.Effort,
		Workspace:        t.Workspace,
		CapturedAtUnixMs: time.Now().UnixMilli(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), selectionNativeBudget)
	defer cancel()
	if r, err := s.classifySelection(ctx, t, false); err == nil {
		snap.Source = r.source
	}
	return snap
}

// selectionEqual 快照的选择面是否等价(比较维度只看选择本身,不看捕获时刻)。
func selectionEqual(a, b *TurnSnapshot) bool {
	return a.ModelID == b.ModelID && a.Effort == b.Effort && a.Source == b.Source
}

// lastTurnSelections 每线程"上一次冻结的轮次选择"(进程内缓存,daemon 重启即空:
// 重启后首轮按"首次观察"处理,不发变化事件 —— UI 由 TurnStarted 直接拿到真值)。
// 【为什么是包级而不是 Server 字段】Task 2 的可改文件圈定不含 server.go,新增
// Server 字段会外溢;进程内缓存对"变化广播"这一通知语义已足够(事实真相在
// store/provider,不在缓存)。
var lastTurnSelections sync.Map // threadID -> *TurnSnapshot

// selectionChangedHook selection_changed 的接线钩子(既有事件通路结论:
// HostEvent 的 oneof 目前只有 turn_finished,TurnServerMessage 也没有对应帧,
// proto 层暂无载体 —— steward 需在 host.proto 增配分支后,把真实广播实现挂进
// 本钩子;届时调用点一处不改)。生产为 nil = 只落日志留痕,不影响任何行为。
var selectionChangedHook func(threadID string, prev, cur *pb.EffectiveModelSelection)

// setSelectionChangedHook 供测试/未来接线临时替换钩子;返回恢复函数。
func setSelectionChangedHook(h func(threadID string, prev, cur *pb.EffectiveModelSelection)) func() {
	prev := selectionChangedHook
	selectionChangedHook = h
	return func() { selectionChangedHook = prev }
}

// resetSelectionTracking 清空进程内"上一次选择"缓存(测试隔离用)。
func resetSelectionTracking() {
	lastTurnSelections.Range(func(key, _ any) bool {
		lastTurnSelections.Delete(key)
		return true
	})
}

// noteSelectionChanged 轮次边界调用:与该线程上一次冻结的选择比较,变化则经
// 钩子广播。首次观察不算变化(没有"上一次");相等不广播 —— "每次选择变化广播
// selection_changed,但不改已开始的 Turn"(计划规则,广播是纯通知)。
func noteSelectionChanged(threadID string, snap *TurnSnapshot) {
	if snap == nil || threadID == "" {
		return
	}
	prevAny, loaded := lastTurnSelections.Load(threadID)
	lastTurnSelections.Store(threadID, snap)
	if !loaded {
		return // 首次观察:UI 从 TurnStarted 帧直接拿真值
	}
	prev, _ := prevAny.(*TurnSnapshot)
	if prev == nil || selectionEqual(prev, snap) {
		return
	}
	log.Printf("[selection] %s 生效选择变化: model=%q effort=%q source=%s → model=%q effort=%q source=%s",
		threadID, prev.ModelID, prev.Effort, prev.Source, snap.ModelID, snap.Effort, snap.Source)
	if selectionChangedHook != nil {
		selectionChangedHook(threadID,
			&pb.EffectiveModelSelection{AgentId: prev.AgentID, ModelId: prev.ModelID, Effort: prev.Effort, Source: prev.Source},
			&pb.EffectiveModelSelection{AgentId: snap.AgentID, ModelId: snap.ModelID, Effort: snap.Effort, Source: snap.Source})
	}
}

// startedFrame 用冻结的轮次快照填 TurnStarted(additive:proto 现有 1~4 号字段)。
// 【为什么读快照不读线程】TurnStarted 的承诺是"这一轮实际在用什么" —— 线程字段
// 在排队续跑边界会被合法改写,只有快照始终如实反映当前轮。effort 空 = 跟随默认,
// 保持缺席(与 threadToPB 的 0→缺席同款纪律,不把空串冒充成已上报值)。
// 【source 暂缺】thread.proto 的 TurnStarted 目前没有 source 字段(1~4 已被
// turn_id/model_id/effort/configuration_confirmed 占用),已列入 steward 待补
// 清单;补号后在此追加一行即可,快照里已备好真值。
func startedFrame(turnID string, snap *TurnSnapshot) *pb.TurnStarted {
	out := &pb.TurnStarted{
		TurnId:  turnID,
		ModelId: snap.ModelID,
	}
	if snap.Effort != "" {
		out.Effort = proto.String(snap.Effort)
	}
	out.ConfigurationConfirmed = snap.Source == SelectionSourceThreadOverride
	return out
}

// ---- ListModels 集成(model.go 的解析所需最小追加的实现体)----

// listModelsEffectiveSelection 给 ListModelsResponse.effective_selection(字段 5)
// 产出结论:带 thread_id 按会话解析;否则按 agent_id 的"无覆盖视角"(等价于
// 一个刚建好、未做任何会话级选择的线程)。错误如实上抛,由调用方并入 StateError
// —— 与 ListModels 既有纪律一致:不伪造结论,也不静默缺席。
func (s *Server) listModelsEffectiveSelection(ctx context.Context, req *pb.ListModelsRequest) (*pb.EffectiveModelSelection, error) {
	if tid := req.GetThreadId(); tid != "" {
		t, ok := s.cfg.Store.Get(tid)
		if !ok {
			return nil, fmt.Errorf("会话 %s 不存在,无法解析有效模型选择", tid)
		}
		sel, err := s.ResolveThreadSelection(ctx, t)
		if err != nil {
			return nil, err
		}
		return &sel, nil
	}
	sel, err := s.ResolveThreadSelection(ctx, &store.Thread{AgentID: req.GetAgentId()})
	if err != nil {
		return nil, err
	}
	return &sel, nil
}
