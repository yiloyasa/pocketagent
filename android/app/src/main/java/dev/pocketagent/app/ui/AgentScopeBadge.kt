package dev.pocketagent.app.ui

import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.ui.theme.PaEditorial

/**
 * 作用域徽章(S2 设置信息架构轮):行内"以下参数仅控制 Codex 生成方式"这类整句
 * 说明,统一收敛成 codex / pi / zcode / hermes 小标签 —— 用户一眼看出这项设置
 * 作用于谁,不用逐句读说明文字。
 *
 * 【为什么是描边 pill 而不是底色 pill】统一卡语言(SettingsScreen.kt 的裁定):
 * 卡内禁止带底色子容器,层级靠颜色不靠底 —— 徽章只画 1dp Hairline 描边,
 * 文字用 Muted,不做填充底色。
 *
 * 【名称与图标都只有一份来源】名字复用 SettingsScreen.kt 的 [agentDisplay]
 * (AGENT_DISPLAY 映射 + 未知 id 回退原文,AUDIT §3.3);图标复用 AgentBrand.kt 的
 * [agentBrandIcon](与「切换 Agent」行同一份)。徽章不自带词表,避免重演
 * "设置里的 Agent 图标和主页对不上"的两份映射事故。
 */

/**
 * 全局显示偏好(「会话与对话」组:思考 / 工具 / 历史轮数)的作用域:
 * 对全部已接入 agent 生效 —— 原 Daemon 页「全局对话显示」的说明即此口径,
 * 聚拢成一级分组后用整排徽章点明"全部"。
 *
 * 【presentation-only】本名单仅决定徽章展示顺序,不参与能力判定/会话路由;
 * 新增接入 agent 时须同步本行(并确认 AGENT_DISPLAY 有其显示名)。
 * 为什么不走动态能力透传:全局显示偏好不作用于外部会话只读导入
 * (opencode/claude-code),动态源会把它们误纳入作用域 —— 口径见 AUDIT 附录 A。
 */
internal val SESSION_DISPLAY_SCOPES: List<String> = listOf("codex", "pi", "zcode", "hermes")

/**
 * 一排作用域徽章。
 *
 * @param agents 作用的 agent id 列表(展示顺序即传入顺序)。
 * @param prefix 徽章前的引导词(默认"作用于");传空串则只渲染徽章本体。
 */
@Composable
fun AgentScopeBadge(
    agents: List<String>,
    modifier: Modifier = Modifier,
    prefix: String = "作用于",
) {
    Row(modifier = modifier, verticalAlignment = Alignment.CenterVertically) {
        if (prefix.isNotEmpty()) {
            Text(
                prefix,
                fontSize = PaEditorial.Type.Micro.fontSize,
                color = PaEditorial.Subtle,
            )
            Spacer(Modifier.width(6.dp))
        }
        agents.forEachIndexed { index, id ->
            AgentScopeTag(id)
            if (index != agents.lastIndex) Spacer(Modifier.width(4.dp))
        }
    }
}

/** 单个 agent 的作用域小标签:1dp Hairline 描边 pill + 品牌图标(原色)+ 名字。 */
@Composable
private fun AgentScopeTag(agentId: String) {
    Row(
        Modifier
            .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.Terminal))
            .padding(horizontal = 6.dp, vertical = 3.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        agentBrandIcon(agentId)?.let { res ->
            Icon(
                painterResource(res), null,
                tint = Color.Unspecified,
                modifier = Modifier.size(12.dp),
            )
            Spacer(Modifier.width(4.dp))
        }
        Text(
            agentDisplay(agentId),
            fontSize = PaEditorial.Type.Micro.fontSize,
            fontWeight = FontWeight.Medium,
            color = PaEditorial.Muted,
            maxLines = 1,
        )
    }
}
