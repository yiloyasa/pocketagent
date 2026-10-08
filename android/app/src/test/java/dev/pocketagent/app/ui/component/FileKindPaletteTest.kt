package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.theme.AgentTheme
import dev.pocketagent.app.ui.theme.PaEditorial
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test

/**
 * D7:FileKind 徽标色的换盘刷新钉子。
 *
 * 【钉什么】Mcp 三族(IMAGE/AUDIO/VIDEO)的 [FileKind.color] 必须**组合期(用时)**
 * 实读 PaEditorial.Mcp —— 旧实现在枚举构造期把当时盘色定格,首次类加载之后
 * 浅/深/黑客三盘切换徽标紫不再刷新。回归口径:换盘后 [PaEditorial.Mcp] 变值,
 * 三族的 color 必须跟着变;非 Mcp 族仍取各自固定槽位,不受牵连。
 */
class FileKindPaletteTest {

    @After
    fun restorePalette() {
        AgentTheme.mode = "dark"
        PaEditorial.apply(dark = false)
    }

    @Test
    fun `mcp trio follows palette switch live`() {
        AgentTheme.mode = "dark"
        PaEditorial.apply(dark = false)
        val light = PaEditorial.Mcp
        assertEquals(light, FileKind.IMAGE.color)
        assertEquals(light, FileKind.AUDIO.color)
        assertEquals(light, FileKind.VIDEO.color)

        PaEditorial.apply(dark = true)
        val dark = PaEditorial.Mcp
        // 前提:两盘的 Mcp 槽位值确实不同(否则此测不构成换盘证据)
        assertNotEquals(light, dark)
        // 枚举不再定格在首次加载的盘色:换盘即实取新值
        assertEquals(dark, FileKind.IMAGE.color)
        assertEquals(dark, FileKind.AUDIO.color)
        assertEquals(dark, FileKind.VIDEO.color)
    }

    @Test
    fun `hacker palette mcp slot too`() {
        AgentTheme.mode = "hacker"
        PaEditorial.apply(dark = true, hacker = true)
        assertEquals(PaEditorial.Mcp, FileKind.IMAGE.color)
        assertEquals(PaEditorial.Mcp, FileKind.AUDIO.color)
        assertEquals(PaEditorial.Mcp, FileKind.VIDEO.color)
    }

    @Test
    fun `non mcp entries keep their fixed slots`() {
        AgentTheme.mode = "dark"
        PaEditorial.apply(dark = false)
        assertEquals(PaEditorial.Running, FileKind.APK.color)
        assertEquals(PaEditorial.Muted, FileKind.TEXT.color)
        // 非 Mcp 族不占 Mcp 紫(与媒体三族保持可辨)
        assertNotEquals(PaEditorial.Mcp, FileKind.APK.color)
    }
}
