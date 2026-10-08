package dev.pocketagent.app.ui.chat

import androidx.compose.foundation.layout.size
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import dev.pocketagent.app.ui.ApprovalPhase
import dev.pocketagent.app.ui.ApprovalRequestUi
import dev.pocketagent.app.ui.ApprovalSheet
import dev.pocketagent.app.ui.component.PaHaptics
import dev.pocketagent.app.ui.TurnSession
import dev.pocketagent.app.ui.agentBrandIcon
import dev.pocketagent.app.ui.approvalUiCopy
import dev.pocketagent.app.ui.askIsAskAction
import dev.pocketagent.app.ui.selectedAgentLabel
import dev.pocketagent.app.ui.toApprovalOptionUi
import dev.pocketagent.app.ui.decideEnabled
import dev.pocketagent.app.ui.markUnavailable
import dev.pocketagent.app.ui.retry
import dev.pocketagent.app.ui.serverConfirmed
import dev.pocketagent.app.ui.settled
import dev.pocketagent.app.ui.submitting
import dev.pocketagent.app.ui.component.PaApprovalDialog
import kotlinx.coroutines.delay

/**
 * 审批弹窗状态机(C1 拆分,2026-10-09;皮 = component/PaApprovalDialog,反问类走
 * ApprovalSheet):请求到达组装 PENDING 并自动弹窗 → 提交 → serverConfirmed →
 * 终态词一拍 → 收弹窗;失败 markUnavailable + 重试;请求消失按相位收场。
 * 从 ChatScreen 巨函数整体迁出,效果与渲染一字不差;弹窗开合旗标
 * [approvalDialogOpen] 由 ChatScreen 持有(时间线入口卡/身份徽章共用)。
 */
@Composable
internal fun ChatApprovalFlow(
    threadId: String,
    threadAgentId: String,
    threadTitle: String,
    threadWorkspace: String?,
    session: TurnSessionUiState,
    st: TurnSession.State,
    hapticView: android.view.View,
    approvalDialogOpen: MutableState<Boolean>,
) {
    // =====================================================================
    // 审批弹窗状态机(README §审批弹窗状态机;皮 = component/PaApprovalDialog,
    // 纯逻辑 = ui/EditorialApproval,数据流 = 现有 daemon ApprovalRequest 链)
    //
    // 请求(TurnSession.APPROVAL_REQUEST → st.pendingApproval)
    //   → 组装 ApprovalRequestUi(PENDING)并自动弹窗
    //   → onDecide → submitting(allow) → TurnSession.answerApproval(现有 SubmitApproval 链)
    //   → 请求从 st.pendingApproval 移除 = 服务端链路收下 → serverConfirmed → settled
    //     → ALLOWED/DENIED 终态词展示一拍 → 清弹窗、清待审批 UI
    //   → 提交失败(st.approvalError 非空且请求仍在)→ markUnavailable(人话原因)+ 重试
    //   → 请求在电脑端被先行处理(APPROVAL_RESOLVED/轮次终局清卡)→ markUnavailable 收场
    // 红线:dismiss(弹窗外点击/返回)= 暂不处理,请求保持待审批,绝不等于拒绝;
    //       不做乐观通过 —— 终态词只在提交链路收下请求后出现。
    // =====================================================================
    var approvalUi by remember(threadId) { mutableStateOf<ApprovalRequestUi?>(null) }
    LaunchedEffect(session.pendingApproval?.requestId) {
        val ap = session.pendingApproval
        if (ap == null) {
            val ui = approvalUi ?: return@LaunchedEffect
            approvalUi = when (ui.phase) {
                // 提交在途时请求消失 = answerApproval 已收下(通道/RPC 二选一成功)
                // → 服务端确认,落终态词(「移除待审批 UI」自此有了确认依据)
                ApprovalPhase.SUBMITTING -> ui.serverConfirmed(ui.submittedChoice == true).settled()
                // 未提交时请求消失 = 已在电脑端被处理(快照回填/竞速清理)→ 如实说,不给假决定
                ApprovalPhase.RECEIVED, ApprovalPhase.PENDING ->
                    ui.markUnavailable("这条审批已在电脑端被处理，无需再次决定")
                else -> ui
            }
        } else if (approvalUi?.requestId != ap.requestId) {
            // 新请求到达:组装白名单文案并自动弹窗(既有"请求即达"行为,容器换成弹窗)
            val copy = approvalUiCopy(ap)
            approvalUi = ApprovalRequestUi(
                requestId = ap.requestId,
                threadId = threadId,
                agentName = selectedAgentLabel(threadAgentId),
                sessionTitle = threadTitle.ifBlank { "会话" },
                purpose = copy.purpose,
                workspace = copy.cwd.ifBlank { threadWorkspace.orEmpty() },
                actionKind = copy.actionKind,
                command = copy.command,
                phase = ApprovalPhase.PENDING,
                // 【R2 三选项装配】daemon(R1-C1)透出的选项列表与详情:有则审批卡
                // 渲染三按钮(本次允许/本会话允许/拒绝);旧 daemon 无此字段 → 空
                // 列表走旧两按钮分支,双向兼容。
                options = ap.optionsList.toApprovalOptionUi(),
                details = ap.details,
            )
            approvalDialogOpen.value = true
        }
    }
    // 提交失败:TurnSession 置 approvalError 且请求仍在 → 暂无法处理(人话原因+重试)
    LaunchedEffect(session.approvalError) {
        val err = session.approvalError ?: return@LaunchedEffect
        val ui = approvalUi ?: return@LaunchedEffect
        if (session.pendingApproval?.requestId == ui.requestId) {
            approvalUi = ui.markUnavailable(err)
        }
    }
    // 相位收尾:CONFIRMED 一拍后落终态词;终态/请求已消失的失效态展示一拍后自动收弹窗。
    // 提交失败的 UNAVAILABLE(请求仍在)**不**自动关 —— 保留请求和决定界面等重试。
    LaunchedEffect(approvalUi?.phase) {
        val ui = approvalUi ?: return@LaunchedEffect
        when (ui.phase) {
            ApprovalPhase.CONFIRMED -> {
                delay(500)
                approvalUi = approvalUi?.settled()
            }
            ApprovalPhase.ALLOWED, ApprovalPhase.DENIED -> {
                delay(900)
                approvalDialogOpen.value = false
                approvalUi = null
            }
            ApprovalPhase.UNAVAILABLE -> if (session.pendingApproval == null) {
                delay(1400)
                approvalDialogOpen.value = false
                approvalUi = null
            }
            else -> {}
        }
    }

    // 反问类(request_user_input / mcp_elicitation)仍走既有问题卡:PaApprovalDialog
    // 的白名单没有"问题/选项/自由输入"的槽位(基座组件禁改),而反问必须把答案经
    // note 通道回给 daemon —— 两类语义在数据层同链,在界面层分容器。红线语义
    // (不可下滑关闭/拦截返回/不替用户做决定)由 PaSheetConvergenceTest 钉住不动。
    session.pendingApproval?.let { ap ->
        if (askIsAskAction(ap.action.name)) {
            ApprovalSheet(
                action = ap,
                error = session.approvalError,
                onDecision = { approved, note ->
                    // 语义触觉(P1-5):审批提交 = 轻击确认 —— 手指离开屏幕的瞬间就有
                    // "已交出去了"的物理回执,不等网络往返。
                    PaHaptics.confirm(hapticView)
                    TurnSession.answerApproval(threadId, ap.requestId, approved, note)
                },
            )
        }
    }

    // 审批弹窗本体:phase 驱动,决定按钮/重试/暂不处理三条通道(见上方状态机注释)
    approvalUi?.let { ui ->
        if (approvalDialogOpen.value) {
            val brandRes = agentBrandIcon(threadAgentId)
            val agentIconSlot: (@Composable () -> Unit)? = if (brandRes != null) {
                {
                    Icon(
                        painterResource(brandRes), null,
                        tint = Color.Unspecified, modifier = Modifier.size(28.dp),
                    )
                }
            } else {
                null
            }
            PaApprovalDialog(
                request = ui,
                onDecide = { allow ->
                    if (ui.decideEnabled) {
                        // 触感按决定语义分档(同动作全局同强度):允许 = confirm
                        // (已交出去);拒绝 = warning(打断电脑端的活,分量更重)。
                        if (allow) PaHaptics.confirm(hapticView) else PaHaptics.warning(hapticView)
                        approvalUi = ui.submitting(allow)
                        // 现有 SubmitApproval 链:通道直发,失败回退 submitApproval RPC;
                        // 请求是否被收下以 st.pendingApproval 的移除为准(见上方状态机)
                        TurnSession.answerApproval(threadId, ui.requestId, allow)
                    }
                },
                onDecideOption = { optionId ->
                    if (ui.decideEnabled) {
                        // 三按钮的第三按钮(acceptForSession 等):同为允许语义 →
                        // confirm 档;选项 id 原词走同一条 answerApproval 链
                        // (approved=true + option,daemon 按协议版本翻译)。
                        PaHaptics.confirm(hapticView)
                        approvalUi = ui.submitting(allow = true, option = optionId)
                        TurnSession.answerApproval(
                            threadId, ui.requestId, approved = true, option = optionId,
                        )
                    }
                },
                onDismiss = {
                    // 关闭 = 暂不处理(请求保持待审批,入口卡/身份行可再次打开)。
                    // 仅提交中/服务端确认中不关 —— 决定已在途,关掉会让确认没有落点。
                    if (ui.phase != ApprovalPhase.SUBMITTING && ui.phase != ApprovalPhase.CONFIRMED) {
                        approvalDialogOpen.value = false
                    }
                },
                onRetry = {
                    st.approvalError = null
                    approvalUi = approvalUi?.retry()
                },
                agentIcon = agentIconSlot,
            )
        }
    }
}
