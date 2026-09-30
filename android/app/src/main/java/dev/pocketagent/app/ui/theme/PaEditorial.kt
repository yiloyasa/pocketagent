package dev.pocketagent.app.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
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
 * premium.css(浅色编辑式稿的权威 CSS;prototype.html 的 :root 是深色稿,
 * 两稿同构,本层一律取 README/premium.css 的浅色编辑式值)。
 *
 * 【定位】这是"默认视觉升级"的**加法基座**:只提供新令牌与语义色档,供第二波
 * 逐页迁移;不新增显示模式、不改 [Pa]/[PaGlass]/[PaHacker] 任何行为。编辑式
 * 令牌是固定浅色盘(冷白画布上的深墨文字),不随 AgentTheme.mode 换肤 ——
 * 侧边栏/代码预览/待审批卡用 [Graphite] 系深色做"层次",不是满屏深色卡。
 *
 * 【语义红线】语义绝不单靠颜色:状态展示必须同时带状态词(见 ui/EditorialStatus
 * 四词映射器);主色 [Primary] 只用于"当前操作"(激活态/主按钮),不拿来做装饰。
 * 小字号(11-12sp)文本用 [EditorialTone.text] 专用变体(对比度 ≥4.5:1),
 * 与 Pa.GreenText 同一套纪律。
 *
 * 【消费方式】两种等价:
 *  1. 直取:`PaEditorial.Canvas`(与 `Pa.Bg` 同风格,适合组件内部);
 *  2. 组合局部:`MaterialTheme.editorial.xxx`(第二波页面统一走这个,
 *     将来若做编辑式深色盘只需换 Local 提供值,调用点零改动)。
 */
object PaEditorial {

    // ---- 画布与表面(premium.css :root) ----
    /** 冷白画布(--bg #F6F7F6):页面最底层。 */
    val Canvas = Color(0xFFF6F7F6)
    /** 瓷白表面(--surface #FFFFFF):卡片、弹窗、输入面。 */
    val Surface = Color(0xFFFFFFFF)
    /** 次级抬升面(--raised #EDF0F2):内嵌块、图标底。 */
    val Raised = Color(0xFFEDF0F2)
    /** 石墨重点面(#1E2835,README):侧边栏、代码预览、待审批卡的深色层。 */
    val Graphite = Color(0xFF1E2835)
    /** 石墨·抽屉(premium.css .drawer #1B2634)。 */
    val GraphiteDrawer = Color(0xFF1B2634)
    /** 石墨·代码箱(premium.css .codebox #1A2430)。 */
    val GraphiteCode = Color(0xFF1A2430)
    /** 细线(--line #E3E7EB):分隔线、描边,普通内容不套卡,靠留白+细线分层。 */
    val Hairline = Color(0xFFE3E7EB)
    /** 深墨正文(--text #19222D)。 */
    val Ink = Color(0xFF19222D)
    /** 次级文本(--muted #697788)。 */
    val Muted = Color(0xFF697788)
    /** 三级文本(--subtle #98A3AE)。 */
    val Subtle = Color(0xFF98A3AE)

    // ---- 语义色(README:主色只用于当前操作) ----
    /** 主色 #386BE7:仅当前操作(激活导航项/主按钮/进行中的选择)。 */
    val Primary = Color(0xFF386BE7)
    /** 主色浅底(--blue-dim #E8EFFF)。 */
    val PrimaryContainer = Color(0xFFE8EFFF)
    /** 运行/已连接 绿 #21977D。 */
    val Running = Color(0xFF21977D)
    /** 运行浅底(--mint-dim #E5F5EF)。 */
    val RunningContainer = Color(0xFFE5F5EF)
    /** 待用户处理 琥珀 #A96726。 */
    val Attention = Color(0xFFA96726)
    /** 琥珀浅底(--amber-dim #FFF1DD)。 */
    val AttentionContainer = Color(0xFFFFF1DD)
    /** 拒绝/未完成 珊瑚红 #C94F4D。 */
    val Denied = Color(0xFFC94F4D)
    /** 珊瑚红浅底(--red-dim #FFF0ED)。 */
    val DeniedContainer = Color(0xFFFFF0ED)

    // ---- 深色面上的文字(premium.css .alert / .drawer / .codebox) ----
    val OnGraphite = Color(0xFFFFFFFF)
    /** 石墨面次级文本(.alert p #BFCCD9)。 */
    val OnGraphiteMuted = Color(0xFFBFCCD9)
    /** 石墨面三级文本(.modal-title small #8B98A6)。 */
    val OnGraphiteSubtle = Color(0xFF8B98A6)
    /** 石墨面细线(.drawer-head border #364655)。 */
    val OnGraphiteHairline = Color(0xFF364655)
    /** 代码箱内代码文字(.codebox #D5E6F8,等宽)。 */
    val CodeText = Color(0xFFD5E6F8)

    /** 遮罩(premium.css .modal-scrim #101722A9)。 */
    val Scrim = Color(0xA9101722)

    // ---- 字阶(README 五档;值与行高取 premium.css 实测) ----
    object Type {
        /** 首页主标题 34/39(.hero-title 34px·1.16·w760)。 */
        val Display = TextStyle(fontSize = 34.sp, lineHeight = 39.sp, fontWeight = FontWeight(760), letterSpacing = (-1.8).sp)
        /** 页面标题 25/32(.nav h1 25px·w720)。 */
        val Title = TextStyle(fontSize = 25.sp, lineHeight = 32.sp, fontWeight = FontWeight(720), letterSpacing = (-0.9).sp)
        /** 模块标题 16/22(.section-title h2 16px·w700)。 */
        val Heading = TextStyle(fontSize = 16.sp, lineHeight = 22.sp, fontWeight = FontWeight.Bold, letterSpacing = (-0.3).sp)
        /** 正文 14/21(正文 13–14 的主档;.session-top .title 14px)。 */
        val Body = TextStyle(fontSize = 14.sp, lineHeight = 21.sp)
        /** 正文小 13/21(.agent-text/.run-row b 13px)。 */
        val BodySmall = TextStyle(fontSize = 13.sp, lineHeight = 21.sp)
        /** 辅助 12/17(.session .desc 12px)。 */
        val Caption = TextStyle(fontSize = 12.sp, lineHeight = 17.sp)
        /** 辅助微 11/17(.section-title small/.host-main span 11px)。 */
        val Micro = TextStyle(fontSize = 11.sp, lineHeight = 17.sp)
        /** 状态词专用 10/14(.state 10px·w650):状态徽章四词。 */
        val State = TextStyle(fontSize = 10.sp, lineHeight = 14.sp, fontWeight = FontWeight(650))
    }

    // ---- 间距(README:8px 网格,外边距 23px,点击区≥44px) ----
    object Space {
        val G4 = 4.dp
        val G8 = 8.dp
        val G12 = 12.dp
        val G16 = 16.dp
        val G24 = 24.dp
        val G32 = 32.dp
        /** 页面外边距 23px(README;premium.css .content 9px 23px 25px)。 */
        val PageMargin = 23.dp
        /** 顶部导航水平内距 22px(premium.css .nav 0 22px;导航与内容各按稿取值)。 */
        val NavGutter = 22.dp
        /** 顶部导航高度 68px(premium.css .nav)。 */
        val NavHeight = 68.dp
        /** 最小点击区 44px(README)。 */
        val TouchMin = 44.dp
        /** 身份/图标方格 42px(premium.css .host-icon/.spark 42x42)。 */
        val IconBox = 42.dp
    }

    // ---- 圆角(premium.css 实测频次:18=大卡、13=图标钮/胶囊/搜索、23=弹窗、100=药丸) ----
    object Radius {
        val IconButton = 13.dp
        val Card = 18.dp
        val Sheet = 23.dp
        val Pill = 100.dp
    }

    /** 阴影规格(premium.css box-shadow 的 blur/dy/ARGB),供 drawBehind/shadow 复用。 */
    @Immutable
    data class ShadowSpec(val blur: Dp, val dy: Dp, val color: Color)

    object Shadow {
        /** 卡片(.status-card 0 8px 26px #293d5610)。 */
        val Card = ShadowSpec(26.dp, 8.dp, Color(0x10293D56))
        /** 待审批深色卡(.alert 0 15px 35px #25344425)。 */
        val Alert = ShadowSpec(35.dp, 15.dp, Color(0x25253444))
        /** 弹窗(.modal 0 30px 80px #07172655)。 */
        val Modal = ShadowSpec(80.dp, 30.dp, Color(0x55071726))
    }
}

/**
 * 语义色档:同一语义的"图形色/小字色/浅底"三件套。
 *
 * 【为什么三件】基础语义色(=README 令牌)给点、图标、进度条等图形(3:1 档);
 * 11-12sp 状态文字必须用 text 档(premium.css .state 的加深变体,对浅底 ≥4.5:1);
 * container 是徽章/胶囊的浅底。PRIMARY 的 text 与 accent 同值 —— 原型如此
 * (.icon-button.active),纪律:主色文字只出现在 ≥14sp 粗体的操作语境,
 * 不用于 11-12sp 辅助文字。
 */
enum class EditorialTone(val accent: Color, val text: Color, val container: Color) {
    /** 主色:仅当前操作。 */
    PRIMARY(PaEditorial.Primary, PaEditorial.Primary, PaEditorial.PrimaryContainer),
    /** 运行/已连接(#21977D;.state.run 文字 #167f65·底 #e2f4ec)。 */
    RUNNING(PaEditorial.Running, Color(0xFF167F65), Color(0xFFE2F4EC)),
    /** 等待用户处理(#A96726;.state.wait 文字 #99621e·底取 --amber-dim)。 */
    ATTENTION(PaEditorial.Attention, Color(0xFF99621E), PaEditorial.AttentionContainer),
    /** 已完成:中性墨(#6a7c8a·底 #e8edef,.state.done)。 */
    NEUTRAL(Color(0xFF6A7C8A), Color(0xFF6A7C8A), Color(0xFFE8EDEF)),
    /** 拒绝/未完成(#C94F4D;.state.fail 文字 #be504c·底 --red-dim)。 */
    DENIED(PaEditorial.Denied, Color(0xFFBE504C), PaEditorial.DeniedContainer),
    /** 灰:离线/不可用态(subtle #98A3AE;.brand 底 #f0f2f4)。 */
    GREY(PaEditorial.Subtle, PaEditorial.Subtle, Color(0xFFF0F2F4)),
}

/**
 * 组合局部令牌:第二波页面统一 `MaterialTheme.editorial.xxx` 取值。
 *
 * 默认值即 [PaEditorial](固定浅色编辑式盘)。PaTheme 不需要改动 —— 静态默认值
 * 全局生效;将来若出"编辑式深色盘",在 PaTheme 里 Compose
 * CompositionLocalProvider(LocalEditorial 提供 < 新实例 >) 即可,纯加法。
 */
val LocalEditorial = staticCompositionLocalOf { PaEditorial }

/** MaterialTheme 扩展:页内统一写作 `MaterialTheme.editorial.Canvas`。 */
val MaterialTheme.editorial: PaEditorial
    @Composable get() = LocalEditorial.current
