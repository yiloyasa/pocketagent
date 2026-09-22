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

    /**
     * 取消置顶(删除会话时用)。幂等:没置顶过就是 no-op。
     *
     * 【为什么删除要显式调它】置顶键留着的后果不只是"集合里多一项":置顶是列表
     * 排序的第一优先级,将来同 id 的会话再建出来(换机重装、重新导入)时会**莫名
     * 顶着**。删除该把这条会话在这台机器上的痕迹一并带走。
     */
    fun unpin(key: String) {
        if (key.isEmpty()) return
        val current = all().toMutableSet()
        if (!current.remove(key)) return
        prefs?.edit()?.putStringSet(KEY, current)?.apply()
    }
}
