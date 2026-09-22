package dev.pocketagent.app.ui.component

import android.content.Context
import dev.pocketagent.app.AgentConnection
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.nio.file.Files
import java.nio.file.StandardCopyOption

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
 * 会话离线快照(P1-8 · O1 档;沿袭审查 C3 / O2)。
 *
 * 【为什么要有它】会话列表与聊天历史原本**完全依赖 daemon**:手机一断网打开 app,
 * 列表空白、聊天页报错"无法加载会话历史" —— 对一个"遥控器"形态的 app 来说,
 * 这是离线体验最大的空洞。这里做**只读副本**:每次成功拉取后落一份到本地,
 * 断线时渲染快照并标注来源,恢复连接后照常走真数据。
 *
 * 【P1-8 限额口径】三层上限,超限一律淘汰"最旧先用"(LRU,顺序由
 * [OfflineStore.lruOrder] / [OfflineStore.selectForEviction] 纯函数给出,可单测):
 *  - 会话条数:最多 [MAX_THREADS] 个会话(50);
 *  - 单会话体积:每会话最近 [MAX_MSG_PER_THREAD] 条、文本合计 [MAX_BYTES_PER_THREAD](400KB);
 *  - 总体积:全部会话快照合计 [MAX_TOTAL_BYTES](5MB),超过按 LRU 淘汰最旧的会话文件。
 * 淘汰只针对"单会话快照文件";列表级文件([THREADS_FILE],线程顺序)与
 * [SEEN_FILE](失效键回收的"最后见过"表)不是快照数据,永远不参与淘汰。
 *
 * 【边界】
 * - 只读语义:快照不参与业务真相,删了不影响功能;
 * - 消息快照只留 role+text:图片/工具调用等无法离线还原的块跳过,宁可少显示,不能错显示;
 * - 写文件一律走原子替换([writeAtomic]:临时文件 + rename),进程被杀也不会留下半截 JSON。
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
        val agentId: String = "codex",
    )

    /** 一条消息快照。 */
    data class Msg(val role: String, val text: String)

    // ---- P1-8 限额口径(三层,见类注释) ----
    private const val MAX_MSG_PER_THREAD = 60
    private const val MAX_BYTES_PER_THREAD = 400 * 1024
    private const val MAX_THREADS = 50
    private const val MAX_TOTAL_BYTES = 5L * 1024 * 1024
    private const val THREADS_FILE = "threads.json"
    private const val SEEN_FILE = "seen.json"

    private var dir: File? = null

    fun init(ctx: Context) {
        val d = PaPaths.snapshotDir(ctx)
        runCatching { d.mkdirs() }
        dir = d
    }

    /** 仅供单测:直接指定快照目录,绕开 Context(android.jar 在 JVM 单测里不可用)。 */
    internal fun initForTest(d: File) {
        runCatching { d.mkdirs() }
        dir = d
    }

    // ------------------------------------------------------------ 会话列表

    /**
     * 落一份会话列表快照(P1-8 稳定 API,阶段 D 的 ChatScreen 集成也用它)。
     * 列表顺序 = 传入顺序(ThreadsScreen 传的是 daemon 原始全量),断线时照此顺序渲染。
     */
    fun saveThreadsSnapshot(rows: List<Row>) {
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
                        put("agent", r.agentId)
                    },
                )
            }
            writeAtomic(File(d, THREADS_FILE), arr.toString())
        }
    }

    /**
     * 读回会话列表快照(P1-8 稳定 API)。无快照 / 文件损坏一律返回空列表 ——
     * 对列表而言"读不出"与"没有"都意味着没东西可渲染,统一成空即可;
     * 损坏不崩,下次成功拉取会整体覆写,自愈。
     */
    fun readThreadsSnapshot(): List<Row> {
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
                    agentId = o.optString("agent", "codex"),
                )
            }
            out
        }.getOrDefault(emptyList())
    }

    /** 旧名兼容入口(既有单测在用),行为与 [saveThreadsSnapshot] 完全一致。 */
    fun saveThreads(rows: List<Row>) = saveThreadsSnapshot(rows)

    /** 旧名兼容入口(既有单测在用),行为与 [readThreadsSnapshot] 完全一致。 */
    fun loadThreads(): List<Row> = readThreadsSnapshot()

    /** 会话列表快照的时间戳(毫秒),用于"离线快照 · HH:mm";无快照返回 0。 */
    fun threadsSnapshotAt(): Long =
        dir?.let { File(it, THREADS_FILE).takeIf { f -> f.isFile }?.lastModified() } ?: 0L

    /**
     * 删除会话后,立刻把该 id 从列表快照里抹掉(原子重写 threads.json)。
     * 【为什么】乐观删除只改了内存里的 threads;离线快照 threads.json 仍留着这条,
     * 一旦之后断网走快照兜底,被删的会话又"复活"了 —— 用户实测"删完又出现"的
     * 表现层诱因之一就是它。这里让快照与内存一致;daemon 删除若最终失败,下次
     * 成功刷新会用真数据整体覆写,不会永久丢真会话(删成功则会随刷新一起消失)。
     */
    fun removeThreadFromSnapshot(id: String) {
        val d = dir ?: return
        if (id.isEmpty()) return
        runCatching {
            val rows = readThreadsSnapshot().filter { it.id != id }
            saveThreadsSnapshot(rows)
        }
    }

    // ------------------------------------------------------------ 会话消息(单线程)

    /**
     * 落一份单线程消息快照。
     *
     * 【与"历史加载轮数"的关系】调用方传入的 [msgs] 已经是请求端按用户设置
     * [dev.pocketagent.app.ui.HistoryRoundsLogic] 的 rounds 收窄后、从 daemon 拉回的
     * "最近 N 轮" —— 所以这份快照 = 最近 N 轮的本地副本,**不是全量**。
     *
     * 【覆盖而非合并】写入按 threadId 做原子替换(见 [writeAtomic]):同一会话新一次拉取会
     * **整体覆盖**旧快照,不会把两轮结果拼在一起。这保证快照永远等于"最近一次成功拉取的
     * 最近 N 轮",避免离线态出现重复或过期消息。
     *
     * 按三层限额裁剪:从最新往回取,条数/体积超限时丢的是最旧的 —— 离线时"最近的对话"
     * 永远比"完整的旧对话"有用。写完立即做两级淘汰(条数 + 总量),保证占用恒有上界。
     */
    @Synchronized
    fun saveMessages(threadId: String, msgs: List<Msg>) {
        val d = dir ?: return
        if (threadId.isEmpty()) return
        runCatching {
            var used = 0
            val kept = mutableListOf<Msg>()
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
            writeAtomic(File(d, msgFileName(threadId)), arr.toString())
            evict(d)
        }
    }

    /**
     * 读一份单线程消息快照(P1-8 稳定 API,阶段 D 的 ChatScreen 离线态用)。
     * 返回 null = 无快照(注意与"有快照但为空"区分);文件损坏也返回 null ——
     * 读到坏 JSON 不能崩,当成"没有快照"走在线/提示分支,下次保存会覆写自愈。
     */
    fun readThreadSnapshot(threadId: String): List<Msg>? {
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

    /** 旧名兼容入口(ChatScreen 在用,本阶段禁止改动该文件),行为与 [readThreadSnapshot] 一致。 */
    fun loadMessages(threadId: String): List<Msg>? = readThreadSnapshot(threadId)

    /**
     * 删掉某个会话的消息快照。
     *
     * 【为什么删除流程必须调它】[saveMessages] 按 threadId 落一份**独立文件**
     * (md5(threadId)+.json),它不在 threads.json 里 —— 所以
     * [removeThreadFromSnapshot] 清不到它。用户删了会话之后,离线兜底走
     * [readThreadSnapshot] 仍能把那条会话的对话整段读回来:"删了还能翻出内容"。
     * 只有把这层也清掉,删除在本地才算完整。
     *
     * 幂等:文件不存在不是错误 —— 删除的目标状态就是"没有",达成即成功。
     */
    @Synchronized
    fun removeMessages(threadId: String) {
        val d = dir ?: return
        if (threadId.isEmpty()) return
        runCatching { File(d, msgFileName(threadId)).delete() }
    }

    /** 该会话消息快照的时间戳(毫秒)。 */
    fun messagesAt(threadId: String): Long =
        dir?.let { File(it, msgFileName(threadId)).takeIf { f -> f.isFile }?.lastModified() } ?: 0L

    private fun msgFileName(threadId: String): String {
        val md = java.security.MessageDigest.getInstance("MD5")
        return md.digest(threadId.toByteArray(Charsets.UTF_8))
            .joinToString("") { "%02x".format(it.toInt() and 0xff) } + ".json"
    }

    // ------------------------------------------------------------ 淘汰与限额

    /**
     * 两级淘汰,写完每个会话快照后调用:
     *  1. 条数:会话快照文件超过 [MAX_THREADS] 个时删最旧的;
     *  2. 总量:合计超过 [MAX_TOTAL_BYTES] 时按 LRU 继续删,直到压回上限内。
     * 该删哪些由 [OfflineStore] 的纯函数给出(可脱离磁盘单测),这里只负责执行。
     * 列表级文件(threads.json / seen.json)与 .tmp 半成品不是快照数据,不在淘汰范围。
     */
    private fun evict(d: File) {
        val files = d.listFiles { f ->
            f.isFile && f.name != THREADS_FILE && f.name != SEEN_FILE && !f.name.endsWith(".tmp")
        } ?: return
        if (files.isEmpty()) return
        val entries = files.map { OfflineStore.CacheEntry(it.name, it.length(), it.lastModified()) }
        // 条数上限:固定删到只剩 MAX_THREADS 个
        val doomed = mutableSetOf<String>()
        if (files.size > MAX_THREADS) {
            OfflineStore.lruOrder(entries).take(files.size - MAX_THREADS)
                .forEach { e -> doomed += e.name }
        }
        // 总量上限:在条数淘汰的基础上继续按 LRU 删,直到 ≤ 5MB
        val surviving = entries.filter { it.name !in doomed }
        OfflineStore.selectForEviction(surviving, MAX_TOTAL_BYTES)
            .forEach { e -> doomed += e.name }
        if (doomed.isEmpty()) return
        for (f in files) if (f.name in doomed) runCatching { f.delete() }
    }

    /**
     * 原子写:先写同名 .tmp 临时文件,再 rename 成正本 —— 进程中途被杀最多留一个
     * .tmp,正本要么是旧完整内容要么是新完整内容,不会出现半截 JSON。
     * rename 用 java.nio(Windows 上 File.renameTo 不肯覆盖已存在的目标,会静默失败),
     * 失败再兜底"先删后 rename"。
     */
    private fun writeAtomic(target: File, text: String) {
        val tmp = File(target.parentFile, target.name + ".tmp")
        runCatching {
            tmp.writeText(text)
            Files.move(tmp.toPath(), target.toPath(), StandardCopyOption.REPLACE_EXISTING)
        }.onFailure {
            runCatching { target.delete() }
            if (!runCatching { tmp.renameTo(target) }.getOrDefault(false)) {
                runCatching { tmp.delete() }
            }
        }
    }

    // ------------------------------------------------------------ 清理与统计

    /**
     * 清空全部会话快照(P1-8 稳定 API;缓存管理页与单测用),返回释放的字节数。
     * 连 seen.json 一起清是安全的:它只是"最后见过"表,清掉后失效键回收从零重新计时。
     */
    fun clearAll(): Long {
        val d = dir ?: return 0L
        val before = d.listFiles()?.sumOf { it.length() } ?: 0L
        d.listFiles()?.forEach { runCatching { it.delete() } }
        return before
    }

    /** 旧名兼容入口(设置页「缓存管理」在用),行为与 [clearAll] 完全一致。 */
    fun clear(): Long = clearAll()

    /** 快照占用(字节,含列表级文件)。 */
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
                runCatching { JSONObject(f.readText()) }.getOrNull()?.let { o ->
                    o.keys().asSequence().associateWith { o.optLong(it) }
                }?.toMutableMap() ?: mutableMapOf()
            } else {
                mutableMapOf()
            }
            val now = System.currentTimeMillis()
            liveIds.filter { it.isNotEmpty() }.forEach { seen[it] = now }
            val stale = seen.filter { (id, at) -> id !in liveIds && now - at > STALE_AFTER_MS }
            if (stale.isNotEmpty()) {
                stale.keys.forEach { id ->
                    AgentConnection.removeThreadModel(id)
                    AgentConnection.removeThreadEffort(id)
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
            writeAtomic(f, JSONObject(seen as Map<*, *>).toString())
        }
    }

    private const val STALE_AFTER_MS = 7 * 24 * 3600_000L
    private const val MAX_SEEN = 500
}
