package dev.pocketagent.app.ui.component

/**
 * 端侧语音引擎(SenseVoice)的共享错误码与异常定义。
 *
 * 历史注:这批定义原落在旧遗留引擎的识别器文件里,2026-09-20 随该引擎整体摘除
 * 迁到本中性位置 —— SenseVoice 链路(识别器/会话执行体)与 ChatScreen 的
 * voiceErrorKind 映射共用,值与语义不变,常量名同步去遗留化(100 号码由
 * 旧名"遗留引擎异常"更名为 [VOICE_ERROR_ENGINE],见 git 历史)。
 *
 * 自定义错误码(落在 SpeechRecognizer 1..9 之外,经 voiceErrorKind 映射到
 * VoiceErrorKind.ENGINE —— "端侧引擎异常")。
 */

/**
 * 端侧引擎运行异常:识别器/录音器创建失败、会话执行体未分类异常等
 * (SenseVoice 会话执行体的 catch(Throwable) 兜底出口)。
 */
internal const val VOICE_ERROR_ENGINE = 100

/**
 * 麦克风不可用错误码:AudioRecord 初始化失败/进不了录音态 —— 典型成因是
 * MIUI 等国产 ROM 在"运行时权限"之外还有独立的「录音」开关(安全中心/权限管理),
 * 或麦克风正被其他应用占用。经 voiceErrorKind 映射到 VoiceErrorKind.MIC_BLOCKED,
 * 人话提示直接给"去哪里开"的可行动指引,不再伪装成引擎异常。
 */
internal const val VOICE_ERROR_MIC_BLOCKED = 102

/**
 * 引擎忙错误码:上一轮会话尚未结束(端侧 SenseVoice 停止后还有秒级
 * "模型加载+转写"空窗)时又请求开始。映射到 VoiceErrorKind.BUSY 给人话提示,
 * 替换旧实现的静默 return(用户点麦克风零反馈的静默失败点之一)。
 * 不复用系统码 8(ERROR_RECOGNIZER_BUSY):组合层把 8 列入"系统服务失败"
 * 名单,引擎侧码混进去会误伤引擎选择逻辑。
 */
internal const val VOICE_ERROR_ENGINE_BUSY = 103

/**
 * 麦克风不可用(初始化失败/进不了录音态)的标记异常:录音器创建处抛出,
 * 由会话执行体 catch 后映射为 [VOICE_ERROR_MIC_BLOCKED]。
 * 是 IllegalStateException 的子类:沿用"非法状态"语义,上层按类型精确捕获。
 */
internal class MicUnavailableException(message: String) : IllegalStateException(message)
