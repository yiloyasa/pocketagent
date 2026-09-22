package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/store"
)

// ContractSuite 是"所有 agent 都必须通过"的同一套运行时契约用例（规格 §15 L1）。
//
// 【为什么放在非 _test.go 文件里】规格要求"所有 agent 都必须通过同一套 runtime
// contract"。如果把断言写死在 runtime 包的 _test.go 里，那就只能覆盖 runtime 自己
// —— 而真正要覆盖的是每家 provider 通过 OpenRuntime 交付的东西。测试助手要跨包
// 复用就必须在正式代码里（与 net/http/httptest 同款取舍）。
//
// 【它测什么、不测什么】它只测**契约语义**：记忆不静默丢、缓存会过期会淘汰、
// 资源超限会拒、重启后能恢复。它不测各 agent 的原生协议行为 —— 那些各有各的
// 测试，放在 provider 自己的包里。
//
// 用法（provider 侧）：
//
//	runtime.ContractSuite{T: t, AgentID: "echo", RT: rt, Backend: backend, Thread: th}.RunAll()
type ContractSuite struct {
	T       *testing.T
	AgentID string
	RT      *AgentRuntime
	// Backend 是 RT 的底层默认后端。TTL/淘汰/重启这几类检查需要直接操作时钟与
	// 持久态，所以必须拿到它 —— 传 nil 会在 RunAll 开头直接失败，而不是静默跳过
	// 那几项检查（静默跳过等于假绿）。
	Backend *Backend
	Thread  *store.Thread
}

// Precondition 检查套件前条件。
//
// 【为什么返回 error 而不是直接 Fatal】要让"前条件缺失时套件必须拒绝执行"
// 这件事本身可被测试 —— 否则事后无法区分"检查通过了"和"检查被静默跳过了"。
// 静默跳过正是本项目 AUDIT 里"假绿"的定义。
func (s ContractSuite) Precondition() error {
	if s.T == nil {
		return fmt.Errorf("runtime: ContractSuite.T 为空")
	}
	if s.RT == nil {
		return fmt.Errorf("契约套件前条件不满足：RT 为空（provider 未交付运行时）")
	}
	if s.Backend == nil {
		return fmt.Errorf("契约套件前条件不满足：Backend 为空（TTL/淘汰/重启三类检查无法执行）")
	}
	if s.Thread == nil {
		return fmt.Errorf("契约套件前条件不满足：Thread 为空")
	}
	if s.AgentID == "" {
		return fmt.Errorf("契约套件前条件不满足：AgentID 为空")
	}
	return nil
}

// RunAll 跑完整套用例。任何一项不达标都会让调用方测试失败。
func (s ContractSuite) RunAll() {
	s.T.Helper()
	if err := s.Precondition(); err != nil {
		s.T.Fatal(err)
	}

	s.DeclarationIsSelfConsistent()
	s.MemoryAppend()
	s.MemoryLoadAndWindow()
	s.MemoryRejectsIncompleteKey()
	s.MemoryCompaction()
	s.MemoryCheckpointAndRestore()
	s.MemorySurvivesRestartOnlyViaCheckpoint()
	s.CacheHit()
	s.CacheMiss()
	s.CacheTTL()
	s.CacheEviction()
	s.CacheStaleDegrade()
	s.CacheNamespacesAreIsolated()
	s.ResourceLimit()
	s.ResourceAccounting()
	s.SessionSourceMatchesDescriptor()
	s.SessionResumeAfterRestart()
	s.CheckpointSaveLoadDrop()
}

func (s ContractSuite) ctx() context.Context { return context.Background() }

func (s ContractSuite) key(kind MemoryKind, scope string) MemoryKey {
	return MemoryKey{AgentID: s.AgentID, ThreadID: s.Thread.ID, Kind: kind, Scope: scope}
}

func (s ContractSuite) requireMemory() MemoryManager {
	s.T.Helper()
	if s.RT.Memory == nil {
		s.T.Fatal("memory 入口缺席：无法执行记忆契约检查")
	}
	return s.RT.Memory
}

// ---- 声明层 ------------------------------------------------------------------

// DeclarationIsSelfConsistent 校验"申报的形态与交付的入口一致"。
// 声明说支持却交不出来，比沉默缺失更糟：调用方会按声明去调，然后撞 nil。
func (s ContractSuite) DeclarationIsSelfConsistent() {
	s.T.Helper()
	rt := s.RT
	if rt.AgentID != s.AgentID {
		s.T.Fatalf("runtime.AgentID=%q 与注册 id=%q 不一致", rt.AgentID, s.AgentID)
	}
	if rt.Descriptor.ID != s.AgentID {
		s.T.Fatalf("descriptor.ID=%q 与注册 id=%q 不一致", rt.Descriptor.ID, s.AgentID)
	}
	if rt.Descriptor.SessionModel == "" {
		s.T.Fatal("descriptor.SessionModel 未申报（会话事实在谁手里必须有答案）")
	}
	rep := VerifyRuntime(rt)
	if !rep.OK() {
		for _, v := range rep.Violations {
			s.T.Errorf("契约违约: %s", v)
		}
	}
}

// ---- 记忆 --------------------------------------------------------------------

// MemoryAppend 对应规格 §15 L1 的 MemoryAppendTest。
func (s ContractSuite) MemoryAppend() {
	s.T.Helper()
	mem := s.requireMemory()
	key := s.key(MemoryConversation, "")
	for i := 1; i <= 3; i++ {
		seq, err := mem.Append(s.ctx(), key, MemoryItem{Role: "user", Payload: []byte(fmt.Sprintf("m%d", i))})
		if err != nil {
			s.T.Fatalf("Append 第 %d 条失败: %v", i, err)
		}
		if seq != int64(i) {
			s.T.Fatalf("sequence 应单调从 1 起：第 %d 条得到 %d", i, seq)
		}
	}
	snap, err := mem.Load(s.ctx(), key)
	if err != nil {
		s.T.Fatalf("Load 失败: %v", err)
	}
	if len(snap.Items) != 3 {
		s.T.Fatalf("Append 3 条后应有 3 项，实得 %d", len(snap.Items))
	}
	if got := string(snap.Items[2].Payload); got != "m3" {
		s.T.Fatalf("第 3 条原文应原样保存，实得 %q", got)
	}
	if snap.Items[2].Kind != MemoryConversation {
		s.T.Fatalf("Append 未回填 Kind，实得 %q", snap.Items[2].Kind)
	}
	if snap.Items[0].At.IsZero() {
		s.T.Fatal("Append 未回填时间戳")
	}
}

// MemoryLoadAndWindow 对应 MemoryLoadTest（含"窗口 = 最近"的语义）。
func (s ContractSuite) MemoryLoadAndWindow() {
	s.T.Helper()
	mem := s.requireMemory()
	empty, err := mem.Load(s.ctx(), s.key(MemoryWorking, "empty"))
	if err != nil {
		s.T.Fatalf("空记忆 Load 不应报错: %v", err)
	}
	if empty == nil || len(empty.Items) != 0 {
		s.T.Fatalf("空记忆应返回空快照，实得 %+v", empty)
	}

	key := s.key(MemoryWorking, "window")
	for i := 1; i <= 5; i++ {
		if _, err := mem.Append(s.ctx(), key, MemoryItem{Payload: []byte(fmt.Sprintf("w%d", i))}); err != nil {
			s.T.Fatalf("Append 失败: %v", err)
		}
	}
	win, err := mem.ReadWindow(s.ctx(), key, MemoryLimit{MaxItems: 2})
	if err != nil {
		s.T.Fatalf("ReadWindow 失败: %v", err)
	}
	if len(win) != 2 {
		s.T.Fatalf("MaxItems=2 应返回 2 项，实得 %d", len(win))
	}
	// 窗口语义 = 最近：应取到 w4、w5，而不是 w1、w2。
	if string(win[0].Payload) != "w4" || string(win[1].Payload) != "w5" {
		s.T.Fatalf("窗口应取最近 2 项 (w4,w5)，实得 (%s,%s)", win[0].Payload, win[1].Payload)
	}
	inc, err := mem.ReadWindow(s.ctx(), key, MemoryLimit{FromSequence: 3})
	if err != nil {
		s.T.Fatalf("增量 ReadWindow 失败: %v", err)
	}
	if len(inc) != 2 || string(inc[0].Payload) != "w4" {
		s.T.Fatalf("FromSequence=3 应返回 w4,w5，实得 %d 项", len(inc))
	}
}

// MemoryRejectsIncompleteKey：维度不全的 key 必须报错而不是写进同一个槽
// （那会造成跨会话串味）。
func (s ContractSuite) MemoryRejectsIncompleteKey() {
	s.T.Helper()
	mem := s.requireMemory()
	if _, err := mem.Append(s.ctx(), MemoryKey{Kind: MemoryConversation}, MemoryItem{Payload: []byte("x")}); !errors.Is(err, ErrMemoryKeyIncomplete) {
		s.T.Fatalf("缺 agent/thread 的 key 应返回 ErrMemoryKeyIncomplete，实得 %v", err)
	}
	if _, err := mem.Load(s.ctx(), MemoryKey{AgentID: s.AgentID}); !errors.Is(err, ErrMemoryKeyIncomplete) {
		s.T.Fatalf("缺 thread/kind 的 key 应返回 ErrMemoryKeyIncomplete，实得 %v", err)
	}
}

// MemoryCompaction 对应规格 §15 L1 的 MemoryCompactionTest，
// 并强制校验"不静默丢数据"这条不变量（CheckMemoryIntegrity）。
func (s ContractSuite) MemoryCompaction() {
	s.T.Helper()
	mem := s.requireMemory()
	key := s.key(MemoryConversation, "compact")
	var appended []int64
	for i := 1; i <= 10; i++ {
		seq, err := mem.Append(s.ctx(), key, MemoryItem{Role: "user", Payload: []byte(fmt.Sprintf("c%02d", i))})
		if err != nil {
			s.T.Fatalf("Append 失败: %v", err)
		}
		appended = append(appended, seq)
	}
	snap, err := mem.Compact(s.ctx(), key, CompactPolicy{KeepRecentItems: 4, Reason: "上下文超预算"})
	if err != nil {
		s.T.Fatalf("Compact 失败: %v", err)
	}
	if len(snap.Items) != 4 {
		s.T.Fatalf("KeepRecentItems=4 应保留 4 项，实得 %d", len(snap.Items))
	}
	if snap.Covered.From != 1 || snap.Covered.To != 6 {
		s.T.Fatalf("摘要覆盖范围应为 1..6，实得 %d..%d", snap.Covered.From, snap.Covered.To)
	}
	if snap.Digest == "" {
		s.T.Fatal("压缩必须留下摘要正文（Digest），否则被压缩的内容无从解释")
	}
	if err := CheckMemoryIntegrity(snap, appended); err != nil {
		s.T.Fatalf("压缩后完整性被破坏（静默丢数据）: %v", err)
	}
	// 压缩幂等：已无可压缩项时不应改变覆盖上界。
	again, err := mem.Compact(s.ctx(), key, CompactPolicy{KeepRecentItems: 4})
	if err != nil {
		s.T.Fatalf("重复 Compact 失败: %v", err)
	}
	if again.Covered.To != snap.Covered.To {
		s.T.Fatalf("重复压缩不应推进覆盖上界：%d → %d", snap.Covered.To, again.Covered.To)
	}
}

// MemoryCheckpointAndRestore 对应 MemoryCheckpointTest / MemoryRestoreTest。
func (s ContractSuite) MemoryCheckpointAndRestore() {
	s.T.Helper()
	mem := s.requireMemory()
	key := s.key(MemoryConversation, "ckpt")
	for i := 1; i <= 3; i++ {
		if _, err := mem.Append(s.ctx(), key, MemoryItem{Payload: []byte(fmt.Sprintf("k%d", i))}); err != nil {
			s.T.Fatalf("Append 失败: %v", err)
		}
	}
	if err := mem.Checkpoint(s.ctx(), key); err != nil {
		s.T.Fatalf("Checkpoint 失败: %v", err)
	}
	// checkpoint 之后继续写：Restore 必须回到 checkpoint 那一刻，而不是回到最新。
	for i := 4; i <= 5; i++ {
		if _, err := mem.Append(s.ctx(), key, MemoryItem{Payload: []byte(fmt.Sprintf("k%d", i))}); err != nil {
			s.T.Fatalf("Append 失败: %v", err)
		}
	}
	if err := mem.Restore(s.ctx(), key); err != nil {
		s.T.Fatalf("Restore 失败: %v", err)
	}
	snap, err := mem.Load(s.ctx(), key)
	if err != nil {
		s.T.Fatalf("Restore 后 Load 失败: %v", err)
	}
	if len(snap.Items) != 3 {
		s.T.Fatalf("Restore 应回到 checkpoint 时的 3 项，实得 %d", len(snap.Items))
	}
	// 没 checkpoint 过的 key 上 Restore 不许清空内存态（那本身就是静默丢数据）。
	other := s.key(MemoryWorking, "no-ckpt")
	if _, err := mem.Append(s.ctx(), other, MemoryItem{Payload: []byte("keepme")}); err != nil {
		s.T.Fatalf("Append 失败: %v", err)
	}
	if err := mem.Restore(s.ctx(), other); err != nil {
		s.T.Fatalf("无 checkpoint 时 Restore 应无害返回: %v", err)
	}
	after, _ := mem.Load(s.ctx(), other)
	if len(after.Items) != 1 {
		s.T.Fatalf("无 checkpoint 的 Restore 不得清空内存态，实得 %d 项", len(after.Items))
	}
}

// MemorySurvivesRestartOnlyViaCheckpoint 对应规格 §15 L1 的 NativeSessionRecoveryTest
// 的记忆侧，并**反证**"重启是真的"：没 checkpoint 的记忆必须消失。
func (s ContractSuite) MemorySurvivesRestartOnlyViaCheckpoint() {
	s.T.Helper()
	mem := s.requireMemory()
	kept := s.key(MemoryConversation, "restart-kept")
	lost := s.key(MemoryConversation, "restart-lost")
	for i := 1; i <= 2; i++ {
		if _, err := mem.Append(s.ctx(), kept, MemoryItem{Payload: []byte(fmt.Sprintf("kept%d", i))}); err != nil {
			s.T.Fatalf("Append 失败: %v", err)
		}
	}
	if _, err := mem.Append(s.ctx(), lost, MemoryItem{Payload: []byte("lost1")}); err != nil {
		s.T.Fatalf("Append 失败: %v", err)
	}
	if err := mem.Checkpoint(s.ctx(), kept); err != nil {
		s.T.Fatalf("Checkpoint 失败: %v", err)
	}
	// 只有 kept 落了盘，lost 没有。
	durable, err := s.Backend.ExportDurable()
	if err != nil {
		s.T.Fatalf("ExportDurable 失败: %v", err)
	}
	if err := s.Backend.ImportDurable(durable); err != nil {
		s.T.Fatalf("ImportDurable（模拟重启）失败: %v", err)
	}

	mem2 := s.RT.Memory
	if err := mem2.Restore(s.ctx(), kept); err != nil {
		s.T.Fatalf("重启后 Restore 失败: %v", err)
	}
	got, err := mem2.Load(s.ctx(), kept)
	if err != nil {
		s.T.Fatalf("重启后 Load 失败: %v", err)
	}
	if len(got.Items) != 2 {
		s.T.Fatalf("已 checkpoint 的记忆必须跨重启存活，实得 %d 项", len(got.Items))
	}
	// 反证：没 checkpoint 的必须丢 —— 否则说明"重启"是假的，整项检查都不成立。
	lostSnap, _ := mem2.Load(s.ctx(), lost)
	if len(lostSnap.Items) != 0 {
		s.T.Fatalf("未 checkpoint 的记忆不该跨重启存活（说明重启模拟是假的），实得 %d 项", len(lostSnap.Items))
	}
}

// ---- 缓存 --------------------------------------------------------------------

func (s ContractSuite) requireCache() CacheManager {
	s.T.Helper()
	if s.RT.Cache == nil {
		s.T.Fatal("cache 入口缺席：无法执行缓存契约检查")
	}
	return s.RT.Cache
}

func (s ContractSuite) cacheKey(name string) CacheKey {
	return CacheKey{Namespace: Namespace(s.AgentID, NsModels), Key: name}
}

// CacheHit 对应 CacheHitTest。
func (s ContractSuite) CacheHit() {
	s.T.Helper()
	c := s.requireCache()
	k := s.cacheKey("hit")
	payload := []byte("models-v1")
	if err := c.Put(s.ctx(), k, CacheValue{Payload: payload}, CachePolicy{TTL: time.Hour}); err != nil {
		s.T.Fatalf("Put 失败: %v", err)
	}
	got, ok, err := c.Get(s.ctx(), k)
	if err != nil {
		s.T.Fatalf("Get 失败: %v", err)
	}
	if !ok {
		s.T.Fatal("新鲜条目应命中")
	}
	if !bytes.Equal(got.Payload, payload) {
		s.T.Fatalf("命中值应原样返回，实得 %q", got.Payload)
	}
	if st := c.Stats(s.ctx(), k.Namespace); st.Hits == 0 {
		s.T.Fatal("命中必须计入 Stats.Hits（没有命中率就无法校准 TTL）")
	}
}

// CacheMiss 对应 CacheMissTest。
func (s ContractSuite) CacheMiss() {
	s.T.Helper()
	c := s.requireCache()
	if _, ok, err := c.Get(s.ctx(), s.cacheKey("never-written")); err != nil || ok {
		s.T.Fatalf("未写入的 key 应未命中，实得 ok=%v err=%v", ok, err)
	}
	if _, _, err := c.Get(s.ctx(), CacheKey{Namespace: "x"}); !errors.Is(err, ErrCacheKeyIncomplete) {
		s.T.Fatalf("缺 key 的 CacheKey 应报 ErrCacheKeyIncomplete，实得 %v", err)
	}
}

// CacheTTL 对应 CacheTTLTest（用注入时钟，不 sleep）。
func (s ContractSuite) CacheTTL() {
	s.T.Helper()
	c := s.requireCache()
	now := time.Now()
	s.Backend.SetClock(func() time.Time { return now })
	defer s.Backend.SetClock(nil)

	k := s.cacheKey("ttl")
	if err := c.Put(s.ctx(), k, CacheValue{Payload: []byte("v")}, CachePolicy{TTL: time.Minute}); err != nil {
		s.T.Fatalf("Put 失败: %v", err)
	}
	now = now.Add(30 * time.Second)
	if _, ok, _ := c.Get(s.ctx(), k); !ok {
		s.T.Fatal("TTL 内应命中")
	}
	now = now.Add(2 * time.Minute)
	if _, ok, _ := c.Get(s.ctx(), k); ok {
		s.T.Fatal("TTL 过后 Get 必须未命中（Get 是严格路径）")
	}
	// 但 Peek 仍能看到它 —— 这正是"过期但可用"的降级素材。
	entry, found, err := c.Peek(s.ctx(), k)
	if err != nil || !found {
		s.T.Fatalf("过期条目 Peek 应仍可读到，实得 found=%v err=%v", found, err)
	}
	if entry.Fresh {
		s.T.Fatal("过期条目 Fresh 应为 false")
	}
}

// CacheEviction 对应 CacheEvictionTest。
func (s ContractSuite) CacheEviction() {
	s.T.Helper()
	c := s.requireCache()
	ns := Namespace(s.AgentID, NsCapabilities)
	policy := CachePolicy{TTL: time.Hour, MaxEntries: 2, Eviction: EvictFIFO}
	for i := 1; i <= 3; i++ {
		k := CacheKey{Namespace: ns, Key: fmt.Sprintf("e%d", i)}
		if err := c.Put(s.ctx(), k, CacheValue{Payload: []byte(fmt.Sprintf("v%d", i))}, policy); err != nil {
			s.T.Fatalf("Put %d 失败: %v", i, err)
		}
	}
	st := c.Stats(s.ctx(), ns)
	if st.Entries != 2 {
		s.T.Fatalf("MaxEntries=2 应只留 2 条，实得 %d", st.Entries)
	}
	if st.Evictions == 0 {
		s.T.Fatal("淘汰必须计数（否则容量策略无法观测）")
	}
	// FIFO：最早写入的 e1 应被淘汰。
	if _, ok, _ := c.Get(s.ctx(), CacheKey{Namespace: ns, Key: "e1"}); ok {
		s.T.Fatal("FIFO 策略下最早写入的条目应被淘汰")
	}
	if _, ok, _ := c.Get(s.ctx(), CacheKey{Namespace: ns, Key: "e3"}); !ok {
		s.T.Fatal("最新写入的条目应保留")
	}

	// EvictNone：满则拒绝写入，且不得留下半个条目。
	ns2 := Namespace(s.AgentID, NsMCP)
	strict := CachePolicy{TTL: time.Hour, MaxEntries: 1, Eviction: EvictNone}
	if err := c.Put(s.ctx(), CacheKey{Namespace: ns2, Key: "s1"}, CacheValue{Payload: []byte("a")}, strict); err != nil {
		s.T.Fatalf("Put s1 失败: %v", err)
	}
	if err := c.Put(s.ctx(), CacheKey{Namespace: ns2, Key: "s2"}, CacheValue{Payload: []byte("b")}, strict); !errors.Is(err, ErrCacheFull) {
		s.T.Fatalf("EvictNone 满时应返回 ErrCacheFull，实得 %v", err)
	}
	if _, ok, _ := c.Get(s.ctx(), CacheKey{Namespace: ns2, Key: "s2"}); ok {
		s.T.Fatal("被拒绝的写入不得留下部分条目")
	}
}

// CacheStaleDegrade：把规格 §6.4 的四条读取规则跑成可执行判据。
func (s ContractSuite) CacheStaleDegrade() {
	s.T.Helper()
	decide := func(fresh, hit, staleAllowed bool) ReadDecision {
		return DecideRead(CacheEntry{Fresh: fresh, StaleAllowed: staleAllowed}, hit)
	}
	cases := []struct {
		name                     string
		hit, fresh, staleAllowed bool
		want                     ReadDecision
	}{
		{"新鲜命中", true, true, false, DecideFresh},
		{"过期但允许降级", true, false, true, DecideStaleUsable},
		{"过期且不许降级", true, false, false, DecideMiss},
		{"无缓存", false, false, true, DecideMiss},
	}
	for _, tc := range cases {
		if got := decide(tc.fresh, tc.hit, tc.staleAllowed); got != tc.want {
			s.T.Errorf("%s: DecideRead=%q 期望 %q", tc.name, got, tc.want)
		}
	}
}

// CacheNamespacesAreIsolated：不同 agent 的缓存不许串味。
//
// 断言必须建立在**两个命名空间都真的写过**的前提下 —— 只写一边再断言
// "另一边读不到"，验证的是一件从未发生的事（那个断言恒真，等于没测）。
func (s ContractSuite) CacheNamespacesAreIsolated() {
	s.T.Helper()
	c := s.requireCache()
	mine := Namespace(s.AgentID, NsModels)
	theirs := Namespace(s.AgentID+"-other", NsModels)
	k1 := CacheKey{Namespace: mine, Key: "iso"}
	k2 := CacheKey{Namespace: theirs, Key: "iso"}
	if err := c.Put(s.ctx(), k1, CacheValue{Payload: []byte("mine")}, CachePolicy{TTL: time.Hour}); err != nil {
		s.T.Fatalf("Put k1 失败: %v", err)
	}
	if err := c.Put(s.ctx(), k2, CacheValue{Payload: []byte("theirs")}, CachePolicy{TTL: time.Hour}); err != nil {
		s.T.Fatalf("Put k2 失败: %v", err)
	}
	got, ok, err := c.Get(s.ctx(), k2)
	if err != nil || !ok {
		s.T.Fatalf("另一命名空间的值应可读，实得 ok=%v err=%v", ok, err)
	}
	if string(got.Payload) != "theirs" {
		s.T.Fatalf("命名空间串味：读 %s 拿到 %q", theirs, got.Payload)
	}
	// Invalidate 前缀语义：清 agent/<id> 要能清掉它下面所有子命名空间，
	// 但不得越界清掉别的 agent（前缀 "agent/stub" 与 "agent/stub-other" 必须分开）。
	if err := c.Invalidate(s.ctx(), Namespace(s.AgentID, "")); err != nil {
		s.T.Fatalf("Invalidate 失败: %v", err)
	}
	if _, ok, _ := c.Get(s.ctx(), k1); ok {
		s.T.Fatal("Invalidate 应做前缀匹配（清 agent/<id> 要清掉 agent/<id>/models）")
	}
	if _, ok, _ := c.Get(s.ctx(), k2); !ok {
		s.T.Fatal("Invalidate 不得越界清掉别的 agent 的缓存")
	}
}

// ---- 资源 --------------------------------------------------------------------

func (s ContractSuite) requireLedger() ResourceLedger {
	s.T.Helper()
	if s.RT.Resources == nil {
		s.T.Fatal("resource 入口缺席：无法执行资源契约检查")
	}
	return s.RT.Resources
}

// ResourceLimit 对应 ResourceLimitTest：硬上限只拒可选资源，必需资源不被拒。
func (s ContractSuite) ResourceLimit() {
	s.T.Helper()
	led := s.requireLedger()
	scope := ScopeAgent(s.AgentID)
	opt := LeaseRequest{
		LeaseID: "opt-" + s.AgentID, OwnerScope: scope, Kind: ResWorkingRAM,
		SoftLimit: 100, HardLimit: 200, SpillPolicy: SpillCompact, Optional: true,
	}
	if _, err := led.Acquire(s.ctx(), opt); err != nil {
		s.T.Fatalf("可选租约申请失败: %v", err)
	}
	if _, err := led.Charge(s.ctx(), opt.LeaseID, 150); err != nil {
		s.T.Fatalf("硬上限内的记账不该失败: %v", err)
	}
	lease, err := led.Charge(s.ctx(), opt.LeaseID, 60)
	if !errors.Is(err, ErrOverHardLimit) {
		s.T.Fatalf("可选资源越过硬上限应返回 ErrOverHardLimit，实得 %v", err)
	}
	if lease != nil {
		s.T.Fatal("被拒绝的记账不得返回租约（否则调用方会以为记上了）")
	}
	after, err := led.Charge(s.ctx(), opt.LeaseID, 1)
	if err != nil {
		s.T.Fatalf("被拒绝后再记 1 字节应成功（用量必须停在拒绝前）: %v", err)
	}
	if after.UsedBytes != 151 {
		s.T.Fatalf("被拒绝的记账不得改变用量：应为 151，实得 %d", after.UsedBytes)
	}
	// 同 scope+kind 上再申请可选租约，累计触硬上限 → 拒。
	sameKind := LeaseRequest{
		LeaseID: "opt2-" + s.AgentID, OwnerScope: scope, Kind: ResWorkingRAM,
		ReservedBytes: 100, SoftLimit: 100, HardLimit: 200, Optional: true,
	}
	if _, err := led.Acquire(s.ctx(), sameKind); !errors.Is(err, ErrOverHardLimit) {
		s.T.Fatalf("同 scope+kind 上累计触硬上限应拒绝新可选租约，实得 %v", err)
	}
	// 必需资源允许越限（拒掉必需资源等于换个名字丢用户数据），但要如实记账。
	required := LeaseRequest{
		LeaseID: "req-" + s.AgentID, OwnerScope: scope, Kind: ResConversation,
		ReservedBytes: 500, SoftLimit: 100, HardLimit: 200, Optional: false,
	}
	if _, err := led.Acquire(s.ctx(), required); err != nil {
		s.T.Fatalf("必需资源不得被硬上限拒绝: %v", err)
	}
	rep := led.Usage(s.ctx(), scope)
	found := false
	for _, id := range rep.OverHard {
		if id == required.LeaseID {
			found = true
		}
	}
	if !found {
		s.T.Fatalf("越限的必需资源必须出现在 Usage.OverHard 里（不许静默）: %+v", rep)
	}
}

// ResourceAccounting：释放幂等 + 用量如实汇总。
func (s ContractSuite) ResourceAccounting() {
	s.T.Helper()
	led := s.requireLedger()
	req := LeaseRequest{
		LeaseID: "acc-" + s.AgentID, OwnerScope: ScopeThread(s.Thread.ID), Kind: ResEventBuffer,
		ReservedBytes: 10, SoftLimit: 1000, HardLimit: 2000, Optional: true,
	}
	if _, err := led.Acquire(s.ctx(), req); err != nil {
		s.T.Fatalf("Acquire 失败: %v", err)
	}
	if _, err := led.Acquire(s.ctx(), req); !errors.Is(err, ErrLeaseExists) {
		s.T.Fatalf("复用活跃租约 id 应返回 ErrLeaseExists，实得 %v", err)
	}
	rep := led.Usage(s.ctx(), ScopeThread(s.Thread.ID))
	if rep.ByKind[ResEventBuffer] != 10 {
		s.T.Fatalf("用量应按 kind 汇总为 10，实得 %d", rep.ByKind[ResEventBuffer])
	}
	if err := led.Release(s.ctx(), req.LeaseID); err != nil {
		s.T.Fatalf("Release 失败: %v", err)
	}
	if err := led.Release(s.ctx(), req.LeaseID); err != nil {
		s.T.Fatalf("Release 应幂等（常在 defer 里调用），实得 %v", err)
	}
	if rep := led.Usage(s.ctx(), ScopeThread(s.Thread.ID)); rep.Leases != 0 {
		s.T.Fatalf("释放后不应再计入用量，实得 %d 个租约", rep.Leases)
	}
	if err := led.Release(s.ctx(), "never-existed"); !errors.Is(err, ErrLeaseNotFound) {
		s.T.Fatalf("释放不存在的租约应返回 ErrLeaseNotFound，实得 %v", err)
	}
}

// ---- 会话 / checkpoint -------------------------------------------------------

// SessionSourceMatchesDescriptor：会话来源的申报必须与实际适配器一致。
func (s ContractSuite) SessionSourceMatchesDescriptor() {
	s.T.Helper()
	if s.RT.NativeSession == nil {
		return // 未交付会话适配器：由 VerifyRuntime 按声明判定是否合规
	}
	if got := s.RT.NativeSession.Source(); got != s.RT.Descriptor.SessionModel {
		s.T.Fatalf("NativeSession.Source()=%q 与 descriptor.SessionModel=%q 不一致", got, s.RT.Descriptor.SessionModel)
	}
}

// SessionResumeAfterRestart 对应 NativeSessionRecoveryTest 的会话侧。
func (s ContractSuite) SessionResumeAfterRestart() {
	s.T.Helper()
	if s.RT.NativeSession == nil {
		return
	}
	ns := s.RT.NativeSession
	if err := ns.Open(s.ctx(), s.Thread); err != nil {
		s.T.Fatalf("Open 失败: %v", err)
	}
	if err := ns.Checkpoint(s.ctx(), s.Thread); err != nil {
		s.T.Fatalf("Checkpoint 失败: %v", err)
	}
	durable, err := s.Backend.ExportDurable()
	if err != nil {
		s.T.Fatalf("ExportDurable 失败: %v", err)
	}
	if err := s.Backend.ImportDurable(durable); err != nil {
		s.T.Fatalf("ImportDurable 失败: %v", err)
	}
	if err := s.RT.NativeSession.Resume(s.ctx(), s.Thread); err != nil {
		s.T.Fatalf("重启后应能按落库引用重挂载会话: %v", err)
	}
	// 反证：没建立过会话的 thread 必须恢复失败，而不是"假装恢复成功"。
	stranger := &store.Thread{ID: s.Thread.ID + "-stranger"}
	if err := s.RT.NativeSession.Resume(s.ctx(), stranger); err == nil {
		s.T.Fatal("没有会话引用的 thread 恢复必须报错，不得假装成功")
	}
}

// CheckpointSaveLoadDrop：checkpoint 的三段语义。
func (s ContractSuite) CheckpointSaveLoadDrop() {
	s.T.Helper()
	if s.RT.Checkpoint == nil {
		return
	}
	cm := s.RT.Checkpoint
	k := CheckpointKey{AgentID: s.AgentID, ThreadID: s.Thread.ID, Name: "suite-probe"}
	payload := []byte(`{"probe":true}`)
	if _, found, err := cm.Load(s.ctx(), k); err != nil || found {
		s.T.Fatalf("未写入时 Load 应 found=false 且无错，实得 found=%v err=%v", found, err)
	}
	if err := cm.Save(s.ctx(), k, payload); err != nil {
		s.T.Fatalf("Save 失败: %v", err)
	}
	got, found, err := cm.Load(s.ctx(), k)
	if err != nil || !found {
		s.T.Fatalf("Save 后 Load 应 found=true，实得 found=%v err=%v", found, err)
	}
	if !bytes.Equal(got, payload) {
		s.T.Fatalf("Load 应原样返回，实得 %q", got)
	}
	// 覆盖写：内容必须整体替换，不得拼接出半截内容。
	if err := cm.Save(s.ctx(), k, []byte("v2")); err != nil {
		s.T.Fatalf("覆盖 Save 失败: %v", err)
	}
	if got, _, _ := cm.Load(s.ctx(), k); string(got) != "v2" {
		s.T.Fatalf("覆盖写应整体替换，实得 %q", got)
	}
	if err := cm.Drop(s.ctx(), k); err != nil {
		s.T.Fatalf("Drop 失败: %v", err)
	}
	if _, found, _ := cm.Load(s.ctx(), k); found {
		s.T.Fatal("Drop 后不应再读到")
	}
	if err := cm.Drop(s.ctx(), k); err != nil {
		s.T.Fatalf("Drop 应幂等，实得 %v", err)
	}
	if err := cm.Save(s.ctx(), CheckpointKey{AgentID: s.AgentID}, []byte("x")); !errors.Is(err, ErrCheckpointKeyIncomplete) {
		s.T.Fatalf("维度不全的 checkpoint key 应报错，实得 %v", err)
	}
}
