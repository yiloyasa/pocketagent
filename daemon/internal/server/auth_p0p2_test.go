package server

// P0-2 端到端验收:经 Server.Authorize 四分支 + authorized 拦截器全链路。
// (devices_test.go 锁的是注册表本体的单元行为,这里锁 gRPC 层的装配正确性:
// oneof 分派、Bearer 解析、撤销后的 PERMISSION_DENIED。)

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
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

// 未启用鉴权时 Authorize 与拦截器全放行(既有语义回归)。
func TestAuthorizeDisabledAuthAllowsAll(t *testing.T) {
	srv := &Server{cfg: Config{PairCode: ""}}
	srv.devices = newDeviceRegistry("", "")
	if err := srv.authorized(ctxWithBearer("anything")); err != nil {
		t.Fatalf("未启用鉴权应放行: %v", err)
	}
	resp, err := srv.Authorize(context.Background(), &pb.AuthorizeRequest{
		Method: &pb.AuthorizeRequest_PairCode{PairCode: "whatever"},
	})
	if err != nil || resp.Token != "" {
		t.Fatalf("未启用鉴权时应返回空 token: %+v err=%v", resp, err)
	}
	// legacy token 计算与旧实现逐字节一致(兼容性的锚点)
	sum := sha256.Sum256([]byte("pair-code:x"))
	if hex.EncodeToString(sum[:]) != legacyTokenHash("x") {
		t.Fatalf("legacyTokenHash 与旧 server.token() 不一致")
	}
}
