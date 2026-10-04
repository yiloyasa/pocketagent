package dev.pocketagent.app.ui.voice

import android.os.SystemClock
import android.util.Log
import dev.pocketagent.app.ui.PaVoiceRecognizer
import dev.pocketagent.app.ui.component.SenseVoiceModelManager
import dev.pocketagent.app.ui.component.SenseVoiceRecognizer
import dev.pocketagent.app.ui.component.senseVoiceErrorFatalForRound
import kotlinx.coroutines.flow.MutableSharedFlow

// =====================================================================================
// P4 Task 5 —— 端侧 SenseVoice 引擎适配
//
// 包装 SenseVoiceRecognizer(录完再转写的批处理引擎):无实时 partial,结果在
// 停止/60 秒采音上限后一次性交付;暂停 = 提前收音(文本以 Partial flush 并入累计,
// 会话保持),恢复 = 同一 sessionId 重起采音轮,最终 Final 聚合全部分段。
//
// 【三种失败的分类映射】
//  · 冷加载撞看门狗      → VoiceErrorKind.MODEL_LOADING(Controller 按引擎类别+
//    "从未交付过 Final"分流,本类不重复计时);
//  · 模型损坏/校验失败   → VoiceErrorKind.MODEL_CORRUPTED;
//  · 磁盘空间不足        → VoiceErrorKind.STORAGE_FULL;
//  后两者来自 SenseVoiceModelManager 的初始化失败信息([classifySenseVoiceManagerError]),
//  并在识别期 100 号错误时结合管理态二次归类。
// =====================================================================================

/**
 * SenseVoiceModelManager 初始化失败信息 → 领域错误分类(纯函数,JVM 单测锁定)。
 *
 * 【已知局限】manager 目前只暴露自由文本 message(无结构化错误码):"内置语音模型
 * 校验失败,请重新安装 App" → 损坏;磁盘/空间类(ENOSPC 的 "No space left on
 * device"、目录创建失败等)→ 磁盘;其余(含 null)按引擎异常兜底。波 3 若给
 * SenseVoiceModelState 增加结构化 error kind,此处应改为精确映射(已登记建议反馈)。
 */
internal fun classifySenseVoiceManagerError(message: String?): VoiceErrorKind {
    if (message == null) return VoiceErrorKind.ENGINE
    val lower = message.lowercase()
    return when {
        lower.contains("校验失败") || lower.contains("损坏") -> VoiceErrorKind.MODEL_CORRUPTED
        lower.contains("space") || lower.contains("空间") || lower.contains("磁盘") -> VoiceErrorKind.STORAGE_FULL
        else -> VoiceErrorKind.ENGINE
    }
}

internal class SenseVoiceEngine private constructor(
    private val modelPath: String,
    /** monotonic 毫秒钟(默认 SystemClock.elapsedRealtime —— 只为耗时统计,判定不依赖它)。 */
    private val nowMillis: () -> Long = { SystemClock.elapsedRealtime() },
) : VoiceEngine {

    override val kind = VoiceEngineKind.SENSEVOICE
    override val events = MutableSharedFlow<VoiceEngineEvent>(extraBufferCapacity = 64)

    /**
     * 本轮致命失败标记([senseVoiceErrorFatalForRound]:9/102/100 致命,7/103 是正常
     * 业务结果)。随 session 生命周期:每次 [start] 重置,只描述"上一轮",不永久污染
     * 后续会话;端侧不据此自动弃用(与系统引擎的弃用名单语义刻意不同 —— 端侧重建
     * 要重新冷加载),仅供诊断与未来引擎再选择策略消费。
     */
    @Volatile var roundFailed = false
        private set

    @Volatile private var sessionId = 0L
    @Volatile private var readySent = false
    @Volatile private var transcribingSent = false
    @Volatile private var paused = false
    @Volatile private var cancelled = false
    private val accumulated = StringBuilder()
    private var segments = 0
    private var startedAtMs = 0L
    private var recognizer: PaVoiceRecognizer? = null

    override fun start(sessionId: Long) {
        this.sessionId = sessionId
        resetRound()
        startedAtMs = nowMillis()
        val path = SenseVoiceModelManager.modelPath()
        if (path == null) {
            // 模型未就绪:按管理态细分(损坏/磁盘/其他),绝不假装已启动
            Log.d(TAG, "SenseVoice start 失败:模型未就绪")
            failed(classifySenseVoiceManagerError(SenseVoiceModelManager.state.value.error))
            return
        }
        ensureRecognizer(path)
        Log.d(TAG, "SenseVoice start sid=$sessionId")
        recognizer?.start()
    }

    override fun pause() {
        if (paused || cancelled) return
        paused = true
        // 批处理引擎无真暂停:提前收音,转写结果以 Partial flush 进累计,会话保持
        recognizer?.stopListening()
    }

    override fun resume() {
        if (!paused || cancelled) return
        paused = false
        transcribingSent = false // 新采音轮:转写进度信号重新可用
        startedAtMs = nowMillis()
        recognizer?.start()
    }

    override fun stop() {
        if (cancelled) return
        if (paused) {
            // 暂停中没有活动采音:直接以已累计文本出稿
            paused = false
            if (accumulated.isBlank()) failed(VoiceErrorKind.NO_MATCH) else deliverFinal()
            return
        }
        recognizer?.stopListening() // 收音收尾,结果经 handleResult → Final
    }

    override fun cancel() {
        cancelled = true
        paused = false
        accumulated.setLength(0)
        recognizer?.stopListening() // 迟到结果被 cancelled 标记吞掉,绝不发 Final
    }

    override fun destroy() {
        recognizer?.destroy()
        recognizer = null
    }

    /** alive 回调:首个 = 收音就绪(Ready),其后首个 = 采音结束进转写(Transcribing)。 */
    private fun handleAlive() {
        if (cancelled) return
        if (!readySent) {
            readySent = true
            emit(VoiceEngineEvent.Ready(sessionId))
            return
        }
        if (!transcribingSent) {
            transcribingSent = true
            emit(VoiceEngineEvent.Transcribing(sessionId))
        }
    }

    private fun handleResult(text: String) {
        if (cancelled) return
        val trimmed = text.trim()
        if (paused) {
            // 暂停 flush:并入累计,保持会话(机器仍 PAUSED),不终结
            if (trimmed.isNotEmpty()) {
                appendSegment(trimmed)
                emit(VoiceEngineEvent.Partial(sessionId, accumulatedText()))
            }
            return
        }
        if (trimmed.isEmpty() && accumulated.isBlank()) {
            failed(VoiceErrorKind.NO_MATCH) // 没听到(识别器把空白结果折成 7,这里双保险)
            return
        }
        if (trimmed.isNotEmpty()) appendSegment(trimmed)
        deliverFinal()
    }

    private fun handleError(code: Int) {
        if (cancelled) return
        roundFailed = senseVoiceErrorFatalForRound(code)
        val kind = when (code) {
            7 -> VoiceErrorKind.NO_MATCH
            103 -> VoiceErrorKind.BUSY
            102 -> VoiceErrorKind.MIC_BLOCKED
            9 -> VoiceErrorKind.INSUFFICIENT_PERMISSIONS
            // 100/未知:结合模型管理态细分三种端侧失败(损坏/磁盘/其他)
            else ->
                SenseVoiceModelManager.state.value.error?.let(::classifySenseVoiceManagerError)
                    ?: VoiceErrorKind.ENGINE
        }
        // 生产日志只记错误分类,绝不落 transcript(Global Constraint)
        Log.d(TAG, "SenseVoice error code=$code kind=$kind fatal=$roundFailed")
        failed(kind)
    }

    /** 生产日志只记字符数/分段数/耗时,绝不落 transcript(Global Constraint)。 */
    private fun deliverFinal() {
        val full = accumulatedText()
        accumulated.setLength(0)
        Log.d(TAG, "SenseVoice final chars=${full.length} segments=$segments elapsedMs=${nowMillis() - startedAtMs}")
        emit(VoiceEngineEvent.Final(sessionId, full))
    }

    private fun failed(kind: VoiceErrorKind) {
        emit(VoiceEngineEvent.Failed(sessionId, kind))
    }

    private fun resetRound() {
        roundFailed = false
        readySent = false
        transcribingSent = false
        paused = false
        cancelled = false
        accumulated.setLength(0)
        segments = 0
    }

    private fun ensureRecognizer(path: String) {
        if (recognizer == null) {
            recognizer = SenseVoiceRecognizer(
                path = path,
                onResult = ::handleResult,
                onError = ::handleError,
                onAlive = ::handleAlive,
            )
        }
    }

    private fun appendSegment(segment: String) {
        segments++
        if (accumulated.isBlank()) accumulated.append(segment) else accumulated.append(' ').append(segment)
    }

    private fun accumulatedText(): String = accumulated.toString().trim()

    private fun emit(e: VoiceEngineEvent) {
        events.tryEmit(e)
    }

    internal companion object {
        private const val TAG = "PaVoice"

        /** 模型就绪才可创建(与旧 pickVoiceEngine 的 senseReady 门一致);未就绪返回 null。 */
        fun create(nowMillis: () -> Long = { SystemClock.elapsedRealtime() }): SenseVoiceEngine? =
            SenseVoiceModelManager.modelPath()?.let { SenseVoiceEngine(it, nowMillis) }
    }
}
