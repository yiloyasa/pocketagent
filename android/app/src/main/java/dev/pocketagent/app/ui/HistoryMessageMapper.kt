package dev.pocketagent.app.ui

import pocketagent.v1.ThreadMessage
import pocketagent.v1.CodexEvent
import pocketagent.v1.Usage
import pocketagent.v1.UsageUpdate

private const val TOOL_PREVIEW_LIMIT = 400

fun historyMessageToChatItems(message: ThreadMessage): List<ChatItem> {
    val items = mutableListOf<ChatItem>()
    for (block in message.blocksList) {
        when {
            block.hasText() -> items += ChatItem(message.role, block.text.text)
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

fun usageUpdateToChatItem(update: UsageUpdate): ChatItem {
    val current = update.usage ?: Usage.getDefaultInstance()
    val total = update.totalUsage ?: Usage.getDefaultInstance()
    return ChatItem(
        role = "usage",
        text = buildString {
            append("本轮 Token：输入 ${current.inputTokens} · 输出 ${current.outputTokens}")
            append("；累计输入 ${total.inputTokens} · 输出 ${total.outputTokens}")
            if (current.costUsd > 0) {
                append(" · 费用 $")
                append("%.4f".format(java.util.Locale.US, current.costUsd))
            }
        },
    )
}
