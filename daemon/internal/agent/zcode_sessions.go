package agent

// 电脑上 ZCode 已有会话的发现:走 ZCode Protocol 的 session/list。
//
// 【为什么走协议而不是读 ~/.zcode/cli/db/db.sqlite】
//  1. zcprobe 与本文件交付前的实机探针(2026-09-18,全新 app-server 进程直发
//     session/list)都实证:列表是 db 里的全量会话,不依赖"当前进程有没有建过会话";
//  2. 应答自带 title(ZCode 自动命名/首条输入)、updatedAt(unix ms)、
//     workspace.workspacePath —— 元数据一次拿全,不需要再开文件;
//  3. daemon 的 go.mod 没有 sqlite 驱动,按红线"不引入方案外的新依赖",
//     sqlite 回退方案不落库 —— 协议路径已经覆盖,报告里说明即可。
//
// 【为什么自起一次性子进程而不是复用 ZcodeProvider 的共享 client】
// ZcodeProvider.client 是私有字段(红线:不碰 zcode.go),而列表是低频只读操作,
// 独立进程互不干扰轮次生命周期,用完即关。成本:一次 node 冷启(本机 <3s),
// 由 server 层的预算包装兜底超时。
//
// 【应答形状(2026-09-18 实测摘录,截断)】
// {"sessions":[{"createdAt":1789720201116,"mode":"build","traceId":"…",
//   "sessionId":"sess_7b01b67f-…","sessionKind":"interactive","status":"idle",
//   "title":"Pocket Agent多子智能体架构对接与全局记忆","titleSource":"generated",
//   "updatedAt":1789760613705,
//   "workspace":{"workspaceKey":"G:\\项目1","workspacePath":"G:\\项目1"}}, …]}

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// zcodeListBudget 拉起一次性 app-server + session/list 的整段上界。
// 冷启本机 <3s,给到 15s 已是宽限;再往上不如让手机端看到明确的失败。
const zcodeListBudget = 15 * time.Second

type ZcodeSessionInfo struct {
	SessionID string // "sess_<uuid>"
	Title     string // ZCode 自动命名(titleSource=generated)或首条输入
	Workspace string // workspace.workspacePath(创建会话时的工作区)
	UpdatedAt time.Time
}

// ZcodeAvailable 报告电脑上 ZCode 是否可用;不可用时给出人话原因
// (server 层据此区分 errUnavailable"没装"与 Internal"真故障")。
func ZcodeAvailable() (bool, string) {
	if _, _, err := zcodeapp.LocateZCode(); err != nil {
		return false, err.Error()
	}
	return true, ""
}

// ScanZcodeSessions 拉电脑上全部 ZCode 会话(协议已按 updatedAt 倒序返回)。
// node / zcode.cjs 定位不到(ZCode 没装)→ 返回错误,由 server 层转成
// errUnavailable 让手机端展示,而不是伪装成"没有会话"。
func ScanZcodeSessions() ([]ZcodeSessionInfo, error) {
	if _, _, err := zcodeapp.LocateZCode(); err != nil {
		return nil, err
	}
	c, err := zcodeapp.Start(zcodeapp.StartOptions{Workspace: os.TempDir()})
	if err != nil {
		return nil, err
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), zcodeListBudget)
	defer cancel()
	raw, err := c.CallCtx(ctx, "session/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	return parseZcodeSessionList(raw), nil
}

// parseZcodeSessionList 解析 session/list 应答。解析失败的字段一律零值放过:
// 列表要的是"大致可见",单个字段缺失不该让整条会话消失。
func parseZcodeSessionList(raw []byte) []ZcodeSessionInfo {
	var lr struct {
		Sessions []struct {
			SessionID string `json:"sessionId"`
			Title     string `json:"title"`
			UpdatedAt int64  `json:"updatedAt"` // unix ms(实测必有)
			Workspace struct {
				WorkspacePath string `json:"workspacePath"`
			} `json:"workspace"`
		} `json:"sessions"`
	}
	if json.Unmarshal(raw, &lr) != nil {
		return nil
	}
	out := make([]ZcodeSessionInfo, 0, len(lr.Sessions))
	for _, ss := range lr.Sessions {
		if ss.SessionID == "" {
			continue // 没有 id 就无法 session/resume,列出来是死条目
		}
		zs := ZcodeSessionInfo{
			SessionID: ss.SessionID,
			Title:     ss.Title,
			Workspace: ss.Workspace.WorkspacePath,
		}
		if ss.UpdatedAt > 0 {
			zs.UpdatedAt = time.UnixMilli(ss.UpdatedAt)
		}
		out = append(out, zs)
	}
	return out
}
