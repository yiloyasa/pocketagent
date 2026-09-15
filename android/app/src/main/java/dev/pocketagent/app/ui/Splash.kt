package dev.pocketagent.app.ui

import android.app.Activity
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
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
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.delay

/**
 * 启动动效：
 * 白底中央逐字出现品牌字标，蓝色光晕在 agent 区域柔和扩散，
 * 完成后直接淡出交给主页，避免旧版“大 Pa 飞入字标”造成跳变。
 */
private val PaFont = FontFamily(Font(R.font.poppins_bold))
private val Ink = Color(0xFF0D0D0F)
private const val FULL = "Pocket agent"

@Composable
fun Splash(onDone: () -> Unit) {
    val context = LocalContext.current
    val view = LocalView.current
    var visible by remember { mutableStateOf(true) }
    var typed by remember { mutableIntStateOf(0) }
    val textAlpha = remember { Animatable(0f) }
    val glow = remember { Animatable(0f) }

    SideEffect {
        val window = (context as? Activity)?.window ?: return@SideEffect
        window.statusBarColor = android.graphics.Color.WHITE
        window.navigationBarColor = android.graphics.Color.WHITE
        WindowCompat.getInsetsController(window, view).apply {
            isAppearanceLightStatusBars = true
            isAppearanceLightNavigationBars = true
        }
    }

    LaunchedEffect(Unit) {
        delay(260)
        textAlpha.animateTo(1f, tween(220, easing = FastOutSlowInEasing))
        for (i in 1..FULL.length) {
            typed = i
            delay(if (FULL[i - 1] == ' ') 150 else 72)
            if (i == 6) {
                glow.animateTo(1f, tween(900, easing = FastOutSlowInEasing))
            }
        }
        delay(520)
        visible = false
        delay(260)
        onDone()
    }

    AnimatedVisibility(visible = visible, exit = fadeOut(tween(260))) {
        Box(
            Modifier
                .fillMaxSize()
                .background(Color.White),
            contentAlignment = Alignment.Center,
        ) {
            Box(
                Modifier
                    .size(220.dp)
                    .alpha(glow.value * 0.82f)
                    .background(
                        Brush.radialGradient(
                            colors = listOf(
                                Pa.Accent.copy(alpha = 0.42f),
                                Pa.Accent.copy(alpha = 0.16f),
                                Color.Transparent,
                            ),
                        ),
                    ),
            )
            Text(
                buildAnnotatedString {
                    FULL.take(typed).forEachIndexed { index, c ->
                        withStyle(
                            SpanStyle(
                                color = if (index < 7) Ink else Color(0xFF4A9DFF),
                            ),
                        ) {
                            append(c.toString())
                        }
                    }
                    if (typed < FULL.length) {
                        withStyle(SpanStyle(color = Color(0xFF8DBEFF))) {
                            append("▍")
                        }
                    }
                },
                fontFamily = PaFont,
                fontWeight = FontWeight.Bold,
                fontSize = 34.sp,
                modifier = Modifier
                    .graphicsLayer {
                        alpha = textAlpha.value
                        val scale = 0.96f + 0.04f * textAlpha.value
                        scaleX = scale
                        scaleY = scale
                    },
            )
        }
    }
}
