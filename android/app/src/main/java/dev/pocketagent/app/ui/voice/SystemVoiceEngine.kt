package dev.pocketagent.app.ui.voice

import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.Bundle
import android.speech.RecognitionListener
import android.speech.RecognizerIntent
import android.speech.SpeechRecognizer
import android.util.Log
import dev.pocketagent.app.ui.VoiceErrorKind as LegacyVoiceErrorKind
import dev.pocketagent.app.ui.systemVoiceRetireNeeded
import dev.pocketagent.app.ui.voiceErrorKind
import kotlinx.coroutines.flow.MutableSharedFlow

// =====================================================================================
// P4 Task 5 —— 系统识别引擎(SpeechRecognizer)适配
//
// 包装"现有系统识别路径"(与 ChatScreen.createSystemVoiceRecognizer 的行为签名一致:
// 预检 → on-device 优先 → zh-CN 自由文本在线优先 → startListening 同步异常兜底),
// 收敛到 VoiceEngine 接口供 VoiceController 驱动。波 3 接线后 ChatScreen 的私有
// 创建函数可整体删除改由本类接管(见建议反馈)。
// =====================================================================================

/**
 * 系统识别错误码 → 新领域错误分类(纯函数,JVM 单测锁定)。
 * 复用旧映射 [voiceErrorKind](其错误码名单/人话分流已被 SenseVoiceFallbackTest、
 * VoiceMicBlockedFeedbackTest 钉死,单一真值源),再按名对齐搬运到新枚举。
 */
internal fun systemVoiceErrorKind(code: Int): VoiceErrorKind = when (voiceErrorKind(code)) {
    LegacyVoiceErrorKind.NO_MATCH -> VoiceErrorKind.NO_MATCH
    LegacyVoiceErrorKind.BUSY -> VoiceErrorKind.BUSY
    LegacyVoiceErrorKind.INSUFFICIENT_PERMISSIONS -> VoiceErrorKind.INSUFFICIENT_PERMISSIONS
    LegacyVoiceErrorKind.CLIENT -> VoiceErrorKind.CLIENT
    LegacyVoiceErrorKind.OTHER -> VoiceErrorKind.OTHER
    LegacyVoiceErrorKind.ENGINE -> VoiceErrorKind.ENGINE
    LegacyVoiceErrorKind.MIC_BLOCKED -> VoiceErrorKind.MIC_BLOCKED
}

internal class SystemVoiceEngine private constructor(
    override val kind: VoiceEngineKind,
    private val handle: Handle,
) : VoiceEngine {

    /** SpeechRecognizer 的最小缝(创建在工厂里完成,便于未来 JVM fake 顶替)。 */
    internal interface Handle {
        fun startListening(intent: Intent)
        fun stopListening()
        fun destroy()
    }

    override val events = MutableSharedFlow<VoiceEngineEvent>(extraBufferCapacity = 64)

    /**
     * 本轮失败标记(计划:致命错误或零回调 → 只在本轮标记 ENGINE_FAILED,不永久污染)。
     * 置位条件:弃用名单错误码(复用 [systemVoiceRetireNeeded],单一真值源)、
     * startListening 同步异常。随 session 生命周期 —— 每次 [start] 重置;是否据此在
     * 会话中途换选端侧引擎,由波 3 的引擎再选择策略消费(旧路径语义:识别器随
     * remember 键翻转重建为端侧)。
     */
    @Volatile var roundFailed = false
        private set

    @Volatile private var sessionId = 0L
    @Volatile private var paused = false
    @Volatile private var cancelled = false
    private val accumulated = StringBuilder()
    private val intent: Intent by lazy { buildListenIntent() }

    val listener: RecognitionListener = object : RecognitionListener {
        override fun onReadyForSpeech(params: Bundle?) {
            emit(VoiceEngineEvent.Ready(sessionId))
        }

        override fun onBeginningOfSpeech() {}
        override fun onRmsChanged(rmsdB: Float) {}
        override fun onBufferReceived(buffer: ByteArray?) {}

        override fun onEndOfSpeech() {
            if (cancelled || paused) return
            emit(VoiceEngineEvent.Transcribing(sessionId))
        }

        override fun onPartialResults(partialResults: Bundle?) {
            if (cancelled || paused) return
            val text = partialResults
                ?.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION)
                ?.firstOrNull()
                ?.trim()
                .orEmpty()
            if (text.isNotEmpty()) emit(VoiceEngineEvent.Partial(sessionId, text))
        }

        override fun onResults(results: Bundle?) {
            if (cancelled) return
            val text = results
                ?.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION)
                ?.firstOrNull()
                ?.trim()
                .orEmpty()
            if (paused) {
                // 暂停 flush:结果并入累计,保持会话(机器仍 PAUSED),不终结
                if (text.isNotEmpty()) {
                    appendSegment(text)
                    emit(VoiceEngineEvent.Partial(sessionId, accumulatedText()))
                }
                return
            }
            if (text.isEmpty() && accumulated.isBlank()) {
                failed(VoiceErrorKind.NO_MATCH) // 没听到:与旧路径同语义(7),不伪装引擎异常
                return
            }
            if (text.isNotEmpty()) appendSegment(text)
            deliverFinal()
        }

        override fun onError(error: Int) {
            if (cancelled) return
            Log.d(TAG, "系统引擎 onError code=$error kind=${systemVoiceErrorKind(error)}")
            if (systemVoiceRetireNeeded(error)) roundFailed = true
            failed(systemVoiceErrorKind(error))
        }

        override fun onEvent(eventType: Int, params: Bundle?) {}
    }

    override fun start(sessionId: Long) {
        this.sessionId = sessionId
        roundFailed = false
        paused = false
        cancelled = false
        accumulated.setLength(0)
        try {
            Log.d(TAG, "系统引擎 start sid=$sessionId kind=$kind")
            handle.startListening(intent)
        } catch (t: Throwable) {
            // 个别 ROM 同步抛(服务未就绪/SecurityException 等):兜底 CLIENT,状态机回 ERROR
            Log.d(TAG, "startListening 同步异常:${t.javaClass.simpleName}")
            roundFailed = true
            failed(VoiceErrorKind.CLIENT)
        }
    }

    override fun pause() {
        if (paused || cancelled) return
        paused = true
        // SpeechRecognizer 无真暂停:停收音但会话保持,结果以 Partial flush 进累计
        runCatching { handle.stopListening() }
            .onFailure { Log.d(TAG, "pause stopListening 异常:${it.javaClass.simpleName}") }
    }

    override fun resume() {
        if (!paused || cancelled) return
        paused = false
        runCatching { handle.startListening(intent) }
            .onFailure {
                Log.d(TAG, "resume startListening 异常:${it.javaClass.simpleName}")
                failed(VoiceErrorKind.CLIENT)
            }
    }

    override fun stop() {
        if (cancelled) return
        if (paused) {
            // 暂停中没有活动采音:直接以已累计文本出稿
            paused = false
            if (accumulated.isBlank()) failed(VoiceErrorKind.NO_MATCH) else deliverFinal()
            return
        }
        runCatching { handle.stopListening() }
            .onFailure { Log.d(TAG, "stop stopListening 异常:${it.javaClass.simpleName}") }
    }

    override fun cancel() {
        cancelled = true
        paused = false
        accumulated.setLength(0)
        runCatching { handle.stopListening() }
    }

    override fun destroy() {
        runCatching { handle.destroy() }
            .onFailure { Log.d(TAG, "destroy 异常:${it.javaClass.simpleName}") }
    }

    private fun emit(e: VoiceEngineEvent) {
        events.tryEmit(e)
    }

    private fun failed(kind: VoiceErrorKind) {
        emit(VoiceEngineEvent.Failed(sessionId, kind))
    }

    private fun appendSegment(segment: String) {
        if (accumulated.isBlank()) accumulated.append(segment) else accumulated.append(' ').append(segment)
    }

    private fun accumulatedText(): String = accumulated.toString().trim()

    /** 生产日志只记字符数,绝不落 transcript(Global Constraint)。 */
    private fun deliverFinal() {
        val full = accumulatedText()
        accumulated.setLength(0)
        Log.d(TAG, "系统引擎 final chars=${full.length}")
        emit(VoiceEngineEvent.Final(sessionId, full))
    }

    /** 与旧路径 createSystemVoiceRecognizer 的识别意图逐项一致(zh-CN 自由文本,在线优先)。 */
    private fun buildListenIntent(): Intent = Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
        putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
        putExtra(RecognizerIntent.EXTRA_LANGUAGE, "zh-CN")
        putExtra(RecognizerIntent.EXTRA_LANGUAGE_PREFERENCE, "zh-CN")
        putExtra(RecognizerIntent.EXTRA_MAX_RESULTS, 1)
        // 在线优先:离线只在装了离线识别包的设备上才有意义(强求离线必吃错误 8/5)
        if (Build.VERSION.SDK_INT >= 23) putExtra(RecognizerIntent.EXTRA_PREFER_OFFLINE, false)
    }

    internal companion object {
        private const val TAG = "PaVoice"

        /**
         * 创建系统引擎;预检不过(isRecognitionAvailable=false:部分无识别服务的 ROM
         * 上 create/startListening 全静默)或识别器创建失败返回 null —— 上层决策表
         * 据此落端侧 SenseVoice 兜底(现有优先级保持:系统 → SenseVoice → UNAVAILABLE)。
         * 必须在主线程调用。
         */
        fun create(context: Context): SystemVoiceEngine? {
            val sysAvailable =
                runCatching { SpeechRecognizer.isRecognitionAvailable(context) }.getOrDefault(false)
            if (!sysAvailable) return null
            val onDevice = Build.VERSION.SDK_INT >= 31 &&
                runCatching { SpeechRecognizer.isOnDeviceRecognitionAvailable(context) }.getOrDefault(false)
            val sr = runCatching {
                if (onDevice) SpeechRecognizer.createOnDeviceSpeechRecognizer(context)
                else SpeechRecognizer.createSpeechRecognizer(context)
            }.getOrNull() ?: return null
            val engine = SystemVoiceEngine(
                kind = if (onDevice) VoiceEngineKind.SYSTEM_ON_DEVICE else VoiceEngineKind.SYSTEM_REMOTE,
                handle = object : Handle {
                    override fun startListening(intent: Intent) {
                        sr.startListening(intent)
                    }

                    override fun stopListening() {
                        sr.stopListening()
                    }

                    override fun destroy() {
                        sr.destroy()
                    }
                },
            )
            sr.setRecognitionListener(engine.listener)
            return engine
        }
    }
}
