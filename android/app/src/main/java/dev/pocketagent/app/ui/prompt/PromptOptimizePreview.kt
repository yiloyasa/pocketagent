package dev.pocketagent.app.ui.prompt

import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.ui.PromptOptimizeStyle
import dev.pocketagent.app.ui.promptOptimizeStyleHint
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.component.TypingDots
import dev.pocketagent.app.ui.theme.PaEditorial

// PromptOptimizePreview.kt —— 优化预览与差异确认(P4 Task 2)。
//
// 【分层】上半部是纯逻辑(意图 + 归约,JVM 可测;测试见
// PromptOptimizePreviewStateTest),下半部是 PaSheet 弹层。原文与候选文本始终
// 分离(候选要进输入框必须经 Apply/OverwriteModified 显式确认);防覆盖闸以
// currentRevision(输入框修订号)对比 state.sourceRevision 裁决 —— 等待期间
// 用户改过输入就拒绝盲应用,必须二选一,绝不静默覆盖。

/**
 * 预览/候选编辑的字符上限(Task 2:长文本预览有滚动 + 字符上限,不阻塞主
 * 线程)。原文展示、候选编辑都按它截断;diff 的计算量同时被行数上限
 * ([PROMPT_DIFF_DP_CAP])与这里的字符上限双重钳死,remember 缓存后重组零重算。
 */
internal const val PROMPT_PREVIEW_MAX_CHARS = 20_000

/**
 * 预览层的用户意图。归约是防覆盖闸的最终裁决([reducePromptOptimizePreview]);
 * 带"写输入框/再发请求"副作用的意图由 ChatScreen 的分发函数落副作用。
 */
internal sealed interface PromptOptimizePreviewIntent {
    /** 应用候选(revision 未前进时的直应用;已前进会被 reducer 拒绝)。 */
    data object Apply : PromptOptimizePreviewIntent

    /** 冲突二选一的「覆盖」支:等待期间改过输入,仍选择用候选覆盖。 */
    data object OverwriteModified : PromptOptimizePreviewIntent

    /** 失败后重试(同一原文、同一风格再发一次请求)。 */
    data object Retry : PromptOptimizePreviewIntent

    /** 撤销:关闭预览层(输入框恢复由 UndoBar 的 onUndo 回调做,只回本次原文)。 */
    data object Undo : PromptOptimizePreviewIntent

    /** 关闭预览层(放弃候选;请求在飞时调用方顺带取消)。 */
    data object Dismiss : PromptOptimizePreviewIntent

    /** 用户编辑候选文本(应用前可改)。 */
    data class EditCandidate(val text: String) : PromptOptimizePreviewIntent

    /** 换风格(弹层内芯片:空闲时可点,点了即按新风格重发请求)。 */
    data class Restyle(val style: PromptOptimizeStyle) : PromptOptimizePreviewIntent

    /** 一次优化请求发起:记原文/风格/发起时刻的输入框修订号。 */
    data class Started(
        val original: String,
        val style: PromptOptimizeStyle,
        val sourceRevision: Long,
    ) : PromptOptimizePreviewIntent

    /** 请求成功(候选文本已 trim 非空,仓库层保证;这里再做双保险)。 */
    data class Succeeded(val candidate: String) : PromptOptimizePreviewIntent

    /** 请求失败(message 是可直接展示的人话)。 */
    data class Failed(val message: String) : PromptOptimizePreviewIntent
}

/** 弹层是否完全空闲(不可见):无请求在飞、无候选、无错误。 */
internal val PromptOptimizePreviewState.isIdle: Boolean
    get() = !loading && !hasCandidate && error == null

/**
 * 优化点击闸(Task 2 连点防重,照 doFork 的 busy 先例):输入非空且当前没有
 * 在途请求。ChatScreen 的「优化」按钮点击先过这里,在途时直接忽略。
 */
internal fun promptOptimizeClickAllowed(state: PromptOptimizePreviewState, input: String): Boolean =
    input.isNotBlank() && !state.loading

/**
 * 预览状态机归约。[currentRevision] 是"现在"的输入框修订号,只有 Apply 用它
 * 做防覆盖裁决;其余意图不读它(参数统一,方便纯函数单测)。
 *
 * 【三态防覆盖契约】
 *  - 未修改(currentRevision == sourceRevision)→ Apply 直接应用(状态清空);
 *  - 已修改 → Apply 是 no-op(不静默覆盖),UI 呈现「覆盖 / 保留新文本」二选一,
 *    用户显式选 [OverwriteModified] 或 [Dismiss];
 *  - 应用后撤销:Apply/OverwriteModified 把状态清空,原文的恢复是调用方在
 *    应用瞬间捕获的值交 UndoBar 回调 —— 撤销只回**本次**应用前的文本。
 *
 * 【迟到结果】层已关/请求已被新一轮顶掉时(loading == false),Succeeded/
 * Failed 一律丢弃,不复活弹层、不串台。
 */
internal fun reducePromptOptimizePreview(
    state: PromptOptimizePreviewState,
    intent: PromptOptimizePreviewIntent,
    currentRevision: Long,
): PromptOptimizePreviewState = when (intent) {
    is PromptOptimizePreviewIntent.Started -> PromptOptimizePreviewState(
        original = intent.original,
        style = intent.style,
        loading = true,
        sourceRevision = intent.sourceRevision,
    )

    is PromptOptimizePreviewIntent.Succeeded ->
        if (!state.loading) {
            state // 迟到结果(层已关/请求已被顶掉):丢弃,不复活弹层
        } else {
            val candidate = intent.candidate.trim()
            if (candidate.isEmpty()) {
                // 仓库层已拒收空串,这里双保险:空返回当失败,绝不进候选槽位
                state.copy(loading = false, error = "优化结果为空,原文未被改动")
            } else {
                state.copy(candidate = candidate, loading = false, error = null)
            }
        }

    is PromptOptimizePreviewIntent.Failed ->
        if (!state.loading) state else state.copy(loading = false, error = intent.message)

    is PromptOptimizePreviewIntent.EditCandidate ->
        if (state.hasCandidate) state.copy(candidate = intent.text) else state

    is PromptOptimizePreviewIntent.Restyle ->
        if (state.loading) state else state.copy(style = intent.style)

    PromptOptimizePreviewIntent.Apply ->
        if (state.hasCandidate && !state.inputModifiedSince(currentRevision)) {
            PromptOptimizePreviewState() // 应用成功:回到空闲(输入框由调用方写)
        } else {
            state // 无候选可应用,或已修改而未走二选一:拒绝盲应用
        }

    PromptOptimizePreviewIntent.OverwriteModified ->
        if (state.hasCandidate) PromptOptimizePreviewState() else state

    PromptOptimizePreviewIntent.Retry ->
        if (state.loading || state.original.isBlank()) state
        else state.copy(loading = true, error = null)

    PromptOptimizePreviewIntent.Undo, PromptOptimizePreviewIntent.Dismiss -> PromptOptimizePreviewState()
}

/** 风格芯片的短标签(完整一句话说明走 [promptOptimizeStyleHint])。 */
internal fun promptOptimizeStyleShortLabel(style: PromptOptimizeStyle): String = when (style) {
    PromptOptimizeStyle.DEFAULT -> "默认"
    PromptOptimizeStyle.CONCISE -> "精简"
    PromptOptimizeStyle.DETAILED -> "详细"
    PromptOptimizeStyle.IMPLEMENTATION_PLAN -> "计划"
    PromptOptimizeStyle.BUG_REPORT -> "排错"
    PromptOptimizeStyle.TEST_PLAN -> "测试"
    PromptOptimizeStyle.CODE_REVIEW -> "审查"
}

/** 预览展示用截断(原文与 diff 同源,超限尾部加省略标记)。 */
private fun truncateForPreview(text: String): String =
    if (text.length <= PROMPT_PREVIEW_MAX_CHARS) {
        text
    } else {
        text.take(PROMPT_PREVIEW_MAX_CHARS) + "\n…(原文过长,已截断展示)"
    }

/**
 * 优化预览弹层(P4 Task 2):风格芯片(可解释模式)→ 常驻说明「保留原意,
 * 不会自动执行」→ 原文/候选上下对比(原文带行级 diff 高亮,候选可编辑)→
 * 扩写提示(Task 3)→ 应用 / 冲突二选一。
 *
 * 【长文本】原文面板与候选编辑框各自滚动,高度都有上限;diff 以 remember
 * 缓存、行数有 [PROMPT_DIFF_DP_CAP] 兜底,组合内不阻塞主线程。
 */
@Composable
internal fun PromptOptimizePreviewSheet(
    state: PromptOptimizePreviewState,
    currentRevision: Long,
    onIntent: (PromptOptimizePreviewIntent) -> Unit,
) {
    PaSheet(
        onDismissRequest = { onIntent(PromptOptimizePreviewIntent.Dismiss) },
        scrollable = false,
    ) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp)) {
            Text(
                "优化预览",
                fontSize = PaEditorial.Type.Heading.fontSize,
                fontWeight = FontWeight.Bold,
                color = PaEditorial.Ink,
            )
            Spacer(Modifier.height(8.dp))

            // 风格芯片行:请求中禁点(等结果),空闲时可换风格立即重发
            Row(
                Modifier.horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                PromptOptimizeStyle.values().forEach { style ->
                    PromptStyleChip(
                        style = style,
                        selected = style == state.style,
                        enabled = !state.loading,
                        onClick = { onIntent(PromptOptimizePreviewIntent.Restyle(style)) },
                    )
                }
            }
            Spacer(Modifier.height(6.dp))
            Text(
                promptOptimizeStyleHint(state.style),
                fontSize = PaEditorial.Type.Caption.fontSize,
                color = PaEditorial.Muted,
            )
            Spacer(Modifier.height(4.dp))
            // 【常驻说明】Task 3:任何状态下都看得到这条边界
            Text(
                "保留原意,不会自动执行;优化结果需你确认才会替换输入框",
                fontSize = PaEditorial.Type.Caption.fontSize,
                color = PaEditorial.Subtle,
            )
            Spacer(Modifier.height(12.dp))

            when {
                state.loading -> {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        TypingDots(dotSize = 4.dp, gap = 3.dp)
                        Spacer(Modifier.width(10.dp))
                        Text(
                            "正在按「${promptOptimizeStyleShortLabel(state.style)}」风格改写,电脑端生成中…",
                            fontSize = PaEditorial.Type.BodySmall.fontSize,
                            color = PaEditorial.Muted,
                        )
                    }
                    Spacer(Modifier.height(16.dp))
                    PreviewActionButton("关闭", filled = false) {
                        onIntent(PromptOptimizePreviewIntent.Dismiss)
                    }
                }

                state.error != null -> {
                    Text(
                        state.error,
                        fontSize = PaEditorial.Type.BodySmall.fontSize,
                        color = PaEditorial.DeniedText,
                    )
                    Spacer(Modifier.height(12.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        PreviewActionButton("重试", filled = true, modifier = Modifier.weight(1f)) {
                            onIntent(PromptOptimizePreviewIntent.Retry)
                        }
                        PreviewActionButton("关闭", filled = false, modifier = Modifier.weight(1f)) {
                            onIntent(PromptOptimizePreviewIntent.Dismiss)
                        }
                    }
                }

                state.hasCandidate -> {
                    val candidate = state.candidate.orEmpty()
                    val originalShown = remember(state.original) { truncateForPreview(state.original) }
                    // 【diff 高亮】原文面板按行级 diff 渲染:被删/被改的行标红删除线,
                    // 其余原样;新增行只出现在候选面板(原文面板保持原文全貌)。
                    val diffLines = remember(originalShown, candidate) {
                        promptLineDiff(originalShown, candidate.take(PROMPT_PREVIEW_MAX_CHARS))
                    }
                    val originalAnnotation = remember(diffLines) {
                        buildAnnotatedString {
                            diffLines.forEach { line ->
                                when (line.kind) {
                                    PromptDiffKind.REMOVED -> withStyle(
                                        SpanStyle(
                                            background = PaEditorial.DeniedContainer,
                                            color = PaEditorial.DeniedText,
                                            textDecoration = TextDecoration.LineThrough,
                                        ),
                                    ) {
                                        append("− ${line.text}\n")
                                    }
                                    PromptDiffKind.ADDED -> Unit // 原文面板不显示新增行
                                    PromptDiffKind.SAME -> append("${line.text}\n")
                                }
                            }
                        }
                    }
                    val overExpanded = remember(state.original, candidate) {
                        promptCandidateOverExpanded(state.original, candidate)
                    }

                    Text("原文", fontSize = PaEditorial.Type.Caption.fontSize, fontWeight = FontWeight.SemiBold, color = PaEditorial.Muted)
                    Spacer(Modifier.height(4.dp))
                    Box(
                        Modifier
                            .fillMaxWidth()
                            .heightIn(max = 170.dp)
                            .clip(RoundedCornerShape(PaEditorial.Radius.IconButton))
                            .background(PaEditorial.Raised)
                            .verticalScroll(rememberScrollState())
                            .semantics { contentDescription = "优化前的原文,含改动高亮" }
                            .padding(12.dp),
                    ) {
                        Text(originalAnnotation, fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Ink)
                    }

                    Spacer(Modifier.height(10.dp))
                    if (overExpanded) {
                        // 【扩写提示】Task 3:候选比原文大幅变长 → 要求复核
                        Text(
                            "候选比原文明显变长,请检查是否加入了原文没有的内容",
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            fontWeight = FontWeight.Medium,
                            color = PaEditorial.AttentionText,
                        )
                        Spacer(Modifier.height(6.dp))
                    }

                    Text("候选(可编辑)", fontSize = PaEditorial.Type.Caption.fontSize, fontWeight = FontWeight.SemiBold, color = PaEditorial.Muted)
                    Spacer(Modifier.height(4.dp))
                    Box(
                        Modifier
                            .fillMaxWidth()
                            .clip(RoundedCornerShape(PaEditorial.Radius.IconButton))
                            .background(PaEditorial.Surface)
                            .semantics { contentDescription = "优化候选文本,可编辑" },
                    ) {
                        BasicTextField(
                            value = candidate,
                            onValueChange = { onIntent(PromptOptimizePreviewIntent.EditCandidate(it.take(PROMPT_PREVIEW_MAX_CHARS))) },
                            modifier = Modifier
                                .fillMaxWidth()
                                .heightIn(min = 96.dp, max = 220.dp)
                                .padding(12.dp),
                            textStyle = TextStyle(fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Ink),
                        )
                    }

                    Spacer(Modifier.height(12.dp))
                    val modified = state.inputModifiedSince(currentRevision)
                    if (modified) {
                        // 【防覆盖二选一】等待期间用户改过输入框:不静默覆盖
                        Text(
                            "等待期间你修改了输入框,请选择:",
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            color = PaEditorial.AttentionText,
                        )
                        Spacer(Modifier.height(6.dp))
                        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            PreviewActionButton("覆盖新输入", filled = true, modifier = Modifier.weight(1f)) {
                                onIntent(PromptOptimizePreviewIntent.OverwriteModified)
                            }
                            PreviewActionButton("保留新文本", filled = false, modifier = Modifier.weight(1f)) {
                                onIntent(PromptOptimizePreviewIntent.Dismiss)
                            }
                        }
                    } else {
                        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            PreviewActionButton("应用到输入框", filled = true, modifier = Modifier.weight(1f)) {
                                onIntent(PromptOptimizePreviewIntent.Apply)
                            }
                            PreviewActionButton("关闭", filled = false, modifier = Modifier.weight(1f)) {
                                onIntent(PromptOptimizePreviewIntent.Dismiss)
                            }
                        }
                    }
                }
            }
            Spacer(Modifier.height(12.dp))
        }
    }
}

/** 风格胶囊芯片(≥48dp 触控,选中态 PrimaryContainer 底)。 */
@Composable
private fun PromptStyleChip(
    style: PromptOptimizeStyle,
    selected: Boolean,
    enabled: Boolean,
    onClick: () -> Unit,
) {
    Box(
        Modifier
            .heightIn(min = 48.dp)
            .clip(RoundedCornerShape(PaEditorial.Radius.Pill))
            .background(if (selected) PaEditorial.PrimaryContainer else PaEditorial.Raised)
            .pressable(enabled = enabled) { onClick() }
            .padding(horizontal = 14.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            promptOptimizeStyleShortLabel(style),
            fontSize = PaEditorial.Type.BodySmall.fontSize,
            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
            color = if (selected) PaEditorial.AccentText else PaEditorial.Muted,
        )
    }
}

/** 弹层动作按钮(≥48dp 触控;filled = 主操作)。 */
@Composable
private fun PreviewActionButton(
    text: String,
    filled: Boolean,
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
) {
    Box(
        modifier
            .heightIn(min = 48.dp)
            .clip(RoundedCornerShape(PaEditorial.Radius.IconButton))
            .background(if (filled) PaEditorial.Primary else PaEditorial.Raised)
            .pressable { onClick() }
            .padding(horizontal = 16.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text,
            fontSize = PaEditorial.Type.Body.fontSize,
            fontWeight = FontWeight.Medium,
            color = if (filled) PaEditorial.OnPrimary else PaEditorial.Ink,
        )
    }
}
