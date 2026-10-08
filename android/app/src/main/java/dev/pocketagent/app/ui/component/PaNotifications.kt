package dev.pocketagent.app.ui.component

import android.Manifest
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.os.Build
import android.util.Log
import androidx.core.app.ActivityCompat
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import dev.pocketagent.app.MainActivity
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.TurnSession
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import org.json.JSONObject

/**
 * 通知触发判定层(纯 Kotlin,零 Android 依赖)。
 *
 * 【为什么单独一层】通知的"什么时候发/什么时候撤"是纯状态机逻辑,而 Android 的
 * NotificationManager 在 JVM 单测里是不可实例化的桩 —— 把判定收敛到这里,
 * 触发矩阵就能用单测钉死(见 PaNotificationAndVoiceTest)。
 *
 * 【监控方式】监控方每拍并集"登记线程(会话页进入时登记,轮次只会从会话页发起)
 * + TurnSession.runningThreadIds() 快照 + 历史拍过的线程",定期(10s)采样各线程
 * 的公开状态与上一拍对比,得出增量的通知动作。
 */
object PaNotificationPolicy {

    /** 单个线程的一拍监控快照(全部来自 TurnSession.State 公开只读字段)。 */
    data class ThreadWatch(
        val running: Boolean,
        val approvalPresent: Boolean,
        val recovering: Boolean,
    )

    /** 一拍采样后应执行的通知动作。 */
    sealed interface Decision {
        /** 发「任务完成」通知。 */
        data class TurnDone(val threadId: String) : Decision
        /** 发高优「需要你的批准」通知(带允许/拒绝按钮)。 */
        data class ApprovalShown(val threadId: String) : Decision
        /** 撤「需要你的批准」通知。 */
        data class ApprovalGone(val threadId: String) : Decision
    }

    /**
     * app 是否在前台。真身在此(纯层),MainActivity onResume/onPause 写入;
     * 放这里而不是 Android 侧对象里,是为了单测能直接驱动"前台/后台矩阵"。
     */
    @Volatile
    var appInForeground: Boolean = false

    /**
     * 触发矩阵:
     * | 上一拍 running | 本拍 running | recovering | 前台 | 动作 |
     * |---|---|---|---|---|
     * | true  | false | false | 后台 | TurnDone(发完成通知) |
     * | true  | false | false | 前台 | 不发(用户正看着,不打扰) |
     * | true  | false | true  | 任意 | 不发(断流恢复中,结果未知,发了就是假消息) |
     * | false / 首见 | 任意 | 任意 | 任意 | 不发(开始跑不算完成,没有 running=false 的"下降沿") |
     * | 审批 null→非 null | 任意 | 任意 | 任意 | ApprovalShown(审批是阻塞点,前台也值得提示) |
     * | 审批 非 null→null | 任意 | 任意 | 任意 | ApprovalGone(撤通知,已解决) |
     *
     * prev == null 表示该线程是监控首见:按"全部 false"的基线处理 ——
     * 首见即有审批也会补发通知(覆盖监控启动晚于审批到达的情况)。
     */
    fun decide(prev: ThreadWatch?, now: ThreadWatch, threadId: String): List<Decision> {
        val out = ArrayList<Decision>(2)
        if (prev != null && prev.running && !now.running && !now.recovering && !appInForeground) {
            out.add(Decision.TurnDone(threadId))
        }
        if (!(prev?.approvalPresent ?: false) && now.approvalPresent) {
            out.add(Decision.ApprovalShown(threadId))
        }
        if (prev?.approvalPresent == true && !now.approvalPresent) {
            out.add(Decision.ApprovalGone(threadId))
        }
        return out
    }

    /**
     * 通知点击深链的 extra 解析:非空才值得导航。
     * (MainActivity onCreate/onNewIntent → PaNotifications.onLaunchIntent 走这里。)
     */
    fun pendingNavigationFrom(extra: String?): String? =
        extra?.trim()?.takeIf { it.isNotEmpty() }

    /**
     * 通知 id:threadId 哈希稳定映射 —— 同一会话的通知 id 恒定,重复通知互相覆盖
     * 而不是刷屏。两通道各占 [10万,50万) 与 [50万,90万) 区间,互不覆盖;
     * 且避开 TurnKeepAliveService 前台服务写死的 id=1。
     */
    fun turnDoneNotificationId(threadId: String): Int =
        100_000 + Math.floorMod(threadId.hashCode(), 400_000)

    fun approvalNotificationId(threadId: String): Int =
        500_000 + Math.floorMod(threadId.hashCode(), 400_000)

    /**
     * 通知正文摘要:压成一行、截 60 字(超出留省略号)。
     * 完成通知的"内容=最后助手文本截 60 字"就是它。
     */
    fun summarizeForNotification(text: String, max: Int = 60): String {
        val oneLine = text.replace(Regex("\\s+"), " ").trim()
        if (oneLine.isEmpty()) return ""
        return if (oneLine.length <= max) oneLine else oneLine.take(max - 1) + "…"
    }

    /** 完成通知正文:取最后一条助手文本做摘要;没有就给人话兜底。 */
    fun turnDoneSummary(lastAssistantText: String): String =
        summarizeForNotification(lastAssistantText).ifBlank { "任务已完成,点按查看结果" }

    /**
     * 审批通知的一行摘要(截 [max] 字):按动作类型取"人最关心的那一行" ——
     * shell 取命令、反问取问题、改文件取首个文件名;解析失败退回原始 JSON。
     * 与聊天页 ApprovalSheet 的分派口径一致,但只取一行做通知预览。
     */
    fun approvalHeadline(actionName: String, jsonArgs: String, max: Int = 80): String {
        val args = runCatching { JSONObject(jsonArgs) }.getOrNull()
        val raw: String = if (args == null) {
            // JSON 解析失败:原样退回(与聊天页审批卡的口径一致,不能让卡片变空白)
            jsonArgs
        } else {
            when (actionName) {
                "shell" -> args.optString("command")
                "request_user_input", "mcp_elicitation" -> args.optString("message")
                "file_change", "apply_patch" -> {
                    val files = args.optJSONArray("files")
                    val count = args.optInt("count", 0)
                    when {
                        files != null && files.length() > 0 ->
                            files.optString(0) + if (files.length() > 1) " 等 ${files.length()} 个文件" else ""
                        count > 0 -> "共 $count 个文件改动"
                        else -> ""
                    }
                }
                else -> jsonArgs
            }
        }
        // AUDIT §3.3:参数解析不出人话时给中性表述,禁显「未知的操作」
        return summarizeForNotification(raw.ifBlank { actionName.ifBlank { "新的操作请求" } }, max)
    }

    /** 审批通知折叠后的详情(BigTextStyle):完整 JSON 截到 [max] 字,够看清在批准什么。 */
    fun approvalDetail(actionName: String, jsonArgs: String, max: Int = 600): String {
        if (jsonArgs.isBlank()) return ""
        return summarizeForNotification(jsonArgs, max)
    }
}

/**
 * P1-6 本地通知(零云路线,全程离线,无 FCM):
 * 1. 「任务完成」:轮次 running→false 且 app 在后台 → 普通通知,点击深链对应会话;
 * 2. 「需要你的批准」:pendingApproval 出现 → HIGH 通知,带 允许/拒绝 两个动作按钮;
 * 3. 审批解决(pendingApproval 清空)→ 自动撤通知。
 *
 * 【动态注册接收器】通知动作按钮走 PendingIntent.getBroadcast → app 进程内动态注册的
 * BroadcastReceiver(ContextCompat.RECEIVER_NOT_EXPORTED,Android14+ 必须显式声明不导出),
 * 收到后调 TurnSession.answerApproval 公开 API(与聊天页审批抽屉同一份状态,
 * 成功后聊天页的抽屉自动收起)。不改 AndroidManifest。
 *
 * 【权限】Android13+ 的 POST_NOTIFICATIONS 运行时权限由 MainActivity 首次恢复时申请;
 * 未授权时发通知静默跳过(不崩、不重试轰炸)。
 *
     * 【只读 TurnSession】本对象对 TurnSession 只调公开 API(peek/answerApproval),
     * 不写它的任何字段 —— TurnSession 由并行执行体独占。轮询用 peek 不用 state:
     * 不创建、不刷访问时刻,"通知轮询不算访问"(C3)由此兑现。
     */
object PaNotifications {

    // ---- 渠道与常量 ----
    const val CHANNEL_APPROVAL = "pa_approval"
    const val CHANNEL_TURN_DONE = "pa_turn_done"
    const val EXTRA_THREAD_ID = "pa_thread_id"
    private const val EXTRA_REQUEST_ID = "pa_request_id"
    private const val ACTION_APPROVE = "dev.pocketagent.app.notification.APPROVE"
    private const val ACTION_DENY = "dev.pocketagent.app.notification.DENY"
    private const val NOTIFICATION_PERMISSION_REQUEST_CODE = 7001
    /**
     * 采样间隔(P1-6 事件化改造):监控范围改为 [TurnSession.runningThreadIds] 快照驱动
     * (TurnSession 侧无变更信号可订阅,见其注释),轮询从逐秒 1s 放宽到 10s;
     * 快照只是遍历一张小 ConcurrentHashMap,开销可忽略。
     */
    private const val POLL_INTERVAL_MS = 10_000L

    private val appScope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    /** 已知线程集合:监控范围 = 用户打开过的会话(轮次只会从会话页发起)。 */
    private val knownThreads: MutableSet<String> = java.util.concurrent.ConcurrentHashMap.newKeySet()

    /** 上一拍快照:decide 的 prev 入参来源。 */
    private val watches = java.util.concurrent.ConcurrentHashMap<String, PaNotificationPolicy.ThreadWatch>()

    /** 会话名缓存:完成通知标题用;拿不到时回退「任务完成」。 */
    private val titles = java.util.concurrent.ConcurrentHashMap<String, String>()

    private val _pendingChat = MutableStateFlow<String?>(null)

    /** 通知点进来的待导航会话(RootNav 消费后调 [clearPendingChatNavigation])。 */
    val pendingChatNavigation: StateFlow<String?> = _pendingChat.asStateFlow()

    @Volatile private var appContext: Context? = null
    @Volatile private var initialized = false
    @Volatile private var monitorStarted = false
    @Volatile private var permissionAsked = false

    /** 前台标志:转发到纯层的真身(MainActivity onResume/onPause 写入)。 */
    var appInForeground: Boolean
        get() = PaNotificationPolicy.appInForeground
        set(value) { PaNotificationPolicy.appInForeground = value }

    /**
     * 初始化:MainActivity onCreate 挂进 init 链。建渠道、注册审批动作接收器、
     * 启动逐秒监控。重复调用幂等。
     */
    fun init(context: Context) {
        if (initialized) return
        initialized = true
        val app = context.applicationContext
        appContext = app
        ensureChannels(app)
        registerApprovalReceiver(app)
        // 种子:任务中心里登记过的消息任务(进程存活但监控晚于轮次启动的兜底)
        runCatching {
            TaskCenter.messages().forEach { if (it.threadId.isNotBlank()) knownThreads.add(it.threadId) }
        }
        startMonitor()
    }

    // ---------------------------------------------------------- 给 UI/Activity 的入口

    /**
     * MainActivity onCreate / onNewIntent 统一入口:提取通知深链的 threadId。
     * RootNav 组合后消费 [pendingChatNavigation] 完成导航。
     */
    fun onLaunchIntent(intent: Intent?) {
        val tid = PaNotificationPolicy.pendingNavigationFrom(intent?.getStringExtra(EXTRA_THREAD_ID))
        if (tid != null) _pendingChat.value = tid
    }

    /** RootNav 消费完深链后清空,避免再次重组重复导航。 */
    fun clearPendingChatNavigation() {
        _pendingChat.value = null
    }

    /** 会话页进入时登记监控范围(ChatScreen LaunchedEffect 调用)。 */
    fun observeThread(threadId: String) {
        if (threadId.isNotBlank()) knownThreads.add(threadId)
    }

    /** 会话名缓存(ChatScreen 拉到会话标题时调用):完成通知的标题用。 */
    fun rememberTitle(threadId: String, title: String) {
        if (threadId.isNotBlank() && title.isNotBlank()) titles[threadId] = title
    }

    /**
     * Android13+ 通知运行时权限:每个进程生命周期只问一次;用户拒绝后
     * areNotificationsEnabled() 为 false,发通知走静默跳过,绝不重试轰炸。
     */
    fun maybeRequestNotificationPermission(activity: android.app.Activity) {
        if (Build.VERSION.SDK_INT < 33) return
        if (permissionAsked) return
        permissionAsked = true
        if (ContextCompat.checkSelfPermission(activity, Manifest.permission.POST_NOTIFICATIONS) ==
            PackageManager.PERMISSION_GRANTED
        ) return
        ActivityCompat.requestPermissions(
            activity,
            arrayOf(Manifest.permission.POST_NOTIFICATIONS),
            NOTIFICATION_PERMISSION_REQUEST_CODE,
        )
    }

    // ---------------------------------------------------------- 监控循环

    private fun startMonitor() {
        if (monitorStarted) return
        monitorStarted = true
        appScope.launch {
            while (true) {
                delay(POLL_INTERVAL_MS)
                // 单拍失败(个别状态读取异常)不终止监控,下一拍重试
                runCatching { tickOnce() }
            }
        }
    }

    /**
     * 一拍采样(P1-6 事件化改造):监控范围 = 登记种子(ChatScreen 进入会话 + 任务中心
     * 兜底,保留)∪ 本拍 [TurnSession.runningThreadIds] 快照 ∪ 历史拍过的线程
     * (watches 的 key —— decide 的下降沿比较依赖上一拍快照,已见过的线程不能掉出
     * 范围,否则"结束的下降沿"会被误判成"首见"而漏发完成通知)。
     * → 读全部范围内线程的公开状态 → 与上一拍对比 → 应用通知动作。
     * internal 供未来扩展,当前只被监控协程调用。
     */
    fun tickOnce() {
        val ctx = appContext ?: return
        val scopeIds = HashSet<String>(knownThreads)
        scopeIds.addAll(TurnSession.runningThreadIds())
        scopeIds.addAll(watches.keys)
        for (tid in scopeIds) {
            // 【C3/P1-4】只读访问:peek 不创建、不刷新访问时刻。旧实现走 TurnSession.state()
            // (getOrPut + 刷 lastAccessAt),把"通知轮询"变成了无限保活 —— 空闲回收
            // 永不触发、已回收的会话被凭空复活、states 无界增长。无会话 = 该线程当前
            // 没有可监控的状态,跳过本拍,并把上一拍快照撤掉(重见时按"首见"基线处理,
            // 不让陈旧的 prev 快照在会话重建后误判出下降沿)。
            val st = TurnSession.peek(tid)
            if (st == null) {
                watches.remove(tid)
                continue
            }
            val now = PaNotificationPolicy.ThreadWatch(
                running = st.running,
                approvalPresent = st.pendingApproval != null,
                recovering = st.recovering,
            )
            val prev = watches.put(tid, now)
            for (decision in PaNotificationPolicy.decide(prev, now, tid)) {
                runCatching { applyDecision(ctx, decision, st) }
            }
        }
    }

    private fun applyDecision(
        ctx: Context,
        decision: PaNotificationPolicy.Decision,
        st: TurnSession.State,
    ) {
        when (decision) {
            is PaNotificationPolicy.Decision.TurnDone -> postTurnDone(ctx, decision.threadId, st)
            is PaNotificationPolicy.Decision.ApprovalShown -> postApproval(ctx, decision.threadId, st)
            is PaNotificationPolicy.Decision.ApprovalGone -> cancelApproval(ctx, decision.threadId)
        }
    }

    // ---------------------------------------------------------- 通知构建

    /** 完成通知:标题=会话名(拿不到回退"任务完成"),内容=最后助手文本截 60 字,点击深链。 */
    private fun postTurnDone(ctx: Context, threadId: String, st: TurnSession.State) {
        if (!canNotify(ctx)) return
        ensureChannels(ctx)
        val title = titles[threadId].orEmpty().ifBlank { "任务完成" }
        val lastAssistant = st.items.lastOrNull { it.role == "assistant" && it.text.isNotBlank() }?.text.orEmpty()
        val text = PaNotificationPolicy.turnDoneSummary(lastAssistant)
        val id = PaNotificationPolicy.turnDoneNotificationId(threadId)
        val notification = NotificationCompat.Builder(ctx, CHANNEL_TURN_DONE)
            .setSmallIcon(R.drawable.ic_lucide_monitor)
            .setContentTitle(title)
            .setContentText(text)
            .setStyle(NotificationCompat.BigTextStyle().bigText(text))
            .setContentIntent(contentIntent(ctx, threadId, id))
            .setAutoCancel(true)
            .setCategory(NotificationCompat.CATEGORY_STATUS)
            .setWhen(System.currentTimeMillis())
            .build()
        safeNotify(ctx, id, notification)
    }

    /**
     * 审批通知:HIGH 渠道,一行摘要 + 折叠详情,动作按钮 允许/拒绝。
     * 点击正文 = 深链进会话(抽屉里看完整上下文再决定)。
     */
    private fun postApproval(ctx: Context, threadId: String, st: TurnSession.State) {
        if (!canNotify(ctx)) return
        ensureChannels(ctx)
        val approval = st.pendingApproval ?: return
        val id = PaNotificationPolicy.approvalNotificationId(threadId)
        val headline = PaNotificationPolicy.approvalHeadline(approval.action.name, approval.action.jsonArgs)
        val detail = PaNotificationPolicy.approvalDetail(approval.action.name, approval.action.jsonArgs)
        // 折叠详情 = 摘要 + 完整参数(截 600 字);详情与摘要一样就不再叠一遍
        val bigText = if (detail.isBlank() || detail == headline) headline else "$headline\n\n$detail"
        val notification = NotificationCompat.Builder(ctx, CHANNEL_APPROVAL)
            .setSmallIcon(R.drawable.ic_lucide_triangle_alert)
            .setContentTitle("需要你的批准")
            .setContentText(headline)
            .setStyle(NotificationCompat.BigTextStyle().bigText(bigText))
            // 允许 / 拒绝:广播给动态接收器;requestCode 用 id 偏移避免与其它 PendingIntent 撞键。
            // NotificationCompat 的 addAction 只收 Action 对象(icon=0 表示不带图标)。
            .addAction(NotificationCompat.Action(0, "允许", actionIntent(ctx, threadId, approval.requestId, ACTION_APPROVE, id + 1)))
            .addAction(NotificationCompat.Action(0, "拒绝", actionIntent(ctx, threadId, approval.requestId, ACTION_DENY, id + 2)))
            .setContentIntent(contentIntent(ctx, threadId, id + 3))
            .setAutoCancel(true)
            .setCategory(NotificationCompat.CATEGORY_EVENT)
            // <Android8 设备没有渠道,优先级兜底
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setWhen(System.currentTimeMillis())
            .build()
        safeNotify(ctx, id, notification)
    }

    /** 撤审批通知(审批被处理/解决时)。 */
    fun cancelApproval(context: Context, threadId: String) {
        runCatching {
            NotificationManagerCompat.from(context)
                .cancel(PaNotificationPolicy.approvalNotificationId(threadId))
        }
    }

    // ---------------------------------------------------------- Intent 与接收器

    /** 点击通知正文:深链 MainActivity,extra 带 thread_id(FLAG 组合保证复用既有实例)。 */
    private fun contentIntent(ctx: Context, threadId: String, requestCode: Int): PendingIntent =
        PendingIntent.getActivity(
            ctx,
            requestCode,
            Intent(ctx, MainActivity::class.java)
                .putExtra(EXTRA_THREAD_ID, threadId)
                .addFlags(
                    Intent.FLAG_ACTIVITY_NEW_TASK or
                        Intent.FLAG_ACTIVITY_CLEAR_TOP or
                        Intent.FLAG_ACTIVITY_SINGLE_TOP,
                ),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )

    /** 动作按钮:广播给动态注册的接收器(NOT_EXPORTED,同 app 的 PendingIntent 可达)。 */
    private fun actionIntent(
        ctx: Context,
        threadId: String,
        requestId: String,
        action: String,
        requestCode: Int,
    ): PendingIntent = PendingIntent.getBroadcast(
        ctx,
        requestCode,
        Intent(action)
            .setPackage(ctx.packageName)
            .putExtra(EXTRA_THREAD_ID, threadId)
            .putExtra(EXTRA_REQUEST_ID, requestId),
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
    )

    private fun registerApprovalReceiver(app: Context) {
        val filter = IntentFilter().apply {
            addAction(ACTION_APPROVE)
            addAction(ACTION_DENY)
        }
        // RECEIVER_NOT_EXPORTED:只有本 app(同 uid,含本 app 发出的 PendingIntent)能送到
        ContextCompat.registerReceiver(app, approvalReceiver, filter, ContextCompat.RECEIVER_NOT_EXPORTED)
    }

    /**
     * 审批动作接收器:允许/拒绝 → TurnSession.answerApproval 公开 API。
     * 与聊天页同一份审批状态:发送成功会清 pendingApproval,聊天页抽屉自动收起;
     * 发送失败(流断开)时 TurnSession 会置 approvalError,聊天页能看到原因 ——
     * 通知这边无论成败都撤掉,残留一条点不动的审批通知只会误导。
     */
    private val approvalReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            val threadId = intent.getStringExtra(EXTRA_THREAD_ID).orEmpty()
            val requestId = intent.getStringExtra(EXTRA_REQUEST_ID).orEmpty()
            if (threadId.isBlank() || requestId.isBlank()) return
            runCatching {
                TurnSession.answerApproval(
                    threadId,
                    requestId,
                    approved = intent.action == ACTION_APPROVE,
                )
            }
            cancelApproval(context, threadId)
        }
    }

    // ---------------------------------------------------------- 底层工具

    private fun ensureChannels(ctx: Context) {
        if (Build.VERSION.SDK_INT < 26) return
        val nm = ctx.getSystemService(NotificationManager::class.java) ?: return
        runCatching {
            nm.createNotificationChannel(
                NotificationChannel(CHANNEL_APPROVAL, "审批请求", NotificationManager.IMPORTANCE_HIGH),
            )
            nm.createNotificationChannel(
                NotificationChannel(CHANNEL_TURN_DONE, "任务完成", NotificationManager.IMPORTANCE_DEFAULT),
            )
        }
    }

    /** 未授权(Android13+ 拒绝过)→ 静默跳过,不崩、不重试。 */
    private fun canNotify(ctx: Context): Boolean = runCatching {
        NotificationManagerCompat.from(ctx).areNotificationsEnabled()
    }.getOrDefault(false)

    private fun safeNotify(ctx: Context, id: Int, notification: android.app.Notification) {
        runCatching {
            NotificationManagerCompat.from(ctx).notify(id, notification)
        }.onFailure { Log.w("PaNotifications", "发送通知失败", it) }
    }
}
