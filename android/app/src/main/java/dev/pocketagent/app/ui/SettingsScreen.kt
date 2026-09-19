package dev.pocketagent.app.ui

import android.widget.Toast
import dev.pocketagent.app.BuildConfig
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
import androidx.compose.foundation.lazy.items
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
import dev.pocketagent.app.ui.component.SenseVoiceModelManager
import dev.pocketagent.app.ui.component.fileKindOf
import dev.pocketagent.app.ui.component.humanSize
import dev.pocketagent.app.ui.component.parseHumanSize
import dev.pocketagent.app.ui.component.PaDownloader
import dev.pocketagent.app.ui.component.SplitButton
import dev.pocketagent.app.ui.component.SplitOption
import dev.pocketagent.app.ui.theme.AgentTheme
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.async
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.supervisorScope
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

// 非 codex 的可切换 agent 清单("切换 Agent"列表 = codex + 这一份)。
// 【图 7 实锤的修复】此前这份硬编码列表漏了 zcode —— 主页网格、品牌图标表
// (AGENT_BRAND_ICON)、daemon 的 provider 注册表里 zcode 都在,唯独设置页
// "切换 Agent"从这份清单构建,于是用户看到的列表只有
// Codex/OpenCode/Claude/Pi/Hermes,ZCode 永远切不到。补上后与主页同源同序。
private val OTHER_AGENT_OPTIONS = listOf(
    "zcode" to "ZCode",
    "opencode" to "OpenCode",
    "claude-code" to "Claude",
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
 * agent id → 品牌图标。
 *
 * 【为什么只是转发】映射表已收敛到 AgentBrand.kt 的 [AGENT_BRAND_ICON] 唯一一份。
 * 这里以前另有一份 private map,与主页的 id 写法不一致,正是
 * "设置里的 Agent 图标和主页对不上"的根因。
 */
private fun agentIcon(id: String): Int? = agentBrandIcon(id)

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
    var settings by remember(AgentConnection.selectedAgent) { mutableStateOf<CodexSettings?>(null) }
    val writeMutex = remember { Mutex() }
    var writeVersion by remember { mutableStateOf(0) }
    var mcpList by remember { mutableStateOf(listOf<McpServerStatus>()) }
    var mcpLoading by remember { mutableStateOf(true) }
    var mcpError by remember { mutableStateOf<String?>(null) }
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
    // 【应用范围】七项调整默认只对**新建**会话生效(daemon 侧快照语义:已有会话
    // 保持创建那一刻的设置)。用户质疑"改了是否真生效"的根源就是这条默认规则
    // 与"改了立刻全生效"的预期冲突 —— 这里提供一个显式开关:开启后每次保存
    // 都会附带"应用到所有会话(含已有)",daemon 把新值批量写进存量 codex 会话的
    // 设置快照。默认关 = 保持快照语义,不静默改老会话。
    var applyToExistingThreads by remember { mutableStateOf(false) }
    // 默认模型选择器(完整选择弹窗,主按钮触发)
    var showModelPicker by remember { mutableStateOf(false) }
    var modelChoices by remember { mutableStateOf(listOf<ModelInfo>()) }
    var modelError by remember { mutableStateOf<String?>(null) }
    var hostError by remember { mutableStateOf<String?>(null) }
    val runningAgent = AgentConnection.selectedAgent
    val connectionError = AgentConnection.connError.collectAsState().value
    val connectionEpoch = AgentConnection.connectionEpoch.collectAsState().value

    /**
     * 逐项真实状态:key → state。
     *
     * 【为什么必须有它】"设置页所有开关点了都没用"的根子,是手机端**无法区分**
     * 已生效 / 未生效 / 不支持 / 连接失败 —— 以前一律显示"已保存"。现在 daemon 按
     * 真实能力逐项回报(setting_status),这里原样取用,由 [settingStateInfo] 映射成文案。
     */
    val settingStates: Map<String, String> = remember(settings) {
        settings?.settingStatusList.orEmpty().associate { it.key to it.state }
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

    /** 只重新读取设置,不提交当前值。顶部“重试”必须走这条读路径。 */
    fun refreshCodexSettings(showFeedback: Boolean = false) {
        scope.launch {
            try {
                val refreshed = withContext(Dispatchers.IO) {
                    AgentConnection.modelService()
                        .getCodexSettings(GetCodexSettingsRequest.newBuilder().setAgentId(runningAgent).build())
                }
                settings = refreshed
                val refreshedModels = withContext(Dispatchers.IO) {
                    AgentConnection.modelServiceSlow().listModels(ListModelsRequest.newBuilder().setAgentId(runningAgent).setRefresh(true).build()).modelsList
                }
                modelChoices = modelChoices.filter { it.provider != runningAgent } + refreshedModels
                settingsError = null
                AgentConnection.showThinking = !refreshed.hideAgentReasoning
                if (showFeedback) showSettingsFeedback("已重新读取")
            } catch (e: Exception) {
                settingsError = "无法加载 Codex 设置：${e.message ?: "连接失败"}"
                if (showFeedback) showSettingsFeedback("读取失败")
            }
        }
    }

    LaunchedEffect(connectionEpoch, runningAgent) {
        mcpLoading = true
        capabilitiesLoading = true
        usageLoading = true
        mcpError = null
        capabilitiesError = null
        try {
            withContext(Dispatchers.IO) {
                settings = AgentConnection.modelService()
                    .getCodexSettings(GetCodexSettingsRequest.newBuilder().setAgentId(runningAgent).build())
            }
            // 【两个"隐藏思考"开关的权威对齐】daemon 的 hide_agent_reasoning 决定
            // 是否推送思考事件;客户端的 showThinking 决定是否渲染思考卡。
            // 进程重启后本地开关可能与 daemon 漂移(用户反馈:关了隐藏仍看不到思考),
            // 每次加载设置都以 daemon 为权威校准一次。
            settings?.let {
                AgentConnection.showThinking = !it.hideAgentReasoning
                AgentConnection.saveAgentDefaults(runningAgent, it.model, it.effort)
            }
        } catch (e: Exception) {
            settingsError = "无法加载 Codex 设置：${e.message ?: "连接失败"}"
        }
        // 账户、模型、MCP、能力、用量各自执行;一个通道失败不会取消兄弟通道。
        supervisorScope {
            async {
                try {
                    host = withContext(Dispatchers.IO) {
                        AgentConnection.hostService().getHostInfo(GetHostInfoRequest.getDefaultInstance())
                    }
                    hostError = null
                } catch (e: Exception) {
                    hostError = e.message ?: "连接失败"
                }
            }
            async {
                try {
                    modelChoices = withContext(Dispatchers.IO) {
                        AgentConnection.modelServiceSlow()
                            .listModels(ListModelsRequest.newBuilder().build())
                            .modelsList
                    }
                    modelError = null
                } catch (e: Exception) {
                    modelError = e.message ?: "连接失败"
                }
            }
            async {
                try {
                    val response = withContext(Dispatchers.IO) {
                        AgentConnection.modelServiceSlow()
                            .listMcpServers(ListMcpServersRequest.newBuilder().build())
                    }
                    if (response.error.isNotBlank()) {
                        mcpList = emptyList()
                        mcpError = response.error
                    } else {
                        mcpList = response.serversList
                        mcpError = null
                    }
                } catch (e: Exception) {
                    mcpError = e.message ?: "连接失败"
                } finally {
                    mcpLoading = false
                }
            }
            async {
                try {
                    val response = withContext(Dispatchers.IO) {
                        AgentConnection.modelServiceSlow()
                            .listCodexCapabilities(ListCodexCapabilitiesRequest.newBuilder().build())
                    }
                    capabilities = response.itemsList
                    capabilitiesError = response.error.takeIf { it.isNotBlank() }
                } catch (e: Exception) {
                    capabilitiesError = e.message ?: "连接失败"
                } finally {
                    capabilitiesLoading = false
                }
            }
            async {
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
        }
        /* 三个面板原本在这里串行读取;现在由 supervisorScope 按通道并发执行。
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
        读取失败只更新对应通道的 error,不写 settingsError。*/
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
        val version = ++writeVersion
        val targetAgent = if (hideReasoning != null) "codex" else runningAgent
        val displayAgent = runningAgent
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
        showSettingsFeedback("保存中…")
        scope.launch {
            try {
                val returned = writeMutex.withLock { withContext(Dispatchers.IO) {
                    AgentConnection.modelService().setCodexSettings(
                        SetCodexSettingsRequest.newBuilder().apply {
                            setAgentId(targetAgent)
                            approval?.let { setApprovalPolicy(it) }
                            sandbox?.let { setSandbox(it) }
                            model?.let { setModel(it) }
                            effort?.let { setEffort(it) }
                            hideReasoning?.let { setHideAgentReasoning(it) }
                            verbosity?.let { setModelVerbosity(it) }
                            reasoningSummary?.let { setModelReasoningSummary(it) }
                            pasteBurst?.let { setDisablePasteBurst(it) }
                            setApplyToExistingThreads(applyToExistingThreads)
                        }
                            .build())
                } }
                if (version != writeVersion) return@launch
                if (AgentConnection.selectedAgent == displayAgent) {
                    settings = if (targetAgent == displayAgent) returned
                        else previous.toBuilder().setHideAgentReasoning(returned.hideAgentReasoning).build()
                }
                if (hideReasoning == null) AgentConnection.saveAgentDefaults(targetAgent, returned.model, returned.effort)
                // 【诚实反馈】daemon 逐项回报真实状态:有 failed / saved_not_applied 时
                // 就别再说"已保存"(那会被理解成"已生效")。真问题由顶部横幅+逐项小字呈现。
                // 勾了"应用到所有会话"时,daemon 还会回执实际写到的存量会话数,
                // 按 counts 如实说"应用到 N 个",不把两件事混成一句含糊的"已保存"。
                val states = returned.settingStatusList.map { it.state }
                val feedback = when {
                    states.any { it == "failed" } -> "已保存,但应用失败"
                    states.any { it == "saved_not_applied" } -> "已保存,尚未生效"
                    applyToExistingThreads && returned.appliedThreads > 0 ->
                        "已保存,并应用到 ${returned.appliedThreads} 个已有会话"
                    applyToExistingThreads -> "已保存(没有需要更新的已有会话)"
                    else -> "已保存"
                }
                showSettingsFeedback(feedback)
                // 【两个"隐藏思考"开关的联动】daemon 侧 hide_agent_reasoning=true 会让它
                // **不再推送**思考事件;而客户端还有一个纯显示开关 AgentConnection.showThinking
                // (ChatScreen 用它决定要不要渲染思考卡)。两者叠加会出现"点了隐藏推理、
                // 可客户端还开着"的矛盾状态 —— 这里以 daemon 侧为权威,把它同步一致。
                hideReasoning?.let { AgentConnection.showThinking = !it }
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                if (version != writeVersion) return@launch
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

    // 二级页"只记一层"的返回目标:记下该二级页是被哪一层打开的(本题只需一层,不引入完整
    // 导航栈)。从首页进入的二级页保持 null,行为不变;项 3a 的"查看全部技能与插件"由
    // Agent 与模型页打开时置为 "agent",返回才能回到 Agent 与模型页而非设置首页。
    var subPageReturn by remember { mutableStateOf<String?>(null) }

    // 从二级页返回:优先回到"打开它的那一层",否则回一级页。物理返回与二级页顶部返回箭头
    // 共用同一逻辑,避免两条返回路径行为不一致。
    val backFromSubPage: () -> Unit = {
        if (subPageReturn != null) { subPage = subPageReturn; subPageReturn = null }
        else subPage = null
    }

    // 返回层级:二级页 →(打开它的那一层)→ 一级页 → 退出设置(此前物理返回直接退出,层级断裂)
    androidx.activity.compose.BackHandler(enabled = true) {
        if (subPage != null) backFromSubPage() else onBack()
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
    if (subPage == "daemon") {
        DaemonPage(onBack = { subPage = null }, onFeedback = { showSettingsFeedback(it) }, writer = codexWriter, models = modelChoices, capabilities = host?.capabilitiesList.orEmpty())
        return
    }
    if (subPage == "appearance") {
        AppearancePage(onBack = { subPage = null }, onFeedback = { showSettingsFeedback(it) })
        return
    }
    if (subPage == "capabilities") {
        // 项 3a:技能/插件统一到**一个**二级页(不再每个 kind 一个"展开更多"弹窗)。
        CapabilitiesPage(
            onBack = backFromSubPage,
            capabilities = capabilities,
            loading = capabilitiesLoading,
            error = capabilitiesError,
        )
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
            mcpError = mcpError,
            codexUsage = codexUsage,
            usageLoading = usageLoading,
            capabilities = capabilities,
            capabilitiesLoading = capabilitiesLoading,
            capabilitiesError = capabilitiesError,
            host = host,
            writeCodex = codexWriter,
            settingStates = settingStates,
            applyToExistingThreads = applyToExistingThreads,
            onToggleApplyToAll = { applyToExistingThreads = it },
            onRetrySettings = { refreshCodexSettings(showFeedback = true) },
            onFeedback = { showSettingsFeedback(it) },
            onShowModelPicker = { showModelPicker = true },
            onShowApprovalPreview = { showApprovalPreview = true },
            // 打开二级页时记下返回目标("agent"),返回才会回到本页而非设置首页。
            onOpenCapabilities = { subPageReturn = "agent"; subPage = "capabilities" },
        )
        // 【必须在 return 之前渲染】AgentPage 是二级页,上面 return 之后下面的
        // 一级页渲染点永远不会执行 —— 预览按钮点了没反应/要退出二级页才冒出来的根因。
        if (showApprovalPreview) {
            ApprovalPreviewSheet(onDismiss = { showApprovalPreview = false })
        }
        if (showModelPicker) {
            ModelPickerDialog(
                // 只列 codex 自己的模型:"默认模型"是 codex 的桌面默认值,
                // pi/zcode 的模型混进来会让人误选别的 agent 的模型(见模型与思考区注释)。
                models = modelChoices.filter { it.provider == runningAgent },
                selected = settings?.model.orEmpty(),
                agentId = runningAgent,
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
                                message.contains("保存中") || message.contains("未生效") -> Pa.Accent
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
                        SettingRow(title = "Daemon 桥接与输入", desc = "连接策略 · 全局对话显示 · 语音 · 实验功能",
                            icon = R.drawable.ic_lucide_server, chevron = true, divider = true, onClick = { subPage = "daemon" })
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
                            value = "v${BuildConfig.VERSION_NAME}",
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
            // 只列 codex 自己的模型(与上方"默认模型"候选同一套收窄,理由同)。
            models = modelChoices.filter { it.provider == runningAgent },
            selected = settings?.model.orEmpty(),
            agentId = runningAgent,
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
            fontSize = 14.sp,
            color = Pa.TextSecondary,
            fontWeight = FontWeight.SemiBold,
        )
    }
}

/**
 * 二级页骨架:返回 + 标题 + 内容。
 *
 * [scrollable] = true(默认):外层 Column 提供 verticalScroll,内容用普通 Column 堆。
 * [scrollable] = false:**内容自己负责滚动**(内容内含 LazyColumn)。
 * 【为什么需要这个开关】LazyColumn 套在 verticalScroll 的 Column 里会因为
 * "无限高度"直接崩 —— 技能与插件页要按分组懒加载(可能上百条),必须走 LazyColumn。
 */
@Composable
private fun SubPageScaffold(
    title: String,
    onBack: () -> Unit,
    scrollable: Boolean = true,
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
        if (scrollable) {
            Column(
                Modifier
                    .weight(1f)
                    .fillMaxWidth()
                    .verticalScroll(rememberScrollState())
                    .navigationBarsPadding()
                    .padding(horizontal = 16.dp, vertical = 6.dp),
                content = content,
            )
        } else {
            Column(
                Modifier
                    .weight(1f)
                    .fillMaxWidth()
                    .navigationBarsPadding()
                    .padding(horizontal = 16.dp, vertical = 6.dp),
                content = content,
            )
        }
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
 * 一条能力(skill / hook / plugin)的展示行。
 *
 * 【description 优先级】disabledReason(为什么用不了)→ description → 来源 xxx。
 * enabled=false 却不说明原因时,用户只会觉得"我装的技能没生效",所以在右侧
 * 额外标一个"已停用"。三项都空就整行留白,不硬凑一句废话。
 */
@Composable
private fun CapabilityRowItem(c: CodexCapability, divider: Boolean) {
    val desc = listOfNotNull(
        c.disabledReason.takeIf { it.isNotBlank() },
        c.description.takeIf { it.isNotBlank() },
        c.source.takeIf { it.isNotBlank() }?.let { "来源 $it" },
    ).firstOrNull().orEmpty()
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

/**
 * 一行设置项的"真实状态"小字(项 3b)。
 *
 * 【只在非"已生效"时出现】已生效没必要刷屏;而"未生效 / 当前 codex 版本不支持 /
 * 连接失败"必须让用户看见 —— 这三种状态的共同点是"用户以为改了,其实没改"。
 * 重试统一放在本页顶部，避免同一次 Codex 探测失败在每一项下面重复显示。
 */
@Composable
private fun SettingStateLine(key: String, states: Map<String, String>, _onRetry: () -> Unit) {
    val info = states[key]?.let { settingStateInfo(it) }
    if (info != null && info.level != "ok") {
        val color = when (info.level) {
            "error" -> Pa.RedText
            "warn" -> Pa.Accent
            else -> Pa.TextTertiary
        }
        Row(
            Modifier.fillMaxWidth().padding(bottom = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(info.text, fontSize = 12.sp, color = color)
        }
    }
}

/**
 * 一行设置项的"电脑端当前值"只读回显小字(桌面只读回显,只增)。
 *
 * 【它与 [SettingStateLine] 的分工】SettingStateLine 说"手机改的这项生效了没有";
 * 本行说"电脑端此刻实际生效的是什么"(daemon 的 desktop_config_summary,来自
 * 只读的 config/read)—— 用户把某项留在"跟随桌面"时,真实生效值就在这里。
 * 只在读到了真实值时显示,无值不占位;纯展示,不改七项的可调性。
 */
@Composable
private fun DesktopEchoLine(settings: CodexSettings?, key: String) {
    val line = DesktopEchoLogic.desktopEchoLine(settings, key) ?: return
    Row(Modifier.fillMaxWidth().padding(bottom = 6.dp)) {
        Text(line, fontSize = 11.sp, color = Pa.TextTertiary)
    }
}

/**
 * 二级页:技能与插件(项 3a 的合并目标)。
 *
 * 【为什么要合并成一个页面】以前「能力 · 技能与插件」区块里,每个 kind 各有一行
 * "展开更多(N)",而它们的 onClick **全都**打开同一个弹窗 —— 于是用户看到
 * "两个入口点开的页面内容完全相同"。现在区块里只剩唯一入口,点击进入这里。
 *
 * 【四态必须分清】读取中 / 读失败(给出原文)/ 确实为空 / 有数据。
 * 绝不允许把"还在读"渲染成"未报告任何技能、hooks 或插件"—— 那会让用户去重装技能。
 *
 * 【分组顺序固定】技能 → Hooks → 插件,未知 kind 附在后面且**不丢弃**
 * (见 [capabilityKindOrder]);组内按名称稳定排序,避免每次打开位置抖动。
 */
@Composable
private fun CapabilitiesPage(
    onBack: () -> Unit,
    capabilities: List<CodexCapability>,
    loading: Boolean,
    error: String?,
) {
    var query by remember { mutableStateOf("") }
    SubPageScaffold(title = "技能与插件", onBack = onBack, scrollable = false) {
        when {
            loading -> {
                Box(
                    Modifier.fillMaxWidth().padding(vertical = 40.dp),
                    contentAlignment = Alignment.Center,
                ) { Text("正在读取…", fontSize = 13.sp, color = Pa.TextTertiary) }
            }

            error != null -> Column(Modifier.fillMaxWidth().padding(vertical = 24.dp)) {
                Text("暂时读不到能力列表", fontSize = 14.sp, color = Pa.RedText)
                Spacer(Modifier.height(6.dp))
                Text(error, fontSize = 11.sp, color = Pa.TextTertiary)
            }

            capabilities.isEmpty() -> Box(
                Modifier.fillMaxWidth().padding(vertical = 40.dp),
                contentAlignment = Alignment.Center,
            ) { Text("这台电脑上的 Codex 未报告任何技能、hooks 或插件", fontSize = 13.sp, color = Pa.TextTertiary) }

            else -> {
                val filtered = filterCapabilities(capabilities, query)
                val groups = groupCapabilities(filtered)
                SettingsTextField(
                    value = query,
                    onValueChange = { query = it },
                    label = "搜索名称或来源",
                )
                Spacer(Modifier.height(10.dp))
                if (filtered.isEmpty()) {
                    // 有数据但被搜索过滤光了:如实说是"没匹配",不要复用"未报告任何"。
                    Box(
                        Modifier.fillMaxWidth().padding(vertical = 30.dp),
                        contentAlignment = Alignment.Center,
                    ) { Text("没有匹配「$query」的技能、hooks 或插件", fontSize = 12.sp, color = Pa.TextTertiary) }
                } else {
                    LazyColumn(
                        Modifier.fillMaxWidth().weight(1f),
                        contentPadding = PaddingValues(bottom = 20.dp),
                    ) {
                        groups.forEach { (kind, list) ->
                            item(key = "header-$kind") {
                                Text(
                                    capabilityKindLabel(kind) + " · " + list.size,
                                    fontSize = 11.sp,
                                    color = Pa.TextTertiary,
                                    modifier = Modifier.padding(top = 12.dp, bottom = 4.dp),
                                )
                            }
                            items(list, key = { c -> kind + ":" + c.path + ":" + c.name }) { c ->
                                CapabilityRowItem(c, divider = true)
                            }
                        }
                    }
                }
            }
        }
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
    ).also { it.onCommit = onCommit }

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
    mcpError: String?,
    codexUsage: CodexUsage?,
    usageLoading: Boolean,
    capabilities: List<CodexCapability>,
    capabilitiesLoading: Boolean,
    capabilitiesError: String?,
    host: HostInfo?,
    writeCodex: CodexWriter,
    /** 逐项真实状态(key → state),来自 daemon 的 setting_status。 */
    settingStates: Map<String, String>,
    /** 应用范围开关:"应用到所有会话(含已有)"。默认关 = 仅新建会话生效(快照语义)。 */
    applyToExistingThreads: Boolean,
    onToggleApplyToAll: (Boolean) -> Unit,
    /** 重试:把当前快照重新提交一次(失败项可恢复)。 */
    onRetrySettings: () -> Unit,
    onFeedback: (String) -> Unit,
    onShowModelPicker: () -> Unit,
    onShowApprovalPreview: () -> Unit,
    /** 打开"查看全部技能与插件"二级页;由父级负责记下返回目标,本页不直接操作路由状态。 */
    onOpenCapabilities: () -> Unit,
) {
    SubPageScaffold(title = "Agent 与模型", onBack = onBack) {
        TextButton(onClick=onRetrySettings) { Text("刷新 Agent 状态和模型",color=Pa.Accent) }
        SectionBlock(icon = R.drawable.ic_lucide_cpu, title = "切换 Agent") {
            Spacer(Modifier.height(10.dp))
            val agentChoices = buildAgents(host?.capabilitiesList.orEmpty()).map { it.id to it.name }
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
                        val installed=buildAgents(host?.capabilitiesList.orEmpty()).any { it.id==id && it.available }
                        if (!installed) onFeedback("该 Agent 尚未安装或尚未接入")
                        else if (runningAgent != id) {
                            AgentConnection.switchAgent(id)
                            onFeedback("已切换到 ${agentDisplay(id)}")
                        }
                    },
                )
            }
        }

        if (runningAgent == "codex") {
            // 顶部诚实状态条:只要存在"未生效 / 不支持 / 连接失败"就出现,并可一键重试。
            // 【绝不静默】以前这里的失败全被吞掉,用户只看到"已保存"。
            val note = overallSettingNote(settingStates.values.toList())
            if (note != null) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .background(Pa.Card, RoundedCornerShape(14.dp))
                        .padding(start = 13.dp, end = 6.dp, top = 8.dp, bottom = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(R.drawable.ic_lucide_triangle_alert), null,
                        tint = Pa.Accent, modifier = Modifier.size(15.dp),
                    )
                    Spacer(Modifier.width(9.dp))
                    Text(note, Modifier.weight(1f), fontSize = 11.5.sp, color = Pa.TextSecondary)
                    TextButton(onClick = onRetrySettings) {
                        Text("重试", color = Pa.Accent, fontSize = 12.sp)
                    }
                }
                Spacer(Modifier.height(12.dp))
            }

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

                // 【说明小字 + 显式应用范围】设置页更改默认只对新建对话生效:已有对话
                // 保持创建那一刻的设置(daemon 侧按线程快照装配参数),不被之后的更改
                // 牵动。用户质疑"改了是否真生效"正源于此 —— 下面给一个显式开关,
                // 开启后每次保存都把新值批量写进已有会话的快照(daemon 回执条数,
                // 反馈文案如实说"应用到 N 个"),默认保持"仅新建"不变。
                Text(
                    "更改默认对新建对话生效,已有对话保持原设置",
                    fontSize = 11.sp,
                    color = Pa.TextSecondary,
                    modifier = Modifier.padding(bottom = 4.dp),
                )
                SettingRow(
                    title = "应用到所有会话(含已有)",
                    desc = "开启后保存的每一项同时写入已有会话;关闭 = 仅新建会话生效",
                    divider = true,
                    control = {
                        Switch(
                            checked = applyToExistingThreads,
                            onCheckedChange = onToggleApplyToAll,
                            colors = SwitchDefaults.colors(
                                checkedTrackColor = Pa.Accent,
                                uncheckedTrackColor = Pa.Hover,
                                uncheckedBorderColor = Color.Transparent,
                            ),
                        )
                    },
                )

                // 【桌面只读回显】Windows 沙箱就绪度一行(windowsSandbox/readiness,
                // 只读诊断):读不到/老 daemon 不占位。电脑端配置读取失败时也在这里
                // 如实说一句 —— "读不到"不能被误读成"没有值"。
                DesktopEchoLogic.sandboxReadinessLine(settings)?.let { line ->
                    Text(
                        line,
                        fontSize = 11.sp,
                        color = Pa.TextTertiary,
                        modifier = Modifier.padding(bottom = 6.dp),
                    )
                }
                if (settings?.desktopEchoError?.isNotBlank() == true) {
                    Text(
                        "电脑端当前值暂时读不到:${settings.desktopEchoError}",
                        fontSize = 11.sp,
                        color = Pa.TextTertiary,
                        modifier = Modifier.padding(bottom = 6.dp),
                    )
                }

                // 审批策略
                val approvalFiltered = filterByManagedPolicy(
                    listOf(
                        Triple("untrusted", "仅信任", "只自动执行白名单命令"),
                        Triple("on-request", "按需询问", "运行命令前先征求批准"),
                        Triple("never", "完全访问", "不询问,直接执行"),
                    ),
                    allowedApproval,
                ) { it.first }
                if (approvalFiltered.isEmpty()) {
                    // 白名单里的取值我们一个都不认识(codex 加了新的策略档位)。
                    // 这种情况给一个不可点的只读行,而不是让下面的下标访问崩掉。
                    SettingRow(
                        title = "审批策略",
                        desc = "当前取值 ${settings?.approvalPolicy.orEmpty()} 不在 App 支持的选项内",
                        divider = true,
                    )
                } else {
                    // 【第 2 轮补齐】追加"跟随桌面"(空串=不覆盖电脑配置):此前用户一旦设了
                    // 具体值就再也回不到跟随桌面 —— 唯一缺口在候选列表。必须在
                    // filterByManagedPolicy **之后**追加("不覆盖"不该被白名单禁掉);放在
                    // else 内,保证上面的"白名单过滤后为空"兜底仍可达、不被追成永不触发。
                    // 索引与 onMainClick 循环逻辑保持原样(从"跟随桌面"点一下切到第一个真实档位)。
                    val approvalOptions = withFollowDesktop(approvalFiltered)
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
                                // 空串(id 为空)= 跟随桌面,字段为空串或缺省时选中;其余按值匹配。
                                selected = if (id.isEmpty()) {
                                    settings?.approvalPolicy.isNullOrEmpty()
                                } else {
                                    settings?.approvalPolicy == id
                                },
                            ) { writeCodex.copy(approval = id).commit() }
                        },
                    )
                }
                SettingStateLine("approval_policy", settingStates, onRetrySettings)
                DesktopEchoLine(settings, "approval_policy")
                Spacer(Modifier.height(8.dp))

                // 沙箱模式
                val sandboxFiltered = filterByManagedPolicy(
                    listOf(
                        Triple("read-only", "只读", "只能读取文件"),
                        Triple("workspace-write", "工作区可写", "仅工作区内可写"),
                        Triple("danger-full-access", "完全访问", "不限制文件与网络"),
                    ),
                    allowedSandbox,
                ) { it.first }
                if (sandboxFiltered.isEmpty()) {
                    SettingRow(
                        title = "沙箱模式",
                        desc = "当前取值 ${settings?.sandbox.orEmpty()} 不在 App 支持的选项内",
                        divider = true,
                    )
                } else {
                    // 【第 2 轮补齐】同审批策略:在过滤结果之上追加"跟随桌面",放在 else 内。
                    val sandboxOptions = withFollowDesktop(sandboxFiltered)
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
                                selected = if (id.isEmpty()) {
                                    settings?.sandbox.isNullOrEmpty()
                                } else {
                                    settings?.sandbox == id
                                },
                            ) { writeCodex.copy(sandbox = id).commit() }
                        },
                    )
                }
                SettingStateLine("sandbox", settingStates, onRetrySettings)
                DesktopEchoLine(settings, "sandbox")
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
                // 【provider 收窄】"默认模型"写的是 codex 的桌面默认值,候选只保留
                // codex 自己的模型。此前这里是全量列表 —— zcode 并入 ListModels 后
                // (设置页 ZCode 分区的数据源),混在一起会让用户把 "zcode/…" 选进
                // codex 设置,turn/start 传给 codex 必然报错。列表不该给出"选了必错"
                // 的选项;pi/zcode 的模型在各自分区内展示。
                val codexModels = modelChoices.filter { it.provider == runningAgent }
                // 默认模型:主=完整选择器,下拉=快速切换
                SplitButton(
                    label = "默认模型",
                    caption = currentModelName,
                    onMainClick = {
                        if (codexModels.isEmpty()) {
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
                        if (codexModels.isEmpty()) {
                            add(SplitOption("未获取到模型列表", "请检查连接后重试") { })
                        } else {
                            codexModels.forEach { m ->
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
                SettingStateLine("model", settingStates, onRetrySettings)
                // 【桌面只读回显】provider 能力徽标(modelProvider/capabilities/read):
                // 解释"为什么某模型不可用"—— 图像生成/联网搜索是 provider 级声明,
                // 不是模型页的开关。没读到时不渲染,不把"没读到"说成"不支持"。
                val capsBadges = DesktopEchoLogic.providerCapsBadges(settings)
                if (capsBadges.isNotEmpty()) {
                    Row(
                        Modifier.fillMaxWidth().padding(bottom = 6.dp),
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        capsBadges.forEach { badge ->
                            Text(
                                badge,
                                fontSize = 10.5.sp,
                                color = Pa.TextSecondary,
                                modifier = Modifier
                                    .background(Pa.Hover, RoundedCornerShape(8.dp))
                                    .padding(horizontal = 8.dp, vertical = 3.dp),
                            )
                        }
                    }
                }
                DesktopEchoLine(settings, "model")
                Spacer(Modifier.height(8.dp))

                // 思考强度
                // 【第 27 轮补齐】官方枚举还有 none(完全关闭推理,最快)
                // 与 max(比 xhigh 更强)。这两档是否可用取决于模型,
                // 所以文案里点明"需模型支持",避免用户选了没反应以为是 bug。
                // 【第 2 轮补齐】末尾追加"跟随桌面"(空串=不覆盖电脑配置),否则设了具体档位
                // 就再也回不去。本项无托管策略过滤,直接在候选之上追加。
                val effortOptions = effortsFor(modelChoices, settings?.model.orEmpty(), runningAgent)
                    .map { Triple(it.id, it.label, "") }
                val effortIdx = effortOptions
                    .indexOfFirst { it.first == settings?.effort }
                // 未知/为空时如实显示"跟随桌面",不 coerceAtLeast(0) 假装成"关闭"。
                val effortLabel = if (effortIdx >= 0) effortOptions[effortIdx].second else "跟随桌面"
                SplitButton(
                    label = "思考强度",
                    mainLabel = effortLabel,
                    onMainClick = {
                        val next = effortOptions[if (effortIdx >= 0) (effortIdx + 1) % effortOptions.size else 0]
                        writeCodex.copy(effort = next.first).commit()
                    },
                    options = effortOptions.map { (id, label, desc) ->
                        SplitOption(
                            title = label,
                            desc = desc,
                            selected = if (id.isEmpty()) {
                                settings?.effort.isNullOrEmpty()
                            } else {
                                settings?.effort == id
                            },
                        ) { writeCodex.copy(effort = id).commit() }
                    },
                )
                SettingStateLine("effort", settingStates, onRetrySettings)
                DesktopEchoLine(settings, "effort")
                Spacer(Modifier.height(6.dp))
            }

            SectionBlock(icon = R.drawable.ic_lucide_sparkles, title = "Codex 输出偏好") {
                Spacer(Modifier.height(10.dp))
                // 【边界说明】这些偏好只作用于"从 App 发起的任务"。
                // daemon 把它们写进自己拉起的 app-server 子进程内存态,
                // 不会改写用户桌面 codex 的 config.toml。
                Text(
                    "以下参数仅控制 Codex 生成方式；所有 Agent 的思考/工具显示在 Daemon 设置中统一管理。",
                    fontSize = 11.sp, color = Pa.TextTertiary,
                    modifier = Modifier.padding(bottom = 10.dp),
                )

                // 【第 2 轮补齐】末尾追加"跟随桌面",否则设了具体值回不去。
                val verbosityOptions = withFollowDesktop(
                    listOf(
                        Triple("low", "精简", "直给结论,少展开解释"),
                        Triple("medium", "适中", "默认,结论与说明平衡"),
                        Triple("high", "详细", "展开更多背景与推导"),
                    ),
                )
                val verbosityIdx = verbosityOptions
                    .indexOfFirst { it.first == settings?.modelVerbosity }
                // 【不能 coerceAtLeast(0)】取不到值(跟随桌面)时退回第一项会让界面显示
                // "精简" —— 与真实生效值不符。如实写"跟随桌面",并一个选项都不标选中。
                val verbosityLabel = if (verbosityIdx >= 0) verbosityOptions[verbosityIdx].second else "跟随桌面"
                SplitButton(
                    label = "回答详略",
                    mainLabel = verbosityLabel,
                    onMainClick = {
                        val next = verbosityOptions[if (verbosityIdx >= 0) (verbosityIdx + 1) % verbosityOptions.size else 0]
                        writeCodex.copy(verbosity = next.first).commit()
                    },
                    options = verbosityOptions.map { (id, label, desc) ->
                        SplitOption(
                            title = label,
                            desc = desc,
                            selected = if (id.isEmpty()) {
                                settings?.modelVerbosity.isNullOrEmpty()
                            } else {
                                settings?.modelVerbosity == id
                            },
                        ) { writeCodex.copy(verbosity = id).commit() }
                    },
                )
                SettingStateLine("model_verbosity", settingStates, onRetrySettings)
                DesktopEchoLine(settings, "model_verbosity")
                Spacer(Modifier.height(8.dp))

                // 【第 2 轮补齐】末尾追加"跟随桌面",否则设了具体值回不去。
                val summaryOptions = withFollowDesktop(
                    listOf(
                        Triple("concise", "精简", "思考摘要只留关键结论"),
                        Triple("detailed", "详细", "保留较完整的推理脉络"),
                    ),
                )
                val summaryIdx = summaryOptions
                    .indexOfFirst { it.first == settings?.modelReasoningSummary }
                // 同上:未知取值如实显示"跟随桌面",不假装选中第一项。
                val summaryLabel = if (summaryIdx >= 0) summaryOptions[summaryIdx].second else "跟随桌面"
                SplitButton(
                    label = "思考摘要",
                    mainLabel = summaryLabel,
                    onMainClick = {
                        val next = summaryOptions[if (summaryIdx >= 0) (summaryIdx + 1) % summaryOptions.size else 0]
                        writeCodex.copy(reasoningSummary = next.first).commit()
                    },
                    options = summaryOptions.map { (id, label, desc) ->
                        SplitOption(
                            title = label,
                            desc = desc,
                            selected = if (id.isEmpty()) {
                                settings?.modelReasoningSummary.isNullOrEmpty()
                            } else {
                                settings?.modelReasoningSummary == id
                            },
                        ) { writeCodex.copy(reasoningSummary = id).commit() }
                    },
                )
                SettingStateLine("model_reasoning_summary", settingStates, onRetrySettings)
                DesktopEchoLine(settings, "model_reasoning_summary")
                Spacer(Modifier.height(8.dp))

                // 粘贴长文本不中断
                // 【为什么这里不是开关 —— 别再把它改回置灰开关】
                // codex app-server 侧**没有任何等效配置键**:随包 schema 逐字核对过,
                // 全部 schema 文件里 paste / burst 零命中(唯一相近的 model_verbosity /
                // model_reasoning_summary 与粘贴无关)。而"粘贴爆破检测"属于 codex 的
                // **终端输入层** —— 本 App 走的是 stdio JSON-RPC,根本不经过那一层,
                // 所以它既不可能中断粘贴,也没有什么可"关掉"的。
                // 结论:"粘贴长文本不中断"是本通道的**既有事实**,不是待开启的功能。
                //   证据链:手机端 → daemon 全程只做原样封装(daemon 侧 turnInput),
                //   不做任何裁剪;daemon 每轮把实际送出的输入字数写进日志
                //   (`[codex] turn/start … 输入=N字`),可逐轮核对。
                // 旧版这里放了一个置灰开关 + "当前 codex 版本不支持该项",看起来就是占位 UI,
                // 且"不支持"三个字反而让用户以为功能缺失。现在改成如实说明 + "已内置"徽标。
                SettingRow(
                    title = "粘贴长文本不中断",
                    desc = "已内置:粘贴的长文本原样完整送达电脑端,不做任何裁剪",
                    divider = false,
                    control = {
                        Text("已内置", fontSize = 11.sp, color = Pa.Green)
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
                // 项 3a:区块内只保留**唯一入口**,点击进入二级页「技能与插件」。
                // 以前每个 kind 各有一行"展开更多(N)",onClick 却全指向同一个弹窗,
                // 用户看到"两个入口点开的页面完全相同";现在统一为一个页面。
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
                    // 汇总(固定顺序 + 未知 kind 不丢弃,与二级页共用同一套纯函数),
                    // 作为唯一入口的 desc;数量写在右侧 value 位。
                    val byKind = capabilities.groupBy { it.kind }
                    val summary = capabilityKindOrder(byKind.keys).joinToString(" · ") { k ->
                        capabilityKindLabel(k) + " " + byKind[k].orEmpty().size
                    }
                    SettingRow(
                        title = "查看全部技能与插件",
                        desc = summary,
                        value = capabilities.size.toString() + " 个",
                        chevron = true,
                        onClick = onOpenCapabilities,
                    )
                }
            }

            SectionBlock(icon = R.drawable.ic_lucide_server, title = "集成 · MCP 服务器") {
                if (mcpList.isEmpty()) {
                    Text(
                        when {
                            mcpLoading -> "正在读取…"
                            mcpError != null -> "暂时读不到 MCP 列表：$mcpError"
                            else -> "这台电脑未配置 MCP 服务器"
                        },
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
            // 【图 7 实锤的修复】切到 Pi/ZCode 时,这里原来永远是一句
            // "接入完成后在此显示"的占位文案 —— 即使 agent 已点亮、会话能跑。
            // 现在按 agent 给真实内容(数据全部来自已有数据源,不新增 RPC):
            //  - Pi:模型清单 + 动态思考档位(daemon 用最近 pi 会话当探针拿
            //    PiProvider.ThinkingLevels 的实时档位)+ 全局默认模型/强度;
            //  - ZCode:模型清单 + 桌面默认强度 + 额度入口(本批占位)。
            when (runningAgent) {
                "pi", "zcode" -> AgentDefaultsSection(runningAgent, settings, modelChoices, writeCodex, onShowModelPicker)
                else -> if (host?.capabilitiesList?.contains("agent_settings:$runningAgent") == true) {
                    AgentDefaultsSection(runningAgent,settings,modelChoices,writeCodex,onShowModelPicker)
                } else SectionBlock(icon = R.drawable.ic_lucide_info, title = agentDisplay(runningAgent)) {
                    Text(
                        "${agentDisplay(runningAgent)} 的模型、权限与工具设置将在接入完成后在此显示；" +
                            "可先在上方「切换 Agent」里回到 Codex。",
                        fontSize = 12.sp, color = Pa.TextTertiary,
                        modifier = Modifier.padding(vertical = 12.dp),
                    )
                }
            }
        }

        if(runningAgent=="codex") SectionBlock(icon = R.drawable.ic_lucide_user, title = "Codex 账户") {
            SettingRow(title=settings?.account?.takeIf { it.isNotBlank() } ?: "由 Codex 原生账户提供",
                desc="模型与用量来自当前桌面 Agent，不以电脑名称推断登录状态")
        }
        Spacer(Modifier.height(20.dp))
    }
}

/** 思考强度 id → 中文短标签(未知取值原样返回,不猜)。 */
private fun effortLabel(id: String): String = when (id) {
    "none" -> "关闭"
    "low" -> "低"
    "medium" -> "中"
    "high" -> "高"
    "xhigh" -> "极高"
    "max" -> "最高"
    else -> id
}

/**
 * 设置页 · Pi 分区(真实内容,替代"接入完成后在此显示"占位)。
 *
 * 【数据来源(不新增 RPC,全部已有通道)】
 *  - 模型清单 / 动态思考档位:ListModels。daemon 侧对 pi 模型统一填
 *    available_efforts(dynamicEffortsFor 用最近更新的 pi 会话当探针,走
 *    PiProvider.ThinkingLevels 实时拉 get_available_thinking_levels,子进程
 *    没挂载时回档位全集)—— 这正是"从最近 pi 会话拿思考档位"的现成通道;
 *  - 全局默认模型/思考强度:设置页七项经 broadcastPrefs 对 pi 同样生效
 *    (PiProvider 实现 PrefsAware),这里的默认值就是 pi 新会话的起点。
 * 会话内的单独覆盖(会话页模型·强度卡)仍以会话为准,本分区只回答"默认是什么"。
 */
@Composable
private fun PiStatusSection(settings: CodexSettings?, modelChoices: List<ModelInfo>) {
    SectionBlock(icon = R.drawable.ic_lucide_cpu, title = "Pi") {
        Spacer(Modifier.height(10.dp))
        val piModels = modelChoices.filter { it.provider == "pi" }
        // 动态思考档位:pi 的模型共享同一份(daemon 侧按 provider 算一次)
        val efforts = piModels
            .firstNotNullOfOrNull { m -> m.availableEffortsList.takeIf { l -> l.isNotEmpty() } }
        SettingRow(
            title = "默认模型",
            desc = "随设置页更新,pi 新会话按它启动",
            value = settings?.model?.takeIf { it.isNotEmpty() }
                ?.let { id -> modelChoices.find { it.id == id }?.displayName ?: id }
                ?: "跟随桌面配置",
            divider = true,
        )
        SettingRow(
            title = "思考强度",
            desc = "设置页全局默认,对 pi 会话真实生效",
            value = settings?.effort?.takeIf { it.isNotEmpty() }?.let { effortLabel(it) } ?: "跟随桌面",
            divider = true,
        )
        SettingRow(
            title = "可用模型",
            desc = if (piModels.isEmpty()) "未获取到 pi 模型(请检查连接后重试)"
            else piModels.joinToString(" · ") { it.displayName.ifBlank { it.id } },
            value = if (piModels.isEmpty()) "—" else "${piModels.size} 个",
            divider = efforts != null,
        )
        if (efforts != null) {
            SettingRow(
                title = "动态思考档位",
                desc = efforts.joinToString(" / "),
                value = "${efforts.size} 档",
                divider = false,
            )
        }
        Spacer(Modifier.height(6.dp))
        Text(
            "会话内可单独覆盖模型与强度;以上默认值随设置页更新。",
            fontSize = 11.sp, color = Pa.TextTertiary,
            modifier = Modifier.padding(bottom = 6.dp),
        )
    }
}

/**
 * 设置页 · ZCode 分区(真实内容:模型 + 额度入口)。
 *
 * 【模型】同 Pi:来自 ListModels(daemon 侧已把 zcode 并入全局列表;模型表
 * 首拉要建一个一次性 ZCode 会话,TTL 缓存兜住频率,失败时至少有
 * "zcode/default 跟随桌面"兜底项)。
 * 【额度】本批只留**入口占位**:ZCode 的额度接口还没接,给一个点了如实说
 * "开发中"的入口 —— 绝不画假数字、不显示"0%"(那会被读成"额度用完了"),
 * 下批接真实数据后再换成用量面板。
 */
@Composable
private fun ZCodeStatusSection(
    settings: CodexSettings?,
    modelChoices: List<ModelInfo>,
    onFeedback: (String) -> Unit,
) {
    SectionBlock(icon = R.drawable.ic_lucide_cpu, title = "ZCode") {
        Spacer(Modifier.height(10.dp))
        val zcModels = modelChoices.filter { it.provider == "zcode" }
        SettingRow(
            title = "思考强度(桌面默认)",
            desc = "设置页全局默认,对 ZCode 会话真实生效",
            value = settings?.effort?.takeIf { it.isNotEmpty() }?.let { effortLabel(it) } ?: "跟随桌面",
            divider = true,
        )
        SettingRow(
            title = "可用模型",
            desc = if (zcModels.isEmpty()) "未获取到 ZCode 模型(请确认电脑端 ZCode 已安装并启动)"
            else zcModels.joinToString(" · ") { it.displayName.ifBlank { it.id } },
            value = if (zcModels.isEmpty()) "—" else "${zcModels.size} 个",
            divider = true,
        )
        SettingRow(
            title = "额度与用量",
            desc = "ZCode 配额使用情况",
            value = "入口",
            chevron = true,
            onClick = { onFeedback("额度显示开发中,下一批接入") },
        )
        Spacer(Modifier.height(6.dp))
        Text(
            "会话的模型与思考档位在会话内切换;额度展示下一批接入。",
            fontSize = 11.sp, color = Pa.TextTertiary,
            modifier = Modifier.padding(bottom = 6.dp),
        )
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
    /** 当前运行的 agent id:模型行左侧显示对应品牌图标。 */
    agentId: String,
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
                    agentId = agentId,
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
                        agentId = agentId,
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
    /** 提供该模型的 agent id:用来显示品牌图标,让"这模型是谁给的"一眼可辨。 */
    agentId: String = "",
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
        // 品牌图标(28dp,与行内 15sp 标题同档次);未知 agent 不留空洞占位。
        // tint = Color.Unspecified:品牌图标要原色,不能被主题色染掉。
        agentBrandIcon(agentId)?.let { res ->
            Icon(
                painterResource(res), null,
                tint = Color.Unspecified,
                modifier = Modifier.size(28.dp),
            )
            Spacer(Modifier.width(12.dp))
        }
        Column(Modifier.weight(1f)) {
            // 【不截断】以前是 maxLines=1 + 省略号,长模型名(如带日期/后缀的
            // "gpt-5.3-codex-high-2026")只能读半截。用户明确要求"名称完整可见",
            // 这里去掉行数上限,让它按需换行完整显示。
            Text(
                title,
                fontSize = 15.sp,
                color = if (selected) Pa.Accent else Pa.TextPrimary,
            )
            if (subtitle.isNotEmpty()) {
                Text(subtitle, fontSize = 10.5.sp, color = Pa.TextTertiary)
            }
        }
        if (selected) {
            Spacer(Modifier.width(8.dp))
            Icon(
                painterResource(R.drawable.ic_lucide_check), null,
                tint = Pa.Accent, modifier = Modifier.size(20.dp),
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
                Text(title, fontSize = 16.sp, color = Pa.TextPrimary, maxLines = 1)
                if (desc.isNotEmpty()) {
                    Text(
                        desc,
                        Modifier.padding(top = 2.dp),
                        fontSize = 13.sp,
                        color = Pa.TextTertiary,
                        maxLines = 2,
                    )
                }
            }
            if (value != null) {
                Spacer(Modifier.width(8.dp))
                Text(
                    value,
                    fontSize = 14.5.sp,
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
                        // 【不截断】原先 maxLines=1 + Ellipsis 会把长地址再裁一次,
                        // 用户根本抄不全。地址是"可操作信息",必须完整可读。
                        if (connState == "ok") connAddr else "USB → 局域网 → Tailscale",
                        fontSize = 10.5.sp,
                        color = Pa.TextTertiary,
                    )
                }
            }

            // ---- 三种连接方式的完整地址(项 4)----
            // 【为什么单独列出来】以前这里只有一行小字显示**单个**地址,用户反映
            // "只显示 Tailscale,看不到 USB 与局域网"。现在把 AgentConnection 已知的
            // 全部端点(USB 直连 / 局域网直连 / Tailscale)完整列出:完整 ip:port +
            // 一句可执行说明 + 当前通道选中标记,点按即可复制。
            val endpoints = AgentConnection.endpoints()
            if (endpoints.isEmpty()) {
                Text(
                    "尚未发现可用地址。插上 USB 后在电脑上执行 adb reverse tcp:7246 tcp:7246," +
                        "或让手机与电脑处于同一局域网 / Tailscale。",
                    Modifier.padding(top = 4.dp, bottom = 8.dp),
                    fontSize = 11.sp, color = Pa.TextTertiary,
                )
            } else {
                endpoints.forEach { ep ->
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .pressable(haptic = true) {
                                // 复制完整地址:整行可点,右侧给可见的"复制"提示。
                                val cm = ctx.getSystemService(android.content.Context.CLIPBOARD_SERVICE)
                                    as android.content.ClipboardManager
                                cm.setPrimaryClip(android.content.ClipData.newPlainText("地址", ep.addr))
                                Toast.makeText(ctx, "已复制 " + ep.addr, Toast.LENGTH_SHORT).show()
                            }
                            .padding(vertical = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Box(
                            Modifier
                                .size(7.dp)
                                .background(if (ep.active) Pa.Green else Pa.TextTertiary, CircleShape),
                        )
                        Spacer(Modifier.width(9.dp))
                        Column(Modifier.weight(1f)) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Text(
                                    ep.label,
                                    fontSize = 12.5.sp,
                                    color = Pa.TextPrimary,
                                    fontWeight = FontWeight.SemiBold,
                                )
                                if (ep.active) {
                                    Spacer(Modifier.width(7.dp))
                                    Text(
                                        "使用中",
                                        Modifier
                                            .background(Pa.AccentDim, RoundedCornerShape(50))
                                            .padding(horizontal = 6.dp, vertical = 1.dp),
                                        fontSize = 9.5.sp,
                                        color = Pa.Accent,
                                    )
                                }
                            }
                            // 完整 ip:port:不设 maxLines / 不用 Ellipsis,长地址也要看全。
                            Text(
                                ep.addr,
                                Modifier.padding(top = 2.dp),
                                fontSize = 12.sp,
                                color = Pa.TextSecondary,
                            )
                            if (ep.detail.isNotBlank()) {
                                Text(
                                    ep.detail,
                                    Modifier.padding(top = 1.dp),
                                    fontSize = 10.sp,
                                    color = Pa.TextTertiary,
                                )
                            }
                        }
                        Spacer(Modifier.width(8.dp))
                        Text("复制", fontSize = 11.5.sp, color = Pa.Accent)
                    }
                }
                Spacer(Modifier.height(4.dp))
            }

            // ---- 已锁定主机指纹(P0-1 TOFU)----
            // 展示已锁定指纹的前 16 位十六进制;「清除」后回到首连信任 —— daemon 重装
            // 或电脑换证书导致的"指纹不匹配"就从这里解开。随 connectionEpoch 重读,
            // 首次成功握手锁定后这一行会自动出现。
            val epochForPin = AgentConnection.connectionEpoch.collectAsState().value
            val lockedPin = remember(epochForPin) { AgentConnection.lockedFingerprint() }
            if (lockedPin != null) {
                Row(
                    Modifier.fillMaxWidth().padding(vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(Modifier.weight(1f)) {
                        Text(
                            "已锁定主机指纹",
                            fontSize = 12.5.sp,
                            color = Pa.TextPrimary,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Text(
                            lockedPin.take(16) + "…(SHA-256,首次连接时锁定)",
                            Modifier.padding(top = 2.dp),
                            fontSize = 10.5.sp,
                            color = Pa.TextTertiary,
                        )
                    }
                    Text(
                        "清除",
                        Modifier.pressable(haptic = true) {
                            AgentConnection.clearHostPin()
                            Toast.makeText(ctx, "已清除指纹,下次连接将重新信任主机", Toast.LENGTH_SHORT).show()
                        },
                        fontSize = 11.5.sp,
                        color = Pa.Red,
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
                    // 【不省略】失败地址同样必须完整可读,方便用户排查。
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
    // 网页快照缓存(OfflineStore 的 pa_offline/pa_url 分区,与上面几项互不重叠)。
    var urlSnapBytes by remember { mutableLongStateOf(0L) }
    var toastMsg by remember { mutableStateOf<String?>(null) }
    var confirmClearTasks by remember { mutableStateOf(false) }

    /** 统计走 IO 线程:遍历目录不能在 UI 线程做。 */
    fun reload() = scope.launch(kotlinx.coroutines.Dispatchers.IO) {
        val pv = dev.pocketagent.app.ui.component.PreviewCache.usage()
        val sn = dev.pocketagent.app.ui.component.ThreadSnapshot.usage()
        val tp = dev.pocketagent.app.ui.component.PaCache.tempUsage(ctx)
        val br = webDataUsage(ctx)
        // 网页快照缓存占用(独立口径,不与 PreviewCache/ThreadSnapshot/PaCache 重叠)
        val us = dev.pocketagent.app.ui.component.OfflineStore.usage()
        // 真实目录大小 —— 这两个数字就是系统存储页看到的口径
        val cd = dev.pocketagent.app.ui.component.PaCache.cacheUsage(ctx)
        val fd = dev.pocketagent.app.ui.component.PaCache.filesUsage(ctx)
        withContext(kotlinx.coroutines.Dispatchers.Main) {
            previewBytes = pv
            snapshotBytes = sn
            tempBytes = tp
            browserBytes = br
            urlSnapBytes = us
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

        // ---- 清理网页快照缓存(仅在有占用时出现;与上面几项口径互不重叠) ----
        if (urlSnapBytes > 0) {
            CacheActionRow(
                title = "清空网页快照缓存",
                desc = "断网浏览将失效",
            ) {
                scope.launch(kotlinx.coroutines.Dispatchers.IO) {
                    val x = dev.pocketagent.app.ui.component.OfflineStore.clear("url")
                    withContext(kotlinx.coroutines.Dispatchers.Main) {
                        reload()
                        toastMsg = "已释放 ${sizeText(x)}"
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
                "网页快照 ${sizeText(urlSnapBytes)} = 断网时可回看的网页内容(pa_offline/pa_url 分区);" +
                    "与上面几项口径互不重叠,「清空网页快照缓存」只清它。",
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
        Text(value, fontSize = 13.5.sp, color = Pa.TextPrimary)
        Text(label, Modifier.padding(top = 2.dp), fontSize = 11.5.sp, color = Pa.TextTertiary)
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
            Text(title, fontSize = 14.sp, color = if (danger) Pa.Red else Pa.TextPrimary)
            Text(desc, Modifier.padding(top = 2.dp), fontSize = 12.sp, color = Pa.TextTertiary)
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

@Composable
private fun AgentDefaultsSection(agentId: String, settings: CodexSettings?, models: List<ModelInfo>, writer: CodexWriter, openModels: () -> Unit) {
    val choices = models.filter { it.provider == agentId }
    if(agentId=="zcode") NativePermissionsSection(agentId, initial=settings)
    SectionBlock(icon = R.drawable.ic_lucide_cpu, title = agentDisplay(agentId)) {
        SplitButton(label = "默认模型", mainLabel = choices.find { it.id == settings?.model }?.displayName ?: "跟随 Agent 默认",
            onMainClick = openModels,
            options = listOf(SplitOption("跟随 Agent 默认", selected = settings?.model.isNullOrEmpty()) { writer.copy(model="").commit() }) +
                choices.filter { !it.id.endsWith("/default") }.map { m -> SplitOption(m.displayName, selected=m.id==settings?.model) { writer.copy(model=m.id).commit() } })
        val levels = effortsFor(choices, settings?.model.orEmpty(), agentId)
        SplitButton(label = "思考强度", mainLabel = effortLabelOf(settings?.effort.orEmpty()),
            onMainClick = {
                val index = levels.indexOfFirst { it.id == settings?.effort }
                writer.copy(effort=levels[(index+1)%levels.size].id).commit()
            }, options = levels.map { e -> SplitOption(e.label, selected=e.id==settings?.effort) { writer.copy(effort=e.id).commit() } })
        Text("${(levels.size-1).coerceAtLeast(0)} 个已报告档位 · 下次发送时应用", fontSize=12.sp, color=Pa.TextSecondary,
            modifier=Modifier.padding(vertical=10.dp))
    }
}

@Composable
private fun DaemonPage(onBack: () -> Unit, onFeedback: (String) -> Unit, writer: CodexWriter, models: List<ModelInfo>, capabilities: List<String>) {
    SubPageScaffold(title = "Daemon 桥接与输入", onBack = onBack) {
        SectionBlock(icon = R.drawable.ic_lucide_server, title = "连接策略") {
            Text("USB → 局域网直连 → Tailscale 保底。已连接时保持当前通道，断流后自动恢复任务结果。",
                fontSize = 13.sp, color = Pa.TextSecondary, modifier = Modifier.padding(vertical=12.dp))
        }
        SectionBlock(icon = R.drawable.ic_lucide_brain, title = "全局对话显示") {
            Spacer(Modifier.height(10.dp))
            // 显示思考过程(展示/隐藏)
            SplitButton(
                label = "显示思考过程",
                mainLabel = if (AgentConnection.showThinking) "展示" else "隐藏",
                onMainClick = {
                    writer.copy(hideReasoning = AgentConnection.showThinking).commit()
                    onFeedback(
                        if (AgentConnection.showThinking) "将流式显示思考过程" else "将只显示对话结果",
                    )
                },
                options = listOf(
                    SplitOption("展示", selected = AgentConnection.showThinking) {
                        writer.copy(hideReasoning = false).commit()
                        onFeedback("将流式显示思考过程")
                    },
                    SplitOption("隐藏", selected = !AgentConnection.showThinking) {
                        writer.copy(hideReasoning = true).commit()
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

        // 实验功能:提示词优化(默认关;依赖电脑端 ZCode 且消耗其 Token Plan 配额)
        SectionBlock(icon = R.drawable.ic_lucide_sparkles, title = "实验功能") {
            Spacer(Modifier.height(10.dp))
            SplitButton(
                label = "优化提示词按钮",
                caption = "在会话输入栏显示「优化」按钮,把口语化输入改写成给 AI 的清晰指令;由下方选定的 Agent 与模型生成，会消耗对应配额",
                mainLabel = if (AgentConnection.optimizePromptEnabled) "开" else "关",
                onMainClick = {
                    AgentConnection.optimizePromptEnabled = !AgentConnection.optimizePromptEnabled
                    onFeedback(if (AgentConnection.optimizePromptEnabled) "已在输入栏启用优化按钮" else "已关闭优化按钮")
                },
                options = listOf(
                    SplitOption("开", selected = AgentConnection.optimizePromptEnabled) {
                        AgentConnection.optimizePromptEnabled = true
                        onFeedback("已在输入栏启用优化按钮")
                    },
                    SplitOption("关", selected = !AgentConnection.optimizePromptEnabled) {
                        AgentConnection.optimizePromptEnabled = false
                        onFeedback("已关闭优化按钮")
                    },
                ),
            )
            val optimizerAgents = models.map { it.provider }.distinct().filter {
                "text_generation:$it" in capabilities || (capabilities.none { c -> c.startsWith("text_generation:") } && it in setOf("codex","pi","zcode"))
            }
            SplitButton(label="优化使用的 Agent", mainLabel=agentDisplay(AgentConnection.optimizeAgent),
                onMainClick={ if(optimizerAgents.isNotEmpty()) {
                    val index=optimizerAgents.indexOf(AgentConnection.optimizeAgent)
                    AgentConnection.setOptimizer(optimizerAgents[(index+1)%optimizerAgents.size], "")
                } },
                options=optimizerAgents.map { id -> SplitOption(agentDisplay(id), selected=id==AgentConnection.optimizeAgent) { AgentConnection.setOptimizer(id, "") } })
            val optimizerModels=models.filter { it.provider==AgentConnection.optimizeAgent && !it.id.endsWith("/default") }
            SplitButton(label="优化使用的模型", mainLabel=optimizerModels.find { it.id==AgentConnection.optimizeModel }?.displayName ?: "Agent 默认模型",
                onMainClick={ if(optimizerModels.isNotEmpty()) AgentConnection.setOptimizer(AgentConnection.optimizeAgent,optimizerModels.first().id) },
                options=listOf(SplitOption("Agent 默认模型", selected=AgentConnection.optimizeModel.isEmpty()) { AgentConnection.setOptimizer(AgentConnection.optimizeAgent, "") }) +
                    optimizerModels.map { m -> SplitOption(m.displayName, selected=m.id==AgentConnection.optimizeModel) { AgentConnection.setOptimizer(AgentConnection.optimizeAgent,m.id) } })
            Spacer(Modifier.height(6.dp))
        }

        // 【端侧语音】离线识别模型管理:系统识别不可用的设备(无识别服务 ROM/无 GMS)
        // 靠它落地语音输入 —— 下载 / 进度 / 失败重试 / 删除释放空间。
        SectionBlock(icon = R.drawable.ic_lucide_mic, title = "语音识别") {
            Spacer(Modifier.height(10.dp))
            val context = LocalContext.current
            LaunchedEffect(Unit) { SenseVoiceModelManager.init(context) }
            val speech by SenseVoiceModelManager.state.collectAsState()
            SettingRow(
                title = "SenseVoice Small INT8（App 内置）",
                desc = speech.error
                    ?: "符合中文输入习惯 · 自动标点与数字规范化 · 音频不离开设备",
                value = when {
                    speech.ready -> "已就绪"
                    speech.preparing -> "初始化 ${speech.preparedBytes * 100 / SenseVoiceModelManager.TOTAL_BYTES}%"
                    else -> "内置"
                },
                divider = false,
            )
            Text(
                "有系统识别服务时可直接使用；否则使用 App 内置 SenseVoice。录音权限被拒绝时只请求权限，不提供外部模型下载入口。",
                fontSize = 11.sp,
                color = Pa.TextSecondary,
            )
            Spacer(Modifier.height(6.dp))
        }
    }
}

@Composable
internal fun NativePermissionsSection(agentId: String, threadId: String = "", initial: CodexSettings? = null) {
    val scope=rememberCoroutineScope()
    var native by remember(agentId,threadId,initial) { mutableStateOf(initial) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var refresh by remember { mutableStateOf(0) }
    LaunchedEffect(agentId,threadId,refresh) {
        if(initial!=null && refresh==0 && threadId.isBlank()) return@LaunchedEffect
        busy=true
        try {
            native=withContext(Dispatchers.IO) { AgentConnection.modelServiceSlow().getCodexSettings(
                GetCodexSettingsRequest.newBuilder().setAgentId(agentId).setThreadId(threadId).build()) }
            error=null
        } catch(e: Exception) { error=e.message } finally { busy=false }
    }
    fun select(mode: String) {
        if(busy) return
        busy=true
        scope.launch {
            try {
                native=withContext(Dispatchers.IO) { AgentConnection.modelServiceSlow().setCodexSettings(
                    SetCodexSettingsRequest.newBuilder().setAgentId(agentId).setThreadId(threadId).setNativeMode(mode).build()) }
                error=null
            } catch(e: Exception) { error="模式未修改：${e.message}" } finally { busy=false }
        }
    }
    val options=native?.nativeModesList.orEmpty().distinct()
    SectionBlock(icon=R.drawable.ic_lucide_shield,title=if(threadId.isBlank()) "ZCode 权限默认值" else "当前会话权限") {
        SplitButton(label="原生权限模式",mainLabel=if(busy) "同步中…" else nativeModeLabel(native?.nativeMode.orEmpty()),
            onMainClick={refresh++}, options=buildList {
                if(threadId.isBlank()) add(SplitOption("跟随原生默认",selected=native?.nativeMode.isNullOrEmpty()) { select("") })
                options.forEach { value -> add(SplitOption(nativeModeLabel(value), desc=value,selected=native?.nativeMode==value) { select(value) }) }
            })
        Text(error ?: native?.nativePermissionsNote.orEmpty(),fontSize=11.sp,color=Pa.TextSecondary,modifier=Modifier.padding(vertical=8.dp))
        if(threadId.isBlank()) Text("默认值用于新建会话；已有会话在对话的更多菜单中调整。",fontSize=11.sp,color=Pa.TextTertiary)
    }
}
internal fun nativeModeLabel(mode: String): String = when(mode) {
    "" -> "跟随原生默认"
    "build" -> "标准构建（build）"
    "edit" -> "自动编辑（edit）"
    "auto" -> "原生自动审批（auto）"
    "yolo" -> "跳过审批（yolo）"
    "plan" -> "计划（原生处理）"
    else -> mode
}
