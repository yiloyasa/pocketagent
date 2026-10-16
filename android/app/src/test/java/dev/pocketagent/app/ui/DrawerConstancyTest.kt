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
        // B4e 起规格清单含 flutter(末位动作项);原七项的顺序与归属逐字不变。
        assertEquals(
            listOf("home", "threads", "taskCenter", "files", "git", "attention", "collab", DRAWER_ID_FLUTTER),
            drawerNavEntrySpecs().map { it.id },
        )
        // 主导航 tab 归属不变(底部页签零回归);动作项 tab=-1
        assertEquals(listOf(0, 1, 4, 2, 3), drawerNavEntrySpecs().take(5).map { it.tab })
        assertEquals(listOf(-1, -1, -1), drawerNavEntrySpecs().takeLast(3).map { it.tab })
    }

    // ── B4e:Flutter UI 入口(可空门控,与 attention/collab 的置灰纪律不同) ──
    //
    // 【为什么 flutter 未接时是「不渲染」而不是「置灰」】attention/collab 未接时
    // 置灰+原因词,是因为它们的目标页在 APK 里、只是调用方没接线;Flutter UI 的
    // 方法面/事件面刚接上(B4b/B4d 在途)、真机冷启未验,摆一个置灰项等于向用户
    // 宣告「有个新 UI 但用不了」——不如等接线后随回调一起出现。故 flutter 不在
    // 「恒定集合」里:未接不渲染、接了才出现(下面的测试钉死这两条)。

    @Test
    fun `Flutter入口_未接不渲染_接了才出现_且恒在末位`() {
        // 未接:七项恒定集合里没有 flutter(不与置灰项混迹)
        val unwired = buildDrawerNavEntries(true, true)
        assertTrue("未接回调时 flutter 条目不应渲染", unwired.none { it.spec.id == DRAWER_ID_FLUTTER })
        assertEquals("未接时仍是原七项", 7, unwired.size)

        // 接了:末位出现、可用、无原因词
        val wired = buildDrawerNavEntries(true, true, flutterWired = true)
        val flutter = wired.last()
        assertEquals("flutter 应排末位", DRAWER_ID_FLUTTER, flutter.spec.id)
        assertTrue("接了就是真入口", flutter.enabled)
        assertNull("真入口不挂原因词", flutter.disabledHint)
        assertEquals("接了才多这一项", 8, wired.size)
    }

    @Test
    fun `Flutter入口_接线不影响其余七项的集合与顺序`() {
        val seven = listOf("home", "threads", "taskCenter", "files", "git", "attention", "collab")
        val wired = buildDrawerNavEntries(true, true, flutterWired = true).map { it.spec.id }
        assertEquals("flutter 只追加不移位", seven + DRAWER_ID_FLUTTER, wired)
    }

    @Test
    fun `Flutter入口_文案如实标注开发中`() {
        val spec = drawerNavEntrySpecs().first { it.id == DRAWER_ID_FLUTTER }
        assertTrue("文案须含「开发中」(不制造已上线预期)", spec.title.contains("开发中"))
        assertFalse("文案不得含「已上线/新版/全新」等宣传词",
            spec.title.contains("已上线") || spec.title.contains("新版") || spec.title.contains("全新"))
        // 动作项:无 tab 选中
        assertEquals(-1, spec.tab)
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
