// panel_test.go —— 面板的钉子:仅回环启动校验、GET 渲染、POST 开关调用路径、
// Host 守卫与白名单拒绝。
//
// 【为什么必须钉】面板是"无鉴权 HTTP + 写通道"的组合,错一处就是安全事故或
// 静默丢写:
//  - 绑到非回环 = 管理面对整个局域网敞开(红线,启动即拒必须成立);
//  - POST 落到错的 CollabPolicy/RuntimePolicy 字段 = "改 A 结果 B 变了";
//  - CAS 丢失(ExpectedRevision 没带)= 两个界面互相覆盖;
//  - 词表外的 key 被放行 = 白名单红线名存实亡;
//  - 页面里混进 token/key = 鉴权信息外泄(红线 2)。
// 这些都是静态看不出来的错误,只有测试能拦。
package panel

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// fakeCtrl 内存假件:记录写调用并断言其形态,失败注入供冲突分支测试。
type fakeCtrl struct {
	mu sync.Mutex

	plane     *pb.ControlPlane
	collab    *pb.CollabPolicy
	collabRev uint64
	runtime   *pb.RuntimePolicy
	runtimeRev uint64

	setCollabCalls  int
	setRuntimeCalls int
	// 最后一次写请求携带的 CAS 期望代数(0 = 没带)。
	lastCollabCAS  uint64
	lastRuntimeCAS uint64
	// 置非 nil 时 SetCollabPolicy 如实返回错误(模拟 revision 冲突/落盘失败)。
	failSetCollab error
}

func (f *fakeCtrl) GetControlPlane(context.Context, *pb.GetControlPlaneRequest) (*pb.ControlPlane, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.plane, nil
}

func (f *fakeCtrl) GetCollabPolicy(context.Context, *pb.GetCollabPolicyRequest) (*pb.CollabPolicyResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return &pb.CollabPolicyResponse{Policy: f.collab, Revision: f.collabRev}, nil
}

func (f *fakeCtrl) SetCollabPolicy(_ context.Context, req *pb.SetCollabPolicyRequest) (*pb.CollabPolicyResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failSetCollab != nil {
		return nil, f.failSetCollab
	}
	f.setCollabCalls++
	f.lastCollabCAS = req.GetExpectedRevision()
	f.collab = req.GetPolicy()
	f.collabRev++
	return &pb.CollabPolicyResponse{Policy: f.collab, Revision: f.collabRev}, nil
}

func (f *fakeCtrl) GetRuntimePolicy(context.Context, *pb.GetRuntimePolicyRequest) (*pb.RuntimePolicyResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return &pb.RuntimePolicyResponse{Policy: f.runtime, Revision: f.runtimeRev}, nil
}

func (f *fakeCtrl) SetRuntimePolicy(_ context.Context, req *pb.SetRuntimePolicyRequest) (*pb.RuntimePolicyResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setRuntimeCalls++
	f.lastRuntimeCAS = req.GetExpectedRevision()
	f.runtime = req.GetPolicy()
	f.runtimeRev++
	return &pb.RuntimePolicyResponse{Policy: f.runtime, Revision: f.runtimeRev}, nil
}

// newFake 一份带八域骨架与既有策略的假件(形态对照 server/control_plane.go 的取值)。
func newFake() *fakeCtrl {
	return &fakeCtrl{
		plane: &pb.ControlPlane{
			DaemonVersion: "0.4.0", HostName: "YANZI-PC", Os: "Windows",
			DataDirLabel: "LOCALAPPDATA",
			Domains: []*pb.ControlDomain{
				{
					Id: "budget", Title: "资源预算", Writable: true,
					Summary: "全局预算已生效",
					Items: []*pb.ControlItem{{
						Id: "budget.global.ram_working_bytes", Label: "工作内存上限",
						Current: "8 MB", Kind: "number", Hint: "单个轮次在内存里干活的额度",
					}},
				},
				{
					Id: "collab", Title: "多 Agent 协同", Writable: true,
					Summary: "协同已开启",
					Items: []*pb.ControlItem{
						{Id: "collab.collaboration_enabled", Label: "协同总开关", Current: "开", Kind: "toggle", Hint: "关掉后 Agent 之间不能再互相派活"},
						{Id: "collab.native_delegate_enabled", Label: "委派子任务", Current: "关", Kind: "toggle"},
						{Id: "collab.fanout_enabled", Label: "并行扇出", Current: "开", Kind: "toggle"},
						{Id: "collab.词表外开关", Label: "来历不明", Current: "也许", Kind: "toggle"},
					},
				},
				{
					Id: "dialysis", Title: "未知事件", Writable: true,
					Summary: "没有待查看的未知事件",
					Items: []*pb.ControlItem{{
						Id: "dialysis.unread", Label: "待查看", Current: "0 条", Kind: "info",
					}},
				},
			},
		},
		collab:    &pb.CollabPolicy{CollaborationEnabled: boolPtr(true), FanoutEnabled: boolPtr(true)},
		collabRev: 3,
		runtime:   &pb.RuntimePolicy{},
	}
}

func boolPtr(b bool) *bool { return &b }

// postForm 以表单形态 POST 并返回响应(不经网络,直打 handler)。
func postForm(t *testing.T, h http.Handler, path string, form url.Values) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "127.0.0.1:7455"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Result()
}

// ---- 红线 1:仅回环绑定 ---------------------------------------------------------

func TestStartRejectsNonLoopback(t *testing.T) {
	for _, addr := range []string{
		"0.0.0.0:7455",    // 全部 IPv4 接口
		"192.168.1.7:7455", // 局域网地址
		":7455",           // 空 host = 全部接口
		"[::]:7455",       // 全部 IPv6 接口
		"",                // 空地址
		"7455",            // 缺 host
	} {
		url, stop, err := Start(addr, newFake())
		if err == nil {
			stop()
			t.Fatalf("Start(%q) 应拒绝非回环/非法地址,却返回了 %s", addr, url)
		}
		if url != "" || stop != nil {
			t.Fatalf("Start(%q) 失败时不应返回可用面板:%q", addr, url)
		}
	}
}

func TestStartBindsLoopbackAndServes(t *testing.T) {
	url, stop, err := Start("127.0.0.1:0", newFake())
	if err != nil {
		t.Fatalf("回环地址应能启动: %v", err)
	}
	defer stop()
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("面板 URL 应落在 127.0.0.1,得到 %q", url)
	}
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET 面板失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / 应 200,得到 %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "资源预算") {
		t.Fatal("面板应渲染八域卡片(缺「资源预算」)")
	}
}

// ---- GET 渲染 ------------------------------------------------------------------

func TestIndexRendersDomainsAndToggleForms(t *testing.T) {
	h := Handler(newFake())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "127.0.0.1:7455"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET / 应 200,得到 %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		"资源预算", "多 Agent 协同", "未知事件",      // 八域卡片(骨架)
		`action="/api/collab"`,                  // 协同开关表单
		`action="/api/unknown"`,                 // B 开关表单
		`name="key" value="collab"`,             // 白名单键入参
		`name="key" value="delegate"`,
		`name="key" value="fanout"`,
		`name="value" value="off"`,              // 总开关当前"开" → 表单切到关
		`content="10"`,                          // meta refresh 10s
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("面板页面缺少 %q\n%s", want, body)
		}
	}
	// 词表外 toggle 现值("也许")不得渲染表单(不猜布尔),只读呈现原文。
	if strings.Contains(body, `value="&#34;`) || strings.Contains(body, "collab.词表外开关\"") {
		t.Fatalf("词表外键不应成为表单入参")
	}
	if !strings.Contains(body, "来历不明") || !strings.Contains(body, "也许") {
		t.Fatal("词表外项应只读呈现原文(原文不丢)")
	}
}

func TestIndexRendersBSwitchState(t *testing.T) {
	f := newFake()
	on := true
	f.runtime = &pb.RuntimePolicy{UnknownRequestPassthrough: &on}
	h := Handler(f)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "127.0.0.1:7455"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	body := w.Body.String()
	// B 开关开 → 表单应切到 off;并带出人话状态。
	if !strings.Contains(body, `<li class="bswitch">`) {
		t.Fatal("未知事件卡应有 B 开关行")
	}
	if !strings.Contains(body, `action="/api/unknown"`) {
		t.Fatal("B 开关应有表单")
	}
}

// ---- POST 调用路径(与手机同一条 RPC 纪律)----------------------------------------

func TestPostCollabToggleWritesSameChannel(t *testing.T) {
	f := newFake()
	h := Handler(f)
	resp := postForm(t, h, "/api/collab", url.Values{"key": {"collab"}, "value": {"off"}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("成功切换应 303 回首页,得到 %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/" {
		t.Fatalf("303 应回 /,得到 %q", loc)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setCollabCalls != 1 {
		t.Fatalf("应恰好一次 SetCollabPolicy,得到 %d", f.setCollabCalls)
	}
	if f.collab.GetCollaborationEnabled() {
		t.Fatal("collab=off 应把 CollaborationEnabled 写为 false")
	}
	// 完整策略提交纪律:未改动的字段必须原样带回(缺席会被 daemon 重置)。
	if f.collab.GetFanoutEnabled() != true {
		t.Fatal("未改动的 fanout 必须原样带回 true(完整策略提交)")
	}
	// CAS 纪律:ExpectedRevision 必须带 Get 时的 revision。
	if f.lastCollabCAS != 3 {
		t.Fatalf("ExpectedRevision 应为 Get 时的 3,得到 %d", f.lastCollabCAS)
	}
}

func TestPostCollabDelegateAndFanout(t *testing.T) {
	f := newFake()
	h := Handler(f)
	for _, tc := range []struct{ key string; wantSet func(*pb.CollabPolicy) bool }{
		{"delegate", func(p *pb.CollabPolicy) bool { return p.GetNativeDelegateEnabled() }},
		{"fanout", func(p *pb.CollabPolicy) bool { return p.GetFanoutEnabled() }},
	} {
		resp := postForm(t, h, "/api/collab", url.Values{"key": {tc.key}, "value": {"on"}})
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("%s=on 应 303,得到 %d", tc.key, resp.StatusCode)
		}
		f.mu.Lock()
		if !tc.wantSet(f.collab) {
			f.mu.Unlock()
			t.Fatalf("%s=on 未落到对应字段", tc.key)
		}
		f.mu.Unlock()
	}
}

func TestPostUnknownToggleWritesRuntimePolicy(t *testing.T) {
	f := newFake()
	h := Handler(f)
	resp := postForm(t, h, "/api/unknown", url.Values{"value": {"on"}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("成功切换应 303,得到 %d", resp.StatusCode)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setRuntimeCalls != 1 {
		t.Fatalf("应恰好一次 SetRuntimePolicy,得到 %d", f.setRuntimeCalls)
	}
	if !f.runtime.GetUnknownRequestPassthrough() {
		t.Fatal("unknown=on 应把 UnknownRequestPassthrough 写为 true")
	}
	if f.lastRuntimeCAS != 0 {
		t.Fatalf("ExpectedRevision 应为 Get 时的 0,得到 %d", f.lastRuntimeCAS)
	}
}

// ---- 白名单与错误分支 ------------------------------------------------------------

func TestPostCollabRejectsUnknownKey(t *testing.T) {
	f := newFake()
	h := Handler(f)
	resp := postForm(t, h, "/api/collab", url.Values{"key": {"自创键"}, "value": {"on"}})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("白名单外 key 应 400,得到 %d", resp.StatusCode)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setCollabCalls != 0 {
		t.Fatalf("白名单外 key 不得产生写调用,实际 %d 次", f.setCollabCalls)
	}
}

func TestPostRejectsBadValue(t *testing.T) {
	f := newFake()
	h := Handler(f)
	for _, path := range []string{"/api/collab", "/api/unknown"} {
		form := url.Values{"value": {"maybe"}}
		if path == "/api/collab" {
			form.Set("key", "collab")
		}
		resp := postForm(t, h, path, form)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s 的非法 value 应 400,得到 %d", path, resp.StatusCode)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setCollabCalls != 0 || f.setRuntimeCalls != 0 {
		t.Fatal("非法 value 不得产生写调用")
	}
}

func TestPostCollabSurfacesCASConflict(t *testing.T) {
	f := newFake()
	f.failSetCollab = errors.New("协同策略已被改动(revision=4,期望 3),请重新读取后再提交")
	h := Handler(f)
	resp := postForm(t, h, "/api/collab", url.Values{"key": {"collab"}, "value": {"off"}})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("CAS 冲突应如实 400(不伪装成功),得到 %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "已被改动") {
		t.Fatalf("冲突原因应原样透出,得到:%s", body)
	}
}

func TestWrongMethodRejected(t *testing.T) {
	h := Handler(newFake())
	req := httptest.NewRequest(http.MethodGet, "/api/collab", nil)
	req.Host = "127.0.0.1:7455"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/collab 应 405,得到 %d", w.Code)
	}
}

// ---- Host 守卫(拦 DNS rebinding)------------------------------------------------

func TestLoopbackGuardRejectsForeignHost(t *testing.T) {
	h := Handler(newFake())
	for _, host := range []string{"attacker.example", "evil.com:80", "10.0.0.9:7455", ""} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("Host=%q 应 403,得到 %d", host, w.Code)
		}
	}
	// 合法回环 Host 放行(端口形态、localhost、IPv6 括号)。
	for _, host := range []string{"127.0.0.1:7455", "localhost:7455", "[::1]:7455"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code == http.StatusForbidden {
			t.Fatalf("Host=%q 是回环,不应被拒", host)
		}
	}
}

// ---- 红线 2:页面无鉴权信息 --------------------------------------------------------

func TestPageContainsNoSecrets(t *testing.T) {
	h := Handler(newFake())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "127.0.0.1:7455"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	body := strings.ToLower(w.Body.String())
	for _, secret := range []string{"token", "bearer", "pair-code", "password", "authorization", "api_key", "apikey"} {
		if strings.Contains(body, secret) {
			t.Fatalf("页面不得出现 %q(红线:无鉴权信息)", secret)
		}
	}
}
