package dev.pocketagent.app.ui.chat

import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.theme.PaEditorial

/**
 * 方向 C(会话页当播放页做透)的沉浸层小件(2026-10-09):
 *  - [immersiveScrollDecision]:滚动方向 → 两层收放(石墨头部收起 + 输入栏胶囊化)
 *    的判定纯函数,阈值/节流/回顶回底特判全部在此,JVM 单测钉死;
 *  - [StreamingEnergyLine] / [RunningPulseDot]:运行态能量语言(A1)的两个落点,
 *    色只走 PaEditorial.Running(铁律见 PaEditorial 头注「方向 C 铁律」),循环
 *    节拍只走 PaMotion.pulse,减弱动效一律退静态。
 *
 * 零行为、零数据管线:本文件只服务视觉/交互层。
 */

// ---- N1 上滑沉浸:滚动方向 → 两层收放的判定(纯函数,可 JVM 单测) ----

/** 头部收起阈值(px):上滑累计越过即收石墨沉浸头部(输入栏仍在)。 */
internal const val HEADER_COLLAPSE_TRIGGER_PX = 96

/** 输入栏胶囊阈值(px):上滑累计越过即完全沉浸(头部+胶囊,正文获得全高)。 */
internal const val COMPOSER_IMMERSIVE_TRIGGER_PX = 192

/**
 * 累计夹逼上限(px):方向反转时旧累计快速作废 —— 防"上滑半屏后轻轻往回一拉
 * 就触发复原"的迟滞(节流防抖的一半在这里,另一半在调用方的差分只算同 item)。
 */
internal const val IMMERSIVE_ACC_CLAMP_PX = 480

/**
 * 滚动方向 → 沉浸判定(返回 null = 维持现状,调用方据此不落值)。
 *
 * 【进入是渐进的】上滑累计先过 [headerTriggerPx] 只收头部,再过 [composerTriggerPx]
 * 收输入栏为单行胶囊 —— 播放页的两段式入场;【退出是一步的】下滑累计过
 * [headerTriggerPx] 头部与输入栏一起复原,或贴底复原 —— 回程不要同样两段,拖泥带水。
 * 特判:
 *  - 回顶:一切复原(头部是导航入口,顶部必须可见);
 *  - 贴底且已沉浸:复原("下滑回到底部输入栏复原"的硬保证)。贴底但未沉浸时
 *    交给方向规则(距底 100px 跟随阈值 > 头部阈值,若贴底即复原,头部在列表
 *    底部附近将永远收不起来)。
 *
 * @param accumulatedPx 同一首可见 item 内的滚动偏移差分累计(>0 上滑读新内容,
 *   <0 下滑回看);跨 item 基线跳变与程序滚动由调用方排除,此处只认累计值。
 * @return Pair(头部收起?, 输入栏沉浸?);null = 维持现状。
 */
internal fun immersiveScrollDecision(
    headerCollapsed: Boolean,
    composerImmersed: Boolean,
    accumulatedPx: Int,
    atTop: Boolean,
    atBottom: Boolean,
    headerTriggerPx: Int = HEADER_COLLAPSE_TRIGGER_PX,
    composerTriggerPx: Int = COMPOSER_IMMERSIVE_TRIGGER_PX,
): Pair<Boolean, Boolean>? {
    if (atTop) return if (headerCollapsed || composerImmersed) Pair(false, false) else null
    if (atBottom && composerImmersed) return Pair(false, false)
    return when {
        accumulatedPx >= composerTriggerPx ->
            if (headerCollapsed && composerImmersed) null else Pair(true, true)
        accumulatedPx >= headerTriggerPx ->
            if (headerCollapsed) null else Pair(true, composerImmersed)
        accumulatedPx <= -headerTriggerPx ->
            if (!headerCollapsed && !composerImmersed) null else Pair(false, false)
        else -> null
    }
}

// ---- A1 流式能量语言(Running 系专用纪律的界面落点) ----

/**
 * 流式能量线(A1):流式正文块底部的 2dp 细线 —— "正在通电"的生成中视觉。
 * 色只走 [PaEditorial.Running](运行态能量色=Running 系专用纪律,主色不混入);
 * 呼吸节拍走 [PaMotion.pulse] 循环令牌;减弱动效 = 静态半亮(反馈仍在,动效消失)。
 * 只动绘制层 alpha,不触发布局 —— 与流式正文逐帧更新同帧不放大开销。
 */
@Composable
internal fun StreamingEnergyLine(modifier: Modifier = Modifier) {
    if (rememberReduceMotion()) {
        Box(
            modifier
                .fillMaxWidth()
                .height(2.dp)
                .background(
                    PaEditorial.Running.copy(alpha = 0.55f),
                    RoundedCornerShape(PaEditorial.Radius.Terminal),
                ),
        )
        return
    }
    val breath = rememberInfiniteTransition(label = "streamEnergy")
    val alpha by breath.animateFloat(
        initialValue = 0.35f,
        targetValue = 1f,
        animationSpec = PaMotion.pulse(),
        label = "streamEnergyBreath",
    )
    Box(
        modifier
            .fillMaxWidth()
            .height(2.dp)
            .background(
                PaEditorial.Running.copy(alpha = alpha),
                RoundedCornerShape(PaEditorial.Radius.Terminal),
            ),
    )
}

/**
 * 运行态能量点:Running 圆点呼吸 —— "机魂在跑"的迷你提示(沉浸态输入胶囊 /
 * 模型胶囊内的状态点)。同一能量纪律:Running 槽位 + PaMotion.pulse +
 * 减弱动效静态。形状通道走实心点(RUNNING 的 PaStatusGlyph 同形,族语言一致)。
 */
@Composable
internal fun RunningPulseDot(modifier: Modifier = Modifier, dotSize: Dp = 6.dp) {
    if (rememberReduceMotion()) {
        Box(
            modifier
                .size(dotSize)
                .background(PaEditorial.Running.copy(alpha = 0.7f), CircleShape),
        )
        return
    }
    val breath = rememberInfiniteTransition(label = "runningDot")
    val alpha by breath.animateFloat(
        initialValue = 0.35f,
        targetValue = 1f,
        animationSpec = PaMotion.pulse(),
        label = "runningDotBreath",
    )
    Box(
        modifier
            .size(dotSize)
            .background(PaEditorial.Running.copy(alpha = alpha), CircleShape),
    )
}
