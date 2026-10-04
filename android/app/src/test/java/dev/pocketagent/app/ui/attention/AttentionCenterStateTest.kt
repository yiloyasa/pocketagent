package dev.pocketagent.app.ui.attention

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import dev.pocketagent.app.ui.relativeTimeLabel

/**
 * Attention Center(Task 5)纯函数的单测:归档四档、筛选、最高优先级、离线时间戳。
 *
 * 【钉哪些钉子】
 *  - 分档口径:RETRY_LATER/IGNORED 归「已忽略」而非「需要我处理」(不催办用户已
 *    婉拒的事);仅供参考的新通知不进四档;已完成/已忽略永远当不了"最高优先级";
 *  - OfflineStore 纪律:离线必须标「本地缓存 · 时刻」,在线不摆字,绝不伪装实时。
 */
class AttentionCenterStateTest {

    private fun unknownEvent(
        id: String,
        requiresUser: Boolean = false,
        status: String = "NEW",
        at: Long = 1_000L,
    ) = AttentionItem.UnknownEvent(
        id = id,
        title = "电脑端新消息 $id",
        detail = "不影响正在进行的任务",
        threadId = "",
        requiresUser = requiresUser,
        processingStatus = status,
        receivedAtUnixMs = at,
    )

    private fun approval(id: String, count: Int = 1) = AttentionItem.Approval(
        id = id, title = "会话 $id", detail = "Codex · $count 项等待你处理",
        actionLabel = "去处理", pendingCount = count,
    )

    private fun recovery() = AttentionItem.Recovery(
        id = "recovery", title = "启动降级:有排队输入丢失", detail = "上次退出有 2 句输入没跑到",
        actionLabel = "去查看",
    )

    private fun collab(needsAction: Boolean) = AttentionItem.Collab(
        id = "collab", title = "多 Agent 协同", detail = "已接入 2 家", needsAction = needsAction,
    )

    // ---- 归档四档 ----------------------------------------------------------------

    @Test
    fun `未知事件按状态与requiresUser分档`() {
        assertEquals(AttentionBucket.NEEDS_ME, attentionBucketOf(unknownEvent("a", requiresUser = true)))
        assertEquals(AttentionBucket.DONE, attentionBucketOf(unknownEvent("b", status = "INSPECTED")))
        assertEquals(AttentionBucket.DONE, attentionBucketOf(unknownEvent("c", status = "ESCALATED")))
        assertEquals(AttentionBucket.DONE, attentionBucketOf(unknownEvent("d", status = "NEEDS_NATIVE_SUPPORT")))
        assertEquals(AttentionBucket.IGNORED, attentionBucketOf(unknownEvent("e", status = "IGNORED")))
        // 稍后再看 = 用户明确说暂不处理:归已忽略,不回「需要我处理」催办。
        assertEquals(AttentionBucket.IGNORED, attentionBucketOf(unknownEvent("f", status = "RETRY_LATER", requiresUser = true)))
    }

    @Test
    fun `仅供参考的新通知不进四档`() {
        // NEW + requires_user=false 是"仅供参考"(不催办也不属于运行/完成):
        // 硬塞任何一档都是错误信号,完整清单仍在「电脑端新消息」弹窗。
        assertNull(attentionBucketOf(unknownEvent("g", requiresUser = false)))
    }

    @Test
    fun `daemon未来新增的处置态按已处理归档`() {
        // 客户端不认识的新处置态:已处理(失败安全方向,详见 attentionBucketOf 注释)。
        assertEquals(AttentionBucket.DONE, attentionBucketOf(unknownEvent("h", status = "SOMETHING_NEW")))
    }

    @Test
    fun `审批与恢复归需要我处理_协同按needsAction分档`() {
        assertEquals(AttentionBucket.NEEDS_ME, attentionBucketOf(approval("t1")))
        assertEquals(AttentionBucket.NEEDS_ME, attentionBucketOf(recovery()))
        assertEquals(AttentionBucket.NEEDS_ME, attentionBucketOf(collab(needsAction = true)))
        assertEquals(AttentionBucket.RUNNING, attentionBucketOf(collab(needsAction = false)))
    }

    // ---- 筛选与排序 ----------------------------------------------------------------

    @Test
    fun `筛选只出对应档_未知事件按时间倒序`() {
        val state = AttentionCenterState(
            items = listOf(
                unknownEvent("newest", requiresUser = true, at = 9_000L),
                unknownEvent("oldest", requiresUser = true, at = 1_000L),
                unknownEvent("mid", requiresUser = true, at = 5_000L),
                unknownEvent("notice", requiresUser = false, at = 7_000L),
                unknownEvent("done", status = "INSPECTED", at = 8_000L),
                approval("t1"),
            ),
            filter = AttentionFilter.NEEDS_ME,
            offline = false,
            cachedAtUnixMs = 0,
        )
        val shown = filterAttention(state)
        // 仅供参考(notice)与已完成(done)不出现在「需要我处理」档。
        assertEquals(listOf("t1", "newest", "mid", "oldest"), shown.map { it.id })
    }

    @Test
    fun `四档筛选各取各档`() {
        val items = listOf(
            unknownEvent("u-need", requiresUser = true),
            unknownEvent("u-done", status = "INSPECTED"),
            unknownEvent("u-ignored", status = "RETRY_LATER"),
            collab(needsAction = false),
        )
        fun shown(filter: AttentionFilter) =
            filterAttention(AttentionCenterState(items = items, filter = filter))
                .map { it.id }
        assertEquals(listOf("u-need"), shown(AttentionFilter.NEEDS_ME))
        assertEquals(listOf("collab"), shown(AttentionFilter.RUNNING))
        assertEquals(listOf("u-done"), shown(AttentionFilter.DONE))
        assertEquals(listOf("u-ignored"), shown(AttentionFilter.IGNORED))
    }

    // ---- 最高优先级摘要(首页一行)---------------------------------------------------

    @Test
    fun `最高优先级_审批压过未知事件压过运行中`() {
        val top = topAttentionSummary(
            listOf(
                unknownEvent("u", requiresUser = true),
                recovery(),
                approval("t1"),
                collab(needsAction = false),
            )
        )
        assertTrue(top is AttentionItem.Approval)
    }

    @Test
    fun `最高优先级_没有待办与运行时返回null而不是拿档案凑数`() {
        // 已完成/已忽略是档案,永远不该被当成"最高优先级"摘要推到首页。
        val archived = listOf(
            unknownEvent("done", status = "INSPECTED"),
            unknownEvent("ignored", status = "IGNORED"),
        )
        assertNull(topAttentionSummary(archived))
        assertNull(topAttentionSummary(emptyList()))
    }

    @Test
    fun `需要我处理计数只数NEEDS_ME档`() {
        assertEquals(
            3,
            attentionNeedsMeCount(
                listOf(
                    approval("t1"),
                    unknownEvent("u", requiresUser = true),
                    recovery(),
                    unknownEvent("notice", requiresUser = false),
                    collab(needsAction = false),
                )
            )
        )
    }

    // ---- 离线时间戳(OfflineStore 纪律:不伪装实时)-----------------------------------

    private val now = 10_000_000_000L

    @Test
    fun `在线不摆任何缓存字样`() {
        assertNull(attentionFreshnessLabel(offline = false, cachedAtUnixMs = now - 60_000, nowMs = now))
    }

    @Test
    fun `离线标注本地缓存与相对时刻`() {
        val label = attentionFreshnessLabel(offline = true, cachedAtUnixMs = now - 3 * 60_000, nowMs = now)!!
        // 相对时间口径与 relativeTimeLabel 同源(同一份纯函数,不另算一套)。
        assertEquals("本地缓存 · ${relativeTimeLabel(now - 3 * 60_000, now)}", label)
        assertEquals("本地缓存 · 3 分钟前", label)
    }

    @Test
    fun `离线且从未加载成功只说本地缓存不编时刻`() {
        val label = attentionFreshnessLabel(offline = true, cachedAtUnixMs = 0, nowMs = now)!!
        assertEquals("本地缓存", label)
        assertFalse(label.contains("分钟前"))
    }

    @Test
    fun `缓存时刻在未来回落绝对日期仍以本地缓存开头`() {
        val label = attentionFreshnessLabel(offline = true, cachedAtUnixMs = now + 60_000, nowMs = now)!!
        assertTrue(label.startsWith("本地缓存"))
    }
}
