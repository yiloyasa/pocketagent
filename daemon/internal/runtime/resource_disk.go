package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// 本文件是 ResourceLedger 的**落盘投影**（D7 收尾 B）。backend.go 的 memLedger
// 仍是唯一权威（本轮禁改它，内存记账语义一个字不动）；本文件只做两件事：
// 重启时把账本装回来、把变化刷出去。落盘惯例与 budget.go / identity.go /
// unknown_events.go 一致：JSON + 原子写（临时文件 → rename）。
//
// 【为什么是"记账投影"而不是权威副本】规格 §6.1 把资源账本归为运行过程数据：
// 丢账的代价是"重启后少看到一段历史用量"，绝不会丢用户数据，也绝不会让语义
// 记忆/会话内容出问题。所以它的取舍与 checkpoint 相反、与 unknown-events 相同：
//   - 双写以**磁盘为准**，但仅指重启装载那一刻 —— 盘上有账就装回来，保证
//     ResourceLease 观测跨重启连续；
//   - **丢账可接受**：损坏或读不出来按空账继续（fail-open），只留告警，
//     绝不因账本问题阻塞轮次或让 daemon 起不来。
//
// 【为什么在 Observe / OpenRuntime 时机刷盘，而不是每次记账后】Acquire/Charge/
// Release 都在 memLedger 内部发生（backend.go，禁改），没有可拦截的变更点；
// 进程退出钩子也不在本轮可改的范围内。所以刷盘周期取**真实流量**：诊断读路径
// （Observe → GetAgentRuntime）与轮次交付路径（openSharedRuntime）顺路同步，
// 指纹未变则跳过写入 —— 等价于"定期刷盘"，且没有空转定时器。

// resourceLedgerFileName 是资源账本的落盘文件名。
const resourceLedgerFileName = "resource-ledger.json"

// ledgerReleasedCap 是账本文件里已释放租约的保留上限。
//
// 【为什么截断已释放的】Release 后的租约在内存里本来就永久保留（memLedger 既有
// 行为），但文件是投影，不该跟着无界增长：长跑进程按轮次释放租约，几天就是几万条。
// 活跃租约一条不丢（观测连续靠的是它们）；已释放的只留最近 N 条供追溯，
// 超出的按"丢账可接受"处理。
const ledgerReleasedCap = 100

// ledgerFile 是账本落盘信封（裸数组会失去扩展位；与 unknownEventsFile 同款理由）。
type ledgerFile struct {
	Leases []ResourceLease `json:"leases"`
}

// ledgerSyncer 是每后端一份的同步状态：装载只做一次；指纹没变不重复写盘。
// 单例的必要性与 diskCacheRegistry 相同（backend.go 禁改，不能加字段）。
type ledgerSyncer struct {
	mu     sync.Mutex
	loaded bool
	// lastFp 是上次落盘内容的指纹。用内容指纹而不是脏标记，是因为账本变更点
	// 不可拦截：脏标记会在"别人改了账、我这边没看见"时漏写，指纹不会。
	lastFp string
}

// ledgerRegistry 是 *Backend -> *ledgerSyncer 的每后端单例注册表。
var ledgerRegistry sync.Map

func (b *Backend) ledgerSyncer() *ledgerSyncer {
	if v, ok := ledgerRegistry.Load(b); ok {
		return v.(*ledgerSyncer)
	}
	s, _ := ledgerRegistry.LoadOrStore(b, &ledgerSyncer{})
	return s.(*ledgerSyncer)
}

// SyncResourceLedger 装载（每进程一次）+ 按变化刷盘。
//
// 返回 error 只反映**本次刷盘**是否成功（调用方决定要不要出声）；装载失败按
// fail-open 处理：告警 + 空账继续，原文件保留供人工查看（与 unknown-events 同款）。
func (b *Backend) SyncResourceLedger() error {
	if b == nil || b.dataDir == "" {
		return nil // 内存模式：无盘可落，历史语义零分叉
	}
	s := b.ledgerSyncer()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		s.loaded = true
		b.loadLedgerLocked(s)
	}
	snap, fp := b.ledgerSnapshot()
	if fp == s.lastFp {
		return nil
	}
	if err := b.writeLedgerFile(snap); err != nil {
		return err
	}
	s.lastFp = fp
	return nil
}

// FlushResourceLedger 无条件刷盘（装载过则带上装载结果）。
// 供测试与将来出现退出钩子时显式调用；语义与 Sync 的刷盘步完全一致。
func (b *Backend) FlushResourceLedger() error {
	if b == nil || b.dataDir == "" {
		return nil
	}
	s := b.ledgerSyncer()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		s.loaded = true
		b.loadLedgerLocked(s)
	}
	snap, fp := b.ledgerSnapshot()
	if err := b.writeLedgerFile(snap); err != nil {
		return err
	}
	s.lastFp = fp
	return nil
}

// loadLedgerLocked 从磁盘装载账本（s.mu 持有；b.mu 在内部分段获取）。
//
// 【为什么只在内存账本为空时装载】与 backend.go hydrate 的"双检"同一条纪律：
// 磁盘是重启那一刻的起点，不是运行中的同步源。内存里已有账（本进程记过任何一笔）
// 时再装，会用旧账覆盖新账 —— 那是把"恢复"用成了"回滚"。
func (b *Backend) loadLedgerLocked(s *ledgerSyncer) {
	path := b.ledgerPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("[resource] WARNING: 资源账本读取失败，按空账继续（原文件保留）: %v", err)
		}
		return
	}
	var f ledgerFile
	if err := json.Unmarshal(raw, &f); err != nil {
		// fail-open：账本是记账性质，损坏就丢弃重建；不删原文件，留一行告警。
		log.Printf("[resource] WARNING: 资源账本损坏（JSON 解析失败），按空账继续（原文件保留在 %s 供人工检查）: %v", path, err)
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.leases) != 0 {
		return // 双检：装载读盘期间本进程已开始记账 —— 内存为新，绝不回滚
	}
	for i := range f.Leases {
		l := f.Leases[i]
		if l.LeaseID == "" {
			continue // 形状不完整的条目跳过而不是猜：猜错会把账记到别人的 id 上
		}
		cp := l
		b.leases[cp.LeaseID] = &cp
	}
}

// ledgerSnapshot 复制一份账本快照并计算内容指纹（b.mu 只在复制时持有）。
func (b *Backend) ledgerSnapshot() (ledgerFile, string) {
	b.mu.Lock()
	out := make([]ResourceLease, 0, len(b.leases))
	for _, l := range b.leases {
		out = append(out, *leaseCopy(l))
	}
	b.mu.Unlock()

	var active []ResourceLease
	var released []ResourceLease
	for _, l := range out {
		if l.ReleasedAt.IsZero() {
			active = append(active, l)
		} else {
			released = append(released, l)
		}
	}
	// 已释放的按释放时间新→旧保留，超出上限的丢弃（取舍见 ledgerReleasedCap）。
	sort.Slice(released, func(i, j int) bool { return released[i].ReleasedAt.After(released[j].ReleasedAt) })
	if len(released) > ledgerReleasedCap {
		released = released[:ledgerReleasedCap]
	}
	file := ledgerFile{Leases: append(active, released...)}
	raw, err := json.Marshal(file)
	if err != nil {
		// ResourceLease 全是可序列化字段，到这里失败只可能是未来加了不可序列化
		// 字段：那必须当场炸在测试里，而不是静默丢整份账。
		panic(fmt.Sprintf("runtime: 资源账本序列化失败: %v", err))
	}
	// 指纹用内容长度 + 末段：同一快照重排出的 JSON 字节相同 → 指纹相同 → 跳过写盘。
	return file, fmt.Sprintf("%d:%x", len(raw), fnv1a(raw))
}

// writeLedgerFile 原子落盘（临时文件 → rename，惯例同 SaveBudgetPolicy）。
// 【为什么不做 fsync】账本是记账投影、丢账可接受；规格 §8.2 的完整 fsync 纪律
// 保留给 checkpoint/身份这类权威状态。每次刷盘都 fsync 的代价买不到对等收益。
func (b *Backend) writeLedgerFile(f ledgerFile) error {
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("runtime: 序列化资源账本失败: %w", err)
	}
	final := b.ledgerPath()
	if err := os.MkdirAll(filepath.Dir(final), 0o700); err != nil {
		return fmt.Errorf("runtime: 建数据目录失败(%s): %w", filepath.Dir(final), err)
	}
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("runtime: 写临时账本文件失败(%s): %w", tmp, err)
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("runtime: 原子替换账本文件失败(%s): %w", final, err)
	}
	return nil
}

func (b *Backend) ledgerPath() string { return filepath.Join(b.dataDir, resourceLedgerFileName) }

// fnv1a 32 位 FNV-1a：指纹只需要"同一内容算出同一位"，不需要抗碰撞。
func fnv1a(b []byte) uint32 {
	var h uint32 = 2166136261
	for _, c := range b {
		h ^= uint32(c)
		h *= 16777619
	}
	return h
}
