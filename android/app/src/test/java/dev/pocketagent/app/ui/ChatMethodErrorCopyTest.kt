package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * 提示条「方法名 → 人话标题」纯函数的单测(L2 真机挑刺:横幅标题露出内部方法名)。
 * 口径:标题换人话,正文保持错误原文一字不动;名单外的方法返回 null,原文照抄。
 */
class ChatMethodErrorCopyTest {

    // ---- rpcMethodLabel ----

    @Test
    fun `三个已知方法换人话`() {
        assertEquals("恢复会话", rpcMethodLabel("session/resume"))
        assertEquals("发送消息", rpcMethodLabel("session/prompt"))
        assertEquals("新建会话", rpcMethodLabel("session/new"))
    }

    @Test
    fun `名单外方法返回null_原文照抄`() {
        assertNull(rpcMethodLabel("thread/get"))
        assertNull(rpcMethodLabel("session/close"))
        assertNull(rpcMethodLabel(""))
    }

    // ---- humanizeMethodError ----

    @Test
    fun `首行为方法名的两行报错_标题换人话正文保留原文`() {
        val raw = "session/resume\n2b070521-93cf-4c56-971a-d16538e1a2bc 失败:"
        val (title, body) = humanizeMethodError(raw)!!
        assertEquals("恢复会话", title)
        assertEquals("2b070521-93cf-4c56-971a-d16538e1a2bc 失败:", body)
    }

    @Test
    fun `方法名与原因同行_英文冒号分隔`() {
        val (title, body) = humanizeMethodError("session/prompt: 电脑端没有响应")!!
        assertEquals("发送消息", title)
        assertEquals("电脑端没有响应", body)
    }

    @Test
    fun `方法名与原因同行_中文冒号分隔`() {
        val (title, body) = humanizeMethodError("session/new：无法创建会话")!!
        assertEquals("新建会话", title)
        assertEquals("无法创建会话", body)
    }

    @Test
    fun `方法名后无正文_正文为空串`() {
        val (title, body) = humanizeMethodError("session/resume")!!
        assertEquals("恢复会话", title)
        assertEquals("", body)
    }

    @Test
    fun `未知方法名前缀_整条不改写`() {
        assertNull(humanizeMethodError("thread/get\n some detail"))
        assertNull(humanizeMethodError("UNAVAILABLE: 网络不可达"))
    }

    @Test
    fun `普通文案与空串_不改写`() {
        assertNull(humanizeMethodError("正在重新获取电脑端任务结果"))
        assertNull(humanizeMethodError(""))
        assertNull(humanizeMethodError("   "))
    }
}
