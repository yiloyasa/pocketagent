package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * TaskCenter 持久化文本解析(纯函数):
 * - 新格式 13 字段:全字段回读(localUri/sourceTitle/sizeText);
 * - 旧格式 10 字段:不丢行,新字段为空串 —— UI 据此进入「按文件名找回本地路径」兜底。
 *
 * 注:分隔符是 0x01 控制字符,源码用 Char(1) 构造,不写反斜杠 u 转义。
 */
class TaskCenterParseTest {

    private val sep = Char(1).toString()

    @Test
    fun parsesNewFormatAllFields() {
        val line = listOf(
            "t1", "thr9", "DOWNLOAD", "报告.docx", "DONE", "1",
            "1700000000000", "0.75", "ok", "2",
            "content://media/external/file/42", "周会纪要", "1.2MB",
        ).joinToString(sep)

        val tasks = TaskCenter.parseTasks(line)

        assertEquals(1, tasks.size)
        val t = tasks[0]
        assertEquals("t1", t.id)
        assertEquals("thr9", t.threadId)
        assertEquals(TaskCenter.Kind.DOWNLOAD, t.kind)
        assertEquals("报告.docx", t.title)
        assertEquals(TaskCenter.Status.DONE, t.status)
        assertTrue(t.read)
        assertEquals(1700000000000L, t.startedAt)
        assertEquals(0.75f, t.progress, 1e-6f)
        assertEquals("ok", t.detail)
        assertEquals(2, t.rounds)
        assertEquals("content://media/external/file/42", t.localUri)
        assertEquals("周会纪要", t.sourceTitle)
        assertEquals("1.2MB", t.sizeText)
    }

    @Test
    fun parsesLegacyTenFieldLineWithoutNewColumns() {
        val line = listOf(
            "t2", "thr1", "DOWNLOAD", "报告.doc", "DONE", "0",
            "1699999999999", "1.0", "", "1",
        ).joinToString(sep)

        val tasks = TaskCenter.parseTasks(line)

        assertEquals(1, tasks.size)
        val t = tasks[0]
        assertEquals("t2", t.id)
        assertEquals("报告.doc", t.title)
        assertEquals(TaskCenter.Status.DONE, t.status)
        // 旧格式没有 11-13 字段 → 空串(这正是旧条目没有查看/分享图标的根因)
        assertEquals("", t.localUri)
        assertEquals("", t.sourceTitle)
        assertEquals("", t.sizeText)
    }

    @Test
    fun runningRestoredAsInterruptedAndShortLinesDropped() {
        val running = listOf(
            "t3", "", "MESSAGE", "继续吗?", "RUNNING", "0", "170", "0", "", "1",
        ).joinToString(sep)

        val tasks = TaskCenter.parseTasks("$running\n只有三个${sep}字段\n\n")

        assertEquals(1, tasks.size)
        // 上次进程被杀时还在跑的任务 → 重启后标记为已中断
        assertEquals(TaskCenter.Status.INTERRUPTED, tasks[0].status)
    }
}
