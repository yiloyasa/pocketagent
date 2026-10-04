package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.model.cardIsFollowDefault
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ModelInfo

/**
 * 会话级模型路由 + 会话操作文案 + 上下文窗口来源的纯函数回归
 * (真机实锤 2026-09-19:图3 模型芯片/弹窗串到别家模型表、图9 "让 Codex 压缩"
 * 出现在 Pi 会话、图10 Pi 会话用量恒 0、codex 窗口必须是上报真值 258400)。
 */
class ChatModelRoutingTest {

    private fun model(id: String, name: String = "", window: Int = 0): ModelInfo =
        ModelInfo.newBuilder()
            .setId(id)
            .setDisplayName(name)
            .setContextWindow(window)
            .build()

    /** daemon ListModels 的全局形状:各 provider 的模型 id 带各自 agent 前缀。 */
    private val globalList = listOf(
        model("codex/GLM-5.3-Flash", "GLM-5.3-Flash"),
        model("codex/default", "Codex(跟随桌面配置)"),
        model("pi/default", "Pi(跟随 Pi 当前配置)"),
        model("pi/zai-coding-cn/glm-5.3", "GLM-5.3", 1_000_000),
        model("zcode/gpt-5.6-sol", "GPT-5.6 Sol"),
    )

    // ---- modelsForAgent:模型表按会话归属 agent 过滤(图3 根因修复)----

    @Test
    fun pi_session_sees_only_pi_models() {
        val got = modelsForAgent(globalList, "pi")
        assertEquals(listOf("pi/default", "pi/zai-coding-cn/glm-5.3"), got.map { it.id })
    }

    @Test
    fun codex_session_sees_only_codex_models() {
        val got = modelsForAgent(globalList, "codex")
        assertEquals(listOf("codex/GLM-5.3-Flash", "codex/default"), got.map { it.id })
    }

    @Test
    fun zcode_session_sees_only_zcode_models() {
        val got = modelsForAgent(globalList, "zcode")
        assertEquals(listOf("zcode/gpt-5.6-sol"), got.map { it.id })
    }

    /** 会话归属还没回填(getThread 未返回)时保持旧行为:全表展示。 */
    @Test
    fun blank_agent_keeps_full_list() {
        assertEquals(globalList.map { it.id }, modelsForAgent(globalList, "").map { it.id })
    }

    /** 未知 agent:空表,绝不拿别家模型凑数。 */
    @Test
    fun unknown_agent_gets_empty_list() {
        assertTrue(modelsForAgent(globalList, "hermes").isEmpty())
    }

    // ---- modelBareName:选中态去掉 agent 前缀显示 ----

    @Test
    fun bare_name_strips_agent_prefix() {
        assertEquals("glm-5.3", modelBareName("pi/glm-5.3"))
        // 三段式只去首段 agent 前缀,余下原样(与 daemon Thread.ModelID 形状一致)
        assertEquals("zai-coding-cn/glm-5.3", modelBareName("pi/zai-coding-cn/glm-5.3"))
        assertEquals("GLM-5.3-Flash", modelBareName("codex/GLM-5.3-Flash"))
    }

    @Test
    fun bare_name_keeps_plain_id_intact() {
        assertEquals("glm-5.3", modelBareName("glm-5.3"))
        assertEquals("", modelBareName("pi/"))
    }

    // ---- compactionConsequenceCopy:压缩文案按会话 agent 路由(图9)----

    @Test
    fun compaction_copy_names_the_thread_agent() {
        assertEquals(
            "让 Pi 压缩历史上下文以释放窗口空间,不会撤销文件改动。",
            compactionConsequenceCopy("pi"),
        )
        assertEquals(
            "让 Codex 压缩历史上下文以释放窗口空间,不会撤销文件改动。",
            compactionConsequenceCopy("codex"),
        )
        assertEquals(
            "让 ZCode 压缩历史上下文以释放窗口空间,不会撤销文件改动。",
            compactionConsequenceCopy("zcode"),
        )
    }

    /** agent 还没回填:中性文案,绝不点名(不冒充 codex)。 */
    @Test
    fun compaction_copy_stays_neutral_for_unknown_agent() {
        assertEquals(
            "压缩历史上下文以释放窗口空间,不会撤销文件改动。",
            compactionConsequenceCopy(""),
        )
    }

    // ---- 窗口来源(图10 / codex 258400)----

    /** codex:模型列表不声明窗口,258400 只随 turn 用量(model_context_window)上报 —— live 必须赢。 */
    @Test
    fun codex_window_is_the_reported_live_value() {
        val w = pickContextWindow(live = 258_400L, modelDeclared = 0, agentDefaultDeclared = 0)
        assertEquals(258_400L, w)
    }

    /** pi:首轮用量推送前用 pi 模型自报窗口(get_available_models 真值),显示 0/1,000,000 而非"未知"。 */
    @Test
    fun pi_window_falls_back_to_declared_window_until_usage_arrives() {
        val w = pickContextWindow(live = 0L, modelDeclared = 1_000_000, agentDefaultDeclared = 0)
        assertEquals(1_000_000L, w)
    }

    /** pi 用量推送到达后(live=stats 的 contextUsage.contextWindow),live 覆盖声明值。 */
    @Test
    fun pi_live_window_overrides_declared_once_usage_arrives() {
        val w = pickContextWindow(live = 983_040L, modelDeclared = 1_000_000, agentDefaultDeclared = 0)
        assertEquals(983_040L, w)
    }

    // ---- 模型卡「跟随默认」判定(P3-T4:选中态来自仓库 display 的路由语义)----

    /**
     * 卡上唯一能回到"跟随默认"的路是 ResetToDefault(model/effort 双空,零值合法),
     * 「跟随默认」行的高亮判定必须同时认两种形状:空 id(零值语义,daemon 钉死
     * 的合法状态)与显式的 $agent/default 条目;具体模型与**别家的** default
     * 条目都不算本会话跟随(pi 会话里出现 codex/default 不得误高亮)。
     */
    @Test
    fun follow_default_detection_covers_zero_value_and_own_default_entry() {
        assertTrue(cardIsFollowDefault("", "pi"))
        assertTrue(cardIsFollowDefault("pi/default", "pi"))
        assertFalse(cardIsFollowDefault("pi/zai-coding-cn/glm-5.3", "pi"))
        assertFalse(cardIsFollowDefault("codex/default", "pi"))
        // agent 未回填(空):只有空 id 算跟随,任何具名 default 都不算
        assertTrue(cardIsFollowDefault("", ""))
        assertFalse(cardIsFollowDefault("/default", ""))
    }
}
