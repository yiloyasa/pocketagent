package dev.pocketagent.app.bridge

import io.grpc.StatusRuntimeException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CancelCollabMemberRequest
import pocketagent.v1.CancelCollabSessionRequest
import pocketagent.v1.CloseTerminalRequest
import pocketagent.v1.DeleteCollabRoleRequest
import pocketagent.v1.DispatchCollabRequest
import pocketagent.v1.FileChunk
import pocketagent.v1.GetCollabPolicyRequest
import pocketagent.v1.GetCollabSessionRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.GetPairingInfoRequest
import pocketagent.v1.GetThreadEventsSinceRequest
import pocketagent.v1.GetThreadHistoryRequest
import pocketagent.v1.ListBranchesRequest
import pocketagent.v1.ListCollabRolesRequest
import pocketagent.v1.ListCollabSessionsRequest
import pocketagent.v1.ListDirectoryRequest
import pocketagent.v1.ListSlashCommandsRequest
import pocketagent.v1.ListTerminalsRequest
import pocketagent.v1.ListThreadsRequest
import pocketagent.v1.ReadFileRequest
import pocketagent.v1.RetryCollabMemberRequest
import pocketagent.v1.SetCollabRoleRequest

/**
 * `dev.pocketagent/bridge` 方法面处理器：method → gRPC 派发 + proto 编解码 + 错误码映射。
 *
 * 【契约来源】`flutter/docs/channel-contract.md` §3 三十条方法 + §5 错误码；Dart 侧
 * 调用点（`pa_bridge.dart` / `chat_turn_stream.dart` / `terminal_stream.dart` /
 * `chat_history_transport.dart` / `chat_input_transport.dart`）是 wire 事实源。
 *
 * 【两条已核实并与文档表不同的口径，实现以 Dart 代码为准】
 *  1. §3 表把 `getCollabSession` / `cancelCollabSession` / `cancelCollabMember` /
 *     `retryCollabMember` / `deleteCollabRole` 的 arguments 写成 `{'id': String}` 类
 *     Map，但 Dart 实际发的是**请求消息 wire 字节**（可选参数缺省 = 空消息字节）。
 *     契约总则 §2.1「有 proto 请求消息的方法 = wire 字节」与代码一致，表是旧的。
 *     本实现按字节收；若收到 Map → DECODE_FAILED（不搞 null/bytes 双形态二分）。
 *  2. §3 表写 `listCollabRoles()` arguments = null，Dart 实际发
 *     `ListCollabRolesRequest().writeToBuffer()`（空字节数组）。同样按字节收。
 *
 * 【线程纪律】gRPC blocking stub 不许上平台线程：handler 里 `launch` +
 * `withContext(Dispatchers.IO)`（见 [handle]）。StateFlow 取值是内存读，无需切线程。
 *
 * 【未实现的方法】如实回 HOST_UNAVAILABLE + 注释留后（红线：不伪造返回）。
 */
class BridgeMethods(
    private val messenger: BridgeBinaryMessenger,
    private val connection: BridgeConnection,
    private val scope: CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.IO),
) {
    init {
        messenger.setMethodCallHandler { call, result -> handle(call, result) }
    }

    /**
     * 一次方法调用的完整生命周期（生产由 handler 回调进入；测试直接调本方法）。
     *
     * 返回语义与 Flutter `MethodChannel.Result` 对齐：
     *  - 正常：`result.success(value)`（proto 字节 / Map / null）；
     *  - 契约违约（入参形态不对）：`result.error(DECODE_FAILED, ...)`；
     *  - gRPC/宿主失败：`result.error(契约码, 人话 message, ...)`。
     */
    fun handle(call: BridgeMethodCall, result: BridgeResult) {
        scope.launch {
            try {
                val value = dispatch(call)
                result.success(value)
            } catch (e: BridgePlatformException) {
                result.error(e.code, e.message, e.details)
            } catch (e: Throwable) {
                // dispatch 内部已把 gRPC 失败收敛成 BridgePlatformException；走到这里
                // 的只可能是桥自身 bug。按 §5 INTERNAL 兜底，消息带上方法名定位。
                result.error(
                    BridgeErrorCodes.INTERNAL,
                    "宿主内部错误(${call.method}): ${e.javaClass.simpleName}: ${e.message ?: "无消息"}",
                    e.javaClass.simpleName,
                )
            }
        }
    }

    /**
     * 方法分发主体（挂起函数：测试用 runBlocking 驱动，生产在 IO 线程执行）。
     *
     * 【为什么未知方法也回 HOST_UNAVAILABLE】双端同 APK 同 versionCode（§2.6），
     * 未知方法名只能是契约漂移（一端改了另一端没改），与「本波未实现」同样表现为
     * 「这个 method 没有可用实现」。但**消息分开**：契约内未实现点名留后事项，
     * 契约外点名漂移——排查时一眼可辨，且都不写死任何方法名单数据结构。
     */
    internal suspend fun dispatch(call: BridgeMethodCall): Any? {
        val method = call.method
        return withContext(Dispatchers.IO) {
            when (method) {
                // ── 连接管理（§3.1~§3.4）──────────────────────────────────
                "getHostInfo" -> {
                    // 入参恒为空消息字节（§2.1）；仍解析一次，坏字节即契约漂移。
                    parseProto(method, call.arguments) { GetHostInfoRequest.parseFrom(it) }
                    val info = rpc(method) {
                        connection.hostService().getHostInfo(GetHostInfoRequest.getDefaultInstance())
                    }
                    // 与探活循环同口径：成功路径一律写 capabilities 共享快照
                    // （AgentConnection.updateHostInfo 的 KDoc：所有成功路径写这里）。
                    connection.updateHostInfo(info)
                    info.toByteArray()
                }

                "connect" -> {
                    val args = stringMap(method, call.arguments, "addr", "token")
                    val addr = args.getValue("addr")
                    if (addr.isBlank()) {
                        // §3.2：宿主侧地址为空/不可解析同样回 INVALID_ADDR
                        // （Dart 侧本地已拒过一次，这里是双端同一条口径）。
                        throw BridgePlatformException(
                            BridgeErrorCodes.INVALID_ADDR, "连接地址不能为空",
                        )
                    }
                    connection.connect(addr, args.getValue("token"))
                    null // §3.2：null = 请求受理，不是"已连上"
                }

                "disconnect" -> {
                    expectNull(method, call.arguments)
                    connection.disconnect()
                    null
                }

                "getConnectionState" -> {
                    expectNull(method, call.arguments)
                    // §3.4 五键，键名 = Kotlin 字段名；顺序即契约表的校验顺序。
                    // StateFlow 取值是内存读，无需切 IO。
                    mapOf(
                        "connState" to connection.connState,
                        "connTransport" to connection.connTransport,
                        "connAddress" to connection.connAddress,
                        "connectionEpoch" to connection.connectionEpoch,
                        "hostCapabilities" to connection.hostCapabilities,
                    )
                }

                // ── 主机信息 / 配对（§3.5）────────────────────────────────
                "pairCode" -> {
                    expectNull(method, call.arguments)
                    val resp = rpc(method) {
                        connection.hostService().getPairingInfo(GetPairingInfoRequest.getDefaultInstance())
                    }
                    // §3.5：空/null 返回 = DECODE_FAILED；配对未开启/来源不允许
                    // 必须抛 PAIR_DISABLED，不得用空返回代替错误。
                    if (!resp.pairingEnabled || resp.pairCode.isEmpty()) {
                        throw BridgePlatformException(
                            BridgeErrorCodes.PAIR_DISABLED,
                            if (resp.pairingEnabled) {
                                "配对码为空：电脑端未生成配对码（daemon 未开启配对）"
                            } else {
                                "配对未开启：电脑端以免鉴权模式运行，或配对功能已关闭"
                            },
                        )
                    }
                    resp.pairCode
                }

                // ── 会话 / 目录 / 文件 / 分支（§3.6~§3.10）─────────────────
                "listThreads" -> {
                    val req = parseProto(method, call.arguments) { ListThreadsRequest.parseFrom(it) }
                    rpc(method) { connection.threadService().listThreads(req) }.toByteArray()
                }

                "listDirectory" -> {
                    val req = parseProto(method, call.arguments) { ListDirectoryRequest.parseFrom(it) }
                    rpc(method) { connection.fileService().listDirectory(req) }.toByteArray()
                }

                "readFile" -> {
                    val req = parseProto(method, call.arguments) { ReadFileRequest.parseFrom(it) }
                    // §3.9：proto 是 server stream（64KB 分块），方法面只回**首块**。
                    // 分块流的完整消费面（下载/PaDownloader）不走本方法。
                    val first: FileChunk = rpc(method) {
                        connection.fileService().readFile(req).let { it ->
                            if (!it.hasNext()) {
                                // 空流 = daemon 一个块都没给。按契约这是违约形态
                                // （首块必带 total_size/text），不拿空消息冒充首块。
                                throw BridgePlatformException(
                                    BridgeErrorCodes.INTERNAL,
                                    "readFile 返回了空的分块流（daemon 未返回任何文件块）",
                                )
                            }
                            it.next()
                        }
                    }
                    first.toByteArray()
                }

                "listBranches" -> {
                    val req = parseProto(method, call.arguments) { ListBranchesRequest.parseFrom(it) }
                    rpc(method) { connection.gitService().listBranches(req) }.toByteArray()
                }

                // ── 协同：角色三条（§3.11~§3.13）──────────────────────────
                "listCollabRoles" -> {
                    // Dart 发空消息字节（§2.1 同 getHostInfo），不是 null。
                    val req = parseProto(method, call.arguments) { ListCollabRolesRequest.parseFrom(it) }
                    rpc(method) { connection.threadService().listCollabRoles(req) }.toByteArray()
                }

                "setCollabRole" -> {
                    val req = parseProto(method, call.arguments) { SetCollabRoleRequest.parseFrom(it) }
                    rpc(method) { connection.threadService().setCollabRole(req) }.toByteArray()
                }

                "deleteCollabRole" -> {
                    val req = parseProto(method, call.arguments) { DeleteCollabRoleRequest.parseFrom(it) }
                    rpc(method) { connection.threadService().deleteCollabRole(req) }.toByteArray()
                }

                // ── 协同：派发 / 快照 / 写动作（§3.14~§3.19）────────────────
                "dispatchCollab" -> {
                    val req = parseProto(method, call.arguments) { DispatchCollabRequest.parseFrom(it) }
                    rpc(method) { connection.threadService().dispatchCollab(req) }.toByteArray()
                }

                "getCollabSession" -> {
                    val req = parseProto(method, call.arguments) { GetCollabSessionRequest.parseFrom(it) }
                    rpc(method) { connection.threadService().getCollabSession(req) }.toByteArray()
                }

                "listCollabSessions" -> {
                    val req = parseProto(method, call.arguments) { ListCollabSessionsRequest.parseFrom(it) }
                    rpc(method) { connection.threadService().listCollabSessions(req) }.toByteArray()
                }

                "cancelCollabSession" -> {
                    val req = parseProto(method, call.arguments) { CancelCollabSessionRequest.parseFrom(it) }
                    rpc(method) { connection.threadService().cancelCollabSession(req) }.toByteArray()
                }

                "cancelCollabMember" -> {
                    val req = parseProto(method, call.arguments) { CancelCollabMemberRequest.parseFrom(it) }
                    rpc(method) { connection.threadService().cancelCollabMember(req) }.toByteArray()
                }

                "retryCollabMember" -> {
                    val req = parseProto(method, call.arguments) { RetryCollabMemberRequest.parseFrom(it) }
                    rpc(method) { connection.threadService().retryCollabMember(req) }.toByteArray()
                }

                // ── 协同策略（§3.20，RuntimeService.GetCollabPolicy）────────
                "getCollabPolicy" -> {
                    val req = parseProto(method, call.arguments) { GetCollabPolicyRequest.parseFrom(it) }
                    rpc(method) { connection.runtimeService().getCollabPolicy(req) }.toByteArray()
                }

                // ── 终端（§3.21~§3.24）────────────────────────────────────
                "listTerminals" -> {
                    val req = parseProto(method, call.arguments) { ListTerminalsRequest.parseFrom(it) }
                    rpc(method) { connection.terminalService().listTerminals(req) }.toByteArray()
                }

                "terminalClose" -> {
                    val args = stringMap(method, call.arguments, "terminalId")
                    rpc(method) {
                        connection.terminalService().closeTerminal(
                            CloseTerminalRequest.newBuilder()
                                .setSessionId(args.getValue("terminalId"))
                                .build(),
                        )
                    }
                    null
                }

                // ── Turn 续传快照（§3.28）─────────────────────────────────
                "turnResync" -> {
                    // §3.28：{'threadId': String, 'afterSequence': int} →
                    // List<Uint8List>（每元素一帧 TurnServerMessage wire 字节）。
                    // proto 侧是 server stream（GetThreadEventsSince），Dart 面无
                    // "请求一次回一段流"的形态，故收齐后一次性返回（空 = 无新帧）。
                    val map = argumentsMap(method, call.arguments)
                    val threadId = map["threadId"] as? String
                        ?: throw decodeKey(method, "threadId", map["threadId"])
                    // Dart int 经 StandardMethodCodec 落到 Kotlin 侧是 Integer；
                    // 缺失（旧调用方）按 0 兜底 = "从头发"，与 proto 注释同口径。
                    val after = map["afterSequence"] as? Number
                        ?: throw decodeKey(method, "afterSequence", map["afterSequence"])
                    val req = GetThreadEventsSinceRequest.newBuilder()
                        .setThreadId(threadId)
                        .setAfterSequence(after.toLong())
                        .build()
                    rpc(method) {
                        val frames = mutableListOf<ByteArray>()
                        val iterator = connection.threadService().getThreadEventsSince(req)
                        while (iterator.hasNext()) frames.add(iterator.next().toByteArray())
                        frames
                    }
                }

                // ── 历史 / 斜杠命令（§3.29~§3.30）─────────────────────────
                "getThreadHistory" -> {
                    val req = parseProto(method, call.arguments) { GetThreadHistoryRequest.parseFrom(it) }
                    rpc(method) { connection.threadService().getThreadHistory(req) }.toByteArray()
                }

                "listSlashCommands" -> {
                    val req = parseProto(method, call.arguments) { ListSlashCommandsRequest.parseFrom(it) }
                    rpc(method) { connection.threadService().listSlashCommands(req) }.toByteArray()
                }

                // ── 契约内、本波不实现（如实缺席，不伪造返回）────────────────
                //
                // startTurn / turnStreamStart / turnStreamSend / turnStreamCancel /
                // terminalAttach / terminalWrite 全部依赖**双向流的生命周期管理**，
                // 而过程帧的消费面在事件通道（`bridge/turn_events`、
                // `bridge/terminal_events`，B4 事件面子任务）。本波只建流不收帧 =
                // 用户发了指令却永远看不到增量/审批/退出帧 —— 那是比"通道未就绪"
                // 更糟的假象（看起来连着，其实瞎了）。故整族留后，页面走
                // HOST_UNAVAILABLE 诚实占位（"轮次通道未就绪（B4 波次）"）。

                "startTurn", "turnStreamStart", "turnStreamSend", "turnStreamCancel",
                "terminalAttach", "terminalWrite" ->
                    throw BridgePlatformException(
                        BridgeErrorCodes.HOST_UNAVAILABLE,
                        "方法 $method 本波未实现：双向流的建流/发帧面已就绪，但过程帧" +
                            "消费面（EventChannel bridge/turn_events、bridge/terminal_events）" +
                            "尚未接线；为避免'建了流却收不到任何帧'的假象，整族留待事件面" +
                            "一波实现（B4 后续）",
                    )

                // 双端同版政策：未知方法名只能是契约漂移（一端改了另一端没改）。
                else -> throw BridgePlatformException(
                    BridgeErrorCodes.HOST_UNAVAILABLE,
                    "未知方法 $method（契约漂移：双端同 APK 同版本，未知方法名只能是" +
                        "channel-contract.md §3 与宿主实现未同步，请同步改契约而非静默吞掉）",
                )
            }
        }
    }

    // ── gRPC 调用收口（24 个已实现方法共用）─────────────────────────────────
    //
    // 【为什么每笔 RPC 都要过这一层】错误码映射表（BridgeErrors.contractCodeOf）必须
    // 对**每一笔** RPC 生效，漏一笔就会出现"同一种失败在 A 方法是 NOT_CONNECTED、
    // 在 B 方法变成未映射的裸 gRPC 异常"。inline + reified 后调用点保持一行，
    // 映射逻辑仍然只有一份。

    /** 执行一笔 gRPC 调用，失败按 [BridgeErrors] 映射表转成契约异常。 */
    private suspend fun <T> rpc(method: String, block: () -> T): T = try {
        block()
    } catch (e: StatusRuntimeException) {
        throw rpcFailure(method, e)
    } catch (e: BridgePlatformException) {
        throw e
    } catch (e: Throwable) {
        // 非 gRPC 异常（本地编码/并发……）同样不许裸抛到 Flutter 侧。
        throw grpcFailure(method, e)
    }

    /** gRPC 失败 → 契约码（[grpcFailure] 收口；pairCode 用专属映射）。 */
    private fun rpcFailure(method: String, t: Throwable): BridgePlatformException =
        if (method == "pairCode") grpcFailure(method, t, ::pairCodeContractCodeOf)
        else grpcFailure(method, t)

    // ── 入参解析（三种形态，逐条与 Dart 调用点对账）─────────────────────────

    /**
     * proto 请求消息：arguments 必须是 wire 字节（空消息 = 空字节数组，§2.1）。
     *
     * 坏字节/非字节形态 → DECODE_FAILED：Dart 侧自己编码，到达宿主的字节不可解析
     * 只可能是双端 protobuf 版本漂移，显式失败不补默认值。
     */
    private fun <T> parseProto(
        method: String,
        arguments: Any?,
        parse: (ByteArray) -> T,
    ): T {
        val bytes = arguments as? ByteArray
            ?: throw BridgePlatformException(
                BridgeErrorCodes.DECODE_FAILED,
                "$method 的 arguments 须为 proto wire 字节，实际 ${arguments?.javaClass?.simpleName ?: "null"}",
            )
        return try {
            parse(bytes)
        } catch (e: Exception) {
            throw BridgePlatformException(
                BridgeErrorCodes.DECODE_FAILED,
                "$method 的请求字节不可解析（双端 protobuf 版本可能漂移）: ${e.message}",
            )
        }
    }

    /** arguments 必须是 null（§2.2 无 proto 请求消息且无参数的方法）。 */
    private fun expectNull(method: String, arguments: Any?) {
        if (arguments != null) {
            throw BridgePlatformException(
                BridgeErrorCodes.DECODE_FAILED,
                "$method 契约要求 arguments 为 null，实际 ${arguments.javaClass.simpleName}",
            )
        }
    }

    /**
     * arguments 必须是 `Map`（键值类型由各调用点逐键校验，见 [stringMap]）。
     *
     * 【为什么拆成两步】五条 Map 形态方法里 `turnResync` 混着 int 值
     * （`afterSequence`），与其余四条的全 String 不同；先验"是 Map"再逐键验类型，
     * 两条路径都不必各写一遍 Map 形态检查。
     */
    private fun argumentsMap(method: String, arguments: Any?): Map<*, *> {
        return arguments as? Map<*, *>
            ?: throw BridgePlatformException(
                BridgeErrorCodes.DECODE_FAILED,
                "$method 的 arguments 须为 Map，实际 ${arguments?.javaClass?.simpleName ?: "null"}",
            )
    }

    /** arguments 必须是 `Map<String, String>` 且 [keys] 全部在场（connect / terminalAttach / terminalClose / turnStreamStart 四条）。 */
    private fun stringMap(method: String, arguments: Any?, vararg keys: String): Map<String, String> {
        val map = argumentsMap(method, arguments)
        val out = LinkedHashMap<String, String>(keys.size)
        for (key in keys) {
            val value = map[key] as? String
                ?: throw decodeKey(method, key, map[key])
            out[key] = value
        }
        return out
    }

    /** 键缺失/类型不符 → DECODE_FAILED（不补空串：补空串会把"键名改了"伪装成业务错误）。 */
    private fun decodeKey(method: String, key: String, value: Any?): BridgePlatformException =
        BridgePlatformException(
            BridgeErrorCodes.DECODE_FAILED,
            "$method 的 arguments 键 $key 须为 String，实际 ${value?.javaClass?.simpleName ?: "null"}",
        )

    /** 生产装配入口（[install] 的接收者）；真 Flutter 侧适配见 BridgeFlutterWiring.kt。 */
    companion object
}