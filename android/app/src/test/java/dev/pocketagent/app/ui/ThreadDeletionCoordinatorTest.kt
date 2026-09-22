package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.ThreadSnapshot
import dev.pocketagent.app.ui.component.TurnCheckpointStore
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

/**
 * 删除会话时的本地状态清理(方案 §4.2 / §8-D2)。
 *
 * 【钉住的根因】删除流程此前只清了列表快照 threads.json,而**消息快照**与**轮次
 * checkpoint** 各是独立文件,模型/强度/用量要等 `pruneStalePrefs` 的 7 天延迟回收。
 * 于是"删了会话"在本地是不完整的:断网兜底仍能把整段对话读回来,磁盘上还留着消息
 * 正文。这两处此前**根本没有删除入口**,是本组用例存在的理由。
 *
 * 【为什么只测 purgeDisk】偏好类(模型/强度/用量/置顶)要 Android Context,纯 JVM
 * 起不来;而磁盘类只需要一个目录 —— 那恰好是缺陷所在的那一半。
 */
class ThreadDeletionCoordinatorTest {

    @get:Rule
    val tmp = TemporaryFolder()

    private lateinit var dir: File

    @Before
    fun setUp() {
        dir = tmp.newFolder("purge")
        ThreadSnapshot.initForTest(dir)
        TurnCheckpointStore.initForTest(tmp.newFolder("ckpt"))
    }

    private fun row(id: String) = ThreadSnapshot.Row(id, "会话$id", 1L, "/w", "", false)

    private fun seedCheckpoint(id: String) {
        TurnCheckpointStore.save(
            TurnCheckpointStore.Checkpoint(
                threadId = id,
                messages = listOf(ThreadSnapshot.Msg("user", "checkpoint 里的消息正文")),
                stream = "", thinking = "", sequence = 1L, active = true, queued = 0,
                model = "codex/default", effort = null, error = null,
            )
        )
    }

    /** 删除一条会话后,列表快照里不能再有它。 */
    @Test
    fun purgesListSnapshotRow() {
        ThreadSnapshot.saveThreadsSnapshot(listOf(row("a"), row("b")))

        ThreadDeletionCoordinator.purgeDisk("a")

        assertEquals(listOf("b"), ThreadSnapshot.readThreadsSnapshot().map { it.id })
    }

    /** 消息快照是**独立文件**,必须单独清 —— 它就是"删了还能翻出对话"那条路。 */
    @Test
    fun purgesMessageSnapshot() {
        ThreadSnapshot.saveMessages("a", listOf(ThreadSnapshot.Msg("user", "离线也能看到我")))
        assertNotNull("前置:消息快照应已落盘", ThreadSnapshot.readThreadSnapshot("a"))

        ThreadDeletionCoordinator.purgeDisk("a")

        assertNull("删除后消息快照必须读不出来", ThreadSnapshot.readThreadSnapshot("a"))
    }

    /** checkpoint 存着完整消息副本,同样必须清。 */
    @Test
    fun purgesTurnCheckpoint() {
        seedCheckpoint("a")
        assertNotNull("前置:checkpoint 应已落盘", TurnCheckpointStore.load("a"))

        ThreadDeletionCoordinator.purgeDisk("a")

        assertNull("删除后 checkpoint 必须读不出来", TurnCheckpointStore.load("a"))
    }

    /** 只清这一条:别的会话的缓存不能被误伤(误删用户数据比残留更糟)。 */
    @Test
    fun leavesOtherThreadsUntouched() {
        ThreadSnapshot.saveThreadsSnapshot(listOf(row("a"), row("b")))
        ThreadSnapshot.saveMessages("b", listOf(ThreadSnapshot.Msg("user", "B 的内容")))
        seedCheckpoint("b")

        ThreadDeletionCoordinator.purgeDisk("a")

        assertEquals(listOf("b"), ThreadSnapshot.readThreadsSnapshot().map { it.id })
        assertNotNull("B 的消息快照不该被动", ThreadSnapshot.readThreadSnapshot("b"))
        assertNotNull("B 的 checkpoint 不该被动", TurnCheckpointStore.load("b"))
    }

    /** 幂等:删一个不存在的会话不该抛 —— 删除的目标状态就是"没有",达成即成功。 */
    @Test
    fun purgeIsIdempotent() {
        ThreadDeletionCoordinator.purgeDisk("zzz-never-existed")
        ThreadDeletionCoordinator.purgeDisk("zzz-never-existed")
        ThreadDeletionCoordinator.purgeDisk("")
    }
}
