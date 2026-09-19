package dev.pocketagent.app.ui.component

import android.annotation.SuppressLint
import android.media.AudioFormat
import android.media.AudioRecord
import android.media.MediaRecorder
import android.util.Log
import dev.pocketagent.app.ui.PaVoiceRecognizer
import org.json.JSONObject
import org.vosk.LibVosk
import org.vosk.LogLevel
import org.vosk.Model
import org.vosk.Recognizer

/**
 * 自定义错误码(落在 SpeechRecognizer 1..9 之外,经 voiceErrorKind 映射到
 * 新增的 VoiceErrorKind.ENGINE —— "端侧引擎异常"):
 *  · [VOICE_ERROR_VOSK_ENGINE]:引擎运行异常(识别器/录音器创建失败等);
 *  · [VOICE_ERROR_VOSK_MODEL]:模型加载失败(文件可能已被用户或清理软件删除)。
 */
internal const val VOICE_ERROR_VOSK_ENGINE = 100

/** 模型加载失败错误码(见 [VOICE_ERROR_VOSK_ENGINE] 说明)。 */
internal const val VOICE_ERROR_VOSK_MODEL = 101

/**
 * 麦克风不可用错误码:AudioRecord 初始化失败/进不了录音态 —— 典型成因是
 * MIUI 等国产 ROM 在"运行时权限"之外还有独立的「录音」开关(安全中心/权限管理),
 * 或麦克风正被其他应用占用。经 voiceErrorKind 映射到 VoiceErrorKind.MIC_BLOCKED,
 * 人话提示直接给"去哪里开"的可行动指引,不再伪装成引擎异常。
 */
internal const val VOICE_ERROR_MIC_BLOCKED = 102

/**
 * 引擎忙错误码:上一轮会话尚未结束(端侧 SenseVoice 停止后还有秒级
 * "模型加载+转写"空窗)时又请求开始。映射到 VoiceErrorKind.BUSY 给人话提示,
 * 替换旧实现的静默 return(用户点麦克风零反馈的静默失败点之一)。
 * 不复用系统码 8(ERROR_RECOGNIZER_BUSY):组合层把 8 列入"系统服务失败"
 * 名单,引擎侧码混进去会误伤引擎选择逻辑。
 */
internal const val VOICE_ERROR_ENGINE_BUSY = 103

/**
 * 麦克风不可用(初始化失败/进不了录音态)的标记异常:录音器创建处抛出,
 * 由会话执行体 catch 后映射为 [VOICE_ERROR_MIC_BLOCKED]。
 */
internal class MicUnavailableException(message: String) : IllegalStateException(message)

/**
 * Vosk JSON 结果 `{"text": "..."}` → 纯文本(非法 JSON/缺字段返回空串,JVM 单测锁定)。
 */
internal fun voskJsonText(json: String): String =
    runCatching { JSONObject(json).optString("text", "") }.getOrDefault("").trim()

/**
 * 端侧识别引擎的最小接口:喂 16kHz PCM16 音频,收分段终稿 / 中间稿 / 收尾终稿。
 * 真实现 [VoskRecognizerEngine] 包 Vosk 的 Recognizer(原生库 JVM 起不来);
 * 单测用 fake 顶替,只测 [runVoskRecognitionLoop] 的调度逻辑。
 */
internal interface PcmRecognizeEngine {
    /**
     * 喂一段 PCM 音频。
     * @return true = 端点检出,该段形成终稿(随后取 [segmentResultJson]);
     *         false = 还在说话中(可取 [partialResultJson])。
     */
    fun acceptWaveForm(data: ByteArray, len: Int): Boolean

    /** acceptWaveForm 返回 true 后:该分段的终稿 JSON。 */
    fun segmentResultJson(): String

    /** 说话中的中间稿 JSON(可为空对象)。 */
    fun partialResultJson(): String

    /** 音频喂完(停止收音)后:收尾残余的终稿 JSON。 */
    fun finalResultJson(): String

    /** 复位内部状态(下一轮 start 前调)。 */
    fun reset()

    /** 释放底层资源。 */
    fun close()
}

/** [PcmRecognizeEngine] 的 Vosk 真实现:一个 Recognizer 对应一轮听写会话。 */
internal class VoskRecognizerEngine(private val model: Model) : PcmRecognizeEngine {
    private val recognizer = Recognizer(model, VOSK_SAMPLE_RATE.toFloat())

    override fun acceptWaveForm(data: ByteArray, len: Int): Boolean = recognizer.acceptWaveForm(data, len)

    // org.vosk.Recognizer 的 Java getter(getResult 等)在 Kotlin 侧即属性形态
    override fun segmentResultJson(): String = recognizer.result

    override fun partialResultJson(): String = recognizer.partialResult

    override fun finalResultJson(): String = recognizer.finalResult

    override fun reset() {
        recognizer.reset()
    }

    override fun close() {
        runCatching { recognizer.close() }
            .onFailure { Log.d("PaVoice", "Vosk recognizer close 异常:${it.javaClass.simpleName}") }
    }
}

/** 采样率:Vosk 模型要求 16kHz。 */
internal const val VOSK_SAMPLE_RATE = 16_000

/**
 * 识别主循环(调度逻辑,JVM 单测锁定):
 *  反复读块 → 喂引擎 → 端点检出取分段终稿并累积;取消/读失败即收尾,冲刷 final
 *  残余后回调全文。取消后在收尾仍取 finalResult(已喂进引擎的残余音频不丢)。
 *
 * @param readBlock 填充 buf 返回读取字节数;<=0 表示源结束或失败
 * @param onAlive 首块音频成功喂入时调一次(等价系统识别的 onReadyForSpeech,喂死区看门狗)
 * @param onPartial 中间稿文本(仅日志/看门狗用途,接口无 partial,不做 UI 投递)
 * @param onFinished 整轮最终文本(分段终稿 + 收尾残余累积)
 */
internal fun runVoskRecognitionLoop(
    readBlock: (ByteArray) -> Int,
    engine: PcmRecognizeEngine,
    isCancelled: () -> Boolean,
    onAlive: () -> Unit,
    onPartial: (String) -> Unit,
    onFinished: (String) -> Unit,
) {
    val buf = ByteArray(3200) // 100ms@16kHz/16bit/单声道,取消指令最迟 ~100ms 后被察觉
    val parts = ArrayList<String>()
    var aliveSent = false
    while (!isCancelled()) {
        val n = runCatching { readBlock(buf) }.getOrDefault(-1)
        if (n <= 0) break
        if (!aliveSent) {
            aliveSent = true
            onAlive()
        }
        val segmentDone = runCatching { engine.acceptWaveForm(buf, n) }.getOrDefault(false)
        if (segmentDone) {
            val seg = voskJsonText(runCatching { engine.segmentResultJson() }.getOrDefault(""))
            if (seg.isNotEmpty()) parts.add(seg)
        } else {
            val partial = voskJsonText(runCatching { engine.partialResultJson() }.getOrDefault(""))
            if (partial.isNotEmpty()) onPartial(partial)
        }
    }
    // 收尾:冲刷引擎残余,取 final 稿(短句通常整段都落在 final 里)
    val tail = voskJsonText(runCatching { engine.finalResultJson() }.getOrDefault(""))
    if (tail.isNotEmpty()) parts.add(tail)
    onFinished(parts.joinToString(" "))
}

/**
 * PaVoiceRecognizer 的端侧实现:AudioRecord(16kHz/单声道/PCM16)采音 →
 * Vosk 识别 → final 结果一次性回调 [onResult]。语义与系统识别包装对齐:
 *  · start():后台起识别会话(首听需加载模型,秒级;之后模型常驻复用);
 *  · stopListening():置停止标志,循环最迟 ~100ms 后退出并交付最终结果;
 *  · destroy():停会话并释放模型/原生资源(离开会话页时)。
 *
 * 【错误】复用 onErrorEvent(Int) 通道:权限不足 → 9(ERROR_INSUFFICIENT_PERMISSIONS)、
 * 模型加载失败 → [VOICE_ERROR_VOSK_MODEL]、引擎/录音器异常 → [VOICE_ERROR_VOSK_ENGINE]。
 *
 * 【线程】全部 vosk 对象只在识别线程触碰;destroy 的"等退出 + 释放模型"整体放
 * 后台线程(PaVoskDestroy),调用线程(UI)立即返回,绝不 join 阻塞。
 */
internal class VoskVoiceRecognizer(
    private val modelPath: String,
    private val onResult: (String) -> Unit,
    private val onErrorEvent: (Int) -> Unit,
    private val onAlive: () -> Unit = {},
) : PaVoiceRecognizer {

    private val lock = Any()
    private var sessionThread: Thread? = null

    @Volatile
    private var stopped = false

    /** destroy 后本实例不可再用(与系统 SpeechRecognizer 语义一致);防与后台释放竞态。 */
    @Volatile
    private var destroyed = false

    /** 模型常驻(首次加载秒级,之后复用);仅识别线程加载、destroy 后台线程释放。 */
    private var model: Model? = null

    override fun start() {
        synchronized(lock) {
            if (destroyed) {
                Log.d("PaVoice", "Vosk start 忽略:实例已 destroy(重进会话页会新建实例)")
                return
            }
            if (sessionThread?.isAlive == true) {
                Log.d("PaVoice", "Vosk start 忽略:上一会话仍在进行(上报 BUSY 给用户反馈)")
                // 【静默失败修复】旧实现只记日志静默 return,用户点击零反馈
                onErrorEvent(VOICE_ERROR_ENGINE_BUSY)
                return
            }
            stopped = false
            sessionThread = Thread({
                try { runSession() } finally {
                    if (destroyed) synchronized(lock) {
                        runCatching { model?.close() }; model=null
                    }
                }
            }, "PaVoskVoice").apply {
                // 识别线程的未捕获异常绝不能带崩 UI:兜底为端侧引擎错误,状态机回 IDLE
                setUncaughtExceptionHandler { _, e ->
                    Log.d("PaVoice", "Vosk 会话线程异常:${e.javaClass.simpleName}: ${e.message}")
                    onErrorEvent(VOICE_ERROR_VOSK_ENGINE)
                }
                start()
            }
        }
    }

    override fun stopListening() {
        stopped = true
        Log.d("PaVoice", "Vosk stopListening:置停止标志,等循环交付 final")
    }

    override fun destroy() {
        destroyed = true
        stopped = true
        val t = synchronized(lock) { sessionThread }
        // 释放动作整体移后台:UI 线程绝不 join(巡逻 P2-a:主线程硬等最长 2s,
        // 贴着 ANR 阈值,说完话退出会话页必卡)。超时放弃本轮模型释放,交给进程回收。
        Thread({
            runCatching { t?.join(2_000) } // 等识别线程退出再释放模型(它可能正在用)
            if (t?.isAlive == true) {
                Log.d("PaVoice", "Vosk destroy:会话线程 2s 未退出,跳过模型释放(下次进程回收)")
                return@Thread
            }
            synchronized(lock) {
                runCatching { model?.close() }
                    .onFailure { Log.d("PaVoice", "Vosk model close 异常:${it.javaClass.simpleName}") }
                model = null
            }
            Log.d("PaVoice", "Vosk destroy 完成")
        }, "PaVoskDestroy").apply { start() }
    }

    /** 一轮识别会话:加载模型 → 建引擎 → 建录音器 → 跑识别循环 → 清理。 */
    private fun runSession() {
        // 原生库的日志默认很啰嗦,压到 WARNING 级(失败时 jar 里带的日志仍可见)
        runCatching { LibVosk.setLogLevel(LogLevel.WARNINGS) }

        val m = ensureModel() ?: return
        val engine = try {
            VoskRecognizerEngine(m)
        } catch (t: Throwable) {
            Log.d("PaVoice", "Vosk 建识别器失败:${t.javaClass.simpleName}: ${t.message}")
            onErrorEvent(VOICE_ERROR_VOSK_ENGINE)
            return
        }
        val record = createAudioRecord() ?: run { engine.close(); return }
        try {
            record.startRecording()
            if (record.recordingState != AudioRecord.RECORDSTATE_RECORDING) {
                // 进不了录音态:多见于 MIUI「录音」开关关闭或麦克风被其他应用占用
                throw MicUnavailableException("录音器未能进入录音态")
            }
            Log.d("PaVoice", "Vosk 会话开始(model=$modelPath)")
            engine.reset()
            runVoskRecognitionLoop(
                readBlock = { buf ->
                    if (stopped) -1 else record.read(buf, 0, buf.size)
                },
                engine = engine,
                isCancelled = { stopped },
                onAlive = {
                    // 等价系统识别的 onReadyForSpeech:喂死区看门狗
                    onAlive()
                },
                onPartial = { p ->
                    // 接口只有 onResult/onErrorEvent:中间稿仅记日志 + 喂看门狗,不投递 UI
                    Log.d("PaVoice", "Vosk partial='$p'")
                    onAlive()
                },
            ) { text ->
                Log.d("PaVoice", "Vosk final='$text'")
                onAlive()
                onResult(text)
            }
        } catch (t: MicUnavailableException) {
            Log.d("PaVoice", "Vosk 麦克风不可用:${t.message}")
            onErrorEvent(VOICE_ERROR_MIC_BLOCKED)
        } catch (t: Throwable) {
            Log.d("PaVoice", "Vosk 会话异常:${t.javaClass.simpleName}: ${t.message}")
            onErrorEvent(VOICE_ERROR_VOSK_ENGINE)
        } finally {
            runCatching { if (record.recordingState == AudioRecord.RECORDSTATE_RECORDING) record.stop() }
            runCatching { record.release() }
            engine.close()
        }
    }

    /** 加载(或复用)模型;失败报 [VOICE_ERROR_VOSK_MODEL] 并返回 null。 */
    private fun ensureModel(): Model? {
        model?.let { return it }
        return try {
            Log.d("PaVoice", "Vosk 加载模型:$modelPath")
            Model(modelPath).also { model = it }
        } catch (t: Throwable) {
            Log.d("PaVoice", "Vosk 模型加载失败:${t.javaClass.simpleName}: ${t.message}")
            onErrorEvent(VOICE_ERROR_VOSK_MODEL)
            null
        }
    }

    /**
     * 建 16kHz/单声道/PCM16 录音器。RECORD_AUDIO 权限由组合层授予流保证;
     * 若真被收回,SecurityException 映射为系统错误码 9,人话提示与系统识别一致。
     */
    @SuppressLint("MissingPermission")
    private fun createAudioRecord(): AudioRecord? {
        return try {
            val minBuf = AudioRecord.getMinBufferSize(
                VOSK_SAMPLE_RATE,
                AudioFormat.CHANNEL_IN_MONO,
                AudioFormat.ENCODING_PCM_16BIT,
            )
            if (minBuf <= 0) throw IllegalStateException("getMinBufferSize=$minBuf")
            val record = AudioRecord(
                MediaRecorder.AudioSource.MIC,
                VOSK_SAMPLE_RATE,
                AudioFormat.CHANNEL_IN_MONO,
                AudioFormat.ENCODING_PCM_16BIT,
                maxOf(minBuf * 2, VOSK_SAMPLE_RATE), // ≥0.5s 缓冲,读块 100ms 不必紧赶
            )
            if (record.state != AudioRecord.STATE_INITIALIZED) {
                record.release()
                // AudioRecord 初始化失败但运行时权限已给 ⇒ 几乎必是 MIUI 等的
                // 独立「录音」开关关闭,或麦克风被其他应用占用 —— 用专用错误码,
                // 人话提示给可行动指引(见 VOICE_ERROR_MIC_BLOCKED)
                throw MicUnavailableException("录音器初始化失败(state=${record.state})")
            }
            record
        } catch (t: SecurityException) {
            Log.d("PaVoice", "Vosk 录音无权限:${t.message}")
            onErrorEvent(9) // SpeechRecognizer.ERROR_INSUFFICIENT_PERMISSIONS(编译期常量)
            null
        } catch (t: Throwable) {
            Log.d("PaVoice", "Vosk 建录音器失败:${t.javaClass.simpleName}: ${t.message}")
            onErrorEvent(VOICE_ERROR_VOSK_ENGINE)
            null
        }
    }
}
