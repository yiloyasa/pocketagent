package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

/**
 * D7-3:网页快照的「截至时刻」([OfflineStore.UrlSnapshot.capturedAt])钉子 ——
 * 离线兜底 toast 要能说清"显示的是哪一刻的网页"。
 *
 * 【口径】capturedAt 优先取自快照**文件名内嵌**的十六进制 epoch 毫秒段
 * (`<urlhash>.<capturedAtHex>.<hexMime>.<hexName>`),写路径([OfflineStore.putUrl])
 * 默认带写入时刻 —— 文件名不会被 LRU touch 改写,第二次查看不再漂移成
 * "上次查看时间"(当年取 mtime 的漂移缺陷在此钉死);
 * 旧格式文件(无时间戳段)照旧 mtime 回退,读不到(<=0)= 无时间戳,
 * 展示层回退旧文案(见 BrowserScreen.offlineSnapshotToastCopy)。
 */
class OfflineSnapshotCapturedAtTest {

    @get:Rule
    val tmp = TemporaryFolder()

    /** 与 OfflineStore 内部同一套 hex 编码(测试侧自备,钉死文件名格式)。 */
    private fun hex(s: String): String =
        s.toByteArray(Charsets.UTF_8).joinToString("") { "%02x".format(it.toInt() and 0xff) }

    private fun urlDir(): File = requireNotNull(OfflineStore.dirFor("url"))

    @Test
    fun `roundtrip preserves capturedAt from write path`() {
        OfflineStore.initForTest(tmp.newFolder("offline"))
        val at = 1728451200123L
        OfflineStore.putUrl("https://example.com/a", byteArrayOf(1, 2, 3), "text/html", "示例页", at)
        val snap = OfflineStore.getUrl("https://example.com/a")
        assertNotNull(snap)
        assertEquals("text/html", snap!!.mime)
        assertEquals("示例页", snap.name)
        assertEquals("capturedAt 应来自写入时间戳且精确一致", at, snap.capturedAt)
    }

    @Test
    fun `put without explicit timestamp stamps write time`() {
        OfflineStore.initForTest(tmp.newFolder("offline-default"))
        val before = System.currentTimeMillis()
        OfflineStore.putUrl("https://example.com/now", byteArrayOf(1), "text/html", "n")
        val after = System.currentTimeMillis()
        val snap = OfflineStore.getUrl("https://example.com/now")
        assertNotNull(snap)
        assertTrue(
            "默认参数应取写入时刻(before=%d got=%d after=%d)".format(before, snap!!.capturedAt, after),
            snap.capturedAt in before..after,
        )
    }

    @Test
    fun `capturedAt comes from filename and never drifts on re-view`() {
        OfflineStore.initForTest(tmp.newFolder("offline-drift"))
        val at = 1728451200123L
        OfflineStore.putUrl("https://example.com/d", byteArrayOf(1), "text/html", "页", at)
        // 把文件 mtime 改成明显不同的时刻:新格式必须以文件名内嵌时间为准,与 mtime 无关
        val f = urlDir().listFiles()!!.single()
        val fakeMtime = at + 86_400_000L
        assertTrue(f.setLastModified(fakeMtime))
        assertEquals("mtime ≠ 文件名时间戳时应取文件名", at, OfflineStore.getUrl("https://example.com/d")!!.capturedAt)
        // 第二次查看:getUrl 会做 LRU touch(setLastModified=现在),capturedAt 不得跟着漂移
        assertEquals("LRU touch 后重看不得漂移", at, OfflineStore.getUrl("https://example.com/d")!!.capturedAt)
        assertTrue("touch 本身应发生过(mtime 被改写)", f.lastModified() >= fakeMtime)
    }

    @Test
    fun `legacy two-segment filename falls back to mtime`() {
        OfflineStore.initForTest(tmp.newFolder("offline-legacy"))
        val url = "https://example.com/legacy"
        // 旧格式:<urlhash>.<hexMime>.<hexName>(无时间戳段),不迁移直接可读
        val legacy = File(urlDir(), OfflineStore.cacheKey(url) + "." + hex("text/html") + "." + hex("旧页面"))
        legacy.writeBytes(byteArrayOf(9, 8))
        val at = 1700000000000L
        assertTrue(legacy.setLastModified(at))
        val snap = OfflineStore.getUrl(url)
        assertNotNull(snap)
        assertEquals("text/html", snap!!.mime)
        assertEquals("旧页面", snap.name)
        assertEquals("旧格式应回退 mtime", at, snap.capturedAt)
    }

    @Test
    fun `zero or malformed timestamp segment falls back to mtime`() {
        OfflineStore.initForTest(tmp.newFolder("offline-fallback"))
        val dir = urlDir()
        val at = 1700000000000L
        // 写入 <=0 → 文件名记 "0" 占位,读取回退 mtime
        OfflineStore.putUrl("https://example.com/z", byteArrayOf(1), "text/html", "z", 0L)
        val zeroFile = dir.listFiles()!!.single()
        assertTrue("capturedAt 段应为 0 占位:${zeroFile.name}", zeroFile.name.contains(".0."))
        assertTrue(zeroFile.setLastModified(at))
        assertEquals(at, OfflineStore.getUrl("https://example.com/z")!!.capturedAt)
        // 首段非合法十六进制 → 同样回退 mtime,不抛异常
        val bad = File(dir, OfflineStore.cacheKey("https://example.com/bad") + ".zz." + hex("text/html") + "." + hex("b"))
        bad.writeBytes(byteArrayOf(1))
        assertTrue(bad.setLastModified(at))
        val snap = OfflineStore.getUrl("https://example.com/bad")
        assertNotNull(snap)
        assertEquals(at, snap!!.capturedAt)
        assertEquals("text/html", snap.mime)
    }

    @Test
    fun `extra trailing segments degrade safely without crash`() {
        OfflineStore.initForTest(tmp.newFolder("offline-extra"))
        val dir = urlDir()
        val at = 1728451200123L
        // >3 段(防御解析):时间戳/mime 仍可解,尾段 junk 使 name 反解失败 → 空串,不编造
        val f = File(dir, OfflineStore.cacheKey("https://example.com/x") + "." + at.toString(16) + "." + hex("text/html") + "." + hex("名") + ".zz")
        f.writeBytes(byteArrayOf(1))
        val snap = OfflineStore.getUrl("https://example.com/x")
        assertNotNull(snap)
        assertEquals(at, snap!!.capturedAt)
        assertEquals("text/html", snap.mime)
        assertEquals("", snap.name)
    }

    @Test
    fun `new format does not regress legacy reader semantics`() {
        OfflineStore.initForTest(tmp.newFolder("offline-semantics"))
        // 新格式文件名恰好也是"全 hex 段",但段数判据下 mime/name 必须与写入一致
        OfflineStore.putUrl("https://example.com/sem", byteArrayOf(1, 2), "text/html", "p.html", 1728451200123L)
        val name = urlDir().listFiles()!!.single().name
        assertEquals("新格式 = urlhash + 3 段", 4, name.split('.').size)
        val snap = OfflineStore.getUrl("https://example.com/sem")
        assertNotNull(snap)
        assertEquals("text/html", snap!!.mime)
        assertEquals("p.html", snap.name)
        assertFalse(snap.name.isEmpty())
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
