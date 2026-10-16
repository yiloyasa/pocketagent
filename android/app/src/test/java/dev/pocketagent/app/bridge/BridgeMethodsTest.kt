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
import pocketagent.v1.ReadFileRequest
import pocketagent.v1.RetryCollabMemberRequest
import pocketagent.v1.SetCollabRoleRequest
import pocketagent.v1.ThreadMessage

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

    // ── 契约内、本波不实现的六条：如实 HOST_UNAVAILABLE ────────────────────

    @Test
    fun `startTurn 回 HOST_UNAVAILABLE_不建只有首帧的假流`() = runBlocking {
        // 过程帧消费面（EventChannel）未接；建了流却收不到增量/审批/完成帧
        // 比"通道未就绪"更糟（用户以为在跑，其实瞎了）。
        val (methods, result, _) = harness()
        methods.handle(BridgeMethodCall("startTurn", ByteArray(0)), result)
        result.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
        assertTrue(result.errorMessage!!.contains("startTurn"))
        assertTrue(result.errorMessage!!.contains("turn_events"))
    }

    @Test
    fun `turnStreamStart 与 turnStreamSend 与 turnStreamCancel 都回 HOST_UNAVAILABLE`() = runBlocking {
        for (m in listOf("turnStreamStart", "turnStreamSend", "turnStreamCancel")) {
            val (methods, result, _) = harness()
            val args: Any? = when (m) {
                "turnStreamStart" -> mapOf("threadId" to "t1")
                "turnStreamSend" -> ByteArray(0)
                else -> null
            }
            methods.handle(BridgeMethodCall(m, args), result)
            result.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
        }
    }

    @Test
    fun `terminalAttach 与 terminalWrite 回 HOST_UNAVAILABLE_输出帧归事件面`() = runBlocking {
        for (m in listOf("terminalAttach", "terminalWrite")) {
            val (methods, result, _) = harness()
            val args: Any? = if (m == "terminalAttach") mapOf("terminalId" to "term-1") else ByteArray(0)
            methods.handle(BridgeMethodCall(m, args), result)
            result.assertError(BridgeErrorCodes.HOST_UNAVAILABLE)
            assertTrue(result.errorMessage!!.contains("terminal_events"))
        }
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
