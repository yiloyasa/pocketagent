package dev.pocketagent.app.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.Color

/**
 * PocketAgent 设计系统 —— 对标参考稿(深色:纯黑/深灰大圆角卡/蓝色点缀)。
 *
 * 【讲解】颜色分层像"海拔":越浮在上面的东西越亮。
 * bg(底)→ card(一级卡)→ cardHigh(抽屉/二级)→ hover(按压)。
 * 所有字段都是 Compose 状态:设置里切换 显示模式 时整 app 一次性换肤。
 */
object Pa {
    var Bg by mutableStateOf(Color(0xFF000000)); private set
    var Card by mutableStateOf(Color(0xFF151517)); private set // 一级卡片
    var CardHigh by mutableStateOf(Color(0xFF1F1F22)); private set // 抽屉、二级内嵌
    var Hover by mutableStateOf(Color(0xFF2A2A2E)); private set // 按压高亮
    var Accent by mutableStateOf(Color(0xFF4A9DFF)); private set // 品牌蓝
    var AccentDim by mutableStateOf(Color(0xFF1E3A5C)); private set // 蓝的暗底(用户气泡)
    var TextPrimary by mutableStateOf(Color(0xFFF2F2F4)); private set
    var TextSecondary by mutableStateOf(Color(0xFF98989F)); private set
    var TextTertiary by mutableStateOf(Color(0xFF5A5A60)); private set
    var Separator by mutableStateOf(Color(0xFF26262A)); private set
    var Green by mutableStateOf(Color(0xFF32D74B)); private set
    var Red by mutableStateOf(Color(0xFFFF453A)); private set
    var Yellow by mutableStateOf(Color(0xFFFFD60A)); private set

    // ---- 小字号文本专用变体(设计规范 §1.1 对比度审计 · 方案 A) ----
    // 基础语义色用于图标/色块/进度条等图形元素(适用 3:1 非文本标准);
    // 下面这组专供 11sp 及以下的文本,对比度均 ≥5.2:1,满足 WCAG AA 正常文本 4.5:1。
    var AccentText by mutableStateOf(Color(0xFF7FC0FF)); private set
    var GreenText by mutableStateOf(Color(0xFF5BE070)); private set
    var RedText by mutableStateOf(Color(0xFFFF7A70)); private set

    /** 数据可视化专用:上下文构成中的「MCP技能」段。两套主题共用。 */
    var Mcp by mutableStateOf(Color(0xFFB584FF)); private set

    var IsDark by mutableStateOf(true); private set

    /**
     * 应用色板。dark=深/浅两套(hacker 也恒为 true);hacker=黑客终端专用磷光绿板。
     * 浅色 = 同一设计语言的反相(白底浅灰卡)。
     */
    internal fun apply(dark: Boolean, hacker: Boolean = false) {
        IsDark = dark
        // 玻璃材质随主题一起换肤,与实色同一时机生效;hacker 有自己的深绿玻璃
        PaGlassToken.apply(dark = dark, hacker = hacker)
        if (hacker) {
            // 【黑客终端】设计意图:CRT 磷光屏的"纯黑底 + 发光绿字"。
            //  - 层级仍走"海拔"模型,但整层压绿:卡片不是灰而是"暗屏上一块稍亮的磷光区";
            //  - Accent 换终端绿 #00E676(Material Green A400,亮度足够在纯黑上发光),
            //    用户气泡底 AccentDim 用极暗绿 #0D2B14——"按下的磷光余辉",不是蓝;
            //  - 文字三档全部带绿相(#C9F7CD/#63A96B/#3D6B45),次级文字像"余辉衰减",
            //    而不是中性灰——灰在纯绿世界里会显脏;
            //  - IsDark 恒为 true:纯黑底没有"浅色版",浅色磷光屏不是这个母题。
            Bg = Color(0xFF000000); Card = Color(0xFF0A120C); CardHigh = Color(0xFF101A12)
            Hover = Color(0xFF182418); Accent = Color(0xFF00E676); AccentDim = Color(0xFF0D2B14)
            TextPrimary = Color(0xFFC9F7CD); TextSecondary = Color(0xFF63A96B); TextTertiary = Color(0xFF3D6B45)
            Separator = Color(0xFF1B2E1E); Green = Color(0xFF4ADE80); Red = Color(0xFFFF6B6B)
            Yellow = Color(0xFFFFD60A)
            // 小字号变体:在暗绿底上把三色整体提亮一档,保 WCAG AA(同 §1.1 审计纪律)
            AccentText = Color(0xFF7DF0A0); GreenText = Color(0xFF5BE070); RedText = Color(0xFFFF8A80)
            Mcp = Color(0xFFB584FF)
        } else if (dark) {
            Bg = Color(0xFF000000); Card = Color(0xFF151517); CardHigh = Color(0xFF1F1F22)
            Hover = Color(0xFF2A2A2E); Accent = Color(0xFF4A9DFF); AccentDim = Color(0xFF1E3A5C)
            TextPrimary = Color(0xFFF2F2F4); TextSecondary = Color(0xFF98989F); TextTertiary = Color(0xFF5A5A60)
            Separator = Color(0xFF26262A); Green = Color(0xFF32D74B); Red = Color(0xFFFF453A)
            Yellow = Color(0xFFFFD60A)
            AccentText = Color(0xFF7FC0FF); GreenText = Color(0xFF5BE070); RedText = Color(0xFFFF7A70)
            Mcp = Color(0xFFB584FF)
        } else {
            Bg = Color(0xFFF6F6F8); Card = Color(0xFFFFFFFF); CardHigh = Color(0xFFF0F0F3)
            Hover = Color(0xFFE4E4E9); Accent = Color(0xFF2F7FE8); AccentDim = Color(0xFFD9E9FF)
            TextPrimary = Color(0xFF17171A); TextSecondary = Color(0xFF6A6A72); TextTertiary = Color(0xFF9E9EA6)
            Separator = Color(0xFFE6E6EB); Green = Color(0xFF1FA843); Red = Color(0xFFE5372B)
            Yellow = Color(0xFFC79400)
            AccentText = Color(0xFF1F63C4); GreenText = Color(0xFF177C33); RedText = Color(0xFFC42B21)
            Mcp = Color(0xFFB584FF)
        }
    }
}

/** 全局主题:根据设置(跟随系统/浅色/深色/黑客终端)应用配色。 */
@Composable
fun PaTheme(content: @Composable () -> Unit) {
    // hacker 恒走深色:纯黑磷光屏天生是暗色母题,不存在"浅色黑客终端"
    val dark = when (AgentTheme.mode) {
        "light" -> false
        "dark", "hacker" -> true
        else -> isSystemInDarkTheme()
    }
    Pa.apply(dark, hacker = AgentTheme.mode == "hacker")
    val scheme = if (dark) darkColorScheme(
        primary = Pa.Accent, onPrimary = Color.White, background = Pa.Bg, onBackground = Pa.TextPrimary,
        surface = Pa.Card, onSurface = Pa.TextPrimary, surfaceVariant = Pa.CardHigh,
        onSurfaceVariant = Pa.TextSecondary, outline = Pa.Separator, error = Pa.Red,
    ) else lightColorScheme(
        primary = Pa.Accent, onPrimary = Color.White, background = Pa.Bg, onBackground = Pa.TextPrimary,
        surface = Pa.Card, onSurface = Pa.TextPrimary, surfaceVariant = Pa.CardHigh,
        onSurfaceVariant = Pa.TextSecondary, outline = Pa.Separator, error = Pa.Red,
    )
    MaterialTheme(colorScheme = scheme, content = content)
}

/** 显示模式状态("system"/"light"/"dark"/"hacker"),AgentConnection.init 时从偏好恢复。 */
object AgentTheme {
    var mode by mutableStateOf("system")
}
