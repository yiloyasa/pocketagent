// pi_usage_stream_test.go —— 0.87.1 流式用量路径与新事件分档的单测:
//
//  1. usageFromMessageUpdate:探针实拍帧(per-message 口径,终帧才有值);
//  2. piUsageTracker:多消息求和、全零过程帧过滤、快照隔离;
//  3. usageUpdateFromStream:与 UsageUpdateFromStats 同口径(增量/累计/窗口);
//  4. usageFromAgentEnd:cost 是对象(探针实拍),曾按 float64 解析恒失败;
//  5. piEventNoticeKind:0.86/0.87 新事件归 KNOWN_PASSTHROUGH,老事件回落
//     piNoticeKind,真未知仍 UNKNOWN;
//  6. ClearQueue:无活子进程按空队列成功;下发/解析/失败映射经窄接口打桩。
//
// 证据:docs/verify/evidence/pi-087-usage-probe.log(pi 0.87.1 真实短轮次)。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/piapp"
	"github.com/pocketagent/daemon/internal/runtime"
)

// 探针实拍(2026-09-25,pi 0.87.1):流式增量帧 usage 全 0,终帧(text_end)
// 带上本条消息真实用量。cost 是对象(不是数字)。
const probeTextEndFrame = `{"type":"message_update","usage":{"input":17500,"output":3,"cacheRead":768,"cacheWrite":0,"reasoning":0,"totalTokens":18271,` +
	`"cost":{"input":0.0245,"output":0.0000132,"cacheRead":0.00019968,"cacheWrite":0,"total":0.02471288}},` +
	`"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":"ok"}}`

const probeTextDeltaFrame = `{"type":"message_update","usage":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"totalTokens":0,` +
	`"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"total":0}},` +
	`"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"ok"}}`

// 终帧解析:字段逐一对上(含 cost.total 与 reasoning)。
func TestUsageFromMessageUpdateFinalFrame(t *testing.T) {
	u := usageFromMessageUpdate(json.RawMessage(probeTextEndFrame))
	if u == nil {
		t.Fatal("终帧应解析出非零 usage")
	}
	if u.GetInputTokens() != 17500 || u.GetOutputTokens() != 3 || u.GetCacheReadTokens() != 768 || u.GetCacheWriteTokens() != 0 {
		t.Fatalf("token 数不符: %+v", u)
	}
	if u.GetReasoningTokens() != 0 {
		t.Fatalf("reasoning 应为 0: %+v", u)
	}
	if u.GetCostUsd() < 0.0247 || u.GetCostUsd() > 0.0248 {
		t.Fatalf("cost 应取 cost.total=0.02471288,得到 %v", u.GetCostUsd())
	}
}

// 过程帧(全 0)与无 usage 字段的帧:一律 nil(新旧版本分流的判据)。
func TestUsageFromMessageUpdateIgnoresZeroFrames(t *testing.T) {
	for name, raw := range map[string]string{
		"全零过程帧":      probeTextDeltaFrame,
		"无usage字段":   `{"type":"message_update","assistantMessageEvent":{"type":"toolcall_start","id":"c1","toolName":"bash"}}`,
		"脏报文":        `not-json`,
		"usage为null": `{"type":"message_update","usage":null,"assistantMessageEvent":{"type":"text_start"}}`,
	} {
		if u := usageFromMessageUpdate(json.RawMessage(raw)); u != nil {
			t.Fatalf("%s 应返回 nil,得到 %+v", name, u)
		}
	}
}

// 多条 assistant 消息(多步轮次):各帧各带一份 per-message usage,求和才是
// 本轮增量;全零过程帧不得重复入账。
func TestPiUsageTrackerSumsPerMessageUsage(t *testing.T) {
	tr := &piUsageTracker{}
	// 消息1:过程帧 x2(0) + 终帧(17500/3/768)
	if tr.observe(json.RawMessage(probeTextDeltaFrame)) {
		t.Fatal("全零帧不应触发推送")
	}
	if !tr.observe(json.RawMessage(probeTextEndFrame)) {
		t.Fatal("终帧应触发推送")
	}
	// 消息2:终帧(100/50)
	if !tr.observe(json.RawMessage(`{"type":"message_update","usage":{"input":100,"output":50,"cacheRead":0,"cacheWrite":10,"reasoning":7,"totalTokens":167,"cost":{"total":0.01}},"assistantMessageEvent":{"type":"text_end"}}`)) {
		t.Fatal("第二条消息终帧应触发推送")
	}
	s := tr.snapshot()
	if s == nil {
		t.Fatal("累计快照不应为 nil")
	}
	if s.GetInputTokens() != 17600 || s.GetOutputTokens() != 53 || s.GetCacheReadTokens() != 768 || s.GetCacheWriteTokens() != 10 || s.GetReasoningTokens() != 7 {
		t.Fatalf("求和口径不符: %+v", s)
	}
	if s.GetCostUsd() < 0.0347 || s.GetCostUsd() > 0.0348 {
		t.Fatalf("cost 应求和 0.02471288+0.01,得到 %v", s.GetCostUsd())
	}
	if !tr.observed() {
		t.Fatal("见过非零帧后 observed 应为 true")
	}
	// 快照隔离:再入账不得改动已发出的快照(UsageUpdate 帧内容稳定)。
	tr.observe(json.RawMessage(`{"type":"message_update","usage":{"input":1,"output":1,"totalTokens":2,"cost":{"total":0}},"assistantMessageEvent":{"type":"text_end"}}`))
	if s.GetInputTokens() != 17600 {
		t.Fatalf("快照被后续入账污染: %+v", s)
	}
}

// 空tracker:observed=false、snapshot=nil(旧版 pi 全程走这条,轮询兜底照旧)。
func TestPiUsageTrackerEmpty(t *testing.T) {
	tr := &piUsageTracker{}
	if tr.observed() {
		t.Fatal("空 tracker observed 应为 false")
	}
	if s := tr.snapshot(); s != nil {
		t.Fatalf("空 tracker snapshot 应为 nil,得到 %+v", s)
	}
}

// 中程帧口径与 UsageUpdateFromStats 对齐:usage=本轮累计,total=基线+本轮,
// 窗口取基线快照;基线为 nil 时 total=本轮、窗口如实留 0。
func TestUsageUpdateFromStream(t *testing.T) {
	acc := &pb.Usage{InputTokens: 17500, OutputTokens: 3, CacheReadTokens: 768, CostUsd: 0.02471288}
	baseline := parseSessionStats(json.RawMessage(`{"tokens":{"input":1000,"output":20,"cacheRead":100,"cacheWrite":5,"total":1125},"cost":{"total":1.5},"contextUsage":{"tokens":1125,"contextWindow":1000000,"percent":0.1125}}`))
	upd := usageUpdateFromStream(acc, baseline)
	if upd.GetUsage().GetInputTokens() != 17500 || upd.GetUsage().GetOutputTokens() != 3 {
		t.Fatalf("usage 应为本轮累计: %+v", upd.GetUsage())
	}
	if upd.GetTotalUsage().GetInputTokens() != 18500 || upd.GetTotalUsage().GetOutputTokens() != 23 {
		t.Fatalf("total 应为基线+本轮: %+v", upd.GetTotalUsage())
	}
	if upd.GetTotalUsage().GetCostUsd() < 1.5247 || upd.GetTotalUsage().GetCostUsd() > 1.5248 {
		t.Fatalf("total cost 应为基线+本轮: %v", upd.GetTotalUsage().GetCostUsd())
	}
	if upd.GetModelContextWindow() != 1000000 {
		t.Fatalf("窗口应取基线快照: %v", upd.GetModelContextWindow())
	}
	// 基线缺失(stats 取不到的会话):total=本轮,窗口 0(App 忽略空窗口)。
	upd2 := usageUpdateFromStream(acc, nil)
	if upd2.GetTotalUsage().GetInputTokens() != 17500 || upd2.GetModelContextWindow() != 0 {
		t.Fatalf("nil 基线口径不符: %+v", upd2)
	}
	// nil 累计(0.87.1 判据未命中就推帧的防御):等价空累计,不 panic。
	if upd3 := usageUpdateFromStream(nil, baseline); upd3 == nil || upd3.GetUsage() == nil {
		t.Fatal("nil 累计应回空累计帧")
	}
}

// usageFromAgentEnd 的 cost 按对象解析(实拍 cost 是 {"total":…} 对象;
// 曾按 float64 解析导致整条 unmarshal 失败、兜底从未生效)。
func TestUsageFromAgentEndCostObject(t *testing.T) {
	raw := json.RawMessage(`{"type":"agent_end","messages":[
		{"role":"user","content":[{"type":"text","text":"hi"}]},
		{"role":"assistant","usage":{"input":14402,"output":29,"cacheRead":3776,"cacheWrite":0,"cost":{"input":0.02,"output":0.0001,"total":0.0201}}}
	]}`)
	u := usageFromAgentEnd(raw)
	if u == nil {
		t.Fatal("cost 为对象时应解析成功(修复前恒 nil)")
	}
	if u.GetInputTokens() != 14402 || u.GetOutputTokens() != 29 || u.GetCacheReadTokens() != 3776 {
		t.Fatalf("token 数不符: %+v", u)
	}
	if u.GetCostUsd() < 0.02 || u.GetCostUsd() > 0.021 {
		t.Fatalf("cost 应取 cost.total: %v", u.GetCostUsd())
	}
}

// 0.86/0.87 新事件(9 个类型 = 7 个逻辑条目,summarization_retry 一族 3 个)
// 分档 KNOWN_PASSTHROUGH;老事件仍由 piNoticeKind 分档;真未知仍 UNKNOWN。
func TestPiEventNoticeKind(t *testing.T) {
	newEvents := []string{
		"turn_start", "turn_end", "message_start", "message_end",
		"entry_appended", "summarization_retry_scheduled",
		"summarization_retry_attempt_start", "summarization_retry_finished",
		"extension_error",
	}
	for _, et := range newEvents {
		if got := piEventNoticeKind(et); got != runtime.KindKnownPassthrough {
			t.Errorf("piEventNoticeKind(%q) = %q, want PASSTHROUGH", et, got)
		}
	}
	// 老事件回落 piNoticeKind:已知透传 / 真未知两个方向都要在。
	if got := piEventNoticeKind("agent_start"); got != runtime.KindKnownPassthrough {
		t.Errorf("老事件 agent_start 应回落为 PASSTHROUGH,得到 %q", got)
	}
	if got := piEventNoticeKind("pi_totally_new_thing"); got != runtime.KindUnknownNotification {
		t.Errorf("真未知应仍为 UNKNOWN,得到 %q", got)
	}
}

// stubQueueClearer queueClearerRPC 打桩:记录调用;err 非空时按错误应答。
type stubQueueClearer struct {
	calls int
	err   error
	q     *piapp.ClearedQueue
}

func (s *stubQueueClearer) ClearQueue(ctx context.Context) (*piapp.ClearedQueue, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.q, nil
}

// ClearQueue 无活子进程:按空队列成功返回,不创建任何子进程。
func TestPiClearQueueNoLiveClient(t *testing.T) {
	p := NewPiProvider()
	steered, followUp, err := p.ClearQueue(context.Background(), "th-none")
	if err != nil || steered != 0 || followUp != 0 {
		t.Fatalf("无活子进程应 (0,0,nil),得到 (%d,%d,%v)", steered, followUp, err)
	}
	if len(p.clients) != 0 {
		t.Fatalf("不应为撤回排队消息冷启子进程,clients=%d", len(p.clients))
	}
}

// clearQueueOn:下发走 piapp 封装、计数取数组长度;失败包 PI_QUEUE_CLEAR_FAILED。
func TestPiClearQueueOnStub(t *testing.T) {
	stub := &stubQueueClearer{q: &piapp.ClearedQueue{Steering: []string{"a"}, FollowUp: []string{"b", "c"}}}
	steered, followUp, err := clearQueueOn(context.Background(), stub)
	if err != nil {
		t.Fatalf("正常路径不应报错: %v", err)
	}
	if stub.calls != 1 {
		t.Fatalf("应恰好下发一次,实际 %d 次", stub.calls)
	}
	if steered != 1 || followUp != 2 {
		t.Fatalf("计数应为 (插队1, 排队2),得到 (%d,%d)", steered, followUp)
	}

	stub2 := &stubQueueClearer{err: errors.New("pipe closed")}
	_, _, err = clearQueueOn(context.Background(), stub2)
	if err == nil {
		t.Fatal("下发失败应返回错误")
	}
	carrier, ok := err.(ErrorInfoCarrier)
	if !ok {
		t.Fatalf("错误应实现 ErrorInfoCarrier,实际 %T", err)
	}
	if info := carrier.ErrorInfo(); info.GetCode() != "PI_QUEUE_CLEAR_FAILED" || !strings.Contains(info.GetMessage(), "撤销排队输入失败") {
		t.Fatalf("错误码/消息不符: %+v", info)
	}
}
