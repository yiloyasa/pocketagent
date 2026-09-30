package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func (s *Server) ListModels(ctx context.Context, req *pb.ListModelsRequest) (*pb.ListModelsResponse, error) {
	out := &pb.ListModelsResponse{}
	var stateErrs []string
	if req.GetThreadId() != "" {
		if thread, ok := s.cfg.Store.Get(req.GetThreadId()); ok {
			if native, ok := s.cfg.Providers[threadAgentID(thread)].(agent.CurrentSelectionReader); ok {
				stateCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				model, effort, err := native.CurrentSelection(stateCtx, thread)
				cancel()
				if err != nil {
					stateErrs = append(stateErrs, err.Error())
				} else {
					out.CurrentModelId = model
					out.CurrentEffort = effort
				}
			}
		}
	}

	// 稳定顺序(2026-09-20 动态化,AUDIT 附录 A 第 2 项):不再写死 agent 名单,
	// 全部从 s.cfg.Providers 派生 —— 桌面桥接 agent(消费桌面偏好的
	// agent.PrefsAware,codex/pi/zcode 在 provider.go 编译期钉死实现)在前,
	// 同组按 ID 字典序;其余 provider 随后按字典序;echo 永远排最后兜底。
	// 新增 provider 自动进列表,无需改这里。
	// 【pi 为什么入列】pi 会话的"模型·强度"卡需要列出 pi 的真实模型并渲染
	// 动态思考档位(available_efforts),不进全局列表手机端就永远选不到。
	// 【zcode 为什么入列(设置页 Pi/ZCode 分区,2026-09-19)】设置页的 ZCode 分区
	// 要显示"ZCode 当前有哪些模型",而手机端能拿到的模型数据只有这一个全局列表;
	// ZCode 分区只做**展示**,Codex 默认模型的候选已被手机端按 provider=="codex"
	// 收窄,不会把 zcode 的模型误写进 codex 设置。
	var desktop, plain []string
	for id, p := range s.cfg.Providers {
		if _, isDesktop := p.(agent.PrefsAware); isDesktop {
			desktop = append(desktop, id)
		} else if id != "echo" {
			plain = append(plain, id)
		}
	}
	sort.Strings(desktop)
	sort.Strings(plain)
	order := append(desktop, plain...)
	if _, ok := s.cfg.Providers["echo"]; ok {
		order = append(order, "echo")
	}

	// ---- 并行拉取 + 整段有界(PA-REAL-0002/0003,2026-09-20)----
	// 以前这里按 order 顺序逐个 p.Models():codex 冷启动初始化期间,排在后面的
	// pi/zcode 模型条目被一起拖住,整段轻松越过手机端 30s RPC 截止 → 整个
	// ListModels 对用户表现为裸 DeadlineExceeded(daemon 日志同一窗口里 pi 报
	// 30s 超时、zcode 报"codex 正忙"字样,即 PA-REAL-0003 的表象)。现在各
	// provider 的拉取**并行**执行、整段等待有界:一个 agent 冷启动不再决定其它
	// agent 的模型何时可见。预算内没回来的 provider 绝不静默缺席 —— StateError
	// 如实说"初始化中,可重试"(既不伪造空表/半表为成功,也不抛裸超时,
	// AUDIT §3.3);被放弃的拉取在后台自行完成并写入各自 TTL 缓存,手机端稍后
	// 重试即秒回。各 provider 的生命周期锁都在各自实例上,并行不引入跨 provider
	// 争用;同 provider 的串行边界(turnMu 轮次 / lifeMu 生命周期)不受影响。
	type modelFetch struct {
		done   chan struct{}
		models []pb.ModelInfo
		err    error
	}
	fetches := make(map[string]*modelFetch, len(order))
	var wg sync.WaitGroup
	for _, id := range order {
		if req.GetAgentId() != "" && req.GetAgentId() != id {
			continue
		}
		p, ok := s.cfg.Providers[id]
		if !ok {
			continue
		}
		if req.GetRefresh() {
			if refresh, ok := p.(agent.ModelCatalogRefresher); ok {
				refresh.InvalidateModels()
			}
		}
		f := &modelFetch{done: make(chan struct{})}
		fetches[id] = f
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(f.done)
			// 结果只写进自己的 fetch 槽位,close(done) 提交;主流程只在 done
			// 关闭后读取 —— 超时放弃后 goroutine 收尾写入也不会与收割竞争。
			if ss, ok := p.(modelListStatusSource); ok {
				f.models, f.err = ss.ModelsStatusCtx(ctx)
				return
			}
			f.models = p.Models()
		}()
	}

	// 等待上界 = min(listModelsWaitBudget, RPC 剩余时间)。全部提前完成就提前
	// 返回,不干等满预算(测试可用更短的 ctx 截止驱动,不必真等 10s)。
	wait := listModelsWaitBudget
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d < wait {
			wait = d
		}
	}
	allDone := make(chan struct{})
	go func() { wg.Wait(); close(allDone) }()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-allDone:
	case <-timer.C:
	}

	// 按 order 收割:输出顺序与顺序版实现完全一致(稳定、可对拍);未完成的
	// provider 记一条"初始化中,可重试"的人话,与拉取失败的真实原因一起从
	// StateError 透传(手机端原样显示,AUDIT §3.3)。
	for _, id := range order {
		f, ok := fetches[id]
		if !ok {
			continue
		}
		select {
		case <-f.done:
			if f.err != nil {
				stateErrs = append(stateErrs, id+": "+f.err.Error())
			}
			appendProviderModels(out, s.cfg.Providers[id], f.models)
		default:
			stateErrs = append(stateErrs, id+": "+agent.ErrModelsInitializing.Error())
		}
	}
	out.StateError = strings.Join(stateErrs, "; ")
	return out, nil
}

// listModelsWaitBudget 是 ListModels 对**全部 provider 并行拉取**的整段等待上界
// (PA-REAL-0002:冷调用 ≤10s 内返回)。实际取它与 RPC 剩余时间的更早者。
const listModelsWaitBudget = 10 * time.Second

// modelListStatusSource 是「模型列表拉取能带错误回执」的可选能力(生产由
// *agent.CodexProvider 实现;server 层类型断言探测,与 zcodeHistorySource 同款)。
// 带回执的 provider 走状态预算通道(失败原因可透传);其余 provider 仍走无
// 错误通道的 Models(),失败时由 provider 自己降级(如只回 default 条目)。
type modelListStatusSource interface {
	ModelsStatusCtx(ctx context.Context) ([]pb.ModelInfo, error)
}

// appendProviderModels 把一份 provider 模型表按全局唯一 id 复制进响应。
// 【必须复制后改】providers 的 Models() 可能返回**缓存的共享切片**(pi 的
// modelsCache 就是缓存本体)。以前直接 m.Id 加前缀,写入的是缓存:TTL 内第二次
// 列表会叠成 "pi/pi/…" 双前缀,并发请求还会形成数据竞争。这里逐字段复制到新
// 消息再改,缓存保持只读(历史回归,model_efforts_test.go 钉死)。
func appendProviderModels(out *pb.ListModelsResponse, p agent.Provider, models []pb.ModelInfo) {
	for i := range models {
		m := &models[i]
		// 【vet】按下标取址遍历,避免 range 复制 pb 消息(内嵌 MessageState 含锁)。
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
		out.Models = append(out.Models, cm)
	}
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
	// 原生配置项写通道(dsh 等;与 native_mode 同一请求面的可选分支):两个都 set
	// 才是一次写请求,写完返回该 agent 的设置快照(手机端随后重拉目录刷新 currentValue)。
	if req.NativeConfigOptionId != nil && req.NativeConfigValue != nil {
		return s.setAgentNativeConfigOption(ctx, id, req.GetNativeConfigOptionId(), req.GetNativeConfigValue(), req.GetThreadId())
	}
	if req.NativeConfigOptionId != nil || req.NativeConfigValue != nil {
		return nil, errInvalidArg("原生配置项写入需要同时提供 option_id 与 value")
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

// setAgentNativeConfigOption 原生配置项写通道:SetCodexSettings 的可选分支
// (native_config_option_id + native_config_value 都 set 时进入)。
//
// 【语义】optionID/value 由 provider 校验(NativeConfigOptions;目录外取值拒绝,
// 禁止裸模型名)。thread_id 给了 → 当场写到那个会话;没给 → provider 按
// "新会话初始值"落(dsh 探针实证:原生开关仅会话内生效不持久,与新会话创建时
// 灌入的既有语义对齐)。
func (s *Server) setAgentNativeConfigOption(ctx context.Context, agentID, optionID, value, threadID string) (*pb.CodexSettings, error) {
	if strings.TrimSpace(optionID) == "" {
		return nil, errInvalidArg("缺少 native_config_option_id")
	}
	provider, ok := s.cfg.Providers[agentID]
	if !ok {
		return nil, errFailedPrecondition("Agent 不可用: " + agentID)
	}
	src, ok := provider.(agent.NativeConfigOptions)
	if !ok {
		return nil, errFailedPrecondition("该 Agent 未提供原生设置写通道: " + agentID)
	}
	var thread *store.Thread
	if tid := strings.TrimSpace(threadID); tid != "" {
		t, ok := s.cfg.Store.Get(tid)
		if !ok || threadAgentID(t) != agentID {
			return nil, errInvalidArg("会话不属于所选 Agent")
		}
		thread = t
	}
	if err := src.SetNativeConfigOption(ctx, thread, optionID, value); err != nil {
		return nil, errUnavailable(err.Error())
	}
	// 【失效纪律】原生配置写成功 = 本地下层事实变了(与 SetCodexSettings 成功后
	// InvalidateAll 同款):dsh 的模型目录数据源就是 session/new 应答里的
	// config-option 状态,写完不清缓存的话,手机端"随后重拉目录"拿到的还是
	// TTL 内的旧 currentValue。谁实现了失效接口就清谁,不强迫所有 provider 摆空架子。
	if inv, ok := provider.(interface{ InvalidateAll() }); ok {
		inv.InvalidateAll()
	}
	if ref, ok := provider.(agent.ModelCatalogRefresher); ok {
		ref.InvalidateModels()
	}
	return s.nativeSettings(ctx, agentID, threadID), nil
}
