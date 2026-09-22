// pi_subagent_sync.go —— pi 子代理运行的存根登记(T-SUBA-PI)。
//
// 【缺口】pi 每次派子代理都会在 <父会话文件同目录>/subagent-artifacts/ 留下
// runId 四件套(input / meta / output / transcript,实测 2026-09-21),但 daemon
// 侧对 pi 的子代理**零处理** —— 手机端因此"看不到子智能体"(用户报障)。
// 本文件在列举会话前把每次运行登记成**轻量存根子线程**,手机端据此在父会话下
// 渲染内联卡(呈现能力早已就绪,缺的只是数据)。
//
// 【父子归属:只在候选唯一时挂 —— 不许猜】
// subagent-artifacts 是**工作区级**目录(同工作区多个 pi 会话共用同一个),所以
// "这份 runId 属于哪次父会话"没有显式指针(meta.json 实测无 parent 字段)。
// 归属判定因此必须保守:
//
//	候选 = 同工作区 且 run 的启动时间落在该会话的活动期内;
//	候选恰好 1 个 → 挂父链;0 个或多个 → **不挂**(诚实变暗,卡不亮也不误挂)。
//
// 宁可看不到归属,也不能把子代理挂到不相干的会话下面。
package server

import (
	"log"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// piSubagentStubPrefix 子代理存根线程的 id 前缀(runId 稳定 → id 稳定 → 幂等)。
const piSubagentStubPrefix = "pi-sub-"

// piSubagentTimeSlack 活动期判定允许的时钟余量:run 的 timestamp 与本地会话
// 的 created/updated 来自不同来源,留一点边界余量,避免边界上的子代理被漏掉。
const piSubagentTimeSlack = 2 * time.Minute

// registerPiSubagentStubs 为所有 pi 会话补登记其子代理运行存根。
// 幂等、只读盘、失败只留痕(存根是"有则更好"的增强,绝不打扰主流程)。
func (s *Server) registerPiSubagentStubs() {
	// 先取一份本地库快照:登记过程中会写入,不能边遍历边改。
	locals := s.cfg.Store.List(false)
	piThreads := make([]*store.Thread, 0, 8)
	for _, t := range locals {
		if threadAgentID(t) == "pi" && strings.TrimSpace(t.ExternalSession.SessionFile) != "" {
			piThreads = append(piThreads, t)
		}
	}
	if len(piThreads) == 0 {
		return
	}
	for _, parent := range piThreads {
		runs, err := agent.ScanPiSubagents(parent.ExternalSession.SessionFile)
		if err != nil {
			log.Printf("[subagent] 扫描 pi 子代理失败 thread=%s: %v", parent.ID, err)
			continue
		}
		for i := range runs {
			s.upsertPiSubagentStub(parent, runs[i], piThreads)
		}
	}
}

// upsertPiSubagentStub 按 runId 幂等登记/更新一条子代理存根。
func (s *Server) upsertPiSubagentStub(parent *store.Thread, run agent.PiSubagentRun, allPi []*store.Thread) {
	id := piSubagentStubPrefix + run.RunID
	// 【墓碑守卫】用户删过的 pi 子代理存根,registerPiSubagentStubs 每次列举会话都扫
	// 磁盘重新登记,没墓碑就会"删了又冒出来"。删除时已把 id 记入墓碑,命中即跳过。
	if s.cfg.Store.IsTombstoned(id) {
		return
	}
	t, exists := s.cfg.Store.Get(id)
	if !exists {
		t = &store.Thread{
			ID:        id,
			AgentID:   "pi",
			Workspace: run.Cwd,
			Status:    pb.ThreadStatus_THREAD_STATUS_IDLE,
			// 【归属】唯一候选才挂,否则留空(诚实变暗)——见文件头。
			ParentThreadID: soleParentForRun(run, allPi),
		}
	}
	if t.Title == "" {
		t.Title = piSubagentTitle(run)
	}
	msgs := piSubagentMessages(run)
	// 消息只增不减:重复扫描时不要把已经落库的内容削掉。
	if len(msgs) > len(t.Messages) {
		t.Messages = msgs
	}
	if exists {
		if err := s.cfg.Store.Update(t); err != nil {
			log.Printf("[subagent] 更新 pi 子代理存根失败 %s: %v", id, err)
		}
		return
	}
	if err := s.cfg.Store.Create(t); err != nil {
		log.Printf("[subagent] 登记 pi 子代理存根失败 %s: %v", id, err)
		return
	}
	log.Printf("[subagent] 已登记 pi 子代理存根 %s(agent=%s parent=%q runId=%s)",
		id, run.Agent, t.ParentThreadID, run.RunID)
}

// soleParentForRun 返回该 run 的唯一候选父会话 id;候选不是恰好 1 个则返回 ""。
func soleParentForRun(run agent.PiSubagentRun, allPi []*store.Thread) string {
	hit := ""
	for _, cand := range allPi {
		if !samePiWorkspace(cand, run) || !inActivityWindow(cand, run) {
			continue
		}
		if hit != "" {
			return "" // 多个候选 = 判不出:不挂(不猜)
		}
		hit = cand.ID
	}
	return hit
}

// samePiWorkspace 判断 run 与某 pi 会话是否属于同一工作区目录。
// artifacts 目录与父会话文件同目录 —— 比目录即比工作区(结构性事实,不是猜测)。
func samePiWorkspace(t *store.Thread, run agent.PiSubagentRun) bool {
	a := agent.PiSubagentArtifactsDir(t.ExternalSession.SessionFile)
	if run.Cwd == "" {
		// run 没给 cwd(meta 缺字段)时退化为"只看时间窗口",已在调用方约束下保守。
		return a != ""
	}
	return a != "" && strings.HasPrefix(a, run.Cwd)
}

// inActivityWindow 判断 run 的启动时间是否落在该会话的活动期内(含余量)。
func inActivityWindow(t *store.Thread, run agent.PiSubagentRun) bool {
	if run.StartedAt.IsZero() {
		return false // 时间未知 = 判不出:不挂
	}
	start := time.UnixMilli(t.CreatedAt).Add(-piSubagentTimeSlack)
	end := time.UnixMilli(t.UpdatedAt).Add(piSubagentTimeSlack)
	if t.UpdatedAt == 0 {
		end = time.Now().Add(piSubagentTimeSlack)
	}
	return !run.StartedAt.Before(start) && !run.StartedAt.After(end)
}

// piSubagentTitle 存根标题:子代理类型 + 启动时刻(人话、可区分多次运行)。
func piSubagentTitle(run agent.PiSubagentRun) string {
	name := strings.TrimSpace(run.Agent)
	if name == "" {
		name = "子代理"
	}
	if run.StartedAt.IsZero() {
		return name
	}
	return name + " · " + run.StartedAt.Format("01-02 15:04")
}

// piSubagentMessages 把子代理对话翻成本地消息(只落有正文的,诚实边界)。
func piSubagentMessages(run agent.PiSubagentRun) []store.StoredMessage {
	out := make([]store.StoredMessage, 0, len(run.Messages)+1)
	if o := strings.TrimSpace(run.Output); o != "" {
		out = append(out, store.StoredMessage{
			Role: "assistant",
			Blocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{
				Text: &pb.TextBlock{Text: "[产出] " + o},
			}}},
		})
	}
	for _, m := range run.Messages {
		role := strings.TrimSpace(m.Role)
		if role == "" {
			role = "assistant"
		}
		out = append(out, store.StoredMessage{
			Role: role,
			Blocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{
				Text: &pb.TextBlock{Text: m.Text},
			}}},
		})
	}
	return out
}
