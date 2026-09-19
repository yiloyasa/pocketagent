// codex_review.go —— "/计划"(发起计划审查)的 daemon 侧桥接。
//
// 【这条链路是什么】手机端斜杠面板的内置伪命令"/计划"→ StartReview RPC →
// 本文件把"审查当前会话拟定的计划"落到 codex 的 review/start;审查产出
// (enteredReviewMode / item/* / turn/completed)以事件形式流回**原会话**,
// 手机端现有的事件时间线直接可见,无需新增回传通道。
//
// 【形状:2026-09-19 对 codex-cli 0.149.1 live 实测】(证据:cmd/codexreadprobe)
//
//	review/start  {threadId, target:{type:"custom",instructions}}
//	           → **发起即回**(实测 0.0s):{turn:{id,status:"inProgress",…},
//	             reviewThreadId:"<原线程 id>"}   ← inline 模式回原线程
//	缺 threadId → -32600 "Invalid request: missing field `threadId`"
//	线程未加载 → -32600 "thread not found: <id>"(app-server 不自动 resume,
//	             所以必须先由 Run() 落地,未加载直接结构化报错,不空跑一轮)
//
// 【target 四选一】v2 schema ReviewTarget:uncommittedChanges / baseBranch /
// commit / custom。"/计划"的语义是"审查会话里的计划文本",对应 custom
// (自由指令,等价旧版 free-form prompt);focus 非空原样下发,空则用默认口径。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
	"github.com/pocketagent/daemon/internal/store"
)

// ErrReviewThreadNotLoaded 线程尚未被当前 app-server 子进程加载(没跑过第一轮,
// 或 daemon 重启后还没 resume)。server 层用 errors.Is 把它翻译成
// FailedPrecondition 的人话,与 CompactThread 的"先发一条消息"同口径。
var ErrReviewThreadNotLoaded = errors.New("codex 线程未加载")

// DefaultReviewFocus focus 为空时的默认审查口径。
// custom target 的 instructions 会作为自由指令交给 codex,这里只圈定"审计划"
// 的范围,不预设结论。
const DefaultReviewFocus = "请审查当前会话中拟定的计划:找出风险、遗漏的步骤、未验证的假设与不可行之处,并给出修改建议。"

// reviewStartBudget review/start 是"发起即回"的写动作,正常亚秒级;
// 30s 预算是防 codex 子进程卡死时长时间占住 lifeMu(同 statusBudget 的取舍)。
const reviewStartBudget = 30 * time.Second

// CodexReviewStart 是 review/start 应答的归一(只留桥接需要的三项)。
type CodexReviewStart struct {
	// ReviewThreadID 承载本次审查的 codex 线程 id(inline = 原线程 id;
	// detached 才会开新线程,我们固定 inline,不传 delivery)。
	ReviewThreadID string
	// TurnID 审查轮次的 codex 侧 id(诊断回指用,可能为空)。
	TurnID string
	// TurnStatus 应答里的轮次状态(实测 "inProgress")。
	TurnStatus string
}

// StartReview 用我们的会话 t 发起一次 codex 计划审查。
//
// focus 是审查重点(可空):空 = DefaultReviewFocus;非空原样作为自由指令下发。
// 线程未加载(ErrReviewThreadNotLoaded)与 codex 报错都按结构化错误返回,
// 绝不静默吞掉 —— 用户明确点了"/计划",没有反馈就是假开关。
func (p *CodexProvider) StartReview(t *store.Thread, focus string) (*CodexReviewStart, error) {
	if t == nil {
		return nil, fmt.Errorf("thread 为空")
	}
	// 必须比对 gen(codex_goal.go 同款纪律):threadMap 里的旧 id 属于上一个
	// 子进程,拿去调用会被 -32600 "thread not found" 拒绝。
	codexTID := p.loadedCodexThreadID(t.ID)
	if codexTID == "" {
		return nil, fmt.Errorf("%w: thread=%s", ErrReviewThreadNotLoaded, t.ID)
	}
	var out *CodexReviewStart
	err := p.withStatusClientBudget(reviewStartBudget, func(ctx context.Context, c *codexapp.Client) error {
		var err error
		out, err = codexReviewStartVia(ctx, c, codexTID, focus)
		return err
	})
	if err != nil {
		log.Printf("[codex] review/start 失败 thread=%s: %v", t.ID, err)
		return nil, err
	}
	log.Printf("[codex] review/start 已发起 thread=%s codexThread=%s turn=%s status=%s",
		t.ID, out.ReviewThreadID, out.TurnID, out.TurnStatus)
	return out, nil
}

// codexReviewStartVia 是 StartReview 的可测核心(同 codexXxxVia 模式:
// caller 可注入 fake,单测断言发出的方法名/参数与应答解析)。
func codexReviewStartVia(ctx context.Context, c codexJSONCaller, codexTID, focus string) (*CodexReviewStart, error) {
	if c == nil {
		return nil, fmt.Errorf("codex caller 为空")
	}
	if strings.TrimSpace(codexTID) == "" {
		return nil, ErrReviewThreadNotLoaded
	}
	instructions := strings.TrimSpace(focus)
	if instructions == "" {
		instructions = DefaultReviewFocus
	}
	// delivery 不传:schema 缺省 = inline,审查跑在原线程上,结果随事件流回。
	raw, err := c.CallCtx(ctx, "review/start", map[string]interface{}{
		"threadId": codexTID,
		"target": map[string]interface{}{
			"type":         "custom",
			"instructions": instructions,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("review/start: %w", err)
	}
	var res struct {
		ReviewThreadId string `json:"reviewThreadId"`
		Turn           struct {
			Id     string `json:"id"`
			Status string `json:"status"`
		} `json:"turn"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("review/start 应答无法解析: %w", err)
	}
	if res.ReviewThreadId == "" {
		// 认不出的应答形状:如实报错,不假装成功(诊断口子的职责是照镜子)。
		return nil, fmt.Errorf("review/start 应答缺 reviewThreadId: %s", clip(string(raw), 120))
	}
	return &CodexReviewStart{
		ReviewThreadID: res.ReviewThreadId,
		TurnID:         res.Turn.Id,
		TurnStatus:     res.Turn.Status,
	}, nil
}
