package server

// external_sessions.go —— 尚未接入 agent 的桌面既有会话**只读**接入(首批:Claude Code)。
//
// 【用户需求】"让尚未支持的 Agent 能在桌面端已有会话中显示消息" —— PA 还没有
// claude/opencode/hermes 的 provider,但这些 agent 在用户电脑上的历史会话应当
// 至少能只读看到。本文件不复刻一套 RPC:直接实现 agents_sessions.go 既有的
// AgentSessionImporter + SessionHistoryReader 能力接口,经 init() 注册进
// builtinSessionImporters —— ListAgentThreads / ImportAgentThread 两个既有 RPC
// 自动多出一个可导入 agent,proto 零改动、既有文件零改动(动态化设计原话:
// "新增可导入 agent 二选一:①注册一个内置适配器;②provider 自带实现")。
//
// 【只读语义与边界】导入 = 把电脑上已有的历史消息落进一条本地线程(与 pi 同款,
// 见 SessionHistoryReader);线程 AgentID="claude" 且 daemon 无此 provider,
// 所以续聊发送会被既有查找路径明确拒绝(ok=false → "不支持的 agent")——
// 这正是"只读展示"的样子:看得到历史,发不出新消息,不冒充已接入。
//
// 【为什么不收 OpenCode / Hermes】2026-09-30 本机实测(externalconv 包注释有
// 全量证据):OpenCode 只有空的 log//repos/,无 storage/ 会话落盘;Hermes 未
// 发现任何会话存储。没有可实证的文件格式就写解析器 = 猜 —— 红线不猜。哪家
// 机器上真有了会话文件,再按真实格式扩 externalconv 与本注册表。

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/pocketagent/daemon/internal/externalconv"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// claudeSessionImporter 把 externalconv 的 Claude Code 只读扫描适配成导入能力。
// root 为空 = 默认 <home>/.claude/projects;测试注入临时根。
type claudeSessionImporter struct{ root string }

func init() {
	if builtinSessionImporters == nil {
		builtinSessionImporters = map[string]AgentSessionImporter{}
	}
	if _, exists := builtinSessionImporters[string(externalconv.AgentClaude)]; !exists {
		builtinSessionImporters[string(externalconv.AgentClaude)] = claudeSessionImporter{}
	}
}

// resolveRoot 返回扫描根:构造时注入的临时根优先(测试用),否则默认
// <home>/.claude/projects;拿不到 home 目录时返回空(扫描按"无会话"处理)。
func (imp claudeSessionImporter) resolveRoot() string {
	if imp.root != "" {
		return imp.root
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return externalconv.ClaudeRoot(home)
}

// ImportModelID —— "跟随桌面当前配置"哨兵值。daemon 尚无 claude provider,
// 该值不会被 applyModel 消费;它只保证导入的线程有一个如实的模型占位。
func (claudeSessionImporter) ImportModelID() string {
	return string(externalconv.AgentClaude) + "/default"
}

// ListDesktopSessions 列出电脑上已有的 Claude Code 会话(只读扫描)。
// 会话存储不存在 = "无会话",返回空列表不报错(导入页语义,见 externalconv.ScanClaude)。
func (imp claudeSessionImporter) ListDesktopSessions(_ context.Context) ([]*pb.AgentThreadInfo, error) {
	metas, err := externalconv.ScanClaude(imp.resolveRoot())
	if err != nil {
		return nil, errInternal(err)
	}
	out := make([]*pb.AgentThreadInfo, 0, len(metas))
	for _, m := range metas {
		out = append(out, &pb.AgentThreadInfo{
			AgentId:         string(m.Agent),
			SessionId:       m.ID,
			SessionFile:     m.File,
			Title:           m.Title,
			UpdatedAtUnixMs: m.Modified.UnixMilli(),
		})
	}
	return out, nil
}

// ReadImportHistory 把会话文件里的历史消息零成本落进本地线程(与 pi 的同名
// 方法同款契约):session_file 为空时按 session_id 反查;历史读不到不挡导入。
func (imp claudeSessionImporter) ReadImportHistory(sessionID, sessionFile string) (string, []store.StoredMessage) {
	file := sessionFile
	if file == "" {
		metas, err := externalconv.ScanClaude(imp.resolveRoot())
		if err == nil {
			for _, m := range metas {
				if m.ID == sessionID {
					file = m.File
					break
				}
			}
		}
	}
	if file == "" {
		return sessionFile, nil
	}
	sess, err := externalconv.ReadClaudeSession(file)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			// 与 pi 同款取舍:历史读不到不挡导入,只记一行日志供排障。
			log.Printf("[sessions] 读取 Claude 会话历史失败(导入继续,无历史): %v", err)
		}
		return file, nil
	}
	msgs := storedTextMessages(sess.Messages, func(m externalconv.Message) (string, string, time.Time) {
		return m.Role, m.Text, m.At
	})
	return file, msgs
}
