// UsageCache.kt —— 会话上下文用量的"最后一次已知值"缓存(断网可读)。
//
// 【为什么要单独落盘】daemon 只在流式过程中推用量,历史接口不带、proto 的 Thread
// 也没有用量字段。手机一旦离线(或自动断网),内存里若没有这份数据,上下文窗口卡
// 就只能显示空态 —— 而用户恰恰是在"想确认刚才用了多少"时才会去点它。把每个会话的
// 最后一次用量连同**时间戳**写进 SharedPreferences,离线打开卡片时先回显这些数字,
// 并如实标出"这是什么时候的",而不是假装数据是实时的。
//
// 【与 ThreadUsageStore 的关系(重要,别当死代码删掉)】
//   - `component/ThreadUsageStore.kt`(既有,由 TurnSession 调用):同样是 per-thread
//     落盘用量,目的是"重开会话时上下文窗口不归零",但它不带 `windowIsFallback`,
//     也没有"离线提示条"所需的时间语义。
//   - 本对象:额外保留 `windowIsFallback` + `updatedAt`,专供上下文窗口卡在**断网**时
//     独立回显与标注新鲜度。
//   - 【为什么暂不合并】两者字段高度重叠,但合并是对共享存储的重构;本轮是多人在改动
//     刚落地的时间点,动共享存储的风险大于收益。**已知技术债**,后续合并方向:以本对象
//     为准(保留 `windowIsFallback`/离线标签语义),让 `ThreadUsageStore` 退化成对它的
//     薄封装或直接删除。
//
// 纯客户端缓存:不改 daemon、不改 proto。序列化 / 陈旧判定 / 提示文案都是纯函数,
// 便于 JVM 单测(见 WorkflowAUsageCacheTest)。
package dev.pocketagent.app.ui.component

import android.content.Context
import android.content.SharedPreferences
import org.json.JSONObject
import pocketagent.v1.Usage
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Date
import java.util.Locale

/** 一个会话"最后一次已知用量"快照。 */
data class UsageSnapshot(
    val contextWindow: Long,
    val windowIsFallback: Boolean,
    val turnUsage: Usage?,
    val totalUsage: Usage?,
    val updatedAt: Long,
)

/** 超过该时长视为"陈旧":仍显示,但提示条改为标注日期而非时刻。 */
internal const val USAGE_STALE_AFTER_MS: Long = 7L * 24 * 60 * 60 * 1000

/** 是否陈旧(`updatedAt<=0` = 无缓存,不算陈旧)。纯函数。 */
internal fun isUsageStale(
    updatedAt: Long,
    now: Long,
    staleAfterMs: Long = USAGE_STALE_AFTER_MS,
): Boolean = updatedAt > 0L && now - updatedAt > staleAfterMs

/** 两个时间戳是否属于同一个自然日(用设备默认时区)。纯函数。 */
internal fun sameCalendarDay(a: Long, b: Long): Boolean {
    val ca = Calendar.getInstance().apply { timeInMillis = a }
    val cb = Calendar.getInstance().apply { timeInMillis = b }
    return ca.get(Calendar.YEAR) == cb.get(Calendar.YEAR) &&
        ca.get(Calendar.DAY_OF_YEAR) == cb.get(Calendar.DAY_OF_YEAR)
}

/**
 * 离线提示条文案。纯函数。
 *
 * 同一天只给时刻("最后更新 14:03");跨天补日期,避免把昨天的数据当刚刚的看。
 */
internal fun usageOfflineLabel(updatedAt: Long, now: Long): String {
    if (updatedAt <= 0L) return "离线 · 暂无缓存"
    val pattern = if (sameCalendarDay(updatedAt, now)) {
        "'最后更新' HH:mm"
    } else {
        "'最后更新' M月d日 HH:mm"
    }
    val fmt = SimpleDateFormat(pattern, Locale.getDefault())
    return "离线 · " + fmt.format(Date(updatedAt))
}

/** Usage -> JSON(保留全部原始数值)。纯函数。 */
internal fun usageToJson(u: Usage?): JSONObject? = u?.let {
    JSONObject()
        .put("in", it.inputTokens)
        .put("out", it.outputTokens)
        .put("cr", it.cacheReadTokens)
        .put("cw", it.cacheWriteTokens)
        .put("cost", it.costUsd)
        .put("reason", it.reasoningTokens)
}

/** JSON -> Usage;损坏时返回 null 而不是抛异常。纯函数。 */
internal fun usageFromJson(o: JSONObject?): Usage? {
    if (o == null) return null
    return try {
        Usage.newBuilder()
            .setInputTokens(o.optLong("in"))
            .setOutputTokens(o.optLong("out"))
            .setCacheReadTokens(o.optLong("cr"))
            .setCacheWriteTokens(o.optLong("cw"))
            .setCostUsd(o.optDouble("cost", 0.0))
            .setReasoningTokens(o.optLong("reason"))
            .build()
    } catch (_: Exception) {
        null
    }
}

/** 快照 -> 字符串。纯函数。 */
internal fun encodeUsageSnapshot(s: UsageSnapshot): String {
    val o = JSONObject()
        .put("cw", s.contextWindow)
        .put("fb", s.windowIsFallback)
        .put("at", s.updatedAt)
    usageToJson(s.turnUsage)?.let { o.put("turn", it) }
    usageToJson(s.totalUsage)?.let { o.put("total", it) }
    return o.toString()
}

/** 字符串 -> 快照;损坏时返回 null。纯函数。 */
internal fun decodeUsageSnapshot(raw: String?): UsageSnapshot? {
    if (raw.isNullOrEmpty()) return null
    return try {
        val o = JSONObject(raw)
        UsageSnapshot(
            contextWindow = o.optLong("cw"),
            windowIsFallback = o.optBoolean("fb", false),
            turnUsage = usageFromJson(o.optJSONObject("turn")),
            totalUsage = usageFromJson(o.optJSONObject("total")),
            updatedAt = o.optLong("at"),
        )
    } catch (_: Exception) {
        null
    }
}

/**
 * 按会话持久化"最后一次已知用量"。
 *
 * 与 `ThreadUsageStore` 的口径一致(都是 per-thread 落盘),但本对象额外保留
 * `windowIsFallback` 与"离线提示条"所需的语义,供上下文窗口卡在断网时独立回显。
 * 调用方需先 [init](幂等,重复调用无副作用)。
 */
object UsageCache {
    private const val PREFS = "pa_usage_card_cache"
    private var prefs: SharedPreferences? = null

    fun init(ctx: Context) {
        if (prefs == null) {
            prefs = ctx.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        }
    }

    fun save(threadId: String, snapshot: UsageSnapshot) {
        if (threadId.isBlank()) return
        prefs?.edit()?.putString(threadId, encodeUsageSnapshot(snapshot))?.apply()
    }

    fun load(threadId: String): UsageSnapshot? {
        val raw = prefs?.getString(threadId, null) ?: return null
        return decodeUsageSnapshot(raw)
    }

    fun remove(threadId: String) {
        prefs?.edit()?.remove(threadId)?.apply()
    }
}
