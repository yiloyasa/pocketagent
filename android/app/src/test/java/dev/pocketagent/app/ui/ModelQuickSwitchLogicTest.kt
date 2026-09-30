package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ModelInfo

/**
 * 模型行「快速切换面板」数据决策钉子(2026-09-30 模型选择 UI 优化):
 * 最近用过(仍可用)MRU 优先 → 可用模型原序补足 → 截到 8 个;
 * "最近"只记真实选择,空串/默认条目不进面板。数据错了的表现是
 * "快速面板切到切不回去的幽灵模型"或"面板里混进跟随桌面",均无声,故钉死。
 */
class ModelQuickSwitchLogicTest {

    private fun model(id: String, name: String = "", window: Int = 0, thinking: Boolean = false): ModelInfo =
        ModelInfo.newBuilder()
            .setId(id)
            .setDisplayName(name)
            .setContextWindow(window)
            .setSupportsThinking(thinking)
            .build()

    private val piModels = listOf(
        model("pi/default", "Pi(跟随 Pi 当前配置)"),
        model("pi/glm-5.3", "GLM-5.3", 1_000_000, thinking = true),
        model("pi/dsv4-flash", "DeepSeek-V41-Flash", 128_000),
        model("pi/kimi", "Kimi"),
    )

    // ---- quickModels:无"最近"时的兜底 ----

    @Test
    fun `无最近记录时按可用模型原序补足且排除 default`() {
        val got = ModelQuickSwitchLogic.quickModels("pi", piModels)
        // pi/default 不进快速面板(跟随桌面的语义在完整卡)
        assertEquals(listOf("pi/glm-5.3", "pi/dsv4-flash", "pi/kimi"), got.map { it.id })
    }

    @Test
    fun `可用表超 8 个时截断到 8`() {
        val many = (1..12).map { model("pi/m$it") }
        val got = ModelQuickSwitchLogic.quickModels("pi", many)
        assertEquals(ModelQuickSwitchLogic.MAX_QUICK, got.size)
        assertEquals("pi/m1", got.first().id)
        assertEquals("pi/m8", got.last().id)
    }

    @Test
    fun `可用表为空返回空面板数据`() {
        assertTrue(ModelQuickSwitchLogic.quickModels("pi", emptyList()).isEmpty())
    }

    @Test
    fun `标签与副文案如实组装`() {
        val got = ModelQuickSwitchLogic.quickModels("pi", listOf(model("pi/glm-5.3", "GLM-5.3", 1_000_000, thinking = true)))
        assertEquals("GLM-5.3", got[0].label)
        assertEquals("上下文 1000k · 支持思考", got[0].detail)
    }

    @Test
    fun `displayName 为空时标签退裸名`() {
        val got = ModelQuickSwitchLogic.quickModels("pi", listOf(model("pi/zai-cn/glm-5.3")))
        assertEquals("zai-cn/glm-5.3", got[0].label) // 只去首段 agent 前缀,与 modelBareName 同口径
    }

    // ---- rememberPick + MRU 排序 ----

    @Test
    fun `最近使用的排最前且保持 MRU 序`() {
        // 用独立 agent id,避免与其他用例共享进程内记忆串味
        val agent = "pi-mru-a"
        ModelQuickSwitchLogic.rememberPick(agent, "pi/dsv4-flash")
        ModelQuickSwitchLogic.rememberPick(agent, "pi/glm-5.3")
        ModelQuickSwitchLogic.rememberPick(agent, "pi/kimi")
        val got = ModelQuickSwitchLogic.quickModels(agent, piModels)
        // MRU 段(最新在前):kimi → glm-5.3 → dsv4-flash;剩余可用只有 default,不进面板
        assertEquals(
            listOf("pi/kimi", "pi/glm-5.3", "pi/dsv4-flash"),
            got.map { it.id },
        )
    }

    @Test
    fun `重复选择去重并前移`() {
        val agent = "pi-mru-b"
        ModelQuickSwitchLogic.rememberPick(agent, "pi/a")
        ModelQuickSwitchLogic.rememberPick(agent, "pi/b")
        ModelQuickSwitchLogic.rememberPick(agent, "pi/a")
        assertEquals(listOf("pi/a", "pi/b"), ModelQuickSwitchLogic.recentOf(agent))
    }

    @Test
    fun `空串与 default 选择不进最近记忆`() {
        val agent = "pi-mru-c"
        ModelQuickSwitchLogic.rememberPick(agent, "")
        ModelQuickSwitchLogic.rememberPick(agent, "pi/default")
        ModelQuickSwitchLogic.rememberPick(agent, "pi/real")
        assertEquals(listOf("pi/real"), ModelQuickSwitchLogic.recentOf(agent))
    }

    @Test
    fun `失效的最近项被淘汰不占面板位`() {
        val agent = "pi-mru-d"
        ModelQuickSwitchLogic.rememberPick(agent, "pi/gone")
        ModelQuickSwitchLogic.rememberPick(agent, "pi/kimi")
        val got = ModelQuickSwitchLogic.quickModels(agent, piModels)
        assertEquals("pi/kimi", got.first().id) // 唯一存活的 MRU 项
        assertTrue("幽灵模型不得出现在面板", got.none { it.id == "pi/gone" })
    }

    @Test
    fun `记忆容量上限 8 只保留最新`() {
        val agent = "pi-mru-e"
        (1..12).forEach { ModelQuickSwitchLogic.rememberPick(agent, "pi/m$it") }
        val recent = ModelQuickSwitchLogic.recentOf(agent)
        assertEquals(ModelQuickSwitchLogic.MAX_QUICK, recent.size)
        assertEquals("pi/m12", recent.first()) // 最新在前
        assertEquals("pi/m5", recent.last()) // 最老的被挤出
    }

    @Test
    fun `未知 agent 的最近记录为空`() {
        assertTrue(ModelQuickSwitchLogic.recentOf("never-picked-agent").isEmpty())
    }
}
