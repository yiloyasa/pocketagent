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
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import dev.pocketagent.app.ui.ChatScreen
import dev.pocketagent.app.ui.FilesScreen
import dev.pocketagent.app.ui.GitScreen
import dev.pocketagent.app.ui.HomeScreen
import dev.pocketagent.app.ui.SettingsScreen
import dev.pocketagent.app.ui.Splash
import dev.pocketagent.app.ui.TerminalScreen
import dev.pocketagent.app.ui.ThreadsScreen
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaTheme
import androidx.core.view.WindowCompat

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        AgentConnection.init(this)
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
            Box(Modifier.fillMaxSize()) {
                when (tab) {
                    0 -> HomeScreen(
                        currentTab = 0,
                        onOpenThread = { id -> nav.navigate("chat/$id") },
                        onOpenTerminal = { nav.navigate("terminal") },
                        onOpenSettings = { nav.navigate("settings") },
                        onGoTab = { tab = it },
                    )
                    1 -> ThreadsScreen(
                        currentTab = 1,
                        onOpenThread = { id -> nav.navigate("chat/$id") },
                        onOpenTerminal = { nav.navigate("terminal") },
                        onOpenSettings = { nav.navigate("settings") },
                        onGoTab = { tab = it },
                    )
                    2 -> FilesScreen(
                        initialPath = null,
                        onOpenTerminal = { nav.navigate("terminal") },
                        onOpenSettings = { nav.navigate("settings") },
                        onGoTab = { tab = it },
                    )
                    else -> GitScreen(
                        workspacePath = null,
                        onOpenTerminal = { nav.navigate("terminal") },
                        onOpenSettings = { nav.navigate("settings") },
                        onGoTab = { tab = it },
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
            )
        }
        composable("terminal") { TerminalScreen(onBack = { nav.popBackStack() }) }
        composable("settings") { SettingsScreen(onBack = { nav.popBackStack() }) }
    }
}

