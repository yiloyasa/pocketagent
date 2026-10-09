package dev.pocketagent.app.ui.component

import androidx.compose.animation.EnterTransition
import androidx.compose.animation.ExitTransition
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.PaDuotoneMotif
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressableLong
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.theme.EditorialTone
import dev.pocketagent.app.ui.theme.PaEditorial

// ---------------------------------------------------------------- 错误翻译映射表(纯函数,单测钉死)

/**
 * 错误的语义分级(展示层翻译的唯一依据)。
 *
 * 【为什么只有三级】盘点的实锤是"`UNAUTHENTICATED: token 无效` 原始文案裸露 6+ 处、
 * 横幅样式不一、无'去配对'出口"。人话翻译只需要分清三件事:
 *  1. [PaErrorKind.UNAUTHENTICATED] 还没和电脑配对 —— 用户**做一步就能解决**
 *     (去配对),琥珀提示而非大红块(审计 23/25:深色下暗红大色块视觉重量过大);
 *  2. [PaErrorKind.CONNECTIVITY] 连不上电脑 —— 网络/通道问题,给「查看排查」与重试;
 *  3. [PaErrorKind.DAEMON] 电脑端报的其他错 —— **透传原文但加人话头**
 *     (「电脑端说：…」),不翻译也不吞(错误翻译是展示层,原始错误全文仍保留在
 *     [PaErrorView.raw],进「详情」展开)。
 *
 * 【判定纪律】大小写不敏感的关键词命中;未认证词表**从严**(只收配对/鉴权语境),
 * `PERMISSION_DENIED` 刻意**不**入表 —— 文件/Git 操作里它多为电脑端文件系统权限,
 * 翻成"还没配对"是误导,按 DAEMON 透传更诚实。
 */
enum class PaErrorKind {
    /** 未配对/鉴权失效:去配对就能解决。 */
    UNAUTHENTICATED,

    /** 连不上电脑:超时/通道/网络类。 */
    CONNECTIVITY,

    /** 其他:电脑端原文透传(加「电脑端说：」人话头)。 */
    DAEMON,
}

/**
 * 错误横幅/错误卡的建议动作(纯展示语义;**能不能摆按钮由调用方决定**)。
 *
 * 【宁缺勿假】[PaErrorBar]/[PaErrorCard] 的对应回调为 null 时动作按钮整个不渲染
 * —— 没有导航通道就不摆"点了没反应"的死按钮(与本 app「不摆假入口」纪律同源)。
 */
enum class PaErrorAction {
    /** 去配对:导航到连接与配对页。 */
    GO_PAIR,

    /** 查看排查:打开排障指南/连接诊断。 */
    TROUBLESHOOT,

    /** 无建议动作(靠重试槽位或纯透传)。 */
    NONE,
}

/**
 * 翻译后的错误展示值。
 *
 * @property kind     语义分级(映射表产出)
 * @property headline 人话一句话(页面上直接读的)
 * @property raw      原始错误全文(诊断信息,**永不丢弃**:进「详情」展开/日志)
 * @property action   建议动作(调用方按自己的导航通道决定是否摆出)
 * @property tone     语义色档(琥珀=有明确下一步;珊瑚=失败)
 */
data class PaErrorView(
    val kind: PaErrorKind,
    val headline: String,
    val raw: String,
    val action: PaErrorAction = PaErrorAction.NONE,
    val tone: EditorialTone = EditorialTone.DENIED,
)

/** 未配对/鉴权失效词表(小写比对;从严,见 [PaErrorKind] 头注)。 */
private val UnauthenticatedNeedles = listOf(
    "unauthenticated",
    "token 无效",
    "请先配对",
    "未配对",
    "配对已失效",
    "未鉴权",
)

/** 连不上电脑词表(小写比对;"connect"/"连接" 一个词覆盖 refused/失败/不上/断开一族)。 */
private val ConnectivityNeedles = listOf(
    "timeout",
    "timed out",
    "超时",
    "unavailable",
    "deadline_exceeded",
    "connect",
    "连接",
    "econnrefused",
    "socket",
    "unknownhost",
    "unable to resolve",
    "网络",
    "断开",
    "channel",
)

/** 人话头:DAEMON 级透传时的前缀(保留原文,只加说话人)。 */
const val PA_ERROR_DAEMON_PREFIX = "电脑端说："

/** DAEMON 级 headline 里原文的缩略上限(全文仍在 raw/详情)。 */
internal const val PA_ERROR_HEADLINE_MAX = 48

/**
 * 原始错误 → 语义分级(纯函数,映射表全表单测钉死)。
 *
 * 【顺序即优先级】先判未配对、再判连不上 —— 未认证词不含连通词,但
 * 「UNAVAILABLE: token 无效」这类混合报错会同时命中两表,未配对优先
 * (用户的下一步是配对,不是排查网络)。
 */
fun paErrorKindOf(raw: String): PaErrorKind {
    val lower = raw.lowercase()
    return when {
        UnauthenticatedNeedles.any { lower.contains(it) } -> PaErrorKind.UNAUTHENTICATED
        ConnectivityNeedles.any { lower.contains(it) } -> PaErrorKind.CONNECTIVITY
        else -> PaErrorKind.DAEMON
    }
}

/** 动作 → 按钮文案(纯函数);NONE 无文案。 */
fun paErrorActionLabelOf(action: PaErrorAction): String? = when (action) {
    PaErrorAction.GO_PAIR -> "去配对"
    PaErrorAction.TROUBLESHOOT -> "查看排查"
    PaErrorAction.NONE -> null
}

/** DAEMON 级的人话 headline:压缩空白 + 截断(全文不丢,进 raw)。 */
internal fun paDaemonHeadlineOf(raw: String): String {
    val flat = raw.replace(Regex("\\s+"), " ").trim()
    if (flat.isEmpty()) return "电脑端没有说明原因"
    return if (flat.length <= PA_ERROR_HEADLINE_MAX) {
        PA_ERROR_DAEMON_PREFIX + flat
    } else {
        PA_ERROR_DAEMON_PREFIX + flat.take(PA_ERROR_HEADLINE_MAX) + "…"
    }
}

/**
 * 原始错误 → 翻译后的展示值(纯函数;全表单测钉死)。
 *
 * 【展示层纪律】本函数只做"怎么说",不做"记不记":raw 原样带出,调用方
 * ([PaErrorBar]/[PaErrorCard])把它放进「详情」展开 —— 诊断信息一条不丢。
 */
fun paErrorViewOf(raw: String): PaErrorView {
    val kind = paErrorKindOf(raw)
    return when (kind) {
        PaErrorKind.UNAUTHENTICATED -> PaErrorView(
            kind = kind,
            headline = "还没和电脑配对",
            raw = raw,
            action = PaErrorAction.GO_PAIR,
            tone = EditorialTone.ATTENTION,
        )
        PaErrorKind.CONNECTIVITY -> PaErrorView(
            kind = kind,
            headline = "连不上电脑",
            raw = raw,
            action = PaErrorAction.TROUBLESHOOT,
            tone = EditorialTone.DENIED,
        )
        PaErrorKind.DAEMON -> PaErrorView(
            kind = kind,
            headline = paDaemonHeadlineOf(raw),
            raw = raw,
            action = PaErrorAction.NONE,
            tone = EditorialTone.DENIED,
        )
    }
}

/**
 * 同屏错误判重(纯函数):两条原始错误是否**同一根源**(同一语义分级)。
 *
 * 【为什么按 kind 判重】盘点实锤"同一屏错误信息重复出现最多 4 次"—— 未配对时
 * 会话列表/电脑 codex/Pi/ZCode 四个来源各报一条 UNAUTHENTICATED,内容同根,
 * 逐条横幅就是把一句话喊四遍。主错误(调用方排第一的那个)已展示时,
 * 同 kind 的其余来源不再各摆一条。任一入参为空 = 无从判重,返回 false(展示)。
 */
fun paIsSameErrorKind(a: String?, b: String?): Boolean {
    if (a.isNullOrBlank() || b.isNullOrBlank()) return false
    return paErrorKindOf(a) == paErrorKindOf(b)
}

// ---------------------------------------------------------------- 空态语境 → duotone 母题(纯函数,单测钉死)

/**
 * 空态语境(空态"为什么空"的一句话归类)。
 *
 * 【语境不符宁缺】duotone 母题只有三个,只配语境真正相符的空态;其余空态
 * 保持单色 lucide 图标([PaDuotoneMotif] 不配 = EmptyState 的 motif=null 分支),
 * 不为统一而硬套。
 */
enum class PaEmptyContext {
    /** 发现/连接语境:空是因为"还没找到/连上电脑"。 */
    DISCOVER,

    /** 配对语境:空是因为"还没配对"。 */
    PAIR,

    /** 审批语境:空是因为"没有等你审批/处理的任务"。 */
    APPROVAL,

    /** 无相符语境:不配图形。 */
    NONE,
}

/**
 * 空态语境 → duotone 母题(纯函数;EmptyState(motif=) 的唯一取值口,
 * 调用点不再各自手挑图形)。
 */
fun paEmptyStateMotifOf(context: PaEmptyContext): PaDuotoneMotif? = when (context) {
    PaEmptyContext.DISCOVER -> PaDuotoneMotif.DISCOVER_RING
    PaEmptyContext.PAIR -> PaDuotoneMotif.PAIR
    PaEmptyContext.APPROVAL -> PaDuotoneMotif.APPROVAL
    PaEmptyContext.NONE -> null
}

// ---------------------------------------------------------------- 组件

/**
 * 页内错误横幅(错误态统一组件之一;区块级用 [PaErrorCard])。
 *
 * 【语义分级】headline 一律来自映射表([paErrorViewOf])或调用方自带的已人话化
 * 文案(如离线快照「离线快照 · HH:mm」—— 那不是 daemon 错误,不走映射表,
 * 由调用方自组 [PaErrorView])。**原始错误永不丢弃**:整条横幅点按/长按展开
 * 「详情」,raw 原文全文可见。
 *
 * 【动作位 · 宁缺勿假】[onGoPair]/[onTroubleshoot] 为 null 时对应按钮不渲染
 * —— 没有导航通道就不摆死按钮。[onRetry] 同理;[retrying]=true 时重试位变成
 * 不可点的「进行中…」(防抖可见反馈,与会话页既有重试同语言)。
 *
 * 【样式】语义色容器([EditorialTone.container])+ 同色小字档 + 同色描边,
 * 13dp 圆角横幅(与被替换的各页裸横幅同规格,颜色/文案/动作三处归一);
 * 读屏合并为一条 contentDescription(语义不单靠颜色)。
 *
 * @param view     翻译后的错误展示值(纯函数产出)
 * @param onRetry  重试回调;null = 不摆重试
 * @param retrying 重试进行中(重试位变文案、不可点)
 * @param retryLabel 重试位文案(默认「重试」;可传「重新扫描」等语境词)
 * @param onGoPair 「去配对」回调;null = 不摆(调用方无配对页导航通道时)
 * @param onTroubleshoot 「查看排查」回调;null = 不摆
 */
@Composable
fun PaErrorBar(
    view: PaErrorView,
    modifier: Modifier = Modifier,
    onRetry: (() -> Unit)? = null,
    retrying: Boolean = false,
    retryLabel: String = "重试",
    onGoPair: (() -> Unit)? = null,
    onTroubleshoot: (() -> Unit)? = null,
) {
    var detailShown by remember(view.raw) { mutableStateOf(false) }
    val reduceMotion = rememberReduceMotion()
    val tone = view.tone
    val shape = RoundedCornerShape(PaEditorial.Radius.IconButton)
    Column(
        modifier
            .fillMaxWidth()
            .pressableLong(
                onClick = { detailShown = !detailShown },
                onLongPress = { detailShown = !detailShown },
            )
            .background(tone.container, shape)
            .border(1.dp, tone.accent.copy(alpha = 0.30f), shape)
            .padding(horizontal = 12.dp, vertical = 4.dp)
            .semantics { contentDescription = "${view.headline}。详情：${view.raw}" },
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(
                painterResource(R.drawable.ic_lucide_triangle_alert), null,
                tint = tone.accent, modifier = Modifier.size(15.dp),
            )
            Spacer(Modifier.width(8.dp))
            Text(
                view.headline,
                Modifier.weight(1f),
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
                fontWeight = FontWeight.SemiBold,
                color = tone.text,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            // 动作位:按分级给下一步;回调缺失则整个不摆(宁缺勿假)。
            val actionLabel = paErrorActionLabelOf(view.action)
            val actionHandler = when (view.action) {
                PaErrorAction.GO_PAIR -> onGoPair
                PaErrorAction.TROUBLESHOOT -> onTroubleshoot
                PaErrorAction.NONE -> null
            }
            if (actionLabel != null && actionHandler != null) {
                Text(
                    actionLabel,
                    Modifier
                        .heightIn(min = PaEditorial.Space.TouchMin)
                        .pressableLong(
                            onClick = { actionHandler() },
                            onLongPress = { detailShown = !detailShown },
                        )
                        .padding(horizontal = 8.dp, vertical = 12.dp),
                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                    fontWeight = FontWeight.SemiBold,
                    // 主色只给当前操作(下一步动作)
                    color = PaEditorial.Primary,
                )
            }
            if (onRetry != null) {
                Text(
                    if (retrying) "进行中…" else retryLabel,
                    Modifier
                        .heightIn(min = PaEditorial.Space.TouchMin)
                        .pressableLong(
                            onClick = { if (!retrying) onRetry() },
                            onLongPress = { detailShown = !detailShown },
                        )
                        .padding(horizontal = 8.dp, vertical = 12.dp),
                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                    fontWeight = FontWeight.SemiBold,
                    color = if (retrying) PaEditorial.Subtle else PaEditorial.Primary,
                )
            }
        }
        // 详情(原始错误全文):点按/长按横幅切换;诊断信息只收起、不丢弃。
        AnimatedVisibility(
            visible = detailShown,
            enter = if (reduceMotion) EnterTransition.None else {
                expandVertically(PaMotion.standard()) + fadeIn(PaMotion.standard())
            },
            exit = if (reduceMotion) ExitTransition.None else {
                shrinkVertically(PaMotion.standard()) + fadeOut(PaMotion.standard())
            },
        ) {
            Text(
                view.raw,
                Modifier
                    .fillMaxWidth()
                    .padding(top = 2.dp, bottom = 8.dp),
                fontSize = PaEditorial.Type.Micro.fontSize, lineHeight = 15.sp,
                fontFamily = PaEditorial.MonoFont,
                color = tone.text,
            )
        }
    }
}

/**
 * 区块级错误卡(错误态统一组件之二;页内横幅用 [PaErrorBar])。
 *
 * 与 [PaErrorBar] 同一套映射表/详情纪律,差别只有三处:
 *  1. 卡片形态(18dp 圆角、12dp 内距)—— 嵌在列表/面板内部的错误用它;
 *  2. [context] 槽:错误来源一行小字(如「电脑会话」「Git」),多来源同屏时
 *     用户分得清哪条是谁的;
 *  3. 重试在卡内为居中整行动作(与 PaApprovalDialog 的「重试」行同款语言)。
 *
 * 【同屏判重】同根源多来源的场景由调用方用 [paIsSameErrorKind] 先行判重,
 * 组件不做去重(它不知道同屏还有什么)。
 *
 * @param context 错误来源说明;null/空 = 不摆来源行
 * (其余参数同 [PaErrorBar])
 */
@Composable
fun PaErrorCard(
    view: PaErrorView,
    modifier: Modifier = Modifier,
    context: String? = null,
    onRetry: (() -> Unit)? = null,
    retrying: Boolean = false,
    retryLabel: String = "重试",
    onGoPair: (() -> Unit)? = null,
    onTroubleshoot: (() -> Unit)? = null,
) {
    var detailShown by remember(view.raw) { mutableStateOf(false) }
    val reduceMotion = rememberReduceMotion()
    val tone = view.tone
    val shape = RoundedCornerShape(PaEditorial.Radius.Card)
    Column(
        modifier
            .fillMaxWidth()
            .pressableLong(
                onClick = { detailShown = !detailShown },
                onLongPress = { detailShown = !detailShown },
            )
            .background(tone.container, shape)
            .border(1.dp, tone.accent.copy(alpha = 0.30f), shape)
            .padding(horizontal = 14.dp, vertical = 12.dp)
            .semantics { contentDescription = "${view.headline}。详情：${view.raw}" },
    ) {
        if (!context.isNullOrBlank()) {
            Text(
                context,
                fontSize = PaEditorial.Type.Micro.fontSize, lineHeight = 14.sp,
                fontWeight = FontWeight.SemiBold,
                color = tone.text,
            )
            Spacer(Modifier.height(2.dp))
        }
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(
                painterResource(R.drawable.ic_lucide_triangle_alert), null,
                tint = tone.accent, modifier = Modifier.size(15.dp),
            )
            Spacer(Modifier.width(8.dp))
            Text(
                view.headline,
                Modifier.weight(1f),
                fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 18.sp,
                fontWeight = FontWeight.SemiBold,
                color = tone.text,
            )
            val actionLabel = paErrorActionLabelOf(view.action)
            val actionHandler = when (view.action) {
                PaErrorAction.GO_PAIR -> onGoPair
                PaErrorAction.TROUBLESHOOT -> onTroubleshoot
                PaErrorAction.NONE -> null
            }
            if (actionLabel != null && actionHandler != null) {
                Text(
                    actionLabel,
                    Modifier
                        .heightIn(min = PaEditorial.Space.TouchMin)
                        .pressableLong(
                            onClick = { actionHandler() },
                            onLongPress = { detailShown = !detailShown },
                        )
                        .padding(horizontal = 8.dp),
                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                    fontWeight = FontWeight.SemiBold,
                    color = PaEditorial.Primary,
                )
            }
        }
        if (onRetry != null) {
            Spacer(Modifier.height(6.dp))
            Row(
                Modifier
                    .fillMaxWidth()
                    .heightIn(min = PaEditorial.Space.TouchMin)
                    .pressableLong(
                        onClick = { if (!retrying) onRetry() },
                        onLongPress = { detailShown = !detailShown },
                    ),
                horizontalArrangement = Arrangement.Center,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                if (retrying) {
                    CircularProgressIndicator(
                        strokeWidth = 2.dp,
                        color = PaEditorial.Primary,
                        modifier = Modifier.size(12.dp),
                    )
                    Spacer(Modifier.width(6.dp))
                    Text(
                        "进行中…",
                        fontSize = PaEditorial.Type.BodySmall.fontSize,
                        color = PaEditorial.Subtle,
                    )
                } else {
                    Icon(
                        painterResource(R.drawable.ic_lucide_refresh_cw), null,
                        tint = PaEditorial.Primary, modifier = Modifier.size(14.dp),
                    )
                    Spacer(Modifier.width(6.dp))
                    Text(
                        retryLabel,
                        fontSize = PaEditorial.Type.BodySmall.fontSize,
                        fontWeight = FontWeight.SemiBold,
                        color = PaEditorial.Primary,
                    )
                }
            }
        }
        AnimatedVisibility(
            visible = detailShown,
            enter = if (reduceMotion) EnterTransition.None else {
                expandVertically(PaMotion.standard()) + fadeIn(PaMotion.standard())
            },
            exit = if (reduceMotion) ExitTransition.None else {
                shrinkVertically(PaMotion.standard()) + fadeOut(PaMotion.standard())
            },
        ) {
            Text(
                view.raw,
                Modifier
                    .fillMaxWidth()
                    .padding(top = 6.dp),
                fontSize = PaEditorial.Type.Micro.fontSize, lineHeight = 15.sp,
                fontFamily = PaEditorial.MonoFont,
                color = tone.text,
            )
        }
    }
}
