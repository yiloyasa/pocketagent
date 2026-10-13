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
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
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
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaTheme
import dev.pocketagent.app.ui.theme.editorialM3Scheme
import androidx.core.view.WindowCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch

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
        // 缓存管家:启动时清扫崩溃残留的临时文件(pa_pdf_* / pa_ziplist*)+
        // OfflineStore 陈旧分区清扫。【启动面 T4】这一串是纯磁盘 IO
        // (cacheDir.listFiles + 删除 + 分区扫盘),原先在主线程 onCreate 里跑,
        // 直接卡首帧;清扫结果没有任何 UI 状态同步依赖(缓存管理页要读时它早已
        // 完成),挪到后台线程,onCreate 不再等它。放在 ThreadSnapshot.init 之后
        // 保持"分区注册表晚于各物理目录初始化"的原有顺序语义。
        CoroutineScope(SupervisorJob() + Dispatchers.IO).launch { PaCache.init(this@MainActivity) }
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
                // 【色板并轨 2026-10-09】编辑式 M3 色板在 NavHost 外层一次包全 app:
                // 原 editorialM3Scheme() 只包 5 屏(Browser/Files/Git/Settings/Terminal),
                // Chat/Threads/Home/Collab/TaskCenter 未包 —— 同屏两种黑漂移;现在
                // Splash + RootNav 全树都在编辑式盘内,全 app 的 M3 原生组件
                // (OutlinedTextField/Button/CircularProgressIndicator/Switch/...)
                // 一律吃编辑式槽位;旧 Pa scheme(PaTheme)降为最外层兼容基底。
                androidx.compose.material3.MaterialTheme(colorScheme = editorialM3Scheme()) {
                    Surface(modifier = Modifier.fillMaxSize(), color = PaEditorial.Canvas) {
                        // 【2026-10-09 分发资格批】Splash 从"串行闸"改"覆盖层":
                        // 旧形态 if(showSplash) Splash() else RootNav() 让 RootNav(含首帧
                        // RPC 预取/引擎装配)整整等 Splash ~1.5s 播完才开始组合,冷启动
                        // 白等;现在 RootNav 在 Splash 之下**并行组合**,RPC 预取与启动
                        // 动画同跑,Splash 播完淡出时主页早已就绪,淡出即见真内容。
                        // 时机语义保持:Splash 内部动画/淡出/onDone 链路一字未动,
                        // onDone 仍是"动画播完+淡出结束"的同一个时点,只是消费它的
                        // 动作从"开始组合 RootNav"变为"摘掉覆盖层"。
                        // 覆盖层吃掉点击:动画期间用户点不到尚未可见的主页,与旧
                        // 串行形态"看不见也点不着"一致,避免淡出中误触穿屏。
                        Box(modifier = Modifier.fillMaxSize()) {
                            RootNav()
                            var showSplash by remember { mutableStateOf(true) }
                            if (showSplash) {
                                Box(
                                    modifier = Modifier
                                        .fillMaxSize()
                                        .clickable(
                                            interactionSource = remember { MutableInteractionSource() },
                                            indication = null,
                                        ) {},
                                ) {
                                    Splash(onDone = { showSplash = false })
                                }
                            }
                        }
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

    // 系统栏随显示模式换肤:把色盘状态槽放进键位(读键即订阅),浅/深/黑客任意
    // 切盘都重挂。原 SideEffect 写法读状态但不订阅,切盘后栏色/图标停留在上一盘
    // —— 与 SettingsScreen.EditorialSystemBars(DisposableEffect(PaEditorial.IsDark))
    // 同一机制,两处键控口径一致。
    androidx.compose.runtime.DisposableEffect(PaEditorial.Canvas, PaEditorial.IsDark) {
        val window = (context as? ComponentActivity)?.window
        if (window != null) {
            window.statusBarColor = PaEditorial.Canvas.toArgb()
            window.navigationBarColor = PaEditorial.Canvas.toArgb()
            WindowCompat.getInsetsController(window, view).apply {
                isAppearanceLightStatusBars = !PaEditorial.IsDark
                isAppearanceLightNavigationBars = !PaEditorial.IsDark
            }
        }
        onDispose { }
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
            // 新手教程完成分流(点击时现判,不记 remember):
            // 首启(onboarding 即栈底,无前一页)→新建 home 并弹掉教程;
            // 重看(自主页进入,有前一页)→直接 popBackStack 回主页。
            // 旧写法一律 navigate("home") 会在重看时叠出第二个 home,
            // 系统返回还回到教程页(且未 markDone 下次冷启重进)。
            // 注意:remember 记首启标记是错的 —— RootNav 的 remember 随
            // NavHost 常驻,重看时仍是旧值,故每次点击现查 previousBackStackEntry。
            val finishOnboarding: () -> Unit = {
                OnboardingPrefs.markDone(context)
                if (nav.previousBackStackEntry == null) {
                    nav.navigate("home") { popUpTo("onboarding") { inclusive = true } }
                } else {
                    nav.popBackStack()
                }
            }
            Onboarding(
                onDone = finishOnboarding,
                // 排障页直达:设置 → 连接与配对(深链 sub=connection)。
                onOpenSettings = { nav.navigate("settings?sub=connection") },
                // 收尾页直达:与看完同路(首启去主页、重看回主页)。
                onGoHome = finishOnboarding,
            )
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
                        // 下载任务点卡片本体 → 直达设置里的「下载与存储」(F2 拆页后
                        // 的正名落点;旧参数 userdata 在 SettingsScreen 内兼容映射同页)
                        onOpenUserData = { nav.navigate("settings?sub=downloads") },
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
                // 技能预算警告卡「查看技能与插件」:直达「技能与插件」页(与侧边栏的
                // 设置一级页分口;normalizeSettingsSub 收录 capabilities)
                onOpenSkillsSettings = { nav.navigate("settings?sub=capabilities") },
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
                // 深链二级页:connection / downloads / console / agent / capabilities;
                // 旧参数 userdata 兼容映射为 downloads(SettingsScreen.normalizeSettingsSub,
                // 任务中心下载卡的历史深链不失效),未知值回一级页。
                openSub = entry.arguments?.getString("sub").orEmpty(),
                // 「下载与存储」页里可以直接用内置浏览器打开已下载的网页/文本
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
                // 结论 2(逐成员会话):派发受理后每个成员有自己的子线程,
                // 进度卡「查看会话」直达 chat/<threadId>(与其他入口同一目标路由)。
                onOpenThread = { id -> nav.navigate("chat/$id") },
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

