package dev.pocketagent.app.ui.anim

import androidx.compose.animation.core.AnimationSpec
import androidx.compose.animation.core.FiniteAnimationSpec
import androidx.compose.animation.core.InfiniteRepeatableSpec
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.SpringSpec
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.snap
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween

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
 * 【动效分层（M3 Expressive：同族组件同弹簧参数）】
 *  - [press]      微交互：按压缩放等瞬时反馈，最快、无过冲；
 *  - [standard]   标准：颜色/透明度/小位移的默认选择，临界阻尼；
 *  - [emphasized] 强调：出场/揭示，允许 <5% 过冲立住；
 *  - [momentum]   动量：只准跟在手势（甩/掷/松手）后面；
 *  - [nav]        导航：NavHost 进出场，退场传更大 stiffness；
 *  - [sheet]      弹层：抽屉/Sheet 大面积位移，慢而柔。
 * 同一层族的组件必须共用同一参数；新增交互先选层再写调用——禁止裸写
 * spring/tween 绕过令牌（循环动效除外，见下）。
 *
 * 【循环动效 2026-09-30 增补】骨架 shimmer、呼吸点、不定进度此前各自裸写
 * `infiniteRepeatable(tween(...))` 绕开令牌；现收敛为 [pulse] / [sweep] 两个
 * 循环令牌（时长见 [Loop]）。循环是"恒定节拍器"不是状态过渡，spring 无时长
 * 语义不适循环，故循环令牌是唯一允许的 tween 家族。使用方迁移（本层不改使用方）：
 *  - PaSkeleton / ThreadsScreen 骨架：`infiniteRepeatable(tween(1200, LinearEasing))`
 *    → `PaMotion.sweep()`；
 *  - TaskCenter 不定进度：`tween(1500, LinearEasing)` → `PaMotion.sweep(PaMotion.Loop.PROGRESS_MS)`；
 *  - HomeScreen 呼吸点 / ThreadStatusBadge：`tween(900, LinearEasing, Reverse)`
 *    → `PaMotion.pulse()`。
 * 减弱动效纪律不变：循环动画的使用方必须继续用 rememberReduceMotion() 自行
 * 退化为静态（令牌层不感知组合环境，保持纯函数可测）。
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

    /** 减弱动效/瞬时到位：状态直达，仅保留"变了"这个事实。
     *  返回类型收窄为 FiniteAnimationSpec：snap 本就是有限动画，fadeIn/fadeOut
     *  等转场函数只收 Finite，宽声明会逼调用点写死类型参数。 */
    fun <T> instant(): FiniteAnimationSpec<T> = snap<T>()

    /**
     * NavHost 转场用（要求 FiniteAnimationSpec）：spring 的 settle 由参数涌现，
     * 也是有限时长语义；打断时从当前值续走，比 tween 的 220ms 硬切高级。
     * 退场传更大的 stiffness（更快让位），保留"进场从容、退场干脆"的节奏。
     */
    fun <T> nav(stiffness: Float = 380f): FiniteAnimationSpec<T> = spring(
        dampingRatio = 1f,
        stiffness = stiffness,
    )

    /** 循环动效时长基线（ms），取自全 app 既有循环的实测值——令牌化后只有
     * 这几个数，新增循环先从这里选，确需新节奏先改这里。
     */
    object Loop {
        /** 骨架 shimmer 扫描一周（PaSkeleton / ThreadsScreen 骨架现状 1200ms）。 */
        const val SHIMMER_MS = 1200

        /** 状态点呼吸一个来回（HomeScreen 呼吸点 / ThreadStatusBadge 现状 900ms）。 */
        const val PULSE_MS = 900

        /** 不定进度条扫一周（TaskCenter indeterminate 现状 1500ms）。 */
        const val PROGRESS_MS = 1500
    }

    /**
     * 循环·呼吸：0→1→0 线性往复（Reverse），用于呼吸点/波纹的"存在感"脉动。
     * 循环是恒定节拍，用线性 tween 而非 spring（见类头【循环动效】）。
     */
    fun <T> pulse(durationMs: Int = Loop.PULSE_MS): InfiniteRepeatableSpec<T> =
        infiniteRepeatable(tween(durationMs, easing = LinearEasing), RepeatMode.Reverse)

    /**
     * 循环·扫掠：起点→终点线性单程后重来（Restart），用于骨架 shimmer /
     * 不定进度的"位移扫过"。更慢的进度条传 [Loop.PROGRESS_MS]。
     */
    fun <T> sweep(durationMs: Int = Loop.SHIMMER_MS): InfiniteRepeatableSpec<T> =
        infiniteRepeatable(tween(durationMs, easing = LinearEasing))
}
