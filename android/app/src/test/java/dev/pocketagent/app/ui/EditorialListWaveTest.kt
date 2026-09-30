package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.theme.EditorialTone
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 编辑式第二波「列表组」(会话管理 04 / 任务中心 08 / 协同 10)迁移新增纯函数的单测。
 *
 * 钉四件事:
 *  1. 会话搜索(临时视图状态):空词放行;标题/工作区/agent 显示名命中;大小写不敏感;
 *  2. 会话页返回层级:抽屉 → 重命名 → 新建 → 组织 → 分组面板 → 搜索 → 筛选 → 主板块
 *     (README:搜索/筛选是临时视图状态,返回先清;搜索排在筛选之前);
 *  3. 任务中心三分区与离线分支:状态词全走 editorialStatusOf;断连时消息类 RUNNING
 *     不摆「正在执行」徽章(README:未连接不能写"正在执行"),下载类不受影响;
 *  4. 协同相位四词映射:接受/运行中=正在执行,完成=已完成,失败=未完成,未派发=无徽章。
 */
class EditorialListWaveTest {

    // ---- matchesThreadSearch(ThreadsScreen)----

    @Test
    fun `搜索空词放行全部`() {
        assertTrue(matchesThreadSearch("", "任意标题", "G:\\ws", "pi"))
        assertTrue(matchesThreadSearch("   ", "任意标题", "G:\\ws", "pi"))
    }

    @Test
    fun `搜索命中标题_工作区与agent显示名`() {
        assertTrue(matchesThreadSearch("登录", "修复登录流程", "G:\\ws", ""))
        assertTrue(matchesThreadSearch("试验", "无关标题", "G:\\试验区", ""))
        assertTrue(matchesThreadSearch("codex", "无关标题", "G:\\ws", ""))
        assertTrue(matchesThreadSearch("Pi", "无关标题", "G:\\ws", "pi"))
        assertTrue(matchesThreadSearch("zcode", "无关标题", "", "zcode"))
    }

    @Test
    fun `搜索大小写不敏感且未命中返回否`() {
        assertTrue(matchesThreadSearch("CODEX", "x", "", ""))
        assertFalse(matchesThreadSearch("不存在的词", "标题", "G:\\ws", "pi"))
    }

    @Test
    fun `搜索agent缺省按codex归属`() {
        // agent_id 空串 = codex(与 daemon 兼容规则同口径),按显示名 "Codex" 匹配
        assertTrue(matchesThreadSearch("Codex", "t", "w", ""))
    }

    // ---- threadsBackTarget(ThreadsScreen)----

    @Test
    fun `返回层级_抽屉最先_主板块最后`() {
        assertEquals(
            ThreadsBackTarget.DRAWER,
            threadsBackTarget(drawerVisible = true, renaming = true, newThreadSheet = true,
                organizing = true, sectionSheetOpen = true, hasQuery = true, filterActive = true),
        )
        assertEquals(
            ThreadsBackTarget.HOME,
            threadsBackTarget(false, false, false, false, false, hasQuery = false, filterActive = false),
        )
    }

    @Test
    fun `返回层级_临时视图状态先于主板块_搜索先于筛选`() {
        assertEquals(
            ThreadsBackTarget.SEARCH,
            threadsBackTarget(false, false, false, false, false, hasQuery = true, filterActive = true),
        )
        assertEquals(
            ThreadsBackTarget.FILTER,
            threadsBackTarget(false, false, false, false, false, hasQuery = false, filterActive = true),
        )
    }

    @Test
    fun `返回层级_浮层仍按既有顺序`() {
        assertEquals(
            ThreadsBackTarget.RENAME,
            threadsBackTarget(false, true, true, true, true, hasQuery = true, filterActive = true),
        )
        assertEquals(
            ThreadsBackTarget.NEW_THREAD,
            threadsBackTarget(false, false, true, true, true, hasQuery = true, filterActive = true),
        )
        assertEquals(
            ThreadsBackTarget.ORGANIZE,
            threadsBackTarget(false, false, false, true, true, hasQuery = true, filterActive = true),
        )
        assertEquals(
            ThreadsBackTarget.SECTION_SHEET,
            threadsBackTarget(false, false, false, false, true, hasQuery = true, filterActive = true),
        )
    }

    // ---- 任务中心:四词状态 / 离线分支 / 三分区 ----

    private fun task(
        id: String,
        kind: TaskCenter.Kind,
        status: TaskCenter.Status,
    ): TaskCenter.Task = TaskCenter.Task(
        id = id, threadId = "thr-$id", kind = kind, title = "任务$id", status = status,
    )

    @Test
    fun `任务状态走统一四词映射`() {
        assertEquals(
            EditorialStatus.RUNNING,
            taskStatusView(task("1", TaskCenter.Kind.MESSAGE, TaskCenter.Status.RUNNING), hostOffline = false)!!.status,
        )
        assertEquals(
            EditorialStatus.COMPLETED,
            taskStatusView(task("2", TaskCenter.Kind.MESSAGE, TaskCenter.Status.DONE), hostOffline = false)!!.status,
        )
        // 取消/中断/失败都是"未完成"(不是"失败"/"已取消"各说各话)
        for (s in listOf(
            TaskCenter.Status.FAILED,
            TaskCenter.Status.CANCELED,
            TaskCenter.Status.INTERRUPTED,
        )) {
            assertEquals(
                EditorialStatus.NOT_COMPLETED,
                taskStatusView(task("3", TaskCenter.Kind.MESSAGE, s), hostOffline = false)!!.status,
            )
        }
    }

    @Test
    fun `离线时消息类运行中不显示正在执行_并给离线提示`() {
        val running = task("1", TaskCenter.Kind.MESSAGE, TaskCenter.Status.RUNNING)
        assertNull(taskStatusView(running, hostOffline = true))
        assertEquals("离线 · 恢复连接后继续", taskOfflineHint(running, hostOffline = true))
        // 在线时同一任务照常显示「正在执行」,且无离线提示
        assertEquals(
            EditorialStatus.RUNNING,
            taskStatusView(running, hostOffline = false)!!.status,
        )
        assertNull(taskOfflineHint(running, hostOffline = false))
    }

    @Test
    fun `离线不影响下载类与消息类非运行态`() {
        // 下载类是手机本机行为,断连也照常「正在执行」(不给离线提示)
        val dl = task("1", TaskCenter.Kind.DOWNLOAD, TaskCenter.Status.RUNNING)
        assertEquals(EditorialStatus.RUNNING, taskStatusView(dl, hostOffline = true)!!.status)
        assertNull(taskOfflineHint(dl, hostOffline = true))
        // 消息类非运行态(如已完成)与离线无关
        val done = task("2", TaskCenter.Kind.MESSAGE, TaskCenter.Status.DONE)
        assertEquals(EditorialStatus.COMPLETED, taskStatusView(done, hostOffline = true)!!.status)
        assertNull(taskOfflineHint(done, hostOffline = true))
    }

    @Test
    fun `三分区计数_待你处理_进行中_已完成`() {
        val stats = taskCenterStats(
            listOf(
                task("a", TaskCenter.Kind.MESSAGE, TaskCenter.Status.RUNNING),
                task("b", TaskCenter.Kind.DOWNLOAD, TaskCenter.Status.RUNNING),
                task("c", TaskCenter.Kind.MESSAGE, TaskCenter.Status.FAILED),
                task("d", TaskCenter.Kind.MESSAGE, TaskCenter.Status.INTERRUPTED),
                task("e", TaskCenter.Kind.DOWNLOAD, TaskCenter.Status.DONE),
                // CANCELED 不进三卡(与旧概览条口径一致),只在分区里显示「未完成」
                task("f", TaskCenter.Kind.MESSAGE, TaskCenter.Status.CANCELED),
            ),
        )
        assertEquals(2, stats.waitingForYou)
        assertEquals(2, stats.running)
        assertEquals(1, stats.completed)
    }

    @Test
    fun `分区按四词分组_空组不出现_顺序为待处理_执行_完成_未完成`() {
        val partitions = taskPartitions(
            listOf(
                task("a", TaskCenter.Kind.MESSAGE, TaskCenter.Status.DONE),
                task("b", TaskCenter.Kind.MESSAGE, TaskCenter.Status.FAILED),
                task("c", TaskCenter.Kind.MESSAGE, TaskCenter.Status.RUNNING),
                task("d", TaskCenter.Kind.MESSAGE, TaskCenter.Status.CANCELED),
            ),
        )
        // 分区一律由 editorialStatusOf 派生:分区头与行内徽章长出同一个词。
        // FAILED/CANCELED 都是"未完成"(终局没做成),"等待你处理"分区在任务中心
        // 天然为空(该状态只出现在会话的审批/输入等待),空组不出现。
        assertEquals(
            listOf(
                EditorialStatus.RUNNING,
                EditorialStatus.COMPLETED,
                EditorialStatus.NOT_COMPLETED,
            ),
            partitions.map { it.first },
        )
        assertEquals(listOf("c"), partitions[0].second.map { it.id })
        assertEquals(listOf("a"), partitions[1].second.map { it.id })
        // 失败与取消同属「未完成」分区(行内各自的"重试"等动作不丢失)
        assertEquals(listOf("b", "d"), partitions[2].second.map { it.id })
        // 分区头取四词本身:徽章与分区头长出同一个词
        assertEquals("正在执行", partitions[0].first.word)
        assertEquals("未完成", partitions[2].first.word)
        // 空组不出现:只有 RUNNING 一项时只有一个分区
        assertEquals(
            1,
            taskPartitions(listOf(task("x", TaskCenter.Kind.MESSAGE, TaskCenter.Status.RUNNING))).size,
        )
    }

    // ---- 协同相位四词映射(CollabLogic)----

    @Test
    fun `协同相位_接受与运行中都是正在执行`() {
        assertEquals(EditorialStatus.RUNNING, collabPhaseStatus(CollabAgentPhase.ACK))
        assertEquals(EditorialStatus.RUNNING, collabPhaseStatus(CollabAgentPhase.RUNNING))
        assertEquals("正在执行", collabPhaseStatus(CollabAgentPhase.RUNNING)!!.word)
        assertEquals(EditorialTone.RUNNING, collabPhaseTone(CollabAgentPhase.ACK))
    }

    @Test
    fun `协同相位_完成与失败各归其位_未派发不摆徽章`() {
        assertEquals(EditorialStatus.COMPLETED, collabPhaseStatus(CollabAgentPhase.DONE))
        assertEquals(EditorialTone.NEUTRAL, collabPhaseTone(CollabAgentPhase.DONE))
        assertEquals(EditorialStatus.NOT_COMPLETED, collabPhaseStatus(CollabAgentPhase.ERROR))
        assertEquals("未完成", collabPhaseStatus(CollabAgentPhase.ERROR)!!.word)
        assertNull(collabPhaseStatus(CollabAgentPhase.IDLE))
        assertEquals(EditorialTone.GREY, collabPhaseTone(CollabAgentPhase.IDLE))
    }
}
