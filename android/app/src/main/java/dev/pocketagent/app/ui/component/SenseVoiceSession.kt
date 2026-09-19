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

    /**
     * 启动一轮会话。
     * @return true = 已启动;false = 忙(上一会话未结束)或已销毁。
     *   旧实现这种情况静默 return —— 端侧 SenseVoice 停止后还有秒级"模型加载+转写"
     *   空窗,空窗内点击麦克风会零反馈(用户感知"完全没反应"的静默失败点),
     *   调用方(SenseVoiceRecognizer.start)据此回报 [dev.pocketagent.app.ui.component.VOICE_ERROR_ENGINE_BUSY]。
     */
    fun start(): Boolean {
        val ticket: Long
        val next: Session
        synchronized(lock) {
            if (destroyed || active) return false
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
            } catch (_: MicUnavailableException) {
                // 麦克风不可用(MIUI「录音」开关/被其他应用占用):专用码给人话指引
                error = VOICE_ERROR_MIC_BLOCKED; ""
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
        return true
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
