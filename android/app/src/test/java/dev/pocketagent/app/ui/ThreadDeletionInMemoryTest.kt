package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.TaskCenter
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 删除会话时一并清掉"按 threadId 索引"的内存态(用户明确要求一起删)。
 *
 * 【为什么它们也在删除范围内】这两处都不是"这条会话还在不在"的权威副本,但都按
 * threadId 索引:任务中心的"消息类"条目 id 就是 `msg:$threadId`,会话删了它还在,
 * 用户会在任务中心看到一条永远点不开的记录;子代理预览按 threadId 缓存,同一 id 的
 * 会话再建出来时会命中旧预览,显示上一世的内容。
 *
 * 【覆盖边界如实说明】[SubAgentsState.forget] 的 previews 是私有 ConcurrentHashMap,
 * 唯一写入路径 requestPreview 是**异步拉网络**的,纯 JVM 测不了;为它加一个仅测试用的
 * 写入挂钩会给生产 API 多开一个口子,不值当 —— 它的实现是一行 Map.remove,风险由
 * 代码审查而非测试兜。
 */
class ThreadDeletionInMemoryTest {

    @Test
    fun `任务中心随会话删除清掉该会话的条目`() {
        TaskCenter.trackMessage("th-del-x1", "要删的会话")
        TaskCenter.trackMessage("th-keep-x1", "留着的会话")
        assertTrue(
            "前置:被删会话的条目应已在任务中心",
            TaskCenter.messages().any { it.threadId == "th-del-x1" },
        )

        ThreadDeletionCoordinator.purgeInMemory("th-del-x1")

        val left = TaskCenter.messages().map { it.threadId }
        assertFalse("被删会话的条目必须消失,否则任务中心留着一条点不开的记录", left.contains("th-del-x1"))
        assertTrue("别的会话的条目不该被误伤", left.contains("th-keep-x1"))
    }

    @Test
    fun `清一个没有条目的会话是幂等的`() {
        // 删除的目标状态就是"没有";调用两次、以及传空串都不该抛。
        ThreadDeletionCoordinator.purgeInMemory("th-never-tracked-x9")
        ThreadDeletionCoordinator.purgeInMemory("th-never-tracked-x9")
        ThreadDeletionCoordinator.purgeInMemory("")
    }

    @Test
    fun `会话删除释放 TurnSession 内存态并关掉恢复信号`() {
        val first = TurnSession.state("th-del-x2")
        // tryReceive().isClosed 是不开挂的关闭探测:open+空缓冲=false,close 后=true。
        assertFalse("前置:未删除时恢复信号通道应是打开的", first.recoverySignal.tryReceive().isClosed)

        ThreadDeletionCoordinator.purgeInMemory("th-del-x2")

        assertTrue(
            "被删会话的恢复信号通道必须关闭 —— 在等的恢复循环据此立即退出,不再为已删除会话补事件",
            first.recoverySignal.tryReceive().isClosed,
        )
        assertTrue(
            "同 id 再建必须拿到全新 State,不得命中上一世的旧状态",
            TurnSession.state("th-del-x2") !== first,
        )
    }
}
