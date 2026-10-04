# PocketAgent 会话真值、模型选择与历史体验 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans`. 本计划可与未知事件/协同计划并行，但 protobuf 和 `ChatScreen.kt` 的修改必须错峰合并。

**Goal:** 统一模型/思考强度的事实来源，并把会话历史从“按轮加载+数组下标搜索”升级为稳定 ID、自动加载、可解释范围和可扩展搜索。

**Architecture:** daemon 保存 Thread/Turn 的有效选择和 TurnSnapshot；Android 使用 `ModelSelectionRepository` 和 reducer，不再让多个 mutable global state 互相覆盖。历史搜索以稳定 `message_id/turn_id` 为定位依据，短期继续 daemon 扫描，后续可无缝迁移 SQLite FTS。

**Tech Stack:** Go store/server、protobuf/gRPC、Kotlin/Compose、JVM 单测；不在本计划中提前引入 SQLite。

**Spec:** `docs/superpowers/specs/2026-10-04-pocketagent-user-experience-and-reliability-upgrade.md`

## Global Constraints

- 当前轮次启动后，模型和 effort 不再被 UI 切换改写。
- UI 必须显示配置来源和生效时机：本轮、下一轮、桌面默认或 Agent 原生当前值。
- 不用 Agent 名单或模型名单硬编码能力；能力来自 daemon 的 `ModelInfo`/capability。
- 历史搜索失败不能静默表现为空结果。
- 搜索结果不得依赖可变数组下标作为长期 ID。
- 长会话加载更早必须保持滚动锚点，不重复下载已加载内容。

## Review Focus

- 切换模型后当前正在运行的 Turn 不被改变。
- 切换到没有对应 effort 的模型时不会保留失效强度。
- daemon 重启或 Android 重启后线程选择仍能恢复。
- 加载更早、回滚、分叉后搜索结果仍定位到正确消息。
- 远端搜索失败、本地未下载和确实无结果三种状态可区分。

### Task 1: 定义有效模型选择协议

**Files:**
- Modify: `proto/v1/model.proto` via protocol steward
- Modify: `proto/v1/thread.proto` via protocol steward only if `Thread` needs a selection field
- Generate: `daemon/internal/gen/pocketagent/v1/*`
- Test: `daemon/internal/server/model_selection_wire_test.go`

**Interfaces:**

```protobuf
message EffectiveModelSelection {
  string agent_id = 1;
  string model_id = 2;
  string effort = 3;
  string source = 4; // thread_override/app_default/desktop_default/native_current
  bool applies_to_next_turn = 5;
  bool configuration_confirmed = 6;
  string explanation = 7;
  repeated string available_efforts = 8;
}
```

将 `EffectiveModelSelection` 作为 `Thread`/`ListModelsResponse` 的 additive 字段，具体字段号由 steward 分配。

- [ ] 测试旧客户端忽略字段仍能选模型。
- [ ] 测试空 model/effort 表示“跟随默认”，不是错误。

### Task 2: daemon 统一选择解析和 TurnSnapshot

**Files:**
- Create: `daemon/internal/server/model_selection.go`
- Modify: `daemon/internal/server/model.go`
- Modify: `daemon/internal/server/thread.go`
- Modify: `daemon/internal/store/store.go` only for migration/accessor
- Test: `daemon/internal/server/model_selection_test.go`
- Test: `daemon/internal/server/turn_started_test.go`

**Interfaces:**

```go
type SelectionResolver interface {
    ResolveThreadSelection(ctx context.Context, t *store.Thread) (pb.EffectiveModelSelection, error)
    ValidateSelection(agentID, modelID, effort string) error
}

type TurnSnapshot struct {
    ThreadID string
    AgentID string
    ModelID string
    Effort string
    Workspace string
    Source string
    CapturedAtUnixMs int64
}
```

规则：

- 创建/更新线程时校验 model 是否属于 agent；未知模型不能静默改成默认。
- effort 不在当前模型能力集合时返回清楚的人话，并建议可用值。
- `StartTurn` 创建 immutable `TurnSnapshot`，所有 Provider 从 snapshot 取配置。
- `TurnStarted` 必须包含实际生效的 model/effort/source。
- 每次选择变化广播 `selection_changed`，但不改已开始的 Turn。

### Task 3: Android 建立 `ModelSelectionRepository`

**Files:**
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/model/ModelSelectionState.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/model/ModelSelectionReducer.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/model/ModelSelectionRepository.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/AgentConnection.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/TurnSession.kt`
- Test: create `ModelSelectionReducerTest.kt`
- Test: modify `ChatModelRoutingTest.kt`, `ModelEffortOptionsTest.kt`

**Interfaces:**

```kotlin
data class EffectiveSelectionUi(
    val agentId: String,
    val modelId: String,
    val effort: String,
    val source: SelectionSource,
    val appliesToNextTurn: Boolean,
    val explanation: String,
    val availableEfforts: List<String>
)

sealed interface ModelSelectionIntent {
    data class SelectModel(val modelId: String): ModelSelectionIntent
    data class SelectEffort(val effort: String): ModelSelectionIntent
    data object ResetToDefault: ModelSelectionIntent
    data object Refresh: ModelSelectionIntent
}
```

- [ ] `selectedModel/defaultModel/threadModels/runtimeModel` 不再直接被 UI 混用。
- [ ] SharedPreferences 只做本地缓存，不作为 daemon 事实源。
- [ ] 页面进入时先显示缓存，再用 daemon response 替换；旧缓存必须标注“上次记录”。
- [ ] 模型卡上显示 Agent、模型、上下文、effort、来源和生效时机。

### Task 4: 重构模型与思考强度卡

**Files:**
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/ChatScreen.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/model/ModelEffortCard.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/model/ModelOptionRow.kt`
- Test: `ModelEffortOptionsTest.kt`, `ChatModelRoutingTest.kt`

- [ ] 从 `ChatScreen.kt` 拆出 `ModelEffortCard`，页面只传 state/intent。
- [ ] effort 选中态来自 `EffectiveSelectionUi`，不再直接读取全局 `AgentConnection.selectedEffort`。
- [ ] 运行中选择显示“将从下一轮生效”。
- [ ] 当前模型不支持原 effort 时，先清理失效值，再显示可选档位。
- [ ] 未知 effort 显示原 id，并提供技术详情，不显示空白。
- [ ] 长模型名、上下文窗口、图片能力、思考能力和可用性均能在窄屏阅读。

### Task 5: 为历史消息补稳定身份

**Files:**
- Modify: `daemon/internal/store/store.go`
- Modify: `daemon/internal/server/thread.go`
- Modify: `proto/v1/thread.proto` via protocol steward
- Modify: `android/app/src/main/proto/v1/thread.proto` via steward
- Generate: `daemon/internal/gen/pocketagent/v1/*`
- Test: `daemon/internal/store/store_message_id_test.go`
- Test: `daemon/internal/server/thread_history_rounds_test.go`

**Interfaces:**

```go
type StoredMessage struct {
    ID string `json:"id"`
    TurnID string `json:"turn_id,omitempty"`
    Role string
    Blocks []pb.ContentBlock
    CreatedAt int64
}
```

规则：

- 新消息生成稳定 ID；旧数据启动时按 thread + index + created_at + content hash 生成迁移 ID。
- ID 生成必须幂等，重复启动不改变已有 ID。
- `ThreadMessage` 增加 `message_id`、`turn_id`。
- fork/rollback/import 必须保持或明确生成新的 message id。

### Task 6: 升级单会话搜索

**Files:**
- Modify: `daemon/internal/server/thread_search.go`
- Modify: `proto/v1/thread.proto` via steward
- Create: `daemon/internal/server/thread_search_ranges.go` if needed
- Test: `daemon/internal/server/thread_search_test.go`

**Interfaces:**

```protobuf
message ThreadMessageHit {
  // existing fields unchanged
  string message_id = 6;
  string turn_id = 7;
  repeated MatchRange ranges = 8;
  bool from_loaded_history = 9;
}
message MatchRange { int32 start = 1; int32 end = 2; }
message SearchThreadMessagesResponse {
  repeated ThreadMessageHit hits = 1;
  bool has_more = 2;
  int32 searched_messages = 3;
  string scope = 4; // full_thread/local_snapshot
}
```

- [ ] 继续支持中文和 Unicode 大小写安全匹配。
- [ ] 每条消息可返回多个命中范围，但回包必须有界。
- [ ] 查询失败返回明确状态，不返回“空结果成功”。
- [ ] limit、cursor、has_more 可支持未来 FTS 迁移。

### Task 7: Android 历史搜索和自动定位

**Files:**
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/history/HistorySearchRepository.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/history/HistorySearchState.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/component/HistoryQuickNav.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/component/HistoryQuickNavLogic.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/ChatScreen.kt`
- Test: `HistoryQuickNavSearchTest.kt`
- Test: create `HistorySearchStateTest.kt`

- [ ] 搜索状态区分：本页结果、全会话结果、仅已下载、远端失败、无结果。
- [ ] 命中不在当前列表时自动请求目标历史片段，而不是只 Toast“请先加载更早”。
- [ ] 使用 `message_id` 定位；`sequence` 仅作为兼容字段。
- [ ] 搜索结果显示匹配高亮、角色、日期、所属轮次和会话标题。
- [ ] 加载更早使用 cursor/oldest message id，避免反复拉取和整列表替换。
- [ ] 保留现有刻度条、日期分组和最新/最早快跳交互。

### Task 8: UI 分层和回归

**Files:**
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/chat/ChatRoute.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/chat/ChatViewModel.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/chat/ChatState.kt`
- Modify: `ChatScreen.kt` only to delegate state and compose child components
- Test: `ChatTimelineTest.kt`, `HistoryMessageMapperTest.kt`, new `ChatStateReducerTest.kt`

- [ ] 长期状态进入 ViewModel/StateFlow；Compose 页面不直接持有 RPC、SharedPreferences 和全局选择状态。
- [ ] 所有 collect 使用 lifecycle-aware collection。
- [ ] 长列表提供稳定 key；流式内容更新不复制整条时间线。
- [ ] 每次拆分后运行受影响测试，禁止一次性重写整个 ChatScreen。

### Task 9: 验收

- [ ] daemon build/vet/test 通过。
- [ ] Android 受影响测试使用 `--rerun` 通过。
- [ ] 真机验证模型切换、effort 切换、运行中切换、重启恢复、长会话搜索、加载更早和离线快照。
- [ ] 证据写入 `docs/verify/evidence/conversation-selection-and-history.md`。
- [ ] 提交：`feat: unify conversation selection and history navigation`。
