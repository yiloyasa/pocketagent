// codex_subagent_sync.go —— codex 原生子代理线程的轻量存根登记(T-S1P)。
//
// 【为什么需要】codex 的子代理(审批升级审查、collab 多代理等)是真线程:
// thread/started 通知的 ThreadInfo 带 parentThreadId(子代理线程非空,S0 实测
// docs/verify/evidence/codex-subagent-s0.md),但 daemon 从不为它们建本地记录
// —— ListThreads 只读本地库,手机端永远看不到这批子线程,内联卡点不亮。
// 本文件在 thread/started 到达时(SyncThreadEvent 的钩子)把子线程以**存根**
// 形态登记进本地库:id/CodexThreadID = codex 子线程 id、ParentThreadID = 父
// 线程的本地 id、标题 = agentNickname 或"子代理"、状态 IDLE。
//
// 【透传原则】只登记通知自带元数据(id/父/昵称/cwd),不造内容、不拉历史:
// 存根 Messages 为空,后续 thread/status/changed、thread/name/updated、
// thread/deleted 等会话级事实经既有 SyncThreadEvent 路径自动命中存根
// (findLocalThreadByCodexID 按 CodexThreadID 匹配) —— 生命周期零额外代码。
package server

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// maybeRegisterCodexSubagentStub 处理一条 thread/started 通知:若它是某线程的
// 子代理线程且 daemon 本地库还没有它,登记轻量存根。幂等,可安全重入
// (SyncThreadEvent 在独立 goroutine 里被调)。
func (s *Server) maybeRegisterCodexSubagentStub(codexChildTID string, params json.RawMessage) {
	if codexChildTID == "" {
		return
	}
	// 【墓碑守卫】用户主动删过的 codex 子代理存根,thread/started 再到达也不再建回:
	// 删除时已把它 id(= CodexThreadID)记入墓碑,这里命中就跳过,否则"删了又冒出来"。
	if s.cfg.Store.IsTombstoned(codexChildTID) {
		log.Printf("[subagent] codex 子线程 %s 已被用户删除(墓碑命中),跳过存根登记", codexChildTID)
		return
	}
	info, ok := parseCodexStartedThread(params)
	if !ok {
		return
	}
	parent, nickname := info.parentAndNickname()
	if parent == "" {
		return // 普通顶层线程(S0 实测主线程 parentThreadId=null):维持既有行为
	}
	// 幂等闸:子线程已在库(已登记过存根 / 用户单独导入过)一律不动。
	if _, ok := s.cfg.Store.Get(codexChildTID); ok {
		return
	}
	for _, t := range s.cfg.Store.List(true) {
		if t.CodexThreadID == codexChildTID {
			return
		}
	}
	// 父必须能回指到本地线程:ParentThreadID 存的是**父的 daemon 本地 id**
	// (手机端 thread/list 快照全按本地 id 匹配,见 SubAgentsLogic.childrenOf)。
	// 父不在库 = 这条 codex 会话从没被 App 接入,子对话无处归属 —— 如实跳过,
	// 不造关系;等父会话被导入后,后续子线程的存根自然能挂上。
	parentLocal := s.findLocalThreadByCodexID(parent)
	if parentLocal == nil {
		log.Printf("[subagent] codex 子线程 %s 的父 %s 不在本地库,跳过存根登记", codexChildTID, parent)
		return
	}
	title := strings.TrimSpace(nickname)
	if title == "" {
		title = "子代理"
	}
	stub := &store.Thread{
		// id 直接用 codex 子线程 id(= CodexThreadID):uuid 全局唯一,与
		// randomID("th") 不撞;存根因此天然可被 chat/{id} 路由与续聊复用。
		ID:             codexChildTID,
		Title:          title,
		Workspace:      strings.TrimSpace(info.Cwd),
		AgentID:        "codex",
		CodexThreadID:  codexChildTID,
		ParentThreadID: parentLocal.ID,
		Status:         pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := s.cfg.Store.Create(stub); err != nil {
		// 存根是"有则显示"的增强:登记失败只留痕,绝不影响既有同步管线。
		log.Printf("[subagent] codex 子线程存根登记失败 codex=%s: %v", codexChildTID, err)
		return
	}
	log.Printf("[subagent] 已登记 codex 子代理线程存根 %s(parent=%s, title=%q)",
		codexChildTID, parentLocal.ID, title)
	// 【T-SUBVIS 接线点 2】登记成功后异步首拉对话内容:内联卡的预览行
	// (SubAgentsState.requestPreview)因此在用户点开前就有内容,而不是先看到
	// 一张只有标题的空卡。用独立 ctx —— 本函数在通知派发的独立 goroutine 里
	// 被调,首拉可能等满 20s 预算,绝不能让通知管线陪着等;失败只留痕,
	// 存根已落库这一点不受影响(拉取是"有则更好"的增强)。
	go func(id string) {
		if cur, ok := s.cfg.Store.Get(id); ok {
			s.maybePullCodexSubagentHistory(context.Background(), cur)
		}
	}(codexChildTID)
}

// codexStartedThread 是 thread/started 通知里 ThreadInfo 的只解析用形状。
// 【两处候选,按实测优先】S0 实测帧里父 id 在 thread.parentThreadId(蛇形列名
// 见 P0 报告的 SessionSource 变体);threadSource 的 subAgent.thread_spawn 内层
// 形状按 P0 描述容错解析 —— 真实流量若证明内层不存在,主字段也已覆盖,内层
// 只是兜底,不猜、不改写。
type codexStartedThread struct {
	Id             string `json:"id"`
	ParentThreadId string `json:"parentThreadId"`
	AgentNickname  string `json:"agentNickname"`
	Cwd            string `json:"cwd"`
	ThreadSource   *struct {
		Type string `json:"type"`
		Sub  *struct {
			Spawn *struct {
				ParentThreadId string `json:"parent_thread_id"`
				ParentThreadID string `json:"parentThreadId"`
				AgentNickname  string `json:"agent_nickname"`
			} `json:"thread_spawn"`
		} `json:"subAgent"`
	} `json:"threadSource"`
}

// parseCodexStartedThread 解析 thread/started 的 params(坏 JSON = 不处理)。
func parseCodexStartedThread(params json.RawMessage) (codexStartedThread, bool) {
	var n struct {
		Thread codexStartedThread `json:"thread"`
	}
	if err := json.Unmarshal(params, &n); err != nil {
		return codexStartedThread{}, false
	}
	return n.Thread, true
}

// parentAndNickname 取(父 codex 线程 id, 子代理昵称):主字段优先,threadSource
// 内层兜底;昵称只在主字段为空时才取兜底值(不覆盖实测主字段)。
func (t codexStartedThread) parentAndNickname() (parent, nickname string) {
	parent, nickname = t.ParentThreadId, t.AgentNickname
	if parent != "" {
		return parent, nickname
	}
	if t.ThreadSource != nil && t.ThreadSource.Sub != nil && t.ThreadSource.Sub.Spawn != nil {
		parent = strings.TrimSpace(t.ThreadSource.Sub.Spawn.ParentThreadId)
		if parent == "" {
			parent = strings.TrimSpace(t.ThreadSource.Sub.Spawn.ParentThreadID)
		}
		if strings.TrimSpace(nickname) == "" {
			nickname = t.ThreadSource.Sub.Spawn.AgentNickname
		}
	}
	return parent, nickname
}
