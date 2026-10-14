package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.TaskCenter

internal fun task(
    id: String,
    kind: TaskCenter.Kind,
    status: TaskCenter.Status,
): TaskCenter.Task = TaskCenter.Task(
    id = id, threadId = "thr-$id", kind = kind, title = "任务$id", status = status,
)
