package dev.pocketagent.app.ui.component

import android.content.Context
import android.os.Build
import android.os.VibrationEffect
import android.os.Vibrator
import android.os.VibratorManager
import android.provider.Settings
import android.view.View

/**
 * 语义触觉的三种波形时长(纯函数,单测钉死 PaHapticsPatternTest;审计 P1-5)。
 *
 * 【为什么波形要抽纯函数】Android 的预置触感(EFFECT_CLICK 等)只在 API 29+ 有,
 * minSdk 26 的旧机要走 [VibrationEffect.createWaveform] 兜底 —— 两套路径必须
 * 长出**同一种语义**,波形数据在这里单测钉死,播放层(Android API)不测:
 *  - CONFIRM:轻击一下(审批提交的"已确认");
 *  - SUCCESS:双击(任务完成的"成了",系统触感的既有惯例);
 *  - WARNING:比确认更重的一下(审批到达的"注意")。
 * 数组形态 = [VibrationEffect.createWaveform] 的 timings:前导 0 + 脉冲(+间隔)。
 */
internal enum class PaHapticKind { CONFIRM, SUCCESS, WARNING }

internal fun paHapticPattern(kind: PaHapticKind): LongArray = when (kind) {
    PaHapticKind.CONFIRM -> longArrayOf(0, 18)
    PaHapticKind.SUCCESS -> longArrayOf(0, 28, 70, 28)
    PaHapticKind.WARNING -> longArrayOf(0, 42)
}

/**
 * 语义触觉统一入口(审计 P1-5):confirm / success / warning 三函数,
 * 调用方一行接入 —— `PaHaptics.confirm(view)`。
 *
 * 【触感分级体系(2026-09-30 微交互轮终审)】三级,同动作全局同强度:
 *  - 轻 [light]:高频微反馈 —— 刻度刷选跨档、长按菜单弹出、pressable(haptic)
 *    的点选;全走系统 tick 级,不惊扰;
 *  - 中 [confirm]([success] 为完成态变体):动作确认 —— 撤销执行、审批提交、
 *    历史刻度松手跳转;一次清晰的轻击;
 *  - 重 [warning]:错误/警示 —— 审批到达等需要"抬头看"的时刻;重击一下。
 * 接入完整性核对(全 app 触感调用点):ChatScreen(审批 warning/提交 confirm/
 * 终局 success)、HomeScreen(confirm)、UndoBar 撤销(confirm)、HistoryTickBar
 * 刷选(light)+松手跳转(confirm)、ContextMenu 弹出(light)+点选(light,
 * pressable)—— 三级各有真实调用点,无"有 API 无人用/有动作无触感"的缺口。
 *
 * 【跟随系统开关】每次触发前检查系统「触感反馈」总开关
 * (Settings.System.HAPTIC_FEEDBACK_ENABLED,读不到时默认开),关了就静默返回;
 * 另外不覆盖 [View.performHapticFeedback] 自身对 view 开关的尊重。
 * 语义绑定系统开关意味着:用户关触感,三处语义触觉一起安静,不再各自为政。
 *
 * 【为什么用 Vibrator 而不是 HapticFeedbackType】Compose 的 HapticFeedbackType
 * 只有 LongPress/TextHandleMove 等两三种,表达不了"确认/完成双击/警示"的语义差;
 * API 29+ 用预置效果(EFFECT_CLICK/EFFECT_DOUBLE_CLICK/EFFECT_HEAVY_CLICK),
 * 旧机用上面的波形兜底,两套路径同语义(见 [paHapticPattern])。
 */
object PaHaptics {

    /**
     * 轻触感(触感分级体系的"轻"档):微交互档 —— 刻度刷选跨档、拖动扫过等
     * 高频低强度反馈。View 层走 CLOCK_TICK(比 VIRTUAL_KEY 更轻的 tick)。
     * Compose 侧等价物是 HapticFeedbackType.TextHandleMove(pressable(haptic=true)
     * 即此档);统一入口的意义是**同一把系统总开关**:用户关触感,轻重档一起安静。
     */
    fun light(view: View) {
        val ctx = view.context ?: return
        if (!systemHapticsEnabled(ctx)) return
        runCatching {
            view.performHapticFeedback(android.view.HapticFeedbackConstants.CLOCK_TICK)
        }
    }

    /** 审批提交:轻击确认。 */
    fun confirm(view: View) = perform(view, PaHapticKind.CONFIRM)

    /** 任务完成(终局帧 ok=true):双击模式。 */
    fun success(view: View) = perform(view, PaHapticKind.SUCCESS)

    /** 审批到达:更重的一下。 */
    fun warning(view: View) = perform(view, PaHapticKind.WARNING)

    private fun perform(view: View, kind: PaHapticKind) {
        val ctx = view.context ?: return
        if (!systemHapticsEnabled(ctx)) return
        val vibrator = vibratorOf(ctx) ?: run {
            // 无振动器(极少数设备):退回 view 触感,至少保留一次物理反馈
            view.performHapticFeedback(android.view.HapticFeedbackConstants.VIRTUAL_KEY)
            return
        }
        try {
            if (Build.VERSION.SDK_INT >= 29) {
                val predefined = when (kind) {
                    PaHapticKind.CONFIRM -> VibrationEffect.EFFECT_CLICK
                    PaHapticKind.SUCCESS -> VibrationEffect.EFFECT_DOUBLE_CLICK
                    PaHapticKind.WARNING -> VibrationEffect.EFFECT_HEAVY_CLICK
                }
                vibrator.vibrate(VibrationEffect.createPredefined(predefined))
            } else {
                val pattern = paHapticPattern(kind)
                // -1 默认振幅:旧机最稳(IntArray:createWaveform 的 amplitudes 参数是 IntArray)
                val amplitudes = IntArray(pattern.size) { -1 }
                vibrator.vibrate(VibrationEffect.createWaveform(pattern, amplitudes, -1))
            }
        } catch (_: Exception) {
            // 触觉是增益路径:任何 ROM 层异常都不允许打断业务动作
        }
    }

    /** 系统触感反馈总开关(设置 → 声音与振动 → 触感);读不到按开处理。 */
    private fun systemHapticsEnabled(ctx: Context): Boolean = runCatching {
        Settings.System.getInt(ctx.contentResolver, Settings.System.HAPTIC_FEEDBACK_ENABLED, 1) != 0
    }.getOrDefault(true)

    private fun vibratorOf(ctx: Context): Vibrator? = runCatching {
        if (Build.VERSION.SDK_INT >= 31) {
            (ctx.getSystemService(Context.VIBRATOR_MANAGER_SERVICE) as? VibratorManager)?.defaultVibrator
        } else {
            @Suppress("DEPRECATION")
            ctx.getSystemService(Context.VIBRATOR_SERVICE) as? Vibrator
        }
    }.getOrNull()?.takeIf { it.hasVibrator() }
}
