package dev.pocketagent.app.ui.model

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.modelBareName
import dev.pocketagent.app.ui.theme.PaEditorial
import pocketagent.v1.ModelInfo

/**
 * 模型选择卡的单行选项(2026-10-04 会话选择与历史体验计划 Task 4 拆分件)。
 *
 * 【它是什么】原 ChatScreen.ModelEffortCard 里"跟随桌面配置"行与每个模型行的
 * 同一份行布局(品牌图标 + 主文案 + 能力副文案 + 选中勾),抽成无状态展示件:
 * 选中态、能力明细全部由调用方算好传入 —— 卡面只管"哪一行被选中、点了谁",
 * 行自己不持状态、不碰 RPC、不碰全局(照 ModelSelectionState 的纪律)。
 *
 * 【窄屏可读(计划验收项)】主文案**不限行数**(长模型名按需换行,绝不省略号截断);
 * 能力副文案(上下文/思考/图片/可用性)经 [modelRowDetail] 拼成一行小字,
 * Text 自身可换行 —— 窄屏放得下就一排,放不下就折行,任何字段都不丢。
 */

/**
 * 模型行能力副文案(纯函数,JVM 单测钉死 —— 见 ModelEffortOptionsTest)。
 *
 * 优先级:可用性说清 > 能力枚举。模型不可用(电脑端 key 未配置/不连通)时
 * 一句话说明原因位,不再罗列能力 —— "能不能用"比"有多强"更先要被读到。
 * 上下文窗口未知(≤0)保持未知:不显示"0k"这种假数字。
 */
internal fun modelRowDetail(
    contextWindow: Int,
    supportsThinking: Boolean,
    supportsImages: Boolean,
    available: Boolean,
): String {
    if (!available) return "暂不可用(电脑端未连通或未配置)"
    val parts = ArrayList<String>(3)
    if (contextWindow > 0) parts += "上下文 ${contextWindow / 1000}k"
    if (supportsThinking) parts += "支持思考"
    if (supportsImages) parts += "支持图片"
    return parts.joinToString(" · ")
}

/**
 * 模型 id → 人话显示名的**统一翻译层**(L2 真机发现 5 后半:Hermes/Pi 会话把
 * daemon 未带展示名的模型以裸 id 直接上屏,如 "hermes/deepseek-flash")。
 *
 * 解析顺序(照 ModelEffortCard「Agent 当前」行的既有风格:displayName 优先,
 * 空串 displayName 不算数、继续往下兜底):
 *  ① [preferred] —— 会话级申报表(与完整卡同源的仓库模型表按 agent 过滤);
 *  ② [secondary] —— 全 agent 大表:申报表按 daemon 会话级口径收窄后,正在运行
 *     但不在申报表里的模型(如被真值校正回退的旧模型)仍能从大表翻出人话名;
 *  ③ modelBareName —— 去 agent 前缀的裸名;无前缀 id 原样返回(兜底即原 id),
 *     绝不空白、不造假名。
 *
 * 纯函数,JVM 单测钉死 —— 见 ModelDisplayLabelTest。
 */
internal fun modelDisplayLabel(
    modelId: String,
    preferred: List<ModelInfo>,
    secondary: List<ModelInfo> = emptyList(),
): String =
    // 空串 displayName 不算翻译成功:跳过该条继续往下一张表找,而不是
    // 停在首个 id 命中上 —— 两表都没给有效名字才落裸名。
    (preferred.firstOrNull { it.id == modelId && it.displayName.isNotBlank() }
        ?: secondary.firstOrNull { it.id == modelId && it.displayName.isNotBlank() })
        ?.displayName
        ?: modelBareName(modelId)

/**
 * 一行模型选项。
 *
 * @param title 主文案(displayName;空时由调用方退回模型 id —— 绝不显示空白行)。
 * @param detail 能力副文案(见 [modelRowDetail];空串则不占位)。
 * @param subtitle 解释性副文案(如"跟随桌面"行的语义说明;空串则不占位)。
 * @param selected 选中态:主文案转主色加粗 + 行尾勾 —— 来源是
 *   `ModelSelectionReducer.display(state)`,本行不自持。
 * @param brandIconRes 品牌图标资源(null = 不留位;必须原色 tint=Unspecified,
 *   品牌色不能被主题染掉 —— 旧卡同纪律)。
 */
@Composable
internal fun ModelOptionRow(
    title: String,
    selected: Boolean,
    onClick: () -> Unit,
    detail: String = "",
    subtitle: String = "",
    brandIconRes: Int? = null,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .pressable(onClick = onClick)
            .padding(horizontal = 4.dp, vertical = 11.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        brandIconRes?.let { res ->
            Icon(
                painterResource(res), null,
                tint = Color.Unspecified, modifier = Modifier.size(28.dp),
            )
            Spacer(Modifier.width(12.dp))
        }
        Column(Modifier.weight(1f)) {
            // 【完整显示,不截断】不限行数,带后缀的长模型名按需换行(旧卡同款修复:
            // 用户反馈"模型名称仍未完整显示");displayName 为空时调用方退回 id。
            Text(
                title,
                fontSize = 15.sp,
                color = if (selected) PaEditorial.Primary else PaEditorial.Ink,
            )
            if (subtitle.isNotBlank()) {
                Text(subtitle, fontSize = 10.5.sp, color = PaEditorial.Subtle)
            }
            if (detail.isNotBlank()) {
                Text(detail, fontSize = 10.5.sp, color = PaEditorial.Subtle)
            }
        }
        if (selected) {
            Spacer(Modifier.width(8.dp))
            Icon(
                painterResource(AppR.drawable.ic_lucide_check), null,
                tint = PaEditorial.Primary, modifier = Modifier.size(20.dp),
            )
        }
    }
}
