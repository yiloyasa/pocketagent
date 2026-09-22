// unknown_events.go —— 未知事件诊断索引的 RPC 面(规格 §9)。
//
// 【它补齐什么】未知通知其实早就在透传(agent 侧 nativeNoticeRecorder 会把它包成
// 诊断事件推给手机),未知反向请求也早就如实回协议错误。缺的是**索引**:发生过什么、
// 频率如何、处置到哪一步,这些以前只留在 daemon 日志里 —— 用户在手机上看不到,
// 也就无法判断"要不要等上游支持""这个实验特性是不是一直在发怪东西"。
//
// 【错误语义】类别/状态取值非法 → InvalidArgument 并列出可用值;取不到 id → NotFound。
// 不做"静默返回空"——那会让客户端把"你写错了参数"和"确实没有记录"混为一谈。
package server

import (
	"context"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// unknownLog 返回诊断台账;没有运行时后端(测试/极简构造)时返回 nil。
func (s *Server) unknownLog() *runtime.UnknownEventLog {
	if s.runtimeBackend == nil {
		return nil
	}
	return s.runtimeBackend.UnknownEvents()
}

// validUnknownKind 校验 normalized_kind 取值。
func validUnknownKind(k string) bool {
	switch k {
	case runtime.KindUnknownNotification, runtime.KindUnknownRequest, runtime.KindUnknownResponse:
		return true
	}
	return false
}

// ListUnknownEvents 列出未知事件(最新在前)。
func (s *Server) ListUnknownEvents(_ context.Context, req *pb.ListUnknownEventsRequest) (*pb.ListUnknownEventsResponse, error) {
	log := s.unknownLog()
	if log == nil {
		// 没有台账时回空清单:"没有记录"与"没有台账"对客户端看到的东西一样(空),
		// 而报错会让诊断页把正常空态显示成故障。
		return &pb.ListUnknownEventsResponse{}, nil
	}
	if k := req.GetKind(); k != "" && !validUnknownKind(k) {
		return nil, errInvalidArg("未知的事件类别 " + k + "(可用: " + strings.Join([]string{
			runtime.KindUnknownNotification, runtime.KindUnknownRequest, runtime.KindUnknownResponse,
		}, " / ") + ")")
	}
	if st := req.GetProcessingStatus(); st != "" && !runtime.ValidProcessingStatus(st) {
		return nil, errInvalidArg("未知的处置状态 " + st + "(可用: " + strings.Join(runtime.KnownProcessingStatuses(), " / ") + ")")
	}
	items := log.List(runtime.ListFilter{
		Kind:     req.GetKind(),
		AgentID:  req.GetAgentId(),
		ThreadID: req.GetThreadId(),
		Status:   req.GetProcessingStatus(),
	}, int(req.GetLimit()))
	resp := &pb.ListUnknownEventsResponse{}
	for _, e := range items {
		resp.Items = append(resp.Items, unknownSummaryToProto(e))
	}
	resp.Total, resp.Dropped = log.Stats()
	return resp, nil
}

// GetUnknownEvent 取单条详情(含脱敏后的原始载荷)。
func (s *Server) GetUnknownEvent(_ context.Context, req *pb.GetUnknownEventRequest) (*pb.UnknownEventResponse, error) {
	log := s.unknownLog()
	if log == nil {
		return nil, errNotFound("未知事件诊断台账未启用")
	}
	if req.GetEventId() == "" {
		return nil, errInvalidArg("缺少 event_id")
	}
	e, ok := log.Get(req.GetEventId())
	if !ok {
		// 与"文件不存在"同款区分:找不到就该说找不到,而不是回一条空记录让客户端
		// 显示成"事件存在但没有内容"。
		return nil, errNotFound("找不到该未知事件(可能已被更早的记录挤出保留窗口)")
	}
	return &pb.UnknownEventResponse{
		Summary:           unknownSummaryToProto(e),
		RawJson:           e.RawPayload,
		DecodedFieldsJson: e.DecodedFields,
		Note:              e.Note,
	}, nil
}

// AcknowledgeUnknownEvent 记录处置动作。
func (s *Server) AcknowledgeUnknownEvent(_ context.Context, req *pb.AcknowledgeUnknownEventRequest) (*pb.AcknowledgeUnknownEventResponse, error) {
	log := s.unknownLog()
	if log == nil {
		return nil, errNotFound("未知事件诊断台账未启用")
	}
	if req.GetEventId() == "" {
		return nil, errInvalidArg("缺少 event_id")
	}
	// 【为什么不给"默认动作"】处置状态是用户意志的表达;补一个默认值等于替用户表态。
	if req.GetAction() == "" {
		return nil, errInvalidArg("缺少 action(可用: " + strings.Join(runtime.KnownProcessingStatuses(), " / ") + ")")
	}
	item, ok, err := log.Acknowledge(req.GetEventId(), req.GetAction(), req.GetNote())
	if err != nil {
		return nil, errInvalidArg(err.Error())
	}
	if !ok {
		return nil, errNotFound("找不到该未知事件(可能已被更早的记录挤出保留窗口)")
	}
	return &pb.AcknowledgeUnknownEventResponse{Item: unknownSummaryToProto(item)}, nil
}

// unknownSummaryToProto 把台账条目翻成契约形态(列表与详情共用同一映射)。
func unknownSummaryToProto(e runtime.EventEnvelope) *pb.UnknownEventSummary {
	return &pb.UnknownEventSummary{
		EventId:          e.ID,
		Sequence:         e.Sequence,
		ReceivedAtUnixMs: e.TimestampMs,
		SourceAgent:      e.SourceAgent,
		SourceMethod:     e.SourceMethod,
		ThreadId:         e.ThreadID,
		NormalizedKind:   e.NormalizedKind,
		SafetyClass:      e.SafetyClass,
		ProcessingStatus: e.ProcessingStatus,
		Revision:         e.Revision,
	}
}
