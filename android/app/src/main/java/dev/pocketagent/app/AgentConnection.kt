package dev.pocketagent.app

import android.util.Log
import android.net.ConnectivityManager
import android.net.Network
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import io.grpc.ClientInterceptor
import io.grpc.ManagedChannel
import io.grpc.Metadata
import io.grpc.MethodDescriptor
import io.grpc.okhttp.OkHttpChannelBuilder
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.HostServiceGrpc
import pocketagent.v1.TerminalServiceGrpc
import pocketagent.v1.FileSystemServiceGrpc
import pocketagent.v1.GitServiceGrpc
import pocketagent.v1.ModelServiceGrpc
import pocketagent.v1.ThreadServiceGrpc
import java.util.concurrent.TimeUnit

/**
 * AgentConnection:与家里电脑 daemon 的连接管理。
 *
 * 【讲解】gRPC 的"Channel"可以理解为一条到服务器的 HTTP/2 长连接。
 * 整个 app 共用一条,按需重建(换地址/断线重连)。OkHttp 传输层是
 * gRPC 官方为 Android 推荐的(netty 太重)。token 走 metadata,相当于
 * HTTP 的 Authorization 头。
 */
object AgentConnection {
    private const val TAG = "AgentConnection"
    private val AUTH_KEY = Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER)
    private var prefs: android.content.SharedPreferences? = null
    private var appCtx: android.content.Context? = null
    private var networkCallback: ConnectivityManager.NetworkCallback? = null
    private val networkChanged = kotlinx.coroutines.flow.MutableSharedFlow<Unit>(extraBufferCapacity = 1)

    // ============ 连接状态(供 UI 实时展示) ============
    /** checking | ok | down */
    val connState = kotlinx.coroutines.flow.MutableStateFlow("checking")
    /** 当前通道:USB 通道 / Tailscale / 局域网直连 */
    val connTransport = kotlinx.coroutines.flow.MutableStateFlow("")
    /** 当前实际地址 */
    val connAddress = kotlinx.coroutines.flow.MutableStateFlow("")
    /** 最近一次连接失败，设置页和首页可直接展示。 */
    val connError = kotlinx.coroutines.flow.MutableStateFlow<ConnectionDiagnostic?>(null)

    fun transportLabel(addr: String): String = when {
        addr.startsWith("127.0.0.1") -> "USB 通道"
        addr.startsWith("100.") -> "Tailscale"
        addr.startsWith("192.168.") || addr.startsWith("10.") -> "局域网直连"
        addr.isBlank() -> ""
        else -> "直连"
    }

    private fun failureFor(error: Throwable, candidate: String): ConnectionDiagnostic {
        val raw = error.message ?: error::class.java.simpleName
        val kind = classifyConnectionFailure(raw)
        return ConnectionDiagnostic(
            kind = kind,
            message = connectionFailureLabel(kind),
            detail = raw,
            address = candidate,
        )
    }

    private fun markConnected(candidate: String) {
        connState.value = "ok"
        connTransport.value = transportLabel(candidate)
        connAddress.value = candidate
        connError.value = null
    }

    // ============ 最近工作区(新建会话/终端选目录用) ============
    val recentWorkspaces = kotlinx.coroutines.flow.MutableStateFlow<List<String>>(emptyList())

    fun addRecentWorkspace(dir: String) {
        val d = dir.trim()
        if (d.isBlank()) return
        val cur = (recentWorkspaces.value - d).toMutableList()
        cur.add(0, d)
        recentWorkspaces.value = cur.take(6)
        prefs?.edit()?.putString("recentDirs", recentWorkspaces.value.joinToString("\n"))?.apply()
    }

    // ============ 前台保活服务(轮次进行中防止系统杀进程) ============
    fun startTurnService() {
        val ctx = appCtx ?: return
        try {
            ctx.startForegroundService(android.content.Intent(ctx, TurnKeepAliveService::class.java))
        } catch (_: Exception) {
        }
    }

    fun stopTurnService() {
        val ctx = appCtx ?: return
        try {
            ctx.stopService(android.content.Intent(ctx, TurnKeepAliveService::class.java))
        } catch (_: Exception) {
        }
    }

    /** MainActivity 启动时调用,恢复上次填写的地址和 token */
    fun init(ctx: android.content.Context) {
        prefs = ctx.getSharedPreferences("agent_conn", android.content.Context.MODE_PRIVATE)
        appCtx = ctx.applicationContext
        val cm = appCtx?.getSystemService(ConnectivityManager::class.java)
        if (cm != null && networkCallback == null) {
            networkCallback = object : ConnectivityManager.NetworkCallback() {
                override fun onAvailable(network: Network) { networkChanged.tryEmit(Unit) }
                override fun onLost(network: Network) { networkChanged.tryEmit(Unit) }
            }
            runCatching { cm.registerDefaultNetworkCallback(networkCallback!!) }
        }
        prefs?.let {
            address = it.getString("addr", address) ?: address
            token = it.getString("token", "") ?: ""
            selectedModel = it.getString("model", "") ?: ""
            selectedEffort = it.getString("effort", "") ?: ""
            uiTextScale = it.getFloat("uiTextScale", 1.0f)
            thinkTextScale = it.getFloat("thinkTextScale", 1.0f)
            toolDetailsExpanded = it.getBoolean("toolDetailsExpanded", false)
            themeMode = it.getString("themeMode", "system") ?: "system"
            multiAgent = it.getBoolean("multiAgent", false)
            lastTailscaleAddr = it.getString("tsAddr", "") ?: ""
            lastLanAddr = it.getString("lanAddr", "") ?: ""
            recentWorkspaces.value = (it.getString("recentDirs", "") ?: "")
                .split("\n").map { s -> s.trim() }.filter { s -> s.isNotEmpty() }
            dev.pocketagent.app.ui.theme.AgentTheme.mode = themeMode
        }
    }

    /** 最近一次见到的 daemon Tailscale 地址(ip:port),持久化;无 USB 时的回退候选。 */
    @Volatile var lastTailscaleAddr: String = ""

    /** daemon 上报的局域网地址(同一 WiFi 直连最快),持久化。 */
    @Volatile var lastLanAddr: String = ""

    /** daemon 上报了局域网 IP 时记下来。 */
    fun rememberLanIp(ip: String) {
        if (ip.isBlank()) return
        val port = address.substringAfterLast(':', "7246")
        val cand = "$ip:$port"
        if (cand != lastLanAddr) {
            lastLanAddr = cand
            prefs?.edit()?.putString("lanAddr", cand)?.apply()
        }
    }

    /** daemon 上报了 Tailscale IP 时记下来(各屏取 HostInfo 后调用)。 */
    fun rememberTailscaleIp(ip: String) {
        if (ip.isBlank()) return
        val port = address.substringAfterLast(':', "7246")
        val cand = "$ip:$port"
        if (cand != lastTailscaleAddr) {
            lastTailscaleAddr = cand
            prefs?.edit()?.putString("tsAddr", cand)?.apply()
        }
    }

    /**
     * 多候选自动回退连接:依次尝试 [当前地址 → USB 反向 → 局域网 → Tailscale]。
     * 每个候选 6 秒:蜂窝网络下 Tailscale 走境外 DERP 中继,TCP 建立可能 3-5 秒,
     * 之前 2.5 秒会误判成"连不上"。
     */
    fun connectWithFallback(): pocketagent.v1.HostInfo? {
        val cands = listOf(address, "127.0.0.1:7246", lastLanAddr, lastTailscaleAddr)
            .filter { it.isNotBlank() }
            .distinct()
        var lastFailure: ConnectionDiagnostic? = null
        connState.value = "checking"
        for (cand in cands) {
            try {
                if (cand != address) {
                    address = cand
                    persist()
                    shutdown()
                }
                val info = hostService()
                    .withDeadlineAfter(6000, java.util.concurrent.TimeUnit.MILLISECONDS)
                    .getHostInfo(GetHostInfoRequest.getDefaultInstance())
                rememberTailscaleIp(info.tailscaleIpv4)
                rememberLanIp(info.lanIpv4)
                markConnected(cand)
                return info
            } catch (e: Exception) {
                lastFailure = failureFor(e, cand)
                // 换下一个候选
            }
        }
        connState.value = "down"
        connError.value = lastFailure ?: ConnectionDiagnostic(
            kind = ConnectionFailureKind.UNAVAILABLE,
            message = connectionFailureLabel(ConnectionFailureKind.UNAVAILABLE),
            detail = "没有可用的连接地址",
            address = address,
        )
        return null
    }

    private fun persist() {
        prefs?.edit()?.putString("addr", address)?.putString("token", token)?.apply()
    }

    /** 设置页保存 token 时调用 */
    fun persistToken() = persist()

    fun persistModel() = persist()

    fun persistEffort() = persist()

    fun setTextScale(ui: Float, think: Float) {
        uiTextScale = ui
        thinkTextScale = think
        prefs?.edit()?.putFloat("uiTextScale", ui)?.putFloat("thinkTextScale", think)?.apply()
    }

    @Volatile private var channel: ManagedChannel? = null
    @Volatile private var address: String = "10.0.2.2:7246" // 模拟器里 10.0.2.2 = 宿主机
    @Volatile var token: String = ""
    /** 用户选定的模型("codex/gpt-5.3" 风格);空 = 跟随 daemon 默认 */
    var selectedModel by mutableStateOf("")
    /** 用户选定的 agent(多 agent 架构预留);当前仅 codex 可用 */
    var selectedAgent by mutableStateOf("codex")
    /** 界面文字缩放(聊天内容);1.0 标准 */
    var uiTextScale by mutableStateOf(1.0f)
    /** 思考显示文字缩放;1.0 标准 */
    var thinkTextScale by mutableStateOf(1.0f)
    /** 普通会话工具详情是否默认展开；审批弹窗使用自己的折叠状态。 */
    private var toolDetailsExpandedState by mutableStateOf(false)
    var toolDetailsExpanded: Boolean
        get() = toolDetailsExpandedState
        set(v) {
            toolDetailsExpandedState = v
            prefs?.edit()?.putBoolean("toolDetailsExpanded", v)?.apply()
        }
    /** 会话内思考强度:空=跟随默认;low/medium/high/xhigh */
    var selectedEffort by mutableStateOf("")
    /** 显示模式:system/light/dark(设置-外观) */
    private var themeModeState by mutableStateOf("system")
    var themeMode: String
        get() = themeModeState
        set(v) {
            themeModeState = v
            prefs?.edit()?.putString("themeMode", v)?.apply()
        }
    /** 多 agent 协同开关(主页) */
    private var multiAgentState by mutableStateOf(false)
    var multiAgent: Boolean
        get() = multiAgentState
        set(v) {
            multiAgentState = v
            prefs?.edit()?.putBoolean("multiAgent", v)?.apply()
        }

    fun currentAddress(): String = address

    /** 切换目标主机(设置用),旧连接立即关闭;地址持久化。 */
    fun setAddress(addr: String) {
        if (addr == address && channel != null) return
        address = addr
        persist()
        shutdown()
    }

    fun setCredentials(addr: String, newToken: String) {
        address = addr.trim()
        token = newToken.trim()
        persist()
        shutdown()
        connState.value = "checking"
        connError.value = null
    }

    fun channel(): ManagedChannel {
        channel?.let { if (!it.isShutdown && !it.isTerminated) return it }
        synchronized(this) {
            channel?.let { if (!it.isShutdown && !it.isTerminated) return it }
            Log.i(TAG, "连接 daemon: $address token=${if (token.isEmpty()) "无" else "有"}")
            val builder = OkHttpChannelBuilder.forTarget(address)
                .usePlaintext() // P0:局域网明文;P1 换 TLS 自签证书
                .keepAliveTime(30, TimeUnit.SECONDS)
                .keepAliveWithoutCalls(true)
                .maxInboundMessageSize(16 * 1024 * 1024)
            if (token.isNotEmpty()) {
                builder.intercept(object : ClientInterceptor {
                    override fun <ReqT, RespT> interceptCall(
                        method: MethodDescriptor<ReqT, RespT>,
                        callOptions: io.grpc.CallOptions,
                        next: io.grpc.Channel,
                    ): io.grpc.ClientCall<ReqT, RespT> {
                        val call = next.newCall(method, callOptions)
                        return object : io.grpc.ForwardingClientCall.SimpleForwardingClientCall<ReqT, RespT>(call) {
                            override fun start(responseListener: Listener<RespT>, headers: Metadata) {
                                headers.put(AUTH_KEY, "Bearer $token")
                                super.start(responseListener, headers)
                            }
                        }
                    }
                })
            }
            return builder.build().also { channel = it }
        }
    }

    fun hostService(): HostServiceGrpc.HostServiceBlockingStub =
        HostServiceGrpc.newBlockingStub(channel()).withDeadlineAfter(8, TimeUnit.SECONDS)

    fun threadService(): ThreadServiceGrpc.ThreadServiceBlockingStub =
        ThreadServiceGrpc.newBlockingStub(channel()).withDeadlineAfter(8, TimeUnit.SECONDS)

    fun threadServiceAsync(): ThreadServiceGrpc.ThreadServiceStub =
        ThreadServiceGrpc.newStub(channel())

    fun terminalServiceAsync(): TerminalServiceGrpc.TerminalServiceStub =
        TerminalServiceGrpc.newStub(channel())

    fun terminalService(): TerminalServiceGrpc.TerminalServiceBlockingStub =
        TerminalServiceGrpc.newBlockingStub(channel()).withDeadlineAfter(8, TimeUnit.SECONDS)

    fun fileService(): FileSystemServiceGrpc.FileSystemServiceBlockingStub =
        FileSystemServiceGrpc.newBlockingStub(channel()).withDeadlineAfter(15, java.util.concurrent.TimeUnit.SECONDS)

    fun gitService(): GitServiceGrpc.GitServiceBlockingStub =
        GitServiceGrpc.newBlockingStub(channel()).withDeadlineAfter(15, java.util.concurrent.TimeUnit.SECONDS)

    fun modelService(): ModelServiceGrpc.ModelServiceBlockingStub =
        ModelServiceGrpc.newBlockingStub(channel()).withDeadlineAfter(15, java.util.concurrent.TimeUnit.SECONDS)

    /** MCP 等慢调用:codex 冷启动可能 30s+,给足 90 秒 */
    fun modelServiceSlow(): ModelServiceGrpc.ModelServiceBlockingStub =
        ModelServiceGrpc.newBlockingStub(channel()).withDeadlineAfter(90, java.util.concurrent.TimeUnit.SECONDS)

    fun shutdown() {
        synchronized(this) {
            channel?.shutdownNow()
            channel = null
        }
    }

    // ================= 自动重连 =================
    // 拔 USB/WiFi 抖动/Tailscale 重连后,app 之前只会显示"未连接"等手动重试。
    // 这里在后台周期探活:断了就走多候选回退(127.0.0.1 USB→Tailscale),
    // 恢复成功把 connectionEpoch +1,各页面监听它自动刷新数据。

    /** 连接恢复计数:每成功重连一次 +1。UI 用 LaunchedEffect(connectionEpoch) 触发刷新。 */
    val connectionEpoch = kotlinx.coroutines.flow.MutableStateFlow(0)

    private var probeScope: kotlinx.coroutines.CoroutineScope? = null

    /** MainActivity 启动时调一次,开启全局探活循环。 */
    fun startAutoReconnect() {
        if (probeScope != null) return
        probeScope = kotlinx.coroutines.CoroutineScope(
            kotlinx.coroutines.SupervisorJob() + kotlinx.coroutines.Dispatchers.IO)
        probeScope?.launch {
            var wasDown = false
            var retryDelayMs = 1000L
            while (true) {
                val ok = try {
                    hostService()
                        .withDeadlineAfter(6000, java.util.concurrent.TimeUnit.MILLISECONDS)
                        .getHostInfo(GetHostInfoRequest.getDefaultInstance())
                    true
                } catch (e: Exception) {
                    connError.value = failureFor(e, address)
                    false
                }
                if (ok) {
                    markConnected(address)
                    if (wasDown) {
                        // 断→通:通知全部页面刷新(host 状态/会话列表/连接卡)
                        connectionEpoch.value += 1
                    }
                    wasDown = false
                    retryDelayMs = 1000L
                    kotlinx.coroutines.withTimeoutOrNull(8000) {
                        networkChanged.first()
                    }
                } else {
                    wasDown = true
                    connState.value = "down"
                    if (connectWithFallback() != null) {
                        // connectWithFallback 已经完成一次断→通状态切换；
                        // 下一轮探活只需保持已恢复状态，避免重复刷新页面。
                        connectionEpoch.value += 1
                        wasDown = false
                    }
                    // 网络恢复事件立即唤醒；没有事件时采用 1/2/5/10 秒退避，
                    // 避免 Tailscale DERP 抖动时每秒反复销毁连接。
                    val wait = retryDelayMs
                    retryDelayMs = (retryDelayMs * 2).coerceAtMost(10_000L)
                    delay(wait)
                }
            }
        }
    }
}
