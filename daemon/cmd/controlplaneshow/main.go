// controlplaneshow —— 电脑端的「统一控制面板」可见化入口。
//
// 【它是谁】连上 daemon,把八个功能域的控制面排版成人话报表(缺省,只读);
// 开关经 -set 参数走与手机控制中心**完全相同**的 RPC 落盘 —— 两端永远看同一个
// daemon 权威状态(2026-09-30 用户需求:daemon 侧集中管理功能开关,与手机一致)。
//
// 【免 token】配对码模式下本工具与 daemon 同机,自动读数据目录 pair-code.txt 计算
// legacy token(与存量手机同一算法);设备 token 也可 -token 显式给。
//
// 用法:
//
//	controlplaneshow -addr 127.0.0.1:7246 [-data-dir <dir>] [-token <t>]
//	controlplaneshow -set collab=off -set fanout=on        # 开关(先 Get 再完整 Set)
//	  可用键:collab / delegate / fanout / unknownRequests   值:on|off
package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7246", "daemon gRPC 地址")
	token := flag.String("token", "", "设备 token(缺省自动读 pair-code.txt 计算 legacy token)")
	dataDir := flag.String("data-dir", "", "daemon 数据目录(缺省 %LOCALAPPDATA%\\PocketAgent\\data)")
	var sets setList
	flag.Var(&sets, "set", "开关:collab|delegate|fanout|unknownRequests=on|off(可重复)")
	htmlOut := flag.String("html", "", "把控制面板渲染成 HTML 文件(路径;-watch 配合即常驻面板)")
	openBrowser := flag.Bool("open", false, "生成 HTML 后用默认浏览器打开")
	watch := flag.Bool("watch", false, "每 5 秒重新拉取并覆盖 HTML")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tok := *token
	if tok == "" {
		tok = legacyTokenFromDataDir(*dataDir)
	}

	// daemon 默认启用 TLS(自签,首启生成);本机工具直接跳过证书校验。
	tlsCreds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
	opts := []grpc.DialOption{grpc.WithTransportCredentials(tlsCreds)}
	if tok != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(tokenCreds(tok)))
	}
	conn, err := grpc.NewClient(*addr, opts...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "连接失败:", err)
		os.Exit(1)
	}
	defer conn.Close()
	rt := pb.NewRuntimeServiceClient(conn)

	// ---- 开关写通道(与手机控制中心同一批 RPC;先 Get 再完整 Set 的纪律见 proto 注释) ----
	for _, it := range sets {
		key, want := strings.ToLower(it.k), it.v == "on" || it.v == "true" || it.v == "1"
		switch key {
		case "collab", "delegate", "fanout":
			got, err := rt.GetCollabPolicy(ctx, &pb.GetCollabPolicyRequest{})
			if err != nil {
				fail("读取协同策略失败", err)
			}
			pol := got.Policy
			if pol == nil {
				pol = &pb.CollabPolicy{}
			}
			switch key {
			case "collab":
				pol.CollaborationEnabled = &want
			case "delegate":
				pol.NativeDelegateEnabled = &want
			case "fanout":
				pol.FanoutEnabled = &want
			}
			if _, err := rt.SetCollabPolicy(ctx, &pb.SetCollabPolicyRequest{Policy: pol, ExpectedRevision: got.Revision}); err != nil {
				fail("写入协同策略失败", err)
			}
			fmt.Printf("✓ 协同策略 %s = %s\n", key, onOff(want))
		case "unknownrequests":
			// 辅线 B 开关走 RuntimePolicy 完整提交:Get 拿用户配置 → 改单字段 → Set(CAS)
			got, err := rt.GetRuntimePolicy(ctx, &pb.GetRuntimePolicyRequest{})
			if err != nil {
				fail("读取运行时策略失败", err)
			}
			pol := got.Policy
			if pol == nil {
				pol = &pb.RuntimePolicy{}
			}
			pol.UnknownRequestPassthrough = &want
			if _, err := rt.SetRuntimePolicy(ctx, &pb.SetRuntimePolicyRequest{Policy: pol, ExpectedRevision: got.Revision}); err != nil {
				fail("写入运行时策略失败", err)
			}
			fmt.Printf("✓ 未知请求透传(B 开关)= %s\n", onOff(want))
		default:
			fmt.Fprintf(os.Stderr, "未知开关键的键名:%s(可用:collab/delegate/fanout/unknownRequests)\n", it.k)
			os.Exit(1)
		}
	}

	// ---- 只读面板:GetControlPlane 八域总览(与手机控制中心同一数据源) ----
	cp, err := rt.GetControlPlane(ctx, &pb.GetControlPlaneRequest{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "GetControlPlane 失败(旧 daemon 没有此 RPC?):", err)
		os.Exit(1)
	}

	if *htmlOut != "" {
		writeHTMLPanel(*htmlOut, cp)
		if !*watch {
			fmt.Println("✓ 控制面板已生成:", *htmlOut)
		}
		if *openBrowser {
			// rundll32 走系统默认浏览器(GUI 链路,无控制台窗口)
			exec.Command("rundll32", "url.dll,FileProtocolHandler", *htmlOut).Start()
		}
	}
	if *watch {
		fmt.Println("面板常驻刷新中(每 5 秒)… Ctrl+C 退出")
		for {
			time.Sleep(5 * time.Second)
			fresh, err := rt.GetControlPlane(context.Background(), &pb.GetControlPlaneRequest{})
			if err == nil {
				writeHTMLPanel(*htmlOut, fresh)
			}
		}
	}
	if *htmlOut != "" {
		return
	}

	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║        PocketAgent daemon · 控制面总览        ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Printf("  主机: %s   版本: %s   系统: %s   数据目录: %s\n",
		cp.HostName, cp.DaemonVersion, cp.Os, cp.DataDirLabel)
	if cp.UptimeUnixMs > 0 {
		fmt.Printf("  已运行: %s\n", humanUptime(cp.UptimeUnixMs))
	}
	fmt.Println()
	for _, d := range cp.Domains {
		status := "●"
		switch d.StatusCode {
		case 1:
			status = "◐" // 注意
		case 2:
			status = "○" // 受限
		}
		w := ""
		if !d.Writable {
			w = "(只读)"
		}
		fmt.Printf("%s %s %s — %s\n", status, d.Title, w, d.Summary)
		for _, it := range d.Items {
			line := fmt.Sprintf("     · %s: %s", it.Label, it.Current)
			if len(it.Options) > 0 {
				line += fmt.Sprintf("  [候选: %s]", strings.Join(it.Options, "/"))
			}
			fmt.Println(line)
			if it.Hint != "" {
				fmt.Printf("       └ %s\n", it.Hint)
			}
		}
		fmt.Println()
	}
	fmt.Println("(● 正常  ◐ 有注意项  ○ 受限;开关也可在手机「控制中心」操作 —— 两端同源)")
	fmt.Println("  命令行开关: controlplaneshow -set collab=off -set fanout=on -set unknownRequests=on")
}

func fail(what string, err error) {
	fmt.Fprintln(os.Stderr, what+":", err)
	os.Exit(1)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// legacyTokenFromDataDir 读 pair-code.txt 计算 legacy token(daemon 侧
// internal/server.LegacyTokenFor 的镜像实现 —— cmd 工具不 import server 包,
// 保持零反向依赖)。拿不到(未配对码模式/路径不对)返回空,由 -token 兜底。
func legacyTokenFromDataDir(dir string) string {
	if dir == "" {
		la := os.Getenv("LOCALAPPDATA")
		if la == "" {
			return ""
		}
		dir = filepath.Join(la, "PocketAgent", "data")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "pair-code.txt"))
	if err != nil {
		return ""
	}
	code := strings.TrimSpace(string(raw))
	if code == "" {
		return ""
	}
	// sha256("pair-code:" + code) 的 hex —— 与 daemon devices.go 同式。
	return legacyTokenHex(code)
}

// setList 收集重复 -set 参数(flag.Var 接口)。
type setItem struct{ k, v string }
type setList []setItem

func (l *setList) String() string { return fmt.Sprint(*l) }
func (l *setList) Set(v string) error {
	i := strings.Index(v, "=")
	if i <= 0 {
		return fmt.Errorf("-set 需要 键=值 形式:%q", v)
	}
	*l = append(*l, setItem{k: v[:i], v: v[i+1:]})
	return nil
}

func humanUptime(startedUnixMs int64) string {
	d := time.Since(time.UnixMilli(startedUnixMs))
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d 秒", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时 %d 分钟", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%d 天 %d 小时", int(d.Hours())/24, int(d.Hours())%24)
	}
}

// tokenCreds 按 daemon 鉴权链路的 metadata 约定携带设备 token
// (auth_p0p2:authorization = "Bearer <token>")。
type tokenCreds string

func (t tokenCreds) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + string(t)}, nil
}

func (t tokenCreds) RequireTransportSecurity() bool { return false }

var _ = metadata.MD{}

// legacyTokenHex 与 daemon devices.go 同式:sha256("pair-code:" + code) 的 hex。
func legacyTokenHex(code string) string {
	sum := sha256.Sum256([]byte("pair-code:" + code))
	return hex.EncodeToString(sum[:])
}
