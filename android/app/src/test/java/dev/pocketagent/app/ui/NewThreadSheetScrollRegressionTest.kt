package dev.pocketagent.app.ui

import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/**
 * T-NS 回归钉子(SLA-21a 真机缺陷,README §12 坑 6 的产品化形态):
 * 工作区 recents 有 6 条时,NewThreadSheet 内容 Column 无 verticalScroll,
 * "浏览电脑文件夹…/取消/创建"整段被推出视口 —— 被滚动裁掉的 Compose 节点
 * 不进无障碍树也不可点,自动化确认键候选词全部落空,上滑也不自愈
 * (证据 docs/verify/evidence/2026-09-20-night/slash/SLA-21a-*-sheet-dump.txt)。
 * recents 更多的用户将完全无法创建会话。
 *
 * 【口径】项目无 Compose UI 测试基建(无 ui-test-junit4/robolectric 依赖,
 * 不为本修复新增),Compose 布局在 JVM 层不可测,故以"源码结构钉子"钉住修复:
 * NewThreadSheet 的内容 Column 必须带 verticalScroll(rememberScrollState()),
 * 且关键控件都在该可滚区域内。真机可达性(bounds 在屏内可点)待 L2 复核。
 */
class NewThreadSheetScrollRegressionTest {

    /** 从模块目录或仓库根向上定位 ThreadsScreen.kt(镜像/仓库两处布局都兼容)。 */
    private fun locateThreadsScreen(): File {
        val rel = "src/main/java/dev/pocketagent/app/ui/ThreadsScreen.kt"
        var dir: File? = File(System.getProperty("user.dir") ?: ".").absoluteFile
        repeat(6) {
            val cur = dir ?: return@repeat
            listOf(File(cur, rel), File(cur, "android/app/$rel")).firstOrNull { it.isFile }?.let { return it }
            dir = cur.parentFile
        }
        error("找不到 ThreadsScreen.kt(从 ${System.getProperty("user.dir")} 向上查 6 层)")
    }

    /** 截取 NewThreadSheet 函数体源码(从声明到下一个顶层 KDoc 注释之前)。 */
    private fun newThreadSheetSource(): String {
        val text = locateThreadsScreen().readText()
        val start = text.indexOf("fun NewThreadSheet(")
        assertTrue("NewThreadSheet 声明应存在", start >= 0)
        val end = text.indexOf("\n/**", start).let { if (it >= 0) it else text.length }
        return text.substring(start, end)
    }

    @Test
    fun `内容列必须可滚动_修复后不得回退`() {
        val src = newThreadSheetSource()
        // 【UI 改版后的钉法】滚动保证结构性收口进统一玻璃浮层 PaSheet(内建
        // heightIn 高度上限 + verticalScroll;PaSheet.kt 本体另由
        // SheetScrollOverflowRegressionTest 的两级断言钉住)。这里钉:
        // NewThreadSheet 必须使用 PaSheet —— 回退成裸 ModalBottomSheet 即失败,
        // 此时必须自带 verticalScroll(rememberScrollState()) 才算守住 SLA-21a。
        assertTrue(
            "NewThreadSheet 应使用统一玻璃浮层 PaSheet(内建滚动与高度上限)—— " +
                "recents 多时创建/取消/浏览按钮会被推出视口且不可点(SLA-21a 回归)",
            src.contains("PaSheet("),
        )
    }

    @Test
    fun `关键控件都在 NewThreadSheet 区域内_6条recents场景不缺元素`() {
        // 证据场景的全部关键元素:recents 列表 + 空态提示 + 浏览入口 + 已选回显 + 取消/创建
        val src = newThreadSheetSource()
        assertTrue(src.contains("recents.map"))
        assertTrue(src.contains("recents.isEmpty()"))
        assertTrue(src.contains("浏览电脑文件夹"))
        assertTrue(src.contains("已选:"))
        assertTrue(src.contains("取消"))
        assertTrue(src.contains("先选工作区"))
        assertTrue(src.contains("创建"))
    }
}
