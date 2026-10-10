package dev.pocketagent.app.ui.chat

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.net.Uri
import android.widget.Toast
import androidx.activity.result.ActivityResultLauncher
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.requiredSize
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.APPROVALS
import dev.pocketagent.app.ui.ATTACH_CHIP_NAME_MAX
import dev.pocketagent.app.ui.Attachment
import dev.pocketagent.app.ui.AttachmentCard
import dev.pocketagent.app.ui.AttachmentKind
import dev.pocketagent.app.ui.ChatAccentColor
import dev.pocketagent.app.ui.ChatDeniedColor
import dev.pocketagent.app.ui.ChatInkColor
import dev.pocketagent.app.ui.ChatMutedColor
import dev.pocketagent.app.ui.ChatSubtleColor
import dev.pocketagent.app.ui.EXPAND_HINT_THRESHOLD
import dev.pocketagent.app.ui.agentBrandIcon
import dev.pocketagent.app.ui.attachmentEntryHiddenFor
import dev.pocketagent.app.ui.INPUT_TOUCH_TARGET_DP
import dev.pocketagent.app.ui.SLASH_UNKNOWN_AGENT_HINT
import dev.pocketagent.app.ui.SlashAction
import dev.pocketagent.app.ui.SlashCommandInfo
import dev.pocketagent.app.ui.TurnSession
import dev.pocketagent.app.ui.applySlashPick
import dev.pocketagent.app.ui.attachmentKind
import dev.pocketagent.app.ui.ChatOfflineBanner
import dev.pocketagent.app.ui.copyMessage
import dev.pocketagent.app.ui.filterSlashCandidates
import dev.pocketagent.app.ui.middleEllipsize
import dev.pocketagent.app.ui.slashCandidatesFor
import dev.pocketagent.app.ui.slashPanelVisible
import dev.pocketagent.app.ui.slashUnknownAgentHintVisible
import dev.pocketagent.app.ui.stripSlashToken
import dev.pocketagent.app.ui.voiceEngineLabel
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.pressableLong
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.ContextRing
import dev.pocketagent.app.ui.PaStatusGlyph
import dev.pocketagent.app.ui.component.PaHaptics
import dev.pocketagent.app.ui.component.PaHostChip
import dev.pocketagent.app.ui.component.PaHostLink
import dev.pocketagent.app.ui.component.PaStatusGlyphSlot
import dev.pocketagent.app.ui.component.connectionBannerCopy
import dev.pocketagent.app.ui.component.paEditorialShadow
import dev.pocketagent.app.ui.component.TypingDots
import dev.pocketagent.app.ui.component.composerShowsPrompt
import dev.pocketagent.app.ui.compactionPending
import dev.pocketagent.app.ui.ringPercentLabel
import dev.pocketagent.app.ui.statusGlyphOf
import dev.pocketagent.app.ui.usageTotalTokens
import dev.pocketagent.app.ui.PromptOptimizeState
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaGlassToken
import dev.pocketagent.app.ui.theme.PaHacker
import dev.pocketagent.app.ui.theme.paGlass
import dev.pocketagent.app.ui.voice.VoiceController
import dev.pocketagent.app.ui.voice.VoiceEngineKind
import dev.pocketagent.app.ui.voice.VoiceEvent
import dev.pocketagent.app.ui.voice.VoicePhase
import dev.pocketagent.app.ui.voice.VoiceSessionState
import pocketagent.v1.CodexSettings

/**
 * 底部输入区子树(C1 拆分,2026-10-09):附件 chips / 斜杠命令卡面 / 输入框 /
 * 模型名全宽行 / 工具行(附件/Git/优化/审批策略/上下文环/语音/发送)。
 *
 * 【从 ChatScreen 巨函数整体迁出,行为零变化】子树自有状态(approvalMenu、
 * 聚焦辉光、行数跟踪、文本落定脉动、模型名闪光、麦克风涟漪/波形、micRippleTick)
 * 全部随迁;共享状态(附件列表、各浮层开合、语音模式)以 [MutableState] 直传。
 * ColumnScope 接收者:模型行 `Modifier.weight(1f)` 依赖列作用域;两条骨架
 * (黑客终端 / 编辑式)共用同一份内容。
 *
 * 【P2-10】草稿以 provider(`() -> String`)传入:所有 `input` 读取都发生在本
 * 子树的组合/回调时机 —— 击键只重组本子树,页面级作用域不再订阅 st.draft。
 */
@Composable
internal fun ColumnScope.ChatComposerArea(
    threadId: String,
    session: TurnSessionUiState,
    // 【F8 遗留反馈 3】核对看门狗收尾位(ChatScreen 的 verifyingSettled 直传):
    // 胶囊运行点据此与头部状态点同拍 —— settled 后停呼吸退静态 Muted 档
    // (档位判定走 capsulePulseTier 纯函数,JVM 单测钉死)。
    verifyingSettled: Boolean,
    ctx: Context,
    hapticView: android.view.View,
    input: () -> String,
    setInput: (String) -> Unit,
    attachments: MutableState<List<Attachment>>,
    threadAgentId: String,
    slashCommands: List<SlashCommandInfo>,
    offlineSnapshot: Boolean,
    hostCaps: List<String>,
    slashGoalSheet: MutableState<Boolean>,
    onCompact: () -> Unit,
    onStartReview: (String) -> Unit,
    onOpenAttachment: (Attachment) -> Unit,
    voiceLandedTick: Int,
    voiceListening: Boolean,
    voiceState: VoiceSessionState,
    voiceReplaceMode: MutableState<Boolean>,
    voiceElapsedSec: Int,
    micPermission: ActivityResultLauncher<String>,
    voiceController: VoiceController,
    reportVoiceUnavailable: () -> Unit,
    expandedInputOpen: MutableState<Boolean>,
    modelName: String,
    effortLabel: String,
    showQuickModels: MutableState<Boolean>,
    modelFlashTick: Int,
    sessionAgent: String,
    codexSettings: CodexSettings?,
    onSetApproval: (String) -> Unit,
    windowIsFallback: Boolean,
    contextWindow: Long,
    showContextCard: MutableState<Boolean>,
    showGitPanel: MutableState<Boolean>,
    pickFiles: ActivityResultLauncher<String>,
    promptOptState: PromptOptimizeState,
    onOptimizeInput: () -> Unit,
    onSend: (String) -> Boolean,
    // ---- 【方向 C · N1 上滑沉浸 / A2 海拔】2026-10-09 新参(纯视觉/交互层) ----
    /** true = 沉浸态:输入栏收窄为单行胶囊(点击 [onExpandImmersed] 展开,正文获得全高)。 */
    immersed: Boolean,
    /** 从胶囊展开全尺寸输入区(胶囊整枚是按钮;数据零位移,草稿/附件全在参数里)。 */
    onExpandImmersed: () -> Unit,
    /** 胶囊点开计数:>0 且变化时,展开分支组合后给输入框请求聚焦(点胶囊=我要输入)。 */
    expandFocusTick: Int,
) {
    // 【动画 ①】麦克风按下涟漪触发器(随迁自 ChatScreen:仅输入区消费)
    var micRippleTick by remember { mutableStateOf(0) }
    // 【N1 沉浸两相】单行胶囊(点击展开) ↔ 全尺寸输入区:AnimatedContent 尺寸过渡
    // (spring 可打断);减弱动效直切(instant,反馈仍在位移消失)。胶囊态不组合
    // 完整输入区 —— 输入栏收成一行,正文自然获得全高。草稿/附件/各浮层开合
    // 全部外置(hoist 在 ChatScreen/参数),两相切换零数据位移、零行为变化。
    val reduceMotion = rememberReduceMotion()
    // 【点胶囊=我要输入】展开过渡后给输入框请求聚焦;拿不到节点(过渡中)就放弃,
    // 用户点一下输入框自然聚焦 —— runCatching 兜底,不崩溃不阻塞。
    val composerFocus = remember { FocusRequester() }
    LaunchedEffect(expandFocusTick) {
        if (expandFocusTick > 0) {
            kotlinx.coroutines.delay(120)
            runCatching { composerFocus.requestFocus() }
        }
    }
    AnimatedContent(
        targetState = immersed,
        transitionSpec = {
            if (reduceMotion) {
                fadeIn(PaMotion.instant()).togetherWith(fadeOut(PaMotion.instant()))
            } else {
                (fadeIn(PaMotion.emphasized()) + expandVertically(PaMotion.emphasized()))
                    .togetherWith(fadeOut(PaMotion.standard()) + shrinkVertically(PaMotion.standard()))
            }
        },
        label = "composerImmerse",
    ) { collapsed ->
        if (collapsed) {
            ChatComposerCapsule(
                input = input,
                threadAgentId = threadAgentId,
                // 【F8 遗留反馈 3】运行点档位:不再裸读 `running || recovering`
                // (看门狗收尾后脉冲仍呼吸,与头部状态点回 idle 不一致)—— 收尾
                // 后退静态 Muted 档,判定在 capsulePulseTier 纯函数。
                pulse = capsulePulseTier(session.running, session.recovering, verifyingSettled),
                onExpand = onExpandImmersed,
            )
        } else {
    // ---- 输入区(对标图五):附件行 + 双行胶囊 [输入 / + 审批 模型·强度 发送] ----
        // 已上传附件 chips:出现/收起走 AnimatedContent(展开+淡入 / 收缩+淡出,
        // 全 spring 可打断)—— 编辑条"带附件 ↔ 无附件"两个状态之间不再硬跳。
        AnimatedContent(
            targetState = attachments.value.isNotEmpty(),
            transitionSpec = {
                (fadeIn(PaMotion.emphasized()) + expandVertically(PaMotion.emphasized()))
                    .togetherWith(fadeOut(PaMotion.standard()) + shrinkVertically(PaMotion.standard()))
            },
            label = "attachRow",
        ) { hasAttachments ->
            if (hasAttachments) {
                Column {
            Row(
                Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                attachments.value.forEach { at ->
                    Row(
                        Modifier
                            // 单击预览 / 长按复制完整文件名(复用消息长按复制的写法)
                            .pressableLong(
                                onClick = { onOpenAttachment(at) },
                                onLongPress = { copyMessage(ctx, at.name) },
                            )
                            .background(PaEditorial.Raised, RoundedCornerShape(PaEditorial.Radius.Chip))
                            // PA-REAL-0004 收尾:chip 命中高度抬到 ≥48dp(原 ≈29dp)。
                            // 垂直 padding 7→0,由"移除叉"的 48dp 触控箱顶起;胶囊圆角/
                            // 配色/内容排布不变,仅整体增高到最小触控标准。
                            .heightIn(min = INPUT_TOUCH_TARGET_DP.dp)
                            .padding(horizontal = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(
                            painterResource(
                                if (attachmentKind(at.name, at.mediaType) == AttachmentKind.IMAGE) {
                                    AppR.drawable.ic_lucide_image
                                } else {
                                    AppR.drawable.ic_lucide_file_text
                                },
                            ),
                            null,
                            tint = PaEditorial.Primary, modifier = Modifier.size(13.dp),
                        )
                        Spacer(Modifier.width(6.dp))
                        // 文件名最多 3 行、可换行;宽度封顶以强制换行(横向滚动行里若不封顶,
                        // 文本会一直延伸并被视口裁掉)。真需省略时保留扩展名(见 middleEllipsize)。
                        Text(
                            middleEllipsize(at.name, ATTACH_CHIP_NAME_MAX),
                            Modifier.widthIn(max = 220.dp),
                            fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Ink,
                            maxLines = 3, softWrap = true, overflow = TextOverflow.Ellipsis,
                        )
                        Spacer(Modifier.width(6.dp))
                        // PA-REAL-0004 收尾:移除叉命中区 13→48dp(原叉仅 13dp,chip 本体
                        // 点击是"预览"不能替代移除,故必须独立放大);叉视觉 13dp 居中不变。
                        // 前置 6dp Spacer 保留,触控箱自带居中留白即视觉间距。
                        Box(
                            Modifier
                                .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                                .pressable { attachments.value = attachments.value - at },
                            contentAlignment = Alignment.Center,
                        ) {
                            Icon(
                                painterResource(AppR.drawable.ic_lucide_x), null,
                                tint = PaEditorial.Subtle,
                                modifier = Modifier.size(13.dp),
                            )
                        }
                    }
                }
                }
                    Spacer(Modifier.height(8.dp))
                }
            }
        }

        // 【生效模型名】已上提到函数作用域(effectiveModel 声明处):
        // hacker 模式的 tmux 状态条在输入栏上方,与本栏共用同一个 modelName。

        var approvalMenu by remember { mutableStateOf(false) }
        // 【聚焦辉光】输入框聚焦/失焦经 spring 淡入淡出 1dp 主色描边:
        // "正在输入"有一圈安静的状态光,而不是生硬的有/无两态。
        val inputInteraction = remember { MutableInteractionSource() }
        val inputFocused by inputInteraction.collectIsFocusedAsState()
        val focusGlow by animateFloatAsState(
            targetValue = if (inputFocused) 1f else 0f,
            animationSpec = PaMotion.standard(),
            label = "inputFocusGlow",
        )
        // 编辑式:瓷白输入卡 + 细线(18dp 圆角);黑客终端保持玻璃材质(26dp)。
        val composerShape = if (PaHacker.isHacker) RoundedCornerShape(PaEditorial.Radius.Sheet) else RoundedCornerShape(PaEditorial.Radius.Card)
        // 【A2 输入栏浮起】海拔语言:输入栏从"贴在底栏的格子"变"浮在海面上的操作台"。
        //  · 浅盘:Shadow.Alert 档投影 + 玻璃高光边(PaGlassToken.Highlight,"光打在上缘",
        //    与石墨用户气泡同一亮边家族)—— 悬浮件的既有档位,零新增令牌;
        //  · 深盘:保持细线(M3 暗色"色调面替代阴影"纪律,不加班驳阴影);
        //  · 黑客终端:玻璃材质不变。
        // 【聚焦浮起增强,对齐既有辉光不叠加】投影 blur/dy 随 focusGlow(0..1)增大、
        // 整卡上浮 2dp —— 与下方既有的主色辉光描边同一驱动、同一弹簧,是同一束
        // "聚焦"的两种表达,不是第二层独立光效。
        val composerShell = when {
            PaHacker.isHacker -> Modifier.paGlass(RoundedCornerShape(PaEditorial.Radius.Sheet), hairline = true)
            PaEditorial.IsDark -> Modifier
                .background(PaEditorial.Surface, composerShape)
                .border(1.dp, PaEditorial.Hairline, composerShape)
            else -> {
                val alert = PaEditorial.Shadow.Alert
                val lift = PaEditorial.ShadowSpec(
                    blur = alert.blur * (1f + 0.20f * focusGlow),
                    dy = alert.dy * (1f + 0.20f * focusGlow),
                    color = alert.color,
                )
                Modifier
                    .offset(y = (-2 * focusGlow).dp)
                    .paEditorialShadow(lift, composerShape)
                    .background(PaEditorial.Surface, composerShape)
                    .border(1.dp, PaGlassToken.Highlight, composerShape)
            }
        }
        Column(
            Modifier
                .fillMaxWidth()
                .then(composerShell)
                .border(
                    1.dp,
                    PaEditorial.Primary.copy(alpha = 0.30f * focusGlow),
                    composerShape,
                )
                .animateContentSize(PaMotion.emphasized()),
        ) {
            // [斜杠命令卡面(纵向)] 会话是面板 agent(zcode/codex/pi 三源)且输入以 "/"
            // 开头时,列出该线程可用的命令 + 内置伪命令(zcode=会话快照 slashCommands[],
            // codex=skills 映射,pi=get_commands 真名映射,daemon 按 agent_id 路由同一
            // RPC;内置伪命令由 slashCandidatesFor 置顶合并,/压缩 认 compact:<id>、
            // /计划 认 review:<id> 能力位,缺位不进列表)。点选按动作分流:
            //   - FILL 普通命令 → applySlashPick 回填(zcode/pi→"/命令 " 保留后续文字,
            //     pi 真命令由 pi 原生展开;codex→自然语言触发技能);
            //   - GOAL/COMPACT/REVIEW 内置伪命令 → 执行动作(弹目标底栏 / doCompact /
            //     doStartReview),不填输入框,并把敲出的 "/词元" 清掉
            //     (stripSlashToken,避免把伪命令名当正文发出;REVIEW 的词元后
            //     残留文字作为审查重点 focus 下发)。
            // 视觉:纵向可滑卡面(最大 320dp,每行名称+说明完整不截断)—— 取代旧
            // "横滚 8 条"候选条,解决 codex/pi 长列表"看不全"的问题;
            // 可见性/过滤/合并/回填纯逻辑在 SlashCommandsLogic.kt。
            val slashCandidates = if (slashPanelVisible(threadAgentId, input(), slashCommands, offlineSnapshot, hostCaps)) {
                filterSlashCandidates(slashCandidatesFor(threadAgentId, slashCommands, hostCaps), input())
            } else {
                emptyList()
            }
            if (slashCandidates.isNotEmpty()) {
                Text(
                    "${slashCandidates.size} 个命令 · 上下滑动查看",
                    fontSize = PaEditorial.Type.Micro.fontSize,
                    color = ChatSubtleColor,
                    modifier = Modifier.padding(start = 16.dp, top = 8.dp),
                )
                Column(
                    Modifier
                        .fillMaxWidth()
                        .heightIn(max = minOf(180, LocalConfiguration.current.screenHeightDp / 4).dp)
                        .verticalScroll(rememberScrollState())
                        .padding(start = 10.dp, end = 10.dp, top = 10.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    slashCandidates.forEach { c ->
                        // 每行一张卡:命令名 + 完整说明(不设 maxLines,描述长也看得全)。
                        Column(
                            Modifier
                                .fillMaxWidth()
                                .pressable {
                                    when (c.action) {
                                        SlashAction.GOAL -> {
                                            setInput(stripSlashToken(input()))
                                            slashGoalSheet.value = true
                                        }
                                        SlashAction.COMPACT -> {
                                            setInput(stripSlashToken(input()))
                                            onCompact()
                                        }
                                        SlashAction.REVIEW -> {
                                            // "/计划":词元后已敲的文字作为审查重点
                                            // (通常为空,daemon 有默认口径),清空输入框后发起。
                                            val reviewFocus = stripSlashToken(input())
                                            setInput("")
                                            onStartReview(reviewFocus)
                                        }
                                        SlashAction.FILL -> setInput(applySlashPick(input(), c, threadAgentId))
                                    }
                                }
                                .background(
                                    PaEditorial.Raised,
                                    RoundedCornerShape(PaEditorial.Radius.IconButton),
                                )
                                .padding(horizontal = 12.dp, vertical = 8.dp),
                        ) {
                            Text(
                                "/" + c.name,
                                fontSize = PaEditorial.Type.Caption.fontSize,
                                fontWeight = FontWeight.SemiBold,
                                color = ChatAccentColor,
                            )
                            if (c.description.isNotBlank()) {
                                Spacer(Modifier.height(4.dp))
                                Text(
                                    c.description,
                                    fontSize = PaEditorial.Type.Caption.fontSize,
                                    color = ChatMutedColor,
                                )
                            }
                        }
                    }
                }
            }
            // 【未知助手空态】会话归属 agent 已知但不在三源面板名单(zcode/codex/pi
            // 之外,如旧库残值/将来的第 4 个 agent)且敲出了 "/" → 人话空态,不静默
            // (AUDIT §3.3:遇未知值给人话,禁止"未知/错误"字样)。agentId 还没拉到
            // (加载中)与离线态都不提示,避免闪错/与离线横幅重复;纯逻辑见
            // slashUnknownAgentHintVisible(SlashCommandsLogic.kt)。
            if (slashUnknownAgentHintVisible(threadAgentId, input(), offlineSnapshot)) {
                Text(
                    SLASH_UNKNOWN_AGENT_HINT,
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = ChatSubtleColor,
                    modifier = Modifier.padding(start = 16.dp, top = 8.dp, bottom = 4.dp),
                )
            }
            // 第一行:输入框 + 整个输入栏右上角的"展开编辑"小标识
            // 【为什么放右上角】它是低频辅助动作。放进工具行会和发送键 / 模型 chip
            // 抢注意力,用户明确要求"不要在右侧,也不要在发送键左边"。挪到整个胶囊的
            // 右上角后:不起眼(小号 + TextTertiary)、又始终可达,不占工具行的横向预算。
            // 【动画 ③·文本落框】识别文本并入时行数可能 0→1/1→2:外层 Box 挂
            // animateContentSize,高度变化平滑过渡(轻量,不引入额外布局);叠加
            // BasicTextField 上一次 alpha "落定"脉动,构成"文本飞入输入框"的过渡感。
            val voiceLandPulse = remember { Animatable(1f) }
            LaunchedEffect(voiceLandedTick) {
                if (voiceLandedTick > 0) {
                    voiceLandPulse.snapTo(0.45f)
                    // 落定脉动走 spring 令牌:中途再收到语音片段,从当前透明度续走不跳变
                    voiceLandPulse.animateTo(1f, PaMotion.emphasized())
                }
            }
            Box(Modifier.fillMaxWidth().animateContentSize()) {
                // 【行数跟踪】展开编辑按钮只在文本占超过 2 行时出现 —— 输入区右上角
                // 常驻 ↗ 与工具行"+"一上一下两个浮点,短文本时像两个入口互相打架
                // (L2 真机挑刺);长文本才是它存在的时刻。行数取自真实排版
                // (TextLayoutResult.lineCount,含自动换行),不按字符数猜。
                var inputWrapLines by remember { mutableStateOf(1) }
                BasicTextField(
                    value = input(),
                    onValueChange = { setInput(it) },
                    onTextLayout = { inputWrapLines = it.lineCount },
                    modifier = Modifier
                        .fillMaxWidth()
                        // 【N1 胶囊点开聚焦】FocusRequester 挂点:expandFocusTick 驱动的
                        // 请求经它落到本输入框(见函数头部 composerFocus 效果块)
                        .focusRequester(composerFocus)
                        // 【动画 ③】文本落定脉动:识别文本并入的一瞬整框 45%→100% 透明度
                        // (spring ~0.3s),只动绘制层(alpha),不触发重新布局,开销可忽略
                        .graphicsLayer { alpha = voiceLandPulse.value }
                        // 右侧预留 30dp:保证右上角图标不会压住光标 / 末尾文字
                        .padding(start = 18.dp, end = 30.dp, top = 12.dp, bottom = 12.dp)
                        .heightIn(min = 24.dp),
                    // 黑客终端模式:输入框字体等宽(JetBrains Mono,令牌 PaEditorial.MonoFont)
                    textStyle = TextStyle(
                        fontSize = PaEditorial.Type.Heading.fontSize,
                        color = ChatInkColor,
                        fontFamily = if (PaHacker.isHacker) PaEditorial.MonoFont else null,
                    ),
                    // 聚焦态喂给外层胶囊的辉光描边(见上方 inputFocused)
                    interactionSource = inputInteraction,
                    cursorBrush = SolidColor(ChatAccentColor),
                    maxLines = 4,
                    // 【P1-4】running 不再锁输入(运行中发消息=插入/排队);
                    // 只在离线快照态停用 —— 离线发不出去,别让用户白打一段话。
                    enabled = true,
                    decorationBox = { inner ->
                        // 黑客终端模式:输入非空时行首渲染 `❯ ` 提示符装饰(PaEditorial.Primary);
                        // 空输入显示 hint,不显示 ❯(见 composerShowsPrompt 纯函数)。
                        Row(verticalAlignment = Alignment.Top) {
                            if (PaHacker.isHacker && composerShowsPrompt(input())) {
                                Text(
                                    "❯ ",
                                    color = ChatAccentColor,
                                    fontSize = PaEditorial.Type.Heading.fontSize,
                                    fontWeight = FontWeight.Bold,
                                    fontFamily = PaEditorial.MonoFont,
                                )
                            }
                            Box {
                                if (input().isEmpty()) {
                                    Text(
                                        when {
                                            offlineSnapshot -> "离线中，可继续编辑草稿；重连后发送…"
                                            // 【P4 Task 6】识别进行中:placeholder 优先展示引擎的
                                            // 部分转写(实时反馈);无字时按相给占位 ——
                                            // 停止后的 STOPPING/TRANSCRIBING(端侧转写窗口)
                                            // 显示"转写中…",出稿后回 IDLE 占位符切回常规文案。
                                            voiceListening ->
                                                // 【P4 Task 6】识别中的实时反馈:优先展示引擎部分转写,
                                                // 没有字时按相给占位(停止后 TRANSCRIBING=转写中)。
                                                voiceState.partialText.ifBlank {
                                                    if (voiceState.phase == VoicePhase.TRANSCRIBING ||
                                                        voiceState.phase == VoicePhase.STOPPING
                                                    ) {
                                                        "转写中…"
                                                    } else {
                                                        "语音输入中…"
                                                    }
                                                }
                                            // 黑客终端模式:hint 改为指令口吻(规格文案)
                                            PaHacker.isHacker -> "输入指令…"
                                            else -> "继续输入,或询问当前进度…"
                                        },
                                        fontSize = PaEditorial.Type.Heading.fontSize,
                                        color = ChatSubtleColor,
                                        fontFamily = if (PaHacker.isHacker) PaEditorial.MonoFont else null,
                                    )
                                }
                                inner()
                            }
                        }
                    },
                )
                // 展开编辑:内容超过 EXPAND_HINT_THRESHOLD 时轻微提亮到 PaEditorial.Primary,
                // 保留原有"长文本可展开大窗"的提示语义;平时保持低调(TextTertiary)。
                // 【条件露出】文本 ≤2 行时整块不渲染(见上方 inputWrapLines)——
                // 只在真的需要大窗编辑时出现,输入区右上角不再常驻一个浮点。
                if (inputWrapLines > 2) {
                Box(
                    Modifier
                        .align(Alignment.TopEnd)
                        .padding(end = 10.dp, top = 6.dp)
                        // PA-REAL-0004 收尾:命中区 22→48dp;视觉图标(15dp)用内层
                        // padding 钉回原 22dp 盒中心(距盒右上角 11dp),位置零漂移。
                        .size(INPUT_TOUCH_TARGET_DP.dp)
                        .pressable { expandedInputOpen.value = true },
                    contentAlignment = Alignment.TopEnd,
                ) {
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_maximize_2), "展开编辑",
                        tint = if (input().length > EXPAND_HINT_THRESHOLD) ChatAccentColor else ChatSubtleColor,
                        modifier = Modifier
                            .padding(end = 3.5.dp, top = 3.5.dp)
                            .size(15.dp),
                    )
                }
                }
            }
            // [模型名全宽行] 当前生效的模型名**恒显且完整**:不截断、不限行数。
            // 【为什么改恒显】上一版只在名字超过 14 字符时才补这一行,于是"长度没超、
            // 但已被工具行省略号挤断"的名字依然读不全(用户反馈"模型名仍未完整显示")。
            // 现在无条件显示全文;整行同时内联"运行中模型确认",把原本独立的一行合并进来
            // (设计取向:能一行放下的不要分行),少一层堆叠且不丢信息。整行作模型/强度
            // 点击入口;所有控件触控箱仍走 INPUT_TOUCH_TARGET_DP(PA-REAL-0004)。
            //
            // 【2026-09-30 快速切换】点击整行(含下拉箭头)直接展开**快速切换面板**:
            // 最近用过的模型单点即切,不再强制进 ModelEffortCard 二级弹窗(两段式太重);
            // 面板尾部「更多…」仍进完整选择卡(跟随桌面/全部模型入口不丢)。模型名
            // 11sp→13sp、Muted→Ink:这是高频操作控件,字号与对比度按主控件档给。
            // 【A3 模型/agent 胶囊】身份行从纯文字行升级为胶囊(Pill,PaStatusBadge 族
            // 语言:药丸圆角 100dp + 容器底 Raised + 状态形通道):[agent 图标] +
            // [模型 · 强度] + [状态形通道] + [展开指示]。快速切换面板入口保持在胶囊内
            // (整枚胶囊仍是唯一点击入口,弹层锚点/触控箱 48dp 口径不变)。
            Box(Modifier.padding(start = 10.dp, end = 10.dp, top = 4.dp, bottom = 2.dp)) {
            Row(Modifier.fillMaxWidth().pressable { showQuickModels.value = true }
                .semantics { contentDescription = "切换模型与思考强度" }
                // PA-REAL-0004:整行高度 ≥48dp(口径不变);胶囊底(Raised)吃满 48dp 身。
                .heightIn(min = INPUT_TOUCH_TARGET_DP.dp)
                .background(PaEditorial.Raised, RoundedCornerShape(PaEditorial.Radius.Pill))
                .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.Pill))
                .padding(start = 12.dp, end = 12.dp), verticalAlignment=Alignment.CenterVertically) {
                // agent 品牌图标:身份一眼可辨;未知 agent 不画图标(不塞通用图标冒充身份)
                agentBrandIcon(sessionAgent)?.let { res ->
                    Icon(
                        painterResource(res), null,
                        tint = Color.Unspecified, modifier = Modifier.size(16.dp),
                    )
                    Spacer(Modifier.width(6.dp))
                }
                // 运行中模型确认:真机上"模型 · 强度 · 最近运行: 模型 · 强度"连读
                // 出现"最大"重复三次(用户裁决:信息过载难读)。精简为把"最近运行"
                // 收进快速切换面板头部(那里才是对比"当前 vs 正在跑"的语境)。
                val runningHint = ""
                val base = modelName + if (effortLabel != "默认") " · $effortLabel" else ""
                // 切换成功行内反馈:modelFlashTick(快速面板/完整卡真实选中后 +1)驱动
                // 模型名一次 Primary→Ink 的轻量闪光(纯绘制层动效,不造假状态;
                // 模型名本体由生效链即时更新,闪的只是"刚变过"的提示)。
                val nameFlash = remember { Animatable(0f) }
                LaunchedEffect(modelFlashTick) {
                    if (modelFlashTick > 0) {
                        nameFlash.snapTo(1f)
                        nameFlash.animateTo(0f, tween(650))
                    }
                }
                val nameColor = lerp(ChatInkColor, ChatAccentColor, nameFlash.value)
                Text(
                    buildAnnotatedString {
                        withStyle(SpanStyle(fontSize = PaEditorial.Type.BodySmall.fontSize, color = nameColor, fontWeight = FontWeight.Medium)) {
                            append(base)
                        }
                        if (runningHint.isNotEmpty()) {
                            withStyle(SpanStyle(fontSize = PaEditorial.Type.Caption.fontSize, color = ChatMutedColor)) {
                                append(runningHint)
                            }
                        }
                    },
                    modifier=Modifier.weight(1f),
                    maxLines=2, overflow=TextOverflow.Ellipsis)
                Spacer(Modifier.width(6.dp))
                // 状态形通道(PaStatusBadge 族):待审批=空心环(琥珀,等你)/运行中=
                // Running 呼吸点;空闲不摆 —— 宁缺勿滥,与上下文环同一"不造假"纪律。
                // 词在语义层(读屏描述见整行 semantics),胶囊内只留形+色不挤字。
                when {
                    session.pendingApproval != null ->
                        PaStatusGlyphSlot(PaStatusGlyph.RING, PaEditorial.Attention)
                    // 【F8 遗留反馈 3 同型统一】全尺寸输入区与胶囊共用 capsulePulseTier
                    // 三档:核对看门狗收尾(settled)后不再呼吸 Running,改静态 Muted 点
                    // (与头部状态点已回 idle 一致)。
                    capsulePulseTier(session.running, session.recovering, verifyingSettled) ==
                        CapsulePulseTier.PULSING -> RunningPulseDot()
                    capsulePulseTier(session.running, session.recovering, verifyingSettled) ==
                        CapsulePulseTier.SETTLED ->
                        PaStatusGlyphSlot(PaStatusGlyph.DOT, PaEditorial.Muted)
                    else -> {}
                }
                Spacer(Modifier.width(6.dp))
                Icon(painterResource(AppR.drawable.ic_lucide_chevron_down), "展开快速切换模型", tint=ChatMutedColor,modifier=Modifier.requiredSize(18.dp))
            }
            // 快速切换面板:锚在模型行上方弹出;focusable 让面板外点击/返回键关闭。
            // 【2026-10-02 流畅度修复】面板从行内 Popup 挪到与 showModelCard 同层的
            // 稳定组合位(见文件底部 showModelCard 挂载点)—— 流式输出每帧重组会
            // 反复打断行内 Popup(真机:轮次运行中点模型行面板闪没/开不出,用户
            // 裁决"切换不流畅"的主因)。稳定层挂载不受输入区逐帧重组影响。
            }
            // 第二行:工具行
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(start = 10.dp, end = 8.dp, bottom = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Row(Modifier.weight(1f).horizontalScroll(rememberScrollState()),
                    verticalAlignment=Alignment.CenterVertically) {
                // [+ 附件]
                // 【image_input 位门控】daemon 逐 agent 宣告 image_input:<id>(dsh 无位,
                // 发图必败),附件按钮是手机端唯一的图片入口 —— 缺位会话隐藏,免得
                // 选完图必吃失败壳;带位 agent 不动(见 attachmentEntryHiddenFor)。
                if (!attachmentEntryHiddenFor(threadAgentId, hostCaps)) {
                Box(
                    Modifier
                        // PA-REAL-0004:触控箱 ≥48dp(原 32dp);图标仍 20dp 居中,视觉不变。
                        .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                        .pressable { pickFiles.launch("*/*") },
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_plus), "添加附件",
                        tint = ChatMutedColor, modifier = Modifier.size(20.dp),
                    )
                }
                Spacer(Modifier.width(2.dp))
                }
                // [Git] 会话项目的版本管理侧栏
                Box(
                    Modifier
                        // PA-REAL-0004 收尾:命中区 32→48dp;图标仍 18dp 居中,视觉不变。
                        .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                        .pressable { showGitPanel.value = true },
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_git_branch), "项目 Git",
                        tint = ChatMutedColor, modifier = Modifier.size(18.dp),
                    )
                }
                Spacer(Modifier.width(2.dp))
                // [优化](实验功能,开关在 设置-外观与阅读,默认关):把口语化输入
                // 改写成给 AI 的清晰指令;可见性走 PromptOptimizeState 状态机
                // (开关开 + 在线 + 输入非空 + 不在请求中)。请求中转圈,防重复点击。
                if (promptOptState.buttonVisible || promptOptState.inFlight) {
                    Box(
                        Modifier
                            // PA-REAL-0004 收尾:命中区 32→48dp;转圈/星形图标绘制
                            // 尺寸不变,只放大不可见命中区。
                            .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                            .pressable { onOptimizeInput() },
                        contentAlignment = Alignment.Center,
                    ) {
                        if (promptOptState.inFlight) {
                            TypingDots(dotSize = 3.dp, gap = 2.dp)
                        } else {
                            Icon(
                                painterResource(AppR.drawable.ic_lucide_sparkles), "优化提示词",
                                tint = ChatAccentColor, modifier = Modifier.size(18.dp),
                            )
                        }
                    }
                    Spacer(Modifier.width(2.dp))
                }
                // [审批策略] 会话内无缝切换 —— 扁平可点区域(图标 + 文字 + chevron),
                // 去掉 PaEditorial.Raised 胶囊底色回落到参考图的视觉层级(只借排版,颜色仍走 Pa)。
                // DropdownMenu 的全部逻辑与 APPROVALS 取值映射保持不变。
                if (sessionAgent == "codex") {
                Box {
                    Row(
                        Modifier
                            .pressable { approvalMenu = true }
                            // PA-REAL-0004 收尾:整 chip 命中高度抬到 ≥48dp(原 ≈25dp)。
                            // heightIn 在 padding 外层(同"切换模型"整行口径),内容在
                            // 48dp 内垂直居中,视觉排布不变。
                            .heightIn(min = INPUT_TOUCH_TARGET_DP.dp)
                            .padding(horizontal = 4.dp, vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        val cur = codexSettings?.approvalPolicy ?: "on-request"
                        val danger = cur == "never"
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_triangle_alert), null,
                            tint = if (danger) ChatDeniedColor else ChatAccentColor,
                            modifier = Modifier.size(13.dp),
                        )
                        Spacer(Modifier.width(5.dp))
                        Text(
                            APPROVALS.find { it.first == cur }?.second ?: "按需询问",
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            maxLines=1, overflow=TextOverflow.Ellipsis, modifier=Modifier.widthIn(max=72.dp),
                            color = if (danger) ChatDeniedColor else ChatInkColor,
                        )
                        Spacer(Modifier.width(2.dp))
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_chevron_down), null,
                            tint = ChatSubtleColor, modifier = Modifier.size(12.dp),
                        )
                    }
                    DropdownMenu(
                        expanded = approvalMenu,
                        onDismissRequest = { approvalMenu = false },
                        containerColor = if (PaHacker.isHacker) PaEditorial.Raised else PaEditorial.Surface,
                        shape = RoundedCornerShape(PaEditorial.Radius.Card),
                    ) {
                        APPROVALS.forEach { (id, label, desc) ->
                            val active = (codexSettings?.approvalPolicy ?: "on-request") == id
                            DropdownMenuItem(
                                text = {
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        Column(Modifier.weight(1f)) {
                                            Text(
                                                label, fontSize = PaEditorial.Type.Body.fontSize,
                                                color = if (active) ChatAccentColor else ChatInkColor,
                                            )
                                            Text(desc, fontSize = PaEditorial.Type.Micro.fontSize, color = ChatSubtleColor)
                                        }
                                        if (active) {
                                            Icon(
                                                painterResource(AppR.drawable.ic_lucide_check), null,
                                                tint = ChatAccentColor, modifier = Modifier.size(16.dp),
                                            )
                                        }
                                    }
                                },
                                onClick = {
                                    approvalMenu = false
                                    onSetApproval(id)
                                },
                            )
                        }
                    }
                }
                }
                // [上下文窗口] 常显:圆形占用环 —— 外圈一个圆环,环内直接显示实际使用占比。
                // 【本轮改版】旧版是「圆形图标 + 上下文 128k」的胶囊:横向占位大,而且
                // 看不出窗口被占了多少。现在环内给占比(≥80% 黄、≥95% 红),窗口总量与
                // 用量明细点开卡面看;窗口未知或还没上报用量时环内是"—",绝不假装 0%。
                val ctxWindowKnown = !windowIsFallback && contextWindow > 0
                val ctxUsage = session.turnUsage
                val ctxRingKnown = ctxWindowKnown && ctxUsage != null
                // 压缩同步:压缩后旧占用已失真,小环进入"已压"态而非继续显示旧占比。
                val ctxCompacted = compactionPending(session.turnUsage, session.lastCompactedAt)
                val ctxRatio =
                    if (ctxRingKnown) usageTotalTokens(ctxUsage).toFloat() / contextWindow.toFloat() else 0f
                // 【空值不渲染】窗口未知或还没有用量时报"—"的空环,悬在工具行中间
                // 像一个没有意义的"—"按钮(L2 真机挑刺);没有真值就整块不出,
                // 拿到窗口+用量(或已压缩)才回场 —— 宁可缺席,不摆占位符。
                if (ctxRingKnown || ctxCompacted) {
                Box(
                    Modifier
                        // PA-REAL-0004 收尾:展示型控件的命中区 34→48dp;环视觉仍
                        // 28dp 居中不变(见下方 ContextRing 的 size),只放大命中区。
                        .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                        .semantics {
                            contentDescription = "上下文窗口 " +
                                (if (ctxCompacted) "已压缩" else ringPercentLabel(ctxRatio, ctxRingKnown)) +
                                if (ctxWindowKnown) " (窗口 ${contextWindow / 1000}k)" else ""
                        }
                        .pressable(haptic = true) { showContextCard.value = true },
                    contentAlignment = Alignment.Center,
                ) {
                    ContextRing(
                        ratio = ctxRatio,
                        windowKnown = ctxRingKnown,
                        modifier = Modifier.size(28.dp),
                        compacted = ctxCompacted,
                    )
                }
                Spacer(Modifier.width(2.dp))
                }
                }
                Spacer(Modifier.width(4.dp))
                // 【P4 Task 6 语音输入】麦克风:无权限先申请(RECORD_AUDIO),有权限启动
                // VoiceController 会话(系统识别优先,端侧 SenseVoice 兜底);识别中图标区
                // 换成波形跳动柱;再点一次 = 停止并交付结果。识别文本只并入输入框
                // (默认追加,长按切替换模式),绝不自动发送(保留编辑权)。
                // 【动画 ②·识别中波形】4 根跳动柱替代旧的整体缩放脉动:柱间时长
                // 错开(430/560/500/650ms)避免"同起同落"的机械感;空闲分支不挂
                // InfiniteTransition,不空转。柱高用 scaleY 绘制层缩放,不改布局。
                val micWave: List<Float> = if (voiceListening) {
                    val wave = rememberInfiniteTransition(label = "micWave")
                    listOf(
                        wave.animateFloat(
                            initialValue = 0.35f, targetValue = 1f,
                            animationSpec = infiniteRepeatable(tween(430), RepeatMode.Reverse),
                            label = "wave1",
                        ).value,
                        wave.animateFloat(
                            initialValue = 1f, targetValue = 0.3f,
                            animationSpec = infiniteRepeatable(tween(560), RepeatMode.Reverse),
                            label = "wave2",
                        ).value,
                        wave.animateFloat(
                            initialValue = 0.3f, targetValue = 1f,
                            animationSpec = infiniteRepeatable(tween(500), RepeatMode.Reverse),
                            label = "wave3",
                        ).value,
                        wave.animateFloat(
                            initialValue = 1f, targetValue = 0.4f,
                            animationSpec = infiniteRepeatable(tween(650), RepeatMode.Reverse),
                            label = "wave4",
                        ).value,
                    )
                } else {
                    listOf(1f, 1f, 1f, 1f)
                }
                // 【动画 ①·按下涟漪】点按重放一圈"扩散 + 淡出"的 Accent 圆环:
                // micRippleTick 每次点击 +1,Animatable 0→1 驱动半径与透明度。
                val micRipple = remember { Animatable(0f) }
                LaunchedEffect(micRippleTick) {
                    if (micRippleTick > 0) {
                        micRipple.snapTo(0f)
                        micRipple.animateTo(1f, tween(420))
                    }
                }
                Box(
                    Modifier
                        // PA-REAL-0004:触控箱 ≥48dp(原 40dp);涟漪/波形/图标的绘制尺寸
                        // 都不变,只是不可见的命中区放大。
                        .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                        // 【P4 Task 6】点按 = 开始/停止(按相裁决);长按 = 切换追加/替换模式。
                        .semantics {
                            // TalkBack 单一描述(图标本体不再各挂 contentDescription,避免读两遍):
                            // 空闲 = 模式;进行中 = 时长 + 引擎名 + 停止指引。
                            contentDescription = if (voiceListening) {
                                "语音输入中,已录 $voiceElapsedSec 秒,${voiceEngineLabel(voiceState.engine)};" +
                                    "点按停止并出稿" + if (voiceReplaceMode.value) ",替换模式" else ""
                            } else if (voiceReplaceMode.value) {
                                "语音输入(替换模式):点按开始,长按切回追加模式"
                            } else {
                                "语音输入:点按开始,长按切换替换模式"
                            }
                        }
                        .pressableLong(
                            onClick = {
                                micRippleTick++ // 【动画 ①】按下即起涟漪(不依赖识别结果)
                                val granted = ContextCompat.checkSelfPermission(
                                    ctx, Manifest.permission.RECORD_AUDIO,
                                ) == PackageManager.PERMISSION_GRANTED
                                when (voiceState.phase) {
                                    // 空闲/失败:开新一轮(无权限则只申请,回答后经 Permission 事件续跑)
                                    VoicePhase.IDLE, VoicePhase.ERROR -> {
                                        if (granted && voiceController.current().engine == VoiceEngineKind.UNAVAILABLE) {
                                            // 引擎没建起来(端侧初始化中/失败):如实报因,可自愈重试
                                            reportVoiceUnavailable()
                                        } else {
                                            voiceController.send(VoiceEvent.Start)
                                        }
                                        if (!granted) micPermission.launch(Manifest.permission.RECORD_AUDIO)
                                    }
                                    // 收音中/启动中:停止并交付最终转写
                                    VoicePhase.STARTING, VoicePhase.LISTENING ->
                                        voiceController.send(VoiceEvent.Stop)
                                    // 暂停中:再点一次 = 恢复(同会话继续,文本累计不清)
                                    VoicePhase.PAUSED -> voiceController.send(VoiceEvent.Start)
                                    // 等权限弹窗/停止收尾/端侧转写中:防抖,忽略连点
                                    VoicePhase.REQUESTING_PERMISSION, VoicePhase.STOPPING,
                                    VoicePhase.TRANSCRIBING,
                                    -> {}
                                }
                            },
                            onLongPress = {
                                voiceReplaceMode.value = !voiceReplaceMode.value
                                PaHaptics.light(hapticView)
                                Toast.makeText(
                                    ctx,
                                    if (voiceReplaceMode.value) "语音输入已切到替换模式:识别结果将覆盖输入框" else "语音输入已切到追加模式:识别结果将接在输入框后",
                                    Toast.LENGTH_SHORT,
                                ).show()
                            },
                        ),
                    contentAlignment = Alignment.Center,
                ) {
                    // 涟漪圆环:仅动画进行中绘制(0..1),结束后自然隐藏
                    val ripple = micRipple.value
                    if (ripple > 0f && ripple < 1f) {
                        Box(
                            Modifier
                                .size(32.dp)
                                .graphicsLayer {
                                    scaleX = 0.5f + ripple
                                    scaleY = 0.5f + ripple
                                }
                                .background(ChatAccentColor.copy(alpha = 0.22f * (1f - ripple)), CircleShape),
                        )
                    }
                    if (voiceListening) {
                        // 识别中:波形柱接管图标位(32dp 盒内 4 柱,基线高 16dp)
                        Row(
                            horizontalArrangement = Arrangement.spacedBy(2.5.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            micWave.forEach { v ->
                                Box(
                                    Modifier
                                        .width(2.5.dp)
                                        .height(16.dp)
                                        .graphicsLayer { scaleY = v }
                                        .background(ChatAccentColor, RoundedCornerShape(PaEditorial.Radius.Terminal)),
                                )
                            }
                        }
                    } else {
                        // 替换模式点亮 Accent 色(视觉反馈);读屏描述统一挂在上方
                        // Box 的 semantics(避免读两遍),图标本体不再各挂描述。
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_mic), null,
                            tint = if (voiceReplaceMode.value) ChatAccentColor else ChatMutedColor,
                            modifier = Modifier.size(18.dp),
                        )
                    }
                }
                Spacer(Modifier.width(4.dp))
                // [发送 / 插入]
                // 【P1-4】running 时输入可用,发送键变为「插入运行中任务」:
                // 图标(列表+勾)与箭头区分,内容描述同步,让读屏与视觉都能分清
                // "开新任务"与"插进正在跑的任务(或排队)"。离线快照态一律禁用。
                val sendEnabled = !offlineSnapshot && input().isNotBlank() && !session.recovering
                val inserting = session.running
                // R2 停止可达性:运行中且输入为空 → 发送键原位变「终止」键(与
                // ChatGPT/Codex 移动端同位语义:拇指不动就能停),不再是灰死的箭头。
                // 运行中且输入非空 = P1-4 的「插入」,语义让位给输入;顶栏(黑客终端)
                // 与「更多」(编辑式)的终止入口原样保留作兜底。
                val canStopTurn = session.running && input().isBlank() && !offlineSnapshot
                Box(
                    Modifier
                        // PA-REAL-0004:触控箱 ≥48dp(原 40dp)。Accent 圆底下沉一层保持
                        // 40dp 视觉不变(按下缩放仍作用于整体),只放大不可见命中区。
                        .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                        // 发送键 = 触感分级 confirm 档(发的动作值得重反馈,不再是
                        // 与普通点击同档的轻触);终止键 = warning 档(打断电脑端的
                        // 活,分量比发送更重)。反馈都在触发瞬间发生。
                        .pressable(scaleDown = 0.88f, enabled = sendEnabled || canStopTurn) {
                            if (canStopTurn) {
                                PaHaptics.warning(hapticView)
                                TurnSession.cancelTurn(threadId)
                                Toast.makeText(ctx, "已请求终止电脑端任务", Toast.LENGTH_SHORT).show()
                            } else {
                                PaHaptics.confirm(hapticView)
                                if (onSend(input())) setInput("")
                            }
                        },
                    contentAlignment = Alignment.Center,
                ) {
                    Box(
                        Modifier
                            .requiredSize(40.dp)
                            .background(
                                when {
                                    canStopTurn -> ChatDeniedColor
                                    sendEnabled -> ChatAccentColor
                                    PaHacker.isHacker -> PaEditorial.Hover
                                    else -> PaEditorial.Raised
                                },
                                CircleShape,
                            ),
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(
                            painterResource(
                                when {
                                    canStopTurn -> AppR.drawable.ic_lucide_square
                                    inserting -> AppR.drawable.ic_lucide_list_checks
                                    else -> AppR.drawable.ic_lucide_arrow_up
                                }
                            ),
                            when {
                                canStopTurn -> "终止任务"
                                inserting -> "插入运行中任务"
                                else -> "发送"
                            },
                            // 内容色:sendEnabled 腿坐 Primary 实底,读 OnPrimary 令牌
                            //(黑客盘电光青上白字 1.81:1);canStopTurn 腿坐 Denied
                            // 实底,读 OnDenied 令牌(深/黑客盘=暗红棕 #3A1514,
                            // 白字仅 2.51/2.77:1 已废)。
                            tint = when {
                                canStopTurn -> PaEditorial.OnDenied
                                sendEnabled -> PaEditorial.OnPrimary
                                else -> ChatSubtleColor
                            },
                            modifier = Modifier.size(18.dp),
                        )
                    }
                }
            }
        }
        } // else(全尺寸输入区)分支到此为止;沉浸态胶囊分支见 [ChatComposerCapsule]
    }
}

/**
 * 沉浸态单行输入胶囊(方向 C · N1,播放页 mini 条语言):agent 图标 + 草稿预览
 * (或占位)+ 运行态点 + 展开指示。整枚胶囊是一个按钮(点击展开全尺寸输入
 * 区),不是可编辑输入框 —— 编辑权在全尺寸态,胶囊只做"输入还挂着"的存在感与
 * 入口。海拔语言与全尺寸态同款(A2):浅盘 Alert 档投影 + 玻璃高光边,深盘细线,
 * 黑客终端玻璃。
 *
 * 【F8 遗留反馈 3】运行点走 [capsulePulseTier] 三档:PULSING = Running 呼吸点
 * (现状口径);SETTLED = 核对看门狗收尾后的静态余辉 —— PaStatusGlyph 实心点
 * 走 Muted 中性档,不呼吸、不占 Running 专用纪律槽位(与头部状态点回 idle
 * 同拍);NONE = 不摆。
 */
@Composable
private fun ChatComposerCapsule(
    input: () -> String,
    threadAgentId: String,
    pulse: CapsulePulseTier,
    onExpand: () -> Unit,
) {
    val capsuleShape = RoundedCornerShape(PaEditorial.Radius.Pill)
    val capsuleShell = when {
        PaHacker.isHacker -> Modifier.paGlass(capsuleShape, hairline = true)
        // 深盘:色调面替代阴影的既有纪律,细线描边不加投影
        PaEditorial.IsDark -> Modifier
            .background(PaEditorial.Surface, capsuleShape)
            .border(1.dp, PaEditorial.Hairline, capsuleShape)
        // 浅盘:Alert 档投影 + 玻璃高光边(光打在上缘),胶囊浮在正文之上
        else -> Modifier
            .paEditorialShadow(PaEditorial.Shadow.Alert, capsuleShape)
            .background(PaEditorial.Surface, capsuleShape)
            .border(1.dp, PaGlassToken.Highlight, capsuleShape)
    }
    Row(
        Modifier
            .fillMaxWidth()
            .then(capsuleShell)
            .pressable(haptic = true) { onExpand() }
            .semantics { contentDescription = "展开输入栏" }
            .padding(horizontal = 14.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        agentBrandIcon(threadAgentId)?.let { res ->
            Icon(
                painterResource(res), null,
                tint = Color.Unspecified, modifier = Modifier.size(18.dp),
            )
            Spacer(Modifier.width(8.dp))
        }
        Text(
            input().ifBlank { "继续输入,或询问当前进度…" },
            Modifier.weight(1f),
            fontSize = PaEditorial.Type.BodySmall.fontSize,
            color = if (input().isBlank()) ChatSubtleColor else ChatInkColor,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        when (pulse) {
            CapsulePulseTier.PULSING -> {
                Spacer(Modifier.width(8.dp))
                // 运行态能量点:Running 系呼吸点(铁律见 PaEditorial 头注;减弱动效静态)
                RunningPulseDot()
            }
            CapsulePulseTier.SETTLED -> {
                Spacer(Modifier.width(8.dp))
                // 【F8 遗留反馈 3】看门狗收尾后的静态余辉点:Muted 中性档实心点
                // (形通道同 PaStatusGlyph 族语言,6dp 与呼吸点同径),不呼吸 ——
                // 与头部状态点回 idle 同拍,恢复链没断的诚实余辉。
                PaStatusGlyphSlot(PaStatusGlyph.DOT, PaEditorial.Muted)
            }
            CapsulePulseTier.NONE -> {}
        }
        Spacer(Modifier.width(8.dp))
        Icon(
            painterResource(AppR.drawable.ic_lucide_chevron_up), null,
            tint = ChatMutedColor, modifier = Modifier.size(16.dp),
        )
    }
}

/**
 * 编辑式骨架的主机状态槽(断线件:主机胶囊 OFFLINE + 断线横幅)。
 * 从 ChatScreen 的 chatHostStatus lambda 整体迁出,渲染一字不差。
 */
@Composable
internal fun ChatHostStatusArea(
    connDown: Boolean,
    hostName: String,
    offlineSnapshotAt: Long,
    chatBanner: dev.pocketagent.app.ui.component.ConnectionBannerState,
    onReload: () -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        // 离线(12):主机胶囊 OFFLINE + 上次更新时间 —— 未连接时状态仍显示
        // 上次更新,绝不出现"正在执行"(hostChipCaption 的构造性保证);
        // 断线横幅保留既有机制(自动重连计数/排队条数/手动重试),草稿不丢。
        // 主机胶囊只认"已知断网"(down);横幅可见性走三态状态机 —— checking
        // 断过也保持可见,两者口径不同是有意的(见 ChatScreen 的 chatBanner 注释)。
        if (connDown) {
            PaHostChip(
                hostName = hostName.ifBlank { "电脑" },
                link = PaHostLink.OFFLINE,
                lastUpdateText = offlineSnapshotAt.takeIf { it > 0 }?.let {
                    java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
                        .format(java.util.Date(it))
                },
            )
        }
        if (chatBanner.visible) {
            ChatOfflineBanner(
                copy = connectionBannerCopy(chatBanner.attempts, TurnSession.totalQueuedInputs()),
                onRetry = onReload,
            )
        }
    }
}
