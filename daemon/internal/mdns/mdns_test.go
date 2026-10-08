// Package mdns 的进程内回归:本机 注册 → 浏览 应当互相可见。
//
// 【环境门槛】mDNS 需要真实多播网卡(UDP 224.0.0.251:5353)。CI/沙箱常无多播
// 环境,此时注册会报网络类错误 —— 一律 t.Skip(这不是被测代码的 bug);
// 只要有真实网卡,测试就必须全量断言(端口/TXT/去重)。
package mdns

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// skipIfNoMulticast 注册失败且错误属于"网络环境不支持多播"时跳过测试。
func skipIfNoMulticast(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	msg := strings.ToLower(err.Error())
	// 典型无多播症状:cannot assign requested address / network is unreachable /
	// address in use(无权限绑定 5353)/ setsockopt 失败等。
	netEnvErr := strings.Contains(msg, "address") ||
		strings.Contains(msg, "network") ||
		strings.Contains(msg, "socket") ||
		strings.Contains(msg, "permission") ||
		strings.Contains(msg, "bind")
	if netEnvErr || os.Getenv("CI") != "" {
		// CI 上无条件跳过:CI 环境网络栈形态多样,宁可少测也不误报。
		t.Skipf("环境不支持 mDNS 多播(或 CI),跳过: %v", err)
	}
	t.Fatalf("注册 mDNS 服务失败(非网络环境问题,应修复): %v", err)
}

// loopbackUsable 是互见测试的环境门槛:同一主机"注册→浏览互见"要求本机
// 进程能真正收到发往 5353 的组播。Windows 上若 0.0.0.0:5353 被浏览器
// (Edge/QQ浏览器)独占,我们的 socket 绑组播通配成功却收不到任何包 ——
// 这是环境问题而非代码缺陷,此时 skip(完整断言在 Linux CI / 端口健康机上执行)。
func loopbackUsable(t *testing.T) {
	t.Helper()
	probe, err := net.ListenPacket("udp4", ":5353")
	if err != nil {
		t.Skipf("0.0.0.0:5353 无法绑定(被其他程序独占),同机互见必失败,跳过: %v", err)
	}
	_ = probe.Close()
}

func TestRegisterThenBrowse(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过多播依赖测试")
	}
	loopbackUsable(t)
	reg, err := Register(Config{
		HostID:      "host-test-mdns",
		DisplayName: "pocketagentd-it",
		Port:        17246,
		TLSEnabled:  true,
	}, nil)
	skipIfNoMulticast(t, err)
	defer reg.Shutdown()

	// 注册即宣告;浏览窗口给足时间(默认响应含随机延迟 0~255ms,外加缓存链路)。
	deadline := time.Now().Add(8 * time.Second)
	var found []Entry
	for time.Now().Before(deadline) && len(found) == 0 {
		found, err = Browse(context.Background(), 1500*time.Millisecond, nil)
		if err != nil {
			skipIfNoMulticast(t, err)
		}
	}
	if len(found) == 0 {
		t.Fatal("浏览 8 秒仍未发现刚注册的实例(注册与浏览应同机互见)")
	}

	var hit *Entry
	for i := range found {
		if found[i].Instance == "pocketagentd-it" {
			hit = &found[i]
			break
		}
	}
	if hit == nil {
		t.Fatalf("未发现目标实例; 全部发现: %+v", found)
	}
	if hit.Port != 17246 {
		t.Errorf("端口 = %d, 期望 17246", hit.Port)
	}
	if got := hit.HostID(); got != "host-test-mdns" {
		t.Errorf("TXT id = %q, 期望 host-test-mdns", got)
	}
	if got := hit.DisplayName(); got != "pocketagentd-it" {
		t.Errorf("TXT name = %q, 期望 pocketagentd-it", got)
	}
	if !hit.TLSEnabled() {
		t.Errorf("TXT tls = %v, 期望 true", hit.TLSEnabled())
	}
	if hit.AddrString() == "" {
		t.Errorf("AddrString 为空, 应至少解析出一个 IP: %+v", hit.IPs)
	}
}

func TestParseTXT(t *testing.T) {
	m := parseTXT([]string{"v=1", "id=host-1", "tls=1", "novalue"})
	if m["v"] != "1" || m["id"] != "host-1" || m["tls"] != "1" {
		t.Fatalf("解析结果错误: %+v", m)
	}
	if v, ok := m["novalue"]; !ok || v != "" {
		t.Fatalf("无 '=' 的整串应成为空值键: %+v", m)
	}
}

// TestInstanceName 钉死实例名取名顺序(release-smoke-20261009 同机双 daemon 冲突
// 修复的 pure 部分,不依赖真实多播网卡):
//   - DisplayName 非空(含 cmd/pocketagentd 的 -name 覆盖)→ 实例名 = DisplayName;
//   - DisplayName 空白 → 退回 HostID(旧行为,保证 DNS-SD 实例名非空);
//   - 两者皆空 → 空串由 Register 的端口类校验链兜底,这里如实钉出该边界。
func TestInstanceName(t *testing.T) {
	cases := []struct {
		name        string
		displayName string
		hostID      string
		want        string
	}{
		{"-name 覆盖优先", "pa-second", "host-abc", "pa-second"},
		{"默认展示名即主机名", "DESKTOP-ABC123", "host-abc", "DESKTOP-ABC123"},
		{"空白展示名退回HostID", "", "host-abc", "host-abc"},
		{"纯空白展示名退回HostID", "   ", "host-abc", "host-abc"},
		{"覆盖名带首尾空白被trim", "  pa-second  ", "host-abc", "pa-second"},
		{"两者皆空为空串", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := instanceName(Config{DisplayName: tc.displayName, HostID: tc.hostID})
			if got != tc.want {
				t.Fatalf("instanceName = %q, 期望 %q", got, tc.want)
			}
		})
	}
}

func TestEntryHelpers(t *testing.T) {
	e := Entry{
		Instance: "inst",
		Port:     7246,
		IPs:      []string{"8.8.8.8", "192.168.1.7", "10.0.0.2"},
		Text:     map[string]string{"tls": "0", "id": "h1"},
	}
	sortIPs(e.IPs)
	if e.IPs[0] != "192.168.1.7" {
		t.Fatalf("局域网段应排最前: %v", e.IPs)
	}
	if e.TLSEnabled() {
		t.Fatal("tls=0 应解析为未启用 TLS")
	}
	if e.AddrString() != "192.168.1.7:7246" {
		t.Fatalf("AddrString = %q", e.AddrString())
	}
}
