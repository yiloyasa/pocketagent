package zcodeapp

// generate.go —— workspace/generateText 的薄封装(对话栏提示词优化专用)。
//
// 【为什么放独立文件】ZcodeProvider 与协议层(client.go)归审批接入智能体所有,
// 本文件与其同包不同文件,只增不改 —— 谁的地盘谁施工,合并零冲突。
//
// 【参数/应答形状:2026-09-19 live 实证收敛】先前"先 text 后 prompt"的两试猜测
// 全被 -32602 拒绝;对真实 app-server 实测后确定唯一形状(zcode.cjs 内 jKe schema):
//
//	请求:
//	  workspace       {workspacePath, workspaceKey}  与 session/create 同款(strict)
//	  selection       {providerId, modelId,
//	                   options?{reasoningLevel}}     是"模型选择",不是编辑器选区!
//	  querySource     string                         来源标记,自报家门
//	  prompt          string                         要生成的内容(refine:与 messages 至少一个)
//	  maxOutputTokens int(可选,但缺省值可能超出模型档位,显式给小值更稳)
//	  operationId     string(可选)                  配对 cancelGenerateText 用
//	应答:
//	  {text, selection, finishReason?, usage?, toolCalls?} —— 顶层 text。
//
//	【选模:候选序列】跟随桌面当前模型优先(current 就是 selection 形状),
//	其后按 settings.model.available 逐个补位(去重)。为什么要候选:live 实测
//	桌面当前模型(minimax)服务端挂 60s 才被取消 —— 单候选会让功能在
//	"当前 provider 恰好坏了"的机器上整体不可用。每个候选 25s 预算,失败先
//	workspace/cancelGenerateText 显式掐掉服务端残留,再换下一个。
//
//	【选模:偏好记忆(提示词优化交付的建议反馈 1)】桌面 current 模型挂死时,
//	首次优化必先耗满一个候选预算(40s)才轮换到下一个候选 —— 且"挂死"通常
//	不会自己好,于是每次都要重交一遍学费。因此把"上次成功的模型"持久化到
//	~/.pocketagentd/model-pref.json:下次生成时若有记忆且该模型仍在快照里,
//	就把它置顶为首选。挂死过的机器从第二次优化起直接命中可用模型,省掉首个
//	坏候选的 40s。规则:只有成功才写偏好;失败不清偏好、降级链照旧;
//	reasoningLevel 用快照现值(候选对象整体沿用,不取记忆里的旧档位)。
//
//	【配额】不特判配额文案,错误原文透传,server 层做人话包装(UI 不炸)。
//
// 【取消】对用户主动取消,同样封装 workspace/cancelGenerateText 即可(接口面
// 已由 attemptOperationId/cancelGenerate 打通),暂无调用方,留作扩展点。
//
// 【输出纯净度(2026-09-19 迭代)】优化智能体反馈:模型不完全遵守"只输出改写
// 文本",会夹带前言/多方案 Markdown。两层兜底,都落在本文件(不动 server 层):
//  1. 指令侧:GenerateText 发请求前给 prompt 追加"输出纪律"收尾段
//     (withOutputDiscipline)——指令头(server 层 buildOptimizeInstruction)+
//     尾(这里)双保险,模型对收尾指令更敏感;
//  2. 出口侧:sanitizeOptimizedOutput 对违规输出做无损解包(剥代码围栏、
//     {"optimized": "..."} JSON 解包),只做可证明无损的处理,绝不启发式截取。
// 结构化约束(responseJsonSchema)已探明不可行:zcode.cjs 的
// zcodeWorkspaceGenerateTextParamsSchema(jKe)是 .strict() 且无该字段,
// 传了必被 -32602 拒;该参数只在 zcode 内部模型层存在,不经 RPC 暴露。

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// generateTextTimeout 整个生成流程(选模 + 至多 3 个候选)的硬上界。
	// 正常路径 live 实测 ~4s(轻任务);真实改写任务在 reasoning 档位高的模型上
	// 实测可超 25s,故单候选给 40s、整体只兜"逐个候选都慢"的最坏情形。
	generateTextTimeout = 130 * time.Second
	// generateTextAttemptBudget 单个候选模型的尝试上界。live 实测:真实改写
	// 任务在 reasoning=max 的模型上可超 25s;再慢的 provider 多半是挂了。
	generateTextAttemptBudget = 40 * time.Second
	// generateTextMaxCandidates 最多尝试的候选模型数。多了必然超时,没意义。
	generateTextMaxCandidates = 3
	// generateTextMaxOutputTokens 改写一段提示词用不了多少 token;显式给一个
	// 保守小值,避开"缺省值超出模型 option 范围"的 -32603(live 踩过)。
	generateTextMaxOutputTokens = 4096
	// generateTextQuerySource 自报家门的来源标记(zcode 侧仅透传记录,不作校验)。
	generateTextQuerySource = "pocketagent-prompt-optimize"
)

// GenerateText 调 workspace/generateText 生成一段文本(提示词优化的引擎入口)。
//
// ctx 为 nil 时用 background;无论传入什么,内部都再加硬上界 ——
// 与 CallCtx 的 callTimeout(120s)取更小者,保证"优化"这个高频轻量动作
// 不会被会话级的宽超时拖住。
//
// 【两步走】generateText 的 selection 需要具体的 provider/model,所以先
// session/create(红线:显式 build)拿模型快照,再发生成请求。两者都走
// 同一条连接;整个进程本来就是一次性拉起用完即关(prompt_opt.go)。
func (c *Client) GenerateText(ctx context.Context, text string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, generateTextTimeout)
	defer cancel()

	candidates, err := c.generateCandidates(ctx)
	if err != nil {
		return "", err
	}
	// 【偏好记忆】上次成功的模型若仍在候选里,置顶为首选(见文件头注释)。
	if pref := loadModelPref(); pref != nil {
		candidates = promotePreferredModel(candidates, pref.ProviderID, pref.ModelID)
	}
	log.Printf("[generate] 候选模型 %d 个:%v", len(candidates), candidateLabels(candidates))

	// 【输出纪律注入】追加收尾纪律段(默认风格行;zcodeapp 拿不到 server 层的
	// style 参数,风格侧重由指令头负责,这里只钉输出形态纪律)。所有候选共用,
	// 注入一次即可。
	text = withOutputDiscipline(text)

	var lastErr error
	for i, sel := range candidates {
		opID := fmt.Sprintf("pa-promptopt-%d-%d", time.Now().UnixNano(), i)
		got, err := c.attemptGenerate(ctx, sel, opID, text)
		if err == nil {
			// 成功才记偏好;失败不清偏好(下面的降级链照旧走完)。
			saveModelPref(sel)
			return got, nil
		}
		log.Printf("[generate] 候选 %d/%d %v 失败: %v", i+1, len(candidates), sel, err)
		lastErr = err
		// 失败(含超时)后服务端可能仍在生成:显式取消,不占着模型排队。
		cancelCtx, cancelCall := context.WithTimeout(ctx, 5*time.Second)
		_, _ = c.CallCtx(cancelCtx, "workspace/cancelGenerateText",
			map[string]interface{}{"operationId": opID})
		cancelCall()
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的生成模型")
	}
	return "", lastErr
}

// attemptGenerate 用一个候选模型跑一次生成请求。
func (c *Client) attemptGenerate(ctx context.Context, sel map[string]interface{}, opID, text string) (string, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, generateTextAttemptBudget)
	defer cancel()

	raw, err := c.CallCtx(attemptCtx, "workspace/generateText", map[string]interface{}{
		"workspace":       tempWorkspace(),
		"selection":       sel,
		"querySource":     generateTextQuerySource,
		"prompt":          text,
		"maxOutputTokens": generateTextMaxOutputTokens,
		"operationId":     opID,
	})
	if err != nil {
		return "", fmt.Errorf("workspace/generateText: %w", err)
	}
	got, err := parseGenerateTextResult(raw)
	if err != nil {
		return "", err
	}
	// 【输出纯净度】对违规输出做无损解包(剥围栏/JSON),再做空判:
	// 剥完若空(如应答只剩一对空围栏)按失败处理,不回填空串。
	got = sanitizeOptimizedOutput(got)
	if got == "" {
		return "", fmt.Errorf("workspace/generateText 应答里没有 text 字段: %s", truncateForLog(raw))
	}
	return got, nil
}

// withOutputDiscipline 给完整指令追加"输出纪律"收尾段。
// 收尾位置是刻意的:长指令场景下模型对最后读到的约束最敏感,与 server 层
// buildOptimizeInstruction 的头部队列形成双保险。
func withOutputDiscipline(prompt string) string {
	return strings.TrimRight(prompt, " \t\r\n") + "\n\n" + OptimizeOutputDiscipline("")
}

// OptimizeOutputDiscipline 按风格返回输出纪律文案。
//
// 【为什么在这层】server 层 buildOptimizeInstruction(指令头)归 server 智能体
// 所有;本函数是输出纯净度纪律的单一出处,导出供 server 层将来接线引用
// (接线后可把 withOutputDiscipline 的默认风格行换成带 style 的完整版)。
// style 只影响"风格侧重"一行,纪律本体(输出形态)完全一致:
//   - concise  → 强调压缩为一句话;
//   - detailed → 强调整理成要点列表;
//   - 其他/空  → 默认:长度与原文相当。
func OptimizeOutputDiscipline(style string) string {
	base := "输出纪律(必须不折不扣地遵守):\n" +
		"- 你的输出会被原样回填进用户输入框,只输出改写后的最终文本本身,一字不多;\n" +
		"- 禁止任何前言、开场白、解释、说明、道歉或收尾语(例如\"好的\"\"以下是改写结果\"\"希望有帮助\");\n" +
		"- 禁止用 Markdown 代码块围栏(```)或标题(#)包裹、修饰输出;\n" +
		"- 只提供一个方案:禁止输出多个版本或备选项(例如\"方案一/方案二\"\"Option A/Option B\");\n" +
		"- 必须保留用户原意的全部关键信息(对象、动作、条件、约束),不得遗漏,也不得替用户新增需求。\n"
	switch strings.TrimSpace(strings.ToLower(style)) {
	case "concise":
		return base + "- 当前为简洁模式:改写结果压缩成一句话,只保留核心诉求。\n"
	case "detailed":
		return base + "- 当前为详细模式:改写结果整理成要点列表,补齐目标的判断依据(不新增需求)。\n"
	default:
		return base + "- 当前为默认模式:改写结果长度与原文相当,表达更清晰、可直接执行。\n"
	}
}

// sanitizeOptimizedOutput 对模型违规输出做"可证明无损"的净化:
//  1. 整段被 Markdown 代码围栏(```...```)包裹 → 剥围栏取内部;
//  2. 整段可解析为 JSON 对象且 optimized 字段是非空字符串 → 解包取该字段
//     (对应"结构化输出"式违规:模型把改写结果包成 {"optimized": "..."} 回给)。
//
// 只在"整段"满足条件时动手,绝不做启发式截取——本链路指令明令禁止模型输出
// JSON/围栏,出现整段包裹即属违规,解包方向明确;而"前言 + 局部 JSON/多方案
// 正文"无法无损判定哪段是答案,保持原样交由指令纪律约束(宁可放过,不可错杀)。
func sanitizeOptimizedOutput(s string) string {
	out := strings.TrimSpace(s)
	if body, ok := stripCodeFence(out); ok {
		out = strings.TrimSpace(body)
	}
	var obj struct {
		Optimized string `json:"optimized"`
	}
	if err := json.Unmarshal([]byte(out), &obj); err == nil && strings.TrimSpace(obj.Optimized) != "" {
		return strings.TrimSpace(obj.Optimized)
	}
	return out
}

// stripCodeFence 判断整段文本是否被 Markdown 代码围栏包裹;是则返回围栏内部
// 内容(去掉首行语言标签)。不是整段包裹(缺开头/缺结尾/只有标签行)返回 false。
func stripCodeFence(s string) (string, bool) {
	if !strings.HasPrefix(s, "```") {
		return "", false
	}
	body := s[len("```"):]
	i := strings.IndexByte(body, '\n')
	if i < 0 {
		return "", false // 只有 "```lang" 一行,没有可剥的内容
	}
	body = body[i+1:]
	end := strings.LastIndex(body, "```")
	if end < 0 {
		return "", false // 只有开头围栏,没有收尾围栏:不动,交由指令纪律
	}
	return body[:end], true
}

// generateCandidates 生成候选模型序列:桌面当前模型优先(current 就是
// selection 形状,live 实证),其后按 available 顺序补位(按 provider+model 去重),
// 总数封顶 generateTextMaxCandidates。
func (c *Client) generateCandidates(ctx context.Context) ([]map[string]interface{}, error) {
	raw, err := c.CallCtx(ctx, "session/create", map[string]interface{}{
		"workspace": tempWorkspace(),
		"mode":      "build", // 红线:显式 build,绝不 yolo
	})
	if err != nil {
		return nil, fmt.Errorf("选模型所需的 session/create 失败: %w", err)
	}
	var snap struct {
		Settings struct {
			Model struct {
				Current   *map[string]interface{} `json:"current"`
				Available []struct {
					Ref struct {
						ProviderID string `json:"providerId"`
						ModelID    string `json:"modelId"`
					} `json:"ref"`
					Reasoning struct {
						DefaultLevel string `json:"defaultLevel"`
					} `json:"reasoning"`
				} `json:"available"`
			} `json:"model"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, fmt.Errorf("模型快照解析失败: %w", err)
	}

	seen := map[string]bool{}
	var out []map[string]interface{}
	add := func(sel map[string]interface{}) {
		p, _ := sel["providerId"].(string)
		m, _ := sel["modelId"].(string)
		if p == "" || m == "" {
			return
		}
		key := p + "/" + m
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, sel)
	}
	// 1) 桌面当前模型(current 形状与 selection 一致,原样透传)
	if cur := snap.Settings.Model.Current; cur != nil {
		add(*cur)
	}
	// 2) available 补位;有 defaultLevel 就带上(live 实证:有的模型强制要求
	//    reasoningLevel,缺了会被拒)
	for _, m := range snap.Settings.Model.Available {
		if len(out) >= generateTextMaxCandidates {
			break
		}
		sel := map[string]interface{}{"providerId": m.Ref.ProviderID, "modelId": m.Ref.ModelID}
		if m.Reasoning.DefaultLevel != "" {
			sel["options"] = map[string]string{"reasoningLevel": m.Reasoning.DefaultLevel}
		}
		add(sel)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("ZCode 模型表为空,无法确定生成用的模型")
	}
	return out, nil
}

// candidateLabels 日志用:候选模型的可读清单。
func candidateLabels(candidates []map[string]interface{}) []string {
	out := make([]string, 0, len(candidates))
	for _, sel := range candidates {
		p, _ := sel["providerId"].(string)
		m, _ := sel["modelId"].(string)
		out = append(out, p+"/"+m)
	}
	return out
}

// tempWorkspace 生成用的临时工作区(与 ZcodeProvider.fetchModels 同款:
// ZCode 只记录不作业;workspaceKey 不传 identity 时就是路径本身)。
func tempWorkspace() map[string]string {
	path := os.TempDir()
	return map[string]string{"workspacePath": path, "workspaceKey": path}
}

// truncateForLog 日志/报错用:应答原文截断到 200 字节,避免刷屏。
func truncateForLog(raw json.RawMessage) string {
	const max = 200
	if len(raw) > max {
		return string(raw[:max]) + "…"
	}
	return string(raw)
}

// parseGenerateTextResult 从应答里抠出生成的文本。
// 【live 实证形状】顶层 text;其余字段(result/content/output、整段字符串)是
// 对 schema 演进的廉价容错,命中实证形状前不碍事。
func parseGenerateTextResult(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var fields struct {
		Text    string `json:"text"`
		Result  string `json:"result"`
		Content string `json:"content"`
		Output  string `json:"output"`
	}
	if err := json.Unmarshal(raw, &fields); err == nil {
		for _, v := range []string{fields.Text, fields.Result, fields.Content, fields.Output} {
			if v != "" {
				return v, nil
			}
		}
	}
	// 整个 result 就是一个 JSON 字符串的情况
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return s, nil
	}
	return "", nil
}

// ---- 模型偏好记忆(上次成功的模型做首选)----
//
// 【存储】~/.pocketagentd/model-pref.json,单个小 JSON,与 events.jsonl/certs
// 同一数据目录惯例。读写全部 best-effort:文件损坏/不可读写只打日志,
// 绝不影响生成本身(没有偏好就按原候选序列来,行为与引入前一致)。

// modelPref 是偏好文件的持久化形状。
type modelPref struct {
	ProviderID      string `json:"providerId"`
	ModelID         string `json:"modelId"`
	UpdatedAtUnixMs int64  `json:"updatedAtUnixMs"`
}

// modelPrefFile 偏好文件绝对路径。包级变量是为了让单测重定向到临时目录
// (generate_test.go 的 TestMain 统一接管,单测绝不读写真实用户目录);
// 拿不到用户主目录时为空串,读写双方都按 no-op 处理。
var modelPrefFile = defaultModelPrefPath()

// defaultModelPrefPath 计算 ~/.pocketagentd/model-pref.json;拿不到主目录返回空串。
func defaultModelPrefPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".pocketagentd", "model-pref.json")
}

// prefMu 串行化偏好文件读写。GenerateText 由 server 层串行调用,但守卫
// 显式加锁比依赖调用方纪律便宜,也让测试可以放心并发读校验。
var prefMu sync.Mutex

// loadModelPref 读取上次成功的模型;没有记忆/读不了/形状坏 一律返回 nil。
func loadModelPref() *modelPref {
	if modelPrefFile == "" {
		return nil
	}
	prefMu.Lock()
	defer prefMu.Unlock()
	raw, err := os.ReadFile(modelPrefFile)
	if err != nil {
		return nil // 首次使用没有文件,常态
	}
	var pref modelPref
	if json.Unmarshal(raw, &pref) != nil || pref.ProviderID == "" || pref.ModelID == "" {
		log.Printf("[generate] 模型偏好文件损坏,忽略: %s", truncateForLog(raw))
		return nil
	}
	return &pref
}

// saveModelPref 记录一次成功的模型(落盘失败只打日志,不影响返回)。
func saveModelPref(sel map[string]interface{}) {
	p, _ := sel["providerId"].(string)
	m, _ := sel["modelId"].(string)
	if p == "" || m == "" || modelPrefFile == "" {
		return
	}
	prefMu.Lock()
	defer prefMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(modelPrefFile), 0o755); err != nil {
		log.Printf("[generate] 记录模型偏好失败(mkdir): %v", err)
		return
	}
	raw, err := json.Marshal(modelPref{ProviderID: p, ModelID: m, UpdatedAtUnixMs: time.Now().UnixMilli()})
	if err != nil {
		return
	}
	// 临时文件 + 改名:半截文件不会出现(半截 = 解析失败 = 视同无记忆)。
	tmp := modelPrefFile + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		log.Printf("[generate] 记录模型偏好失败(write): %v", err)
		return
	}
	if err := os.Rename(tmp, modelPrefFile); err != nil {
		log.Printf("[generate] 记录模型偏好失败(rename): %v", err)
	}
}

// promotePreferredModel 把 (provider, model) 匹配的候选挪到队首,其余保持
// 原相对顺序。找不到(模型下架/记忆过期)原样返回 —— 记忆不匹配不惩罚。
func promotePreferredModel(candidates []map[string]interface{}, providerID, modelID string) []map[string]interface{} {
	for i, sel := range candidates {
		p, _ := sel["providerId"].(string)
		m, _ := sel["modelId"].(string)
		if p == providerID && m == modelID {
			if i == 0 {
				return candidates // 已是首选
			}
			out := make([]map[string]interface{}, 0, len(candidates))
			out = append(out, sel)
			out = append(out, candidates[:i]...)
			out = append(out, candidates[i+1:]...)
			return out
		}
	}
	return candidates
}
