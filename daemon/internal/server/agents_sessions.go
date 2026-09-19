package server

// 电脑上 pi / zcode 已有会话:列出 + 一键导入为可续聊的本地线程。
// 与 codexsessions.go 同一套体验,差异只在列表来源与历史落法:
//   - pi:纯文件扫描(不依赖进程),导入时历史直接从 .jsonl 落进本地线程;
//   - zcode:协议 session/list(一次性 app-server 子进程),导入只绑元信息 ——
//     ZCode 的消息抽取需要拉起进程再 resume(重、且无文件可读),而
//     ZcodeProvider.ensureSession 对 ExternalSession.SessionID 非空的线程会
//     session/resume 重挂载,首次发消息起 agent 侧上下文就是全的。
//     代价:导入后"手机端时间线"从导入时刻起算,电脑端既有消息不在本地历史里。

import (
	"context"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// zcodeSessionListLimit 是 zcode session/list 结果的 daemon 侧截断上限:电脑上
// 历史会话可能积累几百条,一次性 app-server 拉全量再整表塞给手机,导入页既慢
// 又难翻。协议应答实测已按 updatedAt 倒序(agent/zcode_sessions.go 顶部注记),
// 这里在超过上限时按时间倒序防御性重排一次再取前 N,"最近 N 条"的语义不依赖
// 协议排序细节;不足上限时原样透传,不改变现有行为。
const zcodeSessionListLimit = 100

// limitZcodeSessions 把 session/list 结果裁到最近 n 条。n<=0 或不足 n 条时原样
// 返回(不改协议顺序);需要截断时按 UpdatedAt 倒序取前 n 条(稳定排序,同一
// 时刻的条目保持协议原序,UpdatedAt 为零值的条目排最后)。不改动入参切片。
func limitZcodeSessions(infos []agent.ZcodeSessionInfo, n int) []agent.ZcodeSessionInfo {
	if n <= 0 || len(infos) <= n {
		return infos
	}
	sorted := make([]agent.ZcodeSessionInfo, len(infos))
	copy(sorted, infos)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
	})
	return sorted[:n]
}

// zcodeListRpcBudget 是 ListAgentThreads("zcode")的调用方预算:一次性
// app-server 冷启 + session/list。agent.ScanZcodeSessions 自身还有 15s 上界,
// 这层包装保证 RPC 侧先到期、先把错误还给手机(pcError 展示),不挂到 callTimeout。
const zcodeListRpcBudget = 10 * time.Second

// importAgentModels 导入线程的初始模型:各 agent 的"跟随桌面当前配置"哨兵值
// (provider 侧 applyModel 对 default 直接过,不做任何 set_model)。
var importAgentModels = map[string]string{
	"pi":    "pi/default",
	"zcode": "zcode/default",
}

func (s *Server) ListAgentThreads(ctx context.Context, req *pb.ListAgentThreadsRequest) (*pb.ListAgentThreadsResponse, error) {
	agentID := strings.ToLower(strings.TrimSpace(req.AgentId))
	if agentID != "" && importAgentModels[agentID] == "" {
		return nil, errInvalidArg("不支持的 agent:" + req.AgentId + "(支持 pi / zcode)")
	}
	out := &pb.ListAgentThreadsResponse{}
	// 空参 = pi + zcode 一起列;某一侧失败不挡另一侧(单独指定时才把错误透传,
	// 让手机端能区分"没装"与"真的空")。
	for _, id := range []string{"pi", "zcode"} {
		if agentID != "" && agentID != id {
			continue
		}
		infos, err := s.listAgentSessions(id)
		if err != nil {
			if agentID != "" {
				return nil, err
			}
			log.Printf("[sessions] 列出电脑上的 %s 会话失败(该侧留空): %v", id, err)
			continue
		}
		for _, info := range infos {
			if s.cfg.Store.SessionHidden(info.AgentId, info.SessionId) {
				continue
			}
			out.Threads = append(out.Threads, info)
		}
	}
	return out, nil
}

// listAgentSessions 按 agent 取电脑上已有会话,统一转成协议结构。
func (s *Server) listAgentSessions(agentID string) ([]*pb.AgentThreadInfo, error) {
	switch agentID {
	case "pi":
		// 扫描侧自带截断(0 = 默认上限,与 zcode 侧 zcodeSessionListLimit 对齐的 100)。
		scanned, err := agent.ScanPiSessions(0)
		if err != nil {
			return nil, errInternal(err)
		}
		out := make([]*pb.AgentThreadInfo, 0, len(scanned))
		for _, ss := range scanned {
			out = append(out, &pb.AgentThreadInfo{
				AgentId:         "pi",
				SessionId:       ss.SessionID,
				SessionFile:     ss.SessionFile,
				Cwd:             ss.Cwd,
				Title:           ss.Title,
				UpdatedAtUnixMs: ss.UpdatedAt.UnixMilli(),
			})
		}
		return out, nil
	case "zcode":
		// ZCode 没装(LocateZCode 失败)= 能力不可用,回 Unavailable 让手机端
		// 显示"去电脑上配置"而不是当故障重试;其余失败按 Internal 透传。
		infos, err := runBounded[[]agent.ZcodeSessionInfo](zcodeListRpcBudget, agent.ScanZcodeSessions)
		if err != nil {
			if ok, why := agent.ZcodeAvailable(); !ok {
				return nil, errUnavailable("电脑上没有可用的 ZCode:" + why)
			}
			return nil, errInternal(err)
		}
		// daemon 侧截断:只把最近的会话给导入页(见 zcodeSessionListLimit)。
		infos = limitZcodeSessions(infos, zcodeSessionListLimit)
		out := make([]*pb.AgentThreadInfo, 0, len(infos))
		for _, ss := range infos {
			out = append(out, &pb.AgentThreadInfo{
				AgentId:         "zcode",
				SessionId:       ss.SessionID,
				Cwd:             ss.Workspace,
				Title:           ss.Title,
				UpdatedAtUnixMs: ss.UpdatedAt.UnixMilli(),
			})
		}
		return out, nil
	}
	return nil, errInvalidArg("不支持的 agent:" + agentID)
}

func (s *Server) ImportAgentThread(ctx context.Context, req *pb.ImportAgentThreadRequest) (*pb.ImportAgentThreadResponse, error) {
	agentID := strings.ToLower(strings.TrimSpace(req.AgentId))
	if importAgentModels[agentID] == "" {
		return nil, errInvalidArg("不支持的 agent:" + req.AgentId + "(支持 pi / zcode)")
	}
	sessionID := strings.TrimSpace(req.SessionId)
	sessionFile := strings.TrimSpace(req.SessionFile)
	if err := s.cfg.Store.SetSessionHidden(agentID, sessionID, false); err != nil {
		return nil, errInternal(err)
	}
	if sessionID == "" && sessionFile == "" {
		return nil, errInvalidArg("session_id 与 session_file 至少要有一个,否则无法续聊重挂载")
	}
	// 已导入过:直接复用现有线程(不重复建)。判定键 = agent + 外部会话 id;
	// 没有 id(理论上不发生)时退回文件路径匹配。
	for _, t := range s.cfg.Store.List(true) {
		if threadAgentID(t) != agentID {
			continue
		}
		same := (sessionID != "" && t.ExternalSession.SessionID == sessionID) ||
			(sessionID == "" && sessionFile != "" &&
				strings.EqualFold(filepath.Clean(t.ExternalSession.SessionFile), filepath.Clean(sessionFile)))
		if same {
			return &pb.ImportAgentThreadResponse{ThreadId: t.ID}, nil
		}
	}
	// ---- 历史落库:目前只有 Pi 有零成本文件来源;ZCode 只导元信息 ----
	var msgs []store.StoredMessage
	if agentID == "pi" {
		file := sessionFile
		if file == "" {
			// 手机端只回传了 id:按 id 反查文件(扫描一次拿 SessionFile)。
			// 0 = 默认上限(最近 100 条):更老的会话按 id 反查不到文件,历史落库跳过。
			scanned, err := agent.ScanPiSessions(0)
			if err == nil {
				for _, ss := range scanned {
					if ss.SessionID == sessionID {
						file = ss.SessionFile
						break
					}
				}
			}
		}
		sessionFile = file
		if file != "" {
			history, err := agent.ReadPiSessionHistory(file)
			if err != nil {
				// 文件刚被删/被移:导入仍应成功(引用还在,续聊时会重挂载失败并明说),
				// 历史缺失只记日志,不让整次导入失败。
				log.Printf("[sessions] 读取 Pi 会话历史失败(导入继续,无历史): %v", err)
			}
			for _, m := range history {
				at := m.At.UnixMilli()
				if at <= 0 {
					at = time.Now().UnixMilli()
				}
				msgs = append(msgs, store.StoredMessage{
					Role:      m.Role,
					Blocks:    []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: m.Text}}}},
					CreatedAt: at,
				})
			}
		}
	}
	// ---- 标题 / 工作区兜底(与 ImportCodexThread 同款) ----
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = agentID + " 会话"
	}
	cwd := strings.TrimSpace(req.Cwd)
	if cwd == "" {
		cwd = s.cfg.Store.DefaultWorkspace()
		if cwd == "" {
			cwd = s.cfg.Workspace
		}
	}
	t := &store.Thread{
		ID:        randomID("th"),
		Title:     title,
		Workspace: cwd,
		ModelID:   importAgentModels[agentID],
		AgentID:   agentID,
		ExternalSession: store.ExternalSessionRef{
			SessionID:   sessionID,
			SessionFile: sessionFile,
			Cwd:         cwd,
		},
	}
	t.Messages = append(t.Messages, msgs...)
	if err := s.cfg.Store.Create(t); err != nil {
		return nil, errInternal(err)
	}
	return &pb.ImportAgentThreadResponse{ThreadId: t.ID}, nil
}
