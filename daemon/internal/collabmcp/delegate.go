package collabmcp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"google.golang.org/grpc/metadata"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// truncateRunes 按字符数截断（rune 计，中文安全）。超限追加标注，让
// coordinator 明确知道"这是截断件，不是全文"（准则 3：父持决策权，
// 子返回有界摘要，不灌全文）。
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + fmt.Sprintf("\n…[已截断：超过 %d 字符上限，只保留前 %d 字符（有界返回）]", max, max)
}

// agentLimiter per-agent 执行闸：同 agent（=同 provider；README 铁律"同
// provider 多会话 turn 锁排队，不可宣称并行"）串行，不同 agent 各持一把锁
// 天然并行（四家 provider 本就是独立子进程）。
//
// 【不是静态名单】锁按首次出现的 agentID 惰性创建 —— 新增 provider 自动
// 获得自己的闸，无需改本包（README §7：静态名单是债）。
type agentLimiter struct {
	mu    sync.Mutex
	slots map[string]chan struct{}
}

func newAgentLimiter() *agentLimiter {
	return &agentLimiter{slots: make(map[string]chan struct{})}
}

// tryAcquire 非阻塞拿锁：拿到返回 release 与 true（未排队）。
func (l *agentLimiter) tryAcquire(agentID string) (release func(), ok bool) {
	ch := l.slot(agentID)
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, true
	default:
		return nil, false
	}
}

// acquire 阻塞拿锁；ctx 取消则放弃（coordinator 等不及撤了，不该继续占位）。
//
// 【优先级选择（T-L2wire 审核小疵修正）】"槽空闲"与"ctx 已取消"双就绪时,
// 裸 select 会随机选分支 —— 先显式裁决已取消的 ctx（取消方不该再占位）,
// 再非阻塞试拿空闲槽,最后才进入阻塞等待,消除随机性。
func (l *agentLimiter) acquire(ctx context.Context, agentID string) (func(), error) {
	ch := l.slot(agentID)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	default:
	}
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (l *agentLimiter) slot(agentID string) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	ch, ok := l.slots[agentID]
	if !ok {
		ch = make(chan struct{}, 1)
		l.slots[agentID] = ch
	}
	return ch
}

// runDelegate agent_delegate 的主干（与 MCP 层解耦：包内用 fake 引擎直测，
// 并发断言不必过协议层）。返回（结果文本, 是否工具级错误）。
//
// 流程：拿 per-agent 执行权（真排队才发"排队中"，准则 7 不伪造并行）→
// 引擎 Delegate（建子线程 + 跑既有轮次机制；引擎回调 child_created 进度）
// → 成功回有界摘要 / 失败如实回错误文本（准则 6）。
//
// 深度锁 1（准则 5）说明：本函数是唯一委派入口。深度/来源记账收归主 daemon
// 单引擎实例（-collab-mcp 子模式是回环薄代理，所有代理客户端的委派都汇入
// 主 daemon，depth 记账跨进程全局可见）；pi 侧经 pi-mcp-adapter 挂载时，其
// 委派子线程的 pi 进程同样继承注入目录——跨进程嵌套由主 daemon 引擎闸统一
// 拒绝（批9审核 P1 处方，T-L2fix 落地），工具保留可见但引擎如实报错。
func (s *Server) runDelegate(ctx context.Context, in delegateArgs, notify ProgressFunc) (string, bool) {
	agentID := strings.TrimSpace(in.AgentID)
	task := strings.TrimSpace(in.Task)
	if agentID == "" || task == "" {
		return "agent_delegate 需要 agent_id 与 task：agent_id 是 daemon 当前已注册的 agent 的 id，task 是要委派的有界子任务描述。", true
	}

	// 并发闸：先非阻塞试拿；只有真拿不到才算"排队"，立刻把排队态通知
	// 出去 —— 秒进执行时不谎报"排队中"（准则 7：排队显式可见，不伪造）。
	release, ok := s.limiter.tryAcquire(agentID)
	if ok {
		defer release()
	} else {
		msg := fmt.Sprintf("排队中：agent %s 正在执行其他委派，本委派已排队（同 agent 串行是事实约束）", agentID)
		log.Printf("[collab] queued agent=%s", agentID)
		if notify != nil {
			notify(StageQueued, "", msg)
		}
		var err error
		release, err = s.limiter.acquire(ctx, agentID)
		if err != nil {
			// coordinator 在排队期间取消（ctx 结束）：如实报"取消"，
			// 不算子任务失败 —— 子线程根本还没建。
			return fmt.Sprintf("委派已取消（排队等待 agent %s 执行权期间上下文结束）：%v", agentID, err), true
		}
		defer release()
	}

	opts := DelegateOptions{
		Workspace: strings.TrimSpace(in.Workspace),
		Model:     strings.TrimSpace(in.Model),
		Effort:    strings.TrimSpace(in.Effort),
		// T-S1Pa：可选父链透传，空 = 子线程父链为空（诚实变暗），引擎不补
		// 默认值（避免第二份默认名单）。
		ParentThreadID: strings.TrimSpace(in.ParentThreadID),
		Progress:       notify,
	}
	childID, summary, err := s.engine.Delegate(ctx, agentID, task, opts)
	if err != nil {
		// 准则 6：错误原样透传 —— 不代拟成功、不吞错、不改写、不重试
		//（重试与否由 coordinator 决定）。
		log.Printf("[collab] delegate failed agent=%s: %v", agentID, err)
		return fmt.Sprintf("委派失败（agent=%s）：%v", agentID, err), true
	}

	s.record(childID, agentID, task)

	head := fmt.Sprintf("子任务完成（agent=%s, thread=%s）", agentID, childID)
	if childID == "" {
		head = fmt.Sprintf("子任务完成（agent=%s）", agentID)
	}
	// 准则 3：终稿摘要超 8192 字符即截断并标注。
	return head + "\n" + truncateRunes(summary, maxSummaryRunes), false
}

// ---- GRPCDelegationEngine 薄代理引擎(T-L2fix 单写者重构)----
//
// -collab-mcp 子模式(cmd/pocketagentd)不再自持 FileStore/providers/引擎,
// 改为:本包的 MCP 工具面原样保留(agent_delegate / agent_status),引擎侧
// 换成本类型 —— 收到 agent_delegate 时经回环 gRPC(127.0.0.1:7246,TLS +
// Bearer token)调主 daemon 的 DelegateSubTask RPC,把委派汇入主 daemon 的
// 单一委派引擎实例。两个 P1 因此关闭:
//   - 写冲突 P1:代理进程不再触碰 threads.json/events.jsonl,主 daemon 是
//     唯一写者(子线程与事件 seq 不再被双进程互相抹掉/污染);
//   - 子生子 P1:主 daemon 引擎的深度记账对所有代理客户端全局可见,子 pi
//     再拉起的全新代理进程打来的委派同样被深度闸覆盖。
// 依赖方向不变:collabmcp 不 import server,只依赖 gen 契约与 DelegationEngine
// 窄接口;真实引擎(主 daemon 侧)仍由 server.collabEngine 提供。

// GRPCDelegationEngine 以 gRPC 客户端身份实现 DelegationEngine 的薄代理。
// client 由 cmd 层注入(拨号/TLS/凭证装配属进程接线,不在本包);token 是
// mcp.json env 注入的 POCKETAGENT_TOKEN(legacy 单 token,主 daemon 的
// Validate 回退认它)。defaultWorkspace 对齐旧子模式语义:工具调用未显式
// 给 workspace 时用它(= 拉起方传入的 -workspace,缺省为代理进程 cwd)。
type GRPCDelegationEngine struct {
	client           pb.ThreadServiceClient
	token            string
	defaultWorkspace string

	mu        sync.Mutex
	lastKnown map[string]ThreadStatus // 经手委派的返回时点状态(agent_status 用)
}

// 编译期钉死:薄代理满足委派引擎窄接口。
var _ DelegationEngine = (*GRPCDelegationEngine)(nil)

// NewGRPCEngine 构造薄代理引擎。client 为 ThreadService 的 gRPC 客户端;
// token 为空时仅当主 daemon 以 -insecure-no-auth 应急运行才可能放行;
// 配对码模式(默认)下 token 缺失/错误一律 Unauthenticated 如实拒绝,
// 错误原样透传(delegateDeviceID fail-closed,无"本机代理放行"旁路)。
func NewGRPCEngine(client pb.ThreadServiceClient, token, defaultWorkspace string) *GRPCDelegationEngine {
	return &GRPCDelegationEngine{
		client:           client,
		token:            strings.TrimSpace(token),
		defaultWorkspace: strings.TrimSpace(defaultWorkspace),
		lastKnown:        map[string]ThreadStatus{},
	}
}

// Delegate 实现 DelegationEngine:把委派原样转成 DelegateSubTask RPC。
// 错误语义(准则 6)不变:gRPC 层错误(鉴权/传输)与应答 error 字段的业务
// 失败都原样上抛,不重试、不改写、不代拟成功。
func (g *GRPCDelegationEngine) Delegate(ctx context.Context, agentID, task string, opts DelegateOptions) (childThreadID, finalSummary string, err error) {
	callCtx := ctx
	if g.token != "" {
		callCtx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+g.token)
	}
	ws := strings.TrimSpace(opts.Workspace)
	if ws == "" {
		ws = g.defaultWorkspace
	}
	resp, err := g.client.DelegateSubTask(callCtx, &pb.DelegateSubTaskRequest{
		AgentId:   strings.TrimSpace(agentID),
		Task:      task,
		Workspace: ws,
		Model:     strings.TrimSpace(opts.Model),
		Effort:    strings.TrimSpace(opts.Effort),
		// T-S1Pa：可选父链原样透传给主 daemon（空 = 不挂父链，主 daemon 侧
		// 维持现状）。pi 侧 v1 恒为空（见 DelegateOptions.ParentThreadID 注释）。
		ParentThreadId: strings.TrimSpace(opts.ParentThreadID),
		// 协同角色：原样透传给主 daemon 解析（薄代理不知道有哪些角色）。
		RoleId: strings.TrimSpace(opts.RoleID),
	})
	if err != nil {
		return "", "", err
	}
	// 应答 error 字段 = 业务失败(agent 不存在/深度拒绝/子轮次失败的人话)。
	if resp.GetError() != "" {
		g.remember(resp.GetChildThreadId(), agentID, false, resp.GetError(), resp.GetState())
		return resp.GetChildThreadId(), "", errors.New(resp.GetError())
	}
	g.remember(resp.GetChildThreadId(), agentID, true, "", resp.GetState())
	return resp.GetChildThreadId(), resp.GetSummary(), nil
}

// StatusOf 实现 DelegationEngine:返回该线程**经本代理进程委派**的返回时点
// 状态。代理模式没有线程库访问权(单写者纪律),更晚的状态变化不跟踪 ——
// Detail 如实标注时点口径;未经手的线程如实报错,绝不编造状态。
func (g *GRPCDelegationEngine) StatusOf(threadID string) (ThreadStatus, error) {
	g.mu.Lock()
	st, ok := g.lastKnown[threadID]
	g.mu.Unlock()
	if !ok {
		return ThreadStatus{}, fmt.Errorf("没有找到线程 %s 的委派记录(代理模式只跟踪本进程经手的委派)", threadID)
	}
	return st, nil
}

// remember 缓存一次委派的返回时点状态(agent_status 的数据源,诚实口径:
// 失败缓 failed + 原始错误,成功缓 daemon 给的 state)。
func (g *GRPCDelegationEngine) remember(threadID, agentID string, ok bool, errMsg, state string) {
	if threadID == "" {
		return
	}
	st := ThreadStatus{ThreadID: threadID, AgentID: agentID, State: state}
	switch {
	case !ok:
		st.State = "failed"
		st.Detail = "委派失败:" + errMsg
	case state == "idle":
		st.Detail = "已完成(委派返回时点状态,代理模式不跟踪后续变化)"
	default:
		st.Detail = "委派返回时点状态,代理模式不跟踪后续变化"
	}
	g.mu.Lock()
	g.lastKnown[threadID] = st
	g.mu.Unlock()
}
