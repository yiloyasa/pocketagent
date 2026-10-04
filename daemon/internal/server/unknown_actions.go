// unknown_actions.go —— 未知事件用户动作的安全执行器(2026-10-04 未知事件用户
// 处理中心计划 Task 6)。
//
// 【它解决什么】动作表(runtime/unknown_action.go)声明了哪些动作"可以给用户看";
// 本文件是按下按钮之后真正做事的一侧。四个白名单动作全部是"读/标记/指引"级:
//
//	inspect      查看脱敏技术原文(复用 GetUnknownEvent 的同一能力,不改任何状态)
//	acknowledge  标记已查看(复用台账 Acknowledge → INSPECTED)
//	retry_later  标记稍后再看(→ RETRY_LATER)
//	open_native  标记为已转交(→ ESCALATED)+ 返回电脑端处理指引
//
// copy 不在白名单:复制是手机端本地行为(剪贴板在手机上),发往电脑端没有意义,
// 执行器对它单独给一句人话说明,而不是笼统的"不认识"。
//
// 【安全红线,逐条落在代码上】
//   - 白名单封口:除四个动作外,任何 action_id(包括客户端伪造的 approve/execute/
//     allow)一律 InvalidArgument —— 展示层动作表是一道卡,这里是执行层第二道卡,
//     两道各自设防,穿掉一层还有一层;
//   - 幂等:先过 revision CAS 再谈幂等(2026-10-04 门禁裁定)—— 版本对得上、且
//     事件已处于目标状态时,短路返回当前状态(不报错、不改 revision);带着旧
//     revision 的重放回冲突,stale 客户端不能拿"幂等"当通行证;
//   - revision CAS:动作请求必须带期望 revision;不匹配回结构化冲突结果(不是
//     error —— 冲突是并发处理的正常业务态),人话说明 + 当前最新摘要一次带回,
//     客户端刷新即恢复,不必再发一次详情请求;
//   - 历史留痕:每次执行(成功/幂等/冲突/拒绝)都写独立处置历史,可审计。
//
// 【RPC 形态说明】现有 proto 没有"执行用户动作"的 RPC,AcknowledgeUnknownEvent
// 的请求消息(event_id/action/note)也不携带 revision —— 它的 action 词表是处置
// 状态(NEW/INSPECTED/...),与动作 ID(inspect/...)是两套口径,承载不了本契约。
// proto 冻结期不得自行改,故本文件先以 server 层 API 形态交付
// (ExecuteUnknownEventAction),steward 补 RPC 后由 pb 请求一比一映射接入
// (字段口径:event_id / expected_revision / action_id / note)。
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// 服务层动作 ID(与 runtime 动作表逐一对应,下划线口径;客户端原样回传)。
const (
	unknownActionInspect     = "inspect"
	unknownActionAcknowledge = "acknowledge"
	unknownActionRetryLater  = "retry_later"
	unknownActionOpenNative  = "open_native"
	// unknownActionCopyID 在展示层动作表里存在(safe=true),但它是手机本地动作,
	// 不进本执行器的白名单 —— 单列常量是为了给一句专门的人话说明。
	unknownActionCopyID = "copy"
)

// 处置历史的结果词表。
const (
	unknownActionResultOK         = "OK"         // 动作已执行
	unknownActionResultIdempotent = "IDEMPOTENT" // 重复提交,事件已是目标状态
	unknownActionResultConflict   = "CONFLICT"   // revision 不匹配,未执行
	unknownActionResultRejected   = "REJECTED"   // 动作不在白名单,拒绝
)

// unknownActionTarget 动作 ID → 目标处置状态;"" = 纯读取动作(不改状态)。
// 第二个返回值 false = 不在白名单(执行层拒绝,与展示层动作表互为两道卡)。
func unknownActionTarget(actionID string) (string, bool) {
	switch actionID {
	case unknownActionInspect:
		return "", true
	case unknownActionAcknowledge:
		return runtime.StatusInspected, true
	case unknownActionRetryLater:
		return runtime.StatusRetryLater, true
	case unknownActionOpenNative:
		return runtime.StatusEscalated, true
	}
	return "", false
}

// 【动作请求/响应 = wire 类型】(steward 已落 proto,见 proto/v1/runtime.proto)
// 请求 pb.UnknownEventActionRequest{event_id, expected_revision, action_id, note};
// 响应 pb.UnknownEventActionResult{ok, conflict, detail, summary, technical,
// new_revision}。字段语义随 proto 注释,本文件只留两层实现共同依赖的裁定:
//
// 【身份绑定】event_id + revision 唯一确定"用户当时看到的那一版事件":后到的动作
// 不可能作用在另一版上而不被发现 —— 全局约束"绑定 identity"的最小完备形态。
//
// 【revision 类型差异与转换语义】台账 revision 语义是 uint64(runtime.EventEnvelope
// 的字段,从 0 起单调 +1),wire 层 expected_revision / new_revision 是 int64
// (而 UnknownEventSummary.revision 又是 uint64 —— wire 内部两处口径就不一致,
// 类型差异只能收敛在本方法入口/出口两处)。64 位平台上真实 revision 远小于
// 2^63,显式转换无损:入口做 uint64(req.GetExpectedRevision())、出口做
// int64(newRevision)(集中在 unknownActionResult),不依赖隐式转换。
//   - ExpectedRevision < 0:病态输入,拒绝(InvalidArgument)。不静默按 0 处理:
//     负数只能来自客户端 bug 或伪造,按 0 处理等于把"发错了"伪装成"还在初始
//     版本",静默吞掉比明说更危险。
//   - ExpectedRevision == 0:proto 口径"0 = 不做 CAS(仅 inspect 这类纯读动作
//     允许)"。落地为:inspect 这类纯读动作不做 CAS(无副作用,宽松无风险,详情
//     页不必先拉 revision);状态型动作仍严格比较 —— 0 同时是合法版本号(新事件
//     就是 0),对正确客户端行为零分叉,对病态客户端(拿 0 去改已确认过的事件)
//     回冲突 = 失败安全方向。
//
// 【为什么冲突不是 error】revision 不匹配说明另一端(另一台设备/更早的重试)先
// 处理了 —— 这是并发使用的正常业务态,不是调用方写错了参数;结果消息把人话说明
// 与"刷新所需的当前摘要/new_revision"一次带回。回个裸错误,客户端还得再发一次
// GetUnknownEvent,弱网下等于罚用户两次往返。

// unknownActionMu 序列化"读快照 → 校验 → 写"临界区。
//
// 【为什么是进程级一把锁】动作执行是用户点按钮级别的低频路径,进程级一把锁的
// 争用可以忽略;换来的确定性是实打实的:同一事件的两个并发动作必然一先一后进
// 临界区,后到者看到的是先到者提交后的状态(幂等短路或 revision 冲突),不会出现
// 两次都基于旧快照各写一遍。历史记账也在锁内,账本顺序与执行顺序一致。
//
// 【残余窗口,如实声明】既有 AcknowledgeUnknownEvent RPC 不经此锁(它在台账自己
// 的锁里"后写赢"),与新执行器并发时仍可能交错 —— 最终状态一致,revision 如实
// 多计一次。要彻底关闭需把 CAS 下沉进台账(runtime 冻结区,归后续任务裁决)。
var unknownActionMu sync.Mutex

// ExecuteUnknownEventAction 执行一个未知事件用户动作(gRPC 方法体,语义见文件头)。
// 与包内其余未用 context 的 RPC 同款,ctx 以 _ 占位。
func (s *Server) ExecuteUnknownEventAction(_ context.Context, req *pb.UnknownEventActionRequest) (*pb.UnknownEventActionResult, error) {
	log := s.unknownLog()
	if log == nil {
		return nil, errNotFound("未知事件诊断台账未启用")
	}
	if req.GetEventId() == "" {
		return nil, errInvalidArg("缺少 event_id")
	}
	// 【病态输入】expected_revision < 0 拒绝而非按 0:负数只能来自客户端 bug 或
	// 伪造;按 0 处理会把"发错了"伪装成"还在初始版本",且 uint64(负数) 会变成
	// 天文数字,冲突结果里的版本号没法读(取舍论证见上方注释块)。
	if req.GetExpectedRevision() < 0 {
		return nil, errInvalidArg("expected_revision 不能为负数")
	}
	// 台账 revision 语义 uint64 ← wire int64,显式转换(无损性论证见上方注释块)。
	expected := uint64(req.GetExpectedRevision())
	actionID := strings.TrimSpace(req.GetActionId())
	if actionID == "" {
		return nil, errInvalidArg("缺少 action_id(可执行: " + unknownActionChoices() + ")")
	}
	target, known := unknownActionTarget(actionID)
	if !known {
		if actionID == unknownActionCopyID {
			return nil, errInvalidArg("复制在手机端本地完成(详情页可直接复制技术原文),不需要发往电脑端")
		}
		s.recordUnknownActionHistory(req.GetEventId(), actionID, unknownActionResultRejected,
			"拒绝不在白名单里的动作")
		return nil, errInvalidArg("不认识的动作 " + actionID + "(可执行: " + unknownActionChoices() + ")")
	}

	unknownActionMu.Lock()
	defer unknownActionMu.Unlock()

	e, ok := log.Get(req.GetEventId())
	if !ok {
		return nil, errNotFound("找不到该未知事件(可能已被更早的记录挤出保留窗口)")
	}

	// revision CAS 先于幂等短路(2026-10-04 门禁裁定):客户端必须先证明它看到的
	// 是哪一版 —— 版本对了、且事件已处于目标态,才算幂等;stale 客户端不能基于
	// 旧视图拿"幂等"当通行证,版本错一律回冲突,让人先刷新再决策。
	// 【口径】状态型动作严格比较(0 是新事件的合法版本号,照常参与比较);
	// inspect 这类纯读动作 expected=0 时不做比较(无副作用,详情页不必先拉
	// revision),非 0 时照常比较 —— 见上方注释块。
	casEnforced := target != "" || expected != 0
	if casEnforced && expected != e.Revision {
		s.recordUnknownActionHistory(e.ID, actionID, unknownActionResultConflict,
			fmt.Sprintf("revision 不匹配(期望 %d,实际 %d),未执行", req.GetExpectedRevision(), e.Revision))
		// 冲突不是 error:new_revision 带回当前值,客户端拿它做下一次 expected_revision。
		return unknownActionResult(false, true,
			"这条事件的状态已发生变化，请刷新后重试。",
			unknownSummaryToProto(e), nil, e.Revision), nil
	}

	// 幂等短路(CAS 之后):版本对得上、且事件已处于目标态,重复提交返回当前状态
	// 而非报错 —— 重复点击、多端一致的重复全部收敛到同一结果。ok=true:幂等也算成功。
	if target != "" && e.ProcessingStatus == target {
		detail := "这条事件已经是这个状态，无需重复处理。"
		if actionID == unknownActionOpenNative {
			detail += unknownOpenNativeHint(e)
		}
		s.recordUnknownActionHistory(e.ID, actionID, unknownActionResultIdempotent,
			"重复提交,事件已是目标状态")
		return unknownActionResult(true, false, detail, unknownSummaryToProto(e), nil, e.Revision), nil
	}

	if target == "" {
		// inspect:纯读取,回脱敏技术原文,不碰状态与 revision(天然幂等)。
		s.recordUnknownActionHistory(e.ID, actionID, unknownActionResultOK, "查看脱敏技术原文")
		return unknownActionResult(true, false,
			"这是这条事件的脱敏技术原文，可以直接复制。",
			unknownSummaryToProto(e), buildUnknownEventResponse(e), e.Revision), nil
	}

	// 状态型动作:复用台账 Acknowledge(状态词表校验/revision 自增/落盘都在那层)。
	item, ok, err := log.Acknowledge(e.ID, target, req.GetNote())
	if err != nil {
		return nil, errInvalidArg(err.Error())
	}
	if !ok {
		return nil, errNotFound("找不到该未知事件(可能已被更早的记录挤出保留窗口)")
	}
	detail := unknownActionAppliedDetail(actionID, e)
	s.recordUnknownActionHistory(e.ID, actionID, unknownActionResultOK, detail)
	return unknownActionResult(true, false, detail, unknownSummaryToProto(item), nil, item.Revision), nil
}

// unknownActionResult 组装 pb 结果消息:ok/conflict 两个布尔集中在此赋值,免得
// 散落各分支出现"Ok 与 Conflict 语义漂移"。newRevision 在台账侧是 uint64,出口
// 显式转 wire 的 int64(64 位平台上真实 revision 远小于 2^63,无损);语义:动作后
// 的 revision,inspect/冲突/幂等时为当前值。technical 仅 inspect 返回(其余 nil)。
func unknownActionResult(ok, conflict bool, detail string, summary *pb.UnknownEventSummary, technical *pb.UnknownEventResponse, newRevision uint64) *pb.UnknownEventActionResult {
	return &pb.UnknownEventActionResult{
		Ok:          ok,
		Conflict:    conflict,
		Detail:      detail,
		Summary:     summary,
		Technical:   technical,
		NewRevision: int64(newRevision),
	}
}

// unknownActionChoices 人话动作清单(错误信息用,与白名单同源 —— 手写一份迟早漏)。
func unknownActionChoices() string {
	return unknownActionInspect + " / " + unknownActionAcknowledge + " / " +
		unknownActionRetryLater + " / " + unknownActionOpenNative
}

// unknownActionAppliedDetail 状态型动作成功后的人话说明(以动作前的快照取上下文)。
func unknownActionAppliedDetail(actionID string, before runtime.EventEnvelope) string {
	switch actionID {
	case unknownActionAcknowledge:
		return "已把这条事件标记为已查看。"
	case unknownActionRetryLater:
		return "已把这条事件标记为稍后再看，它会留在列表里，不会打扰你。"
	case unknownActionOpenNative:
		return "已把这条事件标记为已转交。" + unknownOpenNativeHint(before)
	}
	return ""
}

// unknownOpenNativeHint 电脑端处理指引。动作语义是"转交",不是"代执行" ——
// 手机端永远不代电脑端做它没看过的决定,指引把用户准确送到能处理的地方;
// correlation id 是能在两端的日志里对齐同一次请求的唯一线索,有就带上。
func unknownOpenNativeHint(e runtime.EventEnvelope) string {
	who := e.SourceAgent
	if who == "" {
		who = "对应 agent"
	}
	hint := "请在电脑端打开 " + who + " 的原生界面处理该请求，本端不代为执行。"
	if e.CorrelationID != "" {
		hint += "技术详情中的 correlation id（" + e.CorrelationID + "）可用于在电脑端对齐这次请求。"
	}
	return hint
}

// ============ 处置历史小账本 ============
//
// 【为什么独立成账,不挂在事件信封上】runtime.UnknownEventLog 在本轮冻结区(只读),
// 往 EventEnvelope 追加历史字段越权;而且分开各有好处:事件信封保持"当前状态"
// 语义(列表/详情要快、要小),历史账保持"逐次动作"语义(审计要全)—— 各司其职。
// 【先例与取舍】文件形状学同目录的 unknown-events.json(带 seq 头的全量快照 +
// 原子改名 + 超限丢最旧),不学 JSONL 追加:动作是点按钮级低频事件,量极小,
// 全量重写没有性能问题,却免掉了 JSONL 的截断/回放逻辑,损坏面更小。
// 【fail-soft】历史写不进去不影响动作本身的返回(动作已在台账生效),只留告警 ——
// 与 unknown-events 同款取舍;dataDir 为空退化为纯内存(与 eventstore/identity
// /unknown-events 同款约定),行为零分叉。

// unknownActionHistoryFileName 处置历史账本的落盘文件名(与 unknown-events.json
// 同目录,一对兄弟账)。
const unknownActionHistoryFileName = "unknown-action-history.json"

// unknownActionHistoryLimit 历史保留条数上限(动作比事件更稀少,500 条足够覆盖
// "最近出过什么事";超限丢最旧 —— 审计窗口诚实地上限化,不无声装满磁盘)。
const unknownActionHistoryLimit = 500

// UnknownActionRecord 是一次动作执行的历史条目(event_id/动作/时间/结果)。
type UnknownActionRecord struct {
	// Sequence 账本内单调序号(从 1 起)。
	Sequence int64 `json:"sequence"`
	// TimestampMs 执行时刻(unix 毫秒)。
	TimestampMs int64 `json:"timestamp_ms"`
	// EventID 被操作的事件;拒绝路径可能带客户端给的任意串,如实记录不清洗。
	EventID string `json:"event_id"`
	// ActionID 动作 ID(含被拒绝的非法值)。
	ActionID string `json:"action_id"`
	// Result 取 unknownActionResult* 之一。
	Result string `json:"result"`
	// Detail 是结果的人话摘要(做了什么/冲突原因/拒绝原因)。
	Detail string `json:"detail,omitempty"`
}

// unknownActionHistory 处置历史账本。
type unknownActionHistory struct {
	mu      sync.Mutex
	path    string // <dataDir>/unknown-action-history.json;空 = 仅内存
	seq     int64
	entries []UnknownActionRecord
}

// unknownActionHistoryFile 是落盘信封(带 seq 头,序号跨重启不断档)。
type unknownActionHistoryFile struct {
	Seq     int64                 `json:"seq"`
	Entries []UnknownActionRecord `json:"entries"`
}

// 账本实例按 dataDir 复用:Server 没有为它单开字段(本轮不动 server.go 的结构体,
// 该文件不归本任务改),进程内同目录共享一本 —— 与台账"进程级"语义一致。
var (
	unknownActionHistoryMu    sync.Mutex
	unknownActionHistoryByDir = map[string]*unknownActionHistory{}
)

// unknownActionHistoryFor 取(或建)dataDir 对应的账本实例。
func unknownActionHistoryFor(dataDir string) *unknownActionHistory {
	unknownActionHistoryMu.Lock()
	defer unknownActionHistoryMu.Unlock()
	if h, ok := unknownActionHistoryByDir[dataDir]; ok {
		return h
	}
	h := newUnknownActionHistory(dataDir)
	unknownActionHistoryByDir[dataDir] = h
	return h
}

// newUnknownActionHistory 打开(或新建)账本;损坏时告警 + 空账继续,原文件保留
// 供人工检查(与 unknown-events.json 同款 fail-soft)。
func newUnknownActionHistory(dataDir string) *unknownActionHistory {
	h := &unknownActionHistory{}
	if dataDir != "" {
		h.path = filepath.Join(dataDir, unknownActionHistoryFileName)
	}
	if h.path == "" {
		return h
	}
	raw, err := os.ReadFile(h.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[unknown] WARNING: 处置历史账本读取失败,本次按空账继续(原文件保留): %v", err)
		}
		return h
	}
	var f unknownActionHistoryFile
	if err := json.Unmarshal(raw, &f); err != nil {
		log.Printf("[unknown] WARNING: 处置历史账本损坏(JSON 解析失败),已按空账继续(原文件保留在 %s 供人工检查): %v", h.path, err)
		return h
	}
	h.seq, h.entries = f.Seq, f.Entries
	return h
}

// record 追加一条历史(超限丢最旧)并落盘。写失败只告警:历史是审计增强,
// 不是动作生效的前提(动作已在台账生效)。
func (h *unknownActionHistory) record(eventID, actionID, result, detail string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	h.entries = append(h.entries, UnknownActionRecord{
		Sequence:    h.seq,
		TimestampMs: time.Now().UnixMilli(),
		EventID:     eventID,
		ActionID:    actionID,
		Result:      result,
		Detail:      detail,
	})
	for len(h.entries) > unknownActionHistoryLimit {
		h.entries = h.entries[1:]
	}
	h.flushLocked()
}

// snapshot 返回历史副本(最新在前,与事件台账同口径)。
func (h *unknownActionHistory) snapshot() []UnknownActionRecord {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]UnknownActionRecord, 0, len(h.entries))
	for i := len(h.entries) - 1; i >= 0; i-- {
		out = append(out, h.entries[i])
	}
	return out
}

// flushLocked 原子落盘(临时文件 + rename),与 unknown-events.json 同款纪律。
func (h *unknownActionHistory) flushLocked() {
	if h.path == "" {
		return
	}
	entries := h.entries
	if entries == nil {
		entries = []UnknownActionRecord{} // 空账写 "[]" 而不是 null
	}
	raw, err := json.MarshalIndent(unknownActionHistoryFile{Seq: h.seq, Entries: entries}, "", "  ")
	if err != nil {
		log.Printf("[unknown] WARNING: 处置历史序列化失败(仅影响审计可见性): %v", err)
		return
	}
	tmp := h.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		log.Printf("[unknown] WARNING: 处置历史写入失败(仅影响审计可见性): %v", err)
		return
	}
	if err := os.Rename(tmp, h.path); err != nil {
		log.Printf("[unknown] WARNING: 处置历史落位失败(仅影响审计可见性): %v", err)
	}
}

// unknownActionHistoryLedger 返回本 Server 数据目录对应的账本(测试与将来的
// 审计查询入口用;生产动作路径统一走 recordUnknownActionHistory)。
func (s *Server) unknownActionHistoryLedger() *unknownActionHistory {
	return unknownActionHistoryFor(s.cfg.DataDir)
}

// recordUnknownActionHistory 是 Server 侧的记账入口(按 cfg.DataDir 定位账本)。
func (s *Server) recordUnknownActionHistory(eventID, actionID, result, detail string) {
	unknownActionHistoryFor(s.cfg.DataDir).record(eventID, actionID, result, detail)
}
