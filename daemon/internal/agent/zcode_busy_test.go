package agent

// zcode_busy_test.go —— PA-REAL-0003 文案窜用修复的回归用例(AUDIT §4:丙类修复
// 必须附带自动化用例):
//  ① zcode 生命周期忙 → 错误文案含"zcode 正忙"、不含"codex 正忙";
//  ② pi 无同款 TryLock/忙哨兵路径(pi.go 全用阻塞 Lock,从不返回忙错误),
//     无窜用面,N/A;
//  ③ errors.Is(err, ErrBusy) 在新包装下仍为真 —— T-14 的 modelsListErr 归类
//     (ErrBusy → ErrModelsInitializing 可重试态)不回归;
//  ④ codex 自身的 ErrBusy 原文案不动(它本来就是 codex),TryLock 路径照旧。
//  ⑤ (T-25)withControlClient 的 TryLock 失败路径同改 ErrZcodeBusy —— 它与
//     withClientCtx 是 zcode 仅有的两条 TryLock 忙路径,钉法与 ①③ 相同。

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// disableZcodeLockWait 把 zcode 的排队窗口临时置 0(等价纯 TryLock 语义)。
//
// 【为什么需要它】本文件钉的是"忙文案归属"与"分级链",不是排队行为。排队窗口
// 生产默认 15 秒,会让持锁用例白等满一个窗口(实测把 -run 'Zcode|Busy' 从 ~10s
// 拖到 35s)。排队本身另有专门用例覆盖,这里只把语义恢复成改动前的纯 TryLock。
func disableZcodeLockWait(t *testing.T) {
	t.Helper()
	old := zcodeLockWait
	zcodeLockWait = 0
	t.Cleanup(func() { zcodeLockWait = old })
}

// TestZcodeBusyMessageAttributesToZcode ①:zcode 自己的生命周期锁被占,
// 文案必须如实指明"zcode 正忙",不得窜用 codex 的忙文案。
func TestZcodeBusyMessageAttributesToZcode(t *testing.T) {
	disableZcodeLockWait(t)
	p := &ZcodeProvider{}
	p.lifeMu.Lock() // 模拟另一个操作正持有 zcode 生命周期锁
	defer p.lifeMu.Unlock()

	err := p.withClientCtx(context.Background(), time.Second, func(context.Context, *zcodeapp.Client) error { return nil })
	if err == nil {
		t.Fatal("生命周期锁被占时应返回忙错误, got=nil")
	}
	if !strings.Contains(err.Error(), "zcode 正忙") {
		t.Fatalf("文案应含 \"zcode 正忙\", got=%v", err)
	}
	if strings.Contains(err.Error(), "codex 正忙") {
		t.Fatalf("zcode 忙不得报成 codex 忙(PA-REAL-0003 文案窜用), got=%v", err)
	}
	if !errors.Is(err, ErrZcodeBusy) {
		t.Fatalf("应 errors.Is(err, ErrZcodeBusy), got=%v", err)
	}
}

// TestZcodeBusyKeepsErrBusyClassification ③:新包装必须维持 ErrBusy 分级链。
// codex.modelsListErr(T-14,PA-REAL-0002)靠 errors.Is(err, ErrBusy) 把忙
// 归类为可重试态 ErrModelsInitializing —— 链断了模型列表的可重试分级就回归。
func TestZcodeBusyKeepsErrBusyClassification(t *testing.T) {
	if !errors.Is(ErrZcodeBusy, ErrBusy) {
		t.Fatal("ErrZcodeBusy 必须保持 errors.Is(_, ErrBusy) 为真(分级兼容)")
	}
	got := modelsListErr(ErrZcodeBusy)
	if !errors.Is(got, ErrModelsInitializing) {
		t.Fatalf("忙错误经 modelsListErr 应归类为 ErrModelsInitializing, got=%v", got)
	}
}

// TestCodexBusyMessageUnchanged ④:codex 自身的 ErrBusy 文案不动,
// withClientCtx 的 TryLock 路径照旧返回原文案。
func TestCodexBusyMessageUnchanged(t *testing.T) {
	const want = "codex 正忙(另一个操作正在初始化/探测),请稍后重试"
	if ErrBusy.Error() != want {
		t.Fatalf("codex ErrBusy 文案不得变更: want=%q got=%q", want, ErrBusy.Error())
	}
	p := &CodexProvider{}
	p.lifeMu.Lock()
	defer p.lifeMu.Unlock()

	err := p.withClientCtx(context.Background(), clientInitBudget, func(context.Context, *codexapp.Client) error { return nil })
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("codex 锁被占时应仍返回 ErrBusy, got=%v", err)
	}
	if err.Error() != want {
		t.Fatalf("codex 忙文案不得变更: want=%q got=%q", want, err.Error())
	}
}

// TestZcodeControlClientBusyMessageAttributesToZcode ⑤(T-25):控制面入口
// withControlClient(selection/mode/命令发现都走它)的 TryLock 失败路径与
// withClientCtx 同款——此前直接返回 ErrBusy,zcode 自己忙被标成"codex 正忙"
// (与 T-23 修的窜用同款)。钉法照抄 ①③:文案归属 zcode + errors.Is 分级为真。
func TestZcodeControlClientBusyMessageAttributesToZcode(t *testing.T) {
	disableZcodeLockWait(t)
	p := &ZcodeProvider{}
	p.lifeMu.Lock() // 模拟另一个操作正持有 zcode 生命周期锁
	defer p.lifeMu.Unlock()

	err := p.withControlClient(context.Background(), func(context.Context, *zcodeapp.Client) error { return nil })
	if err == nil {
		t.Fatal("生命周期锁被占时应返回忙错误, got=nil")
	}
	if !strings.Contains(err.Error(), "zcode 正忙") {
		t.Fatalf("文案应含 \"zcode 正忙\", got=%v", err)
	}
	if strings.Contains(err.Error(), "codex 正忙") {
		t.Fatalf("zcode 忙不得报成 codex 忙(文案窜用), got=%v", err)
	}
	if !errors.Is(err, ErrZcodeBusy) {
		t.Fatalf("应 errors.Is(err, ErrZcodeBusy), got=%v", err)
	}
	if !errors.Is(err, ErrBusy) {
		t.Fatal("withControlClient 忙错误必须保持 errors.Is(_, ErrBusy) 为真(分级兼容)")
	}
}
