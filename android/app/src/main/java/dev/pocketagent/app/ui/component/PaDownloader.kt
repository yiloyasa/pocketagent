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
    private var appCtx: Context? = null
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val running = java.util.concurrent.atomic.AtomicInteger(0)

    fun init(ctx: Context) {
        if (appCtx == null) appCtx = ctx.applicationContext
    }

    /** 从电脑下载一个文件到手机的"下载"目录(整份进度上报任务中心)。 */
    fun downloadFromPc(remotePath: String) {
        val ctx = appCtx ?: return
        val name = remotePath.substringAfterLast('/').substringAfterLast('\\').ifBlank { "下载文件" }
        val taskId = TaskCenter.trackDownload(remotePath, name)
        // 任务中心立刻能看到这条;保活服务同时拉起
        if (running.incrementAndGet() == 1) AgentConnection.startTurnService()
        scope.launch {
            var sink: OutputStream? = null
            try {
                val target = createSink(ctx, name, "application/octet-stream")
                sink = target.first
                val display = target.second
                var written = 0L
                var total = 0L
                val stream = AgentConnection.fileService().readFile(
                    ReadFileRequest.newBuilder().setPath(remotePath).setMaxBytes(0).build(),
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
                sink.flush()
                TaskCenter.finishDownload(taskId, true, "已保存到「$display」")
            } catch (e: Exception) {
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
            try {
                val target = createSink(ctx, safeName, "text/plain")
                sink = target.first
                val bytes = text.toByteArray()
                sink.write(bytes)
                sink.flush()
                TaskCenter.finishDownload(taskId, true, "已保存到「${target.second}」")
            } catch (e: Exception) {
                TaskCenter.finishDownload(taskId, false, "保存失败:${e.message ?: "未知错误"}")
            } finally {
                runCatching { sink?.close() }
                if (running.decrementAndGet() == 0) AgentConnection.stopTurnService()
            }
        }
    }

    /**
     * 建一个写入口。
     * API 29+ 走 MediaStore.Downloads(免存储权限,落进系统"下载"目录);
     * 更低版本退到 App 自己的外部目录,避免申请 WRITE_EXTERNAL_STORAGE。
     */
    private fun createSink(ctx: Context, name: String, mime: String): Pair<OutputStream, String> {
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
            out to "下载/PocketAgent/$name"
        } else {
            val dir = ctx.getExternalFilesDir(Environment.DIRECTORY_DOWNLOADS)
                ?: ctx.filesDir
            if (!dir.exists()) dir.mkdirs()
            val file = java.io.File(dir, name)
            file.outputStream() to file.absolutePath
        }
    }

    private fun fmt(bytes: Long): String = when {
        bytes >= 1024 * 1024 -> String.format(java.util.Locale.US, "%.1fMB", bytes / 1048576f)
        bytes >= 1024 -> String.format(java.util.Locale.US, "%.0fKB", bytes / 1024f)
        else -> "${bytes}B"
    }
}
