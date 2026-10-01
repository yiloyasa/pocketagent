package dev.pocketagent.app.ui.component

/**
 * 上下文菜单(长按锚点菜单,ContextMenu.kt)的纯决策层 —— 无 Composable/Android
 * 依赖,JVM 单测钉住(ContextMenuLogicTest)。
 *
 * 本轮(微交互质感轮)从 ContextMenu.kt 抽出来的两件事:
 *   1. **危险项红色置底**:破坏性动作永远排在菜单最后(误触距离最远),其余项
 *     保持传入顺序 —— 排序错了的表现是"删除项混在常规操作中间",无声,故钉死;
 *  2. **防出屏钳制**:锚点优先右下展开、越界翻转、翻转也救不了时安全边兜底。
 *     定位错了的表现是"菜单被屏幕边缘裁掉一半",同样无声,故钉死。
 */
object ContextMenuLogic {

    /**
     * 菜单项排序:非危险项按原序在前,危险项(destructive)按原序置底。
     * 稳定排序:同类内部相对顺序不变,不重排调用方给好的语义序。
     */
    fun orderedContextMenuItems(items: List<ContextMenuItem>): List<ContextMenuItem> {
        val (regular, destructive) = items.partition { !it.destructive }
        return regular + destructive
    }

    /** 一次钳制后的菜单落位:左上角坐标(px)+ 双向翻转标记(入场动画轴用)。 */
    data class MenuPlacement(
        val x: Float,
        val y: Float,
        /** 水平翻到锚点左侧展开(入场 scale 的 pivot 跟随翻到右缘)。 */
        val flipX: Boolean,
        /** 垂直翻到锚点上方展开(pivot 翻到下缘)。 */
        val flipY: Boolean,
    )

    /**
     * 锚点触点 → 菜单左上角落位(全 px):
     *  1. 右下放得下 → 原地展开(贴手指,确认操作对象);
     *  2. 水平/垂直越界 → 各自翻转到左侧/上方;
     *  3. 翻转后仍压线(锚点在角上/菜单比屏宽)→ 钳回 [safePx] 安全边内,
     *     绝不出屏。双侧钳制的上界取 max(safe, screen-safe-size):屏比菜单窄的
     *     极端小窗下上下界收敛为同一个安全边值,不产生负坐标也不崩。
     */
    fun menuPlacement(
        pointXPx: Float,
        pointYPx: Float,
        menuWPx: Float,
        menuHPx: Float,
        screenWPx: Float,
        screenHPx: Float,
        safePx: Float,
    ): MenuPlacement {
        val flipX = pointXPx + menuWPx > screenWPx - safePx
        val flipY = pointYPx + menuHPx > screenHPx - safePx
        val x = (if (flipX) pointXPx - menuWPx else pointXPx)
            .coerceIn(safePx, (screenWPx - safePx - menuWPx).coerceAtLeast(safePx))
        val y = (if (flipY) pointYPx - menuHPx else pointYPx)
            .coerceIn(safePx, (screenHPx - safePx - menuHPx).coerceAtLeast(safePx))
        return MenuPlacement(x, y, flipX, flipY)
    }
}
