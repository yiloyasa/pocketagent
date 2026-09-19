package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * 主页「多agent协同」开关开发中闸门的单测:
 * 功能未实现,任何"打开"请求都不生效(状态恒为关),打开请求带"开发中"提示,
 * 关闭请求静默 —— 锁定"不许假装可用"这条产品红线。
 */
class MultiAgentToggleTest {

    @Test
    fun `打开请求不生效并提示开发中`() {
        val (next, toast) = multiAgentToggleResult(true)
        assertEquals(false, next)
        assertEquals(MULTI_AGENT_HINT_DEV, toast)
    }

    @Test
    fun `关闭请求静默保持关`() {
        val (next, toast) = multiAgentToggleResult(false)
        assertEquals(false, next)
        assertNull(toast)
    }

    @Test
    fun `提示文案说明功能尚未开放`() {
        // 文案必须讲真话:"开发中" + 尚未开放,不能出现"已开启/可用"类假可用字样
        assert(MULTI_AGENT_HINT_DEV.contains("开发中"))
        assert(MULTI_AGENT_HINT_DEV.contains("尚未开放"))
    }
}
