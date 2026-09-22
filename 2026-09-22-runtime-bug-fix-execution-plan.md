# PocketAgent 统一 Runtime 问题修复与架构优化执行方案

> 文档类型：诊断结论、实施方案、审核方案  
> 生成日期：2026-09-22  
> 适用项目：`G:\项目1\pocketagent`  
> 当前阶段：只读诊断完成，尚未开始修改生产代码  
> 控制面位置：每个用户自己的电脑上的 `pocketagentd`，手机只负责控制和显示

## 1. 结论先行

当前“App 删除会话后又出现”不是单一 UI 缓存问题，而是多个会话来源没有经过同一个 Runtime Kernel 统一仲裁的问题。

PocketAgent 当前至少存在以下会话来源：

1. daemon 本地 `threads.json`；
2. Codex 原生会话列表；
3. Pi/ZCode 原生会话列表；
4. Android 会话列表快照；
5. Android 消息快照、checkpoint、模型选择、用量、置顶和任务中心缓存。

当前删除流程只删除了部分本地状态。只要原生会话删除失败、导入接口重新读取原生会话、App 处于离线状态读取旧快照，或者迟到的子代理事件重新登记线程，被删除会话就可能重新出现。

图一和图二也不是互相独立的问题：

- 图一是 Codex 的命令进入了需要键盘输入的交互状态，但当前 Codex thread 内命令协议没有可用的 stdin 写入 RPC。手机不能伪造一个输入框把内容写回去。
- 图二表示任务被判断为完成，但最终 assistant 文本为空。当前实现用 `(完成,无文本输出)` 占位，掩盖了“真正无文本”“只产生了工具产物”和“适配器丢失正文”之间的区别。
- 断线恢复慢，是因为连接重建、事件增量恢复、历史全量回填和任务终局检测耦合在一起，Android 仍使用 800ms 首次等待和随后 5s 周期。
- 未知事件诊断已有索引，但已知或可安全透传的事件仍可能污染 UNKNOWN。
- 所有 Agent 的 Runtime 接口已经出现，但 memory/cache/resource 仍没有全部由统一持久化后端接管。

因此本次不能只加若干 `filterNot` 或单独修 Codex。正确目标是：

```text
所有状态输入
    ↓
Provider Adapter
    ↓
Runtime Kernel
    ├── DeletionRegistry
    ├── EventRouter
    ├── OutputNormalizer
    ├── RecoveryCoordinator
    ├── MemoryManager
    ├── CacheManager
    ├── ResourceLedger
    └── CheckpointManager
    ↓
daemon 持久化状态 / 手机可见状态
```

## 2. 当前项目状态与工作树边界

本方案生成前只做了只读检查，没有修改源码、README 或 `AUDIT.md`。

当前仓库：

```text
G:\项目1\pocketagent
```

当前 Git：

```text
分支：main
HEAD：932b59a feat(runtime): W11b 设置页「运行时」区段(恢复横幅/形态卡/预算/未知事件/协同策略)
```

当前已有未提交改动，实施时不得覆盖：

```text
M android/app/src/main/java/dev/pocketagent/app/ui/CollabLogic.kt
M android/app/src/main/java/dev/pocketagent/app/ui/CollabState.kt
M android/app/src/main/java/dev/pocketagent/app/ui/RuntimeSection.kt
M android/app/src/test/java/dev/pocketagent/app/ui/RuntimeSectionLogicTest.kt
?? tools/device/_pa_verify_runtime.py
```

README 当前存在真值漂移：

```text
G:\项目1\pocketagent\README.md:37
```

README 记录的是旧 HEAD、旧工作树状态和旧 W11 进度，不能在实施前直接覆盖。README、AUDIT 和 Git 状态应在代码及测试证据完成后统一同步。

现有审核唯一真源：

```text
G:\项目1\pocketagent\AUDIT.md:1-4
G:\项目1\pocketagent\AUDIT.md:34-45
G:\项目1\pocketagent\AUDIT.md:47-62
G:\项目1\pocketagent\AUDIT.md:91-102
G:\项目1\pocketagent\AUDIT.md:106-119
```

本方案不另造一套 L0/L1/L2 定义，所有实施审核都映射回 `AUDIT.md`。

## 3. 当前方案对接度

以下是工程判断，不是测试通过率。

| 能力 | 当前情况 | 对接度 |
|---|---|---:|
| daemon 控制面 | 已有 `RuntimeService`、策略、未知事件索引、资源账本接口 | 较高 |
| 所有 Agent 接入统一 Runtime | 有接口和声明，但 memory/cache/resource 仍未全部持久化统一 | 中等偏低 |
| Codex 输出处理 | 有 delta、item completed、turn completed 三层处理 | 中等 |
| 空文本处理 | 仍生成 `(完成,无文本输出)` 普通文本 | 低 |
| 上下文压缩恢复 | 能显示压缩状态，但没有正文连续性校验 | 中等偏低 |
| 断线增量恢复 | 已有 `GetThreadEventsSince` | 中等 |
| 断线恢复速度 | 仍依赖轮询，终局通知没有成为主要驱动 | 中等偏低 |
| 未知事件 | 已有保存、查询、确认接口 | 中等 |
| 未知事件分类 | 正常事件和真正未知事件尚未完全分层 | 低 |
| 协同后端策略 | daemon 已是策略真源 | 较高 |
| 协同主页面入口 | 页面存在，但主导航缺少完整 route | 中等偏低 |
| 删除墓碑 | 已覆盖部分子代理同步链 | 中等偏低 |
| 删除彻底性 | 外部列表、导入、Android 缓存未统一过滤 | 低 |

当前状态应描述为：

> Runtime 架构骨架已经存在，但删除、输出、事件、恢复和 Agent 资源状态还没有全部经过同一个 Runtime Kernel。

## 4. 根因证据

### 4.1 删除后又出现

#### 4.1.1 App 会同时读取本地和电脑端会话

Android 刷新首先读取 daemon 本地会话：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ThreadsScreen.kt:249-255
```

随后读取 Codex 原生会话：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ThreadsScreen.kt:288-312
```

最后读取 Pi/ZCode 原生会话：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ThreadsScreen.kt:313-343
```

所以删除 `threads.json` 中的本地 Thread，不等于从会话管理界面的全部数据源删除。

#### 4.1.2 Codex 原生列表没有过滤墓碑

Codex 列表接口：

```text
G:\项目1\pocketagent\daemon\internal\server\codexsessions.go:53-107
```

当前把扫描结果直接返回给手机，没有检查：

```go
s.cfg.Store.IsTombstoned(ss.ID)
```

现有复活链：

```text
DeleteThread
    ↓
本地 Thread 被删除
    ↓
Codex 原生删除失败或原生文件仍存在
    ↓
ListCodexThreads 再次读到原生会话
    ↓
Android 发现本地没有相同 codex_thread_id
    ↓
会话重新出现在“电脑上的 Codex 会话”
```

#### 4.1.3 Pi/ZCode 列表只过滤隐藏状态

Pi/ZCode 列表接口：

```text
G:\项目1\pocketagent\daemon\internal\server\agents_sessions.go:166-197
```

当前只检查：

```go
s.cfg.Store.SessionHidden(info.AgentId, info.SessionId)
```

普通删除路径并不一定设置 `HiddenSessions`：

```text
G:\项目1\pocketagent\daemon\internal\server\thread.go:352-401
```

`SetSessionHidden` 主要在 `local_only=true` 时使用。原生删除失败后，Pi/ZCode 会话仍会被扫描出来。

#### 4.1.4 导入接口没有检查删除墓碑

Pi/ZCode 导入入口：

```text
G:\项目1\pocketagent\daemon\internal\server\agents_sessions.go:247-311
```

当前一进入就可能执行：

```text
G:\项目1\pocketagent\daemon\internal\server\agents_sessions.go:254-257
```

```go
SetSessionHidden(agentID, sessionID, false)
```

但是在解除隐藏前没有检查删除墓碑。

Codex 导入入口：

```text
G:\项目1\pocketagent\daemon\internal\server\codexsessions.go:140-180
```

当前也没有在 `ImportCodexThread` 前检查：

```go
s.cfg.Store.IsTombstoned(req.CodexThreadId)
```

因此原生文件仍存在时，删除过的会话可以再次导入。

#### 4.1.5 墓碑写入失败仍可能返回删除成功

删除主流程：

```text
G:\项目1\pocketagent\daemon\internal\server\thread.go:332-421
```

墓碑写入：

```text
G:\项目1\pocketagent\daemon\internal\server\thread.go:424-437
```

当前墓碑写入失败只记录日志：

```go
log.Printf("[tombstone] 记录删除墓碑失败...")
```

然后继续删除本地 Thread 并返回成功。这会产生：

```text
本地 Thread 被删
墓碑没有落盘
原生 Thread 仍然存在
下一次同步重新导入或显示
```

#### 4.1.6 现有墓碑只覆盖部分登记链

已有的 Pi 子代理防复活：

```text
G:\项目1\pocketagent\daemon\internal\server\pi_subagent_sync.go:64-70
```

已有的 Codex 子线程防复活：

```text
G:\项目1\pocketagent\daemon\internal\server\codex_subagent_sync.go:30-39
```

这两条保护是有效的，但没有覆盖：

```text
ListCodexThreads
ListAgentThreads
ImportCodexThread
ImportAgentThread
Android 离线列表快照
Android 消息快照
墓碑写入失败
```

已有测试也只覆盖子代理存根：

```text
G:\项目1\pocketagent\daemon\internal\server\thread_delete_tombstone_test.go:25-92
```

还没有覆盖外部列表和导入接口。

### 4.2 Android 本地缓存没有统一删除

当前乐观删除：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ThreadsScreen.kt:450-487
```

只调用：

```kotlin
ThreadSnapshot.removeThreadFromSnapshot(t.id)
```

实现：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\component\ThreadSnapshot.kt:154-168
```

它只从列表级快照中移除一行。

消息快照使用按 Thread ID 计算的独立文件：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\component\ThreadSnapshot.kt:187-240
```

当前没有在删除流程中同步删除消息快照。

此外，checkpoint 仍由：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\component\TurnCheckpointStore.kt:12-76
```

独立管理，删除流程没有统一清理入口。

模型和 effort 的清理由：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\AgentConnection.kt:752-756
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\AgentConnection.kt:812-816
```

用量清理由：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\component\ThreadUsageStore.kt:103-108
```

这些清理目前通过延迟回收或其他流程间接完成，不是删除操作的原子组成部分。

### 4.3 图一：命令等待终端输入

Codex 交互事件处理：

```text
G:\项目1\pocketagent\daemon\internal\agent\codex_events.go:390-407
```

当前处理是：

```text
捕获 terminalInteraction
显示命令正在等待输入
提示用户去电脑端完成
```

这是诚实降级，不是完整手机输入能力。当前协议没有适用于 thread 内 `commandExecution` 的 stdin 写入 RPC，因此不能在手机上增加一个看似可用但实际写不进去的输入框。

如果未来要实现手机输入，必须由 daemon 自己托管 PTY 执行器，建立独立的：

```text
InteractiveCommandSession
- session_id
- stdin write
- stdout/stderr stream
- cancel
- approval
- resource lease
- reconnect/replay
- permission boundary
```

Codex 原生 `commandExecution` 与 daemon PTY 不能混用。

### 4.4 图二：任务完成但无文本输出

Codex 当前最终输出处理：

```text
G:\项目1\pocketagent\daemon\internal\agent\codex.go:1395-1412
```

当前逻辑：

```go
text := streamText.String()
if text == "" {
    text = finalText.String()
}
if text == "" {
    text = "(完成,无文本输出)"
}
```

正文来源已有三层：

```text
G:\项目1\pocketagent\daemon\internal\agent\codex.go:1019-1027
G:\项目1\pocketagent\daemon\internal\agent\codex.go:1030-1037
G:\项目1\pocketagent\daemon\internal\agent\codex.go:1084-1093
```

已有 `thread/read` 解析能力：

```text
G:\项目1\pocketagent\daemon\internal\agent\codex_threadread.go:1-109
```

但还没有接入“最终正文为空时的自动历史补偿”。

目前不能仅凭图片断言一定是上下文压缩造成的。需要用 fixture 区分：

1. 正常 `agentMessage/delta`；
2. 只有 `item/completed.agentMessage`；
3. 只有 `turn/completed.turn.items.agentMessage`；
4. 压缩前有 delta，压缩后没有 delta；
5. `turn/completed` 结构变化；
6. 只有图片、工具或浏览器产物；
7. 真正无文本。

### 4.5 事件序列化失败会把原帧替换成 `{}` 

Turn 事件：

```text
G:\项目1\pocketagent\daemon\internal\server\event_sequence.go:30-39
```

主机事件：

```text
G:\项目1\pocketagent\daemon\internal\server\event_sequence.go:153-161
```

当前序列化失败时：

```go
payload = []byte("{}")
```

这会丢失原始事件，直接影响：

```text
断线回放
未知事件诊断
终局识别
上下文压缩状态恢复
输出补偿
```

必须改成 UTF-8 规范化、原始载荷限长脱敏，以及明确的 `EVENT_ENCODING_FAILED` 诊断事件。

### 4.6 断线恢复慢

Android 恢复主流程：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\TurnSession.kt:872-960
```

当前行为：

```text
首次等待 800ms
之后每个周期等待 5s
尝试重新连接
拉 GetThreadEventsSince
失败时拉 GetThreadHistory
最长等待约 30 分钟
```

增量恢复本身位于：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\TurnSession.kt:816-855
```

主要问题是：

```text
连接重试与任务恢复耦合
终局通知没有作为主要驱动
每次恢复可能重新建立连接
事件轮转后才回退全量
全量回填后需要清空断点避免重复
```

### 4.7 所有 Agent 的 Runtime 持久化还未统一

当前 Runtime 后端明确说明：

```text
G:\项目1\pocketagent\daemon\internal\runtime\backend.go:99-112
```

现在主要只持久化 checkpoint：

```text
memory/cache/资源记账仍走内存实现
```

因此当前不能声称所有 Agent 已经获得统一的 memory/cache/resource/checkpoint 持久化能力。

### 4.8 协同主入口不完整

主页当前可以打开协同角色，但主页面没有完整的协同 route：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\MainActivity.kt:177-194
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\MainActivity.kt:288-304
```

后续应增加：

```text
composable("collab")
onOpenCollab
主页“开始协同”按钮
```

协同策略仍由用户电脑上的 daemon 决定，手机不保存权威开关。

## 5. 统一目标架构

### 5.1 控制面位置

控制面位于：

```text
用户自己的电脑
└── pocketagentd
    ├── Runtime Kernel
    ├── DeletionRegistry
    ├── EventStore
    ├── Provider Adapter
    ├── AgentRuntime
    ├── Memory/Cache/Resource/Checkpoint
    └── gRPC/TLS endpoint
```

手机：

```text
连接控制端
显示状态
提交用户输入
提交用户审批
选择协同成员
查看诊断
```

手机不是：

```text
会话权威数据库
删除权威
Agent 资源调度权威
原生会话同步权威
```

### 5.2 统一 Runtime Kernel

```text
RuntimeKernel
├── EventRouter
├── OutputNormalizer
├── DeletionRegistry
├── RecoveryCoordinator
├── MemoryManager
├── CacheManager
├── ResourceLedger
├── CheckpointManager
└── ProviderAdapter
```

每个 Agent 都必须通过同一套描述接口返回：

```text
agent_id
native_session_ref
memory_state
cache_state
resource_state
checkpoint_state
output_state
recovery_state
capabilities
```

Provider 只负责和原生 Agent 通信，不得自行决定：

```text
是否重新创建已删除 Thread
未知事件是否当成错误
空正文是否伪装成普通文本
缓存是否无限保留
恢复是否重发原 prompt
资源是否绕过 Runtime 预算
```

## 6. 删除数据模型与持久化方案

### 6.1 删除记录

建议新增：

```text
G:\项目1\pocketagent\daemon\internal\runtime\deletion_registry.go
G:\项目1\pocketagent\daemon\internal\server\deletion_filter.go
```

统一记录结构：

```text
DeletionRecord
- schema_version: uint32
- deletion_id: string
- created_at_unix_ms: int64
- local_thread_id: string
- parent_thread_id: string
- agent_id: string
- codex_thread_id: string
- external_session_id: string
- canonical_session_file: string
- child_thread_ids: repeated string
- delete_mode: LOCAL_ONLY | NATIVE_REQUESTED
- native_delete_status: NOT_REQUESTED | SUCCEEDED | RESIDUAL | UNKNOWN
- residual_reason: string
- state: ACTIVE | RESTORED
```

### 6.2 兼容现有 `threads.json`

现有结构：

```text
G:\项目1\pocketagent\daemon\internal\store\store.go:166-196
```

已有：

```go
Tombstones map[string]int64
```

迁移策略：

1. 保留读取旧 `Tombstones`；
2. 读取时转换成 `DeletionRecord`；
3. 新版本新增 `schema_version` 和 `deletion_records`；
4. 迁移成功后原子写回；
5. 迁移失败保留原文件并隔离坏文件；
6. 旧版本忽略新字段时，至少仍能读取旧墓碑；
7. 新版本不允许旧格式覆盖新格式。

### 6.3 删除语义

#### `local_only = false`

含义：

```text
从 App 默认列表移除
尝试删除原生会话
原生删除失败时保留残留诊断
默认列表和导入入口仍隐藏
只有显式恢复/重新导入动作才清除墓碑
```

#### `local_only = true`

含义：

```text
只从当前 App 隐藏
电脑原生会话保留
使用 HiddenSessions，不使用永久删除墓碑
可以通过“重新导入”恢复
```

两种语义不能混为一谈。

## 7. RPC 契约方案

### 7.1 现有 RPC 必须统一执行过滤

不增加 RPC 也能先修复删除复活，以下接口必须经过 `DeletionRegistry`：

```protobuf
rpc DeleteThread(DeleteThreadRequest) returns (Empty);
rpc ListThreads(ListThreadsRequest) returns (ListThreadsResponse);
rpc ListCodexThreads(ListCodexThreadsRequest) returns (ListCodexThreadsResponse);
rpc ImportCodexThread(ImportCodexThreadRequest) returns (ImportCodexThreadResponse);
rpc ListAgentThreads(ListAgentThreadsRequest) returns (ListAgentThreadsResponse);
rpc ImportAgentThread(ImportAgentThreadRequest) returns (ImportAgentThreadResponse);
```

错误语义：

```text
已删除且没有显式恢复：
  Import RPC → FailedPrecondition

墓碑写入失败：
  DeleteThread → Internal 或 Unavailable

原生删除失败但墓碑已经落盘：
  DeleteThread → 成功
  native_delete_status = RESIDUAL
  默认列表不返回
```

不能用响应体里的自定义错误替代 gRPC status。现有 Runtime proto 已明确使用 gRPC status：

```text
G:\项目1\pocketagent\proto\v1\runtime.proto:15-18
```

### 7.2 输出契约

建议在 `TurnCompleted` 中新增字段，保持旧字段不变：

```protobuf
enum OutputState {
  OUTPUT_STATE_UNSPECIFIED = 0;
  TEXT = 1;
  EMPTY = 2;
  ARTIFACT_ONLY = 3;
  ERROR = 4;
  RECOVERED_FROM_NATIVE_HISTORY = 5;
  ADAPTER_LOST_OUTPUT = 6;
}

message TurnCompleted {
  string turn_id = 1;
  repeated ContentBlock final_blocks = 2;
  Usage usage = 3;
  int64 elapsed_ms = 4;
  OutputState output_state = 5;
  string output_source = 6;
  string diagnostic_id = 7;
}
```

必须同时修改：

```text
G:\项目1\pocketagent\proto\v1\thread.proto
G:\项目1\pocketagent\android\app\src\main\proto\v1\thread.proto
```

并重新生成 Go/Android 代码。

### 7.3 删除诊断 RPC

第一阶段可以只通过已有 Runtime 未知事件和日志诊断残留。为了让用户能在手机上查看“原生删除失败但已隐藏”的记录，建议第二阶段新增：

```protobuf
rpc ListDeletionRecords(ListDeletionRecordsRequest)
    returns (ListDeletionRecordsResponse);

message ListDeletionRecordsRequest {
  string state = 1;       // ACTIVE / RESTORED / 空=全部
  string agent_id = 2;   // 空=全部
  int32 limit = 3;        // <=0 使用 daemon 默认上限
}

message DeletionRecord {
  string deletion_id = 1;
  int64 created_at_unix_ms = 2;
  string local_thread_id = 3;
  string agent_id = 4;
  string codex_thread_id = 5;
  string external_session_id = 6;
  string session_file = 7;
  string delete_mode = 8;
  string native_delete_status = 9;
  string residual_reason = 10;
  string state = 11;
}

message ListDeletionRecordsResponse {
  repeated DeletionRecord records = 1;
}
```

RPC 变更必须同步两份 proto，并在提交信息中说明 RPC 数量变化。

## 8. 分阶段执行方案

### D0：只读基线和故障夹具

责任范围：

```text
daemon/internal/server
daemon/internal/store
daemon/internal/agent/codex
android/ThreadsScreen
android/ThreadSnapshot
android/TurnSession
```

建立 fixture：

```text
删除后原生列表仍存在
删除后 ListCodexThreads
删除后 ListAgentThreads
删除后重新 Import
daemon 重启
App 重启
App 离线读取快照
父线程删除后迟到子代理事件
7 种 Codex 输出结构
非法 UTF-8 事件
断线和事件轮转
```

产物：

```text
G:\项目1\pocketagent\docs\verify\evidence\deletion-root-cause.md
G:\项目1\pocketagent\docs\verify\testdata\output-contract\
G:\项目1\pocketagent\docs\verify\testdata\recovery\
```

### D1：统一删除注册表

修改：

```text
G:\项目1\pocketagent\daemon\internal\runtime\deletion_registry.go
G:\项目1\pocketagent\daemon\internal\server\deletion_filter.go
G:\项目1\pocketagent\daemon\internal\server\thread.go
G:\项目1\pocketagent\daemon\internal\server\codexsessions.go
G:\项目1\pocketagent\daemon\internal\server\agents_sessions.go
G:\项目1\pocketagent\daemon\internal\server\thread_ext.go
G:\项目1\pocketagent\daemon\internal\server\pi_subagent_sync.go
G:\项目1\pocketagent\daemon\internal\server\codex_subagent_sync.go
G:\项目1\pocketagent\daemon\internal\store\store.go
```

必须新增测试：

```text
TestDeletedCodexSessionFilteredFromList
TestDeletedAgentSessionFilteredFromList
TestDeletedCodexSessionCannotBeImported
TestDeletedAgentSessionCannotBeImported
TestTombstoneWriteFailureDoesNotReportSuccess
TestDeletedSessionDoesNotReturnAfterDaemonRestart
TestDeletedParentBlocksLateChildRegistration
TestNativeDeleteFailureLeavesDiagnosticResidualButHidesSession
```

### D2：Android 删除协调器

新增：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ThreadDeletionCoordinator.kt
```

接入：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ThreadsScreen.kt:455-487
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ThreadsScreen.kt:518-547
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ThreadsScreen.kt:1267-1294
```

统一清理：

```text
ThreadSnapshot 列表快照
ThreadSnapshot 消息快照
TurnCheckpointStore
AgentConnection 模型缓存
AgentConnection effort 缓存
ThreadUsageStore
UsageCache
ThreadPins
SubAgentsState
TaskCenter
DeletedSessionStore
```

### D3：统一输出契约和 Codex 补偿

修改：

```text
G:\项目1\pocketagent\proto\v1\thread.proto
G:\项目1\pocketagent\android\app\src\main\proto\v1\thread.proto
G:\项目1\pocketagent\daemon\internal\runtime\output_contract.go
G:\项目1\pocketagent\daemon\internal\agent\codex.go
G:\项目1\pocketagent\daemon\internal\agent\codex_threadread.go
G:\项目1\pocketagent\daemon\internal\server\turn_transport.go
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\TurnSession.kt
```

实施顺序：

```text
delta
    ↓
item completed
    ↓
turn completed items
    ↓
native history/thread-read
    ↓
TEXT / ARTIFACT_ONLY / EMPTY / ADAPTER_LOST_OUTPUT
```

删除 `(完成,无文本输出)` 普通 assistant 占位逻辑。

### D4：事件序列化和恢复链

修改：

```text
G:\项目1\pocketagent\daemon\internal\server\event_sequence.go
G:\项目1\pocketagent\daemon\internal\eventstore\
G:\项目1\pocketagent\daemon\internal\server\turn_transport.go
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\AgentConnection.kt
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\TurnSession.kt
```

目标：

```text
WatchEvents 负责实时终局通知
GetThreadEventsSince 负责补缺口
GetThreadHistory 只负责事件丢窗后的全量回退
连接管理与任务恢复解耦
```

### D5：未知事件分类和诊断

修改：

```text
G:\项目1\pocketagent\daemon\internal\runtime\unknown_events.go
G:\项目1\pocketagent\daemon\internal\agent\native_notifications.go
G:\项目1\pocketagent\daemon\internal\agent\codex.go
G:\项目1\pocketagent\daemon\internal\server\runtime.go
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\RuntimeSection.kt
```

分类：

```text
KNOWN_PROJECTED
KNOWN_PASSTHROUGH
UNKNOWN_NOTIFICATION
UNKNOWN_REQUEST
UNKNOWN_RESPONSE
```

### D6：主页面协同入口

修改：

```text
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\MainActivity.kt
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\HomeScreen.kt
G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\CollabScreen.kt
```

新增：

```text
composable("collab")
onOpenCollab
主页“开始协同”按钮
```

### D7：所有 Agent 接入统一 Runtime

修改：

```text
G:\项目1\pocketagent\daemon\internal\runtime\
G:\项目1\pocketagent\daemon\internal\agent\provider.go
G:\项目1\pocketagent\daemon\internal\agent\codex.go
G:\项目1\pocketagent\daemon\internal\agent\pi.go
G:\项目1\pocketagent\daemon\internal\agent\zcode.go
G:\项目1\pocketagent\daemon\internal\agent\dsh.go
G:\项目1\pocketagent\daemon\internal\agent\openai_compat.go
G:\项目1\pocketagent\daemon\internal\agent\echo.go
```

每个 Agent 都必须能通过相同合同返回：

```text
memory
cache
resource
checkpoint
output
recovery
capabilities
```

### D8：数据目录迁移、兼容矩阵和文档同步

旧目录：

```text
%USERPROFILE%\.pocketagentd\
```

新目录：

```text
%LOCALAPPDATA%\PocketAgent\data\
```

迁移要求：

```text
检测旧目录
复制到临时目录
校验
fsync
atomic rename
写 migration marker
迁移失败保留旧目录
支持回滚
```

兼容矩阵至少包含：

| daemon | Android | 预期 |
|---|---|---|
| 旧 daemon | 旧 APK | 原有功能不回退 |
| 新 daemon | 旧 APK | 新字段被旧 APK 忽略，基础聊天和删除可用 |
| 旧 daemon | 新 APK | 新能力按 capability 降级，不伪造入口 |
| 新 daemon | 新 APK | 完整输出、删除、恢复、诊断能力 |

## 9. 量化验收标准

### 9.1 删除

以下全部满足才算修复：

```text
1. 删除后立即从本地列表消失
2. 连续刷新 10 次不重新出现
3. daemon 重启后不重新出现
4. App 重启后不重新出现
5. App 离线读快照不重新出现
6. Codex/Pi/ZCode 原生删除失败时默认列表仍不出现
7. 未显式恢复前，Import RPC 不能绕过墓碑
8. 父线程删除后，迟到子线程事件不能重建子线程
9. 消息快照和 checkpoint 被清理
10. 墓碑写入失败时 DeleteThread 不返回成功
```

### 9.2 输出

```text
1. 7 类输出 fixture 全部通过
2. 空文本不再生成普通 assistant 占位文本
3. ARTIFACT_ONLY 和 EMPTY 可区分
4. 压缩后仍能取得最终正文
5. 适配器丢失正文时显示明确诊断，不显示“任务完成”
```

### 9.3 断线恢复

```text
网络恢复后 5 秒内开始补事件
60 秒内重新完成鉴权
事件保留窗口内补齐率 100%
sequence 不重复应用
收到终局事件后不等待下一次 5 秒轮询
只有事件丢窗时才调用全量历史
```

### 9.4 未知事件

```text
已知 reasoning/userMessage/summary 类事件不再污染 UNKNOWN
UNKNOWN_NOTIFICATION 不阻塞主流程
UNKNOWN_REQUEST 不自动批准
UNKNOWN_RESPONSE 不使连接崩溃
原始载荷可查询、脱敏、限长
```

### 9.5 所有 Agent

```text
Codex / Pi / ZCode / DSH / OpenAI Compatible / Echo
全部通过同一套 Runtime 合同测试
每家都能显示 memory/cache/resource/checkpoint/recovery 状态
```

## 10. 审核方案

### 10.1 变更定级

本次改动预计命中：

```text
proto/**                    → 甲类
daemon/internal/runtime/** → 甲类
daemon/internal/server/**  → 乙类
daemon/internal/agent/**   → 乙类
android/**/ui/**            → 乙类
*.md                        → 丁类
```

不能把 Runtime、删除注册表和输出契约降级成普通小修复。

### 10.2 L0

按照：

```text
G:\项目1\pocketagent\AUDIT.md:34-45
```

执行：

```powershell
Set-Location G:\项目1\pocketagent
python .\tools\audit\pa_audit.py --json
```

确认：

```text
两份 proto 内容完全一致
生成代码与 proto 同批更新
APK 包含新增类和字面量
daemon 运行的是新版本
```

### 10.3 L1

Daemon：

```powershell
Set-Location G:\项目1\pocketagent\daemon
go test -count=1 ./...
CGO_ENABLED=1 go test -race ./internal/server ./internal/agent ./internal/store ./internal/codexapp ./internal/piapp ./internal/zcodeapp
```

当前只读检查时 PowerShell 找不到 `go` 命令，因此正式执行前必须先确认 Go 1.27.1 已加入 PATH，或使用项目配置的 Go 可执行文件路径。命令不可用不能记为测试通过。

Android：

```powershell
Set-Location G:\项目1\pocketagent
python .\tools\device\_pa_gradle.py test
```

必须：

```text
使用 --rerun
解析 tests/failures/errors
核对结果 XML 时间戳为本轮生成
不能接受 Gradle UP-TO-DATE 假绿
```

### 10.4 L2

真机流程：

```text
电脑端保留原生会话
手机删除
连续刷新 10 次
退出 App
重启 App
断网
恢复网络
重启 daemon
重新进入会话管理
```

每条证据包含：

```text
APK SHA-256
daemon version/commit
测试时间
结论
证据路径
失败归因
```

证据目录：

```text
G:\项目1\pocketagent\docs\verify\evidence\
```

截图和录屏不入 Git，只提交结论 Markdown/JSON。

## 11. README、AUDIT 和 Git 同步

### 11.1 README

实现和验收完成后更新：

```text
G:\项目1\pocketagent\README.md
```

必须同步：

```text
实际 HEAD
实际 dirty/clean 状态
RuntimeService RPC 数
删除墓碑行为
输出契约
恢复 RTO/RPO
未知事件分类
所有 Agent Runtime 接入状态
协同主页面入口
daemon 日志真实路径
当前未完成项
```

### 11.2 AUDIT

只在现有 `AUDIT.md` 中补充：

```text
删除复活回归用例
输出契约回归用例
非法 UTF-8 回归用例
墓碑写入失败验收
事件恢复指标
所有 Agent Runtime 合同测试
```

不再另造与 AUDIT 冲突的 L0/L1/L2 体系。

### 11.3 提交顺序

不得覆盖当前未提交改动。建议将变更拆成：

```text
fix(runtime): prevent deleted sessions from resurrection
fix(runtime): normalize empty agent outputs
fix(events): preserve invalid utf8 diagnostics
fix(recovery): use durable event replay for reconnect
feat(ui): expose collaboration entry from home
feat(runtime): unify agent memory cache resource checkpoint
docs: reconcile runtime control-plane truth
```

不要把源码、APK、截图、临时日志和文档真值更新混成一个无法审核的大提交。

## 12. 非目标

本轮不做：

```text
1. 不把 Tailscale 嵌入 PocketAgent
2. 不建立云中继服务器
3. 不把手机变成状态权威
4. 不修改用户 ~/.codex/config.toml、~/.pi、~/.dsh 等桌面配置
5. 不伪造 Codex thread 内命令 stdin 写入能力
6. 不自动批准未知 request
7. 不用静态 agent/model 白名单替代能力声明
8. 不因为一次原生删除失败就让会话重新进入默认列表
9. 不把旧 APK 强行升级成必须理解新字段
10. 不在未完成测试和产物自证前更新 README 为“已完成”
```

## 13. 待决策事项

以下问题需要在实施前明确，不能靠代码默认猜测：

1. 删除墓碑的保留期限是永久、180 天，还是由用户在诊断页显式清除？
2. `local_only=false` 且原生删除失败时，是否允许提供“查看残留并重新导入”入口？
3. 是否在第一阶段新增 `ListDeletionRecords` RPC，还是先使用日志和现有 Runtime 诊断？
4. 新数据目录迁移是否在本轮执行，还是先保留旧目录兼容并另开迁移任务？
5. Codex 空文本补偿的 `thread/read` 最大等待预算是否定为 5 秒？
6. 断线恢复的终局通知是否统一由 `WatchEvents` 驱动，还是保留短周期轮询作为兼容 fallback？

## 14. 最终完成定义

只有同时满足以下条件，才可以说本轮完成：

```text
删除会话在所有列表、导入、缓存、重启和断线场景下不复活
空文本有明确 OutputState，不再伪装成普通文本
Codex 压缩后能够通过原生历史补偿恢复最终正文
事件序列化失败不会再变成 {}
断线恢复达到 RTO/RPO 目标
未知事件按通知/request/response 分类
主页存在可用的协同入口
所有 Agent 通过统一 Runtime 合同暴露 memory/cache/resource/checkpoint
README 与 AUDIT 反映实际状态
Git 提交可按功能和审核边界追溯
L0/L1/L2 全部通过
```

