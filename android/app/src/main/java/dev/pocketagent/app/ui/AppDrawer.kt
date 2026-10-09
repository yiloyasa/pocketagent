package dev.pocketagent.app.ui

import androidx.activity.compose.BackHandler
import dev.pocketagent.app.BuildConfig
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.Crossfade
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Popup
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.pressableLong
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.component.PaHostLink
import dev.pocketagent.app.ui.component.drawerFollowOffsetPx
import dev.pocketagent.app.ui.component.hostStatusTone
import dev.pocketagent.app.ui.component.hostStatusWord
import dev.pocketagent.app.ui.theme.PaEditorial
import kotlin.math.roundToInt
import kotlinx.coroutines.delay

/**
 * 编辑式侧边栏的石墨层取值(docs/design premium.css「The navigation rail is a
 * deliberately dark layer」块;PaEditorial 收口了底色/细线/文字档,下面几个是
 * 抽屉专属的层次色,不进令牌层 —— 有缺陷报基座,不改令牌文件)。
 */
private object DrawerSkin {
    /** .drawer 右缘细线 #384959。 */
    val Edge = Color(0xFF384959)
    /** .drawer-head .logo 底 #3b5772(品牌方块)。 */
    val Logo = Color(0xFF3B5772)
    /** .drawer-head small 的薄荷绿 #91d8c2(已连接副文)。 */
    val HeadOk = Color(0xFF91D8C2)
    /** .drawer-section 分区标签 #8da0b0。 */
    val Section = Color(0xFF8DA0B0)
    /** .drawer-link 文字 #bac9d7。 */
    val Link = Color(0xFFBAC9D7)
    /** .drawer.collapsed .drawer-link #becbd8(rail 态图标)。 */
    val RailIcon = Color(0xFFBECBD8)
    /** .drawer-link.active 底 #31475e / 字 #f5faff(当前项高亮)。 */
    val ActiveBg = Color(0xFF31475E)
    val ActiveText = Color(0xFFF5FAFF)
    /** .drawer-link .badge 底 #9d6038(琥珀家族,任务角标)。 */
    val Badge = Color(0xFF9D6038)
    /** .drawer-head .icon-button #a9bed2。 */
    val HeadIcon = Color(0xFFA9BED2)
}

/** 面板宽:展开 330dp(面板模式)/ 收起 76dp(rail 模式)。跟手位移与宽度动画共用,两处不会算岔。 */
private val DrawerPanelWidth = 330.dp
private val DrawerRailWidth = 76.dp

// ---------------------------------------------------------------- 导航条目(恒定层)

/** 置灰原因词(状态词落字,不单靠灰度):消息中心在未接线的页面。 */
internal const val DRAWER_HINT_ATTENTION_OFF = "在主页查看"

/** 置灰原因词:协同能力未接入(daemon 无 collab:<id> 接线事实)。 */
internal const val DRAWER_HINT_COLLAB_OFF = "未接入"

/**
 * 抽屉导航项的稳定规格(纯数据,id/图标/标题/归属 tab 全部静态)。
 * [tab] ≥0 = 主导航 tab 项;-1 = 动作项(无选中态,如消息中心/协同)。
 */
internal data class DrawerNavEntrySpec(
    val id: String,
    val icon: Int,
    val title: String,
    val tab: Int,
)

/** 规格接线后的一行:同一规格只有 [enabled]/[disabledHint] 随接线状态变化。 */
internal data class DrawerNavEntry(
    val spec: DrawerNavEntrySpec,
    val enabled: Boolean,
    /** 置灰时的一句真实原因;enabled=true 恒为 null。 */
    val disabledHint: String?,
)

/** 稳定规格清单(纯函数,单测钉死):顺序即渲染顺序,任何接线组合下不变。 */
internal fun drawerNavEntrySpecs(): List<DrawerNavEntrySpec> = listOf(
    DrawerNavEntrySpec("home", R.drawable.ic_lucide_monitor, "主页", 0),
    DrawerNavEntrySpec("threads", R.drawable.ic_lucide_message_square, "会话", 1),
    DrawerNavEntrySpec("taskCenter", R.drawable.ic_lucide_list_checks, "任务中心", 4),
    DrawerNavEntrySpec("files", R.drawable.ic_lucide_folder, "文件", 2),
    DrawerNavEntrySpec("git", R.drawable.ic_lucide_git_branch, "Git", 3),
    DrawerNavEntrySpec("attention", R.drawable.ic_lucide_info, "消息中心", -1),
    DrawerNavEntrySpec("collab", R.drawable.ic_lucide_share_2, "多 Agent 协同", -1),
)

/**
 * 导航条目集合恒定(纯函数,单测覆盖 DrawerConstancyTest):
 *
 * 【修的是什么】上轮盘点实锤「同一分钟内多 Agent 协同条目时有时无」—— 旧实现把
 * 消息中心/协同两个条目绑在调用方的瞬态状态上(协同绑 sharedCaps 快照、消息中心
 * 绑注意项有无),状态一变条目就凭空增减,导航位置跳变。
 * 【怎么修】条目**恒渲染**,只按接线状态切可用性:未接线 = 置灰(石墨面的 Muted
 * 槽 OnGraphiteSubtle)+ 原因词落字 + 点按无动作,**不再消失**。
 * 【置灰原因词是真话】协同未接线 =「未接入」(能力事实);消息中心未接线 =
 * 「在主页查看」(中心是主页的浮层,其余页面不接线 —— 不是功能不存在)。
 *
 * @param attentionWired 调用方是否接了消息中心回调
 * @param collabWired    调用方是否接了协同回调(首页在 daemon 报 collab:<id> 时才接)
 */
internal fun buildDrawerNavEntries(
    attentionWired: Boolean,
    collabWired: Boolean,
): List<DrawerNavEntry> = drawerNavEntrySpecs().map { spec ->
    when (spec.id) {
        "attention" -> DrawerNavEntry(spec, attentionWired, DRAWER_HINT_ATTENTION_OFF.takeIf { !attentionWired })
        "collab" -> DrawerNavEntry(spec, collabWired, DRAWER_HINT_COLLAB_OFF.takeIf { !collabWired })
        else -> DrawerNavEntry(spec, enabled = true, disabledHint = null)
    }
}

/**
 * 全局侧边栏 —— 底部导航栏移除后的唯一主导航(编辑式石墨层)。
 *
 * 【视觉】premium.css .drawer:实心石墨 #1B2634(PaEditorial.GraphiteDrawer)+
 * 右缘 1dp 细线,不再是玻璃半透明 —— 侧边栏是"刻意与画布区分的深色层",
 * 层次而非满屏深色(README §视觉系统)。遮罩走 PaEditorial.Scrim。
 *
 * 【滑出方向】面板贴左边缘,必须从**左侧**滑入(初始偏移 `-it`,整块先移到屏幕外),
 * 之前写成 `it / 3`(正值)会让面板从右侧"倒着"挤进来 —— 这就是"从相反方向弹出"的根因。
 *
 * 【退场】调用方仍是 `if (showDrawer) AppDrawer(...)`,组件一旦被摘掉就来不及播退场动画。
 * 所以这里自己管一个 `closing` 标志:关闭请求先把 targetState 置 false 播完退场,
 * 等 currentState 也归位后才回调 `onClose()` 让调用方卸载 —— 无需改任何调用点。
 * 顺带把"点了导航项要跳的页"挂到退场结束之后再执行,视觉上就是"先收抽屉、再换页"
 * (README:选择页面先收抽屉再导航)。
 */
@Composable
fun AppDrawer(
    currentTab: Int,
    hostTitle: String,
    onClose: () -> Unit,
    onGoTab: (Int) -> Unit,
    onOpenTerminal: () -> Unit,
    onOpenSettings: () -> Unit,
    onOpenTaskCenter: () -> Unit = {},
    /**
     * 重看新手教程(抽屉「更多」分组 + 离线态"看怎么连"的落点)。
     * 可空:没传的页面**不渲染**教程入口 —— 与 onOpenCollab 同一纪律,不摆点了没反应的假入口。
     */
    onOpenOnboarding: (() -> Unit)? = null,
    /** 传了就显示"新建对话"主操作(聊天页用;其余页面各有自己的新建入口)。 */
    onNewThread: (() -> Unit)? = null,
    /** 传了就显示"网页"快捷入口(内置浏览器)。 */
    onOpenBrowser: (() -> Unit)? = null,
    /** 传了就可用"多 Agent 协同"导航项(首页在 daemon 报 collab:<id> 时才传)。未传 = 条目置灰「未接入」,不再消失(抽屉恒定修复)。 */
    onOpenCollab: (() -> Unit)? = null,
    /**
     * 传了就可用"消息中心"入口(2026-10-04 用户处理中心计划 Task 5)。
     * 未传 = 条目置灰「在主页查看」,不再消失(抽屉恒定修复:条目集合恒定,
     * 不随调用方瞬态状态增减 —— 修"同一分钟内条目时有时无"的实锤)。
     */
    onOpenAttention: (() -> Unit)? = null,
    /** 消息中心角标 = 「需要我处理」条数(与首页摘要同源;0 = 不显示角标)。 */
    attentionBadge: Int = 0,
    /**
     * 跟手进度(0..1)。传了就用它驱动遮罩与面板位移,不再播入场动画 ——
     * 手指在哪面板就在哪,松手由调用方吸附。
     */
    progress: Float? = null,
) {
    val reduceMotion = rememberReduceMotion()

    val visibleState = remember { MutableTransitionState(false) }
    var closing by remember { mutableStateOf(false) }
    var pendingAction by remember { mutableStateOf<(() -> Unit)?>(null) }

    /** 请求关闭;[then] 会在抽屉完全收起后执行(用它承载"跳转到某个页面")。 */
    fun requestClose(then: (() -> Unit)? = null) {
        if (closing) return
        pendingAction = then
        closing = true
        visibleState.targetState = false
    }

    LaunchedEffect(Unit) { visibleState.targetState = true }
    // 退场跑完(currentState 也变 false)才真正通知调用方卸载,并执行挂起的跳转
    LaunchedEffect(visibleState.currentState, visibleState.targetState) {
        if (closing && !visibleState.targetState && !visibleState.currentState) {
            onClose()
            pendingAction?.invoke()
            pendingAction = null
        }
    }

    // 返回键优先关侧边栏(README 回退层级:抽屉 → 关闭抽屉;修复"按返回直接退回桌面")
    BackHandler(enabled = true) { requestClose() }

    val shown = visibleState.targetState || visibleState.currentState
    // 【令牌化】遮罩浓淡走 PaMotion.standard(spring 可打断);减弱动效时 snap 瞬时到位
    val animatedScrim by animateFloatAsState(
        targetValue = if (shown) 1f else 0f,
        animationSpec = if (reduceMotion) PaMotion.instant() else PaMotion.standard(),
        label = "drawerScrim",
    )
    // 跟手时进度由外部给,否则走内部动画
    val scrimAlpha = progress ?: animatedScrim

    val unread by TaskCenter.unreadCount.collectAsState()
    // 折叠偏好:展开(图标+文字) / 收起(窄 rail 仅图标)。AgentConnection 内已持久化。
    val collapsed = AgentConnection.drawerCollapsed

    // 遮罩:铺满整个屏幕(全屏点击区),淡入淡出。
    // 【遮罩取编辑式值】PaEditorial.Scrim(#101722A9)= premium.css .modal-scrim,
    // 与石墨抽屉同一套冷色层次;不再走玻璃令牌的黑色遮罩。
    Box(
        Modifier
            .fillMaxSize()
            .background(PaEditorial.Scrim.copy(alpha = PaEditorial.Scrim.alpha * scrimAlpha))
            // scaleDown=1f:遮罩本身不做缩放,只当点击区
            .pressable(scaleDown = 1f) { requestClose() },
    ) {
        // 动效:全部走 spring 令牌(可打断=丝滑根基)。
        // 跟手时无内部动画(位移由手势驱动);减弱动效时按设计规范 §1.5 退化为
        // "仅淡入/淡出",且用 snap(零时长)——spring 不受系统动画缩放影响,只有
        // snap 能真正做到"瞬时到位"。fadeIn 要求 FiniteAnimationSpec,而
        // PaMotion.instant() 的静态类型是 AnimationSpec,故此处直接用同义的 snap()。
        val enter = when {
            progress != null -> androidx.compose.animation.EnterTransition.None
            reduceMotion -> fadeIn(androidx.compose.animation.core.snap())
            else -> fadeIn(PaMotion.sheet()) + slideInHorizontally(PaMotion.sheet()) { -it }
        }
        val exit = when {
            progress != null -> androidx.compose.animation.ExitTransition.None
            reduceMotion -> fadeOut(androidx.compose.animation.core.snap())
            else -> fadeOut(PaMotion.sheet()) + slideOutHorizontally(PaMotion.sheet()) { -it }
        }

        // 跟手时:整体位移 = 面板宽 × (1 - 进度)。面板宽用真实 dp 换算像素
        // (drawerFollowOffsetPx 的口径),不再用"屏宽 × 比例"估 —— 估出来的数
        // 和面板实际宽度差几十 px,收起态会在屏幕左缘留下一条合不拢的缝。
        val panelPx = with(LocalDensity.current) {
            (if (AgentConnection.drawerCollapsed) DrawerRailWidth else DrawerPanelWidth).toPx()
        }
        // 【placement 层位移】进度经 rememberUpdatedState 进 offset{} 的块:块在
        // **placement 阶段**执行,每帧直接拿到最新进度。不把 graphicsLayer 挂在
        // AnimatedVisibility 上 —— 那条路里动画层可能吞掉纯取值链的状态失效,
        // 表现为拖动时面板偶发不跟帧;跟手必须落在 placement 层。
        val progressState = rememberUpdatedState(progress)

        AnimatedVisibility(
            visibleState = visibleState,
            enter = enter,
            exit = exit,
            modifier = Modifier
                .align(Alignment.CenterStart)
                .fillMaxHeight()
                .offset {
                    val p = progressState.value
                    if (p != null) IntOffset(drawerFollowOffsetPx(p, panelPx).roundToInt(), 0) else IntOffset.Zero
                },
        ) {
            // 折叠/展开宽度平滑过渡(rail ↔ 面板);spring 令牌,可打断
            val panelWidth by animateDpAsState(
                targetValue = if (collapsed) DrawerRailWidth else DrawerPanelWidth,
                animationSpec = PaMotion.standard(),
                label = "panelWidth",
            )
            DrawerPanel(
                modifier = Modifier.width(panelWidth),
                currentTab = currentTab,
                hostTitle = hostTitle,
                unread = unread,
                collapsed = collapsed,
                onGoTab = { requestClose { onGoTab(it) } },
                onOpenTerminal = { requestClose { onOpenTerminal() } },
                onOpenSettings = { requestClose { onOpenSettings() } },
                onOpenTaskCenter = { requestClose { onOpenTaskCenter() } },
                onOpenOnboarding = onOpenOnboarding?.let { cb -> { requestClose { cb() } } },
                onOpenCollab = onOpenCollab?.let { cb -> { requestClose { cb() } } },
                onOpenAttention = onOpenAttention?.let { cb -> { requestClose { cb() } } },
                attentionBadge = attentionBadge,
                onToggleCollapsed = { AgentConnection.drawerCollapsed = !AgentConnection.drawerCollapsed },
                onNewThread = onNewThread,
                onOpenBrowser = onOpenBrowser,
                onClose = { requestClose() },
            )
        }
    }
}

/**
 * 面板本体 —— 与遮罩解耦,方便单独调样式。
 *
 * 【双模式】collapsed=false 展开:品牌头 + 导航分组 + 底部元信息;
 * collapsed=true 收起:76dp 窄 rail,仅图标垂直排列,点图标直接跳转,
 * 长按图标浮出文字提示(Android 无悬停,长按为等价交互)。
 * 宽度切换由外层 animateDpAsState 驱动,内容用 Crossfade 淡切,偏好已持久化。
 * 展开与收起是**同一份恒定导航清单**(buildDrawerNavEntries):顺序、图标、当前项高亮、
 * 任务角标完全一致(README §侧边栏与回退)。
 */
@Composable
private fun DrawerPanel(
    modifier: Modifier = Modifier,
    currentTab: Int,
    hostTitle: String,
    unread: Int,
    collapsed: Boolean,
    onGoTab: (Int) -> Unit,
    onOpenTerminal: () -> Unit,
    onOpenSettings: () -> Unit,
    onOpenTaskCenter: () -> Unit,
    onOpenOnboarding: (() -> Unit)? = null,
    onOpenCollab: (() -> Unit)? = null,
    onOpenAttention: (() -> Unit)? = null,
    attentionBadge: Int = 0,
    onToggleCollapsed: () -> Unit,
    onNewThread: (() -> Unit)? = null,
    onOpenBrowser: (() -> Unit)? = null,
    onClose: () -> Unit,
) {
    // 主导航(恒定集合,2026-10-09 抽屉恒定修复):设计 06 的顺序,任务中心带
    // 未读角标;消息中心/多 Agent 协同**恒渲染**,未接线置灰而非消失(见
    // buildDrawerNavEntries 的 KDoc —— 修"同一分钟内协同条目时有时无"的实锤)。
    val navEntries = buildDrawerNavEntries(
        attentionWired = onOpenAttention != null,
        collabWired = onOpenCollab != null,
    )
    val connLink = homeHostLink(AgentConnection.connState.value)

    Column(
        modifier
            .fillMaxHeight()
            // 【石墨实心层】premium.css .drawer:背景 #1b2634 + 右缘 1dp #384959,
            // 上下缘贴屏幕边(边界靠屏幕边不靠圆角);投影给一档,面板"浮"在内容之上。
            .background(PaEditorial.GraphiteDrawer)
            .border(width = 1.dp, color = DrawerSkin.Edge)
            .shadow(elevation = 10.dp)
            .statusBarsPadding()
            .navigationBarsPadding(),
    ) {
        Crossfade(
            targetState = collapsed,
            animationSpec = PaMotion.standard(),
            label = "drawerMode",
            modifier = Modifier.weight(1f),
        ) { c ->
            if (c) {
                RailContent(
                    currentTab = currentTab,
                    unread = unread,
                    navEntries = navEntries,
                    onGoTab = onGoTab,
                    onOpenTerminal = onOpenTerminal,
                    onOpenSettings = onOpenSettings,
                    onOpenTaskCenter = onOpenTaskCenter,
                    onOpenOnboarding = onOpenOnboarding,
                    onOpenCollab = onOpenCollab,
                    onOpenAttention = onOpenAttention,
                    attentionBadge = attentionBadge,
                    onToggleCollapsed = onToggleCollapsed,
                    onNewThread = onNewThread,
                    onOpenBrowser = onOpenBrowser,
                )
            } else {
                Column(
                    Modifier
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState())
                        .padding(horizontal = 16.dp),
                ) {
                    Spacer(Modifier.height(12.dp))

                    // ---------- 品牌头:logo 方块 + 名称 + 主机连接一行放下 ----------
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Box(
                            Modifier
                                .size(40.dp)
                                .background(DrawerSkin.Logo, RoundedCornerShape(PaEditorial.Radius.IconButton)),
                            contentAlignment = Alignment.Center,
                        ) {
                            Image(
                                bitmap = agentBitmap(R.mipmap.ic_launcher),
                                contentDescription = null,
                                modifier = Modifier.size(32.dp).clip(RoundedCornerShape(PaEditorial.Radius.Chip)),
                                contentScale = ContentScale.Crop,
                            )
                        }
                        Spacer(Modifier.width(12.dp))
                        Column(Modifier.weight(1f)) {
                            Text(
                                "PocketAgent",
                                fontSize = PaEditorial.Type.Heading.fontSize,
                                color = PaEditorial.OnGraphite,
                                fontWeight = FontWeight.SemiBold,
                            )
                            Spacer(Modifier.height(2.dp))
                            DrawerHostStatus(hostTitle = hostTitle, link = connLink)
                        }
                        IconActionButton(
                            R.drawable.ic_lucide_chevron_left, "收起为图标栏", onToggleCollapsed,
                            bg = Color.Transparent, size = 34.dp, iconSize = 17.dp,
                            tint = DrawerSkin.HeadIcon,
                        )
                        Spacer(Modifier.width(2.dp))
                        IconActionButton(
                            R.drawable.ic_lucide_x, "关闭", onClose,
                            bg = Color.Transparent, size = 34.dp, iconSize = 17.dp,
                            tint = DrawerSkin.HeadIcon,
                        )
                    }

                    Spacer(Modifier.height(12.dp))
                    // 头部细线(premium.css .drawer-head border #364655)
                    Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.OnGraphiteHairline))

                    // ---------- 离线可执行说明(空态纪律):电脑没连上时,给一条
                    // "现在能做什么"的路 —— 点它进新手教程的连接步骤,不摆纯报错牌 ----------
                    if (connLink == PaHostLink.OFFLINE && onOpenOnboarding != null) {
                        Spacer(Modifier.height(12.dp))
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .heightIn(min = 44.dp)
                                .pressable(haptic = true) { onOpenOnboarding() }
                                .background(DrawerSkin.ActiveBg, RoundedCornerShape(PaEditorial.Radius.IconButton))
                                .padding(horizontal = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Icon(
                                painterResource(R.drawable.ic_lucide_monitor), null,
                                tint = DrawerSkin.HeadOk, modifier = Modifier.size(18.dp),
                            )
                            Spacer(Modifier.width(12.dp))
                            Column(Modifier.weight(1f)) {
                                Text(
                                    "还没连上电脑",
                                    fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.OnGraphite,
                                    fontWeight = FontWeight.Medium,
                                )
                                Text(
                                    "点这里,看怎么让电脑出现在手机上",
                                    fontSize = PaEditorial.Type.Caption.fontSize, color = DrawerSkin.HeadOk,
                                )
                            }
                            Icon(
                                painterResource(R.drawable.ic_lucide_chevron_right), null,
                                tint = DrawerSkin.HeadOk, modifier = Modifier.size(16.dp),
                            )
                        }
                    }

                    // ---------- 新建对话(仅聊天页传入了该回调) ----------
                    if (onNewThread != null) {
                        Spacer(Modifier.height(12.dp))
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .pressable(haptic = true) { onNewThread() }
                                .background(PaEditorial.Primary, RoundedCornerShape(PaEditorial.Radius.IconButton))
                                .padding(vertical = 12.dp),
                            horizontalArrangement = Arrangement.Center,
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Icon(
                                painterResource(R.drawable.ic_lucide_plus), null,
                                tint = PaEditorial.OnGraphite, modifier = Modifier.size(16.dp),
                            )
                            Spacer(Modifier.width(8.dp))
                            Text(
                                "新建对话", fontSize = PaEditorial.Type.Body.fontSize,
                                color = PaEditorial.OnGraphite, fontWeight = FontWeight.SemiBold,
                            )
                        }
                    }

                    // ---------- 工作空间(主导航,恒定集合;任务中心带未读角标) ----------
                    DrawerGroupLabel("工作空间")
                    navEntries.forEach { e ->
                        DrawerItem(
                            icon = e.spec.icon,
                            title = e.spec.title,
                            selected = e.spec.tab == currentTab,
                            badge = when {
                                e.spec.tab == 4 -> unread
                                e.spec.id == "attention" -> attentionBadge
                                else -> 0
                            },
                            enabled = e.enabled,
                            hint = e.disabledHint,
                            // 动作项按 id 分派;未接线时 enabled=false 点按不可达(置灰不消失)
                            onClick = {
                                when (e.spec.id) {
                                    "attention" -> onOpenAttention?.invoke()
                                    "collab" -> onOpenCollab?.invoke()
                                    else -> onGoTab(e.spec.tab)
                                }
                            },
                        )
                    }

                    // ---------- 更多:工具与本机入口 ----------
                    DrawerGroupLabel("更多")
                    DrawerItem(
                        icon = R.drawable.ic_lucide_square_terminal,
                        title = "终端",
                        selected = false,
                        badge = 0,
                        onClick = onOpenTerminal,
                    )
                    if (onOpenBrowser != null) {
                        DrawerItem(
                            icon = R.drawable.ic_lucide_globe,
                            title = "网页",
                            selected = false,
                            badge = 0,
                            onClick = onOpenBrowser,
                        )
                    }
                    DrawerItem(
                        icon = R.drawable.ic_lucide_settings,
                        title = "设置",
                        selected = false,
                        badge = 0,
                        onClick = onOpenSettings,
                    )
                    if (onOpenOnboarding != null) {
                        DrawerItem(
                            icon = R.drawable.ic_lucide_sparkles,
                            title = "新手教程",
                            selected = false,
                            badge = 0,
                            onClick = onOpenOnboarding,
                        )
                    }

                    Spacer(Modifier.height(16.dp))
                } // 展开滚动内容
            } // Crossfade 双模式
        }

        // ---------- 底部元信息(仅展开态;rail 态空间有限留白) ----------
        if (!collapsed) {
            Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.OnGraphiteHairline))
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                val model = AgentConnection.selectedModel
                Text(
                    model.takeIf { it.isNotEmpty() } ?: "跟随桌面配置",
                    Modifier.weight(1f),
                    fontSize = PaEditorial.Type.Micro.fontSize,
                    color = PaEditorial.OnGraphiteSubtle,
                    maxLines = 1,
                )
                Text("v${BuildConfig.VERSION_NAME}", fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.OnGraphiteSubtle)
            }
        }
    }
}

/**
 * 品牌头下的主机连接行:状态点(连接三态语义色)+ 状态词(hostStatusWord,
 * 四类共享组件的同一套词),展开态专有 —— "我在连哪台电脑"一眼可见。
 */
@Composable
private fun DrawerHostStatus(hostTitle: String, link: PaHostLink) {
    val dot = hostStatusTone(link).accent
    val text = when (link) {
        PaHostLink.CONNECTED -> "$hostTitle · ${hostStatusWord(link)}"
        PaHostLink.CONNECTING -> hostStatusWord(link) // 正在建立连接…
        PaHostLink.OFFLINE -> if (hostTitle == "未连接电脑") hostTitle else "$hostTitle · ${hostStatusWord(link)}"
    }
    Row(verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(6.dp).background(dot, CircleShape))
        Spacer(Modifier.width(6.dp))
        Text(
            text,
            fontSize = PaEditorial.Type.Micro.fontSize,
            color = if (link == PaHostLink.CONNECTED) DrawerSkin.HeadOk else PaEditorial.OnGraphiteSubtle,
            maxLines = 1,
        )
    }
}

/**
 * 收起态窄 rail:仅图标垂直排列,与展开态共用 [navEntries](同一恒定集合:
 * 同序/同图标/同高亮/同角标/同置灰态)。点图标直接执行跳转;长按浮出文字提示
 * (≈桌面 hover tooltip,1.5s 后自动消失),置灰项的长按提示带原因词。
 */
@Composable
private fun RailContent(
    currentTab: Int,
    unread: Int,
    navEntries: List<DrawerNavEntry>,
    onGoTab: (Int) -> Unit,
    onOpenTerminal: () -> Unit,
    onOpenSettings: () -> Unit,
    onOpenTaskCenter: () -> Unit,
    onOpenOnboarding: (() -> Unit)?,
    onOpenCollab: (() -> Unit)?,
    onOpenAttention: (() -> Unit)?,
    attentionBadge: Int,
    onToggleCollapsed: () -> Unit,
    onNewThread: (() -> Unit)? = null,
    onOpenBrowser: (() -> Unit)? = null,
) {
    Column(
        Modifier
            .fillMaxSize()
            .padding(horizontal = 16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Spacer(Modifier.height(16.dp))
        // Logo:点按展开完整面板(折叠态的"找回文字"入口);44dp = TouchMin 点击区
        Box(
            Modifier
                .size(44.dp)
                .clip(RoundedCornerShape(PaEditorial.Radius.IconButton))
                .pressable { onToggleCollapsed() },
            contentAlignment = Alignment.Center,
        ) {
            Image(
                bitmap = agentBitmap(R.mipmap.ic_launcher),
                contentDescription = "展开侧边栏",
                contentScale = ContentScale.Crop,
                modifier = Modifier.size(44.dp),
            )
        }
        Spacer(Modifier.height(16.dp))
        if (onNewThread != null) {
            RailIcon(R.drawable.ic_lucide_plus, "新建对话", onClick = onNewThread)
            Spacer(Modifier.height(8.dp))
        }
        // 主导航:与展开态同一份恒定清单、同一顺序、同一角标;未接线置灰而非消失
        navEntries.forEach { e ->
            RailIcon(
                e.spec.icon,
                // rail 没有文字:置灰原因词并进长按提示/读屏名,不摆哑巴灰块
                label = if (e.enabled) e.spec.title
                else "${e.spec.title}(${e.disabledHint})",
                selected = e.spec.tab == currentTab,
                badge = when {
                    e.spec.tab == 4 -> unread
                    e.spec.id == "attention" -> attentionBadge
                    else -> 0
                },
                enabled = e.enabled,
            ) {
                when (e.spec.id) {
                    "attention" -> onOpenAttention?.invoke()
                    "collab" -> onOpenCollab?.invoke()
                    else -> onGoTab(e.spec.tab)
                }
            }
            Spacer(Modifier.height(8.dp))
        }
        RailDivider()
        RailIcon(R.drawable.ic_lucide_square_terminal, "终端", onClick = onOpenTerminal)
        Spacer(Modifier.height(8.dp))
        if (onOpenBrowser != null) {
            RailIcon(R.drawable.ic_lucide_globe, "网页", onClick = onOpenBrowser)
            Spacer(Modifier.height(8.dp))
        }
        RailIcon(R.drawable.ic_lucide_settings, "设置", onClick = onOpenSettings)
        Spacer(Modifier.height(8.dp))
        if (onOpenOnboarding != null) {
            RailIcon(R.drawable.ic_lucide_sparkles, "新手教程", onClick = onOpenOnboarding)
            Spacer(Modifier.height(8.dp))
        }
        Spacer(Modifier.weight(1f))
        RailIcon(R.drawable.ic_lucide_chevron_right, "展开", onClick = onToggleCollapsed)
        Spacer(Modifier.height(12.dp))
    }
}

/** rail 分隔细线(石墨面上的 #364655)。 */
@Composable
private fun RailDivider() {
    Box(
        Modifier
            .padding(vertical = 8.dp)
            .width(32.dp)
            .height(1.dp)
            .background(PaEditorial.OnGraphiteHairline),
    )
}

/**
 * rail 图标钮:44dp 圆角方块(≥44dp 点击区);选中态 .drawer-link.active 同款
 * #31475e 底 + 近白图标;长按在右侧浮出文字提示。
 * [enabled]=false 置灰(石墨面的 Muted 槽 OnGraphiteSubtle):点按无动作,
 * 长按提示照常浮出(label 已并进原因词)—— 条目恒在,能力如实。
 */
@Composable
private fun RailIcon(
    iconRes: Int,
    label: String,
    selected: Boolean = false,
    badge: Int = 0,
    enabled: Boolean = true,
    onClick: () -> Unit,
) {
    var showTip by remember { mutableStateOf(false) }
    LaunchedEffect(showTip) {
        if (showTip) {
            delay(1500)
            showTip = false
        }
    }
    val tint = when {
        !enabled -> PaEditorial.OnGraphiteSubtle
        selected -> DrawerSkin.ActiveText
        else -> DrawerSkin.RailIcon
    }
    Box {
        Box(
            Modifier
                .size(44.dp)
                .pressableLong(
                    onClick = { if (enabled) onClick() },
                    onLongPress = { showTip = true },
                )
                .background(
                    if (selected) DrawerSkin.ActiveBg else Color.Transparent,
                    RoundedCornerShape(PaEditorial.Radius.IconButton),
                ),
            contentAlignment = Alignment.Center,
        ) {
            DrawerBadge(count = if (enabled) badge else 0) {
                Icon(
                    painterResource(iconRes), label,
                    tint = tint,
                    modifier = Modifier.size(20.dp),
                )
            }
        }
        if (showTip) {
            val tipOffset = with(LocalDensity.current) { IntOffset(10.dp.roundToPx(), 0) }
            Popup(alignment = Alignment.CenterEnd, offset = tipOffset) {
                Text(
                    label,
                    Modifier
                        .background(PaEditorial.OnGraphite, RoundedCornerShape(PaEditorial.Radius.Chip))
                        .padding(horizontal = 10.dp, vertical = 5.dp),
                    color = PaEditorial.GraphiteDrawer,
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    maxLines = 1,
                )
            }
        }
    }
}

/** 分组小标题:字号小、加字距(.drawer-section #8da0b0),分区标签的"轻层级"靠它。 */
@Composable
private fun DrawerGroupLabel(text: String) {
    Text(
        text,
        Modifier.padding(start = 12.dp, top = 20.dp, bottom = 8.dp),
        fontSize = PaEditorial.Type.Micro.fontSize,
        color = DrawerSkin.Section,
        fontWeight = FontWeight.Medium,
        letterSpacing = 1.4.sp, // 中文字距拉开一档
    )
}

/**
 * 导航行:.drawer-link 语言 —— 46dp 最小高度、12dp 圆角、图标 + 单行文字;
 * 选中态 #31475e 底 + #f5faff 字(当前页高亮);任务中心行尾挂未读角标。
 * [enabled]=false 置灰态(恒定修复):图标/文字落石墨面的 Muted 槽
 * (OnGraphiteSubtle),原因词落字(状态不单靠灰度),点按无动作,角标隐藏。
 */
@Composable
private fun DrawerItem(
    icon: Int,
    title: String,
    selected: Boolean,
    badge: Int,
    onClick: () -> Unit,
    enabled: Boolean = true,
    hint: String? = null,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 1.dp)
            .pressable(haptic = true, enabled = enabled) { onClick() }
            .background(
                if (selected) DrawerSkin.ActiveBg else Color.Transparent,
                RoundedCornerShape(PaEditorial.Radius.IconButton),
            )
            .padding(start = 12.dp, end = 12.dp)
            .heightIn(min = 48.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(icon), null,
            tint = when {
                !enabled -> PaEditorial.OnGraphiteSubtle
                selected -> DrawerSkin.ActiveText
                else -> DrawerSkin.Link
            },
            modifier = Modifier.size(18.dp),
        )
        Spacer(Modifier.width(12.dp))
        Text(
            title,
            Modifier.weight(1f),
            fontSize = PaEditorial.Type.BodySmall.fontSize,
            color = when {
                !enabled -> PaEditorial.OnGraphiteSubtle
                selected -> DrawerSkin.ActiveText
                else -> DrawerSkin.Link
            },
            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
            maxLines = 1,
        )
        if (!enabled && hint != null) {
            // 置灰原因词(状态词落字):「在主页查看」/「未接入」,真实原因不哑灰
            Text(
                hint,
                fontSize = PaEditorial.Type.Micro.fontSize,
                color = PaEditorial.OnGraphiteSubtle,
                maxLines = 1,
            )
        }
        if (enabled && badge > 0) {
            DrawerBadgeText(count = badge)
        }
    }
}

/** 角标锚点(rail 图标用):有角标时叠在图标右上。 */
@Composable
private fun DrawerBadge(count: Int, content: @Composable () -> Unit) {
    if (count > 0) {
        Box(contentAlignment = Alignment.TopEnd) {
            Box(Modifier.padding(top = 2.dp, end = 2.dp)) { content() }
            Box(
                Modifier
                    .align(Alignment.TopEnd)
                    .background(DrawerSkin.Badge, RoundedCornerShape(PaEditorial.Radius.Terminal))
                    .padding(horizontal = 4.dp, vertical = 1.dp),
            ) {
                Text(
                    if (count > 99) "99+" else count.toString(),
                    fontSize = PaEditorial.Type.State.fontSize, lineHeight = 11.sp,
                    color = PaEditorial.OnGraphite, fontWeight = FontWeight.SemiBold,
                )
            }
        }
    } else {
        content()
    }
}

/** 行尾文字角标(展开态导航行用;.drawer-link .badge #9d6038 白字)。 */
@Composable
private fun DrawerBadgeText(count: Int) {
    Box(
        Modifier
            .background(DrawerSkin.Badge, RoundedCornerShape(PaEditorial.Radius.Terminal))
            .padding(horizontal = 6.dp, vertical = 3.dp),
    ) {
        Text(
            if (count > 99) "99+" else count.toString(),
            fontSize = PaEditorial.Type.Micro.fontSize, lineHeight = 12.sp,
            color = PaEditorial.OnGraphite, fontWeight = FontWeight.SemiBold,
        )
    }
}
