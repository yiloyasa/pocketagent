package dev.pocketagent.app.ui.component

import android.content.Context
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.io.File
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.security.MessageDigest
import java.util.concurrent.atomic.AtomicBoolean

internal data class SenseVoiceModelState(
    val ready: Boolean = false,
    val preparing: Boolean = false,
    val preparedBytes: Long = 0,
    val error: String? = null,
)

/**
 * SenseVoice is shipped inside the APK. First launch only extracts verified
 * assets into app-private storage; there is no runtime model download or
 * external model chooser.
 */
internal object SenseVoiceModelManager {
    const val TOTAL_BYTES = 239_549_735L
    private const val ASSET_DIR = "sensevoice"
    private val files = listOf(
        SenseVoiceModelFile("model.int8.onnx", 239_233_841L, "c71f0ce00bec95b07744e116345e33d8cbbe08cef896382cf907bf4b51a2cd51"),
        SenseVoiceModelFile("tokens.txt", 315_894L, "f449eb28dc567533d7fa59be34e2abca8784f771850c78a47fb731a31429a1dc"),
    )
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val busy = AtomicBoolean(false)
    private val mutable = MutableStateFlow(SenseVoiceModelState())
    val state = mutable.asStateFlow()
    @Volatile private var appContext: Context? = null
    @Volatile private var root: File? = null

    @Synchronized fun init(context: Context) {
        if (appContext == null) {
            appContext = context.applicationContext
            root = PaPaths.sensevoiceDir(context.applicationContext)
        }
        prepareBundledModel()
    }

    fun modelPath(): String? = root?.absolutePath?.takeIf { state.value.ready }

    fun prepareBundledModel() {
        val context = appContext ?: return
        val dir = root ?: return
        if (state.value.ready) return
        if (!busy.compareAndSet(false, true)) return
        mutable.value = SenseVoiceModelState(preparing = true)
        scope.launch {
            try {
                check(dir.mkdirs() || dir.isDirectory) { "无法创建端侧语音目录" }
                var completed = 0L
                for (spec in files) {
                    val target = File(dir, spec.name)
                    if (valid(target, spec)) {
                        completed += spec.size
                        mutable.value = SenseVoiceModelState(preparing = true, preparedBytes = completed)
                        continue
                    }
                    val partial = File(dir, spec.name + ".part")
                    partial.delete()
                    context.assets.open("$ASSET_DIR/${spec.name}").use { input ->
                        partial.outputStream().use { output ->
                            val buffer = ByteArray(128 * 1024)
                            var copied = 0L
                            while (true) {
                                val n = input.read(buffer)
                                if (n < 0) break
                                copied += n
                                check(copied <= spec.size) { "内置语音模型长度异常" }
                                output.write(buffer, 0, n)
                                mutable.value = SenseVoiceModelState(preparing = true, preparedBytes = completed + copied)
                            }
                        }
                    }
                    check(valid(partial, spec)) { "内置语音模型校验失败，请重新安装 App" }
                    Files.move(partial.toPath(), target.toPath(), StandardCopyOption.REPLACE_EXISTING)
                    completed += spec.size
                }
                mutable.value = SenseVoiceModelState(ready = true, preparedBytes = TOTAL_BYTES)
            } catch (e: Exception) {
                mutable.value = SenseVoiceModelState(error = e.message ?: "端侧语音初始化失败")
            } finally { busy.set(false) }
        }
    }

    private fun valid(file: File, spec: SenseVoiceModelFile): Boolean =
        file.isFile && file.length() == spec.size && sha256(file) == spec.sha256

    private fun sha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(128 * 1024)
            while (true) {
                val n = input.read(buffer)
                if (n < 0) break
                digest.update(buffer, 0, n)
            }
        }
        return digest.digest().joinToString("") { "%02x".format(it.toInt() and 255) }
    }
}
