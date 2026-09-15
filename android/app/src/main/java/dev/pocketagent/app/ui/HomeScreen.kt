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

    // 返回层级:从二级页面返回主页时自动展开侧边栏
    LaunchedEffect(openDrawerSignal) {
        if (openDrawerSignal > 0) showDrawer = true
    }
    var multiAgent by remember { mutableStateOf(AgentConnection.multiAgent) }
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
        } catch (e: Exception) {
            loadError = "首页数据加载失败：${e.message ?: "连接失败"}"
        }
    }

    val agents = remember {
        listOf(
            AgentInfo("codex", "Codex", "app-server", "Codex app-server 后端", R.drawable.agent_codex, true),
            AgentInfo("opencode", "OpenCode", "stdio", "HTTP + SSE 会话后端", R.drawable.agent_opencode, false),
            AgentInfo("claude-code", "Claude", "stdio", "stream-json stdio 后端", R.drawable.agent_claudecode, false),
            AgentInfo("pi", "Pi", "stdio", "RPC stdio 后端", R.drawable.agent_pi, false),
            AgentInfo("hermes", "Hermes", "ACP", "ACP stdio 后端", R.drawable.agent_hermes, false),
            AgentInfo("dsh", "DSH", "网页", "网页版 agent(浏览器接入)", R.drawable.agent_dsh, false),
            AgentInfo("zcode", "ZCode", "CLI", "ZCode CLI 后端", R.drawable.agent_zcode, false),
        )
    }

    // 左边缘右滑也能唤出侧边栏(问题 7),与左上角图标点按是同一个入口的两种手势
    Column(
        modifier.fillMaxSize().background(Pa.Bg).statusBarsPadding()
            .edgeSwipeToOpenDrawer { showDrawer = true },
    ) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            // 左上角:菜单入口(全 App 统一用汉堡图标;品牌 logo 只出现在侧边栏与启动页)
            IconActionButton(R.drawable.ic_lucide_menu, "菜单", { showDrawer = true }, bg = Pa.Card)
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
                                            Toast.makeText(ctx, "已切换到「${a.name}」", Toast.LENGTH_SHORT).show()
                                        } else {
                                            Toast.makeText(ctx, "「${a.name}」持续开发中,敬请期待", Toast.LENGTH_SHORT).show()
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

            // ---- 多agent协同:关=只显示在用的 agent;开=显示/添加协同 agents ----
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
                                if (multiAgent) "已开启:多个 agent 一起接任务" else "关闭中:仅使用选中的 agent",
                                fontSize = 11.sp, color = Pa.TextTertiary,
                            )
                        }
                        Switch(
                            checked = multiAgent,
                            onCheckedChange = {
                                multiAgent = it
                                AgentConnection.multiAgent = it
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
                                } else {
                                    Toast.makeText(ctx, "「${a.name}」持续开发中,敬请期待", Toast.LENGTH_SHORT).show()
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
                    Text("Codex", fontSize = 11.sp, color = Pa.TextTertiary)
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
                items(recent.take(6), key = { it.id }, span = { GridItemSpan(4) }) { t ->
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .pressable { onOpenThread(t.id) }
                            .background(Pa.Card, RoundedCornerShape(16.dp))
                            .padding(horizontal = 14.dp, vertical = 12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Image(
                            bitmap = agentBitmap(R.drawable.agent_codex),
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

    // 新建任务(与其他入口同款:文件式选工作区)
    if (showNewThread) {
        NewThreadSheet(
            onDismiss = { showNewThread = false },
            onCreate = { ws ->
                showNewThread = false
                scope.launch {
                    try {
                        val builder = CreateThreadRequest.newBuilder()
                        if (AgentConnection.selectedModel.isNotEmpty()) {
                            builder.setModelId(AgentConnection.selectedModel)
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
    if (showDrawer) {
        AppDrawer(
            currentTab = currentTab,
            hostTitle = host?.displayName ?: "未连接电脑",
            onClose = { showDrawer = false },
            onGoTab = { onGoTab(it) },
            onOpenTerminal = { onOpenTerminal() },
            onOpenSettings = { onOpenSettings() },
            onOpenTaskCenter = onOpenTaskCenter,
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
            if (a.available) a.method else "开发中",
            fontSize = 9.sp,
            color = if (a.available) Pa.Accent.copy(alpha = 0.75f) else Pa.TextTertiary.copy(alpha = 0.5f),
            maxLines = 1,
        )
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
                            if (!a.available) Toast.makeText(ctx, "「${a.name}」持续开发中", Toast.LENGTH_SHORT).show()
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
