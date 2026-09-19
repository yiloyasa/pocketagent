// codex_goal.go —— 常驻目标(thread/goal/*)的接入。
//
// 【为什么需要这个文件】Codex 侧有完整的 goal 面:thread/goal/set|get|clear,
// 外加状态机(active / paused / blocked / usageLimited / budgetLimited / complete)、
// tokenBudget 与 thread/goal/updated|cleared 通知。
//
// 而我们的 SetThreadGoal 以前只有一句 `t.Goal = req.Goal` + 落库 ——
// 手机端设的"常驻目标"只存在于我们自己的库里,codex 根本不知道有这回事。
// 表现就是用户说的"我明明设了目标,它好像没当回事":这是个假开关。
//
// 这里的职责就一件事:把目标真正下发,并处理"暂时下发不了"的两种情况 ——
//  1. 线程还没被 app-server 加载(从没跑过第一轮,或子进程重启后还没 resume)
//     → 先暂存,Run() 里拿到 codex threadId 后补发,保证"先设目标再开聊"也生效;
//  2. 清空目标 → 走 thread/goal/clear,而不是 set 一个空串。
package agent

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
)

// loadedCodexThreadID 取该线程在当前子进程里已加载的 codex threadId;没有则返回空。
//
// 必须比对 gen:子进程重启后 threadMap 里的旧 id 属于上一个进程,
// 拿它去调用会被 codex 静默吞掉(和 thread/start 那个坑同源)。
func (p *CodexProvider) loadedCodexThreadID(threadID string) string {
	gen := int(p.gen.Load())
	if ref, hit := p.codexThreadRef(threadID); hit && ref.gen == gen {
		return ref.id
	}
	return ""
}

// SetGoal 把常驻目标下发给 codex。
// threadID 是我们自己的会话 id;goal 为空串表示清除目标。
//
// 下发不了(线程未加载)不算失败:暂存起来等 Run() 补发,调用方照常落库即可。
func (p *CodexProvider) SetGoal(threadID, goal string) error {
	mounted := false
	err := p.withStatusClient(func(ctx context.Context, c *codexapp.Client) error {
		id := p.loadedCodexThreadID(threadID)
		if id == "" {
			return nil
		}
		mounted = true
		return pushGoalViaContext(ctx, c, id, goal)
	})
	if err != nil || !mounted {
		p.stashGoal(threadID, goal)
	}
	return err
}

// stashGoal 记录/清除待下发目标。
func (p *CodexProvider) stashGoal(threadID, goal string) {
	p.goalMu.Lock()
	defer p.goalMu.Unlock()
	p.pendingGoals[threadID] = goal
}

// pushGoal 真正调用 codex 的 goal 接口。
func (p *CodexProvider) pushGoal(codexTID, goal string) error {
	return pushGoalVia(p.client, codexTID, goal)
}

// pushGoalVia 是 pushGoal 的可测核心(同 codexXxxVia 模式:caller 可注入 fake)。
//
// 【字段名实锤(2026-09-19 live,codex-cli 0.149.1)】thread/goal/set 的参数名是
// **objective**(v2 schema ThreadGoalSetParams:properties 只有 threadId / objective /
// status / tokenBudget,根本没有 goal);旧形状 {"goal":...} 会被 -32600
// "no goal exists" 拒绝(证据:cmd/codexreadprobe 实测输出)。
// clear 的参数只有 threadId(ThreadGoalClearParams),两版一致不受影响。
func pushGoalVia(c codexJSONCaller, codexTID, goal string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return pushGoalViaContext(ctx, c, codexTID, goal)
}
func pushGoalViaContext(ctx context.Context, c codexJSONCaller, codexTID, goal string) error {
	var (
		raw json.RawMessage
		err error
	)
	if strings.TrimSpace(goal) == "" {
		raw, err = c.CallCtx(ctx, "thread/goal/clear", map[string]interface{}{"threadId": codexTID})
	} else {
		raw, err = c.CallCtx(ctx, "thread/goal/set", map[string]interface{}{
			"threadId":  codexTID,
			"objective": goal,
		})
	}
	if err != nil {
		log.Printf("[codex] goal 下发失败 thread=%s: %v", codexTID, err)
		return err
	}
	log.Printf("[codex] goal 已下发 thread=%s goal=%.80q resp=%.160s", codexTID, goal, string(raw))
	return nil
}

// applyPendingGoal 在 Run() 拿到 codexTID 之后补发暂存的目标。
// 静默失败即可:目标本身仍在我们自己的库里,下一轮还有机会。
func (p *CodexProvider) applyPendingGoal(threadID, codexTID string) {
	p.goalMu.Lock()
	goal, ok := p.pendingGoals[threadID]
	if ok {
		delete(p.pendingGoals, threadID)
	}
	p.goalMu.Unlock()
	if !ok {
		return
	}
	if err := p.pushGoal(codexTID, goal); err == nil {
		log.Printf("[codex] 补发暂存目标 thread=%s", threadID)
	} else {
		p.stashGoal(threadID, goal)
	}
}

// goalStatusFromNotification 解析 thread/goal/updated 通知里的状态机字段。
// 返回 (goal, status, tokensUsed, timeUsedSeconds);拿不到时返回零值。
//
// 【字段名实锤】外层包裹键是 goal(ThreadGoalUpdatedNotification required:
// goal/threadId,这一层没写错),但 ThreadGoal 本体的文本字段是 **objective**
// (v2 schema 与 live 应答一致,同 thread/goal/get 的应答形状);旧代码解析
// 内层 "goal" 永远拿到空串,通知里目标文本被静默丢掉。
func goalStatusFromNotification(params json.RawMessage) (string, string, int64, int64) {
	var n struct {
		Goal struct {
			Objective       string `json:"objective"`
			Status          string `json:"status"`
			TokensUsed      int64  `json:"tokensUsed"`
			TimeUsedSeconds int64  `json:"timeUsedSeconds"`
		} `json:"goal"`
	}
	if json.Unmarshal(params, &n) != nil {
		return "", "", 0, 0
	}
	return n.Goal.Objective, n.Goal.Status, n.Goal.TokensUsed, n.Goal.TimeUsedSeconds
}
