package dev.pocketagent.app.bridge

import io.grpc.Status
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 错误码映射表逐条钉死（契约 §5 + BridgeErrors 的两张表）。
 *
 * 【为什么逐条而不是抽样】映射表是 UI 分支依据（重试/去配对/仅展示），任何一行
 * 悄悄变化都会让某个错误态走错分支；逐条断言后回归立刻可见。
 */
class BridgeErrorsTest {

    // ── 通用表（24 个方法共用）────────────────────────────────────────────

    @Test
    fun `UNAVAILABLE 映射 NOT_CONNECTED_依据是尚未连上daemon`() {
        // 通道未建/半死态/daemon 被杀，都是"这条调用没通到电脑端"。
        assertEquals(BridgeErrorCodes.NOT_CONNECTED, contractCodeOf(Status.Code.UNAVAILABLE))
    }

    @Test
    fun `UNAUTHENTICATED 映射 AUTH_REQUIRED_依据是token缺失过期`() {
        // daemon 鉴权拦截器拒绝；§5 对 AUTH_REQUIRED 的触发条件逐字如此。
        assertEquals(BridgeErrorCodes.AUTH_REQUIRED, contractCodeOf(Status.Code.UNAUTHENTICATED))
    }

    @Test
    fun `INVALID_ARGUMENT 落 INTERNAL 且 message 透传 daemon 原文`() {
        // 契约七码没有"入参被拒"这一档（§6：宿主只抛表内 code），最诚实的落点是
        // 兜底码 + 人话原文；若 §5 将来增码，只改 contractCodeOf 一处。
        val e = grpcFailure("listThreads", grpcError(Status.Code.INVALID_ARGUMENT, "limit 不能为负"))
        assertEquals(BridgeErrorCodes.INTERNAL, e.code)
        assertEquals("limit 不能为负", e.message)
    }

    @Test
    fun `DEADLINE_EXCEEDED 落 INTERNAL_依据是电脑端响应超时`() {
        val e = grpcFailure("readFile", grpcError(Status.Code.DEADLINE_EXCEEDED, "超时"))
        assertEquals(BridgeErrorCodes.INTERNAL, e.code)
        assertEquals("超时", e.message)
    }

    @Test
    fun `PERMISSION_DENIED 在通用表落 INTERNAL_专属语义由 pairCode 表接管`() {
        // 同一状态码在通用表没有专属含义（daemon 拒绝的理由因方法而异）；
        // pairCode 的"配对码仅回环来源"在专属表落 PAIR_DISABLED。
        assertEquals(BridgeErrorCodes.INTERNAL, contractCodeOf(Status.Code.PERMISSION_DENIED))
        assertEquals(
            BridgeErrorCodes.PAIR_DISABLED,
            pairCodeContractCodeOf(Status.Code.PERMISSION_DENIED),
        )
    }

    @Test
    fun `NOT_FOUND 落 INTERNAL 且原文透传_协同会话不存在时人话不丢`() {
        val e = grpcFailure("getCollabSession", grpcError(Status.Code.NOT_FOUND, "协同会话不存在: s1"))
        assertEquals(BridgeErrorCodes.INTERNAL, e.code)
        assertEquals("协同会话不存在: s1", e.message)
    }

    @Test
    fun `UNIMPLEMENTED 落 INTERNAL_旧daemon缺该RPC时如实说明`() {
        val e = grpcFailure("listSlashCommands", grpcError(Status.Code.UNIMPLEMENTED))
        assertEquals(BridgeErrorCodes.INTERNAL, e.code)
        // daemon 没给描述时退回"RPC <方法> 失败: <码>"，绝不回空串（§5）。
        assertTrue(e.message!!.contains("listSlashCommands"))
        assertTrue(e.message!!.contains("UNIMPLEMENTED"))
    }

    @Test
    fun `七种状态码全覆盖_表内码只有约定的三个`() {
        // 显式钉死"只有三个状态码有专属映射"，其余全部兜底 —— 新增映射必须是
        // 有意识的决定，不会是顺手加一行。
        val mapped = Status.Code.entries
            .filter { contractCodeOf(it) != BridgeErrorCodes.INTERNAL }
            .toSet()
        assertEquals(setOf(Status.Code.UNAVAILABLE, Status.Code.UNAUTHENTICATED), mapped)
    }

    // ── message 口径 ──────────────────────────────────────────────────────

    @Test
    fun `daemon 无描述时 message 退回方法名加状态码_绝不空串`() {
        val e = grpcFailure("listTerminals", grpcError(Status.Code.UNAVAILABLE))
        assertEquals("RPC listTerminals 失败: UNAVAILABLE", e.message)
    }

    @Test
    fun `非 gRPC 异常落 INTERNAL 并带异常类名_宿主内部错误可读`() {
        val e = grpcFailure("getHostInfo", IllegalStateException("本地编码炸了"))
        assertEquals(BridgeErrorCodes.INTERNAL, e.code)
        assertTrue(e.message!!.contains("IllegalStateException"))
        assertTrue(e.message!!.contains("本地编码炸了"))
    }

    @Test
    fun `details 带 gRPC 状态码名_UI 上层可展开诊断`() {
        val e = grpcFailure("listThreads", grpcError(Status.Code.UNAVAILABLE, "x"))
        assertEquals("UNAVAILABLE", e.details)
    }

    // ── 契约码常量本身（与 Dart PaBridgeErrorCodes 逐字同值）────────────────

    @Test
    fun `七个契约码常量与 channel-contract 5 逐字一致`() {
        assertEquals("HOST_UNAVAILABLE", BridgeErrorCodes.HOST_UNAVAILABLE)
        assertEquals("DECODE_FAILED", BridgeErrorCodes.DECODE_FAILED)
        assertEquals("NOT_CONNECTED", BridgeErrorCodes.NOT_CONNECTED)
        assertEquals("AUTH_REQUIRED", BridgeErrorCodes.AUTH_REQUIRED)
        assertEquals("INVALID_ADDR", BridgeErrorCodes.INVALID_ADDR)
        assertEquals("PAIR_DISABLED", BridgeErrorCodes.PAIR_DISABLED)
        assertEquals("INTERNAL", BridgeErrorCodes.INTERNAL)
    }

    @Test
    fun `BridgePlatformException 三件套可原样读出_映射到真PlatformException无损失`() {
        val e = BridgePlatformException("NOT_CONNECTED", "人话", "UNAVAILABLE")
        assertEquals("NOT_CONNECTED", e.code)
        assertEquals("人话", e.message)
        assertEquals("UNAVAILABLE", e.details)
        assertNull(e.cause)
    }

    @Test
    fun `通道名常量与 Dart kPaBridgeMethodChannel 逐字一致`() {
        assertEquals("dev.pocketagent/bridge", BRIDGE_CHANNEL_NAME)
    }
}
