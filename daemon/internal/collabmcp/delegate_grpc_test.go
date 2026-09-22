// delegate_grpc_test.go —— GRPCDelegationEngine 薄代理引擎的包内测试(T-L2fix)。
//
// 与 collabmcp_test.go 的 fakeEngine 互补:那边钉工具层协议语义(排队/截断/
// 错误透传),这边钉"工具层 → gRPC 客户端引擎 → DelegateSubTask"这条新调用链:
// 应答 error 字段 → 工具级错误(人话原样透传,不代拟成功)、gRPC 层错误 →
// 原样上抛、StatusOf 只答经手的委派(代理模式无线程库访问权,诚实口径)、
// token 注入与 workspace 默认值填充。gRPC 客户端接口用包内 fake 直测,
// 不起网络(拨号/TLS 装配属 cmd 层接线)。
package collabmcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// fakeThreadClient 可注入的 ThreadServiceClient 假实现:只覆写 DelegateSubTask,
// 其余方法不会被薄代理调用(内嵌接口,误调用显式 panic 优于静默错行为)。
type fakeThreadClient struct {
	pb.ThreadServiceClient

	resp *pb.DelegateSubTaskResponse
	err  error

	// 捕获的请求与出站 metadata(钉 Bearer 注入与字段透传)。
	lastReq *pb.DelegateSubTaskRequest
	lastMD  metadata.MD
}

func (f *fakeThreadClient) DelegateSubTask(ctx context.Context, in *pb.DelegateSubTaskRequest, _ ...grpc.CallOption) (*pb.DelegateSubTaskResponse, error) {
	f.lastReq = in
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		f.lastMD = md
	}
	return f.resp, f.err
}

// newGRPCServer 构造挂薄代理引擎的 collab MCP 服务器,返回主干与 fake。
func newGRPCServer(t *testing.T, fc *fakeThreadClient, token, defaultWS string) (*Server, *fakeThreadClient) {
	t.Helper()
	s := New(NewGRPCEngine(fc, token, defaultWS), "test")
	return s, fc
}

// TestGRPCProxyDelegateSuccess 成功链路:请求字段原样透传(agent_id/task/
// workspace/model/effort),Bearer token 注入出站 metadata;应答转成工具
// 结果文本(含子线程 id 与摘要,有界)。
func TestGRPCProxyDelegateSuccess(t *testing.T) {
	fc := &fakeThreadClient{resp: &pb.DelegateSubTaskResponse{
		ChildThreadId: "th-child-1", Summary: "子任务终稿", State: "idle",
	}}
	s, fc := newGRPCServer(t, fc, "tok-123", "")

	text, isErr := s.runDelegate(context.Background(), delegateArgs{
		AgentID: "echo", Task: "帮查日志", Workspace: "G:/ws", Model: "echo/m1", Effort: "high",
	}, nil)
	if isErr {
		t.Fatalf("成功委派不应是工具级错误: %s", text)
	}
	if !strings.Contains(text, "th-child-1") || !strings.Contains(text, "子任务终稿") {
		t.Fatalf("工具结果应含子线程 id 与摘要: %q", text)
	}
	if fc.lastReq.GetAgentId() != "echo" || fc.lastReq.GetTask() != "帮查日志" ||
		fc.lastReq.GetWorkspace() != "G:/ws" || fc.lastReq.GetModel() != "echo/m1" || fc.lastReq.GetEffort() != "high" {
		t.Fatalf("请求字段未原样透传: %+v", fc.lastReq)
	}
	if got := fc.lastMD.Get("authorization"); len(got) != 1 || got[0] != "Bearer tok-123" {
		t.Fatalf("Bearer token 未注入出站 metadata: %v", fc.lastMD)
	}

	// StatusOf:经手的委派 → 缓存的返回时点状态(idle)。
	st, err := s.engine.StatusOf("th-child-1")
	if err != nil || st.State != "idle" {
		t.Fatalf("经手线程应答出缓存状态: %+v err=%v", st, err)
	}
}

// TestGRPCProxyParentThreadIDPassthrough T-S1Pa:可选父链的链路透传钉子。
// 工具入参 parent_thread_id → DelegateOptions → DelegateSubTask 请求(proto
// 字段 6);缺省空 = 请求字段为空(不补默认、不伪造)—— 主 daemon 侧据此
// 维持"父链为空,卡不亮不误报"。pi 侧 v1 恒走空路径(见 engine.go 注释)。
func TestGRPCProxyParentThreadIDPassthrough(t *testing.T) {
	fc := &fakeThreadClient{resp: &pb.DelegateSubTaskResponse{ChildThreadId: "th-c", State: "idle"}}
	s, fc := newGRPCServer(t, fc, "tok", "")

	if _, isErr := s.runDelegate(context.Background(), delegateArgs{
		AgentID: "echo", Task: "带父链", ParentThreadID: "th-parent-9",
	}, nil); isErr {
		t.Fatal("带父链委派不应失败")
	}
	if got := fc.lastReq.GetParentThreadId(); got != "th-parent-9" {
		t.Fatalf("parent_thread_id 未透传到请求字段 6: %q", got)
	}

	// 缺省空:不补默认。
	if _, isErr := s.runDelegate(context.Background(), delegateArgs{AgentID: "echo", Task: "无父链"}, nil); isErr {
		t.Fatal("无父链委派不应失败")
	}
	if got := fc.lastReq.GetParentThreadId(); got != "" {
		t.Fatalf("缺省时请求父链应为空: %q", got)
	}
}

// TestGRPCProxyDelegateDefaultWorkspace 工具调用未给 workspace → 填引擎的
// defaultWorkspace(对齐旧子模式 Config.Workspace 语义);显式给 → 原样透传。
func TestGRPCProxyDelegateDefaultWorkspace(t *testing.T) {
	fc := &fakeThreadClient{resp: &pb.DelegateSubTaskResponse{ChildThreadId: "th-d", State: "idle"}}
	s, fc := newGRPCServer(t, fc, "", "G:/default-ws")

	if _, isErr := s.runDelegate(context.Background(), delegateArgs{AgentID: "echo", Task: "t"}, nil); isErr {
		t.Fatal("委派不应失败")
	}
	if fc.lastReq.GetWorkspace() != "G:/default-ws" {
		t.Fatalf("空 workspace 应填默认值: %q", fc.lastReq.GetWorkspace())
	}
}

// TestGRPCProxyDelegateBusinessError 应答 error 字段 = 业务失败:工具级错误
// + 人话原样透传(准则 6:不代拟成功、不改写);子线程 id 仍进缓存,agent_status
// 如实回 failed + 原始错误。
func TestGRPCProxyDelegateBusinessError(t *testing.T) {
	fc := &fakeThreadClient{resp: &pb.DelegateSubTaskResponse{
		ChildThreadId: "th-err-1", State: "", Error: "电脑上没有可用的 ghost agent",
	}}
	s, _ := newGRPCServer(t, fc, "tok", "")

	text, isErr := s.runDelegate(context.Background(), delegateArgs{AgentID: "ghost", Task: "t"}, nil)
	if !isErr {
		t.Fatalf("业务失败必须是工具级错误: %s", text)
	}
	if !strings.Contains(text, "电脑上没有可用的 ghost agent") {
		t.Fatalf("错误应人话原样透传: %q", text)
	}
	st, err := s.engine.StatusOf("th-err-1")
	if err != nil || st.State != "failed" || !strings.Contains(st.Detail, "ghost") {
		t.Fatalf("失败线程缓存状态不符: %+v err=%v", st, err)
	}
}

// TestGRPCProxyDelegateTransportError gRPC 层错误(鉴权/传输)原样上抛,
// 不吞错、不代拟结果。
func TestGRPCProxyDelegateTransportError(t *testing.T) {
	rpcErr := status.Error(codes.Unauthenticated, "token 无效,请先配对")
	fc := &fakeThreadClient{err: rpcErr}
	s, _ := newGRPCServer(t, fc, "bad-token", "")

	_, _, err := s.engine.Delegate(context.Background(), "echo", "t", DelegateOptions{})
	if !errors.Is(err, rpcErr) {
		t.Fatalf("gRPC 层错误应原样上抛: %v", err)
	}
	text, isErr := s.runDelegate(context.Background(), delegateArgs{AgentID: "echo", Task: "t"}, nil)
	if !isErr || !strings.Contains(text, "token 无效") {
		t.Fatalf("工具结果应含传输层错误原文: %q isErr=%v", text, isErr)
	}
}

// TestGRPCProxyStatusOfUnknownThread 未经手的线程如实报错 —— 代理模式没有
// 线程库访问权,绝不编造状态(准则 6)。
func TestGRPCProxyStatusOfUnknownThread(t *testing.T) {
	fc := &fakeThreadClient{}
	s, _ := newGRPCServer(t, fc, "", "")
	if _, err := s.engine.StatusOf("th-never-seen"); err == nil {
		t.Fatal("未经手的线程必须报错,不得编造状态")
	} else if !strings.Contains(err.Error(), "th-never-seen") {
		t.Fatalf("报错应指名线程: %v", err)
	}
}

// TestGRPCProxyEmptyTokenStillCalls 空 token:仍照常发 RPC(不伪造鉴权),
// 由主 daemon 如实拒绝 —— 断言出站无 Authorization 头、错误原样透传。
func TestGRPCProxyEmptyTokenStillCalls(t *testing.T) {
	rpcErr := status.Error(codes.Unauthenticated, "token 无效,请先配对")
	fc := &fakeThreadClient{err: rpcErr}
	s, fc := newGRPCServer(t, fc, "", "")

	_, isErr := s.runDelegate(context.Background(), delegateArgs{AgentID: "echo", Task: "t"}, nil)
	if !isErr {
		t.Fatal("无 token 被拒必须是工具级错误")
	}
	if _, ok := fc.lastMD["authorization"]; ok {
		t.Fatalf("空 token 不应注入 Authorization 头: %v", fc.lastMD)
	}
}
