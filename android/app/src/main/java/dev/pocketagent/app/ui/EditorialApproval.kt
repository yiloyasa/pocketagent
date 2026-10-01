package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.theme.EditorialTone

/**
 * 审批弹窗状态机(README §审批弹窗状态机)的纯逻辑层 —— 不接 RPC、无 Android
 * 依赖,JVM 单测钉死(EditorialApprovalLogicTest);Compose 皮在
 * component/PaApprovalDialog,第二波把 daemon 的 ApprovalRequest 接到
 * [ApprovalRequestUi] 上即可。
 *
 * 【状态流】收到请求(RECEIVED)→ 待决定(PENDING)→ 提交中(SUBMITTING)→
 * 服务端确认(CONFIRMED)→ 已允许(ALLOWED)/已拒绝(DENIED)。
 * 另有 UNAVAILABLE:请求失效/重复/跨会话/超时/提交失败 —— 决定按钮禁用,
 * 展示原因与重试;**绝不显示已批准,不做乐观通过**。
 *
 * 【dismiss ≠ 拒绝】本层没有"关闭=拒绝"这条边:关闭弹窗是宿主的 onDismiss
 * 回调(暂不处理,请求留在待处理区),与 onDecide(false) 完全两条通道。
 */
enum class ApprovalPhase {
    /** 收到请求:事件已落地,展示层短暂经过,即可决定。 */
    RECEIVED,

    /** 待决定:按钮可用。 */
    PENDING,

    /** 提交中:禁重复点击,但仍可查看全部上下文。 */
    SUBMITTING,

    /** 服务端已确认:终态词(已允许/已拒绝)落地前的一拍。 */
    CONFIRMED,

    /** 已允许:服务端确认后唯一可以"移除待处理"的依据。 */
    ALLOWED,

    /** 已拒绝。 */
    DENIED,

    /** 暂无法处理:失效/重复/超时/提交失败,展示原因与重试。 */
    UNAVAILABLE,
}

/**
 * 审批请求的 UI 值对象(第二波由 daemon 事件装配)。
 *
 * 【只放必要信息】README 弹窗白名单:Agent 与会话、操作目的、工作区、操作类型、
 * 原始命令 —— 不放风险评分、不放 agent 品牌断言([agentName] 由调用方填,
 * 未知给中性占位词,组件只留图标槽)。
 *
 * @param submittedChoice SUBMITTING 中用户点的那颗按钮(true=允许/false=拒绝),
 *   用于在按钮上回显"提交中…"而非两颗都显示;提交失败进 UNAVAILABLE 时保留,
 *   供"上一次尝试没有成功"的措辞与重试。
 * @param unavailableReason UNAVAILABLE 时的人话原因(失效/重复/超时/提交失败),
 *   由调用方给具体文案,组件只负责展示。
 * @param queuePosition / queueTotal 多请求按到达顺序的数量(1 起始)。
 * @param options 审批卡可选项(daemon ApprovalRequest.options,codex 0.159.2
 *   三按钮审批);**空 = 旧 daemon**,组件走旧两按钮,行为一字不变。
 * @param details daemon 给的人话详情(proposedExecpolicyAmendment /
 *   commandActions 摘要);null/空白不渲染,不摆空壳。
 * @param submittedOption SUBMITTING 中用户点的选项 id(第三按钮起才有;null =
 *   旧两按钮口径,回显按 [submittedChoice] 落位)。
 */
data class ApprovalRequestUi(
    val requestId: String,
    val threadId: String,
    val agentName: String,
    val sessionTitle: String,
    val purpose: String,
    val workspace: String,
    val actionKind: String,
    val command: String,
    val phase: ApprovalPhase = ApprovalPhase.PENDING,
    val submittedChoice: Boolean? = null,
    val unavailableReason: String? = null,
    val queuePosition: Int = 1,
    val queueTotal: Int = 1,
    val options: List<ApprovalOptionUi> = emptyList(),
    val details: String? = null,
    val submittedOption: String? = null,
)

/** 决定按钮是否可用:只有收到请求/待决定两态可点(提交中禁重复,终态/失效禁用)。 */
val ApprovalRequestUi.decideEnabled: Boolean
    get() = phase == ApprovalPhase.RECEIVED || phase == ApprovalPhase.PENDING

/** 重试是否可用:只有 UNAVAILABLE 有"重试"可讲(其余态没有失败可重试)。 */
val ApprovalRequestUi.retryEnabled: Boolean
    get() = phase == ApprovalPhase.UNAVAILABLE

/**
 * 进入提交中:记录用户点的按钮用于单按钮回显;非可决定态调用是**无操作**
 * (纯函数不抛异常,防御调用方双击竞态 —— 第二次点击落在 SUBMITTING 时不改状态)。
 *
 * [option] 是三按钮审批的选项 id(如 acceptForSession);null/空 = 旧两按钮
 * 口径,只记 [ApprovalRequestUi.submittedChoice]。acceptForSession 也是允许:
 * allow=true + option,布尔语义与旧端完全一致。
 */
fun ApprovalRequestUi.submitting(allow: Boolean, option: String? = null): ApprovalRequestUi =
    if (decideEnabled) {
        copy(phase = ApprovalPhase.SUBMITTING, submittedChoice = allow, submittedOption = option)
    } else this

/** 服务端确认(带结果):仅提交中可进;允许/拒绝从此以服务端确认为唯一依据。 */
fun ApprovalRequestUi.serverConfirmed(allow: Boolean): ApprovalRequestUi =
    if (phase == ApprovalPhase.SUBMITTING) {
        copy(phase = ApprovalPhase.CONFIRMED, submittedChoice = allow)
    } else this

/** 确认落地为终态词:CONFIRMED → 已允许/已拒绝;其余态无操作。 */
fun ApprovalRequestUi.settled(): ApprovalRequestUi =
    if (phase == ApprovalPhase.CONFIRMED) {
        copy(phase = if (submittedChoice == true) ApprovalPhase.ALLOWED else ApprovalPhase.DENIED)
    } else this

/**
 * 快路径:提交中直接拿到服务端结果(跳过 CONFIRMED 一拍)。
 * 仅提交中可进 —— PENDING/UNAVAILABLE 上调用是无操作,杜绝对未提交请求的乐观通过。
 */
fun ApprovalRequestUi.confirmed(allow: Boolean): ApprovalRequestUi =
    if (phase == ApprovalPhase.SUBMITTING) {
        copy(phase = if (allow) ApprovalPhase.ALLOWED else ApprovalPhase.DENIED)
    } else this

/**
 * 标记暂无法处理(请求失效/重复/跨会话/超时/提交失败)。可决定态与提交中都可进;
 * 提交中进入时保留 [ApprovalRequestUi.submittedChoice] —— 界面要讲清
 * "上一次尝试没有成功",而不是装作没点过。**任何路径都不会产生 ALLOWED**。
 */
fun ApprovalRequestUi.markUnavailable(reason: String): ApprovalRequestUi =
    if (phase in arrayOf(
            ApprovalPhase.RECEIVED, ApprovalPhase.PENDING,
            ApprovalPhase.SUBMITTING, ApprovalPhase.UNAVAILABLE,
        )
    ) {
        copy(phase = ApprovalPhase.UNAVAILABLE, unavailableReason = reason)
    } else this

/** 重试:UNAVAILABLE → 回到待决定重新校验来源状态;其余态无操作。 */
fun ApprovalRequestUi.retry(): ApprovalRequestUi =
    if (phase == ApprovalPhase.UNAVAILABLE) {
        copy(phase = ApprovalPhase.PENDING, unavailableReason = null)
    } else this

/** 阶段的状态词(组件状态徽章用;措辞对齐四词纪律,不用「错误」「失败」字样)。 */
val ApprovalRequestUi.phaseWord: String
    get() = when (phase) {
        ApprovalPhase.RECEIVED -> "收到审批请求"
        ApprovalPhase.PENDING -> "等待你处理"
        ApprovalPhase.SUBMITTING -> "提交中…"
        ApprovalPhase.CONFIRMED -> "服务端已确认"
        ApprovalPhase.ALLOWED -> "已允许"
        ApprovalPhase.DENIED -> "已拒绝"
        ApprovalPhase.UNAVAILABLE -> "暂无法处理"
    }

/** 阶段的语义色档:待决定=琥珀;提交中/已允许=绿;已拒绝=珊瑚红;暂无法处理=琥珀(可重试的阻断)。 */
val ApprovalRequestUi.phaseTone: EditorialTone
    get() = when (phase) {
        ApprovalPhase.RECEIVED, ApprovalPhase.PENDING -> EditorialTone.ATTENTION
        ApprovalPhase.SUBMITTING, ApprovalPhase.CONFIRMED, ApprovalPhase.ALLOWED -> EditorialTone.RUNNING
        ApprovalPhase.DENIED -> EditorialTone.DENIED
        ApprovalPhase.UNAVAILABLE -> EditorialTone.ATTENTION
    }

/**
 * 多请求到达顺序的数量词:单请求返回空串(不摆"共 1 项"的空壳);
 * [position] 越界按边界收敛(0 → 第 1 项,超总数 → 第 total 项)。
 */
fun approvalQueueLabel(position: Int, total: Int): String {
    if (total <= 1) return ""
    val pos = position.coerceIn(1, total)
    return "共 $total 项待审批 · 当前第 $pos 项"
}

/**
 * 请求状态 → 形通道(与四词状态同一套 [PaStatusGlyph],纯函数,单测钉死
 * StatusGlyphMappingTest)。状态行因此是**词 + 色 + 形**三通道,色弱也可辨:
 *  - 收到/待决定 = 空心环(等你来填,与「等待你处理」同形);
 *  - 提交中/服务端已确认 = 实心点(进行中;组件层提交中另加转圈,形不替代动);
 *  - 已允许 = 对勾(收尾确认);已拒绝/暂无法处理 = 叹号(要回头看)。
 */
fun approvalPhaseGlyph(phase: ApprovalPhase): PaStatusGlyph = when (phase) {
    ApprovalPhase.RECEIVED, ApprovalPhase.PENDING -> PaStatusGlyph.RING
    ApprovalPhase.SUBMITTING, ApprovalPhase.CONFIRMED -> PaStatusGlyph.DOT
    ApprovalPhase.ALLOWED -> PaStatusGlyph.CHECK
    ApprovalPhase.DENIED, ApprovalPhase.UNAVAILABLE -> PaStatusGlyph.EXCLAMATION
}

// ---------------------------------------------------------------- 危险分级

/**
 * 危险分级档(审批弹窗风险区的输入):**常规**(读类,默认安静)/ **高危**
 * (删除、覆盖写入、网络外发、提升权限 —— 弹窗给出醒目但可读的风险条)。
 *
 * 【分级只影响提示强度,不影响可达性】高危与常规的决定按钮(尤其「拒绝」)
 * 完全同等可达 —— 分级回答"要多大声提醒",绝不回答"要不要拦着你不许点"。
 */
enum class ApprovalRisk { ROUTINE, HIGH }

/** 危险分级结果:[risk] 为高危时 [reasons] 非空(命中原因,去重、固定顺序)。 */
data class ApprovalRiskSignal(val risk: ApprovalRisk, val reasons: List<String>)

// 命令级危险特征(大小写不敏感;词表保守取向:宁可多提醒一句,不给破坏性
// 操作静默放行 —— 假阳性的代价是用户多看一眼命令原文,假阴性的代价是数据)。
private val ApprovalRiskDelete = Regex(
    """\b(rm|rmdir|unlink|shred|del|rd|trash)\b|git\s+clean|删除""",
    RegexOption.IGNORE_CASE,
)
private val ApprovalRiskOverwrite = Regex(
    // >>? = 单个 > 或 >>(重定向写入都算);曾误写 >>?> 漏掉单 > 重定向(ApprovalRiskTest 钉住)
    """>>?|\b(tee|dd|mv|cp)\b|sed\s+-i|mkfs|--force|覆盖""",
    RegexOption.IGNORE_CASE,
)
private val ApprovalRiskNetwork = Regex(
    """\b(curl|wget|nc|netcat|ssh|scp|sftp|rsync)\b|https?://|ftp://|git\s+(push|pull|fetch)|\b(pip3?|npm|yarn|pnpm|brew|apt-get|apt)\s+(install|add|i\b|upgrade)""",
    RegexOption.IGNORE_CASE,
)
private val ApprovalRiskPrivilege = Regex(
    """\b(sudo|doas|su|runas|chmod|chown|pkexec)\b""",
    RegexOption.IGNORE_CASE,
)

/**
 * 危险分级判定(纯函数,单测钉死 ApprovalRiskTest)。
 *
 * 两个信号源:**操作类型词**(写入/补丁/改动文件类、提权类,来自 daemon 的
 * action 名或其人话标签 —— 「写入文件 · 仅本次」这类后缀先剥掉)与**命令原文**
 * (删除/覆盖/网络外发/提权特征词)。命中任意一条即高危;原因按固定顺序去重,
 * 弹窗用「 · 」并列展示。
 */
fun approvalRiskOf(actionKind: String, command: String): ApprovalRiskSignal {
    val kind = actionKind.substringBefore('·').trim()
    val reasons = LinkedHashSet<String>()
    val writeKind = kind.contains("写入") || kind.contains("补丁") || kind.contains("修改文件") ||
        kind.equals("write_file", true) || kind.equals("apply_patch", true) || kind.equals("file_change", true)
    val privilegeKind = kind.contains("提升权限") || kind.contains("提权") || kind.equals("permissions", true)
    if (writeKind) reasons.add("覆盖或改动文件")
    if (ApprovalRiskDelete.containsMatchIn(command)) reasons.add("删除文件")
    if (ApprovalRiskOverwrite.containsMatchIn(command)) reasons.add("覆盖或写入")
    if (ApprovalRiskNetwork.containsMatchIn(command)) reasons.add("对外网络请求")
    if (privilegeKind || ApprovalRiskPrivilege.containsMatchIn(command)) reasons.add("提升权限")
    return ApprovalRiskSignal(
        risk = if (reasons.isEmpty()) ApprovalRisk.ROUTINE else ApprovalRisk.HIGH,
        reasons = reasons.toList(),
    )
}

/**
 * 危险分级 → 风险条的视觉态(纯函数,单测钉死 ApprovalRiskTest)。
 *
 * 【分级只变音量,不变可达性】高危 = 珊瑚警示条(词「高危操作」+ 命中原因并列,
 * 原因用「 · 」连接,读屏一次念全);常规 = 安静一行(「常规操作」,组件层再缀
 * 一句核对原文的提示,不摆红色吓唬读类操作)。两种档位下决定按钮(尤其拒绝)
 * 尺寸、顺序、可用性完全一致 —— 这里的输出只描述提示条长什么样。
 */
data class ApprovalRiskVisual(
    /** true = 高危警示条(珊瑚);false = 常规安静行(中性)。 */
    val alert: Boolean,
    /** 风险条标题词:高危操作 / 常规操作。 */
    val headline: String,
    /** 高危命中原因,「 · 」并列;常规档为 null(不摆空壳)。 */
    val reasonsLabel: String?,
)

/** 风险条视觉态映射:[ApprovalRiskSignal] → 警示条/安静行的词与原因并列文本。 */
fun approvalRiskVisualOf(signal: ApprovalRiskSignal): ApprovalRiskVisual =
    if (signal.risk == ApprovalRisk.HIGH) {
        ApprovalRiskVisual(
            alert = true,
            headline = "高危操作",
            reasonsLabel = signal.reasons.joinToString(" · "),
        )
    } else {
        ApprovalRiskVisual(alert = false, headline = "常规操作", reasonsLabel = null)
    }

// ---------------------------------------------------------------- 三按钮选项(C1)

/**
 * 审批卡上的一个可选项(来自 daemon ApprovalRequest.options,codex 0.159.2
 * 三按钮审批增量)。[id] 是 codex 应答枚举原词(accept / acceptForSession /
 * decline,旧词 approved / approved_for_session / denied),应答时**原样**回填
 * ApprovalDecision.option,由 daemon 按协议版本翻译 —— 手机端不做词形转换。
 */
data class ApprovalOptionUi(
    val id: String,
    val label: String = "",
    val description: String = "",
)

/** daemon proto 选项 → UI 值对象(调用方装配 [ApprovalRequestUi.options] 用)。 */
fun List<pocketagent.v1.ApprovalOption>.toApprovalOptionUi(): List<ApprovalOptionUi> =
    map { ApprovalOptionUi(id = it.id, label = it.label, description = it.description) }

/** 选项按钮的语义色档:主允许(石墨实底)/ 次允许(浅底描边)/ 拒绝(红)。 */
enum class ApprovalOptionRole { PRIMARY_ALLOW, SECONDARY_ALLOW, DENY }

/** 渲染用的一颗选项按钮:[id] 应答原词、[label] 钉死文案、[role] 决定色档与应答通道。 */
data class ApprovalOptionButton(
    val id: String,
    val label: String,
    val role: ApprovalOptionRole,
)

// deny 类特征词:decline / denied / deny / reject(大小写不敏感;旧词 denied 也命中)。
// 不命中才按允许类渲染 —— 宁可红得保守,不给破坏性选项穿允许色的外衣。
// 【I2 修复】原词表 "deni|declin|reject" 漏了 deny 本形(d-e-n-y 第四字母是 y 不是 i),
// "DenyOnce" 这类自定义 deny 选项被误判成允许类(ApprovalThreeOptionTest 钉住)。
private val ApprovalOptionDenyPattern = Regex("deny|deni|declin|reject", RegexOption.IGNORE_CASE)

// 三个已知 id(与旧词枚举)的钉死文案与色档(手机端语序:主按钮最短最常用;
// "本会话允许"讲清作用域;旧词 approved 系列是同一语义的历史拼写,同色同文。
// cancel 也是合法应答但**不作第三按钮广告**,不在映射表里)。
private val KnownOptionRoles = mapOf(
    "accept" to ApprovalOptionRole.PRIMARY_ALLOW,
    "approved" to ApprovalOptionRole.PRIMARY_ALLOW,
    "acceptForSession" to ApprovalOptionRole.SECONDARY_ALLOW,
    "approved_for_session" to ApprovalOptionRole.SECONDARY_ALLOW,
)
private val KnownOptionLabels = mapOf(
    "accept" to "本次允许",
    "approved" to "本次允许",
    "acceptForSession" to "本会话允许",
    "approved_for_session" to "本会话允许",
    "decline" to "拒绝",
    "denied" to "拒绝",
)

/**
 * options → 按钮序列(纯函数,单测钉死 ApprovalThreeOptionTest)。
 *
 * 【映射规则】已知 id 用钉死文案与色档:accept(旧词 approved)=主允许「本次允许」/
 * acceptForSession(旧词 approved_for_session)=次允许「本会话允许」/
 * decline 与一切 deny 类(旧词 denied)=红「拒绝」;未知 id 不猜语义,文案用
 * daemon 给的 label(空则回退 id),deny 特征词命中的按拒绝渲染,其余按次允许渲染。
 * 【排序】允许类在前、拒绝殿后,同类保持 daemon 顺序(stable)—— 卡片上
 * 先给出口、再给刹车,与旧两按钮"左拒右许"同一空间语言。
 * 【空 options = 旧 daemon】返回空列表,组件据此走旧两按钮,行为一字不变。
 */
fun approvalOptionButtonsOf(options: List<ApprovalOptionUi>): List<ApprovalOptionButton> =
    options.map { o ->
        val denyLike = ApprovalOptionDenyPattern.containsMatchIn(o.id) ||
            ApprovalOptionDenyPattern.containsMatchIn(o.label)
        ApprovalOptionButton(
            id = o.id,
            label = KnownOptionLabels[o.id] ?: o.label.ifBlank { o.id },
            role = KnownOptionRoles[o.id]
                ?: if (denyLike) ApprovalOptionRole.DENY else ApprovalOptionRole.SECONDARY_ALLOW,
        )
    }.sortedByDescending { it.role != ApprovalOptionRole.DENY } // 稳定排序:允许在前,拒绝殿后

/**
 * 该选项按钮是否处于「提交中」单按钮回显(纯函数,单测钉死)。只有用户点中的
 * 那颗转圈,其余保持可读 —— 与旧两按钮的 [ApprovalRequestUi.submittedChoice]
 * 纪律一致:[submittedOption] 空(旧口径)时允许类回显落主按钮、拒绝类按
 * submittedChoice=false 回显;有选项 id 时严格按 id 落位,绝不两颗同转。
 */
fun approvalOptionEcho(
    button: ApprovalOptionButton,
    phase: ApprovalPhase,
    submittedChoice: Boolean?,
    submittedOption: String?,
): Boolean {
    if (phase != ApprovalPhase.SUBMITTING) return false
    return if (submittedOption.isNullOrBlank()) {
        // 旧口径(没带选项 id = 点的主允许/拒绝):回显只落主允许或拒绝,次允许不亮
        (button.role == ApprovalOptionRole.PRIMARY_ALLOW && submittedChoice == true) ||
            (button.role == ApprovalOptionRole.DENY && submittedChoice == false)
    } else {
        button.id == submittedOption
    }
}
