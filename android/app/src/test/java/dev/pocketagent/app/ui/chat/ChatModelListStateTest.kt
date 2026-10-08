package dev.pocketagent.app.ui.chat

// ChatState 模型大表三意图的 JVM 钉子(2026-10-09 效果层外移 · 任务3)。
//
// 10s 巡检循环体(sessionScope.launch 路径)在 JVM 不可驱动,其**状态转换**
// (ModelListSet/ModelListMerged/ModelListReset 三个意图)按任务口径在此钉死,
// 与迁出前 ChatScreen 巡检效果内的三段原式一一对应:
//  1. ModelListSet 整表替换(静态预取,失败项 null 不发意图由调用点承担);
//  2. ModelListMerged 按 id 首段过滤本 agent 旧行 + native 申报行追加
//     (L2 发现⑤根因收尾:provider 字段是上游供应商名,按 id 首段过滤才剔得掉),
//     且「先比对再赋值」—— 内容没变返回原实例,消除每 10s 一次的空转重组;
//  3. ModelListReset 清空(统一提示条重试,原 models = emptyList())。

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotSame
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ModelInfo

class ChatModelListStateTest {

    private fun model(id: String, name: String = "名-$id"): ModelInfo =
        ModelInfo.newBuilder().setId(id).setDisplayName(name).build()

    // ---- ModelListSet ----

    @Test
    fun `ModelListSet 整表替换`() {
        val base = ChatStateReducer.initial(20)
        val s = ChatStateReducer.reduce(base, ChatIntent.ModelListSet(listOf(model("codex/gpt"), model("pi/zai/glm"))))
        assertEquals(2, s.models.size)
        assertEquals("codex/gpt", s.models[0].id)
    }

    // ---- ModelListMerged ----

    @Test
    fun `ModelListMerged 按 id 首段过滤本agent旧行并追加申报行`() {
        val base = ChatStateReducer.initial(20).let {
            it.copy(
                models = listOf(
                    model("codex/gpt-5"),          // 本 agent(codex)旧行 → 剔除
                    model("codex/gpt-4"),          // 本 agent 旧行 → 剔除
                    model("pi/zai-coding-cn/glm"), // 别家行 → 保留
                ),
            )
        }
        val s = ChatStateReducer.reduce(
            base,
            ChatIntent.ModelListMerged("codex", listOf(model("codex/gpt-5.3"), model("codex/mini"))),
        )
        // 本 agent 旧行剔除、native 申报行唯一入库、别家行保留且次序不变
        assertEquals(listOf("pi/zai-coding-cn/glm", "codex/gpt-5.3", "codex/mini"), s.models.map { it.id })
    }

    @Test
    fun `ModelListMerged 内容没变返回原实例_先比对再赋值`() {
        val base = ChatStateReducer.initial(20).let {
            it.copy(models = listOf(model("codex/gpt-5.3")))
        }
        val s1 = ChatStateReducer.reduce(base, ChatIntent.ModelListMerged("codex", listOf(model("codex/gpt-5.3"))))
        // proto ModelInfo 按内容 equals:合并结果与现值一致 → 返回原实例(不触发下游重组)
        assertSame(base, s1)
        // 内容真的变了 → 新实例
        val s2 = ChatStateReducer.reduce(base, ChatIntent.ModelListMerged("codex", listOf(model("codex/mini"))))
        assertNotSame(base, s2)
        assertEquals(listOf("codex/mini"), s2.models.map { it.id })
    }

    // ---- ModelListReset ----

    @Test
    fun `ModelListReset 清空模型大表`() {
        val base = ChatStateReducer.initial(20).let { it.copy(models = listOf(model("codex/gpt"))) }
        val s = ChatStateReducer.reduce(base, ChatIntent.ModelListReset)
        assertTrue(s.models.isEmpty())
        // 其余字段不受影响(只清 models)
        assertEquals(base.roundsLoaded, s.roundsLoaded)
        assertEquals(base.threadTitle, s.threadTitle)
    }

    @Test
    fun `initial 模型大表为空`() {
        assertTrue(ChatStateReducer.initial(20).models.isEmpty())
    }
}
