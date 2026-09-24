package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 本文件是「按会话的记忆清单与清除」——内存管理页面（手机端）的数据源。
//
// 【要解决的问题】Observe 只能给出 per-agent 的合计（threads/items/bytes/kinds），
// 回答不了"哪几个会话占了记忆、各占多少、能不能只清掉某一个"。而"让某个走歪了的
// 会话的记忆重新开始"恰恰是用户最需要的管理粒度 —— 清空整家 agent 等于伤及无辜。
//
// 【数据源取舍：内存 keys 遍历 + 盘上载荷解码，不用 sidecar 索引】
//
//   - 文件名反查不可行：checkpoint 文件名 = checkpointFileComponent(各字段)+哈希后缀，
//     组件被截断（48 字节）且非法字符被替换成 '_'，反推不出真实 thread_id；
//   - 但 memory checkpoint 的**载荷是 JSON**（memoryCheckpoint{Snapshot{Key},Seq}），
//     信封可解码（decodeCheckpoint）—— 枚举目录、解载荷即得精确的 MemoryKey 连同
//     条目/字节/序号。已装载的 key 内存态至少与盘上一样新（每轮 Checkpoint 落盘），
//     用内存值；未装载的（重启后从未触碰）用盘上载荷值，Loaded=false 如实标注。
//   - 不选 sidecar 索引的理由：索引是"同一事实的第二份副本"，谁负责写、崩溃窗口、
//     与文件失真时信谁 —— 三个问题都要答案；而载荷本身就在盘上，读它永远不撒谎。
//     代价是每次清单都要解一遍目录内的文件（规模 = checkpoint 数，单文件 KB 级），
//     清单是用户手动打开的低频操作，付得起。
//
// 【清除绝不触碰原生 agent 的用户数据】ClearThreadMemory 只动 PocketAgent 自己的
// 记忆槽与 memory checkpoint 文件（都在 <dataDir>/checkpoints/ 本实现自己的命名
// 空间里）。~/.codex、~/.pi 等原生侧数据从头到尾不进本文件的任何路径。
//
// 【锁纪律与既有读路径一致】内存态在 b.mu 下取；盘上目录扫描在锁外做（它只读
// 自己目录里的文件，不碰 Backend 状态）；文件删除走 Checkpoints().Drop —— 两种
// 后端（文件/内存）各自实现好了幂等删除，这里不另起一套"直接 os.Remove"的第二路径。

// ThreadMemoryRow 是清单里一个会话的记忆合计。
type ThreadMemoryRow struct {
	ThreadID string
	Entries  int
	Bytes    int64
	// Seq 是该会话全部 kind 槽里最高的 sequence 高水位（每轮 Append 单调递增）。
	Seq int64
	// LastCheckpoint 是最后一次落盘时刻（取 checkpoint 文件的 mtime）。
	// 零值 = 该会话还没有任何槽落过盘（比如刚写入还没到轮末 checkpoint 点）。
	LastCheckpoint time.Time
	// Loaded 标记本进程是否已把该会话装载进内存。false = 重启后从未触碰，
	// 行值来自盘上 checkpoint 载荷 —— 两个口径分开报，用户才不会被
	// "看起来都一样"骗过：未装载的行值是落盘那一刻的快照，可能比实时旧。
	Loaded bool
}

// ThreadMemoryTotals 是一个 agent 名下记忆的合计。
type ThreadMemoryTotals struct {
	Threads int
	Entries int
	Bytes   int64
}

// ListThreadMemories 列出某 agent 名下按会话聚合的记忆清单（按字节降序；
// 同字节按 thread_id 字典序 —— map 遍历无序，不钉死次序同一份数据两次刷新会换位）。
// 返回值永不报错：目录读不了按"盘上没有"处理，与 checkpoint 观测同一口径。
func (b *Backend) ListThreadMemories(agentID string) ([]ThreadMemoryRow, ThreadMemoryTotals) {
	var totals ThreadMemoryTotals
	if b == nil || agentID == "" {
		return nil, totals
	}
	type acc struct {
		entries int
		bytes   int64
		seq     int64
		last    time.Time
		loaded  bool
		hasRow  bool // 有任何证据（内存态/盘上文件）才给行，全零行是噪音
	}
	threads := map[string]*acc{}
	touch := func(threadID string) *acc {
		a := threads[threadID]
		if a == nil {
			a = &acc{}
			threads[threadID] = a
		}
		return a
	}

	// ---- 内存侧：已装载（或本进程写过）的 key，实时值 ----
	b.mu.Lock()
	prefix := agentID + "/"
	for ks, items := range b.items {
		if !strings.HasPrefix(ks, prefix) {
			continue
		}
		key, ok := parseMemoryKeyString(ks)
		if !ok {
			continue // 形状意外的 key 跳过而不是猜归属（与 observeMemoryLocked 同款理由）
		}
		var bytes int64
		for i := range items {
			bytes += int64(len(items[i].Payload))
		}
		a := touch(key.ThreadID)
		a.hasRow = true
		a.loaded = a.loaded || b.hydrated[ks]
		a.entries += len(items)
		a.bytes += bytes
		if b.seq[ks] > a.seq {
			a.seq = b.seq[ks]
		}
	}
	b.mu.Unlock()

	// ---- 盘侧：给每个盘上记录补充"最后落盘时刻"；未装载的 key 连数值一起取
	// （锁外做文件 IO）。已装载的 key 数值仍用内存态（至少与盘上一样新，混计就是
	// 双份），但文件 mtime 照取 —— "最后一次 checkpoint 落盘"的时刻就定义在文件上，
	// 内存态里没有这个事实。
	for _, d := range b.scanMemoryCheckpoints(func(k MemoryKey) bool { return k.AgentID == agentID }) {
		b.mu.Lock()
		loaded := b.hydrated[d.key.String()]
		b.mu.Unlock()
		a := touch(d.key.ThreadID)
		a.hasRow = true
		if d.mtime.After(a.last) {
			a.last = d.mtime
		}
		if loaded {
			a.loaded = true
			continue
		}
		a.entries += len(d.cp.Snapshot.Items)
		a.bytes += d.cp.Snapshot.Bytes
		if d.cp.Seq > a.seq {
			a.seq = d.cp.Seq
		}
	}

	rows := make([]ThreadMemoryRow, 0, len(threads))
	for tid, a := range threads {
		if !a.hasRow || (a.entries == 0 && a.bytes == 0 && a.seq == 0 && a.last.IsZero()) {
			continue // 全零行既没东西可看也没东西可清，列出来只会让人怀疑出了故障
		}
		rows = append(rows, ThreadMemoryRow{
			ThreadID:       tid,
			Entries:        a.entries,
			Bytes:          a.bytes,
			Seq:            a.seq,
			LastCheckpoint: a.last,
			Loaded:         a.loaded,
		})
		totals.Threads++
		totals.Entries += a.entries
		totals.Bytes += a.bytes
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Bytes != rows[j].Bytes {
			return rows[i].Bytes > rows[j].Bytes
		}
		return rows[i].ThreadID < rows[j].ThreadID
	})
	return rows, totals
}

// ClearThreadMemory 清掉一个会话名下的全部 PocketAgent 侧记忆（所有 kind 槽的
// 内存态 + 对应的 memory checkpoint 持久层），返回清除后该 agent 的剩余合计。
//
// 返回 found=false 表示该会话名下既没有内存态也没有持久层记录（"没有可清的"），
// 由调用方决定怎么向用户表达 —— 本层不把它当错误，因为"重复点清除"必须幂等成功。
//
// 【并发窗口，如实交代】清内存与删文件之间，正在跑的轮次可能又 Append/Checkpoint
// 了新记忆：那份"清除之后的新记忆"按"重新开始"语义保留，不算清除失败。反过来
// （先删文件、清内存前轮次恰好落盘）留下的文件会在下一轮 Checkpoint 被覆盖，
// 内存态仍是清空后的 —— 两个窗口都不产生"清了还在"的假象，只是新记忆得以存活。
//
// 【高水位一并清】b.seq 也删掉：清除的语义是"这个会话的记忆重新开始"，
// 序号从 1 重排 —— 保留旧高水位的话，重启后的 Append 会从旧序号接着走，
// 与"记忆已经清空"的可见事实对不上。
func (b *Backend) ClearThreadMemory(agentID, threadID string) (ThreadMemoryTotals, bool, error) {
	var totals ThreadMemoryTotals
	if agentID == "" || threadID == "" {
		return totals, false, ErrMemoryKeyIncomplete
	}
	// 持久层侧先扫（扫描函数自己管锁，绝不能在 b.mu 持有时调它 —— RWMutex
	// 不可重入，持锁再进就是自死锁，实测就是这么卡死的）：文件后端扫目录，
	// 内存后端扫 durable map（它就是后者的"盘"）。
	var disk []memoryCheckpointRecord
	if b.dataDir != "" {
		disk = b.scanMemoryCheckpoints(func(k MemoryKey) bool {
			return k.AgentID == agentID && k.ThreadID == threadID
		})
	} else {
		disk = b.scanDurableMemoryCheckpoints(func(k MemoryKey) bool {
			return k.AgentID == agentID && k.ThreadID == threadID
		})
	}
	// 内存侧：收齐该会话全部 key（items 是写路径必经，但 hydrated/seq 可能单独
	// 存在 —— 空会话也可能留下"探过盘"的痕迹，一并清才叫清干净）。
	prefix := agentID + "/" + threadID + "/"
	b.mu.Lock()
	keySet := map[string]MemoryKey{}
	for ks := range b.items {
		if strings.HasPrefix(ks, prefix) {
			if k, ok := parseMemoryKeyString(ks); ok {
				keySet[ks] = k
			}
		}
	}
	for ks := range b.hydrated {
		if strings.HasPrefix(ks, prefix) {
			if k, ok := parseMemoryKeyString(ks); ok {
				keySet[ks] = k
			}
		}
	}
	for ks := range b.seq {
		if strings.HasPrefix(ks, prefix) {
			if k, ok := parseMemoryKeyString(ks); ok {
				keySet[ks] = k
			}
		}
	}
	found := len(keySet) > 0 || len(disk) > 0
	for ks := range keySet {
		b.clearMemoryKeyLocked(ks)
	}
	b.mu.Unlock()

	if !found {
		return totals, false, nil
	}
	// 删持久层在锁外：文件 IO 不该持 b.mu；Drop 自己会取 ckptMu（文件后端）或
	// b.mu（内存后端），持着 b.mu 调它就是自死锁。幂等：文件没了再删也是成功。
	ctx := context.Background()
	for _, d := range disk {
		if err := b.Checkpoints().Drop(ctx, d.ck); err != nil {
			return totals, true, err
		}
	}
	_, totals = b.ListThreadMemories(agentID)
	return totals, true, nil
}

// clearMemoryKeyLocked 删掉一条 key 的全部内存态（b.mu 持有时调用）。
// 六张 map 一张不落：漏任何一张，该 key 就以"半清除"状态复活（如 hydrated 漏了
// → 下次操作重新探盘 → 装回已删文件的旧账；seq 漏了 → 序号接着旧高水位走）。
func (b *Backend) clearMemoryKeyLocked(ks string) {
	delete(b.items, ks)
	delete(b.covered, ks)
	delete(b.digest, ks)
	delete(b.version, ks)
	delete(b.seq, ks)
	delete(b.hydrated, ks)
}

// memoryCheckpointRecord 是从持久层解出的一份 memory checkpoint 的线索：
// key 是记忆键，ck 是持久层键（删除用），cp 是载荷，mtime 是文件修改时刻
// （内存后端无文件，mtime 为零值 —— 清单本来也不对内存后端报时刻）。
type memoryCheckpointRecord struct {
	key   MemoryKey
	ck    CheckpointKey
	cp    *memoryCheckpoint
	mtime time.Time
}

// decodeMemoryCheckpointRaw 从持久层的原始字节解出 memory checkpoint 载荷。
//
// 【为什么要兼容两种格式】两种后端的存储格式历史上就不对称：文件后端的文件带
// 信封（magic+长度+摘要，见 encodeCheckpoint）；内存后端的 durable map 存的是
// **裸 JSON**（见 memCheckpoints.Save —— 它按原样存调用方给的 payload）。本文件
// 不改动既有存储格式（那会让旧数据读不出），只在读取侧两种都认：先试信封，
// 解不开就按裸 JSON 试。两种都解不开 = 不是 memory checkpoint，跳过。
func decodeMemoryCheckpointRaw(raw []byte) (*memoryCheckpoint, bool) {
	payload := raw
	if p, ok := decodeCheckpoint(raw); ok {
		payload = p
	}
	var mc memoryCheckpoint
	if json.Unmarshal(payload, &mc) != nil || mc.Snapshot == nil || mc.Snapshot.Key.Validate() != nil {
		return nil, false
	}
	return &mc, true
}

// scanMemoryCheckpoints 枚举文件后端 checkpoint 目录里"属于本后端的 memory
// checkpoint"，按 want 过滤。解不开信封/不是 memory 载荷的文件跳过：
// 清单与清除都不做损坏裁决 —— 损坏文件的处置是规格 §8.2 的恢复分支，不是这里。
func (b *Backend) scanMemoryCheckpoints(want func(MemoryKey) bool) []memoryCheckpointRecord {
	if b == nil || b.dataDir == "" {
		return nil
	}
	dir := checkpointsDir(b.dataDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // 目录不存在 = 还没写过 checkpoint，不是错误
	}
	var out []memoryCheckpointRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ckpt") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		mc, ok := decodeMemoryCheckpointRaw(raw)
		if !ok {
			continue
		}
		if want != nil && !want(mc.Snapshot.Key) {
			continue
		}
		rec := memoryCheckpointRecord{
			key: mc.Snapshot.Key,
			ck:  memoryCheckpointKey(mc.Snapshot.Key),
			cp:  mc,
		}
		if info, ierr := e.Info(); ierr == nil {
			rec.mtime = info.ModTime()
		}
		out = append(out, rec)
	}
	return out
}

// scanDurableMemoryCheckpoints 内存后端的同款枚举：b.durable 就是它的"盘"。
// 不扫的话，"清除"只删了内存态、durable 里的旧账还在 —— 下一次 hydrate 探盘
// 就把刚清掉的记忆原样装回，清除等于没清。
func (b *Backend) scanDurableMemoryCheckpoints(want func(MemoryKey) bool) []memoryCheckpointRecord {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []memoryCheckpointRecord
	for _, raw := range b.durable {
		mc, ok := decodeMemoryCheckpointRaw(raw)
		if !ok {
			continue
		}
		if want != nil && !want(mc.Snapshot.Key) {
			continue
		}
		out = append(out, memoryCheckpointRecord{
			key: mc.Snapshot.Key,
			ck:  memoryCheckpointKey(mc.Snapshot.Key),
			cp:  mc,
		})
	}
	return out
}
