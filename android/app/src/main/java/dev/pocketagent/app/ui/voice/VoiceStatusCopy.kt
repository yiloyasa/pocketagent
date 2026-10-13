package dev.pocketagent.app.ui.voice

// =====================================================================================
// 语音人话状态(快赢版):收音/转写/失败三态的人话收敛 + 引擎优先级声明。
//
// 纯 Kotlin,零 Android API 依赖,JVM 单测钉死(VoiceStatusCopyTest)。
// =====================================================================================

/**
 * 语音会话相 → 人话(纯函数)。
 *
 * 收敛口径:收音中 = 「听着呢」;采音结束后的转写窗口(STOPPING/TRANSCRIBING,
 * 端侧停采后的"模型加载+转写"秒级窗口) = 「转文字中」;本轮失败 = 「失败重试」
 * (点按即开新一轮,错误分类的人话Toast 照旧按 kind 取)。其余相各一句:
 * 空闲 = 待命指引;等权限/启动中 = 准备中;暂停 = 已暂停点按继续。
 * 措辞避开「未知/错误」与英文技术词。
 */
internal fun voicePhaseCopy(phase: VoicePhase): String = when (phase) {
    VoicePhase.LISTENING -> "听着呢"
    VoicePhase.STOPPING, VoicePhase.TRANSCRIBING -> "转文字中"
    VoicePhase.ERROR -> "失败重试"
    VoicePhase.PAUSED -> "已暂停，点按继续"
    VoicePhase.REQUESTING_PERMISSION, VoicePhase.STARTING -> "准备中"
    VoicePhase.IDLE -> "点按开始说话"
}

/**
 * 引擎优先级声明(纯函数,单测钉死)。
 *
 * 麦克风优先走系统识别:系统本机 → 系统在线 → 端侧 → 未就绪。
 * 与 ChatScreen 的 pickVoiceEngine 决策表同序(系统可用一律系统侧),
 * 这里把"系统优先"钉成领域层的显式顺序,装配层照单取首个可用。
 */
internal fun voiceEnginePreferenceOrder(): List<VoiceEngineKind> = listOf(
    VoiceEngineKind.SYSTEM_ON_DEVICE,
    VoiceEngineKind.SYSTEM_REMOTE,
    VoiceEngineKind.SENSEVOICE,
    VoiceEngineKind.UNAVAILABLE
)
