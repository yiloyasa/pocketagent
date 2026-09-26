// codex_unknown_passthrough_test.go —— 辅线 B「未知反向请求透传确认」的 codex 三态钉子。
//
// 【钉什么】codex 的未知反向请求(reqUnknown 分支)在 B 开关下的三态:
//  1. OFF(默认)→ 不问手机、不产生任何应答,交回 reqUnknown 既有分支:
//     -32601 + 台账 UNKNOWN_REQUEST(与历史逐字节一致);
//  2. ON + 用户允许 → 复用 requestApproval 的 approve 回路弹通用确认卡,允许后
//     回空对象 {}(codex-schema 的 JSONRPCResponse 对 result 不设约束,未知方法
//     无业务结果形状,{} 是最小诚实应答)+ 台账 KNOWN_PASSTHROUGH;
//  3. ON + 拒绝/超时 → 同样交回既有 -32601 + 台账分支,绝不把"没等到"当成同意。
//
// 【注入方式】与 codex_approval_test.go 同款:responder 走 codexApprovalResponder
// 最小接口注 fake,approve 直接注入,不依赖真 codex 子进程。requestApproval 应答
// 通道的其余形状(decision 枚举/竞速治理)由既有测试钉住,此处不重复。
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

const codexPassthroughProbeMethod = "futureDept/codexProbe" // 探针专用:真实上游不会发

// TestCodexUnknownRequestPassthroughAllow ON + 允许:确认卡走 requestApproval 同一条
// approve 通道(name=unknown_request、method/payload 原文在卡上),允许后回空对象
// {} 并落 KNOWN_PASSTHROUGH 台账(带会话归属、脱敏载荷、应答语义备注)。
func TestCodexUnknownRequestPassthroughAllow(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	p := NewCodexProvider()
	rsp := &fakeCodexResponder{}
	var waiting atomic.Bool
	approve := func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		// 确认卡必须是通用形状:绝不把"未知请求"伪装成某个已知的审批动作类型。
		if call.GetName() != "unknown_request" {
			t.Errorf("确认卡动作名应为 unknown_request,得到 %q", call.GetName())
		}
		if !strings.Contains(call.GetJsonArgs(), codexPassthroughProbeMethod) {
			t.Errorf("确认卡必须带 method 原文: %s", call.GetJsonArgs())
		}
		if !strings.Contains(call.GetJsonArgs(), "未识别的操作") {
			t.Errorf("确认卡必须写明这是未识别的操作: %s", call.GetJsonArgs())
		}
		return true, "", nil
	}
	ok := p.consumeCodexUnknownRequest(rsp, 41, codexPassthroughProbeMethod, "th-cx1",
		json.RawMessage(`{"token":"secret-cx","x":1}`), context.Background(),
		func(*pb.TurnServerMessage) {}, approve, &waiting)
	if !ok {
		t.Fatal("允许路应终结该请求(交回 reqUnknown 分支会再回 -32601,变成重复应答)")
	}
	if n := rsp.resultCount(); n != 1 {
		t.Fatalf("允许路应回恰好一条成功应答,得到 %d 条", n)
	}
	b, _ := json.Marshal(rsp.results[0])
	if string(b) != "{}" {
		t.Fatalf("允许路应回空对象 {}(最小诚实应答),得到 %s", b)
	}
	if rsp.errCount() != 0 {
		t.Fatalf("允许路不得再回 error: %v", rsp.errCodes)
	}
	var hit bool
	for _, e := range ledgerEntries(t, "codex", codexPassthroughProbeMethod) {
		hit = true
		if e.NormalizedKind != runtime.KindKnownPassthrough {
			t.Fatalf("允许路应记 KNOWN_PASSTHROUGH,得到 %q", e.NormalizedKind)
		}
		if e.ThreadID != "th-cx1" {
			t.Fatalf("台账应带会话归属,得到 %q", e.ThreadID)
		}
		if strings.Contains(e.RawPayload, "secret-cx") {
			t.Fatalf("台账载荷必须脱敏: %s", e.RawPayload)
		}
		if !strings.Contains(e.Note, "空对象") {
			t.Fatalf("台账应写明回的是空对象 {}: %q", e.Note)
		}
	}
	if !hit {
		t.Fatal("允许路必须落 KNOWN_PASSTHROUGH 台账备查")
	}
	if waiting.Load() {
		t.Fatal("waiting 旗标应复位(看门狗要恢复计时)")
	}
}

// TestCodexUnknownRequestPassthroughDenyFallsToLegacy ON + 拒绝:handler 自身不得
// 产生任何应答(成功或 error 都不行),交回 reqUnknown 既有分支统一回 -32601 +
// UNKNOWN_REQUEST 台账 —— 与 OFF 态走的是同一段代码,台账口径天然一致。
func TestCodexUnknownRequestPassthroughDenyFallsToLegacy(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	p := NewCodexProvider()
	rsp := &fakeCodexResponder{}
	var waiting atomic.Bool
	asked := false
	ok := p.consumeCodexUnknownRequest(rsp, 42, "futureDept/codexDeny", "th-cx2",
		json.RawMessage(`{"x":1}`), context.Background(),
		func(*pb.TurnServerMessage) {}, func(context.Context, *pb.ToolCallBlock) (bool, string, error) {
			asked = true
			return false, "", nil // 用户在手机上点了拒绝
		}, &waiting)
	if !asked {
		t.Error("拒绝前提是先问过:approve 应被调用")
	}
	if ok {
		t.Fatal("拒绝路不得终结请求:应交回既有 -32601+台账分支")
	}
	if n := rsp.resultCount(); n != 0 {
		t.Fatalf("拒绝路不得自行回成功应答,记录到 %d 条", n)
	}
	if n := rsp.errCount(); n != 0 {
		t.Fatalf("拒绝路由调用方既有分支回 error,handler 不得抢答: %v", rsp.errCodes)
	}
	if waiting.Load() {
		t.Fatal("waiting 旗标应复位")
	}
}

// TestCodexUnknownRequestPassthroughChannelFailureFallsToLegacy ON + 通道故障/超时:
// approve 返回错误(server 侧 2min 预算到点返回 DeadlineExceeded 同形)→ 按拒绝
// 处理,交回既有 -32601 分支 —— 失败安全方向:最坏损失是"用户本想允许但没来得及点"。
func TestCodexUnknownRequestPassthroughChannelFailureFallsToLegacy(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	p := NewCodexProvider()
	rsp := &fakeCodexResponder{}
	var waiting atomic.Bool
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	ok := p.consumeCodexUnknownRequest(rsp, 43, "futureDept/codexTimeout", "th-cx3",
		json.RawMessage(`{"x":1}`), ctx,
		func(*pb.TurnServerMessage) {}, func(c context.Context, _ *pb.ToolCallBlock) (bool, string, error) {
			<-c.Done() // 用户不点,直到超时
			return false, "", c.Err()
		}, &waiting)
	if ok {
		t.Fatal("超时路不得终结请求:应交回既有 -32601+台账分支(绝不把没等到当成同意)")
	}
	if n := rsp.resultCount(); n != 0 {
		t.Fatalf("超时路不得回成功应答,记录到 %d 条", n)
	}
	if waiting.Load() {
		t.Fatal("waiting 旗标应复位")
	}
}

// TestCodexUnknownRequestPassthroughOffIsLegacy OFF(默认):不问手机、不产生任何
// 应答 —— reqUnknown 分支落回历史行为(-32601 + UNKNOWN_REQUEST 台账)。
func TestCodexUnknownRequestPassthroughOffIsLegacy(t *testing.T) {
	defer stubUnknownPassthrough(false)()
	p := NewCodexProvider()
	rsp := &fakeCodexResponder{}
	var waiting atomic.Bool
	ok := p.consumeCodexUnknownRequest(rsp, 44, "futureDept/codexOff", "th-cx4",
		json.RawMessage(`{"x":1}`), context.Background(),
		func(*pb.TurnServerMessage) {}, func(context.Context, *pb.ToolCallBlock) (bool, string, error) {
			t.Error("OFF 态绝不得询问手机(开关没开就没有确认卡)")
			return true, "", nil
		}, &waiting)
	if ok {
		t.Fatal("OFF 态必须交回既有分支")
	}
	if n := rsp.resultCount() + rsp.errCount(); n != 0 {
		t.Fatalf("OFF 态 handler 不得产生任何应答,记录到 %d 条", n)
	}
}

// TestCodexUnknownRequestPassthroughResolvedRace ON + 等待期间桌面端已解决
// (serverRequest/resolved):双方都不该再应答 —— 迟到的 {} 或 -32601 都是重复
// 应答协议噪声;手机确认卡同步撤下(幽灵审批卡治理,与 handleCodexApproval 同款)。
func TestCodexUnknownRequestPassthroughResolvedRace(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	p := NewCodexProvider()
	rsp := &fakeCodexResponder{}
	var waiting atomic.Bool
	var sinkFrames []*pb.TurnServerMessage
	ok := p.consumeCodexUnknownRequest(rsp, 45, "futureDept/codexResolved", "th-cx5",
		json.RawMessage(`{"x":1}`), context.Background(),
		func(m *pb.TurnServerMessage) { sinkFrames = append(sinkFrames, m) },
		func(context.Context, *pb.ToolCallBlock) (bool, string, error) {
			// 桌面端在此刻解决该请求(serverRequest/resolved 同形):关闭竞速信号
			p.resolvePendingServerRequest(json.RawMessage("45"))
			// 给 resolved 分支让路:approve 迟到的应答落进带缓冲 channel,由 GC 回收
			time.Sleep(20 * time.Millisecond)
			return true, "", nil
		}, &waiting)
	if !ok {
		t.Fatal("已解决路应终结请求(交回既有分支会回迟到的 -32601)")
	}
	if n := rsp.resultCount() + rsp.errCount(); n != 0 {
		t.Fatalf("已解决后绝不应答(codex 侧已不在等待),记录到 %d 条", n)
	}
	ghost := false
	for _, m := range sinkFrames {
		if m.GetApprovalResolved() != nil {
			ghost = true
		}
	}
	if !ghost {
		t.Fatal("已解决路应发 ApprovalResolved 让手机撤下确认卡(幽灵卡治理)")
	}
	if waiting.Load() {
		t.Fatal("waiting 旗标应复位")
	}
}
