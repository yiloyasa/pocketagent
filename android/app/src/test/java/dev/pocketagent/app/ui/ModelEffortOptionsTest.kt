package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.model.SelectionSource
import dev.pocketagent.app.ui.model.cardEffortLevels
import dev.pocketagent.app.ui.model.cardSelectedEffort
import dev.pocketagent.app.ui.model.modelRowDetail
import dev.pocketagent.app.ui.model.selectionSourceLabel
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ModelInfo

/**
 * 思考强度档位选择(effortsFor / effortLabelOf)的纯函数回归:
 * daemon 在 ModelInfo.available_efforts 透出动态档位(pi)时按它渲染,
 * 留空(codex/zcode)时回内置默认档 EFFORTS。
 *
 * 【P3-T4 同步】模型/强度卡拆到 ui/model 后,卡面新增的纯函数一并钉在这里
 * (照 ModelSelectionReducerTest 风格:每条用例对应一条计划验收规则):
 *  - [cardEffortLevels]:daemon 真值档位优先,缺失退 effortsFor;
 *  - [cardSelectedEffort]:失效 effort 展示层清理(先清理再显示可选档位),
 *    无法判定保留原值不猜,未知 effort 保留原 id;
 *  - [modelRowDetail]:窄屏可读的能力副文案(上下文/思考/图片/可用性);
 *  - [selectionSourceLabel]:来源人话 + 词表外原文降级。
 */
class ModelEffortOptionsTest {

    private fun model(id: String, provider: String, vararg efforts: String): ModelInfo =
        ModelInfo.newBuilder()
            .setId(id)
            .setProvider(provider)
            .addAllAvailableEfforts(efforts.toList())
            .build()

    @Test
    fun dynamic_levels_render_for_selected_pi_model() {
        val models = listOf(
            model("codex/gpt-5.4", "codex"),
            model("pi/zai-coding-cn/glm-5.3", "pi", "low", "high", "max"),
        )
        val got = effortsFor(models, "pi/zai-coding-cn/glm-5.3", "pi")
        // 首位恒是"跟随桌面"(空 id),其后按 daemon 透出的顺序渲染
        assertEquals(listOf("", "low", "high", "max"), got.map { it.id })
        assertEquals("默认", got[0].label)
        assertEquals("低", got[1].label)
        assertEquals("高", got[2].label)
        assertEquals("最大", got[3].label)
    }

    @Test
    fun empty_efforts_fall_back_to_builtin_defaults() {
        val models = listOf(
            model("codex/gpt-5.4", "codex"),
            model("pi/zai-coding-cn/glm-5.3", "pi"), // 无动态档位
        )
        val got = effortsFor(models, "codex/gpt-5.4", "codex")
        // 【2026-10-02 修复】注释承诺"回内置默认档 EFFORTS",实现此前却只剩"默认"
        // 一枚胶囊 = codex/zcode/dsh/hermes 的强度选择实际瘫痪。修复后按内置档渲染。
        assertEquals(listOf("", "low", "medium", "high", "xhigh"), got.map { it.id })
        assertEquals(listOf("默认", "低", "中", "高", "极高"), got.map { it.label })
    }

    @Test
    fun follow_desktop_on_pi_thread_follows_agent_levels() {
        val models = listOf(
            model("codex/gpt-5.4", "codex"),
            model("pi/zai-coding-cn/glm-5.3", "pi", "off", "minimal", "low", "high"),
        )
        // "跟随桌面配置"(模型空串)+ 会话归属 pi(档位制 agent):按 pi 的档位渲染
        val got = effortsFor(models, "", "pi")
        assertEquals(listOf("", "off", "minimal", "low", "high"), got.map { it.id })
    }

    @Test
    fun pi_unknown_model_shows_default_only_no_fabrication() {
        // 档位制 agent(pi)的**未知模型**没带档位 → 只显示"默认",不伪造
        // (RuntimeStateRegressionTest 同语义,两处一致)。
        val models = listOf(
            model("pi/zai-coding-cn/glm-5.3", "pi", "off", "low"),
            model("pi/private/custom-v999", "pi"),
        )
        val got = effortsFor(models, "pi/private/custom-v999", "pi")
        assertEquals(listOf(""), got.map { it.id })
    }

    @Test
    fun unknown_level_label_falls_back_to_raw_id() {
        assertEquals("默认", effortLabelOf(""))
        assertEquals("关", effortLabelOf("off"))
        assertEquals("极低", effortLabelOf("minimal"))
        assertEquals("极高", effortLabelOf("xhigh"))
        assertEquals("最大", effortLabelOf("max"))
        // daemon 未来新增档位:显示原 id,不显示空白
        assertEquals("ultra", effortLabelOf("ultra"))
    }

    @Test
    fun dynamic_labels_never_blank() {
        val got = effortsFor(
            listOf(model("pi/m", "pi", "off", "medium", "max")),
            "pi/m",
            "pi",
        )
        assertTrue(got.all { it.label.isNotBlank() })
    }

    // ---- P3-T4:cardEffortLevels(daemon 真值档位优先,缺失退 effortsFor)----

    @Test
    fun card_levels_prefer_daemon_truth_and_always_prepend_default() {
        val models = listOf(model("pi/glm-5.3", "pi", "low", "high"))
        // daemon 真值给了档位(去重去空,顺序保持 daemon 序):按它渲染,首位恒补"默认"
        val got = cardEffortLevels(models, "pi", "pi/glm-5.3", listOf("max", "low", "low", ""))
        assertEquals(listOf("", "max", "low"), got.map { it.id })
        assertEquals("默认", got[0].label)
        assertEquals("最大", got[1].label)
        assertEquals("低", got[2].label)
    }

    @Test
    fun card_levels_fall_back_to_effortsfor_when_daemon_silent() {
        val models = listOf(model("pi/glm-5.3", "pi", "low", "high", "max"))
        // daemon 未给 available_efforts(上次记录缓存/旧 daemon):退 effortsFor 同一结论
        assertEquals(
            effortsFor(models, "pi/glm-5.3", "pi").map { it.id },
            cardEffortLevels(models, "pi", "pi/glm-5.3", emptyList()).map { it.id },
        )
        // codex(从不透出档位)退内置默认档
        val codex = listOf(model("codex/gpt-5.4", "codex"))
        assertEquals(
            listOf("", "low", "medium", "high", "xhigh"),
            cardEffortLevels(codex, "codex", "codex/gpt-5.4", emptyList()).map { it.id },
        )
    }

    // ---- P3-T4:cardSelectedEffort(失效 effort 先清理再显示;不猜;未知保留原 id)----

    @Test
    fun card_selected_effort_cleans_stale_value_before_display() {
        // daemon 真值档位集合不含当前强度 → 高亮清成"默认"(跟随默认),不显示失效选中
        assertEquals("", cardSelectedEffort(emptyList(), "pi", "pi/glm-5.3", "xhigh", listOf("low", "high", "max")))
        // 在集合里 → 原样保留
        assertEquals("high", cardSelectedEffort(emptyList(), "pi", "pi/glm-5.3", "high", listOf("low", "high", "max")))
        // 真值没给集合:按模型表判定 —— pi 档位制 agent 的已知模型没带档位 → 失效强度清
        val piFamily = listOf(model("pi/glm-5.3", "pi", "low", "high"), model("pi/private/x", "pi"))
        assertEquals("", cardSelectedEffort(piFamily, "pi", "pi/private/x", "max", emptyList()))
        // 非档位 agent(codex 系)→ 内置默认档,xhigh 合法保留
        val codex = listOf(model("codex/gpt-5.4", "codex"))
        assertEquals("xhigh", cardSelectedEffort(codex, "codex", "codex/gpt-5.4", "xhigh", emptyList()))
    }

    @Test
    fun card_selected_effort_keeps_value_when_undecidable_no_guessing() {
        // "跟随默认"(模型空):档位由 agent 当前模型决定,客户端无法判定 → 保留不猜
        assertEquals("max", cardSelectedEffort(emptyList(), "pi", "", "max", emptyList()))
        // 未知模型且无任何档位原料 → 保留原值,等 daemon 真值回填校正(与 cleanEffortFor 同规则)
        assertEquals("high", cardSelectedEffort(emptyList(), "pi", "pi/unknown", "high", emptyList()))
        // 未知 effort(词表外):daemon 真值给了它 → 保留原 id(展示层 else -> id,不空白)
        assertEquals("ultra", cardSelectedEffort(emptyList(), "pi", "pi/glm-5.3", "ultra", listOf("low", "ultra")))
        // 空强度 = 跟随默认,原样
        assertEquals("", cardSelectedEffort(emptyList(), "pi", "pi/glm-5.3", "", listOf("low")))
    }

    // ---- P3-T4:modelRowDetail(窄屏可读的能力副文案)----

    @Test
    fun model_row_detail_reports_availability_first() {
        // 不可用:一句话说清,不再罗列能力("能不能用"优先于"有多强")
        assertEquals("暂不可用(电脑端未连通或未配置)", modelRowDetail(1_000_000, true, true, available = false))
    }

    @Test
    fun model_row_detail_composes_known_capabilities_only() {
        assertEquals("上下文 1000k · 支持思考 · 支持图片", modelRowDetail(1_000_000, true, true, true))
        assertEquals("上下文 258k · 支持思考", modelRowDetail(258_400, true, false, true))
        // 窗口未知保持未知:不显示"0k"假数字
        assertEquals("支持思考", modelRowDetail(0, true, false, true))
        assertEquals("", modelRowDetail(0, false, false, true))
    }

    // ---- P3-T4:selectionSourceLabel(来源人话 + 词表外原文降级)----

    @Test
    fun selection_source_labels_are_human_readable() {
        assertEquals("本会话选择", selectionSourceLabel(SelectionSource.THREAD_OVERRIDE, "thread_override"))
        assertEquals("App 默认", selectionSourceLabel(SelectionSource.APP_DEFAULT, "app_default"))
        assertEquals("桌面默认", selectionSourceLabel(SelectionSource.DESKTOP_DEFAULT, "desktop_default"))
        assertEquals("Agent 当前值", selectionSourceLabel(SelectionSource.NATIVE_CURRENT, "native_current"))
        // 词表外新增来源:不报错,如实展示 daemon 原文;连原文都没有才落"未知来源"
        assertEquals("future_source", selectionSourceLabel(SelectionSource.UNKNOWN, "future_source"))
        assertEquals("未登记来源", selectionSourceLabel(SelectionSource.UNKNOWN, ""))
    }
}
