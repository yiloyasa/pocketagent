package dev.pocketagent.app.ui.component

import android.content.Context
import android.content.SharedPreferences
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
        var title: String,
        var status: Status,
        var progress: Float = 0f,
        var detail: String = "",
        var read: Boolean = false,
        var startedAt: Long = System.currentTimeMillis(),
        var rounds: Int = 1,
    )

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
        if (_tasks.value.none { it.id == id }) {
            _tasks.value = _tasks.value + Task(
                id = id,
                threadId = threadId,
                kind = Kind.DOWNLOAD,
                title = title,
                status = Status.RUNNING,
            )
            persist()
        }
        return id
    }

    fun updateDownload(id: String, progress: Float, detail: String = "") {
        val t = _tasks.value.firstOrNull { it.id == id } ?: return
        t.progress = progress.coerceIn(0f, 1f)
        if (detail.isNotEmpty()) t.detail = detail
        persist()
    }

    fun finishDownload(id: String, ok: Boolean, detail: String = "") {
        val t = _tasks.value.firstOrNull { it.id == id } ?: return
        t.status = if (ok) Status.DONE else Status.FAILED
        t.progress = if (ok) 1f else t.progress
        t.detail = detail
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

    fun clearFinished() {
        _tasks.value = _tasks.value.filter { it.status == Status.RUNNING }
        persist()
    }

    // ---------------------------------------------------------------- 内部

    private fun persist() {
        // 【必须】StateFlow 只按引用比对,原地修改 List 里的 Task 不会通知订阅者。
        // 这里统一重新发射一次,保证任务中心列表与角标能收到变更。
        _tasks.value = _tasks.value.toList()

        // 【角标刷新】未读数总是从任务清单重算 —— 这样"开始一轮/结束一轮"
        // 也会把侧边栏角标点起来(此前 refreshUnread 只在已读操作里调,角标恒 0)。
        if (viewing) _tasks.value.forEach { it.read = true }
        _unreadCount.value = _tasks.value.count { !it.read }

        val p = prefs ?: return
        val text = _tasks.value.joinToString("\n") { t ->
            listOf(
                t.id, t.threadId, t.kind.name, t.title, t.status.name,
                if (t.read) "1" else "0", t.startedAt.toString(),
                t.progress.toString(), t.detail, t.rounds.toString(),
            ).joinToString("\u0001") { it.replace("\u0001", " ").replace("\n", " ") }
        }
        p.edit().putString("tasks", text).putInt("unread", _unreadCount.value).apply()
    }

    private fun restore() {
        val p = prefs ?: return
        val raw = p.getString("tasks", "") ?: ""
        val list = raw.split("\n").mapNotNull { line ->
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
                )
            }.getOrNull()
        }
        _tasks.value = list
        // 未读数以持久化值为准(任务清单可能被裁剪)
        _unreadCount.value = maxOf(p.getInt("unread", 0), list.count { !it.read })
    }
}
