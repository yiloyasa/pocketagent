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
	"log"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// ---------- 1) TurnServerMessage 帧记账 ----------

// recordTurnFrame 给一帧分配 seq 并写入事件库,返回该 seq(写盘失败也返回
// 内存序号 —— 帧序号对手机端单调这件事比单帧落盘更不能破)。
//
// 【为什么在发送锁内调用】StartTurn 的 sink 有并发发送方(codex 事件分发 /
// 审批 / 终局),seq 分配与磁盘写入顺序必须与发送顺序一致,否则回放的帧序
// 会和手机已收到的顺序打架。调用方负责持有 sendMu。
func (s *Server) recordTurnFrame(threadID, turnID string, ev *pb.TurnServerMessage) int64 {
	payload, err := protojson.Marshal(ev)
	if err != nil {
		// protojson 对合法生成类型不该失败;真发生了也记一行空载荷,
		// 保住 seq 连续性(seq 在 Append 内独立递增,不受载荷影响)。
		log.Printf("[eventseq] 帧序列化失败 type=%s: %v", turnFrameType(ev), err)
		payload = []byte("{}")
	}
	return s.events.Append(threadID, turnID, "turn."+turnFrameType(ev), payload)
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
	payload, err := protojson.Marshal(ev)
	if err != nil {
		log.Printf("[eventseq] 主机事件序列化失败: %v", err)
		payload = []byte("{}")
	}
	threadID := ""
	if tf := ev.GetTurnFinished(); tf != nil {
		threadID = tf.GetThreadId()
	}
	ev.Sequence = s.events.Append(threadID, "", "host."+hostEventType(ev), payload)
	for _, ch := range s.hostWatch.snapshot() {
		select {
		case ch <- ev:
		default:
		}
	}
}

// hostEventType 主机事件的类型名(host.<type>)。
func hostEventType(ev *pb.HostEvent) string {
	switch ev.Event.(type) {
	case *pb.HostEvent_WorkspaceChanged:
		return "workspace_changed"
	case *pb.HostEvent_ModelReady:
		return "model_ready"
	case *pb.HostEvent_UpdateAvailable:
		return "update_available"
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
