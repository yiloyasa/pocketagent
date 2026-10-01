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
    /**
     * 一单派发在飞(RPC 未返回)。防重复提交的**硬闸**:置位期间 dispatch /
     * redispatchMember 直接忽略 —— 双击、IME 回车、重派连点都只算一单。
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
     * 【语义】不是"一个 agent 派子智能体",而是"多个对等 agent 各干各的" ——
     * daemon 给每个成员各开一个子线程同时跑,成员之间无父子关系。
     *
     * 【相位是真状态,不是动画】
     *  - ACK     = 手机上确实把指令交出去了(客户端事实);
     *  - RUNNING = daemon 已**受理**(子线程已建、轮次已起飞),结果还得等;
     *  - ERROR   = 这家受理失败,daemon 给了人话原因;
     *  - DONE    = 子线程跑完(由按 thread_id 回填)。
     * 受理(ack)不等于执行成功 —— 把受理显示成"已完成"就是伪造结果。
     */
    fun dispatch(instruction: String) {
        val cmd = instruction.trim()
        if (dispatching) return
        if (cmd.isEmpty() || selected.isEmpty()) return
        send(cmd, selected.toList(), memberOnly = false)
    }

    /**
     * 重派单个成员(结论 2:失败卡的「重派该 Agent」):其余成员的相位/原因**原样
     * 不动** —— 一家失败不拖垮其余,同样也不该在补跑一家时搅动别家。
     * 指令沿用 [lastInstruction](重派是对上一条共同指令的重来,不接受新指令文本);
     * 该成员旧的子线程 id 先移除:重派受理结果回来前不摆「查看会话」假入口。
     */
    fun redispatchMember(agentId: String) {
        if (dispatching) return
        val cmd = lastInstruction?.trim()?.takeIf { it.isNotEmpty() } ?: return
        if (agentId !in selected) return
        send(cmd, listOf(agentId), memberOnly = true)
    }

    /**
     * 派发的公共通道:[dispatch](全员)与 [redispatchMember](单成员)共用,
     * 相位/线程/错误的落点按 [memberOnly] 分流 —— 全员失败是整体错误位,单成员
     * 失败只落该成员,原因各归各位,不互相污染。
     */
    private fun send(cmd: String, ordered: List<String>, memberOnly: Boolean) {
        dispatching = true
        lastInstruction = cmd
        dispatchError = null
        if (memberOnly) {
            // 重派:先撤旧线程与旧原因 —— 受理结果回来前,卡上不该残留上一单的凭据。
            val ids = ordered.toSet()
            threadIds = threadIds.filterKeys { it !in ids }
            memberErrors = memberErrors.filterKeys { it !in ids }
        } else {
            memberErrors = emptyMap()
        }
        // 每个目标 agent 先落 ACK(已收到,客户端事实)
        phases = phases + ordered.associateWith { CollabAgentPhase.ACK }
        // 成员列表定序一次:后面按索引与返回项对位,避免 Set 迭代顺序带来的歧义。
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
                // 连 RPC 都没通(未连接/鉴权/传输):如实给原文,不粉饰。
                // 落点按范围分流:全员派发是整体错误位;单成员重派只落该成员,
                // 其余成员的相位与原因原样保留。
                if (memberOnly) {
                    phases = phases + ordered.associateWith { CollabAgentPhase.ERROR }
                    memberErrors = memberErrors + ordered.associateWith { extractRpcMessage(e) }
                } else {
                    dispatchError = extractRpcMessage(e)
                    phases = ordered.associateWith { CollabAgentPhase.ERROR }
                }
            } finally {
                dispatching = false
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
        dispatching = false
    }
}
