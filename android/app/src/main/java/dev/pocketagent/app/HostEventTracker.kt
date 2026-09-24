package dev.pocketagent.app

import pocketagent.v1.HostEvent

/**
 * WatchEvents 的客户端断点状态机。
 *
 * sequence=0 是旧 daemon 的兼容事件：可以消费，但不能污染可持久化断点。
 * 正数事件只允许单调前进；重复或迟到事件在进入 UI 之前丢弃。
 */
internal class HostEventTracker(initialSequence: Long = 0L) {
    var lastSequence: Long = initialSequence.coerceAtLeast(0L)
        private set

    fun accept(event: HostEvent): Boolean {
        val sequence = event.sequence
        if (sequence <= 0L) return true
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
