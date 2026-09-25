package dev.pocketagent.app.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import dev.pocketagent.app.AgentConnection
import io.grpc.Status
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.AcknowledgeUnknownEventRequest
import pocketagent.v1.AgentCacheUsage
import pocketagent.v1.AgentCheckpointUsage
import pocketagent.v1.AgentMemoryUsage
import pocketagent.v1.AgentRecoveryState
import pocketagent.v1.AgentRuntimeResponse
import pocketagent.v1.CollabPolicy
import pocketagent.v1.CollabPolicyResponse
import pocketagent.v1.ClearThreadMemoryRequest
import pocketagent.v1.ClearThreadMemoryResponse
import pocketagent.v1.GetAgentRuntimeRequest
import pocketagent.v1.GetCollabPolicyRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.GetRuntimePolicyRequest
import pocketagent.v1.GetRuntimeStateRequest
import pocketagent.v1.GetUnknownEventRequest
import pocketagent.v1.HostInfo
import pocketagent.v1.HostRecoveryReport
import pocketagent.v1.ListThreadMemoriesRequest
import pocketagent.v1.ListThreadMemoriesResponse
import pocketagent.v1.ListUnknownEventsRequest
import pocketagent.v1.ListUnknownEventsResponse
import pocketagent.v1.RuntimeContractReport
import pocketagent.v1.RuntimeLimits
import pocketagent.v1.RuntimePolicyResponse
import pocketagent.v1.RuntimeStateResponse
import pocketagent.v1.SetCodexSettingsRequest
import pocketagent.v1.SetCollabPolicyRequest
import pocketagent.v1.SetRuntimePolicyRequest
import pocketagent.v1.UnknownEventResponse
import pocketagent.v1.UnknownEventSummary
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * 运行时控制面(W11)的数据层:**RPC 调用 + daemon 原文 → UI 模型的映射**。
 *
 * 【为什么单独成层,而不是直接写在设置页里】设置页是 Compose 渲染,单测跑不了;
 * 而这一层最容易犯的错是**静默造值**(optional 项缺席时读 getter 得到 0/false,
 * 于是"没设上限"被显示成"封禁"、"没配策略"显示成"全关")。把映射抽成纯函数,
 * 就能在 `test` 阶段把这些钉子钉死,不必依赖真机与 daemon。
 *
 * 【它不做什么】不碰 Compose 排版(见 [RuntimeSection]);不发明任何 daemon 语义 ——
 * 判据一律取自 daemon 回传的字段(status / effective / declared / issues),不在
 * 手机端二次推导(那会给"同一事实两处推导"留后门,规格 §21.2)。
 */

// ---- 能力判定 ---------------------------------------------------------------

/**
 * 本机是否支持运行时控制面(daemon 的 capabilities 里有 `runtime.v1`)。
 *
 * 【为什么不试错式地点亮】旧 daemon 没有这些 RPC,点了必然 UNIMPLEMENTED。
 * 摆一排必然失败的开关比不摆更糟 —— 用户会以为功能坏了(规格 §21.3)。
 */
fun runtimeControlCapable(capabilities: List<String>): Boolean =
    capabilities.any { it.equals("runtime.v1", ignoreCase = true) }

// ---- 启动恢复横幅 ------------------------------------------------------------

/** 启动恢复横幅的展示模型([degraded] 决定配色与是否默认展开明细)。 */
data class RecoveryBanner(
    val status: String,
    val headline: String,
    /** 逐条"哪句没跑到"(用户裁定:有明细就必须逐条给,只报条数等于没说)。 */
    val lostLines: List<String>,
    /** 涉及几个会话 —— 与条数分开:同一会话丢两句是两句话、一个会话。 */
    val threadCount: Int,
    val degraded: Boolean,
)

/**
 * 把启动恢复报告映射成横幅;**返回 null = 旧 daemon 根本没报告这一项,整块隐藏**。
 *
 * 【degraded 为什么用 `status != "ok"` 而不是自己综合四路健康度】
 * daemon 侧的 overallStatus 已经把"会话库被隔离 / 设备表降级 / 身份不可用 /
 * 有排队输入丢失"折算成 ok|degraded|corrupted,协议层再算一遍就是两处推导。
 * 唯一的例外是 status 为空:那只能说明我们没读懂,此时绝不能外推成"一切正常"。
 */
fun recoveryBanner(report: HostRecoveryReport?): RecoveryBanner? {
    if (report == null) return null
    val status = report.status
    val journal = if (report.hasTurnJournal()) report.turnJournal else null
    val lost = journal?.lostInputsList.orEmpty().map { item ->
        val preview = item.preview.ifBlank { "(内容未能读出)" }
        val at = timeLabel(item.createdAtUnixMs)
        if (at.isEmpty()) "「$preview」" else "「$preview」 · $at"
    }
    return RecoveryBanner(
        status = status,
        headline = report.summary.ifBlank { "启动状态:$status" },
        lostLines = lost,
        threadCount = journal?.threadIdsCount ?: 0,
        degraded = status != "ok",
    )
}

// ---- 本机总览 ----------------------------------------------------------------

/** 本机运行时总览(总览行 + 后续各块共用的版本/代数信息)。 */
data class RuntimeOverview(
    val stateLabel: String,
    val degraded: Boolean,
    val version: String,
    val budgetRevision: Long,
    val collabRevision: Long,
    val dataDir: String,
)

/** 总览;**null = 没读到(旧 daemon 或 RPC 失败),UI 走空态而不是显示一排 0**。 */
fun runtimeOverview(resp: RuntimeStateResponse?): RuntimeOverview? {
    if (resp == null) return null
    val degraded = resp.runtimeState != "ready"
    return RuntimeOverview(
        // 取值域由 daemon 定义(ready / degraded);将来多一档时原样透出,不猜。
        stateLabel = when (resp.runtimeState) {
            "ready" -> "运行时形态就绪"
            "degraded" -> "有 agent 未交代清楚运行时形态"
            else -> resp.runtimeState
        },
        degraded = degraded,
        version = resp.daemonVersion.ifBlank { "版本未报告" },
        budgetRevision = resp.budgetPolicyRevision,
        collabRevision = resp.collabPolicyRevision,
        dataDir = resp.dataDir,
    )
}

// ---- 每 agent 形态卡 ---------------------------------------------------------

/** 形态卡里的一行(会话 / 恢复 / 审批 / 子代理 / 记忆 / 缓存)。 */
data class ContractRow(val label: String, val value: String)

/** 一个 agent 的运行时形态申报与核验结果。 */
data class AgentContractCard(
    val agentId: String,
    val title: String,
    val versionLine: String,
    val declared: Boolean,
    /** 未申报时的人话说明(AUDIT §3.3:讲清事实,不写"未知/错误")。 */
    val note: String?,
    val rows: List<ContractRow>,
    val issues: List<String>,
    val limitations: List<String>,
    /** 与 daemon 的 ContractReport.OK() 同判据:无违约即达标(不自行发明标准)。 */
    val ok: Boolean,
)

fun agentContractCards(resp: RuntimeStateResponse?): List<AgentContractCard> =
    resp?.contractsList.orEmpty().map { agentContractCard(it) }

fun agentContractCard(r: RuntimeContractReport): AgentContractCard {
    val declared = r.declared
    // 未申报时**不摊开六个空形态行**:那六行什么都不表示,反而遮住"这家根本没交代"这件事。
    val rows = if (!declared) emptyList() else listOfNotNull(
        contractRow("会话", sessionModelLabel(r.sessionModel), r.sessionModel),
        contractRow("恢复", resumeModelLabel(r.resumeModel), r.resumeModel),
        contractRow("审批", approvalModelLabel(r.approvalModel), r.approvalModel),
        contractRow("子代理", subagentModelLabel(r.subagentModel), r.subagentModel),
        contractRow("记忆", memoryModelLabel(r.memoryModel), r.memoryModel),
        contractRow("缓存", cacheModelLabel(r.cacheModel), r.cacheModel),
    )
    return AgentContractCard(
        agentId = r.agentId,
        title = r.displayName.ifBlank { r.agentId },
        versionLine = r.version.ifBlank { "版本未报告" },
        declared = declared,
        note = if (declared) null else "该 agent 未申报运行时形态",
        rows = rows,
        issues = r.issuesList.map { if (it.detail.isBlank()) it.code else "${it.code}:${it.detail}" },
        limitations = r.limitationsList.map {
            if (it.reason.isBlank()) it.capability else "${it.capability}:${it.reason}"
        },
        ok = r.issuesList.isEmpty(),
    )
}

private fun contractRow(label: String, value: String, raw: String): ContractRow? =
    if (raw.isBlank()) null else ContractRow(label, value)

// 六个形态字段的值域见 daemon/internal/runtime/descriptor.go。
// 【为什么是 when + else 照抄】值域由 daemon 单方面定义;出现新取值时照抄原文,
// 比回落到一个"看起来差不多"的中文更安全 —— 后者会被当成已支持。

fun sessionModelLabel(raw: String): String = when (raw) {
    "NATIVE" -> "原生会话"
    "DAEMON" -> "由 daemon 托管"
    "HYBRID" -> "混合"
    "NONE" -> "无持久会话"
    else -> raw
}

fun resumeModelLabel(raw: String): String = when (raw) {
    "native" -> "原生恢复"
    "reattach" -> "重挂载"
    "none" -> "不可恢复"
    else -> raw
}

fun approvalModelLabel(raw: String): String = when (raw) {
    "native" -> "原生审批"
    "daemon" -> "daemon 中转"
    "none" -> "无审批"
    else -> raw
}

fun subagentModelLabel(raw: String): String = when (raw) {
    "native" -> "原生子代理"
    "delegate" -> "可委派"
    "none" -> "不支持"
    else -> raw
}

fun memoryModelLabel(raw: String): String = when (raw) {
    "native" -> "原生"
    "daemon" -> "由 daemon 承担"
    "hybrid" -> "混合"
    else -> raw
}

fun cacheModelLabel(raw: String): String = when (raw) {
    "native" -> "原生"
    "daemon" -> "由 daemon 承担"
    else -> raw
}

// ---- 每 agent 用量(§9.5 后四块)----------------------------------------------

/** 形态卡展开体里的用量行(GetAgentRuntime 的 memory/cache/checkpoint/recovery 四块)。 */
data class RuntimeUsageRow(val label: String, val value: String, val key: String? = null)

// memory.kinds 的分层顺序照 proto 注释里的 working/conversation/summary/artifact;
// map 本身无序,不钉一个顺序的话同一份数据两次刷新会换位,看起来像出了问题。
private val memoryKindOrder = listOf("working", "conversation", "summary", "artifact")

/**
 * GetAgentRuntime 响应的四块用量 → 展开体数值行(排在资源租约行之下)。
 *
 * 【为什么 resp 为 null 时返回空而不是占位】null = 整条 RPC 没成功,那是"没问到",
 * 不是"daemon 没提供";失败原因已由展开体里资源行的 note 说清,这里再补四行占位
 * 等于把一次网络失败放大成四个疑点。
 *
 * 【为什么逐块查 has】proto3 的消息字段缺席时读 getter 会得到全零对象,"旧 daemon
 * 没这块"与"这块的真实观测就是 0"在界面上必须分得开:前者说"daemon 未提供",
 * 后者照实显示 0 —— 把缺席画成 0 就是造了一份假账。
 */
fun agentRuntimeUsageRows(resp: AgentRuntimeResponse?): List<RuntimeUsageRow> {
    if (resp == null) return emptyList()
    return listOf(
        memoryUsageRow(if (resp.hasMemory()) resp.memory else null),
        cacheUsageRow(if (resp.hasCache()) resp.cache else null),
        checkpointUsageRow(if (resp.hasCheckpoint()) resp.checkpoint else null),
        recoveryUsageRow(if (resp.hasRecovery()) resp.recovery else null),
    )
}

/** 记忆块:线程 / 条目 / 字节,带分层计数(键名照 proto 原文透出,值域由 daemon 定义)。 */
fun memoryUsageRow(m: AgentMemoryUsage?): RuntimeUsageRow {
    if (m == null) return RuntimeUsageRow("记忆用量", "daemon 未提供")
    val parts = mutableListOf("${m.threads} 会话", "${m.items} 条", formatBytesLabel(m.bytes))
    if (m.kindsMap.isNotEmpty()) {
        parts += m.kindsMap.entries
            .sortedWith(
                compareBy(
                    { memoryKindOrder.indexOf(it.key).let { i -> if (i >= 0) i else Int.MAX_VALUE } },
                    { it.key },
                )
            )
            .joinToString(" / ") { "${it.key} ${it.value}" }
    }
    // key="memory" 是内存管理入口的锚点:daemon 真给了 memory 块,这一行才可点进
    // 内存详情/管理弹窗;缺席(null)时没有入口 —— 没有数据的入口就是假入口。
    return RuntimeUsageRow("记忆用量", parts.joinToString(" · "), key = "memory")
}

/** 缓存块:命名空间 / 条目 / 字节 / 命中三计数 —— 可再生缓存的量与效率一起给,照 proto。 */
fun cacheUsageRow(c: AgentCacheUsage?): RuntimeUsageRow {
    if (c == null) return RuntimeUsageRow("缓存用量", "daemon 未提供")
    return RuntimeUsageRow(
        "缓存用量",
        listOf(
            "${c.namespaces} 命名空间",
            "${c.entries} 条",
            formatBytesLabel(c.bytes),
            "命中 ${c.hits} / 未命中 ${c.misses} / 淘汰 ${c.evictions}",
        ).joinToString(" · "),
    )
}

/** 检查点块:个数与字节。 */
fun checkpointUsageRow(c: AgentCheckpointUsage?): RuntimeUsageRow {
    if (c == null) return RuntimeUsageRow("检查点", "daemon 未提供")
    return RuntimeUsageRow("检查点", "${c.count} 个 · ${formatBytesLabel(c.bytes)}")
}

/**
 * 恢复块:最近一次恢复尝试的结果。
 *
 * 【为什么"未上报"绝不能画成"失败"】proto 里 AgentRecoveryState 明文说明:daemon 侧
 * 目前还没有产出方,reported 恒为 false —— "没这一项"和"恢复失败"是完全不同的两件事,
 * 把前者读成后者是无中生有的坏消息。这是本块唯一的判据,其余字段照 daemon 原文带出。
 */
fun recoveryUsageRow(r: AgentRecoveryState?): RuntimeUsageRow {
    if (r == null) return RuntimeUsageRow("最近恢复", "daemon 未提供")
    if (!r.reported) return RuntimeUsageRow("最近恢复", "该 agent 未上报恢复状态")
    val parts = mutableListOf(
        when {
            r.succeeded -> "恢复成功"
            r.attempted -> "尝试过恢复,未成功"
            else -> "有恢复记录,未说明结果"
        },
    )
    // "none" 表示没有恢复来源可说,不占一段字。
    if (r.from.isNotBlank() && r.from != "none") parts += "来源 ${recoveredFromLabel(r.from)}"
    if (r.reason.isNotBlank()) parts += r.reason
    if (r.at.isNotBlank()) parts += r.at
    return RuntimeUsageRow("最近恢复", parts.joinToString(" · "))
}

/** 恢复来源的展示:已知取值译成人话,值域外的照抄原文(与六个形态标签同一纪律)。 */
fun recoveredFromLabel(raw: String): String = when (raw) {
    "checkpoint" -> "检查点"
    "native" -> "原生"
    "corrupted" -> "损坏副本"
    else -> raw
}

// ---- 内存管理(按会话的记忆清单与清除) ----------------------------------------

/**
 * 内存详情/管理弹窗里的一个会话行(ListThreadMemories 的一行 → UI 模型)。
 *
 * 【为什么次序不重排】daemon 侧已按字节降序排好(同字节按 thread_id 字典序)。
 * 客户端再排一遍就是"同一事实两处推导",两边口径一旦漂移,用户看到的次序就会
 * 无缘无故变 —— 保持 daemon 的次序,谁排的谁负责。
 */
data class ThreadMemoryRowUi(
    val threadId: String,
    val entries: Int,
    val bytes: Long,
    val seq: Long,
    /** 最后落盘时刻的人话;空串 = 从未落盘(照 proto 的 0 值语义,不编一个时刻)。 */
    val lastCheckpointLabel: String,
    /** false = 重启后从未触碰,值来自盘上落盘那一刻的快照 —— 必须如实标注,不当实时值。 */
    val loaded: Boolean,
)

/**
 * ListThreadMemories 响应 → 会话行列表;**null = 没问到(RPC 失败/旧 daemon),返回空**。
 * 空列表与"没有记忆"由调用方用加载态区分,这里不造第三种值。
 */
fun mapThreadMemories(resp: ListThreadMemoriesResponse?): List<ThreadMemoryRowUi> {
    if (resp == null) return emptyList()
    return resp.rowsList.map { r ->
        ThreadMemoryRowUi(
            threadId = r.threadId,
            entries = r.entries,
            bytes = r.bytes,
            seq = r.seq,
            lastCheckpointLabel = if (r.lastCheckpointUnixMs > 0) relativeTimeLabel(r.lastCheckpointUnixMs) else "",
            loaded = r.loaded,
        )
    }
}

/** 一个会话行的人话正文:"N 条 · 大小 · 落盘 时间[ · 未装载]"。 */
fun threadMemoryLine(r: ThreadMemoryRowUi): String = buildString {
    append(r.entries).append(" 条")
    append(" · ").append(formatBytesLabel(r.bytes))
    if (r.seq > 0) append(" · 序号到 ").append(r.seq)
    if (r.lastCheckpointLabel.isNotEmpty()) append(" · 落盘 ").append(r.lastCheckpointLabel)
    if (!r.loaded) append(" · 未装载")
}

/** 弹窗顶部的合计行模型。 */
data class ThreadMemoryTotalsUi(val threads: Int, val entries: Int, val bytes: Long)

/** 合计行的人话:"N 会话 · M 条 · X"。 */
fun threadTotalsLine(t: ThreadMemoryTotalsUi): String =
    "${t.threads} 会话 · ${t.entries} 条 · ${formatBytesLabel(t.bytes)}"

/**
 * 从行列表现算合计 —— 清除的**乐观更新**唯一算法:行没了,合计同步少。
 * daemon 回传的真值(清除响应里的剩余合计)到达后整行替换它;这里只负责
 * "点了清除到响应回来"这个窗口里界面不出现"行没了合计还在"的自相矛盾。
 */
fun threadTotalsFrom(rows: List<ThreadMemoryRowUi>): ThreadMemoryTotalsUi = ThreadMemoryTotalsUi(
    threads = rows.size,
    entries = rows.sumOf { it.entries },
    bytes = rows.sumOf { it.bytes },
)

/** 乐观移除一个会话行(返回新列表;不动原列表 —— 回滚就是换回原引用)。 */
fun threadRowsWithout(rows: List<ThreadMemoryRowUi>, threadId: String): List<ThreadMemoryRowUi> =
    rows.filter { it.threadId != threadId }

/**
 * 某 agent 申报的记忆形态(descriptor 的 memory_model 原文);null = 没读到申报。
 * 【为什么从总览里找而不是记住上一次】形态申报是启动期核验的快照,总览就是
 * 它的唯一出口;在这里另存一份会跟总览漂移。
 */
fun memoryModelOf(resp: RuntimeStateResponse?, agentId: String): String? =
    resp?.contractsList.orEmpty()
        .firstOrNull { it.agentId == agentId }
        ?.memoryModel?.takeIf { it.isNotBlank() }

/**
 * 记忆形态的人话解释(弹窗顶部那一段)。讲清"记忆记在哪、清除动的是什么",
 * 不讲实现术语;值域外照抄原文(与六个形态标签同一纪律)。
 */
fun memoryModelExplanation(raw: String): String = when (raw) {
    "daemon" -> "这家 agent 的会话记忆由 daemon 记在电脑端的数据目录里。下面的清除只删 PocketAgent 记的这份,不影响电脑端的其他数据。"
    "hybrid" -> "电脑端原生程序保存对话原文,PocketAgent 只记轻量索引与检查点。清除只删这些索引,原生会话内容不受影响。"
    "native" -> "对话记忆由电脑端原生程序自己管理,PocketAgent 侧没有存会话记忆,也没有可清的东西。"
    else -> raw
}

/** 分层计数的人话:已知四层译成人话,值域外照抄 proto 原文。 */
fun memoryKindLabel(raw: String): String = when (raw) {
    "working" -> "工作记忆"
    "conversation" -> "会话记忆"
    "summary" -> "摘要"
    "artifact" -> "产物与索引"
    else -> raw
}

/**
 * 内存管理 RPC 是否属于"此 daemon 未提供"(UNIMPLEMENTED)。
 *
 * 【为什么按状态码判而不是试错重试】旧 daemon 没有这两条 RPC,调用会稳定地回
 * UNIMPLEMENTED —— 这是能力缺席的确定信号,据此把入口灰显并给一句事实陈述;
 * 其余失败(网络/参数)照常按失败呈现,不与"版本没这功能"混为一谈。
 */
internal fun memoryMgmtUnsupported(e: Throwable): Boolean =
    Status.fromThrowable(e).code == Status.Code.UNIMPLEMENTED

/** 分层计数的一行(kind 原文 + 条数)。 */
data class MemoryKindCount(val kind: String, val count: Int)

/**
 * memory.kinds → 排好序的分层计数(照 [memoryKindOrder] 的次序;值域外的排最后、
 * 按键名字典序)。map 本身无序,不钉死次序的话同一份数据两次刷新会换位。
 */
fun memoryKindsList(kinds: Map<String, Int>): List<MemoryKindCount> =
    kinds.entries
        .sortedWith(
            compareBy(
                { memoryKindOrder.indexOf(it.key).let { i -> if (i >= 0) i else Int.MAX_VALUE } },
                { it.key },
            )
        )
        .map { MemoryKindCount(it.key, it.value) }

// ---- 预算 --------------------------------------------------------------------

data class BudgetRow(val key: String, val label: String, val value: String)

/**
 * 预算行(**只用 daemon 给的 effective_global**)。
 *
 * 【为什么不拿 policy 自己判】policy 是带缺席字段的部分对象:用户只改了一项时,
 * 其余项在线上就是 0。手机端手里没有"内置基线"这份数据(它在 daemon 代码里),
 * 把 0 显示出来就是"内存 0 B / 并行 0 路"这类假事实。
 *
 * 【缺席与显式 0 必须分开】逐项查 presence:未设置 = 不出行;显式 0 = 出行且值为 0。
 */
fun budgetRows(limits: RuntimeLimits?): List<BudgetRow> {
    if (limits == null) return emptyList()
    val out = mutableListOf<BudgetRow>()
    if (limits.hasRamWorkingBytes()) {
        out += BudgetRow("ram_working_bytes", "工作内存", formatBytesLabel(limits.ramWorkingBytes))
    }
    if (limits.hasRamEventBufferBytes()) {
        out += BudgetRow("ram_event_buffer_bytes", "事件缓冲", formatBytesLabel(limits.ramEventBufferBytes))
    }
    if (limits.hasRamSessionIndexBytes()) {
        out += BudgetRow("ram_session_index_bytes", "会话索引", formatBytesLabel(limits.ramSessionIndexBytes))
    }
    if (limits.hasDiskConversationBytes()) {
        out += BudgetRow("disk_conversation_bytes", "会话存储", formatBytesLabel(limits.diskConversationBytes))
    }
    if (limits.hasDiskCacheBytes()) {
        out += BudgetRow("disk_cache_bytes", "缓存", formatBytesLabel(limits.diskCacheBytes))
    }
    if (limits.hasDiskArtifactBytes()) {
        out += BudgetRow("disk_artifact_bytes", "产物", formatBytesLabel(limits.diskArtifactBytes))
    }
    if (limits.hasMaxParallelTurns()) {
        out += BudgetRow("max_parallel_turns", "并行轮次", limits.maxParallelTurns.toString())
    }
    return out
}

/** 字节额度的人话呈现(daemon 侧按 1<<20 计的 MB,标签沿用同一口径避免换算假象)。 */
fun formatBytesLabel(bytes: Long): String {
    val abs = if (bytes < 0) -bytes else bytes
    val unit: String
    val div: Long
    when {
        abs >= 1L shl 30 -> { unit = "GB"; div = 1L shl 30 }
        abs >= 1L shl 20 -> { unit = "MB"; div = 1L shl 20 }
        abs >= 1L shl 10 -> { unit = "KB"; div = 1L shl 10 }
        else -> { unit = "B"; div = 1L }
    }
    if (div == 1L) return "$bytes $unit"
    val v = bytes.toDouble() / div
    return if (v == Math.floor(v)) "${v.toLong()} $unit"
    else String.format(Locale.US, "%.1f %s", v, unit)
}

// ---- 协同策略 ----------------------------------------------------------------

data class CollabPolicyRow(val key: String, val label: String, val value: String)

/**
 * 协同策略行(**只用 daemon 给的 effective**)。
 *
 * 【为什么不能拿 policy 自己判】与 [budgetRows] 同款理由:policy 是带缺席项的部分
 * 对象。daemon 侧 Effective() 把未设置项填成"与现状一致"的默认(全开、扇出不限)
 * 之后才给出来 —— 客户端手里没有"默认是什么"这份知识(它在 daemon 的代码里)。
 */
fun collabPolicyRows(effective: CollabPolicy?): List<CollabPolicyRow> {
    if (effective == null) return emptyList()
    val out = mutableListOf<CollabPolicyRow>()
    if (effective.hasCollaborationEnabled()) {
        out += CollabPolicyRow("collaboration_enabled", "多 agent 协同", onOffLabel(effective.collaborationEnabled))
    }
    if (effective.hasNativeDelegateEnabled()) {
        out += CollabPolicyRow("native_delegate_enabled", "委派(agent 派子任务)", onOffLabel(effective.nativeDelegateEnabled))
    }
    if (effective.hasFanoutEnabled()) {
        out += CollabPolicyRow("fanout_enabled", "协同扇出", onOffLabel(effective.fanoutEnabled))
    }
    if (effective.hasMaxParallelMembers()) {
        val n = effective.maxParallelMembers
        // 上限 <=0 在 daemon 语义里是"不限"(见 runtime.CollabPolicy.MaxMembers);
        // 显示成"0 家"会让人以为一家都不许派 —— 而事实相反。
        out += CollabPolicyRow("max_parallel_members", "单次扇出成员上限", if (n <= 0) "不限" else "$n 家")
    }
    return out
}

private fun onOffLabel(v: Boolean): String = if (v) "开启" else "关闭"

// ---- 未知事件 ----------------------------------------------------------------

data class UnknownEventLine(
    val eventId: String,
    val title: String,
    val subtitle: String,
    val statusLabel: String,
    /** 来源 agent 名(弹窗折叠行单独展示用);daemon 未给时为空串。 */
    val sourceAgent: String = "",
    /**
     * daemon 生成的一句话人话翻译(说"发生了什么")。
     * **null = 此 daemon 未提供**(旧版本 daemon 或旧台账条目),走 [degradedNote] 降级,
     * 手机端绝不替 daemon 编一句看起来像翻译的文案。
     */
    val summary: String? = null,
    /** summary 缺席时的固定降级说明(见 [UNKNOWN_SUMMARY_FALLBACK_NOTE]);有翻译时为 null。 */
    val degradedNote: String? = null,
    /** 相对时间(「3 分钟前」);无时间戳时为空串。 */
    val relativeTime: String = "",
)

/**
 * summary 缺席(旧 daemon / 旧台账条目)时的固定降级说明。
 *
 * 【为什么是这句】这是**事实陈述**:翻译没给出来,原因是手里的 daemon 版本没实现
 * 这个字段 —— 它不判断对错、不定性,所以既不写「未知」也不写「错误」
 * (AUDIT §3.3:机器无从裁决用户可见性,索性只陈述事实)。
 */
const val UNKNOWN_SUMMARY_FALLBACK_NOTE = "此 daemon 版本未提供翻译"

/**
 * 相对时间标签:刚刚 / N 分钟前 / N 小时前 / N 天前;超过一周回落绝对日期。
 *
 * 【为什么 now 由调用方注入】相对时间是 (unixMs, now) 的二元函数;把"现在"藏在
 * System.currentTimeMillis() 里,单测就只能断言"刚生成那一刻的值",跑慢一秒就翻车。
 */
fun relativeTimeLabel(unixMs: Long, nowMs: Long): String {
    if (unixMs <= 0) return ""
    val diff = nowMs - unixMs
    // 客户端时钟偏慢(时间戳在未来):回落绝对日期,不显示"负 N 分钟前"。
    if (diff < 0) return timeLabel(unixMs)
    return when {
        diff < 60_000L -> "刚刚"
        diff < 3_600_000L -> "${diff / 60_000L} 分钟前"
        diff < 86_400_000L -> "${diff / 3_600_000L} 小时前"
        diff < 7L * 86_400_000L -> "${diff / 86_400_000L} 天前"
        else -> timeLabel(unixMs)
    }
}

/** 单参重载: now 取当前时钟(仅 UI 直接调用;单测一律走双参纯函数)。 */
fun relativeTimeLabel(unixMs: Long): String =
    relativeTimeLabel(unixMs, System.currentTimeMillis())

/**
 * 安全分级的展示文案。
 *
 * 【为什么 UNKNOWN 不写"未知"】daemon 侧分类规则尚未定义,故安全分级恒为 UNKNOWN、
 * 如实标注。手机端照抄成"未知"会被读成"我们这边出了问题";事实是上游还没定义 ——
 * 两者要能分开(AUDIT §3.3)。
 */
fun safetyClassLabel(raw: String): String =
    if (raw.isBlank() || raw == "UNKNOWN") "尚未分类" else raw

/** 未知事件的处理状态 → 人话;未知取值原样返回。 */
fun unknownStatusLabel(raw: String): String = when (raw) {
    "NEW" -> "待处理"
    "INSPECTED" -> "已查看"
    "IGNORED" -> "已忽略"
    "RETRY_LATER" -> "稍后再看"
    "ESCALATED" -> "已升级"
    "NEEDS_NATIVE_SUPPORT" -> "待上游支持"
    else -> raw
}

/** 未知事件的归类 → 人话;未知取值原样返回。 */
fun unknownKindLabel(raw: String): String = when (raw) {
    "UNKNOWN_NOTIFICATION" -> "新通知"
    "UNKNOWN_REQUEST" -> "新请求"
    "UNKNOWN_RESPONSE" -> "新响应"
    else -> raw
}

/**
 * 列表项:标题用**原生方法名原文**(不翻译、不改写),只把归类/状态译成人话。
 * 方法名缺席时才退回归类文案 —— 空白标题会让整个列表看起来像坏了。
 *
 * summary 是 daemon 给的一句话人话,有就透出;缺席时降级为原文 + 固定说明
 * (见 [UNKNOWN_SUMMARY_FALLBACK_NOTE]),不造翻译。
 */
fun unknownEventLine(s: UnknownEventSummary): UnknownEventLine {
    val kindLabel = unknownKindLabel(s.normalizedKind)
    val title = s.sourceMethod.ifBlank { kindLabel }.ifBlank { s.eventId }
    val parts = listOf(s.sourceAgent, kindLabel, timeLabel(s.receivedAtUnixMs))
        .filter { it.isNotBlank() }
    val summary = s.summary.ifBlank { null }
    return UnknownEventLine(
        eventId = s.eventId,
        title = title,
        subtitle = parts.joinToString(" · ").ifBlank { "原生发来了一条新事件" },
        statusLabel = unknownStatusLabel(s.processingStatus),
        sourceAgent = s.sourceAgent,
        summary = summary,
        degradedNote = if (summary == null) UNKNOWN_SUMMARY_FALLBACK_NOTE else null,
        relativeTime = relativeTimeLabel(s.receivedAtUnixMs),
    )
}

/** 台账脚注:**null = 无需说明**(空台账,或一条都没丢 —— 不写"丢弃 0 条"这种噪音)。 */
fun unknownLedgerNote(total: Long, dropped: Long): String? {
    if (total <= 0) return null
    if (dropped <= 0) return "累计 $total 条"
    return "累计 $total 条,超出上限已丢弃 $dropped 条"
}

private fun timeLabel(unixMs: Long): String {
    if (unixMs <= 0) return ""
    return SimpleDateFormat("MM-dd HH:mm", Locale.getDefault()).format(Date(unixMs))
}

// ---- RPC 数据层 --------------------------------------------------------------

/** 加载相位(与 [CollabRolesLoadState] 同款三态,便于各处统一处理)。 */
enum class RuntimeLoadState { LOADING, READY, ERROR }

/**
 * 运行时控制面的数据层:**五组 RPC 各拉各的,一组失败不拖垮其余四组**。
 *
 * 【为什么不做成一个"全成功才算成功"的整体】这五块来自不同的波次(能力声明 /
 * 预算 / 未知事件 / 协同策略),旧 daemon 可能只有其中一部分。一块失败就清空全部,
 * 会让"这台 daemon 支持多少"这件事变成全有或全无 —— 而用户恰恰需要看到"哪块没接上"。
 * 因此逐块记错([sectionErrors]),UI 按块显示人话原因。
 *
 * 【提交必带 revision】Set 提交的是**完整策略**而非补丁(proto 明文警告过)。本层把
 * "Get → 改 → Set + CAS"这条纪律固化进 [saveBudget] / [saveCollabPolicy] 的签名里,
 * 调用方拿不到"只提交一个字段"这条路。
 */
object RuntimeControlState {
    /**
     * 消息透析弹窗开关。提升到共享状态而非 RuntimeSection 本地 remember:
     * 设置页内容根的推背模糊(paPushBack)要读它 —— 浮层开时整页内容后退一层,
     * 玻璃弹窗才有真磨砂。状态在本文件 = 数据层,弹窗 UI 在 RuntimeSection。
     */
    var dialysisSheetOpen by mutableStateOf(false)

    /**
     * 此 daemon 未提供内存管理(ListThreadMemories/ClearThreadMemory 回 UNIMPLEMENTED)。
     * 【为什么记住而不是每次试】确定的能力缺席要反映到入口上:灰显 + 一句事实陈述,
     * 而不是让用户每次点开都撞一次"必然失败"。连接到别的 daemon 时 reset 清零。
     */
    var memoriesUnsupported by mutableStateOf(false)
        private set

    const val SECTION_HOST = "host"
    const val SECTION_STATE = "state"
    const val SECTION_BUDGET = "budget"
    const val SECTION_COLLAB = "collab"
    const val SECTION_UNKNOWN = "unknown"

    var loadState by mutableStateOf(RuntimeLoadState.LOADING)
        private set

    /** 启动恢复报告;null = 旧 daemon 未报告(UI 隐藏横幅)。 */
    var recovery by mutableStateOf<HostRecoveryReport?>(null)
        private set
    var overview by mutableStateOf<RuntimeStateResponse?>(null)
        private set
    var budget by mutableStateOf<RuntimePolicyResponse?>(null)
        private set
    var collab by mutableStateOf<CollabPolicyResponse?>(null)
        private set
    var unknownEvents by mutableStateOf<List<UnknownEventSummary>>(emptyList())
        private set
    var unknownTotal by mutableStateOf(0L)
        private set
    var unknownDropped by mutableStateOf(0L)
        private set

    /** 分块失败原因(块 key → daemon 原文/传输原因);成功或未尝试的块不出现在这里。 */
    var sectionErrors by mutableStateOf<Map<String, String>>(emptyMap())
        private set

    /**
     * 记一条分块失败原因(保存/确认失败时由 UI 写回同一处呈现)。
     *
     * 【为什么失败也要归到"块"上】用户看到的是**一块内容**;原因只有落在同一块上,
     * 他才分得清"是预算没保存成"还是"这台电脑压根没连上"。
     */
    fun noteSectionError(section: String, message: String) {
        sectionErrors = sectionErrors + (section to message)
    }

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    private data class Fetched(
        val host: Result<HostInfo>,
        val state: Result<RuntimeStateResponse>,
        val budget: Result<RuntimePolicyResponse>,
        val collab: Result<CollabPolicyResponse>,
        val unknown: Result<ListUnknownEventsResponse>,
    )

    /** 五路并发拉取(同步等全部返回;每路各自超时在 stub 上设)。 */
    private suspend fun fetchAll(): Fetched = withContext(Dispatchers.IO) {
        coroutineScope {
            val host = async { runCatching { AgentConnection.hostService().getHostInfo(GetHostInfoRequest.getDefaultInstance()) } }
            val state = async { runCatching { AgentConnection.runtimeService().getRuntimeState(GetRuntimeStateRequest.getDefaultInstance()) } }
            val budgetJob = async { runCatching { AgentConnection.runtimeService().getRuntimePolicy(GetRuntimePolicyRequest.getDefaultInstance()) } }
            val collabJob = async { runCatching { AgentConnection.runtimeService().getCollabPolicy(GetCollabPolicyRequest.getDefaultInstance()) } }
            val unknown = async { runCatching { AgentConnection.runtimeService().listUnknownEvents(ListUnknownEventsRequest.getDefaultInstance()) } }
            Fetched(host.await(), state.await(), budgetJob.await(), collabJob.await(), unknown.await())
        }
    }

    /** 拉取全部五块(进页面时调一次;失败只更新状态,不抛)。 */
    fun load() {
        loadState = RuntimeLoadState.LOADING
        sectionErrors = emptyMap()
        scope.launch {
            val s = fetchAll()
            val errs = mutableMapOf<String, String>()
            s.host
                .onSuccess { info ->
                    // 旧 daemon 不带 recovery 字段 → 保持 null(整块隐藏),不摆假横幅。
                    recovery = if (info.hasRecovery()) info.recovery else null
                    // 顺手刷新全局 capabilities 快照:与本项目既有约定一致("GetHostInfo 成功后
                    // 由各调用方写入共享快照")。否则用户从旧 daemon 升上来时,入口要等下一轮探活。
                    AgentConnection.updateHostCapabilities(info.capabilitiesList)
                }
                .onFailure { errs[SECTION_HOST] = extractRpcMessage(it) }
            s.state
                .onSuccess { overview = it }
                .onFailure { errs[SECTION_STATE] = extractRpcMessage(it) }
            s.budget
                .onSuccess { budget = it }
                .onFailure { errs[SECTION_BUDGET] = extractRpcMessage(it) }
            s.collab
                .onSuccess { collab = it }
                .onFailure { errs[SECTION_COLLAB] = extractRpcMessage(it) }
            s.unknown
                .onSuccess {
                    unknownEvents = it.itemsList
                    unknownTotal = it.total
                    unknownDropped = it.dropped
                }
                .onFailure { errs[SECTION_UNKNOWN] = extractRpcMessage(it) }
            sectionErrors = errs
            // 总览都读不到时才算整体失败:它是这一页的骨架,其余块缺失由分块原因说明。
            loadState = if (overview == null) RuntimeLoadState.ERROR else RuntimeLoadState.READY
        }
    }

    /**
     * 提交预算改动:在**当前完整策略**上改(调用方只描述改哪一项),带 revision 做 CAS。
     * 成功返回 null,失败返回 daemon 原文。
     */
    suspend fun saveBudget(mutate: (pocketagent.v1.RuntimePolicy.Builder) -> Unit): String? {
        val cur = budget ?: return "还没读到当前预算,请先刷新再改"
        return try {
            val next = cur.policy.toBuilder().also(mutate).build()
            val resp = withContext(Dispatchers.IO) {
                AgentConnection.runtimeService().setRuntimePolicy(
                    SetRuntimePolicyRequest.newBuilder()
                        .setPolicy(next)
                        .setExpectedRevision(cur.revision)
                        .build()
                )
            }
            budget = resp
            sectionErrors = sectionErrors - SECTION_BUDGET
            null
        } catch (e: Exception) {
            extractRpcMessage(e)
        }
    }

    /** 提交协同策略改动(同 [saveBudget] 的纪律)。 */
    suspend fun saveCollabPolicy(mutate: (CollabPolicy.Builder) -> Unit): String? {
        val cur = collab ?: return "还没读到当前协同策略,请先刷新再改"
        return try {
            val next = cur.policy.toBuilder().also(mutate).build()
            val resp = withContext(Dispatchers.IO) {
                AgentConnection.runtimeService().setCollabPolicy(
                    SetCollabPolicyRequest.newBuilder()
                        .setPolicy(next)
                        .setExpectedRevision(cur.revision)
                        .build()
                )
            }
            collab = resp
            sectionErrors = sectionErrors - SECTION_COLLAB
            null
        } catch (e: Exception) {
            extractRpcMessage(e)
        }
    }

    /**
     * 控制中心写协同开关前的快照保障:协同策略响应缺席(还没走到「运行时」区段的
     * 读取,或上次读取失败)时先 Get 一次。读到的响应**写回共享真值 [collab]** ——
     * 两个界面仍共用同一份数,谁写的都算数。返回失败原因,null = 快照就绪。
     */
    suspend fun ensureCollabSnapshot(): String? {
        if (collab != null) return null
        return try {
            collab = withContext(Dispatchers.IO) {
                AgentConnection.runtimeService().getCollabPolicy(GetCollabPolicyRequest.getDefaultInstance())
            }
            null
        } catch (e: Exception) {
            extractRpcMessage(e)
        }
    }

    /** 控制中心写预算数值前的快照保障(与 [ensureCollabSnapshot] 同款理由)。 */
    suspend fun ensureBudgetSnapshot(): String? {
        if (budget != null) return null
        return try {
            budget = withContext(Dispatchers.IO) {
                AgentConnection.runtimeService().getRuntimePolicy(GetRuntimePolicyRequest.getDefaultInstance())
            }
            null
        } catch (e: Exception) {
            extractRpcMessage(e)
        }
    }

    /**
     * 控制中心写一条 agents 桌面偏好。**路由:既有 SetCodexSettings(单键补丁,
     * presence 纪律)** —— 请求里只 set 被改的那一项,其余字段缺席 = daemon 不改该项
     * (proto 钉死的语义);值照调用方原样,空串是"跟随桌面"的合法取值不是缺席。
     * 键→字段的映射与词表在 [applyControlAgentsSetting](纯函数,ControlPlaneLogicTest 钉死)。
     *
     * 【为什么成功后要双写本地】与设置页 writeCodex 同一条纪律:model/effort 还要进
     * AgentConnection.saveAgentDefaults(新会话的初始模型/强度从那里读)—— 控制中心
     * 改了模型而侧栏/新会话还按旧值开,就是"两边各写各的"事故。返回 null = 成功,
     * 失败返回 daemon 原文(与 saveBudget 同款签名)。
     */
    suspend fun saveCodexSetting(agentId: String, key: String, value: String): String? = try {
        val req = SetCodexSettingsRequest.newBuilder().setAgentId(agentId)
        check(applyControlAgentsSetting(req, key, value)) { "桌面偏好没有对应的写通道:$key" }
        val returned = withContext(Dispatchers.IO) {
            AgentConnection.modelService().setCodexSettings(req.build())
        }
        if (key == "model" || key == "effort") {
            AgentConnection.saveAgentDefaults(agentId, returned.model, returned.effort)
        }
        null
    } catch (e: Exception) {
        extractRpcMessage(e)
    }

    /** 确认一条未知事件;成功用 daemon 落库后的真值替换本地行(避免回显 stale 状态)。 */
    suspend fun acknowledge(eventId: String, action: String, note: String): String? = try {
        val item = withContext(Dispatchers.IO) {
            AgentConnection.runtimeService().acknowledgeUnknownEvent(
                AcknowledgeUnknownEventRequest.newBuilder()
                    .setEventId(eventId)
                    .setAction(action)
                    .setNote(note)
                    .build()
            ).item
        }
        unknownEvents = unknownEvents.map { if (it.eventId == item.eventId) item else it }
        null
    } catch (e: Exception) {
        extractRpcMessage(e)
    }

    /** 展开某条未知事件的原始载荷(列表页不需要背 payload,点开才拉)。 */
    suspend fun unknownEventDetail(eventId: String): Result<UnknownEventResponseOf> =
        runCatching {
            val r = withContext(Dispatchers.IO) {
                AgentConnection.runtimeService().getUnknownEvent(
                    GetUnknownEventRequest.newBuilder().setEventId(eventId).build()
                )
            }
            mapUnknownEventDetail(r)
        }

    /** 展开某 agent 的运行时资源用量(形态卡来自总览,用量要单独问)。 */
    suspend fun agentRuntime(agentId: String): Result<AgentRuntimeResponse> = runCatching {
        withContext(Dispatchers.IO) {
            AgentConnection.runtimeService().getAgentRuntime(
                GetAgentRuntimeRequest.newBuilder().setAgentId(agentId).build()
            )
        }
    }

    /**
     * 按会话列某 agent 的记忆清单。命中 UNIMPLEMENTED 时把 [memoriesUnsupported]
     * 记成事实 —— 入口据此灰显;其余失败照常走 Result.failure,由调用方呈现原因。
     */
    suspend fun listThreadMemories(agentId: String): Result<ListThreadMemoriesResponse> {
        val r = runCatching {
            withContext(Dispatchers.IO) {
                AgentConnection.runtimeService().listThreadMemories(
                    ListThreadMemoriesRequest.newBuilder().setAgentId(agentId).build()
                )
            }
        }
        if (r.isFailure && memoryMgmtUnsupported(r.exceptionOrNull() ?: return r)) {
            memoriesUnsupported = true
        }
        return r
    }

    /**
     * 清一个会话的 PocketAgent 侧记忆。**乐观更新在调用方(sheet)**:这里只发请求、
     * 回传 daemon 的剩余合计真值,成功/失败都原样交给调用方裁决 —— 回滚需要还原
     * 调用那一刻的行快照,只有调用方还留着它。
     */
    suspend fun clearThreadMemory(agentId: String, threadId: String): Result<ClearThreadMemoryResponse> =
        runCatching {
            withContext(Dispatchers.IO) {
                AgentConnection.runtimeService().clearThreadMemory(
                    ClearThreadMemoryRequest.newBuilder()
                        .setAgentId(agentId)
                        .setThreadId(threadId)
                        .build()
                )
            }
        }

    /** 退出页面:清空本地快照(下次进入重新拉,避免展示上一台机器的数据)。 */
    fun reset() {
        recovery = null; overview = null; budget = null; collab = null
        unknownEvents = emptyList(); unknownTotal = 0L; unknownDropped = 0L
        sectionErrors = emptyMap()
        memoriesUnsupported = false
        loadState = RuntimeLoadState.LOADING
    }

    /**
     * 异常 → 可透传原文:优先 daemon 设的 Status.description(干净人话,去掉 gRPC
     * 状态码前缀);空白退回 message;再空白退回类名(可核查的真实信息)。
     *
     * 【internal 而非 private】控制中心(ControlPlaneLogic)的失败呈现必须与本层
     * 同一条规则 —— daemon 的 description 原文优先,不各写一份"差不多"的提取逻辑。
     */
    internal fun extractRpcMessage(e: Throwable): String {
        val desc = Status.fromThrowable(e).description
        if (!desc.isNullOrBlank()) return desc
        return e.message?.takeIf { it.isNotBlank() } ?: e::class.java.simpleName
    }
}

/** 未知事件详情(展开后才拉的内容:原文三段 + 新 daemon 的透析表/详细解读)。 */
data class UnknownEventResponseOf(
    val rawJson: String,
    val decodedFieldsJson: String,
    val note: String,
    /** 字段级透析表;**空 = 此 daemon 未提供**(旧版本),UI 对应区块整块不渲染。 */
    val digest: List<UnknownEventDigestRow> = emptyList(),
    /** 详细人话解读;**null = 未提供**,同上。 */
    val translation: String? = null,
)

/** 透析表的一行:字段名 / 值类型 / 脱敏预览 / 含义人话。 */
data class UnknownEventDigestRow(
    val key: String,
    val kind: String,
    val preview: String,
    /** daemon 识别不出含义时为 null —— UI 只显示 key+预览,**不许猜一个含义填进去**。 */
    val meaning: String?,
)

/**
 * GetUnknownEvent 响应 → 详情模型(digest/translation 的唯一映射点)。
 *
 * 【为什么空串一律归空值】新 daemon 才给 digest/translation;旧 daemon 的响应里
 * 这两个字段读出来是空串/空列表。若在此处拿旧字段(raw_json 等)"拼一份差不多的",
 * 用户会以为透析内容来自 daemon —— 那是我们编的。空值让 UI 整块缺席,才是如实。
 */
fun mapUnknownEventDetail(r: UnknownEventResponse): UnknownEventResponseOf =
    UnknownEventResponseOf(
        rawJson = r.rawJson,
        decodedFieldsJson = r.decodedFieldsJson,
        note = r.note,
        digest = r.digestList.map {
            UnknownEventDigestRow(
                key = it.key,
                kind = it.kind,
                preview = it.preview,
                meaning = it.meaning.ifBlank { null },
            )
        },
        translation = r.translation.ifBlank { null },
    )
