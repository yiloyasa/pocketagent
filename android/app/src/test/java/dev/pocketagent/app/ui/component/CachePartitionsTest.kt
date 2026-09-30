package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.CacheCleanupLogic
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

/**
 * 缓存分区注册表三项化(2026-09-30)的钉子:
 *  1. 注册表恰有三项分区(url/checkpoint/snapshot),口径与各自自管 store 的上限同源;
 *  2. 逻辑分区的统计/清理走 [OfflineStore.partitionStats] / [OfflineStore.clear],
 *     物理目录经 PaPaths 注入(磁盘契约不迁移不重命名);
 *  3. TurnCheckpointStore / ThreadSnapshot 的只读 usage(字节+条目数)与 LRU 常量值不变;
 *  4. ThreadSnapshot.Msg.savedAtMs 向后兼容:新写入有时间戳,旧 JSON 读回 0;
 *  5. CacheCleanupLogic:检查点 = SAFE 可单独清,会话快照仍 RISKY,凭据仍永不出局。
 */
class CachePartitionsTest {

    @get:Rule
    val tmp = TemporaryFolder()

    private lateinit var root: File
    private lateinit var ckptDir: File
    private lateinit var snapDir: File

    @Before
    fun setUp() {
        root = tmp.newFolder("offline")
        ckptDir = tmp.newFolder("turn-checkpoints")
        snapDir = tmp.newFolder("pa_snap")
        OfflineStore.initForTest(
            root,
            anchors = mapOf(
                OfflineStore.Anchor.TURN_CHECKPOINTS to ckptDir,
                OfflineStore.Anchor.SNAPSHOT to snapDir,
            ),
        )
    }

    /** 与 ThreadSnapshot.msgFileName 同算法:md5(threadId) + .json */
    private fun msgFile(threadId: String): File {
        val md = java.security.MessageDigest.getInstance("MD5")
        val name = md.digest(threadId.toByteArray(Charsets.UTF_8))
            .joinToString("") { "%02x".format(it.toInt() and 0xff) } + ".json"
        return File(snapDir, name)
    }

    // ---- 注册表形状 ----

    @Test
    fun registryHasExactlyThreePartitionsWithStableContracts() {
        assertEquals(listOf("url", "checkpoint", "snapshot"), OfflineStore.partitions.map { it.id })
        assertEquals(listOf("网页快照", "轮次检查点", "会话快照"), OfflineStore.partitions.map { it.label })
        val byId = OfflineStore.partitions.associateBy { it.id }
        // 网页快照:物理分区,80MB / 30 天 TTL,可清理
        assertEquals(OfflineStore.Anchor.OFFLINE, byId.getValue("url").anchor)
        assertEquals(80L * 1024 * 1024, byId.getValue("url").maxBytes)
        assertTrue(byId.getValue("url").ttlMs > 0)
        // 轮次检查点 / 会话快照:逻辑分区,上限与各自 store 的常量同源
        assertEquals(OfflineStore.Anchor.TURN_CHECKPOINTS, byId.getValue("checkpoint").anchor)
        assertEquals(TurnCheckpointStore.MAX_TOTAL_BYTES, byId.getValue("checkpoint").maxBytes)
        assertEquals(OfflineStore.Anchor.SNAPSHOT, byId.getValue("snapshot").anchor)
        assertEquals(5L * 1024 * 1024, byId.getValue("snapshot").maxBytes)
        // 三项都可清理,但分级语义由 CacheCleanupLogic 决定(检查点 SAFE / 快照 RISKY)
        assertTrue(byId.values.all { it.clearable })
    }

    // ---- 分区统计与清理(逻辑分区走 PaPaths 注入的物理目录) ----

    @Test
    fun partitionStatsReportsBytesEntriesAndClearability() {
        File(ckptDir, "a.json").writeText("{\"thread\":\"a\"}")
        File(ckptDir, "b.json").writeText("{\"thread\":\"bbbb\"}")
        val stats = OfflineStore.partitionStats("checkpoint")!!
        assertEquals("轮次检查点", stats.label)
        assertEquals(2, stats.entries)
        assertEquals(File(ckptDir, "a.json").length() + File(ckptDir, "b.json").length(), stats.bytes)
        assertTrue(stats.clearable)
        // 目录还没被建出(未 init 的极端态)→ 0 值但不丢"可清理性"元数据
        val bare = OfflineStore.partitionStats("snapshot")!!
        assertEquals(0L, bare.bytes)
        assertEquals(0, bare.entries)
        assertTrue(bare.clearable)
        // 未知 id → null(与 dirFor 同款口径)
        assertNull(OfflineStore.partitionStats("nope"))
        // 一次取齐三项
        assertEquals(listOf("url", "checkpoint", "snapshot"), OfflineStore.allPartitionStats().map { it.id })
    }

    @Test
    fun clearCheckpointPartitionFreesBytesAndKeepsOthersIntact() {
        File(ckptDir, "a.json").writeText("12345")
        File(snapDir, "keep.json").writeText("678")
        val freed = OfflineStore.clear("checkpoint")
        assertEquals(5L, freed)
        assertEquals(0, ckptDir.listFiles()?.size ?: -1)
        // 会话快照分区不被殃及(各分区清理各走各的目录)
        assertEquals(1, snapDir.listFiles()?.size)
    }

    // ---- TurnCheckpointStore:只读 usage + 常量值不变 ----

    @Test
    fun checkpointUsageCountsJsonFilesAndBytes() {
        assertEquals(TurnCheckpointStore.Usage(0L, 0), TurnCheckpointStore.usage())
        TurnCheckpointStore.initForTest(ckptDir)
        TurnCheckpointStore.save(
            TurnCheckpointStore.Checkpoint(
                "t1", listOf(ThreadSnapshot.Msg("user", "hi")), "流", "想", 1,
                true, 0, "m", null, null,
            ),
        )
        TurnCheckpointStore.save(
            TurnCheckpointStore.Checkpoint(
                "t2", emptyList(), "", "", 2,
                false, 0, "", null, "e",
            ),
        )
        val usage = TurnCheckpointStore.usage()
        assertEquals(2, usage.entries)
        assertTrue(usage.bytes > 0)
        // LRU 上限常量:值不变(原 save() 内硬编码 50 / 8MB)
        assertEquals(50, TurnCheckpointStore.MAX_CHECKPOINTS)
        assertEquals(8L * 1024 * 1024, TurnCheckpointStore.MAX_TOTAL_BYTES)
        // 删一个后条目数随之下降(既有 delete 语义保持)
        TurnCheckpointStore.delete("t1")
        assertEquals(1, TurnCheckpointStore.usage().entries)
    }

    // ---- ThreadSnapshot:usageStats + savedAtMs 兼容 ----

    @Test
    fun snapshotUsageStatsCountsOnlySnapshotDataFiles() {
        ThreadSnapshot.initForTest(snapDir)
        ThreadSnapshot.saveThreadsSnapshot(listOf(ThreadSnapshot.Row("a", "标题", 1L, "/w", "", false)))
        ThreadSnapshot.saveMessages("a", listOf(ThreadSnapshot.Msg("user", "内容")))
        val stats = ThreadSnapshot.usageStats()
        // 条目 = 消息快照文件;threads.json 是列表级文件,不计入
        assertEquals(1, stats.entries)
        assertEquals(msgFile("a").length(), stats.bytes)
        // 旧口径 usage()(含列表级文件)保持原语义
        assertTrue(ThreadSnapshot.usage() >= stats.bytes)
    }

    @Test
    fun savedAtMsIsStampedOnWriteAndReadsZeroFromLegacyJson() {
        ThreadSnapshot.initForTest(snapDir)
        ThreadSnapshot.saveMessages("new", listOf(ThreadSnapshot.Msg("user", "新写入")))
        val fresh = ThreadSnapshot.readThreadSnapshot("new")!!
        assertEquals(1, fresh.size)
        assertTrue("新写入的消息应带上落盘时间戳", fresh[0].savedAtMs > 0L)

        // 旧格式 JSON(只有 r/t,没有 at):读回 role/text 完整,savedAtMs = 0(向后兼容)
        msgFile("legacy").writeText("""[{"r":"user","t":"旧消息"}]""")
        val read = ThreadSnapshot.readThreadSnapshot("legacy")!!
        assertEquals(listOf(ThreadSnapshot.Msg("user", "旧消息", 0L)), read)
        assertEquals("旧消息", read[0].text)
        assertEquals(0L, read[0].savedAtMs)
    }

    // ---- CacheCleanupLogic:检查点 SAFE、快照 RISKY、凭据永不出局 ----

    @Test
    fun checkpointIsSafeWhileSnapshotStaysRiskyAndCredentialNeverListed() {
        assertEquals(CacheCleanupLogic.Risk.SAFE, CacheCleanupLogic.riskOf(CacheCleanupLogic.Kind.CHECKPOINT))
        assertEquals(CacheCleanupLogic.Risk.RISKY, CacheCleanupLogic.riskOf(CacheCleanupLogic.Kind.SNAPSHOT))
        assertTrue(CacheCleanupLogic.isClearable(CacheCleanupLogic.Kind.CHECKPOINT))
        assertFalse(CacheCleanupLogic.isCredential(CacheCleanupLogic.Kind.CHECKPOINT))

        val items = listOf(
            CacheCleanupLogic.Item("会话快照", CacheCleanupLogic.Kind.SNAPSHOT, 100, 3),
            CacheCleanupLogic.Item("轮次检查点", CacheCleanupLogic.Kind.CHECKPOINT, 40, 2),
            CacheCleanupLogic.Item("网页快照", CacheCleanupLogic.Kind.URL_SNAPSHOT, 200, 5),
            CacheCleanupLogic.Item("token", CacheCleanupLogic.Kind.CREDENTIAL, 50),
        )
        // 三参构造仍兼容(entries 默认 0)
        assertEquals(0, CacheCleanupLogic.Item("x", CacheCleanupLogic.Kind.TEMP, 1L).entries)
        // 检查点在可直清项里;高风险只剩会话快照;凭据永不出现在任何清理目标里
        assertTrue(CacheCleanupLogic.safeItems(items).map { it.key }.containsAll(listOf("轮次检查点", "网页快照")))
        assertEquals(listOf("会话快照"), CacheCleanupLogic.riskyItems(items).map { it.key })
        assertTrue(CacheCleanupLogic.filterClearable(items).none { it.kind == CacheCleanupLogic.Kind.CREDENTIAL })
        assertEquals(3, items.first { it.key == "会话快照" }.entries)
    }
}
