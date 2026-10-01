package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.RuntimePolicy
import pocketagent.v1.RuntimePolicyResponse

/**
 * 直控开关状态词与禁用原因(R1-C:状态词辅助不只靠色;禁用要说清"为什么")的钉子。
 *
 * 【与 ControlDirectSwitchLogicTest 的分工】那份钉"能不能点/位置在哪"(呈现态),
 * 这份钉"词怎么说/原因怎么讲"—— 三种禁用(没读到/读不懂/提交中)对应的下一步
 * 动作完全不同,词与原因必须与禁用判定同源,永不互相矛盾。
 */
class DirectSwitchStatusWordTest {

    private fun item(current: String): ControlItemRow =
        ControlItemRow(
            domainId = CONTROL_DOMAIN_COLLAB,
            id = COLLAB_MASTER_SWITCH_ID,
            label = "协同总开关",
            current = current,
            kind = "toggle",
            hint = "",
            action = ControlItemAction.COLLAB_TOGGLE,
        )

    private fun budget(passthrough: Boolean): RuntimePolicyResponse =
        RuntimePolicyResponse.newBuilder()
            .setPolicy(RuntimePolicy.newBuilder().setUnknownRequestPassthrough(passthrough).build())
            .build()

    // ---- 协同总开关:状态词 -------------------------------------------------------

    @Test
    fun `协同开关词 项缺席为空串(开关不摆)`() {
        assertEquals("", collabMasterSwitchWord(null, busy = false))
    }

    @Test
    fun `协同开关词 开关与提交中_现值认不出给未识别`() {
        assertEquals("开", collabMasterSwitchWord(item("开"), busy = false))
        assertEquals("关", collabMasterSwitchWord(item("关"), busy = false))
        assertEquals("提交中", collabMasterSwitchWord(item("开"), busy = true))
        assertEquals("未识别", collabMasterSwitchWord(item("也许"), busy = false))
    }

    // ---- 协同总开关:禁用原因 ------------------------------------------------------

    @Test
    fun `协同开关原因 可点时为null_提交中与未识别各有其词`() {
        assertNull(collabMasterSwitchDisabledReason(item("开"), busy = false))
        assertEquals(
            "正在提交上一项修改,稍候自动恢复",
            collabMasterSwitchDisabledReason(item("开"), busy = true),
        )
        val reason = collabMasterSwitchDisabledReason(item("也许"), busy = false)
        assertTrue("原因必须带出原文现值,用户才知道是哪台电脑说了什么", reason!!.contains("也许"))
    }

    @Test
    fun `协同开关原因 项缺席为null(不渲染也无需解释)`() {
        assertNull(collabMasterSwitchDisabledReason(null, busy = false))
    }

    // ---- B 开关:状态词与禁用原因 ---------------------------------------------------

    @Test
    fun `B开关词 快照缺席是未读取_绝不给假关`() {
        assertEquals("未读取", unknownPassthroughSwitchWord(null, busy = false))
        assertEquals("开", unknownPassthroughSwitchWord(budget(true), busy = false))
        assertEquals("关", unknownPassthroughSwitchWord(budget(false), busy = false))
        assertEquals("提交中", unknownPassthroughSwitchWord(budget(true), busy = true))
    }

    @Test
    fun `B开关原因 快照在手为null_缺席指向连接主机`() {
        assertNull(unknownPassthroughSwitchDisabledReason(budget(true), busy = false))
        assertEquals(
            "正在读取电脑端状态",
            unknownPassthroughSwitchDisabledReason(null, busy = true),
        )
        assertEquals(
            "尚未读到电脑端状态,连接主机后可用",
            unknownPassthroughSwitchDisabledReason(null, busy = false),
        )
    }
}
