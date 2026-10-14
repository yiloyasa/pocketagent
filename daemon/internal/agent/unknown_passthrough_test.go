// unknown_passthrough_test.go —— 辅线 B「未知反向请求透传确认」的行为矩阵钉子
// (README §7 的 B 开关:全局默认关,ON 时未知反向请求先弹手机通用确认卡)。
//
// 【钉什么】三态矩阵,dsh 与 zcode 两家(本轮接线范围;codex/pi 留报告):
//  1. OFF(默认)→ 与历史行为逐字节一致:回 -32601 + 台账 UNKNOWN_REQUEST;
//  2. ON + 用户允许 → 回空对象 {} + 台账 KNOWN_PASSTHROUGH(用户看过脱敏原文后
//     授权这次试探 —— 尽力应答,不是伪造业务结果);
//  3. ON + 拒绝/超时 → 回落到 OFF 同款:-32601 + 台账。
//
// 【红线对照】允许→{} 是"用户知情授权的试探性应答",决策权在用户;超时/拒绝绝不
// 伪造"同意"(红线 1),也绝不伪造业务 decision(红线 2)—— {} 是协议层的"空结果",
// 内容对 agent 透明,不冒充任何业务语义。
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// stubUnknownPassthrough 替换 B 开关查询点,返回恢复函数(测试互不串味)。
func stubUnknownPassthrough(on bool) func() {
	old := unknownPassthroughQuery
	unknownPassthroughQuery = func() bool { return on }
	return func() { unknownPassthroughQuery = old }
}

// ledgerEntries 按 agent+method 过滤台账条目(循环体见 ledger_query_testkit)。
func ledgerEntries(t *testing.T, agentID, method string) []runtime.EventEnvelope {
	t.Helper()
	return ledgerByMethod(agentID, method)
}

// ---- dsh 三态 ------------------------------------------------------------------

func TestDshUnknownRequestPassthroughAllow(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	const method = "futureDept/passthroughProbe" // 探针专用:真实上游不会发
	fake, done, free := runWithPendingApproval(t, func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		// 确认卡必须是通用形状:name=unknown_request,args 带脱敏后的 method+payload,
		// 绝不把"未知请求"伪装成某个已知的动作类型。
		if call.GetName() != "unknown_request" {
			t.Errorf("确认卡动作名应为 unknown_request,得到 %q", call.GetName())
		}
		if !strings.Contains(call.GetJsonArgs(), method) {
			t.Errorf("确认卡必须带 method 原文: %s", call.GetJsonArgs())
		}
		return true, "", nil
	})
	fake.reqFn(31, method, json.RawMessage(`{"token":"secret-req","x":1}`))
	free()
	if err := <-done; err != nil {
		t.Fatalf("Run 应照常完成: %v", err)
	}

	// 允许 → 成功应答,结果 = 空对象 {}(协议层"空结果",不是任何业务 decision)。
	snap := fake.snapshot()
	if len(snap.responds) != 1 || snap.responds[0].isError {
		t.Fatalf("允许路应回成功应答: %+v", snap.responds)
	}
	b, _ := json.Marshal(snap.responds[0].result)
	if string(b) != "{}" {
		t.Fatalf("允许路应回空对象 {},得到 %s", b)
	}

	// 台账:KNOWN_PASSTHROUGH 备查,载荷脱敏。
	var hit bool
	for _, e := range ledgerEntries(t, "dsh", method) {
		hit = true
		if e.NormalizedKind != runtime.KindKnownPassthrough {
			t.Fatalf("允许路应记 KNOWN_PASSTHROUGH,得到 %q", e.NormalizedKind)
		}
		if strings.Contains(e.RawPayload, "secret-req") {
			t.Fatalf("台账载荷必须脱敏: %s", e.RawPayload)
		}
		if !strings.Contains(e.Note, "空对象") {
			t.Fatalf("台账应写明回的是空对象 {}: %q", e.Note)
		}
	}
	if !hit {
		t.Fatal("允许路必须落 KNOWN_PASSTHROUGH 台账备查")
	}
}

func TestDshUnknownRequestPassthroughDeny(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	const method = "futureDept/passthroughDeny"
	fake, done, free := runWithPendingApproval(t, func(context.Context, *pb.ToolCallBlock) (bool, string, error) {
		return false, "", nil // 用户在手机上点了拒绝
	})
	fake.reqFn(32, method, json.RawMessage(`{"x":1}`))
	free()
	if err := <-done; err != nil {
		t.Fatalf("Run 应照常完成: %v", err)
	}
	snap := fake.snapshot()
	if len(snap.responds) != 1 || !snap.responds[0].isError || snap.responds[0].errCode != -32601 {
		t.Fatalf("拒绝路应回 -32601: %+v", snap.responds)
	}
	var hit bool
	for _, e := range ledgerEntries(t, "dsh", method) {
		hit = true
		if e.NormalizedKind != runtime.KindUnknownRequest {
			t.Fatalf("拒绝路应记 UNKNOWN_REQUEST,得到 %q", e.NormalizedKind)
		}
	}
	if !hit {
		t.Fatal("拒绝路必须落 UNKNOWN_REQUEST 台账")
	}
}

func TestDshUnknownRequestPassthroughTimeout(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	old := dshApprovalTimeout
	dshApprovalTimeout = 80 * time.Millisecond
	t.Cleanup(func() { dshApprovalTimeout = old })

	const method = "futureDept/passthroughTimeout"
	fake, done, free := runWithPendingApproval(t, func(ctx context.Context, _ *pb.ToolCallBlock) (bool, string, error) {
		<-ctx.Done() // 用户不点,直到超时
		return false, "", ctx.Err()
	})
	fake.reqFn(33, method, json.RawMessage(`{"x":1}`))
	free()
	if err := <-done; err != nil {
		t.Fatalf("Run 应照常完成: %v", err)
	}
	snap := fake.snapshot()
	if len(snap.responds) != 1 || !snap.responds[0].isError || snap.responds[0].errCode != -32601 {
		t.Fatalf("超时路应回落 -32601(绝不把没等到当成同意): %+v", snap.responds)
	}
	if n := len(ledgerEntries(t, "dsh", method)); n == 0 {
		t.Fatal("超时路必须落 UNKNOWN_REQUEST 台账")
	}
}

// TestDshUnknownRequestPassthroughOffIsLegacy OFF(默认)必须与历史行为逐字节一致:
// -32601 + UNKNOWN_REQUEST 台账,绝不弹卡(approve 不应被调用)。
func TestDshUnknownRequestPassthroughOffIsLegacy(t *testing.T) {
	defer stubUnknownPassthrough(false)()
	const method = "futureDept/passthroughOff"
	asked := false
	fake, done, free := runWithPendingApproval(t, func(context.Context, *pb.ToolCallBlock) (bool, string, error) {
		asked = true
		return true, "", nil
	})
	fake.reqFn(34, method, json.RawMessage(`{"x":1}`))
	free()
	if err := <-done; err != nil {
		t.Fatalf("Run 应照常完成: %v", err)
	}
	if asked {
		t.Fatal("OFF 态绝不得询问手机(开关没开就没有确认卡)")
	}
	snap := fake.snapshot()
	if len(snap.responds) != 1 || !snap.responds[0].isError || snap.responds[0].errCode != -32601 {
		t.Fatalf("OFF 态应回 -32601: %+v", snap.responds)
	}
	if n := len(ledgerEntries(t, "dsh", method)); n == 0 {
		t.Fatal("OFF 态必须落 UNKNOWN_REQUEST 台账")
	}
}

// ---- zcode 三态(注入 fake responder 直调,与 zcode_approval_test.go 同款)--------

func TestZcodeUnknownRequestPassthroughAllow(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	p := NewZcodeProvider()
	rsp := &fakeZcodeResponder{}
	var waiting atomic.Bool
	approve := func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		if call.GetName() != "unknown_request" {
			t.Errorf("确认卡动作名应为 unknown_request,得到 %q", call.GetName())
		}
		if !strings.Contains(call.GetJsonArgs(), "futureDept/zc") {
			t.Errorf("确认卡必须带 method 原文: %s", call.GetJsonArgs())
		}
		return true, "", nil
	}
	ok := p.handleZcodeUnknownRequest(json.RawMessage(`"srv-b1"`), "futureDept/zc", "th-b1",
		json.RawMessage(`{"token":"secret-zc"}`), context.Background(), rsp, approve, &waiting, time.Second)
	if !ok {
		t.Fatal("允许路应消费该请求(交回客户端层会变 -32601)")
	}
	if got := rsp.lastResult(t); len(got) != 0 {
		t.Fatalf("允许路应回空对象 {},得到 %v", got)
	}
	var hit bool
	for _, e := range ledgerEntries(t, "zcode", "futureDept/zc") {
		hit = true
		if e.NormalizedKind != runtime.KindKnownPassthrough {
			t.Fatalf("允许路应记 KNOWN_PASSTHROUGH,得到 %q", e.NormalizedKind)
		}
		if strings.Contains(e.RawPayload, "secret-zc") {
			t.Fatalf("台账载荷必须脱敏: %s", e.RawPayload)
		}
	}
	if !hit {
		t.Fatal("允许路必须落 KNOWN_PASSTHROUGH 台账备查")
	}
	if waiting.Load() {
		t.Fatal("waiting 旗标应复位(看门狗要恢复计时)")
	}
}

func TestZcodeUnknownRequestPassthroughDenyFallsToUnknown(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	p := NewZcodeProvider()
	rsp := &fakeZcodeResponder{}
	var waiting atomic.Bool
	approve := func(context.Context, *pb.ToolCallBlock) (bool, string, error) {
		return false, "", nil
	}
	ok := p.handleZcodeUnknownRequest(json.RawMessage(`"srv-b2"`), "futureDept/zcDeny", "th-b2",
		json.RawMessage(`{"x":1}`), context.Background(), rsp, approve, &waiting, time.Second)
	if ok {
		t.Fatal("拒绝路不得消费请求:应交回调用方走既有 -32601+台账,与 OFF 同款")
	}
	if n := rsp.resultCount(); n != 0 {
		t.Fatalf("拒绝路不得自行回成功应答,记录到 %d 条", n)
	}
}

func TestZcodeUnknownRequestPassthroughTimeoutFallsToUnknown(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	p := NewZcodeProvider()
	rsp := &fakeZcodeResponder{}
	var waiting atomic.Bool
	approve := func(ctx context.Context, _ *pb.ToolCallBlock) (bool, string, error) {
		<-ctx.Done()
		return false, "", ctx.Err()
	}
	ok := p.handleZcodeUnknownRequest(json.RawMessage(`"srv-b3"`), "futureDept/zcT", "th-b3",
		json.RawMessage(`{"x":1}`), context.Background(), rsp, approve, &waiting, 50*time.Millisecond)
	if ok {
		t.Fatal("超时路不得消费请求:应交回调用方走既有 -32601+台账")
	}
	if waiting.Load() {
		t.Fatal("waiting 旗标应复位")
	}
}

// ---- 策略位解析(纯函数)--------------------------------------------------------

func TestUnknownRequestPassthroughPolicySemantics(t *testing.T) {
	// 缺席(默认)= 关:optional bool 的缺席语义必须落到"不透传",绝不是"已授权"。
	if unknownPassthroughOn(runtime.BudgetPolicy{}) {
		t.Fatal("策略缺席必须解析为关(默认关是安全姿态)")
	}
	on := true
	if !unknownPassthroughOn(runtime.BudgetPolicy{UnknownRequestPassthrough: &on}) {
		t.Fatal("显式 true 必须解析为开")
	}
	off := false
	if unknownPassthroughOn(runtime.BudgetPolicy{UnknownRequestPassthrough: &off}) {
		t.Fatal("显式 false 必须解析为关")
	}
}
