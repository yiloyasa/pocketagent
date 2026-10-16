package dev.pocketagent.app.bridge

import io.flutter.plugin.common.BinaryMessenger
import io.grpc.StatusRuntimeException
import io.grpc.stub.ClientCallStreamObserver
import io.grpc.stub.StreamObserver
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
import pocketagent.v1.TerminalClientMessage
import pocketagent.v1.TurnClientMessage

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
 * 【双向流六方法（turnStreamStart/Send/Cancel + terminalAttach/Write）】
 * 建流/摘流走 B4d 的 `BridgeEventChannels` 预留 machinery（startTurnStream 等），
 * 本文件只负责：入参解析（Map/字节/null 三形态）→ 取 stub 与 sink → 调 machinery →
 * 错误码收口。**sink 与 stub 的获取经 [BridgeStreamWiring] seam 注入**：B4d 的
 * `turnHandler.onListen` 才持有 Flutter 框架传来的那个 sink，且它不对外暴露；
 * 由装配侧（installEventChannels 的调用方）把实例接进来。未注入时六方法如实回
 * HOST_UNAVAILABLE 并点名缺什么（不建「建了流却收不到帧」的假流）。
 *
 * 【未实现的方法】如实回 HOST_UNAVAILABLE + 注释留后（红线：不伪造返回）。
 */
class BridgeMethods(
    private val messenger: BridgeBinaryMessenger,
    private val connection: BridgeConnection,
    /**
     * 双向流六方法的装配 seam（turn/terminal 的 stub 取数 + sink 来源）。
     *
     * null = 未装配：六方法如实回 HOST_UNAVAILABLE 点名缺什么。生产由
     * `installEventChannels` 的调用方经 [BridgeStreamWiring] 接进来；测试注入
     * fake 驱动真 machinery。
     */
    private val streams: BridgeStreamWiring? = null,
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
                    // 同时摘掉宿主侧的 Attach 双向流：unary CloseTerminal 只让 daemon
                    // 关 PTY，本地那条 bidi 流不摘就会继续往 Dart 推帧（订阅方可能
                    // 已取消订阅 → sink 指向已死的 messenger）。防泄漏 + 不推幽灵帧。
                    //
                    // 【为什么在 finally】unary 失败（daemon 不可达/已关）时本地那条
                    // 流**更需要**摘——daemon 明明已经回应不了，本地还挂着它推帧就是
                    // 幽灵帧源。故报错照旧报，摘流照旧摘，两者不互相取消。
                    try {
                        rpc(method) {
                            connection.terminalService().closeTerminal(
                                CloseTerminalRequest.newBuilder()
                                    .setSessionId(args.getValue("terminalId"))
                                    .build(),
                            )
                        }
                    } finally {
                        streams?.cancelTerminalStream()
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

                // ── 双向流：turn 三条（§3.25~§3.27）──────────────────────
                //
                // 时序（Dart chat_session.dart：_subscribe() 先订阅 turn_events，
                // 再 turnStreamStart → turnStreamSend 首帧）：sink 由 B4d 的
                // turnHandler.onListen 持有并经 [BridgeStreamWiring] 交进来；
                // stub 由 seam 取（AgentConnection.threadServiceAsync()）。
                // 任一项缺失即 HOST_UNAVAILABLE 点名——不建收不到帧的假流。

                "turnStreamStart" -> {
                    // §3.25：{'threadId': String} → null（建流受理，首帧另发）。
                    val args = stringMap(method, call.arguments, "threadId")
                    val wiring = streams ?: throw streamsNotWired(method, "turn")
                    val sink = wiring.turnSink()
                        ?: throw BridgePlatformException(
                            BridgeErrorCodes.HOST_UNAVAILABLE,
                            "$method 无法建流：Dart 尚未订阅 bridge/turn_events（sink 不在）。" +
                                "订阅时序见 chat_session.dart 的 _subscribe() —— 先订阅再建流。",
                        )
                    wiring.startTurnStream(args.getValue("threadId"), sink)
                    // 等请求 observer 就绪再回「受理」：Dart 紧随其后的
                    // turnStreamSend(首帧) 要有落点，否则首帧会打到空 observer 上
                    // （假错误）。B4d 的会话协程跑在它自己的 IO scope 上。
                    wiring.awaitTurnRequestObserver(STREAM_READY_TIMEOUT_MS)
                    null
                }

                "turnStreamSend" -> {
                    // §3.26：TurnClientMessage wire 字节 → null。
                    // 字节由 Dart 按 proto 封（input/supplement/approval/cancel
                    // 四形态统一入口），宿主只验形透传，不拆 oneof。
                    val req = parseProto(method, call.arguments) { TurnClientMessage.parseFrom(it) }
                    val observer = streams?.turnRequestObserver()
                        ?: throw BridgePlatformException(
                            BridgeErrorCodes.HOST_UNAVAILABLE,
                            "$method 无处可发：宿主侧没有已建的 StartTurn 流（未 turnStreamStart，" +
                                "或流已摘/建流失败）。这是调用时序问题，不是连接问题。",
                        )
                    // observer.onNext 可能抛（流已断/cancelled）——按 gRPC 失败收口，
                    // 不裸抛到 Flutter 侧。
                    rpc(method) { observer.onNext(req) }
                    null
                }

                "turnStreamCancel" -> {
                    // §3.27：null → null。只摘宿主侧流资源，不发 cancel 帧
                    // （ Dart 侧显式取消走 turnStreamSend(cancel 帧)，见 chat_session.cancel()）。
                    expectNull(method, call.arguments)
                    streams?.cancelTurnStream()
                    null
                }

                // ── 双向流：terminal 三条（§3.22~§3.24）───────────────────
                //
                // 时序（Dart terminal_data.dart 的 open()：先 _dropSubscription +
                // 订阅 terminal_events，再 terminalAttach → terminalWrite(open 帧)）。

                "terminalAttach" -> {
                    // §3.22：{'terminalId': String} → null（订阅受理）。
                    val args = stringMap(method, call.arguments, "terminalId")
                    val wiring = streams ?: throw streamsNotWired(method, "terminal")
                    val sink = wiring.terminalSink()
                        ?: throw BridgePlatformException(
                            BridgeErrorCodes.HOST_UNAVAILABLE,
                            "$method 无法建流：Dart 尚未订阅 bridge/terminal_events（sink 不在）。" +
                                "订阅时序见 terminal_data.dart 的 open() —— 先订阅再 attach。",
                        )
                    wiring.startTerminalStream(args.getValue("terminalId"), sink)
                    // 等请求 observer 就绪再回「受理」：Dart 紧随其后的
                    // terminalWrite(open 帧) 要有落点（terminal_data.dart:377-386）。
                    wiring.awaitTerminalRequestObserver(STREAM_READY_TIMEOUT_MS)
                    null
                }

                "terminalWrite" -> {
                    // §3.23：TerminalClientMessage wire 字节 → null。
                    val req = parseProto(method, call.arguments) { TerminalClientMessage.parseFrom(it) }
                    val observer = streams?.terminalRequestObserver()
                        ?: throw BridgePlatformException(
                            BridgeErrorCodes.HOST_UNAVAILABLE,
                            "$method 无处可发：宿主侧没有已建的 Attach 流（未 terminalAttach，" +
                                "或流已摘/建流失败）。这是调用时序问题，不是连接问题。",
                        )
                    rpc(method) { observer.onNext(req) }
                    null
                }

                "startTurn" ->
                    // §3.6 unary 首帧方法：Dart 的 PaBridge.startTurn 仍在用
                    // （pa_bridge.dart:373），但它只发首帧、无消费面，与双向流
                    // 六方法是两套入口。本波只接双向流六条；unary 首帧的"发了
                    // 首帧却看不到过程帧"比不接更糟（假象），继续诚实缺席。
                    throw BridgePlatformException(
                        BridgeErrorCodes.HOST_UNAVAILABLE,
                        "方法 startTurn 本波未实现：它是无消费面的 unary 首帧入口，" +
                            "轮次的完整形态是双向流（turnStreamStart/Send/Cancel，" +
                            "见 chat_session.dart）；为避免'发了首帧却看不到任何过程帧'的" +
                            "假象，unary 入口继续留待双向流面稳定后一并取舍",
                    )

                // ── 契约内、本波不实现（如实缺席，不伪造返回）────────────────
                //
                // terminalClose 已实现（见上「终端」节：Map → CloseTerminal
                // unary RPC）。双向流六条已接（见上）。此处仅剩未知方法名。

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
    companion object {
        /**
         * 等双向流请求 observer 就绪的上界（毫秒）。
         *
         * 【为什么是 5s】B4d 的会话协程跑在 IO 上，正常一次线程跳转就到了；5s 覆盖
         * daemon 半死态下 `stub.startTurn` 的挂住，同时不让方法调用无限期占着 IO。
         * 超时走结构化 HOST_UNAVAILABLE（见 [BridgeEventChannelsWiring.awaitObserver]）。
         */
        internal const val STREAM_READY_TIMEOUT_MS: Long = 5_000
    }
}

/**
 * 双向流六方法（turnStreamStart/Send/Cancel + terminalAttach/Write/Close）的
 * 装配 seam：stub 取数 + sink 来源 + 请求 observer 出口。
 *
 * 【为什么需要这一层，而不是 BridgeMethods 直接调 BridgeEventChannels】
 *  B4d 的 `BridgeEventChannels` 把三件关键东西都收在私有字段里：
 *   - **sink**：`turnHandler.onListen(arguments, events)` 收到的那个 Flutter
 *     `EventSink`——框架传入、只活在 handler 里，且 B4d 不对外暴露；
 *   - **请求 observer**：`TurnStreamSession` 协程里 `stub.startTurn(observer)`
 *     的返回值，同样私有；
 *   - **建流/摘流**：`startTurnStream` / `cancelTurnStream` 是 `internal`
 *     （同模块可调，但拿不到实例）。
 *  `BridgeMethods` 的构造只收到 `BridgeBinaryMessenger`（方法面注册面），拿不到
 * 事件面的 `BridgeEventMessenger`，也没有 `BridgeEventChannels` 实例。故收成
 * 可注入 seam：装配侧（installEventChannels 的调用方）实现它并接进来。
 *
 * 【为什么不改 B4d 加 getter】B4d 是已落盘成品（只读纪律）。且加了 getter 也
 * 只解决一半：`BridgeMethods` 仍需一个 `BridgeEventChannels` 实例才能调
 * machinery —— 那个实例的创建点在装配侧（`installEventChannels`），本来就得
 * 经一道注入。seam 把「取 sink / 取 stub / 取 observer / 建摘流」四件事一次
 * 收口，装配侧只需一行 `BridgeStreamWiring.from(channels, stubs, feeder)`。
 *
 * 【为什么是 public 接口】它是 [BridgeMethods] 公开构造器的形参类型（生产装配点
 * 在另一个包：`dev.pocketagent.app.flutterhost`），Kotlin 不允许 public 签名露出
 * internal 类型。实现类 [BridgeEventChannelsWiring] 仍是 internal（只有桥内部与
 * 测试用得到）。
 */
interface BridgeStreamWiring {
    /** 当前 turn 事件面的 sink（Dart 已订阅 bridge/turn_events）；null = 未订阅。 */
    fun turnSink(): BridgeEventSink?

    /** 当前 terminal 事件面的 sink；null = 未订阅。 */
    fun terminalSink(): BridgeEventSink?

    /** 为 [threadId] 建 StartTurn 双向流（内部调 B4d 的 startTurnStream）。 */
    fun startTurnStream(threadId: String, sink: BridgeEventSink)

    /** 摘掉当前 turn 流。 */
    fun cancelTurnStream()

    /** 为 [terminalId] 建 Attach 双向流。 */
    fun startTerminalStream(terminalId: String, sink: BridgeEventSink)

    /** 摘掉当前 terminal 流。 */
    fun cancelTerminalStream()

    /** StartTurn 的请求侧 observer（turnStreamSend 的发帧落点）；null = 无流。 */
    fun turnRequestObserver(): StreamObserver<TurnClientMessage>?

    /** Attach 的请求侧 observer（terminalWrite 的发帧落点）；null = 无流。 */
    fun terminalRequestObserver(): StreamObserver<TerminalClientMessage>?

    /**
     * 等 StartTurn 的请求 observer 就绪（有界，见 [BridgeEventChannelsWiring] 的
     * 「为什么必须等」）。建流方法返回前调用，保证紧随的首帧有落点。
     */
    fun awaitTurnRequestObserver(timeoutMs: Long)

    /** 等 Attach 的请求 observer 就绪（同上）。 */
    fun awaitTerminalRequestObserver(timeoutMs: Long)
}

/**
 * [BridgeStreamWiring] 的生产/测试通用实现：把 `BridgeEventChannels` 实例、
 * daemon async stub、Dart 订阅时收到的 sink 三者接起来。
 *
 * 【请求 observer 为什么拿得到】B4d 的 `startTurnStream(threadId, stub, sink)`
 * 的 `stub` 形参由**调用方**提供（[BridgeThreadStreamService] 是接口，可假
 * 实现），其协程内部 `stub.startTurn(responseObserver)` 的返回值就是请求
 * observer。本类传入记录型装饰器 [RecordingThreadStreamStub]：真 stub 被调，
 * 返回值被留在这里，`turnRequestObserver()` 即可取到——**不改 B4d 一行**。
 *
 * 【生产 stubs 从哪来】`AgentConnection.threadServiceAsync()` /
 * `terminalServiceAsync()`（B4b 已有的 async stub 取数面）。由装配侧包成
 * [BridgeThreadStreamService] / [BridgeTerminalStreamService] 传进来——与
 * `AgentConnectionBridge` 同纪律（薄包转发，不引入第二份 RPC 实现）。
 */
internal class BridgeEventChannelsWiring(
    private val channels: BridgeEventChannels,
    threadStub: BridgeThreadStreamService,
    terminalStub: BridgeTerminalStreamService,
    private val sinkFeeder: StreamSinkFeeder,
) : BridgeStreamWiring {

    private val turnStub = RecordingThreadStreamStub(threadStub)
    private val terminalStubRecording = RecordingTerminalStreamStub(terminalStub)

    override fun turnSink(): BridgeEventSink? = sinkFeeder.turnSink()
    override fun terminalSink(): BridgeEventSink? = sinkFeeder.terminalSink()

    override fun startTurnStream(threadId: String, sink: BridgeEventSink) {
        channels.startTurnStream(threadId, turnStub, sink)
    }

    override fun cancelTurnStream() {
        // 先摘 B4d 的会话（它会 cancel 掉 gRPC 流），再清本地 observer 引用：
        // 顺序反了会让「已摘的流」仍能被 turnStreamSend 打到。
        channels.cancelTurnStream()
        turnStub.requestObserver = null
    }

    override fun startTerminalStream(terminalId: String, sink: BridgeEventSink) {
        channels.startTerminalStream(terminalId, terminalStubRecording, sink)
    }

    override fun cancelTerminalStream() {
        channels.cancelTerminalStream()
        terminalStubRecording.requestObserver = null
    }

    override fun turnRequestObserver(): StreamObserver<TurnClientMessage>? =
        turnStub.requestObserver

    override fun terminalRequestObserver(): StreamObserver<TerminalClientMessage>? =
        terminalStubRecording.requestObserver

    /**
     * 等 B4d 的会话协程把 StartTurn 的请求 observer 交出来（有界）。
     *
     * 【为什么必须等】Dart 的时序是 `await turnStreamStart(...)` 紧接
     * `await turnStreamSend(首帧)`（chat_session.dart:152-153）。而 B4d 的
     * `TurnStreamSession` 是在自己的 IO scope 里 `launch` 后调
     * `stub.startTurn(...)`——建流方法返回时那个协程未必已跑到。不等待就回
     * 「受理」，紧随的首帧会打到空 observer 上（"无处可发"的假错误）。
     *
     * 【为什么有界】`stub.startTurn` 理论上可挂住（daemon 半死态）；无界等待会
     * 把方法调用挂在 IO 线程上。超时即抛结构化错误，由 Dart 侧的错误分支处置。
     */
    override fun awaitTurnRequestObserver(timeoutMs: Long) {
        awaitObserver({ turnStub.ready() }, timeoutMs, "turnStreamStart")
    }

    override fun awaitTerminalRequestObserver(timeoutMs: Long) {
        awaitObserver({ terminalStubRecording.ready() }, timeoutMs, "terminalAttach")
    }

    /**
     * 轮询等 observer 就绪。
     *
     * 【为什么睡而不是 yield】调用线程是 `Dispatchers.IO`（dispatch 的
     * withContext），`Thread.yield()` 不承诺让**另一个** IO 线程的协程前进。
     */
    private fun awaitObserver(ready: () -> Boolean, timeoutMs: Long, what: String) {
        val deadline = System.currentTimeMillis() + timeoutMs
        while (!ready()) {
            if (System.currentTimeMillis() > deadline) {
                throw BridgePlatformException(
                    BridgeErrorCodes.HOST_UNAVAILABLE,
                    "$what 的请求流未就绪：daemon 侧 ${timeoutMs}ms 内没有把流交出来" +
                        "（通道未建/半死/daemon 无应答）。",
                )
            }
            Thread.sleep(5)
        }
    }

    companion object
}

/** 记录型 StartTurn stub：转调真 stub，留痕返回的请求 observer。 */
private class RecordingThreadStreamStub(
    private val delegate: BridgeThreadStreamService,
) : BridgeThreadStreamService {
    @Volatile var requestObserver: StreamObserver<TurnClientMessage>? = null

    /**
     * observer 是否已就绪（B4d 的协程已调过 [startTurn]）。
     *
     * 【为什么要这个信号】建流方法返回时 B4d 的会话协程可能还没跑到
     * `stub.startTurn`——方法面据此等待，保证紧随的首帧有落点。
     */
    fun ready(): Boolean = requestObserver != null

    override fun startTurn(
        responseObserver: StreamObserver<pocketagent.v1.TurnServerMessage>,
    ): StreamObserver<TurnClientMessage> {
        val observer = delegate.startTurn(responseObserver)
        requestObserver = observer
        return observer
    }
}

/** 记录型 Attach stub：同上（terminal 侧）。 */
private class RecordingTerminalStreamStub(
    private val delegate: BridgeTerminalStreamService,
) : BridgeTerminalStreamService {
    @Volatile var requestObserver: StreamObserver<TerminalClientMessage>? = null

    /** observer 是否已就绪（见 [RecordingThreadStreamStub.ready]）。 */
    fun ready(): Boolean = requestObserver != null

    override fun attach(
        responseObserver: StreamObserver<pocketagent.v1.TerminalServerMessage>,
    ): StreamObserver<TerminalClientMessage> {
        val observer = delegate.attach(responseObserver)
        requestObserver = observer
        return observer
    }
}

/**
 * 「Dart 订阅时收到的 sink」的喂入面（[BridgeEventChannelsWiring] 的协作方）。
 *
 * 生产由装配侧实现：`installEventChannels` 建出 `BridgeEventChannels` 后，把
 * `turnHandler.onListen(arguments, events)` 的 `events` 交到这里。B4d 的 handler
 * 是私有匿名对象、没有出口，装配侧要观察 listen 动作只能经自己包的那层
 * messenger——接线图与协调项登记见
 * `docs/reviews/2026-10-16/b4g-bidi-methods.md`。
 *
 * 测试直接摆值（fake feeder）。
 *
 * 【为什么不是 fun interface】两个查询方法（turn/terminal）语义成对但返回不同类型
 * 的 seam，合成一个 SAM 会逼其中一方返回 `Any?`——那是把类型安全换成 convenience。
 */
internal interface StreamSinkFeeder {
    fun turnSink(): BridgeEventSink?
    fun terminalSink(): BridgeEventSink?
}

/** [BridgeStreamWiring] 缺失时的统一错误（六方法共用：点名缺什么，不模糊）。 */
private fun streamsNotWired(method: String, kind: String): BridgePlatformException =
    BridgePlatformException(
        BridgeErrorCodes.HOST_UNAVAILABLE,
        "方法 $method 本波未接线：${kind}双向流的装配 seam（BridgeStreamWiring）" +
            "未注入 BridgeMethods。建流需要两样东西——daemon 的 async stub 与" +
            "Dart 订阅事件面时收到的 sink；后者只活在 B4d 的 handler 里，" +
            "需由装配侧接进来（见 b4g-bidi-methods.md 协调项登记）",
    )

// ── 生产装配：BridgeStreamWiring 的一行接法 ────────────────────────────────

/**
 * 生产 stub：`AgentConnection.threadServiceAsync()` → [BridgeThreadStreamService]。
 *
 * 【为什么不走 BridgeConnection】B4b 的窄接口只覆盖 24 个已实现方法用到的
 * blocking RPC；两个 async stub 是双向流才有的取数需求，而 BridgeConnection.kt
 * 本波只读不改（文件清单纪律）。与 `AgentConnectionBridge` 同纪律：在此薄包
 * 转发，不引入第二份 RPC 实现、不新增缓存。
 *
 * 【async stub 为什么不设 deadline】双向流按事件长期存活，整条流设 deadline
 * 会把它定时杀死（AgentConnection.watchCollabSession 的同款注释）。断线退避
 * 重订由 Dart 侧重订阅触发（契约 §4）。
 */
internal fun agentThreadStreamStub(): BridgeThreadStreamService =
    object : BridgeThreadStreamService {
        override fun startTurn(
            responseObserver: StreamObserver<pocketagent.v1.TurnServerMessage>,
        ): StreamObserver<TurnClientMessage> =
            dev.pocketagent.app.AgentConnection.threadServiceAsync()
                .startTurn(responseObserver)
    }

/** 生产 stub：`AgentConnection.terminalServiceAsync()` → [BridgeTerminalStreamService]。 */
internal fun agentTerminalStreamStub(): BridgeTerminalStreamService =
    object : BridgeTerminalStreamService {
        override fun attach(
            responseObserver: StreamObserver<pocketagent.v1.TerminalServerMessage>,
        ): StreamObserver<TerminalClientMessage> =
            dev.pocketagent.app.AgentConnection.terminalServiceAsync()
                .attach(responseObserver)
    }

/**
 * 观察「Dart 订阅 turn_events / terminal_events」的 messenger 装饰器。
 *
 * 【它解决什么】B4d 的 `turnHandler.onListen(arguments, events)` 是私有匿名
 * 对象，装配侧拿不到那个 `events`（真 sink）。而 `BridgeEventChannels` 的
 * handler 是经 `messenger.setStreamHandler(name, handler)` 注册的——装饰这层
 * messenger 即可在注册时把 B4d 的 handler 包一层：Dart 订阅进来时，本装饰器
 * 先收到 `onListen(arguments, events)`，把 `events` 记进 [feed]，再转调 B4d
 * 的原 handler。**不改 B4d 一行**，sink 就交了出来的。
 *
 * 【为什么不两个都包】turn 与 terminal 两条通道的 handler 都要包（各记各的
 * sink）；主事件面（bridge/events）不包（它与双向流无关，原样转发）。
 *
 * 【线程】onListen/onCancel 由 Flutter 框架在平台线程调用；[feed] 的写入用
 * @Volatile + 同步块，读取在 IO 上跑（dispatch 里）——无锁竞争。
 */
internal class SinkObservingMessenger(
    private val delegate: BridgeEventMessenger,
    private val feed: MutableStreamSinkFeed,
) : BridgeEventMessenger {
    override fun setStreamHandler(channelName: String, handler: BridgeEventStreamHandler) {
        val wrapped: BridgeEventStreamHandler = when (channelName) {
            BRIDGE_TURN_EVENT_CHANNEL_NAME -> object : BridgeEventStreamHandler {
                override fun onListen(arguments: Any?, events: BridgeEventSink) {
                    feed.turnSink = events
                    handler.onListen(arguments, events)
                }

                override fun onCancel(arguments: Any?) {
                    feed.turnSink = null
                    handler.onCancel(arguments)
                }
            }

            BRIDGE_TERMINAL_EVENT_CHANNEL_NAME -> object : BridgeEventStreamHandler {
                override fun onListen(arguments: Any?, events: BridgeEventSink) {
                    feed.terminalSink = events
                    handler.onListen(arguments, events)
                }

                override fun onCancel(arguments: Any?) {
                    feed.terminalSink = null
                    handler.onCancel(arguments)
                }
            }

            else -> handler
        }
        delegate.setStreamHandler(channelName, wrapped)
    }
}

/** [StreamSinkFeeder] 的可写实现（[SinkObservingMessenger] 写、wiring 读）。 */
internal class MutableStreamSinkFeed : StreamSinkFeeder {
    @Volatile var turnSink: BridgeEventSink? = null
    @Volatile var terminalSink: BridgeEventSink? = null

    override fun turnSink(): BridgeEventSink? = turnSink
    override fun terminalSink(): BridgeEventSink? = terminalSink
}

/**
 * 生产装配入口：把 `BridgeEventChannels` 实例接成 [BridgeStreamWiring]。
 *
 * ```
 * val feed = MutableStreamSinkFeed()
 * val channels = BridgeEventChannels(SinkObservingMessenger(inner, feed), sources, log)
 * val wiring = BridgeEventChannelsWiring(
 *     channels, agentThreadStreamStub(), agentTerminalStreamStub(), feed,
 * )
 * BridgeMethods(FlutterBinaryMessengerAdapter(messenger), connection, wiring)
 * ```
 *
 * 【为什么装配侧要包 messenger】见 [SinkObservingMessenger]：B4d 的 handler 私有，
 * sink 只能在这一层观察。`installEventChannels` 目前自己建
 * `FlutterEventMessengerAdapter`，故这一步要由它的调用方（flutterhost 的
 * installChannelShell）做——**协调项登记**见
 * `docs/reviews/2026-10-16/b4g-bidi-methods.md`。
 */

/**
 * 生产装配入口：方法面 + 事件面 + 双向流 wiring 三件套一次装好（B4g 协调项 C1 的落点）。
 *
 * 【为什么需要它】[BridgeEventChannelsWiring] 与内部版 [installEventChannels] 都是
 * internal（只 bridge 包内可见），而调用方 `dev.pocketagent.app.flutterhost` 在另一
 * 个包——装配逻辑（internal 件的组合）必须留在本包，对外只露这一个 public 函数。
 * B4g 报告把这称为「协调项 C1：需装配侧落地」；本函数即其落地。
 *
 * 【装配顺序（鸡生蛋的解法）】wiring 持 channels（startTurnStream 要调它的
 * machinery），methods 持 wiring（六方法的 dispatch 要经它）——顺序只能是
 * channels → wiring → methods。故不用封装版 [installEventChannels]（它内部
 * 自建 channels 并返回，wiring 无从插入），直接用 internal 两段版先建 channels，
 * 再建 wiring，最后建 methods。
 *
 * 【三件套的共享纪律】方法面与事件面**共享同一个 connection 实例**：B4f 的
 * host_info 帧只能来自「经本实例的 updateHostInfo 成功路径」
 * （[EventFeedingConnection] 装饰），两个连接各走各的话 host_info 帧将永远静默。
 * 调用方（flutterhost）传进来的 connection 会被原样用于两路，本函数不另建。
 *
 * 【sink 从哪来】[SinkObservingMessenger] 装饰事件面的 messenger：Dart 订阅
 * turn_events / terminal_events 时，onListen 收到的真 sink 记进 [MutableStreamSinkFeed]，
 * wiring 经它取——不改 B4d 一行（其 handler 私有，见 B4g 报告 §核心设计）。
 *
 * @return 装配出的 [BridgeEventChannels]（调用方持有供 shutdown 时停协程防泄漏）。
 */
fun installBridge(
    messenger: BinaryMessenger,
    connection: BridgeConnection = AgentConnectionBridge(),
    log: BridgeLog = AndroidLog,
): BridgeEventChannels {
    // ① 事件面 channels（internal 两段版：messenger seam + sources）
    val sources: BridgeEventSources = when (connection) {
        is BridgeEventSources -> connection
        else -> AgentEventSources(connection)
    }
    val feed = MutableStreamSinkFeed()
    val channels = BridgeEventChannels(
        SinkObservingMessenger(FlutterEventMessengerAdapter(messenger), feed),
        sources,
        log,
    )
    // ② 双向流 wiring（持 channels 与两个生产 async stub）
    val wiring = BridgeEventChannelsWiring(
        channels = channels,
        threadStub = agentThreadStreamStub(),
        terminalStub = agentTerminalStreamStub(),
        sinkFeeder = feed,
    )
    // ③ 方法面（持 wiring：六条双向流方法的 dispatch 经它走真 gRPC）
    BridgeMethods(FlutterBinaryMessengerAdapter(messenger), connection, wiring)
    return channels
}
