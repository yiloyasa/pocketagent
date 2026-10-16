package dev.pocketagent.app.flutterhost

import android.app.Activity
import android.content.ActivityNotFoundException
import android.content.Context
import android.util.Log
import android.widget.Toast

/**
 * 「打开 Flutter UI」的唯一入口壳。
 *
 * 【为什么单独一个 object 而不是在各页面里写 startActivity】入口的语义有三件
 * 必须一起做的事,散到调用方就会漏:
 *   ① 显式 Intent + 目标类(见 [FlutterHostLauncher.intentFor]);
 *   ② **预热**——引擎冷启 ~300ms,点击时才起引擎会让用户先看一帧白屏;
 *      这里在 startActivity 之前先 warmup(幂等,已热则零成本);
 *   ③ 失败兜底——FlutterHostActivity 起不来时(极端情况:引擎初始化抛错)
 *      不能静默无反应,给一句人话。
 *
 * 【为什么本波( B4a )不从任何页面调用它】AppDrawer / SettingsScreen / HomeScreen
 * 都不在本波允许改动的文件范围内(改动面收敛到「只挂壳」),而 MainActivity 里
 * 没有任何可挂入口的现成位置——硬塞一个 compose 路由会造出「没人导航到的死
 * 代码」(红线:不摆假入口)。故本波交付「可用的入口 + 经 intent 可启动的壳」,
 * UI 入口留到 B4b/B4c 与方法面一起接(那时才有东西可看)。
 * 现在就能验证的启动方式(真机/模拟器):
 *   `adb shell am start -n dev.pocketagent.app/.flutterhost.FlutterHostActivity`
 *
 * 【文案为什么是「开发中」】Flutter UI 的方法面/事件面都还没接(B4b),用户点进去
 * 看到的是一个连不上桌面的壳。入口若叫「Flutter UI」而不标状态,用户会以为
 * 「新 UI 上线了但坏了」。诚实标注 = 不制造错误预期。
 */
object FlutterHostEntry {
    private const val TAG = "FlutterHostEntry"

    /** 入口文案(调用方直接用,不自己另写一份)。 */
    const val LABEL: String = "Flutter UI（开发中）"

    /**
     * 打开 Flutter UI。
     *
     * 幂等:重复调用只是重复 startActivity(系统按 launchMode 处理),
     * warmup 那侧不会重复起 VM。
     *
     * @param activity 用 Activity 的 context(startActivity 需要 Activity context;
     *   用 Application context 会抛 AndroidRuntimeException)。
     */
    fun open(activity: Activity) {
        try {
            // 先预热再跳:把冷启藏在转场动画的时间里(见类 KDoc ②)。
            FlutterHostEngine.warmup(activity)
        } catch (e: Throwable) {
            // 预热失败不阻止跳转:FlutterHostActivity.provideFlutterEngine 会再试
            // 一次(它自己调 warmup),真起不来时由那边的异常路径处理。
            // 这里只记日志——预热失败不是用户可行动的错,不该弹窗打断。
            Log.w(TAG, "Flutter 引擎预热失败(仍尝试打开): ${e.javaClass.simpleName}: ${e.message}")
        }
        try {
            activity.startActivity(FlutterHostLauncher.intentFor(activity))
        } catch (e: ActivityNotFoundException) {
            // 目标 activity 不在 manifest 里才会到这(理论上集成做了就有);
            // 宁可说人话,也不静默无反应。
            Log.e(TAG, "FlutterHostActivity 未在 manifest 声明", e)
            Toast.makeText(activity, "Flutter UI 入口不可用（未安装）", Toast.LENGTH_LONG).show()
        }
    }

    /**
     * 无 Activity 时的降级:只预热,不起页面。
     *
     * 供「进程起来就先热一份」的场景(将来若在 Application.onCreate 里调用)。
     * 失败同样只记日志——预热是优化,不是必需路径。
     */
    fun warmupOnly(context: Context) {
        try {
            FlutterHostEngine.warmup(context)
        } catch (e: Throwable) {
            Log.w(TAG, "Flutter 引擎预热失败: ${e.javaClass.simpleName}: ${e.message}")
        }
    }
}
