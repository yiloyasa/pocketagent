package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.DELIVERY_OVERFLOW
import dev.pocketagent.app.ui.DELIVERY_PENDING
import dev.pocketagent.app.ui.DELIVERY_QUEUED
import dev.pocketagent.app.ui.DELIVERY_STEERED
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * P0-2 发送状态 → 图标/文案映射(纯函数,见 DeliveryBadge.kt)。
 * 两级等待(时钟)+ 一级已提交(单勾)+ 一级拒收(红!);「daemon 开始执行」
 * 的双勾级因协议无对应回执帧暂缺(TurnDelivery.kt 的帧序契约),在映射注释里留了升级位。
 */
class DeliveryBadgeTest {

    @Test
    fun pending_maps_to_clock_with_sending_copy() {
        val visual = deliveryVisual(DELIVERY_PENDING, 0)
        assertEquals(DeliveryIconKind.CLOCK, visual.icon)
        assertEquals("发送中…", visual.label)
        assertFalse(visual.failure)
    }

    @Test
    fun queued_maps_to_clock_and_keeps_position_copy() {
        // 既有「排队中 · 第N位」句式保留(queuePos>1);第 1 位用「下一条」
        val third = deliveryVisual(DELIVERY_QUEUED, 3)
        assertEquals(DeliveryIconKind.CLOCK, third.icon)
        assertEquals("排队中 · 第3位", third.label)
        assertFalse(third.failure)

        val next = deliveryVisual(DELIVERY_QUEUED, 1)
        assertEquals("排队中 · 下一条", next.label)
    }

    @Test
    fun steered_maps_to_single_check_with_submitted_copy() {
        // 审计 P0-2 口径:已提交 daemon = 单勾 + 「已提交」
        val visual = deliveryVisual(DELIVERY_STEERED, 0)
        assertEquals(DeliveryIconKind.CHECK_SINGLE, visual.icon)
        assertEquals("已提交", visual.label)
        assertFalse(visual.failure)
    }

    @Test
    fun overflow_maps_to_exclamation_and_is_retryable() {
        val visual = deliveryVisual(DELIVERY_OVERFLOW, 0)
        assertEquals(DeliveryIconKind.EXCLAMATION, visual.icon)
        assertEquals("队列已满,未送达", visual.label)
        assertTrue(visual.failure)
        assertTrue(deliveryRetryable(DELIVERY_OVERFLOW))
    }

    @Test
    fun only_overflow_is_retryable() {
        assertFalse(deliveryRetryable(DELIVERY_PENDING))
        assertFalse(deliveryRetryable(DELIVERY_QUEUED))
        assertFalse(deliveryRetryable(DELIVERY_STEERED))
        assertFalse(deliveryRetryable(""))
    }

    @Test
    fun unknown_state_yields_empty_label_so_badge_hides() {
        // 映射外的状态一律空串(组件据此不渲染),绝不猜一个标签出来
        val visual = deliveryVisual("some-future-state", 0)
        assertEquals(DeliveryIconKind.CLOCK, visual.icon)
        assertEquals("", visual.label)
        assertFalse(visual.failure)
    }
}
