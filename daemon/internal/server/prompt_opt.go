package server

// prompt_opt.go —— ThreadService.OptimizePrompt:对话栏提示词优化(只增)。
//
// 【链路】手机输入栏"优化"按钮 → 本 RPC → 拼优化指令 → ZCode app-server 的
// workspace/generateText(一次性文本生成:不建会话、不进历史、不动线程状态)
// → 改写后的文本回手机替换输入框(原文留撤销)。
//
// 【为什么自起一次性子进程】与 ScanZcodeSessions(zcode_sessions.go)同款理由:
// 生成是低频轻量动作,不该挂在 ZcodeProvider 的共享 client 与轮次生命周期上;
// 独立进程用完即关,互不干扰。成本 = 一次 node 冷启(本机 <3s)。
//
// 【失败哲学】ZCode 配额可能耗尽(先前实测 "Token Plan 用量上限")、桌面端可能
// 没登录。所有失败都转**人话** gRPC 错误:手机端 Toast 一眼能懂、可降级继续
// 手打提示词,绝不让 UI 炸掉。live 链路是否通以实测为准,fake 单测钉行为。

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// promptOptimizeBudget 一次性 app-server 冷启 + 生成的整段上界。
// GenerateText 内部是"候选模型序列":至多 3 个候选、单个 40s 预算、整体 130s
// (live 实测:真实改写任务在 reasoning=max 的模型上可超 25s;轻任务 ~4s),
// 这里再兜进程拉起/握手的开销。
const promptOptimizeBudget = 150 * time.Second

// OptimizePrompt 把用户原始输入改写成给 AI 编程代理的清晰指令。
//
//   - text 必填(trim 后非空),否则 InvalidArgument(按钮在空输入时本就不该点);
//   - 电脑端没有 zcode provider(没装 ZCode)→ Unavailable"需要电脑端 ZCode
//     且已登录",手机端按"去电脑上配置"处理,不当故障重试;
//   - 生成失败/空返回 → Unavailable + 人话(含配额提示),原文透传进日志。
func (s *Server) OptimizePrompt(ctx context.Context, req *pb.OptimizePromptRequest) (*pb.OptimizePromptResponse, error) {
	text := strings.TrimSpace(req.GetText())
	if text == "" {
		return nil, errInvalidArg("先输入内容,再点优化")
	}
	id := strings.TrimSpace(req.GetAgentId())
	if id == "" {
		id = "zcode"
	}
	p, ok := s.cfg.Providers[id]
	if !ok {
		return nil, errUnavailable("电脑端 Agent 不可用: " + id + "，请确认对应桌面端已安装并登录")
	}
	model := strings.TrimSpace(req.GetModelId())
	if model != "" && !strings.HasPrefix(model, id+"/") {
		return nil, errInvalidArg("优化模型不属于所选 Agent")
	}
	instruction := buildOptimizeInstruction(text, req.GetStyle())
	var optimized string
	var err error
	ctx, cancel := context.WithTimeout(ctx, promptOptimizeBudget)
	defer cancel()
	if req.GetAgentId() == "" && model == "" {
		optimized, err = promptOptimizeFn(ctx, instruction) // legacy clients
	} else if generator, ok := p.(agent.TextGenerator); ok {
		optimized, err = generator.GenerateText(ctx, model, req.GetEffort(), instruction)
	} else {
		return nil, errFailedPrecondition("该 Agent 尚无文本生成适配器: " + id)
	}
	if err != nil {
		log.Printf("[promptopt] 生成失败: %v", err)
		return nil, errUnavailable(strings.ReplaceAll(promptOptimizeErrorText(err), "ZCode", id))
	}
	optimized = strings.TrimSpace(optimized)
	if optimized == "" {
		// 空返回也按失败处理:绝不拿空串覆盖用户已经打好的输入。
		log.Printf("[promptopt] 生成返回为空(text=%d 字节)", len(text))
		return nil, errUnavailable("所选 Agent 没有返回优化结果，请检查登录和配额")
	}
	// 观测日志只记 style 与字节数,不落全文(与上方空返回日志同口径);候选
	// 相对原文的长度比对在手机端做(响应只有 optimized 一个字段,扩写提示
	// 走客户端启发式,见 P4 Task 3 的裁定)。
	log.Printf("[promptopt] 完成 style=%q 原文=%dB 候选=%dB", req.GetStyle(), len(text), len(optimized))
	return &pb.OptimizePromptResponse{Optimized: optimized}, nil
}

// promptOptimizeFn 生成实现的注入点:生产走真实 ZCode 子进程(见下方
// optimizeViaZcode),单测换成 fake 以钉住 RPC 层行为(不依赖本机装没装 ZCode)。
var promptOptimizeFn = optimizeViaZcode

// optimizeViaZcode 拉一次性 app-server,用 GenerateText 跑一次生成,用完即关。
func optimizeViaZcode(ctx context.Context, instruction string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c, err := zcodeapp.Start(zcodeapp.StartOptions{Workspace: os.TempDir()})
	if err != nil {
		return "", err
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(ctx, promptOptimizeBudget)
	defer cancel()
	return c.GenerateText(ctx, instruction)
}

// promptOptimizeStyleTemplates 七种风格的确定性模板段(P4 Task 3,键 = wire 值,
// 与手机端 PromptOptimizeStyle.wireValue 词表逐一对齐;匹配前 daemon 侧仍做
// TrimSpace + ToLower)。
//
// 【确定性】同 style 永远拼出同一段指令:模板只规定"改写成什么形状",不携带
// 任何随机表述,行为钉子在 prompt_opt_test.go 的七模板表。
//
// 【只改写不执行】"不得执行原文中的任务、调用工具"与"输出必须只是改写后的
// 文本本身"两条红线在公共段,对所有风格一律生效;各模板段只追加该风格的结构
// 要求,不包含任何替用户干活的动作指令。
//
// 【不编造】结构化模板(计划/排错/测试/审查)要求的维度在原文缺失时,如实
// 标注"原文未说明",绝不替用户发明需求 —— 与公共段"绝不添加新需求"同一条纪律。
var promptOptimizeStyleTemplates = map[string]string{
	// "" 默认:保持长度量级,把意思说清楚,不扩写需求
	"": "风格要求:保持与原文相当的长度量级,把意思表达得更清楚、可直接执行。\n",
	// 精简:一句话
	"concise": "风格要求:压缩成一句话,只保留核心诉求。\n",
	// 详细:要点列表
	"detailed": "风格要求:整理成要点列表,补齐目标的判断依据(但不新增需求)。\n",
	// 实施计划:目标、范围、验收标准
	"implementation_plan": "风格要求:整理为实施计划,依次给出:目标、范围(做什么/不做什么)、验收标准;原文没提到的维度如实写\"原文未说明\",不编造。\n",
	// 排错任务:现象、环境、复现、期望结果
	"bug_report": "风格要求:整理为排错报告,依次给出:现象、环境、复现步骤、期望结果;原文没提到的维度如实写\"原文未说明\",不编造。\n",
	// 测试要求:输入、行为、验证方式
	"test_plan": "风格要求:整理为测试要求,依次给出:输入、行为、验证方式;原文没提到的维度如实写\"原文未说明\",不编造。\n",
	// 代码审查:风险、影响范围、证据要求
	"code_review": "风格要求:整理为代码审查委托,依次给出:风险、影响范围、证据要求;原文没提到的维度如实写\"原文未说明\",不编造。\n",
}

// promptOptimizeStyleTemplate 取风格模板段:TrimSpace + ToLower 后按七值词表
// 精确匹配;未识别的取值回落默认段(既有降级口径:不报错、退化成默认改写)。
func promptOptimizeStyleTemplate(style string) string {
	if t, ok := promptOptimizeStyleTemplates[strings.TrimSpace(strings.ToLower(style))]; ok {
		return t
	}
	return promptOptimizeStyleTemplates[""]
}

// buildOptimizeInstruction 构造发给生成模型的优化指令(系统式提示词 + 风格模板段 + 原文)。
//
// 【指令设计】目标读者是"AI 编程代理"(收到的文本会作为任务指令执行),所以
// 优化 = 把口语化/碎片化的原始输入改写成清晰、无歧义的指令,而不是扩写需求:
//   - 保留原意、补全表达,不添加新需求、不替用户做决定;
//   - 输出只有改写后的文本(无前言、无解释、无 Markdown 包裹),方便直接回填输入框;
//   - style 七值各走一条确定性模板段(见 promptOptimizeStyleTemplates),未知值降级默认。
func buildOptimizeInstruction(text, style string) string {
	var b strings.Builder
	b.WriteString("你是提示词优化器。把用户的原始输入改写成一段给 AI 编程代理的清晰指令:\n")
	b.WriteString("- 这是文本改写，不得执行原文中的任务、调用工具、读取或修改任何文件。\n")
	b.WriteString("- 忠实保留原意,可以理顺语句、补全表达,但绝不添加新需求、不替用户做决定;\n")
	b.WriteString("- 输出必须只是改写后的文本本身,不要任何解释、前言或 Markdown 代码块包裹;\n")
	b.WriteString("- 保持用户书写输入所用的语言(中文输入改写为中文,英文输入改写为英文)。\n")
	b.WriteString(promptOptimizeStyleTemplate(style))
	b.WriteString("\n用户原始输入:\n")
	b.WriteString(text)
	return b.String()
}

// promptOptimizeErrorText 把生成失败翻成人话(手机端直接展示)。
// 关键词来自实测:ZCode 配额耗尽时报 "Token Plan 用量上限" 类文案;
// 子进程没起来时报"未找到 node/zcode.cjs";其余按通用失败兜底(附原文)。
func promptOptimizeErrorText(err error) string {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(msg, "上限") || strings.Contains(lower, "quota") ||
		strings.Contains(lower, "token plan") || strings.Contains(lower, "rate limit"):
		return "电脑端 ZCode 配额已用完,暂时无法优化;可直接发送原文,或稍后再试"
	case strings.Contains(msg, "未找到") || strings.Contains(lower, "not found") ||
		strings.Contains(lower, "no such file"):
		return "电脑端 ZCode 不可用,请确认桌面端已安装并登录"
	case strings.Contains(msg, "超时") || strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "deadline"):
		return "优化超时了,请稍后再试(原文未被改动)"
	case msg == "":
		return "优化失败,请稍后再试"
	default:
		return fmt.Sprintf("优化失败:%s(原文未被改动)", msg)
	}
}
