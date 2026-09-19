// tlsprobe:拨 daemon 的 TLS 端口,抢下服务器证书并打印关键字段。
//
// 用途:P0-1 验收("tlsprobe 对真 daemon 打出指纹"),以及手机端
// TOFU 配对前人工核对指纹 —— 两边指纹一致才说明连的是同一台电脑。
// 纯 Go crypto 实现,不依赖 openssl。
//
// 用法:
//
//	tlsprobe [-addr 127.0.0.1:7246]
package main

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"time"
)

// tlsVersionName 把 TLS 版本号翻成人话(排障时看得懂)。
func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:7246", "daemon 的 TLS 监听地址")
	timeout := flag.Duration("timeout", 5*time.Second, "连接超时")
	flag.Parse()

	// 自签证书无法过系统 CA 校验,这里只负责"抢证书",校验交给指纹比对。
	d := &net.Dialer{Timeout: *timeout}
	conn, err := tls.DialWithDialer(d, "tcp", *addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2"}, // gRPC 走 HTTP/2,顺带验证协商
	})
	if err != nil {
		log.Fatalf("TLS 握手失败(%s): %v", *addr, err)
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		log.Fatalf("服务器未提供证书")
	}
	leaf := state.PeerCertificates[0]
	sum := sha256.Sum256(leaf.Raw)
	fp := hex.EncodeToString(sum[:])
	neg := "无"
	if len(state.NegotiatedProtocol) > 0 {
		neg = state.NegotiatedProtocol
	}
	fmt.Printf("地址:     %s\n", *addr)
	fmt.Printf("主题:     %s\n", leaf.Subject.String())
	fmt.Printf("签发者:   %s\n", leaf.Issuer.String())
	fmt.Printf("有效期:   %s ~ %s\n", leaf.NotBefore.Format("2006-01-02"), leaf.NotAfter.Format("2006-01-02"))
	fmt.Printf("SHA-256:  %s\n", fp)
	fmt.Printf("指纹前16: %s…\n", fp[:16])
	fmt.Printf("ALPN:     %s\n", neg)
	fmt.Printf("TLS版本:  %s\n", tlsVersionName(state.Version))
}
