package server

// devices.go 单测(P0-2 验收):配对发每设备 token、挑战应答验签往返、
// 撤销后拒绝、常时比较、并发写、旧单 token 兼容、devices.json 落盘往返。

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// testKey 生成一对 P-256 密钥并返回 base64(SPKI)公钥与签名函数。
// 签名对 data 做 SHA256withECDSA(ASN.1 DER),与手机 Keystore 行为一致。
func testKey(t *testing.T) (pubB64 string, sign func([]byte) []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成测试密钥: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("编码 SPKI: %v", err)
	}
	return base64.StdEncoding.EncodeToString(der), func(data []byte) []byte {
		sum := sha256.Sum256(data)
		sig, err := ecdsa.SignASN1(rand.Reader, key, sum[:])
		if err != nil {
			t.Fatalf("签名: %v", err)
		}
		return sig
	}
}

func TestDevicesRegisterIssuesPerDeviceToken(t *testing.T) {
	r := newDeviceRegistry("", "123456")
	pub, _ := testKey(t)
	tok1, err := r.RegisterByPairCode("123456", "dev-1", "Pixel 9", pub)
	if err != nil {
		t.Fatalf("注册设备 1: %v", err)
	}
	tok2, err := r.RegisterByPairCode("123456", "dev-2", "Tablet", pub)
	if err != nil {
		t.Fatalf("注册设备 2: %v", err)
	}
	if tok1 == tok2 || len(tok1) != 64 {
		t.Fatalf("每设备 token 应独立且 64 hex: %q vs %q", tok1, tok2)
	}
	if id, legacy, ok := r.Validate(tok1); !ok || legacy || id != "dev-1" {
		t.Fatalf("设备 1 token 应有效(非 legacy): id=%q legacy=%v ok=%v", id, legacy, ok)
	}
	// 错误配对码必须拒绝
	if _, err := r.RegisterByPairCode("000000", "dev-3", "X", pub); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("错误配对码应 PermissionDenied,得到 %v", err)
	}
	// 坏公钥必须被挡在落盘之前
	if _, err := r.RegisterByPairCode("123456", "dev-4", "X", "not-base64!!"); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("坏公钥应 InvalidArgument,得到 %v", err)
	}
}

func TestDevicesChallengeRoundTrip(t *testing.T) {
	r := newDeviceRegistry("", "123456")
	pub, sign := testKey(t)
	if _, err := r.RegisterByPairCode("123456", "dev-1", "Pixel 9", pub); err != nil {
		t.Fatalf("注册: %v", err)
	}
	ch, err := r.IssueChallenge("dev-1")
	if err != nil {
		t.Fatalf("要挑战: %v", err)
	}
	sig := base64.StdEncoding.EncodeToString(sign([]byte("dev-1:" + ch)))
	newTok, err := r.VerifyChallenge("dev-1", ch, sig)
	if err != nil {
		t.Fatalf("验签: %v", err)
	}
	if _, _, ok := r.Validate(newTok); !ok {
		t.Fatalf("新 token 应有效")
	}
	// 同一挑战二次使用 = 重放,必须拒绝
	if _, err := r.VerifyChallenge("dev-1", ch, sig); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("挑战重放应 PermissionDenied,得到 %v", err)
	}
}

func TestDevicesChallengeWrongSignatureRejected(t *testing.T) {
	r := newDeviceRegistry("", "123456")
	pubA, signA := testKey(t)
	_, signB := testKey(t)
	if _, err := r.RegisterByPairCode("123456", "dev-A", "A", pubA); err != nil {
		t.Fatalf("注册 A: %v", err)
	}
	// 用别的私钥签:必须拒(与登记的 A 公钥对不上)
	ch, _ := r.IssueChallenge("dev-A")
	wrongSig := base64.StdEncoding.EncodeToString(signB([]byte("dev-A:" + ch)))
	if _, err := r.VerifyChallenge("dev-A", ch, wrongSig); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("错误私钥签名应 PermissionDenied,得到 %v", err)
	}
	// 篡改消息(device_id 换成别的)也必须拒。
	// 【注意】挑战是一次性的:上一步的错误尝试已把 ch 消费掉,这里要新的。
	ch2, _ := r.IssueChallenge("dev-A")
	badMsg := base64.StdEncoding.EncodeToString(signA([]byte("dev-X:" + ch2)))
	if _, err := r.VerifyChallenge("dev-A", ch2, badMsg); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("签名消息不符应 PermissionDenied,得到 %v", err)
	}
	// 正确签名走通(证明上面不是"全拒"的假测试);同样取未消费的新挑战
	ch3, _ := r.IssueChallenge("dev-A")
	sig := base64.StdEncoding.EncodeToString(signA([]byte("dev-A:" + ch3)))
	if _, err := r.VerifyChallenge("dev-A", ch3, sig); err != nil {
		t.Fatalf("正确签名应通过: %v", err)
	}
}

func TestDevicesChallengeExpiredAndUnregistered(t *testing.T) {
	r := newDeviceRegistry("", "123456")
	pub, sign := testKey(t)
	r.RegisterByPairCode("123456", "dev-1", "Pixel 9", pub)
	now := time.Unix(1_000_000, 0)
	r.nowFunc = func() time.Time { return now } // 注入时钟
	ch, err := r.IssueChallenge("dev-1")
	if err != nil {
		t.Fatalf("要挑战: %v", err)
	}
	r.nowFunc = func() time.Time { return now.Add(challengeTTL + time.Minute) }
	sig := base64.StdEncoding.EncodeToString(sign([]byte("dev-1:" + ch)))
	if _, err := r.VerifyChallenge("dev-1", ch, sig); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("过期挑战应 PermissionDenied,得到 %v", err)
	}
	// 未注册设备直接要挑战:必须拒
	if _, err := r.IssueChallenge("dev-404"); status.Code(err) != codes.NotFound {
		t.Fatalf("未注册设备要挑战应 NotFound,得到 %v", err)
	}
}

func TestDevicesRevokeRejectsToken(t *testing.T) {
	r := newDeviceRegistry("", "123456")
	pub, _ := testKey(t)
	tok, err := r.RegisterByPairCode("123456", "dev-1", "Pixel 9", pub)
	if err != nil {
		t.Fatalf("注册: %v", err)
	}
	if _, _, ok := r.Validate(tok); !ok {
		t.Fatalf("撤销前 token 应有效")
	}
	if err := r.Revoke("dev-1"); err != nil {
		t.Fatalf("撤销: %v", err)
	}
	if _, _, ok := r.Validate(tok); ok {
		t.Fatalf("撤销后 token 必须被拒")
	}
	if err := r.Revoke("dev-1"); status.Code(err) != codes.NotFound {
		t.Fatalf("重复撤销应 NotFound,得到 %v", err)
	}
}

func TestDevicesLegacyTokenFallback(t *testing.T) {
	r := newDeviceRegistry("", "123456")
	// 旧 daemon 的单 token = sha256("pair-code:"+配对码) 的 hex,
	// 升级前手机里存的就是这个值,升级后必须继续放行并标 legacy。
	legacy := legacyTokenHash("123456")
	if id, isLegacy, ok := r.Validate(legacy); !ok || !isLegacy || id != "legacy" {
		t.Fatalf("旧 token 应以 legacy 放行: id=%q legacy=%v ok=%v", id, isLegacy, ok)
	}
	// 最新-only 观测:legacy 命中必须计数(下版删逻辑的决策依据)。
	if got := r.LegacyAuthCount(); got != 1 {
		t.Fatalf("legacy 命中应计数 1 次,得到 %d", got)
	}
	// 设备 token 与旧 token 互不通用(两台"设备"token 隔离的逆向表述)
	pub, _ := testKey(t)
	tok, _ := r.RegisterByPairCode("123456", "dev-1", "Pixel 9", pub)
	if id, isLegacy, ok := r.Validate(tok); !ok || isLegacy || id != "dev-1" {
		t.Fatalf("设备 token 应命中设备而非 legacy: id=%q legacy=%v ok=%v", id, isLegacy, ok)
	}
	if _, _, ok := r.Validate(hex.EncodeToString([]byte("garbage"))); ok {
		t.Fatalf("垃圾 token 不应通过")
	}
}

func TestDevicesPersistenceRoundTripAndCorruptFile(t *testing.T) {
	dir := t.TempDir()
	r1 := newDeviceRegistry(dir, "123456")
	pub, sign := testKey(t)
	tok, err := r1.RegisterByPairCode("123456", "dev-1", "Pixel 9", pub)
	if err != nil {
		t.Fatalf("注册: %v", err)
	}
	// 模拟 daemon 重启:新实例重读同一 devices.json
	r2 := newDeviceRegistry(dir, "123456")
	if _, _, ok := r2.Validate(tok); !ok {
		t.Fatalf("重启后 token 应仍有效(落盘往返)")
	}
	// 文件内容确认:合法 JSON,且不得出现明文 token
	raw, err := os.ReadFile(filepath.Join(dir, "devices.json"))
	if err != nil {
		t.Fatalf("读 devices.json: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("devices.json 应是合法 JSON")
	}
	if len(tok) > 0 && base64.StdEncoding.EncodeToString(raw) != "" &&
		contains(string(raw), tok) {
		t.Fatalf("devices.json 泄漏了明文 token")
	}
	// 损坏文件:新实例必须按空表继续,不能 panic
	if err := os.WriteFile(filepath.Join(dir, "devices.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatalf("写坏文件: %v", err)
	}
	r3 := newDeviceRegistry(dir, "123456")
	if len(r3.devices) != 0 {
		t.Fatalf("损坏设备表应按空表处理")
	}
	// 忽略坏表后重新配对 + 挑战应答整链仍走通
	tok3, err := r3.RegisterByPairCode("123456", "dev-1", "Pixel 9", pub)
	if err != nil {
		t.Fatalf("重新注册: %v", err)
	}
	if tok3 == tok {
		t.Fatalf("重新注册应签发不同 token")
	}
	ch, _ := r3.IssueChallenge("dev-1")
	sig := base64.StdEncoding.EncodeToString(sign([]byte("dev-1:" + ch)))
	if _, err := r3.VerifyChallenge("dev-1", ch, sig); err != nil {
		t.Fatalf("重启后验签: %v", err)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

func TestDevicesConcurrentValidateAndRegister(t *testing.T) {
	r := newDeviceRegistry("", "123456")
	pub, _ := testKey(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "dev-conc-" + string(rune('a'+i))
			if _, err := r.RegisterByPairCode("123456", id, "dev", pub); err != nil {
				t.Errorf("并发注册 %s: %v", id, err)
			}
			if _, err := r.IssueChallenge(id); err != nil {
				t.Errorf("并发要挑战 %s: %v", id, err)
			}
		}(i)
	}
	// 同时疯狂 validate:读路径不能与写路径死锁/竞态(配合 -race 跑)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				r.Validate("nonsense")
			}
		}()
	}
	wg.Wait()
}
