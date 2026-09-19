// codex_review_test.go —— "/计划"桥接(review/start)的回归测试(fake app-server,不拉子进程)。
//
// 【要钉死的东西】
//  1. 方法名与参数形状:严格按 2026-09-19 对 codex-cli 0.149.1 的 live 实测
//     (证据:cmd/codexreadprobe 输出):{threadId, target:{type:"custom",
//     instructions}},delivery 不传(缺省 inline,审查跑在原线程);
//  2. focus 语义:空串/纯空白 → DefaultReviewFocus;非空 trim 后原样下发;
//  3. 应答解析:reviewThreadId + turn.{id,status}(实测 status="inProgress",
//     发起即回);缺 reviewThreadId 视为认不出的形状,如实报错;
//  4. 线程未加载 → ErrReviewThreadNotLoaded(app-server 不自动 resume,
//     实测未加载的 id 会被 -32600 "thread not found" 拒绝);
//  5. 【防再犯】发出的键与 codex-schema/v2 的 ReviewStartParams 一致
//     (schema 改形状时单测先红,不再出现"fake 自洽但真机被拒")。
package agent

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/store"
)

// reviewStartLiveLike 与 live 实测同形的应答(ReviewStartResponse:
// required = reviewThreadId + turn;inline 模式 reviewThreadId = 原线程 id)。
const reviewStartLiveLike = `{"turn":{"id":"01a0b804-4292-7b11-b611-92d4cc1fa4d6",` +
	`"items":[{"type":"userMessage","id":"x","content":[{"type":"text","text":"审查重点"}]}],` +
	`"itemsView":"notLoaded","status":"inProgress","error":null,` +
	`"startedAt":null,"completedAt":null,"durationMs":null},` +
	`"reviewThreadId":"01a0b804-2320-7b22-8d66-bf8c72ab2094"}`

func newReviewFake() *fakeCodexCaller {
	return &fakeCodexCaller{replies: map[string]fakeReply{
		"review/start": {raw: json.RawMessage(reviewStartLiveLike)},
	}}
}

// ---- 1) 方法名 + 参数形状 ----

func TestCodexReviewStartMethodAndShape(t *testing.T) {
	fake := newReviewFake()
	out, err := codexReviewStartVia(nil, fake, "01a0b804-2320", "重点审风险")
	if err != nil {
		t.Fatalf("review/start 不该报错: %v", err)
	}
	call := fake.lastCall()
	if call.method != "review/start" {
		t.Fatalf("方法名 = %q, want review/start", call.method)
	}
	var params struct {
		ThreadId string `json:"threadId"`
		Target   struct {
			Type         string `json:"type"`
			Instructions string `json:"instructions"`
		} `json:"target"`
		Delivery interface{} `json:"delivery"`
	}
	if err := json.Unmarshal(call.params, &params); err != nil {
		t.Fatalf("params 不是对象: %s", call.params)
	}
	if params.ThreadId != "01a0b804-2320" {
		t.Fatalf("threadId 必传: %s", call.params)
	}
	if params.Target.Type != "custom" {
		t.Fatalf("计划审查的 target 必须是 custom(自由指令), got %q", params.Target.Type)
	}
	if params.Target.Instructions != "重点审风险" {
		t.Fatalf("focus 应原样进 instructions: %q", params.Target.Instructions)
	}
	if params.Delivery != nil {
		t.Fatalf("delivery 不传(缺省 inline,结果流回原会话): %s", call.params)
	}
	// 应答归一:实测字段全部到位
	if out.ReviewThreadID != "01a0b804-2320-7b22-8d66-bf8c72ab2094" {
		t.Fatalf("ReviewThreadID = %q", out.ReviewThreadID)
	}
	if out.TurnID != "01a0b804-4292-7b11-b611-92d4cc1fa4d6" || out.TurnStatus != "inProgress" {
		t.Fatalf("Turn 解析错: %q %q", out.TurnID, out.TurnStatus)
	}
}

// ---- 2) focus 为空 → 默认口径 ----

func TestCodexReviewStartEmptyFocusUsesDefault(t *testing.T) {
	for _, focus := range []string{"", "   "} {
		fake := newReviewFake()
		if _, err := codexReviewStartVia(nil, fake, "t1", focus); err != nil {
			t.Fatalf("focus=%q 不该报错: %v", focus, err)
		}
		var params map[string]interface{}
		if err := json.Unmarshal(fake.lastCall().params, &params); err != nil {
			t.Fatalf("params 不是对象: %v", fake.lastCall().params)
		}
		target := params["target"].(map[string]interface{})
		if target["instructions"] != DefaultReviewFocus {
			t.Fatalf("focus=%q 应使用默认口径: got %q", focus, target["instructions"])
		}
	}
	// 前后空白 trim,不把空格当"审查重点"下发
	fake := newReviewFake()
	_, _ = codexReviewStartVia(nil, fake, "t1", "  审风险  ")
	var params map[string]interface{}
	_ = json.Unmarshal(fake.lastCall().params, &params)
	if got := params["target"].(map[string]interface{})["instructions"]; got != "审风险" {
		t.Fatalf("focus 应 trim 后下发: %q", got)
	}
}

// ---- 3) 线程未加载 / 坏应答的结构化错误 ----

func TestCodexReviewStartRequiresLoadedThread(t *testing.T) {
	fake := newReviewFake()
	_, err := codexReviewStartVia(nil, fake, "", "x")
	if !errors.Is(err, ErrReviewThreadNotLoaded) {
		t.Fatalf("空 codexTID 应返回 ErrReviewThreadNotLoaded, got %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatal("未加载时绝不该发出请求(app-server 不自动 resume,发了也是 thread not found)")
	}
}

func TestCodexReviewStartBadResponse(t *testing.T) {
	// 缺 reviewThreadId:认不出的形状,如实报错
	fake := &fakeCodexCaller{replies: map[string]fakeReply{
		"review/start": {raw: json.RawMessage(`{"turn":{"id":"x","status":"inProgress"}}`)},
	}}
	if _, err := codexReviewStartVia(nil, fake, "t1", ""); err == nil || !strings.Contains(err.Error(), "reviewThreadId") {
		t.Fatalf("缺 reviewThreadId 应报错: %v", err)
	}
	// 非 JSON:报错不 panic
	fake2 := &fakeCodexCaller{replies: map[string]fakeReply{
		"review/start": {raw: json.RawMessage(`not-json`)},
	}}
	if _, err := codexReviewStartVia(nil, fake2, "t1", ""); err == nil {
		t.Fatal("坏 JSON 应报错")
	}
	// caller 侧错误(codex -32600)原样上抛
	fake3 := &fakeCodexCaller{replies: map[string]fakeReply{
		"review/start": {err: errors.New("codex 返回错误 -32600: thread not found: t1")},
	}}
	_, err := codexReviewStartVia(nil, fake3, "t1", "")
	if err == nil || !strings.Contains(err.Error(), "thread not found") {
		t.Fatalf("codex 错误应带原因上抛: %v", err)
	}
}

// ---- 4) provider 层:线程未加载直接结构化报错(不碰子进程)----

func TestStartReviewNotLoadedProvider(t *testing.T) {
	p := NewCodexProvider()
	_, err := p.StartReview(nil, "")
	if err == nil || !strings.Contains(err.Error(), "thread 为空") {
		t.Fatalf("nil thread 应报错: %v", err)
	}
	// 新 provider 的 threadMap 为空 = 线程未加载;必须在不拉子进程的前提下报错
	_, err = p.StartReview(&store.Thread{ID: "th-none"}, "审查计划")
	if !errors.Is(err, ErrReviewThreadNotLoaded) {
		t.Fatalf("未加载线程应返回 ErrReviewThreadNotLoaded, got %v", err)
	}
}

// ---- 5) schema 对照测试:发出的键必须与 codex-schema/v2 一致 ----

func TestCodexReviewStartShapeMatchesSchema(t *testing.T) {
	schema := loadGoalSchema(t, "ReviewStartParams.json")

	// schema 事实:required = threadId + target;target 走 definitions.ReviewTarget
	if !containsStr(schema.Required, "threadId") || !containsStr(schema.Required, "target") {
		t.Fatalf("ReviewStartParams.required 应含 threadId/target: %v", schema.Required)
	}
	if _, ok := schema.Definitions["ReviewTarget"]; !ok {
		t.Fatal("schema 缺 ReviewTarget 定义 —— 结构变了,须重新 live 实测后再改解析代码")
	}

	// 我们发出的键 ⊆ schema properties;required ⊆ 我们发出的键
	fake := newReviewFake()
	if _, err := codexReviewStartVia(nil, fake, "t1", "审风险"); err != nil {
		t.Fatalf("review/start 不该报错: %v", err)
	}
	var emitted map[string]interface{}
	if err := json.Unmarshal(fake.lastCall().params, &emitted); err != nil {
		t.Fatalf("params 不是对象: %v", fake.lastCall().params)
	}
	for k := range emitted {
		if _, ok := schema.Properties[k]; !ok {
			t.Fatalf("review/start 发出的键 %q 不在 schema properties 里(即真机会拒): %v", k, emitted)
		}
	}
	for _, r := range schema.Required {
		if _, ok := emitted[r]; !ok {
			t.Fatalf("schema required 的键 %q 未随 review/start 发出: %v", r, emitted)
		}
	}
	// target 内层:键 ⊆ custom 变体的 properties(instructions + type)
	target, ok := emitted["target"].(map[string]interface{})
	if !ok {
		t.Fatalf("target 应是对象: %v", emitted)
	}
	typ, _ := target["type"].(string)
	if _, ok := target["instructions"]; !ok || typ != "custom" {
		t.Fatalf("target 必须是 {type:custom, instructions:…}: %v", target)
	}
}
