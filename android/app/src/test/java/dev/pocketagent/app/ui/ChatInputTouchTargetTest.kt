package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/**
 * 【PA-REAL-0004 回归】聊天页输入区触控箱最小尺寸。
 *
 * 缺陷(2026-09-19 真机,440dpi):发送 18.2dp / 语音输入 18.2dp / 添加附件 20.0dp /
 * "切换模型与思考强度"整行 37.8dp,均低于 Material 无障碍最小交互尺寸 48dp。
 * 修复:四个控件统一引用 [INPUT_TOUCH_TARGET_DP] 常量做触控箱(sizeIn(min 48dp) /
 * heightIn(min 48dp)),视觉图标保持 18–20dp 不变。
 *
 * 【T-D2 收尾扩展(2026-09-20)】用户已批准把输入区剩余同类小触控区统一放大,
 * 逐控件结构钉如下(Git 按钮 32→48 / 优化按钮 32→48 / 上下文环 34→48(展示型,
 * 环视觉 28dp 不变) / 展开编辑 22→48(图标钉回原右上角位) / 审批策略 chip
 * ≈25→48(heightIn 外层) / 附件 chip 移除叉 13→48(独立触控箱,chip 本体点击
 * 是"预览"不能替代移除))。
 *
 * 【验证口径】Compose 布局本身 JVM 测不了(布局落在 Instrumentation/真机),本用例
 * 钉的是修饰符实际引用的常量与控件源码结构,防止后续被改小而无声回归;真机 bounds
 * 复核走 L2 —— uiautomator dump 中节点 bounds 两条边均 ≥ 132px(48dp × 2.75 @440dpi)。
 */
class ChatInputTouchTargetTest {

    /** 触控箱常量不得低于 Material 最小交互尺寸 48dp。 */
    @Test
    fun touchTargetAtLeastMaterialMinimum48dp() {
        assertTrue(
            "输入区触控箱常量 $INPUT_TOUCH_TARGET_DP dp < 48dp,PA-REAL-0004 回归被破坏",
            INPUT_TOUCH_TARGET_DP >= 48,
        )
    }

    /** 440dpi(小米 2211133C 实测,dp=px/2.75)下 48dp 恰为 132px:L2 复核口径换算钉死。 */
    @Test
    fun touchTargetConvertsTo132pxAt440dpi() {
        assertEquals(132f, INPUT_TOUCH_TARGET_DP * 2.75f, 0.01f)
    }

    /** 常量必须为正偶数(成对 padding/居中推理时不出半像素歧义),防手滑改成怪值。 */
    @Test
    fun touchTargetIsPositiveEvenDp() {
        assertTrue(INPUT_TOUCH_TARGET_DP > 0)
        assertEquals(0, INPUT_TOUCH_TARGET_DP % 2)
    }

    // ---- T-D2 收尾:逐控件源码结构钉(字符串锚点定位,行号漂移免疫) ----

    /** 截取 [from] 锚点到其后首个 [to] 锚点之间的源码片段(均须存在且有序)。 */
    private fun fragment(from: String, to: String): String {
        val text = locate("src/main/java/dev/pocketagent/app/ui/chat/ChatComposer.kt").readText()
        val start = text.indexOf(from)
        assertTrue("锚点缺失: $from", start >= 0)
        val end = text.indexOf(to, start + from.length)
        assertTrue("锚点缺失或在 [from] 之前: $to", end > start)
        return text.substring(start, end)
    }

    private val touchBox =
        "sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)"
    private val touchHeight = "heightIn(min = INPUT_TOUCH_TARGET_DP.dp)"

    /** [Git] 按钮触控箱 ≥48dp(原 32dp)。 */
    @Test
    fun gitButtonTouchBoxReferences48dpConstant() {
        val src = fragment("// [Git] 会话项目的版本管理侧栏", "// [优化]")
        assertTrue("Git 按钮缺 $touchBox 触控箱(T-D2 回归)", src.contains(touchBox))
        assertFalse("Git 按钮仍用固定 32dp 盒(T-D2 回归)", src.contains(".size(32.dp)"))
    }

    /** [优化] 按钮触控箱 ≥48dp(原 32dp)。 */
    @Test
    fun optimizeButtonTouchBoxReferences48dpConstant() {
        val src = fragment("// [优化]", "// [审批策略]")
        assertTrue("优化按钮缺 $touchBox 触控箱(T-D2 回归)", src.contains(touchBox))
        assertFalse("优化按钮仍用固定 32dp 盒(T-D2 回归)", src.contains(".size(32.dp)"))
    }

    /** [审批策略] chip 整体命中高度 ≥48dp(原 ≈25dp,heightIn 在 padding 外层)。 */
    @Test
    fun approvalChipHeightReferences48dpConstant() {
        val src = fragment("// [审批策略]", "DropdownMenu(")
        assertTrue("审批策略 chip 缺 $touchHeight(T-D2 回归)", src.contains(touchHeight))
    }

    /** 上下文环(展示型)命中区 ≥48dp(原 34dp);环视觉必须保持 28dp 不变。 */
    @Test
    fun contextRingHitBox48dpWhileRingVisualStays28dp() {
        val hit = fragment("// [上下文窗口]", "ContextRing(")
        assertTrue("上下文环缺 $touchBox 命中区(T-D2 回归)", hit.contains(touchBox))
        assertFalse("上下文环仍用固定 34dp 盒(T-D2 回归)", hit.contains(".size(34.dp)"))
        // 环本体是展示层:视觉尺寸不许跟着命中区一起放大
        val ring = fragment("ContextRing(", "compacted = ctxCompacted,")
        assertTrue("ContextRing 视觉尺寸不再是 28dp —— 展示层被误放大", ring.contains(".size(28.dp)"))
    }

    /** 展开编辑命中区 ≥48dp(原 22dp);视觉图标(15dp)钉回原 22dp 盒中心。 */
    @Test
    fun expandEditHitBox48dpWithIconPinnedToOriginalCorner() {
        // 片段终点用图标的收尾 size(15.dp):覆盖到 Icon 定位 modifier 为止
        val src = fragment(".padding(end = 10.dp, top = 6.dp)", ".size(15.dp)")
        assertTrue("展开编辑命中区未引用 48dp 常量(T-D2 回归)", src.contains(".size(INPUT_TOUCH_TARGET_DP.dp)"))
        // 视觉不漂移的双保险:图标改 TopEnd 对齐 + 3.5dp 内边距(=原 22dp 盒中心 11dp 处)
        assertTrue("展开编辑图标未钉回原右上角位(contentAlignment=TopEnd 丢失)", src.contains("contentAlignment = Alignment.TopEnd"))
        assertTrue("展开编辑图标定位 padding(end=3.5,top=3.5) 丢失,视觉会漂移", src.contains("padding(end = 3.5.dp, top = 3.5.dp)"))
        assertFalse("展开编辑仍用固定 22dp 盒(T-D2 回归)", src.contains(".size(22.dp)"))
    }

    /** 附件 chip 移除叉:独立 48×48 触控箱(原仅 13dp);chip 本体命中高度 ≥48dp。 */
    @Test
    fun attachmentChipAndRemoveXMeet48dp() {
        val src = fragment("attachments.value.forEach { at ->", "attachments.value - at }")
        assertTrue("附件 chip 缺 $touchHeight(T-D2 回归)", src.contains(touchHeight))
        assertTrue("移除叉缺独立 $touchBox 触控箱(T-D2 回归)", src.contains(touchBox))
        // 旧形态钉死:13dp 图标直接挂 clickable 的写法不许回来
        assertFalse(
            "移除叉回到 13dp 图标直连 clickable 的旧形态(T-D2 回归)",
            Regex("""\.size\(13\.dp\)\s*\n\s*\.pressable""").containsMatchIn(src),
        )
    }
}
