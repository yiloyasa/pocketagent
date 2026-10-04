package dev.pocketagent.app.ui.voice

// =====================================================================================
// P4 Task 4 —— VoiceSession 归约器(纯 Kotlin,零 Android API 依赖,可 JVM 单测)
//
// 职责边界:reducer 只处理状态(相迁移/文本/错误/静音标记),不调用任何 Android API;
// 引擎的启停由 [reduce] 产出的 [VoiceEffect] 描述,由 VoiceController 照单执行。
// =====================================================================================

/**
 * 空文本 Partial = 引擎进度信号(不含文本):引擎适配层把"收音就绪"(Ready)与
 * "转写开始"(Transcribing)映射为 `Partial(sessionId, "")`,reducer 据此推进相
 * (STARTING→LISTENING、STOPPING→TRANSCRIBING)而不触碰文本。
 * 设计约束:spec 的 VoiceEvent 逐字固定、没有 Ready/Transcribing 事件,进度信号
 * 只能复用 Partial 通道;选空文本是因为它不可能与真实转写混淆(真转写有字)。
 */
internal const val VOICE_PROGRESS_SIGNAL = ""

/**
 * 识别文本长度上限(字符)。
 *
 * 【上限依据】采音时长上限是 60 秒(captureSenseVoiceAudio 的 maxMillis=60_000);
 * 正常中文语速约 5 字/秒 → 单轮 ≈300 字;英文/路径/命令等密集文本按 ~15 字符/秒
 * → ≈900 字符。4_096(2^12 对齐)给足 ≥4 倍余量,远高于任何真实听写,同时保证
 * 异常识别服务刷屏时输入框与 effect 队列都有硬界(计划检查项:识别文本必须有
 * 长度上限,Channel.UNLIMITED 只保留为受控 effect 队列)。截断保留头部 —— 用户
 * 最先说出的话在开头,丢尾部比丢开头危害小。
 */
internal const val VOICE_TEXT_MAX_CHARS = 4_096

/** 超长识别文本截断(保留头部,依据见 [VOICE_TEXT_MAX_CHARS])。 */
internal fun truncateVoiceText(text: String): String =
    if (text.length <= VOICE_TEXT_MAX_CHARS) text else text.take(VOICE_TEXT_MAX_CHARS)

/** 待引擎执行的一次性指令(纯数据,Controller 照单驱动 [VoiceEngine])。 */
internal sealed interface VoiceEffect {
    /** 启动引擎开始收音;参数 = 本次会话 id,引擎所有回调必须原样带回它。 */
    data class StartEngine(val sessionId: Long) : VoiceEffect
    data object PauseEngine : VoiceEffect
    data object ResumeEngine : VoiceEffect
    data object StopEngine : VoiceEffect
    data object CancelEngine : VoiceEffect
}

/** 一次归约的结果:新状态 + 至多一条待执行引擎指令(事件被拒/被丢弃时 effect=null)。 */
internal data class VoiceReduction(val state: VoiceSessionState, val effect: VoiceEffect? = null)

/**
 * 状态迁移合法性表(行=当前相,列=事件;「-」=非法,如实拒绝并保持原状态,effect=null)。
 *
 * | 事件\相           | IDLE | REQ_PERM | STARTING | LISTENING | PAUSED | STOPPING | TRANSCRIBING | ERROR |
 * |------------------|------|----------|----------|-----------|--------|----------|--------------|-------|
 * | Start            | →REQ_PERM(新会话) | - | - | - | →LISTENING(Resume) | - | - | →REQ_PERM(新会话) |
 * | Permission(true) | - | →STARTING(StartEngine) | - | - | - | - | - | - |
 * | Permission(false)| - | →ERROR(权限) | - | - | - | - | - | - |
 * | Pause            | - | - | - | →PAUSED | - | - | - | - |
 * | Stop             | - | →IDLE(Cancel,空请求即放弃) | →STOPPING | →STOPPING | →STOPPING | - | - | - |
 * | Cancel           | - | →IDLE | →IDLE | →IDLE | →IDLE | →IDLE | →IDLE | →IDLE(清错误) |
 * | Partial 非空     | - | - | →LISTENING | 更新文本 | 更新文本(flush) | - | 更新文本 | - |
 * | Partial 空(进度) | - | - | →LISTENING | - | - | →TRANSCRIBING | - | - |
 * | Final            | - | - | →IDLE | →IDLE | →IDLE | →IDLE | →IDLE | - |
 * | Error            | - | →ERROR | →ERROR | →ERROR | →ERROR | →ERROR | →ERROR | 更新 kind |
 * | SessionSwitched  | 置静音 | 置静音+→IDLE(Cancel) | 置静音+→IDLE(Stop) | 同左 | 同左 | 同左 | 同左 | 置静音+→IDLE |
 *
 * 【设计取舍】
 *  · Error 一律落 ERROR 相(计划规则逐字);NO_MATCH("没听到")等业务性失败也走
 *    ERROR 相,展示层按 kind 决定口吻,下一轮 Start 照常从 ERROR 重启。
 *  · PAUSED 里的 Start 是"恢复"(同一会话,sessionId 不变,引擎 resume);IDLE/ERROR
 *    里的 Start 是"新会话"(sessionId 由 Controller 递增分配,文本清空,静音解除)。
 *  · 静音期(SessionSwitched 之后)会话回调一律丢弃:旧识别器残余的文本/错误不得
 *    进入新会话(Global Constraint + Review Focus 红线);Start 是用户在新会话的
 *    主动动作,顺带解除静音。
 */
internal fun reduce(current: VoiceSessionState, event: VoiceEvent): VoiceReduction {
    // 门 1:静音门 —— 切会话后,旧引擎残余的识别回调(文本/错误)直接丢弃。
    if (current.mutedByThreadSwitch && event.isSessionCallback()) return VoiceReduction(current)
    // 门 2:sessionId 门 —— 识别回调必须属于当前会话;旧 sessionId = 迟到结果,直接丢弃。
    //     (与门 1 互为双保险:静音在下一轮 Start 解除后,旧 sessionId 的迟到事件仍被本门拦住。)
    val sid = event.sessionIdOrNull()
    if (sid != null && sid != current.sessionId) return VoiceReduction(current)
    return reduceOpen(current, event)
}

/** 事件是否为"引擎/会话回调"(受静音门与 sessionId 门约束的事件)。 */
private fun VoiceEvent.isSessionCallback(): Boolean = when (this) {
    is VoiceEvent.Partial, is VoiceEvent.Final, is VoiceEvent.Error -> true
    else -> false
}

/** 事件携带的会话锚(无则 null),供静音门/sessionId 门与 Controller 喂狗判定共用。 */
internal fun VoiceEvent.sessionIdOrNull(): Long? = when (this) {
    is VoiceEvent.Partial -> sessionId
    is VoiceEvent.Final -> sessionId
    is VoiceEvent.Error -> sessionId
    else -> null
}

private fun reduceOpen(current: VoiceSessionState, event: VoiceEvent): VoiceReduction = when (event) {
    is VoiceEvent.Start -> when (current.phase) {
        VoicePhase.IDLE, VoicePhase.ERROR ->
            VoiceReduction(current.freshSession())
        VoicePhase.PAUSED ->
            VoiceReduction(current.copy(phase = VoicePhase.LISTENING), VoiceEffect.ResumeEngine)
        else -> VoiceReduction(current) // 已在进行中的相再点开始:非法,拒绝
    }

    is VoiceEvent.Permission -> when {
        current.phase != VoicePhase.REQUESTING_PERMISSION -> VoiceReduction(current)
        event.granted ->
            VoiceReduction(current.copy(phase = VoicePhase.STARTING), VoiceEffect.StartEngine(current.sessionId))
        // 计划规则:Permission(false) → ERROR 相 + 权限 kind(人话指引在展示层按 kind 取)
        else ->
            VoiceReduction(
                current.copy(phase = VoicePhase.ERROR, error = VoiceErrorKind.INSUFFICIENT_PERMISSIONS),
            )
    }

    is VoiceEvent.Pause ->
        if (current.phase == VoicePhase.LISTENING) {
            VoiceReduction(current.copy(phase = VoicePhase.PAUSED), VoiceEffect.PauseEngine)
        } else {
            VoiceReduction(current)
        }

    is VoiceEvent.Stop -> when (current.phase) {
        // 还在等权限就点了停止:没有引擎可停,等价放弃请求
        VoicePhase.REQUESTING_PERMISSION ->
            VoiceReduction(current.copy(phase = VoicePhase.IDLE, partialText = "", finalText = "", error = null), VoiceEffect.CancelEngine)
        VoicePhase.STARTING, VoicePhase.LISTENING, VoicePhase.PAUSED ->
            VoiceReduction(current.copy(phase = VoicePhase.STOPPING), VoiceEffect.StopEngine)
        else -> VoiceReduction(current) // 已在收尾(IDLE/STOPPING/TRANSCRIBING/ERROR)再点停止:拒绝
    }

    is VoiceEvent.Cancel -> when (current.phase) {
        VoicePhase.IDLE -> VoiceReduction(current)
        // 取消 = 放弃本轮:丢弃草稿文本与错误;引擎侧由 cancel 吞掉未交付结果
        else ->
            VoiceReduction(
                current.copy(phase = VoicePhase.IDLE, partialText = "", finalText = "", error = null),
                VoiceEffect.CancelEngine,
            )
    }

    is VoiceEvent.Partial -> {
        val progressOnly = event.text == VOICE_PROGRESS_SIGNAL
        when {
            // 进度信号:推进相,不动文本(STARTING 里收到首个回调=引擎活着;STOPPING 里收到=转写开始)
            progressOnly && current.phase == VoicePhase.STARTING ->
                VoiceReduction(current.copy(phase = VoicePhase.LISTENING))
            progressOnly && current.phase == VoicePhase.STOPPING ->
                VoiceReduction(current.copy(phase = VoicePhase.TRANSCRIBING))
            progressOnly && (current.phase == VoicePhase.LISTENING || current.phase == VoicePhase.PAUSED || current.phase == VoicePhase.TRANSCRIBING) ->
                VoiceReduction(current)
            // 真文本:STARTING 首个带字 Partial 同时证明引擎存活并给出文本
            !progressOnly && (current.phase == VoicePhase.STARTING || current.phase == VoicePhase.LISTENING) ->
                VoiceReduction(current.copy(phase = VoicePhase.LISTENING, partialText = truncateVoiceText(event.text)))
            // 暂停期 flush / 转写期的补充文本:只更新文本,相不动
            !progressOnly && (current.phase == VoicePhase.PAUSED || current.phase == VoicePhase.TRANSCRIBING) ->
                VoiceReduction(current.copy(partialText = truncateVoiceText(event.text)))
            // IDLE/REQ_PERM/ERROR 收到文本:非法,拒绝
            else -> VoiceReduction(current)
        }
    }

    is VoiceEvent.Final -> when (current.phase) {
        VoicePhase.STARTING, VoicePhase.LISTENING, VoicePhase.PAUSED, VoicePhase.STOPPING, VoicePhase.TRANSCRIBING ->
            // reducer 只存;"并入输入框"的追加语义在 Controller/面板层(Task 6)
            VoiceReduction(
                current.copy(phase = VoicePhase.IDLE, finalText = truncateVoiceText(event.text), partialText = ""),
            )
        else -> VoiceReduction(current) // IDLE/REQ_PERM/ERROR 收 Final:非法/迟到,拒绝
    }

    is VoiceEvent.Error -> when {
        current.phase == VoicePhase.IDLE -> VoiceReduction(current) // 会话已关闭后的错误:迟到,拒绝
        current.phase == VoicePhase.ERROR ->
            VoiceReduction(current.copy(error = event.kind)) // 已在 ERROR:最新分类覆盖
        else -> VoiceReduction(current.copy(phase = VoicePhase.ERROR, error = event.kind))
    }

    is VoiceEvent.SessionSwitched -> {
        val muted = current.copy(
            threadId = event.newThreadId,
            mutedByThreadSwitch = true,
            partialText = "",
            finalText = "",
            error = null,
        )
        when (current.phase) {
            VoicePhase.IDLE, VoicePhase.ERROR -> VoiceReduction(muted.copy(phase = VoicePhase.IDLE))
            VoicePhase.REQUESTING_PERMISSION ->
                VoiceReduction(muted.copy(phase = VoicePhase.IDLE), VoiceEffect.CancelEngine)
            // 计划规则:切会话即停录;残余 Final 由静音门丢弃,绝不落新会话
            VoicePhase.STARTING, VoicePhase.LISTENING, VoicePhase.PAUSED, VoicePhase.STOPPING, VoicePhase.TRANSCRIBING ->
                VoiceReduction(muted.copy(phase = VoicePhase.IDLE), VoiceEffect.StopEngine)
        }
    }
}

/** 新会话的初始状态:清空上一轮文本与错误,解除静音;sessionId 由 Controller 递增分配。 */
private fun VoiceSessionState.freshSession(): VoiceSessionState = copy(
    phase = VoicePhase.REQUESTING_PERMISSION,
    partialText = "",
    finalText = "",
    error = null,
    mutedByThreadSwitch = false,
)

/**
 * 死区看门狗触发判定(纯函数,JVM 单测锁定;VoiceController 的 8 秒兜底据此开火)。
 *
 * 与旧路径 ui.voiceDeadZoneTimedOut 同语义,额外钉死两条守卫:
 *  1. 会话守卫 —— 只有"当前会话仍是被武装的那一轮"才可触发(start 之后用户又 Cancel/
 *     新开一轮,迟到的超时不得误伤新会话);
 *  2. 相守卫 —— 只在 STARTING/LISTENING(等引擎首个回调的窗口)判死;STOPPING 在等
 *     转写交付、PAUSED 是用户主动暂停,都不算死区(Controller 对"零回调就 Stop"另有
 *     立即收口,防卡 STOPPING)。
 * elapsedMs 必须来自 monotonic 时钟(nanoTime/elapsedRealtime),见 VoiceController。
 */
internal fun voiceDeadZoneShouldFire(
    elapsedMs: Long,
    gotCallback: Boolean,
    currentPhase: VoicePhase,
    currentSessionId: Long,
    armedSessionId: Long,
    timeoutMs: Long = VOICE_DEAD_ZONE_TIMEOUT_MS,
): Boolean =
    !gotCallback && elapsedMs >= timeoutMs &&
        currentSessionId == armedSessionId &&
        (currentPhase == VoicePhase.STARTING || currentPhase == VoicePhase.LISTENING)
