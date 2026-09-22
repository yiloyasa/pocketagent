package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// MemoryKind 把"agent 语义记忆"分成规格 §6.3 的四层。
//
// 【为什么不与缓存混在一起】三类资源的留存语义完全不同：
//   - 运行时 RAM：可回收、可落盘，丢了不影响正确性；
//   - 语义记忆：用户数据或可恢复过程数据，丢 = 会话上下文缺失；
//   - 可再生缓存：丢了重拉即可。
//
// 混在一起就会出现"缓存淘汰顺手删掉用户消息"—— 规格 §6.1 明文禁止的形态。
type MemoryKind string

const (
	// MemoryWorking 当前 turn：当前输入/部分回复/当前工具调用/当前审批/当前子任务/当前错误。
	// 轮次结束后归档或压缩，不能无限累积。
	MemoryWorking MemoryKind = "working"

	// MemoryConversation thread 的持续上下文：用户消息、assistant 消息、工具调用与结果、
	// 审批记录、模型与 effort、工作区、agent、外部 session 引用。
	MemoryConversation MemoryKind = "conversation"

	// MemorySummary 上下文过长时的摘要：目标/进展/已完成/未完成/重要决策/文件变化/
	// 失败尝试/用户偏好。必须记录覆盖的 sequence 范围（见 MemorySnapshot.Covered）。
	MemorySummary MemoryKind = "summary"

	// MemoryArtifact 工具输出、文件引用与子 agent 产出（不直接全部塞进 prompt）。
	MemoryArtifact MemoryKind = "artifact"
)

var (
	// ErrMemoryKeyIncomplete 表示 MemoryKey 缺必需维度。
	// 【为什么 fail closed】维度不全的 key 会把两个 thread 的记忆写到同一个槽里 ——
	// 那是"跨会话串味"，比报错严重得多。
	ErrMemoryKeyIncomplete = errors.New("runtime: memory key 不完整（需要 agent_id 与 thread_id）")
	// ErrMemoryClosed 表示 manager 已被 Close，不再接受读写。
	ErrMemoryClosed = errors.New("runtime: memory manager 已关闭")
)

// MemoryKey 定位一份记忆。
type MemoryKey struct {
	AgentID  string     `json:"agent_id"`
	ThreadID string     `json:"thread_id"`
	Kind     MemoryKind `json:"kind"`
	// Scope 是可选的附加维度（如 workspace 路径、artifact 名）。
	// 空 = 该 (agent, thread, kind) 下的默认槽。
	Scope string `json:"scope,omitempty"`
}

// Validate 检查 key 是否具备定位一份记忆的最小维度。
func (k MemoryKey) Validate() error {
	if k.AgentID == "" || k.ThreadID == "" {
		return ErrMemoryKeyIncomplete
	}
	if k.Kind == "" {
		return fmt.Errorf("%w: kind 为空", ErrMemoryKeyIncomplete)
	}
	return nil
}

// String 给日志与测试断言用（含 Scope，便于定位串味）。
func (k MemoryKey) String() string {
	if k.Scope == "" {
		return fmt.Sprintf("%s/%s/%s", k.AgentID, k.ThreadID, k.Kind)
	}
	return fmt.Sprintf("%s/%s/%s#%s", k.AgentID, k.ThreadID, k.Kind, k.Scope)
}

// SequenceRange 是闭区间的 sequence 覆盖范围。
// 【为什么必须记账】压缩把 N 条消息换成一段摘要时，只有记下"摘要覆盖了哪些
// sequence"，才能证明没有消息凭空消失。这个范围是"不静默丢数据"的唯一凭据。
type SequenceRange struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// Empty 报告范围是否尚未覆盖任何 sequence。
//
// 【为什么把 {0,0} 也算空 —— 这条是踩出来的】sequence 从 1 起分配
// （Append 先自增再取用），所以 0 永远不是合法 sequence，{0,0} 只可能是
// "零值 = 什么都没覆盖"。若按 To<From 判定，零值会被理解成"覆盖了 0..0"，
// Extend 时下界就卡在 0 再也推不上去 —— 实测表现是"压缩 seq 1..6 之后
// 覆盖范围报成 0..6"，而 CheckMemoryIntegrity 会拿这个错下界去比对保留项，
// 完整性判据随之失真。零值语义在 Go 里是契约的一部分，不是实现细节。
func (r SequenceRange) Empty() bool { return r.To == 0 || r.To < r.From }

// Covers 报告某 sequence 是否落在范围内。
// 空范围不覆盖任何东西 —— 包括序号 0（否则空范围会对 seq=0 回答"覆盖了"，
// 而 0 根本不是合法 sequence，调用方据此判断就会得出反向结论）。
func (r SequenceRange) Covers(seq int64) bool {
	return !r.Empty() && seq >= r.From && seq <= r.To
}

// Extend 返回把 seq 纳入后的范围（不缩小已有范围）。
func (r SequenceRange) Extend(seq int64) SequenceRange {
	if r.Empty() {
		return SequenceRange{From: seq, To: seq}
	}
	if seq < r.From {
		r.From = seq
	}
	if seq > r.To {
		r.To = seq
	}
	return r
}

// MemoryItem 一条记忆项。Payload 是**原文**，本层不解释它 ——
// 解释属于 agent 侧（各 provider 的 message 结构各不相同）。
type MemoryItem struct {
	Sequence int64             `json:"sequence"`
	Kind     MemoryKind        `json:"kind"`
	Role     string            `json:"role,omitempty"`
	Payload  []byte            `json:"payload"`
	At       time.Time         `json:"at"`
	Meta     map[string]string `json:"meta,omitempty"`
}

// MemorySnapshot 是一份记忆的完整快照。
type MemorySnapshot struct {
	Key   MemoryKey    `json:"key"`
	Items []MemoryItem `json:"items"`
	Bytes int64        `json:"bytes"`
	// Covered 是已被压缩进摘要、因而不再逐条保留的 sequence 范围。
	// 不变量（由契约测试守住）：
	//   Covered ∪ {每条保留项的 sequence} 必须覆盖历史上出现过的全部 sequence。
	// 任何 sequence 同时不在两者中 = 静默丢数据 = 违约。
	Covered SequenceRange `json:"covered"`
	// Digest 是摘要正文（Kind=MemorySummary 时非空）。压缩产物写在这里，
	// 与"覆盖范围"一起构成可审计的替换关系。
	Digest string `json:"digest,omitempty"`
	// Version 每次写入自增，供"后台刷新读到旧值"这类竞态判定。
	Version int64     `json:"version"`
	At      time.Time `json:"at"`
}

// MemoryLimit 是一次读取的窗口约束。
type MemoryLimit struct {
	MaxItems int   `json:"max_items"`
	MaxBytes int64 `json:"max_bytes"`
	// FromSequence > 0 时只返回该 sequence 之后（不含）的项，用于增量补读。
	FromSequence int64 `json:"from_sequence,omitempty"`
}

// Truncated 报告窗口是否被任一上限截断（调用方据此决定是否需要翻页或压缩）。
func (l MemoryLimit) Truncated(items int, bytes int64) bool {
	if l.MaxItems > 0 && items >= l.MaxItems {
		return true
	}
	return l.MaxBytes > 0 && bytes >= l.MaxBytes
}

// CompactPolicy 是压缩策略（规格 §6.3 SummaryMemory）。
type CompactPolicy struct {
	// KeepRecentItems 必须保留的最近项数（压缩只能动更早的项）。
	// 0 表示"按 TargetBytes 自行决定"。
	KeepRecentItems int `json:"keep_recent_items"`
	// TargetBytes 压缩后的目标字节上限；0 表示不限。
	TargetBytes int64 `json:"target_bytes"`
	// Reason 人类可读的触发原因（"上下文超预算"/"轮次结束归档"/"用户手动压缩"）。
	Reason string `json:"reason,omitempty"`
}

// MemoryManager 是公共内存入口（规格 §6.2）。
//
// 【实现者须知】Append 是唯一写路径；Load/ReadWindow/Compact 是读路径；
// Checkpoint/Restore 负责把状态搬到盘上再搬回来 —— Restore 之后 Load 的结果
// 必须与 Checkpoint 之前一致，这是恢复契约的全部意义。
//
// 【不允许的降级】任何实现都不得在 Compact 里把"没被摘要覆盖"的项直接删掉。
// 预算压力下的正确动作是先压缩成摘要（并在 Covered 里记账）、必要时落盘，
// 而不是静默删除。允许的是"显式声明本 agent 不支持压缩"（写进 Limitations）。
type MemoryManager interface {
	// Load 取一份记忆快照。不存在时返回空快照（不返回 error）。
	Load(ctx context.Context, key MemoryKey) (*MemorySnapshot, error)
	// Append 追加一条记忆项，返回其被分配的 sequence（>=1 单调递增）。
	Append(ctx context.Context, key MemoryKey, item MemoryItem) (int64, error)
	// ReadWindow 按窗口约束读取（由旧到新）。
	ReadWindow(ctx context.Context, key MemoryKey, limit MemoryLimit) ([]MemoryItem, error)
	// Compact 压缩记忆并返回压缩后的快照。
	Compact(ctx context.Context, key MemoryKey, policy CompactPolicy) (*MemorySnapshot, error)
	// Checkpoint 把当前记忆写到持久层。
	Checkpoint(ctx context.Context, key MemoryKey) error
	// Restore 从持久层读回（覆盖内存态）。
	Restore(ctx context.Context, key MemoryKey) error
}
