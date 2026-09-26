// pi_unknown_passthrough_test.go —— 辅线 B「未知反向请求透传确认」的 pi 三态钉子。
//
// 【钉什么】pi 的反向请求面是 extension_ui_request:已知 dialog(select/confirm/
// input/editor)由 UIRequestHandler 专线处理,已知 fire-and-forget(notify/
// setStatus/setWidget/setTitle/set_editor_text,实测帧见 docs/verify/evidence/
// pi-p0-probe.md)无应答通道 —— 两者都不是"未知",任何状态下都维持原样静默。
// 只有**未知 method**(上游发版新增的 UI 交互)进透传确认:
//  1. OFF(默认)→ 原样静默(不问手机、不回写、不落 UNKNOWN 台账);
//  2. ON + 允许 → 回写 extension_ui_response 空对象 {}(pi payload 通道无业务
//     结果形状,{} 是尽力应答)+ 台账 KNOWN_PASSTHROUGH;
//  3. ON + 拒绝/超时 → 回写原生兜底拒绝 {"cancelled":true}(pi 的 extension_ui_
//     response 没有 -32601 错误码通道,cancelled 是协议内唯一的安全拒绝形状,
//     与 UIRequestHandler 既有失败安全同款)+ 台账 UNKNOWN_REQUEST。
package agent

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// fakePiUIResponder 记录回写的假应答面(RespondUI 窄接口,真 *piapp.Client 天然满足)。
type fakePiUIResponder struct {
	mu       sync.Mutex
	ids      []string
	payloads []map[string]any
}

func (f *fakePiUIResponder) RespondUI(id string, payload map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ids = append(f.ids, id)
	f.payloads = append(f.payloads, payload)
	return nil
}

func (f *fakePiUIResponder) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.ids)
}

func (f *fakePiUIResponder) last(t *testing.T) (string, map[string]any) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.ids) == 0 {
		t.Fatal("应有一次 extension_ui_response 回写")
	}
	return f.ids[len(f.ids)-1], f.payloads[len(f.payloads)-1]
}

func TestPiUnknownRequestPassthroughAllow(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	p := NewPiProvider()
	rsp := &fakePiUIResponder{}
	frame := []byte(`{"type":"extension_ui_request","id":"u1","method":"futureDialog","message":"试一下","token":"secret-pi"}`)
	approve := func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		if call.GetName() != "unknown_request" {
			t.Errorf("确认卡动作名应为 unknown_request,得到 %q", call.GetName())
		}
		if !strings.Contains(call.GetJsonArgs(), "futureDialog") {
			t.Errorf("确认卡必须带 method 原文: %s", call.GetJsonArgs())
		}
		if !strings.Contains(call.GetJsonArgs(), "未识别的操作") {
			t.Errorf("确认卡必须写明这是未识别的操作: %s", call.GetJsonArgs())
		}
		return true, "", nil
	}
	ok := p.consumePiUnknownUIRequest(rsp, frame, "th-pi1", context.Background(), approve)
	if !ok {
		t.Fatal("未知请求 + 允许:应消费并回写")
	}
	if n := rsp.count(); n != 1 {
		t.Fatalf("应恰好回写一次,得到 %d 次", n)
	}
	id, payload := rsp.last(t)
	if id != "u1" {
		t.Fatalf("回写 id 应为请求原 id,得到 %q", id)
	}
	if len(payload) != 0 {
		t.Fatalf("允许路应回空对象 {}(尽力应答),得到 %v", payload)
	}
	var hit bool
	for _, e := range ledgerEntries(t, "pi", "futureDialog") {
		hit = true
		if e.NormalizedKind != runtime.KindKnownPassthrough {
			t.Fatalf("允许路应记 KNOWN_PASSTHROUGH,得到 %q", e.NormalizedKind)
		}
		if e.ThreadID != "th-pi1" {
			t.Fatalf("台账应带会话归属,得到 %q", e.ThreadID)
		}
		if strings.Contains(e.RawPayload, "secret-pi") {
			t.Fatalf("台账载荷必须脱敏: %s", e.RawPayload)
		}
	}
	if !hit {
		t.Fatal("允许路必须落 KNOWN_PASSTHROUGH 台账备查")
	}
}

func TestPiUnknownRequestPassthroughDeny(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	p := NewPiProvider()
	rsp := &fakePiUIResponder{}
	frame := []byte(`{"type":"extension_ui_request","id":"u2","method":"futureDialogDeny"}`)
	ok := p.consumePiUnknownUIRequest(rsp, frame, "th-pi2", context.Background(),
		func(context.Context, *pb.ToolCallBlock) (bool, string, error) {
			return false, "", nil // 用户在手机上点了拒绝
		})
	if !ok {
		t.Fatal("拒绝路也应消费请求(pi 的 consume 事件分支没有可回落的应答路径)")
	}
	id, payload := rsp.last(t)
	if id != "u2" {
		t.Fatalf("拒绝路回写 id 应为 %q,得到 %q", "u2", id)
	}
	if v, ok := payload["cancelled"]; !ok || v != true {
		t.Fatalf("拒绝路应回原生兜底拒绝 {cancelled:true}(pi 无 -32601 通道),得到 %v", payload)
	}
	var hit bool
	for _, e := range ledgerEntries(t, "pi", "futureDialogDeny") {
		hit = true
		if e.NormalizedKind != runtime.KindUnknownRequest {
			t.Fatalf("拒绝路应记 UNKNOWN_REQUEST,得到 %q", e.NormalizedKind)
		}
	}
	if !hit {
		t.Fatal("拒绝路必须落 UNKNOWN_REQUEST 台账")
	}
}

func TestPiUnknownRequestPassthroughTimeout(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	p := NewPiProvider()
	rsp := &fakePiUIResponder{}
	frame := []byte(`{"type":"extension_ui_request","id":"u3","method":"futureDialogTimeout"}`)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	ok := p.consumePiUnknownUIRequest(rsp, frame, "th-pi3", ctx,
		func(c context.Context, _ *pb.ToolCallBlock) (bool, string, error) {
			<-c.Done() // 用户不点,直到超时
			return false, "", c.Err()
		})
	if !ok {
		t.Fatal("超时路也应消费请求:按拒绝回写,绝不把没等到当成同意")
	}
	_, payload := rsp.last(t)
	if v, ok := payload["cancelled"]; !ok || v != true {
		t.Fatalf("超时路应回 {cancelled:true},得到 %v", payload)
	}
	if n := len(ledgerEntries(t, "pi", "futureDialogTimeout")); n == 0 {
		t.Fatal("超时路必须落 UNKNOWN_REQUEST 台账")
	}
}

func TestPiUnknownRequestPassthroughOffIsLegacy(t *testing.T) {
	defer stubUnknownPassthrough(false)()
	p := NewPiProvider()
	rsp := &fakePiUIResponder{}
	frame := []byte(`{"type":"extension_ui_request","id":"u4","method":"futureDialogOff"}`)
	ok := p.consumePiUnknownUIRequest(rsp, frame, "th-pi4", context.Background(),
		func(context.Context, *pb.ToolCallBlock) (bool, string, error) {
			t.Error("OFF 态绝不得询问手机(开关没开就没有确认卡)")
			return true, "", nil
		})
	if ok {
		t.Fatal("OFF 态必须维持原样静默(返回未消费)")
	}
	if n := rsp.count(); n != 0 {
		t.Fatalf("OFF 态不得回写,记录到 %d 次", n)
	}
	if n := len(ledgerEntries(t, "pi", "futureDialogOff")); n != 0 {
		t.Fatalf("OFF 态不得新增 UNKNOWN 台账(原样 = 静默),记录到 %d 条", n)
	}
}

// TestPiUnknownRequestSkipsKnownMethods 已知 method 永远不进透传确认:dialog 四种
// 由 UIRequestHandler 专线处理(事件流里再见到必须放过,否则会弹两张卡);
// fire-and-forget 五种无应答通道,回写只会制造协议噪声。
func TestPiUnknownRequestSkipsKnownMethods(t *testing.T) {
	defer stubUnknownPassthrough(true)()
	p := NewPiProvider()
	rsp := &fakePiUIResponder{}
	for _, m := range []string{"select", "confirm", "input", "editor",
		"notify", "setStatus", "setWidget", "setTitle", "set_editor_text"} {
		frame := []byte(`{"type":"extension_ui_request","id":"k-` + m + `","method":"` + m + `","message":"x"}`)
		if ok := p.consumePiUnknownUIRequest(rsp, frame, "th-pi-k", context.Background(),
			func(context.Context, *pb.ToolCallBlock) (bool, string, error) {
				t.Errorf("已知 method %q 绝不得弹未知请求确认卡", m)
				return true, "", nil
			}); ok {
			t.Fatalf("已知 method %q 不得被未知请求路径消费", m)
		}
	}
	if n := rsp.count(); n != 0 {
		t.Fatalf("已知 method 不得回写,记录到 %d 次", n)
	}
}
