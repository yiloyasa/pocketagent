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
// 【取值表统一从 runtime.KnownKinds() 取】两处各写一份字符串列表迟早对不上,
// 那种"筛选项里有、校验却说非法"的自相矛盾最难查。
func validUnknownKind(k string) bool {
	for _, known := range runtime.KnownKinds() {
		if k == known {
			return true
		}
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
		return nil, errInvalidArg("未知的事件类别 " + k + "(可用: " + strings.Join(runtime.KnownKinds(), " / ") + ")")
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

// unknownSupportBundleHint 是 support_bundle_hint(字段 8)的固定文案,由 server 层
// 直填(2026-10-04 未知事件用户处理中心计划,主模型裁决):支持包引导不是事件的
// 语义,固定一句即可 —— 排查时维护者最需要的是技术详情里的 correlation id,
// 提示语把它指出来;对全部 kind 一视同仁(它与事件内容无关)。
const unknownSupportBundleHint = "如需进一步处理，可联系维护者并提供技术详情中的 correlation id。"

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
	return buildUnknownEventResponse(e), nil
}

// buildUnknownEventResponse 把台账条目翻成详情响应(详情 RPC 与动作执行器的
// inspect 共用同一份能力,见 unknown_actions.go):原文/字段透析/翻译/建议下一步/
// 支持包提示。两个入口必须同一份产出 —— 否则"点动作按钮看到的"和"直接拉详情
// 看到的"会互相矛盾。
// 【新字段 7/8 从这里出】recommended_next_step 来自 PresentUnknownEvent 的确定性
// 输出(词表外 kind 为空,客户端不占位);support_bundle_hint 是 server 层固定文案。
func buildUnknownEventResponse(e runtime.EventEnvelope) *pb.UnknownEventResponse {
	tr := translateEventSafely(e)
	pres := presentUnknownEventSafely(e)
	resp := &pb.UnknownEventResponse{
		Summary:             unknownSummaryToProto(e),
		RawJson:             e.RawPayload,
		DecodedFieldsJson:   e.DecodedFields,
		Note:                e.Note,
		RecommendedNextStep: pres.RecommendedNextStep,
		SupportBundleHint:   unknownSupportBundleHint,
	}
	// 字段级透析表:与 Translation 同源(读时翻译),已知事件同样有 ——
	// "daemon 已把这条投影成思考过程"配上关键字段,比方法名有用得多。
	for _, f := range tr.Fields {
		resp.Digest = append(resp.Digest, &pb.UnknownEventField{
			Key: f.Key, Kind: f.Kind, Preview: f.Preview, Meaning: f.Meaning,
		})
	}
	resp.Translation = tr.Detail
	return resp
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
// Summary 是读时人话翻译(见 runtime.TranslateEvent):已知方法翻成"daemon 对它
// 做了什么",未知方法翻成"它长什么样、像不像错误"。
// 【翻译失败宁可留空】翻译是摘要不是裁决:任何异常下都宁可不给,也不给一份
// 可能错的翻译误导人工排查(与 runtime 侧"不编造"红线同一取向)。
//
// 【新字段 12~17(用户处理中心)读时注入】presentUnknownEventSafely 调
// runtime.PresentUnknownEvent 生成确定性呈现,随 wire 返回 —— 与 Summary 同为
// 读时生成:台账里存的仍是原始事实,文案升级后老事件立刻享受新文案,不用迁移。
// 词表外的 kind(KNOWN_*/空/未来值)呈现全空,客户端按协议降级显示方法名原文。
// 事件与线程的关联仍走既有 thread_id/turn_id 通路,server 不新增字段、不虚构归属。
func unknownSummaryToProto(e runtime.EventEnvelope) *pb.UnknownEventSummary {
	pres := presentUnknownEventSafely(e)
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
		Summary:          translateEventSafely(e).Summary,
		PlainTitle:       pres.PlainTitle,
		PlainExplanation: pres.PlainExplanation,
		Impact:           pres.Impact,
		RequiresUser:     pres.RequiresUser,
		CanOpenNative:    pres.CanOpenNative,
		Actions:          unknownActionsToProto(pres.Actions),
	}
}

// unknownActionsToProto 把 runtime 的固定动作表映射成 UserActionDescriptor
// (字段一一对应,不改写不过滤)。
// 【红线沿袭】动作内容完全来自 runtime 的动作表 —— 那张表就是安全边界本身
// (unknown_action_test 逐 kind 反证它不含"允许"类动作);server 层只做逐字段
// 搬运,表里没有的动作在这里不可能长出来。执行侧另有白名单封口,见 unknown_actions.go。
func unknownActionsToProto(as []runtime.UnknownUserAction) []*pb.UserActionDescriptor {
	if len(as) == 0 {
		return nil
	}
	out := make([]*pb.UserActionDescriptor, 0, len(as))
	for _, a := range as {
		out = append(out, &pb.UserActionDescriptor{
			ActionId:             a.ID,
			Label:                a.Label,
			Description:          a.Description,
			Safe:                 a.Safe,
			RequiresConfirmation: a.RequiresConfirmation,
			Destructive:          a.Destructive,
		})
	}
	return out
}

// presentUnknownEventSafely 包装纯函数呈现:极端输入下 panic 也降级为空呈现,
// 绝不让列表/详情 RPC 因摘要失败而报错(与 translateEventSafely 同款取舍) ——
// 呈现失败只影响人话摘要,不影响事件落账与主轮次。
func presentUnknownEventSafely(e runtime.EventEnvelope) (pres runtime.UnknownPresentation) {
	defer func() {
		if recover() != nil {
			pres = runtime.UnknownPresentation{}
		}
	}()
	return runtime.PresentUnknownEvent(e)
}

// translateEventSafely 包装纯函数翻译:极端输入下 panic 也降级为空翻译,
// 绝不让诊断 RPC 因翻译失败而报错 —— 台账查询是诊断路径,不是业务路径。
func translateEventSafely(e runtime.EventEnvelope) (tr runtime.Translation) {
	defer func() {
		if recover() != nil {
			tr = runtime.Translation{}
		}
	}()
	return runtime.TranslateEvent(e.SourceAgent, e.SourceMethod, e.NormalizedKind, e.RawPayload)
}
