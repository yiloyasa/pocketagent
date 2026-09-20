package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.CodexCapability

/**
 * WorkflowB 单测:能力分组映射(项 3a)+ 设置项状态映射(项 3b)的纯逻辑。
 *
 * 【重点钉子】未知 kind 不丢弃、未知 state 不崩 —— 这两条都是"扩展性"的硬要求,
 * 也是最容易在下次改动中被破坏的地方。
 */
class WorkflowBCapabilityTest {

    private fun cap(kind: String, name: String, source: String = ""): CodexCapability =
        CodexCapability.newBuilder().setKind(kind).setName(name).setSource(source).build()

    // ---- 项 3a:kind 映射 ----

    @Test
    fun `known kind 用中文名, unknown kind 原样显示而不是丢弃`() {
        assertEquals("技能", capabilityKindLabel("skill"))
        assertEquals("Hooks", capabilityKindLabel("hook"))
        assertEquals("插件", capabilityKindLabel("plugin"))
        // 未知类型:必须原样显示,而不是空串 / "其他"把信息抹掉
        assertEquals("subagent", capabilityKindLabel("subagent"))
        assertEquals("其他", capabilityKindLabel(""))
    }

    @Test
    fun `kind 顺序：已知固定序在前, 未知排序附后且不丢失`() {
        val order = capabilityKindOrder(listOf("plugin", "subagent", "skill", "hook", "zzz"))
        assertEquals(listOf("skill", "hook", "plugin", "subagent", "zzz"), order)
    }

    @Test
    fun `groupCapabilities 按固定顺序分组, 组内按名称稳定排序`() {
        val items = listOf(
            cap("plugin", "zeta"),
            cap("skill", "b-skill"),
            cap("skill", "A-skill"),
            cap("weird", "x"),
        )
        val groups = groupCapabilities(items)
        assertEquals(listOf("skill", "plugin", "weird"), groups.map { it.first })
        // 组内不区分大小写排序
        assertEquals(listOf("A-skill", "b-skill"), groups[0].second.map { it.name })
        // 未知 kind 的条目仍在结果里
        assertEquals("x", groups[2].second.single().name)
    }

    @Test
    fun `filterCapabilities 按名称或来源过滤, 空查询原样返回`() {
        val items = listOf(
            cap("skill", "Alpha", "workspace"),
            cap("skill", "beta", "marketplace"),
            cap("plugin", "Gamma", "marketplace"),
        )
        assertEquals(3, filterCapabilities(items, "").size)
        assertEquals(3, filterCapabilities(items, "   ").size)
        assertEquals(listOf("Alpha"), filterCapabilities(items, "alph").map { it.name })
        assertEquals(listOf("beta", "Gamma"), filterCapabilities(items, "MARKET").map { it.name })
    }

    // ---- 项 3b:设置项状态映射 ----

    @Test
    fun `五种已知 state 各自映射到正确的语义`() {
        assertEquals("已生效", settingStateInfo("applied").text)
        assertEquals("ok", settingStateInfo("applied").level)
        assertEquals("已保存,未生效", settingStateInfo("saved_not_applied").text)
        assertEquals("warn", settingStateInfo("saved_not_applied").level)
        assertEquals("当前 codex 版本不支持该项", settingStateInfo("unsupported").text)
        assertEquals("连接失败", settingStateInfo("failed").text)
        assertEquals("error", settingStateInfo("failed").level)
        assertEquals("跟随电脑端配置", settingStateInfo("follow_desktop").text)
    }

    @Test
    fun `未知 state 兜底显示原文而不是崩或空白`() {
        val info = settingStateInfo("needs_model_support")
        assertEquals("needs_model_support", info.text)
        assertEquals("muted", info.level)
        // 空 state = daemon 缺席,人话口径"状态暂不可用",不显「未知」(AUDIT §3.3)
        assertEquals("状态暂不可用", settingStateInfo("").text)
    }

    @Test
    fun `全部状态文案都不含未知字样_AUDIT_3_3红线`() {
        // 已知分支 + 空串 + 未知取值兜底,逐一断言不含「未知」且非空白
        val states = listOf(
            "", "applied", "saved_not_applied", "unsupported", "failed",
            "unreachable", "follow_desktop", "locked", "pending", "needs_model_support",
        )
        states.forEach { s ->
            val text = settingStateInfo(s).text
            assertFalse("用户可见文案不得含「未知」: '$s' -> $text", text.contains("未知"))
            assertTrue("文案不得为空白: '$s' -> $text", text.isNotBlank())
        }
    }

    @Test
    fun `overallSettingNote：只统计可恢复问题, unsupported 与 follow_desktop 不触发横幅`() {
        assertNull(overallSettingNote(listOf("applied", "applied", "follow_desktop")))
        assertNull(overallSettingNote(emptyList()))
        // unsupported 是静态事实、不可重试,已在该项那一行显示 → 不顶横幅
        assertNull(overallSettingNote(listOf("applied", "unsupported")))
        assertNull(overallSettingNote(listOf("follow_desktop", "unsupported")))
        // 可恢复的问题才提示
        val note = overallSettingNote(listOf("applied", "unsupported", "failed", "saved_not_applied"))
        assertEquals(true, note != null)
        assertTrue(note!!.contains("1 项连接失败"))
        assertTrue(note.contains("1 项尚未生效"))
    }

    // ---- 项 3b 第二轮:下拉"跟随桌面"追加 + 开关显示态(可单测的纯函数) ----

    @Test
    fun `withFollowDesktop 空列表得到恰好一项且 id 为空串`() {
        val r = withFollowDesktop(emptyList())
        assertEquals(1, r.size)
        assertEquals("", r[0].first)
        assertEquals("跟随桌面", r[0].second)
    }

    @Test
    fun `withFollowDesktop 非空时新项追加在末尾且总数加一`() {
        val base = listOf(
            Triple("low", "低", "d1"),
            Triple("high", "高", "d2"),
        )
        val r = withFollowDesktop(base)
        assertEquals(3, r.size)
        assertEquals("", r.last().first)
        // 原候选原样保留、顺序不变,只有末尾多出"跟随桌面"
        assertEquals(base, r.dropLast(1))
    }

    @Test
    fun `withFollowDesktop 入参已含空串时不产生重复项`() {
        val base = listOf(
            Triple("low", "低", "d1"),
            Triple("", "跟随桌面", "x"),
        )
        val r = withFollowDesktop(base)
        assertEquals(2, r.size)
        assertEquals(1, r.count { it.first.isEmpty() })
    }

    @Test
    fun `switchChecked 不支持的项永不为真`() {
        assertEquals(false, switchChecked(raw = true, unsupported = true))
        assertEquals(true, switchChecked(raw = true, unsupported = false))
        assertEquals(false, switchChecked(raw = false, unsupported = true))
        assertEquals(false, switchChecked(raw = false, unsupported = false))
    }

    // ---- 第 3 轮:unreachable(codex 一时没响应)的文案 ----

    @Test
    fun `unreachable 映射为可重试的轻语气而不是连接失败`() {
        val info = settingStateInfo("unreachable")
        assertEquals("warn", info.level)
        assertTrue(info.text.contains("可重试"))
    }

    @Test
    fun `overallSettingNote 把 unreachable 说成尚未确认而不是连接失败`() {
        val note = overallSettingNote(listOf("unreachable", "unreachable", "applied"))
        assertEquals(true, note != null)
        assertTrue(note!!.contains("2 项尚未确认,可重试"))
        assertEquals(false, note.contains("连接失败"))
    }

    @Test
    fun `overallSettingNote 仍不因 unreachable 之外的无害状态触发`() {
        assertNull(overallSettingNote(listOf("applied", "unsupported", "follow_desktop")))
    }
}
