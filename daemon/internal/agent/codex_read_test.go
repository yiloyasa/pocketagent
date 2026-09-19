// codex_read_test.go —— 四个只读口子桥接的回归测试(fake app-server,不拉子进程)。
//
// 【要钉死的东西】
//  1. 方法名与参数形状:严格按 2026-09-19 对 codex-cli 0.149.1 的 live 实测
//     (证据:cmd/codexreadprobe 输出,应答样例即实测原文);
//  2. 应答解析:config/read 的 null 字段与 granular 审批策略、goal=null 的
//     "无目标不是错误"、capabilities 三 bool、readiness 三态与未知状态透传;
//  3. wrapper 不该吞错:withStatusClient 失败时错误如实上抛。
//
// 【为什么不直接调 wrapper】wrapper 走 withStatusClient → 真实子进程生命周期,
// codexapp.Client 的字段无法从本包构造;桥接核心抽成 codexXxxVia(ctx, caller),
// 生产路径由 *codexapp.Client 满足接口,fake 在这里顶上 —— 方法名/参数/解析全覆盖。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// fakeCodexCaller 记录每次调用的方法名与参数,按方法名回放预制应答。
type fakeCodexCaller struct {
	calls   []fakeCodexCall
	replies map[string]fakeReply // method → 应答
}

type fakeCodexCall struct {
	method string
	params json.RawMessage
}

type fakeReply struct {
	raw json.RawMessage
	err error
}

func (f *fakeCodexCaller) CallCtx(_ context.Context, method string, params interface{}) (json.RawMessage, error) {
	raw, _ := json.Marshal(params)
	f.calls = append(f.calls, fakeCodexCall{method: method, params: raw})
	if r, ok := f.replies[method]; ok {
		return r.raw, r.err
	}
	return nil, errors.New("fake 未配置该方法的应答: " + method)
}

func (f *fakeCodexCaller) lastCall() fakeCodexCall { return f.calls[len(f.calls)-1] }

// ---- 1) config/read ----

// 与 live 实测同形的应答(null 字段保留,desktop 大对象截断示意)。
const configReadLiveLike = `{"config":{"model":"dexacct.MThk.Z3B0LTUuNi1zb2w","review_model":null,` +
	`"model_context_window":272000,"model_provider":"dex_ai","approval_policy":null,` +
	`"sandbox_mode":"workspace-write","model_reasoning_effort":"high",` +
	`"model_reasoning_summary":"concise","model_verbosity":"medium"},` +
	`"layers":[{"name":"user","file":"C:\\Users\\u\\.codex\\config.toml","version":"v1"}],` +
	`"origins":{"model":{"name":{"type":"user"},"version":"v1"}}}`

func TestCodexConfigReadMethodAndParse(t *testing.T) {
	fake := &fakeCodexCaller{replies: map[string]fakeReply{
		"config/read": {raw: json.RawMessage(configReadLiveLike)},
	}}
	got, err := codexConfigReadVia(context.Background(), fake, `G:\w`, true)
	if err != nil {
		t.Fatalf("config/read 不该报错: %v", err)
	}
	// 方法名与参数:includeLayers 必须透传,cwd 非空必须带上
	call := fake.lastCall()
	if call.method != "config/read" {
		t.Fatalf("方法名 = %q, want config/read", call.method)
	}
	var params map[string]interface{}
	if err := json.Unmarshal(call.params, &params); err != nil {
		t.Fatalf("params 不是对象: %v", call.params)
	}
	if params["includeLayers"] != true {
		t.Fatalf("includeLayers 应为 true: %v", params)
	}
	if params["cwd"] != `G:\w` {
		t.Fatalf("cwd 应透传: %v", params)
	}
	// 解析:null 一律落成零值,而不是 "null" 字符串
	if got.Model != "dexacct.MThk.Z3B0LTUuNi1zb2w" || got.ModelProvider != "dex_ai" {
		t.Fatalf("model/provider 解析错: %q %q", got.Model, got.ModelProvider)
	}
	if got.Effort != "high" || got.Summary != "concise" || got.Verbosity != "medium" {
		t.Fatalf("effort/summary/verbosity 解析错: %q %q %q", got.Effort, got.Summary, got.Verbosity)
	}
	if got.SandboxMode != "workspace-write" || got.ContextWindow != 272000 {
		t.Fatalf("sandbox/window 解析错: %q %d", got.SandboxMode, got.ContextWindow)
	}
	if got.ApprovalPolicy != "" {
		t.Fatalf("approval_policy=null 应归一为空串, got %q", got.ApprovalPolicy)
	}
	if len(got.Layers) != 1 || got.Layers[0].Name != "user" || got.Layers[0].File == "" {
		t.Fatalf("layers 未解析: %+v", got.Layers)
	}
	if got.Raw == "" {
		t.Fatalf("Raw 应保留原始应答供诊断")
	}
}

func TestCodexConfigReadGranularApprovalAndOptionalParams(t *testing.T) {
	fake := &fakeCodexCaller{replies: map[string]fakeReply{
		"config/read": {raw: json.RawMessage(
			`{"config":{"approval_policy":{"granular":{"rules":true,"sandbox_approval":true,"mcp_elicitations":false}}}}`)},
	}}
	got, err := codexConfigReadVia(context.Background(), fake, "", false)
	if err != nil {
		t.Fatalf("不该报错: %v", err)
	}
	if got.ApprovalPolicy != "granular" {
		t.Fatalf("granular 形状应归一为 \"granular\", got %q", got.ApprovalPolicy)
	}
	// cwd 为空且 includeLayers=false:两个参数都不该出现(而不是传 null/false)
	var params map[string]interface{}
	_ = json.Unmarshal(fake.lastCall().params, &params)
	if _, ok := params["cwd"]; ok {
		t.Fatalf("cwd 为空不应出现在参数里: %v", params)
	}
	if _, ok := params["includeLayers"]; ok {
		t.Fatalf("includeLayers=false 不应出现在参数里: %v", params)
	}
}

func TestCodexConfigReadErrorPropagates(t *testing.T) {
	fake := &fakeCodexCaller{replies: map[string]fakeReply{
		"config/read": {err: errors.New("codex 返回错误 -32601: not supported")},
	}}
	if _, err := codexConfigReadVia(context.Background(), fake, "", false); err == nil {
		t.Fatal("错误必须如实上抛,不能伪装成空配置")
	}
}

// ---- 2) thread/goal/get ----

// live 实测(已设目标)原文。
const goalGetLiveLike = `{"goal":{"threadId":"01a0b689","objective":"探针临时目标","status":"active",` +
	`"tokenBudget":null,"tokensUsed":0,"timeUsedSeconds":0,"createdAt":1789768876,"updatedAt":1789768876}}`

func TestCodexGoalGetMethodAndParse(t *testing.T) {
	fake := &fakeCodexCaller{replies: map[string]fakeReply{
		"thread/goal/get": {raw: json.RawMessage(goalGetLiveLike)},
	}}
	got, err := codexGoalGetVia(context.Background(), fake, "01a0b689")
	if err != nil {
		t.Fatalf("不该报错: %v", err)
	}
	call := fake.lastCall()
	if call.method != "thread/goal/get" {
		t.Fatalf("方法名 = %q, want thread/goal/get", call.method)
	}
	var params map[string]interface{}
	_ = json.Unmarshal(call.params, &params)
	if params["threadId"] != "01a0b689" {
		t.Fatalf("threadId 必传: %v", params)
	}
	if got == nil {
		t.Fatal("goal 非空却解析成 nil")
	}
	// 字段名是 objective(schema/live 实证),不是 goal
	if got.Objective != "探针临时目标" || got.Status != "active" || got.ThreadID != "01a0b689" {
		t.Fatalf("goal 内容解析错: %+v", got)
	}
	if got.HasTokenBudget {
		t.Fatalf("tokenBudget=null 应记为未设: %+v", got)
	}
	if got.CreatedAtUnixS != 1789768876 || got.UpdatedAtUnixS != 1789768876 {
		t.Fatalf("时间戳解析错: %d %d", got.CreatedAtUnixS, got.UpdatedAtUnixS)
	}
}

func TestCodexGoalGetNilGoalIsNotError(t *testing.T) {
	fake := &fakeCodexCaller{replies: map[string]fakeReply{
		// live 实测(未设目标)原文就是 {"goal":null}
		"thread/goal/get": {raw: json.RawMessage(`{"goal":null}`)},
	}}
	got, err := codexGoalGetVia(context.Background(), fake, "t1")
	if err != nil {
		t.Fatalf("无目标不该报错: %v", err)
	}
	if got != nil {
		t.Fatalf("goal=null 应返回 nil, got %+v", got)
	}
}

func TestCodexGoalGetTokenBudgetSet(t *testing.T) {
	fake := &fakeCodexCaller{replies: map[string]fakeReply{
		"thread/goal/get": {raw: json.RawMessage(
			`{"goal":{"threadId":"t1","objective":"o","status":"active","tokenBudget":1000,"tokensUsed":5,"timeUsedSeconds":9,"createdAt":1,"updatedAt":2}}`)},
	}}
	got, err := codexGoalGetVia(context.Background(), fake, "t1")
	if err != nil {
		t.Fatalf("不该报错: %v", err)
	}
	if !got.HasTokenBudget || got.TokenBudget != 1000 || got.TokensUsed != 5 || got.TimeUsedSeconds != 9 {
		t.Fatalf("tokenBudget/用量解析错: %+v", got)
	}
}

// ---- 3) modelProvider/capabilities/read ----

func TestCodexProviderCapsMethodAndParse(t *testing.T) {
	// live 实测原文:三个字段全 bool 必有
	fake := &fakeCodexCaller{replies: map[string]fakeReply{
		"modelProvider/capabilities/read": {raw: json.RawMessage(`{"namespaceTools":true,"imageGeneration":true,"webSearch":true}`)},
	}}
	got, err := codexProviderCapsVia(context.Background(), fake)
	if err != nil {
		t.Fatalf("不该报错: %v", err)
	}
	call := fake.lastCall()
	if call.method != "modelProvider/capabilities/read" {
		t.Fatalf("方法名 = %q", call.method)
	}
	if string(call.params) != "{}" {
		t.Fatalf("params 应为空对象(schema 必带), got %s", call.params)
	}
	if !got.ImageGeneration || !got.NamespaceTools || !got.WebSearch {
		t.Fatalf("三能力应全 true: %+v", got)
	}
	// false 也要如实解析(false 不是缺字段)
	fake2 := &fakeCodexCaller{replies: map[string]fakeReply{
		"modelProvider/capabilities/read": {raw: json.RawMessage(`{"namespaceTools":true,"imageGeneration":false,"webSearch":false}`)},
	}}
	got2, err := codexProviderCapsVia(context.Background(), fake2)
	if err != nil {
		t.Fatalf("不该报错: %v", err)
	}
	if got2.ImageGeneration || got2.WebSearch || !got2.NamespaceTools {
		t.Fatalf("false 未如实解析: %+v", got2)
	}
}

// ---- 4) windowsSandbox/readiness ----

func TestCodexSandboxReadinessMethodAndParse(t *testing.T) {
	for status, want := range map[string]string{
		"ready":          SandboxReady,
		"notConfigured":  SandboxNotConfigured,
		"updateRequired": SandboxUpdateRequired,
	} {
		fake := &fakeCodexCaller{replies: map[string]fakeReply{
			"windowsSandbox/readiness": {raw: json.RawMessage(`{"status":"` + status + `"}`)},
		}}
		got, err := codexSandboxReadinessVia(context.Background(), fake)
		if err != nil {
			t.Fatalf("%s: 不该报错: %v", status, err)
		}
		if call := fake.lastCall(); call.method != "windowsSandbox/readiness" {
			t.Fatalf("方法名 = %q", call.method)
		}
		// schema:params 为 null,请求里不带 params
		if call := fake.lastCall(); string(call.params) != "null" {
			t.Fatalf("params 应为 null(不带字段), got %s", call.params)
		}
		if got != want {
			t.Fatalf("status = %q, want %q", got, want)
		}
	}
}

func TestCodexSandboxReadinessUnknownStatusPassthrough(t *testing.T) {
	// codex 新版本若新增状态,必须如实透传给调用方,不能吞掉或误判
	fake := &fakeCodexCaller{replies: map[string]fakeReply{
		"windowsSandbox/readiness": {raw: json.RawMessage(`{"status":"setupInProgress"}`)},
	}}
	got, err := codexSandboxReadinessVia(context.Background(), fake)
	if err != nil || got != "setupInProgress" {
		t.Fatalf("未知状态应透传: got %q err %v", got, err)
	}
	// 缺 status:如实报错(照镜子口子不能伪装成 ready)
	fake2 := &fakeCodexCaller{replies: map[string]fakeReply{
		"windowsSandbox/readiness": {raw: json.RawMessage(`{}`)},
	}}
	if _, err := codexSandboxReadinessVia(context.Background(), fake2); err == nil {
		t.Fatal("缺 status 应报错")
	}
}

// ---- wrapper 不在本文件单测:走 withStatusClient 会拉起真实子进程
// (codexapp.Client 无法从本包构造,零值 provider 一调用就会真启动 app-server)。
// wrapper 只是"确保初始化 + 调 codexXxxVia + 记日志"的薄壳,错误上抛由
// withStatusClient 的既有行为保证;方法名/参数/解析全部由上面的 Via 级测试钉住。
