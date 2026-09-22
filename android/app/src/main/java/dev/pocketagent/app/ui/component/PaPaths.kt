package dev.pocketagent.app.ui.component

import android.content.Context
import android.os.Environment
import java.io.File

/**
 * App 落盘路径的**唯一真源**（与 README §9「存储与内存管理」同一口径）。
 *
 * 【为什么要有它】App 在 `filesDir` / `cacheDir` / 外部下载目录下自建多个子目录 ——
 * 会话快照、离线快照、轮次检查点、端侧语音模型、预览缓存、临时文件……这些路径名若散落
 * 各模块硬编码，就会出现「同一目录名多处写死、改一处漏一处」；而清理逻辑（缓存管理页、
 * `tools/ops/cleanup_project.py` 的定时清理）恰恰最容易因此漏删或误删。这里把**目录名与
 * 临时文件前缀**收敛到一处，各模块统一引用，改名只改这里。
 *
 * 【口径】（与 README §9 数据落点表一致）
 *  - `filesDir`   = 用户数据（随卸载清除，不计入「缓存」口径）；
 *  - `cacheDir`   = 缓存（系统低存储时也会回收）；
 *  - 外部下载目录 = 用户显式下载内容（清理**不得**删除）。
 *
 * 【不变量】目录名一经发布即为**磁盘契约**：改名必须配迁移，否则旧目录变孤儿、
 * 用户的历史缓存/快照会「凭空消失」。[CacheCleanupLogic] 的风险分级依赖本表定位清理目标。
 */
object PaPaths {

    // ---------------------------------------------------------------- filesDir（用户数据）

    /** 会话离线快照目录（`ThreadSnapshot`）。 */
    const val DIR_SNAPSHOT = "pa_snap"

    /** 轮次检查点目录（`TurnCheckpointStore`）。 */
    const val DIR_TURN_CHECKPOINTS = "turn-checkpoints"

    /** 端侧 SenseVoice 模型目录（`SenseVoiceModelManager`）。 */
    const val DIR_SENSEVOICE = "sensevoice-int8"

    // ---------------------------------------------------------------- cacheDir（缓存）

    /** 离线缓存根（其下再按 `OfflineStore.Partition.subDir` 分区，如 `pa_url`）。 */
    const val DIR_OFFLINE = "pa_offline"

    /** 预览内容缓存目录（`PreviewCache`）。 */
    const val DIR_PREVIEW = "pa_preview"

    /** PDF 渲染临时文件前缀（`PcFilePreview` 落 `pa_pdf_<nanos>.pdf`）。 */
    const val TMP_PDF_PREFIX = "pa_pdf_"

    /** zip 目录列表临时文件前缀（`PcFilePreview` 落 `pa_ziplist*.zip`）。 */
    const val TMP_ZIPLIST_PREFIX = "pa_ziplist"

    // ---------------------------------------------------------------- 其它

    /** 内置浏览器的 WebView 数据目录（系统在 `dataDir` 下托管，非我们创建但归我们清理）。 */
    const val DIR_APP_WEBVIEW = "app_webview"

    // ---------------------------------------------------------------- 便捷构造（统一「哪个目录 + 哪个名字」）

    fun snapshotDir(ctx: Context): File = File(ctx.filesDir, DIR_SNAPSHOT)

    fun turnCheckpointsDir(ctx: Context): File = File(ctx.filesDir, DIR_TURN_CHECKPOINTS)

    fun sensevoiceDir(ctx: Context): File = File(ctx.filesDir, DIR_SENSEVOICE)

    fun offlineRoot(ctx: Context): File = File(ctx.cacheDir, DIR_OFFLINE)

    fun previewDir(ctx: Context): File = File(ctx.cacheDir, DIR_PREVIEW)

    fun webviewDir(ctx: Context): File = File(ctx.applicationInfo.dataDir, DIR_APP_WEBVIEW)

    /** 用户显式下载目录（外部 files/DOWNLOADS；不可用时回落内建 filesDir）。 */
    fun downloadsDir(ctx: Context): File =
        ctx.getExternalFilesDir(Environment.DIRECTORY_DOWNLOADS) ?: ctx.filesDir
}
