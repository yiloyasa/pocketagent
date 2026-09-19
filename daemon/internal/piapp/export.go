// export.go —— pi S 级增量命令的 Client 封装:export_html / compact /
// get_available_thinking_levels(2026-09-19 覆盖审计 §7-4 点名的 S 项)。
//
// 【为什么单独成文件】client.go 正被 P1/P2 修复占用;同包新文件承接新方法,
// 不碰既有代码。应答形状均经本机 0.85.1 实测,原始报文见
// docs/verify/evidence/pi-s-probe.md(bundle 源码与实测两面对照过)。
package piapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// 等待上界(与 agent 侧的 piCallBudget 同纪律:绝不能无限等)。
const (
	// exportBudget export_html 渲染整棵会话树 + 写文件,MB 级会话要数秒。
	exportBudget = 60 * time.Second
	// compactBudget compact = 真实跑一次 LLM 摘要请求(几十秒量级)。
	compactBudget = 180 * time.Second
	// thinkingLevelsBudget 档位查询是纯内存读,秒级。
	thinkingLevelsBudget = 10 * time.Second
)

// ExportHTML 把当前会话导出为离线 HTML 存档,写到 outputPath(建议绝对路径;
// Pi 端只在 outputPath 为空时才自己起默认文件名),返回 Pi 实际写入的路径。
//
// 【0.85.1 实测】请求 {"type":"export_html","outputPath":"..."} →
// 应答 data={"path":"..."},文件确实落在 outputPath(37 万字节样例)。
// 【前提】会话已落盘:--no-session 的内存会话 Pi 报
// "Cannot export in-memory session to HTML";一条对话都没有报
// "Nothing to export yet - start a conversation first"。调用方(provider)
// 已用 ExternalSession.SessionFile 挡住前一种。
func (c *Client) ExportHTML(ctx context.Context, outputPath string) (string, error) {
	raw, err := c.CallCtx(ctx, "export_html", map[string]any{"outputPath": outputPath}, exportBudget)
	if err != nil {
		return "", err
	}
	return parseExportHTMLResponse(raw)
}

// parseExportHTMLResponse 解析 export_html 应答 data(抽成纯函数便于单测)。
func parseExportHTMLResponse(raw json.RawMessage) (string, error) {
	var d struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(raw, &d) != nil || d.Path == "" {
		return "", fmt.Errorf("export_html 应答缺少 path: %.200s", raw)
	}
	return d.Path, nil
}

// CompactResult compact 的成功应答(0.85.1 session.compact 的 compactionResult;
// 成功形状来自 bundle 源码,错误路径已实测)。
type CompactResult struct {
	Summary              string          `json:"summary"`              // 摘要文本(替换被压缩的历史进上下文)
	FirstKeptEntryID     string          `json:"firstKeptEntryId"`     // 压缩分界:此条目起的近期原文保留
	TokensBefore         int64           `json:"tokensBefore"`         // 压缩前上下文 token
	EstimatedTokensAfter int64           `json:"estimatedTokensAfter"` // 压缩后预估 token
	Usage                json.RawMessage `json:"usage"`                // 摘要请求自身用量(形状随模型,原样保留)
	Details              json.RawMessage `json:"details"`              // 桌面端渲染明细(原样保留)
}

// Compact 手动压缩当前会话上下文,等价桌面端 /compact。等真实压缩完成后返回。
//
// 【实测语义(接前必读)】Pi 侧第一步就是 abort 当前正在跑的轮次 —— 调用方
// (agent.PiProvider.CompactThread)必须先挡住"有轮次在跑"的情况,绝不能让
// 压缩悄悄杀掉用户的任务。小会话会被拒:"Nothing to compact (session too
// small)"(实测);已压缩过再压:"Already compacted"。期间 Pi 会发
// compaction_start/compaction_end 事件(daemon 的 MapStatusEvent 已映射)。
func (c *Client) Compact(ctx context.Context, customInstructions string) (*CompactResult, error) {
	params := map[string]any{}
	if customInstructions != "" {
		params["customInstructions"] = customInstructions
	}
	raw, err := c.CallCtx(ctx, "compact", params, compactBudget)
	if err != nil {
		return nil, err
	}
	var out CompactResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("compact 应答无法解析: %w", err)
	}
	return &out, nil
}

// AvailableThinkingLevels 拉当前会话模型支持的思考档位。
//
// 【0.85.1 实测(glm-5.3)】应答 data={"levels":["low","high","max"]} ——
// Pi 按模型的 reasoning 能力 + thinkingLevelMap 过滤全集;无模型时回全集
// off/minimal/low/medium/high/xhigh/max。所以这个列表必须按会话实时取,
// 不能写死(daemon 侧只留同名全集做无子进程时的兜底)。
func (c *Client) AvailableThinkingLevels(ctx context.Context) ([]string, error) {
	raw, err := c.CallCtx(ctx, "get_available_thinking_levels", nil, thinkingLevelsBudget)
	if err != nil {
		return nil, err
	}
	var d struct {
		Levels []string `json:"levels"`
	}
	if json.Unmarshal(raw, &d) != nil || len(d.Levels) == 0 {
		return nil, errors.New("get_available_thinking_levels 应答缺少 levels")
	}
	return d.Levels, nil
}
