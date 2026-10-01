package dev.pocketagent.app.ui

import pocketagent.v1.RuntimePolicyResponse

/**
 * 控制中心「直控开关」逻辑层(2026-09-30 双端直控波次):把控制中心域行头部的两个
 * 直控 Switch 的取值/可用性钉成纯函数,写动作复用既有通道,不新增任何写路径。
 *
 * 【为什么单独成层】直控开关最容易犯的错是"状态失真时替 daemon 说话":
 *  - 现值还没读到就渲染一个可点的开关(点了先 Get,拿到的是旧世界);
 *  - toggle 现值认不出(词表外)却猜一个布尔上屏;
 *  - B 开关的预算快照缺席时显示假"关"且可点(把"没读到"画成"是关的")。
 * 这些都被本层的纯函数钉死在单测里(ControlDirectSwitchLogicTest),UI 只排版。
 *
 * 【写通道刻意不新增】协同总开关走既有 SetCollabPolicy(经 ControlPlaneState.applyToggle
 * 的"先 Get 再完整 Set + CAS"纪律);B 开关走既有 SetRuntimePolicy 的
 * unknown_request_passthrough 字段(与设置页「实验功能」区完全同一份数据、同一条
 * 通道,见 [applyUnknownPassthroughToggle])。给直控开关另配写路径 = 给同一份
 * 状态造第二份真值。
 */

/**
 * 一个直控开关的呈现态。
 * @param checked 开关位置(照 daemon 真值,不做乐观改写)。
 * @param enabled 是否可点:false = 加载中/现值未读到/别的开关正在提交 —— 如实禁用。
 */
data class DirectSwitchUiState(val checked: Boolean, val enabled: Boolean)

/** collab 域的稳定项 id(协同总开关;daemon 契约钉死,客户端按 id 匹配)。 */
const val COLLAB_MASTER_SWITCH_ID = "collab.collaboration_enabled"

/**
 * collab 域里直控的"协同总开关"项。
 * null = 域不匹配 / daemon 快照里没有该项 —— 头部不摆开关(降级为既有展开编辑),
 * 不造一个没有数据来源的假入口。
 */
internal fun collabMasterSwitchItem(domain: ControlDomainRow?): ControlItemRow? =
    domain?.takeIf { it.id == CONTROL_DOMAIN_COLLAB }
        ?.items?.firstOrNull { it.id == COLLAB_MASTER_SWITCH_ID }

/**
 * 协同总开关的呈现态:现值必须是 daemon 词表内的"开"/"关"才可点
 * ([controlToggleOn] 返回 null = 认不出,禁用并让展开体只读呈现原文);
 * 别的开关正在提交(busy)时同样禁用 —— 防连点叠加两次 Set。
 */
internal fun collabMasterSwitchState(item: ControlItemRow?, busy: Boolean): DirectSwitchUiState {
    val on = item?.let { controlToggleOn(it.current) }
    return DirectSwitchUiState(checked = on == true, enabled = on != null && !busy)
}

// ---- 状态词与禁用原因(R1-C:状态词辅助,不只靠色;禁用要说清"为什么") ----
//
// 【为什么要词不要色】开关被禁用时用户只知道"点不动",不知道是"没读到/读不懂/
// 正在提交"里的哪一种 —— 三种的下一步动作完全不同(重连 / 重读 / 等一下)。
// 状态词给"此刻是什么",禁用原因给"为什么动不了、该怎么办";颜色(灰)只作辅助。

/**
 * 协同总开关的状态词。null 项 = 开关不摆(装配点判空),返回空串;
 * 提交中 > 现值未识别 > 开/关(与禁用判定同源,词和位置永不互相矛盾)。
 */
internal fun collabMasterSwitchWord(item: ControlItemRow?, busy: Boolean): String = when {
    item == null -> ""
    busy -> "提交中"
    else -> controlToggleOn(item.current)?.let { if (it) "开" else "关" } ?: "未识别"
}

/**
 * 协同总开关的禁用原因;null = 可点(不需要解释)。item 缺席时开关本体不渲染,
 * 也无需原因(装配点根本不会摆出来)。
 */
internal fun collabMasterSwitchDisabledReason(item: ControlItemRow?, busy: Boolean): String? = when {
    item == null -> null
    busy -> "正在提交上一项修改,稍候自动恢复"
    controlToggleOn(item.current) == null -> "电脑端现值「${item.current}」无法识别,已暂停直控"
    else -> null
}

/**
 * 未知请求透传 B 开关的状态词。快照缺席 = "未读取"(与禁用判定同源:
 * 状态函数把缺席画成禁用,这里就绝不给"关" —— 不显示假状态)。
 */
internal fun unknownPassthroughSwitchWord(budget: RuntimePolicyResponse?, busy: Boolean): String = when {
    budget == null -> "未读取"
    busy -> "提交中"
    unknownRequestPassthroughOn(budget.policy) -> "开"
    else -> "关"
}

/** B 开关的禁用原因;null = 可点。缺席 = 需连接主机(结论 5:依赖主机的项断连要讲清)。 */
internal fun unknownPassthroughSwitchDisabledReason(budget: RuntimePolicyResponse?, busy: Boolean): String? = when {
    budget != null -> null
    busy -> "正在读取电脑端状态"
    else -> "尚未读到电脑端状态,连接主机后可用"
}

/**
 * 未知请求透传 B 开关的呈现态。数据源 = RuntimeControlState.budget.policy 的
 * unknown_request_passthrough(与设置页实验区同一份快照)。
 *
 * 【budget 快照缺席 = 禁用】[unknownRequestPassthroughOn] 对 null 显示"关"是
 * 设置页的降级呈现;但直控开关是可点的,把"没读到"画成"是关的"且可点,用户一按
 * 就会先把快照拉下来再写 —— 中间若真值是"开",这次点击就写反了。所以直控位必须
 * 等快照在手才启用(点了 SetRuntimePolicy 弹窗里的同款纪律:先 Get 再改)。
 */
internal fun unknownPassthroughSwitchState(budget: RuntimePolicyResponse?, busy: Boolean): DirectSwitchUiState =
    DirectSwitchUiState(
        checked = unknownRequestPassthroughOn(budget?.policy),
        enabled = budget != null && !busy,
    )

/**
 * 透析域头部 B 开关的写动作(与设置页实验区 setPassthrough 同语义同通道):
 * 先确保预算快照在手(CAS 需要 revision),再完整策略提交,成功后回读
 * GetControlPlane 控制中心真值同步 —— 不做乐观改写。
 * 返回失败原因(daemon 原文),null = 成功;调用方落在本区段的写失败位。
 */
internal suspend fun applyUnknownPassthroughToggle(on: Boolean): String? {
    RuntimeControlState.ensureBudgetSnapshot()?.let { return it }
    val err = RuntimeControlState.saveBudget { b -> applyUnknownRequestPassthrough(b, on) }
    if (err != null) return err
    ControlPlaneState.refreshSuspend()
    return null
}
