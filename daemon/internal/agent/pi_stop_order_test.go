// pi_stop_order_test.go —— 用户中止轮次时序的回归钉(2026-10-01,G2)。
//
// 【钉什么】0.99.1 rpc-commands.md 明说 "abort continues queued messages when
// they remain in the session"(abort 不清原生队列,反而让队列续跑),取消配方
// 是 clear_queue 先于 abort。旧实现先 abort 后 clear_queue:预投递的原生
// followUp 会在停止后被继续执行 —— 与"终止任务=连排队的也不跑"相反。
// stopTurnOn 必须先 clear_queue 后 abort,且 clear 失败仍要 abort、错误如实上抛。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/piapp"
)

// fakeStopClient 记录下发顺序的桩(不依赖子进程)。
type fakeStopClient struct {
	calls    []string
	clearErr error
}

func (f *fakeStopClient) ClearQueue(ctx context.Context) (*piapp.ClearedQueue, error) {
	f.calls = append(f.calls, "clear_queue")
	if f.clearErr != nil {
		return nil, f.clearErr
	}
	return &piapp.ClearedQueue{Steering: []string{"s1"}, FollowUp: []string{"f1", "f2"}}, nil
}

func (f *fakeStopClient) CallCtx(ctx context.Context, typ string, params map[string]any, timeout time.Duration) (json.RawMessage, error) {
	f.calls = append(f.calls, typ)
	return nil, nil
}

// 编译期钉死:桩形状与窄接口一致(生产 *piapp.Client 天然满足,零适配)。
var _ piStopClient = (*fakeStopClient)(nil)

func TestStopTurnOnClearsQueueBeforeAbort(t *testing.T) {
	f := &fakeStopClient{}
	steered, followUp, err := stopTurnOn(context.Background(), f)
	if err != nil {
		t.Fatalf("clear 成功时 err 应为 nil: %v", err)
	}
	if steered != 1 || followUp != 2 {
		t.Fatalf("撤销计数不符: steered=%d followUp=%d", steered, followUp)
	}
	if len(f.calls) != 2 || f.calls[0] != "clear_queue" || f.calls[1] != "abort" {
		t.Fatalf("下发顺序必须是 clear_queue → abort, got %v", f.calls)
	}
}

func TestStopTurnOnClearFailureStillAborts(t *testing.T) {
	f := &fakeStopClient{clearErr: errors.New("boom")}
	steered, followUp, err := stopTurnOn(context.Background(), f)
	if err == nil {
		t.Fatal("clear 失败必须上抛错误(调用方据此保留水位)")
	}
	var pe *piError
	if !errors.As(err, &pe) || pe.ErrorInfo().Code != "PI_QUEUE_CLEAR_FAILED" {
		t.Fatalf("错误应为结构化 piError(PI_QUEUE_CLEAR_FAILED), got %v", err)
	}
	if steered != 0 || followUp != 0 {
		t.Fatalf("clear 失败不得虚报撤销计数: %d/%d", steered, followUp)
	}
	if len(f.calls) != 2 || f.calls[0] != "clear_queue" || f.calls[1] != "abort" {
		t.Fatalf("clear 失败也不得跳过 abort(当前操作必须停), got %v", f.calls)
	}
}
