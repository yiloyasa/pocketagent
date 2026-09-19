// pi_export_route_test.go —— ExportThreadHtml RPC 的路由与失败透传回归。
//
// 【为什么必须钉死】导出是 pi 侧独有能力(PiProvider.ExportHTML),路由错 =
// 用户按"导出 HTML"拿到报错或干脆导出了错误会话;provider 缺能力时必须显式
// 报"没有可用的 pi",绝不静默降级。fake 注入遵循 piCompactStub 同款最小接口模式。
package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// piExportStub 在 PiProviderStub(纯 provider 接口)之上补 ExportHTML,
// 记录调用并按注入值返回 —— 不真拉 pi 子进程渲染。
type piExportStub struct {
	PiProviderStub
	calls int
	path  string
	err   error
}

func (s *piExportStub) ExportHTML(*store.Thread) (string, error) {
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	return s.path, nil
}

// piExportThread 建一个已落地(挂载过)的 pi 线程。
func piExportThread(id string) *store.Thread {
	return &store.Thread{
		ID: id, AgentID: "pi",
		ExternalSession: store.ExternalSessionRef{SessionFile: `C:\sessions\` + id + ".jsonl"},
	}
}

// pi 路径:provider 被恰好调一次,电脑上的绝对路径原样透出。
func TestExportThreadHtmlRoutesToPi(t *testing.T) {
	st := mustStore(t)
	th := piExportThread("th-piexp-1")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	exp := &piExportStub{path: `C:\ws\.pocketagent-exports\th-piexp-1.html`}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": exp}})
	res, err := srv.ExportThreadHtml(context.Background(), &pb.ExportThreadHtmlRequest{ThreadId: th.ID})
	if err != nil {
		t.Fatalf("pi 路径应成功: %v", err)
	}
	if exp.calls != 1 {
		t.Fatalf("ExportHTML 应恰好被调一次,实际 %d", exp.calls)
	}
	if res.Path != exp.path {
		t.Fatalf("应透出电脑上的绝对路径: got %q want %q", res.Path, exp.path)
	}
}

// pi 侧失败(未落地 / 渲染失败)必须如实变成 gRPC 错误并带人话原因,不能假成功。
func TestExportThreadHtmlPropagatesFailure(t *testing.T) {
	st := mustStore(t)
	th := piExportThread("th-piexp-2")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	exp := &piExportStub{err: errors.New("export_html 失败: 子进程已退出")}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": exp}})
	_, err := srv.ExportThreadHtml(context.Background(), &pb.ExportThreadHtmlRequest{ThreadId: th.ID})
	if status.Code(err) != codes.Internal {
		t.Fatalf("失败应映射为 Internal,实际 %v", err)
	}
	if !strings.Contains(err.Error(), "export_html 失败") {
		t.Fatalf("应透传 pi 的失败原因: %v", err)
	}
}

// 非 pi 会话显式人话报错(FailedPrecondition),且不触发任何导出动作。
func TestExportThreadHtmlRejectsNonPi(t *testing.T) {
	st := mustStore(t)
	th := &store.Thread{ID: "th-codex-1", AgentID: "codex", CodexThreadID: "cx-1"}
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	exp := &piExportStub{}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{
		"pi": exp,
		// codex 在场也不该被选去导出:能力按会话归属路由,不静默换人
		"codex": &PiProviderStub{},
	}})
	_, err := srv.ExportThreadHtml(context.Background(), &pb.ExportThreadHtmlRequest{ThreadId: th.ID})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("非 pi 会话应报 FailedPrecondition,实际 %v", err)
	}
	if exp.calls != 0 {
		t.Fatalf("不应触发 pi 导出,实际调了 %d 次", exp.calls)
	}
}

// 注册表里的 pi 不具备导出能力(如 stub/旧实现)时显式报"没有可用的 pi"。
func TestExportThreadHtmlWithoutExporter(t *testing.T) {
	st := mustStore(t)
	th := piExportThread("th-piexp-3")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": &PiProviderStub{}}})
	_, err := srv.ExportThreadHtml(context.Background(), &pb.ExportThreadHtmlRequest{ThreadId: th.ID})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("pi 无导出能力应报 Unavailable,实际 %v", err)
	}
	if !strings.Contains(err.Error(), "pi") {
		t.Fatalf("应报 pi 不可用: %v", err)
	}
}

// 入参与会话存在性:空 id → InvalidArgument;未知 id → NotFound。
func TestExportThreadHtmlBadRequestAndMissing(t *testing.T) {
	st := mustStore(t)
	exp := &piExportStub{}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": exp}})
	if _, err := srv.ExportThreadHtml(context.Background(), &pb.ExportThreadHtmlRequest{ThreadId: " "}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("空 id 应报 InvalidArgument,实际 %v", err)
	}
	if _, err := srv.ExportThreadHtml(context.Background(), &pb.ExportThreadHtmlRequest{ThreadId: "th-不存在"}); status.Code(err) != codes.NotFound {
		t.Fatalf("未知会话应报 NotFound,实际 %v", err)
	}
	if exp.calls != 0 {
		t.Fatalf("失败请求不应触发导出,实际调了 %d 次", exp.calls)
	}
}
