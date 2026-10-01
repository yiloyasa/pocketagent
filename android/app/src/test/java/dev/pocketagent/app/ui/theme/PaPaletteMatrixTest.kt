package dev.pocketagent.app.ui.theme

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import org.junit.AfterClass
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 三盘槽位完备性 + 对比度达标矩阵(2026-09-30 主题根层终审的回归钉)。
 *
 * 【钉什么】
 *  1. **完备性钉**:34 个颜色槽位 × 浅/深/黑客三盘逐值钉死——任何"某槽位只调过
 *     一盘"的漂移(apply 漏灌某槽位时它会停留在上一盘的值)在这里必然炸出;
 *  2. **同构规则钉**:NeutralText/GreyText 三盘都等于 Muted(灰阶 inactive 与
 *     次级文本同值,盘内不允许出现两个近似灰);
 *  3. **对比度钉**:正文/辅助(Ink/Muted)与全部 *Text 小字档对各自盘底 ≥4.5:1
 *     (WCAG AA 正常文本),三级档(Subtle/OnGraphiteSubtle)与 CodeText ≥3:1
 *     / ≥4.5:1 —— 用 [PaContrast] 纯函数对三盘全量验算;
 *  4. **玻璃钉**:PaGlassToken 三盘三值(玻璃是随盘换肤的材质令牌)。
 *
 * 改令牌值 = 必须同时改本文件对应行,让改动在 review 里可见。
 */
class PaPaletteMatrixTest {

    // ---- 槽位取值器(34 个颜色槽位;新增槽位必须在此登记,否则完备性钉测不到它) ----

    private val slots: List<Pair<String, () -> Color>> = listOf(
        "Canvas" to { PaEditorial.Canvas },
        "Surface" to { PaEditorial.Surface },
        "Raised" to { PaEditorial.Raised },
        "Hover" to { PaEditorial.Hover },
        "Graphite" to { PaEditorial.Graphite },
        "GraphiteDrawer" to { PaEditorial.GraphiteDrawer },
        "GraphiteCode" to { PaEditorial.GraphiteCode },
        "Hairline" to { PaEditorial.Hairline },
        "Ink" to { PaEditorial.Ink },
        "Muted" to { PaEditorial.Muted },
        "Subtle" to { PaEditorial.Subtle },
        "Primary" to { PaEditorial.Primary },
        "PrimaryContainer" to { PaEditorial.PrimaryContainer },
        "Running" to { PaEditorial.Running },
        "RunningContainer" to { PaEditorial.RunningContainer },
        "Attention" to { PaEditorial.Attention },
        "AttentionContainer" to { PaEditorial.AttentionContainer },
        "Denied" to { PaEditorial.Denied },
        "DeniedContainer" to { PaEditorial.DeniedContainer },
        "AccentText" to { PaEditorial.AccentText },
        "RunningText" to { PaEditorial.RunningText },
        "DeniedText" to { PaEditorial.DeniedText },
        "AttentionText" to { PaEditorial.AttentionText },
        "NeutralText" to { PaEditorial.NeutralText },
        "NeutralContainer" to { PaEditorial.NeutralContainer },
        "GreyText" to { PaEditorial.GreyText },
        "GreyContainer" to { PaEditorial.GreyContainer },
        "Mcp" to { PaEditorial.Mcp },
        "OnGraphite" to { PaEditorial.OnGraphite },
        "OnGraphiteMuted" to { PaEditorial.OnGraphiteMuted },
        "OnGraphiteSubtle" to { PaEditorial.OnGraphiteSubtle },
        "OnGraphiteHairline" to { PaEditorial.OnGraphiteHairline },
        "CodeText" to { PaEditorial.CodeText },
        "Scrim" to { PaEditorial.Scrim },
    )

    private fun snapshot(): Map<String, Color> =
        slots.associate { (name, get) -> name to get() }

    // ---- 三盘权威值表(与 PaEditorial.apply 逐行对应;改盘先改这里) ----

    private val light = mapOf(
        "Canvas" to 0xFFF6F7F6, "Surface" to 0xFFFFFFFF, "Raised" to 0xFFEDF0F2,
        "Hover" to 0xFFE8ECEF, "Graphite" to 0xFF1E2835, "GraphiteDrawer" to 0xFF1B2634,
        "GraphiteCode" to 0xFF1A2430, "Hairline" to 0xFFE3E7EB,
        "Ink" to 0xFF19222D, "Muted" to 0xFF5F6C7C, "Subtle" to 0xFF828B95,
        "Primary" to 0xFF386BE7, "PrimaryContainer" to 0xFFE8EFFF,
        "Running" to 0xFF21977D, "RunningContainer" to 0xFFE5F5EF,
        "Attention" to 0xFFA96726, "AttentionContainer" to 0xFFFFF1DD,
        "Denied" to 0xFFC94F4D, "DeniedContainer" to 0xFFFFF0ED,
        "AccentText" to 0xFF2B5BC7, "RunningText" to 0xFF157C63, "DeniedText" to 0xFFB84D49,
        "AttentionText" to 0xFF99621E, "NeutralText" to 0xFF5F6C7C, "NeutralContainer" to 0xFFE8EDEF,
        "GreyText" to 0xFF5F6C7C, "GreyContainer" to 0xFFF0F2F4,
        "Mcp" to 0xFFB584FF,
        "OnGraphite" to 0xFFFFFFFF, "OnGraphiteMuted" to 0xFFBFCCD9,
        "OnGraphiteSubtle" to 0xFF8B98A6, "OnGraphiteHairline" to 0xFF364655,
        "CodeText" to 0xFFD5E6F8, "Scrim" to 0xA9101722,
    )

    private val dark = mapOf(
        "Canvas" to 0xFF0F141C, "Surface" to 0xFF161D28, "Raised" to 0xFF1D2532,
        "Hover" to 0xFF232C3B, "Graphite" to 0xFF1A2230, "GraphiteDrawer" to 0xFF141B27,
        "GraphiteCode" to 0xFF10161F, "Hairline" to 0xFF2A3442,
        "Ink" to 0xFFE8EDF4, "Muted" to 0xFFA2AEBD, "Subtle" to 0xFF6B7887,
        "Primary" to 0xFF6C9BF2, "PrimaryContainer" to 0xFF1D3050,
        "Running" to 0xFF4CC2A4, "RunningContainer" to 0xFF12312B,
        "Attention" to 0xFFE5B36B, "AttentionContainer" to 0xFF33270F,
        "Denied" to 0xFFF08580, "DeniedContainer" to 0xFF3A2123,
        "AccentText" to 0xFF9DBDF8, "RunningText" to 0xFF7DD8C0, "DeniedText" to 0xFFF4A9A4,
        "AttentionText" to 0xFFEBC98F, "NeutralText" to 0xFFA2AEBD, "NeutralContainer" to 0xFF1D2532,
        "GreyText" to 0xFFA2AEBD, "GreyContainer" to 0xFF1D2532,
        "Mcp" to 0xFFBC8CFF,
        "OnGraphite" to 0xFFE8EDF4, "OnGraphiteMuted" to 0xFF97A5B5,
        "OnGraphiteSubtle" to 0xFF667384, "OnGraphiteHairline" to 0xFF2A3442,
        "CodeText" to 0xFFD5E6F8, "Scrim" to 0x99000000,
    )

    private val hacker = mapOf(
        "Canvas" to 0xFF050507, "Surface" to 0xFF0C0D11, "Raised" to 0xFF12141A,
        "Hover" to 0xFF1A1D26, "Graphite" to 0xFF0A0B0F, "GraphiteDrawer" to 0xFF08090C,
        "GraphiteCode" to 0xFF050507, "Hairline" to 0xFF1C2028,
        "Ink" to 0xFFE6E9EF, "Muted" to 0xFF8B93A3, "Subtle" to 0xFF5B6372,
        "Primary" to 0xFF22D3EE, "PrimaryContainer" to 0xFF0E2A33,
        "Running" to 0xFF2DD4BF, "RunningContainer" to 0xFF0B2B27,
        "Attention" to 0xFFFBBF24, "AttentionContainer" to 0xFF332708,
        "Denied" to 0xFFF87171, "DeniedContainer" to 0xFF331B1B,
        "AccentText" to 0xFF67E8F9, "RunningText" to 0xFF5EEAD4, "DeniedText" to 0xFFFCA5A5,
        "AttentionText" to 0xFFFCD34D, "NeutralText" to 0xFF8B93A3, "NeutralContainer" to 0xFF12141A,
        "GreyText" to 0xFF8B93A3, "GreyContainer" to 0xFF12141A,
        "Mcp" to 0xFFA78BFA,
        "OnGraphite" to 0xFFE6E9EF, "OnGraphiteMuted" to 0xFF8B93A3,
        "OnGraphiteSubtle" to 0xFF5C6473, "OnGraphiteHairline" to 0xFF1C2028,
        "CodeText" to 0xFF67E8F9, "Scrim" to 0xB3000000,
    )

    private fun applyPalette(dark: Boolean, hacker: Boolean) = PaEditorial.apply(dark, hacker)

    private fun applyAndSnapshot(expected: Map<String, Long>, dark: Boolean, hacker: Boolean): Map<String, Color> {
        applyPalette(dark, hacker)
        val snap = snapshot()
        expected.forEach { (name, argb) ->
            assertEquals("槽位 $name 与权威值表不符", Color(argb), snap[name])
        }
        return snap
    }

    // ---- 1. 完备性钉:34 槽位 × 3 盘逐值 ----

    @Test
    fun `浅色盘_全部槽位逐值钉死`() {
        applyAndSnapshot(light, dark = false, hacker = false)
    }

    @Test
    fun `深色盘_全部槽位逐值钉死`() {
        applyAndSnapshot(dark, dark = true, hacker = false)
    }

    @Test
    fun `黑客盘_全部槽位逐值钉死`() {
        applyAndSnapshot(hacker, dark = true, hacker = true)
    }

    @Test
    fun `完备性_每个槽位三盘都有被刻意调过的值_不允许某槽位只调过一盘`() {
        val l = applyAndSnapshot(light, dark = false, hacker = false)
        val d = applyAndSnapshot(dark, dark = true, hacker = false)
        val h = applyAndSnapshot(hacker, dark = true, hacker = true)
        // 白名单:CodeText 在浅盘也只出现在石墨代码箱(恒暗底),浅==深是母题而非漂移
        val carryOver = setOf("CodeText")
        slots.forEach { (name, _) ->
            if (name !in carryOver) {
                assertNotEquals("槽位 $name 深盘与浅盘同值——疑似 apply 漏灌", l[name], d[name])
                assertNotEquals("槽位 $name 黑客盘与浅盘同值——疑似 apply 漏灌", l[name], h[name])
            }
        }
    }

    @Test
    fun `同构规则_NeutralText与GreyText三盘并轨到Muted_盘内不留近似灰`() {
        val l = applyAndSnapshot(light, dark = false, hacker = false)
        val d = applyAndSnapshot(dark, dark = true, hacker = false)
        val h = applyAndSnapshot(hacker, dark = true, hacker = true)
        listOf(l, d, h).forEach { snap ->
            assertEquals("NeutralText 必须并轨 Muted", snap["Muted"], snap["NeutralText"])
            assertEquals("GreyText 必须并轨 Muted", snap["Muted"], snap["GreyText"])
        }
    }

    @Test
    fun `换盘可逆_三盘往返后回到权威浅色`() {
        applyPalette(dark = true, hacker = true)
        applyPalette(dark = true, hacker = false)
        applyPalette(dark = false, hacker = false)
        slots.forEach { (name, get) ->
            assertEquals("槽位 $name 往返后漂移", Color(light.getValue(name)), get())
        }
        assertTrue(!PaEditorial.IsDark)
    }

    @Test
    fun `IsDark判定_浅假深真_黑客恒真`() {
        applyPalette(dark = false, hacker = false); assertEquals(false, PaEditorial.IsDark)
        applyPalette(dark = true, hacker = false); assertEquals(true, PaEditorial.IsDark)
        applyPalette(dark = true, hacker = true); assertEquals(true, PaEditorial.IsDark)
    }

    // ---- 2. 阴影三档随盘(暗盘降档,M3 色调面替代阴影) ----

    @Test
    fun `阴影三档随盘降档_暗盘blur与dy不大于浅盘`() {
        applyPalette(dark = false, hacker = false)
        val lightCard = PaEditorial.Shadow.Card
        applyPalette(dark = true, hacker = false)
        val darkCard = PaEditorial.Shadow.Card
        applyPalette(dark = true, hacker = true)
        val hackerCard = PaEditorial.Shadow.Card
        assertTrue(darkCard.blur <= lightCard.blur && darkCard.dy <= lightCard.dy)
        assertTrue(hackerCard.blur <= lightCard.blur && hackerCard.dy <= lightCard.dy)
        assertEquals(PaEditorial.ShadowSpec(26.dp, 8.dp, Color(0x10293D56)), lightCard)
        assertEquals(PaEditorial.ShadowSpec(18.dp, 4.dp, Color(0x14304A66)), darkCard)
        assertEquals(PaEditorial.ShadowSpec(16.dp, 3.dp, Color(0x140D1117)), hackerCard)
    }

    // ---- 3. 玻璃材质令牌三盘三值 ----

    @Test
    fun `玻璃令牌三盘三值`() {
        PaGlassToken.apply(dark = false, hacker = false)
        assertEquals(Color(0xF2FFFFFF), PaGlassToken.Tint)
        assertEquals(Color(0x5CFFFFFF), PaGlassToken.Highlight)
        assertEquals(Color(0x33201E2C), PaGlassToken.Scrim)
        PaGlassToken.apply(dark = true, hacker = false)
        assertEquals(Color(0xE6151517), PaGlassToken.Tint)
        assertEquals(Color(0x2EFFFFFF), PaGlassToken.Highlight)
        assertEquals(Color(0x66000000), PaGlassToken.Scrim)
        PaGlassToken.apply(dark = true, hacker = true)
        assertEquals(Color(0xE60C0D11), PaGlassToken.Tint)
        assertEquals(Color(0x2422D3EE), PaGlassToken.Highlight)
        assertEquals(Color(0x73000000), PaGlassToken.Scrim)
    }

    // ---- 4. 对比度钉:三盘全量验算(PaContrast 纯函数) ----

    private fun assertReadable(fg: Color?, bgs: List<Color?>, min: Float, label: String) {
        requireNotNull(fg) { "槽位缺失:$label" }
        bgs.forEachIndexed { i, bg ->
            val r = PaContrast.contrastRatio(fg, requireNotNull(bg))
            assertTrue(
                "$label 对比率 $r 低于 $min(第 ${i + 1} 个底色)",
                r >= min,
            )
        }
    }

    @Test
    fun `对比度_正文与辅助档三盘对画布面全量AA`() {
        val surfaces = { s: Map<String, Color> ->
            listOf(s["Canvas"], s["Surface"], s["Raised"]).filterNotNull()
        }
        mapOf(
            "浅" to applyAndSnapshot(light, dark = false, hacker = false),
            "深" to applyAndSnapshot(dark, dark = true, hacker = false),
            "黑客" to applyAndSnapshot(hacker, dark = true, hacker = true),
        ).forEach { (palette, s) ->
            val bgs = surfaces(s)
            assertReadable(s["Ink"], bgs, PaContrast.AA_TEXT, "$palette.Ink(正文)")
            assertReadable(s["Muted"], bgs, PaContrast.AA_TEXT, "$palette.Muted(辅助)")
        }
    }

    @Test
    fun `对比度_三级档三盘至少3比1`() {
        mapOf(
            "浅" to applyAndSnapshot(light, dark = false, hacker = false),
            "深" to applyAndSnapshot(dark, dark = true, hacker = false),
            "黑客" to applyAndSnapshot(hacker, dark = true, hacker = true),
        ).forEach { (palette, s) ->
            val bgs = listOf(s["Canvas"], s["Surface"], s["Raised"]).filterNotNull()
            assertReadable(s["Subtle"], bgs, PaContrast.AA_GRAPHIC, "$palette.Subtle(三级)")
            val graphite = listOf(s["Graphite"], s["GraphiteDrawer"], s["GraphiteCode"]).filterNotNull()
            assertReadable(s["OnGraphite"], graphite, PaContrast.AA_TEXT, "$palette.OnGraphite")
            assertReadable(s["OnGraphiteMuted"], graphite, PaContrast.AA_TEXT, "$palette.OnGraphiteMuted")
            assertReadable(s["OnGraphiteSubtle"], graphite, PaContrast.AA_GRAPHIC, "$palette.OnGraphiteSubtle")
        }
    }

    @Test
    fun `对比度_小字语义档与代码字三盘全量AA`() {
        mapOf(
            "浅" to applyAndSnapshot(light, dark = false, hacker = false),
            "深" to applyAndSnapshot(dark, dark = true, hacker = false),
            "黑客" to applyAndSnapshot(hacker, dark = true, hacker = true),
        ).forEach { (palette, s) ->
            fun triple(fgName: String, containerName: String) =
                assertReadable(
                    s[fgName],
                    listOf(s[containerName], s["Surface"], s["Canvas"]),
                    PaContrast.AA_TEXT,
                    "$palette.$fgName(11-12sp 小字档)",
                )
            triple("AccentText", "PrimaryContainer")
            triple("RunningText", "RunningContainer")
            triple("DeniedText", "DeniedContainer")
            triple("AttentionText", "AttentionContainer")
            triple("NeutralText", "NeutralContainer")
            triple("GreyText", "GreyContainer")
            assertReadable(
                s["CodeText"], listOf(s["GraphiteCode"]),
                PaContrast.AA_TEXT, "$palette.CodeText(代码字)",
            )
        }
    }

    @Test
    fun `对比度工具_黑白锚点钉值`() {
        // 纯函数自检:黑/白 = 21:1,同色 = 1:1
        assertEquals(21f, PaContrast.contrastRatio(Color.Black, Color.White), 0.01f)
        assertEquals(1f, PaContrast.contrastRatio(Color.Black, Color.Black), 0.01f)
        assertTrue(PaContrast.meets(Color.White, Color.Black))
    }

    companion object {
        @AfterClass
        @JvmStatic
        fun restoreLight() {
            // 防测试间串色:矩阵跑完恢复默认浅盘
            PaEditorial.apply(dark = false, hacker = false)
            PaGlassToken.apply(dark = false, hacker = false)
        }
    }
}
