package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 审批危险分级的纯函数单测(approvalRiskOf / approvalRiskVisualOf)。
 *
 * 【分级纪律】词表保守取向:宁可多提醒一句,不给破坏性操作静默放行 ——
 * 假阳性代价是用户多看一眼命令原文,假阴性代价是数据。分级只决定风险条的
 * 音量(高危警示条 vs 常规安静行),**不决定按钮可达性**。
 */
class ApprovalRiskTest {

    private fun riskOf(kind: String = "执行命令 · 仅本次", command: String) =
        approvalRiskOf(kind, command)

    // ---- 常规:读类操作默认安静 ----

    @Test
    fun `读类命令与未知动作_常规档_无原因`() {
        assertEquals(ApprovalRisk.ROUTINE, riskOf(command = "ls -la").risk)
        assertEquals(ApprovalRisk.ROUTINE, riskOf(command = "cat README.md").risk)
        assertEquals(ApprovalRisk.ROUTINE, riskOf(command = "git status").risk)
        assertEquals(ApprovalRisk.ROUTINE, riskOf(command = "./gradlew test").risk)
        assertTrue(riskOf(command = "ls").reasons.isEmpty())
    }

    @Test
    fun `操作类型后缀剥离_仅本次不参与判定`() {
        // 「执行命令 · 仅本次」剥掉后缀后是普通执行词,不命中写入类
        // (「应用补丁」这类写入词本身就是要命中的动作类型,见下方写入类用例)
        assertEquals(ApprovalRisk.ROUTINE, riskOf(kind = "执行命令 · 仅本次", command = "").risk)
    }

    // ---- 高危:删除 ----

    @Test
    fun `删除类命令命中_删除文件`() {
        val r = riskOf(command = "rm -rf build")
        assertEquals(ApprovalRisk.HIGH, r.risk)
        assertTrue(r.reasons.contains("删除文件"))
        assertEquals(ApprovalRisk.HIGH, riskOf(command = "git clean -fd").risk)
        assertEquals(ApprovalRisk.HIGH, riskOf(command = "rmdir tmp").risk)
        // 大小写不敏感
        assertEquals(ApprovalRisk.HIGH, riskOf(command = "RM build").risk)
        // 中文「删除」也算
        assertTrue(riskOf(command = "删除整个目录").reasons.contains("删除文件"))
    }

    // ---- 高危:覆盖/写入 ----

    @Test
    fun `覆盖与重定向类命中_覆盖或写入`() {
        assertTrue(riskOf(command = "echo hi > out.txt").reasons.contains("覆盖或写入"))
        assertTrue(riskOf(command = "echo hi >> log").reasons.contains("覆盖或写入"))
        assertTrue(riskOf(command = "sed -i 's/a/b/' f").reasons.contains("覆盖或写入"))
        assertTrue(riskOf(command = "git reset --hard --force").reasons.contains("覆盖或写入"))
    }

    // ---- 高危:对外网络 ----

    @Test
    fun `网络外发命中_对外网络请求`() {
        assertTrue(riskOf(command = "curl https://example.com").reasons.contains("对外网络请求"))
        assertTrue(riskOf(command = "wget http://x/y").reasons.contains("对外网络请求"))
        assertTrue(riskOf(command = "git push origin main").reasons.contains("对外网络请求"))
        assertTrue(riskOf(command = "npm install left-pad").reasons.contains("对外网络请求"))
        assertTrue(riskOf(command = "scp f.txt host:").reasons.contains("对外网络请求"))
    }

    // ---- 高危:提权 ----

    @Test
    fun `提权命令与提权动作类型命中_提升权限`() {
        assertTrue(riskOf(command = "sudo apt install x").reasons.contains("提升权限"))
        assertTrue(riskOf(command = "chmod 777 /usr").reasons.contains("提升权限"))
        // 动作类型词也能单独定级(daemon 的 permissions 动作,命令可能为空)
        val r = riskOf(kind = "提升权限 · 仅本次", command = "")
        assertEquals(ApprovalRisk.HIGH, r.risk)
        assertEquals(listOf("提升权限"), r.reasons)
    }

    // ---- 多原因:固定顺序 + 去重 ----

    @Test
    fun `多特征命中_原因按固定顺序去重`() {
        val r = riskOf(command = "sudo rm -rf / && curl http://x")
        assertEquals(
            listOf("删除文件", "对外网络请求", "提升权限"),
            r.reasons,
        )
        // 同类特征重复出现只记一次
        assertEquals(listOf("删除文件"), riskOf(command = "rm a && rm b && rmdir c").reasons)
    }

    @Test
    fun `写入类动作类型命中_覆盖或改动文件`() {
        assertEquals(
            listOf("覆盖或改动文件"),
            riskOf(kind = "写入文件 · 仅本次", command = "").reasons,
        )
        assertEquals(
            listOf("覆盖或改动文件"),
            riskOf(kind = "修改文件", command = "").reasons,
        )
    }

    // ---- 风险条视觉态:只变音量,不变可达性 ----

    @Test
    fun `高危档_警示条_原因并列`() {
        val v = approvalRiskVisualOf(riskOf(command = "rm -rf build && curl http://x"))
        assertTrue(v.alert)
        assertEquals("高危操作", v.headline)
        assertEquals("删除文件 · 对外网络请求", v.reasonsLabel)
    }

    @Test
    fun `常规档_安静行_不摆空壳原因`() {
        val v = approvalRiskVisualOf(riskOf(command = "ls -la"))
        assertEquals(false, v.alert)
        assertEquals("常规操作", v.headline)
        assertNull(v.reasonsLabel)
    }

    @Test
    fun `常规档也必须给标题词_两种档位词互不相同`() {
        val quiet = approvalRiskVisualOf(approvalRiskOf("", ""))
        val loud = approvalRiskVisualOf(approvalRiskOf("", "sudo rm -rf /"))
        assertTrue(quiet.headline.isNotBlank())
        assertTrue(loud.headline.isNotBlank())
        assertTrue(quiet.headline != loud.headline)
    }
}
