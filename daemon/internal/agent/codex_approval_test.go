package agent

// codex_approval_test.go —— P1-1 回归钉子:审批通道缺失(approve==nil,如
// TestModel 诊断 RPC 路径传 nil)时,必须回 JSON-RPC error(-32000 审批通道
// 不可用),绝不替用户伪造 {"decision":"denied"};approve!=nil 的真实决定
// 应答形状(旧版 approved/denied 枚举)不受影响。
//
// 【注入方式】审批问询已抽到 handleCodexApproval(与 zcode 的
// handleZcodePermission 同款模式),responder 走 codexApprovalResponder
// 最小接口,单测注 fake,不依赖真子进程。

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// fakeCodexResponder 记录应答的假 codexapp.Client 应答面。
type fakeCodexResponder struct {
	mu       sync.Mutex
	results  []map[string]interface{} // Respond 的 result
	errCodes []int                    // RespondError 的 code
	errMsgs  []string                 // RespondError 的 message
}

func (f *fakeCodexResponder) Respond(id json.RawMessage, result interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch m := result.(type) {
	case map[string]interface{}:
		f.results = append(f.results, m)
	case map[string]string:
		// codex 审批 decision 应答用 map[string]string,归一后统一记录。
		conv := make(map[string]interface{}, len(m))
		for k, v := range m {
			conv[k] = v
		}
		f.results = append(f.results, conv)
	}
	return nil
}

func (f *fakeCodexResponder) RespondError(id json.RawMessage, code int, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errCodes = append(f.errCodes, code)
	f.errMsgs = append(f.errMsgs, message)
	return nil
}

func (f *fakeCodexResponder) resultCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.results)
}

func (f *fakeCodexResponder) errCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.errCodes)
}

// TestCodexApprovalNilChannelErrors approve=nil:收到 JSON-RPC error 而非
// denied 决定 —— daemon 不替用户做审批决定(红线 1)。
func TestCodexApprovalNilChannelErrors(t *testing.T) {
	p := NewCodexProvider()
	rsp := &fakeCodexResponder{}
	var waiting atomic.Bool
	p.handleCodexApproval(rsp, json.RawMessage("7"), reqApprovalCommand, "execCommandApproval",
		json.RawMessage(`{"command":["rm","-rf","tmp"],"cwd":"."}`), nil,
		context.Background(), func(*pb.TurnServerMessage) {}, nil, &waiting)
	if n := rsp.resultCount(); n != 0 {
		t.Fatalf("通道缺失绝不应回 decision,记录到 %d 条应答", n)
	}
	if rsp.errCount() != 1 {
		t.Fatalf("应回恰好一个 JSON-RPC error,得到 %d 个", rsp.errCount())
	}
	if code, msg := rsp.errCodes[0], rsp.errMsgs[0]; code != -32000 || !strings.Contains(msg, "审批通道不可用") {
		t.Fatalf("应回 -32000 + 审批通道不可用,得到 %d %q", code, msg)
	}
	if waiting.Load() {
		t.Fatal("waiting 旗标不应被置位(没有等待任何手机应答)")
	}
}

// TestCodexApprovalUserDecisionUnchanged approve!=nil:真实决定照旧翻译成
// 协议枚举(旧版 execCommandApproval 用 approved/denied),抽出函数不改语义。
func TestCodexApprovalUserDecisionUnchanged(t *testing.T) {
	p := NewCodexProvider()
	var waiting atomic.Bool
	sink := func(*pb.TurnServerMessage) {}

	rspDeny := &fakeCodexResponder{}
	p.handleCodexApproval(rspDeny, json.RawMessage("8"), reqApprovalCommand, "execCommandApproval",
		json.RawMessage(`{"command":["ls"]}`), nil,
		context.Background(), sink, func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
			return false, "", nil
		}, &waiting)
	if n := rspDeny.resultCount(); n != 1 || rspDeny.results[0]["decision"] != "denied" {
		t.Fatalf("真实拒绝应照旧回 denied,得到 %v", rspDeny.results)
	}
	if rspDeny.errCount() != 0 {
		t.Fatalf("真实决定不应产生 error,得到 %v", rspDeny.errCodes)
	}

	rspAllow := &fakeCodexResponder{}
	p.handleCodexApproval(rspAllow, json.RawMessage("9"), reqApprovalCommand, "execCommandApproval",
		json.RawMessage(`{"command":["ls"]}`), nil,
		context.Background(), sink, func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
			return true, "", nil
		}, &waiting)
	if n := rspAllow.resultCount(); n != 1 || rspAllow.results[0]["decision"] != "approved" {
		t.Fatalf("真实允许应照旧回 approved,得到 %v", rspAllow.results)
	}
}

// TestCodexApprovalChannelErrorUnchanged approve!=nil 但通道故障(断流):
// 维持既有口径回 -32000"审批通道失败",不伪装成用户意志。
func TestCodexApprovalChannelErrorUnchanged(t *testing.T) {
	p := NewCodexProvider()
	rsp := &fakeCodexResponder{}
	var waiting atomic.Bool
	p.handleCodexApproval(rsp, json.RawMessage("10"), reqApprovalCommand, "execCommandApproval",
		json.RawMessage(`{"command":["ls"]}`), nil,
		context.Background(), func(*pb.TurnServerMessage) {}, func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
			return false, "", context.Canceled
		}, &waiting)
	if code, msg := rsp.errCodes[0], rsp.errMsgs[0]; code != -32000 || !strings.Contains(msg, "审批通道失败") {
		t.Fatalf("通道故障应回 -32000 审批通道失败,得到 %d %q", code, msg)
	}
	if n := rsp.resultCount(); n != 0 {
		t.Fatalf("通道故障不应回 decision,记录到 %d 条应答", n)
	}
}
