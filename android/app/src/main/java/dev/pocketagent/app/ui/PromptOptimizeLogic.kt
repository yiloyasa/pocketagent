package dev.pocketagent.app.ui

// PromptOptimizeLogic.kt —— 输入栏"优化提示词"的纯逻辑(无 Android 依赖,可 JVM 单测)。
//
// 【链路】Sparkles 按钮(仅实验开关开且输入非空时可见)→ RPC OptimizePrompt
// → 成功:替换输入框内容 + UndoBar「已优化提示词[撤销]」;失败:人话 Toast。
// 本文件放纯逻辑:按钮可见性状态机、优化风格词表(枚举→wire)、请求参数模型、
// 失败文案分类(kind + 人话);RPC 组装/调用归 PromptOptimizeRepository
// (ui/prompt/),Compose 状态留在 ChatScreen(与 voiceReduce 同款分工:
// 纯逻辑可测,副作用留 UI)。

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
internal fun optimizePromptErrorText(code: String?, description: String?, connected: Boolean): String =
    classifyPromptOptimizeFailure(code, description, connected).humanMessage

/**
 * 优化风格(P4 Task 1):输入栏"优化"按钮可选择的改写口径。
 *
 * 【为什么是枚举 + wireValue】proto 的 OptimizePromptRequest.style 是 **string**
 * (proto/v1/thread.proto: `string style = 2`),daemon 侧按小写字符串匹配拼
 * 优化指令(prompt_opt.go buildOptimizeInstruction)。所以手机端用类型安全的
 * 枚举收口用户选择,发 RPC 时经 [wireValue] 映射回 daemon 的字符串词表。
 *
 * 【daemon 侧模板词表(P4 Task 3 已落地,daemon/internal/server/prompt_opt.go
 * 的 promptOptimizeStyleTemplates)】
 *   - ""          → 默认:保持长度量级,把意思说清楚,不扩写需求;
 *   - "concise"   → 精简:压缩成一句话;
 *   - "detailed"  → 详细:整理成要点列表;
 *   - "implementation_plan" → 实施计划:目标、范围、验收标准;
 *   - "bug_report"          → 排错任务:现象、环境、复现、期望结果;
 *   - "test_plan"           → 测试要求:输入、行为、验证方式;
 *   - "code_review"         → 代码审查:风险、影响范围、证据要求。
 *
 * 【降级安全】daemon 对未识别的 style 仍走默认段(map 未命中 → 默认模板),
 * 所以旧 daemon 配新手机不会报错,只是退化成默认改写;七个值现已全通。词表
 * 大小写口径与 daemon 一致:全小写 + 下划线(daemon 先 TrimSpace + ToLower 再匹配)。
 */
internal enum class PromptOptimizeStyle(val wireValue: String) {
    DEFAULT(""),
    CONCISE("concise"),
    DETAILED("detailed"),
    IMPLEMENTATION_PLAN("implementation_plan"),
    BUG_REPORT("bug_report"),
    TEST_PLAN("test_plan"),
    CODE_REVIEW("code_review"),
}

/**
 * 风格的一句话人话说明(P4 Task 3「可解释」):预览弹层的风格芯片与设置页的
 * 模式说明共用同一词表,保证"设置里读到的"与"优化时看到的"口径一致;文案与
 * daemon 模板段(promptOptimizeStyleTemplates)逐条对齐。
 */
internal fun promptOptimizeStyleHint(style: PromptOptimizeStyle): String = when (style) {
    PromptOptimizeStyle.DEFAULT -> "默认:把意思说清楚,不扩写需求"
    PromptOptimizeStyle.CONCISE -> "精简:压缩成一句话"
    PromptOptimizeStyle.DETAILED -> "详细:整理成要点列表"
    PromptOptimizeStyle.IMPLEMENTATION_PLAN -> "实施计划:整理目标、范围、验收标准"
    PromptOptimizeStyle.BUG_REPORT -> "排错任务:整理现象、环境、复现、期望结果"
    PromptOptimizeStyle.TEST_PLAN -> "测试要求:整理输入、行为和验证方式"
    PromptOptimizeStyle.CODE_REVIEW -> "代码审查:整理风险、影响范围、证据要求"
}

/**
 * 一次优化请求的参数(P4 Task 1):仓库层组装 wire 请求的唯一输入。
 *
 * 【为什么是显式模型】现状 ChatScreen 内联组装 OptimizePromptRequest 时只传
 * text/agentId/modelId,proto 早已就位的 style/effort 从未进 wire;升级后由
 * [dev.pocketagent.app.ui.prompt.PromptOptimizeRepository] 按本模型统一组装,
 * style/effort/优化 Agent/模型全部显式透传(可测、可钉)。
 *
 * @param original 输入栏原始文本(仓库层 trim 后进 wire;空白原文直接短路不发 RPC)
 * @param style 改写风格,经 [PromptOptimizeStyle.wireValue] 映射进 wire
 * @param agentId 优化用的 Agent(daemon 侧在其上生成改写,可与当前会话不同)
 * @param modelId 优化用的模型(空 = daemon 侧该 Agent 的默认模型)
 * @param effort 优化用的推理档位(daemon 侧拼进生成请求;空 = 默认档)
 * @param requestVersion 发起请求时输入框的修订号(预览层防覆盖用:
 *   请求返回时若输入框 revision 已前进,Task 2 的预览层据此要求用户二选一,
 *   不覆盖新输入。本层不解读、只透传)。
 */
internal data class PromptOptimizeRequestModel(
    val original: String,
    val style: PromptOptimizeStyle,
    val agentId: String,
    val modelId: String,
    val effort: String,
    val requestVersion: Long,
)

/** 优化失败的机器可读分类(配额/登录/连接/超时四类人话映射 + 本地终态)。 */
internal enum class PromptOptimizeErrorKind {
    /** 电脑端生成模型配额/用量上限耗尽。 */
    QUOTA,

    /** 电脑端登录状态失效或无权限。 */
    AUTH,

    /** 手机与电脑断连,或桌面端 ZCode 子进程不可用。 */
    CONNECTION,

    /** 超时(daemon 预算耗尽或手机端 deadline 先到)。 */
    TIMEOUT,

    /** daemon 返回了空改写文本(协议上不该发生,双保险拒收,绝不拿空串当成功)。 */
    EMPTY,

    /** 请求被取消(用户退出页面/发起新请求),不是错误,原文保持不动。 */
    CANCELLED,

    /** 其余未分类失败。 */
    OTHER,
}

/**
 * 优化失败:[humanMessage] 是可直接 Toast/弹层展示的人话(非空口径),[kind] 供
 * 调用方做重试/埋点分类。
 *
 * 【为什么另立 humanMessage】继承来的 `RuntimeException.message` 因 Throwable
 * 声明固定是 `String?`,而本类构造时全部路径(分类器/兜底文案/空响应/取消)
 * 传入的都是非空字面量或非空实参 —— 所以收一个非空别名属性供 Kotlin 侧直接
 * 取用,免去调用方逐处 `!!` 或 `?: 兜底`;底层 message 照常带上(兼容既有
 * 测试与 Result.failure 链路)。
 */
internal class PromptOptimizeException(
    val kind: PromptOptimizeErrorKind,
    val humanMessage: String,
) : RuntimeException(humanMessage)

/**
 * gRPC 失败 → (kind, 人话) 的统一分类(P4 Task 1 口径;daemon Task 3 落地后细化)。
 *
 * 【分类顺序】未连接短路 → daemon 人话 description 透传(kind 按关键词修正)
 * → 无 description 按 code 兜底人话。与既有 [optimizePromptErrorText] 同一条
 * 优先级链,只是额外给出机器可读 kind。
 *
 * 【四类人话映射】
 *   - 配额 QUOTA:code=RESOURCE_EXHAUSTED,或 description 含"上限/配额/quota/token plan"
 *     (关键词来自 daemon 实测文案:"Token Plan 用量上限"、"配额已用完");
 *   - 登录 AUTH:code=UNAUTHENTICATED/PERMISSION_DENIED,或 description 含"登录";
 *   - 连接 CONNECTION:code=UNAVAILABLE,或未连接短路;
 *   - 超时 TIMEOUT:code=DEADLINE_EXCEEDED(daemon 侧取消多因预算耗尽,CANCELLED
 *     沿用既有口径也归超时文案;手机端**主动取消**走 CancellationException,
 *     在仓库层直接归 [PromptOptimizeErrorKind.CANCELLED],不进本分类)。
 */
internal fun classifyPromptOptimizeFailure(
    statusCode: String?,
    description: String?,
    connected: Boolean,
): PromptOptimizeException {
    if (!connected) return PromptOptimizeException(PromptOptimizeErrorKind.CONNECTION, "未连接电脑,请先连接再优化")
    if (!description.isNullOrBlank()) {
        return PromptOptimizeException(kindFromDescription(description, statusCode), description)
    }
    return PromptOptimizeException(kindFromCode(statusCode), fallbackText(statusCode))
}

/** 无 daemon 人话时按 code 定 kind。 */
private fun kindFromCode(code: String?): PromptOptimizeErrorKind = when (code) {
    "UNAUTHENTICATED", "PERMISSION_DENIED" -> PromptOptimizeErrorKind.AUTH
    "RESOURCE_EXHAUSTED" -> PromptOptimizeErrorKind.QUOTA
    "UNAVAILABLE" -> PromptOptimizeErrorKind.CONNECTION
    "DEADLINE_EXCEEDED", "CANCELLED" -> PromptOptimizeErrorKind.TIMEOUT
    else -> PromptOptimizeErrorKind.OTHER
}

/** daemon 人话在手的 kind 修正:关键词命中配额/登录即改判,否则回落 code 口径。 */
private fun kindFromDescription(description: String, code: String?): PromptOptimizeErrorKind {
    val lower = description.lowercase()
    return when {
        listOf("上限", "配额", "quota", "token plan").any { lower.contains(it) } ->
            PromptOptimizeErrorKind.QUOTA
        description.contains("登录") -> PromptOptimizeErrorKind.AUTH
        else -> kindFromCode(code)
    }
}

/** 无 daemon 人话时的兜底文案(与既有 Toast 口径一致,不新造句子)。 */
private fun fallbackText(code: String?): String = when (code) {
    "UNAUTHENTICATED", "PERMISSION_DENIED" -> "电脑端登录状态失效,请在电脑端重新登录后重试"
    "RESOURCE_EXHAUSTED" -> "优化配额已用尽,可直接发送原文,或稍后再试"
    "UNAVAILABLE" -> "连接电脑中断或 ZCode 不可用,请检查连接与桌面端 ZCode"
    "DEADLINE_EXCEEDED", "CANCELLED" -> "优化超时了,请稍后再试(原文未被改动)"
    "INVALID_ARGUMENT" -> "先输入内容,再点优化"
    else -> "优化失败,请稍后再试(原文未被改动)"
}
