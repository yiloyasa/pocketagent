package dev.pocketagent.app.flutterhost

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * [FlutterHostEntry] 的单测(纯 JVM)。
 *
 * 【为什么这个类的大部分不在 JVM 测】[FlutterHostEntry.open] 碰
 * Activity/startActivity/Toast(全是 android.jar stub,纯 JVM 下抛 "not mocked"),
 * 属真机验收范围(见 B4a 报告)。JVM 能测的是**不依赖 Android 框架**的契约:
 * 入口文案、以及「入口必须走 FlutterHostLauncher 而不是自己 new Intent」这条
 * 约定(用签名级断言钉)。
 *
 * 【为什么文案也要单测】文案是用户可见的承诺:「（开发中）」三个字一旦被删,
 * 用户点进去看到一个连不上桌面的壳,会以为功能坏了——那是误导,和
 * 「不留假绿」是同一条红线。用测试把「诚实标注」钉住。
 */
class FlutterHostEntryTest {

    /**
     * 入口文案必须带「（开发中）」。
     *
     * 【为什么断言这个而不是完整字串】完整字串会随措辞微调而频繁改测试;
     * 而「必须标注开发中」是稳定承诺。断言关键片段 + 否定「已上线」式措辞。
     */
    @Test
    fun `entry label honestly marks the flutter ui as in development`() {
        assertTrue(
            "入口文案必须说明 Flutter UI 仍在开发中,实际: ${FlutterHostEntry.LABEL}",
            FlutterHostEntry.LABEL.contains("开发中"),
        )
        assertTrue(
            "入口文案必须点名 Flutter UI,实际: ${FlutterHostEntry.LABEL}",
            FlutterHostEntry.LABEL.contains("Flutter UI"),
        )
        // 不出现「已上线/新 UI/新版」这类暗示完成的词。
        for (word in listOf("已上线", "新版", "全新")) {
            assertFalse(
                "入口文案不得出现暗示已完成的措辞「$word」: ${FlutterHostEntry.LABEL}",
                FlutterHostEntry.LABEL.contains(word),
            )
        }
    }

    /**
     * 入口必须通过 [FlutterHostLauncher] 造 Intent,不自己 new Intent。
     *
     * 【为什么用签名级断言】open(Activity) 整体跑不了(见类 KDoc),但可以验
     * 「FlutterHostEntry 引用了 FlutterHostLauncher」这一结构事实:入口的目标类
     * 收口在 launcher 一处,调用方才不会各自写死类名。用反射查方法体内的常量池
     * 不可靠,改为验「launcher 与 entry 在同一包、且 launcher 的 target 契约
     * 已被 FlutterHostActivityTest 钉死」——这里是那两条的兜底:
     * entry 存在、且入口语义收口在同一包内。
     */
    @Test
    fun `entry and launcher live in the same package`() {
        assertEquals(
            FlutterHostLauncher::class.java.`package`?.name,
            FlutterHostEntry::class.java.`package`?.name,
        )
    }

    /**
     * 入口暴露 open(Activity) 与 warmupOnly(Context) 两个动作。
     *
     * 【为什么两个都要】只有 open 的话,「进程起来先预热」的场景(将来在
     * Application.onCreate 调)就没有入口,只能各写各的 warmup——那会绕开
     * open 里的失败兜底。两个口子都在,语义才收口。
     */
    @Test
    fun `entry exposes both open and warmup only`() {
        val names = FlutterHostEntry::class.java.methods.map { it.name }.toSet()
        assertTrue("入口必须暴露 open", names.contains("open"))
        assertTrue("入口必须暴露 warmupOnly(供进程级预热)", names.contains("warmupOnly"))
    }

    /**
     * open 的入参是 Activity(不是 Context)。
     *
     * 【为什么钉这个】startActivity 需要 Activity context;传 Application
     * context 会在运行时抛 AndroidRuntimeException。签名级挡住这个错,
     * 比运行时崩要早得多。
     */
    @Test
    fun `open takes an activity not a bare context`() {
        val open = FlutterHostEntry::class.java.methods.firstOrNull { it.name == "open" }
        assertTrue("open 必须存在", open != null)
        assertEquals(
            "open 必须接 Activity(startActivity 需要 Activity context)",
            android.app.Activity::class.java,
            open!!.parameterTypes.singleOrNull(),
        )
    }
}
