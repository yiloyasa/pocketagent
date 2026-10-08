package dev.pocketagent.app.ui.component

import android.content.Context
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.io.File
import java.util.concurrent.atomic.AtomicBoolean

internal data class SenseVoiceModelState(
    val ready: Boolean = false,
    val preparing: Boolean = false,
    val preparedBytes: Long = 0,
    val error: String? = null,
    /** 【2026-10-09 分发资格批】模型不在本地且未在下载:需先触发下载才有语音可用。 */
    val needsDownload: Boolean = false,
    /**
     * 【2026-10-09 修复轮】曾尝试下载且失败未恢复(显式失败标志)。
     *
     * 【为什么必须单独立位】此前 needsDownload=true 同时覆盖"从没下载过"与
     * "下载失败回退"两种落点,设置页唯一判别通道是 error 文案是否等于
     * [NEEDS_DOWNLOAD_MESSAGE] 占位 —— 文案一改(或失败恰好带出同文案)判定就翻。
     * 现在失败由本位明说:置位只发生在 [prepareBundledModel] 的失败路径;
     * ready / 重校验通过(verifyLocalModel)即随新状态归零 = "已恢复"。
     * 消费方(SettingsScreen.senseVoiceDownloadAction)以它判"重试下载"。
     */
    val downloadFailed: Boolean = false,
)

/**
 * SenseVoice 模型管理(2026-10-09 分发资格批起**不再随 APK 打包**):
 * 239MB ONNX 曾作为 assets 打进 APK(220MB 的绝对大头),现改为按需获取——
 *
 *  · **本地已有且校验通过**(filesDir,SHA-256 逐字节复核)→ 直接就绪;
 *  · **本地没有** → 停在 [SenseVoiceModelState.needsDownload] 态,零网络零拷贝
 *    (首启零成本);直到语音被真正启用(用户点麦克风走 [prepareBundledModel]
 *    的重试链,或后续设置页接入下载按钮)才经 [SenseVoiceModelFiles] 触发
 *    下载/断点续传;
 *  · 下载源与 SHA-256 烧录常量同源:仓库构建脚本 android/build-ascii.ps1 用的
 *    就是这组固定上游(revision 钉死)+ 双镜像 + 全量哈希校验,手机端沿用同一
 *    常量,校验语义与旧"内置解压复核"完全一致。
 *
 * 【UI 接线现状】语音设置入口在 SettingsScreen(禁碰文件),其现有状态卡已能
 * 渲染 preparing/preparedBytes/error("初始化 X%"/错误文案),下载进度沿用该
 * 通道展示;needsDownload 目前无消费者 —— TODO(主模型裁决):在设置页语音卡
 * 加"下载语音模型(约 228 MB)"按钮,点击调 [prepareBundledModel] 即可。
 */
internal object SenseVoiceModelManager {
    const val TOTAL_BYTES = 239_549_735L

    /** 需要下载时的状态文案(供既有 error 展示通道透出;刻意避开"校验失败/损坏/
     *  空间/磁盘"字样,防止 classifySenseVoiceManagerError 误归类)。 */
    const val NEEDS_DOWNLOAD_MESSAGE = "语音模型未下载(首次使用需联网下载,约 228 MB)"

    private val files = listOf(
        SenseVoiceModelFile("model.int8.onnx", 239_233_841L, "c71f0ce00bec95b07744e116345e33d8cbbe08cef896382cf907bf4b51a2cd51"),
        SenseVoiceModelFile("tokens.txt", 315_894L, "f449eb28dc567533d7fa59be34e2abca8784f771850c78a47fb731a31429a1dc"),
    )
    // 与 android/build-ascii.ps1 双源同款:HF 主源 + 国内镜像,固定 revision。
    // SenseVoiceModelFiles 按 base + 文件名逐字下载,.part 断点续传与哈希校验内置。
    private val downloadBases = listOf(
        "https://huggingface.co/csukuangfj/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17/resolve/2365baeacb507f821a0c8120fcee3d484dba7a07/",
        "https://hf-mirror.com/csukuangfj/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17/resolve/2365baeacb507f821a0c8120fcee3d484dba7a07/",
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
        verifyLocalModel()
    }

    fun modelPath(): String? = root?.absolutePath?.takeIf { state.value.ready }

    /**
     * 只校验不下载:本地齐且哈希对 → ready;否则停在 needsDownload 态(零网络,
     * 首启零成本)。进 Chat/语音设置页的 [init] 都走这里,绝不意外触发 239MB 下载。
     */
    fun verifyLocalModel() {
        val dir = root ?: return
        if (state.value.ready || state.value.preparing) return
        if (!busy.compareAndSet(false, true)) return
        mutable.value = SenseVoiceModelState(preparing = true)
        scope.launch {
            try {
                check(dir.mkdirs() || dir.isDirectory) { "无法创建端侧语音目录" }
                val probe = SenseVoiceModelFiles(dir, downloadBases.first(), files)
                mutable.value = if (probe.isReady()) {
                    SenseVoiceModelState(ready = true, preparedBytes = TOTAL_BYTES)
                } else {
                    // TODO(UI 接线,主模型裁决):设置页语音卡按 needsDownload 显示
                    // "下载语音模型"按钮 → 调 prepareBundledModel()。现状:ChatScreen
                    // 点麦克风的 reportVoiceUnavailable 重试链会因 error 非空自动调它,
                    // 下载进度经 preparing/preparedBytes 在既有状态卡透出。
                    SenseVoiceModelState(needsDownload = true, error = NEEDS_DOWNLOAD_MESSAGE)
                }
            } catch (e: Exception) {
                mutable.value = SenseVoiceModelState(error = e.message ?: "端侧语音初始化失败")
            } finally { busy.set(false) }
        }
    }

    /**
     * 就绪模型(旧入口,语义升级):本地齐 → 直接就绪;缺 → 触发下载/断点续传
     * (HF 主源失败自动落 hf-mirror 镜像,SHA-256 沿烧录常量逐字节复核)。
     * 幂等,busy 挡板防重入;只在语音被真正启用时才会被调用到下载分支。
     */
    fun prepareBundledModel() {
        val dir = root ?: return
        if (state.value.ready) return
        if (!busy.compareAndSet(false, true)) return
        mutable.value = SenseVoiceModelState(preparing = true, preparedBytes = state.value.preparedBytes)
        scope.launch {
            try {
                check(dir.mkdirs() || dir.isDirectory) { "无法创建端侧语音目录" }
                val probe = SenseVoiceModelFiles(dir, downloadBases.first(), files)
                if (probe.isReady()) {
                    mutable.value = SenseVoiceModelState(ready = true, preparedBytes = TOTAL_BYTES)
                    return@launch
                }
                var lastError: Exception? = null
                for (base in downloadBases) {
                    try {
                        SenseVoiceModelFiles(dir, base, files).install { done ->
                            mutable.value = SenseVoiceModelState(preparing = true, preparedBytes = done)
                        }
                        lastError = null
                        break
                    } catch (e: Exception) {
                        // 记住最后一个源的错误,换下一个镜像;.part 保留供断点续传
                        lastError = e
                    }
                }
                if (lastError != null) throw lastError
                mutable.value = SenseVoiceModelState(ready = true, preparedBytes = TOTAL_BYTES)
            } catch (e: Exception) {
                mutable.value = SenseVoiceModelState(
                    needsDownload = true,
                    // 【显式失败标志】下载失败落点:与"从没下载过"(downloadFailed=false)
                    // 在状态位上可判别,UI 不再只靠 error 文案比对占位常量。
                    downloadFailed = true,
                    error = e.message ?: "语音模型下载失败",
                )
            } finally { busy.set(false) }
        }
    }
}
