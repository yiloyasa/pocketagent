// Package mdns 实现daemon 的 mDNS 局域网自动发现(P0-3)。
//
// 【讲解】mDNS(DNS-SD)就是局域网里的"按键说话"广播:daemon 启动时向
// 224.0.0.251:5353 宣告"_pocketagent._tcp 服务在这台机器的 7246 端口",
// 同一 WiFi 的手机无需手填 IP 就能"听到"它。选型 github.com/grandcat/zeroconf:
// 单包同时覆盖 注册(服务端应答查询)+ 浏览(客户端发查询),API 带 TXT 记录与
// 接口绑定,且支持 Shutdown 时发 GOODBYE 包即时注销 —— 换 hashicorp/mdns 的话
// 注销语义弱(只能弃掉 listener,缓存会残留 TTL 时间),故优先前者。
//
// 广播内容(细则规定):
//
//	服务类型  _pocketagent._tcp.local.
//	端口      7246(实际 gRPC 监听端口)
//	TXT       v=1(协议版本) / id=<host_id>(主机稳定标识) /
//	          name=<display_name>(展示名) / tls=1|0(端口是否启用了 TLS)
package mdns

import (
	"context"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

// 服务类型与域名。zeroconf 的传参格式是 (service="_pocketagent._tcp", domain="local"),
// 完整服务实例名拼出来是 "instance._pocketagent._tcp.local."。
const (
	Service = "_pocketagent._tcp"
	Domain  = "local"
)

// TXT 记录的键名(DNS-SD 规范:键值对用 "key=value" 字符串)。
const (
	TXTKeyVersion = "v"
	TXTKeyHostID  = "id"
	TXTKeyName    = "name"
	TXTKeyTLS     = "tls"
)

// 协议版本:手机端靠它判断 TXT 结构是否是自己认识的格式。
const ProtocolVersion = "1"

// Config 注册参数。
type Config struct {
	// HostID 主机稳定标识(daemon server 的 host_id),手机端用它区分"这是哪台电脑"。
	HostID string
	// DisplayName 展示名(与 GetHostInfo.displayName 一致),同时用作 mDNS 实例名。
	DisplayName string
	// Port gRPC 实际监听端口(一般 7246)。
	Port int
	// TLSEnabled 端口上是否启用了 TLS(P0-1 起默认 true)。
	TLSEnabled bool
}

// Registrar 持有已注册的 mDNS 服务,进程退出前调 [Registrar.Shutdown] 注销。
type Registrar struct {
	server *zeroconf.Server
}

// Register 向局域网宣告本机的 _pocketagent._tcp 服务。
//
// ifaces 传 nil 表示绑定全部多播 capable 接口(生产用法);
// 单测可传指定接口缩小范围。多播绑定失败(无多播网卡/被防火墙禁)会返回错误,
// 调用方(见 cmd/pocketagentd)应降级为日志警告,绝不能崩 —— mDNS 只是便利功能。
func Register(cfg Config, ifaces []net.Interface) (*Registrar, error) {
	if cfg.Port <= 0 {
		return nil, fmt.Errorf("mdns: 非法端口 %d", cfg.Port)
	}
	warnIfPort5353Hijacked()
	tlsFlag := "0"
	if cfg.TLSEnabled {
		tlsFlag = "1"
	}
	text := []string{
		TXTKeyVersion + "=" + ProtocolVersion,
		TXTKeyHostID + "=" + cfg.HostID,
		TXTKeyName + "=" + cfg.DisplayName,
		TXTKeyTLS + "=" + tlsFlag,
	}
	// 实例名取展示名;为空时退回 host_id,保证实例名不为空(DNS-SD 要求非空)。
	instance := cfg.DisplayName
	if strings.TrimSpace(instance) == "" {
		instance = cfg.HostID
	}
	srv, err := zeroconf.Register(instance, Service, Domain, cfg.Port, text, ifaces)
	if err != nil {
		return nil, fmt.Errorf("mdns: 注册 _pocketagent._tcp 失败: %w", err)
	}
	return &Registrar{server: srv}, nil
}

// Shutdown 发 GOODBYE 并停掉应答循环,让局域网内缓存尽快过期。
// 幂等,可安全重复调用。
func (r *Registrar) Shutdown() {
	if r == nil || r.server == nil {
		return
	}
	r.server.Shutdown()
}

// Entry 是浏览(发现)结果:一台局域网内可见的 PocketAgent daemon。
type Entry struct {
	Instance string            // mDNS 实例名(= daemon 展示名)
	Host     string            // 实例主机名(zeroconf 的 SRV target)
	IPs      []string          // 宣告的 IPv4/IPv6 地址(已按"局域网段优先"排序)
	Port     int               // gRPC 端口
	Text     map[string]string // TXT 键值(id/name/tls/v)
}

// HostID 取 TXT 里的主机标识,缺失返回空串。
func (e Entry) HostID() string { return e.Text[TXTKeyHostID] }

// DisplayName 取 TXT 里的展示名(其次实例名),用于 UI 显示。
func (e Entry) DisplayName() string {
	if n := e.Text[TXTKeyName]; n != "" {
		return n
	}
	return e.Instance
}

// TLSEnabled 端口是否启用 TLS(tls=1)。解析失败一律按 true(与 P0-1 默认一致)。
func (e Entry) TLSEnabled() bool { return e.Text[TXTKeyTLS] != "0" }

// Browse 在局域网内浏览 _pocketagent._tcp 服务,阻塞 timeout 后返回全部发现。
// 结果按实例名去重(zeroconf 会因缓存刷新/多网卡对同一实例回调多次)。
func Browse(ctx context.Context, timeout time.Duration, ifaces []net.Interface) ([]Entry, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("mdns: 创建浏览器失败: %w", err)
	}
	entries := make(chan *zeroconf.ServiceEntry, 16)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := resolver.Browse(ctx, Service, Domain, entries); err != nil {
		return nil, fmt.Errorf("mdns: 浏览 _pocketagent._tcp 失败: %w", err)
	}

	seen := make(map[string]*Entry) // 实例名 → 汇总结果(同实例多网卡地址合并)
	for ent := range entries {
		if ent == nil {
			continue
		}
		// 去重键用公开字段 Instance:ServiceInstanceName() 依赖库内部私有字段,
		// 浏览回调构造的 ServiceEntry 里不保证已被填充。
		key := ent.Instance
		cur, ok := seen[key]
		if !ok {
			cur = &Entry{
				Instance: ent.Instance,
				Host:     strings.TrimSuffix(ent.HostName, "."),
				Port:     ent.Port,
				Text:     parseTXT(ent.Text),
			}
			seen[key] = cur
		} else {
			// 同一实例再次出现:端口/文本以最新为准,地址取并集。
			cur.Port = ent.Port
			for k, v := range parseTXT(ent.Text) {
				cur.Text[k] = v
			}
		}
		for _, ip := range ent.AddrIPv4 {
			cur.IPs = appendUnique(cur.IPs, ip.String())
		}
		for _, ip := range ent.AddrIPv6 {
			cur.IPs = appendUnique(cur.IPs, ip.String())
		}
	}
	// chan 在 ctx 超时/取消后由 zeroconf 关闭,到这里说明浏览窗口结束。
	out := make([]Entry, 0, len(seen))
	for _, e := range seen {
		sortIPs(e.IPs)
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Instance < out[j].Instance })
	return out, nil
}

// parseTXT 把 zeroconf 给的 "k=v" 字符串列表整理成 map(无 '=' 的整串当 key、值为空)。
func parseTXT(raw []string) map[string]string {
	m := make(map[string]string, len(raw))
	for _, s := range raw {
		k, v, _ := strings.Cut(s, "=")
		m[k] = v
	}
	return m
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// sortIPs 简单排序:让 192.168./10. 等常见局域网段排前面,方便手机端取第一个。
func sortIPs(ips []string) {
	rank := func(ip string) int {
		switch {
		case strings.HasPrefix(ip, "192.168."):
			return 0
		case strings.HasPrefix(ip, "10."):
			return 1
		case strings.HasPrefix(ip, "172.16."), strings.HasPrefix(ip, "172.17."),
			strings.HasPrefix(ip, "172.18."), strings.HasPrefix(ip, "172.19."),
			strings.HasPrefix(ip, "172.2"), strings.HasPrefix(ip, "172.30."),
			strings.HasPrefix(ip, "172.31."):
			return 2
		case strings.HasPrefix(ip, "100."): // Tailscale 虚拟网段,mDNS 不该出现,兜底即可
			return 3
		default:
			return 4
		}
	}
	sort.SliceStable(ips, func(i, j int) bool { return rank(ips[i]) < rank(ips[j]) })
}

// AddrString 把 Entry 格式化成 "ip:port"(取排序后第一个 IP);无可用 IP 返回空串。
func (e Entry) AddrString() string {
	if len(e.IPs) == 0 {
		return ""
	}
	return net.JoinHostPort(e.IPs[0], strconv.Itoa(e.Port))
}

// warnIfPort5353Hijacked 自检 mDNS 标准端口 5353 是否被其他进程**独占**。
//
// 【为什么要自检】Windows 上浏览器(Microsoft Edge/QQ浏览器等)会以独占方式绑定
// 0.0.0.0:5353(它们的投屏/发现功能,不设 SO_REUSEADDR)。此时我们的组播 socket
// 仍能"绑成功"(绑定在 224.0.0.0:5353 通配上,不会报错),但内核会把收到的 mDNS
// 包全部交给独占者 —— 表现为:注册无异常、手机却永远发现不了本机。
// 探测方法很轻:试着绑一次 0.0.0.0:5353,被拒即说明端口被独占,打警告提示用户;
// 立刻关闭探测 socket,不影响后续真正的组播绑定。
func warnIfPort5353Hijacked() {
	probe, err := net.ListenPacket("udp4", ":5353")
	if err == nil {
		_ = probe.Close() // 端口健康,探测完立即释放
		return
	}
	log.Printf("WARNING: UDP 5353(mDNS 标准端口)已被其他程序独占(%v)。"+
		"常见元凶:Microsoft Edge / QQ 浏览器等后台常驻的浏览器。"+
		"在此状态下手机大概率无法通过 mDNS 发现本机;退出相关浏览器或禁用其后台后重启 daemon 即可恢复。",
		err)
}
