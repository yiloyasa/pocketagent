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
import pocketagent.v1.AuthorizeRequest
import pocketagent.v1.ChallengeAnswer
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

    /**
     * 连接方式注册表 —— **唯一扩展点**。新增一种连接方式只需在这里加一条:
     * 候选生成器(addrProvider)+ 可执行说明(hintText)+ 展示标签(label/transport)+ 匹配规则(matcher)。
     * 候选队列、连接方式列表、通道标签**全部由它派生**,不再有第二处散落的字符串字面量。
     *
     * 注册表顺序 = 候选展开顺序:USB(adb reverse 反向隧道)→ 局域网直连 → Tailscale → 用户手填(兜底);
     * 候选队列的最终次序由 [candidateAddresses] 按通道分桶稳定排序给出(见其 KDoc),
     * 通道标签与「连接方式」列表直接按注册表顺序派生。
     */
    private val METHODS: List<ConnMethod> = listOf(
        ConnMethod(
            id = "usb", label = "USB 直连", transport = "USB 通道",
            addrProvider = { USB_ADDR },
            hintText = { "需在电脑上执行 adb reverse tcp:7246 tcp:7246" },
            matcher = { it.startsWith("127.0.0.1") },
        ),
        ConnMethod(
            id = "lan", label = "局域网直连", transport = "局域网直连",
            addrProvider = { lastLanAddr.ifBlank { null } },
            hintText = { "手机与电脑连同一个 WiFi 时最快,地址由电脑端上报" },
            matcher = { it.startsWith("192.168.") || it.startsWith("10.") },
            dedupAgainstMdns = true,
        ),
        ConnMethod(
            id = "tailscale", label = "Tailscale", transport = "Tailscale",
            addrProvider = { lastTailscaleAddr.ifBlank { null } },
            hintText = { "手机在外部网络时,经 Tailscale 虚拟内网直连电脑" },
            matcher = { it.startsWith("100.") },
        ),
        ConnMethod(
            id = "manual", label = "手动地址", transport = "直连",
            addrProvider = { manualAddress.ifBlank { address }.ifBlank { null } },
            hintText = { "设置页「连接」里手动填写的地址" },
            matcher = { true },
        ),
    )

    /** 由候选地址推断通道名(USB 通道 / 局域网直连 / Tailscale / 直连),空白返回空串。 */
    fun transportLabel(addr: String): String {
        if (addr.isBlank()) return ""
        return METHODS.firstOrNull { it.matches(addr) }?.transport ?: "直连"
    }

    /**
     * 全部已知连接端点(USB → 局域网 → Tailscale → 手动),供 UI 只读展示。
     *
     * 【宁缺勿假】地址必须是完整 ip:port;拿不到(例如 daemon 还没上报过局域网 IP)
     * 就不放进列表,而不是塞一个空串凑数。列表内按地址去重(不同方式可能指向同一台机器)。
     */
    fun endpoints(): List<ConnEndpoint> {
        val cur = connAddress.value.ifBlank { lastGoodAddr }
        return METHODS.mapNotNull { m ->
            val addr = m.suggest() ?: return@mapNotNull null
            ConnEndpoint(
                id = m.id,
                label = m.label,
                addr = addr,
                detail = m.detail(),
                active = addr == cur,
            )
        }.distinctBy { it.addr }
    }

    private fun failureFor(error: Throwable, candidate: String): ConnectionDiagnostic {
        val raw = error.message ?: error::class.java.simpleName
        // 【异常链分类】指纹不匹配的 PinMismatchException 会被 OkHttp 包进
        // SSLException,只看 error.message 会丢;这里沿 cause 链找。
        val kind = classifyConnectionFailure(error)
        // 【可执行提示】USB 回环连不上,九成是电脑侧没建反向隧道 —— 只报"不可达"
        // 用户无从下手,所以这里把 adb reverse 那句直接写进 detail。
        val detail = when {
            kind == ConnectionFailureKind.CERT_MISMATCH ->
                "$raw(主机可能被冒充,或电脑端 daemon 重装;确认安全后在" +
                    "「设置 → 用户数据管理 → 已锁定主机指纹」清除后重连)"
            candidate == USB_ADDR ->
                "$raw(USB 直连需在电脑上执行 adb reverse tcp:7246 tcp:7246)"
            else -> raw
        }
        return ConnectionDiagnostic(
            kind = kind,
            message = connectionFailureLabel(kind),
            detail = detail,
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
        // P0-1:证书指纹的持久化存取(TOFU 首连存储,设置页可清除)。
        pinStore = SharedPrefsPinStore(ctx)
        // token 加密收官:创建加密库并把旧明文 prefs 里的 token 无缝迁入
        // (工厂内先写加密库、后删明文键,两步间被杀也幂等不丢 token);
        // Keystore 异常(个别 ROM)在工厂内捕获 → 降级旧明文库 + 打日志,
        // 只影响存储强度,绝不把连接/初始化弄死。
        tokenStore = TokenStores.create(
            secureProvider = { EncryptedTokenStore(ctx) },
            legacy = PrefsTokenStore(ctx),
            log = { msg -> android.util.Log.w(TAG, msg) },
        )
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
            // token 从敏感库读(迁移后明文键已删;降级时敏感库即旧明文库,同样拿得到,
            // 升级安装后旧用户无感续用)。
            token = tokenStore.get(TokenMigration.KEY_TOKEN) ?: ""
            selectedModel = it.getString("model", "") ?: ""
            defaultModel = selectedModel
            selectedAgent = it.getString("agent", "codex") ?: "codex"
            selectedEffort = it.getString("effort", "") ?: ""
            defaultEffort = selectedEffort
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
            // 提示词优化(实验功能):默认关,在外观与阅读里手动打开
            optimizePromptEnabled = it.getBoolean("optimizePrompt", false)
            optimizeAgent = it.getString("optimizeAgent", "zcode") ?: "zcode"
            optimizeModel = it.getString("optimizeModel", "") ?: ""
            drawerCollapsedState = it.getBoolean("drawerCollapsed", true)
            // P0-2:恢复本机设备 ID(UUID);没有就立刻生成一份持久化,
            // 它是 daemon 侧设备注册表里"这台手机"的主键。
            deviceId = it.getString("deviceId", "") ?: ""
            ensureDeviceId()
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

    // ============ P0-1/P0-2:设备身份与证书指纹 ============

    /** 证书指纹存取:生产走 SharedPreferences,init 前用内存兜底(仍保证 TLS 可用)。 */
    @Volatile private var pinStore: PinStore = MemoryPinStore()

    /**
     * token 等敏感值的存取(仿 [pinStore] 模式):生产走 [TokenStores.create]
     * (Keystore 加密库,异常自动降级旧明文 prefs 并无缝迁移);init 前用
     * 内存兜底,JVM 单测不碰 Android 类。token 明文从此只存在于内存([token])
     * 与敏感库中,不再写入 agent_conn prefs。
     */
    @Volatile private var tokenStore: TokenStore = MemoryTokenStore()

    /** Android Keystore 签名器:懒加载,首次真正用到(配对/续期)才碰 Keystore。 */
    private val keystoreSigner: DeviceKeySigner by lazy { KeystoreDeviceKeySigner() }

    /** 本机设备 ID(UUID,持久化);daemon 侧按它登记设备、发放独立 token。 */
    @Volatile var deviceId: String = ""
        private set

    fun ensureDeviceId(): String {
        if (deviceId.isNotBlank()) return deviceId
        val id = java.util.UUID.randomUUID().toString()
        deviceId = id
        prefs?.edit()?.putString("deviceId", id)?.apply()
        return id
    }

    /** 已锁定的主机证书指纹(TOFU 首连存储);null = 尚未锁定。设置页展示用。 */
    fun lockedFingerprint(): String? = pinStore.savedPin()

    /** 清除已存指纹回到 TOFU:下次成功握手会重新锁定(设置页"清除"按钮)。 */
    fun clearHostPin() {
        pinStore.clearPin()
        android.util.Log.i(TAG, "已清除主机指纹,回到首连信任")
    }

    /** 仅 JVM 单测用:切换 TOFU 状态(无 pin=串行 / 有 pin=并行)以锁定安全行为。 */
    internal fun setPinStoreForTest(store: PinStore) {
        pinStore = store
    }

    /** 仅 JVM 单测用:注入假 TokenStore,锁定"token 读写走敏感库"的接线行为。 */
    internal fun setTokenStoreForTest(store: TokenStore) {
        tokenStore = store
    }

    /** 仅 JVM 单测用:注入假的 mDNS 发现器,验证候选链的注入/去重/移除行为。 */
    internal fun setMdnsDiscoveryForTest(d: dev.pocketagent.app.ui.component.MdnsDiscovery?) {
        mdnsDiscovery = d
    }

    /**
     * 配对:优先携设备身份(UUID + P-256 公钥)注册,领每设备独立 token;
     * Keystore 不可用/签名失败/daemon 是旧版 → 自动降级旧 pair_code 流程
     * (不带公钥,daemon 发 legacy 单 token)。返回是否拿到了 token。
     */
    fun authorizeWithPairCode(pairCode: String): Boolean = authorizeWithPairCodeDetailed(pairCode).ok

    /**
     * [authorizeWithPairCode] 的带错误透传版本(设置页保存流程的唯一入口):
     * ok=true 时 token 已经 [saveToken] 写入敏感库并持久化;失败时 [PairCodeResult.error]
     * 为 daemon/传输层**原文**(AUDIT §3.3:如实透传,禁显"未知/错误"裸字样 —— 例如
     * 配对码错就透传 daemon 的"配对码错误",用户才知道该去看电脑上的新码)。
     *
     * 【T-32】此前本函数在整个 UI 层零调用,App 卡死 UNAUTHENTICATED 且无任何配对入口;
     * 现在设置页保存流程按输入形态(见 [credentialModeFor])自动走这里。
     */
    fun authorizeWithPairCodeDetailed(pairCode: String): PairCodeResult {
        val code = pairCode.trim()
        if (code.isEmpty()) {
            return PairCodeResult(false, "配对码为空,请填电脑上显示的 8 位配对码")
        }
        val pub = try {
            if (keystoreSigner.available()) keystoreSigner.publicKeyBase64() else null
        } catch (_: Exception) {
            null
        }
        var firstError: String? = null
        if (!pub.isNullOrBlank()) {
            val req = AuthorizeRequest.newBuilder()
                .setPairCode(code)
                .setDeviceId(ensureDeviceId())
                .setDevicePubKey(pub)
                .setDeviceName(android.os.Build.MODEL ?: "android")
                .build()
            try {
                val resp = hostService().authorize(req)
                if (resp.token.isNotBlank()) {
                    saveToken(resp.token)
                    android.util.Log.i(TAG, "设备配对成功(每设备独立 token)")
                    return PairCodeResult(true)
                }
                firstError = "daemon 接受了配对但没有返回 token"
            } catch (e: Exception) {
                firstError = e.passThroughMessage()
                android.util.Log.w(TAG, "设备配对失败,降级旧流程: ${e.message}")
            }
        }
        // 降级:旧 pair_code 流程(不带公钥),与旧 daemon 也兼容
        return try {
            val resp = hostService().authorize(
                AuthorizeRequest.newBuilder()
                    .setPairCode(code)
                    .setDeviceName(android.os.Build.MODEL ?: "android")
                    .build())
            if (resp.token.isNotBlank()) {
                saveToken(resp.token)
                android.util.Log.i(TAG, "已用旧配对流程获取 token(legacy)")
                PairCodeResult(true)
            } else {
                PairCodeResult(false, firstError ?: "daemon 接受了配对但没有返回 token")
            }
        } catch (e: Exception) {
            android.util.Log.w(TAG, "旧配对流程也失败: ${e.message}")
            PairCodeResult(false, e.passThroughMessage())
        }
    }

    /** 异常 → 可透传原文:有 message 用 message;空白退回类名(可核查的真实信息,非"未知错误")。 */
    private fun Throwable.passThroughMessage(): String =
        message?.takeIf { it.isNotBlank() } ?: this::class.java.simpleName

    /**
     * token 失效(AUTH 错误)时的挑战应答续期:
     * challenge_request → Keystore 签 device_id:challenge → challenge_answer 换新 token。
     * 纯增益路径:任一步失败返回 false 且不抛异常,绝不断连死锁。
     */
    fun renewTokenViaChallenge(): Boolean {
        if (deviceId.isBlank()) return false
        val flow = DeviceAuthFlow(deviceId, keystoreSigner, object : DeviceAuthFlow.Rpc {
            override fun requestChallenge(deviceId: String): String? = try {
                hostService().authorize(
                    AuthorizeRequest.newBuilder()
                        .setChallengeRequest(deviceId)
                        .setDeviceId(deviceId)
                        .build(),
                ).challenge.takeIf { it.isNotBlank() }
            } catch (e: Exception) {
                android.util.Log.w(TAG, "要挑战失败: ${e.message}")
                null
            }

            override fun submitAnswer(deviceId: String, challenge: String, signatureBase64: String): String? = try {
                hostService().authorize(
                    AuthorizeRequest.newBuilder()
                        .setChallengeAnswer(
                            ChallengeAnswer.newBuilder()
                                .setChallenge(challenge)
                                .setSignature(signatureBase64),
                        )
                        .setDeviceId(deviceId)
                        .build(),
                ).token.takeIf { it.isNotBlank() }
            } catch (e: Exception) {
                android.util.Log.w(TAG, "挑战应答失败: ${e.message}")
                null
            }
        })
        val newToken = flow.renewToken() ?: return false
        saveToken(newToken)
        android.util.Log.i(TAG, "token 已通过挑战应答续期")
        return true
    }

    private fun saveToken(t: String) {
        token = t
        persistTokenSecure()
    }

    /** token 单独落敏感库(加密库;Keystore 降级时即旧明文库)。非敏感字段仍走 [persist]。 */
    private fun persistTokenSecure() {
        tokenStore.put(TokenMigration.KEY_TOKEN, token)
    }

    /** 最近一次见到的 daemon Tailscale 地址(ip:port),持久化;无 USB 时的回退候选。 */
    @Volatile var lastTailscaleAddr: String = ""

    /** daemon 上报的局域网地址(同一 WiFi 直连最快),持久化。 */
    @Volatile var lastLanAddr: String = ""

    /**
     * 最近一次成功连接的地址,持久化;冷启动时排在候选队列首位 —— 唯一例外是 USB
     * 反向隧道候选实际可达时让位于 USB(PA-REAL-0006,排序见 [ConnectionCandidateOrder])。
     */
    @Volatile private var lastGoodAddr: String = ""

    /** 仅 JVM 单测用:读 lastGoodAddr(私有态,回归用例要恢复现场)。 */
    internal fun lastGoodAddrForTest(): String = lastGoodAddr

    /** 仅 JVM 单测用:写 lastGoodAddr,钉死「USB 可达→USB 胜 / 不可达→lastGoodAddr 胜」的接线。 */
    internal fun setLastGoodAddrForTest(v: String) {
        lastGoodAddr = v
    }

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
     * 列表类 RPC 的超时:codex 冷启动约 1~2s、子进程重启窗口更久,放宽到 20s 避免
     * 被短暂的 codex 无响应误判成"离线"。配合 daemon 侧 2.5s 同步预算,实际很少用满,
     * 主要是给 codex 上游抖动留出自愈余地(见 daemon 的 syncThreadsFromCodex)。
     */
    private const val LIST_RETRY_TIMEOUT_MS = 20_000L

    /**
     * 候选地址顺序(回退原则的落点,排序规则单一事实源 = [ConnectionCandidateOrder]):
     *   USB 可达:USB(反向隧道实测可达) → 上次成功地址 → mDNS 发现 → 局域网 → 手动 → Tailscale
     *   USB 不可达:上次成功地址 → USB(毫秒级快失败,兜底) → mDNS 发现 → 局域网 → 手动 → Tailscale
     *
     * 【PA-REAL-0006】此前 lastGoodAddr 无条件压过 USB,插着数据线冷启动仍走局域网,
     * 与 NET-01「USB 优先」相悖。现在 USB 候选在场时先做一次短窗口可达性探测
     * ([UsbReachabilityProbe]):可达才插到 lastGoodAddr 之前;不可达维持既有语义。
     *
     * 【为什么 USB 不做前置判定就进队列】
     * 早先这里有一道 `if (isUsbDebugging())` 门槛,依据是 `UsbManager.deviceList.isNotEmpty()`。
     * 但手机插在电脑上时是 USB 设备角色(UFP):真机 `dumpsys usb` 显示
     * data_role=device / host_connected=false,此时 deviceList 必为空 —— 判定恒为 false,
     * USB 候选从不进队列。(实测:USB 调试已开、`adb reverse tcp:7246` 已注册,
     * App 仍显示「局域网直连」。)
     *
     * 【P0-3 mDNS 注入】mDNS 发现的主机在 USB/lastGoodAddr 之后、历史 lanAddr 之前
     * (mDNS 是"此刻真实可见"的,比历史记录新鲜);发现的主机下线/5 分钟过期后,
     * hosts flow 更新,这里下次读取自然不再包含 —— "消失自动移出候选"就是实时读 flow 的结果。
     * 历史 lanAddr 与 mDNS 同 IP 时会被去重(保留 mDNS 的,更新鲜)。
     *
     * @param usbProbe 可达性探测器;null(默认)= 不探测,维持既有队列(USB 在首位)。
     *                 生产路径([connectWithFallback])传 [SocketUsbReachabilityProbe],
     *                 JVM 单测传 mock 钉死两种排序。
     */
    fun candidateAddresses(usbProbe: UsbReachabilityProbe? = null): List<String> =
        candidatesFromRegistry(usbProbe, METHODS)

    /** 仅 JVM 单测用:读连接方式注册表,回归用例据此构造「注册表 + 新增 method」切片。 */
    internal fun methodsForTest(): List<ConnMethod> = METHODS

    /**
     * [candidateAddresses] 的可测核心:注册表作为参数注入,JVM 回归用例据此钉死
     * 「注册表新增连接方式自动进候选」(AUDIT §3.3-1:候选禁止按 id 字面量挑选,
     * 否则新增连接方式会被静默丢弃 —— 既非透传也非报错)。
     */
    internal fun candidatesFromRegistry(
        usbProbe: UsbReachabilityProbe?,
        registry: List<ConnMethod>,
    ): List<String> {
        val mdns = mdnsDiscovery?.current().orEmpty()
        val mdnsAddrs = mdns.map { it.address() }
        val mdnsHosts = mdns.map { it.host }
        // 其余候选一律由注册表遍历产出:新增连接方式 = 注册表加一条,自动进候选。
        // 历史上报类候选(dedupAgainstMdns)与 mDNS 同主机去重(保留 mDNS 的实时值);
        // 用户手填等显式意图不参与该去重。
        // 次序:先按注册表展开,再按通道分桶做稳定排序,恢复既有队列
        // 「mDNS/局域网 → 手动等其他 → Tailscale」;USB 回环候选在这里与队首的
        // usbCandidate 重复,由 [ConnectionCandidateOrder] 的 distinct 收敛,
        // USB 与 lastGoodAddr 的插队定序同样只在 [ConnectionCandidateOrder]。
        val others = (mdnsAddrs + registry.mapNotNull { m ->
            m.suggest()?.takeIf { addr ->
                !m.dedupAgainstMdns || mdnsHosts.none { addr.startsWith("$it:") }
            }
        }).sortedBy { cand ->
            when (transportLabel(cand)) {
                "USB 通道" -> 0
                "局域网直连" -> 1
                "Tailscale" -> 3
                else -> 2
            }
        }
        // 短窗口快速握手:USB 可达 → USB 胜过 lastGoodAddr;不可达/未探测 → lastGoodAddr 胜。
        val usbReachable = usbProbe?.isReachable(USB_ADDR, USB_TIMEOUT_MS) ?: true
        return ConnectionCandidateOrder.order(
            usbCandidate = USB_ADDR,
            usbReachable = usbReachable,
            lastGoodAddr = lastGoodAddr,
            others = others,
        )
    }

    /**
     * P0-3:mDNS 发现器。init 后由 [startAutoReconnect] 创建并启动,
     * 发现结果实时进入候选链;null = 尚未初始化(JVM 单测场景)。
     */
    @Volatile var mdnsDiscovery: dev.pocketagent.app.ui.component.MdnsDiscovery? = null
        private set

    /**
     * 多候选自动回退连接:【并行竞速】所有候选同时探测,谁先通谁上。
     * 串行逐个试在最坏情况要等满 N×超时(4 候选 × 6s = 24s+,用户反馈
     * "重连等待时间过长");并行后重连耗时 ≈ 最快可用候选的 RTT(通常 <1s)。
     * 优先级不再体现在"谁先试",而体现在"失败时按谁的诊断报错"。
     *
     * 【P0-3 TOFU 竞速防护】**未存指纹(首次配对)时改串行**:USB 先试,
     * 失败再逐个试 mDNS/历史候选 —— 竞速下同一网络若有第二台 PocketAgent 主机,
     * 谁先应答就把证书指纹锁给谁,可能锁错主机(阶段 A 报告点名的风险)。
     * 首次配对锁定的必须是"用户想连的那台"。已存指纹后恢复并行竞速:
     * 错误主机在 TLS 握手就被 PinTrustManager 拒绝,并行没有锁错问题。
     */
    private val connectionLock = Any()
    fun connectWithFallback(freshChannel: Boolean = false): pocketagent.v1.HostInfo? = synchronized(connectionLock) {
        // A page refresh or recovery poll must not destroy healthy turn streams.
        if (!freshChannel && connState.value == "ok") {
            try { return hostService().getHostInfo(GetHostInfoRequest.getDefaultInstance()) }
            catch (_: Exception) { }
        }
        // PA-REAL-0006:排序前先探测 USB 反向隧道是否真实在场(短窗口 TCP 握手),
        // 可达则 USB 压过 lastGoodAddr;不可达立即落入原候选队列,回退/竞速架构不变。
        val cands = candidateAddresses(SocketUsbReachabilityProbe)
        connState.value = "checking"
        // freshChannel=true(连接已判死/用户显式重试):强制丢弃旧通道再回退。
        // OkHttp 半死态:daemon 被反复杀死后通道对象未 shutdown、判活认为健康,
        // 但底层连接已坏不再自愈;候选地址没变时旧逻辑会复用死通道 → 只能冷启动恢复。
        // 默认 false:HomeScreen 等导航刷新不动可能仍在服务的通道。
        if (freshChannel) recycleChannel("回退连接前强制重建传输")
        val outcome = CandidateProbe.run(
            cands = cands,
            serial = true,
            timeoutOf = { cand -> if (cand == USB_ADDR) USB_TIMEOUT_MS else REMOTE_TIMEOUT_MS },
            budgetMs = REMOTE_TIMEOUT_MS + 1000L,
            probe = ::probeHost,
        )
        val winner = outcome.winner
        if (winner != null) {
            val (cand, info) = winner
            if (cand != address) {
                address = cand
                persist()
                shutdown()
            }
            rememberTailscaleIp(info.tailscaleIpv4)
            rememberLanIp(info.lanIpv4)
            markConnected(cand)
            return info
        }
        connState.value = "down"
        connError.value = outcome.firstFailure?.let { failureFor(it, cands.first()) }
            ?: ConnectionDiagnostic(
                kind = ConnectionFailureKind.UNAVAILABLE,
                message = connectionFailureLabel(ConnectionFailureKind.UNAVAILABLE),
                detail = "没有可用的连接地址",
                address = address,
            )
        return null
    }

    /**
     * TOFU 串/并行判定(单一事实源,JVM 单测锁定):
     * 已锁定主机指纹(=之前配对成功过) → false(并行竞速);
     * 尚未锁定(首次配对/用户清除了指纹) → true(串行,先 USB 后逐个)。
     */
    fun serialModeForTofu(): Boolean = pinStore.savedPin() == null

    /**
     * 真实探测:建独立临时通道向 [addr] 发 GetHostInfo,探完即弃
     * (互不干扰,也不碰正在用的主通道;主连接由 [channel] 按需重建)。
     * 仅供 [CandidateProbe] 回调;超时由 deadline 控制,失败以异常抛出。
     */
    private fun probeHost(addr: String, timeoutMs: Long): pocketagent.v1.HostInfo {
        val ch = buildChannel(addr, keepAlive = false)
        try {
            return HostServiceGrpc.newBlockingStub(ch)
                .withWaitForReady()
                .withDeadlineAfter(timeoutMs, TimeUnit.MILLISECONDS)
                .getHostInfo(GetHostInfoRequest.getDefaultInstance())
        } finally {
            runCatching { ch.shutdownNow() }
        }
    }

    /** 非敏感连接字段落 prefs;token 已改走敏感库([persistTokenSecure]),明文不再落盘。 */
    private fun persist() {
        prefs?.edit()
            ?.putString("addr", address)
            ?.putString("manualAddr", manualAddress)
            ?.apply()
    }

    /** 设置页保存 token 时调用:token 走敏感库,地址等其余字段照旧落 prefs。 */
    fun persistToken() {
        persistTokenSecure()
        persist()
    }

    /** 兼容旧调用:等价于把当前 selectedModel 记为全局默认。 */
    fun persistModel() = setGlobalModel(selectedModel)

    /** 兼容旧调用:等价于把当前 selectedEffort 记为全局默认。 */
    fun persistEffort() = setGlobalEffort(selectedEffort)

    // ============ 全局默认模型 vs 会话级模型 ============
    // 区分两者,才能解决"在 A 会话切了模型,B 会话跟着变 / 退出再进被重置"。
    //   defaultModel  —— 会话列表里选的"默认模型"(空 = 跟随桌面配置)
    //   selectedModel —— 当前生效值(进入某会话时 = 该会话覆盖值 ?: defaultModel)
    // 注意:属性名不能叫 globalModel —— 它会生成 setGlobalModel(String),
    // 与下面的 fun setGlobalModel 构成 JVM 签名冲突。
    // 【必须可观察】设置页改默认值时,停留在后台的会话页要能重新断言自己的
    // 覆盖值(LaunchedEffect 以它为键),否则显示与实际生效会分叉。
    var defaultModel by mutableStateOf("")
        private set

    fun modelForAgent(agentId: String): String =
        prefs?.getString("agentModel:$agentId", null)
            ?: defaultModel.takeIf { it.startsWith("$agentId/") }.orEmpty()

    fun effortForAgent(agentId: String): String =
        prefs?.getString("agentEffort:$agentId", null)
            ?: if (agentId == "codex") defaultEffort else ""

    fun saveAgentDefaults(agentId: String, model: String, effort: String) {
        prefs?.edit()?.putString("agentModel:$agentId", model)
            ?.putString("agentEffort:$agentId", effort)?.apply()
        if (selectedAgent == agentId) {
            defaultModel = model
            defaultEffort = effort
        }
    }

    /** 设置全局默认模型并持久化(会话内的临时切换请用 setThreadModel)。 */
    fun setGlobalModel(id: String) {
        defaultModel = id
        selectedModel = id
        prefs?.edit()?.putString("model", id)?.apply()
        saveAgentDefaults(selectedAgent,id,effortForAgent(selectedAgent))
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

    /** 回收:会话在电脑端被删后清掉本地残留的模型键(否则键随会话数无上界增长)。 */
    fun removeThreadModel(threadId: String) {
        if (threadId.isBlank()) return
        threadModels.remove(threadId)
        prefs?.edit()?.remove("threadModel:$threadId")?.apply()
    }

    // ============ 全局默认思考强度 vs 会话级思考强度 ============
    // 与模型同一套语义(见上方"全局默认模型 vs 会话级模型"),否则会出现
    // "模型按会话记、强度却是全局单值"的混乱:A 会话调了强度,B 会话跟着变,
    // 重启 App 后强度的修改整个丢失(旧实现只读不写 prefs)。
    //   defaultEffort  —— 设置页选的"思考强度"默认值(空 = 跟随桌面配置)
    //   selectedEffort —— 当前生效值(进入某会话时 = 该会话覆盖值 ?: defaultEffort)
    var defaultEffort by mutableStateOf("")
        private set

    /** 设置全局默认思考强度并持久化(会话内的临时调整请用 setThreadEffort)。 */
    fun setGlobalEffort(id: String) {
        defaultEffort = id
        selectedEffort = id
        prefs?.edit()?.putString("effort", id)?.apply()
        saveAgentDefaults(selectedAgent,modelForAgent(selectedAgent),id)
    }

    // ============ 按会话记住思考强度 ============
    private val threadEfforts = java.util.concurrent.ConcurrentHashMap<String, String>()

    /** 读取该会话记住的思考强度;null = 该会话从未选过(用全局默认)。 */
    fun threadEffort(threadId: String): String? {
        if (threadId.isBlank()) return null
        threadEfforts[threadId]?.let { return it }
        val raw = prefs?.getString("threadEffort:$threadId", null) ?: return null
        threadEfforts[threadId] = raw
        return raw
    }

    /** 记住该会话选定的思考强度("" = 跟随桌面配置),立即持久化。 */
    fun setThreadEffort(threadId: String, effortId: String) {
        if (threadId.isBlank()) return
        threadEfforts[threadId] = effortId
        selectedEffort = effortId
        prefs?.edit()?.putString("threadEffort:$threadId", effortId)?.apply()
    }

    /** Native notifications update only their own thread, not the currently
     * visible Agent's global controls. Null means the native field was absent. */
    fun cacheNativeSelection(threadId: String, model: String?, effort: String?) {
        if (threadId.isBlank()) return
        val edit = prefs?.edit()
        model?.takeIf { it.isNotBlank() }?.let {
            threadModels[threadId] = it
            edit?.putString("threadModel:$threadId", it)
        }
        effort?.let {
            threadEfforts[threadId] = it
            edit?.putString("threadEffort:$threadId", it)
        }
        edit?.apply()
    }

    /** 回收:会话在电脑端被删后清掉本地残留的强度键(与模型键同一时机)。 */
    fun removeThreadEffort(threadId: String) {
        if (threadId.isBlank()) return
        threadEfforts.remove(threadId)
        prefs?.edit()?.remove("threadEffort:$threadId")?.apply()
    }

    /** 切换当前运行的 agent(设置页第二板块跟随它变化)。 */
    fun switchAgent(agentId: String) {
        val id = agentId.trim()
        if (id.isEmpty()) return
        selectedAgent = id
        selectedModel = modelForAgent(id)
        selectedEffort = effortForAgent(id)
        defaultModel = selectedModel
        defaultEffort = selectedEffort
        prefs?.edit()?.putString("agent", id)?.apply()
    }

    fun setTextScale(ui: Float, think: Float) {
        uiTextScale = ui
        thinkTextScale = think
        prefs?.edit()?.putFloat("uiTextScale", ui)?.putFloat("thinkTextScale", think)?.apply()
    }

    @Volatile private var channel: ManagedChannel? = null
    @Volatile private var address: String = "10.0.2.2:7246" // 模拟器里 10.0.2.2 = 宿主机
    /** 内存中的 token(敏感库的运行时副本,gRPC 拦截器读它;落盘一律经 [persistTokenSecure])。 */
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
    /**
     * 实验功能:输入栏"优化提示词"按钮(走电脑端 ZCode 的一次性文本生成)。
     * 默认关:依赖电脑端 ZCode 且消耗其配额,用户在外观与阅读里显式打开才露出。
     */
    private var optimizePromptEnabledState by mutableStateOf(false)
    var optimizePromptEnabled: Boolean
        get() = optimizePromptEnabledState
        set(v) {
            optimizePromptEnabledState = v
            prefs?.edit()?.putBoolean("optimizePrompt", v)?.apply()
        }
    var optimizeAgent by mutableStateOf("zcode")
        private set
    var optimizeModel by mutableStateOf("")
        private set
    fun setOptimizer(agent: String, model: String) {
        optimizeAgent=agent; optimizeModel=model
        prefs?.edit()?.putString("optimizeAgent",agent)?.putString("optimizeModel",model)?.apply()
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
        persistTokenSecure()
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
            return buildChannel(address, keepAlive = true).also { channel = it }
        }
    }

    /**
     * 建一条到 [addr] 的通道(TLS + 证书指纹锁定 + 鉴权拦截)。
     * keepAlive=false 用于一次性探测通道:探完即弃,不需要保活参数。
     */
    private fun buildChannel(addr: String, keepAlive: Boolean): ManagedChannel {
        val builder = OkHttpChannelBuilder.forTarget(addr)
            .maxInboundMessageSize(16 * 1024 * 1024)
        applyPinnedTls(builder)
        if (keepAlive) {
            builder
                .keepAliveTime(30, TimeUnit.SECONDS)
                // 半死连接(Tailscale 掉线但 TCP 未 RST)最坏 30+10s 检出,
                // 用默认值 20s 会把"假在线"多拖 10 秒。
                .keepAliveTimeout(10, TimeUnit.SECONDS)
                .keepAliveWithoutCalls(true)
        }
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
        return builder.build()
    }

    /**
     * P0-1:所有通道一律 TLS(自签证书)+ 指纹锁定。
     * TrustManager = [PinTrustManager](TOFU 首连存指纹,之后不匹配即断);
     * hostnameVerifier 全放行 —— pin 的是证书指纹而非主机名,IP/域名怎么变都无所谓,
     * 校验在 TrustManager 里已经完成。
     * SSLContext 初始化失败(极端 ROM 管制)时回退明文并打警告,至少不把连接整个弄死。
     */
    private fun applyPinnedTls(builder: OkHttpChannelBuilder) {
        try {
            val trustManager = PinTrustManager(pinStore)
            val sslContext = javax.net.ssl.SSLContext.getInstance("TLS")
            // TrustManager 注入 SSLContext:grpc-okhttp 1.66 的 sslSocketFactory
            // 只有单参重载,指纹校验由该工厂产出的 SSLSocket 在握手中执行。
            sslContext.init(null, arrayOf<javax.net.ssl.TrustManager>(trustManager), null)
            builder.useTransportSecurity()
                .sslSocketFactory(sslContext.socketFactory)
                .hostnameVerifier { _, _ -> true }
        } catch (e: Exception) {
            android.util.Log.w(TAG, "TLS 初始化失败,回退明文(仅本通道): ${e.message}")
            builder.usePlaintext()
        }
    }

    fun hostService(): HostServiceGrpc.HostServiceBlockingStub =
        HostServiceGrpc.newBlockingStub(channel()).withDeadlineAfter(8, TimeUnit.SECONDS)

    fun threadService(): ThreadServiceGrpc.ThreadServiceBlockingStub =
        ThreadServiceGrpc.newBlockingStub(channel()).withDeadlineAfter(LIST_RETRY_TIMEOUT_MS, TimeUnit.MILLISECONDS)

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

    /**
     * 无条件丢弃当前 gRPC 通道,下次 [channel] 调用时重建全新传输。
     * 治愈 OkHttp 传输的 UNAVAILABLE 半死态:daemon 被反复杀死后,通道对象本身
     * 未 shutdown/terminated([channel] 的判活认为它健康),但底层连接已坏且不再
     * 重试;[shutdown] 又只在「地址变化」时才被调用,候选地址不变时这条死通道
     * 会被一直复用,表现为 UNAVAILABLE 直到冷启动。回退连接前调用本函数即可自愈。
     */
    fun recycleChannel(reason: String) {
        synchronized(this) {
            val old = channel ?: return
            channel = null
            Log.w(TAG, "回收 gRPC 通道: $reason")
            old.shutdownNow()
        }
    }

    // ================= 自动重连 =================
    // 拔 USB/WiFi 抖动/Tailscale 重连后,app 之前只会显示"未连接"等手动重试。
    // 这里在后台周期探活:断了就走多候选回退(127.0.0.1 USB→Tailscale),
    // 恢复成功把 connectionEpoch +1,各页面监听它自动刷新数据。

    /** 连接恢复计数:每成功重连一次 +1。UI 用 LaunchedEffect(connectionEpoch) 触发刷新。 */
    val connectionEpoch = kotlinx.coroutines.flow.MutableStateFlow(0)

    private var probeScope: kotlinx.coroutines.CoroutineScope? = null

    /**
     * 曾有 RPC 因 token 无效被拒(connError 为 AUTH)时,尝试挑战应答续期。
     * 成功则清掉错误横幅;失败保持现状(token 可能只是 daemon 旧版不支持挑战,
     * 或设备已被撤销 —— 后者只能重新配对,这里不打扰用户)。
     */
    private fun maybeRenewAfterAuthError() {
        val diag = connError.value ?: return
        if (diag.kind != ConnectionFailureKind.AUTH) return
        if (token.isEmpty()) return
        if (renewTokenViaChallenge()) {
            connError.value = null // 已换新锁,别让"token 无效"继续挂在设置页
        }
    }

    /** MainActivity 启动时调一次,开启全局探活循环。 */
    fun startAutoReconnect() {
        // P0-3:mDNS 发现跟随 AgentConnection 生命周期启动(幂等;无 NsdManager/权限
        // 异常时内部降级为不可用,不影响下面的探活循环)。
        if (mdnsDiscovery == null && appCtx != null) {
            mdnsDiscovery = dev.pocketagent.app.ui.component.AndroidMdnsEngine
                .create(appCtx!!)
                .also { runCatching { it.start() } }
        } else {
            mdnsDiscovery?.ensureStarted()
        }
        if (probeScope != null) return
        probeScope = kotlinx.coroutines.CoroutineScope(
            kotlinx.coroutines.SupervisorJob() + kotlinx.coroutines.Dispatchers.IO)
        probeScope?.launch {
            var wasDown = false
            var retryDelayMs = 1000L
            while (true) {
                val ok = try {
                    val info = hostService()
                        .withDeadlineAfter(6000, java.util.concurrent.TimeUnit.MILLISECONDS)
                        .getHostInfo(GetHostInfoRequest.getDefaultInstance())
                    // 探活成功时也刷新局域网/Tailscale 地址:换网络(家里 WiFi → 公司 WiFi)后
                    // IP 会变,这里保证候选队列与「连接方式」列表跟着更新。
                    rememberTailscaleIp(info.tailscaleIpv4)
                    rememberLanIp(info.lanIpv4)
                    true
                } catch (e: Exception) {
                    connError.value = failureFor(e, address)
                    false
                }
                if (ok) {
                    // P0-2:上一轮出现过 AUTH 类失败(token 失效)时,趁连接已恢复
                    // 用挑战应答无感续期一次;失败不影响当前连接(纯增益路径)。
                    maybeRenewAfterAuthError()
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
                    // P0-3:断线时重试 mDNS 发现 —— 网络切换(WiFi↔蜂窝)后
                    // NsdManager 的发现常静默失效,重发一次 discover 才能再次看见电脑。
                    mdnsDiscovery?.ensureStarted()
                    if (connectWithFallback(freshChannel = true) != null) {
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

/** 一个可用的连接端点。id 稳定,用于 UI 排序与去重。 */
data class ConnEndpoint(
    val id: String,      // "usb" | "lan" | "tailscale" | "manual"
    val label: String,   // "USB 直连" | "局域网直连" | "Tailscale" | "手动地址"
    val addr: String,    // 完整 ip:port,如 "127.0.0.1:7246" / "192.168.1.7:7246" / "100.x.y.z:7246"
    val detail: String,  // 可执行说明,如 USB: "需在电脑上执行 adb reverse tcp:7246 tcp:7246"
    val active: Boolean, // 是否当前正在使用
)

/**
 * 一种连接方式(扩展点)。新增连接方式 = 在 [AgentConnection] 的 METHODS 里加一条,其余自动生效。
 *
 * @param addrProvider 产出该方式已知的完整 ip:port;拿不到返回 null(该方式不进候选/不进列表)
 * @param hintText     展示给用户的"怎么用 / 连不上怎么办"的可执行说明
 * @param matcher      判断某个已生效地址是否属于本方式(用于通道标签)
 * @param dedupAgainstMdns true = 该方式产出的候选与 mDNS 已发现主机同 IP 时让位
 *                         (历史上报不如 mDNS 新鲜,如局域网历史地址);用户手填等
 *                         显式意图一律 false,不参与该去重。
 */
class ConnMethod(
    val id: String,
    val label: String,
    val transport: String,
    private val addrProvider: () -> String?,
    private val hintText: () -> String,
    private val matcher: (String) -> Boolean,
    val dedupAgainstMdns: Boolean = false,
) {
    /** 完整的 ip:port;缺端口/空白一律丢弃(宁缺勿假)。 */
    fun suggest(): String? =
        addrProvider()?.trim()?.takeIf { it.isNotEmpty() && it.contains(':') }

    fun detail(): String = hintText()

    fun matches(addr: String): Boolean = matcher(addr)
}

/** [CandidateProbe.run] 的结果:胜者(地址+探测产物)或队首候选的失败原因。 */
data class CandidateOutcome<T>(
    val winner: Pair<String, T>?,
    /** 队首候选(cands.first())抛出的异常;其余候选的失败是噪音,不记录。 */
    val firstFailure: Throwable?,
)

/**
 * 候选探测调度器:同一套探测([probe])支撑"串行逐个试"与"并行竞速"两种策略,
 * 策略由 TOFU 状态决定(见 [AgentConnection.serialModeForTofu])。
 *
 * 【为什么做成注入 probe 的纯调度器】串/并行切换是安全行为(阶段 A 遗留风险的修复),
 * 必须有 JVM 单测锁定 —— 把 OkHttp/gRPC 探测抽出为 lambda,单测就能用假 probe
 * 精确断言"无 pin 串行顺序、有 pin 并行同时起跑",不需要真网络。
 */
object CandidateProbe {

    /**
     * @param cands     候选地址(优先级从高到低)
     * @param serial    true = 按优先级逐个试,第一个成功即停(不再碰后续候选);
     *                  false = 全部候选并行起跑,谁先成功谁上。
     * @param timeoutOf 单候选探测超时(串行模式下每个候选各用各的;并行模式同样生效于 deadline)
     * @param budgetMs  整个探测过程的总等待上界(并行模式收口;串行模式不使用)
     * @param probe     探测函数:成功返回产物,失败抛异常
     */
    fun <T> run(
        cands: List<String>,
        serial: Boolean,
        timeoutOf: (String) -> Long,
        budgetMs: Long,
        probe: (String, Long) -> T,
    ): CandidateOutcome<T> {
        if (cands.isEmpty()) return CandidateOutcome(null, null)
        return if (serial) runSerial(cands, timeoutOf, probe) else runRace(cands, timeoutOf, budgetMs, probe)
    }

    /** 串行:USB 先试,失败再逐个试后续候选。第一个成功即刻返回,后续候选不再探测。 */
    private fun <T> runSerial(
        cands: List<String>,
        timeoutOf: (String) -> Long,
        probe: (String, Long) -> T,
    ): CandidateOutcome<T> {
        var firstFailure: Throwable? = null
        for (cand in cands) {
            try {
                return CandidateOutcome(cand to probe(cand, timeoutOf(cand)), null)
            } catch (e: Throwable) {
                // 失败原因只记队首候选的(它是最该通的那条),其余候选的失败是噪音
                if (cand == cands.first() && firstFailure == null) firstFailure = e
            }
        }
        return CandidateOutcome(null, firstFailure)
    }

    /**
     * 并行竞速:全部候选同时起跑,谁先成功谁上;总等待上界 [budgetMs]。
     * 与原 raceCandidates 语义一致(失败只记队首;胜者产生后不等其余线程退出,
     * 由各 probe 自己的超时/收尾兜底)。
     */
    private fun <T> runRace(
        cands: List<String>,
        timeoutOf: (String) -> Long,
        budgetMs: Long,
        probe: (String, Long) -> T,
    ): CandidateOutcome<T> {
        val latch = java.util.concurrent.CountDownLatch(1)
        val winnerRef = java.util.concurrent.atomic.AtomicReference<Pair<String, T>?>(null)
        val firstFailure = java.util.concurrent.atomic.AtomicReference<Throwable?>(null)
        cands.forEach { cand ->
            Thread {
                try {
                    val result = probe(cand, timeoutOf(cand))
                    if (winnerRef.compareAndSet(null, cand to result)) latch.countDown()
                } catch (e: Throwable) {
                    if (cand == cands.first()) firstFailure.compareAndSet(null, e)
                }
            }.apply { isDaemon = true; name = "pa-probe-$cand" }.start()
        }
        latch.await(budgetMs, java.util.concurrent.TimeUnit.MILLISECONDS)
        return CandidateOutcome(winnerRef.get(), firstFailure.get())
    }
}

// ================= T-32:设置页"配对码 / token"输入的形态决策(纯函数,单一事实源) =================

/**
 * 设置页连接区"配对码 / token"输入框值的语义,由 [credentialModeFor] 唯一判定。
 * 保存流程据此决定:先配对注册领真 token(PAIR_CODE)/ 按 token 直连(TOKEN)/ 留空直连(EMPTY)。
 */
enum class CredentialMode {
    /** 8 位纯数字 = 配对码形态:先走 [AgentConnection.authorizeWithPairCodeDetailed] 注册领真 token,再连接。 */
    PAIR_CODE,

    /** 其余非空值 = daemon 已发的 token:按原 token 直连(现状行为)。 */
    TOKEN,

    /** 留空:不带 token 直连(免鉴权 / 旧 daemon 场景,现状行为)。 */
    EMPTY,
}

/** 配对码形态判定:trim 后恰为 8 位 ASCII 纯数字(daemon 配对码的固定形态)。 */
fun isPairCodeForm(v: String): Boolean {
    val t = v.trim()
    return t.length == 8 && t.all { it in '0'..'9' }
}

/** 输入框值 → 语义:空白=留空 / 8 位纯数字=配对码 / 其余=token(JVM 回归钉死三路分支)。 */
fun credentialModeFor(v: String): CredentialMode = when {
    v.isBlank() -> CredentialMode.EMPTY
    isPairCodeForm(v) -> CredentialMode.PAIR_CODE
    else -> CredentialMode.TOKEN
}

/** 配对结果:ok=是否拿到 token(已入敏感库并持久化);error=失败原文(供 UI 如实透传)。 */
data class PairCodeResult(val ok: Boolean, val error: String? = null)
