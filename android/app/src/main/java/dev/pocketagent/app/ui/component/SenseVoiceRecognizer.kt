package dev.pocketagent.app.ui.component

import android.annotation.SuppressLint
import android.media.AudioFormat
import android.media.AudioRecord
import android.media.MediaRecorder
import android.os.Handler
import android.os.Looper
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
    override fun start() = runner.start()
    override fun stopListening() = runner.stopListening()
    override fun destroy() = runner.destroy()

    @SuppressLint("MissingPermission")
    private fun runSession(session: SenseVoiceSessionRunner.Session): String {
        val min=AudioRecord.getMinBufferSize(16_000,AudioFormat.CHANNEL_IN_MONO,AudioFormat.ENCODING_PCM_16BIT)
        check(min>0) { "录音格式不可用" }
        val record=AudioRecord(MediaRecorder.AudioSource.VOICE_RECOGNITION,16_000,
            AudioFormat.CHANNEL_IN_MONO,AudioFormat.ENCODING_PCM_16BIT,maxOf(min*2,32_000))
        val audio=try {
            check(record.state==AudioRecord.STATE_INITIALIZED)
            if(session.stopped) return ""
            record.startRecording()
            check(record.recordingState==AudioRecord.RECORDSTATE_RECORDING)
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
                }
                text.toString().trim()
            } finally { engine.release() }
        }
    }
    companion object { private val nativeLock=Any() }
}
