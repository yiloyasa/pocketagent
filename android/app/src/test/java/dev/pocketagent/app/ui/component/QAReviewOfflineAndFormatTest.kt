package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * QA 独立复核补充测试(项 2 离线兜底边界 + 项 6 缓存纯函数边界 + 项 7 格式/导航边界)。
 *
 * 【为什么补这些】现有 WorkflowC 系列测的是"主路径",这里专攻**边界与口径**:
 *  - 离线用时缓存的"陈旧判定"必须严格大于(等于阈值不算陈旧),否则会平白多一条日期标签;
 *  - 部分损坏的缓存载荷应"尽量还原"而不是整体丢弃(丢整条 = 断网时数字归零);
 *  - 文档格式 / 导航策略里大小写、带点、空串、端口、百分号编码这些最容易漏的输入;
 *  - LRU 淘汰的临界值(total == 上限 时不能多删一个)。
 *
 * 只读断言,不改任何源码。
 */
class QAReviewOfflineAndFormatTest {

    // ---------------- 项 2:离线兜底(UsageCache 纯逻辑边界) ----------------

    @Test
    fun staleness_threshold_is_strictly_greater_than() {
        // now - updatedAt 恰好等于阈值:不算陈旧(边界必须稳定,否则提示条会在阈值那天跳变)
        assertFalse(isUsageStale(1_000, 1_000 + USAGE_STALE_AFTER_MS))
        assertTrue(isUsageStale(1_000, 1_000 + USAGE_STALE_AFTER_MS + 1))
        // 无缓存(<=0)永不判陈旧
        assertFalse(isUsageStale(0, Long.MAX_VALUE))
    }

    @Test
    fun partially_valid_payload_is_restored_not_discarded() {
        // 只有 cw / at:其余字段缺失也要能解出来(断网时宁可"部分数字"也不要空态)
        val snap = decodeUsageSnapshot("""{"cw":200000,"at":123}""")
        assertNotNull("部分字段缺失不应整体判为损坏", snap)
        assertEquals(200_000L, snap!!.contextWindow)
        assertEquals(123L, snap.updatedAt)
        assertNull("缺 turn 就应是 null,而不是抛异常/造 0 值", snap.turnUsage)
        assertFalse(snap.windowIsFallback)
    }

    @Test
    fun null_usage_encodes_to_null() {
        assertNull(usageToJson(null))
    }

    @Test
    fun offline_label_for_cached_data_always_says_offline_and_marks_freshness() {
        val now = System.currentTimeMillis()
        // 同一天(updatedAt 取 now 本身,跨午夜也不会误判):只给时刻
        val sameDay = usageOfflineLabel(now, now)
        assertTrue(sameDay.startsWith("离线 · 最后更新"))
        assertFalse("同一天不该补日期", sameDay.contains("月"))
        // 跨天(3 天前):必须补出日期,避免把旧数据当刚刚的
        val older = usageOfflineLabel(now - 3L * 24 * 60 * 60 * 1000, now)
        assertTrue(older.startsWith("离线 · 最后更新"))
        assertTrue("跨天必须带日期", older.contains("月"))
    }

    @Test
    fun same_calendar_day_helper_is_reflexive_and_date_sensitive() {
        val now = System.currentTimeMillis()
        assertTrue(sameCalendarDay(now, now))
        assertFalse(sameCalendarDay(now, now + 3L * 24 * 60 * 60 * 1000))
    }

    // ---------------- 项 6:OfflineStore LRU / 统计边界 ----------------

    @Test
    fun select_for_eviction_never_over_deletes_at_the_boundary() {
        val e = listOf(
            OfflineStore.CacheEntry("a", 40, 1),
            OfflineStore.CacheEntry("b", 60, 2),
        )
        // 总量恰好等于上限:一个都不删
        assertEquals(emptyList<String>(), OfflineStore.selectForEviction(e, 100).map { it.name })
        // 只差 1 字节:也只删最旧的那一个,刚好压回上限
        assertEquals(listOf("a"), OfflineStore.selectForEviction(e, 99).map { it.name })
        // 空集:恒为空,不崩
        assertEquals(emptyList<String>(), OfflineStore.selectForEviction(emptyList(), 0))
    }

    @Test
    fun cache_key_is_stable_and_handles_special_chars() {
        // 含特殊字符/超长/中文的键都必须映射成等长 32 位十六进制(可做文件名)
        listOf("https://a/b?x=1&y=中文", "", " ", "a".repeat(500)).forEach { raw ->
            val k = OfflineStore.cacheKey(raw)
            assertEquals(32, k.length)
            assertTrue(k.all { it in "0123456789abcdef" })
        }
    }

    // ---------------- 项 7:DocFormatRegistry 边界 ----------------

    @Test
    fun registry_tolerates_blank_and_dotless_inputs() {
        assertNull(DocFormatRegistry.matchExt(""))
        assertNull(DocFormatRegistry.matchExt("   "))
        assertEquals("", DocFormatRegistry.extOf("noext"))
        assertNull(DocFormatRegistry.formatFor("", ""))
        // 未知格式一律交给系统应用,不崩
        assertEquals(DocOpenMode.OPEN_SYSTEM, DocFormatRegistry.modeFor("namewithoutdot"))
    }

    @Test
    fun registry_matches_mime_case_insensitively_and_strips_params() {
        assertEquals(".pdf", DocFormatRegistry.extFor("APPLICATION/PDF"))
        assertEquals(".md", DocFormatRegistry.extFor("Text/Markdown; charset=utf-8"))
    }

    @Test
    fun compound_archive_extension_resolves_to_save_only() {
        // .tar.gz 的 lastSeg 是 gz,须落进压缩包分类 → SAVE_ONLY(不假装能内置预览)
        assertEquals(DocOpenMode.SAVE_ONLY, DocFormatRegistry.modeFor("backup.tar.gz"))
        assertEquals(FileKind.ARCHIVE, DocFormatRegistry.formatFor("backup.tar.gz")!!.kind)
    }

    // ---------------- 项 7:WebNavPolicy 边界 ----------------

    @Test
    fun web_scheme_detection_is_case_insensitive() {
        assertTrue(WebNavPolicy.isWebScheme("HTTPS://Www.Baidu.com"))
        assertTrue(WebNavPolicy.isWebScheme("  https://x  "))
        assertFalse(WebNavPolicy.isWebScheme(""))
    }

    @Test
    fun external_scheme_detection_is_case_insensitive_and_rejects_relative() {
        assertTrue(WebNavPolicy.isExternalScheme("Weixin://dl/chat"))
        assertTrue(WebNavPolicy.isExternalScheme("INTENT://x#Intent;end"))
        // 相对链接(无 scheme)一律不是外跳
        assertFalse(WebNavPolicy.isExternalScheme("/a/b"))
        assertFalse(WebNavPolicy.isExternalScheme("//host/path"))
        assertFalse(WebNavPolicy.isExternalScheme(":"))
    }

    @Test
    fun trusted_local_host_covers_loopback_and_only_private_ranges() {
        // 调用方传的是 Uri.host(不含端口)
        assertTrue(WebNavPolicy.trustedLocalHost("127.0.0.1"))
        assertTrue(WebNavPolicy.trustedLocalHost("localhost"))
        assertTrue(WebNavPolicy.trustedLocalHost("[::1]"))
        // 全网卡地址不是"本地可信"—— 证书错误时绝不能放行
        assertFalse(WebNavPolicy.trustedLocalHost("0.0.0.0"))
        // 公网/域名一律不可信
        assertFalse(WebNavPolicy.trustedLocalHost("www.baidu.com"))
        assertFalse(WebNavPolicy.trustedLocalHost("100.63.0.1"))
    }

    @Test
    fun disposition_filename_prefers_rfc5987_then_plain_then_url() {
        // 同时有 filename* 与 filename:必须优先 RFC5987(百分号编码)
        assertEquals(
            "报告.pdf",
            WebNavPolicy.fileNameFromDisposition(
                "attachment; filename*=UTF-8''%E6%8A%A5%E5%91%8A.pdf; filename=\"x.pdf\"",
                "https://x/a",
            ),
        )
        // 大小写不敏感
        assertEquals(
            "a.txt",
            WebNavPolicy.fileNameFromDisposition("attachment; FILENAME=\"a.txt\"", "https://x/a"),
        )
        // 都没有:从 URL 末段取,剥掉 query / fragment
        assertEquals("b.zip", WebNavPolicy.fileNameFromDisposition("", "https://x/a/b.zip?t=1#f"))
        // URL 也没有文件名:给兜底名而不是空串
        assertEquals("下载文件", WebNavPolicy.fileNameFromDisposition("", "https://x/"))
    }
}
