package dev.pocketagent.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import dev.pocketagent.app.ui.BrowserScreen
import dev.pocketagent.app.ui.ChatScreen
import dev.pocketagent.app.ui.FilesScreen
import dev.pocketagent.app.ui.GitScreen
import dev.pocketagent.app.ui.HomeScreen
import dev.pocketagent.app.ui.SettingsScreen
import dev.pocketagent.app.ui.Splash
import dev.pocketagent.app.ui.TaskCenterScreen
import dev.pocketagent.app.ui.TerminalScreen
import dev.pocketagent.app.ui.ThreadsScreen
import dev.pocketagent.app.ui.component.PaCache
import dev.pocketagent.app.ui.component.PaDownloader
import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.component.ThreadPins
import dev.pocketagent.app.ui.component.ThreadSnapshot
import dev.pocketagent.app.ui.component.ThreadUsageStore
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaTheme
import androidx.core.view.WindowCompat

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        AgentConnection.init(this)
        TaskCenter.init(this) // 任务中心:恢复未读数与任务清单(杀进程不丢)
        ThreadUsageStore.init(this) // 会话上下文用量缓存:重开会话时回显
        ThreadPins.init(this) // 会话置顶偏好
        ThreadSnapshot.init(this) // 会话离线快照:断线时列表/最近消息仍可读
        PaDownloader.init(this) // 下载到手机(会话输出/电脑文件)
        PaCache.init(this) // 缓存管家:启动时清扫崩溃残留的临时文件(pa_pdf_* / pa_ziplist*)
        AgentConnection.startAutoReconnect() // 断线自动重连(USB→Tailscale 回退),恢复后全局刷新
        enableEdgeToEdge()
        setContent {
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
        }
    }
}

/**
 * 路由:home(主页/会话/文件/Git 四 tab)+ chat/terminal(全屏)。
 */
@Composable
private fun RootNav() {
    val nav = rememberNavController()
    var tab by remember { mutableIntStateOf(0) }
    // 二级页面按返回时:回到主页并自动展开侧边栏(用户定义的返回层级)
    var openHomeDrawer by remember { mutableIntStateOf(0) }
    val backToHome = {
        tab = 0
        openHomeDrawer += 1
    }
    val context = LocalContext.current
    val view = LocalView.current

    SideEffect {
        val window = (context as? ComponentActivity)?.window ?: return@SideEffect
        window.statusBarColor = Pa.Bg.toArgb()
        window.navigationBarColor = Pa.Bg.toArgb()
        WindowCompat.getInsetsController(window, view).apply {
            isAppearanceLightStatusBars = !Pa.IsDark
            isAppearanceLightNavigationBars = !Pa.IsDark
        }
    }

    NavHost(
        navController = nav,
        startDestination = "home",
        // 【动效】进场慢一点、退场快一点 —— 苹果式转场节奏
        enterTransition = { fadeIn(tween(220)) + slideInHorizontally(tween(220)) { it / 20 } },
        exitTransition = { fadeOut(tween(140)) },
        popEnterTransition = { fadeIn(tween(220)) },
        popExitTransition = { fadeOut(tween(140)) + slideOutHorizontally(tween(140)) { it / 20 } },
    ) {
        composable("home") {
            // 【无底栏】主导航全部收进左上角图标弹出的侧边栏
            val openTaskCenter = { tab = 4 }
            Box(Modifier.fillMaxSize()) {
                when (tab) {
                    0 -> HomeScreen(
                        currentTab = 0,
                        openDrawerSignal = openHomeDrawer,
                        onOpenThread = { id -> nav.navigate("chat/$id") },
                        onOpenTerminal = { nav.navigate("terminal") },
                        onOpenSettings = { nav.navigate("settings") },
                        onOpenTaskCenter = openTaskCenter,
                        onOpenBrowser = { nav.navigate("browser") },
                        onGoTab = { tab = it },
                        onBackToHome = backToHome,
                    )
                    1 -> ThreadsScreen(
                        currentTab = 1,
                        onOpenThread = { id -> nav.navigate("chat/$id") },
                        onOpenTerminal = { nav.navigate("terminal") },
                        onOpenSettings = { nav.navigate("settings") },
                        onOpenTaskCenter = openTaskCenter,
                        onOpenBrowser = { nav.navigate("browser") },
                        onGoTab = { tab = it },
                        onBackToHome = backToHome,
                    )
                    2 -> FilesScreen(
                        initialPath = null,
                        onOpenTerminal = { nav.navigate("terminal") },
                        onOpenSettings = { nav.navigate("settings") },
                        onOpenTaskCenter = openTaskCenter,
                        // 文件浏览器现在把支持预览的文件交给内置浏览器,所以要能带地址
                        onOpenBrowser = { url ->
                            nav.navigate("browser?url=" + android.net.Uri.encode(url))
                        },
                        onGoTab = { tab = it },
                        onBackToHome = backToHome,
                    )
                    3 -> GitScreen(
                        workspacePath = null,
                        onOpenTerminal = { nav.navigate("terminal") },
                        onOpenSettings = { nav.navigate("settings") },
                        onOpenTaskCenter = openTaskCenter,
                        onOpenBrowser = { nav.navigate("browser") },
                        onGoTab = { tab = it },
                        onBackToHome = backToHome,
                    )
                    else -> TaskCenterScreen(
                        currentTab = 4,
                        onOpenThread = { id -> nav.navigate("chat/$id") },
                        onOpenTerminal = { nav.navigate("terminal") },
                        onOpenSettings = { nav.navigate("settings") },
                        onGoTab = { tab = it },
                        // 下载项要用内置浏览器打开,所以这里需要带地址
                        onOpenBrowser = { url ->
                            nav.navigate("browser?url=" + android.net.Uri.encode(url))
                        },
                        // 下载任务点卡片本体 → 直达设置里的「用户数据管理」
                        onOpenUserData = { nav.navigate("settings?sub=userdata") },
                        onBackToHome = backToHome,
                    )
                }
            }
        }
        composable("chat/{threadId}") { entry ->
            ChatScreen(
                threadId = entry.arguments?.getString("threadId") ?: "",
                onBack = { nav.popBackStack() }, // 修复返回键无效
                onOpenThread = { id -> nav.navigate("chat/$id") { launchSingleTop = true } },
                onOpenSettings = { nav.navigate("settings") },
                onOpenTerminal = { nav.navigate("terminal") },
                onOpenTaskCenter = {
                    tab = 4
                    nav.popBackStack("home", inclusive = false)
                },
                onGoTab = { t ->
                    tab = t
                    nav.popBackStack("home", inclusive = false)
                },
                onOpenBrowser = { url ->
                    nav.navigate("browser?url=" + android.net.Uri.encode(url))
                },
            )
        }
        composable("terminal") { TerminalScreen(onBack = { nav.popBackStack() }) }
        // 内置浏览器:会话里的链接与"网页"入口都来这里
        composable(
            route = "browser?url={url}",
            arguments = listOf(navArgument("url") { type = NavType.StringType; defaultValue = "" }),
        ) { entry ->
            BrowserScreen(
                initialUrl = entry.arguments?.getString("url").orEmpty(),
                onBack = { nav.popBackStack() },
            )
        }
        composable(
            route = "settings?sub={sub}",
            arguments = listOf(navArgument("sub") { type = NavType.StringType; defaultValue = "" }),
        ) { entry ->
            SettingsScreen(
                onBack = { nav.popBackStack() },
                // 任务中心点下载卡片 → 直达「用户数据管理」二级页
                openUserData = entry.arguments?.getString("sub") == "userdata",
                // "用户数据管理"里可以直接用内置浏览器打开已下载的网页/文本
                onOpenBrowser = { url ->
                    nav.navigate("browser?url=" + android.net.Uri.encode(url))
                },
            )
        }
    }
}

