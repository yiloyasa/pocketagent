package dev.pocketagent.app.ui.model

/**
 * 模型/思考强度选择的仓库层状态与意图(2026-10-04 会话选择与历史体验计划 Task 3)。
 *
 * 【它是什么】ChatScreen 里 `selectedModel/defaultModel/threadModels/runtimeModel`
 * 一组可变全局状态互相覆盖、谁也说不清最后生效的是哪个 —— 本文件把它们收敛成
 * 一棵 [ModelSelectionState] 状态树:daemon 的 `EffectiveModelSelection`
 * (model.proto,四级来源收敛结论)是**唯一事实源**,本层只是它的客户端投影;
 * 手机本地的 SharedPreferences 降级为"上次记录"缓存(永远标注 [EffectiveSelectionUi.fromCache],
 * 永远不当事实源)。
 *
 * 【轮次红线】本层只表达"下一次选择是什么",绝不触碰正在运行的轮次:
 * 没有 Cancel/Start/改写 TurnSession 状态的任何通道 —— 运行中切换 =
 * daemon 口径的 `applies_to_next_turn`(从下一轮生效),UI 只如实渲染。
 */

/**
 * 选择结论的来源(proto EffectiveModelSelection.source 的稳定字符串,四值钉死;
 * 新增来源不改 proto,客户端遇到不认识的取值落 [UNKNOWN] 并把原文留在
 * [EffectiveSelectionUi.sourceRaw] 如实展示,不得报错 —— 与 CollabStatus.UNKNOWN 同纪律)。
 */
enum class SelectionSource(val raw: String) {
    /** 本会话显式覆盖(手机端选择/委派指定)。 */
    THREAD_OVERRIDE("thread_override"),

    /** App 侧默认(daemon 落库的默认模型)。 */
    APP_DEFAULT("app_default"),

    /** 桌面端默认(如 codex config 的当前值)。 */
    DESKTOP_DEFAULT("desktop_default"),

    /** agent 原生当前值(会话实挂的模型/强度)。 */
    NATIVE_CURRENT("native_current"),

    /** daemon 给了本客户端不认识的来源串(新版本新增);原文见 sourceRaw。 */
    UNKNOWN("");
}

/**
 * 一个上下文下"实际生效"的模型/思考强度选择(计划 Task 3 接口的原样落地)。
 *
 * 【零值语义(proto 钉死,勿改)】[modelId]/[effort] 为空串 = "跟随默认",是**合法状态**,
 * 不是错误也不是"未设置";任何一层都不许把空串当异常,更不许擅自改写成具体值
 * (一旦写死,用户就永远回不到"跟随默认")。
 *
 * 字段顺序 = 计划接口清单顺序;其后为本计划的**增量字段**(均有明确归属,
 * 与 CollabSessionState 增量 needsResync 同先例)。
 */
data class EffectiveSelectionUi(
    /** 归属的 agent id("codex" / "pi" / …);空 = 当前默认 agent。 */
    val agentId: String,
    /** 生效的模型 id("pi/glm-5.3" 风格);空 = 跟随默认(合法取值)。 */
    val modelId: String,
    /** 生效的思考强度(low/medium/high/…);空 = 跟随默认(合法取值)。 */
    val effort: String,
    /** 结论来自哪一级来源。 */
    val source: SelectionSource,
    /**
     * true = 这份选择尚未落进任何已开始的轮次,将从**下一轮**生效
     * (运行中切换的正确口径:绝不改写正在运行的 Turn)。
     */
    val appliesToNextTurn: Boolean,
    /** 人话解释:为什么生效的是这个(daemon 原文;空 = 无)。UI 直接展示。 */
    val explanation: String,
    /** 当前模型实际可用的思考强度档位(daemon 填充;空 = 客户端用内置默认档)。 */
    val availableEfforts: List<String>,
    // ---- 增量字段(计划接口外,客户端约定)----

    /** 来源原文([SelectionSource.UNKNOWN] 降级展示用;已知来源时与 source.raw 同)。 */
    val sourceRaw: String = "",
    /** 用户是否对这份配置显式确认过(区分"系统替你选的"与"你选过的")。 */
    val configurationConfirmed: Boolean = false,
    /** true = 这是本地"上次记录"缓存,尚未被 daemon 真值替换(UI 必须标注「上次记录」)。 */
    val fromCache: Boolean = false,
    /**
     * true = 乐观投影(客户端事实:用户刚点选、daemon 真值还没回填)。
     * 与 CollabStatus.SUBMITTED 同一性质 —— 是诚实的客户端事实,不是编造的 daemon 事实;
     * daemon 真值一到即被替换,下发失败则整体回滚。
     */
    val optimistic: Boolean = false,
)

/** 一次在飞的选择下发(乐观更新的回滚锚点)。 */
data class PendingSelection(
    /** 下发前的上一份选择(daemon 真值或 null=还没有);失败时整体退回它。 */
    val previous: EffectiveSelectionUi?,
)

/**
 * 模型选择状态树(单一投影点,ModelSelectionRepository 持有 StateFlow)。
 */
data class ModelSelectionState(
    /** 当前观察的会话 id;空 = 全局(无会话上下文,如新会话草稿)。 */
    val threadId: String = "",
    /** 会话归属 agent(空 = 还没回填/默认 agent)。 */
    val agentId: String = "",
    /**
     * daemon 真值或乐观投影(GetThread(24)/ListModels(5) 回填,或用户刚点选)。
     * null = daemon 真值未达且没有乐观更新。
     */
    val selection: EffectiveSelectionUi? = null,
    /** 本地"上次记录"缓存(进入页面先显示它;daemon 真值一到即弃)。 */
    val cached: EffectiveSelectionUi? = null,
    /** 会话 agent 的模型表(daemon ListModels;失效 effort 清理与档位渲染的原料)。 */
    val models: List<pocketagent.v1.ModelInfo> = emptyList(),
    /** 在飞的选择下发(非 null = 乐观更新待确认,失败回滚到 pending.previous)。 */
    val pending: PendingSelection? = null,
    /** Refresh/首次装载在飞。 */
    val loading: Boolean = false,
    /** 传输层/操作层错误的人话呈现(daemon 原文优先;真值到达即清)。 */
    val error: String? = null,
)

/**
 * 用户对模型选择的意图(计划 Task 3 接口,原样落地)。
 * 全部经 ModelSelectionRepository.send 落到缓存/RPC —— UI 不直接碰
 * SharedPreferences、不直接写 AgentConnection 的散落全局状态。
 */
sealed interface ModelSelectionIntent {
    /** 选定模型(空串 = "跟随默认",合法取值)。 */
    data class SelectModel(val modelId: String) : ModelSelectionIntent

    /** 选定思考强度(空串 = "跟随默认",合法取值)。 */
    data class SelectEffort(val effort: String) : ModelSelectionIntent

    /** 清会话覆盖:model/effort 双双置空(零值语义 = 跟随默认)。 */
    data object ResetToDefault : ModelSelectionIntent

    /** 重拉 daemon 真值(替换缓存/校正乐观投影)。 */
    data object Refresh : ModelSelectionIntent
}
