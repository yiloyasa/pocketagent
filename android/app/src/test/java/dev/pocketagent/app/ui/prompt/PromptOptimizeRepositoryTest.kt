package dev.pocketagent.app.ui.prompt

// PromptOptimizeRepositoryTest —— 优化仓库(P4 Task 1)的 JVM 单测,逐条钉:
//   1) 请求参数组装:style/effort/agentId/modelId 真正进 wire(透传补丁)、原文 trim、
//      独立 deadline 透传到最后一跳;
//   2) 空响应:失败而非成功,绝不拿空串当改写结果;
//   3) 取消语义:CANCELLED kind,不是错误文案;
//   4) gRPC 错误人话映射四类:配额/登录/连接/超时 + daemon 人话透传。
// 注入 callOptimize 伪实现,不碰真 channel(Android 依赖不出现在本测试)。

import dev.pocketagent.app.ui.PromptOptimizeErrorKind
import dev.pocketagent.app.ui.PromptOptimizeException
import dev.pocketagent.app.ui.PromptOptimizeRequestModel
import dev.pocketagent.app.ui.PromptOptimizeStyle
import io.grpc.Status
import io.grpc.StatusRuntimeException
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.OptimizePromptRequest
import pocketagent.v1.OptimizePromptResponse
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

class PromptOptimizeRepositoryTest {

    /** 组装测试用请求模型(带前后空白,验证 trim 口径)。 */
    private fun requestModel(
        style: PromptOptimizeStyle = PromptOptimizeStyle.DEFAULT,
        original: String = "  帮我修登录超时的 bug  ",
    ) = PromptOptimizeRequestModel(
        original = original,
        style = style,
        agentId = "zcode",
        modelId = "pi/zai-coding-cn/glm-5.3",
        effort = "high",
        requestVersion = 7L,
    )

    /** 成功应答构造器。 */
    private fun ok(optimized: String): OptimizePromptResponse =
        OptimizePromptResponse.newBuilder().setOptimized(optimized).build()

    private fun grpcError(code: Status, description: String?): Exception {
        val s = if (description == null) code else code.withDescription(description)
        return StatusRuntimeException(s)
    }

    // ---- 1. 请求参数组装:style/effort/agent/model 真正进 wire ----

    @Test
    fun `请求参数全部进 wire 且原文 trim deadline 透传`() = runBlocking {
        val capturedWire = AtomicReference<OptimizePromptRequest>()
        val capturedDeadline = AtomicReference(-1L)
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { deadlineSeconds, wire ->
                capturedWire.set(wire)
                capturedDeadline.set(deadlineSeconds)
                ok("  把登录超时问题说清楚,给出复现步骤  ")
            },
            deadlineSeconds = 42L, // 注入小值,验证"独立 deadline 传到最后一跳"
            connected = { true },
        )

        val result = repo.optimize(requestModel(style = PromptOptimizeStyle.CONCISE))

        assertTrue(result.isSuccess)
        // 成功值也是 trim 后的候选文本
        assertEquals("把登录超时问题说清楚,给出复现步骤", result.getOrThrow())
        val wire = capturedWire.get()
        // 【透传补丁的钉子】现状 ChatScreen 内联组装只 set text/agent/model,
        // 这里钉死 style/effort 必须同车进 wire
        assertEquals("帮我修登录超时的 bug", wire.text)
        assertEquals("concise", wire.style)
        assertEquals("high", wire.effort)
        assertEquals("zcode", wire.agentId)
        assertEquals("pi/zai-coding-cn/glm-5.3", wire.modelId)
        assertEquals(42L, capturedDeadline.get().toLong())
    }

    @Test
    fun `默认风格发空串 detailed 发 detailed`() = runBlocking {
        val capturedWire = AtomicReference<OptimizePromptRequest>()
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, wire ->
                capturedWire.set(wire); ok("改写")
            },
            connected = { true },
        )
        repo.optimize(requestModel(style = PromptOptimizeStyle.DEFAULT))
        assertEquals("", capturedWire.get().style) // 默认 = daemon 的空串分支
        repo.optimize(requestModel(style = PromptOptimizeStyle.IMPLEMENTATION_PLAN))
        assertEquals("implementation_plan", capturedWire.get().style)
    }

    // ---- 2. 空响应:明确失败 ----

    @Test
    fun `空响应返回 EMPTY 失败不当成功`() = runBlocking {
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, _ -> ok("") },
            connected = { true },
        )
        val result = repo.optimize(requestModel())
        assertTrue(result.isFailure)
        val e = result.exceptionOrNull() as PromptOptimizeException
        assertEquals(PromptOptimizeErrorKind.EMPTY, e.kind)
        assertEquals("优化结果为空,原文未被改动", e.message)
    }

    @Test
    fun `纯空白响应视同空响应`() = runBlocking {
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, _ -> ok("   \n ") },
            connected = { true },
        )
        val result = repo.optimize(requestModel())
        val e = result.exceptionOrNull() as PromptOptimizeException
        assertEquals(PromptOptimizeErrorKind.EMPTY, e.kind)
    }

    // ---- 短路:不发必然失败的 RPC ----

    @Test
    fun `空白原文短路不发请求`() = runBlocking {
        val calls = AtomicInteger(0)
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, _ -> calls.incrementAndGet(); ok("不该被调用") },
            connected = { true },
        )
        val result = repo.optimize(requestModel(original = "   "))
        assertTrue(result.isFailure)
        val e = result.exceptionOrNull() as PromptOptimizeException
        assertEquals(PromptOptimizeErrorKind.OTHER, e.kind)
        assertEquals("先输入内容,再点优化", e.message)
        assertEquals(0, calls.get())
    }

    @Test
    fun `未连接短路归 CONNECTION 且不发请求`() = runBlocking {
        val calls = AtomicInteger(0)
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, _ -> calls.incrementAndGet(); ok("不该被调用") },
            connected = { false },
        )
        val result = repo.optimize(requestModel())
        val e = result.exceptionOrNull() as PromptOptimizeException
        assertEquals(PromptOptimizeErrorKind.CONNECTION, e.kind)
        assertEquals("未连接电脑,请先连接再优化", e.message)
        assertEquals(0, calls.get())
    }

    // ---- 3. 取消语义 ----

    @Test
    fun `取消返回 CANCELLED 失败而非通用错误`() = runBlocking {
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, _ -> throw CancellationException("调用方协程被取消") },
            connected = { true },
        )
        val result = repo.optimize(requestModel())
        assertTrue(result.isFailure)
        val e = result.exceptionOrNull() as PromptOptimizeException
        assertEquals(PromptOptimizeErrorKind.CANCELLED, e.kind)
        assertEquals("优化已取消,原文未被改动", e.message)
        // 取消不是错误文案,不能落进 OTHER/TIMEOUT 误导用户重试
        assertFalse(e.message!!.contains("失败"))
        assertFalse(e.message!!.contains("超时"))
    }

    // ---- 4. gRPC 错误人话映射四类 ----

    @Test
    fun `配额错误映射 QUOTA 且 daemon 人话透传`() = runBlocking {
        val daemonText = "电脑端 ZCode 配额已用完,暂时无法优化;可直接发送原文,或稍后再试"
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, _ -> throw grpcError(Status.UNAVAILABLE, daemonText) },
            connected = { true },
        )
        val result = repo.optimize(requestModel())
        val e = result.exceptionOrNull() as PromptOptimizeException
        assertEquals(PromptOptimizeErrorKind.QUOTA, e.kind) // 关键词改判,code 本身是 UNAVAILABLE
        assertEquals(daemonText, e.message)
    }

    @Test
    fun `配额 code 无描述时兜底人话`() = runBlocking {
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, _ -> throw grpcError(Status.RESOURCE_EXHAUSTED, null) },
            connected = { true },
        )
        val e = repo.optimize(requestModel()).exceptionOrNull() as PromptOptimizeException
        assertEquals(PromptOptimizeErrorKind.QUOTA, e.kind)
        assertEquals("优化配额已用尽,可直接发送原文,或稍后再试", e.message)
    }

    @Test
    fun `登录错误映射 AUTH`() = runBlocking {
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, _ -> throw grpcError(Status.UNAUTHENTICATED, null) },
            connected = { true },
        )
        val e = repo.optimize(requestModel()).exceptionOrNull() as PromptOptimizeException
        assertEquals(PromptOptimizeErrorKind.AUTH, e.kind)
        assertEquals("电脑端登录状态失效,请在电脑端重新登录后重试", e.message)
    }

    @Test
    fun `连接错误映射 CONNECTION`() = runBlocking {
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, _ -> throw grpcError(Status.UNAVAILABLE, null) },
            connected = { true },
        )
        val e = repo.optimize(requestModel()).exceptionOrNull() as PromptOptimizeException
        assertEquals(PromptOptimizeErrorKind.CONNECTION, e.kind)
        assertEquals("连接电脑中断或 ZCode 不可用,请检查连接与桌面端 ZCode", e.message)
    }

    @Test
    fun `请求途中断连也归 CONNECTION`() = runBlocking {
        // catch 时重读 connected():请求发出去之后才掉线,不能误导成"失败请重试"
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, _ -> throw grpcError(Status.UNKNOWN, null) },
            connected = { false },
        )
        val e = repo.optimize(requestModel()).exceptionOrNull() as PromptOptimizeException
        assertEquals(PromptOptimizeErrorKind.CONNECTION, e.kind)
        assertEquals("未连接电脑,请先连接再优化", e.message)
    }

    @Test
    fun `超时错误映射 TIMEOUT`() = runBlocking {
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, _ -> throw grpcError(Status.DEADLINE_EXCEEDED, null) },
            connected = { true },
        )
        val e = repo.optimize(requestModel()).exceptionOrNull() as PromptOptimizeException
        assertEquals(PromptOptimizeErrorKind.TIMEOUT, e.kind)
        assertEquals("优化超时了,请稍后再试(原文未被改动)", e.message)
    }

    @Test
    fun `未知异常兜底 OTHER 人话`() = runBlocking {
        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { _, _ -> throw RuntimeException("boom") },
            connected = { true },
        )
        val e = repo.optimize(requestModel()).exceptionOrNull() as PromptOptimizeException
        assertEquals(PromptOptimizeErrorKind.OTHER, e.kind)
        assertEquals("优化失败,请稍后再试(原文未被改动)", e.message)
    }
}
