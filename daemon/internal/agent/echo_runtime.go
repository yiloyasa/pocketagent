package agent

import (
	"context"
	"errors"

	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// echoAgentID 是 Echo 的注册 id（与 DetectProviders 的 m["echo"] 同源）。
const echoAgentID = "echo"

// errEchoNilThread 表示缺 thread 无法交付运行时。
// fail closed 而不是返回一个无主的运行时：thread 缺失时"交付成功"会让调用方
// 往一个不存在的会话上写记忆，比直接报错难查得多。
var errEchoNilThread = errors.New("echo: thread 为空，无法交付运行时")

// EchoProvider 的运行时接线（P0.5「Echo 先作为最低契约测试 provider」）。
//
// 【为什么是 Echo 先】它没有任何原生 session、没有原生缓存、没有原生记忆 ——
// 也就是规格 §6 最担心的那类"退化成无记忆、无缓存、无恢复的临时执行器"。
// 让它先跑通完整契约，等于证明了"没有原生能力的 agent 也能达标"这条路成立：
// 后面接 OpenAI 兼容、或者任何第三方 agent，照抄它的形状即可。
//
// 【默认实现而非空壳】四个入口全部由 runtime.Backend 提供真实语义（不是返回
// nil 的占位）。日后 Echo 若想用原生实现，只换 AgentRuntime 里对应字段。

// echoRuntimeDescriptor 是 Echo 对自己运行时形态的**如实申报**。
//
// 【为什么 Limitations 列了这么多】契约的要点是"缺失可见"。Echo 做不到的项
// 在这里逐条点名，UI 就能显示"该 agent 未报告此项"，而不是整块面板消失、
// 让人以为是自己没找到。注意区分两类记忆/缓存：
//   - runtime.memory / runtime.cache = daemon 侧公共入口（Echo **有**）
//   - memory.native / cache.native    = 原生自带的记忆与缓存（Echo **没有**）
//
// 把这两类混为一谈，就会出现"因为没原生 API 所以判定它没有记忆"的错误结论 ——
// 那正是规格 §6 开头那句禁令要拦的东西。
func echoRuntimeDescriptor() runtime.AgentDescriptor {
	return runtime.AgentDescriptor{
		ID:          echoAgentID,
		DisplayName: "回声模式(无需 API key)",
		Capabilities: []runtime.Capability{
			// 四个必需入口：Echo 交付默认实现，因此如实声明支持。
			runtime.CapRuntimeMemory,
			runtime.CapRuntimeCache,
			runtime.CapRuntimeCheckpoint,
			runtime.CapRuntimeResource,
			// Echo 会经 sink 逐帧吐事件（ThinkingDelta / TextDelta / ToolCall）。
			runtime.CapTurnStream,
			// Echo 能自报运行时形态（就是本描述符）。
			runtime.CapRuntimeCapabilities,
		},
		SessionModel:  runtime.SourceDaemon,
		ResumeModel:   runtime.ResumeReattach,
		ApprovalModel: runtime.ApprovalNone,
		SubagentModel: runtime.SubagentNone,
		MemoryModel:   runtime.MemoryDaemon,
		CacheModel:    runtime.CacheDaemon,
		Limitations: []runtime.Limitation{
			{Capability: runtime.CapThreadResume, Reason: "无原生会话可续：Echo 不持有任何外部会话状态"},
			{Capability: runtime.CapSessionExport, Reason: "无可导出的原生会话"},
			{Capability: runtime.CapTurnSteer, Reason: "不支持运行中插话，server 层走排队兜底（见 turn_queue.go）"},
			{Capability: runtime.CapApprovalRequest, Reason: "不产生审批请求（Echo 不调用 approve hook）"},
			{Capability: runtime.CapApprovalResume, Reason: "同上：无审批请求即无恢复"},
			{Capability: runtime.CapSubagentSpawn, Reason: "无子智能体概念"},
			{Capability: runtime.CapSubagentEvents, Reason: "无子智能体概念"},
			{Capability: runtime.CapSubagentTranscript, Reason: "无子智能体概念"},
			{Capability: runtime.CapCollabDelegate, Reason: "未把 collab MCP 挂进会话，无跨 agent 委派能力"},
			{Capability: runtime.CapMemoryNative, Reason: "无原生记忆接口；记忆由 daemon 侧 runtime.memory 承担"},
			{Capability: runtime.CapMemoryCompaction, Reason: "无原生压缩；压缩由 runtime 的 Compact 承担"},
			{Capability: runtime.CapCacheNative, Reason: "无原生缓存契约；缓存由 daemon 侧 runtime.cache 承担"},
		},
	}
}

// Descriptor 实现 runtime.Participant（Echo 的形态申报）。
func (e *EchoProvider) Descriptor() runtime.AgentDescriptor { return echoRuntimeDescriptor() }

// OpenRuntime 实现 runtime.Participant：为某个 thread 交付运行时入口。
//
// 【为什么不在这里 Open 会话】OpenRuntime 是个交付动作，不是状态变更动作。
// 在这里顺手把 thread.ExternalSession 写掉，会让"取运行时"这个读起来无害的
// 调用带上副作用 —— 调用方在健康探测或诊断导出里调它就会改状态。
// 会话在轮次开始时才 Open（见 server 侧的轮次启动路径）。
func (e *EchoProvider) OpenRuntime(_ context.Context, thread *store.Thread) (*runtime.AgentRuntime, error) {
	if thread == nil {
		return nil, errEchoNilThread
	}
	return sharedRuntimeBackend().FullRuntime(echoRuntimeDescriptor(), thread.ID), nil
}

// EchoRuntimeBackend 暴露默认后端，供契约测试与诊断使用。
// （TTL / 淘汰 / 重启三类检查需要直接操作时钟与持久态，拿不到后端就只能跳过 ——
// 而静默跳过检查是本项目定义的假绿。）
func (e *EchoProvider) EchoRuntimeBackend() *runtime.Backend { return sharedRuntimeBackend() }

// 【记忆写入搬去了哪】原先这里有个 recordTurn：每轮把用户输入与回复写进公共
// runtime 记忆（+ checkpoint），作为"契约被使用而不只是被交付"的证据点。
// D7 把这件事收归 server 的公共轮次路径（server/turn_memory.go）—— 逐家 provider
// 各写一份会产出五份重复实现，且"谁接了"只在那五个文件里才看得见；收归之后新加
// 一家 provider 自动入列。echo 的形态声明（SessionModel=DAEMON、MemoryModel=DAEMON）
// 不变，server 侧按申报写会话原文，落点与语义与原先一致
// （对应回归用例见 server/turn_memory_test.go::TestEchoTurnWritesConversationThroughServer）。
