package dev.pocketagent.app.ui.chat

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import dev.pocketagent.app.ui.ChatScreen

/**
 * ChatScreen 的入口包装(2026-10-04 计划 Task 8,第一层分层)。
 *
 * 【职责】构造 [ChatViewModel] 并传给 ChatScreen —— 页面级长期状态
 * (离线快照/历史加载/身份索引/搜索定位底座)由 ViewModel 持有,Compose 页面
 * 只 collect。对外签名与 ChatScreen 逐参一致,导航层(MainActivity)切到本入口
 * 即完成接入;既有 ChatScreen 直连调用也兼容(ChatScreen 对空 ViewModel 兜底
 * 自建),两入口行为等价。
 *
 * 【threadId 键控】remember(threadId) 跟随导航参数:launchSingleTop 复用同一
 * ChatRoute 实例时,换会话即换 ViewModel —— 页面级状态(含离线快照标志)不跨
 * 会话串线,与迁出前 ChatScreen 里 remember(threadId) 的键控语义一致。
 */
@Composable
fun ChatRoute(
    threadId: String,
    onBack: () -> Unit = {},
    onOpenThread: (String) -> Unit = {},
    onOpenSettings: () -> Unit = {},
    onOpenTerminal: () -> Unit = {},
    onOpenTaskCenter: () -> Unit = {},
    onGoTab: (Int) -> Unit = {},
    /** 打开内置浏览器(带地址);传空串表示只打开浏览器首页。 */
    onOpenBrowser: (String) -> Unit = {},
) {
    val chat = remember(threadId) { ChatViewModel() }
    ChatScreen(
        threadId = threadId,
        onBack = onBack,
        onOpenThread = onOpenThread,
        onOpenSettings = onOpenSettings,
        onOpenTerminal = onOpenTerminal,
        onOpenTaskCenter = onOpenTaskCenter,
        onGoTab = onGoTab,
        onOpenBrowser = onOpenBrowser,
        chat = chat,
    )
}
