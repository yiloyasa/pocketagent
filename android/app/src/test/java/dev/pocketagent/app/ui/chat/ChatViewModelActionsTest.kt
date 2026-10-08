package dev.pocketagent.app.ui.chat

// ChatViewModel 会话动作的 JVM 钉子(2026-10-09 效果层外移 · 任务2/任务1 阶段②)。
//
// 【fake 缝】ChatRpcGateway 是 ChatViewModel 的 RPC 注入缝(学 HistorySearchRepository
// 的 lambda 注入先例):本测试注入 fake 子类,不 mock 网络、不碰真 channel、不触
// Dispatchers.Main(sessionScope 的 launch 路径 —— bindSession/startModelInspection ——
// 在 JVM 不可驱动,不在本文件范围)。
//
// 覆盖面(与迁出前 ChatScreen 效果层逐字对应):
//  1. 八个会话动作的 Result 包装:成功携带 daemon 回包;失败透传原异常实例与
//     message(UI 层 catch(e) 的 `e.message ?: "…失败"` 文案依赖这一点);
//  2. clearQueuedInputs 的回执条数映射(resp.steered/followUp → QueueCounts);
//  3. setApprovalPolicy 的入参透传(approvalPolicy/sandbox 原样进请求);
//  4. loadThreadHead 成功路径:getThread 回包 → ThreadHeaderLoaded 四项落位、
//     ThreadHeadResult(modelId/goalLine)映射、待审批透传 TurnSession、失败路径
//     loadError 原文案("无法加载会话：<msg>",消息空时退"连接失败")并返回 null。

import dev.pocketagent.app.ui.TurnSession
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ApprovalRequest
import pocketagent.v1.ClearQueuedInputsResponse
import pocketagent.v1.CodexSettings
import pocketagent.v1.GetThreadHistoryResponse
import pocketagent.v1.ListModelsResponse
import pocketagent.v1.Thread as PThread

class ChatViewModelActionsTest {

    /** 全记录 fake:默认成功返回默认实例;按方法塞异常即测失败路径。 */
    private class FakeGateway : ChatRpcGateway {
        val calls = mutableListOf<String>()

        var forkResult: PThread = PThread.newBuilder().setId("fork-1").setTitle("分叉副本").build()
        var forkError: Exception? = null
        var rollbackError: Exception? = null
        var compactError: Exception? = null
        var reviewError: Exception? = null
        var clearQueuedResponse = ClearQueuedInputsResponse.newBuilder().setSteered(1).setFollowUp(2).build()
        var clearQueuedError: Exception? = null
        var codexSettingsResult = CodexSettings.getDefaultInstance()
        var lastApprovalPolicy: String? = null
        var lastSandbox: String? = null
        var guardianError: Exception? = null
        var goalResult: PThread = PThread.getDefaultInstance()
        var lastGoal: String? = null

        var threadResponse: PThread = PThread.getDefaultInstance()
        var threadError: Exception? = null

        override suspend fun getThread(threadId: String): PThread {
            calls += "getThread:$threadId"
            threadError?.let { throw it }
            return threadResponse
        }

        override suspend fun threadHistory(threadId: String, rounds: Int): GetThreadHistoryResponse {
            calls += "threadHistory:$threadId:$rounds"
            return GetThreadHistoryResponse.getDefaultInstance()
        }

        override suspend fun forkThread(threadId: String): PThread {
            calls += "fork:$threadId"
            forkError?.let { throw it }
            return forkResult
        }

        override suspend fun rollbackThread(threadId: String, turns: Int) {
            calls += "rollback:$threadId:$turns"
            rollbackError?.let { throw it }
        }

        override suspend fun compactThread(threadId: String) {
            calls += "compact:$threadId"
            compactError?.let { throw it }
        }

        override suspend fun startReview(threadId: String, focus: String) {
            calls += "review:$threadId:$focus"
            reviewError?.let { throw it }
        }

        override suspend fun clearQueuedInputs(threadId: String): ClearQueuedInputsResponse {
            calls += "clearQueued:$threadId"
            clearQueuedError?.let { throw it }
            return clearQueuedResponse
        }

        override suspend fun setCodexSettings(approvalPolicy: String, sandbox: String): CodexSettings {
            calls += "setCodexSettings:$approvalPolicy:$sandbox"
            lastApprovalPolicy = approvalPolicy
            lastSandbox = sandbox
            return codexSettingsResult
        }

        override suspend fun approveGuardianDeniedAction(threadId: String, reviewId: String, targetItemId: String) {
            calls += "guardian:$threadId:$reviewId:$targetItemId"
            guardianError?.let { throw it }
        }

        override suspend fun setThreadGoal(threadId: String, goal: String): PThread {
            calls += "setGoal:$threadId:$goal"
            lastGoal = goal
            return goalResult
        }

        override suspend fun listModels(agentId: String, threadId: String): ListModelsResponse {
            calls += "listModels:$agentId:$threadId"
            return ListModelsResponse.getDefaultInstance()
        }
    }

    private fun vmWith(gateway: FakeGateway): ChatViewModel =
        ChatViewModel(stepRounds = 20).also { it.gateway = gateway }

    // ---- 1. Result 包装:成功携带回包 / 失败透传原异常 ----

    @Test
    fun `forkThread 成功包装 Result 携带 daemon 回包`() = kotlinx.coroutines.runBlocking {
        val gw = FakeGateway()
        val vm = vmWith(gw)
        val r = vm.forkThread("t-fork")
        assertTrue(r.isSuccess)
        assertEquals("fork-1", r.getOrThrow().id)
        assertEquals("分叉副本", r.getOrThrow().title)
        assertEquals(listOf("fork:t-fork"), gw.calls)
    }

    @Test
    fun `forkThread 失败透传原异常实例与消息`() = kotlinx.coroutines.runBlocking {
        val gw = FakeGateway()
        val boom = RuntimeException("电脑不在线")
        gw.forkError = boom
        val vm = vmWith(gw)
        val r = vm.forkThread("t-fork")
        assertTrue(r.isFailure)
        assertSame(boom, r.exceptionOrNull())
        assertEquals("电脑不在线", r.exceptionOrNull()?.message)
    }

    @Test
    fun `rollbackThread_compactThread_startReview 失败透传原异常消息`() = kotlinx.coroutines.runBlocking {
        val gw = FakeGateway()
        gw.rollbackError = RuntimeException("回滚被拒")
        gw.compactError = RuntimeException("压缩被拒")
        gw.reviewError = RuntimeException("review 拒绝")
        val vm = vmWith(gw)
        assertEquals("回滚被拒", vm.rollbackThread("t", 2).exceptionOrNull()?.message)
        assertEquals("压缩被拒", vm.compactThread("t").exceptionOrNull()?.message)
        assertEquals("review 拒绝", vm.startReview("t", "重点").exceptionOrNull()?.message)
        // 成功腿:Result<Unit> 且调用参数原样到达网关
        val gw2 = FakeGateway()
        assertTrue(vmWith(gw2).rollbackThread("t-ok", 3).isSuccess)
        assertTrue(gw2.calls.contains("rollback:t-ok:3"))
    }

    @Test
    fun `clearQueuedInputs 映射回执条数 steered_followUp`() = kotlinx.coroutines.runBlocking {
        val gw = FakeGateway()
        gw.clearQueuedResponse = ClearQueuedInputsResponse.newBuilder().setSteered(1).setFollowUp(2).build()
        val r = vmWith(gw).clearQueuedInputs("t-q")
        assertTrue(r.isSuccess)
        assertEquals(1, r.getOrThrow().steered)
        assertEquals(2, r.getOrThrow().followUp)
        // 失败透传
        gw.clearQueuedError = RuntimeException("只接 pi 会话")
        val f = vmWith(gw).clearQueuedInputs("t-q")
        assertEquals("只接 pi 会话", f.exceptionOrNull()?.message)
    }

    @Test
    fun `setApprovalPolicy 入参原样进请求且回包入 Result`() = kotlinx.coroutines.runBlocking {
        val gw = FakeGateway()
        val authoritative = CodexSettings.newBuilder().setApprovalPolicy("on-request").setSandbox("workspace-write").build()
        gw.codexSettingsResult = authoritative
        val vm = vmWith(gw)
        val r = vm.setApprovalPolicy(sandbox = "workspace-write", approvalPolicy = "on-request")
        assertTrue(r.isSuccess)
        assertSame(authoritative, r.getOrThrow())
        assertEquals("on-request", gw.lastApprovalPolicy)
        assertEquals("workspace-write", gw.lastSandbox)
    }

    @Test
    fun `approveGuardianDenied 参数原样透传且失败包装`() = kotlinx.coroutines.runBlocking {
        val gw = FakeGateway()
        val vm = vmWith(gw)
        assertTrue(vm.approveGuardianDenied("t-g", "review-9", "item-3").isSuccess)
        assertTrue(gw.calls.contains("guardian:t-g:review-9:item-3"))
        gw.guardianError = RuntimeException("审查结论已过期")
        assertEquals("审查结论已过期", vm.approveGuardianDenied("t-g", "review-9", "item-3").exceptionOrNull()?.message)
    }

    @Test
    fun `setThreadGoal 目标原文照传并回包 Thread`() = kotlinx.coroutines.runBlocking {
        val gw = FakeGateway()
        gw.goalResult = PThread.newBuilder().setGoal("全程用 Kotlin").build()
        val vm = vmWith(gw)
        val r = vm.setThreadGoal("t-goal", "全程用 Kotlin")
        assertTrue(r.isSuccess)
        assertEquals("全程用 Kotlin", r.getOrThrow().goal)
        assertEquals("全程用 Kotlin", gw.lastGoal)
    }

    // ---- loadThreadHead(任务1 阶段②):成功映射 / 失败 loadError 原文案 ----

    @Test
    fun `loadThreadHead 成功回填头部四项并映射 modelId_goalLine`() = kotlinx.coroutines.runBlocking {
        val gw = FakeGateway()
        gw.threadResponse = PThread.newBuilder()
            .setTitle("修复登录页")
            .setStatus(pocketagent.v1.ThreadStatus.THREAD_STATUS_IDLE)
            .setWorkspace("/home/me/repo")
            .setAgentId("codex")
            .setModelId("zai-coding-cn/glm-5.3")
            .setGoal("全程用 Kotlin")
            .build()
        val vm = vmWith(gw)
        val head = vm.loadThreadHead("t-head-1")
        assertNotNull(head)
        assertEquals("zai-coding-cn/glm-5.3", head!!.modelId)
        assertEquals("目标:全程用 Kotlin", head.goalLine)
        // ThreadHeaderLoaded 四项落位(空工作区→null / 空 agentId→"codex" 的原式在 reducer)
        val s = vm.state.value
        assertEquals("修复登录页", s.threadTitle)
        assertEquals("/home/me/repo", s.threadWorkspace)
        assertEquals("codex", s.threadAgentId)
        assertNull(s.loadError)
        assertEquals("t-head-1", gw.calls.single().substringAfter("getThread:"))
        // 无待审批时 TurnSession 不被误种
        assertNull(TurnSession.state("t-head-1").pendingApproval)
    }

    @Test
    fun `loadThreadHead 待审批透传到 TurnSession`() = kotlinx.coroutines.runBlocking {
        val gw = FakeGateway()
        gw.threadResponse = PThread.newBuilder()
            .setStatus(pocketagent.v1.ThreadStatus.THREAD_STATUS_IDLE)
            .addPendingApprovals(ApprovalRequest.newBuilder().setRequestId("ap-1"))
            .build()
        val head = vmWith(gw).loadThreadHead("t-head-2")
        assertNotNull(head)
        assertNotNull(TurnSession.state("t-head-2").pendingApproval)
        assertEquals("ap-1", TurnSession.state("t-head-2").pendingApproval?.requestId)
    }

    @Test
    fun `loadThreadHead 失败返回 null 并落 loadError 原文案`() = kotlinx.coroutines.runBlocking {
        val gw = FakeGateway()
        gw.threadError = RuntimeException("连接失败:超时")
        val vm = vmWith(gw)
        assertNull(vm.loadThreadHead("t-head-3"))
        assertEquals("无法加载会话：连接失败:超时", vm.state.value.loadError)
        // 消息为空的异常退"连接失败"兜底(迁出前 elvis 原式)
        val gw2 = FakeGateway()
        gw2.threadError = RuntimeException()
        val vm2 = vmWith(gw2)
        assertNull(vm2.loadThreadHead("t-head-4"))
        assertEquals("无法加载会话：连接失败", vm2.state.value.loadError)
    }
}
