package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.CollabPolicy
import pocketagent.v1.HostDeviceHealth
import pocketagent.v1.HostIdentityHealth
import pocketagent.v1.HostLostQueueInput
import pocketagent.v1.HostRecoveryReport
import pocketagent.v1.HostStoreHealth
import pocketagent.v1.HostTurnJournalHealth
import pocketagent.v1.RuntimeContractIssue
import pocketagent.v1.RuntimeContractReport
import pocketagent.v1.RuntimeLimitation
import pocketagent.v1.RuntimeLimits
import pocketagent.v1.RuntimeStateResponse
import pocketagent.v1.UnknownEventSummary

/**
 * 运行时控制面(W11)Android 数据层的映射层单测。
 *
 * 【为什么这一层必须钉死】它是"daemon 说了什么"到"手机显示什么"的唯一转换点。
 * 这里最容易犯的错不是崩溃,而是**静默造值**:proto3 的 optional 项缺席时读 getter
 * 会得到 0/false,于是"没设上限"被显示成"封禁"、"没配策略"被显示成"全部关闭" ——
 * 界面上看起来完全正常,用户却据此做了错误决定(规格 §21.2、开工清单「已知坑」第一条)。
 * 所以每个 `*_未设置时不造值` 用例都是一颗钉子。
 */
class RuntimeStateLogicTest {

    private fun okReport(): HostRecoveryReport = HostRecoveryReport.newBuilder()
        .setStatus("ok")
        .setSummary("启动正常:会话库、设备表、身份文件均已就绪")
        .setStore(HostStoreHealth.newBuilder().setOutcome("clean").setHasBackup(true))
        .setTurnJournal(HostTurnJournalHealth.newBuilder().setPendingCount(0))
        .setDevices(HostDeviceHealth.newBuilder().setDegraded(false))
        .setIdentity(HostIdentityHealth.newBuilder().setDegraded(false))
        .build()

    // ---- 入口能力判定 ----------------------------------------------------------

    @Test
    fun `旧daemon不带能力声明时整块入口不点亮`() {
        // 规格 §21.3:旧 daemon 没有 runtime.v1 → 隐藏整块入口,而不是摆一排
        // 点了必然报错(或不报错但永远空)的假开关。
        assertTrue(runtimeControlCapable(listOf("terminal", "fs", "runtime.v1")))
        assertTrue(runtimeControlCapable(listOf("RUNTIME.V1")))
        assertFalse(runtimeControlCapable(listOf("terminal", "fs", "git")))
        assertFalse(runtimeControlCapable(emptyList()))
    }

    // ---- 启动恢复横幅 ----------------------------------------------------------

    @Test
    fun `旧daemon未报告恢复情况时不摆横幅`() {
        // 旧 daemon 的 GetHostInfo 里没这个字段 = 未报告。此时必须整块隐藏,
        // 不能拿"空报告"渲染成一条"一切正常"的假横幅。
        assertNull(recoveryBanner(null))
    }

    @Test
    fun `启动干净时横幅不降级且无丢失明细`() {
        val b = recoveryBanner(okReport())!!
        assertFalse(b.degraded)
        assertTrue(b.lostLines.isEmpty())
        assertEquals("启动正常:会话库、设备表、身份文件均已就绪", b.headline)
    }

    @Test
    fun `有排队输入丢失时逐条列出丢了哪句`() {
        // 用户裁定(README §0.3 下一刀):「有丢失明细时必须逐条显示'哪句没跑到'」。
        // 只报"丢了 2 条"等于没说 —— 用户无从决定要不要重发。
        val report = okReport().toBuilder()
            .setStatus("degraded")
            .setSummary("启动降级:上次退出有 2 条排队输入未执行(已丢失,不会自动重发)")
            .setTurnJournal(
                HostTurnJournalHealth.newBuilder()
                    .setPendingCount(2)
                    .addThreadIds("thread-a")
                    .addLostInputs(
                        HostLostQueueInput.newBuilder()
                            .setThreadId("thread-a").setPreview("帮我改一下登录接口").setCreatedAtUnixMs(1_756_000_000_000)
                    )
                    .addLostInputs(
                        HostLostQueueInput.newBuilder()
                            .setThreadId("thread-b").setPreview("顺便跑下测试").setCreatedAtUnixMs(1_756_000_600_000)
                    )
            )
            .build()
        val b = recoveryBanner(report)!!
        assertTrue(b.degraded)
        assertEquals(2, b.lostLines.size)
        assertTrue(b.lostLines[0].contains("帮我改一下登录接口"))
        assertTrue(b.lostLines[1].contains("顺便跑下测试"))
    }

    @Test
    fun `会话库被隔离留档时横幅降级`() {
        val report = okReport().toBuilder()
            .setStatus("corrupted")
            .setSummary("会话库 threads.json 解析失败")
            .setStore(HostStoreHealth.newBuilder().setOutcome("quarantined").setHasBackup(false))
            .build()
        assertTrue(recoveryBanner(report)!!.degraded)
    }

    @Test
    fun `status缺席时按降级处理而不是外推正常`() {
        // 判定权在 daemon(它才知道各数据源的真实加载路径)。它为空的唯一合理解释是
        // 我们没读懂它 —— 此时显示"一切正常"是最坏的选择,保守降级才是如实的。
        val report = okReport().toBuilder().setStatus("").build()
        assertTrue(recoveryBanner(report)!!.degraded)
    }

    // ---- 本机总览 --------------------------------------------------------------

    @Test
    fun `总览缺响应时不造值`() {
        assertNull(runtimeOverview(null))
    }

    @Test
    fun `形态不合格时总览降级并给出人话`() {
        val resp = RuntimeStateResponse.newBuilder()
            .setDaemonVersion("0.2.2")
            .setRuntimeState("degraded")
            .setBudgetPolicyRevision(4)
            .setCollabPolicyRevision(2)
            .setDataDir("C:\\Users\\x\\.pocketagentd")
            .build()
        val ov = runtimeOverview(resp)!!
        assertTrue(ov.degraded)
        assertTrue(ov.stateLabel.contains("未交代"))
        assertEquals("0.2.2", ov.version)
        assertEquals(4L, ov.budgetRevision)
    }

    @Test
    fun `形态就绪时总览不降级`() {
        val resp = RuntimeStateResponse.newBuilder().setRuntimeState("ready").build()
        val ov = runtimeOverview(resp)!!
        assertFalse(ov.degraded)
    }

    // ---- 每 agent 形态卡 -------------------------------------------------------

    @Test
    fun `形态未知的取值原样呈现不猜`() {
        // 值域由 daemon 单方面定义(proto 里刻意用 string 而非 enum)。出现我们
        // 不认识的取值时,唯一正确的做法是照抄,而不是回落到某个"看起来差不多"的中文。
        assertEquals("WEIRD-NEW", sessionModelLabel("WEIRD-NEW"))
        assertEquals("mystery", resumeModelLabel("mystery"))
    }

    @Test
    fun `形态已知取值译成人话`() {
        assertEquals("原生会话", sessionModelLabel("NATIVE"))
        assertEquals("由 daemon 托管", sessionModelLabel("DAEMON"))
        assertEquals("重挂载", resumeModelLabel("reattach"))
        assertEquals("daemon 中转", approvalModelLabel("daemon"))
        assertEquals("可委派", subagentModelLabel("delegate"))
        assertEquals("混合", memoryModelLabel("hybrid"))
        assertEquals("由 daemon 承担", cacheModelLabel("daemon"))
    }

    @Test
    fun `未申报形态的agent给出人话提示而不是空形态`() {
        // AUDIT §3.3:禁"未知/错误"。未申报是一件可以讲清楚的事实,不是错误。
        val card = agentContractCard(
            RuntimeContractReport.newBuilder()
                .setAgentId("echo").setDeclared(false)
                .addIssues(
                    RuntimeContractIssue.newBuilder()
                        .setCode("participant.not-declared").setDetail("未实现 runtime.Participant")
                )
                .build()
        )
        assertFalse(card.declared)
        assertNotNull(card.note)
        assertTrue(card.note!!.contains("未申报"))
        // 未申报时形态行一律不显示 —— 摊开六个空行比一句人话更难懂。
        assertTrue(card.rows.isEmpty())
    }

    @Test
    fun `显示名缺席时回落到agent_id`() {
        val card = agentContractCard(
            RuntimeContractReport.newBuilder().setAgentId("pi").setDeclared(true).build()
        )
        assertEquals("pi", card.title)
    }

    @Test
    fun `契约违约逐条带出code与detail`() {
        val card = agentContractCard(
            RuntimeContractReport.newBuilder()
                .setAgentId("pi").setDisplayName("Pi").setDeclared(true)
                .setSessionModel("NATIVE").setResumeModel("reattach")
                .setApprovalModel("daemon").setSubagentModel("delegate")
                .setMemoryModel("hybrid").setCacheModel("daemon")
                .addIssues(
                    RuntimeContractIssue.newBuilder()
                        .setCode("runtime.entry-silent-missing").setDetail("缺 runtime.checkpoint")
                )
                .addLimitations(
                    RuntimeLimitation.newBuilder().setCapability("memory.native").setReason("未实现原生压缩")
                )
                .build()
        )
        assertTrue(card.declared)
        assertNull(card.note)
        assertEquals(1, card.issues.size)
        assertTrue(card.issues[0].contains("runtime.entry-silent-missing"))
        assertTrue(card.issues[0].contains("缺 runtime.checkpoint"))
        assertTrue(card.limitations[0].contains("memory.native"))
        assertFalse(card.ok)
        // 申报齐备时六行形态全出。
        assertEquals(6, card.rows.size)
    }

    // ---- 预算 ------------------------------------------------------------------

    private fun fullLimits(): RuntimeLimits = RuntimeLimits.newBuilder()
        .setRamWorkingBytes(8L shl 20)
        .setRamEventBufferBytes(4L shl 20)
        .setRamSessionIndexBytes(2L shl 20)
        .setDiskConversationBytes(32L shl 20)
        .setDiskCacheBytes(16L shl 20)
        .setDiskArtifactBytes(256L shl 20)
        .setMaxParallelTurns(3)
        .build()

    @Test
    fun `预算整份缺席时不出行`() {
        assertTrue(budgetRows(null).isEmpty())
    }

    @Test
    fun `预算项未设置时不造0`() {
        // 全空的 RuntimeLimits = daemon 一项都没给。逐项读 getter 会得到 7 个 0,
        // 显示出来就是"内存 0 B / 并行 0 路" —— 而真实生效的是内置基线。
        assertTrue(budgetRows(RuntimeLimits.getDefaultInstance()).isEmpty())
    }

    @Test
    fun `显式0与未设置必须区分`() {
        // 这是 proto3 optional 存在的全部理由:未设 = 继承上一级;显式 0 = 明令封禁。
        val explicitZero = RuntimeLimits.newBuilder().setMaxParallelTurns(0).build()
        val rows = budgetRows(explicitZero)
        assertEquals(1, rows.size)
        assertEquals("0", rows[0].value)
    }

    @Test
    fun `预算全齐时七项都在且字节按MB呈现`() {
        val rows = budgetRows(fullLimits())
        assertEquals(7, rows.size)
        assertEquals("8 MB", rows.first { it.key == "ram_working_bytes" }.value)
        assertEquals("256 MB", rows.first { it.key == "disk_artifact_bytes" }.value)
        assertEquals("3", rows.first { it.key == "max_parallel_turns" }.value)
    }

    @Test
    fun `字节格式化不把零头抹成整数`() {
        assertEquals("0 B", formatBytesLabel(0))
        assertEquals("512 B", formatBytesLabel(512))
        assertEquals("1.5 KB", formatBytesLabel(1536))
        assertEquals("8 MB", formatBytesLabel(8L shl 20))
    }

    // ---- 未知事件 --------------------------------------------------------------

    @Test
    fun `安全分级未定不写成未知`() {
        // daemon 侧 safety_class 恒为 UNKNOWN(分类规则未定,如实标注)。手机端若
        // 照抄成"未知",用户会以为是自己哪里配错了 —— 事实是上游还没定义。
        assertEquals("尚未分类", safetyClassLabel("UNKNOWN"))
        assertEquals("FOO-CLASS", safetyClassLabel("FOO-CLASS"))
    }

    @Test
    fun `处理状态译成人话且未知取值原样`() {
        assertEquals("待处理", unknownStatusLabel("NEW"))
        assertEquals("待上游支持", unknownStatusLabel("NEEDS_NATIVE_SUPPORT"))
        assertEquals("WHATEVER", unknownStatusLabel("WHATEVER"))
    }

    @Test
    fun `列表项标题用原生方法名原文`() {
        val line = unknownEventLine(
            UnknownEventSummary.newBuilder()
                .setEventId("evt-1")
                .setSourceAgent("codex")
                .setSourceMethod("item/weird/thing/started")
                .setNormalizedKind("UNKNOWN_NOTIFICATION")
                .setSafetyClass("UNKNOWN")
                .setProcessingStatus("NEW")
                .setReceivedAtUnixMs(1_756_000_000_000)
                .build()
        )
        // 标题 = 原生方法名原文,不翻译、不改写(README §7 主线 A:能透传就透传)。
        assertEquals("item/weird/thing/started", line.title)
        assertTrue(line.subtitle.contains("codex"))
        assertTrue(line.subtitle.contains("新通知"))
        assertEquals("待处理", line.statusLabel)
        assertEquals("evt-1", line.eventId)
    }

    @Test
    fun `方法名缺席时用归类兜底而不是空白标题`() {
        val line = unknownEventLine(
            UnknownEventSummary.newBuilder()
                .setEventId("evt-2").setNormalizedKind("UNKNOWN_REQUEST").build()
        )
        assertEquals("新请求", line.title)
    }

    @Test
    fun `台账脚注说明丢弃了多少条`() {
        assertTrue(unknownLedgerNote(120, 8)!!.contains("8"))
        assertTrue(unknownLedgerNote(120, 8)!!.contains("120"))
        // 没有丢弃时不该出现"丢弃 0 条"这种噪音 —— 但仍然要报累计总数。
        assertFalse(unknownLedgerNote(120, 0)!!.contains("丢弃"))
        assertTrue(unknownLedgerNote(120, 0)!!.contains("120"))
        // 空台账连脚注都不该有(没有内容可说明)。
        assertNull(unknownLedgerNote(0, 0))
    }

    // ---- 协同策略 --------------------------------------------------------------

    @Test
    fun `协同策略整份缺席时不出行`() {
        assertTrue(collabPolicyRows(null).isEmpty())
    }

    @Test
    fun `协同策略项未设置时不造false`() {
        // daemon 未给 effective(旧实现)时只回 policy,缺席项在线上是 0/false。
        // 读 getter 会显示成"三家全部关闭" —— 那正是 W10 之前"改了不生效"的同款误导。
        assertTrue(collabPolicyRows(CollabPolicy.getDefaultInstance()).isEmpty())
    }

    @Test
    fun `生效协同策略全齐时四项都在`() {
        val rows = collabPolicyRows(
            CollabPolicy.newBuilder()
                .setCollaborationEnabled(true)
                .setNativeDelegateEnabled(false)
                .setFanoutEnabled(true)
                .setMaxParallelMembers(2)
                .build()
        )
        assertEquals(4, rows.size)
        assertEquals("开启", rows.first { it.key == "collaboration_enabled" }.value)
        assertEquals("关闭", rows.first { it.key == "native_delegate_enabled" }.value)
        assertEquals("2 家", rows.first { it.key == "max_parallel_members" }.value)
    }

    @Test
    fun `扇出成员上限为零表示不限而不是零家`() {
        val rows = collabPolicyRows(
            CollabPolicy.newBuilder().setMaxParallelMembers(0).build()
        )
        assertEquals("不限", rows.first { it.key == "max_parallel_members" }.value)
    }
}
