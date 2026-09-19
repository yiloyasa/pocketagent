package zcodeapp

// generate_test.go —— GenerateText 封装的行为钉子(fake,不拉子进程)。
// 形状依据 2026-09-19 live 实测(zcode.cjs jKe schema):
//   1) 先 session/create(显式 build)拿模型快照;候选 = 桌面 current 优先,
//      available 按序补位(去重,封顶 3 个);
//   2) 每候选一次 workspace/generateText:workspace{workspacePath,workspaceKey} +
//      selection{providerId,modelId,options?{reasoningLevel}} + querySource +
//      prompt + maxOutputTokens + operationId;应答顶层 text;
//   3) 候选失败(含超时)→ 先 cancelGenerateText(同 operationId)再换下一个;
//   4) 全部失败 → 最后一个错误原文透传;模型表为空 → 报错且不发生成请求;
//   5) parseGenerateTextResult 的形状容错表;
//   6) 模型偏好记忆(建议反馈 1):成功落盘、下次置顶、失败不清偏好。
//
// 驱动方式与 client_test.go 相同:内存 stdin + 手动 dispatch 应答帧。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain 把模型偏好文件重定向到临时目录:偏好记忆上线后,GenerateText 成功
// 会写 ~/.pocketagentd/model-pref.json、生成前会读它 —— 单测绝不读写真实用户
// 目录,也不让本机真实偏好串进来改变候选顺序(否则测试相互污染)。
func TestMain(m *testing.M) {
	modelPrefFile = filepath.Join(os.TempDir(),
		fmt.Sprintf("pa-generate-pref-test-%d", time.Now().UnixNano()), "model-pref.json")
	os.Exit(m.Run())
}

const snapshotTwoModels = `{"settings":{"model":{
	"current":{"providerId":"p-cur","modelId":"m-cur","options":{"reasoningLevel":"high"}},
	"available":[
		{"ref":{"providerId":"p-cur","modelId":"m-cur"}},
		{"ref":{"providerId":"p1","modelId":"m1"},"reasoning":{"defaultLevel":"mid"}},
		{"ref":{"providerId":"p2","modelId":"m2"},"reasoning":{"defaultLevel":"max"}}]}}}`

func TestGenerateTextUsesDesktopCurrentModel(t *testing.T) {
	c, buf := newTestClient()
	type out struct {
		text string
		err  error
	}
	ch := make(chan out, 1)
	go func() {
		s, err := c.GenerateText(context.Background(), "把这句话改清楚")
		ch <- out{s, err}
	}()

	// 第一帧:session/create(红线:显式 build)→ 回快照,current 是桌面当前模型
	create := waitFrame(t, buf, "session/create")
	if params, _ := create["params"].(map[string]interface{}); params["mode"] != "build" {
		t.Fatalf("session/create 必须显式 build, got %v", create["params"])
	}
	replyOK(c, create, snapshotTwoModels)

	// 第二帧:workspace/generateText,参数必须符合实证 schema
	gen := waitFrame(t, buf, "workspace/generateText")
	p, ok := gen["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("generateText 缺 params: %v", gen)
	}
	ws, _ := p["workspace"].(map[string]interface{})
	if ws["workspacePath"] == "" || ws["workspacePath"] != ws["workspaceKey"] {
		t.Fatalf("workspace 形状不对: %v", p["workspace"])
	}
	sel, _ := p["selection"].(map[string]interface{})
	if sel["providerId"] != "p-cur" || sel["modelId"] != "m-cur" {
		t.Fatalf("应优先桌面 current 模型, got %v", p["selection"])
	}
	if p["querySource"] == "" {
		t.Fatalf("缺 querySource: %v", p)
	}
	// prompt 必须携带原文,且带输出纪律收尾(输出纯净度迭代:指令尾双保险)
	promptStr, _ := p["prompt"].(string)
	if !strings.HasPrefix(promptStr, "把这句话改清楚") {
		t.Fatalf("prompt 应以原文开头, got %q", promptStr)
	}
	if !strings.Contains(promptStr, "输出纪律") || !strings.Contains(promptStr, "只提供一个方案") {
		t.Fatalf("prompt 应追加输出纪律收尾段, got %q", promptStr)
	}
	if _, ok := p["maxOutputTokens"].(float64); !ok {
		t.Fatalf("应显式带 maxOutputTokens, got %v", p)
	}
	if p["operationId"] == "" {
		t.Fatalf("应带 operationId(配对 cancel), got %v", p)
	}
	replyOK(c, gen, `{"text":"改写完成","finishReason":"stop"}`)

	r := <-ch
	if r.err != nil {
		t.Fatalf("不该报错: %v", r.err)
	}
	if r.text != "改写完成" {
		t.Fatalf("应返回顶层 text, got %q", r.text)
	}
	// 成功路径不该有取消调用
	if strings.Contains(buf.String(), "cancelGenerateText") {
		t.Fatalf("成功路径不应调 cancelGenerateText: %s", buf.String())
	}
}

func TestGenerateTextFallsBackToNextCandidate(t *testing.T) {
	c, buf := newTestClient()
	type out struct {
		text string
		err  error
	}
	ch := make(chan out, 1)
	go func() {
		s, err := c.GenerateText(context.Background(), "x")
		ch <- out{s, err}
	}()

	create := waitFrame(t, buf, "session/create")
	replyOK(c, create, snapshotTwoModels)

	// 候选 1(current)失败:模拟服务端把挂死的模型请求取消掉
	gen1 := waitFrame(t, buf, "workspace/generateText")
	sel1, _ := gen1["params"].(map[string]interface{})["selection"].(map[string]interface{})
	if sel1["providerId"] != "p-cur" {
		t.Fatalf("候选 1 应是 current, got %v", sel1)
	}
	op1, _ := gen1["params"].(map[string]interface{})["operationId"].(string)
	replyErr(c, gen1, "Model request was cancelled.")

	// 失败后必须显式取消服务端残留(operationId 配对);cancel 也要回包
	cancelFrame := waitFrame(t, buf, "workspace/cancelGenerateText")
	if cancelFrame["params"].(map[string]interface{})["operationId"] != op1 {
		t.Fatalf("cancel 的 operationId 应与失败请求配对: %v", cancelFrame["params"])
	}
	replyOK(c, cancelFrame, `{"operationId":"op","cancelled":true}`)

	// 候选 2 = available 第一个与 current 不同者,成功
	gen2 := waitFrameN(t, buf, "workspace/generateText", 2)
	sel2, _ := gen2["params"].(map[string]interface{})["selection"].(map[string]interface{})
	if sel2["providerId"] != "p1" || sel2["modelId"] != "m1" {
		t.Fatalf("候选 2 应是 available 里第一个未试过的, got %v", sel2)
	}
	if sel2["options"].(map[string]interface{})["reasoningLevel"] != "mid" {
		t.Fatalf("候选 2 应带 defaultLevel, got %v", sel2)
	}
	replyOK(c, gen2, `{"text":"ok"}`)

	r := <-ch
	if r.err != nil {
		t.Fatalf("候选 2 成功就不该报错: %v", r.err)
	}
	if r.text != "ok" {
		t.Fatalf("got %q", r.text)
	}
}

func TestGenerateTextAllCandidatesFailSurfacesLastError(t *testing.T) {
	c, buf := newTestClient()
	errCh := make(chan error, 1)
	go func() {
		_, err := c.GenerateText(context.Background(), "x")
		errCh <- err
	}()

	create := waitFrame(t, buf, "session/create")
	replyOK(c, create, snapshotTwoModels)

	// 3 个候选全部失败(封顶 generateTextMaxCandidates,不无限换)
	for i := 1; i <= generateTextMaxCandidates; i++ {
		gen := waitFrameN(t, buf, "workspace/generateText", i)
		replyErr(c, gen, "Token Plan 用量上限")
		if i < generateTextMaxCandidates {
			waitCancelAndReply(t, buf, c, i) // 取消帧回包后再喂下一个失败
		}
	}

	err := <-errCh
	if err == nil {
		t.Fatal("全部候选失败必须报错")
	}
	// 配额类错误原文必须透传(server 层据此做人话分类)
	if !strings.Contains(err.Error(), "Token Plan 用量上限") {
		t.Fatalf("错误原文应透传: %v", err)
	}
}

func TestGenerateTextEmptyModelTableIsError(t *testing.T) {
	c, buf := newTestClient()
	errCh := make(chan error, 1)
	go func() {
		_, err := c.GenerateText(context.Background(), "x")
		errCh <- err
	}()

	create := waitFrame(t, buf, "session/create")
	replyOK(c, create, `{"settings":{"model":{"available":[]}}}`)

	err := <-errCh
	if err == nil {
		t.Fatal("模型表为空必须报错,否则下一步 generateText 必然被拒")
	}
	if !strings.Contains(err.Error(), "模型表为空") {
		t.Fatalf("错误应说明原因: %v", err)
	}
	if n := strings.Count(buf.String(), "workspace/generateText"); n != 0 {
		t.Fatalf("模型表为空就不该发生成请求, 发了 %d 帧", n)
	}
}

// ---- 帧等待辅助:轮询内存 stdin 缓冲,攒齐第 n 个指定方法的请求帧 ----

func waitFrame(t *testing.T, buf *syncBuffer, method string) map[string]interface{} {
	t.Helper()
	return waitFrameN(t, buf, method, 1)
}

func waitFrameN(t *testing.T, buf *syncBuffer, method string, n int) map[string]interface{} {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		count := 0
		for _, line := range strings.Split(buf.String(), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var m map[string]interface{}
			if json.Unmarshal([]byte(line), &m) != nil || m["method"] != method {
				continue
			}
			count++
			if count == n {
				return m
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等第 %d 个 %s 帧超时,已见: %s", n, method, buf.String())
	return nil
}

// waitCancelAndReply 等第 n 个 cancelGenerateText 帧并回包(与生产调用同步)。
func waitCancelAndReply(t *testing.T, buf *syncBuffer, c *Client, n int) {
	t.Helper()
	frame := waitFrameN(t, buf, "workspace/cancelGenerateText", n)
	replyOK(c, frame, `{"operationId":"op","cancelled":true}`)
}

// replyOK / replyErr 往 pending 表里投递应答帧(id 从请求帧原样取回)。
func replyOK(c *Client, frame map[string]interface{}, result string) {
	id, _ := frame["id"].(float64)
	c.dispatch(&wireMsg{
		ID:     json.RawMessage(fmt.Sprintf("%.0f", id)),
		Result: json.RawMessage(result),
	})
}

func replyErr(c *Client, frame map[string]interface{}, msg string) {
	id, _ := frame["id"].(float64)
	var errFrame wireMsg
	line := fmt.Sprintf(`{"id":%.0f,"error":{"code":-32000,"message":%q}}`, id, msg)
	if err := json.Unmarshal([]byte(line), &errFrame); err != nil {
		panic(err) // 测试自身拼帧出错,直接炸
	}
	c.dispatch(&errFrame)
}

func TestParseGenerateTextResultShapes(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`{"text":"你好"}`, "你好"},
		{`{"result":"R"}`, "R"},
		{`{"content":"C"}`, "C"},
		{`{"output":"O"}`, "O"},
		{`{"text":"","result":"R"}`, "R"}, // 空字段跳过取下一个
		{`"整段就是字符串"`, "整段就是字符串"},          // result 本身是 JSON 字符串
		{`{"message":{"text":"嵌套"}}`, ""}, // 嵌套形状不支持:返回空,上层报错
		{`{}`, ""},                        // 无字段:空
	}
	for i, c := range cases {
		got, err := parseGenerateTextResult([]byte(c.raw))
		if err != nil {
			t.Fatalf("case %d: 不该报错: %v", i, err)
		}
		if got != c.want {
			t.Fatalf("case %d: got %q want %q", i, got, c.want)
		}
	}
	if got, err := parseGenerateTextResult(nil); err != nil || got != "" {
		t.Fatalf("nil 输入应返回空串无错: got %q err %v", got, err)
	}
}

// TestOptimizeOutputDisciplineStyles 钉输出纪律文案:三档风格共享纪律本体
// (只输出终稿/无前言/无 Markdown/单方案/保原意),concise 与 detailed 的
// 风格侧重行互不相同,且默认档与两者都不同。
func TestOptimizeOutputDisciplineStyles(t *testing.T) {
	mustContain := []string{
		"只输出改写后的最终文本",
		"前言",
		"Markdown",
		"只提供一个方案",
		"保留用户原意",
	}
	concise := OptimizeOutputDiscipline("concise")
	detailed := OptimizeOutputDiscipline("detailed")
	def := OptimizeOutputDiscipline("")
	unknown := OptimizeOutputDiscipline("  FANCY ") // 未知风格回落默认档
	for _, v := range []string{concise, detailed, def} {
		for _, kw := range mustContain {
			if !strings.Contains(v, kw) {
				t.Fatalf("纪律文案缺关键词 %q: %q", kw, v)
			}
		}
	}
	if !strings.Contains(concise, "压缩成一句话") || concise == detailed {
		t.Fatalf("concise 档应有压缩侧重且异于 detailed: %q", concise)
	}
	if !strings.Contains(detailed, "要点列表") {
		t.Fatalf("detailed 档应强调要点列表: %q", detailed)
	}
	if def != unknown {
		t.Fatalf("未知风格应回落默认档")
	}
	// 注入路径:原文在前、纪律收尾在后,内容与默认档一致
	wrapped := withOutputDiscipline("原始指令")
	if !strings.HasPrefix(wrapped, "原始指令\n\n") || !strings.HasSuffix(wrapped, def) {
		t.Fatalf("注入形状应为 原文+空行+默认纪律, got %q", wrapped)
	}
}

// TestSanitizeOptimizedOutput 钉出口净化:剥围栏与 {"optimized":...} 解包,
// 其余一切输出(含无法无损判定的前言混排)原样保留,绝不启发式截取。
func TestSanitizeOptimizedOutput(t *testing.T) {
	cases := []struct{ raw, want string }{
		// 正常输出原样
		{"把登录按钮改成加载两秒后提交,并给出验证方式", "把登录按钮改成加载两秒后提交,并给出验证方式"},
		{"第一行\n第二行", "第一行\n第二行"},
		// 围栏包裹:剥围栏(带/不带语言标签、多行正文)
		{"```text\n改写后的文本\n```", "改写后的文本"},
		{"```\n改写后的文本\n```", "改写后的文本"},
		{"```markdown\n- 要点一\n- 要点二\n```", "- 要点一\n- 要点二"},
		// JSON 包裹:整段才解包
		{`{"optimized":"改写后的文本"}`, "改写后的文本"},
		{"```json\n{\"optimized\":\"改写后的文本\"}\n```", "改写后的文本"},
		{"{\n  \"optimized\": \"第一行\\n第二行\"\n}", "第一行\n第二行"},
		// 容错:不动原样返回
		{`{"optimized":""}`, `{"optimized":""}`},   // 空 optimized 不解包
		{`{"optimized":123}`, `{"optimized":123}`}, // 非字符串不解包
		{`{"other":"x"}`, `{"other":"x"}`},         // 缺 optimized 字段
		{"```\n只有开头围栏", "```\n只有开头围栏"},             // 围栏不完整:不动
		{"以下是结果:\n```json\n{\"optimized\":\"x\"}\n```", "以下是结果:\n```json\n{\"optimized\":\"x\"}\n```"}, // 前言混排:无法无损判定,不动
	}
	for i, c := range cases {
		if got := sanitizeOptimizedOutput(c.raw); got != c.want {
			t.Fatalf("case %d: got %q want %q", i, got, c.want)
		}
	}
	// 与 parse 串联:应答顶层 text 里带围栏时,attemptGenerate 的出口也走净化
	if got := sanitizeOptimizedOutput("  ```text\n去空白再剥围栏\n```  "); got != "去空白再剥围栏" {
		t.Fatalf("首尾空白应先剥: got %q", got)
	}
}

// ---- 模型偏好记忆(建议反馈 1):成功落盘 / 下次首选 / 失败不清偏好 ----

// readPrefFile 读偏好文件原文(测试校验用);不存在返回空串。
func readPrefFile(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(modelPrefFile)
	if err != nil {
		return ""
	}
	return string(raw)
}

// TestGenerateTextSavesPreferredModelOnSuccess 场景 1:成功后偏好落盘。
// 无任何预置记忆 → 走原候选序列(current 优先),成功的是 current,
// 落盘的偏好就必须是 current 的 provider/model。
func TestGenerateTextSavesPreferredModelOnSuccess(t *testing.T) {
	// 干净起点:上一次测试的落盘不许串进来
	_ = os.Remove(modelPrefFile)
	c, buf := newTestClient()
	ch := make(chan struct{}, 1)
	go func() {
		_, _ = c.GenerateText(context.Background(), "x")
		ch <- struct{}{}
	}()

	create := waitFrame(t, buf, "session/create")
	replyOK(c, create, snapshotTwoModels)
	gen := waitFrame(t, buf, "workspace/generateText")
	replyOK(c, gen, `{"text":"ok"}`)
	<-ch

	var pref modelPref
	if err := json.Unmarshal([]byte(readPrefFile(t)), &pref); err != nil {
		t.Fatalf("成功后偏好应落盘且可解析: %v", err)
	}
	if pref.ProviderID != "p-cur" || pref.ModelID != "m-cur" {
		t.Fatalf("偏好应是成功候选 p-cur/m-cur, got %v", pref)
	}
	if pref.UpdatedAtUnixMs <= 0 {
		t.Fatalf("应带时间戳: %v", pref)
	}
}

// TestGenerateTextPromotesLastGoodModel 场景 2:下次首选。
// 预置记忆 = p2/m2(快照 available 里的最后一个):置顶后首个生成请求就该打它,
// 不再先撞挂死的桌面 current;reasoningLevel 取快照现值(max),不用记忆旧档。
func TestGenerateTextPromotesLastGoodModel(t *testing.T) {
	_ = os.MkdirAll(filepath.Dir(modelPrefFile), 0o755)
	prefJSON := `{"providerId":"p2","modelId":"m2","updatedAtUnixMs":1}`
	if err := os.WriteFile(modelPrefFile, []byte(prefJSON), 0o644); err != nil {
		t.Fatalf("预置偏好失败: %v", err)
	}

	c, buf := newTestClient()
	ch := make(chan struct{}, 1)
	go func() {
		_, _ = c.GenerateText(context.Background(), "x")
		ch <- struct{}{}
	}()

	create := waitFrame(t, buf, "session/create")
	replyOK(c, create, snapshotTwoModels)

	gen := waitFrame(t, buf, "workspace/generateText")
	sel, _ := gen["params"].(map[string]interface{})["selection"].(map[string]interface{})
	if sel["providerId"] != "p2" || sel["modelId"] != "m2" {
		t.Fatalf("有记忆时首选应是上次成功的 p2/m2, got %v", sel)
	}
	// 档位用快照现值:available 里 p2 的 defaultLevel 是 max
	if opts, _ := sel["options"].(map[string]interface{}); opts["reasoningLevel"] != "max" {
		t.Fatalf("reasoningLevel 应取快照现值 max, got %v", sel)
	}
	replyOK(c, gen, `{"text":"ok"}`)
	<-ch
}

// TestGenerateTextPromoteFallsBackWhenPrefMissingFromSnapshot 场景 2b:
// 记忆里的模型已不在快照(下架/换号)→ 原样走原候选序列,不惩罚。
func TestGenerateTextPromoteFallsBackWhenPrefMissingFromSnapshot(t *testing.T) {
	_ = os.MkdirAll(filepath.Dir(modelPrefFile), 0o755)
	if err := os.WriteFile(modelPrefFile,
		[]byte(`{"providerId":"gone","modelId":"gone-model","updatedAtUnixMs":1}`), 0o644); err != nil {
		t.Fatalf("预置偏好失败: %v", err)
	}

	c, buf := newTestClient()
	ch := make(chan struct{}, 1)
	go func() {
		_, _ = c.GenerateText(context.Background(), "x")
		ch <- struct{}{}
	}()

	create := waitFrame(t, buf, "session/create")
	replyOK(c, create, snapshotTwoModels)
	gen := waitFrame(t, buf, "workspace/generateText")
	sel, _ := gen["params"].(map[string]interface{})["selection"].(map[string]interface{})
	if sel["providerId"] != "p-cur" {
		t.Fatalf("记忆失效应回退原候选序列(current 优先), got %v", sel)
	}
	replyOK(c, gen, `{"text":"ok"}`)
	<-ch
}

// TestGenerateTextFailureKeepsPreference 场景 3:失败不清偏好。
// 有记忆(p2/m2)但三个候选全失败:偏好文件必须原样保留 —— 挂死是暂时的,
// 记忆是"上次成功"的事实,失败不构成推翻它的理由。
func TestGenerateTextFailureKeepsPreference(t *testing.T) {
	_ = os.MkdirAll(filepath.Dir(modelPrefFile), 0o755)
	prefJSON := `{"providerId":"p2","modelId":"m2","updatedAtUnixMs":42}`
	if err := os.WriteFile(modelPrefFile, []byte(prefJSON), 0o644); err != nil {
		t.Fatalf("预置偏好失败: %v", err)
	}

	c, buf := newTestClient()
	errCh := make(chan error, 1)
	go func() {
		_, err := c.GenerateText(context.Background(), "x")
		errCh <- err
	}()

	create := waitFrame(t, buf, "session/create")
	replyOK(c, create, snapshotTwoModels)
	for i := 1; i <= generateTextMaxCandidates; i++ {
		gen := waitFrameN(t, buf, "workspace/generateText", i)
		replyErr(c, gen, "Model request was cancelled.")
		if i < generateTextMaxCandidates {
			waitCancelAndReply(t, buf, c, i)
		}
	}
	if err := <-errCh; err == nil {
		t.Fatal("全部候选失败应报错")
	}
	if got := readPrefFile(t); got != prefJSON {
		t.Fatalf("失败不得改动偏好:\n got %q\nwant %q", got, prefJSON)
	}
}

// TestPromotePreferredModel 钉置顶语义:匹配候选挪到队首、其余保持原相对顺序;
// 已在队首时零改动(不复制);找不到时原样返回。
func TestPromotePreferredModel(t *testing.T) {
	mk := func(p, m string) map[string]interface{} {
		return map[string]interface{}{"providerId": p, "modelId": m}
	}
	same := func(got []map[string]interface{}, want ...[2]string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("长度 = %d, want %d", len(got), len(want))
		}
		for i, w := range want {
			p, _ := got[i]["providerId"].(string)
			m, _ := got[i]["modelId"].(string)
			if p != w[0] || m != w[1] {
				t.Fatalf("位次 %d = %s/%s, want %s/%s", i, p, m, w[0], w[1])
			}
		}
	}
	cands := []map[string]interface{}{mk("a", "1"), mk("b", "2"), mk("c", "3")}

	// 中间挪到队首,其余保序
	same(promotePreferredModel(cands, "b", "2"), [2]string{"b", "2"}, [2]string{"a", "1"}, [2]string{"c", "3"})
	// 已在队首(a):顺序不变
	same(promotePreferredModel(cands, "a", "1"), [2]string{"a", "1"}, [2]string{"b", "2"}, [2]string{"c", "3"})
	// 找不到:原样
	same(promotePreferredModel(cands, "z", "9"), [2]string{"a", "1"}, [2]string{"b", "2"}, [2]string{"c", "3"})
}
