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
     *   调用方(SenseVoiceRecognizer.start)据此回报 [VOICE_ERROR_ENGINE_BUSY]。
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
                error = VOICE_ERROR_ENGINE; ""
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

/**
 * 端侧错误码是否"本轮致命"(引擎本轮已不可再交付结果,适配层据此标记 roundFailed):
 *  7(没听到)/103(引擎忙)是正常业务结果,原样重试即可;9(权限)/102(麦克风被拦)/
 *  100(未分类引擎异常)视为本轮致命。
 *
 * 消费方是 ui/voice/SenseVoiceEngine:roundFailed 随 session 生命周期 —— 每次 start
 * 重置,只描述"上一轮",不永久污染后续会话;端侧不据此自动弃用(与系统引擎的弃用
 * 名单语义刻意不同:端侧重建要重新冷加载),只做本轮标记供诊断与未来引擎再选择
 * 策略使用。纯函数,JVM 单测锁定(VoiceSessionReducerTest)。
 */
internal fun senseVoiceErrorFatalForRound(code: Int): Boolean = code != 7 && code != 103

/**
 * 【看门狗时钟契约】nowMillis 默认取 System.nanoTime()(monotonic,不受用户改系统
 * 时间/NTP 跳变影响):采音时长上限与 8 秒零回调死区判定的喂狗链路都依赖它,绝不可
 * 换回 System.currentTimeMillis(墙钟可回拨,会把正常听写误判成死区)。
 * 2026-10-04 P4 Task 5 monotonic 化复核钉死:本文件是端侧链路唯一的时钟源,已单调;
 * 旧路径 ChatScreen 看门狗的墙钟改 monotonic 随波 3 接线切换到 ui/voice/VoiceController。
 */
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
