package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 抽屉导航条目集合恒定的单测(2026-10-09 恒定修复):
 * 修上轮盘点实锤的「同一分钟内多 Agent 协同条目时有时无」—— 条目集合不得随
 * 调用方接线状态增减,未接线一律置灰而非消失。
 */
class DrawerConstancyTest {

    /** 四种接线组合全遍历:消息中心/协同各自的回调有/无。 */
    private val wiringCombos: List<Pair<Boolean, Boolean>> = listOf(
        true to true,
        true to false,
        false to true,
        false to false,
    )

    @Test
    fun `条目集合恒定_任意接线组合下id序列与总数不变`() {
        val baseline = buildDrawerNavEntries(true, true).map { it.spec.id }
        wiringCombos.forEach { (attention, collab) ->
            val entries = buildDrawerNavEntries(attention, collab)
            assertEquals("接线组合 ($attention, $collab) 下条目总数漂移", 7, entries.size)
            assertEquals("接线组合 ($attention, $collab) 下条目顺序漂移", baseline, entries.map { it.spec.id })
        }
    }

    @Test
    fun `规格清单_顺序与归属稳定`() {
        assertEquals(
            listOf("home", "threads", "taskCenter", "files", "git", "attention", "collab"),
            drawerNavEntrySpecs().map { it.id },
        )
        // 主导航 tab 归属不变(底部页签零回归);动作项 tab=-1
        assertEquals(listOf(0, 1, 4, 2, 3), drawerNavEntrySpecs().take(5).map { it.tab })
        assertEquals(listOf(-1, -1), drawerNavEntrySpecs().takeLast(2).map { it.tab })
    }

    @Test
    fun `主导航五项_永远可用_原因词恒空`() {
        wiringCombos.forEach { (attention, collab) ->
            buildDrawerNavEntries(attention, collab).take(5).forEach { e ->
                assertTrue("「${e.spec.title}」不应置灰", e.enabled)
                assertNull("「${e.spec.title}」不应有原因词", e.disabledHint)
            }
        }
    }

    @Test
    fun `消息中心_接线可用_未接线置灰带真实原因`() {
        val wired = buildDrawerNavEntries(attentionWired = true, collabWired = true)
            .first { it.spec.id == "attention" }
        assertTrue(wired.enabled)
        assertNull(wired.disabledHint)

        val unwired = buildDrawerNavEntries(attentionWired = false, collabWired = true)
            .first { it.spec.id == "attention" }
        assertFalse(unwired.enabled)
        assertEquals(DRAWER_HINT_ATTENTION_OFF, unwired.disabledHint)
        assertEquals("在主页查看", DRAWER_HINT_ATTENTION_OFF)
    }

    @Test
    fun `协同_接线可用_未接线置灰带真实原因`() {
        val wired = buildDrawerNavEntries(attentionWired = true, collabWired = true)
            .first { it.spec.id == "collab" }
        assertTrue(wired.enabled)
        assertNull(wired.disabledHint)

        val unwired = buildDrawerNavEntries(attentionWired = true, collabWired = false)
            .first { it.spec.id == "collab" }
        assertFalse(unwired.enabled)
        assertEquals(DRAWER_HINT_COLLAB_OFF, unwired.disabledHint)
        assertEquals("未接入", DRAWER_HINT_COLLAB_OFF)
    }

    @Test
    fun `置灰互不串扰_只影响各自的条目`() {
        val onlyCollab = buildDrawerNavEntries(attentionWired = false, collabWired = true)
        assertFalse(onlyCollab.first { it.spec.id == "attention" }.enabled)
        assertTrue(onlyCollab.first { it.spec.id == "collab" }.enabled)

        val onlyAttention = buildDrawerNavEntries(attentionWired = true, collabWired = false)
        assertTrue(onlyAttention.first { it.spec.id == "attention" }.enabled)
        assertFalse(onlyAttention.first { it.spec.id == "collab" }.enabled)
    }
}
