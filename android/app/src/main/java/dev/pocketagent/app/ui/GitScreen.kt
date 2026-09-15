package dev.pocketagent.app.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.clickable
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
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
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontFamily
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
import dev.pocketagent.app.ui.component.SwipeAction
import dev.pocketagent.app.ui.component.SwipeRevealRow
import dev.pocketagent.app.ui.theme.Pa
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
    var showDrawer by remember { mutableStateOf(false) }
    // 返回层级:抽屉开着先关抽屉,否则回到主页并展开侧边栏(而不是直接退回桌面)
    BackHandler(enabled = true) {
        if (showDrawer) showDrawer = false else onBackToHome()
    }
    val scope = rememberCoroutineScope()
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
        Modifier.fillMaxSize().background(Pa.Bg)
            .edgeSwipeToOpenDrawer { showDrawer = true },
    ) {
        // 头部与会话/文件页统一
        Row(
            Modifier
                .fillMaxWidth()
                .statusBarsPadding()
                .padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconActionButton(AppR.drawable.ic_lucide_menu, "菜单", { showDrawer = true }, bg = Pa.Card)
            Spacer(Modifier.width(12.dp))
            Text(
                "Git" + (status?.branch?.takeIf { it.isNotEmpty() }?.let { " · $it" } ?: ""),
                Modifier.weight(1f),
                fontSize = 22.sp,
                fontWeight = FontWeight.Bold,
                color = Pa.TextPrimary,
            )
            if (busy) {
                CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp, color = Pa.Accent)
                Spacer(Modifier.width(12.dp))
            }
            IconActionButton(AppR.drawable.ic_lucide_folder, "切换仓库", {
                repoInput = repo
                showRepoPicker = true
            }, bg = Pa.Card)
            Spacer(Modifier.width(10.dp))
            IconActionButton(AppR.drawable.ic_lucide_refresh_cw, "刷新", { refresh() }, bg = Pa.Card)
        }

        // 当前仓库路径:一眼看到"在检查电脑上的哪个目录"
        Text(
            repo.ifBlank { "未解析到仓库目录,点右上角文件夹图标选一个" },
            Modifier
                .fillMaxWidth()
                .padding(start = 16.dp, end = 16.dp, top = 2.dp),
            fontSize = 10.5.sp,
            fontFamily = FontFamily.Monospace,
            color = Pa.TextTertiary,
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
                    fontSize = 12.sp,
                    color = Pa.Accent,
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
            error?.let {
                Text(
                    it.removePrefix("INVALID_ARGUMENT: ").removePrefix("INTERNAL: "),
                    Modifier
                        .fillMaxWidth()
                        .background(Pa.Red.copy(alpha = 0.12f), RoundedCornerShape(12.dp))
                        .padding(12.dp),
                    color = Pa.Red,
                    fontSize = 12.5.sp,
                )
                Spacer(Modifier.height(8.dp))
            }

            when {
                // 有明确错误时不再同时摆一个"这里还不是 Git 仓库"的空态 —— 两者并排会让人分不清到底哪个是真的
                error != null && status == null -> {}
                notRepo -> EmptyState(
                    AppR.drawable.ic_lucide_git_branch,
                    "这个目录不是 Git 仓库",
                    "换一个仓库目录,或在电脑上对该目录执行 git init",
                )
                status == null -> {}
                else -> {
                    val st = status!!

                    // ---- 分支卡(含提交入口) ----
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .background(Pa.Card, RoundedCornerShape(14.dp))
                            .padding(14.dp),
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            androidx.compose.material3.Icon(
                                painterResource(AppR.drawable.ic_lucide_git_branch),
                                null,
                                tint = Pa.Accent,
                                modifier = Modifier.size(16.dp),
                            )
                            Spacer(Modifier.width(8.dp))
                            Text(
                                st.branch.ifEmpty { "(detached)" },
                                fontSize = 15.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = Pa.TextPrimary,
                            )
                            if (st.upstream.isNotEmpty()) {
                                Spacer(Modifier.width(8.dp))
                                Text(
                                    (if (st.ahead) "↑" else "") + (if (st.behind) "↓" else "") + " " + st.upstream,
                                    fontSize = 11.sp,
                                    color = Pa.TextSecondary,
                                )
                            }
                            Spacer(Modifier.weight(1f))
                            Text(
                                "${st.changesList.size} 个变更",
                                fontSize = 11.sp,
                                color = Pa.TextTertiary,
                            )
                        }
                        if (st.changesList.isNotEmpty()) {
                            Spacer(Modifier.height(8.dp))
                            TextButton(
                                onClick = { showCommit = true },
                                enabled = !busy,
                                modifier = Modifier.align(Alignment.End),
                            ) { Text("提交全部", color = Pa.Accent, fontSize = 13.sp) }
                        }
                    }
                    Spacer(Modifier.height(10.dp))

                    when (tab) {
                        // ---------- 变更:左滑暂存/取消暂存,点开看 diff ----------
                        0 -> {
                            if (st.changesList.isEmpty()) {
                                Text(
                                    "工作区干净,没有待提交的改动",
                                    Modifier.padding(vertical = 20.dp),
                                    fontSize = 13.sp,
                                    color = Pa.TextSecondary,
                                )
                            } else {
                                LazyColumn(
                                    Modifier.weight(1f).fillMaxWidth(),
                                    verticalArrangement = Arrangement.spacedBy(6.dp),
                                ) {
                                    items(st.changesList, key = { it.path }) { c ->
                                        val staged = c.stagedStatus.isNotEmpty() && c.worktreeStatus.isEmpty()
                                        SwipeRevealRow(
                                            rowKey = c.path,
                                            openKey = openKey,
                                            actions = listOf(
                                                SwipeAction(
                                                    "stage",
                                                    if (staged) "取消暂存" else "暂存",
                                                    AppR.drawable.ic_lucide_check,
                                                    Pa.AccentText,
                                                ),
                                                SwipeAction(
                                                    "discard", "丢弃",
                                                    AppR.drawable.ic_lucide_trash_2,
                                                    Color.White, destructive = true,
                                                ),
                                            ),
                                            onAction = { a ->
                                                when (a.id) {
                                                    "stage" -> stagePath(c.path, staged)
                                                    "discard" -> pendingDiscard = c.path
                                                }
                                            },
                                            rowBackground = Pa.Card,
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
                                    item { Spacer(Modifier.height(80.dp)) }
                                }
                            }
                        }

                        // ---------- 历史:提交列表,点开看该提交的 patch ----------
                        1 -> {
                            if (log.isEmpty()) {
                                Text(
                                    "没有提交历史(仓库还没有 commit)",
                                    Modifier.padding(vertical = 20.dp),
                                    fontSize = 13.sp,
                                    color = Pa.TextSecondary,
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
                                Text(
                                    "没有读到分支信息",
                                    Modifier.padding(vertical = 20.dp),
                                    fontSize = 13.sp,
                                    color = Pa.TextSecondary,
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

    // diff 查看器:行级染色(+绿 / -红 / @@ 蓝),不再是"一大坨灰字"
    diff?.let { d ->
        AlertDialog(
            onDismissRequest = { diff = null },
            title = { Text(diffTitle, fontSize = 14.sp, maxLines = 2, overflow = TextOverflow.Ellipsis) },
            text = {
                DiffView(
                    text = d,
                    modifier = Modifier.height(440.dp).fillMaxWidth(),
                )
            },
            confirmButton = { TextButton(onClick = { diff = null }) { Text("关闭") } },
        )
    }

    // 丢弃单个文件的改动(危险操作,二次确认)
    pendingDiscard?.let { path ->
        AlertDialog(
            onDismissRequest = { pendingDiscard = null },
            title = { Text("丢弃改动?", fontSize = 15.sp) },
            text = {
                Text(
                    "将撤销 $path 上未暂存的修改,此操作不可恢复。",
                    fontSize = 12.5.sp,
                    color = Pa.TextSecondary,
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    pendingDiscard = null
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
                }) { Text("丢弃", color = Pa.Red) }
            },
            dismissButton = {
                TextButton(onClick = { pendingDiscard = null }) { Text("取消") }
            },
        )
    }

    // 切换分支(有未提交改动时 git 会拒绝,由 daemon 的错误信息回显)
    pendingCheckout?.let { name ->
        AlertDialog(
            onDismissRequest = { pendingCheckout = null },
            title = { Text("切换到 $name?", fontSize = 15.sp) },
            text = {
                Text(
                    "工作区有未提交改动时,git 会拒绝切换,可先提交或丢弃。",
                    fontSize = 12.5.sp,
                    color = Pa.TextSecondary,
                )
            },
            confirmButton = {
                TextButton(onClick = {
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
                }) { Text("切换", color = Pa.Accent) }
            },
            dismissButton = {
                TextButton(onClick = { pendingCheckout = null }) { Text("取消") }
            },
        )
    }

    // 新建分支
    if (showCreateBranch) {
        AlertDialog(
            onDismissRequest = { showCreateBranch = false },
            title = { Text("新建分支", fontSize = 15.sp) },
            text = {
                OutlinedTextField(
                    value = newBranch,
                    onValueChange = { newBranch = it },
                    singleLine = true,
                    label = { Text("分支名") },
                    modifier = Modifier.fillMaxWidth(),
                )
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        val name = newBranch.trim()
                        if (name.isEmpty()) return@TextButton
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
                    },
                ) { Text("创建", color = Pa.Accent) }
            },
            dismissButton = {
                TextButton(onClick = { showCreateBranch = false }) { Text("取消") }
            },
        )
    }

    // 提交对话框
    if (showCommit) {
        AlertDialog(
            onDismissRequest = { showCommit = false },
            title = { Text("提交全部变更") },
            text = {
                OutlinedTextField(
                    commitMsg, { commitMsg = it },
                    label = { Text("提交信息") },
                    modifier = Modifier.fillMaxWidth(), minLines = 2,
                )
            },
            confirmButton = {
                TextButton(
                    enabled = commitMsg.isNotBlank() && !busy,
                    onClick = {
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
                    },
                ) { Text("提交") }
            },
            dismissButton = { TextButton(onClick = { showCommit = false }) { Text("取消") } },
        )
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

    if (showDrawer) {
        AppDrawer(
            currentTab = currentTab,
            hostTitle = "Git",
            onClose = { showDrawer = false },
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
            .background(if (selected) Pa.Accent else Pa.Card, RoundedCornerShape(50))
            .padding(horizontal = 14.dp, vertical = 7.dp),
    ) {
        Text(label, fontSize = 12.sp, color = if (selected) Color.White else Pa.TextSecondary)
    }
}

/** 变更行:路径 + 暂存/工作区状态码(带颜色,和 git status 的语感一致)。 */
@Composable
private fun ChangeRow(c: GitFileChange) {
    val staged = c.stagedStatus.isNotEmpty() && c.worktreeStatus.isEmpty()
    Column(
        Modifier
            .fillMaxWidth()
            .background(Pa.Card, RoundedCornerShape(12.dp))
            .padding(horizontal = 12.dp, vertical = 10.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                Modifier
                    .size(20.dp)
                    .background(if (staged) Pa.Accent.copy(alpha = 0.18f) else Pa.Hover, RoundedCornerShape(6.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    (c.worktreeStatus.ifEmpty { c.stagedStatus }).take(2).ifEmpty { "?" },
                    fontSize = 10.sp,
                    color = if (staged) Pa.Accent else Pa.TextSecondary,
                )
            }
            Spacer(Modifier.width(9.dp))
            Text(
                c.path,
                Modifier.weight(1f),
                fontSize = 13.sp,
                fontFamily = FontFamily.Monospace,
                color = Pa.TextPrimary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (staged) {
                Text("已暂存", fontSize = 10.sp, color = Pa.Accent)
            }
        }
        Text(
            "暂存:${c.stagedStatus.ifEmpty { "-" }} · 工作区:${c.worktreeStatus.ifEmpty { "-" }} · 左滑可暂存/丢弃",
            Modifier.padding(top = 3.dp, start = 29.dp),
            fontSize = 10.sp,
            color = Pa.TextTertiary,
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
            .background(Pa.Card, RoundedCornerShape(12.dp))
            .pressable(haptic = true) { onClick() }
            .padding(horizontal = 12.dp, vertical = 10.dp),
    ) {
        Text(
            c.message.lineSequence().firstOrNull().orEmpty().ifEmpty { "(无提交信息)" },
            fontSize = 13.sp,
            color = Pa.TextPrimary,
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
            fontSize = 10.sp,
            color = Pa.TextTertiary,
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
            .background(Pa.Card, RoundedCornerShape(12.dp))
            .pressable(haptic = true) { onClick() }
            .padding(horizontal = 12.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        androidx.compose.material3.Icon(
            painterResource(AppR.drawable.ic_lucide_git_branch),
            null,
            tint = if (b.isCurrent) Pa.Accent else Pa.TextTertiary,
            modifier = Modifier.size(14.dp),
        )
        Spacer(Modifier.width(9.dp))
        Text(
            b.name,
            Modifier.weight(1f),
            fontSize = 13.sp,
            color = if (b.isCurrent) Pa.Accent else Pa.TextPrimary,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        Text(
            if (b.isCurrent) "当前" else if (b.isRemote) "远端" else "切换",
            fontSize = 10.5.sp,
            color = if (b.isCurrent) Pa.Accent else Pa.TextTertiary,
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
        Text("(无差异内容)", fontSize = 12.sp, color = Pa.TextTertiary, modifier = modifier)
        return
    }
    LazyColumn(modifier = modifier) {
        items(lines.size) { i ->
            val line = lines[i]
            val color = when {
                line.startsWith("diff --git") || line.startsWith("index ") ||
                    line.startsWith("--- ") || line.startsWith("+++ ") -> Pa.TextTertiary
                line.startsWith("@@") -> Pa.Accent
                line.startsWith("+") -> Pa.Green
                line.startsWith("-") -> Pa.Red
                else -> Pa.TextSecondary
            }
            Text(
                line.ifEmpty { " " },
                fontSize = 10.sp,
                fontFamily = FontFamily.Monospace,
                color = color,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}
