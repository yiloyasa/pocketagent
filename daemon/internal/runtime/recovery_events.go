package runtime

import (
	"context"
	"sort"
	"strings"
)

// 本文件是 per-agent recovery 的**产出方**（D7 收尾 C）。
//
// 【要解决的问题】RecoveryState 此前恒为 RecoveredNone（observation.go 的注释
// 如实记录了这一点）：恢复状态挂在 per-thread 的临时 AgentRuntime 上，而恢复是
// 跨进程的事件，没有任何东西往里写 —— GetAgentRuntime 的 recovery 块永远空白，
// 客户端分不清"没这功能"和"没恢复过"。
//
// 【最小诚实语义】只报告**本进程内发生过、且有证据**的恢复动作。证据取自两处
// 真实状态，不新增任何记账层：
//   1. b.hydrated：backend.go 的 hydrate 在"首次触碰某条 memory key"时置位，
//      它是每进程每 key 一次的 —— 置位即"本进程咨询过持久层"；
//   2. b.Checkpoints().Load：盘上真的有该 key 的 memory checkpoint。
// 两者同时成立 = "本进程从磁盘装载过该 agent 的记忆状态" —— 这就是规格说的
// 重启恢复，一个字都不用编。
//
// 【为什么是推导而不是在 hydrate 里埋钩子】hydrate 在 backend.go 里，本轮禁改
// （另一条工作流刚收口）。推导的判据完全来自上述进程内事实，误差窗口只有一个：
// hydrate 探盘成功的同时另一 goroutine 抢先写入新状态（双检让装载退让）——此时
// "装载"其实没发生，但进程内证据无法区分"装回了旧账"和"写入了新账"。接受这个
// 小概率的过度上报，因为方向是安全的：它声称"恢复过"而实际"写入过"，两者都是
// "状态来自本进程的一次成功交接"，没有编造失败或损坏。
//
// 【②断线重连为什么只有词表没有产出方】provider 客户端的重连（如 pi.go 的
// replayMissed）分散在各自内部，没有可从 runtime/观测侧接入的现成钩子，而
// provider 文件本轮禁改。按 unknown_events.go 的 UNKNOWN_RESPONSE 同一先例：
// 词表留位 + 注释声明"当前没有代码产生它"，绝不硬造一条"重连成功"。

const (
	// KindRecoveryHydrate 表示"checkpoint/memory 从磁盘 hydrate 装载（重启恢复）"。
	KindRecoveryHydrate = "hydrate"
	// KindRecoveryReconnect 是"provider 客户端断线重连成功"的词表预留档。
	// **当前无产出方**（原因见文件头）：在 UI 上看到这个类别之前，别以为
	// 真有这类事件 —— 它说明词表被人手工填过。
	KindRecoveryReconnect = "reconnect"
)

// RecoveryEvent 是一条按 agent 归属的恢复事件（诊断导出与 per-thread 明细用）。
//
// 【刻意没有 At 字段】事件由证据推导而来：hydrated 标记只记"发生过"，不记时刻。
// 填一个"推导时刻"去冒充"发生时刻"，比留空更不诚实。
type RecoveryEvent struct {
	Kind     string `json:"kind"`
	AgentID  string `json:"agent_id"`
	ThreadID string `json:"thread_id,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// recoveryProbe 是一条待验证的恢复证据：某条 memory key 本进程已咨询过持久层，
// 且内存里确实持有数据（present）。
type recoveryProbe struct {
	key     MemoryKey
	present bool
}

// RecoveryEvents 返回该 agent 在本进程内发生过、且有磁盘证据的恢复事件。
//
// 【为什么按 agent 隔离是底线】recovery 块与 memory/cache/checkpoint 一样是
// 诊断页的展示口径：串了家，用户会看到"codex 恢复过"而那其实是 echo 的事件。
func (b *Backend) RecoveryEvents(agentID string) []RecoveryEvent {
	// 载体必须是文件后端：内存后端的 hydrated 只证明"同进程内咨询过内存 map"，
	// 那不是恢复，报出来就是编造（有 dataDir 才存在跨进程这回事）。
	if b == nil || b.dataDir == "" {
		return nil
	}
	probes := b.hydratedMemoryKeysOf(agentID)
	if len(probes) == 0 {
		return nil
	}
	ctx := context.Background()
	var out []RecoveryEvent
	for _, p := range probes {
		if !p.present {
			// hydrate 跑过但内存为空：要么盘上本就无 checkpoint，要么装载被并发
			// 写入退让 —— 都不构成"装回了数据"。
			continue
		}
		_, found, err := b.Checkpoints().Load(ctx, memoryCheckpointKey(p.key))
		if err != nil || !found {
			continue // 读失败按"没有证据"处理：推导路径绝不因自身故障报假事件
		}
		out = append(out, RecoveryEvent{
			Kind:     KindRecoveryHydrate,
			AgentID:  p.key.AgentID,
			ThreadID: p.key.ThreadID,
			Detail:   "memory checkpoint 从磁盘装载（kind=" + string(p.key.Kind) + "）",
		})
	}
	return out
}

// recoveryStateFor 把证据折算成既有的 RecoveryState 形状（observation.go 消费）。
//
// At 刻意留空：推导不出发生时刻，宁缺毋造（字段含义见 RecoveryState 的注释）。
func (b *Backend) recoveryStateFor(agentID string) RecoveryState {
	if len(b.RecoveryEvents(agentID)) == 0 {
		return RecoveryState{From: RecoveredNone}
	}
	return RecoveryState{
		Attempted: true,
		Succeeded: true,
		From:      RecoveredCheckpoint,
	}
}

// hydratedMemoryKeysOf 收集该 agent 名下"本进程已 hydrate 过"的 memory key
// （b.mu 只在读取时持有；磁盘探测在 RecoveryEvents 里锁外做，与 checkpoint
// 观测的同款纪律）。
//
// key 字符串的解析与 observeMemoryLocked 同源（agent/thread/kind[#scope]）——
// 两处共用同一形状约定，谁改了 MemoryKey.String() 谁就会同时碰坏这两个读者。
func (b *Backend) hydratedMemoryKeysOf(agentID string) []recoveryProbe {
	if b == nil || agentID == "" || b.dataDir == "" {
		return nil
	}
	b.mu.Lock()
	prefix := agentID + "/"
	var out []recoveryProbe
	for ks := range b.hydrated {
		if !strings.HasPrefix(ks, prefix) {
			continue
		}
		key, ok := parseMemoryKeyString(ks)
		if !ok {
			continue // 形状意外的 key 跳过而不是猜（与 observeMemoryLocked 同款理由）
		}
		out = append(out, recoveryProbe{key: key, present: !b.memoryAbsentLocked(key)})
	}
	b.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].key.String() < out[j].key.String() })
	return out
}

// parseMemoryKeyString 还原 MemoryKey.String() 的产出（含可选 Scope）。
func parseMemoryKeyString(s string) (MemoryKey, bool) {
	parts := strings.SplitN(s, "/", 3)
	if len(parts) != 3 {
		return MemoryKey{}, false
	}
	kind, scope := parts[2], ""
	if i := strings.IndexByte(kind, '#'); i >= 0 {
		scope, kind = kind[i+1:], kind[:i]
	}
	key := MemoryKey{AgentID: parts[0], ThreadID: parts[1], Kind: MemoryKind(kind), Scope: scope}
	if key.Validate() != nil {
		return MemoryKey{}, false
	}
	return key, true
}
