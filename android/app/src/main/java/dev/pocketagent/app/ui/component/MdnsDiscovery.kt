package dev.pocketagent.app.ui.component

import android.annotation.SuppressLint
import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import java.util.concurrent.atomic.AtomicBoolean

/**
 * P0-3 mDNS 局域网自动发现(手机端)。
 *
 * 【讲解】daemon 在电脑上向局域网宣告 "_pocketagent._tcp" 服务(见 daemon 侧
 * internal/mdns 包);手机端用系统 NsdManager "听"这个宣告,把发现的电脑
 * (IP + 端口 + 主机标识)作为连接候选注入 [dev.pocketagent.app.AgentConnection]。
 * 效果:手机连同一个 WiFi、没手填过地址也能自动找到家里那台电脑。
 *
 * 分层设计(为了让核心逻辑能跑 JVM 单测):
 *  - [MdnsEngine]:系统调用抽象(发现/解析),纯 Kotlin 接口,不含 android 类型;
 *  - [MdnsDiscovery]:状态机核心(去重、过期、排序、resolve 重试),只依赖 [MdnsEngine],
 *    JVM 单测注入 fake engine 即可覆盖全部行为;
 *  - [AndroidMdnsEngine]:真正的 NsdManager 桥(只在设备上运行,单测不加载)。
 *
 * TXT 记录与 daemon 侧约定对齐:v=1 / id=<host_id> / name=<display_name> / tls=1|0。
 */

/** 一台通过 mDNS 发现的 PocketAgent daemon。 */
data class DiscoveredHost(
    /** mDNS 实例名(= daemon 展示名)。 */
    val instanceName: String,
    /** 服务主机 IPv4(如 192.168.1.7),完整地址 = "$host:$port"。 */
    val host: String,
    /** gRPC 端口(一般 7246)。 */
    val port: Int,
    /** daemon 主机稳定标识(TXT id);旧版 daemon 可能缺省为空。 */
    val hostId: String,
    /** 展示名(TXT name,其次实例名)。 */
    val displayName: String,
    /** 端口是否启用 TLS(TXT tls,缺省按 true —— P0-1 起 daemon 默认 TLS)。 */
    val tls: Boolean,
    /** 发现/刷新时刻(由注入时钟给出,用于 5 分钟过期)。 */
    val seenAtMs: Long,
) {
    /** 完整 ip:port,可直接作为 gRPC 目标。 */
    fun address(): String = "$host:$port"
}

/** NsdManager 系统调用的抽象:让 [MdnsDiscovery] 核心逻辑 JVM 可测。 */
interface MdnsEngine {
    /**
     * 开始发现 [serviceType] 类型服务。回调统一走 [callbacks]。
     * 返回 false = 环境不可用(NsdManager 缺失/ discoverServices 抛异常),不抛异常。
     */
    fun startDiscovery(serviceType: String, callbacks: MdnsEngineCallbacks): Boolean

    /** 停止发现(幂等;未在发现中调用应当无操作)。 */
    fun stopDiscovery()

    /** 解析某服务的 IP/端口/TXT(discovery 运行中才有效);失败经 [MdnsEngineCallbacks.onResolveFailed] 回报。 */
    fun resolve(serviceName: String, callbacks: MdnsEngineCallbacks)
}

/** [MdnsEngine] 的回调集合(与 NsdManager 的两类 listener 对齐后收拢)。 */
interface MdnsEngineCallbacks {
    /** 发现了新服务(仅名字;IP/端口需再 resolve)。 */
    fun onFound(serviceName: String)

    /** 服务消失(下线/离开网络)。 */
    fun onLost(serviceName: String)

    /** 解析成功:host 为 IPv4 字符串,txt 为全部 TXT 键值。 */
    fun onResolved(serviceName: String, host: String, port: Int, txt: Map<String, String>)

    /** 解析失败(可重试)。 */
    fun onResolveFailed(serviceName: String)

    /** 发现过程级错误(如内部错误码);发现已停时也会先回调 onStop。 */
    fun onError()
}

class MdnsDiscovery(
    private val engine: MdnsEngine,
    /** Application 级作用域:生命周期跟随 AgentConnection(start/stop 由它驱动)。 */
    private val scope: CoroutineScope,
    /** 时钟注入(JVM 单测可控时间);生产用系统毫秒。 */
    private val nowMs: () -> Long = { System.currentTimeMillis() },
    /** 本机局域网前缀提供者(如 "192.168.1"),用于"同网段优先"排序;取不到返回 null。 */
    private val lanPrefixProvider: () -> String? = { currentLanPrefix() },
    /**
     * 日志注入:核心类要跑 JVM 单测,不能直接用 android.util.Log
     * (JVM 上是 stub,一调就抛)。生产由 [AndroidMdnsEngine.create] 传 android Log。
     */
    private val log: (level: String, msg: String) -> Unit = { _, _ -> },
    private val serviceType: String = SERVICE_TYPE,
) {
    private val lock = Any()
    private val hostsByInstance = LinkedHashMap<String, DiscoveredHost>() // 实例名 → 主机(保持发现顺序)
    private val _hosts = MutableStateFlow<List<DiscoveredHost>>(emptyList())

    /** 当前可见的全部 daemon(已过滤过期、同网段优先排序);UI/候选注入都读它。 */
    val hosts: StateFlow<List<DiscoveredHost>> = _hosts.asStateFlow()

    private val started = AtomicBoolean(false)
    private val callbacks = object : MdnsEngineCallbacks {
        override fun onFound(serviceName: String) {
            markSeen(serviceName)
            // 找到新实例立刻 resolve;失败由 [scheduleResolveRetry] 带退避重试。
            resolveWithRetry(serviceName, attempt = 0)
        }

        override fun onLost(serviceName: String) {
            synchronized(lock) { hostsByInstance.remove(serviceName) }
            publish()
        }

        override fun onResolved(serviceName: String, host: String, port: Int, txt: Map<String, String>) {
            val known = synchronized(lock) { hostsByInstance.containsKey(serviceName) }
            if (!known) return // 已 onLost,迟到的解析结果直接丢弃
            synchronized(lock) {
                hostsByInstance[serviceName] = DiscoveredHost(
                    instanceName = serviceName,
                    host = host,
                    port = port,
                    hostId = txt[TXT_ID]?.trim() ?: "",
                    displayName = txt[TXT_NAME]?.trim()?.takeIf { n -> n.isNotBlank() } ?: serviceName,
                    tls = txt[TXT_TLS]?.trim() != "0", // 缺省按 TLS(P0-1 起 daemon 默认开)
                    seenAtMs = nowMs(),
                )
            }
            publish()
        }

        override fun onResolveFailed(serviceName: String) {
            // 解析是 NsdManager 的异步两步,失败常见(WiFi 抖动/系统服务忙);
            // 带上限的重试,绝不无限循环。
            scheduleResolveRetry(serviceName)
        }

        override fun onError() {
            // 系统级错误:保留已发现列表(它们可能仍有效),允许之后重新 start。
            running.set(false)
        }
    }

    @Volatile private var retryGeneration = 0
    private val running = AtomicBoolean(false)

    /**
     * 启动发现(幂等)。环境异常(无 NsdManager/权限被禁)返回 false 且不崩,
     * 之后可再次调用重试(候选注入方在探活循环里周期性 ensureStarted)。
     */
    fun start(): Boolean {
        if (running.get()) return true
        retryGeneration++ // 作废旧一轮的重试任务
        publish()         // 先把(可能还有效的)旧结果刷一遍过期
        val ok = try {
            engine.startDiscovery(serviceType, callbacks)
        } catch (e: Exception) {
            log("W", "mDNS 启动异常: ${e.message}")
            false
        }
        running.set(ok)
        if (!ok) {
            log("I", "mDNS 发现不可用(权限/环境),自动连接退回历史候选")
        } else {
            // 周期清理:5 分钟没刷新的记录即使没有 onLost 事件(系统偶发不回调)也会过期,
            // hosts flow 与候选注入读到的始终是"最近真的活着"的主机。
            // (不判 isActive:scope 是 Application 级,协程靠 running/gen 条件退出。)
            val gen = retryGeneration
            scope.launch {
                while (true) {
                    delay(EXPIRE_SWEEP_MS)
                    if (gen != retryGeneration || !running.get()) break
                    publish()
                }
            }
        }
        return ok
    }

    /** 与 [start] 等价,语义化别名:探活循环每轮调用,已启动时空开销。 */
    fun ensureStarted(): Boolean = start()

    /** 停止系统发现(列表保留,由过期时间自然清空)。幂等。 */
    fun stop() {
        running.set(false)
        retryGeneration++
        try {
            engine.stopDiscovery()
        } catch (e: Exception) {
            log("W", "mDNS 停止异常: ${e.message}")
        }
    }

    /** 已发现且未过期的主机(过滤未解析完成的占位,同网段优先排序)。 */
    fun current(): List<DiscoveredHost> {
        val all = synchronized(lock) { hostsByInstance.values.toList() }
        val live = filterExpired(all).filter { it.port > 0 && it.host.isNotBlank() }
        return sortedByLan(live, lanPrefixProvider())
    }

    // ---------- 内部:resolve 重试 ----------

    private val resolveMutex = Mutex()
    private val pendingRetries = java.util.concurrent.ConcurrentHashMap<String, Int>()

    private fun resolveWithRetry(serviceName: String, attempt: Int) {
        scope.launch {
            resolveMutex.withLock {
                try {
                    engine.resolve(serviceName, callbacks)
                } catch (e: Exception) {
                    log("W", "resolve 调用异常 $serviceName: ${e.message}")
                    onResolveFailedQuietly(serviceName, attempt)
                }
            }
        }
    }

    private fun onResolveFailedQuietly(serviceName: String, attempt: Int) {
        if (attempt < MAX_RESOLVE_RETRY) scheduleRetry(serviceName, attempt + 1)
    }

    private fun scheduleResolveRetry(serviceName: String) {
        val attempt = (pendingRetries[serviceName] ?: 0)
        if (attempt >= MAX_RESOLVE_RETRY) {
            pendingRetries.remove(serviceName)
            log("I", "resolve 多次失败,放弃本轮: $serviceName")
            return
        }
        scheduleRetry(serviceName, attempt + 1)
    }

    private fun scheduleRetry(serviceName: String, attempt: Int) {
        pendingRetries[serviceName] = attempt
        val gen = retryGeneration
        scope.launch {
            delay(RETRY_DELAY_MS * attempt) // 1x/2x/3x 线性退避
            if (gen == retryGeneration && running.get()) {
                resolveWithRetry(serviceName, attempt)
            }
        }
    }

    private fun markSeen(serviceName: String) {
        synchronized(lock) {
            // onFound 时还没有 IP:占位(port=0),publish 时被过滤;
            // 作用是"已知这个名字",resolve 成功后转正。
            hostsByInstance.putIfAbsent(serviceName, DiscoveredHost(
                instanceName = serviceName, host = "", port = 0,
                hostId = "", displayName = serviceName, tls = true, seenAtMs = nowMs(),
            ))
        }
    }

    // ---------- 内部:发布(去重/过期/排序都在这一步收口,与 [current] 同一条过滤路径) ----------

    private fun publish() {
        _hosts.value = current()
    }

    private fun filterExpired(list: List<DiscoveredHost>): List<DiscoveredHost> {
        val now = nowMs()
        val expired = list.filter { now - it.seenAtMs > EXPIRE_MS }
        if (expired.isNotEmpty()) {
            synchronized(lock) { expired.forEach { hostsByInstance.remove(it.instanceName) } }
        }
        return list.filter { now - it.seenAtMs <= EXPIRE_MS }
    }

    companion object {
        /** 与 daemon 侧 internal/mdns 约定的服务类型(结尾点号是 DNS-SD 习惯写法)。 */
        const val SERVICE_TYPE = "_pocketagent._tcp."

        /** TXT 键(与 daemon 侧 mdns 包对齐)。 */
        const val TXT_ID = "id"
        const val TXT_NAME = "name"
        const val TXT_TLS = "tls"

        /** 发现记录过期时间:5 分钟没刷新即从候选中移除(换网/关机后不再误连)。 */
        const val EXPIRE_MS = 5 * 60 * 1000L

        /** 过期巡检周期(每分钟扫一遍过期记录,刷新 flow)。 */
        const val EXPIRE_SWEEP_MS = 60_000L

        /** 单个实例 resolve 失败重试上限。 */
        const val MAX_RESOLVE_RETRY = 3

        /** 重试基础延迟(乘以尝试次数线性退避)。 */
        const val RETRY_DELAY_MS = 1_000L

        /**
         * "同网段优先"排序:与手机本机同 /24 前缀的排前(直连路径最快),
         * 其余按发现顺序。纯函数,JVM 可测。
         */
        fun sortedByLan(hosts: List<DiscoveredHost>, lanPrefix: String?): List<DiscoveredHost> {
            if (lanPrefix.isNullOrBlank()) return hosts
            return hosts.sortedBy { if (it.host.startsWith("$lanPrefix.")) 0 else 1 }
        }

        /** "192.168.1.7" → "192.168.1"(/24 前缀);非法/空返回 null。 */
        fun lanPrefixOf(ip: String): String? {
            val parts = ip.trim().split(".")
            if (parts.size != 4 || parts.any { it.isEmpty() || it.toIntOrNull() == null }) return null
            return parts.take(3).joinToString(".")
        }

        /**
         * 取手机本机当前局域网前缀(遍历网卡,跳过回环/Tailscale 100.x/link-local)。
         * 生产默认实现;JVM 单测不触碰(注入 lanPrefixProvider 替代)。
         */
        fun currentLanPrefix(): String? = try {
            val it = java.net.NetworkInterface.getNetworkInterfaces().asSequence()
                .filter { it.isUp && !it.isLoopback }
                .flatMap { it.inetAddresses.asSequence() }
                .filter { !it.isLoopbackAddress && it is java.net.Inet4Address }
                .map { it.hostAddress ?: "" }
                .filter { it.isNotEmpty() && !it.startsWith("100.") && !it.startsWith("169.254.") }
            it.firstOrNull()?.let(::lanPrefixOf)
        } catch (e: Exception) {
            null
        }
    }
}

/**
 * 生产 [MdnsEngine]:桥接系统 NsdManager。
 * 仅在 Android 设备上实例化;JVM 单测不加载本类(不引用即不触发 stub)。
 */
@SuppressLint("NewApi") // NsdManager 自 API 16 起可用,minSdk=26 无需版本分支
class AndroidMdnsEngine(context: Context) : MdnsEngine {
    private val nsdManager: NsdManager? = try {
        context.getSystemService(Context.NSD_SERVICE) as? NsdManager
    } catch (e: Exception) {
        android.util.Log.w("MdnsDiscovery", "获取 NsdManager 失败: ${e.message}")
        null
    }

    /** discovery 阶段缓存的服务信息(resolve 需要 NsdServiceInfo 对象,不能只有名字)。 */
    private val pending = java.util.concurrent.ConcurrentHashMap<String, NsdServiceInfo>()

    @Volatile private var discoveryListener: NsdManager.DiscoveryListener? = null
    @Volatile private var callbacksRef: MdnsEngineCallbacks? = null

    override fun startDiscovery(serviceType: String, callbacks: MdnsEngineCallbacks): Boolean {
        val mgr = nsdManager ?: return false
        callbacksRef = callbacks
        if (discoveryListener != null) return true // 已在发现中:幂等
        val listener = object : NsdManager.DiscoveryListener {
            override fun onDiscoveryStarted(serviceType: String?) {}

            override fun onStartDiscoveryFailed(serviceType: String?, errorCode: Int) {
                discoveryListener = null
                android.util.Log.w("MdnsDiscovery", "启动发现失败: $serviceType err=$errorCode")
                callbacks.onError()
            }

            override fun onServiceFound(serviceInfo: NsdServiceInfo?) {
                val info = serviceInfo ?: return
                pending[info.serviceName] = info
                callbacks.onFound(info.serviceName)
            }

            override fun onServiceLost(serviceInfo: NsdServiceInfo?) {
                val info = serviceInfo ?: return
                pending.remove(info.serviceName)
                callbacks.onLost(info.serviceName)
            }

            override fun onDiscoveryStopped(serviceType: String?) {
                discoveryListener = null
            }

            override fun onStopDiscoveryFailed(serviceType: String?, errorCode: Int) {
                // 停不掉就让系统自然超时,下次 start 前置判断会放行重试
                discoveryListener = null
            }
        }
        return try {
            discoveryListener = listener
            mgr.discoverServices(serviceType, NsdManager.PROTOCOL_DNS_SD, listener)
            true
        } catch (e: Exception) {
            discoveryListener = null
            android.util.Log.w("MdnsDiscovery", "discoverServices 异常: ${e.message}")
            false
        }
    }

    override fun stopDiscovery() {
        val mgr = nsdManager ?: return
        val listener = discoveryListener ?: return
        try {
            mgr.stopServiceDiscovery(listener)
        } catch (e: Exception) {
            android.util.Log.w("MdnsDiscovery", "stopServiceDiscovery 异常: ${e.message}")
        }
        discoveryListener = null
        pending.clear()
    }

    override fun resolve(serviceName: String, callbacks: MdnsEngineCallbacks) {
        val mgr = nsdManager ?: return
        val info = pending[serviceName] ?: run {
            callbacks.onResolveFailed(serviceName)
            return
        }
        val listener = object : NsdManager.ResolveListener {
            override fun onResolveFailed(info: NsdServiceInfo?, errorCode: Int) {
                callbacks.onResolveFailed(serviceName)
            }

        @Suppress("DEPRECATION") // host 在 API 34 起 deprecated,但替代 API 需 API 34+;minSdk=26 必须用旧字段
        override fun onServiceResolved(info: NsdServiceInfo?) {
            val r = info ?: run {
                callbacks.onResolveFailed(serviceName)
                return
            }
            val host = r.host?.hostAddress
                if (host.isNullOrBlank()) {
                    callbacks.onResolveFailed(serviceName)
                    return
                }
                val txt = HashMap<String, String>()
                try {
                    r.attributes.forEach { (k, v) -> if (v != null) txt[k] = String(v, Charsets.UTF_8) }
                } catch (e: Exception) {
                    android.util.Log.w("MdnsDiscovery", "TXT 解析异常(忽略): ${e.message}")
                }
                callbacks.onResolved(serviceName, host, r.port, txt)
            }
        }
        try {
            // resolveService(API 34 起 deprecated,被 executor 版取代);minSdk=26 用旧签名全版本可用
            @Suppress("DEPRECATION")
            mgr.resolveService(info, listener)
        } catch (e: Exception) {
            android.util.Log.w("MdnsDiscovery", "resolveService 异常: ${e.message}")
            callbacks.onResolveFailed(serviceName)
        }
    }

    companion object {
        /** 组装一个生产可用的 [MdnsDiscovery](Application 作用域,IO 调度)。 */
        fun create(context: Context): MdnsDiscovery {
            val app = context.applicationContext
            return MdnsDiscovery(
                engine = AndroidMdnsEngine(app),
                scope = CoroutineScope(SupervisorJob() + Dispatchers.IO),
                log = { level, msg ->
                    if (level == "W") android.util.Log.w("MdnsDiscovery", msg)
                    else android.util.Log.i("MdnsDiscovery", msg)
                },
            )
        }
    }
}
