// collab_policy.go —— 协同策略的控制面(规格 §11)。
//
// 【它解决什么】"允不允许 agent 互相派活、一次最多几家并行"此前是**启动参数**
// (-collab-max-parallel)与手机本地开关的混合体:前者只有重启才能改,后者换台手机
// 就没了、而且实际上没有任何执行点在读它。本文件让策略由 daemon 持久化、可就地修改、
// 并且**真的**作用在扇出与委派入口上。
//
// 【错误语义】与既有服务一致走 gRPC status:参数/校验 InvalidArgument,
// 并发冲突 FailedPrecondition,落盘失败 Internal。
package server

import (
	"context"
	"fmt"
	"log"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// collabPolicySnapshot 取策略快照(读锁 + 值拷贝)。
//
// 【为什么每次现取】策略可被 SetCollabPolicy 就地修改,"改完立即生效"的判据就是
// 下一个读取方看到新值。缓存一份算好的结果会把生效时机推迟到下次重启。
func (s *Server) collabPolicySnapshot() runtime.CollabPolicy {
	s.collabMu.RLock()
	defer s.collabMu.RUnlock()
	return s.collabPolicy
}

// GetCollabPolicy 读回当前协同策略。
func (s *Server) GetCollabPolicy(_ context.Context, _ *pb.GetCollabPolicyRequest) (*pb.CollabPolicyResponse, error) {
	return s.collabPolicyResponse(s.collabPolicySnapshot()), nil
}

// SetCollabPolicy 提交新策略:校验 → CAS → 原子落盘 → 立即生效。
func (s *Server) SetCollabPolicy(_ context.Context, req *pb.SetCollabPolicyRequest) (*pb.CollabPolicyResponse, error) {
	if req.GetPolicy() == nil {
		return nil, errInvalidArg("缺少 policy")
	}
	next := collabPolicyFromProto(req.GetPolicy())

	s.collabMu.Lock()
	defer s.collabMu.Unlock()

	if exp := req.GetExpectedRevision(); exp != 0 && exp != s.collabPolicy.Revision {
		return nil, errFailedPrecondition(fmt.Sprintf(
			"协同策略已被改动(revision=%d,期望 %d),请重新读取后再提交", s.collabPolicy.Revision, exp))
	}
	// 写入代数由 daemon 递增,不接受客户端指定值。
	next.Revision = s.collabPolicy.Revision + 1
	if err := next.Validate(); err != nil {
		return nil, errInvalidArg(err.Error())
	}
	if err := runtime.SaveCollabPolicy(s.cfg.DataDir, next); err != nil {
		// 落盘失败如实上报:回成功会让用户以为策略生效了,而下次重启就变回旧值。
		return nil, errInternal(err)
	}
	s.collabPolicy = next
	log.Printf("[collab] 协同策略已更新 revision=%d(已落盘 %s)",
		next.Revision, runtime.CollabPolicyPath(s.cfg.DataDir))
	return s.collabPolicyResponse(next), nil
}

// collabPolicyResponse 组装响应:用户配置 + 实际生效值(打底默认后)。
func (s *Server) collabPolicyResponse(p runtime.CollabPolicy) *pb.CollabPolicyResponse {
	return &pb.CollabPolicyResponse{
		Policy:     collabPolicyToProto(p),
		Revision:   p.Revision,
		PolicyPath: runtime.CollabPolicyPath(s.cfg.DataDir),
		Effective:  collabPolicyToProto(p.Effective()),
	}
}

// collabPolicyToProto 把领域模型翻成契约形态(唯一转换点,避免 optional/指针对照漏项)。
func collabPolicyToProto(p runtime.CollabPolicy) *pb.CollabPolicy {
	out := &pb.CollabPolicy{
		CollaborationEnabled:  p.CollaborationEnabled,
		NativeDelegateEnabled: p.NativeDelegateEnabled,
		FanoutEnabled:         p.FanoutEnabled,
	}
	if p.MaxParallelMembers != nil {
		v := int32(*p.MaxParallelMembers)
		out.MaxParallelMembers = &v
	}
	return out
}

func collabPolicyFromProto(p *pb.CollabPolicy) runtime.CollabPolicy {
	if p == nil {
		return runtime.CollabPolicy{}
	}
	out := runtime.CollabPolicy{
		CollaborationEnabled:  p.CollaborationEnabled,
		NativeDelegateEnabled: p.NativeDelegateEnabled,
		FanoutEnabled:         p.FanoutEnabled,
	}
	if p.MaxParallelMembers != nil {
		v := int(*p.MaxParallelMembers)
		out.MaxParallelMembers = &v
	}
	return out
}
