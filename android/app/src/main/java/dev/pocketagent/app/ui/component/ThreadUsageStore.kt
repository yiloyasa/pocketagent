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
        val updatedAt: Long,
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
        if (contextWindow > 0) obj.put("cw", contextWindow)
        obj.put("at", System.currentTimeMillis())
        p.edit().putString(threadId, obj.toString()).apply()
    }

    fun load(threadId: String): Snapshot? {
        val p = prefs ?: return null
        val raw = p.getString(threadId, null) ?: return null
        return try {
            val o = org.json.JSONObject(raw)
            Snapshot(
                turn = jsonToUsage(o.optString("turn")),
                total = jsonToUsage(o.optString("total")),
                contextWindow = o.optLong("cw"),
                updatedAt = o.optLong("at"),
            )
        } catch (_: Exception) {
            null
        }
    }
}
