package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder

/**
 * D7-3:网页快照的「截至时刻」([OfflineStore.UrlSnapshot.capturedAt])钉子 ——
 * 离线兜底 toast 要能说清"显示的是哪一刻的网页"。
 *
 * 【口径】capturedAt 取自快照文件的修改时间(getUrl 在 LRU touch **之前**读,
 * touch 写入的是"本次查看时刻",不是网页截至时刻);读不到(<=0)= 存量无
 * 时间戳,展示层回退旧文案(见 BrowserScreen.offlineSnapshotToastCopy)。
 */
class OfflineSnapshotCapturedAtTest {

    @get:Rule
    val tmp = TemporaryFolder()

    @Test
    fun `roundtrip returns snapshot with capture timestamp`() {
        OfflineStore.initForTest(tmp.newFolder("offline"))
        OfflineStore.putUrl("https://example.com/a", byteArrayOf(1, 2, 3), "text/html", "示例页")
        val snap = OfflineStore.getUrl("https://example.com/a")
        assertNotNull(snap)
        assertEquals("text/html", snap!!.mime)
        assertEquals("示例页", snap.name)
        assertTrue("文件修改时间应作为截至时刻(>0)", snap.capturedAt > 0)
    }

    @Test
    fun `legacy style constructor defaults capturedAt to zero`() {
        // 存量语义:没有可靠时间戳 = 0,展示层据此回退旧文案,不编造时间
        assertEquals(0L, OfflineStore.UrlSnapshot(byteArrayOf(1), "text/html", "old").capturedAt)
    }

    @Test
    fun `miss returns null`() {
        OfflineStore.initForTest(tmp.newFolder("offline2"))
        assertNull(OfflineStore.getUrl("https://example.com/none"))
    }
}
