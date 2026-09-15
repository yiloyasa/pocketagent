# -*- coding: utf-8 -*-
# P4 Android 补丁:启动页、设置路由、终端头部、聊天标题、会话页设置入口
import io

# ---- 1) TerminalScreen:头部与其他页统一(返回钮+标题+状态点),去掉大 TopAppBar ----
p = r'C:\dev\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\TerminalScreen.kt'
s = open(p, encoding='utf-8').read()
s = s.replace('''import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar''',
'''import androidx.compose.material3.Text''')
s = s.replace('''import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding''',
'''import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape''')
s = s.replace('import dev.pocketagent.app.AgentConnection',
'''import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.theme.Pa''')
s = s.replace('''@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TerminalScreen() {
    val bridge = remember { TerminalBridge() }

    DisposableEffect(bridge) {
        bridge.start()
        onDispose { bridge.close() }
    }

    Scaffold(topBar = { TopAppBar(title = { Text("终端") }) }) { pad ->
        AndroidView(
            modifier = Modifier.fillMaxSize().padding(pad),''',
'''@Composable
fun TerminalScreen(onBack: () -> Unit = {}) {
    val bridge = remember { TerminalBridge() }

    DisposableEffect(bridge) {
        bridge.start()
        onDispose { bridge.close() }
    }

    Column(Modifier.fillMaxSize().background(Pa.Bg)) {
        // 头部与会话/设置页统一:返回钮 + 标题 + 状态点
        Row(
            Modifier
                .fillMaxWidth()
                .statusBarsPadding()
                .padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
        ) {
            IconActionButton(AppR.drawable.ic_lucide_chevron_left, "返回", onBack, bg = Pa.Card)
            Spacer(Modifier.width(10.dp))
            Text("终端", Modifier.weight(1f), fontSize = 16.sp, color = Pa.TextPrimary)
            Box(
                Modifier.size(8.dp).background(Pa.Green, CircleShape),
            )
            Spacer(Modifier.width(14.dp))
        }
        AndroidView(
            modifier = Modifier.fillMaxSize(),''')
# 收尾:把原来的 )} 结构对上 —— 原 Scaffold 结尾是 ")\n}" 形式
s = s.replace('''                    loadUrl("file:///android_asset/terminal.html")
                }
            },
        )
    }
}''',
'''                    loadUrl("file:///android_asset/terminal.html")
                }
            },
        )
    }
}''')
open(p, 'w', encoding='utf-8').write(s)
print('TerminalScreen ok')

# ---- 2) MainActivity:启动页 + settings 路由 + 终端返回 + 主页设置入口 ----
p = r'C:\dev\pocketagent\android\app\src\main\java\dev\pocketagent\app\MainActivity.kt'
s = open(p, encoding='utf-8').read()
s = s.replace('import dev.pocketagent.app.ui.HomeScreen',
'''import dev.pocketagent.app.ui.HomeScreen
import dev.pocketagent.app.ui.SettingsScreen
import dev.pocketagent.app.ui.Splash''')
s = s.replace('''        setContent {
            PaTheme {
                Surface(modifier = Modifier.fillMaxSize(), color = Pa.Bg) { RootNav() }
            }
        }''',
'''        setContent {
            PaTheme {
                Surface(modifier = Modifier.fillMaxSize(), color = Pa.Bg) {
                    var showSplash by remember { mutableStateOf(true) }
                    if (showSplash) {
                        Splash(onDone = { showSplash = false })
                    } else {
                        RootNav()
                    }
                }
            }
        }''')
s = s.replace('''                        0 -> HomeScreen(
                            onOpenThread = { id -> nav.navigate("chat/$id") },
                            onOpenTerminal = { nav.navigate("terminal") },
                        )''',
'''                        0 -> HomeScreen(
                            onOpenThread = { id -> nav.navigate("chat/$id") },
                            onOpenTerminal = { nav.navigate("terminal") },
                            onOpenSettings = { nav.navigate("settings") },
                        )''')
s = s.replace('''            composable("terminal") { TerminalScreen() }''',
'''            composable("terminal") { TerminalScreen(onBack = { nav.popBackStack() }) }
            composable("settings") { SettingsScreen(onBack = { nav.popBackStack() }) }''')
open(p, 'w', encoding='utf-8').write(s)
print('MainActivity ok')

# ---- 3) ThreadsScreen:齿轮从抽屉改为设置页路由 ----
p = r'C:\dev\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ThreadsScreen.kt'
s = open(p, encoding='utf-8').read()
s = s.replace('fun ThreadsScreen(onOpenThread: (String) -> Unit, onOpenTerminal: () -> Unit, modifier: Modifier = Modifier) {',
'fun ThreadsScreen(onOpenThread: (String) -> Unit, onOpenTerminal: () -> Unit, onOpenSettings: () -> Unit = {}, modifier: Modifier = Modifier) {')
s = s.replace('IconActionButton(R.drawable.ic_lucide_settings, "设置", { showSettings = true })',
'IconActionButton(R.drawable.ic_lucide_settings, "设置", onOpenSettings)')
# 移除 SettingsSheet 调用与定义(设置已是独立页)
s = s.replace('''    if (showSettings) {
        SettingsSheet(onDismiss = { showSettings = false })
    }
''', '')
# 删掉 SettingsSheet 定义(从 @OptIn 到文件尾的该函数块)
i = s.find('@OptIn(ExperimentalMaterial3Api::class)\n@Composable\nprivate fun SettingsSheet')
if i > 0:
    s = s[:i].rstrip() + '\n'
s = s.replace('    var showSettings by remember { mutableStateOf(false) }\n', '')
open(p, 'w', encoding='utf-8').write(s)
print('ThreadsScreen ok')

# ---- 4) ChatScreen:顶栏显示真实标题 ----
p = r'C:\dev\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ChatScreen.kt'
s = open(p, encoding='utf-8').read()
s = s.replace('''    LaunchedEffect(threadId) {
        try {
            val resp = AgentConnection.threadService().getThreadHistory(''',
'''    var threadTitle by remember { mutableStateOf("任务") }
    LaunchedEffect(threadId) {
        try {
            val t = AgentConnection.threadService().getThread(
                pocketagent.v1.GetThreadRequest.newBuilder().setThreadId(threadId).build())
            if (t.title.isNotEmpty()) threadTitle = t.title
        } catch (_: Exception) {
        }
        try {
            val resp = AgentConnection.threadService().getThreadHistory(''')
s = s.replace('Text("任务", Modifier.weight(1f), fontSize = 16.sp, color = Pa.TextPrimary)',
'Text(threadTitle, Modifier.weight(1f), fontSize = 16.sp, color = Pa.TextPrimary, maxLines = 1)')
open(p, 'w', encoding='utf-8').write(s)
print('ChatScreen ok')
