// thread_ext.go —— 会话组织与审查推翻的 server 层接线。
//
// 覆盖 proto 里新增的那批 RPC:
//
//	ForkThread / RollbackThread          分支、回滚
//	ListThreadSections / Create / Rename / Delete / MoveThreadToSection   分组
//	ApproveGuardianDeniedAction          推翻审查者的驳回
//
// 以及一个"以前缺掉的角色":把 codex 发来的会话生命周期通知
// (归档/取消归档/删除/改名)同步回本地库 —— 见 SyncThreadEvent。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// ---------- 会话生命周期同步 ----------

// SyncThreadEvent 响应 codex 的会话级通知,把变化写回本地库。
//
// 【为什么需要一个"接收方"】codex 是这些事实的来源:用户在电脑上归档/删除了会话,
// App 应该跟着变。以前我们只在跑任务时监听通知,于是任务之外发生的变化全部丢失,
// 表现是"电脑上删掉的会话,手机上点开还是活的"。
//
// 【只做最小可靠动作】改名改标题、归档改状态、删除删本地条目。
// 回滚(reverted)不改本地消息 —— 我们无法从通知里得知具体丢了哪些回合,
// 凭空裁剪只会让手机上的历史比电脑上更不可信;重新拉取历史才是正解,
// 这件事由手机端下拉刷新完成。
func (s *Server) SyncThreadEvent(codexThreadID, method string, params json.RawMessage) {
	// codex 原生子代理线程(T-S1P):thread/started 带父 id 时以轻量存根登记
	// (codex_subagent_sync.go),使 thread/list 能带出。普通线程 parentThreadId
	// 为空,钩子内部直接返回,后续行为与从前完全一致。
	if method == "thread/started" {
		s.maybeRegisterCodexSubagentStub(codexThreadID, params)
	}
	if codexThreadID == "" {
		return
	}
	local := s.findLocalThreadByCodexID(codexThreadID)
	if local == nil {
		return // 这条 codex 会话没被导入到 App 里,与我们无关
	}
	switch method {
	case "thread/archived":
		s.setLocalArchived(local, true)
		log.Printf("[sync] codex 归档了会话 %s(codex=%s)", local.ID, codexThreadID)
	case "thread/unarchived":
		s.setLocalArchived(local, false)
		log.Printf("[sync] codex 取消归档 %s(codex=%s)", local.ID, codexThreadID)
	case "thread/deleted":
		if err := s.cfg.Store.Delete(local.ID); err != nil {
			log.Printf("[sync] 删除本地会话 %s 失败: %v", local.ID, err)
		} else {
			log.Printf("[sync] codex 删除了会话 %s(codex=%s),本地已同步删除", local.ID, codexThreadID)
		}
	case "thread/name/updated":
		var n struct {
			ThreadName *string `json:"threadName"`
		}
		if json.Unmarshal(params, &n) != nil {
			return
		}
		name := ""
		if n.ThreadName != nil {
			name = strings.TrimSpace(*n.ThreadName)
		}
		if name != "" && name != local.Title {
			local.Title = name
			logStoreUpdateErr("codex-rename-sync", local.ID, s.cfg.Store.Update(local))
		}
	case "thread/status/changed":
		// codex 的 active/idle 与我们的 RUNNING/IDLE 对得上;notLoaded 不动
		// (那只是"没加载进内存",不代表会话状态变了)。
		var n struct {
			Status struct {
				Type string `json:"type"`
			} `json:"status"`
		}
		if json.Unmarshal(params, &n) != nil {
			return
		}
		if local.Status == pb.ThreadStatus_THREAD_STATUS_ARCHIVED {
			return // 归档是更强的状态,别被 idle 覆盖掉
		}
		switch n.Status.Type {
		case "active":
			s.setLocalStatus(local, pb.ThreadStatus_THREAD_STATUS_RUNNING)
		case "idle":
			s.setLocalStatus(local, pb.ThreadStatus_THREAD_STATUS_IDLE)
			// 【T-SUBVIS 接线点 3】codex 子代理线程由 active 转 idle = 这一轮跑完,
			// 全量已成定局:异步重拉一次 thread/read 把对话内容补进存根(函数内自带
			// "非子代理存根/在飞/已有拉取"守卫,细节见 codex_subagent_history.go)。
			// 这里的 ParentThreadID 前置判断只为不给普通线程白起一个 goroutine;
			// 用 go 是因为重拉可能等满 20s 预算,不能拖住这条同步管线。
			if local.ParentThreadID != "" {
				go s.RefreshCodexSubagentHistory(codexThreadID)
			}
		}
	}
}

func (s *Server) findLocalThreadByCodexID(codexThreadID string) *store.Thread {
	for _, t := range s.cfg.Store.List(true) {
		if t.CodexThreadID == codexThreadID {
			return t
		}
	}
	return nil
}

func (s *Server) setLocalArchived(t *store.Thread, archived bool) {
	status := pb.ThreadStatus_THREAD_STATUS_IDLE
	if archived {
		status = pb.ThreadStatus_THREAD_STATUS_ARCHIVED
	}
	s.setLocalStatus(t, status)
}

func (s *Server) setLocalStatus(t *store.Thread, status pb.ThreadStatus) {
	if t.Status == status {
		return
	}
	t.Status = status
	logStoreUpdateErr("status-sync", t.ID, s.cfg.Store.Update(t))
}

// ---------- 分组(section)----------

func (s *Server) ListThreadSections(ctx context.Context, _ *pb.ListThreadSectionsRequest) (*pb.ListThreadSectionsResponse, error) {
	out := &pb.ListThreadSectionsResponse{}
	p := s.codexProvider()
	if p == nil {
		return out, nil
	}
	sections, err := p.ThreadSections()
	if err != nil {
		// 拉不到就返回空列表而不是报错:设置/侧栏不该因为一次网络抖动整页打不开。
		log.Printf("[section] 列举分组失败: %v", err)
		return out, nil
	}
	// 会话数由本地统计:codex 的分组接口不带计数,为每个分组各查一次会话
	// 是 N 次往返,不值当。
	counts := map[string]int32{}
	for _, t := range s.cfg.Store.List(true) {
		if t.SectionID != "" {
			counts[t.SectionID]++
		}
	}
	for _, sec := range sections {
		sec.ThreadCount = counts[sec.Id]
	}
	out.Sections = sections
	return out, nil
}

func (s *Server) CreateThreadSection(ctx context.Context, req *pb.CreateThreadSectionRequest) (*pb.ThreadSection, error) {
	p, err := s.requireCodex()
	if err != nil {
		return nil, err
	}
	sec, err := p.CreateThreadSection(req.Name)
	if err != nil {
		return nil, errInternal(err)
	}
	return sec, nil
}

func (s *Server) RenameThreadSection(ctx context.Context, req *pb.RenameThreadSectionRequest) (*pb.ThreadSection, error) {
	p, err := s.requireCodex()
	if err != nil {
		return nil, err
	}
	sec, err := p.RenameThreadSection(req.SectionId, req.Name)
	if err != nil {
		return nil, errInternal(err)
	}
	return sec, nil
}

// DeleteThreadSection 删除分组。
//
// 【注意】codex 删分组只删"分组"本身,组里的会话会变成未分组 —— 不会连会话一起删。
// 这一点要在手机端说清楚,否则用户不敢点。
func (s *Server) DeleteThreadSection(ctx context.Context, req *pb.DeleteThreadSectionRequest) (*pb.Empty, error) {
	p, err := s.requireCodex()
	if err != nil {
		return nil, err
	}
	if err := p.DeleteThreadSection(req.SectionId); err != nil {
		return nil, errInternal(err)
	}
	// 本地缓存跟着清:否则这些会话会挂在一个已经不存在的分组下面,再也找不到。
	// 【失败要上报(T-17,原体检 P2-4)】分组已在 codex 侧删掉,这里落库失败意味着
	// 手机上的会话仍挂在已删除的分组下,清单错乱且重启 daemon 前好不了 —— 这是
	// 有现成错误通道的 RPC 收尾,不再走"吞错仅日志"的埋点。title-sync /
	// status-sync 等没有回传通道的点位维持日志口径不变(见 store_observability.go)。
	if err := s.clearSectionLocalCache(req.SectionId); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// clearSectionLocalCache 把本地库里仍挂在 sectionID 下的会话逐条摘出来
// (SectionID 置空并落盘)。任一条落盘失败即中止并返回错误;失败那条同时留
// op=section-clear 埋点日志,排障口径与其它落盘点一致,而上报给手机的是人话文案
// (底层原因不透给 UI,AUDIT §3.3)。
func (s *Server) clearSectionLocalCache(sectionID string) error {
	for _, t := range s.cfg.Store.List(true) {
		if t.SectionID != sectionID {
			continue
		}
		t.SectionID = ""
		err := s.cfg.Store.Update(t)
		logStoreUpdateErr("section-clear", t.ID, err)
		if err != nil {
			return errInternal(errors.New("删除分组成功,但会话清单同步失败,请重试或重启 daemon"))
		}
	}
	return nil
}

// MoveThreadToSection 把会话移进分组;sectionId 为空 = 移出分组。
//
// 【覆盖范围:三家 agent 都能分组,但生效路径分两条】分组在 UI 上对全部 agent
// 的会话开放(长按面板/左滑"分组",ThreadsScreen 的 SECTION_COVERAGE_NOTE 也
// 明说"三家都可以移入"),但生效路径不同:
//   - codex:桌面端也有分组概念,先下发 codex(threadSection 是它的事实来源),
//     成功后落库缓存 —— 不下发就是假开关(归档同款教训)。
//   - pi / zcode:agent 自己没有分组概念,分组纯本地落库,不要求 codex 在场。
//     修复前这里一刀切要求 codexProvider 且线程已落地,pi/zcode 会话(永远没有
//     codex_thread_id)被硬拒,报的还是"先发一条消息"这种牛头不对马嘴的话 ——
//     与 UI 的覆盖说明自相矛盾(用户实锤待补项,2026-09-20 T-05)。
func (s *Server) MoveThreadToSection(ctx context.Context, req *pb.MoveThreadToSectionRequest) (*pb.Thread, error) {
	t, err := s.requireThread(req.ThreadId)
	if err != nil {
		return nil, err
	}
	if threadAgentID(t) == "codex" {
		p, err := s.requireCodex()
		if err != nil {
			return nil, err
		}
		if t.CodexThreadID == "" {
			return nil, errFailedPrecondition("这个会话还没有在电脑上落地(先发一条消息),暂时无法分组")
		}
		if err := p.MoveThreadToSection(t.CodexThreadID, req.SectionId); err != nil {
			return nil, errInternal(err)
		}
	}
	// 收尾两条路径共用:codex 下发成功后落库缓存,pi/zcode 本地落库即生效。
	// codex 侧的列表同步(mergeCodexEntries)只碰 CodexThreadID 非空的线程,
	// 不会把 pi/zcode 手动分的组冲掉。
	t.SectionID = req.SectionId
	if err := s.cfg.Store.Update(t); err != nil {
		return nil, errInternal(err)
	}
	return s.threadToPB(t), nil
}

// ---------- 分支 / 回滚 ----------

// ForkThread 从当前会话分出一条新会话(原会话不动)。
//
// 【为什么落成一条新的本地会话而不是"跳过去"】分叉的意义是"从这里试另一条路,
// 原路留着"。手机上必须同时看得见两条,所以这里直接建一条本地线程指向新 codex 线程,
// 用户点开就是分叉点的上下文。
func (s *Server) ForkThread(ctx context.Context, req *pb.ForkThreadRequest) (*pb.Thread, error) {
	src, err := s.requireThread(req.ThreadId)
	if err != nil {
		return nil, err
	}
	// pi 路径:pi 0.99.1 的 clone 在"当前叶"分出新分支会话文件,与 server 语义
	// ("分出一条新会话,原会话不动")直接对齐 —— pi_fork_plan.md §0 的语义裁决。
	// 仿 CompactThread 的 pi 分家先例(thread.go:600-611):会话归属以 AgentID 为
	// 准,显式失败不静默换人;req.LastTurnId 是 codex 回合 id 语义,pi 侧 v1 无
	// entryId 映射,固定"从最新状态分叉"(降级不破坏,§3 v2 展望)。
	if threadAgentID(src) == "pi" {
		pc, ok := s.cfg.Providers["pi"].(piShapedForker)
		if !ok {
			return nil, errUnavailable("电脑上没有可用的 pi")
		}
		return s.forkPiThread(src, pc)
	}
	p, err := s.requireCodex()
	if err != nil {
		return nil, err
	}
	if src.CodexThreadID == "" {
		return nil, errFailedPrecondition("这个会话还没有在电脑上落地(先发一条消息),暂时无法分叉")
	}
	newCodexID, err := p.ForkThread(src.CodexThreadID, req.LastTurnId)
	if err != nil {
		return nil, errInternal(err)
	}
	t := &store.Thread{
		ID:            randomID("th"),
		Title:         forkTitle(src.Title),
		Workspace:     src.Workspace,
		ModelID:       src.ModelID,
		Effort:        src.Effort,
		SectionID:     src.SectionID,
		CodexThreadID: newCodexID,
		CreatedAt:     time.Now().UnixMilli(),
		UpdatedAt:     time.Now().UnixMilli(),
		// 【必须把历史读进来】真机验过:分叉出来的会话如果本地不带消息,
		// 手机上打开就是**空白时间线**(GetThreadHistory 只读本地 store,
		// 并没有"去 codex 拉历史"这条路径 —— 这里曾经写着有,是错的)。
		// 空白的分叉看着等于"分叉功能坏了",所以分叉时就按导入那套把
		// 新 codex 线程的 rollout 读进来。
		Messages: forkSeedMessages(newCodexID, src, codexRolloutMessages),
	}
	if err := s.cfg.Store.Create(t); err != nil {
		return nil, errInternal(err)
	}
	// 【把分叉标题也推给 codex】否则下次同步标题时,codex 会用**源会话的名字**
	// 覆盖掉我们加的那个「· 分支」标记 —— 真机上表现为列表里两条一模一样的会话,
	// 用户根本分不清哪条是分叉。codex 侧改名失败不影响分叉本身,记日志即可。
	if err := p.SetCodexThreadName(newCodexID, t.Title); err != nil {
		log.Printf("[fork] 把标题写回 codex 失败(不影响分叉): %v", err)
	}
	log.Printf("[fork] %s → %s(codex %s → %s)", src.ID, t.ID, src.CodexThreadID, newCodexID)
	return s.threadToPB(t), nil
}

// forkPiThread pi 线程的分叉组装(ForkThread 的 pi 分支载体,避免在原函数里叠
// 分支;codex 路径一行不动):provider 完成 clone 并回传新会话引用,这里负责建
// 本地线程与落库 —— 与 mount"回填由 server 落库"的分工一致(pi_fork_plan.md §3-7)。
//
// 【不填 CodexThreadID】pi 线程的身份在 ExternalSession;历史播种走
// forkSeedMessages("", src, nil) 的兜底路径(newCodexID=="" 时复制 src.Messages,
// 天然复用 —— pi 侧没有"读新线程 rollout"的 load 通道,v1 以源历史播种)。
func (s *Server) forkPiThread(src *store.Thread, pc piShapedForker) (*pb.Thread, error) {
	if src.ExternalSession.SessionFile == "" {
		return nil, errFailedPrecondition("这个会话还没有在电脑上落地(先发一条消息),暂时无法分叉")
	}
	ref, err := pc.ForkThread(src)
	if err != nil {
		return nil, errInternal(err)
	}
	t := &store.Thread{
		ID:              randomID("th"),
		AgentID:         "pi",
		Title:           forkTitle(src.Title),
		Workspace:       src.Workspace,
		ModelID:         src.ModelID,
		Effort:          src.Effort,
		SectionID:       src.SectionID,
		ExternalSession: ref,
		CreatedAt:       time.Now().UnixMilli(),
		UpdatedAt:       time.Now().UnixMilli(),
		// 与 codex 路径同理由(thread.go GetThreadHistory 只读本地 store):
		// 分叉会话不带消息,手机上打开就是空白时间线 —— 用源历史播种。
		Messages: forkSeedMessages("", src, nil),
	}
	if err := s.cfg.Store.Create(t); err != nil {
		return nil, errInternal(err)
	}
	// 【把分叉标题也推给 pi】否则下次 pi 侧标题同步(set_session_name/TitleHandler)
	// 可能用源会话的名字顶掉「· 分支」标记(codex 路径 SetCodexThreadName 同款教训)。
	// Rename 走 titlePusher(生产实现 pi.go Rename → set_session_name):新线程此刻
	// 在 provider 侧还没有子进程,会冷启一个挂上分支文件的客户端再改名 —— 顺带把
	// 新线程的子进程按新线程 id 就位,后续 Run 免一次 spawn。失败不影响分叉本身,
	// 记日志即可(与 codex 路径同纪律)。
	if tp, ok := pc.(titlePusher); ok {
		if err := tp.Rename(t, t.Title); err != nil {
			log.Printf("[fork] 把标题写回 pi 失败(不影响分叉): %v", err)
		}
	}
	log.Printf("[fork] %s → %s(pi %s → %s)", src.ID, t.ID,
		filepath.Base(src.ExternalSession.SessionFile), filepath.Base(ref.SessionFile))
	return s.threadToPB(t), nil
}

// forkSeedMessages 决定分叉出来的本地会话带哪些历史。
//
// 优先读新 codex 线程的 rollout(那是权威副本:codex 的 fork 会把源历史复制过去,
// 包括电脑上跑过的、手机从没见过的那些回合)。读不到(rollout 还没落盘)时退回
// 复制源会话已知的消息 —— 宁可少显示,也不能让用户面对一个空白会话。
func forkSeedMessages(
	newCodexID string,
	src *store.Thread,
	load func(string) ([]store.StoredMessage, error),
) []store.StoredMessage {
	if newCodexID != "" {
		if msgs, err := load(newCodexID); err == nil && len(msgs) > 0 {
			return msgs
		} else if err != nil {
			log.Printf("[fork] 读取新线程 %s 的 rollout 失败,退回源会话消息: %v", newCodexID, err)
		}
	}
	// 复制一份(切片共用底层数组会在后续裁剪时互相影响)
	out := make([]store.StoredMessage, len(src.Messages))
	copy(out, src.Messages)
	return out
}

func forkTitle(src string) string {
	base := strings.TrimSpace(src)
	if base == "" {
		base = "会话"
	}
	r := []rune(base)
	if len(r) > 20 {
		r = r[:20]
	}
	return string(r) + " · 分支"
}

// RollbackThread 回滚最近 N 个回合。
//
// 【必须让用户知道的两件事】(手机端负责提示,这里在协议层不重复解释)
//  1. 只回滚**对话历史**,不会撤销 agent 已经对本地文件做的改动。要撤销文件改动,
//     得靠 git(这也是为什么这个入口应当和 git 面板一起用)。
//  2. 被丢掉的回合找不回来。
func (s *Server) RollbackThread(ctx context.Context, req *pb.RollbackThreadRequest) (*pb.Thread, error) {
	t, err := s.requireThread(req.ThreadId)
	if err != nil {
		return nil, err
	}
	if req.NumTurns <= 0 {
		return nil, errInvalidArg("回滚回合数必须大于 0")
	}
	p, err := s.requireCodex()
	if err != nil {
		return nil, err
	}
	if t.CodexThreadID == "" {
		return nil, errFailedPrecondition("这个会话还没有在电脑上落地(先发一条消息),暂时无法回滚")
	}
	if err := p.RollbackThread(t.CodexThreadID, req.NumTurns); err != nil {
		return nil, errInternal(err)
	}
	// 本地历史同步裁掉对应的尾部消息:每条 assistant 回复算一个回合的收尾。
	// 这样手机端立刻看到"退回去了",不必等重新拉取。取不满就不动 ——
	// 宁可暂时多显示,也不要凭空删掉用户还看得见的内容。
	//
	// 【先把回合数存下来】下面的循环会把它减到 0,直接在 log 里打 req.NumTurns
	// 永远是 0 —— 排障时会看到"回滚 0 个回合"却少了 5 条消息,无从下手。
	asked := req.NumTurns
	cut := 0
	for ; req.NumTurns > 0; req.NumTurns-- {
		at := lastTurnStart(t.Messages)
		if at < 0 {
			break
		}
		t.Messages = t.Messages[:at]
		cut++
	}
	if err := s.cfg.Store.Update(t); err != nil {
		return nil, errInternal(err)
	}
	log.Printf("[rollback] 会话 %s 请求回滚 %d 个回合,实际裁掉 %d(本地消息 %d 条)",
		t.ID, asked, cut, len(t.Messages))
	return s.threadToPB(t), nil
}

// lastTurnStart 找最后一轮从哪里开始(该轮的用户消息下标)。找不到返回 -1。
func lastTurnStart(msgs []store.StoredMessage) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return i
		}
	}
	return -1
}

// ---------- 推翻审查者的驳回 ----------

// ApproveGuardianDeniedAction 用户推翻自动审查的驳回结论,要求继续执行。
func (s *Server) ApproveGuardianDeniedAction(ctx context.Context, req *pb.ApproveGuardianDeniedActionRequest) (*pb.Empty, error) {
	t, err := s.requireThread(req.ThreadId)
	if err != nil {
		return nil, err
	}
	p, err := s.requireCodex()
	if err != nil {
		return nil, err
	}
	if t.CodexThreadID == "" {
		return nil, errFailedPrecondition("这个会话还没有在电脑上落地")
	}
	if err := p.ApproveGuardianDenied(t.CodexThreadID, req.ReviewId, req.TargetItemId); err != nil {
		// 这类失败几乎都是"结论已经过期/找不到了"。用 FAILED_PRECONDITION 而不是
		// 内部错误:手机端据此提示"这条审查已经过期,请重新发起动作",而不是"系统故障"。
		return nil, errFailedPrecondition(err.Error())
	}
	return &pb.Empty{}, nil
}

// ---------- 会话列表:搜索 / 分组过滤 / 排序 / 归档 ----------

// codexSyncBudget 是同步 codex 会话列表时给 codex 子进程的时间预算。
//
// 为什么是 2.5s:codex 冷启动约 1~2s、子进程重启窗口更久,但 ListThreads 是
// 最高频的会话列表接口,绝不能因为上游(codex 令牌失效 / 子进程反复重启)卡住
// 而让手机端误判"离线"。超时或失败时一律放弃同步、直接用本地库返回 —— 本地库
// 才是会话列表的事实来源,codex 只用来补全分组/归档/标题这些"慢一致性"字段。
const codexSyncBudget = 2500 * time.Millisecond

// syncThreadsFromCodex 把 codex 的会话列表同步进本地库(分组、归档、标题、时间)。
//
// 【只在"用户真的用了过滤/排序"时才打这次同步】原因很实际:daemon 的 codex
// 子进程可能没在跑,一次同步要等它冷启动(1~2 秒)。会话列表是最高频的页面,
// 不该为了一致性每次都付这个代价。而一旦用户点了搜索/分组/归档页,
// 他对"多等一下、但结果是全的"是有预期的。
//
// 【有界时间预算】codex 不可用时(令牌 403 / 子进程反复重启)本次同步必须在
// codexSyncBudget 内结束:成功就用结果补全本地库;超时/失败就放弃,ListThreads
// 直接读本地库返回。绝不让 ListThreads 因此失败或长时间阻塞(否则手机端 8s
// 超时 → 被误标成"离线")。syncCodexInFlight 保证同一时刻只有一个同步在跑,
// 避免 codex 不可用时一堆 goroutine 在 withClient 生命周期锁上排队。
func (s *Server) syncThreadsFromCodex(searchTerm string, includeArchived bool) {
	if !s.syncCodexInFlight.CompareAndSwap(false, true) {
		return
	}
	p := s.codexProvider()
	if p == nil {
		s.syncCodexInFlight.Store(false)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), codexSyncBudget)
	defer cancel()
	type syncResult struct {
		entries []agent.CodexThreadEntry
		err     error
	}
	ch := make(chan syncResult, 1)
	go func() {
		// 标志位由本 goroutine 在真正结束时清除(含 callTimeout 尾),
		// 这样超时路径下不会把新请求又放进来去抢生命周期锁。
		defer s.syncCodexInFlight.Store(false)
		entries, err := p.ListThreadEntries(searchTerm, "", "", false, includeArchived, 0)
		ch <- syncResult{entries, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil || len(r.entries) == 0 {
			if r.err != nil {
				log.Printf("[sync] codex 同步失败,改用本地库返回: %v", r.err)
			}
			return
		}
		// 同步成功:在 ListThreads 读本地库之前把变化写回(见 ListThreads 第 103 行)。
		s.mergeCodexEntries(r.entries)
	case <-ctx.Done():
		// 预算用完:放弃同步,ListThreads 早已用本地库返回,这里直接走人,
		// 绝不阻塞。后台 goroutine 仍会跑完并在结束时清标志位。
		log.Printf("[sync] codex 同步超时(%s),改用本地库返回", codexSyncBudget)
		return
	}
}

// mergeCodexEntries 把 codex 拉回的会话条目写回本地库(分组/归档/标题有变化才更新)。
// 抽出来是因为 syncThreadsFromCodex 现在要在"同步成功"分支里调用。
func (s *Server) mergeCodexEntries(entries []agent.CodexThreadEntry) {
	byCodex := map[string]*store.Thread{}
	for _, t := range s.cfg.Store.List(true) {
		if t.CodexThreadID != "" {
			byCodex[t.CodexThreadID] = t
		}
	}
	for _, e := range entries {
		t, ok := byCodex[e.ID]
		if !ok {
			continue
		}
		changed := false
		if t.SectionID != e.SectionID {
			t.SectionID = e.SectionID
			changed = true
		}
		want := pb.ThreadStatus_THREAD_STATUS_IDLE
		if e.Archived {
			want = pb.ThreadStatus_THREAD_STATUS_ARCHIVED
		}
		if t.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING && t.Status != want {
			t.Status = want
			changed = true
		}
		if e.Title != "" && e.Title != t.Title {
			t.Title = e.Title
			changed = true
		}
		if changed {
			logStoreUpdateErr("codex-list-sync", t.ID, s.cfg.Store.Update(t))
		}
	}
}

// searchTermMatches 本地兜底搜索(codex 版本不支持 searchTerm 时仍能用)。
func searchTermMatches(t *store.Thread, term string) bool {
	if term == "" {
		return true
	}
	low := strings.ToLower(term)
	if strings.Contains(strings.ToLower(t.Title), low) {
		return true
	}
	for _, m := range t.Messages {
		// 【vet】按下标取址遍历,避免 range 复制 pb 消息(内嵌 MessageState 含锁)。
		for i := range m.Blocks {
			b := &m.Blocks[i]
			if tb, ok := b.Block.(*pb.ContentBlock_Text); ok {
				if strings.Contains(strings.ToLower(tb.Text.Text), low) {
					return true
				}
			}
		}
	}
	return false
}
