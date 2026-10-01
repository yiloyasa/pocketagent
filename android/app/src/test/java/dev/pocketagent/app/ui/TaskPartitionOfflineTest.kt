package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.TaskCenter
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 任务中心离线分区与失败"下一步"文案的单测(R3-C · 不确定时刻的诚实):
 *  1. 断线时消息类运行中任务归「离线待恢复」分区 —— 分区头**不写「正在执行」**
 *     (README:未连接时绝不显示,分区头也是渲染路径之一);
 *  2. 在线口径与既有四词分区完全同源(旧单参 taskPartitions 行为不变形);
 *  3. 每个失败态的下一步文案:原因之外给一条可执行的出路,禁责备、禁术语、禁含糊字眼。
 */
class TaskPartitionOfflineTest {

    private fun task(
        id: String,
        kind: TaskCenter.Kind,
        status: TaskCenter.Status,
    ): TaskCenter.Task = TaskCenter.Task(
        id = id, threadId = "thr-$id", kind = kind, title = "任务$id", status = status,
    )

    // ---- taskPartitionRows:断线分区 ----

    @Test
    fun `离线时消息类运行中归离线待恢复分区_头不写正在执行`() {
        val rows = taskPartitionRows(
            listOf(
                task("m", TaskCenter.Kind.MESSAGE, TaskCenter.Status.RUNNING),
                task("d", TaskCenter.Kind.DOWNLOAD, TaskCenter.Status.RUNNING),
            ),
            hostOffline = true,
        )

        assertEquals(2, rows.size)
        val off = rows.first { it.offlineResume }
        assertEquals("离线待恢复", off.word)
        assertEquals(listOf("m"), off.tasks.map { it.id })
        // 下载类是手机本机行为,断连也照常归「正在执行」分区
        val running = rows.first { !it.offlineResume }
        assertEquals("正在执行", running.word)
        assertEquals(listOf("d"), running.tasks.map { it.id })
        assertTrue("离线分区的头绝不能写「正在执行」", off.word != "正在执行")
    }

    @Test
    fun `在线时无离线分区_且与旧四词分区同源`() {
        val tasks = listOf(
            task("a", TaskCenter.Kind.MESSAGE, TaskCenter.Status.DONE),
            task("b", TaskCenter.Kind.MESSAGE, TaskCenter.Status.FAILED),
            task("c", TaskCenter.Kind.MESSAGE, TaskCenter.Status.RUNNING),
            task("d", TaskCenter.Kind.MESSAGE, TaskCenter.Status.CANCELED),
            task("e", TaskCenter.Kind.DOWNLOAD, TaskCenter.Status.RUNNING),
        )
        val rows = taskPartitionRows(tasks, hostOffline = false)

        assertTrue(rows.none { it.offlineResume })
        // 单参 taskPartitions(既有单测钉住的口径)与双参行完全同源
        assertEquals(
            taskPartitions(tasks),
            rows.map { editorialStatusOf(it.tasks.first().status) to it.tasks },
        )
    }

    @Test
    fun `断线时不影响下载类与已完成分区`() {
        val rows = taskPartitionRows(
            listOf(
                task("x", TaskCenter.Kind.DOWNLOAD, TaskCenter.Status.DONE),
                task("y", TaskCenter.Kind.MESSAGE, TaskCenter.Status.INTERRUPTED),
            ),
            hostOffline = true,
        )
        assertEquals(listOf("已完成", "未完成"), rows.map { it.word })
        assertTrue(rows.none { it.offlineResume })
    }

    // ---- failureNextStep:失败态的"下一步" ----

    @Test
    fun `消息类失败_断线时先引导恢复连接`() {
        assertEquals("点右侧重试,找回这次的结果", failureNextStep(TaskCenter.Kind.MESSAGE, false, null, true))
        assertEquals("恢复连接后点重试,找回这次的结果", failureNextStep(TaskCenter.Kind.MESSAGE, true, null, true))
    }

    @Test
    fun `下载类失败_按重新下载的连接依赖分流`() {
        // 电脑来源:断线时直说"现在取不了",不让人白点
        assertEquals(
            "点右侧重新下载,从电脑再取一次",
            failureNextStep(TaskCenter.Kind.DOWNLOAD, false, true, true),
        )
        assertEquals(
            "电脑连不上时取不了;恢复连接后点重新下载",
            failureNextStep(TaskCenter.Kind.DOWNLOAD, true, true, true),
        )
        // URL 来源:只靠手机有网,断的是电脑也照常可重试
        assertEquals(
            "点右侧重新下载,再取一次",
            failureNextStep(TaskCenter.Kind.DOWNLOAD, true, false, true),
        )
        // 无来源可重放:出路是回会话或移除,不给假重试
        assertEquals(
            "内容在会话里还在;可回会话重新保存,或移除这条记录",
            failureNextStep(TaskCenter.Kind.DOWNLOAD, false, null, true),
        )
        assertEquals(
            "这条记录没有可再取的文件,可移除",
            failureNextStep(TaskCenter.Kind.DOWNLOAD, false, null, false),
        )
    }

    // ---- missingFileNextStep:文件丢失态的"下一步" ----

    @Test
    fun `文件丢失_按可重取性给出路`() {
        assertEquals(
            "文件已不在手机上;可点重新下载再取一次(需要电脑在线)",
            missingFileNextStep(true, true),
        )
        assertEquals(
            "文件已不在手机上;可点重新下载再取一次",
            missingFileNextStep(false, false),
        )
        assertEquals(
            "文件已不在手机上;可回会话重新保存,或移除这条记录",
            missingFileNextStep(null, true),
        )
        assertEquals(
            "文件已不在手机上;这条记录可移除",
            missingFileNextStep(null, false),
        )
        // 断线时需要电脑的重取不给入口:文案与行尾动作一致,引导回头再来
        assertEquals(
            "文件已不在手机上;恢复连接后可重新下载,或先移除这条记录",
            missingFileNextStep(true, true, hostOffline = true),
        )
    }

    // ---- 文案纪律:全矩阵禁含糊字眼、禁责备语气 ----

    @Test
    fun `全部下一步文案不含禁词且非空白`() {
        val samples = buildList {
            for (kind in listOf(TaskCenter.Kind.MESSAGE, TaskCenter.Kind.DOWNLOAD)) {
                for (offline in listOf(true, false)) {
                    for (pc in listOf(true, false, null)) {
                        for (src in listOf(true, false)) {
                            add(failureNextStep(kind, offline, pc, src))
                            add(missingFileNextStep(pc, src))
                        }
                    }
                }
            }
        }
        assertTrue(samples.isNotEmpty())
        samples.forEach { text ->
            assertTrue("文案不得为空白: $text", text.isNotBlank())
            assertFalse("用户可见文案不得含「未知」: $text", text.contains("未知"))
            assertFalse("用户可见文案不得含「错误」: $text", text.contains("错误"))
            // 责备语气红线:不出现指责任何人的措辞
            assertFalse("文案不得指责用户操作: $text", text.contains("你删") || text.contains("你误"))
        }
    }
}
