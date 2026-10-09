package dev.pocketagent.app.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.EmptyState
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.edgeSwipeToOpenDrawer
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.PaEmptyContext
import dev.pocketagent.app.ui.component.PaErrorCard
import dev.pocketagent.app.ui.component.PaHaptics
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.component.PaSkeletonList
import dev.pocketagent.app.ui.component.SkeletonKind
import dev.pocketagent.app.ui.component.SwipeAction
import dev.pocketagent.app.ui.component.SwipeRevealRow
import dev.pocketagent.app.ui.component.paEmptyStateMotifOf
import dev.pocketagent.app.ui.component.paErrorViewOf
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.EditorialTone
import dev.pocketagent.app.ui.component.PaNavIconButton
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CheckoutBranchRequest
import pocketagent.v1.CreateBranchRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.GitBranch
import pocketagent.v1.GitCommitInfo
import pocketagent.v1.GitCommitRequest
import pocketagent.v1.GitDiffRequest
import pocketagent.v1.GitFileChange
import pocketagent.v1.DiscardChangesRequest
import pocketagent.v1.GitLogRequest
import pocketagent.v1.GitStageRequest
import pocketagent.v1.GitStatus
import pocketagent.v1.GitStatusRequest
import pocketagent.v1.GitUnstageRequest
import pocketagent.v1.ListBranchesRequest

/**
 * 解析默认 Git 仓库:在 daemon 上报的工作区 + 最近工作区里,
 * 优先挑一个"真的是 git 仓库"的目录,避免一进 Git 栏就是空态。
 * 返回 (选中的仓库, 全部候选)。
 */
private suspend fun resolveGitRepo(): Pair<String, List<String>> = withContext(Dispatchers.IO) {
    val info = runCatching {
        AgentConnection.connectWithFallback()
            ?: AgentConnection.hostService().getHostInfo(GetHostInfoRequest.getDefaultInstance())
    }.getOrNull()
    val cands = ((info?.workspacesList ?: emptyList()) + AgentConnection.recentWorkspaces.value)
        .map { it.trim() }
        .filter { it.isNotEmpty() }
        .distinct()
    val best = cands.firstOrNull { path ->
        runCatching {
            AgentConnection.gitService().status(GitStatusRequest.newBuilder().setRepo(path).build())
            true
        }.getOrDefault(false)
    } ?: cands.firstOrNull().orEmpty()
    best to cands
}

/**
 * Git 面板:状态 → diff → 提交,全部走 daemon 的 GitService(子进程调 git)。
 * 与其他页统一:纯黑背景 + 深灰圆角卡 + 细字标题头部。
 */
@Composable
fun GitScreen(
    workspacePath: String?,
    currentTab: Int = 3,
    onOpenTerminal: () -> Unit = {},
    onOpenSettings: () -> Unit = {},
    onOpenTaskCenter: () -> Unit = {},
    onOpenBrowser: () -> Unit = {},
    /** 返回层级:二级页面按返回 → 回主页并展开侧边栏。 */
    onBackToHome: () -> Unit = {},
    onGoTab: (Int) -> Unit = {},
) {
    // 【色板并轨 2026-10-09】编辑式 M3 色板已上提 MainActivity NavHost 外层包全 app,
    // 本页不再重复包裹(editorialM3Scheme 定义也已上提 theme 层)
    EditorialSystemBars()
    GitScreenBody(
        workspacePath, currentTab, onOpenTerminal, onOpenSettings,
        onOpenTaskCenter, onOpenBrowser, onBackToHome, onGoTab,
    )
}

@Composable
private fun GitScreenBody(
    workspacePath: String?,
    currentTab: Int = 3,
    onOpenTerminal: () -> Unit = {},
    onOpenSettings: () -> Unit = {},
    onOpenTaskCenter: () -> Unit = {},
    onOpenBrowser: () -> Unit = {},
    onBackToHome: () -> Unit = {},
    onGoTab: (Int) -> Unit = {},
) {
    var showDrawer by remember { mutableStateOf(false) }
    // 侧边栏跟手:与主页/会话页同一套 controller
    val drawerCtrl = dev.pocketagent.app.ui.component.rememberDrawerController()
    val scope = rememberCoroutineScope()
    val view = LocalView.current
    var repo by remember { mutableStateOf(workspacePath ?: "") }
    var status by remember { mutableStateOf<GitStatus?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var diff by remember { mutableStateOf<String?>(null) }
    var showCommit by remember { mutableStateOf(false) }
    var commitMsg by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var notRepo by remember { mutableStateOf(false) }
    // 仓库切换:Git 栏之前"点了没内容、也没交互",核心是仓库路径没解析好。
    var showRepoPicker by remember { mutableStateOf(false) }
    var repoInput by remember { mutableStateOf("") }
    var candidates by remember { mutableStateOf(listOf<String>()) }
    // ---- 三个页签:变更 / 历史 / 分支 ----
    var tab by remember { mutableStateOf(0) }
    var log by remember { mutableStateOf(listOf<GitCommitInfo>()) }
    var branches by remember { mutableStateOf(listOf<GitBranch>()) }
    var diffTitle by remember { mutableStateOf("变更") }
    var openKey = remember { mutableStateOf<Any?>(null) }
    var showCreateBranch by remember { mutableStateOf(false) }
    var newBranch by remember { mutableStateOf("") }
    var pendingDiscard by remember { mutableStateOf<String?>(null) }
    var pendingCheckout by remember { mutableStateOf<String?>(null) }

    /**
     * 【返回层级 · 分级回退】Git 页里叠了 6 个层:仓库选择 / 提交 / 建分支 /
     * diff 查看 / 丢弃确认 / 切分支确认 / 侧边栏。之前的 BackHandler 只认
     * "抽屉 vs 回主页",任何一个弹层开着按返回都会直接跳回主页,把用户正在
     * 做的事全丢掉。这里按从外向内的顺序逐层消费,全部关掉才回主板块。
     *  @return true = 已被消费
     */
    fun backOneLevel(): Boolean = when {
        showRepoPicker -> { showRepoPicker = false; true }
        showCommit -> { showCommit = false; true }
        showCreateBranch -> { showCreateBranch = false; true }
        diff != null -> { diff = null; true }
        pendingDiscard != null -> { pendingDiscard = null; true }
        pendingCheckout != null -> { pendingCheckout = null; true }
        drawerCtrl.visible -> { drawerCtrl.close(); true }
        else -> false
    }

    BackHandler(enabled = true) {
        if (!backOneLevel()) onBackToHome()
    }

    /** 看某个文件的未暂存/已暂存 diff。 */
    fun openFileDiff(path: String, staged: Boolean) {
        scope.launch {
            try {
                val text = withContext(Dispatchers.IO) {
                    AgentConnection.gitService().diff(
                        GitDiffRequest.newBuilder()
                            .setRepo(repo)
                            .setPath(path)
                            .setStaged(staged)
                            .build())
                        .unifiedDiff
                }
                diffTitle = (if (staged) "已暂存 · " else "") + path
                diff = text
            } catch (e: Exception) {
                error = "diff 失败:${e.message}"
            }
        }
    }

    fun refresh() {
        if (repo.isEmpty()) return
        busy = true
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    status = AgentConnection.gitService()
                        .status(GitStatusRequest.newBuilder().setRepo(repo).build())
                    // 历史与分支一起拉:切页签不再等网络
                    log = runCatching {
                        AgentConnection.gitService().log(
                            GitLogRequest.newBuilder().setRepo(repo).setLimit(40).build())
                            .commitsList
                    }.getOrDefault(emptyList())
                    branches = runCatching {
                        AgentConnection.gitService().listBranches(
                            ListBranchesRequest.newBuilder().setRepo(repo).build())
                            .branchesList
                    }.getOrDefault(emptyList())
                }
                error = null
                notRepo = false
            } catch (e: Exception) {
                val msg = "${e.message}"
                val lower = msg.lowercase()
                notRepo = false
                // 非仓库是常态而不是故障:给出友好空态,不刷红字。
                // gRPC 会把 git 的 stderr 包在 status message 里,这里多匹配几种写法。
                if (lower.contains("not a git repository") ||
                    lower.contains("not a git repo") ||
                    lower.contains("不是 git")
                ) {
                    notRepo = true
                } else {
                    error = msg
                }
            } finally {
                busy = false
            }
        }
    }

    /** 看某个提交的 patch(git show)。 */
    fun openCommitDiff(c: GitCommitInfo) {
        scope.launch {
            try {
                val text = withContext(Dispatchers.IO) {
                    AgentConnection.gitService().diff(
                        GitDiffRequest.newBuilder().setRepo(repo).setCommit(c.hash).build())
                        .unifiedDiff
                }
                diffTitle = "${c.shortHash} · ${c.message.lineSequence().firstOrNull().orEmpty()}"
                diff = text
            } catch (e: Exception) {
                error = "查看提交失败:${e.message}"
            }
        }
    }

    /** 暂存 / 取消暂存单个文件。 */
    fun stagePath(path: String, staged: Boolean) {
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    if (staged) {
                        AgentConnection.gitService().unstage(
                            GitUnstageRequest.newBuilder().setRepo(repo).addPaths(path).build())
                    } else {
                        AgentConnection.gitService().stage(
                            GitStageRequest.newBuilder().setRepo(repo).addPaths(path).build())
                    }
                }
                openKey.value = null
                refresh()
            } catch (e: Exception) {
                error = (if (staged) "取消暂存失败:" else "暂存失败:") + "${e.message}"
            }
        }
    }

    /**
     * 解析默认仓库:在 daemon 上报的工作区 + 最近工作区里,
     * 优先挑一个"真的是 git 仓库"的目录,避免一进 Git 栏就是空态。
     */
    suspend fun resolveDefaultRepo(): Pair<String, List<String>> = resolveGitRepo()

    // 断线重连后自动重取(之前只在首帧跑一次,连上后仍然空着)
    val reconnectTick = AgentConnection.connectionEpoch.collectAsState().value
    LaunchedEffect(reconnectTick) {
        if (repo.isBlank()) {
            val (best, all) = resolveDefaultRepo()
            repo = best
            candidates = all
        }
        refresh()
    }

    // 左边缘右滑也能唤出侧边栏(问题 7)
    Column(
        Modifier.fillMaxSize().background(PaEditorial.Canvas)
            .edgeSwipeToOpenDrawer(
                        onDragFraction = drawerCtrl::onDrag,
                        onRelease = drawerCtrl::onRelease,
                    ),
    ) {
        // 头部与会话/文件页统一
        Row(
            Modifier
                .fillMaxWidth()
                .statusBarsPadding()
                .padding(start = PaEditorial.Space.NavGutter, end = 14.dp, top = 8.dp, bottom = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            PaNavIconButton(
                onClick = { drawerCtrl.open() },
                contentDescription = "菜单",
                iconRes = AppR.drawable.ic_lucide_menu,
            )
            Spacer(Modifier.width(12.dp))
            Text(
                "Git" + (status?.branch?.takeIf { it.isNotEmpty() }?.let { " · $it" } ?: ""),
                Modifier.weight(1f),
                style = PaEditorial.Type.Title,
                color = PaEditorial.Ink,
                maxLines = 1,
            )
            if (busy) {
                CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp, color = PaEditorial.Primary)
                Spacer(Modifier.width(12.dp))
            }
            PaNavIconButton(
                onClick = {
                    repoInput = repo
                    showRepoPicker = true
                },
                contentDescription = "切换仓库",
                iconRes = AppR.drawable.ic_lucide_folder,
            )
            Spacer(Modifier.width(8.dp))
            PaNavIconButton(
                onClick = { refresh() },
                contentDescription = "刷新",
                iconRes = AppR.drawable.ic_lucide_refresh_cw,
            )
        }

        // 当前仓库路径:一眼看到"在检查电脑上的哪个目录"
        Text(
            repo.ifBlank { "未解析到仓库目录,点右上角文件夹图标选一个" },
            Modifier
                .fillMaxWidth()
                .padding(start = 16.dp, end = 16.dp, top = 2.dp),
            fontSize = PaEditorial.Type.Micro.fontSize,
            fontFamily = PaEditorial.MonoFont,
            color = PaEditorial.Muted,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )

        // ---- 页签:变更 / 历史 / 分支(daemon 的 10 个 git RPC 之前只用了 status) ----
        Row(
            Modifier.padding(start = 16.dp, end = 16.dp, top = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            val changeCount = status?.changesList?.size ?: 0
            GitTab(if (changeCount > 0) "变更 $changeCount" else "变更", tab == 0) { tab = 0 }
            Spacer(Modifier.width(8.dp))
            GitTab("历史", tab == 1) { tab = 1 }
            Spacer(Modifier.width(8.dp))
            GitTab("分支", tab == 2) { tab = 2 }
            Spacer(Modifier.weight(1f))
            if (repo.isNotEmpty() && !notRepo) {
                Text(
                    "新建分支",
                    Modifier.pressable { newBranch = ""; showCreateBranch = true },
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = PaEditorial.Primary,
                )
            }
        }
        Spacer(Modifier.height(10.dp))

        Column(
            Modifier
                .weight(1f)
                .fillMaxWidth()
                .padding(horizontal = 16.dp)
                .navigationBarsPadding(),
        ) {
            error?.let { raw ->
                // 统一错误卡(PaErrorCard):原文经映射表翻成人话(未配对 → 「还没和电脑配对
                // · 去配对」;连通类 → 「连不上电脑 · 查看排查」),原始错误(含 gRPC 前缀)
                // 点按卡片展开「详情」可查 —— 盘点实锤"裸 mono 小字 + 无重试不一致",此处归一。
                PaErrorCard(
                    view = paErrorViewOf(raw),
                    context = "Git",
                    onRetry = { refresh() },
                    onGoPair = onOpenSettings,
                )
                Spacer(Modifier.height(8.dp))
            }

            when {
                // 有明确错误时不再同时摆一个"这里还不是 Git 仓库"的空态 —— 两者并排会让人分不清到底哪个是真的
                // 【不再整页空白】盘点实锤(截图 29):错误横幅后 tab + 空白,无空态结构。
                // 错误卡在上面讲"为什么",这里补空态讲"现在是什么状态 + 下一步"(发现/连接语境)。
                error != null && status == null -> {
                    EmptyState(
                        AppR.drawable.ic_lucide_monitor,
                        "拿不到仓库状态",
                        "连接恢复后点右上角「刷新」重试;或换一个仓库目录",
                        motif = paEmptyStateMotifOf(PaEmptyContext.DISCOVER),
                    )
                }
                notRepo -> EmptyState(
                    AppR.drawable.ic_lucide_git_branch,
                    "这个目录不是 Git 仓库",
                    "换一个仓库目录,或在电脑上对该目录执行 git init",
                )
                status == null -> {
                    // 首次拉状态:骨架占位,不白屏
                    if (busy) {
                        PaSkeletonList(
                            SkeletonKind.CARD_BLOCK,
                            Modifier.fillMaxWidth().padding(top = 6.dp),
                            count = 4,
                        )
                    }
                }
                else -> {
                    val st = status!!

                    // ---- 分支卡(含提交入口) ----
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
                            .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.Card))
                            .padding(14.dp),
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            androidx.compose.material3.Icon(
                                painterResource(AppR.drawable.ic_lucide_git_branch),
                                null,
                                tint = PaEditorial.Primary,
                                modifier = Modifier.size(16.dp),
                            )
                            Spacer(Modifier.width(8.dp))
                            Text(
                                st.branch.ifEmpty { "(detached)" },
                                fontSize = PaEditorial.Type.Heading.fontSize,
                                fontWeight = FontWeight.SemiBold,
                                color = PaEditorial.Ink,
                            )
                            if (st.upstream.isNotEmpty()) {
                                Spacer(Modifier.width(8.dp))
                                Text(
                                    (if (st.ahead) "↑" else "") + (if (st.behind) "↓" else "") + " " + st.upstream,
                                    fontSize = PaEditorial.Type.Caption.fontSize,
                                    color = PaEditorial.Muted,
                                )
                            }
                            Spacer(Modifier.weight(1f))
                            Text(
                                "${st.changesList.size} 个变更",
                                fontSize = PaEditorial.Type.Caption.fontSize,
                                color = PaEditorial.Muted,
                            )
                        }
                        if (st.changesList.isNotEmpty()) {
                            Spacer(Modifier.height(8.dp))
                            TextButton(
                                onClick = { showCommit = true },
                                enabled = !busy,
                                modifier = Modifier.align(Alignment.End),
                            ) { Text("提交全部", color = PaEditorial.Primary, fontSize = PaEditorial.Type.BodySmall.fontSize) }
                        }
                    }
                    Spacer(Modifier.height(10.dp))

                    when (tab) {
                        // ---------- 变更:按 已暂存/未暂存/未跟踪 分组,左滑暂存,点开看 diff ----------
                        0 -> {
                            if (st.changesList.isEmpty()) {
                                EmptyState(
                                    AppR.drawable.ic_lucide_circle_check,
                                    "工作区干净",
                                    "没有待提交的改动;电脑上有新变化时点右上角「刷新」",
                                )
                            } else {
                                // 分组规则在 GitStatusLogic.kt(纯函数):已暂存 → 未暂存 → 未跟踪
                                val sections = remember(st.changesList) { groupChanges(st.changesList) }
                                LazyColumn(
                                    Modifier.weight(1f).fillMaxWidth(),
                                    verticalArrangement = Arrangement.spacedBy(6.dp),
                                ) {
                                    sections.forEach { (group, rows) ->
                                        item(key = "sec-" + group.name) {
                                            Text(
                                                gitGroupTitle(group) + " · ${rows.size}",
                                                Modifier.padding(start = 2.dp, top = 8.dp, bottom = 2.dp),
                                                fontSize = PaEditorial.Type.Caption.fontSize,
                                                fontWeight = FontWeight.SemiBold,
                                                color = PaEditorial.Muted,
                                            )
                                        }
                                        items(rows, key = { it.path }) { c ->
                                            val staged = c.stagedStatus.isNotEmpty() && c.worktreeStatus.isEmpty()
                                            SwipeRevealRow(
                                                rowKey = c.path,
                                                openKey = openKey,
                                                actions = listOf(
                                                    SwipeAction(
                                                        "stage",
                                                        if (staged) "取消暂存" else "暂存",
                                                        AppR.drawable.ic_lucide_check,
                                                        PaEditorial.Primary,
                                                    ),
                                                    SwipeAction(
                                                        "discard", "丢弃",
                                                        AppR.drawable.ic_lucide_trash_2,
                                                        // 坐 Denied 实底,读 OnDenied 令牌
                                                        PaEditorial.OnDenied, destructive = true,
                                                    ),
                                                ),
                                                onAction = { a ->
                                                    when (a.id) {
                                                        "stage" -> stagePath(c.path, staged)
                                                        // 丢弃不可恢复:进红色强确认,不在滑动里直接生效
                                                        "discard" -> pendingDiscard = c.path
                                                    }
                                                },
                                                rowBackground = PaEditorial.Surface,
                                                onClick = {
                                                    if (openKey.value == c.path) {
                                                        openKey.value = null
                                                    } else {
                                                        openFileDiff(c.path, staged)
                                                    }
                                                },
                                            ) {
                                                ChangeRow(c)
                                            }
                                        }
                                    }
                                    item { Spacer(Modifier.height(80.dp)) }
                                }
                            }
                        }

                        // ---------- 历史:提交列表,点开看该提交的 patch ----------
                        1 -> {
                            if (log.isEmpty()) {
                                EmptyState(
                                    AppR.drawable.ic_lucide_clock,
                                    "没有提交历史",
                                    "这个仓库还没有 commit;在电脑上提交后点「刷新」",
                                )
                            } else {
                                LazyColumn(
                                    Modifier.weight(1f).fillMaxWidth(),
                                    verticalArrangement = Arrangement.spacedBy(6.dp),
                                ) {
                                    items(log, key = { it.hash }) { c ->
                                        CommitRow(c) { openCommitDiff(c) }
                                    }
                                    item { Spacer(Modifier.height(80.dp)) }
                                }
                            }
                        }

                        // ---------- 分支:切换 / 新建 ----------
                        else -> {
                            if (branches.isEmpty()) {
                                EmptyState(
                                    AppR.drawable.ic_lucide_git_branch,
                                    "没有读到分支信息",
                                    "确认这个目录是 Git 仓库后点「刷新」,或换一个仓库",
                                )
                            } else {
                                LazyColumn(
                                    Modifier.weight(1f).fillMaxWidth(),
                                    verticalArrangement = Arrangement.spacedBy(6.dp),
                                ) {
                                    items(branches, key = { it.name }) { b ->
                                        BranchRow(b) {
                                            if (!b.isCurrent) pendingCheckout = b.name
                                        }
                                    }
                                    item { Spacer(Modifier.height(80.dp)) }
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // diff 查看器:行级染色(+绿 / -红 / @@ 蓝)。内容区保持代码深色底
    // (石墨代码箱,与侧边栏/代码预览同一纪律);【D2 模态家族统一】裸 AlertDialog
    // → PaSheet centered 玻璃皮。语义零变化:点遮罩/返回/「关闭」都只是收起查看器
    // (diff = null),不动工作区任何状态。
    diff?.let { d ->
        PaSheet(onDismissRequest = { diff = null }, centered = true) {
            Column(Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 18.dp)) {
                Text(
                    // 深路径标题中间省略,保住文件名与扩展名
                    middleEllipsize(diffTitle, 64),
                    fontSize = PaEditorial.Type.Body.fontSize, lineHeight = 20.sp,
                    maxLines = 2, overflow = TextOverflow.Ellipsis,
                    color = PaEditorial.Ink,
                )
                Spacer(Modifier.height(10.dp))
                Column(
                    Modifier
                        .fillMaxWidth()
                        .background(PaEditorial.GraphiteCode, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .padding(horizontal = 10.dp, vertical = 8.dp),
                ) {
                    DiffView(
                        text = d,
                        modifier = Modifier.height(440.dp).fillMaxWidth(),
                    )
                }
                Spacer(Modifier.height(16.dp))
                GitSheetDialogButton(
                    text = "关闭",
                    tones = GitSheetButtonTones.primary,
                    modifier = Modifier.fillMaxWidth(),
                ) { diff = null }
            }
        }
    }

    // 丢弃单个文件的改动 —— 危险级 DESTRUCTIVE(gitActionDanger):不可恢复、Git 也找不回。
    // 红色实心确认钮 + 后果写明,与普通"确认"分级(gitDangerConfirmLabel 给出动作词)。
    // 【D2 模态家族统一】裸 AlertDialog → PaSheet centered;危险实心钮的 Denied/OnDenied
    // 色槽原样保留(本文件自己的危险分级语言)。语义零变化:遮罩/返回 = 取消。
    pendingDiscard?.let { path ->
        PaSheet(onDismissRequest = { pendingDiscard = null }, centered = true) {
            Column(Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 18.dp)) {
                Text(
                    "丢弃改动?",
                    fontSize = PaEditorial.Type.Heading.fontSize,
                    color = PaEditorial.Ink,
                )
                Spacer(Modifier.height(10.dp))
                Text(
                    "将撤销 ${middleEllipsize(path, 40)} 上未暂存的修改。\n" +
                        "未提交的内容不会进回收站,也无法通过 Git 找回,此操作不可恢复。",
                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                    color = PaEditorial.Muted,
                )
                Spacer(Modifier.height(18.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    GitSheetDialogButton(
                        text = "取消",
                        tones = GitSheetButtonTones.secondary,
                        modifier = Modifier.weight(1f),
                    ) { pendingDiscard = null }
                    GitSheetDialogButton(
                        text = gitDangerConfirmLabel("discard"),
                        tones = GitSheetButtonTones.destructiveSolid,
                        modifier = Modifier.weight(1f),
                    ) {
                        pendingDiscard = null
                        PaHaptics.warning(view)
                        scope.launch {
                            try {
                                withContext(Dispatchers.IO) {
                                    AgentConnection.gitService().discardChanges(
                                        DiscardChangesRequest.newBuilder()
                                            .setRepo(repo).addPaths(path).build())
                                }
                                openKey.value = null
                                refresh()
                            } catch (e: Exception) {
                                error = "丢弃失败:${e.message}"
                            }
                        }
                    }
                }
            }
        }
    }

    // 切换分支(有未提交改动时 git 会拒绝,由 daemon 的错误信息回显)
    // 【D2 模态家族统一】裸 AlertDialog → PaSheet centered;普通确认的主按钮走
    // 石墨重点面。语义零变化:遮罩/返回 = 取消(pendingCheckout = null)。
    pendingCheckout?.let { name ->
        PaSheet(onDismissRequest = { pendingCheckout = null }, centered = true) {
            Column(Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 18.dp)) {
                Text(
                    "切换到 $name?",
                    fontSize = PaEditorial.Type.Heading.fontSize,
                    color = PaEditorial.Ink,
                )
                Spacer(Modifier.height(10.dp))
                Text(
                    "工作区有未提交改动时,git 会拒绝切换,可先提交或丢弃。",
                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                    color = PaEditorial.Muted,
                )
                Spacer(Modifier.height(18.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    GitSheetDialogButton(
                        text = "取消",
                        tones = GitSheetButtonTones.secondary,
                        modifier = Modifier.weight(1f),
                    ) { pendingCheckout = null }
                    GitSheetDialogButton(
                        text = "切换",
                        tones = GitSheetButtonTones.primary,
                        modifier = Modifier.weight(1f),
                    ) {
                        pendingCheckout = null
                        scope.launch {
                            try {
                                withContext(Dispatchers.IO) {
                                    AgentConnection.gitService().checkoutBranch(
                                        CheckoutBranchRequest.newBuilder()
                                            .setRepo(repo).setBranch(name).build())
                                }
                                refresh()
                            } catch (e: Exception) {
                                error = "切换分支失败:${e.message}"
                            }
                        }
                    }
                }
            }
        }
    }

    // 新建分支
    // 【D2 模态家族统一】裸 AlertDialog → PaSheet centered + 输入行;M3 的
    // dismiss/confirm 底部文字钮重排为 PaSheet 约定的编辑式按钮行(取消/创建)。
    // 语义零变化:空名不打 RPC 直接返回;遮罩/返回 = 取消(输入不提交)。
    if (showCreateBranch) {
        PaSheet(onDismissRequest = { showCreateBranch = false }, centered = true) {
            Column(Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 18.dp)) {
                Text(
                    "新建分支",
                    fontSize = PaEditorial.Type.Heading.fontSize,
                    color = PaEditorial.Ink,
                )
                Spacer(Modifier.height(12.dp))
                OutlinedTextField(
                    value = newBranch,
                    onValueChange = { newBranch = it },
                    singleLine = true,
                    label = { Text("分支名") },
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(18.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    GitSheetDialogButton(
                        text = "取消",
                        tones = GitSheetButtonTones.secondary,
                        modifier = Modifier.weight(1f),
                    ) { showCreateBranch = false }
                    GitSheetDialogButton(
                        text = "创建",
                        tones = GitSheetButtonTones.primary,
                        modifier = Modifier.weight(1f),
                    ) {
                        val name = newBranch.trim()
                        if (name.isEmpty()) return@GitSheetDialogButton
                        showCreateBranch = false
                        scope.launch {
                            try {
                                withContext(Dispatchers.IO) {
                                    AgentConnection.gitService().createBranch(
                                        CreateBranchRequest.newBuilder()
                                            .setRepo(repo).setName(name).build())
                                }
                                refresh()
                            } catch (e: Exception) {
                                error = "新建分支失败:${e.message}"
                            }
                        }
                    }
                }
            }
        }
    }

    // 提交对话框
    // 【D2 模态家族统一】裸 AlertDialog → PaSheet centered + 输入行;提交钮按
    // PaSheet 约定重排为石墨主按钮,禁用条件原样(commit 非空且不在飞行中)。
    // 语义零变化:遮罩/返回 = 取消(showCommit = false,信息保留在 commitMsg)。
    if (showCommit) {
        PaSheet(onDismissRequest = { showCommit = false }, centered = true) {
            Column(Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 18.dp)) {
                Text(
                    "提交全部变更",
                    fontSize = PaEditorial.Type.Heading.fontSize,
                    color = PaEditorial.Ink,
                )
                Spacer(Modifier.height(12.dp))
                OutlinedTextField(
                    commitMsg, { commitMsg = it },
                    label = { Text("提交信息") },
                    modifier = Modifier.fillMaxWidth(), minLines = 2,
                )
                Spacer(Modifier.height(18.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    GitSheetDialogButton(
                        text = "取消",
                        tones = GitSheetButtonTones.secondary,
                        modifier = Modifier.weight(1f),
                    ) { showCommit = false }
                    GitSheetDialogButton(
                        text = "提交",
                        tones = GitSheetButtonTones.primary,
                        enabled = commitMsg.isNotBlank() && !busy,
                        modifier = Modifier.weight(1f),
                    ) {
                        busy = true
                        scope.launch {
                            try {
                                withContext(Dispatchers.IO) {
                                    AgentConnection.gitService().commit(
                                        GitCommitRequest.newBuilder().setRepo(repo).setMessage(commitMsg.trim()).build())
                                }
                                showCommit = false
                                commitMsg = ""
                                refresh()
                            } catch (e: Exception) {
                                error = "提交失败:${e.message}"
                            } finally {
                                busy = false
                            }
                        }
                    }
                }
            }
        }
    }

    // 切换仓库:已知工作区 + 手填路径(修复"看不到任何 git 内容")
    // 仓库选择:与"新建对话选工作区"完全同款 —— 可以在手机上浏览整台电脑的目录,
    // 不再只能从几个候选里挑或者手打路径。
    if (showRepoPicker) {
        FolderPickerSheet(
            onDismiss = { showRepoPicker = false },
            onPick = { path ->
                showRepoPicker = false
                repo = path
                repoInput = path
                notRepo = false
                error = null
                refresh()
            },
        )
    }

    if (drawerCtrl.visible) {
        AppDrawer(
            currentTab = currentTab,
            hostTitle = "Git",
            progress = drawerCtrl.progress,
            onClose = { drawerCtrl.close() },
            onGoTab = { onGoTab(it) },
            onOpenTerminal = { onOpenTerminal() },
            onOpenSettings = { onOpenSettings() },
            onOpenTaskCenter = onOpenTaskCenter,
            onOpenBrowser = { onOpenBrowser() },
        )
    }
}


/** 页签 chip(变更 / 历史 / 分支),与设置页、会话页同款胶囊样式。 */
@Composable
private fun GitTab(label: String, selected: Boolean, onClick: () -> Unit) {
    Box(
        Modifier
            .pressable(haptic = true) { onClick() }
            // 触达区 ≥44dp(工程纪律 R2-D 7):视觉仍按胶囊排,热区拉到下限
            .heightIn(min = 44.dp)
            .background(
                // chips 家族统一(2026-10-09):选中 = 石墨底白字,未选中 = 透明底细线描边,
                // 与 EditorialFilterChip/KindTab 同一语言(盘点实锤:此处原为蓝底选中,与全局不一致)
                if (selected) PaEditorial.Graphite else Color.Transparent,
                RoundedCornerShape(PaEditorial.Radius.Pill),
            )
            .then(
                if (selected) Modifier else Modifier.border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.Pill)),
            )
            .padding(horizontal = 14.dp, vertical = 7.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
            color = if (selected) PaEditorial.OnGraphite else PaEditorial.Muted,
        )
    }
}

/** 变更行:路径 + 暂存/工作区状态码(带颜色,和 git status 的语感一致)。 */
/** git porcelain 状态字母 → 中文标签(纯函数,可单测);空串 = 无改动。 */
internal fun gitStatusLabel(letter: String): String = when (letter) {
    "" -> "-"
    "M" -> "修改"
    "A" -> "新增"
    "D" -> "删除"
    "R" -> "改名"
    "C" -> "复制"
    "U" -> "冲突"
    "T" -> "类型变化"
    "?" -> "未跟踪"
    else -> letter // 未知状态原样显示,不吞
}

/** 是否未跟踪文件(porcelain `??`:两列都是 "?")。 */
internal fun isUntrackedChange(staged: String, worktree: String): Boolean =
    staged == "?" && worktree == "?"

@Composable
private fun ChangeRow(c: GitFileChange) {
    val staged = c.stagedStatus.isNotEmpty() && c.worktreeStatus.isEmpty()
    val untracked = isUntrackedChange(c.stagedStatus, c.worktreeStatus)
    // 路径拆两段:文件名必须**永远完整可读**(旧实现整串末尾省略,
    // 长路径下 android/app/.../Foo.kt 的文件名整个被切掉,只剩目录前缀);
    // 目录段单独一行小字,末尾省略也不影响辨认。
    val fileName = c.path.substringAfterLast('/')
    val dirPart = c.path.substringBeforeLast('/', "")
    Column(
        Modifier
            .fillMaxWidth()
            .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .padding(horizontal = 12.dp, vertical = 10.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                Modifier
                    .size(20.dp)
                    .background(
                        if (staged) PaEditorial.PrimaryContainer else PaEditorial.Raised,
                        RoundedCornerShape(PaEditorial.Radius.Terminal),
                    ),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    // 未跟踪显示"新"而不是"?"(用户反馈:?? 看不懂);
                    // 其余保持工作区优先的单字母 + 中文在下方说明行里。
                    if (untracked) "新" else (c.worktreeStatus.ifEmpty { c.stagedStatus }).take(2).ifEmpty { "?" },
                    fontSize = PaEditorial.Type.Micro.fontSize,
                    color = if (staged) PaEditorial.Primary else PaEditorial.Muted,
                )
            }
            Spacer(Modifier.width(9.dp))
            Text(
                // 长文件名中间省略保扩展名(FilenameDisplay 规则);目录段在下一行单独展示
                middleEllipsize(fileName, 28),
                Modifier.weight(1f),
                fontSize = PaEditorial.Type.BodySmall.fontSize,
                fontFamily = PaEditorial.MonoFont,
                color = PaEditorial.Ink,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (staged) {
                Text("已暂存", fontSize = PaEditorial.Type.Micro.fontSize, color = EditorialTone.RUNNING.text, fontWeight = FontWeight.SemiBold)
            }
        }
        if (dirPart.isNotEmpty()) {
            Text(
                dirPart,
                Modifier.padding(top = 2.dp, start = 29.dp),
                fontSize = PaEditorial.Type.Micro.fontSize,
                fontFamily = PaEditorial.MonoFont,
                color = PaEditorial.Muted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        Text(
            // 状态字母翻成中文:暂存:M/工作区:? 这种写法就是把 git 内部表示直接糊用户脸上。
            if (untracked) {
                "未跟踪 · 左滑可暂存/丢弃"
            } else {
                "暂存:${gitStatusLabel(c.stagedStatus)} · 工作区:${gitStatusLabel(c.worktreeStatus)} · 左滑可暂存/丢弃"
            },
            Modifier.padding(top = 3.dp, start = 29.dp),
            fontSize = PaEditorial.Type.Micro.fontSize,
            color = PaEditorial.Muted,
            maxLines = 1,
        )
    }
}

/** 提交行:标题 + 短哈希/作者/时间,点开看这个提交的 patch。 */
@Composable
private fun CommitRow(c: GitCommitInfo, onClick: () -> Unit) {
    Column(
        Modifier
            .fillMaxWidth()
            .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .pressable(haptic = true) { onClick() }
            .padding(horizontal = 12.dp, vertical = 10.dp),
    ) {
        Text(
            c.message.lineSequence().firstOrNull().orEmpty().ifEmpty { "(无提交信息)" },
            fontSize = PaEditorial.Type.BodySmall.fontSize,
            color = PaEditorial.Ink,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
        )
        Spacer(Modifier.height(3.dp))
        val when_ = if (c.timestampUnixMs > 0) {
            java.text.SimpleDateFormat("MM-dd HH:mm", java.util.Locale.getDefault())
                .format(java.util.Date(c.timestampUnixMs))
        } else ""
        Text(
            listOf(c.shortHash.ifEmpty { c.hash.take(7) }, c.author, when_)
                .filter { it.isNotEmpty() }
                .joinToString(" · "),
            fontSize = PaEditorial.Type.Micro.fontSize,
            color = PaEditorial.Muted,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

/** 分支行:当前分支高亮标记,点其他分支走切换确认。 */
@Composable
private fun BranchRow(b: GitBranch, onClick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .pressable(haptic = true) { onClick() }
            .padding(horizontal = 12.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        androidx.compose.material3.Icon(
            painterResource(AppR.drawable.ic_lucide_git_branch),
            null,
            tint = if (b.isCurrent) PaEditorial.Primary else PaEditorial.Muted,
            modifier = Modifier.size(14.dp),
        )
        Spacer(Modifier.width(9.dp))
        Text(
            b.name,
            Modifier.weight(1f),
            fontSize = PaEditorial.Type.BodySmall.fontSize,
            color = if (b.isCurrent) PaEditorial.Primary else PaEditorial.Ink,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        Text(
            if (b.isCurrent) "当前" else if (b.isRemote) "远端" else "切换",
            fontSize = PaEditorial.Type.Micro.fontSize,
            color = if (b.isCurrent) PaEditorial.Primary else PaEditorial.Muted,
        )
    }
}

/**
 * diff 渲染:按行染色(+ 绿 / - 红 / @@ 蓝 / 文件头灰),
 * 用 LazyColumn 承载,几万行的 patch 也不会卡。
 */
@Composable
private fun DiffView(text: String, modifier: Modifier = Modifier) {
    val lines = remember(text) { text.split("\n") }
    if (lines.isEmpty() || text.isBlank()) {
        Text("(无差异内容)", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.OnGraphiteSubtle, modifier = modifier)
        return
    }
    LazyColumn(modifier = modifier) {
        items(lines.size) { i ->
            val line = lines[i]
            // 深色代码箱上的行色:文件头灰 / @@ 亮蓝 / + 薄荷绿 / - 浅珊瑚 / 正文代码色
            val color = when {
                line.startsWith("diff --git") || line.startsWith("index ") ||
                    line.startsWith("--- ") || line.startsWith("+++ ") -> PaEditorial.OnGraphiteSubtle
                line.startsWith("@@") -> Color(0xFF7FA7F0)
                line.startsWith("+") -> Color(0xFF5FD4AE)
                line.startsWith("-") -> Color(0xFFF2A09E)
                else -> PaEditorial.CodeText
            }
            Text(
                line.ifEmpty { " " },
                fontSize = PaEditorial.Type.Micro.fontSize,
                fontFamily = PaEditorial.MonoFont,
                color = color,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}

// ---- PaSheet centered 弹窗的编辑式按钮(D2 模态家族统一;规格照抄
// PaApprovalDialog 的 ApprovalButton:≥50dp 触达、IconButton 圆角、色调取槽位)----

/** 弹窗按钮的编辑式色调(容器/描边/内容三色,一律取 PaEditorial 槽位)。 */
private data class GitSheetButtonTones(val container: Color, val outline: Color, val content: Color) {
    companion object {
        /** 主操作:石墨实底白字(README:主操作用石墨重点面)。 */
        val primary get() = GitSheetButtonTones(PaEditorial.Graphite, PaEditorial.Graphite, PaEditorial.OnGraphite)

        /** 次操作(取消):浅底描边。 */
        val secondary get() = GitSheetButtonTones(PaEditorial.Raised, PaEditorial.Hairline, PaEditorial.Ink)

        /** 危险确认(本文件分级语言):红色实心 + OnDenied 字,原 pendingDiscard 实心钮同款。 */
        val destructiveSolid get() = GitSheetButtonTones(PaEditorial.Denied, PaEditorial.Denied, PaEditorial.OnDenied)
    }
}

@Composable
private fun GitSheetDialogButton(
    text: String,
    tones: GitSheetButtonTones,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(PaEditorial.Radius.IconButton)
    Row(
        modifier
            .heightIn(min = 50.dp) // 最小触达纪律
            .background(if (enabled) tones.container else tones.container.copy(alpha = 0.55f), shape)
            .border(1.dp, if (enabled) tones.outline else tones.outline.copy(alpha = 0.55f), shape)
            .pressable(enabled = enabled, onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 13.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.Center,
    ) {
        Text(
            text,
            fontSize = PaEditorial.Type.Body.fontSize,
            fontWeight = FontWeight.SemiBold,
            color = (if (enabled) tones.content else tones.content.copy(alpha = 0.55f)),
            maxLines = 1,
        )
    }
}
