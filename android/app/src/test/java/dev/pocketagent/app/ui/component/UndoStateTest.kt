package dev.pocketagent.app.ui.component

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import org.junit.After
import org.junit.Assert.*
import org.junit.Test

class UndoStateTest {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Unconfined)
    private val state = UndoState(scope)

    @After fun close() { scope.cancel() }

    @Test fun `leaving after undo never submits deletion`() {
        var restored = 0
        var deleted = 0
        state.show("delete", onUndo = { restored++ }, onCommit = { deleted++ })
        val item = state.current!!
        assertTrue(state.runUndo(item) { assertTrue(it) })
        state.commitNow()
        assertEquals(1, restored)
        assertEquals(0, deleted)
    }

    @Test fun `undo owns operation before suspended restoration finishes`() {
        val restore = CompletableDeferred<Unit>()
        var deleted = 0
        var success = false
        state.show("delete", onUndo = { restore.await() }, onCommit = { deleted++ })
        val item = state.current!!
        assertTrue(state.runUndo(item) { success = it })
        state.commitNow()
        restore.complete(Unit)
        assertTrue(success)
        assertEquals(0, deleted)
    }

    @Test fun `rapid duplicate undo executes once`() {
        var restored = 0
        state.show("delete", onUndo = { restored++ })
        val item = state.current!!
        assertTrue(state.runUndo(item) {})
        assertFalse(state.runUndo(item) {})
        assertEquals(1, restored)
    }

    @Test fun `old feedback cannot clear next undo window`() {
        state.show("first", onUndo = {})
        val first = state.current!!
        state.runUndo(first) {}
        state.show("second", onUndo = {})
        state.clear(first)
        assertEquals("second", state.current!!.message)
    }

    @Test fun `timeout then leaving commits exactly once`() {
        var deleted = 0
        state.show("delete", onUndo = {}, onCommit = { deleted++ })
        val item = state.current!!
        state.timeout(item)
        state.timeout(item)
        state.commitNow()
        assertEquals(1, deleted)
        assertFalse(state.visible)
    }

    @Test fun `replacing pending operation commits previous but not new`() {
        val committed = mutableListOf<String>()
        state.show("first", onUndo = {}, onCommit = { committed.add("first") })
        val first = state.current!!
        state.show("second", onUndo = {}, onCommit = { committed.add("second") })
        state.timeout(first)
        assertEquals(listOf("first"), committed)
        assertEquals("second", state.current!!.message)
        assertFalse(state.runUndo(first) {})
    }

    @Test fun `replacing restored operation never commits it`() {
        var deleted = 0
        state.show("first", onUndo = {}, onCommit = { deleted++ })
        state.runUndo(state.current!!) {}
        state.show("second", onUndo = {})
        assertEquals(0, deleted)
    }

    @Test fun `failed restoration reports failure without deleting behind user`() {
        var result: Boolean? = null
        var deleted = 0
        state.show("first", onUndo = { error("restore failed") }, onCommit = { deleted++ })
        state.runUndo(state.current!!) { result = it }
        state.commitNow()
        assertEquals(false, result)
        assertEquals(0, deleted)
    }
}
