package dev.pocketagent.app.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * 编辑式设计令牌层(PaEditorial)=「每个界面整体升级」工程的**唯一事实来源**。
 *
 * 【出处】docs/design/2026-09-29-pocketagent-ui/README.md §视觉系统 + 同目录
 * premium.css(浅色编辑式稿的权威 CSS)。
 *
 * 【三色盘同构】2026-09-30 起,本层与 [Pa] 同机制:全部颜色是 `mutableStateOf`
 * 可变单例字段,由 [apply] 按**显示模式**(浅色/深色/黑客终端)灌入三张同槽位
 * 色盘 —— 槽位名即语义,三盘同一套组件结构,只换色(「显示逻辑大差不差」)。
 * 消费端(直取 `PaEditorial.X` 700+ 处与 `MaterialTheme.editorial.*`)零改动
 * 响应切换:读取发生在组合期,状态变更自动重组。
 *
 * 【三盘的视觉母题】
 *  - LIGHT(默认):冷白画布 + 瓷白面 + 石墨重点面,编辑式层级(原型定稿值)。
 *  - DARK:同构反相,按 M3 暗色规范走"色调面替代阴影"(surface≈N10,文字
 *    Tone90,角色配对 ≥4.5:1);阴影降档。
 *  - HACKER:GitHub Dark 基底(中性纯黑零色相)+ 一处电光青 #22D3EE 强调,
 *    语法色只做小面积点缀 —— 与 [Pa] hacker 分支同一母题,色值同源对齐。
 *
 * 【语义红线】语义绝不单靠颜色:状态展示必须同时带状态词(见 ui/EditorialStatus
 * 四词映射器);主色 [Primary] 只用于"当前操作"。11-12sp 小字用各语义的
 * *Text 档(对比度 ≥4.5:1),与 Pa.GreenText 同一套纪律。
 *
 * 【消费方式】两种等价:
 *  1. 直取:`PaEditorial.Canvas`(适合组件内部);
 *  2. 组合局部:`MaterialTheme.editorial.xxx`。
 */
object PaEditorial {

    // ---- 画布与表面 ----
    /** 页面最底层(浅:--bg #F6F7F6 冷白)。 */
    var Canvas by mutableStateOf(Color(0xFFF6F7F6)); private set
    /** 卡片、弹窗、输入面(浅:--surface #FFFFFF 瓷白)。 */
    var Surface by mutableStateOf(Color(0xFFFFFFFF)); private set
    /** 内嵌块、图标底(浅:--raised #EDF0F2)。 */
    var Raised by mutableStateOf(Color(0xFFEDF0F2)); private set
    /** 按压高亮(浅:#E8ECEF)。 */
    var Hover by mutableStateOf(Color(0xFFE8ECEF)); private set
    /** 石墨重点面(浅:#1E2835):侧边栏、代码预览、待审批卡的深色层。 */
    var Graphite by mutableStateOf(Color(0xFF1E2835)); private set
    /** 石墨·抽屉(浅:premium.css .drawer #1B2634)。 */
    var GraphiteDrawer by mutableStateOf(Color(0xFF1B2634)); private set
    /** 石墨·代码箱(浅:premium.css .codebox #1A2430)。 */
    var GraphiteCode by mutableStateOf(Color(0xFF1A2430)); private set
    /** 细线:分隔线、描边(浅:--line #E3E7EB)。 */
    var Hairline by mutableStateOf(Color(0xFFE3E7EB)); private set
    /** 正文(浅:--text #19222D 深墨)。 */
    var Ink by mutableStateOf(Color(0xFF19222D)); private set
    /** 次级文本(浅:--muted #697788)。 */
    var Muted by mutableStateOf(Color(0xFF697788)); private set
    /** 三级文本(浅:--subtle #98A3AE)。 */
    var Subtle by mutableStateOf(Color(0xFF98A3AE)); private set

    // ---- 语义色(README:主色只用于当前操作) ----
    /** 主色(浅:#386BE7)仅当前操作(激活导航项/主按钮/进行中的选择)。 */
    var Primary by mutableStateOf(Color(0xFF386BE7)); private set
    /** 主色浅底(浅:--blue-dim #E8EFFF)。 */
    var PrimaryContainer by mutableStateOf(Color(0xFFE8EFFF)); private set
    /** 运行/已连接(浅:#21977D)。 */
    var Running by mutableStateOf(Color(0xFF21977D)); private set
    /** 运行浅底(浅:--mint-dim #E5F5EF)。 */
    var RunningContainer by mutableStateOf(Color(0xFFE5F5EF)); private set
    /** 待用户处理 琥珀(浅:#A96726)。 */
    var Attention by mutableStateOf(Color(0xFFA96726)); private set
    /** 琥珀浅底(浅:--amber-dim #FFF1DD)。 */
    var AttentionContainer by mutableStateOf(Color(0xFFFFF1DD)); private set
    /** 拒绝/未完成 珊瑚红(浅:#C94F4D)。 */
    var Denied by mutableStateOf(Color(0xFFC94F4D)); private set
    /** 珊瑚红浅底(浅:--red-dim #FFF0ED)。 */
    var DeniedContainer by mutableStateOf(Color(0xFFFFF0ED)); private set

    // ---- 小字号文本专用变体(11-12sp 文本对比度 ≥4.5:1;同 Pa.*Text 纪律) ----
    var AccentText by mutableStateOf(Color(0xFF2B5BC7)); private set
    var RunningText by mutableStateOf(Color(0xFF167F65)); private set
    var DeniedText by mutableStateOf(Color(0xFFBE504C)); private set
    var AttentionText by mutableStateOf(Color(0xFF99621E)); private set
    /** 已完成(中性)的小字/图形档(浅:.state.done #6a7c8a)。 */
    var NeutralText by mutableStateOf(Color(0xFF6A7C8A)); private set
    /** 已完成徽章底(浅:.state.done #e8edef)。 */
    var NeutralContainer by mutableStateOf(Color(0xFFE8EDEF)); private set
    /** 灰:离线/不可用的小字档(浅:=subtle #98A3AE)。 */
    var GreyText by mutableStateOf(Color(0xFF98A3AE)); private set
    /** 灰徽章底(浅:.brand 底 #f0f2f4)。 */
    var GreyContainer by mutableStateOf(Color(0xFFF0F2F4)); private set
    /** 数据可视化专用:上下文构成中的「MCP技能」段。 */
    var Mcp by mutableStateOf(Color(0xFFB584FF)); private set

    // ---- 深色面上的文字(浅盘:.alert / .drawer / .codebox) ----
    var OnGraphite by mutableStateOf(Color(0xFFFFFFFF)); private set
    var OnGraphiteMuted by mutableStateOf(Color(0xFFBFCCD9)); private set
    var OnGraphiteSubtle by mutableStateOf(Color(0xFF8B98A6)); private set
    var OnGraphiteHairline by mutableStateOf(Color(0xFF364655)); private set
    /** 代码箱内代码文字(等宽)。 */
    var CodeText by mutableStateOf(Color(0xFFD5E6F8)); private set

    /** 弹窗遮罩(浅:premium.css .modal-scrim #101722A9)。 */
    var Scrim by mutableStateOf(Color(0xA9101722)); private set

    /** 当前盘是否暗色(系统栏图标/暗色专属分支的唯一判定源)。 */
    var IsDark by mutableStateOf(false); private set

    /**
     * 按显示模式灌入色盘。由 [PaTheme] 在每次重组时与 [Pa.apply] 同步调用;
     * 三盘同槽位,调用方(700+ 处)零改动换肤。
     */
    internal fun apply(dark: Boolean, hacker: Boolean = false) {
        if (hacker) {
            // 【黑客终端】GitHub Dark 基底 + 电光青强调(与 Pa hacker 分支同源):
            // 中性纯黑零色相,近白正文,青只给激活态/当前值,语法紫做点缀。
            Canvas = Color(0xFF050507); Surface = Color(0xFF0C0D11); Raised = Color(0xFF12141A)
            Hover = Color(0xFF1A1D26); Graphite = Color(0xFF0A0B0F); GraphiteDrawer = Color(0xFF08090C)
            GraphiteCode = Color(0xFF050507); Hairline = Color(0xFF1C2028)
            Ink = Color(0xFFE6E9EF); Muted = Color(0xFF8B93A3); Subtle = Color(0xFF565E6E)
            Primary = Color(0xFF22D3EE); PrimaryContainer = Color(0xFF0E2A33)
            Running = Color(0xFF2DD4BF); RunningContainer = Color(0xFF0B2B27)
            Attention = Color(0xFFFBBF24); AttentionContainer = Color(0xFF332708)
            Denied = Color(0xFFF87171); DeniedContainer = Color(0xFF331B1B)
            AccentText = Color(0xFF67E8F9); RunningText = Color(0xFF5EEAD4); DeniedText = Color(0xFFFCA5A5)
            AttentionText = Color(0xFFFCD34D); NeutralText = Color(0xFF8B93A3); NeutralContainer = Color(0xFF12141A)
            GreyText = Color(0xFF565E6E); GreyContainer = Color(0xFF12141A)
            Mcp = Color(0xFFA78BFA)
            OnGraphite = Color(0xFFE6E9EF); OnGraphiteMuted = Color(0xFF8B93A3)
            OnGraphiteSubtle = Color(0xFF565E6E); OnGraphiteHairline = Color(0xFF1C2028)
            CodeText = Color(0xFF67E8F9); Scrim = Color(0xB3000000)
            IsDark = true
            Shadow.Card = ShadowSpec(16.dp, 3.dp, Color(0x140D1117))
            Shadow.Alert = ShadowSpec(20.dp, 6.dp, Color(0x1A0C0D11))
            Shadow.Modal = ShadowSpec(40.dp, 12.dp, Color(0x66000000))
        } else if (dark) {
            // 【深色】同构反相,按 M3 暗色规范:surface≈中性 Tone10、文字 Tone90
            // (角色配对 ≥4.5:1),暗色用色调面替代阴影(阴影降档)。
            Canvas = Color(0xFF0F141C); Surface = Color(0xFF161D28); Raised = Color(0xFF1D2532)
            Hover = Color(0xFF232C3B); Graphite = Color(0xFF1A2230); GraphiteDrawer = Color(0xFF141B27)
            GraphiteCode = Color(0xFF10161F); Hairline = Color(0xFF2A3442)
            Ink = Color(0xFFE8EDF4); Muted = Color(0xFFA2AEBD); Subtle = Color(0xFF6B7887)
            Primary = Color(0xFF6C9BF2); PrimaryContainer = Color(0xFF1D3050)
            Running = Color(0xFF4CC2A4); RunningContainer = Color(0xFF12312B)
            Attention = Color(0xFFE5B36B); AttentionContainer = Color(0xFF33270F)
            Denied = Color(0xFFF08580); DeniedContainer = Color(0xFF3A2123)
            AccentText = Color(0xFF9DBDF8); RunningText = Color(0xFF7DD8C0); DeniedText = Color(0xFFF4A9A4)
            AttentionText = Color(0xFFEBC98F); NeutralText = Color(0xFFA2AEBD); NeutralContainer = Color(0xFF1D2532)
            GreyText = Color(0xFF6B7887); GreyContainer = Color(0xFF1D2532)
            Mcp = Color(0xFFBC8CFF)
            OnGraphite = Color(0xFFE8EDF4); OnGraphiteMuted = Color(0xFF97A5B5)
            OnGraphiteSubtle = Color(0xFF5F6D7E); OnGraphiteHairline = Color(0xFF2A3442)
            CodeText = Color(0xFFD5E6F8); Scrim = Color(0x99000000)
            IsDark = true
            Shadow.Card = ShadowSpec(18.dp, 4.dp, Color(0x14304A66))
            Shadow.Alert = ShadowSpec(24.dp, 8.dp, Color(0x1A101820))
            Shadow.Modal = ShadowSpec(48.dp, 16.dp, Color(0x73000000))
        } else {
            // 【浅色】原型定稿值(premium.css),默认盘,零漂移。
            Canvas = Color(0xFFF6F7F6); Surface = Color(0xFFFFFFFF); Raised = Color(0xFFEDF0F2)
            Hover = Color(0xFFE8ECEF); Graphite = Color(0xFF1E2835); GraphiteDrawer = Color(0xFF1B2634)
            GraphiteCode = Color(0xFF1A2430); Hairline = Color(0xFFE3E7EB)
            Ink = Color(0xFF19222D); Muted = Color(0xFF697788); Subtle = Color(0xFF98A3AE)
            Primary = Color(0xFF386BE7); PrimaryContainer = Color(0xFFE8EFFF)
            Running = Color(0xFF21977D); RunningContainer = Color(0xFFE5F5EF)
            Attention = Color(0xFFA96726); AttentionContainer = Color(0xFFFFF1DD)
            Denied = Color(0xFFC94F4D); DeniedContainer = Color(0xFFFFF0ED)
            AccentText = Color(0xFF2B5BC7); RunningText = Color(0xFF167F65); DeniedText = Color(0xFFBE504C)
            AttentionText = Color(0xFF99621E); NeutralText = Color(0xFF6A7C8A); NeutralContainer = Color(0xFFE8EDEF)
            GreyText = Color(0xFF98A3AE); GreyContainer = Color(0xFFF0F2F4)
            Mcp = Color(0xFFB584FF)
            OnGraphite = Color(0xFFFFFFFF); OnGraphiteMuted = Color(0xFFBFCCD9)
            OnGraphiteSubtle = Color(0xFF8B98A6); OnGraphiteHairline = Color(0xFF364655)
            CodeText = Color(0xFFD5E6F8); Scrim = Color(0xA9101722)
            IsDark = false
            Shadow.Card = ShadowSpec(26.dp, 8.dp, Color(0x10293D56))
            Shadow.Alert = ShadowSpec(35.dp, 15.dp, Color(0x25253444))
            Shadow.Modal = ShadowSpec(80.dp, 30.dp, Color(0x55071726))
        }
    }

    // ---- 字阶(README 五档;值与行高取 premium.css 实测;模式无关) ----
    object Type {
        /** 首页主标题 34/39(.hero-title 34px·1.16·w760)。 */
        val Display = TextStyle(fontSize = 34.sp, lineHeight = 39.sp, fontWeight = FontWeight(760), letterSpacing = (-1.8).sp)
        /** 页面标题 25/32(.nav h1 25px·w720)。 */
        val Title = TextStyle(fontSize = 25.sp, lineHeight = 32.sp, fontWeight = FontWeight(720), letterSpacing = (-0.9).sp)
        /** 模块标题 16/22(.section-title h2 16px·w700)。 */
        val Heading = TextStyle(fontSize = 16.sp, lineHeight = 22.sp, fontWeight = FontWeight.Bold, letterSpacing = (-0.3).sp)
        /** 正文 14/21。 */
        val Body = TextStyle(fontSize = 14.sp, lineHeight = 21.sp)
        /** 正文小 13/21。 */
        val BodySmall = TextStyle(fontSize = 13.sp, lineHeight = 21.sp)
        /** 辅助 12/17。 */
        val Caption = TextStyle(fontSize = 12.sp, lineHeight = 17.sp)
        /** 辅助微 11/17。 */
        val Micro = TextStyle(fontSize = 11.sp, lineHeight = 17.sp)
        /** 状态词专用 10/14(.state 10px·w650):状态徽章四词。 */
        val State = TextStyle(fontSize = 10.sp, lineHeight = 14.sp, fontWeight = FontWeight(650))
    }

    // ---- 间距(README:8px 网格,外边距 23px,点击区≥44px;模式无关) ----
    object Space {
        val G4 = 4.dp
        val G8 = 8.dp
        val G12 = 12.dp
        val G16 = 16.dp
        val G24 = 24.dp
        val G32 = 32.dp
        /** 页面外边距 23px(README;premium.css .content 9px 23px 25px)。 */
        val PageMargin = 23.dp
        /** 顶部导航水平内距 22px(premium.css .nav 0 22px)。 */
        val NavGutter = 22.dp
        /** 顶部导航高度 68px(premium.css .nav)。 */
        val NavHeight = 68.dp
        /** 最小点击区 44px(README)。 */
        val TouchMin = 44.dp
        /** 身份/图标方格 42px(premium.css .host-icon/.spark 42x42)。 */
        val IconBox = 42.dp
    }

    // ---- 圆角(premium.css 实测:18=大卡、13=图标钮/胶囊、23=弹窗、100=药丸;模式无关) ----
    object Radius {
        val IconButton = 13.dp
        val Card = 18.dp
        val Sheet = 23.dp
        val Pill = 100.dp
    }

    /** 阴影规格(blur/dy/ARGB),随色盘切换(暗盘按 M3 规范降档,靠色调面分层)。 */
    @Immutable
    data class ShadowSpec(val blur: Dp, val dy: Dp, val color: Color)

    object Shadow {
        var Card by mutableStateOf(ShadowSpec(26.dp, 8.dp, Color(0x10293D56))); internal set
        var Alert by mutableStateOf(ShadowSpec(35.dp, 15.dp, Color(0x25253444))); internal set
        var Modal by mutableStateOf(ShadowSpec(80.dp, 30.dp, Color(0x55071726))); internal set
    }
}

/**
 * 语义色档:同一语义的"图形色/小字色/浅底"三件套。三档均**动态读当前盘**
 * ([PaEditorial] 的 mutableStateOf 槽位),在组合内取值即随显示模式换肤。
 *
 * 枚举身份不变(RUNNING/ATTENTION/... 的身份被多处映射器与测试钉住);
 * 浅色盘的值与 2026-09-30 之前的固定浅色一致(仅 RUNNING.container 由
 * #E2F4EC 归并到 RunningContainer #E5F5EF,视觉不可辨)。
 * PRIMARY 的 text 与 accent 同值 —— 原型纪律:主色文字只出现在 ≥14sp 粗体
 * 操作语境,不用于 11-12sp 辅助文字(那些用各自的 *Text 档)。
 */
enum class EditorialTone {
    /** 主色:仅当前操作。 */
    PRIMARY,
    /** 运行/已连接。 */
    RUNNING,
    /** 等待用户处理。 */
    ATTENTION,
    /** 已完成:中性。 */
    NEUTRAL,
    /** 拒绝/未完成。 */
    DENIED,
    /** 灰:离线/不可用态。 */
    GREY;

    val accent: Color get() = when (this) {
        PRIMARY -> PaEditorial.Primary
        RUNNING -> PaEditorial.Running
        ATTENTION -> PaEditorial.Attention
        NEUTRAL -> PaEditorial.NeutralText
        DENIED -> PaEditorial.Denied
        GREY -> PaEditorial.Subtle
    }

    val text: Color get() = when (this) {
        PRIMARY -> PaEditorial.Primary
        RUNNING -> PaEditorial.RunningText
        ATTENTION -> PaEditorial.AttentionText
        NEUTRAL -> PaEditorial.NeutralText
        DENIED -> PaEditorial.DeniedText
        GREY -> PaEditorial.GreyText
    }

    val container: Color get() = when (this) {
        PRIMARY -> PaEditorial.PrimaryContainer
        RUNNING -> PaEditorial.RunningContainer
        ATTENTION -> PaEditorial.AttentionContainer
        NEUTRAL -> PaEditorial.NeutralContainer
        DENIED -> PaEditorial.DeniedContainer
        GREY -> PaEditorial.GreyContainer
    }
}

/**
 * 组合局部令牌:页内统一 `MaterialTheme.editorial.xxx` 取值。
 *
 * 默认值即 [PaEditorial] 单例;三色盘切换靠 [PaEditorial.apply] 就地改字段
 * (与 [Pa.apply] 同机制),Local 无需换值 —— 两种消费方式都自动响应。
 */
val LocalEditorial = staticCompositionLocalOf { PaEditorial }

/** MaterialTheme 扩展:页内统一写作 `MaterialTheme.editorial.Canvas`。 */
val MaterialTheme.editorial: PaEditorial
    @Composable get() = LocalEditorial.current
