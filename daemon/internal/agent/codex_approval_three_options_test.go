package agent

// codex_approval_three_options_test.go —— codex 0.159.2 审批三按钮增量的回归钉子。
//
// 【钉什么】0.159.2 的 requestApproval 应答枚举新增:
//   - acceptForSession:允许且本会话内同类请求不再询问(旧版 approved_for_session);
//   - cancel:拒绝且立即中止当前轮次(旧版 abort)。
//
// 三条红线:
//  1. 审批卡必须带三选项(允许/本次及本会话允许/拒绝),id 与应答枚举一一对应;
//  2. 旧手机两按钮路径(不带选项)的应答枚举一字不变(旧端兼容);
//  3. cancel 选项必须落到 codex 认识的 decision 值上(新版 cancel/旧版 abort),
//     由 codex 侧打断当前轮次,daemon 不伪造其他应答。
//
// 枚举精确字符串以 codex-schema/CommandExecutionRequestApprovalResponse.json
// 与 ExecCommandApprovalResponse.json(0.159.2)为准。

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// cardOptions 取出审批卡 JSON 里的 options 键。
func cardOptions(t *testing.T, cardJson string) []map[string]interface{} {
	t.Helper()
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(cardJson), &args); err != nil {
		t.Fatalf("审批卡 JSON 解析失败: %v\n%s", err, cardJson)
	}
	opts, _ := args["options"].([]interface{})
	out := make([]map[string]interface{}, 0, len(opts))
	for _, o := range opts {
		m, _ := o.(map[string]interface{})
		out = append(out, m)
	}
	return out
}

// optionIds 只取 id 序列。
func optionIds(opts []map[string]interface{}) []string {
	ids := make([]string, 0, len(opts))
	for _, o := range opts {
		id, _ := o["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

func idsEqual(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestCodexApprovalCardExposesThreeOptions 新版与旧版审批卡都带三选项,
// id 用各自协议版本的应答枚举原词。
func TestCodexApprovalCardExposesThreeOptions(t *testing.T) {
	cases := []struct {
		method string
		kind   codexRequestKind
		params string
		want   []string
	}{
		{"item/commandExecution/requestApproval", reqApprovalCommand,
			`{"itemId":"i1","kind":"command","command":"cargo build","cwd":"/w"}`,
			[]string{"accept", "acceptForSession", "decline"}},
		// writeStdin 与 command 同一枚举(schema Response 未按 kind 分叉)
		{"item/commandExecution/requestApproval", reqApprovalCommand,
			`{"itemId":"i2","approvalId":"a1","kind":"writeStdin","reason":"写输入"}`,
			[]string{"accept", "acceptForSession", "decline"}},
		{"item/fileChange/requestApproval", reqApprovalFileChange,
			`{"itemId":"i3","reason":"改文件"}`,
			[]string{"accept", "acceptForSession", "decline"}},
		{"execCommandApproval", reqApprovalCommand,
			`{"callId":"c1","command":["go","test"]}`,
			[]string{"approved", "approved_for_session", "denied"}},
		{"applyPatchApproval", reqApprovalPatch,
			`{"callId":"c2","fileChanges":{"x.go":{}}}`,
			[]string{"approved", "approved_for_session", "denied"}},
	}
	for _, c := range cases {
		card := buildApprovalCall(c.kind, c.method, json.RawMessage(c.params), nil)
		opts := cardOptions(t, card.JsonArgs)
		if got := optionIds(opts); !idsEqual(got, c.want...) {
			t.Fatalf("%s 卡三选项 id = %v, want %v", c.method, got, c.want)
		}
		if opts[1]["label"] != "本次及本会话允许" {
			t.Fatalf("%s 第三按钮文案 = %v, want 本次及本会话允许", c.method, opts[1]["label"])
		}
	}
}

// TestCodexApprovalCardKeepsOldFieldsOldClientsUnaware 旧端兼容:卡片序列化后
// 老字段(kind/command/cwd/reason)完整如初 —— 旧手机忽略未知 options 键,
// 渲染出的仍是原来的双按钮卡,内容一个不少。
func TestCodexApprovalCardKeepsOldFieldsOldClientsUnaware(t *testing.T) {
	card := buildApprovalCall(reqApprovalCommand, "item/commandExecution/requestApproval",
		json.RawMessage(`{"itemId":"i1","kind":"command","command":"rm -rf build","cwd":"/w","reason":"清理"}`), nil)
	if card.Name != toolNameShell || card.Id != "i1" {
		t.Fatalf("老字段漂移:name=%q id=%q", card.Name, card.Id)
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(card.JsonArgs), &args); err != nil {
		t.Fatalf("卡 JSON 损坏: %v", err)
	}
	for _, k := range []string{"kind", "command", "cwd", "reason"} {
		if _, ok := args[k]; !ok {
			t.Errorf("老字段 %s 丢失,旧手机卡片内容不再完整: %v", k, args)
		}
	}
	// 0.159.2 之前的老 codex(参数里没有新字段):不得凭空加 details 键。
	legacy := buildApprovalCall(reqApprovalCommand, "execCommandApproval",
		json.RawMessage(`{"callId":"c9","command":["ls"]}`), nil)
	var legacyArgs map[string]interface{}
	_ = json.Unmarshal([]byte(legacy.JsonArgs), &legacyArgs)
	if _, ok := legacyArgs["details"]; ok {
		t.Errorf("无新字段的审批不应有 details 键: %v", legacyArgs["details"])
	}
}

// TestCodexApprovalCardDetailsPlainText proposedExecpolicyAmendment /
// commandActions 只拼人话详情,不做结构化 UI;两者都缺时不加 details 键。
func TestCodexApprovalCardDetailsPlainText(t *testing.T) {
	card := buildApprovalCall(reqApprovalCommand, "item/commandExecution/requestApproval",
		json.RawMessage(`{
			"itemId":"i1","kind":"command","command":"git push origin main","cwd":"/w",
			"proposedExecpolicyAmendment":["git push origin *","git fetch *"],
			"commandActions":[
				{"type":"read","command":"cat","name":"main.go","path":"/w/main.go"},
				{"type":"listFiles","command":"ls","path":"/w"},
				{"type":"search","command":"grep","path":"/w","query":"TODO"},
				{"type":"unknown","command":"make"}
			]}`), nil)
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(card.JsonArgs), &args); err != nil {
		t.Fatalf("卡 JSON 解析失败: %v", err)
	}
	details, _ := args["details"].(string)
	if details == "" {
		t.Fatalf("带新字段的审批卡必须有人话 details: %s", card.JsonArgs)
	}
	for _, want := range []string{"这条命令会", "读取", "列出目录", "搜索", "免审批", "2 条命令规则"} {
		if !strings.Contains(details, want) {
			t.Errorf("details 缺少 %q: %s", want, details)
		}
	}
	// 结构化 UI 红线:原始数组不得透传成独立键,人话进 details 一个键就够。
	for _, banned := range []string{"proposedExecpolicyAmendment", "commandActions"} {
		if _, ok := args[banned]; ok {
			t.Errorf("卡上不应有结构化键 %s(只做 details 人话)", banned)
		}
	}

	empty := buildApprovalCall(reqApprovalCommand, "item/commandExecution/requestApproval",
		json.RawMessage(`{"itemId":"i2","kind":"command","command":"echo hi"}`), nil)
	var emptyArgs map[string]json.RawMessage
	_ = json.Unmarshal([]byte(empty.JsonArgs), &emptyArgs)
	if _, ok := emptyArgs["details"]; ok {
		t.Errorf("无新字段时不应有 details 键: %s", empty.JsonArgs)
	}
}

// TestParseApprovalChoice note 信封解析:收得住选项,不错杀反问答案。
func TestParseApprovalChoice(t *testing.T) {
	cases := []struct {
		name     string
		note     string
		wantOpt  string
		wantNote string
	}{
		{"信封", `{"option":"acceptForSession"}`, "acceptForSession", ""},
		{"信封带note", `{"option":"cancel","note":"太危险"}`, "cancel", "太危险"},
		{"旧端纯文本", "换个方式吧", "", "换个方式吧"},
		{"旧端空", "", "", ""},
		{"反问答案不是信封", `{"q1":{"answers":["A"]}}`, "", `{"q1":{"answers":["A"]}}`},
		{"option非字符串不是信封", `{"option":{"answers":["A"]}}`, "", `{"option":{"answers":["A"]}}`},
		{"多余键不是信封", `{"option":"accept","qid":"q1"}`, "", `{"option":"accept","qid":"q1"}`},
		{"坏JSON", `{"option":`, "", `{"option":`},
	}
	for _, c := range cases {
		got := parseApprovalChoice(c.note)
		if got.Option != c.wantOpt || got.Note != c.wantNote {
			t.Errorf("%s: parseApprovalChoice(%q) = {%q,%q}, want {%q,%q}",
				c.name, c.note, got.Option, got.Note, c.wantOpt, c.wantNote)
		}
	}
}

// TestCodexApprovalDecisionOptionTable 选项 id → 各协议版本 decision 枚举的翻译表。
func TestCodexApprovalDecisionOptionTable(t *testing.T) {
	cases := []struct {
		method, option string
		allow          bool
		want           string
	}{
		// 0.159.2 新枚举
		{"item/commandExecution/requestApproval", "acceptForSession", true, "acceptForSession"},
		{"item/fileChange/requestApproval", "acceptForSession", true, "acceptForSession"},
		{"item/commandExecution/requestApproval", "cancel", false, "cancel"},
		{"item/fileChange/requestApproval", "cancel", false, "cancel"},
		// 旧版对应词
		{"execCommandApproval", "approved_for_session", true, "approved_for_session"},
		{"applyPatchApproval", "approved_for_session", true, "approved_for_session"},
		{"execCommandApproval", "abort", false, "abort"},
		// 归一:同一语义的两个 id,按协议版本翻到正确一侧
		{"execCommandApproval", "cancel", false, "abort"},
		{"item/commandExecution/requestApproval", "abort", false, "cancel"},
		{"execCommandApproval", "acceptForSession", true, "approved_for_session"},
		{"item/commandExecution/requestApproval", "approved_for_session", true, "acceptForSession"},
		// 空/未知 option = 旧端两按钮,枚举一字不变
		{"item/commandExecution/requestApproval", "", true, "accept"},
		{"item/commandExecution/requestApproval", "", false, "decline"},
		{"execCommandApproval", "", true, "approved"},
		{"execCommandApproval", "", false, "denied"},
		{"item/commandExecution/requestApproval", "someFutureOption", true, "accept"},
	}
	for _, c := range cases {
		if got := codexApprovalDecision(c.method, c.option, c.allow); got != c.want {
			t.Errorf("codexApprovalDecision(%q, %q, %v) = %q, want %q",
				c.method, c.option, c.allow, got, c.want)
		}
	}
}

// approveWithNote 生成返回固定 (allow, note) 的审批钩子。
func approveWithNote(allow bool, note string) ApprovalHook {
	return func(context.Context, *pb.ToolCallBlock) (bool, string, error) {
		return allow, note, nil
	}
}

// TestCodexApprovalCancelOptionEndsTurn cancel 路径:拒绝+中止 —— 新版回
// decision=cancel(codex 收到后立即打断当前轮次),旧版回 abort。
func TestCodexApprovalCancelOptionEndsTurn(t *testing.T) {
	p := NewCodexProvider()
	var waiting atomic.Bool
	sink := func(*pb.TurnServerMessage) {}

	rspV2 := &fakeCodexResponder{}
	p.handleCodexApproval(rspV2, 21, reqApprovalCommand, "item/commandExecution/requestApproval",
		json.RawMessage(`{"itemId":"i1","kind":"command","command":"dangerous"}`), nil,
		context.Background(), sink, approveWithNote(false, `{"option":"cancel"}`), &waiting)
	if n := rspV2.resultCount(); n != 1 || rspV2.results[0]["decision"] != "cancel" {
		t.Fatalf("新版 cancel 选项应回 decision=cancel,得到 %v", rspV2.results)
	}

	rspLegacy := &fakeCodexResponder{}
	p.handleCodexApproval(rspLegacy, 22, reqApprovalCommand, "execCommandApproval",
		json.RawMessage(`{"callId":"c1","command":["dangerous"]}`), nil,
		context.Background(), sink, approveWithNote(false, `{"option":"abort"}`), &waiting)
	if n := rspLegacy.resultCount(); n != 1 || rspLegacy.results[0]["decision"] != "abort" {
		t.Fatalf("旧版 abort 选项应回 decision=abort,得到 %v", rspLegacy.results)
	}
}

// TestCodexApprovalAcceptForSessionOption 会话内免再问:新版 acceptForSession,
// 旧版 approved_for_session —— 上游据此把同类请求静默放行到本会话结束。
func TestCodexApprovalAcceptForSessionOption(t *testing.T) {
	p := NewCodexProvider()
	var waiting atomic.Bool
	sink := func(*pb.TurnServerMessage) {}

	rspV2 := &fakeCodexResponder{}
	p.handleCodexApproval(rspV2, 23, reqApprovalCommand, "item/commandExecution/requestApproval",
		json.RawMessage(`{"itemId":"i1","kind":"command","command":"npm test"}`), nil,
		context.Background(), sink, approveWithNote(true, `{"option":"acceptForSession"}`), &waiting)
	if n := rspV2.resultCount(); n != 1 || rspV2.results[0]["decision"] != "acceptForSession" {
		t.Fatalf("新版会话允许应回 decision=acceptForSession,得到 %v", rspV2.results)
	}

	rspLegacy := &fakeCodexResponder{}
	p.handleCodexApproval(rspLegacy, 24, reqApprovalCommand, "execCommandApproval",
		json.RawMessage(`{"callId":"c1","command":["npm","test"]}`), nil,
		context.Background(), sink, approveWithNote(true, `{"option":"approved_for_session"}`), &waiting)
	if n := rspLegacy.resultCount(); n != 1 || rspLegacy.results[0]["decision"] != "approved_for_session" {
		t.Fatalf("旧版会话允许应回 decision=approved_for_session,得到 %v", rspLegacy.results)
	}
}

// TestCodexApprovalOldPhoneUnchanged 旧端兼容(整链):老手机不带选项,只回
// approved 布尔 —— 各类审批的应答枚举与三按钮增量之前一字不差。
func TestCodexApprovalOldPhoneUnchanged(t *testing.T) {
	p := NewCodexProvider()
	var waiting atomic.Bool
	sink := func(*pb.TurnServerMessage) {}

	cases := []struct {
		name, method string
		kind         codexRequestKind
		params       string
		allow        bool
		want         string
	}{
		{"新版允许", "item/commandExecution/requestApproval", reqApprovalCommand,
			`{"itemId":"i1","kind":"command","command":"ls"}`, true, "accept"},
		{"新版拒绝", "item/fileChange/requestApproval", reqApprovalFileChange,
			`{"itemId":"i2"}`, false, "decline"},
		{"旧版允许", "execCommandApproval", reqApprovalCommand,
			`{"callId":"c1","command":["ls"]}`, true, "approved"},
		{"旧版拒绝", "applyPatchApproval", reqApprovalPatch,
			`{"callId":"c2","fileChanges":{}}`, false, "denied"},
	}
	for i, c := range cases {
		rsp := &fakeCodexResponder{}
		p.handleCodexApproval(rsp, int64(30+i), c.kind, c.method,
			json.RawMessage(c.params), nil,
			context.Background(), sink, approveWithNote(c.allow, ""), &waiting)
		if n := rsp.resultCount(); n != 1 || rsp.results[0]["decision"] != c.want {
			t.Errorf("%s: decision = %v, want %q", c.name, rsp.results, c.want)
		}
	}
}

// TestCodexApprovalNoteEnvelopeDoesNotLeakIntoAskUser 反问卡不受信封影响:
// 答案 JSON 原样进 splitAnswerNote(除非真带了 option 键——不该发生,剥掉)。
func TestCodexApprovalNoteEnvelopeDoesNotLeakIntoAskUser(t *testing.T) {
	p := NewCodexProvider()
	var waiting atomic.Bool
	sink := func(*pb.TurnServerMessage) {}
	rsp := &fakeCodexResponder{}
	p.handleCodexApproval(rsp, 40, reqAskUser, "item/tool/requestUserInput",
		json.RawMessage(`{"itemId":"i1","questions":[{"id":"q1","question":"用哪个?","options":[{"label":"A"}]}]}`), nil,
		context.Background(), sink, approveWithNote(true, `{"q1":{"answers":["A"]}}`), &waiting)
	if n := rsp.resultCount(); n != 1 {
		t.Fatalf("反问应答应答一次,得到 %v", rsp.results)
	}
	answers, _ := rsp.results[0]["answers"].(map[string]interface{})
	if answers == nil || answers["q1"] == nil {
		t.Fatalf("反问答案应原样解析: %v", rsp.results[0])
	}
}
