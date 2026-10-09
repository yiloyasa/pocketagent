// unknown_action.go —— 未知事件的确定性用户呈现与安全动作表
// (2026-10-04 未知事件用户处理中心计划 Task 2)。
//
// 【它解决什么】台账里的 UNKNOWN_* 条目以前只有方法名与技术原文;本文件按
// normalized_kind 套**固定文案**生成用户能读的标题/解释/影响,并给出 daemon 判定
// 安全的动作按钮表,由 server 层映射到 proto 的 UserActionDescriptor 与
// UnknownEventSummary 12~17 / UnknownEventResponse 7(已接线,见
// server/unknown_events.go 的 unknownActionsToProto / PresentUnknownEvent 调用点)。
//
// 【红线,按方案原文钉在这里】
//   - 确定性纯函数:按 NormalizedKind 查固定规则表,不经 LLM、不做载荷语义推断、
//     没有随机性 —— 同一封信封永远得到同一份呈现(unknown_action_test 快照钉住);
//   - 绝不为 UNKNOWN_REQUEST 产生 allow/approve/confirm 类动作(全局约束:"UNKNOWN_REQUEST
//     默认不显示'允许'按钮";安全钉子扫遍全部 kind 反证);
//   - 缺席值不造值:文案只来自规则表,不从信封字段编造;词表外的 kind(两档"已知"、
//     空、未来新值)全空返回,客户端按协议约定降级显示方法名原文(proto:空值时降级,
//     不编造);
//   - 只读:不改信封、不写台账、不落盘 —— 呈现失败只影响摘要,不影响事件登记与主轮次。
//
// 【为什么是固定文案而不是拼装载荷】人话解释承担"发生了什么、daemon 怎么处理了"
// 的安全沟通,措辞必须先经人审;载荷内容一律留在技术详情(原文可查、可复制),
// 普通 UI 不直接显示 JSON(全局约束)。方法名/来源 agent 原文不翻译、不改写,
// 由客户端从 summary 的 source_agent/source_method 原样展示。
package runtime

// UnknownUserAction 是 daemon 判定为**可以给用户看**的一个动作按钮。
// 【与 wire 的关系】字段一一对应 proto UserActionDescriptor(action_id/label/description/
// safe/requires_confirmation/destructive),映射由 server 层完成
// (server/unknown_events.go unknownActionsToProto);本层不 import gen 包。
type UnknownUserAction struct {
	// ID 是动作稳定键(inspect/copy/acknowledge/open_native/retry_later),客户端原样回传。
	ID string
	// Label 是按钮人话文案(daemon 给,客户端不翻译)。
	Label string
	// Description 一句话说明按下之后会发生什么;空 = 无。
	Description string
	// Safe 声明该动作安全;只有 Safe=true 才允许被渲染成可点按钮。
	Safe bool
	// RequiresConfirmation=true 表示执行前要再向用户确认一次(幂等动作也可能要求)。
	RequiresConfirmation bool
	// Destructive=true 表示有破坏性,UI 必须按危险样式渲染。
	Destructive bool
}

// UnknownPresentation 是一条未知事件面向用户的完整呈现
// (wire 对应 UnknownEventSummary 12~17 与 UnknownEventResponse 7)。
type UnknownPresentation struct {
	// PlainTitle 一行标题(说「来了个什么东西」);列表与卡片共用。
	PlainTitle string
	// PlainExplanation 两三句话:发生了什么、daemon 是怎么处理的。
	PlainExplanation string
	// Impact 对当前任务的影响(如「不影响正在进行的任务」「该操作未执行」)。
	Impact string
	// RequiresUser=true 进「需要我处理」分组;false 仅供参考,不催办。
	RequiresUser bool
	// CanOpenNative=true 表示电脑端有原生界面、动作表里有"在电脑端处理"类动作;
	// false = 客户端别画入口(没有动作兜底的入口是空头承诺)。
	CanOpenNative bool
	// Actions 是 daemon 判定安全的可选动作;空 = 只能查看/复制技术原文,客户端别造按钮。
	Actions []UnknownUserAction
	// RecommendedNextStep 建议的下一步(人话);空 = 无建议,客户端不占位。
	RecommendedNextStep string
}

// 三个 UNKNOWN_* kind 各自动作表的固定定义。Safe 全部为 true:这是方案 Task 6 圈定的
// "明确安全"动作集(inspect/acknowledge/retry-later/open-native,外加查看原文用的 copy);
// 任何新动作进这张表之前必须先过安全审查 —— 这张表就是安全边界本身,
// unknown_action_test 的安全钉子逐 kind 反证它不含允许类动作。
// 【为什么 RequiresConfirmation/Destructive 全 false】五个动作都是幂等的读/标记/跳转,
// 不执行任何未声明的副作用;破坏性动作(如丢弃载荷)不在方案范围内,一个都不设。
var (
	unknownActionInspect = UnknownUserAction{
		ID: "inspect", Label: "查看", Description: "展开查看这条事件的脱敏技术原文", Safe: true,
	}
	unknownActionCopy = UnknownUserAction{
		ID: "copy", Label: "复制", Description: "把脱敏技术原文复制到剪贴板", Safe: true,
	}
	unknownActionAcknowledge = UnknownUserAction{
		ID: "acknowledge", Label: "标记已查看", Description: "把这条事件标记为已查看", Safe: true,
	}
	unknownActionOpenNative = UnknownUserAction{
		ID: "open_native", Label: "在电脑端处理",
		Description: "在电脑端的原生界面处理这个请求，本端不代为执行", Safe: true,
	}
	unknownActionRetryLater = UnknownUserAction{
		ID: "retry_later", Label: "待上游支持", Description: "把这条事件标记为稍后再看", Safe: true,
	}
)

// PresentUnknownEvent 把一条未知事件信封翻成面向用户的呈现。确定性纯函数:
// 按 NormalizedKind 查固定规则表;唯一的方法名依赖是 UNKNOWN_REQUEST 标题追加
// SourceMethod 原文(不知道被请求的是什么操作,用户就无从判断要不要去电脑端处理)
// —— 只原样拼接、不改写不翻译,方法名缺席时回退固定标题(缺席值不造值)。
// 其余输出与载荷内容、来源 agent 无关;不改信封、不写台账、不产生副作用。
//
// 【词表外的 kind 怎么办】两档"已知"(KNOWN_PROJECTED/KNOWN_PASSTHROUGH)的人话走
// TranslateEvent(技术详情页),但不进用户处理中心 —— 这里没有它们的固定文案,全空
// 返回,客户端按协议约定降级显示方法名原文。空 kind、未来的新 kind 同理:
// 宁可空,不猜。
func PresentUnknownEvent(env EventEnvelope) UnknownPresentation {
	switch env.NormalizedKind {
	case KindUnknownNotification:
		// 未知通知:只是"没见过的新消息",不阻塞轮次、不显示成失败 —— 三个动作
		// 全是读取/标记,一个都不碰任务状态。
		return UnknownPresentation{
			PlainTitle:          "电脑端传来了新消息",
			PlainExplanation:    "当前版本还没有对应的卡片显示方式，内容已原样保留。",
			Impact:              "不影响正在进行的任务",
			RequiresUser:        false,
			CanOpenNative:       true,
			Actions:             []UnknownUserAction{unknownActionInspect, unknownActionCopy, unknownActionAcknowledge},
			RecommendedNextStep: "可在消息透析中查看原文",
		}
	case KindUnknownRequest:
		// 未知反向请求:安全红线所在地 —— 没有自动执行,也绝不提供"允许"类动作;
		// 用户的出路是去电脑端处理(那里有完整上下文)或标记待上游支持。
		// 标题带方法原文(主模型裁决 2026-10-04):原文照拼、绝不改写,
		// 缺席时回退固定标题 —— 不知道请求的是什么,安全决策无从谈起。
		title := "电脑端请求了一个操作"
		if env.SourceMethod != "" {
			title += "：" + env.SourceMethod
		}
		return UnknownPresentation{
			PlainTitle:          title,
			PlainExplanation:    "电脑端请求了一个当前版本无法安全判断的操作，因此没有自动执行。",
			Impact:              "该操作未执行；如涉及你关心的任务，请在电脑端处理",
			RequiresUser:        true,
			CanOpenNative:       true,
			Actions:             []UnknownUserAction{unknownActionInspect, unknownActionCopy, unknownActionOpenNative, unknownActionRetryLater},
			RecommendedNextStep: "在电脑端完成该操作后任务会继续",
		}
	case KindUnknownResponse:
		// 未知响应:对不上在途请求的结果,已留痕 —— 与通知一样不阻塞,但用户需要
		// 知道"有个结果没找到主人"。不设 open_native(动作表里没有对应动作,
		// CanOpenNative 不开空头承诺)。
		return UnknownPresentation{
			PlainTitle:          "电脑端返回了一条结果",
			PlainExplanation:    "电脑端返回了一条结果，但当前版本找不到它对应的请求。原始结果已保留。",
			Impact:              "不影响正在进行的任务",
			RequiresUser:        false,
			CanOpenNative:       false,
			Actions:             []UnknownUserAction{unknownActionInspect, unknownActionCopy},
			RecommendedNextStep: "可在消息透析中查看原始结果",
		}
	}
	return UnknownPresentation{}
}
