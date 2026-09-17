package dev.pocketagent.app.ui.component

import android.content.Context
import dev.pocketagent.app.AgentConnection
import org.json.JSONArray
import org.json.JSONObject
import java.io.File

/** "刚刚 / X 分钟前 / X 小时前 / X 天前" —— 离线快照提示里的相对时间。 */
fun agoText(atMs: Long): String {
    val d = (System.currentTimeMillis() - atMs).coerceAtLeast(0)
    return when {
        d < 60_000 -> "刚刚"
        d < 3_600_000 -> "${d / 60_000} 分钟前"
        d < 86_400_000 -> "${d / 3_600_000} 小时前"
        else -> "${d / 86_400_000} 天前"
    }
}

/**
 * 会话离线快照(审查 C3 / O1 / O2)。
 *
 * 【为什么要有它】会话列表与聊天历史原本**完全依赖 daemon**:手机一断网打开 app,
 * 列表空白、聊天页报错"无法加载会话历史" —— 对一个"遥控器"形态的 app 来说,
 * 这是离线体验最大的空洞。这里做**只读副本**:每次成功拉取后落一份到本地,
 * 断线时渲染快照并标注来源,恢复连接后照常走真数据。
 *
 * 【边界】
 * - 只读:没有任何写入口,不参与业务,删了不影响功能;
 * - 限额:每会话最近 [MAX_MSG_PER_THREAD] 条、文本 [MAX_BYTES_PER_THREAD],
 *   最多缓存 [MAX_THREADS] 个会话(LRU,按文件修改时间淘汰);
 * - 消息快照只留 role+text:图片/工具调用等无法离线还原的块跳过,宁可少显示,不能错显示。
 */
object ThreadSnapshot {

    /** 会话列表的一条快照。 */
    data class Row(
        val id: String,
        val title: String,
        val updatedAtUnixMs: Long,
        val workspace: String,
        val codexThreadId: String,
        val archived: Boolean,
        /**
         * 所属分组。**字段是后加的**,旧快照里没有这个键 —— 所以读写都走 opt*
         * (缺键读成空串),不带上它的话离线时"某个分组"那一格只能显示全量列表,
         * 用户会以为分组串了。
         */
        val sectionId: String = "",
    )

    /** 一条消息快照。 */
    data class Msg(val role: String, val text: String)

    private const val MAX_MSG_PER_THREAD = 60
    private const val MAX_BYTES_PER_THREAD = 200 * 1024
    private const val MAX_THREADS = 30
    private const val THREADS_FILE = "threads.json"

    private var dir: File? = null

    fun init(ctx: Context) {
        val d = File(ctx.filesDir, "pa_snap")
        runCatching { d.mkdirs() }
        dir = d
    }

    /** 仅供单测:直接指定快照目录,绕开 Context(android.jar 在 JVM 单测里不可用)。 */
    internal fun initForTest(d: File) {
        runCatching { d.mkdirs() }
        dir = d
    }

    // ------------------------------------------------------------ 会话列表

    fun saveThreads(rows: List<Row>) {
        val d = dir ?: return
        runCatching {
            val arr = JSONArray()
            rows.forEach { r ->
                arr.put(
                    JSONObject().apply {
                        put("id", r.id)
                        put("t", r.title)
                        put("u", r.updatedAtUnixMs)
                        put("w", r.workspace)
                        put("c", r.codexThreadId)
                        put("a", r.archived)
                        put("s", r.sectionId)
                    },
                )
            }
            File(d, THREADS_FILE).writeText(arr.toString())
        }
    }

    fun loadThreads(): List<Row> {
        val d = dir ?: return emptyList()
        return runCatching {
            val f = File(d, THREADS_FILE)
            if (!f.isFile) return@runCatching emptyList()
            val arr = JSONArray(f.readText())
            val out = mutableListOf<Row>()
            for (i in 0 until arr.length()) {
                val o = arr.getJSONObject(i)
                out += Row(
                    id = o.optString("id"),
                    title = o.optString("t"),
                    updatedAtUnixMs = o.optLong("u"),
                    workspace = o.optString("w"),
                    codexThreadId = o.optString("c"),
                    archived = o.optBoolean("a"),
                    sectionId = o.optString("s"),
                )
            }
            out
        }.getOrDefault(emptyList())
    }

    /** 会话列表快照的时间戳(毫秒),用于"离线快照 · X 分钟前";无快照返回 0。 */
    fun threadsSnapshotAt(): Long =
        dir?.let { File(it, THREADS_FILE).takeIf { f -> f.isFile }?.lastModified() } ?: 0L

    // ------------------------------------------------------------ 会话消息

    fun saveMessages(threadId: String, msgs: List<Msg>) {
        val d = dir ?: return
        if (threadId.isEmpty()) return
        runCatching {
            var used = 0
            val kept = mutableListOf<Msg>()
            // 从最新往回取:离线条数/体积超限时,丢的是最旧的
            for (m in msgs.asReversed()) {
                if (kept.size >= MAX_MSG_PER_THREAD) break
                val cost = m.text.length * 2 // UTF-16 粗估
                if (used + cost > MAX_BYTES_PER_THREAD) break
                used += cost
                kept += m
            }
            val arr = JSONArray()
            kept.asReversed().forEach { m ->
                arr.put(JSONObject().apply { put("r", m.role); put("t", m.text) })
            }
            File(d, msgFileName(threadId)).writeText(arr.toString())
            evict(d)
        }
    }

    /** 无快照返回 null(注意与"有快照但为空"区分)。 */
    fun loadMessages(threadId: String): List<Msg>? {
        val d = dir ?: return null
        if (threadId.isEmpty()) return null
        return runCatching {
            val f = File(d, msgFileName(threadId))
            if (!f.isFile) return@runCatching null
            val arr = JSONArray(f.readText())
            val out = mutableListOf<Msg>()
            for (i in 0 until arr.length()) {
                val o = arr.getJSONObject(i)
                out += Msg(o.optString("r"), o.optString("t"))
            }
            out
        }.getOrNull()
    }

    /** 该会话消息快照的时间戳(毫秒)。 */
    fun messagesAt(threadId: String): Long =
        dir?.let { File(it, msgFileName(threadId)).takeIf { f -> f.isFile }?.lastModified() } ?: 0L

    private fun msgFileName(threadId: String): String {
        val md = java.security.MessageDigest.getInstance("MD5")
        return md.digest(threadId.toByteArray(Charsets.UTF_8))
            .joinToString("") { "%02x".format(it.toInt() and 0xff) } + ".json"
    }

    /** LRU:会话快照超过 [MAX_THREADS] 个时删最旧的(总量不大,但要有上界)。 */
    private fun evict(d: File) {
        val files = d.listFiles { f -> f.isFile && f.name != THREADS_FILE } ?: return
        if (files.size <= MAX_THREADS) return
        files.sortedBy { it.lastModified() }
            .take(files.size - MAX_THREADS)
            .forEach { runCatching { it.delete() } }
    }

    /** 清空全部会话快照(缓存管理页用),返回释放的字节数。 */
    fun clear(): Long {
        val d = dir ?: return 0L
        val before = d.listFiles()?.sumOf { it.length() } ?: 0L
        d.listFiles()?.forEach { runCatching { it.delete() } }
        return before
    }

    /** 快照占用(字节)。 */
    fun usage(): Long = dir?.listFiles()?.sumOf { it.length() } ?: 0L

    // ------------------------------------------------------------ 失效偏好键回收(C4)

    /**
     * 会话在电脑端被删掉后,手机端的 `threadModel:<id>` 与用量键会永久残留。
     * 这里按"最后见到该会话的时间"延迟 [STALE_AFTER_MS] 回收 ——
     * 延迟是必要的:会话列表偶发拉不全(比如只拉到活跃、没拉归档),
     * 立刻删会把用户的模型选择抹掉。
     *
     * @param liveIds 本次刷新里真实存在的会话 id
     */
    fun pruneStalePrefs(liveIds: Set<String>) {
        val d = dir ?: return
        if (liveIds.isEmpty()) return // 空列表多半是"没拉到",不做任何删除
        runCatching {
            val f = File(d, SEEN_FILE)
            val seen = if (f.isFile) {
                org.json.JSONObject(f.readText()).let { o ->
                    o.keys().asSequence().associateWith { o.optLong(it) }
                }.toMutableMap()
            } else {
                mutableMapOf()
            }
            val now = System.currentTimeMillis()
            liveIds.filter { it.isNotEmpty() }.forEach { seen[it] = now }
            val stale = seen.filter { (id, at) -> id !in liveIds && now - at > STALE_AFTER_MS }
            if (stale.isNotEmpty()) {
                stale.keys.forEach { id ->
                    AgentConnection.removeThreadModel(id)
                    ThreadUsageStore.remove(id)
                    seen.remove(id)
                }
            }
            // 映射本身也要有上界:只保留最近见过的,防止它无限增长
            if (seen.size > MAX_SEEN) {
                seen.entries.sortedByDescending { it.value }
                    .take(MAX_SEEN)
                    .associate { it.key to it.value }
                    .let { kept ->
                        seen.clear()
                        seen.putAll(kept)
                    }
            }
            f.writeText(org.json.JSONObject(seen as Map<*, *>).toString())
        }
    }

    private const val STALE_AFTER_MS = 7 * 24 * 3600_000L
    private const val MAX_SEEN = 500
    private const val SEEN_FILE = "seen.json"
}
