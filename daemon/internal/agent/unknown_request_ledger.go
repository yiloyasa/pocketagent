// unknown_request_ledger.go —— 四家 UNKNOWN_REQUEST 台账信封的共享出口。
//
// 【它解决什么】codex/dsh/hermes/zcode 收到不认识的反向请求时,除了回 -32601,
// 还要往诊断台账记一条 UNKNOWN_REQUEST(规格 §9:回 error 只让对端知道"这次
// 没做成","我们收到过一个不认识的请求"需要能事后查 —— 保留脱敏后的原始
// payload、关联会话)。这段信封曾是四份手写重复,Note 文案靠人肉保持一致;
// 收敛成一个出口后:字段填法只有一处,安全拒绝注记的措辞只有**一个字面量**,
// 四家台账口径结构上不可能漂移(pi 的两条注记是另一路语义 —— 协议没有 -32601
// 通道,走 cancelled:true 兜底,文案不同,刻意不并入这里)。
package agent

import (
	"encoding/json"

	"github.com/pocketagent/daemon/internal/runtime"
)

// unknownRequestRejectionNote 是"已回 -32601 安全拒绝"这一路的台账注记。
// 字面量与收敛前四家逐字一致(既有断言按子串匹配"未替用户做任何审批决定",
// 见 dsh/hermes 的 unknown_request_ledger 测试)。
const unknownRequestRejectionNote = "已回协议错误 -32601(安全拒绝;未替用户做任何审批决定)"

// recordUnknownRequestRejection 登记"收到不认识的反向请求,已回 -32601"这条
// UNKNOWN_REQUEST 台账。threadID 是我们自己的会话 id;codex/zcode 传
// req.Thread.ID,dsh/hermes 的分派函数签名里没有会话上下文,传空串 —— 与它们
// 原实现"不写 ThreadID 字段"零值等价,留空的原因(其中一个调用点在握手路径,
// 拿别的 id 冒充更糟)写在各自调用点的注释里,不在此处复述。
//
// 【谁调它】codex 的 reqUnknown 分支(且仅限真正未知的 reqUnknown)、dsh/
// hermes/zcode 的"方法名不是 session/request_permission"分支。日志、
// RespondError 的顺序与文案都在调用点,本函数只管台账。
func recordUnknownRequestRejection(threadID, agentID, method string, params json.RawMessage) {
	sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
		ThreadID:       threadID,
		SourceAgent:    agentID,
		SourceMethod:   method,
		NormalizedKind: runtime.KindUnknownRequest,
		RawPayload:     redactNativePayload(params),
		Note:           unknownRequestRejectionNote,
	})
}
