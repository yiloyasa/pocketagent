package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Git 变更分组与操作危险分级(GitStatusLogic.kt)的单测 —— R2-D 落地项 4:
 * "状态分组(改动/暂存/未跟踪)、操作按钮按危险分级"。
 */
class GitStatusLogicTest {

    private fun change(
        path: String,
        staged: String,
        worktree: String,
    ) = pocketagent.v1.GitFileChange.newBuilder()
        .setPath(path)
        .setStagedStatus(staged)
        .setWorktreeStatus(worktree)
        .build()

    // ---- 单个变更分组 ----

    @Test
    fun `分组 暂存码非空且工作区为空 属已暂存`() {
        assertEquals(GitChangeGroup.STAGED, changeGroup("M", ""))
        assertEquals(GitChangeGroup.STAGED, changeGroup("A", ""))
        assertEquals(GitChangeGroup.STAGED, changeGroup("D", ""))
    }

    @Test
    fun `分组 工作区有码属未暂存 含暂存后又改`() {
        assertEquals(GitChangeGroup.WORKTREE, changeGroup("", "M"))
        assertEquals(GitChangeGroup.WORKTREE, changeGroup("M", "M"))
        assertEquals(GitChangeGroup.WORKTREE, changeGroup("A", "D"))
    }

    @Test
    fun `分组 双问号属未跟踪`() {
        assertEquals(GitChangeGroup.UNTRACKED, changeGroup("?", "?"))
    }

    @Test
    fun `分组 proto 便捷入口与字符串入口一致`() {
        val c = change("a.kt", "M", "M")
        assertEquals(changeGroup(c.stagedStatus, c.worktreeStatus), changeGroup(c))
    }

    // ---- 列表分组 ----

    @Test
    fun `列表分组 顺序恒为 已暂存-未暂存-未跟踪 空组跳过`() {
        val sections = groupChanges(
            listOf(
                change("new.txt", "?", "?"),
                change("b.kt", "", "M"),
                change("a.kt", "M", ""),
                change("c.md", "?", "?"),
            ),
        )
        assertEquals(
            listOf(GitChangeGroup.STAGED, GitChangeGroup.WORKTREE, GitChangeGroup.UNTRACKED),
            sections.map { it.first },
        )
        assertEquals(listOf("a.kt"), sections[0].second.map { it.path })
        assertEquals(listOf("b.kt"), sections[1].second.map { it.path })
        // 组内保持 daemon(git status)原序
        assertEquals(listOf("new.txt", "c.md"), sections[2].second.map { it.path })
    }

    @Test
    fun `列表分组 全空列表给空结果`() {
        assertTrue(groupChanges(emptyList()).isEmpty())
    }

    @Test
    fun `列表分组 只有未跟踪时不出空节头`() {
        val sections = groupChanges(listOf(change("x", "?", "?")))
        assertEquals(1, sections.size)
        assertEquals(GitChangeGroup.UNTRACKED, sections[0].first)
    }

    // ---- 分组标题 ----

    @Test
    fun `分组标题 三组互不相同且非空`() {
        val titles = GitChangeGroup.entries.map { gitGroupTitle(it) }
        assertTrue(titles.all { it.isNotBlank() })
        assertEquals(titles.size, titles.distinct().size)
    }

    // ---- 危险分级 ----

    @Test
    fun `危险分级 丢弃是破坏性 暂存安全 提交切分支需确认`() {
        assertEquals(GitDanger.DESTRUCTIVE, gitActionDanger("discard"))
        assertEquals(GitDanger.SAFE, gitActionDanger("stage"))
        assertEquals(GitDanger.SAFE, gitActionDanger("unstage"))
        assertEquals(GitDanger.SAFE, gitActionDanger("diff"))
        assertEquals(GitDanger.CONFIRM, gitActionDanger("commit"))
        assertEquals(GitDanger.CONFIRM, gitActionDanger("checkout"))
        assertEquals(GitDanger.CONFIRM, gitActionDanger("create-branch"))
    }

    @Test
    fun `危险分级 未知操作宁可多确认一次`() {
        assertEquals(GitDanger.CONFIRM, gitActionDanger("not-a-known-action"))
        assertEquals(GitDanger.CONFIRM, gitActionDanger(""))
    }

    @Test
    fun `危险确认文案 丢弃给动作词`() {
        assertEquals("丢弃", gitDangerConfirmLabel("discard"))
        assertEquals("确认", gitDangerConfirmLabel("anything-else"))
    }

    // ---- 「新建分支」入口门控(盘点 45:未连接/未拉到 status 时不得摆可点入口) ----

    @Test
    fun `新建分支入口 仓库已解析且已拉到status才亮`() {
        assertTrue(gitNewBranchEntryOn(repo = "/home/me/proj", notRepo = false, statusLoaded = true))
    }

    @Test
    fun `新建分支入口 未拉到status一律不亮_不裸露RPC错误`() {
        // 未连接/首拉中/上次刷新失败:status==null,建分支必败,入口整行不渲染
        assertTrue(!gitNewBranchEntryOn(repo = "/home/me/proj", notRepo = false, statusLoaded = false))
    }

    @Test
    fun `新建分支入口 未选仓库或非仓库目录不亮`() {
        assertTrue(!gitNewBranchEntryOn(repo = "", notRepo = false, statusLoaded = true))
        assertTrue(!gitNewBranchEntryOn(repo = "/home/me/docs", notRepo = true, statusLoaded = true))
        assertTrue(!gitNewBranchEntryOn(repo = "", notRepo = true, statusLoaded = false))
    }

    // ---- 既有单文件状态映射回归(与 GitScreen.kt 内的映射互补覆盖) ----

    @Test
    fun `状态中文映射 高频码不丢`() {
        assertEquals("修改", gitStatusLabel("M"))
        assertEquals("新增", gitStatusLabel("A"))
        assertEquals("删除", gitStatusLabel("D"))
        assertEquals("未跟踪", gitStatusLabel("?"))
        assertEquals("-", gitStatusLabel(""))
        // 未知码原样透出,不吞
        assertEquals("X", gitStatusLabel("X"))
    }
}
