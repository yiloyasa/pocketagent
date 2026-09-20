package agent

// codex_models_status_test.go —— PA-REAL-0002 回归(agent 侧)。
//
// 【缺陷回顾】2026-09-19 真机:daemon 冷启动窗口内首次 ListModels 吃满 30s
// 客户端超时(DeadlineExceeded),之后连续 3 次才 8ms 正常。codex 的 model/list
// 走 withClientCtx 时,initialize 握手期间的等待随调用方截止时间走(手机端 30s),
// 且全程持有 lifeMu 把其它 codex 调用挡成 ErrBusy。
//
// 【钉死的行为】ModelsStatusCtx(状态预算通道):
//  1. 锁被占(另一个操作正在初始化/探测)→ 预算内返回**可重试的人话错误**
//     ErrModelsInitializing("模型列表初始化中,请稍后重试"),不再裸阻塞/裸超时;
//  2. 返回的列表永远含真实的 "default"(跟随桌面)条目 —— 绝不返回空列表
//     冒充成功(空列表会让手机端模型抽屉"看起来正常但选不了");
//  3. 调用方给过更紧的截止时间时按更早者执行(探测路径不被拖慢);
//  4. 错误归类只针对"暂时取不到"(忙/超时/取消);codex 的原生错误原文透传,
//     绝不改写(README §7:显示诊断而非伪造)。

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestModelsStatusCtxBusyReturnsRetryableError 锁被占:预算内返回初始化中(可重试)。
func TestModelsStatusCtxBusyReturnsRetryableError(t *testing.T) {
	p := &CodexProvider{modelsBudget: 200 * time.Millisecond}
	p.lifeMu.Lock()
	defer p.lifeMu.Unlock()

	start := time.Now()
	ms, err := p.ModelsStatusCtx(context.Background())
	elapsed := time.Since(start)

	if !errors.Is(err, ErrModelsInitializing) {
		t.Fatalf("锁被占时应返回 ErrModelsInitializing(可重试态), got=%v", err)
	}
	if !strings.Contains(err.Error(), "初始化中") || !strings.Contains(err.Error(), "稍后重试") {
		t.Fatalf("错误文案应是给用户看的人话, got=%q", err.Error())
	}
	if elapsed > time.Second {
		t.Fatalf("应在预算(200ms)内返回而不是吃满客户端超时, 实际 %v", elapsed)
	}
	// 不伪造:列表仍含真实的 default(跟随桌面)条目,但没有伪造的具体模型。
	if len(ms) != 1 || ms[0].Id != "default" || ms[0].Provider != "codex" {
		t.Fatalf("应只含真实的 default 条目, got=%+v", ms)
	}
}

// TestModelsStatusCtxHonorsCallerDeadline 调用方截止时间比默认预算更紧时取更早者。
func TestModelsStatusCtxHonorsCallerDeadline(t *testing.T) {
	p := &CodexProvider{} // 未注入 modelsBudget → 默认 statusBudget(10s)
	p.lifeMu.Lock()
	defer p.lifeMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := p.ModelsStatusCtx(ctx)

	if !errors.Is(err, ErrModelsInitializing) {
		t.Fatalf("预算耗尽应返回可重试的初始化中, got=%v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("应按调用方 150ms 截止返回(不反向放大到 10s), 实际 %v", elapsed)
	}
}

// TestModelsListErrClassification 错误归类纯函数:忙/超时/取消 → 可重试态,
// 且原始错误保留在错误链上(上层 errors.Is 分级不受影响);codex 原生错误
// 原样透传,不改写、不吞掉。
func TestModelsListErrClassification(t *testing.T) {
	for _, err := range []error{ErrBusy, context.DeadlineExceeded, context.Canceled} {
		got := modelsListErr(err)
		if !errors.Is(got, ErrModelsInitializing) {
			t.Fatalf("%v 应归类为 ErrModelsInitializing, got=%v", err, got)
		}
		if !errors.Is(got, err) {
			t.Fatalf("原始错误应保留在错误链上供 errors.Is 分级: %v", got)
		}
		if !strings.Contains(got.Error(), "初始化中") {
			t.Fatalf("人话文案丢失: %q", got.Error())
		}
	}
	native := errors.New("codex 返回错误 -1: Failed to refresh token")
	if got := modelsListErr(native); got != native {
		t.Fatalf("codex 原生错误必须原样透传(不加工), got=%v", got)
	}
}
