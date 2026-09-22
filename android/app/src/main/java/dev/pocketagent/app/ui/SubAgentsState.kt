package dev.pocketagent.app.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import dev.pocketagent.app.AgentConnection
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.GetThreadHistoryRequest
import pocketagent.v1.ListThreadsRequest
import pocketagent.v1.Thread as PThread
import pocketagent.v1.ThreadStatus
import java.util.concurrent.ConcurrentHashMap

/**
 * 子代理线程组(T-SUB,蓝本 2026-09-20-subagent-conversation-ui-plan.md §4/§5 +
 * S1 修正案 docs/plans/2026-09-21-subagent-ui-s1-revision.md):子代理=真线程,
 * 数据源=thread/list 按 parentThreadId 过滤 + 既有线程管线全复用。
 *
 * 【分工】
 *  - [SubAgentsLogic]:纯函数(过滤/状态推导/文案/兄弟导航),JVM 单测钉死;
 *  - [SubAgentsState]:RPC 拉取与缓存(thread/list 快照 + 子线程预览),Compose 状态。
 *
 * 【S1 修正案的两个真值依据】
 *  1. codex 子代理线程是真线程(thread/started 应答带 parentThreadId,见
 *     docs/verify/evidence/codex-subagent-s0.md),子对话=直接复用 chat/{id}
 *     路由打开子线程,流式/输入/审批/工具卡全功能,零 proto 改动;
 *  2. 内联卡数据=子线程 meta(标题/条数) + 运行状态(thread/list + TurnSession
 *     运行集),不是新帧。
 */

/** 子代理运行态(蓝本 §4.1 状态点六态;当前数据可推出的子集见 [SubAgentsLogic.runStateOf])。 */
enum class SubAgentRunState {
    QUEUED,
    RUNNING,
    WAITING_APPROVAL,
    DONE,
    FAILED,
    CANCELLED,
}

/**
 * 子代理线程的 UI 侧轻快照(内联卡与子对话来源行的唯一数据模型)。
 *
 * [parentThreadId] 非空 = 它是某条主线程的子代理线程 —— 这也是内联卡/来源行的
 * 唯一门控依据:关系未知(空串)时一律按普通会话处理,无卡不误报。
 */
data class SubAgentThreadMeta(
    val threadId: String = "",
    val parentThreadId: String = "",
    val title: String = "",
    val agentId: String = "",
    val messageCount: Int = 0,
    val createdAtMs: Long = 0L,
    val updatedAtMs: Long = 0L,
    /** daemon 落库 last_turn_error 非空(上一轮失败,如实显示失败态)。 */
    val lastTurnFailed: Boolean = false,
    /** Thread.Status = WAITING_APPROVAL(子线程也有自己的审批)。 */
    val waitingApproval: Boolean = false,
    /**
     * Thread.Status = RUNNING(daemon 落库快照视角的运行中,P-1):本机实时流
     * 集合(TurnSession)在父轮结束/进程重启后会看不到仍在后台跑的子线程,
     * 快照状态兜底 —— 不让后台运行中的子线程被误标"已完成"。
     */
    val statusRunning: Boolean = false,
)

/** 子代理分组视图:当前线程作为"子线程"回指它的父与兄弟(来源行/兄弟切换用)。 */
data class SubAgentChildGroup(
    val parentThreadId: String,
    val parentTitle: String,
    /** 同父的全部子线程(启动顺序);含当前线程自己。 */
    val siblings: List<SubAgentThreadMeta>,
)

/** 纯函数区(无 Compose/无 RPC,全部 JVM 可测)。 */
object SubAgentsLogic {

    /**
     * 取某条主线程下的子代理线程组(蓝本 §4.2:行间顺序=启动顺序,不做智能排序)。
     *
     * 【门控】parentThreadId 为空 = 没有任何关系信息,直接返回空组
     * (调用方据此不渲染卡片 —— "无子线程=无卡不误报")。
     */
    fun childrenOf(all: List<SubAgentThreadMeta>, parentThreadId: String): List<SubAgentThreadMeta> {
        if (parentThreadId.isBlank()) return emptyList()
        return all
            .filter { it.threadId.isNotBlank() && it.parentThreadId == parentThreadId }
            .sortedWith(compareBy({ it.createdAtMs }, { it.threadId }))
    }

    /**
     * 运行状态推导,优先级从高到低:
     *  1) 本机实时流集合(TurnSession.runningThreadIds,含恢复中)→ RUNNING;
     *  2) 子线程挂着自己的审批 → WAITING_APPROVAL;
     *  3) daemon 落库快照 Thread.Status=RUNNING → RUNNING(P-1:实时流集合看不到
     *     的后台运行 —— 父轮结束瞬间/他端触发 —— 快照兜底,不误标"已完成");
     *  4) daemon 落库的 last_turn_error → FAILED;
     *  5) 其余(IDLE/ARCHIVED)→ DONE。
     * QUEUED/CANCELLED 目前无数据来源,保留枚举位(状态点配色/文案已备)。
     */
    fun runStateOf(meta: SubAgentThreadMeta, liveRunningIds: Set<String>): SubAgentRunState = when {
        meta.threadId.isNotEmpty() && meta.threadId in liveRunningIds -> SubAgentRunState.RUNNING
        meta.waitingApproval -> SubAgentRunState.WAITING_APPROVAL
        meta.statusRunning -> SubAgentRunState.RUNNING
        meta.lastTurnFailed -> SubAgentRunState.FAILED
        else -> SubAgentRunState.DONE
    }

    /** 状态短文案(卡片右缘/迷你行尾)。 */
    fun statusLabel(state: SubAgentRunState): String = when (state) {
        SubAgentRunState.QUEUED -> "排队中"
        SubAgentRunState.RUNNING -> "运行中"
        SubAgentRunState.WAITING_APPROVAL -> "等待审批"
        SubAgentRunState.DONE -> "已完成"
        SubAgentRunState.FAILED -> "失败"
        SubAgentRunState.CANCELLED -> "已取消"
    }

    /**
     * 最后一条消息预览:压平全部空白(流式文本里的换行/多空格)→ 单行,
     * 超过 [maxChars] 截断加省略号。空输入返回空串(调用方不占位,不伪造)。
     */
    fun previewOf(raw: String?, maxChars: Int = 120): String {
        val flat = (raw ?: "").replace(WHITESPACE, " ").trim()
        if (flat.isEmpty()) return ""
        return if (flat.length <= maxChars) flat else flat.take(maxChars) + "…"
    }

    /**
     * 预览结果落账(P-2,纯函数 JVM 钉死;[SubAgentsState.requestPreview] 的
     * 成功/失败两路都走这里):
     *  - 失败(text=null)→ 移除占位键:允许卡片下次重组/重进时重试再拉,
     *    占位键不被失败永久吞掉(网络抖动不该让预览永远缺失);
     *  - 成功但空白 → 占位保留:该线程确实没有文本预览,防止反复空拉;
     *  - 成功非空 → 写缓存,返回 true(调用方据此自增 tick 驱动重组)。
     */
    internal fun settlePreview(
        cache: MutableMap<String, String>,
        threadId: String,
        text: String?,
    ): Boolean {
        if (text == null) {
            cache.remove(threadId)
            return false
        }
        if (text.isBlank()) return false
        cache[threadId] = text
        return true
    }

    /**
     * 组卡头部计数行(蓝本 §4.2 "3 个子智能体 · 1 运行 1 排队 1 完成"的现况版):
     * 只列非零段,全部终态时只剩"完成/失败"。空组返回空串。
     */
    fun groupSummaryLine(states: List<SubAgentRunState>): String {
        if (states.isEmpty()) return ""
        val running = states.count { it == SubAgentRunState.RUNNING }
        val waiting = states.count { it == SubAgentRunState.WAITING_APPROVAL }
        val failed = states.count { it == SubAgentRunState.FAILED }
        val done = states.count { it == SubAgentRunState.DONE || it == SubAgentRunState.CANCELLED }
        val parts = mutableListOf<String>()
        if (running > 0) parts += "$running 运行"
        if (waiting > 0) parts += "$waiting 等审批"
        if (failed > 0) parts += "$failed 失败"
        if (done > 0) parts += "$done 完成"
        return parts.joinToString(" ")
    }

    /** 组卡默认展开规则(蓝本 §4.2):运行中(含等审批/排队)默认展开,全部终态默认折叠。 */
    fun groupIsActive(states: List<SubAgentRunState>): Boolean = states.any {
        it == SubAgentRunState.RUNNING || it == SubAgentRunState.WAITING_APPROVAL || it == SubAgentRunState.QUEUED
    }

    /**
     * 组卡展开态(P-3,纯函数 JVM 钉死):用户手动表态优先;未表态(null)才落
     * [groupIsActive] 默认值。调用方把用户意图存进"键只含 children"的 remember
     * —— 运行集(liveRunIds)变化只重算 states,不重置用户意图,用户手动收起后
     * 父轮结束不自动弹开。
     */
    internal fun expandedOf(userIntent: Boolean?, states: List<SubAgentRunState>): Boolean =
        userIntent ?: groupIsActive(states)

    /** 兄弟计数(来源行 "◀ n/N ▶"):当前线程不在组里返回 (0,0),调用方隐藏计数。 */
    fun siblingPos(children: List<SubAgentThreadMeta>, currentId: String): Pair<Int, Int> {
        val idx = children.indexOfFirst { it.threadId == currentId }
        if (idx < 0) return 0 to 0
        return (idx + 1) to children.size
    }

    /** 兄弟导航:delta=-1 上一个 / +1 下一个;越界或当前不在组里返回 null(按钮置灰/隐藏)。 */
    fun siblingNav(children: List<SubAgentThreadMeta>, currentId: String, delta: Int): String? {
        val idx = children.indexOfFirst { it.threadId == currentId }
        if (idx < 0) return null
        val target = idx + delta
        if (target < 0 || target >= children.size) return null
        return children[target].threadId
    }

    private val WHITESPACE = Regex("\\s+")

    /**
     * pb.Thread → UI 快照。
     *
     * 【T-S1P 接通】契约已补 parent_thread_id(thread.proto 字段 22),daemon
     * 侧两路填充:codex 原生子线程经 thread/started 存根登记(server 层,
     * ParentThreadID = 父的本地线程 id);值如实透传 —— 空 = 关系未知/顶层
     * 会话,childrenOf 恒返空组,门控关闭不误报(行为与缺口期一致)。
     */
    internal fun toMeta(t: PThread): SubAgentThreadMeta = SubAgentThreadMeta(
        threadId = t.id,
        parentThreadId = t.parentThreadId,
        title = t.title.ifBlank { "子任务" },
        agentId = t.agentId,
        messageCount = t.messageCount,
        createdAtMs = t.createdAtUnixMs,
        updatedAtMs = t.updatedAtUnixMs,
        lastTurnFailed = t.lastTurnError.isNotBlank(),
        waitingApproval = t.status == ThreadStatus.THREAD_STATUS_WAITING_APPROVAL,
        statusRunning = t.status == ThreadStatus.THREAD_STATUS_RUNNING,
    )
}

/** 子代理线程组的拉取与缓存(单例,与 TurnSession 同款"页面只是视图"的分工)。 */
object SubAgentsState {

    /** thread/list 最近一次快照(全量);子组在渲染时按 parentThreadId 现筛。 */
    var threads by mutableStateOf<List<SubAgentThreadMeta>>(emptyList())
        private set

    /** 最后一次成功刷新时间戳(0=从未);参与运行集快照的 remember 键。 */
    var lastRefreshedAt by mutableStateOf(0L)
        private set

    /** 子线程"最后一条消息"预览缓存(threadId → 文本;空串=拉取中/确无文本预览,失败键移除允许重试,P-2)。 */
    private val previews = ConcurrentHashMap<String, String>()

    /** 预览落缓存时自增,驱动读缓存的卡片重组(值本身无语义)。 */
    var previewTick by mutableStateOf(0)
        private set

    /** app 级作用域:刷新不随页面销毁中断。 */
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    /**
     * 刷新线程快照(触发时机:打开主会话时顺带一次 + 本会话轮次结束后一次,
     * 触发点在 ChatScreen;失败静默沿用上次快照 —— 子代理卡是"有则显示",
     * 拉不到不能打扰主对话)。
     */
    fun refresh() {
        scope.launch {
            try {
                val list = withContext(Dispatchers.IO) {
                    AgentConnection.threadService()
                        .listThreads(
                            ListThreadsRequest.newBuilder()
                                .setIncludeArchived(false)
                                .build(),
                        )
                        .threadsList
                }
                threads = list.map { SubAgentsLogic.toMeta(it) }
                lastRefreshedAt = System.currentTimeMillis()
            } catch (_: Exception) {
                // 静默:保留上次快照
            }
        }
    }

    /**
     * 当前线程作为"子线程"的分组回指(子对话页来源行/兄弟切换的数据源)。
     * 不是任何主线程的子线程(或关系未知)→ null,调用方不显示来源行。
     */
    fun childGroupOf(threadId: String): SubAgentChildGroup? {
        val self = threads.firstOrNull { it.threadId == threadId } ?: return null
        if (self.parentThreadId.isBlank()) return null
        val parent = threads.firstOrNull { it.threadId == self.parentThreadId }
        return SubAgentChildGroup(
            parentThreadId = self.parentThreadId,
            parentTitle = parent?.title.orEmpty(),
            siblings = SubAgentsLogic.childrenOf(threads, self.parentThreadId),
        )
    }

    fun cachedPreview(threadId: String): String? = previews[threadId]

    /**
     * 忘掉某个会话的子代理预览缓存(删除会话时用)。
     *
     * 【为什么这也要清】previews 按 threadId 缓存**子代理对话的预览文本**。会话删了
     * 它还在,不只是白占一点内存:同一 id 的会话再建出来(重新导入、换机恢复)时会命中
     * 旧预览,让用户看到"上一世"的子代理内容 —— 而那属于已经删掉的那条会话。
     */
    fun forget(threadId: String) {
        if (threadId.isEmpty()) return
        previews.remove(threadId)
    }

    /**
     * 卡片可见时按需拉"最后一条消息"作预览(thread/list 不带正文预览,
     * 每个子线程一次 getThreadHistory 尾部小分页,结果缓存)。
     * 成功非空落缓存;空白=该线程确无文本预览(占位保留防反复空拉);
     * 失败移除占位键(P-2),预览行不占位、不伪造,重试交给下次可见。
     */
    fun requestPreview(threadId: String) {
        if (threadId.isBlank() || previews.containsKey(threadId)) return
        previews[threadId] = "" // 占位防重:拉到结果前不重复发请求
        scope.launch {
            try {
                val text = withContext(Dispatchers.IO) {
                    AgentConnection.threadService()
                        .getThreadHistory(
                            GetThreadHistoryRequest.newBuilder()
                                .setThreadId(threadId)
                                // daemon 语义:先按 limit 尾截(0=全部)——只要最后几条
                                .setLimit(SUB_PREVIEW_HISTORY_LIMIT)
                                .build(),
                        )
                        .messagesList
                        .lastOrNull()
                        ?.blocksList
                        ?.filter { it.hasText() }
                        ?.joinToString("\n") { it.text.text }
                        .orEmpty()
                }
                if (SubAgentsLogic.settlePreview(previews, threadId, text)) {
                    previewTick++
                }
            } catch (_: Exception) {
                // 预览拉不到就不显示,不打扰;但失败要移除占位键(P-2),
                // 否则 "" 占位永不清理 —— 卡片下次可见时允许重试再拉
                SubAgentsLogic.settlePreview(previews, threadId, null)
            }
        }
    }

    /** 预览小分页条数(daemon 按 limit 尾截,取尾即最近消息)。 */
    private const val SUB_PREVIEW_HISTORY_LIMIT = 8

    /** 长按快览弹层的小分页条数(快览只展示最近 5 条,拉 12 条留余量)。 */
    const val PEEK_HISTORY_LIMIT = 12
}
