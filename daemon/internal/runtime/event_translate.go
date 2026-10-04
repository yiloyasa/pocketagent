// event_translate.go —— 事件翻译层:把台账里的每条事件(已知/未知)翻成人话。
//
// 【它解决什么】诊断页以前只给 method 名与原始 JSON —— 用户看到
// "turn.steerQueued" 仍然是术语,不是解释。本文件在**读取时**做纯函数翻译:
// 不写台账、不改 EventEnvelope,老条目(翻译层上线前记录的)也能翻。
//
// 【红线,按方案原文钉在这里】
//   - 翻译是摘要不是裁决:翻译结论与原文冲突时,一律以 raw_json 原文为准
//     (每条 Detail 都带这句话,不是仅注释约定);
//   - 绝不编造载荷里没有的事实:含义识别不出就留空,意图判断不了就写
//     "意图不明,需要人工看原文";
//   - 输入 payload 已脱敏,翻译过程只消化输入、不引入任何新敏感信息
//     (纯函数:输出 ⊆ 输入 + 词表措辞,由 event_translate_test 反证钉住)。
//
// 【为什么读时翻译而不是登记时算好存起来】模板会持续扩容:存下来的翻译会
// 停在登记那一刻的词表版本,读时翻译则让老条目永远吃到最新措辞 —— 这正是
// "不只是术语"的需求在时间维度上的样子。
package runtime

import (
	"encoding/json"
	"sort"
	"strings"
)

// EventField 是字段级透析表的一行(对应 proto UnknownEventField)。
type EventField struct {
	// Key 字段名原文(不改写、不翻译 —— 名字本身是排查线索)。
	Key string
	// Kind 值类型:text / number / bool / array / object / null。
	Kind string
	// Preview 脱敏预览,超过 60 字截断(值本身已由上游脱敏,此处只截断不加工)。
	Preview string
	// Meaning 字段名可识别时的中文含义;识别不出留空 —— 不猜。
	Meaning string
}

// Translation 是一条事件的完整人话翻译。
type Translation struct {
	// Summary 一句话(≤60 字左右),列表页直接展示;不含方法名术语。
	Summary string
	// Detail 多句详解:语义解读 + daemon 处置 + 红线声明。
	Detail string
	// Fields 字段级透析表;载荷可拆则给(已知事件同样给),不可拆为空。
	Fields []EventField
}

// previewMaxRunes 是单字段预览的长度上限(中文字按 rune 计)。
const previewMaxRunes = 60

// detailFieldLimit 是 Detail 里展开透析表的行数上限:超出部分如实说明"其余
// 见原文",防止一个 200 字段的载荷把详情页顶爆(字段全表仍完整给到 Digest)。
const detailFieldLimit = 12

// eventKnowledgeKey 用 agent 与 method 拼知识库主键。
// 【为什么用 \x1f 分隔】method 本身含 "/" 甚至可能含 ":"(codex item、dsh 变体),
// 用可打印字符做分隔符迟早撞出假命中;控制字符做键分隔是编译器与人都读不错的写法。
func eventKnowledgeKey(agent, method string) string {
	return agent + "\x1f" + method
}

// knownEventSummaries 是已知方法的知识库:agent\x1fmethod → 一句话人话。
//
// 【条目来源】各 provider 分类器 + 事件对照表的注释(agent 包):
// codexGlobalNoticeKind / codexMethodNoticeKind / codexItemNoticeKind
// (codex.go、native_notifications.go)、piNoticeKind(pi.go)、
// zcodeNoticeKind(zcode.go)、dshNoticeKind(dsh.go 的事件对照表)。
// 词表扩容 = 在这里加一行,翻译层零改动 —— 这就是"同一框架"的扩展方式。
var knownEventSummaries = func() map[string]string {
	m := make(map[string]string)
	for _, src := range []map[string]string{
		codexEventSummaries, piEventSummaries, zcodeEventSummaries, dshEventSummaries,
	} {
		for k, v := range src {
			if _, dup := m[k]; dup {
				panic("event_translate: 重复词表键 " + k)
			}
			m[k] = v
		}
	}
	return m
}()

// knownEventPrefixes 是"方法带运行时后缀变体"的前缀知识库(最长前缀优先)。
// 【为什么单独一张表】dsh 的通知 method 形如 session/update:tool_call:completed,
// knownEventPrefixes(dsh 前缀词表)驻 knowledge_dsh.go。

// fieldMeaningGlossary 是字段名 → 中文含义的对照表。
// 【为什么只收通用词】领域词(如 codex 的 codexErrorInfo)给错含义比不给更糟;
// 匹配规则是归一化后**精确相等**,识别不出留空 —— "不猜"是方案红线。
var fieldMeaningGlossary = map[string]string{
	"id": "标识符", "threadid": "会话标识", "turnid": "轮次标识",
	"sessionid": "会话标识", "requestid": "请求标识", "correlationid": "关联标识",
	"count": "数量", "total": "总数", "progress": "进度", "status": "状态",
	"error": "错误信息", "message": "消息内容", "items": "条目列表",
	"tokens": "token 用量", "tokencount": "token 用量", "usage": "用量统计",
	"input": "输入", "output": "输出", "timestamp": "时间戳",
	"name": "名称", "title": "标题", "path": "文件路径", "cwd": "工作目录",
	"kind": "类型", "type": "类型", "model": "模型", "modelid": "模型标识",
	"tool": "工具", "reason": "原因", "success": "是否成功", "result": "结果",
	"data": "数据", "text": "正文内容", "delta": "增量内容", "snapshot": "状态快照",
	"version": "版本", "pid": "进程号",
	// 脱敏表里的凭据字段:含义如实标注"已脱敏",值本身是 [redacted] 占位。
	"token": "凭据(已脱敏)", "apikey": "凭据(已脱敏)", "secret": "凭据(已脱敏)",
	"password": "凭据(已脱敏)", "authorization": "凭据(已脱敏)",
}

// TranslateEvent 把一条事件翻成人话。纯函数:不动台账、不动信封、不产出副作用。
//
// kind 建议传台账里登记的 NormalizedKind:它决定 Detail 里的处置描述;
// 传空时不写处置句(宁可少说,不说错)。
func TranslateEvent(sourceAgent, method, kind, redactedPayload string) Translation {
	payload := strings.TrimSpace(redactedPayload)

	// ---- 第一步:知识库查已知方法(已知事件同样做字段透析,Detail 里给关键字段) ----
	summary, known := knownEventSummaries[eventKnowledgeKey(sourceAgent, method)]
	if !known {
		summary, known = matchKnownPrefix(sourceAgent, method)
	}

	// ---- 第二步:载荷能拆就拆(已知/未知通用) ----
	fields, structure := dissectPayload(payload)

	// ---- 第三步:组装 Summary ----
	tr := Translation{Fields: fields}
	if known {
		tr.Summary = summary
	} else {
		tr.Summary = unknownSummary(structure, fields)
	}

	// ---- 第四步:组装 Detail:语义/意图 + 处置 + 字段行 + 红线 ----
	var b strings.Builder
	if known {
		b.WriteString(tr.Summary + "。")
	} else {
		b.WriteString(intentClause(fields, structure))
	}
	if clause := disposalClause(kind); clause != "" {
		b.WriteString(clause)
	}
	if n := len(fields); n > 0 {
		b.WriteString("字段透析:")
		limit := n
		if limit > detailFieldLimit {
			limit = detailFieldLimit
		}
		for _, f := range fields[:limit] {
			b.WriteString(fieldLine(f))
		}
		if n > limit {
			b.WriteString("(其余 " + itoa(n-limit) + " 个字段见原文)")
		}
	}
	b.WriteString("【本翻译是摘要不是裁决:与原文冲突时,以 raw_json 原文为准。】")
	tr.Detail = b.String()
	return tr
}

// matchKnownPrefix 前缀匹配带变体后缀的方法(dsh 的 session/update:* 等),最长优先。
func matchKnownPrefix(agent, method string) (string, bool) {
	best, bestLen := "", 0
	for _, e := range knownEventPrefixes {
		if e.agent == agent && strings.HasPrefix(method, e.methodPrefix) && len(e.methodPrefix) > bestLen {
			best, bestLen = e.summary, len(e.methodPrefix)
		}
	}
	if bestLen == 0 {
		return "", false
	}
	return best, true
}

// payloadStructure 描述载荷的整体形态,供 Summary 与 Detail 如实描述。
type payloadStructure struct {
	form string // "object" / "array" / "scalar" / "text" / "empty"
	n    int    // object 的字段数或 array 的元素数(拆不出时为 0)
	text string // text/scalar 形态下的原文预览(截断)
	// nestedKeys 是嵌套结构内部的字段名(不含顶层):启发式意图要看全,
	// `{"detail":{"fail":true}}` 与 `{"fail":true}` 一样是错误信号。
	nestedKeys []string
}

// dissectPayload 拆解载荷:顶层对象逐字段成表,其余形态只记录整体描述。
func dissectPayload(payload string) ([]EventField, payloadStructure) {
	if payload == "" {
		return nil, payloadStructure{form: "empty"}
	}
	if payload[0] != '{' && payload[0] != '[' {
		// JSON 标量(如 `42`)与非 JSON 文本在这里区分:前者是合法 JSON。
		if isJSONValue(payload) {
			return nil, payloadStructure{form: "scalar", text: clipPreview(payload)}
		}
		return nil, payloadStructure{form: "text", text: clipPreview(payload)}
	}
	if payload[0] == '[' {
		var arr []json.RawMessage
		if json.Unmarshal([]byte(payload), &arr) != nil {
			return nil, payloadStructure{form: "text", text: clipPreview(payload)}
		}
		return nil, payloadStructure{form: "array", n: len(arr), nestedKeys: collectNestedKeys(arr)}
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal([]byte(payload), &obj) != nil {
		return nil, payloadStructure{form: "text", text: clipPreview(payload)}
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	// 排序让透析表顺序稳定可测;字段名原文不改写。
	sort.Strings(keys)
	fields := make([]EventField, 0, len(keys))
	var nested []string
	for _, k := range keys {
		fields = append(fields, EventField{
			Key:     k,
			Kind:    jsonKindOf(obj[k]),
			Preview: jsonPreviewOf(obj[k]),
			Meaning: meaningOf(k),
		})
		// 嵌套结构的字段名交给意图启发式(透析表仍只列顶层,详情以原文为准)。
		var v any
		if json.Unmarshal(obj[k], &v) == nil {
			nested = append(nested, walkKeys(v, 0)...)
		}
	}
	return fields, payloadStructure{form: "object", n: len(obj), nestedKeys: nested}
}

// collectNestedKeys 收集顶层数组内各元素的字段名(意图启发式用)。
func collectNestedKeys(arr []json.RawMessage) []string {
	var keys []string
	for _, item := range arr {
		var v any
		if json.Unmarshal(item, &v) == nil {
			keys = append(keys, walkKeys(v, 0)...)
		}
	}
	return keys
}

// walkKeys 递归收集嵌套字段名;depth 有上限,深嵌套只说明载荷复杂,不缺这些名字。
func walkKeys(v any, depth int) []string {
	const maxWalkDepth = 6
	if depth >= maxWalkDepth {
		return nil
	}
	switch x := v.(type) {
	case map[string]any:
		var keys []string
		for k, child := range x {
			keys = append(keys, k)
			keys = append(keys, walkKeys(child, depth+1)...)
		}
		return keys
	case []any:
		var keys []string
		for _, child := range x {
			keys = append(keys, walkKeys(child, depth+1)...)
		}
		return keys
	}
	return nil
}

// unknownSummary 给未知方法组装一句话:说清形态 + 如实的意图判断。
func unknownSummary(st payloadStructure, fields []EventField) string {
	intent := intentPhrase(fields, st)
	switch st.form {
	case "empty":
		return "收到一条未知事件,没有携带内容"
	case "object":
		if st.n == 0 {
			return "收到一条未知事件,载荷是空对象" + intent
		}
		return "收到一条未知事件,载荷含 " + itoa(st.n) + " 个字段" + intent
	case "array":
		return "收到一条未知事件,载荷是一个数组,共 " + itoa(st.n) + " 项" + intent
	case "scalar":
		return "收到一条未知事件,载荷是一个 JSON 单值(" + st.text + ")" + intent
	default:
		return "收到一条未知事件,内容是一段文本(" + st.text + ")" + intent
	}
}

// intentPhrase 由启发式给意图短语;判断不了就明说"意图不明"。
func intentPhrase(fields []EventField, st payloadStructure) string {
	keys := append(allFieldNames(fields), st.nestedKeys...)
	switch {
	case containsAnyKey(keys, "error", "fail"):
		return ",看起来像一次错误上报"
	case containsAnyKey(keys, "progress", "count", "items", "total"):
		return ",可能是进度或列表类上报"
	default:
		return ",意图不明,需要人工看原文"
	}
}

// intentClause 是 Detail 里的意图段落(与 Summary 的短语同一套判断,展开说)。
func intentClause(fields []EventField, st payloadStructure) string {
	keys := append(allFieldNames(fields), st.nestedKeys...)
	var b strings.Builder
	b.WriteString("daemon 没有见过这个方法,以下是对载荷的客观拆解。")
	switch {
	case containsAnyKey(keys, "error", "fail"):
		b.WriteString("载荷里带有 error/fail 类字段,看起来像一次错误上报;")
	case containsAnyKey(keys, "progress", "count", "items", "total"):
		b.WriteString("载荷里带有 progress/count/items 类字段,可能是进度或列表类上报;")
	default:
		b.WriteString("凭字段名判断不了它的用途,意图不明,需要人工看原文;")
	}
	if st.form == "text" && st.text != "" {
		b.WriteString("原文预览:" + st.text + "。")
	}
	return b.String()
}

// disposalClause 按 normalized_kind 写明 daemon 的处置(方案:Detail 里写处置)。
// kind 为空或词表外时不写 —— 宁可少一句,不写错一句。
func disposalClause(kind string) string {
	switch kind {
	case KindKnownProjected:
		return "处置:daemon 已把这条事件投影成协议内的正式消息,手机端应已收到对应内容,此处留痕仅供核对。"
	case KindKnownPassthrough:
		return "处置:daemon 认识这条事件但没有正式投影,只原样透传留痕,不影响主流程。"
	case KindUnknownNotification:
		return "处置:daemon 没见过这条通知,已留痕登记,未影响主流程;若反复出现,建议把本页原文反馈给开发者。"
	case KindUnknownRequest:
		return "处置:这是一条 daemon 不认识的反向请求,按安全规则未默认执行,已回协议错误拒绝。"
	case KindUnknownResponse:
		return "处置:这是一条没能对应到任何在途请求的响应,已留痕,不影响进行中的任务。"
	}
	return ""
}

// fieldLine 把一行透析表渲染成 Detail 里的句子片段。
func fieldLine(f EventField) string {
	line := " " + f.Key + "(" + f.Kind + "): " + f.Preview
	if f.Meaning != "" {
		line += " —— " + f.Meaning + ";"
	} else {
		line += ";"
	}
	return line
}

// allFieldNames 递归收集字段名(含嵌套),供启发式意图判断;深度有上限防深嵌套。
func allFieldNames(fields []EventField) []string {
	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.Key)
	}
	return keys
}

// containsAnyKey 判断字段名集合(归一化后)是否含任一关键词的子串。
// 【为什么是子串不是全等】error 类字段常叫 lastError / errorCode;这里只是
// 启发式(结论措辞是"看起来像"),子串匹配的召回收益大于误报成本。
func containsAnyKey(keys []string, words ...string) bool {
	for _, k := range keys {
		norm := normalizeKey(k)
		for _, w := range words {
			if strings.Contains(norm, w) {
				return true
			}
		}
	}
	return false
}

// normalizeKey 把字段名归一成小写、去下划线/连字符的形式(对照表按此匹配)。
func normalizeKey(k string) string {
	k = strings.ToLower(k)
	k = strings.ReplaceAll(k, "_", "")
	return strings.ReplaceAll(k, "-", "")
}

// meaningOf 查字段含义;识别不出返回空 —— 不猜(方案红线)。
func meaningOf(key string) string {
	return fieldMeaningGlossary[normalizeKey(key)]
}

// jsonKindOf 按首字节判值类型(入参必是合法 JSON 值的字节)。
func jsonKindOf(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return "null"
	}
	switch s[0] {
	case '"':
		return "text"
	case '{':
		return "object"
	case '[':
		return "array"
	case 't', 'f':
		return "bool"
	case 'n':
		return "null"
	default:
		return "number"
	}
}

// jsonPreviewOf 生成脱敏预览:字符串值去引号取原文;结构/标量取紧凑原文。
// 只截断、不加工 —— 值已在登记前脱敏,翻译层再"整理"反而可能引入歧义。
func jsonPreviewOf(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if jsonKindOf(raw) == "text" {
		var unquoted string
		if json.Unmarshal(raw, &unquoted) == nil {
			s = unquoted
		}
	}
	return clipPreview(s)
}

// clipPreview 截到 previewMaxRunes 个字符(超长取前 N-1 字加省略号,总数不超限)。
func clipPreview(s string) string {
	runes := []rune(s)
	if len(runes) <= previewMaxRunes {
		return s
	}
	return string(runes[:previewMaxRunes-1]) + "…"
}

// isJSONValue 判断一段文本是否为合法 JSON 标量(数字/布尔/null)。
func isJSONValue(s string) bool {
	var v any
	return json.Unmarshal([]byte(s), &v) == nil
}

// itoa 免依赖的最小整数转字符串(本文件只需要非负小整数)。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// TranslateEnvelope 是台账信封的人话翻译入口(读时翻译):从信封取来源/方法/类别/脱敏
// 载荷,走 TranslateEvent 同一条纯函数路径。不改信封、不写台账 —— 老条目(翻译层上线
// 前登记的)同样能翻。
//
// 【它与 PresentUnknownEvent 的分工】本入口服务"技术详情"页:逐字段透析 + 处置句 +
// 原文红线;PresentUnknownEvent(unknown_action.go)服务用户处理中心的卡片,按 kind 套
// 固定文案。两条入口互不依赖、可以并存:一个是拆开读,一个是说人话,都不做裁决。
func TranslateEnvelope(env EventEnvelope) Translation {
	return TranslateEvent(env.SourceAgent, env.SourceMethod, env.NormalizedKind, env.RawPayload)
}
