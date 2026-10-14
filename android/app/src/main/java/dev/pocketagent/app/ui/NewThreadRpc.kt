package dev.pocketagent.app.ui

import dev.pocketagent.app.AgentConnection
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import pocketagent.v1.CreateThreadRequest

/**
 * 三处新建入口(主页「新建任务」/ 会话列表 / 聊天页「新建对话」)共享的建会话 RPC:
 * 请求体构造(agent / 模型 / 工作区)+ IO 调用,成功只返回新会话 id。
 *
 * [applyAgentSelection] 决定是否套 [createThreadAgentId] 选中映射:
 *  - true  = 主页与聊天页口径(P1:设置页/主页切换后,新建才真正走选中 agent;
 *    codex 或空保持不传 = 旧语义,server 端默认路由到 codex);
 *  - false = 会话列表入口的既有口径(直接传 selectedAgent,行为保持零变化)。
 *
 * 停用闸([newThreadAgentAllowed])、确认触感、失败 Toast 的文案各入口不同,
 * 全部留在各入口 —— 这里只收敛「构造请求 + 发 RPC」这一段。
 */
internal suspend fun createThreadRpc(ws: String, applyAgentSelection: Boolean): String {
    val builder = CreateThreadRequest.newBuilder().setAgentId(AgentConnection.selectedAgent)
    if (AgentConnection.selectedModel.isNotEmpty()) {
        builder.setModelId(AgentConnection.modelForAgent(AgentConnection.selectedAgent))
    }
    if (applyAgentSelection) {
        // 选中值 -> 应写入请求的 agent_id(纯函数,NewThreadAgentSelectionTest 覆盖):
        // codex 或空返回 "" = 不覆盖,保持旧语义(server 端默认路由到 codex)。
        val agentId = createThreadAgentId(AgentConnection.selectedAgent)
        if (agentId.isNotEmpty()) {
            builder.setAgentId(agentId)
        }
    }
    if (ws.isNotBlank()) {
        builder.setWorkspace(ws)
    }
    return withContext(Dispatchers.IO) {
        AgentConnection.threadService().createThread(builder.build()).id
    }
}
