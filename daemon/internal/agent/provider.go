// Package agent 是 agent 的"大脑循环":拿到用户输入 → 调大模型 → 模型要用工具
// (读文件/跑命令)→ 把结果喂回去 → 模型继续想……直到模型认为任务完成。
//
// 【讲解】Provider 是"接哪家大模型"的抽象。每个 Provider 只需要实现一个
// Run 方法:输入对话历史和工具集,把过程事件实时吐给 sink(发给手机),
// 最后返回完整的回复块。换模型 = 换 Provider,其余代码不动。
package agent

import (
	"context"
	"log"
	"os"

	"github.com/pocketagent/daemon/internal/codexapp"
	"github.com/pocketagent/daemon/internal/dshapp"
	"github.com/pocketagent/daemon/internal/hermesapp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/piapp"
	"github.com/pocketagent/daemon/internal/store"
)

// Sink 接收过程事件,server 层把它转成 gRPC 流消息推给手机。
//
// 【vet】参数为指针:pb 消息内嵌 MessageState(含锁),按值传会被 go vet copylocks
// 拒绝(字面量定义处报 "func passes lock by value")。所有权随调用转移,
// 实现方只读不入库原指针、不得修改入参内容。
type Sink func(ev *pb.TurnServerMessage)

// ApprovalHook 在执行危险工具前询问手机用户"允许吗?"。
// 返回 (是否允许, 用户备注)。P0 的 echo provider 不会触发它。
// 【vet】参数为指针:pb 消息按值传会被 go vet copylocks 拒绝;hook 只读入参,
// 不得修改其内容。
type ApprovalHook func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error)

// ErrorInfoCarrier lets the server preserve structured provider diagnostics
// (HTTP status, URL and retryability) in the final TurnFailed frame.
type ErrorInfoCarrier interface {
	ErrorInfo() *pb.Error
}

// TurnRequest 一轮 agent 执行的输入。
type TurnRequest struct {
	Ephemeral   bool
	Thread      *store.Thread
	InputBlocks []pb.ContentBlock
	Workspace   string
	// MemoryContextPrefix 是 server 层在请求前组装好的记忆上下文前缀（四层：
	// 会话元数据 → 已保存记忆 → 早期摘要 → 近期消息，组装与预算见
	// server/turn_memory_context.go）。可选注入：目前仅 OpenAI 兼容 provider
	// 消费（并入首条 user 消息前的 system 补充段），其余 provider 不读该字段、
	// 零影响；空串 = 本轮不注入。protobuf 不承载（仅 Go 内存态，不出进程）。
	MemoryContextPrefix string
}

// Provider 大模型提供商。
// CollabAware 由"已把本地 collab MCP 挂到自己会话里"的 provider 实现 —— 也就是
// 它那一侧的模型**看得见 agent_delegate**,因而能派发/接收子智能体。
//
// 【为什么是接口而不是 agent 名单】谁接入协同取决于"接线事实"(pi 写了 mcp.json、
// dsh 注入了会话级 mcpServers),而不是它是谁。与 PrefsAware / TextGenerator /
// AgentSessionImporter 同款分工:新增一家只要提供同款事实就自动入列,daemon 侧
// 零改动(README §7「静态名单是债」)。
type CollabAware interface {
	// CollabReady 报告本 provider 当前是否已具备跨 agent 委派能力。
	// 判据必须是**可核验的接线事实**,不是配置开关的意愿。
	CollabReady() bool
}

type Provider interface {
	ID() string          // "glm" / "anthropic" / "echo"
	DisplayName() string // "智谱 GLM" ...
	Models() []pb.ModelInfo
	// Run 执行完整一轮:内部可能多次调模型 API(工具循环),
	// 过程事件经 sink 发出,返回最终 assistant 消息块。
	Run(ctx context.Context, req TurnRequest, sink Sink, approve ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error)
}

// Steerer 是「运行中还能插话」的可选扩展能力(P1-4)。
//
// codex 走原生 turn/steer 把新输入插进正在运行的轮次;不支持该能力的 provider
// (echo / OpenAI 兼容)由 server 层排队,当前轮次终局后自动作为新输入继续跑。
// server 层用类型断言探测该接口, provider 想支持就实现它,不想支持就不用管。
type Steerer interface {
	// Steer 向 threadID 正在执行的轮次插入一条输入。
	// steered=false 表示「此刻插不进去」(轮次未开始/已结束/上游拒绝),
	// 调用方应退回排队;codexTurnID 是上游应答里的轮次 id(可为空,诊断用)。
	Steer(ctx context.Context, threadID string, blocks []pb.ContentBlock) (steered bool, codexTurnID string, err error)
}

// PrefsAware 是「消费桌面端设置(手机设置页七项偏好)」的可选扩展能力。
//
// 【与 Steerer 同款取舍】server 层用类型断言探测,provider 想消费就实现,
// 不想就不用管:echo / OpenAI 兼容 provider 没有"桌面偏好"概念,不该被迫实现
// 空方法。codex 的 ApplyPrefs 签名天然一致(七项设置的原始消费方),无需适配器;
// pi / zcode 各自消费语义上说得通的子集:
//   - pi:HideReasoning(事件路径过滤 ThinkingDelta)+ 桌面默认 model/effort;
//   - zcode:HideReasoning(同款过滤)+ 桌面默认 effort(进 setModel 的
//     options.reasoningLevel)。审批策略/沙箱不映射 —— ZCode 权限模式固定
//     mode:"build"(红线),审批策略/沙箱仅对 codex 生效。
type PrefsAware interface {
	// ApplyPrefs 用一份**完整快照**覆盖内存态(server 层已按 presence 合并完毕;
	// 空串是有意义的取值 = 跟随桌面,provider 侧照单全收,不做"非空才覆盖")。
	ApplyPrefs(DesktopPrefs)
}

// 编译期钉死:四个桌面 agent provider 都必须消费桌面偏好。
var (
	_ PrefsAware = (*CodexProvider)(nil)
	_ PrefsAware = (*PiProvider)(nil)
	_ PrefsAware = (*ZcodeProvider)(nil)
	_ PrefsAware = (*DshProvider)(nil)
)

// DesktopPrefsFromStore 从持久化存储组装一份完整偏好快照。
// server 的设置广播(SetCodexSettings)与启动灌入(New)共用同一来源,
// 保证两条路径灌进 provider 的值永远一致(model/effort 取全局默认的落盘字段)。
func DesktopPrefsFromStore(st *store.FileStore) DesktopPrefs {
	prefs := st.CodexPrefs()
	return DesktopPrefs{
		ApprovalPolicy:    prefs.ApprovalPolicy,
		Sandbox:           prefs.Sandbox,
		Model:             st.DefaultModelID(),
		Effort:            st.DefaultEffort(),
		HideReasoning:     prefs.HideReasoning,
		Verbosity:         prefs.Verbosity,
		ReasoningSummary:  prefs.ReasoningSummary,
		DisablePasteBurst: prefs.DisablePasteBurst,
	}
}

// DetectProviders 根据环境变量自动发现可用 provider。
// 【讲解】这就是 BYOK 的雏形:钥匙(key)放环境变量,daemon 启动时自己看
// 有哪几把钥匙,手机端就能列出可用模型。P1 会把钥匙管理做成 app 设置页。
func DetectProviders() map[string]Provider {
	m := map[string]Provider{}
	if k := os.Getenv("GLM_API_KEY"); k != "" {
		m["glm"] = NewOpenAICompatible(
			"glm", "智谱 GLM",
			"https://open.bigmodel.cn/api/paas/v4",
			k,
			[]ModelSpec{
				{ID: "glm-4.7", DisplayName: "GLM-4.7", ContextWindow: 128_000, Thinking: true, Images: true},
				{ID: "glm-4.7-flash", DisplayName: "GLM-4.7 Flash(免费)", ContextWindow: 128_000, Thinking: false, Images: true},
				{ID: "glm-4.6", DisplayName: "GLM-4.6", ContextWindow: 200_000, Thinking: true, Images: true},
			},
		)
	}
	if k := os.Getenv("DEEPSEEK_API_KEY"); k != "" {
		m["deepseek"] = NewOpenAICompatible(
			"deepseek", "DeepSeek",
			"https://api.deepseek.com/v1",
			k,
			[]ModelSpec{
				{ID: "deepseek-chat", DisplayName: "DeepSeek Chat", ContextWindow: 64_000},
				{ID: "deepseek-reasoner", DisplayName: "DeepSeek Reasoner", ContextWindow: 64_000, Thinking: true},
			},
		)
	}
	// P1 再加:anthropic(海外网络)、ollama(本地模型,零成本)

	// codex(桌面 agent 桥接):装了 codex CLI 就能直接用桌面端 agent,
	// 和原作 Mobile Codex 同款架构。
	if _, err := codexapp.LocateCodex(); err == nil {
		m["codex"] = NewCodexProvider()
	}

	// pi(桌面 agent 桥接,Pi coding agent):node + cli.js 都定位得到、
	// --version 能出号就注册(探测是廉价的本地命令,不起子进程会话)。
	// 【zcode 注册点占位】zcode provider 由 T3 智能体在此处并列注册
	// (internal/agent/zcode*.go),主模型合并时接入。
	if node, cli, err := piapp.LocatePi(); err == nil {
		if v := piapp.Version(node, cli); v != "" {
			log.Printf("[providers] pi 探测成功: version=%s cli=%s", v, cli)
			m["pi"] = NewPiProvider()
		} else {
			log.Printf("[providers] pi 存在但 --version 失败(node=%s),不注册", node)
		}
	}

	// zcode(桌面 agent 桥接):装了 node + ZCode 的 zcode.cjs 才注册,
	// 与 codex/pi 并列(ZCode Protocol app-server --stdio)。
	RegisterZcode(m)

	// dsh(桌面 agent 桥接):node + @deepseek-ai/dsh 入口都定位得到才注册。
	// 探测只做"文件存在 + --version"(5min 缓存),绝不在启动时做 ACP initialize
	// 握手(PA-REAL-0002/0003 教训:一个 agent 的冷启动不许拖住别人)。
	// 注册与否不依赖 DEEPSEEK_API_KEY:key 缺失时线程照建,首条 prompt 才以
	// MISSING_CREDENTIAL 如实报错(P0 探针第 7 项实证)。
	if node, bin, err := dshapp.LocateDsh(); err == nil {
		if v := dshapp.Version(node, bin); v != "" {
			log.Printf("[providers] dsh 探测成功: version=%s bin=%s", v, bin)
			m["dsh"] = NewDshProvider()
		} else {
			log.Printf("[providers] dsh 存在但 --version 失败(bin=%s),不注册", bin)
		}
	}

	// hermes(桌面 agent 桥接,Nous Research):hermes-agent npm 包的 ACP stdio
	// 适配器(venv python -m acp_adapter)。探测只做"venv python 存在"(runtimeReady
	// 的充分证据),不起子进程、不做 initialize —— 与 codex/pi 同红线。未配置
	// 模型/凭据时线程照建,首轮 prompt 以人话错误如实回给手机端。
	if py, dir, err := hermesapp.LocateHermes(); err == nil {
		log.Printf("[providers] hermes 探测成功: python=%s acp=%s", py, dir)
		m["hermes"] = NewHermesProvider()
	} else {
		log.Printf("[providers] hermes 未就绪(%v),不注册", err)
	}

	// echo 永远在场:没有任何 key 时也能把整条链路(手机→协议→daemon)跑通。
	m["echo"] = NewEcho()
	return m
}

// ModelSpec 描述一个具体模型。
type ModelSpec struct {
	ID            string
	DisplayName   string
	ContextWindow int
	Thinking      bool
	Images        bool
}

// ModelCatalogRefresher invalidates adapter-owned metadata without restarting any active Agent.
type ModelCatalogRefresher interface{ InvalidateModels() }
