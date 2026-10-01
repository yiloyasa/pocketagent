package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.TaskCenter

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

// ---------------------------------------------------------------- 失败态的"下一步"(R3-C)

/**
 * 失败文案审计(R3-C · 不确定时刻的诚实)的补全层:任务中心失败行的条目详情
 * (「下载失败:…」)已经回答了**为什么**,这里统一回答**现在能做什么** ——
 * 每个失败态给一条可执行的下一步(重试 / 恢复连接再试 / 回会话 / 移除),
 * 不带责备语气,不堆技术栈术语;断线时绝不引导用户去点一个此刻注定失败的按钮。
 *
 * 纯函数,单测钉死(TaskPartitionOfflineTest);文案同样遵守 AUDIT §3.3 禁词。
 *
 * @param kind 任务类别(消息类=找回结果;下载类=按重新下载的连接依赖分流)
 * @param hostOffline 当前与电脑断连
 * @param redownloadNeedsPc 重新下载是否需要电脑在线(null = 没有可再取的来源,
 *   见 component/TaskRetryGate.kt 的 [dev.pocketagent.app.ui.component.redownloadNeedsPc])
 * @param hasSourceThread 失败条目是否还关联着来源会话(决定"回会话"这条出路存不存在)
 */
internal fun failureNextStep(
    kind: TaskCenter.Kind,
    hostOffline: Boolean,
    redownloadNeedsPc: Boolean?,
    hasSourceThread: Boolean,
): String = when {
    kind == TaskCenter.Kind.MESSAGE ->
        if (hostOffline) "恢复连接后点重试,找回这次的结果"
        else "点右侧重试,找回这次的结果"

    // 下载类,没有可重放的来源(blob / 文本另存):出路是回会话或移除,不给假重试
    redownloadNeedsPc == null ->
        if (hasSourceThread) "内容在会话里还在;可回会话重新保存,或移除这条记录"
        else "这条记录没有可再取的文件,可移除"

    // 下载类,要连电脑:断线时先说清"现在取不了",不让人白点
    redownloadNeedsPc == true && hostOffline -> "电脑连不上时取不了;恢复连接后点重新下载"
    redownloadNeedsPc == true -> "点右侧重新下载,从电脑再取一次"

    // 下载类,只靠手机有网(URL 来源):与电脑连接无关,断连也照常可重试
    else -> "点右侧重新下载,再取一次"
}

// ---------------------------------------------------------------- 新功能面的失败文案(R1 深接残项收口)

/**
 * 终止轮次的 CancelTurn RPC 时限:超时就按"电脑没有回应"口径给文案并停下,
 * 不让取消协程在断线场景无限挂起(会话页的终止键也就没了下一步)。
 */
internal const val CANCEL_TURN_TIMEOUT_MS: Long = 15_000L

/**
 * 终止轮次的请求没有(及时)送达电脑时的横幅文案(纯函数,单测钉死)。
 *
 * 【为什么分两档】RPC 直接报异常 = 大概率没送达,任务多半还在跑;等满时限
 * 没回应 = 终止可能已生效也可能没生效,不能替电脑下结论。两档都遵守本文件
 * 顶部的 ErrorCopy 纪律:不用「失败/未知/错误」裸词开头收尾、不责备、给
 * **与界面上真实存在的按钮对齐**的下一步 —— 顶栏/发送键上的「终止」还在
 * (st.running 未清),提示条右侧的「重试」会重新接上并找回已产出的内容。
 * 异常原文按 [humanizeError] 三档透传,不吞、不含糊。
 *
 * @param timedOut true = 等满 [CANCEL_TURN_TIMEOUT_MS] 电脑没有回应;
 *                 false = RPC 直接报了异常
 * @param e        原始异常(透传原文用)
 */
internal fun cancelTurnFailureCopy(timedOut: Boolean, e: Throwable?): String {
    val detail = humanizeError(e)
    return if (timedOut) {
        "终止指令电脑暂时没有回应，可能已生效也可能还在运行；可再按一次「终止」，或点「重试」找回已产出的内容（$detail）"
    } else {
        "终止指令没有送到电脑，任务可能仍在运行；可再按一次「终止」，或点「重试」找回已产出的内容（$detail）"
    }
}

/**
 * 审批应答(允许 / 仅本会话允许 / 拒绝 —— 三选项同一条 answerApproval 链)
 * 没有送到电脑时,弹窗「暂无法处理」区的原因文案(纯函数,单测钉死)。
 *
 * 人话陈述事实 + 下一步:弹窗里这个原因的**正下方就是「重试」**,文案与
 * 按钮动作一致(点重试回到待决定,再交一次);异常原文按 [humanizeError]
 * 透传。禁「未知/错误」裸词,不责备任何一方。
 */
internal fun approvalAnswerFailureCopy(e: Throwable?): String =
    "这个决定没有送到电脑；恢复连接后点「重试」再交一次（${humanizeError(e)}）"

/**
 * 「已完成」的下载条目文件丢失时的行内说明(纯函数):先陈述事实(文件不在了),
 * 再给与 [failureNextStep] 同一套口径的出路 —— 手机上的文件可能被用户或清理软件
 * 移除,这不是任何人的错,文案里也不指责任何人。断线时需要电脑的重取不给入口,
 * 文案与行尾动作保持一致(引导恢复连接后再来,而不是指一个此刻不存在的按钮)。
 */
internal fun missingFileNextStep(
    redownloadNeedsPc: Boolean?,
    hasSourceThread: Boolean,
    hostOffline: Boolean = false,
): String = when {
    redownloadNeedsPc == true && hostOffline -> "文件已不在手机上;恢复连接后可重新下载,或先移除这条记录"
    redownloadNeedsPc == true -> "文件已不在手机上;可点重新下载再取一次(需要电脑在线)"
    redownloadNeedsPc == null ->
        if (hasSourceThread) "文件已不在手机上;可回会话重新保存,或移除这条记录"
        else "文件已不在手机上;这条记录可移除"

    else -> "文件已不在手机上;可点重新下载再取一次"
}
