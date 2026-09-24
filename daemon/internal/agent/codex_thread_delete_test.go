// codex_thread_delete_test.go —— thread/delete 真删同步的 agent 侧单测
// (fake codex app-server,基建同 codex_effect_test.go)。
//
// 【钉什么】DeleteCodexThread 必须把 `thread/delete` 请求**真的发给** codex,
// 且参数形状是协议要求的 {"threadId": ...}(0.156.1 运行时实测仍支持该方法,
// docs/verify/evidence/codex-0156-probe.log 第四节;消失的是 thread/rollback,
// 与本路径无关)。空 id 守卫一并钉住。
package agent

import (
	"strings"
	"testing"
)

func TestDeleteCodexThreadIssuesProtocolCall(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)

	if err := p.DeleteCodexThread("fake-thread-del-1"); err != nil {
		t.Fatalf("thread/delete 应成功(fake 默认空对象应答): %v", err)
	}

	var hit bool
	for _, r := range fakeCodexRequests(t, logPath) {
		if r.Method != "thread/delete" {
			continue
		}
		// 参数必须带 threadId 且值正确 —— 这就是"真删同步"的调用证据。
		if !strings.Contains(string(r.Params), `"threadId":"fake-thread-del-1"`) {
			t.Fatalf("thread/delete 参数形状变了, got=%s", r.Params)
		}
		hit = true
	}
	if !hit {
		t.Fatal("thread/delete 请求没有发出:fake 日志里没有该方法")
	}
}

func TestDeleteCodexThreadRejectsEmptyID(t *testing.T) {
	p, _ := newFakeCodexProvider(t, false)
	if err := p.DeleteCodexThread(""); err == nil {
		t.Fatal("空 codex 线程 id 应被守卫拒绝,而不是发出一个必然失败的调用")
	}
}
