package dev.pocketagent.app.ui.anim

import androidx.compose.animation.core.AnimationSpec
import androidx.compose.animation.core.FiniteAnimationSpec
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.SpringSpec
import androidx.compose.animation.core.snap
import androidx.compose.animation.core.spring

/**
 * 全局动效令牌（UI 改版 ·「丝滑跟手」的地基）。
 *
 * 【为什么收敛到一处】此前按压缩放一处 spring(520)、抽屉一处阈值、转场一处
 * tween(220)——各处手感全凭当时手感拍脑袋，改一处别处不知道。令牌化之后：
 * 手感调优只动这个文件，全 app 一起变。
 *
 * 【参数取自 Apple《Designing Fluid Interfaces》的公开建议】
 *  - **默认临界阻尼（damping=1.0）**：优雅、无过冲、不抢戏。绝大多数 UI 状态
 *    切换用它——没有动量来源的东西突然弹一下，读作"廉价"而不是"高级"；
 *  - **动量驱动才给回弹（damping≈0.8-0.9）**：只有手势本身带着速度（甩、掷、
 *    拖拽松手）时，过冲才是物理正确的；
 *  - spring 没有"时长"，settle 时间从参数里涌现（stiffness=ω²，ζ=1 时
 *    settle≈4/ω）。420 ≈ 0.3-0.4s 的"跟手但不僵硬"甜点，与既有 press 一致。
 *
 * 【可中断性是铁律】所有令牌都是 spring——spring 从**当前值**继续动画，任何
 * 时刻被新手势打断都不会跳帧（tween 从起点重播，打断即"砖墙"）。手势驱动的
 * 地方禁止换 tween。
 *
 * 【减弱动效】rememberReduceMotion() 为 true 时改用 [instant]（无位移的
 * 状态直达），反馈仍在、位移消失——"减少动效"≠"没有反馈"。
 */
object PaMotion {

    /** 标准状态切换：临界阻尼、~0.3s。颜色、透明度、小位移的默认选择。 */
    fun <T> standard(): SpringSpec<T> = spring(
        dampingRatio = 1f,
        stiffness = 420f,
    )

    /** 强调出场：轻微过冲（0.9）的缩放/揭示——仍接近临界，过冲幅度 <5%。 */
    fun <T> emphasized(): SpringSpec<T> = spring(
        dampingRatio = 0.9f,
        stiffness = 380f,
    )

    /** 动量驱动：拖拽/轻扫松手后的落位，允许明显过冲（0.8）。只准跟在手势后面用。 */
    fun <T> momentum(): SpringSpec<T> = spring(
        dampingRatio = 0.8f,
        stiffness = 380f,
    )

    /** 抽屉/Sheet：更慢更柔（0.3s 感 + 少量过冲），大面积位移的从容感。 */
    fun <T> sheet(): SpringSpec<T> = spring(
        dampingRatio = 0.85f,
        stiffness = 300f,
    )

    /** 按压反馈（沿用既有 pressable 手感）：按下即响应，不容拖延。 */
    fun <T> press(): SpringSpec<T> = spring(
        dampingRatio = Spring.DampingRatioNoBouncy,
        stiffness = 520f,
    )

    /** 减弱动效/瞬时到位：状态直达，仅保留"变了"这个事实。 */
    fun <T> instant(): AnimationSpec<T> = snap<T>()

    /**
     * NavHost 转场用（要求 FiniteAnimationSpec）：spring 的 settle 由参数涌现，
     * 也是有限时长语义；打断时从当前值续走，比 tween 的 220ms 硬切高级。
     * 退场传更大的 stiffness（更快让位），保留"进场从容、退场干脆"的节奏。
     */
    fun <T> nav(stiffness: Float = 380f): FiniteAnimationSpec<T> = spring(
        dampingRatio = 1f,
        stiffness = stiffness,
    )
}
