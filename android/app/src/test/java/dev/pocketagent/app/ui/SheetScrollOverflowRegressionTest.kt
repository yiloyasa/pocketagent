package dev.pocketagent.app.ui

import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/**
 * T-SS 回归钉子(同款 sheet 溢出清扫,承接 T-NS/SLA-21a 口径):
 * sheet 内容列无 verticalScroll 时,列表条目一多(recents / 自建分组 / 会话回合数),
 * 关键操作按钮会被推出视口 —— 被滚动裁掉的 Compose 节点不进无障碍树也不可点
 * (README §12 坑 6 的产品化形态)。本轮清扫四处:
 *  1. TerminalScreen 切换工作目录 sheet:目录 recents 无上限 → 内容列整体可滚;
 *  2. ThreadsScreen 分组整理 sheet(organizeTarget):自建分组无上限 → 内容列整体可滚;
 *  3. ChatScreen 回滚会话 sheet(rollbackOpen):选项随回合数增长 → 内容列整体可滚;
 *  4. ChatScreen 设置会话目标 sheet(slashGoalSheet):BasicTextField 只有 min 无 max,
 *     长目标把"保存目标"推出视口 → heightIn 封顶,输入内容在框内滚动(按钮不动,
 *     交互破坏最小)。
 *  5. HomeScreen 管理 agent 抽屉(ManageAgentsSheet):agents 接入数无上限
 *     (dsh 接入后继续增长)→ 列表列整体可滚(pa_audit sheet_overflow 规则首例命中,
 *     T-MS 修复)。
 *
 * 【口径】同 T-NS:项目无 Compose UI 测试基建,JVM 层不可测布局,
 * 以"源码结构钉子"钉住修复;真机可达性(bounds 在屏内可点,
 * 矮视口/横屏纳入口径)待 L2 复核。
 */
class SheetScrollOverflowRegressionTest {

    /** 从模块目录或仓库根向上定位指定源码文件(镜像/仓库两处布局都兼容)。 */
    private fun locate(vararg relCandidates: String): File {
        var dir: File? = File(System.getProperty("user.dir") ?: ".").absoluteFile
        repeat(6) {
            val cur = dir ?: return@repeat
            relCandidates.forEach { rel ->
                File(cur, rel).takeIf { it.isFile }?.let { return it }
                File(cur, "android/app/$rel").takeIf { it.isFile }?.let { return it }
            }
            dir = cur.parentFile
        }
        error("找不到源码文件 $relCandidates(从 ${System.getProperty("user.dir")} 向上查 6 层)")
    }

    /** 截取 [startMarker, endMarker) 之间的源码段(sheet 区域)。 */
    private fun section(text: String, startMarker: String, endMarker: String): String {
        val start = text.indexOf(startMarker)
        assertTrue("区域起始标记应存在: $startMarker", start >= 0)
        val end = text.indexOf(endMarker, start)
        assertTrue("区域结束标记应存在于起始之后: $endMarker", end > start)
        return text.substring(start, end)
    }

    private fun terminalSource(): String =
        locate("src/main/java/dev/pocketagent/app/ui/TerminalScreen.kt").readText()

    private fun threadsSource(): String =
        locate("src/main/java/dev/pocketagent/app/ui/ThreadsScreen.kt").readText()

    private fun chatSource(): String =
        locate("src/main/java/dev/pocketagent/app/ui/ChatScreen.kt").readText()

    private fun homeSource(): String =
        locate("src/main/java/dev/pocketagent/app/ui/HomeScreen.kt").readText()

    @Test
    fun `终端切换工作目录sheet_内容列必须可滚动且关键操作可达`() {
        val src = section(terminalSource(), "if (showDirPicker) {", "FolderPickerSheet(")
        assertTrue(
            "切换工作目录 sheet 内容 Column 缺 verticalScroll(rememberScrollState()) —— " +
                "目录 recents 无上限,多时浏览/切换操作行被推出视口且不可点(T-SS 回归)",
            src.contains("verticalScroll(rememberScrollState())"),
        )
        // 关键控件必须都在该可滚区域内:目录行(含默认目录)+ 浏览入口
        assertTrue(src.contains("availableRecents"))
        assertTrue(src.contains("默认目录"))
        assertTrue(src.contains("浏览电脑文件夹…"))
    }

    @Test
    fun `分组整理sheet_内容列必须可滚动且新建分组分叉可达`() {
        val src = section(threadsSource(), "organizeTarget?.let", "\"分叉会话\"")
        assertTrue(
            "分组整理 sheet 内容 Column 缺 verticalScroll(rememberScrollState()) —— " +
                "自建分组(sections)无上限,多时新建分组并移入/分叉被推出视口且不可点(T-SS 回归)",
            src.contains("verticalScroll(rememberScrollState())"),
        )
        // 关键控件必须都在该可滚区域内:未分组行 + 全部分组行 + 新建入口 + 分叉
        assertTrue(src.contains("sections.forEach"))
        assertTrue(src.contains("未分组"))
        assertTrue(src.contains("新建分组并移入…"))
    }

    @Test
    fun `回滚会话sheet_内容列必须可滚动且确认取消可达`() {
        val src = section(chatSource(), "if (rollbackOpen) {", "\"确认回滚\"")
        assertTrue(
            "回滚会话 sheet 内容 Column 缺 verticalScroll(rememberScrollState()) —— " +
                "回滚选项随会话回合数增长,长会话时取消/确认回滚被推出视口且不可点(T-SS 回归)",
            src.contains("verticalScroll(rememberScrollState())"),
        )
        // 关键控件必须都在该可滚区域内:选项列表 + 取消(确认回滚本身是段尾边界,单独钉)
        assertTrue(src.contains("options.forEach"))
        assertTrue(src.contains("取消"))
        assertTrue(chatSource().contains("\"确认回滚\""))
    }

    @Test
    fun `设置会话目标sheet_输入框高度必须封顶且保存按钮不随长目标出视口`() {
        val src = section(chatSource(), "if (slashGoalSheet) {", "\"已设置会话目标\"")
        assertTrue(
            "目标输入框缺 heightIn(min=..., max=...) 封顶 —— 目标文本一长输入框无限增高," +
                "把\"保存目标\"推出视口不可点(T-SS 回归)",
            src.contains(".heightIn(min = 72.dp, max = 200.dp)"),
        )
        // 保存按钮必须在同一 sheet 区域内(输入框封顶后总高有界,按钮恒可达)
        assertTrue(src.contains("保存目标"))
    }

    @Test
    fun `管理agent抽屉_列表列必须可滚动且开关条目在滚区域内`() {
        val src = section(homeSource(), "fun ManageAgentsSheet(", "// ---- 新建会话的 agent 接线")
        val scrollAt = src.indexOf("verticalScroll(rememberScrollState())")
        assertTrue(
            "管理 agent 抽屉列表列缺 verticalScroll(rememberScrollState()) —— " +
                "agents 接入数无上限(dsh 接入后继续增长),条目一多底部行开关被推出视口" +
                "且不可点(被裁节点不进无障碍树,README §12 坑 6;pa_audit sheet_overflow 回归)",
            scrollAt >= 0,
        )
        // 开关条目列表必须位于滚动列之内(列表/开关的声明晚于滚动修饰符,即被它包裹)
        val listAt = src.indexOf("agents.forEach")
        val switchAt = src.indexOf("Switch(")
        assertTrue("agents.forEach 应在 verticalScroll 列内(声明晚于滚动修饰符)", listAt > scrollAt)
        assertTrue("行内 Switch 应在 verticalScroll 列内(声明晚于滚动修饰符)", switchAt > scrollAt)
        // 关键元素都在 sheet 区域内:标题 + 副标题 + 固定头关闭按钮 + 条目副信息
        assertTrue(src.contains("管理 agent"))
        assertTrue(src.contains("停用后不再显示"))
        assertTrue(src.contains("\"关闭\""))
        assertTrue(src.contains("a.method"))
    }
}
