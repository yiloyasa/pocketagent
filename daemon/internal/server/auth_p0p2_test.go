package server

// P0-2 端到端验收:经 Server.Authorize 四分支 + authorized 拦截器全链路。
// (devices_test.go 锁的是注册表本体的单元行为,这里锁 gRPC 层的装配正确性:
// oneof 分派、Bearer 解析、撤销后的 PERMISSION_DENIED。)
//
// 【PA-REAL-0001 追加】鉴权默认开启后的回归资产:
//   - 空 pair code 不再等于"关闭鉴权",一律 fail closed;
//   - 任务①:自动生成的配对码激活鉴权(未配对被拒,配对成功放行);
//   - 任务③:-insecure-no-auth 仅放行回环,非回环(含缺 peer 信息)一律拒绝;
//   - GetHostInfo.auth_required 如实反映两种模式。
//   任务②(pair-code.txt 复用不换码)在 cmd/pocketagentd/paircode_test.go;
//   任务④(错误配对码被拒)由 TestAuthorizeLegacyPairCodeStillWorks 与
//   TestAuthorizePairRegistersDeviceAndTokenAuthorizes 的伪造 token 断言覆盖。

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// newAuthTestServer 起一个启用鉴权的 Server(内存设备表)。
func newAuthTestServer(t *testing.T) *Server {
	t.Helper()
	srv := &Server{
		cfg: Config{PairCode: "654321"},
	}
	srv.devices = newDeviceRegistry("", "654321")
	srv.hostID = "host-test"
	srv.activeTurns = newActiveTurns()
	return srv
}

// ctxWithBearer 构造带 Authorization 头的入站 context。
func ctxWithBearer(token string) context.Context {
	md := metadata.Pairs("authorization", "Bearer "+token)
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestAuthorizePairRegistersDeviceAndTokenAuthorizes(t *testing.T) {
	srv := newAuthTestServer(t)
	pub, _ := testKey(t)
	resp, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method:       &pb.AuthorizeRequest_PairCode{PairCode: "654321"},
		DeviceName:   "Pixel 9",
		DeviceId:     "dev-e2e",
		DevicePubKey: pub,
	})
	if err != nil {
		t.Fatalf("配对注册: %v", err)
	}
	if resp.Token == "" || resp.DeviceId != "dev-e2e" {
		t.Fatalf("应返回每设备 token 与设备 ID 回显: %+v", resp)
	}
	// 每设备 token 必须过拦截器
	if err := srv.authorized(ctxWithBearer(resp.Token)); err != nil {
		t.Fatalf("设备 token 应放行: %v", err)
	}
	// 错误 token 必须拒绝
	if err := srv.authorized(ctxWithBearer("forged")); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("伪造 token 应 Unauthenticated,得到 %v", err)
	}
}

func TestAuthorizeLegacyPairCodeStillWorks(t *testing.T) {
	srv := newAuthTestServer(t)
	resp, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method: &pb.AuthorizeRequest_PairCode{PairCode: "654321"},
	})
	if err != nil {
		t.Fatalf("旧路径配对: %v", err)
	}
	// 旧路径返回 sha256("pair-code:654321") 的 hex,拦截器同样放行
	if err := srv.authorized(ctxWithBearer(resp.Token)); err != nil {
		t.Fatalf("legacy token 应放行: %v", err)
	}
	// 错配对码拒绝
	if _, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method: &pb.AuthorizeRequest_PairCode{PairCode: "999999"},
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("错配对码应 PermissionDenied,得到 %v", err)
	}
}

func TestAuthorizeTokenRenewalBranch(t *testing.T) {
	srv := newAuthTestServer(t)
	pub, _ := testKey(t)
	reg, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method:       &pb.AuthorizeRequest_PairCode{PairCode: "654321"},
		DeviceId:     "dev-r",
		DevicePubKey: pub,
	})
	if err != nil {
		t.Fatalf("注册: %v", err)
	}
	// token 分支:有效 → 原样续期
	renewed, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method: &pb.AuthorizeRequest_Token{Token: reg.Token},
	})
	if err != nil {
		t.Fatalf("续期: %v", err)
	}
	if renewed.Token != reg.Token {
		t.Fatalf("有效 token 续期应原样返回")
	}
	// 无效 token → 拒绝
	if _, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method: &pb.AuthorizeRequest_Token{Token: "junk"},
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("无效 token 续期应 PermissionDenied,得到 %v", err)
	}
}

func TestAuthorizeChallengeFlowEndToEnd(t *testing.T) {
	srv := newAuthTestServer(t)
	pub, sign := testKey(t)
	if _, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method:       &pb.AuthorizeRequest_PairCode{PairCode: "654321"},
		DeviceId:     "dev-ch",
		DevicePubKey: pub,
	}); err != nil {
		t.Fatalf("注册: %v", err)
	}
	// 1) 要挑战
	req, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method:   &pb.AuthorizeRequest_ChallengeRequest{ChallengeRequest: "dev-ch"},
		DeviceId: "dev-ch",
	})
	if err != nil {
		t.Fatalf("要挑战: %v", err)
	}
	if req.Challenge == "" {
		t.Fatalf("应返回挑战数")
	}
	// 2) Keystore 式签名(签名内容 = device_id:challenge)
	sig := base64.StdEncoding.EncodeToString(sign([]byte("dev-ch:" + req.Challenge)))
	ans, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method: &pb.AuthorizeRequest_ChallengeAnswer{ChallengeAnswer: &pb.ChallengeAnswer{
			Challenge: req.Challenge,
			Signature: sig,
		}},
		DeviceId: "dev-ch",
	})
	if err != nil {
		t.Fatalf("挑战应答: %v", err)
	}
	if ans.Token == "" {
		t.Fatalf("应返回新 token")
	}
	if err := srv.authorized(ctxWithBearer(ans.Token)); err != nil {
		t.Fatalf("新 token 应放行: %v", err)
	}
	// 3) 撤销后同 token → PERMISSION_DENIED(拦截器返回 Unauthenticated)
	if err := srv.devices.Revoke("dev-ch"); err != nil {
		t.Fatalf("撤销: %v", err)
	}
	if err := srv.authorized(ctxWithBearer(ans.Token)); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("撤销后应 Unauthenticated,得到 %v", err)
	}
	// 4) 未注册设备要挑战 → NotFound
	if _, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method:   &pb.AuthorizeRequest_ChallengeRequest{ChallengeRequest: "dev-404"},
		DeviceId: "dev-404",
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("未注册设备要挑战应 NotFound,得到 %v", err)
	}
}

// 【PA-REAL-0001 回归①】空 pair code 不再等于"关闭鉴权":拦截器 fail closed,
// Authorize 拒绝配对,绝不允许签出可用的"空 token"。(旧版此场景全放行 = P0 根因。)
func TestEmptyPairCodeFailsClosed(t *testing.T) {
	srv := &Server{cfg: Config{PairCode: ""}}
	srv.devices = newDeviceRegistry("", "")
	// 带任何 token / 不带凭据,一律拒绝
	if err := srv.authorized(ctxWithBearer("anything")); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("空 pair code 应 fail closed(Unauthenticated),得到 %v", err)
	}
	if err := srv.authorized(context.Background()); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("空 pair code 无凭据应 Unauthenticated,得到 %v", err)
	}
	// 配对请求同样被拒(旧版这里会返回空 token)
	if _, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method: &pb.AuthorizeRequest_PairCode{PairCode: "whatever"},
	}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("空 pair code 时配对应 FailedPrecondition,得到 %v", err)
	}
	// legacy token 计算与旧实现逐字节一致(兼容性的锚点)
	sum := sha256.Sum256([]byte("pair-code:x"))
	if hex.EncodeToString(sum[:]) != legacyTokenHash("x") {
		t.Fatalf("legacyTokenHash 与旧 server.token() 不一致")
	}
}

// 【PA-REAL-0001 回归·任务①】模拟入口自动生成配对码后的启动态:鉴权激活,未配对
// 请求被拒;HostService 配对前置放行;正确配对码换到 token 后拦截器放行。
func TestAutoGeneratedPairCodeActivatesAuth(t *testing.T) {
	code := "08374651" // 模拟 main 生成的 8 位数字码
	srv := &Server{cfg: Config{PairCode: code}}
	srv.devices = newDeviceRegistry("", code)

	// 未配对:无凭据请求被拒
	if err := srv.authorized(context.Background()); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("未配对请求应 Unauthenticated,得到 %v", err)
	}
	// 错误 token 被拒
	if err := srv.authorized(ctxWithBearer("forged")); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("伪造 token 应 Unauthenticated,得到 %v", err)
	}
	// HostService 配对前置放行(手机要先读到 auth_required)
	if err := srv.checkAccess(context.Background(), "/pocketagent.v1.HostService/GetHostInfo"); err != nil {
		t.Fatalf("HostService 配对前置应放行: %v", err)
	}
	// 正确配对码 → 换 token → 拦截器放行
	resp, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method: &pb.AuthorizeRequest_PairCode{PairCode: code},
	})
	if err != nil {
		t.Fatalf("配对: %v", err)
	}
	if err := srv.authorized(ctxWithBearer(resp.Token)); err != nil {
		t.Fatalf("配对所得 token 应放行: %v", err)
	}
}

// 【PA-REAL-0001 回归·任务③】-insecure-no-auth 显式降级:回环放行、非回环拒绝
// (HostService 一视同仁)、缺 peer 信息按非回环处理(fail closed)。
func TestInsecureNoAuthLoopbackGate(t *testing.T) {
	srv := &Server{cfg: Config{PairCode: "", InsecureNoAuth: true}}
	srv.devices = newDeviceRegistry("", "")

	peerCtx := func(ip string) context.Context {
		return peer.NewContext(context.Background(), &peer.Peer{
			Addr: &net.TCPAddr{IP: net.ParseIP(ip), Port: 51000},
		})
	}
	// 回环:IPv4/IPv6 都放行(含 HostService 与普通方法)
	if err := srv.checkAccess(peerCtx("127.0.0.1"), "/pocketagent.v1.ThreadService/ListThreads"); err != nil {
		t.Fatalf("IPv4 回环应放行: %v", err)
	}
	if err := srv.checkAccess(peerCtx("::1"), "/pocketagent.v1.HostService/GetHostInfo"); err != nil {
		t.Fatalf("IPv6 回环应放行: %v", err)
	}
	// 局域网来源:即使 HostService 也拒绝
	if err := srv.checkAccess(peerCtx("192.168.1.66"), "/pocketagent.v1.HostService/GetHostInfo"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("局域网连接应 PermissionDenied,得到 %v", err)
	}
	// 公网/远程来源:拒绝
	if err := srv.checkAccess(peerCtx("100.64.0.8"), "/pocketagent.v1.ThreadService/ListThreads"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("非回环连接应 PermissionDenied,得到 %v", err)
	}
	// 缺 peer 信息:fail closed
	if err := srv.checkAccess(context.Background(), "/pocketagent.v1.HostService/Ping"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("无 peer 信息应 PermissionDenied(fail closed),得到 %v", err)
	}
}

// 【PA-REAL-0001 回归·附加】GetHostInfo.auth_required 如实反映:
// 配对码模式(默认)= true;显式 -insecure-no-auth = false。
func TestGetHostInfoAuthRequiredReflectsMode(t *testing.T) {
	newSrv := func(t *testing.T, insecure bool) *Server {
		t.Helper()
		fs, err := store.NewFileStore(t.TempDir())
		if err != nil {
			t.Fatalf("store: %v", err)
		}
		return &Server{cfg: Config{Store: fs, PairCode: "12345678", InsecureNoAuth: insecure}}
	}
	info, err := newSrv(t, false).GetHostInfo(context.Background(), &pb.GetHostInfoRequest{})
	if err != nil {
		t.Fatalf("GetHostInfo: %v", err)
	}
	if !info.AuthRequired {
		t.Fatalf("配对码模式 auth_required 应为 true")
	}
	info, err = newSrv(t, true).GetHostInfo(context.Background(), &pb.GetHostInfoRequest{})
	if err != nil {
		t.Fatalf("GetHostInfo: %v", err)
	}
	if info.AuthRequired {
		t.Fatalf("insecure 模式 auth_required 应为 false")
	}
}
