package dev.pocketagent.app.ui.component

import android.content.ContentValues
import android.content.Context
import android.os.Build
import android.os.Environment
import android.provider.MediaStore
import dev.pocketagent.app.AgentConnection
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import pocketagent.v1.ReadFileRequest
import java.io.OutputStream

/**
 * 下载到手机 —— 会话里输出的文件/内容可以落到手机本地,并且**在后台跑**。
 *
 * 【链路】daemon 的 ReadFile 是分块流(64KB/块):这里边收边写 MediaStore,
 * 进度实时喂给 [TaskCenter](任务中心"文件下载"分区可见),
 * 同时复用 [AgentConnection.startTurnService] 的前台服务保活 —— 切后台/锁屏不断。
 *
 * 【为什么不用系统 DownloadManager】它的输入必须是 URL,
 * 而我们要通过 gRPC 从电脑那台 daemon 取流,只能自己写。
 */
object PaDownloader {

    /** 单文件下载上限 512MB:足够装下 APK/安装包/文档,同时防止误触超大文件。 */
    private const val MAX_DOWNLOAD_BYTES = 512L * 1024 * 1024
    private var appCtx: Context? = null
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val running = java.util.concurrent.atomic.AtomicInteger(0)

    fun init(ctx: Context) {
        if (appCtx == null) appCtx = ctx.applicationContext
    }

    /**
     * 从电脑下载一个文件到手机。
     * @param sourceTitle 来源会话标题 —— 任务中心展示「来自哪个会话」
     */
    /** 从电脑下载一个文件到手机的"下载"目录(整份进度上报任务中心)。 */
    fun downloadFromPc(remotePath: String, sourceTitle: String = "", threadId: String = "") {
        val ctx = appCtx ?: return
        val name = remotePath.substringAfterLast('/').substringAfterLast('\\').ifBlank { "下载文件" }
        val taskId = TaskCenter.trackDownload(remotePath, name, threadId)
        TaskCenter.annotateDownload(taskId, sourceTitle)
        // 任务中心立刻能看到这条;保活服务同时拉起
        if (running.incrementAndGet() == 1) AgentConnection.startTurnService()
        scope.launch {
            var sink: OutputStream? = null
            // 【失败清理】记录已建好的写入位置:半路失败时把半截文件删掉,
            // 否则系统「下载/PocketAgent」里会堆积 0KB/半截的残留(用户在 app 里看不到,只能去文件管理里翻)。
            var targetUri: String? = null
            try {
                val target = createSink(ctx, name, mimeOf(name))
                sink = target.first
                targetUri = target.third
                val display = target.second
                var written = 0L
                var total = 0L
                val stream = AgentConnection.fileService().readFile(
                    ReadFileRequest.newBuilder()
                        // 【必须显式给上限】max_bytes=0 在 daemon 侧表示「用服务端默认值」,
                        // 而默认只有 2MB —— 下载 20MB 的 APK 会被静默截断成 2MB(用户看到的"大小不对")。
                        .setPath(remotePath)
                        .setMaxBytes(MAX_DOWNLOAD_BYTES)
                        .build(),
                )
                while (stream.hasNext()) {
                    val chunk = stream.next()
                    if (total == 0L && chunk.totalSize > 0) total = chunk.totalSize
                    val bytes = chunk.data.toByteArray()
                    sink.write(bytes)
                    written += bytes.size
                    TaskCenter.updateDownload(
                        taskId,
                        if (total > 0) written.toFloat() / total else 0f,
                        "${fmt(written)}${if (total > 0) " / ${fmt(total)}" else ""}",
                    )
                }
                // 【完整性校验】流正常结束但没下满(daemon 中途退出/被代理截断)——
                // 之前这种情况会报"成功"但文件是坏的,现在按失败处理并清理。
                if (total > 0 && written < total) {
                    throw IllegalStateException("传输不完整:${fmt(written)} / ${fmt(total)}")
                }
                sink.flush()
                TaskCenter.finishDownload(
                    taskId, true, "已保存到「$display」",
                    localUri = target.third, sizeText = fmt(written),
                )
                targetUri = null // 成功:文件归用户,不再清理
            } catch (e: Exception) {
                deletePartialFile(ctx, targetUri, sink)
                TaskCenter.finishDownload(taskId, false, "下载失败:${e.message ?: "未知错误"}")
            } finally {
                runCatching { sink?.close() }
                if (running.decrementAndGet() == 0) AgentConnection.stopTurnService()
            }
        }
    }

    /** 把一段文本(会话输出/代码块)存成手机上的文件。 */
    fun saveTextAsFile(fileName: String, text: String, threadId: String = "") {
        val ctx = appCtx ?: return
        val safeName = fileName.replace(Regex("[\\\\/:*?\"<>|]"), "_").take(80).ifBlank { "会话输出.txt" }
        val key = "$threadId/$safeName"
        val taskId = TaskCenter.trackDownload(key, safeName, threadId)
        if (running.incrementAndGet() == 1) AgentConnection.startTurnService()
        scope.launch {
            var sink: OutputStream? = null
            var targetUri: String? = null
            try {
                // 【MIME 必须按名字给】之前这里硬编码 text/plain,MediaStore 会据此
                // 把 DISPLAY_NAME 补成 ".md.txt" —— 文件名和内容对不上,
                // 分享出去对方也认不出是 Markdown。统一走 FileKind。
                val target = createSink(ctx, safeName, mimeOf(safeName))
                sink = target.first
                targetUri = target.third
                val bytes = text.toByteArray()
                sink.write(bytes)
                sink.flush()
                TaskCenter.finishDownload(
                    taskId, true, "已保存到「${target.second}」",
                    localUri = target.third, sizeText = fmt(bytes.size.toLong()),
                )
                targetUri = null
            } catch (e: Exception) {
                deletePartialFile(ctx, targetUri, sink)
                TaskCenter.finishDownload(taskId, false, "保存失败:${e.message ?: "未知错误"}")
            } finally {
                runCatching { sink?.close() }
                if (running.decrementAndGet() == 0) AgentConnection.stopTurnService()
            }
        }
    }

    /**
     * 从**网页 URL** 下载文件到手机(内置浏览器 setDownloadListener 的落点)。
     *
     * 【为什么不用系统 DownloadManager】它把文件丢进系统下载、且我们拿不到统一的
     * 任务中心进度/离线快照;这里自己用 HttpURLConnection 流式落盘,顺带把中小文件
     * 存一份到 [OfflineStore](同一 URL 再次打开可离线渲染,与"离线优先"打通)。
     *
     * @param cookie 当前 WebView 的 Cookie(百度/抖音等登录态站点下载依赖它)
     */
    fun downloadFromUrl(
        url: String,
        name: String,
        mime: String,
        userAgent: String = "",
        cookie: String = "",
        sourceTitle: String = "",
        threadId: String = "",
    ) {
        val ctx = appCtx ?: return
        val safeName = sanitizeName(name, mime)
        val taskId = TaskCenter.trackDownload(url, safeName, threadId)
        TaskCenter.annotateDownload(taskId, sourceTitle)
        if (running.incrementAndGet() == 1) AgentConnection.startTurnService()
        scope.launch {
            var sink: OutputStream? = null
            var targetUri: String? = null
            try {
                val conn = (java.net.URL(url).openConnection() as java.net.HttpURLConnection).apply {
                    connectTimeout = 15_000
                    readTimeout = 30_000
                    instanceFollowRedirects = true
                    setRequestProperty("Accept", "*/*")
                    if (userAgent.isNotEmpty()) setRequestProperty("User-Agent", userAgent)
                    if (cookie.isNotEmpty()) setRequestProperty("Cookie", cookie)
                }
                conn.connect()
                val code = conn.responseCode
                if (code !in 200..299) throw IllegalStateException("服务器返回 HTTP $code")
                val total = conn.contentLengthLong
                val resolvedMime = conn.contentType?.substringBefore(';')?.trim().orEmpty()
                    .ifBlank { mime.ifBlank { mimeOf(safeName) } }
                val target = createSink(ctx, safeName, resolvedMime)
                sink = target.first
                targetUri = target.third
                var written = 0L
                // 小于 4MB 的顺带留一份离线快照(同 URL 再开能离线渲染),太大就只落盘不缓存
                val keepSnapshot = total in 1..(4L * 1024 * 1024)
                val buffered = if (keepSnapshot) java.io.ByteArrayOutputStream() else null
                conn.inputStream.use { input ->
                    val buf = ByteArray(64 * 1024)
                    while (true) {
                        val n = input.read(buf)
                        if (n < 0) break
                        sink.write(buf, 0, n)
                        written += n
                        buffered?.write(buf, 0, n)
                        TaskCenter.updateDownload(
                            taskId,
                            if (total > 0) written.toFloat() / total else 0f,
                            "${fmt(written)}${if (total > 0) " / ${fmt(total)}" else ""}",
                        )
                    }
                }
                if (total > 0 && written < total) {
                    throw IllegalStateException("传输不完整:${fmt(written)} / ${fmt(total)}")
                }
                sink.flush()
                if (buffered != null && written > 0) {
                    OfflineStore.putUrl(url, buffered.toByteArray(), resolvedMime, safeName)
                }
                TaskCenter.finishDownload(
                    taskId, true, "已保存到「${target.second}」",
                    localUri = target.third, sizeText = fmt(written),
                )
                targetUri = null
            } catch (e: Exception) {
                deletePartialFile(ctx, targetUri, sink)
                TaskCenter.finishDownload(taskId, false, "下载失败:${e.message ?: "未知错误"}")
            } finally {
                runCatching { sink?.close() }
                if (running.decrementAndGet() == 0) AgentConnection.stopTurnService()
            }
        }
    }

    /**
     * 把页面里用 JS 读出来的 base64 内容(blob:/data: 兜底)落盘。
     * blob: 是浏览器内部的临时 URL,HttpURLConnection 取不到 —— 只能在页面里 fetch 后回传。
     */
    fun saveBase64(name: String, mime: String, base64: String) {
        val ctx = appCtx ?: return
        val safeName = sanitizeName(name, mime)
        val taskId = TaskCenter.trackDownload("blob:${System.currentTimeMillis()}:$safeName", safeName)
        if (running.incrementAndGet() == 1) AgentConnection.startTurnService()
        scope.launch {
            var sink: OutputStream? = null
            var targetUri: String? = null
            try {
                val bytes = android.util.Base64.decode(base64, android.util.Base64.DEFAULT)
                val target = createSink(ctx, safeName, mime.ifBlank { mimeOf(safeName) })
                sink = target.first
                targetUri = target.third
                sink.write(bytes)
                sink.flush()
                TaskCenter.finishDownload(
                    taskId, true, "已保存到「${target.second}」",
                    localUri = target.third, sizeText = fmt(bytes.size.toLong()),
                )
                targetUri = null
            } catch (e: Exception) {
                deletePartialFile(ctx, targetUri, sink)
                TaskCenter.finishDownload(taskId, false, "保存失败:${e.message ?: "未知错误"}")
            } finally {
                runCatching { sink?.close() }
                if (running.decrementAndGet() == 0) AgentConnection.stopTurnService()
            }
        }
    }

    /** 文件名规范化:清掉非法字符、缺后缀时按 MIME 补上、限长。 */
    private fun sanitizeName(name: String, mime: String): String {
        var n = name.replace(Regex("[\\\\/:*?\"<>|]"), "_").trim().ifBlank { "下载文件" }
        if (!n.contains('.')) {
            val ext = DocFormatRegistry.extFor(mime)
            if (ext.isNotEmpty()) n = "$n$ext"
        }
        return n.take(120)
    }

    /**
     * 建一个写入口。
     * API 29+ 走 MediaStore.Downloads(免存储权限,落进系统"下载"目录);
     * 更低版本退到 App 自己的外部目录,避免申请 WRITE_EXTERNAL_STORAGE。
     */
    private fun createSink(ctx: Context, name: String, mime: String): Triple<OutputStream, String, String> {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            val values = ContentValues().apply {
                put(MediaStore.MediaColumns.DISPLAY_NAME, name)
                put(MediaStore.MediaColumns.MIME_TYPE, mime)
                put(MediaStore.MediaColumns.RELATIVE_PATH, Environment.DIRECTORY_DOWNLOADS + "/PocketAgent")
            }
            val uri = ctx.contentResolver.insert(
                MediaStore.Downloads.EXTERNAL_CONTENT_URI,
                values,
            ) ?: throw IllegalStateException("系统未允许创建文件")
            val out = ctx.contentResolver.openOutputStream(uri)
                ?: throw IllegalStateException("无法写入文件")
            Triple(out, "下载/PocketAgent/$name", uri.toString())
        } else {
            val dir = ctx.getExternalFilesDir(Environment.DIRECTORY_DOWNLOADS)
                ?: ctx.filesDir
            if (!dir.exists()) dir.mkdirs()
            val file = java.io.File(dir, name)
            Triple(file.outputStream(), file.absolutePath, android.net.Uri.fromFile(file).toString())
        }
    }

    /**
     * 失败路径清理半截文件:MediaStore 条目整条删掉(不留 0KB 残尸),
     * 低版本 file:// 直接删文件。删不动就算了 —— 不让清理失败掩盖真正的下载错误。
     */
    private fun deletePartialFile(ctx: Context, uri: String?, sink: OutputStream?) {
        if (uri == null) return
        runCatching { sink?.close() }
        runCatching {
            val parsed = android.net.Uri.parse(uri)
            if (parsed.scheme == "content") {
                ctx.contentResolver.delete(parsed, null, null)
            } else {
                parsed.path?.let { java.io.File(it).delete() }
            }
        }
    }

    /**
     * 打开已下载到手机的文件 —— 按 [FileKind.preview] 三态分派。
     *
     *  · IN_APP   → 交给内置浏览器(网页/文本/Markdown/图片/Office/PDF 都能渲染);
     *  · SYSTEM   → 交给系统应用(安装包/旧版 Office/音视频);
     *  · SAVE_ONLY→ 没有承接方,明确告知保存位置,而不是静默失败。
     */
    fun openLocalFile(
        ctx: Context,
        uri: String,
        title: String,
        detail: String = "",
        onOpenInBrowser: ((String) -> Unit)? = null,
    ) {
        if (uri.isEmpty()) {
            toast(ctx, if (detail.isNotEmpty()) detail else "文件位置未知")
            return
        }
        // 【悬空条目】用户可能在系统文件管理器/清理软件里把文件删了,
        // 而任务条目还留着 localUri —— 直接打开会静默失败或白屏。
        // 先校验存在性,失效就明说并给出去处。
        if (!localFileExists(ctx, uri)) {
            toast(ctx, "文件已被移动或删除\n可在「设置 → 用户数据管理」里移除该条目")
            return
        }
        val kind = fileKindOf(title)
        if (kind.preview == PreviewMode.IN_APP && onOpenInBrowser != null) {
            onOpenInBrowser(uri)
            return
        }
        if (kind.preview == PreviewMode.SAVE_ONLY) {
            toast(ctx, "这个格式没有可打开的应用\n已保存到:${detail.ifEmpty { uri }}")
            return
        }
        // 【file:// 必须转 content://】API 24+ 直接 ACTION_VIEW 一个 file:// 会抛
        // FileUriExposedException,被外层 catch 吞掉后就表现为"没有能打开的应用"。
        // 这里统一经 FileProvider 换成可授权的 content://。
        val parsed = android.net.Uri.parse(uri)
        val dataUri: android.net.Uri = if (parsed.scheme == "file") {
            val path = parsed.path
            if (path.isNullOrEmpty()) {
                toast(ctx, "文件位置无效")
                return
            }
            runCatching {
                androidx.core.content.FileProvider.getUriForFile(
                    ctx, ctx.packageName + ".fileprovider", java.io.File(path),
                )
            }.getOrNull() ?: run {
                toast(ctx, "这个文件不在可共享的目录内\n已保存到:${detail.ifEmpty { uri }}")
                return
            }
        } else {
            parsed
        }
        val intent = android.content.Intent(android.content.Intent.ACTION_VIEW).apply {
            setDataAndType(dataUri, kind.mime)
            addFlags(android.content.Intent.FLAG_GRANT_READ_URI_PERMISSION)
            addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        try {
            ctx.startActivity(intent)
        } catch (_: Exception) {
            toast(ctx, "手机上还没有能打开 ${kind.label} 的应用\n已保存到:${detail.ifEmpty { uri }}")
        }
    }

    /**
     * 清理悬空下载条目:文件已被用户/清理软件删掉,但任务条目还留着,
     * 点「查看」会静默失败(审查 D2)。返回清理掉的条数。
     * 只动 localUri 非空且确认失效的条目;localUri 为空的(老版本历史条目)不动。
     */
    fun pruneDanglingTasks(ctx: Context): Int {
        val targets = TaskCenter.tasks.value.filter {
            it.kind == TaskCenter.Kind.DOWNLOAD && it.localUri.isNotEmpty()
        }
        var n = 0
        targets.forEach { t ->
            if (!localFileExists(ctx, t.localUri)) {
                TaskCenter.remove(t.id)
                n++
            }
        }
        return n
    }

    /** 本地文件是否还实际存在(系统侧可能已被用户或清理软件删除)。 */
    fun localFileExists(ctx: Context, uri: String): Boolean {
        return runCatching {
            val parsed = android.net.Uri.parse(uri)
            when (parsed.scheme) {
                "content" -> ctx.contentResolver.openFileDescriptor(parsed, "r")?.use { true } ?: false
                "file" -> parsed.path?.let { java.io.File(it).exists() } ?: false
                else -> false
            }
        }.getOrDefault(false)
    }

    /**
     * 查本地文件真实字节数(content:// 走 MediaStore 的 file descriptor,file:// 走 File.length)。
     * 文件不存在或读取失败返回 null —— 调用方据此如实显示「未知」,而**绝不用 0 冒充**
     * "没占用"(之前下载合计在 sizeText 缺失时会算出 0,等于骗用户"什么都没下")。
     * 纯本地查询,零 daemon 往返,断网也能用。
     */
    fun queryLocalFileSize(ctx: Context, uri: String): Long? {
        if (uri.isEmpty()) return null
        return runCatching {
            val parsed = android.net.Uri.parse(uri)
            when (parsed.scheme) {
                "content" -> ctx.contentResolver.openFileDescriptor(parsed, "r")?.use { it.statSize } ?: null
                "file" -> parsed.path?.let { java.io.File(it).length().takeIf { s -> s > 0L } }
                else -> null
            }
        }.getOrNull()
    }

    /**
     * 按文件名在「下载/PocketAgent」里找回文件 —— 旧版本的历史条目没存 localUri
     * (早期 finishDownload 不落盘这个字段),但文件多半还躺在原处。
     * 找到就返回可打开的 URI(条目由调用方补回,查看/分享图标随之恢复);
     * 找不到(已删除/改名)返回空串。同名多个时取最近添加的一个。
     */
    fun resolveLocalUri(ctx: Context, fileName: String): String {
        if (fileName.isBlank()) return ""
        return runCatching {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                val sel =
                    "${MediaStore.MediaColumns.DISPLAY_NAME}=? AND ${MediaStore.MediaColumns.RELATIVE_PATH}=?"
                ctx.contentResolver.query(
                    MediaStore.Downloads.EXTERNAL_CONTENT_URI,
                    arrayOf(MediaStore.MediaColumns._ID),
                    sel,
                    arrayOf(fileName, Environment.DIRECTORY_DOWNLOADS + "/PocketAgent/"),
                    MediaStore.MediaColumns.DATE_ADDED + " DESC",
                )?.use { c ->
                    if (c.moveToFirst()) {
                        val id = c.getLong(0)
                        android.content.ContentUris
                            .withAppendedId(MediaStore.Downloads.EXTERNAL_CONTENT_URI, id)
                            .toString()
                    } else {
                        ""
                    }
                } ?: ""
            } else {
                val dir = ctx.getExternalFilesDir(Environment.DIRECTORY_DOWNLOADS)
                val f = dir?.let { java.io.File(it, fileName) }
                if (f != null && f.exists()) android.net.Uri.fromFile(f).toString() else ""
            }
        }.getOrDefault("")
    }

    /**
     * 分享手机上的文件。
     * 【为什么不用 FileProvider 转一道】下载走的是 MediaStore,拿到的已经是
     * `content://media/...` 这种可授权的 URI,直接带上读权限即可;
     * 低版本(<Q)落到 App 外部目录时才是 `file://`,那种情况由 Provider 兜底。
     */
    fun shareLocalFile(ctx: Context, uri: String, title: String = "") {
        if (uri.isEmpty()) return
        val kind = fileKindOf(title)
        runCatching {
            val parsed = android.net.Uri.parse(uri)
            val safe = if (parsed.scheme == "file") {
                androidx.core.content.FileProvider.getUriForFile(
                    ctx,
                    ctx.packageName + ".fileprovider",
                    java.io.File(parsed.path ?: return),
                )
            } else {
                parsed
            }
            val intent = android.content.Intent(android.content.Intent.ACTION_SEND).apply {
                type = kind.mime
                putExtra(android.content.Intent.EXTRA_STREAM, safe)
                addFlags(android.content.Intent.FLAG_GRANT_READ_URI_PERMISSION)
            }
            ctx.startActivity(android.content.Intent.createChooser(intent, "分享文件"))
        }.onFailure { toast(ctx, "分享失败:${it.message ?: "未知错误"}") }
    }

    private fun toast(ctx: Context, msg: String) {
        android.widget.Toast.makeText(ctx, msg, android.widget.Toast.LENGTH_LONG).show()
    }

    /** 后缀 → MIME:统一由格式登记表提供,避免两处清单走偏。 */
    private fun mimeOf(name: String): String = DocFormatRegistry.mimeFor(name)

    private fun fmt(bytes: Long): String = when {
        bytes >= 1024 * 1024 -> String.format(java.util.Locale.US, "%.1fMB", bytes / 1048576f)
        bytes >= 1024 -> String.format(java.util.Locale.US, "%.0fKB", bytes / 1024f)
        else -> "${bytes}B"
    }
}
