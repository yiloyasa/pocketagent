package dev.pocketagent.app.ui.prompt

// PromptOptimizePreviewStateTest —— 优化预览纯逻辑的 JVM 单测(P4 Task 2/3),逐条钉:
//   1) 生命周期:发起→加载(记原文/风格/修订号)、成功→候选、空候选→失败、失败→错误位
//      且原文槽位不动、层已关后的迟到结果丢弃、候选可编辑、重试清错;
//   2) revision 防覆盖三态:未修改→直接应用 / 已修改→拒绝盲应用走二选一 /
//      应用后撤销只回本次原文(撤销语义) ;
//   3) 连点 busy 闸:在途请求时不可再发起(照 doFork 先例);
//   4) 行级 diff 纯函数:全等/纯增/纯删/中段对齐/CRLF/超限退化/空串;
//   5) 扩写启发式阈值(Task 3):>2 倍 或 >500 字任一命中,空白原文不判;
//   6) 风格短标签与一句话说明(Task 3 可解释):七值齐全、互不撞车。

import dev.pocketagent.app.ui.PromptOptimizeStyle
import dev.pocketagent.app.ui.promptOptimizeStyleHint
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PromptOptimizePreviewStateTest {

    private val idle = PromptOptimizePreviewState()

    /** 发起一次请求(默认原文/风格/修订号 5)。 */
    private fun started(
        rev: Long = 5L,
        style: PromptOptimizeStyle = PromptOptimizeStyle.DEFAULT,
        original: String = "帮我修登录超时",
    ): PromptOptimizePreviewState =
        reducePromptOptimizePreview(
            idle,
            PromptOptimizePreviewIntent.Started(original, style, rev),
            currentRevision = rev,
        )

    /** 成功拿到候选(默认候选文本)。 */
    private fun withCandidate(
        rev: Long = 5L,
        candidate: String = "把登录超时问题说清楚,给出复现步骤",
        style: PromptOptimizeStyle = PromptOptimizeStyle.DEFAULT,
    ): PromptOptimizePreviewState =
        reducePromptOptimizePreview(started(rev, style), PromptOptimizePreviewIntent.Succeeded(candidate), rev)

    // ---- 1. 生命周期 ----

    @Test
    fun `发起请求进入加载并记录原文风格与修订号`() {
        val s = started(rev = 5, style = PromptOptimizeStyle.BUG_REPORT)
        assertTrue(s.loading)
        assertEquals("帮我修登录超时", s.original)
        assertEquals(PromptOptimizeStyle.BUG_REPORT, s.style)
        assertEquals(5L, s.sourceRevision)
        assertNull(s.error)
        assertFalse(s.hasCandidate)
        // 发起是全新状态:上一轮残留的候选/错误一并清掉
        val overStale = reducePromptOptimizePreview(
            withCandidate(),
            PromptOptimizePreviewIntent.Started("新的问题", PromptOptimizeStyle.CONCISE, 9),
            currentRevision = 9,
        )
        assertTrue(overStale.loading)
        assertFalse(overStale.hasCandidate)
    }

    @Test
    fun `成功落入候选并清错误`() {
        val s = reducePromptOptimizePreview(
            started().copy(error = "上次的错误"),
            PromptOptimizePreviewIntent.Succeeded("  改写后的文本  "),
            currentRevision = 5,
        )
        assertFalse(s.loading)
        assertEquals("改写后的文本", s.candidate) // 仓库层 trim 口径一致,双保险再 trim
        assertNull(s.error)
        assertTrue(s.hasCandidate)
    }

    @Test
    fun `空候选当失败绝不进候选槽位`() {
        val s = reducePromptOptimizePreview(started(), PromptOptimizePreviewIntent.Succeeded("   "), 5)
        assertFalse(s.loading)
        assertFalse(s.hasCandidate)
        assertEquals("优化结果为空,原文未被改动", s.error)
    }

    @Test
    fun `失败落错误位且原文槽位不动`() {
        val s = reducePromptOptimizePreview(started(), PromptOptimizePreviewIntent.Failed("优化超时了"), 5)
        assertFalse(s.loading)
        assertEquals("优化超时了", s.error)
        assertEquals("帮我修登录超时", s.original) // 失败不改变原文
        assertFalse(s.hasCandidate)
    }

    @Test
    fun `层已关后迟到的成功与失败都被丢弃`() {
        // 用户在请求在飞时关掉了弹层(loading = false)
        val dismissed = reducePromptOptimizePreview(started(), PromptOptimizePreviewIntent.Dismiss, 5)
        assertTrue(dismissed.isIdle)
        val lateOk = reducePromptOptimizePreview(dismissed, PromptOptimizePreviewIntent.Succeeded("迟到候选"), 5)
        assertTrue(lateOk.isIdle) // 不复活弹层
        val lateErr = reducePromptOptimizePreview(dismissed, PromptOptimizePreviewIntent.Failed("迟到失败"), 5)
        assertNull(lateErr.error)
    }

    @Test
    fun `候选可编辑且无候选时编辑被忽略`() {
        val edited = reducePromptOptimizePreview(
            withCandidate(),
            PromptOptimizePreviewIntent.EditCandidate("用户改过的候选"),
            5,
        )
        assertEquals("用户改过的候选", edited.candidate)
        // 空闲态的编辑是 no-op
        val ignored = reducePromptOptimizePreview(idle, PromptOptimizePreviewIntent.EditCandidate("x"), 5)
        assertFalse(ignored.hasCandidate)
    }

    @Test
    fun `重试清错误回到加载 空原文不能重试 在途不能重试`() {
        val failed = reducePromptOptimizePreview(started(), PromptOptimizePreviewIntent.Failed("超时"), 5)
        val retrying = reducePromptOptimizePreview(failed, PromptOptimizePreviewIntent.Retry, 5)
        assertTrue(retrying.loading)
        assertNull(retrying.error)
        assertEquals("帮我修登录超时", retrying.original) // 重试同一原文
        // 空原文 / 在途:重试 no-op
        val blankOriginal = reducePromptOptimizePreview(
            idle.copy(original = "  "), PromptOptimizePreviewIntent.Retry, 5,
        )
        assertFalse(blankOriginal.loading)
        val duringFlight = reducePromptOptimizePreview(started(), PromptOptimizePreviewIntent.Retry, 5)
        assertTrue(duringFlight.loading) // 保持一次在飞,没有叠加
    }

    // ---- 2. revision 防覆盖三态 ----

    @Test
    fun `三态之一 未修改直接应用`() {
        val s = withCandidate(rev = 5)
        assertFalse(s.inputModifiedSince(5L))
        val applied = reducePromptOptimizePreview(s, PromptOptimizePreviewIntent.Apply, currentRevision = 5L)
        assertTrue(applied.isIdle) // 应用成功,状态清空(输入框由调用方写候选)
    }

    @Test
    fun `三态之二 已修改拒绝盲应用 走覆盖或保留二选一`() {
        val s = withCandidate(rev = 5)
        assertTrue(s.inputModifiedSince(6L)) // 用户在等待期间改过输入框
        // 盲 Apply 被拒绝:状态原样保留(绝不静默覆盖新输入)
        val blind = reducePromptOptimizePreview(s, PromptOptimizePreviewIntent.Apply, currentRevision = 6L)
        assertEquals(s, blind)
        assertTrue(blind.hasCandidate)
        // 二选一之「覆盖」:显式选择后才应用
        val overwritten = reducePromptOptimizePreview(s, PromptOptimizePreviewIntent.OverwriteModified, 6L)
        assertTrue(overwritten.isIdle)
        // 二选一之「保留新文本」:等价于放弃候选关层
        val kept = reducePromptOptimizePreview(s, PromptOptimizePreviewIntent.Dismiss, 6L)
        assertTrue(kept.isIdle)
    }

    @Test
    fun `三态之三 应用后撤销只回本次原文且不复活候选`() {
        // 应用瞬间调用方捕获"应用前的输入"交 UndoBar(reduce 是纯函数,原文恢复
        // 属调用方副作用);纯侧可钉的是:应用后状态清空,撤销/再撤销不复活任何东西。
        val applied = reducePromptOptimizePreview(withCandidate(), PromptOptimizePreviewIntent.Apply, 5L)
        assertTrue(applied.isIdle)
        val undone = reducePromptOptimizePreview(applied, PromptOptimizePreviewIntent.Undo, 5L)
        assertTrue(undone.isIdle)
        assertFalse(undone.hasCandidate)
        assertNull(undone.error)
        // 覆盖路径同理:OverwriteModified 应用后,撤销同样不复活
        val overwrittenThenUndo = reducePromptOptimizePreview(
            reducePromptOptimizePreview(withCandidate(), PromptOptimizePreviewIntent.OverwriteModified, 6L),
            PromptOptimizePreviewIntent.Undo,
            6L,
        )
        assertTrue(overwrittenThenUndo.isIdle)
    }

    @Test
    fun `换风格空闲时可改在途时禁用`() {
        val restyled = reducePromptOptimizePreview(
            withCandidate(style = PromptOptimizeStyle.DEFAULT),
            PromptOptimizePreviewIntent.Restyle(PromptOptimizeStyle.TEST_PLAN),
            5,
        )
        assertEquals(PromptOptimizeStyle.TEST_PLAN, restyled.style)
        // 在途(loading)时换风格 no-op:请求带着发起时的风格跑完
        val duringFlight = reducePromptOptimizePreview(
            started(style = PromptOptimizeStyle.DEFAULT),
            PromptOptimizePreviewIntent.Restyle(PromptOptimizeStyle.TEST_PLAN),
            5,
        )
        assertEquals(PromptOptimizeStyle.DEFAULT, duringFlight.style)
    }

    // ---- 3. 连点 busy 闸 ----

    @Test
    fun `busy 闸 空输入或在途都不可再发起`() {
        // 空闲 + 非空输入:可点
        assertTrue(promptOptimizeClickAllowed(idle, "帮我修 bug"))
        // 空输入(含纯空白):不可点
        assertFalse(promptOptimizeClickAllowed(idle, ""))
        assertFalse(promptOptimizeClickAllowed(idle, "   "))
        // 在途:不可点(快速连点不会生成第二个并发请求)
        assertFalse(promptOptimizeClickAllowed(started(), "帮我修 bug"))
    }

    // ---- 4. 行级 diff 纯函数 ----

    @Test
    fun `diff 完全一致输出全 SAME`() {
        val lines = promptLineDiff("a\nb\nc", "a\nb\nc")
        assertEquals(3, lines.size)
        assertTrue(lines.all { it.kind == PromptDiffKind.SAME })
        assertEquals(listOf("a", "b", "c"), lines.map { it.text })
    }

    @Test
    fun `diff 纯新增与纯删除`() {
        val added = promptLineDiff("a", "a\nb\nc")
        assertEquals(listOf(PromptDiffKind.SAME, PromptDiffKind.ADDED, PromptDiffKind.ADDED), added.map { it.kind })
        val removed = promptLineDiff("a\nx\ny", "a")
        assertEquals(listOf(PromptDiffKind.SAME, PromptDiffKind.REMOVED, PromptDiffKind.REMOVED), removed.map { it.kind })
    }

    @Test
    fun `diff 中段替换按 LCS 对齐并保住前后缀`() {
        val lines = promptLineDiff("a\nb\nc", "a\nx\nc")
        assertEquals(
            listOf(
                PromptDiffKind.SAME to "a",
                PromptDiffKind.REMOVED to "b",
                PromptDiffKind.ADDED to "x",
                PromptDiffKind.SAME to "c",
            ),
            lines.map { it.kind to it.text },
        )
    }

    @Test
    fun `diff CRLF 按行分隔且行内容不带 CR`() {
        val lines = promptLineDiff("a\r\nb", "a\r\nb\r\nc")
        assertEquals(listOf(PromptDiffKind.SAME to "a", PromptDiffKind.SAME to "b", PromptDiffKind.ADDED to "c"), lines.map { it.kind to it.text })
    }

    @Test
    fun `diff 中段超上限退化为整段删加 不跑 DP`() {
        val old = (1..401).joinToString("\n") { "old$it" }
        val new = (1..401).joinToString("\n") { "new$it" }
        val lines = promptLineDiff(old, new)
        assertEquals(802, lines.size)
        assertTrue(lines.take(401).all { it.kind == PromptDiffKind.REMOVED })
        assertTrue(lines.drop(401).all { it.kind == PromptDiffKind.ADDED })
    }

    @Test
    fun `diff 空串边界`() {
        assertTrue(promptLineDiff("", "").isEmpty())
        assertEquals(2, promptLineDiff("", "a\nb").size)
        assertTrue(promptLineDiff("", "a\nb").all { it.kind == PromptDiffKind.ADDED })
        assertEquals(1, promptLineDiff("a", "").size)
        assertEquals(PromptDiffKind.REMOVED, promptLineDiff("a", "").single().kind)
    }

    // ---- 5. 扩写启发式阈值(Task 3) ----

    @Test
    fun `扩写 两倍阈值 超过才命中`() {
        val o = "字".repeat(100)
        assertFalse(promptCandidateOverExpanded(o, "字".repeat(200))) // 恰好 2 倍不算
        assertTrue(promptCandidateOverExpanded(o, "字".repeat(201))) // 超过 2 倍
    }

    @Test
    fun `扩写 绝对增量阈值 超过 500 字才命中`() {
        val o = "字".repeat(1000)
        assertFalse(promptCandidateOverExpanded(o, o + "字".repeat(500))) // +500 不算
        assertTrue(promptCandidateOverExpanded(o, o + "字".repeat(501))) // +501 命中
    }

    @Test
    fun `扩写 按 trim 后长度比较且空白原文不判`() {
        assertTrue(promptCandidateOverExpanded("  x  ", "x" + "字".repeat(300))) // 短原文被塞长
        assertFalse(promptCandidateOverExpanded("   ", "一大段很长很长的候选文本"))
        assertFalse(promptCandidateOverExpanded("", "候选"))
    }

    // ---- 6. 风格短标签与说明(Task 3 可解释) ----

    @Test
    fun `七风格短标签与一句话说明齐全且互异`() {
        val labels = PromptOptimizeStyle.values().map { promptOptimizeStyleShortLabel(it) }
        assertEquals(7, labels.size)
        assertEquals(7, labels.toSet().size) // 互异:芯片不会有两颗同名
        val hints = PromptOptimizeStyle.values().map { promptOptimizeStyleHint(it) }
        assertEquals(7, hints.toSet().size)
        // 口径抽查:与 daemon 模板段逐条对齐的说法(设置页同词表)
        assertEquals("默认:把意思说清楚,不扩写需求", promptOptimizeStyleHint(PromptOptimizeStyle.DEFAULT))
        assertEquals("精简:压缩成一句话", promptOptimizeStyleHint(PromptOptimizeStyle.CONCISE))
        assertEquals("实施计划:整理目标、范围、验收标准", promptOptimizeStyleHint(PromptOptimizeStyle.IMPLEMENTATION_PLAN))
        assertEquals("代码审查:整理风险、影响范围、证据要求", promptOptimizeStyleHint(PromptOptimizeStyle.CODE_REVIEW))
    }
}
