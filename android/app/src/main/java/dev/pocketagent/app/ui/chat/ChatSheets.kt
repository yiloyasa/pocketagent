package dev.pocketagent.app.ui.chat

import android.content.Context
import android.widget.Toast
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.Button
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.ChatDeniedColor
import dev.pocketagent.app.ui.Attachment
import dev.pocketagent.app.ui.TurnSession
import dev.pocketagent.app.ui.compactionConsequenceCopy
import dev.pocketagent.app.ui.compactActionOn
import dev.pocketagent.app.ui.copyMessage
import dev.pocketagent.app.ui.forkActionOn
import dev.pocketagent.app.ui.forkConsequence
import dev.pocketagent.app.ui.loadAttachmentPreview
import dev.pocketagent.app.ui.rollbackActionOn
import dev.pocketagent.app.ui.rollbackConsequence
import dev.pocketagent.app.ui.rollbackOptionLabel
import dev.pocketagent.app.ui.rollbackOptions
import dev.pocketagent.app.ui.rollbackSummary
import dev.pocketagent.app.ui.GoalDisplayLogic
import dev.pocketagent.app.ui.NativePermissionsSection
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.pressableLong
import dev.pocketagent.app.ui.component.DrawerController
import dev.pocketagent.app.ui.component.PaHaptics
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.SheetAction
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaHacker
import dev.pocketagent.app.ui.theme.paGlass
import kotlinx.coroutines.launch
import pocketagent.v1.Thread as PThread

/**
 * 会话页低频面板族(C1 拆分,2026-10-09):附件大图预览 / 更多(终止-导出-压缩-
 * 分叉-回滚) / 斜杠伪命令"/目标" / 回滚确认。各自从 ChatScreen 巨函数整体迁出,
 * 渲染与交互一字不差;RPC(本组仅 SetThreadGoal)已收 ChatViewModel(2026-10-09
 * 效果层外移,面板只消费 Result 出 Toast)。
 */
@Composable
internal fun ChatAttachmentViewer(
    selectedAttachment: MutableState<Attachment?>,
    ctx: Context,
) {
    selectedAttachment.value?.let { attachment ->
        val preview = loadAttachmentPreview(ctx, attachment)
        // 【2026-10-09 收编】原裸 Dialog(usePlatformDefaultWidth = false)→ PaSheet
        // fullBleed 满幅变体(居中家族的"看图"档):大图预览要满幅沉浸,塞 400dp 卡
        // 会毁掉检视体验;关闭语义逐位一致(返回键/点窗外 = onDismissRequest),
        // 点内容仍由内容自有的 clickable 承接"点图即关"。
        // scrollable = false:大图预览无滚动。
        PaSheet(
            onDismissRequest = { selectedAttachment.value = null },
            centered = true,
            fullBleed = true,
            scrollable = false,
        ) {
            Column(
                Modifier
                    .fillMaxSize()
                    .background(Color.Black.copy(alpha = 0.92f))
                    .clickable { selectedAttachment.value = null },
            ) {
                // 标题行:预览态显示**完整文件名**(不省略、可换行),长按复制完整名。
                // 这样"名字被裁"的老问题在点开预览后彻底消失(需求:预览标题行要完整可见)。
                // 【N4】statusBars padding:fullBleed 满幅档铺到 Dialog 窗口顶,标题行
                // 不让开状态栏会被刘海/挖孔裁切(真机实锤)。
                Text(
                    attachment.name,
                    Modifier
                        .fillMaxWidth()
                        .statusBarsPadding()
                        .pressableLong(
                            onClick = {},
                            onLongPress = { copyMessage(ctx, attachment.name) },
                        )
                        .padding(horizontal = 20.dp, vertical = 16.dp),
                    color = Color.White,
                    fontSize = PaEditorial.Type.Body.fontSize,
                    maxLines = 3,
                    softWrap = true,
                    overflow = TextOverflow.Ellipsis,
                )
                Box(
                    Modifier.fillMaxWidth().weight(1f),
                    contentAlignment = Alignment.Center,
                ) {
                    if (preview != null) {
                        androidx.compose.foundation.Image(
                            preview,
                            contentDescription = attachment.name,
                            contentScale = ContentScale.Fit,
                            modifier = Modifier
                                .fillMaxSize()
                                .padding(18.dp),
                        )
                    } else {
                        Text("无法预览：${attachment.name}", color = Color.White)
                    }
                }
            }
        }
    }
}

/**
 * 「更多」面板(C1 拆分,2026-10-09):终止/最近会话/导出 + 能力位门控的
 * 压缩/分叉/回滚。从 ChatScreen 巨函数整体迁出,渲染与门控一字不差。
 */
@Composable
internal fun ChatMoreSheet(
    showMore: MutableState<Boolean>,
    showExport: MutableState<Boolean>,
    rollbackOpen: MutableState<Boolean>,
    drawerCtrl: dev.pocketagent.app.ui.component.DrawerController,
    threadId: String,
    threadAgentId: String,
    hostCaps: List<String>,
    running: Boolean,
    compactBusy: Boolean,
    ctx: android.content.Context,
    hapticView: android.view.View,
    onCompact: () -> Unit,
    onFork: () -> Unit,
) {
    // ---- 更多:分叉 / 回滚 ----
    if (showMore.value) {
        // 【本轮换 PaSheet】统一玻璃浮层(滚动/高度上限/安全区内建)
        PaSheet(onDismissRequest = { showMore.value = false }) {
            Column(Modifier.fillMaxWidth().padding(bottom = 26.dp)) {
                Text(
                    "会话操作",
                    Modifier.fillMaxWidth().padding(horizontal = 24.dp),
                    fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink,
                )
                Spacer(Modifier.height(10.dp))
                if (threadAgentId=="zcode") {
                    Column(Modifier.fillMaxWidth().padding(horizontal=20.dp)) {
                        NativePermissionsSection("zcode",threadId)
                    }
                }
                // 【能力位门控】三个动作按 daemon 逐 agent 宣告的 compact/fork/
                // rollback:<agentId> 位点亮(判定见 ThreadActionGateTest 钉死的纯函数):
                // dsh 会话三项全无位 → 只剩标题与(zcode 的)权限区,不摆点了必败的
                // 假入口;分隔线只在两个可见区段之间出现,不出现在末尾。
                val compactOn = compactActionOn(threadAgentId, hostCaps)
                val forkOn = forkActionOn(threadAgentId, hostCaps)
                val rollbackOn = rollbackActionOn(threadAgentId, hostCaps)
                // 编辑式骨架(05):顶栏只留一个次操作(「更多」),低频动作收进本面板;
                // 黑客终端顶栏按钮原样保留,不重复给入口。
                if (!PaHacker.isHacker) {
                    if (running) {
                        SheetAction(AppR.drawable.ic_lucide_square, "终止当前任务", ChatDeniedColor) {
                            showMore.value = false
                            // 终止 = warning 档(与顶栏终止键、发送键原位终止同档)
                            PaHaptics.warning(hapticView)
                            TurnSession.cancelTurn(threadId)
                            Toast.makeText(ctx, "已请求终止电脑端任务", Toast.LENGTH_SHORT).show()
                        }
                        Spacer(Modifier.height(6.dp))
                    }
                    SheetAction(AppR.drawable.ic_lucide_menu, "最近会话") {
                        showMore.value = false
                        drawerCtrl.open()
                    }
                    SheetAction(AppR.drawable.ic_lucide_download, "导出对话") {
                        showMore.value = false
                        showExport.value = true
                    }
                    if (threadAgentId == "zcode" || compactOn || forkOn || rollbackOn) {
                        Spacer(Modifier.height(6.dp))
                        HorizontalDivider(Modifier.padding(horizontal = 24.dp), color = PaEditorial.Hairline)
                        Spacer(Modifier.height(6.dp))
                    }
                }
                if (compactOn) {
                    SheetAction(
                        AppR.drawable.ic_lucide_refresh_cw,
                        if (compactBusy) "压缩中…" else "压缩上下文",
                        PaEditorial.Primary,
                    ) { onCompact() }
                    Text(
                        // 文案按会话归属 agent 动态(daemon 的 CompactThread 已按 AgentID
                        // 路由):Pi 会话说"让 Pi 压缩",codex 会话说"让 Codex 压缩"。
                        // 真机实锤(图9):过去硬编码"让 Codex 压缩…",Pi 会话里张冠李戴。
                        compactionConsequenceCopy(threadAgentId),
                        Modifier.padding(start = 24.dp, end = 24.dp, bottom = 10.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Subtle,
                    )
                    if (forkOn || rollbackOn) {
                        HorizontalDivider(Modifier.padding(horizontal = 24.dp), color = PaEditorial.Hairline)
                    }
                }
                if (forkOn) {
                    SheetAction(AppR.drawable.ic_lucide_git_branch, "分叉会话") { onFork() }
                    Text(
                        forkConsequence(),
                        Modifier.padding(start = 24.dp, end = 24.dp, bottom = 10.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Subtle,
                    )
                    if (rollbackOn) {
                        HorizontalDivider(Modifier.padding(horizontal = 24.dp), color = PaEditorial.Hairline)
                    }
                }
                if (rollbackOn) {
                    SheetAction(
                        AppR.drawable.ic_lucide_undo_2, "回滚…", PaEditorial.Denied,
                    ) {
                        showMore.value = false
                        rollbackOpen.value = true
                    }
                    Text(
                        rollbackSummary(),
                        Modifier.padding(start = 24.dp, end = 24.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Subtle,
                    )
                }
            }
        }
    }
}

/**
 * 斜杠伪命令"/目标"底栏(C1 拆分,2026-10-09):SetThreadGoal 落库并就地刷新
 * 顶栏目标文案。从 ChatScreen 巨函数整体迁出,交互一字不差。
 * 【2026-10-09 效果层外移】RPC 收进 [ChatViewModel.setThreadGoal](Result 形态),
 * 本面板经 [onSetGoal] 消费:成功刷新顶栏文案/关闭/人话回执,失败人话 Toast,
 * 文案与迁出前逐字一致。
 */
@Composable
internal fun ChatGoalSheet(
    slashGoalSheet: MutableState<Boolean>,
    threadGoalLine: MutableState<String?>,
    threadId: String,
    ctx: android.content.Context,
    scope: kotlinx.coroutines.CoroutineScope,
    /** 设置目标(目标原文照传):VM 出 Result,这里只出 Toast 与就地刷新。 */
    onSetGoal: suspend (String) -> Result<PThread>,
) {
    // ---- 斜杠面板伪命令"/目标":设置会话目标(SetThreadGoal)----
    // 由斜杠卡面的 /目标 伪命令打开(GOAL 动作,见上方候选段)。daemon 的
    // SetThreadGoal 按 agent 落库并下发(codex thread/goal/set、zcode session/goal,
    // pi 落库跟随),成功后用返回的 Thread 刷新顶栏目标文案,不整页重拉。
    if (slashGoalSheet.value) {
        var goalDraft by remember { mutableStateOf("") }
        var goalSaving by remember { mutableStateOf(false) }
        // 【本轮换 PaSheet】统一玻璃浮层
        PaSheet(onDismissRequest = { if (!goalSaving) slashGoalSheet.value = false }) {
            Column(Modifier.fillMaxWidth().padding(horizontal = 24.dp).padding(bottom = 26.dp)) {
                Text(
                    "设置会话目标",
                    Modifier.fillMaxWidth(),
                    fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink,
                )
                Text(
                    "目标会作为常驻上下文:agent 在这个会话里的每一轮都会看到它。",
                    Modifier.padding(top = 4.dp, bottom = 10.dp),
                    fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Subtle,
                )
                BasicTextField(
                    value = goalDraft,
                    onValueChange = { goalDraft = it },
                    modifier = Modifier
                        .fillMaxWidth()
                        // min 72 给短目标留形,max 200 封顶:目标再长输入框也不再增高,
                        // "保存目标"按钮始终在视口内(否则长目标把按钮推出 sheet 不可点,
                        // 同 SLA-21a / README §12 坑 6)。高度有界后 CoreTextField 内容
                        // 内部滚动并跟随光标,交互破坏最小(整列滚动会把按钮滚走)。
                        .heightIn(min = 72.dp, max = 200.dp)
                        .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .padding(12.dp),
                    textStyle = TextStyle(fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Ink),
                    cursorBrush = SolidColor(PaEditorial.Primary),
                    decorationBox = { inner ->
                        Box {
                            if (goalDraft.isEmpty()) {
                                Text(
                                    "例如:全程用 Kotlin,改动只落在 daemon 模块…",
                                    fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Subtle,
                                )
                            }
                            inner()
                        }
                    },
                )
                Spacer(Modifier.height(12.dp))
                Box(
                    Modifier
                        .fillMaxWidth()
                        .pressable(haptic = true) {
                            if (goalSaving) return@pressable
                            if (goalDraft.isBlank()) {
                                Toast.makeText(ctx, "先写点什么作为目标(或清空后关闭)", Toast.LENGTH_SHORT).show()
                                return@pressable
                            }
                            goalSaving = true
                            scope.launch {
                                try {
                                    // getOrThrow:失败从原 RPC 抛出点浮出,catch 文案零变化
                                    val t = onSetGoal(goalDraft.trim()).getOrThrow()
                                    // 顶栏目标文案就地刷新(GoalDisplayLogic 与 GetThread 同一套口径)
                                    threadGoalLine.value = GoalDisplayLogic.goalLine(t)
                                    slashGoalSheet.value = false
                                    Toast.makeText(ctx, "已设置会话目标", Toast.LENGTH_SHORT).show()
                                } catch (e: Exception) {
                                    Toast.makeText(ctx, e.message ?: "设置目标失败", Toast.LENGTH_LONG).show()
                                } finally {
                                    goalSaving = false
                                }
                            }
                        }
                        .background(if (goalSaving) PaEditorial.Hover else PaEditorial.Primary, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .padding(vertical = 13.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        if (goalSaving) "正在保存…" else "保存目标",
                        // saving 腿坐 Hover 底保持白字;就绪腿坐 Primary 实底读 OnPrimary
                        fontSize = PaEditorial.Type.Body.fontSize,
                        color = if (goalSaving) Color.White else PaEditorial.OnPrimary,
                        fontWeight = FontWeight.Medium,
                    )
                }
            }
        }
    }
}

/**
 * 回滚面板(C1 拆分,2026-10-09):选丢几个回合再确认;回合数从时间线用户消息
 * 现数,数不出就不给选项。从 ChatScreen 巨函数整体迁出,渲染一字不差。
 */
@Composable
internal fun ChatRollbackSheet(
    rollbackOpen: MutableState<Boolean>,
    userTurnCount: Int,
    rollbackBusy: Boolean,
    onRollback: (Int) -> Unit,
) {
    // ---- 回滚:选丢几个回合,再确认 ----
    // 回合数从时间线里现数(每条用户消息 = 一个回合)。数不出回合(历史还没加载出来)
    // 就不给选项 —— 免得用户选完必然失败。
    if (rollbackOpen.value) {
        val turnTotal = userTurnCount
        val options = rollbackOptions(turnTotal)
        var picked by remember(turnTotal) { mutableStateOf(1) }
        // 【本轮换 PaSheet】统一玻璃浮层;滚动由封装内建(选项随回合数增长时
        // "取消/确认回滚"不会被推出视口,同 SLA-21a 语义),内容列不再自带 verticalScroll。
        PaSheet(onDismissRequest = { rollbackOpen.value = false }) {
            // 整列可滚(PaSheet 提供):选项随回合数增长(每条用户消息=一个回合),长会话时
            // "取消/确认回滚"保持可点(被滚动裁掉的节点不进无障碍树也不可点,README §12 坑 6)。
            Column(
                Modifier
                    .fillMaxWidth()
                    .padding(bottom = 26.dp),
            ) {
                Text(
                    "回滚会话",
                    Modifier.fillMaxWidth().padding(horizontal = 24.dp),
                    fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink,
                )
                if (options.isEmpty()) {
                    Text(
                        "这个会话还没有可回滚的回合。",
                        Modifier.padding(horizontal = 24.dp, vertical = 10.dp),
                        fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Subtle,
                    )
                } else {
                    Text(
                        rollbackConsequence(picked),
                        Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 6.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
                    )
                    options.forEach { n ->
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .pressable { picked = n }
                                .padding(horizontal = 24.dp, vertical = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Icon(
                                painterResource(
                                    if (picked == n) AppR.drawable.ic_lucide_circle_check
                                    else AppR.drawable.ic_lucide_undo_2,
                                ),
                                null,
                                tint = if (picked == n) PaEditorial.Denied else PaEditorial.Subtle,
                                modifier = Modifier.size(18.dp),
                            )
                            Spacer(Modifier.width(12.dp))
                            Text(
                                rollbackOptionLabel(n, turnTotal),
                                fontSize = PaEditorial.Type.Heading.fontSize,
                                color = if (picked == n) PaEditorial.Ink else PaEditorial.Muted,
                            )
                        }
                    }
                    Row(
                        Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 10.dp),
                        horizontalArrangement = Arrangement.End,
                    ) {
                        TextButton(onClick = { rollbackOpen.value = false }, enabled = !rollbackBusy) {
                            Text("取消")
                        }
                        Spacer(Modifier.width(8.dp))
                        Button(
                            enabled = !rollbackBusy,
                            onClick = { onRollback(picked) },
                            colors = androidx.compose.material3.ButtonDefaults.buttonColors(
                                containerColor = PaEditorial.Denied,
                            ),
                            shape = RoundedCornerShape(PaEditorial.Radius.IconButton),
                        ) { Text("确认回滚", color = PaEditorial.OnDenied) }
                    }
                }
            }
        }
    }
}
