package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/pocketagent/daemon/internal/store"
)

// ---- 原生会话 ----------------------------------------------------------------

// NativeSessionAdapter 统一"原生会话"的开关与恢复（规格 §6.2）。
//
// 【契约】Source() 是**声明**，必须与本 agent 在 AgentDescriptor.SessionModel
// 里申报的值一致。两者不一致意味着恢复路径在按错误前提决策（例如 daemon 以为
// 可以靠原生 session 恢复，实际那家 agent 根本没有）—— 属于契约违约，
// 由 VerifyDeclaration 检查，不靠人记住。
type NativeSessionAdapter interface {
	// Open 为一个 thread 建立新的原生会话。
	Open(ctx context.Context, thread *store.Thread) error
	// Resume 按 thread 里落库的引用重挂载已有原生会话。
	Resume(ctx context.Context, thread *store.Thread) error
	// Checkpoint 把"恢复所需的引用"写回 thread（由调用方持久化）。
	Checkpoint(ctx context.Context, thread *store.Thread) error
	// Close 释放原生会话资源（不删除会话内容）。
	Close(ctx context.Context, thread *store.Thread) error
	// Source 声明原生会话是不是事实来源。
	Source() SessionSource
}

// ---- checkpoint --------------------------------------------------------------

var (
	// ErrCheckpointKeyIncomplete 表示 CheckpointKey 缺定位维度。
	ErrCheckpointKeyIncomplete = errors.New("runtime: checkpoint key 不完整（需要 agent_id 与 thread_id）")
)

// CheckpointKey 定位一份 checkpoint。
type CheckpointKey struct {
	AgentID  string `json:"agent_id"`
	ThreadID string `json:"thread_id"`
	// Name 区分同一 thread 下的多份 checkpoint（如 "conversation"/"summary"/"session-ref"）。
	Name string `json:"name"`
}

// Validate 检查 key 是否可定位。
func (k CheckpointKey) Validate() error {
	if k.AgentID == "" || k.ThreadID == "" || k.Name == "" {
		return ErrCheckpointKeyIncomplete
	}
	return nil
}

// String 给日志与测试断言用。
func (k CheckpointKey) String() string {
	return fmt.Sprintf("%s/%s/%s", k.AgentID, k.ThreadID, k.Name)
}

// CheckpointManager 把可恢复状态写到持久层并读回来（规格 §6.2 / §8.2）。
//
// 【为什么必须有它】没有 checkpoint 的 agent，daemon 重启后只能"重新开始"，
// 而用户看到的是同一个会话 —— 表现为会话内容莫名缺失。规格 §8.2 要求写入
// 顺序为「临时文件 → flush → fsync → atomic rename → 更新 journal」，
// 这个顺序由实现方保证；本接口只规定**语义**：Save 之后 Load 必须能读回
// 完全相同的内容，Drop 之后再 Load 必须报 not-found。
type CheckpointManager interface {
	// Save 覆盖写入。实现必须保证"要么读到旧内容、要么读到新内容"，
	// 不得出现半截内容（原子性）。
	Save(ctx context.Context, key CheckpointKey, payload []byte) error
	// Load 读回。found=false 表示从未写过（不是错误）。
	Load(ctx context.Context, key CheckpointKey) (payload []byte, found bool, err error)
	// Drop 删除。删除不存在的 checkpoint 视为成功（幂等）。
	Drop(ctx context.Context, key CheckpointKey) error
}

// ---- 恢复来源 ----------------------------------------------------------------

// RecoverySource 说明"这次恢复是从哪儿接回来的"（规格 §8.2 的三分支）。
//
// 【为什么要有 CORRUPTED 这个显式值】规格要求"两者均损坏时进入明确的
// CORRUPTED 状态"，而不是静默按空启动。静默按空启动的后果是用户以为
// 自己的会话/设备消失了 —— 这是规格 §8.2 点名禁止的形态。
type RecoverySource string

const (
	// RecoveredNone 尚未尝试恢复。
	RecoveredNone RecoverySource = "none"
	// RecoveredCheckpoint 从 daemon 快照恢复。
	RecoveredCheckpoint RecoverySource = "checkpoint"
	// RecoveredNative 通过原生 session/resume 恢复。
	RecoveredNative RecoverySource = "native"
	// RecoveredCorrupted 快照与 journal 均不可读 —— 明确的损坏态，等待人工处置。
	RecoveredCorrupted RecoverySource = "corrupted"
)

// RecoveryState 是一次恢复尝试的可诊断记录。
// 【为什么要记它】UI 要显示"最近一次恢复结果"（规格 §12），而 provider 内部的
// 恢复尝试分散在各处、结果无人汇总。把结果收敛成一个结构，才能既给 UI 又给诊断。
type RecoveryState struct {
	// Attempted 是否尝试过恢复。
	Attempted bool `json:"attempted"`
	// Succeeded 是否成功接回。
	Succeeded bool `json:"succeeded"`
	// From 恢复来源。
	From RecoverySource `json:"from"`
	// Reason 失败原因（成功时为空）。
	Reason string `json:"reason,omitempty"`
	// At 最后更新时间。
	At string `json:"at,omitempty"`
}
