package dev.pocketagent.app.ui.component

import android.content.Context
import java.io.File

/**
 * 离线存储统一入口(cacheDir 下的一块自留地)。
 *
 * 【它解决什么】
 *  · **可扩展的缓存分区注册表** —— 新增一类缓存(网页快照/缩略图/…)只加一行 [partitions];
 *  · **可单测的纯函数** —— LRU 顺序、淘汰集合、字节统计、陈旧判定、缓存键全部是纯逻辑,
 *    不碰磁盘也能验证(历史坑:`f.delete()` 之后 `f.length()` 恒 0,导致算出"没占用");
 *  · **网页快照缓存** —— 断网时同一个 URL 命中本地快照就直接渲染,不再白屏等超时。
 *
 * 【分区注册表(2026-09-30 扩为三项)】[partitions] 是缓存管理页聚合展示的唯一清单:
 * 名字/占用/条目数/可清理性都从这里出发([partitionStats])。其中
 *  - `url` 是**物理分区**(pa_offline 下真实子目录,本类自管读写);
 *  - `checkpoint` / `snapshot` 是**逻辑分区**:物理目录仍是既有磁盘契约
 *    (filesDir/turn-checkpoints、filesDir/pa_snap,经 [PaPaths] 取,见 [Anchor]),
 *    读写与 LRU 由 TurnCheckpointStore / ThreadSnapshot 各自自管 —— 注册表只做
 *    "统一登记口径",**不迁移、不重命名任何现有目录与文件**。
 *
 * 【边界】只动自己的子目录(pa_offline/…)与注册表登记的逻辑分区目录;
 * 不碰 cacheDir 里的其它内容(WebView 等)。
 */
object OfflineStore {

    /**
     * 分区物理落点。
     *  - [Anchor.OFFLINE]:cacheDir/pa_offline/<subDir>(本类自管的物理分区);
     *  - [Anchor.TURN_CHECKPOINTS] / [Anchor.SNAPSHOT]:逻辑分区,目录由 [PaPaths]
     *    的便捷构造给出(启动时在 [init] 解析一次,不持 Context)。
     */
    enum class Anchor { OFFLINE, TURN_CHECKPOINTS, SNAPSHOT }

    /**
     * 一个缓存分区(扩展点)。新增一类缓存 = 在 [partitions] 里加一行。
     * @param maxBytes  该分区容量上限,超限按 LRU 淘汰;
     * @param clearable false = 用户显式下载内容,「清理缓存」**不得**删除;
     * @param ttlMs     生命周期:距上次使用超过该时长视为陈旧,启动清扫时删除;
     *                  0 = 永不过期(只受 LRU 容量约束);
     * @param anchor    物理落点(默认 pa_offline 下的子目录;逻辑分区见 [Anchor])。
     *                  非 OFFLINE 时 [Partition.subDir] 仅作登记说明,物理目录由 anchor 决定。
     */
    data class Partition(
        val id: String,
        val label: String,
        val subDir: String,
        val maxBytes: Long,
        val clearable: Boolean,
        val ttlMs: Long = 0L,
        val anchor: Anchor = Anchor.OFFLINE,
    )

    val partitions: List<Partition> = listOf(
        // 1) 网页快照(物理分区):cacheDir/pa_offline/pa_url。30 天没用过的快照视为陈旧,
        //    启动时清掉 —— 离线兜底只看"最近看过的",更老的大概率页面已更新,
        //    留着既占空间又可能展示过期内容。
        Partition("url", "网页快照", "pa_url", 80L * 1024 * 1024, true, 30L * 24 * 60 * 60 * 1000),
        // 2) 轮次检查点(逻辑分区):物理目录 = PaPaths.turnCheckpointsDir()
        //    (filesDir/turn-checkpoints,磁盘契约,不迁移不重命名)。读写与 LRU 由
        //    TurnCheckpointStore 自管(50 个/8MB,见其文件顶部常量);这里登记口径供
        //    缓存管理页聚合展示,「单独清」动作走 OfflineStore.clear("checkpoint")。
        Partition(
            "checkpoint", "轮次检查点", PaPaths.DIR_TURN_CHECKPOINTS,
            8L * 1024 * 1024, true, 0L, Anchor.TURN_CHECKPOINTS,
        ),
        // 3) 会话快照(逻辑分区):物理目录 = PaPaths.snapshotDir()(filesDir/pa_snap,
        //    磁盘契约)。读写与三层限额由 ThreadSnapshot 自管(50 会话/5MB)。clearable=true
        //    指"可清理",但**清理动作必须走 ThreadSnapshot.clearAll()**(CacheCleanupLogic
        //    把 SNAPSHOT 定为 RISKY,二次确认),不走 OfflineStore.clear —— 分级语义由
        //    CacheCleanupLogic 钉死,注册表只登记展示口径。
        Partition(
            "snapshot", "会话快照", PaPaths.DIR_SNAPSHOT,
            5L * 1024 * 1024, true, 0L, Anchor.SNAPSHOT,
        ),
    )

    /** 一条缓存条目(不含文件句柄,便于纯函数单测)。 */
    data class CacheEntry(val name: String, val size: Long, val lastUsedMs: Long)

    // -------------------------------------------------------------- 纯函数(可单测)

    /** 一个分区的当前占用(调用方先取好每个条目的字节数)。 */
    fun bytesUsed(entries: List<CacheEntry>): Long = entries.sumOf { it.size }

    /** 一个分区的聚合口径(缓存管理页「名称/占用/条目数/可清理性」一行的数据源)。 */
    data class PartitionStats(
        val id: String,
        val label: String,
        val bytes: Long,
        /** 条目数 = 分区目录内的普通文件数(pa_url/检查点每文件即一条;pa_snap 含列表级文件)。 */
        val entries: Int,
        /** 可清理性元数据:直接来自 [Partition.clearable]。 */
        val clearable: Boolean,
    )

    /**
     * 某分区的聚合统计(未知 id 返回 null;目录尚未建出时给 0 值,不装作没有这个分区)。
     * 【复用纯函数】字节口径走 [CacheEntry] + [bytesUsed],与淘汰/清理完全一致
     * (不会出现"列表说 10KB、清理却没东西删"的口径错位)。
     */
    fun partitionStats(id: String): PartitionStats? {
        val p = registry.firstOrNull { it.id == id } ?: return null
        val files = dirFor(id)?.listFiles()?.filter { it.isFile } ?: emptyList()
        val entries = files.map { CacheEntry(it.name, it.length(), it.lastModified()) }
        return PartitionStats(p.id, p.label, bytesUsed(entries), entries.size, p.clearable)
    }

    /** 全部分区的聚合统计(注册表顺序),缓存管理页一次取齐。 */
    fun allPartitionStats(): List<PartitionStats> = registry.mapNotNull { partitionStats(it.id) }

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
    fun cacheKey(raw: String): String =
        // 统一走 PaFormat.stableHashMd5,收敛见审查报告 util 节。
        stableHashMd5(raw)

    // -------------------------------------------------------------- 目录与读写

    private var cacheRoot: File? = null

    /** 逻辑分区的物理目录(生产在 [init] 经 [PaPaths] 解析一次;单测经 [initForTest] 注入)。 */
    private var anchorDirs: Map<Anchor, File> = emptyMap()

    /**
     * 当前生效的分区表。生产恒等于 [partitions];单测可注入自定义分区来覆盖
     * `clearable=false` 等分支(见 [initForTest])。
     */
    private var registry: List<Partition> = partitions

    fun init(ctx: Context) {
        val root = PaPaths.offlineRoot(ctx)
        runCatching { root.mkdirs() }
        cacheRoot = root
        // 逻辑分区的物理目录是既有磁盘契约(TurnCheckpointStore/ThreadSnapshot 的自管目录,
        // 它们各自 init 时已 mkdirs),这里只解析引用,不代建、不迁移。
        anchorDirs = mapOf(
            Anchor.TURN_CHECKPOINTS to PaPaths.turnCheckpointsDir(ctx),
            Anchor.SNAPSHOT to PaPaths.snapshotDir(ctx),
        )
        registry = partitions
        registry.filter { it.anchor == Anchor.OFFLINE }
            .forEach { runCatching { File(root, it.subDir).mkdirs() } }
    }

    /**
     * 仅供单测:直接指定根目录,绕开 Context(android.jar 在 JVM 单测里不可用)。
     * [parts] 默认用生产 [partitions],测试可注入(例如一个 clearable=false 的分区);
     * [anchors] 给逻辑分区指定物理目录(缺省 = 该分区 dirFor 返回 null,统计为 0)。
     */
    internal fun initForTest(
        root: File,
        parts: List<Partition> = partitions,
        anchors: Map<Anchor, File> = emptyMap(),
    ) {
        runCatching { root.mkdirs() }
        cacheRoot = root
        anchorDirs = anchors
        registry = parts
        parts.filter { it.anchor == Anchor.OFFLINE }
            .forEach { runCatching { File(root, it.subDir).mkdirs() } }
    }

    /** 某分区的目录(未初始化或 id 未知返回 null;逻辑分区按 anchor 取 [PaPaths] 解析的目录)。 */
    fun dirFor(id: String): File? {
        val p = registry.firstOrNull { it.id == id } ?: return null
        return when (p.anchor) {
            Anchor.OFFLINE -> cacheRoot?.let { root -> File(root, p.subDir) }
            Anchor.TURN_CHECKPOINTS -> anchorDirs[Anchor.TURN_CHECKPOINTS]
            Anchor.SNAPSHOT -> anchorDirs[Anchor.SNAPSHOT]
        }
    }

    /** pa_offline 的总占用(字节)。 */
    fun usage(): Long = cacheRoot?.let { dirSize(it) } ?: 0L

    /**
     * 某分区的当前占用(字节);未知 id 返回 0。
     * 【复用纯函数】用 [CacheEntry] + [bytesUsed] 统计,不另写一套目录扫描 ——
     * 缓存管理页列"每一项占多大"时直接调它,保证统计口径与淘汰/清理完全一致
     * (不会遇到"列表说 10KB、清理却没东西删"的口径错位)。
     */
    fun partitionUsage(id: String): Long {
        val d = dirFor(id) ?: return 0L
        val files = d.listFiles() ?: return 0L
        return bytesUsed(files.map { CacheEntry(it.name, it.length(), it.lastModified()) })
    }

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

    private fun dirSize(dir: File): Long =
        // 统一走 PaFormat.dirSizeRecursive,收敛见审查报告 util 节。
        dirSizeRecursive(dir)

    // -------------------------------------------------------------- 网页快照缓存(离线浏览)

    /**
     * 一份网页快照:原始字节 + MIME + 建议文件名 + 截至时刻。
     * [capturedAt] 是快照的"截至哪一刻"(epoch ms,取自快照文件修改时间);
     * <=0 = 存量快照无可靠时间戳,展示层回退旧文案(不编造时间)。
     */
    data class UrlSnapshot(val bytes: ByteArray, val mime: String, val name: String, val capturedAt: Long = 0L) {
        // ByteArray 的 data class equals 是引用比较,这里补上内容比较,避免"看起来相等其实不等"
        override fun equals(other: Any?): Boolean {
            if (this === other) return true
            if (other !is UrlSnapshot) return false
            return mime == other.mime && name == other.name &&
                capturedAt == other.capturedAt && bytes.contentEquals(other.bytes)
        }

        override fun hashCode(): Int {
            var r = mime.hashCode()
            r = 31 * r + name.hashCode()
            r = 31 * r + capturedAt.hashCode()
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
            // 截至时刻必须在 LRU touch 之前读:touch 会把修改时间改写成"本次查看时刻",
            // 那是"最近看过",不是"网页截至哪一刻"。读失败(0)按无时间戳回退旧文案。
            val mtime = hit.lastModified()
            val capturedAt = if (mtime > 0) mtime else 0L
            runCatching { hit.setLastModified(System.currentTimeMillis()) }
            UrlSnapshot(bytes, mime, name, capturedAt)
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
