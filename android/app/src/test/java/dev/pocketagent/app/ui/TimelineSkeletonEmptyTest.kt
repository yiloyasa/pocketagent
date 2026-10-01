package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 时间线首载骨架 / 空会话空态判定的单测(R1 残项收口:hermes 等会话首次进入
 * 旧实现整屏空白 —— 在途像卡死,空历史没有交代)。骨架与空态互斥且都只在
 * "数据真实处于那一态"时出现,绝不盖住错误横幅或离线副本。
 */
class TimelineSkeletonEmptyTest {

    // ---- shouldShowTimelineSkeleton:首次历史在途 ----

    @Test
    fun `首载在途_空时间线无错无副本_显示骨架`() {
        assertTrue(
            shouldShowTimelineSkeleton(
                itemsEmpty = true, historyLoaded = false,
                hasLoadError = false, offlineSnapshot = false,
            ),
        )
    }

    @Test
    fun `骨架_四个否定条件各各排除`() {
        assertFalse("有内容不摆", shouldShowTimelineSkeleton(false, false, false, false))
        assertFalse("历史已加载完不摆", shouldShowTimelineSkeleton(true, true, false, false))
        assertFalse("报错时不摆_横幅已交代", shouldShowTimelineSkeleton(true, false, true, false))
        assertFalse("离线副本不摆_副本本身就是内容", shouldShowTimelineSkeleton(true, false, false, true))
    }

    // ---- shouldShowTimelineEmptyState:确实一条都没有 ----

    @Test
    fun `加载完且为空_干净态_显示空态`() {
        assertTrue(
            shouldShowTimelineEmptyState(
                itemsEmpty = true, historyLoaded = true, hasLoadError = false,
                offlineSnapshot = false, running = false, recovering = false,
            ),
        )
    }

    @Test
    fun `空态_未加载完_不显示_那是骨架的事`() {
        assertFalse(
            shouldShowTimelineEmptyState(true, historyLoaded = false, hasLoadError = false, offlineSnapshot = false, running = false, recovering = false),
        )
    }

    @Test
    fun `空态_运行中或恢复中不显示_轮次帧马上会来`() {
        assertFalse(
            shouldShowTimelineEmptyState(true, true, false, false, running = true, recovering = false),
        )
        assertFalse(
            shouldShowTimelineEmptyState(true, true, false, false, running = false, recovering = true),
        )
    }

    @Test
    fun `空态_报错与离线副本不显示`() {
        assertFalse(
            shouldShowTimelineEmptyState(true, true, hasLoadError = true, offlineSnapshot = false, running = false, recovering = false),
        )
        assertFalse(
            shouldShowTimelineEmptyState(true, true, false, offlineSnapshot = true, running = false, recovering = false),
        )
    }

    @Test
    fun `空态_有内容不显示`() {
        assertFalse(
            shouldShowTimelineEmptyState(itemsEmpty = false, historyLoaded = true, hasLoadError = false, offlineSnapshot = false, running = false, recovering = false),
        )
    }

    // ---- timelineEmptyStateCopy ----

    @Test
    fun `空态文案_带上agent名_在跟谁说话可辨`() {
        val (head, sub) = timelineEmptyStateCopy("Hermes")
        assertEquals("还没有消息", head)
        assertTrue(sub.contains("Hermes"))
        assertTrue(sub.contains("发第一条消息"))
    }

    @Test
    fun `空态文案_归属未知退中性agent_不冒名`() {
        val (_, sub) = timelineEmptyStateCopy("")
        assertTrue(sub.contains("agent"))
        assertFalse(sub.contains("发第一条消息，和  ")) // 不出双空格的破句
    }

    @Test
    fun `空态文案_无禁词`() {
        val (head, sub) = timelineEmptyStateCopy("Codex")
        listOf(head, sub).forEach {
            assertFalse(it.contains("未知"))
            assertFalse(it.contains("失败"))
            assertFalse(it.contains("错误"))
        }
    }
}
