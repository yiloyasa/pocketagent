package dev.pocketagent.app.ui

import pocketagent.v1.CodexEvent
import pocketagent.v1.ThreadMessage
import pocketagent.v1.Usage
import pocketagent.v1.UsageUpdate

private const val TOOL_PREVIEW_LIMIT = 400

fun historyMessageToChatItems(message: ThreadMessage): List<ChatItem> {
    val items = mutableListOf<ChatItem>()
    for (block in message.blocksList) {
        when {
            block.hasText() -> items += ChatItem(
                message.role, block.text.text,
                sentAtMs = message.createdAtUnixMs,
            )
            block.hasImage() -> items += ChatItem(
                role = message.role,
                attachments = listOf(
                    Attachment(
                        name = "图片",
                        pcPath = "",
                        size = block.image.data.size(),
                        mediaType = block.image.mediaType,
                        previewData = block.image.data.toByteArray(),
                    ),
                ),
                sentAtMs = message.createdAtUnixMs,
            )
            block.hasThinking() -> items += ChatItem("thinking", block.thinking.text)
            block.hasToolCall() -> items += ChatItem(
                role = "tool",
                toolName = block.toolCall.name,
            )
            block.hasToolResult() -> items += ChatItem(
                role = "tool",
                toolName = "工具结果",
                text = block.toolResult.output.take(TOOL_PREVIEW_LIMIT),
            )
        }
    }
    if (items.isEmpty()) {
        items += ChatItem(message.role)
    }
    return items
}

fun historyToChatItems(messages: Iterable<ThreadMessage>): List<ChatItem> =
    messages.flatMap(::historyMessageToChatItems)

fun codexEventToChatItem(event: CodexEvent): ChatItem = ChatItem(
    role = "codex_event",
    text = event.message,
    codexEvent = event,
)

/**
 * 这条 codex 事件该不该进聊天时间线。
 *
 * 【背景:一段被"一刀切"砍掉的功能】
 * 早先为了不让重试、限流抖动这类信息刷屏,时间线里对所有 codex 事件一律
 * `-> Unit`(什么都不渲染)。但后来接入的一批事件**不是诊断噪音,而是用户
 * 必须知情甚至必须动手的事**:
 *
 *   审查者驳回了某个动作 —— 不显示,用户只知道"任务莫名其妙失败了";
 *   配置/沙箱/弃用警告     —— 不显示,用户下次遇到同样问题还是不知道为什么;
 *   回合文件改动 diff      —— 不显示,用户不知道 agent 到底动了什么;
 *   技能列表变化           —— 不显示,用户以为"我装的技能没生效";
 *   排队状态               —— 不显示,用户以为卡死了。
 *
 * 所以改成按"用户是否需要知情"分类,而不是按"是不是 codex 发来的"。
 *
 * 【分类依据必须是 daemon 的实际发射表,不是 proto 里的名字】
 * 这一点很容易搞错,而且错了就会把聊天页灌爆 —— daemon 侧
 * (`internal/agent/codex_events.go`) 把**一大批高频流式通知**都挂在兜底
 * `STATUS` 这个 kind 上:"命令输出"、"命令执行过程"、"Token 用量更新"、
 * "执行计划生成中"、"文件变更预览"、"MCP 工具进度"、"线程状态更新"。
 * 只看名字根本看不出来 STATUS 是流式兜底,于是任何"没列出的就默认显示"
 * 的写法都会让这些每秒好几条的增量涌进时间线。下面把它们逐条列进 false,
 * 正是为了不让"顺手 render 一下"把当初的刷屏问题带回来。
 *
 * 保持纯函数,好在单测里钉住 —— "要不要显示"这类规则最容易在重构中悄悄
 * 反转,而反转之后的表现是"功能静默消失",没有任何报错。
 */
fun shouldShowCodexEventInTimeline(event: CodexEvent): Boolean = when (event.kind) {
    // ---- 不进时间线:高频流式,或有更合适的归属面 ----
    CodexEvent.Kind.ERROR, // 归属错误提示条(st.errorMessage)
    CodexEvent.Kind.STATUS, // 兜底流式:命令输出/用量/计划增量/MCP 进度/线程状态
    CodexEvent.Kind.RETRY_STARTED, // 重试过程;重试彻底失败时会另发 ERROR
    CodexEvent.Kind.RETRY_STOPPED,
    CodexEvent.Kind.RATE_LIMIT, // 归属用量/限流面,不进聊天正文
    // javalite 生成的常量名是 proto 原文(前缀是 CODEX_EVENT_ 而非枚举名
    // KIND_,所以不会被剥掉),这里必须写全称。
    CodexEvent.Kind.CODEX_EVENT_UNSPECIFIED,
    -> false

    // ---- 进时间线:低频,且用户必须知情/可以动手 ----
    CodexEvent.Kind.GUARDIAN_REVIEW, // 可推翻驳回(卡片上有「仍然执行」)
    CodexEvent.Kind.TURN_DIFF, // 本轮到底改了哪些文件
    CodexEvent.Kind.DEPRECATION,
    CodexEvent.Kind.CONFIG_WARNING,
    CodexEvent.Kind.SANDBOX_WARNING,
    CodexEvent.Kind.HOOK,
    CodexEvent.Kind.SKILLS_CHANGED,
    CodexEvent.Kind.QUEUE,
    CodexEvent.Kind.MODEL_SAFETY,
    CodexEvent.Kind.WARNING, // codex 警告 / guardian 安全警告:解释"为什么被拦"
    CodexEvent.Kind.MODEL_REROUTED, // 解释"模型怎么换了"
    CodexEvent.Kind.CONTEXT_COMPACTED, // 解释"为什么它开始忘掉前面的内容"
    -> true

    // ---- 兜底:默认显示 ----
    // 能走到这里只有一种可能:proto 新增了 kind(而 kind 只由我们自己的
    // daemon 映射产生)。也就是说"我们加了新映射却忘了在这里分类" ——
    // 这种情况宁可多显示一条、让人看见并补上分类,也不要静默丢掉。
    // 注意这不构成刷屏风险:高频通知在 daemon 侧就已归到 STATUS,不会落到
    // 这个分支。
    else -> true
}

/**
 * 同一条事件是否应当在时间线上"原地更新"而不是新增一条卡片。
 *
 * 【两类必须原地更新的】1) 同一次审查:codex 会先发 started 再发 completed,
 * 两条都映射成审查事件,各插一张卡就会出现"同一件事两张卡";2) 回合 diff:
 * turn/diff/updated 一轮里可能发很多次,逐个追加会把时间线灌满 diff。
 *
 * 用 reviewId 做键、没有 reviewId 时退回 kind —— 后者正是 diff 与队列通知的情形。
 */
fun codexEventTimelineKey(event: CodexEvent): String = when {
    event.reviewId.isNotBlank() -> "review:" + event.reviewId
    // 这三类在一轮里会反复到达,只保留最新一张。
    event.kind == CodexEvent.Kind.TURN_DIFF ||
        event.kind == CodexEvent.Kind.QUEUE ||
        event.kind == CodexEvent.Kind.MODEL_SAFETY -> "kind:" + event.kind.name
    else -> ""
}

/**
 * 这条审查结论是否给得出"仍然执行"入口。
 *
 * 只有确实被驳回(或超时/中止)才有意义:已放行的没什么可推翻,
 * 还在评估中的点了也无效(daemon 侧同样会拒)。
 */
fun guardianOverridable(event: CodexEvent): Boolean {
    if (event.kind != CodexEvent.Kind.GUARDIAN_REVIEW) return false
    if (event.reviewId.isBlank()) return false
    return event.reviewStatus == "denied" ||
        event.reviewStatus == "timedOut" ||
        event.reviewStatus == "aborted"
}

/** 审查结论的一句话结论(手机端卡片标题用)。 */
fun guardianStatusLabel(status: String): String = when (status) {
    "inProgress" -> "审查中"
    "approved" -> "已放行"
    "denied" -> "已驳回"
    "timedOut" -> "审查超时"
    "aborted" -> "审查中止"
    "strictReviewRequired" -> "需要严格审查"
    else -> ""
}

/** 风险等级的中文短标签(空串表示 codex 没给)。 */
fun riskLevelLabel(level: String): String = when (level.lowercase()) {
    "low" -> "低风险"
    "medium" -> "中风险"
    "high" -> "高风险"
    "critical" -> "极高风险"
    else -> ""
}

fun usageUpdateToChatItem(update: UsageUpdate): ChatItem {
    val current = update.usage ?: Usage.getDefaultInstance()
    // total 与 TurnSession.projectedTotalUsage(R4-T4)同源收口:proto 消息字段未设置
    // 时 getter 返回默认实例而非 null,`?:` 挡不住 —— 必须 hasTotalUsage() 判"没设置"。
    // 历史回放 hermes 的 usage_update 从不设置 total_usage,这里未设置就置 null、
    // 不写"累计"段,而不是把"daemon 没报累计"渲染成"累计输入 0 · 输出 0"。
    val total = projectedTotalUsage(update)
    return ChatItem(
        role = "usage",
        text = buildString {
            append("本轮 Token：输入 ${current.inputTokens} · 输出 ${current.outputTokens}")
            if (total != null) {
                append("；累计输入 ${total.inputTokens} · 输出 ${total.outputTokens}")
            }
            if (current.costUsd > 0) {
                append(" · 费用 $")
                append("%.4f".format(java.util.Locale.US, current.costUsd))
            }
        },
    )
}
