package dev.pocketagent.app

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import dev.pocketagent.app.ui.anim.PaMotion
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
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.collectAsState
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
import dev.pocketagent.app.ui.CollabRoleEditorScreen
import dev.pocketagent.app.ui.CollabRolesScreen
import dev.pocketagent.app.ui.CollabScreen
import dev.pocketagent.app.ui.FilesScreen
import dev.pocketagent.app.ui.GitScreen
import dev.pocketagent.app.ui.HomeScreen
import dev.pocketagent.app.ui.Onboarding
import dev.pocketagent.app.ui.OnboardingPrefs
import dev.pocketagent.app.ui.SettingsScreen
import dev.pocketagent.app.ui.Splash
import dev.pocketagent.app.ui.TaskCenterScreen
import dev.pocketagent.app.ui.TerminalScreen
import dev.pocketagent.app.ui.ThreadsScreen
import dev.pocketagent.app.ui.component.PaCache
import dev.pocketagent.app.ui.component.PaDownloader
import dev.pocketagent.app.ui.component.PaNotifications
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
        dev.pocketagent.app.ui.component.TurnCheckpointStore.init(this)
        dev.pocketagent.app.ui.TurnSession.restoreRunningCheckpoints()
        ThreadSnapshot.init(this) // 会话离线快照:断线时列表/最近消息仍可读
        PaDownloader.init(this) // 下载到手机(会话输出/电脑文件)
        PaCache.init(this) // 缓存管家:启动时清扫崩溃残留的临时文件(pa_pdf_* / pa_ziplist*)
        // 【P1-6】本地通知:建渠道 + 动态注册审批动作接收器 + 启动逐秒监控。
        // 必须在 TaskCenter.init 之后(种子要读任务清单)。
        PaNotifications.init(this)
        // 【P1-6】通知深链:冷启动带 extra(thread_id)时记录待导航会话,
        // RootNav 组合后消费;热启动(复用实例)走 onNewIntent 同一入口。
        PaNotifications.onLaunchIntent(intent)
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

    // ---- P1-6:通知深链热启动(通知点击时 app 还活着 → 复用本实例) ----
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        PaNotifications.onLaunchIntent(intent)
    }

    // ---- P1-6:前台标志(通知"任务完成"只在后台发)+ 通知运行时权限申请 ----
    override fun onResume() {
        super.onResume()
        PaNotifications.appInForeground = true
        PaNotifications.maybeRequestNotificationPermission(this)
    }

    override fun onPause() {
        super.onPause()
        PaNotifications.appInForeground = false
    }

    // P1-6:通知权限申请结果刻意不做任何处理 —— ComponentActivity 已把
    // onRequestPermissionsResult 收进 ActivityResultRegistry(不可再 override),
    // 默认行为即"静默跳过":用户拒绝后 areNotificationsEnabled() 为 false,
    // 发通知侧会直接跳过,想开再去系统设置。
}

/**
 * 路由:home(主页/会话/文件/Git 四 tab)+ chat/terminal(全屏)。
 */
@Composable
private fun RootNav() {
    val nav = rememberNavController()
    val context = LocalContext.current
    // 首启检测:未看过新手教程则从 onboarding 起步,看完切回 home
    val startDestination = remember {
        if (OnboardingPrefs.isDone(context)) "home" else "onboarding"
    }
    var tab by remember { mutableIntStateOf(0) }
    // 二级页面按返回时:回到主页并自动展开侧边栏(用户定义的返回层级)
    var openHomeDrawer by remember { mutableIntStateOf(0) }
    val backToHome = {
        tab = 0
        openHomeDrawer += 1
    }
    val view = LocalView.current

    // 【P1-6】通知深链:通知点进来自动打开对应会话。onCreate 冷启动与
    // onNewIntent 热启动都汇到 PaNotifications.pendingChatNavigation,
    // 这里统一消费后清空,避免重组时重复导航。
    val pendingChat by PaNotifications.pendingChatNavigation.collectAsState()
    LaunchedEffect(pendingChat) {
        val tid = pendingChat ?: return@LaunchedEffect
        if (tid.isNotBlank()) {
            nav.navigate("chat/$tid") { launchSingleTop = true }
        }
        PaNotifications.clearPendingChatNavigation()
    }

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
        startDestination = startDestination,
        // 【动效】spring 化的页面转场:tween 是"播完为止"的固定时长,连点两次
        // 会跳帧;spring 从当前值续走,打断永远平滑(可中断性 = 丝滑的根基)。
        // 进场带 1/20 屏宽的滑入 + 淡入(空间连续性:去哪从哪来,对称路径)。
        enterTransition = { fadeIn(PaMotion.nav()) + slideInHorizontally(PaMotion.nav()) { it / 20 } },
        exitTransition = { fadeOut(PaMotion.nav(900f)) },
        popEnterTransition = { fadeIn(PaMotion.nav()) },
        popExitTransition = { fadeOut(PaMotion.nav(900f)) + slideOutHorizontally(PaMotion.nav(900f)) { it / 20 } },
    ) {
        composable("onboarding") {
            // 新手教程:看完写标志并切回主页(无返回栈残留)
            Onboarding(onDone = {
                OnboardingPrefs.markDone(context)
                nav.navigate("home") { popUpTo("onboarding") { inclusive = true } }
            })
        }
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
                        onOpenCollabRoles = { nav.navigate("collabRoles") },
                        onOpenCollab = { nav.navigate("collab") },
                        onGoTab = { tab = it },
                        onBackToHome = backToHome,
                        onOpenOnboarding = { nav.navigate("onboarding") },
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
        // ---- 协同扇出主页:一条指令派给多个对等 agent ----
        // 【为什么必须补这条路由】CollabScreen.kt 早已实现(437 行完整编排台:
        // agent 多选、相位卡、角色弹层、共享指令输入),但全 android/app/src 内
        // **零调用方** —— 用户从任何入口都进不去,能力等于不存在。方案 §4.8 说的
        // "主导航缺少完整 route" 就是这里。
        composable("collab") {
            CollabScreen(
                onBack = { nav.popBackStack() },
                // 编排台里的"管理岗位"再进角色列表页,与主页入口同一条路由。
                onOpenCollabRoles = { nav.navigate("collabRoles") },
            )
        }
        // ---- 协同角色配置:列表页 ----
        composable("collabRoles") {
            CollabRolesScreen(
                onBack = { nav.popBackStack() },
                onOpenEditor = { roleId -> nav.navigate("collabRoleEditor/$roleId") },
            )
        }
        // ---- 协同角色编辑页(roleId 为空 = 新建) ----
        composable(
            route = "collabRoleEditor/{roleId}",
            arguments = listOf(navArgument("roleId") { type = NavType.StringType; defaultValue = "" }),
        ) { entry ->
            CollabRoleEditorScreen(
                roleId = entry.arguments?.getString("roleId").orEmpty(),
                onBack = { nav.popBackStack() },
            )
        }
    }
}

