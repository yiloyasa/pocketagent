package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * P1-4 状态机单测(纯函数层):
 *  1. 运行中发消息的投递状态流转(空闲发/运行中发 → Steered/Queued 帧处理/队列位次/拒收);
 *  2. 离线快照渲染分支(断连+空时间线 / 有连接 / 恢复)与横幅文案。
 *
 * TurnSession.send 本体依赖 AgentConnection(gRPC/SharedPreferences),JVM 单测
 * 无法实例化;它对帧的处理就是调用这里钉住的这几个纯函数,流转规则在此收敛。
 */
class TurnDeliveryAndOfflineTest {

    // ------------------------------------------------------------ 投递状态机

    @Test
    fun `标签文案覆盖全部投递状态`() {
        assertEquals("发送中…", deliveryLabel(DELIVERY_PENDING, 0))
        assertEquals("已插入运行中任务", deliveryLabel(DELIVERY_STEERED, 0))
        assertEquals("排队中 · 下一条", deliveryLabel(DELIVERY_QUEUED, 1))
        assertEquals("排队中 · 第3位", deliveryLabel(DELIVERY_QUEUED, 3))
        assertEquals("队列已满,未送达", deliveryLabel(DELIVERY_OVERFLOW, 0))
        assertEquals("", deliveryLabel("", 0)) // 普通消息无标签
    }

    @Test
    fun `Queued回执仅在position大于0时计入排队数`() {
        assertTrue(queuedAckAccepted(1))
        assertTrue(queuedAckAccepted(10))
        assertFalse(queuedAckAccepted(0)) // daemon 队列满拒收
        assertFalse(queuedAckAccepted(-1))
    }

    @Test
    fun `回执Toast文案只认Steered与Queued帧_其余状态不摆`() {
        // 【2026-10-14 帧驱动】Toast 只跟 daemon 实际回执走:Steered 帧 = 真插话,
        // Queued 帧 = 真排队(hermes 带 steer 位但 Steer 恒 false、实际落这里);
        // 提交瞬间不再按位预判(steerSubmitNotice 有位返回 null)。
        assertEquals("已插入运行中任务", deliveryAckNotice(DELIVERY_STEERED))
        assertEquals("已排队，当前轮结束后执行", deliveryAckNotice(DELIVERY_QUEUED))
        // pending = 等回执的中间态(徽标已显示"发送中…");overflow = 队列满拒收,
        // 由「队列已满,未送达」徽标 + 重试入口如实呈现;空串 = 普通消息。
        assertNull(deliveryAckNotice(DELIVERY_PENDING))
        assertNull(deliveryAckNotice(DELIVERY_OVERFLOW))
        assertNull(deliveryAckNotice(""))
    }

    @Test
    fun `Steered回执落到最早一条发送中的消息`() {
        val items = listOf(
            ChatItem("user", "第一条"), // 早已入库的正常消息,不受影响
            ChatItem("user", "第二条", deliverState = DELIVERY_QUEUED, queuePos = 1),
            ChatItem("user", "第三条", deliverState = DELIVERY_PENDING),
        )
        val out = applyDeliveryAckTo(items, DELIVERY_STEERED, 0)
        assertEquals("", out[0].deliverState)
        // FIFO:最早 pending 的(第三条)被回执,已排队的第二条不动
        assertEquals(DELIVERY_QUEUED, out[1].deliverState)
        assertEquals(DELIVERY_STEERED, out[2].deliverState)
    }

    @Test
    fun `Queued回执带上队列位次`() {
        val items = listOf(ChatItem("user", "x", deliverState = DELIVERY_PENDING))
        val out = applyDeliveryAckTo(items, DELIVERY_QUEUED, 2)
        assertEquals(DELIVERY_QUEUED, out[0].deliverState)
        assertEquals(2, out[0].queuePos)
    }

    @Test
    fun `队列满拒收时标记未送达`() {
        val items = listOf(ChatItem("user", "x", deliverState = DELIVERY_PENDING))
        val out = applyDeliveryAckTo(items, DELIVERY_OVERFLOW, 0)
        assertEquals(DELIVERY_OVERFLOW, out[0].deliverState)
        assertEquals("队列已满,未送达", deliveryLabel(out[0].deliverState, out[0].queuePos))
    }

    @Test
    fun `没有发送中的消息时回执是空操作`() {
        val items = listOf(ChatItem("user", "普通消息"))
        assertSame(items, applyDeliveryAckTo(items, DELIVERY_STEERED, 0))
    }

    @Test
    fun `轮次终局只收发送中与已插入的标签_排队与未送达保留`() {
        val items = listOf(
            ChatItem("user", "已插入", deliverState = DELIVERY_STEERED),
            ChatItem("user", "发送中", deliverState = DELIVERY_PENDING),
            ChatItem("user", "排队中", deliverState = DELIVERY_QUEUED, queuePos = 1),
            ChatItem("user", "未送达", deliverState = DELIVERY_OVERFLOW),
            ChatItem("user", "普通"),
        )
        val out = clearDeliveryTags(items, queuedOnly = false)!!
        assertEquals("", out[0].deliverState)
        assertEquals("", out[1].deliverState)
        assertEquals(DELIVERY_QUEUED, out[2].deliverState) // 排队的还没轮到
        assertEquals(DELIVERY_OVERFLOW, out[3].deliverState) // 未送达必须继续可见
        assertEquals("", out[4].deliverState)
    }

    @Test
    fun `QueueFlushed只收排队标签`() {
        val items = listOf(
            ChatItem("user", "排队1", deliverState = DELIVERY_QUEUED, queuePos = 1),
            ChatItem("user", "排队2", deliverState = DELIVERY_QUEUED, queuePos = 2),
            ChatItem("user", "未送达", deliverState = DELIVERY_OVERFLOW),
        )
        val out = clearDeliveryTags(items, queuedOnly = true)!!
        assertEquals("", out[0].deliverState)
        assertEquals("", out[1].deliverState)
        assertEquals(DELIVERY_OVERFLOW, out[2].deliverState)
    }

    @Test
    fun `没有可收的标签时返回null避免整表替换`() {
        val items = listOf(ChatItem("user", "普通"))
        assertNull(clearDeliveryTags(items, queuedOnly = true))
        assertNull(clearDeliveryTags(items, queuedOnly = false))
        assertNull(clearAllDeliveryTags(items))
    }

    @Test
    fun `流结束时清掉全部标签_不留幽灵排队`() {
        val items = listOf(
            ChatItem("user", "排队", deliverState = DELIVERY_QUEUED, queuePos = 2),
            ChatItem("user", "发送中", deliverState = DELIVERY_PENDING),
        )
        val out = clearAllDeliveryTags(items)!!
        assertTrue(out.all { it.deliverState.isEmpty() && it.queuePos == 0 })
    }

    // ------------------------------------------------------------ 离线快照分支

    @Test
    fun `已知断网且时间线为空才用离线快照渲染`() {
        assertTrue(shouldRenderOfflineSnapshot("down", itemsEmpty = true))
        // 有连接时行为完全不变
        assertFalse(shouldRenderOfflineSnapshot("ok", itemsEmpty = true))
        // checking 是巡检中间态,不是已知断网(不闪假离线)
        assertFalse(shouldRenderOfflineSnapshot("checking", itemsEmpty = true))
        // 已有消息(在线加载成功过/正在跑的任务)不覆盖
        assertFalse(shouldRenderOfflineSnapshot("down", itemsEmpty = false))
    }

    @Test
    fun `离线横幅文案_无快照时间戳不显示`() {
        assertNull(offlineSnapshotBanner(0L))
        assertNull(offlineSnapshotBanner(-1L))
        // 人话三段(快赢版):这是X小时前的旧对话+连上自动更新+只能看不能发;
        // 时长只读 ThreadSnapshot 已有时间戳(注入假钟,不编造)
        val now = 1_700_000_000_000L
        val text = offlineSnapshotBanner(now - 3L * 3_600_000L, now)
        assertNotNull(text)
        assertEquals("这是3小时前的旧对话，连上自动更新，现在只能看不能发", text)
    }
}
