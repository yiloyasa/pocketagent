// Package tlscert 负责 daemon 的 TLS 自签证书:首次启动生成,之后一直复用。
//
// 【为什么自签 + 指纹锁定】手机和电脑之间没有域名,公网 CA 签不了;
// 自建 CA 又太重。参考原作 companion 方案:自签一张 20 年证书,
// 手机端第一次连接时记下叶子证书的 SHA-256 指纹(TOFU,Trust On First Use),
// 之后每次握手都比对 —— 指纹不变 = 还是那台电脑,与主机名/IP 无关,
// 所以换了 WiFi、IP 变了都不影响。
package tlscert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// 证书文件名(存于 ~/.pocketagentd/certs/)。
const (
	CertFileName = "server-cert.pem"
	KeyFileName  = "server-key.pem"
	// 有效期 20 年:设备级长期凭证,与原作方案一致。
	validFor = 20 * 365 * 24 * time.Hour
)

// Paths 返回证书与私钥的落盘路径(dataDir 即 ~/.pocketagentd)。
func Paths(dataDir string) (certPath, keyPath string) {
	dir := filepath.Join(dataDir, "certs")
	return filepath.Join(dir, CertFileName), filepath.Join(dir, KeyFileName)
}

// Ensure 保证 dataDir/certs 下有一对可用的自签证书:
// 已存在则直接加载复用;缺失则生成 ECDSA P-256 自签证书并落盘(私钥 0600)。
// 返回可直接交给 gRPC 的 tls.Certificate。
func Ensure(dataDir string) (tls.Certificate, error) {
	certPath, keyPath := Paths(dataDir)
	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		return cert, nil
	}
	certPEM, keyPEM, err := generate()
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := os.MkdirAll(filepath.Dir(certPath), 0o700); err != nil {
		return tls.Certificate{}, fmt.Errorf("创建证书目录失败: %w", err)
	}
	// 先写证书再写私钥;私钥权限收紧到属主(Windows 上多数卷忽略,保留语义)。
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return tls.Certificate{}, fmt.Errorf("写证书失败: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return tls.Certificate{}, fmt.Errorf("写私钥失败: %w", err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("解析刚生成的证书失败: %w", err)
	}
	return cert, nil
}

// generate 生成一张 ECDSA P-256 自签服务器证书:
// CN=pocketagent-daemon,SAN 覆盖 localhost/127.0.0.1/::1。
// 【刻意不写局域网 IP】手机 pin 的是证书指纹而非主机名,IP 变了照样能认;
// 写死当时的 LAN IP 反而在换网络后造成证书内容与预期不符的困惑。
func generate() (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("生成 ECDSA 密钥失败: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("生成序列号失败: %w", err)
	}
	now := time.Now()
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "pocketagent-daemon", Organization: []string{"PocketAgent"}},
		NotBefore:             now.Add(-24 * time.Hour), // 容忍两端时钟小幅偏差
		NotAfter:              now.Add(validFor),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("自签证书失败: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("序列化私钥失败: %w", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}
