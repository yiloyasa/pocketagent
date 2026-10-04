# PocketAgent 未知事件用户处理中心 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans`. 未知事件的安全语义必须先由 daemon 定义，Android 不得自行猜测可执行动作。

**Goal:** 将未知事件从“设置页里的技术台账”升级为会话内可解释、全局可追踪、用户可安全处理的消息中心。

**Architecture:** 保留现有 `UnknownEventLog`、原始载荷和处置状态；新增确定性的人话解释和 `UserActionDescriptor`，由 daemon 决定哪些动作安全可展示。与事件关联线程时在时间线内显示；无关联事件进入全局 Attention Center；技术原文默认折叠。

**Tech Stack:** Go `runtime`/`server`、protobuf/gRPC、Kotlin/Compose、现有 `TurnSession` 和 `RuntimeSection`。

**Spec:** `docs/superpowers/specs/2026-10-04-pocketagent-user-experience-and-reliability-upgrade.md`

## Global Constraints

- `UNKNOWN_REQUEST` 默认不显示“允许”按钮。
- 只有适配器明确声明 safe action，且 action 参数已结构化，才能进入用户操作区。
- 人话解释是确定性摘要，不由 LLM 代替安全判断。
- 技术原文必须可查看、可复制、可导出；普通 UI 不直接显示 JSON。
- 任何 action 都必须幂等、带事件 revision，并绑定 event/thread/request identity。

## Review Focus

- unknown notification 不应阻塞正常轮次，也不应被显示成失败。
- unknown request 不能被“忽略”误解成已执行或已拒绝。
- 事件详情拉取失败不能显示技术异常堆栈。
- 旧 daemon 没有新字段时，客户端仍能显示方法名和明确的降级说明。
- 同一事件重复点击、快速展开/收起和多设备同时处理不能覆盖新状态。

### Task 1: 定义未知事件用户动作模型

**Files:**
- Modify: `proto/v1/runtime.proto` via protocol steward
- Modify: `android/app/src/main/proto/v1/runtime.proto` via protocol steward
- Generate: `daemon/internal/gen/pocketagent/v1/*`
- Test: `daemon/internal/server/unknown_events_wire_test.go`

**Interfaces:**

```protobuf
message UserActionDescriptor {
  string action_id = 1;
  string label = 2;
  string description = 3;
  bool safe = 4;
  bool requires_confirmation = 5;
  bool destructive = 6;
}

message UnknownEventSummary {
  // existing fields 1-11 unchanged
  string plain_title = 12;
  string plain_explanation = 13;
  string impact = 14;
  bool requires_user = 15;
  bool can_open_native = 16;
  repeated UserActionDescriptor actions = 17;
}

message UnknownEventResponse {
  // existing fields 1-6 unchanged
  string recommended_next_step = 7;
  string support_bundle_hint = 8;
}
```

- [ ] 字段只增不改；旧端忽略新字段仍可显示 `summary`。
- [ ] 为 `UNKNOWN_NOTIFICATION`、`UNKNOWN_REQUEST`、`UNKNOWN_RESPONSE` 写 golden fixtures。

### Task 2: 扩展 daemon 事件信封与确定性翻译

**Files:**
- Modify: `daemon/internal/runtime/unknown_events.go`
- Modify: `daemon/internal/runtime/event_translate.go`
- Create: `daemon/internal/runtime/unknown_action.go`
- Test: `daemon/internal/runtime/unknown_events_test.go`
- Test: `daemon/internal/runtime/event_translate_test.go`
- Test: create `unknown_action_test.go`

**Interfaces:**

```go
type UnknownUserAction struct {
    ID string
    Label string
    Description string
    Safe bool
    RequiresConfirmation bool
    Destructive bool
}

type UnknownPresentation struct {
    PlainTitle string
    PlainExplanation string
    Impact string
    RequiresUser bool
    CanOpenNative bool
    Actions []UnknownUserAction
    RecommendedNextStep string
}

func PresentUnknownEvent(env EventEnvelope) UnknownPresentation
```

固定规则：

- notification：`电脑端传来一条新消息，当前版本还没有对应的卡片显示方式。`；不阻塞轮次；动作只能是查看、复制、已查看。
- request：如果没有安全 action，`电脑端请求了一个当前版本无法安全判断的操作，因此没有自动执行。`；动作是查看、复制、在电脑端处理、待上游支持。
- response：`电脑端返回了一条结果，但当前版本找不到它对应的请求。原始结果已保留。`
- 任何方法名、字段名无法解释时保留原文，不填充猜测。

- [ ] 测试敏感字段脱敏后仍能生成摘要。
- [ ] 测试未知 method 不会得到 `allow/approve` action。
- [ ] 测试翻译失败只影响摘要，不影响事件落盘和主轮次。

### Task 3: 将未知事件接入时间线事件模型

**Files:**
- Inspect/Modify: `daemon/internal/agent/unknown_passthrough.go`
- Inspect/Modify: `daemon/internal/server/unknown_events.go`
- Modify: `proto/v1/thread.proto` only if an additive `UnknownEventNotice` is required
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/TurnSession.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/UnknownEventCardModel.kt`
- Test: `daemon/internal/server/unknown_events_test.go`
- Test: create `android/app/src/test/java/dev/pocketagent/app/ui/UnknownEventCardModelTest.kt`

**Interfaces:**

```kotlin
data class UnknownEventCardModel(
    val eventId: String,
    val title: String,
    val explanation: String,
    val impact: String,
    val requiresUser: Boolean,
    val actions: List<UserActionModel>,
    val technicalAvailable: Boolean,
    val status: String
)
```

- [ ] 有 `thread_id/turn_id` 的事件在对应会话时间线中显示卡片。
- [ ] 没有会话关联的事件只进入全局中心，不虚构归属。
- [ ] 卡片显示“任务是否受影响”，区分通知、阻塞请求和无法关联响应。
- [ ] 事件卡不阻塞正常文本流，展开详情按需请求 `GetUnknownEvent`。

### Task 4: 改造 `RuntimeSection` 为技术详情入口

**Files:**
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/RuntimeSection.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/unknown/UnknownEventDetailSheet.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/unknown/UnknownEventCopy.kt`
- Test: `android/app/src/test/java/dev/pocketagent/app/ui/RuntimeSectionLogicTest.kt`

- [ ] 将“消息透析”入口改成普通用户能理解的“电脑端新消息”。
- [ ] 列表分为“需要你处理”“已查看”“仅供参考”。
- [ ] 每行显示 plain title、影响、来源 Agent、时间和状态。
- [ ] 原文、字段透析和 correlation id 放入“技术详情”。
- [ ] 错误详情统一转换为“暂时没读到详情，请稍后再试”，技术异常只进日志。
- [ ] `AcknowledgeUnknownEvent` 支持用户输入可选 note，不能始终传空字符串。

### Task 5: 建立全局 Attention Center 接入点

**Files:**
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/attention/AttentionItem.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/attention/AttentionCenterState.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/HomeScreen.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/AppDrawer.kt`
- Test: create `AttentionCenterStateTest.kt`

**Interfaces:**

```kotlin
sealed interface AttentionItem {
    val id: String
    val title: String
    val detail: String
    val actionLabel: String?
    data class UnknownEvent(...): AttentionItem
    data class Approval(...): AttentionItem
    data class Recovery(...): AttentionItem
    data class Collab(...): AttentionItem
}
```

- [ ] 首页只显示数量和最高优先级摘要，不复制完整详情。
- [ ] Attention Center 支持按“需要我处理 / 运行中 / 已完成 / 已忽略”筛选。
- [ ] 点击后回到正确会话或设置详情，返回后保留滚动位置。
- [ ] offline 时显示本地缓存时间，不能把旧数据伪装成实时状态。

### Task 6: 支持 action 的安全执行和 revision

**Files:**
- Modify: `daemon/internal/server/unknown_events.go`
- Create: `daemon/internal/server/unknown_actions.go`
- Test: `daemon/internal/server/unknown_actions_test.go`

- [ ] action 请求必须带 `event_id + revision`。
- [ ] revision 不匹配返回可理解的冲突说明，客户端重新拉取详情。
- [ ] 只实现明确安全的 action：inspect、acknowledge、retry-later、open-native。
- [ ] 不实现泛化 `approve unknown request`。
- [ ] action 执行结果写入事件处置历史，便于审计。

### Task 7: 验收

- [ ] daemon unknown event/runtime/server 测试通过。
- [ ] Android 定向 JVM 测试使用 `--rerun` 通过。
- [ ] 真机验证通知、未知请求、未知响应、详情加载失败、重复操作、离线打开。
- [ ] 证据写入 `docs/verify/evidence/unknown-event-action-center.md`。
- [ ] 提交：`feat: add actionable unknown event center`。
