package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * [TaskCenter.trackDownload] 对已结束条目「原地复活」的单测(R3-C · 结论 3):
 * 失败/已完成的下载点「重新下载」后,条目必须立刻回到进行中并清掉旧进度与旧文件
 * 位置 —— 任务中心当场给出"在动"的反馈,而不是一条纹丝不动的死记录;
 * 已在下载中的同一条重复登记则**不**重置(进度条不许被打回零再爬)。
 *
 * 注:JVM 单测里 TaskCenter 未 init(prefs=null),persist 只做内存发射,不碰磁盘。
 */
class TaskRedownloadResetTest {

    /** 每个用例用独立 fileKey,避免单例任务清单在用例间串扰。 */
    private fun key() = "r3c-reset-" + System.nanoTime()

    @Test
    fun `重新下载把已结束条目原地复活为进行中并清旧位置`() {
        val k = key()
        val id = TaskCenter.trackDownload(k, "报告.pdf", "thr-1")
        TaskCenter.finishDownload(
            id, ok = true, detail = "已保存到「下载/PocketAgent/报告.pdf」",
            localUri = "content://media/external/file/1", sizeText = "1.0MB",
        )
        assertEquals(TaskCenter.Status.DONE, TaskCenter.tasks.value.first { it.id == id }.status)

        val again = TaskCenter.trackDownload(k, "报告.pdf", "thr-1")

        assertEquals("同一文件复用同一条目,不另起一行", id, again)
        val t = TaskCenter.tasks.value.first { it.id == id }
        assertEquals(TaskCenter.Status.RUNNING, t.status)
        assertEquals(0f, t.progress, 1e-6f)
        assertEquals("", t.detail)
        assertEquals("", t.localUri)
        assertEquals("", t.sizeText)

        TaskCenter.remove(id)
    }

    @Test
    fun `进行中的同一条重复登记不重置进度`() {
        val k = key()
        val id = TaskCenter.trackDownload(k, "a.bin", "")
        TaskCenter.updateDownload(id, 0.5f)

        TaskCenter.trackDownload(k, "a.bin", "")

        val t = TaskCenter.tasks.value.first { it.id == id }
        assertEquals(TaskCenter.Status.RUNNING, t.status)
        assertEquals("重复登记不得把进度打回零", 0.5f, t.progress, 1e-6f)

        TaskCenter.remove(id)
    }

    @Test
    fun `失败的条目同样可复活`() {
        val k = key()
        val id = TaskCenter.trackDownload(k, "pkg.apk", "thr-2")
        TaskCenter.finishDownload(id, ok = false, detail = "下载失败:传输不完整 1.0MB / 2.0MB")
        assertEquals(TaskCenter.Status.FAILED, TaskCenter.tasks.value.first { it.id == id }.status)

        TaskCenter.trackDownload(k, "pkg.apk", "thr-2")

        val t = TaskCenter.tasks.value.first { it.id == id }
        assertEquals(TaskCenter.Status.RUNNING, t.status)
        assertEquals(0f, t.progress, 1e-6f)

        TaskCenter.remove(id)
    }
}
