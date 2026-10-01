package dev.pocketagent.app.ui.theme

import androidx.compose.ui.graphics.Color
import kotlin.math.pow

/**
 * WCAG 2.x 对比度纯函数(令牌层的配套工具,2026-09-30 对比度终审引入)。
 *
 * 【为什么放 theme/】"某档文字在某盘底上可读"是设计系统的**约束**而非消费方
 * 的责任;把 sRGB 相对亮度/对比率算式收进令牌层,三盘的达标与否就能被 JVM
 * 单测钉死(见 PaPaletteMatrixTest),改任何文字槽位色值都会被立刻验算。
 *
 * 【纯函数纪律】只依赖 androidx.compose.ui.graphics.Color(值类型,JVM 可测),
 * 不触碰 Compose 组合环境/Android 框架类。
 */
object PaContrast {

    /** WCAG AA 正常文本最低对比率(11-14sp 正文/辅助/小字档的门槛)。 */
    const val AA_TEXT = 4.5f

    /** WCAG 图形件/大字号的最低对比率(三级文本档、图标、色块的门槛)。 */
    const val AA_GRAPHIC = 3.0f

    /** sRGB 相对亮度(WCAG 2.x 定义:线性化后按 0.2126/0.7152/0.0722 加权)。 */
    fun relativeLuminance(color: Color): Float {
        fun channel(c: Float): Float =
            if (c <= 0.04045f) c / 12.92f else ((c + 0.055f) / 1.055f).pow(2.4f)
        return 0.2126f * channel(color.red) +
            0.7152f * channel(color.green) +
            0.0722f * channel(color.blue)
    }

    /** 对比率(L1+0.05)/(L2+0.05),亮者为分子;范围 [1, 21]。 */
    fun contrastRatio(fg: Color, bg: Color): Float {
        val lf = relativeLuminance(fg)
        val lb = relativeLuminance(bg)
        val lighter = maxOf(lf, lb)
        val darker = minOf(lf, lb)
        return (lighter + 0.05f) / (darker + 0.05f)
    }

    /** 是否达到 [min] 对比率(默认 WCAG AA 正常文本 4.5:1)。 */
    fun meets(fg: Color, bg: Color, min: Float = AA_TEXT): Boolean =
        contrastRatio(fg, bg) >= min
}
