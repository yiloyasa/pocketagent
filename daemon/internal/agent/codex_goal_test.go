// codex_goal_test.go —— 常驻目标下发(thread/goal/set|clear)与通知解析的回归测试。
//
// 【要钉死的东西】
//  1. set 请求的参数名是 **objective**(v2 schema ThreadGoalSetParams + 2026-09-19
//     对 codex-cli 0.149.1 的 live 实测:传 {"goal":...} 被 -32600 "no goal exists"
//     拒绝,证据:cmd/codexreadprobe 输出)。这里曾断言过错误的 "goal" 形状 ——
//     单测自洽、真机被拒,教训就是第 4 条的 schema 对照测试;
//  2. clear 请求只带 threadId(ThreadGoalClearParams);
//  3. thread/goal/updated 通知:外层包裹键是 goal,内层文本字段是 objective
//     (同 get 应答的 ThreadGoal 形状);
//  4. 【防再犯】直接读 codex-schema/v2 的 JSON Schema 文件,断言我们发出的键
//     与 schema 的 properties/required 一致 —— schema 改字段名时单测先红,
//     不再出现"fake 自洽但真机被拒"。
package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ---- 1) thread/goal/set:参数名必须是 objective ----

func TestCodexGoalSetSendsObjectiveField(t *testing.T) {
	// 应答形状取 live 实测原文(ThreadGoalSetResponse:外层 goal 包裹一条 ThreadGoal)
	fake := &fakeCodexCaller{replies: map[string]fakeReply{
		"thread/goal/set": {raw: json.RawMessage(goalGetLiveLike)},
	}}
	if err := pushGoalVia(fake, "01a0b689", "把 README 补完"); err != nil {
		t.Fatalf("set 不该报错: %v", err)
	}
	call := fake.lastCall()
	if call.method != "thread/goal/set" {
		t.Fatalf("方法名 = %q, want thread/goal/set", call.method)
	}
	var params map[string]interface{}
	if err := json.Unmarshal(call.params, &params); err != nil {
		t.Fatalf("params 不是对象: %v", call.params)
	}
	// 【历史缺陷】参数名曾是 "goal",codex 0.149.1 直接 -32600 拒收
	if params["objective"] != "把 README 补完" {
		t.Fatalf("set 必须用 objective 传目标文本: %v", params)
	}
	if _, ok := params["goal"]; ok {
		t.Fatalf("set 参数里绝不能出现 goal 键(真机会拒): %v", params)
	}
	if params["threadId"] != "01a0b689" {
		t.Fatalf("threadId 必传: %v", params)
	}
}

// ---- 2) thread/goal/clear:只带 threadId ----

func TestCodexGoalClearSendsOnlyThreadId(t *testing.T) {
	// 应答形状按 ThreadGoalClearResponse(required: cleared)
	fake := &fakeCodexCaller{replies: map[string]fakeReply{
		"thread/goal/clear": {raw: json.RawMessage(`{"cleared":true}`)},
	}}
	if err := pushGoalVia(fake, "01a0b689", "  "); err != nil {
		t.Fatalf("clear 不该报错: %v", err)
	}
	call := fake.lastCall()
	if call.method != "thread/goal/clear" {
		t.Fatalf("方法名 = %q, want thread/goal/clear", call.method)
	}
	if string(call.params) != `{"threadId":"01a0b689"}` {
		t.Fatalf("clear 参数应只有 threadId, got %s", call.params)
	}
}

// ---- 3) thread/goal/updated 通知解析:内层文本字段是 objective ----

func TestGoalStatusFromNotificationReadsObjective(t *testing.T) {
	// 形状与 v2 schema ThreadGoalUpdatedNotification / live 应答一致
	params := json.RawMessage(`{"threadId":"01a0b689","turnId":null,` +
		`"goal":{"threadId":"01a0b689","objective":"把 README 补完","status":"active",` +
		`"tokenBudget":null,"tokensUsed":7,"timeUsedSeconds":3,"createdAt":1,"updatedAt":2}}`)
	goal, status, tokens, secs := goalStatusFromNotification(params)
	// 【历史缺陷】内层曾解析 "goal" 键,永远拿到空串
	if goal != "把 README 补完" {
		t.Fatalf("通知里的目标文本应从 objective 解析, got %q", goal)
	}
	if status != "active" || tokens != 7 || secs != 3 {
		t.Fatalf("状态/用量解析错: %q %d %d", status, tokens, secs)
	}
	// cleared 通知只有 threadId:解析出零值,goalText 由调用方按方法名置空
	if g, s, tk, ts := goalStatusFromNotification(json.RawMessage(`{"threadId":"t1"}`)); g != "" || s != "" || tk != 0 || ts != 0 {
		t.Fatalf("cleared 通知应解析出零值: %q %q %d %d", g, s, tk, ts)
	}
	// 非法 JSON:零值不 panic
	if g, _, _, _ := goalStatusFromNotification(json.RawMessage(`not-json`)); g != "" {
		t.Fatalf("非法参数应返回零值, got %q", g)
	}
}

// ---- 4) schema 对照测试:发出的 JSON 键必须与 codex-schema/v2 一致 ----

// schemaShape 是本测试需要的 JSON Schema(draft-07)最小切片。
type schemaShape struct {
	Properties  map[string]json.RawMessage `json:"properties"`
	Required    []string                   `json:"required"`
	Definitions map[string]struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	} `json:"definitions"`
}

// loadGoalSchema 从仓库的 codex-schema/v2 读一份 schema 文件。
// 路径以本测试文件为锚(daemon/internal/agent → 仓库根/codex-schema/v2),
// 不依赖 go test 的工作目录。
func loadGoalSchema(t *testing.T, name string) schemaShape {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("拿不到测试文件路径")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "codex-schema", "v2", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 schema 失败(%s): %v —— schema 对照测试依赖仓库内 codex-schema", path, err)
	}
	var s schemaShape
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("schema 解析失败(%s): %v", path, err)
	}
	return s
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestCodexGoalSetShapeMatchesSchema(t *testing.T) {
	schema := loadGoalSchema(t, "ThreadGoalSetParams.json")

	// schema 事实:properties 里有 objective,没有 goal;required 至少 threadId
	if _, ok := schema.Properties["objective"]; !ok {
		t.Fatal("ThreadGoalSetParams.properties 应含 objective —— schema 变了,先核对 live 再改代码与测试")
	}
	if _, ok := schema.Properties["goal"]; ok {
		t.Fatal("ThreadGoalSetParams.properties 不该有 goal —— 若 schema 新增了 goal,须重新 live 实测后再放行")
	}

	// 我们发出的键 ⊆ schema properties;schema required ⊆ 我们发出的键
	fake := &fakeCodexCaller{replies: map[string]fakeReply{
		"thread/goal/set": {raw: json.RawMessage(goalGetLiveLike)},
	}}
	if err := pushGoalVia(fake, "t1", "写一份周报"); err != nil {
		t.Fatalf("set 不该报错: %v", err)
	}
	var emitted map[string]interface{}
	if err := json.Unmarshal(fake.lastCall().params, &emitted); err != nil {
		t.Fatalf("params 不是对象: %v", fake.lastCall().params)
	}
	for k := range emitted {
		if _, ok := schema.Properties[k]; !ok {
			t.Fatalf("set 发出的键 %q 不在 schema properties 里(即真机会拒): %v", k, emitted)
		}
	}
	for _, r := range schema.Required {
		if _, ok := emitted[r]; !ok {
			t.Fatalf("schema required 的键 %q 未随 set 发出: %v", r, emitted)
		}
	}
}

func TestGoalNotificationShapeMatchesSchema(t *testing.T) {
	schema := loadGoalSchema(t, "ThreadGoalUpdatedNotification.json")

	// 外层:required 必含包裹键 goal 与 threadId
	if !containsStr(schema.Required, "goal") || !containsStr(schema.Required, "threadId") {
		t.Fatalf("ThreadGoalUpdatedNotification.required 应含 goal/threadId: %v", schema.Required)
	}
	// 内层 ThreadGoal 定义:文本字段是 objective(必填),没有 goal 字段
	tg, ok := schema.Definitions["ThreadGoal"]
	if !ok {
		t.Fatal("schema 缺 ThreadGoal 定义 —— 结构变了,须重新核对解析代码")
	}
	if !containsStr(tg.Required, "objective") {
		t.Fatalf("ThreadGoal.required 应含 objective: %v", tg.Required)
	}
	if _, ok := tg.Properties["goal"]; ok {
		t.Fatal("ThreadGoal.properties 不该有 goal 键")
	}

	// 代码 tie-back:严格按 schema 的 required 造一条通知,解析必须拿到 objective ——
	// 若解析方向退回内层 "goal",这里立刻红(与 live 无关的可执行契约)
	params := map[string]interface{}{
		"threadId": "t1",
		"goal": map[string]interface{}{
			"threadId":        "t1",
			"objective":       "schema 造的目标",
			"status":          "active",
			"tokensUsed":      1,
			"timeUsedSeconds": 2,
			"createdAt":       3,
			"updatedAt":       4,
		},
	}
	raw, _ := json.Marshal(params)
	goal, status, _, _ := goalStatusFromNotification(raw)
	if goal != "schema 造的目标" || status != "active" {
		t.Fatalf("按 schema 形状造的通知应解析出 objective: goal=%q status=%q", goal, status)
	}
}
