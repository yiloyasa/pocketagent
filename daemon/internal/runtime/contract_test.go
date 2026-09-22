package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/pocketagent/daemon/internal/store"
)

// stubProvider 是契约测试用的最小 Participant：只声明，不接任何原生 agent。
// 它存在的意义是让 runtime 包的契约用例可以自测 —— 生产 provider 的契约测试
// 在 agent 包里对真实 provider 跑同一套 ContractSuite。
type stubProvider struct {
	desc     AgentDescriptor
	backend  *Backend
	failOpen bool
}

func (p *stubProvider) Descriptor() AgentDescriptor { return p.desc }

func (p *stubProvider) OpenRuntime(_ context.Context, th *store.Thread) (*AgentRuntime, error) {
	if p.failOpen {
		return nil, errors.New("stub: 运行时未就绪")
	}
	return p.backend.FullRuntime(p.desc, th.ID), nil
}

// compliantDescriptor 是一份完整达标的最小申报（四个入口全部声明支持）。
func compliantDescriptor(id string) AgentDescriptor {
	return AgentDescriptor{
		ID:            id,
		DisplayName:   "Stub",
		Version:       "test",
		Capabilities:  append([]Capability(nil), RequiredEntryPoints...),
		SessionModel:  SourceDaemon,
		ResumeModel:   ResumeReattach,
		ApprovalModel: ApprovalNone,
		SubagentModel: SubagentNone,
		MemoryModel:   MemoryDaemon,
		CacheModel:    CacheDaemon,
	}
}

// TestContractSuiteOnDefaultBackend 是 Wave 1 的绿灯：默认后端必须满足全部契约。
func TestContractSuiteOnDefaultBackend(t *testing.T) {
	desc := compliantDescriptor("stub")
	b := NewBackend()
	th := &store.Thread{ID: "th-suite"}
	rt, err := (&stubProvider{desc: desc, backend: b}).OpenRuntime(context.Background(), th)
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	ContractSuite{T: t, AgentID: "stub", RT: rt, Backend: b, Thread: th}.RunAll()
}

// TestContractSuiteRefusesWithoutPreconditions：前条件缺失必须拒绝执行，
// 而不是静默跳掉几项检查冒充通过。
func TestContractSuiteRefusesWithoutPreconditions(t *testing.T) {
	b := NewBackend()
	th := &store.Thread{ID: "th"}
	rt := b.FullRuntime(compliantDescriptor("stub"), th.ID)
	cases := []struct {
		name string
		s    ContractSuite
	}{
		{"RT 为空", ContractSuite{T: t, AgentID: "stub", Backend: b, Thread: th}},
		{"Backend 为空", ContractSuite{T: t, AgentID: "stub", RT: rt, Thread: th}},
		{"Thread 为空", ContractSuite{T: t, AgentID: "stub", RT: rt, Backend: b}},
		{"AgentID 为空", ContractSuite{T: t, RT: rt, Backend: b, Thread: th}},
	}
	for _, tc := range cases {
		if err := tc.s.Precondition(); err == nil {
			t.Errorf("%s: 前条件缺失必须报错，实得 nil", tc.name)
		}
	}
	ok := ContractSuite{T: t, AgentID: "stub", RT: rt, Backend: b, Thread: th}
	if err := ok.Precondition(); err != nil {
		t.Fatalf("完整套件不该报前条件错误: %v", err)
	}
}

// ---- 门禁必须能红 ------------------------------------------------------------

// TestVerifyDeclarationRejectsUndeclaredAgent：没实现 Participant 的 provider
// 必须被点名（这是"连形态都不申报"的最差情况）。
func TestVerifyDeclarationRejectsUndeclaredAgent(t *testing.T) {
	rep := VerifyDeclaration("ghost", "我不是 provider")
	if rep.OK() {
		t.Fatal("未实现 Participant 的 provider 必须违约")
	}
	if rep.Violations[0].Code != ViolationNotDeclared {
		t.Fatalf("违约编码应为 %q，实得 %q", ViolationNotDeclared, rep.Violations[0].Code)
	}
	if rep.Declared {
		t.Fatal("未声明时 Declared 应为 false")
	}
}

// TestVerifyDeclarationRejectsSilentMissingEntryPoint：四个必需入口
// 既没实现也没声明不支持 = 沉默缺失，必须报警。
func TestVerifyDeclarationRejectsSilentMissingEntryPoint(t *testing.T) {
	desc := compliantDescriptor("stub")
	// 去掉 resource 入口，且不加 Limitation —— 这正是"悄悄没有"的形态。
	var caps []Capability
	for _, c := range desc.Capabilities {
		if c != CapRuntimeResource {
			caps = append(caps, c)
		}
	}
	desc.Capabilities = caps

	rep := VerifyDeclaration("stub", &stubProvider{desc: desc, backend: NewBackend()})
	if rep.OK() {
		t.Fatal("沉默缺失必需入口必须违约")
	}
	found := false
	for _, v := range rep.Violations {
		if v.Code == ViolationEntrySilentMissing {
			found = true
		}
	}
	if !found {
		t.Fatalf("应报告 %q，实得 %+v", ViolationEntrySilentMissing, rep.Violations)
	}
}

// TestVerifyDeclarationAcceptsExplicitLimitation：做不到但**明说**，属于合规。
// 这是契约的刻意设计：允许声明不支持，只禁止沉默。
func TestVerifyDeclarationAcceptsExplicitLimitation(t *testing.T) {
	desc := compliantDescriptor("stub")
	var caps []Capability
	for _, c := range desc.Capabilities {
		if c != CapRuntimeResource {
			caps = append(caps, c)
		}
	}
	desc.Capabilities = caps
	desc.Limitations = []Limitation{{
		Capability: CapRuntimeResource,
		Reason:     "该 agent 无并发预算概念（本轮 P0.5 未接入）",
	}}

	rep := VerifyDeclaration("stub", &stubProvider{desc: desc, backend: NewBackend()})
	if !rep.OK() {
		t.Fatalf("显式声明不支持应合规，实得 %+v", rep.Violations)
	}
	if len(rep.Limitations) != 1 {
		t.Fatalf("Limitations 应原样带出（供 UI 显示「该 agent 未报告此项」），实得 %d 条", len(rep.Limitations))
	}
}

// TestVerifyDeclarationRejectsIDMismatch
func TestVerifyDeclarationRejectsIDMismatch(t *testing.T) {
	desc := compliantDescriptor("echo") // 申报的 ID 与注册 id 不一致
	rep := VerifyDeclaration("pi", &stubProvider{desc: desc, backend: NewBackend()})
	if rep.OK() {
		t.Fatal("descriptor.ID 与注册 id 不一致必须违约")
	}
	if rep.Violations[0].Code != ViolationIDMismatch {
		t.Fatalf("违约编码应为 %q，实得 %q", ViolationIDMismatch, rep.Violations[0].Code)
	}
}

// TestVerifyDeclarationRejectsSelfContradiction：既声明支持又声明不支持。
func TestVerifyDeclarationRejectsSelfContradiction(t *testing.T) {
	desc := compliantDescriptor("stub")
	desc.Limitations = []Limitation{{Capability: CapRuntimeMemory, Reason: "自相矛盾"}}
	rep := VerifyDeclaration("stub", &stubProvider{desc: desc, backend: NewBackend()})
	found := false
	for _, v := range rep.Violations {
		if v.Code == ViolationCapabilityContradicted {
			found = true
		}
	}
	if !found {
		t.Fatalf("应报告 %q，实得 %+v", ViolationCapabilityContradicted, rep.Violations)
	}
}

// TestVerifyRuntimeRejectsDeclaredButAbsent：声明说支持、运行时交不出来 ——
// 比沉默缺失更糟，因为调用方会按声明去调，然后撞 nil。
func TestVerifyRuntimeRejectsDeclaredButAbsent(t *testing.T) {
	desc := compliantDescriptor("stub")
	b := NewBackend()
	th := &store.Thread{ID: "th"}
	rt := b.FullRuntime(desc, th.ID)
	rt.Memory = nil // 声明了 CapRuntimeMemory 却没交付

	rep := VerifyRuntime(rt)
	if rep.OK() {
		t.Fatal("声明支持却未交付必须违约")
	}
	if rep.Violations[0].Code != ViolationEntryDeclaredButAbsent {
		t.Fatalf("违约编码应为 %q，实得 %q", ViolationEntryDeclaredButAbsent, rep.Violations[0].Code)
	}
}

// TestVerifyRuntimeRejectsSourceMismatch：会话来源申报与适配器不一致。
func TestVerifyRuntimeRejectsSourceMismatch(t *testing.T) {
	desc := compliantDescriptor("stub")
	desc.SessionModel = SourceNative // 谎报成原生会话是事实来源
	b := NewBackend()
	th := &store.Thread{ID: "th"}
	rt := b.FullRuntime(desc, th.ID) // 实际交付的是 DaemonSession（Source=DAEMON）

	rep := VerifyRuntime(rt)
	found := false
	for _, v := range rep.Violations {
		if v.Code == ViolationSourceMismatch {
			found = true
		}
	}
	if !found {
		t.Fatalf("应报告 %q，实得 %+v", ViolationSourceMismatch, rep.Violations)
	}
}

// TestCheckMemoryIntegrityCatchesSilentLoss：完整性判据本身必须能抓出丢数据。
// 一个永远返回 nil 的检查器等于没检查。
func TestCheckMemoryIntegrityCatchesSilentLoss(t *testing.T) {
	key := MemoryKey{AgentID: "stub", ThreadID: "th", Kind: MemoryConversation}

	// 场景一：seq 4 既不在覆盖范围内、也不在保留项里 → 静默丢失。
	broken := &MemorySnapshot{
		Key:     key,
		Covered: SequenceRange{From: 1, To: 3},
		Items:   []MemoryItem{{Sequence: 5}, {Sequence: 6}},
	}
	if err := CheckMemoryIntegrity(broken, []int64{1, 2, 3, 4, 5, 6}); err == nil {
		t.Fatal("seq 4 凭空消失必须被抓住")
	}

	// 场景二：保留项与覆盖范围之间有空档（seq 4、5 被吞）→ 同样违约。
	gapped := &MemorySnapshot{
		Key:     key,
		Covered: SequenceRange{From: 1, To: 3},
		Items:   []MemoryItem{{Sequence: 6}},
	}
	if err := CheckMemoryIntegrity(gapped, []int64{1, 2, 3, 4, 5, 6}); err == nil {
		t.Fatal("覆盖范围与保留项之间有空档必须被抓住")
	}

	// 对照组：无缝衔接的合法快照必须通过（否则这条判据会把正常压缩也拦下）。
	healthy := &MemorySnapshot{
		Key:     key,
		Covered: SequenceRange{From: 1, To: 4},
		Items:   []MemoryItem{{Sequence: 5}, {Sequence: 6}},
	}
	if err := CheckMemoryIntegrity(healthy, []int64{1, 2, 3, 4, 5, 6}); err != nil {
		t.Fatalf("合法快照不该被判违约: %v", err)
	}
}

// ---- 纯函数判据 --------------------------------------------------------------

func TestClassifyPressure(t *testing.T) {
	cases := []struct {
		name             string
		used, soft, hard int64
		want             Pressure
	}{
		{"远低于软上限", 10, 100, 200, PressureNormal},
		{"恰好触软上限", 100, 100, 200, PressureSoft},
		{"介于软硬之间", 150, 100, 200, PressureSoft},
		{"恰好触硬上限", 200, 100, 200, PressureHard},
		{"无上限配置", 99999, 0, 0, PressureNormal},
	}
	for _, tc := range cases {
		if got := ClassifyPressure(tc.used, tc.soft, tc.hard); got != tc.want {
			t.Errorf("%s: ClassifyPressure(%d,%d,%d)=%q 期望 %q", tc.name, tc.used, tc.soft, tc.hard, got, tc.want)
		}
	}
}

// TestBudgetRejectsSoftAboveHard：soft > hard 的配置会让软上限形同不存在，
// 必须在装载时就被拒，而不是留到运行时表现怪异。
func TestBudgetRejectsSoftAboveHard(t *testing.T) {
	if err := (Budget{Soft: 300, Hard: 200}).Check(); err == nil {
		t.Fatal("软上限高于硬上限必须被拒")
	}
	if err := (Budget{Soft: 100, Hard: 200}).Check(); err != nil {
		t.Fatalf("合法预算不该报错: %v", err)
	}
	if err := (Budget{Soft: -1}).Check(); err == nil {
		t.Fatal("负预算必须被拒")
	}
	if err := (Budget{Soft: 0, Hard: 0}).Check(); err != nil {
		t.Fatalf("0 表示不限，应合法: %v", err)
	}
}

// ---- 跨重启的 sequence 单调性 -------------------------------------------------

// TestMemorySequenceNeverReuses：重启后 sequence 不得从 1 重来。
// 复用的后果是摘要的 Covered 范围与新项的序号重叠 —— 完整性判据会把正常
// 压缩误判成丢数据（或反过来，把真丢数据掩掉）。
func TestMemorySequenceNeverReuses(t *testing.T) {
	b := NewBackend()
	th := &store.Thread{ID: "th"}
	rt := b.FullRuntime(compliantDescriptor("stub"), th.ID)
	key := MemoryKey{AgentID: "stub", ThreadID: th.ID, Kind: MemoryConversation}

	for i := 1; i <= 3; i++ {
		if _, err := rt.Memory.Append(context.Background(), key, MemoryItem{Payload: []byte("x")}); err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
	}
	if err := rt.Memory.Checkpoint(context.Background(), key); err != nil {
		t.Fatalf("Checkpoint 失败: %v", err)
	}
	durable, err := b.ExportDurable()
	if err != nil {
		t.Fatalf("ExportDurable 失败: %v", err)
	}
	if err := b.ImportDurable(durable); err != nil {
		t.Fatalf("ImportDurable 失败: %v", err)
	}

	seq, err := rt.Memory.Append(context.Background(), key, MemoryItem{Payload: []byte("y")})
	if err != nil {
		t.Fatalf("重启后 Append 失败: %v", err)
	}
	if seq != 4 {
		t.Fatalf("重启后 sequence 必须接着走（4），实得 %d —— 复用序号会让摘要覆盖范围失去意义", seq)
	}
}

// TestDurableRoundTripPreservesCheckpoints：落盘态导出/导入必须无损
// （否则"重启后恢复"就是在恢复一个被截断的东西）。
func TestDurableRoundTripPreservesCheckpoints(t *testing.T) {
	b := NewBackend()
	cm := b.Checkpoints()
	key := CheckpointKey{AgentID: "stub", ThreadID: "th", Name: "probe"}
	payload := []byte{0x00, 0x01, 0xff, 0xfe, 'x', '\n'}
	if err := cm.Save(context.Background(), key, payload); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	durable, err := b.ExportDurable()
	if err != nil {
		t.Fatalf("ExportDurable 失败: %v", err)
	}
	if err := b.ImportDurable(durable); err != nil {
		t.Fatalf("ImportDurable 失败: %v", err)
	}
	got, found, err := b.Checkpoints().Load(context.Background(), key)
	if err != nil || !found {
		t.Fatalf("导入后应能读回 checkpoint，实得 found=%v err=%v", found, err)
	}
	if len(got) != len(payload) {
		t.Fatalf("二进制内容长度应保持一致：%d → %d", len(payload), len(got))
	}
	for i := range payload {
		if got[i] != payload[i] {
			t.Fatalf("第 %d 字节不一致: %x → %x", i, payload[i], got[i])
		}
	}
}

// ---- 回归钉子：SequenceRange 零值语义 -----------------------------------------

// TestSequenceRangeZeroValueIsEmpty 钉住一个踩出来的 bug：
// 零值 {0,0} 必须被当成"什么都没覆盖"。若按 To<From 判定，零值会被读成
// "覆盖了 0..0"，Extend 时下界永久卡在 0 —— 实测表现是"压缩 seq 1..6 之后
// 覆盖范围报成 0..6"，进而让完整性判据拿错下界比对。
//
// 把零值语义单独钉住而不是只靠 Compact 用例，是因为 Compact 的断言只覆盖
// 一种路径；零值语义是契约级性质，任何用到 SequenceRange 的地方都依赖它。
func TestSequenceRangeZeroValueIsEmpty(t *testing.T) {
	var zero SequenceRange
	if !zero.Empty() {
		t.Fatalf("零值 %+v 必须被判定为空（sequence 从 1 起，0 永远不是合法序号）", zero)
	}
	if zero.Covers(0) || zero.Covers(1) {
		t.Fatal("空范围不得覆盖任何序号")
	}
	if got := zero.Extend(1); got.From != 1 || got.To != 1 {
		t.Fatalf("空范围 Extend(1) 应为 1..1，实得 %d..%d（下界卡 0 就是这个 bug）", got.From, got.To)
	}

	r := SequenceRange{From: 3, To: 7}
	if r.Empty() {
		t.Fatal("3..7 不是空范围")
	}
	if !r.Covers(3) || !r.Covers(7) || r.Covers(2) || r.Covers(8) {
		t.Fatal("Covers 边界应为闭区间 3..7")
	}
	if got := r.Extend(2); got.From != 2 || got.To != 7 {
		t.Fatalf("Extend(2) 应只推下界: 实得 %d..%d", got.From, got.To)
	}
	if got := r.Extend(9); got.From != 3 || got.To != 9 {
		t.Fatalf("Extend(9) 应只推上界: 实得 %d..%d", got.From, got.To)
	}
	if got := r.Extend(5); got != r {
		t.Fatalf("Extend 落在范围内时不应改变范围: %+v → %+v", r, got)
	}
	// 已被覆盖的序号不得因为再 Extend 而缩小范围。
	if got := r.Extend(1); got.From != 1 || got.To != 7 {
		t.Fatalf("Extend 只许扩大范围: 实得 %d..%d", got.From, got.To)
	}
}
