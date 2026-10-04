package dev.pocketagent.app.ui.voice

import dev.pocketagent.app.ui.component.senseVoiceErrorFatalForRound
import dev.pocketagent.app.ui.systemVoiceRetireNeeded
import dev.pocketagent.app.ui.voiceApplyToInput
import dev.pocketagent.app.ui.voiceEngineLabel
import dev.pocketagent.app.ui.voiceErrorText
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableSharedFlow
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * P4 Task 4/5 守护单测:VoiceSession 领域状态机(reducer)+ VoiceController 状态机壳。
 *
 * 覆盖面(计划验收口径,全相轨迹):
 *  1. 正常轨迹:申请权限 → 收音 → 停止 → 转写 → 出稿(八态全部可达);
 *  2. 权限拒绝:落 ERROR 相 + 权限 kind;
 *  3. 死区超时:看门狗纯函数边界 + Controller monotonic 假钟触发(含端侧冷加载分流);
 *  4. 迟到回调:旧 sessionId 的 Partial/Final/Error 一律丢弃(Review Focus 红线);
 *  5. 会话切换:置静音、清文本、停引擎、静音期回调丢弃、新 Start 解除;
 *  6. 非法迁移:如实拒绝并保持原状态(矩阵);
 *  7. 文本长度上限:超上限截断(依据见 VOICE_TEXT_MAX_CHARS 注释);
 *  8. 引擎适配纯件:端侧致命标记、管理器三失败映射、系统错误码对齐搬运。
 *
 * 约定:reducer 不分配 sessionId —— 测试里用 `copy(sessionId = n)` 模拟
 * VoiceController.send(Start) 的发号步骤(发号逻辑在 Controller,见其用例)。
 */
class VoiceSessionReducerTest {

    // ------------------------------------------------------------ 1) 正常全相轨迹

    @Test
    fun `正常全相轨迹_申请权限_收音_停止_转写_出稿_八态可达`() {
        var s = VoiceSessionState()
        assertEquals(VoicePhase.IDLE, s.phase)

        // Start:IDLE → REQUESTING_PERMISSION(新会话草稿,清空旧文本/错误/静音)
        val r1 = reduce(s.copy(finalText = "上一轮", error = VoiceErrorKind.ENGINE), VoiceEvent.Start)
        assertEquals(VoicePhase.REQUESTING_PERMISSION, r1.state.phase)
        assertEquals("", r1.state.finalText)
        assertNull(r1.state.error)
        assertFalse(r1.state.mutedByThreadSwitch)
        assertNull(r1.effect)
        s = r1.state.copy(sessionId = 1L) // Controller 在此步发号

        // Permission(true) → STARTING + StartEngine(带会话锚)
        val r2 = reduce(s, VoiceEvent.Permission(true))
        assertEquals(VoicePhase.STARTING, r2.state.phase)
        assertEquals(VoiceEffect.StartEngine(1L), r2.effect)
        s = r2.state

        // 空文本 Partial(引擎 Ready 进度信号)→ LISTENING,文本不动
        val r3 = reduce(s, VoiceEvent.Partial(1L, VOICE_PROGRESS_SIGNAL))
        assertEquals(VoicePhase.LISTENING, r3.state.phase)
        assertEquals("", r3.state.partialText)
        s = r3.state

        // 非空 Partial → 文本更新(引擎首个带字回调同时证明存活)
        val r4 = reduce(s, VoiceEvent.Partial(1L, "今天天气"))
        assertEquals(VoicePhase.LISTENING, r4.state.phase)
        assertEquals("今天天气", r4.state.partialText)
        s = r4.state

        // Pause/Resume 子轨迹:暂停保持会话,恢复继续(见下一个用例),此处直接 Stop
        val r5 = reduce(s, VoiceEvent.Stop)
        assertEquals(VoicePhase.STOPPING, r5.state.phase)
        assertEquals(VoiceEffect.StopEngine, r5.effect)
        s = r5.state

        // 空文本 Partial(转写开始进度信号)→ TRANSCRIBING
        val r6 = reduce(s, VoiceEvent.Partial(1L, VOICE_PROGRESS_SIGNAL))
        assertEquals(VoicePhase.TRANSCRIBING, r6.state.phase)
        s = r6.state

        // Final → IDLE,finalText 只存不追加,partialText 清空(追加语义在 Controller/面板层)
        val r7 = reduce(s, VoiceEvent.Final(1L, "今天天气不错"))
        assertEquals(VoicePhase.IDLE, r7.state.phase)
        assertEquals("今天天气不错", r7.state.finalText)
        assertEquals("", r7.state.partialText)
        assertNull(r7.effect)
    }

    @Test
    fun `暂停恢复轨迹_暂停期flush文本_恢复同会话继续_停止出稿`() {
        var s = listening(sessionId = 1L, partial = "一半")
        val r1 = reduce(s, VoiceEvent.Pause)
        assertEquals(VoicePhase.PAUSED, r1.state.phase)
        assertEquals(VoiceEffect.PauseEngine, r1.effect)
        s = r1.state

        // 暂停期引擎 flush 已收文本:相保持 PAUSED,只更新文本
        val r2 = reduce(s, VoiceEvent.Partial(1L, "一半 补充"))
        assertEquals(VoicePhase.PAUSED, r2.state.phase)
        assertEquals("一半 补充", r2.state.partialText)
        s = r2.state

        // PAUSED 里 Start = 恢复:同一会话(不重新发号),引擎 resume
        val r3 = reduce(s, VoiceEvent.Start)
        assertEquals(VoicePhase.LISTENING, r3.state.phase)
        assertEquals(VoiceEffect.ResumeEngine, r3.effect)
        assertEquals(1L, r3.state.sessionId)
        assertEquals("一半 补充", r3.state.partialText) // 累计不清
        s = r3.state

        val r4 = reduce(s, VoiceEvent.Stop)
        assertEquals(VoicePhase.STOPPING, r4.state.phase)
        val r5 = reduce(r4.state, VoiceEvent.Final(1L, "一半 补充 补完"))
        assertEquals(VoicePhase.IDLE, r5.state.phase)
        assertEquals("一半 补充 补完", r5.state.finalText)
    }

    // ------------------------------------------------------------ 2) 权限拒绝

    @Test
    fun `权限拒绝_落ERROR相_权限kind_拒绝态可再次Start重申`() {
        var s = reduce(VoiceSessionState(), VoiceEvent.Start).state.copy(sessionId = 1L)
        val r = reduce(s, VoiceEvent.Permission(false))
        assertEquals(VoicePhase.ERROR, r.state.phase)
        assertEquals(VoiceErrorKind.INSUFFICIENT_PERMISSIONS, r.state.error)
        assertNull(r.effect) // 无引擎可停(还没启动)
        // ERROR 相可 Start 重来(再弹权限)
        val r2 = reduce(r.state, VoiceEvent.Start)
        assertEquals(VoicePhase.REQUESTING_PERMISSION, r2.state.phase)
        assertNull(r2.state.error)
    }

    // ------------------------------------------------------------ 3) 死区超时

    @Test
    fun `死区判定纯函数_阈值边界_相与会话守卫`() {
        // 恰好到阈值触发,未到不触发,有回调不触发
        assertTrue(voiceDeadZoneShouldFire(8_000, false, VoicePhase.STARTING, 1L, 1L))
        assertFalse(voiceDeadZoneShouldFire(7_999, false, VoicePhase.LISTENING, 1L, 1L))
        assertFalse(voiceDeadZoneShouldFire(9_999, true, VoicePhase.LISTENING, 1L, 1L))
        // 相守卫:STOPPING 在等转写交付、PAUSED 是用户主动暂停,都不算死区
        assertFalse(voiceDeadZoneShouldFire(9_999, false, VoicePhase.STOPPING, 1L, 1L))
        assertFalse(voiceDeadZoneShouldFire(9_999, false, VoicePhase.PAUSED, 1L, 1L))
        // 会话守卫:用户已开新一轮,迟到的超时不得误伤
        assertFalse(voiceDeadZoneShouldFire(9_999, false, VoicePhase.LISTENING, 2L, 1L))
        // 阈值可注入(单测/未来可调)
        assertTrue(voiceDeadZoneShouldFire(500, false, VoicePhase.STARTING, 1L, 1L, timeoutMs = 500))
    }

    @Test
    fun `死区超时事件_落ERROR相_分类DEAD_ZONE_ERROR相内新分类覆盖`() {
        var s = listening(sessionId = 1L)
        val r = reduce(s, VoiceEvent.Error(1L, VoiceErrorKind.DEAD_ZONE))
        assertEquals(VoicePhase.ERROR, r.state.phase)
        assertEquals(VoiceErrorKind.DEAD_ZONE, r.state.error)
        // ERROR 相内再次报错:最新分类覆盖(不叠加相迁移)
        val r2 = reduce(r.state, VoiceEvent.Error(1L, VoiceErrorKind.MODEL_LOADING))
        assertEquals(VoicePhase.ERROR, r2.state.phase)
        assertEquals(VoiceErrorKind.MODEL_LOADING, r2.state.error)
    }

    // ------------------------------------------------------------ 4) 迟到回调丢弃

    @Test
    fun `迟到回调_旧sessionId的Partial_Final_Error一律丢弃_不污染新会话`() {
        var s = reduce(VoiceSessionState(), VoiceEvent.Start).state.copy(sessionId = 1L)
        s = reduce(s, VoiceEvent.Permission(true)).state
        s = reduce(s, VoiceEvent.Partial(1L, "旧会话首句")).state
        s = reduce(s, VoiceEvent.Final(1L, "旧会话定稿")).state
        assertEquals(VoicePhase.IDLE, s.phase)

        // 新一轮会话:Controller 发号 sid=2
        s = reduce(s, VoiceEvent.Start).state.copy(sessionId = 2L)
        val baseline = s

        // 旧 sessionId 的三类回调全部原样退回(状态不变、无效果)
        val latePartial = reduce(s, VoiceEvent.Partial(1L, "旧会话迟到文本"))
        assertSame(baseline, latePartial.state)
        assertNull(latePartial.effect)
        val lateFinal = reduce(s, VoiceEvent.Final(1L, "旧会话迟到定稿"))
        assertSame(baseline, lateFinal.state)
        assertNull(lateFinal.effect)
        val lateError = reduce(s, VoiceEvent.Error(1L, VoiceErrorKind.DEAD_ZONE))
        assertSame(baseline, lateError.state)
        assertNull(lateError.effect)
        assertEquals("", latePartial.state.partialText)
        assertEquals("", lateFinal.state.finalText)
    }

    // ------------------------------------------------------------ 5) 会话切换静音

    @Test
    fun `会话切换_置静音清文本停引擎_静音期回调一律丢弃_新Start解除`() {
        var s = listening(sessionId = 1L, partial = "切走前说的")
        val r = reduce(s, VoiceEvent.SessionSwitched("thread-2"))
        // 停录 + 静音 + 残余文本丢弃,不落新会话草稿
        assertEquals(VoicePhase.IDLE, r.state.phase)
        assertTrue(r.state.mutedByThreadSwitch)
        assertEquals("thread-2", r.state.threadId)
        assertEquals("", r.state.partialText)
        assertEquals("", r.state.finalText)
        assertEquals(VoiceEffect.StopEngine, r.effect)
        s = r.state

        // 静音期:旧识别器残余的文本/错误一律丢弃(即使 sessionId 恰好匹配当前号)
        val base = s
        val droppedPartial = reduce(s, VoiceEvent.Partial(1L, "残余文本"))
        assertSame(base, droppedPartial.state)
        val droppedFinal = reduce(s, VoiceEvent.Final(1L, "残余定稿"))
        assertSame(base, droppedFinal.state)
        val droppedError = reduce(s, VoiceEvent.Error(1L, VoiceErrorKind.BUSY))
        assertSame(base, droppedError.state)

        // 用户在新会话主动点击 = 新一轮 Start:解除静音、清空草稿
        val r2 = reduce(s, VoiceEvent.Start)
        assertFalse(r2.state.mutedByThreadSwitch)
        assertEquals(VoicePhase.REQUESTING_PERMISSION, r2.state.phase)
        assertEquals("", r2.state.finalText)
    }

    @Test
    fun `会话切换_空闲或权限申请中同样置静音_权限申请中补Cancel_空闲不产效果`() {
        // 空闲切换:静音照常置位("点了停止又立刻切会话"场景的兜底),无引擎效果
        val idle = VoiceSessionState(threadId = "thread-1")
        val r = reduce(idle, VoiceEvent.SessionSwitched("thread-2"))
        assertTrue(r.state.mutedByThreadSwitch)
        assertEquals(VoicePhase.IDLE, r.state.phase)
        assertEquals("thread-2", r.state.threadId)
        assertNull(r.effect)

        // 还在等权限就切走:取消请求(没有引擎可停)
        var s = reduce(VoiceSessionState(), VoiceEvent.Start).state.copy(sessionId = 1L)
        val r2 = reduce(s, VoiceEvent.SessionSwitched("thread-3"))
        assertEquals(VoicePhase.IDLE, r2.state.phase)
        assertTrue(r2.state.mutedByThreadSwitch)
        assertEquals(VoiceEffect.CancelEngine, r2.effect)

        // 静音后再次切换:只更新 threadId,静音保持
        val r3 = reduce(r2.state, VoiceEvent.SessionSwitched("thread-4"))
        assertTrue(r3.state.mutedByThreadSwitch)
        assertEquals("thread-4", r3.state.threadId)
    }

    // ------------------------------------------------------------ 6) 非法迁移拒绝

    @Test
    fun `非法迁移矩阵_如实拒绝并保持原状态_不产效果`() {
        data class Case(val state: VoiceSessionState, val event: VoiceEvent, val label: String)

        val cases = listOf(
            // IDLE:没有活动会话,一切会话回调与启动后事件都非法
            Case(VoiceSessionState(sessionId = 1L), VoiceEvent.Stop, "IDLE+Stop"),
            Case(VoiceSessionState(sessionId = 1L), VoiceEvent.Pause, "IDLE+Pause"),
            Case(VoiceSessionState(sessionId = 1L), VoiceEvent.Cancel, "IDLE+Cancel"),
            Case(VoiceSessionState(sessionId = 1L), VoiceEvent.Partial(1L, "字"), "IDLE+Partial"),
            Case(VoiceSessionState(sessionId = 1L), VoiceEvent.Partial(1L, ""), "IDLE+进度"),
            Case(VoiceSessionState(sessionId = 1L), VoiceEvent.Final(1L, "字"), "IDLE+Final"),
            Case(VoiceSessionState(sessionId = 1L), VoiceEvent.Error(1L, VoiceErrorKind.OTHER), "IDLE+Error"),
            Case(VoiceSessionState(sessionId = 1L), VoiceEvent.Permission(true), "IDLE+Permission(true)"),
            Case(VoiceSessionState(sessionId = 1L), VoiceEvent.Permission(false), "IDLE+Permission(false)"),
            // 已在进行中的相:重复 Start/权限回答/Pause 非法
            Case(listening(sessionId = 1L), VoiceEvent.Start, "LISTENING+Start"),
            Case(listening(sessionId = 1L), VoiceEvent.Permission(true), "LISTENING+Permission(true)"),
            Case(listening(sessionId = 1L), VoiceEvent.Permission(false), "LISTENING+Permission(false)"),
            Case(listening(sessionId = 1L).copy(phase = VoicePhase.STARTING), VoiceEvent.Pause, "STARTING+Pause"),
            Case(listening(sessionId = 1L).copy(phase = VoicePhase.STARTING), VoiceEvent.Start, "STARTING+Start"),
            Case(listening(sessionId = 1L).copy(phase = VoicePhase.PAUSED), VoiceEvent.Pause, "PAUSED+Pause"),
            Case(listening(sessionId = 1L).copy(phase = VoicePhase.STOPPING), VoiceEvent.Partial(1L, "字"), "STOPPING+Partial非空"),
            Case(listening(sessionId = 1L).copy(phase = VoicePhase.STOPPING), VoiceEvent.Stop, "STOPPING+Stop"),
            Case(listening(sessionId = 1L).copy(phase = VoicePhase.TRANSCRIBING), VoiceEvent.Stop, "TRANSCRIBING+Stop"),
            Case(listening(sessionId = 1L).copy(phase = VoicePhase.TRANSCRIBING), VoiceEvent.Pause, "TRANSCRIBING+Pause"),
            // ERROR:旧会话已终,文本回调与收尾指令非法(Start/Cancel 合法,另测)
            Case(
                VoiceSessionState(sessionId = 1L, phase = VoicePhase.ERROR, error = VoiceErrorKind.ENGINE),
                VoiceEvent.Partial(1L, "字"),
                "ERROR+Partial",
            ),
            Case(
                VoiceSessionState(sessionId = 1L, phase = VoicePhase.ERROR, error = VoiceErrorKind.ENGINE),
                VoiceEvent.Final(1L, "字"),
                "ERROR+Final",
            ),
            Case(
                VoiceSessionState(sessionId = 1L, phase = VoicePhase.ERROR, error = VoiceErrorKind.ENGINE),
                VoiceEvent.Stop,
                "ERROR+Stop",
            ),
            // 权限申请中:只等权限回答
            Case(
                VoiceSessionState(sessionId = 1L, phase = VoicePhase.REQUESTING_PERMISSION),
                VoiceEvent.Pause,
                "REQ_PERM+Pause",
            ),
            Case(
                VoiceSessionState(sessionId = 1L, phase = VoicePhase.REQUESTING_PERMISSION),
                VoiceEvent.Partial(1L, "字"),
                "REQ_PERM+Partial",
            ),
        )
        for (case in cases) {
            val r = reduce(case.state, case.event)
            assertSame("${case.label}:应保持原状态", case.state, r.state)
            assertNull("${case.label}:拒绝时不应产效果", r.effect)
        }
    }

    @Test
    fun `合法收尾_Cancel从任意活动相与ERROR回IDLE并清草稿`() {
        val active = listOf(
            VoicePhase.REQUESTING_PERMISSION,
            VoicePhase.STARTING,
            VoicePhase.LISTENING,
            VoicePhase.PAUSED,
            VoicePhase.STOPPING,
            VoicePhase.TRANSCRIBING,
            VoicePhase.ERROR,
        )
        for (phase in active) {
            val s = VoiceSessionState(
                sessionId = 1L,
                phase = phase,
                partialText = "草稿",
                finalText = "旧稿",
                error = VoiceErrorKind.ENGINE,
            )
            val r = reduce(s, VoiceEvent.Cancel)
            assertEquals("$phase:取消后回 IDLE", VoicePhase.IDLE, r.state.phase)
            assertEquals(VoiceEffect.CancelEngine, r.effect)
            assertEquals("", r.state.partialText)
            assertEquals("", r.state.finalText)
            assertNull(r.state.error)
        }
    }

    @Test
    fun `STARTING里Final兜底收稿_引擎未发Ready即出稿也能落地`() {
        val s = VoiceSessionState(sessionId = 1L, phase = VoicePhase.STARTING)
        val r = reduce(s, VoiceEvent.Final(1L, "直接定稿"))
        assertEquals(VoicePhase.IDLE, r.state.phase)
        assertEquals("直接定稿", r.state.finalText)
    }

    // ------------------------------------------------------------ 7) 文本长度上限

    @Test
    fun `识别文本超上限截断_保留头部_进度信号不清空已有文本`() {
        // 上限依据:60 秒采音上限 × 密集文本 ~15 字符/秒 ≈ 900;4_096 留 ≥4 倍余量
        assertEquals(4_096, VOICE_TEXT_MAX_CHARS)
        val over = "啊".repeat(VOICE_TEXT_MAX_CHARS + 100)

        var s = listening(sessionId = 1L)
        val r = reduce(s, VoiceEvent.Partial(1L, over))
        assertEquals(VOICE_TEXT_MAX_CHARS, r.state.partialText.length)
        assertEquals("啊".repeat(VOICE_TEXT_MAX_CHARS), r.state.partialText) // 保留头部
        s = r.state

        // 空文本进度信号在收音中是中性事件:不清空已有部分文本
        val neutral = reduce(s, VoiceEvent.Partial(1L, VOICE_PROGRESS_SIGNAL))
        assertEquals(r.state.partialText, neutral.state.partialText)

        s = reduce(s, VoiceEvent.Stop).state
        val final = reduce(s, VoiceEvent.Final(1L, over))
        assertEquals(VOICE_TEXT_MAX_CHARS, final.state.finalText.length)
        assertEquals("", final.state.partialText)
    }

    // ------------------------------------------------------------ 8) 引擎适配纯件

    @Test
    fun `端侧致命标记_7没听到与103引擎忙不致命_其余致命`() {
        assertFalse(senseVoiceErrorFatalForRound(7))
        assertFalse(senseVoiceErrorFatalForRound(103))
        // 9 权限 / 102 麦克风被拦 / 100 引擎异常:本轮致命(roundFailed 随下次 start 重置)
        assertTrue(senseVoiceErrorFatalForRound(9))
        assertTrue(senseVoiceErrorFatalForRound(102))
        assertTrue(senseVoiceErrorFatalForRound(100))
    }

    @Test
    fun `管理器失败三分类_校验失败_磁盘_其他兜底`() {
        assertEquals(
            VoiceErrorKind.MODEL_CORRUPTED,
            classifySenseVoiceManagerError("内置语音模型校验失败，请重新安装 App"),
        )
        assertEquals(
            VoiceErrorKind.MODEL_CORRUPTED,
            classifySenseVoiceManagerError("模型文件损坏"),
        )
        assertEquals(VoiceErrorKind.STORAGE_FULL, classifySenseVoiceManagerError("No space left on device"))
        assertEquals(VoiceErrorKind.STORAGE_FULL, classifySenseVoiceManagerError("磁盘空间不足"))
        assertEquals(VoiceErrorKind.ENGINE, classifySenseVoiceManagerError("无法创建端侧语音目录"))
        assertEquals(VoiceErrorKind.ENGINE, classifySenseVoiceManagerError(null))
    }

    @Test
    fun `系统错误码对齐搬运_弃用名单复用旧映射不漂移`() {
        assertEquals(VoiceErrorKind.NO_MATCH, systemVoiceErrorKind(7))
        assertEquals(VoiceErrorKind.BUSY, systemVoiceErrorKind(8))
        assertEquals(VoiceErrorKind.INSUFFICIENT_PERMISSIONS, systemVoiceErrorKind(9))
        assertEquals(VoiceErrorKind.MIC_BLOCKED, systemVoiceErrorKind(102))
        assertEquals(VoiceErrorKind.OTHER, systemVoiceErrorKind(3))
        assertEquals(VoiceErrorKind.CLIENT, systemVoiceErrorKind(5))
        assertEquals(VoiceErrorKind.ENGINE, systemVoiceErrorKind(100))
        // 弃用名单判定仍以 ui.systemVoiceRetireNeeded 为单一真值源(引擎本轮失败标记用它)
        assertTrue(systemVoiceRetireNeeded(3))
        assertFalse(systemVoiceRetireNeeded(7))
        assertFalse(systemVoiceRetireNeeded(102)) // 端侧码到达时系统引擎未被选中,不得误伤
    }

    // ------------------------------------------------------------ 9) Controller 状态机壳

    /** 手动看门狗调度器:测试注入,配合假钟确定性驱动"到点开火",不依赖虚拟时间。 */
    private class ManualWatchdog {
        private class Entry(val fire: () -> Unit)
        private val entries = mutableListOf<Entry>()
        fun schedule(delayMs: Long, fire: () -> Unit): () -> Unit {
            val entry = Entry(fire)
            entries += entry
            return { entries.remove(entry) }
        }

        fun fireAll() {
            val snapshot = entries.toList()
            entries.clear()
            snapshot.forEach { it.fire() }
        }

        fun size(): Int = entries.size
    }

    private class MutableClock(var now: Long = 0L)

    private class FakeEngine(override val kind: VoiceEngineKind = VoiceEngineKind.SYSTEM_ON_DEVICE) : VoiceEngine {
        override val events = MutableSharedFlow<VoiceEngineEvent>(extraBufferCapacity = 64)
        val started = mutableListOf<Long>()
        var pauseCalls = 0
        var resumeCalls = 0
        var stopCalls = 0
        var cancelCalls = 0
        var destroyCalls = 0

        override fun start(sessionId: Long) {
            started += sessionId
        }

        override fun pause() {
            pauseCalls++
        }

        override fun resume() {
            resumeCalls++
        }

        override fun stop() {
            stopCalls++
        }

        override fun cancel() {
            cancelCalls++
        }

        override fun destroy() {
            destroyCalls++
        }

        fun emit(event: VoiceEngineEvent) {
            check(events.tryEmit(event)) { "引擎事件发射失败" }
        }
    }

    /** Unconfined:引擎事件收集器在 bindEngine 时同步订阅,emit 即同步送达 send。 */
    private fun newController(
        engine: FakeEngine? = FakeEngine(),
        clock: MutableClock = MutableClock(),
        watchdog: ManualWatchdog = ManualWatchdog(),
    ): Pair<VoiceController, CoroutineScope> {
        val scope = CoroutineScope(Dispatchers.Unconfined + Job())
        val controller = VoiceController(
            scope = scope,
            clockMillis = { clock.now },
            scheduleWatchdog = { delayMs, onFire -> watchdog.schedule(delayMs, onFire) },
        )
        controller.bindEngine(engine)
        return controller to scope
    }

    private fun startToListening(controller: VoiceController, engine: FakeEngine, sid: Long) {
        controller.send(VoiceEvent.Start)
        controller.send(VoiceEvent.Permission(true))
        assertEquals(sid, controller.current().sessionId)
        engine.emit(VoiceEngineEvent.Ready(sid))
        assertEquals(VoicePhase.LISTENING, controller.current().phase)
    }

    @Test
    fun `Controller_Start发号递增_权限通过才启动引擎_引擎事件按sessionId认亲`() {
        val engine = FakeEngine()
        val (controller, scope) = newController(engine)
        try {
            controller.send(VoiceEvent.Start)
            assertEquals(VoicePhase.REQUESTING_PERMISSION, controller.current().phase)
            assertEquals(1L, controller.current().sessionId)
            assertEquals(0, engine.started.size) // 权限未过,引擎不得启动

            controller.send(VoiceEvent.Permission(true))
            assertEquals(VoicePhase.STARTING, controller.current().phase)
            assertEquals(listOf(1L), engine.started)

            engine.emit(VoiceEngineEvent.Ready(1L))
            assertEquals(VoicePhase.LISTENING, controller.current().phase)

            engine.emit(VoiceEngineEvent.Partial(1L, "你好"))
            assertEquals("你好", controller.current().partialText)

            engine.emit(VoiceEngineEvent.Final(1L, "你好世界"))
            assertEquals(VoicePhase.IDLE, controller.current().phase)
            assertEquals("你好世界", controller.current().finalText)
            assertEquals("", controller.current().partialText)

            // 旧会话(1)的迟到 Partial:控制器/reducer 双门丢弃
            controller.send(VoiceEvent.Cancel)
            controller.send(VoiceEvent.Start)
            assertEquals(2L, controller.current().sessionId)
            engine.emit(VoiceEngineEvent.Partial(1L, "旧会话文本"))
            assertEquals("", controller.current().partialText)
        } finally {
            scope.cancel()
        }
    }

    @Test
    fun `Controller_死区看门狗_零回调到点触发_有回调不触发_会话换号后迟到超时不误伤`() {
        val engine = FakeEngine()
        val clock = MutableClock()
        val watchdog = ManualWatchdog()
        val (controller, scope) = newController(engine, clock, watchdog)
        try {
            // 第一轮:零回调撞满 8 秒(monotonic 假钟)→ DEAD_ZONE
            controller.send(VoiceEvent.Start)
            controller.send(VoiceEvent.Permission(true))
            assertEquals(1, watchdog.size()) // start 派发时武装
            clock.now = 8_000
            watchdog.fireAll()
            assertEquals(VoicePhase.ERROR, controller.current().phase)
            assertEquals(VoiceErrorKind.DEAD_ZONE, controller.current().error)

            // 第二轮:首个回调到达 = 服务活着,到点不开火
            controller.send(VoiceEvent.Start)
            assertEquals(2L, controller.current().sessionId)
            controller.send(VoiceEvent.Permission(true))
            clock.now = 16_000
            engine.emit(VoiceEngineEvent.Ready(2L)) // 先喂狗再放行超时
            assertEquals(VoicePhase.LISTENING, controller.current().phase)
            watchdog.fireAll()
            assertEquals(VoicePhase.LISTENING, controller.current().phase)
        } finally {
            scope.cancel()
        }
    }

    @Test
    fun `Controller_端侧死区分流_从未出稿按冷加载_出过稿按服务无响应`() {
        // 端侧引擎 + 本绑定从未交付 Final:死区按"模型加载中"(MODEL_LOADING)
        val coldEngine = FakeEngine(VoiceEngineKind.SENSEVOICE)
        val clock = MutableClock()
        val watchdog = ManualWatchdog()
        val (coldController, scope1) = newController(coldEngine, clock, watchdog)
        try {
            coldController.send(VoiceEvent.Start)
            coldController.send(VoiceEvent.Permission(true))
            clock.now = 8_000
            watchdog.fireAll()
            assertEquals(VoiceErrorKind.MODEL_LOADING, coldController.current().error)
        } finally {
            scope1.cancel()
        }

        // 端侧引擎 + 已交付过 Final(模型已暖):死区如实按"服务无响应"(DEAD_ZONE)
        val warmEngine = FakeEngine(VoiceEngineKind.SENSEVOICE)
        val clock2 = MutableClock()
        val watchdog2 = ManualWatchdog()
        val (warmController, scope2) = newController(warmEngine, clock2, watchdog2)
        try {
            startToListening(warmController, warmEngine, 1L)
            warmEngine.emit(VoiceEngineEvent.Final(1L, "已出过稿"))
            assertEquals(VoicePhase.IDLE, warmController.current().phase)
            // 新一轮死区
            warmController.send(VoiceEvent.Start)
            warmController.send(VoiceEvent.Permission(true))
            clock2.now = 8_000
            watchdog2.fireAll()
            assertEquals(VoicePhase.ERROR, warmController.current().phase)
            assertEquals(VoiceErrorKind.DEAD_ZONE, warmController.current().error)
        } finally {
            scope2.cancel()
        }
    }

    @Test
    fun `Controller_零回调就点停止_立即按死区收口_不卡STOPPING`() {
        val engine = FakeEngine()
        val (controller, scope) = newController(engine)
        try {
            controller.send(VoiceEvent.Start)
            controller.send(VoiceEvent.Permission(true))
            assertEquals(VoicePhase.STARTING, controller.current().phase)
            controller.send(VoiceEvent.Stop)
            assertEquals(1, engine.stopCalls)
            // 引擎自 start 起零回调:stop 等不来 Final,立即报死区回 ERROR
            assertEquals(VoicePhase.ERROR, controller.current().phase)
            assertEquals(VoiceErrorKind.DEAD_ZONE, controller.current().error)
        } finally {
            scope.cancel()
        }
    }

    @Test
    fun `Controller_暂停恢复与切会话静音_引擎指令与丢弃语义照单执行`() {
        val engine = FakeEngine()
        val (controller, scope) = newController(engine)
        try {
            startToListening(controller, engine, 1L)
            controller.send(VoiceEvent.Pause)
            assertEquals(VoicePhase.PAUSED, controller.current().phase)
            assertEquals(1, engine.pauseCalls)
            controller.send(VoiceEvent.Start) // PAUSED 里 Start = 恢复,同会话不发新号
            assertEquals(VoicePhase.LISTENING, controller.current().phase)
            assertEquals(1L, controller.current().sessionId)
            assertEquals(1, engine.resumeCalls)

            // 切会话:静音 + 停引擎;引擎迟到的 Final 被静音门丢弃
            controller.send(VoiceEvent.SessionSwitched("thread-2"))
            assertEquals(VoicePhase.IDLE, controller.current().phase)
            assertTrue(controller.current().mutedByThreadSwitch)
            assertEquals(1, engine.stopCalls)
            engine.emit(VoiceEngineEvent.Final(1L, "切走后的残余"))
            assertEquals("", controller.current().finalText)

            // 新会话 Start 解除静音;引擎换绑后状态 engine 类别随绑更新
            controller.send(VoiceEvent.Start)
            assertFalse(controller.current().mutedByThreadSwitch)
            assertEquals(2L, controller.current().sessionId)
        } finally {
            scope.cancel()
        }
    }

    @Test
    fun `Controller_onPageHidden_收音中发Stop_权限申请中发Cancel_空闲不动`() {
        val engine = FakeEngine()
        val (controller, scope) = newController(engine)
        try {
            // 空闲:页面隐藏不动
            controller.onPageHidden()
            assertEquals(VoicePhase.IDLE, controller.current().phase)

            startToListening(controller, engine, 1L)
            controller.onPageHidden() // Stop 语义
            assertEquals(VoicePhase.STOPPING, controller.current().phase)
            assertEquals(1, engine.stopCalls)

            engine.emit(VoiceEngineEvent.Final(1L, "收尾稿"))
            assertEquals(VoicePhase.IDLE, controller.current().phase)
            controller.onPageHidden() // 已空闲,不动
            assertEquals(VoicePhase.IDLE, controller.current().phase)

            // 还在等权限就离开页面:取消请求
            controller.send(VoiceEvent.Start)
            assertEquals(VoicePhase.REQUESTING_PERMISSION, controller.current().phase)
            controller.onPageHidden()
            assertEquals(VoicePhase.IDLE, controller.current().phase)
            assertEquals(1, engine.cancelCalls)
        } finally {
            scope.cancel()
        }
    }

    @Test
    fun `Controller_重绑引擎销毁旧引擎_状态engine随绑更新_release全收尾`() {
        val e1 = FakeEngine(VoiceEngineKind.SYSTEM_ON_DEVICE)
        val (controller, scope) = newController(e1)
        try {
            assertEquals(VoiceEngineKind.SYSTEM_ON_DEVICE, controller.current().engine)
            val e2 = FakeEngine(VoiceEngineKind.SENSEVOICE)
            controller.bindEngine(e2)
            assertEquals(1, e1.destroyCalls) // 旧引擎立即销毁防泄漏
            assertEquals(VoiceEngineKind.SENSEVOICE, controller.current().engine)
            // e2 的回调照常驱动状态机
            controller.send(VoiceEvent.Start)
            controller.send(VoiceEvent.Permission(true))
            e2.emit(VoiceEngineEvent.Ready(1L))
            assertEquals(VoicePhase.LISTENING, controller.current().phase)

            controller.release()
            assertEquals(1, e2.destroyCalls)
            assertEquals(VoicePhase.IDLE, controller.current().phase)
            assertEquals(VoiceEngineKind.UNAVAILABLE, controller.current().engine)
        } finally {
            scope.cancel()
        }
    }

    @Test
    fun `Controller_引擎未绑定时的启动指令如实报错_不无声卡STARTING`() {
        val (controller, scope) = newController(engine = null)
        try {
            controller.send(VoiceEvent.Start)
            controller.send(VoiceEvent.Permission(true))
            assertEquals(VoicePhase.ERROR, controller.current().phase)
            assertEquals(VoiceErrorKind.ENGINE, controller.current().error)
        } finally {
            scope.cancel()
        }
    }

    // ------------------------------------------------------------ 10) 接线纯件(P4 Task 4/5/6 波 3 补钉)

    @Test
    fun `接线文案映射_共享七类沿用旧文案_端侧四类新增可行动`() {
        // 共享七类:与旧路径文案逐字一致(权限/MIUI 指引零漂移,委托旧真值源)
        assertEquals("没听到,再试一次", voiceErrorText(VoiceErrorKind.NO_MATCH))
        assertEquals("语音服务正忙,稍后再试", voiceErrorText(VoiceErrorKind.BUSY))
        assertEquals(
            "系统语音服务权限受限,已自动切换端侧识别;若仍失败,请在系统设置里开启麦克风权限",
            voiceErrorText(VoiceErrorKind.INSUFFICIENT_PERMISSIONS),
        )
        assertEquals("语音识别不可用", voiceErrorText(VoiceErrorKind.CLIENT))
        assertEquals("识别失败,请重试", voiceErrorText(VoiceErrorKind.OTHER))
        assertEquals("端侧语音引擎异常,请重试", voiceErrorText(VoiceErrorKind.ENGINE))
        val mic = voiceErrorText(VoiceErrorKind.MIC_BLOCKED)
        assertTrue("MIC_BLOCKED 须点名系统设置与安全中心", mic.contains("系统设置") && mic.contains("安全中心"))
        // 新增四类:死区/冷加载/模型损坏/磁盘满 各有真相文案
        assertEquals("语音服务无响应,请重试或检查语音服务", voiceErrorText(VoiceErrorKind.DEAD_ZONE))
        assertEquals("正在加载语音模型,请稍候再试", voiceErrorText(VoiceErrorKind.MODEL_LOADING))
        assertTrue(voiceErrorText(VoiceErrorKind.MODEL_CORRUPTED).contains("重新安装"))
        assertTrue(voiceErrorText(VoiceErrorKind.STORAGE_FULL).contains("存储"))
    }

    @Test
    fun `接线落框_追加为默认不覆盖已有内容_替换整体覆盖_不改写大小写标点`() {
        // 追加(默认):空格衔接,不覆盖
        assertEquals("草稿 说的", voiceApplyToInput("草稿", "说的", replace = false))
        assertEquals("说的", voiceApplyToInput("", "说的", replace = false))
        // 替换:覆盖输入框
        assertEquals("新稿", voiceApplyToInput("草稿", "新稿", replace = true))
        assertEquals("", voiceApplyToInput("草稿", "", replace = true))
        // 英文/路径/命令逐字保留原始大小写与标点(计划全局约束),两种模式都不改写
        assertEquals("git status --short", voiceApplyToInput("", "git status --short", replace = false))
        assertEquals("Fix NPE at MainActivity.kt:42!", voiceApplyToInput("a", "Fix NPE at MainActivity.kt:42!", replace = true))
    }

    @Test
    fun `接线引擎读屏名_四类各有短名`() {
        assertEquals("系统识别(本机)", voiceEngineLabel(VoiceEngineKind.SYSTEM_ON_DEVICE))
        assertEquals("系统识别(在线)", voiceEngineLabel(VoiceEngineKind.SYSTEM_REMOTE))
        assertEquals("端侧识别", voiceEngineLabel(VoiceEngineKind.SENSEVOICE))
        assertEquals("语音引擎未就绪", voiceEngineLabel(VoiceEngineKind.UNAVAILABLE))
    }

    // ------------------------------------------------------------ 工具

    /** 直达 LISTENING 的状态(等价 Start→Permission(true)→Ready 进度信号的迁移结果)。 */
    private fun listening(sessionId: Long, partial: String = ""): VoiceSessionState = VoiceSessionState(
        sessionId = sessionId,
        phase = VoicePhase.LISTENING,
        partialText = partial,
    )
}
