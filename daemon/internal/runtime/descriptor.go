// Package runtime 定义 PocketAgent 的本机运行时契约（Local Runtime Control Plane
// 的"契约层"）。
//
// 【这一层解决什么问题】当前 daemon 的状态与能力分散在 server、agent、term、
// eventstore、collabmcp 各自手里：谁能恢复、谁有缓存、谁没有 memory，没有任何
// 统一的地方能回答。设计规格 §6 的硬性要求是：
//
//	任何 agent 接入 PocketAgent，都必须拥有统一的运行时内存、语义记忆、缓存、
//	资源预算、checkpoint 和恢复接口。不能因为某个 agent 没有 Codex 类似的原生
//	API，就退化为无记忆、无缓存、无恢复的临时执行器。
//
// 本包只做三件事：
//
//  1. 定义契约：Descriptor（自报形态）、MemoryManager、CacheManager、
//     ResourceLedger、CheckpointManager、NativeSessionAdapter、Participant。
//  2. 提供默认实现（Backend）：让"暂时没有原生能力"的 agent 也能立刻满足契约，
//     而不是留空壳。留空壳正是规格要点名的形态。
//  3. 提供契约校验器（VerifyDeclaration）：把"缺什么"变成机器可读的报告。
//     **沉默缺失才是要消灭的形态；显式声明"我不支持"是允许的。**
//
// 【依赖方向】runtime → store → pb。runtime 不 import agent / server，
// 因此 agent 可以 import runtime 而不成环（已实测 store 不依赖任何 internal 包）。
//
// 【为什么这里没有 Kernel】规格 §5 的 Kernel（统一持有身份/状态/journal/recovery）
// 是 P1 的收敛动作。本包先立契约与校验器；等 P1 把状态持有者收进来时，
// Kernel 直接实现本包的接口即可，不需要改契约。
package runtime

import "sort"

// ---- 能力声明 ----------------------------------------------------------------

// Capability 是 agent 对外声明的一项运行时能力。
//
// 【为什么是字符串而不是枚举/序号】它要同时服务两个用途：① daemon 内部判定
// （UI 与 server 只按 capability 决定入口，绝不按 agent 名字分支）；② 跨进程
// 透传（GetHostInfo.capabilities 已是 []string，手机端按前缀匹配）。
// 字符串常量组让新增能力不必改协议、不必两端同时上线。
type Capability string

const (
	// 协议会话类
	CapThreadCreate Capability = "thread.create"
	CapThreadResume Capability = "thread.resume"
	CapTurnStream   Capability = "turn.stream"
	CapTurnSteer    Capability = "turn.steer"
	// 审批类
	CapApprovalRequest Capability = "approval.request"
	CapApprovalResume  Capability = "approval.resume"
	// 子智能体 / 协同类
	CapSubagentSpawn      Capability = "subagent.spawn"
	CapSubagentEvents     Capability = "subagent.events"
	CapSubagentTranscript Capability = "subagent.transcript"
	CapCollabDelegate     Capability = "collab.delegate"
	CapSessionExport      Capability = "session.export"
	// 原生记忆 / 缓存类
	CapMemoryNative     Capability = "memory.native"
	CapMemoryCompaction Capability = "memory.compaction"
	CapCacheNative      Capability = "cache.native"

	// 统一运行时入口类 —— 规格 §13 P0.5 要求"所有 agent 都必须拥有
	// memory/cache/checkpoint/resource 入口"，这四个标识就是那四道门的机器可读名字。
	CapRuntimeMemory     Capability = "runtime.memory"
	CapRuntimeCache      Capability = "runtime.cache"
	CapRuntimeCheckpoint Capability = "runtime.checkpoint"
	CapRuntimeResource   Capability = "runtime.resource"
)

// RequiredEntryPoints 是 P0.5 要求每个 agent 都必须交付的最小入口集。
//
// 交付方式有二，走哪条都算合规：
//   - 真实现（如 Echo 用 Backend 的默认实现 + 自己的原生接线）；
//   - 在 AgentDescriptor.Limitations 里**显式声明不支持**（并把原因写清）。
//
// 唯一不合规的是"既没实现、也没声明"——那正是沉默缺失。
var RequiredEntryPoints = []Capability{
	CapRuntimeMemory,
	CapRuntimeCache,
	CapRuntimeCheckpoint,
	CapRuntimeResource,
}

// ---- 会话模型 ----------------------------------------------------------------

// SessionSource 说明"原生会话是不是事实来源"。
//
// 【为什么一个类型管两件事】规格 §4.5/§6.2 分别用 SessionModel（descriptor 字段）
// 和 SessionSource（NativeSessionAdapter.Source()）表达同一件事：会话事实在谁手里。
// 合成一个类型可以避免两个平行枚举各自漂移 —— 一旦漂移，恢复路径就会按错误
// 的前提决策（这正是"原生态与 daemon 态各说各话"的形态）。
type SessionSource string

const (
	// SourceNative 原生 agent 会话是事实来源（codex thread/resume）。
	SourceNative SessionSource = "NATIVE"
	// SourceDaemon PocketAgent 自己保存会话事实（无原生会话能力的 agent）。
	SourceDaemon SessionSource = "DAEMON"
	// SourceHybrid 原生会话保存正文，daemon 保存索引、摘要和恢复状态（pi / zcode / dsh）。
	SourceHybrid SessionSource = "HYBRID"
	// SourceNone 不具备持久会话能力，必须使用 daemon 侧兜底。
	SourceNone SessionSource = "NONE"
)

// ResumeModel 描述"daemon/子进程重启后怎么把会话接回来"。
type ResumeModel string

const (
	// ResumeNative 走原生 session/resume。
	ResumeNative ResumeModel = "native"
	// ResumeReattach 按落库引用重挂载（daemon 侧握手，如 pi switch_session）。
	ResumeReattach ResumeModel = "reattach"
	// ResumeNone 不可恢复，只能新建会话。
	ResumeNone ResumeModel = "none"
)

// ApprovalModel 描述审批请求从哪来。
type ApprovalModel string

const (
	// ApprovalNative 原生协议自带审批请求（codex / zcode）。
	ApprovalNative ApprovalModel = "native"
	// ApprovalDaemon 审批由 daemon 中继到手机（无原生审批的 agent）。
	ApprovalDaemon ApprovalModel = "daemon"
	// ApprovalNone 不产生审批。
	ApprovalNone ApprovalModel = "none"
)

// SubagentModel 描述子智能体从哪来（规格 §11 的三类协同之一）。
type SubagentModel string

const (
	// SubagentNative 原生 agent 自己决定创建子 agent。
	SubagentNative SubagentModel = "native"
	// SubagentDelegate 由用户或父 agent 显式委派。
	SubagentDelegate SubagentModel = "delegate"
	// SubagentNone 不支持子智能体。
	SubagentNone SubagentModel = "none"
)

// MemoryModel 描述语义记忆的归属。
type MemoryModel string

const (
	// MemoryNative 原生压缩/记忆（如 codex context compaction）。
	MemoryNative MemoryModel = "native"
	// MemoryDaemon daemon 侧 conversation memory 是主要记忆。
	MemoryDaemon MemoryModel = "daemon"
	// MemoryHybrid 原生存正文，daemon 存摘要与索引。
	MemoryHybrid MemoryModel = "hybrid"
)

// CacheModel 描述可再生缓存的归属。
type CacheModel string

const (
	// CacheNative 原生提供缓存语义（如 codex 的 capabilities/usage 查询）。
	CacheNative CacheModel = "native"
	// CacheDaemon 由 daemon 侧的 CacheManager 承担。
	CacheDaemon CacheModel = "daemon"
)

// ---- 描述符 ------------------------------------------------------------------

// Limitation 是一条"我做不到"的显式声明。
//
// 【为什么必须有它】契约的价值在于"缺失可见"。没有 Limitations，一个没实现
// memory 的 agent 和一个实现了但恰好没数据的 agent 在外部观察上完全一样 ——
// 那正是规格 §6 要消灭的形态。Capability 字段本身就是机器可读的编码，
// 不再引入第二套 code 命名空间。
type Limitation struct {
	Capability Capability `json:"capability"`
	Reason     string     `json:"reason"`
}

// AgentDescriptor 是 agent 对自己运行时形态的**声明**（纯数据，不得有副作用）。
//
// 【为什么不是从 agent 名字推导】按名字分支判定能力是项目红线（README §0.5 /
// AUDIT §3.3「白名单只许减、不许增」）。能力必须由 provider 自己申报，
// daemon 只读申报结果。
type AgentDescriptor struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	// Version 由 provider 自报（"v0.85.1" 这类）；空 = 未探测到版本。
	// 有它之后 host 层的版本展示就不必再对特定 agent 名字特判。
	Version      string       `json:"version,omitempty"`
	Capabilities []Capability `json:"capabilities"`

	SessionModel  SessionSource `json:"session_model"`
	ResumeModel   ResumeModel   `json:"resume_model"`
	ApprovalModel ApprovalModel `json:"approval_model"`
	SubagentModel SubagentModel `json:"subagent_model"`
	MemoryModel   MemoryModel   `json:"memory_model"`
	CacheModel    CacheModel    `json:"cache_model"`

	// Limitations 列出本 agent **明确不支持**的能力及原因。
	// 契约要求：做不到的要在这里点名，不许沉默。
	Limitations []Limitation `json:"limitations,omitempty"`
}

// Has 报告是否声明了某项能力。
func (d AgentDescriptor) Has(c Capability) bool {
	for _, have := range d.Capabilities {
		if have == c {
			return true
		}
	}
	return false
}

// HasAll 报告是否声明了全部给定能力（空参数集恒为 true）。
func (d AgentDescriptor) HasAll(cs ...Capability) bool {
	for _, c := range cs {
		if !d.Has(c) {
			return false
		}
	}
	return true
}

// Limit 返回某能力被显式声明为不支持的那条 Limitation。
func (d AgentDescriptor) Limit(c Capability) (Limitation, bool) {
	for _, l := range d.Limitations {
		if l.Capability == c {
			return l, true
		}
	}
	return Limitation{}, false
}

// SortedCapabilities 返回稳定排序后的能力副本，供日志、测试与跨端透传使用
// （与 host.go GetHostInfo 的 `sort.Strings(caps) // 稳定输出` 同款理由：
// 输出不稳定会让客户端缓存与测试断言变成掷骰子）。
func (d AgentDescriptor) SortedCapabilities() []Capability {
	out := make([]Capability, len(d.Capabilities))
	copy(out, d.Capabilities)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
