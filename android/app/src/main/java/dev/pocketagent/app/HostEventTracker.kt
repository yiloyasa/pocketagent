package dev.pocketagent.app

import pocketagent.v1.HostEvent

/**
 * WatchEvents 的客户端断点状态机。
 *
 * 最新-only 政策(0.3.1 起,只维护最新版 daemon):daemon 恒发正数 sequence,
 * 不再有旧 daemon 的 sequence=0 兼容事件。非正数 sequence 视为协议违规,
 * 直接丢弃,绝不消费、绝不污染可持久化断点。
 * 正数事件只允许单调前进；重复或迟到事件在进入 UI 之前丢弃。
 */
internal class HostEventTracker(initialSequence: Long = 0L) {
    var lastSequence: Long = initialSequence.coerceAtLeast(0L)
        private set

    fun accept(event: HostEvent): Boolean {
        val sequence = event.sequence
        if (sequence <= lastSequence) return false
        lastSequence = sequence
        return true
    }
}

internal object HostEventRouter {
    fun turnFinishedThreadId(event: HostEvent): String? =
        if (event.hasTurnFinished() && event.turnFinished.threadId.isNotBlank()) {
            event.turnFinished.threadId
        } else {
            null
        }
}
