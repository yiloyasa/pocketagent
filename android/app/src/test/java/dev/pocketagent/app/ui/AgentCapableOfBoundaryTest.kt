package dev.pocketagent.app.ui

// AgentCapableOfBoundaryTest —— agentCapableOf 判定函数的边界用例(纯 JVM,无 Android 依赖)。
//
// 【分工】agentCapableOf 是手机端所有能力位门控的统一判定入口(SlashCommandsLogic.kt,
// daemon 在 GetHostInfo.capabilities 里逐 agent 宣告 `<位>:<agentId>`,见
// daemon/internal/server/host.go 与 docs/2026-09-30-agent-capability-matrix.md)。
// 既有钉子:ThreadActionGateTest 沿"入口包装函数"(compactActionOn/forkActionOn/
// rollbackActionOn)与 steer/queue_clear 用例覆盖了主路径 —— 本文件直接打
// agentCapableOf 本体,钉它**自身**的边界口径,防止将来判定函数被改坏而入口
// 包装函数的用例恰好没踩到:
//   - 未知 agent / 未知能力位 = 拒绝(硬红线"无能力不显示假入口");
//   - 空能力集 = 拒绝(旧 daemon/未连机);
//   - 大小写/首尾空白不敏感(与 queueClearVisible、runtimeControlCapable 同口径);
//   - 整串相等语义:冒号后带空格、别家 agent 的位、超集串都不误命中。
//
// 【红线口径出处】SlashCommandsLogic.kt agentCapableOf KDoc:"agentId 为空/空白
// = 绑定不到具体会话,一律 false;只认绑定同一 agent 的位"。

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class AgentCapableOfBoundaryTest {

    /** 模拟 daemon 能力位回报(子集即可,位族全集见矩阵文档主矩阵 A)。 */
    private val caps = listOf(
        "agent:pi:v0.87.1",
        "agent:zcode",
        "steer:codex", "steer:pi",
        "queue_clear:pi",
        "image_input:zcode",
        "compact:codex", "compact:pi", "compact:zcode",
        "review:codex",
        "export_html:pi",
        "runtime.v1",
    )

    // ---- 未知 agent:别家的位不开我家的门 ----

    @Test
    fun `未知agent_即便能力集里有位也拒绝`() {
        // daemon 从不上报这些 id;旧库残值/将来第 5 家在位到位前一律拒绝
        assertFalse(agentCapableOf("steer", "nope", caps))
        assertFalse(agentCapableOf("compact", "dsh", caps))
        assertFalse(agentCapableOf("steer", "echo", caps))
    }

    // ---- 未知能力位:agent 真实存在也没这门 ----

    @Test
    fun `未知能力位_即便agent真实存在也拒绝`() {
        // 位族全集里没有 "fork:pi"(fork 仅 codex),更没有 "usage" 这类未生成过的位
        assertFalse(agentCapableOf("fork", "pi", caps))
        assertFalse(agentCapableOf("usage", "pi", caps))
        assertFalse(agentCapableOf("rollback", "pi", caps))
    }

    @Test
    fun `未知能力位_空能力集与真实agent组合仍拒绝`() {
        assertFalse(agentCapableOf("usage", "pi", emptyList()))
        assertFalse(agentCapableOf("", "pi", caps)) // 空位名:不会存在 ":pi" 串
    }

    // ---- 空集与空 agentId ----

    @Test
    fun `空能力集_一律拒绝`() {
        for (id in listOf("codex", "pi", "zcode", "dsh")) {
            assertFalse(agentCapableOf("steer", id, emptyList()))
        }
    }

    @Test
    fun `agentId为null空串或空白_一律拒绝`() {
        assertFalse(agentCapableOf("steer", null, caps))
        assertFalse(agentCapableOf("steer", "", caps))
        assertFalse(agentCapableOf("steer", "   ", caps))
    }

    // ---- 大小写与首尾空白不敏感 ----

    @Test
    fun `大小写与首尾空白不敏感`() {
        assertTrue(agentCapableOf("STEER", "pi", caps))
        assertTrue(agentCapableOf("steer", "PI", caps))
        assertTrue(agentCapableOf(" Steer ", " pi ", caps))
        assertTrue(agentCapableOf("steer", "pi", listOf("  steer:pi  ")))
    }

    // ---- 整串相等语义:防前缀/子串误命中 ----

    @Test
    fun `位串冒号后带空格_不误命中`() {
        // daemon 不会发这种格式;整串相等(而非 split/包含)是既定口径
        assertFalse(agentCapableOf("steer", "pi", listOf("steer: pi")))
    }

    @Test
    fun `更长的位串不是该位的前缀命中`() {
        // "steer:pi" 不该被 "steer:pi:v2" 这类超集串命中(daemon 不发;防御将来格式演化)
        assertFalse(agentCapableOf("steer", "pi", listOf("steer:pi:v2")))
        // 反向:位名带尾缀也不该命中(review:codex 在场不等于 reviewX:codex 在场)
        assertFalse(agentCapableOf("reviews", "codex", caps))
    }

    @Test
    fun `成员位带版本号的pi格式_不走agentCapableOf命中`() {
        // agent:pi:v0.87.1 是带版本号的成员位;agentCapableOf 是整串相等判定,
        // 对版本号格式**不命中** —— 成员位(装没装/已装 vX)的点亮走
        // resolveAgentAvailability 的前缀匹配(见 OnboardingAgentCapabilityTest)。
        assertFalse(agentCapableOf("agent", "pi", caps))
        // 非版本号成员位(agent:zcode)才按整串相等命中
        assertTrue(agentCapableOf("agent", "zcode", caps))
        // 版本号串也不构成其他位的命中源
        assertFalse(agentCapableOf("agent", "piv0", caps))
    }
}
