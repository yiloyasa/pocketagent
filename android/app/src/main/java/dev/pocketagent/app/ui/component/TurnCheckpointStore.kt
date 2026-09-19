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
    fun init(context: Context) { root = File(context.applicationContext.filesDir, "turn-checkpoints").also { it.mkdirs() } }
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
                if (i >= 50 || bytes > 8 * 1024 * 1024) f.delete()
            }
        }
    }

    @Synchronized fun load(threadId: String): Checkpoint? =
        root?.let { read(File(it, filename(threadId))) }

    @Synchronized fun active(): List<Checkpoint> =
        root?.listFiles { f -> f.extension == "json" }?.mapNotNull(::read)?.filter { it.active }.orEmpty()

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
