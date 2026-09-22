// unknown_events_test.go —— 未知事件诊断 RPC 的回归用例(规格 §9)。
//
// 验收点:
//
//	① 列表按最新在前返回,可按会话/类别过滤,并带上累计与丢弃条数;
//	② 非法类别/状态 → InvalidArgument(不静默返回空,免得"参数写错"与"确实没有"混淆);
//	③ 详情取回脱敏载荷;不存在的 id → NotFound(而不是回一条空记录);
//	④ 确认更新状态;缺 action / 非法 action 都拒绝(处置状态是用户意志,不给默认值)。
package server

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

func newUnknownTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{Store: st, RuntimeBackend: runtime.NewBackend()})
	t.Cleanup(func() { _ = srv.events.Close() })
	return srv
}

func TestListUnknownEventsMapsAndFilters(t *testing.T) {
	srv := newUnknownTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()
	log.Record(runtime.EventEnvelope{SourceAgent: "codex", SourceMethod: "brandNewThing", ThreadID: "th-1"})
	log.Record(runtime.EventEnvelope{SourceAgent: "dsh", SourceMethod: "weird/request", NormalizedKind: runtime.KindUnknownRequest})

	resp, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{})
	if err != nil {
		t.Fatalf("ListUnknownEvents 失败: %v", err)
	}
	if len(resp.GetItems()) != 2 {
		t.Fatalf("应回 2 条,得到 %d", len(resp.GetItems()))
	}
	if resp.GetItems()[0].GetSourceMethod() != "weird/request" {
		t.Fatalf("应最新在前(诊断页问的是'最近来了什么'): %+v", resp.GetItems()[0])
	}
	if resp.GetTotal() != 2 {
		t.Fatalf("应带累计条数,得到 %d", resp.GetTotal())
	}

	byThread, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{ThreadId: "th-1"})
	if err != nil {
		t.Fatalf("按会话过滤失败: %v", err)
	}
	if len(byThread.GetItems()) != 1 || byThread.GetItems()[0].GetSourceMethod() != "brandNewThing" {
		t.Fatalf("按会话过滤结果不对: %+v", byThread.GetItems())
	}

	// 非法取值必须明说,而不是回一个空列表。
	if _, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{Kind: "NOPE"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("非法类别应为 InvalidArgument,得到 %v", err)
	}
	if _, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{ProcessingStatus: "NOPE"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("非法状态应为 InvalidArgument,得到 %v", err)
	}
}

func TestGetAndAcknowledgeUnknownEvent(t *testing.T) {
	srv := newUnknownTestServer(t)
	ctx := context.Background()
	e := srv.runtimeBackend.UnknownEvents().Record(runtime.EventEnvelope{
		SourceAgent: "codex", SourceMethod: "brandNewThing",
		RawPayload: `{"apikey":"[redacted]","note":"hi"}`,
	})

	got, err := srv.GetUnknownEvent(ctx, &pb.GetUnknownEventRequest{EventId: e.ID})
	if err != nil {
		t.Fatalf("GetUnknownEvent 失败: %v", err)
	}
	if got.GetRawJson() != `{"apikey":"[redacted]","note":"hi"}` {
		t.Fatalf("应回脱敏后的原始载荷: %q", got.GetRawJson())
	}
	if got.GetSummary().GetNormalizedKind() != runtime.KindUnknownNotification {
		t.Fatalf("类别不对: %q", got.GetSummary().GetNormalizedKind())
	}

	// 不存在的 id 必须说"找不到",不能回一条空记录(客户端会显示成"事件存在但没内容")。
	if _, err := srv.GetUnknownEvent(ctx, &pb.GetUnknownEventRequest{EventId: "ue-nope"}); status.Code(err) != codes.NotFound {
		t.Fatalf("不存在的 id 应为 NotFound,得到 %v", err)
	}

	ack, err := srv.AcknowledgeUnknownEvent(ctx, &pb.AcknowledgeUnknownEventRequest{
		EventId: e.ID, Action: runtime.StatusIgnored, Note: "已知噪音",
	})
	if err != nil {
		t.Fatalf("确认失败: %v", err)
	}
	if ack.GetItem().GetProcessingStatus() != runtime.StatusIgnored || ack.GetItem().GetRevision() != 1 {
		t.Fatalf("确认未生效: %+v", ack.GetItem())
	}

	// 处置状态是用户意志:不给默认值,非法值也拒绝。
	if _, err := srv.AcknowledgeUnknownEvent(ctx, &pb.AcknowledgeUnknownEventRequest{EventId: e.ID}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("缺 action 应为 InvalidArgument,得到 %v", err)
	}
	if _, err := srv.AcknowledgeUnknownEvent(ctx, &pb.AcknowledgeUnknownEventRequest{EventId: e.ID, Action: "WHATEVER"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("非法 action 应为 InvalidArgument,得到 %v", err)
	}
}

// TestUnknownEventsWithoutBackendReturnsEmpty 没有运行时后端时(极简构造)回空而不是报错。
//
// 【为什么】对客户端而言"没有记录"与"没有台账"看到的东西一样(空);报错会让诊断页
// 把正常空态显示成故障。
func TestUnknownEventsWithoutBackendReturnsEmpty(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{Store: st})
	t.Cleanup(func() { _ = srv.events.Close() })

	resp, err := srv.ListUnknownEvents(context.Background(), &pb.ListUnknownEventsRequest{})
	if err != nil {
		t.Fatalf("无后端时列表不该报错: %v", err)
	}
	if len(resp.GetItems()) != 0 {
		t.Fatalf("应回空清单,得到 %+v", resp.GetItems())
	}
}
