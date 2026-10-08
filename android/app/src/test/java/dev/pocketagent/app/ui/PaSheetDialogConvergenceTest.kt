package dev.pocketagent.app.ui

import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/**
 * 居中确认弹窗收敛钉子(2026-10-09 UI 收尾波;口径同 PaSheetConvergenceTest /
 * SheetScrollOverflowRegressionTest:项目无 Compose UI 测试基建,布局不可 JVM
 * 实测,以"源码结构钉子"钉住收编与红线)。
 *
 * 范围:FilesScreen / GitScreen / RuntimeSection / SettingsScreen /
 * UnknownEventDetailSheet 五个文件的 14 处裸 M3 AlertDialog,全部收编进
 * PaSheet centered 变体(玻璃材质 + spring 出场统一,见 PaSheet KDoc)。
 * 钉三件事:
 *  1) 六个文件里不再有裸 AlertDialog(回潮即红);
 *  2) 每处收编点的 dismissal/确认回调语义锚点原样保留(回调体一字未动地搬家,
 *     这里钉的是语义靶标,不是排版);
 *  3) 危险确认钮一律走 PaEditorial 的 Denied 色槽(禁平然色裸值)。
 *
 * 【测试边界声明(2026-10-09 性能批)】本文件只防**结构回潮**,不证行为:
 * 结构锚点通过 ≠ 弹窗行为正确 —— 开合交互/dismissal 语义/返回键与焦点路径
 * 在 JVM 上不可实测(项目无 Compose UI 测试基建),行为级验证属 L2 真机
 * 自查范畴,本文件不承诺、本轮性能批也不在此补行为用例。
 */
class PaSheetDialogConvergenceTest {

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

    private fun filesSource(): String =
        locate("src/main/java/dev/pocketagent/app/ui/FilesScreen.kt").readText()

    private fun gitSource(): String =
        locate("src/main/java/dev/pocketagent/app/ui/GitScreen.kt").readText()

    private fun runtimeSource(): String =
        locate("src/main/java/dev/pocketagent/app/ui/RuntimeSection.kt").readText()

    private fun settingsSource(): String =
        locate("src/main/java/dev/pocketagent/app/ui/SettingsScreen.kt").readText()

    private fun unknownSource(): String =
        locate("src/main/java/dev/pocketagent/app/ui/unknown/UnknownEventDetailSheet.kt").readText()

    /** 截取 [startMarker, endMarker) 之间的源码段。 */
    private fun section(text: String, startMarker: String, endMarker: String): String {
        val start = text.indexOf(startMarker)
        assertTrue("区域起始标记应存在: $startMarker", start >= 0)
        val end = text.indexOf(endMarker, start)
        assertTrue("区域结束标记应存在于起始之后: $endMarker", end > start)
        return text.substring(start, end)
    }

    /** 收编点通用断言:走 PaSheet centered + 遮罩/返回的关闭语义锚点在场。 */
    private fun assertPaSheetCentered(src: String, label: String, dismissAnchor: String) {
        assertTrue(
            "$label 应经 PaSheet 居中变体渲染(收编回裸 AlertDialog 即红)",
            src.contains("PaSheet(") && src.contains("centered = true"),
        )
        assertTrue(
            "$label 的关闭语义锚点丢失:$dismissAnchor(遮罩/返回/取消必须与收编前同通道)",
            src.contains(dismissAnchor),
        )
    }

    // ---- 0) 裸 AlertDialog 清零 ----

    @Test
    fun `五个文件内不再有裸AlertDialog`() {
        listOf(
            "FilesScreen" to filesSource(),
            "GitScreen" to gitSource(),
            "RuntimeSection" to runtimeSource(),
            "SettingsScreen" to settingsSource(),
            "UnknownEventDetailSheet" to unknownSource(),
        ).forEach { (name, src) ->
            assertTrue(
                "$name 里不应再有裸 material3.AlertDialog(14 处已全部收编进 PaSheet centered);" +
                    "若确需恢复,必须先过模态家族审计",
                !src.contains("material3.AlertDialog"),
            )
        }
    }

    // ---- 1) FilesScreen ×2 ----

    @Test
    fun `FilesScreen_删除电脑文件确认_危险确认语义原样`() {
        val src = section(filesSource(), "pendingDelete?.let", "askDownload?.let")
        assertPaSheetCentered(src, "Files 删除确认", "onDismissRequest = { pendingDelete = null }")
        // 确认回调语义:清挂起态 → 删除电脑上的条目(动作发生在电脑上,文案红线)
        assertTrue(src.contains("pendingDelete = null"))
        assertTrue(src.contains("deleteEntry(e)"))
        assertTrue("删除确认的危险钮应走 Denied 色槽", src.contains("FilesSheetButtonTones.danger"))
    }

    @Test
    fun `FilesScreen_无法内预览引导下载_下载动作与回执原样`() {
        val src = section(filesSource(), "askDownload?.let", "if (drawerCtrl.visible)")
        assertPaSheetCentered(src, "Files 下载引导", "onDismissRequest = { askDownload = null }")
        assertTrue(src.contains("PaDownloader.downloadFromPc(e.path)"))
        assertTrue(src.contains("已开始下载 "))
    }

    // ---- 2) GitScreen ×5 ----

    @Test
    fun `GitScreen_diff查看器_单关闭语义原样`() {
        val src = section(gitSource(), "diff?.let", "pendingDiscard?.let")
        assertPaSheetCentered(src, "Git diff 查看器", "onDismissRequest = { diff = null }")
        assertTrue("DiffView 内容区应原样保留", src.contains("DiffView("))
        assertTrue("diff 内容保持石墨代码箱深底", src.contains("PaEditorial.GraphiteCode"))
    }

    @Test
    fun `GitScreen_丢弃改动确认_危险实心钮与warning触感原样`() {
        val src = section(gitSource(), "pendingDiscard?.let", "pendingCheckout?.let")
        assertPaSheetCentered(src, "Git 丢弃确认", "onDismissRequest = { pendingDiscard = null }")
        assertTrue("确认词应来自 gitDangerConfirmLabel 分级", src.contains("gitDangerConfirmLabel(\"discard\")"))
        assertTrue("丢弃确认应保留 warning 档触感", src.contains("PaHaptics.warning(view)"))
        assertTrue("丢弃 RPC 应原样", src.contains("discardChanges("))
        assertTrue("本文件的危险分级是红色实心钮(Denied 实底 + OnDenied 字)", src.contains("destructiveSolid"))
    }

    @Test
    fun `GitScreen_切换分支确认_切换RPC原样`() {
        val src = section(gitSource(), "pendingCheckout?.let", "// 新建分支")
        assertPaSheetCentered(src, "Git 切换分支确认", "onDismissRequest = { pendingCheckout = null }")
        assertTrue(src.contains("checkoutBranch("))
    }

    @Test
    fun `GitScreen_新建分支_空名守卫与创建RPC原样`() {
        val src = section(gitSource(), "if (showCreateBranch)", "// 提交对话框")
        assertPaSheetCentered(src, "Git 新建分支", "onDismissRequest = { showCreateBranch = false }")
        assertTrue("空分支名不得打 RPC(守卫原样)", src.contains("if (name.isEmpty()) return@GitSheetDialogButton"))
        assertTrue(src.contains("createBranch("))
    }

    @Test
    fun `GitScreen_提交对话框_禁用条件与清空回填原样`() {
        val src = section(gitSource(), "if (showCommit)", "// 切换仓库")
        assertPaSheetCentered(src, "Git 提交对话框", "onDismissRequest = { showCommit = false }")
        assertTrue("提交钮禁用条件原样(信息非空且不在飞行中)", src.contains("commitMsg.isNotBlank() && !busy"))
        assertTrue(src.contains("GitCommitRequest"))
        assertTrue("提交成功后应清空信息回填", src.contains("commitMsg = \"\""))
    }

    // ---- 3) RuntimeSection ×4 ----

    @Test
    fun `RuntimeSection_控制中心数值编辑_校验与写路由原样`() {
        val src = section(runtimeSource(), "private fun ControlNumberEditDialog(", "private fun ControlSelectEditDialog(")
        assertPaSheetCentered(src, "控制中心数值编辑", "PaSheet(onDismissRequest = onDismiss, centered = true)")
        assertTrue("非负整数校验原样", src.contains("请填一个非负整数"))
        assertTrue("写路由按 domainId 原样", src.contains("ControlPlaneState.applyNumber"))
    }

    @Test
    fun `RuntimeSection_控制中心选择编辑_预选纪律与写路由原样`() {
        val src = section(runtimeSource(), "private fun ControlSelectEditDialog(", "private fun BudgetEditDialog(")
        assertPaSheetCentered(src, "控制中心选择编辑", "PaSheet(onDismissRequest = onDismiss, centered = true)")
        assertTrue("现值不在候选不预选(预选纪律原样)", src.contains("item.current.takeIf { it in item.options }"))
        assertTrue("未预选保存禁用", src.contains("enabled = selected != null"))
        assertTrue(src.contains("ControlPlaneState.applySelect"))
    }

    @Test
    fun `RuntimeSection_预算项编辑_恢复默认与MB换算原样`() {
        val src = section(runtimeSource(), "private fun BudgetEditDialog(", "/** 只取现值的数字部分")
        assertPaSheetCentered(src, "预算项编辑", "PaSheet(onDismissRequest = onDismiss, centered = true)")
        assertTrue("恢复默认动作保留(写 null)", src.contains("saveRuntimeEdit(row.key, null)"))
        assertTrue("保存换算原样(字节类键 MB→字节)", src.contains("saveRuntimeEdit(row.key, scaled)"))
        assertTrue(src.contains("n shl 20"))
    }

    @Test
    fun `RuntimeSection_会话记忆清除确认_提交中门控与乐观回滚原样`() {
        val src = section(runtimeSource(), "pendingClear?.let", "/** 清单里的一个会话行")
        // 提交中(clearing)时遮罩/返回不再关:门控表达式原样
        assertPaSheetCentered(src, "会话记忆清除确认", "onDismissRequest = { if (!clearing) pendingClear = null }")
        assertTrue("提交中两钮同禁", src.contains("enabled = !clearing"))
        assertTrue("乐观更新与快照回滚链路原样", src.contains("threadRowsWithout(snapshot, target.threadId)"))
        assertTrue(src.contains("clearThreadMemory"))
    }

    // ---- 4) SettingsScreen ×4 ----

    @Test
    fun `SettingsScreen_清除主机指纹确认_清除与回执原样`() {
        val src = section(settingsSource(), "if (confirmClearPin)", "// ---- 删除已下载文件的二次确认")
        assertPaSheetCentered(src, "清除主机指纹确认", "onDismissRequest = { confirmClearPin = false }")
        assertTrue(src.contains("AgentConnection.clearHostPin()"))
        assertTrue(src.contains("已清除指纹,下次连接将重新信任主机"))
        assertTrue("危险钮应走 Denied 色槽", src.contains("SettingsSheetButtonTones.danger"))
    }

    @Test
    fun `SettingsScreen_删除已下载文件确认_按实际结果回执原样`() {
        // 段序:confirmClearPin 在前、pendingDelete 在后,段尾取 CacheSection 的 KDoc
        val src = section(settingsSource(), "pendingDelete?.let", "存储占用 / 缓存管理")
        assertPaSheetCentered(src, "删除已下载文件确认", "onDismissRequest = { pendingDelete = null }")
        assertTrue("按实际删除结果说话(删不掉不谎报已删除)", src.contains("文件可能已被系统清理,条目已移除"))
        assertTrue(src.contains("deleteLocalFile(ctx, target.localUri)"))
        assertTrue(src.contains("TaskCenter.remove(target.id)"))
    }

    @Test
    fun `SettingsScreen_清空任务条目确认_只清记录不碰文件原样`() {
        val src = section(settingsSource(), "if (confirmClearTasks)", "// ---- 会话快照二次确认")
        assertPaSheetCentered(src, "清空任务条目确认", "onDismissRequest = { confirmClearTasks = false }")
        assertTrue(src.contains("TaskCenter.clearFinished()"))
        assertTrue(src.contains("已清空已完成的条目"))
    }

    @Test
    fun `SettingsScreen_清空会话快照确认_IO线程清与主线程收尾原样`() {
        val src = section(settingsSource(), "if (confirmClearSnap)", "/** 缓存分区的一格数字")
        assertPaSheetCentered(src, "清空会话快照确认", "onDismissRequest = { confirmClearSnap = false }")
        assertTrue(src.contains("ThreadSnapshot.clearAll()"))
        assertTrue(src.contains("Dispatchers.IO"))
        assertTrue(src.contains("已释放 "))
    }

    // ---- 5) UnknownEventDetailSheet ×1 ----

    @Test
    fun `UnknownEventDetailSheet_动作确认闸门_确认才执行动作原样`() {
        val src = section(unknownSource(), "pendingConfirm?.let", "/** 详情拉取状态")
        assertPaSheetCentered(src, "未知事件动作确认", "onDismissRequest = { pendingConfirm = null }")
        // 确认钮才 runAction(带当前备注);取消/遮罩/返回不执行
        assertTrue(src.contains("runAction(card, action, notes[card.eventId] ?: \"\")"))
        assertTrue("描述缺席应回退到占位人话", src.contains("action.description.ifBlank { UnknownEventCopy.ACTION_DONE }"))
    }
}
