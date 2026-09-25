// pi_followup.go —— pi 的 followUp 原生投递(FollowUpper 可选能力)与
// queue_update 事件 → Queued/QueueFlushed 回执链的对账。
//
// 【为什么要有这一层】pi 0.87.1 的 `prompt` 命令带 `streamingBehavior:"followUp"`
// 时,输入进入 pi 子进程内存里的 followUp 队列,**当前轮结束后、settled 前自动
// 续跑**——整段续跑发生在同一次 provider.Run 里,daemon 不必再拿自己的内存队列
// 接力(那条路径仍保留为兜底)。证据:docs/verify/evidence/pi-followup-wire-probe.md
// (bundle 逐行核对 + pi-s-probe.md 活体帧)。
//
// 【红线】本层只做"投递"与"对账",不碰 pi 的任何 set_auto_* 写通道(上轮探针
// 实证那些调用会落盘用户 ~/.pi 全局配置)。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/piapp"
)

// FollowUpper 是「轮次进行中把输入预投递为上游原生 followUp 队列」的可选扩展能力
// (与 Steerer / QueueClearer 同款探测模式:server 层类型断言,想支持的 provider
// 自行实现)。原生队列优先,daemon 内存队列降级为兜底。
//
// 【与 Steerer 的分工】steer = 插进**当前**轮次(下一次 LLM 调用前生效);
// followUp = 当前轮**结束后**才执行。server 的投递点先试 steer,steer 插不进
// (轮次间隙/上游拒收)再试 followUp,最后才落 daemon 队列。
type FollowUpper interface {
	// FollowUp 把一条输入预投递进 threadID 当前轮次的上游 followUp 队列。
	// delivered=false 表示「此刻投不进」(无运行中轮次/本轮已 settled/子进程
	// 不可用/上游拒绝),调用方应回落 daemon 内存队列兜底 —— 输入绝不丢。
	FollowUp(ctx context.Context, threadID string, blocks []pb.ContentBlock) (delivered bool, err error)
}

// piFollowUpBudget 单次 followUp 投递的上界(受理毫秒级;上界只兜子进程临终
// 一刻。与 Steer 内部的 15s 同口径)。
const piFollowUpBudget = 15 * time.Second

// 编译期钉死能力声明(与 QueueClearer 的 var _ 同款)。
var _ FollowUpper = (*PiProvider)(nil)

// promptRPC followUp 投递对 piapp.Client 的窄接口(单测打桩钉外发形状,不依赖
// 子进程;piapp.Client 的 Prompt 签名天然满足,生产零适配)。
type promptRPC interface {
	Prompt(ctx context.Context, message string, images []any, streamingBehavior string) error
}

// FollowUp 实现 FollowUpper。
//
// 【两道守卫,都不发网络请求】
//  1. active:该线程必须有运行中的轮次(与 Steer 同款登记;Run 收尾清除);
//  2. settled:本轮尚未见过 agent_settled —— settled 后 pi 已空闲,prompt 会
//     **立即执行成新轮次**而非排队(pi 空闲时忽略 streamingBehavior,证据
//     §1),那就不是 followUp 语义了。守卫把 Run 收尾阶段(get_entries/统计
//     等秒级窗口)的投递请求挡回 daemon 队列兜底。
func (p *PiProvider) FollowUp(ctx context.Context, threadID string, blocks []pb.ContentBlock) (bool, error) {
	p.steerMu.Lock()
	streaming := p.active[threadID]
	settled := p.settled[threadID]
	p.steerMu.Unlock()
	if !streaming {
		return false, errors.New("当前没有正在运行的轮次")
	}
	if settled {
		return false, errors.New("本轮已终局,不再受理原生排队")
	}
	c := p.aliveClient(threadID)
	if c == nil {
		return false, errors.New("Pi 子进程不可用")
	}
	return p.followUpOn(ctx, c, threadID, blocks)
}

// followUpOn 真实投递(抽窄接口参数便于打桩,与 clearQueueOn/sendSessionName 同模式)。
func (p *PiProvider) followUpOn(ctx context.Context, c promptRPC, _ string, blocks []pb.ContentBlock) (bool, error) {
	message, images := blocksToPrompt(blocks)
	sctx, cancel := context.WithTimeout(ctx, piFollowUpBudget)
	defer cancel()
	// 失败(含旧版 pi 的"流式期间不认识 streamingBehavior"类拒收)原样上抛,
	// 由 server 回落 daemon 队列 —— 与 Steer 失败转排队同语义,输入不丢。
	if err := c.Prompt(sctx, message, images, piapp.PromptFollowUp); err != nil {
		return false, err
	}
	return true, nil
}

// ---- queue_update → Queued/QueueFlushed 回执链 ----

// reconcileNativeQueue 用一条 queue_update 事件对账原生队列,产出该发给手机的
// 回执帧(0..n 个)。
//
// 【为什么要对账】预投递成功的输入不再经 daemon 队列,手机端的"排队计数"靠
// Queued(+1)/QueueFlushed(−1) 帧维持;pi 的 queue_update 带的是**完整当前
// 队列**(followUp 数组,证据 §2),与上一水位比较即可无损推断入队/出队条数:
//   - 变长(我们或上游刚入队)→ 每多一条发 1 帧 Queued{Position/Depth=队列长};
//   - 变短(pi 取走队首续跑 / clear_queue)→ 每少一条发 1 帧 QueueFlushed
//     {Remaining=出队后队列长};
//   - steering 数组的变化不产生帧(它不占手机排队计数,且由 Steered 回执负责)。
//
// 【水位跨 Run 存续】原生队列活在 pi 子进程内存里,跨轮次(甚至跨"轮次失败"
// 的续跑)都有效,所以 fuSeen 不在 Run 开始时清零 —— 上一 Run 没清完的条目在
// 本 Run 出队时照样收账。脏报文按"看不懂就不出帧"处理,绝不伪造回执。
func (p *PiProvider) reconcileNativeQueue(threadID string, raw json.RawMessage) []*pb.TurnServerMessage {
	var e struct {
		FollowUp []string `json:"followUp"`
	}
	if json.Unmarshal(raw, &e) != nil {
		return nil
	}
	p.steerMu.Lock()
	prev := p.fuSeen[threadID]
	now := len(e.FollowUp)
	var out []*pb.TurnServerMessage
	switch {
	case now > prev:
		for pos := prev + 1; pos <= now; pos++ {
			out = append(out, &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Queued{Queued: &pb.Queued{
				Position: int32(pos), Depth: int32(now),
			}}})
		}
	case now < prev:
		for i := 0; i < prev-now; i++ {
			out = append(out, &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_QueueFlushed{QueueFlushed: &pb.QueueFlushed{
				// TurnId 有意留空:这不是 daemon 续跑出的新轮次(pi 在同一
				// settled 周期内续跑,流与"运行中"状态保持),手机端只用本帧
				// 收计数、撤"排队中"标签。
				Remaining: int32(now),
			}}})
		}
	}
	p.fuSeen[threadID] = now
	p.steerMu.Unlock()
	return out
}

// forgetNativeQueue 丢弃该线程的原生队列水位(线程删除时随 clients/cursors
// 一并回收;重挂载后的新子进程队列必然为空)。
func (p *PiProvider) forgetNativeQueue(threadID string) {
	p.steerMu.Lock()
	delete(p.fuSeen, threadID)
	delete(p.settled, threadID)
	p.steerMu.Unlock()
}
