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
import androidx.compose.foundation.layout.fillMaxHeight
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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.produceState
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
import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.component.fileKindOf
import dev.pocketagent.app.ui.component.humanSize
import dev.pocketagent.app.ui.component.parseHumanSize
import dev.pocketagent.app.ui.component.PaDownloader
import dev.pocketagent.app.ui.component.SplitButton
import dev.pocketagent.app.ui.component.SplitOption
import dev.pocketagent.app.ui.theme.AgentTheme
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CodexCapability
import pocketagent.v1.CodexSettings
import pocketagent.v1.CodexUsage
import pocketagent.v1.GetCodexSettingsRequest
import pocketagent.v1.GetCodexUsageRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.HostInfo
import pocketagent.v1.ListCodexCapabilitiesRequest
import pocketagent.v1.ListMcpServersRequest
import pocketagent.v1.ListModelsRequest
import pocketagent.v1.McpServerStatus
import pocketagent.v1.ModelInfo
import pocketagent.v1.RateLimitWindow
import pocketagent.v1.SetCodexSettingsRequest

private val OTHER_AGENT_OPTIONS = listOf(
    "opencode" to "OpenCode",
    "claude" to "Claude",
    "pi" to "Pi",
    "hermes" to "Hermes",
)

/** agent id → 展示名(二级页标题、agent 列表共用)。 */
private val AGENT_DISPLAY = (listOf("codex" to "Codex") + OTHER_AGENT_OPTIONS).toMap()

/**
 * 按托管策略(configRequirements)过滤可选项。
 *
 * 【为什么必须过滤 —— 一个"点了没反应"的经典坑】桌面上的 codex 可能被
 * 管理员/MDM 通过 requirements.toml 约束,白名单之外的取值会被 codex **直接拒绝**。
 * 而设置页走的是乐观更新:先改本地状态、再发请求。于是用户看到的是
 * "已改成完全访问",实际压根没生效,而且没有任何报错 —— 他会以为这个 App 坏了。
 * 与其给一个点了没用的选项,不如一开始就不给。
 *
 * [allowed] 为空表示该项无限制(未被托管),原样返回全部可选项。
 */
private fun <T> filterByManagedPolicy(
    all: List<T>,
    allowed: List<String>,
    id: (T) -> String,
): List<T> = if (allowed.isEmpty()) all else all.filter { id(it) in allowed }

/** 状态胶囊:一行小字 + 语义色底。用于 MCP 服务器"就绪/失败"这类一眼判断。 */
@Composable
private fun StatusChip(text: String, color: Color) {
    Text(
        text,
        Modifier
            .background(color.copy(alpha = 0.16f), RoundedCornerShape(6.dp))
            .padding(horizontal = 7.dp, vertical = 2.dp),
        color = color,
        fontSize = 10.5.sp,
        fontWeight = FontWeight.Medium,
    )
}

/**
 * 用量与限流面板。
 *
 * 【为什么值得占一屏】用户在手机外面发任务之前,第一个要确认的就是"还有没有额度"。
 * 以前只有 account/rateLimits/updated 被当告警转发,没有一个可以主动查看的地方 ——
 * 想知道额度只能回电脑。而手机上最贵的成本就是"为了看一眼数字而走回电脑前"。
 *
 * 【读不到时不能显示 0】这是最容易做错的一步:额度读不到(未登录 / 地区受限 /
 * 断网)时如果照常渲染 0%,用户会理解成"额度用完了",然后不敢发任务 ——
 * 一次数据缺失被读成一次业务结论。所以读不到就直说读不到,并带上原因。
 */
@Composable
private fun CodexUsageBody(usage: CodexUsage?, loading: Boolean) {
    if (usage == null) {
        Text(
            if (loading) "正在读取…" else "未获取到用量信息",
            fontSize = 12.sp, color = Pa.TextTertiary,
            modifier = Modifier.padding(vertical = 12.dp),
        )
        return
    }
    if (usage.error.isNotBlank()) {
        Text("暂时读不到额度", fontSize = 12.5.sp, color = Pa.TextPrimary)
        Spacer(Modifier.height(4.dp))
        Text(usage.error, fontSize = 11.sp, color = Pa.TextTertiary)
        return
    }

    Row(verticalAlignment = Alignment.CenterVertically) {
        Text(
            usage.planType.ifBlank { "未报告套餐" },
            Modifier.weight(1f),
            fontSize = 13.sp,
            color = Pa.TextPrimary,
            fontWeight = FontWeight.Medium,
        )
        if (usage.spendControlReached) {
            StatusChip("已达上限", Pa.Red)
        }
    }
    Spacer(Modifier.height(10.dp))

    RateLimitRow("5 小时", usage.primary)
    Spacer(Modifier.height(8.dp))
    RateLimitRow("每周", usage.secondary)

    // 累计用量属于"看一眼就够"的补充信息,放在最后,不跟限流条抢注意力。
    val extras = buildList {
        if (usage.lifetimeTokens > 0) {
            add("累计 ${formatTokenCount(usage.lifetimeTokens)} tokens")
        }
        if (usage.peakDailyTokens > 0) {
            add("单日峰值 ${formatTokenCount(usage.peakDailyTokens)}")
        }
        if (usage.currentStreakDays > 0) add("连续 ${usage.currentStreakDays} 天")
    }
    if (extras.isNotEmpty()) {
        Spacer(Modifier.height(9.dp))
        Text(extras.joinToString(" · "), fontSize = 11.sp, color = Pa.TextTertiary)
    }
    if (usage.hasCredits || usage.creditBalance.isNotBlank()) {
        Spacer(Modifier.height(4.dp))
        Text(
            if (usage.unlimitedCredits) "额度不限量" else "剩余额度 ${usage.creditBalance}",
            fontSize = 11.sp,
            color = Pa.TextTertiary,
        )
    }
}

/**
 * 一条限流窗口:名称 + 百分比 + 进度条 + 重置倒计时。
 * 数据缺失(usedPercent 与 resets 都是 0 且窗口长度未知)时整条不显示 ——
 * 画一条永远 0% 的进度条比不画更容易误导。
 */
@Composable
private fun RateLimitRow(fallbackLabel: String, window: RateLimitWindow?) {
    if (window == null) return
    val hasData = window.usedPercent > 0 || window.resetsAtUnixS > 0 || window.windowDurationMins > 0
    if (!hasData) return

    val pct = usedPercentClamped(window)
    val barColor = when (usageLevelColorPercent(pct)) {
        2 -> Pa.Red
        1 -> Pa.Yellow
        else -> Pa.Green
    }
    val reset = resetCountdownLabel(window.resetsAtUnixS, System.currentTimeMillis())

    Column(Modifier.fillMaxWidth()) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                rateLimitWindowLabel(window.windowDurationMins, fallbackLabel),
                Modifier.weight(1f),
                fontSize = 12.sp,
                color = Pa.TextSecondary,
            )
            Text("$pct%", fontSize = 12.sp, color = barColor, fontWeight = FontWeight.Medium)
        }
        Spacer(Modifier.height(5.dp))
        Box(
            Modifier
                .fillMaxWidth()
                .height(6.dp)
                .background(Pa.Bg, RoundedCornerShape(50)),
        ) {
            Box(
                Modifier
                    .fillMaxWidth(pct / 100f)
                    .fillMaxHeight()
                    .background(barColor, RoundedCornerShape(50)),
            )
        }
        if (reset.isNotBlank()) {
            Spacer(Modifier.height(4.dp))
            Text(reset, fontSize = 10.5.sp, color = Pa.TextTertiary)
        }
    }
}

/** 大数字缩写成 k / M,避免"1234567 tokens"这种读不出量级的写法。 */
private fun formatTokenCount(value: Long): String = when {
    value >= 1_000_000L -> String.format(java.util.Locale.US, "%.1fM", value / 1_000_000.0)
    value >= 1_000L -> String.format(java.util.Locale.US, "%.0fk", value / 1_000.0)
    else -> value.toString()
}
/** 托管策略生效时的说明条:告诉用户"为什么选项变少了",而不是让他自己发现少了。 */
@Composable
private fun ManagedPolicyNote(text: String) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(Pa.Yellow.copy(alpha = 0.10f), RoundedCornerShape(11.dp))
            .padding(horizontal = 11.dp, vertical = 9.dp),
        verticalAlignment = Alignment.Top,
    ) {
        Icon(
            painterResource(R.drawable.ic_lucide_shield),
            null,
            tint = Pa.Yellow,
            modifier = Modifier.size(13.dp).padding(top = 1.dp),
        )
        Spacer(Modifier.width(7.dp))
        Text(
            text,
            fontSize = 11.sp,
            color = Pa.TextSecondary,
            modifier = Modifier.weight(1f),
        )
    }
}

/**
 * agent id → 品牌图标:与主页网格用的是**同一组 drawable**,
 * 不再用 cpu 线性图标占位(用户反馈:设置里的 Agent 图标和主页对不上)。
 * 主页叫 claude-code,这里历史上叫 claude,两个 id 都映射到同一张图。
 */
private val AGENT_ICON = mapOf(
    "codex" to R.drawable.agent_codex,
    "opencode" to R.drawable.agent_opencode,
    "claude" to R.drawable.agent_claudecode,
    "claude-code" to R.drawable.agent_claudecode,
    "pi" to R.drawable.agent_pi,
    "hermes" to R.drawable.agent_hermes,
    "dsh" to R.drawable.agent_dsh,
    "zcode" to R.drawable.agent_zcode,
)

private fun agentIcon(id: String): Int? = AGENT_ICON[id]

private fun agentDisplay(id: String): String = AGENT_DISPLAY[id] ?: id.ifBlank { "Agent" }

/**
 * 设置页(重做版:一级导航、二级编辑)。
 *
 * 【信息架构】
 *  1. 一级页 = 电脑连接卡(现状一目了然)+ 少量导航行(外观 / Agent / 用户数据 / 关于),
 *     不再放"范围分段"(全局/当前/其他三段切换把同一批设置拆到三处,实际很难找到东西);
 *  2. 所有编辑动作收进二级页:外观与阅读 / Agent 与模型 / 用户数据管理;
 *  3. 多选项设置仍统一 SplitButton;设置项不写解释性小字(字号预览除外)。
 *
 * 【逻辑未变】审批/沙箱/模型/思考强度照样走 writeCodex 乐观更新 + 失败回滚;
 * 连接凭证照样走 setCredentials + connectWithFallback。
 */
@Composable
fun SettingsScreen(
    onBack: () -> Unit,
    /** 任务中心点下载卡片进来时直达「用户数据管理」二级页。 */
    openUserData: Boolean = false,
    /** 用内置浏览器查看已下载的网页/文本(由 MainActivity 提供导航)。 */
    onOpenBrowser: (String) -> Unit = {},
    modifier: Modifier = Modifier,
) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var addr by remember { mutableStateOf(AgentConnection.configuredAddress()) }
    var token by remember { mutableStateOf(AgentConnection.token) }
    var settings by remember { mutableStateOf<CodexSettings?>(null) }
    var mcpList by remember { mutableStateOf(listOf<McpServerStatus>()) }
    var mcpLoading by remember { mutableStateOf(true) }
    // 用量/限流:以前只把 account/rateLimits/updated 当告警转发,没有可查看的面板,
    // 用户想知道"还有没有额度"只能回到电脑上。
    var codexUsage by remember { mutableStateOf<CodexUsage?>(null) }
    var usageLoading by remember { mutableStateOf(true) }
    // 能力可见性:这台电脑上这个 agent 到底有哪些技能/hooks/插件。
    var capabilities by remember { mutableStateOf(listOf<CodexCapability>()) }
    // 【必须与"确实是空的"分开】能力是本地读取(快),用量要打上游(慢)。
    // 如果共用一个 loading 标志,慢调用还没回来时能力面板就会显示
    // "未报告任何技能" —— 把"还没加载"说成"确实没有",用户会以为功能坏了。
    var capabilitiesLoading by remember { mutableStateOf(true) }
    // 读失败的原因。必须与"确实是空的"分开:失败时渲染成"未报告任何技能"
    // 等于告诉用户"你没装技能",而真相是这次没读到 —— 用户会去重装技能,
    // 白折腾一圈。这是这块 UI 里最容易漏掉的第四个状态。
    var capabilitiesError by remember { mutableStateOf<String?>(null) }
    var host by remember { mutableStateOf<HostInfo?>(null) }
    var showApprovalPreview by remember { mutableStateOf(false) }
    var connectionSaving by remember { mutableStateOf(false) }
    var settingsError by remember { mutableStateOf<String?>(null) }
    var settingsFeedback by remember { mutableStateOf<String?>(null) }
    // 默认模型选择器(完整选择弹窗,主按钮触发)
    var showModelPicker by remember { mutableStateOf(false) }
    var modelChoices by remember { mutableStateOf(listOf<ModelInfo>()) }
    val runningAgent = AgentConnection.selectedAgent
    val connectionError = AgentConnection.connError.collectAsState().value
    val connectionEpoch = AgentConnection.connectionEpoch.collectAsState().value

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
                host = AgentConnection.hostService().getHostInfo(GetHostInfoRequest.getDefaultInstance())
            }
        } catch (e: Exception) {
            settingsError = "无法加载 Codex 设置：${e.message ?: "连接失败"}"
        }
        // 三个面板依次拉取,但**每个面板有自己的 loading 标志**。
        //
        // 【为什么不是并发】试过并发,结果三个调用一起失败:
        // UNAVAILABLE: Channel shutdownNow invoked。原因是自动重连探活会在
        // 判定连接异常时回收整条 gRPC 通道(shutdownNow),并发的长调用会被一起打断。
        // 串行时只有当前这一个调用受影响,其余面板照常出数据。
        //
        // 【顺序:先本地后上游】MCP 与能力是 codex 本地读取(毫秒级),用量要打
        // chatgpt.com(可能几十秒,还会超时)。把用量放最后,慢调用就不会挡住
        // 前面两个面板先显示出来。
        //
        // 【为什么每个面板必须有自己的标志】以前三个面板共用 settingsLoading,
        // 而那个标志只代表"设置本体"读完了 —— 于是"还在读取"会被显示成
        // "未获取到 MCP 列表" / "未报告任何技能",用户据此以为 MCP 没配、
        // 技能没装,而真相只是还没回来。
        try {
            mcpList = withContext(Dispatchers.IO) {
                AgentConnection.modelServiceSlow()
                    .listMcpServers(ListMcpServersRequest.newBuilder().build())
                    .serversList
            }
        } catch (e: Exception) {
            if (settingsError == null) settingsError = "无法加载 MCP 列表：${e.message ?: "连接失败"}"
        } finally {
            mcpLoading = false
        }
        try {
            capabilities = withContext(Dispatchers.IO) {
                AgentConnection.modelServiceSlow()
                    .listCodexCapabilities(ListCodexCapabilitiesRequest.newBuilder().build())
                    .itemsList
            }
        } catch (e: Exception) {
            // 不抢占 settingsError(它属于设置本体),但要留下自己的原因,
            // 供面板区分"读不到"和"确实没有"。
            capabilitiesError = e.message ?: "连接失败"
        } finally {
            capabilitiesLoading = false
        }
        // 用量:读不到时 daemon 会把原因放在 usage.error 里带回来,这里不把它当异常 ——
        // "读不到"本身就是要展示给用户的信息(未登录 / 地区不支持 / 网络)。
        try {
            codexUsage = withContext(Dispatchers.IO) {
                AgentConnection.modelServiceSlow()
                    .getCodexUsage(GetCodexUsageRequest.newBuilder().build())
            }
        } catch (e: Exception) {
            codexUsage = CodexUsage.newBuilder()
                .setError(e.message ?: "连接失败")
                .build()
        } finally {
            usageLoading = false
        }
    }

    /**
     * 桌面端设置写入:审批/沙箱/默认模型/思考强度 + 行为与降噪四件套。
     * daemon 每轮 turn/start 显式携带(codex 会话内真实生效),
     * 绝不改写桌面全局 config.toml。
     *
     * 所有参数为 null 表示"不改这一项" —— 与既有四件套同一套语义,
     * 保证不会把用户没碰过的设置清空。
     */
    fun writeCodex(
        approval: String? = null,
        sandbox: String? = null,
        model: String? = null,
        effort: String? = null,
        hideReasoning: Boolean? = null,
        verbosity: String? = null,
        reasoningSummary: String? = null,
        pasteBurst: Boolean? = null,
    ) {
        val previous = settings ?: CodexSettings.getDefaultInstance()
        val optimistic = previous.toBuilder()
        approval?.let { optimistic.setApprovalPolicy(it) }
        sandbox?.let { optimistic.setSandbox(it) }
        model?.let { optimistic.setModel(it) }
        effort?.let { optimistic.setEffort(it) }
        hideReasoning?.let { optimistic.setHideAgentReasoning(it) }
        verbosity?.let { optimistic.setModelVerbosity(it) }
        reasoningSummary?.let { optimistic.setModelReasoningSummary(it) }
        pasteBurst?.let { optimistic.setDisablePasteBurst(it) }
        val next = optimistic.build()
        settings = next
        settingsError = null
        // 【双写:daemon + 本地】daemon 的 codex 设置决定电脑侧默认模型;
        // 但侧边栏底部、会话页模型条读的是 AgentConnection.selectedModel
        // —— 两个存储原本互不通信,导致"设置选了 DeepSeek、侧栏还显示跟随桌面配置"。
        // 这里把最终值同步到本地一份,新会话按它初始化。
        model?.let { AgentConnection.setGlobalModel(it) }
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
                            .setHideAgentReasoning(next.hideAgentReasoning)
                            .setModelVerbosity(next.modelVerbosity)
                            .setModelReasoningSummary(next.modelReasoningSummary)
                            .setDisablePasteBurst(next.disablePasteBurst)
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

    /**
     * AgentPage 用的写入器。注意 [CodexWriter.onCommit] 必须在每次重组时
     * 重新指向最新的 [writeCodex] —— 它闭包里捕获了 `settings` 快照,
     * 一旦漏更新,连点两次设置会以过期快照做乐观更新。
     */
    val codexWriter = remember { CodexWriter() }
    codexWriter.onCommit = ::writeCodex

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
                withContext(Dispatchers.IO) { AgentConnection.connectWithFallback(freshChannel = true) }
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

    // ---------------- 二级页路由(不引入导航组件,设置页自洽) ----------------
    var subPage by remember { mutableStateOf<String?>(if (openUserData) "userdata" else null) }

    // 返回层级:二级页 → 一级页;一级页 → 退出设置(此前物理返回直接退出,层级断裂)
    androidx.activity.compose.BackHandler(enabled = true) {
        if (subPage != null) subPage = null else onBack()
    }

    if (subPage == "userdata") {
        UserDataPage(
            onBack = { subPage = null },
            addr = addr,
            onAddr = { addr = it },
            token = token,
            onToken = { token = it },
            connectionSaving = connectionSaving,
            errorMessage = connectionError?.message,
            errorAddress = connectionError?.address.orEmpty(),
            onSaveAndTest = { saveAndTestConnection() },
            onSaveOnly = { saveCredentialsOnly() },
            onRescan = { rescanCandidates() },
            onOpenBrowser = onOpenBrowser,
        )
        return
    }
    if (subPage == "appearance") {
        AppearancePage(onBack = { subPage = null }, onFeedback = { showSettingsFeedback(it) })
        return
    }
    if (subPage == "agent") {
        AgentPage(
            onBack = { subPage = null },
            runningAgent = runningAgent,
            settings = settings,
            modelChoices = modelChoices,
            mcpList = mcpList,
            mcpLoading = mcpLoading,
            codexUsage = codexUsage,
            usageLoading = usageLoading,
            capabilities = capabilities,
            capabilitiesLoading = capabilitiesLoading,
            capabilitiesError = capabilitiesError,
            host = host,
            writeCodex = codexWriter,
            onFeedback = { showSettingsFeedback(it) },
            onShowModelPicker = { showModelPicker = true },
            onShowApprovalPreview = { showApprovalPreview = true },
        )
        // 【必须在 return 之前渲染】AgentPage 是二级页,上面 return 之后下面的
        // 一级页渲染点永远不会执行 —— 预览按钮点了没反应/要退出二级页才冒出来的根因。
        if (showApprovalPreview) {
            ApprovalPreviewSheet(onDismiss = { showApprovalPreview = false })
        }
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
        return
    }

    val connStateTop = AgentConnection.connState.collectAsState().value
    val connTransportTop = AgentConnection.connTransport.collectAsState().value
    val connSummary = when (connStateTop) {
        "ok" -> "已连接 · " + connTransportTop.ifEmpty { "直连" }
        "down" -> "未连接"
        else -> "检查中"
    }
    val themeLabel = listOf(
        "system" to "跟随系统", "light" to "浅色", "dark" to "深色",
    ).firstOrNull { it.first == AgentConnection.themeMode }?.second ?: "跟随系统"
    val uiScaleLabel = when {
        AgentConnection.uiTextScale < 0.95f -> "小"
        AgentConnection.uiTextScale > 1.05f -> "大"
        else -> "标准"
    }

    // ---------------- 一级页 ----------------
    Column(
        modifier
            .fillMaxSize()
            .background(Pa.Bg)
            .statusBarsPadding(),
    ) {
        Row(
            Modifier.fillMaxWidth().padding(start = 12.dp, end = 16.dp, top = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconActionButton(R.drawable.ic_lucide_chevron_left, "返回", onBack, bg = Pa.Card)
            Column(Modifier.fillMaxWidth().padding(start = 10.dp)) {
                Text("设置", fontSize = 26.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold)
            }
        }

        LazyColumn(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .navigationBarsPadding(),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 14.dp, bottom = 20.dp),
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

            // ---------- 电脑连接卡(点进去 = 用户数据管理) ----------
            item(key = "hero") {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable(haptic = true) { subPage = "userdata" }
                        .background(Pa.Card, RoundedCornerShape(18.dp))
                        .padding(horizontal = 14.dp, vertical = 14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Box(
                        Modifier
                            .size(46.dp)
                            .background(
                                if (connStateTop == "ok") Pa.AccentDim else Pa.Hover,
                                RoundedCornerShape(14.dp),
                            ),
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(
                            painterResource(R.drawable.ic_lucide_monitor), null,
                            tint = if (connStateTop == "ok") Pa.Accent else Pa.TextSecondary,
                            modifier = Modifier.size(22.dp),
                        )
                    }
                    Spacer(Modifier.width(12.dp))
                    Column(Modifier.weight(1f)) {
                        Text(
                            host?.displayName?.ifBlank { "我的电脑" } ?: "我的电脑",
                            fontSize = 15.5.sp,
                            color = Pa.TextPrimary,
                            fontWeight = FontWeight.SemiBold,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Text(
                            connSummary +
                                (host?.os?.takeIf { it.isNotBlank() }?.let { " · $it" } ?: ""),
                            Modifier.padding(top = 2.dp),
                            fontSize = 11.5.sp,
                            color = Pa.TextTertiary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    Icon(
                        painterResource(R.drawable.ic_lucide_chevron_right), null,
                        tint = Pa.TextTertiary, modifier = Modifier.size(16.dp),
                    )
                }
            }

            // ---------- 通用 ----------
            item(key = "general") {
                Column {
                    GroupLabel(R.drawable.ic_lucide_sliders, "通用")
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .background(Pa.Card, RoundedCornerShape(16.dp))
                            .padding(horizontal = 14.dp),
                    ) {
                        SettingRow(
                            title = "外观与阅读",
                            icon = R.drawable.ic_lucide_monitor,
                            value = themeLabel + " · " + uiScaleLabel,
                            chevron = true,
                            divider = true,
                            onClick = { subPage = "appearance" },
                        )
                        SettingRow(
                            title = "用户数据管理",
                            desc = "连接 · 下载内容",
                            icon = R.drawable.ic_lucide_server,
                            value = connSummary,
                            chevron = true,
                            onClick = { subPage = "userdata" },
                        )
                    }
                }
            }

            // ---------- Agent ----------
            item(key = "agent") {
                Column {
                    GroupLabel(R.drawable.ic_lucide_cpu, "Agent")
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .background(Pa.Card, RoundedCornerShape(16.dp))
                            .padding(horizontal = 14.dp),
                    ) {
                        SettingRow(
                            title = "Agent 与模型",
                            desc = "切换 Agent · 权限 · 模型 · MCP",
                            icon = R.drawable.ic_lucide_cpu,
                            value = agentDisplay(runningAgent),
                            chevron = true,
                            onClick = { subPage = "agent" },
                        )
                    }
                }
            }

            // ---------- 关于 ----------
            item(key = "about") {
                Column {
                    GroupLabel(R.drawable.ic_lucide_info, "关于")
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .background(Pa.Card, RoundedCornerShape(16.dp))
                            .padding(horizontal = 14.dp),
                    ) {
                        SettingRow(
                            title = "版本",
                            icon = R.drawable.ic_lucide_info,
                            value = "v0.1.0",
                            divider = true,
                        )
                        SettingRow(
                            title = "电脑",
                            icon = R.drawable.ic_lucide_monitor,
                            value = listOfNotNull(host?.os, host?.arch).joinToString("/").ifBlank { "—" },
                        )
                    }
                }
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

/** 卡片外的灰字分组标题(小图标 + 12sp 灰字)。 */
@Composable
private fun GroupLabel(icon: Int, title: String) {
    Row(
        Modifier.padding(start = 14.dp, top = 18.dp, bottom = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(icon), null,
            tint = Pa.TextSecondary,
            modifier = Modifier.size(15.dp),
        )
        Spacer(Modifier.width(7.dp))
        Text(
            title,
            fontSize = 13.sp,
            color = Pa.TextSecondary,
            fontWeight = FontWeight.SemiBold,
        )
    }
}

/** 二级页骨架:返回 + 标题 + 可滚动内容。 */
@Composable
private fun SubPageScaffold(
    title: String,
    onBack: () -> Unit,
    content: @Composable androidx.compose.foundation.layout.ColumnScope.() -> Unit,
) {
    Column(Modifier.fillMaxSize().background(Pa.Bg)) {
        Row(
            Modifier
                .fillMaxWidth()
                .statusBarsPadding()
                .padding(start = 12.dp, end = 16.dp, top = 8.dp, bottom = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconActionButton(R.drawable.ic_lucide_chevron_left, "返回", onBack, bg = Pa.Card)
            Spacer(Modifier.width(10.dp))
            Text(title, fontSize = 20.sp, fontWeight = FontWeight.Bold, color = Pa.TextPrimary)
        }
        Column(
            Modifier
                .weight(1f)
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .navigationBarsPadding()
                .padding(horizontal = 16.dp, vertical = 6.dp),
            content = content,
        )
    }
}

/**
 * 二级页:外观与阅读 —— 字号 / 思考 / 工具详情 / 显示模式。
 */
@Composable
private fun AppearancePage(
    onBack: () -> Unit,
    onFeedback: (String) -> Unit,
) {
    SubPageScaffold(title = "外观与阅读", onBack = onBack) {
        SectionBlock(icon = R.drawable.ic_lucide_monitor, title = "字号") {
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
                    onFeedback("界面字号已保存")
                },
                options = uiOptions.map { (_, label, v) ->
                    SplitOption(
                        title = label,
                        selected = kotlin.math.abs(v - AgentConnection.uiTextScale) < 0.02f,
                    ) {
                        AgentConnection.setTextScale(v, AgentConnection.thinkTextScale)
                        onFeedback("界面字号已保存")
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
                    onFeedback("思考字号已保存")
                },
                options = thinkOptions.map { (_, label, v) ->
                    SplitOption(
                        title = label,
                        selected = kotlin.math.abs(v - AgentConnection.thinkTextScale) < 0.02f,
                    ) {
                        AgentConnection.setTextScale(AgentConnection.uiTextScale, v)
                        onFeedback("思考字号已保存")
                    }
                },
            )
            Spacer(Modifier.height(6.dp))
        }

        SectionBlock(icon = R.drawable.ic_lucide_brain, title = "对话内容") {
            Spacer(Modifier.height(10.dp))
            // 显示思考过程(展示/隐藏)
            SplitButton(
                label = "显示思考过程",
                mainLabel = if (AgentConnection.showThinking) "展示" else "隐藏",
                onMainClick = {
                    AgentConnection.showThinking = !AgentConnection.showThinking
                    onFeedback(
                        if (AgentConnection.showThinking) "将流式显示思考过程" else "将只显示对话结果",
                    )
                },
                options = listOf(
                    SplitOption("展示", selected = AgentConnection.showThinking) {
                        AgentConnection.showThinking = true
                        onFeedback("将流式显示思考过程")
                    },
                    SplitOption("隐藏", selected = !AgentConnection.showThinking) {
                        AgentConnection.showThinking = false
                        onFeedback("将只显示对话结果")
                    },
                ),
            )
            Spacer(Modifier.height(8.dp))

            // 工具详情(折叠/展开)
            SplitButton(
                label = "工具详情",
                mainLabel = if (AgentConnection.toolDetailsExpanded) "展开" else "折叠",
                onMainClick = {
                    AgentConnection.toolDetailsExpanded = !AgentConnection.toolDetailsExpanded
                    onFeedback("工具详情设置已保存")
                },
                options = listOf(
                    SplitOption(
                        "折叠", selected = !AgentConnection.toolDetailsExpanded,
                    ) {
                        AgentConnection.toolDetailsExpanded = false
                        onFeedback("工具详情设置已保存")
                    },
                    SplitOption(
                        "展开", selected = AgentConnection.toolDetailsExpanded,
                    ) {
                        AgentConnection.toolDetailsExpanded = true
                        onFeedback("工具详情设置已保存")
                    },
                ),
            )
            Spacer(Modifier.height(6.dp))
        }

        SectionBlock(icon = R.drawable.ic_lucide_sliders, title = "显示模式") {
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
                    AgentConnection.themeMode = next.first
                    AgentTheme.mode = next.first
                    onFeedback("显示模式已保存")
                },
                options = modeOptions.map { (id, name, desc) ->
                    SplitOption(
                        title = name,
                        desc = desc,
                        selected = AgentConnection.themeMode == id,
                    ) {
                        AgentConnection.themeMode = id
                        AgentTheme.mode = id
                        onFeedback("显示模式已保存")
                    }
                },
            )
            Spacer(Modifier.height(6.dp))
        }
        Spacer(Modifier.height(20.dp))
    }
}

/**
 * 二级页:Agent 与模型 —— 切换 Agent + 当前 agent(codex)的专属设置。
 */
/**
 * Codex 设置写入器。
 *
 * 【为什么用类而不是函数类型】Kotlin 的**函数类型形参不允许命名实参**
 * ("Named arguments are prohibited for function types"),而这里从 4 个参数
 * 涨到 8 个之后,`writeCodex(null, null, "gpt-5", null)` 这种全 null 位置实参
 * 已经无法看出在改哪一项 —— 极容易把 effort 写进 model 位。
 * 换成一个带具名方法的类,调用处就能写 `writeCodex.model(id)` /
 * `writeCodex.effort(id)`,编译期就不会串位。
 */
class CodexWriter(
    private val approval: String? = null,
    private val sandbox: String? = null,
    private val model: String? = null,
    private val effort: String? = null,
    private val hideReasoning: Boolean? = null,
    private val verbosity: String? = null,
    private val reasoningSummary: String? = null,
    private val pasteBurst: Boolean? = null,
) {
    /** 显式构造一个"只改某一项"的写入器;其余项带 null 表示不动。 */
    fun copy(
        approval: String? = this.approval,
        sandbox: String? = this.sandbox,
        model: String? = this.model,
        effort: String? = this.effort,
        hideReasoning: Boolean? = this.hideReasoning,
        verbosity: String? = this.verbosity,
        reasoningSummary: String? = this.reasoningSummary,
        pasteBurst: Boolean? = this.pasteBurst,
    ) = CodexWriter(
        approval, sandbox, model, effort,
        hideReasoning, verbosity, reasoningSummary, pasteBurst,
    )

    /**
     * 真正落库。全参数为 null 表示"不改任何一项" —— 与既有四件套同一套语义,
     * 保证不会把用户没碰过的设置清空。
     */
    fun commit() = onCommit(
        approval, sandbox, model, effort,
        hideReasoning, verbosity, reasoningSummary, pasteBurst,
    )

    /** 由调用方注入的实际写入实现(乐观更新 + gRPC + 失败回滚)。 */
    var onCommit: (
        String?, String?, String?, String?,
        Boolean?, String?, String?, Boolean?,
    ) -> Unit = { _, _, _, _, _, _, _, _ -> }
}

@Composable
private fun AgentPage(
    onBack: () -> Unit,
    runningAgent: String,
    settings: CodexSettings?,
    modelChoices: List<ModelInfo>,
    mcpList: List<McpServerStatus>,
    mcpLoading: Boolean,
    codexUsage: CodexUsage?,
    usageLoading: Boolean,
    capabilities: List<CodexCapability>,
    capabilitiesLoading: Boolean,
    capabilitiesError: String?,
    host: HostInfo?,
    writeCodex: CodexWriter,
    onFeedback: (String) -> Unit,
    onShowModelPicker: () -> Unit,
    onShowApprovalPreview: () -> Unit,
) {
    SubPageScaffold(title = "Agent 与模型", onBack = onBack) {
        SectionBlock(icon = R.drawable.ic_lucide_cpu, title = "切换 Agent") {
            Spacer(Modifier.height(10.dp))
            val agentChoices = listOf("codex" to "Codex") + OTHER_AGENT_OPTIONS
            agentChoices.forEachIndexed { i, (id, name) ->
                SettingRow(
                    title = name,
                    desc = if (runningAgent == id) "当前运行中" else "点击切换到此 Agent",
                    // 品牌图标原样显示(不染灰),尺寸大一档与 14sp 标题对齐
                    icon = agentIcon(id),
                    iconTint = null,
                    iconSize = 22.dp,
                    chevron = true,
                    divider = i != agentChoices.lastIndex,
                    onClick = {
                        if (runningAgent != id) {
                            AgentConnection.switchAgent(id)
                            onFeedback("已切换到 ${agentDisplay(id)}")
                        }
                    },
                )
            }
        }

        if (runningAgent == "codex") {
            SectionBlock(icon = R.drawable.ic_lucide_shield, title = "权限与安全") {
                Spacer(Modifier.height(10.dp))
                // 托管策略:被 requirements.toml 约束时,只渲染白名单里的取值。
                // 读取/写入都还是原字段,只是可选项被收窄。
                val allowedApproval = settings?.allowedApprovalPoliciesList.orEmpty()
                val allowedSandbox = settings?.allowedSandboxModesList.orEmpty()
                if (settings?.managed == true) {
                    ManagedPolicyNote(
                        settings.managedNote.ifBlank {
                            "这台电脑上的 Codex 受管理员策略约束,部分选项已被限制。"
                        },
                    )
                    Spacer(Modifier.height(10.dp))
                }

                // 审批策略
                val approvalOptions = filterByManagedPolicy(
                    listOf(
                        Triple("untrusted", "仅信任", "只自动执行白名单命令"),
                        Triple("on-request", "按需询问", "运行命令前先征求批准"),
                        Triple("never", "完全访问", "不询问,直接执行"),
                    ),
                    allowedApproval,
                ) { it.first }
                if (approvalOptions.isEmpty()) {
                    // 白名单里的取值我们一个都不认识(codex 加了新的策略档位)。
                    // 这种情况给一个不可点的只读行,而不是让下面的下标访问崩掉。
                    SettingRow(
                        title = "审批策略",
                        desc = "当前取值 ${settings?.approvalPolicy.orEmpty()} 不在 App 支持的选项内",
                        divider = true,
                    )
                } else {
                    // 【不能 coerceAtLeast(0)】codex 没报告这个值时(桌面未显式配置,
                    // 用 codex 自己的默认),退回列表第一项会让界面显示"仅信任" ——
                    // 用户会以为自己已经处在那个模式里,而实际生效的是别的值。
                    // 取不到就如实写"跟随桌面",并且一个选项都不标选中。
                    val approvalIdx = approvalOptions
                        .indexOfFirst { it.first == settings?.approvalPolicy }
                    val approvalLabel = if (approvalIdx >= 0) {
                        approvalOptions[approvalIdx].second
                    } else {
                        "跟随桌面"
                    }
                    SplitButton(
                        label = "审批策略",
                        mainLabel = approvalLabel,
                        onMainClick = {
                            val next = approvalOptions[if (approvalIdx >= 0) (approvalIdx + 1) % approvalOptions.size else 0]
                            writeCodex.copy(approval = next.first).commit()
                        },
                        options = approvalOptions.map { (id, label, desc) ->
                            SplitOption(
                                title = label,
                                desc = desc,
                                selected = settings?.approvalPolicy == id,
                            ) { writeCodex.copy(approval = id).commit() }
                        },
                    )
                }
                Spacer(Modifier.height(8.dp))

                // 沙箱模式
                val sandboxOptions = filterByManagedPolicy(
                    listOf(
                        Triple("read-only", "只读", "只能读取文件"),
                        Triple("workspace-write", "工作区可写", "仅工作区内可写"),
                        Triple("danger-full-access", "完全访问", "不限制文件与网络"),
                    ),
                    allowedSandbox,
                ) { it.first }
                if (sandboxOptions.isEmpty()) {
                    SettingRow(
                        title = "沙箱模式",
                        desc = "当前取值 ${settings?.sandbox.orEmpty()} 不在 App 支持的选项内",
                        divider = true,
                    )
                } else {
                    // 同上:取值未知时如实显示"跟随桌面",不假装选中第一项。
                    val sandboxIdx = sandboxOptions
                        .indexOfFirst { it.first == settings?.sandbox }
                    val sandboxLabel = if (sandboxIdx >= 0) {
                        sandboxOptions[sandboxIdx].second
                    } else {
                        "跟随桌面"
                    }
                    SplitButton(
                        label = "沙箱模式",
                        mainLabel = sandboxLabel,
                        onMainClick = {
                            val next = sandboxOptions[if (sandboxIdx >= 0) (sandboxIdx + 1) % sandboxOptions.size else 0]
                            writeCodex.copy(sandbox = next.first).commit()
                        },
                        options = sandboxOptions.map { (id, label, desc) ->
                            SplitOption(
                                title = label,
                                desc = desc,
                                selected = settings?.sandbox == id,
                            ) { writeCodex.copy(sandbox = id).commit() }
                        },
                    )
                }
                Spacer(Modifier.height(10.dp))

                Box(
                    Modifier
                        .fillMaxWidth()
                        .pressable(haptic = true) { onShowApprovalPreview() }
                        .background(Pa.Bg, RoundedCornerShape(13.dp))
                        .padding(vertical = 12.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text("预览审批弹窗", fontSize = 13.5.sp, color = Pa.TextPrimary)
                }
                Spacer(Modifier.height(6.dp))
            }

            SectionBlock(icon = R.drawable.ic_lucide_cpu, title = "模型与思考") {
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
                            onFeedback("未获取到模型列表,请检查连接")
                        } else {
                            onShowModelPicker()
                        }
                    },
                    options = buildList {
                        add(
                            SplitOption(
                                "跟随桌面配置",
                                "使用电脑上当前 agent 的默认模型",
                                selected = settings?.model.isNullOrEmpty(),
                            ) { writeCodex.copy(model = "").commit() },
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
                                    ) { writeCodex.copy(model = m.id).commit() },
                                )
                            }
                        }
                    },
                )
                Spacer(Modifier.height(8.dp))

                // 思考强度
                // 【第 27 轮补齐】官方枚举还有 none(完全关闭推理,最快)
                // 与 max(比 xhigh 更强)。这两档是否可用取决于模型,
                // 所以文案里点明"需模型支持",避免用户选了没反应以为是 bug。
                val effortOptions = listOf(
                    Triple("none", "关闭", "不做推理,最快(需模型支持)"),
                    Triple("low", "低", "最快,浅推理"),
                    Triple("medium", "中", "速度与深度平衡"),
                    Triple("high", "高", "更慢,更细"),
                    Triple("xhigh", "极高", "最慢,最细"),
                    Triple("max", "最高", "超过极高(需模型支持)"),
                )
                val effortIdx = effortOptions
                    .indexOfFirst { it.first == settings?.effort }
                    .coerceAtLeast(0)
                SplitButton(
                    label = "思考强度",
                    mainLabel = effortOptions[effortIdx].second,
                    onMainClick = {
                        val next = effortOptions[(effortIdx + 1) % effortOptions.size]
                        writeCodex.copy(effort = next.first).commit()
                    },
                    options = effortOptions.map { (id, label, desc) ->
                        SplitOption(
                            title = label,
                            desc = desc,
                            selected = settings?.effort == id,
                        ) { writeCodex.copy(effort = id).commit() }
                    },
                )
                Spacer(Modifier.height(6.dp))
            }

            SectionBlock(icon = R.drawable.ic_lucide_sparkles, title = "行为与降噪") {
                Spacer(Modifier.height(10.dp))
                // 【边界说明】这些偏好只作用于"从 App 发起的任务"。
                // daemon 把它们写进自己拉起的 app-server 子进程内存态,
                // 不会改写用户桌面 codex 的 config.toml。
                Text(
                    "仅影响从 App 发起的任务,不改动电脑上的 Codex 配置。",
                    fontSize = 11.sp, color = Pa.TextTertiary,
                    modifier = Modifier.padding(bottom = 10.dp),
                )

                SettingRow(
                    title = "隐藏推理过程",
                    desc = "不再推送思考事件,只留结论(手机端刷屏最少)",
                    divider = true,
                    control = {
                        Switch(
                            checked = settings?.hideAgentReasoning == true,
                            onCheckedChange = { writeCodex.copy(hideReasoning = it).commit() },
                            colors = SwitchDefaults.colors(
                                checkedTrackColor = Pa.Accent,
                                uncheckedTrackColor = Pa.Hover,
                                uncheckedBorderColor = Color.Transparent,
                            ),
                        )
                    },
                )

                val verbosityOptions = listOf(
                    Triple("low", "精简", "直给结论,少展开解释"),
                    Triple("medium", "适中", "默认,结论与说明平衡"),
                    Triple("high", "详细", "展开更多背景与推导"),
                )
                val verbosityIdx = verbosityOptions
                    .indexOfFirst { it.first == settings?.modelVerbosity }
                    .coerceAtLeast(0)
                SplitButton(
                    label = "回答详略",
                    mainLabel = verbosityOptions[verbosityIdx].second,
                    onMainClick = {
                        val next = verbosityOptions[(verbosityIdx + 1) % verbosityOptions.size]
                        writeCodex.copy(verbosity = next.first).commit()
                    },
                    options = verbosityOptions.map { (id, label, desc) ->
                        SplitOption(
                            title = label,
                            desc = desc,
                            selected = settings?.modelVerbosity == id,
                        ) { writeCodex.copy(verbosity = id).commit() }
                    },
                )
                Spacer(Modifier.height(8.dp))

                val summaryOptions = listOf(
                    Triple("concise", "精简", "思考摘要只留关键结论"),
                    Triple("detailed", "详细", "保留较完整的推理脉络"),
                )
                val summaryIdx = summaryOptions
                    .indexOfFirst { it.first == settings?.modelReasoningSummary }
                    .coerceAtLeast(0)
                SplitButton(
                    label = "思考摘要",
                    mainLabel = summaryOptions[summaryIdx].second,
                    onMainClick = {
                        val next = summaryOptions[(summaryIdx + 1) % summaryOptions.size]
                        writeCodex.copy(reasoningSummary = next.first).commit()
                    },
                    options = summaryOptions.map { (id, label, desc) ->
                        SplitOption(
                            title = label,
                            desc = desc,
                            selected = settings?.modelReasoningSummary == id,
                        ) { writeCodex.copy(reasoningSummary = id).commit() }
                    },
                )
                Spacer(Modifier.height(8.dp))

                SettingRow(
                    title = "粘贴长文本不中断",
                    desc = "粘贴大段代码时不触发爆破检测,避免被截断",
                    divider = false,
                    control = {
                        Switch(
                            checked = settings?.disablePasteBurst == true,
                            onCheckedChange = { writeCodex.copy(pasteBurst = it).commit() },
                            colors = SwitchDefaults.colors(
                                checkedTrackColor = Pa.Accent,
                                uncheckedTrackColor = Pa.Hover,
                                uncheckedBorderColor = Color.Transparent,
                            ),
                        )
                    },
                )
                Spacer(Modifier.height(6.dp))
            }

            SectionBlock(icon = R.drawable.ic_lucide_server, title = "用量与限流") {
                Spacer(Modifier.height(10.dp))
                CodexUsageBody(usage = codexUsage, loading = usageLoading)
                Spacer(Modifier.height(6.dp))
            }

            SectionBlock(icon = R.drawable.ic_lucide_sparkles, title = "能力 · 技能与插件") {
                // 数量过多时默认只展示前 DEFAULT_VISIBLE_PER_KIND 条,避免 29 个技能
                // 把整页铺满;被折叠的条目点"展开更多"用底部弹窗看全部。
                val DEFAULT_VISIBLE_PER_KIND = 3
                val EXPAND_SHEET_MAX_HEIGHT = 420.dp
                // 放在分支前:确保「读取中 / 为空」的提前 return 不会把它跳过。
                var showAllCapabilities by remember { mutableStateOf(false) }

                Spacer(Modifier.height(10.dp))
                if (capabilities.isEmpty()) {
                    // 四态必须分清:还在读 / 读失败 / 读完确实没有 / 有数据。
                    // 前两个都不能渲染成最后一个 —— "还在读"说成"未报告任何",
                    // 用户以为技能没装;"读失败"说成"未报告任何",用户会去重装技能,
                    // 白折腾一圈。这块 UI 最容易漏的就是第四个状态。
                    val emptyText = when {
                        capabilitiesLoading -> "正在读取…"
                        capabilitiesError != null -> "暂时读不到能力列表：" + capabilitiesError
                        else -> "这台电脑上的 Codex 未报告任何技能、hooks 或插件"
                    }
                    Text(
                        emptyText,
                        fontSize = 12.sp, color = Pa.TextTertiary,
                        modifier = Modifier.padding(vertical = 12.dp),
                    )
                } else {
                    val byKind = capabilities.groupBy { it.kind }
                    // 顺序固定:技能 → hooks → 插件。用 codex 返回的顺序会随
                    // 目录扫描结果抖动,同一台机器每次打开位置都不一样。
                    val kindsOrder = listOf("skill" to "技能", "hook" to "Hooks", "plugin" to "插件")
                    // 弹窗要列出所有分类的全部条目,提前算好总数供标题复用。
                    val totalCapabilityCount = byKind.values.sumOf { it.size }

                    // 折叠列表与弹窗共用同一套条目渲染(含"为什么用不了 / 已停用")。
                    @Composable
                    fun CapabilityItemRow(c: CodexCapability, divider: Boolean) {
                        // 描述优先给"为什么用不了":enabled=false 却不说明原因,
                        // 用户只会觉得"我装的技能没生效"。三项都空就整行留白,
                        // 不硬凑一句废话。
                        val desc = listOfNotNull(
                            c.disabledReason.takeIf { it.isNotBlank() },
                            c.description.takeIf { it.isNotBlank() },
                            c.source.takeIf { it.isNotBlank() }?.let { "来源 $it" },
                        ).firstOrNull().orEmpty()
                        // control 是必填的 RowScope 槽位(默认空实现),所以
                        // 这里不做"传 null 表示没有控件",而是在槽位内部判断。
                        SettingRow(
                            title = c.name.ifBlank { "(未命名)" },
                            desc = desc,
                            divider = divider,
                            control = {
                                if (!c.enabled) {
                                    Text("已停用", fontSize = 11.sp, color = Pa.TextTertiary)
                                }
                            },
                        )
                    }

                    kindsOrder.forEach { (kind, label) ->
                        val items = byKind[kind].orEmpty()
                        if (items.isEmpty()) return@forEach
                        Text(
                            "$label · ${items.size}",
                            fontSize = 11.sp,
                            color = Pa.TextTertiary,
                            modifier = Modifier.padding(bottom = 4.dp),
                        )
                        val visible = items.take(DEFAULT_VISIBLE_PER_KIND)
                        val hasMore = items.size > DEFAULT_VISIBLE_PER_KIND
                        visible.forEachIndexed { i, c ->
                            // 折叠视图里最后一条可见项:若还有被折叠的条目,保留
                            // 分隔线与"展开更多"行衔接;否则按原逻辑(非末项才分隔)。
                            val isLastVisible = i == visible.lastIndex
                            CapabilityItemRow(c, divider = !(isLastVisible && !hasMore))
                        }
                        if (hasMore) {
                            SettingRow(
                                title = "展开更多(${items.size - DEFAULT_VISIBLE_PER_KIND})",
                                desc = "",
                                divider = false,
                                chevron = true,
                                onClick = { showAllCapabilities = true },
                            )
                        }
                        Spacer(Modifier.height(8.dp))
                    }

                    // 弹窗:列出所有分类的全部条目,内容可上下滑动(用 LazyColumn,勿用 Column 硬塞)。
                    if (showAllCapabilities) {
                        androidx.compose.material3.AlertDialog(
                            onDismissRequest = { showAllCapabilities = false },
                            containerColor = Pa.CardHigh,
                            title = {
                                Text(
                                    "技能与插件($totalCapabilityCount 个)",
                                    color = Pa.TextPrimary, fontSize = 17.sp,
                                )
                            },
                            text = {
                                LazyColumn(
                                    Modifier
                                        .fillMaxWidth()
                                        .heightIn(max = EXPAND_SHEET_MAX_HEIGHT),
                                ) {
                                    kindsOrder.forEach { (kind, label) ->
                                        val items = byKind[kind].orEmpty()
                                        if (items.isEmpty()) return@forEach
                                        item {
                                            Text(
                                                "$label · ${items.size}",
                                                fontSize = 11.sp,
                                                color = Pa.TextTertiary,
                                                modifier = Modifier.padding(vertical = 4.dp),
                                            )
                                        }
                                        items.forEachIndexed { i, c ->
                                            item {
                                                CapabilityItemRow(c, divider = i != items.lastIndex)
                                            }
                                        }
                                    }
                                }
                            },
                            confirmButton = {
                                TextButton(onClick = { showAllCapabilities = false }) {
                                    Text("收起", color = Pa.AccentText, fontSize = 15.sp)
                                }
                            },
                        )
                    }
                }
            }

            SectionBlock(icon = R.drawable.ic_lucide_server, title = "集成 · MCP 服务器") {
                if (mcpList.isEmpty()) {
                    Text(
                        if (mcpLoading) "正在读取…" else "未获取到 MCP 列表",
                        fontSize = 12.sp, color = Pa.TextTertiary,
                        modifier = Modifier.padding(vertical = 12.dp),
                    )
                } else {
                    mcpList.forEachIndexed { index, m ->
                        SettingRow(
                            title = m.serverTitle.ifBlank { m.name },
                            // 只显示工具数的话,"某个 MCP 用不了"在手机上完全看不出来。
                            // 鉴权状态直接回答是不是授权问题,错误原因直接给出失败理由。
                            desc = buildString {
                                append("${m.tools} 个工具")
                                if (m.authStatus.isNotBlank() && m.authStatus != "unsupported") {
                                    append(" · 鉴权 ${m.authStatus}")
                                }
                                if (m.resourceCount > 0) append(" · ${m.resourceCount} 个资源")
                                if (m.serverVersion.isNotBlank()) append(" · v${m.serverVersion}")
                                if (m.error.isNotBlank()) append(" · ${m.error}")
                            },
                            divider = index != mcpList.lastIndex,
                            control = {
                                val ok = m.status == "ready" && m.error.isBlank()
                                StatusChip(
                                    text = if (ok) "就绪" else if (m.error.isNotBlank()) "失败" else m.status,
                                    color = if (ok) Pa.Green else Pa.Red,
                                )
                            },
                        )
                    }
                }
            }
        } else {
            SectionBlock(icon = R.drawable.ic_lucide_info, title = agentDisplay(runningAgent)) {
                Text(
                    "${agentDisplay(runningAgent)} 的模型、权限与工具设置将在接入完成后在此显示；" +
                        "可先在上方「切换 Agent」里回到 Codex。",
                    fontSize = 12.sp, color = Pa.TextTertiary,
                    modifier = Modifier.padding(vertical = 12.dp),
                )
            }
        }

        SectionBlock(icon = R.drawable.ic_lucide_user, title = "Codex 账户") {
            SettingRow(
                title = host?.displayName ?: "…",
                desc = "已登录 · 模型与用量走桌面端账号",
            )
        }
        Spacer(Modifier.height(20.dp))
    }
}

/** 下载任务状态文案。 */
private fun statusLabel(s: TaskCenter.Status): String = when (s) {
    TaskCenter.Status.RUNNING -> "下载中"
    TaskCenter.Status.DONE -> "已完成"
    TaskCenter.Status.FAILED -> "失败"
    TaskCenter.Status.CANCELED -> "已取消"
    TaskCenter.Status.INTERRUPTED -> "已中断"
}

/**
 * 删除手机上的文件。
 * @return 是否真的删掉了(uri 为空视为无可删,返回 true 以免误报;删除异常/未命中返回 false)。
 * 调用方据此决定提示 —— 删不掉还 toast「已删除」是欺骗用户。
 */
private fun deleteLocalFile(ctx: android.content.Context, uri: String): Boolean {
    if (uri.isEmpty()) return true
    return runCatching {
        val parsed = android.net.Uri.parse(uri)
        if (parsed.scheme == "content") {
            ctx.contentResolver.delete(parsed, null, null) > 0
        } else {
            parsed.path?.let { java.io.File(it).delete() } ?: false
        }
    }.getOrDefault(false)
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
 * 设置分组:标题在**卡片外**(灰色小字 + 小图标),卡片内是一行行设置条目。
 * (二级页内部继续沿用这一语言。)
 */
@Composable
fun SectionBlock(
    icon: Int,
    title: String,
    content: @Composable androidx.compose.foundation.layout.ColumnScope.() -> Unit,
) {
    Column(Modifier.fillMaxWidth()) {
        GroupLabel(icon, title)
        Column(
            Modifier
                .fillMaxWidth()
                .background(Pa.Card, RoundedCornerShape(16.dp))
                .padding(horizontal = 14.dp, vertical = 2.dp),
        ) {
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
    /** 行首小图标(参考主流设置页:每行一个线性图标,便于扫读)。 */
    icon: Int? = null,
    /** 图标着色。传 null 表示原样显示(用于 Agent 品牌 PNG,不能被染成灰色)。 */
    iconTint: Color? = Pa.TextSecondary,
    /** 图标尺寸;品牌图标比 17dp 的线性图标大一档才协调。 */
    iconSize: androidx.compose.ui.unit.Dp = 17.dp,
    /** 右侧"当前值"文字(如 深色 / 简体中文 / v0.1.0),点击行时一眼看到现状。 */
    value: String? = null,
    /** 行尾 chevron,表示"点进去还有内容"。 */
    chevron: Boolean = false,
    /** 传了就是可点击的导航行。 */
    onClick: (() -> Unit)? = null,
    /** 【显式命名】加个名字,Switch 这种"当前值即控件"的开关可以在语义上
     *  顶掉右侧 value 文字位,不会被 Compose 误判成两个挨着的内容。 */
    control: @Composable RowScope.() -> Unit = {},
) {

    Column {
        Row(
            Modifier
                .fillMaxWidth()
                .then(if (onClick != null) Modifier.pressable(haptic = true) { onClick() } else Modifier)
                .padding(vertical = 11.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (icon != null) {
                Icon(
                    painterResource(icon), null,
                    tint = iconTint ?: Color.Unspecified,
                    modifier = Modifier.size(iconSize),
                )
                Spacer(Modifier.width(12.dp))
            }
            Column(Modifier.weight(1f)) {
                Text(title, fontSize = 15.sp, color = Pa.TextPrimary, maxLines = 1)
                if (desc.isNotEmpty()) {
                    Text(
                        desc,
                        Modifier.padding(top = 2.dp),
                        fontSize = 12.sp,
                        color = Pa.TextTertiary,
                        maxLines = 2,
                    )
                }
            }
            if (value != null) {
                Spacer(Modifier.width(8.dp))
                Text(
                    value,
                    fontSize = 13.5.sp,
                    color = Pa.TextTertiary,
                    maxLines = 1,
                )
            }
            control()
            if (chevron) {
                Spacer(Modifier.width(6.dp))
                Icon(
                    painterResource(R.drawable.ic_lucide_chevron_right), null,
                    tint = Pa.TextTertiary,
                    modifier = Modifier.size(15.dp),
                )
            }
        }
        if (divider) {
            Box(Modifier.fillMaxWidth().height(1.dp).background(Pa.Separator))
        }
    }
}

/**
 * 一条已下载文件在本机的真实状态:是否还在、真实体积(查不到为 null)。
 * 用于在离线情况下诚实区分"能打开 / 已被删除 / 位置未知",绝不装作能打开。
 */
private data class DlFileState(val exists: Boolean, val realSize: Long?)

/**
 * 二级页:用户数据管理 —— 连接地址 + 全部下载内容。
 *
 * 【设计取向】连接编辑与下载内容(查看 / 分享 / 删除)统一收在这里;
 * 文件类型与任务中心用同一套 [fileKindOf] 判定,两处显示不会打架。
 *
 * 【离线可靠】下载列表只来自本地持久化的任务清单(SharedPreferences),不依赖任何 daemon
 * 状态;查看/分享直接用 [PaDownloader] 的本地 content URI,零 daemon 往返。
 */
@Composable
private fun UserDataPage(
    onBack: () -> Unit,
    addr: String,
    onAddr: (String) -> Unit,
    token: String,
    onToken: (String) -> Unit,
    connectionSaving: Boolean,
    errorMessage: String?,
    errorAddress: String,
    onSaveAndTest: () -> Unit,
    onSaveOnly: () -> Unit,
    onRescan: () -> Unit,
    onOpenBrowser: (String) -> Unit,
) {
    val ctx = androidx.compose.ui.platform.LocalContext.current
    val allTasks by TaskCenter.tasks.collectAsState()
    // 【离线可靠】列表只来自本地持久化的任务清单(SharedPreferences),不依赖任何 daemon 状态 ——
    // 断网/连不上时照样渲染,不会变成"0 个文件"。
    val downloads = allTasks.filter { it.kind == TaskCenter.Kind.DOWNLOAD }
    val doneDownloads = downloads.filter { it.status == TaskCenter.Status.DONE }

    // 每条已下载文件的"是否还在 / 真实体积":本地查 MediaStore / 文件系统,零 daemon 往返。
    // 结果进 state,列表据此决定显示「查看/分享」还是「文件已被删除」。
    val dlStates by produceState(emptyMap<String, DlFileState>(), downloads) {
        value = withContext(Dispatchers.IO) {
            doneDownloads.associate { d ->
                val exists = if (d.localUri.isNotEmpty()) PaDownloader.localFileExists(ctx, d.localUri) else false
                val size = if (exists) PaDownloader.queryLocalFileSize(ctx, d.localUri) else null
                d.id to DlFileState(exists, size)
            }
        }
    }

    // 下载占用:优先用"文件真实字节数"(MediaStore 查回),查不到才回退到条目上记录的体积字符串;
    // 两者都没有就记为未知 —— 绝不用 0 冒充"没占用"。
    val downloadBytes = doneDownloads.sumOf { d ->
        val real = dlStates[d.id]?.realSize
        real ?: if (d.sizeText.isNotEmpty()) parseHumanSize(d.sizeText) else null ?: 0L
    }
    val downloadUnknown = doneDownloads.any { d ->
        val real = dlStates[d.id]?.realSize
        (real ?: if (d.sizeText.isNotEmpty()) parseHumanSize(d.sizeText) else null) == null
    }
    val doneCount = doneDownloads.size

    // 旧版本下载条目是 10 字段格式,没有 localUri → 查看/分享图标被门控隐藏。
    // 进页面按文件名反向找回本地路径,找回后 attachLocalUriIfEmpty→persist()
    // 会重新发射列表,图标随之出现。legacyTried 记录已尝试的 id,防止重组时反复查 MediaStore。
    val legacyTried = remember { mutableSetOf<String>() }
    LaunchedEffect(downloads) {
        for (d in downloads) {
            if (d.status != TaskCenter.Status.DONE || d.localUri.isNotEmpty()) continue
            if (!legacyTried.add(d.id)) continue
            val uri = withContext(Dispatchers.IO) { PaDownloader.resolveLocalUri(ctx, d.title) }
            if (uri.isNotEmpty()) TaskCenter.attachLocalUriIfEmpty(d.id, uri)
        }
    }
    val connState = AgentConnection.connState.collectAsState().value
    val transport = AgentConnection.connTransport.collectAsState().value
    val connAddr = AgentConnection.connAddress.collectAsState().value

    SubPageScaffold(title = "用户数据管理", onBack = onBack) {
        SectionBlock(icon = R.drawable.ic_lucide_globe, title = "连接") {
            Row(
                Modifier.fillMaxWidth().padding(vertical = 11.dp),
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
                            "ok" -> "已连接 · " + transport
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
            if (errorMessage != null) {
                Text(
                    errorMessage + (if (errorAddress.isNotEmpty()) "\n地址:" + errorAddress else ""),
                    Modifier
                        .fillMaxWidth()
                        .background(Pa.Bg, RoundedCornerShape(12.dp))
                        .padding(11.dp),
                    fontSize = 11.sp,
                    color = Pa.Red,
                )
                Spacer(Modifier.height(10.dp))
            }
            SettingsTextField(
                value = addr,
                onValueChange = onAddr,
                label = "地址 (host:port)",
                keyboardType = KeyboardType.Uri,
            )
            Spacer(Modifier.height(10.dp))
            SettingsTextField(
                value = token,
                onValueChange = onToken,
                label = "配对 token(未配对留空)",
                password = true,
            )
            Spacer(Modifier.height(12.dp))
            SplitButton(
                label = if (connectionSaving) "连接中…" else "保存并测试连接",
                onMainClick = onSaveAndTest,
                options = listOf(
                    SplitOption("仅保存凭证", "不立即测试,交给自动重连接管") { onSaveOnly() },
                    SplitOption("重新扫描候选地址", "USB → 局域网 → Tailscale 依次尝试") { onRescan() },
                    SplitOption("清空输入", "重置地址与 token") {
                        onAddr(""); onToken("")
                    },
                ),
            )
            Spacer(Modifier.height(6.dp))
        }

        Spacer(Modifier.height(14.dp))

        SectionBlock(
            icon = R.drawable.ic_lucide_download,
            title = "下载内容 · " + doneCount + " 个文件" +
                // 总计:用户问"到底占了我多少空间"。查不到真实体积就如实标「未知」,不用 0 冒充。
                when {
                    downloadUnknown -> " · 占用未知"
                    downloadBytes > 0 -> " · 共 " + humanSize(downloadBytes)
                    else -> ""
                },
        ) {
            if (downloads.isEmpty()) {
                Text(
                    "还没有下载内容。在会话右上角「导出对话」或左上角菜单的任务中心里下载后,会出现在这里。",
                    Modifier.padding(vertical = 14.dp),
                    fontSize = 11.5.sp,
                    color = Pa.TextTertiary,
                )
            } else {
                downloads.forEach { d ->
                    val kind = fileKindOf(d.title)
                    Column {
                        Row(
                            Modifier.fillMaxWidth().padding(vertical = 11.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Box(
                                Modifier
                                    .size(34.dp)
                                    .background(kind.color.copy(alpha = 0.16f), RoundedCornerShape(10.dp)),
                                contentAlignment = Alignment.Center,
                            ) {
                                Icon(
                                    painterResource(kind.iconRes), null,
                                    tint = kind.color,
                                    modifier = Modifier.size(16.dp),
                                )
                            }
                            Spacer(Modifier.width(11.dp))
                            Column(Modifier.weight(1f)) {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Text(
                                        d.title.ifEmpty { "(未命名文件)" },
                                        Modifier.weight(1f, fill = false),
                                        fontSize = 13.sp,
                                        color = Pa.TextPrimary,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                    Spacer(Modifier.width(6.dp))
                                    Text(
                                        kind.label,
                                        Modifier
                                            .background(kind.color.copy(alpha = 0.18f), RoundedCornerShape(50))
                                            .padding(horizontal = 6.dp, vertical = 1.dp),
                                        fontSize = 9.5.sp,
                                        color = kind.color,
                                    )
                                }
                                Text(
                                    listOf(
                                        d.sizeText,
                                        if (d.sourceTitle.isNotEmpty()) "来自「" + d.sourceTitle + "」" else "",
                                        statusLabel(d.status),
                                    ).filter { it.isNotEmpty() }.joinToString("  ·  "),
                                    Modifier.padding(top = 2.dp),
                                    fontSize = 10.5.sp,
                                    color = Pa.TextTertiary,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                )
                                if (d.detail.isNotEmpty()) {
                                    Text(
                                        d.detail,
                                        fontSize = 9.5.sp,
                                        color = Pa.TextTertiary,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                }
                            }
                            val canOpen = d.status == TaskCenter.Status.DONE &&
                                d.localUri.isNotEmpty() && (dlStates[d.id]?.exists ?: false)
                            when {
                                canOpen -> {
                                    // 查看:网页/文本/Markdown/图片 → 内置浏览器;安装包/文档 → 系统打开。
                                    // 直接用本地 content URI,零 daemon 往返,断网也能开。
                                    IconActionButton(
                                        R.drawable.ic_lucide_globe, "查看",
                                        {
                                            PaDownloader.openLocalFile(
                                                ctx = ctx, uri = d.localUri, title = d.title,
                                                detail = d.detail, onOpenInBrowser = onOpenBrowser,
                                            )
                                        },
                                        bg = Color.Transparent, size = 32.dp, iconSize = 15.dp,
                                    )
                                    IconActionButton(
                                        R.drawable.ic_lucide_share_2, "分享",
                                        { PaDownloader.shareLocalFile(ctx, d.localUri, d.title) },
                                        bg = Color.Transparent, size = 32.dp, iconSize = 15.dp,
                                    )
                                }
                                d.status == TaskCenter.Status.DONE -> {
                                    // 文件本体真丢了(被系统/清理软件删掉)或位置未知:如实告知,
                                    // 绝不装作能打开;只给删除(移除死条目)。
                                    Text(
                                        if (d.localUri.isEmpty()) "位置未知" else "文件已被删除",
                                        Modifier.padding(end = 6.dp),
                                        fontSize = 10.5.sp, color = Pa.Red,
                                    )
                                }
                            }
                            IconActionButton(
                                R.drawable.ic_lucide_trash_2, "删除",
                                {
                                    // 条目无论如何都移除(留着只会变成打不开的死条目),
                                    // 但提示按实际删除结果说 —— 删不掉不再谎报「已删除」。
                                    val deleted = deleteLocalFile(ctx, d.localUri)
                                    TaskCenter.remove(d.id)
                                    Toast.makeText(
                                        ctx,
                                        if (deleted) "已删除" else "文件可能已被系统清理,条目已移除",
                                        Toast.LENGTH_SHORT,
                                    ).show()
                                },
                                bg = Color.Transparent, size = 32.dp, iconSize = 15.dp,
                                tint = Pa.TextTertiary,
                            )
                        }
                        Box(Modifier.fillMaxWidth().height(1.dp).background(Pa.Separator))
                    }
                }
            }
        }
        CacheSection(
            taskCount = allTasks.size,
            downloadCount = doneCount,
            downloadBytes = downloadBytes,
            downloadUnknown = downloadUnknown,
        )
        Spacer(Modifier.height(20.dp))
    }
}

/**
 * 存储占用 / 缓存管理:让用户看得见"app 到底占了多少本地空间",并给出清理入口。
 *
 * 【口径必须与系统严格一致】系统「设置 → 应用 → 存储占用」是这样分的:
 *   缓存     = `cacheDir` 的**真实递归大小**
 *   用户数据 = `filesDir`(以及 SP / 数据库 / WebView 数据目录)
 * 本页以前只把自建的三项(预览 / 快照 / 临时)相加,于是出现
 * "系统说 25.72MB、App 说 62KB" —— 差的正是 **WebView 写在 cacheDir 里的
 * HTTP 缓存与 DOM Storage**,以及被错算进"缓存"、其实属于用户数据的会话快照。
 * 现在直接量真实目录,两边必然对得上。
 *
 * 【清理策略】(对应审查 2.3 矩阵)
 * - 临时文件 / 悬空条目 / 浏览器数据 / 预览缓存:可再生,随时可清;
 * - 会话快照:清了会失去离线可看的内容(联网后重建),所以单独说明;
 * - 已下载的文件本体:**不在这里动** —— 那属于用户内容,只在上面的列表里逐条删;
 * - 登录凭证与设置:任何清理项都不碰,避免"清缓存把自己清掉线"。
 */
@Composable
private fun CacheSection(
    taskCount: Int,
    downloadCount: Int,
    downloadBytes: Long,
    downloadUnknown: Boolean,
) {
    val ctx = androidx.compose.ui.platform.LocalContext.current
    val scope = rememberCoroutineScope()
    // 与系统一致的两项
    var cacheBytes by remember { mutableLongStateOf(0L) }
    var userBytes by remember { mutableLongStateOf(0L) }
    // 明细(用于「详情」里解释"这些数字里都是什么")
    var previewBytes by remember { mutableLongStateOf(0L) }
    var snapshotBytes by remember { mutableLongStateOf(0L) }
    var tempBytes by remember { mutableLongStateOf(0L) }
    var browserBytes by remember { mutableLongStateOf(0L) }
    var toastMsg by remember { mutableStateOf<String?>(null) }
    var confirmClearTasks by remember { mutableStateOf(false) }

    /** 统计走 IO 线程:遍历目录不能在 UI 线程做。 */
    fun reload() = scope.launch(kotlinx.coroutines.Dispatchers.IO) {
        val pv = dev.pocketagent.app.ui.component.PreviewCache.usage()
        val sn = dev.pocketagent.app.ui.component.ThreadSnapshot.usage()
        val tp = dev.pocketagent.app.ui.component.PaCache.tempUsage(ctx)
        val br = webDataUsage(ctx)
        // 真实目录大小 —— 这两个数字就是系统存储页看到的口径
        val cd = dev.pocketagent.app.ui.component.PaCache.cacheUsage(ctx)
        val fd = dev.pocketagent.app.ui.component.PaCache.filesUsage(ctx)
        withContext(kotlinx.coroutines.Dispatchers.Main) {
            previewBytes = pv
            snapshotBytes = sn
            tempBytes = tp
            browserBytes = br
            cacheBytes = cd
            userBytes = fd
        }
    }
    LaunchedEffect(Unit) { reload() }

    toastMsg?.let { msg ->
        LaunchedEffect(msg) {
            Toast.makeText(ctx, msg, Toast.LENGTH_SHORT).show()
            toastMsg = null
        }
    }

    SectionBlock(icon = R.drawable.ic_lucide_trash_2, title = "存储占用") {
        // ---- 三格总览:口径与系统存储页一致 ----
        Row(Modifier.fillMaxWidth().padding(vertical = 10.dp)) {
            StatCell2("缓存", sizeText(cacheBytes), Modifier.weight(1f))
            StatCell2("用户数据", sizeText(userBytes), Modifier.weight(1f))
            // 下载占用:查不到真实体积就如实标「未知」,绝不用 0 冒充
            StatCell2("下载", if (downloadUnknown) "未知" else sizeText(downloadBytes), Modifier.weight(1f))
        }

        // ---- 清理动作:一行标题 + 一行短后果(≤16 字),完整说明收进下方「详情」 ----
        CacheActionRow(
            title = "清理临时与失效缓存",
            desc = "清残留,可随时",
        ) {
            scope.launch(kotlinx.coroutines.Dispatchers.IO) {
                val temps = dev.pocketagent.app.ui.component.PaCache.sweepTemp(ctx)
                val dangling = dev.pocketagent.app.ui.component.PaDownloader.pruneDanglingTasks(ctx)
                withContext(kotlinx.coroutines.Dispatchers.Main) {
                    reload()
                    toastMsg = when {
                        temps == 0 && dangling == 0 -> "没有需要清理的内容"
                        else -> "已清理:${listOfNotNull(
                            if (temps > 0) "$temps 个临时文件" else null,
                            if (dangling > 0) "$dangling 个失效条目" else null,
                        ).joinToString("、")}"
                    }
                }
            }
        }

        // ---- 清理预览与快照缓存(离线能力会一起清空,所以说明清楚) ----
        if (previewBytes + snapshotBytes > 0) {
            CacheActionRow(
                title = "清空预览与会话快照",
                desc = "断网预览将失效",
            ) {
                scope.launch(kotlinx.coroutines.Dispatchers.IO) {
                    val a = dev.pocketagent.app.ui.component.PreviewCache.clear()
                    val b = dev.pocketagent.app.ui.component.ThreadSnapshot.clear()
                    withContext(kotlinx.coroutines.Dispatchers.Main) {
                        reload()
                        toastMsg = "已释放 ${sizeText(a + b)}"
                    }
                }
            }
        }

        // ---- 清理浏览器数据 ----
        CacheActionRow(
            title = "清理浏览器数据",
            desc = "网站需重新登录",
        ) {
            dev.pocketagent.app.ui.component.PaCache.clearBrowserData(ctx)
            reload()
            toastMsg = "浏览器数据已清理"
        }

        // ---- 清空任务条目(保留文件本体,需二次确认) ----
        CacheActionRow(
            title = "清空已完成的任务条目",
            desc = "仅删记录,留文件",
            danger = true,
        ) {
            confirmClearTasks = true
        }

        // ---- 详情:口径与清理后果的完整说明(默认折叠,信息不丢、不刷屏) ----
        CollapsibleDetail {
            Text(
                "缓存 ${sizeText(cacheBytes)} = 浏览器缓存(粗算) ${sizeText(browserBytes)}" +
                    " + 预览缓存 ${sizeText(previewBytes)} + 临时文件 ${sizeText(tempBytes)}" +
                    " + 其余 cacheDir 内容;与系统「存储占用 → 缓存」同口径,可对照。",
                Modifier.padding(bottom = 4.dp),
                fontSize = 10.5.sp, color = Pa.TextTertiary,
            )
            Text(
                "用户数据 ${sizeText(userBytes)} = 会话快照 ${sizeText(snapshotBytes)}" +
                    " + 设置与登录凭证;下方清理项都不会动到它。",
                Modifier.padding(bottom = 4.dp),
                fontSize = 10.5.sp, color = Pa.TextTertiary,
            )
            Text(
                "下载 ${if (downloadUnknown) "占用未知" else sizeText(downloadBytes)} · $downloadCount 个文件" +
                    " · 任务条目 $taskCount 条;文件本体在系统下载目录,只能在上方列表逐条删除。",
                Modifier.padding(bottom = 4.dp),
                fontSize = 10.5.sp, color = Pa.TextTertiary,
            )
            Text(
                "清理临时与失效缓存:删崩溃残留的临时文件与「文件已被删除」的下载条目;" +
                    "不碰预览缓存、不碰登录状态,可随时清。",
                Modifier.padding(bottom = 4.dp),
                fontSize = 10.5.sp, color = Pa.TextTertiary,
            )
            Text(
                "清空预览与会话快照:释放 ${sizeText(previewBytes + snapshotBytes)};" +
                    "后果:断网时「刚看过的文件」和最近对话打不开,联网后自动重建;不影响登录与电脑端。",
                Modifier.padding(bottom = 4.dp),
                fontSize = 10.5.sp, color = Pa.TextTertiary,
            )
            Text(
                "清理浏览器数据:清掉内置浏览器的 HTTP 缓存 / Cookie / 网页存储;" +
                    "后果:浏览器里登录过的网站需重新登录;不影响电脑端与 App 设置。",
                Modifier.padding(bottom = 4.dp),
                fontSize = 10.5.sp, color = Pa.TextTertiary,
            )
            Text(
                "清空已完成的任务条目:只清任务中心记录列表,不删已下载文件,也不减上方「下载」占用。",
                fontSize = 10.5.sp, color = Pa.TextTertiary,
            )
        }
    }

    if (confirmClearTasks) {
        androidx.compose.material3.AlertDialog(
            onDismissRequest = { confirmClearTasks = false },
            title = { Text("清空任务条目?", color = Pa.TextPrimary) },
            text = {
                Text(
                    "只移除任务中心的记录,已下载到手机的文件不受影响。此操作不可撤销。",
                    color = Pa.TextSecondary, fontSize = 13.sp,
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    TaskCenter.clearFinished()
                    confirmClearTasks = false
                    reload()
                    toastMsg = "已清空已完成的条目"
                }) { Text("清空", color = Pa.Red) }
            },
            dismissButton = {
                TextButton(onClick = { confirmClearTasks = false }) {
                    Text("取消", color = Pa.TextSecondary)
                }
            },
            containerColor = Pa.CardHigh,
        )
    }
}

/** 缓存分区的一格数字。 */
@Composable
private fun StatCell2(label: String, value: String, modifier: Modifier = Modifier) {
    Column(modifier, horizontalAlignment = androidx.compose.ui.Alignment.CenterHorizontally) {
        Text(value, fontSize = 13.sp, color = Pa.TextPrimary)
        Text(label, Modifier.padding(top = 2.dp), fontSize = 10.sp, color = Pa.TextTertiary)
    }
}

/** 缓存管理里的一个可执行动作行。 */
@Composable
private fun CacheActionRow(
    title: String,
    desc: String,
    danger: Boolean = false,
    onClick: () -> Unit,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .pressable(haptic = true, onClick = onClick)
            .padding(vertical = 10.dp),
        verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(title, fontSize = 13.sp, color = if (danger) Pa.Red else Pa.TextPrimary)
            Text(desc, Modifier.padding(top = 2.dp), fontSize = 10.5.sp, color = Pa.TextTertiary)
        }
        Icon(
            painterResource(R.drawable.ic_lucide_chevron_right), null,
            tint = Pa.TextTertiary, modifier = Modifier.size(15.dp),
        )
    }
}

/**
 * 默认折叠的「详情」展开区:把长解释收进来,默认不刷屏,点一下才展开。
 * 用简单 state 切换,不引第三方依赖。
 */
@Composable
private fun CollapsibleDetail(content: @Composable () -> Unit) {
    var expanded by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxWidth().padding(top = 4.dp)) {
        Row(
            Modifier
                .fillMaxWidth()
                .pressable(haptic = false, onClick = { expanded = !expanded })
                .padding(vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text("详情", fontSize = 11.sp, color = Pa.TextTertiary)
            Spacer(Modifier.weight(1f))
            Text(if (expanded) "▾" else "▸", fontSize = 12.sp, color = Pa.TextTertiary)
        }
        if (expanded) {
            content()
        }
    }
}

/** 人类可读的体积。 */
private fun sizeText(bytes: Long): String = when {
    bytes >= 1024 * 1024 -> String.format(java.util.Locale.US, "%.1fMB", bytes / 1048576f)
    bytes >= 1024 -> String.format(java.util.Locale.US, "%.0fKB", bytes / 1024f)
    else -> "${bytes}B"
}

/** WebView 数据占用(app_webview 目录),粗算即可 —— 只用于给用户看个量级。 */
private fun webDataUsage(ctx: android.content.Context): Long {
    val dir = java.io.File(ctx.applicationInfo.dataDir, "app_webview")
    return runCatching {
        fun size(f: java.io.File): Long =
            if (f.isDirectory) (f.listFiles()?.sumOf { size(it) } ?: 0L) else f.length()
        size(dir)
    }.getOrDefault(0L)
}
