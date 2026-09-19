package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// thinkingLevelSource 动态思考档位来源的最小接口(生产由 *agent.PiProvider 实现;
// server 层类型断言探测,与 zcodeHistorySource 同款模式,单测可注 fake)。
type thinkingLevelSource interface {
	ThinkingLevels(t *store.Thread) []string
}

// dynamicEffortsFor 取一个 provider 的动态思考档位列表;provider 没有该能力时
// 返回 nil(proto 不填 = 手机端用内置默认档,与 codex/zcode 的语义一致)。
//
// 【为什么拿"最近会话"当探针】PiProvider.ThinkingLevels 按线程找活着的子进程
// 实时取档位(get_available_thinking_levels 随模型变化);而模型列表是全局
// RPC,没有线程上下文。这里复用最近更新的一个该 agent 会话作探针 —— 该会话
// 挂载着就拿到真实动态档位,没挂载就由 ThinkingLevels 回全集兜底(它自己
// 不起子进程,不会拖慢模型列表)。
func (s *Server) dynamicEffortsFor(p agent.Provider) []string {
	src, ok := p.(thinkingLevelSource)
	if !ok {
		return nil
	}
	probe := &store.Thread{AgentID: p.ID()}
	newest := int64(-1)
	for _, t := range s.cfg.Store.List(true) {
		if threadAgentID(t) != p.ID() {
			continue
		}
		if t.UpdatedAt > newest {
			newest = t.UpdatedAt
			probe = t
		}
	}
	return src.ThinkingLevels(probe)
}

func (s *Server) ListModels(ctx context.Context, req *pb.ListModelsRequest) (*pb.ListModelsResponse, error) {
	out := &pb.ListModelsResponse{}
	if req.GetThreadId() != "" {
		if thread, ok := s.cfg.Store.Get(req.GetThreadId()); ok {
			if native, ok := s.cfg.Providers[threadAgentID(thread)].(agent.CurrentSelectionReader); ok {
				stateCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				model, effort, err := native.CurrentSelection(stateCtx, thread)
				cancel()
				if err != nil {
					out.StateError = err.Error()
				} else {
					out.CurrentModelId = model
					out.CurrentEffort = effort
				}
			}
		}
	}

	// 稳定顺序:桌面 agent(codex/pi/zcode)在前,echo 排最后兜底。
	// 【pi 为什么入列】pi 会话的"模型·强度"卡需要列出 pi 的真实模型并渲染
	// 动态思考档位(available_efforts),不进全局列表手机端就永远选不到。
	// 【zcode 为什么入列(设置页 Pi/ZCode 分区,2026-09-19)】设置页的 ZCode 分区
	// 要显示"ZCode 当前有哪些模型",而手机端能拿到的模型数据只有这一个全局列表;
	// ZCode 分区只做**展示**,Codex 默认模型的候选已被手机端按 provider=="codex"
	// 收窄,不会把 zcode 的模型误写进 codex 设置。
	order := []string{"codex", "pi", "zcode"}
	known := map[string]bool{"codex": true, "pi": true, "zcode": true, "echo": true}
	var extra []string
	for id := range s.cfg.Providers {
		if !known[id] {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	order = append(order, extra...)
	order = append(order, "echo")
	for _, id := range order {
		if req.GetAgentId() != "" && req.GetAgentId() != id {
			continue
		}
		p, ok := s.cfg.Providers[id]
		if !ok {
			continue
		}
		// 动态思考档位:每个 provider 算一次,填给它的全部模型;
		// 无动态档位能力的 provider(codex/zcode/glm/…)为 nil,不填。
		if req.GetRefresh() {
			if refresh, ok := p.(agent.ModelCatalogRefresher); ok {
				refresh.InvalidateModels()
			}
		}
		efforts := []string(nil) // Concrete model metadata must not be borrowed from another live session.
		// 【vet】按下标取址遍历,避免 range 复制 pb 消息(内嵌 MessageState 含锁)。
		models := p.Models()
		for i := range models {
			m := &models[i]
			// 【必须复制后改】providers 的 Models() 会返回**缓存的共享切片**
			// (pi 的 modelsCache 就是缓存本体)。以前直接 m.Id 加前缀,写入的是
			// 缓存:TTL 内第二次列表会叠成 "pi/pi/…" 双前缀,并发请求还会形成
			// 数据竞争。这里逐字段复制到新消息再改,缓存保持只读。
			cm := &pb.ModelInfo{
				Id:               p.ID() + "/" + strings.TrimPrefix(m.Id, p.ID()+"/"), // 全局唯一:"pi/zai-coding-cn/glm-5.3"
				Provider:         p.ID(),
				DisplayName:      m.DisplayName,
				ContextWindow:    m.ContextWindow,
				SupportsThinking: m.SupportsThinking,
				SupportsImages:   m.SupportsImages,
				Available:        m.Available,
			}
			cm.AvailableEfforts = append([]string(nil), m.AvailableEfforts...)
			if m.Id == "default" && len(efforts) > 0 {
				cm.AvailableEfforts = append([]string(nil), efforts...)
			}
			out.Models = append(out.Models, cm)
		}
	}

	return out, nil
}

func (s *Server) SetDefaultModel(ctx context.Context, req *pb.SetDefaultModelRequest) (*pb.Empty, error) {
	if err := s.cfg.Store.SetDefaultModelID(req.ModelId); err != nil {
		return nil, errInternal(err)
	}
	return &pb.Empty{}, nil
}

func (s *Server) TestModel(ctx context.Context, req *pb.TestModelRequest) (*pb.TestModelResponse, error) {
	p := s.providerFor(req.ModelId)
	if p == nil {
		return &pb.TestModelResponse{Ok: false, Error: "找不到 provider"}, nil
	}
	prompt := req.Prompt
	if prompt == "" {
		prompt = "回复 pong 两个字母即可,不要多余内容。"
	}
	start := time.Now()
	blocks, _, err := p.Run(ctx, agent.TurnRequest{
		Thread:      &store.Thread{ID: randomID("model-test"), AgentID: p.ID(), ModelID: req.ModelId, Workspace: s.cfg.Workspace},
		Workspace:   s.cfg.Workspace,
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: prompt}}}},
	}, func(*pb.TurnServerMessage) {}, nil)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return &pb.TestModelResponse{Ok: false, Error: err.Error(), LatencyMs: elapsed}, nil
	}
	reply := ""
	// 【vet】按下标取址遍历,避免 range 复制 pb 消息(内嵌 MessageState 含锁)。
	for i := range blocks {
		b := &blocks[i]
		if t, ok := b.Block.(*pb.ContentBlock_Text); ok {
			reply += t.Text.Text
		}
	}
	return &pb.TestModelResponse{Ok: true, Reply: reply, LatencyMs: elapsed}, nil
}

// ---- Codex 桌面端设置 / MCP ----

func (s *Server) codexProvider() *agent.CodexProvider {
	if p, ok := s.cfg.Providers["codex"].(*agent.CodexProvider); ok {
		return p
	}
	return nil
}

// ---------- 桌面只读回显(四个只读口子透出手机:config/read + capabilities + sandbox)----------
//
// 【这段做什么】codexSettingsSnapshot 回答的是"daemon 落盘/下发了什么";
// 下面这段补上另一半:"电脑端 codex 此刻实际生效的是什么"。数据全部来自
// internal/agent/codex_read.go 的只读口子,daemon 绝不借它们写任何桌面配置。
//
// 【为什么跳过 probeOverride 生效的 Server】probeOverride 是既有的测试注入点
// (newProbeTestServer 一族);注入生效说明这是**不许拉起真实 codex 子进程**的
// 单测环境 —— 桌面回读会真的 spawn app-server,必须让路。归一/填充逻辑的
// 可测性由下方纯函数(desktopConfigSummaryFrom 等)独立保证,不依赖真实读。
func (s *Server) fillCodexDesktopEcho(ctx context.Context, out *pb.CodexSettings) {
	if s.probeOverride != nil {
		return
	}
	p := s.codexProvider()
	if p == nil {
		return // 电脑上没有 codex:setting_status 已逐项报 failed,这里不重复喊
	}
	// config/read:cwd 取默认工作区,项目层配置按它解析(与能力列表同款取法)。
	cwd := s.cfg.Store.DefaultWorkspace()
	if cwd == "" {
		cwd = s.cfg.Workspace
	}
	snap, err := p.ReadDesktopConfig(ctx, cwd, false)
	if err != nil {
		// 读不到 ≠ 没有值:如实说原因,各字段留空,手机端对空值不显示副文案。
		out.DesktopEchoError = "电脑端配置读取失败:" + errTextOrEmpty(err)
	} else {
		out.DesktopConfigSummary = desktopConfigSummaryFrom(snap)
	}
	// provider 能力:失败静默 —— 徽标缺席的含义是"这回没读到",不是"不支持"。
	if caps, capsErr := p.ReadProviderCapabilities(ctx); capsErr == nil {
		out.ProviderCaps = providerCapsFrom(caps)
	}
	// Windows 沙箱就绪度:三种状态或未知原文,原样透传(照镜子,不加工)。
	if ready, readyErr := p.ReadSandboxReadiness(ctx); readyErr == nil {
		out.SandboxReadiness = ready
	}
}

// desktopConfigSummaryFrom 把 config/read 归一快照翻成协议消息(纯函数,可单测)。
// Raw 不透传:desktop 大对象留在 daemon 侧,手机只拿七项对照值。
func desktopConfigSummaryFrom(snap *agent.CodexConfigSnapshot) *pb.CodexDesktopConfigSummary {
	if snap == nil {
		return nil
	}
	return &pb.CodexDesktopConfigSummary{
		Model:          desktopModelSlug(snap.Model),
		Provider:       snap.ModelProvider,
		Effort:         snap.Effort,
		Summary:        snap.Summary,
		Verbosity:      snap.Verbosity,
		SandboxMode:    snap.SandboxMode,
		ApprovalPolicy: snap.ApprovalPolicy,
	}
}

// desktopModelSlug 把桌面当前模型归一成与模型列表可对齐的 slug(纯函数,可单测)。
//
// 【为什么只动 dexacct. 前缀】config/read 实测可能回账号组限定的长 id,形状为
// "dexacct.<base64>.<endpoint>.<model>"(见 codex_read.go 注释),与手机模型列表
// 的裸 slug 对不上。前三段是基建段(base64 标准字母表不含点;endpoint 段是
// URL 转义,点也被转义),所以从第 4 段起原样拼接即为模型名 —— 版本点
// ("gpt-5.4-turbo")得以保留,一刀切取最后一个点段会把 "…gpt-5.4" 砍成 "4"。
// 少于 4 段说明不是预期的长 id 形状,原样返回(照镜子,不瞎猜);不含前缀的
// 普通 slug 同样原样返回。
func desktopModelSlug(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	if strings.HasPrefix(model, "dexacct.") {
		parts := strings.Split(model, ".")
		if len(parts) >= 4 {
			return strings.Join(parts[3:], ".")
		}
	}
	return model
}

// providerCapsFrom 把 provider 能力声明翻成协议消息(纯函数,可单测)。
func providerCapsFrom(caps *agent.CodexProviderCaps) *pb.CodexProviderCaps {
	if caps == nil {
		return nil
	}
	return &pb.CodexProviderCaps{
		ImageGeneration: caps.ImageGeneration,
		WebSearch:       caps.WebSearch,
		NamespaceTools:  caps.NamespaceTools,
	}
}

// GetCodexSettings / ListMcpServers 见 model_ext.go(带托管策略与 MCP 详情)。

// SetCodexSettings 写入 codex 偏好。
//
// 【第 2 轮重做 —— 用户反馈"所有设置项都无法实际生效"的正面修复】
// 以前这个函数有三个致命问题:① codex 不存在时静默返回成功;② 只有 model/effort
// 落盘,其余全是内存态(重启即丢);③ "零值=不改"把"改回跟随桌面/关掉开关"堵死。
// 现在按下面的顺序做,任一环节失败都**如实反映**在返回的 setting_status 里,
// 不再假装成功:
//
//  1. presence 合并:只有客户端显式 set 的字段才覆盖(absent 保持原值);
//  2. 落盘到 store(daemon 重启后仍在);
//  3. 应用到 provider 内存态(每轮 turn/start、thread/start 据此携带);
//  4. 探测能否连上 codex,把"已生效 / 未生效 / 不支持 / 连接失败"逐项回报。
func (s *Server) SetCodexSettings(ctx context.Context, req *pb.SetCodexSettingsRequest) (*pb.CodexSettings, error) {
	s.prefsMu.Lock()
	defer s.prefsMu.Unlock()
	if id := req.GetAgentId(); id != "" && id != "codex" {
		return s.setAgentSettings(ctx, req)
	}
	p := s.codexProvider()
	if p == nil && req.HideAgentReasoning == nil {
		// 【绝不静默丢弃】以前这里 p==nil 时直接跳过、RPC 仍返回成功,
		// App 侧显示"已保存" —— 这是最典型的"点了没反应"。
		return nil, errInternal(fmt.Errorf("电脑上没有可用的 codex,设置无法生效(请确认已安装 codex CLI)"))
	}

	// ---- 1) presence 合并 ----
	// 已持久化的值作为基线;proto3 optional 让"显式 set 空串/ false"与"没传"可区分。
	prefs, model, effort := mergeCodexPrefs(
		s.cfg.Store.CodexPrefs(),
		s.cfg.Store.DefaultModelID(),
		s.cfg.Store.DefaultEffort(),
		req,
	)

	// ---- 2) 落盘(持久化是"改了又变回去"的唯一根治手段)----
	if err := s.cfg.Store.SetCodexPrefs(prefs); err != nil {
		return nil, errInternal(err)
	}
	if err := s.cfg.Store.SetDefaultModelID(model); err != nil {
		return nil, errInternal(err)
	}
	if err := s.cfg.Store.SetDefaultEffort(effort); err != nil {
		return nil, errInternal(err)
	}

	// ---- 2.5) 应用范围:可选"应用到所有会话(含已有)" ----
	// 默认(false/缺省)保持"仅新建对话生效"的快照语义;用户在设置页显式勾选后,
	// 把本次合并出的新值批量写进所有存量 codex 会话的 SettingsSnapshot,
	// 让已有会话的后续每一轮也按新值装配(codex.go 的三级优先级里快照最优先,
	// 所以必须改快照本身,改全局偏好对有快照的老会话是无效的)。
	// pi/zcode 会话不消费快照,不在批量范围内。
	applied := 0
	if req.GetApplyToExistingThreads() {
		n, err := s.cfg.Store.ApplySettingsSnapshotToThreads("codex", turnSettingsFromPrefs(agent.DesktopPrefs{
			ApprovalPolicy:   prefs.ApprovalPolicy,
			Sandbox:          prefs.Sandbox,
			Model:            model,
			Effort:           effort,
			HideReasoning:    prefs.HideReasoning,
			Verbosity:        prefs.Verbosity,
			ReasoningSummary: prefs.ReasoningSummary,
		}))
		if err != nil {
			return nil, errInternal(err)
		}
		applied = n
	}

	// ---- 3) 应用到 provider 内存态 ----
	// 传完整快照(含默认模型):provider 侧一律照单全收,空串才等于"跟随桌面"。
	// 【本轮扩展】不再只发给 codex:凡是实现了 agent.PrefsAware 的 provider
	// (codex/pi/zcode)都广播一份,七项设置对所有桌面 agent 真实生效;
	// 各 provider 只消费自己语义说得通的子集(见 agent.PrefsAware 接口注释)。
	s.broadcastPrefs(agent.DesktopPrefs{
		ApprovalPolicy:    prefs.ApprovalPolicy,
		Sandbox:           prefs.Sandbox,
		Model:             model,
		Effort:            effort,
		HideReasoning:     prefs.HideReasoning,
		Verbosity:         prefs.Verbosity,
		ReasoningSummary:  prefs.ReasoningSummary,
		DisablePasteBurst: prefs.DisablePasteBurst,
	})
	// 【第 28 轮】设置可能影响能力/用量/MCP 展示(如模型列表账号组),清缓存让
	// 设置页重读时立刻看到新值,而不是等 TTL 到期。
	if p != nil {
		p.InvalidateAll()
	}

	// ---- 4) 有界回读 ----
	// 【第 3 轮:落盘成功就必须返回】以前这里无界地递归调 GetCodexSettings,
	// 于是"探测 codex 卡死"会传染给保存路径 —— 用户点了保存永远不返回。
	// 现在:回读走共用快照 + 显式上界(codexSetProbeBudget),且快照的值一律来自
	// store。即便探测超时,也**必然**返回落盘后的值(把 state 降级为 unreachable),
	// 绝不会把"回读失败"变成"保存失败"。
	out := s.codexSettingsSnapshot(ctx, codexSetProbeBudget)
	// 应用范围回执:勾选"应用到所有会话"时,手机端据此如实说"已应用到 N 个会话",
	// 不把"保存成功"和"存量会话也改了"混成一句含糊的"已保存"。
	out.AppliedThreads = int32(applied)
	return out, nil
}

// broadcastPrefs 把一份完整偏好快照广播给所有实现了 agent.PrefsAware 的 provider。
//
// 【为什么是可选接口而不是加进 Provider】与 Steerer 同款取舍:provider 想消费
// 桌面偏好就实现 PrefsAware,server 层类型断言探测;echo/OpenAI 兼容 provider
// 没有该概念,不被迫实现空方法。codex 的 ApplyPrefs 签名天然一致,无需适配器。
// 入参是 server 层按 presence 合并后的完整快照(model/effort 为落盘的全局默认
// 值),provider 侧照单全收。SetCodexSettings(保存时)与 New(启动灌入)都走
// 广播语义,保证"改设置"与"重启"两条路径行为一致。
func (s *Server) broadcastPrefs(prefs agent.DesktopPrefs) {
	for _, prov := range s.cfg.Providers {
		if pa, ok := prov.(agent.PrefsAware); ok {
			scoped := prefs
			if prov.ID() != "codex" {
				defaults := s.cfg.Store.AgentDefaults(prov.ID())
				scoped.Model, scoped.Effort = defaults.Model, defaults.Effort
				scoped.NativeMode = defaults.NativeMode
			}
			pa.ApplyPrefs(scoped)
		}
	}
}

// mergeCodexPrefs 按 presence 语义把一次设置请求合并到既有值上。
//
// 【为什么单独抽成纯函数】这是本轮最容易出错、也最该被测试钉死的一段逻辑:
//   - **absent 字段必须原样保留** —— 否则用户只改"思考强度",却把没碰过的
//     "默认模型/审批策略"一起清空(历史上就是这个 bug 的另一面);
//   - **present 的空串是真值** —— model="" 表示"跟随桌面",必须能写回去。
//
// 返回合并后的偏好、默认模型、思考强度。纯函数,无副作用,便于单测。
func mergeCodexPrefs(
	base store.CodexPrefs,
	model, effort string,
	req *pb.SetCodexSettingsRequest,
) (store.CodexPrefs, string, string) {
	out := base
	if req.ApprovalPolicy != nil {
		out.ApprovalPolicy = req.GetApprovalPolicy()
	}
	if req.Sandbox != nil {
		out.Sandbox = req.GetSandbox()
	}
	if req.Model != nil {
		model = req.GetModel()
	}
	if req.Effort != nil {
		effort = req.GetEffort()
	}
	if req.HideAgentReasoning != nil {
		out.HideReasoning = req.GetHideAgentReasoning()
	}
	if req.ModelVerbosity != nil {
		out.Verbosity = req.GetModelVerbosity()
	}
	if req.ModelReasoningSummary != nil {
		out.ReasoningSummary = req.GetModelReasoningSummary()
	}
	if req.DisablePasteBurst != nil {
		out.DisablePasteBurst = req.GetDisablePasteBurst()
	}
	return out, model, effort
}

func (s *Server) agentSettingsSnapshot(id string) *pb.CodexSettings {
	d := s.cfg.Store.AgentDefaults(id)
	return &pb.CodexSettings{Model: d.Model, Effort: d.Effort, NativeMode: d.NativeMode, HideAgentReasoning: s.cfg.Store.CodexPrefs().HideReasoning}
}
func (s *Server) setAgentSettings(ctx context.Context, req *pb.SetCodexSettingsRequest) (*pb.CodexSettings, error) {
	id := req.GetAgentId()
	if _, ok := s.cfg.Providers[id]; !ok {
		return nil, errFailedPrecondition("Agent 不可用: " + id)
	}
	d := s.cfg.Store.AgentDefaults(id)
	if req.Model != nil {
		model := req.GetModel()
		if model != "" && !strings.HasPrefix(model, id+"/") {
			return nil, errInvalidArg("模型不属于 " + id)
		}
		d.Model = model
	}
	if req.Effort != nil {
		d.Effort = req.GetEffort()
	}
	if req.NativeMode != nil {
		mode := strings.TrimSpace(req.GetNativeMode())
		if id != "zcode" {
			return nil, errFailedPrecondition("该 Agent 未报告此原生模式接口")
		}
		if mode != "" {
			source, ok := s.cfg.Providers[id].(interface {
				NativeModes(context.Context) ([]string, error)
			})
			if !ok {
				return nil, errFailedPrecondition("Agent 原生模式接口暂不可用")
			}
			modes, err := source.NativeModes(ctx)
			if err != nil {
				return nil, errUnavailable(err.Error())
			}
			supported := false
			for _, v := range modes {
				if v == mode {
					supported = true
				}
			}
			if !supported {
				return nil, errInvalidArg("Agent 当前未接受该模式: " + mode)
			}
		}
		d.NativeMode = mode
		if req.GetThreadId() != "" {
			thread, ok := s.cfg.Store.Get(req.GetThreadId())
			if !ok || threadAgentID(thread) != id {
				return nil, errInvalidArg("会话不属于所选 Agent")
			}
			if mode != "" {
				native, ok := s.cfg.Providers[id].(interface {
					SetSessionMode(context.Context, *store.Thread, string) (agent.ZcodeSessionCapabilities, error)
				})
				if !ok {
					return nil, errFailedPrecondition("Agent 不支持会话模式设置")
				}
				actual, err := native.SetSessionMode(ctx, thread, mode)
				if err != nil {
					return nil, errUnavailable(err.Error())
				}
				d.NativeMode = actual.CurrentMode
			}
			if thread.SettingsSnapshot == nil {
				thread.SettingsSnapshot = &store.TurnSettings{}
			}
			thread.SettingsSnapshot.NativeMode = d.NativeMode
			if err := s.cfg.Store.Update(thread); err != nil {
				return nil, errInternal(err)
			}
			return s.nativeSettings(ctx, id, thread.ID), nil
		}
	}
	if err := s.cfg.Store.SetAgentDefaults(id, d); err != nil {
		return nil, errInternal(err)
	}
	if req.GetApplyToExistingThreads() {
		if _, err := s.cfg.Store.ApplySettingsSnapshotToThreads(id, &d); err != nil {
			return nil, errInternal(err)
		}
	}
	s.broadcastPrefs(agent.DesktopPrefsFromStore(s.cfg.Store))
	return s.nativeSettings(ctx, id, ""), nil
}
