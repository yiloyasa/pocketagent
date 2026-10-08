package dev.pocketagent.app.ui.component

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.PaStatusGlyph
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.EditorialTone
import dev.pocketagent.app.ui.theme.PaEditorial

/**
 * 主机连接三态(PaHostChip 的输入口径)。
 *
 * 【为什么只有三态】README §状态:顶部统一主机状态,只回答"连没连上"——
 * 已连接(绿)/ 连接中(中性)/ 离线(灰 + 上次更新时间)。"正在执行"是**会话/任务**
 * 的状态词,与连接态严格分家:离线时绝不出现(见 [hostChipCaption] 的构造性保证)。
 */
enum class PaHostLink { CONNECTED, CONNECTING, OFFLINE }

/** 连接态 → 状态词(语义双通道之一:颜色之外必须有词)。 */
fun hostStatusWord(link: PaHostLink): String = when (link) {
    PaHostLink.CONNECTED -> "已连接"
    PaHostLink.CONNECTING -> "连接中"
    PaHostLink.OFFLINE -> "离线"
}

/** 连接态 → 语义色档:已连接绿、连接中中性墨、离线灰(README §视觉系统)。 */
fun hostStatusTone(link: PaHostLink): EditorialTone = when (link) {
    PaHostLink.CONNECTED -> EditorialTone.RUNNING
    PaHostLink.CONNECTING -> EditorialTone.NEUTRAL
    PaHostLink.OFFLINE -> EditorialTone.GREY
}

/**
 * 连接态 → 形通道(与四词状态同一套 [PaStatusGlyph],纯函数,单测钉死
 * StatusGlyphMappingTest):已连接=对勾(通了的确认形)/ 连接中=实心点
 * (进行中)/ 离线=空心环(未接通)。三态颜色本就不同(绿/墨/灰),形状
 * 是给色弱用户的第三条保险 —— 词、色、形三线同义。
 */
fun hostStatusGlyph(link: PaHostLink): PaStatusGlyph = when (link) {
    PaHostLink.CONNECTED -> PaStatusGlyph.CHECK
    PaHostLink.CONNECTING -> PaStatusGlyph.DOT
    PaHostLink.OFFLINE -> PaStatusGlyph.RING
}

/**
 * 胶囊副标题文案(纯函数,单测钉死 EditorialHostChipCopyTest)。
 *
 * 【口径】
 *  - 已连接:展示实际采用的通道(README:连接候选并行尝试,成功后只展示实际
 *    采用的通道),由调用方给 [channelLabel](如「局域网 · 会话实时同步」);
 *    没给就只说「已连接」,不编通道。
 *  - 连接中:中性措辞「正在建立连接…」,不冒充已连接也不吓人;给了通道标注
 *    (目标通道)则并列展示。
 *  - 离线:**必须**带「上次更新 <时间>」(README:未连接时状态仍显示上次更新
 *    时间,不能写"正在执行")。[lastUpdateText] 空缺时说「暂无更新记录」,
 *    绝不回退到任何"正在…"的措辞 —— 这是构造性保证:本函数根本接不到运行态。
 */
fun hostChipCaption(link: PaHostLink, channelLabel: String?, lastUpdateText: String?): String =
    when (link) {
        PaHostLink.CONNECTED -> channelLabel?.takeIf { it.isNotBlank() } ?: "已连接"
        PaHostLink.CONNECTING ->
            if (channelLabel.isNullOrBlank()) "正在建立连接…" else "正在建立连接… · $channelLabel"
        PaHostLink.OFFLINE ->
            if (lastUpdateText.isNullOrBlank()) "离线 · 暂无更新记录" else "离线 · 上次更新 $lastUpdateText"
    }

/** 读屏语义:主机名 + 状态词 + 副标题一次念全,不靠颜色区分。 */
fun hostChipA11y(hostName: String, link: PaHostLink, caption: String): String =
    "主机 $hostName，${hostStatusWord(link)}，$caption"

/**
 * 主机状态胶囊(README「实现落地建议」四类共享组件之一;第二波把首页/抽屉/
 * 设置的顶部主机状态统一接到这里)。
 *
 * 【样式】status-card(premium.css):瓷白底、1dp 细线描边、18dp 圆角、卡片阴影,
 * 42dp 图标方格(13dp 圆角)+ 标题行 + 状态行。语义双通道:状态点用色,状态词
 * 跟在主机名后([hostStatusWord]),副标题再复述([hostChipCaption])。
 *
 * 【图标槽】[icon] 缺省渲染中性 monitor 细线图标 —— 主机是设备不是品牌,
 * 不冒认任何 Agent 品牌;Agent 品牌图由调用方按项目已有品牌传入。
 *
 * @param hostName 主机显示名(如「工作站」)。
 * @param link     连接三态。
 * @param channelLabel 实际采用通道的标注(已连接)/ 目标通道(连接中);离线时忽略。
 * @param lastUpdateText 上次更新时间的人话文本(离线时必显,建议调用方传相对/绝对时间)。
 * @param onClick  整卡可点(进主机详情/连接管理);null = 纯展示,无箭头。
 * @param icon     42dp 图标槽;null = 中性 monitor 占位。
 */
@Composable
fun PaHostChip(
    hostName: String,
    link: PaHostLink,
    modifier: Modifier = Modifier,
    channelLabel: String? = null,
    lastUpdateText: String? = null,
    onClick: (() -> Unit)? = null,
    icon: (@Composable () -> Unit)? = null,
) {
    val tone = hostStatusTone(link)
    val caption = hostChipCaption(link, channelLabel, lastUpdateText)
    Row(
        modifier
            .then(if (onClick != null) Modifier.pressable { onClick() } else Modifier)
            .heightIn(min = 77.dp)
            .shadow(
                elevation = 2.dp, shape = PaCardShape,
                ambientColor = PaEditorial.Shadow.Card.color,
                spotColor = PaEditorial.Shadow.Card.color,
            )
            .background(PaEditorial.Surface, PaCardShape)
            .border(1.dp, PaEditorial.Hairline, PaCardShape)
            .padding(horizontal = 16.dp, vertical = 17.dp)
            .semantics { contentDescription = hostChipA11y(hostName, link, caption) },
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier
                .size(42.dp)
                .background(tone.container, RoundedCornerShape(PaEditorial.Radius.IconButton)),
            contentAlignment = Alignment.Center,
        ) {
            if (icon != null) {
                icon()
            } else {
                Icon(
                    painterResource(R.drawable.ic_lucide_monitor), null,
                    tint = tone.accent, modifier = Modifier.size(20.dp),
                )
            }
        }
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            // 标题行:主机名 · 状态词(词与点同色通道,词是语义本体)
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    hostName,
                    fontSize = PaEditorial.Type.Body.fontSize, lineHeight = 20.sp,
                    fontWeight = FontWeight.Bold,
                    color = PaEditorial.Ink,
                    maxLines = 1,
                )
                Spacer(Modifier.width(6.dp))
                Text("·", fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Subtle)
                Spacer(Modifier.width(6.dp))
                Text(
                    hostStatusWord(link),
                    fontSize = PaEditorial.Type.Body.fontSize, lineHeight = 20.sp,
                    fontWeight = FontWeight.Bold,
                    color = PaEditorial.Ink,
                    maxLines = 1,
                )
            }
            Spacer(Modifier.height(3.dp))
            // 状态行:形通道字形(环/点/勾,与四词状态同一套)+ 副标题(通道/上次更新)
            Row(verticalAlignment = Alignment.CenterVertically) {
                PaStatusGlyphSlot(hostStatusGlyph(link), tone.accent)
                Spacer(Modifier.width(6.dp))
                Text(
                    caption,
                    fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp,
                    color = tone.text,
                    maxLines = 1,
                )
            }
        }
        if (onClick != null) {
            Spacer(Modifier.width(8.dp))
            Icon(
                painterResource(R.drawable.ic_lucide_chevron_right), null,
                tint = PaEditorial.Subtle, modifier = Modifier.size(18.dp),
            )
        }
    }
}

private val PaCardShape = RoundedCornerShape(PaEditorial.Radius.Card)
