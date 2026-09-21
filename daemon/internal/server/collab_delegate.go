// collab_delegate.go —— 跨 agent 委派桥的引擎侧接线(T-L2wire)。
//
// collabmcp(internal/collabmcp,T-L2a)定义了委派核心窄接口 DelegationEngine:
// 工具层只认接口、不 import server;本文件用 collabEngine 包装 *Server 实现该
// 接口,把 T-L2a 的 MCP 骨架接到真实的线程机制上:
//
//   - Delegate:复用 CreateThread(thread.go,agent_id 不存在的人话报错免费
//     获得,动态列举当前可用 agent,无静态名单)→ effort 覆盖(语义对齐
//     applyTurnOverrides)→ 落 user 消息 → runTurnOnce(StartTurn 轮次主体
//     原样抽取,thread.go)—— 委派子线程与手机轮次走完全相同的落库/收尾
//     路径,daemon 不当"第五个大脑"。
//   - StatusOf:Store.Get + activeTurns.Has + turnQueue.Len +
//     LastTurnError→failed,映射成 collabmcp.ThreadStatus 原样透传。
//
// 【深度锁 1(七准则⑤"子不生子")的两道闸】
// 第一道(结构性, piapp 侧):daemon 只把 collab MCP 配置挂到用户主动接入的
// coordinator;委派子线程走内部线程机制,不挂载任何 collab 配置 —— 该闸的
// 落地属 piapp 启动路径(不在本文件所有权内,由主模型统筹)。
// 第二道(本文件,引擎入口 belt-and-braces):collabEngine 给每个委派创建的
// 子线程打来源标记(childThreads 登记表),并在子轮次在飞期间(depth>0)拒绝
// 一切新的 Delegate 调用 —— 工具可见性保留(模型看得见 agent_delegate),
// 但引擎拒绝执行、如实报错,绝不代拟结果(collabmcp 准则 6:诚实报错不算
// 伪造)。
//
// 【T-L2fix 单写者重构:深度锁为什么跨进程成立】-collab-mcp 子模式已重写为
// stdio-MCP → 回环 gRPC 的薄代理(cmd/pocketagentd),自身不再持有引擎;
// 所有代理进程(含子 pi 再拉起的代理进程)的委派经 DelegateSubTask RPC
// 汇入主 daemon 的**单一** collabEngine 实例(见下方 collabEngines 登记表
// 与 Server.collabEngineFor)。于是 childThreads 登记簿、depth 与按设备
// 在飞计数(inflightByDevice)对全部代理客户端全局可见 —— 这正是单写者
// 重构的红利:深度计数不再是某个子进程的私有内存,而是主 daemon 的全局
// 真值,"子不生子"不再被跨进程击穿。DelegateSubTask handler 按"发起方
// token 对应 device"记账:同一 device 在飞委派 ≥1 即拒绝(跨进程语义补齐),
// 其余设备的在飞委派仍由共享 depth 闸保守拒绝(错误文本如实说明,由
// coordinator 择机重试)。
// 【为什么这道闸同时排除同 agent 自死锁】父 Delegate 持 per-agent 信号量
// 阻塞等子轮次;子若能再委派同 agent,会在同一把信号量上排队等父释放 ——
// 死锁。depth 闸在进入信号量逻辑之前就拒绝嵌套委派,该路径不可能发生。
package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/collabmcp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// maxDelegateTurnDuration 委派子轮次的时长上界(计划 §6.5 死锁防护三件套
// 之一)。手机轮次的时长纪律由各 provider 自带(codex/zcode 60 分钟超时打断
// + 无事件看门狗,pi 单命令/审批应答上界),委派子轮次经 runTurnOnce →
// provider.Run 天然继承同款纪律;但委派与手机轮次有一个关键差别:
// coordinator 在工具调用上**同步阻塞**,必须有引擎侧的显式上界兜底 —— 到点
// 取消子轮次,错误如实回传(已产出内容照常落库,与手机轮次失败同语义)。
const maxDelegateTurnDuration = 30 * time.Minute

// 编译期钉死:collabEngine 满足委派引擎窄接口(依赖方向:collabmcp 不
// import server,server 单向依赖 collabmcp;见 collabmcp/engine.go 注释)。
var _ collabmcp.DelegationEngine = (*collabEngine)(nil)

// collabEngine 把 *Server 包装成 DelegationEngine,并持有深度锁第二道闸的
// 状态(子线程来源登记 + 在飞子轮次深度 + 按发起方设备的在飞记账)。
// 【T-L2fix】生产路径上每个进程只有一个引擎实例(主 daemon 经
// Server.collabEngineFor 惰性创建,见 collabEngines):所有 -collab-mcp
// 代理进程的委派经 DelegateSubTask RPC 汇入它,depth/inflightByDevice 因此
// 是全局真值。NewCollabDelegationEngine(独立实例)保留给测试与既有窄接口
// 接线,两者闸语义一致。
type collabEngine struct {
	srv *Server

	mu           sync.Mutex
	childThreads map[string]bool // 委派创建的子线程 id 来源标记(进程生命周期)
	depth        int             // 在飞子轮次数;>0 期间拒绝新的委派(深度锁 1)
	// inflightByDevice 按"发起方 token 对应 device"记账在飞委派数(T-L2fix
	// 跨进程深度闸:DelegateSubTask handler 以连接身份为键,≥1 即拒绝)。
	// legacy 单 token 命中时键为 "legacy"(Validate 的回退路径返回值)。
	inflightByDevice map[string]int
}

// collabEngines 引擎单例登记表:键 *Server → *collabEngine。生产进程只有
// 一个 Server;测试会建多个 Server,各自拿到独立引擎互不串扰。条目随进程
// 存活(Server 也随进程存活),不清理。
var collabEngines sync.Map

// collabEngineFor 返回绑定该 Server 的单例委派引擎。DelegateSubTask RPC
// handler 经它取引擎 —— 单例保证所有代理连接共享同一份深度记账(单写者
// 重构的核心,见类型注释)。
func (s *Server) collabEngineFor() *collabEngine {
	if v, ok := collabEngines.Load(s); ok {
		return v.(*collabEngine)
	}
	v, _ := collabEngines.LoadOrStore(s, &collabEngine{
		srv:              s,
		childThreads:     map[string]bool{},
		inflightByDevice: map[string]int{},
	})
	return v.(*collabEngine)
}

// NewCollabDelegationEngine 用真实线程机制(server)构造委派引擎。
// 【T-L2fix】-collab-mcp 子模式已改为回环 gRPC 薄代理,生产主 daemon 走
// Server.collabEngineFor 单例;本构造器保留给测试与既有窄接口接线。
func NewCollabDelegationEngine(srv *Server) collabmcp.DelegationEngine {
	return &collabEngine{srv: srv, childThreads: map[string]bool{}, inflightByDevice: map[string]int{}}
}

// errDelegateDepthRejected 深度锁 1 的统一拒绝文案(同实例 depth 闸与跨进程
// per-device 闸共用):如实说明"子不生子",绝不代拟结果(准则 6)。
var errDelegateDepthRejected = errors.New("委派被拒绝:委派子任务不能再委派子任务(深度锁 1:子不生子)。请直接完成当前子任务并返回终稿")

// Delegate 实现 collabmcp.DelegationEngine:为 agentID 建子线程并同步跑完
// 一个轮次,返回(子线程 id, 终稿摘要)。
//
// 【与 StartTurn 的三点差异,均为委派场景的必然,不是第二套语义】
//  1. 无手机流:子线程过程帧本次不下发、不桥接(sink 为空汇;"子帧桥接"
//     属后续任务);终稿/错误经返回值交给 collabmcp 工具层(准则 6:
//     错误原样透传,重试与否由 coordinator 决定)。
//  2. 无审批中继:当前无用户在环,approve 一律拒绝 —— 绝不代用户同意
//     (伪造 decision = 红线);授权卡(准则 4)属后续任务 L2b。
//  3. 无排队续跑:一次委派 = 一轮,没有读手 goroutine 与内存队列。
func (e *collabEngine) Delegate(ctx context.Context, agentID, task string, opts collabmcp.DelegateOptions) (childThreadID, finalSummary string, err error) {
	// ---- 深度锁 1 第二道闸(同实例):子轮次在飞期间拒绝嵌套委派 ----
	// 在闸处就拒绝,后CreateThread、后进 per-agent 信号量 —— 同 agent
	// 自委派死锁与子生子在引擎入口即被排除。工具仍可见(模型看得见),
	// 拒绝是诚实报错,不是伪造结果。
	e.mu.Lock()
	if e.depth > 0 {
		e.mu.Unlock()
		return "", "", errDelegateDepthRejected
	}
	e.depth++
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.depth--
		e.mu.Unlock()
	}()
	return e.delegate(ctx, agentID, task, opts)
}

// DelegateForDevice 跨进程委派入口(DelegateSubTask RPC 专用,T-L2fix):
// 除同实例 depth 闸外,另按发起方 deviceId 记账 —— 该 device 已有在飞委派
// (≥1)即拒绝。所有 -collab-mcp 代理进程的委派都汇入同一个引擎实例,
// 记账因此对全部代理连接全局可见:子 pi 再拉起的全新代理进程打来的委派,
// 与父代理的委派在同一份登记簿上相遇,深度锁不再被跨进程击穿。
// deviceID 为空(无法识别连接身份)同样拒绝 —— 保守面如实报错。
func (e *collabEngine) DelegateForDevice(ctx context.Context, deviceID, agentID, task string, opts collabmcp.DelegateOptions) (childThreadID, finalSummary string, err error) {
	if strings.TrimSpace(deviceID) == "" {
		return "", "", errors.New("委派被拒绝:无法识别发起方设备(缺少已配对的 Bearer token);已配对设备才可委派")
	}
	e.mu.Lock()
	if e.depth > 0 || e.inflightByDevice[deviceID] > 0 {
		e.mu.Unlock()
		return "", "", errDelegateDepthRejected
	}
	e.depth++
	e.inflightByDevice[deviceID]++
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.inflightByDevice[deviceID]--
		if e.inflightByDevice[deviceID] <= 0 {
			delete(e.inflightByDevice, deviceID) // 归零即清键,登记簿不积累
		}
		e.depth--
		e.mu.Unlock()
	}()
	return e.delegate(ctx, agentID, task, opts)
}

// delegate 委派主体(建子线程 → 落库 → runTurnOnce),由 Delegate 与
// DelegateForDevice 在各自的深度闸之后调用。
func (e *collabEngine) delegate(ctx context.Context, agentID, task string, opts collabmcp.DelegateOptions) (childThreadID, finalSummary string, err error) {
	s := e.srv

	// 1) 建子线程:agentID 不在注册表 → CreateThread 人话报错(动态列举
	// 当前可用 agent);workspace/model 留空 = daemon/agent 默认(CreateThread
	// 侧语义,工具层不补默认值,避免第二份默认名单)。
	th, err := s.CreateThread(ctx, &pb.CreateThreadRequest{
		AgentId:   agentID,
		Workspace: opts.Workspace,
		ModelId:   opts.Model,
	})
	if err != nil {
		return "", "", err
	}
	childID := th.GetId()
	// 子线程来源标记(深度锁 1 的登记簿;进程生命周期,不清理 —— 量级 =
	// 委派次数,内存可忽略)。
	e.mu.Lock()
	e.childThreads[childID] = true
	e.mu.Unlock()

	if opts.Progress != nil {
		opts.Progress(collabmcp.StageChildCreated, childID, "已创建子线程 "+childID)
	}

	t, ok := s.cfg.Store.Get(childID)
	if !ok {
		return "", "", fmt.Errorf("子线程 %s 创建后不可读(存储异常)", childID)
	}

	// 2) effort 覆盖:语义对齐 applyTurnOverrides(thread.go)—— 空串是
	// 有效取值(跟随桌面默认);新建子线程的 t.Effort 本就是空串,空覆盖
	// 即恒等赋值。Store.Update 落库,"只影响本轮及之后、重启不丢"同样成立。
	t.Effort = strings.TrimSpace(opts.Effort)
	if err := s.cfg.Store.Update(t); err != nil {
		return "", "", errInternal(err)
	}

	// 3) 轮次执行:activeTurns 租约(StatusOf 的 running 判定 + 让子线程
	// 可被 CancelTurn RPC 取消)→ providerForThread 显式选人(选不出硬错误
	// 不回退,与手机轮次同纪律)→ 落 user 消息 → runTurnOnce。
	// runCtx 带 maxDelegateTurnDuration 上界:手机轮次的时长纪律靠 provider
	// 自带(经 runTurnOnce 天然继承),委派另有引擎侧显式兜底(见常量注释)。
	runCtx, cancelRun := context.WithTimeout(ctx, maxDelegateTurnDuration)
	defer cancelRun()
	lease, lerr := s.activeTurns.acquire(runCtx, t.ID, cancelRun)
	if lerr != nil {
		return "", "", lerr
	}
	defer s.activeTurns.Clear(t.ID, lease)

	provider, perr := s.providerForThread(t)
	if perr != nil {
		return "", "", perr
	}

	// 子帧桥接属后续任务:过程帧本次不下发;思考仍经 thinkingRecorder 随
	// assistant 消息落库(与手机轮次同一落库语义)。
	childSink, thinkBuf := thinkingRecorder(func(ev *pb.TurnServerMessage) {})

	// 准则 4(委派必须被授权)的当前形态:无用户在环,一律拒绝,绝不代拟
	// "同意"(伪造 decision = 红线)。授权卡属后续任务 L2b。
	approve := func(_ context.Context, _ *pb.ToolCallBlock) (bool, string, error) {
		return false, "委派子任务暂无审批通道,已拒绝该操作", nil
	}

	t.Messages = append(t.Messages, store.StoredMessage{
		Role: "user",
		Blocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{
			Text: &pb.TextBlock{Text: task},
		}}},
		CreatedAt: time.Now().UnixMilli(),
	})
	t.Status = pb.ThreadStatus_THREAD_STATUS_RUNNING
	// 与 StartTurn 同纪律:启动轮落库尽力而为,失败必须留痕 —— 否则磁盘
	// 异常期间整轮历史静默丢失。
	logStoreUpdateErr("delegate-turn-start", t.ID, s.cfg.Store.Update(t))
	taskBlocks := t.Messages[len(t.Messages)-1].Blocks

	summary, _, rerr := s.runTurnOnce(runCtx, provider, randomID("turn"), t,
		taskBlocks, childSink, thinkBuf, approve)
	if rerr != nil {
		// *turnRunError 解包后如实回传(collabmcp 不改写,工具结果 =
		// 错误文本);落库失败直接透传 errInternal。两种都不得伪造成功。
		if re, ok := rerr.(*turnRunError); ok {
			return childID, "", re.err
		}
		return childID, "", rerr
	}
	// 终稿摘要 = runTurnOnce 返回的 assistant 终稿文本(与落库消息同源,
	// 思考块非文本块不混入;有界截断由 collabmcp 工具层负责,准则 3)。
	return childID, summary, nil
}

// StatusOf 实现 collabmcp.DelegationEngine:线程的运行时状态 + 落库状态
// 映射成 ThreadStatus。未知线程如实报错(collabmcp 层原样透传,不编造
// "运行中")。State 取值引擎侧自由给出,collabmcp 不做白名单翻译。
func (e *collabEngine) StatusOf(threadID string) (collabmcp.ThreadStatus, error) {
	t, ok := e.srv.cfg.Store.Get(threadID)
	if !ok {
		return collabmcp.ThreadStatus{}, fmt.Errorf("没有找到线程 %s", threadID)
	}
	st := collabmcp.ThreadStatus{ThreadID: t.ID, AgentID: threadAgentID(t)}
	switch {
	case e.srv.activeTurns.Has(t.ID):
		st.State = "running"
		st.Detail = "子任务正在运行"
	case e.srv.turnQueue.Len(t.ID) > 0:
		st.State = "queued"
		st.Detail = fmt.Sprintf("有 %d 条输入在队列中等待执行", e.srv.turnQueue.Len(t.ID))
	case t.LastTurnError != "":
		st.State = "failed"
		st.Detail = t.LastTurnError
	default:
		st.State = "idle"
		st.Detail = "已完成"
	}
	return st, nil
}

// ---- DelegateSubTask RPC(T-L2fix:跨 agent 委派的跨进程落点)----
//
// -collab-mcp 薄代理进程(cmd/pocketagentd)经回环 gRPC 调本 RPC;handler
// 把请求转成单例引擎的 DelegateForDevice 调用。业务失败走应答的 error 字段
// + 人话,gRPC status 不承载业务失败(与既有风格对齐);gRPC status 仅留给
// 鉴权/传输层错误。鉴权闸门在既有 UnaryAuth 拦截器(checkAccess →
// authorized:已配对设备的 Bearer token 才可进来),handler 内再解析一次
// token 拿发起方 device 身份,用于跨进程深度记账(见 DelegateForDevice)。

// DelegateSubTask 实现 pb.ThreadServiceServer 的跨 agent 委派(只增)。
func (s *Server) DelegateSubTask(ctx context.Context, req *pb.DelegateSubTaskRequest) (*pb.DelegateSubTaskResponse, error) {
	deviceID, err := s.delegateDeviceID(ctx)
	if err != nil {
		return nil, err
	}
	eng := s.collabEngineFor()
	childID, summary, derr := eng.DelegateForDevice(ctx, deviceID,
		req.GetAgentId(), req.GetTask(), collabmcp.DelegateOptions{
			Workspace: req.GetWorkspace(),
			Model:     req.GetModel(),
			Effort:    req.GetEffort(),
		})
	resp := &pb.DelegateSubTaskResponse{ChildThreadId: childID, Summary: summary}
	if derr != nil {
		// 业务失败走 error 字段 + 人话原文,不伪造成功、不靠 gRPC status
		// 承载(准则 6:重试与否由 coordinator 决定)。
		resp.Error = derr.Error()
	}
	// state 尽力而为:子线程已建则回其返回时点状态,供代理进程的
	// agent_status 如实展示;拿不到就留空,不编造。
	if childID != "" {
		if st, serr := eng.StatusOf(childID); serr == nil {
			resp.State = st.State
		}
	}
	return resp, nil
}

// delegateDeviceID 从入站 metadata 解析发起方设备身份:Bearer token 经设备
// 注册表 Validate(与 authorized 同一条链:先设备表、再 legacy 回退),
// 返回 device id(legacy 单 token 命中时为 "legacy")。
// 【与拦截器的分工】鉴权拒绝已由 UnaryAuth 完成;这里再验一次是拿身份而非
// 重复设卡 —— Validate 带 60 秒 last_seen 节流,双调用不会把 devices.json
// 当心跳打。拿不到合法身份返回 Unauthenticated(fail closed)。
func (s *Server) delegateDeviceID(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "缺少 metadata")
	}
	for _, v := range md.Get("authorization") {
		token, ok := strings.CutPrefix(v, "Bearer ")
		if !ok {
			continue
		}
		if deviceID, _, ok := s.devices.Validate(token); ok {
			return deviceID, nil
		}
	}
	return "", status.Error(codes.Unauthenticated, "token 无效,请先配对")
}
