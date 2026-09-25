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
//	⑧ approvals 域如实计数挂起的审批请求。
package server

import (
	"context"
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
