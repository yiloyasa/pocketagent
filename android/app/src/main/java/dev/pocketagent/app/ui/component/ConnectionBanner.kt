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
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
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
 * 断连全局横幅(审计 P1-2):daemon 断连时三屏(Chat/Home/Threads)共用同一条
 * 状态横幅 —— 「连接已断开，正在重连(第 N 次)… N 条排队中」+ 手动重试。
 * 重连成功(connState 回 ok)自动消失。
 *
 * 【只呈现,不另加重连】自动重连的探活循环是 [AgentConnection.startAutoReconnect]
 * 既有职责,本组件只读 connState 状态流;手动重试也只回调宿主屏既有的刷新入口
 * ([onRetry]),由各屏走自己的 connectWithFallback/列表刷新链,不在横幅里再起一条
 * 重连逻辑(两处重连会互相打断退避节奏)。
 *
 * 【为什么只认 "down" 不认 "checking"】checking 是连接巡检的中间态,不是"已知断网";
 * 按 != "ok" 判会在巡检瞬间闪假横幅 —— 与 ChatScreen 顶部 connState 长注释同一纪律。
 *
 * 【第 N 次怎么数】探活循环每走完一轮回退链仍失败,connState 会再次落回 "down";
 * 每次落回记一次,回 "ok" 清零。这是呈现层计数,不参与也不影响真实重连。
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
    val connState by AgentConnection.connState.collectAsState()
    // 呈现层重连轮数:每次整轮回退链判死(connState 落回 down)记一次,恢复清零
    var reconnectAttempts by remember { mutableIntStateOf(0) }
    LaunchedEffect(connState) {
        when (connState) {
            "down" -> reconnectAttempts += 1
            "ok" -> reconnectAttempts = 0
            // checking 是巡检中间态:不计数、不出横幅
        }
    }
    if (connState != "down") return

    Row(
        modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 2.dp)
            // 语义状态条(不算卡):警示色 8% 透明底 + 16dp 圆角,
            // 与 Home/Threads 既有错误横幅同一套语言(黄=等待,红=失败)
            .pressable { onRetry() }
            .background(PaEditorial.Attention.copy(alpha = 0.08f), RoundedCornerShape(16.dp))
            .padding(horizontal = 14.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(R.drawable.ic_lucide_refresh_cw), null,
            tint = PaEditorial.Attention, modifier = Modifier.size(14.dp),
        )
        Spacer(Modifier.width(9.dp))
        Text(
            connectionBannerCopy(reconnectAttempts, TurnSession.totalQueuedInputs()),
            Modifier.weight(1f),
            fontSize = 12.sp,
            color = PaEditorial.Attention,
            maxLines = 2,
        )
        TextButton(onClick = onRetry) {
            Text("重试", fontSize = 12.sp, color = PaEditorial.Primary)
        }
    }
}
