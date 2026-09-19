// 本文件:电脑 dev server 探测(P2)。
//
// 对齐原作 MobileCodex BrowserService.ListDevServers:手机"电脑预览"一键拿到
// "电脑上现在跑着哪些 dev server"。daemon 对本机常见 dev 端口逐个 TCP 拨号,
// 拨通的端口发一个 `GET /` 读响应头:状态行以 "HTTP/" 开头才算网页服务,顺手
// 取 Server 响应头与首页 <title> 给列表展示。
//
// 【红线遵从】纯 net 包实现,不起任何子进程(不 netstat/tasklist,不闪黑窗)。
//
// 【耗时】每端口 1s 预算、全部端口并发探测,整扫通常 <1.5s,硬上限 8s
// (手机侧 RPC deadline 也是 8s,两边对齐)。结果缓存 60s:预览面板反复
// 开合不打端口,手机手动刷新才是重扫入口。
package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// devServerPorts 常见 dev server 端口表。
// 3000/3001 Next/React/Express;4200 Angular;5000 Flask/.NET;
// 5173/5174 Vite;8000 Django/python http.server;8080/8081 Java/通用;
// 8888 Jupyter;9000 PHP/常用调试口。顺序即拨号顺序(并发,无先后语义)。
var devServerPorts = []int{3000, 3001, 4200, 5000, 5173, 5174, 8000, 8080, 8081, 8888, 9000}

const (
	previewPortTimeout = 1 * time.Second  // 单端口预算(拨号+发请求+读响应头)
	previewScanBudget  = 8 * time.Second  // 整扫硬上限
	previewCacheTTL    = 60 * time.Second // 缓存有效期;手机刷新才重扫
	previewTitleMax    = 64 << 10         // 提取 <title> 时最多读的响应体字节数
)

// dialContexter 是 net.Dialer.DialContext 的最小接口,单测注入 fake 用。
type dialContexter interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// previewDialer 拨号注入点:生产为 nil(走真实网络栈);单测换成 fake 免真网。
var previewDialer dialContexter

// previewCache 扫描结果缓存。锁横跨整次扫描:并发请求只有一个真正扫描,
// 其余等锁后直接共享这份结果(比各自扫一遍省时也省端口打扰)。
var previewCache struct {
	mu      sync.Mutex
	at      time.Time // 本次结果生成时刻
	servers []*pb.DevServer
	// now 可注入假时钟,TTL 单测不必真睡 60s;nil = 生产用真实时间。
	now func() time.Time
}

// previewNow 取缓存时钟(nil 回退真实时间)。
func previewNow() time.Time {
	if previewCache.now != nil {
		return previewCache.now()
	}
	return time.Now()
}

// ListDevServers 手机取"电脑上的 dev server"清单(P2,HostService 新面)。
//
// 缓存命中直接回;过期才重扫。不做独立失效通道 —— 手机刷新按钮重进本 RPC,
// 撞上 TTL 自然重扫,语义最简且够用。
func (s *Server) ListDevServers(ctx context.Context, _ *pb.ListDevServersRequest) (*pb.DevServerList, error) {
	servers, at := cachedDevServers(ctx)
	return &pb.DevServerList{
		Servers:         servers,
		ScannedAtUnixMs: at.UnixMilli(),
	}, nil
}

// cachedDevServers 带 60s TTL 的缓存读取;过期时重扫并回填。
func cachedDevServers(ctx context.Context) ([]*pb.DevServer, time.Time) {
	previewCache.mu.Lock()
	defer previewCache.mu.Unlock()
	now := previewNow()
	if previewCache.servers != nil && now.Sub(previewCache.at) < previewCacheTTL {
		return previewCache.servers, previewCache.at
	}
	previewCache.servers = scanDevServers(ctx)
	previewCache.at = now
	return previewCache.servers, previewCache.at
}

// scanDevServers 并发探测端口表,返回开着且应答 HTTP 的端口(按端口升序)。
// ctx 已带 deadline(RPC 传手机侧 8s)就直接用,否则套我们自己的 8s 上限。
func scanDevServers(ctx context.Context) []*pb.DevServer {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, previewScanBudget)
		defer cancel()
	}
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out []*pb.DevServer
	)
	for _, port := range devServerPorts {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			// 每端口独立 1s 超时:一个慢端口不拖累别人,也不吃总预算。
			pctx, cancel := context.WithTimeout(ctx, previewPortTimeout)
			defer cancel()
			if srv := probePort(pctx, port); srv != nil {
				mu.Lock()
				out = append(out, srv)
				mu.Unlock()
			}
		}(port)
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].GetPort() < out[j].GetPort() })
	for _, srv := range out {
		log.Printf("[preview] 发现 dev server :%d (%s)", srv.GetPort(), previewLabel(srv))
	}
	return out
}

// probePort 拨 127.0.0.1:port 并发 `GET /`(HTTP/1.0 —— 对端不会用 chunked
// 编码回包,首页能按原始字节直读,<title> 才提得出来)。端口关闭、非 HTTP、
// 超时一律返回 nil:绝大多数端口本来就是"没人听",这是正常路径不是错误。
func probePort(ctx context.Context, port int) *pb.DevServer {
	d := previewDialer
	if d == nil {
		d = &net.Dialer{}
	}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil
	}
	defer conn.Close()
	// 本端口的硬期限:取 min(1s, ctx 剩余),调用方预算更紧时尊重调用方。
	deadline := time.Now().Add(previewPortTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)
	req := "GET / HTTP/1.0\r\nHost: 127.0.0.1\r\nUser-Agent: pocketagent-preview-probe\r\n\r\n"
	if _, err = io.WriteString(conn, req); err != nil {
		return nil
	}
	// http.ReadResponse 逐字节解析状态行+响应头:对端哪怕回的是 SSH 横幅
	// 也只是 err 返回,绝不会 panic 或挂死(期限兜底)。
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	srv := &pb.DevServer{
		Port: int32(port),
		Url:  fmt.Sprintf("http://127.0.0.1:%d", port),
	}
	srv.ServerHeader = strings.TrimSpace(resp.Header.Get("Server"))
	srv.Title = extractTitle(resp.Body)
	return srv
}

// extractTitle 从响应体前 64KB 里抠 <title> 文本(大小写不敏感、允许标签带
// 属性)。找不到返回空。不做 HTML 实体解码:dev 页标题基本都是纯文本,
// 极个别 &amp; 显示原样,比引一个 HTML 解析器划算。
func extractTitle(r io.Reader) string {
	body, err := io.ReadAll(io.LimitReader(r, previewTitleMax))
	if err != nil && len(body) == 0 {
		return ""
	}
	m := titleRe.FindSubmatch(body)
	if m == nil {
		return ""
	}
	t := strings.Join(strings.Fields(string(m[1])), " ") // 折叠换行与连续空白
	if t == "" {
		return ""
	}
	if runes := []rune(t); len(runes) > 120 {
		t = string(runes[:120]) + "…"
	}
	return t
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// previewLabel 日志/展示用可读名:Server 头最直白(如 "Werkzeug/3.0"),
// 其次 <title>;都没有就标 "http"(vite/webpack 这类不带 Server 头,靠标题兜底)。
func previewLabel(s *pb.DevServer) string {
	if v := strings.TrimSpace(s.GetServerHeader()); v != "" {
		return v
	}
	if v := strings.TrimSpace(s.GetTitle()); v != "" {
		return v
	}
	return "http"
}
