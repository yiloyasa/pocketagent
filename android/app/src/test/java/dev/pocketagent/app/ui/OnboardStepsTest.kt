package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 新手教程脊柱的单测(首次体验轮):**连接主机 → 配对 → 发第一条指令**,
 * 每步一屏一句话一动作,口语化不堆术语;已完成步骤用真实连接状态标注,不重复打扰。
 */
class OnboardStepsTest {

    // ---- 脊柱顺序 ----

    @Test
    fun `脊柱顺序_连接_配对_首条指令_排障`() {
        val ids = onboardSteps(connected = false).map { it.id }
        assertEquals(listOf("connect", "pair", "first-task", "trouble"), ids)
    }

    @Test
    fun `前三步就是调研钉死的三件事`() {
        val spine = onboardSteps(connected = false).take(3).map { it.id }
        assertEquals(listOf("connect", "pair", "first-task"), spine)
    }

    // ---- 完成位只认真实连接状态 ----

    @Test
    fun `已连接_连接与配对两步标记完成_不再展开打扰`() {
        val steps = onboardSteps(connected = true)
        assertTrue(steps.first { it.id == "connect" }.done)
        assertTrue(steps.first { it.id == "pair" }.done)
    }

    @Test
    fun `未连接_全部步骤按未完成讲`() {
        val steps = onboardSteps(connected = false)
        assertTrue(steps.none { it.done })
    }

    @Test
    fun `首条指令没有可靠完成事实_不编造_恒按未完成`() {
        val steps = onboardSteps(connected = true)
        assertFalse(steps.first { it.id == "first-task" }.done)
    }

    // ---- 一屏一句话一动作 ----

    @Test
    fun `每步都有非空的一句话与一个动作`() {
        onboardSteps(connected = false).forEach { step ->
            assertTrue("步骤 ${step.id} 缺一句话", step.sentence.isNotBlank())
            assertTrue("步骤 ${step.id} 缺一个动作", step.action.isNotBlank())
        }
    }

    @Test
    fun `已完成的步骤带替换话术_首条指令不带`() {
        val steps = onboardSteps(connected = true)
        assertTrue(steps.first { it.id == "connect" }.note.isNotBlank())
        assertTrue(steps.first { it.id == "pair" }.note.isNotBlank())
        assertFalse(steps.first { it.id == "first-task" }.note.isNotBlank())
    }

    // ---- 不堆术语:禁词表扫描(面向通用用户) ----

    @Test
    fun `全部文案不含术语禁词`() {
        val banned = listOf("token", "daemon", "tailnet", "adb", "grpc", "api", "cli", "paguard", "onnx")
        onboardSteps(connected = true).forEach { step ->
            val all = "${step.sentence} ${step.action} ${step.note}".lowercase()
            banned.forEach { word ->
                assertFalse("步骤 ${step.id} 出现术语「$word」", all.contains(word))
            }
        }
    }
}
