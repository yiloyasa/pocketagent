package server

// dev server 探测(P2)的单测。
//
// 【设计】fakeDialer 用 net.Pipe 把 canned 响应回给**生产探针代码**,即 HTTP
// 解析、title 提取、超时预算走的都是真实路径,fake 只替掉 TCP 拨号 —— 不碰
// 真网络,端口占用为零,结果确定。缓存 TTL 用注入假时钟,不必真睡 60s。

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// fakeDialer:open[端口]=canned 响应;表里没有(或为空串)= 模拟"连接拒绝"。
// 拨号计数是原子的:scanDevServers 并发拨号。
type fakeDialer struct {
	open  map[int]string
	dials atomic.Int32
}

func (f *fakeDialer) DialContext(_ context.Context, _ string, addr string) (net.Conn, error) {
	f.dials.Add(1)
	_, portStr, splitErr := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)
	resp, ok := f.open[port]
	if splitErr != nil || !ok || resp == "" {
		return nil, fmt.Errorf("dial tcp %s: connect: connection refused (fake)", addr)
	}
	c1, c2 := net.Pipe()
	go func() {
		// Pipe 是同步的:先吃掉探针的请求(不读会卡住对端 Write),再回 canned 响应。
		buf := make([]byte, 512)
		_, _ = c2.Read(buf)
		_, _ = io.WriteString(c2, resp)
		c2.Close() // 关闭让客户端读到 EOF(响应体结束)
	}()
	return c1, nil
}

func (f *fakeDialer) dialCount() int { return int(f.dials.Load()) }

// fakeClock 可拨动的假时钟(TTL 单测用)。
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

// httpResp 造一个最小 HTTP/1.0 响应。探针发的是 HTTP/1.0,对端不会用
// chunked,这里按原始字节回即可被 http.ReadResponse + extractTitle 消费。
func httpResp(server, title string) string {
	var b strings.Builder
	b.WriteString("HTTP/1.0 200 OK\r\n")
	if server != "" {
		b.WriteString("Server: " + server + "\r\n")
	}
	b.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
	b.WriteString("<!doctype html><html><head><title>" + title + "</title></head>" +
		"<body><div id=\"app\"></div></body></html>")
	return b.String()
}

// testPreviewReset 复位全局缓存与注入点,避免用例间互相污染。
func testPreviewReset() {
	previewCache.mu.Lock()
	previewCache.servers = nil
	previewCache.at = time.Time{}
	previewCache.now = nil // nil = 生产真实时间
	previewCache.mu.Unlock()
	previewDialer = nil
}

func TestProbePortDetectsServerAndTitle(t *testing.T) {
	testPreviewReset()
	defer testPreviewReset()
	previewDialer = &fakeDialer{open: map[int]string{
		5173: httpResp("vite", "Heart Site Dev"),
	}}
	srv := probePort(context.Background(), 5173)
	if srv == nil {
		t.Fatal("期望 5173 被识别为 HTTP 服务,实际为 nil")
	}
	if srv.GetPort() != 5173 {
		t.Errorf("Port = %d, 期望 5173", srv.GetPort())
	}
	if srv.GetServerHeader() != "vite" {
		t.Errorf("ServerHeader = %q, 期望 \"vite\"", srv.GetServerHeader())
	}
	if srv.GetTitle() != "Heart Site Dev" {
		t.Errorf("Title = %q, 期望 \"Heart Site Dev\"", srv.GetTitle())
	}
	if srv.GetUrl() != "http://127.0.0.1:5173" {
		t.Errorf("Url = %q, 期望 \"http://127.0.0.1:5173\"", srv.GetUrl())
	}
}

func TestProbePortNoServerHeaderTitleFallback(t *testing.T) {
	testPreviewReset()
	defer testPreviewReset()
	// vite/webpack 这类 dev server 通常不带 Server 头:字段为空,可读名回退 <title>。
	previewDialer = &fakeDialer{open: map[int]string{
		3000: httpResp("", "My React App"),
	}}
	srv := probePort(context.Background(), 3000)
	if srv == nil {
		t.Fatal("期望 3000 被识别为 HTTP 服务,实际为 nil")
	}
	if srv.GetServerHeader() != "" {
		t.Errorf("ServerHeader = %q, 期望空", srv.GetServerHeader())
	}
	if got := previewLabel(srv); got != "My React App" {
		t.Errorf("previewLabel = %q, 期望回退到标题 \"My React App\"", got)
	}
}

func TestProbePortRejectsNonHTTP(t *testing.T) {
	testPreviewReset()
	defer testPreviewReset()
	// 端口开着但不是 HTTP(SSH 横幅):绝不能混进预览列表。
	previewDialer = &fakeDialer{open: map[int]string{
		9000: "SSH-2.0-OpenSSH_9.4\r\nProtocol mismatch.\r\n",
	}}
	if srv := probePort(context.Background(), 9000); srv != nil {
		t.Fatalf("非 HTTP 应答不应被识别,得到 %+v", srv)
	}
}

func TestProbePortClosedPort(t *testing.T) {
	testPreviewReset()
	defer testPreviewReset()
	previewDialer = &fakeDialer{open: map[int]string{}} // 全部拒连:绝大多数端口的常态
	if srv := probePort(context.Background(), 8080); srv != nil {
		t.Fatalf("拒连端口应返回 nil,得到 %+v", srv)
	}
}

func TestScanDevServersSortedAndSkipsClosed(t *testing.T) {
	testPreviewReset()
	defer testPreviewReset()
	previewDialer = &fakeDialer{open: map[int]string{
		8080: httpResp("Werkzeug/3.0 Python/3.12", "Flask App"),
		3000: httpResp("", "Next.js"),
		5173: httpResp("vite", ""),
		// 3001/4200/5000/5174/8000/8081/8888/9000 不在表里 = 关闭,应被跳过
	}}
	out := scanDevServers(context.Background())
	if len(out) != 3 {
		t.Fatalf("期望发现 3 个服务,得到 %d:%v", len(out), out)
	}
	if out[0].GetPort() != 3000 || out[1].GetPort() != 5173 || out[2].GetPort() != 8080 {
		t.Fatalf("结果应按端口升序,实际顺序: %d,%d,%d",
			out[0].GetPort(), out[1].GetPort(), out[2].GetPort())
	}
}

func TestDevServerCacheTTL(t *testing.T) {
	testPreviewReset()
	defer testPreviewReset()
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	previewCache.now = clock.Now
	fd := &fakeDialer{open: map[int]string{5173: httpResp("vite", "Heart Site Dev")}}
	previewDialer = fd
	ctx := context.Background()

	cachedDevServers(ctx) // 首次:真实扫描(拨端口表里每一个端口)
	if got := fd.dialCount(); got != len(devServerPorts) {
		t.Fatalf("首次应真实扫描 1 轮(每端口一拨,共 %d),实际拨号 %d 次", len(devServerPorts), got)
	}
	cachedDevServers(ctx) // 60s 内:命中缓存
	clock.Advance(59 * time.Second)
	cachedDevServers(ctx) // 累计 59s:仍命中
	if got := fd.dialCount(); got != len(devServerPorts) {
		t.Fatalf("TTL 内不应重扫,拨号已达 %d 次", got)
	}
	clock.Advance(2 * time.Second) // 累计 61s > 60s TTL:重扫
	cachedDevServers(ctx)
	if got := fd.dialCount(); got != 2*len(devServerPorts) {
		t.Fatalf("过期后应重扫(拨号应为 %d 次),实际 %d 次", 2*len(devServerPorts), got)
	}
}

func TestListDevServersRPC(t *testing.T) {
	testPreviewReset()
	defer testPreviewReset()
	previewDialer = &fakeDialer{open: map[int]string{
		5173: httpResp("vite", "Heart Site Dev"),
	}}
	s := &Server{} // RPC 只走缓存与扫描,不依赖 Server 其他字段
	resp, err := s.ListDevServers(context.Background(), &pb.ListDevServersRequest{})
	if err != nil {
		t.Fatalf("ListDevServers 报错: %v", err)
	}
	if len(resp.GetServers()) != 1 || resp.GetServers()[0].GetPort() != 5173 {
		t.Fatalf("应只发现 :5173,实际 %v", resp.GetServers())
	}
	if resp.GetScannedAtUnixMs() <= 0 {
		t.Errorf("ScannedAtUnixMs 应为正,实际 %d", resp.GetScannedAtUnixMs())
	}
	if _, err = s.ListDevServers(context.Background(), &pb.ListDevServersRequest{}); err != nil {
		t.Fatalf("第二次调用报错: %v", err)
	}
	if got := previewDialer.(*fakeDialer).dialCount(); got != len(devServerPorts) {
		t.Errorf("第二次调用应走缓存(拨号仍应为 1 轮 = %d 次),实际 %d 次", len(devServerPorts), got)
	}
}
