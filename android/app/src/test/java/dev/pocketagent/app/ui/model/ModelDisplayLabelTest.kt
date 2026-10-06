package dev.pocketagent.app.ui.model

import org.junit.Assert.assertEquals
import org.junit.Test
import pocketagent.v1.ModelInfo

/**
 * 统一翻译层 modelDisplayLabel 的纯函数钉子(L2 真机发现 5 后半:Hermes/Pi 会话
 * 把 daemon 未带展示名的模型以裸 id 直接上屏,如 "hermes/deepseek-flash")。
 *
 * 解析顺序钉死:申报表 displayName → 大表 displayName → 裸名(无前缀 id 原样,
 * 兜底即原 id)。数据错了的表现是快速面板/头部把裸 id 亮给用户,无声,故钉死。
 */
class ModelDisplayLabelTest {

    private fun model(id: String, name: String = ""): ModelInfo =
        ModelInfo.newBuilder().setId(id).setDisplayName(name).build()

    @Test
    fun `申报表 displayName 优先`() {
        val scoped = listOf(model("hermes/deepseek-flash", "Hermes · DeepSeek Flash"))
        assertEquals("Hermes · DeepSeek Flash", modelDisplayLabel("hermes/deepseek-flash", scoped))
    }

    @Test
    fun `申报表 displayName 为空串时退大表 displayName`() {
        val scoped = listOf(model("hermes/deepseek-flash")) // daemon 会话级回包缺展示名
        val global = listOf(model("hermes/deepseek-flash", "DeepSeek Flash"))
        assertEquals("DeepSeek Flash", modelDisplayLabel("hermes/deepseek-flash", scoped, global))
    }

    @Test
    fun `id 只在大表时也翻得出人话名`() {
        // 正在跑的模型不在申报表里(如被真值校正回退的旧模型):大表补位
        val scoped = listOf(model("pi/deepseek/deepseek-v4-pro", "DeepSeek V4 Pro"))
        val global = listOf(model("pi/zai-coding-cn/glm-5.3-flash", "GLM-5.3-Flash"))
        assertEquals(
            "GLM-5.3-Flash",
            modelDisplayLabel("pi/zai-coding-cn/glm-5.3-flash", scoped, global),
        )
    }

    @Test
    fun `两表都无该 id 时退去前缀裸名`() {
        assertEquals("deepseek-flash", modelDisplayLabel("hermes/deepseek-flash", emptyList()))
    }

    @Test
    fun `无前缀 id 兜底即原 id 原样`() {
        assertEquals("deepseek-flash", modelDisplayLabel("deepseek-flash", emptyList()))
    }

    @Test
    fun `三段式 id 只去首段与 modelBareName 同口径`() {
        val global = listOf(model("pi/zai-coding-cn/glm-5.3-flash", "GLM-5.3-Flash"))
        // displayName 命中时不做任何前缀处理
        assertEquals("GLM-5.3-Flash", modelDisplayLabel("pi/zai-coding-cn/glm-5.3-flash", emptyList(), global))
        // 无 displayName:只去首段 "pi/",余下原样保留(不猜第二段语义)
        assertEquals("zai-coding-cn/glm-5.3-flash", modelDisplayLabel("pi/zai-coding-cn/glm-5.3-flash", emptyList()))
    }

    @Test
    fun `空串 displayName 不算翻译成功必须继续兜底`() {
        // daemon 给了空串展示名:不许显示空,也不许拿空串当翻译结果
        val scoped = listOf(model("hermes/deepseek-flash", ""))
        val global = listOf(model("hermes/deepseek-flash", " "))
        assertEquals("deepseek-flash", modelDisplayLabel("hermes/deepseek-flash", scoped, global))
    }

    @Test
    fun `secondary 缺省时单表调用合法`() {
        val scoped = listOf(model("pi/glm-5.3", "GLM-5.3"))
        assertEquals("GLM-5.3", modelDisplayLabel("pi/glm-5.3", scoped))
        assertEquals("glm-5.3", modelDisplayLabel("pi/glm-5.3", emptyList()))
    }
}
