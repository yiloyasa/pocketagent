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
// 落地属 piapp 启动路径(不在本文件所有权内,由主模型统筹)。这是"子不生子"
// 的主闸:子线程根本看不见 agent_delegate,自然调不起来。
// 第二道(本文件,引擎入口 belt-and-braces):collabEngine 给每个委派创建的
// 子线程打来源标记(childThreads 登记表),并在入口精确判定"发起方线程是否
// 本身就是委派子线程"(opts.ParentThreadID ∈ childThreads)→ 是则拒绝。
// 主闸漏挂时(某些路径未来可能漏接结构性闸)本判定兜底:只要委派方显式给了
// 父链、且父链指向一个委派子线程,就在此被拦下。工具可见性保留(模型看得见
// agent_delegate),但引擎拒绝执行、如实报错,绝不代拟结果(collabmcp 准则 6:
// 诚实报错不算伪造)。
// 【深度锁第三道闸(X2,2026-09-30):父链推断的推理防线】第二道闸依赖发起方
// 如实上报父链(opts.ParentThreadID),而 v1 链路里该入参恒空 → handler 经
// soleParentTurn 从活跃轮次推断。推断的隐含假设"唯一非子活跃轮次就是发起方"
// 会被"委派子线程发起再委派"击穿:此时父轮次仍同步阻塞在自己的 agent_delegate
// 工具调用上(租约未放),真正发起方是仍在运行的子线程 —— 旧逻辑把阻塞父
// 误判为唯一父,父链指向非子线程,深度闸形同虚设。DelegateSubTask 因此改用
// resolveParentThreadIDGuarded:推断出唯一候选且仍有子轮次在飞时,不再信任
// 推断,以深度拒绝收场(见该函数注释)。
// 【为什么深度锁不再靠 depth>0 一刀切】早期实现用"全局在飞数 > 0"同时承担
// 两件事:既当并发闸又当深度锁,导致"只能同时派 1 个子智能体"。二者语义本应
// 拆开:并发是"有界上限"(可并行 N 个),深度锁是"子不生子"的精确判定。本文件
// 据此拆分 —— 并发走 maxCollabParallel / maxCollabParallelPerDevice 有界计数,
// 深度锁走 childThreads 精确判定,二者互不耦合(见 Delegate / DelegateForDevice
// 入口注释)。
//
// 【T-L2fix 单写者重构:深度锁与并发账跨进程成立】-collab-mcp 子模式已重写为
// stdio-MCP → 回环 gRPC 的薄代理(cmd/pocketagentd),自身不再持有引擎;
// 所有代理进程(含子 pi 再拉起的代理进程)的委派经 DelegateSubTask RPC
// 汇入主 daemon 的**单一** collabEngine 实例(见下方 collabEngines 登记表
// 与 Server.collabEngineFor)。于是 childThreads 登记簿、在飞并发计数
// (inflight / inflightByDevice)对全部代理客户端全局可见 —— 这正是单写者
// 重构的红利:并发账与深度判定不再是某个子进程的私有内存,而是主 daemon 的
// 全局真值。DelegateSubTask handler 按"发起方 token 对应 device"记账:同一
// device 在飞委派达单设备上限、或全局达上限即拒绝(跨进程语义补齐),错误文本
// 如实说明,由 coordinator 择机重试。
// 【为什么这道闸同时排除同 agent 自死锁 + 互锁】父 Delegate 持 per-agent 信号量
// 阻塞等子轮次;子若能再委派同 agent,会在同一把信号量上排队等父释放 ——
// 死锁。深度锁判定(子线程的父链指向子线程)在进入信号量逻辑之前就拒绝嵌套
// 委派,该路径不可能发生。又因并发超限一律"如实拒绝、不排队",父轮次同步
// 阻塞等待时不会陷入"排队等子、子又等父"的互锁。
package server

import (
	"context"
	"errors"
	"fmt"
	"log"
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

// maxCollabParallel 并行委派(同时派出的子智能体)全局上限。
// 有界并发,**不是排队**:排队会让父轮次同步阻塞等子、子又等父而互锁(见文件
// 头注释)。超限如实报错、由 coordinator 择机重试,绝不伪造成功或静默排队。
// 默认值 3;-collab-max-parallel flag(经 SetCollabMaxParallel)可覆盖;0/负数
// 视为默认。
var maxCollabParallel = 3

// maxCollabParallelPerDevice 单设备(发起方 token 对应 device)并行委派上限。
// 跨进程场景(多代理进程汇入同一引擎)按 device 记账,防止单设备占满全局并发。
// 语义与 maxCollabParallel 一致,默认同样 3。
var maxCollabParallelPerDevice = 3

// errDelegateDisabled 表示委派已被运行时策略关闭(规格 §11 的 native_delegate_enabled)。
//
// 【为什么单列一个错误值】调用方(父轮次的工具调用)要能区分"这次被策略挡了"与
// "引擎故障":前者是用户可改的状态,提示该说"去控制面开启",而不是当成故障重试。
var errDelegateDisabled = errors.New("委派已在运行时策略里关闭(可在控制面重新开启)")

// SetCollabMaxParallel 供 cmd/pocketagentd 的 -collab-max-parallel flag 接线:
// 把全局/单设备并行上限设为 n。n <= 0 视为默认(3),由本函数统一归一化,引擎
// 只读这两个包级变量、不重复校验范围。整进程只此一处写入并发上限。
func SetCollabMaxParallel(n int) {
	if n <= 0 {
		n = 3
	}
	maxCollabParallel = n
	maxCollabParallelPerDevice = n
}

// 编译期钉死:collabEngine 满足委派引擎窄接口(依赖方向:collabmcp 不
// import server,server 单向依赖 collabmcp;见 collabmcp/engine.go 注释)。
var _ collabmcp.DelegationEngine = (*collabEngine)(nil)

// collabEngine 把 *Server 包装成 DelegationEngine,并持有两道闸的状态:
//   - 深度锁第二道闸:子线程来源登记(childThreads);
//   - 有界并发账:全局在飞数(inflight)+ 按发起方 device 的在飞数
//     (inflightByDevice)。
//
// 【T-L2fix】生产路径上每个进程只有一个引擎实例(主 daemon 经
// Server.collabEngineFor 惰性创建,见 collabEngines):所有 -collab-mcp
// 代理进程的委派经 DelegateSubTask RPC 汇入它,childThreads/inflight/
// inflightByDevice 因此是全局真值。NewCollabDelegationEngine(独立实例)保留
// 给测试与既有窄接口接线,两者闸语义一致。
type collabEngine struct {
	srv *Server

	mu sync.Mutex
	// childThreads 委派创建的子线程 id 来源标记(进程生命周期,不清理)。
	// 深度锁第二道闸的判定依据:发起方线程(opts.ParentThreadID)若在此登记
	// 簿内 → 说明是"子在生子"→ 拒绝(见 Delegate / DelegateForDevice 入口)。
	childThreads map[string]bool
	// inflight 当前在飞委派数(全局有界并发上限 maxCollabParallel 检查用)。
	// 它只是"计数",不再承担深度锁语义 —— 深度锁改由 childThreads 精确判定,
	// 与"子不生子"解耦、与并发上限解耦。
	inflight int
	// inflightByDevice 按"发起方 token 对应 device"记账在飞委派数(T-L2fix
	// 跨进程并发闸:DelegateSubTask handler 以连接身份为键,达单设备上限即
	// 拒绝)。legacy 单 token 命中时键为 "legacy"(Validate 的回退路径返回值)。
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
	// W10:协同策略闸(规格 §11)。放在所有闸的最前面 —— 策略关闭是"这件事根本不该
	// 发生",比深度/并发闸更靠前;否则关掉之后仍会看到"已达并发上限"这类误导性原因。
	if !e.srv.collabPolicySnapshot().DelegateEnabled() {
		return "", "", errDelegateDisabled
	}
	// ---- 深度锁 1 第二道闸(入口精确判定,取代旧版 depth>0 一刀切)----
	// 发起方线程(opts.ParentThreadID)若本身就在 childThreads 登记簿 → 说明
	// 是"子在生子" → 拒绝。主闸是结构性闸(子线程不挂 collab MCP,看不见
	// agent_delegate),本判定是兜底:只要委派方显式给了父链、且父链指向一个
	// 委派子线程,就在此拦下。绝不代拟结果(collabmcp 准则 6)。
	// 注意:本引擎不靠"在飞数 > 0"挡嵌套 —— 那是并发闸的事,二者已解耦。
	e.mu.Lock()
	if opts.ParentThreadID != "" && e.childThreads[opts.ParentThreadID] {
		e.mu.Unlock()
		return "", "", errDelegateDepthRejected
	}
	// ---- 有界并发闸(取代旧版 depth>0 单飞)----
	// 全局在飞数达上限即拒绝。**不排队**:父轮次对 agent_delegate 同步阻塞,
	// 排队会让父永远等不到子返回而互锁(见文件头注释)。超限如实报错,由
	// coordinator 择机重试,绝不伪造成功。
	if e.inflight >= maxCollabParallel {
		e.mu.Unlock()
		return "", "", errDelegateParallelLimit(maxCollabParallel)
	}
	e.inflight++
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.inflight--
		e.mu.Unlock()
	}()
	return e.delegate(ctx, agentID, task, opts)
}

// DelegateForDevice 跨进程委派入口(DelegateSubTask RPC 专用,T-L2fix):
// 除同实例深度锁判定外,另按发起方 deviceId 记账 —— 该 device 在飞数达单设备
// 上限即拒绝。所有 -collab-mcp 代理进程的委派都汇入同一个引擎实例,记账因此
// 对全部代理连接全局可见:子 pi 再拉起的全新代理进程打来的委派,与父代理的
// 委派在同一份登记簿上相遇,深度锁与并发账不再被跨进程击穿。
// deviceID 为空(无法识别连接身份)同样拒绝 —— 保守面如实报错。
func (e *collabEngine) DelegateForDevice(ctx context.Context, deviceID, agentID, task string, opts collabmcp.DelegateOptions) (childThreadID, finalSummary string, err error) {
	if strings.TrimSpace(deviceID) == "" {
		return "", "", errors.New("委派被拒绝:无法识别发起方设备(缺少已配对的 Bearer token);已配对设备才可委派")
	}
	// W10:协同策略闸,与 Delegate 同一条(跨进程入口不能绕过策略)。
	if !e.srv.collabPolicySnapshot().DelegateEnabled() {
		return "", "", errDelegateDisabled
	}
	e.mu.Lock()
	// 深度锁 1(同 Delegate 入口判定):父链指向委派子线程 → 子在生子 → 拒绝。
	if opts.ParentThreadID != "" && e.childThreads[opts.ParentThreadID] {
		e.mu.Unlock()
		return "", "", errDelegateDepthRejected
	}
	// 有界并发闸:全局或本 device 达上限即拒(不排队,防父轮次同步阻塞互锁)。
	if e.inflight >= maxCollabParallel || e.inflightByDevice[deviceID] >= maxCollabParallelPerDevice {
		e.mu.Unlock()
		return "", "", fmt.Errorf("委派被拒绝:已达并行上限(全局 %d / 本设备 %d),请等待已有子任务完成后再派发", maxCollabParallel, maxCollabParallelPerDevice)
	}
	e.inflight++
	e.inflightByDevice[deviceID]++
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.inflightByDevice[deviceID]--
		if e.inflightByDevice[deviceID] <= 0 {
			delete(e.inflightByDevice, deviceID) // 归零即清键,登记簿不积累
		}
		e.inflight--
		e.mu.Unlock()
	}()
	return e.delegate(ctx, agentID, task, opts)
}

// errDelegateParallelLimit 并发上限拒绝文案:如实说明已达并行上限 N,请等待
// 已有子任务完成,绝不伪造成功(准则 6)。
func errDelegateParallelLimit(n int) error {
	return fmt.Errorf("委派被拒绝:已达并行上限 %d,请等待已有子任务完成后再派发", n)
}

// soleParentTurn 推断委派发起方(父)线程 id:取当前活跃轮次(activeTurns)的
// 线程 id 快照,排除已知委派子线程(childThreads)后,若恰好剩 1 个 → 它是唯一
// 父轮次,返回它。
// 原理:agent_delegate 由父轮次同步发起,父此刻必持 activeTurns 租约;子轮次
// 虽也在 activeTurns 中,但已登记在 childThreads,可排除。故即便并行派出多个
// 子代理,父轮次仍是唯一候选 → 父链推断依旧成立。
// 多于 1 个或 0 个活跃候选 → 无法唯一确定父,返回 ("", false):如实留空,
// 不猜、不伪造父链(准则 6)。
func (e *collabEngine) soleParentTurn() (string, bool) {
	ids := e.srv.activeTurns.IDs()
	candidate := ""
	e.mu.Lock()
	for _, id := range ids {
		if e.childThreads[id] {
			continue // 排除委派子线程:它不是父
		}
		if candidate != "" {
			// 已出现第二个非子候选 → 无法唯一确定父,放弃推断
			e.mu.Unlock()
			return "", false
		}
		candidate = id
	}
	e.mu.Unlock()
	return candidate, candidate != ""
}

// resolveParentThreadID 决定子线程的父链归属:显式给了父链就用显式的(经
// collabmcp 工具入参 / proto 字段 6 / DelegateOptions.ParentThreadID 透传);
// 没给则尝试从活跃轮次推断(soleParentTurn)。推断不出如实返回空串 —— 父链
// 留空则手机端子代理卡不亮(诚实变暗),不误报、不伪造。
// 【生产 RPC 入口不用本函数】DelegateSubTask 走 resolveParentThreadIDGuarded
// (X2 深度锁推理防线);本函数保留给引擎级直调方与既有钉子测试(T-S1Pb),
// 二者对"显式父链 / 无活跃轮次 / 唯一父候选"的语义完全一致。
func (e *collabEngine) resolveParentThreadID(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	if pid, ok := e.soleParentTurn(); ok {
		return pid
	}
	// 推断不出:如实留空,不猜不伪造。handler 处会据此记一行日志。
	return ""
}

// hasActiveChildTurn 报告此刻是否有委派子线程的轮次仍在活跃(activeTurns)。
// 深度锁推理防线的判据:有子轮次在飞 = 某个父轮次正同步阻塞在它自己的
// agent_delegate 工具调用上等子返回 —— 阻塞中的线程不可能同时发起新委派。
func (e *collabEngine) hasActiveChildTurn() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, id := range e.srv.activeTurns.IDs() {
		if e.childThreads[id] {
			return true
		}
	}
	return false
}

// resolveParentThreadIDGuarded RPC 入口(DelegateSubTask)专用的父链解析,
// 在 resolveParentThreadID 的推断之上加深度锁推理防线(X2,2026-09-30):
//
// soleParentTurn 的推断依据是"agent_delegate 由父轮次同步发起,父此刻必持
// activeTurns 租约" —— 该前提单向成立:唯一非子活跃轮次也可能只是**仍阻塞
// 中的父轮次**(它在自己早前的 agent_delegate 调用上等子轮次返回),真正的
// 发起方是仍在运行的委派子线程。旧逻辑此时把阻塞父误判为唯一父,推断出的
// 父链指向非子线程,第二道闸(opts.ParentThreadID ∈ childThreads)被
// "子线程再委派"完整绕过。本函数的防线:推断出唯一候选、且此刻仍有委派
// 子轮次在飞时,不再信任推断,以 errDelegateDepthRejected 如实拒绝 ——
// 同步阻塞语义保证候选父不可能是本次发起方,发起方只能是(某个代理进程树上
// 的)子线程。
//
// 被误拒的只剩两种窄窗口,均为"稍后重试即可"的保守面、错误如实说明(准则 6):
//  1. 协调器借并行工具调用同时派发多个委派(一个会话内同时两个在飞);
//  2. 父轮次刚被取消、子轮次尚未收尾的瞬间,另一协调器恰好发起委派。
//
// 深度锁是安全不变量("子不生子"),宁可保守拦截也不放行 —— 与并发闸
// "超限如实报错、由 coordinator 择机重试"同一纪律。
//
// 显式父链不经本防线(它由引擎入口闸按 childThreads 精确判定,语义不变)。
func (e *collabEngine) resolveParentThreadIDGuarded(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit), nil
	}
	pid, ok := e.soleParentTurn()
	if !ok {
		// 推断不出(无活跃轮次/候选不唯一):如实留空,不猜不伪造。
		return "", nil
	}
	if e.hasActiveChildTurn() {
		// 唯一候选 + 有子轮次在飞:候选父此刻必然阻塞在自己的 agent_delegate
		// 调用上,不可能是发起方 → 发起方是子线程 → 深度拒绝。
		return "", errDelegateDepthRejected
	}
	return pid, nil
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
	// 2.5) 可选父链(T-S1Pa):发起方显式给父线程 id(proto 字段 6 / delegate
	// 工具入参 parent_thread_id)且非空时,子线程 ParentThreadID = 该值,随
	// 下面的 Store.Update 一并落库 —— 手机端 thread/list 由此把子线程归入
	// 父会话的子代理组,点亮内联卡;空(缺省)维持父链为空,卡不亮不误报,
	// 与 T-S1Pa 之前行为一致。不校验父 id 是否在库(诚实透传:归属登记不
	// 做第二份存在性裁决;手机端按本地 id 匹配,匹配不上自然不亮)。
	// 【与 codex 存根链正交(T-S1P,核实结论)】codex 原生子代理线程的归属
	// 走另一条路:SyncThreadEvent 的 thread/started 钩子
	// (maybeRegisterCodexSubagentStub,codex_subagent_sync.go)以**轻量存根**
	// 登记不在库的新线程。两条链不重叠也不会互相覆盖:①本函数建的委派子
	// 线程 CodexThreadID 为空(建线程时不带 codex 会话),存根链按
	// CodexThreadID/线程 id 幂等闸命中"已在库"直接返回,永不改写本处落库
	// 的 ParentThreadID;②存根链只在"本地库无此线程"时才创建,本处总是
	// 先建后写,无竞争面。
	if pid := strings.TrimSpace(opts.ParentThreadID); pid != "" {
		t.ParentThreadID = pid
	}
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
	t.TurnStartedUnixMs = time.Now().UnixMilli() // 轮次开始时刻:与 StartTurn 同口径
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
	// 父链归属:agent_delegate 入参 parent_thread_id 本应由宿主/链路基建注入,
	// 但 v1 全仓从未赋值(实证:子智能体卡片不显示)。此处补齐"宿主自动注入":
	// 显式给了就用显式的;没给则从活跃轮次推断唯一父轮次(resolveParentThreadIDGuarded,
	// 带 X2 深度锁推理防线:唯一候选 + 子轮次在飞 = 发起方是子线程,如实拒绝,
	// 详见该函数注释)。推断不出如实留空、手机端子代理卡变暗,不伪造父链(准则 6)。
	parentID, perr := eng.resolveParentThreadIDGuarded(req.GetParentThreadId())
	if perr != nil {
		// 深度锁推理防线拦截:业务失败走 error 字段 + 人话原文,不建子线程、
		// 不伪造成功、不靠 gRPC status 承载(准则 6)。
		log.Printf("DelegateSubTask: 深度锁推理防线拦截(唯一父候选仍阻塞在自身委派上,发起方应为委派子线程): %v", perr)
		return &pb.DelegateSubTaskResponse{Error: perr.Error()}, nil
	}
	if strings.TrimSpace(req.GetParentThreadId()) == "" && parentID == "" {
		// 推断失败的如实留痕:不 guess、不粉饰,便于排障。
		log.Printf("DelegateSubTask: 无法从活跃轮次推断父线程(parent_thread_id 为空且活跃父轮次不唯一),子线程父链留空")
	}
	// 协同角色展开(引用即生效):role_id 非空时,角色的人设被前置拼进子任务
	// prompt,它的 agent/workspace/model/effort 兜底**本次没显式给**的字段
	// (显式值优先,详见 resolveDelegateWithRole 注释)。
	// 为什么在这里一次性展开而不是透传给引擎:角色是"配置便利"而非引擎能力,
	// 展开后与父模型把值写全的效果完全等价 —— 引擎因此不必知道自己需要查配置。
	resolved, rerr := s.resolveDelegateWithRole(req)
	if rerr != nil {
		// 未定义/非法的角色 = **业务失败**,走 error 字段 + 人话,不靠 gRPC
		// status 承载(准则 6)—— 是否重试由 coordinator 自己判断。伪造成功
		// (悄悄无视 role_id 当没配过)才是红线:那会让父模型以为人设已生效。
		return &pb.DelegateSubTaskResponse{Error: rerr.Error()}, nil
	}
	childID, summary, derr := eng.DelegateForDevice(ctx, deviceID,
		resolved.agentID, resolved.task, collabmcp.DelegateOptions{
			Workspace: resolved.workspace,
			Model:     resolved.model,
			Effort:    resolved.effort,
			// T-S1Pa：可选父链（proto 字段 6 / 工具入参 parent_thread_id）经
			// resolveParentThreadID 解析后透传入引擎;非空 = 子线程父链落库、
			// 手机端点亮内联卡;空 = 父链为空,诚实变暗。
			ParentThreadID: parentID,
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
