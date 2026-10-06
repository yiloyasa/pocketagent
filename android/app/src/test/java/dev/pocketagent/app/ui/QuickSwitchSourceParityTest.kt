package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.model.cardEffortLevels
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ModelInfo

/**
 * L2 真机发现 5(前半)钉子:快捷切换面板与完整模型卡(ui/model.ModelEffortCard)
 * 的模型清单必须同源 —— 面板与卡都从「仓库申报表 × modelsForAgent」取数。
 *
 * 【根因记录】面板旧实现读本页全局大表(daemon 不带 agent 过滤的 ListModels 回包
 * + 巡检合并),而巡检合并的过滤键是 provider 字段:大表里 pi 系模型的 provider 是
 * 上游供应商(如 zai-coding-cn)而非 "pi",永远滤不掉 —— 于是 pi 会话面板 6 个
 * 模型、完整卡按申报 2 个。本测试用同一形状的两张表把"新源一致 / 旧源膨胀 /
 * 档位跟着源走"三条规则钉死(接线在 Compose 层,JVM 侧钉数据规则)。
 */
class QuickSwitchSourceParityTest {

    private fun model(id: String, provider: String, name: String = "", vararg efforts: String): ModelInfo =
        ModelInfo.newBuilder()
            .setId(id)
            .setProvider(provider)
            .setDisplayName(name)
            .addAllAvailableEfforts(efforts.toList())
            .build()

    /** daemon 不带 agent 过滤的全 agent 大表:pi 系 provider 是上游供应商(L2 实锤形状)。 */
    private val globalTable = listOf(
        model("codex/gpt-5.4", "codex", "GPT-5.4"),
        model("pi/deepseek/deepseek-v4-flash", "deepseek", "DeepSeek V4.1 Flash", "off", "low", "high", "max"),
        model("pi/deepseek/deepseek-v4-pro", "deepseek", "DeepSeek V4 Pro", "off", "low", "high", "max"),
        model("pi/zai-coding-cn/glm-4.6v", "zai-coding-cn", "GLM-4.6V"),
        model("pi/zai-coding-cn/glm-5.3", "zai-coding-cn", "GLM-5.3"),
        model("pi/zai-coding-cn/glm-5.3-flash", "zai-coding-cn", "GLM-5.3-Flash"),
        model("pi/zai-coding-cn/glm-5.3-highspeed", "zai-coding-cn", "GLM-5.3 Highspeed"),
    )

    /** daemon 会话级申报表(listModelsWithSelection 按 agent/thread 收窄):provider=agent。 */
    private val scopedTable = listOf(
        model("pi/default", "pi", "Pi(跟随 Pi 当前配置)"),
        model("pi/deepseek/deepseek-v4-flash", "pi", "DeepSeek V4.1 Flash", "off", "low", "high", "max"),
        model("pi/deepseek/deepseek-v4-pro", "pi", "DeepSeek V4 Pro", "off", "low", "high", "max"),
    )

    @Test
    fun `面板与卡同源时模型清单一致`() {
        // 两卡同一条取数式:modelsForAgent(仓库表, 会话 agent)
        val cardModels = modelsForAgent(scopedTable, "pi")
        val panelModels = ModelQuickSwitchLogic.quickModels("pi", cardModels)
        // 卡面非 default 条目 = 面板条目(default 的语义在卡里有专门行,面板不承载)
        assertEquals(
            cardModels.filter { !it.id.endsWith("/default") }.map { it.id },
            panelModels.map { it.id },
        )
    }

    @Test
    fun `旧全局大表做源时面板膨胀到全 agent 表(L2 观察到的 6 模型形状)`() {
        // 对照钉:同一 quickModels 函数,喂旧源(prefix 过滤挡不住 provider=上游的行)
        // 就得到 6 个 —— 证明修的是"源"不是"面板算法",防回改。
        val staleSource = modelsForAgent(globalTable, "pi")
        val got = ModelQuickSwitchLogic.quickModels("pi", staleSource)
        assertEquals(6, got.size)
    }

    @Test
    fun `申报表外的 agent 模型不串进面板`() {
        val cardModels = modelsForAgent(scopedTable, "pi")
        val got = ModelQuickSwitchLogic.quickModels("pi", cardModels)
        assertTrue(got.none { it.id.startsWith("codex/") })
        assertTrue(got.none { it.id.endsWith("/default") })
    }

    @Test
    fun `档位跟着源走_申报表给 pi 动态全集与卡一致`() {
        val unified = modelsForAgent(scopedTable, "pi")
        val panel = cardEffortLevels(
            models = unified,
            agentId = "pi",
            modelId = "pi/deepseek/deepseek-v4-pro",
            daemonEfforts = emptyList(),
        )
        // provider=pi 的申报行动态档位被 effortsFor 认出:首位恒补"默认"
        assertEquals(listOf("", "off", "low", "high", "max"), panel.map { it.id })
    }

    @Test
    fun `档位跟着源走_旧全局大表 provider 不匹配只得内置档(根因对照)`() {
        val stale = modelsForAgent(globalTable, "pi")
        val got = effortsFor(stale, "pi/deepseek/deepseek-v4-pro", "pi")
        // 大表 provider=上游供应商,effortsFor 的 agent 匹配落空 → 退内置五档
        // (L2 截图 06a 的"默认/低/中/高/极高"正是它,而完整卡同时显示 pi 动态集)
        assertEquals(listOf("", "low", "medium", "high", "xhigh"), got.map { it.id })
    }

    @Test
    fun `daemon 真值档位优先于两表(与卡同一条 cardEffortLevels 口径)`() {
        val unified = modelsForAgent(scopedTable, "pi")
        val got = cardEffortLevels(
            models = unified,
            agentId = "pi",
            modelId = "pi/deepseek/deepseek-v4-pro",
            daemonEfforts = listOf("minimal", "max"),
        )
        assertEquals(listOf("", "minimal", "max"), got.map { it.id })
    }

    @Test
    fun `仓库申报表为空时面板与卡一同空等刷新`() {
        val cardModels = modelsForAgent(emptyList<ModelInfo>(), "pi")
        assertTrue(cardModels.isEmpty())
        assertTrue(ModelQuickSwitchLogic.quickModels("pi", cardModels).isEmpty())
        // 面板空态文案据此判 cardModels.isEmpty():与卡"Agent 暂未报告模型,请刷新"同真值
    }
}
