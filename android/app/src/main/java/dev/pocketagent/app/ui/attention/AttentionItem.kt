package dev.pocketagent.app.ui.attention

/**
 * Attention Center 的一条注意项(2026-10-04 未知事件用户处理中心计划 Task 5)。
 *
 * 【四类从哪来】
 *  - [UnknownEvent]:电脑端新消息(daemon 未知事件台账,Task 4 卡片模型的投影);
 *  - [Approval]:会话待审批(与首页「需要你处理」区同源,ListThreads 快照);
 *  - [Recovery]:电脑端启动恢复异常(GetHostInfo.recovery,degraded 才成条);
 *  - [Collab]:多 Agent 协同(needsAction 区分"等你决策"与"运行中")。
 *
 * 【字段纪律】title/detail/actionLabel 都是**已有事实的人话搬运**,不在此层生成
 * 新判断:未知事件的 title/impact 来自 daemon 固定文案表,审批/恢复来自各自既有
 * 展示口径。没有真实数据源的类(Collab)只定义形态,调用方没有事实就不构造 ——
 * 宁缺勿假,不造第四类内容凑数。
 */
sealed interface AttentionItem {
    /** 稳定 id(未知事件 = eventId;审批 = threadId;其余为固定键)。 */
    val id: String
    /** 一行人话标题。 */
    val title: String
    /** 一两句说明(影响/来源/数量),不复制完整详情 —— 完整内容在各自详情处。 */
    val detail: String
    /** 行尾动作文案;null = 没有专属动作词(点击即去详情)。 */
    val actionLabel: String?

    /** 电脑端新消息(未知事件;Task 4 的 [dev.pocketagent.app.ui.UnknownEventCardModel] 投影)。 */
    data class UnknownEvent(
        override val id: String,
        override val title: String,
        override val detail: String,
        override val actionLabel: String? = null,
        /** 关联会话;空 = 无归属 —— 点击只进设置详情,不虚构跳转目标。 */
        val threadId: String,
        val requiresUser: Boolean,
        /** 处置状态原文(NEW/INSPECTED/…),归组由 [attentionBucketOf] 判。 */
        val processingStatus: String,
        val receivedAtUnixMs: Long,
    ) : AttentionItem

    /** 会话待审批(点击进会话,审批弹窗接住 —— 既有链路,不新建路由)。 */
    data class Approval(
        override val id: String,
        override val title: String,
        override val detail: String,
        override val actionLabel: String? = null,
        val pendingCount: Int,
    ) : AttentionItem

    /** 电脑端启动恢复异常(点击进设置的连接/运行时详情)。 */
    data class Recovery(
        override val id: String,
        override val title: String,
        override val detail: String,
        override val actionLabel: String? = null,
    ) : AttentionItem

    /**
     * 多 Agent 协同。**当前没有"协同待决策"的真实数据源**(collab:<id> 只声明接线),
     * 调用方拿不到事实就不要构造本条 —— 形态先落位,数据等 daemon 侧信号。
     */
    data class Collab(
        override val id: String,
        override val title: String,
        override val detail: String,
        override val actionLabel: String? = null,
        /** true = 等用户决策(进「需要我处理」);false = 运行中事实(进「运行中」)。 */
        val needsAction: Boolean,
    ) : AttentionItem
}
