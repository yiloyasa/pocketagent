package dev.pocketagent.app.ui

import android.widget.Toast
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CodexSettings
import pocketagent.v1.GetCodexSettingsRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.HostInfo
import pocketagent.v1.ListMcpServersRequest
import pocketagent.v1.SetCodexSettingsRequest

enum class SettingsScope(val id: String, val label: String) {
    GLOBAL("global", "全局"),
    CODEX("codex", "Codex"),
    OTHER("other", "其他 Agent"),
}

private val OTHER_AGENT_OPTIONS = listOf(
    "opencode" to "OpenCode",
    "claude" to "Claude",
    "pi" to "Pi",
    "hermes" to "Hermes",
)

/**
 * 设置 —— 分两大板块:
 *   全局:对所有 agent 与 APP 界面生效(连接 / 显示模式 / 外观与阅读)
 *   Codex:桌面端 agent 专属(权限与安全 / 模型与思考 / 账户 / MCP)
 *   更多:其他 agent 预留 / 关于
 * 行样式对标 Codex 桌面端:标题/描述 + 右侧控件。
 */
@Composable
fun SettingsScreen(onBack: () -> Unit, modifier: Modifier = Modifier) {
    val scope = androidx.compose.runtime.rememberCoroutineScope()
    val ctx = androidx.compose.ui.platform.LocalContext.current
    var addr by remember { mutableStateOf(AgentConnection.currentAddress()) }
    var token by remember { mutableStateOf(AgentConnection.token) }
    var settings by remember { mutableStateOf<CodexSettings?>(null) }
    var settingsLoading by remember { mutableStateOf(true) }
    var mcpList by remember { mutableStateOf(listOf<pocketagent.v1.McpServerStatus>()) }
    var host by remember { mutableStateOf<HostInfo?>(null) }
    var showApprovalPreview by remember { mutableStateOf(false) }
    var connectionSaving by remember { mutableStateOf(false) }
    var settingsError by remember { mutableStateOf<String?>(null) }
    var settingsFeedback by remember { mutableStateOf<String?>(null) }
    var selectedScope by remember { mutableStateOf(SettingsScope.GLOBAL) }
    var selectedOtherAgent by remember { mutableStateOf(OTHER_AGENT_OPTIONS.first().first) }
    val connectionError = AgentConnection.connError.collectAsState().value
    val connectionEpoch = AgentConnection.connectionEpoch.collectAsState().value

    fun showSettingsFeedback(message: String) {
        settingsFeedback = message
        if (message != "保存中…") {
            Toast.makeText(ctx, message, Toast.LENGTH_SHORT).show()
        }
        scope.launch {
            delay(1400)
            if (settingsFeedback == message) settingsFeedback = null
        }
    }

    LaunchedEffect(connectionEpoch) {
        try {
            withContext(Dispatchers.IO) {
                settings = AgentConnection.modelService()
                    .getCodexSettings(GetCodexSettingsRequest.newBuilder().build())
                settingsLoading = false
                host = AgentConnection.hostService().getHostInfo(GetHostInfoRequest.getDefaultInstance())
            }
        } catch (e: Exception) {
            settingsLoading = false
            settingsError = "无法加载 Codex 设置：${e.message ?: "连接失败"}"
        }
        try {
            withContext(Dispatchers.IO) {
                mcpList = AgentConnection.modelServiceSlow()
                    .listMcpServers(ListMcpServersRequest.newBuilder().build())
                    .serversList
            }
        } catch (e: Exception) {
            if (settingsError == null) settingsError = "无法加载 MCP 列表：${e.message ?: "连接失败"}"
        }
    }

    /**
     * 桌面端设置写入:审批/沙箱/默认模型/思考强度四件套。
     * daemon 每轮 turn/start 显式携带(codex 会话内真实生效),
     * 绝不改写桌面全局 config.toml。
     */
    fun writeCodex(approval: String? = null, sandbox: String? = null, model: String? = null, effort: String? = null) {
        val previous = settings ?: CodexSettings.getDefaultInstance()
        val optimistic = previous.toBuilder()
        approval?.let { optimistic.setApprovalPolicy(it) }
        sandbox?.let { optimistic.setSandbox(it) }
        model?.let { optimistic.setModel(it) }
        effort?.let { optimistic.setEffort(it) }
        val next = optimistic.build()
        settings = next
        settingsError = null
        showSettingsFeedback("保存中…")
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    settings = AgentConnection.modelService().setCodexSettings(
                        SetCodexSettingsRequest.newBuilder()
                            .setApprovalPolicy(next.approvalPolicy)
                            .setSandbox(next.sandbox)
                            .setModel(next.model)
                            .setEffort(next.effort)
                            .build())
                }
                showSettingsFeedback("已保存")
            } catch (e: Exception) {
                settings = previous
                settingsError = "设置保存失败：${e.message ?: "连接失败"}"
                showSettingsFeedback("保存失败")
            }
        }
    }

    Column(modifier.fillMaxSize().background(Pa.Bg).statusBarsPadding()) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconActionButton(R.drawable.ic_lucide_chevron_left, "返回", onBack, bg = Pa.Card)
            Spacer(Modifier.width(10.dp))
            Text("设置", Modifier.weight(1f), fontSize = 16.sp, color = Pa.TextPrimary)
        }

        Column(
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp)
                .navigationBarsPadding(),
            verticalArrangement = Arrangement.spacedBy(22.dp),
        ) {
            settingsError?.let { message ->
                Row(
                    Modifier
                        .fillMaxWidth()
                        .background(Pa.Card, RoundedCornerShape(12.dp))
                        .padding(horizontal = 12.dp, vertical = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(message, Modifier.weight(1f), fontSize = 12.sp, color = Pa.Red)
                    TextButton(onClick = {
                        settingsError = null
                        AgentConnection.connectionEpoch.value += 1
                    }) {
                        Text("重试", color = Pa.Accent)
                    }
                }
            }
            settingsFeedback?.let { message ->
                Text(
                    message,
                    Modifier.fillMaxWidth().padding(horizontal = 4.dp),
                    fontSize = 12.sp,
                    color = when {
                        message.contains("失败") -> Pa.Red
                        message.contains("保存中") -> Pa.Accent
                        else -> Pa.Green
                    },
                )
            }

            SettingsGroupHeader("设置范围")
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                SettingsScope.entries.forEach { scopeOption ->
                    val selected = selectedScope == scopeOption
                    Box(
                        Modifier
                            .weight(1f)
                            .pressable { selectedScope = scopeOption }
                            .background(
                                if (selected) Pa.Accent else Pa.Card,
                                RoundedCornerShape(12.dp),
                            )
                            .padding(vertical = 10.dp),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            scopeOption.label,
                            fontSize = 12.sp,
                            color = if (selected) Color.White else Pa.TextSecondary,
                            maxLines = 1,
                        )
                    }
                }
            }

            if (selectedScope == SettingsScope.GLOBAL) {
                // ==================== 全局板块 ====================
                SettingsGroupHeader("全局 · 对所有 agent 与界面生效")

            // ---- 连接 ----
            SectionBlock(
                icon = R.drawable.ic_lucide_globe,
                title = "连接",
                desc = "daemon 地址与配对(外网用 Tailscale IP 直连)",
            ) {
                val connState = AgentConnection.connState.collectAsState().value
                val transport = AgentConnection.connTransport.collectAsState().value
                val connAddr = AgentConnection.connAddress.collectAsState().value
                SettingRow(
                    title = when (connState) {
                        "ok" -> "已连接 · $transport"
                        "down" -> "未连接 · 自动重连中"
                        else -> "检查中…"
                    },
                    desc = if (connState == "ok") connAddr else "每 5 秒自动重试 · USB → 局域网 → Tailscale 回退",
                    divider = true,
                ) {}
                connectionError?.let { diagnostic ->
                    Text(
                        "${diagnostic.message}\n地址：${diagnostic.address}",
                        Modifier
                            .fillMaxWidth()
                            .background(Pa.CardHigh, RoundedCornerShape(10.dp))
                            .padding(10.dp),
                        fontSize = 11.sp,
                        color = Pa.Red,
                    )
                }
                OutlinedTextField(
                    addr, { addr = it },
                    label = { Text("地址 (host:port)") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(12.dp),
                )
                Spacer(Modifier.height(10.dp))
                OutlinedTextField(
                    token, { token = it },
                    label = { Text("配对 token(未配对留空)") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(12.dp),
                    visualTransformation = PasswordVisualTransformation(),
                )
                Spacer(Modifier.height(12.dp))
                Box(
                    Modifier
                        .fillMaxWidth()
                        .pressable(enabled = !connectionSaving) {
                            connectionSaving = true
                            settingsError = null
                            scope.launch {
                                val connected = try {
                                    withContext(Dispatchers.IO) {
                                        AgentConnection.setCredentials(addr, token)
                                        AgentConnection.connectWithFallback()
                                    }
                                } catch (e: Exception) {
                                    settingsError = "连接失败：${e.message ?: "未知错误"}"
                                    null
                                }
                                connectionSaving = false
                                if (connected != null) {
                                    Toast.makeText(
                                        ctx,
                                        "连接成功 · ${AgentConnection.connTransport.value}",
                                        Toast.LENGTH_SHORT,
                                    ).show()
                                } else {
                                    settingsError = AgentConnection.connError.value?.message
                                        ?: "连接失败"
                                }
                            }
                        }
                        .background(Pa.Accent, RoundedCornerShape(14.dp))
                        .padding(vertical = 12.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        if (connectionSaving) "连接中…" else "保存并测试连接",
                        fontSize = 15.sp,
                        color = Color.White,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
            }

            // ---- 显示模式 ----
            SectionBlock(
                icon = R.drawable.ic_lucide_monitor,
                title = "显示模式",
                desc = "跟随系统 / 浅色 / 深色",
            ) {
                Row(
                    Modifier.fillMaxWidth().padding(top = 6.dp),
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    listOf("system" to "跟随系统", "light" to "浅色", "dark" to "深色").forEach { (id, label) ->
                        val sel = AgentConnection.themeMode == id
                        Box(
                            Modifier
                                .weight(1f)
                                .pressable {
                                    AgentConnection.themeMode = id
                                    dev.pocketagent.app.ui.theme.AgentTheme.mode = id
                                    showSettingsFeedback("显示模式已保存")
                                }
                                .background(if (sel) Pa.Accent else Pa.CardHigh, RoundedCornerShape(12.dp))
                                .padding(vertical = 10.dp),
                            contentAlignment = Alignment.Center,
                        ) {
                            Text(label, fontSize = 13.sp, color = if (sel) Color.White else Pa.TextSecondary)
                        }
                    }
                }
            }

            // ---- 外观与阅读 ----
            SectionBlock(
                icon = R.drawable.ic_lucide_file_text,
                title = "外观与阅读",
                desc = "界面与思考内容的字号",
            ) {
                SettingRow(title = "界面文字大小", desc = "聊天与列表文字") {}
                ChoicePills(
                    options = listOf("0.9" to "小", "1.0" to "标准", "1.15" to "大"),
                    selected = AgentConnection.uiTextScale.toString(),
                    onPick = {
                        v ->
                        AgentConnection.setTextScale(v.toFloat(), AgentConnection.thinkTextScale)
                        showSettingsFeedback("界面字号已保存")
                    },
                )
                SettingRow(title = "思考显示大小", desc = "💭 思考条文字") {}
                ChoicePills(
                    options = listOf("0.85" to "小", "1.0" to "标准", "1.25" to "大"),
                    selected = AgentConnection.thinkTextScale.toString(),
                    onPick = {
                        v ->
                        AgentConnection.setTextScale(AgentConnection.uiTextScale, v.toFloat())
                        showSettingsFeedback("思考字号已保存")
                    },
                )
                SettingRow(title = "工具详情", desc = "控制普通会话中的工具卡片，审批弹窗始终独立折叠") {}
                ChoicePills(
                    options = listOf("false" to "折叠", "true" to "展开"),
                    selected = AgentConnection.toolDetailsExpanded.toString(),
                    onPick = {
                        AgentConnection.toolDetailsExpanded = it.toBoolean()
                        showSettingsFeedback("工具详情设置已保存")
                    },
                )
            }
            }

            if (selectedScope == SettingsScope.CODEX) {
                // ==================== Codex 板块 ====================
                SettingsGroupHeader("Codex · 桌面端 agent 专属")

            // ---- 权限与安全 ----
            SectionBlock(
                icon = R.drawable.ic_lucide_shield,
                title = "权限与安全",
                desc = "命令执行审批与文件访问范围(随每轮任务下发生效)",
            ) {
                SettingRow(
                    title = "审批策略",
                    desc = "agent 运行命令/写文件前是否先征求批准",
                ) {}
                ChoicePills(
                    options = listOf(
                        "untrusted" to "仅信任",
                        "on-request" to "按需询问",
                        "never" to "完全访问",
                    ),
                    selected = settings?.approvalPolicy,
                    onPick = { writeCodex(approval = it) },
                    enabled = !settingsLoading,
                )
                SettingRow(
                    title = "沙箱模式",
                    desc = "限制 agent 可触碰的文件与网络",
                ) {}
                ChoicePills(
                    options = listOf(
                        "read-only" to "只读",
                        "workspace-write" to "工作区可写",
                        "danger-full-access" to "完全访问",
                    ),
                    selected = settings?.sandbox,
                    onPick = { writeCodex(sandbox = it) },
                    enabled = !settingsLoading,
                )
                // 审批弹窗预览:提前认识这个界面怎么用
                SettingRow(
                    title = "审批弹窗预览",
                    desc = "看看 agent 请求执行命令时的弹窗长什么样、怎么操作",
                ) {}
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Button(
                        onClick = { showApprovalPreview = true },
                        shape = RoundedCornerShape(12.dp),
                        colors = androidx.compose.material3.ButtonDefaults.buttonColors(containerColor = Pa.Card),
                        contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 16.dp, vertical = 8.dp),
                        modifier = Modifier.weight(1f),
                    ) { Text("预览审批弹窗", fontSize = 13.sp, color = Pa.TextPrimary) }
                }
            }

            // ---- 模型与思考 ----
            SectionBlock(
                icon = R.drawable.ic_lucide_cpu,
                title = "模型与思考",
                desc = "默认模型与思考强度(会话内可临时切换)",
            ) {
                SettingRow(
                    title = "默认模型",
                    desc = settings?.model?.ifEmpty { "跟随桌面配置" } ?: "跟随桌面配置",
                ) {}
                SettingRow(title = "思考强度", desc = "推理投入,越高越慢越细") {}
                ChoicePills(
                    options = listOf(
                        "low" to "低", "medium" to "中", "high" to "高", "xhigh" to "极高",
                    ),
                    selected = settings?.effort,
                    onPick = { writeCodex(effort = it) },
                    enabled = !settingsLoading,
                )
            }

            // ---- Codex 账户 ----
            SectionBlock(
                icon = R.drawable.ic_lucide_user,
                title = "Codex 账户",
                desc = "桌面端登录的 ChatGPT 账号",
            ) {
                SettingRow(
                    title = host?.displayName ?: "…",
                    desc = "已登录 · 模型与用量走桌面端账号",
                ) {}
            }

            // ---- 集成 · MCP ----
            SectionBlock(
                icon = R.drawable.ic_lucide_server,
                title = "集成 · MCP 服务器",
                desc = "桌面端 codex 已配置的工具服务器",
            ) {
                if (mcpList.isEmpty()) {
                    Text(
                        if (settingsLoading) "加载中…" else "未获取到 MCP 列表",
                        fontSize = 12.sp, color = Pa.TextTertiary,
                        modifier = Modifier.padding(vertical = 6.dp),
                    )
                } else {
                    mcpList.forEach { m ->
                        SettingRow(title = m.name, desc = "${m.tools} 个工具", divider = true) {}
                    }
                }
            }
            }

            if (selectedScope == SettingsScope.OTHER) {
                // ==================== 其他 Agent 板块 ====================
                SettingsGroupHeader("其他 Agent · 独立配置入口")

            SectionBlock(
                icon = R.drawable.ic_lucide_info,
                title = "关于",
                desc = "设备与工作区",
            ) {
                SettingRow(title = host?.displayName ?: "…", desc = "电脑 · ${host?.os ?: ""}/${host?.arch ?: ""}", divider = true) {}
                SettingRow(title = "PocketAgent", desc = "v0.1.0 · 复刻并超越 Mobile Codex") {}
            }

            SectionBlock(
                icon = R.drawable.ic_lucide_cpu,
                title = "其他 agent",
                desc = "OpenCode / Claude / Pi / Hermes / DSH / ZCode",
            ) {
                SettingRow(
                    title = "当前 Agent",
                    desc = "选择后，该 Agent 的设置不会混入 Codex 设置",
                ) {}
                ChoicePills(
                    options = OTHER_AGENT_OPTIONS,
                    selected = selectedOtherAgent,
                    onPick = { selectedOtherAgent = it },
                )
                Text(
                    "${OTHER_AGENT_OPTIONS.firstOrNull { it.first == selectedOtherAgent }?.second ?: "Agent"} " +
                        "的模型、权限和工具设置将在对应接入完成后显示。",
                    fontSize = 12.sp, color = Pa.TextTertiary,
                    modifier = Modifier.padding(vertical = 4.dp),
                )
            }
            }
            Spacer(Modifier.height(12.dp))
        }
    }

    if (showApprovalPreview) {
        ApprovalPreviewSheet(onDismiss = { showApprovalPreview = false })
    }
}

/** 设置页分组大标题。 */
@Composable
fun SettingsGroupHeader(title: String) {
    Column(Modifier.fillMaxWidth()) {
        Spacer(Modifier.height(4.dp))
        Text(
            title,
            fontSize = 12.sp,
            color = Pa.Accent,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(start = 4.dp),
        )
    }
}

/**
 * 审批弹窗预览:与聊天中真实弹出的审批卡同款样式 + 使用说明。
 * 真实审批:agent 要执行危险命令时弹出 → 允许=执行 / 拒绝=不执行但继续对话;
 * 不操作时任务结束前自动拒绝;断网时 daemon 自动拒绝并让 agent 继续作答。
 */
@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun ApprovalPreviewSheet(onDismiss: () -> Unit) {
    val state = androidx.compose.material3.rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var expanded by androidx.compose.runtime.remember { androidx.compose.runtime.mutableStateOf(false) }
    androidx.compose.material3.ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = state,
        containerColor = Pa.CardHigh,
        shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
    ) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 24.dp)) {
            Row(
                Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("允许执行?", Modifier.weight(1f), fontSize = 19.sp, color = Pa.TextPrimary)
                Text(
                    if (expanded) "收起" else "展开",
                    fontSize = 11.sp,
                    color = Pa.Accent,
                )
                IconActionButton(
                    if (expanded) R.drawable.ic_lucide_chevron_left else R.drawable.ic_lucide_chevron_right,
                    if (expanded) "收起详情" else "展开详情",
                    { expanded = !expanded },
                    bg = Color.Transparent,
                    size = 36.dp,
                    iconSize = 18.dp,
                )
            }
            Text(
                "agent 请求执行以下命令(这是示例,不会真的执行)",
                fontSize = 13.sp, color = Pa.TextSecondary,
                modifier = Modifier.padding(top = 4.dp),
            )
            if (expanded) {
                Text(
                    "{\"command\": \"Remove-Item C:\\\\project\\\\old-build -Recurse\", \"cwd\": \"C:\\\\project\"}",
                    fontSize = 12.sp,
                    color = Pa.TextPrimary,
                    fontFamily = androidx.compose.ui.text.font.FontFamily.Monospace,
                    modifier = Modifier
                        .fillMaxWidth()
                        .background(Pa.Bg, RoundedCornerShape(12.dp))
                        .padding(12.dp)
                        .padding(vertical = 4.dp),
                )
            }
            Text(
                "怎么用:点【允许】= 电脑上真实执行这条命令;点【拒绝】= 不执行," +
                    "agent 会继续对话并说明;放着不管 = 任务结束前自动拒绝。" +
                    "断网收不到弹窗时,daemon 会自动拒绝并让 agent 继续作答,不会卡死任务。",
                fontSize = 12.sp, color = Pa.TextTertiary,
                modifier = Modifier.padding(vertical = 10.dp),
            )
            Row(Modifier.fillMaxWidth().padding(bottom = 20.dp)) {
                Box(
                    Modifier
                        .weight(1f)
                        .pressable { onDismiss() }
                        .background(Pa.Hover, RoundedCornerShape(14.dp))
                        .padding(vertical = 12.dp),
                    contentAlignment = Alignment.Center,
                ) { Text("拒绝(示例)", fontSize = 15.sp, color = Pa.TextPrimary) }
                Spacer(Modifier.width(12.dp))
                Box(
                    Modifier
                        .weight(1f)
                        .pressable { onDismiss() }
                        .background(Pa.Accent, RoundedCornerShape(14.dp))
                        .padding(vertical = 12.dp),
                    contentAlignment = Alignment.Center,
                ) { Text("允许(示例)", fontSize = 15.sp, color = Color.White, fontWeight = FontWeight.SemiBold) }
            }
        }
    }
}

/** 分区:图标+标题+描述 的分区头,内部一张行卡。 */
@Composable
fun SectionBlock(
    icon: Int,
    title: String,
    desc: String,
    content: @Composable androidx.compose.foundation.layout.ColumnScope.() -> Unit,
) {
    // 整体包一层 Column:SectionBlock 作为父滚动 Column 的单个子项,
    // 否则父级 spacedBy 会把 头/卡 之间也撑开 22dp
    Column {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                Modifier.size(34.dp).background(Pa.Card, RoundedCornerShape(11.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(painterResource(icon), null, tint = Pa.Accent, modifier = Modifier.size(17.dp))
            }
            Spacer(Modifier.width(10.dp))
            Column {
                Text(title, fontSize = 15.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold)
                Text(desc, fontSize = 11.sp, color = Pa.TextTertiary)
            }
        }
        Spacer(Modifier.height(10.dp))
        Column(
            Modifier
                .fillMaxWidth()
                .background(Pa.Card, RoundedCornerShape(18.dp))
                .padding(horizontal = 14.dp, vertical = 6.dp),
        ) {
            content()
        }
    }
}

/** 桌面端式设置行:左标题/描述,右控件,行间细分隔线。 */
@Composable
fun SettingRow(
    title: String,
    desc: String,
    divider: Boolean = false,
    control: @Composable androidx.compose.foundation.layout.RowScope.() -> Unit = {},
) {
    Column {
        Spacer(Modifier.height(8.dp))
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(title, fontSize = 13.5.sp, color = Pa.TextPrimary, maxLines = 1)
                if (desc.isNotEmpty()) {
                    Text(desc, fontSize = 11.sp, color = Pa.TextTertiary)
                }
            }
            control()
        }
        if (divider) {
            Spacer(Modifier.height(8.dp))
            Box(Modifier.fillMaxWidth().height(1.dp).background(Pa.Separator))
        } else {
            Spacer(Modifier.height(8.dp))
        }
    }
}

/** 芯片选项组(选中=蓝底白字)。selected 与 option id 直接比较。 */
@Composable
fun ChoicePills(
    options: List<Pair<String, String>>,
    selected: String?,
    onPick: (String) -> Unit,
    enabled: Boolean = true,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .horizontalScroll(rememberScrollState())
            .padding(top = 6.dp, bottom = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        options.forEach { (id, label) ->
            val sel = selected == id
            Box(
                Modifier
                    .pressable(enabled = enabled) { onPick(id) }
                    .background(if (sel) Pa.Accent else Pa.CardHigh, RoundedCornerShape(50))
                    .padding(horizontal = 13.dp, vertical = 7.dp),
            ) {
                Text(label, fontSize = 12.sp, color = if (sel) Color.White else Pa.TextSecondary)
            }
        }
    }
}

/** 字号选项胶囊(选中=蓝底)。 */
@Composable
fun ScaleOption(label: String, value: Float, current: Float, onPick: (Float) -> Unit) {
    val selected = kotlin.math.abs(current - value) < 0.01f
    Box(
        Modifier
            .pressable { onPick(value) }
            .background(if (selected) Pa.Accent else Pa.CardHigh, RoundedCornerShape(50))
            .padding(horizontal = 16.dp, vertical = 8.dp),
    ) {
        Text(label, fontSize = 13.sp, color = if (selected) Color.White else Pa.TextSecondary)
    }
}
