package dev.pocketagent.app.ui.history

import io.grpc.Status
import io.grpc.StatusRuntimeException
import pocketagent.v1.GetThreadHistoryResponse
import pocketagent.v1.SearchThreadMessagesResponse
import pocketagent.v1.ThreadMessage

/**
 * 历史全文搜索的 RPC 仓库(Task 7):面板与逻辑层永远不直接碰 gRPC,宿主
 * (ChatScreen)只注入两条通道 —— SearchThreadMessages(搜索)与
 * GetThreadHistory(自动定位拉目标片段),其余(RPC 调用、proto → 纯模型映射、
 * 按 message_id 去重、五态归一、失败人话分类)都收在这里。
 *
 * 【失败语义】[search] 从不把失败伪装成空结果:任何异常都归一为
 * [HistorySearchStatus.RemoteError],reason 走 [failureReason] 的人话分类
 * (Review Focus 红线);协程取消原样重抛,不吞结构化取消。
 *
 * 【IO 线程】两条 RPC 通道的实现方负责调度(withContext(Dispatchers.IO)),
 * 仓库不做线程假设 —— 与 ChatScreen 既有 ThreadSearcher 接线同一纪律。
 */
class HistorySearchRepository(
    private val threadId: String,
    private val searchRpc: ThreadSearchRpc,
    private val historyRpc: ThreadHistoryRpc? = null,
) {
    /**
     * SearchThreadMessages 的 RPC 缝。抛异常 = 远端失败(仓库归一为 RemoteError);
     * 返回响应原样交给 [mapResponse]。
     */
    fun interface ThreadSearchRpc {
        suspend fun search(threadId: String, query: String, limit: Int): SearchThreadMessagesResponse
    }

    /** GetThreadHistory 的 RPC 缝(命中不在已加载内容里时,自动定位拉目标片段用)。 */
    fun interface ThreadHistoryRpc {
        suspend fun fetch(rounds: Int): GetThreadHistoryResponse
    }

    /**
     * 发起一次全文搜索:成功 → Results(三分支口径)/ NoResults;失败 →
     * RemoteError(人话)。永远不回 Idle(那是面板的初始态,不是搜索结果)。
     */
    suspend fun search(query: String, limit: Int = DEFAULT_LIMIT): HistorySearchStatus =
        try {
            mapResponse(searchRpc.search(threadId, query, limit), query)
        } catch (e: kotlinx.coroutines.CancellationException) {
            throw e // 结构化取消不吞:面板切查询/关浮层时旧任务该停就停
        } catch (e: Exception) {
            HistorySearchStatus.RemoteError(query, failureReason(e))
        }

    /**
     * 自动定位:按目标轮数拉历史片段(daemon 幂等回尾部),返回原始
     * ThreadMessage 列表;落地(只补头部/滚动锚点)由宿主做 —— 那是列表状态,
     * 归 ChatScreen 管。未注入历史通道时显式报错,不静默。
     */
    suspend fun fetchTargetMessages(rounds: Int): List<ThreadMessage> {
        val rpc = historyRpc ?: error("未注入历史拉取通道,无法自动定位")
        return rpc.fetch(rounds).messagesList
    }

    companion object {
        /** 单次搜索请求的命中上限(与浮层展示容量同源,daemon 侧另有 100 封顶)。 */
        const val DEFAULT_LIMIT = 30

        /**
         * proto 回包 → 五态(纯映射,单测可直接喂 proto builder)。
         * 命中先过 [HistorySearchState.dedupeHits](message_id 主键去重,空 ID 退
         * sequence);ranges 异常值按钳制处理、钳完仍无意义则丢弃(proto 契约:
         * 范围只用于高亮,绝不参与定位);has_more 是权威分页位,原样透传。
         */
        internal fun mapResponse(resp: SearchThreadMessagesResponse, query: String): HistorySearchStatus {
            val known = HistorySearchState.scopeKnown(resp.scope)
            val hits = HistorySearchState.dedupeHits(
                resp.hitsList.map { h ->
                    HistorySearchHit(
                        messageId = h.messageId,
                        turnId = h.turnId,
                        sequence = h.sequence,
                        role = h.role,
                        snippet = h.snippet,
                        createdAtMs = h.createdAtUnixMs,
                        ranges = h.rangesList.mapNotNull { r ->
                            val s = r.start.coerceAtLeast(0)
                            val e = r.end.coerceAtLeast(s)
                            if (e <= s) null else HistoryMatchSpan(s, e)
                        },
                        fromLoadedHistory = h.fromLoadedHistory,
                        searchedTotal = resp.searchedMessages,
                    )
                },
            )
            if (hits.isEmpty()) {
                return HistorySearchStatus.NoResults(
                    query = query,
                    kind = HistorySearchState.resultsKindOf(resp.scope, hits),
                    scopeKnown = known,
                )
            }
            return HistorySearchStatus.Results(
                query = query,
                kind = HistorySearchState.resultsKindOf(resp.scope, hits),
                hits = hits,
                hasMore = resp.hasMore,
                searchedMessages = resp.searchedMessages,
                scopeKnown = known,
            )
        }

        /**
         * RPC 失败 → 人话(状态如实呈现,不裸抛 status 字符串也不编造原因)。
         * 搜索与历史拉取共用:文案保持"连不上/已失效/不可用"级别的中性表述。
         */
        fun failureReason(e: Throwable): String {
            if (e is StatusRuntimeException) {
                return when (e.status.code) {
                    Status.Code.NOT_FOUND -> "会话在电脑端已不存在(可能被删除)"
                    Status.Code.INVALID_ARGUMENT -> "请求参数不可用"
                    Status.Code.UNAVAILABLE, Status.Code.DEADLINE_EXCEEDED -> "电脑端暂时连不上,请稍后再试"
                    Status.Code.UNAUTHENTICATED, Status.Code.PERMISSION_DENIED -> "设备配对已失效,请重新连接电脑"
                    else -> e.status.description?.takeIf { it.isNotBlank() } ?: "电脑端返回了错误"
                }
            }
            return when (e) {
                is java.io.IOException, is java.util.concurrent.TimeoutException -> "连接电脑失败,请检查网络后重试"
                else -> e.message?.takeIf { it.isNotBlank() }?.let { "请求失败:$it" } ?: "请求失败,请稍后重试"
            }
        }
    }
}
