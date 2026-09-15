package server

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hidewin"
)

func (s *Server) GetHostInfo(ctx context.Context, _ *pb.GetHostInfoRequest) (*pb.HostInfo, error) {
	osName, arch, shell := platformInfo()
	ws := s.cfg.Store.DefaultWorkspace()
	if ws == "" {
		ws = s.cfg.Workspace
	}
	return &pb.HostInfo{
		HostId:        s.hostID,
		DisplayName:   hostDisplayName(),
		Os:            osName,
		Arch:          arch,
		DaemonVersion: "0.1.0",
		Workspaces:    []string{ws},
		Capabilities:  []string{"terminal", "fs", "git", "agent"},
		AuthRequired:  s.cfg.PairCode != "",
		DefaultShell:  shell,
		TailscaleIpv4: detectTailscaleIP(),
		LanIpv4:       detectLanIP(),
	}, nil
}

// detectLanIP 找一个可用于局域网直连的本机 IPv4(192.168.x / 10.x / 172.16-31.x),
// 跳过 Tailscale 虚拟网卡(100.x)和回环。同一 WiFi 下手机直连它最快。
// 结果缓存 5 分钟:GetHostInfo 被手机每 8 秒探活一次,不缓存就会不停拉起子进程。
func detectLanIP() string {
	lanIPMutex.Lock()
	defer lanIPMutex.Unlock()
	if time.Since(lanIPAt) < 5*time.Minute {
		return lanIPCache
	}
	lanIPAt = time.Now()
	lanIPCache = ""
	cmd := exec.Command("ipconfig")
	hidewin.Apply(cmd)
	out, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			l := strings.TrimSpace(line)
			if !strings.HasPrefix(l, "IPv4") {
				continue
			}
			ip := strings.TrimSpace(strings.SplitN(strings.SplitN(l, ":", 2)[1], "(", 2)[0])
			if strings.HasPrefix(ip, "100.") || strings.HasPrefix(ip, "127.") || strings.HasPrefix(ip, "169.254.") {
				continue
			}
			if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") ||
				strings.HasPrefix(ip, "172.16.") || strings.HasPrefix(ip, "172.17.") ||
				strings.HasPrefix(ip, "172.18.") || strings.HasPrefix(ip, "172.19.") ||
				strings.HasPrefix(ip, "172.2") || strings.HasPrefix(ip, "172.30.") ||
				strings.HasPrefix(ip, "172.31.") {
				lanIPCache = ip
				return lanIPCache
			}
		}
	}
	// 非 Windows 兜底
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLoopback() && ipn.IP.To4() != nil {
				ip := ipn.IP.String()
				if !strings.HasPrefix(ip, "100.") && !strings.HasPrefix(ip, "169.254.") {
					lanIPCache = ip
					return lanIPCache
				}
			}
		}
	}
	return ""
}

var (
	lanIPCache string
	lanIPAt    time.Time
	lanIPMutex sync.Mutex
)

var (
	tsIPCache string
	tsIPAt    time.Time
	tsIPMutex sync.Mutex
)

// detectTailscaleIP 探测本机 Tailscale IPv4(结果缓存 5 分钟)。
// 用官方 CLI(免 API 鉴权),没装或没登录就返回空。
func detectTailscaleIP() string {
	tsIPMutex.Lock()
	defer tsIPMutex.Unlock()
	if time.Since(tsIPAt) < 5*time.Minute {
		return tsIPCache
	}
	tsIPAt = time.Now()
	tsIPCache = ""
	for _, cand := range []string{
		"tailscale",
		filepath.Join(os.Getenv("ProgramFiles"), "Tailscale", "tailscale.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Tailscale", "tailscale.exe"),
		"/usr/bin/tailscale",
		"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
	} {
		if cand == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		tscmd := exec.CommandContext(ctx, cand, "ip", "-4")
		hidewin.Apply(tscmd)
		out, err := tscmd.Output()
		cancel()
		if err != nil {
			continue
		}
		ip := strings.TrimSpace(strings.Fields(string(out))[0])
		if strings.HasPrefix(ip, "100.") {
			tsIPCache = ip
			break
		}
	}
	return tsIPCache
}

func (s *Server) Authorize(ctx context.Context, req *pb.AuthorizeRequest) (*pb.AuthorizeResponse, error) {
	if s.cfg.PairCode == "" {
		return &pb.AuthorizeResponse{Token: "", ExpiresAtUnix: 0}, nil // 未启用鉴权
	}
	code, ok := req.Method.(*pb.AuthorizeRequest_PairCode)
	if !ok || code.PairCode != s.cfg.PairCode {
		return nil, errPermissionDenied("配对码错误")
	}
	return &pb.AuthorizeResponse{Token: s.token(), ExpiresAtUnix: 0}, nil
}

func (s *Server) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{
		ServerTimeUnixMs: time.Now().UnixMilli(),
		Status:           "ok",
	}, nil
}

func (s *Server) WatchEvents(req *pb.WatchEventsRequest, stream pb.HostService_WatchEventsServer) error {
	// P0:心跳式空转;P1 在这里推送工作区/模型/更新事件
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			// 暂无事件,仅保活
		}
	}
}
