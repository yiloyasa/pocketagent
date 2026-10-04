// unknown_actions_test.go —— 未知事件用户动作安全执行器的回归用例
// (2026-10-04 未知事件用户处理中心计划 Task 6;波末接线后请求/响应用 wire 类型)。
//
// 验收点:
//
//	① revision 冲突回结构化冲突结果(ok=false/conflict=true,人话说明 + 当前摘要
//	   + new_revision),事件状态分毫不动;
//	② 四个安全动作各自成功路径:inspect 不改状态,状态型动作 revision +1、状态到位;
//	③ 幂等:相同动作 + 正确 revision 重复提交 ok=true 返回当前状态;stale 重放
//	   回冲突 —— CAS 先于幂等(2026-10-04 门禁裁定),版本对不上不给"幂等"通行证;
//	④ 白名单封口:未知动作与 approve/execute/allow 一律 InvalidArgument,copy 给专门说明;
//	⑤ expected_revision 口径:负数拒绝;inspect 的 0 免 CAS(纯读),状态型动作的 0
//	   照常比较(0 是新事件的合法版本号);
//	⑥ 处置历史逐次落账(内存快照与盘上文件一致,含 event_id/动作/时间/结果);
//	⑦ 并发同一事件:结果集合确定 —— 无论不同动作还是相同动作并发,后到者版本
//	   已过期回冲突,恰一生效(本用例应配 -race 跑,锁内临界区是它的被测对象)。
package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// newUnknownActionTestServer 与 newUnknownTestServer 的差别只有一处:显式给
// DataDir —— 处置历史账本按 dataDir 建实例并落盘,每个测试独立目录互不串账
// (账本实例按 dataDir 复用,不给 DataDir 会落到共享的内存空账,断言就不干净了)。
func newUnknownActionTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{
		Store:          st,
		RuntimeBackend: runtime.NewBackend(),
		DataDir:        t.TempDir(),
	})
	t.Cleanup(func() { _ = srv.events.Close() })
	return srv
}

func TestUnknownActionRevisionConflict(t *testing.T) {
	srv := newUnknownActionTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()
	e := log.Record(runtime.EventEnvelope{SourceAgent: "codex", SourceMethod: "brandNew"})

	// 另一端先处理了一次:revision 0 → 1,状态 IGNORED。
	if _, ok, err := log.Acknowledge(e.ID, runtime.StatusIgnored, "已知噪音"); err != nil || !ok {
		t.Fatalf("预置已处理状态失败: ok=%v err=%v", ok, err)
	}

	got, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ExpectedRevision: 0, ActionId: unknownActionAcknowledge,
	})
	if err != nil {
		t.Fatalf("冲突不应是 error(它是正常业务态): %v", err)
	}
	if !got.GetConflict() || got.GetOk() {
		t.Fatalf("revision 不匹配应回冲突结果(ok=false): %+v", got)
	}
	// 人话冲突说明,让客户端知道该做什么(刷新后重试)。
	if !strings.Contains(got.GetDetail(), "刷新") {
		t.Fatalf("冲突说明应引导客户端刷新: %q", got.GetDetail())
	}
	// 带回的是当前最新摘要 —— 客户端拿它刷新即恢复,不必再发一次详情请求。
	if got.GetSummary().GetRevision() != 1 || got.GetSummary().GetProcessingStatus() != runtime.StatusIgnored {
		t.Fatalf("冲突结果应带回当前最新摘要: %+v", got.GetSummary())
	}
	// new_revision 带回当前值,客户端拿它做下一次 expected_revision。
	if got.GetNewRevision() != 1 {
		t.Fatalf("冲突结果应带当前 revision: %+v", got)
	}
	// 事件状态分毫不动:冲突的请求没有产生任何写入。
	cur, ok := log.Get(e.ID)
	if !ok || cur.Revision != 1 || cur.ProcessingStatus != runtime.StatusIgnored {
		t.Fatalf("冲突后事件不应被改动: ok=%v %+v", ok, cur)
	}
	// 历史留了 CONFLICT 一笔。
	if recs := srv.unknownActionHistoryLedger().snapshot(); len(recs) != 1 ||
		recs[0].Result != unknownActionResultConflict || recs[0].EventID != e.ID {
		t.Fatalf("冲突应落历史: %+v", recs)
	}
}

func TestUnknownActionInspectReturnsRedactedRaw(t *testing.T) {
	srv := newUnknownActionTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()
	e := log.Record(runtime.EventEnvelope{
		SourceAgent:   "codex",
		SourceMethod:  "brandNew",
		RawPayload:    `{"apikey":"[redacted]","note":"hi"}`,
		DecodedFields: `{"apikey":"string"}`,
		CorrelationID: "resp-77",
	})

	got, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ExpectedRevision: int64(e.Revision), ActionId: unknownActionInspect,
	})
	if err != nil {
		t.Fatalf("inspect 失败: %v", err)
	}
	if got.GetConflict() || !got.GetOk() {
		t.Fatalf("版本一致时 inspect 应 ok 且不冲突: %+v", got)
	}
	if got.GetTechnical() == nil {
		t.Fatalf("inspect 必须带回技术详情: %+v", got)
	}
	// 与 GetUnknownEvent 同一能力:原文、字段透析、支持包提示一个不少。
	if got.GetTechnical().GetRawJson() != `{"apikey":"[redacted]","note":"hi"}` {
		t.Fatalf("应回脱敏后的原始载荷: %q", got.GetTechnical().GetRawJson())
	}
	if len(got.GetTechnical().GetDigest()) == 0 {
		t.Fatalf("应带字段透析表: %+v", got.GetTechnical())
	}
	if got.GetTechnical().GetSupportBundleHint() == "" {
		t.Fatalf("技术详情应带支持包提示")
	}
	// inspect 是纯读取:状态与 revision 原样,new_revision 为当前值。
	cur, _ := log.Get(e.ID)
	if cur.ProcessingStatus != runtime.StatusNew || cur.Revision != 0 {
		t.Fatalf("inspect 不得改状态或 revision: %+v", cur)
	}
	if got.GetNewRevision() != int64(cur.Revision) {
		t.Fatalf("inspect 结果应带当前 revision: %+v", got)
	}
}

func TestUnknownActionThreeStatusActions(t *testing.T) {
	srv := newUnknownActionTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()

	// acknowledge → INSPECTED,revision +1,摘要同步刷新。
	e1 := log.Record(runtime.EventEnvelope{SourceAgent: "pi", SourceMethod: "brandNewPi"})
	got, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e1.ID, ExpectedRevision: int64(e1.Revision), ActionId: unknownActionAcknowledge,
	})
	if err != nil {
		t.Fatalf("acknowledge 失败: %v", err)
	}
	if !got.GetOk() || got.GetConflict() {
		t.Fatalf("acknowledge 应 ok: %+v", got)
	}
	if got.GetSummary().GetProcessingStatus() != runtime.StatusInspected || got.GetSummary().GetRevision() != 1 {
		t.Fatalf("acknowledge 未生效: %+v", got.GetSummary())
	}
	if got.GetNewRevision() != 1 {
		t.Fatalf("动作后 new_revision 应为 1: %+v", got)
	}
	if !strings.Contains(got.GetDetail(), "已查看") {
		t.Fatalf("acknowledge 应有人话结果: %q", got.GetDetail())
	}

	// retry_later → RETRY_LATER。
	e2 := log.Record(runtime.EventEnvelope{SourceAgent: "dsh", SourceMethod: "weird/request", NormalizedKind: runtime.KindUnknownRequest})
	got, err = srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e2.ID, ExpectedRevision: int64(e2.Revision), ActionId: unknownActionRetryLater,
	})
	if err != nil {
		t.Fatalf("retry_later 失败: %v", err)
	}
	if !got.GetOk() || got.GetSummary().GetProcessingStatus() != runtime.StatusRetryLater || got.GetSummary().GetRevision() != 1 {
		t.Fatalf("retry_later 未生效: %+v", got)
	}

	// open_native → ESCALATED(已转交)+ 电脑端指引(带 correlation id)。
	e3 := log.Record(runtime.EventEnvelope{
		SourceAgent:    "dsh",
		SourceMethod:   "weird/request",
		NormalizedKind: runtime.KindUnknownRequest,
		CorrelationID:  "resp-9",
	})
	got, err = srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e3.ID, ExpectedRevision: int64(e3.Revision), ActionId: unknownActionOpenNative,
	})
	if err != nil {
		t.Fatalf("open_native 失败: %v", err)
	}
	if !got.GetOk() || got.GetSummary().GetProcessingStatus() != runtime.StatusEscalated || got.GetSummary().GetRevision() != 1 {
		t.Fatalf("open_native 未生效: %+v", got)
	}
	for _, want := range []string{"电脑端", "不代为执行", "correlation id", "resp-9"} {
		if !strings.Contains(got.GetDetail(), want) {
			t.Fatalf("open_native 指引缺 %q: %q", want, got.GetDetail())
		}
	}
}

func TestUnknownActionIdempotentRepeat(t *testing.T) {
	srv := newUnknownActionTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()
	e := log.Record(runtime.EventEnvelope{SourceAgent: "pi", SourceMethod: "brandNewPi"})

	if _, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ExpectedRevision: int64(e.Revision), ActionId: unknownActionAcknowledge,
	}); err != nil {
		t.Fatalf("首次 acknowledge 失败: %v", err)
	}

	// 相同动作 + 正确 revision(1)重复提交:幂等 —— ok=true 返回当前状态而非报错。
	got, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ExpectedRevision: 1, ActionId: unknownActionAcknowledge,
	})
	if err != nil {
		t.Fatalf("重复 acknowledge(正确 revision)不应报错: %v", err)
	}
	if got.GetConflict() || !got.GetOk() {
		t.Fatalf("重复 acknowledge 应幂等成功: %+v", got)
	}
	if got.GetSummary().GetRevision() != 1 || got.GetSummary().GetProcessingStatus() != runtime.StatusInspected {
		t.Fatalf("重复提交应返回当前状态: %+v", got.GetSummary())
	}
	if got.GetNewRevision() != 1 {
		t.Fatalf("幂等结果应带当前 revision: %+v", got)
	}
	if !strings.Contains(got.GetDetail(), "已经") {
		t.Fatalf("幂等结果应说明已是该状态: %q", got.GetDetail())
	}

	// stale 重放(旧 revision 0):CAS 先于幂等 —— 回冲突,不给"幂等"当通行证
	// (2026-10-04 门禁裁定);冲突结果带当前 revision,客户端刷新后可重试。
	got, err = srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ExpectedRevision: 0, ActionId: unknownActionAcknowledge,
	})
	if err != nil || !got.GetConflict() || got.GetOk() {
		t.Fatalf("stale 重放应回冲突: err=%v got=%+v", err, got)
	}
	if got.GetNewRevision() != 1 {
		t.Fatalf("stale 重放的冲突应带当前 revision: %+v", got)
	}

	// 幂等与重放都不重复 bump revision:台账里仍是一次确认。
	if cur, _ := log.Get(e.ID); cur.Revision != 1 {
		t.Fatalf("重复提交不得再 bump revision: %+v", cur)
	}

	// open_native 的幂等重复照样带回电脑端指引 —— 用户重复点按钮仍能看到去哪处理。
	e2 := log.Record(runtime.EventEnvelope{SourceAgent: "zcode", SourceMethod: "weird", NormalizedKind: runtime.KindUnknownRequest})
	if _, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e2.ID, ExpectedRevision: int64(e2.Revision), ActionId: unknownActionOpenNative,
	}); err != nil {
		t.Fatalf("首次 open_native 失败: %v", err)
	}
	got, err = srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e2.ID, ExpectedRevision: 1, ActionId: unknownActionOpenNative, // 正确 revision
	})
	if err != nil || got.GetConflict() || !got.GetOk() {
		t.Fatalf("重复 open_native 应幂等返回: err=%v got=%+v", err, got)
	}
	if !strings.Contains(got.GetDetail(), "电脑端") {
		t.Fatalf("重复 open_native 应仍带电脑端指引: %q", got.GetDetail())
	}
}

// TestUnknownActionExpectedRevisionSemantics expected_revision 的类型与口径钉子:
// wire 是 int64,台账是 uint64(显式转换收敛在方法出入口,64 位平台无损)。
//   - 负数 = 病态输入,拒绝而不是按 0(否则"发错了"被伪装成"还在初始版本");
//   - 0 的双重身份:纯读动作(inspect)免 CAS —— 详情页不必先拉 revision;
//     状态型动作的 0 照常参与比较(0 是新事件的合法版本号,拿 0 去改已确认过的
//     事件回冲突 = 失败安全方向)。
func TestUnknownActionExpectedRevisionSemantics(t *testing.T) {
	srv := newUnknownActionTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()
	e := log.Record(runtime.EventEnvelope{SourceAgent: "pi", SourceMethod: "brandNewPi"})
	// 预置 revision 1:事件已被另一端 acknowledge 过一次。
	if _, ok, err := log.Acknowledge(e.ID, runtime.StatusInspected, ""); err != nil || !ok {
		t.Fatalf("预置已查看状态失败: ok=%v err=%v", ok, err)
	}

	// 负数:拒绝。
	if _, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ExpectedRevision: -1, ActionId: unknownActionAcknowledge,
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected_revision<0 应为 InvalidArgument,得到 %v", err)
	}

	// inspect 带 0:纯读免 CAS,不冲突(proto 口径:0 = 不做 CAS,仅纯读允许)。
	got, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ExpectedRevision: 0, ActionId: unknownActionInspect,
	})
	if err != nil || got.GetConflict() || !got.GetOk() {
		t.Fatalf("纯读动作 expected=0 不应冲突: err=%v got=%+v", err, got)
	}
	if got.GetNewRevision() != 1 {
		t.Fatalf("inspect 结果应带当前 revision: %+v", got)
	}

	// 状态型动作带 0:照常比较 —— 事件已在 revision 1,拿 0 来改应回冲突。
	got, err = srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ExpectedRevision: 0, ActionId: unknownActionAcknowledge,
	})
	if err != nil || !got.GetConflict() || got.GetOk() {
		t.Fatalf("状态型动作 expected=0 对已确认事件应冲突: err=%v got=%+v", err, got)
	}
	if got.GetNewRevision() != 1 {
		t.Fatalf("冲突结果应带回当前 revision: %+v", got)
	}

	// inspect 明确带对版本(1):照常通过。
	got, err = srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ExpectedRevision: 1, ActionId: unknownActionInspect,
	})
	if err != nil || got.GetConflict() {
		t.Fatalf("inspect 带正确版本不应冲突: err=%v got=%+v", err, got)
	}
}

func TestUnknownActionRejectsUnknownAndUnsafe(t *testing.T) {
	srv := newUnknownActionTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()
	e := log.Record(runtime.EventEnvelope{SourceAgent: "codex", SourceMethod: "brandNew"})

	// 安全红线在执行层的封口:任何"允许/执行"类 ID 都进不来,伪造也不行。
	for _, action := range []string{"approve", "execute", "allow", "allow_once", "confirm", "whatever"} {
		_, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
			EventId: e.ID, ExpectedRevision: int64(e.Revision), ActionId: action,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("动作 %q 应为 InvalidArgument,得到 %v", action, err)
		}
		if !strings.Contains(err.Error(), "inspect") {
			t.Fatalf("拒绝信息应列出可执行动作: %v", err)
		}
	}
	// copy 是展示层动作但属于手机本地行为,给专门的人话说明。
	_, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ActionId: unknownActionCopyID,
	})
	if status.Code(err) != codes.InvalidArgument || !strings.Contains(err.Error(), "本地") {
		t.Fatalf("copy 应有专门的本地说明: %v", err)
	}
	// 缺 action_id 同样拒绝,且列出口径一致的清单。
	_, err = srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{EventId: e.ID})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("缺 action_id 应为 InvalidArgument,得到 %v", err)
	}
	// 上述全部拒绝未产生任何状态变化。
	if cur, _ := log.Get(e.ID); cur.Revision != 0 || cur.ProcessingStatus != runtime.StatusNew {
		t.Fatalf("被拒绝的请求不得改事件: %+v", cur)
	}
	// 非白名单动作留了 REJECTED 历史(事件 id 如实绑定,便于审计伪造尝试)。
	recs := srv.unknownActionHistoryLedger().snapshot()
	if len(recs) != 6 {
		t.Fatalf("应有 6 条 REJECTED 历史(approve/execute/allow/allow_once/confirm/whatever),得到 %d", len(recs))
	}
	for _, r := range recs {
		if r.Result != unknownActionResultRejected || r.EventID != e.ID || r.TimestampMs == 0 {
			t.Fatalf("拒绝历史不完整: %+v", r)
		}
	}
}

func TestUnknownActionInputValidationAndMissingEvent(t *testing.T) {
	srv := newUnknownActionTestServer(t)
	ctx := context.Background()
	// 缺 event_id。
	if _, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{ActionId: unknownActionInspect}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("缺 event_id 应为 InvalidArgument,得到 %v", err)
	}
	// 不存在的 event。
	if _, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: "ue-nope", ActionId: unknownActionInspect,
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("不存在的 event 应为 NotFound,得到 %v", err)
	}
	// 没有运行时后端(极简构造):台账未启用按 NotFound 说清,不装"没有记录"。
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	bare := New(Config{Store: st})
	t.Cleanup(func() { _ = bare.events.Close() })
	if _, err := bare.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: "ue-1", ActionId: unknownActionInspect,
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("无后端应为 NotFound,得到 %v", err)
	}
}

// TestUnknownActionHistoryPersistsToFile 处置历史落盘:内存快照与盘上文件一致。
func TestUnknownActionHistoryPersistsToFile(t *testing.T) {
	srv := newUnknownActionTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()
	e := log.Record(runtime.EventEnvelope{SourceAgent: "pi", SourceMethod: "brandNewPi"})

	if _, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ExpectedRevision: int64(e.Revision), ActionId: unknownActionAcknowledge,
	}); err != nil {
		t.Fatalf("acknowledge 失败: %v", err)
	}
	if _, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ExpectedRevision: 0, ActionId: "approve",
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("approve 应被拒绝: %v", err)
	}

	recs := srv.unknownActionHistoryLedger().snapshot()
	if len(recs) != 2 {
		t.Fatalf("应有两笔历史(OK + REJECTED),得到 %d", len(recs))
	}
	// 最新在前:后发生的 REJECTED 排前面;序号单调。
	if recs[0].Result != unknownActionResultRejected || recs[1].Result != unknownActionResultOK {
		t.Fatalf("历史应最新在前: %+v", recs)
	}
	if recs[0].Sequence <= recs[1].Sequence {
		t.Fatalf("序号应单调: %+v", recs)
	}
	for _, r := range recs {
		if r.EventID != e.ID || r.ActionID == "" || r.TimestampMs == 0 || r.Result == "" {
			t.Fatalf("历史条目缺关键字段(event_id/动作/时间/结果): %+v", r)
		}
	}

	// 盘上文件与内存一致(seq 头 + 条目)。
	raw, err := os.ReadFile(filepath.Join(srv.cfg.DataDir, unknownActionHistoryFileName))
	if err != nil {
		t.Fatalf("历史文件应已落盘: %v", err)
	}
	var f unknownActionHistoryFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("历史文件应是合法 JSON: %v", err)
	}
	if f.Seq != 2 || len(f.Entries) != 2 {
		t.Fatalf("盘上历史应与内存一致: seq=%d entries=%d", f.Seq, len(f.Entries))
	}
}

// TestUnknownActionConcurrentSameEvent 并发同一事件(-race 写法):两个动作同时
// 打到同一事件的同一版本上。进程级执行锁保证一先一后进临界区,结果集合确定:
// 不同动作 → 恰一生效、另一者冲突;相同动作 → 一者生效、另一者幂等。
func TestUnknownActionConcurrentSameEvent(t *testing.T) {
	srv := newUnknownActionTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()

	for i := 0; i < 20; i++ {
		e := log.Record(runtime.EventEnvelope{SourceAgent: "pi", SourceMethod: "brandNewPi"})

		// 变体一:两个不同动作抢同一版本 —— 恰好一个生效。
		var wg sync.WaitGroup
		res := make([]*pb.UnknownEventActionResult, 2)
		errs := make([]error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			res[0], errs[0] = srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
				EventId: e.ID, ExpectedRevision: int64(e.Revision), ActionId: unknownActionAcknowledge,
			})
		}()
		go func() {
			defer wg.Done()
			res[1], errs[1] = srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
				EventId: e.ID, ExpectedRevision: int64(e.Revision), ActionId: unknownActionRetryLater,
			})
		}()
		wg.Wait()
		for j, err := range errs {
			if err != nil {
				t.Fatalf("第 %d 轮:并发动作 %d 不应报错: %v", i, j, err)
			}
		}
		applied := 0
		for _, r := range res {
			if r.GetOk() && !r.GetConflict() {
				applied++
			}
		}
		if applied != 1 {
			t.Fatalf("第 %d 轮:不同动作应恰一生效,得到 %d(%+v / %+v)", i, applied, res[0], res[1])
		}
		cur, _ := log.Get(e.ID)
		if cur.Revision != 1 ||
			(cur.ProcessingStatus != runtime.StatusInspected && cur.ProcessingStatus != runtime.StatusRetryLater) {
			t.Fatalf("第 %d 轮:终态应二选一且 revision=1: %+v", i, cur)
		}

		// 变体二:同一动作并发抢同一版本 —— 后到者版本已过期,CAS 先于幂等
		// (门禁裁定),回冲突而非"幂等通行证";结果同样恰一生效,revision 只加一次。
		e2 := log.Record(runtime.EventEnvelope{SourceAgent: "pi", SourceMethod: "brandNewPi"})
		wg.Add(2)
		go func() {
			defer wg.Done()
			res[0], errs[0] = srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
				EventId: e2.ID, ExpectedRevision: int64(e2.Revision), ActionId: unknownActionAcknowledge,
			})
		}()
		go func() {
			defer wg.Done()
			res[1], errs[1] = srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
				EventId: e2.ID, ExpectedRevision: int64(e2.Revision), ActionId: unknownActionAcknowledge,
			})
		}()
		wg.Wait()
		for j, err := range errs {
			if err != nil {
				t.Fatalf("第 %d 轮:同动作并发 %d 不应报错: %v", i, j, err)
			}
		}
		applied = 0
		for _, r := range res {
			if r.GetOk() && !r.GetConflict() {
				applied++
			}
		}
		if applied != 1 {
			t.Fatalf("第 %d 轮:同动作并发应恰一生效,得到 %d(%+v / %+v)", i, applied, res[0], res[1])
		}
		if cur2, _ := log.Get(e2.ID); cur2.Revision != 1 || cur2.ProcessingStatus != runtime.StatusInspected {
			t.Fatalf("第 %d 轮:同动作并发终态应稳定且 revision=1: %+v", i, cur2)
		}
	}
}
