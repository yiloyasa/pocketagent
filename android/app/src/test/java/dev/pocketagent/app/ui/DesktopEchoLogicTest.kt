package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.CodexDesktopConfigSummary
import pocketagent.v1.CodexProviderCaps
import pocketagent.v1.CodexSettings

/**
 * 桌面只读回显展示纯函数的钉子(桌面只读四口子透出手机的最后一段)。
 *
 * 重点钉三条纪律:
 *  1. 空值/缺省不占位 —— 没读到绝不下饭一句"电脑端当前:";
 *  2. 认不出的取值原样展示 —— codex 新增档位时手机端照镜子,不瞎翻译;
 *  3. "没读到 provider 能力"不渲染徽标 —— 不把"没读到"说成"不支持"。
 */
class DesktopEchoLogicTest {

    /** 组一份"daemon 读到了桌面配置"的 CodexSettings。 */
    private fun settingsWith(
        model: String = "gpt-5.4-turbo",
        provider: String = "dex_ai",
        effort: String = "high",
        summary: String = "detailed",
        verbosity: String = "medium",
        sandboxMode: String = "workspace-write",
        approvalPolicy: String = "on-request",
    ): CodexSettings = CodexSettings.newBuilder()
        .setDesktopConfigSummary(
            CodexDesktopConfigSummary.newBuilder()
                .setModel(model)
                .setProvider(provider)
                .setEffort(effort)
                .setSummary(summary)
                .setVerbosity(verbosity)
                .setSandboxMode(sandboxMode)
                .setApprovalPolicy(approvalPolicy),
        )
        .build()

    @Test
    fun `echo line renders localized label for known raw values`() {
        val s = settingsWith()
        assertEquals("电脑端当前:按需询问", DesktopEchoLogic.desktopEchoLine(s, "approval_policy"))
        assertEquals("电脑端当前:工作区可写", DesktopEchoLogic.desktopEchoLine(s, "sandbox"))
        assertEquals("电脑端当前:gpt-5.4-turbo", DesktopEchoLogic.desktopEchoLine(s, "model"))
        assertEquals("电脑端当前:高", DesktopEchoLogic.desktopEchoLine(s, "effort"))
        assertEquals("电脑端当前:适中", DesktopEchoLogic.desktopEchoLine(s, "model_verbosity"))
        assertEquals("电脑端当前:详细", DesktopEchoLogic.desktopEchoLine(s, "model_reasoning_summary"))
    }

    @Test
    fun `unknown raw value is echoed verbatim`() {
        // codex 以后新增档位:原样展示,不猜文案。
        val s = settingsWith(effort = "ultra", approvalPolicy = "yolo")
        assertEquals("电脑端当前:ultra", DesktopEchoLogic.desktopEchoLine(s, "effort"))
        assertEquals("电脑端当前:yolo", DesktopEchoLogic.desktopEchoLine(s, "approval_policy"))
    }

    @Test
    fun `blank value and missing summary occupy no line`() {
        // 电脑端该项未显式配置:codex 用内置默认,手机端不占位。
        assertNull(DesktopEchoLogic.desktopEchoLine(settingsWith(effort = ""), "effort"))
        // 整份 desktop_config_summary 缺省(读取失败 / 老 daemon):全部不占位。
        val bare = CodexSettings.getDefaultInstance()
        assertNull(DesktopEchoLogic.desktopEchoLine(bare, "model"))
        assertNull(DesktopEchoLogic.desktopEchoLine(null, "model"))
    }

    @Test
    fun `hide_agent_reasoning has no desktop counterpart`() {
        // codex 无此配置键:七项里唯独它没有"电脑端当前"回显。
        assertNull(DesktopEchoLogic.desktopEchoLine(settingsWith(), "hide_agent_reasoning"))
    }

    @Test
    fun `sandbox readiness line maps known states and echoes unknown`() {
        assertEquals(
            "Windows 沙箱:可用",
            DesktopEchoLogic.sandboxReadinessLine(
                settingsWith().toBuilder().setSandboxReadiness("ready").build(),
            ),
        )
        assertEquals(
            "Windows 沙箱:未配置",
            DesktopEchoLogic.sandboxReadinessLine(
                settingsWith().toBuilder().setSandboxReadiness("notConfigured").build(),
            ),
        )
        assertEquals(
            "Windows 沙箱:需要更新",
            DesktopEchoLogic.sandboxReadinessLine(
                settingsWith().toBuilder().setSandboxReadiness("updateRequired").build(),
            ),
        )
        // 未知新状态:原样展示(照镜子)。
        assertEquals(
            "Windows 沙箱:weirdNewState",
            DesktopEchoLogic.sandboxReadinessLine(
                settingsWith().toBuilder().setSandboxReadiness("weirdNewState").build(),
            ),
        )
        // 没读到:不占位。
        assertNull(DesktopEchoLogic.sandboxReadinessLine(settingsWith()))
        assertNull(DesktopEchoLogic.sandboxReadinessLine(null))
    }

    @Test
    fun `provider caps badges render only when the caps message exists`() {
        // 没读到 provider_caps:空列表,绝不渲染"都不支持"。
        assertTrue(DesktopEchoLogic.providerCapsBadges(settingsWith()).isEmpty())
        // 读到了但都是 false:也不渲染徽标(没有可吹的能力)。
        val noneTrue = settingsWith().toBuilder()
            .setProviderCaps(CodexProviderCaps.newBuilder()).build()
        assertTrue(DesktopEchoLogic.providerCapsBadges(noneTrue).isEmpty())
        // 读到了且有真值:按真实布尔渲染。
        val caps = settingsWith().toBuilder()
            .setProviderCaps(
                CodexProviderCaps.newBuilder().setImageGeneration(true).setWebSearch(true),
            )
            .build()
        assertEquals(listOf("图像生成", "联网搜索"), DesktopEchoLogic.providerCapsBadges(caps))
    }
}
