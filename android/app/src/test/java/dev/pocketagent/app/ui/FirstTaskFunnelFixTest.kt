package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/**
 * 首任务漏斗修复回归(Adrian 对抗审核快赢版三件套):
 *  1. 手动填地址升为主操作(HomeScreen.manualAddSpec + discoveryItems 主按钮);
 *  2. 新建任务默认工作区放行(NewThreadSheet 空 picked 可建,不传 workspace);
 *  3. 教程前三页各加真实直达按钮(Onboarding.onboardActionFor)。
 *
 * 【为什么纯函数 + 源码钉子两级】Compose 布局在 JVM 层不可测(项目无 ui-test
 * 基建,不新增依赖),判定口径抽成纯函数直接断言;渲染接线(主色/放行/按钮存在)
 * 用"源码结构钉子"钉住,回退即红(同 NewThreadSheetScrollRegressionTest 口径)。
 */
class FirstTaskFunnelFixTest {

    /** 用户可见文案禁词(与 OnboardStepsTest 同表,新文案一律过筛)。 */
    private val banned = listOf("token", "daemon", "tailnet", "adb", "grpc", "api", "cli", "paguard", "onnx")

    private fun assertNoBanned(where: String, text: String) {
        val lower = text.lowercase()
        banned.forEach { word ->
            assertFalse("「$where」出现术语「$word」", lower.contains(word))
        }
    }

    // ---- 1. 手动填地址升为主操作 ----

    @Test
    fun `手动入口恒为主按钮规格_与主机卡同级`() {
        val spec = manualAddSpec()
        assertTrue("手动入口必须是主按钮(大触区+主色)", spec.primary)
        assertTrue("触高至少 48dp 才接得住首屏漏斗", spec.minHeightDp >= 48)
    }

    @Test
    fun `手动入口文案保留手动输入_无术语`() {
        val label = manualAddSpec().label
        assertTrue("文案必须讲清是手动填", label.contains("手动输入"))
        assertNoBanned("手动入口按钮", label)
    }

    // ---- 2. 新建任务默认工作区放行 ----

    @Test
    fun `空工作区也放行创建_不再卡空串`() {
        assertTrue("空串必须放行(走服务端默认位置)", newThreadCreateEnabled(""))
        assertTrue("空白串必须放行", newThreadCreateEnabled("   "))
        assertTrue("已选工作区照常放行", newThreadCreateEnabled("/sdcard/项目"))
    }

    @Test
    fun `创建按钮文案_没选讲默认_选了普通创建`() {
        assertTrue("没选要讲清走默认位置", newThreadCreateLabel("").contains("默认"))
        assertTrue("空白串同样讲默认", newThreadCreateLabel("  ").contains("默认"))
        assertEquals("选了就是普通创建", "创建", newThreadCreateLabel("/sdcard/项目"))
        assertNoBanned("创建按钮空态文案", newThreadCreateLabel(""))
    }

    @Test
    fun `空recents说明行给先用默认位置开始_无术语`() {
        assertTrue(
            "recents 为空必须给“先用默认位置开始”一行出路",
            NEW_THREAD_DEFAULT_HINT.contains("先用默认位置开始"),
        )
        assertNoBanned("空 recents 说明行", NEW_THREAD_DEFAULT_HINT)
    }

    // ---- 3. 教程前三页直达按钮 ----

    @Test
    fun `前三页各有真实直达按钮_去向复用既有接线`() {
        val connect = onboardActionFor("connect")
        val pair = onboardActionFor("pair")
        val firstTask = onboardActionFor("first-task")
        assertNotNull("连接页必须有直达按钮", connect)
        assertNotNull("配对页必须有直达按钮", pair)
        assertNotNull("首任务页必须有直达按钮", firstTask)
        // 去向只许 settings/home(MainActivity 已有接线,不新增回调)。
        assertEquals("home", connect!!.second)
        assertEquals("home", pair!!.second)
        assertEquals("home", firstTask!!.second)
        assertTrue("三页文案各不相同", setOf(connect.first, pair.first, firstTask.first).size == 3)
    }

    @Test
    fun `排障页按钮原样保留_未知步骤无按钮`() {
        val trouble = onboardActionFor("trouble")
        assertNotNull(trouble)
        assertEquals("去设置填写地址", trouble!!.first)
        assertEquals("settings", trouble.second)
        assertNull("未知步骤不得冒出按钮", onboardActionFor("no-such-step"))
    }

    @Test
    fun `直达按钮文案无术语`() {
        listOf("connect", "pair", "first-task", "trouble").forEach { id ->
            assertNoBanned("教程 $id 直达按钮", onboardActionFor(id)!!.first)
        }
    }

    // ---- 源码结构钉子:渲染接线回退即红 ----

    /** 从模块目录或仓库根向上定位源码文件(镜像/仓库两处布局都兼容)。 */
    private fun locateSource(rel: String): String {
        var dir: File? = File(System.getProperty("user.dir") ?: ".").absoluteFile
        repeat(6) {
            val cur = dir ?: return@repeat
            listOf(File(cur, rel), File(cur, "android/app/$rel")).firstOrNull { it.isFile }
                ?.let { return it.readText() }
            dir = cur.parentFile
        }
        error("找不到 $rel(从 ${System.getProperty("user.dir")} 向上查 6 层)")
    }

    @Test
    fun `HomeScreen_手动入口用主色大按钮_bottomNote保留`() {
        val src = locateSource("src/main/java/dev/pocketagent/app/ui/HomeScreen.kt")
        val manualStart = src.indexOf("item(key = \"manualAdd\")")
        assertTrue("discoveryItems 手动入口项必须存在", manualStart >= 0)
        val manualBlock = src.substring(manualStart, (src.indexOf("item(key = \"bottomNote\")", manualStart)
            .takeIf { it >= 0 } ?: src.length))
        assertTrue("手动入口必须走 manualAddSpec 口径", manualBlock.contains("manualAddSpec()"))
        assertTrue("手动入口必须是主色按钮(与主机卡同级)", manualBlock.contains("PaEditorial.Primary"))
        assertTrue("bottomNote 排查注记必须保留", src.contains("item(key = \"bottomNote\")"))
    }

    @Test
    fun `ThreadsScreen_新建面板放行空工作区_空态有默认位置说明`() {
        val src = locateSource("src/main/java/dev/pocketagent/app/ui/ThreadsScreen.kt")
        val start = src.indexOf("fun NewThreadSheet(")
        assertTrue("NewThreadSheet 声明应存在", start >= 0)
        val end = src.indexOf("\n/**", start).let { if (it >= 0) it else src.length }
        val body = src.substring(start, end)
        assertTrue("创建按钮必须走放行口径", body.contains("newThreadCreateEnabled(picked)"))
        assertTrue("按钮文案必须走默认位置口径", body.contains("newThreadCreateLabel(picked)"))
        assertTrue("空 recents 必须有默认位置说明", body.contains("NEW_THREAD_DEFAULT_HINT"))
        assertTrue("空串直接透传 onCreate(调用方省略 workspace 走默认)", body.contains("onCreate(picked)"))
    }

    @Test
    fun `Onboarding_前三页经统一口径接直达按钮`() {
        val src = locateSource("src/main/java/dev/pocketagent/app/ui/Onboarding.kt")
        assertTrue("教程页必须经 onboardActionFor 统一接线", src.contains("onboardActionFor(step.id)"))
        assertTrue("直达按钮渲染机制必须保留", src.contains("page.actionLabel != null && page.actionId != null"))
    }
}
