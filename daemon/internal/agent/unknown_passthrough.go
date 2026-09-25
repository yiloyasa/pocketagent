// unknown_passthrough.go —— 辅线 B「未知反向请求透传确认」(README §7 的 B 开关)。
//
// 【它解决什么】四家 agent 收到不认识的反向请求时,缺省只有一条路:回 -32601 +
// 台账留痕(绝不伪造 decision,红线 2)。审计多次指出这不够:请求体里可能装着
// agent 真心想做的事,一律拒死 = "系统替用户把门焊死了"。本文件实现用户拍板的
// 权衡:**试探性未知操作由用户知情授权,而非一律拒绝** —— 开关 ON 时,未知反向
// 请求先弹一张通用确认卡(脱敏后的 method+payload 原文),用户看过原文后:
//   - 允许 → 给 agent 回空对象 {}(尽力应答,不是伪造业务结果;结果内容对 agent
//     透明 —— 它在请求一个我们不懂的 API,用户看过原文后允许应答空结果,语义是
//     "用户授权这次试探"),台账记 KNOWN_PASSTHROUGH 备查;
//   - 拒绝/超时(沿用各家 2min 预算)/审批通道缺失 → 落回既有安全拒绝路径:
//     -32601 + 台账 UNKNOWN_REQUEST,与 OFF 态逐字节一致。
//
// 【为什么默认关】这是把"机器替用户说不"换成"每一次都问用户"的实验性取舍:
// 未识别的请求可能高频出现(上游发版新增方法),全问会把手机变成确认轰炸机。
// 所以开关 OFF(默认)时行为与历史完全一致,ON 才启用 —— 风险由用户知情选择。
//
// 【范围】本轮接线 zcode 与 dsh 两家(未知请求路径最清晰);codex 的 reqUnknown
// 分支在 Run 闭包深处且涉及 requestApproval 专用形状,pi 的同类路径另行说明,
// 均留报告不接。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync/atomic"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// unknownRequestCallName 通用确认卡的动作名。手机审批抽屉按动作名分派渲染,
// 该名不落在任何已认识的动作(shell/file_change/…/反问类)里,走通用"需要你确认"
// 形状:标题/副标题 + 完整参数原文 + 理由行 + 拒绝/允许双按钮 —— 正是未知请求
// 该有的呈现:不冒充任何已知动作类型。
const unknownRequestCallName = "unknown_request"

// unknownPassthroughQuery B 开关查询点。生产实现读 runtime-policy.json
// (unknownPassthroughFromDisk);抽成变量是给测试留的桩位(行为矩阵要在同一
// 进程里钉 OFF/ON 两态,不可能每次去改数据目录)。
var unknownPassthroughQuery = unknownPassthroughFromDisk

// unknownPassthroughOn 纯函数:策略 → 开关。nil(缺席)= 关 —— 缺席绝不是
// "已授权":读不出明确的 true,就绝不能拿用户的授权当真。
func unknownPassthroughOn(p runtime.BudgetPolicy) bool {
	return p.UnknownRequestPassthrough != nil && *p.UnknownRequestPassthrough
}

// unknownPassthroughFromDisk 生产实现:读 <dataDir>/runtime-policy.json 的全局布尔。
//
// 【为什么每次现读,不缓存】SetRuntimePolicy 的生效语义是"落盘成功后立即生效",
// 磁盘文件因此永远是最近一次成功提交的真值 —— 现读 = 热改零延迟生效,免掉
// "缓存到几时失效"这类新的错误源。未知反向请求本身是低频事件(每次都会等用户
// 点卡,秒级起),一次小文件读的开销可忽略。数据目录为空(内存模式)或读取失败
// 一律按关处理:读不到真值时绝不能当成"已授权透传"。
func unknownPassthroughFromDisk() bool {
	runtimeDataDirMu.Lock()
	dir := runtimeDataDir
	runtimeDataDirMu.Unlock()
	if dir == "" {
		return false
	}
	p, err := runtime.LoadBudgetPolicy(dir)
	if err != nil {
		return false
	}
	return unknownPassthroughOn(p)
}

// buildUnknownRequestCall 构造通用确认卡:标题语义走 reason 字段,正文 = 脱敏后的
// method + payload 原文。绝不把未知请求伪装成某个已认识的动作类型 —— 用户要点着
// "允许"的东西,必须如实是"一个我们未识别的操作"。
//
// 【为什么带 reason 键】手机审批卡正文按 jsonArgs 渲染,reason 键会渲染成
// 「理由:…」一行(与 shell 卡的"理由"同一通道)—— 用户靠它分清"这是未知操作
// 试探"而不是又一次普通审批。payload 走 redactNativePayload(脱敏 + 8KiB 上限),
// 与台账同源:卡上看到的和事后查到的,是同一份原文。
func buildUnknownRequestCall(agentID, method string, params json.RawMessage) *pb.ToolCallBlock {
	args := map[string]interface{}{
		"reason": "agent 请求执行一个未识别的操作",
		"method": method,
	}
	if s := redactNativePayload(params); s != "" {
		var payload interface{}
		if json.Unmarshal([]byte(s), &payload) == nil {
			args["payload"] = payload // 保持结构化原文,不做二次加工
		} else {
			args["payload"] = s
		}
	}
	return &pb.ToolCallBlock{
		Id:       agentID + "-unknown-request",
		Name:     unknownRequestCallName,
		JsonArgs: mustJSON(args),
	}
}

// unknownPassthroughLedgerNote 允许路的台账备注:写明应答语义,事后排查时能分清
// "用户授权的试探应答"与"伪造业务结果"(后者永远不会发生,前者必须可查证)。
const unknownPassthroughLedgerNote = "用户已在手机查看脱敏原文并允许;已回空对象 {}(尽力应答,非业务结果)"

// handleDshUnknownRequest dsh 的未知反向请求透传确认(B 开关 ON 时由
// handleDshRequest 的未知分支调用)。
//
// 返回 true = 已消费(用户允许,已回 {});返回 false = 未消费,交回调用方走既有
// 安全拒绝路径(-32601 + 台账 UNKNOWN_REQUEST,与 OFF 态逐字节一致)。拒绝/超时
// 不自行回任何应答 —— 复用调用方的既有登记与应答,两类路径的台账口径天然一致。
//
// 【vet】approve 拿到的卡为指针:pb 消息含锁,按值传 hook 会被 copylocks 拒绝。
func (p *DshProvider) handleDshUnknownRequest(rsp dshClient, id int64, method string, params json.RawMessage, ctx context.Context, approve ApprovalHook, waiting *atomic.Bool) bool {
	if approve == nil {
		// 审批通道缺失(TestModel 诊断 RPC 等路径):手机从未被询问,绝不能当成
		// "用户看过并允许" —— 基础设施问题不伪装成用户意志(红线 1)。
		return false
	}
	call := buildUnknownRequestCall("dsh", method, params)
	waiting.Store(true)
	defer waiting.Store(false)
	// 沿用 dsh 审批同款 2min 预算:超时按拒绝处理(失败安全方向 —— 最坏损失是
	// "用户本想允许但没来得及点",而不是"没答应却被当成答应了")。
	askCtx, cancel := context.WithTimeout(ctx, dshApprovalTimeout)
	defer cancel()
	allow, _, askErr := approve(askCtx, call)
	switch {
	case askErr != nil && errors.Is(askErr, context.DeadlineExceeded):
		log.Printf("[dsh] 未知请求确认 %v 无应答,按拒绝处理(method=%s)", dshApprovalTimeout, method)
		return false
	case askErr != nil:
		log.Printf("[dsh] 未知请求确认通道失败,按拒绝处理(method=%s): %v", method, askErr)
		return false
	case !allow:
		log.Printf("[dsh] 用户拒绝未知请求(method=%s)", method)
		return false
	}
	// 用户看过脱敏原文后允许:回空对象 {} + 台账 KNOWN_PASSTHROUGH(语义取舍见
	// 文件头注释 —— 试探性未知操作由用户知情授权,不是伪造业务结果)。
	_ = rsp.Respond(id, map[string]interface{}{})
	sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
		SourceAgent:    "dsh",
		SourceMethod:   method,
		NormalizedKind: runtime.KindKnownPassthrough,
		RawPayload:     redactNativePayload(params),
		Note:           unknownPassthroughLedgerNote,
	})
	return true
}

// handleZcodeUnknownRequest zcode 的未知反向请求透传确认(B 开关 ON 时由 Run 的
// ReqHandler 调用;抽成方法便于注入单测,与 handleZcodePermission 同款模式)。
//
// 返回 true = 已消费(用户允许,已回 {});返回 false = 未消费,交回 Run 的既有
// 路径落台账并让 zcodeapp 客户端层统一回 -32601,与 OFF 态逐字节一致。
// budget 常态传 zcodeUIBudget(120s ≈ 2min 预算),测试注入毫秒级短超时。
func (p *ZcodeProvider) handleZcodeUnknownRequest(rawID json.RawMessage, method, threadID string, params json.RawMessage, ctx context.Context, rsp zcodeResponder, approve ApprovalHook, waiting *atomic.Bool, budget time.Duration) bool {
	if approve == nil {
		// 审批通道缺失:手机从未被询问,交回既有安全拒绝路径,不伪装用户意志。
		return false
	}
	actx, acancel := context.WithTimeout(ctx, budget)
	defer acancel()
	// askViaPhone 内部置位/复位 waiting 旗标(等手机决定是合法静默,看门狗放行)。
	allow, _, askErr := p.askViaPhone(actx, buildUnknownRequestCall("zcode", method, params), approve, waiting)
	switch {
	case askErr != nil && errors.Is(askErr, context.DeadlineExceeded):
		log.Printf("[zcode] 未知请求确认等待超时(%s),按拒绝处理(method=%s)", budget, method)
		return false
	case askErr != nil:
		log.Printf("[zcode] 未知请求确认通道失败,按拒绝处理(method=%s): %v", method, askErr)
		return false
	case !allow:
		log.Printf("[zcode] 用户拒绝未知请求(method=%s)", method)
		return false
	}
	// 用户看过脱敏原文后允许:回空对象 {}(语义取舍见文件头注释)。台账带
	// threadID,与既有 UNKNOWN_REQUEST 登记同粒度,事后能对回会话。
	_ = rsp.Respond(rawID, map[string]interface{}{})
	sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
		ThreadID:       threadID,
		SourceAgent:    "zcode",
		SourceMethod:   method,
		NormalizedKind: runtime.KindKnownPassthrough,
		RawPayload:     redactNativePayload(params),
		Note:           unknownPassthroughLedgerNote,
	})
	return true
}
