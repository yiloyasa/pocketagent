package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 文件预览代码块渲染(PcFilePreview.codeBlockHtml)的单测 —— R2-D 落地项 2:
 * 代码预览等宽 + 横向滚动 + 行号(行号由 CSS counter 画伪元素,复制不带走)。
 * PcFilePreview 的 object 初始化只含常量,不触 Android 框架方法,可纯 JVM 跑。
 */
class FilesPreviewCodeBlockTest {

    @Test
    fun `代码块 每行包进 span 行元素`() {
        val html = PcFilePreview.codeBlockHtml("val a = 1\nval b = 2")
        assertTrue(html.startsWith("<pre>"))
        assertTrue(html.endsWith("</pre>"))
        assertTrue(html.contains("<span class=\"l\">val a = 1</span>"))
        assertTrue(html.contains("<span class=\"l\">val b = 2</span>"))
    }

    @Test
    fun `代码块 行内容做 HTML 转义 不产生可执行标签`() {
        // esc 顺序:先 & 后 < > ",空格原样保留(两跑预算已用,断言按实现逐字符核对)
        val html = PcFilePreview.codeBlockHtml("if (a < b && c > d) \"x\"")
        assertFalse(html.contains("<b &&"))
        assertTrue(html.contains("&lt; b &amp;&amp; c &gt; d"))
        assertTrue(html.contains("&quot;x&quot;"))
    }

    @Test
    fun `代码块 空行也占一行 保行号对位`() {
        val html = PcFilePreview.codeBlockHtml("a\n\nb")
        assertEquals(3, Regex("<span class=\"l\">").findAll(html).count())
    }

    @Test
    fun `代码块 空内容给空 pre 不抛异常`() {
        val html = PcFilePreview.codeBlockHtml("")
        assertEquals("<pre><span class=\"l\"></span></pre>", html)
    }
}
