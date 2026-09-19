package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.PcmRecognizeEngine
import dev.pocketagent.app.ui.component.VOICE_ERROR_VOSK_ENGINE
import dev.pocketagent.app.ui.component.VOICE_ERROR_VOSK_MODEL
import dev.pocketagent.app.ui.component.VoskModelEvent
import dev.pocketagent.app.ui.component.VoskModelFetcher
import dev.pocketagent.app.ui.component.VoskModelFiles
import dev.pocketagent.app.ui.component.VoskModelInstaller
import dev.pocketagent.app.ui.component.VoskModelPhase
import dev.pocketagent.app.ui.component.VoskModelState
import dev.pocketagent.app.ui.component.runVoskRecognitionLoop
import dev.pocketagent.app.ui.component.voskJsonText
import dev.pocketagent.app.ui.component.voskModelReduce
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.io.FileOutputStream
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream

/**
 * 端侧离线语音识别(Vosk 回落)的纯逻辑单测:
 *  1. 引擎选择决策表(系统可用 / Vosk 就绪 → SYSTEM/VOSK/UNAVAILABLE);
 *  2. 模型管理状态机(未下载/下载中/就绪/失败重试,网络层 fake);
 *  3. 模型文件校验与安全解压(zip-slip 防护);
 *  4. 安装编排(下载 → 解压 → 校验 → 成功/失败事件,fetcher 可注入);
 *  5. 识别主循环调度(PcmRecognizeEngine + 数据源均为 fake,Vosk 原生库不参与);
 *  6. Vosk JSON 结果提取与端侧错误码映射。
 */
class VoskVoiceFallbackTest {

    @get:Rule
    val tmp = TemporaryFolder()

    // ------------------------------------------------------------ 引擎选择决策表

    @Test
    fun `引擎选择_系统可用一律SYSTEM_在线质量优先`() {
        assertEquals(VoiceEngineKind.SYSTEM, pickVoiceEngine(sysAvailable = true, voskReady = true))
        assertEquals(VoiceEngineKind.SYSTEM, pickVoiceEngine(sysAvailable = true, voskReady = false))
    }

    @Test
    fun `引擎选择_系统不可用模型就绪走VOSK_都没Ready则UNAVAILABLE`() {
        // 小米 sdk=36 isRecognitionAvailable=false 的目标场景:模型下完即可用
        assertEquals(VoiceEngineKind.VOSK, pickVoiceEngine(sysAvailable = false, voskReady = true))
        assertEquals(VoiceEngineKind.UNAVAILABLE, pickVoiceEngine(sysAvailable = false, voskReady = false))
    }

    // ------------------------------------------------------------ 模型状态机

    private val initial = VoskModelState()

    private fun reduceAll(vararg events: VoskModelEvent): VoskModelState =
        events.fold(initial) { st, e -> voskModelReduce(st, e) }

    @Test
    fun `状态机_下载中进度按total折算_超发封顶1`() {
        var st = reduceAll(VoskModelEvent.DownloadStarted)
        assertEquals(VoskModelPhase.DOWNLOADING, st.phase)
        st = voskModelReduce(st, VoskModelEvent.DownloadProgress(25, 100))
        assertEquals(0.25f, st.progress, 0.0001f)
        assertEquals(25L, st.downloadedBytes)
        // total 未知:progress 保持 0,只涨字节数(UI 显示"已下载 xx")
        st = voskModelReduce(st, VoskModelEvent.DownloadProgress(50, 0))
        assertEquals(0f, st.progress, 0.0001f)
        assertEquals(50L, st.downloadedBytes)
        // 字节数异常超过 total:封顶,不出负/超1
        st = voskModelReduce(st, VoskModelEvent.DownloadProgress(200, 100))
        assertEquals(1f, st.progress, 0.0001f)
    }

    @Test
    fun `状态机_乱序进度事件不污染非下载态`() {
        // FAILED 后迟到的进度事件(协程取消前最后一拍)不允许改写终态
        val failed = reduceAll(VoskModelEvent.DownloadStarted, VoskModelEvent.Failed("网络断开"))
        val st = voskModelReduce(failed, VoskModelEvent.DownloadProgress(99, 100))
        assertEquals(VoskModelPhase.FAILED, st.phase)
        assertEquals("网络断开", st.message)
        // READY 后同理
        val ready = reduceAll(VoskModelEvent.DownloadStarted, VoskModelEvent.Succeeded(42_000L))
        val st2 = voskModelReduce(ready, VoskModelEvent.DownloadProgress(1, 2))
        assertEquals(VoskModelPhase.READY, st2.phase)
        assertEquals(42_000L, st2.diskBytes)
    }

    @Test
    fun `状态机_解压置进度满格_成功进就绪并记磁盘占用`() {
        val st = reduceAll(
            VoskModelEvent.DownloadStarted,
            VoskModelEvent.DownloadProgress(42_000, 42_000),
            VoskModelEvent.Extracting,
        )
        assertEquals(VoskModelPhase.DOWNLOADING, st.phase)
        assertEquals(1f, st.progress, 0.0001f)
        assertEquals("解压校验中", st.message)
        val ready = voskModelReduce(st, VoskModelEvent.Succeeded(41_800L))
        assertEquals(VoskModelPhase.READY, ready.phase)
        assertEquals(41_800L, ready.diskBytes)
        assertTrue(ready.ready)
    }

    @Test
    fun `状态机_失败保留原因_重试从DownloadStarted重走`() {
        val st = reduceAll(
            VoskModelEvent.DownloadStarted,
            VoskModelEvent.DownloadProgress(10, 100),
            VoskModelEvent.Failed("HTTP 503"),
        )
        assertEquals(VoskModelPhase.FAILED, st.phase)
        assertEquals("HTTP 503", st.message)
        // 重试 = 重新 DownloadStarted,进度与消息全部清零
        val retried = voskModelReduce(st, VoskModelEvent.DownloadStarted)
        assertEquals(VoskModelPhase.DOWNLOADING, retried.phase)
        assertEquals(0f, retried.progress, 0.0001f)
        assertEquals("", retried.message)
    }

    @Test
    fun `状态机_磁盘刷新_就绪记占用_缺失回未下载_删除复位`() {
        val ready = voskModelReduce(initial, VoskModelEvent.DiskRefreshed(ready = true, diskBytes = 4096L))
        assertEquals(VoskModelPhase.READY, ready.phase)
        assertEquals(4096L, ready.diskBytes)
        // 模型被用户/清理软件删掉:回未下载(引擎选择随之 UNAVAILABLE → 引导下载)
        val missing = voskModelReduce(ready, VoskModelEvent.DiskRefreshed(ready = false, diskBytes = 0L))
        assertEquals(VoskModelPhase.NOT_DOWNLOADED, missing.phase)
        // 删除:复位为未下载
        val deleted = voskModelReduce(ready, VoskModelEvent.Deleted)
        assertEquals(VoskModelPhase.NOT_DOWNLOADED, deleted.phase)
        assertEquals(0L, deleted.diskBytes)
    }

    @Test
    fun `状态机_目录在但校验不过_半损坏回FAILED引导重下`() {
        // 先就绪,随后目录半损坏(conf 在、final.mdl 坏):目录还在但三件套不齐
        val ready = voskModelReduce(initial, VoskModelEvent.DiskRefreshed(ready = true, diskBytes = 42_000L))
        val corrupted = voskModelReduce(
            ready,
            VoskModelEvent.DiskRefreshed(ready = false, diskBytes = 0L, dirExists = true),
        )
        // 必须回 FAILED(可重试下载)而不是 NOT_DOWNLOADED,更不能滞留 READY:
        // 滞留 READY 时每轮识别重复失败而设置页仍显示"已下载"
        assertEquals(VoskModelPhase.FAILED, corrupted.phase)
        assertFalse(corrupted.ready)
        assertTrue(corrupted.message.contains("重新下载"))
        // FAILED 可重试:DownloadStarted 重走,进度与消息清零
        val retried = voskModelReduce(corrupted, VoskModelEvent.DownloadStarted)
        assertEquals(VoskModelPhase.DOWNLOADING, retried.phase)
        assertEquals("", retried.message)
        // 半损坏后重下成功:回到 READY 且磁盘占用刷新
        val healed = voskModelReduce(retried, VoskModelEvent.Succeeded(42_000L))
        assertEquals(VoskModelPhase.READY, healed.phase)
        assertEquals(42_000L, healed.diskBytes)
    }

    // ------------------------------------------------------------ 模型文件校验与解压

    /** 建一个最小合法模型目录(conf/model.conf + am/final.mdl + graph/HCLr.fst)。 */
    private fun buildValidModelDir(root: File, name: String = "vosk-model-small-cn-0.22", hclg: Boolean = false): File {
        val dir = File(root, name)
        File(dir, "conf").mkdirs()
        File(dir, "am").mkdirs()
        File(dir, "graph").mkdirs()
        File(dir, "conf/model.conf").writeText("conf")
        File(dir, "am/final.mdl").writeText("mdl")
        File(dir, "graph/${if (hclg) "HCLG.fst" else "HCLr.fst"}").writeText("fst")
        return dir
    }

    private fun zipModelDir(zip: File, topDir: String = "vosk-model-small-cn-0.22") {
        ZipOutputStream(FileOutputStream(zip)).use { zos ->
            fun put(name: String, content: String) {
                zos.putNextEntry(ZipEntry(name))
                zos.write(content.toByteArray())
                zos.closeEntry()
            }
            put("$topDir/conf/model.conf", "conf")
            put("$topDir/am/final.mdl", "mdl")
            put("$topDir/graph/HCLr.fst", "fst")
        }
    }

    @Test
    fun `文件校验_必需三件套齐才算模型_HCLr与HCLG皆可`() {
        val root = tmp.newFolder("models1")
        val dir = buildValidModelDir(root)
        assertTrue(VoskModelFiles.isValidModelDir(dir))
        // 大模型解码图变体
        val dir2 = buildValidModelDir(tmp.newFolder("models2"), hclg = true)
        assertTrue(VoskModelFiles.isValidModelDir(dir2))
        // 缺解码图 / 缺声学模型 / 空目录 / 不存在
        File(dir, "graph/HCLr.fst").delete()
        assertFalse(VoskModelFiles.isValidModelDir(dir))
        assertFalse(VoskModelFiles.isValidModelDir(File(dir, "conf")))
        assertFalse(VoskModelFiles.isValidModelDir(File(root, "nope")))
    }

    @Test
    fun `解压_带顶层目录展开后通过校验_占用为文件字节和`() {
        val root = tmp.newFolder("unzip-ok")
        val zip = File(root, "m.zip")
        zipModelDir(zip)
        VoskModelFiles.unzip(zip, root)
        val modelDir = File(root, "vosk-model-small-cn-0.22")
        assertTrue(VoskModelFiles.isValidModelDir(modelDir))
        val expected = "conf".length + "mdl".length + "fst".length
        assertEquals(expected.toLong(), VoskModelFiles.dirSize(modelDir))
        // 删除后目录消失,dirSize 归零
        assertTrue(VoskModelFiles.deleteRecursively(modelDir))
        assertEquals(0L, VoskModelFiles.dirSize(modelDir))
    }

    @Test
    fun `解压_zip-slip逃逸路径一律拒绝`() {
        val root = tmp.newFolder("unzip-evil")
        val zip = File(root, "evil.zip")
        ZipOutputStream(FileOutputStream(zip)).use { zos ->
            zos.putNextEntry(ZipEntry("../evil.txt"))
            zos.write("boom".toByteArray())
            zos.closeEntry()
        }
        val before = root.listFiles()?.size ?: 0
        try {
            VoskModelFiles.unzip(zip, root)
            throw AssertionError("zip-slip 未被拦截")
        } catch (e: SecurityException) {
            assertTrue(e.message!!.contains("非法路径"))
        }
        // 逃逸文件绝不能出现
        assertEquals(before, root.listFiles()?.size ?: 0)
        assertFalse(File(root.parentFile, "evil.txt").exists())
    }

    // ------------------------------------------------------------ 安装编排(fake fetcher)

    @Test
    fun `安装_下载解压校验全通_事件序列正确且zip清理`() {
        val root = tmp.newFolder("install-ok")
        val zip = File(root, "staged.zip")
        zipModelDir(zip)
        val staged = zip.readBytes()
        val fetcher = VoskModelFetcher { _, target, onProgress ->
            target.writeBytes(staged)
            onProgress(staged.size.toLong() / 2, staged.size.toLong())
            onProgress(staged.size.toLong(), staged.size.toLong())
        }
        val events = mutableListOf<VoskModelEvent>()
        VoskModelInstaller(fetcher, root) { events.add(it) }.install("https://example.com/m.zip")
        assertEquals(
            listOf(
                VoskModelEvent.DownloadStarted::class,
                VoskModelEvent.DownloadProgress::class, // fake fetcher 上报两次进度
                VoskModelEvent.DownloadProgress::class,
                VoskModelEvent.Extracting::class,
                VoskModelEvent.Succeeded::class,
            ),
            events.map { it::class },
        )
        assertTrue(events.last() is VoskModelEvent.Succeeded)
        assertTrue(VoskModelFiles.isValidModelDir(File(root, "vosk-model-small-cn-0.22")))
        // 成功后 zip 清掉,不留双份占磁盘
        assertFalse(File(root, "vosk-model-small-cn-0.22.zip").exists())
    }

    @Test
    fun `安装_下载失败_报Failed并清光半成品_可重试`() {
        val root = tmp.newFolder("install-fail")
        val fetcher = VoskModelFetcher { _, _, _ -> throw RuntimeException("网络断开") }
        val events = mutableListOf<VoskModelEvent>()
        VoskModelInstaller(fetcher, root) { events.add(it) }.install("https://example.com/m.zip")
        assertTrue(events.last() is VoskModelEvent.Failed)
        assertEquals("网络断开", (events.last() as VoskModelEvent.Failed).message)
        // 半成品清理:zip 与模型目录都不残留(残留会被下轮 install 开头再清一次)
        assertFalse(File(root, "vosk-model-small-cn-0.22.zip").exists())
        assertFalse(File(root, "vosk-model-small-cn-0.22").exists())
        // 重试:同一目录再装一次,这次网络通 → 成功(失败可重试)
        val zip = File(root, "staged.zip")
        zipModelDir(zip)
        val staged = zip.readBytes()
        val events2 = mutableListOf<VoskModelEvent>()
        val retryFetcher = VoskModelFetcher { _, target, _ -> target.writeBytes(staged) }
        VoskModelInstaller(retryFetcher, root) { events2.add(it) }.install("https://example.com/m.zip")
        assertTrue(events2.last() is VoskModelEvent.Succeeded)
    }

    @Test
    fun `安装_包内容缺文件_校验失败报Failed`() {
        val root = tmp.newFolder("install-badzip")
        val zip = File(root, "staged.zip")
        ZipOutputStream(FileOutputStream(zip)).use { zos ->
            zos.putNextEntry(ZipEntry("vosk-model-small-cn-0.22/readme.txt"))
            zos.write("只有readme的假包".toByteArray())
            zos.closeEntry()
        }
        val staged = zip.readBytes()
        val events = mutableListOf<VoskModelEvent>()
        VoskModelInstaller({ _, target, _ -> target.writeBytes(staged) }, root) { events.add(it) }
            .install("https://example.com/m.zip")
        val last = events.last()
        assertTrue(last is VoskModelEvent.Failed)
        assertTrue((last as VoskModelEvent.Failed).message.contains("不完整"))
        assertFalse(File(root, "vosk-model-small-cn-0.22").exists())
    }

    // ------------------------------------------------------------ SHA-256 哈希校验(下载后、解压前)

    @Test
    fun `哈希_sha256Hex标准向量_大小写与空白期望按跳过处理`() {
        val empty = tmp.newFile("empty.bin").also { it.writeBytes(ByteArray(0)) }
        assertEquals(
            "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
            VoskModelFiles.sha256Hex(empty),
        )
        val abc = tmp.newFile("abc.bin").also { it.writeBytes("abc".toByteArray()) }
        assertEquals(
            "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
            VoskModelFiles.sha256Hex(abc),
        )
        // 空白期望 = 跳过校验(容未来换包未更新常量,不把正常包拒之门外)
        assertTrue(VoskModelFiles.sha256Matches(abc, ""))
        assertTrue(VoskModelFiles.sha256Matches(abc, "   "))
        // 大写形式的期望值也能对上(比对忽略大小写)
        assertTrue(VoskModelFiles.sha256Matches(abc, VoskModelFiles.sha256Hex(abc).uppercase()))
        // 不匹配
        assertFalse(VoskModelFiles.sha256Matches(abc, "deadbeef"))
    }

    @Test
    fun `安装_哈希匹配_正常解压成功`() {
        val root = tmp.newFolder("install-hash-ok")
        val zip = File(root, "staged.zip")
        zipModelDir(zip)
        val staged = zip.readBytes()
        val expected = VoskModelFiles.sha256Hex(zip)
        val events = mutableListOf<VoskModelEvent>()
        VoskModelInstaller({ _, target, _ -> target.writeBytes(staged) }, root) { events.add(it) }
            .install("https://example.com/m.zip", expected)
        assertTrue(events.last() is VoskModelEvent.Succeeded)
        assertTrue(VoskModelFiles.isValidModelDir(File(root, "vosk-model-small-cn-0.22")))
    }

    @Test
    fun `安装_哈希不匹配_拒绝解压清残留报FAILED`() {
        val root = tmp.newFolder("install-hash-bad")
        val zip = File(root, "staged.zip")
        zipModelDir(zip) // 包本身是好的,但期望哈希对不上 = 传输损坏/被替换
        val staged = zip.readBytes()
        val events = mutableListOf<VoskModelEvent>()
        VoskModelInstaller({ _, target, _ -> target.writeBytes(staged) }, root) { events.add(it) }
            .install("https://example.com/m.zip", "3af8b0e7deadbeef")
        val last = events.last()
        assertTrue(last is VoskModelEvent.Failed)
        assertTrue((last as VoskModelEvent.Failed).message.contains("校验和不符"))
        // 坏包绝不落进模型目录:zip 与解压目录一律清掉(可重试)
        assertFalse(File(root, "vosk-model-small-cn-0.22.zip").exists())
        assertFalse(File(root, "vosk-model-small-cn-0.22").exists())
    }

    @Test
    fun `安装_期望哈希为空_跳过哈希校验照常成功`() {
        val root = tmp.newFolder("install-hash-skip")
        val zip = File(root, "staged.zip")
        zipModelDir(zip)
        val staged = zip.readBytes()
        val events = mutableListOf<VoskModelEvent>()
        VoskModelInstaller({ _, target, _ -> target.writeBytes(staged) }, root) { events.add(it) }
            .install("https://example.com/m.zip", "")
        assertTrue(events.last() is VoskModelEvent.Succeeded)
        assertTrue(VoskModelFiles.isValidModelDir(File(root, "vosk-model-small-cn-0.22")))
    }

    // ------------------------------------------------------------ 识别主循环(fake 引擎 + fake 数据源)

    /** 引擎 fake:第 segmentAt(1 起)块检出端点,各 JSON 固定。 */
    private class FakeEngine(private val segmentAt: Set<Int>) : PcmRecognizeEngine {
        var blocks = 0
        var resets = 0
        var closed = false
        override fun acceptWaveForm(data: ByteArray, len: Int): Boolean {
            blocks++
            return blocks in segmentAt
        }
        override fun segmentResultJson(): String = """{"text":"你好"}"""
        override fun partialResultJson(): String = """{"text":"你"}"""
        override fun finalResultJson(): String = """{"text":"世界"}"""
        override fun reset() { resets++ }
        override fun close() { closed = true }
    }

    @Test
    fun `循环_数据源自然结束_冲刷final并交付累积全文`() {
        val engine = FakeEngine(segmentAt = setOf(2))
        var reads = 0
        val alives = intArrayOf(0)
        val partials = mutableListOf<String>()
        val finished = arrayOfNulls<String>(1)
        runVoskRecognitionLoop(
            readBlock = { _ -> if (reads++ < 3) 100 else -1 },
            engine = engine,
            isCancelled = { false },
            onAlive = { alives[0]++ },
            onPartial = { partials.add(it) },
            onFinished = { finished[0] = it },
        )
        assertEquals(3, engine.blocks)
        // 端点分段(第2块)收"你好",收尾 final 冲出"世界"
        assertEquals("你好 世界", finished[0])
        // 首块音频成功喂入 = onAlive 恰好一次(等价 onReadyForSpeech,喂看门狗)
        assertEquals(1, alives[0])
        // 非端点块的中间稿(第1、3块)各回调一次
        assertEquals(listOf("你", "你"), partials)
    }

    @Test
    fun `循环_取消立即退出但已喂音频的final残余不丢`() {
        val engine = FakeEngine(segmentAt = emptySet())
        var reads = 0
        val finished = arrayOfNulls<String>(1)
        runVoskRecognitionLoop(
            readBlock = { _ -> if (reads++ < 5) 100 else -1 },
            engine = engine,
            isCancelled = { reads >= 2 }, // 第 2 块后用户点了停止
            onAlive = {},
            onPartial = {},
            onFinished = { finished[0] = it },
        )
        assertEquals(2, engine.blocks)
        assertEquals("世界", finished[0]) // stopListening 后仍冲出 final,识别不白听
    }

    @Test
    fun `循环_读块异常按源结束处理_绝不外抛`() {
        val engine = FakeEngine(segmentAt = emptySet())
        val finished = arrayOfNulls<String>(1)
        runVoskRecognitionLoop(
            readBlock = { _ -> throw IllegalStateException("AudioRecord 挂了") },
            engine = engine,
            isCancelled = { false },
            onAlive = {},
            onPartial = {},
            onFinished = { finished[0] = it },
        )
        assertEquals("世界", finished[0])
        assertEquals(0, engine.blocks) // 一次都没喂进引擎
    }

    // ------------------------------------------------------------ JSON 提取与端侧错误码

    @Test
    fun `vosk结果JSON提取_正常_空白_非法_空对象`() {
        assertEquals("你好世界", voskJsonText("""{"text": "你好世界"}"""))
        assertEquals("", voskJsonText("""{"text": "  "}"""))
        assertEquals("", voskJsonText("""{"text": ""}"""))
        assertEquals("", voskJsonText("""{"partial": "x"}"""))
        assertEquals("", voskJsonText("not-json{"))
        assertEquals("", voskJsonText(""))
    }

    @Test
    fun `端侧错误码映射到ENGINE_系统码映射不受影响`() {
        assertEquals(VoiceErrorKind.ENGINE, voiceErrorKind(VOICE_ERROR_VOSK_ENGINE))
        assertEquals(VoiceErrorKind.ENGINE, voiceErrorKind(VOICE_ERROR_VOSK_MODEL))
        // 系统码 1..9 原有映射不被新分支吞掉
        assertEquals(VoiceErrorKind.OTHER, voiceErrorKind(1))
        assertEquals(VoiceErrorKind.NO_MATCH, voiceErrorKind(7))
        assertEquals(VoiceErrorKind.BUSY, voiceErrorKind(8))
        assertEquals(VoiceErrorKind.INSUFFICIENT_PERMISSIONS, voiceErrorKind(9))
        // 新分类的人话提示
        assertEquals("端侧语音引擎异常,请重试", voiceErrorText(VoiceErrorKind.ENGINE))
    }

    @Test
    fun `模型加载失败错误码触发磁盘复核回连_其余一律不触发`() {
        // VOICE_ERROR_VOSK_MODEL = 模型加载失败:必须回连 refreshFromDisk,
        // 磁盘复核把半损坏目录打回 FAILED 引导重下(否则状态滞留 READY 反复失败)
        assertTrue(voiceModelRefreshNeeded(VOICE_ERROR_VOSK_MODEL))
        // 引擎运行异常(录音器/识别器创建失败)与模型目录完整性无关,不触发
        assertFalse(voiceModelRefreshNeeded(VOICE_ERROR_VOSK_ENGINE))
        // 系统识别错误码与其余任意码都不触发
        assertFalse(voiceModelRefreshNeeded(1))
        assertFalse(voiceModelRefreshNeeded(9))
        assertFalse(voiceModelRefreshNeeded(-1))
    }
}
