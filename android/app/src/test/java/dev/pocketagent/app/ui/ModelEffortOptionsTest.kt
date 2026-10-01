package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ModelInfo

/**
 * 思考强度档位选择(effortsFor / effortLabelOf)的纯函数回归:
 * daemon 在 ModelInfo.available_efforts 透出动态档位(pi)时按它渲染,
 * 留空(codex/zcode)时回内置默认档 EFFORTS。
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
}
