package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// 本文件让 codex / pi / zcode / dsh / OpenAI 兼容 五家接入统一运行时契约（P0.5）。
//
// 【设计要点一：能力来自接口事实，不来自名单】
// describeRuntime 里的能力判定全部走类型断言（Steerer / CollabAware），与 host.go
// 既有的 `collab:<id>` 生成逻辑同源。agentShape 只承载"接口探测不出来的那部分"
// （会话/记忆/缓存的归属 + 原生能力），且每条都在声明处注明证据。
// 任何"按 agent 名字决定能力"的写法都违反 README §0.5 与 AUDIT §3.3。
//
// 【设计要点二：不改动 5 家 provider 自己的文件】
// Go 允许方法声明在任何同包文件里。集中放置让"契约接线"与"协议实现"可以分开
// 审阅，接入面也一眼可见（谁接入了、接入成什么样）。
//
// 【本轮取证的边界（诚实声明）】
//   - 已声明：由接口事实推导的能力、协议里能直接读到的能力（thread/resume、
//     switch_session、session/resume、compaction、ApprovalRequest 发射点）。
//   - 未声明：本轮未逐项取证的可选能力（例如各家 provider 内部的模型缓存是否
//     应算"原生 cache 契约"）。**未声明不等于不支持** —— 它表示"本轮未取证"。
//     这类项按项目既有规则处理（UI 无该能力即不显示专属入口），P1 逐项补齐。
//     把它们写成"不支持"的 Limitation 是另一类错误：把"没查"说成"做不到"。

// agentShape 是接口探测不出来的形态声明。
type agentShape struct {
	sessionModel  runtime.SessionSource
	resumeModel   runtime.ResumeModel
	approvalModel runtime.ApprovalModel
	subagentModel runtime.SubagentModel
	memoryModel   runtime.MemoryModel
	cacheModel    runtime.CacheModel

	// nativeCaps 是该 provider 原生具备、但接口探测不出来的能力。
	// 声明处必须写明证据（文件:行），便于复核。
	nativeCaps []runtime.Capability

	// limits 是**已取证**的"做不到"及原因。
	// 只写有证据的：把"未取证"塞进这里会把"没查"伪装成"做不到"。
	limits []runtime.Limitation
}

// describeRuntime 用「接口事实 + 该家自报的形态」拼出描述符。
func describeRuntime(p Provider, shape agentShape) runtime.AgentDescriptor {
	d := runtime.AgentDescriptor{
		ID:            p.ID(),
		DisplayName:   p.DisplayName(),
		SessionModel:  shape.sessionModel,
		ResumeModel:   shape.resumeModel,
		ApprovalModel: shape.approvalModel,
		SubagentModel: shape.subagentModel,
		MemoryModel:   shape.memoryModel,
		CacheModel:    shape.cacheModel,
	}

	// 四个必需入口 + 自报能力：由共享默认后端交付（见 openSharedRuntime），
	// 因此如实声明支持 —— 这是本波真正新增的能力，不是纸面声明。
	d.Capabilities = append(d.Capabilities, runtime.RequiredEntryPoints...)
	d.Capabilities = append(d.Capabilities, runtime.CapRuntimeCapabilities)
	// 所有 provider 都经 sink 逐帧吐事件（含 OpenAI 兼容：它发 Text/Thinking/ToolCall）。
	d.Capabilities = append(d.Capabilities, runtime.CapTurnStream)

	// 接口事实 ①：会插话的才有 turn.steer（实测：codex codex_steer.go:51、pi pi.go:965）。
	if _, ok := p.(Steerer); ok {
		d.Capabilities = append(d.Capabilities, runtime.CapTurnSteer)
	}
	// 接口事实 ②：协同判定走"接线事实"而非意愿，与 host.go 的 collab:<id> 同一条判据。
	if ca, ok := p.(CollabAware); ok && ca.CollabReady() {
		d.Capabilities = append(d.Capabilities, runtime.CapCollabDelegate)
	}

	d.Capabilities = append(d.Capabilities, shape.nativeCaps...)
	d.Limitations = append(d.Limitations, shape.limits...)
	return d
}

// ---- 各家的形态（每条注明证据）------------------------------------------------

// codexShape 证据：
//   - 原生会话：codex.go:852 / 1694 thread/resume（ResumeModel=Native）
//   - 会话归属：Hybrid —— 原生 rollout 在 codex 侧，daemon 侧 threads.json 存消息（README §9.1）
//   - 原生压缩：codex_compact.go + codex.go:1091 处理 contextCompaction item
//   - 原生缓存：capabilities_cache.go（capCache/mcpCache/usageCache）
//   - 子代理：codex_subagents.go / codex_subagent_history.go + codex.go:624 subAgentActivity
//   - 审批：codex.go 发 TurnServerMessage_ApprovalResolved
var codexShape = agentShape{
	sessionModel:  runtime.SourceHybrid,
	resumeModel:   runtime.ResumeNative,
	approvalModel: runtime.ApprovalNative,
	subagentModel: runtime.SubagentNative,
	memoryModel:   runtime.MemoryHybrid,
	cacheModel:    runtime.CacheNative,
	nativeCaps: []runtime.Capability{
		runtime.CapThreadResume,
		runtime.CapApprovalRequest,
		runtime.CapApprovalResume,
		runtime.CapSubagentSpawn,
		runtime.CapSubagentEvents,
		runtime.CapSubagentTranscript,
		runtime.CapMemoryNative,
		runtime.CapMemoryCompaction,
		runtime.CapCacheNative,
	},
}

// piShape 证据：
//   - 原生会话重挂：pi.go:17 / 249 switch_session + get_state 校验（ResumeModel=Reattach）
//   - 原生压缩：pi.go:717 处理 compaction_start/end
//   - 子代理：pi_subagents.go
//   - 审批：**未取证到 pi 向 daemon 发射 ApprovalRequest**，故 ApprovalNone。
//     注：其内建权限模型若产生审批，应在接入时补声明（保守取值 = 与当前行为一致）。
var piShape = agentShape{
	sessionModel:  runtime.SourceHybrid,
	resumeModel:   runtime.ResumeReattach,
	approvalModel: runtime.ApprovalNone,
	subagentModel: runtime.SubagentNative,
	memoryModel:   runtime.MemoryHybrid,
	cacheModel:    runtime.CacheDaemon,
	nativeCaps: []runtime.Capability{
		runtime.CapThreadResume,
		runtime.CapSubagentSpawn,
		runtime.CapSubagentEvents,
		runtime.CapSubagentTranscript,
		runtime.CapMemoryNative,
		runtime.CapMemoryCompaction,
	},
	limits: []runtime.Limitation{
		{Capability: runtime.CapCacheNative, Reason: "provider 内模型缓存尚未收归公共 CacheManager（P1 迁移项，见 zcode.go:104 / pi.go 的 modelsCache）"},
	},
}

// zcodeShape 证据：
//   - 原生会话重挂：zcode.go:19-20 threadMap + Thread.ExternalSession.SessionID 落库，
//     子进程重启后走 session/resume（ResumeModel=Reattach）
//   - 审批：未取证到发射 ApprovalRequest → ApprovalNone
//   - 子代理：未取证到接线（规格 §7 提到 ZCode 有子 agent 能力差异）。
//     **保守取 SubagentNone**：与当前 host.go 的 capabilities 行为一致，不虚报入口。
//     若实际具备，应在 P1 取证后改正。
var zcodeShape = agentShape{
	sessionModel:  runtime.SourceHybrid,
	resumeModel:   runtime.ResumeReattach,
	approvalModel: runtime.ApprovalNone,
	subagentModel: runtime.SubagentNone,
	memoryModel:   runtime.MemoryDaemon,
	cacheModel:    runtime.CacheDaemon,
	nativeCaps: []runtime.Capability{
		runtime.CapThreadResume,
	},
	limits: []runtime.Limitation{
		{Capability: runtime.CapCacheNative, Reason: "projection/模型缓存留在 provider 内（zcode.go:104 refByModel、:106 projByThread），尚未收归公共 CacheManager（P1 迁移项）"},
		{Capability: runtime.CapMemoryCompaction, Reason: "未取证到原生压缩实现；压缩由 runtime 的 Compact 承担"},
	},
}

// dshShape 证据：
//   - 原生会话重挂：dsh.go:707 session/new、:735 session/resume（ResumeModel=Reattach）
//   - 审批：dsh.go:1071 发射 TurnServerMessage_ApprovalRequest（全家唯一实测发射点）
//   - 子代理：未取证到接线 → SubagentNone（同上，保守取值）
var dshShape = agentShape{
	sessionModel:  runtime.SourceHybrid,
	resumeModel:   runtime.ResumeReattach,
	approvalModel: runtime.ApprovalNative,
	subagentModel: runtime.SubagentNone,
	memoryModel:   runtime.MemoryDaemon,
	cacheModel:    runtime.CacheDaemon,
	nativeCaps: []runtime.Capability{
		runtime.CapThreadResume,
		runtime.CapApprovalRequest,
		runtime.CapApprovalResume,
	},
	limits: []runtime.Limitation{
		{Capability: runtime.CapCacheNative, Reason: "config catalog 缓存留在 provider 内（dsh.go:126），尚未收归公共 CacheManager（P1 迁移项）"},
		{Capability: runtime.CapMemoryCompaction, Reason: "未取证到原生压缩实现；压缩由 runtime 的 Compact 承担"},
	},
}

// openAIShape 证据：
//   - 无任何原生会话：openai_compat.go 不发 thread/session 类调用（ResumeModel=None）
//   - 事件面：只发 Text/Thinking/ToolCall（不发 Usage、不发 CodexEvent）
//   - 这正是规格 §7 点名的形态：「通常没有稳定原生 session → daemon 是主要事实来源」
var openAIShape = agentShape{
	sessionModel:  runtime.SourceDaemon,
	resumeModel:   runtime.ResumeNone,
	approvalModel: runtime.ApprovalNone,
	subagentModel: runtime.SubagentNone,
	memoryModel:   runtime.MemoryDaemon,
	cacheModel:    runtime.CacheDaemon,
	limits: []runtime.Limitation{
		{Capability: runtime.CapThreadResume, Reason: "无稳定原生 session；会话事实由 daemon conversation memory 承担"},
		{Capability: runtime.CapSessionExport, Reason: "无可导出的原生会话"},
		{Capability: runtime.CapMemoryNative, Reason: "无原生记忆接口；记忆由 daemon 侧 runtime.memory 承担"},
	},
}

// ---- Participant 实现（集中于此，provider 自身文件零改动）---------------------

func (p *CodexProvider) Descriptor() runtime.AgentDescriptor {
	return describeRuntime(p, codexShape)
}

func (p *PiProvider) Descriptor() runtime.AgentDescriptor {
	return describeRuntime(p, piShape)
}

func (p *ZcodeProvider) Descriptor() runtime.AgentDescriptor {
	return describeRuntime(p, zcodeShape)
}

func (p *DshProvider) Descriptor() runtime.AgentDescriptor {
	return describeRuntime(p, dshShape)
}

func (p *OpenAICompatible) Descriptor() runtime.AgentDescriptor {
	return describeRuntime(p, openAIShape)
}

func (p *CodexProvider) OpenRuntime(_ context.Context, th *store.Thread) (*runtime.AgentRuntime, error) {
	return openSharedRuntime(p, th)
}

func (p *PiProvider) OpenRuntime(_ context.Context, th *store.Thread) (*runtime.AgentRuntime, error) {
	return openSharedRuntime(p, th)
}

func (p *ZcodeProvider) OpenRuntime(_ context.Context, th *store.Thread) (*runtime.AgentRuntime, error) {
	return openSharedRuntime(p, th)
}

func (p *DshProvider) OpenRuntime(_ context.Context, th *store.Thread) (*runtime.AgentRuntime, error) {
	return openSharedRuntime(p, th)
}

func (p *OpenAICompatible) OpenRuntime(_ context.Context, th *store.Thread) (*runtime.AgentRuntime, error) {
	return openSharedRuntime(p, th)
}

// openSharedRuntime 用共享默认后端交付四个入口 + daemon 侧会话适配器。
//
// 【为什么现在就用默认实现而不是等原生实现】规格 §13 P0.5 的目标是"所有 agent
// 都必须拥有 memory/cache/checkpoint/resource 入口"。让它们先落在共享默认实现上，
// 契约当天成立；日后某家想换原生实现，只替换 AgentRuntime 里的对应字段。
// 反过来若先等原生实现，这段时间里它们仍是"无记忆、无缓存、无恢复"的执行器 ——
// 正是规格要点名的形态。
func openSharedRuntime(p runtime.Participant, th *store.Thread) (*runtime.AgentRuntime, error) {
	if th == nil {
		return nil, fmt.Errorf("%s: thread 为空，无法交付运行时", p.Descriptor().ID)
	}
	desc := p.Descriptor()
	rt := sharedRuntimeBackend().FullRuntime(desc, th.ID)
	// 共享默认后端交付的 DaemonSession 只对"会话事实在 daemon 侧"的家成立。
	// 对声明 NATIVE/HYBRID 的家（codex/pi/zcode/dsh），原生会话适配器属 P1 迁移项，
	// 此刻尚未交付；塞一个 Source=DAEMON 的替身会构成 source-mismatch 违约
	// （声明 HYBRID 却交付 DAEMON），并让恢复路径按错误前提决策。
	// 置 nil 才是诚实表达 —— VerifyRuntime 与契约套件都按"未交付"处理。
	if desc.SessionModel != runtime.SourceDaemon {
		rt.NativeSession = nil
	}
	return rt, nil
}

// ---- 共享默认后端 -------------------------------------------------------------

var (
	sharedBackendOnce sync.Once
	sharedBackend     *runtime.Backend
)

// sharedRuntimeBackend 返回进程共享的默认后端（惰性初始化，并发安全）。
//
// 【为什么全进程一份而不是每家一份】它就是"运行时状态容器"，诊断页需要一次
// 看到全部 agent 的资源与缓存占用（规格 §12 的 AgentResourceCard）；分成多份
// 就得先知道有哪几家才能问全 —— 又变回名单驱动的设计。
func sharedRuntimeBackend() *runtime.Backend {
	sharedBackendOnce.Do(func() { sharedBackend = runtime.NewBackend() })
	return sharedBackend
}
