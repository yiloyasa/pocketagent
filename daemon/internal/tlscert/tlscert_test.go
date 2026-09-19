package tlscert

// P0-1 验收单测:证书生成属性、落盘复用、匹配 pin 握手成功、错 pin 握手失败。

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

// pinnedClientCreds 构造"只认指定指纹"的客户端 TLS 凭证:
// 跳过系统 CA 校验(自签),改在校验回调里比对叶子证书 DER 的 SHA-256。
// 这与手机端 PinTrustManager 是同一套逻辑的 Go 侧镜像,专供测试。
func pinnedClientCreds(pinHex string) credentials.TransportCredentials {
	return credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true, // 自签证书不做 CA 链/主机名校验,pin 即保证
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return x509.ErrUnsupportedAlgorithm // 没有证书可 pin
			}
			sum := sha256.Sum256(rawCerts[0])
			if hex.EncodeToString(sum[:]) != pinHex {
				return x509.CertificateInvalidError{Reason: x509.Expired} // 指纹不匹配
			}
			return nil
		},
	})
}

func fingerprintOf(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// startTLSBufconnServer 起一个 TLS gRPC 服务(bufconn 内存链路,注册了
// 标准 health 服务),返回内存拨号器与服务器证书指纹。
func startTLSBufconnServer(t *testing.T, dataDir string) (dialer func(context.Context, string) (net.Conn, error), pin string) {
	t.Helper()
	cert, err := Ensure(dataDir)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}})))
	healthpb.RegisterHealthServer(gs, health.NewServer())
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	return func(context.Context, string) (net.Conn, error) { return lis.Dial() }, fingerprintOf(cert.Certificate[0])
}

// healthCheck 发一次真实 RPC,强迫完成 TLS 握手;返回错误供断言。
func healthCheck(t *testing.T, dialer func(context.Context, string) (net.Conn, error), creds credentials.TransportCredentials) error {
	t.Helper()
	cc, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer cc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := healthpb.NewHealthClient(cc).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("health 状态应 SERVING,得到 %v", resp.Status)
	}
	return nil
}

func TestTLSHandshakePinMatchSucceeds(t *testing.T) {
	dialer, pin := startTLSBufconnServer(t, t.TempDir())
	if err := healthCheck(t, dialer, pinnedClientCreds(pin)); err != nil {
		t.Fatalf("匹配 pin 的握手应成功: %v", err)
	}
}

func TestTLSHandshakePinMismatchFails(t *testing.T) {
	dialer, pin := startTLSBufconnServer(t, t.TempDir())
	wrong := pin
	if wrong[0] == '0' {
		wrong = "1" + wrong[1:]
	} else {
		wrong = "0" + wrong[1:]
	}
	if err := healthCheck(t, dialer, pinnedClientCreds(wrong)); err == nil {
		t.Fatalf("错 pin 的握手必须失败")
	}
}

func TestEnsureGeneratesAndReloads(t *testing.T) {
	dir := t.TempDir()
	cert1, err := Ensure(dir)
	if err != nil {
		t.Fatalf("首次生成: %v", err)
	}
	leaf, err := x509.ParseCertificate(cert1.Certificate[0])
	if err != nil {
		t.Fatalf("解析证书: %v", err)
	}
	if leaf.Subject.CommonName != "pocketagent-daemon" {
		t.Fatalf("CN 应为 pocketagent-daemon,得到 %q", leaf.Subject.CommonName)
	}
	if leaf.PublicKeyAlgorithm != x509.ECDSA {
		t.Fatalf("应为 ECDSA 密钥,得到 %v", leaf.PublicKeyAlgorithm)
	}
	if pub, ok := leaf.PublicKey.(*ecdsa.PublicKey); !ok || pub.Curve != elliptic.P256() {
		t.Fatalf("应为 P-256 曲线")
	}
	if want := 20 * 365 * 24 * time.Hour; leaf.NotAfter.Sub(leaf.NotBefore) < want-25*time.Hour {
		t.Fatalf("有效期应约 20 年,得到 %v", leaf.NotAfter.Sub(leaf.NotBefore))
	}
	hostOK, ipOK := false, false
	for _, d := range leaf.DNSNames {
		if d == "localhost" {
			hostOK = true
		}
	}
	for _, ip := range leaf.IPAddresses {
		if ip.String() == "127.0.0.1" {
			ipOK = true
		}
	}
	if !hostOK || !ipOK {
		t.Fatalf("SAN 应含 localhost 与 127.0.0.1: %v %v", leaf.DNSNames, leaf.IPAddresses)
	}
	// 第二次 Ensure 必须复用同一张证书(daemon 重启不换指纹,否则手机端 pin 全失效)
	cert2, err := Ensure(dir)
	if err != nil {
		t.Fatalf("二次加载: %v", err)
	}
	if fingerprintOf(cert1.Certificate[0]) != fingerprintOf(cert2.Certificate[0]) {
		t.Fatalf("已存在证书必须复用,不应重新生成")
	}
	// 证书/私钥文件确实落在了约定位置
	certPath, keyPath := Paths(dir)
	for _, p := range []string{certPath, keyPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("证书文件应存在: %s: %v", p, err)
		}
	}
}
