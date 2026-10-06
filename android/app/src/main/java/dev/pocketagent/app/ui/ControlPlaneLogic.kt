package dev.pocketagent.app.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import dev.pocketagent.app.AgentConnection
import io.grpc.Status
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CollabPolicy
import pocketagent.v1.ControlItem
import pocketagent.v1.ControlPlane
import pocketagent.v1.GetControlPlaneRequest
import pocketagent.v1.RuntimePolicy

/**
 * 控制中心(GetControlPlane)的数据层:**一次聚合读看全 daemon 的可控域,写动作
 * 全部再路由到既有 RPC**。
 *
 * 【为什么单独成层】与 [RuntimeControlState] 同一理由:这层最容易犯的错是"替 daemon
 * 说话"—— 重排/改写 summary、把 toggle 的非"开/关"现值猜成布尔、把 MB 输入当字节存、
 * 把老 daemon 的 UNIMPLEMENTED 渲染成故障。映射抽成纯函数,这些钉子在 `test` 阶段
 * 就被钉死(ControlPlaneLogicTest),不必依赖真机与 daemon。
 *
 * 【写通道刻意不新增】ControlItem 只是呈现;toggle 走既有 SetCollabPolicy、
 * budget 数值走既有 SetRuntimePolicy(完整策略 + CAS,纪律在 [RuntimeControlState]
 * 的 save* 里),collab 数值走既有 SetCollabPolicy,agents 桌面偏好走既有
 * SetCodexSettings。给总览另配一套写路径 = 给同一份状态造第二份真值。
 * 接通的通道之外一律只读呈现 —— 没接通的写通道绝不摆入口。
 */

// ---- 域 id(daemon 契约钉死的稳定键,客户端按 id 匹配、不按 title) --------------

const val CONTROL_DOMAIN_BUDGET = "budget"
const val CONTROL_DOMAIN_COLLAB = "collab"
const val CONTROL_DOMAIN_AGENTS = "agents"
const val CONTROL_DOMAIN_MEMORY = "memory"
const val CONTROL_DOMAIN_DIALYSIS = "dialysis"

/** collab 域的扇出成员上限项(daemon 钉死的稳定 id;数值写通道走既有 SetCollabPolicy)。 */
const val COLLAB_MEMBERS_ID = "collab.max_parallel_members"

/**
 * collab 域的委派/扇出子开关项 id(daemon 契约钉死的稳定键,摘要组合 [collabSummaryOverride]
 * 按 id 取项、不按 label;与 [COLLAB_TOGGLE_IDS] 词表同源)。
 */
internal const val COLLAB_DELEGATE_ID = "collab.native_delegate_enabled"
internal const val COLLAB_FANOUT_ID = "collab.fanout_enabled"

// ---- 展示模型 ------------------------------------------------------------------

/** 状态圆点三档(0 灰 / 1 黄 / 2 红;语义色只用在 8dp 小圆点上)。 */
enum class ControlDotKind { NORMAL, ATTENTION, LIMITED }

/**
 * 一项的写动作。已接通的写通道:collab 的 toggle、budget/collab 的数值、
 * agents 的桌面偏好 select(候选由 daemon 给);其余一律 READ_ONLY ——
 * 没接通的写通道不摆入口。
 */
enum class ControlItemAction { COLLAB_TOGGLE, COLLAB_NUMBER, BUDGET_NUMBER, AGENTS_SELECT, READ_ONLY }

/** 域内一项的展示模型(字段全部照 daemon 原文带出,不翻译不改写)。 */
data class ControlItemRow(
    /** 所属域 id:写动作路由要用的参数,由 [controlDomainRows] 填,不让调用方再猜。 */
    val domainId: String = "",
    val id: String,
    val label: String,
    val current: String,
    /** 照 proto 原文:toggle / select / number / info;出现新值照抄,渲染为只读。 */
    val kind: String,
    val hint: String,
    /** select 型候选(proto ControlItem.options 照抄);非 select 项为空。 */
    val options: List<String> = emptyList(),
    val action: ControlItemAction,
)

/** 一个功能域的展示模型。 */
data class ControlDomainRow(
    val id: String,
    val title: String,
    val summary: String,
    val writable: Boolean,
    /** 照 proto 原样(0/1/2);文案怎么显示由 [ControlDotKind] 与 summary 原文决定。 */
    val statusCode: Int,
    val dotKind: ControlDotKind,
    val items: List<ControlItemRow>,
)

// ---- 映射纯函数 ----------------------------------------------------------------

/**
 * 状态码 → 圆点档位。proto 只定义 0/1/2;出现别的值按最保守的 LIMITED 呈现
 * (宁多一分提醒,不把读不懂的值画成"一切正常")。
 */
fun controlDotKind(statusCode: Int): ControlDotKind = when (statusCode) {
    0 -> ControlDotKind.NORMAL
    1 -> ControlDotKind.ATTENTION
    else -> ControlDotKind.LIMITED
}

/**
 * budget 域 item id 里的策略键:"budget.global.ram_working_bytes" →
 * "ram_working_bytes"。**只在七个已知键内放行** —— daemon 词表之外的键我们没有
 * 对应的写字段,放行就是静默丢改动。null = 不在路由表内(只读)。
 */
fun controlBudgetKey(itemId: String): String? {
    if (!itemId.startsWith("budget.global.")) return null
    val key = itemId.removePrefix("budget.global.")
    return if (key in BUDGET_KEYS) key else null
}

private val BUDGET_KEYS = setOf(
    "ram_working_bytes",
    "ram_event_buffer_bytes",
    "ram_session_index_bytes",
    "disk_conversation_bytes",
    "disk_cache_bytes",
    "disk_artifact_bytes",
    "max_parallel_turns",
)

private val COLLAB_TOGGLE_IDS = setOf(
    "collab.collaboration_enabled",
    "collab.native_delegate_enabled",
    "collab.fanout_enabled",
)

/**
 * agents 域 item id 里的设置键词表 = CodexSettings 的**字符串**字段(proto model.proto
 * 的 key 词表)。bool 键(hide_agent_reasoning / disable_paste_burst)刻意不在表内:
 * select 的候选是字符串,把"开/关"猜成 true/false 就是造一份假账 —— 它们的写呈现
 * 契约(开关形态)没落地前保持只读。
 */
private val AGENTS_SETTING_KEYS = setOf(
    "approval_policy",
    "sandbox",
    "model",
    "effort",
    "model_verbosity",
    "model_reasoning_summary",
)

/** agents 域一条桌面偏好的写目标:SetCodexSettingsRequest 的 agent_id + 设置键。 */
data class ControlAgentsTarget(val agentId: String, val key: String)

/**
 * agents 域 item id → 写目标:"agents.codex.approval_policy" → (codex, approval_policy)。
 * 设置键**从最右一段拆**(键词表钉死,agentId 本身含点号也不怕);null = 不在路由表内
 * (只读)—— 路由表外放行就是静默丢改动。
 */
fun controlAgentsTarget(itemId: String): ControlAgentsTarget? {
    if (!itemId.startsWith("agents.")) return null
    val rest = itemId.removePrefix("agents.")
    val key = rest.substringAfterLast('.', "")
    if (key !in AGENTS_SETTING_KEYS) return null
    val agentId = rest.substringBeforeLast('.', "")
    if (agentId.isBlank()) return null
    return ControlAgentsTarget(agentId, key)
}

/**
 * agents 域桌面偏好的**值域校验**:写入值必须在 daemon 给的候选(options)里。
 * 返回 null = 放行;非 null = 拒绝原因(UI 原样显示)。枚举写错档的脏值在客户端
 * 先拦下 —— daemon 还没见过的取值,放过去就是让它替我们兜底。
 */
fun controlAgentsOptionInvalid(value: String, options: List<String>): String? = when {
    options.isEmpty() -> "此项目前没有可用的候选值"
    value !in options -> "所选值不在此项提供的候选里"
    else -> null
}

/**
 * agents 域一条桌面偏好 → SetCodexSettingsRequest。**presence 纪律**(proto 钉死):
 * 恰好 set 被改的那一个字段,值照调用方原样 —— 空串是"跟随桌面"的合法取值,必须
 * 显式 set 而不是缺席(缺席=不改,用户就永远回不到"跟随桌面")。
 * 返回 false = 词表外,拒绝写。
 */
fun applyControlAgentsSetting(b: pocketagent.v1.SetCodexSettingsRequest.Builder, key: String, value: String): Boolean {
    if (key !in AGENTS_SETTING_KEYS) return false
    when (key) {
        "approval_policy" -> b.setApprovalPolicy(value)
        "sandbox" -> b.setSandbox(value)
        "model" -> b.setModel(value)
        "effort" -> b.setEffort(value)
        "model_verbosity" -> b.setModelVerbosity(value)
        "model_reasoning_summary" -> b.setModelReasoningSummary(value)
    }
    return true
}

/**
 * 控制中心 collab 数值 → CollabPolicy.Builder。成员上限是**计数语义**(0=不限,不是
 * 清掉),不做 MB 换算;单字段写复用既有 [applyCollabEdit]。返回 false = 路由表外
 * (预算键/开关键都不是这条通道),拒绝写。
 */
fun applyControlCollabNumber(b: CollabPolicy.Builder, itemId: String, value: Long): Boolean {
    if (itemId != COLLAB_MEMBERS_ID) return false
    applyCollabEdit(b, "max_parallel_members", value)
    return true
}

/**
 * 控制中心数值输入框的单位口径:**计数键给计数单位,字节键给 MB** —— 单位错就是
 * 口径错(把"3 家"当"3 MB"存,值差 2^20 倍;把 MB 键标成"家",存储口径立即失真)。
 */
fun controlNumberUnit(itemId: String): String = when (itemId) {
    "budget.global.max_parallel_turns" -> "路"
    COLLAB_MEMBERS_ID -> "家"
    else -> "MB"
}

/**
 * 域行是否可展开:**域内至少有一项接通了写通道**(动态判定)。daemon 的 writable 是
 * 域级声明("有通道"),但客户端只对**接通的**通道摆入口 —— 全只读的域(纯事实项,
 * 或通道尚未接通)不摆展开箭头,免得展开体里一行可写项都没有,像给了个假编辑区。
 */
fun controlDomainExpandable(items: List<ControlItemRow>): Boolean =
    items.any { it.action != ControlItemAction.READ_ONLY }

/**
 * 一项的写动作判定。路由表与 daemon/internal/server/control_plane.go 的取值形态
 * 一一对应:collab 的三个 toggle、collab/budget 的 number、agents 的桌面偏好
 * select;其余(agents 的事实项、select 无候选/词表外、未来新 kind)一律只读 ——
 * 没接通的写通道绝不摆入口。
 */
fun controlItemAction(domainId: String, item: ControlItem): ControlItemAction = when {
    domainId == CONTROL_DOMAIN_COLLAB && item.kind == "toggle" && item.id in COLLAB_TOGGLE_IDS ->
        ControlItemAction.COLLAB_TOGGLE
    domainId == CONTROL_DOMAIN_COLLAB && item.kind == "number" && item.id == COLLAB_MEMBERS_ID ->
        ControlItemAction.COLLAB_NUMBER
    domainId == CONTROL_DOMAIN_BUDGET && item.kind == "number" && controlBudgetKey(item.id) != null ->
        ControlItemAction.BUDGET_NUMBER
    // agents 域的桌面偏好(每 agent 桌面审批/沙箱等枚举)走既有 SetCodexSettings:
    // 路由按 "agents.<agentId>.<键>" 拆 agentId 与设置键;候选(options)必须由
    // daemon 给齐 —— 没有可选值的编辑入口是假入口,仍按只读呈现。
    domainId == CONTROL_DOMAIN_AGENTS && item.kind == "select" &&
        controlAgentsTarget(item.id) != null && item.optionsCount > 0 ->
        ControlItemAction.AGENTS_SELECT
    else -> ControlItemAction.READ_ONLY
}

private fun controlItemRow(domainId: String, item: ControlItem): ControlItemRow = ControlItemRow(
    domainId = domainId,
    id = item.id,
    label = item.label,
    current = item.current,
    kind = item.kind,
    hint = item.hint,
    options = item.optionsList,
    action = controlItemAction(domainId, item),
)

/**
 * ControlPlane → 域行列表。**null = 整条 RPC 没成功,返回空**(失败不清空旧值的
 * 责任在 [ControlPlaneState],这里只管"有没有数据");次序照 daemon 钉死的域清单,
 * 客户端不重排 —— 同一份数据两次刷新版面换位,看起来就像出了问题。
 */
fun controlDomainRows(plane: ControlPlane?): List<ControlDomainRow> {
    if (plane == null) return emptyList()
    return plane.domainsList.map { d ->
        ControlDomainRow(
            id = d.id,
            title = d.title,
            summary = d.summary,
            writable = d.writable,
            statusCode = d.statusCode,
            dotKind = controlDotKind(d.statusCode),
            items = d.itemsList.map { controlItemRow(d.id, it) },
        )
    }
}

/**
 * toggle 现值 → 布尔。daemon 侧只产 "开"/"关";别的值说明我们没读懂(新版本换了
 * 词表/新 kind 混进来),返回 null 让 UI 显示原文 —— 猜一个布尔值就是造一份假账。
 */
fun controlToggleOn(current: String): Boolean? = when (current) {
    "开" -> true
    "关" -> false
    else -> null
}

/**
 * 扇出成员上限现值("2")→ 人话段("上限 2";0 → "上限不限",daemon membersText 同一
 * 语义:0 是不限不是零家);读不出(缺席/非纯数值)→ 空串,调用方整段不摆 —— 不编数字。
 */
internal fun collabMembersCapPhrase(current: String): String {
    val s = current.trim()
    if (s.isEmpty() || !s.all { it.isDigit() }) return ""
    val n = s.toLongOrNull() ?: return ""
    return if (n <= 0) "上限不限" else "上限 $n"
}

/**
 * 协同域摘要的展示位组合(L2 真机发现 3:daemon 摘要只看总闸 —— 总开即宣称
 * 「委派与扇出可用」,而委派/扇出子开关各有「关」;子项关着时派发被 daemon 拒,
 * 横幅如实、摘要却说"可用",两者互相矛盾)。
 *
 * 组合口径(照 daemon 词表「开/关」,不猜;对应 daemon/internal/server/control_plane.go
 * controlDomainCollab 的三态):
 *  - 总开关关 → 「协同已关闭」(子开关再开也没用,与 daemon 侧同一句实话);
 *  - 总开关开 → 按子项实况拼:**两路全开才说「委派与扇出可用」**,否则逐项报实况
 *    (「委派关 · 扇出开(上限 2)」);上限段只跟「扇出开」走(扇出关时上限没有意义);
 *  - 子项在而现值认不出(词表外)→ 该项报「未识别」,不猜布尔;
 *  - 总开关/子项缺席(快照形态不在预期)→ 返回 null,**照 daemon 原文呈现** ——
 *    拼不出完整实况就退回 daemon 的话,不拼接半份事实。
 *
 * 【为什么是展示位组合而不是改 daemon】同一快照两个读法才有"App 与 daemon 说的
 * 不一样"的新矛盾;组合用的现值就是本域 items 的 daemon 原文,数据层
 * ([controlDomainRows])照旧原文不改写,单测钉住两层分工。
 */
internal fun collabSummaryOverride(domain: ControlDomainRow): String? {
    if (domain.id != CONTROL_DOMAIN_COLLAB) return null
    val byId = domain.items.associateBy { it.id }
    // 总开关:项缺席 / 现值认不出 → 拼不出实况,退 daemon 原文。
    val masterOn = byId[COLLAB_MASTER_SWITCH_ID]?.let { controlToggleOn(it.current) } ?: return null
    if (!masterOn) return "协同已关闭"
    // 子开关:项缺席 → 退 daemon 原文;项在而现值认不出 → 保留 null,该项报
    // 「未识别」(与直控开关状态词同款),不猜布尔 —— 两种缺席不许混为一谈。
    val delegateItem = byId[COLLAB_DELEGATE_ID] ?: return null
    val fanoutItem = byId[COLLAB_FANOUT_ID] ?: return null
    val delegateOn = controlToggleOn(delegateItem.current)
    val fanoutOn = controlToggleOn(fanoutItem.current)
    val capSuffix = collabMembersCapPhrase(byId[COLLAB_MEMBERS_ID]?.current.orEmpty())
        .takeIf { it.isNotEmpty() }?.let { "($it)" } ?: ""
    return if (delegateOn == true && fanoutOn == true) {
        "协同已开启 · 委派与扇出可用$capSuffix"
    } else {
        "协同已开启 · " + listOf(
            when (delegateOn) { true -> "委派开"; false -> "委派关"; null -> "委派未识别" },
            when (fanoutOn) { true -> "扇出开$capSuffix"; false -> "扇出关"; null -> "扇出未识别" },
        ).joinToString(" · ")
    }
}

/**
 * 控制中心 toggle → CollabPolicy.Builder。返回 false = 该键没有对应写通道
 * (调用方必须把它当失败处理,而不是静默不写)。
 *
 * 【为什么在 Builder 上收口】与「运行时→协同策略」区段共用同一组 setter:同一个
 * 开关从两个界面写,写的是同一个字段、同一套 CAS,不存在"两边各写各的"。
 */
fun applyControlToggle(b: CollabPolicy.Builder, itemId: String, on: Boolean): Boolean {
    return when (itemId) {
        "collab.collaboration_enabled" -> { b.setCollaborationEnabled(on); true }
        "collab.native_delegate_enabled" -> { b.setNativeDelegateEnabled(on); true }
        "collab.fanout_enabled" -> { b.setFanoutEnabled(on); true }
        else -> false
    }
}

/**
 * 控制中心 budget 数值 → RuntimePolicy.Builder。**入参是 MB(字节键)或原值
 * (计数键)**,字节键在这里换算成字节 —— 输入框口径与 daemon 现值口径一致,
 * 存储口径与字段语义一致,两级口径各自对齐。返回 false = 路由表外,拒绝写。
 */
fun applyControlBudgetNumber(b: RuntimePolicy.Builder, itemId: String, value: Long): Boolean {
    val key = controlBudgetKey(itemId) ?: return false
    // 单字段写与 presence 语义(null=清掉)都复用既有 [applyBudgetEdit],不另立一份。
    val bytesOrCount = if (key == "max_parallel_turns") value else value shl 20
    applyBudgetEdit(b, key, bytesOrCount)
    return true
}

/**
 * 数字编辑框的预填值。**只回填能与输入框口径(MB/计数)对上的现值**:
 * "8 MB" → "8"、"3" → "3";"8192 KB"/"12345 B" 换不成整数 MB,宁可留空让用户
 * 对照上方"当前值"原文重输 —— 回填 "8192" 会把 8 MB 放大成 8192 MB。
 */
fun controlNumberPrefill(current: String): String {
    val s = current.trim()
    if (s.endsWith(" MB")) {
        val digits = s.removeSuffix(" MB").trim()
        return if (digits.isNotEmpty() && digits.all { it.isDigit() }) digits else ""
    }
    return if (s.isNotEmpty() && s.all { it.isDigit() }) s else ""
}

/**
 * daemon 启动时刻 → 「已运行 N」人话。now 由调用方注入,保持纯函数
 * (与 [relativeTimeLabel] 同款纪律:相对时间是 (unixMs, now) 的二元函数)。
 * 档位:不到 1 分钟「刚刚启动」;不到 1 小时按分钟;不到 1 天按小时;更长按天。
 */
fun uptimeDurationLabel(startedUnixMs: Long, nowMs: Long): String {
    if (startedUnixMs <= 0) return ""
    val diff = nowMs - startedUnixMs
    // 手机时钟偏慢(启动时刻在未来)回落"刚刚启动",不显示"负 N 分钟"。
    if (diff < 60_000L) return "刚刚启动"
    return when {
        diff < 3_600_000L -> "已运行 ${diff / 60_000L} 分钟"
        diff < 86_400_000L -> "已运行 ${diff / 3_600_000L} 小时"
        else -> "已运行 ${diff / 86_400_000L} 天"
    }
}

/**
 * 入口行的事实串:"主机名 · v版本 · 已运行 3 小时"。缺了哪段就不摆哪段
 * (不以分隔符开头/结尾)—— 空字段补占位符等于编一份身份。
 */
fun controlPlaneEntryDesc(plane: ControlPlane, nowMs: Long): String = listOfNotNull(
    plane.hostName.takeIf { it.isNotBlank() },
    plane.daemonVersion.takeIf { it.isNotBlank() }?.let { "v$it" },
    uptimeDurationLabel(plane.uptimeUnixMs, nowMs).takeIf { it.isNotEmpty() },
).joinToString(" · ")

/** 展开体顶部的身份事实行:"Windows · 数据目录:LOCALAPPDATA"(照 proto 标签原样)。 */
fun controlPlaneFactsLine(plane: ControlPlane): String = listOfNotNull(
    plane.os.takeIf { it.isNotBlank() },
    plane.dataDirLabel.takeIf { it.isNotBlank() }?.let { "数据目录:$it" },
).joinToString(" · ")

/**
 * memory 域里第一个"按 agent"的记忆项(id = memory.<agentId>)的 agentId。
 * **null = 还没有 Agent 接入**(只剩合计项)—— 内存弹窗必须落在具体 agent 上,
 * 没有目标就禁用入口,不盲开一个空弹窗。
 */
fun memoryAgentTargetOf(items: List<ControlItemRow>): String? =
    items.firstOrNull { it.id.startsWith("memory.") && it.id != "memory.total" }
        ?.id?.removePrefix("memory.")?.takeIf { it.isNotBlank() }

/**
 * 本机是否"此 daemon 未提供控制面"(GetControlPlane 回 UNIMPLEMENTED)。
 *
 * 【为什么按状态码判而不是试错重试】与 [memoryMgmtUnsupported] 同款判据:老 daemon
 * 对新 RPC 稳定回 UNIMPLEMENTED,这是能力缺席的确定信号,据此把入口整体隐藏;
 * 其余失败(网络/参数)照常按失败呈现,不与"版本没这功能"混为一谈。
 */
internal fun controlPlaneUnsupported(e: Throwable): Boolean =
    Status.fromThrowable(e).code == Status.Code.UNIMPLEMENTED

// ---- RPC 数据层 ----------------------------------------------------------------

/**
 * 控制中心状态(仿 [RuntimeControlState] 风格)。
 *
 * 【失败不清空旧值】刷新失败时 [plane] 保持最后一次成功快照,失败原因进
 * [refreshError] —— "看到的一直是最后一次成功的数据 + 一行原因",而不是一失败
 * 就整块消失,让用户分不清"坏了"还是"换页了"。
 *
 * 【不做乐观改写】toggle/number 提交成功后回读 GetControlPlane:开关停在什么位置
 * 由 daemon 的下一份快照决定,"看到的就是生效的"由 daemon 保证 —— 手机端不先改
 * 界面再赌 RPC 成功。
 */
object ControlPlaneState {
    var loadState by mutableStateOf(RuntimeLoadState.LOADING)
        private set

    /** 最后一次成功的控制面快照;失败时保持旧值。null = 还没成功读到过。 */
    var plane by mutableStateOf<ControlPlane?>(null)
        private set

    /**
     * 此 daemon 未提供控制面(GetControlPlane 回 UNIMPLEMENTED)。**入口整体隐藏**
     * —— 摆一排点了必然空着的域,比不摆更糟;这也是一种事实陈述(版本事实)。
     */
    var unsupported by mutableStateOf(false)
        private set

    /** 最近一次刷新失败原因(daemon 原文/传输原因);成功后清空。 */
    var refreshError by mutableStateOf<String?>(null)
        private set

    /** 正在提交的项(itemId);提交期间所有开关禁用,避免连点叠加两次 Set。 */
    var busyItemId by mutableStateOf<String?>(null)
        private set

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    private suspend fun fetchPlane(): ControlPlane = withContext(Dispatchers.IO) {
        AgentConnection.runtimeService().getControlPlane(GetControlPlaneRequest.getDefaultInstance())
    }

    /** 拉一次控制面(进页面时调;失败只记原因,不清旧值,不抛)。 */
    fun refresh() {
        scope.launch { refreshSuspend() }
    }

    /**
     * 刷新的可挂起版本:写动作成功后回读用。返回失败原因(null = 成功或能力缺席)。
     * UNIMPLEMENTED 记成 [unsupported] 事实并清掉失败原因 —— 能力缺席不是失败。
     */
    suspend fun refreshSuspend(): String? {
        if (unsupported) return null
        return try {
            plane = fetchPlane()
            refreshError = null
            loadState = RuntimeLoadState.READY
            null
        } catch (e: Exception) {
            if (controlPlaneUnsupported(e)) {
                unsupported = true
                refreshError = null
                null
            } else {
                val msg = RuntimeControlState.extractRpcMessage(e)
                refreshError = msg
                if (plane == null) loadState = RuntimeLoadState.ERROR
                msg
            }
        }
    }

    /**
     * 切一个 collab toggle。**路由:既有 SetCollabPolicy(完整策略 + CAS)**,
     * 键 → setter 的映射在 [applyControlToggle](纯函数,单测钉死)。成功后回读
     * GetControlPlane —— 不做乐观改写。
     */
    suspend fun applyToggle(domainId: String, itemId: String, newValue: Boolean): Result<Unit> {
        busyItemId = itemId
        try {
            if (domainId != CONTROL_DOMAIN_COLLAB) {
                return Result.failure(IllegalStateException("该开关暂不支持在控制中心切换"))
            }
            // 协同快照缺席时先 Get 一次:控制中心可能先于「运行时」区段被使用,
            // 没有 revision 就没法做 CAS(失败原因由快照保障函数给出)。
            RuntimeControlState.ensureCollabSnapshot()?.let { return Result.failure(IllegalStateException(it)) }
            val err = RuntimeControlState.saveCollabPolicy { b ->
                check(applyControlToggle(b, itemId, newValue)) { "协同开关没有对应的写通道:$itemId" }
            }
            if (err != null) return Result.failure(IllegalStateException(err))
            refreshSuspend()
            return Result.success(Unit)
        } finally {
            busyItemId = null
        }
    }

    /**
     * 改一个数值。**路由:各自域的既有 Set RPC(完整策略 + CAS)** —— budget 走
     * SetRuntimePolicy、collab 走 SetCollabPolicy(成员上限属协同策略树,两套落盘
     * 两套 CAS,写错对象=改了不生效);入参 MB(字节键)/原值(计数键),换算在
     * [applyControlBudgetNumber] / [applyControlCollabNumber]。成功后回读
     * GetControlPlane,不做乐观改写。
     */
    suspend fun applyNumber(domainId: String, itemId: String, newValue: Long): Result<Unit> {
        busyItemId = itemId
        try {
            when (domainId) {
                CONTROL_DOMAIN_BUDGET -> {
                    // 预算快照同理:CAS 需要 revision,缺席先 Get(与 GetRuntimePolicy 同款读法)。
                    RuntimeControlState.ensureBudgetSnapshot()?.let { return Result.failure(IllegalStateException(it)) }
                    val err = RuntimeControlState.saveBudget { b ->
                        check(applyControlBudgetNumber(b, itemId, newValue)) { "预算项没有对应的写通道:$itemId" }
                    }
                    if (err != null) return Result.failure(IllegalStateException(err))
                }
                CONTROL_DOMAIN_COLLAB -> {
                    // 协同快照缺席先 Get(与 applyToggle 同款理由):CAS 需要 revision。
                    RuntimeControlState.ensureCollabSnapshot()?.let { return Result.failure(IllegalStateException(it)) }
                    val err = RuntimeControlState.saveCollabPolicy { b ->
                        check(applyControlCollabNumber(b, itemId, newValue)) { "协同数值没有对应的写通道:$itemId" }
                    }
                    if (err != null) return Result.failure(IllegalStateException(err))
                }
                else -> return Result.failure(IllegalStateException("该项暂不支持在控制中心编辑"))
            }
            refreshSuspend()
            return Result.success(Unit)
        } finally {
            busyItemId = null
        }
    }

    /**
     * 切一个 agents 桌面偏好(select)。**路由:既有 SetCodexSettings**;键→字段的
     * 映射在 [applyControlAgentsSetting],值域以 daemon 给的 options 为准
     * ([controlAgentsOptionInvalid])。成功后回读 GetControlPlane,不做乐观改写。
     */
    suspend fun applySelect(domainId: String, itemId: String, newValue: String, options: List<String>): Result<Unit> {
        busyItemId = itemId
        try {
            if (domainId != CONTROL_DOMAIN_AGENTS) {
                return Result.failure(IllegalStateException("该项暂不支持在控制中心切换"))
            }
            val target = controlAgentsTarget(itemId)
                ?: return Result.failure(IllegalStateException("该项没有对应的写通道:$itemId"))
            // 值域校验放在路由之后、写之前:枚举写错档的脏值在客户端先拦下并给原因,
            // 不放过去让 daemon 替我们兜底。
            controlAgentsOptionInvalid(newValue, options)?.let { return Result.failure(IllegalStateException(it)) }
            val err = RuntimeControlState.saveCodexSetting(target.agentId, target.key, newValue)
            if (err != null) return Result.failure(IllegalStateException(err))
            refreshSuspend()
            return Result.success(Unit)
        } finally {
            busyItemId = null
        }
    }

    /** 退出页面/换连 daemon 时清空本地快照(下次进入重新拉,避免展示上一台的数据)。 */
    fun reset() {
        plane = null
        refreshError = null
        unsupported = false
        busyItemId = null
        loadState = RuntimeLoadState.LOADING
    }
}
