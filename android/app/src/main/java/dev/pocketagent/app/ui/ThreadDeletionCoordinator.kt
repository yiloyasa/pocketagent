package dev.pocketagent.app.ui

import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.component.ThreadPins
import dev.pocketagent.app.ui.component.ThreadSnapshot
import dev.pocketagent.app.ui.component.ThreadUsageStore
import dev.pocketagent.app.ui.component.TurnCheckpointStore
import dev.pocketagent.app.ui.component.UsageCache

/**
 * 删除会话时的**本地状态统一清理**入口。
 *
 * 【为什么需要一个协调器】一条会话在手机端有六处本地状态记着它:
 *  1. 列表快照 `threads.json`          — [ThreadSnapshot.removeThreadFromSnapshot]
 *  2. 消息快照 `md5(id).json`          — [ThreadSnapshot.removeMessages]
 *  3. 轮次 checkpoint(**含消息副本**)  — [TurnCheckpointStore.delete]
 *  4. 会话模型选择                     — [AgentConnection.removeThreadModel]
 *  5. 会话思考强度                     — [AgentConnection.removeThreadEffort]
 *  6. 用量快照 / 置顶键                — [ThreadUsageStore.remove] / [UsageCache.remove] / [ThreadPins.unpin]
 *
 * 它们散在五个 object 里,而删除流程此前**只调了第 1 个**;第 4-6 个要等
 * [ThreadSnapshot.pruneStalePrefs] 的延迟回收,而那个阈值是 **7 天**。后果是真实
 * 可感的:删掉的会话,断网兜底还能把整段对话读回来;模型选择在一周内仍指向一条
 * 不存在的会话,之后新建同 id 的会话会"继承"它。
 *
 * 协调器的职责是让删除成为一个**原子动作**:要么都清,要么都留 —— 而不是"到处
 * 清一点,漏了也没人发现"。
 *
 * 【为什么不复用 pruneStalePrefs】那是"按最后见到该会话的时间延迟回收",为的是不
 * 误删偶发拉不全的会话(列表只拉到活跃、没拉归档时,立刻删会抹掉用户的模型选择)。
 * 删除是**用户明确的意志**,不必等、也不该等。两者判据不同,不能合并。
 */
object ThreadDeletionCoordinator {

    /**
     * 清掉本次删除在**磁盘**上留下的痕迹(列表快照 + 消息快照 + checkpoint)。
     *
     * 与 [purgePreferences] 分开,是为了能在纯 JVM 单测里验证:磁盘类只需要一个目录,
     * 偏好类要 Android Context。
     */
    fun purgeDisk(threadId: String) {
        if (threadId.isBlank()) return
        ThreadSnapshot.removeThreadFromSnapshot(threadId)
        ThreadSnapshot.removeMessages(threadId)
        TurnCheckpointStore.delete(threadId)
    }

    /**
     * 清掉本次删除在**偏好**里留下的痕迹(模型 / 强度 / 用量 / 置顶)。
     *
     * @param pinKey 置顶键。它**不一定**等于本地 id —— [ThreadPins] 的键优先取 codex
     *   原生 id(见其文件头:这样导入前后是同一个键)。所以要由调用方按同一条规则
     *   给出,不能在协调器里另猜一遍,否则猜错就是"删了但置顶还在"。
     */
    fun purgePreferences(threadId: String, pinKey: String? = null) {
        if (threadId.isBlank()) return
        AgentConnection.removeThreadModel(threadId)
        AgentConnection.removeThreadEffort(threadId)
        ThreadUsageStore.remove(threadId)
        UsageCache.remove(threadId)
        if (!pinKey.isNullOrBlank()) ThreadPins.unpin(pinKey)
    }

    /**
     * 清掉本次删除在**内存态**里留下的痕迹(聊天会话状态 / 任务中心条目 / 子代理预览缓存)。
     *
     * 【为什么这几处也要一起删(用户明确要求)】它们不是"这条会话还在不在"的权威副本,
     * 但都**按 threadId 索引**:
     *   - 聊天会话状态([TurnSession])常驻 app 进程:不释放的话,断流恢复循环会继续
     *     为一个已删除的会话补事件,内存层"复活";同 id 重建也会命中旧 State;
     *   - 任务中心的"消息类"条目 id 就是 `msg:$threadId`,会话删了它还在,用户会看到
     *     一条永远点不开的记录;
     *   - 子代理预览按 threadId 缓存,**同一 id 的会话再建出来时会命中旧预览**,显示
     *     上一世的内容。
     * 都只在 **daemon 确认删除成功之后**才清(见 [purge] 的调用点):那时没有仍在跑的
     * 轮次,清掉条目不会丢掉"它还在跑"的可见性。
     */
    fun purgeInMemory(threadId: String) {
        if (threadId.isBlank()) return
        TurnSession.release(threadId)
        TaskCenter.removeByThread(threadId)
        SubAgentsState.forget(threadId)
    }

    /**
     * 全清。**daemon 确认删除成功后**调用一次。
     *
     * 【为什么必须在成功之后】daemon 可能拒绝删除(例如"会话正在运行,请先停止")。
     * 那时会话还在,本地状态必须先样留着 —— 提前清 = 用户看到失败提示,却发现自己
     * 的模型选择和离线对话都没了。
     */
    fun purge(threadId: String, pinKey: String? = null) {
        purgeDisk(threadId)
        purgePreferences(threadId, pinKey)
        purgeInMemory(threadId)
    }
}
