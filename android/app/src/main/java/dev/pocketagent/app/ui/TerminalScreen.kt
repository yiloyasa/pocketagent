@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package dev.pocketagent.app.ui

import android.annotation.SuppressLint
import android.util.Base64
import android.webkit.JavascriptInterface
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.res.painterResource
import dev.pocketagent.app.ui.anim.pressable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.theme.Pa
import pocketagent.v1.OpenTerminal
import pocketagent.v1.TerminalClientMessage
import pocketagent.v1.TerminalInput
import pocketagent.v1.TerminalResize
import pocketagent.v1.TerminalServerMessage
import java.util.UUID
import java.util.concurrent.LinkedBlockingQueue
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.withContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.CloseTerminalRequest
import pocketagent.v1.ListDirectoryRequest

/**
 * 终端页:WebView 里跑 xterm.js,和 daemon 的 TerminalService 双向流对接。
 *
 * 【讲解】数据流:按键 → JS 桥(AndroidTerm.onKey)→ gRPC Input →
 * 电脑上的 PTY → Output(ANSI 字节)→ base64 → JS termWrite() → 上屏。
 * base64 编码是为了绕开 JS 字符串转义——终端字节流里什么控制字符都有。
 */
@Composable
fun TerminalScreen(onBack: () -> Unit = {}) {
    // 工作目录:先从当前有效目录/最近工作区中解析,找不到时使用 daemon 默认目录。
    // 不能直接取最近一项:历史记录可能指向已删除或已卸载的盘符。
    // 切换 = 换 sessionKey 整体重建(旧 PTY 关闭,新 PTY 在新目录起 shell)。
    var cwd by rememberSaveable { mutableStateOf("") }
    var sessionKey by rememberSaveable { mutableIntStateOf(0) }
    var terminalReady by remember { mutableStateOf(false) }
    var showDirPicker by remember { mutableStateOf(false) }
    var showBrowser by remember { mutableStateOf(false) }
    var switching by remember { mutableStateOf(false) }
    var switchError by remember { mutableStateOf<String?>(null) }
    var availableRecents by remember { mutableStateOf(emptyList<String>()) }
    val recents by AgentConnection.recentWorkspaces.collectAsState()
    val scope = androidx.compose.runtime.rememberCoroutineScope()
    val bridge = androidx.compose.runtime.key(sessionKey, cwd) {
        remember(sessionKey, cwd) { TerminalBridge(cwd) }
    }
    val terminalState by bridge.state.collectAsState()
    val terminalError by bridge.error.collectAsState()

    LaunchedEffect(Unit) {
        val candidates = buildList {
            if (cwd.isNotBlank()) add(cwd)
            addAll(AgentConnection.recentWorkspaces.value)
            val hostWorkspace = runCatching {
                withContext(Dispatchers.IO) {
                    AgentConnection.hostService()
                        .getHostInfo(GetHostInfoRequest.getDefaultInstance())
                }.workspacesList.firstOrNull().orEmpty()
            }.getOrDefault("")
            if (hostWorkspace.isNotBlank()) add(hostWorkspace)
        }.distinct()

        // 用文件服务确认目录仍然存在,避免过期 recent workspace 直接把终端启动到错误目录。
        val valid = withContext(Dispatchers.IO) {
            candidates.firstOrNull { candidate ->
                runCatching {
                    AgentConnection.fileService()
                        .listDirectory(
                            ListDirectoryRequest.newBuilder().setPath(candidate).build(),
                        )
                    true
                }.getOrDefault(false)
            }
        }
        if (valid != null) {
            cwd = valid
        }
        terminalReady = true
    }

    LaunchedEffect(showDirPicker, recents) {
        if (!showDirPicker) return@LaunchedEffect
        availableRecents = withContext(Dispatchers.IO) {
            recents.filter { candidate ->
                runCatching {
                    AgentConnection.fileService()
                        .listDirectory(
                            ListDirectoryRequest.newBuilder().setPath(candidate).build(),
                        )
                    true
                }.getOrDefault(false)
            }
        }
    }

    fun switchDirectory(next: String) {
        if (switching) return
        if (next == cwd) {
            showDirPicker = false
            showBrowser = false
            return
        }
        switching = true
        switchError = null
        scope.launch {
            val valid = next.isBlank() || withContext(Dispatchers.IO) {
                runCatching {
                    AgentConnection.fileService()
                        .listDirectory(
                            ListDirectoryRequest.newBuilder().setPath(next).build(),
                        )
                    true
                }.getOrDefault(false)
            }
            if (valid) {
                if (next.isNotBlank()) AgentConnection.addRecentWorkspace(next)
                cwd = next
                sessionKey++
                showDirPicker = false
                showBrowser = false
            } else {
                switchError = "目录不可用，原终端仍保持不变：$next"
            }
            switching = false
        }
    }

    Column(Modifier.fillMaxSize().background(Pa.Bg)) {
        // 头部与会话/设置页统一:返回钮 + 标题 + 目录 + 状态点
        Row(
            Modifier
                .fillMaxWidth()
                .statusBarsPadding()
                .padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
        ) {
            IconActionButton(AppR.drawable.ic_lucide_chevron_left, "返回", onBack, bg = Pa.Card)
            Spacer(Modifier.width(10.dp))
            Column(Modifier.weight(1f)) {
                Text("终端", fontSize = 16.sp, color = Pa.TextPrimary)
                Text(
                    cwd.ifBlank { "默认目录(点右侧文件夹切换)" },
                    fontSize = 10.5.sp, color = Pa.TextTertiary, maxLines = 1,
                )
                Text(
                    when {
                        !terminalReady -> "正在准备终端…"
                        terminalState == "connected" -> "终端已连接"
                        terminalState == "error" -> terminalError ?: "终端连接失败"
                        terminalState == "closed" -> "终端已关闭"
                        else -> "正在连接终端…"
                    },
                    fontSize = 10.5.sp,
                    color = when {
                        !terminalReady -> Pa.TextTertiary
                        terminalState == "connected" -> Pa.Green
                        terminalState == "error" -> Pa.Red
                        else -> Pa.TextTertiary
                    },
                    maxLines = 1,
                )
            }
            IconActionButton(
                AppR.drawable.ic_lucide_folder, "切换目录",
                { showDirPicker = true }, bg = Pa.Card,
            )
            Spacer(Modifier.width(8.dp))
            Box(
                Modifier.size(8.dp).background(
                    when {
                        !terminalReady -> Pa.TextTertiary
                        terminalState == "connected" -> Pa.Green
                        terminalState == "error" -> Pa.Red
                        else -> Pa.TextTertiary
                    },
                    CircleShape,
                ),
            )
            Spacer(Modifier.width(14.dp))
        }
        // sessionKey 必须同时重建 WebView。只重建 gRPC bridge 会留下旧的
        // JavaScript 回调，切换目录后按键和输出仍然会发给旧 PTY。
        if (!terminalReady) {
            Box(Modifier.fillMaxSize(), contentAlignment = androidx.compose.ui.Alignment.Center) {
                Text("正在准备终端…", fontSize = 13.sp, color = Pa.TextTertiary)
            }
        } else {
            DisposableEffect(bridge) {
                bridge.start()
                onDispose { bridge.close() }
            }
            // AndroidView 默认会复用同一个 WebView；目录切换时必须连同页面一起换掉，
            // 否则标题虽然已经更新，xterm 仍会显示旧 PTY 的提示符和历史输出。
            androidx.compose.runtime.key(sessionKey, cwd) {
                AndroidView(
                    modifier = Modifier.fillMaxSize(),
                    factory = { ctx ->
                        WebView(ctx).apply {
                            @SuppressLint("SetJavaScriptEnabled")
                            settings.javaScriptEnabled = true
                            webViewClient = WebViewClient()
                            addJavascriptInterface(
                                TerminalJsBridge(
                                    onKey = { bridge.sendInput(it) },
                                    onReady = { bridge.onPageReady() },
                                    onResize = { c, r -> bridge.sendResize(c, r) },
                                    onReport = { android.util.Log.i("PaTerm", "PAGE $it") },
                                ),
                                "AndroidTerm",
                            )
                            bridge.bindWebView(this)
                            loadUrl("file:///android_asset/terminal.html")
                        }
                    },
                )
            }
        }
    }

    // 目录选择抽屉:默认 / 最近工作区 / 手动输入;选定即重建终端会话
    if (showDirPicker) {
        androidx.compose.material3.ModalBottomSheet(
            onDismissRequest = { showDirPicker = false },
            containerColor = Pa.CardHigh,
            shape = androidx.compose.foundation.shape.RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
        ) {
            Text(
                "切换工作目录", Modifier.fillMaxWidth().padding(horizontal = 24.dp),
                fontSize = 17.sp, color = Pa.TextPrimary,
            )
            Text(
                "新终端将在所选目录启动(旧会话关闭)",
                Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 4.dp),
                fontSize = 12.sp, color = Pa.TextSecondary,
            )
            if (switching) {
                Text(
                    "正在切换目录…",
                    Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 4.dp),
                    fontSize = 12.sp, color = Pa.Accent,
                )
            }
            switchError?.let {
                Text(
                    it,
                    Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 4.dp),
                    fontSize = 12.sp, color = Pa.Red,
                )
            }
            Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp)) {
                (listOf("") + availableRecents).distinct().forEach { dir ->
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .pressable(enabled = !switching) { switchDirectory(dir) }
                            .background(Pa.Card, RoundedCornerShape(12.dp))
                            .padding(horizontal = 12.dp, vertical = 10.dp),
                        verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
                    ) {
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_folder), null,
                            tint = Pa.Accent, modifier = Modifier.size(16.dp),
                        )
                        Spacer(Modifier.width(10.dp))
                        Text(
                            dir.ifBlank { "默认目录" },
                            fontSize = 13.sp, color = Pa.TextPrimary, maxLines = 1,
                        )
                    }
                    Spacer(Modifier.height(6.dp))
                }
                // 文件式浏览电脑选文件夹
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable(enabled = !switching) { showBrowser = true }
                        .background(Pa.Card, RoundedCornerShape(12.dp))
                        .padding(horizontal = 12.dp, vertical = 12.dp),
                    verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_monitor), null,
                        tint = Pa.Accent, modifier = Modifier.size(16.dp),
                    )
                    Spacer(Modifier.width(10.dp))
                    Text("浏览电脑文件夹…", fontSize = 13.sp, color = Pa.Accent)
                }
                Spacer(Modifier.height(18.dp))
            }
        }
        if (showBrowser) {
            FolderPickerSheet(
                onDismiss = { showBrowser = false },
                onPick = {
                    showBrowser = false
                    showDirPicker = false
                    cwd = it
                    sessionKey++
                },
            )
        }
    }
}

/** JS → Kotlin 的桥:xterm.js 的 onData(按键序列)从这里进 gRPC。 */
private class TerminalJsBridge(
    private val onKey: (String) -> Unit,
    private val onReady: () -> Unit,
    private val onResize: (Int, Int) -> Unit = { _, _ -> },
    private val onReport: (String) -> Unit = {},
) {
    @JavascriptInterface
    fun onKey(data: String) { onKey.invoke(data) }

    @JavascriptInterface
    fun ready() { onReady.invoke() }

    @JavascriptInterface
    fun resize(cols: Int, rows: Int) { onResize.invoke(cols, rows) }

    @JavascriptInterface
    fun report(info: String) { onReport.invoke(info) }
}

/**
 * TerminalBridge:gRPC 双向流生命周期管理。
 * 【讲解】朴素实现:一个发送队列 + 一个写线程。gRPC 观察者不允许并发写,
 * 队列把 WebView JS 线程和 IO 线程解耦。
 */
class TerminalBridge(private val cwd: String = "") {
    private val sessionId = UUID.randomUUID().toString().substring(0, 8)
    private val sendQueue = LinkedBlockingQueue<TerminalClientMessage>()
    private val cols = 80
    private val rows = 24

    /** 页面就绪前的输出先攒在这里,ready 后一次性 flush(否则提示符会在 xterm 初始化前丢失)。 */
    private val pendingOutput = java.util.concurrent.ConcurrentLinkedQueue<String>()
    private val mainHandler = android.os.Handler(android.os.Looper.getMainLooper())
    @Volatile private var terminalWebView: WebView? = null
    @Volatile private var pageReady = false
    val state = MutableStateFlow("connecting")
    val error = MutableStateFlow<String?>(null)

    fun bindWebView(view: WebView) {
        terminalWebView = view
    }

    fun onPageReady() {
        android.util.Log.i("PaTerm", "page ready, pending=${pendingOutput.size}")
        pageReady = true
        val flush = ArrayList<String>()
        while (true) {
            val b64 = pendingOutput.poll() ?: break
            flush.add(b64)
        }
        if (flush.isNotEmpty()) {
            mainHandler.post {
                flush.forEach(::evalWrite)
            }
        }
    }

    private fun evalWrite(b64: String) {
        terminalWebView?.evaluateJavascript("window.termWrite('$b64')", null)
    }

    private fun errorLabel(t: Throwable): String {
        val raw = t.message.orEmpty()
        return when {
            raw.contains("EOF", ignoreCase = true) ->
                "终端连接已结束，请重新连接"
            raw.contains("UNAVAILABLE", ignoreCase = true) ->
                "电脑连接已断开，正在等待重连"
            raw.isBlank() -> "终端连接失败"
            else -> raw
        }
    }

    @Volatile private var closed = false
    @Volatile private var started = false
    @Volatile private var requestObserver: io.grpc.stub.StreamObserver<TerminalClientMessage>? = null

    fun sendInput(data: String) {
        offer(
            TerminalClientMessage.newBuilder()
                .setInput(TerminalInput.newBuilder().setData(com.google.protobuf.ByteString.copyFromUtf8(data)))
                .build()
        )
    }

    fun sendResize(c: Int = cols, r: Int = rows) {
        offer(
            TerminalClientMessage.newBuilder()
                .setResize(TerminalResize.newBuilder().setCols(c).setRows(r))
                .build()
        )
    }

    private fun offer(msg: TerminalClientMessage) {
        if (!closed) sendQueue.offer(msg)
    }

    @Synchronized
    fun start() {
        if (started || closed) return
        started = true
        active = this
        state.value = "connecting"
        error.value = null
        val stub = AgentConnection.terminalServiceAsync()

        val responseObserver = object : io.grpc.stub.StreamObserver<TerminalServerMessage> {
            override fun onNext(value: TerminalServerMessage) {
                when (value.msgCase) {
                    TerminalServerMessage.MsgCase.OPENED -> {
                        state.value = "connected"
                    }
                    TerminalServerMessage.MsgCase.OUTPUT -> {
                        val b64 = Base64.encodeToString(value.output.data.toByteArray(), Base64.NO_WRAP)
                        android.util.Log.i("PaTerm", "output ${value.output.data.size()}B ready=$pageReady")
                        if (pageReady) {
                            mainHandler.post { evalWrite(b64) }
                        } else {
                            pendingOutput.offer(b64)
                        }
                    }
                    TerminalServerMessage.MsgCase.EXIT -> {
                        state.value = "closed"
                        close()
                    }
                    TerminalServerMessage.MsgCase.ERROR -> {
                        error.value = value.error.message.ifBlank { "终端服务返回错误" }
                        state.value = "error"
                        close()
                    }
                    else -> {}
                }
            }
            override fun onError(t: Throwable) {
                if (closed) return
                // 切换目录/离开页面时，旧流的关闭回调可能在后台晚到；
                // 旧 bridge 已标记 closed，不应再把正常收尾记录为错误。
                android.util.Log.e("PaTerm", "stream error", t)
                error.value = errorLabel(t)
                state.value = "error"
                close()
            }
            override fun onCompleted() {
                if (!closed) state.value = "closed"
                close()
            }
        }

        val ro = stub.attach(responseObserver)
        requestObserver = ro
        android.util.Log.i("PaTerm", "attach sent, sessionId=$sessionId")

        Thread {
            try {
                ro.onNext(
                    TerminalClientMessage.newBuilder()
                        .setOpen(
                            OpenTerminal.newBuilder()
                                .setSessionId(sessionId)
                                .setCols(cols)
                                .setRows(rows)
                                .also { b -> if (cwd.isNotBlank()) b.setCwd(cwd) }
                        )
                        .build()
                )
                while (!closed) {
                    val msg = sendQueue.poll(1, java.util.concurrent.TimeUnit.SECONDS) ?: continue
                    // 【性能】30ms 合并窗口:连打/粘贴时多个按键拼成一个包,
                    // 高延迟链路上显著减少往返次数(gRPC 包数)
                    Thread.sleep(30)
                    val merged = StringBuilder()
                    var m: TerminalClientMessage? = msg
                    while (m != null) {
                        if (m.hasInput()) merged.append(m.input.data.toStringUtf8())
                        else ro.onNext(m) // resize 等非输入消息直接发
                        m = sendQueue.poll()
                    }
                    if (merged.isNotEmpty()) {
                        ro.onNext(
                            TerminalClientMessage.newBuilder()
                                .setInput(
                                    TerminalInput.newBuilder()
                                        .setData(com.google.protobuf.ByteString.copyFromUtf8(merged.toString()))
                                )
                                .build()
                        )
                    }
                }
            } catch (_: Exception) {
            } finally {
                try { ro.onCompleted() } catch (_: Exception) {}
            }
        }.start()
    }

    fun close() {
        if (closed) return
        closed = true
        val observer = requestObserver
        requestObserver = null
        // 先显式关闭服务端 PTY，再结束 gRPC 流，避免切换目录后旧 shell
        // 仍然留在后台；顺序也能减少旧流把 UNKNOWN: EOF 冒泡到新界面。
        Thread {
            runCatching {
                AgentConnection.terminalService()
                    .closeTerminal(CloseTerminalRequest.newBuilder().setSessionId(sessionId).build())
            }
            runCatching { observer?.onCompleted() }
        }.start()
        if (state.value == "connecting" || state.value == "connected") {
            state.value = "closed"
        }
        if (active === this) {
            active = null
        }
        terminalWebView = null
    }

    companion object {
        @Volatile var active: TerminalBridge? = null
    }
}
