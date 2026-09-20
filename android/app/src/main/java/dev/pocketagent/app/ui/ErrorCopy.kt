package dev.pocketagent.app.ui

/**
 * 异常 → 用户可见文案(纯函数,JVM 单测钉死 ErrorCopyTest)。
 *
 * 【AUDIT §3.3 限期项 P2-3 的统一口径】UI 禁显「未知/错误」这类含糊字眼,
 * 遇到异常必须给人话或原生原文,三档取值:
 *  1. 异常自带非空 message → **透传原文**(原生信息,能透传就透传,README §7);
 *  2. message 为空/空白 → 透传**异常类名**(如"错误类型：IOException")——
 *     类名是可核查的真实信息,不是含糊其辞,也不是"未知错误";
 *  3. 连异常都没有(理论不发生,兜底) → "出了点问题，请重试"。
 *
 * 全部三档都不含「未知」字样,grep 自查口径见 ErrorCopyTest。
 */
internal fun humanizeError(e: Throwable?): String =
    e?.message?.takeIf { it.isNotBlank() }
        ?: e?.let { "错误类型：${it::class.java.simpleName}" }
        ?: "出了点问题，请重试"
