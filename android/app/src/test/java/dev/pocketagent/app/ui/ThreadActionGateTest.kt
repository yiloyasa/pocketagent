package dev.pocketagent.app.ui

// ThreadActionGateTest —— 会话级动作能力位门控的 JVM 单测。
//
// 【背景】daemon 2026-09-25 能力位对齐(基线 511269d):GetHostInfo.capabilities
// 按 agent 逐个宣告 compact/fork/rollback/review/export_html/steer:<agentId>
// (见 daemon/internal/server/host.go,与 queue_clear:<id> 同族)。手机端这些
// 功能入口此前无位可查,只能按 agent 名单硬编码或全量显示 —— dsh 会话里
// "分叉/回滚"点了必败。本轮起能力位是唯一门控数据源:位在才亮入口,缺位=
// 隐藏,不显示点了必败的假入口(与 QueueClearGateTest 的 queue_clear:pi 同口径)。

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ThreadActionGateTest {

    /** 模拟 daemon 四家 agent 的能力位全集(按 host.go 的接口事实摆位)。 */
    private val capsAll = listOf(
        "compact:codex", "compact:pi", "compact:zcode",
        "fork:codex", "rollback:codex", "review:codex",
        "export_html:pi",
        "image_input:codex", "image_input:pi", "image_input:zcode",
        "steer:codex", "steer:pi",
        "queue_clear:pi", "runtime.v1",
    )

    // ---- 压缩上下文(compact:<agentId>)----

    @Test
    fun `压缩入口_三家有位dsh无位`() {
        for (id in listOf("codex", "pi", "zcode")) {
            assertTrue(compactActionOn(id, capsAll))
        }
        // dsh 无压缩实现(daemon 实测不宣告 compact:dsh):入口隐藏
        assertFalse(compactActionOn("dsh", capsAll))
    }

    @Test
    fun `压缩入口_旧daemon无位不显示假入口`() {
        assertFalse(compactActionOn("codex", emptyList()))
        assertFalse(compactActionOn("pi", listOf("runtime.v1", "queue_clear:pi")))
    }

    @Test
    fun `压缩入口_别家的位不开我家的门`() {
        assertFalse(compactActionOn("codex", listOf("compact:pi")))
        assertFalse(compactActionOn("pi", listOf("compact:zcode")))
    }

    // ---- 分叉会话(fork:<agentId>,现仅 codex)----

    @Test
    fun `分叉入口_仅codex有位`() {
        assertTrue(forkActionOn("codex", capsAll))
        for (id in listOf("pi", "zcode", "dsh")) {
            assertFalse(forkActionOn(id, capsAll))
        }
        assertFalse(forkActionOn("codex", emptyList()))
    }

    // ---- 回滚(rollback:<agentId>,现仅 codex)----

    @Test
    fun `回滚入口_仅codex有位`() {
        assertTrue(rollbackActionOn("codex", capsAll))
        for (id in listOf("pi", "zcode", "dsh")) {
            assertFalse(rollbackActionOn(id, capsAll))
        }
        assertFalse(rollbackActionOn("codex", emptyList()))
    }

    // ---- 通用口径:agentId 绑定 + 大小写/空白不敏感 ----

    @Test
    fun `agentId为空或空白_一律不开入口`() {
        // 会话信息还没拉到(GetThread 在路上)时绑定不到具体会话,绝不开入口
        assertFalse(compactActionOn("", capsAll))
        assertFalse(compactActionOn("  ", capsAll))
        assertFalse(compactActionOn(null, capsAll))
        assertFalse(forkActionOn(null, capsAll))
        assertFalse(rollbackActionOn(null, capsAll))
    }

    @Test
    fun `位与agentId大小写空白不敏感`() {
        assertTrue(compactActionOn(" Pi ", listOf(" COMPACT:pi ")))
        assertTrue(forkActionOn("CODEX", listOf("fork:codex")))
        assertTrue(rollbackActionOn(" codex ", listOf(" ROLLBACK:CODEX ")))
    }

    // ---- 附件/图片入口(image_input:<agentId> 缺位隐藏门)----

    @Test
    fun `附件图片入口_dsh无位隐藏带位agent不动`() {
        // daemon 按 agent.ImageInputter 断言逐家宣告 image_input(host.go:151):
        // codex/pi/zcode 带位,dsh 无位(发图如实报错)—— 缺位会话隐藏入口,
        // 免得用户选完图必吃失败壳;带位 agent 行为不变(入口照常)。
        assertTrue(attachmentEntryHiddenFor("dsh", capsAll))
        assertTrue(attachmentEntryHiddenFor(" DSH ", capsAll))
        for (id in listOf("codex", "pi", "zcode")) {
            assertFalse(attachmentEntryHiddenFor(id, capsAll))
        }
    }

    @Test
    fun `附件图片入口_旧daemon无位或agentId未绑定一律隐藏`() {
        // 旧 daemon 不宣告 image_input 位:全量隐藏(与 compact 缺位同口径,
        // 不摆点了必败的假按钮);agentId 空/空白 = 会话信息未拉到、绑定不到
        // 具体会话,按 agentCapableOf 口径视为缺位 → 隐藏。
        assertTrue(attachmentEntryHiddenFor("pi", emptyList()))
        assertTrue(attachmentEntryHiddenFor("codex", listOf("runtime.v1", "queue_clear:pi")))
        assertTrue(attachmentEntryHiddenFor(null, capsAll))
        assertTrue(attachmentEntryHiddenFor("", capsAll))
        assertTrue(attachmentEntryHiddenFor("  ", capsAll))
    }

    @Test
    fun `附件图片入口_位与agentId大小写空白不敏感`() {
        assertFalse(attachmentEntryHiddenFor(" Pi ", listOf(" IMAGE_INPUT:pi ")))
        // 别家的位不开我家的门
        assertTrue(attachmentEntryHiddenFor("pi", listOf("image_input:zcode")))
    }

    // ---- 运行中发送回执(steer:<agentId> 两态文案)----

    @Test
    fun `运行中发送回执_有steer位报已插入无位报已排队`() {
        assertEquals("已插入运行中任务", steerSubmitNotice(true))
        assertEquals("已排队", steerSubmitNotice(false))
    }

    @Test
    fun `steer位按agent绑定判定`() {
        // codex 原生 turn/steer、pi 原生插队;zcode/dsh 由 daemon 排队兜底
        assertTrue(agentCapableOf("steer", "pi", capsAll))
        assertTrue(agentCapableOf("steer", "codex", capsAll))
        assertFalse(agentCapableOf("steer", "zcode", capsAll))
        assertFalse(agentCapableOf("steer", "zcode", emptyList()))
    }

    @Test
    fun `回执文案不带禁词`() {
        for (s in listOf(steerSubmitNotice(true), steerSubmitNotice(false))) {
            assertFalse(s.contains("未知"))
            assertFalse(s.contains("错误"))
        }
    }
}
