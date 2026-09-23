package dev.pocketagent.app.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import dev.pocketagent.app.AgentConnection
import io.grpc.Status
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CollabMember
import pocketagent.v1.DispatchCollabRequest

/**
 * 协同会话的本地状态层 + 协同模式开关持久化。
 *
 * 【为什么是本地状态 + 诚实占位,不伪造 daemon 数据】
 * 一条指令"扇出给多个 agent 并行执行"的 daemon 侧能力由主代理并行实现,
 * 本文件不碰 proto、不假装已有结果。状态对象只持有**客户端真实可知**的相位:
 *  - ACK(已收到指令):手机确实把指令交出去了,是客户端事实;
 *  - RUNNING(执行中 · 等待回报):扇出已提交,结果须等 daemon 接口,如实标"等待";
 *  - 绝不写死"某 agent 回复了 XXX"这类假结果。
 * 真实进度由下方预留的 daemon 通道回填(见 dispatch 注释里的 RPC 清单)。
 */
object CollabState {
    /** 本次协同选中的 agent id 集合(多选)。 */
    var selected by mutableStateOf<Set<String>>(emptySet())
        private set
    /** agent id → 绑定角色 id(空 = 未配置)。 */
    var roleBinding by mutableStateOf<Map<String, String>>(emptyMap())
        private set
    /** agent id → 派发相位(指令发出后才有条目)。 */
    var phases by mutableStateOf<Map<String, CollabAgentPhase>>(emptyMap())
        private set
    /** 已发出的指令(最近一条用于回显;历史暂只保留单条,够演示并行扇出)。 */
    var lastInstruction by mutableStateOf<String?>(null)
        private set
    /** 本次协同的会话 id(daemon 下发,归组与事后回溯用;未派发时为空)。 */
    var sessionId by mutableStateOf<String?>(null)
        private set
    /** agent id → 该 agent 的子线程 id:扇出受理成功后才有,供后续按线程查状态。 */
    var threadIds by mutableStateOf<Map<String, String>>(emptyMap())
        private set
    /** agent id → 该成员自己的失败原因(人话原文)。一家失败不该挡住别家,所以按成员存。 */
    var memberErrors by mutableStateOf<Map<String, String>>(emptyMap())
        private set
    /** 整体派发失败(连 RPC 都没通)时的人话原因;成员级失败看 [memberErrors]。 */
    var dispatchError by mutableStateOf<String?>(null)
        private set

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    fun toggle(id: String) { selected = toggleCollabSelection(selected, id) }
    // 【为什么叫 setSelection 而不是 setSelected】`var selected` 由委托生成了
    // JVM 上的 setSelected(Set) —— 这里再声明同名函数会撞 Platform declaration clash。
    // 改名而不是删除:批量设置(全选/清空)是要保留的能力,只是不能占用委托的签名。
    fun setSelection(ids: Set<String>) { selected = ids }
    fun bindRole(agentId: String, roleId: String?) { roleBinding = bindCollabRole(roleBinding, agentId, roleId) }

    /**
     * 发出一条协同指令:**扇出给所有选中 agent,让它们并行执行同一条指令**。
     *
     * 【语义】不是"一个 agent 派子智能体",而是"多个对等 agent 各干各的" ——
     * daemon 给每个成员各开一个子线程同时跑,成员之间无父子关系。
     *
     * 【相位是真状态,不是动画】
     *  - ACK     = 手机上确实把指令交出去了(客户端事实);
     *  - RUNNING = daemon 已**受理**(子线程已建、轮次已起飞),结果还得等;
     *  - ERROR   = 这家受理失败,daemon 给了人话原因;
     *  - DONE    = 子线程跑完(由 [refreshMemberStates] 按 thread_id 回填)。
     * 受理(ack)不等于执行成功 —— 把受理显示成"已完成"就是伪造结果。
     *
     * 【为什么要按位置对应 items】返回项与请求成员顺序一致,按索引取 agent id
     * 才不会在"同一个 agent 出现多次 / 成员未指定 agent"时错位。
     */
    fun dispatch(instruction: String) {
        val cmd = instruction.trim()
        if (cmd.isEmpty() || selected.isEmpty()) return
        lastInstruction = cmd
        dispatchError = null
        memberErrors = emptyMap()
        // 每个选中 agent 先落 ACK(已收到,客户端事实)
        phases = selected.associateWith { CollabAgentPhase.ACK }
        // 成员列表定序一次:后面按索引与返回项对位,避免 Set 迭代顺序带来的歧义。
        val ordered = selected.toList()
        val members = ordered.map { id ->
            CollabMember.newBuilder()
                .setAgentId(id)
                .setRoleId(roleBinding[id].orEmpty())
                .build()
        }
        scope.launch {
            // 短暂延时只为让"已收到"这一拍在界面上可见一瞬(纯 UX,非网络调用),
            // 随后进入真实派发;派发结果决定各家相位,不编造任何输出。
            delay(140)
            try {
                val resp = withContext(Dispatchers.IO) {
                    AgentConnection.threadService().dispatchCollab(
                        DispatchCollabRequest.newBuilder()
                            .setInstruction(cmd)
                            .addAllMembers(members)
                            .build()
                    )
                }
                sessionId = resp.sessionId.ifBlank { null }
                val nextPhases = phases.toMutableMap()
                val nextThreads = threadIds.toMutableMap()
                val nextErrors = memberErrors.toMutableMap()
                resp.itemsList.forEachIndexed { i, item ->
                    // item.agentId 在"成员没指定 agent"时可能为空,此时回退到请求里那个。
                    val aid = item.agentId.ifBlank { ordered.getOrNull(i).orEmpty() }
                    if (aid.isEmpty()) return@forEachIndexed
                    if (item.ack) {
                        nextPhases[aid] = CollabAgentPhase.RUNNING
                        if (item.threadId.isNotBlank()) nextThreads[aid] = item.threadId
                    } else {
                        nextPhases[aid] = CollabAgentPhase.ERROR
                        nextErrors[aid] = item.error.ifBlank { "该成员派发失败,但 daemon 没给出原因" }
                    }
                }
                phases = nextPhases
                threadIds = nextThreads
                memberErrors = nextErrors
            } catch (e: Exception) {
                // 连 RPC 都没通(未连接/鉴权/传输):整体失败,如实给原文,不粉饰。
                dispatchError = extractRpcMessage(e)
                phases = selected.associateWith { CollabAgentPhase.ERROR }
            }
        }
    }

    /**
     * 异常 → 可透传原文:优先 daemon 设的 Status.description(干净人话,去掉 gRPC
     * 状态码前缀);空白退回 e.message;再空白退回类名(可核查的真实信息,非"未知错误")。
     */
    private fun extractRpcMessage(e: Exception): String {
        val desc = Status.fromThrowable(e).description
        if (!desc.isNullOrBlank()) return desc
        return e.message?.takeIf { it.isNotBlank() } ?: e::class.java.simpleName
    }

    /** 退出/重开协同:清空本次会话的本地状态。 */
    fun reset() {
        selected = emptySet(); roleBinding = emptyMap(); phases = emptyMap(); lastInstruction = null
        sessionId = null; threadIds = emptyMap(); memberErrors = emptyMap(); dispatchError = null
    }
}
