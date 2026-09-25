// prompt.go —— piapp.Client 的 prompt 命令封装(传输层,不含任何业务转写)。
//
// 【streamingBehavior(pi 0.87.1)】轮次流式期间,不带该字段的 prompt 会被 pi
// 直接拒收(success:false,docs/rpc-commands.md "During streaming");带
// "followUp" 则进入 pi 自己的 followUp 队列(当前轮结束后自动执行,回执经
// queue_update 事件)。字段只对**流式期间**生效:pi 空闲时忽略它并立即执行
// —— 调用方(意图是排队)必须自己先确认"有运行中轮次"再投,见
// docs/verify/evidence/pi-followup-wire-probe.md(bundle 逐行核对)。
package piapp

import (
	"context"
	"time"
)

// prompt 的 streamingBehavior 取值(原文字段,不翻译)。
const (
	// PromptSteer 运行中插话:当前助手轮的工具调用结束后、下一次 LLM 调用前注入。
	PromptSteer = "steer"
	// PromptFollowUp 当前轮结束后才投递(pi 侧排队,当前轮 settled 前自动续跑)。
	PromptFollowUp = "followUp"
)

// piPromptBudget prompt 受理的等待上界:受理(排队/开跑)是毫秒级动作,30s 对齐
// agent 包 piCallBudget 的量级(prompt.go 不依赖 agent 包常量,独立声明)。
const piPromptBudget = 30 * time.Second

// Prompt 发一条用户输入。behavior 为空串 = 不带 streamingBehavior 字段(首轮
// prompt 的既有形状,与 0.85.1 老版本零分叉);否则原样透传("steer"/"followUp")。
// 返回 nil = pi 已受理(受理后的失败走事件流,不以第二条 response 出现 ——
// rpc-commands.md 明说,与 CallCtx 的既有纪律一致)。
func (c *Client) Prompt(ctx context.Context, message string, images []any, behavior string) error {
	params := map[string]any{"message": message}
	if len(images) > 0 {
		params["images"] = images
	}
	if behavior != "" {
		params["streamingBehavior"] = behavior
	}
	_, err := c.CallCtx(ctx, "prompt", params, piPromptBudget)
	return err
}
