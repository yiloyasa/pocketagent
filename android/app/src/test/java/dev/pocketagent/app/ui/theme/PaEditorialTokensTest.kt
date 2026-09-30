package dev.pocketagent.app.ui.theme

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 编辑式令牌层的钉值单测:令牌是"唯一事实来源",值被静默改动 = 全 app 视觉漂移。
 * 这里把 README §视觉系统 + premium.css 的每个关键值钉死 —— 要改令牌,先改这里,
 * 让改动在 review 里可见。
 */
class PaEditorialTokensTest {

    // ---- 颜色令牌(README §视觉系统 原文值) ----

    @Test
    fun `画布与表面与石墨与细线与深墨`() {
        assertEquals(Color(0xFFF6F7F6), PaEditorial.Canvas)
        assertEquals(Color(0xFFFFFFFF), PaEditorial.Surface)
        assertEquals(Color(0xFFEDF0F2), PaEditorial.Raised)
        assertEquals(Color(0xFF1E2835), PaEditorial.Graphite)
        assertEquals(Color(0xFF1B2634), PaEditorial.GraphiteDrawer)
        assertEquals(Color(0xFF1A2430), PaEditorial.GraphiteCode)
        assertEquals(Color(0xFFE3E7EB), PaEditorial.Hairline)
        assertEquals(Color(0xFF19222D), PaEditorial.Ink)
        assertEquals(Color(0xFF697788), PaEditorial.Muted)
        assertEquals(Color(0xFF98A3AE), PaEditorial.Subtle)
    }

    @Test
    fun `语义色_主色只用于当前操作_四档语义色`() {
        assertEquals(Color(0xFF386BE7), PaEditorial.Primary)
        assertEquals(Color(0xFFE8EFFF), PaEditorial.PrimaryContainer)
        assertEquals(Color(0xFF21977D), PaEditorial.Running)
        assertEquals(Color(0xFFE5F5EF), PaEditorial.RunningContainer)
        assertEquals(Color(0xFFA96726), PaEditorial.Attention)
        assertEquals(Color(0xFFFFF1DD), PaEditorial.AttentionContainer)
        assertEquals(Color(0xFFC94F4D), PaEditorial.Denied)
        assertEquals(Color(0xFFFFF0ED), PaEditorial.DeniedContainer)
    }

    @Test
    fun `石墨面文字与代码箱与遮罩`() {
        assertEquals(Color(0xFFFFFFFF), PaEditorial.OnGraphite)
        assertEquals(Color(0xFFBFCCD9), PaEditorial.OnGraphiteMuted)
        assertEquals(Color(0xFF8B98A6), PaEditorial.OnGraphiteSubtle)
        assertEquals(Color(0xFF364655), PaEditorial.OnGraphiteHairline)
        assertEquals(Color(0xFFD5E6F8), PaEditorial.CodeText)
        assertEquals(Color(0xA9101722), PaEditorial.Scrim)
    }

    // ---- 字阶五档(README:25/32、34/39、16/22、正文 13-14/21、辅助 11-12/17) ----

    @Test
    fun `字阶钉值`() {
        assertEquals(34.sp, PaEditorial.Type.Display.fontSize)
        assertEquals(39.sp, PaEditorial.Type.Display.lineHeight)
        assertEquals(25.sp, PaEditorial.Type.Title.fontSize)
        assertEquals(32.sp, PaEditorial.Type.Title.lineHeight)
        assertEquals(16.sp, PaEditorial.Type.Heading.fontSize)
        assertEquals(22.sp, PaEditorial.Type.Heading.lineHeight)
        assertEquals(14.sp, PaEditorial.Type.Body.fontSize)
        assertEquals(21.sp, PaEditorial.Type.Body.lineHeight)
        assertEquals(13.sp, PaEditorial.Type.BodySmall.fontSize)
        assertEquals(12.sp, PaEditorial.Type.Caption.fontSize)
        assertEquals(17.sp, PaEditorial.Type.Caption.lineHeight)
        assertEquals(11.sp, PaEditorial.Type.Micro.fontSize)
        assertEquals(10.sp, PaEditorial.Type.State.fontSize)
    }

    @Test
    fun `字重与字距钉值_编辑式标题是紧字距重字重`() {
        assertEquals(FontWeight(760), PaEditorial.Type.Display.fontWeight)
        assertEquals(FontWeight(720), PaEditorial.Type.Title.fontWeight)
        assertEquals((-1.8f).sp, PaEditorial.Type.Display.letterSpacing)
        assertEquals((-0.9f).sp, PaEditorial.Type.Title.letterSpacing)
    }

    // ---- 间距与点击区(8px 网格 / 23px 边距 / 44px 点击区) ----

    @Test
    fun `间距钉值_网格边距点击区`() {
        assertEquals(23.dp, PaEditorial.Space.PageMargin)
        assertEquals(44.dp, PaEditorial.Space.TouchMin)
        assertEquals(68.dp, PaEditorial.Space.NavHeight)
        assertEquals(22.dp, PaEditorial.Space.NavGutter)
        assertEquals(42.dp, PaEditorial.Space.IconBox)
        assertEquals(8.dp, PaEditorial.Space.G8)
    }

    @Test
    fun `网格步进是8px系_四档大间距都是8的倍数`() {
        // 4 是半格(允许的微调步),其余主步必须落在 8px 网格上
        assertEquals(4.dp, PaEditorial.Space.G4)
        assertEquals(16.dp, PaEditorial.Space.G16)
        assertEquals(24.dp, PaEditorial.Space.G24)
        assertEquals(32.dp, PaEditorial.Space.G32)
        val gridValue = PaEditorial.Space.G24.value
        assertTrue("G24 必须是 8 的倍数", gridValue % 8f == 0f)
    }

    @Test
    fun `圆角与阴影钉值`() {
        assertEquals(13.dp, PaEditorial.Radius.IconButton)
        assertEquals(18.dp, PaEditorial.Radius.Card)
        assertEquals(23.dp, PaEditorial.Radius.Sheet)
        assertEquals(26.dp, PaEditorial.Shadow.Card.blur)
        assertEquals(8.dp, PaEditorial.Shadow.Card.dy)
        assertEquals(Color(0x10293D56), PaEditorial.Shadow.Card.color)
        assertEquals(80.dp, PaEditorial.Shadow.Modal.blur)
        assertEquals(Color(0x55071726), PaEditorial.Shadow.Modal.color)
    }

    // ---- 语义色档三件套(基础色/小字加深变体/浅底,取 premium.css .state) ----

    @Test
    fun `色档三件套钉值_小字变体对浅底加深`() {
        assertEquals(Color(0xFF167F65), EditorialTone.RUNNING.text)
        // 2026-09-30 三色盘改造:RUNNING.container 归并到 RunningContainer(#E5F5EF,
        // 原 .state.run 底 #E2F4EC 与 --mint-dim 差异不可辨,少一个槽位)
        assertEquals(Color(0xFFE5F5EF), EditorialTone.RUNNING.container)
        assertEquals(Color(0xFF99621E), EditorialTone.ATTENTION.text)
        assertEquals(Color(0xFF6A7C8A), EditorialTone.NEUTRAL.text)
        assertEquals(Color(0xFFE8EDEF), EditorialTone.NEUTRAL.container)
        assertEquals(Color(0xFFBE504C), EditorialTone.DENIED.text)
        assertEquals(Color(0xFFFFF0ED), EditorialTone.DENIED.container)
    }

    @Test
    fun `基础语义色与令牌同源_色档不另起炉灶`() {
        // 色档的 accent 必须就是令牌本值:徽章的点、图标的色与语义令牌一比一
        assertEquals(PaEditorial.Running, EditorialTone.RUNNING.accent)
        assertEquals(PaEditorial.Attention, EditorialTone.ATTENTION.accent)
        assertEquals(PaEditorial.Denied, EditorialTone.DENIED.accent)
        assertEquals(PaEditorial.Primary, EditorialTone.PRIMARY.accent)
    }

    // ---- 三色盘同构(2026-09-30):浅/深/黑客同槽位,apply 换盘,组件零改动 ----

    @Test
    fun `深色盘钉值_M3暗色音调面_角色配对满足AA`() {
        PaEditorial.apply(dark = true, hacker = false)
        try {
            assertEquals(Color(0xFF0F141C), PaEditorial.Canvas)
            assertEquals(Color(0xFF161D28), PaEditorial.Surface)
            assertEquals(Color(0xFF1D2532), PaEditorial.Raised)
            assertEquals(Color(0xFFE8EDF4), PaEditorial.Ink)
            assertEquals(Color(0xFF6C9BF2), PaEditorial.Primary)
            assertEquals(Color(0xFFA2AEBD), PaEditorial.Muted)
            assertTrue(PaEditorial.IsDark)
            // 色档三件套同样动态:DARK 盘下语义档随盘
            assertEquals(Color(0xFF7DD8C0), EditorialTone.RUNNING.text)
            assertEquals(PaEditorial.Running, EditorialTone.RUNNING.accent)
        } finally {
            PaEditorial.apply(dark = false, hacker = false) // 恢复默认浅色,防测试间串色
        }
    }

    @Test
    fun `黑客盘钉值_GitHubDark基底电光青强调`() {
        PaEditorial.apply(dark = true, hacker = true)
        try {
            assertEquals(Color(0xFF050507), PaEditorial.Canvas)
            assertEquals(Color(0xFF0C0D11), PaEditorial.Surface)
            assertEquals(Color(0xFF22D3EE), PaEditorial.Primary)
            assertEquals(Color(0xFFE6E9EF), PaEditorial.Ink)
            assertEquals(Color(0xFF2DD4BF), PaEditorial.Running)
            assertEquals(Color(0xFF67E8F9), PaEditorial.CodeText)
            assertTrue(PaEditorial.IsDark)
            assertEquals(PaEditorial.Primary, EditorialTone.PRIMARY.accent)
        } finally {
            PaEditorial.apply(dark = false, hacker = false)
        }
    }

    @Test
    fun `apply恢复浅色后与默认钉值一致_同构色盘可逆`() {
        PaEditorial.apply(dark = true, hacker = true)
        PaEditorial.apply(dark = false, hacker = false)
        assertEquals(Color(0xFFF6F7F6), PaEditorial.Canvas)
        assertEquals(Color(0xFF386BE7), PaEditorial.Primary)
        assertFalse(PaEditorial.IsDark)
        // 字阶/间距/圆角模式无关,换盘不动
        assertEquals(23.dp, PaEditorial.Space.PageMargin)
        assertEquals(18.dp, PaEditorial.Radius.Card)
    }
}
