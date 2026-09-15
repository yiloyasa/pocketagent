package dev.pocketagent.app.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
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
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.GitDiffRequest
import pocketagent.v1.GitCommitRequest
import pocketagent.v1.GitStatus
import pocketagent.v1.GitStatusRequest

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
    onGoTab: (Int) -> Unit = {},
) {
    var showDrawer by remember { mutableStateOf(false) }
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

    fun refresh() {
        if (repo.isEmpty()) return
        busy = true
        scope.launch {
            try {
                status = withContext(Dispatchers.IO) {
                    AgentConnection.gitService().status(GitStatusRequest.newBuilder().setRepo(repo).build())
                }
                error = null
                notRepo = false
            } catch (e: Exception) {
                val msg = "${e.message}"
                val lower = msg.lowercase()
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
            Image(
                bitmap = agentBitmap(AppR.mipmap.ic_launcher),
                contentDescription = "菜单",
                modifier = Modifier
                    .size(40.dp)
                    .pressable { showDrawer = true },
                contentScale = ContentScale.Crop,
            )
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

        Column(
            Modifier
                .weight(1f)
                .fillMaxWidth()
                .padding(horizontal = 16.dp)
                .navigationBarsPadding(),
        ) {
            error?.let {
                Text(
                    it,
                    Modifier
                        .fillMaxWidth()
                        .background(Pa.Card, RoundedCornerShape(12.dp))
                        .padding(12.dp),
                    color = Pa.Red,
                    fontSize = 13.sp,
                )
                Spacer(Modifier.height(8.dp))
            }

            when {
                notRepo -> EmptyState(
                    AppR.drawable.ic_lucide_git_branch,
                    "这里还不是 Git 仓库",
                    "在电脑上 git init 后,这里可以看状态、diff 和提交",
                )
                status == null && error == null && busy -> {}
                status != null -> {
                    val st = status!!
                    // 分支卡
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
                        }
                        Spacer(Modifier.height(4.dp))
                        Text(
                            "${st.changesList.size} 个变更 · $repo",
                            fontSize = 11.sp,
                            color = Pa.TextTertiary,
                            maxLines = 1,
                        )
                        if (st.changesList.isNotEmpty()) {
                            Spacer(Modifier.height(10.dp))
                            androidx.compose.material3.TextButton(
                                onClick = { showCommit = true },
                                enabled = !busy,
                                modifier = Modifier.align(Alignment.End),
                            ) { Text("提交全部", color = Pa.Accent, fontSize = 13.sp) }
                        }
                    }
                    Spacer(Modifier.height(8.dp))

                    if (st.changesList.isEmpty()) {
                        Text(
                            "工作区干净 ✨",
                            Modifier.padding(16.dp),
                            fontSize = 13.sp,
                            color = Pa.TextSecondary,
                        )
                    }
            LazyColumn(
                modifier = Modifier
                    .weight(1f)
                    .fillMaxWidth(),
                verticalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                        items(st.changesList, key = { it.path }) { c ->
                            Column(
                                Modifier
                                    .fillMaxWidth()
                                    .background(Pa.Card, RoundedCornerShape(12.dp))
                                    .clickable {
                                        scope.launch {
                                            try {
                                                diff = withContext(Dispatchers.IO) {
                                                    AgentConnection.gitService()
                                                        .diff(
                                                            GitDiffRequest.newBuilder()
                                                                .setRepo(repo)
                                                                .setPath(c.path)
                                                                .build(),
                                                        )
                                                        .unifiedDiff
                                                }
                                            } catch (e: Exception) {
                                                error = "diff 失败:${e.message}"
                                            }
                                        }
                                    }
                                    .padding(horizontal = 12.dp, vertical = 10.dp),
                            ) {
                                Text(
                                    c.path,
                                    fontSize = 13.sp,
                                    fontFamily = FontFamily.Monospace,
                                    color = Pa.TextPrimary,
                                )
                                Spacer(Modifier.height(2.dp))
                                Text(
                                    "暂存:${c.stagedStatus.ifEmpty { "-" }} · 工作区:${c.worktreeStatus.ifEmpty { "-" }}",
                                    fontSize = 10.sp,
                                    color = Pa.TextSecondary,
                                )
                            }
                        }
                    }
                }
            }
        }
    }

    // diff 查看器
    diff?.let { d ->
        AlertDialog(
            onDismissRequest = { diff = null },
            title = { Text("变更 (git diff)", fontSize = 14.sp) },
            text = {
                Text(
                    d.take(30000).ifEmpty { "(无未暂存变更)" },
                    fontSize = 10.sp,
                    fontFamily = FontFamily.Monospace,
                    modifier = Modifier.height(420.dp),
                )
            },
            confirmButton = { TextButton(onClick = { diff = null }) { Text("关闭") } },
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
    if (showRepoPicker) {
        AlertDialog(
            onDismissRequest = { showRepoPicker = false },
            title = { Text("切换 Git 仓库", fontSize = 15.sp) },
            text = {
                Column {
                    if (candidates.isEmpty()) {
                        Text("没有已知的工作区目录", fontSize = 12.sp, color = Pa.TextTertiary)
                    }
                    candidates.forEach { c ->
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .pressable {
                                    repo = c
                                    showRepoPicker = false
                                    refresh()
                                }
                                .padding(vertical = 9.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Text(
                                c,
                                Modifier.weight(1f),
                                fontSize = 12.sp,
                                fontFamily = FontFamily.Monospace,
                                color = if (c == repo) Pa.Accent else Pa.TextPrimary,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            if (c == repo) {
                                Text("当前", fontSize = 10.sp, color = Pa.Accent)
                            }
                        }
                    }
                    Spacer(Modifier.height(10.dp))
                    OutlinedTextField(
                        repoInput, { repoInput = it },
                        label = { Text("或手填目录路径") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    TextButton(onClick = {
                        val v = repoInput.trim()
                        if (v.isNotBlank()) {
                            repo = v
                            showRepoPicker = false
                            refresh()
                        }
                    }) { Text("使用该目录", color = Pa.Accent) }
                }
            },
            confirmButton = {
                TextButton(onClick = { showRepoPicker = false }) { Text("关闭") }
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
        )
    }
}
