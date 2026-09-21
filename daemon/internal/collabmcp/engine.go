package collabmcp

import "context"

// DelegationEngine 是跨 agent 委派的核心窄接口：collabmcp（工具层）只认它，
// 不 import 任何 server/agent 包（依赖方向：server 侧实现本接口后注入 New）。
//
// server 侧真实实现走既有线程创建 + StartTurn 机制（接线属后续任务，本包用
// 可注入 fake 完成包内测试），精确接线点见包汇报 / 下方注释：
//   - Delegate：CreateThread（internal/server/thread.go:86，agentID 在
//     s.cfg.Providers 注册表即接入，无静态名单）→ 跑既有 StartTurn 轮次机制
//    （thread.go:748；providerForThread thread.go:1108 按 AgentID 显式选
//     provider，选不出硬错误不回退）→ 阻塞等轮次终局，assistant 终稿文本
//     作为 finalSummary。【绝不重写轮次循环】—— daemon 不当"第五个大脑"。
//   - StatusOf：读 activeTurns / turnQueue / 线程落库状态，映射成 ThreadStatus。
type DelegationEngine interface {
	// Delegate 同步执行一次委派：为 agentID 建子线程并跑完一个轮次，阻塞
	// 返回（子线程 id, 终稿摘要）。
	//
	// 失败语义（准则 6）：返回非 nil err 即如实透传给 coordinator（工具
	// 结果 = 错误文本），实现方不得伪造成功、不得静默吞错重试；collabmcp
	// 不改写错误内容。重试/换 agent/上报用户的决定权在 coordinator（模型
	// /用户），daemon 不代拟。
	Delegate(ctx context.Context, agentID, task string, opts DelegateOptions) (childThreadID, finalSummary string, err error)

	// StatusOf 查询某个委派子线程的当前状态（排队/运行/等待审批/终态）。
	// 未知 threadID 返回错误（工具层如实透传，不编造状态）。
	StatusOf(threadID string) (ThreadStatus, error)
}

// DelegateOptions 委派可选参数；全部留空 = 各 agent 默认（由引擎侧决定，
// 工具层只透传，不解释、不补默认值 —— 避免第二份默认名单）。
type DelegateOptions struct {
	// Workspace 工作区覆盖；空 = daemon 默认工作区。
	Workspace string
	// Model 模型覆盖（形如 "agent/model-id"）；空 = agent 当前默认。
	Model string
	// Effort 思考强度覆盖；空 = agent 当前默认。
	Effort string

	// Progress 引擎侧在关键节点回调的过程事件；可为 nil，实现方必须容忍。
	//
	// 本版仅两级（子线程 sink 桥接是后续任务，届时在此追加新 stage，
	// 向后兼容）：
	//   queued        —— 排队中：collabmcp 在信号量等待时自发，不经引擎；
	//   child_created —— 子线程已创建：引擎在建完子线程后回调，threadID
	//                     为子线程 id。
	Progress ProgressFunc
}

// ProgressFunc 一次委派过程回调。stage 取值见 DelegateOptions.Progress 与
// 下方常量；threadID 在 child_created 时为子线程 id（其余可为空）；message
// 是人话描述，会直接作为 MCP progress 通知文本给 coordinator。
type ProgressFunc func(stage, threadID, message string)

// 首版定义的 stage 取值（后续 sink 桥接在此追加，不改动既有取值）。
const (
	StageQueued       = "queued"        // 排队中（同 agent 串行，事实约束）
	StageChildCreated = "child_created" // 子线程已创建
)

// ThreadStatus 是子线程的粗粒度状态。
//
// State 取值由引擎给出（queued/running/waiting_approval/idle/failed/…），
// collabmcp 不做白名单翻译（"静态名单是债"），原样透传给 coordinator。
type ThreadStatus struct {
	ThreadID string
	AgentID  string
	State    string
	// Detail 人话补充（排队原因/审批等待/失败原因等）。引擎如实给，
	// daemon 不代拟、不修饰。
	Detail string
}
