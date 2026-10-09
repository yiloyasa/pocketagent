package dev.pocketagent.app.ui.component

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.snap
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.pressableLong
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.paGlass

// ============================================================================
// 运行中会话 mini 条(UI 方向 C · N1 双层世界的招牌件)
//
// 【设计依据】docs/design/ui-language-research-20261009.md §2.1 N1:网易云的
// "列表页 ↔ 播放页"双层世界 —— 运行中的会话以底部 mini 条常驻,点按直达会话页
// (会话页即"播放页",沉浸态由会话页自己承担,mini 条只服务列表世界)。
// 【放置契约】组件只负责"长得像播放条";挂载点在 HomeScreen(HOME 世界)底部、
// 系统导航栏上方。Chat 页内不挂载(会话页自带沉浸态,不叠第二层 chrome)。
// 【数据契约】本组件零业务依赖:只消费 [PaRunningSessionUi] 纯展示模型,
// daemon/TurnSession → 展示模型的映射在调用方(HomeScreen)完成,本文件不 import
// 任何会话/连接类型 —— 判定口径全是下面的纯函数,JVM 单测钉死。
// 【令牌纪律】零新增令牌档位:圆角 Radius.Card(对齐输入栏浮起语言)/ Radius.Pill
// (能量细线胶囊)、弹簧 PaMotion.sheet(出现/消失)、循环 PaMotion.pulse(呼吸,
// 减弱动效静态);暗面按 C2 铁律只用 Graphite 槽位(paGlass 的 tint 覆写)。
// ============================================================================

/**
 * mini 条一拍的展示模型(纯数据):一个运行中会话的"名片"。
 *
 * @param threadId      会话 id(点按跳转/停止动作的唯一定位键)。
 * @param title         会话名(调用方已兜底,不空白)。
 * @param agentLabel    归属 agent 展示名(「Codex」等)。
 * @param statusWhisper 状态细语:耗时/最近活动(如「运行 5 分钟」);空白时组件
 *                      回退「正在执行…」(见 [paSessionBarWhisper],状态词不缺席)。
 * @param iconRes       agent 图标资源(调用方按 agent 归属给出)。
 * @param startedSortKey 启动排序键(turn_started 优先,缺则 updatedAt);「最近
 *                      启动的一个」由此择出,纯函数可测。
 * @param stoppable     本机有没有真实停止通道(手机端发起、TurnSession 仍在跑的
 *                      会话才为真;电脑端起的会话没有停止通道,不摆假停止位)。
 */
data class PaRunningSessionUi(
    val threadId: String,
    val title: String,
    val agentLabel: String,
    val statusWhisper: String,
    val iconRes: Int,
    val startedSortKey: Long,
    val stoppable: Boolean = false,
)

/** mini 条可见性判定(纯函数):≥1 个运行中会话才常驻,0 个整条不渲染。 */
fun paSessionBarVisible(sessions: List<PaRunningSessionUi>): Boolean = sessions.isNotEmpty()

/**
 * 主会话选择(纯函数):多会话并行时,mini 条只露「最近启动」的一个。
 * [startedSortKey] 最大者胜;并列取列表先出现的(调用方按新→旧排序传入时,
 * 并列即"列表序更靠前",与首页正在进行区的排序口径一致)。
 */
fun paSessionBarPrimary(sessions: List<PaRunningSessionUi>): PaRunningSessionUi? =
    sessions.maxByOrNull { it.startedSortKey }

/**
 * 右缘「N」角标文案(纯函数):只有 1 个会话不摆角标(条本身就是那一个);
 * 2..99 报数;>99 封顶 99+(与抽屉角标同一口径,数字永不把条撑破)。
 */
fun paSessionBarBadgeText(runningCount: Int): String? = when {
    runningCount <= 1 -> null
    runningCount <= 99 -> runningCount.toString()
    else -> "99+"
}

/** 状态细语兜底(纯函数):时间源缺席时给「正在执行…」,细语行永不空白。 */
fun paSessionBarWhisper(statusMeta: String): String = statusMeta.ifBlank { "正在执行…" }

/**
 * 运行态能量细线(N3:运行态能量色=品牌记忆点,只出现在呼吸点/进度/转场):
 * Running 色的小胶囊,以 PaMotion.pulse(900ms 往复)呼吸;系统减弱动效开启时
 * 静态(循环动效的退场纪律)。供 mini 条与 Home 运行中大卡共用(同一母题)。
 */
@Composable
internal fun PaRunningEnergyLine(
    modifier: Modifier = Modifier,
    height: androidx.compose.ui.unit.Dp = 30.dp,
) {
    val reduceMotion = rememberReduceMotion()
    val alpha: Float = if (reduceMotion) {
        1f
    } else {
        rememberInfiniteTransition(label = "paEnergyLine")
            .animateFloat(
                initialValue = 1f,
                targetValue = 0.35f,
                animationSpec = PaMotion.pulse(),
                label = "paEnergyAlpha",
            )
            .value
    }
    Box(
        modifier
            .width(4.dp)
            .height(height)
            .graphicsLayer { this.alpha = alpha }
            .background(PaEditorial.Running, RoundedCornerShape(PaEditorial.Radius.Pill)),
    )
}

/**
 * 运行中会话 mini 条:底部常驻悬浮条(网易云播放条语言)。
 *
 * 【交互】
 *  - 点按条身 = 跳转主会话(最近启动的一个);
 *  - 右缘「N」角标(≥2 个会话时出现)= 点按弹运行中列表小面板(PaSheet 底部形态,
 *    Sheet 不是路由,不新增页面);
 *  - [onStopSession] 给出**且**会话 [PaRunningSessionUi.stoppable] 为真时右端有
 *    停止位(现有停止语义的接线在调用方);否则只做跳转(尾部落一个 chevron
 *    指示可点)—— 电脑端发起的会话手机端没有停止通道,不摆点了没反应的假停止。
 *
 * 【动效】出现/消失走 PaMotion.sheet 弹簧(可打断);减弱动效退化为瞬时淡入淡出。
 * 【材质】玻璃高光边 + Graphite 暗面(C2 铁律:暗面只许 Graphite 槽位,三盘同构
 * 自动换肤);浅盘再叠 Shadow.Alert 海拔(对齐输入栏浮起语言;深色盘按 M3 暗色
 * 规范走色调面,不加班驳阴影 —— 与 PaSheet 同一门控)。
 */
@Composable
fun PaSessionBar(
    sessions: List<PaRunningSessionUi>,
    onOpenSession: (String) -> Unit,
    modifier: Modifier = Modifier,
    /** 停止动作(现有停止语义);null = 不提供停止位,条只做跳转。 */
    onStopSession: ((String) -> Unit)? = null,
) {
    val primary = paSessionBarPrimary(sessions)
    val reduceMotion = rememberReduceMotion()
    // 多会话列表面板的开关;会话全部结束(条退场)时同步收起,不留空壳
    var showList by remember { mutableStateOf(false) }
    LaunchedEffect(sessions.size) {
        if (sessions.isEmpty()) showList = false
    }

    AnimatedVisibility(
        visible = primary != null,
        enter = if (reduceMotion) {
            fadeIn(snap())
        } else {
            fadeIn(PaMotion.sheet()) + slideInVertically(PaMotion.sheet()) { it / 2 }
        },
        exit = if (reduceMotion) {
            fadeOut(snap())
        } else {
            fadeOut(PaMotion.sheet()) + slideOutVertically(PaMotion.sheet()) { it / 2 }
        },
        modifier = modifier,
    ) {
        val current = primary
        if (current != null) {
            val shape = RoundedCornerShape(PaEditorial.Radius.Card)
            val barDesc = "正在执行:${current.title},点按查看,共 ${sessions.size} 个会话在运行"
            Row(
                Modifier
                    .fillMaxWidth()
                    .semantics { contentDescription = barDesc }
                    // 玻璃高光边 + Graphite 暗面(C2);玻璃底色显式覆写,不吃 PaGlassToken.Tint
                    .paGlass(shape, tint = PaEditorial.Graphite, hairline = true)
                    // 浅盘海拔语言:悬浮条 = Alert 档(与浮层家族同档);深色盘走细线语言
                    .then(
                        if (PaEditorial.IsDark) Modifier
                        else Modifier.paEditorialShadow(PaEditorial.Shadow.Alert, shape),
                    )
                    .pressableLong(
                        onClick = { onOpenSession(current.threadId) },
                        onLongPress = if (onStopSession != null && current.stoppable) {
                            { onStopSession(current.threadId) }
                        } else {
                            { onOpenSession(current.threadId) }
                        },
                    )
                    // 高度约束在 padding 之前:56dp 是整条高度(内含上下 10dp 边距),
                    // 内容区恰好吃满 36dp 图标的高度线
                    .heightIn(min = 56.dp)
                    .padding(horizontal = 14.dp, vertical = 10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                PaRunningEnergyLine()
                Spacer(Modifier.width(12.dp))
                Image(
                    painterResource(current.iconRes),
                    contentDescription = null,
                    modifier = Modifier
                        .size(36.dp)
                        .clip(RoundedCornerShape(PaEditorial.Radius.Chip)),
                    contentScale = ContentScale.Crop,
                )
                Spacer(Modifier.width(10.dp))
                Column(Modifier.weight(1f)) {
                    Text(
                        current.title,
                        fontSize = PaEditorial.Type.BodySmall.fontSize,
                        lineHeight = 19.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = PaEditorial.OnGraphite,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    Spacer(Modifier.height(2.dp))
                    Text(
                        paSessionBarWhisper(current.statusWhisper),
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        lineHeight = 16.sp,
                        color = PaEditorial.OnGraphiteMuted,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                Spacer(Modifier.width(8.dp))
                // 右缘「N」角标:多会话时出现,点按弹运行中列表(总数口径,见纯函数)
                paSessionBarBadgeText(sessions.size)?.let { badgeText ->
                    Box(
                        Modifier
                            .pressable { showList = true }
                            .background(
                                PaEditorial.RunningContainer,
                                RoundedCornerShape(PaEditorial.Radius.Pill),
                            )
                            .semantics { contentDescription = "查看全部 $badgeText 个运行中的会话" }
                            .padding(horizontal = 10.dp, vertical = 4.dp),
                    ) {
                        Text(
                            badgeText,
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            fontWeight = FontWeight.SemiBold,
                            color = PaEditorial.RunningText,
                        )
                    }
                    Spacer(Modifier.width(6.dp))
                }
                if (onStopSession != null && current.stoppable) {
                    IconActionButton(
                        R.drawable.ic_lucide_square,
                        "停止「${current.title}」",
                        { onStopSession(current.threadId) },
                        bg = Color.Transparent,
                        size = 38.dp,
                        iconSize = 16.dp,
                        tint = PaEditorial.OnGraphiteMuted,
                    )
                } else {
                    Icon(
                        painterResource(R.drawable.ic_lucide_chevron_right),
                        contentDescription = null,
                        tint = PaEditorial.OnGraphiteSubtle,
                        modifier = Modifier.size(18.dp),
                    )
                }
            }
        }
    }

    // 运行中列表小面板(PaSheet 底部形态):点击角标弹出,选会话直达,行尾可停
    if (showList) {
        PaSessionListSheet(
            sessions = sessions,
            onDismiss = { showList = false },
            onOpen = { id ->
                showList = false
                onOpenSession(id)
            },
            onStop = onStopSession,
        )
    }
}

/** 运行中列表小面板:条目 = 图标 + 会话名 + 状态细语,行尾停止位(可选)。 */
@Composable
private fun PaSessionListSheet(
    sessions: List<PaRunningSessionUi>,
    onDismiss: () -> Unit,
    onOpen: (String) -> Unit,
    onStop: ((String) -> Unit)?,
) {
    PaSheet(onDismissRequest = onDismiss) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp)) {
            Text(
                "运行中的会话",
                fontSize = PaEditorial.Type.Heading.fontSize,
                fontWeight = FontWeight.Bold,
                color = PaEditorial.Ink,
            )
            Text(
                "${sessions.size} 个正在执行,点按直达",
                Modifier.padding(top = 3.dp),
                fontSize = PaEditorial.Type.Caption.fontSize,
                color = PaEditorial.Muted,
            )
            Spacer(Modifier.height(10.dp))
            sessions.forEachIndexed { index, s ->
                Row(
                    Modifier
                        .fillMaxWidth()
                        .heightIn(min = 56.dp)
                        .pressable(haptic = true) { onOpen(s.threadId) },
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Image(
                        painterResource(s.iconRes),
                        contentDescription = null,
                        modifier = Modifier
                            .size(32.dp)
                            .clip(RoundedCornerShape(PaEditorial.Radius.Chip)),
                        contentScale = ContentScale.Crop,
                    )
                    Spacer(Modifier.width(12.dp))
                    Column(Modifier.weight(1f)) {
                        Text(
                            s.title,
                            fontSize = PaEditorial.Type.BodySmall.fontSize,
                            fontWeight = FontWeight.SemiBold,
                            color = PaEditorial.Ink,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Text(
                            "${s.agentLabel} · ${paSessionBarWhisper(s.statusWhisper)}",
                            Modifier.padding(top = 2.dp),
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            color = PaEditorial.Muted,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    if (onStop != null && s.stoppable) {
                        IconActionButton(
                            R.drawable.ic_lucide_square,
                            "停止「${s.title}」",
                            { onStop(s.threadId) },
                            bg = Color.Transparent,
                            size = 38.dp,
                            iconSize = 15.dp,
                            tint = PaEditorial.Subtle,
                        )
                    } else {
                        Icon(
                            painterResource(R.drawable.ic_lucide_chevron_right),
                            contentDescription = null,
                            tint = PaEditorial.Subtle,
                            modifier = Modifier.size(16.dp),
                        )
                    }
                }
                if (index < sessions.lastIndex) {
                    Box(
                        Modifier
                            .fillMaxWidth()
                            .height(1.dp)
                            .background(PaEditorial.Hairline),
                    )
                }
            }
            Spacer(Modifier.height(12.dp))
        }
    }
}
