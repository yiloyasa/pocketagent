package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.PaHostLink
import dev.pocketagent.app.ui.component.hostStatusGlyph
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 形通道映射的纯函数单测(状态徽章/主机胶囊/审批弹窗共用的第三语义通道)。
 *
 * 【红线】语义绝不单靠颜色:四词状态、主机连接三态、审批请求状态各有
 * 词 + 色 + 形三条通道;本组钉死"形"的映射 —— 色弱用户扫一眼也要分得清。
 */
class StatusGlyphMappingTest {

    // ---- 四词状态 → 形(空心环/实心点/对勾/叹号) ----

    @Test
    fun `四词状态各得一形_映射钉死`() {
        assertEquals(PaStatusGlyph.RING, statusGlyphOf(EditorialStatus.WAITING_FOR_YOU))
        assertEquals(PaStatusGlyph.DOT, statusGlyphOf(EditorialStatus.RUNNING))
        assertEquals(PaStatusGlyph.CHECK, statusGlyphOf(EditorialStatus.COMPLETED))
        assertEquals(PaStatusGlyph.EXCLAMATION, statusGlyphOf(EditorialStatus.NOT_COMPLETED))
    }

    @Test
    fun `四词状态四形互不相同_色弱不靠红绿也能分档`() {
        val glyphs = EditorialStatus.values().map { statusGlyphOf(it) }
        assertEquals(4, glyphs.toSet().size)
        // 完成与否不依赖红绿:收尾两态是对勾 vs 叹号,形已分开
        assertTrue(statusGlyphOf(EditorialStatus.COMPLETED) != statusGlyphOf(EditorialStatus.NOT_COMPLETED))
    }

    // ---- 主机连接三态 → 形(与四词同一套字形) ----

    @Test
    fun `主机三态各得一形_离线空心环绝不冒充连通`() {
        assertEquals(PaStatusGlyph.CHECK, hostStatusGlyph(PaHostLink.CONNECTED))
        assertEquals(PaStatusGlyph.DOT, hostStatusGlyph(PaHostLink.CONNECTING))
        assertEquals(PaStatusGlyph.RING, hostStatusGlyph(PaHostLink.OFFLINE))
        assertEquals(3, PaHostLink.values().map { hostStatusGlyph(it) }.toSet().size)
    }

    // ---- 审批请求状态 → 形(弹窗状态行第三通道) ----

    @Test
    fun `审批状态全相位都有形映射`() {
        assertEquals(PaStatusGlyph.RING, approvalPhaseGlyph(ApprovalPhase.RECEIVED))
        assertEquals(PaStatusGlyph.RING, approvalPhaseGlyph(ApprovalPhase.PENDING))
        assertEquals(PaStatusGlyph.DOT, approvalPhaseGlyph(ApprovalPhase.SUBMITTING))
        assertEquals(PaStatusGlyph.DOT, approvalPhaseGlyph(ApprovalPhase.CONFIRMED))
        assertEquals(PaStatusGlyph.CHECK, approvalPhaseGlyph(ApprovalPhase.ALLOWED))
        assertEquals(PaStatusGlyph.EXCLAMATION, approvalPhaseGlyph(ApprovalPhase.DENIED))
        assertEquals(PaStatusGlyph.EXCLAMATION, approvalPhaseGlyph(ApprovalPhase.UNAVAILABLE))
    }

    @Test
    fun `审批终态两形互异_已允许与已拒绝不靠红绿区分`() {
        assertTrue(approvalPhaseGlyph(ApprovalPhase.ALLOWED) != approvalPhaseGlyph(ApprovalPhase.DENIED))
    }

    @Test
    fun `同义状态同形_等待你处理与待决定共用空心环`() {
        // 与四词状态同一套形:同义必同形,全 app 只有一套形状语言
        assertEquals(
            statusGlyphOf(EditorialStatus.WAITING_FOR_YOU),
            approvalPhaseGlyph(ApprovalPhase.PENDING),
        )
    }
}
