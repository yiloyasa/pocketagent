package dev.pocketagent.app.ui.chat

import android.content.Context
import android.widget.Toast
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.APPROVALS
import dev.pocketagent.app.ui.ApprovalEntryCard
import dev.pocketagent.app.ui.Attachment
import dev.pocketagent.app.ui.ChatAccentColor
import dev.pocketagent.app.ui.ChatDeniedColor
import dev.pocketagent.app.ui.ChatItem
import dev.pocketagent.app.ui.ChatOfflineBanner
import dev.pocketagent.app.ui.CodexEventCard
import dev.pocketagent.app.ui.EditorialDeliveryBadge
import dev.pocketagent.app.ui.InAppMarkdown
import dev.pocketagent.app.ui.MessageEnterAnimation
import dev.pocketagent.app.ui.PlanPanel
import dev.pocketagent.app.ui.SubAgentsLogic
import dev.pocketagent.app.ui.SubAgentsState
import dev.pocketagent.app.ui.SubAgentThreadMeta
import dev.pocketagent.app.ui.ThinkingCard
import dev.pocketagent.app.ui.TurnSession
import dev.pocketagent.app.ui.AttachmentCard
import dev.pocketagent.app.ui.SubAgentCard
import dev.pocketagent.app.ui.SubAgentGroupCard
import dev.pocketagent.app.ui.SubAgentSourceLine
import dev.pocketagent.app.ui.agentBrandIcon
import dev.pocketagent.app.ui.OfflineBannerOutline
import dev.pocketagent.app.ui.OfflineBannerText
import dev.pocketagent.app.ui.ToolCallCard
import dev.pocketagent.app.ui.askIsAskAction
import dev.pocketagent.app.ui.chatItemContentType
import dev.pocketagent.app.ui.editorialDeliveryCopy
import dev.pocketagent.app.ui.extractPcPaths
import dev.pocketagent.app.ui.humanizeMethodError
import dev.pocketagent.app.ui.rememberAttachmentThumbnail
import dev.pocketagent.app.ui.offlineSnapshotBanner
import dev.pocketagent.app.ui.queueClearVisible
import dev.pocketagent.app.ui.selectedAgentLabel
import dev.pocketagent.app.ui.shouldShowTimelineEmptyState
import dev.pocketagent.app.ui.shouldShowTimelineSkeleton
import dev.pocketagent.app.ui.timelineEmptyStateCopy
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.ConnectionBanner
import dev.pocketagent.app.ui.component.ContextMenuState
import dev.pocketagent.app.ui.component.DeliveryStatusBadge
import dev.pocketagent.app.ui.component.DrawerController
import dev.pocketagent.app.ui.component.HistoryDragState
import dev.pocketagent.app.ui.component.HistoryQuickBubble
import dev.pocketagent.app.ui.component.HistoryQuickNavLogic
import dev.pocketagent.app.ui.component.HistoryTickBar
import dev.pocketagent.app.ui.component.PaHaptics
import dev.pocketagent.app.ui.component.PaSkeletonList
import dev.pocketagent.app.ui.component.paEditorialShadow
import dev.pocketagent.app.ui.component.SkeletonKind
import dev.pocketagent.app.ui.component.TerminalBlockKind
import dev.pocketagent.app.ui.component.TerminalFootnote
import dev.pocketagent.app.ui.component.TerminalOutputBlock
import dev.pocketagent.app.ui.component.TerminalToolBlock
import dev.pocketagent.app.ui.component.TerminalUserBlock
import dev.pocketagent.app.ui.component.TypingDots
import dev.pocketagent.app.ui.component.deliveryRetryable
import dev.pocketagent.app.ui.component.deliveryVisual
import dev.pocketagent.app.ui.component.formatTurnFootnote
import dev.pocketagent.app.ui.component.longPressMenu
import dev.pocketagent.app.ui.component.terminalBlockKind
import dev.pocketagent.app.ui.component.terminalScanlines
import dev.pocketagent.app.ui.isVerifyingRecoveryCopy
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaGlassToken
import dev.pocketagent.app.ui.theme.PaHacker
import dev.pocketagent.app.ui.theme.paGlass
import kotlinx.coroutines.launch
import pocketagent.v1.CodexEvent

/**
 * 连接/加载/恢复提示条的文案装配(e2e-paired-20261009 问题2,纯函数,JVM 单测钉死)。
 *
 * 优先级原样保留:恢复失败 > 恢复中(等待)> 轮次错误 > 页面加载错误。
 * 【核对看门狗收尾】[verifyingSettled]=true 时「正在核对…」这类中间态文案被吞掉
 * (返回 null = 横幅消退)—— daemon 核对信号丢失/延迟时 UI 侧 10s 按本地真相收尾,
 * 不再无限期声称"核对中";恢复失败的 [recoveryError] 与 FAILED 真错误是终局真相,
 * 不参与收尾、照常展示。武装条件(ChatScreen 侧)保证 settled 只会在核对文案族上
 * 置真,这里对 errorMessage 的吞掉再压一道 recovering 在挂的防御。
 */
internal fun recoveryNoticeCopy(
    recoveryError: String?,
    recovering: Boolean,
    verifyingSettled: Boolean,
    errorMessage: String?,
    loadError: String?,
): String? = when {
    recoveryError != null -> recoveryError
    recovering && !verifyingSettled -> errorMessage ?: "连接已断开，正在恢复电脑端任务结果"
    verifyingSettled && isVerifyingRecoveryCopy(errorMessage) -> null
    errorMessage != null -> errorMessage
    loadError != null -> loadError
    else -> null
}

/**
 * 【P0 崩溃修复 2026-10-09】相邻用户消息分组判定(安全读纯函数,JVM 单测钉死)。
 *
 * 竞态背景:打开有历史的会话时 [TurnSession.State.items] 被整表换血
 * (prefillOfflineSnapshot/loadHistoryTail 先 clear 再 addAll,State 也可能被
 * 空闲回收后 getOrPut 重建),而 LazyColumn 可能仍按旧布局给 item 内容 lambda
 * 调度陈旧 index —— P1-6 上提的 itemRolesNow 快照此刻可为空表,裸读
 * `roles[index-1]`/`roles[index+1]` 抛 IndexOutOfBoundsException
 * (真机实崩:Index 46 out of bounds for length 0)。
 * 越界一律视为"无邻居"(false = 不分组),与竞态帧的降级视觉语义一致;
 * 正常帧下标永远在界内,行为与原式逐位相同。
 */
internal fun userGroupNeighbors(roles: List<String>, index: Int): Pair<Boolean, Boolean> =
    (roles.getOrNull(index - 1) == "user") to (roles.getOrNull(index + 1) == "user")

/**
 * 会话中段(时间线区)子树(C1 拆分,2026-10-09):目标行/子代理入口/提示条/
 * 离线横幅/审批入口卡/计划面板/消息列表/流式气泡/骨架空态/历史刻度条/未读胶囊。
 *
 * 【从 ChatScreen 巨函数整体迁出,行为零变化】子树自有状态(displayedStream 节流
 * 快照、prevItemCount 进入动画武装、逐条消息的 selectMode)全部随迁,remember
 * 跟随本组合;页面级共享状态与动作用参数收窄 —— 共享的浮层开合旗标以
 * [MutableState] 直传(读写同一引用,避免 value/setter 成对参数翻倍)。
 * ColumnScope 接收者:黑客终端骨架(原 Column)与编辑式骨架(PaScreenScaffold)
 * 共用本段,内部 `Box(Modifier.weight(1f))` 依赖列作用域。
 */
@Composable
internal fun ColumnScope.ChatTimelineArea(
    threadId: String,
    threadTitle: String,
    threadAgentId: String,
    session: TurnSessionUiState,
    st: TurnSession.State,
    // 【e2e-paired-20261009 问题2】核对看门狗收尾位(ChatScreen 10s 兜底):
    // true = 「正在核对…」中间态已按本地真相收尾 —— 琥珀提示条消退、不再声称
    // 运行中;真失败(recoveryError/FAILED 文案)不受影响,照常展示。
    verifyingSettled: Boolean,
    ctx: Context,
    hapticView: android.view.View,
    drawerCtrl: DrawerController,
    hasFailure: Boolean,
    threadGoalLine: String?,
    loadError: String?,
    offlineSnapshot: Boolean,
    offlineSnapshotAt: Long,
    historyTruncated: Boolean,
    loadingEarlier: Boolean,
    queueClearOn: Boolean,
    uiScale: Float,
    thinkScale: Float,
    listState: LazyListState,
    messageMenu: ContextMenuState,
    messageSelectArmed: MutableState<ChatItem?>,
    peekSub: MutableState<SubAgentThreadMeta?>,
    approvalDialogOpen: MutableState<Boolean>,
    planPanelOpen: MutableState<Boolean>,
    showExport: MutableState<Boolean>,
    showMore: MutableState<Boolean>,
    showHistoryPanel: MutableState<Boolean>,
    unreadBlocks: MutableState<Int>,
    followBottom: MutableState<Boolean>,
    historyEntries: List<HistoryQuickNavLogic.Entry>,
    historyCurrentIndex: Int,
    historyDrag: HistoryDragState,
    historyDayStarts: List<Boolean>,
    onOpenThread: (String) -> Unit,
    onOpenBrowser: (String) -> Unit,
    onOpenSettings: () -> Unit,
    onApproveGuardian: (CodexEvent) -> Unit,
    onRetryDelivery: (ChatItem) -> Unit,
    onOpenAttachment: (Attachment) -> Unit,
    onClearQueued: () -> Unit,
    onLoadEarlier: () -> Unit,
    onReload: () -> Unit,
    onReloadAfterFailure: () -> Unit,
    onRetryNotice: () -> Unit,
    onReloadSnapshot: () -> Unit,
    onJumpTick: (Int) -> Unit,
    onScrubTick: (Int) -> Unit,
    onScrubEnd: () -> Unit,
    onMenuTarget: (ChatItem) -> Unit,
    scope: kotlinx.coroutines.CoroutineScope,
) {
    // 派生标量随迁:最近一次执行计划(时间线 plan 卡取最新,流式草稿兜底)
    val latestPlan = session.latestPlan
    // 中段条块的水平留白:黑客骨架外层无边距,条块自带 14dp;编辑式骨架由
    // PaScreenScaffold 统一给 23dp 页边距,条块不再另加(8px 网格不叠两层)。
    val midGutter = if (PaHacker.isHacker) 14.dp else 0.dp
    if (PaHacker.isHacker) {
    // ---- 顶栏:☰ 侧边栏(会话列表/搜索)+ 标题 + 状态点 ----
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = 14.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        IconActionButton(AppR.drawable.ic_lucide_menu, "菜单", { drawerCtrl.open() }, bg = PaEditorial.Surface)
        Spacer(Modifier.width(14.dp))
        // 【会话页顶部:所用 agent 的图标 + 名称】此前顶栏只有标题,用户看不出
        // "这轮在跟哪个 agent 说话"——尤其四个 agent 能力差异明显(能否派发子
        // 智能体、能否看子代理对话),归属必须一眼可见。图标走 AgentBrand 唯一
        // 映射;未知 agent 不画图标只留名字(不塞通用图标冒充身份)。
        agentBrandIcon(threadAgentId)?.let { res ->
            Icon(
                painterResource(res), null,
                tint = PaEditorial.Primary, modifier = Modifier.size(15.dp),
            )
            Spacer(Modifier.width(5.dp))
        }
        Text(
            selectedAgentLabel(threadAgentId),
            fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted, maxLines = 1,
        )
        Spacer(Modifier.width(8.dp))
        // 【必须显式 Ellipsis】Modifier.weight(1f) 已把宽度约束死,默认
    // TextOverflow.Clip 会"干净地"切掉尾巴且不画省略号,看起来像文字坏了。
    Text(
        threadTitle, Modifier.weight(1f), fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink,
        maxLines = 1, overflow = TextOverflow.Ellipsis,
    )
        // 终止正在跑的轮次(用户反馈:任务跑飞了没法自己停)
        if (session.running) {
            IconActionButton(
                AppR.drawable.ic_lucide_square, "终止任务",
                {
                    // 终止 = warning 档(打断电脑端的活;与发送键原位终止、
                    // 「更多」面板终止同一档 —— 同动作全局同强度)
                    PaHaptics.warning(hapticView)
                    TurnSession.cancelTurn(threadId)
                    Toast.makeText(ctx, "已请求终止电脑端任务", Toast.LENGTH_SHORT).show()
                },
                bg = PaEditorial.Surface, size = 34.dp, iconSize = 16.dp, tint = PaEditorial.Denied,
            )
            Spacer(Modifier.width(8.dp))
        }
        IconActionButton(
            AppR.drawable.ic_lucide_download, "导出对话",
            { showExport.value = true },
            bg = PaEditorial.Surface, size = 34.dp, iconSize = 16.dp,
        )
        Spacer(Modifier.width(8.dp))
        IconActionButton(
            AppR.drawable.ic_lucide_ellipsis, "更多",
            { showMore.value = true },
            bg = PaEditorial.Surface, size = 34.dp, iconSize = 16.dp,
        )
        Spacer(Modifier.width(8.dp))
        Box(
            Modifier
                .size(14.dp)
                .pressable(
                    scaleDown = 0.86f,
                    enabled = hasFailure,
                ) {
                    // 重试链收进回调(ChatScreen 侧持有 chatVm/codexSettings/models/reloadToken)
                    onReloadAfterFailure()
                }
                .padding(3.dp)
                .background(if (hasFailure) PaEditorial.Denied else PaEditorial.Primary, CircleShape),
        )
        Spacer(Modifier.width(14.dp))
    }

    // ---- 断连全局横幅(P1-2):daemon 断连时与主页/会话列表同一条状态 ----
    // 只在 connState==down 时出现(见组件注释的 checking 纪律),重连成功自动消失;
    // 手动重试走本页既有 reloadToken 重载链(横幅自己不实现重连)。
    ConnectionBanner(onRetry = onReload)
    } // PaHacker:旧顶栏 + 断连横幅(黑客终端专属 chrome 到此为止)

    // ---- 会话目标(桌面只读回显,只增):有则显示一行,无则不占位 ----
    // 数据来自 GetThread 的 goal_detail(电脑端 thread/goal/get 现读),
    // 文案由 GoalDisplayLogic 计算;与标题行同边距,单行省略不顶时间线。
    threadGoalLine?.let { goal ->
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = midGutter),
        ) {
            Text(
                goal,
                fontSize = PaEditorial.Type.Caption.fontSize,
                color = PaEditorial.Muted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }

    // ---- 子对话来源行(T-SUB,蓝本 §5.1):当前线程是某主线程的子代理线程时, ----
    // 标题下补一行"来自主对话 · 父标题" + 兄弟 ◀ n/N ▶ 切换钮(降级钮方案,
    // 取舍见 SubAgentCard.kt 文件头)。切兄弟 = 导航 chat/{siblingId} 打开
    // 上一条/下一条子线程,转场走 NavHost 全局动效(与主对话↔子对话同款)。
    // ---- 子代理接入口(规范 §18.1/§18.2):固定在会话页顶部,不随消息滚动 ----
    // 以前它是消息列表里的一个 item:滚两下就找不到,语义上还像"一条消息"。
    // 现在上移到主对话区域**之上**的固定区;顺带落实 §18.2 —— 子代理消息不再
    // 混进主对话流(主对话只呈现自己的消息),子代理只能从这里的入口下钻。
    val subChildren = remember(SubAgentsState.threads, threadId) {
        SubAgentsLogic.childrenOf(SubAgentsState.threads, threadId)
    }
    val subRunIds = remember(session.running, SubAgentsState.lastRefreshedAt) {
        TurnSession.runningThreadIds() // TurnSession 无锁快照,不进 Compose 状态
    }
    // 【出入场】子代理卡区块出现/消失走 AnimatedContent(淡入+轻微上移 / 淡出,
    // 全 spring 可打断):subChildren 出现的瞬间卡片"浮入",清空时平滑让位,
    // 不硬跳占位。内容刷新(数量不变)不重播转场(同一 targetState)。
    AnimatedContent(
        targetState = subChildren.isNotEmpty(),
        transitionSpec = {
            (fadeIn(PaMotion.emphasized()) + slideInVertically(PaMotion.emphasized()) { it / 6 })
                .togetherWith(fadeOut(PaMotion.standard()))
        },
        label = "subAgentEntry",
    ) { present ->
        if (present) {
        Box(Modifier.fillMaxWidth().padding(horizontal = midGutter, vertical = 2.dp)) {
            if (subChildren.size == 1) {
                val only = subChildren[0]
                SubAgentCard(
                    meta = only,
                    state = SubAgentsLogic.runStateOf(only, subRunIds),
                    onOpen = { onOpenThread(only.threadId) },
                    onPeek = { peekSub.value = only },
                )
            } else {
                SubAgentGroupCard(
                    children = subChildren,
                    liveRunIds = subRunIds,
                    onOpen = { meta -> onOpenThread(meta.threadId) },
                    onPeek = { meta -> peekSub.value = meta },
                )
            }
        }
        }
    }

    // 【门控】父 id 明确已知才显示;关系未知(契约缺口)时不占位、不猜。
    val subChildGroup = remember(threadId, SubAgentsState.threads) {
        SubAgentsState.childGroupOf(threadId)
    }
    if (subChildGroup != null) {
        val (subPos, subTotal) = SubAgentsLogic.siblingPos(subChildGroup.siblings, threadId)
        // 父会话是否仍可达(在本库快照里):决定"返回主对话"是可用还是置灰。
        val parentReachable = SubAgentsState.threads.any {
            it.threadId == subChildGroup.parentThreadId
        }
        SubAgentSourceLine(
            parentTitle = subChildGroup.parentTitle,
            pos = subPos,
            total = subTotal,
            onPrev = {
                SubAgentsLogic.siblingNav(subChildGroup.siblings, threadId, -1)?.let(onOpenThread)
            },
            onNext = {
                SubAgentsLogic.siblingNav(subChildGroup.siblings, threadId, +1)?.let(onOpenThread)
            },
            // 规范 §18.3:父会话仍在本库里才给入口;不可达(已删除/未同步)时传 null
            // → 控件置灰,不给点了没反应的假入口。
            onBackToParent = if (parentReachable) {
                { onOpenThread(subChildGroup.parentThreadId) }
            } else {
                null
            },
        )
    }

    // ---- 连接 / 加载 / 恢复提示条 ----
    // 【统一出口】原先这里只渲染 loadError,于是 TurnSession 里那一整套
    // "断线后自动把电脑端任务结果捞回来"的状态(recovering / recoveryError /
    // errorMessage)**在界面上完全不可见**。真机表现就是用户说的:
    // "连接中断后仍显示红色、没有任何等待 UI" —— 他没法区分
    // "正在自动恢复"和"彻底失败了",只能干等。
    //
    // 现在按优先级合并成一条提示,并且等待态带进度指示:
    //   恢复失败 > 恢复中(等待) > 轮次错误 > 页面加载错误
    // 【e2e-paired-20261009 问题2】核对看门狗收尾(verifyingSettled)后,「正在
    // 核对…」中间态横幅在此消退(recoveryNoticeCopy 纯函数,单测钉死);恢复失败
    // 的 recoveryError 与 FAILED 真错误不受影响。
    run {
        val waiting = session.recovering && !verifyingSettled
        val noticeMsg = recoveryNoticeCopy(
            recoveryError = session.recoveryError,
            recovering = session.recovering,
            verifyingSettled = verifyingSettled,
            errorMessage = session.errorMessage,
            loadError = loadError,
        )
        if (noticeMsg != null) {
            // 等待态用琥珀(警示但非失败),彻底失败/加载失败用珊瑚红
            // (编辑式语义色档:ATTENTION / DENIED;黑客终端保持旧黄/红)
            val tone = if (PaHacker.isHacker) {
                if (waiting) PaEditorial.Attention else PaEditorial.Denied
            } else {
                if (waiting) PaEditorial.Attention else PaEditorial.Denied
            }
            // 【人话标题】daemon 报错常把内部方法名当首行("session/resume\n<id> 失败:"),
            // 直接上屏像天书(L2 真机挑刺);标题换人话(恢复会话/发送消息/新建会话),
            // 正文保持错误原文;没有方法名前缀时整条原文照抄(纯函数 humanizeMethodError)。
            val split = humanizeMethodError(noticeMsg)
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = midGutter, vertical = 2.dp)
                    .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .padding(horizontal = 12.dp, vertical = 7.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                // 等待态:转圈告诉用户"它在动",不是卡死了
                if (waiting) {
                    CircularProgressIndicator(
                        Modifier.size(12.dp),
                        strokeWidth = 1.5.dp,
                        color = tone,
                    )
                    Spacer(Modifier.width(8.dp))
                }
                Column(Modifier.weight(1f)) {
                    Text(
                        split?.first ?: noticeMsg,
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        color = if (!waiting && offlineSnapshot) tone.copy(alpha = 0.85f) else tone,
                        // 拆不出方法名时整条原文照抄,保留原 2 行;拆出后标题一行
                        maxLines = if (split == null) 2 else 1,
                    )
                    split?.second?.takeIf { it.isNotBlank() }?.let {
                        Text(
                            it,
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            color = if (!waiting && offlineSnapshot) tone.copy(alpha = 0.85f) else tone,
                            maxLines = 2,
                        )
                    }
                }
                // 自动恢复进行中不给"重试"—— 重复点只会打断它自己的重试节奏。
                // 只有"彻底失败"和"页面加载失败"才需要用户推一把。
                if (!waiting) {
                    // 提示条重试链收进回调(恢复链重试 or 清错误重载,ChatScreen 侧持有)
                    TextButton(onClick = onRetryNotice) {
                        Text(
                            "重试",
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            color = PaEditorial.Primary,
                        )
                    }
                }
            }
        }
    }

    // ---- 离线快照横幅(P1-4 阶段 D:断连进页用本地只读副本渲染)----
    // 与错误提示条分开:它不是"失败",是"明确知道自己在看旧数据"。
    // 输入栏同时停用(见下方输入区),恢复连接后 connectionEpoch 变化自动回到在线。
    if (offlineSnapshot) {
        val banner = offlineSnapshotBanner(offlineSnapshotAt)
        if (banner != null) {
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = midGutter, vertical = 2.dp)
                    .background(PaEditorial.AttentionContainer, RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .border(1.dp, OfflineBannerOutline, RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .padding(horizontal = 12.dp, vertical = 7.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(
                    painterResource(AppR.drawable.ic_lucide_info), null,
                    tint = PaEditorial.Attention, modifier = Modifier.size(13.dp),
                )
                Spacer(Modifier.width(8.dp))
                Text(
                    "$banner · 显示的是电脑端会话的本地副本,输入已停用",
                    Modifier.weight(1f),
                    fontSize = PaEditorial.Type.Caption.fontSize, color = OfflineBannerText, maxLines = 2,
                )
                TextButton(onClick = onReloadSnapshot) {
                Text(
                    "重试",
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = PaEditorial.Primary,
                )
                }
            }
        }
    }

    // ---- 待审批入口卡(05 会话页;README:审批只在弹窗里决策)----
    // 这张卡只是**入口**(点开 PaApprovalDialog),卡上不放任何批准/拒绝按钮;
    // 弹窗关闭(暂不处理)后从这里或顶部身份行再次进入。请求与弹窗相位的状态机
    // 见上方"审批弹窗状态机"块。反问类请求有自己的常驻问题卡,不进这张入口。
    val pendingApprovalEntry = session.pendingApproval
    if (pendingApprovalEntry != null && !askIsAskAction(pendingApprovalEntry.action.name)) {
        ApprovalEntryCard(onOpen = { approvalDialogOpen.value = true })
    }

    // ---- 执行计划面板 ----
    // 【用户诉求】计划不再随时间线滚走:未完成时在会话原位置仍能看到步骤,
    // 跑完之后那张卡不移除,而是把它**提到会话上方**收成一个可展开的面板,
    // 需要回顾时点开,不需要时收起只留一行进度摘要。
    //
    // 【本轮修正 · 真因】这里原先还挂着 `&& AgentConnection.showThinking`。
    // 那是「显示思考过程」的开关,跟交互计划毫无关系 —— 用户一旦把它关掉,
    // 计划面板就跟着一起消失,现象正是"桌面 agent 明明有这个能力,却不显示"。
    // 计划该不该出现只取决于**有没有计划数据**,不该受其它显示开关连坐。
    if (latestPlan.isNotEmpty()) {
        PlanPanel(
            steps = latestPlan,
            // 【P2-13】planStream 以 provider 下沉:草稿只在 PlanPanel 内部(展开区)
            // 订阅,不再让整个时间线区随每个计划 delta 全区重组。
            draft = { if (session.running) st.planStream else "" },
            running = session.running,
            expanded = planPanelOpen.value,
            onToggle = { planPanelOpen.value = !planPanelOpen.value },
            outerGutter = midGutter,
        )
    }

    // ---- 流式正文节流快照(P1-22;【S2 收敛】已改为源头直通)----
    // 历史:delta 帧高频直喂 InAppMarkdown,即便解析已异步化(MarkdownText 的
    // produceState),高频重组 + 解析排队仍会让长流式累计 O(n²) 掉帧,曾在本地
    // 做 ~120ms 合并快照。P1-5 之后 TurnSession 侧已把 delta 收进 streamBuffer,
    // 按 STREAM_RENDER_THROTTLE_MS 的 120ms 口径发布快照 —— st.streamText 本身
    // 就是 120ms 一次的合并结果,本地再 delay 一拍成了最坏 240ms 的双重节流。
    // 现改为**直通读 st.streamText**:保留派生结构(displayedStream 仍是
    // remember(st) 的本地态,下游消费点全部不变),只去掉额外 delay。
    // 【最终帧一致性语义保持】两条不经等待立即追平的路径照旧成立:
    //   1) 流结束(!streaming);
    //   2) 正文被整段转存进 items(TurnSession 在工具调用开始时把 streamText 清空 ——
    //      快照必须同步清空,否则该段会在气泡里残留成"既在历史又在气泡"的重复)。
    // collect 每次发布都重读最新值,任何时刻快照最终必等于 streamText,尾巴不丢。
    var displayedStream by remember(st) { mutableStateOf("") }
    LaunchedEffect(st) {
        snapshotFlow { st.streaming to st.streamText }
            .collect { (running, text) ->
                when {
                    !running || text.isEmpty() -> displayedStream = text
                    text != displayedStream -> displayedStream = text
                }
            }
    }

    // ---- 流式思考节流快照(P1-7;【S2 收敛】与 displayedStream 同款直通)----
    // 旧实现 item 内容直读 st.thinkingText,每个思考 delta 全文重排版一次;P1-5
    // 之后 st.thinkingText 已是 TurnSession 侧 120ms 合并发布的快照,本地不再叠加
    // delay(双重节流收敛同 displayedStream)。「文本清空(思考已落卡)立即追平」
    // 的终局一致性语义保持 —— 落卡的瞬间快照同步清空,卡片不会在时间线上残留或
    // 延迟出现超过一帧。可见性条件读快照(delta 只触发本快照写入,不逐帧重组
    // 整个时间线区)。
    var displayedThinking by remember(st) { mutableStateOf("") }
    LaunchedEffect(st) {
        snapshotFlow { st.thinkingText }
            .collect { text ->
                when {
                    text.isEmpty() -> displayedThinking = text
                    text != displayedThinking -> displayedThinking = text
                }
            }
    }

    // ---- 消息列表(思考过程 / 工具调用都在时间线原位置展示)----
    // 【进入动画的武装记录】记住"上一帧组合时的条目数":仅在"末尾追加了新消息"
    // 的那一帧允许进入动画(见 MessageEnterAnimation)。null = 本页首帧,绝不播
    // (打开会话加载历史时整套静默出现);SideEffect 在组合后登记当前数量,
    // 回滚缩容/加载更早前插都不满足"末尾追加",不会误触发。
    val prevItemCount = remember(threadId) { mutableStateOf<Int?>(null) }
    // 「撤回排队」入口(pi 0.87.1 clear_queue):
    // 【能力位数据源】读 hostCaps 共享快照(与主页网格/设置页同一真源),门控
    // 纯函数 queueClearVisible 锁三重条件 —— 计算随参数 queueClearOn 下放
    // (ChatScreen 侧同式求值),这里不再重复。
    // 按钮只挂在最后一条排队消息上:同一条队列一张入口,不逐条重复
    val lastQueuedIndex = session.lastQueuedIndex
    // 【P1-6】item 内容 lambda 的直读上提:st.items.lastIndex/size/[index±1] 与
    // prevItemCount.value 原先在**每个 item 的内容 lambda 里**读 —— 每个可见 item
    // 的重组作用域都被订阅到整表/这些状态,任何一条消息变化都会让全可见区逐条重算。
    // 现在在列表作用域一次读出(快照进普通局部值),item lambda 只按 index 取用,
    // 不再订阅快照状态。
    val itemCountNow = st.items.size
    val lastItemIndexNow = itemCountNow - 1
    val prevItemCountNow = prevItemCount.value
    val itemRolesNow = ArrayList<String>(itemCountNow).also { roles ->
        for (i in 0 until itemCountNow) roles += st.items[i].role
    }
    // 消息列表容器(Box):列表本体 + P0-1 的「↓ N 条新内容」悬浮胶囊同层,
    // 常规气泡与 hacker 终端块共用(锚定状态与胶囊都只此一份)。
    Box(Modifier.weight(1f)) {
    LazyColumn(
        Modifier
            .fillMaxSize()
            // 黑客终端骨架自带 16dp;编辑式骨架由 PaScreenScaffold 统一 23dp 页边距
            .padding(horizontal = if (PaHacker.isHacker) 16.dp else 0.dp)
            // 黑客终端模式:CRT 扫描线(2dp 周期暗线,alpha 4%,只碰绘制层);
            // 叠在内容之上,常规主题零影响(见 TerminalBlocks.terminalScanlines)。
            .then(if (PaHacker.isHacker) Modifier.terminalScanlines() else Modifier),
        state = listState,
        // 黑客终端模式:块间留白 22dp(Warp 口径:留白>分割线);常规主题 14dp 不动
        verticalArrangement = Arrangement.spacedBy(if (PaHacker.isHacker) 22.dp else 14.dp),
        contentPadding = PaddingValues(vertical = 12.dp),
    ) {
        // 【F-D 加载更早】历史被截断时在时间线顶部给出的增量入口;离线快照
        // (只读旧数据)不显示。点击后重拉更大的轮数并保持滚动锚点,见 loadEarlier。
        if (historyTruncated && !offlineSnapshot) {
            item(key = "load-earlier") {
                Box(
                    Modifier.fillMaxWidth().padding(vertical = 2.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    if (loadingEarlier) {
                        CircularProgressIndicator(modifier = Modifier.size(18.dp), strokeWidth = 2.dp)
                    } else {
                        TextButton(onClick = onLoadEarlier) {
                            Text(
                                "加载更早",
                                fontSize = PaEditorial.Type.BodySmall.fontSize,
                                color = PaEditorial.Primary,
                            )
                        }
                    }
                }
            }
        }
        // 【PF4】contentType 按渲染形态分桶(见 chatItemContentType):滚动回收只
        // 同形态复用,不同形态不再互毁重建。
        // 【C2 稳定 key】ChatItem 构造处统一领了全进程唯一 id(见 ChatItem.id):
        // 历史前插(addAll(0,…))时既有条目 key 不位移,可见区 item 不再整体
        // 重执行;漏领 id 的旧路径(空串)按位置回退,不会重复 key 崩列表。
        itemsIndexed(
            st.items,
            key = { index, item -> item.id.ifEmpty { "pos-$index" } },
            contentType = { _, item -> chatItemContentType(PaHacker.isHacker, item) },
        ) { index, item ->
            // 进入动画只武装"新消息追加"的那一帧(见上方 prevItemCount 注释)
            // 【P1-6】三个直读全部换用列表作用域快照(见 itemCountNow 注释)
            // 【P0 修复 2026-10-09】items 为空(打开历史会话的整表换血/State 重建空窗)
            // 时绝不武装进入动画 —— 此帧的 index 是旧布局残留,动画与新内容无关;
            // 正常帧 itemCountNow>0 恒成立,行为零变化。
            val enterArm = prevItemCountNow != null &&
                itemCountNow > 0 &&
                index == lastItemIndexNow &&
                itemCountNow > prevItemCountNow
            // R2-A:本条消息的「选择文本」模式。长按默认开菜单(见用户/回复气泡的
            // longPressMenu),选一段文字改为从菜单进入 —— 手柄拖选能力不丢,但不再
            // 与菜单抢同一个长按手势。滑出屏幕即复位(remember 跟随 item 生命周期)。
            var selectMode by remember { mutableStateOf(false) }
            LaunchedEffect(messageSelectArmed.value) {
                // 比引用不比值:内容完全相同的两条消息只命中被长按的那一条
                if (messageSelectArmed.value === item) {
                    selectMode = true
                    messageSelectArmed.value = null
                }
            }
            MessageEnterAnimation(arm = enterArm) {
                // C3 内存上限截断的折叠标记行:细行提示「已折叠 N 条早期消息」,
                // 两种主题同一形态,不参与进入动画语义(它出现在中部而非尾部)。
                if (item.role == "items_trimmed") {
                    Row(
                        Modifier.fillMaxWidth().padding(vertical = 1.dp),
                        horizontalArrangement = Arrangement.Center,
                    ) {
                        Text(
                            "⋯ 已折叠 ${item.trimmedCount} 条早期消息",
                            fontSize = PaEditorial.Type.Micro.fontSize,
                            color = PaEditorial.Subtle,
                        )
                    }
                } else if (PaHacker.isHacker) {
                // 黑客终端模式:主对话读作"终端会话"——四类 Block(输入/输出/工具/脚注),
                // 映射走纯函数 terminalBlockKind(JVM 单测钉住);无终端形态的 item
                // (thinking/codex_event)保持既有渲染,plan/usage 照旧不进时间线。
                when (terminalBlockKind(item)) {
                    TerminalBlockKind.USER -> TerminalUserBlock(
                        item, uiScale,
                        // 「撤回排队」入口与常规主题同一门控:只挂最后一条排队消息
                        queueClearOn = queueClearOn && index == lastQueuedIndex,
                        onClearQueued = onClearQueued,
                        // P0-2:daemon 拒收(overflow)时整条 Block 可点击重发,
                        // 与常规气泡同一 retryDelivery(失败级才武装,见组件内判定)
                        onRetryDelivery = { onRetryDelivery(item) },
                        attachmentsContent = if (item.attachments.isEmpty()) null else { {
                            item.attachments.chunked(2).forEach { rowAttachments ->
                                Row(
                                    Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                ) {
                                    rowAttachments.forEach { attachment ->
                                        AttachmentCard(
                                            attachment = attachment,
                                            // 【P0-2/P1-8】key 直接用 previewData 引用(equals=引用
                                            // 比较)—— 旧 key `previewData?.contentHashCode()` 在
                                            // 每次重组的键求值里对整图(1-5MB)做哈希;两段式
                                            // ~128px 解码挪进 produceState/IO(见 rememberAttachmentThumbnail)。
                                            preview = rememberAttachmentThumbnail(ctx, attachment),
                                            modifier = Modifier.weight(1f),
                                            onClick = { onOpenAttachment(attachment) },
                                        )
                                    }
                                    if (rowAttachments.size == 1) {
                                        Spacer(Modifier.weight(1f))
                                    }
                                }
                            }
                        } },
                    )
                    TerminalBlockKind.TOOL -> TerminalToolBlock(item, uiScale)
                    TerminalBlockKind.OUTPUT -> TerminalOutputBlock(
                        item.text, uiScale,
                        onOpenUrl = onOpenBrowser,
                    )
                    TerminalBlockKind.KEEP_EXISTING -> when (item.role) {
                        "thinking" -> if (AgentConnection.showThinking) {
                            ThinkingCard(item.text, thinkScale, streaming = false)
                        } else Unit
                        "codex_event" -> item.codexEvent?.let { ev ->
                            CodexEventCard(
                                event = ev,
                                uiScale = uiScale,
                                guardianOverridden = session.guardianOverridden.contains(ev.reviewId),
                                onApproveGuardian = { onApproveGuardian(it) },
                                onOpenSettings = onOpenSettings,
                            )
                        }
                        else -> Unit
                    }
                    TerminalBlockKind.SKIP -> Unit
                }
                } else when (item.role) {
                "user" -> {
                    // 【D3-2 同 author 连发分组】相邻两条用户消息视觉上并成一组
                    // (ChatGPT/Claude 的气泡语言):后续消息上提 40% 间距(14dp→8.4dp),
                    // 相接两侧的圆角收小(7dp)读作"同一段话的延续"。纯视觉:offset
                    // 只动摆放位置,LazyColumn 的排列/锚定/回收行为零变化,黑客终端
                    // 块(22dp 大留白的终端语言)不参与分组。
                    // 【P1-6】邻居角色改读列表作用域快照(itemRolesNow),不再订阅整表
                    // 【P0 崩溃修复 2026-10-09】原裸读 itemRolesNow[index±1] 在
                    // "items 整表换血 × LazyColumn 旧布局调度陈旧 index"竞态下越界
                    // (真机实崩:Index 46 out of bounds for length 0,见
                    // userGroupNeighbors 注释)。改安全读:越界降级为"无邻居"。
                    val (prevIsUser, nextIsUser) = userGroupNeighbors(itemRolesNow, index)
                    val bubbleShape = if (PaHacker.isHacker) {
                        RoundedCornerShape(PaEditorial.Radius.Terminal)
                    } else {
                        RoundedCornerShape(
                            topStart = if (prevIsUser) 7.dp else 20.dp,
                            topEnd = 7.dp,
                            bottomEnd = if (nextIsUser) 7.dp else 20.dp,
                            bottomStart = if (nextIsUser) 7.dp else 20.dp,
                        )
                    }
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .offset(y = if (prevIsUser && !PaHacker.isHacker) (-5.6).dp else 0.dp),
                        horizontalArrangement = Arrangement.End,
                    ) {
                    // 编辑式(05):用户消息 = 石墨重点面气泡 + 白字,投递徽标挂在气泡
                    // 下方右对齐(待发送→已送达→执行中;失败保留内容可重试);
                    // 黑客终端保持旧气泡(AccentDim 终端块)与原徽标位(气泡内)。
                    Column(Modifier.widthIn(max = 320.dp), horizontalAlignment = Alignment.End) {
                    Column(
                        Modifier
                            .widthIn(max = 300.dp)
                            // R2-A:长按气泡 = 上下文菜单(复制全文/选择文本/引用回复,
                            // 失败级再加「重试发送」)。P0-2 失败级(队列满拒收)的
                            // "点按重发"保留为单击动作(longPressMenu 契约:挂了它就
                            // 不再叠加 clickable,单击经 onClick 传入);其余投递状态
                            // 点了没有可重试的东西,不武装点按。
                            .longPressMenu(
                                messageMenu,
                                onClick = if (deliveryRetryable(item.deliverState) && !offlineSnapshot) {
                                    { onRetryDelivery(item) }
                                } else null,
                            )
                                // 编辑式:石墨重点面(#1E2835)承载用户输入,与瓷白助手正文
                                // 拉开层级;黑客终端模式:圆角收 6dp 读"终端块"(契约见 PaHacker)。
                                .background(
                                    if (PaHacker.isHacker) PaEditorial.PrimaryContainer else PaEditorial.Graphite,
                                    bubbleShape,
                                )
                                // D3-2 石墨气泡 0.5dp 玻璃高光边:同一块玻璃家族的亮边
                                // 语言(PaGlassToken.Highlight,"光打在上缘"),实底石墨
                                // 从"贴纸"变"材质"。黑客终端块保持旧语言。
                                .then(
                                    if (PaHacker.isHacker) Modifier
                                    else Modifier.border(0.5.dp, PaGlassToken.Highlight, bubbleShape),
                                )
                                .padding(horizontal = 13.dp, vertical = 11.dp),
                            verticalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            item.attachments.chunked(2).forEach { rowAttachments ->
                                Row(
                                    Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                ) {
                                    rowAttachments.forEach { attachment ->
                                        AttachmentCard(
                                            attachment = attachment,
                                            // 【P0-2/P1-8】同上:key 用 previewData 引用,解码走
                                            // produceState/IO 的两段式缩略图
                                            preview = rememberAttachmentThumbnail(ctx, attachment),
                                            modifier = Modifier.weight(1f),
                                            onClick = { onOpenAttachment(attachment) },
                                        )
                                    }
                                    if (rowAttachments.size == 1) {
                                        Spacer(Modifier.weight(1f))
                                    }
                                }
                            }
                            if (item.text.isNotBlank()) {
                                // R2-A:长按默认开上下文菜单;要拖选一段从菜单进
                                // 「选择文本」(与回复气泡同一交互,见 assistant 分支注释)
                                if (selectMode) {
                                    SelectionContainer {
                                        Text(
                                            item.text,
                                            color = if (PaHacker.isHacker) PaEditorial.Ink else PaEditorial.OnGraphite,
                                            fontSize = (14 * uiScale).sp,
                                            lineHeight = (21 * uiScale).sp,
                                            // 黑客终端模式:等宽字体是风格的本体(契约见 PaHacker)。
                                            fontFamily = if (PaHacker.isHacker) PaEditorial.MonoFont else null,
                                        )
                                    }
                                } else {
                                    Text(
                                        item.text,
                                        color = if (PaHacker.isHacker) PaEditorial.Ink else PaEditorial.OnGraphite,
                                        fontSize = (14 * uiScale).sp,
                                        lineHeight = (21 * uiScale).sp,
                                        // 黑客终端模式:等宽字体是风格的本体(契约见 PaHacker)。
                                        fontFamily = if (PaHacker.isHacker) PaEditorial.MonoFont else null,
                                    )
                                }
                                if (selectMode) {
                                    Text(
                                        "完成选择",
                                        Modifier
                                            .pressable { selectMode = false }
                                            .padding(top = 4.dp),
                                        fontSize = PaEditorial.Type.Micro.fontSize,
                                        fontWeight = FontWeight.SemiBold,
                                        color = ChatAccentColor,
                                    )
                                }
                            }
                            // 黑客终端模式:投递徽标留在气泡内(旧位置,行为不变);
                            // 编辑式徽标移到气泡下方(见外层 Column 尾部)。
                            if (PaHacker.isHacker && item.deliverState.isNotEmpty()) {
                                val visual = deliveryVisual(item.deliverState, item.queuePos)
                                if (visual.label.isNotEmpty()) {
                                    Spacer(Modifier.height(2.dp))
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        DeliveryStatusBadge(item.deliverState, item.queuePos)
                                    }
                                }
                            }
                        }
                        // 编辑式投递徽标 + 「撤回排队」(四态措辞对齐设计:待发送→已送达→
                        // 执行中;失败保留内容可重试)。QueueFlushed 后标签被 TurnSession
                        // 收掉,这条消息自动变回普通用户消息。
                        if (!PaHacker.isHacker && item.deliverState.isNotEmpty()) {
                            val copy = editorialDeliveryCopy(item.deliverState, item.queuePos)
                            if (copy.word.isNotEmpty()) {
                                Spacer(Modifier.height(3.dp))
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    EditorialDeliveryBadge(item.deliverState, item.queuePos)
                                    // 「撤回排队」:只挂在最后一条排队消息的回执行上,
                                    // 门控见 queueClearOn 的注释 —— 撤回是对
                                    // 还没执行的输入做反悔,与"终止任务"同级的
                                    // 干预动作,用红字让分量看得见。
                                    if (queueClearOn && index == lastQueuedIndex) {
                                        Text(
                                            "撤回排队",
                                            Modifier
                                                .pressable(haptic = true) { onClearQueued() }
                                                .padding(horizontal = 8.dp, vertical = 2.dp),
                                            fontSize = PaEditorial.Type.Micro.fontSize,
                                            color = ChatDeniedColor,
                                        )
                                    }
                                }
                            }
                        }
                    }
                }
                }
                // 思考过程:可折叠卡片(字号跟随"思考显示大小"设置);
                // 受全局"显示思考过程"开关控制,关闭时只看正文与结果
                "thinking" -> if (AgentConnection.showThinking) {
                    ThinkingCard(item.text, thinkScale, streaming = false)
                } else Unit
                // 执行计划:已提到会话上方的可折叠面板里(见 PlanPanel),
                // 时间线里不再重复一张卡 —— 避免同一步骤在屏幕上有两份。
                "plan" -> Unit
                // 工具调用:一次调用一条卡,含参数/输出/耗时,可展开看详情
                "tool" -> ToolCallCard(item, uiScale)
                // 后台 Codex/MCP 事件只影响右上角状态点，不再把诊断卡片
                // 连续插入会话正文。
                // 【本轮修正】以前这里是一句 `-> Unit`,把所有 codex 事件
                // 一律丢弃 —— 连"审查者驳回了动作"这种用户必须知情(而且可以
                // 推翻)的事件也一起丢了。现在按 shouldShowCodexEventInTimeline
                // 的分类渲染:诊断类在源头就不进 items,进 items 的都要看得见。
                "codex_event" -> item.codexEvent?.let { ev ->
                    CodexEventCard(
                        event = ev,
                        uiScale = uiScale,
                        guardianOverridden = session.guardianOverridden.contains(ev.reviewId),
                        onApproveGuardian = { onApproveGuardian(it) },
                        onOpenSettings = onOpenSettings,
                    )
                }
                "usage" -> Unit
                else -> Column(
                    Modifier
                        .fillMaxWidth()
                        // R2-A:长按回复 = 上下文菜单(复制全文/选择文本/引用回复)。
                        // 【与旧交互的衔接】此前正文套常驻 SelectionContainer,长按
                        // 直接进拖选 —— "长按复制整条"因此被移除过一次(手势互抢)。
                        // 现在长按优先开菜单,拖选改为菜单「选择文本」进入的显式模式
                        // (见本 item 顶部 selectMode):两个能力都在,不再互抢手势,
                        // 与 ChatGPT/Claude 官方 App 的消息菜单同构。
                            .longPressMenu(messageMenu) { onMenuTarget(item) },
                ) {
                    // 选择模式:手柄拖选任意片段,系统工具栏给"复制/全选";
                    // 平时渲染普通正文,长按交给菜单。「完成选择」退出。
                    if (selectMode) {
                        SelectionContainer {
                            InAppMarkdown(
                                text = item.text,
                                modifier = Modifier.fillMaxWidth(),
                                fontSize = (if (PaHacker.isHacker) 15f else 14f) * uiScale,
                                onOpenUrl = onOpenBrowser,
                            )
                        }
                    } else {
                        InAppMarkdown(
                            text = item.text,
                            modifier = Modifier.fillMaxWidth(),
                            fontSize = (if (PaHacker.isHacker) 15f else 14f) * uiScale,
                            onOpenUrl = onOpenBrowser,
                        )
                    }
                    if (selectMode) {
                        Text(
                            "完成选择",
                            Modifier
                                .pressable { selectMode = false }
                                .padding(top = 4.dp),
                            fontSize = PaEditorial.Type.Micro.fontSize,
                            fontWeight = FontWeight.SemiBold,
                            color = ChatAccentColor,
                        )
                    }
                    // 消息里如果给了电脑上的文件路径,唯一的动作是"用浏览器打开"
                    // —— 会话正文只负责展示输出;要存到手机,在浏览器顶栏点"下载到手机"。
                    val pcFiles = remember(item.text) { extractPcPaths(item.text) }
                    if (pcFiles.isNotEmpty()) {
                        pcFiles.take(3).forEach { path ->
                            val fileName = path.substringAfterLast('/').substringAfterLast('\\')
                            Row(
                                Modifier.padding(top = 6.dp).fillMaxWidth(),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Row(
                                    Modifier.pressable(haptic = true) {
                                        onOpenBrowser("pc:" + path)
                                    },
                                    verticalAlignment = Alignment.CenterVertically,
                                ) {
                                    Icon(
                                        painterResource(AppR.drawable.ic_lucide_globe), null,
                                        tint = ChatAccentColor, modifier = Modifier.size(13.dp),
                                    )
                                    Spacer(Modifier.width(5.dp))
                                    Text("在浏览器打开 $fileName", fontSize = PaEditorial.Type.Micro.fontSize, color = ChatAccentColor)
                                }
                            }
                        }
                    }
                }
                }
            }
        }
        // 流式思考:边想边展示,和落定的思考卡同款样式
        // 【P1-7】直读 st.thinkingText 换成 120ms 节流快照 displayedThinking:
        // 逐 delta 全文重排版收敛为每 120ms 一次;落卡清空的瞬间快照立即追平,
        // 卡片消失时机与旧实现同帧。
        if (st.running && AgentConnection.showThinking && displayedThinking.isNotEmpty()) {
            item(key = "streaming-thinking") {
                ThinkingCard(displayedThinking, thinkScale, streaming = true)
            }
        }
        // 流式执行计划:已由上方 PlanPanel 承担(它同时兼做收起的进度条),
        // 这里不再重复渲染,否则同一步骤会出现两份。
        if (session.streaming) {
            item(key = "streaming-bubble") {
                if (PaHacker.isHacker) {
                // 黑客终端模式:流式输出块(块尾 braille 转轮,80ms/帧);
                // 等首 token 时正文为空,TerminalOutputBlock 只渲染转轮本身。
                // 【节流(审查 open 项 7,2026-10-04 评估收口)】TerminalOutputBlock
                // 正文并非纯文本:内部走 MarkdownText → produceState(key=source)
                // 对每次 source 变化全文重解析 parseMarkdownForDisplay。直喂
                // st.streamText = 每个 delta 帧一次全文重解析(解析虽在
                // Dispatchers.Default、且有取消合并,长流式仍是累计 O(n²) 的
                // CPU 热点)。改喂上方 displayedStream(120ms 合并快照,P1-22
                // 同一数据源),与非黑客路径同一节流口径;终帧一致性由该快照的
                // 「!streaming 立即追平」语义兜底,尾巴不丢。
                TerminalOutputBlock(
                    displayedStream, uiScale,
                    onOpenUrl = onOpenBrowser,
                    streaming = true,
                )
                } else {
                // 【流式两相切换】等首 token(三点) → 正文流式:AnimatedContent +
                // spring 过渡(淡入+轻微上移,可打断)。AnimatedContent 自带尺寸
                // 过渡,相位切换时高度不跳变;正文帧间更新不重播转场(同一 targetState)。
                // 相位跟节流快照走(displayedStream 而非 streamText):首帧正文
                // 就绪前保持"三点"相位,不会闪一帧空正文。
                AnimatedContent(
                    targetState = displayedStream.isEmpty(),
                    transitionSpec = {
                        (fadeIn(PaMotion.emphasized()) +
                            slideInVertically(PaMotion.emphasized()) { it / 8 })
                            .togetherWith(fadeOut(PaMotion.standard()))
                    },
                    label = "streamPhase",
                ) { waiting ->
                    if (waiting) {
                        // 等待首个 token:三点跳动"正在输入"指示(替代旧的光标闪烁)。
                        // 编辑式:瓷白卡面+细线(旧玻璃材质保留给黑客终端以外的主题切换
                        // 会打架,这里按模式取面);三点自身只动 graphicsLayer(见 TypingDots)。
                        Row(Modifier.fillMaxWidth()) {
                            Box(
                                Modifier
                                    .then(
                                        if (PaHacker.isHacker) Modifier.paGlass(RoundedCornerShape(PaEditorial.Radius.Card))
                                        else Modifier
                                            .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
                                            .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.Card)),
                                    )
                                    .padding(horizontal = 16.dp, vertical = 14.dp),
                            ) {
                                TypingDots()
                            }
                        }
                    } else {
                        // 流式输出中:正文 + 底部 Running 系能量细线跟随
                        // 【A1 流式能量线(2026-10-09 方向 C)】"正在生成"视觉从跟随三点
                        // 升级为能量细线:色只走 PaEditorial.Running(运行态能量色=
                        // Running 系专用纪律,见 PaEditorial 头注铁律 2;主色不混入),
                        // 呼吸走 PaMotion.pulse,减弱动效静态 —— 见 StreamingEnergyLine。
                        // 首 token 前的"三点"相位(TypingDots 卡)保留:零正文时细线
                        // 单独撑不住"在思考"的语义,两相语言(等三点/流中细线)分工。
                        // 【P1-22】喂节流快照 displayedStream,不再逐 delta 帧直喂全文
                        // 【D4-1】streaming=true:新长出来的块播 ~140ms fade-in
                        // (MarkdownStreamingFade,方案与节流管线的关系见其注释)
                        Column(Modifier.fillMaxWidth()) {
                            InAppMarkdown(
                                text = displayedStream,
                                modifier = Modifier.fillMaxWidth(),
                                fontSize = (if (PaHacker.isHacker) 15f else 14f) * uiScale,
                                onOpenUrl = onOpenBrowser,
                                streaming = true,
                            )
                            StreamingEnergyLine(Modifier.padding(top = 8.dp))
                        }
                    }
                }
                }
            }
        }
        // 黑客终端模式:回合脚注(④)—— 轮次终局后挂在列表末尾右对齐。
        // 【不造数据】只在"有真实终局知识"时出现:运行中/恢复中绝不显示;
        // 有失败信息给 ✗,有真实 token 用量给 ✓ N tok,什么都没有只给 ✓。
        if (PaHacker.isHacker && !session.running && !session.recovering && session.recoveryError == null &&
            (session.turnUsage != null || session.errorMessage != null)
        ) {
            item(key = "turn-footnote") {
                TerminalFootnote(
                    formatTurnFootnote(
                        success = session.errorMessage == null,
                        elapsedMs = 0.0, // 回合时长无现成数据,不编造
                        outputTokens = session.turnUsage?.outputTokens ?: 0,
                    ),
                )
            }
        }
    }
    // 【R1 收口】hermes 等会话首次进入的两条"空白路"补齐(时间线 Box 的同层
    // 覆盖层,不进 LazyColumn —— 头部 item 会让既有的 historyListOffset 滚动
    // 锚定差一位;而骨架/空态只在 st.items 为空时出现,覆盖层无遮挡问题,
    // 列表与覆盖层在同一帧原子切换,不会闪叠):
    //  · 历史在途 → 消息气泡骨架(与真实气泡同轮廓,切内容不跳变;整块折叠为
    //    单条「正在加载」读屏播报);
    //  · 历史加载完且确实为空 → 空态(事实 + 下一步),不发愣。
    // 判定走纯函数(shouldShowTimelineSkeleton / shouldShowTimelineEmptyState,
    // TimelineSkeletonEmptyTest 钉死):报错横幅、离线副本、运行/恢复中都不摆。
    if (shouldShowTimelineSkeleton(
            itemsEmpty = session.itemsEmpty,
            historyLoaded = session.historyLoaded,
            hasLoadError = loadError != null,
            offlineSnapshot = offlineSnapshot,
        )
    ) {
        // 气泡实际高度 ≈82dp,按它推条数,切到真实内容时高度不跳
        PaSkeletonList(SkeletonKind.MESSAGE_BUBBLE, perRowHeight = 82.dp)
    }
    if (shouldShowTimelineEmptyState(
            itemsEmpty = session.itemsEmpty,
            historyLoaded = session.historyLoaded,
            hasLoadError = loadError != null,
            offlineSnapshot = offlineSnapshot,
            running = session.running,
            recovering = session.recovering,
        )
    ) {
        val (emptyHead, emptySub) = timelineEmptyStateCopy(selectedAgentLabel(threadAgentId))
        Column(
            Modifier.fillMaxWidth().padding(top = 96.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Icon(
                painterResource(AppR.drawable.ic_lucide_message_square), null,
                tint = PaEditorial.Subtle, modifier = Modifier.size(28.dp),
            )
            Spacer(Modifier.height(10.dp))
            Text(
                emptyHead,
                fontSize = PaEditorial.Type.Body.fontSize, lineHeight = 20.sp,
                fontWeight = FontWeight.SemiBold,
                color = PaEditorial.Muted,
            )
            Spacer(Modifier.height(4.dp))
            Text(
                emptySub,
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 18.sp,
                color = PaEditorial.Subtle,
                textAlign = TextAlign.Center,
            )
        }
    }
        // 历史消息快速查看(视频样板二版+R1 实时刷选):左缘垂直刻度条 —— 每条
        // 用户消息一个刻度点(Canvas 一次绘制);轻点开完整列表浮层;按住拖动 =
        // 预览气泡跟手 + **列表实时滚动到手指所指**(拖到哪看到哪),松手精确锚定。
        HistoryTickBar(
            count = historyEntries.size,
            currentIndex = historyCurrentIndex,
            drag = historyDrag,
            onOpenPanel = { showHistoryPanel.value = true },
            onJump = { entryIndex -> onJumpTick(entryIndex) },
            onScrub = { entryIndex -> onScrubTick(entryIndex) },
            onScrubEnd = onScrubEnd,
            dayStarts = historyDayStarts,
            modifier = Modifier
                .align(Alignment.CenterStart)
                // 【真机修正 2026-10-01】本 Box 已被 PaScreenScaffold 收进 23dp
                // 页边距内,直接 align 会把刻度条推进正文列(实测 bounds x=63px
                // 起,正压在助手文本上)——负 offset 拉回屏幕真实左缘,视觉
                // 10dp 全程悬在边距区,这才是不挡字的硬保证。
                .offset(x = -PaEditorial.Space.PageMargin),
        )
        // 按住拖动的紧凑预览气泡:只在预览期间存在(scale+fade 出入场),
        // 松手即收 —— 平时零遮挡。align TopStart 让气泡 Y 与刻度条同坐标系;
        // 同样负 offset 出页边距,内部 16dp 间隙才是"距屏幕左缘 16dp"。
        HistoryQuickBubble(
            drag = historyDrag,
            entries = historyEntries,
            modifier = Modifier
                .align(Alignment.TopStart)
                .offset(x = -PaEditorial.Space.PageMargin),
        )
        // P0-1「↓ N 条新内容」胶囊:仅在用户离底且确有未读新块时浮现(PaEditorial.Primary 底)。
        // 点击平滑滚回底部、恢复跟随并清零未读(轻触觉由 pressable 自带)——
        // 流式输出从此不再把上滑阅读历史的用户拽回来。
        // 【全限定名】这里在 Column 作用域内,ColumnScope.AnimatedVisibility 扩展
        // 会抢走隐式接收者解析;盒子层要的是顶层函数,写全限定名绕开歧义。
        androidx.compose.animation.AnimatedVisibility(
            visible = unreadBlocks.value > 0 && !followBottom.value,
            modifier = Modifier
                .align(Alignment.BottomEnd)
                .padding(end = 20.dp, bottom = 12.dp),
            enter = fadeIn(PaMotion.emphasized()) +
                slideInVertically(PaMotion.emphasized()) { it / 2 },
            exit = fadeOut(PaMotion.standard()),
            label = "unreadPill",
        ) {
            Row(
                Modifier
                    .pressable(haptic = true) {
                        scope.launch {
                            followBottom.value = true
                            unreadBlocks.value = 0
                            val total = session.itemCount + if (session.streaming) 1 else 0
                            if (total > 0) listState.animateScrollToItem(total - 1)
                        }
                    }
                    // D3-1 浅盘悬浮胶囊海拔:Card 档阴影让它"浮"在时间线之上;
                    // 深色盘(含黑客)保持既有平语言,不加班驳阴影
                    .then(
                        if (PaEditorial.IsDark) Modifier
                        else Modifier.paEditorialShadow(PaEditorial.Shadow.Card, RoundedCornerShape(PaEditorial.Radius.IconButton)),
                    )
                    .background(ChatAccentColor, RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .padding(horizontal = 12.dp, vertical = 7.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(
                    painterResource(AppR.drawable.ic_lucide_chevron_down), null,
                    tint = PaEditorial.OnPrimary, modifier = Modifier.size(12.dp),
                )
                Spacer(Modifier.width(4.dp))
                Text(
                    "${unreadBlocks.value.coerceAtLeast(1)} 条新内容",
                    color = PaEditorial.OnPrimary,
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    fontWeight = FontWeight.Medium,
                )
            }
        }
    }
    // 组合完成后登记本帧条目数(下一帧据此判定"是否末尾追加了新消息")
    SideEffect { prevItemCount.value = st.items.size }
}
