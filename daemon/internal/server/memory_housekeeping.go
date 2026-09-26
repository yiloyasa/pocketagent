// memory_housekeeping.go —— P1「轮末自动收敛」（memory 分层设计稿 §4 P1）。
//
// 【它补的是什么缺口】Backend.Compact（裁剪 + Covered 记账 + 结构摘要拼接）此前只活在
// 契约测试里——全仓没有任何生产调用点，"主动收敛没有执行点"。本文件把它挂进轮次收尾
// 路径：runTurnOnce 的两处 assistant 落库点（thread.go）的下游、终局帧与主机事件交付
// 之后，收敛从此随每一轮自动发生。零模型调用：摘要一律用 runtime 包的结构摘要
// （structuralDigest），语义摘要归 P2 的可插拔 Summarizer。
//
// 【为什么摘要行还要登记进 summary 槽】Compact 只把摘要写进原槽快照的 Digest 字段，
// 而诊断页的 kinds["summary"] 数的是 summary 槽里的**条目**——不登记，这个计数恒为 0，
// 设计稿 P1 的"手机端零改动可见"就是空话。摘要行带覆盖范围（covered_from/to）与来源槽，
// 与轮次索引条目同款自我描述纪律（Meta["entry"]）。
//
// 【失败不阻断轮次】与 recordTurnMemory 同一条纪律：收敛是加法运维，任何失败只记日志，
// 绝不让用户为它多等一帧或丢一轮回复。
package server

import (
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/pocketagent/daemon/internal/agent"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// 轮末收敛的 Compact 策略（设计稿 §4 P1 的初始定值）。
const (
	// compactKeepRecentItems 收敛后必须保留的最近条数——最近上下文是唯一还有
	// 活跃读者的部分（P2 的 daemon 家上下文组装），压它等于把"最近说过什么"压没。
	compactKeepRecentItems = 50
	// compactTargetBytes 收敛后的目标字节上限；Compact 会在不违反保留条数的前提下
	// 继续往前扩，把保留项压到这个量级以内。
	compactTargetBytes = int64(1) << 20
	// compactReason 触发原因，进 Compact 的可审计链路（Reason 字段）。
	compactReason = "轮末收敛"
)

// 摘要条目的 Meta 键（与 turn_memory.go 的索引条目共用 metaEntryKind=entry 的自我描述约定）。
const (
	entryCompactDigest = "compact_digest" // metaEntryKind 的取值：轮末收敛摘要行
	metaCoveredFrom    = "covered_from"   // 本行覆盖的 sequence 下界（闭区间）
	metaCoveredTo      = "covered_to"     // 本行覆盖的 sequence 上界（闭区间）
	metaSourceKind     = "source_kind"    // 摘要来自哪个槽（conversation / artifact）
	metaCompactReason  = "reason"         // 触发原因（与 CompactPolicy.Reason 同值）
)

// memorySlotThreshold 是一个记忆槽的收敛水位（条数与字节任一越过即触发）。
type memorySlotThreshold struct {
	MaxItems int
	MaxBytes int64 // 0 = 不看字节
}

// memorySlotThresholds 是 P1 的初始阈值表（设计稿 §4 P1：conversation 条数 > 200 或
// 字节 > 2MB；artifact 索引条数 > 400；量级对齐规格 §6.5 示例 32MB/256MB 的保守零头）。
//
// 【待校准】规格 §6.5 自认这是"上线前要压力测试校准"的基线——这里的数字同样只是
// "有水位"，不是"水位是对的"。P1 先以变量形式放在包级：后续接配置覆盖时改这里，
// 轮次路径不必再动。
var memorySlotThresholds = map[runtime.MemoryKind]memorySlotThreshold{
	runtime.MemoryConversation: {MaxItems: 200, MaxBytes: 2 << 20},
	runtime.MemoryArtifact:     {MaxItems: 400},
	// summary 槽是轮末收敛自己造出来的（P1 之前无人写），设计稿的表里因此没有它。
	// 给一个与索引槽同级的条数上限兜底：摘要行一行对应一次收敛，正常永远到不了，
	// 但"摘要行本身无限累积"这条后路必须堵上——否则收敛动作自己成了新的泄漏源。
	runtime.MemorySummary: {MaxItems: 400},
}

// afterTurnMemoryHousekeeping 轮次收尾的记忆整理：对本轮真正写过的槽做一次水位检查，
// 越线（槽阈值或 ResourceLedger 软/硬压力）即 Compact + Checkpoint。
//
// 【按申报的形态分槽，与写入链同一判据】daemon 家写 conversation 槽、hybrid 家写
// artifact 索引槽（turn_memory.go 的分档判据）；native 家与未申报者什么都不写，
// 也就没有可收敛的东西——跑一遍只浪费两次 Load，直接跳过。
//
// 【summary 槽两种家都收】它装的是收敛自己产出的摘要行，来源槽是谁不重要。
func (s *Server) afterTurnMemoryHousekeeping(provider agent.Provider, t *store.Thread) {
	if s == nil || s.runtimeBackend == nil || provider == nil || t == nil {
		return
	}
	switch memoryModelOf(provider) {
	case runtime.MemoryDaemon:
		s.convergeMemorySlot(provider, t, runtime.MemoryConversation)
	case runtime.MemoryHybrid:
		s.convergeMemorySlot(provider, t, runtime.MemoryArtifact)
	default:
		// memory.native（正文在原生侧）与未申报（不知道它有没有记忆）：没写过就无可收敛。
		return
	}
	s.convergeMemorySlot(provider, t, runtime.MemorySummary)
}

// convergeMemorySlot 对一个记忆槽做"水位检查 → Compact → Checkpoint"。
//
// 【幂等闸门】Compact 之后 Covered/Digest 不再变化时（保留项已少于 KeepRecentItems，
// 再压只能压空气槽），后续触发是纯空转——在"变了才落盘、才登记摘要行"这里截住，
// 保证同一状态反复触发零副作用（预算软压在 P1 是累计记账，收敛后仍会报压，
// 这个闸门是"软压常驻"与"每轮白干"之间的那道墙）。
func (s *Server) convergeMemorySlot(provider agent.Provider, t *store.Thread, kind runtime.MemoryKind) {
	agentID := provider.ID()
	key := runtime.MemoryKey{AgentID: agentID, ThreadID: t.ID, Kind: kind}
	ctx := context.Background()
	before, err := s.runtimeBackend.Memory().Load(ctx, key)
	if err != nil {
		log.Printf("[memory] %s 轮末收敛读水位失败: thread=%s kind=%s err=%v", agentID, t.ID, kind, err)
		return
	}
	th := memorySlotThresholds[kind]
	overThreshold := len(before.Items) > th.MaxItems || (th.MaxBytes > 0 && before.Bytes > th.MaxBytes)
	if !overThreshold && !s.memoryBudgetUnderPressure(agentID, t, kind) {
		return
	}
	after, err := s.runtimeBackend.Memory().Compact(ctx, key, runtime.CompactPolicy{
		KeepRecentItems: compactKeepRecentItems,
		TargetBytes:     compactTargetBytes,
		Reason:          compactReason,
	})
	if err != nil {
		log.Printf("[memory] %s 轮末收敛失败（不影响轮次）: thread=%s kind=%s err=%v", agentID, t.ID, kind, err)
		return
	}
	if after.Covered == before.Covered && after.Digest == before.Digest {
		return // 幂等闸门：没有可压的项，后续动作（摘要行/落盘）全部跳过
	}
	log.Printf("[memory] %s 轮末收敛 %s 槽: 条目 %d→%d 字节 %d→%d covered=%d..%d",
		agentID, kind, len(before.Items), len(after.Items), before.Bytes, after.Bytes,
		after.Covered.From, after.Covered.To)
	if kind != runtime.MemorySummary {
		// 摘要行只对"业务槽"的收敛登记；summary 槽自己的收敛若再登记摘要的摘要，
		// 就成了没有底的自指（它的量由上面的阈值表兜底）。
		s.appendTurnDigestSummary(provider, t, kind, before.Digest, after)
	}
	// 收敛只动内存态，必须紧随一次 Checkpoint 才算落定（与 recordTurnMemory 轮末
	// 落盘同款：失败只记日志——下一次轮末会再试，脏状态不跨进程存活）。
	if err := s.runtimeBackend.Memory().Checkpoint(ctx, key); err != nil {
		log.Printf("[memory] %s 收敛后 checkpoint 失败: thread=%s kind=%s err=%v", agentID, t.ID, kind, err)
	}
}

// appendTurnDigestSummary 把本次收敛新增的结构摘要行登记进 summary 槽。
//
// 【只登记新增段，不登记全量】Compact 把新段拼在旧 Digest 之后；若每次都把全量
// Digest 抄进 summary 槽，收敛 N 次就是 O(N²) 的重复正文。只取新增段，摘要行才与
// "发生了几次收敛"一一对应。
func (s *Server) appendTurnDigestSummary(provider agent.Provider, t *store.Thread, source runtime.MemoryKind, prevDigest string, after *runtime.MemorySnapshot) {
	segment := after.Digest
	if prevDigest != "" && strings.HasPrefix(after.Digest, prevDigest) {
		segment = strings.TrimPrefix(strings.TrimPrefix(after.Digest, prevDigest), "\n")
	}
	if segment == "" {
		return
	}
	key := runtime.MemoryKey{AgentID: provider.ID(), ThreadID: t.ID, Kind: runtime.MemorySummary}
	item := runtime.MemoryItem{
		Kind:    runtime.MemorySummary,
		Role:    "digest",
		Payload: []byte(segment),
		Meta: map[string]string{
			metaEntryKind:     entryCompactDigest,
			metaCoveredFrom:   strconv.FormatInt(after.Covered.From, 10),
			metaCoveredTo:     strconv.FormatInt(after.Covered.To, 10),
			metaSourceKind:    string(source),
			metaCompactReason: compactReason,
		},
	}
	ctx := context.Background()
	if _, err := s.runtimeBackend.Memory().Append(ctx, key, item); err != nil {
		log.Printf("[memory] %s 摘要行登记失败（不影响轮次）: thread=%s source=%s err=%v",
			provider.ID(), t.ID, source, err)
		return
	}
	if err := s.runtimeBackend.Memory().Checkpoint(ctx, key); err != nil {
		log.Printf("[memory] %s 摘要槽 checkpoint 失败: thread=%s err=%v", provider.ID(), t.ID, err)
	}
}

// memoryBudgetUnderPressure 报告该会话某槽的记忆租约是否处于软/硬压力
// （压力非 normal ⇔ Usage 报告的 OverSoft/OverHard 名单里有这条租约）。
//
// 【设计稿的"Usage 返回 OverSoft"在此落地】软/硬限分离（见 memoryBudgetAdmit 的
// 额度映射）保证了首次越线先呈现为软压；越过硬限同样是"该收敛了"——拒写已由
// 写入门卫（memoryBudgetAdmit）执行，这里补上收敛动作，两条防线各管各的。
func (s *Server) memoryBudgetUnderPressure(agentID string, t *store.Thread, kind runtime.MemoryKind) bool {
	if s.runtimeBackend == nil || t == nil {
		return false
	}
	rep := s.runtimeBackend.Resources().Usage(context.Background(), runtime.ScopeThread(t.ID))
	id := memoryLeaseID(agentID, t.ID, kind)
	for _, it := range rep.Items {
		if it.LeaseID == id {
			return it.Pressure() != runtime.PressureNormal
		}
	}
	return false // 从未记过账 = 无从谈压力
}
