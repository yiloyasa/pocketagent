package dev.pocketagent.app.ui

import io.grpc.Status
import io.grpc.StatusRuntimeException
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.CollabPolicy
import pocketagent.v1.SetCodexSettingsRequest
import pocketagent.v1.ControlDomain
import pocketagent.v1.ControlItem
import pocketagent.v1.ControlPlane
import pocketagent.v1.RuntimeLimits
import pocketagent.v1.RuntimePolicy

/**
 * 控制中心(GetControlPlane 聚合读 + 既有写通道再路由)数据层的钉子。
 *
 * 【为什么必须钉】控制中心把 daemon 给的域/项**一次看全**,错的形态也跟着"看全":
 *  - 把 daemon 的 summary 改写/重排,用户核不了账(同源纪律:两个界面一个数);
 *  - 把 toggle 写到错的 CollabPolicy 字段上(表现"改 A 结果 B 变了",界面看不出);
 *  - MB 输入被当字节存(值差 2^20 倍,预算立即失真);
 *  - 老 daemon 的 UNIMPLEMENTED 被当成"坏了"渲染成故障,而不是入口隐藏。
 * 这些都是静默错误,只有单测能拦。
 */
class ControlPlaneLogicTest {

    /** 对照 daemon/internal/server/control_plane.go 的真实取值形态拼一份总览。 */
    private fun plane(): ControlPlane = ControlPlane.newBuilder()
        .setDaemonVersion("0.4.0")
        .setHostName("YANZI-PC")
        .setOs("Windows")
        .setUptimeUnixMs(1_000L)
        .setDataDirLabel("LOCALAPPDATA")
        .addDomains(
            ControlDomain.newBuilder()
                .setId("budget").setTitle("资源预算").setWritable(true).setStatusCode(0)
                .setSummary("全局预算已生效:并行轮次上限 3、事件缓冲 4 MB;改动立即生效,不必重启")
                .addItems(
                    ControlItem.newBuilder()
                        .setId("budget.global.ram_working_bytes").setLabel("工作内存上限")
                        .setCurrent("8 MB").setKind("number")
                        .setHint("单个轮次在内存里干活的额度;到线触发压缩,不丢会话内容。设为 0 表示封禁")
                )
                .addItems(
                    ControlItem.newBuilder()
                        .setId("budget.global.max_parallel_turns").setLabel("并行轮次上限")
                        .setCurrent("3").setKind("number").setHint("同时进行的轮次数量")
                )
        )
        .addDomains(
            ControlDomain.newBuilder()
                .setId("collab").setTitle("多 Agent 协同").setWritable(true).setStatusCode(0)
                .setSummary("协同已开启:委派与扇出可用,单次扇出上限 2 家")
                .addItems(
                    ControlItem.newBuilder()
                        .setId("collab.collaboration_enabled").setLabel("协同总开关")
                        .setCurrent("开").setKind("toggle")
                        .setHint("关掉后 Agent 之间不能再互相派活,各自会话不受影响")
                )
                .addItems(
                    ControlItem.newBuilder()
                        .setId("collab.native_delegate_enabled").setLabel("委派子任务")
                        .setCurrent("关").setKind("toggle").setHint("允许 Agent 把子任务派给别的 Agent 执行")
                )
                .addItems(
                    ControlItem.newBuilder()
                        .setId("collab.max_parallel_members").setLabel("扇出成员上限")
                        .setCurrent("2").setKind("number").setHint("单次扇出最多同时给几家;0 表示不限")
                )
        )
        .addDomains(
            ControlDomain.newBuilder()
                .setId("agents").setTitle("Agent 接入").setWritable(true)
                .setSummary("已接入 1 家 Agent,形态申报全部合格")
                .addItems(
                    ControlItem.newBuilder()
                        .setId("agents.codex.session").setLabel("Codex · 会话形态")
                        .setCurrent("原生自管").setKind("info")
                )
                .addItems(
                    // 桌面偏好项的契约形态(proto ControlItem:kind=select,options=候选,
                    // 由 daemon 给)—— 客户端路由必须按这份形态就绪:daemon 何时透出,
                    // 控制中心何时亮编辑入口,客户端不自带一份本地词表。
                    ControlItem.newBuilder()
                        .setId("agents.codex.approval_policy").setLabel("Codex · 审批策略")
                        .setCurrent("on-request").setKind("select")
                        .addOptions("untrusted").addOptions("on-request").addOptions("never")
                        .setHint("Agent 要执行敏感操作时先问你到什么程度")
                )
        )
        .addDomains(
            ControlDomain.newBuilder()
                .setId("memory").setTitle("会话记忆").setWritable(true).setStatusCode(0)
                .setSummary("记忆合计:2 个会话 / 7 条 / 12 KB")
                .addItems(
                    ControlItem.newBuilder()
                        .setId("memory.codex").setLabel("Codex 名下记忆")
                        .setCurrent("1 个会话 / 3 条 / 8 KB").setKind("info")
                )
                .addItems(
                    ControlItem.newBuilder()
                        .setId("memory.total").setLabel("全部 Agent 合计")
                        .setCurrent("2 个会话 / 7 条 / 12 KB").setKind("info")
                )
        )
        .addDomains(
            ControlDomain.newBuilder()
                .setId("dialysis").setTitle("透析台账").setWritable(true).setStatusCode(1)
                .setSummary("有 2 条没见过的原生事件等你查看")
        )
        .addDomains(
            ControlDomain.newBuilder()
                .setId("transports").setTitle("连接通道").setWritable(false).setStatusCode(0)
                .setSummary("局域网地址可用(192.168.1.8);Tailscale 未检测到")
                .addItems(
                    ControlItem.newBuilder()
                        .setId("transports.lan").setLabel("局域网直连")
                        .setCurrent("192.168.1.8").setKind("info")
                )
        )
        .build()

    // ---- 域 → 行模型映射 ------------------------------------------------------

    @Test
    fun `域次序与文案照_daemon_原文带出_不重排不改写`() {
        val rows = controlDomainRows(plane())
        // 次序是 daemon 钉死的(新增域在表尾);客户端重排 = 前后两次刷新版面换位。
        assertEquals(
            listOf("budget", "collab", "agents", "memory", "dialysis", "transports"),
            rows.map { it.id },
        )
        assertEquals("资源预算", rows[0].title)
        // summary 一字不改:两个界面看到同一个数,靠的是共用同一份读法。
        assertEquals(
            "局域网地址可用(192.168.1.8);Tailscale 未检测到",
            rows[5].summary,
        )
        assertEquals(true, rows[0].writable)
        assertEquals(false, rows[5].writable)
        assertEquals("工作内存上限", rows[0].items[0].label)
        assertEquals("8 MB", rows[0].items[0].current)
    }

    @Test
    fun `总览缺席时无行_不摆假骨架`() {
        // null = 整条 RPC 没成功,是"没问到";摆一排空行等于把失败画成内容。
        assertTrue(controlDomainRows(null).isEmpty())
    }

    @Test
    fun `状态码映射三档圆点_status2_摘要原文照显_不渲染成故障文案`() {
        // daemon 约定:2 可能只是"未装配/取不到",客户端显示 summary 原文即可,
        // 不自行把 2 渲染成"故障"——红点是小面积档位标记,文案仍是 daemon 的话。
        val degraded = ControlDomain.newBuilder()
            .setId("dialysis").setTitle("透析台账").setStatusCode(2)
            .setSummary("诊断台账未启用(运行时后端未装配)")
            .build()
        val row = controlDomainRows(
            ControlPlane.newBuilder().addDomains(degraded).build()
        ).single()
        assertEquals(ControlDotKind.LIMITED, row.dotKind)
        assertEquals("诊断台账未启用(运行时后端未装配)", row.summary)
        // 降级域可能连 items 都没有(panic 降级只保 id/title/summary),不能崩。
        assertTrue(row.items.isEmpty())

        assertEquals(ControlDotKind.NORMAL, controlDotKind(0))
        assertEquals(ControlDotKind.ATTENTION, controlDotKind(1))
    }

    @Test
    fun `写通道路由_collab_toggle_两域数值_agents桌面偏好_接通的才给写动作`() {
        val rows = controlDomainRows(plane())
        val budget = rows[0].items
        val collab = rows[1].items
        val agents = rows[2].items

        assertEquals(ControlItemAction.BUDGET_NUMBER, budget[0].action)
        assertEquals(ControlItemAction.BUDGET_NUMBER, budget[1].action)
        assertEquals(ControlItemAction.COLLAB_TOGGLE, collab[0].action)
        assertEquals(ControlItemAction.COLLAB_TOGGLE, collab[1].action)
        // collab 的数值(扇出成员上限)与 budget 数值同轮接通:走既有 SetCollabPolicy
        // (成员上限属协同策略树,与预算两套落盘两套 CAS,写错对象=改了不生效)。
        assertEquals(ControlItemAction.COLLAB_NUMBER, collab[2].action)
        // agents 域的桌面偏好 select(候选由 daemon 给)路由到既有 SetCodexSettings。
        assertEquals(ControlItemAction.AGENTS_SELECT, agents[1].action)
        // agents 域的事实项(会话形态)仍只读 —— 它们没有写语义。
        assertEquals(ControlItemAction.READ_ONLY, agents[0].action)
        // 只读域(writable=false)与别的 kind 也不给写动作。
        assertEquals(ControlItemAction.READ_ONLY, rows[5].items[0].action)
        // 候选缺席的 select:没有可选值的编辑入口是假入口,仍按只读呈现。
        val noOptions = ControlDomain.newBuilder()
            .setId("agents").setTitle("Agent 接入").setWritable(true)
            .addItems(
                ControlItem.newBuilder()
                    .setId("agents.codex.sandbox").setLabel("Codex · 沙箱")
                    .setCurrent("workspace-write").setKind("select")
            )
            .build()
        assertEquals(
            ControlItemAction.READ_ONLY,
            controlDomainRows(ControlPlane.newBuilder().addDomains(noOptions).build())
                .single().items.single().action,
        )
        // agents 路由只认 SetCodexSettings 的键词表:collab 域冒出的 select、agents 域里
        // 词表外的键,都没有对应写字段 —— 只读呈现,不摆假编辑入口。
        val unroutable = ControlPlane.newBuilder()
            .addDomains(
                ControlDomain.newBuilder()
                    .setId("collab").setTitle("多 Agent 协同").setWritable(true)
                    .addItems(
                        ControlItem.newBuilder()
                            .setId("collab.未来的下拉项").setLabel("某策略")
                            .setCurrent("甲").setKind("select").addOptions("甲").addOptions("乙")
                    )
            )
            .addDomains(
                ControlDomain.newBuilder()
                    .setId("agents").setTitle("Agent 接入").setWritable(true)
                    .addItems(
                        ControlItem.newBuilder()
                            .setId("agents.codex.native_mode").setLabel("Codex · 原生形态")
                            .setCurrent("x").setKind("select").addOptions("x").addOptions("y")
                    )
            )
            .build()
        val unroutableRows = controlDomainRows(unroutable)
        assertEquals(ControlItemAction.READ_ONLY, unroutableRows[0].items.single().action)
        assertEquals(ControlItemAction.READ_ONLY, unroutableRows[1].items.single().action)
    }

    @Test
    fun `行模型带_domainId_与_options_写路由参数不靠调用方再猜`() {
        val rows = controlDomainRows(plane())
        assertEquals("budget", rows[0].items[0].domainId)
        assertEquals("collab", rows[1].items[2].domainId)
        assertEquals("agents", rows[2].items[1].domainId)
        assertEquals(listOf("untrusted", "on-request", "never"), rows[2].items[1].options)
        // 事实项没有候选:空列表,不为它编一份词表。
        assertEquals(emptyList<String>(), rows[2].items[0].options)
    }

    // ---- toggle 现值与写路由 ----------------------------------------------------

    @Test
    fun `toggle_现值只认_daemon_开关原文_认不出就渲染为只读文本`() {
        // daemon 侧 toggle() 只产 "开"/"关";别的值(空/数字/英文)说明我们没读懂,
        // 返回 null 让 UI 显示原文 —— 猜一个布尔值就是造一份假账。
        assertEquals(true, controlToggleOn("开"))
        assertEquals(false, controlToggleOn("关"))
        assertNull(controlToggleOn("2"))
        assertNull(controlToggleOn(""))
        assertNull(controlToggleOn("on"))
    }

    @Test
    fun `collab_toggle_写路由_三个开关各映射到自己的字段且其余原样带回`() {
        // 与 RuntimeSectionLogicTest 的六键预算同款纪律:映射错位在界面上完全看不出来。
        val cases = linkedMapOf(
            "collab.collaboration_enabled" to 0,
            "collab.native_delegate_enabled" to 1,
            "collab.fanout_enabled" to 2,
        )
        cases.forEach { (itemId, expectedIndex) ->
            val b = CollabPolicy.newBuilder().setCollaborationEnabled(true)
            assertTrue("itemId=$itemId 应有写通道", applyControlToggle(b, itemId, false))
            val p = b.build()
            val flags = listOf(
                p.hasCollaborationEnabled() && !p.collaborationEnabled,
                p.hasNativeDelegateEnabled() && !p.nativeDelegateEnabled,
                p.hasFanoutEnabled() && !p.fanoutEnabled,
            )
            assertEquals("itemId=$itemId 应恰好动一个开关", 1, flags.count { it })
            assertEquals("itemId=$itemId 写错了开关", expectedIndex, flags.indexOf(true))
            // 其余项原样带回:collaboration_enabled 仍是原值 true。
            if (itemId != "collab.collaboration_enabled") assertTrue(p.collaborationEnabled)
        }
    }

    @Test
    fun `collab_toggle_未知键拒绝写_避免静默丢改动`() {
        val b = CollabPolicy.newBuilder()
        assertFalse(applyControlToggle(b, "collab.没听过的开关", true))
        assertFalse(applyControlToggle(b, "budget.global.ram_working_bytes", true))
        assertEquals(0, b.build().serializedSize)
    }

    // ---- budget 数值写路由 ------------------------------------------------------

    @Test
    fun `budget_number_写路由_MB_换算成字节且只动点中的那一项`() {
        val base = RuntimePolicy.newBuilder()
            .setGlobal(
                RuntimeLimits.newBuilder()
                    .setRamWorkingBytes(8L shl 20)
                    .setMaxParallelTurns(3)
            )
            .build()
        val b = base.toBuilder()
        // 控制中心输入单位是 MB(daemon 现值也是 MB 口径),存进字段前必须换成字节。
        assertTrue(applyControlBudgetNumber(b, "budget.global.disk_cache_bytes", 16))
        val g = b.build().global
        assertEquals(16L shl 20, g.diskCacheBytes)
        // 其余项原样带回 —— "提交完整策略"最容易丢的部分。
        assertEquals(8L shl 20, g.ramWorkingBytes)
        assertEquals(3, g.maxParallelTurns)
    }

    @Test
    fun `budget_number_计数键不换算_并行轮次原值入字段`() {
        val b = RuntimePolicy.newBuilder()
        assertTrue(applyControlBudgetNumber(b, "budget.global.max_parallel_turns", 5))
        assertEquals(5, b.build().global.maxParallelTurns)
    }

    @Test
    fun `budget_number_未知键与越域键拒绝写`() {
        val b = RuntimePolicy.newBuilder()
        assertFalse(applyControlBudgetNumber(b, "budget.global.没听过的键", 1))
        // collab 的成员上限不在预算树上(两套落盘与 CAS),路由必须拒收。
        assertFalse(applyControlBudgetNumber(b, "collab.max_parallel_members", 1))
        assertEquals(0, b.build().serializedSize)
    }

    // ---- 数字编辑预填 -----------------------------------------------------------

    @Test
    fun `数字预填_MB_现值与纯数字可回填_非_MB_单位不预填`() {
        assertEquals("8", controlNumberPrefill("8 MB"))
        assertEquals("3", controlNumberPrefill("3"))
        assertEquals("0", controlNumberPrefill("0"))
        // 8192 KB = 8 MB:输入框单位是 MB,把 "8192" 回填进去会放大 1024 倍 —— 宁可不填,
        // 让用户看着上方"当前值原文"重输。
        assertEquals("", controlNumberPrefill("8192 KB"))
        assertEquals("", controlNumberPrefill("12345 B"))
        assertEquals("", controlNumberPrefill("不限"))
        assertEquals("", controlNumberPrefill(""))
        // 带 MB 单位但非整数(理论不出现):不猜整数值。
        assertEquals("", controlNumberPrefill("1.5 MB"))
    }

    // ---- UNIMPLEMENTED 降级 -----------------------------------------------------

    @Test
    fun `UNIMPLEMENTED_判为老_daemon_其余失败不判`() {
        // 能力缺席的确定信号才隐藏入口;网络抖动照常按失败呈现,不与版本混为一谈。
        assertTrue(controlPlaneUnsupported(StatusRuntimeException(Status.UNIMPLEMENTED)))
        assertFalse(controlPlaneUnsupported(StatusRuntimeException(Status.UNAVAILABLE)))
        assertFalse(controlPlaneUnsupported(StatusRuntimeException(Status.INTERNAL)))
        assertFalse(controlPlaneUnsupported(RuntimeException("连接断了")))
    }

    // ---- 运行时长人话 ------------------------------------------------------------

    @Test
    fun `uptime_人话档位_now_注入可测`() {
        val now = 1_700_000_000_000L
        assertEquals("刚刚启动", uptimeDurationLabel(now, now))
        assertEquals("刚刚启动", uptimeDurationLabel(now - 59_999L, now))
        assertEquals("已运行 1 分钟", uptimeDurationLabel(now - 60_000L, now))
        assertEquals("已运行 45 分钟", uptimeDurationLabel(now - 45 * 60_000L, now))
        assertEquals("已运行 3 小时", uptimeDurationLabel(now - 3 * 3_600_000L, now))
        assertEquals("已运行 23 小时", uptimeDurationLabel(now - 23 * 3_600_000L, now))
        assertEquals("已运行 2 天", uptimeDurationLabel(now - 2 * 86_400_000L, now))
        // 手机时钟偏慢(启动时刻在未来):回落"刚刚启动",不显示负数。
        assertEquals("刚刚启动", uptimeDurationLabel(now + 60_000L, now))
        // 启动时刻缺席(0/负):返回空串,UI 整段不显示,不编一个"已运行 0 天"。
        assertEquals("", uptimeDurationLabel(0L, now))
    }

    // ---- 入口行与事实行 ----------------------------------------------------------

    @Test
    fun `入口行事实串_版本_主机_时长_缺了就不摆`() {
        val now = 1_700_000_000_000L
        // 夹具的 uptimeUnixMs 是占位(epoch),这里注入"now-3h 的启动时刻"再验时长档。
        val started = plane().toBuilder().setUptimeUnixMs(now - 3 * 3_600_000L).build()
        assertEquals(
            "YANZI-PC · v0.4.0 · 已运行 3 小时",
            controlPlaneEntryDesc(started, now),
        )
        // 主机名缺席:不以分隔符开头/结尾。
        val noHost = started.toBuilder().setHostName("").build()
        assertEquals(
            "v0.4.0 · 已运行 3 小时",
            controlPlaneEntryDesc(noHost, now),
        )
    }

    @Test
    fun `身份事实行_系统与数据目录标签_原样带出`() {
        assertEquals(
            "Windows · 数据目录:LOCALAPPDATA",
            controlPlaneFactsLine(plane()),
        )
        assertEquals("", controlPlaneFactsLine(ControlPlane.getDefaultInstance()))
    }

    // ---- memory 域的弹窗入口 ------------------------------------------------------

    @Test
    fun `memory_弹窗入口取第一个按_agent_的项_total_不算`() {
        val memory = controlDomainRows(plane()).first { it.id == "memory" }
        assertEquals("codex", memoryAgentTargetOf(memory.items))
        // 只有合计项(还没有 Agent 接入)时没有弹窗目标 —— 入口禁用,不盲开一个空 agent。
        val totalOnly = ControlDomain.newBuilder()
            .setId("memory").setTitle("会话记忆").setWritable(true)
            .addItems(
                ControlItem.newBuilder().setId("memory.total").setLabel("全部 Agent 合计")
                    .setCurrent("0 个会话 / 0 条 / 0 B").setKind("info")
            )
            .build()
        assertNull(
            memoryAgentTargetOf(
                controlDomainRows(ControlPlane.newBuilder().addDomains(totalOnly).build())
                    .single().items
            )
        )
    }

    // ---- agents 域:桌面偏好 → SetCodexSettings 路由 ------------------------------

    @Test
    fun `agents_路由拆解_设置键从最右一段拆_agentId_原样带出`() {
        assertEquals(ControlAgentsTarget("codex", "approval_policy"), controlAgentsTarget("agents.codex.approval_policy"))
        assertEquals(ControlAgentsTarget("zcode", "model"), controlAgentsTarget("agents.zcode.model"))
        // agentId 本身含点号也不怕:设置键永远从最右一段拆(键词表钉死,不与 id 形状互相猜)。
        assertEquals(ControlAgentsTarget("a.b", "effort"), controlAgentsTarget("agents.a.b.effort"))
        // 键不在词表 / 不是 agents 前缀 / 缺 agentId:一律不路由 —— 路由表外放行就是静默丢改动。
        assertNull(controlAgentsTarget("collab.max_parallel_members"))
        assertNull(controlAgentsTarget("budget.global.ram_working_bytes"))
        assertNull(controlAgentsTarget("agents..sandbox"))
        assertNull(controlAgentsTarget("agents.codex"))
        // bool 键(隐藏推理/粘贴爆破)没有 select-with-options 的写契约:select 的候选是
        // 字符串,把"开/关"猜成 true/false 就是造一份假账 —— 保持只读,等它的呈现契约。
        assertNull(controlAgentsTarget("agents.codex.hide_agent_reasoning"))
        assertNull(controlAgentsTarget("agents.codex.disable_paste_burst"))
    }

    private fun agentsReqOf(key: String, value: String): SetCodexSettingsRequest {
        val b = SetCodexSettingsRequest.newBuilder()
        assertTrue("key=$key 应有写通道", applyControlAgentsSetting(b, key, value))
        return b.build()
    }

    @Test
    fun `agents_写路由_六键各写各的字段且恰好一个_presence_纪律`() {
        // 与 RuntimeSectionLogicTest 的 collab 三开关同款纪律:映射错位在界面上完全看不出来。
        val cases = linkedMapOf(
            "approval_policy" to Pair("never") { r: SetCodexSettingsRequest -> r.hasApprovalPolicy() && r.approvalPolicy == "never" },
            "sandbox" to Pair("workspace-write") { r: SetCodexSettingsRequest -> r.hasSandbox() && r.sandbox == "workspace-write" },
            "model" to Pair("codex/gpt-5.1") { r: SetCodexSettingsRequest -> r.hasModel() && r.model == "codex/gpt-5.1" },
            "effort" to Pair("high") { r: SetCodexSettingsRequest -> r.hasEffort() && r.effort == "high" },
            "model_verbosity" to Pair("low") { r: SetCodexSettingsRequest -> r.hasModelVerbosity() && r.modelVerbosity == "low" },
            "model_reasoning_summary" to Pair("concise") { r: SetCodexSettingsRequest -> r.hasModelReasoningSummary() && r.modelReasoningSummary == "concise" },
        )
        cases.forEach { (key, case) ->
            val (value, hit) = case
            val r = agentsReqOf(key, value)
            val presence = listOf(
                r.hasApprovalPolicy(), r.hasSandbox(), r.hasModel(), r.hasEffort(),
                r.hasModelVerbosity(), r.hasModelReasoningSummary(),
            )
            assertEquals("key=$key 应恰好 set 一个字段(缺席=不改,presence 是请求的唯一语义)", 1, presence.count { it })
            assertTrue("key=$key 写错了字段或值被加工", hit(r))
        }
    }

    @Test
    fun `agents_写路由_空串是跟随桌面的合法取值_必须显式set而不是缺席`() {
        // proto 钉死的教训:默认模型"跟随桌面"写的是空串 —— 缺席=不改,用户就永远
        // 回不到"跟随桌面"。六个字符串键一律显式 set,空串照写。
        val r = agentsReqOf("model", "")
        assertTrue(r.hasModel())
        assertEquals("", r.model)
        val r2 = agentsReqOf("approval_policy", "")
        assertTrue(r2.hasApprovalPolicy())
        assertEquals("", r2.approvalPolicy)
    }

    @Test
    fun `agents_写路由_词表外的键拒绝_避免静默丢改动`() {
        val b = SetCodexSettingsRequest.newBuilder()
        assertFalse(applyControlAgentsSetting(b, "native_mode", "x"))
        assertFalse(applyControlAgentsSetting(b, "hide_agent_reasoning", "true"))
        assertFalse(applyControlAgentsSetting(b, "没听过的键", "x"))
        // 一个字段都没动:拒写的请求必须原样空着,不能带半个脏字段出去。
        assertEquals(0, b.build().serializedSize)
    }

    @Test
    fun `agents_值域校验_只放行_daemon_候选内的值_拒绝给原因`() {
        val options = listOf("untrusted", "on-request", "never")
        assertNull(controlAgentsOptionInvalid("on-request", options))
        // 枚举写错档的脏值,客户端先拦下并给一句原因(daemon 原文之外不编词)。
        val bad = controlAgentsOptionInvalid("yolo", options)
        assertTrue(bad != null && bad.isNotBlank())
        // 候选缺席(空列表):没有任何可写值,写什么都是拒。
        assertTrue(controlAgentsOptionInvalid("never", emptyList()) != null)
        // 空串必须真的在候选里才算数:候选没给"跟随桌面"就不许空提交。
        assertTrue(controlAgentsOptionInvalid("", options) != null)
        val withFollow = listOf("", "untrusted")
        assertNull(controlAgentsOptionInvalid("", withFollow))
    }

    // ---- collab 域:数值(扇出成员上限)→ SetCollabPolicy 路由 ----------------------

    @Test
    fun `collab_number_写路由_成员上限进协同策略_其余键拒绝`() {
        val base = CollabPolicy.newBuilder().setCollaborationEnabled(true).setMaxParallelMembers(2).build()
        val b = base.toBuilder()
        assertTrue(applyControlCollabNumber(b, "collab.max_parallel_members", 5))
        val p = b.build()
        assertEquals(5, p.maxParallelMembers)
        // 其余项原样带回 —— "提交完整策略"最容易丢的部分。
        assertTrue(p.collaborationEnabled)
        // 0 是合法取值(daemon 语义:0=不限),必须显式落字段,不是"清掉"。
        val b0 = base.toBuilder()
        assertTrue(applyControlCollabNumber(b0, "collab.max_parallel_members", 0))
        val p0 = b0.build()
        assertEquals(0, p0.maxParallelMembers)
        assertTrue(p0.hasMaxParallelMembers())
        // 预算键不属于协同树(两套落盘与 CAS),越域拒绝;collab 的开关也不是数值通道。
        val empty = CollabPolicy.newBuilder()
        assertFalse(applyControlCollabNumber(empty, "budget.global.ram_working_bytes", 1))
        assertFalse(applyControlCollabNumber(empty, "collab.collaboration_enabled", 1))
        assertEquals(0, empty.build().serializedSize)
    }

    // ---- 数值输入框的单位与域行展开判定 ---------------------------------------------

    @Test
    fun `数值单位_并行轮次路_成员家_字节键MB_单位错就是口径错`() {
        assertEquals("路", controlNumberUnit("budget.global.max_parallel_turns"))
        assertEquals("家", controlNumberUnit("collab.max_parallel_members"))
        assertEquals("MB", controlNumberUnit("budget.global.ram_working_bytes"))
        assertEquals("MB", controlNumberUnit("budget.global.disk_cache_bytes"))
    }

    @Test
    fun `域行展开判定_域内有接通写通道的项才可展开`() {
        val rows = controlDomainRows(plane())
        // budget / collab / agents(含偏好项)都有接通的写通道,可展开。
        assertTrue(controlDomainExpandable(rows[0].items))
        assertTrue(controlDomainExpandable(rows[1].items))
        assertTrue(controlDomainExpandable(rows[2].items))
        // 纯事实的 memory 与只读 transports:一项写通道都没有,不摆展开箭头 ——
        // agents 域的事实项已由上方形态卡带出,展开体只剩只读行时是假编辑入口。
        assertFalse(controlDomainExpandable(rows[3].items))
        assertFalse(controlDomainExpandable(rows[5].items))
        // panic 降级域连 items 都没有:不可展开。
        assertFalse(controlDomainExpandable(emptyList()))
    }
}
