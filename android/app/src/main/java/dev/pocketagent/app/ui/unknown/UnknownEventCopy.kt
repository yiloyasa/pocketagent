package dev.pocketagent.app.ui.unknown

/**
 * 「电脑端新消息」(未知事件用户处理中心,2026-10-04 计划 Task 4)的文案集中地。
 *
 * 【为什么集中】这一片的措辞有三条红线要一起看住:
 *  - 人话优先:入口与分组用普通用户能懂的词,不出现「透析」「未知事件」这类技术台账词;
 *  - 降级是事实陈述:写"版本较旧、暂无详细信息",不写"未知/错误"(AUDIT §3.3:
 *    机器无从裁决用户可见性,索性只陈述事实);
 *  - 失败不吓人:详情读不到是"稍后再试",技术异常只进日志,不上界面。
 * 文案改一处即全 app 生效,也方便评审对照计划原文逐句核。
 */
object UnknownEventCopy {

    // ---- 入口与标题 ----

    /** 弹窗标题(替代旧的「消息透析」:普通用户不知道"透析"是什么)。 */
    const val SHEET_TITLE = "电脑端新消息"

    /** 控制中心域行的人话入口标题(计划 Task 4:入口改「电脑端新消息」)。 */
    const val ENTRY_TITLE = "电脑端新消息"

    /**
     * 控制中心域行标题的客户端覆盖:只有透析域(dialysis,daemon 契约钉死的稳定 id)
     * 按计划改成人话;其余域标题照 daemon 原文,不改写(daemon 事实不过客户端的手)。
     */
    fun entryTitleOverride(domainId: String, daemonTitle: String): String =
        if (domainId == "dialysis") ENTRY_TITLE else daemonTitle

    // ---- 空态 ----

    const val EMPTY_TITLE = "最近没有电脑端的新消息"
    const val EMPTY_HINT = "电脑端发来新事件后会登记在这里"

    // ---- 降级(旧 daemon 无 12~17 人话字段)----

    /** 固定降级说明:只陈述版本事实,不写「未知/错误」,不猜内容补位。 */
    const val DEGRADED_NOTE = "当前电脑端版本较旧，暂无详细信息"

    // ---- 分组(Task 4:需要你处理 / 已查看 / 仅供参考)----

    const val GROUP_NEEDS_USER = "需要你处理"
    const val GROUP_HANDLED = "已查看"
    const val GROUP_REFERENCE = "仅供参考"

    // ---- 详情加载 ----

    const val DETAIL_LOADING = "读取中…"

    /** 详情加载失败的人话(技术异常只进日志,不上界面 —— 评审焦点红线)。 */
    const val DETAIL_LOAD_FAILED = "暂时没读到详情，请稍后再试"

    // ---- 技术详情(原文 / 字段透析 / correlation id 折叠处)----

    const val TECHNICAL_SECTION_TITLE = "技术详情"
    const val TECHNICAL_SHOW = "查看技术详情"
    const val TECHNICAL_HIDE = "收起技术详情"
    const val FIELD_DIGEST_TITLE = "逐字段"

    // ---- 动作 ----

    /** 动作执行成功的人话兜底(daemon 的 detail 缺席时才用,正常情况照抄 daemon 原文)。 */
    const val ACTION_DONE = "已完成"

    /** 动作执行失败的人话(daemon 的错误说明可用时优先透传 daemon 原文)。 */
    const val ACTION_FAILED = "操作没有完成，请稍后再试"

    /** revision 冲突(事件已在别处被处理)的人话:说清"已为你刷新",不用用户再猜。 */
    const val CONFLICT_REFRESHED = "这条事件的状态已变化，已为你刷新"

    /** acknowledge 等状态型动作的可选备注输入。 */
    const val NOTE_LABEL = "备注（可选）"
    const val NOTE_PLACEHOLDER = "可以给电脑端留一句话"

    // ---- copy(手机本地剪贴板,绝不发 daemon)----

    const val COPY_SUCCESS = "已复制技术原文"
    const val COPY_EMPTY = "还没有可复制的技术原文"
}
