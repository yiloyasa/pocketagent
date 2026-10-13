package dev.pocketagent.app.ui.model

import pocketagent.v1.EffectiveModelSelection
import pocketagent.v1.ListModelsResponse
import pocketagent.v1.ModelInfo

/**
 * 模型选择纯函数 reducer(2026-10-04 计划 Task 3):状态 + 意图 → 状态。
 *
 * 【设计纪律】(照 CollabSessionReducer 先例)
 *  - **纯函数、无 IO、无 Android 类**:全部规则 JVM 单测可断言
 *    (ModelSelectionReducerTest),改坏任何一条映射在测试里红,而不是在真机上被看见;
 *  - **daemon 是唯一事实源**:真值(GetThread 24 / ListModels 5)一到整树替换,
 *    缓存标注与乐观投影一并退场;本地 SharedPreferences 只是"上次记录"缓存;
 *  - **零值合法**:model/effort 空串 = 跟随默认,本层绝不清除、绝不改写成具体值;
 *  - **轮次红线**:reducer 里没有任何轮次概念 —— SelectModel/SelectEffort 只把
 *    "将从下一轮生效"标在 [EffectiveSelectionUi.appliesToNextTurn] 上(客户端乐观口径),
 *    运行中的 Turn 在本层不可达、不可改写。
 */
object ModelSelectionReducer {

    /** 内置默认档(与 ChatScreen EFFORTS 的 id 集合同源口径;agent 从不透出动态档位时用它)。 */
    val BUILTIN_EFFORT_IDS: Set<String> = setOf("", "low", "medium", "high", "xhigh")

    // ---- 映射:daemon proto → 客户端投影(字段对照只写这一遍) ----

    /** daemon source 字符串 → [SelectionSource];不认识的取值(含空)如实落 UNKNOWN。 */
    fun mapSource(raw: String): SelectionSource = when (raw) {
        "thread_override" -> SelectionSource.THREAD_OVERRIDE
        "app_default" -> SelectionSource.APP_DEFAULT
        "desktop_default" -> SelectionSource.DESKTOP_DEFAULT
        "native_current" -> SelectionSource.NATIVE_CURRENT
        else -> SelectionSource.UNKNOWN
    }

    /** proto EffectiveModelSelection → 客户端投影(最新 daemon 恒带该字段)。 */
    fun mapSelection(sel: EffectiveModelSelection, fromCache: Boolean = false): EffectiveSelectionUi =
        EffectiveSelectionUi(
            agentId = sel.agentId,
            modelId = sel.modelId,
            effort = sel.effort,
            source = mapSource(sel.source),
            sourceRaw = sel.source,
            appliesToNextTurn = sel.appliesToNextTurn,
            configurationConfirmed = sel.configurationConfirmed,
            explanation = sel.explanation,
            availableEfforts = normalizeEfforts(sel.availableEffortsList),
            fromCache = fromCache,
        )

    /**
     * ListModelsResponse → 选择投影。
     * 最新-only(0.3.1 起):新 daemon 恒带 effective_selection(5)；缺席为协议
     * 违规，直接抛错，绝不再按 current_model_id(2)/current_effort(3) 旧字段兜底。
     */
    fun fromListModelsResponse(
        resp: ListModelsResponse,
        agentId: String,
        fromCache: Boolean = false,
    ): EffectiveSelectionUi {
        // agentId 仅供日志定位，投影以 daemon 真值为准。
        require(resp.hasEffectiveSelection()) {
            "电脑端未回传选择结论(agent=$agentId)，请确认电脑端为最新版 daemon"
        }
        return mapSelection(resp.effectiveSelection, fromCache)
    }

    /**
     * Thread → 选择投影。
     * 最新-only(0.3.1 起):新 daemon 恒带 effective_selection(24)；缺席为协议
     * 违规，直接抛错，绝不再按 model_id(5)/effort(19) 旧字段兜底。
     */
    fun fromThread(thread: pocketagent.v1.Thread, fromCache: Boolean = false): EffectiveSelectionUi {
        require(thread.hasEffectiveSelection()) {
            "会话未回传选择结论(agent=${thread.agentId})，请确认电脑端为最新版 daemon"
        }
        return mapSelection(thread.effectiveSelection, fromCache)
    }

    /** 两路真值合并:[primary] 优先(通常为 ListModels 5,带 available_efforts);null 退 [secondary]。 */
    fun merge(primary: EffectiveSelectionUi?, secondary: EffectiveSelectionUi?): EffectiveSelectionUi? =
        primary ?: secondary

    // ---- 档位集合(失效 effort 清理的判定原料;规则与 Task 4 的 UI 规则同源,数据层先落) ----

    /**
     * 模型可用档位集合(daemon ModelInfo.available_efforts 透出时按它;留空 =
     * 客户端内置默认档,与 effortsFor 的回退规则同源)。返回 null = **无法判定**
     * (模型不在表里且没有 daemon 给的档位):调用方必须保留原值,不猜。
     */
    fun allowedEfforts(
        models: List<ModelInfo>,
        agentId: String,
        modelId: String,
    ): Set<String>? {
        // daemon 真值优先:选中模型带动态档位(已按会话 agent 过滤的表内查找)
        val dynamic = models.firstOrNull { it.id == modelId }?.availableEffortsList.orEmpty()
        if (dynamic.isNotEmpty()) return dynamic.toSet()
        // 该 agent 从不透出档位(codex/zcode 系)→ 内置默认档
        val agentIsEffortBacked = models.any { it.provider == agentId && it.availableEffortsCount > 0 }
        if (!agentIsEffortBacked) return BUILTIN_EFFORT_IDS
        // 档位制 agent(pi)的已知模型却没带档位 → 只剩"跟随默认"(未知/私有模型不伪造档位)
        return setOf("")
    }

    /**
     * 切模型时的失效 effort 清理(Task 4 规则在本层先落数据逻辑):
     * 新模型的档位集合不含当前强度 → 清成空串(跟随默认,零值合法);
     * 集合为 null(无法判定)→ **保留原值**,等 daemon 真值回填校正,不猜。
     */
    fun cleanEffortFor(
        models: List<ModelInfo>,
        agentId: String,
        currentEffort: String,
        newModelId: String,
    ): String {
        if (currentEffort.isBlank()) return ""
        val allowed = allowedEfforts(models, agentId, newModelId) ?: return currentEffort
        return if (currentEffort in allowed) currentEffort else ""
    }

    /**
     * 档位表归一:未知 effort **保留原 id 展示**(daemon 未来新增取值时 UI 不空白,
     * effortLabelOf 的 else -> id 同纪律),去重、去空白项、顺序保持 daemon 顺序。
     */
    fun normalizeEfforts(raw: List<String>): List<String> =
        raw.filter { it.isNotBlank() }.distinct()

    // ---- 状态流转 ----

    /** 空状态(进入页面、尚未读缓存/尚未拿到真值)。 */
    fun initial(): ModelSelectionState = ModelSelectionState()

    /**
     * 进入页面第 ① 步:先显示本地"上次记录"([cached] 必带 fromCache=true 标注)。
     * cached = null(该会话从未在手机上选过)→ 状态原样,等 daemon 真值。
     */
    fun withCachedSelection(state: ModelSelectionState, cached: EffectiveSelectionUi?): ModelSelectionState {
        if (cached == null) return state
        return state.copy(cached = cached.copy(fromCache = true))
    }

    /**
     * 进入页面第 ② 步 / Refresh 回填:daemon 真值整树替换。
     * 缓存标注、乐观投影、在飞 pending、错误一并退场 —— 真值即结论。
     * [loading] 由调用方决定(Refresh 路径 false;开屏真值早于 loading 置位时也 false)。
     */
    fun applyDaemon(state: ModelSelectionState, sel: EffectiveSelectionUi): ModelSelectionState =
        state.copy(
            selection = sel.copy(fromCache = false, optimistic = false),
            cached = null,
            pending = null,
            error = null,
            loading = false,
        )

    /** 下发已安全落盘(缓存/RPC 成功):清在飞标记,保留乐观投影等真值回填。 */
    fun confirmSent(state: ModelSelectionState): ModelSelectionState = state.copy(pending = null)

    /**
     * 下发失败:整体退回 [PendingSelection.previous](下发前的最后一份已知选择),
     * 错误人话上屏。缓存/真值如何补写归仓库层,reducer 只管状态。
     */
    fun rollback(state: ModelSelectionState, error: String): ModelSelectionState {
        val pending = state.pending ?: return state.copy(error = error)
        return state.copy(
            selection = pending.previous,
            pending = null,
            error = error,
        )
    }

    /** UI 该显示哪一份:daemon 真值/乐观投影优先,退"上次记录"缓存。 */
    fun display(state: ModelSelectionState): EffectiveSelectionUi? =
        state.selection ?: state.cached

    /**
     * 应用一条 [ModelSelectionIntent](乐观更新部分;Refresh 的拉取动作归仓库层)。
     * 返回新状态(不可变;同值幂等原样返回,不发无谓的 pending)。
     */
    fun reduce(state: ModelSelectionState, intent: ModelSelectionIntent): ModelSelectionState {
        return when (intent) {
            is ModelSelectionIntent.SelectModel -> selectModel(state, intent.modelId)
            is ModelSelectionIntent.SelectEffort -> selectEffort(state, intent.effort)
            ModelSelectionIntent.ResetToDefault -> resetToDefault(state)
            ModelSelectionIntent.Refresh -> state.copy(loading = true)
        }
    }

    /**
     * 选模型(乐观):
     *  - 新 modelId + 失效 effort 清理(见 [cleanEffortFor]);
     *  - 来源 = THREAD_OVERRIDE(手机端显式选择即会话覆盖,proto 注释口径);
     *  - appliesToNextTurn = true(客户端乐观口径:【轮次红线】绝不改写正在运行的
     *    Turn,daemon 真值回填后以 daemon 的 applies_to_next_turn 为准);
     *  - 在飞锚点记下上一份选择,失败整体回滚。
     */
    private fun selectModel(state: ModelSelectionState, modelId: String): ModelSelectionState {
        val cur = currentSelection(state)
        val cleaned = cleanEffortFor(state.models, cur.agentId.ifBlank { state.agentId }, cur.effort, modelId)
        // 幂等:同模型且失效清理不改变强度 → 原样返回,不发无谓的 pending/回填
        if (cur.modelId == modelId && cur.effort == cleaned) return state
        return state.copy(
            selection = cur.copy(
                modelId = modelId,
                effort = cleaned,
                source = SelectionSource.THREAD_OVERRIDE,
                sourceRaw = SelectionSource.THREAD_OVERRIDE.raw,
                appliesToNextTurn = true,
                configurationConfirmed = false,
                explanation = "",
                availableEfforts = normalizeEfforts(
                    state.models.firstOrNull { it.id == modelId }?.availableEffortsList ?: emptyList(),
                ),
                fromCache = false,
                optimistic = true,
            ),
            pending = PendingSelection(previous = state.selection ?: state.cached),
        )
    }

    /**
     * 选思考强度(乐观):未知 effort **保留原 id**(展示层 else -> id 兜底),
     * 合法性由 daemon ValidateSelection 在真值回填时裁决 —— 本层不猜、不拒。
     */
    private fun selectEffort(state: ModelSelectionState, effort: String): ModelSelectionState {
        val cur = currentSelection(state)
        // 幂等:同强度原样返回(无论来源是缓存还是真值,同值重申不发 pending)
        if (cur.effort == effort) return state
        return state.copy(
            selection = cur.copy(
                effort = effort,
                source = SelectionSource.THREAD_OVERRIDE,
                sourceRaw = SelectionSource.THREAD_OVERRIDE.raw,
                appliesToNextTurn = true,
                configurationConfirmed = false,
                explanation = "",
                fromCache = false,
                optimistic = true,
            ),
            pending = PendingSelection(previous = state.selection ?: state.cached),
        )
    }

    /**
     * 清会话覆盖(乐观):model/effort **双双置空** —— 零值语义 = 跟随默认,
     * 是合法状态不是错误(,proto 钉死;这也是唯一能回到"跟随默认"的路)。
     */
    private fun resetToDefault(state: ModelSelectionState): ModelSelectionState {
        val cur = currentSelection(state)
        // 幂等:已是"跟随默认"(双空,零值合法)→ 原样返回,不发无谓的 pending
        if (cur.modelId.isEmpty() && cur.effort.isEmpty()) return state
        return state.copy(
            selection = cur.copy(
                modelId = "",
                effort = "",
                source = SelectionSource.THREAD_OVERRIDE,
                sourceRaw = SelectionSource.THREAD_OVERRIDE.raw,
                appliesToNextTurn = true,
                configurationConfirmed = false,
                explanation = "",
                fromCache = false,
                optimistic = true,
            ),
            pending = PendingSelection(previous = state.selection ?: state.cached),
        )
    }

    /** 乐观计算的基準:优先 daemon 真值/既有乐观投影,退"上次记录"缓存,再退空结论。 */
    private fun currentSelection(state: ModelSelectionState): EffectiveSelectionUi =
        state.selection
            ?: state.cached?.copy(fromCache = false, optimistic = false)
            ?: EffectiveSelectionUi(
                agentId = state.agentId,
                // 空基线 = 从未有过任何选择的"跟随默认"态(零值合法,source 如实未知)
                modelId = "",
                effort = "",
                source = SelectionSource.UNKNOWN,
                sourceRaw = "",
                appliesToNextTurn = false,
                explanation = "",
                availableEfforts = emptyList(),
            )
}
