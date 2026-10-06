// Package panel —— daemon 进程内的本地 Web 管理面板(2026-09-30,默认关)。
//
// 【它解决什么】电脑端此前只有 controlplaneshow 这类命令行报表与 -set 开关,用户要求
// "电脑端可见的统一控制面":浏览器里能看到八域总览、能直接点开关。本包在 daemon
// 进程内起一个**仅绑定回环**的 HTTP 服务,页面数据与手机控制中心同源
// (GetControlPlane),开关写动作与手机走**完全相同**的 RPC 路径
// (SetCollabPolicy / SetRuntimePolicy 的进程内直调 —— 同一把锁、同一份落盘、
// 同一套 CAS),不存在"两边各写各的"。
//
// 【安全红线(三条,全部由代码强制)】
//  1. 只绑回环:Start 对传入地址做预检(非回环直接拒绝启动),监听成功后再核验
//     内核实际绑定的地址必须 IsLoopback(双保险,localhost 解析结果以内核为准);
//     另有请求级 Host 头守卫拦 DNS rebinding(远端网页把域名解析到 127.0.0.1
//     借用户浏览器打内下面板的经典手法)。
//  2. 无鉴权信息不外泄:页面不含 token/key/配对码 —— Controller 接口根本没有
//     任何取 token 的面,想泄也没门。
//  3. 只读 + 白名单开关:GET / 只读;POST 只有两条(协同三开关、未知请求透传
//     B 开关),键白名单校验,词表外一律拒绝 —— 不给"任意写"留口子。
//
// 【实现最稳的调用路径】daemon 进程内直接调自己的 server 方法(而非自拨回环
// gRPC):少一层拨号/TLS/token 装配,与本进程零网络往返,且鉴权拦截器天然不
// 参与(面板是本机用户自己的操作面,与 gRPC 设备鉴权是两回事)。
package panel

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// Controller 面板依赖的最小控制面(生产由 *server.Server 直接满足:方法签名
// 一致,无需任何适配)。测试塞假件,与 server 包的探针注入同款取舍。
type Controller interface {
	GetControlPlane(context.Context, *pb.GetControlPlaneRequest) (*pb.ControlPlane, error)
	GetCollabPolicy(context.Context, *pb.GetCollabPolicyRequest) (*pb.CollabPolicyResponse, error)
	SetCollabPolicy(context.Context, *pb.SetCollabPolicyRequest) (*pb.CollabPolicyResponse, error)
	GetRuntimePolicy(context.Context, *pb.GetRuntimePolicyRequest) (*pb.RuntimePolicyResponse, error)
	SetRuntimePolicy(context.Context, *pb.SetRuntimePolicyRequest) (*pb.RuntimePolicyResponse, error)
}

// Start 校验地址仅回环、监听并在后台 goroutine 提供 HTTP 面板。
//
// 返回面板 URL(以内核实际绑定地址拼出,-panel 127.0.0.1:0 也能如实上报)与
// stop 函数;非回环地址(含空 host = 绑全部网卡)直接返回错误,调用方应当场
// 拒绝启动 —— 这是红线,不是警告。
func Start(addr string, ctrl Controller) (url string, stop func(), err error) {
	if strings.TrimSpace(addr) == "" {
		return "", nil, fmt.Errorf("面板地址为空")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", nil, fmt.Errorf("面板地址需要 host:port 形式(如 127.0.0.1:7455): %w", err)
	}
	if !isLoopbackHost(host) {
		return "", nil, fmt.Errorf("管理面板只允许绑定回环地址(127.0.0.1/::1/localhost),拒绝 %q", host)
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return "", nil, err
	}
	// 双保险:内核实际绑定的地址必须回环。"localhost" 由系统解析,可能落
	// ::1 或 127.0.0.1;以监听结果为准,解析出非回环(被劫持的 hosts 等)就地拒绝。
	if tcp, ok := lis.Addr().(*net.TCPAddr); !ok || !tcp.IP.IsLoopback() {
		lis.Close()
		return "", nil, fmt.Errorf("管理面板实际绑定地址非回环(%s),拒绝启动", lis.Addr())
	}
	// ReadHeaderTimeout:慢速请求头洪泛(slowloris)的兜底。仅回环监听风险本就
	// 低,但零成本补上 —— 没有它时恶意/卡死的本机进程可长期占住连接。
	srv := &http.Server{Handler: Handler(ctrl), ReadHeaderTimeout: 10 * time.Second}
	done := make(chan struct{})
	go func() {
		defer close(done)
		// 进程退出或 stop 时 Serve 返回;本地面板的错误无需向调用方传播
		// (启动成功与否已在上面定论,运行中关闭是预期路径)。
		_ = srv.Serve(lis)
	}()
	url = "http://" + lis.Addr().String()
	return url, func() { _ = srv.Close(); <-done }, nil
}

// Handler 组装面板路由:GET / 只读总览;POST /api/collab、/api/unknown 两条
// 白名单开关。外层套回环守卫(Host 头校验,拦 DNS rebinding)。
func Handler(ctrl Controller) http.Handler {
	h := &pageHandler{ctrl: ctrl}
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.serveIndex)
	mux.HandleFunc("/api/collab", h.serveCollab)
	mux.HandleFunc("/api/unknown", h.serveUnknown)
	return loopbackGuard{next: mux}
}

// loopbackGuard 请求级回环守卫:Host 头必须是回环形态,否则 403。
//
// 【为什么监听回环还要查 Host】监听只挡"谁能建立 TCP 连接";DNS rebinding 的
// 攻击流量同样来自 127.0.0.1(受害者的浏览器替远端网页发请求)。浏览器发出的
// Host 头携带页面域名,rebinding 域名的 Host 不是回环形态 —— 一行校验就能拦下,
// 而本机用户直连 127.0.0.1:7455 时 Host 恰是回环,正常使用零感知。
//
// 【为什么写方法还要查 Origin/Sec-Fetch-Site(CSRF 守卫)】Host 守卫拦得住
// rebinding,拦不住经典 CSRF:恶意网页直接向 http://127.0.0.1:7455 发跨站
// 表单/no-cors fetch POST,此时 Host 恰是回环形态、守卫全绿 —— 而 POST 的
// 响应虽不可读,开关的**副作用**已经落地(协同/委派/扇出、B 开关被陌生网页
// 静默翻转)。浏览器对跨站 POST 必带 Origin 头(非回环来源即拒);现代浏览器
// 另带 Sec-Fetch-Site(cross-site 即拒)。两道都缺 = 非浏览器客户端(curl、
// 运维脚本),放行 —— 面板的合法写方只有本机页面与脚本。
type loopbackGuard struct{ next http.Handler }

func (g loopbackGuard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackHost(r.Host) {
		http.Error(w, "管理面板仅限本机回环访问(Host 头非回环已拒绝)", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if org := strings.TrimSpace(r.Header.Get("Origin")); org != "" && !isLoopbackHost(originHost(org)) {
			http.Error(w, "管理面板写操作拒绝跨站来源(Origin 非本机回环)", http.StatusForbidden)
			return
		}
		switch strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")) {
		case "", "same-origin", "same-site", "none":
			// 缺头(非浏览器)或同源/同站/用户直开:放行。
		default:
			http.Error(w, "管理面板写操作拒绝跨站请求(Sec-Fetch-Site)", http.StatusForbidden)
			return
		}
	}
	g.next.ServeHTTP(w, r)
}

// originHost 从 Origin 头(形如 "http://127.0.0.1:7455")里剥出 host[:port],
// 交给 isLoopbackHost 判定。无 scheme(如 "null")时原样返回 —— 判否,安全方向。
func originHost(origin string) string {
	if i := strings.Index(origin, "://"); i >= 0 {
		origin = origin[i+3:]
	}
	if i := strings.IndexAny(origin, "/?"); i >= 0 {
		origin = origin[:i]
	}
	return origin
}

// isLoopbackHost 判断 host(可带端口,可 IPv6 括号)是否回环形态。
// 空串不是回环(等价"任意接口"),显式拒绝。
func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

// ---- 写通道:两条白名单开关(与手机控制中心同一批 RPC,先 Get 再完整 Set + CAS)--

// serveCollab POST /api/collab:form 字段 key(collab|delegate|fanout)+ value(on|off)。
func (h *pageHandler) serveCollab(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "表单解析失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	want, ok := parseToggleValue(r.FormValue("value"))
	if !ok {
		http.Error(w, `value 仅支持 "on"/"off"`, http.StatusBadRequest)
		return
	}
	// Get → 改单字段 → 完整 Set + ExpectedRevision(CAS):与手机端
	// RuntimeControlState.saveCollabPolicy、controlplaneshow -set 同一条纪律。
	// Set 提交的是完整策略,缺席项会被重置 —— 必须带回 daemon 读回的那份完整对象。
	got, err := h.ctrl.GetCollabPolicy(r.Context(), &pb.GetCollabPolicyRequest{})
	if err != nil {
		http.Error(w, "读取协同策略失败: "+err.Error(), http.StatusBadGateway)
		return
	}
	pol := got.GetPolicy()
	if pol == nil {
		pol = &pb.CollabPolicy{}
	}
	switch r.FormValue("key") {
	case "collab":
		pol.CollaborationEnabled = &want
	case "delegate":
		pol.NativeDelegateEnabled = &want
	case "fanout":
		pol.FanoutEnabled = &want
	default:
		// 白名单外:拒绝且不产生任何写调用(红线 3)。
		http.Error(w, "未知开关键(可用:collab/delegate/fanout)", http.StatusBadRequest)
		return
	}
	if _, err := h.ctrl.SetCollabPolicy(r.Context(), &pb.SetCollabPolicyRequest{
		Policy: pol, ExpectedRevision: got.GetRevision(),
	}); err != nil {
		http.Error(w, "写入协同策略失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	// 成功后 303 回首页:浏览器发起 GET /,页面数据重新聚合 —— "每次操作后
	// 回读刷新"由这个跳转天然达成,页面不做任何乐观改写。
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// serveUnknown POST /api/unknown:form 字段 value(on|off),写 RuntimePolicy 的
// unknown_request_passthrough(辅线 B 开关;与手机设置页实验区同语义同通道)。
func (h *pageHandler) serveUnknown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "表单解析失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	want, ok := parseToggleValue(r.FormValue("value"))
	if !ok {
		http.Error(w, `value 仅支持 "on"/"off"`, http.StatusBadRequest)
		return
	}
	got, err := h.ctrl.GetRuntimePolicy(r.Context(), &pb.GetRuntimePolicyRequest{})
	if err != nil {
		http.Error(w, "读取运行时策略失败: "+err.Error(), http.StatusBadGateway)
		return
	}
	pol := got.GetPolicy()
	if pol == nil {
		pol = &pb.RuntimePolicy{}
	}
	pol.UnknownRequestPassthrough = &want
	if _, err := h.ctrl.SetRuntimePolicy(r.Context(), &pb.SetRuntimePolicyRequest{
		Policy: pol, ExpectedRevision: got.GetRevision(),
	}); err != nil {
		http.Error(w, "写入运行时策略失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// parseToggleValue "on"→(true,true)、"off"→(false,true);其余(含空)拒绝。
func parseToggleValue(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on":
		return true, true
	case "off":
		return false, true
	}
	return false, false
}

// pageHandler 承载 Controller,路由函数挂在它上面。
type pageHandler struct{ ctrl Controller }

// serveIndex GET /:聚合读(GetControlPlane + GetRuntimePolicy)并渲染总览页。
func (h *pageHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "仅支持 GET", http.StatusMethodNotAllowed)
		return
	}
	cp, err := h.ctrl.GetControlPlane(r.Context(), &pb.GetControlPlaneRequest{})
	if err != nil {
		http.Error(w, "读取控制面失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rp, err := h.ctrl.GetRuntimePolicy(r.Context(), &pb.GetRuntimePolicyRequest{})
	if err != nil {
		http.Error(w, "读取运行时策略失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = renderIndex(w, cp, rp)
}
