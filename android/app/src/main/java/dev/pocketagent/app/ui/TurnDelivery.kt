package dev.pocketagent.app.ui

/**
 * P1-4「运行中继续发消息」的投递状态机 —— 纯函数集中在这里,便于 JVM 单测钉住。
 *
 * 【帧序契约(daemon 侧,thread.go / turn_queue.go)】
 *   手机在轮次 running 时经同一条 StartTurn 流再发一条 TurnInput:
 *   - codex 插话成功   → Steered 回执
 *   - 入队(echo/排队) → Queued 回执(position 从 1 起;0 = 队列满拒收)
 *   - 当前轮终局        → Completed/Failed
 *   - 队首出队执行      → QueueFlushed → Started → 常规增量
 * 手机端据此把该条用户消息在「发送中 → 已插入 / 排队中(第N位) / 未送达」
 * 之间流转,QueueFlushed 后变回普通用户消息样式。
 */

/** 已发出,等 daemon 回执。 */
const val DELIVERY_PENDING = "pending"
/** 已插入运行中的轮次(codex turn/steer 成功)。 */
const val DELIVERY_STEERED = "steered"
/** 排队中,等当前轮次终局后自动执行。 */
const val DELIVERY_QUEUED = "queued"
/** 队列满被拒收 —— 这条消息 daemon 没有接收。 */
const val DELIVERY_OVERFLOW = "overflow"

/** 时间线小标签文案(空串 = 不显示标签,普通用户消息)。 */
internal fun deliveryLabel(state: String, queuePos: Int): String = when (state) {
    DELIVERY_PENDING -> "发送中…"
    DELIVERY_STEERED -> "已插入运行中任务"
    DELIVERY_QUEUED -> if (queuePos > 1) "排队中 · 第${queuePos}位" else "排队中 · 下一条"
    DELIVERY_OVERFLOW -> "队列已满,未送达"
    else -> ""
}

/** Queued 回执是否被 daemon 接受(position<=0 = 拒收,不得计入排队数)。 */
internal fun queuedAckAccepted(position: Int): Boolean = position > 0

/** 最早一条还挂着「发送中」的消息下标;没有 = -1。 */
internal fun firstPendingDeliveryIndex(items: List<ChatItem>): Int =
    items.indexOfFirst { it.deliverState == DELIVERY_PENDING }

/**
 * daemon 回执落到时间线:Steered/Queued 都指向「最早发出、还没回执」的那条
 * (同一条流上先发先回,顺序天然一致),原位更新该条消息的投递状态。
 */
internal fun applyDeliveryAckTo(items: List<ChatItem>, deliverState: String, queuePos: Int): List<ChatItem> {
    val idx = firstPendingDeliveryIndex(items)
    if (idx < 0) return items
    return items.toMutableList().also { it[idx] = it[idx].copy(deliverState = deliverState, queuePos = queuePos) }
}

/**
 * 收掉投递标签(变回普通用户消息样式)。没有要收的返回 null(调用方免做整表替换)。
 *
 * @param queuedOnly true  = 只收「排队中」(QueueFlushed:队首开始执行)
 *                   false = 收「发送中/已插入」(轮次终局:它们属于刚结束的这轮);
 *                           「排队中」与「未送达」保留 —— 排队的还没轮到,
 *                           未送达是必须继续可见的失败事实。
 */
internal fun clearDeliveryTags(items: List<ChatItem>, queuedOnly: Boolean): List<ChatItem>? {
    var changed = false
    val out = items.map { item ->
        val hit = if (queuedOnly) item.deliverState == DELIVERY_QUEUED
        else item.deliverState == DELIVERY_PENDING || item.deliverState == DELIVERY_STEERED
        if (hit) {
            changed = true
            item.copy(deliverState = "", queuePos = 0)
        } else {
            item
        }
    }
    return if (changed) out else null
}

/** 全部清空(流结束/恢复回填前的兜底,不留幽灵标签)。 */
internal fun clearAllDeliveryTags(items: List<ChatItem>): List<ChatItem>? {
    if (items.none { it.deliverState.isNotEmpty() }) return null
    return items.map { if (it.deliverState.isEmpty()) it else it.copy(deliverState = "", queuePos = 0) }
}
