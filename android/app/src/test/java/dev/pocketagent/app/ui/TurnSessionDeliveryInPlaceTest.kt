package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.Queued
import pocketagent.v1.QueueFlushed
import pocketagent.v1.Steered
import pocketagent.v1.TurnCompleted
import pocketagent.v1.TurnServerMessage

/**
 * 【C3 收尾】投递标签更新的"索引赋值"调用形态语义钉(经 applyServerFrame 走
 * 实时流/回放同一条状态机):
 *
 *  - STEERED/QUEUED/OVERFLOW:只改 firstPendingDeliveryIndex 定位到的**那一条**
 *    (沿用旧 id,其余字段不动),其余条目保持原实例 —— 替代旧 clear+addAll
 *    整表替换后,内容语义必须与 TurnDelivery 纯函数逐字段一致(纯函数本体
 *    由 TurnDeliveryAndOfflineTest 钉住,这里钉的是调用点的等价性);
 *  - QUEUE_FLUSHED / 轮次终局:只收该收的标签,不该动的(排队/未送达)保留;
 *  - 找不到 pending 条目的兜底 = 空操作(等价旧实现 swapItems 判同引用跳过)。
 */
class TurnSessionDeliveryInPlaceTest {

    private fun newState(vararg items: ChatItem): TurnSession.State {
        val s = TurnSession.State()
        s.items.addAll(items)
        return s
    }

    private fun steeredFrame(): TurnServerMessage =
        TurnServerMessage.newBuilder().setSteered(Steered.getDefaultInstance()).build()

    private fun queuedFrame(position: Int): TurnServerMessage =
        TurnServerMessage.newBuilder().setQueued(Queued.newBuilder().setPosition(position)).build()

    private fun queueFlushedFrame(): TurnServerMessage =
        TurnServerMessage.newBuilder().setQueueFlushed(QueueFlushed.getDefaultInstance()).build()

    private fun completedFrame(): TurnServerMessage =
        TurnServerMessage.newBuilder().setCompleted(TurnCompleted.getDefaultInstance()).build()

    // ------------------------------------------------------------ Steered/Queued/OVERFLOW

    @Test
    fun `Steered回执原位改最早一条发送中_其余条目原实例与id不动`() {
        val settled = ChatItem("user", "早先入库", id = "a")
        val queued = ChatItem("user", "先排着", deliverState = DELIVERY_QUEUED, queuePos = 1, id = "b")
        val pending = ChatItem("user", "刚发出", deliverState = DELIVERY_PENDING, id = "c")
        val s = newState(settled, queued, pending)

        TurnSession.applyServerFrame("t-steered", s, steeredFrame(), replay = false)

        // FIFO:最早 pending 的第三条被置为 steered,沿用旧 id
        assertEquals(DELIVERY_STEERED, s.items[2].deliverState)
        assertEquals("c", s.items[2].id)
        assertEquals(0, s.items[2].queuePos)
        // 未命中条目:同一实例、内容原样(不做整表替换)
        assertSame(settled, s.items[0])
        assertSame(queued, s.items[1])
        assertEquals(DELIVERY_QUEUED, s.items[1].deliverState)
    }

    @Test
    fun `Queued回执带队列位次且排队计数加一`() {
        val pending = ChatItem("user", "x", deliverState = DELIVERY_PENDING, id = "b")
        val s = newState(pending)

        TurnSession.applyServerFrame("t-queued", s, queuedFrame(2), replay = false)

        assertEquals(1, s.queuedCount)
        assertEquals(DELIVERY_QUEUED, s.items[0].deliverState)
        assertEquals(2, s.items[0].queuePos)
        assertEquals("b", s.items[0].id)
    }

    @Test
    fun `队列满拒收标记未送达_不计排队数`() {
        val pending = ChatItem("user", "x", deliverState = DELIVERY_PENDING, id = "b")
        val s = newState(pending)

        TurnSession.applyServerFrame("t-overflow", s, queuedFrame(0), replay = false)

        assertEquals(0, s.queuedCount)
        assertEquals(DELIVERY_OVERFLOW, s.items[0].deliverState)
        assertEquals(0, s.items[0].queuePos)
        assertEquals("队列已满,未送达", deliveryLabel(s.items[0].deliverState, s.items[0].queuePos))
    }

    // ------------------------------------------------------------ 兜底分支(找不到 pending)

    @Test
    fun `没有发送中的条目时回执是空操作_其余条目原实例不动`() {
        val plain = ChatItem("user", "普通消息", id = "a")
        val s = newState(plain)

        TurnSession.applyServerFrame("t-noop-steered", s, steeredFrame(), replay = false)

        assertSame("无 pending 时不得替换任何条目", plain, s.items[0])
        assertEquals("", s.items[0].deliverState)
        assertEquals(0, s.queuedCount)
    }

    @Test
    fun `没有pending时被接受的Queued回执仍计数_条目不动`() {
        // 与旧实现一致:accepted Queued 先计数(回执已收),标签无对象可挂则跳过
        val plain = ChatItem("user", "普通消息", id = "a")
        val s = newState(plain)

        TurnSession.applyServerFrame("t-noop-queued", s, queuedFrame(1), replay = false)

        assertEquals(1, s.queuedCount)
        assertSame(plain, s.items[0])
        assertEquals("", s.items[0].deliverState)
    }

    // ------------------------------------------------------------ QueueFlushed / 轮次终局

    @Test
    fun `QueueFlushed只收排队标签_未送达保留_计数减一`() {
        val q1 = ChatItem("user", "排队1", deliverState = DELIVERY_QUEUED, queuePos = 1, id = "a")
        val q2 = ChatItem("user", "排队2", deliverState = DELIVERY_QUEUED, queuePos = 2, id = "b")
        val over = ChatItem("user", "未送达", deliverState = DELIVERY_OVERFLOW, id = "c")
        val s = newState(q1, q2, over)
        s.queuedCount = 2

        TurnSession.applyServerFrame("t-flushed", s, queueFlushedFrame(), replay = false)

        assertEquals(1, s.queuedCount)
        assertEquals("", s.items[0].deliverState)
        assertEquals("", s.items[1].deliverState)
        assertEquals("a", s.items[0].id)
        assertEquals("b", s.items[1].id)
        assertSame("未送达条目不是本拍改动对象", over, s.items[2])
        assertEquals(DELIVERY_OVERFLOW, s.items[2].deliverState)
    }

    @Test
    fun `轮次终局收发送中与已插入标签_排队与未送达保留`() {
        val steered = ChatItem("user", "已插入", deliverState = DELIVERY_STEERED, id = "a")
        val pending = ChatItem("user", "发送中", deliverState = DELIVERY_PENDING, id = "b")
        val queued = ChatItem("user", "排队中", deliverState = DELIVERY_QUEUED, queuePos = 1, id = "c")
        val over = ChatItem("user", "未送达", deliverState = DELIVERY_OVERFLOW, id = "d")
        val plain = ChatItem("user", "普通", id = "e")
        val s = newState(steered, pending, queued, over, plain)

        val sawEnd = TurnSession.applyServerFrame("t-terminal", s, completedFrame(), replay = true)

        assertTrue("COMPLETED 是终局帧", sawEnd)
        assertEquals("", s.items[0].deliverState)
        assertEquals("", s.items[1].deliverState)
        assertEquals(DELIVERY_QUEUED, s.items[2].deliverState) // 排队的还没轮到
        assertEquals(DELIVERY_OVERFLOW, s.items[3].deliverState) // 未送达必须继续可见
        assertEquals("", s.items[4].deliverState)
        // 幸存条目保持原实例(本拍只改了前两条)
        assertSame(queued, s.items[2])
        assertSame(over, s.items[3])
        assertSame(plain, s.items[4])
    }

    @Test
    fun `无可收标签时终局帧不产生多余条目`() {
        val plain = ChatItem("user", "普通", id = "a")
        val s = newState(plain)

        TurnSession.applyServerFrame("t-terminal-clean", s, completedFrame(), replay = true)

        assertEquals(1, s.items.size)
        assertSame(plain, s.items[0])
    }
}
