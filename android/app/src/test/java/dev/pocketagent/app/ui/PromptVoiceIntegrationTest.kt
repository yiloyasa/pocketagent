package dev.pocketagent.app.ui

// PromptVoiceIntegrationTest —— 提示词优化与语音输入的跨组件 JVM 集成测试
// (2026-10-04 release-and-end-to-end-validation 计划 Task 4,验证清单第 5/6 条)。
//
// 【组装口径】真 Repository + fake RPC + 真预览归约 + 真语音归约(不 mock 网络):
//  - 提示词(第 5 条):真 GrpcPromptOptimizeRepository(注入 fake callOptimize,
//    生产档 170s deadline 原样透传)+ 真预览状态机 reducePromptOptimizePreview +
//    输入框修订号联动 + 真行级 diff(P4-T2 的纯单测在这里升级为跨组件全链:
//    「仓库 fake → 候选 → 预览归约 → 防覆盖三态 → 应用/覆盖」,输入变化不被覆盖);
//  - 语音(第 6 条):ui/voice 真归约。VoiceSessionReducerTest 既有 17+ 用例已
//    钉死"迟到回调丢弃"(旧 sessionId 门)与"会话切换静音"(静音门)两条半边;
//    本文件只补一条**跨会话完整闭环**(切会话 → 旧 Final 丢弃 → 新会话 Start
//    重开 → 更迟到的旧 Final 仍被 sessionId 门拦住 → 新会话 Final 正常落新会话),
//    与既有用例不重复(既有用例都没把新会话走到出稿落地)。
//
// 被测产物只测不改;voice/** 产物与 internal/** 不触碰。

import dev.pocketagent.app.ui.prompt.GrpcPromptOptimizeRepository
import dev.pocketagent.app.ui.prompt.PromptOptimizePreviewIntent
import dev.pocketagent.app.ui.prompt.PromptOptimizePreviewState
import dev.pocketagent.app.ui.prompt.PromptDiffKind
import dev.pocketagent.app.ui.prompt.isIdle
import dev.pocketagent.app.ui.prompt.reducePromptOptimizePreview
import dev.pocketagent.app.ui.prompt.promptCandidateOverExpanded
import dev.pocketagent.app.ui.prompt.promptLineDiff
import dev.pocketagent.app.ui.prompt.promptOptimizeClickAllowed
import dev.pocketagent.app.ui.voice.VoiceEffect
import dev.pocketagent.app.ui.voice.VoiceEvent
import dev.pocketagent.app.ui.voice.VoicePhase
import dev.pocketagent.app.ui.voice.VoiceSessionState
import dev.pocketagent.app.ui.voice.reduce
import io.grpc.Status
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.OptimizePromptResponse
import pocketagent.v1.OptimizePromptRequest
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference

class PromptVoiceIntegrationTest {

    // ================================================================
    // 5) 优化期间输入变化不会被覆盖(跨组件全链)
    // ================================================================

    @Test
    fun `优化全链_等待期间输入修订_候选不被静默覆盖_二选一覆盖支落地`() {
        var input = "  帮我修登录超时的 bug  "
        var revision = 5L
        val capturedWire = AtomicReference<OptimizePromptRequest>()
        val capturedDeadline = AtomicLong(-1L)
        val rpcEntered = CompletableDeferred<Unit>()
        val rpcRelease = CompletableDeferred<OptimizePromptResponse>()

        val repo = GrpcPromptOptimizeRepository(
            callOptimize = { deadlineSeconds, wire ->
                capturedWire.set(wire)
                capturedDeadline.set(deadlineSeconds)
                rpcEntered.complete(Unit)
                rpcRelease.await() // 挂住:模拟 daemon 长生成(用户在等待期可继续输入)
            },
            connected = { true },
            // ioDispatcher 用默认:fake RPC 的挂起不占真线程
        )

        runBlocking {
            // 发起:点击闸放行 → Started 记原文与发起时刻的输入修订号
            assertTrue(promptOptimizeClickAllowed(PromptOptimizePreviewState(), input))
            var preview = reducePromptOptimizePreview(
                PromptOptimizePreviewState(),
                PromptOptimizePreviewIntent.Started(input, PromptOptimizeStyle.DEFAULT, revision),
                currentRevision = revision,
            )
            assertTrue(preview.loading)
            assertEquals(5L, preview.sourceRevision)

            var rpcResult: Result<String>? = null
            val job = launch {
                rpcResult = repo.optimize(
                    PromptOptimizeRequestModel(
                        original = input,
                        style = PromptOptimizeStyle.DEFAULT,
                        agentId = "zcode",
                        modelId = "pi/zai-coding-cn/glm-5.3",
                        effort = "high",
                        requestVersion = revision,
                    ),
                )
            }
            rpcEntered.await()
            // 跨组件口径:请求模型 → wire 四字段 + 生产档 170s deadline 原样透传
            assertEquals("帮我修登录超时的 bug", capturedWire.get().text)
            assertEquals("high", capturedWire.get().effort)
            assertEquals(170L, capturedDeadline.get())

            // 等待期间用户改输入:setInput 写新文本 + 修订号前进(5 → 6)
            input = "等待期间用户改的新文本"
            revision = 6L

            val candidate = "把登录超时问题说清楚,给出复现步骤"
            rpcRelease.complete(OptimizePromptResponse.newBuilder().setOptimized(candidate).build())
            job.join()
            assertTrue(rpcResult!!.isSuccess)
            assertEquals(candidate, rpcResult!!.getOrThrow())

            // 迟到语境下的结果进预览:候选落位,但修订号已前进 → 防覆盖二选一
            preview = reducePromptOptimizePreview(
                preview,
                PromptOptimizePreviewIntent.Succeeded(rpcResult!!.getOrThrow()),
                currentRevision = revision,
            )
            assertTrue(preview.hasCandidate)
            assertFalse(preview.loading)
            assertTrue(preview.inputModifiedSince(revision)) // 6 != 5:改过

            // 三态之二:盲 Apply 被拒 —— 绝不静默覆盖等待期间的新输入
            val blind = reducePromptOptimizePreview(
                preview, PromptOptimizePreviewIntent.Apply, currentRevision = revision,
            )
            assertEquals(preview, blind)
            assertTrue(blind.hasCandidate)

            // 二选一之「覆盖」:用户显式选择后才应用(状态清空,调用方写输入框)
            val overwritten = reducePromptOptimizePreview(
                preview, PromptOptimizePreviewIntent.OverwriteModified, currentRevision = revision,
            )
            assertTrue(overwritten.isIdle)
            input = candidate // ChatScreen 分发函数的落框动作

            // 二选一之「保留新文本」对照路径:Dismiss 同样干净退场,候选不进框
            var kept = reducePromptOptimizePreview(
                preview, PromptOptimizePreviewIntent.Apply, currentRevision = revision,
            )
            kept = reducePromptOptimizePreview(kept, PromptOptimizePreviewIntent.Dismiss, currentRevision = revision)
            assertTrue(kept.isIdle)
        }
    }

    @Test
    fun `优化全链_未修改直应用_候选进输入框_diff与扩写提示数据就位`() {
        runBlocking {
            // 多行夹具:行级 diff 对"单行原文 vs 单行候选"只会产出整行删+整行加,
            // 无 SAME 行 —— 三类差异齐全必须用多行文本(与既有 diff 纯单测同口径)
            val original = "帮我修登录超时\n给出复现步骤"
            val candidate = "把登录超时问题说清楚\n给出复现步骤"
            val repo = GrpcPromptOptimizeRepository(
                callOptimize = { _, _ -> OptimizePromptResponse.newBuilder().setOptimized(candidate).build() },
                connected = { true },
            )

            var preview = reducePromptOptimizePreview(
                PromptOptimizePreviewState(),
                PromptOptimizePreviewIntent.Started(original, PromptOptimizeStyle.DEFAULT, 3L),
                currentRevision = 3L,
            )
            val result = repo.optimize(
                PromptOptimizeRequestModel(
                    original = original, style = PromptOptimizeStyle.DEFAULT,
                    agentId = "zcode", modelId = "pi/zai-coding-cn/glm-5.3",
                    effort = "high", requestVersion = 3L,
                ),
            )
            preview = reducePromptOptimizePreview(
                preview, PromptOptimizePreviewIntent.Succeeded(result.getOrThrow()), currentRevision = 3L,
            )
            assertTrue(preview.hasCandidate)
            assertFalse(preview.inputModifiedSince(3L)) // 等待期间没动输入

            // 三态之一:未修改 → Apply 直应用,调用方把候选写进输入框
            val applied = reducePromptOptimizePreview(
                preview, PromptOptimizePreviewIntent.Apply, currentRevision = 3L,
            )
            assertTrue(applied.isIdle)

            // 预览渲染数据就位:行级 diff 判出"中段一行被改写、尾行原样保留"
            val diff = promptLineDiff(original, candidate)
            assertTrue(diff.any { it.kind == PromptDiffKind.REMOVED })
            assertTrue(diff.any { it.kind == PromptDiffKind.ADDED })
            assertTrue(diff.any { it.kind == PromptDiffKind.SAME })
            // 量级正常的候选不触发扩写告警
            assertFalse(promptCandidateOverExpanded(original, candidate))
        }
    }

    @Test
    fun `优化全链_超时失败原文槽位不动_人话上屏_重试换成功`() {
        var failMode = true
        runBlocking {
            val original = "帮我修登录超时"
            val repo = GrpcPromptOptimizeRepository(
                callOptimize = { _, _ ->
                    if (failMode) {
                        throw Status.DEADLINE_EXCEEDED.asRuntimeException()
                    } else {
                        OptimizePromptResponse.newBuilder().setOptimized("第二次成功的候选").build()
                    }
                },
                connected = { true },
            )

            var preview = reducePromptOptimizePreview(
                PromptOptimizePreviewState(),
                PromptOptimizePreviewIntent.Started(original, PromptOptimizeStyle.DEFAULT, 2L),
                currentRevision = 2L,
            )
            val failed = repo.optimize(
                PromptOptimizeRequestModel(
                    original = original, style = PromptOptimizeStyle.DEFAULT,
                    agentId = "zcode", modelId = "pi/zai-coding-cn/glm-5.3",
                    effort = "high", requestVersion = 2L,
                ),
            )
            assertTrue(failed.isFailure)
            val ex = failed.exceptionOrNull()!! as PromptOptimizeException
            assertEquals(PromptOptimizeErrorKind.TIMEOUT, ex.kind)
            preview = reducePromptOptimizePreview(
                preview, PromptOptimizePreviewIntent.Failed(ex.humanMessage), currentRevision = 2L,
            )
            // 失败只写错误位:原文槽位分毫不动(防覆盖契约的失败半边)
            assertEquals("优化超时了,请稍后再试(原文未被改动)", preview.error)
            assertEquals(original, preview.original)
            assertFalse(preview.hasCandidate)
            assertTrue(preview.original.isNotBlank()) // 重试的原料还在

            // 重试 → 成功 → 候选落位(同一原文、同一风格)
            failMode = false
            preview = reducePromptOptimizePreview(
                preview, PromptOptimizePreviewIntent.Retry, currentRevision = 2L,
            )
            assertTrue(preview.loading)
            assertNull(preview.error)
            val retried = repo.optimize(
                PromptOptimizeRequestModel(
                    original = preview.original, style = preview.style,
                    agentId = "zcode", modelId = "pi/zai-coding-cn/glm-5.3",
                    effort = "high", requestVersion = 2L,
                ),
            )
            preview = reducePromptOptimizePreview(
                preview, PromptOptimizePreviewIntent.Succeeded(retried.getOrThrow()), currentRevision = 2L,
            )
            assertTrue(preview.hasCandidate && !preview.loading)
        }
    }

    // ================================================================
    // 6) 语音迟到回调不会进入新会话(跨会话完整闭环,不与既有用例重复)
    // ================================================================

    /**
     * 与 VoiceSessionReducerTest 既有两条的分工(避免重复覆盖):
     *  - `迟到回调_旧sessionId的Partial_Final_Error一律丢弃_不污染新会话` 钉的是
     *    sessionId 门这半边;
     *  - `会话切换_置静音清文本停引擎_静音期回调一律丢弃_新Start解除` 钉的是静音门
     *    与解除,但停在 REQUESTING_PERMISSION,**没有把新会话走到出稿**。
     * 本条补完整闭环:切会话 → 旧 Final 被静音门丢弃 → 新会话 Start 重开发号 →
     * 双保险(解除静音后更迟到的旧 sessionId Final 仍被 sessionId 门拦)→
     * 新会话引擎链路 → 新 Final 正常落**新会话**的 finalText。
     */
    @Test
    fun `语音跨会话_切会话旧Final丢弃_新会话重开双保险拦旧稿_新Final正常落`() {
        // 旧会话(thread-1):发号 sid=1,权限过、引擎起、说到一半
        var s = reduce(VoiceSessionState(threadId = "thread-1"), VoiceEvent.Start).state.copy(sessionId = 1L)
        val started = reduce(s, VoiceEvent.Permission(true))
        assertEquals(VoiceEffect.StartEngine(1L), started.effect)
        s = started.state
        s = reduce(s, VoiceEvent.Partial(1L, "切走前说的")).state
        assertEquals(VoicePhase.LISTENING, s.phase)

        // 切会话(thread-2):停引擎 + 静音 + 残余文本清空
        val switched = reduce(s, VoiceEvent.SessionSwitched("thread-2"))
        assertEquals(VoiceEffect.StopEngine, switched.effect)
        s = switched.state
        assertTrue(s.mutedByThreadSwitch)
        assertEquals("thread-2", s.threadId)
        assertEquals("", s.partialText)

        // 迟到的旧 Final:静音门丢弃,不进新会话
        val lateFinal = reduce(s, VoiceEvent.Final(1L, "旧会话迟到定稿"))
        assertSame(s, lateFinal.state)
        assertNull(lateFinal.effect)

        // 用户在新会话主动 Start:解除静音、新一轮发号(Controller 分配 sid=2)
        s = reduce(s, VoiceEvent.Start).state.copy(sessionId = 2L)
        assertFalse(s.mutedByThreadSwitch)

        // 双保险:解除静音后,更迟到的旧 sessionId Final 仍被 sessionId 门拦住
        val laterFinal = reduce(s, VoiceEvent.Final(1L, "更迟到的旧定稿"))
        assertSame(s, laterFinal.state)
        assertNull(laterFinal.effect)
        assertEquals("", laterFinal.state.finalText)

        // 新会话正常链路:权限 → 引擎(带新会话锚)→ 收音 → 出稿落新会话
        val perm = reduce(s, VoiceEvent.Permission(true))
        assertEquals(VoiceEffect.StartEngine(2L), perm.effect)
        val listening = reduce(perm.state, VoiceEvent.Partial(2L, "新会话说的"))
        assertEquals(VoicePhase.LISTENING, listening.state.phase)
        val final = reduce(listening.state, VoiceEvent.Final(2L, "新会话定稿"))
        assertEquals(VoicePhase.IDLE, final.state.phase)
        assertEquals("新会话定稿", final.state.finalText) // 新稿落新会话,旧稿从未到达
        assertEquals("", final.state.partialText)
        assertEquals("thread-2", final.state.threadId)
        assertFalse(final.state.mutedByThreadSwitch)
    }
}
