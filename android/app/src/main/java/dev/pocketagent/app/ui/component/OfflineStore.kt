package dev.pocketagent.app.ui.component

import android.content.Context
import java.io.File
import java.security.MessageDigest

/**
 * 离线存储统一入口(cacheDir 下的一块自留地)。
 *
 * 【它解决什么】
 *  · **可扩展的缓存分区注册表** —— 新增一类缓存(网页快照/缩略图/…)只加一行 [partitions];
 *  · **可单测的纯函数** —— LRU 顺序、淘汰集合、字节统计、陈旧判定、缓存键全部是纯逻辑,
 *    不碰磁盘也能验证(历史坑:`f.delete()` 之后 `f.length()` 恒 0,导致算出"没占用");
 *  · **网页快照缓存** —— 断网时同一个 URL 命中本地快照就直接渲染,不再白屏等超时。
 *
 * 【边界】只动自己的子目录(pa_offline/…),不碰 cacheDir 里的其它内容(WebView 等)。
 */
object OfflineStore {

    /**
     * 一个缓存分区(扩展点)。新增一类缓存 = 在 [partitions] 里加一行。
     * @param maxBytes  该分区容量上限,超限按 LRU 淘汰;
     * @param clearable false = 用户显式下载内容,「清理缓存」**不得**删除。
     * @param ttlMs     生命周期:距上次使用超过该时长视为陈旧,启动清扫时删除;
     *                  0 = 永不过期(只受 LRU 容量约束)。
     */
    data class Partition(
        val id: String,
        val label: String,
        val subDir: String,
        val maxBytes: Long,
        val clearable: Boolean,
        val ttlMs: Long = 0L,
    )

    val partitions: List<Partition> = listOf(
        // 网页快照:30 天没用过的快照视为陈旧,启动时清掉 —— 离线兜底只看"最近看过的",
        // 更老的大概率页面已更新,留着既占空间又可能展示过期内容。
        Partition("url", "网页快照", "pa_url", 80L * 1024 * 1024, true, 30L * 24 * 60 * 60 * 1000),
    )

    /** 一条缓存条目(不含文件句柄,便于纯函数单测)。 */
    data class CacheEntry(val name: String, val size: Long, val lastUsedMs: Long)

    // -------------------------------------------------------------- 纯函数(可单测)

    /** 一个分区的当前占用(调用方先取好每个条目的字节数)。 */
    fun bytesUsed(entries: List<CacheEntry>): Long = entries.sumOf { it.size }

    /** 按"最近最少使用"升序(最旧在最前);lastUsedMs 相同则按名字稳定排序。 */
    fun lruOrder(entries: List<CacheEntry>): List<CacheEntry> =
        entries.sortedWith(compareBy({ it.lastUsedMs }, { it.name }))

    /**
     * 为把总量压回 [maxBytes] 内,需要淘汰的条目(最旧优先,直到剩余 ≤ 上限)。
     * 纯函数:同样的输入恒得同样的输出,不碰磁盘。
     */
    fun selectForEviction(entries: List<CacheEntry>, maxBytes: Long): List<CacheEntry> {
        var total = bytesUsed(entries)
        if (total <= maxBytes) return emptyList()
        val doomed = mutableListOf<CacheEntry>()
        for (e in lruOrder(entries)) {
            if (total <= maxBytes) break
            doomed += e
            total -= e.size
        }
        return doomed
    }

    /** 陈旧判定:距上次使用超过 [ttlMs] 视为陈旧(ttlMs <= 0 → 永不过期)。 */
    fun isStale(lastUsedMs: Long, nowMs: Long, ttlMs: Long): Boolean =
        ttlMs > 0 && nowMs - lastUsedMs > ttlMs

    /** 缓存键:md5(原始字符串),把任意长度/含特殊字符的键映射成安全文件名(32 位十六进制)。 */
    fun cacheKey(raw: String): String {
        val md = MessageDigest.getInstance("MD5")
        return md.digest(raw.toByteArray(Charsets.UTF_8))
            .joinToString("") { "%02x".format(it.toInt() and 0xff) }
    }

    // -------------------------------------------------------------- 目录与读写

    private const val ROOT_DIR = "pa_offline"

    private var cacheRoot: File? = null

    /**
     * 当前生效的分区表。生产恒等于 [partitions];单测可注入自定义分区来覆盖
     * `clearable=false` 等分支(见 [initForTest])。
     */
    private var registry: List<Partition> = partitions

    fun init(ctx: Context) {
        val root = File(ctx.cacheDir, ROOT_DIR)
        runCatching { root.mkdirs() }
        cacheRoot = root
        registry = partitions
        registry.forEach { runCatching { File(root, it.subDir).mkdirs() } }
    }

    /**
     * 仅供单测:直接指定根目录,绕开 Context(android.jar 在 JVM 单测里不可用)。
     * [parts] 默认用生产 [partitions],测试可注入(例如一个 clearable=false 的分区)。
     */
    internal fun initForTest(root: File, parts: List<Partition> = partitions) {
        runCatching { root.mkdirs() }
        cacheRoot = root
        registry = parts
        parts.forEach { runCatching { File(root, it.subDir).mkdirs() } }
    }

    /** 某分区的目录(未初始化或 id 未知返回 null)。 */
    fun dirFor(id: String): File? {
        val root = cacheRoot ?: return null
        val p = registry.firstOrNull { it.id == id } ?: return null
        return File(root, p.subDir)
    }

    /** pa_offline 的总占用(字节)。 */
    fun usage(): Long = cacheRoot?.let { dirSize(it) } ?: 0L

    /**
     * 清空某个**可清理**分区,返回释放字节数。
     * 【契约】`clearable=false` 的分区(用户显式下载内容)以及未知 id 一律返回 0、不动任何文件 ——
     * 「清理缓存」绝不能删用户已下载的内容。
     */
    fun clear(id: String): Long {
        val p = registry.firstOrNull { it.id == id } ?: return 0L
        if (!p.clearable) return 0L
        val d = dirFor(id) ?: return 0L
        val before = dirSize(d)
        d.listFiles()?.forEach { runCatching { it.deleteRecursively() } }
        return before
    }

    /** 对某分区执行 LRU 淘汰(用纯函数算出该删哪些,再删)。 */
    fun evictIfNeeded(id: String) {
        val p = registry.firstOrNull { it.id == id } ?: return
        val d = dirFor(id) ?: return
        val files = d.listFiles() ?: return
        // 长度必须在删除前取:delete 之后 length() 恒为 0(历史事故:会清空整个目录)
        val entries = files.map { CacheEntry(it.name, it.length(), it.lastModified()) }
        val doomed = selectForEviction(entries, p.maxBytes).map { it.name }.toSet()
        if (doomed.isEmpty()) return
        for (f in files) if (f.name in doomed) runCatching { f.delete() }
    }

    /**
     * 生命周期清扫:删掉某分区里"距上次使用超过 ttlMs"的陈旧条目,返回删除数。
     * ttlMs <= 0 的分区永不过期,直接返回 0。启动时由 [sweepAllStale] 统一调用,
     * 不做后台定时器 —— 遥控器形态下,启动清扫已经足够把占用压回合理区间。
     */
    fun sweepStale(id: String, nowMs: Long = System.currentTimeMillis()): Int {
        val p = registry.firstOrNull { it.id == id } ?: return 0
        if (p.ttlMs <= 0L) return 0
        val d = dirFor(id) ?: return 0
        val files = d.listFiles() ?: return 0
        var removed = 0
        for (f in files) {
            if (f.isFile && isStale(f.lastModified(), nowMs, p.ttlMs)) {
                if (runCatching { f.delete() }.getOrDefault(false)) removed++
            }
        }
        return removed
    }

    /** 启动时对全部有生命周期的分区做一次陈旧清扫,返回总删除数。 */
    fun sweepAllStale(nowMs: Long = System.currentTimeMillis()): Int =
        registry.sumOf { sweepStale(it.id, nowMs) }

    private fun dirSize(dir: File): Long {
        val files = dir.listFiles() ?: return 0L
        var total = 0L
        for (f in files) total += if (f.isDirectory) dirSize(f) else f.length()
        return total
    }

    // -------------------------------------------------------------- 网页快照缓存(离线浏览)

    /** 一份网页快照:原始字节 + MIME + 建议文件名。 */
    data class UrlSnapshot(val bytes: ByteArray, val mime: String, val name: String) {
        // ByteArray 的 data class equals 是引用比较,这里补上内容比较,避免"看起来相等其实不等"
        override fun equals(other: Any?): Boolean {
            if (this === other) return true
            if (other !is UrlSnapshot) return false
            return mime == other.mime && name == other.name && bytes.contentEquals(other.bytes)
        }

        override fun hashCode(): Int {
            var r = mime.hashCode()
            r = 31 * r + name.hashCode()
            r = 31 * r + bytes.contentHashCode()
            return r
        }
    }

    /** 存一份网页快照。文件名 = md5(url) + "." + hex(mime) + "." + hex(name)(mime/name 编码进文件名,免 sidecar)。 */
    fun putUrl(url: String, bytes: ByteArray, mime: String, name: String) {
        val d = dirFor("url") ?: return
        if (url.isBlank() || bytes.isEmpty()) return
        runCatching {
            val key = cacheKey(url)
            File(d, "$key.${hex(mime)}.${hex(name)}").writeBytes(bytes)
            evictIfNeeded("url")
        }
    }

    /** 取一份网页快照;命中即 touch(参与 LRU),未命中返回 null。 */
    fun getUrl(url: String): UrlSnapshot? {
        val d = dirFor("url") ?: return null
        if (url.isBlank()) return null
        return runCatching {
            val key = cacheKey(url)
            val hit = d.listFiles { f -> f.isFile && f.name.startsWith("$key.") }
                ?.maxByOrNull { it.lastModified() } ?: return@runCatching null
            val rest = hit.name.substringAfter("$key.") // "<hexMime>.<hexName>"
            val mime = unhex(rest.substringBefore('.'))
            val name = unhex(rest.substringAfter('.', ""))
            val bytes = hit.readBytes()
            runCatching { hit.setLastModified(System.currentTimeMillis()) }
            UrlSnapshot(bytes, mime, name)
        }.getOrNull()
    }

    private fun hex(s: String): String =
        s.toByteArray(Charsets.UTF_8).joinToString("") { "%02x".format(it.toInt() and 0xff) }

    private fun unhex(s: String): String {
        if (s.isEmpty() || s.length % 2 != 0) return ""
        return runCatching {
            val b = ByteArray(s.length / 2) { i -> s.substring(i * 2, i * 2 + 2).toInt(16).toByte() }
            String(b, Charsets.UTF_8)
        }.getOrDefault("")
    }
}
