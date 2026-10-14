package dev.pocketagent.app.ui.component

import java.io.File

/** 与 ThreadSnapshot.msgFileName 同算法:md5(threadId) + .json */
internal fun msgFile(dir: File, threadId: String): File {
    val md = java.security.MessageDigest.getInstance("MD5")
    val name = md.digest(threadId.toByteArray(Charsets.UTF_8))
        .joinToString("") { "%02x".format(it.toInt() and 0xff) } + ".json"
    return File(dir, name)
}
