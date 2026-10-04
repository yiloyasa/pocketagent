// protocol_test.go —— 探针修正的纯函数钉子:双层编码 encode-decode、探针握手/
// 会话夹具解析、缺 key 错误归一化、审批选项挑选(AUDIT §4 缺陷即测试)。
// 夹具逐字取自 docs/verify/evidence/dsh-p0-probe.md 实测原始帧。
package dshapp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// probeHandshakeJSON 探针第 1 项 initialize 应答(逐字)。
const probeHandshakeJSON = `{
  "protocolVersion": 1,
  "agentInfo": {"name": "deepseek-harness-acp", "version": "0.0.1"},
  "agentCapabilities": {
    "mcpCapabilities": {"http": true},
    "promptCapabilities": {"image": false, "audio": false, "embeddedContext": false},
    "sessionCapabilities": {"close": {}, "list": {}, "resume": {}}
  },
  "authMethods": []
}`

// probeConfigOptionsJSON 探针第 2 项 session/new 应答的 configOptions(逐字,
// 顶层数组部分)。model 的 value/currentValue 是双层编码串。
const probeConfigOptionsJSON = `[
  {
    "id": "model", "name": "Model", "category": "model", "type": "select",
    "currentValue": "[\"deepseek-official\",\"deepseek-v4-flash\"]",
    "options": [
      {
        "group": "deepseek-official", "name": "DeepSeek",
        "options": [
          {"value": "[\"deepseek-official\",\"deepseek-flash\"]", "name": "DeepSeek-V41-Flash"},
          {"value": "[\"deepseek-official\",\"deepseek-v4-flash\"]", "name": "DeepSeek-V4-Flash",
           "description": "Fast, efficient, and economical; suited to focused, routine, or parallel tasks."},
          {"value": "[\"deepseek-official\",\"deepseek-v4-pro\"]", "name": "DeepSeek-V4-Pro"},
          {"value": "[\"deepseek-official\",\"deepseek-v4-flash-vision-exp\"]", "name": "DeepSeek-V4-Flash-Vision-Exp"}
        ]
      }
    ]
  },
  {
    "id": "reasoning_effort", "name": "Reasoning effort", "category": "thought_level", "type": "select",
    "currentValue": "high",
    "options": [
      {"value": "off", "name": "Off"},
      {"value": "low", "name": "Low"},
      {"value": "high", "name": "High"},
      {"value": "max", "name": "Max"}
    ]
  }
]`

// TestDecodeModelTupleProbeFixture 探针实测双层编码串必须能解回二元组,
// 且 encode(decode(x)) == x(往返不变,字节级)。
func TestDecodeModelTupleProbeFixture(t *testing.T) {
	fixtures := []string{
		`["deepseek-official","deepseek-v4-flash"]`,
		`["deepseek-official","deepseek-flash"]`,
		`["deepseek-official","deepseek-v4-pro"]`,
	}
	for _, fx := range fixtures {
		route, model, ok := DecodeModelTuple(fx)
		if !ok {
			t.Fatalf("探针实测编码串应可解码: %s", fx)
		}
		if route != "deepseek-official" || model == "" {
			t.Fatalf("解码结果不符: route=%q model=%q", route, model)
		}
		if got := EncodeModelTuple(route, model); got != fx {
			t.Fatalf("encode(decode(x)) 必须逐字节还原 x:\n got=%s\nwant=%s", got, fx)
		}
		if back, ok := ValidModelValue(fx); !ok || back != fx {
			t.Fatalf("ValidModelValue 应原样放行合法编码串: %s -> %q,%v", fx, back, ok)
		}
	}
}

// TestValidModelValueRejectsBareModelName 红线钉子:裸模型名(探针修正①"禁止
// 裸模型名")必须被判非法,绝不能混进 set_config_option 的 value。
func TestValidModelValueRejectsBareModelName(t *testing.T) {
	for _, bad := range []string{
		"deepseek-v4-flash",             // 裸模型名
		"deepseek-official",             // 裸 route
		`["deepseek-official"]`,         // 一元组
		`["deepseek-official","a","b"]`, // 三元组
		``,                              // 空串
		`[123,456]`,                     // 非字符串数组
	} {
		if got, ok := ValidModelValue(bad); ok {
			t.Fatalf("裸值 %q 应被判非法,却返回 %q", bad, got)
		}
	}
}

// TestParseModelCatalogWithProbeFixture 探针第 2 项夹具:4 个模型叶子 + 当前
// 编码串 + 4 档 effort(off/low/high/max,当前 high)。
func TestParseModelCatalogWithProbeFixture(t *testing.T) {
	var opts []ConfigOption
	if err := json.Unmarshal([]byte(probeConfigOptionsJSON), &opts); err != nil {
		t.Fatalf("探针夹具应可解析: %v", err)
	}
	cat := ParseModelCatalog(opts)
	if cat.ModelValue != `["deepseek-official","deepseek-v4-flash"]` {
		t.Fatalf("当前 model 编码串不符: %q", cat.ModelValue)
	}
	if len(cat.ModelOptions) != 4 {
		t.Fatalf("模型叶子应 4 个,实得 %d: %+v", len(cat.ModelOptions), cat.ModelOptions)
	}
	wantModels := map[string]string{
		`["deepseek-official","deepseek-flash"]`:               "DeepSeek-V41-Flash",
		`["deepseek-official","deepseek-v4-flash"]`:            "DeepSeek-V4-Flash",
		`["deepseek-official","deepseek-v4-pro"]`:              "DeepSeek-V4-Pro",
		`["deepseek-official","deepseek-v4-flash-vision-exp"]`: "DeepSeek-V4-Flash-Vision-Exp",
	}
	for _, m := range cat.ModelOptions {
		if wantModels[m.Value] != m.Name {
			t.Fatalf("模型叶子 value→name 不符: %+v", m)
		}
	}
	if cat.EffortValue != "high" {
		t.Fatalf("当前 effort 应为 high: %q", cat.EffortValue)
	}
	if strings.Join(cat.Efforts, ",") != "off,low,high,max" {
		t.Fatalf("effort 档位应 off/low/high/max: %v", cat.Efforts)
	}
}

// TestParseInitializeFixture 探针第 1 项夹具:能力协商面(修正②④的前置)。
func TestParseInitializeFixture(t *testing.T) {
	res := &InitializeResult{}
	if err := json.Unmarshal([]byte(probeHandshakeJSON), res); err != nil {
		t.Fatalf("探针握手夹具应可解析: %v", err)
	}
	if res.ProtocolVersion != 1 || res.AgentInfo.Name != "deepseek-harness-acp" {
		t.Fatalf("握手快照不符: %+v", res.AgentInfo)
	}
	if !res.CanList() || !res.CanResume() || !res.CanClose() {
		t.Fatal("实测 sessionCapabilities close/list/resume 应全在场")
	}
	if res.SupportsImageInput() {
		t.Fatal("实测 promptCapabilities.image=false,不得报支持")
	}
	if len(res.AuthMethods) != 0 {
		t.Fatalf("实测 authMethods 为空: %v", res.AuthMethods)
	}
}

// TestIsMissingCredential 探针修正③:code=-32603 + 无 data + "no API key"
// 才归 MISSING_CREDENTIAL;其余一律不归(错误措辞取探针原始帧)。
func TestIsMissingCredential(t *testing.T) {
	probeMsg := `Internal error: turn failed: llm-deepseek: no API key for provider route "deepseek-official"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment`
	yes := &RPCError{Method: "session/prompt", Code: -32603, Message: probeMsg}
	if !IsMissingCredential(yes) {
		t.Fatal("探针实测缺 key 错误应命中 MISSING_CREDENTIAL")
	}
	no := []*RPCError{
		{Method: "m", Code: -32603, Message: probeMsg, Data: json.RawMessage(`{"x":1}`)}, // 有 data 不归
		{Method: "m", Code: -32000, Message: probeMsg},                                   // 其他 code 不归
		{Method: "m", Code: -32603, Message: "Internal error: turn failed: boom"},        // 非 key 词不归
	}
	for i, e := range no {
		if IsMissingCredential(e) {
			t.Fatalf("用例 %d 不应命中: %+v", i, e)
		}
	}
	if IsMissingCredential(errors.New("no API key")) {
		t.Fatal("非 *RPCError 不应命中")
	}
	if msg, ok := RPCErrorMessage(yes); !ok || msg != probeMsg {
		t.Fatal("RPCErrorMessage 应原样取回 dsh message")
	}
}

// TestRequestPermissionOptions 审批选项挑选:按 kind 前缀,allow 取首个、reject
// 取首个;尾部兜底不选中 allow 侧;无 reject 侧时显式报错由上层 RespondError。
func TestRequestPermissionOptions(t *testing.T) {
	raw := []byte(`{"sessionId":"s1","options":[
		{"optionId":"ok1","name":"Allow","kind":"allow_once"},
		{"optionId":"no1","name":"Reject","kind":"reject_once"}]}`)
	p, err := ParseRequestPermission(raw)
	if err != nil {
		t.Fatalf("合法参数应可解析: %v", err)
	}
	if p.AllowOptionID() != "ok1" || p.RejectOptionID() != "no1" {
		t.Fatalf("allow/reject 选项不符: %q %q", p.AllowOptionID(), p.RejectOptionID())
	}
	// 只有 allow 侧:reject 显式报空(上层 RespondError,不猜不伪造)。
	onlyAllow := []byte(`{"sessionId":"s1","options":[{"optionId":"ok1","name":"Allow","kind":"allow_once"}]}`)
	p2, _ := ParseRequestPermission(onlyAllow)
	if p2.RejectOptionID() != "" {
		t.Fatalf("无 reject 侧应返回空串: %q", p2.RejectOptionID())
	}
	// 坏参数:缺 options → 错误。
	if _, err := ParseRequestPermission([]byte(`{"sessionId":"s1"}`)); err == nil {
		t.Fatal("缺 options 应报错")
	}
}

// TestPermissionOutcomeShape 应答形状(ACP v1:判别字段 outcome;真机待验项)。
func TestPermissionOutcomeShape(t *testing.T) {
	sel := PermissionOutcomeSelected("ok1")
	b, _ := json.Marshal(sel)
	want := `{"outcome":{"optionId":"ok1","outcome":"selected"}}`
	if string(b) != want {
		t.Fatalf("selected 应答形状不符:\n got=%s\nwant=%s", b, want)
	}
}
