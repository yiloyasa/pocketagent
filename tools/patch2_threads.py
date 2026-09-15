# -*- coding: utf-8 -*-
# P3 补丁 2:ThreadsScreen(归档过滤 + 长按菜单 + 重命名)
p = r'C:\dev\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ThreadsScreen.kt'
s = open(p, encoding='utf-8').read()

# 状态变量
s = s.replace('''    var showModels by remember { mutableStateOf(false) }
    var models by remember { mutableStateOf(listOf<ModelInfo>()) }''',
'''    var showModels by remember { mutableStateOf(false) }
    var models by remember { mutableStateOf(listOf<ModelInfo>()) }
    var showArchived by remember { mutableStateOf(false) }
    var actionThread by remember { mutableStateOf<PThread?>(null) }
    var renameThread by remember { mutableStateOf<PThread?>(null) }
    var renameText by remember { mutableStateOf("") }''')

# 列表加载带归档过滤
s = s.replace('''                threads = AgentConnection.threadService()
                    .listThreads(ListThreadsRequest.newBuilder().build())
                    .threadsList''',
'''                threads = AgentConnection.threadService()
                    .listThreads(
                        ListThreadsRequest.newBuilder()
                            .setIncludeArchived(showArchived)
                            .build())
                    .threadsList''')
s = s.replace('    LaunchedEffect(Unit) { refresh() }',
              '    LaunchedEffect(Unit, showArchived) { refresh() }')

# 过滤 chips
s = s.replace('''            // ---- 会话列表 ----
            if (threads.isEmpty()) {''',
'''            // ---- 过滤 chips:活跃 / 已归档 ----
            Row(Modifier.padding(start = 16.dp, end = 16.dp, top = 2.dp)) {
                FilterChip2("活跃", !showArchived) { showArchived = false }
                Spacer(Modifier.width(8.dp))
                FilterChip2("已归档", showArchived) { showArchived = true }
            }

            // ---- 会话列表 ----
            if (threads.isEmpty()) {''')

# 长按
s = s.replace('''                    items(threads, key = { it.id }) { t ->
                        ThreadCard(t) { onOpenThread(t.id) }
                    }''',
'''                    items(threads, key = { it.id }) { t ->
                        ThreadCard(t, onClick = { onOpenThread(t.id) }, onLongPress = { actionThread = t })
                    }''')

# 空列表文案随过滤变化
s = s.replace('''                EmptyState(
                    iconRes = R.drawable.ic_lucide_list_checks,
                    title = "还没有会话",
                    hint = "点右下角 + 开始你的第一个任务",
                )''',
'''                EmptyState(
                    iconRes = R.drawable.ic_lucide_archive,
                    title = if (showArchived) "没有归档的会话" else "还没有会话",
                    hint = if (showArchived) "长按会话可选择归档" else "点右下角 + 开始你的第一个任务",
                )''')

# ThreadCard 长按参数
s = s.replace('''@Composable
private fun ThreadCard(t: PThread, onClick: () -> Unit) {
    Column(
        Modifier
            .fillMaxWidth()
            .pressable { onClick() }''',
'''@Composable
private fun ThreadCard(t: PThread, onClick: () -> Unit, onLongPress: () -> Unit) {
    Column(
        Modifier
            .fillMaxWidth()
            .pressableLong(onClick = onClick, onLongPress = onLongPress)''')

# 归档徽标(卡片内,已归档列表中显示)
s = s.replace('''        Text(
            "${t.messageCount} 条消息 · " +
                java.text.SimpleDateFormat("MM-dd HH:mm").format(java.util.Date(t.updatedAtUnixMs)),
            fontSize = 12.sp,
            color = Pa.TextSecondary,
            modifier = Modifier.padding(top = 4.dp),
        )''',
'''        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(top = 4.dp)) {
            if (t.status == pocketagent.v1.ThreadStatus.THREAD_STATUS_ARCHIVED) {
                Text("已归档", fontSize = 10.sp, color = Pa.Accent)
                Text("  ·  ", fontSize = 10.sp, color = Pa.TextTertiary)
            }
            Text(
                "${t.messageCount} 条消息 · " +
                    java.text.SimpleDateFormat("MM-dd HH:mm").format(java.util.Date(t.updatedAtUnixMs)),
                fontSize = 12.sp,
                color = Pa.TextSecondary,
            )
        }''')

# 长按菜单 + 重命名对话框(插在 if (showModels) 前)
s = s.replace('''    if (showModels) {''',
'''    // 长按操作菜单
    actionThread?.let { t ->
        ModalBottomSheet(
            onDismissRequest = { actionThread = null },
            containerColor = Pa.CardHigh,
            shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
        ) {
            Text(
                t.title.ifEmpty { "(未命名)" },
                Modifier.padding(horizontal = 24.dp, vertical = 6.dp),
                fontSize = 16.sp, color = Pa.TextPrimary, maxLines = 1,
            )
            Spacer(Modifier.height(6.dp))
            SheetAction(R.drawable.ic_lucide_pencil, "重命名") {
                renameText = t.title
                renameThread = t
                actionThread = null
            }
            SheetAction(
                R.drawable.ic_lucide_archive,
                if (t.status == pocketagent.v1.ThreadStatus.THREAD_STATUS_ARCHIVED) "恢复会话" else "归档",
            ) {
                scope.launch {
                    withContext(Dispatchers.IO) {
                        AgentConnection.threadService().archiveThread(
                            pocketagent.v1.ArchiveThreadRequest.newBuilder()
                                .setThreadId(t.id)
                                .setArchived(t.status != pocketagent.v1.ThreadStatus.THREAD_STATUS_ARCHIVED)
                                .build())
                    }
                    refresh()
                }
                actionThread = null
            }
            SheetAction(R.drawable.ic_lucide_trash_2, "删除", tint = Pa.Red) {
                scope.launch {
                    withContext(Dispatchers.IO) {
                        AgentConnection.threadService().deleteThread(
                            pocketagent.v1.DeleteThreadRequest.newBuilder().setThreadId(t.id).build())
                    }
                    refresh()
                }
                actionThread = null
            }
            Spacer(Modifier.height(30.dp))
        }
    }

    // 重命名对话框
    renameThread?.let { t ->
        androidx.compose.material3.AlertDialog(
            onDismissRequest = { renameThread = null },
            title = { Text("重命名会话", fontSize = 17.sp) },
            text = {
                OutlinedTextField(renameText, { renameText = it }, singleLine = true, modifier = Modifier.fillMaxWidth())
            },
            confirmButton = {
                TextButton(onClick = {
                    scope.launch {
                        withContext(Dispatchers.IO) {
                            AgentConnection.threadService().renameThread(
                                pocketagent.v1.RenameThreadRequest.newBuilder()
                                    .setThreadId(t.id).setTitle(renameText.trim()).build())
                        }
                        renameThread = null
                        refresh()
                    }
                }) { Text("保存", color = Pa.Accent) }
            },
            dismissButton = { TextButton(onClick = { renameThread = null }) { Text("取消") } },
        )
    }

    if (showModels) {''')

# 底部追加 FilterChip2 / SheetAction 组件 + import
s = s.replace('import dev.pocketagent.app.ui.anim.pressable\n',
              'import dev.pocketagent.app.ui.anim.pressable\nimport dev.pocketagent.app.ui.anim.pressableLong\n')

s = s.rstrip() + '''

/** 小过滤 chip(活跃/已归档)。 */
@Composable
fun FilterChip2(label: String, selected: Boolean, onClick: () -> Unit) {
    Box(
        Modifier
            .pressable { onClick() }
            .background(
                if (selected) Pa.AccentDim else Pa.Card,
                RoundedCornerShape(50),
            )
            .padding(horizontal = 16.dp, vertical = 7.dp),
    ) {
        Text(label, fontSize = 13.sp, color = if (selected) Pa.Accent else Pa.TextSecondary)
    }
}

/** 抽屉操作行:图标 + 文案(重命名/归档/删除)。 */
@Composable
fun SheetAction(iconRes: Int, label: String, tint: Color = Pa.TextPrimary, onClick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .pressable { onClick() }
            .padding(horizontal = 24.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(painterResource(iconRes), null, tint = tint, modifier = Modifier.size(20.dp))
        Spacer(Modifier.width(14.dp))
        Text(label, fontSize = 15.sp, color = tint)
    }
}
'''
open(p, 'w', encoding='utf-8').write(s)
print('ThreadsScreen ok')
