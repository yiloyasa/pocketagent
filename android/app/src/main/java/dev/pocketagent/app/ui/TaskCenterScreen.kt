package dev.pocketagent.app.ui

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.EmptyState
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.edgeSwipeToOpenDrawer
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.component.DocFormatRegistry
import dev.pocketagent.app.ui.component.PaDownloader
import dev.pocketagent.app.ui.component.PaNavIconButton
import dev.pocketagent.app.ui.component.PaScreenScaffold
import dev.pocketagent.app.ui.component.PaStatusBadge
import dev.pocketagent.app.ui.component.RedownloadPlan
import dev.pocketagent.app.ui.component.RetryGate
import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.component.fileKindOf
import dev.pocketagent.app.ui.component.redownloadNeedsPc
import dev.pocketagent.app.ui.component.redownloadPlanOf
import dev.pocketagent.app.ui.theme.EditorialTone
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.editorial
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import java.text.SimpleDateFormat
import java.util.Date

// ---------------------------------------------------------------- 纯函数(单测钉死)

/**
 * 任务行的四词状态(纯函数):状态一律经 [editorialStatusOf](TaskCenter.Status 重载)
 * 映射,全页不写第二个口径。
 *
 * 【离线分支】daemon 断连时,消息类任务的「正在执行」无从证实(README:未连接时
 * 状态不能写"正在执行")→ 返回 null,行内不摆状态徽章,改给离线提示(见
 * [taskOfflineHint])。下载类是**手机本机**行为(与 daemon 连接无关),照常映射。
 */
internal fun taskStatusView(task: TaskCenter.Task, hostOffline: Boolean): EditorialStatusView? =
    if (task.kind == TaskCenter.Kind.MESSAGE && hostOffline && task.status == TaskCenter.Status.RUNNING) {
        null
    } else {
        EditorialStatusView(editorialStatusOf(task.status))
    }

/**
 * 离线分支的行内提示(纯函数):只在 [taskStatusView] 返回 null 的同一条件下给词,
 * 两处共用一套判定,不会出现"徽章没了提示也没了"的空行。
 */
internal fun taskOfflineHint(task: TaskCenter.Task, hostOffline: Boolean): String? =
    if (task.kind == TaskCenter.Kind.MESSAGE && hostOffline && task.status == TaskCenter.Status.RUNNING) {
        "离线 · 恢复连接后继续"
    } else {
        null
    }

/**
 * 任务中心三分区的计数(纯函数):待你处理(FAILED/INTERRUPTED)/ 进行中(RUNNING)/
 * 已完成(DONE)。设计稿 08 顶部三张卡就是这个分区;CANCELED 归入"已结束"不算进
 * 三卡(与旧概览条口径一致:failed 只数 FAILED+INTERRUPTED)。
 */
internal data class TaskCenterStats(val waitingForYou: Int, val running: Int, val completed: Int)

internal fun taskCenterStats(tasks: List<TaskCenter.Task>): TaskCenterStats = TaskCenterStats(
    waitingForYou = tasks.count {
        it.status == TaskCenter.Status.FAILED || it.status == TaskCenter.Status.INTERRUPTED
    },
    running = tasks.count { it.status == TaskCenter.Status.RUNNING },
    completed = tasks.count { it.status == TaskCenter.Status.DONE },
)

/**
 * 任务分区的一行:分区头文案 + 该组任务。四词分区的头 = 状态词本身(与行内徽章
 * 长出同一个词);断线时消息类运行中任务落到 [offlineResume] 分区 —— 头**不写
 * 「正在执行」**(README:未连接时绝不显示),改写「离线待恢复」,与行内
 * 隐去徽章 + 离线提示是同一套事实,不让分区头把徽章刚收起的词又摆回来。
 */
internal data class TaskPartitionRow(
    val word: String,
    val offlineResume: Boolean,
    val tasks: List<TaskCenter.Task>,
)

/**
 * 当前页签的分区行(纯函数):按四词状态分成
 * 等待你处理 / 离线待恢复(仅断线时) / 正在执行 / 已完成 / 未完成 五组
 * (空组不出现),组序即优先级(README:待你处理优先于运行中)。
 * 下载类的「正在执行」是手机本机行为,断连也照常归执行分区。
 */
internal fun taskPartitionRows(
    tasks: List<TaskCenter.Task>,
    hostOffline: Boolean,
): List<TaskPartitionRow> {
    val offlineMessageRunning = if (hostOffline) {
        tasks.filter { it.kind == TaskCenter.Kind.MESSAGE && it.status == TaskCenter.Status.RUNNING }
    } else {
        emptyList()
    }
    return listOf(
        TaskPartitionRow(
            EditorialStatus.WAITING_FOR_YOU.word,
            false,
            tasks.filter { editorialStatusOf(it.status) == EditorialStatus.WAITING_FOR_YOU },
        ),
        TaskPartitionRow("离线待恢复", true, offlineMessageRunning),
        TaskPartitionRow(
            EditorialStatus.RUNNING.word,
            false,
            tasks.filter {
                it.status == TaskCenter.Status.RUNNING &&
                    !(hostOffline && it.kind == TaskCenter.Kind.MESSAGE)
            },
        ),
        TaskPartitionRow(
            EditorialStatus.COMPLETED.word,
            false,
            tasks.filter { it.status == TaskCenter.Status.DONE },
        ),
        TaskPartitionRow(
            EditorialStatus.NOT_COMPLETED.word,
            false,
            tasks.filter {
                it.status != TaskCenter.Status.RUNNING && it.status != TaskCenter.Status.DONE
            },
        ),
    ).filter { it.tasks.isNotEmpty() }
}

/**
 * 当前页签的分区(纯函数,在线口径):四词状态分组(空组不出现)。
 * 渲染走 [taskPartitionRows](含断线分区);本函数保留四词口径本身,
 * 供既有单测钉住"分区头与行内徽章长出同一个词"。
 */
internal fun taskPartitions(tasks: List<TaskCenter.Task>): List<Pair<EditorialStatus, List<TaskCenter.Task>>> =
    taskPartitionRows(tasks, hostOffline = false).map { row ->
        // hostOffline=false 时离线分区恒为空,首任务的四词即分区口径
        editorialStatusOf(row.tasks.first().status) to row.tasks
    }

/**
 * 任务中心(设计稿 08 · 编辑式)。
 *
 * 【无弹窗】未读只以角标告知,点进来直接看列表 —— 符合"不使用弹窗"的红线。
 * 【单一真相】这里只渲染 [TaskCenter.tasks] 的快照;轮次进度仍由 TurnSession 持有,
 * 所以不存在"任务中心说完成了、聊天页还在转"的双份状态。
 * 【分区】顶部三张概览卡(待你处理/进行中/已完成)+ 列表按四词分区(空组不出现);
 * 徽章全走 [editorialStatusOf] + [PaStatusBadge],消息/下载双页签与左右滑动保留。
 */
@Composable
fun TaskCenterScreen(
    currentTab: Int = 4,
    onOpenThread: (String) -> Unit,
    onOpenTerminal: () -> Unit,
    onOpenSettings: () -> Unit,
    onGoTab: (Int) -> Unit,
    onOpenBrowser: (String) -> Unit = {},
    /** 下载类任务点卡片本体 → 跳「用户数据管理」(文件的具体管理都在那)。 */
    onOpenUserData: () -> Unit = {},
    /** 返回层级:二级页面按返回 → 回主页并展开侧边栏。 */
    onBackToHome: () -> Unit = {},
    modifier: Modifier = Modifier,
) {
    val ed = MaterialTheme.editorial
    var kindTab by remember { mutableStateOf(TaskCenter.Kind.MESSAGE) }
    // 侧边栏跟手:与主页/会话页同一套 controller
    val drawerCtrl = dev.pocketagent.app.ui.component.rememberDrawerController()
    // 【返回层级 · 分级回退】抽屉 → 页签(文件下载→消息处理)→ 主板块。
    BackHandler(enabled = true) {
        when {
            drawerCtrl.visible -> drawerCtrl.close()
            kindTab != TaskCenter.Kind.MESSAGE -> kindTab = TaskCenter.Kind.MESSAGE
            else -> onBackToHome()
        }
    }

    val all by TaskCenter.tasks.collectAsState()
    val ctx = androidx.compose.ui.platform.LocalContext.current
    // 主机连接态:只用于离线分支(消息类 RUNNING 不得显示「正在执行」,见 taskStatusView)
    val connState = AgentConnection.connState.collectAsState().value
    val hostOffline = connState == "down"
    // 【重试防重入】每行一扇门闩:冷却窗口内的第二次点击一律拒收(消息类双击会
    // 连发两次结果找回;下载类双击会并发两条同名下载流);"重试中"视觉态到点自动退场。
    val retryGates = remember { java.util.concurrent.ConcurrentHashMap<String, RetryGate>() }
    var retryingIds by remember { mutableStateOf(setOf<String>()) }
    // 【文件丢失态】「打开文件」先验存在性:失效的行内转"文件丢失",给出
    // 重新下载/移除的下一步,而不是弹一个 toast 就没了下文。
    var missingFileIds by remember { mutableStateOf(setOf<String>()) }
    val scope = androidx.compose.runtime.rememberCoroutineScope()

    /** 打开已下载的文件:先验存在(手机侧文件可能已被清理),失效转"文件丢失"态。 */
    fun requestOpenFile(task: TaskCenter.Task) {
        if (task.localUri.isEmpty()) {
            missingFileIds = missingFileIds + task.id
            return
        }
        scope.launch {
            val exists = kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
                PaDownloader.localFileExists(ctx, task.localUri)
            }
            if (exists) {
                PaDownloader.openLocalFile(
                    ctx = ctx,
                    uri = task.localUri,
                    title = task.title,
                    detail = task.detail,
                    onOpenInBrowser = onOpenBrowser,
                )
            } else {
                missingFileIds = missingFileIds + task.id
            }
        }
    }

    /**
     * 失败态重试 / 文件丢失重新下载的唯一入口:门闩拒重入 → 按类别分派 →
     * 重试视觉到冷却窗口结束自动退场。重试语义(结论 3):点了必须有下文 ——
     * 下载类经 [TaskCenter.trackDownload] 原地复活为进行中(进度条接手反馈)。
     * 需要电脑在线的重试(消息找回 / 电脑文件重下)在断线时**不消费门闩直接拒收**:
     * 行内根本不摆可点入口(见行尾动作),这里再拦一道防其它入口绕过。
     */
    fun requestRetry(task: TaskCenter.Task) {
        val plan = if (task.kind == TaskCenter.Kind.DOWNLOAD) {
            redownloadPlanOf(task.id.removePrefix("dl:"))
        } else {
            RedownloadPlan.NONE
        }
        if (hostOffline && (task.kind == TaskCenter.Kind.MESSAGE || plan == RedownloadPlan.FROM_PC)) return
        val gate = retryGates.getOrPut(task.id) { RetryGate() }
        if (!gate.tryBegin()) return
        retryingIds = retryingIds + task.id
        when {
            // 中断/失败的消息类:不只是"引路"—— 先从电脑端把已完成的结果同步回来
            // (找到对应消息并输出到手机端),再进会话
            task.kind == TaskCenter.Kind.MESSAGE && task.threadId.isNotEmpty() -> {
                TurnSession.resyncFromDaemon(task.threadId)
                onOpenThread(task.threadId)
            }
            task.kind == TaskCenter.Kind.DOWNLOAD -> {
                missingFileIds = missingFileIds - task.id
                when (plan) {
                    RedownloadPlan.FROM_URL -> PaDownloader.downloadFromUrl(
                        url = task.id.removePrefix("dl:"),
                        name = task.title,
                        mime = DocFormatRegistry.mimeFor(task.title),
                        sourceTitle = task.sourceTitle,
                        threadId = task.threadId,
                    )
                    RedownloadPlan.FROM_PC -> PaDownloader.downloadFromPc(
                        remotePath = task.id.removePrefix("dl:"),
                        sourceTitle = task.sourceTitle,
                        threadId = task.threadId,
                    )
                    // 不可重取的条目不摆重试钮,走不到这一支
                    RedownloadPlan.NONE -> Unit
                }
            }
        }
        scope.launch {
            kotlinx.coroutines.delay(gate.cooldownMs)
            retryingIds = retryingIds - task.id
        }
    }
    // 旧版本下载条目无 localUri:进任务中心时按文件名找回本地路径,
    // DONE 行的「打开文件」才能正常走 openLocalFile(列表元素原地改+persist 重发射,
    // List.equals 按元素比较 → LaunchedEffect 不会因此重启,legacyTried 双保险防重查)。
    val legacyTried = remember { mutableSetOf<String>() }
    LaunchedEffect(all) {
        for (t in all) {
            if (t.kind != TaskCenter.Kind.DOWNLOAD || t.status != TaskCenter.Status.DONE) continue
            if (t.localUri.isNotEmpty() || !legacyTried.add(t.id)) continue
            val uri = kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
                dev.pocketagent.app.ui.component.PaDownloader.resolveLocalUri(ctx, t.title)
            }
            if (uri.isNotEmpty()) TaskCenter.attachLocalUriIfEmpty(t.id, uri)
        }
    }

    // 进入页面即视为"看过这一屏" —— 清角标(设计规范:角标只负责把人引过来)。
    // 延迟 500ms 再清,避免刚进页面角标就闪没(设计规范 §8)。
    LaunchedEffect(Unit) {
        delay(500)
        TaskCenter.markAllRead()
    }
    // 停留期间新完成的任务直接算已读(设计规范 §8:"已看见不增加未读")
    DisposableEffect(Unit) {
        TaskCenter.setViewing(true)
        onDispose { TaskCenter.setViewing(false) }
    }

    Box(
        modifier
            .fillMaxSize()
            .background(ed.Canvas)
            // 左边缘右滑唤出侧边栏(问题 7)
            .edgeSwipeToOpenDrawer(
                onDragFraction = drawerCtrl::onDrag,
                onRelease = drawerCtrl::onRelease,
            ),
    ) {
        PaScreenScaffold(
            title = "任务中心",
            leading = {
                PaNavIconButton(
                    onClick = { drawerCtrl.open() },
                    contentDescription = "菜单",
                    iconRes = R.drawable.ic_lucide_menu,
                )
            },
            trailing = {
                PaNavIconButton(
                    onClick = onOpenSettings,
                    contentDescription = "设置",
                    iconRes = R.drawable.ic_lucide_settings,
                )
            },
        ) {
            // ---------- 三分区概览卡(设计稿 08):待你处理 / 进行中 / 已完成 ----------
            val stats = taskCenterStats(all)
            Row(
                Modifier.fillMaxWidth().padding(top = 4.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                StatCard(
                    value = stats.waitingForYou.toString(),
                    label = "待你处理",
                    valueColor = EditorialTone.ATTENTION.accent,
                    modifier = Modifier.weight(1f),
                )
                StatCard(
                    value = stats.running.toString(),
                    label = "进行中",
                    valueColor = EditorialTone.RUNNING.accent,
                    modifier = Modifier.weight(1f),
                )
                StatCard(
                    value = stats.completed.toString(),
                    label = "已完成",
                    valueColor = ed.Ink,
                    modifier = Modifier.weight(1f),
                )
            }

            // ---------- 分类切换(消息/下载):选中 = 石墨底白字,未选中 = 瓷白细线 ----------
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(top = 12.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                KindTab("消息与审批", kindTab == TaskCenter.Kind.MESSAGE, Modifier.weight(1f)) {
                    kindTab = TaskCenter.Kind.MESSAGE
                }
                KindTab("文件下载", kindTab == TaskCenter.Kind.DOWNLOAD, Modifier.weight(1f)) {
                    kindTab = TaskCenter.Kind.DOWNLOAD
                }
            }

            Spacer(Modifier.height(6.dp))

            // ---------- 两个窗口:左右滑动切换(与顶部 chip 双向联动) ----------
            val pager = rememberPagerState(pageCount = { 2 })
            LaunchedEffect(kindTab) {
                val target = if (kindTab == TaskCenter.Kind.MESSAGE) 0 else 1
                if (pager.currentPage != target) pager.animateScrollToPage(target)
            }
            LaunchedEffect(pager.currentPage) {
                val k = if (pager.currentPage == 0) TaskCenter.Kind.MESSAGE else TaskCenter.Kind.DOWNLOAD
                if (kindTab != k) kindTab = k
            }

            HorizontalPager(
                state = pager,
                modifier = Modifier.fillMaxSize(),
            ) { page ->
                val pageKind = if (page == 0) TaskCenter.Kind.MESSAGE else TaskCenter.Kind.DOWNLOAD
                val rows = all.filter { it.kind == pageKind }
                if (rows.isEmpty()) {
                    EmptyState(
                        iconRes = R.drawable.ic_lucide_list_checks,
                        title = if (pageKind == TaskCenter.Kind.MESSAGE) "还没有处理中的任务" else "还没有下载任务",
                        hint = if (pageKind == TaskCenter.Kind.MESSAGE) {
                            "在会话里发一条消息,这里会显示它的进度;左右滑动可切到文件下载"
                        } else {
                            "在会话里点文件旁的下载,进度会汇总到这里;左右滑动可切回消息处理"
                        },
                    )
                } else {
                    LazyColumn(
                        Modifier.fillMaxSize(),
                        contentPadding = androidx.compose.foundation.layout.PaddingValues(top = 4.dp, bottom = 96.dp),
                    ) {
                        // 四词分区(空组不出现):分区头 = 状态词本身;断线时消息类运行中
                        // 落「离线待恢复」分区 —— 头不写「正在执行」(状态词纪律,全渲染路径)
                        taskPartitionRows(rows, hostOffline).forEach { row ->
                            item(key = "header-${pageKind.name}-${row.word}") {
                                Text(
                                    row.word,
                                    Modifier.padding(top = 16.dp, bottom = 2.dp),
                                    style = PaEditorial.Type.Heading,
                                    color = ed.Ink,
                                )
                            }
                            items(row.tasks, key = { it.id }) { task ->
                                TaskRow(
                                    task = task,
                                    hostOffline = hostOffline,
                                    fileMissing = task.id in missingFileIds,
                                    retrying = task.id in retryingIds,
                                    onClick = {
                                        TaskCenter.markRead(task.id)
                                        when {
                                            // 消息处理 → 回到那个会话
                                            task.kind == TaskCenter.Kind.MESSAGE && task.threadId.isNotEmpty() ->
                                                onOpenThread(task.threadId)
                                            // 下载项 → 点整行进「用户数据管理」(查看/分享/删除都在那),
                                            // 右侧的地球图标才是"直接打开文件"
                                            task.kind == TaskCenter.Kind.DOWNLOAD -> onOpenUserData()
                                        }
                                    },
                                    onRemove = { TaskCenter.remove(task.id) },
                                    onRetry = { requestRetry(task) },
                                    onOpenFile = { requestOpenFile(task) },
                                )
                                HorizontalHairline()
                            }
                        }
                    }
                }
            }
        }

        // ---------- 底部操作:清理已完成(出现/退场 spring 化,随时可打断) ----------
        val hasFinished = all.any { it.status != TaskCenter.Status.RUNNING }
        AnimatedVisibility(
            visible = hasFinished,
            enter = fadeIn(PaMotion.standard()) + slideInVertically(PaMotion.sheet()) { it },
            exit = fadeOut(PaMotion.nav(700f)) + slideOutVertically(PaMotion.nav(600f)) { it },
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .fillMaxWidth()
                .padding(horizontal = PaEditorial.Space.PageMargin, vertical = 20.dp),
        ) {
            Row(
                Modifier
                    .fillMaxWidth()
                    .navigationBarsPadding()
                    .pressable(haptic = true) { TaskCenter.clearFinished() }
                    // 次操作条:瓷白 + 细线(不与主操作抢层级)
                    .background(ed.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
                    .border(1.dp, ed.Hairline, RoundedCornerShape(PaEditorial.Radius.Card))
                    .padding(vertical = 13.dp),
                horizontalArrangement = Arrangement.Center,
            ) {
                Text("清理已完成的任务", fontSize = 14.sp, color = ed.Muted)
            }
        }

        // ---------- 侧边栏(与其他 tab 一致) ----------
        if (drawerCtrl.visible) {
            AppDrawer(
                currentTab = currentTab,
                hostTitle = "本机任务",
                progress = drawerCtrl.progress,
                onClose = { drawerCtrl.close() },
                onGoTab = { onGoTab(it) },
                onOpenTerminal = onOpenTerminal,
                onOpenSettings = onOpenSettings,
                onOpenTaskCenter = {},
                onOpenBrowser = { drawerCtrl.close(); onOpenBrowser("") },
            )
        }
    }
}

/** 细分隔线(列表行间;留白 + 细线分层,不套卡)。 */
@Composable
private fun HorizontalHairline() {
    Box(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 2.dp)
            .height(1.dp)
            .background(MaterialTheme.editorial.Hairline),
    )
}

/** 三分区概览卡:瓷白 + 细线 + 卡片阴影;数字按语义色,标签中性(设计稿 08)。 */
@Composable
private fun StatCard(
    value: String,
    label: String,
    valueColor: androidx.compose.ui.graphics.Color,
    modifier: Modifier = Modifier,
) {
    val ed = MaterialTheme.editorial
    Column(
        modifier
            .background(ed.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
            .border(1.dp, ed.Hairline, RoundedCornerShape(PaEditorial.Radius.Card))
            .padding(horizontal = 14.dp, vertical = 12.dp),
    ) {
        Text(value, fontSize = 22.sp, lineHeight = 26.sp, fontWeight = FontWeight(720), color = valueColor)
        Text(label, fontSize = 12.sp, lineHeight = 17.sp, color = ed.Muted, modifier = Modifier.padding(top = 2.dp))
    }
}

@Composable
private fun KindTab(label: String, selected: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    val ed = MaterialTheme.editorial
    Box(
        modifier
            .heightIn(min = PaEditorial.Space.TouchMin)
            .pressable(haptic = true) { onClick() }
            .background(
                if (selected) ed.Graphite else ed.Surface,
                RoundedCornerShape(PaEditorial.Radius.Pill),
            )
            .then(
                if (selected) Modifier
                else Modifier.border(1.dp, ed.Hairline, RoundedCornerShape(PaEditorial.Radius.Pill)),
            )
            .padding(horizontal = 14.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            fontSize = 13.sp, lineHeight = 18.sp,
            fontWeight = if (selected) FontWeight(650) else FontWeight.Normal,
            color = if (selected) PaEditorial.OnGraphite else ed.Ink,
            maxLines = 1,
        )
    }
}

/**
 * 单条任务行(设计稿 08:类型图标浅底方格 + 标题 + 四词徽章 + 元信息 + 行尾动作;
 * 留白 + 细分隔线,不套卡)。
 * 进行中:语义色进度条 + 可点"打开会话";失败/中断:重试入口(不伪装可恢复);
 * 文件丢失:行内转"重新下载/移除",不摆点了没反应的死按钮(重试语义,结论 3)。
 */
@Composable
private fun TaskRow(
    task: TaskCenter.Task,
    hostOffline: Boolean,
    fileMissing: Boolean,
    retrying: Boolean,
    onClick: () -> Unit,
    onRemove: () -> Unit,
    onRetry: (() -> Unit)?,
    onOpenFile: (() -> Unit)? = null,
) {
    val ed = MaterialTheme.editorial
    // 四词状态(含离线分支:断连时消息类 RUNNING 不摆「正在执行」徽章)
    val statusView = taskStatusView(task, hostOffline)
    val tone = statusView?.status?.tone ?: EditorialTone.GREY
    // 下载项按文件类型出图(用户反馈:只显示"2.0MB"看不出这是什么文件)
    val kind = if (task.kind == TaskCenter.Kind.DOWNLOAD) fileKindOf(task.title) else null
    // 重新下载可行性(下载类):决定失败/文件丢失态的行尾动作与下一步文案
    val redownload = if (task.kind == TaskCenter.Kind.DOWNLOAD) {
        redownloadPlanOf(task.id.removePrefix("dl:"))
    } else {
        RedownloadPlan.NONE
    }
    val icon = when (task.kind) {
        TaskCenter.Kind.MESSAGE -> R.drawable.ic_lucide_message_square
        TaskCenter.Kind.DOWNLOAD -> kind?.iconRes ?: R.drawable.ic_lucide_download
    }
    val iconTint = kind?.color ?: tone.accent
    val iconBg = if (kind != null) ed.Raised else tone.container

    Column(
        Modifier
            .fillMaxWidth()
            .pressable(haptic = true) { onClick() }
            .padding(vertical = 12.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            // 行首类型图标:42dp 浅底方格(13dp 圆角;语义底色,图标用图形色)
            Box(
                Modifier
                    .size(PaEditorial.Space.IconBox)
                    .background(iconBg, RoundedCornerShape(13.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(painterResource(icon), null, tint = iconTint, modifier = Modifier.size(20.dp))
            }
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        task.title.ifEmpty { "(未命名任务)" },
                        Modifier.weight(1f, fill = false),
                        fontSize = 14.sp, lineHeight = 20.sp,
                        fontWeight = FontWeight(650),
                        color = ed.Ink,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    if (kind != null) {
                        Spacer(Modifier.width(6.dp))
                        // 类型标签:纯色文字(不摆底色 pill)
                        Text(kind.label, fontSize = 10.sp, color = ed.Muted)
                    }
                    if (task.rounds > 1) {
                        Spacer(Modifier.width(6.dp))
                        Text("${task.rounds} 轮", fontSize = 10.sp, color = ed.Muted)
                    }
                }
                Spacer(Modifier.height(2.dp))
                // 元信息行:时间 · 四词徽章 · 来源/详情(语义双通道:词 + 色)
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        SimpleDateFormat("MM-dd HH:mm").format(Date(task.startedAt)),
                        fontSize = 11.sp, lineHeight = 16.sp,
                        color = ed.Subtle,
                    )
                    statusView?.let {
                        Spacer(Modifier.width(8.dp))
                        PaStatusBadge(it)
                    }
                }
                taskOfflineHint(task, hostOffline)?.let { hint ->
                    Text(
                        hint,
                        Modifier.padding(top = 2.dp),
                        fontSize = 11.sp, lineHeight = 16.sp,
                        color = EditorialTone.GREY.text,
                    )
                }
                // 失败/中断:条目详情已答"为什么",这里统一补"现在能做什么"
                // (原因 + 下一步,不带责备语气,不堆技术栈术语 —— 结论 3)
                if (task.status == TaskCenter.Status.FAILED || task.status == TaskCenter.Status.INTERRUPTED) {
                    Text(
                        failureNextStep(
                            kind = task.kind,
                            hostOffline = hostOffline,
                            redownloadNeedsPc = if (task.kind == TaskCenter.Kind.DOWNLOAD) {
                                redownloadNeedsPc(redownload)
                            } else {
                                null
                            },
                            hasSourceThread = task.threadId.isNotEmpty(),
                        ),
                        Modifier.padding(top = 2.dp),
                        fontSize = 11.sp, lineHeight = 16.sp,
                        color = EditorialTone.GREY.text,
                    )
                }
                // 已完成的下载文件丢了:先陈述事实,再给同一套口径的出路
                if (fileMissing && task.status == TaskCenter.Status.DONE &&
                    task.kind == TaskCenter.Kind.DOWNLOAD
                ) {
                    Text(
                        missingFileNextStep(
                            redownloadNeedsPc(redownload),
                            task.threadId.isNotEmpty(),
                            hostOffline,
                        ),
                        Modifier.padding(top = 2.dp),
                        fontSize = 11.sp, lineHeight = 16.sp,
                        color = EditorialTone.GREY.text,
                    )
                }
                if (task.sourceTitle.isNotEmpty()) {
                    Text(
                        "来自会话「" + task.sourceTitle + "」",
                        Modifier.padding(top = 2.dp),
                        fontSize = 11.sp, lineHeight = 16.sp,
                        color = ed.Subtle,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                } else if (task.detail.isNotEmpty()) {
                    Text(
                        // detail 形如「已保存到「下载/PocketAgent/xxx」」,比 content:// 友好
                        task.detail,
                        Modifier.padding(top = 2.dp),
                        fontSize = 11.sp, lineHeight = 16.sp,
                        color = ed.Subtle,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                if (task.sizeText.isNotEmpty()) {
                    Text(
                        task.sizeText,
                        Modifier.padding(top = 2.dp),
                        fontSize = 11.sp, lineHeight = 16.sp,
                        color = ed.Subtle,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
            Spacer(Modifier.width(8.dp))
            when {
                // 重试中:文字占位代替图标、不可点 —— 防双击重入的可见反馈(结论 3)
                retrying && onRetry != null &&
                    (
                        task.status == TaskCenter.Status.FAILED ||
                            task.status == TaskCenter.Status.INTERRUPTED ||
                            (task.status == TaskCenter.Status.DONE && fileMissing)
                        ) ->
                    RetryInFlightLabel()
                task.status == TaskCenter.Status.DONE &&
                    task.kind == TaskCenter.Kind.DOWNLOAD && !fileMissing && onOpenFile != null ->
                    RowActionIcon(R.drawable.ic_lucide_globe, "打开文件", onOpenFile, PaEditorial.Primary)
                // 文件丢失:能重取的给「重新下载」,不能重取的只给「移除」—— 不摆死按钮;
                // 需要电脑的重取在断线时同样不给入口(此刻点了注定失败,文案让用户回头再来)
                task.status == TaskCenter.Status.DONE &&
                    task.kind == TaskCenter.Kind.DOWNLOAD && fileMissing &&
                    redownload != RedownloadPlan.NONE &&
                    !(redownload == RedownloadPlan.FROM_PC && hostOffline) &&
                    onRetry != null ->
                    RowActionIcon(R.drawable.ic_lucide_refresh_cw, "重新下载", onRetry, PaEditorial.Primary)
                task.status == TaskCenter.Status.FAILED || task.status == TaskCenter.Status.INTERRUPTED -> when {
                    // 失败显示重试,不伪装成已完成(README §状态);断线时消息找回无从做起,
                    // 不给一个点了必失败的按钮(行内文案引导恢复连接后再来)
                    task.kind == TaskCenter.Kind.MESSAGE && !hostOffline && onRetry != null ->
                        RowActionIcon(R.drawable.ic_lucide_refresh_cw, "重试", onRetry, PaEditorial.Primary)
                    // 下载类失败:旧实现这里的「重试」点了毫无反应(死按钮)——
                    // 可重取的改「重新下载」(走门闩 + 原地复活),不可重取的诚实给「移除」
                    task.kind == TaskCenter.Kind.DOWNLOAD &&
                        redownload != RedownloadPlan.NONE &&
                        !(redownload == RedownloadPlan.FROM_PC && hostOffline) &&
                        onRetry != null ->
                        RowActionIcon(R.drawable.ic_lucide_refresh_cw, "重新下载", onRetry, PaEditorial.Primary)
                    else ->
                        RowActionIcon(R.drawable.ic_lucide_x, "移除", onRemove, ed.Subtle)
                }
                else ->
                    RowActionIcon(R.drawable.ic_lucide_x, "移除", onRemove, ed.Subtle)
            }
        }
        if (task.status == TaskCenter.Status.RUNNING) {
            Spacer(Modifier.height(10.dp))
            TaskProgressBar(progress = task.progress, track = ed.Hairline, fill = EditorialTone.RUNNING.accent)
        }
    }
}

/** 行尾动作钮:44dp 点击区内的裸图标钮(打开文件/重试/移除)。 */
@Composable
private fun RowActionIcon(iconRes: Int, label: String, onClick: () -> Unit, tint: androidx.compose.ui.graphics.Color) {
    Box(
        Modifier
            .size(PaEditorial.Space.TouchMin)
            .pressable { onClick() },
        contentAlignment = Alignment.Center,
    ) {
        Icon(painterResource(iconRes), label, tint = tint, modifier = Modifier.size(18.dp))
    }
}

/**
 * 重试中的行尾占位(结论 3:重试点了必须有"进行中"的反馈):文字代替图标,
 * 且不是可点目标 —— 第二次点击落在它身上不会触发任何重入。
 */
@Composable
private fun RetryInFlightLabel() {
    Box(
        Modifier.size(PaEditorial.Space.TouchMin),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            "重试中",
            fontSize = 10.sp, lineHeight = 14.sp,
            color = MaterialTheme.editorial.Muted,
        )
    }
}

/** 任务进度条:4dp 细轨道;真实进度 spring 流动,不确定进度匀速扫光(减弱动效退化静态)。 */
@Composable
private fun TaskProgressBar(progress: Float, track: androidx.compose.ui.graphics.Color, fill: androidx.compose.ui.graphics.Color) {
    BoxWithConstraints(
        Modifier
            .fillMaxWidth()
            .height(4.dp)
            // 【修复光线外溢】扫描高亮用 translationX 移动,默认不被父容器裁剪,
            // 高亮会画出进度条、压到卡片文字。clip 之后光带只在 4dp 轨道内扫。
            .clip(RoundedCornerShape(50))
            .background(track, RoundedCornerShape(50)),
    ) {
        if (progress > 0f) {
            // 真实进度:spring 过渡(任务层保证单调不减),进度推进是"流"过去的,
            // 不是一格一格跳;减弱动效时瞬时到位。
            val reduceMotion = rememberReduceMotion()
            val animated by animateFloatAsState(
                targetValue = progress.coerceIn(0.02f, 1f),
                animationSpec = if (reduceMotion) PaMotion.instant() else PaMotion.standard(),
                label = "taskProgress",
            )
            Box(
                Modifier
                    .fillMaxWidth(animated)
                    .fillMaxHeight()
                    .background(fill, RoundedCornerShape(50)),
            )
        } else if (rememberReduceMotion()) {
            // 减弱动效:不确定进度退化为静态半程条(状态可见,位移消失)
            Box(
                Modifier
                    .fillMaxWidth(0.40f)
                    .fillMaxHeight()
                    .background(fill.copy(alpha = 0.6f), RoundedCornerShape(50)),
            )
        } else {
            // 不确定进度:一道 40% 宽的高光匀速扫过(左→右循环,两端羽化)
            val trackPx = with(LocalDensity.current) { maxWidth.toPx() }
            val transition = rememberInfiniteTransition(label = "indeterminate")
            val sweep by transition.animateFloat(
                initialValue = 0f,
                targetValue = 1f,
                animationSpec = infiniteRepeatable(
                    animation = tween(1500, easing = LinearEasing),
                    repeatMode = RepeatMode.Restart,
                ),
                label = "sweep",
            )
            Box(
                Modifier
                    .fillMaxWidth(0.40f)
                    .fillMaxHeight()
                    .graphicsLayer {
                        translationX = trackPx * 1.4f * sweep - trackPx * 0.40f
                    }
                    .background(
                        androidx.compose.ui.graphics.Brush.horizontalGradient(
                            listOf(
                                fill.copy(alpha = 0f),
                                fill.copy(alpha = 0.9f),
                                fill.copy(alpha = 0f),
                            ),
                        ),
                        RoundedCornerShape(50),
                    ),
            )
        }
    }
}
