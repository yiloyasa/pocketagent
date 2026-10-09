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
// 【范围】本轮接线 zcode 与 dsh(未知请求路径最清晰)、codex(reqUnknown 分支
// 复用 requestApproval 的 approve 回路与 serverRequest/resolved 竞速治理,见
// consumeCodexUnknownRequest)、pi(extension_ui_request 的未知 method,见
// consumePiUnknownUIRequest —— pi 的应答通道只有 extension_ui_response payload,
// 拒绝路用协议原生的 {"cancelled":true} 兜底,没有 -32601 错误码形状)。
//
// 【2026-10-09 波 A 收编】四段 handler 原是"一份骨架 × 四份手写接线"(approve
// 缺失检查 → 2min 预算 → 三分支日志 → 允许回 {} + KNOWN_PASSTHROUGH 台账/拒绝
// 回落,逐字重复四遍)。共享骨架抽成 askUnknownApproval:通道检查、预算、
// waiting 旗标、三分支日志、允许路的应答出口与台账出口全部收进骨架;四家只剩
// 各自的应答形状闭包(Respond / Respond / RespondUI)与协议专有面(codex 的
// resolved 竞速、pi 的 cancelled:true 拒绝尾)。
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

// unknownRequestSafeRejectionFact —— UNKNOWN_REQUEST「被安全拒绝(回协议错误)」
// 这一事实的承载说明(2026-10-04 未知事件用户处理中心计划 Task 3 追加,零行为变更)。
//
// 【事实在哪、由谁承载】B 开关关/用户拒绝/超时/通道故障时,本文件的各透传入口
// 返回 false,交回调用方的既有安全拒绝路径:dsh.go / zcode.go / codex.go 的未知
// 请求分支回 -32601 并把 UNKNOWN_REQUEST 写进台账(pi 无 -32601 形状,走
// cancelled:true 兜底,见 consumePiUnknownUIRequest 拒绝路)。台账条目
// (EventEnvelope,normalized_kind=UNKNOWN_REQUEST)就是这一事实的既有载体:
// id/序号/时间/方法/线程/脱敏原文/处置状态俱全。server 层读出时经
// runtime.PresentUnknownEvent 生成"没有自动执行"的人话呈现,随 UnknownEventSummary
// 新字段(12~17)下发 —— "被安全拒绝"对用户可见的形态由此而来,不再需要新结构。
//
// 【为什么本文件不追加承载字段】"拒绝"是这些函数的返回 false,不是新状态;再造
// 一个字段等于第二份真值。逐次过程有既有日志(log.Printf),终局有台账留痕 ——
// 两处已足够,冻结结构零改动。若未来需要"拒绝原因细分"(超时 vs 用户拒绝),
// 应走台账 note 或 steward 侧协议字段,不在 provider 结构体上就地生长。

// errUnknownRequestResolved codex 专有哨兵:等手机期间桌面端已解决该请求
// (serverRequest/resolved 竞速)。askUnknownApproval 据此把请求按"已消费"终结
// —— 不再应答、不落台账(收尾在 codex 的 approve 闭包内完成,见
// consumeCodexUnknownRequest)。
var errUnknownRequestResolved = errors.New("unknown request already resolved on desktop")

// askUnknownApproval 未知反向请求透传确认的共享骨架(dsh/zcode/codex/pi 四家
// 接线,2026-10-09 波 A 从四份手写重复收编成一份)。
//
// 阶段与出口:
//  1. approve == nil → 审批通道缺失(TestModel 诊断 RPC 等路径):手机从未被
//     询问,绝不能当成"用户看过并允许" —— 基础设施问题不伪装成用户意志
//     (红线 1)。返回 (false, false):未问询、未消费,交回调用方的既有安全
//     拒绝路径;
//  2. budget 内问手机(waiting 旗标置位/复位:等手机决定是合法静默,看门狗放行;
//     pi 无此旗标传 nil;budget=0 表示不另设预算 —— codex 的 server 侧 approve
//     自带 2min,旧实现即直透 ctx);
//  3. 三分支日志,均按拒绝处理(失败安全方向 —— 最坏损失是"用户本想允许但没
//     来得及点",而不是"没答应却被当成答应了"),返回 (true, false):
//     问过了但未消费,交回调用方既有路径(dsh/zcode/codex 的 -32601 +
//     UNKNOWN_REQUEST 台账由调用方落,与 OFF 态走同一段代码,口径天然一致;
//     pi 的 cancelled:true 尾在 consumePiUnknownUIRequest);
//  4. 用户允许 → respondAllow 闭包回空对象 {}(尽力应答,不是伪造业务结果;
//     应答形状差异 Respond/RespondUI 收进闭包)+ 台账 KNOWN_PASSTHROUGH
//     (允许路台账出口收进本骨架,四处同款:SourceAgent/SourceMethod/脱敏载荷/
//     应答语义备注;带会话归属的三家与不带的 dsh 走同一份字段,空串即无归属),
//     返回 (true, true)。
//
// 【codex 的专有出口】其 approve 闭包内含 serverRequest/resolved 竞速治理,桌面端
// 已解决时返回 errUnknownRequestResolved 哨兵:本骨架按 (true, true) 终结 ——
// 请求已按专有路径收尾(撤卡、注销登记),不得再应答、不落台账。
func askUnknownApproval(ctx context.Context, agentID, threadID, method string, params json.RawMessage,
	approve ApprovalHook, waiting *atomic.Bool, budget time.Duration, respondAllow func()) (asked, consumed bool) {

	if approve == nil {
		return false, false
	}
	call := buildUnknownRequestCall(agentID, method, params)
	if waiting != nil {
		waiting.Store(true)
		defer waiting.Store(false)
	}
	actx := ctx
	if budget > 0 {
		var cancel context.CancelFunc
		actx, cancel = context.WithTimeout(ctx, budget)
		defer cancel()
	}
	allow, _, askErr := approve(actx, call)
	switch {
	case askErr != nil && errors.Is(askErr, errUnknownRequestResolved):
		return true, true
	case askErr != nil && budget > 0 && errors.Is(askErr, context.DeadlineExceeded):
		// 预算内无应答:按拒绝处理。budget=0(codex)不设此分支 —— server 侧
		// approve 自带 2min 预算,DeadlineExceeded 到这里与通道失败同观感,保持
		// 其旧实现的单一"通道失败"日志口径。
		log.Printf("[%s] 未知请求确认 %v 无应答,按拒绝处理(method=%s)", agentID, budget, method)
		return true, false
	case askErr != nil:
		log.Printf("[%s] 未知请求确认通道失败,按拒绝处理(method=%s): %v", agentID, method, askErr)
		return true, false
	case !allow:
		log.Printf("[%s] 用户拒绝未知请求(method=%s)", agentID, method)
		return true, false
	}
	// 用户看过脱敏原文后允许(语义取舍见文件头注释 —— 试探性未知操作由用户知情
	// 授权,不是伪造业务结果):先回空对象 {},再落 KNOWN_PASSTHROUGH 台账备查。
	respondAllow()
	sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
		ThreadID:       threadID,
		SourceAgent:    agentID,
		SourceMethod:   method,
		NormalizedKind: runtime.KindKnownPassthrough,
		RawPayload:     redactNativePayload(params),
		Note:           unknownPassthroughLedgerNote,
	})
	return true, true
}

// handleDshUnknownRequest dsh 的未知反向请求透传确认(B 开关 ON 时由
// handleDshRequest 的未知分支调用)。骨架(通道检查/dshApprovalTimeout 预算/
// 三分支日志/允许路应答+台账)在 askUnknownApproval,此处只接 dsh 的应答形状
// (Respond 回空对象)。
//
// 返回 true = 已消费(用户允许,已回 {});返回 false = 未消费,交回调用方走既有
// 安全拒绝路径(-32601 + 台账 UNKNOWN_REQUEST,与 OFF 态逐字节一致)。拒绝/超时
// 不自行回任何应答 —— 复用调用方的既有登记与应答,两类路径的台账口径天然一致。
//
// 【vet】approve 拿到的卡为指针:pb 消息含锁,按值传 hook 会被 copylocks 拒绝。
func (p *DshProvider) handleDshUnknownRequest(rsp dshClient, id int64, method string, params json.RawMessage, ctx context.Context, approve ApprovalHook, waiting *atomic.Bool) bool {
	_, consumed := askUnknownApproval(ctx, "dsh", "", method, params, approve, waiting, dshApprovalTimeout,
		func() { _ = rsp.Respond(id, map[string]interface{}{}) })
	return consumed
}

// handleZcodeUnknownRequest zcode 的未知反向请求透传确认(B 开关 ON 时由 Run 的
// ReqHandler 调用;抽成方法便于注入单测,与 handleZcodePermission 同款模式)。
// 骨架在 askUnknownApproval,此处只接 zcode 的应答形状(Respond 原样回显 rawID)。
//
// 返回 true = 已消费(用户允许,已回 {});返回 false = 未消费,交回 Run 的既有
// 路径落台账并让 zcodeapp 客户端层统一回 -32601,与 OFF 态逐字节一致。
// budget 常态传 zcodeUIBudget(120s ≈ 2min 预算),测试注入毫秒级短超时。
func (p *ZcodeProvider) handleZcodeUnknownRequest(rawID json.RawMessage, method, threadID string, params json.RawMessage, ctx context.Context, rsp zcodeResponder, approve ApprovalHook, waiting *atomic.Bool, budget time.Duration) bool {
	_, consumed := askUnknownApproval(ctx, "zcode", threadID, method, params, approve, waiting, budget,
		func() { _ = rsp.Respond(rawID, map[string]interface{}{}) })
	return consumed
}

// consumeCodexUnknownRequest codex 的未知反向请求透传确认(B 开关 ON 时由 Run 的
// reqHandler 的 reqUnknown 分支调用)。
//
// 【应答通道复用】codex 未知请求与 requestApproval 共用同一条 approve/deny 回路:
// 构造一张 unknown_request 确认卡走既有审批钩子(server 侧同一 2min 预算,见
// thread.go 的 approve 实现),确认卡经既有审批卡通道渲染(理由行含「未识别的
// 操作」+ 脱敏 payload 原文)—— 审批语义红线不动:不伪造 decision,用户没答应
// 就没有答应。
//
// 【允许路的应答形状:空对象 {}】codex-schema 的 ServerRequest 是闭合枚举,未知
// 方法没有任何应答 schema;JSONRPCResponse 只约束 {id, result},result 不设约束
// (`"result": true` 即任意 JSON 值)。所以 {} 是协议合法的最小诚实应答 —— 尽力
// 应答,不是伪造业务结果(语义取舍见文件头注释)。
//
// 【serverRequest/resolved 竞速(与 handleCodexApproval 同款治理)】等手机期间
// 桌面端可能已解决该请求:approve 闭包(askWithResolvedRace)里 goroutine 问
// 手机并与 resolved 信号 select,桌面端先赢时撤卡(sink 发 ApprovalResolved,
// 幽灵审批卡治理)、注销登记、返回 errUnknownRequestResolved 哨兵 —— 骨架据此
// 终结请求(已消费),此刻再回 {} 或 -32601 都是重复应答协议噪声,也不落台账。
//
// 返回 false = 未消费(开关关/审批通道缺失/拒绝/超时/通道故障),调用方交回
// reqUnknown 分支的既有路径:-32601 + UNKNOWN_REQUEST 台账,与 OFF 态走同一段
// 代码,台账口径天然一致。
func (p *CodexProvider) consumeCodexUnknownRequest(rsp codexApprovalResponder, id json.RawMessage, method, threadID string, params json.RawMessage, ctx context.Context, sink Sink, approve ApprovalHook, waiting *atomic.Bool) bool {
	if !unknownPassthroughQuery() {
		return false
	}
	// askWithResolvedRace:骨架的 approve 面 + codex 专有的桌面端解决竞速。
	askWithResolvedRace := func(actx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		resolved := p.registerPendingServerRequest(id)
		type approveOut struct {
			allow bool
			note  string
			err   error
		}
		outCh := make(chan approveOut, 1)
		go func() {
			a, n, e := approve(actx, call)
			outCh <- approveOut{a, n, e}
		}()
		select {
		case out := <-outCh:
			p.unregisterPendingServerRequest(id)
			return out.allow, out.note, out.err
		case <-resolved:
			// 迟到的手机应答落进带缓冲的 outCh,由 GC 回收,无泄漏。
			log.Printf("[codex] 未知请求 %s(id=%s)已在桌面端被解决,不再等待手机确认", method, codexRequestIDKey(id))
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ApprovalResolved{ApprovalResolved: &pb.ApprovalResolved{
				Note: "这条审批已在电脑端被处理,手机上的审批卡已同步关闭。",
			}}})
			p.unregisterPendingServerRequest(id)
			return false, "", errUnknownRequestResolved
		}
	}
	// budget=0:server 侧 approve 自带 2min 预算,骨架不另设(旧实现即直透 ctx,
	// 超时/断流按"通道失败"留痕,口径不变)。
	_, consumed := askUnknownApproval(ctx, "codex", threadID, method, params, askWithResolvedRace, waiting, 0,
		func() { _ = rsp.Respond(id, map[string]interface{}{}) })
	return consumed
}

// piUIRequestKnown 判定一个 extension_ui_request 的 method 是否**已知**。
//
// 已知面两块(piapp.Client 文档 + 探针实测帧,docs/verify/evidence/pi-p0-probe.md):
//   - dialog 四种:select/confirm/input/editor —— Run 的 UIRequestHandler 专线
//     处理并回写,事件流里再见到必须放过(否则同一请求弹两张卡);
//   - fire-and-forget 五种:notify/setStatus/setWidget/setTitle/set_editor_text
//     —— 无应答通道,对它们回写 extension_ui_response 只会制造协议噪声。
//
// 不在这两块里的 method = 未知反向请求(上游发版新增的 UI 交互),才进透传确认。
func piUIRequestKnown(method string) bool {
	switch method {
	case "select", "confirm", "input", "editor":
		return true
	case "notify", "setStatus", "setWidget", "setTitle", "set_editor_text":
		return true
	}
	return false
}

// piUIRequestKnownFrame 判定一帧 extension_ui_request 原文是否已知 method
// (in-flight 闸前置判别用,与 consumePiUnknownUIRequest 的帧内判别同词表)。
// 解析失败/缺 method = 未知 —— 坏帧不在此处猜,照旧走透传确认入口的既有
// 处置(consume 里会再解一次并按未知静默),两处判据同源不分叉。
func piUIRequestKnownFrame(raw json.RawMessage) bool {
	var probe struct {
		Method string `json:"method"`
	}
	if json.Unmarshal(raw, &probe) != nil {
		return false
	}
	return piUIRequestKnown(probe.Method)
}

// piUIClient 未知请求回写面对 piapp.Client 的窄接口(单测打桩,不依赖子进程;
// RespondUI 真客户端天然满足,生产零适配 —— 与 codexApprovalResponder 同款模式)。
type piUIClient interface {
	RespondUI(id string, payload map[string]any) error
}

// consumePiUnknownUIRequest pi 的未知反向请求透传确认(B 开关 ON 时由 Run 的
// consume 事件泵的 extension_ui_request 分支异步调用 —— 事件泵绝不能被等手机的
// 2min 阻塞,调用方负责 go 出去,与 piapp dispatch 对 dialog 的并发纪律同款)。
//
// 【pi 与 dsh/zcode 的形状差异,如实声明】pi 的反向请求没有 JSON-RPC 应答通道:
// extension_ui_response 只有 payload,没有 -32601 错误码形状。所以:
//   - 允许 → 回写空 payload {}(尽力应答,语义同 dsh/zcode 的空对象)+ 台账
//     KNOWN_PASSTHROUGH(允许路出口在 askUnknownApproval 骨架内);
//   - 拒绝/超时 → 回写协议原生兜底拒绝 {"cancelled":true}(piapp 的失败安全
//     同款:dialog 无 timeout 会永久挂起,answered cancelled 是唯一能"放走"
//     等待方的安全拒绝)+ 台账 UNKNOWN_REQUEST,备注写明走了 cancelled 而非
//     -32601,事后排查不误判 —— 该尾留在本函数:pi 没有"交回调用方回 -32601"
//     的既有路径(OFF 态是静默),骨架的未消费出口接不住它。
//
// 返回 true = 已消费(已回写);返回 false = 未消费(开关关/已知 method/id 缺失/
// 审批通道缺失),调用方维持原样静默 —— OFF 态与历史逐字节一致(pi 的未知请求
// 历史行为就是静默忽略,没有可回落的应答分支)。
func (p *PiProvider) consumePiUnknownUIRequest(c piUIClient, raw json.RawMessage, threadID string, ctx context.Context, approve ApprovalHook) bool {
	if !unknownPassthroughQuery() {
		return false
	}
	var frame struct {
		ID     string `json:"id"`
		Method string `json:"method"`
	}
	if json.Unmarshal(raw, &frame) != nil || frame.ID == "" || piUIRequestKnown(frame.Method) {
		// id 缺失无法回写;已知 method 归各自专线。都交回调用方原样静默。
		return false
	}
	asked, consumed := askUnknownApproval(ctx, "pi", threadID, frame.Method, raw, approve, nil, piUIBudget,
		func() { _ = c.RespondUI(frame.ID, map[string]any{}) })
	if !asked {
		// 审批通道缺失:手机从未被询问,交回原样静默,不伪装用户意志(红线 1)。
		return false
	}
	if consumed {
		return true // 允许路:骨架已回 {} + 记 KNOWN_PASSTHROUGH
	}
	// 拒绝/超时/通道故障:回写 pi 协议原生兜底拒绝(没有 -32601 可用,见函数注释)
	// + 台账 UNKNOWN_REQUEST。台账必须落:pi 对未知请求的 OFF 态是静默,若这里也
	// 静默,"拒绝过一次未知请求"就无据可查。
	_ = c.RespondUI(frame.ID, map[string]any{"cancelled": true})
	sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
		ThreadID:       threadID,
		SourceAgent:    "pi",
		SourceMethod:   frame.Method,
		NormalizedKind: runtime.KindUnknownRequest,
		RawPayload:     redactNativePayload(raw),
		Note:           "已回协议原生兜底拒绝 cancelled:true(pi 的 extension_ui_response 无 -32601 错误码通道;未替用户做任何审批决定)",
	})
	return true
}
