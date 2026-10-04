# PocketAgent 多 Agent 协同闭环 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans`. 先完成 baseline plan；本计划的 protobuf 修改必须由 protocol steward 合并。

**Goal:** 让多 Agent 协同从“派发后打开子会话”升级为可恢复、可观察、可审批、可取消、可隔离和可比较的协同任务。

**Architecture:** 引入持久化 `CollabSession` 聚合和 `CollabMemberRun` 成员状态；daemon 统一发布协同事件，Android 通过快照+流恢复状态。保持现有 `DispatchCollab` 受理语义，不把受理伪装成完成；默认禁止多个可写成员共享工作区。

**Tech Stack:** Go server/store/eventstore、protobuf/gRPC streaming、Kotlin/Compose、现有 `SubmitApproval`、Git worktree/Windows 文件系统。

**Spec:** `docs/superpowers/specs/2026-10-04-pocketagent-user-experience-and-reliability-upgrade.md`

## Global Constraints

- 不自动批准成员的工具请求；审批必须对应真实的 `thread_id/request_id`。
- `ACK` 只表示已受理，`DONE` 只能由真实终局事件或持久状态推导。
- 多成员写任务必须使用隔离 worktree；只读分析可以共享工作区但必须明确标记。
- 协同事件必须有界、可取消、可恢复；禁止每事件无限启动 goroutine。
- 旧客户端仍可调用 `DispatchCollab`，新字段和新 RPC 采用 additive 兼容。

## Review Focus

- 成员轮次完成、失败、取消、daemon 重启后，协同状态是否最终收敛。
- 父 RPC 返回后，后台协同任务是否仍能被取消和观察。
- 一个成员等待审批时，其他成员是否继续运行且不被误报为完成。
- 多成员同时修改时，是否存在共享工作区污染或错误合并。
- 协同会话页面关闭、重开、断线后，状态是否从 daemon 真值恢复。

### Task 1: 定义并冻结协同协议增量

**Files:**
- Modify: `proto/v1/thread.proto`
- Modify: `android/app/src/main/proto/v1/thread.proto`
- Generate: `daemon/internal/gen/pocketagent/v1/*`
- Test: `daemon/internal/server/collab_session_wire_test.go`

**Interfaces:**

新增以下消息/字段，字段号由 protocol steward 分配并登记：

```protobuf
message CollabSessionSnapshot {
  string session_id = 1;
  string instruction = 2;
  string status = 3;
  string workspace = 4;
  int64 created_at_unix_ms = 5;
  int64 updated_at_unix_ms = 6;
  repeated CollabMemberSnapshot members = 7;
  string error = 8;
}

message CollabMemberSnapshot {
  string agent_id = 1;
  string role_id = 2;
  string thread_id = 3;
  string status = 4;
  string model_id = 5;
  string effort = 6;
  string progress = 7;
  string summary = 8;
  string error = 9;
  repeated ApprovalRequest pending_approvals = 10;
  string workspace = 11;
}

message WatchCollabSessionRequest { string session_id = 1; int64 after_sequence = 2; }
message CollabSessionEvent {
  int64 sequence = 1;
  string session_id = 2;
  string member_thread_id = 3;
  string kind = 4;
  CollabSessionSnapshot snapshot = 5;
  CollabMemberSnapshot member = 6;
}
```

新增 RPC：

```protobuf
rpc GetCollabSession(GetCollabSessionRequest) returns (CollabSessionSnapshot);
rpc ListCollabSessions(ListCollabSessionsRequest) returns (ListCollabSessionsResponse);
rpc WatchCollabSession(WatchCollabSessionRequest) returns (stream CollabSessionEvent);
rpc CancelCollabSession(CancelCollabSessionRequest) returns (CollabSessionSnapshot);
rpc CancelCollabMember(CancelCollabMemberRequest) returns (CollabMemberSnapshot);
rpc RetryCollabMember(RetryCollabMemberRequest) returns (CollabMemberSnapshot);
```

- [ ] 写 proto compatibility golden test：旧字段解码不丢失，新字段被旧客户端忽略。
- [ ] 生成 Go/Android 代码。
- [ ] 运行两副本 MD5 校验。

### Task 2: 建立 daemon 协同持久化模型

**Files:**
- Create: `daemon/internal/store/collab_store.go`
- Modify: `daemon/internal/store/store.go` only for shared atomic persistence hooks
- Test: `daemon/internal/store/collab_store_test.go`

**Interfaces:**

```go
type CollabSessionStore interface {
    CreateSession(ctx context.Context, s CollabSession) error
    GetSession(ctx context.Context, id string) (CollabSession, bool)
    ListSessions(ctx context.Context, includeFinished bool) []CollabSession
    UpdateSession(ctx context.Context, s CollabSession) error
    AppendEvent(ctx context.Context, e CollabEvent) (int64, error)
    ReplayEvents(ctx context.Context, id string, after int64) []CollabEvent
}
```

短期继续使用原子 JSON 文件或现有 store 的持久化机制；不要在本计划中提前引入 SQLite。每次写入必须：

- 临时文件；
- flush/close；
- atomic rename；
- 失败保留旧文件；
- 启动时损坏隔离并生成 recovery report。

- [ ] 测试首次运行、重启恢复、损坏文件、重复 session id、成员状态合并。
- [ ] 测试事件序号单调、回放和 retention 缺口。

### Task 3: 把 `DispatchCollab` 改造成“创建会话+受理成员”

**Files:**
- Modify: `daemon/internal/server/collab_session.go`
- Create: `daemon/internal/server/collab_session_state.go`
- Create: `daemon/internal/server/collab_session_events.go`
- Test: `daemon/internal/server/collab_session_test.go`

**Interfaces:**

```go
type CollabCoordinator interface {
    Dispatch(ctx context.Context, req DispatchRequest) (CollabSessionSnapshot, error)
    Snapshot(ctx context.Context, sessionID string) (CollabSessionSnapshot, error)
    Watch(ctx context.Context, sessionID string, after int64) (<-chan CollabSessionEvent, error)
    CancelSession(ctx context.Context, sessionID string) error
    CancelMember(ctx context.Context, sessionID, threadID string) error
    RetryMember(ctx context.Context, sessionID, threadID string) error
}
```

- [ ] `DispatchCollab` 先创建 session，再逐成员创建 `CollabMemberRun`。
- [ ] 每个成员受理成功只写 `ACCEPTED/RUNNING`，不写 DONE。
- [ ] `runCollabMember` 的所有出口都调用一次终局更新：`COMPLETED`、`FAILED`、`CANCELLED`、`WAITING_APPROVAL`。
- [ ] 对迟到的成员事件使用 generation/session revision 防止旧轮次覆盖新轮次。
- [ ] 将当前 `childSink := thinkingRecorder(func(ev) {})` 改为有界事件转换器：只转发进度、审批、工具开始/结束、最终摘要和错误；原始大文本按策略截断或落盘引用。
- [ ] 事件广播和持久化不能阻塞 Agent 主轮次；写失败必须可观察但不能伪造完成。

### Task 4: 接入审批、取消和重试

**Files:**
- Modify: `daemon/internal/server/collab_session_state.go`
- Modify: `daemon/internal/server/approval_resume.go`
- Modify: `daemon/internal/server/turn_cancel_conduction.go` or current cancel implementation
- Test: `daemon/internal/server/collab_approval_test.go`

**Interfaces:**

- 协同成员的 `ApprovalRequest` 必须携带真实 `thread_id/request_id`。
- UI 继续复用现有 `SubmitApproval`；协同状态只负责把 pending approval 映射到成员卡。
- `CancelCollabMember` 只取消对应 thread；`CancelCollabSession` 逐成员发起取消并等待状态收敛。

- [ ] 测试一个成员等待审批、其他成员完成的部分终局。
- [ ] 测试审批过期、重复提交、跨线程提交被拒绝。
- [ ] 测试取消后的迟到 `TurnCompleted` 不会把成员重新标为 DONE。
- [ ] 测试重试会创建新的 member generation，不覆盖旧结果。

### Task 5: 实现工作区模式与隔离

**Files:**
- Create: `daemon/internal/server/collab_workspace.go`
- Create: `daemon/internal/server/collab_workspace_test.go`
- Modify: `daemon/internal/server/collab_session.go`
- Modify: `proto/v1/thread.proto` via protocol steward only

**Interfaces:**

```go
type CollabWorkspaceMode string
const (
    WorkspaceReadOnly CollabWorkspaceMode = "read_only"
    WorkspaceIsolatedWorktree CollabWorkspaceMode = "isolated_worktree"
)

type CollabWorkspaceManager interface {
    Prepare(ctx context.Context, sessionID, memberID, requested string, mode CollabWorkspaceMode) (WorkspaceLease, error)
    Cleanup(ctx context.Context, lease WorkspaceLease, keepArtifacts bool) error
}
```

规则：

- `read_only`：成员不得执行写操作；若 Agent 请求写入，显示审批/拒绝原因。
- `isolated_worktree`：每个成员独立 branch/worktree，结果保存 diff 和测试证据。
- 多成员写任务没有隔离能力时，整体拒绝并给出人话原因，不能静默退回共享目录。
- 路径必须经过统一 workspace guard，处理 junction、盘根和用户目录。

- [ ] 测试同一仓库创建两个隔离 worktree。
- [ ] 测试清理失败不删除用户原始目录。
- [ ] 测试成员取消和 daemon 重启后的 lease recovery。

### Task 6: Android 协同状态仓库和恢复

**Files:**
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/collab/CollabSessionRepository.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/collab/CollabSessionReducer.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/collab/CollabSessionState.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/CollabState.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/CollabScreen.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/AgentConnection.kt` only for typed streaming stub access
- Test: `android/app/src/test/java/dev/pocketagent/app/ui/CollabDispatchFlowTest.kt`
- Test: create `CollabSessionReducerTest.kt`

**Interfaces:**

```kotlin
data class CollabSessionState(
    val sessionId: String?,
    val status: CollabStatus,
    val instruction: String,
    val members: List<CollabMemberUi>,
    val lastSequence: Long,
    val loading: Boolean,
    val error: String?
)

sealed interface CollabIntent {
    data class Dispatch(val instruction: String): CollabIntent
    data class CancelMember(val threadId: String): CollabIntent
    data class RetryMember(val threadId: String): CollabIntent
    data object CancelSession: CollabIntent
    data class OpenApproval(val threadId: String, val requestId: String): CollabIntent
}
```

- [ ] `CollabScreen` 不再直接读取一组散落的全局 mutable state。
- [ ] 进入页面优先 `GetCollabSession`，再 `WatchCollabSession(after_sequence)`。
- [ ] 断线先显示最后一个真快照，恢复后按 seq 续传；超窗时全量 snapshot。
- [ ] `DONE`、`FAILED`、`WAITING_APPROVAL`、`CANCELLED` 均由 daemon 事件驱动。
- [ ] 成员卡显示查看会话、查看原因、重试、取消、审批等真实入口。

### Task 7: 协同 UI 和人话文案

**Files:**
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/CollabScreen.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/CollabLogic.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/collab/CollabCopy.kt`
- Test: `CollabDispatchFlowTest.kt`, `CollabRolesLogicTest.kt`

- [ ] 状态统一为：已提交、已受理、执行中、等待确认、已完成、未完成、已取消。
- [ ] 每个禁用按钮旁边显示禁用原因。
- [ ] 总体状态显示 `2/3 已完成 · 1 个等待确认`，不能只显示运行中。
- [ ] 结果区域区分“成员原始结果”和“汇总 Agent 结果”。
- [ ] 不自动合并 diff；先提供比较、查看和选择。

### Task 8: 验收

- [ ] daemon 定向测试、`go test -race` 覆盖协同包。
- [ ] Android 定向 JVM 测试使用 `--rerun`。
- [ ] 真机验证：成员完成、成员失败、成员等待审批、取消、重试、离开页面后重开、daemon 重启后恢复。
- [ ] 证据写入 `docs/verify/evidence/collab-session-closure.md`。
- [ ] 提交：`feat: close the collab session lifecycle`。
