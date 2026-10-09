package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.theme.EditorialTone
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 统一错误态/空态组件(PaStateViews)纯逻辑的单测。
 *
 * 钉四件事:
 *  1. **错误映射表全表**:未配对词表 / 连不上词表 / 其余透传,每一档的
 *     headline、动作、色档、raw 保真逐条断言(改词表先改这里);
 *  2. 优先级:混合报错(如「UNAVAILABLE: token 无效」)未配对优先于连不上;
 *  3. 同屏判重:paIsSameErrorKind 按 kind 判同根源,空入参不判重;
 *  4. 空态判定纯函数:paEmptyStateMotifOf 语境 → duotone 母题一一对应,无语境返回 null。
 */
class PaStateViewsTest {

    // ---------------------------------------------------------------- paErrorKindOf(分级全表)

    @Test
    fun `未配对词表全命中`() {
        // gRPC 状态码原文(盘点实锤的主力文案)
        assertEquals(PaErrorKind.UNAUTHENTICATED, paErrorKindOf("UNAUTHENTICATED: token 无效"))
        assertEquals(PaErrorKind.UNAUTHENTICATED, paErrorKindOf("unauthenticated: invalid token"))
        // 设置页/控制台的人话变体
        assertEquals(PaErrorKind.UNAUTHENTICATED, paErrorKindOf("token 无效,请先配对"))
        assertEquals(PaErrorKind.UNAUTHENTICATED, paErrorKindOf("设备配对已失效,请重新连接电脑"))
        assertEquals(PaErrorKind.UNAUTHENTICATED, paErrorKindOf("尚未配对"))
        assertEquals(PaErrorKind.UNAUTHENTICATED, paErrorKindOf("通道未鉴权"))
        // 大小写不敏感
        assertEquals(PaErrorKind.UNAUTHENTICATED, paErrorKindOf("Please pair first: 未配对"))
    }

    @Test
    fun `连不上词表全命中`() {
        assertEquals(PaErrorKind.CONNECTIVITY, paErrorKindOf("DEADLINE_EXCEEDED: connect timed out"))
        assertEquals(PaErrorKind.CONNECTIVITY, paErrorKindOf("java.net.SocketTimeoutException: Read timed out"))
        assertEquals(PaErrorKind.CONNECTIVITY, paErrorKindOf("UNAVAILABLE: io exception"))
        assertEquals(PaErrorKind.CONNECTIVITY, paErrorKindOf("Connection refused"))
        assertEquals(PaErrorKind.CONNECTIVITY, paErrorKindOf("Failed to connect to /10.0.2.2:7246"))
        assertEquals(PaErrorKind.CONNECTIVITY, paErrorKindOf("列表加载失败：连接失败"))
        assertEquals(PaErrorKind.CONNECTIVITY, paErrorKindOf("网络不可用"))
        assertEquals(PaErrorKind.CONNECTIVITY, paErrorKindOf("通道已断开"))
        assertEquals(PaErrorKind.CONNECTIVITY, paErrorKindOf("Unable to resolve host \"pc\""))
        assertEquals(PaErrorKind.CONNECTIVITY, paErrorKindOf("操作超时"))
    }

    @Test
    fun `其余错误透传为DAEMON档`() {
        assertEquals(PaErrorKind.DAEMON, paErrorKindOf("INVALID_ARGUMENT: branch not found"))
        assertEquals(PaErrorKind.DAEMON, paErrorKindOf("会话正在运行,请先停止任务再删除"))
        // PERMISSION_DENIED 刻意不入未配对表:文件/Git 场景多为电脑端文件系统权限
        assertEquals(PaErrorKind.DAEMON, paErrorKindOf("PERMISSION_DENIED: Permission denied"))
        assertEquals(PaErrorKind.DAEMON, paErrorKindOf("删除失败:Permission denied"))
        assertEquals(PaErrorKind.DAEMON, paErrorKindOf("UNKNOWN: something else"))
    }

    @Test
    fun `混合报错未配对优先于连不上`() {
        // 同时命中两表:用户的下一步是配对,不是排查网络
        assertEquals(PaErrorKind.UNAUTHENTICATED, paErrorKindOf("UNAVAILABLE: token 无效"))
        assertEquals(
            PaErrorKind.UNAUTHENTICATED,
            paErrorKindOf("unauthenticated: 连接失败"),
        )
    }

    // ---------------------------------------------------------------- paErrorViewOf(翻译全表)

    @Test
    fun `未配对档_人话头加去配对动作加琥珀档`() {
        val raw = "UNAUTHENTICATED: token 无效"
        val view = paErrorViewOf(raw)
        assertEquals(PaErrorKind.UNAUTHENTICATED, view.kind)
        assertEquals("还没和电脑配对", view.headline)
        assertEquals(PaErrorAction.GO_PAIR, view.action)
        assertEquals("去配对", paErrorActionLabelOf(view.action))
        assertEquals(EditorialTone.ATTENTION, view.tone)
        // 展示层翻译不丢诊断信息:原文一字不动地带出
        assertEquals(raw, view.raw)
    }

    @Test
    fun `连不上档_人话头加查看排查动作加珊瑚档`() {
        val raw = "UNAVAILABLE: io exception"
        val view = paErrorViewOf(raw)
        assertEquals(PaErrorKind.CONNECTIVITY, view.kind)
        assertEquals("连不上电脑", view.headline)
        assertEquals(PaErrorAction.TROUBLESHOOT, view.action)
        assertEquals("查看排查", paErrorActionLabelOf(view.action))
        assertEquals(EditorialTone.DENIED, view.tone)
        assertEquals(raw, view.raw)
    }

    @Test
    fun `DAEMON档_保留原文只加人话头`() {
        val raw = "会话正在运行,请先停止任务再删除"
        val view = paErrorViewOf(raw)
        assertEquals(PaErrorKind.DAEMON, view.kind)
        assertEquals("电脑端说：$raw", view.headline)
        assertEquals(PaErrorAction.NONE, view.action)
        assertNull(paErrorActionLabelOf(view.action))
        assertEquals(EditorialTone.DENIED, view.tone)
        assertEquals(raw, view.raw)
    }

    @Test
    fun `DAEMON档_长原文缩略但raw保真`() {
        val long = "这是电脑端返回的一段很长很长的错误信息".repeat(10)
        val view = paErrorViewOf(long)
        assertTrue(view.headline.startsWith("电脑端说："))
        assertTrue(view.headline.endsWith("…"))
        // headline 缩略到上限附近,raw 永远是全文
        assertTrue(view.headline.length < long.length)
        assertEquals(long, view.raw)
    }

    @Test
    fun `DAEMON档_空原文有兜底人话`() {
        assertEquals("电脑端没有说明原因", paErrorViewOf("").headline)
        assertEquals("电脑端没有说明原因", paErrorViewOf("   \n  ").headline)
    }

    @Test
    fun `DAEMON档_headline压缩空白换行`() {
        val view = paErrorViewOf("第一行\n  第二行\t第三行")
        assertEquals("电脑端说：第一行 第二行 第三行", view.headline)
    }

    @Test
    fun `NONE动作无按钮文案`() {
        assertNull(paErrorActionLabelOf(PaErrorAction.NONE))
        assertEquals("去配对", paErrorActionLabelOf(PaErrorAction.GO_PAIR))
        assertEquals("查看排查", paErrorActionLabelOf(PaErrorAction.TROUBLESHOOT))
    }

    // ---------------------------------------------------------------- paIsSameErrorKind(同屏判重)

    @Test
    fun `同一根源的不同来源判重`() {
        // 盘点实锤:未配对时列表/电脑 codex/Pi/ZCode 四来源各报一条,同根只讲一遍
        assertTrue(
            paIsSameErrorKind(
                "会话列表加载失败：UNAUTHENTICATED: token 无效",
                "读取电脑上的 codex 会话失败:UNAUTHENTICATED: token 无效",
            ),
        )
        assertTrue(
            paIsSameErrorKind(
                "读取电脑上的 Pi 会话失败:UNAUTHENTICATED: token 无效",
                "读取电脑上的 ZCode 会话失败:token 无效,请先配对",
            ),
        )
        // 连通类同理
        assertTrue(paIsSameErrorKind("连接失败", "Connection refused"))
    }

    @Test
    fun `不同根源不判重`() {
        assertFalse(paIsSameErrorKind("UNAUTHENTICATED: token 无效", "Connection refused"))
        assertFalse(paIsSameErrorKind("INVALID_ARGUMENT: bad path", "列表加载失败：连接失败"))
    }

    @Test
    fun `空入参不判重_宁可重复展示也不吞错`() {
        assertFalse(paIsSameErrorKind(null, "连接失败"))
        assertFalse(paIsSameErrorKind("连接失败", null))
        assertFalse(paIsSameErrorKind(null, null))
        assertFalse(paIsSameErrorKind("", "连接失败"))
    }

    @Test
    fun `判重函数不改变各自headline`() {
        // 同 kind 的 DAEMON 档 headline 各带各自原文(判重只决定"摆不摆",不合并文本):
        // 两条都判为同根源,但 headline 互不相同,展示时仍各说各的原文。
        val a = paErrorViewOf("会话正在运行,请先停止任务再删除")
        val b = paErrorViewOf("分支不存在")
        assertTrue(paIsSameErrorKind("会话正在运行,请先停止任务再删除", "分支不存在"))
        assertNotEquals(a.headline, b.headline)
    }

    // ---------------------------------------------------------------- paEmptyStateMotifOf(空态判定)

    @Test
    fun `空态语境与母题一一对应`() {
        assertEquals(
            dev.pocketagent.app.ui.anim.PaDuotoneMotif.DISCOVER_RING,
            paEmptyStateMotifOf(PaEmptyContext.DISCOVER),
        )
        assertEquals(
            dev.pocketagent.app.ui.anim.PaDuotoneMotif.PAIR,
            paEmptyStateMotifOf(PaEmptyContext.PAIR),
        )
        assertEquals(
            dev.pocketagent.app.ui.anim.PaDuotoneMotif.APPROVAL,
            paEmptyStateMotifOf(PaEmptyContext.APPROVAL),
        )
    }

    @Test
    fun `无相符语境宁缺不配图形`() {
        assertNull(paEmptyStateMotifOf(PaEmptyContext.NONE))
    }
}
