package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Test
import java.util.Calendar

/**
 * D7-3:离线快照 toast 文案(纯函数)。快照带截至时间 → 「离线:显示
 * M月d日 HH:mm 的网页快照」,交代"看到的是哪一刻的网页";存量快照无
 * 时间戳(<=0)回退旧文案,绝不编造时间。
 */
class BrowserSnapshotToastTest {

    /** 用本地时区构造固定时刻(格式化同走默认时区,断言不受 CI 时区影响)。 */
    private fun atMs(year: Int, month: Int, day: Int, hour: Int, minute: Int): Long {
        val cal = Calendar.getInstance()
        cal.clear()
        cal.set(year, month - 1, day, hour, minute, 0)
        return cal.timeInMillis
    }

    @Test
    fun `shows as-of time when capturedAt present`() {
        assertEquals(
            "离线:显示 10月9日 21:05 的网页快照",
            offlineSnapshotToastCopy(atMs(2026, 10, 9, 21, 5)),
        )
        // 个位月/日不补零(M月d日),时分配零(HH:mm)
        assertEquals(
            "离线:显示 1月2日 08:07 的网页快照",
            offlineSnapshotToastCopy(atMs(2026, 1, 2, 8, 7)),
        )
    }

    @Test
    fun `falls back to legacy copy without timestamp`() {
        assertEquals("离线:显示本地网页快照", offlineSnapshotToastCopy(0L))
        assertEquals("离线:显示本地网页快照", offlineSnapshotToastCopy(-1L))
    }
}
