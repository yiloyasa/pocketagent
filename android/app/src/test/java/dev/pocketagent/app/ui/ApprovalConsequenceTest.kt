package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 审批敢点(快赢版)回归:默认焦点 + 一句话后果 + 关闭文案。
 *
 * 红线:只改呈现与默认焦点,审批语义(状态机/应答通道)一字不动 ——
 * 本文件只钉文案与焦点 id,不碰 EditorialApprovalLogicTest 的状态机。
 */
class ApprovalConsequenceTest {

    // ---- 一句话后果:只读已有字段,不编造 ----

    @Test
    fun `常规读操作_后果只说只查看_不编造文件名`() {
        val copy = approvalConsequenceCopy("执行命令 · 仅本次", "ls -la")
        assertEquals("这一步只查看，不会改动内容；点拒绝也是停在这里，什么都不改。", copy)
    }

    @Test
    fun `高危操作_后果并列命中原因_不断言具体路径`() {
        val copy = approvalConsequenceCopy("执行命令 · 仅本次", "sudo rm -rf build")
        // 原因来自 approvalRiskOf 同一信号源(删除文件/提升权限),非编造
        assertTrue(copy.contains("删除文件"))
        assertTrue(copy.contains("提升权限"))
        assertTrue(copy.startsWith("点允许会继续执行"))
        assertTrue(copy.endsWith("什么都不改。"))
    }

    @Test
    fun `写入类动作_后果同样高危_原因来自动作类型词`() {
        val copy = approvalConsequenceCopy("写入文件 · 仅本次", "")
        assertTrue(copy.contains("覆盖或改动文件"))
    }

    // ---- 默认焦点:永远最安全的那颗(拒绝) ----

    @Test
    fun `旧两按钮_默认焦点是拒绝`() {
        assertEquals("deny", approvalDefaultFocusButtonId(emptyList()))
    }

    @Test
    fun `三按钮_默认焦点是拒绝的原词id`() {
        val id = approvalDefaultFocusButtonId(
            listOf(
                ApprovalOptionUi("accept"),
                ApprovalOptionUi("acceptForSession"),
                ApprovalOptionUi("decline"),
            ),
        )
        assertEquals("decline", id)
    }

    @Test
    fun `未知deny类选项_默认焦点仍落拒绝色档`() {
        val id = approvalDefaultFocusButtonId(listOf(ApprovalOptionUi("DenyOnce"), ApprovalOptionUi("custom_weird", "奇怪选项")))
        assertEquals("DenyOnce", id)
    }

    // ---- 关闭文案:稍后再定,任务先停着 ----

    @Test
    fun `关闭承诺行_明确写稍后再定任务先停着`() {
        assertTrue(APPROVAL_DISMISS_COPY.contains("稍后再定，任务先停着"))
        // 关闭 ≠ 拒绝的承诺仍在
        assertTrue(APPROVAL_DISMISS_COPY.contains("不会自动允许或拒绝"))
    }

    // ---- 用户可见文案禁词纪律 ----

    @Test
    fun `审批新增文案_无禁词`() {
        val copies = listOf(
            approvalConsequenceCopy("", "ls"),
            approvalConsequenceCopy("", "sudo rm -rf /"),
            APPROVAL_DISMISS_COPY,
        )
        copies.forEach { copy ->
            assertFalse("不应出现「未知」:$copy", copy.contains("未知"))
            assertFalse("不应出现「错误」:$copy", copy.contains("错误"))
            assertFalse("用户文案禁英文技术词 token:$copy", copy.lowercase().contains("token"))
            assertFalse("用户文案禁英文技术词 daemon:$copy", copy.lowercase().contains("daemon"))
            assertFalse("用户文案禁英文技术词 adb:$copy", copy.lowercase().contains("adb"))
        }
    }
}
