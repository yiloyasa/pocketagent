// control_plane_test.go —— 控制面总览(GetControlPlane,RPC 84)的回归用例。
//
// 验收点:
//
//	① 一次调用返回全部 8 个域,顺序稳定、id/title/summary 齐备;
//	② uptime 有值且不超过当下(daemon 启动时刻确实被记住了);
//	③ budget 域的数值与 GetRuntimePolicy.effective_global **同源**(改了策略两边
//	   必须一起变 —— 同源是"两个界面看到的数字永远一致"的唯一保障);
//	④ collab 域与 GetCollabPolicy.effective 同源(四项逐一对上);
//	⑤ 单域降级不炸整体:panic 的域单独回 status_code=2,其余域照常;
//	⑥ 没有运行时后端的装配里,memory/dialysis/resources 三个域如实降级为
//	   status_code=2,整条 RPC 仍成功返回全部域;
//	⑦ dialysis 域如实计数未知事件的 NEW 条数,确认后跟着变;
//	⑧ approvals 域如实计数挂起的审批请求;
//	⑨ agents 域对 codex 透出桌面偏好 select 项:候选全部动态(模型列表 /
//	   托管白名单),current 与 GetCodexSettings 同值,无动态候选的键
//	   (model_verbosity / model_reasoning_summary)宁缺毋假不透出;
//	⑩ 候选拿不到(模型列表失败 / 未托管)或 codex 未注册时,select 项整项缺席、
//	   域不降级 —— 不摆假入口;
//	⑪ codex 拉候选受短预算约束:总览页是聚合读,单个 agent 的设置目录拖不垮整页。
package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/version"
)

// wantDomainIDs 是契约钉死的域清单与次序:客户端按 id 匹配排版,次序抖动会让
// "同一页面前后两次刷新换来换去"。新增域在这里补一行即可。
var wantDomainIDs = []string{
	"budget", "collab", "agents", "memory", "dialysis", "approvals", "transports", "resources",
}

func newControlPlaneTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{
		Store:          st,
		Providers:      map[string]agent.Provider{"echo": agent.NewEcho()},
		DataDir:        dir,
		BudgetPolicy:   runtime.BudgetPolicy{Global: runtime.DefaultLimits()},
		CollabPolicy:   runtime.DefaultCollabPolicy(),
		RuntimeBackend: runtime.NewBackend(),
	})
	t.Cleanup(func() { _ = srv.events.Close() })
	return srv
}

func findDomain(t *testing.T, cp *pb.ControlPlane, id string) *pb.ControlDomain {
	t.Helper()
	for _, d := range cp.GetDomains() {
		if d.GetId() == id {
			return d
		}
	}
	t.Fatalf("控制面里没有域 %q(实际: %v)", id, cp.GetDomains())
	return nil
}

func findItem(t *testing.T, d *pb.ControlDomain, id string) *pb.ControlItem {
	t.Helper()
	for _, it := range d.GetItems() {
		if it.GetId() == id {
			return it
		}
	}
	t.Fatalf("域 %s 里没有条目 %q(实际: %v)", d.GetId(), id, d.GetItems())
	return nil
}

// TestGetControlPlaneAssemblesAllDomains 覆盖验收点 ①②。
func TestGetControlPlaneAssemblesAllDomains(t *testing.T) {
	srv := newControlPlaneTestServer(t)
	ctx := context.Background()

	cp, err := srv.GetControlPlane(ctx, &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("GetControlPlane 失败: %v", err)
	}
	// 身份事实:版本与宿主名来自与 GetHostInfo 同源的取数,不许各编一份。
	if cp.GetDaemonVersion() != version.App {
		t.Fatalf("daemon_version 应与 version.App 同源: %q != %q", cp.GetDaemonVersion(), version.App)
	}
	if cp.GetHostName() == "" || cp.GetOs() == "" {
		t.Fatalf("host_name/os 不可为空: %q / %q", cp.GetHostName(), cp.GetOs())
	}
	// uptime:启动时刻有值,且不可能是未来(时钟没疯)。
	now := time.Now().UnixMilli()
	if cp.GetUptimeUnixMs() <= 0 || cp.GetUptimeUnixMs() > now+time.Second.Milliseconds() {
		t.Fatalf("uptime 应为 daemon 启动时刻(>0 且不超过当下),得到 %d(now=%d)", cp.GetUptimeUnixMs(), now)
	}
	// data_dir_label 是诊断标签,绝不能携带绝对路径(路径里的用户名是敏感信息)。
	if cp.GetDataDirLabel() == "" {
		t.Fatal("data_dir_label 不应为空")
	}
	if strings.ContainsAny(cp.GetDataDirLabel(), `/\`) || strings.Contains(cp.GetDataDirLabel(), ":") {
		t.Fatalf("data_dir_label 不应含路径分隔符/盘符(要防用户名泄漏): %q", cp.GetDataDirLabel())
	}
	// 8 个域,次序与 id 全部钉死。
	if len(cp.GetDomains()) != len(wantDomainIDs) {
		t.Fatalf("应有 %d 个域,得到 %d 个", len(wantDomainIDs), len(cp.GetDomains()))
	}
	for i, want := range wantDomainIDs {
		got := cp.GetDomains()[i]
		if got.GetId() != want {
			t.Fatalf("第 %d 个域应为 %q,得到 %q", i, want, got.GetId())
		}
		if got.GetTitle() == "" || got.GetSummary() == "" {
			t.Fatalf("域 %s 的 title/summary 都必须由 daemon 给齐: %q / %q", want, got.GetTitle(), got.GetSummary())
		}
		if got.GetStatusCode() < 0 || got.GetStatusCode() > 2 {
			t.Fatalf("域 %s 的 status_code 越界: %d", want, got.GetStatusCode())
		}
	}
}

// TestControlPlaneBudgetMirrorsRuntimePolicy 覆盖验收点 ③:budget 数值与
// GetRuntimePolicy.effective_global 同源 —— 提交新策略后,两边必须一起变。
func TestControlPlaneBudgetMirrorsRuntimePolicy(t *testing.T) {
	srv := newControlPlaneTestServer(t)
	ctx := context.Background()

	quota := int64(123456)
	if _, err := srv.SetRuntimePolicy(ctx, &pb.SetRuntimePolicyRequest{
		Policy: &pb.RuntimePolicy{Global: &pb.RuntimeLimits{RamEventBufferBytes: &quota}},
	}); err != nil {
		t.Fatalf("提交预算策略失败: %v", err)
	}
	pol, err := srv.GetRuntimePolicy(ctx, &pb.GetRuntimePolicyRequest{})
	if err != nil {
		t.Fatalf("GetRuntimePolicy 失败: %v", err)
	}
	cp, err := srv.GetControlPlane(ctx, &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("GetControlPlane 失败: %v", err)
	}
	budget := findDomain(t, cp, "budget")
	if !budget.GetWritable() {
		t.Fatal("budget 域有写通道(SetRuntimePolicy),writable 必须为 true")
	}
	// 同源判据:控制面条目的 current 必须等于同一时刻 effective_global 的值
	// (字节走 formatBytes 的展示口径;数值走十进制字符串)。
	if got, want := findItem(t, budget, "budget.global.ram_event_buffer_bytes").GetCurrent(),
		formatBytes(pol.GetEffectiveGlobal().GetRamEventBufferBytes()); got != want {
		t.Fatalf("事件缓冲不同源: 控制面 %q vs 策略页 %q", got, want)
	}
	if got, want := findItem(t, budget, "budget.global.max_parallel_turns").GetCurrent(),
		fmt.Sprintf("%d", pol.GetEffectiveGlobal().GetMaxParallelTurns()); got != want {
		t.Fatalf("并行轮次不同源: 控制面 %q vs 策略页 %q", got, want)
	}
	// 七项数值域一项不缺 —— 缺一项,手机总览页就少一个可以就地改的额度。
	for _, want := range []string{
		"budget.global.ram_working_bytes", "budget.global.ram_event_buffer_bytes",
		"budget.global.ram_session_index_bytes", "budget.global.disk_conversation_bytes",
		"budget.global.disk_cache_bytes", "budget.global.disk_artifact_bytes",
		"budget.global.max_parallel_turns",
	} {
		it := findItem(t, budget, want)
		if it.GetCurrent() == "" {
			t.Fatalf("%s 的 current 不应为空", want)
		}
	}
}

// TestControlPlaneCollabMirrorsCollabPolicy 覆盖验收点 ④:collab 四项与
// GetCollabPolicy.effective 同源(用户提交后,总览页与协同策略页必须一致)。
func TestControlPlaneCollabMirrorsCollabPolicy(t *testing.T) {
	srv := newControlPlaneTestServer(t)
	ctx := context.Background()

	off := false
	members := int32(2)
	if _, err := srv.SetCollabPolicy(ctx, &pb.SetCollabPolicyRequest{
		Policy: &pb.CollabPolicy{
			CollaborationEnabled: &off,
			FanoutEnabled:        &off,
			MaxParallelMembers:   &members,
		},
	}); err != nil {
		t.Fatalf("提交协同策略失败: %v", err)
	}
	pol, err := srv.GetCollabPolicy(ctx, &pb.GetCollabPolicyRequest{})
	if err != nil {
		t.Fatalf("GetCollabPolicy 失败: %v", err)
	}
	cp, err := srv.GetControlPlane(ctx, &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("GetControlPlane 失败: %v", err)
	}
	collab := findDomain(t, cp, "collab")
	if !collab.GetWritable() {
		t.Fatal("collab 域有写通道(SetCollabPolicy),writable 必须为 true")
	}
	wantToggles := map[string]bool{
		"collab.collaboration_enabled":   pol.GetEffective().GetCollaborationEnabled(),
		"collab.native_delegate_enabled": pol.GetEffective().GetNativeDelegateEnabled(),
		"collab.fanout_enabled":          pol.GetEffective().GetFanoutEnabled(),
	}
	for id, wantOn := range wantToggles {
		got := findItem(t, collab, id).GetCurrent()
		want := "开"
		if !wantOn {
			want = "关"
		}
		if got != want {
			t.Fatalf("%s 不同源: 控制面 %q vs 策略页 %q", id, got, want)
		}
	}
	if got := findItem(t, collab, "collab.max_parallel_members").GetCurrent(); got != "2" {
		t.Fatalf("扇出成员上限不同源: 控制面 %q vs 策略页 %d", got, pol.GetEffective().GetMaxParallelMembers())
	}}

// TestBuildControlDomainDegradesOnPanic 覆盖验收点 ⑤:任何一个域的组装 panic
// 都只降级它自己 —— 总览页是聚合读,一个域的故障不该把整页掀翻。
func TestBuildControlDomainDegradesOnPanic(t *testing.T) {
	degraded := buildControlDomain("boom", "故障域", func() *pb.ControlDomain {
		panic("组装炸了")
	})
	if degraded == nil {
		t.Fatal("panic 后必须回一份降级域,而不是把 panic 冒给调用方")
	}
	if degraded.GetId() != "boom" || degraded.GetTitle() != "故障域" {
		t.Fatalf("降级域要保留 id/title(客户端才知道哪里坏了): %+v", degraded)
	}
	if degraded.GetStatusCode() != 2 {
		t.Fatalf("降级域 status_code 应为 2(受限),得到 %d", degraded.GetStatusCode())
	}
	if degraded.GetSummary() == "" {
		t.Fatal("降级域必须给一句如实说明")
	}
	// 反证:正常域经同一包装必须原样通过 —— 包装不能把好域也改了。
	normal := buildControlDomain("ok", "正常域", func() *pb.ControlDomain {
		return &pb.ControlDomain{Id: "ok", StatusCode: 0, Summary: "一切正常"}
	})
	if normal.GetStatusCode() != 0 || normal.GetSummary() != "一切正常" {
		t.Fatalf("包装改变了正常域的结果: %+v", normal)
	}
}

// TestControlPlaneDegradesWithoutBackend 覆盖验收点 ⑥:没有运行时后端的装配
// (极简测试路径)里,memory/dialysis/resources 如实降级,其余域照常。
func TestControlPlaneDegradesWithoutBackend(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{Store: st}) // 无 RuntimeBackend
	t.Cleanup(func() { _ = srv.events.Close() })

	cp, err := srv.GetControlPlane(context.Background(), &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("无后端时 GetControlPlane 也不该失败: %v", err)
	}
	if got := len(cp.GetDomains()); got != len(wantDomainIDs) {
		t.Fatalf("降级只能降域,不能丢域: 应有 %d 个,得到 %d 个", len(wantDomainIDs), got)
	}
	for _, id := range []string{"memory", "dialysis", "resources"} {
		d := findDomain(t, cp, id)
		if d.GetStatusCode() != 2 {
			t.Fatalf("无后端时 %s 域应降级为 status_code=2,得到 %d", id, d.GetStatusCode())
		}
		if d.GetSummary() == "" {
			t.Fatalf("降级域 %s 必须说明原因", id)
		}
	}
	// 不依赖后端的域照常给全条目 —— "少装后端"不是"少给预算页"的理由。
	for _, id := range []string{"budget", "collab", "transports"} {
		d := findDomain(t, cp, id)
		if len(d.GetItems()) == 0 {
			t.Fatalf("无后端时 %s 域仍应有条目", id)
		}
	}
}

// TestControlPlaneDialysisCountsNewEvents 覆盖验收点 ⑦:未知事件的 NEW 条数
// 如实呈现,确认(五处置之一)后跟着变。
func TestControlPlaneDialysisCountsNewEvents(t *testing.T) {
	srv := newControlPlaneTestServer(t)
	ctx := context.Background()

	srv.runtimeBackend.UnknownEvents().Record(runtime.EventEnvelope{SourceAgent: "codex", SourceMethod: "brandNew/one"})
	srv.runtimeBackend.UnknownEvents().Record(runtime.EventEnvelope{SourceAgent: "dsh", SourceMethod: "weird/two"})

	cp, err := srv.GetControlPlane(ctx, &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("GetControlPlane 失败: %v", err)
	}
	dialysis := findDomain(t, cp, "dialysis")
	if !dialysis.GetWritable() {
		t.Fatal("dialysis 域有写通道(AcknowledgeUnknownEvent 五处置),writable 必须为 true")
	}
	if got := findItem(t, dialysis, "dialysis.unread").GetCurrent(); !strings.Contains(got, "2") {
		t.Fatalf("两条 NEW 事件应计为 2,得到 %q", got)
	}
	if dialysis.GetStatusCode() != 1 {
		t.Fatalf("有待查看事件时 status_code 应为 1(有注意项),得到 %d", dialysis.GetStatusCode())
	}

	// 确认一条(IGNORED)→ 未读计数跟着变 —— 控制面读的是台账真值,不是快照。
	if _, err := srv.AcknowledgeUnknownEvent(ctx, &pb.AcknowledgeUnknownEventRequest{
		EventId: srv.runtimeBackend.UnknownEvents().List(runtime.ListFilter{Status: runtime.StatusNew}, 1)[0].ID,
		Action:  runtime.StatusIgnored,
	}); err != nil {
		t.Fatalf("确认事件失败: %v", err)
	}
	cp2, err := srv.GetControlPlane(ctx, &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("第二次 GetControlPlane 失败: %v", err)
	}
	if got := findItem(t, findDomain(t, cp2, "dialysis"), "dialysis.unread").GetCurrent(); !strings.Contains(got, "1") {
		t.Fatalf("确认一条后应只剩 1 条未读,得到 %q", got)
	}
}

// TestControlPlaneApprovalsCount 覆盖验收点 ⑧:挂起的审批请求如实计数。
func TestControlPlaneApprovalsCount(t *testing.T) {
	srv := newControlPlaneTestServer(t)
	ctx := context.Background()

	approvals := newTurnApprovals()
	srv.approvalSessions.Store("th-1", approvals)
	// 按真实挂起流程组装:add 登记等待通道,attach 落请求 —— 只 attach 不 add
	// 的请求 deliver 不了,计数就永远降不下去(真实路径见 thread.go 的审批挂起点)。
	approvals.add("req-1")
	approvals.attach(&pb.ApprovalRequest{RequestId: "req-1"})

	cp, err := srv.GetControlPlane(ctx, &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("GetControlPlane 失败: %v", err)
	}
	ap := findDomain(t, cp, "approvals")
	if got := findItem(t, ap, "approvals.pending").GetCurrent(); !strings.Contains(got, "1") {
		t.Fatalf("一条挂起审批应计为 1,得到 %q", got)
	}
	if ap.GetStatusCode() != 1 {
		t.Fatalf("有待处理的审批时 status_code 应为 1,得到 %d", ap.GetStatusCode())
	}

	// 决策送达后请求离开挂起集合 → 计数归零、状态回正常。
	approvals.deliver(&pb.ApprovalDecision{RequestId: "req-1"})
	cp2, err := srv.GetControlPlane(ctx, &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("第二次 GetControlPlane 失败: %v", err)
	}
	ap2 := findDomain(t, cp2, "approvals")
	if got := findItem(t, ap2, "approvals.pending").GetCurrent(); !strings.Contains(got, "0") {
		t.Fatalf("处理后应计数为 0,得到 %q", got)
	}
	if ap2.GetStatusCode() != 0 {
		t.Fatalf("无待处理审批时 status_code 应为 0,得到 %d", ap2.GetStatusCode())
	}
}

// ---- agents 域:codex 桌面偏好 select 项 --------------------------------------

// codexPrefsStub 是 agents 域桌面偏好的假 codex:只实现控制面需要的两个动态
// 候选源(生产对应 *agent.CodexProvider 的 model/list 与 configRequirements/read),
// 绝不拉真进程;其余 Provider 方法按测试件红线报错,被调到就响。
type codexPrefsStub struct {
	models          []pb.ModelInfo
	modelsErr       error
	requirements    agent.CodexRequirements
	requirementsErr error
}

func (p *codexPrefsStub) ID() string          { return "codex" }
func (p *codexPrefsStub) DisplayName() string { return "测试 codex" }
func (p *codexPrefsStub) Models() []pb.ModelInfo {
	return p.models
}
func (p *codexPrefsStub) Run(context.Context, agent.TurnRequest, agent.Sink, agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, errors.New("测试件不执行轮次")
}
func (p *codexPrefsStub) ModelsStatusCtx(context.Context) ([]pb.ModelInfo, error) {
	return p.models, p.modelsErr
}
func (p *codexPrefsStub) RequirementsCtx(context.Context) (agent.CodexRequirements, error) {
	return p.requirements, p.requirementsErr
}

// blockUntilCtxDone 把调用挂到 ctx 结束为止:钉"控制面拉候选受短预算约束"。
type blockUntilCtxDone struct{}

func (blockUntilCtxDone) ID() string          { return "codex" }
func (blockUntilCtxDone) DisplayName() string { return "阻塞 codex" }
func (blockUntilCtxDone) Models() []pb.ModelInfo {
	return nil
}
func (blockUntilCtxDone) Run(context.Context, agent.TurnRequest, agent.Sink, agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, errors.New("测试件不执行轮次")
}
func (blockUntilCtxDone) ModelsStatusCtx(ctx context.Context) ([]pb.ModelInfo, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (blockUntilCtxDone) RequirementsCtx(ctx context.Context) (agent.CodexRequirements, error) {
	<-ctx.Done()
	return agent.CodexRequirements{}, ctx.Err()
}

// newAgentsPrefsTestServer 建一台带 codex 假件的控制面测试机,并把桌面偏好
// 预置进 store —— 预置走的是 SetCodexSettings 落盘的**同一组 store 写口**,
// 这样 current 的同源断言验的是真实读写闭环,不是测试自说自话。
func newAgentsPrefsTestServer(t *testing.T, providers map[string]agent.Provider) *Server {
	t.Helper()
	dir := t.TempDir()
	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	if err := st.SetCodexPrefs(store.CodexPrefs{
		ApprovalPolicy:   "on-request",
		Sandbox:          "workspace-write",
		Verbosity:        "low",
		ReasoningSummary: "auto",
	}); err != nil {
		t.Fatalf("预置 codex 偏好失败: %v", err)
	}
	if err := st.SetDefaultModelID("codex/gpt-5.1"); err != nil {
		t.Fatalf("预置默认模型失败: %v", err)
	}
	if err := st.SetDefaultEffort("high"); err != nil {
		t.Fatalf("预置思考强度失败: %v", err)
	}
	srv := New(Config{
		Store:          st,
		Providers:      providers,
		DataDir:        dir,
		BudgetPolicy:   runtime.BudgetPolicy{Global: runtime.DefaultLimits()},
		CollabPolicy:   runtime.DefaultCollabPolicy(),
		RuntimeBackend: runtime.NewBackend(),
	})
	t.Cleanup(func() { _ = srv.events.Close() })
	return srv
}

// itemAbsent 反向断言:词条必须不在域里(宁缺毋假,不摆假入口)。
func itemAbsent(t *testing.T, d *pb.ControlDomain, id string) {
	t.Helper()
	for _, it := range d.GetItems() {
		if it.GetId() == id {
			t.Fatalf("域 %s 不应有条目 %q(实际: %v)", d.GetId(), id, d.GetItems())
		}
	}
}

// TestControlPlaneAgentsCodexSelectItems 覆盖验收点 ⑨:候选全动态、current 与
// GetCodexSettings 同值、无动态候选的两键不透出。
func TestControlPlaneAgentsCodexSelectItems(t *testing.T) {
	stub := &codexPrefsStub{
		models: []pb.ModelInfo{
			{Id: "default"},
			{Id: "gpt-5.1", AvailableEfforts: []string{"low", "high"}},
		},
		requirements: agent.CodexRequirements{
			Managed:                 true,
			AllowedApprovalPolicies: []string{"untrusted", "on-request", "never"},
			AllowedSandboxModes:     []string{"read-only", "workspace-write", "danger-full-access"},
		},
	}
	srv := newAgentsPrefsTestServer(t, map[string]agent.Provider{"codex": stub})
	ctx := context.Background()

	cp, err := srv.GetControlPlane(ctx, &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("GetControlPlane 失败: %v", err)
	}
	agents := findDomain(t, cp, "agents")
	// 同源基准:与 GetCodexSettings(同一把 store 读数)当场对账。
	st, err := srv.GetCodexSettings(ctx, &pb.GetCodexSettingsRequest{})
	if err != nil {
		t.Fatalf("GetCodexSettings 失败: %v", err)
	}

	cases := []struct {
		id      string
		current string
		options []string
	}{
		{"agents.codex.approval_policy", st.GetApprovalPolicy(), []string{"untrusted", "on-request", "never"}},
		{"agents.codex.sandbox", st.GetSandbox(), []string{"read-only", "workspace-write", "danger-full-access"}},
		{"agents.codex.model", st.GetModel(), []string{"codex/default", "codex/gpt-5.1"}},
	}
	for _, tc := range cases {
		it := findItem(t, agents, tc.id)
		if it.GetKind() != "select" {
			t.Fatalf("%s 应为 select,得到 %q", tc.id, it.GetKind())
		}
		if it.GetCurrent() != tc.current {
			t.Fatalf("%s 的 current 应与 GetCodexSettings 同值: %q != %q", tc.id, it.GetCurrent(), tc.current)
		}
		if len(it.GetOptions()) != len(tc.options) {
			t.Fatalf("%s 候选数应 %d,得到 %v", tc.id, len(tc.options), it.GetOptions())
		}
		for i, want := range tc.options {
			if it.GetOptions()[i] != want {
				t.Fatalf("%s 候选[%d] 应 %q,得到 %q", tc.id, i, want, it.GetOptions()[i])
			}
		}
		if it.GetHint() == "" || it.GetLabel() == "" {
			t.Fatalf("%s 的 label/hint 必须由 daemon 给齐: %q / %q", tc.id, it.GetLabel(), it.GetHint())
		}
	}
	// effort:当前默认模型(codex/gpt-5.1)申报了档位 → 候选就是它的档位,
	// current 与 GetCodexSettings.Effort 同值。
	effort := findItem(t, agents, "agents.codex.effort")
	if effort.GetKind() != "select" || effort.GetCurrent() != st.GetEffort() {
		t.Fatalf("effort 应为 select 且 current 同源: %+v vs %q", effort, st.GetEffort())
	}
	if got := effort.GetOptions(); len(got) != 2 || got[0] != "low" || got[1] != "high" {
		t.Fatalf("effort 候选应为当前模型申报的档位,得到 %v", got)
	}
	// model_verbosity / model_reasoning_summary:codex 无枚举查询,候选拿不到
	// 就整项不透出 —— 宁缺毋假,不写死名单冒充候选。
	itemAbsent(t, agents, "agents.codex.model_verbosity")
	itemAbsent(t, agents, "agents.codex.model_reasoning_summary")
	// store 里的 verbosity/summary 有值也不该漏成 select:词表六键只透出拿得到
	// 动态候选的四键。
}

// TestControlPlaneAgentsSelectsSkipWithoutCandidates 覆盖验收点 ⑩:候选源失败
// (模型列表报错 / 未托管白名单为空)时,对应 select 整项缺席,形态 info 项照旧,
// 域不因此降级 —— 没有可选值的编辑入口是假入口。
func TestControlPlaneAgentsSelectsSkipWithoutCandidates(t *testing.T) {
	// 模型列表失败 + 未托管(白名单全空)→ 六键一个都不摆。
	srv := newAgentsPrefsTestServer(t, map[string]agent.Provider{"codex": &codexPrefsStub{
		modelsErr: agent.ErrModelsInitializing,
	}})
	cp, err := srv.GetControlPlane(context.Background(), &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("GetControlPlane 失败: %v", err)
	}
	agents := findDomain(t, cp, "agents")
	for _, it := range agents.GetItems() {
		if it.GetKind() == "select" {
			t.Fatalf("候选拿不到时不应有 select 项: %+v", it)
		}
	}
	// 形态事实项照常在场(info 不依赖动态候选)。
	findItem(t, agents, "agents.codex.session")
	findItem(t, agents, "agents.codex.capabilities")

	// 模型列表正常 + 托管策略读不到 → 只摆 model/effort,approval/sandbox 缺席。
	srv2 := newAgentsPrefsTestServer(t, map[string]agent.Provider{"codex": &codexPrefsStub{
		models:         []pb.ModelInfo{{Id: "gpt-5.1", AvailableEfforts: []string{"low"}}},
		requirementsErr: errors.New("codex 未响应"),
	}})
	cp2, err := srv2.GetControlPlane(context.Background(), &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("第二次 GetControlPlane 失败: %v", err)
	}
	agents2 := findDomain(t, cp2, "agents")
	findItem(t, agents2, "agents.codex.model")
	findItem(t, agents2, "agents.codex.effort")
	itemAbsent(t, agents2, "agents.codex.approval_policy")
	itemAbsent(t, agents2, "agents.codex.sandbox")
}

// TestControlPlaneAgentsNoCodexNoSelects 覆盖验收点 ⑩(codex 未注册的降级):
// agents 域照常给已注册 agent 的形态事实,不出现任何 codex 条目,更不假造 select。
func TestControlPlaneAgentsNoCodexNoSelects(t *testing.T) {
	srv := newControlPlaneTestServer(t) // 只有 echo
	cp, err := srv.GetControlPlane(context.Background(), &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("GetControlPlane 失败: %v", err)
	}
	agents := findDomain(t, cp, "agents")
	if agents.GetStatusCode() == 2 {
		t.Fatalf("codex 未注册只是没有偏好可选,域本身不该降级: %+v", agents)
	}
	for _, it := range agents.GetItems() {
		if strings.HasPrefix(it.GetId(), "agents.codex.") {
			t.Fatalf("codex 未注册不应有 codex 条目: %+v", it)
		}
		if it.GetKind() == "select" {
			t.Fatalf("非 codex agent 不走桌面偏好,不应有 select 项: %+v", it)
		}
	}
	findItem(t, agents, "agents.echo.session")
}

// TestControlPlaneAgentsPrefsBudgetBounded 覆盖验收点 ⑪:codex 候选源挂死时,
// 拉候选必须被短预算截断、对应 select 跳过 —— 总览页是聚合读,一个 agent 的
// 设置目录不能把整页八个域拖住。
func TestControlPlaneAgentsPrefsBudgetBounded(t *testing.T) {
	srv := newAgentsPrefsTestServer(t, map[string]agent.Provider{"codex": blockUntilCtxDone{}})
	start := time.Now()
	cp, err := srv.GetControlPlane(context.Background(), &pb.GetControlPlaneRequest{})
	if err != nil {
		t.Fatalf("GetControlPlane 失败: %v", err)
	}
	if elapsed := time.Since(start); elapsed > agentsPrefsBudget+2*time.Second {
		t.Fatalf("codex 候选拉取应被 %v 预算截断,实际耗时 %v", agentsPrefsBudget, elapsed)
	}
	agents := findDomain(t, cp, "agents")
	for _, it := range agents.GetItems() {
		if it.GetKind() == "select" {
			t.Fatalf("候选超时的键应整项跳过,不应出现 select: %+v", it)
		}
	}
	// 其余域照常返回(聚合读不被单域拖垮的整页级证据)。
	findDomain(t, cp, "budget")
	findDomain(t, cp, "transports")
}

// TestCodexPrefsOptionHelpers 纯函数钉子:模型候选补前缀/去重保序;effort 候选
// 优先当前模型申报档,缺失时并集兜底,无人申报时整项放弃。
func TestCodexPrefsOptionHelpers(t *testing.T) {
	models := []pb.ModelInfo{
		{Id: "default"},
		{Id: "gpt-5.1", AvailableEfforts: []string{"low", "high"}},
		{Id: "gpt-5.1"}, // 重复 id:去重保序
		{Id: "o4-mini", AvailableEfforts: []string{"high", "minimal"}},
	}
	got := codexModelOptions(models)
	want := []string{"codex/default", "codex/gpt-5.1", "codex/o4-mini"}
	if len(got) != len(want) {
		t.Fatalf("模型候选应 %v,得到 %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("模型候选[%d] 应 %q,得到 %q(保序去重)", i, want[i], got[i])
		}
	}
	// 当前模型申报了档位 → 就用它的(档位随模型走,摆别家的档是骗人)。
	if got := codexEffortOptions(models, nil, "codex/gpt-5.1"); len(got) != 2 || got[0] != "low" {
		t.Fatalf("当前模型申报档位应原样透出,得到 %v", got)
	}
	// 当前是 codex/default(跟随桌面,尚无具体模型)→ 全部模型档位并集,首见序。
	if got := codexEffortOptions(models, nil, "codex/default"); len(got) != 3 || got[0] != "low" || got[2] != "minimal" {
		t.Fatalf("无具体模型时应并集兜底,得到 %v", got)
	}
	// 列表失败 / 无人申报档位 → nil(调用方整项跳过)。
	if got := codexEffortOptions(models, errors.New("boom"), ""); got != nil {
		t.Fatalf("列表失败应返回 nil,得到 %v", got)
	}
	if got := codexEffortOptions([]pb.ModelInfo{{Id: "default"}}, nil, ""); got != nil {
		t.Fatalf("无人申报档位应返回 nil,得到 %v", got)
	}
}
