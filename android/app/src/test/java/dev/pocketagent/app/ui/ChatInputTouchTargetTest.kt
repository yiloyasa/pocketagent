package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 【PA-REAL-0004 回归】聊天页输入区触控箱最小尺寸。
 *
 * 缺陷(2026-09-19 真机,440dpi):发送 18.2dp / 语音输入 18.2dp / 添加附件 20.0dp /
 * "切换模型与思考强度"整行 37.8dp,均低于 Material 无障碍最小交互尺寸 48dp。
 * 修复:四个控件统一引用 [INPUT_TOUCH_TARGET_DP] 常量做触控箱(sizeIn(min 48dp) /
 * heightIn(min 48dp)),视觉图标保持 18–20dp 不变。
 *
 * 【验证口径】Compose 布局本身 JVM 测不了(布局落在 Instrumentation/真机),本用例
 * 钉的是修饰符实际引用的常量,防止后续被改小而无声回归;真机 bounds 复核走 L2 ——
 * uiautomator dump 中四节点 bounds 两条边均 ≥ 132px(48dp × 2.75 @440dpi)。
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
}
