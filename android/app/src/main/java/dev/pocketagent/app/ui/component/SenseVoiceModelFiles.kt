package dev.pocketagent.app.ui.component

import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import java.nio.file.Files
import java.nio.file.StandardCopyOption

internal data class SenseVoiceModelFile(val name: String, val size: Long, val sha256: String)

/** I/O implementation is injectable; downloading and resuming use identical validation. */
internal class SenseVoiceModelFiles(
    private val root: File,
    private val base: String,
    private val specs: List<SenseVoiceModelFile>,
    private val open: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection },
) {
    private fun valid(file: File, spec: SenseVoiceModelFile): Boolean =
        file.isFile && file.length() == spec.size && sha256(file) == spec.sha256

    fun isReady(): Boolean = specs.all { valid(File(root, it.name), it) }

    fun install(progress: (Long) -> Unit) {
        check(root.mkdirs() || root.isDirectory) { "无法创建模型目录" }
        var completed = 0L
        for (spec in specs) {
            val target = File(root, spec.name)
            val partial = File(root, "${spec.name}.part")
            if (valid(target, spec)) {
                completed += spec.size
                progress(completed)
                continue
            }
            if (!valid(partial, spec)) {
                if (partial.length() >= spec.size) partial.delete()
                var offset = partial.length()
                val connection = open(URL(base + spec.name))
                connection.connectTimeout = 20_000
                connection.readTimeout = 30_000
                if (offset > 0) connection.setRequestProperty("Range", "bytes=$offset-")
                try {
                    val code = connection.responseCode
                    check(code == 200 || code == 206) { "模型下载失败 HTTP $code" }
                    if (code == 200) offset = 0
                    if (code == 206) {
                        val range = connection.getHeaderField("Content-Range").orEmpty()
                        val expected = "bytes $offset-${spec.size - 1}/${spec.size}"
                        if (range != expected) {
                            partial.delete()
                            error("模型断点响应错误，请重试")
                        }
                    }
                    var received = offset
                    java.io.FileOutputStream(partial, offset > 0).use { output ->
                        connection.inputStream.use { input ->
                            val buffer = ByteArray(64 * 1024)
                            var lastReport = received
                            while (true) {
                                val n = input.read(buffer)
                                if (n < 0) break
                                received += n
                                if (received > spec.size) {
                                    output.close()
                                    partial.delete()
                                    error("模型下载长度异常")
                                }
                                output.write(buffer, 0, n)
                                if (received - lastReport >= 256 * 1024) {
                                    progress(completed + received)
                                    lastReport = received
                                }
                            }
                        }
                    }
                    check(partial.length() == spec.size) { "模型下载不完整，下次将续传" }
                    if (!valid(partial, spec)) {
                        partial.delete()
                        error("模型校验失败，请重试")
                    }
                } finally {
                    connection.disconnect()
                }
            }
            Files.move(partial.toPath(), target.toPath(), StandardCopyOption.REPLACE_EXISTING)
            completed += spec.size
            progress(completed)
        }
    }

    private fun sha256(file: File): String =
        // 统一走 PaFormat.sha256File,收敛见审查报告 util 节。
        sha256File(file)
}
