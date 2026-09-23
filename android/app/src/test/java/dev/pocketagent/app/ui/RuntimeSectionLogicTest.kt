package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.CollabPolicy
import pocketagent.v1.RuntimePolicy

/**
 * 「运行时」区段里**唯一有判据的部分**的单测。
 *
 * 【为什么区段本体不测、只测这三样】Compose 的排版由编译器保证,而下面这几件事
 * 编译器一句话都不说:
 *  - 某个 key 映射到了**错误的字段**(表现是"改 A 结果 B 变了",界面上完全看不出来);
 *  - `null` 被写成 0 而不是清掉 presence(表现是"恢复默认"变成了"钉死为 0");
 *  - 把协同策略的成员上限写到预算对象上(它俩是两套独立的落盘与 CAS)。
 * 这三种都是静默错误,只有单测能拦。
 */
class RuntimeSectionLogicTest {

    @Test
    fun `预算编辑只动点中的那一项`() {
        val base = RuntimePolicy.newBuilder()
            .setGlobal(
                pocketagent.v1.RuntimeLimits.newBuilder()
                    .setRamWorkingBytes(8L shl 20)
                    .setMaxParallelTurns(3)
            )
            .build()
        val next = base.toBuilder().also { applyBudgetEdit(it, "disk_cache_bytes", 16L shl 20) }.build()

        assertEquals(16L shl 20, next.global.diskCacheBytes)
        // 其余项原样带回 —— 这正是"提交完整策略"里最容易丢的部分。
        assertEquals(8L shl 20, next.global.ramWorkingBytes)
        assertEquals(3, next.global.maxParallelTurns)
        assertTrue(next.global.hasDiskCacheBytes())
    }

    @Test
    fun `预算编辑传 null 是恢复未设置而不是写 0`() {
        val base = RuntimePolicy.newBuilder()
            .setGlobal(pocketagent.v1.RuntimeLimits.newBuilder().setMaxParallelTurns(3))
            .build()
        val next = base.toBuilder().also { applyBudgetEdit(it, "max_parallel_turns", null) }.build()

        // presence 被清掉 = 沿用上一级 / 内置基线;写成 0 就是"明令禁止并行"了。
        assertFalse(next.global.hasMaxParallelTurns())
    }

    @Test
    fun `显式 0 是设置而不是未设置`() {
        val next = RuntimePolicy.newBuilder()
            .also { applyBudgetEdit(it, "max_parallel_turns", 0L) }
            .build()

        assertTrue(next.global.hasMaxParallelTurns())
        assertEquals(0, next.global.maxParallelTurns)
    }

    @Test
    fun `六个字节预算键各映射到自己的那个字段`() {
        // 只断言"恰好设置了一项"是不够的:把 cache 的值写进 artifact 也满足"一项"。
        // 这里连**是哪一项**一起钉住 —— 映射错位在界面上完全看不出来(改 A 结果 B 变了)。
        val keys = linkedMapOf(
            "ram_working_bytes" to 0,
            "ram_event_buffer_bytes" to 1,
            "ram_session_index_bytes" to 2,
            "disk_conversation_bytes" to 3,
            "disk_cache_bytes" to 4,
            "disk_artifact_bytes" to 5,
        )
        keys.forEach { (key, expectedIndex) ->
            val b = RuntimePolicy.newBuilder()
            applyBudgetEdit(b, key, 1L shl 20)
            val g = b.build().global
            val flags = listOf(
                g.hasRamWorkingBytes(),
                g.hasRamEventBufferBytes(),
                g.hasRamSessionIndexBytes(),
                g.hasDiskConversationBytes(),
                g.hasDiskCacheBytes(),
                g.hasDiskArtifactBytes(),
            )
            assertEquals("key=$key 应恰好设置一项", 1, flags.count { it })
            assertEquals("key=$key 写错了字段", expectedIndex, flags.indexOf(true))
        }
    }

    @Test
    fun `成员上限写在协同策略上而不是预算上`() {
        val b = CollabPolicy.newBuilder()
        applyCollabEdit(b, "max_parallel_members", 2L)
        val p = b.build()

        assertTrue(p.hasMaxParallelMembers())
        assertEquals(2, p.maxParallelMembers)
    }

    @Test
    fun `成员上限恢复未设置`() {
        val b = CollabPolicy.newBuilder().setMaxParallelMembers(2)
        applyCollabEdit(b, "max_parallel_members", null)

        assertFalse(b.build().hasMaxParallelMembers())
    }

    @Test
    fun `单位文案回填只取数字`() {
        assertEquals("8", currentNumeric("8 MB"))
        assertEquals("3", currentNumeric("3"))
        // "不限"没有数字可回填 —— 回填成 0 会把"不限"变成"一家都不许派"。
        assertEquals("", currentNumeric("不限"))
    }

    @Test
    fun `编辑框打开时光标在末尾而不是开头`() {
        // 真机实测踩到(W11c 走查):光标落在位置 0 时,直接输入会把新值插到旧值**前面** ——
        // 预填 "3"、敲 "4" 得到 "43",用户以为设了 4、实际设了 43,且预算被改错没有任何提示。
        val v = budgetInputValue("3")
        assertEquals("3", v.text)
        assertEquals(1, v.selection.start)
        assertEquals(1, v.selection.end)

        // 带单位的现值同理:回填 "8" 且光标在 "8" 之后。
        val bytes = budgetInputValue("8 MB")
        assertEquals("8", bytes.text)
        assertEquals(1, bytes.selection.end)
    }

    @Test
    fun `分块标签已知的译成人话未知的原样返回`() {
        assertEquals("启动恢复", sectionLabel(RuntimeControlState.SECTION_HOST))
        assertEquals("预算", sectionLabel(RuntimeControlState.SECTION_BUDGET))
        assertEquals("某个新块", sectionLabel("某个新块"))
    }
}
