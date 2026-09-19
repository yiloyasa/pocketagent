package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import pocketagent.v1.CodexGoalDetail
import pocketagent.v1.Thread

/**
 * 会话目标展示纯函数的钉子(goal_detail 电脑端现读 / goal 本地兜底 → 顶栏文案)。
 *
 * 重点钉取值优先级与"无目标不占位":重连/换端后手机顶栏必须以电脑端
 * thread/goal/get 现读值为准,本地库只做兜底。
 */
class GoalDisplayLogicTest {

    private fun threadWith(
        localGoal: String = "",
        detail: CodexGoalDetail? = null,
    ): Thread {
        val b = Thread.newBuilder()
        if (localGoal.isNotEmpty()) b.setGoal(localGoal)
        detail?.let { b.setGoalDetail(it) }
        return b.build()
    }

    @Test
    fun `desktop goal detail wins over local goal`() {
        val t = threadWith(
            localGoal = "本地旧目标",
            detail = CodexGoalDetail.newBuilder().setObjective("电脑端新目标").setStatus("active").build(),
        )
        assertEquals("目标:电脑端新目标 · 进行中", GoalDisplayLogic.goalLine(t))
    }

    @Test
    fun `local goal is the fallback when desktop detail is absent`() {
        // 老 daemon / 读不到:退回本地库目标,不带状态后缀。
        assertEquals("目标:把测试全跑绿", GoalDisplayLogic.goalLine(threadWith(localGoal = "把测试全跑绿")))
    }

    @Test
    fun `no goal anywhere renders no line`() {
        assertNull(GoalDisplayLogic.goalLine(null))
        assertNull(GoalDisplayLogic.goalLine(threadWith()))
        // detail 存在但 objective 为空、本地也没有:仍不占位。
        assertNull(
            GoalDisplayLogic.goalLine(
                threadWith(detail = CodexGoalDetail.getDefaultInstance()),
            ),
        )
    }

    @Test
    fun `status labels cover codex goal state machine and drop unknown`() {
        assertEquals("进行中", GoalDisplayLogic.statusLabel("active"))
        assertEquals("已暂停", GoalDisplayLogic.statusLabel("paused"))
        assertEquals("受阻", GoalDisplayLogic.statusLabel("blocked"))
        assertEquals("额度受限", GoalDisplayLogic.statusLabel("usageLimited"))
        assertEquals("预算受限", GoalDisplayLogic.statusLabel("budgetLimited"))
        assertEquals("已完成", GoalDisplayLogic.statusLabel("complete"))
        // 未知新状态:不追加后缀,避免瞎翻译。
        assertNull(GoalDisplayLogic.statusLabel("somethingNew"))
        assertNull(GoalDisplayLogic.statusLabel(""))
    }

    @Test
    fun `objective with unknown status shows no suffix`() {
        val t = threadWith(
            detail = CodexGoalDetail.newBuilder().setObjective("重构 daemon").setStatus("weird").build(),
        )
        assertEquals("目标:重构 daemon", GoalDisplayLogic.goalLine(t))
    }
}
