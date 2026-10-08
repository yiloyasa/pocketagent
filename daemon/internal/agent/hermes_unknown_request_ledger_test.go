// hermes_unknown_request_ledger_test.go —— 2026-10-07 六家横切旁路审计移交项的
// 收口钉子(docs/reviews/2026-10-07/unknown-bypass-audit.md §四:hermes 的
// session/request_permission 未知方法路径回 -32601 但无台账登记,四家同位路径
// 均有;比照 dsh_unknown_request_ledger_test.go 同款口径)。
//
// 【钉什么】hermes 收到未登记反向请求时是**一对**动作:回 -32601 协议错误(不
// 伪造 decision,应答语义与改前逐字节一致,由既有的
// TestHermesUnknownReverseRequestRejected 钉住)+ 在诊断台账落 UNKNOWN_REQUEST
// 留痕。"回错了"用户当下会发现,"没留痕"要等事后排查才发现 —— 本文件钉后半边。
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// TestHermesUnknownReverseRequestLandsInUnknownRequestLedger 未知反向请求除了回
// -32601,必须以 UNKNOWN_REQUEST 进诊断台账:载荷脱敏、标注"未替用户做任何
// 审批决定" —— 事后排查时能看到"上游发来过什么、我们怎么处置的"。
func TestHermesUnknownReverseRequestLandsInUnknownRequestLedger(t *testing.T) {
	const unseenMethod = "futureDept/neverSeenRequest" // 探针专用:真实上游不会发
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)

	p.handleRequestPermission(fake, 21, unseenMethod, json.RawMessage(`{"token":"secret-req","x":1}`),
		context.Background(), func(*pb.TurnServerMessage) {}, nil, "th-hermes-approval")

	// 回复侧:-32601 语义不变,绝不带任何 decision 形状的成功应答。
	fake.mu.Lock()
	if len(fake.respondErr) != 1 || fake.respondErr[0].Code != -32601 || fake.respondErr[0].ID != 21 {
		fake.mu.Unlock()
		t.Fatalf("未知反向请求应回 -32601(语义不变): %+v", fake.respondErr)
	}
	fake.mu.Unlock()

	// 台账侧:UNKNOWN_REQUEST + 脱敏 + 处置说明。
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "hermes"}, 0) {
		if e.SourceMethod != unseenMethod {
			continue
		}
		if e.NormalizedKind != runtime.KindUnknownRequest {
			t.Fatalf("应记 UNKNOWN_REQUEST,得到 %q", e.NormalizedKind)
		}
		if strings.Contains(e.RawPayload, "secret-req") {
			t.Fatalf("载荷必须脱敏,得到 %s", e.RawPayload)
		}
		if !strings.Contains(e.Note, "未替用户做任何审批决定") {
			t.Fatalf("台账应注明安全拒绝语义(不留痕等于默许了'也许批过'),得到 %q", e.Note)
		}
		return
	}
	t.Fatal("未知反向请求必须落 UNKNOWN_REQUEST 诊断台账(红线:不自动批准,也要能事后查)")
}

// TestHermesKnownPermissionMethodSkipsUnknownLedger 已知审批方法不得冒充未知:
// request_permission 正常路径(此处取 -32602 参数坏形分支)不得在台账留下
// UNKNOWN_REQUEST 登记 —— 台账里的 UNKNOWN_REQUEST 必须专指"不认识的方法"。
func TestHermesKnownPermissionMethodSkipsUnknownLedger(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)

	p.handleRequestPermission(fake, 33, "session/request_permission", json.RawMessage(`{"bad":`),
		context.Background(), func(*pb.TurnServerMessage) {}, nil, "th-hermes-approval")

	fake.mu.Lock()
	if len(fake.respondErr) != 1 || fake.respondErr[0].Code != -32602 {
		fake.mu.Unlock()
		t.Fatalf("坏形参数应回 -32602: %+v", fake.respondErr)
	}
	fake.mu.Unlock()

	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "hermes"}, 0) {
		if e.SourceMethod == "session/request_permission" {
			t.Fatalf("已知方法的参数级失败不得记 UNKNOWN_REQUEST(那是未知方法专用档): %+v", e)
		}
	}
}
