package dev.pocketagent.app.ui.component

import org.junit.Assert.*
import org.junit.Test
import org.junit.Rule
import org.junit.rules.TemporaryFolder

class TurnCheckpointStoreTest {
    @get:Rule val temporary = TemporaryFolder()

    @Test fun partialTextAndCursorRecoverTogetherWithoutCompletingTheTask() {
        TurnCheckpointStore.initForTest(temporary.newFolder())
        val checkpoint = TurnCheckpointStore.Checkpoint("native-session",
            listOf(ThreadSnapshot.Msg("user","继续执行")), "离线前的一半", "思考片段", 52,
            true, 1, "new-agent/private-model", "adaptive", null)
        TurnCheckpointStore.save(checkpoint)
        assertEquals(checkpoint, TurnCheckpointStore.load("native-session"))
        assertEquals(listOf(checkpoint), TurnCheckpointStore.active())
        TurnCheckpointStore.save(checkpoint.copy(stream="", active=false, error="native error"))
        assertTrue(TurnCheckpointStore.active().isEmpty())
        assertEquals("native error", TurnCheckpointStore.load("native-session")?.error)
    }

    @Test fun oversizedPartialNeverKeepsAnInvalidCursor() {
        TurnCheckpointStore.initForTest(temporary.newFolder())
        TurnCheckpointStore.save(TurnCheckpointStore.Checkpoint("large", emptyList(), "x".repeat(600_000),
            "", 900, true, 0, "", null, null))
        assertNull(TurnCheckpointStore.load("large"))
    }
}
