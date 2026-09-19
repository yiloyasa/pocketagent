package dev.pocketagent.app.ui.component

import java.io.File
import java.io.ByteArrayInputStream
import java.net.HttpURLConnection
import java.net.URL
import java.security.MessageDigest
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder

class SenseVoiceModelFilesTest {
    @get:Rule val tmp = TemporaryFolder()
    private val bytes = "test-model-payload".toByteArray()
    private val spec = SenseVoiceModelFile("model.bin",bytes.size.toLong(),
        MessageDigest.getInstance("SHA-256").digest(bytes).joinToString("") { "%02x".format(it) })
    private class Response(val payload: ByteArray, val code: Int, val range: String?=null) : HttpURLConnection(URL("https://test.invalid/model.bin")) {
        override fun connect() {}
        override fun disconnect() {}
        override fun usingProxy()=false
        override fun getResponseCode()=code
        override fun getHeaderField(name: String?): String? = if(name=="Content-Range") range else null
        override fun getInputStream()=ByteArrayInputStream(payload)
    }
    @Test fun sameSizeCorruptionIsRejected() {
        val root=tmp.newFolder()
        File(root,spec.name).writeBytes(ByteArray(bytes.size))
        File(root,"verified").writeText("old marker")
        assertFalse(SenseVoiceModelFiles(root,"",listOf(spec)).isReady())
    }
    @Test fun validResumeInstallsVerifiedPayload() {
        val root=tmp.newFolder()
        File(root,"${spec.name}.part").writeBytes(bytes.copyOf(4))
        val response=Response(bytes.copyOfRange(4,bytes.size),206,"bytes 4-${bytes.lastIndex}/${bytes.size}")
        val files=SenseVoiceModelFiles(root,"https://test.invalid/",listOf(spec)) { response }
        val progress=mutableListOf<Long>()
        files.install { progress.add(it) }
        assertEquals("bytes=4-",response.getRequestProperty("Range"))
        assertTrue(files.isReady())
        assertEquals(bytes.size.toLong(),progress.last())
        assertFalse(File(root,"${spec.name}.part").exists())
    }
    @Test fun ignoredRangeRestarts() {
        val root=tmp.newFolder()
        File(root,"${spec.name}.part").writeText("bad prefix")
        val files=SenseVoiceModelFiles(root,"https://test.invalid/",listOf(spec)) { Response(bytes,200) }
        files.install {}
        assertTrue(files.isReady())
    }
    @Test fun wrongRangeClearsPoisonedPartialThenRetries() {
        val root=tmp.newFolder()
        val partial=File(root,"${spec.name}.part").also { it.writeBytes(bytes.copyOf(4)) }
        var wrong=true
        val files=SenseVoiceModelFiles(root,"https://test.invalid/",listOf(spec)) {
            if(wrong) Response(bytes,206,"bytes 0-${bytes.lastIndex}/${bytes.size}") else Response(bytes,200)
        }
        assertThrows(Exception::class.java) { files.install {} }
        assertFalse(partial.exists())
        wrong=false
        files.install {}
        assertTrue(files.isReady())
    }
    @Test fun hashMismatchCanRetry() {
        val root=tmp.newFolder()
        var wrong=true
        val files=SenseVoiceModelFiles(root,"https://test.invalid/",listOf(spec)) { Response(if(wrong) ByteArray(bytes.size) else bytes,200) }
        assertThrows(Exception::class.java) { files.install {} }
        assertFalse(File(root,"${spec.name}.part").exists())
        wrong=false
        files.install {}
        assertTrue(files.isReady())
    }
    @Test fun completedPartialPromotesWithoutNetwork() {
        val root=tmp.newFolder()
        File(root,"${spec.name}.part").writeBytes(bytes)
        val files=SenseVoiceModelFiles(root,"https://test.invalid/",listOf(spec)) { error("network must not run") }
        files.install {}
        files.install {}
        assertTrue(files.isReady())
    }
}
