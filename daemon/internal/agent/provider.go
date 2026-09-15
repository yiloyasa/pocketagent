// Package agent 是 agent 的"大脑循环":拿到用户输入 → 调大模型 → 模型要用工具
// (读文件/跑命令)→ 把结果喂回去 → 模型继续想……直到模型认为任务完成。
//
// 【讲解】Provider 是"接哪家大模型"的抽象。每个 Provider 只需要实现一个
// Run 方法:输入对话历史和工具集,把过程事件实时吐给 sink(发给手机),
// 最后返回完整的回复块。换模型 = 换 Provider,其余代码不动。
package agent

import (
	"context"
	"os"

	"github.com/pocketagent/daemon/internal/codexapp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// Sink 接收过程事件,server 层把它转成 gRPC 流消息推给手机。
type Sink func(pb.TurnServerMessage)

// ApprovalHook 在执行危险工具前询问手机用户"允许吗?"。
// 返回 (是否允许, 用户备注)。P0 的 echo provider 不会触发它。
type ApprovalHook func(ctx context.Context, call pb.ToolCallBlock) (bool, string, error)

// ErrorInfoCarrier lets the server preserve structured provider diagnostics
// (HTTP status, URL and retryability) in the final TurnFailed frame.
type ErrorInfoCarrier interface {
	ErrorInfo() *pb.Error
}

// TurnRequest 一轮 agent 执行的输入。
type TurnRequest struct {
	Thread      *store.Thread
	InputBlocks []pb.ContentBlock
	Workspace   string
}

// Provider 大模型提供商。
type Provider interface {
	ID() string          // "glm" / "anthropic" / "echo"
	DisplayName() string // "智谱 GLM" ...
	Models() []pb.ModelInfo
	// Run 执行完整一轮:内部可能多次调模型 API(工具循环),
	// 过程事件经 sink 发出,返回最终 assistant 消息块。
	Run(ctx context.Context, req TurnRequest, sink Sink, approve ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error)
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
