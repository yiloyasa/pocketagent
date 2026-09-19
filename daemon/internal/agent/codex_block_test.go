package agent

// codex_block_test.go —— withClient 的"有界化"回归测试(daemon 侧)。
//
// 【为什么这两条必须钉住】
//  1. TryLock:锁被占用时**立即**返回 ErrBusy,而不是排队等锁 —— 排队正是把
//     "一次探测卡死"放大成"设置页/模型列表/MCP 全面卡死"的根因。
//  2. ProbeHook:探测注入点可用(生产为 nil,走真实探测)。

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestWithClientTryLockReturnsBusyImmediately 锁被占时,ReadyCtx 立即 ErrBusy。
func TestWithClientTryLockReturnsBusyImmediately(t *testing.T) {
	p := &CodexProvider{}
	p.lifeMu.Lock()
	defer p.lifeMu.Unlock()

	start := time.Now()
	err := p.ReadyCtx(context.Background())
	elapsed := time.Since(start)

	if !errors.Is(err, ErrBusy) {
		t.Fatalf("拿不到锁时应返回 ErrBusy, got=%v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("应立返回 ErrBusy(不排队等锁),实际等了 %v", elapsed)
	}
}

// TestReadyCtxHonorsProbeHook 探测注入点生效,且错误原样透传给上层分级。
func TestReadyCtxHonorsProbeHook(t *testing.T) {
	p := &CodexProvider{ProbeHook: func(context.Context) error { return ErrBusy }}
	if err := p.ReadyCtx(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatalf("ProbeHook 未被使用或错误未透传: %v", err)
	}
}
