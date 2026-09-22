package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import pocketagent.v1.CollabRole

/**
 * 协同角色纯函数的钉子(需求④):抽出来的规则最容易被"顺手改个条件"悄悄反转,
 * 且反转后没有任何报错,只能等用户委派失败才暴露。这里用单测钉死三条:
 *  1. id 校验(空 / 含空格 / 超长 64)→ 与 daemon 同口径的人话文案,合法返回 null;
 *  2. 显示名回落:name 空 → 回落 id;
 *  3. 摘要行拼接:agent + 非空 model/effort,全空返回空串(不占位、不伪造)。
 * 这些函数无 Compose / 无 RPC 依赖,JVM 直接跑,不碰真机/daemon。
 */
class CollabRolesLogicTest {

    // ---- id 校验 ----

    @Test
    fun `empty id is rejected with human message`() {
        assertEquals("协同角色 id 不能为空", validateCollabRoleId(""))
        assertEquals("协同角色 id 不能为空", validateCollabRoleId("   "))
    }

    @Test
    fun `id with whitespace is rejected with human message`() {
        assertEquals("协同角色 id 不能含空格", validateCollabRoleId("reviewer x"))
        assertEquals("协同角色 id 不能含空格", validateCollabRoleId("reviewer\tx"))
    }

    @Test
    fun `id longer than 64 is rejected with human message`() {
        val tooLong = "a".repeat(COLLAB_ROLE_ID_MAX + 1)
        assertEquals("协同角色 id 过长(上限 $COLLAB_ROLE_ID_MAX)", validateCollabRoleId(tooLong))
    }

    @Test
    fun `valid id passes with null`() {
        assertNull(validateCollabRoleId("reviewer"))
        assertNull(validateCollabRoleId("sec-review-01"))
        // 恰好 64 字合法(边界)
        assertNull(validateCollabRoleId("a".repeat(COLLAB_ROLE_ID_MAX)))
    }

    // ---- 显示名回落 ----

    private fun role(id: String, name: String = "", agentId: String = "", model: String = "", effort: String = ""): CollabRole =
        CollabRole.newBuilder().setId(id).setName(name).setAgentId(agentId).setModel(model).setEffort(effort).build()

    @Test
    fun `display name falls back to id when name blank`() {
        assertEquals("安全审查者", collabRoleDisplayName(role("reviewer", name = "安全审查者")))
        assertEquals("reviewer", collabRoleDisplayName(role("reviewer", name = "")))
        assertEquals("reviewer", collabRoleDisplayName(role("reviewer", name = "   ")))
    }

    // ---- 摘要行拼接 ----

    @Test
    fun `summary line joins agent and non-empty model or effort`() {
        assertEquals(
            "Codex · gpt-5 · high",
            collabRoleSummaryLine(role("r", agentId = "codex", model = "gpt-5", effort = "high")),
        )
        // 只填 model
        assertEquals(
            "Pi · claude-3",
            collabRoleSummaryLine(role("r", agentId = "pi", model = "claude-3")),
        )
        // 只填 effort(未填 model)
        assertEquals(
            "ZCode · medium",
            collabRoleSummaryLine(role("r", agentId = "zcode", effort = "medium")),
        )
        // agent 名为未知 id 时原样显示(与 selectedAgentLabel 一致)
        assertEquals(
            "myagent · low",
            collabRoleSummaryLine(role("r", agentId = "myagent", effort = "low")),
        )
    }

    @Test
    fun `summary line falls back to id when agent blank but id present`() {
        assertEquals("reviewer", collabRoleSummaryLine(role("reviewer")))
    }

    @Test
    fun `summary line returns empty when both agent and id blank`() {
        assertEquals("", collabRoleSummaryLine(role("")))
    }
}
