package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 设置一级页(R1-C 信息架构轮)取值/摘要纯函数的钉子。
 *
 * 【为什么必须钉】一级导航行新增了"当前值摘要"列,这些摘要的取值纪律:
 *  - 本地偏好类(桥接开关/下载清单/主机指纹)断连必须照常出值 —— 摘要缺席
 *    会被读成"这项丢了";
 *  - 依赖主机的项(审批/沙箱/电脑信息)快照缺席必须明确标"需连接主机",
 *    绝不显示空占位"—"(会被读成"没有值");
 *  - 词表外的原始值显示原文,不猜(AUDIT §3.3)。
 */
class SettingsOverviewLogicTest {

    // ---- 连接摘要 ---------------------------------------------------------------

    @Test
    fun `连接摘要 ok 带通道名 空通道回落直连`() {
        assertEquals("已连接 · USB", connectionSummary("ok", "USB"))
        assertEquals("已连接 · 直连", connectionSummary("ok", ""))
    }

    @Test
    fun `连接摘要 断连与检查中`() {
        assertEquals("未连接", connectionSummary("down", "USB"))
        assertEquals("检查中", connectionSummary("checking", ""))
    }

    // ---- 下载与存储摘要(断连可用)-------------------------------------------------

    @Test
    fun `下载与存储摘要 0 个不摆 0`() {
        assertEquals("暂无下载内容", downloadStorageSummary(0))
        assertEquals("3 个文件", downloadStorageSummary(3))
    }

    // ---- 主机依赖摘要(断连标注"需连接主机")--------------------------------------

    @Test
    fun `电脑摘要 有值才拼 缺席按连接状态如实说`() {
        assertEquals("Windows/x64", hostPlatformSummary("Windows", "x64", "ok"))
        assertEquals("Windows", hostPlatformSummary("Windows", "", "ok"))
        assertEquals("读取中…", hostPlatformSummary(null, null, "ok"))
        assertEquals("需连接主机", hostPlatformSummary(null, null, "down"))
        assertEquals("检查中", hostPlatformSummary(null, null, "checking"))
    }

    @Test
    fun `审批标签 词表内译中文_空为跟随桌面_词表外显示原文`() {
        assertEquals("仅信任", approvalPolicyLabel("untrusted"))
        assertEquals("按需询问", approvalPolicyLabel("on-request"))
        assertEquals("完全访问", approvalPolicyLabel("never"))
        assertEquals("跟随桌面", approvalPolicyLabel(null))
        assertEquals("跟随桌面", approvalPolicyLabel(""))
        assertEquals("new-policy", approvalPolicyLabel("new-policy"))
    }

    @Test
    fun `沙箱标签 词表内译中文_空为跟随桌面_词表外显示原文`() {
        assertEquals("只读", sandboxModeLabel("read-only"))
        assertEquals("工作区可写", sandboxModeLabel("workspace-write"))
        assertEquals("完全访问", sandboxModeLabel("danger-full-access"))
        assertEquals("跟随桌面", sandboxModeLabel(null))
        assertEquals("trace", sandboxModeLabel("trace"))
    }

    @Test
    fun `审批与沙箱摘要 快照缺席标需连接主机 不显示空占位`() {
        assertEquals("需连接主机", approvalSandboxSummary(null, null, settingsReady = false))
        assertEquals(
            "未连接时绝不能把空值渲染成'跟随桌面'(会被读成已确认的取值)",
            "需连接主机",
            approvalSandboxSummary("", "", settingsReady = false),
        )
    }

    @Test
    fun `审批与沙箱摘要 读到后给审批·沙箱拼串`() {
        assertEquals(
            "按需询问 · 工作区可写",
            approvalSandboxSummary("on-request", "workspace-write", settingsReady = true),
        )
        assertEquals(
            "跟随桌面 · 只读",
            approvalSandboxSummary("", "read-only", settingsReady = true),
        )
    }

    // ---- S2/F2:Daemon 桥接行摘要随「Daemon 桥接与输入」页消亡而移除 ----------
    // (F2:一级页 Daemon 行删除,daemonBridgeSummary 纯函数一并下线,
    //  优化按钮开关改在「对话与输入」组就地展示,不再有行摘要。)

    // ---- 主机信任摘要(「连接与配对」页内复用同一纯函数)--------------------------

    @Test
    fun `主机信任摘要 未锁定陈述事实_已锁定给前16位`() {
        assertEquals("首连时自动锁定", hostTrustSummary(null))
        assertEquals("首连时自动锁定", hostTrustSummary(""))
        val pin = "0123456789abcdef0123456789abcdef"
        val shown = hostTrustSummary(pin)
        assertEquals("已锁定 · 0123456789abcdef", shown)
        assertTrue("摘要只给前 16 位,不整串上屏", !shown.contains("0123456789abcdef0123"))
    }
}
