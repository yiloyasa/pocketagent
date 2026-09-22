package dev.pocketagent.app.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import dev.pocketagent.app.AgentConnection
import io.grpc.Status
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CollabRole
import pocketagent.v1.DeleteCollabRoleRequest
import pocketagent.v1.ListCollabRolesRequest
import pocketagent.v1.SetCollabRoleRequest

/** 角色列表的加载状态(纯枚举,UI 据此区分 加载中 / 就绪(含空) / 失败)。 */
enum class CollabRolesLoadState { LOADING, READY, ERROR }

/**
 * 协同角色配置的数据层:RPC 拉取/保存/删除 + Compose 可观察状态。
 *
 * 【与 [CollabRolesLogic] 的分工】本对象只管"网络 + 状态",所有展示规则(id 校验/
 * 显示名/摘要)都在 Logic 里,单测不依赖真机/daemon。
 *
 * 【错误透传】daemon 返回的 InvalidArgument 等人话文案,经 [extractRpcMessage] 只取
 * Status.description(干净的人话,不含 "INVALID_ARGUMENT:" 前缀),绝不泛化成"操作失败"
 * (AUDIT §3.3:禁显"未知/错误"裸字样)。
 *
 * 【gRPC 调用范式】沿用 [AgentConnection.threadService] 的阻塞 stub(与 ListThreads/
 * GetThreadHistory 同一套用法);三个新 RPC 的入参/回参由 proto 生成类直接给,无需手写序列化。
 */
object CollabRolesState {
    var roles by mutableStateOf<List<CollabRole>>(emptyList())
        private set
    var loadState by mutableStateOf(CollabRolesLoadState.LOADING)
        private set
    /** 最近一次加载失败的原文(成功/加载中为空),供 UI 如实展示。 */
    var loadError by mutableStateOf<String?>(null)
        private set

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    /**
     * 拉取角色列表(幂等:空列表 = 用户还没配过任何角色,不是错误,UI 走空态文案)。
     * 进列表页与回主页(点亮协同角色计数)都会调一次;失败只更新状态,不抛。
     */
    fun load() {
        loadState = CollabRolesLoadState.LOADING
        loadError = null
        scope.launch {
            try {
                val list = withContext(Dispatchers.IO) {
                    AgentConnection.threadService()
                        .listCollabRoles(ListCollabRolesRequest.getDefaultInstance())
                        .rolesList
                }
                roles = list
                loadState = CollabRolesLoadState.READY
            } catch (e: Exception) {
                loadError = extractRpcMessage(e)
                loadState = CollabRolesLoadState.ERROR
            }
        }
    }

    /**
     * 保存(新增/编辑共用):按 id 幂等 upsert。成功返回 null,失败返回 daemon 原文。
     * 调用方负责客户端前置校验(见 [validateCollabRoleId]),这里只透传后端错误。
     * 成功后用落库后的真值(daemon 可能 trim 过 id)刷新本地列表,避免回显 stale。
     */
    suspend fun save(role: CollabRole): String? = try {
        val saved = withContext(Dispatchers.IO) {
            AgentConnection.threadService()
                .setCollabRole(SetCollabRoleRequest.newBuilder().setRole(role).build())
                .role
        }
        roles = roles.filter { it.id != saved.id } + saved
        null
    } catch (e: Exception) {
        extractRpcMessage(e)
    }

    /**
     * 删除(幂等:删不存在的 id,daemon 不报错;本地直接移除)。
     * 成功返回 null;极端传输错误返回原文。
     */
    suspend fun delete(id: String): String? = try {
        withContext(Dispatchers.IO) {
            AgentConnection.threadService()
                .deleteCollabRole(DeleteCollabRoleRequest.newBuilder().setId(id).build())
        }
        roles = roles.filter { it.id != id }
        null
    } catch (e: Exception) {
        extractRpcMessage(e)
    }

    /**
     * 异常 → 可透传原文:优先 daemon 设的 Status.description(干净人话,去掉 gRPC 状态码前缀);
     * 空白退回 e.message;再空白退回类名(可核查的真实信息,非"未知错误")。
     */
    private fun extractRpcMessage(e: Exception): String {
        val desc = Status.fromThrowable(e).description
        if (!desc.isNullOrBlank()) return desc
        return e.message?.takeIf { it.isNotBlank() } ?: e::class.java.simpleName
    }
}
