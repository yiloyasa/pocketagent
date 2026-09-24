// dsh_unknown_request_ledger_test.go —— AUDIT §3.3「mock 未知反向请求」的台账半边
// (README §7 红线:UNKNOWN_REQUEST 绝不自动批准)。
//
// 【钉什么】dsh 收到未登记反向请求时是**一对**动作:回 -32601 协议错误(不伪造
// decision)+ 在诊断台账落 UNKNOWN_REQUEST 留痕。前半边已由
// TestDshUnknownReverseRequestNoFabricatedDecision 钉住;本文件钉后半边 ——
// "回错了"用户当下会发现,"没留痕"要等事后排查才发现,失效原因不同。
//
// 【为什么只钉 dsh】四个 provider 里,未知反向请求的"回 error + 落台账"成对实现
// 且可注入驱动的是 dsh(handleDshRequest + runWithPendingApproval 既有基建);
// codex 的同款分支在 Run 的闭包里,现有 fake 子进程 harness 不发反向请求,新增
// 文件动不了它 —— 如实留作残留缺口(见交付报告)。B 开关("把决定权还给用户",
// 辅线)daemon 侧尚无实现,ON 态无从钉起,现状即"默认关"态。
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

// TestDshUnknownReverseRequestLandsInUnknownRequestLedger 未知反向请求除了回
// -32601,必须以 UNKNOWN_REQUEST 进诊断台账:载荷脱敏、标注"未替用户做任何
// 审批决定" —— 事后排查时能看到"上游发来过什么、我们怎么处置的"。
func TestDshUnknownReverseRequestLandsInUnknownRequestLedger(t *testing.T) {
	const unseenMethod = "futureDept/neverSeenRequest" // 探针专用:真实上游不会发
	fake, done, free := runWithPendingApproval(t, nil)
	fake.reqFn(21, unseenMethod, json.RawMessage(`{"token":"secret-req","x":1}`))
	free()
	if err := <-done; err != nil {
		t.Fatalf("Run 应照常完成(未知请求只影响该次应答): %v", err)
	}

	// 回复侧:必须回 -32601 error,绝不带任何 decision 形状的成功应答。
	snap := fake.snapshot()
	if len(snap.responds) != 1 || !snap.responds[0].isError {
		t.Fatalf("未知反向请求应回 error: %+v", snap.responds)
	}
	if snap.responds[0].errCode != -32601 {
		t.Fatalf("应回 -32601: %+v", snap.responds[0])
	}

	// 台账侧:UNKNOWN_REQUEST + 脱敏 + 处置说明。
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "dsh"}, 0) {
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
