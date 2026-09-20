package agent

// zcode_approval_test.go —— P2-6 回归钉子:审批等待必须有上界(对齐 pi 的
// piUIBudget=120s),超时回 deny + 人话;通道故障(断流)仍回 -32000。
//
// 【为什么可以注入 fake】审批分支抽到了 handleZcodePermission:responder 走
// zcodeResponder 最小接口,zcodeapp.Client 天然满足;budget 作参数,测试注入
// 毫秒级超时,不用真等 120s,也不依赖真子进程。

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// fakeZcodeResponder 记录应答的假 Client 应答面。
type fakeZcodeResponder struct {
	mu       sync.Mutex
	results  []map[string]interface{} // Respond 的 result
	errCodes []int                    // RespondError 的 code
	errMsgs  []string                 // RespondError 的 message
}

func (f *fakeZcodeResponder) Respond(rawID json.RawMessage, result interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if m, ok := result.(map[string]interface{}); ok {
		f.results = append(f.results, m)
	}
	return nil
}

func (f *fakeZcodeResponder) RespondError(rawID json.RawMessage, code int, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errCodes = append(f.errCodes, code)
	f.errMsgs = append(f.errMsgs, message)
	return nil
}

func (f *fakeZcodeResponder) lastResult(t *testing.T) map[string]interface{} {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.results) == 0 {
		t.Fatal("没有 Respond 调用")
	}
	return f.results[len(f.results)-1]
}

func (f *fakeZcodeResponder) lastErr(t *testing.T) (int, string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.errCodes) == 0 {
		t.Fatal("没有 RespondError 调用")
	}
	return f.errCodes[len(f.errCodes)-1], f.errMsgs[len(f.errMsgs)-1]
}

func (f *fakeZcodeResponder) resultCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.results)
}

var zcodeApprovalTestParams = json.RawMessage(`{"requestId":"req-1","toolCallId":"tc-1","toolName":"Bash","input":{"command":"ls"}}`)

// TestZcodeApprovalTimeoutAutoDeny 手机不点审批 → 120s(测试注入 50ms)超时后
// 必须 deny 回人话,不能无限挂着;waiting 旗标复位(看门狗恢复计数)。
func TestZcodeApprovalTimeoutAutoDeny(t *testing.T) {
	p := NewZcodeProvider()
	rsp := &fakeZcodeResponder{}
	var waiting atomic.Bool
	// 模拟"手机端一直不应答":approve 挂到 ctx 超时才返回 DeadlineExceeded。
	approve := func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		<-ctx.Done()
		return false, "", ctx.Err()
	}
	ok := p.handleZcodePermission(json.RawMessage(`"server-1"`), zcodeApprovalTestParams,
		context.Background(), rsp, approve, &waiting, 50*time.Millisecond)
	if !ok {
		t.Fatal("审批分支应消费该请求")
	}
	ans := rsp.lastResult(t)
	if ans["decision"] != "deny" {
		t.Fatalf("超时应自动 deny,得到 %v", ans)
	}
	reason, _ := ans["reason"].(string)
	if !strings.Contains(reason, "超时") {
		t.Fatalf("deny 原因应是超时人话,得到 %q", reason)
	}
	if waiting.Load() {
		t.Fatal("waiting 旗标应复位(看门狗要恢复计时)")
	}
}

// TestZcodeApprovalChannelError 非超时的通道故障(手机断流)维持既有口径:
// 回 -32000 error,不把基础设施问题伪装成用户拒绝。
func TestZcodeApprovalChannelError(t *testing.T) {
	p := NewZcodeProvider()
	rsp := &fakeZcodeResponder{}
	var waiting atomic.Bool
	approve := func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		return false, "", errors.New("stream broken")
	}
	p.handleZcodePermission(json.RawMessage(`"server-2"`), zcodeApprovalTestParams,
		context.Background(), rsp, approve, &waiting, time.Second)
	code, msg := rsp.lastErr(t)
	if code != -32000 {
		t.Fatalf("通道故障应回 -32000,得到 %d", code)
	}
	if !strings.Contains(msg, "审批通道失败") {
		t.Fatalf("错误信息应说明通道失败: %q", msg)
	}
}

// TestZcodeApprovalUserDecision 用户真实决定原样透传:allow → schema Jb 形状;
// deny 带用户给的 note(reason 原文回传)。
func TestZcodeApprovalUserDecision(t *testing.T) {
	p := NewZcodeProvider()
	var waiting atomic.Bool

	rspAllow := &fakeZcodeResponder{}
	p.handleZcodePermission(json.RawMessage(`"server-3"`), zcodeApprovalTestParams,
		context.Background(), rspAllow, func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
			return true, "", nil
		}, &waiting, time.Second)
	if ans := rspAllow.lastResult(t); ans["decision"] != "allow" || ans["reason"] != "Approved once" {
		t.Fatalf("allow 应答形状错误: %v", ans)
	}

	rspDeny := &fakeZcodeResponder{}
	p.handleZcodePermission(json.RawMessage(`"server-4"`), zcodeApprovalTestParams,
		context.Background(), rspDeny, func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
			return false, "这个命令太危险", nil
		}, &waiting, time.Second)
	if ans := rspDeny.lastResult(t); ans["decision"] != "deny" || ans["reason"] != "这个命令太危险" {
		t.Fatalf("deny 应答形状错误: %v", ans)
	}
}

// TestZcodeApprovalBadParams 参数无法解析回 -32602(请求被消费,不落到默认 -32601)。
func TestZcodeApprovalBadParams(t *testing.T) {
	p := NewZcodeProvider()
	rsp := &fakeZcodeResponder{}
	var waiting atomic.Bool
	ok := p.handleZcodePermission(json.RawMessage(`"server-5"`), json.RawMessage(`{broken`),
		context.Background(), rsp, nil, &waiting, time.Second)
	if !ok {
		t.Fatal("参数解析失败也应消费请求")
	}
	if code, msg := rsp.lastErr(t); code != -32602 || !strings.Contains(msg, "审批参数") {
		t.Fatalf("应回 -32602,得到 %d %q", code, msg)
	}
}

// TestZcodeApprovalNilChannelErrors P1-2 回归钉子:审批通道缺失(approve==nil,
// 如 TestModel 诊断 RPC 路径)必须回 error 而非 deny 决定 —— 手机从未被询问,
// 绝不得假托"用户在手机上拒绝了该操作"写进 agent 对话上下文(红线 1)。
func TestZcodeApprovalNilChannelErrors(t *testing.T) {
	p := NewZcodeProvider()
	rsp := &fakeZcodeResponder{}
	var waiting atomic.Bool
	ok := p.handleZcodePermission(json.RawMessage(`"server-6"`), zcodeApprovalTestParams,
		context.Background(), rsp, nil, &waiting, time.Second)
	if !ok {
		t.Fatal("通道缺失分支也应消费该请求")
	}
	if n := rsp.resultCount(); n != 0 {
		t.Fatalf("通道缺失绝不应回 deny 决定,记录到 %d 条应答", n)
	}
	code, msg := rsp.lastErr(t)
	if code != -32000 {
		t.Fatalf("应回 -32000,得到 %d", code)
	}
	if !strings.Contains(msg, "审批通道不可用") {
		t.Fatalf("错误信息应说明通道不可用: %q", msg)
	}
	if strings.Contains(msg, "用户") {
		t.Fatalf("错误信息不得假托用户意志: %q", msg)
	}
	if waiting.Load() {
		t.Fatal("waiting 旗标不应被置位(没有等待任何手机应答)")
	}
}
