package dev.pocketagent.app.ui

// PromptOptimizeLogicTest —— 输入栏"优化提示词"纯逻辑的 JVM 单测:
//   1) 状态机:按钮可见性四条件(开关/离线/非空/在飞)与请求生命周期迁移;
//   2) 失败文案分类:未连接短路、daemon 人话透传、按 code 兜底;
//   3) 风格词表(P4 Task 1):七种枚举 → daemon wire 字符串的映射;
//   4) 失败 kind 分类(P4 Task 1):配额/登录/连接/超时四类人话映射。

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PromptOptimizeLogicTest {

    private val inputsOn = { text: String ->
        PromptOptimizeEvent.InputsChanged(input = text, featureEnabled = true, offline = false)
    }

    // ---- 状态机:可见性 ----

    @Test
    fun `按钮仅开关开且输入非空且在线时可见`() {
        val idle = PromptOptimizeState()
        assertTrue(reducePromptOptimize(idle, inputsOn("帮我修 bug")).buttonVisible)
        // 开关关
        assertFalse(
            reducePromptOptimize(
                idle,
                PromptOptimizeEvent.InputsChanged("修 bug", featureEnabled = false, offline = false),
            ).buttonVisible,
        )
        // 离线快照态
        assertFalse(
            reducePromptOptimize(
                idle,
                PromptOptimizeEvent.InputsChanged("修 bug", featureEnabled = true, offline = true),
            ).buttonVisible,
        )
        // 输入空白(空串/纯空白都不露出)
        assertFalse(reducePromptOptimize(idle, inputsOn("")).buttonVisible)
        assertFalse(reducePromptOptimize(idle, inputsOn("   ")).buttonVisible)
    }

    // ---- 状态机:请求生命周期 ----

    @Test
    fun `点击后进入请求中且按钮隐藏防重复点击`() {
        val visible = reducePromptOptimize(PromptOptimizeState(), inputsOn("修 bug"))
        val started = reducePromptOptimize(visible, PromptOptimizeEvent.OptimizeStarted)
        assertTrue(started.inFlight)
        assertFalse(started.buttonVisible)
        // 请求在飞时输入再变化:可见性保持 false(防重复点击)
        val during = reducePromptOptimize(started, inputsOn("修 bug 改了改"))
        assertFalse(during.buttonVisible)
        assertTrue(during.inFlight)
        // 不可见的按钮点击是 no-op:状态不进入请求中
        val hidden = reducePromptOptimize(PromptOptimizeState(), inputsOn(""))
        val ignored = reducePromptOptimize(hidden, PromptOptimizeEvent.OptimizeStarted)
        assertFalse(ignored.inFlight)
    }

    @Test
    fun `终局回到闲置并在输入变化后重新可见`() {
        val visible = reducePromptOptimize(PromptOptimizeState(), inputsOn("修 bug"))
        val started = reducePromptOptimize(visible, PromptOptimizeEvent.OptimizeStarted)
        // 成功:输入已被替换(优化后文本非空)→ InputsChanged 重新点亮按钮
        val done = reducePromptOptimize(started, PromptOptimizeEvent.OptimizeSucceeded)
        assertFalse(done.inFlight)
        assertTrue(reducePromptOptimize(done, inputsOn("优化后的清晰指令")).buttonVisible)
        // 失败:原文未动 → 同样回到可见
        val failed = reducePromptOptimize(started, PromptOptimizeEvent.OptimizeFailed)
        assertFalse(failed.inFlight)
        assertTrue(reducePromptOptimize(failed, inputsOn("修 bug")).buttonVisible)
    }

    // ---- 失败文案分类 ----

    @Test
    fun `未连接优先短路不透传 daemon 文案`() {
        assertEquals(
            "未连接电脑,请先连接再优化",
            optimizePromptErrorText("UNAVAILABLE", "需要电脑端 ZCode 且已登录", connected = false),
        )
    }

    @Test
    fun `daemon 人话文案原样透传`() {
        // 配额/ZCode 不可用等分类都由 daemon 侧生成,手机端不二次意译
        assertEquals(
            "电脑端 ZCode 配额已用完,暂时无法优化;可直接发送原文,或稍后再试",
            optimizePromptErrorText("UNAVAILABLE", "电脑端 ZCode 配额已用完,暂时无法优化;可直接发送原文,或稍后再试", connected = true),
        )
    }

    @Test
    fun `description 缺失按 code 兜底`() {
        assertEquals(
            "优化超时了,请稍后再试(原文未被改动)",
            optimizePromptErrorText("DEADLINE_EXCEEDED", null, connected = true),
        )
        assertEquals(
            "连接电脑中断或 ZCode 不可用,请检查连接与桌面端 ZCode",
            optimizePromptErrorText("UNAVAILABLE", "", connected = true),
        )
        assertEquals(
            "先输入内容,再点优化",
            optimizePromptErrorText("INVALID_ARGUMENT", null, connected = true),
        )
        assertEquals(
            "优化失败,请稍后再试(原文未被改动)",
            optimizePromptErrorText("UNKNOWN", null, connected = true),
        )
    }

    // ---- 风格词表:P4 Task 1 枚举 → daemon wire 字符串 ----

    @Test
    fun `七种风格映射到 daemon 词表`() {
        // 口径:全小写 + 下划线(daemon TrimSpace+ToLower 后匹配);默认 = 空串。
        assertEquals("", PromptOptimizeStyle.DEFAULT.wireValue)
        assertEquals("concise", PromptOptimizeStyle.CONCISE.wireValue)
        assertEquals("detailed", PromptOptimizeStyle.DETAILED.wireValue)
        assertEquals("implementation_plan", PromptOptimizeStyle.IMPLEMENTATION_PLAN.wireValue)
        assertEquals("bug_report", PromptOptimizeStyle.BUG_REPORT.wireValue)
        assertEquals("test_plan", PromptOptimizeStyle.TEST_PLAN.wireValue)
        assertEquals("code_review", PromptOptimizeStyle.CODE_REVIEW.wireValue)
        // 七值齐全且 wire 词表互不撞车(撞车 = 两种风格发同一条 daemon 指令)
        assertEquals(7, PromptOptimizeStyle.values().size)
        assertEquals(7, PromptOptimizeStyle.values().map { it.wireValue }.toSet().size)
    }

    // ---- 风格说明(P4 Task 3 可解释):预览芯片与设置页共用的词表 ----

    @Test
    fun `七风格各有一句话说明且口径与 daemon 模板对齐`() {
        // 说明词表七值齐全、互不撞车(撞车 = 两种模式在 UI 上分不开)
        val hints = PromptOptimizeStyle.values().map { promptOptimizeStyleHint(it) }
        assertEquals(7, hints.size)
        assertEquals(7, hints.toSet().size)
        // 每条说明都含该风格 daemon 模板段的关键词(与 prompt_opt.go 的
        // promptOptimizeStyleTemplates 逐条对齐,改模板必须同步改这里)
        val markerOf = mapOf(
            PromptOptimizeStyle.DEFAULT to "不扩写需求",
            PromptOptimizeStyle.CONCISE to "一句话",
            PromptOptimizeStyle.DETAILED to "要点列表",
            PromptOptimizeStyle.IMPLEMENTATION_PLAN to "验收标准",
            PromptOptimizeStyle.BUG_REPORT to "复现",
            PromptOptimizeStyle.TEST_PLAN to "验证方式",
            PromptOptimizeStyle.CODE_REVIEW to "证据要求",
        )
        PromptOptimizeStyle.values().forEach { style ->
            val hint = promptOptimizeStyleHint(style)
            assertTrue(
                "style=$style 的说明 $hint 应含 daemon 模板关键词 ${markerOf[style]}",
                hint.contains(markerOf[style]!!),
            )
        }
    }

    // ---- 失败分类(kind):P4 Task 1 四类人话映射 ----

    @Test
    fun `配额类 code 与关键词都归 QUOTA`() {
        assertEquals(
            PromptOptimizeErrorKind.QUOTA,
            classifyPromptOptimizeFailure("RESOURCE_EXHAUSTED", null, connected = true).kind,
        )
        // daemon 实测配额文案以 UNAVAILABLE 等通用 code 回来,靠 description 关键词改判
        val quota = classifyPromptOptimizeFailure(
            "UNAVAILABLE",
            "电脑端 ZCode 配额已用完,暂时无法优化;可直接发送原文,或稍后再试",
            connected = true,
        )
        assertEquals(PromptOptimizeErrorKind.QUOTA, quota.kind)
        assertEquals("电脑端 ZCode 配额已用完,暂时无法优化;可直接发送原文,或稍后再试", quota.message)
    }

    @Test
    fun `登录类 code 与关键词都归 AUTH`() {
        assertEquals(
            PromptOptimizeErrorKind.AUTH,
            classifyPromptOptimizeFailure("UNAUTHENTICATED", null, connected = true).kind,
        )
        assertEquals(
            PromptOptimizeErrorKind.AUTH,
            classifyPromptOptimizeFailure("PERMISSION_DENIED", null, connected = true).kind,
        )
        val login = classifyPromptOptimizeFailure("UNKNOWN", "电脑端登录状态失效,请重新登录", connected = true)
        assertEquals(PromptOptimizeErrorKind.AUTH, login.kind)
        assertEquals("电脑端登录状态失效,请重新登录", login.message) // daemon 人话透传
    }

    @Test
    fun `连接类含未连接短路归 CONNECTION`() {
        assertEquals(
            PromptOptimizeErrorKind.CONNECTION,
            classifyPromptOptimizeFailure("UNAVAILABLE", null, connected = true).kind,
        )
        // 未连接短路优先级最高:即使 daemon 说了别的也先拦下
        assertEquals(
            PromptOptimizeErrorKind.CONNECTION,
            classifyPromptOptimizeFailure(null, null, connected = false).kind,
        )
        assertEquals(
            "未连接电脑,请先连接再优化",
            classifyPromptOptimizeFailure(null, null, connected = false).message,
        )
    }

    @Test
    fun `超时与 daemon 侧取消归 TIMEOUT 其余归 OTHER`() {
        assertEquals(
            PromptOptimizeErrorKind.TIMEOUT,
            classifyPromptOptimizeFailure("DEADLINE_EXCEEDED", null, connected = true).kind,
        )
        assertEquals(
            PromptOptimizeErrorKind.TIMEOUT,
            classifyPromptOptimizeFailure("CANCELLED", null, connected = true).kind,
        )
        assertEquals(
            PromptOptimizeErrorKind.OTHER,
            classifyPromptOptimizeFailure("INVALID_ARGUMENT", null, connected = true).kind,
        )
        assertEquals(
            "先输入内容,再点优化",
            classifyPromptOptimizeFailure("INVALID_ARGUMENT", null, connected = true).message,
        )
        // 手机端主动取消不走本分类(仓库层直接归 CANCELLED),这里只兜未知类
        assertEquals(
            PromptOptimizeErrorKind.OTHER,
            classifyPromptOptimizeFailure("UNKNOWN", null, connected = true).kind,
        )
    }
}
