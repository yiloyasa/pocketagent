package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/**
 * Chat 浮层收敛的结构钉子(JVM 可测的口径,同 SheetScrollOverflowRegressionTest:
 * 项目无 Compose UI 测试基建,布局不可 JVM 实测,以"源码结构钉子"钉住迁移与红线)。
 *
 * 钉四件事:
 *  1) ChatScreen 里不再有裸 ModalBottomSheet —— 两处遗留(ApprovalSheet /
 *     ExpandedInputSheet)全部收敛进 PaSheet 参数化入口;
 *  2) PaSheet 的三个新参数默认值必须等于收敛前的行为(sheetState/properties/
 *     contentWindowInsets),老调用点零行为变化;
 *  3) 审批卡红线语义一字不动:不可下滑关闭(confirmValueChange 拒 Hidden)+
 *     拦截返回键(shouldDismissOnBackPress=false)+ 不替用户做决定
 *     (onDismissRequest 为空,关闭只能由用户点拒绝/允许/取消);
 *  4) 大窗编辑的 safeDrawing insets 修复不回退(顶部不压状态栏)。
 */
class PaSheetConvergenceTest {

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

    private fun chatSource(): String =
        locate("src/main/java/dev/pocketagent/app/ui/ChatScreen.kt").readText()

    private fun paSheetSource(): String =
        locate("src/main/java/dev/pocketagent/app/ui/component/PaSheet.kt").readText()

    /** ApprovalSheet 段:从函数签名起到 ExpandedInputSheet 签名前。 */
    private fun approvalSection(): String {
        val src = chatSource()
        val start = src.indexOf("fun ApprovalSheet(")
        assertTrue("ApprovalSheet 应存在", start >= 0)
        val end = src.indexOf("private fun ExpandedInputSheet(", start)
        assertTrue("ExpandedInputSheet 应在 ApprovalSheet 之后", end > start)
        return src.substring(start, end)
    }

    /** ExpandedInputSheet 段:函数签名起到下一个顶层私有函数为止。 */
    private fun expandedInputSection(): String {
        val src = chatSource()
        val start = src.indexOf("fun ExpandedInputSheet(")
        assertTrue("ExpandedInputSheet 应存在", start >= 0)
        val end = src.indexOf("private fun ExportChatSheet(", start)
        assertTrue("ExportChatSheet 应在 ExpandedInputSheet 之后", end > start)
        return src.substring(start, end)
    }

    // ---- 1) 裸 sheet 收口 ----

    @Test
    fun `ChatScreen_内不再有裸ModalBottomSheet调用`() {
        val calls = Regex("""(?<!\.)\bModalBottomSheet\s*\(""").findAll(chatSource()).count()
        assertEquals(
            "ChatScreen 里不应再有裸 ModalBottomSheet(审批卡/大窗编辑都应走 PaSheet 参数化入口);" +
                "若确需恢复,必须先过 sheet_overflow 审计与红线复核",
            0, calls,
        )
    }

    @Test
    fun `审批卡与大窗编辑_都走PaSheet`() {
        assertTrue("ApprovalSheet 应经 PaSheet 渲染", approvalSection().contains("PaSheet("))
        assertTrue("ExpandedInputSheet 应经 PaSheet 渲染", expandedInputSection().contains("PaSheet("))
    }

    // ---- 2) PaSheet 参数化默认值 = 收敛前行为 ----

    /** PaSheet 的签名段(fun PaSheet 到函数体开始),签名里不得出现实验类型。 */
    private fun paSheetSignature(): String {
        val src = paSheetSource()
        val start = src.indexOf("fun PaSheet(")
        assertTrue("PaSheet 应存在", start >= 0)
        val bodyStart = src.indexOf("{ content:", start).let { if (it < 0) src.indexOf(") {", start) else it }
        assertTrue("PaSheet 签名结束应存在", bodyStart > start)
        return src.substring(start, bodyStart)
    }

    @Test
    fun `PaSheet新参数默认值保持收敛前行为`() {
        val src = paSheetSource()
        // 【签名纪律】material3 1.3.0 把 SheetState / ModalBottomSheetProperties /
        // BottomSheetDefaults 整个类都标了 @ExperimentalMaterial3Api;实验类型进
        // 公开签名(或实验调用进 @Composable 默认参数)会把 opt-in 传播到每一个
        // PaSheet 调用点(编译实证:Collab/Home/RuntimeSection/Settings/SubAgent/
        // FolderPicker/ContextUsageCard 九处全红)。签名里只准稳定类型。
        val sig = paSheetSignature()
        assertFalse(
            "PaSheet 签名出现 SheetState(实验类)—— opt-in 会扩散到全部调用点",
            sig.contains("SheetState"),
        )
        assertFalse(
            "PaSheet 签名出现 ModalBottomSheetProperties(实验类)—— opt-in 会扩散到全部调用点",
            sig.contains("ModalBottomSheetProperties"),
        )
        // 不可下滑关闭以稳定布尔承载,内部映射 confirmValueChange 拒绝 Hidden
        assertTrue(
            "PaSheet 缺 swipeDismissEnabled 参数(等义 confirmValueChange 拒绝 Hidden)",
            src.contains("swipeDismissEnabled: Boolean = true"),
        )
        assertTrue(
            "PaSheet 内部缺 confirmValueChange 拒绝 Hidden 的映射 —— swipeDismissEnabled=false 语义落空",
            src.contains("confirmValueChange = { it != SheetValue.Hidden }"),
        )
        // 拦截返回键:ModalBottomSheetProperties 实验类不进签名,以稳定布尔等义承载
        assertTrue(
            "PaSheet 缺 interceptBackPress 参数(等义 properties.shouldDismissOnBackPress=false)",
            src.contains("interceptBackPress: Boolean = false"),
        )
        assertTrue(
            "PaSheet 内部未把 interceptBackPress 映射成 shouldDismissOnBackPress",
            src.contains("ModalBottomSheetProperties(shouldDismissOnBackPress = !interceptBackPress)"),
        )
        // contentWindowInsets 默认:与 M3 默认值逐位一致(safeDrawing 只取底部;
        // 经反编译字节码核实)。实验的 BottomSheetDefaults(整个类标了
        // @ExperimentalMaterial3Api)不得再被 import —— 引用它就会重新扩散 opt-in
        assertTrue(
            "PaSheet 缺 contentWindowInsets 参数或默认值不是 safeDrawing.only(Bottom)",
            src.contains("contentWindowInsets: @Composable () -> WindowInsets = {") &&
                src.contains("WindowInsets.safeDrawing.only(WindowInsetsSides.Bottom)") &&
                !src.contains("import androidx.compose.material3.BottomSheetDefaults"),
        )
        // 实验对象必须真的传给 ModalBottomSheet(钉住"参数化"不是摆设)
        assertTrue(src.contains("properties = effectiveProperties"))
        assertTrue(src.contains("contentWindowInsets = contentWindowInsets"))
    }

    // ---- 3) 审批卡红线语义(一字不动) ----

    @Test
    fun `审批卡_不可下滑关闭与拦截返回键语义原样保留`() {
        val section = approvalSection()
        // 不可下滑关闭:swipeDismissEnabled = false(PaSheet 内部映射
        // confirmValueChange 拒绝 Hidden,映射本身由上面的签名钉子钉住)
        assertTrue(
            "审批卡缺 swipeDismissEnabled = false —— 不可下滑关闭红线被破坏",
            section.contains("swipeDismissEnabled = false"),
        )
        // 拦截返回键:interceptBackPress = true(内部映射 shouldDismissOnBackPress = false)
        assertTrue(
            "审批卡缺 interceptBackPress = true —— 拦截返回键红线被破坏",
            section.contains("interceptBackPress = true"),
        )
        // 不替用户做决定:onDismissRequest 为空(系统关闭请求一律不执行关闭动作)
        assertTrue(
            "审批卡 onDismissRequest 不再为空 —— 「不替用户做决定」红线被破坏",
            section.contains("onDismissRequest = { }"),
        )
        // safeDrawing insets 修复不回退(顶部不压状态栏)
        assertTrue(
            "审批卡缺 contentWindowInsets = safeDrawing —— 顶部压状态栏的历史修复被回退",
            section.contains("contentWindowInsets = { WindowInsets.safeDrawing }"),
        )
    }

    // ---- 4) 大窗编辑 insets 修复不回退 ----

    @Test
    fun `大窗编辑_safeDrawing修复不回退`() {
        assertTrue(
            "ExpandedInputSheet 缺 contentWindowInsets = safeDrawing —— 顶部压状态栏的历史修复被回退",
            expandedInputSection().contains("contentWindowInsets = { WindowInsets.safeDrawing }"),
        )
    }

    // ---- 5) 抽屉推背 + 撤回排队入口的接线钉子 ----

    @Test
    fun `会话内容根_抽屉打开时挂推背模糊`() {
        val src = chatSource()
        assertTrue(
            "ChatScreen 内容根缺 paPushBack(drawerCtrl.visible) —— 最近会话抽屉没有推背模糊",
            src.contains(".paPushBack(drawerCtrl.visible)"),
        )
    }

    @Test
    fun `撤回排队按钮_门控与RPC接线存在`() {
        val src = chatSource()
        assertTrue("缺 queueClearVisible 门控调用", src.contains("queueClearVisible("))
        assertTrue("缺「撤回排队」按钮文案", src.contains("撤回排队"))
        assertTrue("缺 ClearQueuedInputsRequest RPC 接线", src.contains("clearQueuedInputs("))
    }
}
