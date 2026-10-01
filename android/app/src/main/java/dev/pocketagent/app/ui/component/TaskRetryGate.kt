package dev.pocketagent.app.ui.component

/**
 * 任务中心「重试语义」的纯逻辑(R3-C · 不确定时刻的诚实)。
 *
 * 【它钉住两件事】
 *  1. **重试防重入**:每个失败态的重试/重新下载点击,冷却窗口内的第二发一律拒收 ——
 *     不双击重入(消息类双击会连发两次结果找回;下载类双击会并发两条同名下载流)。
 *     门闩只是呈现层纪律,不改变任何底层重试链路。
 *  2. **重新下载的可行性分类**:任务条目只存了 fileKey(id 去掉 `dl:` 前缀),
 *     能不能"再取一次"取决于来源是什么 —— URL 与电脑路径可以重取;
 *     网页 blob 落盘 / 会话文本另存这两类在手机侧没有可重放的来源,
 *     **不能摆一个点了没反应的「重新下载」死按钮**,只能引导回会话或移除条目。
 *
 * 纯逻辑、无 Android 依赖,单测钉死(TaskRetryGateTest)。
 */

/** 重试冷却默认时长:视觉上足够让"重试中"被看见,又短到不拖慢真实重试。 */
const val TASK_RETRY_COOLDOWN_MS = 1500L

/**
 * 重试防重入门闩(纯逻辑):[tryBegin] 放行第一次点击并记时,冷却窗口内的
 * 后续调用一律返回 false;[inFlight] 给"重试中"视觉态(窗口内为 true)。
 * 时钟由调用方注入(生产传默认钟,单测传毫秒数),不依赖真实睡眠。
 */
class RetryGate(val cooldownMs: Long = TASK_RETRY_COOLDOWN_MS) {

    /** 上次放行的时刻;初值取负冷却,保证第一次点击恒放行(也不会溢出)。 */
    @Volatile
    private var lastAllowedAt: Long = -cooldownMs

    /** 是否放行这次点击:放行即记录时刻。 */
    fun tryBegin(nowMs: Long = System.currentTimeMillis()): Boolean {
        if (nowMs - lastAllowedAt < cooldownMs) return false
        lastAllowedAt = nowMs
        return true
    }

    /** 距上次放行是否仍在冷却窗口内(调用方据此显示"重试中"并禁点)。 */
    fun inFlight(nowMs: Long = System.currentTimeMillis()): Boolean =
        nowMs - lastAllowedAt < cooldownMs
}

/**
 * 重新下载的可行性分类(由下载任务的 fileKey 推断,见 [redownloadPlanOf])。
 *
 * 【为什么宁缺毋滥】fileKey 是登记时的原始键,文本另存的键是 `threadId/文件名`,
 * 与电脑端**相对路径**形态上无法区分 —— 与其摆一个必然失败的「重新下载」,
 * 不如归为 NONE 引导回会话(内容还在会话里)或移除条目。电脑端下载的键
 * 实际上几乎都是绝对路径,分类覆盖得住真实场景。
 */
enum class RedownloadPlan {
    /** 来源是网页 URL:重新下载只需要手机有网,与电脑连接无关。 */
    FROM_URL,

    /** 来源是电脑上的文件:重新下载需要 daemon 连接在线。 */
    FROM_PC,

    /** 没有可重放的来源(blob 落盘 / 会话文本另存):不摆「重新下载」。 */
    NONE,
}

/**
 * fileKey → 重新下载方案(纯函数)。
 * 判定顺序:URL 前缀 → Windows 盘符/反斜杠 → POSIX 绝对路径 → 其余一律 NONE。
 */
fun redownloadPlanOf(fileKey: String): RedownloadPlan = when {
    fileKey.startsWith("http://") || fileKey.startsWith("https://") -> RedownloadPlan.FROM_URL
    fileKey.contains('\\') -> RedownloadPlan.FROM_PC
    Regex("^[A-Za-z]:[\\\\/]").containsMatchIn(fileKey) -> RedownloadPlan.FROM_PC
    fileKey.startsWith("/") -> RedownloadPlan.FROM_PC
    else -> RedownloadPlan.NONE
}

/**
 * 重新下载对连接的依赖(null = 没有可再取的来源):
 * FROM_URL 只要有网;FROM_PC 必须连上电脑 —— 失败态的"下一步"建议据此分流,
 * 断线时绝不引导用户去点一个此刻注定失败的按钮。
 */
fun redownloadNeedsPc(plan: RedownloadPlan): Boolean? = when (plan) {
    RedownloadPlan.FROM_URL -> false
    RedownloadPlan.FROM_PC -> true
    RedownloadPlan.NONE -> null
}
