package dev.pocketagent.app.ui.component

import android.content.Context
import android.content.SharedPreferences

/**
 * 会话置顶 —— 纯本地偏好(不改 daemon、不改 proto)。
 *
 * 置顶键取 `codexThreadId`(有则用之),这样"电脑里那条 codex 会话"
 * 在导入成本地会话前后是同一个键,置顶状态不会丢;没有 codex id 时退回本地 id。
 */
object ThreadPins {
    private const val PREFS = "pa_thread_pins"
    private const val KEY = "ids"
    private var prefs: SharedPreferences? = null

    fun init(ctx: Context) {
        if (prefs == null) {
            prefs = ctx.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        }
    }

    fun all(): Set<String> = prefs?.getStringSet(KEY, emptySet())?.toSet() ?: emptySet()

    fun isPinned(key: String): Boolean = key.isNotEmpty() && all().contains(key)

    /** 切换置顶,返回切换后的状态(true = 已置顶)。 */
    fun toggle(key: String): Boolean {
        if (key.isEmpty()) return false
        val current = all().toMutableSet()
        val nowPinned = if (current.remove(key)) {
            false
        } else {
            current.add(key)
            true
        }
        prefs?.edit()?.putStringSet(KEY, current)?.apply()
        return nowPinned
    }
}
