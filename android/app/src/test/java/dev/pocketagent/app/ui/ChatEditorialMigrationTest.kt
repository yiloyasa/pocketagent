package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ToolCallBlock
import pocketagent.v1.ApprovalRequest

/**
 * 会话页编辑式迁移(2026-09-29 UI 整体升级·第二波 2/4)新增纯函数的钉子:
 *
 *  1) 投递徽标四态措辞对齐设计(待发送→已送达→执行中;失败保留内容可重试),
 *     daemon 的四级回执状态不变,只换嘴;
 *  2) daemon 审批请求 → PaApprovalDialog 白名单文案(目的/操作类型/命令/工作区)
 *     与原审批卡同一套映射,只换容器不换信息、不编新义。
 */
class ChatEditorialMigrationTest {

    // ---- 投递徽标措辞(四态) ----

    @Test
    fun `pending 读作 待发送`() {
        val c = editorialDeliveryCopy(DELIVERY_PENDING, 0)
        assertEquals("待发送", c.word)
        assertEquals("", c.detail)
        assertFalse(c.failed)
    }

    @Test
    fun `steered 读作 已送达·执行中`() {
        val c = editorialDeliveryCopy(DELIVERY_STEERED, 0)
        assertEquals("已送达", c.word)
        assertEquals("执行中", c.detail)
        assertFalse(c.failed)
    }

    @Test
    fun `queued 读作 已送达·排队中且位次如实`() {
        val first = editorialDeliveryCopy(DELIVERY_QUEUED, 1)
        assertEquals("已送达", first.word)
        assertEquals("排队中 · 下一条", first.detail)
        val third = editorialDeliveryCopy(DELIVERY_QUEUED, 3)
        assertEquals("排队中 · 第3位", third.detail)
        assertFalse(first.failed)
    }

    @Test
    fun `overflow 读作 未送达且保留内容可重试`() {
        val c = editorialDeliveryCopy(DELIVERY_OVERFLOW, 0)
        assertEquals("未送达", c.word)
        assertTrue(c.detail.contains("内容已保留"))
        assertTrue(c.detail.contains("重发"))
        assertTrue(c.failed)
    }

    @Test
    fun `未知投递状态不显示徽标`() {
        assertTrue(editorialDeliveryCopy("some-future-state", 0).word.isEmpty())
    }

    // ---- 审批弹窗文案组装(白名单) ----

    private fun shellRequest(json: String): ApprovalRequest = ApprovalRequest.newBuilder()
        .setRequestId("req-1")
        .setAction(ToolCallBlock.newBuilder().setName("shell").setJsonArgs(json))
        .build()

    @Test
    fun `shell 审批 目的与命令如实组装`() {
        val copy = approvalUiCopy(
            shellRequest("""{"command":"./gradlew test","cwd":"/home/u/proj"}"""),
        )
        assertEquals("agent 请求执行以下命令", copy.purpose)
        assertEquals("./gradlew test", copy.command)
        assertEquals("/home/u/proj", copy.cwd)
        assertTrue(copy.actionKind.startsWith("执行命令"))
    }

    @Test
    fun `shell 审批 操作类型标注 仅本次`() {
        val copy = approvalUiCopy(shellRequest("""{"command":"ls"}"""))
        assertTrue(
            "授权范围如实标注一次性(actionKind 应含「仅本次」): ${copy.actionKind}",
            copy.actionKind.endsWith("仅本次"),
        )
    }

    @Test
    fun `理由透传进目的不丢弃`() {
        val copy = approvalUiCopy(
            shellRequest("""{"command":"rm -rf build","reason":"清理构建产物"}"""),
        )
        assertTrue(copy.purpose.contains("理由：清理构建产物"))
    }

    @Test
    fun `file_change 列出文件与数量`() {
        val copy = approvalUiCopy(
            ApprovalRequest.newBuilder()
                .setRequestId("req-2")
                .setAction(
                    ToolCallBlock.newBuilder()
                        .setName("file_change")
                        .setJsonArgs("""{"files":["a.kt","b.kt"],"count":2}"""),
                )
                .build(),
        )
        assertTrue(copy.command.contains("a.kt"))
        assertTrue(copy.command.contains("b.kt"))
        assertTrue(copy.command.contains("共 2 个文件"))
        assertTrue(copy.actionKind.startsWith("修改文件"))
    }

    @Test
    fun `permissions 展开权限 JSON`() {
        val copy = approvalUiCopy(
            ApprovalRequest.newBuilder()
                .setRequestId("req-3")
                .setAction(
                    ToolCallBlock.newBuilder()
                        .setName("permissions")
                        .setJsonArgs("""{"permissions":{"read":["/tmp"]}}"""),
                )
                .build(),
        )
        assertTrue(copy.command.contains("/tmp"))
        assertTrue(copy.actionKind.startsWith("提升权限"))
    }

    @Test
    fun `未知动作类型给人话词而不是空白`() {
        val copy = approvalUiCopy(
            ApprovalRequest.newBuilder()
                .setRequestId("req-4")
                .setAction(ToolCallBlock.newBuilder().setName("exotic_op").setJsonArgs("{}"))
                .build(),
        )
        assertEquals("exotic_op · 仅本次", copy.actionKind)
        assertEquals("agent 请求执行以下操作", copy.purpose)
    }

    @Test
    fun `actionKind 词表覆盖既有动作类型`() {
        assertEquals("执行命令", approvalActionKindLabel("shell"))
        assertEquals("修改文件", approvalActionKindLabel("file_change"))
        assertEquals("应用补丁", approvalActionKindLabel("apply_patch"))
        assertEquals("提升权限", approvalActionKindLabel("permissions"))
        assertEquals("写入文件", approvalActionKindLabel("write_file"))
        // 反问类不是"审批动作",但若走错容器也有中性词可显,不露空白
        assertEquals("未知操作", approvalActionKindLabel(""))
    }
}
