package dev.pocketagent.app.bridge

import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.CancelCollabMemberRequest
import pocketagent.v1.CancelCollabSessionRequest
import pocketagent.v1.CollabMember
import pocketagent.v1.CollabRole
import pocketagent.v1.ContentBlock
import pocketagent.v1.DeleteCollabRoleRequest
import pocketagent.v1.DispatchCollabRequest
import pocketagent.v1.GetCollabSessionRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.GetThreadEventsSinceRequest
import pocketagent.v1.GetThreadHistoryRequest
import pocketagent.v1.HostInfo
import pocketagent.v1.ListBranchesRequest
import pocketagent.v1.ListCollabRolesRequest
import pocketagent.v1.ListCollabSessionsRequest
import pocketagent.v1.ListDirectoryRequest
import pocketagent.v1.ListSlashCommandsRequest
import pocketagent.v1.ListThreadsRequest
import pocketagent.v1.OpenTerminal
import pocketagent.v1.ReadFileRequest
import pocketagent.v1.RetryCollabMemberRequest
import pocketagent.v1.SetCollabRoleRequest
import pocketagent.v1.TerminalClientMessage
import pocketagent.v1.TerminalInput
import pocketagent.v1.TerminalResize
import pocketagent.v1.TextBlock
import pocketagent.v1.ThreadMessage
import pocketagent.v1.TurnClientMessage
import pocketagent.v1.TurnInput
import pocketagent.v1.TurnSupplement

/**
 * `BridgeMethods` 方法面对账测试：请求解析 → stub 调用 → 响应**字节级**断言。
 *
 * 【为什么全部字节级】契约 §2.3「返回值一律 proto wire 字节」——Dart 侧
 * `bytesOrThrow` 反序列化后按字段用。若桥在中间做了投影/补默认值/重排，
 * 字节级断言立刻暴露；字段级相等但字节不同也说明"编解码路径与 daemon 不同源"。
 *
 * 【为什么 fake 直接返回构建好的 proto】桥的 parseFrom → RPC → toByteArray 全链
 * 被真实执行，唯一的假点是"daemon 返回什么"——那正是要摆布的变量。
 */
class BridgeMethodsTest {

    // ── 装配 ──────────────────────────────────────────────────────────────

    private fun harness(
        conn: FakeConnection = FakeConnection(),
    ): Triple<BridgeMethods, RecordingResult, FakeConnection> {
        val messenger = RecordingMessenger()
        val methods = BridgeMethods(messenger, conn)
        // 构造即注册 handler（生产由 Flutter 的 MethodChannel 触发）。
        assertNotNull("构造时必须注册 handler", messenger.handler)
        return Triple(methods, RecordingResult(), conn)
    }

    /** 走 handler 全路径（launch + withContext(IO)），同步等结果。 */
    private fun RecordingResult.invoke(method: String, arguments: Any?): RecordingResult {
        val (methods, _, _) = harness()
        methods.handle(BridgeMethodCall(method, arguments), this)
        awaitSettled()
        return this
    }

    /**
     * 等 handler 的协程落地。
     *
     * 【为什么不用 Thread.sleep 固定值】协程在 Dispatchers.IO 上跑，固定睡眠要么
     * 不够（偶发红）要么浪费。这里用「结果已回 或 超时」轮询：生产路径本身同步
     * （fake 无阻塞），通常一个 yield 就够。
     */
    private fun RecordingResult.awaitSettled() {
        val deadline = System.currentTimeMillis() + 5_000
        while (!successCalled && !errorCalled && !notImplementedCalled) {
            if (System.currentTimeMillis() > deadline) {
                throw AssertionError("handler 5s 未回结果：协程卡死或结果回调没被调用")
            }
            Thread.yield()
        }
    }

    /** 便捷断言：方法回 proto 字节，且反序列化后字段等于期望。 */
    private fun RecordingResult.assertBytes(method: String): ByteArray {
        awaitSettled()
        assertTrue("$method 应走 success，实际 error=$errorCode/$errorMessage", successCalled)
        val bytes = success as? ByteArray
        assertNotNull("$method 返回值须为 proto wire 字节，实际 ${success?.javaClass?.simpleName}", bytes)
        return bytes!!
    }

    private fun RecordingResult.assertError(code: String) {
        awaitSettled()
        assertTrue("应走 error，实际 success=$success", errorCalled)
        assertEquals(code, errorCode)
    }

    // ── §3.1 getHostInfo ──────────────────────────────────────────────────

    @Test
    fun `getHostInfo 解析空消息字节并回传 HostInfo wire 字节`() = runBlocking {
        val conn = FakeConnection()
        conn.host.hostInfo = hostInfo(capabilities = arrayOf("terminal", "fs", "agent:pi"))
        val (methods, result, _) = harness(conn)

        methods.handle(BridgeMethodCall("getHostInfo", GetHostInfoRequest.newBuilder().build().toByteArray()), result)
        result.awaitSettled()

        val bytes = result.assertBytes("getHostInfo")
        val parsed = HostInfo.parseFrom(bytes)
        assertEquals("host-1", parsed.hostId)
        assertEquals("YANZI-PC", parsed.displayName)
        assertEquals(listOf("terminal", "fs", "agent:pi"), parsed.capabilitiesList)
    }

    @Test
    fun `getHostInfo 成功路径写 capabilities 共享快照_与探活循环同口径`() = runBlocking {
        val conn = FakeConnection()
        val info = hostInfo(capabilities = arrayOf("agent:zcode"))
        conn.host.hostInfo = info
        val (methods, result, _) = harness(conn)

        methods.handle(BridgeMethodCall("getHostInfo", GetHostInfoRequest.getDefaultInstance().toByteArray()), result)
        result.awaitSettled()

        // AgentConnection.updateHostInfo 的 KDoc：所有成功路径写共享快照。
        // 桥的 getHostInfo 也是成功路径，漏写会让某条渲染链按空 capabilities 渲染。
        assertEquals(listOf(info), conn.updateHostInfoCalls)
    }

    @Test
    fun `getHostInfo 入参不是字节时回 DECODE_FAILED_不补默认值`() = runBlocking {
        val (methods, result, _) = harness()
        methods.handle(BridgeMethodCall("getHostInfo", "not-bytes"), result)
        result.assertError(BridgeErrorCodes.DECODE_FAILED)
        assertTrue(result.errorMessage!!.contains("getHostInfo"))
    }

    @Test
    fun `getHostInfo 入参是坏字节时回 DECODE_FAILED_双端protobuf版本漂移显式失败`() = runBlocking {
        val (methods, result, _) = harness()
        // 合法 varint 后接非法 wire type 的残字节，parseFrom 必炸。
        methods.handle(BridgeMethodCall("getHostInfo", byteArrayOf(0x08, 0xFF.toByte(), 0xFF.toByte(), 0xFF.toByte())), result)
        result.assertError(BridgeErrorCodes.DECODE_FAILED)
        assertTrue(result.errorMessage!!.contains("不可解析"))
    }

    @Test
    fun `getHostInfo daemon 不可达时回 NOT_CONNECTED_UNAVAILABLE 映射`() = runBlocking {
        val conn = FakeConnection()
        conn.host.failure = grpcError(io.grpc.Status.Code.UNAVAILABLE, "transport 半死")
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("getHostInfo", ByteArray(0)), result)
        result.assertError(BridgeErrorCodes.NOT_CONNECTED)
        assertEquals("transport 半死", result.errorMessage)
    }

    // ── §3.2~§3.4 connect / disconnect / getConnectionState ───────────────

    @Test
    fun `connect 受理即回 null_地址与 token 原样进连接层`() = runBlocking {
        val conn = FakeConnection()
        val (methods, result, _) = harness(conn)
        methods.handle(
            BridgeMethodCall(
                "connect",
                mapOf("addr" to "192.168.1.5:7246", "token" to "tok-123"),
            ),
            result,
        )
        result.awaitSettled()
        assertTrue(result.successCalled)
        // §3.2：null = 请求受理，不是"已连上"。
        assertNull(result.success)
        assertEquals(listOf("192.168.1.5:7246" to "tok-123"), conn.connectCalls)
    }

    @Test
    fun `connect 空地址在宿主侧同样即拒_INVALID_ADDR_与Dart本地拒同口径`() = runBlocking {
        val (methods, result, _) = harness()
        methods.handle(BridgeMethodCall("connect", mapOf("addr" to "  ", "token" to "")), result)
        result.assertError(BridgeErrorCodes.INVALID_ADDR)
    }

    @Test
    fun `connect 入参缺 token 键时回 DECODE_FAILED_不补空串`() = runBlocking {
        val (methods, result, _) = harness()
        methods.handle(BridgeMethodCall("connect", mapOf("addr" to "1.2.3.4:7246")), result)
        result.assertError(BridgeErrorCodes.DECODE_FAILED)
        assertTrue(result.errorMessage!!.contains("token"))
    }

    @Test
    fun `disconnect 回 null 并触发连接层断开`() = runBlocking {
        val conn = FakeConnection()
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("disconnect", null), result)
        result.awaitSettled()
        assertTrue(result.successCalled)
        assertNull(result.success)
        assertEquals(1, conn.disconnectCalls)
    }

    @Test
    fun `disconnect 收到非 null 入参时回 DECODE_FAILED_契约要求 null`() = runBlocking {
        val (methods, result, _) = harness()
        methods.handle(BridgeMethodCall("disconnect", mapOf("x" to 1)), result)
        result.assertError(BridgeErrorCodes.DECODE_FAILED)
    }

    @Test
    fun `getConnectionState 五键恒有值_键名等于Kotlin字段名`() = runBlocking {
        val conn = FakeConnection(
            connState = "ok",
            connTransport = "USB 通道",
            connAddress = "127.0.0.1:7246",
            connectionEpoch = 7,
            hostCapabilities = listOf("terminal", "agent:pi"),
        )
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("getConnectionState", null), result)
        result.awaitSettled()

        @Suppress("UNCHECKED_CAST")
        val map = result.success as? Map<String, Any>
        assertNotNull("返回值须为 Map", map)
        // §3.4 五键（校验顺序 = 契约表键序）。
        assertEquals("ok", map!!["connState"])
        assertEquals("USB 通道", map["connTransport"])
        assertEquals("127.0.0.1:7246", map["connAddress"])
        assertEquals(7, map["connectionEpoch"])
        assertEquals(listOf("terminal", "agent:pi"), map["hostCapabilities"])
        // 不多不少正好五个：多键 = 契约漂移（Dart 的 PaConnState 只认这五个）。
        assertEquals(5, map.size)
    }

    @Test
    fun `getConnectionState 不依赖连接_快照读取本身不会因断连失败`() = runBlocking {
        // §3.4 明说：快照读取不依赖连接；connState="down" 时五键仍须有值。
        val conn = FakeConnection(
            connState = "down",
            connTransport = "",
            connAddress = "",
            connectionEpoch = 0,
            hostCapabilities = emptyList(),
        )
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("getConnectionState", null), result)
        result.awaitSettled()
        @Suppress("UNCHECKED_CAST")
        val map = result.success as Map<String, Any>
        assertEquals("down", map["connState"])
        assertEquals(emptyList<String>(), map["hostCapabilities"])
    }

    // ── §3.5 pairCode ─────────────────────────────────────────────────────

    @Test
    fun `pairCode 回非空配对码字符串`() = runBlocking {
        val conn = FakeConnection()
        conn.host.pairing = pairingResponse("12345678")
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("pairCode", null), result)
        result.awaitSettled()
        assertTrue(result.successCalled)
        assertEquals("12345678", result.success)
    }

    @Test
    fun `pairCode 配对未开启时回 PAIR_DISABLED_不用空返回代替错误`() = runBlocking {
        // §3.5：来源不允许/未开启必须抛 PAIR_DISABLED。insecure-no-auth 模式下
        // daemon 如实回 pairing_enabled=false（host.go:87）。
        val conn = FakeConnection()
        conn.host.pairing = pairingResponse("", enabled = false)
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("pairCode", null), result)
        result.assertError(BridgeErrorCodes.PAIR_DISABLED)
    }

    @Test
    fun `pairCode 非回环来源 PermissionDenied 映射 PAIR_DISABLED_而非通用表`() = runBlocking {
        // daemon host.go:83-85：非回环来源一律拒绝（配对码绝不 remote 泄露）。
        // 同一 PERMISSION_DENIED 在其他方法落 INTERNAL，这里必须落 PAIR_DISABLED。
        val conn = FakeConnection()
        conn.host.failure = grpcError(
            io.grpc.Status.Code.PERMISSION_DENIED,
            "配对码仅对本机回环(USB)来源提供;请用 USB 连接或在电脑屏幕上查看配对码",
        )
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("pairCode", null), result)
        result.assertError(BridgeErrorCodes.PAIR_DISABLED)
        assertTrue(result.errorMessage!!.contains("回环"))
    }

    // ── §3.7 listThreads ──────────────────────────────────────────────────

    @Test
    fun `listThreads 空消息请求原样过桥_响应字节反序列化一致`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.listThreads = listThreadsResponse("t1", "t2")
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("listThreads", ByteArray(0)), result)
        val bytes = result.assertBytes("listThreads")

        // 字节级：桥的 toByteArray 与 fake 持有的 proto 逐字节相同。
        assertArrayEquals(conn.thread.listThreads!!.toByteArray(), bytes)
        assertEquals(listOf("t1", "t2"), pocketagent.v1.ListThreadsResponse.parseFrom(bytes).threadsList.map { it.id })
    }

    @Test
    fun `listThreads 请求字段原样到达_桥不做投影筛选`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.listThreads = listThreadsResponse("t1")
        val (methods, result, _) = harness(conn)
        val request = ListThreadsRequest.newBuilder()
            .setIncludeArchived(true)
            .setLimit(20)
            .setSearchTerm("bug")
            .setSortAscending(true)
            .build()
        methods.handle(BridgeMethodCall("listThreads", request.toByteArray()), result)
        result.awaitSettled()

        val seen = conn.thread.lastListThreadsRequest
        assertNotNull("stub 必须收到请求", seen)
        assertTrue(seen!!.includeArchived)
        assertEquals(20, seen.limit)
        assertEquals("bug", seen.searchTerm)
        assertTrue(seen.sortAscending)
    }

    // ── §3.8 listDirectory ────────────────────────────────────────────────

    @Test
    fun `listDirectory 响应字节级一致`() = runBlocking {
        val conn = FakeConnection()
        conn.file.listDirectory = listDirectoryResponse("src/", "README.md")
        val (methods, result, _) = harness(conn)
        val req = ListDirectoryRequest.newBuilder().setPath("/ws").setIncludeHidden(true).build()
        methods.handle(BridgeMethodCall("listDirectory", req.toByteArray()), result)
        val bytes = result.assertBytes("listDirectory")
        assertArrayEquals(conn.file.listDirectory!!.toByteArray(), bytes)
        val parsed = pocketagent.v1.ListDirectoryResponse.parseFrom(bytes)
        assertEquals(2, parsed.entriesCount)
        assertTrue(parsed.getEntries(0).isDir)
        assertFalse(parsed.getEntries(1).isDir)
    }

    // ── §3.9 readFile（首块语义）──────────────────────────────────────────

    @Test
    fun `readFile 只回首块_后续块被丢弃`() = runBlocking {
        val conn = FakeConnection()
        conn.file.readFileChunks = listOf(
            fileChunk("第一块内容", totalSize = 20),
            fileChunk("第二块内容", totalSize = 20),
            fileChunk("第三块内容", totalSize = 20),
        )
        val (methods, result, _) = harness(conn)
        methods.handle(
            BridgeMethodCall("readFile", ReadFileRequest.newBuilder().setPath("/ws/a.txt").build().toByteArray()),
            result,
        )
        val bytes = result.assertBytes("readFile")
        val chunk = pocketagent.v1.FileChunk.parseFrom(bytes)
        assertEquals("第一块内容", chunk.data.toStringUtf8())
        assertEquals(20L, chunk.totalSize)
        assertTrue(chunk.text)
    }

    @Test
    fun `readFile 空流回 INTERNAL_不拿空消息冒充首块`() = runBlocking {
        // 契约首块必带 total_size/text；daemon 一个块都不给 = 违约形态，
        // 不能回空 FileChunk 让 Dart 侧以为"文件为空"。
        val conn = FakeConnection()
        conn.file.readFileChunks = emptyList()
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("readFile", ByteArray(0)), result)
        result.assertError(BridgeErrorCodes.INTERNAL)
        assertTrue(result.errorMessage!!.contains("空的分块流"))
    }

    // ── §3.10 listBranches ────────────────────────────────────────────────

    @Test
    fun `listBranches 响应字节级一致_当前分支标记透传`() = runBlocking {
        val conn = FakeConnection()
        conn.git.listBranches = listBranchesResponse("main", "feature/x", "origin/main")
        val (methods, result, _) = harness(conn)
        val req = ListBranchesRequest.newBuilder().setRepo("/ws").build()
        methods.handle(BridgeMethodCall("listBranches", req.toByteArray()), result)
        val bytes = result.assertBytes("listBranches")
        assertArrayEquals(conn.git.listBranches!!.toByteArray(), bytes)
        val parsed = pocketagent.v1.ListBranchesResponse.parseFrom(bytes)
        assertEquals(3, parsed.branchesCount)
        assertTrue(parsed.getBranches(0).isCurrent)
        assertFalse(parsed.getBranches(1).isCurrent)
    }

    // ── §3.11~§3.13 协同角色三条 ──────────────────────────────────────────

    @Test
    fun `listCollabRoles 空消息字节可解析_响应字节级一致`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.listCollabRoles = listCollabRolesResponse(collabRole("reviewer"), collabRole("writer"))
        val (methods, result, _) = harness(conn)
        // Dart 发 ListCollabRolesRequest().writeToBuffer() = 空字节数组（不是 null）。
        methods.handle(BridgeMethodCall("listCollabRoles", ByteArray(0)), result)
        val bytes = result.assertBytes("listCollabRoles")
        assertArrayEquals(conn.thread.listCollabRoles!!.toByteArray(), bytes)
        assertEquals(listOf("reviewer", "writer"), pocketagent.v1.ListCollabRolesResponse.parseFrom(bytes).rolesList.map { it.id })
    }

    @Test
    fun `setCollabRole 请求八字段原样过桥`() = runBlocking {
        val conn = FakeConnection()
        val role = CollabRole.newBuilder()
            .setId("reviewer")
            .setName("安全审查")
            .setAgentId("codex")
            .setDescription("只读审查")
            .setInstructions("先看 diff")
            .setModel("gpt-5.3")
            .setEffort("high")
            .setWorkspace("/ws")
            .build()
        conn.thread.setCollabRole = setCollabRoleResponse(role)
        val (methods, result, _) = harness(conn)
        methods.handle(
            BridgeMethodCall("setCollabRole", SetCollabRoleRequest.newBuilder().setRole(role).build().toByteArray()),
            result,
        )
        val bytes = result.assertBytes("setCollabRole")
        val seen = conn.thread.lastSetCollabRoleRequest
        assertEquals("reviewer", seen!!.role.id)
        assertEquals("/ws", seen.role.workspace)
        // 响应是"落库后读回来的真值"，字节须与 daemon 回的完全一致。
        assertArrayEquals(conn.thread.setCollabRole!!.toByteArray(), bytes)
    }

    @Test
    fun `deleteCollabRole 请求 id 原样过桥_响应可为空消息`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.deleteCollabRole = deleteCollabRoleResponse()
        val (methods, result, _) = harness(conn)
        methods.handle(
            BridgeMethodCall("deleteCollabRole", DeleteCollabRoleRequest.newBuilder().setId("writer").build().toByteArray()),
            result,
        )
        val bytes = result.assertBytes("deleteCollabRole")
        assertEquals("writer", conn.thread.lastDeleteCollabRoleRequest!!.id)
        // DeleteCollabRoleResponse 是空消息：writeToBuffer 长度 0（合法返回）。
        assertEquals(0, bytes.size)
    }

    // ── §3.14~§3.19 协同派发/快照/写动作 ──────────────────────────────────

    @Test
    fun `dispatchCollab 请求成员顺序原样过桥`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.dispatchCollab = dispatchCollabResponse("sess-1")
        val (methods, result, _) = harness(conn)
        val request = DispatchCollabRequest.newBuilder()
            .setInstruction("一起 review 这个 PR")
            .addMembers(CollabMember.newBuilder().setAgentId("codex"))
            .addMembers(CollabMember.newBuilder().setAgentId("pi").setRoleId("reviewer"))
            .build()
        methods.handle(BridgeMethodCall("dispatchCollab", request.toByteArray()), result)
        val bytes = result.assertBytes("dispatchCollab")

        val seen = conn.thread.lastDispatchCollabRequest!!
        assertEquals("一起 review 这个 PR", seen.instruction)
        assertEquals(listOf("codex", "pi"), seen.membersList.map { it.agentId })
        assertEquals("reviewer", seen.getMembers(1).roleId)
        assertEquals("sess-1", pocketagent.v1.DispatchCollabResponse.parseFrom(bytes).sessionId)
    }

    @Test
    fun `getCollabSession 按字节收参_Dart侧实际发请求消息而非Map`() = runBlocking {
        // §3 表写 arguments={'sessionId': String}，但 Dart 代码发的是
        // GetCollabSessionRequest 字节 —— 实现以 Dart 为准（契约 §2.1）。
        val conn = FakeConnection()
        conn.thread.getCollabSession = collabSessionSnapshot("sess-9")
        val (methods, result, _) = harness(conn)
        methods.handle(
            BridgeMethodCall("getCollabSession", GetCollabSessionRequest.newBuilder().setSessionId("sess-9").build().toByteArray()),
            result,
        )
        val bytes = result.assertBytes("getCollabSession")
        assertEquals("sess-9", conn.thread.lastGetCollabSessionRequest!!.sessionId)
        assertEquals("sess-9", pocketagent.v1.CollabSessionSnapshot.parseFrom(bytes).sessionId)
    }

    @Test
    fun `getCollabSession 若收到 Map 形态回 DECODE_FAILED_不做双形态二分`() = runBlocking {
        // 红线：不做 null/bytes 两种形态的版本二分（§2.1）。旧表里的 Map 形态
        // 到达即契约漂移，显式失败。
        val (methods, result, _) = harness()
        methods.handle(BridgeMethodCall("getCollabSession", mapOf("sessionId" to "s1")), result)
        result.assertError(BridgeErrorCodes.DECODE_FAILED)
    }

    @Test
    fun `listCollabSessions includeFinished 原样过桥`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.listCollabSessions = listCollabSessionsResponse("s1", "s2")
        val (methods, result, _) = harness(conn)
        val req = ListCollabSessionsRequest.newBuilder().setIncludeFinished(true).build()
        methods.handle(BridgeMethodCall("listCollabSessions", req.toByteArray()), result)
        val bytes = result.assertBytes("listCollabSessions")
        assertTrue(conn.thread.lastListCollabSessionsRequest!!.includeFinished)
        assertEquals(2, pocketagent.v1.ListCollabSessionsResponse.parseFrom(bytes).sessionsCount)
    }

    @Test
    fun `cancelCollabSession 回收敛后的会话快照字节`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.cancelCollabSession = collabSessionSnapshot("s5")
        val (methods, result, _) = harness(conn)
        methods.handle(
            BridgeMethodCall("cancelCollabSession", CancelCollabSessionRequest.newBuilder().setSessionId("s5").build().toByteArray()),
            result,
        )
        val bytes = result.assertBytes("cancelCollabSession")
        assertEquals("s5", conn.thread.lastCancelCollabSessionRequest!!.sessionId)
        assertEquals("s5", pocketagent.v1.CollabSessionSnapshot.parseFrom(bytes).sessionId)
    }

    @Test
    fun `cancelCollabMember sessionId加threadId 双定位原样过桥`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.cancelCollabMember = collabMemberSnapshot("th-1")
        val (methods, result, _) = harness(conn)
        methods.handle(
            BridgeMethodCall(
                "cancelCollabMember",
                CancelCollabMemberRequest.newBuilder().setSessionId("s5").setThreadId("th-1").build().toByteArray(),
            ),
            result,
        )
        val bytes = result.assertBytes("cancelCollabMember")
        val seen = conn.thread.lastCancelCollabMemberRequest!!
        assertEquals("s5", seen.sessionId)
        assertEquals("th-1", seen.threadId)
        assertEquals("th-1", pocketagent.v1.CollabMemberSnapshot.parseFrom(bytes).threadId)
    }

    @Test
    fun `retryCollabMember 回新成员卡字节`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.retryCollabMember = collabMemberSnapshot("th-2")
        val (methods, result, _) = harness(conn)
        methods.handle(
            BridgeMethodCall(
                "retryCollabMember",
                RetryCollabMemberRequest.newBuilder().setSessionId("s5").setThreadId("th-2").build().toByteArray(),
            ),
            result,
        )
        val bytes = result.assertBytes("retryCollabMember")
        assertEquals("th-2", conn.thread.lastRetryCollabMemberRequest!!.threadId)
        assertEquals("th-2", pocketagent.v1.CollabMemberSnapshot.parseFrom(bytes).threadId)
    }

    // ── §3.20 getCollabPolicy（RuntimeService）─────────────────────────────

    @Test
    fun `getCollabPolicy 走 RuntimeService 而非 ThreadService`() = runBlocking {
        val conn = FakeConnection()
        conn.runtime.collabPolicy = collabPolicyResponse(3)
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("getCollabPolicy", ByteArray(0)), result)
        val bytes = result.assertBytes("getCollabPolicy")
        assertArrayEquals(conn.runtime.collabPolicy!!.toByteArray(), bytes)
        assertEquals(3L, pocketagent.v1.CollabPolicyResponse.parseFrom(bytes).revision)
    }

    // ── §3.21~§3.24 终端 ──────────────────────────────────────────────────

    @Test
    fun `listTerminals 响应字节级一致`() = runBlocking {
        val conn = FakeConnection()
        conn.terminal.listTerminals = listTerminalsResponse("term-1", "term-2")
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("listTerminals", ByteArray(0)), result)
        val bytes = result.assertBytes("listTerminals")
        assertArrayEquals(conn.terminal.listTerminals!!.toByteArray(), bytes)
        assertEquals(listOf("term-1", "term-2"), pocketagent.v1.ListTerminalsResponse.parseFrom(bytes).sessionsList.map { it.sessionId })
    }

    @Test
    fun `terminalClose Map 入参拆成 CloseTerminalRequest_回 null`() = runBlocking {
        val conn = FakeConnection()
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("terminalClose", mapOf("terminalId" to "term-1")), result)
        result.awaitSettled()
        assertTrue(result.successCalled)
        assertNull(result.success)
        assertEquals("term-1", conn.terminal.lastCloseTerminalRequest!!.sessionId)
    }

    // ── §3.28 turnResync（server stream 收齐一次性返回）────────────────────

    @Test
    fun `turnResync 收齐所有帧为字节数组_空流回空列表`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.eventsSince = listOf(turnServerFrame(11), turnServerFrame(12), turnServerFrame(13))
        val (methods, result, _) = harness(conn)
        methods.handle(
            BridgeMethodCall("turnResync", mapOf("threadId" to "t1", "afterSequence" to 10)),
            result,
        )
        result.awaitSettled()

        val frames = result.success as? List<*>
        assertNotNull("turnResync 须回 List<Uint8List>", frames)
        assertEquals(3, frames!!.size)
        // 逐帧反序列化：sequence 与水位语义正确。
        val seqs = frames.map { pocketagent.v1.TurnServerMessage.parseFrom(it as ByteArray).sequence }
        assertEquals(listOf(11L, 12L, 13L), seqs)
        assertEquals("t1", conn.thread.lastGetThreadEventsSinceRequest!!.threadId)
        assertEquals(10L, conn.thread.lastGetThreadEventsSinceRequest!!.afterSequence)
    }

    @Test
    fun `turnResync 无新帧时回空列表_不是错误`() = runBlocking {
        // 契约：空 = 无新帧（合法返回，不是错误）。
        val conn = FakeConnection()
        conn.thread.eventsSince = emptyList()
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("turnResync", mapOf("threadId" to "t1", "afterSequence" to 99)), result)
        result.awaitSettled()
        assertTrue(result.successCalled)
        assertEquals(emptyList<ByteArray>(), result.success)
        assertEquals(99L, conn.thread.lastGetThreadEventsSinceRequest!!.afterSequence)
    }

    @Test
    fun `turnResync afterSequence 不是数字时回 DECODE_FAILED`() = runBlocking {
        val (methods, result, _) = harness()
        methods.handle(BridgeMethodCall("turnResync", mapOf("threadId" to "t1", "afterSequence" to "10")), result)
        result.assertError(BridgeErrorCodes.DECODE_FAILED)
        assertTrue(result.errorMessage!!.contains("afterSequence"))
    }

    // ── §3.29~§3.30 历史 / 斜杠命令 ───────────────────────────────────────

    @Test
    fun `getThreadHistory threadId加limit加rounds 原样过桥`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.getThreadHistory = getThreadHistoryResponse("user", "assistant")
        val (methods, result, _) = harness(conn)
        val req = GetThreadHistoryRequest.newBuilder().setThreadId("t1").setLimit(50).setRounds(20).build()
        methods.handle(BridgeMethodCall("getThreadHistory", req.toByteArray()), result)
        val bytes = result.assertBytes("getThreadHistory")

        val seen = conn.thread.lastGetThreadHistoryRequest!!
        assertEquals("t1", seen.threadId)
        assertEquals(50, seen.limit)
        assertEquals(20, seen.rounds)
        val parsed = pocketagent.v1.GetThreadHistoryResponse.parseFrom(bytes)
        assertEquals(listOf("user", "assistant"), parsed.messagesList.map(ThreadMessage::getRole))
    }

    @Test
    fun `listSlashCommands refresh 标记原样过桥`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.listSlashCommands = listSlashCommandsResponse("goal", "compact", "review")
        val (methods, result, _) = harness(conn)
        val req = ListSlashCommandsRequest.newBuilder().setThreadId("t1").setRefresh(true).build()
        methods.handle(BridgeMethodCall("listSlashCommands", req.toByteArray()), result)
        val bytes = result.assertBytes("listSlashCommands")

        assertTrue(conn.thread.lastListSlashCommandsRequest!!.refresh)
        assertEquals(listOf("goal", "compact", "review"), pocketagent.v1.ListSlashCommandsResponse.parseFrom(bytes).commandsList.map { it.name })
    }

    // ── 双向流六方法：turn 三条 + terminal 三条（§3.22~§3.27）────────────

    /**
     * 双向流装配：真 B4d machinery + 可编程 fake async stub + 可写 sink feed。
     *
     * 【为什么不 stub 自己写的假实现】`BridgeEventChannels` 是真类（B4d 成品），
     * 经其公开构造器装配；`BridgeThreadStreamService` / `BridgeTerminalStreamService`
     * 是 B4d 定义的窄接口（protoc stub 是 final class，只能假实现——与 B4b/B4d
     * 测试同纪律）。桥的 dispatch → 入参解析 → machinery → observer 全链被真实
     * 执行，唯一假点是「daemon 回什么」。
     *
     * 【为什么每个用例一份】`BridgeEventChannels` 自带 IO scope 与收集 job，
     * 用后必须 shutdown（防协程泄漏）；共享一份会让用例间通过 turnSession 串味。
     */
    private inner class StreamFixture {
        val feed = MutableStreamSinkFeed()
        val innerMessenger = RecordingEventMessenger()
        /**
         * 经 [SinkObservingMessenger] 注册：Dart 订阅 turn/terminal 事件面时那个
         * 真 sink 被记进 [feed]——与生产同一条路径（B4d 的 handler 私有，sink
         * 只能在这一层观察到，见 [SinkObservingMessenger] KDoc）。
         */
        val eventMessenger: BridgeEventMessenger = SinkObservingMessenger(innerMessenger, feed)
        val sources = FakeEventSourcesForStreams()
        val channels = BridgeEventChannels(eventMessenger, sources, FakeLogForStreams())
        val turnStub = TurnStreamFakeStub()
        val terminalStub = TerminalStreamFakeStub()
        val wiring = BridgeEventChannelsWiring(
            channels = channels,
            threadStub = turnStub,
            terminalStub = terminalStub,
            sinkFeeder = feed,
        )
        val conn = FakeConnection()

        val methods: BridgeMethods = BridgeMethods(RecordingMessenger(), conn, wiring)

        /**
         * 每个方法调用一份**新的**结果接收者。
         *
         * 【为什么每次新建】`RecordingResult` 是终态记录器（success/error 只记一次），
         * 复用会让第二次调用的断言读到第一次的终态（"应走 error，实际 success=null"）。
         * 与既有用例 `harness()` 每次返回新 RecordingResult 同纪律。
         */
        fun invoke(method: String, arguments: Any?): RecordingResult {
            val r = RecordingResult()
            methods.handle(BridgeMethodCall(method, arguments), r)
            return r
        }

        /** 模拟 Dart 订阅 turn_events（chat_session.dart 的 _subscribe）。 */
        fun subscribeTurn(sink: BridgeEventSink = FakeStreamSink()) {
            innerMessenger.handlers[BRIDGE_TURN_EVENT_CHANNEL_NAME]!!.onListen(null, sink)
        }

        /** 模拟 Dart 订阅 terminal_events（terminal_data.dart 的 open 前置订阅）。 */
        fun subscribeTerminal(sink: BridgeEventSink = FakeStreamSink()) {
            innerMessenger.handlers[BRIDGE_TERMINAL_EVENT_CHANNEL_NAME]!!.onListen(null, sink)
        }

        fun shutdown() = channels.shutdown()
    }

    /**
     * 等 [condition] 为真或超时（协程在 IO 上跑，固定睡眠要么不够要么浪费）。
     *
     * B4d 的 `TurnStreamSession` 建流在它自己的 IO scope 的协程里：`turnStreamStart`
     * 方法调用返回时，那个协程可能还没走到 `stub.startTurn`——故双向流用例用本函数
     * 等 machinery 落地，而不是假设同步。
     *
     * 【为什么睡而不是 yield】runBlocking 的调用线程是 main；`Thread.yield()` 在
     * main 上不保证让 IO 线程的协程前进（JVM 调度不承诺跨线程让渡）。
     */
    private fun awaitUntil(what: String, timeoutMs: Long = 5_000, condition: () -> Boolean) {
        val deadline = System.currentTimeMillis() + timeoutMs
        while (!condition()) {
            if (System.currentTimeMillis() > deadline) {
                throw AssertionError("等待 $what ${timeoutMs}ms 未满足（协程卡死或推流路径没走通）")
            }
            Thread.sleep(5)
        }
    }

    @Test
    fun `turnStreamStart 建流并受理_首帧另由 turnStreamSend 发`() = runBlocking {
        val f = StreamFixture()
        f.subscribeTurn()

        val result = f.invoke("turnStreamStart", mapOf("threadId" to "t-1"))
        awaitUntil("startTurn 应被调") { f.turnStub.startCalls == 1 }
        result.awaitSettled()

        // §3.25：返回 null = 建流受理（不是"轮次已开始"——首帧还没发）。
        assertTrue("turnStreamStart 应走 success", result.successCalled)
        assertNull(result.success)
        // B4d 的 TurnStreamSession 协程里调 stub.startTurn：真 machinery 跑通
        // （threadId 由 B4d 绑进会话，startCalls 即"流真建了"的判决点）。
        assertEquals(1, f.turnStub.startCalls)
        f.shutdown()
    }

    @Test
    fun `turnStreamSend 字节经 observer 进流_input 与 supplement 两形态`() = runBlocking {
        val f = StreamFixture()
        f.subscribeTurn()
        val start = f.invoke("turnStreamStart", mapOf("threadId" to "t-1"))
        awaitUntil("startTurn 应被调") { f.turnStub.startCalls == 1 }
        start.awaitSettled()

        // Dart 按 proto 封帧：input 首帧 / supplement 追问都走这里
        // （chat_session.dart 的 startTurn 与 steer 两处）。
        val inputFrame = TurnClientMessage.newBuilder()
            .setInput(
                TurnInput.newBuilder()
                    .setThreadId("t-1")
                    .addBlocks(ContentBlock.newBuilder().setText(TextBlock.newBuilder().setText("你好"))),
            )
            .build()
        val r1 = f.invoke("turnStreamSend", inputFrame.toByteArray())
        r1.awaitSettled()
        assertTrue("turnStreamSend 应走 success", r1.successCalled)
        assertNull(r1.success)

        // 帧原样进流：observer 收到的就是桥 parseFrom 出来的那条（字段级对账）。
        awaitUntil("observer 应收到帧") { f.turnStub.requestObserver.seen.isNotEmpty() }
        val seen = f.turnStub.requestObserver.seen.single()
        assertEquals("t-1", seen.input.threadId)
        assertEquals("你好", seen.input.getBlocks(0).text.text)

        // supplement 追问帧：同一条 observer，不另建流。
        val supplementFrame = TurnClientMessage.newBuilder()
            .setSupplement(
                TurnSupplement.newBuilder()
                    .addBlocks(ContentBlock.newBuilder().setText(TextBlock.newBuilder().setText("补充"))),
            )
            .build()
        val r2 = f.invoke("turnStreamSend", supplementFrame.toByteArray())
        awaitUntil("observer 应收到第二帧") { f.turnStub.requestObserver.seen.size == 2 }
        assertEquals("补充", f.turnStub.requestObserver.seen[1].supplement.getBlocks(0).text.text)
        r2.awaitSettled()
        assertEquals("追问不得另建流", 1, f.turnStub.startCalls)
        f.shutdown()
    }

    @Test
    fun `turnStreamSend 坏字节回 DECODE_FAILED_不补默认值也不进流`() = runBlocking {
        val f = StreamFixture()
        f.subscribeTurn()
        val start = f.invoke("turnStreamStart", mapOf("threadId" to "t-1"))
        awaitUntil("startTurn 应被调") { f.turnStub.startCalls == 1 }
        start.awaitSettled()

        // 合法 varint 后接非法 wire type 的残字节，parseFrom 必炸（同 getHostInfo 用例）。
        val result = f.invoke(
            "turnStreamSend",
            byteArrayOf(0x0A, 0xFF.toByte(), 0xFF.toByte(), 0xFF.toByte()),
        )
        result.assertError(BridgeErrorCodes.DECODE_FAILED)
        assertTrue(result.errorMessage!!.contains("turnStreamSend"))
        // 坏帧不毒化流：宿主侧先拦，一帧坏不影响后续帧（契约 §4 同纪律）。
        assertTrue("坏字节不得进流", f.turnStub.requestObserver.seen.isEmpty())
        f.shutdown()
    }

    @Test
    fun `turnStreamSend 入参不是字节时回 DECODE_FAILED_不做双形态二分`() = runBlocking {
        val f = StreamFixture()
        f.subscribeTurn()
        val start = f.invoke("turnStreamStart", mapOf("threadId" to "t-1"))
        awaitUntil("startTurn 应被调") { f.turnStub.startCalls == 1 }
        start.awaitSettled()

        // Dart 侧若误发 Map 形态（契约漂移）：显式失败，不补默认值。
        val result = f.invoke("turnStreamSend", mapOf("input" to "x"))
        result.assertError(BridgeErrorCodes.DECODE_FAILED)
        assertTrue("坏字节不得进流", f.turnStub.requestObserver.seen.isEmpty())
        f.shutdown()
    }

    @Test
    fun `turnStreamSend 未 start 就发回结构化错误_不许 panic`() = runBlocking {
        val f = StreamFixture()
        // 不 turnStreamStart 直接 send：宿主侧没有流，如实报"无处可发"。
        val result = f.invoke("turnStreamSend", TurnClientMessage.getDefaultInstance().toByteArray())
        result.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
        assertTrue(result.errorMessage!!.contains("无处可发"))
        // 关键：走的是结构化错误分支，不是 handle 的 INTERNAL 兜底（panic 会被收敛）。
        assertEquals("时序问题不是宿主内部错误", null, result.errorDetails)
        f.shutdown()
    }

    @Test
    fun `turnStreamCancel 摘掉宿主侧流_之后 send 无处可发`() = runBlocking {
        val f = StreamFixture()
        f.subscribeTurn()
        val start = f.invoke("turnStreamStart", mapOf("threadId" to "t-1"))
        awaitUntil("startTurn 应被调") { f.turnStub.startCalls == 1 }
        start.awaitSettled()

        // §3.27：arguments = null → null。只摘宿主侧资源，不发 cancel 帧
        // （Dart 侧显式取消走 turnStreamSend(cancel 帧)，见 chat_session.cancel()）。
        val cancel = f.invoke("turnStreamCancel", null)
        cancel.awaitSettled()
        assertTrue("turnStreamCancel 应走 success", cancel.successCalled)
        assertNull(cancel.success)
        assertTrue("gRPC 流应被 cancel", f.turnStub.requestObserver.cancelled)

        // 摘了再 send = 无处可发（不是打到已断的流上）。
        val after = f.invoke("turnStreamSend", TurnClientMessage.getDefaultInstance().toByteArray())
        after.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
        assertTrue(after.errorMessage!!.contains("无处可发"))
        f.shutdown()
    }

    @Test
    fun `turnStreamCancel 收到非 null 入参时回 DECODE_FAILED`() = runBlocking {
        val f = StreamFixture()
        f.subscribeTurn()
        val result = f.invoke("turnStreamCancel", mapOf("x" to 1))
        result.assertError(BridgeErrorCodes.DECODE_FAILED)
        f.shutdown()
    }

    @Test
    fun `turnStreamStart 缺 threadId 键时回 DECODE_FAILED_不补空串也不建流`() = runBlocking {
        val f = StreamFixture()
        f.subscribeTurn()
        val result = f.invoke("turnStreamStart", mapOf("threadID" to "t-1"))
        result.assertError(BridgeErrorCodes.DECODE_FAILED)
        assertTrue(result.errorMessage!!.contains("threadId"))
        // 键名错 = 契约漂移，不得建流。
        assertEquals("缺键不得建流", 0, f.turnStub.startCalls)
        f.shutdown()
    }

    @Test
    fun `turnStreamStart 在 Dart 未订阅事件面时回 HOST_UNAVAILABLE_不建假流`() = runBlocking {
        val f = StreamFixture()
        // 不 subscribeTurn：sink 不在。此刻建流 = 建一条永远收不到过程帧的流
        // （比"通道未就绪"更糟的假象），故如实回错且不调 startTurn。
        val result = f.invoke("turnStreamStart", mapOf("threadId" to "t-1"))
        result.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
        assertTrue(result.errorMessage!!.contains("sink"))
        assertEquals("无 sink 不得建流", 0, f.turnStub.startCalls)
        f.shutdown()
    }

    @Test
    fun `terminalAttach 建流并受理_open 帧另由 terminalWrite 发`() = runBlocking {
        val f = StreamFixture()
        f.subscribeTerminal()

        val result = f.invoke("terminalAttach", mapOf("terminalId" to "sess-1"))
        awaitUntil("attach 应被调") { f.terminalStub.attachCalls == 1 }
        result.awaitSettled()

        assertTrue("terminalAttach 应走 success", result.successCalled)
        assertNull(result.success)
        assertEquals(1, f.terminalStub.attachCalls)
        f.shutdown()
    }

    @Test
    fun `terminalWrite 字节经 observer 进流_open 与 input 与 resize 三形态`() = runBlocking {
        val f = StreamFixture()
        f.subscribeTerminal()
        val attach = f.invoke("terminalAttach", mapOf("terminalId" to "sess-1"))
        awaitUntil("attach 应被调") { f.terminalStub.attachCalls == 1 }
        attach.awaitSettled()

        // terminal_data.dart 的 open()：attach → write(open 帧)。
        val openFrame = TerminalClientMessage.newBuilder()
            .setOpen(
                OpenTerminal.newBuilder()
                    .setSessionId("sess-1")
                    .setCwd("/ws")
                    .setCols(80)
                    .setRows(24),
            )
            .build()
        val r1 = f.invoke("terminalWrite", openFrame.toByteArray())
        r1.awaitSettled()
        assertTrue("terminalWrite 应走 success", r1.successCalled)
        assertNull(r1.success)
        awaitUntil("observer 应收到 open 帧") { f.terminalStub.requestObserver.seen.isNotEmpty() }
        val seenOpen = f.terminalStub.requestObserver.seen.single()
        assertEquals("sess-1", seenOpen.open.sessionId)
        assertEquals("/ws", seenOpen.open.cwd)
        assertEquals(80, seenOpen.open.cols)
        assertEquals(24, seenOpen.open.rows)

        // input 帧（terminal_data.dart 的 sendInput）与 resize 帧：同一条 observer。
        val inputFrame = TerminalClientMessage.newBuilder()
            .setInput(TerminalInput.newBuilder().setData(com.google.protobuf.ByteString.copyFromUtf8("ls\r")))
            .build()
        val r2 = f.invoke("terminalWrite", inputFrame.toByteArray())
        awaitUntil("observer 应收到 input 帧") { f.terminalStub.requestObserver.seen.size == 2 }
        assertEquals("ls\r", f.terminalStub.requestObserver.seen[1].input.data.toStringUtf8())
        r2.awaitSettled()

        val resizeFrame = TerminalClientMessage.newBuilder()
            .setResize(TerminalResize.newBuilder().setCols(120).setRows(40))
            .build()
        val r3 = f.invoke("terminalWrite", resizeFrame.toByteArray())
        awaitUntil("observer 应收到 resize 帧") { f.terminalStub.requestObserver.seen.size == 3 }
        assertEquals(120, f.terminalStub.requestObserver.seen[2].resize.cols)
        assertEquals(40, f.terminalStub.requestObserver.seen[2].resize.rows)
        r3.awaitSettled()
        assertEquals("input/resize 不得另建流", 1, f.terminalStub.attachCalls)
        f.shutdown()
    }

    @Test
    fun `terminalWrite 坏字节回 DECODE_FAILED_且不进流`() = runBlocking {
        val f = StreamFixture()
        f.subscribeTerminal()
        val attach = f.invoke("terminalAttach", mapOf("terminalId" to "sess-1"))
        awaitUntil("attach 应被调") { f.terminalStub.attachCalls == 1 }
        attach.awaitSettled()

        val result = f.invoke(
            "terminalWrite",
            byteArrayOf(0x0A, 0xFF.toByte(), 0xFF.toByte(), 0xFF.toByte()),
        )
        result.assertError(BridgeErrorCodes.DECODE_FAILED)
        assertTrue("坏字节不得进流", f.terminalStub.requestObserver.seen.isEmpty())
        f.shutdown()
    }

    @Test
    fun `terminalWrite 未 attach 就写回结构化错误_不许 panic`() = runBlocking {
        val f = StreamFixture()
        val result = f.invoke("terminalWrite", TerminalClientMessage.getDefaultInstance().toByteArray())
        result.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
        assertTrue(result.errorMessage!!.contains("无处可发"))
        assertEquals("时序问题不是宿主内部错误", null, result.errorDetails)
        f.shutdown()
    }

    @Test
    fun `terminalAttach 在 Dart 未订阅事件面时回 HOST_UNAVAILABLE_不建假流`() = runBlocking {
        val f = StreamFixture()
        val result = f.invoke("terminalAttach", mapOf("terminalId" to "sess-1"))
        result.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
        assertTrue(result.errorMessage!!.contains("sink"))
        assertEquals("无 sink 不得建流", 0, f.terminalStub.attachCalls)
        f.shutdown()
    }

    @Test
    fun `terminalClose 关 daemon 会话并摘掉宿主侧 attach 流`() = runBlocking {
        val f = StreamFixture()
        f.subscribeTerminal()
        val attach = f.invoke("terminalAttach", mapOf("terminalId" to "sess-1"))
        awaitUntil("attach 应被调") { f.terminalStub.attachCalls == 1 }
        attach.awaitSettled()

        // §3.24：{'terminalId': String} → null。unary CloseTerminal + 摘本地流
        // （unary 只让 daemon 关 PTY，本地 bidi 流不摘会继续推幽灵帧）。
        val close = f.invoke("terminalClose", mapOf("terminalId" to "sess-1"))
        close.awaitSettled()
        assertTrue("terminalClose 应走 success", close.successCalled)
        assertNull(close.success)
        assertEquals("sess-1", f.conn.terminal.lastCloseTerminalRequest!!.sessionId)
        assertTrue("宿主侧 attach 流应被摘", f.terminalStub.requestObserver.cancelled)

        // 摘了再写 = 无处可发。
        val after = f.invoke("terminalWrite", TerminalClientMessage.getDefaultInstance().toByteArray())
        after.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
        f.shutdown()
    }

    @Test
    fun `terminalClose 的 unary RPC 失败照旧映射契约码_本地摘流不受影响`() = runBlocking {
        val f = StreamFixture()
        f.conn.terminal.failure = grpcError(io.grpc.Status.Code.UNAVAILABLE, "通道半死")
        f.subscribeTerminal()
        val attach = f.invoke("terminalAttach", mapOf("terminalId" to "sess-1"))
        awaitUntil("attach 应被调") { f.terminalStub.attachCalls == 1 }
        attach.awaitSettled()

        val close = f.invoke("terminalClose", mapOf("terminalId" to "sess-1"))
        // daemon 关了但本地还挂着流 = 幽灵帧源；unary 失败也要摘（防泄漏优先于报错）。
        close.assertError(BridgeErrorCodes.NOT_CONNECTED)
        assertTrue("unary 失败也要摘本地流", f.terminalStub.requestObserver.cancelled)
        f.shutdown()
    }

    @Test
    fun `未注入装配 seam 时三条建流方法如实回 HOST_UNAVAILABLE_点名缺什么`() = runBlocking {
        // 生产默认（streams = null）：不建假流，消息点名缺的是装配 seam。
        val cases = listOf(
            "turnStreamStart" to mapOf("threadId" to "t1"),
            "terminalAttach" to mapOf("terminalId" to "term-1"),
        )
        for ((m, args) in cases) {
            val (methods, r, _) = harness()
            methods.handle(BridgeMethodCall(m, args), r)
            r.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
            assertTrue("$m 的消息应点名未接线", r.errorMessage!!.contains("未接线"))
        }
        // terminalClose 的 unary 面本来就已实现（缺 seam 只影响摘流，不影响受理）。
        val (m2, r2, conn2) = harness()
        m2.handle(BridgeMethodCall("terminalClose", mapOf("terminalId" to "term-1")), r2)
        r2.awaitSettled()
        assertTrue("terminalClose 的 unary 面不依赖 seam", r2.successCalled)
        assertNull(r2.success)
        assertEquals("term-1", conn2.terminal.lastCloseTerminalRequest!!.sessionId)
    }

    @Test
    fun `send 与 write 在未注入 seam 时回 HOST_UNAVAILABLE_不 panic`() = runBlocking {
        val cases = listOf(
            "turnStreamSend" to TurnClientMessage.getDefaultInstance().toByteArray(),
            "terminalWrite" to TerminalClientMessage.getDefaultInstance().toByteArray(),
        )
        for ((m, args) in cases) {
            val (methods, r, _) = harness()
            methods.handle(BridgeMethodCall(m, args), r)
            r.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
            assertTrue("$m 的消息应点名无处可发", r.errorMessage!!.contains("无处可发"))
        }
        // turnStreamCancel 只摘资源：无 seam 时是干净的 no-op（回 null，不报错）。
        val (m3, r3, _) = harness()
        m3.handle(BridgeMethodCall("turnStreamCancel", null), r3)
        r3.awaitSettled()
        assertTrue("无 seam 时 cancel 是 no-op", r3.successCalled)
        assertNull(r3.success)
    }

    @Test
    fun `observer onNext 抛 gRPC 失败照旧映射契约码_不裸抛到Flutter侧`() = runBlocking {
        // 线程纪律（BridgeMethods KDoc）：dispatch 整体在 withContext(IO) 里，
        // observer.onNext 经 rpc{} 收口——gRPC 失败映射成契约码而不是裸异常。
        val f = StreamFixture()
        f.subscribeTurn()
        val start = f.invoke("turnStreamStart", mapOf("threadId" to "t-1"))
        awaitUntil("startTurn 应被调") { f.turnStub.startCalls == 1 }
        start.awaitSettled()

        f.turnStub.requestObserver.failOnNext =
            grpcError(io.grpc.Status.Code.UNAVAILABLE, "transport 半死")
        val result = f.invoke("turnStreamSend", TurnClientMessage.getDefaultInstance().toByteArray())
        result.assertError(BridgeErrorCodes.NOT_CONNECTED)
        assertEquals("transport 半死", result.errorMessage)
        // details 带 gRPC 状态码名（UI 上层可展开诊断，与 24 个已实现方法同口径）。
        assertEquals("UNAVAILABLE", result.errorDetails)
        f.shutdown()
    }

    @Test
    fun `过程帧经 Dart 订阅的 sink 推回来_B4d machinery 真跑`() = runBlocking {
        // 联动判决点：turnStreamStart 之后，B4d 的 TurnStreamSession 把 daemon
        // 推来的 TurnServerMessage 逐帧转成 wire 字节，经**同一个 sink** 出去。
        val f = StreamFixture()
        val sink = FakeStreamSink()
        f.subscribeTurn(sink)
        val start = f.invoke("turnStreamStart", mapOf("threadId" to "t-1"))
        awaitUntil("startTurn 应被调") { f.turnStub.startCalls == 1 }
        start.awaitSettled()

        // daemon 侧回一帧（responseObserver.onNext 由 B4d 转给 sink）。
        f.turnStub.responseObserver.onNext(turnServerFrame(7))
        awaitUntil("sink 应收到一帧") { sink.frames.isNotEmpty() }
        val bytes = sink.frames.single() as ByteArray
        assertEquals(7L, pocketagent.v1.TurnServerMessage.parseFrom(bytes).sequence)
        f.shutdown()
    }

    @Test
    fun `daemon 侧流错误经 sink 报出来_方法面不吞`() = runBlocking {
        val f = StreamFixture()
        val sink = FakeStreamSink()
        f.subscribeTurn(sink)
        val start = f.invoke("turnStreamStart", mapOf("threadId" to "t-1"))
        awaitUntil("startTurn 应被调") { f.turnStub.startCalls == 1 }
        start.awaitSettled()

        // 流错误走 sink.error（契约 §4「宿主 EventSink.error → 流错误，code 原样」），
        // 不经方法面 success/error——方法调用早已回 null（受理语义）。
        f.turnStub.responseObserver.onError(
            grpcError(io.grpc.Status.Code.INTERNAL, "daemon 侧炸了"),
        )
        awaitUntil("sink 应报流错误") { sink.errors.isNotEmpty() }
        val (code, message, _) = sink.errors.single()
        assertEquals(BridgeErrorCodes.INTERNAL, code)
        assertTrue(message!!.contains("daemon 侧炸了"))
        f.shutdown()
    }

    @Test
    fun `建流时 daemon 迟迟不交 observer_有界等待后回结构化错误_不无限挂住`() = runBlocking {
        // B4d 的会话协程跑在它自己的 IO scope 上：正常一次线程跳转就把 observer
        // 交出来，但 daemon 半死态下可能迟迟不交。方法面有界等待（5s）后如实回错，
        // 而不是把调用无限期挂在 IO 上（红线：不留假受理）。
        val f = StreamFixture()
        f.subscribeTurn()
        // 让 stub 的 startTurn 挂住 12s（> 5s 上界）：模拟"daemon 不应答"。
        // 【为什么是 12s】方法面的有界等待上界是 5s；这里必须显著大于它，否则
        // observer 在等待窗口内就交了、测不到超时路径。
        f.turnStub.startTurnDelayMs = 12_000

        val result = f.invoke("turnStreamStart", mapOf("threadId" to "t-1"))
        // 本用例的等待上界要宽于方法面的 5s（否则测试自己的轮询先超时，
        // 把"方法面如实回错"误判成"协程卡死"）。
        awaitUntil("方法面应回超时错误", 15_000) { result.errorCalled || result.successCalled }
        result.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
        assertTrue(result.errorMessage!!.contains("未就绪"))
        // 关键：是结构化错误而不是 handle 的 INTERNAL 兜底（panic 收敛）。
        assertEquals("有界等待超时不是宿主内部错误", null, result.errorDetails)
        f.shutdown()
    }

    @Test
    fun `建流成功后紧随的首帧有落点_不因协程竞态打空 observer`() = runBlocking {
        // 竞态判决点：turnStreamStart 返回时 observer 必须已就绪——否则 Dart 的
        // `await turnStreamStart(...)` 紧接 `await turnStreamSend(首帧)` 会打到
        // 空 observer 上，报一个"无处可发"的假错误（chat_session.dart:152-153）。
        // 让 stub 的 startTurn 故意慢 80ms（拉宽竞态窗口），方法面仍须等它就绪。
        val f = StreamFixture()
        f.subscribeTurn()
        f.turnStub.startTurnDelayMs = 80

        val start = f.invoke("turnStreamStart", mapOf("threadId" to "t-1"))
        start.awaitSettled()
        assertTrue("建流应受理", start.successCalled)

        // 紧随的首帧（不再 awaitUntil——竞态窗口已被方法面的等待收窄）。
        val send = f.invoke("turnStreamSend", TurnClientMessage.getDefaultInstance().toByteArray())
        send.awaitSettled()
        assertTrue("首帧应成功进流", send.successCalled)
        awaitUntil("observer 应收到帧") { f.turnStub.requestObserver.seen.isNotEmpty() }
        f.shutdown()
    }

    // ── 未知方法：契约漂移，同样 HOST_UNAVAILABLE ──────────────────────────

    @Test
    fun `未知方法回 HOST_UNAVAILABLE 且消息点名契约漂移`() = runBlocking {
        val (methods, result, _) = harness()
        methods.handle(BridgeMethodCall("someFutureMethod", null), result)
        result.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
        assertTrue(result.errorMessage!!.contains("未知方法"))
        assertTrue(result.errorMessage!!.contains("someFutureMethod"))
    }

    @Test
    fun `大小写不同的方法名也是未知方法_方法名逐字匹配`() = runBlocking {
        // 契约字符串大小写敏感：Dart 侧写 'listThreads'，'listthreads' 不存在。
        val (methods, result, _) = harness()
        methods.handle(BridgeMethodCall("listthreads", ByteArray(0)), result)
        result.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
        assertTrue(result.errorMessage!!.contains("未知方法"))
    }

    // ── 错误码映射在真实方法面上的落地（不只纯函数层）─────────────────────

    @Test
    fun `UNAUTHENTICATED 在 listThreads 上落 AUTH_REQUIRED`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.failure = grpcError(io.grpc.Status.Code.UNAUTHENTICATED, "token 过期")
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("listThreads", ByteArray(0)), result)
        result.assertError(BridgeErrorCodes.AUTH_REQUIRED)
        assertEquals("token 过期", result.errorMessage)
    }

    @Test
    fun `NOT_FOUND 在 getCollabSession 上落 INTERNAL 且人话透传`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.failure = grpcError(io.grpc.Status.Code.NOT_FOUND, "协同会话不存在: s1")
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("getCollabSession", ByteArray(0)), result)
        result.assertError(BridgeErrorCodes.INTERNAL)
        assertEquals("协同会话不存在: s1", result.errorMessage)
        // details 带 gRPC 状态码名，UI 上层可展开诊断。
        assertEquals("NOT_FOUND", result.errorDetails)
    }

    @Test
    fun `INVALID_ARGUMENT 在 deleteCollabRole 上落 INTERNAL_daemon原文透传`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.failure = grpcError(io.grpc.Status.Code.INVALID_ARGUMENT, "角色 id 不能为空")
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("deleteCollabRole", ByteArray(0)), result)
        result.assertError(BridgeErrorCodes.INTERNAL)
        assertEquals("角色 id 不能为空", result.errorMessage)
    }

    @Test
    fun `非 gRPC 异常也收敛成 INTERNAL_绝不裸抛到Flutter侧`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.failure = IllegalStateException("本地编码炸了")
        val (methods, result, _) = harness(conn)
        methods.handle(BridgeMethodCall("listThreads", ByteArray(0)), result)
        result.assertError(BridgeErrorCodes.INTERNAL)
        assertTrue(result.errorMessage!!.contains("IllegalStateException"))
    }
}

// ── 双向流六方法的假实现（B4d 窄接口 + seam 级 machinery）─────────────────
//
// 命名带 StreamFake 前缀：B4d 的 BridgeEventChannelsTest 在**同一包**里已有
// 同名 top-level fake（FakeThreadStreamStub），Kotlin 不允许同包同名顶层声明
// （Redeclaration）——前缀区分是唯一解（不共享 fixture：两边断言的面不同）。

/** 记录型 [BridgeEventMessenger]：capture 三个 handler 供测试直接驱动 onListen/onCancel。 */
internal class RecordingEventMessenger : BridgeEventMessenger {
    val handlers = HashMap<String, BridgeEventStreamHandler>()
    override fun setStreamHandler(channelName: String, handler: BridgeEventStreamHandler) {
        handlers[channelName] = handler
    }
}

/**
 * 双向流用例的 [BridgeEventSources]：五个 StateFlow + 两条 SharedFlow 测试可推。
 *
 * 与 B4d 测试的 FakeSources 同结构（[BridgeConnection] 的 20+ 快照成员委托
 * [FakeConnection]）——双向流用例不读快照，委托即零成本。
 */
internal class FakeEventSourcesForStreams(
    private val delegate: BridgeConnection = FakeConnection(),
) : BridgeEventSources, BridgeConnection by delegate {
    override val connStateFlow = kotlinx.coroutines.flow.MutableStateFlow("checking")
    override val connTransportFlow = kotlinx.coroutines.flow.MutableStateFlow("")
    override val connAddressFlow = kotlinx.coroutines.flow.MutableStateFlow("")
    override val connectionEpochFlow = kotlinx.coroutines.flow.MutableStateFlow(0)
    override val hostCapabilitiesFlow = kotlinx.coroutines.flow.MutableStateFlow<List<String>>(emptyList())
    override val hostEventsFlow =
        kotlinx.coroutines.flow.MutableSharedFlow<pocketagent.v1.HostEvent>(extraBufferCapacity = 8)
    override val hostInfoFlow =
        kotlinx.coroutines.flow.MutableSharedFlow<pocketagent.v1.HostInfo>(extraBufferCapacity = 8)
}

/** 记录型 [BridgeLog]：不落 Android 框架（JVM 单测零 Robolectric）。 */
internal class FakeLogForStreams : BridgeLog {
    val lines = ArrayList<String>()
    override fun w(tag: String, message: String) { lines.add("$tag: $message") }
}

/** 记录型 [BridgeEventSink]：帧/错误/关流三语义各自记录。 */
internal class FakeStreamSink : BridgeEventSink {
    val frames = ArrayList<Any?>()
    val errors = ArrayList<Triple<String, String?, Any?>>()
    var ended = false
    override fun success(event: Any?) { frames.add(event) }
    override fun error(errorCode: String, errorMessage: String?, errorDetails: Any?) {
        errors.add(Triple(errorCode, errorMessage, errorDetails))
    }
    override fun endOfStream() { ended = true }
}

/**
 * 假 StartTurn stub（B4d 的 [BridgeThreadStreamService]）：
 *  - `startTurn` 返回**可编程的请求 observer**（onNext 留痕 + 可摆失败 + 记 cancel）；
 *  - `responseObserver` 暴露给测试，模拟 daemon 推帧/报错。
 */
internal class TurnStreamFakeStub : BridgeThreadStreamService {
    var startCalls = 0

    /**
     * `startTurn` 的故意延迟（毫秒）：拉宽「B4d 协程还没交出 observer」的竞态窗口。
     *
     * 用来验两件事：① 方法面的有界等待真的在等（竞态窗口内首帧仍有落点）；
     * ② 超过上界时回结构化错误而不是无限挂住。
     */
    var startTurnDelayMs: Long = 0

    lateinit var responseObserver: io.grpc.stub.StreamObserver<pocketagent.v1.TurnServerMessage>
    val requestObserver = FakeRequestObserver<TurnClientMessage>()

    override fun startTurn(
        responseObserver: io.grpc.stub.StreamObserver<pocketagent.v1.TurnServerMessage>,
    ): io.grpc.stub.StreamObserver<TurnClientMessage> {
        startCalls += 1
        if (startTurnDelayMs > 0) Thread.sleep(startTurnDelayMs)
        this.responseObserver = responseObserver
        return requestObserver
    }
}

/** 假 Attach stub（B4d 的 [BridgeTerminalStreamService]）：同 [TurnStreamFakeStub]。 */
internal class TerminalStreamFakeStub : BridgeTerminalStreamService {
    var attachCalls = 0
    lateinit var responseObserver: io.grpc.stub.StreamObserver<pocketagent.v1.TerminalServerMessage>
    val requestObserver = FakeRequestObserver<TerminalClientMessage>()

    override fun attach(
        responseObserver: io.grpc.stub.StreamObserver<pocketagent.v1.TerminalServerMessage>,
    ): io.grpc.stub.StreamObserver<TerminalClientMessage> {
        attachCalls += 1
        this.responseObserver = responseObserver
        return requestObserver
    }
}

/**
 * 可编程的请求 observer：留痕 onNext 收到的帧 + 可摆 onNext 失败 + 记录 cancel。
 *
 * 【为什么实现 ClientCallStreamObserver】B4d 的 `TurnStreamSession.cancel()` 摘流时
 * 调的是 `(requestObserver as? ClientCallStreamObserver<*>)?.cancel(...)`——不实现
 * 这个抽象类，摘流路径（turnStreamCancel / terminalClose）就只是取消协程而没取消
 * gRPC 流（泄漏）。实现后「摘流真的取消了 gRPC 调用」可被断言。
 *
 * 【为什么要可摆失败】`turnStreamSend` 的 `observer.onNext` 真会抛（流已断/
 * cancelled），桥必须经 `rpc{}` 收口成契约码——这条路径只有真抛一次才测得出来。
 *
 * 【流控方法为什么全部空转】本 fake 只被单线程驱动（测试线程 → 桥 → observer），
 * 没有真实传输层的背压/就绪语义；`isReady` 恒 true + 其余 no-op 是最小可信集。
 */
internal class FakeRequestObserver<T> : io.grpc.stub.ClientCallStreamObserver<T>() {
    val seen = ArrayList<T>()
    var failOnNext: Throwable? = null
    var cancelled = false

    override fun onNext(value: T) {
        failOnNext?.let { throw it }
        seen.add(value)
    }

    override fun onError(t: Throwable) = Unit

    override fun onCompleted() = Unit

    override fun cancel(message: String?, cause: Throwable?) {
        cancelled = true
    }

    override fun isReady(): Boolean = true

    override fun setOnReadyHandler(handler: Runnable?): Unit = Unit

    override fun disableAutoInboundFlowControl(): Unit = Unit

    override fun request(count: Int) = Unit

    override fun setMessageCompression(enable: Boolean): Unit = Unit
}
