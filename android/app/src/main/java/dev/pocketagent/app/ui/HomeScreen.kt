package dev.pocketagent.app.ui

import android.widget.Toast
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorMatrix
import androidx.compose.ui.graphics.ColorMatrixColorFilter
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.edgeSwipeToOpenDrawer
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.component.ConnectionBanner
import dev.pocketagent.app.ui.component.ThreadStatusBadge
import dev.pocketagent.app.ui.component.threadStatusBadge
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.paPushBack
import androidx.core.graphics.drawable.toBitmap
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CreateThreadRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.HostInfo
import pocketagent.v1.ListThreadsRequest
import pocketagent.v1.Thread as PThread
import pocketagent.v1.ThreadStatus

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
 * 主页 —— 按用户草图 + UX 深审 P0-3 的 GitHub Mobile 范式重排
 * (需要我处理的事 > 正在运行的事 > 新建 > 实体网格 > 历史):
 * [Pa 图标=菜单(左上)] [设置(右上)] → 连接状态卡 →
 * 待你处理(条件渲染:有待审批的会话卡置顶) → 运行中(条件渲染:RUNNING 会话横条) →
 * 新建任务/电脑预览 → agents 板块(磁贴带「运行中」小徽章) → 最近任务 →
 * 多agent协同(开关语义:关=只显示在用 agent,开=显示已加入协同的 agents)。
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
    val scope = androidx.compose.runtime.rememberCoroutineScope()
    val ctx = LocalContext.current
    var host by remember { mutableStateOf<HostInfo?>(null) }
    var recent by remember { mutableStateOf(listOf<PThread>()) }
    var showManage by remember { mutableStateOf(false) }
    var showDrawer by remember { mutableStateOf(false) }
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
    // 自动重连恢复后(connectionEpoch 变化)自动刷新,拔 USB 再回来无需手点重试
    val reconnectTick = AgentConnection.connectionEpoch.collectAsState().value

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
            }
        } catch (e: kotlin.coroutines.cancellation.CancellationException) {
            // effect 因 reloadKey/重连 epoch 变化被重启时协程会收到取消,
            // 这是正常重启不是加载失败,别把"The coroutine scope left the
            // composition"写进错误横幅(旧实现重试一次挂一次的根因)
            throw e
        } catch (e: Exception) {
            loadError = "首页数据加载失败：${e.message ?: "连接失败"}"
        }
    }

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
    // 这一层同时驱动两处:① 协同卡片的状态点与文案;② 协同行"只显示在用 agent
    // 还是显示全部已加入协同的 agents"。
    val collabIds = remember(sharedCaps) { resolveCollabAgentIds(sharedCaps) }
    val multiAgent = collabIds.isNotEmpty()
    // 支持协同时,顺带把协同角色列表拉一次,供首页这行的数量展示;
    // 失败/无数据都不影响首页(宁缺勿假,数量留空即不显示数字)。
    LaunchedEffect(collabIds) {
        if (collabIds.isNotEmpty()) CollabRolesState.load()
    }
    // 停用偏好生效链(AgentConnection.disabledAgents 可观察,拨开关立即重算):
    // 已停用 agent 从网格/协同行/最近任务隐藏 —— 管理抽屉副标题「停用后不再显示」的兑现;
    // 恢复入口恒在管理抽屉(它收全量 agents 清单)。偏好层只隐藏,不改写 available,
    // 与 T-10 的 capabilities 能力判定互不覆盖(恢复后判定照旧)。
    val disabledAgents = AgentConnection.disabledAgents
    val visibleAgents = remember(agents, disabledAgents) { filterAgents(agents, disabledAgents) }

    // 【P0-3 注意力三分】待审批 / 运行中 / 其余,数据就是本页现有的 listThreads
    // 应答(daemon 每条 Thread 都带 status 与 pending_approvals 快照,零新增 RPC);
    // 刷新节奏与最近任务同源(reloadKey / 重连 epoch),不做额外轮询。
    val attention = remember(recent, disabledAgents) { partitionHomeAttention(recent, disabledAgents) }
    // agent 磁贴「运行中」小徽章:该 agent 名下有运行中会话(已剔除停用)就点亮
    val runningAgentIds = remember(attention) { agentIdsOfRunning(attention.running) }

    // 【推背模糊】任一浮层(侧边栏/管理面板/引导弹窗/新建任务)激活时,主页内容根
    // 模糊+缩放+压暗 —— 浮层是玻璃,透过玻璃看内容才是真磨砂。模糊挂在被覆盖的
    // 内容层上(整屏一次一层),不在浮层自己身上,性能纪律见 PaGlass.kt。
    val overlayActive = drawerCtrl.visible || showManage || guideAgent != null || showNewThread

    // 左边缘右滑也能唤出侧边栏(问题 7),与左上角图标点按是同一个入口的两种手势
    Column(
        modifier.fillMaxSize().background(Pa.Bg).statusBarsPadding()
            .paPushBack(overlayActive)
            .edgeSwipeToOpenDrawer(
                        onDragFraction = drawerCtrl::onDrag,
                        onRelease = drawerCtrl::onRelease,
                    ),
    ) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            // 顶部工具钮:裸图标(icon-only,无底、无阴影、无发丝边)。反馈只在按下
            // 瞬间(IconActionButton 内建 pressable 缩放)——首页唯一"浮"的东西应该
            // 是内容本身,不是工具钮的容器。
            IconActionButton(R.drawable.ic_lucide_menu, "菜单", { drawerCtrl.open() }, bg = Color.Transparent)
            // 右上角:设置
            IconActionButton(R.drawable.ic_lucide_settings, "设置", onOpenSettings, bg = Color.Transparent)
        }

        LazyVerticalGrid(
            columns = GridCells.Fixed(4),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 16.dp),
            // 【统一卡语言】卡与卡间距 12dp(全 app 唯一卡距)
            verticalArrangement = Arrangement.spacedBy(12.dp),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            // ---- 断连全局横幅(P1-2):与会话页/会话列表同一条断连状态 ----
            // 只在 connState==down 时渲染(组件内部判定),重连成功自动消失;
            // 手动重试走本页既有 reloadKey 重载链(横幅自己不实现重连)。
            item(span = { GridItemSpan(4) }) {
                ConnectionBanner(onRetry = { reloadKey++ })
            }

            // 连接卡(点按重试)
            item(span = { GridItemSpan(4) }) {
                ConnectionCardCompact(host = host, onRetry = { reloadKey++ })
            }
            loadError?.let { message ->
                item(span = { GridItemSpan(4) }) {
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .pressable { reloadKey++ }
                            // 单层:错误色 8% 透明底——横幅是语义状态,不是层级;圆角随统一卡型 16dp
                            .background(Pa.Red.copy(alpha = 0.08f), RoundedCornerShape(16.dp))
                            .padding(horizontal = 14.dp, vertical = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(message, Modifier.weight(1f), fontSize = 12.sp, color = Pa.Red, maxLines = 2)
                        Text("重试", fontSize = 12.sp, color = Pa.Accent)
                    }
                }
            }

            // ---- 待你处理(P0-3 首位,条件渲染):有待审批的会话卡置顶 ----
            // 【为什么排第一】出事的东西最优先(GitHub Mobile 范式):审批卡在等用户,
            // 埋在网格/历史里就是"agent 停摆等人却不知道"。无待审批 → 整区不渲染,
            // 不摆"一切正常"的空壳。
            if (attention.pendingApproval.isNotEmpty()) {
                item(span = { GridItemSpan(4) }) {
                    PendingAttentionSection(threads = attention.pendingApproval, onOpenThread = onOpenThread)
                }
            }

            // ---- 运行中(P0-3 第二位,条件渲染):RUNNING 会话横条 ----
            // 与待审批互斥归组(挂着审批的会话只出现在上面,不重复占运行位);
            // 无运行中 → 整区不渲染。
            if (attention.running.isNotEmpty()) {
                item(span = { GridItemSpan(4) }) {
                    RunningAttentionSection(threads = attention.running, onOpenThread = onOpenThread)
                }
            }

            // 新建任务入口:按"需要我处理 > 运行中 > 新建 > 实体网格"的节拍,
            // 排在两个注意区之后、agent 网格之前(P0-3 审计口径)。
            // 独立小行(统一卡:单层 Pa.Card·16dp 圆角)
            item(span = { GridItemSpan(4) }) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable { showNewThread = true }
                        .background(Pa.Card, RoundedCornerShape(16.dp))
                        .padding(horizontal = 16.dp, vertical = 14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(painterResource(R.drawable.ic_lucide_plus), null, tint = Pa.Accent, modifier = Modifier.size(22.dp))
                    Spacer(Modifier.width(12.dp))
                    Text("新建任务", Modifier.weight(1f), fontSize = 15.sp, color = Pa.TextPrimary)
                    // 右侧 agent 名标签:跟随唯一选中源 AgentConnection.selectedAgent 实时显示
                    // (设置页「切换 Agent」/ 主页网格点选都写它),不再硬编码 "Codex"。
                    // 【加「当前:」前缀】裸放 "dsh"/"Pi" 像一行没有归属的杂字(L2 真机挑刺),
                    // 前缀点明"新建的任务归谁",信息保留、语义自明;保持 11sp TextTertiary 不与标题抢层级。
                    Text("当前: " + selectedAgentLabel(AgentConnection.selectedAgent), fontSize = 11.sp, color = Pa.TextTertiary)
                }
            }

            // ---- 电脑预览:浏览器里一键看电脑上正跑着的 dev server(P2)----
            // 复用既有 onOpenBrowser 入口打开内置浏览器,浏览器顶栏的"电脑预览"
            // 按钮列出 daemon 探到的端口(3000/5173/8000…),点一项直接加载。
            // 与「新建任务」同组入口行,跟随其上移,不拆散。
            item(span = { GridItemSpan(4) }) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable { onOpenBrowser() }
                        .background(Pa.Card, RoundedCornerShape(16.dp))
                        .padding(horizontal = 16.dp, vertical = 14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(painterResource(R.drawable.ic_lucide_monitor), null, tint = Pa.Accent, modifier = Modifier.size(22.dp))
                    Spacer(Modifier.width(12.dp))
                    Text("电脑预览", Modifier.weight(1f), fontSize = 15.sp, color = Pa.TextPrimary)
                    Text("dev server", fontSize = 11.sp, color = Pa.TextTertiary)
                }
            }

            // ---- agent 板块:标题 + 直接铺背景的网格(无外层大卡) ----
            // 【区块节拍】区块间距统一 20dp(网格 12dp + 本行 8dp):与「最近任务」
            // 区块头同款节奏,板块与板块读得分得开,卡与卡仍守 12dp 统一卡距。
            item(span = { GridItemSpan(4) }) {
                Column(Modifier.fillMaxWidth().padding(top = 8.dp)) {
                    // 区块头:标题在左、「管理」入口在右,直接放背景上(不再包卡)
                    Row(
                        Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text("Agents", fontSize = 16.sp, color = Pa.TextPrimary)
                        // 管理 agent 抽屉入口(停用/恢复都在抽屉里):网格只负责"停用后
                        // 不再显示",恢复入口必须永远可达(抽屉收全量清单),否则停用的
                        // agent 会连恢复的门一起消失。
                        Text(
                            "管理",
                            Modifier.pressable { showManage = true }.padding(horizontal = 6.dp, vertical = 2.dp),
                            fontSize = 12.sp, color = Pa.Accent,
                        )
                    }
                    Spacer(Modifier.height(12.dp))
                    // 磁贴:真实图标 + 灰阶对比(不可用),透明底直接铺在页面背景上;
                    // 已停用不在列。磁贴本身 pressable(按下即缩放),不再有任何底盒。
                    androidx.compose.foundation.layout.Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
                        visibleAgents.chunked(4).forEach { rowAgents ->
                            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly) {
                                rowAgents.forEach { a ->
                                    // 「运行中」小徽章(P1-1):该 agent 名下有运行中会话时,
                                    // 磁贴右上角脉冲绿点 —— 状态语义与「运行中」区同源同色
                                    AgentTile(a, runningBadge = a.id in runningAgentIds) {
                                        if (a.available) {
                                            // 选中 agent = 切换后端,不再自动跳进终端页。
                                            // 终端只在用户主动点【终端】时打开,平时保持
                                            // "无感":切换 agent 不打断当前界面。
                                            AgentConnection.switchAgent(a.id)
                                            onGoTab(1)
                                            Toast.makeText(ctx, "已切换到「${a.name}」", Toast.LENGTH_SHORT).show()
                                        } else {
                                            // 灰显=引导:弹窗讲清"为什么不可用、怎么装"
                                            guideAgent = a
                                        }
                                    }
                                }
                                // 补位对齐 4 列
                                repeat(4 - rowAgents.size) { Spacer(Modifier.width(72.dp)) }
                            }
                        }
                    }
                }
            }

            // ---- 最近任务 ----
            item(span = { GridItemSpan(4) }) {
                Row(
                    Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 2.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text("最近任务", fontSize = 16.sp, color = Pa.TextPrimary)
                    // 【假入口修复】「全部 →」长得像链接却没挂任何点击(L2 自查补挑刺);
                    // 接到会话列表页(tab 1)= "看全部任务"的真出口,与「管理」同款行尾语言。
                    Text(
                        "全部 →",
                        Modifier.pressable { onGoTab(1) }.padding(horizontal = 6.dp, vertical = 2.dp),
                        fontSize = 13.sp, color = Pa.TextSecondary,
                    )
                }
            }
                if (recent.isEmpty()) {
                    item(span = { GridItemSpan(4) }) {
                        // 空状态:直接铺背景(无容器),靠留白与居中自成一块。
                        // 【36→24dp】36dp 超出区块间距上限(≤32dp),收进规格带。
                        Column(
                            Modifier.fillMaxWidth().padding(vertical = 24.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                        ) {
                            Icon(painterResource(R.drawable.ic_lucide_list_checks), null, tint = Pa.TextTertiary, modifier = Modifier.size(36.dp))
                            Text("连接电脑后,任务会出现在这里", Modifier.padding(top = 12.dp), fontSize = 13.sp, color = Pa.TextSecondary)
                        }
                    }
                } else {
                    // 最近任务同步排除已停用 agent 的会话(归属缺省按 codex,与 threadAgentIdOf 同口径)
                    items(recent.filter { threadAgentIdOf(it.agentId) !in disabledAgents && it.agentId.ifBlank { "codex" } == AgentConnection.selectedAgent }.take(6), key = { it.id }, span = { GridItemSpan(4) }) { t ->
                        // 会话三态徽章(P1-1):与 ThreadsScreen 会话卡同一纯函数,
                        // 同一会话在主页与列表长出同一种状态(运行中/待你处理/中断)。
                        val badge = threadStatusBadge(t.status, t.pendingApprovalsCount, t.lastTurnError)
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .pressable { onOpenThread(t.id) }
                                .background(Pa.Card, RoundedCornerShape(16.dp))
                                .padding(horizontal = 16.dp, vertical = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Image(
                                // 【agent 徽标】按线程归属 agent 选图(agent_id 缺省 = codex),
                                // 复用 ThreadsScreen 同款映射:导入的 Pi/ZCode 会话不再错标 Codex。
                                bitmap = agentBitmap(agentBadgeRes(t.agentId)),
                                contentDescription = null,
                                modifier = Modifier.size(26.dp),
                                contentScale = ContentScale.Crop,
                            )
                            Spacer(Modifier.width(10.dp))
                            Column(Modifier.weight(1f)) {
                                Text(
                                    t.title.ifEmpty { "(未命名)" },
                                    fontSize = 14.sp, color = Pa.TextPrimary, maxLines = 1,
                                    overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                                )
                                // 【加相对时间】ListThreads 的 Thread 只有消息计数与更新时刻,
                                // 没有最后一条消息正文源 —— 预览要么多打一轮 getThread RPC
                                // 要么造假;改用「N 条消息 · 相对时间」补时间维度(纯函数见
                                // [threadRecentMeta],单测钉死)。
                                Text(
                                    threadRecentMeta(t.messageCount, t.updatedAtUnixMs, System.currentTimeMillis()),
                                    fontSize = 11.sp, color = Pa.TextTertiary,
                                )
                                // 中断摘要(P1-1 Failed 徽章的"最后错误摘要"):一行缩略,
                                // 让"这单为什么停了"不用点进会话就能看到;正常态无此行。
                                badge?.detail?.let { summary ->
                                    Text(
                                        summary,
                                        fontSize = 10.5.sp, color = Pa.RedText, maxLines = 1,
                                        overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                                    )
                                }
                            }
                            badge?.let { ThreadStatusBadge(it) }
                        }
                    }
                }

            // ---- 多agent协同:真实状态(不再是"开发中"假开关) ----
            // 【P0-3 重排说明】区块整体后移到「最近任务」之后:范式是"要我处理的 >
            // 运行中的 > 新建 > 实体网格 > 历史",协同入口不属于注意区;块内结构
            // 与视觉维持现状(风格统一已达标,本轮不动)。
            // 状态完全由 daemon 的接线事实驱动:capabilities 里的 collab:<id>。
            // 这里刻意**不做成可拨的开关** —— 它不是用户偏好,而是"电脑上接线完成
            // 与否"的事实回显;可拨 = 假可用(旧实现正是栽在这)。所以只读展示 +
            // 常显状态点,让"已接入几家、谁"一眼可见。
            // 【区块节拍】与 Agents 板块同款:区块间 20dp。
            item(span = { GridItemSpan(4) }) {
                Column(Modifier.fillMaxWidth().padding(top = 8.dp)) {
                    // 区块头(标题行尾状态点 + 标题 + 副标):直接铺背景,不包卡。
                    // 【状态点收进行内】旧版绿点悬在标题块左缘外,像贴错位置的孤点
                    // (L2 真机挑刺);移到标题行末尾,与文案同一基线,不再占独立列。
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Text("多agent协同", fontSize = 16.sp, color = Pa.TextPrimary)
                                Spacer(Modifier.width(6.dp))
                                Box(
                                    Modifier
                                        .size(8.dp)
                                        .background(
                                            if (collabIds.isEmpty()) Pa.TextTertiary else Pa.Green,
                                            CircleShape,
                                        ),
                                )
                            }
                            Text(
                                collabSubtitle(collabIds),
                                fontSize = 11.sp,
                                color = if (collabIds.isEmpty()) Pa.TextTertiary else Pa.AccentText,
                            )
                        }
                    }
                    // 协同角色入口:只有设备确实支持协同时才显示(无能力不显示假入口)。
                    // 列出已配置数量,点进去是角色列表页(真正完成"分工编排")。
                    if (multiAgent) {
                        Spacer(Modifier.height(10.dp))
                        // 【开始协同:主入口,排在"协同角色"之前】两者刻意分两行:
                        // 这行是"现在就派活",下一行是"改岗位配置"。混成一行会让用户
                        // 以为协同必须先配角色才能用。
                        // 【统一卡】单层 Pa.Card·16dp 圆角·无阴影无发丝边·水平内距 16dp;
                        // 卡内图标裸放(无底盒)——"外层大卡包白色行卡"的三层观感即由此打平。
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .pressable { onOpenCollab() }
                                .background(Pa.Card, RoundedCornerShape(16.dp))
                                .padding(horizontal = 16.dp, vertical = 14.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            // 图标 22dp:与「新建任务」/「电脑预览」行的 22dp 图标同一档
                            // (旧 18dp 在这两张主入口卡上偏小,层级读起来比次级入口还弱)
                            Icon(painterResource(R.drawable.ic_lucide_git_branch), null, tint = Pa.Accent, modifier = Modifier.size(22.dp))
                            Spacer(Modifier.width(10.dp))
                            Column(Modifier.weight(1f)) {
                                Text("开始协同", fontSize = 13.5.sp, color = Pa.TextPrimary)
                                Text(
                                    if (collabIds.isEmpty()) "当前没有可协同的 agent" else "一条指令派给多个 agent 并行",
                                    // 副文升 TextSecondary:入口卡的说明是正文信息,不是脚注
                                    fontSize = 11.sp, color = Pa.TextSecondary,
                                )
                            }
                            Icon(painterResource(R.drawable.ic_lucide_chevron_right), null, tint = Pa.TextTertiary, modifier = Modifier.size(16.dp))
                        }
                        Spacer(Modifier.height(12.dp))
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .pressable { onOpenCollabRoles() }
                                .background(Pa.Card, RoundedCornerShape(16.dp))
                                .padding(horizontal = 16.dp, vertical = 14.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            // 图标 22dp:与「开始协同」同档(两卡是同组主入口,视觉重量应一致)
                            Icon(painterResource(R.drawable.ic_lucide_list_checks), null, tint = Pa.Accent, modifier = Modifier.size(22.dp))
                            Spacer(Modifier.width(10.dp))
                            Column(Modifier.weight(1f)) {
                                Text("协同角色", fontSize = 13.5.sp, color = Pa.TextPrimary)
                                val roleCount = if (CollabRolesState.loadState == CollabRolesLoadState.READY) CollabRolesState.roles.size else null
                                Text(
                                    if (roleCount == null) "点按管理岗位配置" else "已配置 $roleCount 个岗位",
                                    // 副文升 TextSecondary:同「开始协同」,入口卡说明不降级
                                    fontSize = 11.sp, color = Pa.TextSecondary,
                                )
                            }
                            Icon(painterResource(R.drawable.ic_lucide_chevron_right), null, tint = Pa.TextTertiary, modifier = Modifier.size(16.dp))
                        }
                    }
                    Spacer(Modifier.height(12.dp))
                    // 协同行:直接铺背景,与主网格同一磁贴语言(透明底)
                    Row(
                        Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        val shown = if (multiAgent) visibleAgents else visibleAgents.filter { it.id == AgentConnection.selectedAgent }
                        shown.forEach { a ->
                            AgentTile(a) {
                                if (a.available) {
                                    AgentConnection.switchAgent(a.id)
                                            onGoTab(1)
                                } else {
                                    // 与主网格一致:灰显点按 → 引导弹窗
                                    guideAgent = a
                                }
                            }
                        }
                        if (multiAgent) {
                            // 添加位:把更多 agent 加进协同(与磁贴同语言:裸 + 号,无底盒)
                            Column(
                                Modifier
                                    .width(72.dp)
                                    .pressable {
                                        Toast.makeText(ctx, "更多 agent 接入后可在此添加", Toast.LENGTH_SHORT).show()
                                    },
                                horizontalAlignment = Alignment.CenterHorizontally,
                            ) {
                                Box(Modifier.size(56.dp), contentAlignment = Alignment.Center) {
                                    Icon(
                                        painterResource(R.drawable.ic_lucide_plus), null,
                                        tint = Pa.TextTertiary, modifier = Modifier.size(26.dp),
                                    )
                                }
                                Text("添加", Modifier.padding(top = 6.dp), fontSize = 11.5.sp, color = Pa.TextTertiary)
                            }
                        }
                    }
                }
            }
        }
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
                                    val builder = CreateThreadRequest.newBuilder().setAgentId(AgentConnection.selectedAgent)
                                    if (AgentConnection.selectedModel.isNotEmpty()) {
                                        builder.setModelId(AgentConnection.modelForAgent(AgentConnection.selectedAgent))
                                    }
                                    // P1 修复:带上当前选中的 agent(设置页/主页切换后新建才真正生效)。
                                    // codex 或空保持不传 = 旧语义,由 server 端默认路由到 codex。
                                    val agentId = createThreadAgentId(AgentConnection.selectedAgent)
                                    if (agentId.isNotEmpty()) {
                                        builder.setAgentId(agentId)
                                    }
                                    if (ws.isNotBlank()) {
                                        builder.setWorkspace(ws)
                                    }
                                    val t = withContext(Dispatchers.IO) {
                                        AgentConnection.threadService().createThread(builder.build())
                                    }
                                    onOpenThread(t.id)
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

    // 左上角图标 → 侧边栏(主导航)
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
        )
    }
}

/**
 * agent 磁贴:官方品牌图标,可用=全彩+白名+连接方式;不可用=暗态。
 * [runningBadge]:该 agent 名下有运行中会话时,磁贴右上角脉冲绿点(P1-1,
 * 数据与「运行中」区同源)。协同行复用本磁贴时不传,维持现状不加徽章。
 */
@Composable
fun AgentTile(a: AgentInfo, runningBadge: Boolean = false, onClick: () -> Unit) {
    Column(
        Modifier.pressable { onClick() }.width(72.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box {
            Image(
                bitmap = agentBitmap(a.iconRes),
                contentDescription = a.name,
                modifier = Modifier
                    .size(56.dp)
                    .then(if (a.available) Modifier else Modifier.alpha(0.22f)),
                contentScale = ContentScale.Crop,
                colorFilter = if (a.available) null else grayscale(),
            )
            if (runningBadge) {
                PulseDot(Pa.Green, Modifier.align(Alignment.TopEnd))
            }
        }
        Text(
            a.name,
            Modifier.padding(top = 6.dp),
            fontSize = 11.5.sp,
            color = if (a.available) Pa.TextPrimary else Pa.TextTertiary,
            maxLines = 1,
        )
        Text(
            // 灰显态给真实原因:能力驱动型="电脑端未安装";未接入型="接入中"
            if (a.available) a.method else unavailableLabel(a.id),
            fontSize = 9.sp,
            color = if (a.available) Pa.Accent.copy(alpha = 0.75f) else Pa.TextTertiary.copy(alpha = 0.5f),
            maxLines = 1,
        )
    }
}

/**
 * agent 可用性引导弹窗:灰显磁贴点击后讲清"为什么不可用、怎么装"。
 * 统一玻璃浮层 PaSheet(玻璃底 + 内建滚动与高度上限,防小屏溢出)。
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
                Text(agent.name, fontSize = 19.sp, color = Pa.TextPrimary)
                Text(
                    // 状态行:占位展示项固定"接入中";能力驱动型分"已装 vX"/"电脑端未安装"两态
                    when {
                        agent.id in placeholderDisplayAgents -> AgentGuideCopy.SUBTITLE_INTEGRATING
                        installedVersion != null -> "已装 $installedVersion"
                        else -> AgentGuideCopy.SUBTITLE_NOT_INSTALLED
                    },
                    fontSize = 12.sp,
                    color = when {
                        agent.id in placeholderDisplayAgents -> Pa.TextSecondary
                        installedVersion != null -> Pa.Green
                        else -> Pa.Red
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
                fontSize = 11.sp, color = Pa.TextTertiary,
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
            .background(Pa.Card, RoundedCornerShape(16.dp))
            .padding(horizontal = 14.dp, vertical = 12.dp),
    ) {
        Text(title, fontSize = 12.sp, color = Pa.TextSecondary)
        Text(body, fontSize = 13.sp, color = Pa.TextPrimary, modifier = Modifier.padding(top = 4.dp))
    }
}

/**
 * 脉冲状态点:注意区语义色的小面积视觉载体(运行中=脉冲绿点,待审批=静态红点)。
 * 系统减弱动效开启时退化为静态点,与全 app 动效纪律一致(TaskCenter/PaSkeleton 同款)。
 */
@Composable
private fun PulseDot(color: Color, modifier: Modifier = Modifier, pulse: Boolean = true) {
    val reduceMotion = rememberReduceMotion()
    if (pulse && !reduceMotion) {
        val transition = rememberInfiniteTransition(label = "pulseDot")
        val alpha by transition.animateFloat(
            initialValue = 1f,
            targetValue = 0.35f,
            animationSpec = infiniteRepeatable(tween(900, easing = LinearEasing), RepeatMode.Reverse),
            label = "pulseAlpha",
        )
        Box(modifier.size(8.dp).graphicsLayer { this.alpha = alpha }.background(color, CircleShape))
    } else {
        Box(modifier.size(8.dp).background(color, CircleShape))
    }
}

/** 注意区区块头:标题 + 行尾语义状态点(与「多agent协同」头同款语言)。 */
@Composable
private fun AttentionSectionHeader(title: String, dotColor: Color, dotPulse: Boolean) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Text(title, fontSize = 16.sp, color = Pa.TextPrimary)
        Spacer(Modifier.width(6.dp))
        PulseDot(dotColor, pulse = dotPulse)
    }
}

/**
 * 「待你处理」区(P0-3 首位):有待审批请求的会话卡。
 * 条件渲染由调用方保证(空列表整区不出现);点卡 = 既有 onOpenThread 进会话,
 * 打开后既有管线(ChatScreen 的 st.pendingApproval → ApprovalSheet)把审批卡
 * 直接送上前,不新建路由。
 */
@Composable
private fun PendingAttentionSection(threads: List<PThread>, onOpenThread: (String) -> Unit) {
    Column(Modifier.fillMaxWidth().padding(top = 8.dp)) {
        AttentionSectionHeader("待你处理", Pa.Red, dotPulse = false)
        Spacer(Modifier.height(12.dp))
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            threads.take(4).forEach { t ->
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable { onOpenThread(t.id) }
                        // 统一卡型:单层 Pa.Card·16dp 圆角·无边无影,与全页同语言
                        .background(Pa.Card, RoundedCornerShape(16.dp))
                        .padding(horizontal = 16.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    PulseDot(Pa.Red, pulse = false)
                    Spacer(Modifier.width(10.dp))
                    Image(
                        bitmap = agentBitmap(agentBadgeRes(t.agentId)),
                        contentDescription = null,
                        modifier = Modifier.size(26.dp),
                        contentScale = ContentScale.Crop,
                    )
                    Spacer(Modifier.width(10.dp))
                    Column(Modifier.weight(1f)) {
                        Text(
                            t.title.ifEmpty { "(未命名)" },
                            fontSize = 14.sp, color = Pa.TextPrimary, maxLines = 1,
                            overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                        )
                        Text(
                            pendingApprovalMeta(t.pendingApprovalsCount),
                            fontSize = 11.sp, color = Pa.TextSecondary,
                        )
                    }
                    // 「待你处理」徽章:小面积语义色(红 10% 底 + 红字),不做大色块
                    Text(
                        "待你处理",
                        Modifier
                            .background(Pa.Red.copy(alpha = 0.10f), RoundedCornerShape(50))
                            .padding(horizontal = 8.dp, vertical = 3.dp),
                        fontSize = 10.5.sp, color = Pa.Red,
                    )
                }
            }
        }
    }
}

/**
 * 「运行中」区(P0-3 第二位):status==RUNNING 的会话横条(脉冲绿点+标题+最近活动)。
 * 条件渲染同上;点条 = 既有 onOpenThread 进对话。
 */
@Composable
private fun RunningAttentionSection(threads: List<PThread>, onOpenThread: (String) -> Unit) {
    Column(Modifier.fillMaxWidth().padding(top = 8.dp)) {
        AttentionSectionHeader("运行中", Pa.Green, dotPulse = true)
        Spacer(Modifier.height(12.dp))
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            threads.take(4).forEach { t ->
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable { onOpenThread(t.id) }
                        .background(Pa.Card, RoundedCornerShape(16.dp))
                        .padding(horizontal = 16.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    PulseDot(Pa.Green)
                    Spacer(Modifier.width(10.dp))
                    Image(
                        bitmap = agentBitmap(agentBadgeRes(t.agentId)),
                        contentDescription = null,
                        modifier = Modifier.size(26.dp),
                        contentScale = ContentScale.Crop,
                    )
                    Spacer(Modifier.width(10.dp))
                    Column(Modifier.weight(1f)) {
                        Text(
                            t.title.ifEmpty { "(未命名)" },
                            fontSize = 14.sp, color = Pa.TextPrimary, maxLines = 1,
                            overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                        )
                        Text(
                            runningMetaLabel(
                                // 起点缺席(旧 daemon/未知)传 null,回退"最近活动"展示
                                if (t.hasTurnStartedUnixMs()) t.turnStartedUnixMs else null,
                                t.updatedAtUnixMs,
                                System.currentTimeMillis(),
                            ),
                            fontSize = 11.sp, color = Pa.TextTertiary,
                        )
                    }
                    Text("运行中", fontSize = 11.sp, color = Pa.Green)
                }
            }
        }
    }
}

@Composable
private fun ConnectionCardCompact(host: HostInfo?, onRetry: () -> Unit = {}, modifier: Modifier = Modifier) {
    val connState = AgentConnection.connState.collectAsState().value
    val transport = AgentConnection.connTransport.collectAsState().value
    val diagnostic = AgentConnection.connError.collectAsState().value
    Row(
        modifier
            .fillMaxWidth()
            .pressable { onRetry() }
            // 页面唯一的 hero 统一卡:单层 Pa.Card·16dp 圆角·无阴影无发丝边;
            // 行内无带背景的子容器(图标裸放),信息层级靠字重与颜色。
            .background(Pa.Card, RoundedCornerShape(16.dp))
            .padding(horizontal = 16.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(painterResource(R.drawable.ic_lucide_monitor), null, tint = Pa.Accent, modifier = Modifier.size(24.dp))
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(host?.displayName ?: "未连接", fontSize = 15.sp, color = Pa.TextPrimary)
            Text(
                if (connState == "ok") "已连接 · $transport"
                else diagnostic?.message ?: "自动重连中 · 点按重试",
                fontSize = 11.sp,
                color = if (connState == "ok") Pa.Accent else if (diagnostic != null) Pa.Red else Pa.TextSecondary,
            )
        }
        Box(
            Modifier.size(8.dp).background(
                when {
                    connState == "ok" && host != null -> Pa.Green
                    connState == "down" -> Pa.Red
                    else -> Pa.TextTertiary
                },
                CircleShape,
            ),
        )
    }
}

/**
 * 管理 agent 抽屉:真实图标 + 名称 + 连接方式副标题 + 开关。
 * 统一玻璃浮层 PaSheet;列表滚动交给 PaSheet 内建滚动(高度上限防溢出,
 * pa_audit 的 sheet_overflow 规则在结构上不可能再触发)。
 */
@Composable
fun ManageAgentsSheet(agents: List<AgentInfo>, onDismiss: () -> Unit) {
    val ctx = LocalContext.current
    PaSheet(onDismissRequest = onDismiss) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp)) {
            Column(Modifier.weight(1f)) {
                Text("管理 agent", fontSize = 19.sp, color = Pa.TextPrimary)
                Text(
                    "停用后不再显示该 agent,正在运行的任务不受影响",
                    fontSize = 12.sp, color = Pa.TextSecondary, modifier = Modifier.padding(top = 2.dp),
                )
            }
            IconActionButton(R.drawable.ic_lucide_x, "关闭", onDismiss, bg = Color.Transparent, size = 38.dp, iconSize = 18.dp)
        }
        Spacer(Modifier.height(12.dp))
        // 之前此处整列 verticalScroll(T-MS):pa_audit sheet_overflow 规则首例命中。
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
                        .background(Pa.Card, RoundedCornerShape(16.dp))
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
                        Text(a.name, fontSize = 15.sp, color = Pa.TextPrimary)
                        Text(a.method + " · " + a.subtitle, fontSize = 11.sp, color = Pa.TextSecondary)
                        // T-MW2:不可用(daemon 未报)与已停用(本地偏好)两态目视可分
                        if (AgentConnection.isAgentDisabled(a.id)) {
                            Text("已停用", fontSize = 10.sp, color = Pa.TextTertiary)
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
                            checkedTrackColor = Pa.Accent,
                            checkedThumbColor = Color(0xFF0A2A4A),
                            uncheckedTrackColor = Pa.Hover,
                            uncheckedThumbColor = Pa.TextTertiary,
                            uncheckedBorderColor = Color.Transparent,
                        ),
                    )
                }
            }
            // 尾部留白随列滚动:列表占满视口时底部也有呼吸空间,滚到底不贴导航条
            Spacer(Modifier.height(30.dp))
        }
    }
}

// ---- 新建会话的 agent 接线(纯函数,单测覆盖 NewThreadAgentSelectionTest)----

/**
 * 新建会话应携带的 agent_id:
 * - 空串 / "codex" → 返回空串,请求不设 agent_id = 旧语义,由 server 端默认路由到 codex;
 * - 其他(pi / zcode / …)→ 原样返回,CreateThreadRequest.agent_id 生效,
 *   server 端 providerForThread 按它路由到对应后端。
 *
 * 【为什么必须收敛成纯函数】新建会话入口有两处(主页"新建任务"行 + ChatScreen 内),
 * 判定逻辑必须完全一致;且 daemon 侧 agent 不可用时仍允许建会话,
 * 首条消息会得到 server 的人话错误 —— 这里不做能力预检。
 */
internal fun createThreadAgentId(selectedAgent: String): String =
    if (selectedAgent.isBlank() || selectedAgent == "codex") "" else selectedAgent.trim()

/**
 * agent id → 展示名:与设置页「切换 Agent」的命名保持一致
 * (SettingsScreen 内映射是 private,这里按同款名单给一份只读展示名;未知 id 原样显示)。
 * 用于主页"新建任务"行右侧标签、NewThreadSheet 确认态。
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
 * 最近任务卡的元信息行(纯函数,单测覆盖):「N 条消息 · 相对时间」。
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

// ---- 主页注意力三分(P0-3 + P1-1:需要我处理的事 > 正在运行的事 > 其余;纯函数,单测覆盖 HomeAttentionSectionsTest)----

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
 *  - 其余:除开两组的剩余(主页「最近任务」仍走它自己的既有过滤,本组为
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

/** 运行中会话的归属 agent 集合(纯函数):agent 磁贴「运行中」小徽章的数据源。 */
internal fun agentIdsOfRunning(running: List<PThread>): Set<String> =
    running.map { threadAgentIdOf(it.agentId) }.toSet()

/**
 * 待审批卡元信息(纯函数):有审批对象=报数;
 * 只有 WAITING_APPROVAL 状态、没有审批对象(外部 agent 的 waiting 语义)=直说在等。
 */
internal fun pendingApprovalMeta(pendingCount: Int): String =
    if (pendingCount > 0) "$pendingCount 个审批待处理" else "等待你的处理"

/**
 * 运行中条元信息(纯函数):有真实起点 → 「运行 N 分钟」;缺席 → 「最近活动 · 相对时间」。
 *
 * 【时长从哪来】daemon 的 Thread.turn_started_unix_ms(2026-09-26 补齐:Status 转
 * RUNNING 时记 now;重启后仍 RUNNING 的会话记重启时刻 —— 真实开始不可追溯)。
 * 此前协议没有该字段,拿 updatedAt 冒充开始时刻就是造假,故降级为"最近活动"
 * 相对时间;现在字段缺席(旧 daemon/未知)时仍走旧展示,数据全真。
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
    return if (rel.isBlank()) "运行中" else "最近活动 · $rel"
}

// ---- agent 停用偏好生效链(管理 agent 抽屉 → 展示/创建入口;纯函数,单测覆盖 AgentDisablePreferenceTest)----

/**
 * agent 是否应出现在展示面(主页网格/协同行/最近任务,纯函数):
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
