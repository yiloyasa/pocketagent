package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.CollabPolicy
import pocketagent.v1.ControlItem
import pocketagent.v1.RuntimePolicy
import pocketagent.v1.RuntimePolicyResponse

/**
 * 控制中心直控开关(协同域头部总开关 + 透析域头部 B 开关)逻辑层的钉子。
 *
 * 【为什么必须钉】直控开关摆在域行头部,是用户不进展开体就碰到的写入口:
 *  - 现值认不出时猜布尔 = 把 daemon 没说的状态替它说了(假账);
 *  - 快照缺席时启用开关 = "点了先把旧世界拉下来再写",可能写反;
 *  - B 开关的数据源接错(接成协同策略)= 改 A 动 B,界面看不出。
 * 这些都在静默中发生,只有单测能拦。
 */
class ControlDirectSwitchLogicTest {

    /** 对照 daemon/internal/server/control_plane.go 的 collab 域真实取值形态。 */
    private fun collabDomain(): ControlDomainRow {
        val domain = ControlDomain.newBuilder()
            .setId("collab").setTitle("多 Agent 协同").setWritable(true).setStatusCode(0)
            .setSummary("协同已开启:委派与扇出可用")
            .addItems(
                ControlItem.newBuilder()
                    .setId("collab.collaboration_enabled").setLabel("协同总开关")
                    .setCurrent("开").setKind("toggle")
                    .setHint("关掉后 Agent 之间不能再互相派活,各自会话不受影响")
            )
            .addItems(
                ControlItem.newBuilder()
                    .setId("collab.native_delegate_enabled").setLabel("委派子任务")
                    .setCurrent("关").setKind("toggle")
            )
            .build()
        return controlDomainRows(
            pocketagent.v1.ControlPlane.newBuilder()
                .addDomains(domain)
                .build()
        ).single()
    }

    // ---- 协同总开关:项提取 -----------------------------------------------------

    @Test
    fun `collab domain yields master switch item`() {
        val item = collabMasterSwitchItem(collabDomain())
        assertEquals(COLLAB_MASTER_SWITCH_ID, item?.id)
        assertEquals("协同总开关", item?.label)
    }

    @Test
    fun `master switch item is null without plane or item`() {
        assertNull(collabMasterSwitchItem(null))
        val emptyDomain = collabDomain().copy(items = collabDomain().items.filterNot { it.id == COLLAB_MASTER_SWITCH_ID })
        assertNull("缺总开关项时不应摆直控入口", collabMasterSwitchItem(emptyDomain))
        val otherDomain = collabDomain().copy(id = CONTROL_DOMAIN_BUDGET)
        assertNull("非 collab 域不摆协同总开关", collabMasterSwitchItem(otherDomain))
    }

    // ---- 协同总开关:呈现态 -----------------------------------------------------

    @Test
    fun `master switch follows daemon truth`() {
        val state = collabMasterSwitchState(collabMasterSwitchItem(collabDomain()), busy = false)
        assertTrue("daemon 说'开',开关应为开", state.checked)
        assertTrue("现值可辨识且空闲,应可点", state.enabled)
    }

    @Test
    fun `master switch disabled while busy`() {
        val state = collabMasterSwitchState(collabMasterSwitchItem(collabDomain()), busy = true)
        assertFalse("提交中必须禁用,防连点叠加两次 Set", state.enabled)
    }

    @Test
    fun `master switch disabled on unreadable current`() {
        val item = collabMasterSwitchItem(collabDomain())!!.copy(current = "也许")
        val state = collabMasterSwitchState(item, busy = false)
        assertFalse("词表外现值不得猜成布尔可点", state.enabled)
        assertFalse(state.checked)
    }

    @Test
    fun `master switch disabled when item missing`() {
        val state = collabMasterSwitchState(null, busy = false)
        assertFalse(state.enabled)
        assertFalse(state.checked)
    }

    // ---- B 开关:呈现态 ----------------------------------------------------------

    private fun budgetResponse(passthrough: Boolean?): RuntimePolicyResponse {
        val b = RuntimePolicyResponse.newBuilder()
        if (passthrough != null) {
            b.policy = RuntimePolicy.newBuilder().setUnknownRequestPassthrough(passthrough).build()
        }
        return b.build()
    }

    @Test
    fun `unknown switch reads runtime policy field`() {
        val on = unknownPassthroughSwitchState(budgetResponse(true), busy = false)
        assertTrue(on.checked)
        assertTrue(on.enabled)
        val off = unknownPassthroughSwitchState(budgetResponse(false), busy = false)
        assertFalse("显式 false(用户选过关)显示为关", off.checked)
        assertTrue(off.enabled)
    }

    @Test
    fun `unknown switch disabled without budget snapshot`() {
        val state = unknownPassthroughSwitchState(null, busy = false)
        assertFalse("预算快照缺席(没读到)必须禁用,不显示假'关'可点", state.enabled)
    }

    @Test
    fun `unknown switch disabled while busy`() {
        val state = unknownPassthroughSwitchState(budgetResponse(true), busy = true)
        assertFalse(state.enabled)
        assertTrue("禁用不等于改值:位置仍照 daemon 真值", state.checked)
    }

    // ---- 写映射复用既有纯函数(回归钉)---------------------------------------------

    @Test
    fun `write mapping stays on unknown_request_passthrough`() {
        val b = RuntimePolicy.newBuilder()
        applyUnknownRequestPassthrough(b, true)
        assertTrue(b.build().unknownRequestPassthrough)
        // presence 纪律:关也要显式 set(false),不能 clear 回缺席。
        val b2 = RuntimePolicy.newBuilder()
        applyUnknownRequestPassthrough(b2, false)
        assertTrue(b2.build().hasUnknownRequestPassthrough())
        assertFalse(b2.build().unknownRequestPassthrough)
    }

    @Test
    fun `collab toggle mapping still targets collaboration_enabled`() {
        val b = CollabPolicy.newBuilder()
        assertTrue(applyControlToggle(b, COLLAB_MASTER_SWITCH_ID, false))
        val p = b.build()
        assertFalse(p.collaborationEnabled)
        assertFalse("只许动总开关,委派/扇出不受牵连", p.hasNativeDelegateEnabled())
        assertFalse(p.hasFanoutEnabled())
    }
}
