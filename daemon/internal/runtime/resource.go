package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ---- 作用域层级 --------------------------------------------------------------

// 规格 §6.5：预算必须可以按「全局 → agent → workspace → thread → collab session」
// 逐级覆盖。下面这组构造函数是那套层级的**唯一词汇表** —— 各处拼字符串
// 一定会拼出 "agent/echo" 与 "agent:echo" 两种写法，然后被当成两个 scope 各记一份账。
func ScopeGlobal() string                { return "global" }
func ScopeAgent(agentID string) string   { return "agent:" + agentID }
func ScopeThread(threadID string) string { return "thread:" + threadID }
func ScopeCollab(sessionID string) string {
	return "collab:" + sessionID
}
func ScopeWorkspace(workspace string) string { return "workspace:" + workspace }

// ResourceKind 是被记账的资源类别（规格 §6.1 的三类资源 + 并发 + 终端）。
type ResourceKind string

const (
	// ResWorkingRAM 运行时 RAM：当前回复、事件缓冲、工具输出、终端输出、协同状态、session 映射。
	ResWorkingRAM ResourceKind = "ram.working"
	// ResEventBuffer 事件缓冲（可 spill 到磁盘）。
	ResEventBuffer ResourceKind = "ram.event_buffer"
	// ResSessionIndex 原生 session 索引（RAM）。
	ResSessionIndex ResourceKind = "ram.session_index"
	// ResConversation 会话记忆落盘量。
	ResConversation ResourceKind = "disk.conversation"
	// ResCacheDisk 可再生缓存落盘量。
	ResCacheDisk ResourceKind = "disk.cache"
	// ResArtifactDisk artifact 落盘量。
	ResArtifactDisk ResourceKind = "disk.artifact"
	// ResConcurrency 并发轮次（max_parallel_turns）。
	ResConcurrency ResourceKind = "concurrency.turns"
	// ResTerminalOut 终端输出。
	ResTerminalOut ResourceKind = "terminal.output"
)

// SpillPolicy 是软上限被触达时的处置方式。
type SpillPolicy string

const (
	// SpillCompact 优先压缩（如把上下文压成摘要）。
	SpillCompact SpillPolicy = "compact"
	// SpillToDisk 落盘换内存。
	SpillToDisk SpillPolicy = "spill"
	// SpillReject 无法压缩也无法落盘 → 拒绝新增（只对可选资源）。
	SpillReject SpillPolicy = "reject"
)

// Pressure 是资源压力档位。
type Pressure string

const (
	// PressureNormal 低于软上限。
	PressureNormal Pressure = "normal"
	// PressureSoft 达到软上限 → 优先压缩和落盘。
	PressureSoft Pressure = "soft"
	// PressureHard 达到硬上限 → 拒绝新增**可选**资源。
	PressureHard Pressure = "hard"
)

// ClassifyPressure 把规格 §6.5 的压力规则固化成纯函数。
//
// 规则（逐字对应规格）：
//
//	资源达到软上限时优先压缩和落盘；达到硬上限时拒绝新增可选资源，
//	但不得静默删除用户数据或伪造任务完成。
//
// 把判定抽出来是为了让"软/硬"这条边界只有一个实现 —— 分散在各方手里时，
// 必然出现一处按 >= 一处按 > 的偏差，表现为"偶尔莫名拒收"。
func ClassifyPressure(used, soft, hard int64) Pressure {
	if hard > 0 && used >= hard {
		return PressureHard
	}
	if soft > 0 && used >= soft {
		return PressureSoft
	}
	return PressureNormal
}

var (
	// ErrOverHardLimit 表示达到硬上限且该资源是可选资源，本次新增被拒绝。
	//
	// 【这不是"资源耗尽"的托词，是一条 contract】收到它的调用方必须：
	//  1. 先尝试压缩 / 落盘（SpillPolicy 指明的动作）；
	//  2. 仍然不行就如实向上报"这一项没做成"。
	// 绝不允许把 ErrOverHardLimit 翻译成"已完成"（伪造完成），
	// 也绝不允许为腾地方删掉用户数据（用户消息、会话记录、凭据）。
	ErrOverHardLimit = errors.New("runtime: 达到硬上限，拒绝新增可选资源")
	// ErrLeaseNotFound 表示租约 id 不存在或已释放。
	ErrLeaseNotFound = errors.New("runtime: 资源租约不存在或已释放")
)

// LeaseRequest 是申请一份资源租约的输入。
type LeaseRequest struct {
	LeaseID       string       `json:"lease_id"`
	OwnerScope    string       `json:"owner_scope"`
	Kind          ResourceKind `json:"resource_kind"`
	ReservedBytes int64        `json:"reserved_bytes"`
	SoftLimit     int64        `json:"soft_limit"`
	HardLimit     int64        `json:"hard_limit"`
	SpillPolicy   SpillPolicy  `json:"spill_policy"`

	// Optional 标记这是"可选资源"。
	//
	// 【为什么必须有这个位】规格只允许硬上限拒绝**可选**资源。把必需资源
	// （保存用户消息、落盘已完成的轮次结果）也拒掉，等于换个名字丢用户数据。
	// 因此 Optional=true → 硬上限时返回 ErrOverHardLimit；
	// Optional=false → 允许越限记账，但 Usage 里会如实反映超限，
	// 供诊断与 UI 显示压力（不静默）。
	Optional bool `json:"optional"`
}

// ResourceLease 一份资源租约（规格 §4.7）。
type ResourceLease struct {
	LeaseID       string       `json:"lease_id"`
	OwnerScope    string       `json:"owner_scope"`
	Kind          ResourceKind `json:"resource_kind"`
	ReservedBytes int64        `json:"reserved_bytes"`
	UsedBytes     int64        `json:"used_bytes"`
	SoftLimit     int64        `json:"soft_limit"`
	HardLimit     int64        `json:"hard_limit"`
	SpillPolicy   SpillPolicy  `json:"spill_policy"`
	Optional      bool         `json:"optional"`
	CreatedAt     time.Time    `json:"created_at"`
	ReleasedAt    time.Time    `json:"released_at,omitempty"`
}

// Pressure 报告本租约当前压力档。
func (l ResourceLease) Pressure() Pressure {
	return ClassifyPressure(l.UsedBytes, l.SoftLimit, l.HardLimit)
}

// UsageReport 是一个作用域下的用量汇总。
type UsageReport struct {
	Scope    string                 `json:"scope"`
	Leases   int                    `json:"leases"`
	ByKind   map[ResourceKind]int64 `json:"by_kind"`
	OverHard []string               `json:"over_hard,omitempty"`
	OverSoft []string               `json:"over_soft,omitempty"`
}

// ResourceLedger 是统一资源记账入口（规格 §6.5）。
//
// 【实现者须知】本接口只记账、只拒绝，**不做任何删除**。
// 需要腾地方是调用方的事（压缩、落盘、请用户决策），因为只有调用方知道
// 哪些内容是可再生的、哪些是用户数据。把删除权下放到记账层，是资源管理
// 变成数据丢失事故的标准路径。
type ResourceLedger interface {
	// Acquire 申请租约。可选资源触硬上限时返回 ErrOverHardLimit。
	Acquire(ctx context.Context, req LeaseRequest) (*ResourceLease, error)
	// Charge 增加某租约的已用量（记账；不触发删除）。
	Charge(ctx context.Context, leaseID string, bytes int64) (*ResourceLease, error)
	// Release 释放租约（ReleasedAt 置位，用量归零）。
	Release(ctx context.Context, leaseID string) error
	// Usage 汇总某作用域（scope 为空 = 全局）的用量。
	Usage(ctx context.Context, scope string) UsageReport
}

// Budget 是一份预算（规格 §6.5 的初始基线形态）。
// 单位：字节；ResConcurrency 这类计数资源复用同一结构（值即个数）。
type Budget struct {
	Soft int64 `json:"soft_bytes"`
	Hard int64 `json:"hard_bytes"`
}

// Check 校验预算自洽（软上限不得超过硬上限）。
// 配置里出现 soft > hard 时，ClassifyPressure 永远先命中 hard，
// 软上限形同不存在 —— 这种配置错误必须在装载时就被拒，不能等到运行时表现怪异。
func (b Budget) Check() error {
	if b.Soft < 0 || b.Hard < 0 {
		return fmt.Errorf("runtime: 预算不得为负 (soft=%d hard=%d)", b.Soft, b.Hard)
	}
	if b.Hard > 0 && b.Soft > b.Hard {
		return fmt.Errorf("runtime: 软上限(%d)不得超过硬上限(%d)", b.Soft, b.Hard)
	}
	return nil
}
