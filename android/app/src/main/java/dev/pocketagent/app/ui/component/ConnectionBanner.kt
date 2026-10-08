package dev.pocketagent.app.ui.component

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.TurnSession
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial

/**
 * 断连全局横幅文案(纯函数,单测钉死 ConnectionBannerCopyTest)。
 *
 * 【口径】断连必须同时回答两个问题:「你在不在修」与「我的消息丢了没有」——
 * 前者用重连次数(第 N 次,数字来自横幅对 connState 判死轮数的呈现层计数),
 * 后者用排队条数(有才报,没有就不摆"0 条排队中"的空壳)。
 * 文案禁「未知」「错误」(AUDIT §3.3);负数入参按零收敛,绝不出现负号。
 */
internal fun connectionBannerCopy(reconnectAttempt: Int, queuedInputs: Int): String {
    val attempt = reconnectAttempt.coerceAtLeast(0)
    val base = if (attempt > 0) "连接已断开，正在重连（第 $attempt 次）…" else "连接已断开，正在重连…"
    val queued = queuedInputs.coerceAtLeast(0)
    return if (queued > 0) "$base $queued 条排队中" else base
}

/**
 * 断连横幅状态(纯函数推进,单测钉死 ConnectionBannerStateTest)。
 *
 * @param visible  横幅当前是否可见
 * @param attempts 重连轮数(每次 connState **从非 down 落回 down** 记一轮,回 ok 清零)
 */
internal data class ConnectionBannerState(val visible: Boolean, val attempts: Int)

/**
 * 三态状态机(断线 / 重连中 / 已恢复):
 *  - **断线**(connState == down):可见,轮数只在一"落回"时 +1 —— 同为 down 的
 *    连续探活失败不算新一轮(旧实现靠 LaunchedEffect(connState) 恰好也只有变化
 *    才触发,这里把口径写成显式状态机);组件刚挂载时已在 down 不虚计一轮。
 *  - **重连中**(connState == checking):断过(attempts > 0)就**保持可见** ——
 *    探活回退链动辄数秒,checking 期间撤掉横幅再放回来就是用户看见的"横幅闪烁";
 *    没断过(启动时的首轮 checking)不出横幅,不闪假警报。
 *  - **已恢复**(connState == ok):横幅直接退场,**不弹"已恢复"的成功横幅** ——
 *    恢复是常态,页面内容自己就是证明;多摆一条绿色横幅才是打扰。
 */
internal fun nextConnectionBannerState(prev: String?, next: String, attempts: Int): ConnectionBannerState = when {
    next == "ok" -> ConnectionBannerState(visible = false, attempts = 0)
    next == "down" -> ConnectionBannerState(
        visible = true,
        attempts = if (prev == "down") attempts else attempts + 1,
    )
    else -> ConnectionBannerState(visible = attempts > 0, attempts = attempts)
}

/**
 * 断连全局横幅(审计 P1-2):daemon 断连时三屏(Chat/Home/Threads)共用同一条
 * 状态横幅 —— 「连接已断开，正在重连(第 N 次)… N 条排队中」+ 手动重试。
 *
 * 【三态不闪】状态推进全在 [nextConnectionBannerState](纯函数,单测钉死):
 * 断线(down)可见 → 重连中(checking)断过就保持可见 —— 探活回退链动辄数秒,
 * checking 期间撤横幅再放回来就是"横幅闪烁" → 已恢复(ok)直接退场,不弹
 * "已恢复"的成功横幅(恢复是常态,多摆一条绿横幅才是打扰)。
 *
 * 【只呈现,不另加重连】自动重连的探活循环是 [AgentConnection.startAutoReconnect]
 * 既有职责,本组件只读 connState 状态流;手动重试也只回调宿主屏既有的刷新入口
 * ([onRetry]),由各屏走自己的 connectWithFallback/列表刷新链,不在横幅里再起一条
 * 重连逻辑(两处重连会互相打断退避节奏)。
 *
 * 【第 N 次怎么数】每次 connState **从非 down 落回 down** 记一轮(回 ok 清零);
 * 组件挂载时就已在 down 不虚计 —— 这是呈现层计数,不参与也不影响真实重连。
 *
 * 【排队条数】[TurnSession.totalQueuedInputs] 全会话排队输入只读快照 —— 断连时
 * 用户最怕"发出去的消息丢了",排队数必须一直可见(输入永不丢,审计 P1-2 口径)。
 */
@Composable
fun ConnectionBanner(
    /** 手动重试:交由宿主屏既有刷新/重连入口(横幅自己不实现重连)。 */
    onRetry: () -> Unit,
    modifier: Modifier = Modifier,
) {
    // 三态状态机:prev 取挂载时的当前值,首帧已在 down 不虚计一轮
    var banner by remember {
        mutableStateOf(ConnectionBannerState(visible = false, attempts = 0))
    }
    var prev by remember { mutableStateOf(AgentConnection.connState.value) }
    // 【重试中反馈 + 防重入】(R3-C 微调):点重试后按钮换「重试中…」并禁点,
    // 冷却窗口内的第二发一律拒收;connState 一有动静即视为这轮重试已有下文,
    // 再兜 2.5s 超时,避免状态不动时按钮卡在"重试中"。
    var retrying by remember { mutableStateOf(false) }
    val retryGate = remember { RetryGate() }
    fun beginRetry() {
        if (!retryGate.tryBegin()) return
        retrying = true
        onRetry()
    }
    LaunchedEffect(retrying) {
        if (retrying) {
            kotlinx.coroutines.delay(2500)
            retrying = false
        }
    }
    LaunchedEffect(Unit) {
        AgentConnection.connState.collect { s ->
            banner = nextConnectionBannerState(prev, s, banner.attempts)
            prev = s
            retrying = false
        }
    }
    if (!banner.visible) return

    Row(
        modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 2.dp)
            // 语义状态条(不算卡):警示色 8% 透明底 + 16dp 圆角,
            // 与 Home/Threads 既有错误横幅同一套语言(黄=等待,红=失败)
            .pressable { beginRetry() }
            .background(PaEditorial.Attention.copy(alpha = 0.08f), RoundedCornerShape(PaEditorial.Radius.Card))
            .padding(horizontal = 14.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(R.drawable.ic_lucide_refresh_cw), null,
            tint = PaEditorial.Attention, modifier = Modifier.size(14.dp),
        )
        Spacer(Modifier.width(9.dp))
        Text(
            connectionBannerCopy(banner.attempts, TurnSession.totalQueuedInputs()),
            Modifier.weight(1f),
            fontSize = PaEditorial.Type.Caption.fontSize,
            color = PaEditorial.Attention,
            maxLines = 2,
        )
        TextButton(onClick = { beginRetry() }, enabled = !retrying) {
            Text(if (retrying) "重试中…" else "重试", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Primary)
        }
    }
}
