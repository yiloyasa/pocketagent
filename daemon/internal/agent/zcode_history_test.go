// zcode_history_test.go —— 导入会话历史惰性拉取的映射测试。
//
// 夹具形状来自探针实证(docs/verify/evidence/zcode-p0-probe.md §4.6,
// session/resume 新进程实测):messages[] 每条 = info{role,semantics,time} + parts[]。
// HistoryFor 本体只是 withClientCtx + resume + parseZcodeHistory 的薄包装
// (要起真子进程,不进单测),映射规则全部钉在这里。
package agent

import (
	"encoding/json"
	"reflect"
	"testing"
)

// zcodeResumeFixture 探针实录形状的裁剪版:时间线事件 / 用户文本 / 助手
// (step-start + reasoning + text + step-finish)/ 多 text 段 / 空文本。
const zcodeResumeFixture = `{"messages":[` +
	// 1) 换模型的系统时间线事件:整条丢弃
	`{"info":{"role":"assistant","semantics":{"kind":"timeline_event"},` +
	`"time":{"created":1789755192907}},` +
	`"parts":[{"type":"timeline","display":"separator","timelineType":"model_change"}]},` +
	// 2) 用户文本
	`{"info":{"role":"user","semantics":{"kind":"user_prompt"},` +
	`"time":{"created":1789755192912}},` +
	`"parts":[{"type":"text","text":"你好,只回复四个字母:PONG"}]},` +
	// 3) 助手回复:reasoning/step-start/step-finish 丢弃,只留 text
	`{"info":{"role":"assistant","semantics":{"kind":"assistant_response"},` +
	`"time":{"created":1789755195126}},` +
	`"parts":[{"type":"step-start"},` +
	`{"type":"reasoning","text":"The user wants PONG."},` +
	`{"type":"text","text":"PONG"},` +
	`{"type":"step-finish","reason":"stop"}]},` +
	// 4) 助手多段 text:按原顺序 \n 连接
	`{"info":{"role":"assistant","semantics":{"kind":"assistant_response"},` +
	`"time":{"created":1789755197000}},` +
	`"parts":[{"type":"text","text":"第一段"},{"type":"step-start"},{"type":"text","text":"第二段"}]},` +
	// 5) 未知角色(system):丢弃
	`{"info":{"role":"system","semantics":{"kind":"note"},` +
	`"time":{"created":1789755198000}},` +
	`"parts":[{"type":"text","text":"系统提示"}]},` +
	// 6) 有 role 但没有任何 text 部件:丢弃
	`{"info":{"role":"assistant","semantics":{"kind":"assistant_response"},` +
	`"time":{"created":1789755199000}},` +
	`"parts":[{"type":"step-start"},{"type":"reasoning","text":"只有思考"}]}` +
	`]}`

func TestParseZcodeHistory(t *testing.T) {
	got := parseZcodeHistory(json.RawMessage(zcodeResumeFixture))
	want := []HistoryMsg{
		{Role: "user", Text: "你好,只回复四个字母:PONG", CreatedAtUnixMs: 1789755192912},
		{Role: "assistant", Text: "PONG", CreatedAtUnixMs: 1789755195126},
		{Role: "assistant", Text: "第一段\n第二段", CreatedAtUnixMs: 1789755197000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("映射结果不符:\n got=%#v\nwant=%#v", got, want)
	}
}

// 空会话/形状异常都不报错:导入线程的历史页不能被快照异常打挂。
func TestParseZcodeHistoryDegenerate(t *testing.T) {
	cases := map[string]string{
		"空messages":   `{"messages":[]}`,
		"缺messages字段": `{"session":{"sessionId":"sess_x"}}`,
		"纯垃圾JSON":     `not-json`,
		"null":        `null`,
	}
	for name, raw := range cases {
		if got := parseZcodeHistory(json.RawMessage(raw)); len(got) != 0 {
			t.Fatalf("%s:应返回空切片,得到 %#v", name, got)
		}
	}
}
