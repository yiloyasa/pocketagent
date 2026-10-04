package dev.pocketagent.app.ui.component

import java.io.File
import java.security.MessageDigest
import java.util.Locale

/**
 * 纯格式化/哈希工具的统一实现（审查报告 docs/reviews/2026-10-04 第七节 util 收敛项）。
 *
 * 【为什么收这里】sha256 文件校验×2（SenseVoiceModelFiles/SenseVoiceModelManager）、
 * 目录递归求和×3（PaCache 同文件两份 + OfflineStore）、MD5 缓存键×3
 * （PaCache/OfflineStore/ThreadSnapshot）、字节格式化×2（RuntimeStateLogic/FilesListLogic）
 * 此前散落各处、口径一致却各自为政。收敛后非测试源码里这些实现只剩本文件一份，
 * 各调用方保留原函数名做薄委托（零行为变更）。全部为不持 Context、不碰 UI 的纯函数。
 */

/** 流式计算文件 SHA-256，返回小写十六进制（取 SenseVoiceModelFiles 版语义：64KB 缓冲逐块喂摘要）。 */
internal fun sha256File(file: File): String {
    val digest = MessageDigest.getInstance("SHA-256")
    file.inputStream().use { input ->
        val buffer = ByteArray(64 * 1024)
        while (true) {
            val n = input.read(buffer)
            if (n < 0) break
            digest.update(buffer, 0, n)
        }
    }
    return digest.digest().joinToString("") { "%02x".format(it.toInt() and 255) }
}

/** 目录递归占用（字节）；目录不存在或不可读按 0 计（与三份历史实现同语义）。 */
internal fun dirSizeRecursive(dir: File): Long {
    val files = dir.listFiles() ?: return 0
    var total = 0L
    for (f in files) total += if (f.isDirectory) dirSizeRecursive(f) else f.length()
    return total
}

/** 缓存键哈希：任意字符串 → MD5 → 32 位小写十六进制（三处历史缓存键同款输出形态）。 */
internal fun stableHashMd5(s: String): String {
    val digest = MessageDigest.getInstance("MD5").digest(s.toByteArray(Charsets.UTF_8))
    // Byte 有符号：直接 %02x 会得到 ffffff80 这种，必须掩成无符号
    return digest.joinToString("") { "%02x".format(it.toInt() and 0xff) }
}

/** 字节额度的人话呈现(daemon 侧按 1<<20 计的 MB,标签沿用同一口径避免换算假象)。 */
internal fun formatBytesLabel(bytes: Long): String {
    val abs = if (bytes < 0) -bytes else bytes
    val unit: String
    val div: Long
    when {
        abs >= 1L shl 30 -> { unit = "GB"; div = 1L shl 30 }
        abs >= 1L shl 20 -> { unit = "MB"; div = 1L shl 20 }
        abs >= 1L shl 10 -> { unit = "KB"; div = 1L shl 10 }
        else -> { unit = "B"; div = 1L }
    }
    if (div == 1L) return "$bytes $unit"
    val v = bytes.toDouble() / div
    return if (v == Math.floor(v)) "${v.toLong()} $unit"
    else String.format(Locale.US, "%.1f %s", v, unit)
}
