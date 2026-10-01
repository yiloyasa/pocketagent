package dev.pocketagent.app.ui

import android.app.Activity
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.view.WindowCompat
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.theme.PaEditorial
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * 启动动效(编辑式版):
 *   1) 背景走当前盘的编辑式画布(PaEditorial.Canvas 随显示模式换肤:浅盘冷白
 *      #F6F7F6 / 黑客·深盘近黑 —— 2026-09-30 起令牌是三盘同槽位,首屏与后续
 *      页面同盘,不再有"固定浅色首屏");
 *   2) 字标先淡入并轻微放大"立住",再逐字打出品牌名,节奏均匀;
 *   3) 到 "agent" 时主色光晕柔和扩散,底部一条进度线同步展开(主色只用于这一处
 *      品牌时刻,不铺满画面);
 *   4) 收尾整屏淡出交接主页,全程无跳变。
 */
private val PaFont = FontFamily(Font(R.font.poppins_bold))
private const val FULL = "Pocket agent"

@Composable
fun Splash(onDone: () -> Unit) {
    val context = LocalContext.current
    val view = LocalView.current
    var visible by remember { mutableStateOf(true) }
    var typed by remember { mutableIntStateOf(0) }

    val markAlpha = remember { Animatable(0f) }
    val markScale = remember { Animatable(0.92f) }
    val glow = remember { Animatable(0f) }
    val sweep = remember { Animatable(0f) }
    // 减弱动效:字标/进度线瞬时到位,整体短驻留后直接交接主页(不播任何位移/缩放)
    val reduceMotion = rememberReduceMotion()

    // 编辑式画布随当前盘(三色盘 2026-09-30 起同槽位换肤:黑客/深盘=近黑,浅盘=冷白):
    // 冷启动首屏与其余页面同盘;系统栏图标按 IsDark,近黑画布上是浅色图标
    // (原固定 isAppearanceLightStatusBars=true 在黑客/深盘 = 深图标坐近黑栏,漂移)。
    val bg = PaEditorial.Canvas
    val ink = PaEditorial.Ink
    val accent = PaEditorial.Primary

    SideEffect {
        val window = (context as? Activity)?.window ?: return@SideEffect
        window.statusBarColor = bg.toArgb()
        window.navigationBarColor = bg.toArgb()
        WindowCompat.getInsetsController(window, view).apply {
            isAppearanceLightStatusBars = !PaEditorial.IsDark
            isAppearanceLightNavigationBars = !PaEditorial.IsDark
        }
    }

    LaunchedEffect(Unit) {
        if (reduceMotion) {
            typed = FULL.length
            markAlpha.snapTo(1f); markScale.snapTo(1f); glow.snapTo(1f); sweep.snapTo(1f)
            delay(420)
            visible = false
            delay(120)
            onDone()
            return@LaunchedEffect
        }
        // 1) 字标先立住:淡入 + 轻微放大并行。
        // 【令牌化】位移/缩放类过渡用 spring(emphasized 的轻微过冲正好是"立住"的物理感),
        // 打字节奏保持 tween(它是逐字内容的节拍器,不是状态过渡,速率要恒定)。
        launch { markAlpha.animateTo(1f, PaMotion.standard()) }
        markScale.animateTo(1f, PaMotion.emphasized())
        delay(140)

        // 2) 逐字打出;到 "agent" 时触发光晕与进度线
        val agentStart = FULL.indexOf("agent") + 1
        val agentEnd = agentStart + "agent".length
        for (i in 1..FULL.length) {
            typed = i
            delay(if (FULL[i - 1] == ' ') 120 else 58)
            if (i == agentStart) {
                launch { glow.animateTo(1f, tween(1100, easing = FastOutSlowInEasing)) }
                launch { sweep.animateTo(1f, tween(FULL.length * 58 + 200, easing = FastOutSlowInEasing)) }
            }
            if (i == agentEnd) {
                // "agent" 打完再补一点光晕,形成收束感
                launch { glow.animateTo(1f, tween(500, easing = FastOutSlowInEasing)) }
            }
        }
        sweep.animateTo(1f, tween(320, easing = FastOutSlowInEasing))
        delay(460)

        // 3) 整屏淡出,交接主页(收尾快而稳,不拖节奏)
        visible = false
        delay(240)
        onDone()
    }

    AnimatedVisibility(visible = visible, exit = fadeOut(PaMotion.standard())) {
        Box(
            Modifier
                .fillMaxSize()
                .background(bg),
            contentAlignment = Alignment.Center,
        ) {
            // 主色光晕:在 "agent" 区域柔和扩散
            Box(
                Modifier
                    .size(240.dp)
                    .alpha(glow.value * 0.85f)
                    .background(
                        Brush.radialGradient(
                            colors = listOf(
                                accent.copy(alpha = 0.45f),
                                accent.copy(alpha = 0.14f),
                                Color.Transparent,
                            ),
                        ),
                    ),
            )

            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                modifier = Modifier.graphicsLayer {
                    alpha = markAlpha.value
                    scaleX = markScale.value
                    scaleY = markScale.value
                },
            ) {
                Text(
                    buildAnnotatedString {
                        FULL.take(typed).forEachIndexed { index, c ->
                            withStyle(
                                SpanStyle(color = if (index < 7) ink else accent),
                            ) {
                                append(c.toString())
                            }
                        }
                        if (typed < FULL.length) {
                            withStyle(SpanStyle(color = accent.copy(alpha = 0.65f))) {
                                append("▍")
                            }
                        }
                    },
                    fontFamily = PaFont,
                    fontWeight = FontWeight.Bold,
                    fontSize = 34.sp,
                )
                Spacer(Modifier.height(16.dp))
                // 进度线:随打字节奏展开,停留时收满(底槽 = 细线令牌)
                Box(
                    Modifier
                        .width(150.dp)
                        .height(3.dp)
                        .background(PaEditorial.Hairline, RoundedCornerShape(50)),
                ) {
                    Box(
                        Modifier
                            .fillMaxWidth(sweep.value.coerceIn(0f, 1f))
                            .fillMaxHeight()
                            .background(accent, RoundedCornerShape(50)),
                    )
                }
            }
        }
    }
}
