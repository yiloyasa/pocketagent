package dev.pocketagent.app.ui

import android.widget.Toast
import dev.pocketagent.app.BuildConfig
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.snap
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
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
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.RuntimeControlState
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.component.PaHostChip
import dev.pocketagent.app.ui.component.PaHostLink
import dev.pocketagent.app.ui.component.PaNavIconButton
import dev.pocketagent.app.ui.component.PaPaths
import dev.pocketagent.app.ui.component.PaScreenScaffold
import dev.pocketagent.app.ui.component.PaSheet
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
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.EditorialTone
import dev.pocketagent.app.ui.theme.paPushBack
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
import pocketagent.v1.ListAgentConfigOptionsRequest
import pocketagent.v1.ListCodexCapabilitiesRequest
import pocketagent.v1.ListMcpServersRequest
import pocketagent.v1.ListModelsRequest
import pocketagent.v1.McpServerStatus
import pocketagent.v1.ModelInfo
import pocketagent.v1.RateLimitWindow
import pocketagent.v1.SetCodexSettingsRequest

// 【色板并轨 2026-10-09】editorialM3Scheme() 原定义在本文件、只包 5 屏子树;
// 现已上提 theme 层(ui/theme/PaEditorial.kt)并由 MainActivity NavHost 外层
// 一次包全 app,本文件的同名定义删除。

/**
 * 编辑式页面的系统栏图标适配:按当前盘 IsDark 决定图标深浅(浅盘→深色图标,
 * 深/黑客盘→浅色图标),键控 IsDark,切换显示模式即重挂 ——
 * MainActivity 的全局挂接不动,只作用于编辑式页面。
 */
@Composable
internal fun EditorialSystemBars() {
    val view = androidx.compose.ui.platform.LocalView.current
    val context = androidx.compose.ui.platform.LocalContext.current
    // 键控当前盘:浅色盘→深色图标;深色/黑客盘→浅色图标;切换显示模式即重挂。
    androidx.compose.runtime.DisposableEffect(PaEditorial.IsDark) {
        val window = (context as? android.app.Activity)?.window
        val controller = window?.let { androidx.core.view.WindowCompat.getInsetsController(it, view) }
        controller?.isAppearanceLightStatusBars = !PaEditorial.IsDark
        controller?.isAppearanceLightNavigationBars = !PaEditorial.IsDark
        onDispose { }
    }
}

/**
 * agent id → 展示名映射(**仅展示用途,非能力名单** —— P1-6 动态化边界)。
 *
 * 「切换 Agent」列表的 agent 集合来自 [buildAgents](按 HostInfo.capabilities
 * 动态构建,daemon 新增 provider 自动进列表);本表只负责"id → 人话名"的展示翻译,
 * 缺省 id 由 [agentDisplay] 直接回退显示 id 原文(AUDIT §3.3:未知值显示原生原文)。
 * 【图 7 实锤的历史坑】此前"切换 Agent"列表由硬编码名单构建,漏了 zcode 导致
 * 永远切不到 —— 名单职责已移交动态数据源,本表不再参与任何能力判定,
 * 只做展示翻译(缺省有兜底,不存在"漏了就切不到"的失败模式)。
 */
private val AGENT_DISPLAY = mapOf(
    "codex" to "Codex",
    "zcode" to "ZCode",
    "opencode" to "OpenCode",
    "claude-code" to "Claude",
    "pi" to "Pi",
    "hermes" to "Hermes",
)

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

/**
 * 状态胶囊:MCP"就绪/失败"这类一眼判断。
 * 【统一卡语言】卡内禁止带底色子容器 → 纯语义色文字,层级靠颜色不靠底。
 */
@Composable
private fun StatusChip(text: String, color: Color) {
    Text(
        text,
        color = color,
        fontSize = PaEditorial.Type.Micro.fontSize,
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
            fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
            modifier = Modifier.padding(vertical = 12.dp),
        )
        return
    }
    if (usage.error.isNotBlank()) {
        Text("暂时读不到额度", fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Ink)
        Spacer(Modifier.height(4.dp))
        Text(usage.error, fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
        return
    }

    Row(verticalAlignment = Alignment.CenterVertically) {
        Text(
            usage.planType.ifBlank { "未报告套餐" },
            Modifier.weight(1f),
            fontSize = PaEditorial.Type.BodySmall.fontSize,
            color = PaEditorial.Ink,
            fontWeight = FontWeight.Medium,
        )
        if (usage.spendControlReached) {
            StatusChip("已达上限", EditorialTone.DENIED.text)
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
        Text(extras.joinToString(" · "), fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
    }
    if (usage.hasCredits || usage.creditBalance.isNotBlank()) {
        Spacer(Modifier.height(4.dp))
        Text(
            if (usage.unlimitedCredits) "额度不限量" else "剩余额度 ${usage.creditBalance}",
            fontSize = PaEditorial.Type.Caption.fontSize,
            color = PaEditorial.Muted,
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
        2 -> PaEditorial.Denied
        1 -> EditorialTone.ATTENTION.accent
        else -> PaEditorial.Running
    }
    val reset = resetCountdownLabel(window.resetsAtUnixS, System.currentTimeMillis())

    Column(Modifier.fillMaxWidth()) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                rateLimitWindowLabel(window.windowDurationMins, fallbackLabel),
                Modifier.weight(1f),
                fontSize = PaEditorial.Type.Caption.fontSize,
                color = PaEditorial.Muted,
            )
            Text("$pct%", fontSize = PaEditorial.Type.Caption.fontSize, color = barColor, fontWeight = FontWeight.Medium)
        }
        Spacer(Modifier.height(5.dp))
        Box(
            Modifier
                .fillMaxWidth()
                .height(6.dp)
                .background(PaEditorial.Canvas, RoundedCornerShape(50)),
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
            Text(reset, fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted)
        }
    }
}

/** 大数字缩写成 k / M,避免"1234567 tokens"这种读不出量级的写法。 */
private fun formatTokenCount(value: Long): String = when {
    value >= 1_000_000L -> String.format(java.util.Locale.US, "%.1fM", value / 1_000_000.0)
    value >= 1_000L -> String.format(java.util.Locale.US, "%.0fk", value / 1_000.0)
    else -> value.toString()
}
/** 托管策略生效时的说明:卡内扁平行(盾图标裸放 + 文字),不做底色子容器。 */
@Composable
private fun ManagedPolicyNote(text: String) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = 11.dp, vertical = 9.dp),
        verticalAlignment = Alignment.Top,
    ) {
        Icon(
            painterResource(R.drawable.ic_lucide_shield),
            null,
            tint = EditorialTone.ATTENTION.accent,
            modifier = Modifier.size(13.dp).padding(top = 1.dp),
        )
        Spacer(Modifier.width(7.dp))
        Text(
            text,
            fontSize = PaEditorial.Type.Caption.fontSize,
            color = PaEditorial.Muted,
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

/**
 * agent id → 人话名(内部可见:AgentScopeBadge 的作用域标签复用同一份词表,
 * 不另立映射 —— 见 AgentScopeBadge.kt 头注的"名称与图标都只有一份来源")。
 */
internal fun agentDisplay(id: String): String = AGENT_DISPLAY[id] ?: id.ifBlank { "Agent" }

/**
 * 全 app 唯一卡型(编辑式统一卡语言):18dp 圆角 + 瓷白实色底 + 1dp 细线描边
 * (premium.css .status-card/.card;无阴影堆叠,层级靠留白与细线)。
 *
 * 【为什么收敛到一个函数】此前玻璃/实色/发丝边/阴影混用,用户裁定「层级很脏」——
 * 乱的不是卡片本身,是卡型不统一。现在设置页所有分区卡只此一种材质;
 * 行间分隔统一 1dp Hairline 细线。
 */
private fun Modifier.paSettingsCard(shape: Shape = RoundedCornerShape(PaEditorial.Radius.Card)): Modifier = composed {
    this
        .background(PaEditorial.Surface, shape)
        .border(1.dp, PaEditorial.Hairline, shape)
}

/**
 * 展开收起(高度动画)的令牌化规格。
 *
 * 【为什么要在这个文件里再包一层】animateContentSize 要求 FiniteAnimationSpec,
 * 而 PaMotion.instant() 的声明返回类型是宽化的 AnimationSpec,传不进去;
 * 令牌文件(PaMotion.kt)禁改,所以这里用同值的 snap() 承接"瞬时到位"档——
 * 默认档是 PaMotion.emphasized()(轻微过冲的揭示,展开收起读作"跟手")。
 */
private fun Modifier.paExpandable(reduceMotion: Boolean): Modifier =
    this.animateContentSize(if (reduceMotion) snap<IntSize>() else PaMotion.emphasized())

// ---- 设置搜索(审计 P1-3 子项;纯数据+纯函数集中在此,JVM 单测) ----

/**
 * 设置搜索索引的一条:一行设置(一级导航行或二级页的代表性分区)的人话入口。
 *
 * 【为什么是静态索引】设置行的 title/desc 散在数千行 Composable 里,没有可枚举的
 * 结构;搜索的目标是"帮用户找到东西在哪个分区",所以索引只收 **分区级** 条目
 * (一级行 + 二级页代表分区),不逐开关收录 —— 逐开关索引会随 UI 演进腐烂,
 * 分区级索引改版时才需要跟着改。
 *
 * @property target 命中后的落点(subPage id);null = 该行不可跳转,仅展示归属。
 */
internal data class SettingsSearchEntry(
    val title: String,
    val desc: String,
    val group: String,
    val target: String? = null,
)

/**
 * 设置搜索索引(静态,字符串与各分区行的 title/desc 同源 —— 改分区文案时
 * 这里一并改)。覆盖:一级导航行(连接 / Agent / 外观 / 会话与对话 / 数据与缓存 /
 * 安全 / 关于 的 S2 新分组)、Agent 与模型/Daemon/用户数据各页的代表分区、
 * 运行时控制台。
 */
internal fun settingsSearchIndex(): List<SettingsSearchEntry> = listOf(
    SettingsSearchEntry("连接与配对", "地址 · 配对码 · 主机指纹", "连接", "userdata"),
    SettingsSearchEntry("Daemon 桥接与输入", "连接策略 · 语音 · 实验功能", "连接", "daemon"),
    SettingsSearchEntry("用户数据管理", "连接 · 配对 · 下载内容 · 存储占用", "连接", "userdata"),
    SettingsSearchEntry("会话与对话", "显示思考 · 工具详情 · 历史轮数", "会话与对话"),
    SettingsSearchEntry("Agent 与模型", "切换 Agent · 权限 · 模型 · MCP", "Agent", "agent"),
    SettingsSearchEntry("外观与阅读", "显示模式 · 文字尺寸", "外观", "appearance"),
    SettingsSearchEntry("下载与存储", "下载内容 · 缓存清理 · 存储占用", "数据与缓存", "userdata"),
    SettingsSearchEntry("审批与沙箱", "命令要不要批准 · 文件可写范围", "安全", "agent"),
    SettingsSearchEntry("主机信任", "首连锁定电脑指纹 · 防止连到别的机器", "安全", "userdata"),
    SettingsSearchEntry("电脑控制台", "预算 · 运行时形态 · 记忆透析", "运行时"),
    SettingsSearchEntry("版本", "版本号 · 电脑系统信息", "关于"),
    SettingsSearchEntry("显示模式", "跟随系统 · 浅色 · 深色 · 黑客终端", "外观与阅读", "appearance"),
    SettingsSearchEntry("字号", "界面文字缩放 · 思考文字缩放", "外观与阅读", "appearance"),
    SettingsSearchEntry("语音识别", "SenseVoice · 按需下载模型", "Daemon 桥接与输入", "daemon"),
    SettingsSearchEntry("切换 Agent", "Codex · Pi · ZCode", "Agent 与模型", "agent"),
    SettingsSearchEntry("审批策略", "仅信任 · 按需询问 · 完全访问", "Agent 与模型", "agent"),
    SettingsSearchEntry("沙箱模式", "读写权限 · 网络访问", "Agent 与模型", "agent"),
    SettingsSearchEntry("模型与思考", "默认模型 · 思考强度 · 可用模型", "Agent 与模型", "agent"),
    SettingsSearchEntry("技能与插件", "能力 · 技能 · 插件列表", "Agent 与模型", "agent"),
    SettingsSearchEntry("MCP 服务器", "集成 · 服务器状态", "Agent 与模型", "agent"),
    SettingsSearchEntry("下载内容", "文件 · 网页 · 快照缓存", "用户数据管理", "userdata"),
    SettingsSearchEntry("存储占用", "清理缓存 · 浏览器数据 · 会话快照", "用户数据管理", "userdata"),
)

/**
 * Daemon 桥接与输入行的值摘要(S2:对话显示散项已聚拢到一级「会话与对话」组,
 * 该页剩的可调项里唯一有开关状态的是实验功能的优化按钮 —— 摘要如实取它;
 * 连接策略是固定事实,语音识别无开关,都不进摘要)。
 */
internal fun daemonBridgeSummary(optimizePromptEnabled: Boolean): String =
    if (optimizePromptEnabled) "优化按钮开" else "优化按钮关"

/**
 * 设置搜索过滤(纯函数):空/空白查询 = 不进入搜索模式(返回空列表,页面照常
 * 渲染分组);非空对 title+desc 大小写不敏感匹配。
 */
internal fun filterSettingsEntries(
    query: String,
    entries: List<SettingsSearchEntry>,
): List<SettingsSearchEntry> {
    val q = query.trim()
    if (q.isEmpty()) return emptyList()
    return entries.filter {
        it.title.contains(q, ignoreCase = true) || it.desc.contains(q, ignoreCase = true)
    }
}

/**
 * 命中区间(纯函数,给标题高亮用):title 命中返回区间,否则 null(命中发生在
 * desc 时标题不高亮,展示层用组名点明归属);空查询恒 null。
 */
internal fun matchRange(title: String, query: String): IntRange? {
    val q = query.trim()
    if (q.isEmpty()) return null
    val idx = title.indexOf(q, ignoreCase = true)
    return if (idx < 0) null else idx until (idx + q.length)
}

/**
 * 设置页(重做版:一级导航、二级编辑)。
 *
 * 【信息架构(S2 调整:按"使用频率 + 风险"重排)】
 *  1. 一级页 = 电脑连接卡(hostStatus 槽)+ 七个卡组:连接(最高频)/ Agent /
 *     外观 / 会话与对话(S2 新增:显示思考 / 工具详情 / 历史轮数,从 Daemon 页
 *     散项聚拢)/ 数据与缓存 / 安全 / 关于(另有运行时=控制中心区段,低频专业域,
 *     能力位点亮才出现)。相关项聚成卡组,组间留白 > 组内行距;每个导航行带
 *     "当前值摘要"——本地偏好断连照常出值,依赖主机的项断连如实标"需连接主机";
 *  2. 所有编辑动作收进二级页:连接与配对+下载与存储 → 用户数据管理 /
 *     Daemon 桥接与输入(连接策略 · 语音 · 实验功能)/ 外观与阅读 / Agent 与模型
 *     (安全组的两行分别深链前两者);
 *  3. 多选项设置仍统一 SplitButton;危险操作(撤销信任/删除数据)二次确认 +
 *     说明后果;开关给状态词与轻触感反馈;
 *  4. 【S2 作用域徽章】"仅作用于某 agent"的分区说明统一收敛为 AgentScopeBadge
 *     小标签(codex/pi/zcode/hermes),谁生效一眼可辨。
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
    // 【色板并轨 2026-10-09】编辑式 M3 色板已上提 MainActivity NavHost 外层包全 app,
    // 本页不再重复包裹
    SettingsScreenBody(onBack, openUserData, onOpenBrowser, modifier)
}

@Composable
private fun SettingsScreenBody(
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
    // 【T-32】配对失败的 daemon 原文。独立状态承载:connError 会被后台自动重连循环
    // 反复覆盖,不能用来展示"配对码错误"这类必须稳定可见的配对结果。
    var pairError by remember { mutableStateOf<String?>(null) }
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
                    // T-DSH 问题 A:成功快照写入共享缓存(与主页网格同一数据源),
                    // 「切换 Agent」的点亮判定不再吃本页独立快照的失败/未就绪窗口。
                    host?.let { AgentConnection.updateHostCapabilities(it.capabilitiesList) }
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
        pairError = null
        scope.launch {
            val result = try {
                withContext(Dispatchers.IO) {
                    // 【配对顺序修复(release-smoke-20261009)】顺序编排收进
                    // saveAndTestConnection(可测核心):先 switchAddress/setCredentials 把
                    // 通道指向新填地址,配对码再在**新目标**上 authorize —— 旧实现先在
                    // 当前通道 authorize,同机有第二个 daemon 时配对码发给错误实例。
                    // 「保存并测试」仍一次完成。原 T-32 注释(配对码形态 → 先注册后连接)
                    // 的形态分流语义不变,只是切地址提前到配对之前。
                    dev.pocketagent.app.saveAndTestConnection(addr, token, dev.pocketagent.app.AgentConnectionSaveOps)
                }
            } catch (e: Exception) {
                settingsError = "连接失败：${humanizeError(e)}"
                connectionSaving = false
                return@launch
            }
            connectionSaving = false
            // 配对成功:输入框同步成真 token(saveAndTestConnection 返回的权威值)——
            // 否则 8 位配对码残留在 token 位,下次保存会把真 token 冲掉。
            result.authToken?.let { token = it }
            when {
                result.connected -> {
                    Toast.makeText(
                        ctx,
                        "连接成功 · ${AgentConnection.connTransport.value}",
                        Toast.LENGTH_SHORT,
                    ).show()
                }
                result.pairError != null -> {
                    // daemon 原文如实透传(AUDIT §3.3:如"配对码错误"),不加工不改写。
                    pairError = result.pairError
                    Toast.makeText(ctx, "配对失败", Toast.LENGTH_SHORT).show()
                }
                else -> {
                    settingsError = AgentConnection.connError.value?.message ?: "连接失败"
                }
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
                settingsError = "保存失败：${humanizeError(e)}"
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
                settingsError = "重扫失败：${humanizeError(e)}"
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
    // 设置搜索(审计 P1-3 子项):非空 = 搜索模式,分组列表让位给过滤后的命中行
    var settingsQuery by remember { mutableStateOf("") }

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
            pairError = pairError,
            onSaveAndTest = { saveAndTestConnection() },
            onSaveOnly = { saveCredentialsOnly() },
            onRescan = { rescanCandidates() },
            onOpenBrowser = onOpenBrowser,
        )
        return
    }
    if (subPage == "daemon") {
        // 【F3 终验】S2 把「全局对话显示」上收一级页后,DaemonPage 不再需要写入通道
        // (思考三项走上方 codexWriter),原 writer 形参随之移除 —— 一个偏好只有一处控件。
        DaemonPage(onBack = { subPage = null }, onFeedback = { showSettingsFeedback(it) }, models = modelChoices, capabilities = host?.capabilitiesList.orEmpty())
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
        // 【推背模糊】Agent 二级页内也会弹出审批预览玻璃浮层,内容根同样后退一层。
        Box(Modifier.fillMaxSize().paPushBack(showApprovalPreview || RuntimeControlState.dialysisSheetOpen)) {
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
        }
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
    // 【版本协商闸 A5】daemon 随探活报告 min_app_version,与本机版本分量比较后点亮
    // (AgentConnection 上轮已就位的 StateFlow,语义化比较在 ConnectionDiagnostics,
    // 7 例单测钉死)。只提示不阻断:这里只订阅出值,探活/连接流程零感知。
    val appVersionOutdatedTop = AgentConnection.appVersionOutdated.collectAsState().value
    val minAppVersionTop = AgentConnection.minAppVersionRequired.collectAsState().value
    // 各导航行的"当前值摘要"(R1-C:不用点进去就知道现状)——取值映射全部在
    // SettingsOverviewLogic 的纯函数里(单测钉死),这里只喂状态。
    val connSummary = connectionSummary(connStateTop, connTransportTop)
    // Daemon 桥接与输入行(S2:对话显示三项已聚拢到下方「会话与对话」组就地展示,
    // 该行摘要改取页内唯一带状态的实验功能开关;本地偏好,断连可读)。
    val daemonSummary = daemonBridgeSummary(AgentConnection.optimizePromptEnabled)
    // 下载与存储行:本地任务清单(断连可用),只数已完成的下载。
    val allTasksTop by TaskCenter.tasks.collectAsState()
    val doneDownloadsTop = allTasksTop.count {
        it.kind == TaskCenter.Kind.DOWNLOAD && it.status == TaskCenter.Status.DONE
    }
    // 主机信任行:随 connectionEpoch 重读(首次握手锁定后从"未锁定"变"已锁定")。
    val epochTop = AgentConnection.connectionEpoch.collectAsState().value
    val lockedPinTop = remember(epochTop) { AgentConnection.lockedFingerprint() }
    val themeLabel = listOf(
        "system" to "跟随系统", "light" to "浅色", "dark" to "深色",
        "hacker" to "黑客终端",
    ).firstOrNull { it.first == AgentConnection.themeMode }?.second ?: "跟随系统"
    val uiScaleLabel = when {
        AgentConnection.uiTextScale < 0.95f -> "小"
        AgentConnection.uiTextScale > 1.05f -> "大"
        else -> "标准"
    }

    // ---------------- 一级页 ----------------
    // 编辑式骨架(README:状态栏 → 顶部导航 → 主机状态 → 内容);系统栏图标
    // 跟编辑式浅色画布走(深色显示模式下内容区也是浅的,图标必须用深色)。
    EditorialSystemBars()
    // 【推背模糊】玻璃浮层(审批预览 PaSheet)打开时,设置内容自身模糊+缩小+压暗——
    // 透过玻璃看过去才是真磨砂(PaGlass 职责分离:模糊在内容层,不在浮层上)。
    PaScreenScaffold(
        title = "设置",
        modifier = modifier
            .fillMaxSize()
            .paPushBack(showApprovalPreview || RuntimeControlState.dialysisSheetOpen),
        leading = {
            PaNavIconButton(
                onClick = onBack,
                contentDescription = "返回",
                iconRes = R.drawable.ic_lucide_chevron_left,
                plain = true,
            )
        },
        hostStatus = {
            // 当前主机(PaHostChip 唯一事实源):点进去 = 连接与配对(用户数据管理页顶部)。
            Column {
                Text(
                    "当前连接",
                    Modifier.padding(start = 2.dp, bottom = 8.dp),
                    fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp,
                    letterSpacing = 1.2.sp, fontWeight = FontWeight.SemiBold,
                    color = PaEditorial.Muted,
                )
                PaHostChip(
                    hostName = host?.displayName?.ifBlank { "我的电脑" } ?: "我的电脑",
                    link = when (connStateTop) {
                        "ok" -> PaHostLink.CONNECTED
                        "down" -> PaHostLink.OFFLINE
                        else -> PaHostLink.CONNECTING
                    },
                    channelLabel = listOfNotNull(
                        connTransportTop.ifBlank { "直连" },
                        host?.os?.takeIf { it.isNotBlank() },
                    ).joinToString(" · "),
                    onClick = { subPage = "userdata" },
                )
                // 版本闸提示行(纯提示位,不做弹窗/不阻断):daemon 报告的最低版本
                // 高于本机 App 时,在连接信息卡内联一条警示,风格随设置页既有警示行
                // (settingsError 同款:DeniedContainer 底 + DENIED 文字,全取
                // PaEditorial 槽位,禁新色)。
                if (appVersionOutdatedTop) {
                    Spacer(Modifier.height(8.dp))
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .background(PaEditorial.DeniedContainer, RoundedCornerShape(PaEditorial.Radius.IconButton))
                            .padding(horizontal = 12.dp, vertical = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(
                            painterResource(R.drawable.ic_lucide_triangle_alert), null,
                            tint = EditorialTone.DENIED.text, modifier = Modifier.size(14.dp),
                        )
                        Spacer(Modifier.width(8.dp))
                        Text(
                            appVersionOutdatedHint(minAppVersionTop),
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            color = EditorialTone.DENIED.text,
                        )
                    }
                }
            }
        },
    ) {
        LazyColumn(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .navigationBarsPadding(),
            contentPadding = PaddingValues(top = 6.dp, bottom = 20.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            item(key = "banner") {
                Column {
                    settingsError?.let { message ->
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .background(PaEditorial.DeniedContainer, RoundedCornerShape(PaEditorial.Radius.IconButton))
                                .padding(horizontal = 14.dp, vertical = 10.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Icon(
                                painterResource(R.drawable.ic_lucide_triangle_alert), null,
                                tint = EditorialTone.DENIED.text, modifier = Modifier.size(15.dp),
                            )
                            Spacer(Modifier.width(9.dp))
                            Text(message, Modifier.weight(1f), fontSize = PaEditorial.Type.Caption.fontSize, color = EditorialTone.DENIED.text)
                            TextButton(onClick = {
                                settingsError = null
                                AgentConnection.connectionEpoch.value += 1
                            }) {
                                Text("重试", color = PaEditorial.Primary)
                            }
                        }
                    }
                    settingsFeedback?.let { message ->
                        Text(
                            message,
                            Modifier.fillMaxWidth().padding(horizontal = 6.dp),
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            color = when {
                                message.contains("失败") -> EditorialTone.DENIED.text
                                message.contains("保存中") || message.contains("未生效") -> PaEditorial.Primary
                                else -> EditorialTone.RUNNING.text
                            },
                        )
                    }
                }
            }

            // ---------- 设置搜索框(P1-3 子项) ----------
            // 简洁实现:顶层一个搜索框 + 过滤后的命中行;不建跨页路由,命中行的
            // 点击只落在一级页本来就暴露的 subPage 上(与点对应导航行同一条路)。
            item(key = "search") {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .paSettingsCard()
                        .padding(horizontal = 16.dp, vertical = 4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(R.drawable.ic_lucide_search), null,
                        tint = PaEditorial.Subtle, modifier = Modifier.size(16.dp),
                    )
                    Spacer(Modifier.width(10.dp))
                    androidx.compose.foundation.text.BasicTextField(
                        value = settingsQuery,
                        onValueChange = { settingsQuery = it },
                        singleLine = true,
                        textStyle = androidx.compose.ui.text.TextStyle(
                            fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Ink,
                        ),
                        cursorBrush = androidx.compose.ui.graphics.SolidColor(PaEditorial.Primary),
                        modifier = Modifier.weight(1f).padding(vertical = 10.dp),
                        decorationBox = { inner ->
                            Box {
                                if (settingsQuery.isEmpty()) {
                                    Text("搜索设置…", fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Muted)
                                }
                                inner()
                            }
                        },
                    )
                    if (settingsQuery.isNotEmpty()) {
                        Icon(
                            painterResource(R.drawable.ic_lucide_x), "清除搜索",
                            tint = PaEditorial.Subtle,
                            modifier = Modifier
                                .size(16.dp)
                                .pressable(haptic = true) { settingsQuery = "" },
                        )
                    }
                }
            }

            if (settingsQuery.isBlank()) {
            // 当前主机卡已上移到骨架的 hostStatus 槽(PaHostChip,点进去 = 连接与配对)。
            //
            // 【S2 分组语义(按"使用频率 + 风险"重排)】一级页七个卡组:连接(最高频)/
            // Agent / 外观 / 会话与对话(S2 新增,自 Daemon 页聚拢)/ 数据与缓存 /
            // 安全 / 关于(另有运行时=控制中心区段,低频专业域,排在会话与对话之后、
            // 数据与缓存之前)。相关项聚成卡组:组间留白(GroupLabel top=16dp)>
            // 组内行距(12dp);组头小字说明本组职责;组内 1dp 细线分隔,组间零分割线
            // (留白分层)。每个导航行带"当前值摘要"—— 本地偏好类断连照常出值,
            // 依赖主机的项(审批/沙箱/电脑信息)断连如实标"需连接主机"。

            // ---------- 连接 ----------
            item(key = "connect") {
                Column {
                    GroupLabel(R.drawable.ic_lucide_globe, "连接", "通道、配对与输入桥接")
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .paSettingsCard()
                            .padding(horizontal = 16.dp),
                    ) {
                        SettingRow(
                            title = "连接与配对",
                            desc = "地址 · 配对码 · 主机指纹",
                            icon = R.drawable.ic_lucide_globe,
                            iconBox = true,
                            value = connSummary,
                            chevron = true,
                            divider = true,
                            onClick = { subPage = "userdata" },
                        )
                        SettingRow(
                            title = "Daemon 桥接与输入",
                            desc = "连接策略 · 语音 · 实验功能",
                            icon = R.drawable.ic_lucide_server,
                            iconBox = true,
                            value = daemonSummary,
                            chevron = true,
                            onClick = { subPage = "daemon" },
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
                            .paSettingsCard()
                            .padding(horizontal = 16.dp),
                    ) {
                        SettingRow(
                            title = "Agent 与模型",
                            desc = "切换 Agent · 权限 · 模型 · MCP",
                            icon = R.drawable.ic_lucide_cpu,
                            iconBox = true,
                            value = agentDisplay(runningAgent),
                            chevron = true,
                            onClick = { subPage = "agent" },
                        )
                    }
                }
            }

            // ---------- 外观 ----------
            item(key = "appearance") {
                Column {
                    GroupLabel(R.drawable.ic_lucide_sliders, "外观")
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .paSettingsCard()
                            .padding(horizontal = 16.dp),
                    ) {
                        SettingRow(
                            title = "外观与阅读",
                            desc = "显示模式 · 文字尺寸",
                            icon = R.drawable.ic_lucide_settings,
                            iconBox = true,
                            value = themeLabel + " · " + uiScaleLabel,
                            chevron = true,
                            onClick = { subPage = "appearance" },
                        )
                    }
                }
            }

            // ---------- 会话与对话(S2 新增)----------
            // 原 Daemon 页「全局对话显示」的三项散项(显示思考 / 工具详情 / 历史轮数)
            // 按使用频率聚拢为一级分组就地展示 —— 它们是对话观感的高频开关,不该藏在
            // 二级页;写入路径原样(思考走 daemon 七项写入通道,工具/历史是本地偏好),
            // Daemon 页不再保留副本(一个偏好只有一处控件,不出现第二个事实源)。
            // 组头作用域徽章:这组对所有已接入 agent 生效(S2 作用域徽章纪律)。
            item(key = "session") {
                Column {
                    GroupLabel(R.drawable.ic_lucide_brain, "会话与对话", "思考与工具的展示 · 历史加载")
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .paSettingsCard()
                            .padding(horizontal = 16.dp),
                    ) {
                        AgentScopeBadge(
                            SESSION_DISPLAY_SCOPES,
                            Modifier.padding(top = 10.dp),
                        )
                        Spacer(Modifier.height(8.dp))

                        // 显示思考过程(展示/隐藏):daemon 侧 hide_agent_reasoning 决定
                        // 是否推送思考事件;客户端 showThinking 以 daemon 为权威在写入
                        // 成功后同步(逻辑原样,自 Daemon 页平移)。
                        SplitButton(
                            label = "显示思考过程",
                            mainLabel = if (AgentConnection.showThinking) "展示" else "隐藏",
                            onMainClick = {
                                codexWriter.copy(hideReasoning = AgentConnection.showThinking).commit()
                                showSettingsFeedback(
                                    if (AgentConnection.showThinking) "将流式显示思考过程" else "将只显示对话结果",
                                )
                            },
                            options = listOf(
                                SplitOption("展示", selected = AgentConnection.showThinking) {
                                    codexWriter.copy(hideReasoning = false).commit()
                                    showSettingsFeedback("将流式显示思考过程")
                                },
                                SplitOption("隐藏", selected = !AgentConnection.showThinking) {
                                    codexWriter.copy(hideReasoning = true).commit()
                                    showSettingsFeedback("将只显示对话结果")
                                },
                            ),
                        )
                        Spacer(Modifier.height(8.dp))

                        // 工具详情(折叠/展开):纯本地偏好。
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

                        // 【F-D 历史按轮加载】打开会话只拉最近 N 轮,更早的靠时间线顶部
                        // 「加载更早」补齐(主=循环切换+下拉选档,自 Daemon 页平移)。
                        val roundsOptions = HistoryRoundsLogic.STEP_OPTIONS
                        val roundsIdx = roundsOptions.indexOf(AgentConnection.historyRounds).coerceAtLeast(0)
                        SplitButton(
                            label = "历史消息加载轮数",
                            caption = "打开会话时只加载最近 N 轮,更早的内容在会话顶部点「加载更早」继续查看",
                            mainLabel = HistoryRoundsLogic.labelFor(roundsOptions[roundsIdx]),
                            onMainClick = {
                                val next = roundsOptions[(roundsIdx + 1) % roundsOptions.size]
                                AgentConnection.historyRounds = next
                                showSettingsFeedback("历史加载已保存:${HistoryRoundsLogic.labelFor(next)}")
                            },
                            options = roundsOptions.map { r ->
                                SplitOption(
                                    HistoryRoundsLogic.labelFor(r),
                                    selected = r == AgentConnection.historyRounds,
                                ) {
                                    AgentConnection.historyRounds = r
                                    showSettingsFeedback("历史加载已保存:${HistoryRoundsLogic.labelFor(r)}")
                                }
                            },
                        )
                        Spacer(Modifier.height(6.dp))
                    }
                }
            }

            // ---------- 运行时(控制中心区段,低频专业域)----------
            // 只加这一行:区段本体在 ui/RuntimeSection.kt(排版 + 交互都在那边),
            // 免得继续往这个大文件里长内容。S2 排序:能力位(runtime.v1)点亮才出现,
            // 按使用频率排在会话与对话之后、数据与缓存之前。
            item(key = "runtime") { RuntimeSection() }

            // ---------- 数据与缓存 ----------
            item(key = "data") {
                Column {
                    GroupLabel(R.drawable.ic_lucide_folder, "数据与缓存", "都保存在这台手机上")
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .paSettingsCard()
                            .padding(horizontal = 16.dp),
                    ) {
                        SettingRow(
                            title = "下载与存储",
                            desc = "下载内容 · 缓存清理 · 存储占用",
                            icon = R.drawable.ic_lucide_folder,
                            iconBox = true,
                            value = downloadStorageSummary(doneDownloadsTop),
                            chevron = true,
                            onClick = { subPage = "userdata" },
                        )
                    }
                }
            }

            // ---------- 安全 ----------
            item(key = "safety") {
                Column {
                    GroupLabel(R.drawable.ic_lucide_shield, "安全", "执行权限与主机信任")
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .paSettingsCard()
                            .padding(horizontal = 16.dp),
                    ) {
                        SettingRow(
                            title = "审批与沙箱",
                            desc = "命令要不要批准 · 文件可写范围",
                            icon = R.drawable.ic_lucide_shield,
                            iconBox = true,
                            // 依赖主机:设置快照缺席(未连接/还没读到)时如实标"需连接主机"。
                            value = approvalSandboxSummary(
                                settings?.approvalPolicy,
                                settings?.sandbox,
                                settings != null,
                            ),
                            chevron = true,
                            divider = true,
                            onClick = { subPage = "agent" },
                        )
                        SettingRow(
                            title = "主机信任",
                            desc = "首连锁定电脑指纹,防止连到别的机器",
                            icon = R.drawable.ic_lucide_monitor,
                            iconBox = true,
                            value = hostTrustSummary(lockedPinTop),
                            chevron = true,
                            onClick = { subPage = "userdata" },
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
                            .paSettingsCard()
                            .padding(horizontal = 16.dp),
                    ) {
                        SettingRow(
                            title = "版本",
                            icon = R.drawable.ic_lucide_info,
                            iconBox = true,
                            value = "v${BuildConfig.VERSION_NAME}",
                            divider = true,
                        )
                        SettingRow(
                            title = "电脑",
                            icon = R.drawable.ic_lucide_monitor,
                            iconBox = true,
                            value = hostPlatformSummary(host?.os, host?.arch, connStateTop),
                        )
                    }
                }
            }
            } else {
                // 搜索模式:分组列表让位给过滤后的命中行。行尾标归属组,
                // 点击只落在一级页本来就暴露的 subPage 上(无 target 的仅展示归属)。
                // 【命中高亮】SettingRow 的 title 只收纯文本,高亮需要 AnnotatedString,
                // 所以命中行自绘(行型与 SettingRow 对齐:同字号/同分隔线,不算第二种卡)。
                val hits = filterSettingsEntries(settingsQuery, settingsSearchIndex())
                item(key = "search-results") {
                    Column {
                        GroupLabel(R.drawable.ic_lucide_search, "搜索结果")
                        Column(
                            Modifier
                                .fillMaxWidth()
                                .paSettingsCard()
                                .padding(horizontal = 16.dp),
                        ) {
                            if (hits.isEmpty()) {
                                Text(
                                    // 空态直说"没有匹配",不显示"未知设置"(禁词纪律)
                                    "没有匹配「${settingsQuery.trim()}」的设置",
                                    Modifier
                                        .fillMaxWidth()
                                        .padding(vertical = 14.dp),
                                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                                    color = PaEditorial.Muted,
                                )
                            } else {
                                hits.forEachIndexed { i, entry ->
                                    val range = matchRange(entry.title, settingsQuery)
                                    Column {
                                        Row(
                                            Modifier
                                                .fillMaxWidth()
                                                .then(
                                                    if (entry.target != null) {
                                                        Modifier.pressable(haptic = true) { subPage = entry.target!! }
                                                    } else Modifier
                                                )
                                                .padding(vertical = 11.dp),
                                            verticalAlignment = Alignment.CenterVertically,
                                        ) {
                                            Icon(
                                                painterResource(R.drawable.ic_lucide_sliders), null,
                                                tint = PaEditorial.Muted, modifier = Modifier.size(17.dp),
                                            )
                                            Spacer(Modifier.width(12.dp))
                                            Column(Modifier.weight(1f)) {
                                                Text(
                                                    androidx.compose.ui.text.buildAnnotatedString {
                                                        append(entry.title)
                                                        range?.let { r ->
                                                            addStyle(
                                                                androidx.compose.ui.text.SpanStyle(
                                                                    color = PaEditorial.Primary,
                                                                    fontWeight = FontWeight.SemiBold,
                                                                ),
                                                                r.first, r.last + 1,
                                                            )
                                                        }
                                                    },
                                                    fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink, maxLines = 1,
                                                )
                                                if (entry.desc.isNotEmpty()) {
                                                    Text(
                                                        entry.desc,
                                                        Modifier.padding(top = 2.dp),
                                                        fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Muted, maxLines = 2,
                                                    )
                                                }
                                            }
                                            Spacer(Modifier.width(8.dp))
                                            Text(entry.group, fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Muted, maxLines = 1)
                                            if (entry.target != null) {
                                                Spacer(Modifier.width(6.dp))
                                                Icon(
                                                    painterResource(R.drawable.ic_lucide_chevron_right), null,
                                                    tint = PaEditorial.Subtle, modifier = Modifier.size(15.dp),
                                                )
                                            }
                                        }
                                        if (i < hits.lastIndex) {
                                            Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))
                                        }
                                    }
                                }
                            }
                        }
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

/**
 * 卡片外的分区标题(编辑式稿 03:11sp/Muted/字距加宽,小图标裸放)。
 * [note] = 组头小字说明(M3 分组语义:组头顺带一句"这组管什么",只作导览,
 * 不承载状态);空 = 不占位,一级页导览组与二级页分组都兼容。
 */
@Composable
private fun GroupLabel(icon: Int, title: String, note: String = "") {
    Row(
        Modifier.padding(start = 2.dp, top = 16.dp, bottom = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(icon), null,
            tint = PaEditorial.Muted,
            modifier = Modifier.size(14.dp),
        )
        Spacer(Modifier.width(7.dp))
        Text(
            title,
            fontSize = PaEditorial.Type.Caption.fontSize,
            lineHeight = 16.sp,
            letterSpacing = 1.2.sp,
            fontWeight = FontWeight.SemiBold,
            color = PaEditorial.Muted,
        )
        if (note.isNotEmpty()) {
            Spacer(Modifier.width(8.dp))
            Text(
                note,
                fontSize = PaEditorial.Type.Micro.fontSize,
                color = PaEditorial.Subtle,
                maxLines = 1,
            )
        }
    }
}

/**
 * 二级页骨架(编辑式统一骨架):PaScreenScaffold(返回裸钮 + 25/32 标题 + 23dp 边距)。
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
    PaScreenScaffold(
        title = title,
        leading = {
            PaNavIconButton(
                onClick = onBack,
                contentDescription = "返回",
                iconRes = R.drawable.ic_lucide_chevron_left,
                plain = true,
            )
        },
    ) {
        if (scrollable) {
            Column(
                Modifier
                    .fillMaxWidth()
                    .weight(1f)
                    .verticalScroll(rememberScrollState())
                    .navigationBarsPadding()
                    .padding(top = 4.dp, bottom = 12.dp),
                content = content,
            )
        } else {
            Column(
                Modifier
                    .fillMaxWidth()
                    .weight(1f)
                    .navigationBarsPadding()
                    .padding(top = 4.dp, bottom = 12.dp),
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
            // 实时预览:改动立刻可见(唯一保留的说明性内容);卡内扁平行,无底色子容器
            Text(
                "预览:这是一段界面文字,当前字号 ×" +
                    String.format(java.util.Locale.US, "%.2f", AgentConnection.uiTextScale),
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 12.dp, vertical = 10.dp),
                fontSize = (13 * AgentConnection.uiTextScale).sp,
                color = PaEditorial.Muted,
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
                // 黑客终端:绿色磷光/等宽/纯黑,效果落在主对话(PaHacker 消费契约)
                Triple("hacker", "黑客终端", "绿色磷光 · 等宽 · 主对话终端风"),
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
                Text("已停用", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
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
private fun SettingStateLine(key: String, states: Map<String, String>) {
    val info = states[key]?.let { settingStateInfo(it) }
    if (info != null && info.level != "ok") {
        val color = when (info.level) {
            "error" -> EditorialTone.DENIED.text
            "warn" -> PaEditorial.Primary
            else -> PaEditorial.Muted
        }
        Row(
            Modifier.fillMaxWidth().padding(bottom = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(info.text, fontSize = PaEditorial.Type.Caption.fontSize, color = color)
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
        Text(line, fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
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
                ) { Text("正在读取…", fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Muted) }
            }

            error != null -> Column(Modifier.fillMaxWidth().padding(vertical = 24.dp)) {
                Text("暂时读不到能力列表", fontSize = PaEditorial.Type.Body.fontSize, color = EditorialTone.DENIED.text)
                Spacer(Modifier.height(6.dp))
                Text(error, fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
            }

            capabilities.isEmpty() -> Box(
                Modifier.fillMaxWidth().padding(vertical = 40.dp),
                contentAlignment = Alignment.Center,
            ) { Text("这台电脑上的 Codex 未报告任何技能、hooks 或插件", fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Muted) }

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
                    ) { Text("没有匹配「$query」的技能、hooks 或插件", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted) }
                } else {
                    LazyColumn(
                        Modifier.fillMaxWidth().weight(1f),
                        contentPadding = PaddingValues(bottom = 20.dp),
                    ) {
                        groups.forEach { (kind, list) ->
                            item(key = "header-$kind") {
                                Text(
                                    capabilityKindLabel(kind) + " · " + list.size,
                                    fontSize = PaEditorial.Type.Caption.fontSize,
                                    color = PaEditorial.Muted,
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
        // 【R1-C 结论 5】依赖主机的项断连要讲清:设置快照没读到(未连接/读取失败)时,
        // 切 Agent 是本地动作照常可用;审批/模型/MCP 等桌面侧项如实标注,不摆假可调。
        if (settings == null) {
            Row(
                Modifier
                    .fillMaxWidth()
                    .paSettingsCard()
                    .padding(horizontal = 16.dp, vertical = 10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(
                    painterResource(R.drawable.ic_lucide_triangle_alert), null,
                    tint = PaEditorial.Primary, modifier = Modifier.size(15.dp),
                )
                Spacer(Modifier.width(9.dp))
                Text(
                    "尚未读到电脑端设置:切换 Agent 照常可用,审批 / 模型 / MCP 等桌面侧项需连接主机后读取与修改",
                    Modifier.weight(1f),
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = PaEditorial.Muted,
                )
            }
            Spacer(Modifier.height(12.dp))
        }
        TextButton(onClick=onRetrySettings) { Text("刷新 Agent 状态和模型",color=PaEditorial.Primary) }
        SectionBlock(icon = R.drawable.ic_lucide_cpu, title = "切换 Agent") {
            Spacer(Modifier.height(10.dp))
            // T-DSH 问题 A:可用性一律读共享 capabilities 快照(AgentConnection
            // .hostCapabilities,主页网格同源)—— 本页 host 独立快照加载失败/未就绪
            // 时,这里不再退化成"只有 Codex 可选"的假灰显;探活循环持续兜底刷新。
            val capsShared = AgentConnection.hostCapabilities.collectAsState().value
            val agentChoices = buildAgents(capsShared).map { it.id to it.name }
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
                        val installed = buildAgents(capsShared).any { it.id == id && it.available }
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
                        .paSettingsCard()
                        .padding(start = 16.dp, end = 6.dp, top = 8.dp, bottom = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(R.drawable.ic_lucide_triangle_alert), null,
                        tint = PaEditorial.Primary, modifier = Modifier.size(15.dp),
                    )
                    Spacer(Modifier.width(9.dp))
                    Text(note, Modifier.weight(1f), fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
                    TextButton(onClick = onRetrySettings) {
                        Text("重试", color = PaEditorial.Primary, fontSize = PaEditorial.Type.Caption.fontSize)
                    }
                }
                Spacer(Modifier.height(12.dp))
            }

            SectionBlock(icon = R.drawable.ic_lucide_shield, title = "权限与安全") {
                Spacer(Modifier.height(10.dp))
                // 【S2 作用域徽章】审批/沙箱写的是 Codex 的设置快照 —— 作用于谁一眼可辨。
                AgentScopeBadge(listOf("codex"), Modifier.padding(bottom = 8.dp))
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
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = PaEditorial.Muted,
                    modifier = Modifier.padding(bottom = 4.dp),
                )
                SettingRow(
                    title = "应用到所有会话(含已有)",
                    desc = "开启后保存的每一项同时写入已有会话;关闭 = 仅新建会话生效",
                    divider = true,
                    control = {
                        // 状态词辅助(R1-C):开关位置不只靠色,旁边给"开/关"文字;
                        // 切换给轻触感反馈(结论 6:轻 = toggle)。
                        val hf = androidx.compose.ui.platform.LocalHapticFeedback.current
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                if (applyToExistingThreads) "开" else "关",
                                fontSize = PaEditorial.Type.Caption.fontSize,
                                color = PaEditorial.Muted,
                            )
                            Switch(
                                checked = applyToExistingThreads,
                                onCheckedChange = { on ->
                                    hf.performHapticFeedback(
                                        androidx.compose.ui.hapticfeedback.HapticFeedbackType.TextHandleMove,
                                    )
                                    onToggleApplyToAll(on)
                                },
                                colors = SwitchDefaults.colors(
                                    checkedTrackColor = PaEditorial.Primary,
                                    uncheckedTrackColor = PaEditorial.Raised,
                                    uncheckedBorderColor = Color.Transparent,
                                ),
                            )
                        }
                    },
                )

                // 【桌面只读回显】Windows 沙箱就绪度一行(windowsSandbox/readiness,
                // 只读诊断):读不到/老 daemon 不占位。电脑端配置读取失败时也在这里
                // 如实说一句 —— "读不到"不能被误读成"没有值"。
                DesktopEchoLogic.sandboxReadinessLine(settings)?.let { line ->
                    Text(
                        line,
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        color = PaEditorial.Muted,
                        modifier = Modifier.padding(bottom = 6.dp),
                    )
                }
                if (settings?.desktopEchoError?.isNotBlank() == true) {
                    Text(
                        "电脑端当前值暂时读不到:${settings.desktopEchoError}",
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        color = PaEditorial.Muted,
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
                SettingStateLine("approval_policy", settingStates)
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
                SettingStateLine("sandbox", settingStates)
                DesktopEchoLine(settings, "sandbox")
                Spacer(Modifier.height(10.dp))

                // 预览入口:卡内扁平行,按压缩放反馈,不做底色子容器
                Box(
                    Modifier
                        .fillMaxWidth()
                        .pressable(haptic = true) { onShowApprovalPreview() }
                        .padding(vertical = 12.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text("预览审批弹窗", fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Primary)
                }
                Spacer(Modifier.height(6.dp))
            }

            SectionBlock(icon = R.drawable.ic_lucide_cpu, title = "模型与思考") {
                Spacer(Modifier.height(10.dp))
                // 【S2 作用域徽章】默认模型/思考强度写的是当前 agent 的桌面默认值。
                AgentScopeBadge(listOf("codex"), Modifier.padding(bottom = 8.dp))
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
                SettingStateLine("model", settingStates)
                // 【桌面只读回显】provider 能力徽标(modelProvider/capabilities/read):
                // 解释"为什么某模型不可用"—— 图像生成/联网搜索是 provider 级声明,
                // 不是模型页的开关。没读到时不渲染,不把"没读到"说成"不支持"。
                // provider 能力徽标:纯文字一行(卡内去底色子容器),层级靠颜色分隔
                val capsBadges = DesktopEchoLogic.providerCapsBadges(settings)
                if (capsBadges.isNotEmpty()) {
                    Text(
                        capsBadges.joinToString(" · "),
                        fontSize = PaEditorial.Type.Micro.fontSize,
                        color = PaEditorial.Muted,
                        modifier = Modifier.padding(bottom = 6.dp),
                    )
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
                SettingStateLine("effort", settingStates)
                DesktopEchoLine(settings, "effort")
                Spacer(Modifier.height(6.dp))
            }

            SectionBlock(icon = R.drawable.ic_lucide_sparkles, title = "Codex 输出偏好") {
                Spacer(Modifier.height(10.dp))
                // 【边界说明(S2 作用域徽章化)】这些偏好只作用于"从 App 发起的任务"。
                // daemon 把它们写进自己拉起的 app-server 子进程内存态,
                // 不会改写用户桌面 codex 的 config.toml。
                // 整句"仅控制 Codex 生成方式"收敛为作用域徽章;跨 agent 的思考/工具
                // 显示已聚拢到一级页「会话与对话」,指路文案同步改。
                AgentScopeBadge(listOf("codex"), Modifier.padding(bottom = 6.dp))
                Text(
                    "仅控制 Codex 的生成方式；全部 Agent 的思考与工具显示在「会话与对话」调整。",
                    fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
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
                SettingStateLine("model_verbosity", settingStates)
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
                SettingStateLine("model_reasoning_summary", settingStates)
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
                        Text("已内置", fontSize = PaEditorial.Type.Caption.fontSize, color = EditorialTone.RUNNING.text)
                    },
                )
                Spacer(Modifier.height(6.dp))
            }

            SectionBlock(icon = R.drawable.ic_lucide_server, title = "用量与限流") {
                Spacer(Modifier.height(10.dp))
                // 【S2 作用域徽章】额度/限流是 Codex 账户(chatgpt.com 上游)的口径。
                AgentScopeBadge(listOf("codex"), Modifier.padding(bottom = 8.dp))
                CodexUsageBody(usage = codexUsage, loading = usageLoading)
                Spacer(Modifier.height(6.dp))
            }

            SectionBlock(icon = R.drawable.ic_lucide_sparkles, title = "能力 · 技能与插件") {
                // 项 3a:区块内只保留**唯一入口**,点击进入二级页「技能与插件」。
                // 以前每个 kind 各有一行"展开更多(N)",onClick 却全指向同一个弹窗,
                // 用户看到"两个入口点开的页面完全相同";现在统一为一个页面。
                Spacer(Modifier.height(10.dp))
                // 【S2 作用域徽章】技能/hooks/插件清单读的是电脑端 Codex 的能力。
                AgentScopeBadge(listOf("codex"), Modifier.padding(bottom = 8.dp))
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
                        fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
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
                        fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
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
                                    color = if (ok) EditorialTone.RUNNING.text else EditorialTone.DENIED.text,
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
            //  - Pi:模型清单 + 动态思考档位(ListModels 里 daemon 动态返回
            //    的 available_efforts)+ 全局默认模型/强度;
            //  - ZCode:模型清单 + 桌面默认强度 + 额度入口(本批占位)。
            when (runningAgent) {
                "pi", "zcode" -> AgentDefaultsSection(runningAgent, settings, modelChoices, writeCodex, onShowModelPicker)
                else -> if (host?.capabilitiesList?.contains("agent_settings:$runningAgent") == true) {
                    AgentDefaultsSection(runningAgent,settings,modelChoices,writeCodex,onShowModelPicker)
                } else SectionBlock(icon = R.drawable.ic_lucide_info, title = agentDisplay(runningAgent)) {
                    Text(
                        "${agentDisplay(runningAgent)} 的模型、权限与工具设置将在接入完成后在此显示；" +
                            "可先在上方「切换 Agent」里回到 Codex。",
                        fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
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
 *    available_efforts(按该模型自身的 reasoning 能力 + thinkingLevelMap
 *    动态过滤档位全集,不依赖活会话探针)—— 手机端原样展示,不造本地名单;
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
            fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
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
            fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
            modifier = Modifier.padding(bottom = 6.dp),
        )
    }
}

/**
 * 下载任务状态文案(四词口径,对齐 PaStatusBadge 的四词纪律:状态词越少越可信,
 * 同义近义词各说各话会稀释语义)。五态收四词:已取消/已中断同收「已停止」
 * (都是"没跑完且不是下载出错",用户的下一步是同一个——重新开始);失败与
 * 停止保持可辨(失败可给原因,停止只说停下)。
 *
 * 【上收为 internal 纯函数(2026-10-09 UI 收尾波)】JVM 钉测试
 * SettingsStatusLabelTest 逐项钉死五态→四词映射与闭合域,防回潮:枚举是闭合域,
 * when 不写 else —— 未来加新状态时编译期即红,强迫回到这里补词,不允许静默
 * 落进任何一个旧词。非法输入(持久化串解析不出枚举)在 TaskCenter.parseTasks
 * 层已回退(整行丢弃),本函数输入域恒为合法枚举。
 */
internal fun statusLabel(s: TaskCenter.Status): String = when (s) {
    TaskCenter.Status.RUNNING -> "下载中"
    TaskCenter.Status.DONE -> "已完成"
    TaskCenter.Status.FAILED -> "失败"
    TaskCenter.Status.CANCELED -> "已停止"
    TaskCenter.Status.INTERRUPTED -> "已停止"
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
 * 自绘输入框:顶部小标签 + 描边输入区。
 *
 * 【为什么不用 OutlinedTextField】Material 默认描边框的字体样式和内边距跟
 * 自绘语言不搭;这里只用 [BasicTextField],外观完全由我们控制。
 *
 * 【统一卡语言】卡内禁止带底色子容器 → 输入框无底色,只靠 1dp 描边划出可输入区;
 * 聚焦时描边亮成品牌蓝(颜色 spring 过渡)——"当前在哪一格"一眼可辨。
 */
@Composable
private fun SettingsTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    password: Boolean = false,
    keyboardType: KeyboardType = KeyboardType.Text,
) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    val borderColor by animateColorAsState(
        targetValue = if (focused) PaEditorial.Primary.copy(alpha = 0.55f) else PaEditorial.Hairline,
        animationSpec = PaMotion.standard(),
        label = "settingsFieldBorder",
    )
    Column(Modifier.fillMaxWidth()) {
        Text(
            label,
            Modifier.padding(start = 3.dp, bottom = 6.dp),
            fontSize = PaEditorial.Type.Micro.fontSize,
            color = PaEditorial.Muted,
        )
        Box(
            Modifier
                .fillMaxWidth()
                .border(1.dp, borderColor, RoundedCornerShape(PaEditorial.Radius.IconButton))
                .padding(horizontal = 13.dp, vertical = 12.dp),
            contentAlignment = Alignment.CenterStart,
        ) {
            if (value.isEmpty()) {
                Text("—", fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Muted)
            }
            BasicTextField(
                value = value,
                onValueChange = onValueChange,
                singleLine = true,
                textStyle = TextStyle(color = PaEditorial.Ink, fontSize = PaEditorial.Type.Body.fontSize),
                cursorBrush = SolidColor(PaEditorial.Primary),
                keyboardOptions = KeyboardOptions(keyboardType = keyboardType),
                visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
                modifier = Modifier.fillMaxWidth(),
                interactionSource = interaction,
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
                .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.Sheet))
                .padding(20.dp),
        ) {
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("选择默认模型", fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink)
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
                        fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
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
                if (selected) PaEditorial.Raised else Color.Transparent,
                RoundedCornerShape(PaEditorial.Radius.IconButton),
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
                fontSize = PaEditorial.Type.Heading.fontSize,
                color = if (selected) PaEditorial.Primary else PaEditorial.Ink,
            )
            if (subtitle.isNotEmpty()) {
                Text(subtitle, fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted)
            }
        }
        if (selected) {
            Spacer(Modifier.width(8.dp))
            Icon(
                painterResource(R.drawable.ic_lucide_check), null,
                tint = PaEditorial.Primary, modifier = Modifier.size(20.dp),
            )
        }
    }
}

/**
 * 审批弹窗预览:与聊天中真实弹出的审批卡同款样式 + 使用说明。
 * 真实审批:agent 要执行危险命令时弹出 → 允许=执行 / 拒绝=不执行但继续对话;
 * 不操作时任务结束前自动拒绝;断网时 daemon 自动拒绝并让 agent 继续作答。
 *
 * 【PaSheet 统一浮层】玻璃底 + 内建滚动;展开/收起详情用 animateContentSize +
 * PaMotion 弹簧——高度变化从当前值续走,连点"展开/收起"不跳帧。
 */
@Composable
fun ApprovalPreviewSheet(onDismiss: () -> Unit) {
    var expanded by remember { mutableStateOf(false) }
    PaSheet(onDismissRequest = onDismiss) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 24.dp)
                .paExpandable(rememberReduceMotion()),
        ) {
            Row(
                Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("允许执行?", Modifier.weight(1f), fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink)
                Text(
                    if (expanded) "收起" else "展开",
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = PaEditorial.Primary,
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
                fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Muted,
                modifier = Modifier.padding(top = 4.dp),
            )
            if (expanded) {
                Text(
                    "{\"command\": \"Remove-Item C:\\\\project\\\\old-build -Recurse\", \"cwd\": \"C:\\\\project\"}",
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = PaEditorial.CodeText,
                    fontFamily = PaEditorial.MonoFont,
                    modifier = Modifier
                        .fillMaxWidth()
                        // 命令原文:石墨代码箱(侧边栏/代码预览保留石墨深色的纪律)
                        .background(PaEditorial.GraphiteCode, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .padding(12.dp)
                        .padding(vertical = 4.dp),
                )
            }
            Text(
                "怎么用:点【允许】= 电脑上真实执行这条命令;点【拒绝】= 不执行," +
                    "agent 会继续对话并说明;放着不管 = 任务结束前自动拒绝。" +
                    "断网收不到弹窗时,daemon 会自动拒绝并让 agent 继续作答,不会卡死任务。",
                fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
                modifier = Modifier.padding(vertical = 10.dp),
            )
            Row(Modifier.fillMaxWidth().padding(bottom = 20.dp)) {
                // 拒绝:浅珊瑚底 + 加深珊瑚字;允许:石墨主操作(主色只给当前操作)
                Box(
                    Modifier
                        .weight(1f)
                        .pressable { onDismiss() }
                        .background(PaEditorial.DeniedContainer, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        // 【三盘同构 2026-09-30】描边原硬编码浅盘值 #E9CFCA,深/黑客盘
                        // 的 DeniedContainer 已反相为暗底,浅粉描边刺眼 —— 对齐
                        // PaApprovalDialog 拒绝卡的既有口径:Denied 30% 淡染。
                        .border(1.dp, PaEditorial.Denied.copy(alpha = 0.30f), RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .padding(vertical = 12.dp),
                    contentAlignment = Alignment.Center,
                ) { Text("拒绝(示例)", fontSize = PaEditorial.Type.Heading.fontSize, color = EditorialTone.DENIED.text) }
                Spacer(Modifier.width(12.dp))
                Box(
                    Modifier
                        .weight(1f)
                        .pressable { onDismiss() }
                        .background(PaEditorial.Graphite, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .padding(vertical = 12.dp),
                    contentAlignment = Alignment.Center,
                ) { Text("允许(示例)", fontSize = PaEditorial.Type.Heading.fontSize, color = Color.White, fontWeight = FontWeight.SemiBold) }
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
                .paSettingsCard()
                .padding(horizontal = 16.dp, vertical = 2.dp),
        ) {
            content()
        }
    }
}

/**
 * 设置行(编辑式稿 03 行语言):左标题(Ink)/副行(Muted),右控件,行间 1dp 细线。
 *
 * [iconBox] = true 时行首图标放进 42dp 抬升面方格(13dp 圆角,与 PaHostChip 同款)
 * —— 一级导航行用(截图 03 的行项目形态);二级页内默认 false,图标裸放保持轻。
 */
@Composable
fun SettingRow(
    title: String,
    desc: String = "",
    divider: Boolean = false,
    /** 行首小图标(参考主流设置页:每行一个线性图标,便于扫读)。 */
    icon: Int? = null,
    /** 图标着色。传 null 表示原样显示(用于 Agent 品牌 PNG,不能被染成灰色)。 */
    iconTint: Color? = PaEditorial.Muted,
    /** 图标尺寸;品牌图标比 17dp 的线性图标大一档才协调。 */
    iconSize: androidx.compose.ui.unit.Dp = 17.dp,
    /** 一级导航行的图标方格形态(42dp 抬升面)。 */
    iconBox: Boolean = false,
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
                // 行距对齐 4/8dp 网格(R1-C):11dp → 12dp,行高 ≥ 48dp 触达区不变。
                .padding(vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (icon != null) {
                if (iconBox) {
                    Box(
                        Modifier
                            .size(42.dp)
                            .background(PaEditorial.Raised, RoundedCornerShape(PaEditorial.Radius.IconButton)),
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(
                            painterResource(icon), null,
                            tint = iconTint ?: Color.Unspecified,
                            modifier = Modifier.size(20.dp),
                        )
                    }
                } else {
                    Icon(
                        painterResource(icon), null,
                        tint = iconTint ?: Color.Unspecified,
                        modifier = Modifier.size(iconSize),
                    )
                }
                Spacer(Modifier.width(12.dp))
            }
            Column(Modifier.weight(1f)) {
                Text(
                    title,
                    fontSize = PaEditorial.Type.Heading.fontSize, lineHeight = 21.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = PaEditorial.Ink,
                    maxLines = 1,
                )
                if (desc.isNotEmpty()) {
                    Text(
                        desc,
                        Modifier.padding(top = 2.dp),
                        fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 17.sp,
                        color = PaEditorial.Muted,
                        maxLines = 2,
                    )
                }
            }
            if (value != null) {
                Spacer(Modifier.width(8.dp))
                Text(
                    value,
                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                    color = PaEditorial.Muted,
                    maxLines = 1,
                )
            }
            control()
            if (chevron) {
                Spacer(Modifier.width(6.dp))
                Icon(
                    painterResource(R.drawable.ic_lucide_chevron_right), null,
                    tint = PaEditorial.Subtle,
                    modifier = Modifier.size(16.dp),
                )
            }
        }
        if (divider) {
            // 行间分隔统一 1dp 细线(编辑式:留白 + 细线分层)
            Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))
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
    /** 【T-32】配对失败的 daemon 原文(如实透传);null = 无配对错误。 */
    pairError: String?,
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
    // 【R1-C 结论 2】危险操作必须二次确认 + 说明后果:本页两个不可逆动作
    // (清除主机指纹=撤销信任;删除下载文件=清除数据)都先挂确认位,弹窗文案
    // 写清"动什么/不动什么/可不可逆",绝不一击即中。
    var confirmClearPin by remember { mutableStateOf(false) }
    var pendingDelete by remember { mutableStateOf<TaskCenter.Task?>(null) }

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
                                "ok" -> PaEditorial.Running
                                "down" -> PaEditorial.Denied
                                else -> PaEditorial.Muted
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
                        fontSize = PaEditorial.Type.Body.fontSize,
                        color = PaEditorial.Ink,
                    )
                    Text(
                        // 【不截断】原先 maxLines=1 + Ellipsis 会把长地址再裁一次,
                        // 用户根本抄不全。地址是"可操作信息",必须完整可读。
                        if (connState == "ok") connAddr else "USB → 局域网 → Tailscale",
                        fontSize = PaEditorial.Type.Micro.fontSize,
                        color = PaEditorial.Muted,
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
                    fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
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
                                .background(if (ep.active) PaEditorial.Running else PaEditorial.Muted, CircleShape),
                        )
                        Spacer(Modifier.width(9.dp))
                        Column(Modifier.weight(1f)) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Text(
                                    ep.label,
                                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                                    color = PaEditorial.Ink,
                                    fontWeight = FontWeight.SemiBold,
                                )
                                if (ep.active) {
                                    Spacer(Modifier.width(7.dp))
                                    // 状态用纯色文字表达(统一卡语言:去底色 pill)
                                    Text(
                                        "使用中",
                                        fontSize = PaEditorial.Type.Micro.fontSize,
                                        color = EditorialTone.RUNNING.text,
                                    )
                                }
                            }
                            // 完整 ip:port:不设 maxLines / 不用 Ellipsis,长地址也要看全。
                            Text(
                                ep.addr,
                                Modifier.padding(top = 2.dp),
                                fontSize = PaEditorial.Type.Caption.fontSize,
                                color = PaEditorial.Muted,
                            )
                            if (ep.detail.isNotBlank()) {
                                Text(
                                    ep.detail,
                                    Modifier.padding(top = 1.dp),
                                    fontSize = PaEditorial.Type.Micro.fontSize,
                                    color = PaEditorial.Muted,
                                )
                            }
                        }
                        Spacer(Modifier.width(8.dp))
                        Text("复制", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Primary)
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
                            fontSize = PaEditorial.Type.BodySmall.fontSize,
                            color = PaEditorial.Ink,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Text(
                            lockedPin.take(16) + "…(SHA-256,首次连接时锁定)",
                            Modifier.padding(top = 2.dp),
                            fontSize = PaEditorial.Type.Micro.fontSize,
                            color = PaEditorial.Muted,
                        )
                    }
                    Text(
                        "清除",
                        // 危险色已区分(Denied);动作本体交给确认弹窗,不一击即清。
                        Modifier.pressable(haptic = true) { confirmClearPin = true },
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        color = PaEditorial.Denied,
                    )
                }
            }

            // 【T-32】配对失败原文:必须紧挨输入框展示(connError 被重连循环覆盖,
            // 不能承载;settingsError 横幅只在一级页渲染,二级页看不见)。
            // 卡内扁平行:语义色文字足矣,不做底色子容器。
            if (pairError != null) {
                Text(
                    "配对失败：$pairError",
                    Modifier
                        .fillMaxWidth()
                        .padding(vertical = 6.dp),
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = PaEditorial.Denied,
                )
                Spacer(Modifier.height(6.dp))
            }
            if (errorMessage != null) {
                Text(
                    errorMessage + (if (errorAddress.isNotEmpty()) "\n地址:" + errorAddress else ""),
                    Modifier
                        .fillMaxWidth()
                        .padding(vertical = 6.dp),
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    // 【不省略】失败地址同样必须完整可读,方便用户排查。
                    color = PaEditorial.Denied,
                )
                Spacer(Modifier.height(6.dp))
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
                label = "配对码 / token(首次连接填电脑上的 8 位配对码)",
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
                // 总计:用户问"到底占了我多少空间"。查不到真实体积就如实标「无法获取」,不用 0 冒充。
                when {
                    downloadUnknown -> " · 占用无法获取"
                    downloadBytes > 0 -> " · 共 " + humanSize(downloadBytes)
                    else -> ""
                },
        ) {
            if (downloads.isEmpty()) {
                Text(
                    "还没有下载内容。在会话右上角「导出对话」或左上角菜单的任务中心里下载后,会出现在这里。",
                    Modifier.padding(vertical = 14.dp),
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = PaEditorial.Muted,
                )
            } else {
                downloads.forEach { d ->
                    val kind = fileKindOf(d.title)
                        Column {
                            Row(
                                Modifier.fillMaxWidth().padding(vertical = 11.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                // 行首文件类型图标裸放(不套 chip 底)
                                Icon(
                                    painterResource(kind.iconRes), null,
                                    tint = kind.color,
                                    modifier = Modifier.size(18.dp),
                                )
                                Spacer(Modifier.width(11.dp))
                                Column(Modifier.weight(1f)) {
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        Text(
                                            d.title.ifEmpty { "(未命名文件)" },
                                            Modifier.weight(1f, fill = false),
                                            fontSize = PaEditorial.Type.BodySmall.fontSize,
                                            color = PaEditorial.Ink,
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis,
                                        )
                                        Spacer(Modifier.width(6.dp))
                                        // 类型标签:纯色文字(统一卡语言:去底色 pill)
                                        Text(
                                            kind.label,
                                            fontSize = PaEditorial.Type.State.fontSize,
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
                                        fontSize = PaEditorial.Type.Micro.fontSize,
                                        color = PaEditorial.Muted,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                    if (d.detail.isNotEmpty()) {
                                        Text(
                                            d.detail,
                                            fontSize = PaEditorial.Type.State.fontSize,
                                            color = PaEditorial.Muted,
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
                                    // 文件本体真丢了(被系统/清理软件删掉)或定位不到:如实告知,
                                    // 绝不装作能打开;只给删除(移除死条目)。
                                    Text(
                                        if (d.localUri.isEmpty()) "文件位置无法获取" else "文件已被删除",
                                        Modifier.padding(end = 6.dp),
                                        fontSize = PaEditorial.Type.Micro.fontSize, color = EditorialTone.DENIED.text,
                                    )
                                }
                            }
                            IconActionButton(
                                R.drawable.ic_lucide_trash_2, "删除",
                                // 一击即删改成先确认(结论 2):文件本体不可恢复,
                                // 弹窗里说清"删手机上的什么、不动电脑上的什么"。
                                { pendingDelete = d },
                                bg = Color.Transparent, size = 32.dp, iconSize = 15.dp,
                                tint = PaEditorial.Denied,
                            )
                        }
                        // 行间分隔:0.5dp 细线(统一卡片语言)
                        Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))
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

    // ---- 清除主机指纹(撤销信任)的二次确认:写清撤销的是什么信任、下次发生什么 ----
    // 【D2 模态家族统一】裸 AlertDialog → PaSheet centered;危险确认钮用 Denied
    // 色槽(浅珊瑚底 + 珊瑚描边 + 加深珊瑚字)。语义零变化:遮罩/返回 = 取消。
    if (confirmClearPin) {
        PaSheet(onDismissRequest = { confirmClearPin = false }, centered = true) {
            Column(Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 18.dp)) {
                Text("清除主机指纹?", color = PaEditorial.Ink, fontWeight = FontWeight.Bold)
                Spacer(Modifier.height(10.dp))
                Text(
                    "手机会忘记这台电脑的指纹,下次连接时重新信任当前主机。" +
                        "在此之前若网络中被换成另一台机器,App 不会拦截。" +
                        "电脑上的数据与配对凭证不受影响。此操作立即生效。",
                    color = PaEditorial.Muted, fontSize = PaEditorial.Type.BodySmall.fontSize,
                )
                Spacer(Modifier.height(18.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    SettingsSheetDialogButton(
                        text = "取消",
                        tones = SettingsSheetButtonTones.secondary,
                        modifier = Modifier.weight(1f),
                    ) { confirmClearPin = false }
                    SettingsSheetDialogButton(
                        text = "清除",
                        tones = SettingsSheetButtonTones.danger,
                        modifier = Modifier.weight(1f),
                    ) {
                        confirmClearPin = false
                        AgentConnection.clearHostPin()
                        Toast.makeText(ctx, "已清除指纹,下次连接将重新信任主机", Toast.LENGTH_SHORT).show()
                    }
                }
            }
        }
    }

    // ---- 删除已下载文件的二次确认:删的是手机上的文件本体,电脑原件不动 ----
    // 【D2 模态家族统一】裸 AlertDialog → PaSheet centered;语义零变化:遮罩/返回
    // = 取消;确认钮按实际删除结果给提示(删不掉不谎报「已删除」)。
    pendingDelete?.let { target ->
        PaSheet(onDismissRequest = { pendingDelete = null }, centered = true) {
            Column(Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 18.dp)) {
                Text(
                    "删除「${target.title.ifEmpty { "未命名文件" }}」?",
                    color = PaEditorial.Ink,
                    fontWeight = FontWeight.Bold,
                )
                Spacer(Modifier.height(10.dp))
                Text(
                    "删除手机上已下载的这份文件本体" +
                        (if (target.sizeText.isNotBlank()) "(约 ${target.sizeText})" else "") +
                        ",电脑上的原件不受影响。此操作不可撤销。",
                    color = PaEditorial.Muted, fontSize = PaEditorial.Type.BodySmall.fontSize,
                )
                Spacer(Modifier.height(18.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    SettingsSheetDialogButton(
                        text = "取消",
                        tones = SettingsSheetButtonTones.secondary,
                        modifier = Modifier.weight(1f),
                    ) { pendingDelete = null }
                    SettingsSheetDialogButton(
                        text = "删除",
                        tones = SettingsSheetButtonTones.danger,
                        modifier = Modifier.weight(1f),
                    ) {
                        pendingDelete = null
                        // 条目无论如何都移除(留着只会变成打不开的死条目),
                        // 但提示按实际删除结果说 —— 删不掉不再谎报「已删除」。
                        val deleted = deleteLocalFile(ctx, target.localUri)
                        TaskCenter.remove(target.id)
                        Toast.makeText(
                            ctx,
                            if (deleted) "已删除" else "文件可能已被系统清理,条目已移除",
                            Toast.LENGTH_SHORT,
                        ).show()
                    }
                }
            }
        }
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
    // 缓存分区(OfflineStore 分区注册表,2026-09-30 起三项):占用 + 条目数。
    // url=网页快照(物理分区);checkpoint=轮次检查点、snapshot=会话快照(逻辑分区,
    // 物理目录是 TurnCheckpointStore/ThreadSnapshot 的既有磁盘契约)。
    var urlSnapBytes by remember { mutableLongStateOf(0L) }
    var urlSnapEntries by remember { mutableIntStateOf(0) }
    var checkpointBytes by remember { mutableLongStateOf(0L) }
    var checkpointEntries by remember { mutableIntStateOf(0) }
    var snapshotEntries by remember { mutableIntStateOf(0) }
    var toastMsg by remember { mutableStateOf<String?>(null) }
    var confirmClearTasks by remember { mutableStateOf(false) }
    var confirmClearSnap by remember { mutableStateOf(false) }

    /** 统计走 IO 线程:遍历目录不能在 UI 线程做。 */
    fun reload() = scope.launch(kotlinx.coroutines.Dispatchers.IO) {
        val pv = dev.pocketagent.app.ui.component.PreviewCache.usage()
        val tp = dev.pocketagent.app.ui.component.PaCache.tempUsage(ctx)
        val br = webDataUsage(ctx)
        // 缓存分区聚合:一次取齐三项分区的名称/占用/条目数/可清理性
        // (字节口径复用 OfflineStore 的 CacheEntry/bytesUsed 纯函数,与淘汰/清理一致,
        // 而不是一把梭量整个目录)。会话快照的占用与 ThreadSnapshot.usage() 同源同值。
        val stats = dev.pocketagent.app.ui.component.OfflineStore.allPartitionStats()
        val stUrl = stats.firstOrNull { it.id == "url" }
        val stCkpt = stats.firstOrNull { it.id == "checkpoint" }
        val stSnap = stats.firstOrNull { it.id == "snapshot" }
        // 真实目录大小 —— 这两个数字就是系统存储页看到的口径
        val cd = dev.pocketagent.app.ui.component.PaCache.cacheUsage(ctx)
        val fd = dev.pocketagent.app.ui.component.PaCache.filesUsage(ctx)
        withContext(kotlinx.coroutines.Dispatchers.Main) {
            previewBytes = pv
            snapshotBytes = stSnap?.bytes ?: 0L
            snapshotEntries = stSnap?.entries ?: 0
            tempBytes = tp
            browserBytes = br
            urlSnapBytes = stUrl?.bytes ?: 0L
            urlSnapEntries = stUrl?.entries ?: 0
            checkpointBytes = stCkpt?.bytes ?: 0L
            checkpointEntries = stCkpt?.entries ?: 0
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
            // 下载占用:查不到真实体积就如实标「无法获取」,绝不用 0 冒充
            StatCell2("下载", if (downloadUnknown) "无法获取" else sizeText(downloadBytes), Modifier.weight(1f))
        }

        // ---- 缓存项明细:每一项是什么、占多大、多少条(需求③ + 分区化聚合) ----
        // 数据来源:OfflineStore 分区注册表(allPartitionStats,三项分区口径统一);
        // 风险档由 [CacheCleanupLogic] 给出,高风险项(会话快照)打「清理需确认」标记,
        // 对应下方清理动作里的二次确认弹窗;凭据(token)从不入列。
        val managedRows = listOf(
            CacheCleanupLogic.Item("会话快照", CacheCleanupLogic.Kind.SNAPSHOT, snapshotBytes, snapshotEntries),
            CacheCleanupLogic.Item("轮次检查点", CacheCleanupLogic.Kind.CHECKPOINT, checkpointBytes, checkpointEntries),
            CacheCleanupLogic.Item("网页快照", CacheCleanupLogic.Kind.URL_SNAPSHOT, urlSnapBytes, urlSnapEntries),
            CacheCleanupLogic.Item("预览缓存", CacheCleanupLogic.Kind.PREVIEW, previewBytes),
            CacheCleanupLogic.Item("临时文件", CacheCleanupLogic.Kind.TEMP, tempBytes),
            CacheCleanupLogic.Item("浏览器数据", CacheCleanupLogic.Kind.BROWSER, browserBytes),
        )
        val managedTotal = managedRows.sumOf { it.bytes }
        Column(Modifier.fillMaxWidth().padding(top = 6.dp, bottom = 4.dp)) {
            managedRows.forEach { row ->
                Row(
                    Modifier.fillMaxWidth().padding(vertical = 6.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(row.key, fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Muted)
                    if (CacheCleanupLogic.riskOf(row.kind) == CacheCleanupLogic.Risk.RISKY) {
                        Text(" · 清理需确认", fontSize = PaEditorial.Type.Caption.fontSize, color = EditorialTone.DENIED.text)
                    }
                    Spacer(Modifier.weight(1f))
                    // 占用 + 条目数(条目数未统计时只显示占用,不摆 0)
                    val entryNote = if (row.entries > 0) " / ${row.entries} 项" else ""
                    Text(sizeText(row.bytes) + entryNote, fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Ink)
                }
            }
            // 小计:只汇总本 App 管理的可清理项,不含下载文件本体(那属于用户内容)
            Row(
                Modifier.fillMaxWidth().padding(vertical = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("本机可清理缓存合计", fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Ink)
                Spacer(Modifier.weight(1f))
                Text(sizeText(managedTotal), fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Ink)
            }
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

        // ---- 清理预览缓存(低风险:纯临时,清了随时可再生,可直接清) ----
        if (previewBytes > 0) {
            CacheActionRow(
                title = "清空预览缓存",
                desc = "临时预览,可随时清",
            ) {
                scope.launch(kotlinx.coroutines.Dispatchers.IO) {
                    val a = dev.pocketagent.app.ui.component.PreviewCache.clear()
                    withContext(kotlinx.coroutines.Dispatchers.Main) {
                        reload()
                        toastMsg = "已释放 ${sizeText(a)}"
                    }
                }
            }
        }

        // ---- 清理会话快照(高风险:清了要重新从电脑拉,离线看不到) → 必须二次确认 ----
        // 交给 [confirmClearSnap] 弹窗,文案写清后果;不会在没有快照时显示假入口。
        if (snapshotBytes > 0) {
            CacheActionRow(
                title = "清空会话快照",
                desc = "清后需联网重拉",
                danger = true,
            ) {
                confirmClearSnap = true
            }
        }

        // ---- 清理轮次检查点(低风险:断点续跑副本,清了只是未完成轮次不能再续跑) ----
        // 仅在有占用时出现;只删 filesDir/turn-checkpoints,与其它项口径互不重叠。
        if (checkpointBytes > 0) {
            CacheActionRow(
                title = "清空轮次检查点",
                desc = "断点续跑将失效",
            ) {
                scope.launch(kotlinx.coroutines.Dispatchers.IO) {
                    val x = dev.pocketagent.app.ui.component.OfflineStore.clear("checkpoint")
                    withContext(kotlinx.coroutines.Dispatchers.Main) {
                        reload()
                        toastMsg = "已释放 ${sizeText(x)}"
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
                fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted,
            )
            Text(
                "用户数据 ${sizeText(userBytes)} = 会话快照 ${sizeText(snapshotBytes)}" +
                    " + 轮次检查点 ${sizeText(checkpointBytes)} + 设置与登录凭证;" +
                    "清理项只动前两者,不碰登录凭证。",
                Modifier.padding(bottom = 4.dp),
                fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted,
            )
            Text(
                "下载 ${if (downloadUnknown) "占用无法获取" else sizeText(downloadBytes)} · $downloadCount 个文件" +
                    " · 任务条目 $taskCount 条;文件本体在系统下载目录,只能在上方列表逐条删除。",
                Modifier.padding(bottom = 4.dp),
                fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted,
            )
            Text(
                "网页快照 ${sizeText(urlSnapBytes)} = 断网时可回看的网页内容(pa_offline/pa_url 分区);" +
                    "与上面几项口径互不重叠,「清空网页快照缓存」只清它。",
                Modifier.padding(bottom = 4.dp),
                fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted,
            )
            Text(
                "轮次检查点 ${sizeText(checkpointBytes)} = 未完成轮次的断点续跑副本" +
                    "(filesDir/turn-checkpoints 分区,最多保留 ${dev.pocketagent.app.ui.component.TurnCheckpointStore.MAX_CHECKPOINTS} 个);" +
                    "「清空轮次检查点」只清它,已完成会话不受影响。",
                Modifier.padding(bottom = 4.dp),
                fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted,
            )
            Text(
                "清理临时与失效缓存:删崩溃残留的临时文件与「文件已被删除」的下载条目;" +
                    "不碰预览缓存、不碰登录状态,可随时清。",
                Modifier.padding(bottom = 4.dp),
                fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted,
            )
            Text(
                "清空预览缓存:释放 ${sizeText(previewBytes)};纯临时内容,清了随时可再生,不影响离线历史。",
                Modifier.padding(bottom = 4.dp),
                fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted,
            )
            Text(
                "清空会话快照(需二次确认):释放 ${sizeText(snapshotBytes)};" +
                    "后果:快照是「最近 N 轮」的本地副本,清掉后离线时看不到这段历史,需联网从电脑重拉;" +
                    "不影响登录与电脑端。",
                Modifier.padding(bottom = 4.dp),
                fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted,
            )
            Text(
                "清理浏览器数据:清掉内置浏览器的 HTTP 缓存 / Cookie / 网页存储;" +
                    "后果:浏览器里登录过的网站需重新登录;不影响电脑端与 App 设置。",
                Modifier.padding(bottom = 4.dp),
                fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted,
            )
            Text(
                "清空已完成的任务条目:只清任务中心记录列表,不删已下载文件,也不减上方「下载」占用。",
                fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted,
            )
        }
    }

    // 【D2 模态家族统一】裸 AlertDialog → PaSheet centered;危险确认钮用 Denied
    // 色槽。语义零变化:遮罩/返回 = 取消;清空只动任务中心记录,不碰已下载文件。
    if (confirmClearTasks) {
        PaSheet(onDismissRequest = { confirmClearTasks = false }, centered = true) {
            Column(Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 18.dp)) {
                Text("清空任务条目?", color = PaEditorial.Ink, fontWeight = FontWeight.Bold)
                Spacer(Modifier.height(10.dp))
                Text(
                    "只移除任务中心的记录,已下载到手机的文件不受影响。此操作不可撤销。",
                    color = PaEditorial.Muted, fontSize = PaEditorial.Type.BodySmall.fontSize,
                )
                Spacer(Modifier.height(18.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    SettingsSheetDialogButton(
                        text = "取消",
                        tones = SettingsSheetButtonTones.secondary,
                        modifier = Modifier.weight(1f),
                    ) { confirmClearTasks = false }
                    SettingsSheetDialogButton(
                        text = "清空",
                        tones = SettingsSheetButtonTones.danger,
                        modifier = Modifier.weight(1f),
                    ) {
                        TaskCenter.clearFinished()
                        confirmClearTasks = false
                        reload()
                        toastMsg = "已清空已完成的条目"
                    }
                }
            }
        }
    }

    // ---- 会话快照二次确认(需求④:有风险项必须让用户看清后果再清) ----
    // 【D2 模态家族统一】裸 AlertDialog → PaSheet centered;语义零变化:遮罩/返回
    // = 取消;清空在 IO 线程执行,主线程收尾(关弹窗/刷新/toast)。
    if (confirmClearSnap) {
        PaSheet(onDismissRequest = { confirmClearSnap = false }, centered = true) {
            Column(Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 18.dp)) {
                Text("清空会话快照?", color = PaEditorial.Ink, fontWeight = FontWeight.Bold)
                Spacer(Modifier.height(10.dp))
                Text(
                    "会话快照是「最近 N 轮」的本地副本,断网时用来回看历史。清空后需要重新从电脑拉取," +
                        "离线时将看不到这段历史;联网后会自动重建,不影响电脑端记录与登录。此操作不可撤销。",
                    color = PaEditorial.Muted, fontSize = PaEditorial.Type.BodySmall.fontSize,
                )
                Spacer(Modifier.height(18.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    SettingsSheetDialogButton(
                        text = "取消",
                        tones = SettingsSheetButtonTones.secondary,
                        modifier = Modifier.weight(1f),
                    ) { confirmClearSnap = false }
                    SettingsSheetDialogButton(
                        text = "清空",
                        tones = SettingsSheetButtonTones.danger,
                        modifier = Modifier.weight(1f),
                    ) {
                        scope.launch(kotlinx.coroutines.Dispatchers.IO) {
                            val freed = dev.pocketagent.app.ui.component.ThreadSnapshot.clearAll()
                            withContext(kotlinx.coroutines.Dispatchers.Main) {
                                confirmClearSnap = false
                                reload()
                                toastMsg = "已释放 ${sizeText(freed)}"
                            }
                        }
                    }
                }
            }
        }
    }
}

/** 缓存分区的一格数字。 */
@Composable
private fun StatCell2(label: String, value: String, modifier: Modifier = Modifier) {
    Column(modifier, horizontalAlignment = androidx.compose.ui.Alignment.CenterHorizontally) {
        Text(value, fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Ink)
        Text(label, Modifier.padding(top = 2.dp), fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
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
            // 行距对齐 4/8dp 网格(R1-C):10dp → 12dp。
            .padding(vertical = 12.dp),
        verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(title, fontSize = PaEditorial.Type.Body.fontSize, fontWeight = FontWeight.SemiBold, color = if (danger) EditorialTone.DENIED.text else PaEditorial.Ink)
            Text(desc, Modifier.padding(top = 2.dp), fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
        }
        Icon(
            painterResource(R.drawable.ic_lucide_chevron_right), null,
            tint = PaEditorial.Subtle, modifier = Modifier.size(15.dp),
        )
    }
}

/**
 * 默认折叠的「详情」展开区:把长解释收进来,默认不刷屏,点一下才展开。
 * 用简单 state 切换,不引第三方依赖。
 *
 * 【展开收起 spring 化】[paExpandable]:高度从当前值续走(可打断、1:1 跟手),
 * 减弱动效开启时瞬时到位。
 */
@Composable
private fun CollapsibleDetail(content: @Composable () -> Unit) {
    var expanded by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxWidth().paExpandable(rememberReduceMotion()).padding(top = 4.dp)) {
        Row(
            Modifier
                .fillMaxWidth()
                .pressable(haptic = false, onClick = { expanded = !expanded })
                .padding(vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text("详情", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
            Spacer(Modifier.weight(1f))
            Text(if (expanded) "▾" else "▸", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
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
    val dir = PaPaths.webviewDir(ctx)
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
    // 「原生设置」分组(2026-09-25 原生设置透出轮):agent 原生报告的目录型配置项,
    // 由 agent_native_settings:<id> 能力位点亮(旧 daemon / 未提供的家自动隐藏,
    // 不摆假开关)。分组自带空态隐藏与错误呈现,Agent 默认模型/强度行保持不变。
    // 【2026-09-30 合并】原生设置(Model/Reasoning effort 等)与下方"默认模型/
    // 思考强度"偏好组对同一 agent 是同一件事的两个通道;原生组存在时它就是权威,
    // 偏好组隐藏(避免同页两处改同一配置);无原生设置的 agent 才回落偏好组。
    var nativeHasContent by androidx.compose.runtime.remember(agentId) { androidx.compose.runtime.mutableStateOf(false) }
    NativeConfigOptionsSection(agentId) { nativeHasContent = it }
    if (!nativeHasContent) SectionBlock(icon = R.drawable.ic_lucide_cpu, title = agentDisplay(agentId)) {
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
        Text("${(levels.size-1).coerceAtLeast(0)} 个已报告档位 · 下次发送时应用", fontSize=12.sp, color=PaEditorial.Muted,
            modifier=Modifier.padding(vertical=10.dp))
    }
}

/**
 * 「原生设置」分组:agent 原生报告的目录型配置项(ListAgentConfigOptions 透传)。
 *
 * 【数据纪律(红线 2 的手机端面)】id/名称/当前值/候选全部原生原文 —— 候选列表只
 * 渲染 daemon 透传的叶子,绝不本地补选项;当前值匹配不到候选时显示原文(不标
 * "未知/错误",事实呈现)。writable=false 的行是只读事实(pi:原生写通道会持久
 * 落桌面全局配置),只显示当前生效值 + 说明,不提供写。
 * 【能力位门控】仅当 daemon 声明 agent_native_settings:<id> 时请求;目录为空且
 * 无错误时整组隐藏(该 agent 此刻没有可示的原生设置,如实,不占位)。
 */
@Composable
internal fun NativeConfigOptionsSection(agentId: String, onHasContent: (Boolean) -> Unit = {}) {
    val caps = AgentConnection.hostCapabilities.collectAsState().value
    if (!caps.contains("agent_native_settings:$agentId")) return
    val scope = rememberCoroutineScope()
    var items by remember(agentId) { mutableStateOf<List<pocketagent.v1.AgentConfigOption>>(emptyList()) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var refresh by remember { mutableStateOf(0) }
    var loaded by remember(agentId) { mutableStateOf(false) }
    LaunchedEffect(agentId, refresh) {
        busy = true
        try {
            val resp = withContext(Dispatchers.IO) {
                AgentConnection.modelServiceSlow().listAgentConfigOptions(
                    ListAgentConfigOptionsRequest.newBuilder().setAgentId(agentId).build())
            }
            items = resp.itemsList
            error = resp.error.ifBlank { null }
        } catch (e: Exception) {
            // 整条读取失败(旧 daemon/断连):不渲染占位错误 —— 分组安静隐藏,
            // 重进设置页自然重试;不把"读不到"渲染成"没有原生设置"。
        } finally {
            busy = false
            loaded = true
        }
    }
    if (!loaded) return
    // 回报"本 agent 有原生设置可示":AgentDefaultsSection 据此隐藏与之重复的
    // daemon 偏好组(默认模型/思考强度) —— 原生组是权威协议面,偏好组只作回退。
    androidx.compose.runtime.LaunchedEffect(items, error) {
        onHasContent(items.isNotEmpty() || error != null)
    }
    if (items.isEmpty() && error == null) return
    // 所有可写行的生效范围说明(dsh:仅会话内生效)在组尾统一呈现一次,不逐行重复。
    val sharedNote = items.firstOrNull { it.writable && it.note.isNotBlank() }?.note.orEmpty()
    SectionBlock(icon = R.drawable.ic_lucide_sliders, title = "原生设置") {
        Spacer(Modifier.height(10.dp))
        if (error != null) {
            Text("暂时读不到原生设置：$error", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
                modifier = Modifier.padding(vertical = 6.dp))
        }
        items.forEachIndexed { index, row ->
            val options = row.optionsList
            if (row.writable && row.type == "select" && options.isNotEmpty()) {
                // 当前值的显示名:匹配到候选叶子用原生名,匹配不到显示原文(事实呈现)。
                val currentLabel = options.find { it.value == row.currentValue }?.name
                    ?: row.currentValue.ifBlank { "未报告" }
                SplitButton(
                    label = row.name.ifBlank { row.id },
                    mainLabel = if (busy) "同步中…" else currentLabel,
                    onMainClick = { refresh++ },
                    options = options.map { leaf ->
                        SplitOption(
                            title = leaf.name.ifBlank { leaf.value },
                            desc = leaf.description,
                            selected = leaf.value == row.currentValue,
                        ) { selectNativeOption(scope, agentId, row.id, leaf.value) { refresh++ } }
                    },
                )
                Spacer(Modifier.height(8.dp))
            } else {
                // 只读行 / 无候选的可写行:当前值原文 + 说明,不提供"切换"控件
                // (没有可核对的原生候选清单就不放行 —— 与 daemon 侧写校验同一纪律)。
                SettingRow(
                    title = row.name.ifBlank { row.id },
                    desc = row.note,
                    value = row.currentValue.ifBlank { "未报告" },
                    divider = index != items.lastIndex,
                )
            }
        }
        if (sharedNote.isNotBlank()) {
            Text(sharedNote, fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
                modifier = Modifier.padding(vertical = 6.dp))
        }
    }
}

/** select 行的切换:写通道走 SetCodexSettings 的原生配置项分支,成功后刷新目录。 */
private fun selectNativeOption(
    scope: kotlinx.coroutines.CoroutineScope,
    agentId: String,
    optionId: String,
    value: String,
    onDone: () -> Unit,
) {
    scope.launch {
        try {
            withContext(Dispatchers.IO) {
                AgentConnection.modelServiceSlow().setCodexSettings(
                    SetCodexSettingsRequest.newBuilder()
                        .setAgentId(agentId)
                        .setNativeConfigOptionId(optionId)
                        .setNativeConfigValue(value)
                        .build())
            }
        } catch (e: Exception) {
            // 失败不做乐观更新:选中态由下一次刷新如实回读,绝不假装切成功。
        }
        onDone()
    }
}

// =====================================================================================
// 设置页接线纯函数(版本闸提示文案 / 语音模型下载按钮判定;JVM 单测钉死,
// 同 SettingsOverviewLogic 纪律:取值与文案收在可测纯函数里,Composable 只喂状态)
// =====================================================================================

/**
 * 版本闸提示行文案(纯函数):App 版本低于 daemon 要求的最低版本时,设置页
 * 连接信息卡的内联警示就这么说。minAppVersion 空白(对端未报告,理论上到不了
 * 这里 —— outdated 为 true 蕴含 min 非空)回落成不带版本号的说法,不摆空 ≥。
 */
internal fun appVersionOutdatedHint(minAppVersion: String): String =
    if (minAppVersion.isBlank()) "App 版本过旧，请更新手机 App"
    else "App 版本过旧，电脑端要求 ≥$minAppVersion，请更新手机 App"

/**
 * 语音卡"下载语音模型"按钮的可见性与文案(纯函数,返回 null = 不显示按钮)。
 *
 * 三态口径(与 SenseVoiceModelManager 的状态流一一对齐,UI 不自持状态):
 *  · ready(就绪)或 preparing(下载中,进度走既有"初始化 X%"通道)→ null,
 *    按钮消失/不出现;
 *  · 其余(!ready && !preparing)必摆按钮 —— 未下载态摆"下载";
 *    下载失败态摆"重试"。两者只有按钮文案之差,失败原因本身经既有 desc 通道
 *    (speech.error)透出,这里不重复拼错误串。
 *
 * 【失败判别主通道 = downloadFailed】Manager 已加显式失败标志
 * (SenseVoiceModelState.downloadFailed,prepareBundledModel 失败置位、
 * ready/重校验通过归零):"重试"判定首选它,不再依赖文案比对。
 * 【NEEDS_DOWNLOAD_MESSAGE 文案比对保留为兼容回退】旧会话里 Manager 状态可能
 * 还是没有该位的旧落点(失败与未下载同落 needsDownload=true,唯一判别通道是
 * error 文案);error 非占位文案仍判"重试",两处引用同一常量,改一处即编译可见。
 */
internal fun senseVoiceDownloadAction(
    ready: Boolean,
    preparing: Boolean,
    downloadFailed: Boolean,
    error: String?,
): String? = when {
    ready || preparing -> null
    downloadFailed -> "重试下载语音模型"
    // 兼容回退:无 downloadFailed 位的旧状态,按文案识别失败(非占位 = 真实原因)。
    error != null && error != SenseVoiceModelManager.NEEDS_DOWNLOAD_MESSAGE -> "重试下载语音模型"
    else -> "下载语音模型（约 228MB，建议 Wi-Fi）"
}

/** writer 形参已随 S2 聚拢移除(F3 终验):对话显示三项只留在一级页「会话与对话」。 */
@Composable
private fun DaemonPage(onBack: () -> Unit, onFeedback: (String) -> Unit, models: List<ModelInfo>, capabilities: List<String>) {
    SubPageScaffold(title = "Daemon 桥接与输入", onBack = onBack) {
        SectionBlock(icon = R.drawable.ic_lucide_server, title = "连接策略") {
            Text("USB → 局域网直连 → Tailscale 保底。已连接时保持当前通道，断流后自动恢复任务结果。",
                fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Muted, modifier = Modifier.padding(vertical=12.dp))
        }
        // 【S2 聚拢】原「全局对话显示」分区(显示思考 / 工具详情 / 历史轮数)已整体
        // 上收为一级页「会话与对话」组(高频观感开关不该藏在二级页);写入路径原样,
        // 这里不再保留副本 —— 一个偏好只有一处控件,不会有第二个事实源。

        // 实验功能:提示词优化(默认关;依赖电脑端 ZCode 且消耗其 Token Plan 配额)
        SectionBlock(icon = R.drawable.ic_lucide_sparkles, title = "实验功能") {
            Spacer(Modifier.height(10.dp))
            SplitButton(
                label = "优化提示词按钮",
                // 模式说明(P4 Task 3「可解释」):七种改写模式与预览弹层风格芯片
                // 共用同一口径(见 promptOptimizeStyleHint / daemon promptOptimizeStyleTemplates),
                // 并写明两条红线 —— 只改写不执行、结果经确认才替换输入框。
                caption = "在会话输入栏显示「优化」按钮,把口语化输入改写成给 AI 的清晰指令;由下方选定的 Agent 与模型生成，会消耗对应配额。" +
                    "改写模式七种:默认(说清楚、不扩写)/精简(一句话)/详细(要点列表)/实施计划(目标·范围·验收)/排错(现象·环境·复现·期望)/" +
                    "测试(输入·行为·验证)/代码审查(风险·影响·证据)。优化只改写、不会自动执行,结果经你确认才会替换输入框",
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
            // 【R1-C 结论 5】优化按钮本体是本地开关,但"用哪个 Agent/模型"的候选来自
            // 电脑端模型表 —— 断连时如实标注,不摆空下拉让人误以为没有可选。
            if (models.isEmpty()) {
                Text(
                    "优化使用的 Agent 与模型列表需连接主机后读取",
                    fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
                    modifier = Modifier.padding(vertical = 6.dp),
                )
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

            // 【辅线 B】未识别操作确认(实验,全局默认关):读 daemon RuntimePolicy 的
            // 全局布尔,写走既有 SetRuntimePolicy(完整策略 + CAS,与「运行时」区段的
            // budget 块共用 [RuntimeControlState] 同一份真值 —— 两个界面一个数)。
            // 开启后 agent 发来的未识别反向请求先弹手机确认卡,由用户逐次决定;
            // 关闭(默认)时 daemon 直接安全拒绝,行为与历史一致。
            LaunchedEffect(Unit) { RuntimeControlState.ensureBudgetSnapshot() }
            val passthroughScope = rememberCoroutineScope()
            val passthroughOn = unknownRequestPassthroughOn(RuntimeControlState.budget?.policy)
            fun setPassthrough(on: Boolean, done: String) {
                passthroughScope.launch {
                    val err = RuntimeControlState.saveBudget { b -> applyUnknownRequestPassthrough(b, on) }
                    onFeedback(err ?: done)
                }
            }
            SplitButton(
                label = "未识别操作确认(实验)",
                caption = "开启后,agent 发来的未识别操作会先征求你的同意,而不是直接拒绝",
                mainLabel = if (passthroughOn) "开" else "关",
                onMainClick = { setPassthrough(!passthroughOn, if (!passthroughOn) "已开启:未识别操作将先征求你的同意" else "已关闭:未识别操作将直接安全拒绝") },
                options = listOf(
                    SplitOption("开", selected = passthroughOn) { setPassthrough(true, "已开启:未识别操作将先征求你的同意") },
                    SplitOption("关", selected = !passthroughOn) { setPassthrough(false, "已关闭:未识别操作将直接安全拒绝") },
                ),
            )
            Spacer(Modifier.height(6.dp))
        }

        // 【端侧语音】离线识别模型管理:系统识别不可用的设备(无识别服务 ROM/无 GMS)
        // 靠它落地语音输入 —— 下载 / 进度 / 失败重试 / 删除释放空间。
        SectionBlock(icon = R.drawable.ic_lucide_mic, title = "语音识别") {
            Spacer(Modifier.height(10.dp))
            val context = LocalContext.current
            LaunchedEffect(Unit) { SenseVoiceModelManager.init(context) }
            val speech by SenseVoiceModelManager.state.collectAsState()
            // 【按需下载正路入口】模型不再随 APK 打包(SenseVoiceModelManager 三态,
            // 上轮已就位):needsDownload → 摆下载按钮(此前只有"点麦克风才触发"
            // 一条暗路);下载中沿用下方既有"初始化 X%"进度通道;就绪后按钮消失;
            // 下载失败 → desc 走既有错误通道 + 重试按钮(判别主通道 =
            // downloadFailed 显式标志,文案比对仅作旧状态兼容回退,见纯函数 KDoc)。
            val downloadAction = senseVoiceDownloadAction(speech.ready, speech.preparing, speech.downloadFailed, speech.error)
            SettingRow(
                title = "SenseVoice Small INT8（按需下载）",
                desc = speech.error
                    ?: "符合中文输入习惯 · 自动标点与数字规范化 · 音频不离开设备",
                value = when {
                    speech.ready -> "已就绪"
                    speech.preparing -> "初始化 ${speech.preparedBytes * 100 / SenseVoiceModelManager.TOTAL_BYTES}%"
                    speech.needsDownload -> "未下载"
                    else -> "内置"
                },
                divider = false,
            )
            if (downloadAction != null) {
                Spacer(Modifier.height(6.dp))
                Row(
                    Modifier
                        .fillMaxWidth()
                        .heightIn(min = PaEditorial.Space.TouchMin)
                        .background(PaEditorial.Raised, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .pressable(haptic = true) { SenseVoiceModelManager.prepareBundledModel() }
                        .padding(horizontal = 16.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.Center,
                ) {
                    Text(
                        downloadAction,
                        fontSize = PaEditorial.Type.Body.fontSize,
                        fontWeight = FontWeight.SemiBold,
                        color = PaEditorial.Primary,
                    )
                }
            }
            Text(
                "有系统识别服务时可直接使用；否则使用 SenseVoice 离线识别(模型首次启用时联网下载一次,音频不离开设备)。录音权限被拒绝时只请求权限。",
                fontSize = PaEditorial.Type.Caption.fontSize,
                color = PaEditorial.Muted,
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
        Text(error ?: native?.nativePermissionsNote.orEmpty(),fontSize=11.sp,color=PaEditorial.Muted,modifier=Modifier.padding(vertical=8.dp))
        if(threadId.isBlank()) Text("默认值用于新建会话；已有会话在对话的更多菜单中调整。",fontSize=11.sp,color=PaEditorial.Muted)
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

// ---- PaSheet centered 弹窗的编辑式按钮(D2 模态家族统一;规格照抄
// PaApprovalDialog 的 ApprovalButton:≥50dp 触达、IconButton 圆角、色调取槽位)----

/** 弹窗按钮的编辑式色调(容器/描边/内容三色,一律取 PaEditorial 槽位)。 */
private data class SettingsSheetButtonTones(val container: Color, val outline: Color, val content: Color) {
    companion object {
        /** 次操作(取消):浅底描边。 */
        val secondary get() = SettingsSheetButtonTones(PaEditorial.Raised, PaEditorial.Hairline, PaEditorial.Ink)

        /** 危险确认:浅珊瑚底 + 珊瑚描边 + 加深珊瑚字(PaApprovalDialog .danger 同款)。 */
        val danger get() = SettingsSheetButtonTones(
            PaEditorial.DeniedContainer,
            PaEditorial.Denied.copy(alpha = 0.35f),
            PaEditorial.DeniedText,
        )
    }
}

@Composable
private fun SettingsSheetDialogButton(
    text: String,
    tones: SettingsSheetButtonTones,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(PaEditorial.Radius.IconButton)
    Row(
        modifier
            .heightIn(min = 50.dp) // 最小触达纪律
            .background(if (enabled) tones.container else tones.container.copy(alpha = 0.55f), shape)
            .border(1.dp, if (enabled) tones.outline else tones.outline.copy(alpha = 0.55f), shape)
            .pressable(enabled = enabled, onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 13.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.Center,
    ) {
        Text(
            text,
            fontSize = PaEditorial.Type.Body.fontSize,
            fontWeight = FontWeight.SemiBold,
            color = (if (enabled) tones.content else tones.content.copy(alpha = 0.55f)),
            maxLines = 1,
        )
    }
}
