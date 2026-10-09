package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 设置搜索(审计 P1-3 子项)纯函数的单测。
 *
 * 口径:空查询=不进入搜索模式(返回空列表,页面照常渲染分组);
 * 非空查询对 title+desc 大小写不敏感匹配;命中区间给高亮用。
 *
 * 【F2 IA 重构同步】索引随新结构改:一入口一落地页(target 全部对齐新 sub id),
 * 消亡页(userdata/daemon/appearance)不得残留;深链归一化纯函数一并列钉。
 */
class SettingsSearchFilterTest {

    private val index = settingsSearchIndex()

    @Test
    fun `索引非空且每条有组名`() {
        assertTrue(index.isNotEmpty())
        assertTrue(index.all { it.group.isNotBlank() && it.title.isNotBlank() })
    }

    @Test
    fun `空查询_返回空列表`() {
        assertTrue(filterSettingsEntries("", index).isEmpty())
        assertTrue(filterSettingsEntries("   ", index).isEmpty())
    }

    @Test
    fun `按标题命中`() {
        val hits = filterSettingsEntries("外观", index)
        // F2 就地化后「外观」是一级组自己的索引条目(组名=标题,target=null)。
        assertTrue(hits.any { it.title == "外观" })
    }

    @Test
    fun `按描述命中_大小写不敏感`() {
        val hits = filterSettingsEntries("mcp", index)
        assertTrue(hits.any { it.title == "Agent 与模型" }) // desc 含 "MCP"
    }

    @Test
    fun `无命中_返回空列表`() {
        assertTrue(filterSettingsEntries("不存在的设置项xyz", index).isEmpty())
    }

    @Test
    fun `命中区间_标题命中给区间_未命中标题给null`() {
        assertEquals(0 until 2, matchRange("对话与输入", "对话"))
        // "未识别操作确认":未识别操作=0..4,确认=5..6
        assertEquals(5 until 7, matchRange("未识别操作确认", "确认"))
        assertNull(matchRange("对话与输入", "模型"))
        assertNull(matchRange("对话与输入", ""))
    }

    // ---- F2 信息架构重排:一入口一落地页 ---------------------------------------

    @Test
    fun `连接与配对条目落connection页`() {
        val entry = index.first { it.title == "连接与配对" }
        assertEquals("connection", entry.target)
    }

    @Test
    fun `下载与存储条目落downloads页`() {
        // F2 拆页:不再落 userdata(旧「用户数据管理」已消亡)。
        assertEquals("downloads", index.first { it.title == "下载与存储" }.target)
    }

    @Test
    fun `主机信任条目并入连接页`() {
        // 一级页「主机信任」行已删除,指纹信任是「连接与配对」页的分区;
        // 搜「主机信任」必须还能到名实相符的页。
        val entry = index.first { it.title == "主机信任" }
        assertEquals("connection", entry.target)
    }

    @Test
    fun `电脑控制台补齐target_搜到即可跳`() {
        // 旧索引「电脑控制台」target=null(搜到也跳不过去);F2 独立二级页后必须可跳。
        assertEquals("console", index.first { it.title == "电脑控制台" }.target)
    }

    @Test
    fun `对话与输入组已入索引_一级就地展示无跳转目标`() {
        val entry = index.first { it.title == "对话与输入" }
        assertEquals("对话与输入", entry.group)
        assertTrue(entry.desc.contains("思考"))
        assertTrue(entry.desc.contains("工具"))
        assertTrue(entry.desc.contains("历史"))
        // 一级分组就地展示,没有二级页可跳:target=null(仅展示归属,与「外观」同口径)
        assertNull(entry.target)
    }

    @Test
    fun `消亡页不再入索引_全部target落新页`() {
        // F2:用户数据管理 / Daemon 桥接与输入 / 外观与阅读 三页消亡,
        // 索引不得再出现旧页名,也不得再指向旧 sub id。
        assertTrue(index.none { it.title == "Daemon 桥接与输入" })
        assertTrue(index.none { it.title == "用户数据管理" })
        assertTrue(index.none { it.title == "外观与阅读" })
        val validTargets = setOf("connection", "downloads", "agent", "capabilities", "console")
        assertTrue(index.all { it.target == null || it.target in validTargets })
    }

    @Test
    fun `搜历史轮数_命中对话与输入组`() {
        val hits = filterSettingsEntries("历史轮数", index)
        assertTrue(hits.any { it.title == "对话与输入" })
    }

    @Test
    fun `搜语音识别_命中对话与输入组条目`() {
        // Daemon 页消亡后,语音识别条目归属新组(不再是「Daemon 桥接与输入」)。
        val hits = filterSettingsEntries("语音识别", index)
        val hit = hits.first { it.title == "语音识别" }
        assertEquals("对话与输入", hit.group)
    }

    // ---- F3 终验:搜新组名本身 -----------------------------------------------

    @Test
    fun `搜对话_到达新组名_组名自洽且不可跳转`() {
        // 一级导航词"对话"必须命中新组(用户搜组名就要能到):
        val hits = filterSettingsEntries("对话", index)
        val hit = hits.firstOrNull { it.title == "对话与输入" }
        assertTrue(hit != null)
        // 命中的就是组自己的索引条目:组名=标题(一级就地展示,无二级页可跳),
        // 关键词覆盖标题与描述两侧(思考/工具/历史都在 desc 里)。
        assertEquals("对话与输入", hit!!.group)
        assertNull(hit.target)
        assertEquals(0 until 2, matchRange(hit.title, "对话"))
    }

    // ---- 深链 sub 参数归一化(userdata 兼容映射) ------------------------------

    @Test
    fun `深链sub归一化_旧userdata映射downloads`() {
        // 任务中心下载卡的历史深链 settings?sub=userdata 不断链。
        assertEquals("downloads", normalizeSettingsSub("userdata"))
    }

    @Test
    fun `深链sub归一化_新页id原样返回`() {
        assertEquals("connection", normalizeSettingsSub("connection"))
        assertEquals("downloads", normalizeSettingsSub("downloads"))
        assertEquals("agent", normalizeSettingsSub("agent"))
        assertEquals("capabilities", normalizeSettingsSub("capabilities"))
        assertEquals("console", normalizeSettingsSub("console"))
    }

    @Test
    fun `深链sub归一化_空串与未知值不猜落点`() {
        assertNull(normalizeSettingsSub(""))
        assertNull(normalizeSettingsSub("   "))
        // 消亡页的旧 id 与任意未知值都回一级页,不猜。
        assertNull(normalizeSettingsSub("daemon"))
        assertNull(normalizeSettingsSub("appearance"))
        assertNull(normalizeSettingsSub("whatever"))
    }

    // ---- 搜索命中子页的返回层(G1 遗留:返回回搜索态而非丢失上下文) ----

    @Test
    fun `返回落点_普通记层回该层`() {
        // Agent 与模型页 →「查看全部技能与插件」:返回要回到 Agent 与模型页。
        assertEquals("agent", settingsBackTargetOf("agent"))
        assertEquals("connection", settingsBackTargetOf("connection"))
    }

    @Test
    fun `返回落点_搜索哨兵与无记层都回一级页`() {
        // 搜索命中打开的子页:回一级页(settingsQuery 未清 = 搜索态原样还原);
        // 首页导航行打开的子页(无记层)行为不变。
        assertNull(settingsBackTargetOf(SETTINGS_RETURN_SEARCH))
        assertNull(settingsBackTargetOf(null))
    }

    @Test
    fun `搜索哨兵不与合法二级页id相撞`() {
        // 哨兵若混进深链 sub 参数,归一化必须按未知值处理(回一级页),不得误开新页。
        assertNull(normalizeSettingsSub(SETTINGS_RETURN_SEARCH))
        // 且它不在搜索索引的合法 target 集合里(索引条目永远不会记出这个歧义)。
        val validTargets = setOf("connection", "downloads", "agent", "capabilities", "console")
        assertTrue(index.all { it.target == null || it.target in validTargets })
        assertTrue(SETTINGS_RETURN_SEARCH !in validTargets)
    }
}
