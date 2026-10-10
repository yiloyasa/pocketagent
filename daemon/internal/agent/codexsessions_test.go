package agent

// codexsessions_test.go —— F9 回归钉子:rollout 解析器双格式支持。
//
// 【钉的是什么】codex 0.160.0 Desktop 的 rollout 把对话内容写在
// response_item/message(payload.role=user|assistant,content[] 里
// input_text|output_text 的 text),旧解析器只认 event_msg 的
// user_message|agent_message → 新格式 rollout 静默解析出 0 条,
// ImportCodexThread 落了 0 消息线程(实锤:"对接一下这个文件夹中的项目")。
// 这里钉三件事:
//  1. 新格式解析出 user/assistant 消息(多 content 条目按 \n 拼接);
//  2. user 侧注入条目按条目跳过(environment_context/user_instructions/
//     AGENTS.md/external_apps/turn_aborted),且不吞并同消息里的真话;
//  3. 旧格式 event_msg 解析逐字不变;两种格式混排同一文件各自解析。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRollout 落一份临时 jsonl rollout,返回路径。
func writeRollout(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rollout-2026-10-09T00-00-00-test.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("写 fixture 失败: %v", err)
	}
	return path
}

// newFormatLine 按 F9 实锤形状构造一行新格式 response_item/message。
func newFormatLine(ts, role string, contentJSON string) string {
	return `{"timestamp":"` + ts + `","type":"response_item","payload":{"type":"message","id":"msg_x","role":"` +
		role + `","content":[` + contentJSON + `]}}`
}

func TestLoadCodexSessionMessagesNewFormat(t *testing.T) {
	path := writeRollout(t,
		`{"timestamp":"2026-10-04T03:26:20.217Z","type":"session_meta","payload":{"cwd":"G:\\项目1","id":"t1"}}`,
		// 真 user 消息:单条 input_text
		newFormatLine("2026-10-04T03:27:00.000Z", "user",
			`{"type":"input_text","text":"对接一下这个文件夹中的项目"}`),
		// 真 assistant 回复:output_text
		newFormatLine("2026-10-04T03:27:05.000Z", "assistant",
			`{"type":"output_text","text":"好的,先看目录结构。"}`),
		// developer 角色:非对话角色,不落
		newFormatLine("2026-10-04T03:26:30.000Z", "developer",
			`{"type":"input_text","text":"You are a coding agent..."}`),
		// 非 message 的 response_item(function_call 等):不落
		`{"timestamp":"2026-10-04T03:27:06.000Z","type":"response_item","payload":{"type":"function_call","name":"shell"}}`,
		// user 消息含两条 content:注入条目 + 真话 → 只落真话
		newFormatLine("2026-10-04T03:28:00.000Z", "user",
			`{"type":"input_text","text":"<environment_context>\n  <cwd>G:\\项目1</cwd>\n</environment_context>"},{"type":"input_text","text":"思考一下该怎么升级"}`),
		// 纯注入消息(全条目被跳过)→ 整条不落
		newFormatLine("2026-10-04T03:28:10.000Z", "user",
			`{"type":"input_text","text":"<turn_aborted>\nThe user interrupted the previous turn on purpose."}`),
	)

	msgs, err := LoadCodexSessionMessages(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("应解析出 3 条(注入/developer/非 message 不落), got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Text != "对接一下这个文件夹中的项目" {
		t.Fatalf("第 1 条应为真 user 消息, got %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Text != "好的,先看目录结构。" {
		t.Fatalf("第 2 条应为 assistant 回复, got %+v", msgs[1])
	}
	// 注入条目被剔除、真话保留(条目级过滤,不是整条丢弃)。
	if msgs[2].Role != "user" || msgs[2].Text != "思考一下该怎么升级" {
		t.Fatalf("第 3 条应剔除注入条目只留真话, got %+v", msgs[2])
	}
	if msgs[0].At.IsZero() {
		t.Fatalf("新格式也应带 timestamp: %+v", msgs[0])
	}
}

func TestLoadCodexSessionMessagesOldFormatUnchanged(t *testing.T) {
	path := writeRollout(t,
		`{"timestamp":"2026-09-13T04:41:54.000Z","type":"event_msg","payload":{"type":"user_message","message":"旧格式的问题"}}`,
		`{"timestamp":"2026-09-13T04:42:00.000Z","type":"event_msg","payload":{"type":"agent_message","message":"旧格式的回答"}}`,
		// 注入前缀在旧格式里不适用(旧格式正文是 codex 转发的用户输入,
		// 不会带这些标记);若有也不跳过 —— 旧格式行为逐字不变。
		`{"timestamp":"2026-09-13T04:42:10.000Z","type":"event_msg","payload":{"type":"agent_message","message":"  "}}`,
		`{"timestamp":"2026-09-13T04:42:20.000Z","type":"event_msg","payload":{"type":"token_count","message":"x"}}`,
	)

	msgs, err := LoadCodexSessionMessages(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("旧格式应解析出 2 条(空白/非对话类型不落), got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Text != "旧格式的问题" {
		t.Fatalf("第 1 条应为旧格式 user, got %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Text != "旧格式的回答" {
		t.Fatalf("第 2 条应为旧格式 assistant, got %+v", msgs[1])
	}
}

func TestLoadCodexSessionMessagesMixedFormats(t *testing.T) {
	// 混排:同一文件两种格式各自解析(线上真实文件是单一格式,这里钉的是
	// 逐行分流的健壮性 —— 新格式上线过渡期不挑文件)。
	path := writeRollout(t,
		newFormatLine("2026-10-09T01:00:00.000Z", "user",
			`{"type":"input_text","text":"新格式的提问"}`),
		`{"timestamp":"2026-10-09T01:00:05.000Z","type":"event_msg","payload":{"type":"user_message","message":"旧格式的提问"}}`,
		newFormatLine("2026-10-09T01:00:10.000Z", "assistant",
			`{"type":"output_text","text":"新格式的回答"}`),
		`{"timestamp":"2026-10-09T01:00:15.000Z","type":"event_msg","payload":{"type":"agent_message","message":"旧格式的回答"}}`,
	)

	msgs, err := LoadCodexSessionMessages(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("混排应各自解析出 4 条, got %d: %+v", len(msgs), msgs)
	}
	got := make([]string, 0, len(msgs))
	for _, m := range msgs {
		got = append(got, m.Role+":"+m.Text)
	}
	want := []string{
		"user:新格式的提问",
		"user:旧格式的提问",
		"assistant:新格式的回答",
		"assistant:旧格式的回答",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 条应为 %q, got %q", i, want[i], got[i])
		}
	}
}

func TestIsCodexInjectedUserEntry(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"<environment_context>\n  <cwd>x</cwd>\n</environment_context>", true},
		{"<user_instructions>foo</user_instructions>", true},
		{"# AGENTS.md instructions\n\n<INSTRUCTIONS>x</INSTRUCTIONS>", true},
		{`<external_codex_apps_open_page>{"page_id":null}</external_codex_apps_open_page>`, true},
		{"<turn_aborted>\nThe user interrupted", true},
		{"  <environment_context>\n  <cwd>x</cwd>", true}, // 前导空白 trim 后前缀命中
		{"对接一下这个文件夹中的项目", false},
		{"普通的用户消息 <environment_context> 在中间", false},
	}
	for _, c := range cases {
		if got := isCodexInjectedUserEntry(c.text); got != c.want {
			t.Fatalf("isCodexInjectedUserEntry(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}
