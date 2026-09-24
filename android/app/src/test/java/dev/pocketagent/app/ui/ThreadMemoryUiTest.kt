package dev.pocketagent.app.ui

import io.grpc.Status
import io.grpc.StatusRuntimeException
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ListThreadMemoriesResponse
import pocketagent.v1.RuntimeContractReport
import pocketagent.v1.RuntimeStateResponse
import pocketagent.v1.ThreadMemoryRow

/**
 * 内存管理(按会话的记忆清单/清除)数据层映射的钉子。
 *
 * 【为什么必须钉】清单从 daemon 到手机隔了两层(proto → UI 模型),最易犯的还是
 * 静默造值:loaded=false 的行值是盘上落盘那一刻的快照,不标注就会被当成实时值;
 * 从未落盘的行(last_checkpoint=0)造一个时刻出来就是编造。另一条硬红线(AUDIT
 * §3.3):降级陈述只说版本事实,不得出现「未知/错误」。
 * 乐观更新的合计算式也只有一份(threadTotalsFrom),行没了合计必须跟着少。
 */
class ThreadMemoryUiTest {

    private fun row(
        id: String,
        entries: Int,
        bytes: Long,
        seq: Long = 1,
        atMs: Long = 0,
        loaded: Boolean = true,
    ) = ThreadMemoryRow.newBuilder()
        .setThreadId(id)
        .setEntries(entries)
        .setBytes(bytes)
        .setSeq(seq)
        .setLastCheckpointUnixMs(atMs)
        .setLoaded(loaded)
        .build()

    // ---- mapThreadMemories ------------------------------------------------------

    @Test
    fun `清单逐字段映射且保持daemon的字节降序`() {
        val resp = ListThreadMemoriesResponse.newBuilder()
            .addRows(row("th-big", 1, 8, 1, atMs = 1_700_000_000_000))
            .addRows(row("th-small", 2, 4, 2))
            .setTotalThreads(2).setTotalEntries(3).setTotalBytes(12)
            .build()

        val rows = mapThreadMemories(resp)
        assertEquals(listOf("th-big", "th-small"), rows.map { it.threadId })
        assertEquals(1, rows[0].entries)
        assertEquals(8L, rows[0].bytes)
        assertEquals(1L, rows[0].seq)
        assertTrue(rows[0].loaded)
        assertTrue(rows[0].lastCheckpointLabel.isNotEmpty()) // 时刻 >0 必须带出人话
        assertEquals("", rows[1].lastCheckpointLabel) // 0 = 从未落盘,不造时刻
        assertTrue(rows[1].loaded)
    }

    @Test
    fun `未装载的行如实带出标记`() {
        val rows = mapThreadMemories(
            ListThreadMemoriesResponse.newBuilder()
                .addRows(row("th-disk", 3, 9, 5, loaded = false))
                .build()
        )
        assertFalse(rows[0].loaded)
        assertTrue(threadMemoryLine(rows[0]).contains("未装载"))
    }

    @Test
    fun `null响应映射为空清单`() {
        assertEquals(emptyList<ThreadMemoryRowUi>(), mapThreadMemories(null))
    }

    // ---- threadMemoryLine / 合计 ------------------------------------------------

    @Test
    fun `会话行人话含条数_大小_序号与落盘时刻`() {
        val line = threadMemoryLine(
            ThreadMemoryRowUi("t", 3, 2048L * 1024, 7, "刚刚", true)
        )
        assertTrue(line.contains("3 条"))
        assertTrue(line.contains("2 MB"))
        assertTrue(line.contains("序号到 7"))
        assertTrue(line.contains("落盘 刚刚"))
        assertFalse(line.contains("未装载"))
    }

    @Test
    fun `乐观合计等于各行求和且清除后同步少`() {
        val rows = listOf(
            ThreadMemoryRowUi("a", 2, 4, 2, "", true),
            ThreadMemoryRowUi("b", 1, 8, 1, "", true),
        )
        assertEquals(ThreadMemoryTotalsUi(2, 3, 12), threadTotalsFrom(rows))

        val after = threadRowsWithout(rows, "a")
        assertEquals(listOf("b"), after.map { it.threadId })
        assertEquals(ThreadMemoryTotalsUi(1, 1, 8), threadTotalsFrom(after))

        // 回滚 = 换回原引用:原列表从未被改动,这是"忠实还原"的结构保证。
        assertEquals(2, rows.size)
    }

    @Test
    fun `合计行人话`() {
        assertEquals("2 会话 · 3 条 · 12 B", threadTotalsLine(ThreadMemoryTotalsUi(2, 3, 12)))
    }

    // ---- 形态人话 ----------------------------------------------------------------

    @Test
    fun `记忆形态人话_三种申报与值域外照抄`() {
        assertTrue(memoryModelExplanation("daemon").contains("数据目录"))
        assertTrue(memoryModelExplanation("daemon").contains("不影响电脑端的其他数据"))
        assertTrue(memoryModelExplanation("hybrid").contains("原生程序保存对话原文"))
        assertTrue(memoryModelExplanation("hybrid").contains("不受影响"))
        assertTrue(memoryModelExplanation("native").contains("没有可清的东西"))
        assertEquals("weird-model", memoryModelExplanation("weird-model"))
    }

    @Test
    fun `分层标签与人话_值域外照抄_次序钉死`() {
        assertEquals("工作记忆", memoryKindLabel("working"))
        assertEquals("会话记忆", memoryKindLabel("conversation"))
        assertEquals("摘要", memoryKindLabel("summary"))
        assertEquals("产物与索引", memoryKindLabel("artifact"))
        assertEquals("mystery", memoryKindLabel("mystery"))

        val kinds = memoryKindsList(
            mapOf("summary" to 2, "artifact" to 1, "working" to 5, "conversation" to 30, "zeta" to 9)
        )
        // 钉死次序:working/conversation/summary/artifact 在前,值域外排最后。
        assertEquals(
            listOf("working", "conversation", "summary", "artifact", "zeta"),
            kinds.map { it.kind },
        )
        assertEquals(5, kinds[0].count)
    }

    @Test
    fun `memoryModelOf从总览找申报_缺席与空白都返回null`() {
        val resp = RuntimeStateResponse.newBuilder()
            .addContracts(
                RuntimeContractReport.newBuilder().setAgentId("echo").setMemoryModel("hybrid")
            )
            .addContracts(RuntimeContractReport.newBuilder().setAgentId("bare").setMemoryModel(""))
            .build()

        assertEquals("hybrid", memoryModelOf(resp, "echo"))
        assertNull(memoryModelOf(resp, "bare")) // 空白申报 = 没申报,不当成""透出
        assertNull(memoryModelOf(resp, "nosuch"))
        assertNull(memoryModelOf(null, "echo"))
    }

    // ---- 降级判定 ----------------------------------------------------------------

    @Test
    fun `UNIMPLEMENTED判为版本未提供_其余失败不混同`() {
        val unimplemented = StatusRuntimeException(Status.UNIMPLEMENTED.withDescription("no such rpc"))
        assertTrue(memoryMgmtUnsupported(unimplemented))

        val notFound = StatusRuntimeException(Status.NOT_FOUND.withDescription("no thread"))
        assertFalse(memoryMgmtUnsupported(notFound))

        assertFalse(memoryMgmtUnsupported(IllegalStateException("boom")))
    }

    // ---- 记忆入口锚点 ------------------------------------------------------------

    @Test
    fun `记忆用量行只在daemon给了memory块时带入口锚点`() {
        val provided = memoryUsageRow(
            pocketagent.v1.AgentMemoryUsage.newBuilder().setThreads(1).setItems(2).setBytes(3).build()
        )
        assertEquals("memory", provided.key)

        assertNull(memoryUsageRow(null).key) // 缺席 = 无入口,不摆假入口
    }
}
