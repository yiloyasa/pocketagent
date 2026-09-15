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
        // 持久化"最后一次连上"的快照:冷启动时 UI 能立即显示上次的通道/地址,
        // 候选队列也把它排最前(同一网络下首次连接最快)。
        prefs?.edit()?.putString("lastGoodAddr", candidate)?.apply()
        lastGoodAddr = candidate
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
            defaultModel = selectedModel
            selectedAgent = it.getString("agent", "codex") ?: "codex"
            selectedEffort = it.getString("effort", "") ?: ""
            uiTextScale = it.getFloat("uiTextScale", 1.0f)
            thinkTextScale = it.getFloat("thinkTextScale", 1.0f)
            toolDetailsExpanded = it.getBoolean("toolDetailsExpanded", false)
            themeMode = it.getString("themeMode", "system") ?: "system"
            multiAgent = it.getBoolean("multiAgent", false)
            lastTailscaleAddr = it.getString("tsAddr", "") ?: ""
            lastLanAddr = it.getString("lanAddr", "") ?: ""
            manualAddress = it.getString("manualAddr", "") ?: ""
            lastGoodAddr = it.getString("lastGoodAddr", "") ?: ""
            showThinkingState = it.getBoolean("showThinking", true)
            drawerCollapsedState = it.getBoolean("drawerCollapsed", true)
            // 连接状态快照立即回显:探活完成前 UI 就能显示"上次连接"的通道与地址,
            // 而不是一片空白(状态本身仍是 checking,探活成功后转 ok)。
            if (lastGoodAddr.isNotBlank()) {
                connAddress.value = lastGoodAddr
                connTransport.value = transportLabel(lastGoodAddr)
            }
            recentWorkspaces.value = (it.getString("recentDirs", "") ?: "")
                .split("\n").map { s -> s.trim() }.filter { s -> s.isNotEmpty() }
            dev.pocketagent.app.ui.theme.AgentTheme.mode = themeMode
        }
    }

    /** 最近一次见到的 daemon Tailscale 地址(ip:port),持久化;无 USB 时的回退候选。 */
    @Volatile var lastTailscaleAddr: String = ""

    /** daemon 上报的局域网地址(同一 WiFi 直连最快),持久化。 */
    @Volatile var lastLanAddr: String = ""

    /** 最近一次成功连接的地址,持久化;冷启动时排在候选队列首位。 */
    @Volatile private var lastGoodAddr: String = ""

    /**
     * 用户手填的兜底地址(设置页「连接」里填的那个)。
     *
     * 必须与 [address](当前生效地址)分开存:回退链命中候选后会把 [address] 改写成该候选,
     * 若两者共用一个字段,用户手填的值会被覆盖 —— 之后一旦拔掉 USB,就再也回不到手填地址。
     */
    @Volatile var manualAddress: String = ""

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

    /** USB 调试通道地址(adb reverse tcp:7246 后,手机本机端口即电脑 daemon)。 */
    private const val USB_ADDR = "127.0.0.1:7246"

    /** 回环候选超时:无隧道时本机端口无人监听,连接会被立刻拒绝(实测约 23ms)。 */
    private const val USB_TIMEOUT_MS = 1500L

    /** 其余候选超时:蜂窝下 Tailscale 走境外 DERP 中继,TCP 建立可能 3-5s。 */
    private const val REMOTE_TIMEOUT_MS = 6000L

    /**
     * 候选地址顺序(回退原则的落点):
     *   USB(adb reverse 反向隧道) → 局域网直连 → Tailscale → 用户手填地址(兜底)
     *
     * 【为什么 USB 不再做前置判定】
     * 早先这里有一道 `if (isUsbDebugging())` 门槛,依据是 `UsbManager.deviceList.isNotEmpty()`。
     * 但手机插在电脑上时是 USB 设备角色(UFP):真机 `dumpsys usb` 显示
     * data_role=device / host_connected=false,此时 deviceList 必为空 —— 判定恒为 false,
     * USB 候选从不进队列。(实测:USB 调试已开、`adb reverse tcp:7246` 已注册,
     * App 仍显示「局域网直连」。)
     *
     * 语义上这道判定也是多余的:回环地址只有在电脑侧**确实存在 adb reverse 隧道**时才连得通,
     * 而「有反向隧道」本身就等价于「正插着线通过电脑调试」。既然无隧道时回环是毫秒级失败,
     * 把它放在队首的成本可忽略,比依赖不可靠的角色判定更稳。
     */
    fun candidateAddresses(): List<String> = buildList {
        // 上次连成功的排最前:同一网络下冷启动直取熟路,免去逐个试错。
        add(lastGoodAddr)
        add(USB_ADDR)
        add(lastLanAddr)
        add(lastTailscaleAddr)
        // 兜底用「用户手填地址」而不是 address:后者会被命中的候选覆盖,手填值会丢。
        manualAddress.ifBlank { address }.let { if (it.isNotBlank()) add(it) }
    }.filter { it.isNotBlank() }.distinct()

    /**
     * 多候选自动回退连接:依次尝试 [USB → 局域网 → Tailscale → 手填地址]。
     * USB 候选用 [USB_TIMEOUT_MS](回环无隧道时毫秒级失败);
     * 其余候选用 [REMOTE_TIMEOUT_MS]:蜂窝下 Tailscale 走境外 DERP 中继,
     * TCP 建立可能 3-5 秒,超时取短了会误判成「连不上」。
     */
    fun connectWithFallback(): pocketagent.v1.HostInfo? {
        val cands = candidateAddresses()
        var lastFailure: ConnectionDiagnostic? = null
        connState.value = "checking"
        for (cand in cands) {
            try {
                if (cand != address) {
                    address = cand
                    persist()
                    shutdown()
                }
                val timeoutMs = if (cand == USB_ADDR) USB_TIMEOUT_MS else REMOTE_TIMEOUT_MS
                val info = hostService()
                    .withWaitForReady() // 通道还在 CONNECTING 时等一等,而不是立刻报失败
                    .withDeadlineAfter(timeoutMs, java.util.concurrent.TimeUnit.MILLISECONDS)
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
        prefs?.edit()
            ?.putString("addr", address)
            ?.putString("manualAddr", manualAddress)
            ?.putString("token", token)
            ?.apply()
    }

    /** 设置页保存 token 时调用 */
    fun persistToken() = persist()

    /** 兼容旧调用:等价于把当前 selectedModel 记为全局默认。 */
    fun persistModel() = setGlobalModel(selectedModel)

    fun persistEffort() = persist()

    // ============ 全局默认模型 vs 会话级模型 ============
    // 区分两者,才能解决"在 A 会话切了模型,B 会话跟着变 / 退出再进被重置"。
    //   defaultModel  —— 会话列表里选的"默认模型"(空 = 跟随桌面配置)
    //   selectedModel —— 当前生效值(进入某会话时 = 该会话覆盖值 ?: defaultModel)
    // 注意:属性名不能叫 globalModel —— 它会生成 setGlobalModel(String),
    // 与下面的 fun setGlobalModel 构成 JVM 签名冲突。
    @Volatile
    var defaultModel: String = ""
        private set

    /** 设置全局默认模型并持久化(会话内的临时切换请用 setThreadModel)。 */
    fun setGlobalModel(id: String) {
        defaultModel = id
        selectedModel = id
        prefs?.edit()?.putString("model", id)?.apply()
    }

    // ============ 按会话记住模型 ============
    // 修复:在会话里切过模型后,退出再进来又被重置成"跟随桌面配置"。
    // 之前 selectedModel 是全局唯一值,被别处覆盖后就丢了会话级的意图;
    // 这里按 threadId 单独记一份(空串也是有效值 = 用户明确选了"跟随桌面配置")。
    private val threadModels = java.util.concurrent.ConcurrentHashMap<String, String>()

    /** 读取该会话记住的模型;null = 该会话从未选过(用全局默认)。 */
    fun threadModel(threadId: String): String? {
        if (threadId.isBlank()) return null
        threadModels[threadId]?.let { return it }
        val raw = prefs?.getString("threadModel:$threadId", null) ?: return null
        threadModels[threadId] = raw
        return raw
    }

    /** 记住该会话选定的模型("" = 跟随桌面配置)。 */
    fun setThreadModel(threadId: String, modelId: String) {
        if (threadId.isBlank()) return
        threadModels[threadId] = modelId
        prefs?.edit()?.putString("threadModel:$threadId", modelId)?.apply()
    }

    /** 切换当前运行的 agent(设置页第二板块跟随它变化)。 */
    fun switchAgent(agentId: String) {
        val id = agentId.trim()
        if (id.isEmpty()) return
        selectedAgent = id
        prefs?.edit()?.putString("agent", id)?.apply()
    }

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
    /** 全局开关:会话中是否展示思考过程与执行步骤(关闭=只看正文和结果,数据仍在后台收集) */
    private var showThinkingState by mutableStateOf(true)
    var showThinking: Boolean
        get() = showThinkingState
        set(v) {
            showThinkingState = v
            prefs?.edit()?.putBoolean("showThinking", v)?.apply()
        }
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

    /**
     * 侧边栏折叠偏好(false=展开:图标+文字;true=收起:窄 rail 仅图标,长按看文字提示)。
     * **默认收起**(不遮挡主内容,需要文字时点一下展开),重启后保留用户习惯。
     */
    private var drawerCollapsedState by mutableStateOf(true)
    var drawerCollapsed: Boolean
        get() = drawerCollapsedState
        set(v) {
            drawerCollapsedState = v
            prefs?.edit()?.putBoolean("drawerCollapsed", v)?.apply()
        }

    fun currentAddress(): String = address

    /**
     * 用户在设置页配置的地址(手填优先),用于输入框回显。
     * 与 [currentAddress] 区分:后者是回退链命中的「当前生效地址」,
     * 走 USB 时会变成 127.0.0.1:7246,直接回显到输入框会让人以为手填值丢了。
     * 实际生效地址走 [connAddress] 展示。
     */
    fun configuredAddress(): String = manualAddress.ifBlank { address }

    /** 切换目标主机(设置用),旧连接立即关闭;地址持久化。 */
    fun setAddress(addr: String) {
        if (addr == address && channel != null) return
        address = addr
        manualAddress = addr
        persist()
        shutdown()
    }

    fun setCredentials(addr: String, newToken: String) {
        val trimmed = addr.trim()
        address = trimmed
        manualAddress = trimmed
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
                        retryDelayMs = 1000L
                    }
                    // 网络恢复事件立即唤醒(原来只在"已连上"分支监听,断线期间
                    // WiFi↔蜂窝切换最坏要干等 10 秒退避);没有事件时 1/2/5/10 秒退避,
                    // 避免 Tailscale DERP 抖动时每秒反复销毁连接。
                    kotlinx.coroutines.withTimeoutOrNull(retryDelayMs) {
                        networkChanged.first()
                    }
                    retryDelayMs = (retryDelayMs * 2).coerceAtMost(10_000L)
                }
            }
        }
    }
}
