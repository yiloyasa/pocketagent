// collab_delegate_rpc_test.go —— DelegateSubTask RPC 的端到端测试(T-L2fix
// 单写者重构)。经 bufconn 起真实 gRPC 服务端(挂既有 UnaryAuth 拦截器),
// 钉四条链路:echo agent 委派全链路(RPC → 单例引擎 → CreateThread →
// runTurnOnce → 落库)、未知 agent 人话报错走 error 字段、深度拒绝(同设备
// 在飞 ≥1 拒绝 + 跨设备共享 depth 闸)、未鉴权拒绝(拦截器 Unauthenticated)。
//
// 【为什么必须过真拦截器】薄代理子模式的生产调用链就是"回环 gRPC + Bearer
// token → UnaryAuth → handler",直接调方法会绕过鉴权闸 —— 鉴权拒绝这条
// 红线必须在装配层钉住。
package server

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// collabRPCPairCode 测试服务器的配对码;legacy 单 token 由它导出
// (server.LegacyTokenFor),与生产 -collab-mcp 代理的取值链一致。
const collabRPCPairCode = "654321"

// newCollabRPCServer 起 bufconn gRPC 服务端(鉴权拦截器 + 真实委派引擎),
// 返回客户端与清理函数。
func newCollabRPCServer(t *testing.T, providers map[string]agent.Provider) (pb.ThreadServiceClient, *Server, func()) {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	srv := New(Config{
		Store:     fs,
		Providers: providers,
		PairCode:  collabRPCPairCode,
		Workspace: t.TempDir(),
	})
	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(srv.UnaryAuth()))
	pb.RegisterThreadServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		gs.Stop()
		t.Fatalf("建 bufconn 客户端失败: %v", err)
	}
	cleanup := func() {
		_ = conn.Close()
		gs.Stop()
	}
	return pb.NewThreadServiceClient(conn), srv, cleanup
}

// clientCtxWithBearer 客户端侧 Authorization 头(与服务端 incoming 对偶)。
func clientCtxWithBearer(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}

// TestDelegateSubTaskEchoFullChain 主链路:legacy token 委派 echo → 成功应答
// (child_thread_id / summary / state=idle / error 空)→ 子线程落库与直调引擎
// 同源(user+assistant、agent 归属、终态 IDLE)→ 单例引擎登记簿打标。
func TestDelegateSubTaskEchoFullChain(t *testing.T) {
	client, srv, cleanup := newCollabRPCServer(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	defer cleanup()

	resp, err := client.DelegateSubTask(clientCtxWithBearer(LegacyTokenFor(collabRPCPairCode)),
		&pb.DelegateSubTaskRequest{AgentId: "echo", Task: "rpc 委派看日志", Effort: "high"})
	if err != nil {
		t.Fatalf("委派 RPC 失败: %v", err)
	}
	if resp.GetError() != "" {
		t.Fatalf("成功委派不应带 error: %q", resp.GetError())
	}
	if !strings.HasPrefix(resp.GetChildThreadId(), "th-") {
		t.Fatalf("child_thread_id 异常: %q", resp.GetChildThreadId())
	}
	if !strings.Contains(resp.GetSummary(), "rpc 委派看日志") {
		t.Fatalf("summary 不含 echo 终稿: %q", resp.GetSummary())
	}
	if resp.GetState() != "idle" {
		t.Fatalf("state 应为 idle: %q", resp.GetState())
	}

	// 落库同源:与手机轮次/直调引擎完全同一条 CreateThread + runTurnOnce 路径。
	got, ok := srv.cfg.Store.Get(resp.GetChildThreadId())
	if !ok {
		t.Fatal("子线程没有落库")
	}
	if got.AgentID != "echo" || got.Status != pb.ThreadStatus_THREAD_STATUS_IDLE || got.Effort != "high" {
		t.Fatalf("子线程落库不符: agent=%s status=%v effort=%q", got.AgentID, got.Status, got.Effort)
	}
	if len(got.Messages) != 2 || got.Messages[0].Role != "user" || got.Messages[1].Role != "assistant" {
		t.Fatalf("子线程消息序不符: %d 条", len(got.Messages))
	}

	// 单例:handler 用的引擎与 collabEngineFor 返回的是同一实例 —— 跨进程
	// 深度记账全局成立的前提(white-box)。
	if srv.collabEngineFor() != srv.collabEngineFor() {
		t.Fatal("collabEngineFor 必须返回同一单例")
	}
	ce := srv.collabEngineFor()
	ce.mu.Lock()
	marked := ce.childThreads[resp.GetChildThreadId()]
	ce.mu.Unlock()
	if !marked {
		t.Fatalf("子线程 %s 没有打来源标记", resp.GetChildThreadId())
	}
}

// TestDelegateSubTaskUnknownAgent 未知 agent → 业务失败走 error 字段 + 人话
// (动态列举可用 agent),不靠 gRPC status 承载,也不留半截子线程。
func TestDelegateSubTaskUnknownAgent(t *testing.T) {
	client, srv, cleanup := newCollabRPCServer(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	defer cleanup()

	resp, err := client.DelegateSubTask(clientCtxWithBearer(LegacyTokenFor(collabRPCPairCode)),
		&pb.DelegateSubTaskRequest{AgentId: "ghost", Task: "任务"})
	if err != nil {
		t.Fatalf("业务失败不应走 gRPC status(与既有风格对齐): %v", err)
	}
	if resp.GetError() == "" {
		t.Fatal("未知 agent 必须在 error 字段人话报错")
	}
	if !strings.Contains(resp.GetError(), "电脑上没有可用的 ghost agent") || !strings.Contains(resp.GetError(), "echo") {
		t.Fatalf("报错文案不符(应人话 + 动态列举): %q", resp.GetError())
	}
	if resp.GetChildThreadId() != "" || resp.GetSummary() != "" {
		t.Fatalf("失败不得伪造子线程/摘要: %+v", resp)
	}
	if threads := srv.cfg.Store.List(true); len(threads) != 0 {
		t.Fatalf("失败委派不应创建子线程: %d 条", len(threads))
	}
}

// TestDelegateSubTaskDepthRejectedSameDevice 深度锁跨进程语义:同一发起方
// (同一 token/device)在飞委派 ≥1 → 第二次委派被拒("子不生子"),拒绝不建
// 子线程;第一个委派放行后正常完成、计数归零,后续委派恢复可用。
func TestDelegateSubTaskDepthRejectedSameDevice(t *testing.T) {
	gate := &gateProvider{entered: make(chan struct{}), release: make(chan struct{})}
	client, srv, cleanup := newCollabRPCServer(t, map[string]agent.Provider{"gate": gate})
	defer cleanup()
	tok := LegacyTokenFor(collabRPCPairCode)

	done := make(chan struct{})
	var firstResp *pb.DelegateSubTaskResponse
	go func() {
		defer close(done)
		firstResp, _ = client.DelegateSubTask(clientCtxWithBearer(tok),
			&pb.DelegateSubTaskRequest{AgentId: "gate", Task: "外层在飞"})
	}()
	select {
	case <-gate.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("第一个委派未进入 provider.Run")
	}

	// 在飞期间同一设备再来一次:拒绝(在飞 ≥1),且未建任何新子线程。
	rej, err := client.DelegateSubTask(clientCtxWithBearer(tok),
		&pb.DelegateSubTaskRequest{AgentId: "gate", Task: "嵌套委派"})
	if err != nil {
		t.Fatalf("深度拒绝应走 error 字段而非 gRPC status: %v", err)
	}
	if !strings.Contains(rej.GetError(), "不能再委派") {
		t.Fatalf("拒绝文案应如实说明深度锁: %q", rej.GetError())
	}
	if rej.GetChildThreadId() != "" {
		t.Fatalf("被拒委派不得有子线程: %q", rej.GetChildThreadId())
	}

	close(gate.release)
	<-done
	if firstResp.GetError() != "" {
		t.Fatalf("外层委派应正常完成: %q", firstResp.GetError())
	}
	if n := len(srv.cfg.Store.List(true)); n != 1 {
		t.Fatalf("全程只应有外层 1 条子线程,实际 %d 条", n)
	}
	// 归零即清键:登记簿不积累(white-box)。
	ce := srv.collabEngineFor()
	ce.mu.Lock()
	inflight := len(ce.inflightByDevice)
	depth := ce.depth
	ce.mu.Unlock()
	if inflight != 0 || depth != 0 {
		t.Fatalf("委派完成后在飞计数应归零: devices=%d depth=%d", inflight, depth)
	}
}

// TestDelegateSubTaskDepthRejectedAcrossDevice 共享 depth 闸:设备 A 的委派
// 在飞期间,设备 B(已配对、token 合法)的委派同样被拒 —— 所有代理进程汇入
// 单引擎后,全局深度对任意连接可见(保守面如实说明,由 coordinator 择机重试)。
func TestDelegateSubTaskDepthRejectedAcrossDevice(t *testing.T) {
	gate := &gateProvider{entered: make(chan struct{}), release: make(chan struct{})}
	client, srv, cleanup := newCollabRPCServer(t, map[string]agent.Provider{"gate": gate})
	defer cleanup()

	pub, _ := testKey(t)
	auth, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method:       &pb.AuthorizeRequest_PairCode{PairCode: collabRPCPairCode},
		DeviceId:     "dev-proxy-b",
		DevicePubKey: pub,
	})
	if err != nil {
		t.Fatalf("注册第二设备失败: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = client.DelegateSubTask(clientCtxWithBearer(LegacyTokenFor(collabRPCPairCode)),
			&pb.DelegateSubTaskRequest{AgentId: "gate", Task: "设备A在飞"})
	}()
	select {
	case <-gate.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("设备A委派未进入 provider.Run")
	}

	rej, err := client.DelegateSubTask(clientCtxWithBearer(auth.Token),
		&pb.DelegateSubTaskRequest{AgentId: "gate", Task: "设备B趁飞而入"})
	if err != nil {
		t.Fatalf("深度拒绝应走 error 字段: %v", err)
	}
	if !strings.Contains(rej.GetError(), "不能再委派") {
		t.Fatalf("跨设备深度拒绝文案不符: %q", rej.GetError())
	}
	close(gate.release)
	<-done
}

// TestDelegateSubTaskUnauthenticated 鉴权红线:无 token / 伪造 token 一律
// Unauthenticated —— 走的是装配层真拦截器(UnaryAuth → checkAccess →
// authorized),已配对设备才可委派。
func TestDelegateSubTaskUnauthenticated(t *testing.T) {
	client, _, cleanup := newCollabRPCServer(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	defer cleanup()

	cases := map[string]context.Context{
		"无token":  context.Background(),
		"伪造token": clientCtxWithBearer("forged-token"),
	}
	for name, ctx := range cases {
		_, err := client.DelegateSubTask(ctx, &pb.DelegateSubTaskRequest{AgentId: "echo", Task: "任务"})
		if err == nil || status.Code(err) != codes.Unauthenticated {
			t.Fatalf("%s: 应 Unauthenticated,得到 %v", name, err)
		}
	}
}
