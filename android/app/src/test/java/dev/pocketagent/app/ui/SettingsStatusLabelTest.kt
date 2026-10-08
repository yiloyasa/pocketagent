package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.TaskCenter
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 下载任务状态四词口径钉子(2026-10-09 UI 收尾波,statusLabel 由 private 上收为
 * internal 纯函数后补的防回潮钉):
 *
 *  1) 全枚举逐项断言:五态 → 四词的映射一字不动(RUNNING=下载中 / DONE=已完成 /
 *     FAILED=失败 / CANCELED=已停止 / INTERRUPTED=已停止);
 *  2) 口径断言:五态恰收成四词 —— 已取消/已中断同收「已停止」(都是"没跑完且
 *     不是下载出错",用户下一步同一个:重新开始);失败与停止保持可辨;
 *  3) 闭合域断言:Status 枚举恰 5 个值。statusLabel 的 when 不写 else(闭合域
 *     穷举),未来新增枚举值时编译期先红;本断言双保险,防止有人"顺手"补了 else
 *     把新状态静默吞进旧词。
 *
 * 【非法/缺省输入的回退位置】TaskCenter.Status 是闭合 Kotlin 枚举,类型层不存在
 * 非法值;持久化串解析不出枚举的"非法输入"在 TaskCenter.parseTasks 层回退
 * (runCatching 整行丢弃,见本文件第 4 个测试)—— 这保证 statusLabel 的输入域
 * 恒为合法枚举,函数自身不需要(也不应该有)fallback 词。
 */
class SettingsStatusLabelTest {

    // ---- 1) 全枚举逐项映射 ----

    @Test
    fun `五态到四词映射逐项钉死`() {
        assertEquals("下载中", statusLabel(TaskCenter.Status.RUNNING))
        assertEquals("已完成", statusLabel(TaskCenter.Status.DONE))
        assertEquals("失败", statusLabel(TaskCenter.Status.FAILED))
        assertEquals("已停止", statusLabel(TaskCenter.Status.CANCELED))
        assertEquals("已停止", statusLabel(TaskCenter.Status.INTERRUPTED))
    }

    // ---- 2) 四词口径 ----

    @Test
    fun `五态恰收成四词_已取消与已中断同词_失败与停止可辨`() {
        val words = TaskCenter.Status.values().map { statusLabel(it) }
        assertEquals(
            "五态应恰收成四词(同义近义词各说各话会稀释四词口径)",
            4,
            words.toSet().size,
        )
        // 同收「已停止」的是 CANCELED/INTERRUPTED 这一对,不是别的组合
        assertEquals(
            "「已停止」应恰被已取消+已中断两态共享",
            setOf(TaskCenter.Status.CANCELED, TaskCenter.Status.INTERRUPTED),
            TaskCenter.Status.values().filter { statusLabel(it) == "已停止" }.toSet(),
        )
        // 失败可给原因、停止只说停下:两词不得混
        assertTrue("失败" != statusLabel(TaskCenter.Status.CANCELED))
        assertTrue("已停止" != statusLabel(TaskCenter.Status.FAILED))
    }

    // ---- 3) 闭合域:新增状态必须回到映射处补词,不允许静默吞进旧词 ----

    @Test
    fun `Status枚举闭合域恰五值_加值时先编译红再由此断言兜底`() {
        assertEquals(
            "TaskCenter.Status 出现新值:statusLabel 的穷举 when 会编译失败," +
                "请回到映射处为新状态定词(四词口径下先判断它归并到哪个既有词或新开一词)," +
                "再更新本断言",
            5,
            TaskCenter.Status.values().size,
        )
        // 输出域也钉死:任何输入的输出都必须落在四词表内
        val allowed = setOf("下载中", "已完成", "失败", "已停止")
        TaskCenter.Status.values().forEach { s ->
            assertTrue("statusLabel($s) 输出不在四词表内", statusLabel(s) in allowed)
        }
    }

    // ---- 4) 非法输入的回退位置:持久层整行丢弃,UI 永远见不到非法状态 ----

    @Test
    fun `持久化串含非法状态时整行回退_不产生任务条目`() {
        val sep = "\u0001"
        // 13 字段完整行,第 5 字段(f[4])是状态:一条合法 RUNNING + 一条非法状态串
        val raw = listOf(
            listOf("dl:ok", "t1", "DOWNLOAD", "正常任务", "DONE", "1", "1000", "0.5", "", "1", "", "", "").joinToString(sep),
            listOf("dl:bad", "t2", "DOWNLOAD", "坏状态任务", "NOT_A_STATUS", "1", "1000", "0.5", "", "1", "", "", "").joinToString(sep),
        ).joinToString("\n")
        val tasks = TaskCenter.parseTasks(raw)
        assertEquals("非法状态串的行应整行丢弃,只留合法行", 1, tasks.size)
        assertEquals("dl:ok", tasks.single().id)
    }
}
