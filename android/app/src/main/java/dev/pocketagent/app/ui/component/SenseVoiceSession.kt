package dev.pocketagent.app.ui.component

/** Pure lifecycle boundary: no stale result may enter a newly started recording. */
internal class SenseVoiceSessionRunner(
    private val post: (() -> Unit) -> Unit,
    private val launch: (() -> Unit) -> Unit,
    private val onResult: (String) -> Unit,
    private val onError: (Int) -> Unit,
    private val onAlive: () -> Unit,
    private val work: (Session) -> String,
) {
    class Session(private val reportAlive: () -> Unit) {
        @Volatile var stopped = false
        @Volatile var cancelled = false
        fun alive() = reportAlive()
    }
    private val lock = Any()
    private var destroyed = false
    private var active = false
    private var generation = 0L
    private var session: Session? = null

    fun start() {
        val ticket: Long
        val next: Session
        synchronized(lock) {
            if (destroyed || active) return
            active = true
            ticket = ++generation
            next = Session { deliver(ticket, onAlive) }
            session = next
        }
        launch {
            var error: Int? = null
            val text = try {
                if (next.stopped) "" else work(next)
            } catch (_: SecurityException) {
                error = 9; ""
            } catch (_: Throwable) {
                error = VOICE_ERROR_VOSK_ENGINE; ""
            }
            // work's finally has already released every resource.
            synchronized(lock) { active = false }
            deliver(ticket) {
                if (error != null) onError(error!!)
                else if (text.isBlank()) onError(7)
                else onResult(text)
            }
        }
    }

    private fun deliver(ticket: Long, callback: () -> Unit) {
        post {
            synchronized(lock) {
                if (!destroyed && generation == ticket) callback()
            }
        }
    }

    fun stopListening() = synchronized(lock) { session?.stopped = true }
    fun destroy(): Unit = synchronized(lock) {
        destroyed = true
        session?.cancelled = true
        session?.stopped = true
        generation++
        Unit
    }
}

internal fun captureSenseVoiceAudio(
    read: (ShortArray, Int) -> Int,
    stopped: () -> Boolean,
    nowMillis: () -> Long = { System.nanoTime() / 1_000_000 },
    idle: () -> Unit = { Thread.sleep(10) },
    maxSamples: Int = 16_000 * 60,
    maxMillis: Long = 60_000,
): FloatArray {
    require(maxSamples > 0 && maxMillis > 0)
    val samples = FloatArray(maxSamples)
    val buffer = ShortArray(minOf(1600, maxSamples))
    val start = nowMillis()
    var count = 0
    while (!stopped() && count < maxSamples && nowMillis() - start < maxMillis) {
        val n = read(buffer, minOf(buffer.size, maxSamples - count))
        if (n < 0) {
            if (stopped()) break
            error("录音读取失败 $n")
        }
        check(n <= minOf(buffer.size, maxSamples - count)) { "录音返回长度异常" }
        if (n == 0) { idle(); continue }
        for (i in 0 until n) samples[count++] = buffer[i] / 32768f
    }
    return samples.copyOf(count)
}
