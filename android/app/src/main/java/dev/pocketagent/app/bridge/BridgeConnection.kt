package dev.pocketagent.app.bridge

import dev.pocketagent.app.AgentConnection
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import pocketagent.v1.CancelCollabMemberRequest
import pocketagent.v1.CancelCollabSessionRequest
import pocketagent.v1.CollabMemberSnapshot
import pocketagent.v1.CollabPolicyResponse
import pocketagent.v1.CollabSessionSnapshot
import pocketagent.v1.DeleteCollabRoleRequest
import pocketagent.v1.DeleteCollabRoleResponse
import pocketagent.v1.DispatchCollabRequest
import pocketagent.v1.DispatchCollabResponse
import pocketagent.v1.Empty
import pocketagent.v1.FileChunk
import pocketagent.v1.GetCollabPolicyRequest
import pocketagent.v1.GetCollabSessionRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.GetPairingInfoRequest
import pocketagent.v1.GetPairingInfoResponse
import pocketagent.v1.GetThreadEventsSinceRequest
import pocketagent.v1.GetThreadHistoryRequest
import pocketagent.v1.GetThreadHistoryResponse
import pocketagent.v1.HostInfo
import pocketagent.v1.ListBranchesRequest
import pocketagent.v1.ListBranchesResponse
import pocketagent.v1.ListCollabRolesRequest
import pocketagent.v1.ListCollabRolesResponse
import pocketagent.v1.ListCollabSessionsRequest
import pocketagent.v1.ListCollabSessionsResponse
import pocketagent.v1.ListDirectoryRequest
import pocketagent.v1.ListDirectoryResponse
import pocketagent.v1.ListSlashCommandsRequest
import pocketagent.v1.ListSlashCommandsResponse
import pocketagent.v1.ListTerminalsRequest
import pocketagent.v1.ListTerminalsResponse
import pocketagent.v1.ListThreadsRequest
import pocketagent.v1.ListThreadsResponse
import pocketagent.v1.ReadFileRequest
import pocketagent.v1.RetryCollabMemberRequest
import pocketagent.v1.SetCollabRoleRequest
import pocketagent.v1.SetCollabRoleResponse
import pocketagent.v1.TurnServerMessage
import pocketagent.v1.CloseTerminalRequest

/**
 * 六个 gRPC 服务的**窄接口**（只列桥真正调用的 RPC）。
 *
 * 【为什么不是直接暴露 blocking stub】protoc 生成的 stub 是 `public final class`
 * （构造器 private，2026-10-15 实测：Kotlin 侧无法继承也无法覆写）——若
 * [BridgeConnection] 的返回类型写成 `HostServiceGrpc.HostServiceBlockingStub`，
 * 测试就只能建真 stub（需要真 channel/真 daemon），JVM 单测直接死路。
 * 收成窄接口后：生产适配器把真 stub 包一层 lambda；测试直接摆数据。
 * 副作用是接口面**只含被调用的 RPC**，顺手钉住"桥没在偷偷调别的"。
 */
interface BridgeHostService {
    fun getHostInfo(request: GetHostInfoRequest): HostInfo
    fun getPairingInfo(request: GetPairingInfoRequest): GetPairingInfoResponse
}

interface BridgeThreadService {
    fun listThreads(request: ListThreadsRequest): ListThreadsResponse
    fun listCollabRoles(request: ListCollabRolesRequest): ListCollabRolesResponse
    fun setCollabRole(request: SetCollabRoleRequest): SetCollabRoleResponse
    fun deleteCollabRole(request: DeleteCollabRoleRequest): DeleteCollabRoleResponse
    fun dispatchCollab(request: DispatchCollabRequest): DispatchCollabResponse
    fun getCollabSession(request: GetCollabSessionRequest): CollabSessionSnapshot
    fun listCollabSessions(request: ListCollabSessionsRequest): ListCollabSessionsResponse
    fun cancelCollabSession(request: CancelCollabSessionRequest): CollabSessionSnapshot
    fun cancelCollabMember(request: CancelCollabMemberRequest): CollabMemberSnapshot
    fun retryCollabMember(request: RetryCollabMemberRequest): CollabMemberSnapshot
    fun getThreadHistory(request: GetThreadHistoryRequest): GetThreadHistoryResponse
    fun listSlashCommands(request: ListSlashCommandsRequest): ListSlashCommandsResponse
    /** server stream：桥收齐后一次返回（turnResync，§3.28）。 */
    fun getThreadEventsSince(request: GetThreadEventsSinceRequest): Iterator<TurnServerMessage>
}

interface BridgeFileService {
    fun listDirectory(request: ListDirectoryRequest): ListDirectoryResponse
    /** server stream：桥只取首块（readFile，§3.9）。 */
    fun readFile(request: ReadFileRequest): Iterator<FileChunk>
}

interface BridgeGitService {
    fun listBranches(request: ListBranchesRequest): ListBranchesResponse
}

interface BridgeTerminalService {
    fun listTerminals(request: ListTerminalsRequest): ListTerminalsResponse
    fun closeTerminal(request: CloseTerminalRequest): Empty
}

interface BridgeRuntimeService {
    fun getCollabPolicy(request: GetCollabPolicyRequest): CollabPolicyResponse
}

/**
 * 桥逻辑对连接层的**全部**依赖面（可注入 seam）。
 *
 * 【为什么是接口而不是直接调 AgentConnection】AgentConnection 是进程级单例
 * （持有 SharedPreferences / ManagedChannel / mDNS / 探活协程），JVM 单测里一旦
 * 真调它就会碰 Android 运行时。接口化后测试用 fake 摆「五条 StateFlow 现值 +
 * 各服务 RPC 返回什么/抛什么」，桥逻辑（解析→派发→编码→错误映射）被完整执行，
 * 不 stub 自己写的假实现。
 *
 * 【为什么恰好是这些方法】逐一对应 §3 三十条方法的数据需求，无富余：
 *  - 五条 StateFlow 现值 → `getConnectionState` 五键（§3.4）；
 *  - 六个窄服务接口 → 24 个已实现方法的 RPC 派发；
 *  - connect/disconnect → §3.2/§3.3；
 *  - updateHostInfo → getHostInfo 成功路径写 capabilities 共享快照（与探活循环
 *    同口径，见 AgentConnection.updateHostInfo 的 KDoc）。
 */
interface BridgeConnection {
    /** "checking" | "ok" | "down"（值域宿主定义，桥原样透传）。 */
    val connState: String
    /** 当前通道标签（"USB 通道" / "局域网直连" / "Tailscale" / "直连" / 空）。 */
    val connTransport: String
    /** 当前实际地址（未连接时可能为空）。 */
    val connAddress: String
    /** 重连自增计数 = 全局刷新信号（§3.4：断线重连后页面整刷）。 */
    val connectionEpoch: Int
    /** 能力位唯一真值（最近一次 GetHostInfo 成功的快照）。 */
    val hostCapabilities: List<String>

    /**
     * 受理一次连接请求（**异步**：返回即"已受理"，不是"已连上"）。
     *
     * 实现方负责：落地址/token、触发回退连接（[AgentConnection.connectWithFallback]），
     * 真值由 connState / connectionEpoch 异步体现（§3.2）。
     */
    fun connect(addr: String, token: String)

    /** 断开：关通道、停事件订阅、连接态置 down（真值同样走 connState）。 */
    fun disconnect()

    // ── 六个窄服务接口（blocking RPC，桥已在 Dispatchers.IO 上调用）──────────

    fun hostService(): BridgeHostService
    fun threadService(): BridgeThreadService
    fun fileService(): BridgeFileService
    fun gitService(): BridgeGitService
    fun terminalService(): BridgeTerminalService
    fun runtimeService(): BridgeRuntimeService

    /** GetHostInfo 成功路径的统一入口（capabilities 快照 + 版本闸）。 */
    fun updateHostInfo(info: HostInfo)
}

/**
 * 生产适配器：[BridgeConnection] → [AgentConnection] 单例。
 *
 * 【六个服务为什么用 lambda 包一层】AgentConnection 暴露的是 protoc 生成的 final
 * stub 类（无法继承/覆写，见 [BridgeHostService] 的 KDoc）。窄接口 + lambda 转发
 * 让生产侧仍然是"一问一答直通 daemon"，不引入第二份 RPC 实现。
 *
 * 【连接语义的两个决定，都记录在此】
 *  1. `connect` 走 `setCredentials`（地址 + token 一起落：地址进 prefs、token 进
 *     敏感库）+ 异步 `connectWithFallback(freshChannel = true)`。**不**同步等连接
 *     结果——§3.2 明说返回 null = 请求受理；同步等会把多候选竞速（最坏 ~10s+）
 *     全压在 Flutter 的平台线程上。
 *     为什么 freshChannel = true：setCredentials 内部已 shutdown 旧通道，此处显式
 *     丢弃半死传输，与「用户显式重试」同档（AgentConnection.recycleChannel 的
 *     KDoc：OkHttp 半死态下复用旧通道只能冷启动恢复）。
 *  2. `disconnect` 走 `shutdown` + 连接态置 "down"。shutdown 会取消 WatchEvents
 *     订阅并关通道；connState 置 down 让 `getConnectionState` 快照与 `conn_state`
 *     事件立刻反映"用户主动断开"，而不是停留在上一次探活的 "ok"。
 *     已知边界：探活循环（startAutoReconnect）随后可能自动重连——那是 app 既有
 *     行为，桥不新增"用户要求断开"的标志位（不在本波改动 AgentConnection）。
 */
class AgentConnectionBridge : BridgeConnection {
    /** 仅承载 connect 的异步回退连接；与 AgentConnection 的探活协程互不归属。 */
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    override val connState: String get() = AgentConnection.connState.value
    override val connTransport: String get() = AgentConnection.connTransport.value
    override val connAddress: String get() = AgentConnection.connAddress.value
    override val connectionEpoch: Int get() = AgentConnection.connectionEpoch.value
    override val hostCapabilities: List<String> get() = AgentConnection.hostCapabilities.value

    override fun connect(addr: String, token: String) {
        AgentConnection.setCredentials(addr, token)
        scope.launch { AgentConnection.connectWithFallback(freshChannel = true) }
    }

    override fun disconnect() {
        AgentConnection.shutdown()
        AgentConnection.connState.value = "down"
    }

    override fun hostService(): BridgeHostService {
        val stub = AgentConnection.hostService()
        return object : BridgeHostService {
            override fun getHostInfo(request: GetHostInfoRequest): HostInfo = stub.getHostInfo(request)
            override fun getPairingInfo(request: GetPairingInfoRequest): GetPairingInfoResponse =
                stub.getPairingInfo(request)
        }
    }

    override fun threadService(): BridgeThreadService {
        val stub = AgentConnection.threadService()
        return object : BridgeThreadService {
            override fun listThreads(request: ListThreadsRequest): ListThreadsResponse =
                stub.listThreads(request)
            override fun listCollabRoles(request: ListCollabRolesRequest): ListCollabRolesResponse =
                stub.listCollabRoles(request)
            override fun setCollabRole(request: SetCollabRoleRequest): SetCollabRoleResponse =
                stub.setCollabRole(request)
            override fun deleteCollabRole(request: DeleteCollabRoleRequest): DeleteCollabRoleResponse =
                stub.deleteCollabRole(request)
            override fun dispatchCollab(request: DispatchCollabRequest): DispatchCollabResponse =
                stub.dispatchCollab(request)
            override fun getCollabSession(request: GetCollabSessionRequest): CollabSessionSnapshot =
                stub.getCollabSession(request)
            override fun listCollabSessions(request: ListCollabSessionsRequest): ListCollabSessionsResponse =
                stub.listCollabSessions(request)
            override fun cancelCollabSession(request: CancelCollabSessionRequest): CollabSessionSnapshot =
                stub.cancelCollabSession(request)
            override fun cancelCollabMember(request: CancelCollabMemberRequest): CollabMemberSnapshot =
                stub.cancelCollabMember(request)
            override fun retryCollabMember(request: RetryCollabMemberRequest): CollabMemberSnapshot =
                stub.retryCollabMember(request)
            override fun getThreadHistory(request: GetThreadHistoryRequest): GetThreadHistoryResponse =
                stub.getThreadHistory(request)
            override fun listSlashCommands(request: ListSlashCommandsRequest): ListSlashCommandsResponse =
                stub.listSlashCommands(request)
            override fun getThreadEventsSince(request: GetThreadEventsSinceRequest): Iterator<TurnServerMessage> =
                stub.getThreadEventsSince(request)
        }
    }

    override fun fileService(): BridgeFileService {
        val stub = AgentConnection.fileService()
        return object : BridgeFileService {
            override fun listDirectory(request: ListDirectoryRequest): ListDirectoryResponse =
                stub.listDirectory(request)
            override fun readFile(request: ReadFileRequest): Iterator<FileChunk> = stub.readFile(request)
        }
    }

    override fun gitService(): BridgeGitService {
        val stub = AgentConnection.gitService()
        return object : BridgeGitService {
            override fun listBranches(request: ListBranchesRequest): ListBranchesResponse =
                stub.listBranches(request)
        }
    }

    override fun terminalService(): BridgeTerminalService {
        val stub = AgentConnection.terminalService()
        return object : BridgeTerminalService {
            override fun listTerminals(request: ListTerminalsRequest): ListTerminalsResponse =
                stub.listTerminals(request)
            override fun closeTerminal(request: CloseTerminalRequest): Empty = stub.closeTerminal(request)
        }
    }

    override fun runtimeService(): BridgeRuntimeService {
        val stub = AgentConnection.runtimeService()
        return object : BridgeRuntimeService {
            override fun getCollabPolicy(request: GetCollabPolicyRequest): CollabPolicyResponse =
                stub.getCollabPolicy(request)
        }
    }

    override fun updateHostInfo(info: HostInfo) = AgentConnection.updateHostInfo(info)
}
