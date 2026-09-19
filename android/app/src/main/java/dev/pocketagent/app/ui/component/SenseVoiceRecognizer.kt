package dev.pocketagent.app.ui.component

import android.annotation.SuppressLint
import android.media.AudioFormat
import android.media.AudioRecord
import android.media.MediaRecorder
import android.os.Handler
import android.os.Looper
import android.util.Log
import com.k2fsa.sherpa.onnx.OfflineModelConfig
import com.k2fsa.sherpa.onnx.OfflineRecognizer
import com.k2fsa.sherpa.onnx.OfflineRecognizerConfig
import com.k2fsa.sherpa.onnx.OfflineSenseVoiceModelConfig
import dev.pocketagent.app.ui.PaVoiceRecognizer

/** Record first, then transcribe in bounded chunks. All native handles are owned
 * by the worker and released on every exit, including navigation and errors. */
internal class SenseVoiceRecognizer(
    private val path: String,
    private val onResult: (String) -> Unit,
    private val onError: (Int) -> Unit,
    private val onAlive: () -> Unit,
) : PaVoiceRecognizer {
    private val main = Handler(Looper.getMainLooper())
    private val runner = SenseVoiceSessionRunner(
        post = { main.post(it) },
        launch = { task -> Thread(task, "PaSenseVoice").start() },
        onResult=onResult, onError=onError, onAlive=onAlive,
        work=::runSession,
    )
    override fun start() {
        if (!runner.start()) {
            // 【静默失败修复】忙(停止后的转写空窗)或已销毁:旧实现静默 return,
            // 用户点麦克风零反馈。销毁态的回调会随页面销毁被丢弃,无副作用。
            Log.d("PaVoice", "SenseVoice start 未启动(忙/已销毁):上报 BUSY")
            main.post { onError(VOICE_ERROR_ENGINE_BUSY) }
        }
    }
    override fun stopListening() = runner.stopListening()
    override fun destroy() = runner.destroy()

    @SuppressLint("MissingPermission")
    private fun runSession(session: SenseVoiceSessionRunner.Session): String {
        val min=AudioRecord.getMinBufferSize(16_000,AudioFormat.CHANNEL_IN_MONO,AudioFormat.ENCODING_PCM_16BIT)
        check(min>0) { "录音格式不可用" }
        val record=AudioRecord(MediaRecorder.AudioSource.VOICE_RECOGNITION,16_000,
            AudioFormat.CHANNEL_IN_MONO,AudioFormat.ENCODING_PCM_16BIT,maxOf(min*2,32_000))
        val audio=try {
            if(record.state!=AudioRecord.STATE_INITIALIZED) {
                // 运行时权限已给仍初始化失败 ⇒ MIUI「录音」开关关闭/麦克风被其他应用占用
                throw MicUnavailableException("录音器初始化失败(state=${record.state})")
            }
            if(session.stopped) return ""
            record.startRecording()
            if(record.recordingState!=AudioRecord.RECORDSTATE_RECORDING) {
                throw MicUnavailableException("录音器未能进入录音态")
            }
            session.alive()
            captureSenseVoiceAudio(
                read={ buffer,size -> record.read(buffer,0,size,AudioRecord.READ_NON_BLOCKING) },
                stopped={session.stopped},
            )
        } finally {
            runCatching { record.stop() }
            record.release()
        }
        if(audio.isEmpty()) return ""
        // 收音结束、转写开始:喂一次看门狗 —— 停止指令后组合层重启了"8 秒零回调
        // 即回收 UI"的兜底,这里(以及下面每块转写完成)回活让它不把正常的长转写
        // 误判成引擎死区。
        session.alive()
        return synchronized(nativeLock) {
            if (session.cancelled) return@synchronized ""
            val engine=OfflineRecognizer(config=OfflineRecognizerConfig(modelConfig=OfflineModelConfig(
                senseVoice=OfflineSenseVoiceModelConfig(model="$path/model.int8.onnx",language="auto",useInverseTextNormalization=true),
                tokens="$path/tokens.txt",numThreads=2,provider="cpu",
            )))
            try {
                val text=StringBuilder()
                var offset=0
                while(offset<audio.size && !session.cancelled) {
                    val end=minOf(audio.size,offset+16_000*15)
                    val stream=engine.createStream()
                    try {
                        stream.acceptWaveform(audio.copyOfRange(offset,end),16_000)
                        engine.decode(stream)
                        val chunk=engine.getResult(stream).text.trim()
                        if(text.isNotEmpty() && chunk.isNotEmpty()) text.append(" ")
                        text.append(chunk)
                    } finally { stream.release() }
                    offset=end
                    session.alive() // 每块转写完成喂狗(首块含模型冷加载,可达数秒以上)
                }
                text.toString().trim()
            } finally { engine.release() }
        }
    }
    companion object { private val nativeLock=Any() }
}
