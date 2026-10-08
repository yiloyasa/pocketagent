package dev.pocketagent.app.ui

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.withStyle
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaHacker

/**
 * 代码着色(2026-10-09 UI 审美精修波 · D1):Markdown 代码块的轻量 token 着色器
 * + 工具卡/回合 diff 的行级着色 —— **纯 Kotlin、零第三方依赖**(依赖极简铁律)。
 *
 * 【为什么是"轻量"】目标是"看得出结构"而不是"IDE 级高亮":keyword / string /
 * comment / number 四类足够让代码块从单色大段灰里立出骨架。正则级近似会认错
 * 少数边界(嵌套注释、C 预处理器),着色是装饰不是语义,认错的代价只是颜色,
 * 不值得为它引入 tree-sitter。
 *
 * 【拼接守恒】[tokenizeCode]/[codeBoxSpans] 输出片段按序拼接恒等于原文 ——
 * 渲染层用全文 AnnotatedString 叠 SpanStyle,一个字符都不能丢(有单测钉死)。
 *
 * 【JVM 可测】本文件除两个"取当前盘语义色"的辅助函数外全部是纯函数,token
 * 分类/diff 分类/JSON 键识别都在单测里钉死(CodeSyntaxTest)。
 */

// ---------------------------------------------------------------- 代码块语法着色

/** 语言族:围栏语言标注 → 同构的着色规则集;未知语言落 PLAIN(退单色)。 */
enum class CodeSyntaxFamily { GO, KOTLIN, JAVA, JS_TS, JSON, BASH, PYTHON, PLAIN }

/** 语言标注 → 语言族(纯函数,大小写/别名归一)。 */
fun codeSyntaxFamily(lang: String): CodeSyntaxFamily {
    val l = lang.trim().lowercase()
    return when (l.substringBefore('-')) {
        "go", "golang" -> CodeSyntaxFamily.GO
        "kotlin", "kt", "kts" -> CodeSyntaxFamily.KOTLIN
        "java" -> CodeSyntaxFamily.JAVA
        "js", "javascript", "jsx", "mjs", "cjs", "ts", "typescript", "tsx" -> CodeSyntaxFamily.JS_TS
        "json", "jsonc", "json5" -> CodeSyntaxFamily.JSON
        "bash", "sh", "shell", "zsh", "shellscript", "console" -> CodeSyntaxFamily.BASH
        "python", "py" -> CodeSyntaxFamily.PYTHON
        else -> CodeSyntaxFamily.PLAIN
    }
}

/** token 四类 + 普通文本。 */
enum class CodeTokenKind { PLAIN, KEYWORD, STRING, COMMENT, NUMBER }

/** 一个着色片段;拼接守恒(见文件头)。 */
data class CodeToken(val text: String, val kind: CodeTokenKind)

/** 各语言族的关键词表(收敛到"读结构必需"的最小集,不做全量 BNF)。 */
private fun codeKeywords(family: CodeSyntaxFamily): Set<String> = when (family) {
    CodeSyntaxFamily.GO -> setOf(
        "func", "package", "import", "var", "const", "type", "struct", "interface", "map",
        "chan", "go", "defer", "if", "else", "for", "range", "return", "switch", "case",
        "default", "break", "continue", "select", "fallthrough", "goto", "nil", "true", "false",
    )
    CodeSyntaxFamily.KOTLIN -> setOf(
        "fun", "val", "var", "class", "object", "interface", "data", "sealed", "enum",
        "companion", "package", "import", "return", "if", "else", "when", "for", "while",
        "do", "in", "is", "as", "null", "true", "false", "this", "super", "suspend",
        "override", "private", "public", "internal", "protected", "init", "try", "catch",
        "finally", "throw", "typealias", "operator", "inline", "const", "abstract", "open",
        "by", "break", "continue",
    )
    CodeSyntaxFamily.JAVA -> setOf(
        "public", "private", "protected", "class", "interface", "enum", "extends",
        "implements", "static", "final", "void", "new", "return", "if", "else", "for",
        "while", "do", "switch", "case", "default", "break", "continue", "try", "catch",
        "finally", "throw", "throws", "import", "package", "this", "super", "null", "true",
        "false", "abstract", "synchronized", "volatile", "transient", "instanceof",
        "native", "record", "var",
    )
    CodeSyntaxFamily.JS_TS -> setOf(
        "function", "const", "let", "var", "class", "extends", "interface", "type", "enum",
        "return", "if", "else", "for", "while", "do", "switch", "case", "default", "break",
        "continue", "new", "delete", "typeof", "instanceof", "in", "of", "import", "export",
        "from", "as", "async", "await", "yield", "try", "catch", "finally", "throw", "this",
        "super", "null", "undefined", "true", "false", "static", "readonly", "declare",
        "namespace", "void", "any", "string", "number", "boolean", "never", "unknown",
    )
    CodeSyntaxFamily.JSON -> setOf("true", "false", "null")
    CodeSyntaxFamily.BASH -> setOf(
        "if", "then", "else", "elif", "fi", "for", "while", "do", "done", "case", "esac",
        "in", "function", "return", "export", "local", "readonly", "source", "exit",
        "break", "continue", "echo", "cd", "set", "unset",
    )
    CodeSyntaxFamily.PYTHON -> setOf(
        "def", "class", "return", "if", "elif", "else", "for", "while", "break", "continue",
        "pass", "import", "from", "as", "with", "try", "except", "finally", "raise",
        "lambda", "yield", "global", "nonlocal", "del", "assert", "in", "is", "not",
        "and", "or", "None", "True", "False", "async", "await", "match", "case",
    )
    CodeSyntaxFamily.PLAIN -> emptySet()
}

/**
 * 逐字符扫描的轻量着色器:keyword / string / comment / number 四类,其余 PLAIN。
 *
 * 【跨行状态】只认两种:C 系块注释(斜杠星到星斜杠)与 Python 三引号串
 * (三个双引号);单/双引号串不跨行(未闭合止于行尾)—— 真实语言里合法字符串本就不换行,
 * 止于行尾保证一行漏配不会把后面整段代码染成串色。
 *
 * 【拼接守恒】所有分支 substring 首尾相接覆盖 [code] 全文,不丢不重。
 */
fun tokenizeCode(code: String, lang: String): List<CodeToken> {
    val family = codeSyntaxFamily(lang)
    if (family == CodeSyntaxFamily.PLAIN || code.isEmpty()) {
        return listOf(CodeToken(code, CodeTokenKind.PLAIN))
    }
    val keywords = codeKeywords(family)
    val hashLine = family == CodeSyntaxFamily.BASH || family == CodeSyntaxFamily.PYTHON
    val slashLine = family == CodeSyntaxFamily.GO || family == CodeSyntaxFamily.KOTLIN ||
        family == CodeSyntaxFamily.JAVA || family == CodeSyntaxFamily.JS_TS
    val backquote = family == CodeSyntaxFamily.JS_TS || family == CodeSyntaxFamily.BASH
    val tokens = mutableListOf<CodeToken>()
    val plain = StringBuilder()
    var inBlockComment = false
    var i = 0

    fun flushPlain() {
        if (plain.isNotEmpty()) {
            tokens += CodeToken(plain.toString(), CodeTokenKind.PLAIN)
            plain.setLength(0)
        }
    }

    fun consumeUntil(marker: String, kind: CodeTokenKind) {
        val close = code.indexOf(marker, i + marker.length)
        if (close < 0) {
            tokens += CodeToken(code.substring(i), kind)
            i = code.length
        } else {
            tokens += CodeToken(code.substring(i, close + marker.length), kind)
            i = close + marker.length
        }
    }

    while (i < code.length) {
        val c = code[i]
        val next = if (i + 1 < code.length) code[i + 1] else ' '
        when {
            // 块注释承接(上一行开注释这一行还在里面)
            inBlockComment -> {
                flushPlain()
                consumeUntil("*/", CodeTokenKind.COMMENT)
                inBlockComment = false
            }
            // 行注释:# 系(bash/python)与 // 系(C 系;JSON 无注释不参与)
            (c == '#' && hashLine) || (c == '/' && next == '/' && slashLine) -> {
                flushPlain()
                val nl = code.indexOf('\n', i)
                val end = if (nl < 0) code.length else nl
                tokens += CodeToken(code.substring(i, end), CodeTokenKind.COMMENT)
                i = end // 换行留给 PLAIN,行边界不吞
            }
            // 块注释开口
            c == '/' && next == '*' && slashLine -> {
                flushPlain()
                inBlockComment = true
            }
            // Python 三引号串/文档串(跨行)
            family == CodeSyntaxFamily.PYTHON && c == '"' &&
                code.startsWith("\"\"\"", i) -> {
                flushPlain()
                consumeUntil("\"\"\"", CodeTokenKind.STRING)
            }
            // 字符串:单/双引号 +(js 模板串/bash 反引号)。转义跳过,未闭合止于行尾
            c == '"' || c == '\'' || (c == '`' && backquote) -> {
                flushPlain()
                var j = i + 1
                while (j < code.length) {
                    val ch = code[j]
                    if (ch == '\\') {
                        j += 2
                        continue
                    }
                    if (ch == c) {
                        j++
                        break
                    }
                    if (ch == '\n') break
                    j++
                }
                tokens += CodeToken(code.substring(i, j.coerceAtMost(code.length)), CodeTokenKind.STRING)
                i = j.coerceAtMost(code.length)
            }
            // 数字:十进制/十六进制/下划线分隔/类型后缀(0xFF、1_000、1.5f)
            c.isDigit() -> {
                flushPlain()
                var j = i + 1
                while (j < code.length) {
                    val ch = code[j]
                    if (ch.isLetterOrDigit() || ch == '_') {
                        j++
                    } else if (ch == '.' && j + 1 < code.length && code[j + 1].isDigit()) {
                        j++
                    } else {
                        break
                    }
                }
                tokens += CodeToken(code.substring(i, j), CodeTokenKind.NUMBER)
                i = j
            }
            // 标识符:词边界判定后查关键词表
            c.isLetter() || c == '_' || c == '$' -> {
                var j = i + 1
                while (j < code.length) {
                    val ch = code[j]
                    if (ch.isLetterOrDigit() || ch == '_' || ch == '$') j++ else break
                }
                val word = code.substring(i, j)
                if (word in keywords) {
                    flushPlain()
                    tokens += CodeToken(word, CodeTokenKind.KEYWORD)
                } else {
                    plain.append(word)
                }
                i = j
            }
            else -> {
                plain.append(c)
                i++
            }
        }
    }
    flushPlain()
    return tokens
}

/** 代码块四类 token 的语义色(组合期读当前盘,三盘换肤即时生效)。 */
data class CodeSyntaxColors(
    val keyword: Color,
    val string: Color,
    val comment: Color,
    val number: Color,
)

/**
 * token → PaEditorial 语义槽位的取色口径(禁新色值,三盘同构):
 *  - keyword:浅/深盘 Primary(蓝,与行内代码同槽);黑客盘换 **Mcp 紫** —— 黑客
 *    盘的纪律是"电光青只给激活态/当前值,语法紫做点缀",关键词是中等面积,
 *    用降饱和的紫而不是刺眼的青(黑客盘降饱和红线);
 *  - string:AttentionText 琥珀(黑客盘 RunningText 薄荷);
 *  - comment:Muted(黑客盘 Subtle,哑到"可略读"正是注释的语义);
 *  - number:RunningText(黑客盘 AccentText 青,数字面积小,允许强调色点缀)。
 */
fun syntaxColorsForCurrentPalette(): CodeSyntaxColors = CodeSyntaxColors(
    keyword = if (PaHacker.isHacker) PaEditorial.Mcp else PaEditorial.Primary,
    string = if (PaHacker.isHacker) PaEditorial.RunningText else PaEditorial.AttentionText,
    comment = if (PaHacker.isHacker) PaEditorial.Subtle else PaEditorial.Muted,
    number = if (PaHacker.isHacker) PaEditorial.AccentText else PaEditorial.RunningText,
)

/** token 流 → AnnotatedString(PLAIN 不加样式,继承 Text 本色;拼接守恒)。 */
fun syntaxTokensToAnnotated(tokens: List<CodeToken>, colors: CodeSyntaxColors): AnnotatedString =
    buildAnnotatedString {
        for (t in tokens) {
            when (t.kind) {
                CodeTokenKind.KEYWORD -> withStyle(SpanStyle(color = colors.keyword)) { append(t.text) }
                CodeTokenKind.STRING -> withStyle(SpanStyle(color = colors.string)) { append(t.text) }
                CodeTokenKind.COMMENT -> withStyle(SpanStyle(color = colors.comment)) { append(t.text) }
                CodeTokenKind.NUMBER -> withStyle(SpanStyle(color = colors.number)) { append(t.text) }
                CodeTokenKind.PLAIN -> append(t.text)
            }
        }
    }

// ---------------------------------------------------------------- 代码箱着色(diff / JSON)

/** 工具卡参数/输出代码箱与回合 diff 的行级分类。 */
enum class CodeBoxKind { PLAIN, DIFF, JSON }

/**
 * diff 启用的最低非空行数:两三行普通输出碰巧以 +/- 开头(如 "++ 完成"两行)
 * 不构成 diff 语境;统一 diff 的 hunk 动辄十几行,3 行下限把误报挡在门外。
 */
internal const val CODE_BOX_DIFF_MIN_LINES = 3

/**
 * 文本 → 代码箱类型(diff 优先于 JSON,再退纯文本):
 *  - DIFF:以 +/-/@@ 开头的行占非空行比例 > 60%(任务口径);
 *  - JSON:trim 后 {…} / […] 配对(工具入参是 daemon 校验过的合法 JSON);
 *  - PLAIN:其余,保持单色。
 */
fun classifyCodeBox(text: String): CodeBoxKind {
    if (text.isBlank()) return CodeBoxKind.PLAIN
    val nonBlank = text.lines().filter { it.isNotBlank() }
    if (nonBlank.size >= CODE_BOX_DIFF_MIN_LINES) {
        val marked = nonBlank.count {
            it.startsWith("+") || it.startsWith("-") || it.startsWith("@@")
        }
        if (marked.toFloat() / nonBlank.size > 0.6f) return CodeBoxKind.DIFF
    }
    val t = text.trim()
    if ((t.startsWith("{") && t.endsWith("}")) || (t.startsWith("[") && t.endsWith("]"))) {
        return CodeBoxKind.JSON
    }
    return CodeBoxKind.PLAIN
}

/** 代码箱片段类型:diff 三态 + JSON 键 + 普通行。 */
enum class CodeBoxSpanKind { PLAIN, DIFF_ADD, DIFF_DEL, DIFF_HUNK, JSON_KEY }

/** 一个代码箱片段;拼接守恒。 */
data class CodeBoxSpan(val text: String, val kind: CodeBoxSpanKind)

/** JSON 行首键:`缩进 "key" :`(引号内容忍转义);值保持代码箱本色。 */
private val jsonKeyPattern = Regex("""^\s*"([^"\\]|\\.)*"\s*:""")

/**
 * 代码箱文本 → 行级着色片段(相邻同类合并,控制 SpanStyle 数量)。
 * PLAIN 直接整段返回(绝大多数输出,零开销)。
 */
fun codeBoxSpans(text: String, kind: CodeBoxKind): List<CodeBoxSpan> {
    if (kind == CodeBoxKind.PLAIN || text.isEmpty()) {
        return listOf(CodeBoxSpan(text, CodeBoxSpanKind.PLAIN))
    }
    val out = mutableListOf<CodeBoxSpan>()
    fun append(s: String, k: CodeBoxSpanKind) {
        if (s.isEmpty()) return
        if (out.isNotEmpty() && out.last().kind == k) {
            out[out.lastIndex] = CodeBoxSpan(out.last().text + s, k)
        } else {
            out += CodeBoxSpan(s, k)
        }
    }
    val lines = text.split('\n')
    lines.forEachIndexed { idx, line ->
        val sep = if (idx == lines.lastIndex) "" else "\n"
        when (kind) {
            CodeBoxKind.DIFF -> when {
                line.startsWith("+") -> append(line + sep, CodeBoxSpanKind.DIFF_ADD)
                line.startsWith("-") -> append(line + sep, CodeBoxSpanKind.DIFF_DEL)
                line.startsWith("@@") -> append(line + sep, CodeBoxSpanKind.DIFF_HUNK)
                else -> append(line + sep, CodeBoxSpanKind.PLAIN)
            }
            CodeBoxKind.JSON -> {
                val m = jsonKeyPattern.find(line)
                if (m != null) {
                    append(m.value, CodeBoxSpanKind.JSON_KEY)
                    append(line.substring(m.value.length) + sep, CodeBoxSpanKind.PLAIN)
                } else {
                    append(line + sep, CodeBoxSpanKind.PLAIN)
                }
            }
            CodeBoxKind.PLAIN -> append(line + sep, CodeBoxSpanKind.PLAIN)
        }
    }
    return out
}

/** 代码箱语义色:plain 由调用方传(浅/深盘 CodeText、黑客盘 Ink/Muted 各自口径)。 */
data class CodeBoxColors(
    val plain: Color,
    val add: Color,
    val del: Color,
    val hunk: Color,
    val key: Color,
)

/**
 * 代码箱取色:diff 行 + → Running 系绿、- → Denied 系红、@@ → 哑色
 * (OnGraphiteSubtle,石墨代码箱上的三级档);JSON 键 → Mcp 紫(石墨深底上
 * 对比 ≥5:1,三盘同槽)。黑客盘用各自的 *Text 档(Canvas 底),非黑客盘用
 * accent 档(GraphiteCode 深底 —— 浅盘的 *Text 档是给浅底加深用的,在深底
 * 上反而读不清)。
 */
fun codeBoxColors(plain: Color): CodeBoxColors = CodeBoxColors(
    plain = plain,
    add = if (PaHacker.isHacker) PaEditorial.RunningText else PaEditorial.Running,
    del = if (PaHacker.isHacker) PaEditorial.DeniedText else PaEditorial.Denied,
    hunk = PaEditorial.OnGraphiteSubtle,
    key = PaEditorial.Mcp,
)

/** 代码箱 → AnnotatedString(diff 检测优先,由调用方或默认 [classifyCodeBox] 判型)。 */
fun codeBoxAnnotated(
    text: String,
    colors: CodeBoxColors,
    kind: CodeBoxKind = classifyCodeBox(text),
): AnnotatedString {
    val spans = codeBoxSpans(text, kind)
    return buildAnnotatedString {
        for (s in spans) {
            when (s.kind) {
                CodeBoxSpanKind.PLAIN -> append(s.text)
                CodeBoxSpanKind.DIFF_ADD -> withStyle(SpanStyle(color = colors.add)) { append(s.text) }
                CodeBoxSpanKind.DIFF_DEL -> withStyle(SpanStyle(color = colors.del)) { append(s.text) }
                CodeBoxSpanKind.DIFF_HUNK -> withStyle(SpanStyle(color = colors.hunk)) { append(s.text) }
                CodeBoxSpanKind.JSON_KEY -> withStyle(SpanStyle(color = colors.key)) { append(s.text) }
            }
        }
    }
}
