package dev.pocketagent.app.ui

import android.widget.Toast
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.SplitButton
import dev.pocketagent.app.ui.component.SplitOption
import dev.pocketagent.app.ui.theme.AgentTheme
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
import pocketagent.v1.ListModelsRequest
import pocketagent.v1.McpServerStatus
import pocketagent.v1.ModelInfo
import pocketagent.v1.SetCodexSettingsRequest

/**
 * 设置范围(三个板块,遵循回退原则):
 *   GLOBAL  —— 对全局所有 Agent 与 APP 界面生效(连接 / 显示 / 阅读)
 *   RUNNING —— 仅对"当前正在运行的 agent"生效,含它的专属设置
 *   OTHER   —— 其他 agent:选中某个即切换到它,并自动跳到它的专属设置页
 */
enum class SettingsScope(val id: String, val label: String) {
    GLOBAL("global", "全局"),
    RUNNING("running", "当前 Agent"),
    OTHER("other", "其他 Agent"),
}

private val OTHER_AGENT_OPTIONS = listOf(
    "opencode" to "OpenCode",
    "claude" to "Claude",
    "pi" to "Pi",
    "hermes" to "Hermes",
)

/** agent id → 展示名(第二板块标题、其他 agent 列表共用)。 */
private val AGENT_DISPLAY = (listOf("codex" to "Codex") + OTHER_AGENT_OPTIONS).toMap()

private fun agentDisplay(id: String): String = AGENT_DISPLAY[id] ?: id.ifBlank { "Agent" }

/** 设置页的分区定义:id 用作 Lazy key,content 为分区内容。 */
private class SettingsSection(
    val id: String,
    val content: @Composable () -> Unit,
)

/**
 * 设置页(三板块精简版)。
 *
 * 【这一版的结构】
 *  1. 页头大标题 → 范围分段(全局 / 当前 Agent / 其他 Agent) → 直接是分区卡片。
 *     上一版夹在中间的"快捷导航 chips"已移除(与分段功能重叠、实际没人用)。
 *  2. **所有多选项设置统一为 SplitButton**:主按钮一键切换/执行默认操作,
 *     下拉箭头直接指定某一项;当前值以 Accent 显示在按钮右侧,替代原来的说明文字。
 *  3. 设置项不再写解释性小字(只保留"界面文字大小"的实时预览),页面更工整。
 *
 * 【逻辑未变】审批/沙箱/模型/思考强度照样走 writeCodex 乐观更新 + 失败回滚;
 * 连接凭证照样走 setCredentials + connectWithFallback。
 */
@Composable
fun SettingsScreen(onBack: () -> Unit, modifier: Modifier = Modifier) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var addr by remember { mutableStateOf(AgentConnection.configuredAddress()) }
    var token by remember { mutableStateOf(AgentConnection.token) }
    var settings by remember { mutableStateOf<CodexSettings?>(null) }
    var settingsLoading by remember { mutableStateOf(true) }
    var mcpList by remember { mutableStateOf(listOf<McpServerStatus>()) }
    var host by remember { mutableStateOf<HostInfo?>(null) }
    var showApprovalPreview by remember { mutableStateOf(false) }
    var connectionSaving by remember { mutableStateOf(false) }
    var settingsError by remember { mutableStateOf<String?>(null) }
    var settingsFeedback by remember { mutableStateOf<String?>(null) }
    var selectedScope by remember { mutableStateOf(SettingsScope.GLOBAL) }
    var selectedOtherAgent by remember { mutableStateOf(OTHER_AGENT_OPTIONS.first().first) }
    // 默认模型选择器(完整选择弹窗,主按钮触发)
    var showModelPicker by remember { mutableStateOf(false) }
    var modelChoices by remember { mutableStateOf(listOf<ModelInfo>()) }
    // 第二板块始终跟随"当前运行的 agent"
    val runningAgent = AgentConnection.selectedAgent
    val connectionError = AgentConnection.connError.collectAsState().value
    val connectionEpoch = AgentConnection.connectionEpoch.collectAsState().value
    val listState = rememberLazyListState()

    // 模型候选(默认模型选择器用)
    LaunchedEffect(connectionEpoch) {
        try {
            withContext(Dispatchers.IO) {
                modelChoices = AgentConnection.modelService()
                    .listModels(ListModelsRequest.newBuilder().build())
                    .modelsList
            }
        } catch (_: Exception) {
            // 连不上时保持为空,选择器会给出提示
        }
    }

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

    /** 连接 · 主操作:保存凭证并立即测试。 */
    fun saveAndTestConnection() {
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
                settingsError = AgentConnection.connError.value?.message ?: "连接失败"
            }
        }
    }

    /** 连接 · 下拉:仅保存凭证,不立即测试,交给自动重连接管。 */
    fun saveCredentialsOnly() {
        scope.launch {
            try {
                withContext(Dispatchers.IO) { AgentConnection.setCredentials(addr, token) }
                Toast.makeText(ctx, "凭证已保存,自动重连将接管", Toast.LENGTH_SHORT).show()
            } catch (e: Exception) {
                settingsError = "保存失败：${e.message ?: "未知错误"}"
            }
        }
    }

    /** 连接 · 下拉:重扫候选地址(不改动已保存的凭证)。 */
    fun rescanCandidates() {
        connectionSaving = true
        settingsError = null
        scope.launch {
            val connected = try {
                withContext(Dispatchers.IO) { AgentConnection.connectWithFallback() }
            } catch (e: Exception) {
                settingsError = "重扫失败：${e.message ?: "未知错误"}"
                null
            }
            connectionSaving = false
            if (connected != null) {
                Toast.makeText(ctx, "已连接 · ${AgentConnection.connTransport.value}", Toast.LENGTH_SHORT).show()
            } else {
                settingsError = AgentConnection.connError.value?.message ?: "所有候选地址均不可达"
            }
        }
    }

    /** 显示模式应用(循环切换与下拉选择共用)。 */
    fun applyThemeMode(id: String) {
        AgentConnection.themeMode = id
        AgentTheme.mode = id
        showSettingsFeedback("显示模式已保存")
    }

    // ---------------- 分区定义 ----------------

    val globalSections = listOf(
        SettingsSection("connection") {
            SectionBlock(
                icon = R.drawable.ic_lucide_globe,
                title = "连接",
            ) {
                val connState = AgentConnection.connState.collectAsState().value
                val transport = AgentConnection.connTransport.collectAsState().value
                val connAddr = AgentConnection.connAddress.collectAsState().value

                // 连接状态摘要:圆点 + 状态 + 传输方式
                Row(
                    Modifier.fillMaxWidth().padding(vertical = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Box(
                        Modifier
                            .size(8.dp)
                            .background(
                                when (connState) {
                                    "ok" -> Pa.Green
                                    "down" -> Pa.Red
                                    else -> Pa.TextTertiary
                                },
                                CircleShape,
                            ),
                    )
                    Spacer(Modifier.width(9.dp))
                    Column(Modifier.weight(1f)) {
                        Text(
                            when (connState) {
                                "ok" -> "已连接 · $transport"
                                "down" -> "未连接 · 自动重连中"
                                else -> "检查中…"
                            },
                            fontSize = 13.5.sp,
                            color = Pa.TextPrimary,
                        )
                        Text(
                            if (connState == "ok") connAddr else "USB → 局域网 → Tailscale",
                            fontSize = 10.5.sp,
                            color = Pa.TextTertiary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
                Box(Modifier.fillMaxWidth().height(1.dp).background(Pa.Separator))
                Spacer(Modifier.height(12.dp))

                connectionError?.let { diagnostic ->
                    Text(
                        "${diagnostic.message}\n地址：${diagnostic.address}",
                        Modifier
                            .fillMaxWidth()
                            .background(Pa.Bg, RoundedCornerShape(12.dp))
                            .padding(11.dp),
                        fontSize = 11.sp,
                        color = Pa.RedText,
                    )
                    Spacer(Modifier.height(10.dp))
                }

                SettingsTextField(
                    value = addr,
                    onValueChange = { addr = it },
                    label = "地址 (host:port)",
                    keyboardType = KeyboardType.Uri,
                )
                Spacer(Modifier.height(10.dp))
                SettingsTextField(
                    value = token,
                    onValueChange = { token = it },
                    label = "配对 token(未配对留空)",
                    password = true,
                )
                Spacer(Modifier.height(14.dp))

                // 拆分按钮:主=保存并测试;下拉=仅保存 / 重扫候选 / 清空输入
                SplitButton(
                    label = if (connectionSaving) "连接中…" else "保存并测试连接",
                    onMainClick = { saveAndTestConnection() },
                    options = listOf(
                        SplitOption("仅保存凭证", "不立即测试,交给自动重连接管") { saveCredentialsOnly() },
                        SplitOption("重新扫描候选地址", "USB → 局域网 → Tailscale 依次尝试") { rescanCandidates() },
                        SplitOption("清空输入", "重置地址与 token 输入框") {
                            addr = ""
                            token = ""
                            showSettingsFeedback("已清空输入")
                        },
                    ),
                )
                Spacer(Modifier.height(4.dp))
            }
        },
        SettingsSection("appearance") {
            SectionBlock(
                icon = R.drawable.ic_lucide_file_text,
                title = "外观与阅读",
            ) {
                Spacer(Modifier.height(10.dp))
                // 界面文字大小:主=循环切换,下拉=三选一
                val uiOptions = listOf(
                    Triple("0.9", "小", 0.9f),
                    Triple("1.0", "标准", 1.0f),
                    Triple("1.15", "大", 1.15f),
                )
                val uiIdx = uiOptions.indexOfFirst {
                    kotlin.math.abs(it.third - AgentConnection.uiTextScale) < 0.02f
                }.coerceAtLeast(0)
                SplitButton(
                    label = "界面文字大小",
                    mainLabel = uiOptions[uiIdx].second,
                    onMainClick = {
                        val next = uiOptions[(uiIdx + 1) % uiOptions.size]
                        AgentConnection.setTextScale(next.third, AgentConnection.thinkTextScale)
                        showSettingsFeedback("界面字号已保存")
                    },
                    options = uiOptions.map { (id, label, v) ->
                        SplitOption(
                            title = label,
                            selected = kotlin.math.abs(v - AgentConnection.uiTextScale) < 0.02f,
                        ) {
                            AgentConnection.setTextScale(v, AgentConnection.thinkTextScale)
                            showSettingsFeedback("界面字号已保存")
                        }
                    },
                )
                Spacer(Modifier.height(8.dp))
                // 实时预览:改动立刻可见(唯一保留的说明性内容)
                Text(
                    "预览:这是一段界面文字,当前字号 ×" +
                        String.format(java.util.Locale.US, "%.2f", AgentConnection.uiTextScale),
                    Modifier
                        .fillMaxWidth()
                        .background(Pa.Bg, RoundedCornerShape(12.dp))
                        .padding(horizontal = 12.dp, vertical = 10.dp),
                    fontSize = (13 * AgentConnection.uiTextScale).sp,
                    color = Pa.TextSecondary,
                )
                Spacer(Modifier.height(8.dp))

                // 思考显示大小
                val thinkOptions = listOf(
                    Triple("0.85", "小", 0.85f),
                    Triple("1.0", "标准", 1.0f),
                    Triple("1.25", "大", 1.25f),
                )
                val thinkIdx = thinkOptions.indexOfFirst {
                    kotlin.math.abs(it.third - AgentConnection.thinkTextScale) < 0.02f
                }.coerceAtLeast(0)
                SplitButton(
                    label = "思考显示大小",
                    mainLabel = thinkOptions[thinkIdx].second,
                    onMainClick = {
                        val next = thinkOptions[(thinkIdx + 1) % thinkOptions.size]
                        AgentConnection.setTextScale(AgentConnection.uiTextScale, next.third)
                        showSettingsFeedback("思考字号已保存")
                    },
                    options = thinkOptions.map { (id, label, v) ->
                        SplitOption(
                            title = label,
                            selected = kotlin.math.abs(v - AgentConnection.thinkTextScale) < 0.02f,
                        ) {
                            AgentConnection.setTextScale(AgentConnection.uiTextScale, v)
                            showSettingsFeedback("思考字号已保存")
                        }
                    },
                )
                Spacer(Modifier.height(8.dp))

                // 工具详情(折叠/展开)
                SplitButton(
                    label = "工具详情",
                    mainLabel = if (AgentConnection.toolDetailsExpanded) "展开" else "折叠",
                    onMainClick = {
                        AgentConnection.toolDetailsExpanded = !AgentConnection.toolDetailsExpanded
                        showSettingsFeedback("工具详情设置已保存")
                    },
                    options = listOf(
                        SplitOption(
                            "折叠", selected = !AgentConnection.toolDetailsExpanded,
                        ) {
                            AgentConnection.toolDetailsExpanded = false
                            showSettingsFeedback("工具详情设置已保存")
                        },
                        SplitOption(
                            "展开", selected = AgentConnection.toolDetailsExpanded,
                        ) {
                            AgentConnection.toolDetailsExpanded = true
                            showSettingsFeedback("工具详情设置已保存")
                        },
                    ),
                )
                Spacer(Modifier.height(8.dp))

                // 显示思考过程(展示/隐藏)
                SplitButton(
                    label = "显示思考过程",
                    mainLabel = if (AgentConnection.showThinking) "展示" else "隐藏",
                    onMainClick = {
                        AgentConnection.showThinking = !AgentConnection.showThinking
                        showSettingsFeedback(
                            if (AgentConnection.showThinking) "将流式显示思考过程" else "将只显示对话结果",
                        )
                    },
                    options = listOf(
                        SplitOption("展示", selected = AgentConnection.showThinking) {
                            AgentConnection.showThinking = true
                            showSettingsFeedback("将流式显示思考过程")
                        },
                        SplitOption("隐藏", selected = !AgentConnection.showThinking) {
                            AgentConnection.showThinking = false
                            showSettingsFeedback("将只显示对话结果")
                        },
                    ),
                )
                Spacer(Modifier.height(8.dp))
            }
        },
        SettingsSection("display") {
            SectionBlock(
                icon = R.drawable.ic_lucide_monitor,
                title = "显示模式",
            ) {
                Spacer(Modifier.height(10.dp))
                val modeOptions = listOf(
                    Triple("system", "跟随系统", "自动跟随系统深浅色"),
                    Triple("light", "浅色", "始终使用浅色界面"),
                    Triple("dark", "深色", "始终使用深色界面"),
                )
                val currentModeIdx = modeOptions
                    .indexOfFirst { it.first == AgentConnection.themeMode }
                    .coerceAtLeast(0)
                SplitButton(
                    label = "显示模式",
                    mainLabel = modeOptions[currentModeIdx].second,
                    onMainClick = {
                        val next = modeOptions[(currentModeIdx + 1) % modeOptions.size]
                        applyThemeMode(next.first)
                    },
                    options = modeOptions.map { (id, name, desc) ->
                        SplitOption(
                            title = name,
                            desc = desc,
                            selected = AgentConnection.themeMode == id,
                        ) { applyThemeMode(id) }
                    },
                )
                Spacer(Modifier.height(6.dp))
            }
        },
        SettingsSection("about") {
            SectionBlock(
                icon = R.drawable.ic_lucide_info,
                title = "关于",
            ) {
                SettingRow(
                    title = host?.displayName ?: "…",
                    desc = "电脑 · ${host?.os ?: ""}/${host?.arch ?: ""}",
                    divider = true,
                )
                SettingRow(title = "PocketAgent", desc = "v0.1.0 · 复刻并超越 Mobile Codex")
            }
        },
    )

    val codexSections = listOf(
        SettingsSection("permissions") {
            SectionBlock(
                icon = R.drawable.ic_lucide_shield,
                title = "权限与安全",
            ) {
                Spacer(Modifier.height(10.dp))
                // 审批策略
                val approvalOptions = listOf(
                    Triple("untrusted", "仅信任", "只自动执行白名单命令"),
                    Triple("on-request", "按需询问", "运行命令前先征求批准"),
                    Triple("never", "完全访问", "不询问,直接执行"),
                )
                val approvalIdx = approvalOptions
                    .indexOfFirst { it.first == settings?.approvalPolicy }
                    .coerceAtLeast(0)
                SplitButton(
                    label = "审批策略",
                    mainLabel = approvalOptions[approvalIdx].second,
                    onMainClick = {
                        val next = approvalOptions[(approvalIdx + 1) % approvalOptions.size]
                        writeCodex(approval = next.first)
                    },
                    options = approvalOptions.map { (id, label, desc) ->
                        SplitOption(
                            title = label,
                            desc = desc,
                            selected = settings?.approvalPolicy == id,
                        ) { writeCodex(approval = id) }
                    },
                )
                Spacer(Modifier.height(8.dp))

                // 沙箱模式
                val sandboxOptions = listOf(
                    Triple("read-only", "只读", "只能读取文件"),
                    Triple("workspace-write", "工作区可写", "仅工作区内可写"),
                    Triple("danger-full-access", "完全访问", "不限制文件与网络"),
                )
                val sandboxIdx = sandboxOptions
                    .indexOfFirst { it.first == settings?.sandbox }
                    .coerceAtLeast(0)
                SplitButton(
                    label = "沙箱模式",
                    mainLabel = sandboxOptions[sandboxIdx].second,
                    onMainClick = {
                        val next = sandboxOptions[(sandboxIdx + 1) % sandboxOptions.size]
                        writeCodex(sandbox = next.first)
                    },
                    options = sandboxOptions.map { (id, label, desc) ->
                        SplitOption(
                            title = label,
                            desc = desc,
                            selected = settings?.sandbox == id,
                        ) { writeCodex(sandbox = id) }
                    },
                )
                Spacer(Modifier.height(10.dp))

                Box(
                    Modifier
                        .fillMaxWidth()
                        .pressable(haptic = true) { showApprovalPreview = true }
                        .background(Pa.Bg, RoundedCornerShape(13.dp))
                        .padding(vertical = 12.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text("预览审批弹窗", fontSize = 13.5.sp, color = Pa.TextPrimary)
                }
                Spacer(Modifier.height(6.dp))
            }
        },
        SettingsSection("model") {
            SectionBlock(
                icon = R.drawable.ic_lucide_cpu,
                title = "模型与思考",
            ) {
                Spacer(Modifier.height(10.dp))
                val currentModelName = settings?.model?.takeIf { it.isNotEmpty() }
                    ?.let { id -> modelChoices.find { it.id == id }?.displayName ?: id }
                    ?: "跟随桌面配置"
                // 默认模型:主=完整选择器,下拉=快速切换
                SplitButton(
                    label = "默认模型",
                    caption = currentModelName,
                    onMainClick = {
                        if (modelChoices.isEmpty()) {
                            showSettingsFeedback("未获取到模型列表,请检查连接")
                        } else {
                            showModelPicker = true
                        }
                    },
                    options = buildList {
                        add(
                            SplitOption(
                                "跟随桌面配置",
                                "使用电脑上当前 agent 的默认模型",
                                selected = settings?.model.isNullOrEmpty(),
                            ) { writeCodex(model = "") },
                        )
                        if (modelChoices.isEmpty()) {
                            add(SplitOption("未获取到模型列表", "请检查连接后重试") { })
                        } else {
                            modelChoices.forEach { m ->
                                add(
                                    SplitOption(
                                        title = m.displayName.ifBlank { m.id },
                                        desc = buildString {
                                            if (m.contextWindow > 0) append("上下文 ${m.contextWindow / 1000}k")
                                            if (m.supportsThinking) {
                                                if (isNotEmpty()) append(" · ")
                                                append("支持思考")
                                            }
                                        },
                                        selected = settings?.model == m.id,
                                    ) { writeCodex(model = m.id) },
                                )
                            }
                        }
                    },
                )
                Spacer(Modifier.height(8.dp))

                // 思考强度
                val effortOptions = listOf(
                    Triple("low", "低", "最快,浅推理"),
                    Triple("medium", "中", "速度与深度平衡"),
                    Triple("high", "高", "更慢,更细"),
                    Triple("xhigh", "极高", "最慢,最细"),
                )
                val effortIdx = effortOptions
                    .indexOfFirst { it.first == settings?.effort }
                    .coerceAtLeast(0)
                SplitButton(
                    label = "思考强度",
                    mainLabel = effortOptions[effortIdx].second,
                    onMainClick = {
                        val next = effortOptions[(effortIdx + 1) % effortOptions.size]
                        writeCodex(effort = next.first)
                    },
                    options = effortOptions.map { (id, label, desc) ->
                        SplitOption(
                            title = label,
                            desc = desc,
                            selected = settings?.effort == id,
                        ) { writeCodex(effort = id) }
                    },
                )
                Spacer(Modifier.height(6.dp))
            }
        },
        SettingsSection("account") {
            SectionBlock(
                icon = R.drawable.ic_lucide_user,
                title = "Codex 账户",
            ) {
                SettingRow(
                    title = host?.displayName ?: "…",
                    desc = "已登录 · 模型与用量走桌面端账号",
                )
            }
        },
        SettingsSection("mcp") {
            SectionBlock(
                icon = R.drawable.ic_lucide_server,
                title = "集成 · MCP 服务器",
            ) {
                if (mcpList.isEmpty()) {
                    Text(
                        if (settingsLoading) "加载中…" else "未获取到 MCP 列表",
                        fontSize = 12.sp, color = Pa.TextTertiary,
                        modifier = Modifier.padding(vertical = 12.dp),
                    )
                } else {
                    mcpList.forEachIndexed { index, m ->
                        SettingRow(
                            title = m.name,
                            desc = "${m.tools} 个工具",
                            divider = index != mcpList.lastIndex,
                        )
                    }
                }
            }
        },
    )

    val sections: List<SettingsSection> = when (selectedScope) {
        SettingsScope.GLOBAL -> globalSections
        SettingsScope.RUNNING ->
            if (runningAgent == "codex") {
                codexSections
            } else {
                // 其他 agent 的专属设置:接入完成后在此填充。
                listOf(
                    SettingsSection("placeholder") {
                        SectionBlock(
                            icon = R.drawable.ic_lucide_cpu,
                            title = "${agentDisplay(runningAgent)} 的专属设置",
                        ) {
                            Text(
                                "${agentDisplay(runningAgent)} 的模型、权限与工具设置将在接入完成后在此显示；" +
                                    "在此之前可先到【其他 Agent】里切回 Codex。",
                                fontSize = 12.sp, color = Pa.TextTertiary,
                                modifier = Modifier.padding(vertical = 12.dp),
                            )
                        }
                    },
                )
            }
        SettingsScope.OTHER -> {
            // 切换 Agent:一个 SplitButton 搞定(主=切到当前选中项,下拉=换一个)
            val agentChoices = listOf("codex" to "Codex") + OTHER_AGENT_OPTIONS
            val pendingIdx = agentChoices
                .indexOfFirst { it.first == selectedOtherAgent }
                .coerceAtLeast(0)
            listOf(
                SettingsSection("switch") {
                    SectionBlock(
                        icon = R.drawable.ic_lucide_cpu,
                        title = "切换 Agent",
                    ) {
                        Spacer(Modifier.height(10.dp))
                        SplitButton(
                            label = "切换 Agent",
                            mainLabel = agentChoices[pendingIdx].second,
                            onMainClick = {
                                val id = agentChoices[pendingIdx].first
                                AgentConnection.switchAgent(id)
                                showSettingsFeedback("已切换到 ${agentDisplay(id)}")
                                selectedScope = SettingsScope.RUNNING
                            },
                            options = agentChoices.map { (id, name) ->
                                SplitOption(
                                    title = name,
                                    selected = runningAgent == id,
                                ) {
                                    selectedOtherAgent = id
                                    AgentConnection.switchAgent(id)
                                    showSettingsFeedback("已切换到 ${agentDisplay(id)}")
                                    selectedScope = SettingsScope.RUNNING
                                }
                            },
                        )
                        Spacer(Modifier.height(8.dp))
                        Text(
                            "当前运行:${agentDisplay(runningAgent)}",
                            Modifier.padding(start = 3.dp),
                            fontSize = 10.5.sp,
                            color = Pa.TextTertiary,
                        )
                        Spacer(Modifier.height(6.dp))
                    }
                },
            )
        }
    }

    // 切换板块时回到顶部
    LaunchedEffect(selectedScope) {
        listState.scrollToItem(0)
    }

    Column(
        modifier
            .fillMaxSize()
            .background(Pa.Bg)
            .statusBarsPadding(),
    ) {
        // ---------- 页头:返回 + 大标题 ----------
        Row(
            Modifier.fillMaxWidth().padding(start = 12.dp, end = 16.dp, top = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconActionButton(R.drawable.ic_lucide_chevron_left, "返回", onBack, bg = Pa.Card)
        }
        Column(Modifier.fillMaxWidth().padding(start = 20.dp, end = 20.dp, top = 10.dp)) {
            Text("设置", fontSize = 30.sp, color = Pa.TextPrimary)
            Text(
                "连接 · 外观 · 各 Agent 的专属配置",
                Modifier.padding(top = 2.dp),
                fontSize = 12.sp,
                color = Pa.TextTertiary,
            )
        }

        Spacer(Modifier.height(14.dp))

        // ---------- 范围分段(全局 / 当前 Agent / 其他) ----------
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp)
                .background(Pa.Card, RoundedCornerShape(50))
                .padding(4.dp),
        ) {
            val scopeTabs = listOf(
                SettingsScope.GLOBAL to "全局",
                SettingsScope.RUNNING to agentDisplay(runningAgent),
                SettingsScope.OTHER to "其他 Agent",
            )
            scopeTabs.forEach { (scopeOption, label) ->
                ScopeTab(
                    label = label,
                    selected = selectedScope == scopeOption,
                    modifier = Modifier.weight(1f),
                ) { selectedScope = scopeOption }
            }
        }

        // ---------- 滚动内容(公告区 + 分区卡片) ----------
        LazyColumn(
            state = listState,
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .navigationBarsPadding(),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 12.dp, bottom = 14.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            item(key = "banner") {
                Column {
                    settingsError?.let { message ->
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .background(Pa.Red.copy(alpha = 0.12f), RoundedCornerShape(14.dp))
                                .padding(horizontal = 14.dp, vertical = 10.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Icon(
                                painterResource(R.drawable.ic_lucide_triangle_alert), null,
                                tint = Pa.RedText, modifier = Modifier.size(15.dp),
                            )
                            Spacer(Modifier.width(9.dp))
                            Text(message, Modifier.weight(1f), fontSize = 12.sp, color = Pa.RedText)
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
                            Modifier.fillMaxWidth().padding(horizontal = 6.dp),
                            fontSize = 12.sp,
                            color = when {
                                message.contains("失败") -> Pa.RedText
                                message.contains("保存中") -> Pa.Accent
                                else -> Pa.GreenText
                            },
                        )
                    }
                }
            }
            sections.forEach { section ->
                item(key = "sec-${section.id}") { section.content() }
            }
        }
    }

    if (showApprovalPreview) {
        ApprovalPreviewSheet(onDismiss = { showApprovalPreview = false })
    }

    // 默认模型选择弹窗(完整选择器,由"默认模型"SplitButton 主按钮触发)
    if (showModelPicker) {
        ModelPickerDialog(
            models = modelChoices,
            selected = settings?.model.orEmpty(),
            onPick = {
                writeCodex(model = it)
                showModelPicker = false
            },
            onDismiss = { showModelPicker = false },
        )
    }
}

/** 范围分段按钮(选中 = 蓝底白字)。 */
@Composable
private fun ScopeTab(label: String, selected: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    Box(
        modifier
            .pressable(haptic = true) { onClick() }
            .background(if (selected) Pa.Accent else Color.Transparent, RoundedCornerShape(50))
            .padding(vertical = 9.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            fontSize = 12.5.sp,
            color = if (selected) Color.White else Pa.TextSecondary,
            fontWeight = if (selected) FontWeight.Medium else FontWeight.Normal,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

/**
 * 自绘输入框:顶部小标签 + 深底圆角输入区。
 *
 * 【为什么不用 OutlinedTextField】Material 默认描边框在纯黑页面上是一圈灰线,
 * 和"深灰圆角卡"的语言不搭;而且自带的内边距会撑高整张卡。这里只用 [BasicTextField],
 * 外观完全由我们控制。
 */
@Composable
private fun SettingsTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    password: Boolean = false,
    keyboardType: KeyboardType = KeyboardType.Text,
) {
    Column(Modifier.fillMaxWidth()) {
        Text(
            label,
            Modifier.padding(start = 3.dp, bottom = 6.dp),
            fontSize = 10.5.sp,
            color = Pa.TextTertiary,
        )
        Box(
            Modifier
                .fillMaxWidth()
                .background(Pa.Bg, RoundedCornerShape(12.dp))
                .padding(horizontal = 13.dp, vertical = 12.dp),
            contentAlignment = Alignment.CenterStart,
        ) {
            if (value.isEmpty()) {
                Text("—", fontSize = 13.5.sp, color = Pa.TextTertiary)
            }
            BasicTextField(
                value = value,
                onValueChange = onValueChange,
                singleLine = true,
                textStyle = TextStyle(color = Pa.TextPrimary, fontSize = 13.5.sp),
                cursorBrush = SolidColor(Pa.Accent),
                keyboardOptions = KeyboardOptions(keyboardType = keyboardType),
                visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}

/**
 * 默认模型选择弹窗:第一项固定为"跟随桌面配置"(空 id),
 * 其余为 daemon 上报的可用模型;小字标出上下文窗口与是否支持思考。
 */
@Composable
private fun ModelPickerDialog(
    models: List<ModelInfo>,
    selected: String,
    onPick: (String) -> Unit,
    onDismiss: () -> Unit,
) {
    Dialog(onDismissRequest = onDismiss) {
        Column(
            Modifier
                .fillMaxWidth()
                .background(Pa.CardHigh, RoundedCornerShape(22.dp))
                .padding(20.dp),
        ) {
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("选择默认模型", fontSize = 17.sp, color = Pa.TextPrimary)
                IconActionButton(
                    R.drawable.ic_lucide_x, "关闭", onDismiss,
                    bg = Color.Transparent, size = 36.dp, iconSize = 18.dp,
                )
            }
            Spacer(Modifier.height(10.dp))
            Column(
                Modifier
                    .fillMaxWidth()
                    .heightIn(max = 360.dp)
                    .verticalScroll(rememberScrollState()),
            ) {
                ModelChoiceRow(
                    title = "跟随桌面配置",
                    subtitle = "使用电脑上当前 agent 的默认模型",
                    selected = selected.isEmpty(),
                    onClick = { onPick("") },
                )
                if (models.isEmpty()) {
                    Text(
                        "未获取到模型列表(请检查连接后重试)",
                        fontSize = 12.sp, color = Pa.TextTertiary,
                        modifier = Modifier.padding(vertical = 8.dp),
                    )
                }
                models.forEach { m ->
                    ModelChoiceRow(
                        title = m.displayName.ifBlank { m.id },
                        subtitle = buildString {
                            if (m.contextWindow > 0) append("上下文 ${m.contextWindow / 1000}k")
                            if (m.supportsThinking) {
                                if (isNotEmpty()) append(" · ")
                                append("支持思考")
                            }
                        },
                        selected = selected == m.id,
                        onClick = { onPick(m.id) },
                    )
                }
            }
        }
    }
}

@Composable
private fun ModelChoiceRow(
    title: String,
    subtitle: String,
    selected: Boolean,
    onClick: () -> Unit,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .pressable(haptic = true) { onClick() }
            .background(
                if (selected) Pa.Hover else Color.Transparent,
                RoundedCornerShape(12.dp),
            )
            .padding(horizontal = 10.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(
                title,
                fontSize = 15.sp,
                color = if (selected) Pa.Accent else Pa.TextPrimary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (subtitle.isNotEmpty()) {
                Text(subtitle, fontSize = 10.5.sp, color = Pa.TextTertiary)
            }
        }
        if (selected) {
            Icon(
                painterResource(R.drawable.ic_lucide_check), null,
                tint = Pa.Accent, modifier = Modifier.size(18.dp),
            )
        }
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
    val state = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var expanded by remember { mutableStateOf(false) }
    ModalBottomSheet(
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
                    color = Pa.AccentText,
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
                    fontFamily = FontFamily.Monospace,
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

/**
 * 分区卡:图标 + 标题 作为**卡片内页眉**,下方一条分隔线,再接设置行。
 */
@Composable
fun SectionBlock(
    icon: Int,
    title: String,
    content: @Composable androidx.compose.foundation.layout.ColumnScope.() -> Unit,
) {
    Column(
        Modifier
            .fillMaxWidth()
            .background(Pa.Card, RoundedCornerShape(20.dp)),
    ) {
        Row(
            Modifier
                .fillMaxWidth()
                .padding(start = 14.dp, end = 14.dp, top = 13.dp, bottom = 11.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                Modifier.size(31.dp).background(Pa.AccentDim, RoundedCornerShape(10.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(painterResource(icon), null, tint = Pa.Accent, modifier = Modifier.size(15.dp))
            }
            Spacer(Modifier.width(10.dp))
            Text(title, fontSize = 14.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold)
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(Pa.Separator))
        Column(Modifier.padding(horizontal = 14.dp, vertical = 2.dp)) {
            content()
        }
    }
}

/** 设置行:左标题,右控件,行间细分隔线(desc 仅用于展示"数据",不再写解释文案)。 */
@Composable
fun SettingRow(
    title: String,
    desc: String = "",
    divider: Boolean = false,
    control: @Composable RowScope.() -> Unit = {},
) {
    Column {
        Row(
            Modifier.fillMaxWidth().padding(vertical = 11.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.weight(1f)) {
                Text(title, fontSize = 13.5.sp, color = Pa.TextPrimary, maxLines = 1)
                if (desc.isNotEmpty()) {
                    Text(
                        desc,
                        Modifier.padding(top = 2.dp),
                        fontSize = 10.5.sp,
                        color = Pa.TextTertiary,
                        maxLines = 2,
                    )
                }
            }
            control()
        }
        if (divider) {
            Box(Modifier.fillMaxWidth().height(1.dp).background(Pa.Separator))
        }
    }
}
