package dev.pocketagent.app.ui.component

import org.junit.Assert.*
import org.junit.Test

class SenseVoiceSessionTest {
    @Test fun `destroy suppresses queued callbacks and cancels worker without waiting`() {
        val main = ArrayDeque<() -> Unit>()
        val workers = ArrayDeque<() -> Unit>()
        val events = mutableListOf<String>()
        val runner = SenseVoiceSessionRunner(
            post = { main.add(it) }, launch = { workers.add(it) },
            onResult = { events.add(it) }, onError = { events.add("error") },
            onAlive = { events.add("alive") },
        ) { session -> session.alive(); "result" }
        runner.start()
        workers.removeFirst()()
        runner.destroy()
        main.forEach { it() }
        runner.start()
        assertTrue(events.isEmpty())
        assertTrue(workers.isEmpty())
    }

    @Test fun `duplicate start is ignored and stale generation cannot deliver into next session`() {
        val main = ArrayDeque<() -> Unit>()
        val workers = ArrayDeque<() -> Unit>()
        val results = mutableListOf<String>()
        var runs = 0
        val runner = SenseVoiceSessionRunner(
            post = { main.add(it) }, launch = { workers.add(it) },
            onResult = { results.add(it) }, onError = { fail("unexpected error") }, onAlive = {},
        ) { "${++runs}" }
        runner.start()
        runner.start()
        assertEquals(1, workers.size)
        workers.removeFirst()()
        runner.start()
        main.removeFirst()()
        assertTrue(results.isEmpty())
        workers.removeFirst()()
        main.removeFirst()()
        assertEquals(listOf("2"), results)
    }

    @Test fun `stop before worker begins avoids work and delivers no match after cleanup`() {
        val workers = ArrayDeque<() -> Unit>()
        val errors = mutableListOf<Int>()
        val runner = SenseVoiceSessionRunner(
            post = { it() }, launch = { workers.add(it) },
            onResult = { fail("unexpected result") }, onError = { errors.add(it) }, onAlive = {},
        ) { fail("stopped session must not acquire resources"); "" }
        runner.start()
        runner.stopListening()
        workers.removeFirst()()
        assertEquals(listOf(7), errors)
    }

    @Test fun `terminal callback happens after resource cleanup and permits restart`() {
        val main = ArrayDeque<() -> Unit>()
        val workers = ArrayDeque<() -> Unit>()
        var closed = false
        val errors = mutableListOf<Int>()
        val runner = SenseVoiceSessionRunner(
            post = { main.add(it) }, launch = { workers.add(it) },
            onResult = { fail("unexpected result") },
            onError = { assertTrue(closed); errors.add(it) }, onAlive = {},
        ) {
            try { throw SecurityException() } finally { closed = true }
        }
        runner.start()
        workers.removeFirst()()
        assertTrue(errors.isEmpty())
        main.removeFirst()()
        assertEquals(listOf(9), errors)
        runner.start()
        assertEquals(1, workers.size)
    }

    @Test fun `capture has sample and elapsed time bounds including zero reads`() {
        val audio = captureSenseVoiceAudio(
            read = { buffer, size -> buffer.fill(16384); size },
            stopped = { false }, nowMillis = { 0L }, idle = {},
            maxSamples = 5, maxMillis = 60,
        )
        assertEquals(5, audio.size)
        assertEquals(0.5f, audio[0], 0f)
        var now = 0L
        var reads = 0
        val empty = captureSenseVoiceAudio(
            read = { _, _ -> reads++; 0 }, stopped = { false },
            nowMillis = { now }, idle = { now += 10 },
            maxSamples = 5, maxMillis = 60,
        )
        assertTrue(empty.isEmpty())
        assertEquals(6, reads)
    }

    @Test fun `capture stops on cancellation and surfaces audio errors`() {
        var reads = 0
        val audio = captureSenseVoiceAudio(
            read = { _, _ -> reads++; 1 }, stopped = { reads == 1 },
            nowMillis = { 0 }, idle = {}, maxSamples = 5,
        )
        assertEquals(1, audio.size)
        assertThrows(IllegalStateException::class.java) {
            captureSenseVoiceAudio(read = { _, _ -> -6 }, stopped = { false })
        }
    }
}
