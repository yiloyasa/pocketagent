package dev.pocketagent.app.ui.chat

import pocketagent.v1.ContentBlock
import pocketagent.v1.TextBlock
import pocketagent.v1.ThreadMessage

/** daemon 回包消息的最小构造(message_id/turn_id 齐全,与生产 daemon 一致)。 */
internal fun msg(id: String, role: String = "user", text: String = "内容-$id"): ThreadMessage =
    ThreadMessage.newBuilder()
        .setMessageId(id)
        .setTurnId("turn-$id")
        .setRole(role)
        .addBlocks(ContentBlock.newBuilder().setText(TextBlock.newBuilder().setText(text)))
        .build()
