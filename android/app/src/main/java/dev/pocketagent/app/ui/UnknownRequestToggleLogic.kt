package dev.pocketagent.app.ui

import pocketagent.v1.RuntimePolicy

/**
 * 辅线 B「未识别操作确认(实验)」开关的读写映射(daemon RuntimePolicy 的全局布尔
 * unknown_request_passthrough;读 GetRuntimePolicy,写既有 SetRuntimePolicy 完整
 * 策略 + CAS)。
 *
 * 【为什么抽成纯函数】这层最容易犯的错是把"缺席"读成"开",或把"用户显式关掉"
 * 写成"缺席"(表现:开关显示的值和 daemon 生效值不一致,或换页回来开关自己弹回)。
 * 映射钉在单测里(UnknownRequestToggleLogicTest),不依赖真机与 daemon。
 *
 * 【为什么落在 Daemon 桥接与输入页而不是控制中心】控制中心的域/项由 daemon 的
 * GetControlPlane 决定(呈现跟着 daemon 走);本开关是 daemon RuntimePolicy 的一等
 * 字段,设置页直接读写同一份策略,与「运行时」区段的 budget 块共用
 * [RuntimeControlState] 这一份真值 —— 两个界面一个数,谁写的都算数。
 */

/**
 * RuntimePolicy → 开关现值。**缺席(用户从未碰过)= 默认关**:daemon 侧语义就是
 * 缺席按关执行(未知反向请求直接 -32601 + 台账),显示必须同口径;反过来,显式
 * false 是"用户选过关",同样显示为关 —— 两种"关"对用户是同一个位置。
 * policy 为 null(还没读到快照)也显示关:等 ensureBudgetSnapshot 拉到后如实校正。
 */
internal fun unknownRequestPassthroughOn(policy: RuntimePolicy?): Boolean =
    policy != null && policy.hasUnknownRequestPassthrough() && policy.unknownRequestPassthrough

/**
 * 开关动作 → RuntimePolicy.Builder。**presence 纪律:关也要显式 set(false)**,
 * 绝不能 clear 让字段回缺席 —— 在"完整策略提交"的语义里两者生效值相同,但
 * "用户显式选过关"与"从未设置"是两份不同的用户意图,写丢就是不忠于所改。
 */
internal fun applyUnknownRequestPassthrough(b: RuntimePolicy.Builder, on: Boolean) {
    b.setUnknownRequestPassthrough(on)
}
