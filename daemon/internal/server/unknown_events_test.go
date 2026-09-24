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
	"strings"
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

// TestUnknownEventListSummariesAreHumanReadable 列表页每项都要带人话摘要。
//
// 【钉什么】已知事件与未知事件都要翻:已知翻成"daemon 对它做了什么"(比方法名
// 有用),未知翻成"它长什么样、像不像错误"。翻译失败宁可留空也不给错翻译 ——
// 但本用例的两个输入都必须翻出来,空摘要在这里算失败。
func TestUnknownEventListSummariesAreHumanReadable(t *testing.T) {
	srv := newUnknownTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()
	log.Record(runtime.EventEnvelope{
		SourceAgent: "codex", SourceMethod: "turn.completed", NormalizedKind: runtime.KindKnownProjected,
	})
	log.Record(runtime.EventEnvelope{
		SourceAgent: "pi", SourceMethod: "brandNewPiEvent", RawPayload: `{"error":{"message":"boom"}}`,
	})

	resp, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{})
	if err != nil {
		t.Fatalf("ListUnknownEvents 失败: %v", err)
	}
	seen := map[string]string{}
	for _, item := range resp.GetItems() {
		if item.GetSummary() == "" {
			t.Fatalf("事件 %s 的 summary 不得为空(人话翻译是本轮的交付)", item.GetEventId())
		}
		seen[item.GetSourceMethod()] = item.GetSummary()
	}
	if s := seen["turn.completed"]; strings.Contains(s, "turn.completed") {
		t.Errorf("已知事件的 summary 复述了方法名: %s", s)
	}
	if s := seen["brandNewPiEvent"]; !strings.Contains(s, "错误上报") {
		t.Errorf("未知事件含 error 字段,summary 应判'像一次错误上报': %s", s)
	}
}

// TestGetUnknownEventCarriesDigestAndTranslation 详情页要给字段级透析表与详细解读。
func TestGetUnknownEventCarriesDigestAndTranslation(t *testing.T) {
	srv := newUnknownTestServer(t)
	ctx := context.Background()
	e := srv.runtimeBackend.UnknownEvents().Record(runtime.EventEnvelope{
		SourceAgent: "zcode", SourceMethod: "brandNewZcodeEvent",
		RawPayload: `{"threadId":"th-9","count":3,"token":"[redacted]","zzUnfamiliar":"v"}`,
	})

	got, err := srv.GetUnknownEvent(ctx, &pb.GetUnknownEventRequest{EventId: e.ID})
	if err != nil {
		t.Fatalf("GetUnknownEvent 失败: %v", err)
	}
	digest := map[string]*pb.UnknownEventField{}
	for _, f := range got.GetDigest() {
		digest[f.GetKey()] = f
	}
	if len(digest) != 4 {
		t.Fatalf("digest 应与载荷字段一一对应(4 个),得到 %d: %+v", len(digest), got.GetDigest())
	}
	if f := digest["threadId"]; f.GetKind() != "text" || f.GetMeaning() != "会话标识" || f.GetPreview() != "th-9" {
		t.Errorf("threadId 行不对: %+v", f)
	}
	if f := digest["count"]; f.GetKind() != "number" || f.GetMeaning() != "数量" {
		t.Errorf("count 行不对: %+v", f)
	}
	if f := digest["zzUnfamiliar"]; f.GetMeaning() != "" {
		t.Errorf("识别不出的字段含义必须留空不猜: %+v", f)
	}
	if !strings.Contains(got.GetTranslation(), "原文") {
		t.Errorf("translation 缺'以原文为准'红线句: %s", got.GetTranslation())
	}
	if !strings.Contains(got.GetTranslation(), "留痕") {
		t.Errorf("translation 应写明 daemon 处置(未知通知已留痕): %s", got.GetTranslation())
	}
}
