package dev.pocketagent.app.ui

import android.widget.Toast
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorMatrix
import androidx.compose.ui.graphics.ColorMatrixColorFilter
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.edgeSwipeToOpenDrawer
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.component.ConnectionBanner
import dev.pocketagent.app.ui.component.DiscoveredHost
import dev.pocketagent.app.ui.component.HomeAgentTileState
import dev.pocketagent.app.ui.component.HomeEmptyStateCopy
import dev.pocketagent.app.ui.component.PaHaptics
import dev.pocketagent.app.ui.component.PaHostChip
import dev.pocketagent.app.ui.component.PaHostLink
import dev.pocketagent.app.ui.component.PaErrorBar
import dev.pocketagent.app.ui.component.PaNavIconButton
import dev.pocketagent.app.ui.component.PaRunningEnergyLine
import dev.pocketagent.app.ui.component.PaRunningSessionUi
import dev.pocketagent.app.ui.component.PaScreenScaffold
import dev.pocketagent.app.ui.component.PaSessionBar
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.component.PaSkeletonList
import dev.pocketagent.app.ui.component.PaStatusBadge
import dev.pocketagent.app.ui.component.SkeletonKind
import dev.pocketagent.app.ui.component.homeAgentGridNote
import dev.pocketagent.app.ui.component.homeAgentGridColumns
import dev.pocketagent.app.ui.component.homeAgentTileLabel
import dev.pocketagent.app.ui.component.homeAgentTileState
import dev.pocketagent.app.ui.component.homeDiscoveryEmptyCopy
import dev.pocketagent.app.ui.component.homeRecentEmptyCopy
import dev.pocketagent.app.ui.component.homeShowsRecentSkeleton
import dev.pocketagent.app.ui.component.paErrorViewOf
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.editorial
import dev.pocketagent.app.ui.theme.paPushBack
import dev.pocketagent.app.ui.attention.AttentionCenterState
import dev.pocketagent.app.ui.attention.AttentionCenterUiState
import dev.pocketagent.app.ui.attention.AttentionFilter
import dev.pocketagent.app.ui.attention.AttentionItem
import dev.pocketagent.app.ui.attention.approvalAttentionItems
import dev.pocketagent.app.ui.attention.attentionFreshnessLabel
import dev.pocketagent.app.ui.attention.attentionNeedsMeCount
import dev.pocketagent.app.ui.attention.filterAttention
import dev.pocketagent.app.ui.attention.recoveryAttentionItem
import dev.pocketagent.app.ui.attention.topAttentionSummary
import dev.pocketagent.app.ui.attention.unknownEventAttentionItems
import androidx.core.graphics.drawable.toBitmap
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CreateThreadRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.HostInfo
import pocketagent.v1.ListThreadsRequest
import pocketagent.v1.Thread as PThread
import pocketagent.v1.ThreadStatus

// ============================================================================
// 编辑式首页(UI 整体升级 · 第二波 1/4 首页组;方向 C 大卡台阶 2026-10-09)
//
// 【信息架构】docs/design/2026-09-29-pocketagent-ui README §页面与主操作 基础上,
// 按 docs/design/ui-language-research-20261009.md 方向 C 的 N5 大卡台阶重排视觉
// 权重(数据流零变化,只改"谁占哪一级"):
//   顶部统一主机状态(PaHostChip)
//   → 运行中会话=大卡(第一级:Graphite 暗面 + 运行态能量细线 + 实时状态语,
//     占双行高;.run-hero)
//   → 待审批=中卡(第二级:白面 + Attention 琥珀系边,.alert 降档)
//   → 新建(主色仅此处当前操作,.new-task)→ 可用 Agent → 多 agent 协同
//     (能力驱动)→ 最近会话(留白 + 细分隔线,不套多层卡)= 其余小卡级。
// 【双层世界】方向 C N1:运行中会话另有底部 mini 条(PaSessionBar,系统导航栏
//   上方常驻悬浮),点按直达会话页 —— Chat 页内不挂载(会话页自带沉浸态)。
// 【发现主机态】screens/01-home-discovery.png:eyebrow + 34/39 Display 主标题 +
//   发现圆环 + 「发现的电脑」卡(选主机 → 输配对码),手动添加是次操作。
// 【令牌纪律】整页取 MaterialTheme.editorial.*(固定浅色盘);石墨深色只出现在
//   运行中大卡、待审批语义与管理抽屉(README:层次而非满屏深色;C2 铁律:
//   暗面=Graphite 专属,禁止自调深色)。管理/引导/配对浮层仍走玻璃
//   PaSheet(基座组件,随主题换肤),其内部保留 Pa.* 与浮层皮肤一致 —— 固定浅色
//   的编辑式墨字不上玻璃,否则深色模式下不可读。
// ============================================================================

/** 首页 agent 清单:iconRes 为官方品牌图标,method 展示连接方式(对标图三)。 */
data class AgentInfo(
    val id: String,
    val name: String,
    val method: String,  // 通过什么方式连接桌面端 agent(网格小字 + 管理抽屉)
    val subtitle: String,
    val iconRes: Int,
    val available: Boolean,
)

/**
 * agent 网格可用性判定(纯函数,单测覆盖 AgentAvailabilityTest)。
 *
 * 【P1-6 动态化后的点亮规则 —— 能力判定一律以 daemon 动态数据为唯一真源】
 *  - 基线可用(baseAvailable,如 codex)恒点亮;
 *  - 其余一律看 HostInfo.capabilities 是否含 "agent:<id>" 前缀 ——
 *    旧格式 "agent:pi" 与新格式 "agent:pi:v0.85.1"(带探测版本号)都点亮,
 *    "agent:pika" 这类别的 id 不会误匹配,大小写不敏感兜一手;
 *  - 不认识的新 agent id(daemon 侧新增第 4 个 provider)无需改本文件,
 *    daemon 报了 "agent:<新id>" 就自动点亮(README §7:静态名单是债,能透传就透传)。
 */
internal fun resolveAgentAvailability(agentId: String, baseAvailable: Boolean, capabilities: List<String>): Boolean =
    baseAvailable || capabilities.any {
        it.equals("agent:$agentId", ignoreCase = true) ||
            it.startsWith("agent:$agentId:", ignoreCase = true)
    }

/**
 * 【展示目录,非能力名单】尚未接入 daemon 数据源的占位 agent
 * (opencode/claude-code/hermes/dsh):磁贴作为「接入中」的展示位保留。
 *
 * 【与能力判定的边界(P1-6 动态化)】任何 agent 的可用性一律由
 * HostInfo.capabilities("agent:<id>")动态决定(见 [resolveAgentAvailability]),
 * 本集合只决定灰显态的副文字口径("接入中" vs "电脑端未安装"),不参与点亮。
 * daemon 侧新增的 agent id 不在本集合 → 自动按能力驱动型展示(灰显="电脑端未安装"),
 * 因此本集合只随 agent 真正接入而**减少**、不随新 agent 增加(白名单只减不增)。
 */
internal val placeholderDisplayAgents = setOf("opencode", "claude-code", "hermes", "dsh")

/**
 * 从 capabilities 提取 agent 已装版本(纯函数):daemon 报 "agent:pi:v0.85.1"
 * → "v0.85.1";旧 daemon 只报 "agent:pi"(无版本)或未装 → null。
 * 引导弹窗据此显示"已装 vX"与"未安装"两种态。
 */
internal fun resolveAgentInstalledVersion(agentId: String, capabilities: List<String>): String? =
    capabilities.firstOrNull { it.startsWith("agent:$agentId:", ignoreCase = true) }
        ?.split(":")?.getOrNull(2)?.takeIf { it.isNotBlank() }

/**
 * 灰显磁贴副文字(纯函数):占位展示目录内=接入中(daemon 无此 agent 的数据源);
 * 其余(含 daemon 动态报上来的新 id)=电脑端未安装(真实原因,替代过时的"开发中")。
 */
internal fun unavailableLabel(agentId: String): String =
    if (agentId in placeholderDisplayAgents) AgentGuideCopy.SUBTITLE_INTEGRATING
    else AgentGuideCopy.SUBTITLE_NOT_INSTALLED

/**
 * 引导弹窗文案(集中常量,便于后续调整)。detect=判断依据(与 daemon 侧探测
 * 逻辑一一对应:piapp.LocatePi/Version、zcodeapp.LocateZCode、codexapp.LocateCodex),
 * install=安装指引。
 */
internal data class AgentGuide(val detect: String, val install: String)

internal object AgentGuideCopy {
    /** 灰显磁贴副文字:能力驱动型 agent 的真实原因。 */
    const val SUBTITLE_NOT_INSTALLED = "电脑端未安装"
    /** 灰显磁贴副文字:尚未接入的基线 agent。 */
    const val SUBTITLE_INTEGRATING = "接入中"

    // ---- 判断依据(daemon 侧探测逻辑)----
    const val DETECT_PI = "怎么判断的：电脑端需能定位 node 与 pi 的 cli.js，且 --version 能出版本号（PI_NODE / PI_CLI 可指定路径）"
    const val DETECT_ZCODE = "怎么判断的：电脑端需同时存在 node 与 ZCode 桌面端的 zcode.cjs（ZCODE_NODE / ZCODE_CJS 可指定路径）"
    const val DETECT_CODEX = "怎么判断的：电脑端需能在 PATH 或 CODEX_BIN 里定位 codex 可执行文件"
    const val DETECT_INTEGRATING = "该 agent 尚未接入 PocketAgent，暂不走电脑端探测"

    // ---- 安装指引 ----
    // 【事实性口径】provider 是 daemon 启动时 DetectProviders 注册的:装好后必须重启
    // 电脑端 daemon 才会点亮,手机端"重连"不够(磁贴智能体原文有偏差,已修正)。
    const val INSTALL_PI = "电脑端未检测到 Pi coding agent。安装：npm install @earendil-works/pi-coding-agent，装好后重启电脑端 daemon 自动点亮"
    const val INSTALL_ZCODE = "未检测到 ZCode 桌面端（node + resources/glm/zcode.cjs）。安装 ZCode 桌面版后重启电脑端 daemon 自动点亮"
    const val INSTALL_CODEX = "未检测到 codex CLI（CODEX_BIN 或 PATH）。安装 OpenAI Codex CLI 后重启电脑端 daemon 自动点亮"
    const val INSTALL_INTEGRATING = "敬请期待后续版本"
}

/** 弹窗文案选择(纯函数,单测覆盖):按 agent id 取判断依据 + 安装指引。 */
internal fun agentGuideCopy(agentId: String): AgentGuide = when (agentId) {
    "pi" -> AgentGuide(AgentGuideCopy.DETECT_PI, AgentGuideCopy.INSTALL_PI)
    "zcode" -> AgentGuide(AgentGuideCopy.DETECT_ZCODE, AgentGuideCopy.INSTALL_ZCODE)
    "codex" -> AgentGuide(AgentGuideCopy.DETECT_CODEX, AgentGuideCopy.INSTALL_CODEX)
    else -> AgentGuide(AgentGuideCopy.DETECT_INTEGRATING, AgentGuideCopy.INSTALL_INTEGRATING)
}

/**
 * 网格清单(纯函数,单测覆盖):基线表 + capabilities 动态点亮 + daemon 新增 agent 动态追加。
 *
 * 【P1-6 边界说明】基线 7 项是**展示目录**(展示名/图标/连接方式文案),不是能力名单:
 *  - 可用性一律由 [resolveAgentAvailability] 按 capabilities 动态判定,基线表不点亮任何东西;
 *  - opencode/claude-code/hermes/dsh 为占位展示项(daemon 无数据源,见 placeholderDisplayAgents);
 *  - daemon 报了基线表没有的 "agent:<新id>" → 走 extra 追加,展示名缺省回退 **id 原文**
 *    (AUDIT §3.3:未知值显示原生原文),图标用通用 CPU 图标 —— 新 agent 零改动自动进网格。
 */
internal fun buildAgents(capabilities: List<String>): List<AgentInfo> = listOf(
    AgentInfo("codex", "Codex", "app-server", "Codex app-server 后端", R.drawable.agent_codex, true),
    AgentInfo("opencode", "OpenCode", "stdio", "HTTP + SSE 会话后端", R.drawable.agent_opencode, false),
    AgentInfo("claude-code", "Claude", "stdio", "stream-json stdio 后端", R.drawable.agent_claudecode, false),
    AgentInfo("pi", "Pi", "stdio", "RPC stdio 后端", R.drawable.agent_pi, false),
    AgentInfo("hermes", "Hermes", "ACP", "ACP stdio 后端", R.drawable.agent_hermes, false),
    AgentInfo("dsh", "DSH", "网页", "网页版 agent(浏览器接入)", R.drawable.agent_dsh, false),
    AgentInfo("zcode", "ZCode", "app-server", "ZCode app-server 后端", R.drawable.agent_zcode, false),
).map { it.copy(available = resolveAgentAvailability(it.id, it.available, capabilities)) }.let { known ->
    val knownIds = known.map { it.id }.toSet()
    val extra = capabilities.filter { it.startsWith("agent:") }.mapNotNull { it.split(":").getOrNull(1) }
        .distinct().filter { it !in knownIds && it != "echo" }.map { id ->
            AgentInfo(id, id, "native", "电脑端适配器", R.drawable.ic_lucide_cpu, true)
        }
    known + extra
}

/** 加载 drawable-nodpi 的 agent PNG 为 ImageBitmap。 */
@Composable
fun agentBitmap(res: Int): androidx.compose.ui.graphics.ImageBitmap {
    val ctx = androidx.compose.ui.platform.LocalContext.current
    return remember(res) {
        androidx.core.content.ContextCompat.getDrawable(ctx, res)!!.toBitmap(256, 256)
            .asImageBitmap()
    }
}

/** 不可用 agent:灰阶 + 降透明(高对比区分)。 */
fun grayscale() = ColorMatrixColorFilter(ColorMatrix().apply { setToSaturation(0f) })

/**
 * 连接流状态 → 主机胶囊三态(纯函数,单测覆盖 HomeEditorialStateTest)。
 *
 * 【口径】connState 只有 ok/checking/down 三值;"正在执行"是会话/任务的状态词,
 * 与连接态严格分家(PaHostChip 的构造性保证),这里只做翻译不做加工。
 */
internal fun homeHostLink(connState: String): PaHostLink = when (connState) {
    "ok" -> PaHostLink.CONNECTED
    "down" -> PaHostLink.OFFLINE
    else -> PaHostLink.CONNECTING
}

/**
 * 离线时主机胶囊的「上次更新」文本(纯函数,单测覆盖 HomeEditorialStateTest):
 * 有成功加载时刻 → 相对时间;没有 → null(胶囊自己会讲「暂无更新记录」,
 * 绝不出现任何"正在…"的运行措辞 —— hostChipCaption 的构造性保证)。
 */
internal fun lastUpdateText(lastLoadedAtMs: Long, nowMs: Long = System.currentTimeMillis()): String? =
    if (lastLoadedAtMs > 0L) relativeTimeLabel(lastLoadedAtMs, nowMs) else null

// ---------------------------------------------------------------- 首页主体

/**
 * 主页 —— 编辑式新设计(02 已连接态 / 01 发现主机态)。
 *
 * 【结构】PaScreenScaffold(菜单 + 标题 + 设置,主机状态槽)→ 内容:
 *  - 已连接:待审批(石墨卡)→ 正在进行(横条 + 「正在执行」徽章)→ 新建(主色)→
 *    可用 Agent → 多 agent 协同(能力驱动)→ 最近会话(留白 + 细分隔线);
 *  - 未连接:发现主机(eyebrow + Display 主标题 + 发现圆环 + 主机卡 → 配对码),
 *    手动添加是次动作;离线时**绝不渲染**任何「正在执行」区块。
 */
@Composable
fun HomeScreen(
    currentTab: Int = 0,
    /** 每次自增表示"请展开侧边栏"(由返回层级触发)。 */
    openDrawerSignal: Int = 0,
    /** 主页是返回层级的终点,这里仅保持签名一致(不拦截返回)。 */
    onBackToHome: () -> Unit = {},
    onOpenThread: (String) -> Unit,
    onOpenTerminal: () -> Unit,
    onOpenSettings: () -> Unit,
    onOpenTaskCenter: () -> Unit = {},
    /** 重看新手教程(透传给侧边栏「更多」入口)。 */
    onOpenOnboarding: () -> Unit = {},
    onOpenBrowser: () -> Unit = {},
    /** 协同角色配置入口:仅在设备确实支持协同时由本页调用(无能力不显示假入口)。 */
    onOpenCollabRoles: () -> Unit = {},
    /** 协同扇出主页入口:一条指令派给多个对等 agent(同样只在下述 multiAgent 分支里露出)。 */
    onOpenCollab: () -> Unit = {},
    onGoTab: (Int) -> Unit = {},
    modifier: Modifier = Modifier,
) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val view = LocalView.current
    var host by remember { mutableStateOf<HostInfo?>(null) }
    var recent by remember { mutableStateOf(listOf<PThread>()) }
    var showManage by remember { mutableStateOf(false) }
    // 灰显磁贴点击后弹"为什么不可用、怎么装"的引导弹窗(非 null 即展示)
    var guideAgent by remember { mutableStateOf<AgentInfo?>(null) }
    // 侧边栏跟手(与会话页同一套 controller)
    val drawerCtrl = dev.pocketagent.app.ui.component.rememberDrawerController()

    // 返回层级:从二级页面返回主页时自动展开侧边栏
    LaunchedEffect(openDrawerSignal) {
        if (openDrawerSignal > 0) drawerCtrl.open()
    }
    var reloadKey by remember { mutableStateOf(0) }
    var loadError by remember { mutableStateOf<String?>(null) }
    var showNewThread by remember { mutableStateOf(false) }
    // 配对流程(01 发现态):选中的主机(非 null 即弹配对码浮层;失败保留输入可重试)
    var pairHost by remember { mutableStateOf<DiscoveredHost?>(null) }
    // 自动重连恢复后(connectionEpoch 变化)自动刷新,拔 USB 再回来无需手点重试
    val reconnectTick = AgentConnection.connectionEpoch.collectAsState().value
    // 最近一次数据加载成功时刻(离线时给 PaHostChip 的「上次更新」供数)
    var lastLoadedAtMs by remember { mutableStateOf(0L) }
    // 首次加载是否已落定(成功或失败都算):列表区的骨架只在首次加载窗口出现,
    // 重试/重连后的刷新不重置 —— 已有内容不闪骨架(判定走 homeShowsRecentSkeleton)
    var firstLoadDone by remember { mutableStateOf(false) }

    LaunchedEffect(reloadKey, reconnectTick) {
        try {
            withContext(Dispatchers.IO) {
                // 多候选回退:当前地址 → USB 反向 → Tailscale
                val info = AgentConnection.connectWithFallback()
                    ?: AgentConnection.hostService().getHostInfo(GetHostInfoRequest.getDefaultInstance())
                host = info
                // T-DSH 问题 A:成功快照写入共享缓存,三条渲染链(主页网格/引导弹窗/
                // 设置页切换 Agent)读同一份,不再各自为政。
                AgentConnection.updateHostCapabilities(info.capabilitiesList)
                recent = AgentConnection.threadService()
                    .listThreads(ListThreadsRequest.newBuilder().build())
                    .threadsList
                loadError = null
                lastLoadedAtMs = System.currentTimeMillis()
            }
        } catch (e: kotlin.coroutines.cancellation.CancellationException) {
            // effect 因 reloadKey/重连 epoch 变化被重启时协程会收到取消,
            // 这是正常重启不是加载失败,别把"The coroutine scope left the
            // composition"写进错误横幅(旧实现重试一次挂一次的根因)
            throw e
        } catch (e: Exception) {
            loadError = "首页数据加载失败：${e.message ?: "连接失败"}"
        }
        // 失败也算"落定":骨架退场,改由错误横幅 + 空态接管(骨架绝不替失败状态撒谎)
        firstLoadDone = true
    }

    val connState = AgentConnection.connState.collectAsState().value
    val connTransport = AgentConnection.connTransport.collectAsState().value
    val link = homeHostLink(connState)
    val connected = link == PaHostLink.CONNECTED

    // agent 网格:可用性按 HostInfo.capabilities 动态判定(daemon 侧报 "agent:<id>"
    // 才点亮;检测不到维持灰显,daemon 部署后自然点亮)。
    // 【T-DSH 问题 A:统一数据源】capabilities 一律读共享快照(hostCapabilities,
    // 探活循环/回退连接/设置页加载都会写入)—— 本页自己的请求失败或未就绪时,
    // 网格仍然反映最近一次 daemon 真值,不再渲染"只剩 codex 可选"的假灰显。
    val sharedCaps = AgentConnection.hostCapabilities.collectAsState().value
    val agents = remember(sharedCaps) { buildAgents(sharedCaps) }
    // 【多agent协同 = 真实状态】是否"已接入协同"不再是用户可拨的开关,而是
    // daemon 接线事实的回显(capabilities 里的 collab:<id>) —— 旧实现做成可拨
    // 开关,结果开关能拨、功能没有 = 假可用(用户实锤)。
    val collabIds = remember(sharedCaps) { resolveCollabAgentIds(sharedCaps) }
    val multiAgent = collabIds.isNotEmpty()
    // 支持协同时,顺带把协同角色列表拉一次,供首页这行的数量展示;
    // 失败/无数据都不影响首页(宁缺勿假,数量留空即不显示数字)。
    LaunchedEffect(collabIds) {
        if (collabIds.isNotEmpty()) CollabRolesState.load()
    }
    // 停用偏好生效链(AgentConnection.disabledAgents 可观察,拨开关立即重算):
    // 已停用 agent 从网格/协同行/最近会话隐藏 —— 管理抽屉副标题「停用后不再显示」的兑现;
    // 恢复入口恒在管理抽屉(它收全量 agents 清单)。偏好层只隐藏,不改写 available,
    // 与 T-10 的 capabilities 能力判定互不覆盖(恢复后判定照旧)。
    val disabledAgents = AgentConnection.disabledAgents
    val visibleAgents = remember(agents, disabledAgents) { filterAgents(agents, disabledAgents) }

    // 【P0-3 注意力三分】待审批 / 运行中 / 其余,数据就是本页现有的 listThreads
    // 应答(daemon 每条 Thread 都带 status 与 pending_approvals 快照,零新增 RPC);
    // 刷新节奏与最近会话同源(reloadKey / 重连 epoch),不做额外轮询。
    val attention = remember(recent, disabledAgents) { partitionHomeAttention(recent, disabledAgents) }
    // agent 磁贴「正在执行」小徽章:该 agent 名下有运行中会话(已剔除停用)就点亮
    val runningAgentIds = remember(attention) { agentIdsOfRunning(attention.running) }

    // 【方向 C · N1 双层世界】运行中会话 mini 条的数据源:TurnSession 既有的
    // peek/runningThreadIds 只读快照(5s 轮询,不新增语义、不算访问)∪ 本页
    // 既有 listThreads 三分的 attention.running(daemon 快照,带标题/归属/起点)。
    // 并集去重映射成展示模型(见 buildPaRunningSessions 纯函数,单测钉死);
    // mini 条只在 Home 世界挂载(下方 Box),Chat 页自带沉浸态不挂载。
    var liveRunningIds by remember { mutableStateOf(setOf<String>()) }
    LaunchedEffect(Unit) {
        while (true) {
            liveRunningIds = TurnSession.runningThreadIds()
            delay(5_000)
        }
    }
    val barSessions = remember(attention, liveRunningIds, recent) {
        buildPaRunningSessions(attention.running, liveRunningIds, recent, System.currentTimeMillis())
    }

    // 【Task 5 Attention Center】电脑端新消息:具备 runtime.v1 且在线时轻量刷新一次
    // (单路 RPC,与设置页共用 RuntimeControlState.unknownEvents 同一份真值,不建
    // 第二数据源);离线时不打 —— 中心用本地缓存 + 时间戳如实呈现。
    val runtimeCapable = remember(sharedCaps) { runtimeControlCapable(sharedCaps) }
    LaunchedEffect(connected, runtimeCapable) {
        if (connected && runtimeCapable) RuntimeControlState.refreshUnknownEvents()
    }
    val unknownCards = remember(RuntimeControlState.unknownEvents) {
        RuntimeControlState.unknownEvents.map { unknownEventCardModel(it) }
    }
    // 注意项清单 = 既有数据源的组装(未知事件 + 待审批 + 启动恢复),全纯函数映射。
    val attentionItems = remember(unknownCards, attention, host) {
        val recoveryReport = host?.takeIf { it.hasRecovery() }?.recovery
        buildList<AttentionItem> {
            addAll(unknownEventAttentionItems(unknownCards))
            addAll(approvalAttentionItems(attention.pendingApproval))
            recoveryAttentionItem(recoveryReport)?.let { add(it) }
        }
    }
    val attentionNeedsMe = remember(attentionItems) { attentionNeedsMeCount(attentionItems) }
    val topAttention = remember(attentionItems) { topAttentionSummary(attentionItems) }
    var showAttentionCenter by remember { mutableStateOf(false) }

    // 【推背模糊】任一浮层(侧边栏/管理面板/引导弹窗/新建任务/配对/消息中心)激活时,
    // 主页内容根模糊+缩放+压暗。模糊挂在被覆盖的内容层上(整屏一次一层),不在浮层自己身上。
    val overlayActive = drawerCtrl.visible || showManage || guideAgent != null ||
        showNewThread || pairHost != null || showAttentionCenter

    // mDNS 发现列表(01 发现态的「发现的电脑」数据源;探活循环已 ensureStarted)
    val discoveredHosts = AgentConnection.mdnsDiscovery
        ?.hosts?.collectAsState()?.value.orEmpty()

    // 系统导航栏 insets(手势条/三键):mini 条的贴底让位与列表尾部让位共用
    val bottomBarInset = WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding()
    val density = LocalDensity.current
    var sessionBarHeightPx by remember { mutableStateOf(0) }
    val sessionBarSpace = with(density) { sessionBarHeightPx.toDp() }

    Box(Modifier.fillMaxSize()) {
        PaScreenScaffold(
            title = if (connected) "工作台" else "连接电脑",
            modifier = modifier
                .fillMaxSize()
                .paPushBack(overlayActive)
                .edgeSwipeToOpenDrawer(
                    onDragFraction = drawerCtrl::onDrag,
                    onRelease = drawerCtrl::onRelease,
                ),
            leading = {
                PaNavIconButton(
                    onClick = { drawerCtrl.open() },
                    contentDescription = "菜单",
                    iconRes = R.drawable.ic_lucide_menu,
                    plain = true,
                )
            },
            trailing = {
                PaNavIconButton(
                    onClick = onOpenSettings,
                    contentDescription = "设置",
                    iconRes = R.drawable.ic_lucide_settings,
                )
            },
            hostStatus = if (connected) {
                {
                    PaHostChip(
                        hostName = host?.displayName ?: "电脑",
                        link = link,
                        channelLabel = connTransport.ifBlank { null },
                        lastUpdateText = lastUpdateText(lastLoadedAtMs),
                        onClick = onOpenSettings, // 主机状态 → 连接管理(README §页面骨架)
                    )
                }
            } else {
                null
            },
        ) {
            LazyColumn(
                Modifier.fillMaxSize(),
                // mini 条可见时给列表尾部让位(条高 ~58 + 间距 12 + 导航栏 insets),
                // 最后一张卡不被悬浮条盖住;条退场后恢复原节奏
                contentPadding = PaddingValues(
                    bottom = if (barSessions.isNotEmpty()) maxOf(sessionBarSpace, 70.dp + bottomBarInset) else bottomBarInset,
                ),
            ) {
                if (connected) {
                    connectedHomeItems(
                        loadError = loadError,
                        onRetry = { reloadKey++ },
                        firstLoadDone = firstLoadDone,
                        attention = attention,
                        attentionExists = attentionItems.isNotEmpty(),
                        attentionNeedsMe = attentionNeedsMe,
                        attentionTopTitle = topAttention?.title,
                        onOpenAttention = { showAttentionCenter = true },
                        onOpenThread = onOpenThread,
                        onOpenTaskCenter = onOpenTaskCenter,
                        onNewThread = { showNewThread = true },
                        currentAgentLabel = selectedAgentLabel(AgentConnection.selectedAgent),
                        onOpenBrowser = onOpenBrowser,
                        visibleAgents = visibleAgents,
                        runningAgentIds = runningAgentIds,
                        onAgentTap = { a ->
                            if (a.available) {
                                // 选中 agent = 切换后端,随后去会话页(既有行为:会话按 agent 分流);
                                // 终端只在用户主动点【终端】时打开。
                                AgentConnection.switchAgent(a.id)
                                Toast.makeText(ctx, "已切换到「${a.name}」", Toast.LENGTH_SHORT).show()
                                onGoTab(1)
                            } else {
                                // 灰显=引导:弹窗讲清"为什么不可用、怎么装"
                                guideAgent = a
                            }
                        },
                        onManageAgents = { showManage = true },
                        collabIds = collabIds,
                        multiAgent = multiAgent,
                        onOpenCollab = onOpenCollab,
                        onOpenCollabRoles = onOpenCollabRoles,
                        onAddAgentTap = {
                            Toast.makeText(ctx, "更多 agent 接入后可在此添加", Toast.LENGTH_SHORT).show()
                        },
                        recent = recent,
                        disabledAgents = disabledAgents,
                        onOpenSessions = { onGoTab(1) },
                    )
                } else {
                    discoveryItems(
                        link = link,
                        hosts = discoveredHosts,
                        onRefresh = { reloadKey++ },
                        onPickHost = { pairHost = it },
                        onManualAdd = onOpenSettings,
                        recent = recent,
                        disabledAgents = disabledAgents,
                        onOpenThread = onOpenThread,
                        onOpenSessions = { onGoTab(1) },
                    )
                }
            }
        }

        // 运行中会话 mini 条(方向 C N1):系统导航栏上方的常驻悬浮位;出现/消失
        // 弹簧由 PaSessionBar 自管。点按=跳转会话;停止位接 TurnSession.cancelTurn
        // 现有停止语义(只对手机端在跑的会话亮出,电脑端会话不摆假停止)。
        PaSessionBar(
            sessions = barSessions,
            onOpenSession = onOpenThread,
            onStopSession = { TurnSession.cancelTurn(it) },
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .onSizeChanged { sessionBarHeightPx = it.height }
                .padding(horizontal = 16.dp)
                .navigationBarsPadding()
                .padding(bottom = 12.dp),
        )
    }

    if (showManage) {
        ManageAgentsSheet(agents = agents, onDismiss = { showManage = false })
    }

    // 灰显磁贴 → 可用性引导弹窗(为什么不可用、怎么装)
    guideAgent?.let { agent ->
        AgentGuideSheet(
            agent = agent,
            // T-DSH 问题 A:与网格同源(共享快照),弹窗口径不再单独依赖本页 host 快照
            capabilities = sharedCaps,
            onDismiss = { guideAgent = null },
        )
    }

    // 新建任务(与其他入口同款:文件式选工作区)
    if (showNewThread) {
        NewThreadSheet(
            onDismiss = { showNewThread = false },
            onCreate = { ws ->
                showNewThread = false
                // 停用偏好同步排除:已停用 agent 不再创建新会话(给出恢复路径,
                // 不静默改路由到别的 agent);恢复后照旧放行。
                if (newThreadAgentAllowed(AgentConnection.selectedAgent, disabledAgents)) {
                    scope.launch {
                        try {
                            val t = createThreadRpc(ws, applyAgentSelection = true)
                            PaHaptics.confirm(view)
                            onOpenThread(t)
                        } catch (e: Exception) {
                            Toast.makeText(ctx, "创建失败:${e.message}", Toast.LENGTH_SHORT).show()
                        }
                    }
                } else {
                    Toast.makeText(
                        ctx,
                        "「${selectedAgentLabel(AgentConnection.selectedAgent)}」已停用,请在主页 agent 卡片右上「管理」恢复后再新建",
                        Toast.LENGTH_SHORT,
                    ).show()
                }
            },
        )
    }

    // 01 发现态 → 选主机,输配对码(独立浮层;关掉 = 暂不配对,不是失败)
    pairHost?.let { picked ->
        PairCodeSheet(
            host = picked,
            onDismiss = { pairHost = null },
            onPaired = {
                pairHost = null
                Toast.makeText(ctx, "配对成功,正在连接…", Toast.LENGTH_SHORT).show()
            },
        )
    }

    // 左上角图标 → 侧边栏(主导航;导航先关抽屉再换页,AppDrawer 内部保证)
    if (drawerCtrl.visible) {
        AppDrawer(
            currentTab = currentTab,
            hostTitle = host?.displayName ?: "未连接电脑",
            progress = drawerCtrl.progress,
            onClose = { drawerCtrl.close() },
            onGoTab = { onGoTab(it) },
            onOpenTerminal = { onOpenTerminal() },
            onOpenSettings = { onOpenSettings() },
            onOpenTaskCenter = onOpenTaskCenter,
            onOpenOnboarding = onOpenOnboarding,
            onOpenBrowser = { onOpenBrowser() },
            onOpenCollab = if (multiAgent) { { onOpenCollab() } } else null,
            // 消息中心入口(抽屉恒定修复 2026-10-09):Home 恒接线 —— 中心自会如实
            // 呈现"这一档暂时没有内容",入口不再随注意项有无闪烁式增减;条目集合
            // 恒定的判定在 AppDrawer.buildDrawerNavEntries(单测钉死)。
            onOpenAttention = { showAttentionCenter = true },
            attentionBadge = attentionNeedsMe,
        )
    }

    // 消息中心浮层(Task 5):四档筛选 + 条目;离线时用缓存时刻如实标注(非实时)。
    if (showAttentionCenter) {
        AttentionCenterSheet(
            items = attentionItems,
            offline = connState != "ok",
            cachedAtUnixMs = RuntimeControlState.unknownLoadedAtMs,
            onDismiss = { showAttentionCenter = false },
            onOpenThread = onOpenThread,
            onOpenSettings = onOpenSettings,
            onOpenCollab = if (multiAgent) onOpenCollab else null,
        )
    }
}

// ---------------------------------------------------------------- 已连接态(02)

/**
 * 已连接首页的 LazyColumn 内容。区块顺序 = 方向 C N5 大卡台阶(2026-10-09 重排,
 * 数据流不变):正在进行(大卡)→ 需要你处理(中卡)→ 消息中心摘要 → 新建 →
 * 可用 Agent → 协同(能力驱动)→ 最近会话。
 */
private fun LazyListScope.connectedHomeItems(
    loadError: String?,
    onRetry: () -> Unit,
    firstLoadDone: Boolean,
    attention: HomeAttentionGroups,
    /** 消息中心摘要(Task 5):有无注意项 / 需要我处理的条数 / 最高优先级的标题。 */
    attentionExists: Boolean,
    attentionNeedsMe: Int,
    attentionTopTitle: String?,
    onOpenAttention: () -> Unit,
    onOpenThread: (String) -> Unit,
    onOpenTaskCenter: () -> Unit,
    onNewThread: () -> Unit,
    currentAgentLabel: String,
    onOpenBrowser: () -> Unit,
    visibleAgents: List<AgentInfo>,
    runningAgentIds: Set<String>,
    onAgentTap: (AgentInfo) -> Unit,
    onManageAgents: () -> Unit,
    collabIds: List<String>,
    multiAgent: Boolean,
    onOpenCollab: () -> Unit,
    onOpenCollabRoles: () -> Unit,
    onAddAgentTap: () -> Unit,
    recent: List<PThread>,
    disabledAgents: Set<String>,
    onOpenSessions: () -> Unit,
) {
    // ---- 断连全局横幅(P1-2,基座组件):connState==down 时组件内部自渲染 ----
    item(key = "banner") { ConnectionBanner(onRetry = onRetry) }

    loadError?.let { message ->
        item(key = "loadError") {
            // 【错误态体系】headline 走 PaStateViews 映射表(未配对→「还没和电脑配对」/
            // 连不上→「连不上电脑」/其余→「电脑端说：…」),原始错误不再作标题裸露 ——
            // 整条原文保住:点按/长按横幅展开「详情」;重试动作原样(与设置页横幅同一套)。
            PaErrorBar(
                paErrorViewOf(message),
                Modifier.padding(top = 12.dp),
                onRetry = onRetry,
            )
        }
    }

    // ---- 大卡台阶第一级:正在进行(方向 C N5)——
    // 运行中会话升为 Graphite 暗面大卡(占双行高,运行态能量细线 + 实时状态语),
    // 与底部 mini 条同母题(运行态能量色=品牌记忆点);无运行中 → 整区不渲染。
    if (attention.running.isNotEmpty()) {
        item(key = "runningHeader") {
            SectionHeader("正在进行", "查看全部 →", onAction = onOpenTaskCenter)
        }
        items(attention.running.take(4), key = { "running-${it.id}" }) { t ->
            RunningHeroCard(thread = t, onClick = { onOpenThread(t.id) })
        }
    }

    // ---- 大卡台阶第二级:需要你处理(待审批中卡)——
    // 待审批从石墨 .alert 降为中卡(白面 + Attention 琥珀系边),视觉权重让位给
    // 运行中大卡;无待审批整区不渲染,不摆空壳。点卡 = 既有 onOpenThread 进会话,
    // 会话内审批弹窗接住,不新建路由。
    if (attention.pendingApproval.isNotEmpty()) {
        item(key = "pendingHeader") {
            SectionHeader("需要你处理", "${attention.pendingApproval.size} 项等待你处理")
        }
        items(attention.pendingApproval.take(4), key = { "pending-${it.id}" }) { t ->
            PendingApprovalCard(thread = t, onClick = { onOpenThread(t.id) })
        }
    }

    // ---- 消息中心摘要(Task 5):只显示数量与最高优先级摘要,完整清单在中心浮层 ----
    // 【为什么不复制详情】首页已有的待审批/运行中区各自承载详情;这里是"全屏注意项"
    // 的总入口(含电脑端新消息与启动恢复),点击进中心浮层,不在首页摊开。
    if (attentionExists) {
        item(key = "attentionCenter") {
            Row(
                Modifier
                    .fillMaxWidth()
                    .heightIn(min = 56.dp)
                    .pressable(haptic = true) { onOpenAttention() },
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(
                    Modifier
                        .size(36.dp)
                        .background(PaEditorial.Raised, RoundedCornerShape(PaEditorial.Radius.Chip)),
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(
                        painterResource(R.drawable.ic_lucide_info), null,
                        tint = MaterialTheme.editorial.Primary, modifier = Modifier.size(18.dp),
                    )
                }
                Spacer(Modifier.width(12.dp))
                Column(Modifier.weight(1f)) {
                    Text(
                        if (attentionNeedsMe > 0) "$attentionNeedsMe 项等待你处理" else "消息中心",
                        fontSize = PaEditorial.Type.BodySmall.fontSize, fontWeight = FontWeight.SemiBold, color = PaEditorial.Ink,
                    )
                    Text(
                        attentionTopTitle ?: "当前没有等待处理的事项",
                        fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp,
                        color = MaterialTheme.editorial.Muted,
                        maxLines = 1, overflow = TextOverflow.Ellipsis,
                    )
                }
                Icon(
                    painterResource(R.drawable.ic_lucide_chevron_right), null,
                    tint = PaEditorial.Subtle, modifier = Modifier.size(17.dp),
                )
            }
        }
    }

    // ---- 开始新任务:主色只用于此处的当前操作(premium.css .new-task) ----
    item(key = "newTaskHeader") {
        SectionHeader("开始新任务", "当前 $currentAgentLabel")
    }
    item(key = "newTask") {
        Row(
            Modifier
                .fillMaxWidth()
                .pressable(haptic = true) { onNewThread() }
                .heightIn(min = 74.dp)
                .shadow(
                    8.dp, RoundedCornerShape(PaEditorial.Radius.Card),
                    spotColor = PaEditorial.Shadow.Card.color,
                    ambientColor = PaEditorial.Shadow.Card.color,
                )
                .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
                .padding(horizontal = 16.dp, vertical = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                Modifier
                    .size(42.dp)
                    .background(PaEditorial.PrimaryContainer, RoundedCornerShape(PaEditorial.Radius.IconButton)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    painterResource(R.drawable.ic_lucide_sparkles), null,
                    tint = MaterialTheme.editorial.Primary, modifier = Modifier.size(20.dp),
                )
            }
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    "告诉 Agent 你想做什么",
                    fontSize = PaEditorial.Type.Body.fontSize, lineHeight = 20.sp,
                    fontWeight = FontWeight.SemiBold, color = PaEditorial.Ink,
                )
                Spacer(Modifier.height(3.dp))
                Text(
                    "选择 Agent、工作区,再开始对话",
                    fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp,
                    color = MaterialTheme.editorial.Muted,
                )
            }
            Spacer(Modifier.width(8.dp))
            Icon(
                painterResource(R.drawable.ic_lucide_chevron_right), null,
                tint = MaterialTheme.editorial.Primary, modifier = Modifier.size(18.dp),
            )
        }
    }

    // ---- 电脑预览(既有能力保留):dev server 一键看,安静行内入口 ----
    item(key = "pcPreview") {
        Row(
            Modifier
                .fillMaxWidth()
                .heightIn(min = 56.dp)
                .pressable(haptic = true) { onOpenBrowser() },
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                Modifier
                    .size(36.dp)
                    .background(PaEditorial.Raised, RoundedCornerShape(PaEditorial.Radius.Chip)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    painterResource(R.drawable.ic_lucide_monitor), null,
                    tint = MaterialTheme.editorial.Primary, modifier = Modifier.size(18.dp),
                )
            }
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text("电脑预览", fontSize = PaEditorial.Type.BodySmall.fontSize, fontWeight = FontWeight.SemiBold, color = PaEditorial.Ink)
                Text("打开电脑上正在运行的 dev server", fontSize = PaEditorial.Type.Caption.fontSize, color = MaterialTheme.editorial.Muted)
            }
            Icon(
                painterResource(R.drawable.ic_lucide_chevron_right), null,
                tint = PaEditorial.Subtle, modifier = Modifier.size(17.dp),
            )
        }
        // 区块之间靠留白分层(下一个区块头自带 16dp 上边距),不再压尾部分割线
    }

    // ---- 可用 Agent:磁贴直接铺背景(README:普通内容不套多层卡) ----
    item(key = "agentsHeader") {
        SectionHeader("可用 Agent", "管理 →", onAction = onManageAgents)
    }
    item(key = "agentsGrid") {
        Column(Modifier.fillMaxWidth().padding(bottom = 4.dp)) {
            BoxWithConstraints(Modifier.fillMaxWidth()) {
                val columns = homeAgentGridColumns(maxWidth.value, LocalDensity.current.fontScale)
                Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
                    visibleAgents.chunked(columns).forEach { rowAgents ->
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            rowAgents.forEach { a ->
                                AgentTile(
                                    a, runningBadge = a.id in runningAgentIds,
                                    modifier = Modifier.weight(1f), onClick = { onAgentTap(a) },
                                )
                            }
                            repeat(columns - rowAgents.size) { Spacer(Modifier.weight(1f)) }
                        }
                    }
                }
            }
            // 空态注记:一个可用 agent 都没有时,给真实原因 + 去向(≥1 个可用不摆字)
            homeAgentGridNote(visibleAgents.count { it.available })?.let { note ->
                Column(Modifier.fillMaxWidth().padding(top = 8.dp)) {
                    Text(
                        note.title,
                        fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
                        fontWeight = FontWeight.SemiBold, color = MaterialTheme.editorial.Muted,
                    )
                    Text(
                        note.hint,
                        Modifier.padding(top = 4.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = PaEditorial.Subtle,
                    )
                    Box(
                        Modifier
                            .padding(top = 4.dp)
                            .heightIn(min = 44.dp)
                            .pressable(haptic = true) { onManageAgents() },
                        contentAlignment = Alignment.CenterStart,
                    ) {
                        Text(
                            note.actionLabel,
                            fontSize = PaEditorial.Type.BodySmall.fontSize, fontWeight = FontWeight.SemiBold,
                            color = MaterialTheme.editorial.Primary,
                        )
                    }
                }
            }
        }
    }

    // ---- 多agent协同:真实状态(daemon collab:<id> 接线事实,非可拨开关) ----
    item(key = "collabSection") {
        Column(Modifier.fillMaxWidth().padding(top = 12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "多 Agent 协同",
                    fontSize = PaEditorial.Type.Heading.fontSize, lineHeight = 22.sp,
                    fontWeight = FontWeight.Bold, color = PaEditorial.Ink,
                )
                Spacer(Modifier.width(8.dp))
                Box(
                    Modifier
                        .size(8.dp)
                        .background(
                            if (collabIds.isEmpty()) PaEditorial.Subtle else PaEditorial.Running,
                            CircleShape,
                        ),
                )
            }
            Text(
                collabSubtitle(collabIds),
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
                color = if (collabIds.isEmpty()) MaterialTheme.editorial.Muted else MaterialTheme.editorial.Primary,
            )
            if (multiAgent) {
                Spacer(Modifier.height(12.dp))
                // 开始协同(派活)与协同角色(岗位配置)分两行,各自成行不混语义
                QuietRow(
                    iconRes = R.drawable.ic_lucide_share_2,
                    title = "开始协同",
                    desc = "一条指令派给多个 agent 并行",
                    onClick = onOpenCollab,
                )
                Hairline()
                val roleCount = if (CollabRolesState.loadState == CollabRolesLoadState.READY) CollabRolesState.roles.size else null
                QuietRow(
                    iconRes = R.drawable.ic_lucide_list_checks,
                    title = "协同角色",
                    desc = if (roleCount == null) "点按管理岗位配置" else "已配置 $roleCount 个岗位",
                    onClick = onOpenCollabRoles,
                )
            }
            Spacer(Modifier.height(8.dp))
            // 协同行:与主网格同一磁贴语言(透明底);多 agent 时给「添加」占位
            Row(
                Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                val shown = if (multiAgent) visibleAgents else visibleAgents.filter { it.id == AgentConnection.selectedAgent }
                shown.forEach { a ->
                    AgentTile(a, runningBadge = false, modifier = Modifier.width(72.dp), onClick = { onAgentTap(a) })
                }
                if (multiAgent) {
                    // 添加位:与磁贴同语言的裸 + 号(占位直说,不假装能加)
                    Column(
                        Modifier
                            .width(72.dp)
                            .pressable(haptic = true) { onAddAgentTap() },
                        horizontalAlignment = Alignment.CenterHorizontally,
                    ) {
                        Box(Modifier.size(56.dp), contentAlignment = Alignment.Center) {
                            Icon(
                                painterResource(R.drawable.ic_lucide_plus), null,
                                tint = PaEditorial.Subtle, modifier = Modifier.size(26.dp),
                            )
                        }
                        Text("添加", Modifier.padding(top = 6.dp), fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Subtle)
                    }
                }
            }
        }
    }

    // ---- 最近会话:留白 + 细分隔线(README:不套多层卡片) ----
    item(key = "recentHeader") {
        SectionHeader("最近会话", "全部会话 →", onAction = onOpenSessions)
    }
    recentItems(
        recent,
        disabledAgents,
        onOpenThread,
        loading = homeShowsRecentSkeleton(connected = true, firstLoadDone, loadError),
        emptyCopy = homeRecentEmptyCopy(recent.isNotEmpty(), currentAgentLabel),
        onEmptyAction = onNewThread,
    )
}

/**
 * 最近会话行组(已连接态与发现态复用)。
 *
 * 【三通道,互斥】
 *  - [loading]=首次加载未回 → 骨架接管(形状=最近会话行轮廓),不摆空态不转圈;
 *  - 过滤后为空 → [RecentEmptyState]:真实状态说明 + 去向按钮([emptyCopy]/
 *    [onEmptyAction] 齐备时);发现态没有安全去向(离线建会话必失败)只给说明;
 *  - 有数据 = 行 + 细分隔线。离线也能回看本地快照 —— 既有能力零回归。
 */
private fun LazyListScope.recentItems(
    recent: List<PThread>,
    disabledAgents: Set<String>,
    onOpenThread: (String) -> Unit,
    loading: Boolean = false,
    emptyCopy: HomeEmptyStateCopy? = null,
    onEmptyAction: (() -> Unit)? = null,
) {
    if (loading) {
        item(key = "recentSkeleton") {
            PaSkeletonList(kind = SkeletonKind.HOME_RECENT_ROW, count = 3, perRowHeight = 56.dp)
        }
        return
    }
    // 最近会话同步排除已停用 agent 的会话(归属缺省按 codex,与 threadAgentIdOf 同口径)
    val shown = recent.filter {
        threadAgentIdOf(it.agentId) !in disabledAgents && it.agentId.ifBlank { "codex" } == AgentConnection.selectedAgent
    }.take(6)
    if (shown.isEmpty()) {
        item(key = "recentEmpty") { RecentEmptyState(emptyCopy, onEmptyAction) }
    } else {
        items(shown, key = { "recent-${it.id}" }) { t ->
            RecentSessionRow(t, onClick = { onOpenThread(t.id) })
            Hairline()
        }
    }
}

/**
 * 最近会话空态:图标 + 真实状态说明 + 明确去向按钮(空态不空白,NN/g 空态三件套)。
 * [copy]/[onAction] 缺位时退化为一句中性说明,绝不出现与当前状态相悖的文案。
 */
@Composable
private fun RecentEmptyState(copy: HomeEmptyStateCopy?, onAction: (() -> Unit)?) {
    Column(
        Modifier.fillMaxWidth().padding(vertical = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Icon(
            painterResource(R.drawable.ic_lucide_list_checks), null,
            tint = PaEditorial.Subtle, modifier = Modifier.size(36.dp),
        )
        if (copy == null) {
            Text(
                "没有可显示的会话",
                Modifier.padding(top = 12.dp),
                fontSize = PaEditorial.Type.BodySmall.fontSize, color = MaterialTheme.editorial.Muted,
            )
        } else {
            Text(
                copy.title,
                Modifier.padding(top = 12.dp),
                fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 20.sp,
                fontWeight = FontWeight.SemiBold, color = PaEditorial.Muted,
            )
            Text(
                copy.hint,
                Modifier.padding(top = 4.dp),
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
                color = MaterialTheme.editorial.Muted,
                textAlign = TextAlign.Center,
            )
            if (onAction != null) {
                // 去向按钮:主色只给"当前操作";触区压满 44dp(README §视觉系统)
                Box(
                    Modifier
                        .padding(top = 16.dp)
                        .heightIn(min = 44.dp)
                        .pressable(haptic = true) { onAction() }
                        .background(PaEditorial.PrimaryContainer, RoundedCornerShape(PaEditorial.Radius.Pill))
                        .padding(horizontal = 20.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        copy.actionLabel,
                        fontSize = PaEditorial.Type.BodySmall.fontSize,
                        fontWeight = FontWeight.SemiBold,
                        color = MaterialTheme.editorial.Primary,
                    )
                }
            }
        }
    }
}

// ---------------------------------------------------------------- 发现主机态(01)

/**
 * 手动填地址入口的按钮规格(纯函数,单测覆盖 FirstTaskFunnelFixTest)。
 *
 * 【为什么是主按钮】首任务漏斗第一步就是"连上电脑":自动发现搜不到时
 * (同网段搜不到、跨网络),这条手动入口是唯一出路;旧实现是次级文字链,
 * 在首屏漏斗里看不见、点不中 —— 必须与主机卡同级(大触区 + 主色)才接得住。
 * bottomNote(服务未启动/跨网络排查提示)保留:按钮是动作,注记是排查说明,
 * 两者语义不重叠。
 */
internal data class ManualAddSpec(
    val label: String,
    /** 恒 true:主按钮规格(大触区 + 主色),与主机卡同级。 */
    val primary: Boolean,
    /** 最小触高 dp(≥48,贴近主机卡 75dp 一档)。 */
    val minHeightDp: Int,
)

internal fun manualAddSpec(): ManualAddSpec =
    ManualAddSpec(label = "+ 手动输入地址或配对码", primary = true, minHeightDp = 56)

/** 发现主机态内容(screens/01-home-discovery.png)。 */
private fun LazyListScope.discoveryItems(
    link: PaHostLink,
    hosts: List<DiscoveredHost>,
    onRefresh: () -> Unit,
    onPickHost: (DiscoveredHost) -> Unit,
    onManualAdd: () -> Unit,
    recent: List<PThread>,
    disabledAgents: Set<String>,
    onOpenThread: (String) -> Unit,
    onOpenSessions: () -> Unit,
) {
    item(key = "eyebrow") {
        Text(
            "POCKET AGENT / CONNECT",
            Modifier.padding(top = 16.dp),
            fontSize = PaEditorial.Type.Caption.fontSize, fontWeight = FontWeight.Bold, letterSpacing = 2.sp,
            color = PaEditorial.Subtle,
        )
    }
    item(key = "hero") {
        Column {
            Spacer(Modifier.height(8.dp))
            // 首页主标题 34/39(Display;README §视觉系统)
            Text(
                "把电脑上的 Agent\n带到手边。",
                style = PaEditorial.Type.Display,
                color = PaEditorial.Ink,
            )
            Spacer(Modifier.height(8.dp))
            Text(
                "找到你的电脑,配对后即可安全继续工作。",
                fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 22.sp, color = MaterialTheme.editorial.Muted,
            )
        }
    }
    item(key = "art") {
        // 发现圆环(premium.css .discovery-art .orb):三层同心圆 + 中性 monitor 细线图标
        // 【D4-3 最内圈呼吸】静态三层圆里最内圈(主色渐变盘)加 pulse 呼吸
        // (PaMotion.pulse 900ms 往复)—— "正在搜索附近主机"的存在感提示;
        // 只碰 graphicsLayer(缩放,绘制层),布局与外两圈零影响;
        // 减弱动效时静态(rememberReduceMotion,循环动效的退场纪律)。
        val reduceMotion = rememberReduceMotion()
        val breathe by if (reduceMotion) {
            remember { mutableStateOf(1f) }
        } else {
            rememberInfiniteTransition(label = "discoveryBreathe")
                .animateFloat(
                    initialValue = 0.965f,
                    targetValue = 1.035f,
                    animationSpec = PaMotion.pulse(),
                    label = "discoveryBreathe",
                )
        }
        Box(Modifier.fillMaxWidth().padding(vertical = 12.dp), contentAlignment = Alignment.Center) {
            Box(
                Modifier
                    .size(240.dp)
                    .background(PaEditorial.Raised, CircleShape),
                contentAlignment = Alignment.Center,
            ) {
                Box(
                    Modifier
                        .size(192.dp)
                        .background(PaEditorial.Hover, CircleShape),
                    contentAlignment = Alignment.Center,
                ) {
                    Box(
                        Modifier
                            .size(154.dp)
                            .graphicsLayer {
                                scaleX = breathe
                                scaleY = breathe
                            }
                            .background(
                                Brush.radialGradient(
                                    colors = listOf(PaEditorial.Surface, PaEditorial.PrimaryContainer, PaEditorial.Raised),
                                ),
                                CircleShape,
                            ),
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(
                            painterResource(R.drawable.ic_lucide_monitor), null,
                            tint = PaEditorial.Primary, modifier = Modifier.size(56.dp),
                        )
                    }
                }
            }
            // 状态词条:白底小签贴圆环右下(状态词与颜色双通道;连接中≠已连接)
            Text(
                when (link) {
                    PaHostLink.CONNECTING -> "正在搜索附近主机"
                    else -> "未连接电脑"
                },
                Modifier
                    .align(Alignment.BottomEnd)
                    .background(MaterialTheme.editorial.Surface, RoundedCornerShape(PaEditorial.Radius.Chip))
                    .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.Chip))
                    .padding(horizontal = 8.dp, vertical = 6.dp),
                fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.AccentText,
            )
        }
    }
    item(key = "discoverHeader") {
        SectionHeader("发现的电脑", "重新搜索", onAction = onRefresh)
    }
    if (hosts.isEmpty()) {
        if (link == PaHostLink.CONNECTING) {
            // 搜索中是加载态不是空态:骨架接管(形状=主机卡轮廓),不摆"正在…"空文
            item(key = "discoverSkeleton") {
                PaSkeletonList(kind = SkeletonKind.HOST_CHOICE, count = 2, perRowHeight = 75.dp)
            }
        } else {
            item(key = "discoverEmpty") {
                val copy = homeDiscoveryEmptyCopy(searching = false)!!
                // 空态三件套:真实说明 + 怎么办 + 去向按钮(手动添加,不让人干等)
                Column(
                    Modifier.fillMaxWidth().padding(vertical = 16.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                ) {
                    Icon(
                        painterResource(R.drawable.ic_lucide_search), null,
                        tint = PaEditorial.Subtle, modifier = Modifier.size(28.dp),
                    )
                    Text(
                        copy.title,
                        Modifier.padding(top = 12.dp),
                        fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 20.sp,
                        fontWeight = FontWeight.SemiBold, color = PaEditorial.Muted,
                    )
                    Text(
                        copy.hint,
                        Modifier.padding(top = 4.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
                        color = MaterialTheme.editorial.Muted,
                        textAlign = TextAlign.Center,
                    )
                    Box(
                        Modifier
                            .padding(top = 12.dp)
                            .heightIn(min = 44.dp)
                            .pressable(haptic = true) { onManualAdd() }
                            .background(PaEditorial.PrimaryContainer, RoundedCornerShape(PaEditorial.Radius.Pill))
                            .padding(horizontal = 20.dp),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            copy.actionLabel,
                            fontSize = PaEditorial.Type.BodySmall.fontSize,
                            fontWeight = FontWeight.SemiBold,
                            color = MaterialTheme.editorial.Primary,
                        )
                    }
                }
            }
        }
    } else {
        items(hosts, key = { "host-${it.instanceName}" }) { h ->
            DiscoveredHostCard(host = h, onClick = { onPickHost(h) })
            Spacer(Modifier.height(8.dp))
        }
    }
    item(key = "manualAdd") {
        // 为什么升为主操作:规格见 manualAddSpec —— 自动发现兜不住时它是唯一出路,
        // 次级文字链在首屏漏斗里接不住人;bottomNote(下一项)原样保留。
        val spec = manualAddSpec()
        Row(
            Modifier
                .fillMaxWidth()
                .padding(top = 4.dp)
                .heightIn(min = spec.minHeightDp.dp)
                .pressable(haptic = true) { onManualAdd() }
                .background(PaEditorial.Primary, RoundedCornerShape(PaEditorial.Radius.Card))
                .padding(horizontal = 16.dp, vertical = 12.dp)
                .semantics { contentDescription = spec.label },
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                painterResource(R.drawable.ic_lucide_monitor), null,
                tint = PaEditorial.OnGraphite, modifier = Modifier.size(20.dp),
            )
            Spacer(Modifier.width(12.dp))
            Text(
                spec.label,
                Modifier.weight(1f),
                fontSize = PaEditorial.Type.Body.fontSize, fontWeight = FontWeight.SemiBold,
                color = PaEditorial.OnGraphite,
            )
            Icon(
                painterResource(R.drawable.ic_lucide_chevron_right), null,
                tint = PaEditorial.OnGraphite, modifier = Modifier.size(18.dp),
            )
        }
    }
    item(key = "bottomNote") {
        Text(
            "若电脑未出现,请确认 Pocket Agent 服务已启动;跨网络可通过已有的远程连接地址添加。",
            Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 16.dp),
            fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 18.sp, color = PaEditorial.Subtle,
        )
    }
    // 离线也保留最近会话(本地快照可回看 —— 既有能力零回归)
    if (recent.isNotEmpty()) {
        item(key = "offlineRecentHeader") {
            SectionHeader("最近会话", "全部会话 →", onAction = onOpenSessions)
        }
        recentItems(recent, disabledAgents, onOpenThread)
    }
}

// ---------------------------------------------------------------- 行/卡组件

/** 11sp 以下小字的加深珊瑚色(= EditorialTone.DENIED.text,对浅底 ≥4.5:1)。 */
private val EditorialDeniedText: Color get() = PaEditorial.DeniedText

/**
 * 区块头:模块标题 16/22 + 行尾辅助小字(可选点击动作,premium.css .section-title)。
 *
 * 【间距/触区】上 16 + 行高 + 下 8,总节奏与旧 23/11 相当(8dp 网格);
 * 行尾动作的内边距把触区撑到 ~33dp 高(裸文字只有 ~17dp),保持视觉紧凑的同时
 * 尽量靠近 README 的 44dp 下限 —— 再大就要抬高整个区块头,交由后续全局定夺。
 */
@Composable
private fun SectionHeader(title: String, action: String, onAction: (() -> Unit)? = null) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(top = 16.dp, bottom = 8.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            title,
            fontSize = PaEditorial.Type.Heading.fontSize, lineHeight = 22.sp,
            fontWeight = FontWeight.Bold, color = PaEditorial.Ink,
        )
        if (onAction != null) {
            Text(
                action,
                Modifier
                    .pressable(haptic = true, onClick = onAction)
                    .padding(horizontal = 8.dp, vertical = 10.dp),
                fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Subtle,
            )
        } else {
            Text(action, fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Subtle)
        }
    }
}

/** 细分隔线(1dp Hairline;README:留白 + 细线分层)。 */
@Composable
private fun Hairline() {
    Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))
}

/**
 * 待审批中卡(方向 C · N5 大卡台阶的第二级):白面 + Attention 琥珀系边,
 * 视觉权重让位给运行中大卡(第一级),但"等待你处理"的语义双通道不变
 * (琥珀盾形通道 + 状态词)。点击 → 对应会话(会话内审批弹窗接住,
 * 背景保留原页,关闭后仍能在本区找到任务)。
 * 【语义边界】系边取 ATTENTION(待处理)而非 DENIED —— DENIED 珊瑚只给真失败,
 * 待审批不是失败(设计依据 §2.3 Things 3 calm 纪律与 C6 取舍)。
 */
@Composable
private fun PendingApprovalCard(thread: PThread, onClick: () -> Unit) {
    val title = thread.title.ifEmpty { "(未命名)" }
    val agentName = selectedAgentLabel(threadAgentIdOf(thread.agentId))
    Row(
        Modifier
            .fillMaxWidth()
            .padding(bottom = 8.dp)
            .pressable(haptic = true) { onClick() }
            .heightIn(min = 64.dp)
            .shadow(
                8.dp, RoundedCornerShape(PaEditorial.Radius.Card),
                spotColor = PaEditorial.Shadow.Card.color,
                ambientColor = PaEditorial.Shadow.Card.color,
            )
            .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
            .border(1.dp, PaEditorial.Attention, RoundedCornerShape(PaEditorial.Radius.Card))
            .padding(horizontal = 14.dp, vertical = 12.dp)
            .semantics {
                contentDescription = "$agentName 请求执行命令,来自「$title」,等待你处理"
            },
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(R.drawable.ic_lucide_shield), null,
            tint = PaEditorial.Attention, modifier = Modifier.size(20.dp),
        )
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(
                "$agentName 请求执行命令",
                fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 19.sp, fontWeight = FontWeight.Bold,
                color = PaEditorial.Ink,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
            Spacer(Modifier.height(4.dp))
            Text(
                "来自「$title」· ${pendingApprovalMeta(thread.pendingApprovalsCount)}",
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
                color = PaEditorial.Muted,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
        }
        Spacer(Modifier.width(8.dp))
        Icon(
            painterResource(R.drawable.ic_lucide_chevron_right), null,
            tint = PaEditorial.Subtle, modifier = Modifier.size(18.dp),
        )
    }
}

/**
 * 运行中会话大卡(方向 C · N5 大卡台阶的第一级 + N4 机魂):
 * Graphite 暗面(C2 铁律:暗面只许 Graphite 槽位)+ 左缘运行态能量细线
 * (Running 呼吸,减弱动效静态,与 mini 条同母题)+ 实时状态语(时间维度),
 * 占双行高(小行 56dp × 2)。状态词唯一来源仍是 PaStatusBadge
 * (EditorialStatus.RUNNING)=「正在执行」。本区块只在主机已连接时渲染 ——
 * 离线绝不显示「正在执行」(构造性保证)。
 */
@Composable
private fun RunningHeroCard(thread: PThread, onClick: () -> Unit) {
    val shape = RoundedCornerShape(PaEditorial.Radius.Card)
    Row(
        Modifier
            .fillMaxWidth()
            .padding(bottom = 10.dp)
            .pressable(haptic = true) { onClick() }
            .heightIn(min = 112.dp)
            .shadow(
                15.dp, shape,
                spotColor = PaEditorial.Shadow.Alert.color,
                ambientColor = PaEditorial.Shadow.Alert.color,
            )
            .background(PaEditorial.Graphite, shape)
            .padding(horizontal = 16.dp, vertical = 16.dp)
            .semantics {
                contentDescription =
                    "「${thread.title.ifEmpty { "(未命名)" }}」正在执行,点按查看"
            },
        verticalAlignment = Alignment.CenterVertically,
    ) {
        PaRunningEnergyLine(height = 44.dp)
        Spacer(Modifier.width(14.dp))
        Image(
            bitmap = agentBitmap(agentBadgeRes(thread.agentId)),
            contentDescription = null,
            modifier = Modifier.size(52.dp),
            contentScale = ContentScale.Crop,
        )
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(
                thread.title.ifEmpty { "(未命名)" },
                fontSize = PaEditorial.Type.Body.fontSize, lineHeight = 20.sp,
                fontWeight = FontWeight.SemiBold, color = PaEditorial.OnGraphite,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
            Spacer(Modifier.height(4.dp))
            Text(
                selectedAgentLabel(threadAgentIdOf(thread.agentId)) + " · " + runningMetaLabel(
                    // 起点缺席(旧 daemon/未知)传 null,回退"最近活动"展示
                    if (thread.hasTurnStartedUnixMs()) thread.turnStartedUnixMs else null,
                    thread.updatedAtUnixMs,
                    System.currentTimeMillis(),
                ),
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp,
                color = PaEditorial.OnGraphiteMuted,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
        }
        Spacer(Modifier.width(8.dp))
        PaStatusBadge(EditorialStatus.RUNNING)
    }
}

/**
 * 最近会话行(留白 + 细分隔线):状态词走 editorialThreadStatusView 四词映射,
 * 未完成时附错误首行摘要(信息等量,换四词口径);徽章 suffix = 相对时间。
 */
@Composable
private fun RecentSessionRow(thread: PThread, onClick: () -> Unit) {
    val statusView = editorialThreadStatusView(
        thread.status,
        thread.pendingApprovalsCount,
        thread.lastTurnError,
    )
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 56.dp)
            .pressable(haptic = true) { onClick() }
            .padding(vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Image(
            bitmap = agentBitmap(agentBadgeRes(thread.agentId)),
            contentDescription = null,
            modifier = Modifier.size(30.dp),
            contentScale = ContentScale.Crop,
        )
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(
                thread.title.ifEmpty { "(未命名)" },
                fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Ink,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
            Spacer(Modifier.height(4.dp))
            Text(
                selectedAgentLabel(threadAgentIdOf(thread.agentId)) + " · " +
                    threadRecentMeta(thread.messageCount, thread.updatedAtUnixMs, System.currentTimeMillis()),
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp, color = MaterialTheme.editorial.Muted,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
            // 未完成摘要:一行缩略("这单为什么停了"不用点进会话就能看到)
            statusView.detail?.let { summary ->
                Text(
                    summary,
                    // 【行高豁免】15.sp 非 Type 令牌:单行摘要(maxLines=1)的紧凑
                    // 行高,比 Micro 档默认 17.sp 收一档,与同列标题行(21)·元信息
                    // 行(16)在 56dp 会话行内的三行堆叠节奏配套。
                    fontSize = PaEditorial.Type.Micro.fontSize, lineHeight = 15.sp,
                    color = EditorialDeniedText, maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
        Spacer(Modifier.width(8.dp))
        PaStatusBadge(
            statusView.status,
            suffix = relativeTimeLabel(thread.updatedAtUnixMs).ifBlank { null },
        )
    }
}

/** 安静入口行(协同/预览等次级入口):36dp 图标格直接铺背景,不套外层卡。 */
@Composable
private fun QuietRow(iconRes: Int, title: String, desc: String, onClick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 56.dp)
            .pressable(haptic = true) { onClick() },
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier
                .size(36.dp)
                .background(PaEditorial.Raised, RoundedCornerShape(PaEditorial.Radius.Chip)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(painterResource(iconRes), null, tint = PaEditorial.Primary, modifier = Modifier.size(18.dp))
        }
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(title, fontSize = PaEditorial.Type.BodySmall.fontSize, fontWeight = FontWeight.SemiBold, color = PaEditorial.Ink)
            Text(
                desc, fontSize = PaEditorial.Type.Caption.fontSize, color = MaterialTheme.editorial.Muted,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
        }
        Icon(
            painterResource(R.drawable.ic_lucide_chevron_right), null,
            tint = PaEditorial.Subtle, modifier = Modifier.size(17.dp),
        )
    }
}

/**
 * 发现的主机卡(premium.css .host-choice):白面、细线、42dp monitor 图标格;
 * 点击 → 配对码浮层(README 主流程:选择主机 → 输入电脑显示的 8 位码)。
 */
@Composable
private fun DiscoveredHostCard(host: DiscoveredHost, onClick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .pressable(haptic = true) { onClick() }
            .heightIn(min = 75.dp)
            .background(MaterialTheme.editorial.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
            .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.Card))
            .padding(16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier
                .size(42.dp)
                .background(PaEditorial.Raised, RoundedCornerShape(PaEditorial.Radius.IconButton)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                painterResource(R.drawable.ic_lucide_monitor), null,
                tint = PaEditorial.Primary, modifier = Modifier.size(22.dp),
            )
        }
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(
                host.displayName,
                fontSize = PaEditorial.Type.Body.fontSize, fontWeight = FontWeight.SemiBold,
                color = PaEditorial.Ink, maxLines = 1,
            )
            Spacer(Modifier.height(5.dp))
            Text(
                "局域网发现 · ${host.address()}",
                fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Subtle,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
        }
        Icon(
            painterResource(R.drawable.ic_lucide_chevron_right), null,
            tint = PaEditorial.Primary, modifier = Modifier.size(18.dp),
        )
    }
}

// ---------------------------------------------------------------- 配对码浮层

/**
 * 配对码浮层(01 发现态的「待配对」):选中的主机 + 8 位码输入。
 *
 * 【与设置页同一条链】setAddress(目标主机)→ authorizeWithPairCodeDetailed(领真
 * token 并持久化)→ setCredentials + connectWithFallback。失败时**保留输入**并把
 * daemon/传输层原文透传(AUDIT §3.3:码错保留输入并给重试;证书变化只显示阻断
 * 信息,核验入口在设置的连接页)。
 */
@Composable
private fun PairCodeSheet(
    host: DiscoveredHost,
    onDismiss: () -> Unit,
    onPaired: () -> Unit,
) {
    val scope = rememberCoroutineScope()
    var code by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    PaSheet(onDismissRequest = { if (!busy) onDismiss() }) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp)) {
            Text("配对「${host.displayName}」", fontSize = PaEditorial.Type.Heading.fontSize, fontWeight = FontWeight.Bold, color = PaEditorial.Ink)
            Spacer(Modifier.height(4.dp))
            Text(
                "输入电脑上显示的 8 位配对码(${host.address()})",
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = MaterialTheme.editorial.Muted,
            )
            Spacer(Modifier.height(14.dp))
            OutlinedTextField(
                value = code,
                onValueChange = { code = it.filter(Char::isDigit).take(8) },
                Modifier.fillMaxWidth(),
                enabled = !busy,
                singleLine = true,
                placeholder = { Text("8 位配对码", fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Subtle) },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
                colors = OutlinedTextFieldDefaults.colors(
                    focusedContainerColor = PaEditorial.Surface,
                    unfocusedContainerColor = PaEditorial.Surface,
                    focusedBorderColor = PaEditorial.Primary,
                    unfocusedBorderColor = PaEditorial.Hairline,
                    focusedTextColor = PaEditorial.Ink,
                    unfocusedTextColor = PaEditorial.Ink,
                    cursorColor = PaEditorial.Primary,
                ),
            )
            error?.let { msg ->
                Spacer(Modifier.height(8.dp))
                // 码错/证书阻断:透传原文,输入保留可改可重试(不做乐观通过)
                Text(msg, fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = EditorialDeniedText)
            }
            Spacer(Modifier.height(16.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                // 暂不配对:次操作(灰面线框按钮),与「配对并连接」两条通道
                Box(
                    Modifier
                        .weight(1f)
                        .heightIn(min = 46.dp)
                        .pressable(haptic = true, enabled = !busy) { onDismiss() }
                        .background(PaEditorial.Raised, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton)),
                    contentAlignment = Alignment.Center,
                ) {
                    Text("取消", fontSize = PaEditorial.Type.Body.fontSize, fontWeight = FontWeight.SemiBold, color = PaEditorial.Ink)
                }
                Box(
                    Modifier
                        .weight(1f)
                        .heightIn(min = 46.dp)
                        .pressable(haptic = true, enabled = !busy && code.length >= 4) {
                            busy = true
                            error = null
                            scope.launch {
                                val addr = host.address()
                                val failure: String? = withContext(Dispatchers.IO) {
                                    try {
                                        AgentConnection.setAddress(addr)
                                        val paired = AgentConnection.authorizeWithPairCodeDetailed(code)
                                        if (paired.ok) {
                                            // 配对成功:真 token 已入敏感库,写凭证并立即连
                                            AgentConnection.setCredentials(addr, AgentConnection.token)
                                            AgentConnection.connectWithFallback()
                                            null
                                        } else {
                                            paired.error ?: "配对失败,请核对配对码后重试"
                                        }
                                    } catch (e: Exception) {
                                        if (e is kotlin.coroutines.cancellation.CancellationException) throw e
                                        e.message ?: "连接失败"
                                    }
                                }
                                busy = false
                                if (failure == null) onPaired() else error = failure
                            }
                        }
                        .background(PaEditorial.Graphite, RoundedCornerShape(PaEditorial.Radius.IconButton)),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        if (busy) "配对中…" else "配对并连接",
                        fontSize = PaEditorial.Type.Body.fontSize, fontWeight = FontWeight.Bold,
                        color = if (busy) PaEditorial.OnGraphiteMuted else PaEditorial.OnGraphite,
                    )
                }
            }
            Spacer(Modifier.height(24.dp))
        }
    }
}

// ---------------------------------------------------------------- agent 磁贴与浮层

/**
 * agent 磁贴:官方品牌图标,三态一眼可辨且**不单靠灰度** —— 状态词落字(副文字):
 *  - 可用:全彩 + 名称墨色 + 连接方式;
 *  - 接入中(展示目录占位):灰度半可见(alpha .45)+「接入中」;
 *  - 电脑端未安装:灰度近隐(alpha .22)+「电脑端未安装」。
 * [runningBadge]:该 agent 名下有运行中会话时,磁贴右上角脉冲绿点(与「正在进行」
 * 区同源)。编辑式令牌:名称墨色/灰、副文字灰,不再用主题蓝做装饰。
 */
@Composable
fun AgentTile(a: AgentInfo, runningBadge: Boolean = false, modifier: Modifier = Modifier, onClick: () -> Unit) {
    val state = homeAgentTileState(a)
    Column(
        modifier.pressable(haptic = true) { onClick() },
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box {
            Image(
                bitmap = agentBitmap(a.iconRes),
                contentDescription = null,
                modifier = Modifier
                    .size(56.dp)
                    .then(
                        when (state) {
                            HomeAgentTileState.AVAILABLE -> Modifier
                            HomeAgentTileState.INTEGRATING -> Modifier.alpha(0.45f)
                            HomeAgentTileState.NOT_INSTALLED -> Modifier.alpha(0.22f)
                        },
                    ),
                contentScale = ContentScale.Crop,
                colorFilter = if (state == HomeAgentTileState.AVAILABLE) null else grayscale(),
            )
            if (runningBadge) {
                PulseDot(PaEditorial.Running, Modifier.align(Alignment.TopEnd))
            }
        }
        Text(
            a.name,
            Modifier.padding(top = 6.dp),
            fontSize = PaEditorial.Type.Caption.fontSize,
            color = if (state == HomeAgentTileState.AVAILABLE) PaEditorial.Ink else PaEditorial.Muted,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            textAlign = TextAlign.Center,
        )
        Text(
            // 灰显态给真实原因:能力驱动型="电脑端未安装";未接入型="接入中"
            homeAgentTileLabel(state, a.method),
            Modifier.padding(top = 2.dp),
            fontSize = PaEditorial.Type.Micro.fontSize,
            color = if (state == HomeAgentTileState.AVAILABLE) PaEditorial.Muted else PaEditorial.Subtle,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            textAlign = TextAlign.Center,
        )
    }
}

/**
 * agent 可用性引导弹窗:灰显磁贴点击后讲清"为什么不可用、怎么装"。
 * 统一玻璃浮层 PaSheet(玻璃底 + 内建滚动与高度上限,防小屏溢出)。
 * 【皮肤说明】浮层是基座玻璃皮肤(随主题换肤),内部已全部走 PaEditorial 语义槽位
 * 与 MaterialTheme.editorial(editorialM3Scheme 三盘同灌,深色/黑客盘同构可用)——
 * 旧 Pa.* 色板引用已清零(PaTheme 的兼容 shim 不再有本页消费点)。
 */
@Composable
internal fun AgentGuideSheet(agent: AgentInfo, capabilities: List<String>, onDismiss: () -> Unit) {
    // 已装版本(daemon 报 "agent:<id>:<version>" 时非 null)→ 状态行区分两态
    val installedVersion = resolveAgentInstalledVersion(agent.id, capabilities)
    val guide = agentGuideCopy(agent.id)
    PaSheet(onDismissRequest = onDismiss) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 20.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Image(
                bitmap = agentBitmap(agent.iconRes),
                contentDescription = agent.name,
                modifier = Modifier.size(44.dp),
                contentScale = ContentScale.Crop,
            )
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text(agent.name, fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink)
                Text(
                    // 状态行:占位展示项固定"接入中";能力驱动型分"已装 vX"/"电脑端未安装"两态
                    when {
                        agent.id in placeholderDisplayAgents -> AgentGuideCopy.SUBTITLE_INTEGRATING
                        installedVersion != null -> "已装 $installedVersion"
                        else -> AgentGuideCopy.SUBTITLE_NOT_INSTALLED
                    },
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = when {
                        agent.id in placeholderDisplayAgents -> PaEditorial.Muted
                        installedVersion != null -> PaEditorial.Running
                        else -> PaEditorial.Denied
                    },
                    modifier = Modifier.padding(top = 2.dp),
                )
            }
            IconActionButton(R.drawable.ic_lucide_x, "关闭", onDismiss, bg = Color.Transparent, size = 38.dp, iconSize = 18.dp)
        }
        Spacer(Modifier.height(12.dp))
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            GuideBlock("怎么判断的", guide.detect)
            GuideBlock("怎么装", guide.install)
        }
        if (agent.id !in placeholderDisplayAgents && installedVersion == null) {
            Text(
                "装好后重连 daemon,磁贴自动点亮",
                Modifier.padding(horizontal = 20.dp, vertical = 6.dp),
                fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Subtle,
            )
        }
        Spacer(Modifier.height(30.dp))
    }
}

/** 引导弹窗里的一块信息卡(标题 + 正文)。 */
@Composable
private fun GuideBlock(title: String, body: String) {
    Column(
        Modifier
            .padding(horizontal = 16.dp)
            .fillMaxWidth()
            .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
            .padding(horizontal = 14.dp, vertical = 12.dp),
    ) {
        Text(title, fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
        Text(body, fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Ink, modifier = Modifier.padding(top = 4.dp))
    }
}

/**
 * 脉冲状态点:注意区语义色的小面积视觉载体(正在执行=脉冲绿点)。
 * 系统减弱动效开启时退化为静态点,与全 app 动效纪律一致(TaskCenter/PaSkeleton 同款)。
 * 【令牌纪律】循环规格走 PaMotion.pulse()(900ms=Loop.PULSE_MS 基线,参数逐位一致),
 * 不再裸写 infiniteRepeatable(tween(...)) —— 令牌层只此一处,改节奏只动 PaMotion。
 */
@Composable
private fun PulseDot(color: Color, modifier: Modifier = Modifier, pulse: Boolean = true) {
    val reduceMotion = rememberReduceMotion()
    if (pulse && !reduceMotion) {
        val transition = rememberInfiniteTransition(label = "pulseDot")
        val alpha by transition.animateFloat(
            initialValue = 1f,
            targetValue = 0.35f,
            animationSpec = PaMotion.pulse(),
            label = "pulseAlpha",
        )
        Box(modifier.size(8.dp).graphicsLayer { this.alpha = alpha }.background(color, CircleShape))
    } else {
        Box(modifier.size(8.dp).background(color, CircleShape))
    }
}

/**
 * 管理 agent 抽屉:真实图标 + 名称 + 连接方式副标题 + 开关。
 * 统一玻璃浮层 PaSheet;列表滚动交给 PaSheet 内建滚动(高度上限防溢出)。
 * 【皮肤说明】同 AgentGuideSheet:浮层内部已全部走 PaEditorial 语义槽位
 * 与 MaterialTheme.editorial(旧 Pa.* 引用已清零)。
 */
@Composable
fun ManageAgentsSheet(agents: List<AgentInfo>, onDismiss: () -> Unit) {
    val ctx = LocalContext.current
    PaSheet(onDismissRequest = onDismiss) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp)) {
            Column(Modifier.weight(1f)) {
                Text("管理 agent", fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink)
                Text(
                    "停用后不再显示该 agent,正在运行的任务不受影响",
                    fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted, modifier = Modifier.padding(top = 2.dp),
                )
            }
            IconActionButton(R.drawable.ic_lucide_x, "关闭", onDismiss, bg = Color.Transparent, size = 38.dp, iconSize = 18.dp)
        }
        Spacer(Modifier.height(12.dp))
        // PaSheet 的内容列已自带滚动与 82% 屏高上限,这里不再嵌套 verticalScroll。
        Column(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            agents.forEach { a ->
                Row(
                    Modifier
                        .fillMaxWidth()
                        .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
                        .padding(horizontal = 14.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Image(
                        bitmap = agentBitmap(a.iconRes),
                        contentDescription = null,
                        modifier = Modifier.size(44.dp),
                        contentScale = ContentScale.Crop,
                    )
                    Spacer(Modifier.width(12.dp))
                    Column(Modifier.weight(1f)) {
                        Text(a.name, fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink)
                        Text(a.method + " · " + a.subtitle, fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
                        // T-MW2:不可用(daemon 未报)与已停用(本地偏好)两态目视可分
                        if (AgentConnection.isAgentDisabled(a.id)) {
                            Text("已停用", fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Subtle)
                        }
                    }
                    Switch(
                        // 开=显示且可用;关=已停用(本地偏好层)。不可用(daemon 未报)agent
                        // 恒为关,不伪造「已停用」。
                        checked = a.available && !AgentConnection.isAgentDisabled(a.id),
                        onCheckedChange = { requestOn ->
                            // T-MW 把开关接成真的:可用 agent 点按=停用/恢复(写偏好,立即生效);
                            // 不可用 agent 点按=状态原地不动 + 真实原因提示(不伪造)。
                            val (nextDisabled, toast) = agentManageToggleResult(
                                requestOn, a.available, AgentConnection.isAgentDisabled(a.id), a.id, a.name,
                            )
                            AgentConnection.setAgentDisabled(a.id, nextDisabled)
                            toast?.let { Toast.makeText(ctx, it, Toast.LENGTH_SHORT).show() }
                        },
                        colors = SwitchDefaults.colors(
                            checkedTrackColor = PaEditorial.Primary,
                            checkedThumbColor = PaEditorial.Ink,
                            uncheckedTrackColor = PaEditorial.Hover,
                            uncheckedThumbColor = PaEditorial.Subtle,
                            uncheckedBorderColor = Color.Transparent,
                        ),
                    )
                }
            }
            // 尾部留白随列滚动:列表占满视口时底部也有呼吸空间
            Spacer(Modifier.height(30.dp))
        }
    }
}

// ---------------------------------------------------------------- 消息中心(Task 5)

/**
 * Attention Center 浮层(统一玻璃浮层 PaSheet):四档筛选 + 条目列表。
 *
 * 【只做呈现与跳转】归档/排序/最高优先级都是 attention 包的纯函数(单测钉住);
 * 点击条目回到正确会话或设置详情,返回后主页列表的滚动位置由 LazyColumn 自行保留。
 * 【OfflineStore 纪律】offline=true 时标注「本地缓存 · 时刻」,绝不把旧数据摆成实时;
 * 在线时不摆任何字(freshnessLabel 返回 null,不占位)。
 */
@Composable
private fun AttentionCenterSheet(
    items: List<AttentionItem>,
    offline: Boolean,
    cachedAtUnixMs: Long,
    onDismiss: () -> Unit,
    onOpenThread: (String) -> Unit,
    onOpenSettings: () -> Unit,
    onOpenCollab: (() -> Unit)?,
) {
    PaSheet(onDismissRequest = onDismiss) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 24.dp)) {
            Text("消息中心", fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink, fontWeight = FontWeight.SemiBold)
            attentionFreshnessLabel(offline, cachedAtUnixMs, System.currentTimeMillis())?.let {
                Text(it, Modifier.padding(top = 3.dp), fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
            }
            Spacer(Modifier.height(10.dp))
            // 四档筛选:需要我处理 / 运行中 / 已完成 / 已忽略(记住上次选择)。
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                AttentionFilter.values().forEach { f ->
                    val selected = AttentionCenterUiState.filter == f
                    Box(
                        Modifier
                            .pressable { AttentionCenterUiState.filter = f }
                            .background(
                                if (selected) PaEditorial.PrimaryContainer else PaEditorial.Raised,
                                RoundedCornerShape(PaEditorial.Radius.Pill),
                            )
                            .padding(horizontal = 12.dp, vertical = 6.dp),
                    ) {
                        Text(
                            f.label,
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            color = if (selected) MaterialTheme.editorial.Primary else MaterialTheme.editorial.Muted,
                            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
                        )
                    }
                }
            }
            Spacer(Modifier.height(6.dp))
            val shown = filterAttention(
                AttentionCenterState(
                    items = items,
                    filter = AttentionCenterUiState.filter,
                    offline = offline,
                    cachedAtUnixMs = cachedAtUnixMs,
                ),
            )
            if (shown.isEmpty()) {
                // 空档如实:这一档现在没有内容(不挪别档数据来填充)。
                Text(
                    "这一档暂时没有内容",
                    Modifier
                        .fillMaxWidth()
                        .padding(vertical = 24.dp),
                    fontSize = PaEditorial.Type.BodySmall.fontSize, color = MaterialTheme.editorial.Muted,
                    textAlign = TextAlign.Center,
                )
            } else {
                shown.forEachIndexed { i, item ->
                    AttentionItemRow(item = item, onClick = {
                        when (item) {
                            is AttentionItem.Approval -> onOpenThread(item.id)
                            is AttentionItem.UnknownEvent -> {
                                if (item.threadId.isNotBlank()) {
                                    onOpenThread(item.threadId)
                                } else {
                                    // 无归属的事件不虚构会话(计划红线:没有会话关联的
                                    // 事件只进全局中心):去设置的运行时详情,并直接把
                                    // 「电脑端新消息」弹窗打开(dialysisSheetOpen 共享状态)。
                                    RuntimeControlState.dialysisSheetOpen = true
                                    onOpenSettings()
                                }
                            }
                            is AttentionItem.Recovery -> onOpenSettings()
                            is AttentionItem.Collab -> onOpenCollab?.invoke()
                        }
                    })
                    if (i < shown.lastIndex) {
                        Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))
                    }
                }
            }
            Spacer(Modifier.height(12.dp))
        }
    }
}

/** 中心里的一行:标题 + 说明(未知事件附相对时间)+ 行尾动作词/状态词;点击整行跳转。 */
@Composable
private fun AttentionItemRow(item: AttentionItem, onClick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 56.dp)
            .pressable(haptic = true) { onClick() }
            .padding(vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(
                item.title,
                fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 19.sp, fontWeight = FontWeight.SemiBold,
                color = PaEditorial.Ink, maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
            val time = (item as? AttentionItem.UnknownEvent)
                ?.receivedAtUnixMs?.takeIf { it > 0L }
                ?.let { relativeTimeLabel(it) }
            val meta = listOf(item.detail.takeIf { it.isNotBlank() }, time)
                .filterNotNull()
                .joinToString(" · ")
            if (meta.isNotEmpty()) {
                Text(
                    meta,
                    Modifier.padding(top = 3.dp),
                    fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp,
                    color = MaterialTheme.editorial.Muted,
                    maxLines = 2, overflow = TextOverflow.Ellipsis,
                )
            }
        }
        item.actionLabel?.let {
            Text(
                it,
                fontSize = PaEditorial.Type.Caption.fontSize, fontWeight = FontWeight.SemiBold,
                color = MaterialTheme.editorial.Primary,
            )
            Spacer(Modifier.width(6.dp))
        }
        (item as? AttentionItem.UnknownEvent)?.let { u ->
            Text(unknownStatusLabel(u.processingStatus), fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Subtle)
            Spacer(Modifier.width(6.dp))
        }
        Icon(
            painterResource(R.drawable.ic_lucide_chevron_right), null,
            tint = PaEditorial.Subtle, modifier = Modifier.size(16.dp),
        )
    }
}

// ---------------------------------------------------------------- 纯函数(单测钉死)

// ---- 新建会话的 agent 接线(纯函数,单测覆盖 NewThreadAgentSelectionTest)----

/**
 * 新建会话应携带的 agent_id:
 * - 空串 / "codex" → 返回空串,请求不设 agent_id = 旧语义,由 server 端默认路由到 codex;
 * - 其他(pi / zcode / …)→ 原样返回,CreateThreadRequest.agent_id 生效,
 *   server 端 providerForThread 按它路由到对应后端。
 *
 * 【为什么必须收敛成纯函数】新建会话入口有两处(主页"新建任务"卡 + ChatScreen 内),
 * 判定逻辑必须完全一致;且 daemon 侧 agent 不可用时仍允许建会话,
 * 首条消息会得到 server 的人话错误 —— 这里不做能力预检。
 */
internal fun createThreadAgentId(selectedAgent: String): String =
    if (selectedAgent.isBlank() || selectedAgent == "codex") "" else selectedAgent.trim()

/**
 * agent id → 展示名:与设置页「切换 Agent」的命名保持一致
 * (SettingsScreen 内映射是 private,这里按同款名单给一份只读展示名;未知 id 原样显示)。
 * 用于主页"新建任务"区块头、待审批卡/最近会话行的 agent 名。
 */
internal fun selectedAgentLabel(agentId: String): String = when (agentId) {
    "", "codex" -> "Codex"
    "opencode" -> "OpenCode"
    "claude" -> "Claude"
    "pi" -> "Pi"
    "hermes" -> "Hermes"
    "zcode" -> "ZCode"
    else -> agentId
}

/**
 * 从 capabilities 提取"已接入跨 agent 委派协同"的 agent id（纯函数）。
 *
 * daemon 报 `collab:<agentId>` —— 语义是"该 agent 已把本地 collab MCP 挂进自己的
 * 会话"，也就是它那一侧的模型**看得见 agent_delegate**、能派发/接收子智能体。
 * 判定完全由 daemon 的接线事实驱动（新增一家自动出现），手机端**不写名单**；
 * 能力缺失 = 该家尚未接线，如实不显示，不做假入口。
 */
internal fun resolveCollabAgentIds(capabilities: List<String>): List<String> =
    capabilities
        .filter { it.startsWith("collab:", ignoreCase = true) }
        .map { it.substringAfter(':').trim() }
        .filter { it.isNotEmpty() }
        .distinct()
        .sorted()

/**
 * 协同行副标题（纯函数）：把已接入的 agent 说成人话。
 * 空列表 = 尚未有任何 agent 接入，给出"为什么没有"的下一句，而不是"开发中"。
 */
internal fun collabSubtitle(agentIds: List<String>): String =
    if (agentIds.isEmpty()) {
        "尚未接入：电脑端启用协作后自动出现"
    } else {
        "已接入 " + agentIds.joinToString("、") + "（可派发子智能体）"
    }

/**
 * 最近会话卡的元信息行(纯函数,单测覆盖):「N 条消息 · 相对时间」。
 *
 * 【为什么不是最后一条消息预览】ListThreads 的 Thread 消息只有 message_count 与
 * updated_at_unix_ms,数据层没有"最后一条消息正文"的源;要预览就得每张卡补一轮
 * getThread RPC(列表页打不起)或拿标题造假 —— 取「条数 · 相对时间」补上时间维度,
 * 时间一律走 daemon 落库的更新时刻,时钟倒挂(时间戳在未来)由 [relativeTimeLabel]
 * 回落绝对日期,不出现"负 N 分钟前"。
 */
internal fun threadRecentMeta(messageCount: Int, updatedAtUnixMs: Long, nowMs: Long): String {
    val count = "$messageCount 条消息"
    val rel = relativeTimeLabel(updatedAtUnixMs, nowMs)
    return if (rel.isBlank()) count else "$count · $rel"
}

// ---- 主页注意力三分(P0-3 + P1-1;纯函数,单测覆盖 HomeAttentionSectionsTest)----

/**
 * 会话三分结果:待审批 / 运行中 / 其余。
 *
 * 【数据源即 ListThreads】daemon 的 threadToPB 对每条会话都带 status
 * (ThreadStatus.RUNNING / WAITING_APPROVAL)与 pending_approvals 快照,
 * 主页现有的 listThreads 应答里就有,不新增任何 RPC。
 */
internal data class HomeAttentionGroups(
    val pendingApproval: List<PThread>,
    val running: List<PThread>,
    val rest: List<PThread>,
)

/**
 * 会话三分(纯函数):待审批组、运行中组、其余组。
 *
 * 【归组规则】
 *  - 待审批:pending_approvals 非空 **或** status==WAITING_APPROVAL —— 后者覆盖
 *    外部 agent(zcode 子会话 waiting 语义)只有状态、没有审批对象的情况;
 *  - 运行中:status==RUNNING,且**不与待审批重复**(一回事话同时挂着审批与
 *    running 状态时归待审批组,等待用户的是它,不该再占一条运行位);
 *  - 其余:除开两组的剩余(主页「最近会话」仍走它自己的既有过滤,本组为
 *    三分完备性而返回,渲染暂不消费)。
 *
 * 【先减损再归组】归档(status==ARCHIVED)、子代理存根(parentThreadId 非空,
 * 与 ThreadsScreen 主列表同口径)、已停用 agent 的会话,三种都不进任何组 ——
 * 停用偏好「停用后不再显示」对注意区同样生效,不给出戏的例外。
 *
 * 【组内排序】updatedAtUnixMs 倒序:最需要被看到的(刚出事的)排最前。
 */
internal fun partitionHomeAttention(threads: List<PThread>, disabledAgents: Set<String>): HomeAttentionGroups {
    val eligible = threads.filter {
        it.parentThreadId.isBlank() &&
            it.status != ThreadStatus.THREAD_STATUS_ARCHIVED &&
            threadAgentIdOf(it.agentId) !in disabledAgents
    }
    val pending = eligible.filter {
        it.pendingApprovalsCount > 0 || it.status == ThreadStatus.THREAD_STATUS_WAITING_APPROVAL
    }.toSet()
    val running = eligible.filter { it !in pending && it.status == ThreadStatus.THREAD_STATUS_RUNNING }.toSet()
    val byRecency = compareByDescending<PThread> { it.updatedAtUnixMs }
    return HomeAttentionGroups(
        pendingApproval = pending.sortedWith(byRecency),
        running = running.sortedWith(byRecency),
        rest = (eligible - pending - running).sortedWith(byRecency),
    )
}

/** 运行中会话的归属 agent 集合(纯函数):agent 磁贴「正在执行」小徽章的数据源。 */
internal fun agentIdsOfRunning(running: List<PThread>): Set<String> =
    running.map { threadAgentIdOf(it.agentId) }.toSet()

/**
 * mini 条的运行中会话并集(方向 C N1;纯函数,单测覆盖 HomeRunningMiniBarTest)。
 *
 * 【两个真源,各取所长】
 *  - [daemonRunning]=首页既有 listThreads 三分的运行组(daemon 快照:带标题/归属/
 *    turn_started,但随 reloadKey/重连才刷新);
 *  - [liveRunningIds]=[TurnSession.runningThreadIds] 只读快照(进程内真相,5s
 *    轮询即新鲜,但只有 threadId)。
 * 并集去重:同 id 以 daemon 快照为准(信息更全);live 独有的(快照滞后窗口期)
 * 从 [knownThreads](本页已加载的会话列表)补全名片,补不到的给未命名占位 ——
 * 数据全真,不编标题。[stoppable] 只有 live 侧为真:停止走 TurnSession.cancelTurn
 * 现有语义,daemon 侧(电脑端发起)的会话手机端没有停止通道,不摆假停止位。
 *
 * 【排序】启动键倒序(turn_started 优先,缺则 updatedAt):「最近启动的一个」
 * 做 mini 条主会话;稳定排序,并列保持传入顺序。
 */
internal fun buildPaRunningSessions(
    daemonRunning: List<PThread>,
    liveRunningIds: Set<String>,
    knownThreads: List<PThread>,
    nowMs: Long,
): List<PaRunningSessionUi> {
    val known = knownThreads.associateBy { it.id }
    val merged = LinkedHashMap<String, PThread>()
    daemonRunning.forEach { if (it.id.isNotBlank()) merged[it.id] = it }
    liveRunningIds.forEach { tid ->
        if (tid.isNotBlank()) {
            merged.getOrPut(tid) {
                known[tid] ?: PThread.newBuilder().setId(tid).setUpdatedAtUnixMs(nowMs).build()
            }
        }
    }
    return merged.values.map { t ->
        val agentId = threadAgentIdOf(t.agentId)
        PaRunningSessionUi(
            threadId = t.id,
            title = t.title.ifBlank { "(未命名)" },
            agentLabel = selectedAgentLabel(agentId),
            statusWhisper = runningMetaLabel(
                if (t.hasTurnStartedUnixMs()) t.turnStartedUnixMs else null,
                t.updatedAtUnixMs,
                nowMs,
            ),
            iconRes = agentBadgeRes(agentId),
            startedSortKey =
                if (t.hasTurnStartedUnixMs() && t.turnStartedUnixMs > 0) t.turnStartedUnixMs
                else t.updatedAtUnixMs,
            stoppable = t.id in liveRunningIds,
        )
    }.sortedByDescending { it.startedSortKey }
}

/**
 * 待审批卡元信息(纯函数,四词口径):有审批对象=报数;
 * 只有 WAITING_APPROVAL 状态、没有审批对象(外部 agent 的 waiting 语义)=直说在等。
 * 【四词纪律】状态词只准「等待你处理」,不再出现「待你处理/待处理」等变体。
 */
internal fun pendingApprovalMeta(pendingCount: Int): String =
    if (pendingCount > 0) "$pendingCount 项等待你处理" else "等待你处理"

/**
 * 运行中条元信息(纯函数):有真实起点 → 「运行 N 分钟」;缺席 → 「最近活动 · 相对时间」。
 *
 * 【时长从哪来】daemon 的 Thread.turn_started_unix_ms(2026-09-26 补齐:Status 转
 * RUNNING 时记 now;重启后仍 RUNNING 的会话记重启时刻 —— 真实开始不可追溯)。
 * 此前协议没有该字段,拿 updatedAt 冒充开始时刻就是造假,故降级为"最近活动"
 * 相对时间;现在字段缺席(旧 daemon/未知)时仍走该展示,数据全真。
 * 【四词纪律】不再返回裸状态词「运行中」—— 状态词由 PaStatusBadge 的
 * 「正在执行」承担,本函数只给时间维度;两个时间都缺席时返回空串(不摆字)。
 */
internal fun runningMetaLabel(
    turnStartedUnixMs: Long?,
    updatedAtUnixMs: Long,
    nowMs: Long,
): String {
    if (turnStartedUnixMs != null && turnStartedUnixMs > 0L) {
        val minutes = (nowMs - turnStartedUnixMs).coerceAtLeast(0L) / 60_000L
        return when {
            minutes < 1L -> "运行不到 1 分钟"
            minutes < 60L -> "运行 $minutes 分钟"
            else -> "运行 ${minutes / 60L} 小时 ${minutes % 60L} 分"
        }
    }
    val rel = relativeTimeLabel(updatedAtUnixMs, nowMs)
    return if (rel.isBlank()) "" else "最近活动 · $rel"
}

// ---- agent 停用偏好生效链(管理 agent 抽屉 → 展示/创建入口;纯函数,单测覆盖 AgentDisablePreferenceTest)----

/**
 * agent 是否应出现在展示面(主页网格/协同行/最近会话,纯函数):
 * 已停用(本地用户偏好层)一律不显示 —— 抽屉副标题「停用后不再显示该 agent」的判定落点。
 * 空白 id 视为未停用(无 id 可停,零行为)。
 */
internal fun shouldShowAgent(agentId: String, disabledAgents: Set<String>): Boolean =
    agentId.trim() !in disabledAgents

/**
 * 展示清单过滤:已停用 agent 从网格/协同行隐藏。
 * 【与能力判定的边界】只做减法,不改写 [AgentInfo.available] —— 可用性仍由
 * [resolveAgentAvailability] 按 daemon capabilities 判定,恢复(从停用集合移除)后
 * 磁贴原样回来,能力判定不受偏好层污染。
 */
internal fun filterAgents(agents: List<AgentInfo>, disabledAgents: Set<String>): List<AgentInfo> =
    agents.filter { shouldShowAgent(it.id, disabledAgents) }

/**
 * 管理 agent 抽屉开关的点按结果(纯函数):
 *  - 可用 agent:开关=真实停用/恢复(写 [AgentConnection.setAgentDisabled] 偏好,立即生效);
 *  - 不可用(daemon 未报)agent:不伪造「已停用」—— 停用态原地不动,只给真实原因提示
 *    (「接入中」/「电脑端未安装」),开关回弹到原状态。
 * @param requestOn 用户想要的目标:开=显示(恢复) / 关=停用
 * @param disabled  当前停用态(不可用 agent 原样返回,防"误恢复"用户先前的停用)
 * @return (目标停用态, 待弹提示;null=不打扰)
 */
internal fun agentManageToggleResult(
    requestOn: Boolean,
    available: Boolean,
    disabled: Boolean,
    agentId: String,
    name: String,
): Pair<Boolean, String?> = when {
    !available -> disabled to "「$name」${unavailableLabel(agentId)}"
    !requestOn -> true to "已停用「$name」,可在此恢复"
    else -> false to "已恢复「$name」"
}

/**
 * 新建任务放行判定(纯函数):已停用 agent 不再创建新会话(入口同步排除)。
 * 恢复后照旧放行。口径:入参 trim 后按 [threadAgentIdOf] 归属判定
 * (空串按 codex;写侧 [AgentConnection.setAgentDisabled] 也 trim,两侧一致)。
 */
internal fun newThreadAgentAllowed(selectedAgent: String, disabledAgents: Set<String>): Boolean =
    threadAgentIdOf(selectedAgent.trim()) !in disabledAgents
