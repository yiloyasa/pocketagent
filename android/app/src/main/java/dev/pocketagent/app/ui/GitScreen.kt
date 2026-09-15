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
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.EmptyState
import dev.pocketagent.app.ui.anim.IconActionButton
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
 * Git 面板:状态 → diff → 提交,全部走 daemon 的 GitService(子进程调 git)。
 * 与其他页统一:纯黑背景 + 深灰圆角卡 + 细字标题头部。
 */
@Composable
fun GitScreen(
    workspacePath: String?,
    currentTab: Int = 3,
    onOpenTerminal: () -> Unit = {},
    onOpenSettings: () -> Unit = {},
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
                // 非仓库是常态而不是故障:给出友好空态,不刷红字
                if (msg.contains("not a git repository")) notRepo = true else error = msg
            } finally {
                busy = false
            }
        }
    }

    LaunchedEffect(Unit) {
        if (repo.isEmpty()) {
            try {
                val info = withContext(Dispatchers.IO) {
                    AgentConnection.hostService().getHostInfo(GetHostInfoRequest.getDefaultInstance())
                }
                repo = info.workspacesList.firstOrNull().orEmpty()
            } catch (e: Exception) {
                error = "连不上 daemon:${e.message}"
            }
        }
        refresh()
    }

    Column(Modifier.fillMaxSize().background(Pa.Bg)) {
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
            IconActionButton(AppR.drawable.ic_lucide_refresh_cw, "刷新", { refresh() }, bg = Pa.Card)
        }

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

    if (showDrawer) {
        AppDrawer(
            currentTab = currentTab,
            hostTitle = "Git",
            onClose = { showDrawer = false },
            onGoTab = { onGoTab(it) },
            onOpenTerminal = { onOpenTerminal() },
            onOpenSettings = { onOpenSettings() },
        )
    }
}
