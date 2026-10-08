package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * CodeSyntax 轻量着色器的纯函数钉子(2026-10-09 UI 精修波 D1 配套):
 *  1) 七语言族判定(围栏标注归一 + 别名 + 未知语言回退 PLAIN);
 *  2) 每个语言族至少 keyword / string / comment / number 各一例;
 *  3) 拼接守恒 —— token 片段按序拼接恒等于原文(渲染层全文叠 SpanStyle,
 *     丢一个字符就是渲染 bug,这条是文件头的红线承诺);
 *  4) 代码箱分类(diff / JSON / PLAIN)与行级片段(+/−/@@ 三态、JSON 键)。
 *
 * 全部只触纯 Kotlin(tokenize/classify/spans 不 import 任何 compose 类),
 * 取色槽位(syntaxColorsForCurrentPalette/codeBoxColors)读当前盘静态态,
 * 留给真机三盘自查,不进 JVM 用例。
 */
class CodeSyntaxTest {

    // ───────────────────── 1) 语言族判定 ─────────────────────

    @Test
    fun `七语言族判定与别名归一`() {
        assertEquals(CodeSyntaxFamily.GO, codeSyntaxFamily("go"))
        assertEquals(CodeSyntaxFamily.GO, codeSyntaxFamily("golang"))
        assertEquals(CodeSyntaxFamily.KOTLIN, codeSyntaxFamily("kotlin"))
        assertEquals(CodeSyntaxFamily.KOTLIN, codeSyntaxFamily("Kt"))
        assertEquals(CodeSyntaxFamily.KOTLIN, codeSyntaxFamily("kts"))
        assertEquals(CodeSyntaxFamily.JAVA, codeSyntaxFamily("java"))
        assertEquals(CodeSyntaxFamily.JS_TS, codeSyntaxFamily("js"))
        assertEquals(CodeSyntaxFamily.JS_TS, codeSyntaxFamily("typescript"))
        assertEquals(CodeSyntaxFamily.JS_TS, codeSyntaxFamily("TSX"))
        assertEquals(CodeSyntaxFamily.JSON, codeSyntaxFamily("json"))
        assertEquals(CodeSyntaxFamily.JSON, codeSyntaxFamily("jsonc"))
        assertEquals(CodeSyntaxFamily.BASH, codeSyntaxFamily("bash"))
        assertEquals(CodeSyntaxFamily.BASH, codeSyntaxFamily("sh"))
        assertEquals(CodeSyntaxFamily.BASH, codeSyntaxFamily("zsh"))
        assertEquals(CodeSyntaxFamily.PYTHON, codeSyntaxFamily("python"))
        assertEquals(CodeSyntaxFamily.PYTHON, codeSyntaxFamily("py"))
    }

    @Test
    fun `带元信息或方言后缀的语言标注取首段归一`() {
        // ```kotlin title=Main.kt 这类围栏标注:语言解析层已截掉元信息,这里兜底 -
        assertEquals(CodeSyntaxFamily.KOTLIN, codeSyntaxFamily("kotlin-script"))
        assertEquals(CodeSyntaxFamily.JS_TS, codeSyntaxFamily("typescript-react"))
    }

    @Test
    fun `未知或空语言回退PLAIN`() {
        assertEquals(CodeSyntaxFamily.PLAIN, codeSyntaxFamily(""))
        assertEquals(CodeSyntaxFamily.PLAIN, codeSyntaxFamily("rust"))
        assertEquals(CodeSyntaxFamily.PLAIN, codeSyntaxFamily("powershell"))
        assertEquals(CodeSyntaxFamily.PLAIN, codeSyntaxFamily("   "))
    }

    // ───────────────── 2) 各语言族 token 分类 ─────────────────

    /** 断言 [code] 里出现 [fragment] 且其 token 类为 [kind];返回 token 流供守恒复用。 */
    private fun assertTokenKind(code: String, lang: String, fragment: String, kind: CodeTokenKind): List<CodeToken> {
        val tokens = tokenizeCode(code, lang)
        val hit = tokens.filter { it.text == fragment }
        assertTrue(
            "lang=$lang 应产出 token「$fragment」=$kind,实际:${tokens.map { "${it.kind}:${it.text}" }}",
            hit.all { it.kind == kind } && hit.isNotEmpty(),
        )
        return tokens
    }

    @Test
    fun `GO族_四类token各一例`() {
        val src = "package main\nfunc run() { n := 42 } // note\n"
        val tokens = assertTokenKind(src, "go", "func", CodeTokenKind.KEYWORD)
        assertTokenKind(src, "go", "// note", CodeTokenKind.COMMENT)
        assertTokenKind(src, "go", "42", CodeTokenKind.NUMBER)
        // go 无字符串面量的用例行里没串,补一条
        val s = "msg := \"hi\"\n"
        assertTokenKind(s, "go", "\"hi\"", CodeTokenKind.STRING)
        assertEquals(src, tokens.joinToString("") { it.text })
    }

    @Test
    fun `KOTLIN族_四类token各一例`() {
        val src = "val x = \"hi\" // 注释\n"
        val tokens = assertTokenKind(src, "kotlin", "val", CodeTokenKind.KEYWORD)
        assertTokenKind(src, "kotlin", "\"hi\"", CodeTokenKind.STRING)
        assertTokenKind(src, "kotlin", "// 注释", CodeTokenKind.COMMENT)
        val withNum = "fun f() = 1\n"
        assertTokenKind(withNum, "kotlin", "fun", CodeTokenKind.KEYWORD)
        assertTokenKind(withNum, "kotlin", "1", CodeTokenKind.NUMBER)
        assertEquals(src, tokens.joinToString("") { it.text })
    }

    @Test
    fun `JAVA族_四类token各一例`() {
        val src = "public void run() { total = 3.14f; } // c\n"
        val tokens = assertTokenKind(src, "java", "public", CodeTokenKind.KEYWORD)
        assertTokenKind(src, "java", "void", CodeTokenKind.KEYWORD)
        assertTokenKind(src, "java", "3.14f", CodeTokenKind.NUMBER)
        assertTokenKind(src, "java", "// c", CodeTokenKind.COMMENT)
        val s = "String g = \"a\";\n"
        assertTokenKind(s, "java", "\"a\"", CodeTokenKind.STRING)
        assertEquals(src, tokens.joinToString("") { it.text })
    }

    @Test
    fun `JS_TS族_四类token各一例_含模板串与十六进制`() {
        val src = "const k = `tpl` + 0xFF; // done\n"
        val tokens = assertTokenKind(src, "ts", "const", CodeTokenKind.KEYWORD)
        assertTokenKind(src, "ts", "`tpl`", CodeTokenKind.STRING)
        assertTokenKind(src, "ts", "0xFF", CodeTokenKind.NUMBER)
        assertTokenKind(src, "ts", "// done", CodeTokenKind.COMMENT)
        val withKw = "return true\n"
        assertTokenKind(withKw, "ts", "return", CodeTokenKind.KEYWORD)
        assertTokenKind(withKw, "ts", "true", CodeTokenKind.KEYWORD)
        assertEquals(src, tokens.joinToString("") { it.text })
    }

    @Test
    fun `JSON族_关键字与串与数字_且不认注释`() {
        val src = "{\"a\": true, \"n\": 42, \"f\": null}"
        val tokens = assertTokenKind(src, "json", "true", CodeTokenKind.KEYWORD)
        assertTokenKind(src, "json", "null", CodeTokenKind.KEYWORD)
        assertTokenKind(src, "json", "\"a\"", CodeTokenKind.STRING)
        assertTokenKind(src, "json", "42", CodeTokenKind.NUMBER)
        // JSON 无注释语法:# 与 // 都必须保持普通文本(误染即测试红)
        val noComment = "{\"u\": \"a//b\"}"
        assertTrue(
            "JSON 里不应出现 COMMENT token",
            tokenizeCode(noComment, "json").none { it.kind == CodeTokenKind.COMMENT },
        )
        assertEquals(src, tokens.joinToString("") { it.text })
    }

    @Test
    fun `BASH族_四类token各一例_hash行注释`() {
        val src = "if [ -n \"${'$'}x\" ]; then\n  exit 0\nfi # guard\n"
        val tokens = assertTokenKind(src, "bash", "if", CodeTokenKind.KEYWORD)
        assertTokenKind(src, "bash", "exit", CodeTokenKind.KEYWORD)
        assertTokenKind(src, "bash", "# guard", CodeTokenKind.COMMENT)
        assertTokenKind(src, "bash", "0", CodeTokenKind.NUMBER)
        assertTokenKind(src, "bash", "\"${'$'}x\"", CodeTokenKind.STRING)
        assertEquals(src, tokens.joinToString("") { it.text })
    }

    @Test
    fun `PYTHON族_四类token各一例_含三引号跨行docstring`() {
        val src = "def f(n):\n    \"\"\"doc\nline2\"\"\"\n    return n + 1\n"
        val tokens = assertTokenKind(src, "python", "def", CodeTokenKind.KEYWORD)
        assertTokenKind(src, "python", "\"\"\"doc\nline2\"\"\"", CodeTokenKind.STRING)
        assertTokenKind(src, "python", "return", CodeTokenKind.KEYWORD)
        assertTokenKind(src, "python", "1", CodeTokenKind.NUMBER)
        assertEquals(src, tokens.joinToString("") { it.text })
        val withComment = "x = 1  # 说明\n"
        assertTokenKind(withComment, "python", "# 说明", CodeTokenKind.COMMENT)
    }

    // ─────────────── 3) 边界行为与拼接守恒 ───────────────

    @Test
    fun `未知语言整段退单色单token`() {
        val src = "func x() { # not go\n}"
        val tokens = tokenizeCode(src, "brainfuck")
        assertEquals(listOf(CodeToken(src, CodeTokenKind.PLAIN)), tokens)
        // 空语言同理(裸围栏)
        assertEquals(listOf(CodeToken(src, CodeTokenKind.PLAIN)), tokenizeCode(src, ""))
        // 空文本
        assertEquals(listOf(CodeToken("", CodeTokenKind.PLAIN)), tokenizeCode("", "kotlin"))
    }

    @Test
    fun `关键词只在词边界命中`() {
        // "functional" 不是 "func";让最长字母游标吃满整个词
        val tokens = tokenizeCode("functional()", "go")
        assertTrue(
            "functional 不应命中 func 关键词",
            tokens.none { it.text == "func" || it.kind == CodeTokenKind.KEYWORD },
        )
    }

    @Test
    fun `字符串内转义不提前闭合`() {
        val tokens = assertTokenKind("s = \"a\\\"b\" + 1", "kotlin", "\"a\\\"b\"", CodeTokenKind.STRING)
        assertEquals("s = \"a\\\"b\" + 1", tokens.joinToString("") { it.text })
    }

    @Test
    fun `未闭合单行串止于行尾不跨行污染`() {
        val src = "x = \"oops\ny = 2"
        val tokens = tokenizeCode(src, "python")
        assertTokenKind(src, "python", "\"oops", CodeTokenKind.STRING)
        // 第二行只有换行+普通文本落 PLAIN,数字照常是 NUMBER —— 串色没有越过行边界
        assertTrue(
            "第二行文本不应被串色污染",
            tokens.any { it.text == "\ny = " && it.kind == CodeTokenKind.PLAIN },
        )
        assertTrue(tokens.any { it.text == "2" && it.kind == CodeTokenKind.NUMBER })
        assertEquals(src, tokens.joinToString("") { it.text })
    }

    @Test
    fun `C系块注释跨行承接_未闭合吞到末尾`() {
        val src = "/* a\nb */ x"
        val tokens = assertTokenKind(src, "java", "/* a\nb */", CodeTokenKind.COMMENT)
        assertEquals(src, tokens.joinToString("") { it.text })
        val unclosed = "k = 1\n/* forever"
        val t2 = tokenizeCode(unclosed, "go")
        assertTrue(t2.last().kind == CodeTokenKind.COMMENT && t2.last().text == "/* forever")
        assertEquals(unclosed, t2.joinToString("") { it.text })
    }

    @Test
    fun `拼接守恒_混合高危样本全语言族`() {
        val samples = mapOf(
            "go" to "package m\n/* c */ s := \"a\\\"b\"\nn = 0x1F // t\n",
            "kotlin" to "val v = \"x\" /* m */ // y\nfun g() = 1_000\n",
            "java" to "record R(int n) {}\nString s = 'c'; // 1.5e3\n",
            "ts" to "type T = `a${'$'}{b}`;\nlet n = 1_000; // t\n",
            "json" to "{\"a\": [1, 2.5, true, null]}",
            "bash" to "if true; then\n  echo \"hi # not comment\"\nfi\n",
            "python" to "def f():\n    \"\"\"t\"\"\"\n    return f\"{1}\"\n",
        )
        samples.forEach { (lang, code) ->
            val tokens = tokenizeCode(code, lang)
            assertEquals(
                "lang=$lang 拼接守恒被破坏",
                code,
                tokens.joinToString("") { it.text },
            )
        }
    }

    // ───────────────── 4) 代码箱分类与行级片段 ─────────────────

    @Test
    fun `代码箱分类_diff需过行数下限与占比双门槛`() {
        // 统一 diff:hunk 头 + 增删行,占比过 60%
        assertEquals(
            CodeBoxKind.DIFF,
            classifyCodeBox("@@ -1,3 +1,4 @@\n+added\n-removed\n context"),
        )
        // 两行以 +/- 开头的普通输出不构成 diff(3 行下限挡误报)
        assertEquals(CodeBoxKind.PLAIN, classifyCodeBox("++ 完成\n-- 跳过"))
        // 3 行非空但只有 1 行带标记:占比 1/3 不过门槛
        assertEquals(CodeBoxKind.PLAIN, classifyCodeBox("+x\nplain\nplain"))
        // JSON 配对
        assertEquals(CodeBoxKind.JSON, classifyCodeBox("{\"k\": 1}"))
        assertEquals(CodeBoxKind.JSON, classifyCodeBox(" [1, 2, 3] "))
        // 其余纯文本
        assertEquals(CodeBoxKind.PLAIN, classifyCodeBox("hello world"))
        assertEquals(CodeBoxKind.PLAIN, classifyCodeBox("   "))
    }

    @Test
    fun `diff行分类_三态与相邻同类合并_拼接守恒`() {
        val src = "+a\n+ b\n-c\n@@ -1 +1 @@\n ctx"
        val spans = codeBoxSpans(src, CodeBoxKind.DIFF)
        assertEquals(
            listOf(
                CodeBoxSpan("+a\n+ b\n", CodeBoxSpanKind.DIFF_ADD), // 相邻同类合并
                CodeBoxSpan("-c\n", CodeBoxSpanKind.DIFF_DEL),
                CodeBoxSpan("@@ -1 +1 @@\n", CodeBoxSpanKind.DIFF_HUNK),
                CodeBoxSpan(" ctx", CodeBoxSpanKind.PLAIN), // 末行无换行分隔
            ),
            spans,
        )
        assertEquals(src, spans.joinToString("") { it.text })
    }

    @Test
    fun `JSON行首键着色_值保持本色_拼接守恒`() {
        // 键识别只认"行首缩进后的 "key":"(格式化 JSON 的常规形态);紧凑单行
        // {"a": 1} 的键不在行首,整行保持 PLAIN —— 这是口径不是缺陷。
        // 行首缩进随键片段同色(正则 ^\s* 并入匹配):空白无字形视觉零差,
        // 且只有这样期望值与实现才拼接守恒。
        val src = "{\n  \"name\": \"x\",\n  \"n\": 1\n}"
        val spans = codeBoxSpans(src, CodeBoxKind.JSON)
        assertEquals(
            listOf(
                CodeBoxSpan("{\n", CodeBoxSpanKind.PLAIN),
                CodeBoxSpan("  \"name\":", CodeBoxSpanKind.JSON_KEY),
                CodeBoxSpan(" \"x\",\n", CodeBoxSpanKind.PLAIN),
                CodeBoxSpan("  \"n\":", CodeBoxSpanKind.JSON_KEY),
                CodeBoxSpan(" 1\n}", CodeBoxSpanKind.PLAIN), // 末行 " 1" 与 "}" 同为 PLAIN,append 合并
            ),
            spans,
        )
        assertEquals(src, spans.joinToString("") { it.text })
    }

    @Test
    fun `PLAIN代码箱零开销直通_含多行`() {
        val src = "line1\nline2\n"
        assertEquals(
            listOf(CodeBoxSpan(src, CodeBoxSpanKind.PLAIN)),
            codeBoxSpans(src, CodeBoxKind.PLAIN),
        )
        // 空文本
        assertEquals(
            listOf(CodeBoxSpan("", CodeBoxSpanKind.PLAIN)),
            codeBoxSpans("", CodeBoxKind.JSON),
        )
    }

    @Test
    fun `diff行数下限常量语义被钉住`() {
        // 阈值是口径的一部分:下调会让"++ 完成"两行误染 diff
        assertEquals(3, CODE_BOX_DIFF_MIN_LINES)
    }
}
