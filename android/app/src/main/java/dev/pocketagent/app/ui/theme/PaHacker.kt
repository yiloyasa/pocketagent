package dev.pocketagent.app.ui.theme

import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/**
 * 黑客终端主题("hacker")的主对话消费契约 —— 主题架构层交付件。
 *
 * 【消费契约(给 ChatScreen 后续轮接线用,本轮不动 ChatScreen)】
 * 主对话在 hacker 模式下按以下两点替换既有默认,其余布局/配色已由 Pa 色板自动换肤:
 *  1. 气泡文本字体 = [PaHacker.mono](即 FontFamily.Monospace)——等宽是终端的骨相,
 *     只换字体不换字号即可让对话读作"终端回显";
 *  2. 气泡圆角 = [PaHacker.bubbleCorner](6dp)——终端没有饱满大圆角,硬朗小圆角
 *     (6dp)既保留圆角防撞手,又读作"一行输出的边框";常规主题请沿用现有默认
 *     (用户气泡主圆角 20dp,见 ChatScreen RoundedCornerShape(20,7,20,20))。
 *
 * 【接线写法建议】`fontFamily = if (PaHacker.isHacker) PaHacker.mono else null`、
 * `shape = PaHacker.bubbleCornerFor(RoundedCornerShape(20.dp, 7.dp, 20.dp, 20.dp))`。
 * isHacker=false 时两个 API 都"退回原样",不接线也不会破坏常规主题。
 *
 * 【为什么常量放 theme/ 而不是 ChatScreen】判定与数值是主题语义,属设计系统;
 * ChatScreen 只做消费。放这里还能被 JVM 测试直接钉住(见 PaHackerTest)。
 */
object PaHacker {
    /** AgentTheme.mode / AgentConnection.themeMode 里本模式的标识串。 */
    const val MODE = "hacker"

    /** 当前是否处于黑客终端模式。直接读 [AgentTheme.mode],JVM 可测。 */
    val isHacker: Boolean get() = AgentTheme.mode == MODE

    /**
     * 等宽字体常量(= FontFamily.Monospace)。
     * 仅在 hacker 模式下消费方才用它替换默认字体(语义由消费方用 [isHacker] 判定);
     * 常规主题换上它反而是回退——系统等宽字体密度差,不做全局替换。
     */
    val mono: FontFamily = FontFamily.Monospace

    /**
     * 主对话气泡圆角(模式响应式):hacker=6dp;常规=20dp(与 ChatScreen 现有
     * 用户气泡主圆角一致,消费方也可以用 [bubbleCornerFor] 直接换算自己的默认值)。
     */
    val bubbleCorner: Dp get() = bubbleCornerFor(20.dp)

    /** 消费契约的函数形态:hacker 时把消费方现有默认圆角压成 6dp,否则原样返回。 */
    fun bubbleCornerFor(default: Dp): Dp = if (isHacker) 6.dp else default

    /**
     * 可选的对话字号微缩系数(0.95):等宽字体天然偏宽,同字号下更早换行;
     * hacker 时对气泡正文乘 0.95 可以找回接近常规主题的每行字符容量。
     * 消费方可选采用,不采用也不破坏契约。
     */
    const val chatFontSizeStep: Float = 0.95f
}
