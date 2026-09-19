package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Markdown 导出正文的纯函数单测(P1-1 回归钉子)。
 *
 * 【背景】修复前导出面板"选中/展示"用 `full ?: exportable`(source),"落盘"却
 * 从内存子集 exportable 按索引取 —— 全量历史(full)加载完成后,selected 的索引
 * 语义已切到 source 上重建,还按老索引取 exportable 就会把"已选 50/50 条"导成
 * 内存里那几条,Toast 还说"已导出 50 条消息"。修复:导出与计数统一走
 * `source.filterIndexed { i,_ -> i in selected }`,条数用过滤后的实际值。
 * 本测试钉死:不管 full 加载前后,导出内容都必须与"展示/选中的那张表"一致。
 */
class ExportChatBodyTest {

    private fun item(role: String, text: String) = ChatItem(role = role, text = text)

    // ---- role 映射与格式 ----

    @Test
    fun `user映射为我_assistant映射为Agent`() {
        val body = buildExportBody(listOf(item("user", "你好"), item("assistant", "你好!有什么可以帮你?")))
        assertEquals("## 我\n你好\n\n## Agent\n你好!有什么可以帮你?\n\n", body)
    }

    @Test
    fun `非user角色一律按Agent渲染`() {
        val body = buildExportBody(listOf(item("tool", "结果")))
        assertEquals("## Agent\n结果\n\n", body)
    }

    @Test
    fun `正文原样不裁剪`() {
        val text = "第一行\n第二行\n### 甚至带 Markdown 标记"
        val body = buildExportBody(listOf(item("user", text)))
        assertTrue(body.contains(text))
    }

    // ---- full 加载前后导出内容一致性(核心回归点)----

    @Test
    fun `full加载前_内存子集全选_导出与展示一致`() {
        // 打开新会话:full 还没回来,source = exportable(内存里的 3 条),
        // selected = source.indices(全选)→ 导出必须恰好是这 3 条。
        val exportable = listOf(
            item("user", "内存1"),
            item("assistant", "内存2"),
            item("user", "内存3"),
        )
        val source = exportable
        val selected = source.indices.toSet()
        val chosen = source.filterIndexed { i, _ -> i in selected }
        assertEquals(3, chosen.size)
        assertEquals("内存1", chosen[0].text)
        assertEquals("内存3", chosen[2].text)
        assertEquals(buildExportBody(chosen), buildExportBody(exportable.filterIndexed { i, _ -> i in selected }))
    }

    @Test
    fun `full加载后_导出取全量source而不是内存子集`() {
        // 打开旧会话:full(50 条)到达后 source 切成全量、selected 以 source 重建
        // (全选)。修复前:仍从内存子集(3 条)按 source 的索引取 → 内容与
        // 计数错位;修复后:导出的是 source 里选中的条目,条数 = 实际过滤条数。
        val full = List(50) { i ->
            if (i % 2 == 0) item("user", "历史消息-$i") else item("assistant", "回复-$i")
        }
        val exportable = full.take(3) // 内存里只有最近几轮
        val source = full
        val selected = source.indices.toSet()

        val chosen = source.filterIndexed { i, _ -> i in selected }
        assertEquals(50, chosen.size) // 条数取实际值,不是 selected.size 之外的口径
        assertEquals("历史消息-48", chosen[48].text)
        assertEquals("回复-49", chosen[49].text)
        // 钉死错位场景:修复前的写法(从 exportable 取)会得出完全不同的内容
        val wrongWay = exportable.filterIndexed { i, _ -> i in selected }
        assertTrue(wrongWay.size != chosen.size)
    }

    @Test
    fun `部分选中_只导出选中条目且顺序保持`() {
        val full = List(10) { i -> item(if (i % 2 == 0) "user" else "assistant", "消息-$i") }
        val source = full
        val selected = setOf(1, 4, 7)
        val chosen = source.filterIndexed { i, _ -> i in selected }
        assertEquals(listOf("消息-1", "消息-4", "消息-7"), chosen.map { it.text })
        val body = buildExportBody(chosen)
        assertTrue(body.contains("## Agent\n消息-1"))
        assertTrue(body.contains("## 我\n消息-4"))
        assertTrue(body.contains("## Agent\n消息-7"))
        assertEquals(3, chosen.size)
    }

    @Test
    fun `空选择_导出为空(调用方已有早退守卫)`() {
        val source = List(5) { i -> item("user", "消息-$i") }
        val chosen = source.filterIndexed { i, _ -> i in emptySet<Int>() }
        assertEquals(0, chosen.size)
        assertEquals("", buildExportBody(chosen))
    }
}
