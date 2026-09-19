package dev.pocketagent.app.ui.component

import android.content.Context
import android.util.Log
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.io.BufferedInputStream
import java.io.File
import java.io.FileInputStream
import java.io.FileOutputStream
import java.net.HttpURLConnection
import java.net.URL
import java.security.MessageDigest
import java.util.zip.ZipInputStream

/**
 * 端侧离线语音识别(Vosk)的模型管理:下载 / 解压 / 校验 / 删除。
 *
 * 【为什么存在】小米等 ROM 没有 SpeechRecognizer 识别服务(isRecognitionAvailable=false),
 * 云端识别又依赖 GMS 不可用 —— 唯一能落地的方案是把 vosk-model-small-cn-0.22(官方页标注
 * 42M)下载到 app 私有目录,用端侧引擎直接出文本。模型不进 APK(撑爆包体),首次启用语音
 * 时按需下载。
 *
 * 【分层】
 *  1. 纯状态机:[VoskModelState] + [voskModelReduce](零 Android 依赖,JVM 单测锁定);
 *  2. 纯文件操作:[VoskModelFiles](解压带 zip-slip 防护,JVM 单测锁定);
 *  3. 安装编排:[VoskModelInstaller](网络层经 [VoskModelFetcher] 注入,单测用 fake);
 *  4. 单例外壳:[VoskModelManager](context 注入 + 协程调度,UI 只碰它)。
 *
 * 【为什么没走 PaDownloader】PaDownloader 的落点是 MediaStore(系统"下载"目录)+ 任务中心
 * 上报,那是给用户可见文件的;模型必须放 app 私有目录(filesDir)且对用户透明,所以这里
 * 自带一个最小的 HttpURLConnection 流式下载(进度回调驱动状态机)。
 */
object VoskModelManager {

    /** 官方中文小模型(轻量,面向 Android/RPi,Apache 2.0)。 */
    const val MODEL_URL = "https://alphacephei.com/vosk/models/vosk-model-small-cn-0.22.zip"

    /** zip 内的顶层目录名,也是模型解压后的目录名。 */
    const val MODEL_DIR_NAME = "vosk-model-small-cn-0.22"

    /** 引导文案用的大小(官方页标注 42M,真实字节数下载完成后以 [VoskModelState.diskBytes] 为准)。 */
    const val MODEL_SIZE_HINT = "42MB"

    /**
     * 模型 zip 的 SHA-256(2026-09-19 官方源两次独立下载各算一次,完全一致;包体
     * 43898754 字节)。下载完成后先验哈希再解压:不匹配按失败清残留,杜绝传输
     * 损坏/被替换的包解压进模型目录。置空可跳过哈希校验(仍保留"可解压 + 必需
     * 文件齐备"的兜底校验)。
     */
    const val MODEL_SHA256 = "3af8b0e7e0f835ae9d414ce5df580237a3cfb08d586c9fbbb0f7ff29ad5b14ba"

    /** 模型根目录名(filesDir/vosk/)。 */
    private const val ROOT_DIR_NAME = "vosk"

    @Volatile
    private var appCtx: Context? = null

    private val ioScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    private val _state = MutableStateFlow(VoskModelState())

    /** 模型状态流:UI(会话页 chip / 设置页行)订阅它渲染。 */
    internal val state: StateFlow<VoskModelState> = _state.asStateFlow()

    /** 下载传输层:默认 HTTP 实现,单测可注入 fake(写 var 仅限测试与启动前配置)。 */
    @Volatile
    internal var fetcher: VoskModelFetcher = HttpVoskModelFetcher

    private var installJob: Job? = null

    /**
     * 幂等初始化:登记 application context 并扫描本地模型现状。
     * 会话页/设置页进页时各调一次都安全;磁盘扫描在 IO 协程(42MB 模型树的
     * 存在性/占用统计不占主线程),就绪态经 [state] 异步生效。
     */
    fun init(ctx: Context) {
        if (appCtx == null) appCtx = ctx.applicationContext
        ioScope.launch { refreshFromDisk() }
    }

    /** 扫描本地模型目录,校准状态(未下载 / 就绪+占用 / 半损坏→失败可重试)。 */
    fun refreshFromDisk() {
        val ctx = appCtx ?: return
        if (_state.value.phase == VoskModelPhase.DOWNLOADING) return
        val dir = modelDir(ctx)
        val valid = VoskModelFiles.isValidModelDir(dir)
        _state.update {
            voskModelReduce(
                it,
                VoskModelEvent.DiskRefreshed(
                    ready = valid,
                    diskBytes = if (valid) VoskModelFiles.dirSize(dir) else 0L,
                    dirExists = dir.exists(),
                ),
            )
        }
    }

    /** 模型是否就绪(引擎选择的回落依据)。 */
    fun isReady(): Boolean = _state.value.phase == VoskModelPhase.READY

    /** 就绪时返回模型目录绝对路径(供 Vosk 引擎加载);未就绪返回 null。 */
    fun modelPath(): String? {
        val ctx = appCtx ?: return null
        val dir = modelDir(ctx)
        return if (VoskModelFiles.isValidModelDir(dir)) dir.absolutePath else null
    }

    /**
     * 触发下载(失败可重试,重复调用幂等):已就绪 / 下载中时是 no-op。
     * 下载 → 解压 → 校验全在 IO 协程,进度事件驱动状态机。
     */
    fun startDownload() {
        val ctx = appCtx ?: return
        // READY 挡板必须连磁盘一起复核:状态可能滞留 READY(扫描后目录才半损坏),
        // 只看相位会让"重新下载"入口被假就绪挡成 no-op,用户永远走不出反复失败。
        if (_state.value.phase == VoskModelPhase.READY && VoskModelFiles.isValidModelDir(modelDir(ctx))) return
        if (installJob?.isActive == true) return
        Log.d("PaVoice", "Vosk 模型开始下载 url=$MODEL_URL")
        installJob = ioScope.launch {
            VoskModelInstaller(fetcher, rootDir(ctx)) { event ->
                // 日志留在单例外壳(纯逻辑类不碰 android.util.Log,JVM 单测可直接跑)
                if (event is VoskModelEvent.Failed) {
                    Log.d("PaVoice", "Vosk 模型安装失败:${event.message}")
                }
                _state.update { voskModelReduce(it, event) }
            }.install(MODEL_URL, MODEL_SHA256)
        }
    }

    /** 删除模型释放空间(下载进行中不允许删,避免与安装过程互踩)。 */
    fun deleteModel() {
        val ctx = appCtx ?: return
        if (installJob?.isActive == true) return
        ioScope.launch {
            VoskModelFiles.deleteRecursively(modelDir(ctx))
            File(rootDir(ctx), "$MODEL_DIR_NAME.zip").delete()
            _state.update { voskModelReduce(it, VoskModelEvent.Deleted) }
            Log.d("PaVoice", "Vosk 模型已删除")
        }
    }

    private fun rootDir(ctx: Context) = File(ctx.filesDir, ROOT_DIR_NAME)

    private fun modelDir(ctx: Context) = File(rootDir(ctx), MODEL_DIR_NAME)
}

/** 模型生命周期阶段:未下载 / 下载中 / 就绪 / 失败(可重试)。 */
internal enum class VoskModelPhase { NOT_DOWNLOADED, DOWNLOADING, READY, FAILED }

/** 模型状态(纯数据):进度 0..1,未知总长时 progress=0 只涨字节数。 */
internal data class VoskModelState(
    val phase: VoskModelPhase = VoskModelPhase.NOT_DOWNLOADED,
    /** 下载进度 0..1;totalBytes 未知(服务器无 content-length)时保持 0,UI 只显示字节数。 */
    val progress: Float = 0f,
    val downloadedBytes: Long = 0L,
    val totalBytes: Long = 0L,
    /** 失败原因(FAILED 态的人话消息)/ 附加说明。 */
    val message: String = "",
    /** 模型目录磁盘占用(就绪态显示用)。 */
    val diskBytes: Long = 0L,
) {
    val ready: Boolean get() = phase == VoskModelPhase.READY
    val downloading: Boolean get() = phase == VoskModelPhase.DOWNLOADING
}

/** 状态机输入事件:磁盘扫描 / 下载开始 / 进度 / 解压 / 成功 / 失败 / 删除。 */
internal sealed interface VoskModelEvent {
    /**
     * 磁盘扫描结果。[dirExists] 区分"校验失败"的两种成因:目录还在但三件套
     * 不齐 = 半损坏(回 FAILED 引导重下);目录整个消失 = 被清理(回未下载)。
     * 默认 false 兼容"目录不在"场景的既有构造点。
     */
    data class DiskRefreshed(
        val ready: Boolean,
        val diskBytes: Long,
        val dirExists: Boolean = false,
    ) : VoskModelEvent

    data object DownloadStarted : VoskModelEvent

    data class DownloadProgress(val downloadedBytes: Long, val totalBytes: Long) : VoskModelEvent

    data object Extracting : VoskModelEvent

    data class Succeeded(val diskBytes: Long) : VoskModelEvent

    data class Failed(val message: String) : VoskModelEvent

    data object Deleted : VoskModelEvent
}

/**
 * 模型状态机主转移(JVM 单测锁定):
 *  · 进度/解压事件只在下载中生效(乱序事件不污染终态);
 *  · 失败保留已下载字节数展示但回 FAILED,下次 startDownload 从头来(残留会先清);
 *  · DiskRefreshed(false) 按成因分流:目录在但校验不过 = 半损坏,回 FAILED
 *    (可重试下载,不落"未下载"伪装从没下过);目录消失才回未下载。
 */
internal fun voskModelReduce(current: VoskModelState, event: VoskModelEvent): VoskModelState = when (event) {
    is VoskModelEvent.DiskRefreshed ->
        when {
            event.ready ->
                current.copy(
                    phase = VoskModelPhase.READY, progress = 1f,
                    diskBytes = event.diskBytes, message = "",
                )
            // 目录还在但必需文件不齐(conf/final.mdl/解码图缺一)= 半损坏:
            // 回 FAILED 留人话原因引导重下,避免滞留 READY 每轮识别重复失败
            // 而设置页仍显示"已下载"。
            event.dirExists ->
                current.copy(
                    phase = VoskModelPhase.FAILED, diskBytes = 0L,
                    message = "模型文件不完整或已损坏,请重新下载",
                )
            // 目录整个消失 = 被用户/清理软件删掉:回未下载,引导首次下载
            else -> VoskModelState(phase = VoskModelPhase.NOT_DOWNLOADED)
        }
    VoskModelEvent.DownloadStarted ->
        current.copy(
            phase = VoskModelPhase.DOWNLOADING, progress = 0f,
            downloadedBytes = 0L, totalBytes = 0L, message = "",
        )
    is VoskModelEvent.DownloadProgress ->
        if (current.phase != VoskModelPhase.DOWNLOADING) {
            current
        } else {
            current.copy(
                downloadedBytes = event.downloadedBytes,
                totalBytes = event.totalBytes,
                progress = if (event.totalBytes > 0) {
                    (event.downloadedBytes.toFloat() / event.totalBytes).coerceIn(0f, 1f)
                } else {
                    0f
                },
            )
        }
    VoskModelEvent.Extracting ->
        if (current.phase != VoskModelPhase.DOWNLOADING) current else current.copy(progress = 1f, message = "解压校验中")
    is VoskModelEvent.Succeeded ->
        current.copy(
            phase = VoskModelPhase.READY, progress = 1f,
            diskBytes = event.diskBytes, message = "",
        )
    is VoskModelEvent.Failed ->
        current.copy(phase = VoskModelPhase.FAILED, message = event.message)
    VoskModelEvent.Deleted -> VoskModelState(phase = VoskModelPhase.NOT_DOWNLOADED)
}

/**
 * 下载传输层抽象:默认 [HttpVoskModelFetcher],单测注入 fake(可编排进度/失败)。
 */
internal fun interface VoskModelFetcher {
    /** 下载 url 到 target;onProgress 回调 (已下载字节, 总字节,总未知时 <=0)。 */
    fun fetch(url: String, target: File, onProgress: (Long, Long) -> Unit)
}

/** HttpURLConnection 流式下载:进度逐块回调;内容长度已知但没下满按失败处理。 */
internal object HttpVoskModelFetcher : VoskModelFetcher {
    override fun fetch(url: String, target: File, onProgress: (Long, Long) -> Unit) {
        val conn = (URL(url).openConnection() as HttpURLConnection).apply {
            connectTimeout = 15_000
            readTimeout = 60_000
            instanceFollowRedirects = true
            setRequestProperty("Accept", "*/*")
        }
        conn.connect()
        try {
            val code = conn.responseCode
            if (code !in 200..299) throw IllegalStateException("服务器返回 HTTP $code")
            val total = conn.contentLengthLong
            target.parentFile?.mkdirs()
            conn.inputStream.use { input ->
                FileOutputStream(target).use { out ->
                    val buf = ByteArray(64 * 1024)
                    var written = 0L
                    while (true) {
                        val n = input.read(buf)
                        if (n < 0) break
                        out.write(buf, 0, n)
                        written += n
                        onProgress(written, total)
                    }
                    out.flush()
                    if (total > 0 && written < total) {
                        throw IllegalStateException("传输不完整:$written/$total")
                    }
                }
            }
        } finally {
            conn.disconnect()
        }
    }
}

/**
 * 安装编排:清残留 → 下载 zip → SHA-256 校验 → 解压(带 zip-slip 防护)→ 校验必需文件 → 清 zip。
 * 全程事件驱动状态机;任何一步失败都清掉半成品并报 FAILED(可重试)。
 *
 * 【校验说明】下载完成后先验官方包 SHA-256(见 [VoskModelManager.MODEL_SHA256],
 * 期望值为空 = 跳过,容未来换包未及时更新常量),不匹配即拒绝解压;哈希过了再以
 * "zip 可解压 + Vosk 必需文件齐备(conf/model.conf、am/final.mdl、graph/HCLr.fst
 * 或 graph/HCLG.fst)"作为完整性兜底校验。
 */
internal class VoskModelInstaller(
    private val fetcher: VoskModelFetcher,
    private val rootDir: File,
    private val onEvent: (VoskModelEvent) -> Unit,
) {
    fun install(url: String, expectedSha256: String = "") {
        val zip = File(rootDir, "${VoskModelManager.MODEL_DIR_NAME}.zip")
        val modelDir = File(rootDir, VoskModelManager.MODEL_DIR_NAME)
        try {
            rootDir.mkdirs()
            // 半途残留一律清掉:zip 重下、model 重解,绝不复用可疑数据
            zip.delete()
            VoskModelFiles.deleteRecursively(modelDir)
            onEvent(VoskModelEvent.DownloadStarted)
            fetcher.fetch(url, zip) { downloaded, total ->
                onEvent(VoskModelEvent.DownloadProgress(downloaded, total))
            }
            onEvent(VoskModelEvent.Extracting)
            // 先验哈希再解压:坏包绝不落进模型目录(不匹配走统一 catch 清残留报 FAILED)
            if (!VoskModelFiles.sha256Matches(zip, expectedSha256)) {
                throw IllegalStateException("模型包校验和不符(下载可能损坏),已清理,请重试")
            }
            VoskModelFiles.unzip(zip, rootDir)
            if (!VoskModelFiles.isValidModelDir(modelDir)) {
                throw IllegalStateException("模型包内容不完整")
            }
            zip.delete()
            onEvent(VoskModelEvent.Succeeded(VoskModelFiles.dirSize(modelDir)))
        } catch (t: Throwable) {
            VoskModelFiles.deleteRecursively(modelDir)
            zip.delete()
            onEvent(VoskModelEvent.Failed(t.message ?: "未知错误"))
        }
    }
}

/** 模型文件的纯文件操作(全部 JVM 可测):目录校验 / 安全解压 / 占用统计 / 递归删除。 */
internal object VoskModelFiles {

    /**
     * 目录是否构成可加载的 Vosk 模型:必需三件套 —— 声学模型、模型配置、解码图
     * (小模型用 lookahead 解码图 HCLr.fst,大模型用 HCLG.fst,两者取其一)。
     * 与 org.vosk.Model 原生加载的前置检查对齐,提前到下载完成时把关。
     */
    fun isValidModelDir(dir: File): Boolean =
        dir.isDirectory &&
            File(dir, "conf/model.conf").isFile &&
            File(dir, "am/final.mdl").isFile &&
            (File(dir, "graph/HCLr.fst").isFile || File(dir, "graph/HCLG.fst").isFile)

    /**
     * 解压 zip 到 targetDir。
     * 【防 zip-slip】恶意压缩包里的 ../ 逃逸路径一律拒绝 —— 模型 URL 虽固定,
     * 但解压器按"不信任输入"写,未来换镜像/换模型不埋雷。
     */
    fun unzip(zip: File, targetDir: File) {
        ZipInputStream(BufferedInputStream(FileInputStream(zip))).use { zis ->
            val canonicalTarget = targetDir.canonicalFile
            while (true) {
                val entry = zis.nextEntry ?: break
                val out = File(targetDir, entry.name)
                if (!out.canonicalFile.path.startsWith(canonicalTarget.path + File.separator) &&
                    out.canonicalFile != canonicalTarget
                ) {
                    throw SecurityException("压缩包内非法路径:${entry.name}")
                }
                if (entry.isDirectory) {
                    out.mkdirs()
                } else {
                    out.parentFile?.mkdirs()
                    FileOutputStream(out).use { zis.copyTo(it) }
                }
                zis.closeEntry()
            }
        }
    }

    /** 目录磁盘占用(递归文件字节数和;目录不存在返回 0)。 */
    fun dirSize(dir: File): Long =
        if (!dir.exists()) 0L else dir.walkTopDown().filter { it.isFile }.sumOf { it.length() }

    /** 递归删除(不存在视为已删成功)。 */
    fun deleteRecursively(dir: File): Boolean = if (!dir.exists()) true else dir.deleteRecursively()

    /** 文件的 SHA-256,小写十六进制串(JVM 可测,MessageDigest 平台无关)。 */
    fun sha256Hex(file: File): String {
        val md = MessageDigest.getInstance("SHA-256")
        FileInputStream(file).use { input ->
            val buf = ByteArray(64 * 1024)
            while (true) {
                val n = input.read(buf)
                if (n < 0) break
                md.update(buf, 0, n)
            }
        }
        return md.digest().joinToString("") { "%02x".format(it) }
    }

    /**
     * 哈希校验:期望值空白 = 跳过(返回 true,校验降级为后续的必需文件兜底);
     * 否则与实际值比对(忽略大小写,兼容大写形式的哈希常量)。
     */
    fun sha256Matches(file: File, expectedHex: String): Boolean {
        if (expectedHex.isBlank()) return true
        return sha256Hex(file).equals(expectedHex.trim(), ignoreCase = true)
    }
}
