package dev.pocketagent.app.bridge

import io.grpc.Status
import io.grpc.StatusRuntimeException
import com.google.protobuf.ByteString
import pocketagent.v1.CancelCollabMemberRequest
import pocketagent.v1.CancelCollabSessionRequest
import pocketagent.v1.CollabMemberSnapshot
import pocketagent.v1.CollabPolicyResponse
import pocketagent.v1.CollabRole
import pocketagent.v1.CollabSessionSnapshot
import pocketagent.v1.ContentBlock
import pocketagent.v1.DeleteCollabRoleRequest
import pocketagent.v1.DeleteCollabRoleResponse
import pocketagent.v1.DirectoryEntry
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
import pocketagent.v1.GitBranch
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
import pocketagent.v1.SlashCommand
import pocketagent.v1.TerminalSessionInfo
import pocketagent.v1.TextBlock
import pocketagent.v1.Thread
import pocketagent.v1.ThreadMessage
import pocketagent.v1.TurnServerMessage
import pocketagent.v1.CloseTerminalRequest

/**
 * JVM 单测的 fake 装配（与生产 [BridgeConnection] 同一接口，行为由各用例摆布）。
 *
 * 【为什么不用 mockito】本项目 Android 单测零 mock 框架（build.gradle.kts 只有
 * junit + org.json），且这里要的是「真 proto 消息往返 + 字节级断言」——fake 直接
 * 返回构建好的 proto，桥的 parseFrom/toByteArray 路径被真实执行，比 verify() 式
 * 断言更接近线上。
 *
 * 【为什么 stub 也要 fake】protoc 生成的 blocking stub 是 `public final class`
 * （构造器 private）——2026-10-15 三种姿势全部编译失败：继承 stub 报 "final,
 * cannot be extended"；继承可继承的 `AbstractBlockingStub<Stub>` 后 RPC 方法报
 * "overrides nothing"（泛型自引用绑定后父类里没有那些方法）；在默认参数里
 * `NoopChannel()` 报 "This type has a constructor..."。所以 [BridgeConnection]
 * 的服务面收成**窄接口**（[BridgeHostService] 等），测试直接实现接口。
 */

// ── 六个服务的 fake（窄接口直实现）──────────────────────────────────────────

internal class FakeHostService : BridgeHostService {
    var hostInfo: HostInfo? = null
    var pairing: GetPairingInfoResponse? = null
    var failure: Throwable? = null

    override fun getHostInfo(request: GetHostInfoRequest): HostInfo =
        failure?.let { throw it } ?: hostInfo ?: error("用例未摆布 hostInfo")

    override fun getPairingInfo(request: GetPairingInfoRequest): GetPairingInfoResponse =
        failure?.let { throw it } ?: pairing ?: error("用例未摆布 pairing")
}

internal class FakeThreadService : BridgeThreadService {
    var listThreads: ListThreadsResponse? = null
    var listCollabRoles: ListCollabRolesResponse? = null
    var setCollabRole: SetCollabRoleResponse? = null
    var deleteCollabRole: DeleteCollabRoleResponse? = null
    var dispatchCollab: DispatchCollabResponse? = null
    var getCollabSession: CollabSessionSnapshot? = null
    var listCollabSessions: ListCollabSessionsResponse? = null
    var cancelCollabSession: CollabSessionSnapshot? = null
    var cancelCollabMember: CollabMemberSnapshot? = null
    var retryCollabMember: CollabMemberSnapshot? = null
    var getThreadHistory: GetThreadHistoryResponse? = null
    var listSlashCommands: ListSlashCommandsResponse? = null
    var eventsSince: List<TurnServerMessage>? = null
    var failure: Throwable? = null
    /** 最近一次收到的请求（用例据此断言「请求原样过桥」，不做投影/筛选）。 */
    var lastListThreadsRequest: ListThreadsRequest? = null
    var lastGetThreadEventsSinceRequest: GetThreadEventsSinceRequest? = null
    var lastGetThreadHistoryRequest: GetThreadHistoryRequest? = null
    var lastListSlashCommandsRequest: ListSlashCommandsRequest? = null
    var lastSetCollabRoleRequest: SetCollabRoleRequest? = null
    var lastDeleteCollabRoleRequest: DeleteCollabRoleRequest? = null
    var lastDispatchCollabRequest: DispatchCollabRequest? = null
    var lastGetCollabSessionRequest: GetCollabSessionRequest? = null
    var lastListCollabSessionsRequest: ListCollabSessionsRequest? = null
    var lastCancelCollabSessionRequest: CancelCollabSessionRequest? = null
    var lastCancelCollabMemberRequest: CancelCollabMemberRequest? = null
    var lastRetryCollabMemberRequest: RetryCollabMemberRequest? = null

    override fun listThreads(request: ListThreadsRequest): ListThreadsResponse {
        lastListThreadsRequest = request
        return failure?.let { throw it } ?: listThreads ?: error("用例未摆布 listThreads")
    }

    override fun listCollabRoles(request: ListCollabRolesRequest): ListCollabRolesResponse =
        failure?.let { throw it } ?: listCollabRoles ?: error("用例未摆布 listCollabRoles")

    override fun setCollabRole(request: SetCollabRoleRequest): SetCollabRoleResponse {
        lastSetCollabRoleRequest = request
        return failure?.let { throw it } ?: setCollabRole ?: error("用例未摆布 setCollabRole")
    }

    override fun deleteCollabRole(request: DeleteCollabRoleRequest): DeleteCollabRoleResponse {
        lastDeleteCollabRoleRequest = request
        return failure?.let { throw it } ?: deleteCollabRole ?: error("用例未摆布 deleteCollabRole")
    }

    override fun dispatchCollab(request: DispatchCollabRequest): DispatchCollabResponse {
        lastDispatchCollabRequest = request
        return failure?.let { throw it } ?: dispatchCollab ?: error("用例未摆布 dispatchCollab")
    }

    override fun getCollabSession(request: GetCollabSessionRequest): CollabSessionSnapshot {
        lastGetCollabSessionRequest = request
        return failure?.let { throw it } ?: getCollabSession ?: error("用例未摆布 getCollabSession")
    }

    override fun listCollabSessions(request: ListCollabSessionsRequest): ListCollabSessionsResponse {
        lastListCollabSessionsRequest = request
        return failure?.let { throw it } ?: listCollabSessions ?: error("用例未摆布 listCollabSessions")
    }

    override fun cancelCollabSession(request: CancelCollabSessionRequest): CollabSessionSnapshot {
        lastCancelCollabSessionRequest = request
        return failure?.let { throw it } ?: cancelCollabSession ?: error("用例未摆布 cancelCollabSession")
    }

    override fun cancelCollabMember(request: CancelCollabMemberRequest): CollabMemberSnapshot {
        lastCancelCollabMemberRequest = request
        return failure?.let { throw it } ?: cancelCollabMember ?: error("用例未摆布 cancelCollabMember")
    }

    override fun retryCollabMember(request: RetryCollabMemberRequest): CollabMemberSnapshot {
        lastRetryCollabMemberRequest = request
        return failure?.let { throw it } ?: retryCollabMember ?: error("用例未摆布 retryCollabMember")
    }

    override fun getThreadHistory(request: GetThreadHistoryRequest): GetThreadHistoryResponse {
        lastGetThreadHistoryRequest = request
        return failure?.let { throw it } ?: getThreadHistory ?: error("用例未摆布 getThreadHistory")
    }

    override fun listSlashCommands(request: ListSlashCommandsRequest): ListSlashCommandsResponse {
        lastListSlashCommandsRequest = request
        return failure?.let { throw it } ?: listSlashCommands ?: error("用例未摆布 listSlashCommands")
    }

    override fun getThreadEventsSince(request: GetThreadEventsSinceRequest): Iterator<TurnServerMessage> {
        lastGetThreadEventsSinceRequest = request
        failure?.let { throw it }
        return (eventsSince ?: error("用例未摆布 eventsSince")).iterator()
    }
}

internal class FakeFileService : BridgeFileService {
    var listDirectory: ListDirectoryResponse? = null
    var readFileChunks: List<FileChunk>? = null
    var failure: Throwable? = null

    override fun listDirectory(request: ListDirectoryRequest): ListDirectoryResponse =
        failure?.let { throw it } ?: listDirectory ?: error("用例未摆布 listDirectory")

    override fun readFile(request: ReadFileRequest): Iterator<FileChunk> {
        failure?.let { throw it }
        return (readFileChunks ?: error("用例未摆布 readFileChunks")).iterator()
    }
}

internal class FakeGitService : BridgeGitService {
    var listBranches: ListBranchesResponse? = null
    var failure: Throwable? = null

    override fun listBranches(request: ListBranchesRequest): ListBranchesResponse =
        failure?.let { throw it } ?: listBranches ?: error("用例未摆布 listBranches")
}

internal class FakeTerminalService : BridgeTerminalService {
    var listTerminals: ListTerminalsResponse? = null
    var failure: Throwable? = null
    var lastCloseTerminalRequest: CloseTerminalRequest? = null

    override fun listTerminals(request: ListTerminalsRequest): ListTerminalsResponse =
        failure?.let { throw it } ?: listTerminals ?: error("用例未摆布 listTerminals")

    override fun closeTerminal(request: CloseTerminalRequest): Empty {
        lastCloseTerminalRequest = request
        failure?.let { throw it }
        return Empty.getDefaultInstance()
    }
}

internal class FakeRuntimeService : BridgeRuntimeService {
    var collabPolicy: CollabPolicyResponse? = null
    var failure: Throwable? = null

    override fun getCollabPolicy(request: GetCollabPolicyRequest): CollabPolicyResponse =
        failure?.let { throw it } ?: collabPolicy ?: error("用例未摆布 collabPolicy")
}

// ── 连接层 fake ────────────────────────────────────────────────────────────

/** 记录型 [BridgeConnection]：五条 StateFlow 现值 + connect/disconnect 调用留痕。 */
internal class FakeConnection(
    override var connState: String = "checking",
    override var connTransport: String = "",
    override var connAddress: String = "",
    override var connectionEpoch: Int = 0,
    override var hostCapabilities: List<String> = emptyList(),
) : BridgeConnection {
    val host = FakeHostService()
    val thread = FakeThreadService()
    val file = FakeFileService()
    val git = FakeGitService()
    val terminal = FakeTerminalService()
    val runtime = FakeRuntimeService()

    var connectCalls: MutableList<Pair<String, String>> = mutableListOf()
    var disconnectCalls: Int = 0
    var updateHostInfoCalls: MutableList<HostInfo> = mutableListOf()

    override fun connect(addr: String, token: String) {
        connectCalls.add(addr to token)
    }

    override fun disconnect() {
        disconnectCalls += 1
    }

    override fun hostService(): BridgeHostService = host
    override fun threadService(): BridgeThreadService = thread
    override fun fileService(): BridgeFileService = file
    override fun gitService(): BridgeGitService = git
    override fun terminalService(): BridgeTerminalService = terminal
    override fun runtimeService(): BridgeRuntimeService = runtime
    override fun updateHostInfo(info: HostInfo) {
        updateHostInfoCalls.add(info)
    }
}

// ── 通道壳 fake ────────────────────────────────────────────────────────────

/** 捕获型 [BridgeResult]：把 success/error 三件套原样留下供断言。 */
internal class RecordingResult : BridgeResult {
    var success: Any? = null
    var successCalled: Boolean = false
    var errorCode: String? = null
    var errorMessage: String? = null
    var errorDetails: Any? = null
    var errorCalled: Boolean = false
    var notImplementedCalled: Boolean = false

    override fun success(response: Any?) {
        successCalled = true
        success = response
    }

    override fun error(errorCode: String?, errorMessage: String?, errorDetails: Any?) {
        errorCalled = true
        this.errorCode = errorCode
        this.errorMessage = errorMessage
        this.errorDetails = errorDetails
    }

    override fun notImplemented() {
        notImplementedCalled = true
    }
}

/** 记录型 [BridgeBinaryMessenger]：捕获注册进来的 handler（生产适配器的站位）。 */
internal class RecordingMessenger : BridgeBinaryMessenger {
    var handler: BridgeMethodCallHandler? = null
    override fun setMethodCallHandler(handler: BridgeMethodCallHandler) {
        this.handler = handler
    }
}

// ── 错误码驱动 + 响应消息构建 helpers ──────────────────────────────────────

/** 构造一个 StatusRuntimeException（错误码映射表的驱动入口）。 */
internal fun grpcError(code: Status.Code, description: String? = null): StatusRuntimeException =
    Status.fromCode(code).withDescription(description).asRuntimeException()

internal fun hostInfo(
    hostId: String = "host-1",
    displayName: String = "YANZI-PC",
    daemonVersion: String = "0.4.0",
    vararg capabilities: String,
): HostInfo = HostInfo.newBuilder()
    .setHostId(hostId)
    .setDisplayName(displayName)
    .setDaemonVersion(daemonVersion)
    .apply { capabilities.forEach { addCapabilities(it) } }
    .build()

internal fun pairingResponse(pairCode: String, enabled: Boolean = true): GetPairingInfoResponse =
    GetPairingInfoResponse.newBuilder()
        .setPairingEnabled(enabled)
        .setPairCode(pairCode)
        .setDaemonVersion("0.4.0")
        .build()

internal fun listThreadsResponse(vararg threadIds: String): ListThreadsResponse =
    ListThreadsResponse.newBuilder().apply {
        threadIds.forEach { addThreads(Thread.newBuilder().setId(it).setTitle("会话 $it")) }
    }.build()

internal fun listDirectoryResponse(vararg names: String): ListDirectoryResponse =
    ListDirectoryResponse.newBuilder().apply {
        names.forEach {
            addEntries(
                DirectoryEntry.newBuilder()
                    .setName(it)
                    .setPath("/ws/$it")
                    .setIsDir(it.endsWith("/"))
                    .setSizeBytes(7),
            )
        }
    }.build()

internal fun fileChunk(
    data: String,
    totalSize: Long,
    text: Boolean = true,
    modifiedUnixMs: Long = 0,
): FileChunk = FileChunk.newBuilder()
    .setData(ByteString.copyFromUtf8(data))
    .setTotalSize(totalSize)
    .setText(text)
    .setModifiedUnixMs(modifiedUnixMs)
    .build()

internal fun listBranchesResponse(vararg names: String): ListBranchesResponse =
    ListBranchesResponse.newBuilder().apply {
        names.forEach { addBranches(GitBranch.newBuilder().setName(it).setIsCurrent(it == "main")) }
    }.build()

internal fun collabRole(id: String, name: String = "角色$id"): CollabRole =
    CollabRole.newBuilder().setId(id).setName(name).setAgentId("codex").build()

internal fun listCollabRolesResponse(vararg roles: CollabRole): ListCollabRolesResponse =
    ListCollabRolesResponse.newBuilder().apply { roles.forEach { addRoles(it) } }.build()

internal fun setCollabRoleResponse(role: CollabRole): SetCollabRoleResponse =
    SetCollabRoleResponse.newBuilder().setRole(role).build()

internal fun deleteCollabRoleResponse(): DeleteCollabRoleResponse =
    DeleteCollabRoleResponse.getDefaultInstance()

internal fun dispatchCollabResponse(sessionId: String): DispatchCollabResponse =
    DispatchCollabResponse.newBuilder().setSessionId(sessionId).build()

internal fun collabSessionSnapshot(sessionId: String): CollabSessionSnapshot =
    CollabSessionSnapshot.newBuilder().setSessionId(sessionId).setStatus("running").build()

internal fun listCollabSessionsResponse(vararg sessionIds: String): ListCollabSessionsResponse =
    ListCollabSessionsResponse.newBuilder().apply {
        sessionIds.forEach { addSessions(collabSessionSnapshot(it)) }
    }.build()

internal fun collabMemberSnapshot(threadId: String): CollabMemberSnapshot =
    CollabMemberSnapshot.newBuilder().setThreadId(threadId).setStatus("completed").build()

internal fun collabPolicyResponse(revision: Long): CollabPolicyResponse =
    CollabPolicyResponse.newBuilder().setRevision(revision).setPolicyPath("/home/u/policy.json").build()

internal fun listTerminalsResponse(vararg sessionIds: String): ListTerminalsResponse =
    ListTerminalsResponse.newBuilder().apply {
        sessionIds.forEach {
            addSessions(TerminalSessionInfo.newBuilder().setSessionId(it).setAlive(true).setShell("powershell"))
        }
    }.build()

internal fun getThreadHistoryResponse(vararg roles: String): GetThreadHistoryResponse =
    GetThreadHistoryResponse.newBuilder().apply {
        roles.forEach {
            addMessages(
                ThreadMessage.newBuilder()
                    .setRole(it)
                    .addBlocks(
                        ContentBlock.newBuilder().setText(TextBlock.newBuilder().setText("hi")),
                    ),
            )
        }
    }.build()

internal fun listSlashCommandsResponse(vararg names: String): ListSlashCommandsResponse =
    ListSlashCommandsResponse.newBuilder().apply {
        names.forEach { addCommands(SlashCommand.newBuilder().setName(it).setDescription("说明")) }
    }.build()

internal fun turnServerFrame(sequence: Long): TurnServerMessage =
    TurnServerMessage.newBuilder().setSequence(sequence).build()
