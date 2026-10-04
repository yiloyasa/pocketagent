package dev.pocketagent.app.ui.history

/**
 * 历史全文搜索的状态机与纯决策层(Task 7,2026-10-04 会话选择与历史体验计划)。
 *
 * 升级前的浮层全文搜索只有"命中列表 + 失败静默降级为空"一种形态,五件事分不出来:
 * 本页命中 / 全会话命中 / 仅已下载命中 / 远端失败 / 确实无结果 —— 失败被伪装成
 * 空结果,用户以为"没有"其实是"没搜成"。本文件把搜索结果与定位决策全部抽成
 * 无 Composable、无 gRPC 依赖的纯数据 + 纯函数(模型类型在文件顶层,决策收在
 * [HistorySearchState] 对象里),JVM 单测钉死(HistorySearchStateTest),反转后的
 * 表现("失败变空结果 / 高亮错位 / 假跳到错误消息")都不报错,必须先钉后用。
 *
 * 【五态(与 plan Task 7 一一对应)】
 *  1. 本页结果      —— [HistorySearchStatus.Results],kind=[ResultsKind.PAGE_LOADED]:
 *     全部命中 from_loaded_history=true(命中确认已在手机时间线里,点一下就在眼前);
 *  2. 全会话结果    —— kind=[ResultsKind.FULL_THREAD]:daemon scope=full_thread 且
 *     from_loaded_history=false(daemon 全量扫描但无法确认已下载,点击需自动定位);
 *  3. 仅已下载      —— kind=[ResultsKind.DOWNLOADED_ONLY]:daemon scope=local_snapshot
 *     (降级口径,结果只覆盖本地快照/已下载片段);
 *  4. 远端失败      —— [HistorySearchStatus.RemoteError]:RPC 失败,reason 是给人看的
 *     人话;**绝不伪装成空结果**(Review Focus 红线);
 *  5. 无结果        —— [HistorySearchStatus.NoResults]:RPC 成功、零命中。
 * 另有 [HistorySearchStatus.Idle] = 尚未搜索(面板初始态),不计入五态。
 *
 * 【daemon 契约(只读,以 internal/server/thread_search.go 为准)】
 *  - ranges 坐标 = 命中消息**全部文本块按 "\n" 拼接后**的字符串上的 rune 半开区间
 *    [start, end);Android 侧用 [HistorySearchState.concatenatedSearchText] 复现同一
 *    拼法后再切高亮段;
 *  - from_loaded_history 当前恒 false(daemon 无法确认设备级已加载锚点)—— 客户端
 *    按"可能未下载"处理,凭 message_id 自动请求目标片段再跳;
 *  - message_id 空 = 旧数据极端窗口,照实为空:定位退回 sequence(兼容字段,daemon
 *    消息数组 0 基下标)兜底,不报错;
 *  - has_more 是权威分页位(daemon 扫完全部消息才落位),原样透传,客户端不猜。
 */

/** daemon MatchRange 的纯数据形态:拼接空间上的 rune 半开区间 [start, end)。 */
data class HistoryMatchSpan(val start: Int, val end: Int)

/**
 * daemon SearchThreadMessages 的一条命中(RPC 无关;proto 映射在仓库做)。
 *
 * 【定位键】[messageId] 非空时它是唯一主键(daemon 稳定消息 id,加载更早/
 * fork/rollback 都不漂移);空 = 旧数据,退 [sequence](daemon 全量消息数组
 * 0 基下标)兼容兜底 —— 见 [HistorySearchState.locateKeyOf]。sequence 与本地
 * LazyColumn 下标**不是同一坐标系**,两者之间不做任何算术换算。
 */
data class HistorySearchHit(
    val messageId: String,
    val turnId: String,
    val sequence: Int,
    val role: String,
    val snippet: String,
    val createdAtMs: Long,
    val ranges: List<HistoryMatchSpan> = emptyList(),
    val fromLoadedHistory: Boolean = false,
    /**
     * 该命中来源回包的 searched_messages(daemon 全量实扫条数;0 = 旧 daemon 未报)。
     * 缺口自动拉取的精确轮数依据:命中随身携带自己那次搜索的全量基准,身份索引
     * 没建/没总数时 [HistorySearchState.planHitLocate] 仍能精确算轮数,不退步进。
     */
    val searchedTotal: Int = 0,
)

/** 命中结果的呈现口径(五态里"有命中"的三分支)。 */
enum class ResultsKind(val label: String) {
    /** 本页结果:全部命中确认已加载(from_loaded_history=true),点一下就在眼前。 */
    PAGE_LOADED("本页命中"),

    /** 全会话结果:daemon 全量扫描,命中可能仍在未下载的历史里,点击自动定位。 */
    FULL_THREAD("全会话命中"),

    /** 仅已下载:daemon scope=local_snapshot,结果只覆盖本地已有内容。 */
    DOWNLOADED_ONLY("已下载命中"),
}

/** 搜索结果状态(面板临时视图状态;Idle=尚未搜索,不计入五态)。 */
sealed interface HistorySearchStatus {
    data object Idle : HistorySearchStatus

    /**
     * 有命中。[hasMore] 是 daemon 权威分页位,原样透传给 UI 提示"还有更多";
     * [scopeKnown]=false 表示旧 daemon 没报 scope,UI 需追加"(范围未知)"标注,
     * 不得臆断为全量。
     */
    data class Results(
        val query: String,
        val kind: ResultsKind,
        val hits: List<HistorySearchHit>,
        val hasMore: Boolean,
        val searchedMessages: Int,
        val scopeKnown: Boolean,
    ) : HistorySearchStatus

    /** RPC 成功、零命中:确实是"没有",与失败严格区分。 */
    data class NoResults(
        val query: String,
        val kind: ResultsKind,
        val scopeKnown: Boolean,
    ) : HistorySearchStatus

    /** RPC 失败:如实呈现人话原因,绝不伪装成空结果(Review Focus 红线)。 */
    data class RemoteError(
        val query: String,
        val reason: String,
    ) : HistorySearchStatus
}

/** 命中的定位键:message_id 优先;空退 sequence(兼容)。 */
sealed interface HistoryLocateKey {
    data class ById(val id: String) : HistoryLocateKey
    data class BySequence(val sequence: Int) : HistoryLocateKey
}

/**
 * 一条已加载 daemon 消息在本地时间线里的身份跨度:该消息映射出的 ChatItem
 * 连续段(历史映射一条 ThreadMessage ≥ 1 个 ChatItem,块拆分见
 * HistoryMessageMapper)。[sequenceInResponse] 是它在**本次回包**内的 0 基
 * 下标;daemon 全量数组坐标由 [HistoryIdentityIndex.firstDaemonIndex] 在查询期
 * 推算 —— 消息级 id 才是主键,下标只是兼容兜底。
 */
data class MessageSpan(
    val messageId: String,
    val turnId: String,
    val sequenceInResponse: Int,
    /** 该消息第一个 ChatItem 在 st.items 里的下标(不含列表头部偏移)。 */
    val itemStart: Int,
    val itemCount: Int,
    val role: String,
)

/**
 * 当前已加载历史的消息级身份索引(ChatScreen 在每次历史拉取接线点重建)。
 *
 * 【sequence 坐标口径】daemon 返回的是尾部(GetThreadHistory 幂等回尾),
 * 已加载的是全量数组的 [firstDaemonIndex, knownTotal) 段:knownTotal 取自
 * 最近一次全文搜索回包的 searched_messages;knownTotal=0(还没搜过/旧 daemon)
 * 且 !fullLoaded 时序号坐标不可信 —— sequence 兜底只在可信时参与,宁可走
 * "拉取再定位"也不拿错位下标假跳。
 *
 * @param fullLoaded 已加载的是否就是全量历史(historyTruncated==false)。
 */
data class HistoryIdentityIndex(
    val spans: List<MessageSpan> = emptyList(),
    val knownTotal: Int = 0,
    val fullLoaded: Boolean = false,
) {
    /** 已加载段在全量数组里的起始下标(knownTotal 未知时按"从 0 开始"尽力)。 */
    val firstDaemonIndex: Int
        get() = if (knownTotal > 0) (knownTotal - spans.size).coerceAtLeast(0) else 0

    /** sequence 坐标是否可信:总数已知,或已加载的就是全量。 */
    val sequenceTrusted: Boolean
        get() = knownTotal > 0 || fullLoaded

    fun findById(id: String): MessageSpan? =
        if (id.isBlank()) null else spans.firstOrNull { it.messageId == id }

    /** 按 daemon 全量数组下标找跨度;坐标不可信或越界回 null(不猜)。 */
    fun findByDaemonSequence(sequence: Int): MessageSpan? {
        if (!sequenceTrusted) return null
        return spans.getOrNull(sequence - firstDaemonIndex)
    }
}

/** 一次命中定位的裁定(纯函数;宿主按裁定执行滚动或拉取)。 */
sealed interface HistoryLocatePlan {
    /** 已在已加载内容里:直接滚到该 ChatItem(st.items 下标,不含列表头部偏移)。 */
    data class Jump(val itemIndex: Int) : HistoryLocatePlan

    /**
     * 缺口:需要拉更大的历史片段。[requestedRounds] 为本次请求轮数;
     * [exact]=true 表示按 knownTotal 精确算出、一次必中(除非消息已被
     * 回滚/分叉移除);false = 旧 daemon 无总数,按档位步进探索。
     */
    data class FetchTarget(val requestedRounds: Int, val exact: Boolean) : HistoryLocatePlan

    /** 有界探索到顶仍无进展:给人话,不再拉。 */
    data class Unresolvable(val reason: String) : HistoryLocatePlan
}

object HistorySearchState {

    /** daemon 搜索覆盖范围的两种口径字符串(proto SearchThreadMessagesResponse.scope)。 */
    const val SCOPE_FULL_THREAD = "full_thread"
    const val SCOPE_LOCAL_SNAPSHOT = "local_snapshot"

    /**
     * 自动定位拉取的轮数上限。精确覆盖所需轮数(roundsToCoverSequence)与步进扩张
     * 都钳到它:超大线程一次全量拉到手机既慢又占内存,到顶仍没找到就诚实失败。
     */
    const val MAX_LOCATE_ROUNDS = 200

    /**
     * 步进扩张(旧 daemon 无 searched_messages、只能按档位逐步放大)的最大尝试次数。
     * 有界探索:到次仍没有就给人话,不让"点一下命中卡"变成无底洞。
     */
    const val MAX_LOCATE_EXPANSIONS = 3

    /** 有命中时按 scope + from_loaded_history 归入三分支(纯函数,单测钉住)。 */
    fun resultsKindOf(scope: String, hits: List<HistorySearchHit>): ResultsKind = when {
        scope == SCOPE_LOCAL_SNAPSHOT -> ResultsKind.DOWNLOADED_ONLY
        hits.isNotEmpty() && hits.all { it.fromLoadedHistory } -> ResultsKind.PAGE_LOADED
        else -> ResultsKind.FULL_THREAD
    }

    /** daemon 是否报告了 scope(空 = 旧 daemon,UI 必须标"范围未知",不臆断)。 */
    fun scopeKnown(scope: String): Boolean = scope.isNotBlank()

    fun locateKeyOf(hit: HistorySearchHit): HistoryLocateKey =
        if (hit.messageId.isNotBlank()) HistoryLocateKey.ById(hit.messageId)
        else HistoryLocateKey.BySequence(hit.sequence)

    /**
     * 在已加载内容里定位命中(message_id 主键 → sequence 兼容),并做角色校验:
     * 命不中回 null(调用方走"文本对齐兜底 → 自动拉取"),角色不符 = 索引过期
     * (回滚/分叉换过列表),同样回 null —— 宁可多拉一次,不假跳。
     */
    fun locateLoadedSpan(hit: HistorySearchHit, identity: HistoryIdentityIndex?): MessageSpan? {
        if (identity == null || identity.spans.isEmpty()) return null
        val span = when (val key = locateKeyOf(hit)) {
            is HistoryLocateKey.ById -> identity.findById(key.id)
            is HistoryLocateKey.BySequence ->
                if (identity.sequenceTrusted) identity.findByDaemonSequence(key.sequence) else null
        } ?: return null
        return if (span.role == hit.role) span else null
    }

    /**
     * 覆盖 daemon 全量数组下标 [sequence] 所需的请求轮数(精确档)。
     * daemon 按轮截断、一轮至少一条消息,所以要 (total - sequence) 条消息 →
     * 请求同样的轮数必然覆盖(实际轮数只会更少,daemon 轮数不足时回全量);
     * 钳到 [MAX_LOCATE_ROUNDS] 防超大线程整段拉爆。
     */
    fun roundsToCoverSequence(sequence: Int, knownTotal: Int): Int =
        (knownTotal - sequence).coerceIn(1, MAX_LOCATE_ROUNDS)

    /** 步进扩张(步进档):当前累计轮数 + 一个设置档位(与「加载更早」同款步进)。 */
    fun nextExpansionRounds(current: Int, step: Int): Int = current + step.coerceAtLeast(1)

    /**
     * 命中定位的总裁定:已加载 → Jump;缺口 → 按精确/步进两档给出拉取计划。
     *
     * 【总数来源链(精确档的前提)】
     *  1. 身份索引 knownTotal(宿主随搜索回包刷新);
     *  2. 命中自带 searchedTotal(仓库从 searched_messages 戳记);
     *  3. messageId 非空 ⇒ 新 daemon(Task 6+,必报 searched_messages)⇒ 以
     *     「下一档累计轮数」为基准代理(total 未知但 daemon 一轮≥一条消息,
     *     请求 total-seq 轮仍必然覆盖到该下标;若真实全量更大,本轮拉取也会
     *     收窄缺口,宿主循环重裁,且有界);
     *  4. messageId 空 ⇒ 旧 daemon(必无 searched_messages)⇒ 只能步进探索。
     * 宿主(ChatScreen)负责执行拉取、落地、再调本函数重裁(循环有界:
     * exact 档按基准一轮必中,步进档由 MAX_LOCATE_EXPANSIONS 兜底)。
     */
    fun planHitLocate(
        hit: HistorySearchHit,
        identity: HistoryIdentityIndex?,
        currentRounds: Int,
        stepRounds: Int,
    ): HistoryLocatePlan {
        locateLoadedSpan(hit, identity)?.let { return HistoryLocatePlan.Jump(it.itemStart) }
        val known = identity?.knownTotal?.takeIf { it > 0 }
            ?: hit.searchedTotal.takeIf { it > 0 }
            ?: if (hit.messageId.isNotBlank()) nextExpansionRounds(currentRounds, stepRounds) else 0
        return if (known > 0) {
            HistoryLocatePlan.FetchTarget(requestedRounds = roundsToCoverSequence(hit.sequence, known), exact = true)
        } else {
            val next = nextExpansionRounds(currentRounds, stepRounds)
            if (next > MAX_LOCATE_ROUNDS) {
                HistoryLocatePlan.Unresolvable("已加载到会话开头,仍未找到该消息")
            } else {
                HistoryLocatePlan.FetchTarget(requestedRounds = next, exact = false)
            }
        }
    }

    /**
     * 按 message_id 去重(保序,保留首条);message_id 为空的旧数据按 sequence
     * 去重 —— 两条通道互不污染(同 id 与同 sequence 的命中各归各的键)。
     */
    fun dedupeHits(hits: List<HistorySearchHit>): List<HistorySearchHit> {
        val seenIds = HashSet<String>()
        val seenSeqs = HashSet<Int>()
        return hits.filter { h ->
            if (h.messageId.isNotBlank()) seenIds.add(h.messageId) else seenSeqs.add(h.sequence)
        }
    }

    // ==== ranges 高亮:拼接空间契约的 Android 复现 ====

    /**
     * 复现 daemon 的搜索正文拼法(契约:一条消息全部**文本块**按 "\n" 相连,
     * 思考块/工具参数不参与、不占坐标)。ranges 的 rune 坐标就在这个空间上,
     * 客户端任何高亮切分都必须先经过它,与 daemon 同一坐标系。
     */
    fun concatenatedSearchText(blocks: List<String>): String = blocks.joinToString("\n")

    /**
     * 把 daemon MatchRange(消息拼接空间 rune 半开区间)映射进命中卡摘录的高亮段。
     *
     * 【偏移推导两级】摘录 = 拼接全文上一段连续窗口(text[lo:hi],截断处以单个
     * "…" 装饰),把 range 平移进摘录需要窗口在拼接空间的起点:
     *  1. **折叠锚定(主)**:在剥掉首尾装饰后的正文里找 query 的首个大小写不敏感
     *     rune 命中 —— 窗口起点必不早于首个命中,故它就是 ranges[0],由此反推窗口
     *     起点,不硬编码 daemon 的摘录窗常量;
     *  2. **窗口规则回退(锚定失败时)**:query 本身跨块/多行时,命中段在摘录里
     *     隔着 "\n" 不连续,锚定必然落空 —— 此时回退 daemon 摘录窗口规则
     *     (镜像 thread_search.go 的 threadSearchContext=40:窗口 = 首命中前 40
     *     字符起,有"…"即窗口截到头之外),由 ranges[0].start 直接定位窗口起点。
     *     对 daemon 合规输入两级推导同值。
     * 落在摘录窗外的 range(第 2~5 处可能在窗外)丢弃不画。
     *
     * 【安全失败模式】range 全部对不齐(ranges 空的老 daemon / 窗外无重叠)一律
     * 回空列表 —— 不高亮,绝不画错位的高亮。
     *
     * @return 区间相对**原样 snippet**(含 "…" 装饰)的 UTF-16 下标闭区间
     *   (IntRange 含端点,可直接喂 AnnotatedString 切段)。
     */
    fun snippetHighlightSpans(
        snippet: String,
        ranges: List<HistoryMatchSpan>,
        query: String,
    ): List<IntRange> {
        if (snippet.isEmpty() || ranges.isEmpty()) return emptyList()
        val hasLead = snippet.startsWith(ELLIPSIS)
        val leadLen = if (hasLead) 1 else 0
        val hasTrail = snippet.length > leadLen && snippet.endsWith(ELLIPSIS)
        val body = snippet.substring(leadLen, snippet.length - (if (hasTrail) 1 else 0))
        if (body.isEmpty()) return emptyList()
        val first = ranges.first()
        val anchor = firstFoldIndexOf(body, query)
        val textOffset = when {
            anchor >= 0 -> {
                // body 起点 = 拼接空间的 (first.start - anchor) 号 rune
                first.start - anchor
            }
            query.isNotBlank() -> {
                // 回退:窗口起点 = 首命中前 SNIPPET_CONTEXT 个字符(daemon 同款钳制)
                (first.start - SNIPPET_CONTEXT).coerceAtLeast(0)
            }
            else -> return emptyList() // 空 query:解释不了 range,保守不画(安全失败模式)
        }
        val out = ArrayList<IntRange>(ranges.size)
        for (r in ranges) {
            val s = r.start - textOffset
            val e = r.end - textOffset
            if (s < 0 || e > body.length || s >= e) continue // 摘录窗外/异常:丢弃不画
            val utf = runeRangeToUtf16(body, s, e) ?: continue
            out += (utf.first + leadLen)..(utf.last + leadLen)
        }
        return out
    }

    private const val ELLIPSIS = "…"

    /**
     * daemon 摘录窗的上下文字符数(镜像 internal/server/thread_search.go 的
     * threadSearchContext=40;仅折叠锚定失败——跨块/多行 query——时参与推导)。
     */
    private const val SNIPPET_CONTEXT = 40

    /**
     * query 在 text 中的首个大小写不敏感命中,返回 **rune 下标**(未命中 -1)。
     * 折叠口径与 daemon 的 equalFoldRunes 对齐:先原值快路径,不等再逐字
     * EqualFold;两侧折叠规则存在极罕见差异时,失败模式是"不高亮"而非"画错位"。
     */
    private fun firstFoldIndexOf(text: String, query: String): Int {
        if (query.isEmpty()) return -1
        val tc = codePointsOf(text)
        val qc = codePointsOf(query)
        if (qc.isEmpty() || qc.size > tc.size) return -1
        outer@ for (i in 0..tc.size - qc.size) {
            for (j in qc.indices) {
                val a = tc[i + j]
                val b = qc[j]
                if (a != b && !Character.toString(a).equals(Character.toString(b), ignoreCase = true)) {
                    continue@outer
                }
            }
            return i
        }
        return -1
    }

    /** String → rune(Int 码点)数组(UTF-16 代理对合成一个码点)。 */
    private fun codePointsOf(s: String): IntArray {
        val out = IntArray(s.length)
        var n = 0
        var i = 0
        while (i < s.length) {
            val cp = s.codePointAt(i)
            out[n++] = cp
            i += Character.charCount(cp)
        }
        return if (n == out.size) out else out.copyOf(n)
    }

    /**
     * rune 半开区间 [startRune, endRune) → UTF-16 闭区间(代理对各占 2 个
     * UTF-16 码元);越界/倒置回 null。
     */
    private fun runeRangeToUtf16(text: String, startRune: Int, endRuneExclusive: Int): IntRange? {
        if (startRune < 0 || endRuneExclusive <= startRune) return null
        var rune = 0
        var start16 = -1
        var end16 = -1
        var i = 0
        while (i <= text.length) {
            if (rune == startRune) start16 = i
            if (rune == endRuneExclusive) {
                end16 = i
                break
            }
            if (i == text.length) break
            val cp = text.codePointAt(i)
            i += Character.charCount(cp)
            rune++
        }
        if (start16 < 0 || end16 < 0 || end16 <= start16) return null
        return start16 until end16
    }
}
