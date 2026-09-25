// zcode_subagent_sync.go —— zcode 原生子代理清单 → 存根子线程登记(P2)。
//
// 【缺口】zcode 的子代理是 Agent/Task 工具,内部对话不进父会话事件流;手机端
// 的子代理卡此前只能靠 tool.updated 的入参/结果"结构推测"
// (agent/zcodeapp ToolInputIsSubagent 那条线)。0.16.9 起 app-server 提供
// session/subagents 原生清单:provider 在轮次终局拉一次(agent/zcode_subagents.go),
// 经 ZcodeSubagentsHandler 钩子送进来,本文件把它落成**存根子线程**,手机端
// 既有子代理卡管线(thread/list 按 parentThreadId 过滤)原样点亮。
//
// 【与 codex/pi 存根的同与不同】
//   - 同:存根形态、幂等 upsert、墓碑守卫(用户删过的存根不再建回);
//   - 不同:父子归属不需要猜 —— codex 靠通知里的 parentThreadId 回查,pi 要
//     在多候选里"唯一才挂",这里清单本来就是**按父会话**取的,直接挂。
//
// 【透传边界】清单只给 type/title/summary/status/时间戳,没有子代理对话内容
// (要内容得对每个 childSession 再走 session/messages,不在本轮范围)。落库
// 的消息只有一条摘要行,标题/状态原样 —— 未知 status 值按 IDLE 处理(不猜),
// 语义映射的出处见 zcodeSubagentStubStatus。
package server

import (
	"log"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// zcodeSubagentStubPrefix 子代理存根线程的 id 前缀(childSessionId 稳定 →
// id 稳定 → 幂等;与 pi 的 "pi-sub-" 同款惯例)。
const zcodeSubagentStubPrefix = "zc-sub-"

// registerZcodeSubagentStubs 把一轮拉到的原生子代理清单登记/更新为存根子线程。
// 由 ZcodeProvider.SubagentsHandler 在轮次终局的独立 goroutine 里调用:
// 幂等、失败只留痕,绝不影响轮次管线。
func (s *Server) registerZcodeSubagentStubs(threadID string, res zcodeapp.SubagentsResult) {
	parent, ok := s.cfg.Store.Get(threadID)
	if !ok {
		// 父线程不在本地库(zcode 会话从未被 App 接入):子代理无处归属,如实跳过,
		// 与 codex 存根"父不在库不造关系"的口径一致。
		return
	}
	for i := range res.Running {
		s.upsertZcodeSubagentStub(parent, &res.Running[i])
	}
	for i := range res.Ended.Items {
		s.upsertZcodeSubagentStub(parent, &res.Ended.Items[i])
	}
}

// upsertZcodeSubagentStub 按 childSessionId 幂等登记/更新一条存根。
func (s *Server) upsertZcodeSubagentStub(parent *store.Thread, e *zcodeapp.SubagentEntry) {
	if strings.TrimSpace(e.ChildSessionID) == "" {
		return // 没有 stable id 就没有幂等键:不登记(不猜)
	}
	id := zcodeSubagentStubPrefix + e.ChildSessionID
	// 【墓碑守卫】用户删过的存根,下一次轮次终局的清单里若还在,不再建回
	// (与 codex/pi 存根同款;每次都查一次墓碑,清单量级很小,可忽略)。
	if s.cfg.Store.IsTombstoned(id) {
		return
	}
	t, exists := s.cfg.Store.Get(id)
	if !exists {
		t = &store.Thread{
			ID:             id,
			AgentID:        "zcode",
			Workspace:      parent.Workspace,
			ParentThreadID: parent.ID,
			// 【时间戳归 store】CreatedAt/UpdatedAt 由 FileStore.Create/Update 统一
			// 落 now,这里不手工赋值(赋了也会被覆盖);清单里的 startedAt/endedAt
			// 是事实,但只体现在摘要消息的 CreatedAt 上。
		}
	}
	changed := false
	// 标题:清单的原生 title 优先(0.16.9 自带),退 subagentType,再退"子代理"。
	if title := zcodeSubagentTitle(*e); title != "" && t.Title != title {
		t.Title = title
		changed = true
	}
	if st := zcodeSubagentStubStatus(*e); t.Status != st {
		t.Status = st
		changed = true
	}
	// 失败终态落 last_turn_error,成功/取消清空 —— 手机卡据此显示失败态。
	if lt := zcodeSubagentLastError(*e); t.LastTurnError != lt {
		t.LastTurnError = lt
		changed = true
	}
	// 摘要行:清单只给 summary,一条 assistant 消息承载(有则更新,无则保持
	// 原状 —— 上一次的摘要比"清空"诚实)。nil = 无新事实。
	if msgs := zcodeSubagentMessages(t.Messages, *e); msgs != nil {
		t.Messages = msgs
		changed = true
	}
	if !changed {
		return // 幂等:没有任何新事实就不写库(UpdatedAt 也由 store 保持原值)
	}
	var err error
	if exists {
		err = s.cfg.Store.Update(t)
	} else {
		err = s.cfg.Store.Create(t)
	}
	if err != nil {
		log.Printf("[subagent] zcode 子代理存根登记失败 %s: %v", id, err)
		return
	}
	log.Printf("[subagent] 已登记 zcode 子代理存根 %s(parent=%s, title=%q, status=%s)",
		id, parent.ID, t.Title, e.Status)
}

// zcodeSubagentTitle 存根标题:原生 title → subagentType → "子代理"。
func zcodeSubagentTitle(e zcodeapp.SubagentEntry) string {
	if s := strings.TrimSpace(e.Title); s != "" {
		return s
	}
	if s := strings.TrimSpace(e.SubagentType); s != "" {
		return s
	}
	return "子代理"
}

// zcodeSubagentStubStatus 状态映射(出处:zcode.cjs TYa/CYa 的构造语义,
// 2026-09-25 bundle 静态提取;未知值一律 IDLE,不猜):
//   - running/blocked → RUNNING:仍在进行的委派(blocked 是后台运行的受阻标记,
//     不是终局);
//   - waiting → WAITING_APPROVAL:zcode 把子会话 status=waiting 定义为"等用户
//     交互"(审批/提问),与手机六态的"待审批"同义;
//   - 其余(ended 页 success/failed/cancelled/lost 与未知值)→ IDLE。
func zcodeSubagentStubStatus(e zcodeapp.SubagentEntry) pb.ThreadStatus {
	switch e.Status {
	case "running", "blocked":
		return pb.ThreadStatus_THREAD_STATUS_RUNNING
	case "waiting":
		return pb.ThreadStatus_THREAD_STATUS_WAITING_APPROVAL
	default:
		return pb.ThreadStatus_THREAD_STATUS_IDLE
	}
}

// zcodeSubagentLastError 失败终态落 last_turn_error(手机端 lastTurnFailed),
// 其余状态清空。
func zcodeSubagentLastError(e zcodeapp.SubagentEntry) string {
	if e.Status == "failed" {
		return "子代理运行失败"
	}
	return ""
}

// zcodeSubagentMessages 摘要消息:summary 非空时落一条 assistant 文本
// (前缀注明来源,和 pi 存根的"[产出]"同款口径)。返回 nil = 没有新事实
// (无摘要或摘要未变),调用方保持原消息不动(幂等)。
func zcodeSubagentMessages(existing []store.StoredMessage, e zcodeapp.SubagentEntry) []store.StoredMessage {
	summary := strings.TrimSpace(e.Summary)
	if summary == "" {
		return nil
	}
	text := "[摘要] " + summary
	if len(existing) == 1 && storedMessageText(existing[0]) == text {
		return nil // 摘要没变:不重排不重写
	}
	return []store.StoredMessage{{
		Role: "assistant",
		Blocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{
			Text: &pb.TextBlock{Text: text},
		}}},
		CreatedAt: firstNonZero(e.EndedAt, e.StartedAt, 0),
	}}
}

// storedMessageText 取一条存根消息的第一段文本(无文本块返回空串)。
func storedMessageText(m store.StoredMessage) string {
	// 【vet】按索引遍历:pb.ContentBlock 内嵌 MessageState(含锁),值遍历会拷锁。
	for i := range m.Blocks {
		if t, ok := m.Blocks[i].Block.(*pb.ContentBlock_Text); ok && t.Text != nil {
			return t.Text.Text
		}
	}
	return ""
}

// firstNonZero 返回第一个非零 int64(全零返回 0,由调用方决定兜底)。
func firstNonZero(vals ...int64) int64 {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}
