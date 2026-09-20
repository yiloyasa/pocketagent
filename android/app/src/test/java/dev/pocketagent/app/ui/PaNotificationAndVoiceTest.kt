package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.PaNotificationPolicy
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * P1-6 本地通知 + P1-9 语音输入 的纯逻辑单测:
 *  1. 通知触发矩阵(running→false 且前台/后台;审批出现/消失);
 *  2. 前台标志读写;extra 深链解析;通知 id 稳定与区间隔离;
 *  3. 语音按钮状态机(空闲/听写/无权限/不可用),SpeechRecognizer 以 [PaVoiceRecognizer]
 *     接口包装,这里用 fake 顶替验证"指令 → 识别器动作"的消费链。
 */
class PaNotificationAndVoiceTest {

    @After
    fun resetForeground() {
        PaNotificationPolicy.appInForeground = false
    }

    // ------------------------------------------------------------ 通知触发矩阵

    private fun watch(running: Boolean, approval: Boolean = false, recovering: Boolean = false) =
        PaNotificationPolicy.ThreadWatch(running, approval, recovering)

    @Test
    fun `完成通知_running降沿且app在后台才发`() {
        PaNotificationPolicy.appInForeground = false
        val out = PaNotificationPolicy.decide(
            prev = watch(running = true),
            now = watch(running = false),
            threadId = "t1",
        )
        assertEquals(listOf<PaNotificationPolicy.Decision>(PaNotificationPolicy.Decision.TurnDone("t1")), out)
    }

    @Test
    fun `完成通知_app在前台时不发`() {
        PaNotificationPolicy.appInForeground = true
        val out = PaNotificationPolicy.decide(
            prev = watch(running = true),
            now = watch(running = false),
            threadId = "t1",
        )
        assertTrue(out.isEmpty())
    }

    @Test
    fun `完成通知_断流恢复中不发_结果未知`() {
        PaNotificationPolicy.appInForeground = false
        val out = PaNotificationPolicy.decide(
            prev = watch(running = true),
            now = watch(running = false, recovering = true),
            threadId = "t1",
        )
        assertTrue(out.isEmpty())
    }

    @Test
    fun `完成通知_没有降沿不发_首见与未在跑都不算完成`() {
        PaNotificationPolicy.appInForeground = false
        // 首见(监控启动晚于轮次)
        assertTrue(
            PaNotificationPolicy.decide(null, watch(running = false), "t1").isEmpty()
        )
        // 上一拍就没在跑:空闲 → 空闲
        assertTrue(
            PaNotificationPolicy.decide(watch(running = false), watch(running = false), "t1").isEmpty()
        )
        // running 升沿(开始跑)不是完成
        assertTrue(
            PaNotificationPolicy.decide(watch(running = false), watch(running = true), "t1").isEmpty()
        )
    }

    @Test
    fun `审批通知_出现发_解决撤`() {
        // null → 非 null:发
        val shown = PaNotificationPolicy.decide(
            prev = watch(running = true, approval = false),
            now = watch(running = true, approval = true),
            threadId = "t1",
        )
        assertEquals(listOf<PaNotificationPolicy.Decision>(PaNotificationPolicy.Decision.ApprovalShown("t1")), shown)
        // 首见即有审批:也补发(监控启动晚于审批到达)
        val shownLate = PaNotificationPolicy.decide(null, watch(running = true, approval = true), "t1")
        assertEquals(listOf<PaNotificationPolicy.Decision>(PaNotificationPolicy.Decision.ApprovalShown("t1")), shownLate)
        // 非 null → null:撤
        val gone = PaNotificationPolicy.decide(
            prev = watch(running = true, approval = true),
            now = watch(running = true, approval = false),
            threadId = "t1",
        )
        assertEquals(listOf<PaNotificationPolicy.Decision>(PaNotificationPolicy.Decision.ApprovalGone("t1")), gone)
        // 审批持续存在:不动
        assertTrue(
            PaNotificationPolicy.decide(
                watch(running = true, approval = true),
                watch(running = true, approval = true),
                "t1",
            ).isEmpty()
        )
    }

    @Test
    fun `同拍_完成加审批解决_两个动作都执行`() {
        PaNotificationPolicy.appInForeground = false
        val out = PaNotificationPolicy.decide(
            prev = watch(running = true, approval = true),
            now = watch(running = false, approval = false),
            threadId = "t1",
        )
        assertEquals(2, out.size)
        assertTrue(out[0] is PaNotificationPolicy.Decision.TurnDone)
        assertTrue(out[1] is PaNotificationPolicy.Decision.ApprovalGone)
    }

    // ------------------------------------------------------------ 前台标志

    @Test
    fun `前台标志单例_可写可读_默认后台`() {
        assertFalse(PaNotificationPolicy.appInForeground)
        PaNotificationPolicy.appInForeground = true
        assertTrue(PaNotificationPolicy.appInForeground)
        PaNotificationPolicy.appInForeground = false
        assertFalse(PaNotificationPolicy.appInForeground)
        // 决策确实吃到这个标志:同一转移,前台与后台结果不同
        PaNotificationPolicy.appInForeground = true
        assertTrue(
            PaNotificationPolicy.decide(watch(true), watch(false), "t").isEmpty()
        )
        PaNotificationPolicy.appInForeground = false
        assertEquals(
            1, PaNotificationPolicy.decide(watch(true), watch(false), "t").size
        )
    }

    // ------------------------------------------------------------ extra 深链解析

    @Test
    fun `extra解析_空值不导航_合法id去空白`() {
        assertNull(PaNotificationPolicy.pendingNavigationFrom(null))
        assertNull(PaNotificationPolicy.pendingNavigationFrom(""))
        assertNull(PaNotificationPolicy.pendingNavigationFrom("   "))
        assertEquals("thread-9", PaNotificationPolicy.pendingNavigationFrom("thread-9"))
        assertEquals("thread-9", PaNotificationPolicy.pendingNavigationFrom("  thread-9 "))
    }

    // ------------------------------------------------------------ 通知 id

    @Test
    fun `通知id_同线程稳定_两通道区间隔离_避开保活服务id1`() {
        for (tid in listOf("abc", "t1", "", "长长长长会话id-1234567890")) {
            val done = PaNotificationPolicy.turnDoneNotificationId(tid)
            val approval = PaNotificationPolicy.approvalNotificationId(tid)
            assertEquals(done, PaNotificationPolicy.turnDoneNotificationId(tid))
            assertEquals(approval, PaNotificationPolicy.approvalNotificationId(tid))
            assertTrue(done in 100_000 until 500_000)
            assertTrue(approval in 500_000 until 900_000)
            assertNotEquals(1, done)
            assertNotEquals(1, approval)
            assertNotEquals(done, approval)
        }
    }

    // ------------------------------------------------------------ 摘要与审批文案

    @Test
    fun `摘要_压成一行并截60字带省略号`() {
        val long = "第一行\n第二行  很多   空格" + "字".repeat(120)
        val s = PaNotificationPolicy.summarizeForNotification(long)
        assertTrue(s.length <= 60)
        assertTrue(s.endsWith("…"))
        assertFalse(s.contains("\n"))
        // 不超长则原样(压空白)
        assertEquals("a b c", PaNotificationPolicy.summarizeForNotification("a\n b\t c"))
        // 空文本
        assertEquals("", PaNotificationPolicy.summarizeForNotification("  \n "))
    }

    @Test
    fun `完成摘要_无助手文本时给人话兜底`() {
        assertEquals("任务已完成,点按查看结果", PaNotificationPolicy.turnDoneSummary(""))
        assertEquals("任务已完成,点按查看结果", PaNotificationPolicy.turnDoneSummary("   "))
        assertEquals("修好了登录页", PaNotificationPolicy.turnDoneSummary("修好了登录页"))
    }

    @Test
    fun `审批摘要_shell取命令_非法JSON退回原文_未知类型取名`() {
        assertEquals(
            "npm run build",
            PaNotificationPolicy.approvalHeadline("shell", """{"command":"npm run build"}"""),
        )
        // 非法 JSON:退回原始参数(截断)
        val fallback = PaNotificationPolicy.approvalHeadline("shell", "not-json{")
        assertEquals("not-json{", fallback)
        // 未知动作且参数为空:给动作名,绝不让通知正文空白
        assertEquals("some_action", PaNotificationPolicy.approvalHeadline("some_action", ""))
    }

    @Test
    fun `审批摘要_改文件取首个文件名并带数量`() {
        val headline = PaNotificationPolicy.approvalHeadline(
            "apply_patch",
            """{"files":["a.py","b.py","c.py"],"count":3}""",
        )
        assertTrue(headline.startsWith("a.py"))
        assertTrue(headline.contains("3 个文件"))
        // 无文件列表但有 count
        assertEquals("共 2 个文件改动", PaNotificationPolicy.approvalHeadline("file_change", """{"count":2}"""))
    }

    @Test
    fun `审批详情_折叠展示完整参数并截断`() {
        assertEquals("", PaNotificationPolicy.approvalDetail("shell", ""))
        val args = """{"command":"${"x".repeat(1000)}"}"""
        val detail = PaNotificationPolicy.approvalDetail("shell", args)
        assertTrue(detail.length <= 600)
        assertTrue(detail.endsWith("…"))
    }

    // ------------------------------------------------------------ 语音状态机

    @Test
    fun `语音_有权限点麦克风_进入听写并产出开始指令`() {
        var ui = voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(hasPermission = true, available = true))
        assertEquals(VoiceInputState.LISTENING, ui.state)
        val fx = pendingVoiceEffects(ui)
        assertTrue(fx.start)
        assertFalse(fx.stop)
        assertNull(fx.append)
        assertNull(fx.toast)
    }

    @Test
    fun `语音_无权限_状态机原地不动_由组合层去申请`() {
        val ui = voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(hasPermission = false, available = true))
        assertEquals(VoiceInputState.IDLE, ui.state)
        assertFalse(pendingVoiceEffects(ui).start)
    }

    @Test
    fun `语音_权限被拒绝_记无权限并给提示`() {
        val ui = voiceReduce(VoiceInputUi(), VoiceEvent.PermissionAnswered(granted = false, available = true))
        assertEquals(VoiceInputState.NO_PERMISSION, ui.state)
        // 【可行动化】拒绝后必须指路"去哪里开"(MIUI 拒过两次后系统不再弹框)
        val toast = pendingVoiceEffects(ui).toast!!
        assertTrue(toast.contains("系统设置"))
        assertTrue(toast.contains("麦克风"))
    }

    @Test
    fun `语音_授权但设备不支持_记不可用`() {
        val ui = voiceReduce(VoiceInputUi(), VoiceEvent.PermissionAnswered(granted = true, available = false))
        assertEquals(VoiceInputState.UNAVAILABLE, ui.state)
        assertNotNull(pendingVoiceEffects(ui).toast)
    }

    @Test
    fun `语音_出结果_文本并入输入框_空结果提示没听到`() {
        var ui = voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(true, true))
        ui = voiceReduce(ui, VoiceEvent.RecognitionResults("加一个登录页"))
        assertEquals(VoiceInputState.IDLE, ui.state)
        assertEquals("加一个登录页", pendingVoiceEffects(ui).append)
        // 空结果:不追加,只提示
        ui = voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(true, true))
        ui = voiceReduce(ui, VoiceEvent.RecognitionResults("   "))
        assertNull(pendingVoiceEffects(ui).append)
        assertEquals("没听到,再试一次", pendingVoiceEffects(ui).toast)
    }

    @Test
    fun `语音_听写中再点一次_停止并出结果`() {
        var ui = voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(true, true))
        ui = voiceReduce(ui, VoiceEvent.MicClicked(true, true))
        assertEquals(VoiceInputState.IDLE, ui.state)
        assertTrue(pendingVoiceEffects(ui).stop)
        // 随后结果到达 → 正常追加
        ui = voiceReduce(ui, VoiceEvent.RecognitionResults("继续"))
        assertEquals("继续", pendingVoiceEffects(ui).append)
    }

    @Test
    fun `语音_出错_回空闲并给人话提示`() {
        var ui = voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(true, true))
        ui = voiceReduce(ui, VoiceEvent.RecognitionFailed(VoiceErrorKind.NO_MATCH))
        assertEquals(VoiceInputState.IDLE, ui.state)
        assertEquals("没听到,再试一次", ui.toast)
        assertEquals("语音服务正忙,稍后再试", voiceErrorText(VoiceErrorKind.BUSY))
        assertEquals("系统语音服务权限受限,已自动切换端侧识别;若仍失败,请在系统设置里开启麦克风权限", voiceErrorText(VoiceErrorKind.INSUFFICIENT_PERMISSIONS))
        assertEquals("语音识别不可用", voiceErrorText(VoiceErrorKind.CLIENT))
        assertEquals("识别失败,请重试", voiceErrorText(VoiceErrorKind.OTHER))
    }

    @Test
    fun `语音_指令消费后一次性字段清空_状态保留`() {
        var ui = voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(true, true))
        ui = voiceEffectsConsumed(ui)
        assertEquals(VoiceInputState.LISTENING, ui.state)
        val fx = pendingVoiceEffects(ui)
        assertFalse(fx.start)
        assertFalse(fx.stop)
        assertNull(fx.append)
        assertNull(fx.toast)
    }

    @Test
    fun `语音_错误码映射`() {
        // SpeechRecognizer 的错误码常量在编译期内联,这里用字面量钉住映射
        assertEquals(VoiceErrorKind.NO_MATCH, voiceErrorKind(7)) // ERROR_NO_MATCH
        assertEquals(VoiceErrorKind.NO_MATCH, voiceErrorKind(6)) // ERROR_SPEECH_TIMEOUT
        assertEquals(VoiceErrorKind.BUSY, voiceErrorKind(8)) // ERROR_RECOGNIZER_BUSY
        assertEquals(VoiceErrorKind.INSUFFICIENT_PERMISSIONS, voiceErrorKind(9)) // ERROR_INSUFFICIENT_PERMISSIONS
        assertEquals(VoiceErrorKind.CLIENT, voiceErrorKind(5)) // ERROR_CLIENT
        assertEquals(VoiceErrorKind.OTHER, voiceErrorKind(1)) // ERROR_NETWORK_TIMEOUT 等
    }

    @Test
    fun `语音_文本并入输入框_不覆盖已有内容`() {
        assertEquals("加一个登录页", joinVoiceText("", "加一个登录页"))
        assertEquals("已有内容 加一个登录页", joinVoiceText("已有内容", "加一个登录页"))
        assertEquals("已有内容 ", joinVoiceText("已有内容 ", ""))
        assertEquals("第一行\n第二段", joinVoiceText("第一行\n", "第二段"))
        // 全空白输入视同空输入,直接放
        assertEquals("语音文本", joinVoiceText("  ", "语音文本"))
    }

    /** SpeechRecognizer 的接口包装:单测用 fake 验证"状态机指令 → 识别器动作"。 */
    private class FakeRecognizer : PaVoiceRecognizer {
        var starts = 0
        var stops = 0
        var destroys = 0
        override fun start() { starts++ }
        override fun stopListening() { stops++ }
        override fun destroy() { destroys++ }
    }

    @Test
    fun `语音_fake识别器_按状态机指令执行start与stop`() {
        val rec = FakeRecognizer()
        // 组合层的消费逻辑(与 ChatScreen LaunchedEffect 一致):取指令 → 执行 → 清除
        fun consume(current: VoiceInputUi): VoiceInputUi {
            val fx = pendingVoiceEffects(current)
            if (fx.start) rec.start()
            if (fx.stop) rec.stopListening()
            return voiceEffectsConsumed(current)
        }
        var ui = consume(voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(true, true)))
        assertEquals(1, rec.starts)
        assertEquals(0, rec.stops)
        // 再点一次 = 停止
        ui = consume(voiceReduce(ui, VoiceEvent.MicClicked(true, true)))
        assertEquals(1, rec.starts)
        assertEquals(1, rec.stops)
        // 空闲时点击带 no-op 消费,不会多发指令
        consume(ui)
        assertEquals(1, rec.starts)
        assertEquals(1, rec.stops)
    }

    // ------------------------------------------------------------ V-1 修复回归(效果投递不丢)

    @Test
    fun `语音_V1_事件经applyVoiceEvent后效果当场取出_展示态不携带一次性字段`() {
        // 点击:指令当场取出,展示态只剩 LISTENING
        val (ui1, fx1) = applyVoiceEvent(VoiceInputUi(), VoiceEvent.MicClicked(true, true))
        assertEquals(VoiceInputState.LISTENING, ui1.state)
        assertTrue(fx1.start)
        // 展示态永不再携带指令 —— 旧实现靠"二次消费写 + equals 重启"在这里丢效果
        val residue = pendingVoiceEffects(ui1)
        assertFalse(residue.start)
        assertFalse(residue.stop)
        assertNull(residue.append)
        assertNull(residue.toast)
        // 识别失败:新指令照常产出,状态回 IDLE
        val (ui2, fx2) = applyVoiceEvent(ui1, VoiceEvent.RecognitionFailed(VoiceErrorKind.CLIENT))
        assertEquals(VoiceInputState.IDLE, ui2.state)
        assertEquals("语音识别不可用", fx2.toast)
    }

    @Test
    fun `语音_V1_连续快速点击_每条指令独立入队_不因同值状态写入被吞`() {
        // 锁嫌疑②:两次连续 MicClicked(开始→立刻停止)落在同一拍时,
        // 旧实现两次 state 写可能同值/竞争而吞掉第二条;新实现逐事件入队,必达。
        var ui = VoiceInputUi()
        val queue = mutableListOf<VoiceEffects>()
        fun feed(event: VoiceEvent) {
            val (display, fx) = applyVoiceEvent(ui, event)
            ui = display
            if (voiceEffectsPending(fx)) queue.add(fx)
        }
        feed(VoiceEvent.MicClicked(true, true)) // 开始
        feed(VoiceEvent.MicClicked(true, true)) // 立刻再点 = 停止
        assertEquals(2, queue.size)
        assertTrue(queue[0].start)
        assertTrue(queue[1].stop)
        // 随后结果到达,append 指令同样入队
        feed(VoiceEvent.RecognitionResults("继续"))
        assertEquals(3, queue.size)
        assertEquals("继续", queue[2].append)
        assertEquals(VoiceInputState.IDLE, ui.state)
    }

    @Test
    fun `语音_V1_不可用设备点击_直接UNAVAILABLE并提示`() {
        val (ui, fx) = applyVoiceEvent(VoiceInputUi(), VoiceEvent.MicClicked(true, false))
        assertEquals(VoiceInputState.UNAVAILABLE, ui.state)
        assertEquals("这台设备不支持语音识别", fx.toast)
        assertTrue(voiceEffectsPending(fx))
    }

    @Test
    fun `语音_V1_start同步抛异常_按CLIENT错误回空闲并给人话提示`() {
        // 胶水层把 startListening 同步异常兜底为 ERROR_CLIENT(编译期常量 = 5,字面量钉住)
        assertEquals(VoiceErrorKind.CLIENT, voiceErrorKind(5))
        val (ui1, _) = applyVoiceEvent(VoiceInputUi(), VoiceEvent.MicClicked(true, true))
        assertEquals(VoiceInputState.LISTENING, ui1.state)
        // start 抛异常 → onErrorEvent(ERROR_CLIENT) → RecognitionFailed(CLIENT)
        val (ui2, fx2) = applyVoiceEvent(ui1, VoiceEvent.RecognitionFailed(voiceErrorKind(5)))
        assertEquals(VoiceInputState.IDLE, ui2.state)
        assertEquals("语音识别不可用", fx2.toast)
        assertTrue(voiceEffectsPending(fx2))
    }

    // ------------------------------------------------------------ 死区超时兜底(V-1 后续)

    @Test
    fun `语音_死区超时判定_到达阈值且零回调才算死区`() {
        // 未到阈值:不判定(哪怕只差 1ms)
        assertFalse(voiceDeadZoneTimedOut(0, gotCallback = false))
        assertFalse(voiceDeadZoneTimedOut(VOICE_DEAD_ZONE_TIMEOUT_MS - 1, gotCallback = false))
        // 到阈值且零回调:判定死区(取等即触发,兜底宁早勿卡)
        assertTrue(voiceDeadZoneTimedOut(VOICE_DEAD_ZONE_TIMEOUT_MS, gotCallback = false))
        assertTrue(voiceDeadZoneTimedOut(VOICE_DEAD_ZONE_TIMEOUT_MS + 5_000, gotCallback = false))
        // 期间收到过任一关键回调(ready/partial/results/error):服务活着,永不判定
        assertFalse(voiceDeadZoneTimedOut(VOICE_DEAD_ZONE_TIMEOUT_MS + 60_000, gotCallback = true))
        // 阈值可注入(看门狗实现无关,判定逻辑独立可测)
        assertTrue(voiceDeadZoneTimedOut(1_000, gotCallback = false, timeoutMs = 1_000))
        assertFalse(voiceDeadZoneTimedOut(999, gotCallback = false, timeoutMs = 1_000))
    }

    @Test
    fun `语音_死区超时事件_回空闲并给人话提示_指令待消费`() {
        // 卡在 LISTENING(零回调死区)→ 看门狗抛 DeadZoneTimeout
        var ui = voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(true, true))
        assertEquals(VoiceInputState.LISTENING, ui.state)
        ui = voiceReduce(ui, VoiceEvent.DeadZoneTimeout(firstColdLoad = false))
        assertEquals(VoiceInputState.IDLE, ui.state)
        assertEquals("语音服务无响应,请重试或检查语音服务", pendingVoiceEffects(ui).toast)
        // 经组合层统一入口(applyVoiceEvent)同样成立:指令当场取出,展示态干净
        val (display, fx) = applyVoiceEvent(
            voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(true, true)).let { voiceEffectsConsumed(it) },
            VoiceEvent.DeadZoneTimeout(firstColdLoad = false),
        )
        assertEquals(VoiceInputState.IDLE, display.state)
        assertEquals("语音服务无响应,请重试或检查语音服务", fx.toast)
        assertTrue(voiceEffectsPending(fx))
        // 空闲态误收死区事件也只是幂等回 IDLE + 提示,不产生 start/stop/append
        val (idle, fxIdle) = applyVoiceEvent(VoiceInputUi(), VoiceEvent.DeadZoneTimeout(firstColdLoad = false))
        assertEquals(VoiceInputState.IDLE, idle.state)
        assertNull(fxIdle.append)
        assertFalse(fxIdle.start)
        assertFalse(fxIdle.stop)
    }

    // ------------------------------------------------- P2-d:端侧首次冷加载的死区分流

    @Test
    fun `语音_P2d_死区文案_端侧首次冷加载提示加载中_其余按服务无响应`() {
        // 首次冷加载(端侧引擎本识别器尚无一次成功识别):文案说真话,引导稍候再试
        assertEquals("正在加载语音模型,请稍候再试", voiceDeadZoneToast(firstColdLoad = true))
        // 已跑通过一轮(模型已暖)或系统引擎:维持原"服务无响应"提示
        assertEquals("语音服务无响应,请重试或检查语音服务", voiceDeadZoneToast(firstColdLoad = false))
    }

    @Test
    fun `语音_P2d_死区事件_端侧首次冷加载_回空闲并提示加载中`() {
        // 卡在 LISTENING(冷加载零回调撞满看门狗)→ 回 IDLE 不卡死,文案是"加载中"
        var ui = voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(true, true))
        assertEquals(VoiceInputState.LISTENING, ui.state)
        ui = voiceReduce(ui, VoiceEvent.DeadZoneTimeout(firstColdLoad = true))
        assertEquals(VoiceInputState.IDLE, ui.state)
        assertEquals("正在加载语音模型,请稍候再试", pendingVoiceEffects(ui).toast)
        // 经组合层统一入口同样成立:指令当场取出,展示态干净
        val (display, fx) = applyVoiceEvent(
            voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(true, true)).let { voiceEffectsConsumed(it) },
            VoiceEvent.DeadZoneTimeout(firstColdLoad = true),
        )
        assertEquals(VoiceInputState.IDLE, display.state)
        assertEquals("正在加载语音模型,请稍候再试", fx.toast)
        assertTrue(voiceEffectsPending(fx))
        // 默认参数 = 非首次加载(系统引擎/模型已暖):与显式 false 等价,老路径文案不变
        assertEquals(VoiceEvent.DeadZoneTimeout(firstColdLoad = false), VoiceEvent.DeadZoneTimeout())
    }
}
