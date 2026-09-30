package dev.pocketagent.app.ui.component

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.nio.file.Files
import java.nio.file.StandardCopyOption

/** Cursor and visible text are committed together. A cursor without its matching
 * text is unsafe: the resumed stream would skip output the user never saw. */
internal object TurnCheckpointStore {

    // ---- LRU 上限(原 save() 内的硬编码,2026-09-30 提取为常量;值不变) ----
    // 最多保留 50 个、合计 8MB,超出按 lastModified 删最旧。OfflineStore 的
    // "checkpoint" 逻辑分区登记的 maxBytes 与此同源(见 OfflineStore.partitions)。
    const val MAX_CHECKPOINTS = 50
    const val MAX_TOTAL_BYTES = 8L * 1024 * 1024

    data class Checkpoint(
        val threadId: String,
        val messages: List<ThreadSnapshot.Msg>,
        val stream: String,
        val thinking: String,
        val sequence: Long,
        val active: Boolean,
        val queued: Int,
        val model: String,
        val effort: String?,
        val error: String?,
    )

    @Volatile private var root: File? = null
    fun init(context: Context) { root = PaPaths.turnCheckpointsDir(context.applicationContext).also { it.mkdirs() } }
    internal fun initForTest(dir: File) { root = dir.also { it.mkdirs() } }

    @Synchronized fun save(checkpoint: Checkpoint) {
        val dir = root ?: return
        runCatching {
            val payload = JSONObject().apply {
                put("thread", checkpoint.threadId)
                put("stream", checkpoint.stream)
                put("thinking", checkpoint.thinking)
                put("sequence", checkpoint.sequence)
                put("active", checkpoint.active)
                put("queued", checkpoint.queued)
                put("model", checkpoint.model)
                checkpoint.effort?.let { put("effort", it) }
                checkpoint.error?.let { put("error", it) }
                put("messages", JSONArray().apply {
                    checkpoint.messages.takeLast(60).forEach { msg ->
                        put(JSONObject().put("role", msg.role).put("text", msg.text))
                    }
                })
            }.toString()
            // Do not truncate a partial message and keep its later cursor.
            // Oversized checkpoints fall back to the separate text snapshot.
            val target = File(dir, filename(checkpoint.threadId))
            if (payload.toByteArray(Charsets.UTF_8).size > 512 * 1024) {
                target.delete()
                return
            }
            val temp = File(dir, target.name + ".tmp")
            temp.writeText(payload)
            Files.move(temp.toPath(), target.toPath(), StandardCopyOption.REPLACE_EXISTING)
            var bytes = 0L
            dir.listFiles { f -> f.extension == "json" }?.sortedByDescending { it.lastModified() }?.forEachIndexed { i, f ->
                bytes += f.length()
                if (i >= MAX_CHECKPOINTS || bytes > MAX_TOTAL_BYTES) f.delete()
            }
        }
    }

    @Synchronized fun load(threadId: String): Checkpoint? =
        root?.let { read(File(it, filename(threadId))) }

    /**
     * 删掉某个会话的轮次 checkpoint。
     *
     * 【为什么必须显式删】checkpoint 里存着**完整消息副本**(见 [Checkpoint.messages])
     * 与流式进度,是"删了会话还能翻出内容"的第二条路径。它此前只能靠 [save] 里的
     * LRU 被动淘汰(超过 50 个或累计 8MB 才动手)—— 那不是删除语义:用户删掉的会话
     * 可能几天后仍躺在磁盘上,而磁盘上留着的是**消息正文**。
     *
     * 幂等:不存在即已完成(删除的目标状态就是"没有")。
     */
    @Synchronized fun delete(threadId: String) {
        val dir = root ?: return
        if (threadId.isEmpty()) return
        runCatching { File(dir, filename(threadId)).delete() }
    }

    @Synchronized fun active(): List<Checkpoint> =
        root?.listFiles { f -> f.extension == "json" }?.mapNotNull(::read)?.filter { it.active }.orEmpty()

    /** 只读用量统计:字节数 + 条目数(缓存管理页聚合展示用,不改任何文件)。 */
    data class Usage(val bytes: Long, val entries: Int)

    /**
     * 只读用量:.json 正本的条数与字节数(.tmp 半成品不计,与 [save] 的 LRU 口径一致;
     * 超大被拒写的 checkpoint 不落盘,自然也不计)。
     */
    fun usage(): Usage {
        val d = root ?: return Usage(0L, 0)
        val files = d.listFiles { f -> f.isFile && f.extension == "json" } ?: return Usage(0L, 0)
        return Usage(files.sumOf { it.length() }, files.size)
    }

    private fun read(file: File): Checkpoint? = runCatching {
        if (!file.isFile || file.length() > 512 * 1024) return null
        val json = JSONObject(file.readText())
        val messages = json.getJSONArray("messages")
        Checkpoint(
            json.getString("thread"),
            (0 until messages.length()).map { i ->
                messages.getJSONObject(i).let { ThreadSnapshot.Msg(it.getString("role"), it.getString("text")) }
            },
            json.optString("stream"), json.optString("thinking"), json.getLong("sequence"),
            json.getBoolean("active"), json.optInt("queued"), json.optString("model"),
            if (json.has("effort")) json.getString("effort") else null,
            if (json.has("error")) json.getString("error") else null,
        )
    }.getOrNull()

    private fun filename(threadId: String): String =
        java.security.MessageDigest.getInstance("SHA-256").digest(threadId.toByteArray(Charsets.UTF_8))
            .joinToString("") { "%02x".format(it.toInt() and 255) } + ".json"
}
