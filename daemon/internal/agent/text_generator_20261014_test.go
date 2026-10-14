// text_generator_20261014_test.go —— P1 补位:hermes / dsh 的 TextGenerator 接线契约
// (假客户端注入,不起真进程)。与 codex/pi 的既有 GenerateText 用例同口径,重点钉
// 本轮新增的四件事:
//
//  1. 真能生成:共享适配器跑一轮、终稿正文回给优化链路(不是空壳);
//  2. 零桌面配置写入:hermes 的 effort 入参被显式忽略(唯一强度入口是家目录
//     config.yaml),dsh 的 effort 走会话内 set_config_option(不持久);
//  3. denyGenerationTool 真挂上:优化轮里的审批请求一律拒答(红线 1,绝不代答
//     也绝不放行),断言应答里选中的是 deny 侧;
//  4. 收尾不留残:threadMap 的 ephemeral 引用清掉,优化轮会话(dsh)session/close 真删。
package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/dshapp"
)

// approvalSelectedOptionID 从审批应答 result 里取选中的 optionId
// (形状 {"outcome":{"outcome":"selected","optionId":…}},ACP 0.9.0)。
func approvalSelectedOptionID(t *testing.T, result any) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("应答体序列化失败: %v", err)
	}
	var outer map[string]any
	if err := json.Unmarshal(raw, &outer); err != nil {
		t.Fatalf("应答体不是对象: %s (%v)", raw, err)
	}
	inner, _ := outer["outcome"].(map[string]any)
	id, _ := inner["optionId"].(string)
	return id
}

// hermesThreadRefs / dshThreadRefs 并发安全读内存引用条数(一次性轮次泄漏钉)。
func hermesThreadRefs(p *HermesProvider) int {
	p.threadMu.Lock()
	defer p.threadMu.Unlock()
	return len(p.threadMap)
}

func dshThreadRefs(p *DshProvider) int {
	p.threadMu.Lock()
	defer p.threadMu.Unlock()
	return len(p.threadMap)
}

// ---- hermes ----

// TestHermesGenerateTextReturnsReplyAndCleansUp 真能生成 + 三处收尾:
// 家目录 config.yaml 零写入(effort 被忽略)、threadMap 无残留、当轮钩子已卸。
func TestHermesGenerateTextReturnsReplyAndCleansUp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", home) // SetReasoningEffort 的写入面(沙箱)
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)

	text, err := p.GenerateText(t.Context(), "hermes/deepseek-flash", "high", "把这句话改写成清晰指令")
	if err != nil {
		t.Fatalf("hermes 生成应成功: %v", err)
	}
	if !strings.Contains(text, "正文片段") {
		t.Fatalf("应回终稿正文,得到 %q", text)
	}
	if fake.promptCalls != 1 || !strings.Contains(fake.promptTexts[0], "把这句话改写成清晰指令") {
		t.Fatalf("应下发一次优化指令: calls=%d texts=%v", fake.promptCalls, fake.promptTexts)
	}

	// 零配置写入(红线 2):effort 入参显式忽略,家目录不许出现强度键。
	raw, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err == nil && strings.Contains(string(raw), "reasoning_effort") {
		t.Fatalf("一次性优化轮不许写用户 config.yaml:\n%s", raw)
	}

	if n := hermesThreadRefs(p); n != 0 {
		t.Fatalf("ephemeral 线程引用必须清掉,泄漏 %d 条", n)
	}
	if notif, req := hermesHandlersOf(fake); notif != nil || req != nil {
		t.Fatal("优化轮收场后当轮钩子必须卸载")
	}
}

// TestHermesGenerateTextDeniesToolCalls 优化轮的审批请求必须拒答:
// approve 钩子 = denyGenerationTool(不是 nil —— nil 会回 -32000 通道错误),
// 应答选中 deny 侧,绝不放行(红线 1)。
func TestHermesGenerateTextDeniesToolCalls(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)

	if _, err := p.GenerateText(t.Context(), "hermes/default", "", "need-approval 让它执行 mkdir"); err != nil {
		t.Fatalf("优化轮应正常收尾: %v", err)
	}
	if len(fake.responds) != 1 {
		t.Fatalf("审批请求应被应答一次,实际 %d 次(%+v)", len(fake.responds), fake.responds)
	}
	if got := approvalSelectedOptionID(t, fake.responds[0].Result); got != "deny" {
		t.Fatalf("denyGenerationTool 应选中 deny 侧,得到 %q", got)
	}
	if len(fake.respondErr) != 0 {
		t.Fatalf("钩子在场时不该走 -32000/-32601 错误通道: %+v", fake.respondErr)
	}
}

// ---- dsh ----

// dshPromptFixture 造一个"回一段正文"的 dsh 假客户端(优化轮夹具)。
func dshPromptFixture(t *testing.T, reply string) *fakeDshClient {
	t.Helper()
	return &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-opt"},
		promptFn: func(_ context.Context, sid string, _ []map[string]interface{}, notify func(string, json.RawMessage)) (json.RawMessage, error) {
			env, _ := json.Marshal(map[string]interface{}{
				"sessionId": sid,
				"update": map[string]interface{}{
					"sessionUpdate": "agent_message_chunk",
					"content":       map[string]interface{}{"type": "text", "text": reply},
				},
			})
			notify("session/update", env)
			return json.RawMessage(`{"stopReason":"end_turn"}`), nil
		},
	}
}

// TestDshGenerateTextReturnsReplyAndClosesSession 真能生成 + 会话内带 effort
// (set_config_option,不持久)+ 优化轮会话 session/close 真删 + threadMap 无残留。
func TestDshGenerateTextReturnsReplyAndClosesSession(t *testing.T) {
	fake := dshPromptFixture(t, "改写后的指令")
	p := newTestDshProvider(t, fake)

	text, err := p.GenerateText(t.Context(), "dsh/default", "low", "把这句话改写成清晰指令")
	if err != nil {
		t.Fatalf("dsh 生成应成功: %v", err)
	}
	if text != "改写后的指令" {
		t.Fatalf("应回终稿正文,得到 %q", text)
	}

	snap := fake.snapshot()
	if len(snap.closeHits) != 1 || snap.closeHits[0] != "s-opt" {
		t.Fatalf("优化轮会话必须 session/close 真删(不给 ~/.dsh 留一次性会话): %+v", snap.closeHits)
	}
	var gotEffort bool
	for _, c := range snap.setCalls {
		if c.optionID == "reasoning_effort" && c.value == "low" {
			gotEffort = true
		}
	}
	if !gotEffort {
		t.Fatalf("effort 应经会话内 set_config_option 下发(不持久): %+v", snap.setCalls)
	}
	if n := dshThreadRefs(p); n != 0 {
		t.Fatalf("ephemeral 线程引用必须清掉,泄漏 %d 条", n)
	}
}

// TestDshGenerateTextDeniesToolCalls 优化轮的审批请求必须拒答(与 hermes 同一条
// 红线):approve 钩子 = denyGenerationTool,应答选中 deny 侧。
func TestDshGenerateTextDeniesToolCalls(t *testing.T) {
	fake := dshPromptFixture(t, "改写后的指令")
	fake.promptFn = func(_ context.Context, sid string, _ []map[string]interface{}, notify func(string, json.RawMessage)) (json.RawMessage, error) {
		env, _ := json.Marshal(map[string]interface{}{
			"sessionId": sid,
			"update": map[string]interface{}{
				"sessionUpdate": "agent_message_chunk",
				"content":       map[string]interface{}{"type": "text", "text": "改写后的指令"},
			},
		})
		notify("session/update", env)
		fake.mu.Lock()
		req := fake.reqFn
		fake.mu.Unlock()
		if req != nil {
			req(42, "session/request_permission", json.RawMessage(permParams))
		}
		return json.RawMessage(`{"stopReason":"end_turn"}`), nil
	}
	p := newTestDshProvider(t, fake)

	if _, err := p.GenerateText(t.Context(), "dsh/default", "", "让它执行 mkdir"); err != nil {
		t.Fatalf("优化轮应正常收尾: %v", err)
	}
	snap := fake.snapshot()
	if len(snap.responds) != 1 {
		t.Fatalf("审批请求应被应答一次,实际 %d 次(%+v)", len(snap.responds), snap.responds)
	}
	// dsh 夹具 permParams 的拒绝侧 optionId 是 reject-once(allow 侧是 allow-once)。
	if got := approvalSelectedOptionID(t, snap.responds[0].result); got != "reject-once" {
		t.Fatalf("denyGenerationTool 应选中拒绝侧(reject-once),得到 %q", got)
	}
	if snap.responds[0].isError {
		t.Fatalf("钩子在场时不该走 error 通道: %+v", snap.responds[0])
	}
}

// TestGenerateTextEmptyPromptReported 空指令在 provider 层如实报错,绝不返回空
// 成功 —— 上层 OptimizePrompt 拿到空串会拿空覆盖用户已打好的输入(它的空返回
// 即失败纪律),这里不许先把错误吞成空串。
func TestGenerateTextEmptyPromptReported(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)
	if _, err := p.GenerateText(context.Background(), "hermes/default", "", "   "); err == nil {
		t.Fatal("空指令应回错误,不许空成功")
	}
	if fake.promptCalls != 0 {
		t.Fatalf("空输入不该下发轮次: prompt=%d", fake.promptCalls)
	}
}
