package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 长按锚点菜单纯决策钉子(2026-09-30 微交互轮,ContextMenuLogic):
 *  1) 危险项红色置底:破坏性动作永远离常规操作最远 —— 排序错了的表现是
 *     "删除混在复制/转发中间",无声误触,故钉死;
 *  2) 防出屏钳制:锚点右下优先、越界翻转、翻转也救不了时安全边兜底 ——
 *     定位错了的表现是"菜单被屏幕边缘裁掉一半",同样无声,故钉死。
 */
class ContextMenuLogicTest {

    private fun item(id: String, destructive: Boolean = false) =
        ContextMenuItem(id = id, label = id, destructive = destructive)

    // ---- 危险项置底(稳定排序) ----

    @Test
    fun `危险项置底且常规项保持原序`() {
        val items = listOf(
            item("copy"), item("delete", destructive = true),
            item("forward"), item("clear", destructive = true), item("reply"),
        )
        val got = ContextMenuLogic.orderedContextMenuItems(items)
        assertEquals(listOf("copy", "forward", "reply", "delete", "clear"), got.map { it.id })
    }

    @Test
    fun `全危险项与全常规项都保持原序`() {
        val allBad = listOf(item("b2", destructive = true), item("b1", destructive = true))
        assertEquals(listOf("b2", "b1"), ContextMenuLogic.orderedContextMenuItems(allBad).map { it.id })
        val allGood = listOf(item("a"), item("b"))
        assertEquals(listOf("a", "b"), ContextMenuLogic.orderedContextMenuItems(allGood).map { it.id })
        assertEquals(emptyList<String>(), ContextMenuLogic.orderedContextMenuItems(emptyList()).map { it.id })
    }

    // ---- 防出屏钳制 ----

    // 统一几何:屏 1000x2000、安全边 12、菜单 220x200
    private val screenW = 1000f
    private val screenH = 2000f
    private val menuW = 220f
    private val menuH = 200f
    private val safe = 12f

    @Test
    fun `屏中锚点右下原地展开_不翻转`() {
        val p = ContextMenuLogic.menuPlacement(500f, 500f, menuW, menuH, screenW, screenH, safe)
        assertEquals(500f, p.x, 1e-4f)
        assertEquals(500f, p.y, 1e-4f)
        assertFalse(p.flipX)
        assertFalse(p.flipY)
    }

    @Test
    fun `右缘越界左翻_下缘越界上翻`() {
        // 水平:x + 220 = 1120 > 988 → 翻到锚点左侧
        val rx = ContextMenuLogic.menuPlacement(900f, 500f, menuW, menuH, screenW, screenH, safe)
        assertTrue(rx.flipX)
        assertFalse(rx.flipY)
        assertEquals(900f - menuW, rx.x, 1e-4f)
        // 垂直:y + 200 = 2100 > 1988 → 翻到锚点上方
        val by = ContextMenuLogic.menuPlacement(500f, 1900f, menuW, menuH, screenW, screenH, safe)
        assertFalse(by.flipX)
        assertTrue(by.flipY)
        assertEquals(1900f - menuH, by.y, 1e-4f)
    }

    @Test
    fun `角上锚点双向翻转且钳回安全边`() {
        // 右下角:双向翻转后坐标仍压线 → 钳到 (screen - safe - menu)
        val p = ContextMenuLogic.menuPlacement(990f, 1990f, menuW, menuH, screenW, screenH, safe)
        assertTrue(p.flipX)
        assertTrue(p.flipY)
        assertEquals(screenW - safe - menuW, p.x, 1e-4f)
        assertEquals(screenH - safe - menuH, p.y, 1e-4f)
    }

    @Test
    fun `极端小窗菜单比屏宽高_坐标收敛到安全边不为负`() {
        // 屏 200x250 < 菜单 220x200(双向都放不下):翻转也救不了,上下界收敛,钳在安全边
        val p = ContextMenuLogic.menuPlacement(50f, 50f, menuW, menuH, 200f, 250f, safe)
        assertEquals(safe, p.x, 1e-4f)
        assertEquals(safe, p.y, 1e-4f)
        assertTrue(p.flipX)
        assertTrue(p.flipY)
    }

    @Test
    fun `锚点越出屏幕时坐标仍钳在屏内`() {
        // 手势坐标异常(负值/超屏)不产生屏外菜单
        val p = ContextMenuLogic.menuPlacement(-500f, -500f, menuW, menuH, screenW, screenH, safe)
        assertEquals(safe, p.x, 1e-4f)
        assertEquals(safe, p.y, 1e-4f)
        assertFalse(p.flipX) // -500 + 220 < 988:右下放得下,但坐标被钳回
        val q = ContextMenuLogic.menuPlacement(5000f, 5000f, menuW, menuH, screenW, screenH, safe)
        assertEquals(screenW - safe - menuW, q.x, 1e-4f)
        assertEquals(screenH - safe - menuH, q.y, 1e-4f)
    }

    @Test
    fun `翻转判定用等号边界_恰好压线算放得下`() {
        // x + menuW == screenW - safe:恰好贴安全边,不翻转(条件是严格大于)
        val p = ContextMenuLogic.menuPlacement(
            screenW - safe - menuW, 500f, menuW, menuH, screenW, screenH, safe,
        )
        assertFalse(p.flipX)
        assertEquals(screenW - safe - menuW, p.x, 1e-4f)
    }
}
