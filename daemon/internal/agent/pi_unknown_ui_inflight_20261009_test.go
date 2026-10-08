// pi_unknown_ui_inflight_20261009_test.go —— 未知 extension_ui_request 在飞上限闸
// (2026-10-09)回归钉:此前事件泵对每条未知请求无条件 go 出去(各以 piUIBudget=
// 120s 为界,窗口内照样堆积),上游扩展异常刷屏时确认 goroutine 与手机确认卡无上界。
// 收口:acquireUnknownUIInFlight 同步占坑(Add 返回值即判定,无 check-then-act
// 竞态),超限直接回"忙"拒绝(piRejectUnknownUIBusy:pi 协议原生兜底
// {cancelled:true} + 台账 UNKNOWN_REQUEST,备注写明 daemon 容量拒绝、非用户决定)。
//
// 【为什么只钉闸与拒绝函数】Run 泵的闸接线依赖真子进程 harness,由既有 pi 契约
// 套件覆盖挂载路径(pi_watchdog_test.go"只钉判定函数"同款取舍);这里钉决策面
// (超限必拒、超限不占坑、释放后恢复)与副作用面(忙拒绝的回写形状 + 台账)。
package agent

import (
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

// TestPiUnknownUIInFlightLimitDefault 生产默认值钉子:上限盖过任何真实场景的并发
// 未知请求(正常交互是"一次一张卡"),只拦异常洪水;改值必须有意识。
func TestPiUnknownUIInFlightLimitDefault(t *testing.T) {
	if piUnknownUIInFlightLimit != 16 {
		t.Fatalf("piUnknownUIInFlightLimit 生产默认应 16,得到 %d", piUnknownUIInFlightLimit)
	}
}

// TestPiUnknownUIInFlightLimitGate 缩时钉闸的决策面:占满窗口后第 limit+1 个必被
// 拒、超限拒绝不占坑(窗口内不泄漏)、释放后恢复放行(计数泄漏会永久拒新请求)。
func TestPiUnknownUIInFlightLimitGate(t *testing.T) {
	old := piUnknownUIInFlightLimit
	piUnknownUIInFlightLimit = 2
	t.Cleanup(func() { piUnknownUIInFlightLimit = old })

	p := NewPiProvider()
	// 占满窗口:前 limit 个占坑成功。
	for i := 0; i < 2; i++ {
		if !p.acquireUnknownUIInFlight() {
			t.Fatalf("第 %d 个应占坑成功(窗口未满)", i+1)
		}
	}
	// 第 limit+1 个必被拒(判定与占坑同一次 Add,无竞态缝)。
	if p.acquireUnknownUIInFlight() {
		p.uiInFlight.Add(-1) // 不把测试自身的坑位泄漏进后续断言
		t.Fatal("第 limit+1 个必须被拒")
	}
	// 超限拒绝不占坑:在飞计数保持 limit,窗口内不泄漏。
	if got := p.uiInFlight.Load(); got != 2 {
		t.Fatalf("超限拒绝不得占坑,uiInFlight 应保持 2,得到 %d", got)
	}
	// 释放一个坑位即恢复放一个(计数守恒,不泄漏)。
	p.uiInFlight.Add(-1)
	if !p.acquireUnknownUIInFlight() {
		t.Fatal("释放窗口后应恢复放行(在飞计数泄漏会永久拒新请求)")
	}
	if got := p.uiInFlight.Load(); got != 2 {
		t.Fatalf("放行后在飞应保持 2(原坑 1 + 新坑 1),得到 %d", got)
	}
	// 收尾退坑:原坑与新坑各退一次,窗口清零(不泄漏)。
	p.uiInFlight.Add(-1)
	p.uiInFlight.Add(-1)
	if got := p.uiInFlight.Load(); got != 0 {
		t.Fatalf("窗口应清零,得到 %d(泄漏 %d 个坑位)", got, got)
	}
}

// TestPiRejectUnknownUIBusyCancelledAndLedger 忙拒绝的副作用面:回写协议原生兜底
// {cancelled:true}(pi 的 extension_ui_response 无 -32601 通道)+ 台账
// UNKNOWN_REQUEST(载荷脱敏、备注写明容量拒绝非用户决定)。这是失败安全处置:
// 被拒方拿到的是"这次没做成",绝不是"用户同意了"。
func TestPiRejectUnknownUIBusyCancelledAndLedger(t *testing.T) {
	rsp := &fakePiUIResponder{}
	frame := []byte(`{"type":"extension_ui_request","id":"u-busy","method":"futureFlood","token":"secret-busy"}`)
	piRejectUnknownUIBusy(rsp, frame, "th-busy")

	if n := rsp.count(); n != 1 {
		t.Fatalf("忙拒绝应恰好回写一次,得到 %d 次", n)
	}
	id, payload := rsp.last(t)
	if id != "u-busy" {
		t.Fatalf("忙拒绝回写 id 应为请求原 id,得到 %q", id)
	}
	if v, ok := payload["cancelled"]; !ok || v != true {
		t.Fatalf("忙拒绝应回原生兜底 {cancelled:true}(pi 无 -32601 通道),得到 %v", payload)
	}
	var hit bool
	for _, e := range ledgerEntries(t, "pi", "futureFlood") {
		hit = true
		if e.NormalizedKind != runtime.KindUnknownRequest {
			t.Fatalf("忙拒绝应记 UNKNOWN_REQUEST,得到 %q", e.NormalizedKind)
		}
		if e.ThreadID != "th-busy" {
			t.Fatalf("台账应带会话归属,得到 %q", e.ThreadID)
		}
		if strings.Contains(e.RawPayload, "secret-busy") {
			t.Fatalf("台账载荷必须脱敏: %s", e.RawPayload)
		}
		if !strings.Contains(e.Note, "非用户决定") {
			t.Fatalf("台账备注必须写明容量拒绝非用户决定: %s", e.Note)
		}
	}
	if !hit {
		t.Fatal("忙拒绝必须落 UNKNOWN_REQUEST 台账(洪水要能事后查)")
	}
}
