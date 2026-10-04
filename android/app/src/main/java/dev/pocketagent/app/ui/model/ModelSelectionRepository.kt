package dev.pocketagent.app.ui.model

import android.util.Log
import io.grpc.Status
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import dev.pocketagent.app.AgentConnection

/**
 * 模型/思考强度选择的仓库(2026-10-04 会话选择与历史体验计划 Task 3):
 * daemon 真值在手机端的单一投影点,照 CollabSessionRepository 的"快照 + 投影"先例。
 *
 * 【恢复链(spec:页面进入先缓存后替换)】
 *  1. `open` 先读本地"上次记录"(SharedPreferences 既有 threadModel:/threadEffort:
 *     键,经 AgentConnection 既有访问器)并**标注 fromCache**(UI 显示「上次记录」);
 *  2. 再 GetThread(effective_selection=24)/ListModels(effective_selection=5) 用
 *     daemon 真值**整树替换**;两路都缺(旧 daemon)→ reducer 按 2/3、5/19 兜底;
 *  3. SelectModel/SelectEffort/ResetToDefault:乐观更新 → 本地缓存(**只做缓存,
 *     非事实源**)→ 成功后 Refresh 用 daemon 真值回填;失败整体回滚 + 错误上屏;
 *  4. 断线:保留最后一份已知选择,错误如实呈现(与协同页"显示的是最后状态"同一承诺)。
 *
 * 【轮次红线】本仓库不发任何取消/改写轮次的调用 —— 没有 CancelTurn、不碰
 * TurnSession 的任何状态。切换选择是纯"下一次配置"下发:运行中的轮次按 daemon
 * TurnSnapshot 的快照配置跑完,applies_to_next_turn 由 daemon 真值说话。
 *
 * 【下发通道(现状口径,波 3/4 接线前保持不变)】
 *  - 会话范围:写 threadModel:/threadEffort: 缓存键 —— TurnSession.buildTurnInput
 *    会在**下一轮 TurnInput** 把它真实发给 daemon(既有机制,daemon Task 2 侧
 *    落库为会话覆盖);本层不发明第二条会话级写 RPC。
 *  - 全局范围(threadId 空):AgentConnection.setGlobalModel/setGlobalEffort
 *    (既有封装)+ `setDefaultModel` RPC(daemon 落库 App 默认 → APP_DEFAULT 来源;
 *    effort 无独立默认 RPC,只落本地,见风险清单)。
 */
object ModelSelectionRepository {

    private const val TAG = "ModelSelectionRepo"

    private val _state = MutableStateFlow(ModelSelectionReducer.initial())
    val state: StateFlow<ModelSelectionState> = _state.asStateFlow()

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    /** 当前观察的会话(离开页面不清,重开页面直接续)。 */
    @Volatile
    private var activeThreadId: String? = null

    @Volatile
    private var activeAgentId: String = ""

    /** 仅测试/诊断用:当前观察的会话 id。 */
    fun currentThreadId(): String? = activeThreadId

    // ---- 生命周期 ----

    /**
     * 进入页面(或切换会话):先缓存后替换。可重入 —— 同会话重复 open 只重走
     * "缓存 → 真值"链,无流可重启(选择是拉式投影,没有常驻事件流)。
     */
    fun open(threadId: String, agentId: String) {
        activeThreadId = threadId
        activeAgentId = agentId
        // 换会话:旧会话的投影不串会话(CollabSessionRepository.dispatch 同纪律)
        _state.update { st ->
            val fresh = st.threadId != threadId || st.agentId != agentId
            val base = if (fresh) ModelSelectionReducer.initial() else st
            base.copy(threadId = threadId, agentId = agentId)
        }
        // ① 先读本地"上次记录"(标注 fromCache;从未选过 = null,不编缓存)
        val cached = readLocalCache(threadId, agentId)
        _state.update { ModelSelectionReducer.withCachedSelection(it, cached) }
        // ② daemon 真值替换
        refresh()
    }

    /** 离开页面:无流可停,仅保留投影(与协同页 onPageHidden 的语义对齐)。 */
    fun onPageHidden() = Unit

    // ---- 意图(模型/强度卡的唯一入口) ----

    /** 处理一条 [ModelSelectionIntent](UI 只发意图,不碰 RPC/缓存/全局状态)。 */
    fun send(intent: ModelSelectionIntent) {
        when (intent) {
            is ModelSelectionIntent.SelectModel -> scope.launch { selectModel(intent.modelId) }
            is ModelSelectionIntent.SelectEffort -> scope.launch { selectEffort(intent.effort) }
            ModelSelectionIntent.ResetToDefault -> scope.launch { resetToDefault() }
            ModelSelectionIntent.Refresh -> refresh()
        }
    }

    private suspend fun selectModel(modelId: String) {
        // ① 乐观更新(含失效 effort 清理;【轮次红线】只是选择,不触碰轮次)
        _state.update { ModelSelectionReducer.reduce(it, ModelSelectionIntent.SelectModel(modelId)) }
        val cleanedEffort = _state.value.selection?.effort.orEmpty()
        val effortChanged = cleanedEffort != previousEffort()
        // ② 缓存(只写缓存,非事实源;失败回滚)。effort 只在真被清理时改写,
        //    不把"从未选过(null)"误标成"显式跟随默认("")"。
        //    【catch Exception 纪律】gRPC/IO 异常都是 Exception;CancellationException
        //    先 re-throw(协程取消不吞),与 refresh()/ChatScreen 既有先例同款。
        try {
            writeSelectionCache(modelId, if (effortChanged) cleanedEffort else null)
        } catch (e: kotlinx.coroutines.CancellationException) {
            throw e
        } catch (e: Exception) {
            Log.w(TAG, "选择缓存写入失败: ${e.message ?: e::class.java.simpleName}")
            rollback(rpcMessage(e))
            return
        }
        // ③ 已安全落盘 → 清在飞标记,refresh 用 daemon 真值回填(无效选择会被真值校正)
        _state.update { ModelSelectionReducer.confirmSent(it) }
        refresh()
    }

    private suspend fun selectEffort(effort: String) {
        _state.update { ModelSelectionReducer.reduce(it, ModelSelectionIntent.SelectEffort(effort)) }
        try {
            writeSelectionCache(modelId = null, effortId = effort)
        } catch (e: kotlinx.coroutines.CancellationException) {
            throw e
        } catch (e: Exception) {
            Log.w(TAG, "强度缓存写入失败: ${e.message ?: e::class.java.simpleName}")
            rollback(rpcMessage(e))
            return
        }
        _state.update { ModelSelectionReducer.confirmSent(it) }
        refresh()
    }

    /** 清会话覆盖:model/effort 双双置空(零值 = 跟随默认,既有缓存键把 "" 当合法显式值)。 */
    private suspend fun resetToDefault() {
        _state.update { ModelSelectionReducer.reduce(it, ModelSelectionIntent.ResetToDefault) }
        try {
            writeSelectionCache(modelId = "", effortId = "")
        } catch (e: kotlinx.coroutines.CancellationException) {
            throw e
        } catch (e: Exception) {
            Log.w(TAG, "重置缓存写入失败: ${e.message ?: e::class.java.simpleName}")
            rollback(rpcMessage(e))
            return
        }
        _state.update { ModelSelectionReducer.confirmSent(it) }
        refresh()
    }

    /**
     * 重拉 daemon 真值(GetThread 24 + ListModels 5/模型表):替换缓存、校正乐观投影。
     * 失败只标错误,不扔已有投影(断线时"显示最后状态"是承诺)。
     */
    fun refresh() {
        val threadId = activeThreadId ?: return
        val agentId = activeAgentId
        _state.update { it.copy(loading = true) }
        scope.launch {
            try {
                val thread = AgentConnection.getThread(threadId)
                val resp = AgentConnection.listModelsWithSelection(agentId, threadId)
                _state.update { st ->
                    // 模型表以本次回包为准(空回包保留旧表,巡检空窗不洗掉原料)
                    val models = if (resp.modelsCount > 0) resp.modelsList else st.models
                    // 竞态守卫:仍有在飞下发时,这份真值可能早于用户点选 —— 只更新
                    // 模型表,不覆盖乐观投影(回填只认"下发后发起的 refresh")
                    if (st.pending != null) {
                        st.copy(models = models, loading = false)
                    } else {
                        // 真值优先级:ListModels(5,带 available_efforts) > Thread(24) > 各自旧 daemon 兜底
                        val sel = ModelSelectionReducer.merge(
                            primary = ModelSelectionReducer.fromListModelsResponse(resp, agentId),
                            secondary = ModelSelectionReducer.fromThread(thread),
                        )
                        val scoped = if (sel != null && sel.agentId.isBlank()) sel.copy(agentId = agentId) else sel
                        if (scoped == null) st.copy(models = models, loading = false)
                        else ModelSelectionReducer.applyDaemon(st.copy(models = models), scoped)
                    }
                }
            } catch (e: kotlinx.coroutines.CancellationException) {
                throw e
            } catch (e: Exception) {
                Log.w(TAG, "选择真值刷新失败: ${e.message ?: e::class.java.simpleName}")
                _state.update { it.copy(loading = false, error = rpcMessage(e)) }
            }
        }
    }

    // ---- 缓存(SharedPreferences 只做"上次记录",非事实源) ----

    /**
     * 读本地"上次记录"。会话范围读 threadModel:/threadEffort:(null = 从未选过,
     * 两个都 null 就没有缓存可显示);全局范围读 agent 默认(modelForAgent/effortForAgent)。
     * 上次的显式选择 → 来源记 THREAD_OVERRIDE;empty 值原样保留(零值合法)。
     */
    private fun readLocalCache(threadId: String, agentId: String): EffectiveSelectionUi? {
        val model: String?
        val effort: String?
        if (threadId.isBlank()) {
            model = AgentConnection.modelForAgent(agentId).takeIf { it.isNotEmpty() }
            effort = AgentConnection.effortForAgent(agentId)
            if (model == null && effort.isNullOrBlank()) return null
        } else {
            model = AgentConnection.threadModel(threadId)
            effort = AgentConnection.threadEffort(threadId)
            if (model == null && effort == null) return null
        }
        return EffectiveSelectionUi(
            agentId = agentId,
            modelId = model.orEmpty(),
            effort = effort.orEmpty(),
            source = SelectionSource.THREAD_OVERRIDE,
            sourceRaw = SelectionSource.THREAD_OVERRIDE.raw,
            // 上次记录是过去的选择,不主张生效时机(false=无"下一轮"结论;真值回填后以 daemon 为准)
            appliesToNextTurn = false,
            explanation = "",
            availableEfforts = emptyList(),
            fromCache = true,
        )
    }

    /**
     * 写选择缓存。[modelId]/[effortId] 传 null = 不动该项。
     * 会话范围走既有 setThreadModel/setThreadEffort(TurnSession 下一轮 TurnInput
     * 读它们发给 daemon);全局范围走 setGlobalModel/setGlobalEffort(既有封装)+
     * setDefaultModel RPC(daemon 落库 App 默认;空串 = 跟随桌面,daemon 端存原串)。
     */
    private fun writeSelectionCache(modelId: String?, effortId: String?) {
        val threadId = activeThreadId.orEmpty()
        if (threadId.isNotBlank()) {
            if (modelId != null) AgentConnection.setThreadModel(threadId, modelId)
            if (effortId != null) AgentConnection.setThreadEffort(threadId, effortId)
        } else {
            if (modelId != null) {
                AgentConnection.setGlobalModel(modelId)
                AgentConnection.pushAppDefaultModel(modelId)
            }
            if (effortId != null) AgentConnection.setGlobalEffort(effortId)
        }
    }

    /** 乐观更新前的 effort 值(判断"失效清理是否真的改写了它")。 */
    private fun previousEffort(): String =
        _state.value.pending?.previous?.effort ?: _state.value.cached?.effort.orEmpty()

    private fun rollback(message: String) {
        _state.update { ModelSelectionReducer.rollback(it, message) }
    }

    /** 异常 → 可透传原文(daemon 的 Status.description 优先,照 CollabSessionRepository 既有纪律)。 */
    private fun rpcMessage(e: Exception): String {
        val desc = Status.fromThrowable(e).description
        if (!desc.isNullOrBlank()) return desc
        return e.message?.takeIf { it.isNotBlank() } ?: e::class.java.simpleName
    }
}
