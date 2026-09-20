package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.MicUnavailableException
import dev.pocketagent.app.ui.component.SenseVoiceSessionRunner
import dev.pocketagent.app.ui.component.VOICE_ERROR_ENGINE
import dev.pocketagent.app.ui.component.VOICE_ERROR_ENGINE_BUSY
import dev.pocketagent.app.ui.component.VOICE_ERROR_MIC_BLOCKED
import dev.pocketagent.app.ui.component.captureSenseVoiceAudio
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assert.assertThrows
import org.junit.Test

/**
 * 端侧 SenseVoice 兜底与语音状态机纯逻辑的守护单测。
 *
 * 【沿革】本文件承自旧遗留端侧引擎的回落测试(2026-09-20 该引擎整体摘除后改写):
 * 引擎选择决策表、端侧错误码映射、死区分流文案等行为钉子以改名/改写方式全部保留;
 * 随引擎移除的纯内部用例(下载状态机/zip 解压/安装编排/哈希校验/识别循环调度)按
 * 审核口径删除,用例总数不低于删前(24 例),删除清单登记于汇报与 AUDIT 附录 B。
 *
 * 覆盖面:
 *  1. 引擎选择决策表(系统 SR 不可用 → 回退端侧 SenseVoice,行为钉子);
 *  2. 端侧自定义错误码(100/102/103)映射与人话提示、未定义码兜底;
 *  3. 死区超时文案分流(冷加载 vs 服务无响应);
 *  4. 采音调度(captureSenseVoiceAudio:归一化/参数校验/超长读/时长上限);
 *  5. 会话执行体生命周期(SenseVoiceSessionRunner:忙/销毁/异常码/投递时序)。
 */
class SenseVoiceFallbackTest {

    // ------------------------------------------------------------ 引擎选择决策表

    @Test
    fun `引擎选择_系统可用一律SYSTEM_在线质量优先`() {
        assertEquals(VoiceEngineKind.SYSTEM, pickVoiceEngine(sysAvailable = true, senseReady = true))
        assertEquals(VoiceEngineKind.SYSTEM, pickVoiceEngine(sysAvailable = true, senseReady = false))
    }

    @Test
    fun `引擎选择_系统不可用端侧就绪回退SENSEVOICE_都不行UNAVAILABLE`() {
        // 行为钉子:小米等无识别服务 ROM(isRecognitionAvailable=false)必须落到端侧,
        // 而不是 UNAVAILABLE;系统与端侧都不行才降级(点击麦克风给人话提示)
        assertEquals(VoiceEngineKind.SENSEVOICE, pickVoiceEngine(sysAvailable = false, senseReady = true))
        assertEquals(VoiceEngineKind.UNAVAILABLE, pickVoiceEngine(sysAvailable = false, senseReady = false))
    }

    // ------------------------------------------------------------ 端侧错误码映射

    @Test
    fun `端侧100引擎异常映射ENGINE_系统码1_7_8_9映射不受影响`() {
        assertEquals(VoiceErrorKind.ENGINE, voiceErrorKind(VOICE_ERROR_ENGINE))
        // 系统码 1..9 原有映射不被端侧分支吞掉
        assertEquals(VoiceErrorKind.OTHER, voiceErrorKind(1))
        assertEquals(VoiceErrorKind.NO_MATCH, voiceErrorKind(7))
        assertEquals(VoiceErrorKind.BUSY, voiceErrorKind(8))
        assertEquals(VoiceErrorKind.INSUFFICIENT_PERMISSIONS, voiceErrorKind(9))
        // 端侧分类的人话提示
        assertEquals("端侧语音引擎异常,请重试", voiceErrorText(VoiceErrorKind.ENGINE))
    }

    @Test
    fun `端侧102麦克风被拦103引擎忙映射_人话提示可行动`() {
        assertEquals(VoiceErrorKind.MIC_BLOCKED, voiceErrorKind(VOICE_ERROR_MIC_BLOCKED))
        val micText = voiceErrorText(VoiceErrorKind.MIC_BLOCKED)
        // MIC_BLOCKED 必须给"去哪里开"的可行动指引(MIUI 安全中心独立「录音」开关)
        assertTrue(micText.contains("安全中心"))
        // 引擎忙:与系统忙同文案
        assertEquals(VoiceErrorKind.BUSY, voiceErrorKind(VOICE_ERROR_ENGINE_BUSY))
        assertEquals("语音服务正忙,稍后再试", voiceErrorText(VoiceErrorKind.BUSY))
    }

    @Test
    fun `未定义码101落OTHER兜底_大码与负数同安全`() {
        // 101=旧遗留引擎的模型加载失败码,2026-09-20 随该引擎整体摘除移除:
        // 该码不再由任何引擎产生,voiceErrorKind 不再有专属分支 —— 若出现落
        // OTHER 兜底("识别失败,请重试"),不误报端侧引擎异常
        assertEquals(VoiceErrorKind.OTHER, voiceErrorKind(101))
        assertEquals(VoiceErrorKind.OTHER, voiceErrorKind(1000))
        assertEquals(VoiceErrorKind.OTHER, voiceErrorKind(-1))
    }

    // ------------------------------------------------------------ 死区分流文案

    @Test
    fun `死区分流文案_端侧首次冷加载提示加载中_默认按服务无响应`() {
        assertEquals("正在加载语音模型,请稍候再试", voiceDeadZoneToast(firstColdLoad = true))
        assertEquals("语音服务无响应,请重试或检查语音服务", voiceDeadZoneToast(firstColdLoad = false))
    }

    @Test
    fun `死区事件经状态机回空闲_文案按冷加载分流_指令待消费`() {
        var ui = voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(true, true))
        assertEquals(VoiceInputState.LISTENING, ui.state)
        // 冷加载分支:回 IDLE + 加载中文案
        ui = voiceReduce(ui, VoiceEvent.DeadZoneTimeout(firstColdLoad = true))
        assertEquals(VoiceInputState.IDLE, ui.state)
        assertEquals("正在加载语音模型,请稍候再试", pendingVoiceEffects(ui).toast)
        // 非冷加载(组合层现役恒传 false):服务无响应文案
        val (display, fx) = applyVoiceEvent(VoiceInputUi(), VoiceEvent.DeadZoneTimeout(firstColdLoad = false))
        assertEquals(VoiceInputState.IDLE, display.state)
        assertEquals("语音服务无响应,请重试或检查语音服务", fx.toast)
        assertTrue(voiceEffectsPending(fx))
        assertFalse(fx.start)
        assertFalse(fx.stop)
    }

    // ------------------------------------------------------------ 采音调度(captureSenseVoiceAudio)

    @Test
    fun `采集_short样本按32768归一化_float输出`() {
        val audio = captureSenseVoiceAudio(
            read = { buffer, _ ->
                buffer[0] = 32767
                buffer[1] = -32768
                2
            },
            stopped = { false }, nowMillis = { 0L }, idle = {},
            maxSamples = 2, maxMillis = 60_000,
        )
        assertEquals(2, audio.size)
        assertEquals(32767 / 32768f, audio[0], 0f)
        assertEquals(-1f, audio[1], 0f)
    }

    @Test
    fun `采集_零采样或零时长参数require拒绝`() {
        assertThrows(IllegalArgumentException::class.java) {
            captureSenseVoiceAudio(read = { _, _ -> 0 }, stopped = { false }, maxSamples = 0)
        }
        assertThrows(IllegalArgumentException::class.java) {
            captureSenseVoiceAudio(read = { _, _ -> 0 }, stopped = { false }, maxMillis = 0)
        }
    }

    @Test
    fun `采集_读返回超长一律视为异常_绝不静默截断`() {
        // read 返回值超过本次申请长度 = 数据源契约被破坏,check 必须抛而不是静默采错
        assertThrows(IllegalStateException::class.java) {
            captureSenseVoiceAudio(
                read = { _, size -> size + 1 },
                stopped = { false }, nowMillis = { 0L }, idle = {},
                maxSamples = 5, maxMillis = 60_000,
            )
        }
    }

    @Test
    fun `采集_时长上限即停_样本未满也收尾`() {
        var reads = 0
        val audio = captureSenseVoiceAudio(
            read = { _, _ -> reads++; 1 },
            stopped = { false },
            nowMillis = { reads * 10L }, // 每读一次推进 10ms
            idle = {},
            maxSamples = 100, maxMillis = 50,
        )
        // 50ms 上限先于 100 样本上限到达:恰好采 5 个样本即收尾,不空转
        assertEquals(5, audio.size)
        assertEquals(5, reads)
    }

    // ------------------------------------------------------------ 会话执行体(SenseVoiceSessionRunner)

    /** 常用骨架:post/launch 均为手动队列,JVM 全确定性。 */
    private fun newRunner(
        main: ArrayDeque<() -> Unit>,
        workers: ArrayDeque<() -> Unit>,
        onResult: (String) -> Unit = {},
        onError: (Int) -> Unit = {},
        onAlive: () -> Unit = {},
        work: (SenseVoiceSessionRunner.Session) -> String,
    ) = SenseVoiceSessionRunner(
        post = { main.add(it) }, launch = { workers.add(it) },
        onResult = onResult, onError = onError, onAlive = onAlive, work = work,
    )

    @Test
    fun `会话_上一会话运行中start被拒_返回false且不派新worker`() {
        val main = ArrayDeque<() -> Unit>()
        val workers = ArrayDeque<() -> Unit>()
        val runner = newRunner(main, workers) { "result" }
        assertTrue(runner.start())
        // 运行中再次点击 = 忙:识别器据此上报 103(VOICE_ERROR_ENGINE_BUSY)给人话提示,
        // 绝不能静默吞掉用户的第二次点击
        assertFalse(runner.start())
        assertEquals(1, workers.size)
    }

    @Test
    fun `会话_destroy后再start返回false_永不再派worker`() {
        val main = ArrayDeque<() -> Unit>()
        val workers = ArrayDeque<() -> Unit>()
        val results = mutableListOf<String>()
        val runner = newRunner(main, workers, onResult = { results.add(it) }) { "result" }
        assertTrue(runner.start())
        workers.removeFirst()()
        main.removeFirst()()
        assertEquals(listOf("result"), results)
        runner.destroy()
        // 已销毁实例不可再用(与系统 SpeechRecognizer 语义一致);重进会话页会新建实例
        assertFalse(runner.start())
        assertTrue(workers.isEmpty())
    }

    @Test
    fun `会话_未分类异常报100端侧引擎异常码`() {
        val main = ArrayDeque<() -> Unit>()
        val workers = ArrayDeque<() -> Unit>()
        val errors = mutableListOf<Int>()
        val runner = newRunner(main, workers, onError = { errors.add(it) }) {
            throw IllegalStateException("转写器挂了")
        }
        runner.start()
        workers.removeFirst()()
        main.removeFirst()()
        // catch(Throwable) 兜底出口:未分类异常 → VOICE_ERROR_ENGINE(100) → 端侧引擎异常提示
        assertEquals(listOf(VOICE_ERROR_ENGINE), errors)
    }

    @Test
    fun `会话_麦克风不可用报102_权限异常报9_类型契约成立`() {
        // MicUnavailableException 必须是 IllegalStateException 子类:执行体按类型
        // 精确捕获,绝不能被更宽的分支提前吞掉
        assertEquals(
            IllegalStateException::class.java,
            MicUnavailableException("录音器初始化失败").javaClass.superclass,
        )

        val micMain = ArrayDeque<() -> Unit>()
        val micWorkers = ArrayDeque<() -> Unit>()
        val micErrors = mutableListOf<Int>()
        val micRunner = newRunner(micMain, micWorkers, onError = { micErrors.add(it) }) {
            throw MicUnavailableException("录音器未能进入录音态")
        }
        micRunner.start()
        micWorkers.removeFirst()()
        micMain.removeFirst()()
        // MIUI「录音」开关关闭/被占用 → 102 → MIC_BLOCKED 可行动指引
        assertEquals(listOf(VOICE_ERROR_MIC_BLOCKED), micErrors)

        val permMain = ArrayDeque<() -> Unit>()
        val permWorkers = ArrayDeque<() -> Unit>()
        val permErrors = mutableListOf<Int>()
        val permRunner = newRunner(permMain, permWorkers, onError = { permErrors.add(it) }) {
            throw SecurityException()
        }
        permRunner.start()
        permWorkers.removeFirst()()
        permMain.removeFirst()()
        // SecurityException 先于通用分支:9(权限)有专门"去设置开启"提示
        assertEquals(listOf(9), permErrors)
    }

    @Test
    fun `会话_空白转写按无匹配上报7_非空白结果正常回调`() {
        val blankMain = ArrayDeque<() -> Unit>()
        val blankWorkers = ArrayDeque<() -> Unit>()
        val blankErrors = mutableListOf<Int>()
        val blankRunner = newRunner(blankMain, blankWorkers, onError = { blankErrors.add(it) }) { "   " }
        blankRunner.start()
        blankWorkers.removeFirst()()
        blankMain.removeFirst()()
        // 空白结果 = 没听到:报 7(ERROR_NO_MATCH)走"没听到,再试一次",不报引擎异常
        assertEquals(listOf(7), blankErrors)

        val okMain = ArrayDeque<() -> Unit>()
        val okWorkers = ArrayDeque<() -> Unit>()
        val results = mutableListOf<String>()
        val okRunner = newRunner(okMain, okWorkers, onResult = { results.add(it) }) { "你好" }
        okRunner.start()
        okWorkers.removeFirst()()
        okMain.removeFirst()()
        assertEquals(listOf("你好"), results)
    }

    @Test
    fun `会话_结果必经post投递_未派发前零回调`() {
        val main = ArrayDeque<() -> Unit>()
        val workers = ArrayDeque<() -> Unit>()
        val results = mutableListOf<String>()
        val runner = newRunner(main, workers, onResult = { results.add(it) }) { "result" }
        runner.start()
        workers.removeFirst()() // 工作完成,回调已排队
        // 投递只发生在 post(主线程)派发时:识别器线程绝不直接碰 UI 状态
        assertTrue(results.isEmpty())
        main.removeFirst()()
        assertEquals(listOf("result"), results)
    }

    @Test
    fun `会话_alive经post透传_工作期喂狗可送达`() {
        val main = ArrayDeque<() -> Unit>()
        val workers = ArrayDeque<() -> Unit>()
        var alives = 0
        val runner = newRunner(main, workers, onAlive = { alives++ }) { session ->
            session.alive()
            session.alive()
            "result"
        }
        runner.start()
        workers.removeFirst()()
        assertEquals(0, alives) // 同样经 post 排队,未派发前不触发
        main.forEach { it() }
        assertEquals(2, alives) // 看门狗喂狗路径:每次 alive 都要能送达
    }

    @Test
    fun `会话_stop后已在运行的工作照常收尾交付`() {
        val main = ArrayDeque<() -> Unit>()
        val workers = ArrayDeque<() -> Unit>()
        val results = mutableListOf<String>()
        val runner = newRunner(main, workers, onResult = { results.add(it) }) { session ->
            // 转写进行中用户点了停止:停止只关采音,不召回已产出的转写
            session.stopped = true
            "result"
        }
        runner.start()
        workers.removeFirst()()
        main.removeFirst()()
        // 停止不取消已交付语义:已完成的转写结果仍落输入框(识别不白听);
        // "停止后不再开始新工作"由 start 前置检查负责(见 stop-before-work 用例)
        assertEquals(listOf("result"), results)
    }

    // ------------------------------------------------------------ 系统弃用名单与死区判定(守护复钉)

    @Test
    fun `系统弃用名单_端侧码与未知码一律不弃用`() {
        // 100+ 端侧自定义码到达时系统引擎本就未被选中,绝不能误伤;
        // 101 已随旧遗留引擎摘除,作为"未知码"同样必须安全
        for (code in listOf(
            VOICE_ERROR_ENGINE, VOICE_ERROR_MIC_BLOCKED, VOICE_ERROR_ENGINE_BUSY,
            0, -1, 10, 99, 101,
        )) {
            assertFalse("code=$code 不应触发弃用", systemVoiceRetireNeeded(code))
        }
    }

    @Test
    fun `死区超时判定_恰好等于阈值触发_有回调或未到点不触发`() {
        assertTrue(voiceDeadZoneTimedOut(8_000, gotCallback = false))
        assertFalse(voiceDeadZoneTimedOut(7_999, gotCallback = false))
        assertTrue(voiceDeadZoneTimedOut(8_001, gotCallback = false))
        // 任一关键回调到达 = 服务活着,绝不判死
        assertFalse(voiceDeadZoneTimedOut(9_999, gotCallback = true))
    }

    @Test
    fun `错误码常量值迁移保真_100_102_103与摘除前一致`() {
        // 2026-09-20 共享定义迁移(旧引擎文件 → VoiceErrors.kt):常量名去遗留化,
        // 值绝不允许漂移 —— 它们落在 SpeechRecognizer 1..9 之外的 100+ 区间,
        // 是"端侧码不误伤系统弃用名单"判定的一部分
        assertEquals(100, VOICE_ERROR_ENGINE)
        assertEquals(102, VOICE_ERROR_MIC_BLOCKED)
        assertEquals(103, VOICE_ERROR_ENGINE_BUSY)
    }

    @Test
    fun `端侧忙103与系统忙8同归BUSY同人话提示`() {
        assertEquals(voiceErrorKind(8), voiceErrorKind(VOICE_ERROR_ENGINE_BUSY))
        assertEquals(voiceErrorText(VoiceErrorKind.BUSY), voiceErrorText(voiceErrorKind(VOICE_ERROR_ENGINE_BUSY)))
    }

    @Test
    fun `人话提示全枚举非空且绝不出现未知或错误字样`() {
        // AUDIT §3.3:UI 禁显「未知/错误」,必须给人话/原生原文 —— 全枚举守护
        for (kind in VoiceErrorKind.entries) {
            val text = voiceErrorText(kind)
            assertTrue("$kind 提示不应为空", text.isNotBlank())
            assertFalse("$kind 提示不应出现「未知」", text.contains("未知"))
            assertFalse("$kind 提示不应出现「错误」", text.contains("错误"))
        }
    }
}
