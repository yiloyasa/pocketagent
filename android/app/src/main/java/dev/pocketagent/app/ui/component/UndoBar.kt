package dev.pocketagent.app.ui.component

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * 提交/撤销用的 app 级作用域。
 *
 * 【为什么不能用 UI 作用域】软删除要求在"用户离开页面"时立即提交真删,
 * 而那一刻组合已经销毁、`rememberCoroutineScope` 随之取消 —— 提交会丢。
 * 所以提交与撤销必须跑在 app 级作用域上,才能跨页面存活。
 */
private val UndoScope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

internal data class UndoItem(
    val id: Long,
    val message: String,
    val durationMs: Long,
    val onUndo: suspend () -> Unit,
    val onCommit: (suspend () -> Unit)?,
)

/**
 * 可撤销提示条的状态(设计规范 §2.4)。
 *
 * 【软删除语义】破坏性操作先在本地生效,5 秒后才真正提交远端。
 * 因此撤销是纯本地操作,**必然成功**,不受断网影响。
 *
 * 用法:
 * ```
 * val undo = rememberUndoState()
 * // 删除时:先本地移除,再登记撤销窗口
 * list.removeAt(i)
 * undo.show("已删除「$title」", onUndo = { list.add(i, row) }, onCommit = { api.delete(id) })
 * // 页面顶层渲染
 * UndoBarHost(undo, Modifier.align(Alignment.TopCenter))
 * ```
 */
@Stable
class UndoState {
    internal var current by mutableStateOf<UndoItem?>(null)
        private set

    private var seq = 0L

    val visible: Boolean get() = current != null

    /**
     * 登记一个可撤销操作。
     * 若已有条目存在,**旧条目立即提交**(符合"连续处理多条时只保留最后一次撤销机会"的设计)。
     */
    fun show(
        message: String,
        onUndo: suspend () -> Unit,
        onCommit: (suspend () -> Unit)? = null,
        durationMs: Long = 5000L,
    ) {
        val previous = current
        current = UndoItem(++seq, message, durationMs, onUndo, onCommit)
        if (previous != null) submit(previous)
    }

    /** 撤销失败后需要保持错误态,由宿主调用以清除。 */
    internal fun clear() {
        current = null
    }

    /** 计时结束,提交真删。 */
    internal fun timeout(item: UndoItem) {
        if (current?.id != item.id) return
        submit(item)
        current = null
    }

    /**
     * 立即提交当前条目(不撤销)。页面离开时调用,避免留下"悬挂的软删除"。
     */
    fun commitNow() {
        current?.let { submit(it) }
        current = null
    }

    private fun submit(item: UndoItem) {
        val job = item.onCommit ?: return
        UndoScope.launch { runCatching { job() } }
    }

    internal fun runUndo(item: UndoItem) {
        UndoScope.launch { runCatching { item.onUndo() } }
    }
}

@Composable
fun rememberUndoState(): UndoState = remember { UndoState() }

/**
 * 可撤销提示条宿主(设计规范 §2.4)。
 *
 * 【位置】页面**顶部**(顶部栏下方 8dp),不从底部弹 —— 避免与输入法、返回手势条抢空间。
 * 同时只存在一条;新操作直接替换旧条,不排队、不叠加。
 *
 * 【无障碍】出现时以 [LiveRegionMode.Polite] 播报一次;「撤销」按钮可被独立聚焦。
 */
@Composable
fun UndoBarHost(
    state: UndoState,
    modifier: Modifier = Modifier,
    topPadding: androidx.compose.ui.unit.Dp = 8.dp,
    /**
     * 是否由组件自己加状态栏内边距。
     *
     * 【为什么必须可关】组件默认要加(见下面 [statusBarsPadding] 处的注释)——
     * 前提是宿主页面的根节点直通屏幕边缘。但文件页(FilesScreen)外面套了 Scaffold,
     * 它的 topBar 已经自己 `statusBarsPadding()` 过,内容区又通过 `padding(pad)`
     * 下移了一整个顶栏高度。此时组件**再**加一次状态栏内边距,提示条就会再往下
     * 掉一个状态栏的高度,正好压在路径标题行和第一条文件上 —— 真机上就是
     * "删除提示条糊在内容上"的那个 bug。
     * 所以:宿主已经消费过顶部 insets 的,传 false。
     */
    applyStatusBarsPadding: Boolean = true,
) {
    val item = state.current
    val density = LocalDensity.current
    val reduceMotion = dev.pocketagent.app.ui.anim.rememberReduceMotion()

    // 撤销结果反馈:null=计时中 / "已恢复" / "撤销失败，请重试"
    var feedback by remember(item?.id) { mutableStateOf<String?>(null) }
    var handled by remember(item?.id) { mutableStateOf(false) }
    val progress = remember { Animatable(1f) }

    LaunchedEffect(item?.id, handled) {
        val it0 = item ?: return@LaunchedEffect
        if (handled) return@LaunchedEffect
        progress.snapTo(1f)
        if (reduceMotion) {
            delay(it0.durationMs)
        } else {
            progress.animateTo(0f, tween(it0.durationMs.toInt(), easing = LinearEasing))
        }
        state.timeout(it0)
    }

    // 页面离开时立即提交,避免悬挂的软删除
    DisposableEffect(Unit) {
        onDispose { state.commitNow() }
    }

    AnimatedVisibility(
        visible = item != null,
        enter = slideInVertically(tween(180)) { -it } + fadeIn(tween(150)),
        exit = slideOutVertically(tween(150)) { -it } + fadeOut(tween(120)),
        modifier = modifier,
    ) {
        val it0 = item ?: return@AnimatedVisibility
        val isError = feedback?.startsWith("撤销失败") == true
        val bg = if (isError) Pa.Red else Pa.CardHigh

        Box(
            Modifier
                // 状态栏内边距:页面根节点直通屏幕边缘时由组件自己保证,
                // 不给就会浮到状态栏里和系统图标重叠(真机实测踩到过)。
                // 宿主已消费过顶部 insets 时由调用方传 false 关掉(见参数注释)。
                .then(if (applyStatusBarsPadding) Modifier.statusBarsPadding() else Modifier)
                .padding(top = topPadding, start = 16.dp, end = 16.dp)
                .fillMaxWidth()
                .widthIn(max = 520.dp)
                .clip(RoundedCornerShape(14.dp))
                .background(bg)
                .border(0.5.dp, Pa.Separator, RoundedCornerShape(14.dp))
                .semantics { liveRegion = LiveRegionMode.Polite },
        ) {
            Row(
                Modifier
                    .fillMaxWidth()
                    .height(44.dp)
                    .padding(start = 14.dp, end = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    feedback ?: it0.message,
                    fontSize = 13.sp,
                    color = if (isError) androidx.compose.ui.graphics.Color.White else Pa.TextPrimary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )

                // 已经撤销成功后,"撤销"按钮与进度条都没有存在的意义(用户看到"已恢复"却还能点撤销会困惑),
                // 所以 feedback != null 时立刻收起,只留结果文案 1.2 秒。
                if (!isError && feedback == null) {
                    Spacer(Modifier.width(8.dp))
                    Box(
                        Modifier
                            .clip(RoundedCornerShape(10.dp))
                            .clickable(enabled = true) {
                                handled = true
                                val scopeMarker = it0
                                UndoScope.launch {
                                    val ok = runCatching { scopeMarker.onUndo() }.isSuccess
                                    feedback = if (ok) "已恢复" else "撤销失败，请重试"
                                    if (ok) {
                                        delay(1200)
                                        state.clear()
                                    }
                                }
                            }
                            .padding(horizontal = 12.dp, vertical = 8.dp),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            "撤销",
                            fontSize = 13.sp,
                            color = Pa.AccentText,
                            fontWeight = FontWeight.Medium,
                        )
                    }
                }
            }

            // 剩余时间进度线(错误态与已撤销后都不显示)
            if (!isError && !reduceMotion && feedback == null) {
                Box(
                    Modifier
                        .align(Alignment.BottomStart)
                        .fillMaxWidth(progress.value.coerceIn(0f, 1f))
                        .height(2.dp)
                        .background(Pa.Accent),
                )
            }
        }
    }
}
