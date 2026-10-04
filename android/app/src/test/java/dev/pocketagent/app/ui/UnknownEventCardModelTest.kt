package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.UnknownEventSummary
import pocketagent.v1.UserActionDescriptor

/**
 * 未知事件卡片模型层(2026-10-04 用户处理中心计划 Task 4)的单测。
 *
 * 【为什么这一层必须钉死】它是 daemon UnknownEventSummary(含 12~17 人话字段)到
 * 手机卡片的唯一转换点,三个静默错误只有单测能拦:
 *  - 旧 daemon 没有新字段时把空值"拼成"看起来像 daemon 给的内容(评审焦点红线);
 *  - copy 动作被当成远端动作发往 daemon(daemon 执行器会拒绝,红线是手机本地剪贴板);
 *  - 处置过的事件(requires_user=true 但已是 INSPECTED 等)被反复归进「需要你处理」催办。
 */
class UnknownEventCardModelTest {

    /** 新 daemon 的完整摘要(人话字段 12~17 齐备,动作表五个白名单动作)。 */
    private fun newDaemonSummary(): UnknownEventSummary = UnknownEventSummary.newBuilder()
        .setEventId("evt-1")
        .setSequence(7)
        .setReceivedAtUnixMs(1_756_000_000_000)
        .setSourceAgent("codex")
        .setSourceMethod("item/weird/thing/started")
        .setThreadId("thread-9")
        .setNormalizedKind("UNKNOWN_REQUEST")
        .setSafetyClass("UNKNOWN")
        .setProcessingStatus("NEW")
        .setRevision(3)
        .setSummary("电脑端请求了一个当前版本无法安全判断的操作")
        .setPlainTitle("电脑端请求了一个操作：item/weird/thing/started")
        .setPlainExplanation("电脑端请求了一个当前版本无法安全判断的操作，因此没有自动执行。")
        .setImpact("该操作未执行；如涉及你关心的任务，请在电脑端处理")
        .setRequiresUser(true)
        .setCanOpenNative(true)
        .addActions(action("inspect", "查看"))
        .addActions(action("copy", "复制"))
        .addActions(action("acknowledge", "标记已查看"))
        .addActions(action("retry_later", "待上游支持"))
        .addActions(action("open_native", "在电脑端处理"))
        .build()

    private fun action(id: String, label: String): UserActionDescriptor =
        UserActionDescriptor.newBuilder()
            .setActionId(id)
            .setLabel(label)
            .setDescription("说明:$id")
            .setSafe(true)
            .build()

    // ---- 人话字段映射 ------------------------------------------------------------

    @Test
    fun `人话字段一比一映射不添不改`() {
        val card = unknownEventCardModel(newDaemonSummary())
        // 全部照 daemon 原文带出:标题/解释/影响不经过客户端改写。
        assertEquals("电脑端请求了一个操作：item/weird/thing/started", card.title)
        assertEquals("电脑端请求了一个当前版本无法安全判断的操作，因此没有自动执行。", card.explanation)
        assertEquals("该操作未执行；如涉及你关心的任务，请在电脑端处理", card.impact)
        assertTrue(card.requiresUser)
        assertTrue(card.technicalAvailable)
        assertFalse(card.degraded)
        assertEquals("NEW", card.status)
        assertEquals("codex", card.sourceAgent)
        assertEquals("item/weird/thing/started", card.sourceMethod)
        assertEquals(3L, card.revision)
        assertEquals("thread-9", card.threadId)
        assertEquals("evt-1", card.eventId)
    }

    @Test
    fun `动作表一比一映射且路由分类正确`() {
        val card = unknownEventCardModel(newDaemonSummary())
        assertEquals(5, card.actions.size)
        // copy = 手机本地剪贴板;inspect = 本端展开详情;其余(含未来新白名单动作)一律回传 daemon。
        assertEquals(UnknownActionRoute.EXPAND_DETAIL, card.actions.first { it.actionId == "inspect" }.route)
        assertEquals(UnknownActionRoute.LOCAL_COPY, card.actions.first { it.actionId == "copy" }.route)
        assertEquals(UnknownActionRoute.REMOTE, card.actions.first { it.actionId == "acknowledge" }.route)
        assertEquals(UnknownActionRoute.REMOTE, card.actions.first { it.actionId == "retry_later" }.route)
        assertEquals(UnknownActionRoute.REMOTE, card.actions.first { it.actionId == "open_native" }.route)
        // label 是 daemon 给的人话,照抄不翻译。
        assertEquals("在电脑端处理", card.actions.first { it.actionId == "open_native" }.label)
    }

    @Test
    fun `不安全动作保留safe标记让UI拒绝渲染`() {
        val summary = newDaemonSummary().toBuilder()
            .addActions(
                UserActionDescriptor.newBuilder()
                    .setActionId("approve").setLabel("允许").setSafe(false).build()
            )
            .build()
        val flagged = unknownEventCardModel(summary).actions.first { it.actionId == "approve" }
        // 模型层原样保留 daemon 的声明;渲染层(只有 safe=true 才可点)据此拒绝 ——
        // 模型不吞掉标记,渲染不发明按钮,两侧职责分开。
        assertFalse(flagged.safe)
        assertEquals(UnknownActionRoute.REMOTE, flagged.route)
    }

    // ---- 旧 daemon 降级 ------------------------------------------------------------

    @Test
    fun `旧daemon无人话字段时降级为方法名原文加事实说明`() {
        // 旧 daemon 没有 12~17 字段:标题 = 方法名原文(不猜),解释/影响为空,
        // 动作表为空(daemon 没给动作,客户端绝不造按钮)。
        val oldDaemon = UnknownEventSummary.newBuilder()
            .setEventId("evt-old")
            .setSourceAgent("pi")
            .setSourceMethod("session/odd/update")
            .setNormalizedKind("UNKNOWN_NOTIFICATION")
            .setProcessingStatus("NEW")
            .setRevision(0)
            .build()
        val card = unknownEventCardModel(oldDaemon)
        assertTrue(card.degraded)
        assertFalse(card.technicalAvailable)
        assertEquals("session/odd/update", card.title)
        assertEquals("", card.explanation)
        assertEquals("", card.impact)
        assertTrue(card.actions.isEmpty())
        // 降级不催办:字段读不全的事件不该把用户拉进「需要你处理」。
        assertFalse(card.requiresUser)
    }

    @Test
    fun `降级且方法名缺席时标题回落归类人话`() {
        val card = unknownEventCardModel(
            UnknownEventSummary.newBuilder().setEventId("evt-2").setNormalizedKind("UNKNOWN_REQUEST").build()
        )
        assertTrue(card.degraded)
        assertEquals("新请求", card.title)
    }

    @Test
    fun `降级优先于requiresUser不因半套字段催办`() {
        // 病态/半套字段(无 plain_title 但 requires_user=true):按降级处理,不催办。
        val odd = newDaemonSummary().toBuilder().clearPlainTitle().build()
        val card = unknownEventCardModel(odd)
        assertTrue(card.degraded)
        assertFalse(card.requiresUser)
    }

    // ---- 动作执行请求组装(CAS + copy 封死)------------------------------------------

    @Test
    fun `copy动作不组装请求_绝不发往daemon`() {
        val copy = unknownEventActionModel(
            UserActionDescriptor.newBuilder().setActionId("copy").setLabel("复制").setSafe(true).build()
        )
        // 复制是手机本地行为(剪贴板在手机上),发往电脑端没有意义 —— 组装层直接封死。
        assertNull(unknownActionRequest("evt-1", 3L, copy, ""))
    }

    @Test
    fun `状态型动作请求带eventId与expectedRevision与note`() {
        val ack = unknownEventActionModel(
            UserActionDescriptor.newBuilder().setActionId("acknowledge").setLabel("标记已查看").setSafe(true).build()
        )
        val req = unknownActionRequest("evt-1", 3L, ack, "已收到,会在电脑端跟进")
        assertNotNull(req)
        assertEquals("evt-1", req!!.eventId)
        // CAS 必带:expected_revision = 客户端最后一次看到的 revision,不许省。
        assertEquals(3L, req.expectedRevision)
        assertEquals("acknowledge", req.actionId)
        // note 透传用户输入:有输入就带上,不做"始终传空串"。
        assertEquals("已收到,会在电脑端跟进", req.note)
    }

    @Test
    fun `用户没写备注时note为空是合法取值`() {
        val retry = unknownEventActionModel(
            UserActionDescriptor.newBuilder().setActionId("retry_later").setLabel("待上游支持").setSafe(true).build()
        )
        val req = unknownActionRequest("evt-1", 3L, retry, "")
        assertNotNull(req)
        assertEquals("", req!!.note)
        assertEquals("retry_later", req.actionId)
    }

    // ---- 列表分组(Task 4:需要你处理 / 已查看 / 仅供参考)--------------------------

    @Test
    fun `分组按requiresUser与处置态三分`() {
        assertEquals(UnknownEventGroup.NEEDS_USER, unknownEventGroupOf("NEW", requiresUser = true))
        // 新通知(requires_user=false)仅供参考,不催办。
        assertEquals(UnknownEventGroup.REFERENCE, unknownEventGroupOf("NEW", requiresUser = false))
    }

    @Test
    fun `处置态优先于requiresUser_已处理的不进需要你处理`() {
        // 另一台设备已把它标记为已查看:即使 requires_user=true 也不再催办。
        assertEquals(UnknownEventGroup.HANDLED, unknownEventGroupOf("INSPECTED", requiresUser = true))
        assertEquals(UnknownEventGroup.HANDLED, unknownEventGroupOf("RETRY_LATER", requiresUser = true))
        assertEquals(UnknownEventGroup.HANDLED, unknownEventGroupOf("ESCALATED", requiresUser = false))
        // daemon 未来新增的处置态(客户端不认识):按已处置处理(失败安全方向 ——
        // 误报待办会催办不存在的问题,漏显示代价小得多)。
        assertEquals(UnknownEventGroup.HANDLED, unknownEventGroupOf("FOO_STATUS", requiresUser = true))
    }

    @Test
    fun `分组清单组内按收到时间倒序`() {
        val base = newDaemonSummary()
        fun at(id: String, ts: Long, status: String) = unknownEventCardModel(
            base.toBuilder().setEventId(id).setReceivedAtUnixMs(ts).setProcessingStatus(status).build()
        )
        val groups = partitionUnknownEvents(
            listOf(
                at("older", 1_000L, "NEW"),
                at("newer", 9_000L, "NEW"),
                at("ref", 5_000L, "INSPECTED"),
                // 通知(requires_user=false,NEW)→ 仅供参考
                at("notice", 7_000L, "NEW").copy(requiresUser = false),
            )
        )
        assertEquals(listOf("newer", "older"), groups.needsUser.map { it.eventId })
        assertEquals(listOf("ref"), groups.handled.map { it.eventId })
        assertEquals(listOf("notice"), groups.reference.map { it.eventId })
    }
}
