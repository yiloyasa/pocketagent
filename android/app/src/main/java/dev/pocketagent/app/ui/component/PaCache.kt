package dev.pocketagent.app.ui.component

import android.content.Context
import java.io.File

/**
 * 本地缓存与临时文件的统一管家(cacheDir 下的一切)。
 *
 * - 崩溃残留清扫:进程被杀时 finally 没跑到的临时文件,下次启动按**已知前缀**清掉;
 * - 预览内容缓存:见 [PreviewCache](离线预览 / 二次预览免传输);
 * - 各分区占用统计:供「设置 → 缓存管理」展示。
 *
 * 【边界】cacheDir 是共享目录(WebView、系统组件都在用),只动自己前缀的文件。
 */
object PaCache {

    /** PcFilePreview 的临时文件前缀:PDF 渲染字节(pa_pdf_*)、zip 目录列表(pa_ziplist*)。 */
    private val TEMP_PREFIXES = arrayOf("pa_pdf_", "pa_ziplist")

    /** 预览内容缓存目录(cacheDir 下的子目录,归 [PreviewCache] 管)。 */
    const val PREVIEW_DIR = "pa_preview"

    fun init(ctx: Context) {
        sweepTemp(ctx)
        PreviewCache.init(ctx)
    }

    /** 启动清扫:删掉崩溃/被杀进程留下的临时文件(单文件最大 64MB,不清会越积越多)。 */
    fun sweepTemp(ctx: Context): Int {
        var removed = 0
        val files = ctx.cacheDir.listFiles() ?: return 0
        for (f in files) {
            if (f.isFile && TEMP_PREFIXES.any { f.name.startsWith(it) }) {
                if (runCatching { f.delete() }.getOrDefault(false)) removed++
            }
        }
        return removed
    }

    /** cacheDir 的当前占用(临时文件 + 预览缓存),供缓存管理页展示。 */
    fun cacheUsage(ctx: Context): Long = dirSize(ctx.cacheDir)

    /** 已知前缀的临时文件当前占用(崩溃残留会体现在这里)。 */
    fun tempUsage(ctx: Context): Long {
        val files = ctx.cacheDir.listFiles() ?: return 0
        return files.filter { f ->
            f.isFile && TEMP_PREFIXES.any { f.name.startsWith(it) }
        }.sumOf { it.length() }
    }

    /** 清理内置浏览器的 Web 数据:DOM Storage + HTTP 缓存 + 表单数据(审查 C5)。
     * 缓存本身对浏览体验有益,所以只做用户主动清理,不自动执行。 */
    fun clearBrowserData(ctx: Context) {
        runCatching { android.webkit.WebStorage.getInstance().deleteAllData() }
        runCatching { android.webkit.WebView(ctx).clearCache(true) }
        runCatching { android.webkit.WebViewDatabase.getInstance(ctx).clearFormData() }
        runCatching { android.webkit.CookieManager.getInstance().removeAllCookies(null) }
        runCatching { android.webkit.CookieManager.getInstance().flush() }
    }

    private fun dirSize(dir: File): Long {
        val files = dir.listFiles() ?: return 0
        var total = 0L
        for (f in files) total += if (f.isDirectory) dirSize(f) else f.length()
        return total
    }
}

/**
 * PC 文件预览的**内容缓存**(审查 C2/O3):缓存原始字节,键 = path+size+mtime 哈希。
 *
 * 【解决什么】
 * - 二次预览同一文件:免掉整份 gRPC 传输(USB 尚可,Tailscale 中继下明显);
 * - **离线可用**:断线后"刚看过的文件"照样能打开 —— 遥控器形态下最重要的兜底。
 *
 * 【失效语义】文件在电脑上被编辑 → mtime 变 → 键变 → 天然 miss 重拉,旧条目等 LRU 淘汰。
 * 没有 size/mtime 的入口(会话消息里的裸路径)不走缓存 —— 宁可慢,不能脏。
 *
 * 【LRU】总量上限 [MAX_TOTAL_BYTES];文件 lastModified 即"最后使用时间",
 * 命中即 touch,淘汰按最旧先删。无索引文件,重启后依然工作。
 */
object PreviewCache {

    /** 预览缓存总量上限(字节);单测会下调以验证 LRU。 */
    internal var maxTotalBytes = 200L * 1024 * 1024

    /** 单文件上限:与预览拉取上限一致,超大的本来就不会进预览管线。 */
    internal var maxEntryBytes = 64L * 1024 * 1024

    private var dir: File? = null

    fun init(ctx: Context) {
        val d = File(ctx.cacheDir, PaCache.PREVIEW_DIR)
        runCatching { d.mkdirs() }
        dir = d
    }

    /** 仅供单测:直接指定缓存目录,绕开 Context。 */
    internal fun initForTest(d: File) {
        runCatching { d.mkdirs() }
        dir = d
    }

    private fun key(path: String, size: Long, mtime: Long): String {
        val md = java.security.MessageDigest.getInstance("MD5")
        val digest = md.digest("$path\u0000$size\u0000$mtime".toByteArray(Charsets.UTF_8))
        // Byte 有符号:直接 %02x 会得到 ffffff80 这种,必须掩成无符号
        return digest.joinToString("") { "%02x".format(it.toInt() and 0xff) }
    }

    /** 命中则返回字节并 touch(更新 LRU 时间);未命中/异常返回 null。 */
    fun get(path: String, size: Long, mtime: Long): ByteArray? {
        val d = dir ?: return null
        val f = File(d, key(path, size, mtime))
        if (!f.isFile) return null
        val bytes = runCatching { f.readBytes() }.getOrNull() ?: return null
        // 【touch 放最后一步】读失败时文件可能已被删,不再参与 LRU
        runCatching { f.setLastModified(System.currentTimeMillis()) }
        return bytes
    }

    /** 写入缓存并执行 LRU 淘汰。调用方需保证 bytes 与 size/mtime 一致(防脏缓存)。 */
    fun put(path: String, size: Long, mtime: Long, bytes: ByteArray) {
        val d = dir ?: return
        if (bytes.size > maxEntryBytes) return
        runCatching {
            val f = File(d, key(path, size, mtime))
            f.writeBytes(bytes)
            evictIfNeeded(d)
        }
    }

    /** 按最旧先删,把总量压回上限内。 */
    private fun evictIfNeeded(d: File) {
        val files = d.listFiles() ?: return
        var total = files.sumOf { it.length() }
        if (total <= maxTotalBytes) return
        val ordered = files.sortedBy { it.lastModified() }
        for (f in ordered) {
            if (total <= maxTotalBytes) break
            // 长度必须在删除前取:delete 之后 length() 恒为 0,total 就永远不会下降,
            // 结果会把整个缓存目录清空(而不是只淘汰最旧的)。
            val len = f.length()
            if (f.delete()) total -= len
        }
    }

    /** 缓存总量(字节),供缓存管理页展示。 */
    fun usage(): Long = dir?.let { dirSize2(it) } ?: 0L

    private fun dirSize2(dir: File): Long {
        val files = dir.listFiles() ?: return 0
        return files.sumOf { if (it.isDirectory) dirSize2(it) else it.length() }
    }

    /** 清空预览缓存(用户主动)。返回释放的字节数。 */
    fun clear(): Long {
        val d = dir ?: return 0L
        val before = dirSize2(d)
        d.listFiles()?.forEach { runCatching { it.deleteRecursively() } }
        return before
    }
}
