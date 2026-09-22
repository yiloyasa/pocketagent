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

// AgentSessionImporter 桌面既有会话导入能力(server 层最小接口,与
// zcodeHistorySource 同款模式)。
//
// 【2026-09-20 动态化,体检 P1-4】以前这里写死 importAgentModels{pi,zcode} 与
// []string{"pi","zcode"} 两份名单:第 4 个桌面 agent 永远进不了导入页,报错
// 文案还会对着未来撒谎("支持 pi / zcode")。现在按能力判定:
//   - pi 的会话扫描是纯文件扫描、zcode 是一次性 app-server 子进程 —— 都长在
//     agent 包的**包级函数**上,不是 provider 方法,而 internal/agent 不归本层改,
//     所以由 server 侧适配器把"扫描源 + 导入初始模型"绑成统一能力接口;
//   - ListAgentThreads / ImportAgentThread 只问"有没有这个能力",报错文案动态
//     列举当前可导入的 agent。新增可导入 agent 二选一:① 注册一个内置适配器
//     (扫描机制长在包级函数上时);② provider 直接实现本接口(下次探测自动
//     入列,本文件零改动)。这与"写死准入名单"的区别:名单拦输入,能力表只
//     声明机制 —— 没有扫描实现的 agent 本来就无法导入,不需要第二道名单再拦。
type AgentSessionImporter interface {
	// ListDesktopSessions 列出电脑上该 agent 的既有会话(失败如实报错)。
	ListDesktopSessions(ctx context.Context) ([]*pb.AgentThreadInfo, error)
	// ImportModelID 导入线程的初始模型:各 agent 的"跟随桌面当前配置"哨兵值
	// (provider 侧 applyModel 对 default 直接过,不做任何 set_model)。
	ImportModelID() string
}

// SessionHistoryReader 导入时的可选扩展:能把外部会话历史零成本落进本地线程
// (目前只有 pi 有纯文件来源;zcode 走协议只绑元信息,见文件头注释)。
type SessionHistoryReader interface {
	// ReadImportHistory 解析外部会话历史;session_file 为空时按 session_id
	// 反查。返回 (反查后的文件路径, 消息);读不到历史不挡导入(调用方只记日志)。
	ReadImportHistory(sessionID, sessionFile string) (string, []store.StoredMessage)
}

// piSessionImporter / zcodeSessionImporter 把包级扫描源适配成导入能力,
// 行为与动态化之前逐字一致(pi 文件扫描落历史;zcode 协议扫描只绑元信息)。
type piSessionImporter struct{}
type zcodeSessionImporter struct{}

func (piSessionImporter) ImportModelID() string    { return "pi/default" }
func (zcodeSessionImporter) ImportModelID() string { return "zcode/default" }

// ReadImportHistory —— pi:历史直接从 .jsonl 会话文件落进本地线程。文件刚被删/
// 被移时:导入仍应成功(引用还在,续聊时会重挂载失败并明说),历史缺失只记日志。
func (piSessionImporter) ReadImportHistory(sessionID, sessionFile string) (string, []store.StoredMessage) {
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
	if file == "" {
		return sessionFile, nil
	}
	history, err := agent.ReadPiSessionHistory(file)
	if err != nil {
		log.Printf("[sessions] 读取 Pi 会话历史失败(导入继续,无历史): %v", err)
		return file, nil
	}
	msgs := make([]store.StoredMessage, 0, len(history))
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
	return file, msgs
}

// builtinSessionImporters 内置能力适配器:扫描机制长在 agent 包包级函数上的
// 那些桌面 agent 在此绑定。新增 provider 若自带 AgentSessionImporter 实现,
// 不经过这里也会被 importableAgents 探测到。
var builtinSessionImporters = map[string]AgentSessionImporter{
	"pi":    piSessionImporter{},
	"zcode": zcodeSessionImporter{},
}

// importableAgents 当前可导入的 agent 全集 = 内置适配器 + 任何自带导入能力的
// provider。map 无序,报错文案与空参遍历统一走 importableAgentIDs(字典序)。
func (s *Server) importableAgents() map[string]AgentSessionImporter {
	out := make(map[string]AgentSessionImporter, len(builtinSessionImporters))
	for id, imp := range builtinSessionImporters {
		out[id] = imp
	}
	for id, p := range s.cfg.Providers {
		if imp, ok := p.(AgentSessionImporter); ok {
			if _, exists := out[id]; !exists {
				out[id] = imp
			}
		}
	}
	return out
}

// importableAgentIDs 当前可导入的 agent ID(字典序;报错文案与空参遍历共用,
// 保证"谁能导"只有一个动态事实来源)。
func importableAgentIDs(m map[string]AgentSessionImporter) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (s *Server) ListAgentThreads(ctx context.Context, req *pb.ListAgentThreadsRequest) (*pb.ListAgentThreadsResponse, error) {
	agentID := strings.ToLower(strings.TrimSpace(req.AgentId))
	importers := s.importableAgents()
	if agentID != "" {
		if _, ok := importers[agentID]; !ok {
			return nil, errInvalidArg("不支持的 agent:" + req.AgentId +
				"(当前可导入: " + strings.Join(importableAgentIDs(importers), ", ") + ")")
		}
	}
	out := &pb.ListAgentThreadsResponse{}
	// 空参 = 全部具备导入能力的 agent 一起列;某一侧失败不挡另一侧(单独指定时
	// 才把错误透传,让手机端能区分"没装"与"真的空")。
	for _, id := range importableAgentIDs(importers) {
		if agentID != "" && agentID != id {
			continue
		}
		infos, err := importers[id].ListDesktopSessions(ctx)
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
			// 【删除墓碑】普通删除路径**不**写 HiddenSessions(那是"移除"专用),所以
			// 只查 Hidden 挡不住真删除:pi/zcode 的 jsonl 文件还在,扫描就会把它列回来。
			// 会话 id 与文件路径都可能被同步链当登记键用(删除侧两个都记),这里两个都查。
			if s.cfg.Store.IsTombstoned(info.SessionId) ||
				(info.SessionFile != "" && s.cfg.Store.IsTombstoned(info.SessionFile)) {
				continue
			}
			out.Threads = append(out.Threads, info)
		}
	}
	return out, nil
}

// ListDesktopSessions —— pi:纯文件扫描,不依赖进程(扫描侧自带截断,0 = 默认
// 上限,与 zcode 侧 zcodeSessionListLimit 对齐的 100)。
func (piSessionImporter) ListDesktopSessions(_ context.Context) ([]*pb.AgentThreadInfo, error) {
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
}

// ListDesktopSessions —— zcode:协议 session/list(一次性 app-server 子进程)。
// ZCode 没装(LocateZCode 失败)= 能力不可用,回 Unavailable 让手机端显示
// "去电脑上配置"而不是当故障重试;其余失败按 Internal 透传。
func (zcodeSessionImporter) ListDesktopSessions(_ context.Context) ([]*pb.AgentThreadInfo, error) {
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

func (s *Server) ImportAgentThread(ctx context.Context, req *pb.ImportAgentThreadRequest) (*pb.ImportAgentThreadResponse, error) {
	agentID := strings.ToLower(strings.TrimSpace(req.AgentId))
	importer, ok := s.importableAgents()[agentID]
	if !ok {
		return nil, errInvalidArg("不支持的 agent:" + req.AgentId +
			"(当前可导入: " + strings.Join(importableAgentIDs(s.importableAgents()), ", ") + ")")
	}
	sessionID := strings.TrimSpace(req.SessionId)
	sessionFile := strings.TrimSpace(req.SessionFile)
	// 【顺序钉:墓碑判定必须在 SetSessionHidden(false) 之前】否则会出现"返回了错误,
	// 隐藏却已被解开"的半完成状态 —— 用户看到失败,而会话已经回到列表。
	// 键与删除侧写入的保持一致(会话 id + 文件路径)。
	if s.cfg.Store.IsTombstoned(sessionID) ||
		(sessionFile != "" && s.cfg.Store.IsTombstoned(sessionFile)) {
		return nil, errFailedPrecondition("会话已被删除;如需恢复请先清除删除记录")
	}
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
	// ---- 历史落库:哪个 agent 有零成本文件来源,由能力接口说了答(目前只有
	// pi 的纯文件扫描;ZCode 只导元信息,续聊时重挂载,见文件头注释) ----
	var msgs []store.StoredMessage
	if hr, ok := importer.(SessionHistoryReader); ok {
		resolved, history := hr.ReadImportHistory(sessionID, sessionFile)
		sessionFile = resolved
		msgs = history
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
		ModelID:   importer.ImportModelID(),
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
