package dev.pocketagent.app.ui

import java.io.File
import org.junit.Assert.assertTrue

/** 从模块目录或仓库根向上定位指定源码文件(镜像/仓库两处布局都兼容)。 */
internal fun locate(vararg relCandidates: String): File {
    var dir: File? = File(System.getProperty("user.dir") ?: ".").absoluteFile
    repeat(6) {
        val cur = dir ?: return@repeat
        relCandidates.forEach { rel ->
            File(cur, rel).takeIf { it.isFile }?.let { return it }
            File(cur, "android/app/$rel").takeIf { it.isFile }?.let { return it }
        }
        dir = cur.parentFile
    }
    error("找不到源码文件 $relCandidates(从 ${System.getProperty("user.dir")} 向上查 6 层)")
}

/** 截取 [startMarker, endMarker) 之间的源码段。 */
internal fun section(text: String, startMarker: String, endMarker: String): String {
    val start = text.indexOf(startMarker)
    assertTrue("区域起始标记应存在: $startMarker", start >= 0)
    val end = text.indexOf(endMarker, start)
    assertTrue("区域结束标记应存在于起始之后: $endMarker", end > start)
    return text.substring(start, end)
}
