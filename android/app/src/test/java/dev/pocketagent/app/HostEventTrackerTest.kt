package dev.pocketagent.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.HostEvent
import pocketagent.v1.TurnFinishedNotification

class HostEventTrackerTest {
    @Test
    fun duplicateSequenceIsConsumedOnlyOnce() {
        val tracker = HostEventTracker(7)
        val first = HostEvent.newBuilder()
            .setSequence(8)
            .setTurnFinished(TurnFinishedNotification.newBuilder().setThreadId("thread-a"))
            .build()
        val duplicate = first.toBuilder().build()

        assertTrue(tracker.accept(first))
        assertFalse(tracker.accept(duplicate))
        assertEquals(8L, tracker.lastSequence)
    }

    @Test
    fun olderSequenceCannotMoveCheckpointBack() {
        val tracker = HostEventTracker()
        assertTrue(tracker.accept(HostEvent.newBuilder().setSequence(12).build()))
        assertFalse(tracker.accept(HostEvent.newBuilder().setSequence(11).build()))
        assertEquals(12L, tracker.lastSequence)
    }

    @Test
    fun nonPositiveSequenceIsRejectedWithoutChangingCheckpoint() {
        // 最新-only(0.3.1):daemon 恒发正数 sequence；0/负数是协议违规，直接丢弃。
        val tracker = HostEventTracker(12)
        assertFalse(tracker.accept(HostEvent.getDefaultInstance()))
        assertFalse(tracker.accept(HostEvent.newBuilder().setSequence(-3).build()))
        assertEquals(12L, tracker.lastSequence)
    }

    @Test
    fun turnFinishedRouteContainsOnlyItsThreadId() {
        val event = HostEvent.newBuilder()
            .setTurnFinished(TurnFinishedNotification.newBuilder().setThreadId("thread-a"))
            .build()

        assertEquals("thread-a", HostEventRouter.turnFinishedThreadId(event))
        assertNull(HostEventRouter.turnFinishedThreadId(HostEvent.getDefaultInstance()))
    }
}
