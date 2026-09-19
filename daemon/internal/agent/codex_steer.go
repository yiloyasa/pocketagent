// codex_steer.go —— P1-4「运行中继续发消息」:codex 的 turn/steer 桥接。
//
// 【协议实测结论(2026-09-18,以 codex-schema/v2 为准)】
//   - turn/steer 是 **JSON-RPC request**(带 id、有应答),不是 notification:
//     schema 三件套 TurnSteerParams.json / TurnSteerResponse.json 都在,
//     应答形状是 {"turnId": "..."}。发了不算数,必须等应答确认插入成功。
//   - params 必填:threadId / input / expectedTurnId;可选:clientUserMessageId。
//     expectedTurnId 是前置条件:「Required active turn id precondition.
//     The request fails when it does not match the currently active turn.」——
//     所以我们必须知道当前运行中轮次在 codex 侧的真实 id(来自 turn/started
//     通知的 turn.id,见 codex.go 的登记逻辑),拿不准就别发,退回排队。
//   - input 与 turn/start 同形:UserInput 数组,文本项 {"type":"text","text":...}。
//
// 【失败语义】steer 失败(轮次还没开始 / codex 拒绝 / 通道错误)不是致命错误:
// server 层收到 steered=false 后会退回「内存排队、终局后自动续跑」路径,
// 用户的输入绝不会因为 steer 不可用而丢失。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// steerTarget 当前正在运行的 codex 轮次的「可插入」信息。
type steerTarget struct {
	codexThreadID string
	codexTurnID   string // turn/started 上报的轮次 id,作为 expectedTurnId
}

// steerBudget 单条 turn/steer 的等待上界。
//
// 用户在手机上等「已插入」的回执,不能让他等 callTimeout(120s);
// 15s 覆盖正常应答,超了就如实回失败转排队 —— 排队路径总能接住。
const steerBudget = 15 * time.Second

// Steer 实现 agent.Steerer:向 threadID 正在执行的轮次插入一条输入。
//
// 返回 (steered, codexTurnID, err):
//   - steered=true  → 插入成功,后续事件继续走现有 sink(不新开轮次);
//   - steered=false → 此刻插不进去(轮次未开始/已结束/上游拒绝),server 层应转排队。
//
// 【为什么直接用共享 client 而不走 withClient】Run() 只持有 turnMu,不持 lifeMu;
// 轮次进行期间 withClientCtx 有「turnActive 时不重启子进程」的守卫,p.client 稳定。
// codexapp.Client 的 CallCtx 自带锁(nextID/pending 走 mu,写走 writeMu),
// 从 reader goroutine 并发调用是安全的 —— 审批反向请求本来就跑在并发 goroutine 里。
func (p *CodexProvider) Steer(ctx context.Context, threadID string, blocks []pb.ContentBlock) (bool, string, error) {
	// 没有轮次在跑:steer 无从谈起(正常情况下 server 只会在自己轮次内调它,
	// 这里是防御 —— 例如 provider 全局的那一轮恰好不是本线程的轮次)。
	if !p.turnActive.Load() {
		return false, "", fmt.Errorf("当前没有正在运行的轮次")
	}
	p.steerMu.Lock()
	target, ok := p.activeSteer[threadID]
	p.steerMu.Unlock()
	if !ok || target.codexThreadID == "" || target.codexTurnID == "" {
		// turn/started 还没到(轮次刚发起):codex 侧尚无 active turn 可校验。
		return false, "", fmt.Errorf("轮次尚未在 codex 侧开始,暂不能插入")
	}
	if p.client == nil {
		return false, "", errNoClient
	}
	var text strings.Builder
	for i := range blocks {
		if tb, okb := blocks[i].Block.(*pb.ContentBlock_Text); okb {
			text.WriteString(tb.Text.Text)
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, steerBudget)
		defer cancel()
	}
	raw, err := p.client.CallCtx(ctx, "turn/steer", steerParams(target.codexThreadID, target.codexTurnID, text.String()))
	if err != nil {
		// codex 拒绝(expectedTurnId 不匹配等)或通道失败:交给排队兜底。
		return false, "", fmt.Errorf("turn/steer: %w", err)
	}
	var res struct {
		TurnId string `json:"turnId"`
	}
	_ = json.Unmarshal(raw, &res)
	return true, res.TurnId, nil
}

// steerParams 构造 turn/steer 的 JSON-RPC params(形状 = TurnSteerParams.json)。
//
// 抽成纯函数是为了单测能把「报文形状」钉死:codex 对必填字段是硬校验,
// 形状错一步,运行中插话就静默退化成排队 —— 用户只会觉得"怎么没插进去"。
func steerParams(codexThreadID, expectedTurnID, text string) map[string]interface{} {
	return map[string]interface{}{
		"threadId":       codexThreadID,
		"input":          turnInput(text),
		"expectedTurnId": expectedTurnID,
	}
}

// setSteerTarget 登记当前轮次目标(turn/started 到达时调用,见 codex.go)。
//
// 【为什么要求 turnActive】通知消费 goroutine 可能在 Run 返回后才收到迟到的
// turn/started;turnActive=false 说明这轮已经结束,登记一个过期目标只会让
// 下一次 steer 拿错 expectedTurnId(codex 会拒,白绕一圈)。
func (p *CodexProvider) setSteerTarget(ourThreadID, codexThreadID, codexTurnID string) {
	if ourThreadID == "" || codexThreadID == "" || codexTurnID == "" {
		return
	}
	if !p.turnActive.Load() {
		return
	}
	p.steerMu.Lock()
	p.activeSteer[ourThreadID] = steerTarget{codexThreadID: codexThreadID, codexTurnID: codexTurnID}
	p.steerMu.Unlock()
}

// clearSteerTarget 轮次收尾时清除登记(Run 的 defer 调用)。
func (p *CodexProvider) clearSteerTarget(ourThreadID string) {
	p.steerMu.Lock()
	delete(p.activeSteer, ourThreadID)
	p.steerMu.Unlock()
}

// turnStartedNotif turn/started 通知参数(只取用得到的字段;
// schema: {threadId, turn{id, items, status, ...}})。
type turnStartedNotif struct {
	ThreadId string `json:"threadId"`
	Turn     struct {
		Id string `json:"id"`
	} `json:"turn"`
}

// parseTurnStarted 解析 turn/started;解析失败返回 nil(调用方忽略)。
func parseTurnStarted(params json.RawMessage) *turnStartedNotif {
	var n turnStartedNotif
	if json.Unmarshal(params, &n) != nil || n.Turn.Id == "" {
		return nil
	}
	return &n
}
