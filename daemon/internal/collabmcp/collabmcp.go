// Package collabmcp 是 daemon 内嵌的本地 MCP 服务器 "pocketagent-collab"，
// 跨 agent 委派桥的 L2 骨架（调研 §6，2026-09-20；本包由 T-L2a 落地）。
//
// 为什么是 MCP：四家 harness 唯一共用的"工具语言"。把 daemon 从"只读 MCP
// 状态"升级为"本地 MCP 服务器宿主"，任何 coordinator agent 通过标准 MCP
// 工具把子任务委派给其他 agent —— 零 proto 改动，完全符合"daemon 只做
// 翻译官"与"能透传就透传"两条铁律。
//
// 工具面（首版最小集，宁可少勿滥，调研 §6.2）：
//   - agent_delegate {agent_id, task, workspace?, model?, effort?}
//     同步委派：daemon 建子线程 → 跑既有 StartTurn 机制 → 阻塞返回子任务
//     终稿摘要（有界 ≤8192 字符）。agent_id 必须是 daemon 当前已注册的
//     agent 的 id —— 调用方显式给，daemon 不猜、不路由。
//   - agent_status {thread_id?}
//     查询子任务线程状态（排队/运行/等待审批/终态）。
//
// 七准则（调研 §3）在本包的落实位置：
//   1. 委派对模型是一个工具：agent_delegate 单一入口，后端多 agent
//     （dsh 接缝哲学）—— 见 registerTools。
//   2. 子代理活动以工具卡呈现：不在本包发明新帧；工具结果与 progress
//      通知就是卡片素材，走既有 ToolCallBlock 通道。
//   3. 父持决策权、子返回有界摘要：truncateRunes，≤8192 字符截断并标注。
//   4. 委派必须被授权：授权卡（首次确认）属后续任务 L2b；本包把引擎
//      Delegate 定为唯一闸门，授权检查加在引擎实现里即可，工具层不重复做。
//   5. 深度锁 1（子不生子）：本 MCP 服务器只挂到"用户主动接入"的
//      coordinator agent；daemon 为委派创建的子线程走内部线程机制，
//      【接线硬性要求】不得给子线程挂载本服务器 —— 子线程因此看不见
//      agent_delegate，天然"子不生子"。
//   6. 失败如实回传：引擎错误原样进工具结果（IsError + 错误文本），
//      daemon 绝不代拟"成功"，重试与否由 coordinator（模型/用户）决定
//      —— 见 runDelegate。
//   7. 同 agent 串行是事实约束：per-agent 信号量（同 agent=同 provider
//      串行，跨 agent 并行），真排队才发"排队中"progress，不伪造并行
//      —— 见 agentLimiter / runDelegate。
//
// 不做的（首版）：agent_spawn 异步、子代理再委派（准则 5）、模型自选
// agent 的智能路由（agent_id 由调用方显式给）。
package collabmcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerName MCP 服务器注册名。各 harness 把它当一条普通本地 MCP server
// 配置接入（codex 红线：绝不改写用户 ~/.codex/config.toml，引导用户自己粘，
// 调研 §6.3；dsh 走隔离 HOME 的 profile 注入）。
const ServerName = "pocketagent-collab"

// 常量上限：都按 rune 数（中文安全）。maxStatusRunes 与摘要同准则 3 ——
// 状态输出也不灌全文。
const (
	// maxSummaryRunes 子任务终稿摘要的字符上限（调研 §6.2 的"≤8K 字符"）。
	maxSummaryRunes = 8 * 1024
	// maxStatusRunes agent_status 输出的字符上限。
	maxStatusRunes = 8 * 1024
	// taskPreviewRunes 委派记录里 task 的预览长度（对齐 truncateTitle 量级）。
	taskPreviewRunes = 80
	// maxRecords 委派记录条数上限（内存态防无限增长；agent_status 不带
	// thread_id 时的兜底列表用）。
	maxRecords = 64
)

// Server 是 collab MCP 服务器本体。包一层是为了持有引擎（DelegationEngine，
// server 侧注入）与委派记录；两个工具的 handler 注册在内部的 *mcp.Server 上。
type Server struct {
	mcp     *mcp.Server
	engine  DelegationEngine
	limiter *agentLimiter

	mu      sync.Mutex
	records []delegationRecord
}

// delegationRecord 一条已完成的委派登记（agent_status 兜底列表用）。
type delegationRecord struct {
	ThreadID  string
	AgentID   string
	Task      string // 已截到 taskPreviewRunes
	StartedAt time.Time
}

// New 构造 collab MCP 服务器并注册工具。engine 是委派核心（真实实现由
// server 侧接线注入，见 DelegationEngine 注释）；version 建议传 daemon
// 版本号，空则 "dev"。
func New(engine DelegationEngine, version string) *Server {
	if version == "" {
		version = "dev"
	}
	s := &Server{
		mcp:     mcp.NewServer(&mcp.Implementation{Name: ServerName, Version: version}, nil),
		engine:  engine,
		limiter: newAgentLimiter(),
	}
	s.registerTools()
	return s
}

// registerTools 注册两个工具。tools/list 的 inputSchema 由 SDK 从入参
// struct 推断（jsonschema tag 进描述），供 coordinator 模型理解。
func (s *Server) registerTools() {
	// 准则 1：委派对模型只有一个工具，后端多 agent。
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name: "agent_delegate",
		Description: "把一个有界子任务委派给另一个 agent（跨 harness 协同）。" +
			"agent_id 必须是 daemon 当前已注册的 agent 的 id（不存在的 id 会返回错误文本，daemon 不做猜测或路由）。" +
			"本工具同步阻塞到子任务完成，返回其终稿摘要（最多 8192 字符，超出截断）。" +
			"同一 agent 的委派会串行排队（事实约束，排队与子线程创建经 progress 通知回报），不同 agent 可并行。",
	}, s.handleDelegate)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name: "agent_status",
		Description: "查询委派子任务的当前状态。" +
			"传 thread_id（agent_delegate 返回的子线程 id）查单个子任务；" +
			"不传则列出本服务器生命周期内的委派记录及各自状态。",
	}, s.handleStatus)
}

// delegateArgs agent_delegate 入参。agent_id/task 必填（无 omitempty 即
// required）；描述写清"agent_id 取 daemon 当前已注册 agent 的 id"。
type delegateArgs struct {
	AgentID string `json:"agent_id" jsonschema:"目标 agent 的 id，必须是 daemon 当前已注册的 agent（由调用方显式给出）"`
	Task    string `json:"task" jsonschema:"要委派的有界子任务描述；完成后只返回不超过 8192 字符的终稿摘要"`
	// 下面三项可选（omitempty => schema 非 required）：留空 = 各 agent 默认。
	Workspace string `json:"workspace,omitempty" jsonschema:"可选：工作区目录覆盖，留空用 daemon 默认工作区"`
	Model     string `json:"model,omitempty" jsonschema:"可选：模型覆盖（形如 agent/model-id），留空用该 agent 当前默认模型"`
	Effort    string `json:"effort,omitempty" jsonschema:"可选：思考强度覆盖，留空用该 agent 当前默认"`
}

// statusArgs agent_status 入参：thread_id 可选。
type statusArgs struct {
	ThreadID string `json:"thread_id,omitempty" jsonschema:"可选：agent_delegate 返回的子线程 id；不传则列出全部委派记录"`
}

func (s *Server) handleDelegate(ctx context.Context, req *mcp.CallToolRequest, in delegateArgs) (*mcp.CallToolResult, any, error) {
	text, isErr := s.runDelegate(ctx, in, s.progressFunc(ctx, req))
	return toolText(text, isErr), nil, nil
}

func (s *Server) handleStatus(_ context.Context, _ *mcp.CallToolRequest, in statusArgs) (*mcp.CallToolResult, any, error) {
	text, isErr := s.runStatus(in)
	return toolText(text, isErr), nil, nil
}

// toolText 把人话文本包成工具结果。isErr=true 时置 IsError（MCP 工具级
// 错误而非协议错误）：coordinator 收到"工具失败了 + 原因"，由它决定重试/
// 换 agent/上报用户 —— daemon 绝不代拟成功（准则 6）。
func toolText(text string, isErr bool) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
		IsError: isErr,
	}
}

// progressFunc 把一次工具调用桥到 MCP progress 通知。MCP 规范要求客户端
// 在请求里给了 progress token 才发通知 —— 没 token/session 时返回 nil，
// 工具层与引擎实现都必须容忍 nil（过程信息不丢：排队态会落 daemon 日志，
// 子线程 id 一定出现在终稿文本里）。
func (s *Server) progressFunc(ctx context.Context, req *mcp.CallToolRequest) ProgressFunc {
	if req == nil || req.Session == nil {
		return nil
	}
	tok := req.Params.GetProgressToken()
	if tok == nil {
		return nil
	}
	sess := req.Session
	var seq float64
	// 【观感修正(T-L2wire 审核)】total 只在"真的发过 queued"后才置 2:
	// 无争用的委派只有 child_created 一条,报 1/2 会让人误以为还有一步;
	// 带 total(省略 0,MCP 规范视为不确定进度)比误导性的假分母诚实。
	var queued bool
	return func(stage, threadID, message string) {
		seq++
		if stage == StageQueued {
			queued = true
		}
		var total float64
		if queued {
			total = 2
		}
		// best-effort：通知发不出去不影响委派本身，终稿文本才是契约。
		_ = sess.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
			ProgressToken: tok,
			Message:       message,
			Progress:      seq,
			Total:         total,
		})
	}
}

// record 登记一条委派记录（agent_status 兜底列表用；只留最近 maxRecords 条）。
func (s *Server) record(threadID, agentID, task string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, delegationRecord{
		ThreadID:  threadID,
		AgentID:   agentID,
		Task:      truncateRunes(task, taskPreviewRunes),
		StartedAt: time.Now(),
	})
	if n := len(s.records) - maxRecords; n > 0 {
		s.records = s.records[n:]
	}
}

// runStatus agent_status 主干。带 thread_id 查单个（引擎 StatusOf；未知
// 线程如实报错，不编造"运行中"）；不带则把本服务器的委派记录逐个问状态。
func (s *Server) runStatus(in statusArgs) (string, bool) {
	tid := strings.TrimSpace(in.ThreadID)
	if tid != "" {
		st, err := s.engine.StatusOf(tid)
		if err != nil {
			return fmt.Sprintf("查询失败：%v", err), true
		}
		var b strings.Builder
		fmt.Fprintf(&b, "thread=%s\nagent=%s\nstate=%s", st.ThreadID, st.AgentID, st.State)
		if st.Detail != "" {
			fmt.Fprintf(&b, "\ndetail=%s", st.Detail)
		}
		return truncateRunes(b.String(), maxStatusRunes), false
	}

	s.mu.Lock()
	records := append([]delegationRecord(nil), s.records...)
	s.mu.Unlock()
	if len(records) == 0 {
		return "尚无委派记录。先调 agent_delegate 委派子任务，再用返回的 thread_id 查询状态；或不带参数列出全部记录。", false
	}
	var b strings.Builder
	fmt.Fprintf(&b, "委派记录 %d 条（新→旧，最多保留 %d 条）：\n", len(records), maxRecords)
	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		state, note := "", ""
		if st, err := s.engine.StatusOf(r.ThreadID); err != nil {
			state = "unknown"
			note = "状态不可得：" + err.Error() // 如实透传，不代拟状态
		} else {
			state, note = st.State, st.Detail
		}
		line := fmt.Sprintf("- thread=%s agent=%s state=%s task=%s", r.ThreadID, r.AgentID, state, r.Task)
		if note != "" {
			line += " | " + note
		}
		b.WriteString(line + "\n")
	}
	return truncateRunes(b.String(), maxStatusRunes), false
}

// MCPServer 暴露底层 *mcp.Server：测试可用 IO/in-memory 传输端到端自测，
// 后续接 Streamable HTTP 双传输时也复用它（调研 §6.1/R1）。常规接入走
// ServeStdio。
func (s *Server) MCPServer() *mcp.Server { return s.mcp }

// ServeStdio 以 stdio 传输运行（首版传输：codex/dsh/pi 都认 stdio）。
// 阻塞直到 ctx 取消或对端关闭。进程模型：接入方把 daemon 当一条本地 MCP
// 子进程挂载（daemon 侧如何起子进程/接 stdio 属接线任务，不在本包）。
func (s *Server) ServeStdio(ctx context.Context) error {
	return s.mcp.Run(ctx, &mcp.StdioTransport{})
}
