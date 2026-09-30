// event_translate_test.go —— 事件翻译层(读时翻译,纯函数)的回归用例。
//
// 验收点:
//
//	① 已知方法的翻译是人话,不是方法名复述(summary 不得出现 method 原样);
//	② 未知 JSON 载荷逐字段透析:类型/预览/含义三列,识别不出的含义留空不猜;
//	③ 启发式意图如实:像错误就说像错误,判断不了就明说"意图不明";
//	④ 非 JSON/空载荷诚实降级,不编造;
//	⑤ Detail 写明 daemon 处置,并带"冲突以原文为准"的红线句;
//	⑥ 翻译只消化脱敏输入,不引入输入之外的新事实。
package runtime

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// runeLen 中文字符按字数而非字节计,钉"Summary ≤60 字左右"。
func runeLen(s string) int { return utf8.RuneCountInString(s) }

// TestKnownMethodsTranslateToPlainLanguage 已知方法的翻译必须是人话。
//
// 【为什么逐家钉】"同一框架"意味着四家 agent 的高频事件都能被翻;任何一家的
// 方法从知识库里漏掉,它就会掉进"未知"路径,诊断页上人话变术语 —— 病根就在这。
func TestKnownMethodsTranslateToPlainLanguage(t *testing.T) {
	cases := []struct {
		agent, method, kind string
		wantKeyword         string // summary 必须含的关键词(人话证据)
		wantDetailPart      string // detail 必须含的处置措辞
	}{
		// codex:方案点名的七个 + 全局高频。
		{"codex", "turn.completed", KindKnownProjected, "完成", "投影"},
		{"codex", "turn.failed", KindKnownProjected, "失败", "投影"},
		{"codex", "contextCompaction", KindKnownProjected, "压缩", "投影"},
		{"codex", "token_count", KindKnownProjected, "用量", "投影"},
		{"codex", "session_configured", KindKnownProjected, "初始化", "投影"},
		{"codex", "item/completed:userMessage", KindKnownPassthrough, "回显", "透传"},
		{"codex", "item/completed:summary", KindKnownPassthrough, "摘要", "透传"},
		{"codex", "thread/name/updated", KindKnownProjected, "标题", "投影"},
		{"codex", "turn/diff/updated", KindKnownPassthrough, "改动", "透传"},
		// codex 0.159.2 收编批(generate-json-schema diff + 活体探针,2026-09-30):
		// authRecovery 两连(模型服务登录态自动恢复)与 terminalInteraction
		// (命令等终端输入;0.159.2 起配套 writeStdin 审批卡)。
		{"codex", "modelProvider/authRecoveryStarted", KindKnownPassthrough, "恢复", "透传"},
		{"codex", "modelProvider/authRecoveryCompleted", KindKnownPassthrough, "恢复", "透传"},
		{"codex", "item/commandExecution/terminalInteraction", KindKnownProjected, "输入", "投影"},
		// pi。
		{"pi", "message_update", KindKnownPassthrough, "消息", "透传"},
		{"pi", "session_info_changed", KindKnownPassthrough, "会话", "透传"},
		{"pi", "compaction_start", KindKnownPassthrough, "压缩", "透传"},
		// zcode。
		{"zcode", "model.streaming", KindKnownProjected, "流式", "投影"},
		{"zcode", "tool.updated", KindKnownProjected, "工具", "投影"},
		// 0.16.9 实测:模型/思考档变化的唯一载体(state.updated,词表措辞随实证更新)。
		{"zcode", "state.updated", KindKnownProjected, "状态", "投影"},
		// steer 类在 zcodeNoticeKind 归 PASSTHROUGH(现网真实 kind);detail 断言
		// "透传" —— 钉"翻译按方法给处置措辞,不依赖分类表"这件事。
		{"zcode", "turn.steerQueued", KindKnownPassthrough, "队列", "透传"},
		{"zcode", "session.modelUpdated", KindKnownProjected, "模型", "投影"},
		// dsh(带状态后缀的变体走前缀匹配)。
		{"dsh", "session/update:context_usage", KindKnownProjected, "上下文", "投影"},
		{"dsh", "session/update:tool_call:completed", KindKnownProjected, "工具", "投影"},
		{"dsh", "session/update:plan", KindKnownPassthrough, "计划", "透传"},
	}
	for _, tc := range cases {
		tr := TranslateEvent(tc.agent, tc.method, tc.kind, `{"sample":1}`)
		if tr.Summary == "" {
			t.Errorf("%s/%s: summary 不得为空", tc.agent, tc.method)
			continue
		}
		if strings.Contains(tr.Summary, tc.method) {
			t.Errorf("%s/%s: summary 复述了方法名 %q,不是人话: %s", tc.agent, tc.method, tc.method, tr.Summary)
		}
		if !strings.Contains(tr.Summary, tc.wantKeyword) {
			t.Errorf("%s/%s: summary 缺关键词 %q: %s", tc.agent, tc.method, tc.wantKeyword, tr.Summary)
		}
		if runeLen(tr.Summary) > 60 {
			t.Errorf("%s/%s: summary 超过 60 字(%d): %s", tc.agent, tc.method, runeLen(tr.Summary), tr.Summary)
		}
		if tc.wantDetailPart != "" && !strings.Contains(tr.Detail, tc.wantDetailPart) {
			t.Errorf("%s/%s: detail 应说明处置(%s): %s", tc.agent, tc.method, tc.wantDetailPart, tr.Detail)
		}
		// 红线:每条 detail 都要声明"翻译是摘要,冲突以原文为准"。
		if !strings.Contains(tr.Detail, "原文") {
			t.Errorf("%s/%s: detail 缺'以原文为准'红线句: %s", tc.agent, tc.method, tr.Detail)
		}
	}
}

// TestUnknownJSONPayloadDissectedFieldByField 未知方法的 JSON 载荷必须逐字段拆解。
func TestUnknownJSONPayloadDissectedFieldByField(t *testing.T) {
	payload := `{"threadId":"th-9","count":3,"ok":true,"tags":["a","b"],` +
		`"meta":{"depth":1},"note":null,"token":"[redacted]","zzUnfamiliar":"x"}`
	tr := TranslateEvent("codex", "brandNewNotificationXYZ", KindUnknownNotification, payload)

	byKey := map[string]EventField{}
	for _, f := range tr.Fields {
		byKey[f.Key] = f
	}
	want := []EventField{
		{Key: "threadId", Kind: "text", Meaning: "会话标识"},
		{Key: "count", Kind: "number", Meaning: "数量"},
		{Key: "ok", Kind: "bool"},
		{Key: "tags", Kind: "array"},
		{Key: "meta", Kind: "object"},
		{Key: "note", Kind: "null"},
		{Key: "token", Kind: "text", Meaning: "凭据(已脱敏)"},
		{Key: "zzUnfamiliar", Kind: "text", Meaning: ""}, // 识别不出 → 留空,不猜
	}
	for _, w := range want {
		got, ok := byKey[w.Key]
		if !ok {
			t.Fatalf("透析表缺字段 %q(应逐字段拆解,一列不少): %+v", w.Key, tr.Fields)
		}
		if got.Kind != w.Kind {
			t.Errorf("字段 %q 类型应为 %s,得到 %s", w.Key, w.Kind, got.Kind)
		}
		if got.Meaning != w.Meaning {
			t.Errorf("字段 %q 含义应为 %q(识别不出必须留空,不猜),得到 %q", w.Key, w.Meaning, got.Meaning)
		}
		if runeLen(got.Preview) > 60 {
			t.Errorf("字段 %q 预览超过 60 字(%d): %s", w.Key, runeLen(got.Preview), got.Preview)
		}
	}
	if got := byKey["threadId"].Preview; got != "th-9" {
		t.Errorf("threadId 预览应为值本身,得到 %q", got)
	}
	if got := byKey["token"].Preview; got != "[redacted]" {
		t.Errorf("脱敏值必须原样进预览(翻译不得'还原'或再加工): %q", got)
	}
	if !strings.Contains(tr.Summary, "未知") {
		t.Errorf("未知事件的 summary 应如实说未知: %s", tr.Summary)
	}
}

// TestUnknownPayloadHeuristics 启发式意图:像错误就说像错误;判断不了就明说。
func TestUnknownPayloadHeuristics(t *testing.T) {
	errorTr := TranslateEvent("pi", "novelEventA", KindUnknownNotification, `{"error":{"message":"boom"}}`)
	if !strings.Contains(errorTr.Summary+errorTr.Detail, "错误上报") {
		t.Errorf("含 error 字段应判'像一次错误上报': %s | %s", errorTr.Summary, errorTr.Detail)
	}
	progressTr := TranslateEvent("dsh", "novelEventB", KindUnknownNotification, `{"progress":50,"total":100}`)
	if !strings.Contains(progressTr.Summary+progressTr.Detail, "进度") {
		t.Errorf("含 progress 字段应判'可能是进度或列表类上报': %s | %s", progressTr.Summary, progressTr.Detail)
	}
	unclearTr := TranslateEvent("zcode", "novelEventC", KindUnknownNotification, `{"weather":"sunny"}`)
	if !strings.Contains(unclearTr.Summary+unclearTr.Detail, "意图不明") {
		t.Errorf("判断不了应如实说'意图不明': %s | %s", unclearTr.Summary, unclearTr.Detail)
	}
	// 嵌套的失败信号同样要被启发式看到(透析表只列顶层,意图判断要看全)。
	nestedTr := TranslateEvent("pi", "novelEventD", KindUnknownNotification, `{"detail":{"fail":true}}`)
	if !strings.Contains(nestedTr.Summary+nestedTr.Detail, "错误上报") {
		t.Errorf("嵌套 fail 字段也应判'像一次错误上报': %s | %s", nestedTr.Summary, nestedTr.Detail)
	}
}

// TestNonJSONAndEmptyPayloadHonestFallback 非 JSON/空载荷必须诚实降级,不编造。
func TestNonJSONAndEmptyPayloadHonestFallback(t *testing.T) {
	text := TranslateEvent("codex", "brandNewThing", KindUnknownNotification, "这是一段纯文本通知")
	if len(text.Fields) != 0 {
		t.Errorf("纯文本没有可拆字段,不得硬造: %+v", text.Fields)
	}
	if !strings.Contains(text.Summary+text.Detail, "文本") {
		t.Errorf("纯文本应如实说明: %s | %s", text.Summary, text.Detail)
	}
	empty := TranslateEvent("codex", "brandNewThing", KindUnknownNotification, "")
	if len(empty.Fields) != 0 {
		t.Errorf("空载荷没有字段: %+v", empty.Fields)
	}
	if !strings.Contains(empty.Summary+empty.Detail, "没有") {
		t.Errorf("空载荷应如实说明没有内容: %s | %s", empty.Summary, empty.Detail)
	}
	arr := TranslateEvent("codex", "brandNewThing", KindUnknownNotification, `[1,2,3]`)
	if !strings.Contains(arr.Summary+arr.Detail, "数组") {
		t.Errorf("顶层数组应如实说明: %s | %s", arr.Summary, arr.Detail)
	}
}

// TestTranslateHandlesPathologicalInput 病态输入不 panic、预览有界。
func TestTranslateHandlesPathologicalInput(t *testing.T) {
	long := strings.Repeat("长", 500)
	tr := TranslateEvent("codex", "brandNewThing", KindUnknownNotification, `{"big":"`+long+`"}`)
	for _, f := range tr.Fields {
		if runeLen(f.Preview) > 60 {
			t.Fatalf("超长值必须截断到 60 字内,得到 %d", runeLen(f.Preview))
		}
	}
	// 深嵌套不得爆栈(递归有深度上限)。
	deep := strings.Repeat(`{"n":`, 64) + "1" + strings.Repeat("}", 64)
	TranslateEvent("codex", "brandNewThing", KindUnknownNotification, deep)
	// 非法 JSON(截断的)按非 JSON 降级,不报错。
	TranslateEvent("codex", "brandNewThing", KindUnknownNotification, `{"cut":`)
}

// TestTranslateAddsNoNewFacts 翻译是消化脱敏输入的纯函数:输出不含输入之外的事实。
//
// 【反证】把载荷里的可识别值换成本测试独有的哨兵,若 summary/detail 里冒出
// 哨兵之外的具体断言(例如把 unknown 说成"成功"),即为编造。
func TestTranslateAddsNoNewFacts(t *testing.T) {
	tr := TranslateEvent("zcode", "brandNewThing", KindUnknownNotification, `{"status":"WEIRD-VALUE-XYZ"}`)
	joined := tr.Summary + tr.Detail
	for _, f := range tr.Fields {
		joined += f.Preview + f.Meaning
	}
	if strings.Contains(joined, "成功") {
		t.Errorf("载荷没有说成功,翻译不得宣判成功: %s", joined)
	}
	if !strings.Contains(joined, "WEIRD-VALUE-XYZ") {
		t.Errorf("透析应保留值预览供人工核对: %s", joined)
	}
}
