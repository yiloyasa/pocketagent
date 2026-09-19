package dev.pocketagent.app.ui

// PromptOptimizeLogicTest —— 输入栏"优化提示词"纯逻辑的 JVM 单测:
//   1) 状态机:按钮可见性四条件(开关/离线/非空/在飞)与请求生命周期迁移;
//   2) 失败文案分类:未连接短路、daemon 人话透传、按 code 兜底。

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
}
