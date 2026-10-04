package dev.pocketagent.app.ui.model

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.EffectiveModelSelection
import pocketagent.v1.ListModelsResponse
import pocketagent.v1.ModelInfo

/**
 * 模型选择 reducer 的纯函数回归(2026-10-04 会话选择与历史体验计划 Task 3)。
 *
 * 钉住的规则(与 Global Constraints / Review Focus 一一对应):
 *  1. 页面进入"先缓存后替换",旧缓存必须标注「上次记录」(fromCache);
 *  2. 乐观更新 + 失败回滚(在飞锚点 pending.previous);
 *  3. ResetToDefault 零值语义(model/effort 空串 = 跟随默认,合法状态);
 *  4. 失效 effort 清理(切到无该档位的模型时不保留失效强度);
 *  5. 未知 effort 保留原 id 展示(daemon 未来新增取值不显示空白);
 *  6. SelectionSource 四值映射全对齐,词表外/空如实降级 UNKNOWN;
 *  7. 旧 daemon 缺席 effective_selection 时按 2/3、5/19 兜底,不报错。
 */
class ModelSelectionReducerTest {

    // ---- 测试材料 ----

    private fun sel(
        modelId: String,
        effort: String = "high",
        source: String = "thread_override",
        nextTurn: Boolean = false,
        vararg efforts: String,
    ): EffectiveModelSelection =
        EffectiveModelSelection.newBuilder()
            .setAgentId("pi")
            .setModelId(modelId)
            .setEffort(effort)
            .setSource(source)
            .setAppliesToNextTurn(nextTurn)
            .setExplanation("测试解释")
            .addAllAvailableEfforts(efforts.toList())
            .build()

    private fun piModel(vararg efforts: String): ModelInfo =
        ModelInfo.newBuilder()
            .setId("pi/glm-5.3")
            .setProvider("pi")
            .addAllAvailableEfforts(efforts.toList())
            .build()

    private fun stateWithTruth(selection: EffectiveSelectionUi, models: List<ModelInfo> = emptyList()) =
        ModelSelectionReducer.applyDaemon(
            ModelSelectionReducer.initial().copy(agentId = "pi", threadId = "t1"),
            selection,
        ).let { if (models.isEmpty()) it else it.copy(models = models) }

    // ---- 6) SelectionSource 映射:全四值 + 未知/空降级 ----

    @Test
    fun selection_source_maps_all_four_daemon_values() {
        assertEquals(SelectionSource.THREAD_OVERRIDE, ModelSelectionReducer.mapSource("thread_override"))
        assertEquals(SelectionSource.APP_DEFAULT, ModelSelectionReducer.mapSource("app_default"))
        assertEquals(SelectionSource.DESKTOP_DEFAULT, ModelSelectionReducer.mapSource("desktop_default"))
        assertEquals(SelectionSource.NATIVE_CURRENT, ModelSelectionReducer.mapSource("native_current"))
    }

    @Test
    fun selection_source_unknown_or_blank_falls_back_to_unknown_with_raw_kept() {
        // 词表外新增来源:不报错,落 UNKNOWN;原文留给 sourceRaw 如实展示
        val mapped = ModelSelectionReducer.mapSelection(sel("pi/glm-5.3", source = "future_source"))
        assertEquals(SelectionSource.UNKNOWN, mapped.source)
        assertEquals("future_source", mapped.sourceRaw)
        // daemon 未给来源(空串)同样降级,不猜
        val blank = ModelSelectionReducer.mapSelection(sel("pi/glm-5.3", source = ""))
        assertEquals(SelectionSource.UNKNOWN, blank.source)
        assertEquals("", blank.sourceRaw)
    }

    // ---- 1) 缓存 → daemon 真值替换流转 + 「上次记录」标注 ----

    @Test
    fun cache_shown_first_with_last_record_mark_then_replaced_by_daemon_truth() {
        val cached = EffectiveSelectionUi(
            agentId = "pi", modelId = "pi/glm-5.3", effort = "high",
            source = SelectionSource.THREAD_OVERRIDE, appliesToNextTurn = false,
            explanation = "", availableEfforts = emptyList(),
        )
        val s1 = ModelSelectionReducer.withCachedSelection(
            ModelSelectionReducer.initial().copy(agentId = "pi", threadId = "t1"),
            cached,
        )
        // ① 先缓存:display=上次记录,且必须带「上次记录」标注;还没有 daemon 真值
        assertEquals(cached.modelId, ModelSelectionReducer.display(s1)?.modelId)
        assertTrue(ModelSelectionReducer.display(s1)?.fromCache == true)
        assertNull(s1.selection)
        // ② 后替换:daemon 真值整树替换,缓存标注与缓存本体一并退场
        val s2 = ModelSelectionReducer.applyDaemon(
            s1,
            ModelSelectionReducer.mapSelection(
                sel("pi/glm-5.3", effort = "max", source = "native_current", nextTurn = true, efforts = arrayOf("low", "high", "max")),
            ),
        )
        assertNull(s2.cached)
        val shown = ModelSelectionReducer.display(s2)
        assertNotNull(shown)
        assertEquals("max", shown!!.effort)
        assertEquals(SelectionSource.NATIVE_CURRENT, shown.source)
        assertTrue(shown.appliesToNextTurn)
        assertFalse(shown.fromCache)
        assertFalse(shown.optimistic)
        assertEquals(listOf("low", "high", "max"), shown.availableEfforts)
    }

    @Test
    fun cache_only_state_survives_when_daemon_never_answered() {
        // 真值未达:继续显示"上次记录"(断线时显示最后已知,不空白、不编造)
        val cached = EffectiveSelectionUi(
            agentId = "pi", modelId = "pi/glm-5.3", effort = "",
            source = SelectionSource.THREAD_OVERRIDE, appliesToNextTurn = false,
            explanation = "", availableEfforts = emptyList(),
        )
        val s = ModelSelectionReducer.withCachedSelection(ModelSelectionReducer.initial(), cached)
        assertNull(ModelSelectionReducer.rollback(s.copy(loading = true), "连接中断").selection)
        assertEquals(cached.modelId, ModelSelectionReducer.display(ModelSelectionReducer.rollback(s, "连接中断"))?.modelId)
    }

    // ---- 2) 乐观更新 + 失败回滚 ----

    @Test
    fun select_model_is_optimistic_and_rollback_restores_previous_truth() {
        val truth = ModelSelectionReducer.mapSelection(sel("pi/glm-5.3", effort = "low", source = "desktop_default"))
        val s0 = stateWithTruth(truth)

        val s1 = ModelSelectionReducer.reduce(s0, ModelSelectionIntent.SelectModel("pi/qwen-max"))
        // 乐观投影:显式选择即会话覆盖;轮次红线口径 = 将从下一轮生效
        val optimistic = s1.selection
        assertNotNull(optimistic)
        assertEquals("pi/qwen-max", optimistic!!.modelId)
        assertEquals("low", optimistic.effort) // 无模型表:档位落入内置默认档,low 合法 → 保留
        assertEquals(SelectionSource.THREAD_OVERRIDE, optimistic.source)
        assertTrue(optimistic.appliesToNextTurn)
        assertTrue(optimistic.optimistic)
        // 在飞锚点记着上一份真值
        assertEquals("pi/glm-5.3", s1.pending?.previous?.modelId)

        // 失败回滚:整体退回真值,错误上屏,pending 清空
        val s2 = ModelSelectionReducer.rollback(s1, "切换失败:连接中断")
        assertEquals("pi/glm-5.3", s2.selection?.modelId)
        assertFalse(s2.selection?.optimistic == true)
        assertNull(s2.pending)
        assertEquals("切换失败:连接中断", s2.error)

        // 没有 pending 时 rollback 只标错误,不动投影
        val s3 = ModelSelectionReducer.rollback(s0, "x")
        assertEquals(truth, s3.selection)
        assertEquals("x", s3.error)
    }

    @Test
    fun daemon_truth_backfill_replaces_optimistic_and_clears_pending() {
        val s0 = stateWithTruth(ModelSelectionReducer.mapSelection(sel("pi/glm-5.3", source = "app_default")))
        val s1 = ModelSelectionReducer.reduce(s0, ModelSelectionIntent.SelectEffort("max"))
        assertTrue(s1.selection?.optimistic == true)
        val sent = ModelSelectionReducer.confirmSent(s1)
        assertNull(sent.pending)
        assertTrue(sent.selection?.optimistic == true) // 仍乐观,等回填
        val backfilled = ModelSelectionReducer.applyDaemon(
            sent,
            ModelSelectionReducer.mapSelection(sel("pi/glm-5.3", effort = "max", source = "thread_override", nextTurn = true)),
        )
        assertEquals("max", backfilled.selection?.effort)
        assertFalse(backfilled.selection?.optimistic == true)
        assertTrue(backfilled.selection?.appliesToNextTurn == true)
        assertNull(backfilled.pending)
        assertNull(backfilled.error)
    }

    @Test
    fun same_value_intents_are_idempotent_no_pending() {
        val s0 = stateWithTruth(ModelSelectionReducer.mapSelection(sel("pi/glm-5.3", effort = "high")))
        assertTrue(ModelSelectionReducer.reduce(s0, ModelSelectionIntent.SelectModel("pi/glm-5.3")) === s0)
        assertTrue(ModelSelectionReducer.reduce(s0, ModelSelectionIntent.SelectEffort("high")) === s0)
        // 已是"跟随默认"(双空)时再 Reset 也是 no-op —— 零值状态稳定不抖
        val empty = stateWithTruth(
            ModelSelectionReducer.mapSelection(sel("", effort = "", source = "app_default")),
        )
        assertTrue(ModelSelectionReducer.reduce(empty, ModelSelectionIntent.ResetToDefault) === empty)
    }

    // ---- 3) ResetToDefault 零值语义 ----

    @Test
    fun reset_to_default_clears_override_with_zero_value_semantics() {
        val s0 = stateWithTruth(
            ModelSelectionReducer.mapSelection(sel("pi/glm-5.3", effort = "max", source = "thread_override")),
        )
        val s1 = ModelSelectionReducer.reduce(s0, ModelSelectionIntent.ResetToDefault)
        val sel1 = s1.selection
        assertNotNull(sel1)
        // 零值 = 跟随默认,是合法状态:双双置空、不清成具体值、不报错
        assertEquals("", sel1!!.modelId)
        assertEquals("", sel1.effort)
        assertEquals(SelectionSource.THREAD_OVERRIDE, sel1.source) // 显式选择"跟随默认"仍是会话覆盖
        assertTrue(sel1.appliesToNextTurn)
        assertTrue(sel1.optimistic)
        assertEquals("pi/glm-5.3", s1.pending?.previous?.modelId) // 回滚锚点保留覆盖值
    }

    // ---- 4) 失效 effort 清理 ----

    @Test
    fun stale_effort_cleared_when_new_model_lacks_that_level() {
        // pi 动态档位 low/high/max:原强度 xhigh 不在集合 → 清成空串(跟随默认,零值合法)
        val models = listOf(piModel("low", "high", "max"))
        val s0 = stateWithTruth(
            ModelSelectionReducer.mapSelection(sel("pi/other", effort = "xhigh", source = "thread_override")),
            models,
        )
        val switched = ModelSelectionReducer.reduce(s0, ModelSelectionIntent.SelectModel("pi/glm-5.3"))
        assertTrue(switched.selection?.optimistic == true)
        assertEquals("pi/glm-5.3", switched.selection?.modelId)
        assertEquals("", switched.selection?.effort) // 失效强度不保留,清成"跟随默认"

        // 新模型支持该强度 → 原值保留
        val s1 = stateWithTruth(
            ModelSelectionReducer.mapSelection(sel("pi/other", effort = "high", source = "thread_override")),
            models,
        )
        val kept = ModelSelectionReducer.reduce(s1, ModelSelectionIntent.SelectModel("pi/glm-5.3"))
        assertEquals("high", kept.selection?.effort)

        // 数据层规则直测(与上面 reduce 级行为同源):
        assertEquals("", ModelSelectionReducer.cleanEffortFor(models, "pi", "xhigh", "pi/glm-5.3"))
        assertEquals("high", ModelSelectionReducer.cleanEffortFor(models, "pi", "high", "pi/glm-5.3"))
        assertEquals("", ModelSelectionReducer.cleanEffortFor(models, "pi", "", "pi/glm-5.3"))
    }

    @Test
    fun effort_cleanup_rules_match_agent_capability_families() {
        // 档位制 agent(pi,同 agent 任一模型带档位)的已知模型没带档位 → 只剩"跟随默认":失效强度必须清
        val piFamily = listOf(
            piModel("low", "high"),
            ModelInfo.newBuilder().setId("pi/private/x").setProvider("pi").build(),
        )
        assertEquals("", ModelSelectionReducer.cleanEffortFor(piFamily, "pi", "max", "pi/private/x"))
        // 非档位 agent(codex 系,从不透出档位)→ 内置默认档:low/medium/high/xhigh 都合法,保留
        val codex = listOf(ModelInfo.newBuilder().setId("codex/gpt-5.4").setProvider("codex").build())
        assertEquals("xhigh", ModelSelectionReducer.cleanEffortFor(codex, "codex", "xhigh", "codex/gpt-5.4"))
        assertEquals("", ModelSelectionReducer.cleanEffortFor(codex, "codex", "max", "codex/gpt-5.4"))
        // 未知模型且 daemon 没给档位 → 无法判定,保留原值等真值回填校正(不猜)
        val empty: List<ModelInfo> = emptyList()
        assertEquals("high", ModelSelectionReducer.cleanEffortFor(empty, "pi", "high", "pi/unknown"))
    }

    // ---- 5) 未知 effort 保留原 id 展示 ----

    @Test
    fun unknown_effort_keeps_raw_id_in_available_efforts_and_optimistic_state() {
        // daemon 未来新增档位:归一只去空/去重,绝不丢原 id(展示层 else -> id 兜底)
        val normalized = ModelSelectionReducer.normalizeEfforts(listOf("low", "ultra", "low", "", "max"))
        assertEquals(listOf("low", "ultra", "max"), normalized)

        val s0 = stateWithTruth(
            ModelSelectionReducer.mapSelection(sel("pi/glm-5.3", effort = "low", efforts = arrayOf("low", "ultra"))),
        )
        val s1 = ModelSelectionReducer.reduce(s0, ModelSelectionIntent.SelectEffort("ultra"))
        assertEquals("ultra", s1.selection?.effort) // 未知档位原 id 保留,不清洗、不拒绝
        assertEquals(listOf("low", "ultra"), s1.selection?.availableEfforts)
    }

    // ---- 7) 旧 daemon 兜底(不报错) ----

    @Test
    fun legacy_list_models_falls_back_to_current_model_and_effort() {
        // 旧 daemon:effective_selection(5) 缺席 → 按 current_model_id(2)/current_effort(3) 兜底
        val legacy = ListModelsResponse.newBuilder()
            .setCurrentModelId("pi/zai-coding-cn/glm-5.3")
            .setCurrentEffort("high")
            .build()
        val mapped = ModelSelectionReducer.fromListModelsResponse(legacy, "pi")
        assertEquals("pi/zai-coding-cn/glm-5.3", mapped.modelId)
        assertEquals("high", mapped.effort)
        assertEquals(SelectionSource.NATIVE_CURRENT, mapped.source)
        // current_effort 是 optional:显式缺席 = 空(跟随默认语义),不报错
        val noEffort = ListModelsResponse.newBuilder().setCurrentModelId("codex/gpt-5.4").build()
        assertEquals("", ModelSelectionReducer.fromListModelsResponse(noEffort, "codex").effort)
    }

    @Test
    fun legacy_thread_falls_back_to_model_id_and_effort_fields() {
        // 旧 daemon:effective_selection(24) 缺席 → 按 model_id(5)/effort(19) 兜底
        val legacy = pocketagent.v1.Thread.newBuilder()
            .setAgentId("pi")
            .setModelId("pi/zai-coding-cn/glm-5.3")
            .setEffort("max")
            .build()
        val mapped = ModelSelectionReducer.fromThread(legacy)
        assertEquals("pi/zai-coding-cn/glm-5.3", mapped.modelId)
        assertEquals("max", mapped.effort)
        assertEquals(SelectionSource.NATIVE_CURRENT, mapped.source)
        assertEquals("pi", mapped.agentId)
    }

    @Test
    fun merge_prefers_list_models_truth_then_thread_truth() {
        val fromList = ModelSelectionReducer.mapSelection(
            sel("pi/glm-5.3", effort = "high", source = "thread_override", efforts = arrayOf("low", "high")),
        )
        val fromThread = ModelSelectionReducer.mapSelection(sel("pi/glm-5.3", effort = "low", source = "native_current"))
        // ListModels(5) 带 available_efforts,优先
        assertEquals("high", ModelSelectionReducer.merge(fromList, fromThread)?.effort)
        // ListModels 缺席(如该 RPC 失败只拿到 Thread)→ 退 Thread(24)
        assertEquals("low", ModelSelectionReducer.merge(null, fromThread)?.effort)
        assertNull(ModelSelectionReducer.merge(null, null))
    }
}
