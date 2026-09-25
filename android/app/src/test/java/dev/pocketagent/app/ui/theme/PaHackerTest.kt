package dev.pocketagent.app.ui.theme

import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 黑客终端主题基座的纯逻辑钉子(JVM)。
 *
 * PaTheme/Pa 是 Compose 状态对象,渲染侧不进 JVM;这里只钉 [PaHacker] 消费契约:
 * hacker 时主对话该拿到的"等宽字体 + 6dp 小圆角"必须成立,换回常规主题后必须
 * 释放(回归钉:防止有人把圆角改成无条件 6dp,让深色主题也变终端风)。
 */
class PaHackerTest {

    @Test
    fun `hacker mode pins monospace font and tight bubble corner`() {
        AgentTheme.mode = "hacker"
        assertTrue(PaHacker.isHacker)
        assertEquals(FontFamily.Monospace, PaHacker.mono)
        assertEquals(6.dp, PaHacker.bubbleCorner)
        // 函数形态契约:无论消费方默认值是什么,hacker 一律压成 6dp
        assertEquals(6.dp, PaHacker.bubbleCornerFor(20.dp))
        assertEquals(6.dp, PaHacker.bubbleCornerFor(26.dp))
    }

    @Test
    fun `switching back to dark releases the hacker pin`() {
        AgentTheme.mode = "dark"
        assertFalse(PaHacker.isHacker)
        // 回归钉:常规主题圆角回到默认(20dp=ChatScreen 用户气泡主圆角),绝不能是 6dp
        assertNotEquals(6.dp, PaHacker.bubbleCorner)
        assertEquals(20.dp, PaHacker.bubbleCorner)
        // 函数形态:不 hacker 时原样返回消费方默认,不接线不破坏常规主题
        assertEquals(18.dp, PaHacker.bubbleCornerFor(18.dp))
    }

    @Test
    fun `mono constant and mode id stay stable as contract`() {
        // mono 是恒定常量(消费方用 isHacker 决定是否采用),不随模式变
        assertEquals(FontFamily.Monospace, PaHacker.mono)
        // 模式串与 SettingsScreen 下拉项/持久化字符串一致,改名即断链
        assertEquals("hacker", PaHacker.MODE)
        assertEquals(0.95f, PaHacker.chatFontSizeStep)
    }
}
