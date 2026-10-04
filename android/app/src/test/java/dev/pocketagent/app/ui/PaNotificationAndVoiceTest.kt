package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.PaNotificationPolicy
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * P1-6 本地通知 + P1-9 语音 的纯逻辑单测:
 *  1. 通知触发矩阵(running→false 且前台/后台;审批出现/消失);
 *  2. 前台标志读写;extra 深链解析;通知 id 稳定与区间隔离;
 *  3. 语音纯逻辑遗留桥钉子(错误码映射/落框拼接)。
 *     【P4 Task 4/5】语音状态机/指令链/死区/静音门已迁 ui/voice,其守护单测在
 *     VoiceSessionReducerTest(含 Controller 状态机壳),此处不再测旧胶水。
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

    // ------------------------------------------------------------ 语音纯逻辑遗留桥
    // 【P4 Task 4/5 迁移说明】语音状态机/指令链/死区兜底/跨会话静音门已整体迁往
    // ui/voice(VoiceSessionReducer + VoiceController,由 VoiceSessionReducerTest 全覆盖),
    // 测已删代码的用例随之移除。这里只保留仍被 ChatScreen 尾部遗留桥提供、并被
    // SystemVoiceEngine(错误码单一真值源)与 Final 落框接线(joinVoiceText)消费的
    // 两个纯函数行为钉子。

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
}
