package dev.pocketagent.app.ui.prompt

import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.ui.PROMPT_OPTIMIZE_DEADLINE_S
import dev.pocketagent.app.ui.PromptOptimizeErrorKind
import dev.pocketagent.app.ui.PromptOptimizeException
import dev.pocketagent.app.ui.PromptOptimizeRequestModel
import dev.pocketagent.app.ui.classifyPromptOptimizeFailure
import io.grpc.Status
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import pocketagent.v1.OptimizePromptRequest
import pocketagent.v1.OptimizePromptResponse
import java.util.concurrent.TimeUnit

/**
 * 提示词优化仓库(P4 Task 1):把"一次优化请求"从 ChatScreen 的内联 RPC 收敛成
 * 独立、可注入、可 JVM 单测的边界。
 *
 * 【全局约束在本层的落法】
 *  - 失败/超时/空返回**不改变原文**:本层不持有输入框,只返回 Result;
 *    调用方只有拿到 success 才允许替换输入,任何 failure(含取消)都必须让
 *    原文保持不动。
 *  - 优化结果**不能自动发送**:success 只是候选文本,发送与否由用户决定
 *    (Task 2 的预览确认层落地)。
 */
internal interface PromptOptimizeRepository {
    /**
     * 发起一次优化。
     *
     * 成功:`Result.success(改写文本,已 trim,非空)`。
     * 失败:`Result.failure(PromptOptimizeException)`,message 是可直接展示的
     * 人话,kind 是机器可读分类;取消归 [PromptOptimizeErrorKind.CANCELLED]。
     * 本层从不抛业务异常(结构化取消的取舍见实现注释),调用方无需 try/catch。
     */
    suspend fun optimize(request: PromptOptimizeRequestModel): Result<String>
}

/**
 * gRPC 实现:经 [AgentConnection] 既有 ThreadService blocking stub 调
 * OptimizePrompt(现状封装:threadService() 已带 20s 列表档 deadline,本层在
 * 其上再 withDeadlineAfter 覆盖为优化专用档)。
 *
 * 【独立 deadline】优化是长请求(daemon 侧整段预算 150s:冷启 + 候选模型逐个
 * 尝试),不能复用 threadService() 默认的 20s 列表档。deadline 取
 * [PROMPT_OPTIMIZE_DEADLINE_S](170s = daemon 150s 预算 + 回程余量,依据见该
 * 常量注释):保证 daemon 的错误文案能跑完回程,而不是被手机先掐成裸超时。
 * 构造参数可注入小值供测试。
 *
 * 【取消语义】取消(用户退出/换会话/发起新请求)返回
 * [PromptOptimizeErrorKind.CANCELLED] 的 failure,调用方据此**不覆盖输入、
 * 不报错 Toast**。取舍:这里把 CancellationException 收进 Result 而不是再抛 ——
 * 每次 optimize 都跑在调用方自有的短命协程里,返回后只剩非挂起的 UI 状态更新,
 * 吞掉取消不会泄漏后续工作;换来调用方"只看 Result、零 try/catch"的可测面。
 * 若未来在 optimize 之后追加挂起调用,需先 review 本注释。
 *
 * 【style/effort 透传补丁】现状 ChatScreen 内联组装 wire 请求只 set
 * text/agentId/modelId,proto 的 style/effort 从未进 wire;本层按请求模型统一
 * 组装四字段 + 风格,ChatScreen 接线归 P4 波 3。
 */
internal class GrpcPromptOptimizeRepository(
    /**
     * 到 wire 的最后一跳,独立成 lambda 以便 JVM 单测注入伪实现(生产装配见
     * [fromAgentConnection]):参数是仓库持有的独立 deadline 秒数与已组装好的
     * wire 请求,返回 daemon 应答。
     */
    private val callOptimize: suspend (deadlineSeconds: Long, wire: OptimizePromptRequest) -> OptimizePromptResponse,
    /** 优化专用 deadline(秒),默认 170s,依据见类注释。 */
    private val deadlineSeconds: Long = PROMPT_OPTIMIZE_DEADLINE_S,
    /** 在线预检,与既有 Toast 分类同口径(connState == "ok");注入以便测试。 */
    private val connected: () -> Boolean = { AgentConnection.connState.value == "ok" },
    /** blocking gRPC 的执行线程;默认 IO,测试可注入直通。 */
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
) : PromptOptimizeRepository {

    override suspend fun optimize(request: PromptOptimizeRequestModel): Result<String> {
        // 空白原文短路:proto 契约要求 text trim 后非空,不发必然失败的 RPC。
        if (request.original.isBlank()) {
            return Result.failure(classifyPromptOptimizeFailure("INVALID_ARGUMENT", null, connected = true))
        }
        // 未连接短路:与既有 optimizePromptErrorText 的 connected 短路同一条链。
        if (!connected()) {
            return Result.failure(classifyPromptOptimizeFailure(null, null, connected = false))
        }
        val wire = OptimizePromptRequest.newBuilder()
            .setText(request.original.trim()) // proto: trim 后必须非空
            .setStyle(request.style.wireValue) // 枚举→daemon 字符串词表,见 PromptOptimizeStyle
            .setAgentId(request.agentId)
            .setModelId(request.modelId)
            .setEffort(request.effort)
            .build()
        return try {
            val resp = withContext(ioDispatcher) { callOptimize(deadlineSeconds, wire) }
            val optimized = resp.optimized.trim()
            if (optimized.isEmpty()) {
                // daemon 侧保证空返回必报错,这里是双保险:绝不拿空串当成功。
                Result.failure(PromptOptimizeException(PromptOptimizeErrorKind.EMPTY, "优化结果为空,原文未被改动"))
            } else {
                Result.success(optimized)
            }
        } catch (e: CancellationException) {
            // 用户取消/离开页面/新请求顶掉旧请求:语义是"原文未被改动",不是错误。
            Result.failure(PromptOptimizeException(PromptOptimizeErrorKind.CANCELLED, "优化已取消,原文未被改动"))
        } catch (e: Exception) {
            val status = Status.fromThrowable(e)
            Result.failure(
                classifyPromptOptimizeFailure(
                    statusCode = status.code.name,
                    description = status.description,
                    // catch 时重读在线状态:断连若发生在请求途中,归"连接"而不是误导性的其他类。
                    connected = connected(),
                ),
            )
        }
    }

    companion object {
        /** 生产装配:走 AgentConnection 既有 ThreadService blocking stub(现状封装)。 */
        fun fromAgentConnection(): GrpcPromptOptimizeRepository =
            GrpcPromptOptimizeRepository(
                callOptimize = { deadlineSeconds, wire ->
                    AgentConnection.threadService()
                        .withDeadlineAfter(deadlineSeconds, TimeUnit.SECONDS)
                        .optimizePrompt(wire)
                },
            )
    }
}
