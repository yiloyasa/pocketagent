package dev.pocketagent.app.ui.component

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.ApprovalPhase
import dev.pocketagent.app.ui.ApprovalRequestUi
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.approvalQueueLabel
import dev.pocketagent.app.ui.decideEnabled
import dev.pocketagent.app.ui.phaseTone
import dev.pocketagent.app.ui.phaseWord
import dev.pocketagent.app.ui.retryEnabled
import dev.pocketagent.app.ui.theme.PaEditorial

/** 拒绝按钮(.danger):浅珊瑚底 + 珊瑚描边 + 加深珊瑚字。 */
private val DenyButtonColors: ButtonTones get() = ButtonTones(
    container = PaEditorial.DeniedContainer, outline = PaEditorial.Denied.copy(alpha = 0.35f), content = PaEditorial.DeniedText,
)

/** 允许按钮(.primary 的编辑式稿形态):石墨底白字(README:主操作用石墨重点面,主色只给"当前操作")。 */
private val AllowButtonColors: ButtonTones get() = ButtonTones(
    container = PaEditorial.Graphite, outline = PaEditorial.Graphite, content = PaEditorial.OnGraphite,
)

private data class ButtonTones(val container: Color, val outline: Color, val content: Color)

/**
 * 审批弹窗(README「实现落地建议」四类共享组件之三;四类里的最上层)。
 *
 * 【状态机】由纯逻辑层 ui/EditorialApproval.kt 承载(单测钉死),本组件只画:
 * 收到请求/待决定 → 决定按钮可用;提交中 → 禁重复点击、单按钮回显「提交中…」、
 * 上下文仍可看;已允许/已拒绝 → 终态词锁定;暂无法处理(失效/重复/超时/提交
 * 失败)→ 禁用决定 + 原因 + 重试。**绝无乐观通过**:组件不存在任何通向
 * onDecide(true) 的内部路径,允许与否只来自宿主对服务端确认的回填。
 *
 * 【dismiss ≠ 拒绝】弹窗外点击、系统返回都走 [onDismiss](= 暂不处理,请求留在
 * 待处理区),与 [onDecide](false) 是两条回调、两条语义 —— 第二波接线时:
 * onDismiss 落"留在待审批",onDecide 才落决定。弹窗底部常驻这句承诺。
 *
 * 【只放必要信息】Agent 与会话、操作目的、工作区、操作类型、原始命令 + 两个
 * 按钮(README §审批弹窗状态机白名单);不放风险评分,不放多余跳转。
 *
 * 【图标槽】[agentIcon] 缺省不渲染任何头像 —— 未知 Agent 保持中性,不冒认品牌。
 *
 * @param request  纯逻辑状态值(见 [ApprovalRequestUi])。
 * @param onDecide 用户点了「拒绝」(false)/「仅本次允许」(true);仅当
 *   [ApprovalRequestUi.decideEnabled] 时组件才会回调。
 * @param onDismiss 弹窗外点击/系统返回 = 暂不处理;**绝不**等价于拒绝。
 * @param onRetry   「重试」(暂无法处理态);仅当 [ApprovalRequestUi.retryEnabled]。
 * @param agentIcon Agent 品牌图标槽(44dp 内);null = 中性留空。
 */
@Composable
fun PaApprovalDialog(
    request: ApprovalRequestUi,
    onDecide: (allow: Boolean) -> Unit,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
    onRetry: () -> Unit = {},
    agentIcon: (@Composable () -> Unit)? = null,
) {
    Dialog(
        onDismissRequest = onDismiss, // 关闭 = 暂不处理;与决定是两条通道
        properties = DialogProperties(usePlatformDefaultWidth = false),
    ) {
        val scrimInteraction = remember { MutableInteractionSource() }
        Box(
            modifier
                .fillMaxSize()
                .background(PaEditorial.Scrim)
                .clickable(interactionSource = scrimInteraction, indication = null) { onDismiss() },
            contentAlignment = Alignment.Center,
        ) {
            val cardInteraction = remember { MutableInteractionSource() }
            Column(
                Modifier
                    .padding(horizontal = 23.dp)
                    .widthIn(max = 360.dp)
                    .clip(RoundedCornerShape(23.dp))
                    .background(PaEditorial.Surface)
                    .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(23.dp))
                    // 卡片吃掉点击,不让冒泡到遮罩(点卡片不等于点窗外)
                    .clickable(interactionSource = cardInteraction, indication = null) {}
                    .padding(horizontal = 22.dp, vertical = 18.dp),
            ) {
                ApprovalBody(request, onDecide, onRetry, agentIcon)
            }
        }
    }
}

@Composable
private fun ApprovalBody(
    request: ApprovalRequestUi,
    onDecide: (allow: Boolean) -> Unit,
    onRetry: () -> Unit,
    agentIcon: (@Composable () -> Unit)?,
) {
    Column(Modifier.verticalScroll(rememberScrollState())) {
        // 拖柄(premium.css .modal .handle)
        Box(
            Modifier
                .align(Alignment.CenterHorizontally)
                .padding(bottom = 14.dp)
                .size(width = 36.dp, height = 4.dp)
                .background(PaEditorial.Subtle, RoundedCornerShape(100.dp)),
        )
        // 标题区:琥珀盾牌 + 「确认执行命令」+ 来自 Agent · 会话 + 排队数量
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                Modifier
                    .size(42.dp)
                    .background(PaEditorial.AttentionContainer, RoundedCornerShape(13.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    painterResource(R.drawable.ic_lucide_shield), null,
                    tint = PaEditorial.Attention, modifier = Modifier.size(20.dp),
                )
            }
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    "确认执行命令",
                    fontSize = 16.sp, lineHeight = 22.sp,
                    fontWeight = FontWeight.Bold, letterSpacing = (-0.3).sp,
                    color = PaEditorial.Ink,
                )
                Text(
                    "来自 ${request.agentName} · ${request.sessionTitle}",
                    fontSize = 11.sp, lineHeight = 16.sp,
                    color = PaEditorial.Muted,
                    maxLines = 1,
                )
                val queue = approvalQueueLabel(request.queuePosition, request.queueTotal)
                if (queue.isNotEmpty()) {
                    Spacer(Modifier.height(2.dp))
                    Text(
                        queue,
                        fontSize = 11.sp, lineHeight = 16.sp,
                        fontWeight = FontWeight(650),
                        color = PaEditorial.Attention,
                        maxLines = 1,
                    )
                }
            }
            if (agentIcon != null) {
                Spacer(Modifier.width(10.dp))
                Box(Modifier.size(36.dp), contentAlignment = Alignment.Center) { agentIcon() }
            }
        }
        Spacer(Modifier.height(12.dp))
        // 阶段状态行:状态词 + 语义色(双通道;liveRegion 让读屏听到状态变化)
        val word = request.phaseWord
        Row(
            Modifier
                .semantics { liveRegion = LiveRegionMode.Polite; contentDescription = "审批状态：$word" },
            verticalAlignment = Alignment.CenterVertically,
        ) {
            val tone = request.phaseTone
            if (request.phase == ApprovalPhase.SUBMITTING) {
                CircularProgressIndicator(
                    strokeWidth = 2.dp, color = tone.accent, modifier = Modifier.size(10.dp),
                )
            } else {
                Box(
                    Modifier
                        .size(8.dp)
                        .background(tone.accent, RoundedCornerShape(100.dp)),
                )
            }
            Spacer(Modifier.width(6.dp))
            Text(word, fontSize = 12.sp, lineHeight = 17.sp, fontWeight = FontWeight(650), color = tone.text)
        }
        Spacer(Modifier.height(8.dp))
        // 操作目的(README 白名单:操作目的;agent 给出的 reason 原文)
        Text(
            request.purpose,
            fontSize = 13.sp, lineHeight = 21.sp,
            color = PaEditorial.Muted,
        )
        Spacer(Modifier.height(12.dp))
        // 范围行:工作区 / 操作类型(premium.css .scope:细线分隔 + 左灰右墨)
        ScopeRow("工作区", request.workspace)
        ScopeRow("操作类型", request.actionKind)
        Spacer(Modifier.height(10.dp))
        // 原始命令:石墨代码箱(侧边栏/代码预览保留石墨深色的纪律)
        if (request.command.isNotBlank()) {
            Text(
                request.command,
                Modifier
                    .fillMaxWidth()
                    .background(PaEditorial.GraphiteCode, RoundedCornerShape(13.dp))
                    .border(1.dp, PaEditorial.GraphiteCode, RoundedCornerShape(13.dp))
                    .padding(horizontal = 14.dp, vertical = 14.dp),
                fontSize = 12.sp, lineHeight = 18.sp,
                color = PaEditorial.CodeText,
                maxLines = 6,
            )
            Spacer(Modifier.height(14.dp))
        }
        // 暂无法处理:原因 + 重试(失效/重复/超时/提交失败;不显示已批准)
        if (request.phase == ApprovalPhase.UNAVAILABLE) {
            Column(
                Modifier
                    .fillMaxWidth()
                    .background(PaEditorial.AttentionContainer, RoundedCornerShape(13.dp))
                    .border(1.dp, PaEditorial.Attention.copy(alpha = 0.30f), RoundedCornerShape(13.dp))
                    .padding(horizontal = 14.dp, vertical = 12.dp),
            ) {
                Text(
                    request.unavailableReason ?: "请求状态已变化，请刷新后重试",
                    fontSize = 12.sp, lineHeight = 17.sp,
                    color = PaEditorial.AttentionText,
                )
                Text(
                    "决定按钮已停用，处理完成后可重试。",
                    fontSize = 11.sp, lineHeight = 16.sp,
                    color = PaEditorial.AttentionText,
                )
            }
            Spacer(Modifier.height(12.dp))
            Row(
                Modifier
                    .fillMaxWidth()
                    .pressable(enabled = request.retryEnabled) { onRetry() },
                horizontalArrangement = Arrangement.Center,
            ) {
                Icon(
                    painterResource(R.drawable.ic_lucide_refresh_cw), null,
                    tint = PaEditorial.Primary, modifier = Modifier.size(14.dp),
                )
                Spacer(Modifier.width(6.dp))
                Text(
                    "重试",
                    fontSize = 13.sp, lineHeight = 18.sp,
                    fontWeight = FontWeight.SemiBold, color = PaEditorial.Primary,
                )
            }
            Spacer(Modifier.height(10.dp))
        }
        // 决定按钮:拒绝 / 仅本次允许(提交中禁重复;终态锁定)
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            ApprovalButton(
                text = if (request.phase == ApprovalPhase.SUBMITTING && request.submittedChoice == false) {
                    "提交中…"
                } else {
                    "拒绝"
                },
                tones = DenyButtonColors,
                enabled = request.decideEnabled,
                showSpinner = request.phase == ApprovalPhase.SUBMITTING && request.submittedChoice == false,
                modifier = Modifier.weight(1f),
            ) { onDecide(false) }
            ApprovalButton(
                text = if (request.phase == ApprovalPhase.SUBMITTING && request.submittedChoice == true) {
                    "提交中…"
                } else {
                    "仅本次允许"
                },
                tones = AllowButtonColors,
                enabled = request.decideEnabled,
                showSpinner = request.phase == ApprovalPhase.SUBMITTING && request.submittedChoice == true,
                modifier = Modifier.weight(1f),
            ) { onDecide(true) }
        }
        Spacer(Modifier.height(10.dp))
        // 常驻承诺:关闭 ≠ 拒绝(README 红线的界面落点)
        Text(
            "关闭弹窗只会暂不处理，不会自动允许或拒绝。",
            Modifier.fillMaxWidth(),
            fontSize = 11.sp, lineHeight = 16.sp,
            textAlign = TextAlign.Center,
            color = PaEditorial.Subtle,
        )
    }
}

@Composable
private fun ScopeRow(label: String, value: String) {
    if (value.isBlank()) return
    Column(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 9.dp),
    ) {
        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(label, fontSize = 12.sp, lineHeight = 17.sp, color = PaEditorial.Muted)
            Text(
                value,
                fontSize = 13.sp, lineHeight = 18.sp,
                fontWeight = FontWeight.Bold, color = PaEditorial.Ink,
                maxLines = 2,
            )
        }
        Spacer(Modifier.height(9.dp))
        Box(
            Modifier
                .fillMaxWidth()
                .height(1.dp)
                .background(PaEditorial.Hairline),
        )
    }
}

@Composable
private fun ApprovalButton(
    text: String,
    tones: ButtonTones,
    enabled: Boolean,
    showSpinner: Boolean,
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(14.dp)
    val container = if (enabled) tones.container else tones.container.copy(alpha = 0.55f)
    Row(
        modifier
            .heightIn(min = 50.dp)
            .background(container, shape)
            .border(1.dp, if (enabled) tones.outline else tones.outline.copy(alpha = 0.55f), shape)
            .pressable(enabled = enabled, onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 13.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.Center,
    ) {
        if (showSpinner) {
            CircularProgressIndicator(
                strokeWidth = 2.dp,
                color = tones.content,
                modifier = Modifier.size(12.dp),
            )
            Spacer(Modifier.width(6.dp))
        }
        Text(
            text,
            fontSize = 14.sp, lineHeight = 20.sp,
            fontWeight = FontWeight.Bold,
            color = if (enabled) tones.content else tones.content.copy(alpha = 0.55f),
            maxLines = 1,
        )
    }
}
