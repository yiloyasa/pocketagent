package dev.pocketagent.app.ui.component

import android.content.Context
import android.content.SharedPreferences
import pocketagent.v1.Usage

/**
 * 会话上下文用量缓存 —— 解决"退出会话再进来,上下文窗口又显示 0%"。
 *
 * 【为什么要本地缓存】
 * daemon 只在流式过程中推 `UsageUpdate`(纯内存态),历史消息接口不带用量,
 * proto 的 Thread 也没有用量字段。于是每打开一次会话,面板都从零开始。
 * 这里把每个会话最后一次看到的用量与窗口大小落盘(像手机 App 的缓存一样),
 * 重开时先回显,新的上报到达后再覆盖。
 *
 * 纯客户端缓存:不改 daemon、不改 proto,老版本 daemon 同样受益。
 */
object ThreadUsageStore {
    private const val PREFS = "pa_thread_usage"
    private var prefs: SharedPreferences? = null

    /** 一个会话的用量快照。 */
    class Snapshot(
        val turn: Usage?,
        val total: Usage?,
        val contextWindow: Long,
        val contextWindowKnown: Boolean,
        val updatedAt: Long,
        /** 最近一次上下文压缩时间(0 = 未压缩过);压缩后 turn 被作废,靠它回显压缩状态。 */
        val compactedAt: Long = 0L,
    )

    fun init(ctx: Context) {
        if (prefs == null) {
            prefs = ctx.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        }
    }

    private fun usageToJson(u: Usage?): String? = u?.let {
        org.json.JSONObject()
            .put("in", it.inputTokens)
            .put("out", it.outputTokens)
            .put("cr", it.cacheReadTokens)
            .put("cw", it.cacheWriteTokens)
            .put("cost", it.costUsd)
            .put("reason", it.reasoningTokens)
            .toString()
    }

    private fun jsonToUsage(raw: String?): Usage? {
        if (raw.isNullOrEmpty()) return null
        return try {
            val o = org.json.JSONObject(raw)
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

    /**
     * 写入(每次收到用量上报时调用)。
     * 与已有快照**合并**:某个维度这次没带值(如只有窗口大小变化)不会把上次的值抹掉。
     */
    fun save(threadId: String, turn: Usage?, total: Usage?, contextWindow: Long) {
        if (threadId.isBlank()) return
        val p = prefs ?: return
        val obj = try {
            p.getString(threadId, null)?.let { org.json.JSONObject(it) } ?: org.json.JSONObject()
        } catch (_: Exception) {
            org.json.JSONObject()
        }
        usageToJson(turn)?.let { obj.put("turn", it) }
        usageToJson(total)?.let { obj.put("total", it) }
        if (contextWindow > 0) {
            obj.put("cw", contextWindow)
            obj.put("cwKnown", true)
        }
        obj.put("at", System.currentTimeMillis())
        p.edit().putString(threadId, obj.toString()).apply()
    }

    /**
     * 上下文压缩同步:作废已缓存的"本 turn 占用"(压缩前的数字已失真,可能超出
     * 实际窗口,绝不能重开会话时再回显),并记下压缩时间。
     *
     * 合并语义与 [save] 一致:total(全程累计)与窗口大小不受影响,只清 turn。
     * JSON 变换抽成纯函数 [markCompactedJson],便于 JVM 单测钉住。
     */
    fun markCompacted(threadId: String, nowMs: Long = System.currentTimeMillis()) {
        if (threadId.isBlank()) return
        val p = prefs ?: return
        val merged = markCompactedJson(p.getString(threadId, null), nowMs)
        p.edit().putString(threadId, merged).apply()
    }

    /** 回收:会话在电脑端被删后清掉残留的用量键。 */
    fun remove(threadId: String) {
        prefs?.edit()?.remove(threadId)?.apply()
    }

    /** 已缓存过用量的会话 id(供失效回收比对)。 */
    fun allIds(): Set<String> = prefs?.all?.keys ?: emptySet()

    fun load(threadId: String): Snapshot? {
        val p = prefs ?: return null
        val raw = p.getString(threadId, null) ?: return null
        return try {
            val o = org.json.JSONObject(raw)
            Snapshot(
                turn = jsonToUsage(o.optString("turn")),
                total = jsonToUsage(o.optString("total")),
                // 旧版本没有真实性标记，不能把旧的默认值重新当作 Codex 实测窗口。
                contextWindow = if (o.optBoolean("cwKnown", false)) o.optLong("cw") else 0L,
                contextWindowKnown = o.optBoolean("cwKnown", false) && o.optLong("cw") > 0,
                updatedAt = o.optLong("at"),
                compactedAt = o.optLong("compactAt"),
            )
        } catch (_: Exception) {
            null
        }
    }
}

/**
 * 压缩落盘的纯 JSON 变换:删掉 "turn"(陈旧占用),写入 "compactAt"。
 * 纯函数:同样的输入恒得同样的输出,不碰 SharedPreferences,可 JVM 单测。
 */
internal fun markCompactedJson(raw: String?, nowMs: Long): String {
    val o = try {
        raw?.let { org.json.JSONObject(it) } ?: org.json.JSONObject()
    } catch (_: Exception) {
        org.json.JSONObject()
    }
    o.remove("turn")
    o.put("compactAt", nowMs)
    o.put("at", nowMs)
    return o.toString()
}
