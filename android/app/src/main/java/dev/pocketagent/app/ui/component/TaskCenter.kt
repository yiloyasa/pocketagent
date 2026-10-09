package dev.pocketagent.app.ui.component

import android.content.Context
import android.content.SharedPreferences
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * 任务中心(设计规范 §2.6 / 计划文档 §1.6)。
 *
 * 【它不拥有状态】消息类任务**不缓存轮次的副本**,只持有 `threadId` 引用,
 * 进度与结果实时去 `TurnSession.state(threadId)` 读。
 * 这样不会出现"任务中心显示完成了、聊天页还在转"的双份真相。
 *
 * 【持久化】未读数与任务清单落到 SharedPreferences:
 * 杀进程重启后未读不丢,重启时仍在 RUNNING 的任务标记为 [Status.INTERRUPTED] 并给出重试入口。
 */
object TaskCenter {

    /** 任务分类 —— 对应任务中心顶部的两个标签页。 */
    enum class Kind { MESSAGE, DOWNLOAD }

    enum class Status { RUNNING, DONE, FAILED, CANCELED, INTERRUPTED }

    /**
     * @param id        任务唯一标识(消息类 = "msg:$threadId";下载类 = "dl:$fileKey")
     * @param threadId  关联会话(下载类可为空)
     * @param rounds    同会话合并后的轮次数(连续多轮合并为一条,避免刷屏)
     */
    class Task(
        val id: String,
        val threadId: String,
        val kind: Kind,
        title: String,
        status: Status,
        progress: Float = 0f,
        detail: String = "",
        read: Boolean = false,
        startedAt: Long = System.currentTimeMillis(),
        rounds: Int = 1,
        /** 下载类:文件落在手机上的位置(content:// 可直接打开,或绝对路径)。 */
        localUri: String = "",
        /** 下载类:来源会话名(展示"从哪个会话下载的")。 */
        sourceTitle: String = "",
        /** 下载类:人类可读的大小,如 1.2MB。 */
        sizeText: String = "",
    ) {
        var title: String by mutableStateOf(title)
        var status: Status by mutableStateOf(status)
        var progress: Float by mutableStateOf(progress)
        var detail: String by mutableStateOf(detail)
        var read: Boolean by mutableStateOf(read)
        var startedAt: Long by mutableStateOf(startedAt)
        var rounds: Int by mutableStateOf(rounds)
        var localUri: String by mutableStateOf(localUri)
        var sourceTitle: String by mutableStateOf(sourceTitle)
        var sizeText: String by mutableStateOf(sizeText)
    }
    // 【为什么用 mutableStateOf 委托】本对象对任务的修改全是"原地改字段"(updateDownload/
    // finishDownload/markRead/attachLocalUriIfEmpty...),而 _tasks 是 StateFlow<List<Task>>:
    // StateFlow 按元素 equals 决定是否发射,列表实例换了但元素还是同一批 → 永远不发射,
    // UI 读到的字段值就一直是旧的。字段改成快照状态后,写入即通知正在组合中读取它的
    // Composable,不再依赖列表发射。

    private var prefs: SharedPreferences? = null

    /**
     * 用户是否正停留在任务中心页(由 [dev.pocketagent.app.ui.TaskCenterScreen] 的
     * DisposableEffect 维护)。为 true 时新产生/新完成的任务直接算已读 ——
     * 设计规范 §8:"任务完成时用户正在任务中心 → 该条不增加未读(已看见)"。
     */
    private var viewing = false

    fun setViewing(v: Boolean) {
        viewing = v
    }

    private val _tasks = MutableStateFlow<List<Task>>(emptyList())
    val tasks: StateFlow<List<Task>> = _tasks.asStateFlow()

    private val _unreadCount = MutableStateFlow(0)

    /** 驱动侧边导航「任务中心」图标右上角的角标。 */
    val unreadCount: StateFlow<Int> = _unreadCount.asStateFlow()

    /** 消息处理类任务。 */
    fun messages(): List<Task> = _tasks.value.filter { it.kind == Kind.MESSAGE }

    /** 文件下载类任务。 */
    fun downloads(): List<Task> = _tasks.value.filter { it.kind == Kind.DOWNLOAD }

    fun init(ctx: Context) {
        if (prefs != null) return
        prefs = ctx.applicationContext.getSharedPreferences("task_center", Context.MODE_PRIVATE)
        restore()
    }

    // ---------------------------------------------------------------- 消息处理

    /**
     * 登记一轮消息处理。同一会话已存在未结束的任务时**合并**为一条并累加轮次。
     * @return 任务 id
     */
    fun trackMessage(threadId: String, title: String): String {
        val id = "msg:$threadId"
        val existing = _tasks.value.firstOrNull { it.id == id }
        if (existing != null && existing.status == Status.RUNNING) {
            existing.rounds += 1
            existing.title = title
            existing.read = false
            existing.startedAt = System.currentTimeMillis()
        } else {
            _tasks.value = _tasks.value.filterNot { it.id == id } + Task(
                id = id,
                threadId = threadId,
                kind = Kind.MESSAGE,
                title = title,
                status = Status.RUNNING,
            )
        }
        persist()
        return id
    }

    /** 轮次结束。用户若正停留在任务中心由调用方调 [markRead]。 */
    fun finishMessage(threadId: String, ok: Boolean, detail: String = "") {
        val t = _tasks.value.firstOrNull { it.id == "msg:$threadId" } ?: return
        t.status = if (ok) Status.DONE else Status.FAILED
        t.progress = if (ok) 1f else t.progress
        t.detail = detail
        persist()
    }

    // ---------------------------------------------------------------- 文件下载

    fun trackDownload(fileKey: String, title: String, threadId: String = ""): String {
        val id = "dl:$fileKey"
        val existing = _tasks.value.firstOrNull { it.id == id }
        when {
            existing == null -> {
                _tasks.value = _tasks.value + Task(
                    id = id,
                    threadId = threadId,
                    kind = Kind.DOWNLOAD,
                    title = title,
                    status = Status.RUNNING,
                )
                persist()
            }
            // 【防重入】同一条已在下载中:直接复用条目,不重置进度 ——
            // 重置会让仍在写入的流把进度条打回零再爬上来,视觉上像"越下越少"。
            existing.status == Status.RUNNING -> Unit
            // 【重新下载】对已结束(完成/失败/中断)的条目**原地复活**:清掉旧进度
            // 与旧文件位置,状态回到进行中 —— 任务中心立刻给出"在动"的反馈,
            // 而不是一条点了重下载却纹丝不动的死记录(旧文件位置若不 clear,
            // 失败后会残留指向已失效文件的 localUri)。
            else -> {
                existing.status = Status.RUNNING
                existing.progress = 0f
                existing.detail = ""
                existing.read = false
                existing.startedAt = System.currentTimeMillis()
                existing.localUri = ""
                existing.sizeText = ""
                persist()
            }
        }
        return id
    }

    fun updateDownload(id: String, progress: Float, detail: String = "") {
        val t = _tasks.value.firstOrNull { it.id == id } ?: return
        // 【单调】分块下载的进度偶尔会因为重试/分片乱序而回退,表现为"进度条倒退"。
        // 这里保证只增不减。
        t.progress = maxOf(t.progress, progress.coerceIn(0f, 1f))
        if (detail.isNotEmpty()) t.detail = detail
        // 【节流】下载分块(64KB/块)到达频繁:512MB 文件 = 8192 次。
        // UI 每块都刷新,但磁盘写合并(每 300ms 最多一次),
        // 完成时的 finishDownload(immediate) 兜底,不会丢最终状态。
        persist(immediate = false)
    }

    /** 下载完成:记录文件位置与大小,供列表点击打开。 */
    fun finishDownload(id: String, ok: Boolean, detail: String = "", localUri: String = "", sizeText: String = "") {
        val t = _tasks.value.firstOrNull { it.id == id } ?: return
        t.status = if (ok) Status.DONE else Status.FAILED
        t.progress = if (ok) 1f else t.progress
        t.detail = detail
        if (localUri.isNotEmpty()) t.localUri = localUri
        if (sizeText.isNotEmpty()) t.sizeText = sizeText
        persist()
    }

    /** 下载类任务登记时补来源会话。 */
    fun annotateDownload(id: String, sourceTitle: String) {
        val t = _tasks.value.firstOrNull { it.id == id } ?: return
        if (sourceTitle.isNotEmpty()) t.sourceTitle = sourceTitle
        persist()
    }

    /**
     * 旧版条目补文件位置:早期版本 localUri 不落盘,升级后历史条目全都丢了
     * "文件在哪" —— 查看/分享图标因此集体消失。按文件名在「下载/PocketAgent」
     * 里找回来后调用这里,**只填空值**(不覆盖新版本的真相)。
     */
    fun attachLocalUriIfEmpty(id: String, uri: String) {
        if (uri.isEmpty()) return
        val t = _tasks.value.firstOrNull { it.id == id } ?: return
        if (t.localUri.isNotEmpty()) return
        t.localUri = uri
        persist()
    }

    // ---------------------------------------------------------------- 已读与检索

    fun markRead(id: String) {
        val t = _tasks.value.firstOrNull { it.id == id } ?: return
        if (t.read) return
        t.read = true
        persist()
    }

    fun markAllRead() {
        if (_tasks.value.none { !it.read }) return
        _tasks.value.forEach { it.read = true }
        persist()
    }

    fun remove(id: String) {
        _tasks.value = _tasks.value.filterNot { it.id == id }
        persist()
    }

    /**
     * 删掉某个会话关联的任务条目(删除会话时用)。
     *
     * 【为什么删会话要管任务中心】消息类任务的 id 就是 `msg:$threadId`(见 [Task] 注释),
     * 会话删了它还在 —— 用户会在任务中心看到一条永远点不开的记录,而那个会话已经不存在。
     * 下载类也可能带 threadId([trackDownload] 的第三参),同理一起清。
     *
     * 【它不取消任何东西】本函数只删条目。若该会话的轮次仍在电脑上跑,删掉条目会丢掉
     * "它还在跑"的可见性 —— 所以调用方(删除流程)只在 **daemon 确认删除成功之后** 调它;
     * 那时轮次要么已结束,要么 daemon 根本不会同意删(它会回"会话正在运行")。
     *
     * 幂等:没有该会话的条目时是 no-op(连落盘都跳过)。
     */
    fun removeByThread(threadId: String) {
        if (threadId.isEmpty()) return
        val before = _tasks.value.size
        _tasks.value = _tasks.value.filterNot { it.threadId == threadId }
        if (_tasks.value.size != before) persist()
    }

    fun clearFinished() {
        _tasks.value = _tasks.value.filter { it.status == Status.RUNNING }
        persist()
    }

    // ---------------------------------------------------------------- 内部

    /** 下载分块高频进度更新的落盘节流间隔。 */
    private const val PERSIST_THROTTLE_MS = 300L

    /** 滚动上限:每类(消息/下载)最多保留这么多条「已结束且已读」的任务,最旧先淘汰。 */
    private const val MAX_FINISHED_PER_KIND = 50

    /** 落盘走单线程串行队列:字符串构建与写盘都不占调用方线程。 */
    private val persistExecutor = java.util.concurrent.Executors.newSingleThreadExecutor { r ->
        Thread(r, "pa-task-persist").apply { isDaemon = true }
    }

    @Volatile
    private var lastDiskWrite = 0L

    /**
     * @param immediate true(默认)= 状态性变化(登记/完成/删除/已读),立即落盘;
     *                 false = 高频进度更新,300ms 合并写盘(UI 不受影响,永远实时)。
     */
    private fun persist(immediate: Boolean = true) {
        applyCap()
        // 【必须】StateFlow 只按引用比对,原地修改 List 里的 Task 不会通知订阅者。
        // 这里统一重新发射一次,保证任务中心列表与角标能收到变更。
        _tasks.value = _tasks.value.toList()

        // 【角标刷新】未读数总是从任务清单重算 —— 这样"开始一轮/结束一轮"
        // 也会把侧边栏角标点起来(此前 refreshUnread 只在已读操作里调,角标恒 0)。
        if (viewing) _tasks.value.forEach { it.read = true }
        _unreadCount.value = _tasks.value.count { !it.read }

        val p = prefs ?: return
        if (immediate) {
            lastDiskWrite = System.currentTimeMillis()
            persistExecutor.execute { writeDisk(p) }
            return
        }
        val now = System.currentTimeMillis()
        if (now - lastDiskWrite < PERSIST_THROTTLE_MS) return // 节流窗口内:稍后的块会再触发
        lastDiskWrite = now
        persistExecutor.execute { writeDisk(p) }
    }

    /** 滚动上限:已结束且已读的任务每类只留最近 [MAX_FINISHED_PER_KIND] 条(索引无限增长会拖慢每次全量重写)。RUNNING 与未读不裁。 */
    private fun applyCap() {
        val all = _tasks.value
        val finishedReadCount = all.count { it.status != Status.RUNNING && it.read }
        if (finishedReadCount <= MAX_FINISHED_PER_KIND * 2) return
        val kept = mutableListOf<Task>()
        for (kind in listOf(Kind.MESSAGE, Kind.DOWNLOAD)) {
            val list = all.filter { it.kind == kind }
            kept += list.filter { it.status == Status.RUNNING || !it.read }
            kept += list.filter { it.status != Status.RUNNING && it.read }
                .sortedByDescending { it.startedAt }
                .take(MAX_FINISHED_PER_KIND)
        }
        if (kept.size < all.size) _tasks.value = kept.toList()
    }

    private fun writeDisk(p: SharedPreferences) {
        val text = _tasks.value.joinToString("\n") { t ->
            listOf(
                t.id, t.threadId, t.kind.name, t.title, t.status.name,
                if (t.read) "1" else "0", t.startedAt.toString(),
                t.progress.toString(), t.detail, t.rounds.toString(),
                // 【修复】localUri/sourceTitle/sizeText 之前没落盘:杀进程重启后
                // 所有下载项都丢了"文件在哪" → 用户数据管理里查看/分享图标集体消失。
                t.localUri, t.sourceTitle, t.sizeText,
            ).joinToString("\u0001") { it.replace("\u0001", " ").replace("\n", " ") }
        }
        p.edit().putString("tasks", text).putInt("unread", _unreadCount.value).apply()
    }

    /**
     * 【E2 残留修复】恢复落盘的任务清单:磁盘读 + [parseTasks] 全量行解析挪到落盘
     * 单线程([persistExecutor]),不再占 MainActivity.onCreate 的主线程 —— 这是启动
     * 路径,此前 getString 与逐行解析都同步跑在首帧之前。
     *
     * 【异步播种与竞态】结果经 StateFlow([_tasks]/[_unreadCount])发布;消费方
     * (抽屉角标/最近会话抽屉/任务中心页/设置页)全部 collectAsState 订阅,
     * 异步播种无感。唯一同步读点是紧随 init 的 PaNotifications.init 种子:
     * 它可能拿到播种前的空清单,但冷启动恢复出的消息任务已被 parseTasks 判为
     * INTERRUPTED(非运行态),而通知监控范围每拍都并上
     * TurnSession.runningThreadIds()+已监控集合,该种子无行为差异。
     * 【竞态兜底】播种落地的极小窗口内若已有新登记(如自动重连即刻开轮),
     * 以内存中的更新真相为准:同 id 去重保留内存版本,磁盘行只补缺 ——
     * 该分支在旧同步实现里不可达;正常时序走 current 为空的快路径,结果与
     * 旧实现逐字段一致。
     */
    private fun restore() {
        val p = prefs ?: return
        persistExecutor.execute {
            runCatching {
                val raw = p.getString("tasks", "") ?: ""
                val restored = parseTasks(raw)
                val current = _tasks.value
                val merged = if (current.isEmpty()) restored
                else restored.filter { r -> current.none { it.id == r.id } } + current
                _tasks.value = merged
                // 未读数以持久化值为准(任务清单可能被裁剪)
                _unreadCount.value = maxOf(p.getInt("unread", 0), merged.count { !it.read })
            }
        }
    }

    /**
     * 持久化文本 → 任务清单(纯函数,单测直接喂旧格式)。
     * 【向后兼容】10 字段是 localUri 落盘前的旧格式;11-13 字段(来源会话/大小)
     * 逐个可缺 —— `getOrNull + orEmpty`,老行不丢,新字段缺席也不炸。
     */
    internal fun parseTasks(raw: String): List<Task> =
        raw.split("\n").mapNotNull { line ->
            if (line.isBlank()) return@mapNotNull null
            val f = line.split("\u0001")
            if (f.size < 10) return@mapNotNull null
            runCatching {
                val status = Status.valueOf(f[4])
                Task(
                    id = f[0],
                    threadId = f[1],
                    kind = Kind.valueOf(f[2]),
                    title = f[3],
                    // 上次进程被杀时还在跑的任务,重启后不可能还在跑 -> 标记为已中断
                    status = if (status == Status.RUNNING) Status.INTERRUPTED else status,
                    read = f[5] == "1",
                    startedAt = f[6].toLongOrNull() ?: System.currentTimeMillis(),
                    progress = f[7].toFloatOrNull() ?: 0f,
                    detail = f[8],
                    rounds = f[9].toIntOrNull() ?: 1,
                    localUri = f.getOrNull(10).orEmpty(),
                    sourceTitle = f.getOrNull(11).orEmpty(),
                    sizeText = f.getOrNull(12).orEmpty(),
                )
            }.getOrNull()
        }
}
