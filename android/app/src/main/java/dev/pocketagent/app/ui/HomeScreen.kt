package dev.pocketagent.app.ui

import android.widget.Toast
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
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
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
import dev.pocketagent.app.ui.theme.Pa
import androidx.core.graphics.drawable.toBitmap
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CreateThreadRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.HostInfo
import pocketagent.v1.ListThreadsRequest
import pocketagent.v1.Thread as PThread

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
 * 主页 —— 按用户草图重排:
 * [Pa 图标=菜单(左上)] [设置(右上)] → 连接状态卡 → agents 板块 →
 * 多agent协同(开关:关=只显示在用 agent,开=显示已加入协同的 agents)→ 最近任务。
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
    // 【多agent协同 = 开发中】(用户实锤:开关可拨但功能未实现 = 假可用)。
    // AgentConnection.multiAgent 不再读也不再写:功能没实现,持久化"已开启"
    // 只会维持一个假开关。展示恒为关,点按只给"开发中"提示(见下方 Switch)。
    var multiAgent by remember { mutableStateOf(false) }
    var reloadKey by remember { mutableStateOf(0) }
    var loadError by remember { mutableStateOf<String?>(null) }
    var showNewThread by remember { mutableStateOf(false) }
    // 自动重连恢复后(connectionEpoch 变化)自动刷新,拔 USB 再回来无需手点重试
    val reconnectTick = AgentConnection.connectionEpoch.collectAsState().value

    LaunchedEffect(reloadKey, reconnectTick) {
        try {
            withContext(Dispatchers.IO) {
                // 多候选回退:当前地址 → USB 反向 → Tailscale
                host = AgentConnection.connectWithFallback()
                    ?: AgentConnection.hostService().getHostInfo(GetHostInfoRequest.getDefaultInstance())
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
    // 才点亮;检测不到维持灰显,daemon 部署后自然点亮)。host 变化时重建。
    val agents = remember(host) { buildAgents(host?.capabilitiesList.orEmpty()) }

    // 左边缘右滑也能唤出侧边栏(问题 7),与左上角图标点按是同一个入口的两种手势
    Column(
        modifier.fillMaxSize().background(Pa.Bg).statusBarsPadding()
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
            // 左上角:菜单入口(全 App 统一用汉堡图标;品牌 logo 只出现在侧边栏与启动页)
            IconActionButton(R.drawable.ic_lucide_menu, "菜单", { drawerCtrl.open() }, bg = Pa.Card)
            // 右上角:设置
            IconActionButton(R.drawable.ic_lucide_settings, "设置", onOpenSettings, bg = Pa.Card)
        }

        LazyVerticalGrid(
            columns = GridCells.Fixed(4),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
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
                            .background(Pa.Card, RoundedCornerShape(16.dp))
                            .padding(horizontal = 14.dp, vertical = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(message, Modifier.weight(1f), fontSize = 12.sp, color = Pa.Red, maxLines = 2)
                        Text("重试", fontSize = 12.sp, color = Pa.Accent)
                    }
                }
            }

            // ---- agent 板块:整块卡片包住网格 ----
            item(span = { GridItemSpan(4) }) {
                Column(
                    Modifier
                        .fillMaxWidth()
                        .background(Pa.Card, RoundedCornerShape(22.dp))
                        .padding(14.dp),
                ) {
                    // 磁贴:真实图标 + 灰阶对比(不可用),尺寸收小
                    androidx.compose.foundation.layout.Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
                        agents.chunked(4).forEach { rowAgents ->
                            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly) {
                                rowAgents.forEach { a ->
                                    AgentTile(a) {
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

            // ---- 多agent协同:开发中。开关恒为关,点按只提示;行内仍显示唯一选中 agent ----
            item(span = { GridItemSpan(4) }) {
                Column(
                    Modifier
                        .fillMaxWidth()
                        .background(Pa.Card, RoundedCornerShape(22.dp))
                        .padding(14.dp),
                ) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text("多agent协同", fontSize = 15.sp, color = Pa.TextPrimary)
                            Text(
                                if (multiAgent) "已开启:多个 agent 一起接任务" else MULTI_AGENT_HINT_DEV,
                                fontSize = 11.sp, color = Pa.TextTertiary,
                            )
                        }
                        Switch(
                            checked = multiAgent,
                            onCheckedChange = { request ->
                                // 【开发中闸门】打开请求一律不生效:状态回弹保持关 + 人话提示,
                                // 绝不写 AgentConnection.multiAgent 假装可用(纯函数可单测)。
                                val (next, hint) = multiAgentToggleResult(request)
                                multiAgent = next
                                hint?.let {
                                    Toast.makeText(ctx, it, Toast.LENGTH_SHORT).show()
                                }
                            },
                            colors = SwitchDefaults.colors(
                                checkedTrackColor = Pa.Accent,
                                uncheckedTrackColor = Pa.Hover,
                                uncheckedBorderColor = Color.Transparent,
                            ),
                        )
                    }
                    Spacer(Modifier.height(10.dp))
                    // 协同行
                    Row(
                        Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        val shown = if (multiAgent) agents else agents.filter { it.id == AgentConnection.selectedAgent }
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
                            // 添加位:把更多 agent 加进协同
                            Column(
                                Modifier
                                    .width(72.dp)
                                    .pressable {
                                        Toast.makeText(ctx, "更多 agent 接入后可在此添加", Toast.LENGTH_SHORT).show()
                                    },
                                horizontalAlignment = Alignment.CenterHorizontally,
                            ) {
                                Box(
                                    Modifier.size(56.dp).background(Pa.Hover, RoundedCornerShape(15.dp)),
                                    contentAlignment = Alignment.Center,
                                ) {
                                    Icon(
                                        painterResource(R.drawable.ic_lucide_plus), null,
                                        tint = Pa.TextSecondary, modifier = Modifier.size(22.dp),
                                    )
                                }
                                Text("添加", Modifier.padding(top = 6.dp), fontSize = 11.5.sp, color = Pa.TextSecondary)
                            }
                        }
                    }
                }
            }

            // 新建任务入口并入 agent 板块行尾?保持独立小行
            item(span = { GridItemSpan(4) }) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable { showNewThread = true }
                        .background(Pa.Card, RoundedCornerShape(18.dp))
                        .padding(14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Box(
                        Modifier.size(40.dp).background(Pa.AccentDim, RoundedCornerShape(12.dp)),
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(painterResource(R.drawable.ic_lucide_plus), null, tint = Pa.Accent, modifier = Modifier.size(20.dp))
                    }
                    Spacer(Modifier.width(12.dp))
                    Text("新建任务", Modifier.weight(1f), fontSize = 15.sp, color = Pa.TextPrimary)
                    // 右侧 agent 名标签:跟随唯一选中源 AgentConnection.selectedAgent 实时显示
                    // (设置页「切换 Agent」/ 主页网格点选都写它),不再硬编码 "Codex"。
                    Text(selectedAgentLabel(AgentConnection.selectedAgent), fontSize = 11.sp, color = Pa.TextTertiary)
                }
            }

            // ---- 电脑预览:浏览器里一键看电脑上正跑着的 dev server(P2)----
            // 复用既有 onOpenBrowser 入口打开内置浏览器,浏览器顶栏的"电脑预览"
            // 按钮列出 daemon 探到的端口(3000/5173/8000…),点一项直接加载。
            item(span = { GridItemSpan(4) }) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable { onOpenBrowser() }
                        .background(Pa.Card, RoundedCornerShape(18.dp))
                        .padding(14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Box(
                        Modifier.size(40.dp).background(Pa.AccentDim, RoundedCornerShape(12.dp)),
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(painterResource(R.drawable.ic_lucide_monitor), null, tint = Pa.Accent, modifier = Modifier.size(20.dp))
                    }
                    Spacer(Modifier.width(12.dp))
                    Text("电脑预览", Modifier.weight(1f), fontSize = 15.sp, color = Pa.TextPrimary)
                    Text("dev server", fontSize = 11.sp, color = Pa.TextTertiary)
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
                    Text("全部 →", fontSize = 13.sp, color = Pa.TextSecondary)
                }
            }
            if (recent.isEmpty()) {
                item(span = { GridItemSpan(4) }) {
                    Column(
                        Modifier.fillMaxWidth().background(Pa.Card, RoundedCornerShape(20.dp)).padding(vertical = 36.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                    ) {
                        Icon(painterResource(R.drawable.ic_lucide_list_checks), null, tint = Pa.TextTertiary, modifier = Modifier.size(36.dp))
                        Text("连接电脑后,任务会出现在这里", Modifier.padding(top = 12.dp), fontSize = 13.sp, color = Pa.TextSecondary)
                    }
                }
            } else {
                items(recent.filter { it.agentId.ifBlank { "codex" } == AgentConnection.selectedAgent }.take(6), key = { it.id }, span = { GridItemSpan(4) }) { t ->
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .pressable { onOpenThread(t.id) }
                            .background(Pa.Card, RoundedCornerShape(16.dp))
                            .padding(horizontal = 14.dp, vertical = 12.dp),
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
                            Text(
                                "${t.messageCount} 条消息",
                                fontSize = 11.sp, color = Pa.TextTertiary,
                            )
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
            capabilities = host?.capabilitiesList.orEmpty(),
            onDismiss = { guideAgent = null },
        )
    }

            // 新建任务(与其他入口同款:文件式选工作区)
            if (showNewThread) {
                NewThreadSheet(
                    onDismiss = { showNewThread = false },
                    onCreate = { ws ->
                        showNewThread = false
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

/** agent 磁贴:官方品牌图标,可用=全彩+白名+连接方式;不可用=暗态。 */
@Composable
fun AgentTile(a: AgentInfo, onClick: () -> Unit) {
    Column(
        Modifier.pressable { onClick() }.width(72.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Image(
            bitmap = agentBitmap(a.iconRes),
            contentDescription = a.name,
            modifier = Modifier
                .size(56.dp)
                .then(if (a.available) Modifier else Modifier.alpha(0.22f)),
            contentScale = ContentScale.Crop,
            colorFilter = if (a.available) null else grayscale(),
        )
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
 * 风格仿 ManageAgentsSheet(同款圆角/拖柄/关闭按钮)。
 */
@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
internal fun AgentGuideSheet(agent: AgentInfo, capabilities: List<String>, onDismiss: () -> Unit) {
    // 已装版本(daemon 报 "agent:<id>:<version>" 时非 null)→ 状态行区分两态
    val installedVersion = resolveAgentInstalledVersion(agent.id, capabilities)
    val guide = agentGuideCopy(agent.id)
    val state = androidx.compose.material3.rememberModalBottomSheetState(skipPartiallyExpanded = true)
    androidx.compose.material3.ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = state,
        containerColor = Pa.CardHigh,
        shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
        dragHandle = {
            Box(Modifier.padding(top = 10.dp).width(40.dp).height(4.dp).background(Pa.Hover, androidx.compose.foundation.shape.CircleShape))
        },
    ) {
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

@Composable
private fun ConnectionCardCompact(host: HostInfo?, onRetry: () -> Unit = {}, modifier: Modifier = Modifier) {
    val connState = AgentConnection.connState.collectAsState().value
    val transport = AgentConnection.connTransport.collectAsState().value
    val diagnostic = AgentConnection.connError.collectAsState().value
    Row(
        modifier
            .fillMaxWidth()
            .pressable { onRetry() }
            .background(Pa.Card, RoundedCornerShape(20.dp))
            .padding(14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier.size(42.dp).background(Pa.AccentDim, RoundedCornerShape(12.dp)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(painterResource(R.drawable.ic_lucide_monitor), null, tint = Pa.Accent, modifier = Modifier.size(20.dp))
        }
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
 */
@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun ManageAgentsSheet(agents: List<AgentInfo>, onDismiss: () -> Unit) {
    val ctx = LocalContext.current
    val state = androidx.compose.material3.rememberModalBottomSheetState(skipPartiallyExpanded = true)
    androidx.compose.material3.ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = state,
        containerColor = Pa.CardHigh,
        shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
        dragHandle = {
            Box(Modifier.padding(top = 10.dp).width(40.dp).height(4.dp).background(Pa.Hover, androidx.compose.foundation.shape.CircleShape))
        },
    ) {
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
        Column(Modifier.padding(horizontal = 16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
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
                    }
                    Switch(
                        checked = a.available,
                        onCheckedChange = {
                            if (!a.available) Toast.makeText(ctx, "「${a.name}」${unavailableLabel(a.id)}", Toast.LENGTH_SHORT).show()
                        },
                        enabled = a.available,
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
        }
        Spacer(Modifier.height(30.dp))
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

// ---- 多agent协同开关:开发中闸门(纯函数,单测覆盖 MultiAgentToggleTest)----

/** "开发中"提示文案(主页开关副标题与点按 Toast 共用同一句,避免两处口径漂移)。 */
internal const val MULTI_AGENT_HINT_DEV = "开发中:多 agent 协同尚未开放"

/**
 * 多agent协同开关的点按结果(纯函数):功能未实现,任何"打开"请求都不生效。
 * @return (下一状态, 待弹提示) —— 下一状态恒为关;"打开"请求附带"开发中"提示,
 *         "关闭"请求静默(本来就关着,无需打扰)。
 */
internal fun multiAgentToggleResult(requestOn: Boolean): Pair<Boolean, String?> =
    if (requestOn) false to MULTI_AGENT_HINT_DEV else false to null
