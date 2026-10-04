package dev.pocketagent.app.ui.voice

// =====================================================================================
// P4 Task 4 —— VoiceSession 领域状态(纯 Kotlin,零 Android API 依赖,可 JVM 单测)
//
// 本文件是语音会话的领域词汇表:状态([VoiceSessionState])、事件([VoiceEvent])、
// 相([VoicePhase])与引擎类别([VoiceEngineKind])均逐字按 implementation plan 的
// spec 接口块定义;识别回调先进入事件流,再由 [reduce](VoiceSessionReducer.kt)
// 生成展示状态与一次性 effects —— 语音引擎生命周期从此不依赖 Compose 重组
// (Global Constraint:离开页面必须明确停止或静音)。
// =====================================================================================

/** 语音识别引擎类别(spec 定义,逐字)。 */
enum class VoiceEngineKind { SYSTEM_ON_DEVICE, SYSTEM_REMOTE, SENSEVOICE, UNAVAILABLE }

/**
 * 语音会话相(spec 定义,逐字,八态):
 *  [IDLE] 空闲,无会话;
 *  [REQUESTING_PERMISSION] 已请求开始,正在等 RECORD_AUDIO 运行时权限回答;
 *  [STARTING] 权限已过、引擎启动中(识别器创建/麦克风初始化/端侧冷加载前段);
 *  [LISTENING] 收音中(引擎首个回调已到 —— 空文本 Partial 进度信号,见 reducer);
 *  [PAUSED] 暂停:收音已停但会话保持,恢复(Start)继续,文本累计不清;
 *  [STOPPING] 停止已请求,等引擎收音收尾并交付最终转写;
 *  [TRANSCRIBING] 采音结束、端侧模型转写中(SenseVoice 停止后秒级"模型加载+转写"窗口);
 *  [ERROR] 本轮失败([error] 给出分类;新一轮 Start 从 ERROR 直接重启)。
 */
enum class VoicePhase { IDLE, REQUESTING_PERMISSION, STARTING, LISTENING, PAUSED, STOPPING, TRANSCRIBING, ERROR }

/**
 * 语音识别失败分类(领域层自持;与旧路径 ChatScreen 的 ui.VoiceErrorKind 同名同序对齐,
 * 另新增端侧三类失败与死区 —— 波 3 接线时由引擎适配层搬运映射,不复用旧枚举,
 * 避免 ui.voice 反向依赖 8000 行的 ChatScreen.kt)。
 *
 * 【新增五类】
 *  [DEAD_ZONE] 死区:预检可用但 start 后 8 秒(monotonic)零回调,个别 ROM 的识别服务
 *              实际"死区",不兜底 UI 会永远卡在收音相;
 *  [MODEL_LOADING] 端侧 SenseVoice 冷加载撞看门狗:真相是"正在加载模型"而非服务死区,
 *              文案分流用(旧路径 firstColdLoad 语义的领域化);
 *  [MODEL_CORRUPTED] 端侧模型损坏/校验失败 → 人话:"请重新安装 App";
 *  [STORAGE_FULL] 磁盘空间不足(模型解压写出失败)→ 人话:"清理存储后重试";
 *  [INSUFFICIENT_PERMISSIONS] 沿用旧语义:运行时权限拒绝与系统识别服务自身无权限(9)
 *              共用,人话给"去哪里开"的可行动指引。
 */
enum class VoiceErrorKind {
    NO_MATCH,
    BUSY,
    INSUFFICIENT_PERMISSIONS,
    CLIENT,
    OTHER,
    ENGINE,
    MIC_BLOCKED,
    DEAD_ZONE,
    MODEL_LOADING,
    MODEL_CORRUPTED,
    STORAGE_FULL,
}

/**
 * 语音会话状态(spec 定义,逐字;仅为易用补默认值,字段名/类型/顺序未动)。
 *
 * [sessionId] 是防迟到污染的锚:识别回调(Partial/Final/Error)必须带回当前
 * sessionId,旧 sessionId 的事件在 reducer 直接丢弃 —— 迟到的最终转写绝不写入
 * 新会话(Review Focus 红线,双保险:sessionId 门 + [mutedByThreadSwitch] 静音门)。
 * [finalText] 由 reducer 只存不追加;"Final 并入输入框"的追加/替换语义在
 * Controller/面板层(Task 6)。
 */
data class VoiceSessionState(
    val sessionId: Long = 0L,
    val threadId: String? = null,
    val phase: VoicePhase = VoicePhase.IDLE,
    val engine: VoiceEngineKind = VoiceEngineKind.UNAVAILABLE,
    val partialText: String = "",
    val finalText: String = "",
    val error: VoiceErrorKind? = null,
    val mutedByThreadSwitch: Boolean = false,
)

/**
 * 语音会话事件(spec 定义,逐字)。
 *
 * 带会话锚的事件:[Partial]/[Final]/[Error] 必须携带产生它的 sessionId;
 * [SessionSwitched] 触发静音门;[Permission] 是权限申请的回答。
 */
sealed interface VoiceEvent {
    data object Start : VoiceEvent
    data object Pause : VoiceEvent
    data object Stop : VoiceEvent
    data object Cancel : VoiceEvent
    data class Partial(val sessionId: Long, val text: String) : VoiceEvent
    data class Final(val sessionId: Long, val text: String) : VoiceEvent
    data class Error(val sessionId: Long, val kind: VoiceErrorKind) : VoiceEvent
    data class SessionSwitched(val newThreadId: String) : VoiceEvent
    data class Permission(val granted: Boolean) : VoiceEvent
}
