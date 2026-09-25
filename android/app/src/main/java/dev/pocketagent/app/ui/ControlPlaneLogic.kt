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
 * 【写通道刻意不新增】ControlItem 只是呈现;toggle 走既有 SetCollabPolicy、budget
 * 数值走既有 SetRuntimePolicy(完整策略 + CAS,纪律在 [RuntimeControlState] 的
 * save* 里)。给总览另配一套写路径 = 给同一份状态造第二份真值。本轮只对接两类:
 * collab 域的 toggle 与 budget 域的 number;agents 域的 SetCodexSettings 路由
 * 留 TODO(见 [controlItemRow]),其余一律只读呈现 —— 没接通的写通道绝不摆入口。
 */

// ---- 域 id(daemon 契约钉死的稳定键,客户端按 id 匹配、不按 title) --------------

const val CONTROL_DOMAIN_BUDGET = "budget"
const val CONTROL_DOMAIN_COLLAB = "collab"
const val CONTROL_DOMAIN_MEMORY = "memory"
const val CONTROL_DOMAIN_DIALYSIS = "dialysis"

// ---- 展示模型 ------------------------------------------------------------------

/** 状态圆点三档(0 灰 / 1 黄 / 2 红;语义色只用在 8dp 小圆点上)。 */
enum class ControlDotKind { NORMAL, ATTENTION, LIMITED }

/**
 * 一项的写动作。**本轮真正对接的只有 COLLAB_TOGGLE 与 BUDGET_NUMBER**;
 * 其余(含未来可能出现的 select)一律 READ_ONLY —— 没接通的写通道不摆入口。
 */
enum class ControlItemAction { COLLAB_TOGGLE, BUDGET_NUMBER, READ_ONLY }

/** 域内一项的展示模型(字段全部照 daemon 原文带出,不翻译不改写)。 */
data class ControlItemRow(
    val id: String,
    val label: String,
    val current: String,
    /** 照 proto 原文:toggle / select / number / info;出现新值照抄,渲染为只读。 */
    val kind: String,
    val hint: String,
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
 * 一项的写动作判定。路由表与 daemon/internal/server/control_plane.go 的取值形态
 * 一一对应:collab 的三个 toggle、budget.global 的七个 number;其余(agents 的
 * SetCodexSettings、collab 的 number、select/未来新 kind)本轮一律只读。
 */
fun controlItemAction(domainId: String, item: ControlItem): ControlItemAction = when {
    domainId == CONTROL_DOMAIN_COLLAB && item.kind == "toggle" && item.id in COLLAB_TOGGLE_IDS ->
        ControlItemAction.COLLAB_TOGGLE
    domainId == CONTROL_DOMAIN_BUDGET && item.kind == "number" && controlBudgetKey(item.id) != null ->
        ControlItemAction.BUDGET_NUMBER
    // TODO(控制中心下一轮):agents 域声明了写通道(每 agent 桌面偏好与原生设置,
    //  走既有 SetCodexSettings/ListAgentConfigOptions)。路由按 "agents.<agentId>.<键>"
    //  拆 agentId 与设置键再映射;本轮先声明未实现 —— 只读呈现,不摆假的编辑入口。
    else -> ControlItemAction.READ_ONLY
}

private fun controlItemRow(domainId: String, item: ControlItem): ControlItemRow = ControlItemRow(
    id = item.id,
    label = item.label,
    current = item.current,
    kind = item.kind,
    hint = item.hint,
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
     * 改一个 budget 数值。**路由:既有 SetRuntimePolicy(完整策略 + CAS)**;入参
     * MB(字节键)/原值(计数键),换算在 [applyControlBudgetNumber]。成功后回读
     * GetControlPlane,不做乐观改写。
     */
    suspend fun applyNumber(domainId: String, itemId: String, newValue: Long): Result<Unit> {
        busyItemId = itemId
        try {
            if (domainId != CONTROL_DOMAIN_BUDGET) {
                return Result.failure(IllegalStateException("该项暂不支持在控制中心编辑"))
            }
            // 预算快照同理:CAS 需要 revision,缺席先 Get(与 GetRuntimePolicy 同款读法)。
            RuntimeControlState.ensureBudgetSnapshot()?.let { return Result.failure(IllegalStateException(it)) }
            val err = RuntimeControlState.saveBudget { b ->
                check(applyControlBudgetNumber(b, itemId, newValue)) { "预算项没有对应的写通道:$itemId" }
            }
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
