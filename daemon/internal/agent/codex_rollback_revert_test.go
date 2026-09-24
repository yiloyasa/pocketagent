// codex_rollback_revert_test.go —— RollbackThread 迁移 thread/revert 的回归钉。
//
// 【背景】codex 0.156.1 删除了 thread/rollback {threadId,numTurns}(探针实证,
// docs/verify/evidence/codex-0156-probe.log),替代口子是 thread/revert
// {threadId,beforeTurnId}。provider 对外语义保持"回退 N 个回合":内部先
// thread/read 拉回合序列,取倒数第 N 个回合的 id 作锚点再 revert。
//
// 【fake 怎么接】同 codex_effect_test.go 的 TestHelperProcess 惯用法;
// PA_FAKE_CODEX_THREAD_READ 注入 thread/read 应答(带 turns[].id),
// PA_FAKE_CODEX_REVERT_ERROR 注入 revert 的 error 帧。
package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

// revertFixture 三个回合的 thread/read 应答(升序:turn-1 最旧)。
const revertFixture = `{"thread":{"id":"fake-rollback","turns":[
	{"id":"turn-1","status":"completed","items":[]},
	{"id":"turn-2","status":"completed","items":[]},
	{"id":"turn-3","status":"completed","items":[]}]}}`

// 请求形状 + 锚点计算:回退 2 个回合,锚点必须是倒数第 2 个回合(turn-2),
// 且旧口子 thread/rollback 不得再出现。
func TestRollbackThreadRevertsBeforeAnchorTurn(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	t.Setenv("PA_FAKE_CODEX_THREAD_READ", revertFixture)

	if err := p.RollbackThread("fake-rollback", 2); err != nil {
		t.Fatalf("revert 路径应成功: %v", err)
	}
	var revert map[string]interface{}
	rollbackSeen := false
	for _, r := range fakeCodexRequests(t, logPath) {
		switch r.Method {
		case "thread/rollback":
			rollbackSeen = true // 0.156.1 已删该方法,发出去必然报错
		case "thread/revert":
			if revert != nil {
				t.Fatal("thread/revert 应恰好发出一次")
			}
			if err := json.Unmarshal(r.Params, &revert); err != nil {
				t.Fatalf("revert params 解析: %v", err)
			}
		}
	}
	if rollbackSeen {
		t.Fatal("仍在对 0.156.1 发已删除的 thread/rollback")
	}
	if revert == nil {
		t.Fatal("thread/revert 请求没有发出")
	}
	if revert["threadId"] != "fake-rollback" {
		t.Fatalf("revert 参数形状变了(缺 threadId 或值不对): %v", revert)
	}
	if revert["beforeTurnId"] != "turn-2" {
		t.Fatalf("锚点应为倒数第 2 个回合 turn-2,得到 %v", revert["beforeTurnId"])
	}
	// 前置读必须真的发生:没有 thread/read 就没有锚点来源。
	readSeen := false
	for _, r := range fakeCodexRequests(t, logPath) {
		if r.Method == "thread/read" {
			readSeen = true
		}
	}
	if !readSeen {
		t.Fatal("thread/read 没有发出:锚点定位的历史读取缺失")
	}
}

// N == 历史长度:清空全部回合仍放行(锚点 = 最旧回合)。
func TestRollbackThreadAllowsFullHistoryRollback(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	t.Setenv("PA_FAKE_CODEX_THREAD_READ", revertFixture)

	if err := p.RollbackThread("fake-rollback", 3); err != nil {
		t.Fatalf("N==历史长度应放行: %v", err)
	}
	revert := map[string]interface{}{}
	for _, r := range fakeCodexRequests(t, logPath) {
		if r.Method == "thread/revert" {
			_ = json.Unmarshal(r.Params, &revert)
		}
	}
	if revert["beforeTurnId"] != "turn-1" {
		t.Fatalf("全量回滚锚点应为最旧回合 turn-1,得到 %v", revert["beforeTurnId"])
	}
}

// N 超过历史长度(缺省 fake 回空 thread/read = 0 回合):显式报错、绝不发 revert
// —— 回滚是破坏性操作,宁可让用户看到"退不动了",不冒多删的险。
func TestRollbackThreadRejectsWhenTurnsInsufficient(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)

	err := p.RollbackThread("fake-rollback", 1)
	if err == nil {
		t.Fatal("0 回合的线程回退 1 个回合应报错")
	}
	if !strings.Contains(err.Error(), "超过会话已有回合数") {
		t.Fatalf("错误应说明回合数不足: %v", err)
	}
	for _, r := range fakeCodexRequests(t, logPath) {
		if r.Method == "thread/revert" {
			t.Fatal("回合数不足时不得发出 thread/revert")
		}
	}
}

// revert 报错必须如实上抛(fake 回 error 帧),不能被"读了历史"这半程吞成成功。
func TestRollbackThreadSurfacesRevertError(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	t.Setenv("PA_FAKE_CODEX_THREAD_READ", revertFixture)
	t.Setenv("PA_FAKE_CODEX_REVERT_ERROR", "native-revert-refused")

	err := p.RollbackThread("fake-rollback", 1)
	if err == nil || !strings.Contains(err.Error(), "native-revert-refused") {
		t.Fatalf("revert 的错误应原样上抛: %v", err)
	}
	if seen := len(fakeCodexRequests(t, logPath)); seen == 0 {
		t.Fatal("请求日志为空,fake 接线断了")
	}
}

// thread/read 给不出锚点回合的 id(老条目可能缺 id):拒绝盲改历史。
func TestRollbackThreadRejectsAnchorWithoutTurnID(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	t.Setenv("PA_FAKE_CODEX_THREAD_READ",
		`{"thread":{"id":"fake-rollback","turns":[{"id":"","status":"completed","items":[]}]}}`)

	err := p.RollbackThread("fake-rollback", 1)
	if err == nil || !strings.Contains(err.Error(), "turnId") {
		t.Fatalf("锚点缺 id 应显式报错: %v", err)
	}
	for _, r := range fakeCodexRequests(t, logPath) {
		if r.Method == "thread/revert" {
			t.Fatal("锚点缺 id 时不得发出 thread/revert")
		}
	}
}
