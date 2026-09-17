// codex_compact.go —— 上下文窗口真实自动压缩。
//
// 【为什么必须这层】codex 只在 tokenUsage/updated 通知里告诉我们"现在用了多少、窗口多大",
// 它自己不一定会压缩(尤其老版本 / 配置未开启自动压缩时)。于是窗口被吃满后,
// 手机端看到的"上下文"会越用越满、任务开始丢前情。这里在 daemon 侧做**真实的**
// 自动压缩:占用率越过阈值就调 codex 的 thread/compact/start,等 ContextCompactedNotification
// 回来后用量自然回落,无需手动清零。
//
// 【防抖是关键】同一线程一旦发起压缩,在收到"已压缩"通知或占用率回落之前,
// 绝不能再发起第二次 —— 否则每次 tokenUsage 上报都会戳一次 codex,既打扰又可能
// 触发 codex 自己的限流。防抖状态机(见 compactState / compactDecide)就是为此而生,
// 且 compactDecide 写成纯函数,单独可测。
package agent

import (
	"fmt"
	"log"

	"github.com/pocketagent/daemon/internal/codexapp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// 自动压缩阈值与滞回(防抖)量。
//
//	threshold  : 累计 token / 窗口 ≥ 该值即发起压缩
//	hysteresis : 已发起后,占用率必须回落到 threshold-hysteresis 以下才重新武装,
//	             避免"刚压完、下一帧又满"导致的抖动重复触发。
const (
	autoCompactThreshold  = 0.85
	autoCompactHysteresis = 0.10
)

// compactState 一个 codex 线程的压缩状态机。
type compactState int8

const (
	// compactArmed:已武装,占用率越阈即可发起压缩。
	compactArmed compactState = iota
	// compactFired:已发起压缩,等待 ContextCompactedNotification 或占用率回落。
	compactFired
)

// compactDecide 纯函数:给定上一状态、当前累计 token、窗口大小、阈值与滞回量,
// 返回新状态与本次是否应发起压缩。
//
// 不碰任何 IO、不依赖全局变量,全靠入参决定输出 → 可直接单测。
// 窗口未知(window<=0)时**绝不**触发 —— 没有分母就谈不上占用率。
func compactDecide(state compactState, totalTokens, window int64, threshold, hysteresis float64) (compactState, bool) {
	if window <= 0 {
		return state, false
	}
	ratio := float64(totalTokens) / float64(window)
	switch state {
	case compactFired:
		// 已发起:只有占用率真正回落(压缩生效了)才重新武装;
		// 否则保持待命,屏蔽这中间的每一帧 tokenUsage 上报。
		if ratio < threshold-hysteresis {
			return compactArmed, false
		}
		return compactFired, false
	default: // compactArmed
		if ratio >= threshold {
			return compactFired, true
		}
		return compactArmed, false
	}
}

// maybeAutoCompact 在收到 thread/tokenUsage/updated 时调用:
// 用状态机判断是否要发起压缩,需要则异步发起(不阻塞事件分发)。
//
// last 为**本 turn**用量(pb.UsageUpdate.usage,即 codex 通知里的 tokenUsage.last),
// window 为真实窗口大小(UsageUpdate.model_context_window,0=未知)。
//
// 【token 口径与桌面端一致】codex 的 TokenUsage.total_tokens = input_tokens + output_tokens
// (cachedInputTokens ⊆ inputTokens,reasoningOutputTokens ⊆ outputTokens,
// OpenAI 语义下 cacheWrite 恒为 0),所以占用 = 输入 + 输出,**绝不**把缓存读/推理
// 再加进来 —— 否则缓存命中越高、占用显示越虚高,压缩也会被过早触发。
func (p *CodexProvider) maybeAutoCompact(codexTID string, last *pb.Usage, window int64) {
	if last == nil {
		return
	}
	sum := last.GetInputTokens() + last.GetOutputTokens()

	p.compactMu.Lock()
	st := p.compactState[codexTID]
	ns, fire := compactDecide(st, sum, window, autoCompactThreshold, autoCompactHysteresis)
	p.compactState[codexTID] = ns
	p.compactMu.Unlock()

	if fire {
		log.Printf("[codex] 上下文占用越阈(窗口=%d 累计=%d),自动发起压缩 thread=%s",
			window, sum, codexTID)
		go p.fireCompact(codexTID)
	}
}

// resetCompactState 重新武装某线程的压缩状态机。
// 在收到 ContextCompactedNotification 时调用:压缩已经发生,允许下次再触发。
func (p *CodexProvider) resetCompactState(codexTID string) {
	p.compactMu.Lock()
	delete(p.compactState, codexTID)
	p.compactMu.Unlock()
}

// fireCompact 真正去调 codex 的 thread/compact/start。
//
// 【只记日志,不影响正常 turn】codex 侧可能因"子进程重启中 / turn 还在跑 /
// 协议不支持"而失败 —— 这些都归为日志,绝不让它冒泡到当前正在跑的任务
// (否则用户会因为"压缩没成功"而看不到任务结果)。
func (p *CodexProvider) fireCompact(codexTID string) {
	err := p.withClient(func(c *codexapp.Client) error {
		_, e := c.Call("thread/compact/start", map[string]interface{}{"threadId": codexTID})
		return e
	})
	if err != nil {
		log.Printf("[codex] 自动压缩发起失败(已忽略,不影响当前任务) thread=%s: %v", codexTID, err)
	}
}

// CompactThread 手动压缩入口:把 RPC 落到真实的 thread/compact/start。
// 失败返回明确错误,由 server 层转成 gRPC 错误码。
func (p *CodexProvider) CompactThread(codexTID string) error {
	err := p.withClient(func(c *codexapp.Client) error {
		_, e := c.Call("thread/compact/start", map[string]interface{}{"threadId": codexTID})
		if e != nil {
			return fmt.Errorf("thread/compact/start: %w", e)
		}
		return nil
	})
	return err
}
