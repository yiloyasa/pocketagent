// runtime_policy.go —— 资源预算的控制面(规格 §6.5 / §13 P1 / §14)。
//
// 【它解决什么】预算此前只有"启动时读一次文件"这一条路:用户要调额度就得手编
// <数据目录>/runtime-policy.json,再重启 daemon —— 有配置、没有控制面。本文件把
// 策略变成运行时可读可改的一等对象:提交即原子落盘、立即生效(所有读取方拿到的都是
// 这份内存态),并把"额度传给了谁"经 GetResourceUsage 变成可观察的量。
//
// 【错误语义】与既有服务一致走 gRPC status:参数/校验问题 InvalidArgument,
// 并发冲突 FailedPrecondition,落盘失败 Internal。
package server

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// effectiveLimits 取某上下文下当前生效的预算。
//
// 【为什么每次重新取,而不是启动时算好存着】预算可以在运行中被改
// (SetRuntimePolicy),而"改完立即生效"的判据正是:下一个读取方看到新值。
// 缓存一份算好的结果会把生效时机推迟到"下次重启 daemon" —— 那正是本波要消掉的形态。
func (s *Server) effectiveLimits(agentID, workspace, threadID, collabID string) runtime.Limits {
	s.budgetMu.RLock()
	policy := s.budgetPolicy
	s.budgetMu.RUnlock()
	return policy.Effective(agentID, workspace, threadID, collabID)
}

// GetRuntimePolicy 读回当前生效的预算策略。
//
// 【为什么 Set 之外还要 Get】手机换机/重装后没有本地副本;没有 Get,"能改"就退化成
// "只能盲改" —— 用户看不到当前值,也就无从判断改没改对、是不是被别人改过。
func (s *Server) GetRuntimePolicy(_ context.Context, _ *pb.GetRuntimePolicyRequest) (*pb.RuntimePolicyResponse, error) {
	s.budgetMu.RLock()
	policy := s.budgetPolicy
	// 实际生效值在同一次读锁内算出:分两次取锁时,中间恰好来一次 Set 就会返回
	// 一对"配置与生效值不属于同一代"的结果 —— 客户端拿去做 diff 会看到假变化。
	effective := policy.Effective("", "", "", "")
	s.budgetMu.RUnlock()
	return &pb.RuntimePolicyResponse{
		Policy:          policyToProto(policy),
		Revision:        policy.Revision,
		PolicyPath:      runtime.BudgetPolicyPath(s.cfg.DataDir),
		EffectiveGlobal: limitsToProto(effective),
	}, nil
}

// SetRuntimePolicy 提交新策略:校验 → CAS → 原子落盘 → 立即生效。
func (s *Server) SetRuntimePolicy(_ context.Context, req *pb.SetRuntimePolicyRequest) (*pb.RuntimePolicyResponse, error) {
	if req.GetPolicy() == nil {
		return nil, errInvalidArg("缺少 policy")
	}
	next := policyFromProto(req.GetPolicy())

	s.budgetMu.Lock()
	defer s.budgetMu.Unlock()

	if exp := req.GetExpectedRevision(); exp != 0 && exp != s.budgetPolicy.Revision {
		// 策略可能被两处同时改(手机设置页 / 电脑上直接编辑文件)。没有这一道,
		// 后提交的一方会静默覆盖另一方 —— 用户看到的是"我明明改过"。
		return nil, errFailedPrecondition(fmt.Sprintf(
			"策略已被改动(revision=%d,期望 %d),请重新读取后再提交", s.budgetPolicy.Revision, exp))
	}
	// 写入代数由 daemon 递增,不接受客户端指定的值 —— 否则客户端凭一个更大的
	// revision 就能让后续所有 CAS 永久失败。
	next.Revision = s.budgetPolicy.Revision + 1
	if err := next.Validate(); err != nil {
		return nil, errInvalidArg(err.Error())
	}
	if err := runtime.SaveBudgetPolicy(s.cfg.DataDir, next); err != nil {
		// 落盘失败必须如实上报:回"成功"会让用户以为策略生效了,而下一轮跑出来的
		// 还是旧额度 —— 内存态与磁盘谁说了算,必须始终是同一个答案。
		return nil, errInternal(err)
	}
	s.budgetPolicy = next
	effective := next.Effective("", "", "", "")
	// 【DiskConversationBytes 热替换】归档目录总量上限的唯一执行体在 store
	// (SetArchiveMaxBytes → evictArchiveLocked),启动装配期由 main 接线一次;
	// 热替换路径若不同步它,改预算后"内存策略已是新值、归档上限仍是旧值" ——
	// 两份真值,与本控制面"改完立即生效"的承诺相反。nil store(测试极简构造)
	// 由 SetArchiveMaxBytes 的 nil 守卫兜住;Effective 已打底内置基线,缺省即
	// 回 32MB 默认,与装配期"未设置维持默认"同口径。
	s.cfg.Store.SetArchiveMaxBytes(derefInt64(effective.DiskConversationBytes))
	log.Printf("[budget] 策略已更新 revision=%d(已落盘 %s)",
		next.Revision, runtime.BudgetPolicyPath(s.cfg.DataDir))
	return &pb.RuntimePolicyResponse{
		Policy:          policyToProto(next),
		Revision:        next.Revision,
		PolicyPath:      runtime.BudgetPolicyPath(s.cfg.DataDir),
		EffectiveGlobal: limitsToProto(effective),
	}, nil
}

// GetResourceUsage 返回某作用域下的租约明细(额度 + 用量 + 压力档)。
func (s *Server) GetResourceUsage(_ context.Context, req *pb.GetResourceUsageRequest) (*pb.ResourceUsageResponse, error) {
	resp := &pb.ResourceUsageResponse{SampledAtUnixMs: time.Now().UnixMilli()}
	if s.runtimeBackend == nil {
		// 不记账模式(测试/极简构造路径):回空清单而不是报错。对客户端而言
		// "没有租约"与"没有记账"看到的都是空;而报错会让诊断页把正常空态显示成故障。
		return resp, nil
	}
	rep := s.runtimeBackend.Resources().Usage(context.Background(), req.GetScope())
	for _, it := range rep.Items {
		resp.Metrics = append(resp.Metrics, resourceMetricToProto(it))
	}
	return resp, nil
}

// resourceMetricToProto 把一份资源租约翻成契约形态。
//
// 【为什么抽成函数】GetResourceUsage 与 GetAgentRuntime 都要它。两处各写一遍时,
// 将来新增一个字段只会改到其中一处 —— 表现为"某个界面上这一项永远是空的",
// 而没有任何测试会喊。
func resourceMetricToProto(it runtime.ResourceLease) *pb.ResourceMetric {
	return &pb.ResourceMetric{
		LeaseId:        it.LeaseID,
		Scope:          it.OwnerScope,
		Kind:           string(it.Kind),
		ReservedBytes:  it.ReservedBytes,
		UsedBytes:      it.UsedBytes,
		SoftLimitBytes: it.SoftLimit,
		HardLimitBytes: it.HardLimit,
		State:          string(it.Pressure()),
		SpillPolicy:    string(it.SpillPolicy),
		Optional:       it.Optional,
	}
}

// ---- 契约 ↔ 领域模型转换 ------------------------------------------------------
//
// 【为什么转换必须集中一处】转换散开时,proto3 optional(presence 位)与 Go 指针
// 之间的对照最容易漏:漏一项的表现是"某一项改了不生效"或"某一项没设却被清零",
// 而这两类错误都不会让任何测试变红 —— 它们只在用户手上显形。

func limitsToProto(l runtime.Limits) *pb.RuntimeLimits {
	out := &pb.RuntimeLimits{
		RamWorkingBytes:       l.RAMWorkingBytes,
		RamEventBufferBytes:   l.RAMEventBufferBytes,
		RamSessionIndexBytes:  l.RAMSessionIndexBytes,
		DiskConversationBytes: l.DiskConversationBytes,
		DiskCacheBytes:        l.DiskCacheBytes,
		DiskArtifactBytes:     l.DiskArtifactBytes,
	}
	if l.MaxParallelTurns != nil {
		v := int32(*l.MaxParallelTurns)
		out.MaxParallelTurns = &v
	}
	return out
}

func limitsFromProto(p *pb.RuntimeLimits) runtime.Limits {
	if p == nil {
		return runtime.Limits{}
	}
	out := runtime.Limits{
		RAMWorkingBytes:       p.RamWorkingBytes,
		RAMEventBufferBytes:   p.RamEventBufferBytes,
		RAMSessionIndexBytes:  p.RamSessionIndexBytes,
		DiskConversationBytes: p.DiskConversationBytes,
		DiskCacheBytes:        p.DiskCacheBytes,
		DiskArtifactBytes:     p.DiskArtifactBytes,
	}
	if p.MaxParallelTurns != nil {
		v := int(*p.MaxParallelTurns)
		out.MaxParallelTurns = &v
	}
	return out
}

func policyToProto(p runtime.BudgetPolicy) *pb.RuntimePolicy {
	out := &pb.RuntimePolicy{
		Global: limitsToProto(p.Global),
		// 辅线 B 开关:*bool ↔ optional bool 直映(缺席保缺席,Set/Get 自然携带)。
		UnknownRequestPassthrough: p.UnknownRequestPassthrough,
	}
	if len(p.Agents) > 0 {
		out.Agents = make(map[string]*pb.RuntimeLimits, len(p.Agents))
		for k, v := range p.Agents {
			out.Agents[k] = limitsToProto(v)
		}
	}
	if len(p.Workspaces) > 0 {
		out.Workspaces = make(map[string]*pb.RuntimeLimits, len(p.Workspaces))
		for k, v := range p.Workspaces {
			out.Workspaces[k] = limitsToProto(v)
		}
	}
	if len(p.Threads) > 0 {
		out.Threads = make(map[string]*pb.RuntimeLimits, len(p.Threads))
		for k, v := range p.Threads {
			out.Threads[k] = limitsToProto(v)
		}
	}
	if len(p.Collabs) > 0 {
		out.Collabs = make(map[string]*pb.RuntimeLimits, len(p.Collabs))
		for k, v := range p.Collabs {
			out.Collabs[k] = limitsToProto(v)
		}
	}
	return out
}

func policyFromProto(p *pb.RuntimePolicy) runtime.BudgetPolicy {
	out := runtime.BudgetPolicy{}
	if p == nil {
		return out
	}
	out.Global = limitsFromProto(p.GetGlobal())
	// 辅线 B 开关:完整策略提交语义的一部分 —— 缺席 = 未设置(= 关),显式 false =
	// 用户选过关,两者都原样落盘,不在此处归一(归一就该发生在行为读取点)。
	out.UnknownRequestPassthrough = p.UnknownRequestPassthrough
	fill := func(dst *map[string]runtime.Limits, src map[string]*pb.RuntimeLimits) {
		if len(src) == 0 {
			return
		}
		*dst = make(map[string]runtime.Limits, len(src))
		for k, v := range src {
			(*dst)[k] = limitsFromProto(v)
		}
	}
	fill(&out.Agents, p.GetAgents())
	fill(&out.Workspaces, p.GetWorkspaces())
	fill(&out.Threads, p.GetThreads())
	fill(&out.Collabs, p.GetCollabs())
	return out
}

// ---- 轮次强制点（规格 §6.5）--------------------------------------------------
//
// 【它做的事】把 Effective 算出的额度交给 ResourceLedger 的租约 soft/hard,并把
// 这一轮的事件字节记上去。于是"当前策略是多少"不再只是一份配置,而是一个能被
// GetResourceUsage 观察到的运行量。
//
// 【它刻意不做的事:不拒绝】事件承载用户数据(回复文本、工具输出、审批请求),
// 到顶也不丢。触顶的语义是"该压缩/落盘了"与"压力可见",不是"少给用户一点"。
// 真正会拒的是可选资源(缓存/artifact),那些在各自的申请点单独判。

// openTurnLease 为一次轮次申请资源租约。返回 nil = 本次不记账(无后端或申请失败)。
func (s *Server) openTurnLease(ctx context.Context, t *store.Thread, turnID string) *runtime.ResourceLease {
	if s.runtimeBackend == nil || t == nil {
		return nil
	}
	quota := derefInt64(s.effectiveLimits(threadAgentID(t), t.Workspace, t.ID, "").RAMEventBufferBytes)
	lease, err := s.runtimeBackend.Resources().Acquire(ctx, runtime.LeaseRequest{
		LeaseID:    "turn:" + t.ID + ":" + turnID,
		OwnerScope: runtime.ScopeThread(t.ID),
		Kind:       runtime.ResEventBuffer,
		// 额度即上限。当前策略模型对每类资源只给一个数值(规格 §6.5 的基线形态),
		// 故 soft 与 hard 同值:到线即该压缩/落盘。本类资源是必需资源
		// (Optional=false),所以到线也不会被拒 —— 越限只体现在 Usage 的压力里。
		SoftLimit:   quota,
		HardLimit:   quota,
		SpillPolicy: runtime.SpillToDisk,
		Optional:    false,
	})
	if err != nil {
		// 申请失败只记日志:记账是诊断增强,绝不能变成"轮次起不来"的新故障点。
		log.Printf("[runtime] 轮次资源租约申请失败(轮次照常进行): %v", err)
		return nil
	}
	return lease
}

// closeTurnLease 释放租约(用量归零、退出活跃集合)。
func (s *Server) closeTurnLease(lease *runtime.ResourceLease) {
	if s.runtimeBackend == nil || lease == nil {
		return
	}
	if err := s.runtimeBackend.Resources().Release(context.Background(), lease.LeaseID); err != nil {
		log.Printf("[runtime] 轮次资源租约释放失败: %v", err)
	}
	// 刷盘放在轮次收尾这个自然边界,而不是 Charge(它是每帧热路径,逐帧写盘
	// 会把记账投影变成 I/O 瓶颈)。丢失窗口由此缩到"崩溃发生在轮次中途"——
	// 账本是记账投影,这个量级的丢失符合"丢账可接受"。
	if err := s.runtimeBackend.SyncResourceLedger(); err != nil {
		log.Printf("[runtime] 轮次资源账本刷盘失败(记账性质,不影响轮次): %v", err)
	}
}

// chargeTurnLease 把一帧事件的字节数记到本轮租约上。
func (s *Server) chargeTurnLease(lease *runtime.ResourceLease, bytes int64) {
	if s.runtimeBackend == nil || lease == nil || bytes <= 0 {
		return
	}
	if _, err := s.runtimeBackend.Resources().Charge(context.Background(), lease.LeaseID, bytes); err != nil {
		log.Printf("[runtime] 轮次资源记账失败(不影响轮次): %v", err)
	}
}

// derefInt64 取指针值;nil → 0 —— 在预算语义里 0 是"未设上限",不是"封禁"。
func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
