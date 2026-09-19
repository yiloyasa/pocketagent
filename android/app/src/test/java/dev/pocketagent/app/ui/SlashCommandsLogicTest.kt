package dev.pocketagent.app.ui

// SlashCommandsLogicTest —— 斜杠命令卡面纯逻辑的 JVM 单测:
//   1) 可见性:只认面板会话(zcode/codex/pi 三源)、离线快照态不显示、输入须以 "/" 开头敲出词元;
//   2) 过滤:前缀匹配大小写不敏感、裸 "/" 全量、无命中退全量、**不再截断条数**(纵向可滑);
//   3) 内置伪命令层:置顶合并、与 daemon 重名让位、非面板会话没有伪命令、
//      动作分流(FILL 回填 / GOAL、COMPACT 执行动作不填输入框);
//   4) 点选回填:替换 "/词元" 并补空格、保留词元后文字、无词元 no-op;
//      codex 技能填自然语言「请使用 <名称> 技能:」;pi 真命令走 "/命令 " 回填(pi 原生展开);
//   5) 词元清理:stripSlashToken 摘掉 "/词元"(伪命令执行后输入框不留残词);
//   6) 缓存:按会话存取、空列表不入缓存(下次进会话重拉)、可清理;
//   7) "/计划"(仅 codex 的内置伪命令):codex 才可见、带 REVIEW 动作、
//      可按中文前缀筛出;失败文案按错误形状分流(daemon 业务错误透传 description,
//      UNIMPLEMENTED/UNAVAILABLE 翻成人话,兜底永远有反馈)。

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class SlashCommandsLogicTest {

    private val cmds = listOf(
        SlashCommandInfo("goal", "常驻目标", "/goal [pause|resume|…]"),
        SlashCommandInfo("compact", "历史压缩", ""),
        SlashCommandInfo("init", "生成项目说明", "/init"),
    )

    // ---- 词元识别 ----

    @Test
    fun `词元取第一个空白前的前缀且必须带斜杠`() {
        assertEquals("/go", slashTokenOf("/go 修复登录"))
        assertEquals("/goal", slashTokenOf("/goal"))
        assertEquals("/g", slashTokenOf("/g\n下一行"))
        assertEquals("/", slashTokenOf("/")) // 裸斜杠也算词元(刚敲出斜杠就见全量列表)
        // 中文伪命令词元(内置伪命令 /目标、/压缩 用中文真名)
        assertEquals("/目", slashTokenOf("/目"))
        assertNull(slashTokenOf("普通输入"))
        assertNull(slashTokenOf(""))
    }

    // ---- 可见性 ----

    @Test
    fun `面板只对面板会话且输入带斜杠词元时可见`() {
        assertTrue(slashPanelVisible("zcode", "/g", cmds, offline = false))
        assertTrue(slashPanelVisible("ZCode", "/goal", cmds, offline = false)) // 大小写不敏感
        // codex 双源:skills 即命令(daemon 侧按 agent_id 自动路由同一 RPC)
        assertTrue(slashPanelVisible("codex", "/g", cmds, offline = false))
        assertTrue(slashPanelVisible("Codex", "/goal", cmds, offline = false))
        // pi 三源:get_commands 真名映射;且内置伪命令常驻,daemon 列表为空也有卡面
        assertTrue(slashPanelVisible("pi", "/g", cmds, offline = false))
        assertTrue(slashPanelVisible("pi", "/", emptyList(), offline = false))
        // 其余归属 / 未知(旧库空串是 codex 兜底,但归属字段层面仍不算面板会话)
        assertFalse(slashPanelVisible(null, "/", cmds, offline = false))
        assertFalse(slashPanelVisible("", "/g", cmds, offline = false))
        // 离线快照态输入已停用,面板不出现
        assertFalse(slashPanelVisible("zcode", "/g", cmds, offline = true))
        // 非斜杠输入不显示
        assertFalse(slashPanelVisible("zcode", "帮我修 bug", cmds, offline = false))
    }

    @Test
    fun `三源判定只认 zcode codex 与 pi`() {
        assertTrue(isSlashPanelAgent("zcode"))
        assertTrue(isSlashPanelAgent("codex"))
        assertTrue(isSlashPanelAgent("pi"))
        assertTrue(isSlashPanelAgent("ZCode"))
        assertTrue(isSlashPanelAgent("PI"))
        assertFalse(isSlashPanelAgent(null))
        assertFalse(isSlashPanelAgent(""))
        assertFalse(isSlashPanelAgent("echo"))
        // 三个单判各自独立、互不误伤
        assertTrue(isZcodeAgent("zcode") && !isCodexAgent("zcode") && !isPiAgent("zcode"))
        assertTrue(isCodexAgent("codex") && !isZcodeAgent("codex") && !isPiAgent("codex"))
        assertTrue(isPiAgent("pi") && !isZcodeAgent("pi") && !isCodexAgent("pi"))
    }

    // ---- 过滤(纵向卡面:不截断条数)----

    @Test
    fun `按前缀过滤且大小写不敏感`() {
        val got = filterSlashCandidates(cmds, "/CO")
        assertEquals(listOf("compact"), got.map { it.name })
        assertEquals(listOf("init"), filterSlashCandidates(cmds, "/in").map { it.name })
        assertEquals(listOf("goal"), filterSlashCandidates(cmds, "/GOAL").map { it.name })
        // 中文伪命令前缀过滤:slashCandidatesFor 合并后按词元筛
        val merged = slashCandidatesFor("pi", cmds)
        assertTrue(merged.none { it.action == SlashAction.GOAL })
    }

    @Test
    fun `裸斜杠全量且无前缀命中退全量`() {
        // 裸 "/" → 全量
        assertEquals(cmds.map { it.name }, filterSlashCandidates(cmds, "/").map { it.name })
        // 词元后带空格(已在敲参数)→ 面板收起
        assertTrue(filterSlashCandidates(cmds, "/goal x").isEmpty())
        // 记不清命令名:前缀无命中退全量,面板保持可用
        assertEquals(cmds.map { it.name }, filterSlashCandidates(cmds, "/xyz").map { it.name })
    }

    @Test
    fun `候选不再截断纵向滑动看全量`() {
        // 旧横滚条的 8 条上限已移除:codex 31 技能 / pi 56 命令都要能看到
        val many = (1..12).map { SlashCommandInfo("cmd$it", "", "") }
        val got = filterSlashCandidates(many, "/")
        assertEquals(12, got.size)
        assertEquals("cmd1", got.first().name)
    }

    // ---- 内置伪命令层 ----

    @Test
    fun `内置伪命令置顶且非面板会话没有候选`() {
        // 置顶:伪命令在最前(用户点名的"目标"入口第一眼可见)
        val merged = slashCandidatesFor("zcode", cmds)
        assertEquals(listOf("goal", "compact", "init"), merged.map { it.name })
        assertTrue(merged.all { it.action == SlashAction.FILL })
        // 非面板会话:连伪命令也没有(面板整体不显示)
        assertTrue(slashCandidatesFor("echo", cmds).isEmpty())
        assertTrue(slashCandidatesFor(null, cmds).isEmpty())
    }

    @Test
    fun `内置伪命令带动作且daemon重名让位`() {
        val builtin = builtinSlashCommands()
        assertEquals(listOf("目标", "压缩"), builtin.map { it.name })
        assertTrue(builtin.all { it.action != SlashAction.FILL })
        // daemon 回了同名命令(假设某天 zcode 有真 "/压缩"):真实命令优先,伪命令让位
        val clash = listOf(SlashCommandInfo("压缩", "真命令", ""))
        val merged = slashCandidatesFor("zcode", clash)
        assertEquals(2, merged.size) // 目标(伪) + 压缩(daemon),无重复
        assertEquals(1, merged.count { it.name == "压缩" })
        assertEquals(SlashAction.FILL, merged.first { it.name == "压缩" }.action)
    }

    @Test
    fun `伪命令点选是动作不是回填`() {
        // applySlashPick 对伪命令是防御 no-op(动作由 UI 在调用前拦截执行)
        val goal = builtinSlashCommands().first { it.action == SlashAction.GOAL }
        val compact = builtinSlashCommands().first { it.action == SlashAction.COMPACT }
        assertEquals("/目", applySlashPick("/目", goal, "zcode"))
        assertEquals("/压", applySlashPick("/压", compact, "pi"))
    }

    // ---- 词元清理(伪命令执行后) ----

    @Test
    fun `stripSlashToken摘掉词元且保留后续文字`() {
        assertEquals("", stripSlashToken("/目标"))
        assertEquals("", stripSlashToken("/目"))
        assertEquals("修复登录", stripSlashToken("/目 修复登录"))
        assertEquals("帮我修 bug", stripSlashToken("帮我修 bug")) // 无词元 no-op
        assertEquals("", stripSlashToken(""))
    }

    // ---- 点选回填 ----

    @Test
    fun `点选把词元替换成命令并保留后续文字`() {
        assertEquals("/goal ", applySlashSelection("/g", "goal"))
        assertEquals("/goal 修复登录", applySlashSelection("/go 修复登录", "goal"))
        // 多行输入:只动第一行的词元,其余原样
        assertEquals("/init\n第二行", applySlashSelection("/in\n第二行", "init"))
        // 无斜杠词元是 no-op(面板不可见时点不到,防御)
        assertEquals("普通输入", applySlashSelection("普通输入", "goal"))
    }

    @Test
    fun `点选三源分流`() {
        val skill = SlashCommandInfo("commit", "提交代码", "") // codex 技能:inputHint 恒空
        // codex 会话:没有 "/xxx" 语法 → 自然语言,发送后 codex 依据技能描述自行触发
        assertEquals("请使用 commit 技能：", applySlashPick("/com", skill, "codex"))
        // zcode 会话:保持 "/命令 " 回填语义
        assertEquals("/goal ", applySlashPick("/g", cmds[0], "zcode"))
        // pi 会话:get_commands 真命令在 prompt 以 /<名> 开头时由 pi 原生展开
        // (extension 直接执行、skill:/模板注入),回填真名即可触发,不转译自然语言
        val piCmd = SlashCommandInfo("websearch", "Open web search curator", "")
        assertEquals("/websearch ", applySlashPick("/web", piCmd, "pi"))
        val piSkill = SlashCommandInfo("skill:mmx-cli", "Use mmx", "")
        assertEquals("/skill:mmx-cli ", applySlashPick("/sk", piSkill, "pi"))
        // codex 将来若带 inputHint:退回模板填充,与 zcode 同款(含补空格)
        assertEquals("/init ", applySlashPick("/i", cmds[2], "codex"))
        // 其余 agent(面板不显示)防御原样
        assertEquals("普通输入", applySlashPick("普通输入", skill, "echo"))
    }

    // ---- 缓存 ----

    @Test
    fun `缓存按会话存取且空列表不入缓存`() {
        val tid = "th-slash-test"
        SlashCommandCache.clear(tid)
        assertNull(SlashCommandCache.get(tid))
        SlashCommandCache.put(tid, cmds)
        assertEquals(cmds, SlashCommandCache.get(tid))
        // daemon 回空(新会话还没落地):不入缓存,下次进会话重拉
        SlashCommandCache.put(tid, emptyList())
        assertEquals(cmds, SlashCommandCache.get(tid))
        SlashCommandCache.clear(tid)
        assertNull(SlashCommandCache.get(tid))
    }

    // ---- /计划(仅 codex 的内置伪命令)----

    @Test
    fun `never advertise review as a native plan mode`() {
        for (id in listOf("codex", "pi", "zcode")) {
            assertTrue(slashCandidatesFor(id,cmds).none { it.action==SlashAction.REVIEW })
        }
        val native=listOf(SlashCommandInfo("plan", "Native plan", "/plan"))
        assertTrue(slashCandidatesFor("zcode",native).any { it.name=="plan" && it.action==SlashAction.FILL })
    }

    @Test
    fun `计划失败文案按错误形状分流`() {
        // daemon 业务错误:透传中文 description,不把 "FAILED_PRECONDITION:" 码带上
        assertEquals(
            "计划模式目前仅 Codex 会话支持",
            slashReviewFailureMessage(
                io.grpc.StatusRuntimeException(
                    io.grpc.Status.FAILED_PRECONDITION.withDescription("计划模式目前仅 Codex 会话支持"),
                ),
            ),
        )
        // 会话未落地的未加载错误同样透传
        assertEquals(
            "这个会话还没有在电脑上落地(先发一条消息),暂时无法发起计划审查",
            slashReviewFailureMessage(
                io.grpc.StatusRuntimeException(
                    io.grpc.Status.FAILED_PRECONDITION
                        .withDescription("这个会话还没有在电脑上落地(先发一条消息),暂时无法发起计划审查"),
                ),
            ),
        )
        // 旧 daemon(没有 StartReview RPC):提示升级电脑端
        assertTrue(
            slashReviewFailureMessage(io.grpc.StatusRuntimeException(io.grpc.Status.UNIMPLEMENTED))
                .contains("daemon 版本过旧"),
        )
        // 连不上电脑:可执行建议
        assertTrue(
            slashReviewFailureMessage(io.grpc.StatusRuntimeException(io.grpc.Status.UNAVAILABLE))
                .contains("连不上电脑"),
        )
        // 非 gRPC 异常:message 有值透传,空值兜底,永远有反馈
        assertEquals("落库失败", slashReviewFailureMessage(IllegalStateException("落库失败")))
        assertEquals(SLASH_REVIEW_FALLBACK, slashReviewFailureMessage(IllegalStateException("")))
    }
}
