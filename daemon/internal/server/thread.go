package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"google.golang.org/protobuf/proto"
)

// ---------- Thread CRUD ----------

// threadToPB 把存储里的会话转成协议结构。
//
// 【关于"审查者驳回"三个字段为什么每次现查】它们不是会话的固有属性,而是
// "最近一次自动审查有没有拦下某个动作"。这个状态只活在 codex 子进程和 daemon
// 内存里(见 agent/codex_guardian.go),落库没有意义 —— 落库反而会在 daemon
// 重启后留下一个点了就报错的僵尸按钮。所以这里每次从 provider 现取。
func (s *Server) threadToPB(t *store.Thread) *pb.Thread {
	out := &pb.Thread{
		LastTurnError: t.LastTurnError,
		Id:            t.ID, Title: t.Title, Workspace: t.Workspace, Goal: t.Goal,
		ModelId: t.ModelID, Status: t.Status, Effort: t.Effort,
		CreatedAtUnixMs: t.CreatedAt, UpdatedAtUnixMs: t.UpdatedAt,
		MessageCount: int32(len(t.Messages)),
		// 把桥接的 codex 线程 id 透出:手机端据此把"活跃"与"电脑"里的
		// 同一个会话合并为一条(否则会显示两份)。
		CodexThreadId: t.CodexThreadID,
		SectionId:     t.SectionID,
		// 子代理线程归属(T-S1P):非空 = 本会话是某线程的子代理线程,手机端
		// 据此过滤出子代理组点亮内联卡;空 = 普通顶层会话(旧行为)。
		ParentThreadId: t.ParentThreadID,
		// 多 agent 接入:会话归属与外部会话引用(手机端渲染 agent 徽标/筛选)。
		AgentId: threadAgentID(t),
		ExternalSession: func() *pb.ExternalSessionRef {
			if t.ExternalSession.SessionFile == "" && t.ExternalSession.SessionID == "" {
				return nil
			}
			return &pb.ExternalSessionRef{
				SessionId:   t.ExternalSession.SessionID,
				SessionFile: t.ExternalSession.SessionFile,
				Cwd:         t.ExternalSession.Cwd,
			}
		}(),
		// 归档只有一个事实来源(Status),这里只是把它翻译成布尔量,
		// 免得手机端要同时判断 status==ARCHIVED 和 archived 两个字段。
		Archived: t.Status == pb.ThreadStatus_THREAD_STATUS_ARCHIVED,
	}
	if approvals, ok := s.approvalSessions.Load(t.ID); ok {
		out.PendingApprovals = approvals.(*turnApprovals).snapshot()
	}
	if p := s.codexProvider(); p != nil && t.CodexThreadID != "" {
		if g := p.GuardianLatest(t.CodexThreadID); g != nil {
			out.GuardianDeniedReviewId = g.ReviewID
			out.GuardianDeniedItemId = g.TargetItemID
			out.GuardianDeniedReason = guardianReasonLine(g)
		}
	}
	return out
}

// guardianReasonLine 把一条审查结论压成手机端能直接显示的一句话。
func guardianReasonLine(g *agent.GuardianReview) string {
	if g == nil {
		return ""
	}
	parts := []string{}
	if g.ActionLabel != "" {
		parts = append(parts, g.ActionLabel)
	}
	if g.RiskLevel != "" {
		parts = append(parts, "风险:"+agent.RiskLevelLabel(g.RiskLevel))
	}
	if g.Rationale != "" {
		parts = append(parts, g.Rationale)
	}
	return strings.Join(parts, " · ")
}

func (s *Server) CreateThread(ctx context.Context, req *pb.CreateThreadRequest) (*pb.Thread, error) {
	ws := req.Workspace
	if ws == "" {
		ws = s.cfg.Store.DefaultWorkspace()
		if ws == "" {
			ws = s.cfg.Workspace
		}
	}
	// 多 agent:请求指定 agent(codex/pi/…)。不存在就显式报错 —— 静默建一个
	// 跑在别的 agent 上的会话,比报错误导得多(调研 §7.1 的隐患)。
	// 【尽力接入(2026-09-20 动态化,AUDIT 附录 A 第 1 项)】只要 agent_id 在
	// 注册表里就接,不问它"在不在任何名单";不在才报错,文案动态列举当前可用
	// 的 agent(新增 provider 自动出现,不写死任何名单)。
	agentID := strings.TrimSpace(req.AgentId)
	if agentID == "" {
		agentID = "codex" // 老客户端不带 agent_id:维持 codex 语义
	}
	if _, ok := s.cfg.Providers[agentID]; !ok {
		return nil, errFailedPrecondition("电脑上没有可用的 " + agentID +
			" agent(未安装或未配置)。当前可用: " + strings.Join(s.availableAgentIDs(), ", "))
	}
	model := req.ModelId
	if model == "" || !strings.HasPrefix(model, agentID+"/") {
		m, merr := s.defaultModelIDFor(agentID)
		if merr != nil {
			return nil, errFailedPrecondition(merr.Error())
		}
		model = m
	}
	title := req.Title
	if title == "" {
		title = "新会话 " + time.Now().Format("01-02 15:04")
	}
	t := &store.Thread{ID: randomID("th"), Title: title, Workspace: ws, ModelID: model, AgentID: agentID}
	// ---- 设置快照:"设置页更改只对新建对话生效"(2026-09-19)----
	// 把此刻生效的桌面偏好(与启动灌入 / SetCodexSettings 广播同源的
	// DesktopPrefsFromStore,即"经 ApplyPrefs 后的值")拷进线程落库。
	// 之后用户再改设置,本会话的后续轮次仍按这份快照装配参数 —— 老会话的
	// 行为不被设置页牵动;要吃到新设置就新建对话。
	// 注意:只快照"新建"这一刻;导入/同步进来的既有桌面会话不动
	// (它们本来就是"已有对话",走全局兜底语义)。
	t.SettingsSnapshot = turnSettingsFromPrefs(agent.DesktopPrefsFromStore(s.cfg.Store))
	if agentID != "codex" {
		d := s.cfg.Store.AgentDefaults(agentID)
		t.SettingsSnapshot = &d
	}
	if err := s.cfg.Store.Create(t); err != nil {
		return nil, errInternal(err)
	}
	return s.threadToPB(t), nil
}

// turnSettingsFromPrefs 从一份完整桌面偏好里提取"会路由进 turn 的七项",
// 作为线程级设置快照(DisablePasteBurst 不进 turn 参数,不入快照)。
func turnSettingsFromPrefs(p agent.DesktopPrefs) *store.TurnSettings {
	return &store.TurnSettings{
		ApprovalPolicy:   p.ApprovalPolicy,
		Sandbox:          p.Sandbox,
		Model:            p.Model,
		Effort:           p.Effort,
		Verbosity:        p.Verbosity,
		ReasoningSummary: p.ReasoningSummary,
		HideReasoning:    p.HideReasoning,
	}
}

// defaultModelIDFor 给指定 agent 的默认模型 id。
//
// 【不伪造 ID(2026-09-20 整改,AUDIT 附录 A 第 3 项)】以前列表拉不到就凭空
// 返回 "<agent>/default" —— 原生删掉/没提供 default 哨兵时,落库的是一个
// 不存在的 ID。现在按确定性优先级解析,解析不出如实报错(调用方给人话):
//  1. 设置页为该 agent 落盘的默认模型(AgentDefaults;codex 维持全局默认语义);
//  2. provider 模型列表里的 "default" 哨兵(跟随桌面当前配置,codex/pi/zcode 都有);
//  3. 模型列表首项;
//  4. 都没有 → 明确报错,绝不编造 ID。
func (s *Server) defaultModelIDFor(agentID string) (string, error) {
	if agentID != "codex" {
		if d := s.cfg.Store.AgentDefaults(agentID); d.Model != "" {
			return d.Model, nil
		}
	}
	p, ok := s.cfg.Providers[agentID]
	if !ok {
		return "", fmt.Errorf("电脑上没有可用的 %s agent,无法确定默认模型", agentID)
	}
	ms := p.Models()
	// 【vet】按下标取址遍历,避免 range 复制 pb 消息(内嵌 MessageState 含锁)。
	for i := range ms {
		if ms[i].Id == "default" {
			return p.ID() + "/default", nil
		}
	}
	if len(ms) > 0 {
		return p.ID() + "/" + ms[0].Id, nil
	}
	return "", fmt.Errorf("电脑上的 %s 还没有可用的模型,暂时无法确定默认模型", agentID)
}

// availableAgentIDs 当前注册表里可用的 agent ID(字典序)。仅供报错文案动态
// 列举 —— 任何"是否接入"的判断都直接查 s.cfg.Providers,不经过任何名单。
func (s *Server) availableAgentIDs() []string {
	ids := make([]string, 0, len(s.cfg.Providers))
	for id := range s.cfg.Providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ListThreads 会话列表。
//
// 【过滤/排序的实现取舍】codex 的 thread/list 支持 searchTerm / archived / limit,
// 但**不支持**分组过滤与排序键;而分组是 codex 侧的事实。所以:
//   - 用户一旦用了搜索/分组/归档中的任意一项,先把 codex 的列表同步进本地
//     (一次往返换来分组与归档状态的正确),再在本地过滤排序;
//   - 什么都没用时,只读本地库 —— 会话列表是最高频页面,不该每次都等 codex 冷启动。
func (s *Server) ListThreads(ctx context.Context, req *pb.ListThreadsRequest) (*pb.ListThreadsResponse, error) {
	// 搜索/分组/看归档:先与电脑对一次账
	if req.SearchTerm != "" || req.SectionId != "" || req.IncludeArchived || req.SortKey != "" {
		s.syncThreadsFromCodex(req.SearchTerm, req.IncludeArchived)
	}

	// 【T-SUBA-PI】pi 子代理运行的存根登记:pi 把每次子代理运行留在磁盘上(父会话
	// 文件同目录的 subagent-artifacts/),此前 daemon 对 pi 子代理零处理 → 手机端
	// 看不到子智能体。这里在列举前补登记(幂等、只读盘、失败只留痕),让子代理以
	// 子线程形态出现在父会话下,内联卡的呈现能力早已就绪。
	s.registerPiSubagentStubs()

	threads := s.cfg.Store.List(req.IncludeArchived)
	// 分组过滤
	if req.SectionId != "" {
		kept := threads[:0]
		for _, t := range threads {
			if t.SectionID == req.SectionId {
				kept = append(kept, t)
			}
		}
		threads = kept
	}
	// 搜索兜底:codex 的 searchTerm 覆盖不到还没同步进来的本地会话(比如刚从
	// rollout 导入、codex 侧还没索引到的),这里再在本地过一遍,宁可多留不可漏。
	if req.SearchTerm != "" {
		kept := threads[:0]
		for _, t := range threads {
			if searchTermMatches(t, req.SearchTerm) {
				kept = append(kept, t)
			}
		}
		threads = kept
	}
	// 排序。store.List 已经按 updated 倒序返回,所以只需在处理 asc / created 时重排。
	if req.SortKey == "created" {
		sortByCreated(threads, req.SortAscending)
	} else if req.SortAscending {
		// 默认键(updated)升序 = 把倒序结果翻过来
		for i, j := 0, len(threads)-1; i < j; i, j = i+1, j-1 {
			threads[i], threads[j] = threads[j], threads[i]
		}
	}
	if req.Limit > 0 && int(req.Limit) < len(threads) {
		threads = threads[:req.Limit]
	}
	out := &pb.ListThreadsResponse{}
	for _, t := range threads {
		out.Threads = append(out.Threads, s.threadToPB(t))
	}
	return out, nil
}

// sortByCreated 按创建时间排(稳定插入排序:会话量在几百条量级,够用且无额外内存)。
func sortByCreated(threads []*store.Thread, asc bool) {
	less := func(a, b *store.Thread) bool { return a.CreatedAt > b.CreatedAt }
	if asc {
		less = func(a, b *store.Thread) bool { return a.CreatedAt < b.CreatedAt }
	}
	for i := 1; i < len(threads); i++ {
		for j := i; j > 0 && less(threads[j], threads[j-1]); j-- {
			threads[j], threads[j-1] = threads[j-1], threads[j]
		}
	}
}

func (s *Server) GetThread(ctx context.Context, req *pb.GetThreadRequest) (*pb.Thread, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	t = s.recoverNativeResult(ctx, t)
	// 【症状②③】会话存在,但它所依托的原生事实来源(电脑上的记录/文件)已经找不到了:
	// 不能静默返回空内容假装"进得去",要如实报错说清是"内容读不出"。
	if reason := s.checkAndMarkStale(t); reason != "" {
		return nil, errNotFound(reason)
	}
	out := s.threadToPB(t)
	// codex 目标态回读:打开会话时以电脑端为准刷新(手机端有则显示"目标:xxx")。
	out.GoalDetail = s.goalDetailFor(ctx, t)
	return out, nil
}

// nativeSessionMissing 判断一个本地线程的"原生事实来源"是否已经找不到了:
// codex 的 rollout 文件被删、pi 的会话文件被移走。只在"原生 id 已绑定且本地还没有
// 任何消息"时查 —— 这样既精准命中"刚删/文件没了"的悬空会话,又避免对每一个健康
// 会话都做一次昂贵磁盘扫描(本地已有消息的会话直接展示本地副本,内容不算丢失)。
// 返回 (是否悬空, 人话原因);原因不含"未知/错误"等词,直接说清是文件找不到。
func (s *Server) nativeSessionMissing(t *store.Thread) (bool, string) {
	if t == nil {
		return false, ""
	}
	if t.CodexThreadID != "" && len(t.Messages) == 0 {
		if _, err := agent.FindCodexSession(t.CodexThreadID); err != nil {
			if errors.Is(err, agent.ErrCodexSessionNotFound) {
				return true, "这个会话在电脑上的记录已经找不到了（可能已被删除或移动），无法打开历史；可以删除它，或重新导入对应的电脑会话"
			}
			// 其它读取故障按"还在"处理,交给后续轮次/导入去如实报错,不在这里猜。
		}
	}
	if t.ExternalSession.SessionFile != "" && len(t.Messages) == 0 && threadAgentID(t) == "pi" {
		if _, statErr := os.Stat(t.ExternalSession.SessionFile); statErr != nil && os.IsNotExist(statErr) {
			return true, "这个 Pi 会话在电脑上的文件已经找不到了（可能已被删除或移动），无法打开；可以删除它，或重新导入对应的电脑会话"
		}
	}
	return false, ""
}

// checkAndMarkStale 若会话原生事实来源已找不到,返回人话原因(调用方据以如实报错);
// 同时把原因落到会话上,让会话列表也能如实反映(症状③:失效会话不再伪装成正常的)。
// 健康会话返回空串。落库只在"确实悬空"时发生且值幂等,不会污染健康会话。
func (s *Server) checkAndMarkStale(t *store.Thread) string {
	missing, reason := s.nativeSessionMissing(t)
	if !missing {
		return ""
	}
	if cur, ok := s.cfg.Store.Get(t.ID); ok {
		cur.LastTurnError = reason
		if err := s.cfg.Store.Update(cur); err != nil {
			logStoreUpdateErr("stale-session-mark", t.ID, err)
		}
	}
	return reason
}

func (s *Server) DeleteThread(ctx context.Context, req *pb.DeleteThreadRequest) (*pb.Empty, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return &pb.Empty{}, nil
	}
	// 【为什么只信租约】以前这里还看 t.Status == RUNNING,而 Status 是**落盘
	// 状态**:轮次异常收尾(原生子进程退出码 1、手机断线、daemon 被杀)时来不及
	// 复位,就会留下一个"僵尸 RUNNING"。它让这条会话**永远删不掉**,直到下次
	// daemon 重启(启动期的 ResolveLegacyRunning 才会复位)——用户实测"任何会话
	// 都删不掉"就有它一份。真正能证明"正在跑"的只有内存里的在飞租约
	// activeTurns:它随进程消失,不可能变成僵尸。
	if s.activeTurns.Has(t.ID) {
		return nil, errFailedPrecondition("会话正在运行，请先停止任务再删除或移除")
	}
	if t.Status == pb.ThreadStatus_THREAD_STATUS_RUNNING {
		// 无租约却标 RUNNING:确认是僵尸,复位后继续删除(不拦用户)。
		t.Status = pb.ThreadStatus_THREAD_STATUS_IDLE
		log.Printf("[thread] 删除会话 %s:发现僵尸 RUNNING(无在飞租约),已复位为 IDLE", t.ID)
	}
	id := threadAgentID(t)
	// 【原生侧删除:尽力而为,失败不阻塞本地删除】
	// 以前任何一步原生删除失败都直接 return error,于是"电脑上的文件动不了"
	// 被放大成"用户永远删不掉这个会话"。用户点删除的本意是"让它从手机上消失",
	// 而电脑侧残留可以在"未导入的电脑会话"列表里重新露面、再由用户处置 ——
	// 比"点了没反应"诚实且可用。因此这里一律:失败只记日志(含原因与残留提示),
	// 继续往下删本地。唯一会真正阻塞的条件是"会话正在运行"(见上方租约判定)。
	if !req.LocalOnly {
		switch id {
		case "codex":
			if t.CodexThreadID != "" {
				if err := agent.DeleteCodexSession(t.CodexThreadID); err != nil {
					log.Printf("[thread] 删除会话 %s:电脑上的 codex 会话未清掉(codex=%s),仅删本地: %v",
						t.ID, t.CodexThreadID, err)
				}
			}
		case "zcode":
			if t.ExternalSession.SessionID != "" {
				if p, ok := s.cfg.Providers[id].(zcodeSessionCloser); ok {
					if err := p.CloseSession(t.ExternalSession.SessionID); err != nil {
						log.Printf("[thread] 删除会话 %s:电脑上的 zcode 会话未关闭(session=%s),仅删本地: %v",
							t.ID, t.ExternalSession.SessionID, err)
					}
				} else {
					// ZCode 当前不可用(未装/子进程起不来):同样不该拦用户删本地。
					log.Printf("[thread] 删除会话 %s:ZCode 当前不可用,跳过原生关闭,仅删本地", t.ID)
				}
			}
		case "pi":
			if p, ok := s.cfg.Providers[id].(piThreadDropper); ok {
				p.DropThread(t.ID)
			}
			if err := agent.DeletePiSession(t.ExternalSession.SessionFile, t.ExternalSession.SessionID); err != nil {
				log.Printf("[thread] 删除会话 %s:电脑上的 pi 会话未清掉(session=%s),仅删本地: %v",
					t.ID, t.ExternalSession.SessionID, err)
			}
		}
	} else {
		sid := t.ExternalSession.SessionID
		if id == "codex" {
			sid = t.CodexThreadID
		}
		if err := s.cfg.Store.SetSessionHidden(id, sid, true); err != nil {
			return nil, errInternal(err)
		}
	}
	if id == "pi" && req.LocalOnly {
		if p, ok := s.cfg.Providers[id].(piThreadDropper); ok {
			p.DropThread(t.ID)
		}
	}
	// 【墓碑】记住"这个会话被用户删过",让原生会话同步链不再把它重新建回来:
	// registerPiSubagentStubs 每次列举会话都扫磁盘重登记 pi 子代理存根,
	// maybeRegisterCodexSubagentStub 在 codex 子线程 thread/started 时登记存根,
	// 它们按"磁盘/原生会话里有没有"判定,不知道用户刚删过 —— 不记墓碑就会
	// "删了又冒出来"。键覆盖各同步链的登记键(本地 id / codex 原生 id / 外部
	// 会话 id·文件),连带子代理存根也在 deleteChildStubs 内一并记。
	// 【顺序】墓碑必须在 Store.Delete 之前落盘。若先删本地、再记墓碑失败,就留下
	// "既删不掉又防不住"的中间态:本地没了,原生侧没有任何保护,下一次刷新即复活。
	// 【为什么失败必须拦住】原实现把墓碑失败当"留痕不阻塞",于是用户收到"删除成功",
	// 会话却复活,现场只剩 daemon 日志里一行 —— 无从解释。宁可当场报错让用户重试。
	// 【为什么只有真删除才记】local_only=true 是"移除"(可恢复的隐藏),它走
	// HiddenSessions;若也写永久墓碑,导入入口的守卫会让用户点"重新导入"被自己刚
	// 才的"移除"永久挡住。两条语义在写入侧就必须分开。
	if !req.LocalOnly {
		if err := s.tombstoneThread(t); err != nil {
			return nil, errInternal(err)
		}
	}

	// 【连带清理子代理存根】删主会话时,它派出的子代理存根(pi-sub-* / codex 存根)
	// 会变成**孤儿** —— 列表里已无父会话可归属,条目却还赖着。用户视角就是
	// "删了会话,子代理还在"。这里一并删掉。
	// 【凭什么认"子代理"】只认 ParentThreadID 这一个**结构性事实**(指向本会话),
	// 不按 id 前缀猜 —— 否则将来换命名就会漏或误伤用户自己建的会话。
	s.deleteChildStubs(t.ID)

	if err := s.cfg.Store.Delete(t.ID); err != nil {
		return nil, errInternal(err)
	}
	return &pb.Empty{}, nil
}

// tombstoneThread 把"这个会话被用户删过"记入墓碑。键覆盖各同步链的登记键:
// 本地线程 id(子代理存根用它当 id)、codex 原生线程 id(= 存根的 CodexThreadID)、
// pi/zcode 外部会话 id 与文件。同步链(maybeRegisterCodexSubagentStub /
// registerPiSubagentStubs)遇到墓碑就跳过,不再把刚删的会话建回来。
// 【只有真删除才记】local_only=true 是"移除"(可恢复的隐藏),走 HiddenSessions,
// 不该进永久墓碑(见调用点)。【返回 error 时调用方必须中止删除】墓碑没落盘就等于
// 同步链没有任何保护,报错优于无痕复活。
func (s *Server) tombstoneThread(t *store.Thread) error {
	if t == nil {
		return nil
	}
	if err := s.cfg.Store.AddTombstones(
		t.ID, t.CodexThreadID, t.ExternalSession.SessionID, t.ExternalSession.SessionFile,
	); err != nil {
		log.Printf("[tombstone] 记录删除墓碑失败,已中止本次删除 id=%s: %v", t.ID, err)
		return err
	}
	return nil
}

// deleteChildStubs 删除所有以 parentID 为父的子代理存根(幂等、失败只留痕)。
// 【为什么"失败不阻塞"】主会话的删除是用户的本意;某个存根删不掉(存储异常)
// 不该把整次删除回滚 —— 留日志即可,下次再删还能命中。
func (s *Server) deleteChildStubs(parentID string) {
	if parentID == "" {
		return
	}
	// 先批量记墓碑(一次落盘),再逐个删除:子代理存根被用户顺手删了,同步链
	// 也不要再建回来 —— 否则会出现"删了子代理、列表一刷新又冒出来"。
	var tombKeys []string
	var toDelete []string
	for _, child := range s.cfg.Store.List(true) {
		if child == nil || child.ID == parentID || child.ParentThreadID != parentID {
			continue
		}
		tombKeys = append(tombKeys, child.ID, child.CodexThreadID)
		toDelete = append(toDelete, child.ID)
	}
	if len(tombKeys) > 0 {
		if err := s.cfg.Store.AddTombstones(tombKeys...); err != nil {
			log.Printf("[thread] 记录子代理存根墓碑失败(留痕不阻塞): %v", err)
		}
	}
	for _, id := range toDelete {
		if err := s.cfg.Store.Delete(id); err != nil {
			log.Printf("[thread] 连带清理子代理存根 %s(parent=%s)失败(留痕不阻塞): %v",
				id, parentID, err)
			continue
		}
		log.Printf("[thread] 已连带清理子代理存根 %s(parent=%s)", id, parentID)
	}
}

// ArchiveThread 归档 / 取消归档。
//
// 【为什么要真的告诉 codex】归档以前只是"我们库里改个状态",于是电脑上的
// codex 会话列表里它还在,手机上却不见了 —— 用户换个设备看就是两套事实。
// codex 侧失败不影响本地结果(它可能是个还没在电脑上落地的新会话),
// 但会记日志,不让"以为归档了其实没有"悄悄发生。
func (s *Server) ArchiveThread(ctx context.Context, req *pb.ArchiveThreadRequest) (*pb.Thread, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	if req.Archived {
		t.Status = pb.ThreadStatus_THREAD_STATUS_ARCHIVED
	} else {
		t.Status = pb.ThreadStatus_THREAD_STATUS_IDLE
	}
	if err := s.cfg.Store.Update(t); err != nil {
		return nil, errInternal(err)
	}
	if p := s.codexProvider(); p != nil && t.CodexThreadID != "" {
		if err := p.SetCodexArchived(t.CodexThreadID, req.Archived); err != nil {
			log.Printf("[archive] 同步到 codex 失败(本地已归档)thread=%s codex=%s: %v",
				t.ID, t.CodexThreadID, err)
		}
	}
	return s.threadToPB(t), nil
}

func (s *Server) SetThreadGoal(ctx context.Context, req *pb.SetThreadGoalRequest) (*pb.Thread, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	if threadAgentID(t) != "codex" && threadAgentID(t) != "zcode" {
		return nil, errFailedPrecondition("该 Agent 未提供原生目标模式")
	}
	t.Goal = req.Goal
	if err := s.cfg.Store.Update(t); err != nil {
		return nil, errInternal(err)
	}
	// 【关键】光落库是个假开关:codex 有 thread/goal/set|clear 与完整状态机,
	// 不下发的话它根本不知道用户设了目标,表现就是"我明明设了目标,它没当回事"。
	// 下发失败不影响落库结果(线程还没被 app-server 加载时是正常情况,
	// provider 会暂存并在第一轮 Run 拿到 codex threadId 后补发)。
	if p, ok := s.cfg.Providers["codex"].(*agent.CodexProvider); ok && threadAgentID(t) == "codex" {
		if err := p.SetGoal(t.ID, req.Goal); err != nil {
			log.Printf("[goal] 下发失败(已落库,稍后补发)thread=%s: %v", t.ID, err)
		}
	}
	// zcode 线程按归属 agent 路由到 session/goal(2026-09-19 控制面探针实证:
	// {sessionId, action:"set"|"clear", objective},与 codex 的双方法同语义;
	// provider 侧同样暂存、Run 终局补发)。抽接口与 zcodeHistorySource 同款,
	// 单测可注入 fake。
	if threadAgentID(t) == "zcode" {
		if p, ok := s.cfg.Providers["zcode"].(zcodeGoalPusher); ok {
			if err := p.SetGoal(t.ID, req.Goal); err != nil {
				log.Printf("[goal] zcode 下发失败(已落库,稍后补发)thread=%s: %v", t.ID, err)
			}
		}
	}
	return s.threadToPB(t), nil
}

// piThreadCompactor pi 侧真实压缩能力(server 层类型断言探测,同
// zcodeHistorySource 的模式:抽最小接口,单测可注 fake,不依赖真子进程)。
// 生产由 *agent.PiProvider 实现(CompactThread,2026-09-19 审计 §7-4 S 项)。
type piThreadCompactor interface {
	CompactThread(t *store.Thread) error
}

// piThreadDropper pi 侧删除回收能力(P2-1:DropThread = Close 子进程 + 清
// clients/cursors,DeleteThread 时同步调用;抽最小接口同上,单测可注 fake)。
type piThreadDropper interface {
	DropThread(threadID string)
}

// titlePusher 重命名下发能力的最小接口(生产由 *agent.PiProvider 实现 ——
// set_session_name;codex 走专用通道 SetCodexThreadName,不经本接口)。
// RenameThread 用它探测"新标题能不能传到子进程侧":不能的(zcode)本地钉住
// TitlePinned,防止自动命名回调把用户改的名字顶回去(T-05)。
type titlePusher interface {
	Rename(t *store.Thread, title string) error
}

// zcodeGoalPusher / zcodeCompacter / zcodeSessionCloser zcode 侧控制面能力的
// 最小接口(生产由 *agent.ZcodeProvider 实现;抽接口同上,单测可注 fake):
// SetGoal = session/goal 下发,CompactSession = session/compact,CloseSession =
// session/close(真删除语义,DeleteThread 时同步)。
type (
	zcodeGoalPusher interface {
		SetGoal(threadID, goal string) error
	}
	zcodeCompacter interface {
		CompactSession(sessionID string) error
	}
	zcodeSessionCloser interface {
		CloseSession(sessionID string) error
	}
)

func (s *Server) CompactThread(ctx context.Context, req *pb.CompactThreadRequest) (*pb.Empty, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	// pi 路径:落 pi 的 compact 命令(同步等真实压缩完成;运行中/未落地由
	// provider 侧闸门拒绝并给结构化错误)。会话归属以 AgentID 为准(显式失败,
	// 不静默换人)—— 与 providerForThread 同纪律。
	if threadAgentID(t) == "pi" {
		pc, ok := s.cfg.Providers["pi"].(piThreadCompactor)
		if !ok {
			return nil, errUnavailable("电脑上没有可用的 pi")
		}
		if err := pc.CompactThread(t); err != nil {
			return nil, errInternal(err)
		}
		return &pb.Empty{}, nil
	}
	// zcode 路径:压缩落到桌面端 session/compact(2026-09-19 控制面探针实证
	// {sessionId} → {response, snapshot};轮次进行中 zcode 以 -32010 拒绝,
	// 错误原样转 gRPC 错误码)。
	if threadAgentID(t) == "zcode" {
		if t.ExternalSession.SessionID == "" {
			return nil, errFailedPrecondition("这个会话还没有在电脑上落地(先发一条消息),暂时无法压缩")
		}
		zc, ok := s.cfg.Providers["zcode"].(zcodeCompacter)
		if !ok {
			return nil, errUnavailable("电脑上没有可用的 zcode")
		}
		if err := zc.CompactSession(t.ExternalSession.SessionID); err != nil {
			return nil, errInternal(err)
		}
		return &pb.Empty{}, nil
	}
	// codex 路径(原有):真实压缩落到 codex 的 thread/compact/start,由桌面端
	// agent 真正执行。手动压缩入口(手机端可后续接),与自动压缩共用同一条 codex 通道。
	p := s.codexProvider()
	if p == nil {
		return nil, errUnavailable("电脑上没有可用的 codex")
	}
	if t.CodexThreadID == "" {
		return nil, errFailedPrecondition("这个会话还没有在电脑上落地(先发一条消息),暂时无法压缩")
	}
	if err := p.CompactThread(t.CodexThreadID); err != nil {
		return nil, errInternal(err)
	}
	return &pb.Empty{}, nil
}

func (s *Server) CancelTurn(ctx context.Context, req *pb.CancelTurnRequest) (*pb.Empty, error) {
	if _, ok := s.cfg.Store.Get(req.ThreadId); !ok {
		return nil, errNotFound("会话不存在")
	}
	// 手机端"终止任务":取到该线程在跑的轮次取消函数并触发。
	// provider.Run 会因 ctx 取消而返回,轮次按 FAILED 收尾(带已产出的部分内容),
	// 手机端收到 Failed 帧后结束转圈;流已断开时也没关系 —— 取消的是电脑端执行本身。
	if cancel := s.activeTurns.Cancel(req.ThreadId); cancel != nil {
		cancel()
		log.Printf("[turn] 手机端主动取消 %s", req.ThreadId)
	}
	return &pb.Empty{}, nil
}

// RenameThread 重命名会话(双端同步:落库后桌面端/手机端读到同一标题)。
//
// 这里额外把标题写回 codex:否则电脑上还是旧标题,用户在电脑上找会话时会以为
// 自己在手机上改的名字没生效。
//
// 【无下发通道的 agent:改名要钉住(T-05)】codex 走 SetCodexThreadName、pi 走
// titlePusher(set_session_name),改名能传到子进程侧,双端同源;zcode 没有
// 这样的通道,改名只能落在本地库 —— 若不钉住,它下一轮的自动命名回调
// (session/titleUpdated → applyZcodeTitle)会把用户刚改的名字顶掉,表现为
// "改名成功,聊了一句又变回去了"。探测方式用能力接口而不是写死 agent 名单:
// 有下发能力的 agent 不钉(电脑端发起的改名照常同步回来),没有的钉,
// 未来新 agent 接入自动获得合理默认(能透传就透传,不能就本地钉住)。
func (s *Server) RenameThread(ctx context.Context, req *pb.RenameThreadRequest) (*pb.Thread, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	t.Title = req.Title
	// codex 的下发通道在下方(SetCodexThreadName);其余 agent 有 titlePusher
	// 能力才算"能传到子进程侧"。
	canPropagate := threadAgentID(t) == "codex"
	if !canPropagate {
		if _, ok := s.cfg.Providers[threadAgentID(t)].(titlePusher); ok {
			canPropagate = true
		}
	}
	if !canPropagate {
		t.TitlePinned = true
	}
	if err := s.cfg.Store.Update(t); err != nil {
		return nil, errInternal(err)
	}
	if p := s.codexProvider(); p != nil && t.CodexThreadID != "" {
		if err := p.SetCodexThreadName(t.CodexThreadID, req.Title); err != nil {
			log.Printf("[rename] 同步标题到 codex 失败(本地已改名)thread=%s: %v", t.ID, err)
		}
	}
	// pi 会话改名同步:set_session_name 下发到 pi 子进程(失败仅日志,
	// 本地已改名是既成事实 —— 与 codex 同纪律)。
	if threadAgentID(t) == "pi" {
		if p, ok := s.cfg.Providers["pi"].(*agent.PiProvider); ok {
			if err := p.Rename(t, req.Title); err != nil {
				log.Printf("[rename] 同步标题到 pi 失败(本地已改名)thread=%s: %v", t.ID, err)
			}
		}
	}
	return s.threadToPB(t), nil
}

// GetThreadHistory 返回历史消息。数据来自我们自己的存储(两类 provider 统一):
// codex 桥接时 server 层同样会把用户输入/最终回复落库。
//
// 【zcode 导入线程的惰性历史拉取】从桌面端导入的 ZCode 会话只绑
// ExternalSession.SessionID,导入前的历史不在本地库 —— 首次打开这类线程且
// 本地一条消息都没有时,调 ZcodeProvider.HistoryFor(resume 返回完整
// messages[])把历史落库缓存,之后(以及打过一轮之后)一律走本地,不再在线。
// 拉取失败降级返回已有内容+日志,绝不把历史页打挂;codex/pi 路径零改动。
func (s *Server) GetThreadHistory(ctx context.Context, req *pb.GetThreadHistoryRequest) (*pb.GetThreadHistoryResponse, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	t = s.recoverNativeResult(ctx, t)
	// 【症状②③】原生事实来源已找不到(文件被删/被移)时,不静默返回空历史假装成功,
	// 如实报错说明"内容读不出";同时把原因落到会话上,让列表也能如实反映(症状③)。
	if reason := s.checkAndMarkStale(t); reason != "" {
		return nil, errNotFound(reason)
	}
	if threadAgentID(t) == "zcode" && t.ExternalSession.SessionID != "" {
		s.maybePullZcodeImportedHistory(ctx, t)
		// Get returns an owned snapshot, not a live store pointer.
		if current, found := s.cfg.Store.Get(t.ID); found {
			t = current
		}
	}
	// 【T-SUBVIS 接线点 1】codex 子代理存根(父链非空、本地无消息)首次打开子
	// 对话页时懒拉 thread/read 全量,与上面 zcode 导入线程同款"拉到即缓存"
	// (细节见 codex_subagent_history.go 文件头)。返回 true 表示确实回填过,
	// 重新取一次快照把落库的消息带上;false 时 t 原样——非子代理存根在此处
	// 零开销返回,既有路径不受影响。
	if s.maybePullCodexSubagentHistory(ctx, t) {
		if current, found := s.cfg.Store.Get(t.ID); found {
			t = current
		}
	}
	msgs := t.Messages
	if req.Limit > 0 && int(req.Limit) < len(msgs) {
		msgs = msgs[len(msgs)-int(req.Limit):]
	}
	// 按轮截断在 limit 之后叠加:limit 先按条数尾截(既有语义、优先级不变),
	// rounds 再在 limit 的结果上收窄 —— 两条约束同时满足才返回。
	msgs = tailByRounds(msgs, int(req.Rounds))
	out := &pb.GetThreadHistoryResponse{}
	for _, m := range msgs {
		blocks := make([]*pb.ContentBlock, 0, len(m.Blocks))
		for i := range m.Blocks {
			blocks = append(blocks, &m.Blocks[i])
		}
		out.Messages = append(out.Messages, &pb.ThreadMessage{
			Role: m.Role, Blocks: blocks, CreatedAtUnixMs: m.CreatedAt,
		})
	}
	// codex 目标态回读:与 GetThread 同源(打开会话即刷新,重连后以电脑端为准)。
	// 非 codex 会话 / 读不到时为 nil,手机端不占位。
	out.GoalDetail = s.goalDetailFor(ctx, t)
	return out, nil
}

// ---------- codex 目标态回读(thread/goal/get 的 RPC 出口)----------

// goalDetailFor 打开会话时现读电脑端 codex 线程的常驻目标(只读)。
//
// 【为什么不在 threadToPB 里做】threadToPB 是列表/创建/归档等所有路径共用的
// 转换器,每条都带一次 RPC 会把会话列表拖慢;这里只在**单线程**读路径
// (GetThread / GetThreadHistory)按需现读。非 codex 会话、没落地到电脑的
// 会话(CodexThreadID 空)直接返回 nil,零开销。
func (s *Server) goalDetailFor(ctx context.Context, t *store.Thread) *pb.CodexGoalDetail {
	if t == nil || threadAgentID(t) != "codex" || t.CodexThreadID == "" {
		return nil
	}
	p, ok := s.cfg.Providers["codex"].(*agent.CodexProvider)
	if !ok {
		return nil // 测试/无 codex 环境:注入 fake 不走这里,生产缺 codex 时不拖累读路径
	}
	gctx, cancel := context.WithTimeout(ctx, codexGoalReadBudget)
	defer cancel()
	g, err := p.ReadThreadGoal(gctx, t.CodexThreadID)
	if err != nil {
		// 读不到不是错误:降级为本地库目标(手机设过就照常显示),绝不把历史页打挂。
		log.Printf("[goal] 回读失败(按本地目标展示)thread=%s codex=%s: %v", t.ID, t.CodexThreadID, err)
		return goalDetailFrom(nil, t.Goal)
	}
	return goalDetailFrom(g, t.Goal)
}

// codexGoalReadBudget 目标回读的上界。provider 侧 thread/goal/get 走
// withStatusClient(内部自带 10s 状态查询上界),这里再给一个更早的显式上界:
// 打开历史页不应等目标态太久,到点降级为本地值。
const codexGoalReadBudget = 8 * time.Second

// goalDetailFrom 把 codex 目标态 + 本地兜底翻成协议消息(纯函数,可单测)。
//
// 【兜底语义】codex 侧没设目标(g==nil)但本地库有(手机设置后落过库,
// 可能尚未补发到 codex)→ objective 用本地值,status 留空;两边都空 → nil
// (手机端"有则显示,无则不占位")。codex 有值时 objective 为空也用本地补。
func goalDetailFrom(g *agent.CodexGoalState, localGoal string) *pb.CodexGoalDetail {
	if g == nil {
		if strings.TrimSpace(localGoal) == "" {
			return nil
		}
		return &pb.CodexGoalDetail{Objective: localGoal}
	}
	out := &pb.CodexGoalDetail{
		Objective:       g.Objective,
		Status:          g.Status,
		TokensUsed:      g.TokensUsed,
		TokenBudget:     g.TokenBudget,
		TimeUsedSeconds: g.TimeUsedSeconds,
	}
	if strings.TrimSpace(out.Objective) == "" {
		out.Objective = localGoal
	}
	return out
}

// tailByRounds 按轮尾部截取历史消息(纯函数,可单测)。
//
// 【一轮的定义】一条 role=="user" 起始,到下一条 user 之前的所有消息。
// 失败轮可能只有 user 无 assistant —— 该 user 自己就算一轮;导入/fork 的
// 种子首条不保证是 user,它属于"任何一轮之前"的悬前部分,不参与轮次计数。
// 【截断方式】从尾向前数 user 轮起点,数满 rounds 轮即从那轮的 user 起切到
// 尾部:最后一条 user 之后的全部 assistant 都属于最后一轮(悬挂容忍)。
// 【兼容铁律】rounds<=0 一律原样返回全部 —— 存量 APK 不传 rounds(=0),
// 导出全量与断流恢复(P1-5 ResyncRequired 后的全量回填)都依赖"全部"语义;
// rounds 大于总轮数(可数轮起点不足 N 个)时同样全量返回。
// 【与 limit 的叠加顺序】调用方先做 limit 条数尾截(既有语义、优先级保持
// 现状),本函数再在 limit 的结果上按轮收窄:两条约束同时满足才返回。
func tailByRounds(msgs []store.StoredMessage, rounds int) []store.StoredMessage {
	if rounds <= 0 || len(msgs) == 0 {
		return msgs
	}
	seen, start := 0, 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			seen++
			if seen == rounds {
				start = i
				break
			}
		}
	}
	if seen < rounds {
		return msgs // 轮数不足:全量
	}
	return msgs[start:]
}

// zcodeHistorySource 是 ZcodeProvider 历史拉取能力的最小接口(生产由
// *agent.ZcodeProvider 实现;抽接口是为了 server 单测注入 fake,不依赖真进程)。
type zcodeHistorySource interface {
	HistoryFor(ctx context.Context, sessionID string) ([]agent.HistoryMsg, error)
}

// zcodeHistoryBudget 单次导入历史拉取的整段上界:与 zcode 侧只读查询同级
// (冷启子进程 + resume)。到点即降级,历史页不能等出无限转圈。
const zcodeHistoryBudget = 20 * time.Second

// zcodeHistoryMu 串行化"首次拉取导入历史":同一导入线程被并发打开时,
// 防止两个请求各拉一次、各 append 一份(双写翻倍)。进程级锁足矣 ——
// 缓存落库后这段逻辑根本不会再进。
var zcodeHistoryMu sync.Mutex

// maybePullZcodeImportedHistory 导入的 zcode 线程本地还没有任何消息时,
// 拉一次桌面端历史并落库缓存。失败降级(已有内容原样返回),不影响本次请求。
func (s *Server) maybePullZcodeImportedHistory(ctx context.Context, t *store.Thread) {
	if t == nil || len(t.Messages) > 0 {
		return // 已有落库 = 首次拉取的缓存,或导入后的新轮次:复用,不在线
	}
	zcodeHistoryMu.Lock()
	defer zcodeHistoryMu.Unlock()
	// 双检:重取快照确认并发下没被先到者拉完。
	cur, ok := s.cfg.Store.Get(t.ID)
	if !ok || len(cur.Messages) > 0 {
		return
	}
	src, ok := s.cfg.Providers["zcode"].(zcodeHistorySource)
	if !ok {
		log.Printf("[zcode] 导入线程 %s 拉历史:zcode provider 不可用,按已有内容返回", t.ID)
		return
	}
	hctx, cancel := context.WithTimeout(ctx, zcodeHistoryBudget)
	defer cancel()
	history, err := src.HistoryFor(hctx, cur.ExternalSession.SessionID)
	if err != nil {
		log.Printf("[zcode] 拉取导入历史失败(降级为已有内容)thread=%s session=%s: %v",
			t.ID, cur.ExternalSession.SessionID, err)
		return
	}
	if len(history) == 0 {
		return // 空历史不缓存:保留下次打开再试的机会
	}
	now := time.Now().UnixMilli()
	for _, h := range history {
		createdAt := h.CreatedAtUnixMs
		if createdAt <= 0 {
			createdAt = now // 兜底:应答缺 time.created 时按导入时刻起算
		}
		cur.Messages = append(cur.Messages, store.StoredMessage{
			Role:      h.Role,
			Blocks:    []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: h.Text}}}},
			CreatedAt: createdAt,
		})
	}
	// Update 合并本次追加,不覆盖拉取期间其他请求更新的元数据。
	if err := s.cfg.Store.Update(cur); err != nil {
		log.Printf("[zcode] 导入历史落盘失败(内存版本次可见,重启后重拉)thread=%s: %v", t.ID, err)
		return
	}
	log.Printf("[zcode] 导入历史已拉取并缓存 %d 条 thread=%s session=%s",
		len(history), t.ID, cur.ExternalSession.SessionID)
}

// ---------- StartTurn:整个产品的心脏 ----------

// 【讲解】这是全项目最重要的一段。gRPC 双向流 = 两条同时流动的河:
//
//	手机 → daemon:开场输入、审批决定、取消
//	daemon → 手机:思考/文本增量、工具过程、审批请求、最终结果
//
// 一条 TCP 连接上跑完一轮 agent,和你用网页版 AI 聊天看到"逐字输出"
// 是同一类技术(Server-Sent Events 思想),只是换成了 gRPC 帧格式。
func (s *Server) StartTurn(stream pb.ThreadService_StartTurnServer) error {
	// 1) 第一帧必须是 TurnInput
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	input := first.GetInput()
	if input == nil {
		return errInvalidArg("第一帧必须是 TurnInput")
	}
	t, ok := s.cfg.Store.Get(input.ThreadId)
	if !ok {
		return errNotFound("会话不存在")
	}
	// Reserve the whole stream (including queued turns and final persistence).
	// Cancellation is not completion: never overlap a replacement with an old
	// provider that is still unwinding. Waiting is bounded by the phone context.
	streamRunCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()
	waitCtx, waitCancel := context.WithTimeout(stream.Context(), 5*time.Second)
	lease, err := s.activeTurns.acquire(waitCtx, t.ID, streamCancel)
	waitCancel()
	if err != nil {
		return errFailedPrecondition("上一轮仍在收尾，请稍后重试")
	}
	defer s.activeTurns.Clear(t.ID, lease)
	t, ok = s.cfg.Store.Get(input.ThreadId)
	if !ok {
		return errNotFound("会话不存在")
	}

	// 【会话内切换模型 / 思考强度 —— 2026-09-18 修复】
	// proto 里 TurnInput.model_id / effort 本来就是"每轮覆盖(可选)",App 每轮都会把
	// 该会话当前选定的值送上来(ChatScreen 进入会话时断言 + TurnSession 发帧)。
	// 但这里过去**从来没读过这两个字段**:provider 取的是建会话时落库的 t.ModelID,
	// 强度取的是 t.Effort(一直是空)→ "在会话里换成 GPT-5.6 / 高强度"永远无效,
	// 实际跑的还是设置页默认模型与桌面强度。实测:会话里选 GPT-5.6 Sol + 高,
	// 电脑端 turn_context 里却是 deepseek-flash + low。
	//
	// 现在:以本轮上报为准覆盖到线程上(空串是**有意义的取值** = 跟随桌面配置,
	// 所以是赋值而不是"非空才覆盖"),下面的 Store.Update 会连同落库,
	// 于是"只影响本轮及之后"成立、重启 daemon 不丢,且依然不碰桌面 config.toml。
	applyTurnOverrides(t, input)

	turnID := randomID("turn")

	// W7:本轮的资源租约(规格 §6.5 的强制点)——把当前预算策略里这一 thread 的
	// 事件缓冲额度交给记账层,随后每帧事件的字节数都记在它上面。额度是**每次现取**的,
	// 所以改过策略后,下一轮申请的租约立刻按新额度记账(不需要重启 daemon)。
	// 【为什么不是闸门】事件承载用户数据(回复文本/工具输出/审批请求),到顶也不丢;
	// 触顶只让压力在 GetResourceUsage 里可见 —— 需要腾地方是压缩/落盘的事。
	turnLease := s.openTurnLease(streamRunCtx, t, turnID)
	defer s.closeTurnLease(turnLease)

	// 【死连接防线】手机掉了(TCP 半死)时,stream.Send 会在流控窗口满后**无限阻塞**;
	// 而 sink 的调用方是 codex 事件分发的唯一消费者 —— 它一堵,codex 的 readLoop
	// 跟着堵,手机上看到的就是"输出一小段后长时间停顿"(2026-09-18 用户反馈实锤)。
	// 防线:① 发送前先查流是否已死(keepalive 杀死/对端 RST),死了直接丢弃,
	// 轮次继续在电脑上跑、结果照常落库(R10 语义不变);② 一旦 Send 报错,
	// 标记死亡,后续事件不再尝试 —— 最多只堵一次 Send(等 keepalive 检出),
	// 而不是每条消息都顺序堵一遍。
	var wireTurnID atomic.Value
	wireTurnID.Store(turnID)
	transport := newTurnTransport(stream.Context(), func(ev *pb.TurnServerMessage) {
		ev.Sequence = s.recordTurnFrame(t.ID, wireTurnID.Load().(string), ev)
		// W7:每帧事件的字节数记到本轮租约上。proto.Size 是纯计算(不编码、不分配),
		// 而这一帧本来就会在下游被序列化一次 —— 再算一次真实编码长度等于白跑一遍编码器。
		s.chargeTurnLease(turnLease, int64(proto.Size(ev)))
	}, stream.Send)
	defer transport.finish()
	sink := agent.Sink(transport.emit)

	// 【项4:思考流落库】sink 总线外再包一层"思考记录器":ThinkingDelta 边推边记,
	// 轮次终局时前置为 ThinkingBlock 随 assistant 消息落库 —— 否则思考文本只活在
	// 手机内存里,重启 app / 重进会话后历史恢复就看不到思考卡(用户实锤的
	// "关掉隐藏推理后思考卡会消失")。实现见 thread_thinking.go。
	var thinkBuf *turnThinkingBuf
	sink, thinkBuf = thinkingRecorder(sink)

	// provider 必须在 sink 就绪后取:选不出(该会话的 agent 不可用)也要能发
	// TurnFailed 帧,而不是让手机对着一个啥都没发生的流干等(显式失败,不回退)。
	provider, perr := s.providerForThread(t)
	if perr != nil {
		log.Printf("[turn] %s", perr.Error())
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Failed{Failed: &pb.TurnFailed{
			TurnId: turnID,
			Error:  &pb.Error{Code: "AGENT_UNAVAILABLE", Message: perr.Error()},
		}}})
		return nil
	}

	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Started{Started: &pb.TurnStarted{
		TurnId: turnID, ModelId: t.ModelID,
	}}})

	// 2) 审批中继:provider 要审批时 → 发给手机 → 等手机答复
	// 【R10】轮次生命周期与手机连接解耦:USB/WiFi 闪断不再杀掉电脑上正在跑
	// 的任务 —— 结果照常落库,手机重连后轮询历史就能拿回完整回复。
	// 只有手机显式"取消"帧、同线程重入打断或 30 分钟上限会中止轮次。
	approvals := newTurnApprovals()
	s.approvalSessions.Store(t.ID, approvals)
	defer s.approvalSessions.CompareAndDelete(t.ID, approvals)
	// streamRunCtx 覆盖"这条流上跑过的所有轮次"(含 P1-4 的排队续跑);
	// 每一轮再派生自己的 runCtx 给 provider。手机取消帧拉闸 streamRunCtx =
	// 当前轮与排队输入一起停(终局判定处统一收口)。
	// rt 供读手查询"当前轮次 id / 能否插话"。排队续跑会换轮次甚至换 provider
	// (排队输入自带模型覆盖),两边都经它读写,避免数据竞争。
	rt := &turnRuntime{}
	rt.update(turnID, provider)
	defer rt.stop(s.turnQueue, t.ID)

	// 后台读手:继续收手机后续帧(审批/取消/运行中新输入),分发给对应通道
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			msg, err := stream.Recv()
			if err != nil {
				return // 流断开 = 只停读;轮次继续在电脑上跑(见上)
			}
			if streamRunCtx.Err() != nil {
				return
			}
			switch m := msg.Msg.(type) {
			case *pb.TurnClientMessage_Approval:
				approvals.deliver(m.Approval)
			case *pb.TurnClientMessage_Cancel:
				// 用户"终止任务":streamCancel 连当前轮与排队输入一起停,
				// 队列在终局判定处清空 —— 取消之后不该再自动跑排队的消息。
				streamCancel()
				return
			case *pb.TurnClientMessage_Input:
				// P1-4:运行中继续发消息 —— codex 走 turn/steer 插入,
				// 其余入内存队列,当前轮终局后由下面的续跑循环自动执行。
				s.handleMidTurnInput(t, m.Input, rt, sink)
			case *pb.TurnClientMessage_Supplement:
				// P1:中途补充上下文
			}
		}
	}()

	// 【vet】call 为指针:pb 消息按值传会被 copylocks 拒绝;只读入参,不改内容。
	approve := func(actx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		reqID := randomID("apr")
		approvalCh := approvals.add(reqID)
		defer approvals.remove(reqID)
		// 原因文案跟着动作类型走(而不是对所有类型都写"危险操作"):
		// 反问类请求被贴"危险操作"会让人误判出了事故。
		request := &pb.ApprovalRequest{RequestId: reqID, Action: call, Reason: agent.ApprovalReason(call)}
		approvals.attach(request)
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ApprovalRequest{ApprovalRequest: request}})
		defer sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ApprovalResolved{ApprovalResolved: &pb.ApprovalResolved{RequestId: reqID, Note: "审批已结束"}}})
		waitCtx, waitCancel := context.WithTimeout(actx, 2*time.Minute)
		defer waitCancel()
		select {
		case d := <-approvalCh:
			return d.Approved, d.Note, nil
		case <-waitCtx.Done():
			return false, "", waitCtx.Err()
		case <-streamRunCtx.Done():
			return false, "", streamRunCtx.Err()
		}
	}

	// 3) 记录用户输入,跑 agent;P1-4:当前轮终局后,排队的输入逐条作为新输入继续跑
	userBlocks := derefBlocks(input.Blocks)
	t.Messages = append(t.Messages, store.StoredMessage{Role: "user", Blocks: userBlocks, CreatedAt: time.Now().UnixMilli()})
	t.Status = pb.ThreadStatus_THREAD_STATUS_RUNNING
	// P2-4:启动轮的输入落库属"尽力而为"(流已开,错误无法回传给手机),
	// 但失败必须留痕 —— 否则磁盘异常期间整轮历史静默丢失。
	logStoreUpdateErr("turn-start", t.ID, s.cfg.Store.Update(t))

	inputBlocks := userBlocks
	for {
		// 轮次主体已抽成 runTurnOnce(2026-09-20,T-L2wire):provider.Run →
		// 思考落库 → 自动标题 → 落库/收尾帧 → 主机事件,逐行保真搬运,
		// StartTurn 与跨 agent 委派引擎(collab_delegate.go)同调 —— daemon
		// 不当"第五个大脑"。这里只保留"这条流"特有的簿记:排队续跑(P1-4)。
		_, userStopped, err := s.runTurnOnce(streamRunCtx, provider, turnID, t, inputBlocks, sink, thinkBuf, approve)
		if err != nil {
			if _, isRunErr := err.(*turnRunError); !isRunErr {
				// 落库失败:与抽取前同点终止(收尾帧已发、主机事件未发)。
				return err
			}
			// 轮次失败:Failed 帧与主机事件已在 runTurnOnce 内发出;与抽取前
			// 一致,单轮失败不拦排队续跑,继续走下面的队列逻辑。
		}
		// 用户主动停(取消帧 / CancelTurn RPC / 重入打断):清空队列并结束。
		// "终止任务"的按钮语义就是停 —— 绝不能停完又自动把排队的跑起来。
		if userStopped {
			rt.stop(s.turnQueue, t.ID)
			return nil
		}
		// P1-4 续跑:取出队首输入作为新一轮。帧序:上一轮终局 → QueueFlushed
		// (手机把"排队中"标签收回普通消息)→ Started → 常规增量。
		next, ok := rt.next(s.turnQueue, t.ID)
		if !ok {
			return nil
		}
		// 排队输入自带的模型/强度覆盖在这里才生效(与首轮同语义:只影响本轮及之后),
		// 换模型 = 换 provider(同 agent 内),和首轮的选择逻辑保持一致;
		// 选不到显式失败(发 Failed 帧),绝不静默回退别的 agent。
		applyTurnOverrides(t, &pb.TurnInput{ModelId: next.modelID, Effort: next.effort})
		provider, perr = s.providerForThread(t)
		if perr != nil {
			log.Printf("[turn] 排队续跑选择 provider 失败: %v", perr.Error())
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Failed{Failed: &pb.TurnFailed{
				TurnId: turnID,
				Error:  &pb.Error{Code: "AGENT_UNAVAILABLE", Message: perr.Error()},
			}}})
			return nil
		}
		turnID = randomID("turn")
		wireTurnID.Store(turnID)
		rt.update(turnID, provider)
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_QueueFlushed{QueueFlushed: &pb.QueueFlushed{
			TurnId: turnID, Remaining: int32(s.turnQueue.Len(t.ID)),
		}}})
		t.Messages = append(t.Messages, store.StoredMessage{Role: "user", Blocks: next.blocks, CreatedAt: time.Now().UnixMilli()})
		t.Status = pb.ThreadStatus_THREAD_STATUS_RUNNING
		logStoreUpdateErr("queued-turn-start", t.ID, s.cfg.Store.Update(t))
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Started{Started: &pb.TurnStarted{
			TurnId: turnID, ModelId: t.ModelID,
		}}})
		inputBlocks = next.blocks
	}
}

// ---------- 单轮主体(StartTurn 与委派引擎共用的心脏)----------

// turnRunError 区分 runTurnOnce 的两种终态错误:provider 轮次失败(本类型
// 包装)与落库失败(直接 errInternal)。StartTurn 对前者继续排队续跑
// (P1-4 语义)、对后者立即终止;委派引擎对前者解包后如实回传 coordinator
// (准则 6:失败如实回传,不代拟成功)。
type turnRunError struct{ err error }

func (e *turnRunError) Error() string { return e.err.Error() }
func (e *turnRunError) Unwrap() error { return e.err }

// concatTextBlocks 取全部文本块的文本顺序拼接(原 StartTurn 错误分支里
// partial 的内联循环抽出):轮次失败时的 PartialText 与委派终稿摘要共用
// 同一实现,行为与抽取前逐字节一致。
func concatTextBlocks(blocks []pb.ContentBlock) string {
	var sb strings.Builder
	// 【vet】按下标取址遍历,避免 range 复制 pb 消息(内嵌 MessageState 含锁)。
	for i := range blocks {
		b := &blocks[i]
		if tb, ok := b.Block.(*pb.ContentBlock_Text); ok {
			sb.WriteString(tb.Text.Text)
		}
	}
	return sb.String()
}

// outputStateOf 判定这一轮「最终正文是怎么来的」,填进 TurnCompleted.output_state。
//
// 【判据】看 final_blocks 里有没有**文本**:
//   - 有非空文本 → TEXT;
//   - 没文本,但有产物块(图片 / 工具调用 / 工具结果)→ ARTIFACT_ONLY;
//   - 其余(空,或只有思考)→ EMPTY。
//
// 【思考块为什么不算产物】思考是过程不是交付物。只有思考没有正文时,用户该看到
// 的是"这轮没有正文",而不是"有产物"—— 后者会让他去附件里找一个不存在的东西。
// 【为什么没有 ERROR】轮次失败走的是 TurnFailed 帧(Failed 与 Completed 互斥),
// 不到这里;enum 里保留 ERROR 是为了让手机端有一个统一的状态视角可用。
// 【为什么不用 final_blocks 是否为空判 EMPTY】空思考块 / 空文本块也算"有块",
// 直接按 len 判定会把"块里没内容"误报成 TEXT。
func outputStateOf(blocks []*pb.ContentBlock) pb.OutputState {
	artifact := false
	for i := range blocks {
		b := blocks[i]
		if b == nil {
			continue
		}
		switch blk := b.Block.(type) {
		case *pb.ContentBlock_Text:
			if strings.TrimSpace(blk.Text.GetText()) != "" {
				return pb.OutputState_OUTPUT_STATE_TEXT
			}
		case *pb.ContentBlock_Image, *pb.ContentBlock_ToolCall, *pb.ContentBlock_ToolResult:
			artifact = true
		case *pb.ContentBlock_Thinking:
			// 过程,不算产物。
		}
	}
	if artifact {
		return pb.OutputState_OUTPUT_STATE_ARTIFACT_ONLY
	}
	return pb.OutputState_OUTPUT_STATE_EMPTY
}

// recoverEmptyTurnOutput 在一轮「最终正文为空」时,问一次原生历史里到底有没有正文。
//
// 【为什么值得多这一次 IO】用户报的"任务完成却什么都没有"里,有一部分是**正文确实
// 存在、只是没经增量到达**(上下文压缩边界、事件漏帧)。只看到空块就报"没有正文",
// 而用户看着电脑上明明有 —— 那正是方案 §4.4 的图二。
//
// 【为什么放在 server 层而不是 provider 里】provider.Run 内部还持着流式状态锁,
// 在那里面等原生 IO 会把同一轮的事件处理一起堵住。NativeTurnReader 本来就是 provider
// 的可选扩展接口(僵尸 RUNNING 恢复用的就是它),这里复用同一条通路、不另造一套。
//
// 返回 (recoveredText, state):
//   - 读到正文      → (正文, RECOVERED_FROM_NATIVE_HISTORY)
//   - 原生记录里也没有正文(turn 正常结束、就是没话说)→ ("", EMPTY)
//   - provider 不支持 / 读取失败/超时 → ("", ADAPTER_LOST_OUTPUT)
//     **不能确认**是"真没有"还是"我们弄丢了" —— 如实标出后者,让用户知道该重试,
//     而不是假装任务正常完成。这正是这一档存在的意义。
func (s *Server) recoverEmptyTurnOutput(ctx context.Context, provider agent.Provider, t *store.Thread) (string, pb.OutputState) {
	reader, ok := provider.(agent.NativeTurnReader)
	if !ok {
		// 该 provider 没有原生历史可查 —— 我们没有理由怀疑它丢了正文。
		return "", pb.OutputState_OUTPUT_STATE_EMPTY
	}
	rctx, cancel := context.WithTimeout(ctx, nativeOutputRecoveryBudget)
	defer cancel()
	res, err := reader.ReadNativeTurn(rctx, t)
	if err != nil {
		return "", pb.OutputState_OUTPUT_STATE_ADAPTER_LOST_OUTPUT
	}
	if res == nil {
		return "", pb.OutputState_OUTPUT_STATE_EMPTY
	}
	if text := strings.TrimSpace(res.Text); text != "" {
		return text, pb.OutputState_OUTPUT_STATE_RECOVERED_FROM_NATIVE_HISTORY
	}
	return "", pb.OutputState_OUTPUT_STATE_EMPTY
}

// nativeOutputRecoveryBudget 空正文补偿读原生历史的等待预算。
// 【为什么给这么小】它在轮次收尾路径上,用户正等着一轮的结束帧;读的是一个本地
// JSONL 文件,2 秒已经远超正常耗时。超时按 ADAPTER_LOST_OUTPUT 处理(见上)。
const nativeOutputRecoveryBudget = 2 * time.Second

// runTurnOnce 跑"一轮"的完整主体:provider.Run → 思考前置落库 → 自动标题
// → 落库 + 收尾帧(Failed/Completed)→ 主机事件。从 StartTurn 的轮次循环体
// 原样抽出(2026-09-20,T-L2wire),StartTurn 与跨 agent 委派引擎
// (collab_delegate.go)同调 —— 委派子线程与手机轮次走完全相同的落库/收尾
// 路径,daemon 绝不重写轮次循环。
//
// 参数与返回值:
//   - ctx:本轮的父上下文(StartTurn 传 streamRunCtx,引擎传委派请求的
//     ctx)。函数内部为 provider 派生独立的 runCtx 并在 Run 返回后取消;
//   - provider:调用方选好的人(StartTurn 经 providerForThread,失败在
//     轮次开始前就有 Failed 帧;引擎同纪律)—— 选择留在调用方是为了保住
//     "选择失败发生在落 user 消息之前"的既有时序;
//   - sink/thinkBuf:调用方经 thinkingRecorder 包好的事件汇与思考缓冲
//     (思考随 assistant 消息落库的语义两边一致);
//   - approve:审批钩子(手机轮次=审批中继;引擎=无用户在环一律拒绝);
//   - finalText:assistant 终稿全部文本块拼接(委派摘要的原料;失败为空);
//   - userStopped:Run 返回那一刻父 ctx 已被取消(手机取消帧/CancelTurn/
//     coordinator 撤回)。必须在 cancel() 之前判定 —— cancel 之后再查
//     Err() 永远非 nil,会把"正常完成"误判成"用户要停"(原注释保留);
//   - err:落库失败 → errInternal(调用方立即终止,与抽取前
//     "return errInternal(err)" 同点同值);provider 失败 → *turnRunError
//     (StartTurn 忽略它继续排队续跑,引擎解包如实回传)。
//
// 【等价性】除"排队续跑簿记(rt/队列/QueueFlushed/换轮次/排队输入落库)"
// 留在外层循环外,本函数逐行保真搬运原循环体;partial 的内联拼接循环换用
// 共享的 concatTextBlocks(同一拼接语义)。StartTurn 行为零变化由全量
// server 测试套佐证。
func (s *Server) runTurnOnce(ctx context.Context, provider agent.Provider, turnID string, t *store.Thread, inputBlocks []pb.ContentBlock, sink agent.Sink, thinkBuf *turnThinkingBuf, approve agent.ApprovalHook) (finalText string, userStopped bool, err error) {
	// 【用户侧记忆写在这里，而不是各调用点的落库处】runTurnOnce 是 StartTurn（含
	// 排队续跑）、跨 agent 委派、协同成员三条路径共用的唯一轮次心脏，四个调用点都把
	// "用户这一轮的输入"原样作为 inputBlocks 传进来（调用方也都是先选好 provider 再
	// append 用户消息，所以此处与"消息落库那一刻"看到的是同一份内容）。
	// 写在各调用点会漏 —— 委派与协同两条路径就是这样：它们自己 append 用户消息后
	// 直接进来，不经过 StartTurn 的那两行，于是子线程只剩 assistant 侧记忆。
	s.recordTurnMemory(provider, t, "user", turnID, inputBlocks)

	// 每轮独立的取消函数:CancelTurn RPC / 重入打断拿到的是"当前这一轮"的;
	// 父 ctx(手机取消帧)被取消时会连带取消这里的子 ctx。
	// 【先取 Err 再 cancel】cancel 之后再查 runCtx.Err() 永远非 nil,
	// 会把"正常完成"误判成"用户要停"。
	runCtx, cancel := context.WithCancel(ctx)
	blocks, usagePtr, runErr := provider.Run(runCtx, agent.TurnRequest{
		Thread: t, InputBlocks: inputBlocks, Workspace: t.Workspace,
	}, sink, approve)
	userStopped = runCtx.Err() != nil
	cancel()
	// 【项4】取走本轮累积的思考文本,前置为 ThinkingBlock 一起落库;
	// take 已清空缓冲,排队续跑的下一轮从空缓冲开始(思考不串轮次)。
	persistBlocks := prependThinkingBlock(blocks, thinkBuf.take())

	// 4) 落库 + 收尾帧(排队续跑的每一轮走完全相同的收尾)
	now := time.Now().UnixMilli()
	// 自动标题:仍是默认"新会话…"时,用第一条用户消息起名(codex 自动命名会再覆盖)
	current, _ := s.cfg.Store.Get(t.ID)
	if strings.HasPrefix(t.Title, "新会话") && current != nil && current.Title == t.Title {
		for _, m := range t.Messages {
			if m.Role != "user" {
				continue
			}
			first := ""
			// 【vet】按下标取址遍历,避免 range 复制 pb 消息(内嵌 MessageState 含锁)。
			for i := range m.Blocks {
				b := &m.Blocks[i]
				if tb, ok := b.Block.(*pb.ContentBlock_Text); ok {
					first = strings.TrimSpace(tb.Text.Text)
					break
				}
			}
			if first == "" {
				continue
			}
			r := []rune(first)
			if len(r) > 24 {
				r = r[:24]
			}
			t.Title = string(r)
			break
		}
	}
	t.LastTurnError = ""
	if runErr != nil {
		t.LastTurnError = runErr.Error()
		partial := concatTextBlocks(blocks)
		// 手机断连/超时/出错也要留住已产出的内容:app 端断流恢复靠轮询
		// 历史判断"电脑跑完了没有",这里不落库它就永远等不到
		if partial != "" {
			t.Messages = append(t.Messages, store.StoredMessage{Role: "assistant", Blocks: persistBlocks, CreatedAt: now})
			s.recordTurnMemory(provider, t, "assistant", turnID, persistBlocks)
		}
		t.Status = pb.ThreadStatus_THREAD_STATUS_IDLE
		failedError := &pb.Error{Code: "AGENT_ERROR", Message: runErr.Error()}
		if carrier, ok := runErr.(agent.ErrorInfoCarrier); ok {
			if info := carrier.ErrorInfo(); info != nil {
				failedError = info
			}
		}
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Failed{Failed: &pb.TurnFailed{
			TurnId: turnID, Error: failedError, PartialText: partial,
		}}})
	} else {
		// 【空正文补偿】先把这一轮的状态判出来;只有在"确实什么都没拿到"时才去问
		// 原生历史 —— 有正文时不查(多一次原生 IO 是纯开销),有产物时也不查
		// (那是 ARTIFACT_ONLY,不是丢正文)。详见 recoverEmptyTurnOutput。
		finalBlocks := refBlocks(persistBlocks)
		state := outputStateOf(finalBlocks)
		source := ""
		if state == pb.OutputState_OUTPUT_STATE_EMPTY {
			recovered, recState := s.recoverEmptyTurnOutput(ctx, provider, t)
			state = recState
			if recovered != "" {
				// 【落库与下发用同一份内容】只补 final_blocks 会让离线快照/事件回放
				// 拿到空白 —— 那是同一轮在不同路径上给出两个事实。
				persistBlocks = append(persistBlocks, pb.ContentBlock{
					Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: recovered}},
				})
				finalBlocks = refBlocks(persistBlocks)
				source = "native_history"
			}
		}
		t.Messages = append(t.Messages, store.StoredMessage{Role: "assistant", Blocks: persistBlocks, CreatedAt: now})
		s.recordTurnMemory(provider, t, "assistant", turnID, persistBlocks)
		t.Usage.InputTokens += usagePtr.GetInputTokens()
		t.Usage.OutputTokens += usagePtr.GetOutputTokens()
		t.Status = pb.ThreadStatus_THREAD_STATUS_IDLE
		// final_blocks 同步带思考块:手机端终局帧只提取文本(finalText),
		// 不会被重复渲染;离线快照/事件回放路径拿到的与落库一致。
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Completed{Completed: &pb.TurnCompleted{
			TurnId: turnID, FinalBlocks: finalBlocks, Usage: usagePtr,
			// 【为什么必须带上它】空正文此前被适配器伪装成一条普通 assistant
			// 文本("(完成,无文本输出)"),手机端只能显示"任务完成 + 一句占位",
			// 用户无从判断该不该重试。OutputState 把"真的没话说""只有产物"
			// "丢了正文"如实分开,手机端据此决定提示什么、要不要重试。
			OutputState:  state,
			OutputSource: source,
		}}})
	}
	if uerr := s.cfg.Store.Update(t); uerr != nil {
		return "", userStopped, errInternal(uerr)
	}
	// 【P1-5】轮次终局发一条主机事件(带 seq 落盘 + WatchEvents 广播):
	// 手机不在 StartTurn 流上(后台推送场景)也能感知"跑完了"。
	if runErr != nil {
		s.emitTurnFinished(t.ID, truncateTitle("任务失败:"+runErr.Error(), 80))
	} else {
		summary := "任务完成"
		if txt := firstUserText(blocks); txt != "" {
			summary = truncateTitle(txt, 80)
		}
		s.emitTurnFinished(t.ID, summary)
	}
	if runErr != nil {
		return "", userStopped, &turnRunError{err: runErr}
	}
	return concatTextBlocks(blocks), userStopped, nil
}

// ---------- provider 选择 ----------

// wirePiTitleHandler 把 Pi 的标题回调接到本地库(仿 zcode/codex 同款落库)。
// Pi 侧在会话名变化(set_session_name 回显 / 电脑端同进程改名)时触发回调;
// 这里按线程 ID 找到会话、标题有变化才写库(防挂载回显形成回调环)。
// 【为什么主体在本文件】server.go 的 New 只保留一行调用,逻辑与测试都收在
// thread.go(pi S 项的接线域,2026-09-19 审计 §7-7)。
func (s *Server) wirePiTitleHandler() {
	pp, ok := s.cfg.Providers["pi"].(*agent.PiProvider)
	if !ok {
		return
	}
	pp.TitleHandler = func(threadID, title string) {
		for _, t := range s.cfg.Store.List(true) {
			if t.ID != threadID || t.Title == title {
				continue
			}
			t.Title = title
			logStoreUpdateErr("pi-title-sync", threadID, s.cfg.Store.Update(t))
			log.Printf("[pi] 标题同步落库 thread=%s title=%.60q", threadID, title)
		}
	}
}

// applyZcodeTitle 把 zcode 的标题回调落库(自动命名同步,与 codex/pi 同语义)。
//
// 【钉住守卫(T-05)】被用户手动改名钉住(TitlePinned)的线程跳过:zcode 没有
// "把手机上的新名字回写给 zcode"的通道,自动命名事件(session/titleUpdated)
// 若无条件落库,用户刚改的名字会在下一轮被顶掉。抽成函数:server.go 的接线
// 只剩一行,规则本身可被单测钉住(与 wirePiTitleHandler 的"逻辑收在 thread.go"
// 同一安排)。
func applyZcodeTitle(st *store.FileStore, threadID, title string) {
	for _, t := range st.List(true) {
		if t.ID != threadID || t.Title == title || t.TitlePinned {
			continue
		}
		t.Title = title
		logStoreUpdateErr("zcode-title-sync", threadID, st.Update(t))
		log.Printf("[zcode] 标题同步落库 thread=%s title=%.60q", threadID, title)
	}
}

// threadAgentID 线程归属 agent:AgentID 缺省(旧库记录)= "codex",
// 与"老记录全是 codex 会话"的事实一致(store 兼容规则)。
func threadAgentID(t *store.Thread) string {
	if t == nil || t.AgentID == "" {
		return "codex"
	}
	return t.AgentID
}

// providerForThread 按线程归属的 agent **显式**选择 provider。
//
// 【为什么不再静默回退默认】旧 providerFor 按 ModelID 前缀选,前缀认不出就退
// 默认 provider —— 单 agent 时代无害,多 agent 之后会变成"我明明选了 Pi,
// 结果发给 Codex 了"(调研 §7.1)。现在:**带归属(AgentID)的会话**找不到
// 对应 provider 就是硬错误,由调用方转 TurnFailed(人话),绝不悄悄换人干活。
//
// 【旧库兼容路径】AgentID 为空的老记录(2026-09-18 前全部如此)保持原有行为:
// 先按 ModelID 前缀选,认不出退默认 provider —— 老用户升级后一行不变。
// 注意 ModelID 可能是空串(App 每轮发空串 = "跟随桌面配置",见 applyTurnOverrides),
// 此时同样走默认 provider,不能硬编码成 codex(测试环境里默认可能根本不是它)。
func (s *Server) providerForThread(t *store.Thread) (agent.Provider, error) {
	if t != nil && t.AgentID != "" {
		if p, ok := s.cfg.Providers[t.AgentID]; ok {
			return p, nil
		}
		return nil, errFailedPrecondition(agentUnavailableHint(t.AgentID))
	}
	// ---- 旧库兼容路径(AgentID 空)----
	if t != nil {
		if i := indexByte(t.ModelID, '/'); i > 0 {
			if p, ok := s.cfg.Providers[t.ModelID[:i]]; ok {
				return p, nil
			}
		}
	}
	if p := s.defaultProvider(); p != nil {
		return p, nil
	}
	return nil, errFailedPrecondition(agentUnavailableHint("codex"))
}

// agentUnavailableHint 找不到会话归属 agent 时的人话(手机端直接展示)。
func agentUnavailableHint(agentID string) string {
	if agentID == "pi" {
		return "电脑上没有可用的 Pi agent(node 或 pi cli.js 未找到)。安装后重启 daemon 即可,本会话的历史不会丢。"
	}
	return "电脑上没有可用的 " + agentID + " agent。请检查它是否已安装/已配置,或新建一个其它 agent 的会话。"
}

// providerFor 依据模型 ID 前缀("glm/glm-4.7")找到对应 provider。
//
// 【多 agent 之后只允许两处调用】非线程路径(无归属上下文)的模型→provider 解析;
// 一切"这个线程用谁跑"的判断一律走 providerForThread(显式失败)。
func (s *Server) providerFor(modelID string) agent.Provider {
	if i := indexByte(modelID, '/'); i >= 0 {
		if p, ok := s.cfg.Providers[modelID[:i]]; ok {
			return p
		}
	}
	return s.defaultProvider()
}

// applyTurnOverrides 把一轮 TurnInput 里的模型/思考强度覆盖写到线程上。
//
// 【为什么单独抽出来】这是"会话内切模型/切强度"唯一真正生效的地方
// (调用点见 StartTurn 的注释)。抽成纯函数才能被单测钉住 —— 它此前压根不存在,
// 于是这个功能静默失效了很久:App 每轮都在发,daemon 一个字都没读。
//
// 【空串为什么是赋值,而不是"跳过"】空串在这里是**用户明确选中的取值**
// "跟随桌面配置"(App 的 picker 里就是空串)。写成"非空才覆盖"的话,用户在会话里
// 切回"跟随桌面"就再也回不去 —— 与 SetCodexSettingsRequest 用 presence 躲开的
// 是同一个坑,只是这里靠"App 每轮必发该字段"的约定来消歧。
func applyTurnOverrides(t *store.Thread, in *pb.TurnInput) {
	if t == nil || in == nil {
		return
	}
	t.ModelID = strings.TrimSpace(in.GetModelId())
	t.Effort = strings.TrimSpace(in.GetEffort())
}

// defaultProvider 挑默认 provider(2026-09-20 动态化,体检 P1-5):
//  1. 设置页落盘的默认模型(store.DefaultModelID)的 provider 前缀 —— 用户
//     显式选过的默认,优先兑现(provider 不在注册表则作废,继续往下);
//  2. 没有设置:注册表里 ID 字典序第一个非 echo provider(Go map 无序,字典序
//     是确定性的"第一个");
//  3. echo 永远最后兜底(零配置也能跑通整条链路)。
//
// 全程不写死任何 agent 名单,未来新增 provider 一视同仁。
func (s *Server) defaultProvider() agent.Provider {
	if id := s.cfg.Store.DefaultModelID(); id != "" {
		if i := indexByte(id, '/'); i > 0 {
			if p, ok := s.cfg.Providers[id[:i]]; ok {
				return p
			}
		}
	}
	ids := make([]string, 0, len(s.cfg.Providers))
	for id := range s.cfg.Providers {
		if id != "echo" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > 0 {
		return s.cfg.Providers[ids[0]]
	}
	return s.cfg.Providers["echo"]
}

func (s *Server) defaultModelID() string {
	p := s.defaultProvider()
	if ms := p.Models(); len(ms) > 0 {
		return p.ID() + "/" + ms[0].Id
	}
	return "echo/echo-1"
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// sortKeys 调试用:稳定遍历 map。
func sortKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var _ = io.EOF

// derefBlocks []*ContentBlock → []ContentBlock
func derefBlocks(in []*pb.ContentBlock) []pb.ContentBlock {
	out := make([]pb.ContentBlock, 0, len(in))
	for _, b := range in {
		if b != nil {
			// 【vet】槽位填充 + proto.Merge:逐字段复制(含 unknownFields),语义等价于
			// *b 整体拷贝,但绕开 append/赋值对 pb 消息的按值拷贝(内嵌 MessageState 含锁)。
			out = append(out, pb.ContentBlock{})
			proto.Merge(&out[len(out)-1], b)
		}
	}
	return out
}

// refBlocks []ContentBlock → []*ContentBlock
func refBlocks(in []pb.ContentBlock) []*pb.ContentBlock {
	out := make([]*pb.ContentBlock, 0, len(in))
	for i := range in {
		out = append(out, &in[i])
	}
	return out
}

// firstUserText 取输入块的纯文本(自动标题用)。
func firstUserText(blocks []pb.ContentBlock) string {
	for i := range blocks {
		if t, ok := blocks[i].Block.(*pb.ContentBlock_Text); ok {
			return strings.TrimSpace(t.Text.Text)
		}
	}
	return ""
}

func truncateTitle(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// activeTurns 记录每线程正在执行的 turn 的取消函数(重入打断用)。
type activeTurns struct {
	mu sync.Mutex
	m  map[string]*activeTurn
}

type activeTurn struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func newActiveTurns() *activeTurns {
	return &activeTurns{m: map[string]*activeTurn{}}
}

func (a *activeTurns) acquire(ctx context.Context, threadID string, cancel context.CancelFunc) (*activeTurn, error) {
	for {
		a.mu.Lock()
		if err := ctx.Err(); err != nil {
			a.mu.Unlock()
			return nil, err
		}
		prev := a.m[threadID]
		if prev == nil {
			lease := &activeTurn{cancel: cancel, done: make(chan struct{})}
			a.m[threadID] = lease
			a.mu.Unlock()
			return lease, nil
		}
		a.mu.Unlock()
		prev.cancel()
		select {
		case <-prev.done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// IDs 返回当前活跃轮次的线程 id 快照。委派父链推断用它（T-S1Pb）：
// agent_delegate 由父轮次发起、父轮次此刻必然持租约，因此活跃集合里排除
// 已知委派子线程后剩下的就是候选父线程。加锁复制，调用方拿到的是快照，
// 不持有锁、不与轮次推进竞争。
func (a *activeTurns) IDs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, 0, len(a.m))
	for id := range a.m {
		out = append(out, id)
	}
	return out
}

func (a *activeTurns) Clear(threadID string, lease *activeTurn) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.m[threadID] == lease {
		delete(a.m, threadID)
		close(lease.done)
	}
}

// Cancel 取消并返回是否确有在跑的轮次。
func (a *activeTurns) Cancel(threadID string) context.CancelFunc {
	a.mu.Lock()
	defer a.mu.Unlock()
	if c := a.m[threadID]; c != nil {
		return c.cancel
	}
	return nil
}

// ---------- 会话导出:离线 HTML 存档 ----------

// piThreadExporter pi 侧离线导出能力的最小接口(生产由 *agent.PiProvider 实现;
// 抽接口同 zcodeHistorySource 模式,单测可注 fake,不依赖真子进程)。
type piThreadExporter interface {
	ExportHTML(t *store.Thread) (string, error)
}

// ExportThreadHtml 把会话导出成离线 HTML 存档(PiProvider.ExportHTML 的 RPC 出口,
// 2026-09-19 审计 §7-4 交付)。返回电脑上的绝对路径,手机端经现有 ReadFile
// (FileChunk 流)下载链路取文件、浏览器离线打开。
//
// 【路由纪律与 CompactThread 一致】会话归属以 AgentID 为准,显式失败不静默换人:
// 目前只有 pi 提供离线导出能力,codex/zcode 会话直接人话报错 —— 绝不悄悄降级成
// "导出纯文本",免得用户以为拿到的是电脑端的完整会话存档。
// 本段是独立追加(放在文件末尾),不触碰 GetThreadHistory 的 zcode 历史拉取区。
func (s *Server) ExportThreadHtml(ctx context.Context, req *pb.ExportThreadHtmlRequest) (*pb.ExportThreadHtmlResponse, error) {
	if strings.TrimSpace(req.GetThreadId()) == "" {
		return nil, errInvalidArg("缺少会话 id")
	}
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	if threadAgentID(t) != "pi" {
		return nil, errFailedPrecondition("目前只有 Pi 会话支持导出 HTML 存档")
	}
	exp, ok := s.cfg.Providers["pi"].(piThreadExporter)
	if !ok {
		return nil, errUnavailable("电脑上没有可用的 pi,无法导出")
	}
	path, err := exp.ExportHTML(t)
	if err != nil {
		// pi 的失败原因(未落地/渲染失败)原样透传,手机端 Toast 可直接展示
		return nil, errInternal(err)
	}
	return &pb.ExportThreadHtmlResponse{Path: path}, nil
}

func (a *activeTurns) Has(id string) bool {
	// nil receiver 语义即"没有任何在飞轮次"(部分构造的 Server,如单测里只填
	// 它关心的字段的那类):查询方一律按"无在飞"处理,不必让它自己判空。
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.m[id]
	return ok
}
