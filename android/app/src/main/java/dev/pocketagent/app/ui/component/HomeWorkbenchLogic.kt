package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.AgentGuideCopy
import dev.pocketagent.app.ui.AgentInfo
import dev.pocketagent.app.ui.placeholderDisplayAgents

/**
 * 首页工作台(HomeScreen)的加载/空态/磁贴三态判定 —— 纯函数层,JVM 单测钉死
 * (HomeWorkbenchLogicTest)。UI 层只做渲染,不掺判定口径。
 *
 * 【为什么抽出来】首屏效率轮的三条硬规矩都要"先判定再渲染":
 *  - 数据没回来前列表区摆骨架,不摆空态(空态只在"确实没有"时出现);
 *  - 空态必须给一句真实状态说明 + 一个明确去向按钮,不空白;
 *  - Agent 磁贴三态(可用/接入中/不可用)不单靠灰度,状态词必须落字。
 * 这些口径抽成纯函数后,UI 怎么改版判定都不漂移。
 */

// ---------------------------------------------------------------- 骨架门控

/**
 * 首页列表区是否显示加载骨架(纯函数):
 *  - 只在**已连接**且**首次加载尚未完成**且**没有加载失败**时出骨架 ——
 *    断连/失败时骨架就是撒谎(看起来在加载,其实不会回来),改走错误横幅与空态;
 *  - 重试/重连后的刷新(reloadKey++)不重置首次完成标记,已有内容不闪骨架。
 */
fun homeShowsRecentSkeleton(connected: Boolean, firstLoadDone: Boolean, loadError: String?): Boolean =
    connected && !firstLoadDone && loadError == null

// ---------------------------------------------------------------- 空态文案(说明 + 去向)

/**
 * 首页空态的三件套:真实状态说明(标题)+ 为什么/怎么办(副文)+ 明确去向(按钮词)。
 * 纯数据,UI 层保证按钮真的能去([onAction] 必须接真实入口)。
 */
data class HomeEmptyStateCopy(
    val title: String,
    val hint: String,
    val actionLabel: String,
)

/**
 * 最近会话空态(纯函数):区分"真没有"与"被当前 Agent 过滤没了"两种事实,
 * 文案不得张冠李戴(旧实现已连接时还写"连接电脑后…",就是在讲错的状态)。
 *
 *  - [hasAnyRecent]=false:本机确实没有会话 → 引导开第一个任务;
 *  - [hasAnyRecent]=true:有会话但都不属于当前选中 Agent → 说明过滤口径,
 *    同样给"开始新任务"的去向(最短路径永远是新开一单)。
 */
fun homeRecentEmptyCopy(hasAnyRecent: Boolean, agentLabel: String): HomeEmptyStateCopy =
    if (hasAnyRecent) {
        HomeEmptyStateCopy(
            title = "「$agentLabel」还没有会话",
            hint = "这里只显示当前 Agent 的会话,其他 Agent 的在会话页",
            actionLabel = "开始新任务",
        )
    } else {
        HomeEmptyStateCopy(
            title = "还没有会话",
            hint = "新建任务后,和电脑上的 Agent 干活",
            actionLabel = "开始新任务",
        )
    }

/**
 * 可用 Agent 网格的空态注记(纯函数):至少一个可用 → null(网格自己会说话);
 * 一个都没有 → 说明 + 去向(管理抽屉,看每个 agent 的真实状态与恢复开关)。
 */
fun homeAgentGridNote(availableCount: Int): HomeEmptyStateCopy? =
    if (availableCount > 0) {
        null
    } else {
        HomeEmptyStateCopy(
            title = "电脑端暂无可用 Agent",
            hint = "在电脑上安装任意受支持的 agent 并重启 daemon,磁贴会自动点亮",
            actionLabel = "查看管理",
        )
    }

/** 保留普通屏四列节奏；窄屏或大字体按实际可用宽度减少列数，不裁掉状态文字。 */
internal fun homeAgentGridColumns(availableWidthDp: Float, fontScale: Float): Int {
    val scale = fontScale.takeIf { it.isFinite() && it > 0f }?.coerceAtLeast(1f) ?: 1f
    val width = availableWidthDp.takeIf { it.isFinite() && it > 0f } ?: 0f
    val tileWidth = 72f * scale
    return ((width + 12f) / (tileWidth + 12f)).toInt().coerceIn(1, 4)
}

/**
 * 发现主机列表的空态(纯函数):搜索中不算空态(那是加载态,由骨架接管,返回 null);
 * 搜完没有 → 真实说明 + 去向(手动输入地址/配对码,不让人干等)。
 */
fun homeDiscoveryEmptyCopy(searching: Boolean): HomeEmptyStateCopy? =
    if (searching) {
        null
    } else {
        HomeEmptyStateCopy(
            title = "暂无发现的主机",
            hint = "确认电脑端 Pocket Agent 服务已启动,或直接手动添加",
            actionLabel = "手动输入地址",
        )
    }

// ---------------------------------------------------------------- Agent 磁贴三态

/**
 * Agent 磁贴三态(纯函数):可用 / 接入中 / 电脑端未安装。
 *
 * 【与能力的边界】判定只读 [AgentInfo.available](由 capabilities 动态点亮)
 * 与展示目录 [placeholderDisplayAgents],不做任何新的能力推断;
 * 三态的**状态词**由 [homeAgentTileLabel] 落字 —— 语义绝不单靠灰度深浅。
 */
enum class HomeAgentTileState {
    /** 可用:全彩磁贴,副文字=连接方式。 */
    AVAILABLE,
    /** 接入中:展示目录占位(daemon 还没有该 agent 的数据源)。 */
    INTEGRATING,
    /** 电脑端未安装:daemon 探测不到该 agent。 */
    NOT_INSTALLED,
}

/** 磁贴三态判定(纯函数)。 */
fun homeAgentTileState(agent: AgentInfo): HomeAgentTileState = when {
    agent.available -> HomeAgentTileState.AVAILABLE
    agent.id in placeholderDisplayAgents -> HomeAgentTileState.INTEGRATING
    else -> HomeAgentTileState.NOT_INSTALLED
}

/** 磁贴三态 → 副文字状态词(纯函数):可用态讲连接方式,灰显态讲真实原因。 */
fun homeAgentTileLabel(state: HomeAgentTileState, method: String): String = when (state) {
    HomeAgentTileState.AVAILABLE -> method
    HomeAgentTileState.INTEGRATING -> AgentGuideCopy.SUBTITLE_INTEGRATING
    HomeAgentTileState.NOT_INSTALLED -> AgentGuideCopy.SUBTITLE_NOT_INSTALLED
}
