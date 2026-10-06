package dev.pocketagent.app.ui

// SlashCommandsLogic.kt —— 斜杠命令候选面板的纯逻辑(无 Android 依赖,可 JVM 单测)。
//
// 【链路】会话是面板 agent(zcode/codex/pi 三源)且输入框以 "/" 开头 → 输入胶囊
// 上方弹**纵向可滑动**命令卡面(名称 + 一句话说明,完整不截断);普通命令点选
// 把已敲的 "/前缀" 回填成 "/命令 "(保留后续文字),内置伪命令点选直接**执行
// 动作**(见 SlashAction,不填输入框)。候选数据来自 daemon 的
// ListSlashCommands(zcode=会话快照 slashCommands[];codex=skills 映射;pi=
// get_commands 真名映射,探针证据 daemon/docs/verify/evidence/pi-cmds-probe.md),
// 按会话进程内缓存(见 SlashCommandCache)。RPC 调用与 Compose 状态写在
// ChatScreen(与 PromptOptimizeLogic 同款分工:纯逻辑可测,副作用留 UI)。

import java.util.LinkedHashMap

/** 手机端 RPC 的 deadline(秒)。daemon 侧整段预算 20s(冷启 + session/resume),再兜余量。 */
internal const val SLASH_COMMANDS_DEADLINE_S = 30L

/** 候选卡面的最大高度(dp,纵向可滑):超出的靠滚动看,不再截断条数。 */
internal const val SLASH_PANEL_MAX_HEIGHT_DP = 320

/** 命令点选后的动作分流:普通命令回填输入框,内置伪命令执行对应动作。 */
enum class SlashAction {
    /** 普通命令:回填 "/命令 "(codex 技能填自然语言),发送后由 agent 触发。 */
    FILL,
    /** 内置伪命令"/目标":打开设置会话目标(SetThreadGoal)弹窗。 */
    GOAL,
    /**
     * 内置伪命令"/压缩":触发 CompactThread RPC 压缩上下文。
     * 可见性挂 `compact:<agentId>` 能力位(daemon 的 CompactThread 按原生形状
     * 三家实现、dsh 没有):位不在的会话列表里就不出现,不摆点了必败的假入口。
     */
    COMPACT,
    /**
     * 内置伪命令"/计划":触发 StartReview RPC 发起计划审查,审查结果以事件
     * 形式流回本会话(daemon 路由见 thread_review.go)。
     * 可见性挂 `review:<agentId>` 能力位 —— 现状仅 codex 宣告(review/start 是
     * codex app-server 的对等能力),将来别家补上实现并宣告位时自动点亮;
     * 缺位不出现,与"点了才被 daemon 拒"相比宁可不在列表里。
     */
    REVIEW,
}

/** daemon ListSlashCommands 里一条命令的手机端展平视图(摆脱 proto 类型,纯逻辑可用)。 */
data class SlashCommandInfo(
    val name: String,               // 命令名(不带斜杠,如 "goal";pi 技能形如 "skill:mmx-cli")
    val description: String,        // 一句话说明(可空)
    val inputHint: String,          // 参数提示(如 "/goal [pause|resume|…]",可空)
    val action: SlashAction = SlashAction.FILL, // 内置伪命令带动作;daemon 命令恒为 FILL
)

/** 会话归属 agent 是否为 zcode(空/未知一律不算,大小写不敏感兜一手)。 */
internal fun isZcodeAgent(agentId: String?): Boolean =
    agentId.equals("zcode", ignoreCase = true)

/** 会话归属 agent 是否为 codex(同上,空/未知不算)。 */
internal fun isCodexAgent(agentId: String?): Boolean =
    agentId.equals("codex", ignoreCase = true)

/** 会话归属 agent 是否为 pi(同上,空/未知不算)。 */
internal fun isPiAgent(agentId: String?): Boolean =
    agentId.equals("pi", ignoreCase = true)

/**
 * 会话归属 agent 是否有斜杠命令面板(三源,只增):
 *   - zcode:app-server 会话快照的 slashCommands[](原有数据源);
 *   - codex:协议没有命令清单 RPC,**skills 即命令**(daemon 侧查证与映射见
 *     internal/agent/codex_commands.go,同一 ListSlashCommands RPC 按 agent_id 自动路由);
 *   - pi:RPC get_commands 真名映射(daemon 侧见 internal/agent/pi.go
 *     SlashCommandsFor;探针实证 extension/prompt/skill 三源都是 /<名> 可直接触发的真命令)。
 * 其余(未知/旧库空串)仍不算。
 */
internal fun isSlashPanelAgent(agentId: String?): Boolean =
    isZcodeAgent(agentId) || isCodexAgent(agentId) || isPiAgent(agentId)

/**
 * 按 agent 绑定的能力位判定(纯函数,门控统一入口):
 * daemon 在 GetHostInfo.capabilities 里逐 agent 宣告 `<位>:<agentId>`(steer/
 * queue_clear/compact/fork/rollback/review/export_html 族,见
 * daemon/internal/server/host.go),位在 = 真。
 *   - 大小写/空白不敏感(与 queueClearVisible、RuntimeStateLogic.runtimeControlCapable 同口径);
 *   - agentId 为空/空白(GetThread 还在路上)= 绑定不到具体会话,一律 false ——
 *     硬红线"无能力不显示假入口",绝不放空;
 *   - 只认绑定同一 agent 的位:别家的位不开我家的门(能力位按 agent 逐个宣告)。
 */
internal fun agentCapableOf(bit: String, agentId: String?, capabilities: List<String>): Boolean {
    val b = bit.trim()
    val id = agentId?.trim().orEmpty()
    if (b.isEmpty() || id.isEmpty()) return false
    return capabilities.any { it.trim().equals("$b:$id", ignoreCase = true) }
}

/**
 * 内置伪命令层(与 daemon 数据源无关的本地命令):
 * 用户点选后**执行动作而非填充文本**(对应 RPC/弹窗由 ChatScreen 接线)。
 *   - /目标:设置会话目标(常驻上下文)→ SetThreadGoal 弹窗;三源通用(daemon
 *     的 SetThreadGoal 按 agent 落库 + 下发 codex/zcode,pi 落库跟随);
 *   - /压缩:压缩会话上下文省 token → CompactThread RPC;可见性随
 *     `compact:<agentId>` 能力位(daemon 已按 AgentID 路由 codex/pi/zcode 三种
 *     原生形状,dsh 没有 —— 缺位不出现);
 *   - /计划:发起计划审查 → StartReview RPC;可见性随 `review:<agentId>` 能力位
 *     (现状仅 codex 宣告:review/start 是 codex app-server 的对等能力)。
 * 【排序意图】内置伪命令是用户点名的"计划/目标模式"入口,置顶展示。
 * 【防御】与 daemon 命令重名时让位(daemon 的同名命令是真实可触发的,优先级更高)。
 */
internal fun builtinSlashCommands(): List<SlashCommandInfo> = listOf(
    SlashCommandInfo("目标", "设置会话目标(常驻上下文,agent 每轮都会看到)", "", SlashAction.GOAL),
    SlashCommandInfo("压缩", "压缩会话历史上下文,省 token", "", SlashAction.COMPACT),
    SlashCommandInfo("计划", "发起计划审查(审查结果以事件流回本会话)", "", SlashAction.REVIEW),
)

/**
 * 内置伪命令按会话归属 agent 与**能力位**过滤后的全集:
 *   - /目标三源通用(pi 没有 goal 概念,落库跟随也不展示 —— daemon 按路由事实
 *     过滤,该事实暂无对应能力位,保留按 agent 判这一处);
 *   - /压缩 认 `compact:<agentId>`、/计划 认 `review:<agentId>`:位不在
 *     (旧 daemon 没发过位 / dsh 没有实现)就绝不进列表 —— 能力位是"看到什么"
 *     与"能不能执行"一致的唯一数据源,面板候选合并(slashCandidatesFor)一律走这里。
 */
internal fun builtinSlashCommandsFor(agentId: String?, capabilities: List<String>): List<SlashCommandInfo> {
    val base = if (isPiAgent(agentId)) builtinSlashCommands().filter { it.action != SlashAction.GOAL }
    else builtinSlashCommands()
    return base.filter { cmd ->
        when (cmd.action) {
            SlashAction.COMPACT -> agentCapableOf("compact", agentId, capabilities)
            SlashAction.REVIEW -> agentCapableOf("review", agentId, capabilities)
            else -> true
        }
    }
}

/**
 * 输入框当前敲出的"命令词元":第一个空白(空格/换行/Tab)之前的前缀。
 * 不以 "/" 开头(或空输入)返回 null —— 面板的开关信号。裸 "/" 也算词元
 * (刚敲出斜杠就要看到全量列表)。
 */
internal fun slashTokenOf(input: String): String? {
    val token = input.takeWhile { !it.isWhitespace() }
    return if (token.startsWith("/")) token else null
}

/**
 * 候选过滤(纯函数,输入是"已合并"的候选全集):
 * 命令名对 "/后" 的词元做大小写不敏感前缀匹配。
 *   - 词元后已敲空白("/goal ")→ 空(命令已选定在敲参数,面板收起);
 *   - 词元为裸 "/" → 全量(纵向卡面可滑动,不再截断条数 —— codex 31 个技能、
 *     pi 56 条命令都要能看到,旧横滚条的 8 条上限是"技能列表不全"观感的根源);
 *   - 无前缀命中 → 退回全量(用户记不清命令名时面板保持可用,同款宽进哲学);
 *   - 候选全集为空 → 空(面板不显示)。
 */
internal fun filterSlashCandidates(commands: List<SlashCommandInfo>, input: String): List<SlashCommandInfo> {
    if (commands.isEmpty()) return emptyList()
    val token = slashTokenOf(input) ?: return emptyList()
    if (input.length > token.length) return emptyList()
    val query = token.removePrefix("/").lowercase()
    if (query.isEmpty()) return commands
    val hit = commands.filter { it.name.lowercase().startsWith(query) }
    return if (hit.isEmpty()) commands else hit
}

/**
 * 面板候选全集:内置伪命令(随能力位点亮,见 builtinSlashCommandsFor)置顶 +
 * daemon 命令(重名让位),一次合并去重。
 * 这是可见性与 UI 共用的唯一候选来源(保证"看到什么"与"能不能开面板"一致)。
 */
internal fun slashCandidatesFor(
    agentId: String?,
    commands: List<SlashCommandInfo>,
    capabilities: List<String>,
): List<SlashCommandInfo> {
    if (!isSlashPanelAgent(agentId)) return emptyList()
    val names = commands.mapTo(HashSet()) { it.name.lowercase() }
    val builtins = builtinSlashCommandsFor(agentId, capabilities)
    val merged = ArrayList<SlashCommandInfo>(commands.size + builtins.size)
    for (b in builtins) {
        val nativeAlias = when (b.action) { SlashAction.GOAL -> "goal"; SlashAction.COMPACT -> "compact"; else -> b.name.lowercase() }
        if (b.name.lowercase() !in names && nativeAlias !in names) merged.add(b)
    }
    merged.addAll(commands)
    return merged
}

/**
 * 面板可见性(单一入口,UI 与单测共用):会话是面板 agent(zcode/codex/pi,见
 * isSlashPanelAgent)且不处于离线快照态,且输入以 "/" 开头敲出了词元,且过滤
 * 结果非空。内置伪命令随能力位点亮(zcode/codex 至少有 /目标兜底;pi 需要
 * compact:<id> 位给 /压缩 —— hostCapabilities 连机即拉,先于进会话,正常在线
 * 时不缺位),所以在线的三源会话哪怕 daemon 命令还没到位也有卡面可弹。
 */
internal fun slashPanelVisible(
    agentId: String?,
    input: String,
    commands: List<SlashCommandInfo>,
    offline: Boolean,
    capabilities: List<String>,
): Boolean {
    if (!isSlashPanelAgent(agentId) || offline) return false
    return filterSlashCandidates(slashCandidatesFor(agentId, commands, capabilities), input).isNotEmpty()
}

/** 未知助手的空态文案(AUDIT §3.3:遇未知值给人话,禁止静默、禁止"未知/错误"字样)。 */
internal const val SLASH_UNKNOWN_AGENT_HINT = "该助手暂无可用命令"

/**
 * 未知助手的空态提示是否显示(纯函数,JVM 单测钉死):
 * 会话归属 agent 已知非空但不在三源面板名单里(zcode/codex/pi 之外,如旧库残值
 * 或将来的第 4 个 agent),用户又敲出了 "/" 词元 → 输入栏上方给人话空态,而不是
 * 面板静默不弹(用户敲了 "/" 却毫无反馈,会以为坏了)。
 *   - agentId 为空/空白 = 会话信息还没拉到(getThread 在路上):不提示,免得
 *     加载瞬间闪一条错误印象的文案;
 *   - 离线快照态:不提示(与面板同口径,离线横幅已另行说明);
 *   - 三源会话永远走正常面板(内置伪命令兜底,见 slashPanelVisible),不进本分支。
 */
internal fun slashUnknownAgentHintVisible(agentId: String?, input: String, offline: Boolean): Boolean {
    if (offline) return false
    if (slashTokenOf(input) == null) return false
    val id = agentId?.trim() ?: return false
    if (id.isEmpty()) return false
    return !isSlashPanelAgent(id)
}

/**
 * 点选回填:把输入开头的 "/词元" 替换成 "/命令 "(注意补一个空格,方便接着
 * 敲参数),词元之后的文字原样保留("/go 修复登录" 选 goal → "/goal 修复登录")。
 * 词元后本来就是空白/换行时不再重复补空格("/in⏎" 选 init → "/init⏎")。
 * 输入没有 "/" 词元时原样返回(调用方只在面板可见时点选,这是防御)。
 */
internal fun applySlashSelection(input: String, commandName: String): String {
    val token = slashTokenOf(input) ?: return input
    val rest = input.drop(token.length)
    val sep = if (rest.isNotEmpty() && rest[0].isWhitespace()) "" else " "
    return "/$commandName$sep$rest"
}

/**
 * 点选回填的三源入口(按会话归属 agent 与命令动作分流,只增):
 *   - 内置伪命令(action != FILL):no-op —— 点选语义是"执行动作",由 ChatScreen
 *     在调用本函数**之前**拦截(见 SlashAction);走到这里是防御,原样返回;
 *   - codex 技能:codex 没有 "/xxx" 语法,技能靠模型读 description 触发 ——
 *     填**自然语言**「请使用 <名称> 技能:」,发送后 codex 自行触发,会话内
 *     真实可用。daemon 侧对 codex 技能的 inputHint 恒为空,这里据此识别;
 *     万一将来 codex 也带 inputHint,退回模板填充,与 zcode 同款;
 *   - zcode / pi 命令:走 [applySlashSelection]("/词元" 回填成 "/命令 ")。
 *     pi 的真命令(get_commands 三源)在 prompt 以 /<名> 开头时由 pi 原生展开
 *     (extension 命令直接执行、skill:/模板注入内容),回填真名即可触发,
 *     不需要自然语言转译。
 * 其余 agent(面板本就不显示)原样防御返回。
 */
internal fun applySlashPick(input: String, command: SlashCommandInfo, agentId: String?): String {
    if (command.action != SlashAction.FILL) return input
    if (isCodexAgent(agentId) && command.inputHint.isBlank()) {
        return "请使用 ${command.name} 技能："
    }
    return applySlashSelection(input, command.name)
}

/**
 * 内置伪命令点选后的输入清理:敲出的 "/词元" 是唤起面板的临时手段,动作执行后
 * 不能残留在输入框里(否则发送会把 "/目标" 当正文发给 agent)。只摘词元、保留
 * 词元后已敲的文字("/目 修复登录" → "修复登录"),结果掐头去尾空白。
 */
internal fun stripSlashToken(input: String): String {
    val token = slashTokenOf(input) ?: return input
    return input.drop(token.length).trim()
}

/**
 * "/计划"发起失败的人话文案(纯函数,JVM 单测钉死):
 *   - gRPC 业务错误(FAILED_PRECONDITION / INVALID_ARGUMENT 等):daemon 的
 *     description 就是写给人看的中文(如"计划模式目前仅 Codex 会话支持"),
 *     直接透传,不把 "FAILED_PRECONDITION: ..." 的英文码砸用户脸上;
 *   - UNIMPLEMENTED:老 daemon 没有 StartReview 这个 RPC —— 提示升级电脑端;
 *   - UNAVAILABLE / DEADLINE_EXCEEDED:连接/超时,给可执行的建议;
 *   - 其余(非 gRPC 异常、空消息):兜底文案,永远有反馈。
 * 与 ChatScreen 的 htmlExportFailureMessage 同款取舍,独立成函数是因为二者的
 * 支持面与文案各说各的事,不共用前缀判断。
 */
internal fun slashReviewFailureMessage(error: Throwable): String {
    val st = (error as? io.grpc.StatusRuntimeException)?.status
        ?: return error.message?.takeIf { it.isNotBlank() } ?: SLASH_REVIEW_FALLBACK
    return when (st.code) {
        io.grpc.Status.Code.UNIMPLEMENTED ->
            "电脑端 daemon 版本过旧,暂不支持计划审查;请先升级电脑端"
        io.grpc.Status.Code.UNAVAILABLE ->
            "连不上电脑,请确认电脑端在线后重试"
        io.grpc.Status.Code.DEADLINE_EXCEEDED ->
            "电脑端响应超时,请稍后重试"
        else -> st.description?.takeIf { it.isNotBlank() }
            ?: error.message?.takeIf { it.isNotBlank() }
            ?: SLASH_REVIEW_FALLBACK
    }
}

/** 兜底文案:认不出错误形状时也让人知道这条反馈对应哪个动作。 */
internal const val SLASH_REVIEW_FALLBACK = "发起计划审查失败"

/**
 * 命令列表的进程内缓存(按会话):进会话、重连、下拉刷新时才真正发 RPC,
 * 每敲一个字符零开销。daemon 侧另有 10 分钟 TTL 缓存兜子进程频率,两侧各管各的。
 *
 * 【淘汰口径(审查 open 项 2,2026-10-04 收口)】LRU 上限 [MAX_CACHED_THREADS]
 * 个会话:
 *   - 访问序 = [get] 命中即 touch(accessOrder=true 的 LinkedHashMap 语义),
 *     面板开着连续敲字符的活跃会话不会被误淘;
 *   - 超限在 [put] 时淘汰最久未访问的会话(LinkedHashMap removeEldestEntry,
 *     均摊 O(1));重度用户(反复进出上百会话)内存有界,不再无界增长;
 *   - [clear](会话删除,接线点 ThreadDeletionCoordinator.purgeInMemory)不受
 *     上限影响,用户明确的删除照旧立即清。
 * 【为何不用 ConcurrentHashMap】访问序 LRU 的 get 本身是结构性变更(touch 要
 * 挪序),ConcurrentHashMap 表达不了;缓存条目 ≤128、读写都是低频面(进出会话
 * /下拉刷新),synchronized 足够。
 */
internal object SlashCommandCache {
    /** 缓存会话数上限:128 个会话 × 每会话几十条命令文本,内存量级可忽略。 */
    internal const val MAX_CACHED_THREADS = 128

    private val byThread = object : LinkedHashMap<String, List<SlashCommandInfo>>(16, 0.75f, true) {
        override fun removeEldestEntry(eldest: MutableMap.MutableEntry<String, List<SlashCommandInfo>>): Boolean =
            size > MAX_CACHED_THREADS
    }

    /** 命中返回缓存列表(touch 访问序);未命中返回 null(空列表不入缓存,恒未命中)。 */
    fun get(threadId: String): List<SlashCommandInfo>? = synchronized(byThread) {
        byThread[threadId]?.takeIf { it.isNotEmpty() }
    }

    /** 空列表不入缓存:新 zcode 会话在电脑上落地后,下次进会话就能拉到命令。 */
    fun put(threadId: String, commands: List<SlashCommandInfo>) {
        if (commands.isNotEmpty()) synchronized(byThread) { byThread[threadId] = commands }
    }

    /** 会话删除/显式失效时清理(ThreadDeletionCoordinator.purgeInMemory 已接线)。 */
    fun clear(threadId: String) {
        synchronized(byThread) { byThread.remove(threadId) }
    }
}
