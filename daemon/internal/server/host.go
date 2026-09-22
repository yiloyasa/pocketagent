package server

import (
	"context"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/piapp"
	"github.com/pocketagent/daemon/internal/version"
)

func (s *Server) GetHostInfo(ctx context.Context, _ *pb.GetHostInfoRequest) (*pb.HostInfo, error) {
	osName, arch, shell := platformInfo()
	ws := s.cfg.Store.DefaultWorkspace()
	if ws == "" {
		ws = s.cfg.Workspace
	}
	// capabilities 动态生成:agent 能力按 DetectProviders 的实际注册结果声明
	// (注册了 pi 就带 "agent:pi",zcode 同理)。手机端据此点亮对应入口,
	// 不再需要硬编码"开发中"名单。echo 也会列出(它确实在场),由客户端按需展示。
	// 【格式 v2】pi 追加版本号 "agent:pi:v0.85.1"(探测细节),手机端引导弹窗
	// 据此显示"已装 vX"与"未安装"两种态;zcode/codex 探测只有"存在与否",
	// 维持 "agent:<id>"。手机端按 "agent:<id>" 前缀匹配,两种格式都点亮。
	var caps []string
	for id, provider := range s.cfg.Providers {
		caps = append(caps, agentCapability(id))
		if _, ok := provider.(agent.PrefsAware); ok {
			caps = append(caps, "agent_settings:"+id)
		}
		if _, ok := provider.(agent.TextGenerator); ok {
			caps = append(caps, "text_generation:"+id)
		}
		// collab 能力:该 provider 是否已把本地 collab MCP 挂进自己的会话(即它那
		// 一侧的模型看得见 agent_delegate)。手机端据此点亮"多 agent 协同"入口,
		// 而不是再摆一个恒假的"开发中"开关。判定走接口,不枚举 agent 名字。
		if ca, ok := provider.(agent.CollabAware); ok && ca.CollabReady() {
			caps = append(caps, "collab:"+id)
		}
	}
	sort.Strings(caps) // 稳定输出,便于测试与客户端缓存
	caps = append(caps, "terminal", "fs", "git", "agent", "event_sequence")
	return &pb.HostInfo{
		HostId:        s.hostID,
		DisplayName:   hostDisplayName(),
		Os:            osName,
		Arch:          arch,
		DaemonVersion: version.App,
		Workspaces:    []string{ws},
		Capabilities:  caps,
		// ^ P1-5:声明支持事件断点续传(帧带 sequence / GetThreadEventsSince /
		// WatchEvents.after_sequence)。手机端只在看到该能力时才走增量恢复路径,
		// 旧 daemon 无此声明 → 手机自动退回历史全量回填,互不踩脚。
		// 【PA-REAL-0001】auth_required 如实反映:配对码模式(默认)= true;
		// 显式 -insecure-no-auth = false。空配对码的异常配置同样按 true 报告
		// (服务确实在拒绝未授权请求)。
		AuthRequired:  s.authRequired(),
		DefaultShell:  shell,
		TailscaleIpv4: detectTailscaleIP(),
		LanIpv4:       detectLanIP(),
		// W6b:把启动恢复报告(含"上次退出时哪几条排队输入丢了")随探活一起带给手机。
		// 走 GetHostInfo 而不是新增 RPC:手机每 8 秒就会问一次这里,用户打开 App 即
		// 能看到,无需额外的轮询通道;代价只是一次结构体拷贝。此前这条信息只落在
		// daemon 日志里,用户端无从知道自己的排队消息没跑成 —— 判为"未达成"。
		Recovery: s.recoveryReport.ToProto(),
	}, nil
}

// HostID 返回本 daemon 的稳定主机标识(进程内随机生成)。
// 供 cmd/pocketagentd 的 mDNS 广播取 TXT id(P0-3),不影响其他语义。
func (s *Server) HostID() string { return s.hostID }

// authRequired 报告"本 daemon 是否在鉴权":唯一关闭方式是显式
// -insecure-no-auth。GetHostInfo 用它如实回答手机端"要不要走配对流程"。
func (s *Server) authRequired() bool { return !s.cfg.InsecureNoAuth }

// agentCapability 生成单个 agent 的能力串(capabilities 生成段的辅助函数)。
//
// 默认 "agent:<id>"(存在即点亮,手机端前缀匹配);pi 额外带版本号
// "agent:pi:v0.85.1" —— 手机端引导弹窗据此区分"已装 vX"与"未安装"。
// 本函数只在 GetHostInfo 对已注册 provider 的循环里被调用,所以 pi 走到
// 这里就代表启动时 DetectProviders 已探明 node + cli.js + --version 可用,
// 版本探测(拉 node 子进程)不会对未安装机器白跑。
func agentCapability(id string) string {
	if id == "pi" {
		if v := cachedPiVersion(); v != "" {
			return "agent:pi:" + v
		}
	}
	// zcode/codex 的探测(LocateZCode/LocateCodex)只有"存在与否",无版本可报;
	// pi 版本探测失败也回落无版本格式 —— 前缀匹配语义不变,只是弹窗少显示版本。
	return "agent:" + id
}

// cachedPiVersion 取 pi 版本,5 分钟缓存。
//
// 【为什么必须缓存】GetHostInfo 被手机每 8 秒探活一次(见 detectLanIP 注释),
// 而 piapp.Version 要拉起 node cli.js --version 子进程(内部超时 10 秒),
// 不缓存就会每 8 秒起一个进程。与 detectLanIP/detectTailscaleIP 同款取舍:
// 首次调用同步探测(健康机器上 node --version 数百毫秒),之后 5 分钟内直接命中。
var (
	piVerCache string
	piVerAt    time.Time
	piVerMu    sync.Mutex
)

func cachedPiVersion() string {
	piVerMu.Lock()
	cached := piVerCache
	if time.Since(piVerAt) < 5*time.Minute {
		piVerMu.Unlock()
		return cached
	}
	piVerAt = time.Now()
	piVerMu.Unlock()
	// Health probes must never wait for a Node process or a VPN route.
	go func() {
		node, cli, err := piapp.LocatePi()
		if err != nil {
			return
		}
		version := piapp.Version(node, cli)
		piVerMu.Lock()
		piVerCache = version
		piVerMu.Unlock()
	}()
	return cached
}

// DisplayName 返回本机展示名(与 GetHostInfo.displayName 同源)。
// 供 cmd/pocketagentd 的 mDNS 广播取实例名/TXT name(P0-3)。
func (s *Server) DisplayName() string { return hostDisplayName() }

// detectLanIP 找一个可用于局域网直连的本机 IPv4(192.168.x / 10.x / 172.16-31.x),
// 跳过 Tailscale 虚拟网卡(100.x)和回环。同一 WiFi 下手机直连它最快。
// 结果缓存 5 分钟:GetHostInfo 被手机每 8 秒探活一次,不缓存就会不停拉起子进程。
func detectLanIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		name := strings.ToLower(iface.Name)
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		skip := false
		for _, virtual := range []string{"tailscale", "tap", "tun", "vpn", "virtual", "vethernet", "docker", "wsl"} {
			if strings.Contains(name, virtual) {
				skip = true
			}
		}
		if skip {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err == nil && ip.To4() != nil && ip.IsPrivate() {
				return ip.String()
			}
		}
	}
	return ""
}

// Enumerating the local tunnel is independent of the outbound VPN/proxy.
// No CLI process, external request or route changes in the health endpoint.
func detectTailscaleIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	_, rangeTS, _ := net.ParseCIDR("100.64.0.0/10")
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || !strings.Contains(strings.ToLower(iface.Name), "tailscale") {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err == nil && rangeTS.Contains(ip) {
				return ip.String()
			}
		}
	}
	return ""
}

// Authorize 四分支(P0-2):
//
//	pair_code + device_id + device_pub_key → 注册设备,发每设备专属 token;
//	pair_code(不带设备身份)→ 存量手机兼容路径,回旧单 token;
//	token                    → 已发 token 的续期确认(有效原样续,无效拒绝);
//	challenge_request        → 给已注册设备发随机挑战数;
//	challenge_answer         → 验 Keystore 签名,验过发新 token。
//
// GetHostInfo 的 auth_required 语义保持不变(见 authRequired)。
func (s *Server) Authorize(ctx context.Context, req *pb.AuthorizeRequest) (*pb.AuthorizeResponse, error) {
	// 【PA-REAL-0001】两种新情形:
	//   insecure-no-auth → 无鉴权模式,无需配对(回环限制由拦截器把守),回空 token;
	//   空 PairCode       → 异常配置(正常启动必自动生成),fail closed 拒绝,
	//                      绝不能像旧版那样静默返回"空 token 全放行"。
	if s.cfg.InsecureNoAuth {
		return &pb.AuthorizeResponse{Token: "", ExpiresAtUnix: 0}, nil
	}
	if s.cfg.PairCode == "" {
		return nil, errFailedPrecondition("服务端未配置配对码,拒绝配对;请重启 daemon(自动生成配对码)或以 -insecure-no-auth 显式关闭鉴权(仅限本机)")
	}
	switch m := req.Method.(type) {
	case *pb.AuthorizeRequest_PairCode:
		// 新流程:手机带本机 UUID + P-256 公钥来注册,领每设备独立 token。
		if req.GetDeviceId() != "" && req.GetDevicePubKey() != "" {
			token, err := s.devices.RegisterByPairCode(m.PairCode, req.GetDeviceId(), req.GetDeviceName(), req.GetDevicePubKey())
			if err != nil {
				return nil, err
			}
			return &pb.AuthorizeResponse{Token: token, DeviceId: req.GetDeviceId()}, nil
		}
		// 旧流程:不带设备身份,回 legacy 单 token(兼容存量手机)。
		if m.PairCode != s.cfg.PairCode {
			return nil, errPermissionDenied("配对码错误")
		}
		return &pb.AuthorizeResponse{Token: s.token(), ExpiresAtUnix: 0}, nil
	case *pb.AuthorizeRequest_Token:
		// 续期:token 有效(设备或 legacy)就原样续,不换值 —— 手机端拿到的
		// 仍是熟悉的那串;无效则拒绝,客户端走 challenge_request 重新认证。
		if _, _, ok := s.devices.Validate(m.Token); !ok {
			return nil, errPermissionDenied("token 无效,请用配对码或挑战应答重新认证")
		}
		return &pb.AuthorizeResponse{Token: m.Token, ExpiresAtUnix: 0}, nil
	case *pb.AuthorizeRequest_ChallengeRequest:
		ch, err := s.devices.IssueChallenge(req.GetChallengeRequest())
		if err != nil {
			return nil, err
		}
		return &pb.AuthorizeResponse{Challenge: ch, DeviceId: req.GetChallengeRequest()}, nil
	case *pb.AuthorizeRequest_ChallengeAnswer:
		ans := req.GetChallengeAnswer()
		token, err := s.devices.VerifyChallenge(req.GetDeviceId(), ans.GetChallenge(), ans.GetSignature())
		if err != nil {
			return nil, err
		}
		return &pb.AuthorizeResponse{Token: token, DeviceId: req.GetDeviceId()}, nil
	default:
		return nil, errInvalidArg("缺少认证方式(pair_code/token/challenge)")
	}
}

func (s *Server) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{
		ServerTimeUnixMs: time.Now().UnixMilli(),
		Status:           "ok",
	}, nil
}

// WatchEvents 主机事件流(P1-5 升级:补发 + 实时广播)。
//
// 连接顺序:先按 after_sequence 回放事件库里仍保留的主机事件(断线期间错过的),
// 再注册进实时广播 —— 顺序保证"先补旧的,后收新的"。已知的窄窗口:回放快照与
// 注册之间新产生的事件可能在补发里缺席(它们还没落盘时回放已开始),主机事件
// 是通知语义,手机端下个心跳周期自然修正,这里不为它加双阶段锁。
func (s *Server) WatchEvents(req *pb.WatchEventsRequest, stream pb.HostService_WatchEventsServer) error {
	if after := req.GetAfterSequence(); after > 0 {
		for _, e := range s.events.ReplayHostEvents(after) {
			var ev pb.HostEvent
			if err := protojson.Unmarshal([]byte(e.PayloadJSON), &ev); err != nil {
				continue // 损坏行跳过
			}
			ev.Sequence = e.Seq
			if err := stream.Send(&ev); err != nil {
				return err
			}
		}
	}
	// 注册进广播,收实时事件;10s 心跳防中间设备掐空闲连接(语义不变)。
	wid, ch := s.hostWatch.add()
	defer s.hostWatch.remove(wid)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case ev := <-ch:
			if err := stream.Send(ev); err != nil {
				return err
			}
		case <-ticker.C:
			// 暂无事件,仅保活
		}
	}
}
