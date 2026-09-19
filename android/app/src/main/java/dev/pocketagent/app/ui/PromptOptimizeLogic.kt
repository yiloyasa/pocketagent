package dev.pocketagent.app.ui

// PromptOptimizeLogic.kt —— 输入栏"优化提示词"的纯逻辑(无 Android 依赖,可 JVM 单测)。
//
// 【链路】Sparkles 按钮(仅实验开关开且输入非空时可见)→ RPC OptimizePrompt
// → 成功:替换输入框内容 + UndoBar「已优化提示词[撤销]」;失败:人话 Toast。
// 本文件只放"按钮可见性状态机"与"失败文案分类"这两块纯逻辑;RPC 调用与
// Compose 状态写在 ChatScreen(与 voiceReduce 同款分工:纯逻辑可测,副作用留 UI)。

/**
 * 优化 RPC 的手机端 deadline(秒)。daemon 侧整段预算 150s(冷启 + 候选模型
 * 逐个尝试),手机端再加余量,保证 daemon 的错误能跑完回程而不是被手机先掐。
 */
internal const val PROMPT_OPTIMIZE_DEADLINE_S = 170L

/**
 * 优化流程的状态机状态。
 *
 * @param buttonVisible 「优化」按钮当前是否可见(开关开 + 在线 + 输入非空 + 不在请求中)
 * @param inFlight 一次优化请求是否进行中(进行中按钮隐藏,防重复点击)
 */
internal data class PromptOptimizeState(
    val buttonVisible: Boolean = false,
    val inFlight: Boolean = false,
)

/** 驱动状态机的事件:输入/开关/连接性变化,与请求生命周期。 */
internal sealed interface PromptOptimizeEvent {
    /** 输入框内容、实验开关或离线态变化(每次重组/状态变化重算可见性)。 */
    data class InputsChanged(
        val input: String,
        val featureEnabled: Boolean,
        val offline: Boolean,
    ) : PromptOptimizeEvent

    /** 用户点了「优化」:进入请求中(按钮随即隐藏)。 */
    data object OptimizeStarted : PromptOptimizeEvent

    /** 优化成功(输入已被替换,撤销交给 UndoBar)。 */
    data object OptimizeSucceeded : PromptOptimizeEvent

    /** 优化失败(文案由 [optimizePromptErrorText] 分类,Toast 呈现)。 */
    data object OptimizeFailed : PromptOptimizeEvent
}

/**
 * 状态机归约。
 *
 * 【可见性规则】按钮 = 开关开 且 不在离线快照态 且 输入非空 且 当前没有请求在飞。
 * 【终局语义】成功/失败都回到"请求中"之前的闲置态:buttonVisible 先归 false,
 * 由紧随其后的 InputsChanged(输入已更新/原文未动)重算 —— 单一事实来源是输入,
 * 状态机只记"请求在飞"这一件事。
 */
internal fun reducePromptOptimize(state: PromptOptimizeState, event: PromptOptimizeEvent): PromptOptimizeState {
    return when (event) {
        is PromptOptimizeEvent.InputsChanged -> state.copy(
            buttonVisible = event.featureEnabled &&
                !event.offline &&
                event.input.isNotBlank() &&
                !state.inFlight,
        )
        PromptOptimizeEvent.OptimizeStarted ->
            if (state.buttonVisible) state.copy(inFlight = true, buttonVisible = false) else state
        PromptOptimizeEvent.OptimizeSucceeded, PromptOptimizeEvent.OptimizeFailed ->
            state.copy(inFlight = false, buttonVisible = false)
    }
}

/**
 * 优化失败 → 人话提示(Toast 用)。
 *
 * 【文案来源】daemon 侧已把失败翻成人话放在 gRPC status 的 description 里
 * (配额用尽 / ZCode 不可用 / 通用失败附原文),这里直接透传;description 缺失
 * (典型:纯网络断连)才按 code 兜底。手机未连接的判定在 RPC 之前短路,
 * 不至于让用户对一条必然失败的请求干等。
 *
 * @param code gRPC status code 名(如 "UNAVAILABLE"),可空
 * @param description gRPC status description(daemon 的人话文案),可空
 * @param connected 手机当前是否与电脑在线(connState == "ok")
 */
internal fun optimizePromptErrorText(code: String?, description: String?, connected: Boolean): String {
    if (!connected) return "未连接电脑,请先连接再优化"
    if (!description.isNullOrBlank()) return description
    return when (code) {
        "DEADLINE_EXCEEDED", "CANCELLED" -> "优化超时了,请稍后再试(原文未被改动)"
        "INVALID_ARGUMENT" -> "先输入内容,再点优化"
        "UNAVAILABLE" -> "连接电脑中断或 ZCode 不可用,请检查连接与桌面端 ZCode"
        else -> "优化失败,请稍后再试(原文未被改动)"
    }
}
