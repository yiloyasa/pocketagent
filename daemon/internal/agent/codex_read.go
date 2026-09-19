// codex_read.go —— 四个只读小口子的 daemon 侧桥接(审计 §7.6/§2,全部 S 级)。
//
// 【这四个口子是干什么的】(2026-09-19 provider-coverage-audit 原文)
//
//	config/read                      → 设置页回显"桌面当前真实值"的现成来源(只读,绝不写)
//	thread/goal/get                  → 重连后回读目标态(此前只有 set/clear+通知)
//	modelProvider/capabilities/read  → 模型页解释"为什么某模型不可用"
//	windowsSandbox/readiness         → Windows 沙箱就绪度诊断(setupStart 是安装器动作,不做)
//
// 【形状:2026-09-19 对 codex-cli 0.149.1 live 实测】(证据:cmd/codexreadprobe)
//
//	config/read  {cwd?,includeLayers?}
//	           → {config:{model,modelProvider,approvalPolicy,sandboxMode,...},layers?,origins}
//	thread/goal/get  {threadId}
//	           → {goal:null} 或 {goal:{threadId,objective,status,tokenBudget,tokensUsed,
//	             timeUsedSeconds,createdAt,updatedAt}}   ← 字段名是 objective,不是 goal
//	modelProvider/capabilities/read  {}
//	           → {namespaceTools,imageGeneration,webSearch} 全 bool 必有
//	windowsSandbox/readiness  params=null
//	           → {status:"ready"|"notConfigured"|"updateRequired"}
//
// 【为什么只做 daemon 侧】审计意图是"供设置页/模型页/诊断回显",最终要到手机上
// 展示需要在 proto 加增量(thread.proto 正被两路并行占用);本文件先把读取与解析
// 做成可单测的 daemon 内部能力,server 层接线随 proto 增量另起(见交付报告)。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/pocketagent/daemon/internal/codexapp"
)

// codexJSONCaller 只读桥接需要的最小调用面。
//
// 【为什么抽这个接口】*codexapp.Client 天然满足它;单测用 fake 实现断言
// "发出的方法名与参数、回放的应答怎么解析",不必拉真实子进程
// (codexapp.Client 的字段不可从本包构造,直接依赖具体类型会让桥接不可测)。
type codexJSONCaller interface {
	CallCtx(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
}

// ---- 1) config/read:桌面当前真实配置回显 ----

// CodexConfigLayer 一层配置来源(includeLayers=true 时返回)。
// name 是来源类型(user/system/project/packagedDefaults/mdm/enterpriseManaged/
// sessionFlags/legacyManagedConfigToml*),file 只在文件型来源才有值。
type CodexConfigLayer struct {
	Name           string `json:"name"`
	File           string `json:"file,omitempty"`
	Version        string `json:"version,omitempty"`
	DisabledReason string `json:"disabledReason,omitempty"`
}

// CodexConfigSnapshot 是 config/read 里我们关心的"桌面当前真实值"。
// 只挑手机端设置页已有的对应项(回显对照用),其余字段进 Raw 供诊断,
// 绝不借这个口子发现新的可写配置 —— 写 config.toml 是项目红线。
type CodexConfigSnapshot struct {
	// Model 桌面当前模型 slug。实测可能是账号组限定的长 id(如
	// "dexacct.<base64>.<endpoint>.<model>"),展示时取最后一段与 model/list 对齐。
	Model string
	// ModelProvider 桌面当前 provider id(如 "dex_ai"/"openai")。
	ModelProvider string
	// Effort / Summary / Verbosity 思考强度、思考摘要、回答详略的桌面当前值。
	Effort, Summary, Verbosity string
	// SandboxMode read-only / workspace-write / danger-full-access。
	SandboxMode string
	// ApprovalPolicy 审批策略的字符串形状(untrusted/on-request/never);
	// granular 对象形状归一为 "granular",未设置(null)为空串。
	ApprovalPolicy string
	// ContextWindow 桌面当前模型上下文窗口(0 = 未设置)。
	ContextWindow int64
	// Layers 配置层信息(includeLayers=false 时为 nil)。
	Layers []CodexConfigLayer
	// Raw 原始 config 对象(截断到 16KB),诊断兜底 —— 没归纳到的字段看这里。
	Raw string
}

// configReadRawLimit Raw 的截断上限。config 里带 desktop 大对象,全量回显刷屏。
const configReadRawLimit = 16 * 1024

// ReadDesktopConfig 读取桌面 codex 当前生效配置(只读)。
// cwd 非空时按该目录解析项目层;includeLayers 时带回层信息。
func (p *CodexProvider) ReadDesktopConfig(ctx context.Context, cwd string, includeLayers bool) (*CodexConfigSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var out *CodexConfigSnapshot
	err := p.withStatusClient(func(ctx context.Context, c *codexapp.Client) error {
		var err error
		out, err = codexConfigReadVia(ctx, c, cwd, includeLayers)
		return err
	})
	if err != nil {
		log.Printf("[codex] config/read 失败: %v", err)
	}
	return out, err
}

// codexConfigReadVia 是 ReadDesktopConfig 的可测核心:直接面对 caller。
func codexConfigReadVia(ctx context.Context, c codexJSONCaller, cwd string, includeLayers bool) (*CodexConfigSnapshot, error) {
	if c == nil {
		return nil, fmt.Errorf("codex caller 为空")
	}
	params := map[string]interface{}{}
	if cwd != "" {
		params["cwd"] = cwd
	}
	if includeLayers {
		params["includeLayers"] = true
	}
	raw, err := c.CallCtx(ctx, "config/read", params)
	if err != nil {
		return nil, fmt.Errorf("config/read: %w", err)
	}
	var res struct {
		Config struct {
			Model                 *string          `json:"model"`
			ModelProvider         *string          `json:"model_provider"`
			ModelReasoningEffort  *string          `json:"model_reasoning_effort"`
			ModelReasoningSummary *string          `json:"model_reasoning_summary"`
			ModelVerbosity        *string          `json:"model_verbosity"`
			SandboxMode           *string          `json:"sandbox_mode"`
			ApprovalPolicy        json.RawMessage  `json:"approval_policy"`
			ModelContextWindow    *int64           `json:"model_context_window"`
			SandboxWorkspaceWrite *json.RawMessage `json:"sandbox_workspace_write"`
			WebSearch             *json.RawMessage `json:"web_search"`
		} `json:"config"`
		// layers 是应答顶层字段(schema ConfigReadResponse),不在 config 里。
		Layers []CodexConfigLayer `json:"layers"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("config/read 应答无法解析: %w", err)
	}
	cfg := res.Config
	out := &CodexConfigSnapshot{
		Model:          derefStr(cfg.Model),
		ModelProvider:  derefStr(cfg.ModelProvider),
		Effort:         derefStr(cfg.ModelReasoningEffort),
		Summary:        derefStr(cfg.ModelReasoningSummary),
		Verbosity:      derefStr(cfg.ModelVerbosity),
		SandboxMode:    derefStr(cfg.SandboxMode),
		ContextWindow:  derefInt64(cfg.ModelContextWindow),
		ApprovalPolicy: normalizeApprovalPolicy(cfg.ApprovalPolicy),
		Layers:         res.Layers,
		Raw:            clip(string(raw), configReadRawLimit),
	}
	return out, nil
}

// normalizeApprovalPolicy 把 approval_policy 归一成展示字符串:
// "untrusted"/"on-request"/"never" 原样;{"granular":{...}} → "granular";null/空 → ""。
func normalizeApprovalPolicy(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		if _, ok := obj["granular"]; ok {
			return "granular"
		}
	}
	// 认不出的形状:原文带上,别静默丢(诊断口子的职责是照镜子)。
	return clip(string(raw), 60)
}

// ---- 2) thread/goal/get:重连后回读目标态 ----

// CodexGoalState 是 codex 侧一条线程的常驻目标快照(goal=null 时 ReadThreadGoal 返回 nil)。
// 字段与 v2 schema ThreadGoal 一致 —— 注意文本字段叫 objective。
type CodexGoalState struct {
	ThreadID        string `json:"threadId"`
	Objective       string `json:"objective"`
	Status          string `json:"status"` // active/paused/blocked/usageLimited/budgetLimited/complete
	TokensUsed      int64  `json:"tokensUsed"`
	TokenBudget     int64  `json:"tokenBudget"` // 0 = 未设
	HasTokenBudget  bool   `json:"-"`
	TimeUsedSeconds int64  `json:"timeUsedSeconds"`
	CreatedAtUnixS  int64  `json:"createdAt"` // 实测秒级时间戳
	UpdatedAtUnixS  int64  `json:"updatedAt"`
}

// ReadThreadGoal 回读一条 codex 线程的常驻目标。
// codexThreadID 是 codex 侧线程 id;无目标时返回 (nil, nil) —— "没设目标"不是错误。
func (p *CodexProvider) ReadThreadGoal(ctx context.Context, codexThreadID string) (*CodexGoalState, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(codexThreadID) == "" {
		return nil, fmt.Errorf("thread/goal/get 需要 codex threadId")
	}
	var out *CodexGoalState
	err := p.withStatusClient(func(ctx context.Context, c *codexapp.Client) error {
		var err error
		out, err = codexGoalGetVia(ctx, c, codexThreadID)
		return err
	})
	if err != nil {
		log.Printf("[codex] thread/goal/get 失败 thread=%s: %v", codexThreadID, err)
	}
	return out, err
}

// codexGoalGetVia 是 ReadThreadGoal 的可测核心。
func codexGoalGetVia(ctx context.Context, c codexJSONCaller, codexThreadID string) (*CodexGoalState, error) {
	if c == nil {
		return nil, fmt.Errorf("codex caller 为空")
	}
	raw, err := c.CallCtx(ctx, "thread/goal/get", map[string]interface{}{"threadId": codexThreadID})
	if err != nil {
		return nil, fmt.Errorf("thread/goal/get: %w", err)
	}
	var res struct {
		Goal *struct {
			ThreadID        string `json:"threadId"`
			Objective       string `json:"objective"`
			Status          string `json:"status"`
			TokensUsed      int64  `json:"tokensUsed"`
			TokenBudget     *int64 `json:"tokenBudget"`
			TimeUsedSeconds int64  `json:"timeUsedSeconds"`
			CreatedAt       int64  `json:"createdAt"`
			UpdatedAt       int64  `json:"updatedAt"`
		} `json:"goal"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("thread/goal/get 应答无法解析: %w", err)
	}
	if res.Goal == nil {
		return nil, nil // 无目标:不是错误
	}
	g := res.Goal
	out := &CodexGoalState{
		ThreadID:        g.ThreadID,
		Objective:       g.Objective,
		Status:          g.Status,
		TokensUsed:      g.TokensUsed,
		TimeUsedSeconds: g.TimeUsedSeconds,
		CreatedAtUnixS:  g.CreatedAt,
		UpdatedAtUnixS:  g.UpdatedAt,
	}
	if g.TokenBudget != nil {
		out.TokenBudget, out.HasTokenBudget = *g.TokenBudget, true
	}
	return out, nil
}

// ---- 3) modelProvider/capabilities/read:provider 级能力 ----

// CodexProviderCaps 是 modelProvider/capabilities/read 的应答(三个字段实测必有)。
// 用途:模型页解释"为什么某模型不可用"(如图像生成/联网搜索当前 provider 给不给)。
type CodexProviderCaps struct {
	ImageGeneration bool `json:"imageGeneration"`
	NamespaceTools  bool `json:"namespaceTools"`
	WebSearch       bool `json:"webSearch"`
}

// ReadProviderCapabilities 读取当前 model provider 的能力声明(只读)。
func (p *CodexProvider) ReadProviderCapabilities(ctx context.Context) (*CodexProviderCaps, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var out *CodexProviderCaps
	err := p.withStatusClient(func(ctx context.Context, c *codexapp.Client) error {
		var err error
		out, err = codexProviderCapsVia(ctx, c)
		return err
	})
	if err != nil {
		log.Printf("[codex] modelProvider/capabilities/read 失败: %v", err)
	}
	return out, err
}

// codexProviderCapsVia 是 ReadProviderCapabilities 的可测核心。
func codexProviderCapsVia(ctx context.Context, c codexJSONCaller) (*CodexProviderCaps, error) {
	if c == nil {
		return nil, fmt.Errorf("codex caller 为空")
	}
	// schema:params 是空对象(必带,只是没有字段)。
	raw, err := c.CallCtx(ctx, "modelProvider/capabilities/read", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("modelProvider/capabilities/read: %w", err)
	}
	out := &CodexProviderCaps{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("modelProvider/capabilities/read 应答无法解析: %w", err)
	}
	return out, nil
}

// ---- 4) windowsSandbox/readiness:Windows 沙箱就绪度 ----

// 沙箱就绪度的三种取值(schema WindowsSandboxReadiness)。
const (
	SandboxReady          = "ready"          // 沙箱可用
	SandboxNotConfigured  = "notConfigured"  // 未配置
	SandboxUpdateRequired = "updateRequired" // 需要更新
)

// ReadSandboxReadiness 查询 Windows 沙箱就绪度(只读;setupStart 是安装器动作,不做)。
// 返回 ready / notConfigured / updateRequired 之一;非 Windows 平台 codex 也应答
// (实测本机 "ready"),含义由桌面端决定,这里只照镜子。
func (p *CodexProvider) ReadSandboxReadiness(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var status string
	err := p.withStatusClient(func(ctx context.Context, c *codexapp.Client) error {
		var err error
		status, err = codexSandboxReadinessVia(ctx, c)
		return err
	})
	if err != nil {
		log.Printf("[codex] windowsSandbox/readiness 失败: %v", err)
	}
	return status, err
}

// codexSandboxReadinessVia 是 ReadSandboxReadiness 的可测核心。
func codexSandboxReadinessVia(ctx context.Context, c codexJSONCaller) (string, error) {
	if c == nil {
		return "", fmt.Errorf("codex caller 为空")
	}
	// schema:params 为 null(请求里连 params 都不带)。
	raw, err := c.CallCtx(ctx, "windowsSandbox/readiness", nil)
	if err != nil {
		return "", fmt.Errorf("windowsSandbox/readiness: %w", err)
	}
	var res struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", fmt.Errorf("windowsSandbox/readiness 应答无法解析: %w", err)
	}
	switch res.Status {
	case SandboxReady, SandboxNotConfigured, SandboxUpdateRequired:
		return res.Status, nil
	case "":
		return "", fmt.Errorf("windowsSandbox/readiness 应答缺 status: %s", clip(string(raw), 120))
	default:
		// 认不出的新状态:如实透传,调用方按未知处理,不吞。
		return res.Status, nil
	}
}

// ---- 小工具 ----

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
