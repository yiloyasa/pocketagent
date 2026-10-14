// hermes_compact.go —— hermes 的「压缩上下文」接线(适配器 slash 命令 /compress)。
//
// 【为什么走会话内文本命令而不是协议方法】2026-10-14 探针实测
// (docs/reviews/2026-10-14/dsh-hermes-capability-probe.md §二 compact 行):
//   - 协议面没有压缩:session/compact → -32601(本机 live 实测);
//   - 但适配器在 prompt() 里做文本分派:text_only 且以 "/" 开头 →
//     _handle_slash_command(server.py:826-841);/compress 走 commands.py
//     compress_now 真压缩(mid-turn blocked 命令之一),空会话秒回
//     "Nothing to compress — conversation is empty."(stopReason=end_turn,
//     实测帧),非空会话经 agent_message_chunk 回结果文本。
// 即:压缩能力在适配器里**真实存在**,缺的只是 daemon 侧接线 —— 本文件把它
// 当成一轮极短的 prompt 跑,回执按 ACP 终局语义判成败(不伪造、不静默)。
//
// 【三道闸,顺序不能换】
//  1. 轮次在跑 → 拒绝:turnActive 快判 + turnMu.TryLock 互斥。/compress 是
//     mid-turn blocked 命令,放进来只会被适配器拒;更硬的理由是钩子归属 ——
//     notif/req 钩子是 client 级的,Run 的 stopPump 会 SetTurnHandlers(nil,nil),
//     并发会互拆对方的钩子,必须串行;
//  2. 会话没落地(既无内存引用又无 ExternalSession)→ 拒绝:没东西可压,与
//     pi(zcode)同文案口径,不静默新开会话;
//  3. 预算(hermesCompactBudget)耗尽 → 先发 session/cancel 让适配器真停,
//     再如实报错,不留"还在压"的悬空轮次(Run 的看门狗三闸同源教训)。
//
// 【审批通道:失败安全,绝不代答】压缩轮若发 session/request_permission,
// 走 handleRequestPermission 的 approve=nil 分支回 JSON-RPC error(-32000):
// 手机此刻不在轮次流里、没有审批卡可弹,基础设施缺失不伪装成用户意志(红线 1)。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/store"
)

// hermesCompressCommand 是适配器 slash 面板里的压缩命令(live /help 清单在列:
// /help /model /tools /context /reset /compress /steer /queue /version)。
const hermesCompressCommand = "/compress"

// hermesCompactBudget 压缩轮的总预算。真压缩(compress_now)要改写历史并回放,
// 量级大于普通短回执,与 pi 的 piCompactBudget 同档取 180s。包级变量仅为测试
// 注入短值(照 piCompactBudget / hermesLockWait 先例),生产不改。
var hermesCompactBudget = 180 * time.Second

// CompactThread 实现 host.go 的 piShapedCompactor 形状 CompactThread(*store.Thread)
// —— server 层 CompactThread RPC 按 AgentID 路由到它,能力位 compact:hermes 由
// 同一形状断言生成。同步等压缩轮终局后返回。
func (p *HermesProvider) CompactThread(t *store.Thread) error {
	if t == nil {
		return fmt.Errorf("hermes: 没有可压缩的会话")
	}
	// 闸① 轮次在跑 / 另一个压缩在跑 → 拒绝(先快判再 TryLock:争用时零开销,
	// 拿不到锁 = 别的操作正持有轮次互斥面,绝不排队等它跑完)。
	p.mu.Lock()
	running := p.turnActive
	p.mu.Unlock()
	if running {
		return errHermesCompactBusy
	}
	if !p.turnMu.TryLock() {
		return errHermesCompactBusy
	}
	defer p.turnMu.Unlock()

	// 闸② 会话没落地 → 拒绝(内存引用在场 = 本进程里真跑过;ExternalSession
	// 在场 = 跨重启可 session/load 重挂载;两者皆无 = 没东西可压)。
	if _, ok := p.threadRefFor(t.ID); !ok && t.ExternalSession.SessionID == "" {
		return errHermesNotLanded
	}

	workspace := t.Workspace
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	ctx, cancel := context.WithTimeout(context.Background(), hermesCompactBudget)
	defer cancel()

	c, err := p.ensureClient(ctx)
	if err != nil {
		return err
	}
	// 会话就绪(与 Run 同一条分路:缓存命中 → ExternalSession 重挂载 → 新建)。
	// 走到闸② 的必是前两种之一,绝不会为压缩凭空开会话。
	sid, err := p.ensureSession(ctx, c, TurnRequest{Thread: t}, workspace)
	if err != nil {
		return err
	}

	// 当轮钩子:压缩轮的 session/update 只做回执留痕(不投影 sink —— 手机此刻
	// 不在这条轮次流里),反向请求按失败安全回 error(见文件头)。钩子随本函数
	// 卸载,与 Run 的 stopPump 同一口径。
	var reply strings.Builder
	c.SetTurnHandlers(func(raw json.RawMessage) {
		ev := hermesapp.ParseSessionUpdate(raw)
		if ev.SessionID == sid && ev.MessageChunk != "" {
			reply.WriteString(ev.MessageChunk)
		}
	}, func(id int64, method string, params json.RawMessage) {
		p.handleRequestPermission(c, id, method, params, ctx, func(*pb.TurnServerMessage) {}, nil, t.ID)
	})
	defer c.SetTurnHandlers(nil, nil)

	log.Printf("[hermes] 手动压缩开始 thread=%s session=%s", t.ID, sid)
	out, err := c.Prompt(ctx, sid, hermesCompressCommand)
	if err != nil {
		if ctx.Err() != nil {
			// 预算耗尽:适配器可能还在压缩,先发取消(Run 看门狗同源),不留悬空轮次。
			p.cancelTurn(c, sid)
			return fmt.Errorf("hermes 压缩超时(预算 %v),已发 session/cancel", hermesCompactBudget)
		}
		return fmt.Errorf("hermes /compress: %w", err)
	}
	if out == nil || strings.TrimSpace(out.StopReason) == "" {
		return fmt.Errorf("hermes /compress 没有回执(stopReason 为空),不判定为压缩成功")
	}
	if hermesapp.StopReasonIsError(out.StopReason) {
		return fmt.Errorf("hermes 压缩失败(stopReason=%s)", out.StopReason)
	}
	// 正文判据与 Run 同源:end_turn 只是"轮次收尾了",适配器把 provider 失败也
	// 包装成 end_turn + 错误正文 —— 压缩回执为空 = 命令没被识别/没有产出,
	// 如实报错,不伪造成功(探针实测:空会话与真压缩都会回一段正文)。
	final := strings.TrimSpace(reply.String())
	if final == "" {
		return fmt.Errorf("hermes /compress 没有回执正文,不判定为压缩成功")
	}
	log.Printf("[hermes] 手动压缩完成 thread=%s session=%s 回执=%q", t.ID, sid, clipString(final, 200))
	return nil
}

// errHermesCompactBusy 忙哨兵(轮次在跑 / 另一个压缩在持锁)。与 errHermesBusy
// 同族文案,但 Unwrap 到 ErrBusy 由调用方按 errors.Is 统一识别忙态。
var errHermesCompactBusy = &busySentinelError{msg: "hermes 正在执行任务,等它跑完再压缩"}

// errHermesNotLanded 会话没落地(pi PI_NOT_MOUNTED / zcode 同文案口径)。
var errHermesNotLanded = fmt.Errorf("这个会话还没有在电脑上落地(先发一条消息),暂时无法压缩")

// 编译期钉死:hermes 声明与 pi 同形状的压缩接口(server CompactThread 路由 +
// host.go piShapedCompactor 能力位断言都按形状走,不按 agent 名字分支)。
var _ interface{ CompactThread(*store.Thread) error } = (*HermesProvider)(nil)
