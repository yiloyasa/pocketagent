package dev.pocketagent.app.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import dev.pocketagent.app.ui.collab.CollabMemberSpec
import dev.pocketagent.app.ui.collab.CollabSessionRepository

/**
 * 协同页的**派发前配置层**:选人、绑角色、防重复派发的硬闸。
 *
 * 【P1 协同闭环(Task 6)后它瘦成了什么】此前这里还拿着派发后的进度
 * (phases/threadIds/memberErrors/sessionId 一组散落的全局 mutable state),
 * 相位全靠 DispatchCollab 的受理回执猜,离开页面再回来什么都没有。现在:
 *  - 派发后的会话/成员/终局全部由 **CollabSessionRepository 的单一状态树**
 *    驱动(daemon 快照恢复 + 事件流跟随,daemon 是唯一事实源);
 *  - 本文件只保留"派发前的用户意图":选哪些 agent、各绑什么角色、上一条
 *    指令的回显、以及 dispatching 硬闸(与 UI 按钮禁用构成防重复提交的双保险)。
 * 真值从 daemon 来,这里不再保存任何"派发后"的事实 —— 受理(ack)更不会
 * 被显示成完成。
 */
object CollabState {
    /** 本次协同选中的 agent id 集合(多选)。 */
    var selected by mutableStateOf<Set<String>>(emptySet())
        private set
    /** agent id → 绑定角色 id(空 = 未配置)。 */
    var roleBinding by mutableStateOf<Map<String, String>>(emptyMap())
        private set
    /** 已发出的指令(最近一条;受理回执未达时协同页用它回显"已提交")。 */
    var lastInstruction by mutableStateOf<String?>(null)
        private set
    /**
     * 一单派发在飞(RPC 未返回)。防重复提交的**硬闸**:置位期间 dispatch /
     * 重复点击直接忽略 —— 双击、IME 回车、连点都只算一单。
     * UI 侧的按钮禁用(collabDispatchDecision)是第一道,这里是不依赖重组时序的第二道。
     */
    var dispatching by mutableStateOf(false)
        private set

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    fun toggle(id: String) { selected = toggleCollabSelection(selected, id) }
    // 【为什么叫 setSelection 而不是 setSelected】`var selected` 由委托生成了
    // JVM 上的 setSelected(Set) —— 这里再声明同名函数会撞 Platform declaration clash。
    // 改名而不是删除:批量设置(全选/清空)是要保留的能力,只是不能占用委托的签名。
    fun setSelection(ids: Set<String>) { selected = ids }

    /** 剪掉已失效的选中项(agent 掉线/被停用后,"已选 N 位"与能派发的对不上)。 */
    fun retainSelection(readyIds: Set<String>) { selected = pruneCollabSelection(selected, readyIds) }
    fun bindRole(agentId: String, roleId: String?) { roleBinding = bindCollabRole(roleBinding, agentId, roleId) }

    /**
     * 发出一条协同指令:**扇出给所有选中 agent,让它们并行执行同一条指令**。
     *
     * 【语义】受理不等于执行成功 —— "已受理/执行中/等待确认/已完成/未完成"
     * 全部由 daemon 事件驱动(CollabSessionRepository),这里只负责把指令和
     * 成员名单交给仓库,失败原因也由仓库如实呈现。
     */
    fun dispatch(instruction: String) {
        val cmd = instruction.trim()
        if (dispatching) return
        if (cmd.isEmpty() || selected.isEmpty()) return
        // 成员名单在这里定稿(选人 + 角色绑定是派发前的意图);定序按选中集合
        // 的稳定顺序,角色绑定逐个带上(空 = 未配置,daemon 用 agent 默认)。
        val members = selected.map { CollabMemberSpec(agentId = it, roleId = roleBinding[it]) }
        dispatching = true
        lastInstruction = cmd
        scope.launch {
            try {
                CollabSessionRepository.dispatch(cmd, members)
            } finally {
                dispatching = false
            }
        }
    }

    /** 退出/重开协同:清空本地的派发前配置;会话视图由仓库关闭事件流、保留快照。 */
    fun reset() {
        selected = emptySet(); roleBinding = emptyMap(); lastInstruction = null
        dispatching = false
        CollabSessionRepository.onPageHidden()
    }
}
