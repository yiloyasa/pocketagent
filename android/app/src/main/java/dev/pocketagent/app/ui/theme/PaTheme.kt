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
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.anim.ProvideReduceMotion

/**
 * PocketAgent 设计系统 —— 对标参考稿(深色:纯黑/深灰大圆角卡/蓝色点缀)。
 *
 * 【讲解】颜色分层像"海拔":越浮在上面的东西越亮。
 * bg(底)→ card(一级卡)→ cardHigh(抽屉/二级)→ hover(按压)。
 * 所有字段都是 Compose 状态:设置里切换 显示模式 时整 app 一次性换肤。
 *
 * 【兼容 shim 2026-10-09】编辑式令牌层 [PaEditorial] 是唯一事实来源,M3 colorScheme
 * 已由 editorialM3Scheme()(MainActivity NavHost 外层一次包全)接管。本对象**保留
 * 构造但仅存量引用过渡用,新代码禁用** —— 2026-10-09 核查:ui/ 下 Pa.* 代码引用
 * 已归零(仅本 shim 自身与 1 处文档提及),下一刀可连同 M3 scheme 一并退役;
 * 在那之前 Pa.* 与 PaEditorial.* 同槽位同盘灌入,不许新增消费点。
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
     * 应用色板。dark=深/浅两套(hacker 也恒为 true);hacker=黑客代码风「代码暗房」色板
     * (中性纯黑底+电光青强调+语法色点缀,v2 重做:全屏绿 tint 被用户否决)。
     * 浅色 = 同一设计语言的反相(白底浅灰卡)。
     */
    internal fun apply(dark: Boolean, hacker: Boolean = false) {
        IsDark = dark
        // 玻璃材质随主题一起换肤,与实色同一时机生效;hacker 有自己的深绿玻璃
        PaGlassToken.apply(dark = dark, hacker = hacker)
        if (hacker) {
            // 【黑客代码风 v2】v1 教训(用户原话:「狗不拉屎的绿看了犯恶心」):把绿
            // tint 到每个表面和每级文字,整屏灰绿一坨,低对比=脏。真正的黑客/代码
            // 美学是**代码编辑器暗色主题**:中性纯黑底、近白正文、高对比,
            // 颜色只出现在该出现的地方——
            //  - 底/卡:中性黑灰,零色相(像 #0D1117 的 GitHub Dark);
            //  - 强调:一处电光青 #22D3EE(终端 HUD 语言),只给标题/激活态/当前值;
            //  - 语法色(Mcp 紫/语法绿/琥珀黄)只做小面积点缀,像代码高亮;
            //  - 文字三档中性:近白/灰/暗灰,靠字重与亮度分层,不靠色相。
            Bg = Color(0xFF050507); Card = Color(0xFF0C0D11); CardHigh = Color(0xFF12141A)
            Hover = Color(0xFF1A1D26); Accent = Color(0xFF22D3EE); AccentDim = Color(0xFF0E2A33)
            TextPrimary = Color(0xFFE6E9EF); TextSecondary = Color(0xFF8B93A3); TextTertiary = Color(0xFF565E6E)
            Separator = Color(0xFF1C2028)
            // Green 在 hacker 下=teal:app 里大量交互图标/链接装饰性使用语义绿,
            // 不换色相的话整屏仍是绿感(用户红线)。teal 对「成功/在线」语义同样成立,
            // 与 Accent 电光青同族,整屏色相收敛为 黑白+青族+语法紫。
            Green = Color(0xFF2DD4BF); Red = Color(0xFFF87171); Yellow = Color(0xFFFBBF24)
            AccentText = Color(0xFF67E8F9); GreenText = Color(0xFF5EEAD4); RedText = Color(0xFFFCA5A5)
            Mcp = Color(0xFFA78BFA)
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
    // 编辑式三色盘与 Pa 同步换肤:全部 editorial.* 消费点(700+ 处直取 + 44 处
    // —— "700+"指 PaEditorial 令牌的正常消费量,非 Pa 残留;
    // 组合局部)随本调用零改动响应 显示模式(浅色/深色/黑客终端)。
    PaEditorial.apply(dark, hacker = AgentTheme.mode == "hacker")
    // 【兼容 shim 2026-10-09】旧 Pa 色板灌的 M3 scheme:仅存量引用过渡用,新代码禁用。
    // editorialM3Scheme()(编辑式盘)在 MainActivity NavHost 外层包在内层,全 app 的
    // M3 组件实际吃到的是编辑式槽位;本 scheme 只作为最外层基底存在。
    // onPrimary 统一读 PaEditorial.OnPrimary:hacker 盘主色是电光青 #22D3EE,
    // 白字对比仅 1.81:1,该盘内容色=暗青 #062B33(8.29:1)——与 editorialM3Scheme
    // 同读同一槽位,两套 M3 色板内容色不漂移。
    // onError 同读 PaEditorial.OnDenied(U2 反馈 C 类):深/黑客盘 error 实底
    // (#FF453A/#F87171)上白字仅 3.41/2.77:1,内容色=暗红棕 #3A1514
    // (4.75/5.85:1,≥4.5 AA)——与 editorialM3Scheme 同读同一槽位不漂移。
    val scheme = if (dark) darkColorScheme(
        primary = Pa.Accent, onPrimary = PaEditorial.OnPrimary, background = Pa.Bg, onBackground = Pa.TextPrimary,
        surface = Pa.Card, onSurface = Pa.TextPrimary, surfaceVariant = Pa.CardHigh,
        onSurfaceVariant = Pa.TextSecondary, outline = Pa.Separator, error = Pa.Red,
        onError = PaEditorial.OnDenied,
    ) else lightColorScheme(
        primary = Pa.Accent, onPrimary = PaEditorial.OnPrimary, background = Pa.Bg, onBackground = Pa.TextPrimary,
        surface = Pa.Card, onSurface = Pa.TextPrimary, surfaceVariant = Pa.CardHigh,
        onSurfaceVariant = Pa.TextSecondary, outline = Pa.Separator, error = Pa.Red,
        onError = PaEditorial.OnDenied,
    )
    ProvideReduceMotion {
        MaterialTheme(colorScheme = scheme, content = content)
    }
}

/** 显示模式状态("system"/"light"/"dark"/"hacker"),AgentConnection.init 时从偏好恢复。 */
object AgentTheme {
    var mode by mutableStateOf("system")
}
