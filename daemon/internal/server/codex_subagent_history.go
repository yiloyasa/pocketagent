// codex_subagent_history.go —— codex 子代理存根的对话内容回填(T-SUBVIS)。
//
// 【缺口在哪】T-S1P 只登记存根(id/父链/昵称),存根 Messages 恒空 —— 手机端
// 内联卡与子对话页因此只有标题没有内容。T-SUBVIS 捕获实证(codex-subagent-
// spawn-capture.md):子线程的对话内容不走任何通知(item/* 带子线程 id,轮次
// 管线按线程过滤全丢;thread/updated 不存在),唯一通道是 thread/read
// {includeTurns:true} 拉全量。
//
// 【接线点(三处,全走本文件的 maybePullCodexSubagentHistory)】
//  1. GetThreadHistory 打开子对话页时(懒拉,与 maybePullZcodeImportedHistory
//     同款先例):存根 Messages 为空才拉,拉到即缓存;
//  2. 存根登记后异步首拉(codex_subagent_sync.go 登记 → go 首拉):让内联卡的
//     预览行(SubAgentsState.requestPreview)在用户点开前就有内容;
//  3. 子线程转 idle(thread/status/changed 经既有 SyncThreadEvent 命中存根):
//     子代理跑完,全量已成定局,此时重拉一次刷新。
//
// 【F9 存量自愈扩容(2026-10-09)】接线点 1 的入口从"子代理存根"放宽到
// "0 消息的普通 codex 线程":旧 rollout 解析器只认 event_msg,新格式
// (codex 0.160.0)静默解析出 0 条,ImportCodexThread 落了 0 消息线程;
// ReadThreadFull(thread/read 权威全量)天然兼容新格式,打开历史页即自愈,
// 落库后与既有本地路径汇合。合并规则与守卫原样沿用。
//
// 【合并规则(宁可少显示,不覆盖/不回退用户看得见的历史)】
// thread/read 拉回的条数 ≥ 本地条数 → 本地整体替换(rollout 是权威副本,
// 包含用户在子对话页插的话 —— StartTurn 走 chat/{childThreadId} 真线程);
// 拉回条数 < 本地 → 放弃本次,保留本地(拉取时点早于本地某次落库的竞态,
// 保守面)。空结果不缓存,保留下次再拉的机会(zcode 同款)。
//
// 【诚实边界】turn 里的 reasoning/subAgentActivity 等 item 原生没有可读正文,
// 不落库、不编造;落库的命令执行带 "[命令]" 前缀原样转写 —— 是展示标注,
// 不是改写内容(README §7:透传,认识的写清楚,不认识的造不出来)。
package server

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// subagentOutputClipRunes 子代理落库时单条命令输出的字符上限。
//
// 【为什么要有上限】thread/read 的命令执行 item 带 aggregatedOutput，一个
// 构建/测试命令的输出轻松上万行；原样落库会把 threads.json 撑大、还让子对话
// 页一屏全是日志。截断的是"落库副本"，不是原件——原始快照仍在 codex 自己的
// rollout 里，用户随时能用桌面端看全量。
const subagentOutputClipRunes = 2000

// clipRunesOr 按 rune 截断(中文安全),超出补省略号;n<=0 视为不设限。
//
// 与 thread.go 的 truncateTitle 同语义但用途不同:那个面向"标题"(短、要求
// 一行),这个面向"命令输出片段"(长、允许换行)。后者截在 rune 边界上,不会把
// 一个中文字劈成半个字节序列。
func clipRunesOr(s string, n int) string {
	if n <= 0 {
		return s // 原语义:n<=0 视为不设限(fallback),helper 的空串语义不适用,故守卫留在本层
	}
	// 统一走 runtime.ClipRunesEllipsis,收敛见审查报告 util 节。
	return runtime.ClipRunesEllipsis(s, n)
}

// subagentHistoryBudget 单次 thread/read 全量拉取的整段上界(与 zcode 拉历史
// 同级;zcodeHistoryBudget=20s 先例)。thread/read 可能带 codex 冷启动,取同值。
const subagentHistoryBudget = 20 * time.Second

// subagentHistoryInflight 同一线程的在飞拉取守卫(防止打开页面 + idle 通知 +
// 登记首拉三路并发打三次 thread/read)。键 = 本地线程 id。
var subagentHistoryInflight sync.Map

// subagentReadFuller 内容回填需要的最小 provider 面(*agent.CodexProvider 满足;
// 抽接口为了单测注 fake,与 zcodeHistorySource 同模式)。
type subagentReadFuller interface {
	ReadThreadFull(ctx context.Context, codexThreadID string) (*agent.CodexThreadSnapshot, error)
}

// maybePullCodexSubagentHistory 按文件头三处接线点调用:是 codex 子代理存根、
// 没有在飞拉取、provider 可用时,拉 thread/read 全量并按合并规则落库。
// 其余情况一律静默返回(存根是"有则显示"的增强,绝不打扰主流程)。
//
// 【F9 存量自愈(2026-10-09)】旧解析器只认 event_msg,新格式 rollout(codex
// 0.160.0)静默解析出 0 条 → ImportCodexThread 落了 0 消息普通线程。放宽入口:
// 普通 codex 线程(agent=codex、ParentThreadID 为空)且本地 0 消息时也走本函数
// 懒拉 —— ReadThreadFull 是权威全量通道,天然兼容新格式;既有守卫全部保留
// (在飞单飞闸、空结果不缓存、拉回少于本地不覆盖、本地轮次在飞不拉)。
func (s *Server) maybePullCodexSubagentHistory(ctx context.Context, t *store.Thread) bool {
	if t == nil || threadAgentID(t) != "codex" || t.CodexThreadID == "" {
		return false // 不是 codex 线程:与既有路径无关
	}
	// 子代理存根(父链非空)无条件可拉;普通线程只做 0 消息自愈首拉。
	if t.ParentThreadID == "" && len(t.Messages) > 0 {
		return false
	}
	if s.activeTurns.Has(t.ID) {
		return false // 本地轮次在飞(用户正在插话):不拉,防覆盖流式
	}
	if _, busy := subagentHistoryInflight.LoadOrStore(t.ID, struct{}{}); busy {
		return false
	}
	defer subagentHistoryInflight.Delete(t.ID)

	p, ok := s.cfg.Providers["codex"].(subagentReadFuller)
	if !ok {
		return false // 测试/无 codex 环境
	}
	// 双检:拿锁期间可能已被先到者拉完/被轮次更新。
	cur, found := s.cfg.Store.Get(t.ID)
	if !found {
		return false
	}
	if cur.ParentThreadID != "" {
		// 子代理存根:首拉只认"空存根";非空存根只在线程转 idle 的刷新路径进来
		// (subagentRefreshWanted 由调用方语境决定,这里保守不复拉)。
		if len(cur.Messages) > 0 && !s.subagentRefreshWanted(cur) {
			return false
		}
	} else if len(cur.Messages) > 0 {
		// 普通 codex 线程:仅限 0 消息自愈;拉过一次(或有本地轮次)一律不进。
		return false
	}
	hctx, cancel := context.WithTimeout(ctx, subagentHistoryBudget)
	defer cancel()
	snap, err := p.ReadThreadFull(hctx, cur.CodexThreadID)
	if err != nil {
		log.Printf("[subagent] 拉取对话失败(降级为空)thread=%s codex=%s: %v",
			cur.ID, cur.CodexThreadID, err)
		return false
	}
	msgs := subagentSnapshotMessages(snap)
	if len(msgs) == 0 {
		return false // 空结果不缓存:保留下次打开再试的机会
	}
	if len(msgs) < len(cur.Messages) {
		log.Printf("[subagent] 拉回 %d 条少于本地 %d 条,放弃覆盖 thread=%s",
			len(msgs), len(cur.Messages), cur.ID)
		return false
	}
	cur.Messages = msgs
	if err := s.cfg.Store.Update(cur); err != nil {
		log.Printf("[subagent] 对话落盘失败(内存可见,重启重拉)thread=%s: %v", cur.ID, err)
		return false
	}
	kind := "子代理对话"
	if cur.ParentThreadID == "" {
		kind = "普通线程历史(0 消息自愈)"
	}
	log.Printf("[subagent] %s已回填 %d 条 thread=%s(codex=%s)", kind, len(msgs), cur.ID, cur.CodexThreadID)
	return true
}

// subagentRefreshWanted 判断非空存根是否需要刷新:当前唯一语境是"子线程转
// idle"的通知路径(SyncThreadEvent 直呼的 refresh 入口),打开页面的懒拉路径
// 只认空存根。独立成谓词便于单测与后续扩展。
func (s *Server) subagentRefreshWanted(t *store.Thread) bool { return false }

// RefreshCodexSubagentHistory 子线程转 idle 时的刷新入口(全量已成定局):
// 无条件重拉一次(不要求空存根),合并规则同上。供 SyncThreadEvent 调。
func (s *Server) RefreshCodexSubagentHistory(codexThreadID string) {
	t := s.findLocalThreadByCodexID(codexThreadID)
	if t == nil || threadAgentID(t) != "codex" || t.CodexThreadID == "" || t.ParentThreadID == "" {
		return
	}
	if s.activeTurns.Has(t.ID) {
		return
	}
	if _, busy := subagentHistoryInflight.LoadOrStore(t.ID, struct{}{}); busy {
		return
	}
	defer subagentHistoryInflight.Delete(t.ID)
	p, ok := s.cfg.Providers["codex"].(subagentReadFuller)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), subagentHistoryBudget)
	defer cancel()
	snap, err := p.ReadThreadFull(ctx, t.CodexThreadID)
	if err != nil {
		log.Printf("[subagent] idle 回填子代理对话失败 thread=%s codex=%s: %v", t.ID, t.CodexThreadID, err)
		return
	}
	msgs := subagentSnapshotMessages(snap)
	if len(msgs) == 0 || len(msgs) < len(t.Messages) {
		return // 空结果不落库;少于本地(竞态)不覆盖
	}
	t.Messages = msgs
	if err := s.cfg.Store.Update(t); err != nil {
		log.Printf("[subagent] idle 回填落盘失败 thread=%s: %v", t.ID, err)
		return
	}
	log.Printf("[subagent] 子代理完成,idle 回填 %d 条 thread=%s(codex=%s)", len(msgs), t.ID, t.CodexThreadID)
}

// subagentSnapshotMessages 把 thread/read 快照翻成本地消息:只落有正文的 item
// (userMessage/agentMessage/命令执行/协作调用摘要),按 turn 顺序。
func subagentSnapshotMessages(snap *agent.CodexThreadSnapshot) []store.StoredMessage {
	if snap == nil {
		return nil
	}
	var out []store.StoredMessage
	for _, turn := range snap.Turns {
		for _, it := range turn.Items {
			switch {
			case it.Role() != "":
				text := strings.TrimSpace(it.Text)
				if text == "" {
					continue
				}
				out = append(out, store.StoredMessage{
					Role:   it.Role(),
					Blocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}}}},
				})
			case it.Type == "commandExecution":
				var b strings.Builder
				b.WriteString("[命令] " + it.Command)
				if it.ExitCode != nil {
					b.WriteString(fmt.Sprintf("\n(退出码 %d)", *it.ExitCode))
				}
				if o := strings.TrimSpace(it.Output); o != "" {
					b.WriteString("\n" + clipRunesOr(o, subagentOutputClipRunes))
				}
				out = append(out, store.StoredMessage{
					Role:   "assistant",
					Blocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: b.String()}}}},
				})
			case it.Type == "collabAgentToolCall" && len(it.ReceiverThreadIDs) > 0:
				// 子代理自己又派了孙代理:如实一句摘要 + 线程 id 透传,不展开。
				out = append(out, store.StoredMessage{
					Role: "assistant",
					Blocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{
						Text: "[协作调用 " + it.CollabTool + " · " + it.CollabStatus + "] " + strings.Join(it.ReceiverThreadIDs, ", "),
					}}}},
				})
			}
			// 其余类型(reasoning/subAgentActivity/webSearch/…):原生无正文或
			// 纯活动刻度,不落库(诚实边界,不造对话)。
		}
	}
	return out
}
