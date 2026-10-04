package dev.pocketagent.app.ui.voice

import kotlinx.coroutines.flow.SharedFlow

// =====================================================================================
// P4 Task 5 —— 语音引擎适配接口
//
// 引擎只做"收音→转写"这一件事,全部回调走 [events] 冷热流,每个事件带 sessionId;
// 状态机、看门狗、静音门都在 reducer/Controller 层 —— 引擎生命周期与 Compose 重组
// 彻底解耦(Global Constraint:语音引擎生命周期不能依赖 Compose 重组)。
// =====================================================================================

/**
 * 语音引擎回调事件(引擎 → Controller;Controller 翻译成 [VoiceEvent] 喂 reducer)。
 * 每个事件携带产生它的 sessionId,Controller/reducer 据此丢弃迟到回调。
 */
internal sealed interface VoiceEngineEvent {
    /** 收音就绪(系统:onReadyForSpeech;端侧:AudioRecord 进入录音态)= 引擎活着的首个证明。 */
    data class Ready(val sessionId: Long) : VoiceEngineEvent

    /** 中间转写(系统:onPartialResults;端侧暂停 flush 的累计文本)。 */
    data class Partial(val sessionId: Long, val text: String) : VoiceEngineEvent

    /** 采音结束、转写开始(系统:onEndOfSpeech;端侧:转写段首个喂狗点)。 */
    data class Transcribing(val sessionId: Long) : VoiceEngineEvent

    /** 最终转写交付(一轮至多一次;引擎聚合多段文本后整体交付)。 */
    data class Final(val sessionId: Long, val text: String) : VoiceEngineEvent

    /** 失败(kind 已映射为领域错误分类;展示层按 kind 取人话)。 */
    data class Failed(val sessionId: Long, val kind: VoiceErrorKind) : VoiceEngineEvent
}

/**
 * 语音识别引擎最小接口。
 *
 * 【会话契约】所有回调必须带回 [start] 收到的 sessionId;一轮 = start → (pause/resume)*
 * → stop/cancel → Final/Failed 收尾。pause/resume 不结束会话:引擎把暂停时的部分文本
 * 以 Partial 形式 flush 并入累计,恢复后在同一 sessionId 下继续收音(系统引擎重新
 * startListening,端侧重新起采音轮),最终 Final 携带全部累计文本。
 *
 * 【为什么不只有 start/stop/pause/resume】[cancel] 与 [destroy] 是计划全局约束
 * ("离开页面必须明确停止或静音""识别器销毁防泄漏")的直接要求:cancel 放弃本轮
 * (未交付结果一律吞掉),destroy 释放底层资源(页面级,引擎实例不可复用)。
 */
internal interface VoiceEngine {
    /** 引擎类别(决策表选型结果;SYSTEM_ON_DEVICE/SYSTEM_REMOTE 在工厂预检后确定)。 */
    val kind: VoiceEngineKind

    /** 回调流;事件必须从主线程语义发射(两个实现都由主线程回调驱动)。 */
    val events: SharedFlow<VoiceEngineEvent>

    /** 开始一轮收音;@param sessionId 本轮回调的会话锚。 */
    fun start(sessionId: Long)

    /** 暂停:停收音但不结束会话,已收文本以 Partial flush。 */
    fun pause()

    /** 恢复:同一 sessionId 继续收音(文本累计不清)。 */
    fun resume()

    /** 停止收音并交付最终转写(一轮的正常收尾)。 */
    fun stop()

    /** 放弃本轮:未交付的转写一律吞掉,绝不发出 Final。 */
    fun cancel()

    /** 释放底层资源;调用后实例不可再用(页面销毁/换引擎时)。 */
    fun destroy()
}
