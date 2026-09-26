// event_sequence.go —— P1-5 事件断点续传的 server 侧接线。
//
// 职责三件事:
//  1. recordTurnFrame:StartTurn 发给手机的每一帧在发送前分配全局单调 seq
//     并落入 eventstore(数据底座),手机断线后凭 after_sequence 增量补帧;
//  2. GetThreadEventsSince:按线程回放 seq > after 的全部帧(升序),
//     超出保留窗口时先发一帧 ResyncRequired 让手机端走全量回退;
//  3. 主机事件(HostEvent)同样入 eventstore 带 seq,WatchEvents 支持
//     after_sequence 补发 + 实时广播。
package server

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/eventstore"
	"github.com/pocketagent/daemon/internal/stderrdrain"
)

// EventTypeEncodingFailed 帧/主机事件序列化失败时落的诊断事件类型名。
//
// 【为什么需要它】原实现在 protojson.Marshal 失败时把载荷写成 `{}`,等于把
// "这一帧到底发生了什么"从事件库里抹掉 —— 断线回放、未知事件诊断、终局识别
// 全靠这份载荷,而现场只剩 daemon 日志里一行,用户侧无从解释。
// 现在失败帧不再伪装成正常帧:明明白白记一条诊断,并带上抢救出来的原文。
const EventTypeEncodingFailed = "EVENT_ENCODING_FAILED"

// maxSalvageBytes 抢救载荷的字节上限。诊断要留线索,但不能让一帧坏数据把
// events.jsonl 撑爆(全局单文件,轮转阈值 10MB)。
const maxSalvageBytes = 2048

// marshalEventPayload 把一帧序列化成事件库载荷。
//
// 返回 (payload, diagType, salvage):
//   - 正常或规范化救回:diagType == "",salvage == ""
//   - 救不回:payload == nil,diagType == EventTypeEncodingFailed,salvage = 抢救原文
//
// 【为什么先做 UTF-8 规范化重试】protojson 对生成类型唯一的真实失败原因是字符串
// 含非法 UTF-8(protobuf 的 appendString 遇 RuneError 直接返回 errInvalidUTF8)。
// 这类字节多半来自被截断的多字节字符(子进程输出/终端回显),替换成 U+FFFD 就能
// 把**整帧**救回来 —— 保真度远高于丢帧或只留诊断。
func marshalEventPayload(ev proto.Message) (payload []byte, diagType, salvage string) {
	if p, err := protojson.Marshal(ev); err == nil {
		return p, "", ""
	}
	if clone := proto.Clone(ev); clone != nil {
		normalizeUTF8(clone.ProtoReflect())
		if p, err := protojson.Marshal(clone); err == nil {
			return p, "", ""
		}
	}
	// 兜底:prototext 把非法字节转义输出,不因 UTF-8 失败 —— 拿它当抢救通道。
	return nil, EventTypeEncodingFailed, prototext.Format(ev)
}

// normalizeUTF8 递归把消息里所有字符串字段的非法 UTF-8 序列替换成 U+FFFD。
//
// 【为什么必须深度遍历】只处理顶层会漏掉正文嵌套在 ContentBlock / 结构体里的
// 情形 —— 而那正是坏字节最常见的来源(agent 输出被塞进 block)。漏掉的话重试
// 仍会失败,退化成诊断事件(能用,但不是设计意图)。
func normalizeUTF8(m protoreflect.Message) {
	// 【为什么先收集再改】Range 期间对同一消息做结构性修改,语义没有保证。
	// 收集完再 Set,把"遍历"与"写入"分开。
	type fieldFix struct {
		fd protoreflect.FieldDescriptor
		v  protoreflect.Value
	}
	var fixes []fieldFix
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
			if fd.MapValue().Kind() == protoreflect.StringKind {
				mp := v.Map()
				type mapFix struct {
					k protoreflect.MapKey
					s string
				}
				var mfix []mapFix
				mp.Range(func(k protoreflect.MapKey, mv protoreflect.Value) bool {
					if s := mv.String(); !utf8.ValidString(s) {
						mfix = append(mfix, mapFix{k, strings.ToValidUTF8(s, "\uFFFD")})
					}
					return true
				})
				for _, f := range mfix {
					mp.Set(f.k, protoreflect.ValueOfString(f.s))
				}
			}
		case fd.IsList():
			list := v.List()
			for i := 0; i < list.Len(); i++ {
				switch fd.Kind() {
				case protoreflect.StringKind:
					if s := list.Get(i).String(); !utf8.ValidString(s) {
						list.Set(i, protoreflect.ValueOfString(strings.ToValidUTF8(s, "\uFFFD")))
					}
				case protoreflect.MessageKind:
					normalizeUTF8(list.Get(i).Message())
				}
			}
		case fd.Kind() == protoreflect.StringKind:
			if s := v.String(); !utf8.ValidString(s) {
				fixes = append(fixes, fieldFix{fd, protoreflect.ValueOfString(strings.ToValidUTF8(s, "\uFFFD"))})
			}
		case fd.Kind() == protoreflect.MessageKind:
			normalizeUTF8(v.Message())
		}
		return true
	})
	for _, f := range fixes {
		m.Set(f.fd, f.v)
	}
}

// diagnosticPayload 造一条诊断事件的**合法 JSON**载荷。
//
// 【为什么不能直接把原消息再 Marshal 一次】那会用同样的坏字节再失败一次 ——
// 诊断行本身必须是我们自己拼的合法 JSON,这是它存在的意义。
// 【为什么先脱敏后截断】顺序反了会把截断处的密钥前缀留在盘上,反而制造泄漏。
func diagnosticPayload(eventType, salvage string) []byte {
	body := map[string]string{
		"error":      EventTypeEncodingFailed,
		"event_type": eventType,
		"salvaged":   boundedRedacted(salvage),
	}
	b, err := json.Marshal(body)
	if err != nil {
		// map[string]string 的 Marshal 不会失败;真发生了也绝不能返回空载荷
		// —— 那等于回到"丢事件"的老问题。
		return []byte(`{"error":"` + EventTypeEncodingFailed + `"}`)
	}
	return b
}

// boundedRedacted 脱敏 + 限长。超长时保留头部(密钥通常在前面刚出现的键值段里,
// 坏字节通常在尾部截断处),并标注被丢掉的字节数,让线索可追溯。
func boundedRedacted(s string) string {
	s = stderrdrain.Redact(s)
	if len(s) <= maxSalvageBytes {
		return s
	}
	return s[:maxSalvageBytes] + "…[已截断 " + strconv.Itoa(len(s)-maxSalvageBytes) + " 字节]"
}

// ---------- 1) TurnServerMessage 帧记账 ----------

// recordTurnFrame 给一帧分配 seq 并写入事件库,返回该 seq(写盘失败也返回
// 内存序号 —— 帧序号对手机端单调这件事比单帧落盘更不能破)。
//
// 【为什么在发送锁内调用】StartTurn 的 sink 有并发发送方(codex 事件分发 /
// 审批 / 终局),seq 分配与磁盘写入顺序必须与发送顺序一致,否则回放的帧序
// 会和手机已收到的顺序打架。调用方负责持有 sendMu。
func (s *Server) recordTurnFrame(threadID, turnID string, ev *pb.TurnServerMessage) int64 {
	eventType := "turn." + turnFrameType(ev)
	payload, diagType, salvage := marshalEventPayload(ev)
	if diagType != "" {
		// 【为什么不写 {}】见 EventTypeEncodingFailed。诊断行用 diag. 前缀,
		// 保证 ReplayTurnFrames 不会把它当成正常帧回放给手机。
		log.Printf("[eventseq] 帧序列化失败 type=%s,改记诊断事件并保留原文", eventType)
		return s.events.Append(threadID, turnID,
			eventstore.TypePrefixDiag+diagType, diagnosticPayload(eventType, salvage))
	}
	return s.events.Append(threadID, turnID, eventType, payload)
}

// turnFrameType 把 oneof 分支名作为事件类型(turn.<type>)。
func turnFrameType(ev *pb.TurnServerMessage) string {
	switch ev.Msg.(type) {
	case *pb.TurnServerMessage_Started:
		return "started"
	case *pb.TurnServerMessage_TextDelta:
		return "text_delta"
	case *pb.TurnServerMessage_ThinkingDelta:
		return "thinking_delta"
	case *pb.TurnServerMessage_ToolCall:
		return "tool_call"
	case *pb.TurnServerMessage_ApprovalRequest:
		return "approval_request"
	case *pb.TurnServerMessage_Usage:
		return "usage"
	case *pb.TurnServerMessage_Completed:
		return "completed"
	case *pb.TurnServerMessage_Failed:
		return "failed"
	case *pb.TurnServerMessage_CodexEvent:
		return "codex_event"
	case *pb.TurnServerMessage_ApprovalResolved:
		return "approval_resolved"
	case *pb.TurnServerMessage_Steered:
		return "steered"
	case *pb.TurnServerMessage_Queued:
		return "queued"
	case *pb.TurnServerMessage_QueueFlushed:
		return "queue_flushed"
	default:
		return "unknown"
	}
}

// ---------- 2) GetThreadEventsSince:增量回放 ----------

// GetThreadEventsSince 把该线程 seq > after_sequence 的全部帧按升序回放给
// 手机,放完即关流(不做 live 跟随 —— 恢复后手机会重新开正常的 StartTurn)。
//
// 两种特殊出口:
//   - after 已落入轮转丢弃区间(或事件库无法证明连续):只发一帧 ResyncRequired,
//     手机端收到后必须退回 GetThreadHistory 全量回填 —— 宁可重拉也不拼错;
//   - after 之后没有新帧:空流直接关(手机端语义 = "已接上")。
func (s *Server) GetThreadEventsSince(req *pb.GetThreadEventsSinceRequest, stream pb.ThreadService_GetThreadEventsSinceServer) error {
	if req.GetThreadId() == "" {
		return errInvalidArg("缺少 thread_id")
	}
	after := req.GetAfterSequence()
	if s.events.BeyondRetention(after) {
		return stream.Send(&pb.TurnServerMessage{
			Msg: &pb.TurnServerMessage_ResyncRequired{ResyncRequired: &pb.ResyncRequired{
				Reason: "请求的区间已被事件轮转丢弃,请全量回退后重新记录 sequence",
			}},
		})
	}
	for _, e := range s.events.ReplayTurnFrames(req.GetThreadId(), after) {
		var msg pb.TurnServerMessage
		if err := protojson.Unmarshal([]byte(e.PayloadJSON), &msg); err != nil {
			// 单帧损坏:跳过。连续性缺口由手机端容忍(帧内 sequence 可对照)。
			log.Printf("[eventseq] 回放帧 seq=%d 反序列化失败,跳过: %v", e.Seq, err)
			continue
		}
		// sequence 以落盘记录为准(载荷里没存 seq,防止双写不一致)
		msg.Sequence = e.Seq
		if err := stream.Send(&msg); err != nil {
			return err
		}
	}
	return nil
}

// ---------- 3) HostEvent 记账 + WatchEvents 广播 ----------

// hostWatchers 活跃的 WatchEvents 流注册表(每流一个带缓冲通道)。
type hostWatchers struct {
	mu   sync.Mutex
	next int64
	m    map[int64]chan *pb.HostEvent
}

func newHostWatchers() *hostWatchers {
	return &hostWatchers{m: map[int64]chan *pb.HostEvent{}}
}

func (h *hostWatchers) add() (int64, chan *pb.HostEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next++
	ch := make(chan *pb.HostEvent, 16)
	h.m[h.next] = ch
	return h.next, ch
}

func (h *hostWatchers) remove(id int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.m, id)
}

func (h *hostWatchers) snapshot() []chan *pb.HostEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]chan *pb.HostEvent, 0, len(h.m))
	for _, ch := range h.m {
		out = append(out, ch)
	}
	return out
}

// emitHostEvent 给主机事件分配 seq、落盘,并广播给所有活跃的 WatchEvents 流。
// 单个订阅者发不进(满/慢)就跳过该订阅者 —— 主机事件是"通知"不是"事实",
// 手机端断线后的补齐走 WatchEvents 的 after_sequence 回放。
func (s *Server) emitHostEvent(ev *pb.HostEvent) {
	if ev == nil {
		return
	}
	eventType := "host." + hostEventType(ev)
	payload, diagType, salvage := marshalEventPayload(ev)
	if diagType != "" {
		log.Printf("[eventseq] 主机事件序列化失败 type=%s,改记诊断事件并保留原文", eventType)
		payload = diagnosticPayload(eventType, salvage)
		eventType = eventstore.TypePrefixDiag + diagType
	}
	threadID := ""
	if tf := ev.GetTurnFinished(); tf != nil {
		threadID = tf.GetThreadId()
	}
	ev.Sequence = s.events.Append(threadID, "", eventType, payload)
	for _, ch := range s.hostWatch.snapshot() {
		select {
		case ch <- ev:
		default:
		}
	}
}

// hostEventType 主机事件的类型名(host.<type>)。
// 2026-09-26:workspace_changed/model_ready/update_available 三分支随契约删除
// (自基线起就无生产点、app 零消费,审计 R6/S2),此处分支一并移除。
func hostEventType(ev *pb.HostEvent) string {
	switch ev.Event.(type) {
	case *pb.HostEvent_TurnFinished:
		return "turn_finished"
	default:
		return "unknown"
	}
}

// emitTurnFinished 轮次终局后发主机事件(P1-5 的第一个真实生产者:
// 后台推送场景 —— 手机不在 StartTurn 流上也能感知"跑完了")。
func (s *Server) emitTurnFinished(threadID, summary string) {
	s.emitHostEvent(&pb.HostEvent{
		TimestampUnixMs: time.Now().UnixMilli(),
		Event: &pb.HostEvent_TurnFinished{TurnFinished: &pb.TurnFinishedNotification{
			ThreadId: threadID, Summary: summary,
		}},
	})
}
