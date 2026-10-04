# PocketAgent 提示词优化与语音输入 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans`. 本计划在 P3 的 UI state/repository 约束稳定后执行；禁止继续把语音和提示词副作用堆回 `ChatScreen.kt`。

**Goal:** 将提示词优化从单按钮替换升级为可预览、可撤销、可选择风格的输入工具；将语音输入从页面胶水升级为独立、可恢复、可测试的 VoiceSession。

**Architecture:** 提示词优化使用 `PromptOptimizeRepository + PreviewState`，原文和候选文本始终分离；语音使用 `VoiceController + VoiceSessionReducer + VoiceEngine`，识别器回调先进入事件流，再由 reducer 生成展示状态和一次性 effects。

**Tech Stack:** Kotlin/Compose/JVM tests、gRPC OptimizePrompt、Android SpeechRecognizer、SenseVoice/Sherpa AAR、现有 UndoBar 和权限系统。

**Spec:** `docs/superpowers/specs/2026-10-04-pocketagent-user-experience-and-reliability-upgrade.md`

## Global Constraints

- 优化失败、超时或空返回不改变原文。
- 优化结果不能自动发送；必须由用户确认。
- 优化指令不得添加原文没有的新需求；检测到明显扩写时必须提示用户复核。
- 语音识别不自动发送，不把迟到结果写入新会话。
- 生产日志不得记录完整语音文本；只记录引擎、耗时、字符数和错误分类。
- 权限失败、系统识别死区和端侧模型初始化失败必须给出可执行人话。
- 语音引擎生命周期不能依赖 Compose 重组；离开页面必须明确停止或静音。

## Review Focus

- 优化请求返回后用户已修改输入框，不能覆盖新输入。
- 快速连续点击优化不会生成多个并发替换结果。
- 优化 Agent/模型不可用时，用户知道是配置问题还是连接问题。
- 语音停止后的迟到回调不会污染新会话。
- 系统识别服务可预检但实际无回调时能切换 SenseVoice。

### Task 1: 接通提示词优化请求字段

**Files:**
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/ChatScreen.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/prompt/PromptOptimizeRepository.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/prompt/PromptOptimizeState.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/PromptOptimizeLogic.kt`
- Test: `android/app/src/test/java/dev/pocketagent/app/ui/PromptOptimizeLogicTest.kt`
- Test: create `PromptOptimizeRepositoryTest.kt`

**Interfaces:**

```kotlin
enum class PromptOptimizeStyle { DEFAULT, CONCISE, DETAILED, IMPLEMENTATION_PLAN, BUG_REPORT, TEST_PLAN, CODE_REVIEW }

data class PromptOptimizeRequestModel(
    val original: String,
    val style: PromptOptimizeStyle,
    val agentId: String,
    val modelId: String,
    val effort: String,
    val requestVersion: Long
)

interface PromptOptimizeRepository {
    suspend fun optimize(request: PromptOptimizeRequestModel): Result<String>
}
```

- [ ] UI 真正设置已有 proto 的 `style` 和 `effort` 字段。
- [ ] 默认 Agent/模型策略明确显示，不再让用户猜优化器和当前会话是否相同。
- [ ] repository 使用独立 deadline，取消时不覆盖输入。
- [ ] test 验证请求参数、取消、空响应和 gRPC 人话错误映射。

### Task 2: 实现优化预览和差异确认

**Files:**
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/prompt/PromptOptimizePreview.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/prompt/PromptOptimizeDiff.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/ChatScreen.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/PromptOptimizeLogic.kt`
- Test: create `PromptOptimizePreviewStateTest.kt`

**Interfaces:**

```kotlin
data class PromptOptimizePreviewState(
    val original: String,
    val candidate: String?,
    val style: PromptOptimizeStyle,
    val loading: Boolean,
    val error: String?,
    val sourceRevision: Long
)

sealed interface PromptOptimizePreviewIntent {
    data object Apply: PromptOptimizePreviewIntent
    data object Retry: PromptOptimizePreviewIntent
    data object Undo: PromptOptimizePreviewIntent
    data object Dismiss: PromptOptimizePreviewIntent
}
```

- [ ] 原文和候选文本分栏或上下对比。
- [ ] 用户可编辑候选文本后再应用。
- [ ] 应用时检查输入框 revision；如果用户在请求期间修改过原文，要求用户选择覆盖或保留新文本。
- [ ] 应用成功后接入现有 UndoBar；撤销只恢复本次原文。
- [ ] 长文本预览有滚动和字符上限，不阻塞主线程。

### Task 3: 增加可解释的优化模式

**Files:**
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/SettingsScreen.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/ChatScreen.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/PromptOptimizeLogic.kt`
- Modify: `daemon/internal/server/prompt_opt.go`
- Test: `daemon/internal/server/prompt_opt_test.go`
- Test: `PromptOptimizeLogicTest.kt`

模式至少包括：

```text
默认：把意思说清楚，不扩写需求
精简：压缩成一句话
实施计划：整理目标、范围、验收标准
排错任务：整理现象、环境、复现、期望结果
测试要求：整理输入、行为和验证方式
代码审查：整理风险、影响范围和证据要求
```

- [ ] 服务端为每种 style 建确定性模板；模板说明不得执行原任务。
- [ ] 返回结果仍只包含改写文本。
- [ ] UI 显示“保留原意，不会自动执行”。
- [ ] 当候选比原文大幅增加时，显示“请检查是否加入了原文没有的内容”。
- [ ] 配额、登录、连接、超时错误分别映射为行动文案。

### Task 4: 建立 VoiceSession 领域状态

**Files:**
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/voice/VoiceSessionState.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/voice/VoiceSessionReducer.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/voice/VoiceController.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/voice/VoiceEngine.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/ChatScreen.kt`
- Test: create `android/app/src/test/java/dev/pocketagent/app/ui/VoiceSessionReducerTest.kt`

**Interfaces:**

```kotlin
enum class VoiceEngineKind { SYSTEM_ON_DEVICE, SYSTEM_REMOTE, SENSEVOICE, UNAVAILABLE }
enum class VoicePhase { IDLE, REQUESTING_PERMISSION, STARTING, LISTENING, PAUSED, STOPPING, TRANSCRIBING, ERROR }

data class VoiceSessionState(
    val sessionId: Long,
    val threadId: String?,
    val phase: VoicePhase,
    val engine: VoiceEngineKind,
    val partialText: String,
    val finalText: String,
    val error: VoiceErrorKind?,
    val mutedByThreadSwitch: Boolean
)

sealed interface VoiceEvent {
    data object Start: VoiceEvent
    data object Pause: VoiceEvent
    data object Stop: VoiceEvent
    data object Cancel: VoiceEvent
    data class Partial(val sessionId: Long, val text: String): VoiceEvent
    data class Final(val sessionId: Long, val text: String): VoiceEvent
    data class Error(val sessionId: Long, val kind: VoiceErrorKind): VoiceEvent
    data class SessionSwitched(val newThreadId: String): VoiceEvent
    data class Permission(val granted: Boolean): VoiceEvent
}
```

- [ ] reducer 只处理状态，不直接调用 Android API。
- [ ] 所有识别回调带 session id；旧 session 的迟到结果被丢弃。
- [ ] 页面切换、旋转、离开页面时有明确 stop/cancel 行为。
- [ ] `Channel.UNLIMITED` 只保留为受控 effect 队列，识别文本本身必须有长度上限。

### Task 5: 适配系统识别和 SenseVoice

**Files:**
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/component/SenseVoiceRecognizer.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/component/SenseVoiceSession.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/ChatScreen.kt` only for wiring
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/voice/SystemVoiceEngine.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/voice/SenseVoiceEngine.kt`
- Test: `SenseVoiceFallbackTest.kt`, `VoiceMicBlockedFeedbackTest.kt`, `SenseVoiceSessionTest.kt`

- [ ] 保留系统识别 → SenseVoice 的现有优先级。
- [ ] 保留 8 秒死区看门狗，但改用 monotonic elapsed time。
- [ ] 系统识别收到致命错误或零回调后只在本轮/设备状态中标记失败，不永久污染其他会话。
- [ ] SenseVoice 冷加载、模型损坏、磁盘空间不足分别显示不同文案。
- [ ] 音频权限和 MIUI“录音”独立开关分开指引。
- [ ] 生产日志不写完整 transcript。

### Task 6: 语音输入交互

**Files:**
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/voice/VoiceInputPanel.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/ChatScreen.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/VoiceErrors.kt` if present
- Test: create `VoiceInputPanelLogicTest.kt`

- [ ] 支持点击开始/点击结束和按住说话两种入口。
- [ ] 显示引擎、录音时长、部分转写、停止/取消。
- [ ] 识别完成只追加到输入框，不自动发送。
- [ ] 支持“追加到当前文本”和“替换当前文本”。
- [ ] 对英文、路径、命令保留原始大小写和标点，不擅自重写代码。
- [ ] 输入框与语音面板均满足 48dp 触控尺寸和 TalkBack 描述。

### Task 7: 提示词优化和语音验收

- [ ] JVM 逻辑测试通过。
- [ ] Android fresh test 使用 `--rerun` 通过。
- [ ] 真机验证：权限拒绝、系统识别死区、SenseVoice 冷启动、暂停/取消、切换会话、旋转、后台回前台、迟到回调。
- [ ] 真机验证：优化预览、修改候选、应用、撤销、优化请求期间修改原文、超时和配额失败。
- [ ] 证据写入 `docs/verify/evidence/prompt-and-voice-input.md`。
- [ ] 提交：`feat: improve prompt optimization and voice input`。
