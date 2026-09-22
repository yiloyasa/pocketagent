# PocketAgent 本机运行时内核与统一 Agent Runtime 设计

**日期**：2026-09-22
**状态**：架构设计规格（评审修订版），尚未实现
**适用范围**：Windows daemon、Android App、原生 agent 适配层、分发包、审核体系
**核心目标**：把当前已经存在的多个功能，收敛为一套由统一运行时约束的系统。

**规范依据**：

- 项目整体上下文：[`README.md`](../../../README.md)，当前唯一上下文，尤其是 §0、§3、§4、§13、§15。
- 项目审核标准：[`AUDIT.md`](../../../AUDIT.md)，当前唯一审核标准。
- 本文只负责补充目标架构、契约草案、实施顺序和验收口径；与 `README.md` 或 `AUDIT.md` 冲突时，代码库事实以项目文档和源码为准，审核执行以 `AUDIT.md` 为准。

---

## 1. 结论

PocketAgent 不应继续以“连接、会话、Codex、Pi、协同、缓存、终端各自实现”的方式扩展。目标架构应当是：

```text
用户手机上的 PocketAgent App
        │
        │ 用户自己的 Tailscale / 局域网 / USB
        │
用户自己的电脑上的 pocketagentd
        │
        ├── 本机运行时内核（Local Runtime Control Plane）
        ├── Codex / Pi / ZCode / DSH / OpenAI Compatible / Echo
        ├── 用户自己的项目、会话、文件和凭据
        └── 用户自己的资源预算、策略和恢复状态
```

这里的“控制面”只存在于**每一个用户自己的电脑**上的 daemon 内部，不存在于开发者电脑，也不要求 PocketAgent 提供云中继。

手机是控制台，不是运行时权威；Tailscale 是用户自己的网络，不是 PocketAgent 的共享服务；原生 agent 是执行插件，不是 PocketAgent 的全局状态中心。

目标状态：

```text
所有 RPC、原生 agent 事件、MCP 委派、终端操作、连接恢复和 UI 状态
        ↓
统一 OperationContext
        ↓
本机运行时内核
        ├── 统一身份与配对
        ├── 统一任务与状态机
        ├── 统一持久化与恢复
        ├── 统一内存与缓存预算
        ├── 统一事件路由与未知事件透析
        ├── 统一 agent 能力注册
        ├── 统一协同调度
        └── 统一诊断和审计
```

---

## 2. 用户电脑、手机和开发机的边界

| 对象 | 所在位置 | 权威职责 |
|---|---|---|
| 用户本机运行时内核 | 用户自己的电脑上的 `pocketagentd` | 管理任务、会话、agent、资源、策略、配对、恢复和事件 |
| Android 控制台 | 用户手机 | 发命令、显示状态、提交审批、查看诊断 |
| 执行面 | 用户电脑上的 Codex、Pi、ZCode、DSH 等进程 | 执行模型循环、工具、文件和命令 |
| 网络面 | 用户自己的 Tailscale | 连接手机和用户电脑 |
| 开发/发布环境 | 项目维护者电脑和 CI | 构建、测试、签名、打包；不进入用户运行链路 |

发布包不得包含或依赖：

- 开发者电脑路径；
- 开发者电脑地址；
- 开发者的 Tailscale 身份；
- 开发者的 token、配对码和 agent 配置；
- 开发者的 `~/.codex`、`~/.pi`、`~/.dsh`；
- PocketAgent 云端中继。

用户安装后，所有运行数据使用用户自己的目录，例如：

```text
%LOCALAPPDATA%\PocketAgent\
├── config\
├── data\
│   ├── installation.json
│   ├── devices.json
│   ├── endpoints.json
│   ├── runtime-policy.json
│   ├── threads.json
│   ├── tasks.json
│   ├── journal\
│   ├── events\
│   ├── checkpoints\
│   ├── memory\
│   ├── cache\
│   └── certs\
├── logs\
└── bin\
```

---

## 3. 当前实现与目标架构的差距

当前项目已经具备较多功能，但这些能力还没有全部经过一个统一的运行时内核。

| 当前实现 | 问题 | 目标改造 |
|---|---|---|
| `server.Server` 直接持有多个状态表 | 状态所有权分散 | 由 `runtime.Kernel` 统一持有并提供接口 |
| `hostID` 在 `server.New` 中使用 `randomID("host")` | daemon 重启后主机身份变化 | 使用持久化 `installation_id` / `host_id` |
| `turnQueue` 主要是进程内队列 | daemon 重启后排队输入丢失 | 进入 Durable Task Journal |
| `CollabPrefs` 在 Android `SharedPreferences` | 手机开关不是真正运行策略 | 由 daemon `runtime-policy.json` 作为唯一事实来源 |
| `capabilities_cache.go` 以 Codex 查询缓存为中心 | Pi、ZCode、DSH 等没有统一缓存契约 | 公共 `CacheManager` + agent namespace |
| 各 provider 自己维护 session map、model cache、projection 等 | 状态语义不一致，容易漏恢复 | 公共 `AgentRuntime` + `NativeSessionAdapter` |
| Android `TurnSession` 拥有大量进程内流式状态 | App 重启和内存压力时恢复边界不统一 | 有界内存窗口 + daemon checkpoint + 事件重放 |
| 各 agent 独立解析未知事件 | 新协议事件可能被丢弃或造成分叉 | 统一 `EventEnvelope` + `EventRouter` |
| 事件、工具输出、子 agent artifact 分散处理 | 无统一预算和留存策略 | `ResourceManager` + `ArtifactStore` |

这些差距不是要求推倒重写，而是要求后续新增和迁移的功能必须经过统一内核。

---

## 4. 领域模型

### 4.1 Installation

代表用户电脑上的一次 PocketAgent 安装。

```text
installation_id
installation_key
host_id
created_at
schema_version
```

`installation_id` 和 `host_id` 都必须持久化，不能由进程启动随机生成。卸载、重置配对和升级的行为必须分别定义，不能混为一谈。

### 4.2 Device

代表一个已配对的手机或其他控制端。

```text
device_id
name
public_key
token_hash
scopes
created_at
last_seen_at
revoked_at
```

配对码只用于首次建立信任；长期连接使用设备独立 token 和设备密钥。

### 4.3 Endpoint

代表一个可连接地址：

```text
endpoint_id
kind
address
host_id
certificate_fingerprint
last_success_at
last_failure_at
failure_count
```

端点失败不能触发重新配对。端点变化和设备身份变化是两个不同问题。

### 4.4 Operation

代表所有可追踪的操作，包括：

```text
创建 thread
执行 turn
提交审批
启动终端
读取文件
委派子任务
协同扇出
恢复任务
清理缓存
```

所有操作必须拥有：

```go
type OperationContext struct {
    InstallationID string
    DeviceID       string
    ThreadID       string
    TaskID         string
    TurnID         string
    CorrelationID  string
    Source         string
    Deadline       time.Time
    ResourceScope  string
}
```

### 4.5 AgentRuntime

代表一个 agent 在某个 thread 上的完整运行时：

```text
AgentRuntime
├── WorkingMemory
├── ConversationMemory
├── SummaryMemory
├── ArtifactMemory
├── NativeSessionState
├── CacheNamespace
├── ResourceLedger
├── Checkpoint
└── RecoveryState
```

### 4.6 EventEnvelope

所有来自 agent、daemon、手机和系统的事件都必须进入统一信封：

```text
sequence
timestamp
installation_id
device_id
thread_id
task_id
turn_id
correlation_id
source_agent
source_method
protocol_version
normalized_kind
safety_class
raw_payload
decoded_fields
processing_status
```

### 4.7 ResourceLease

任何可能持续增长的内存、缓存、事件、artifact、终端输出和并发任务都必须拥有资源租约：

```text
lease_id
owner_scope
resource_kind
reserved_bytes
used_bytes
soft_limit
hard_limit
spill_policy
created_at
released_at
```

---

## 5. 本机运行时内核

新增目标目录：

```text
daemon/internal/runtime/
├── kernel.go
├── identity.go
├── operation.go
├── state.go
├── policy.go
├── journal.go
├── recovery.go
├── resource.go
├── event.go
├── capability.go
├── agent_runtime.go
├── memory.go
├── cache.go
├── checkpoint.go
└── collab.go
```

核心结构：

```go
type Kernel struct {
    Identity     *IdentityManager
    State        *StateStore
    Journal      *Journal
    Recovery     *RecoveryManager
    Resources    *ResourceManager
    Events       *EventRouter
    Capabilities *CapabilityRegistry
    Policies     *PolicyEngine
    Endpoints    *EndpointManager
    Agents       *AgentRuntimeRegistry
    Collab       *CollabCoordinator
}
```

`server`、`agent`、`term`、`eventstore` 和 `collabmcp` 不应继续各自定义跨模块状态的事实来源。它们可以保留实现细节，但跨模块状态必须经由 `Kernel`。

---

## 6. 所有 agent 的统一内存、记忆和缓存机制

这是本设计的硬性要求：

> **任何 agent 接入 PocketAgent，都必须拥有统一的运行时内存、语义记忆、缓存、资源预算、checkpoint 和恢复接口。不能因为某个 agent 没有 Codex 类似的原生 API，就退化为无记忆、无缓存、无恢复的临时执行器。**

### 6.1 三种资源必须分开

#### 运行时 RAM

```text
当前回复
事件缓冲
工具输出
终端输出
协同状态
session 映射
```

要求：

```text
有预算
有硬上限
实时记账
可以落盘
可以回收
```

#### Agent 语义记忆

```text
当前轮次上下文
会话历史
历史摘要
工具结果
用户偏好
工作区信息
外部原生 session 引用
```

这是用户数据或可恢复过程数据，不得被当作普通缓存静默删除。

#### 可再生缓存

```text
模型列表
能力列表
账号状态
MCP 状态
session 索引
事件索引
artifact 索引
文件预览
```

缓存必须有 TTL、容量上限、命中统计和淘汰策略。

### 6.2 公共 AgentRuntime 接口

目标接口：

```go
type AgentRuntime struct {
    AgentID       string
    ThreadID      string
    SessionID     string
    Memory        *MemoryManager
    Cache         *CacheManager
    Resources     *ResourceLedger
    Checkpoint    *CheckpointManager
    NativeSession NativeSessionAdapter
    Recovery      *RecoveryState
}
```

公共内存接口：

```go
type MemoryManager interface {
    Load(ctx context.Context, key MemoryKey) (*MemorySnapshot, error)
    Append(ctx context.Context, key MemoryKey, item MemoryItem) error
    ReadWindow(ctx context.Context, key MemoryKey, limit MemoryLimit) ([]MemoryItem, error)
    Compact(ctx context.Context, key MemoryKey, policy CompactPolicy) (*MemorySnapshot, error)
    Checkpoint(ctx context.Context, key MemoryKey) error
    Restore(ctx context.Context, key MemoryKey) error
}
```

公共缓存接口：

```go
type CacheManager interface {
    Get(ctx context.Context, key CacheKey) (CacheValue, bool, error)
    Put(ctx context.Context, key CacheKey, value CacheValue, policy CachePolicy) error
    Invalidate(ctx context.Context, namespace string) error
    Stats(ctx context.Context, namespace string) CacheStats
}
```

原生会话接口：

```go
type NativeSessionAdapter interface {
    Open(ctx context.Context, thread *store.Thread) error
    Resume(ctx context.Context, thread *store.Thread) error
    Checkpoint(ctx context.Context, thread *store.Thread) error
    Close(ctx context.Context, thread *store.Thread) error
    Source() SessionSource
}
```

`SessionSource`：

```text
NATIVE   原生 agent 会话是事实来源
DAEMON   PocketAgent 自己保存会话事实
HYBRID   原生会话保存正文，daemon 保存索引、摘要和恢复状态
NONE     不具备持久会话能力，必须使用 daemon 侧兜底
```

### 6.3 统一内存分层

#### WorkingMemory

保存当前 turn：

```text
当前输入
部分回复
当前工具调用
当前审批
当前子 agent 状态
当前错误
```

轮次结束后归档或压缩，不能无限累积。

#### ConversationMemory

保存 thread 的持续上下文：

```text
用户消息
assistant 消息
工具调用和工具结果
审批记录
模型和 effort
工作区
agent
外部 session 引用
```

#### SummaryMemory

当上下文过长时保存：

```text
目标
进展
已完成工作
未完成工作
重要决策
文件变化
失败尝试
用户偏好
```

摘要必须记录：

```text
覆盖的 sequence 范围
生成时间
生成 agent
摘要版本
```

#### ArtifactMemory

工具输出、文件引用和子 agent 产出以 artifact 保存，不直接全部塞进 prompt：

```text
artifact_id
thread_id
turn_id
agent_id
kind
path
size
hash
retention
```

### 6.4 统一缓存命名空间

缓存按 agent、workspace 和 thread 隔离：

```text
agent/<agent_id>/models
agent/<agent_id>/capabilities
agent/<agent_id>/account
agent/<agent_id>/mcp
agent/<agent_id>/session-index
agent/<agent_id>/transcript-index
agent/<agent_id>/artifact-index
agent/<agent_id>/projection
```

缓存策略：

```go
type CachePolicy struct {
    TTL           time.Duration
    MaxEntries    int
    MaxBytes      int64
    StaleAllowed  bool
    PersistToDisk bool
    Eviction      EvictionPolicy
}
```

读取规则：

```text
新鲜命中       → 直接返回
过期但可使用   → 先返回旧值，后台刷新
没有缓存       → 请求 agent，成功后写入
刷新失败       → 保留旧值，产生诊断事件
```

### 6.5 初始预算模型

下面是实现阶段的初始基线，不是永久固定值；上线前要用压力测试校准。

```json
{
  "agent_id": "pi",
  "limits": {
    "ram_working_bytes": 8388608,
    "ram_event_buffer_bytes": 4194304,
    "ram_session_index_bytes": 2097152,
    "disk_conversation_bytes": 33554432,
    "disk_cache_bytes": 16777216,
    "disk_artifact_bytes": 268435456,
    "max_parallel_turns": 1
  }
}
```

预算必须可以按：

```text
全局
agent
workspace
thread
collab session
```

逐级覆盖。资源达到软上限时优先压缩和落盘；达到硬上限时拒绝新增可选资源，但不得静默删除用户数据或伪造任务完成。

---

## 7. 各 agent 的统一接入要求

不同 agent 的差异只能存在于 adapter 内部，不应改变公共 runtime 契约。

| Agent | 原生事实来源 | 必须接入的公共能力 | 特殊适配 |
|---|---|---|---|
| Codex | `thread/resume`、原生 token usage、原生 compaction | memory、cache、checkpoint、resource、recovery | 原生 session、token usage、context compaction |
| Pi | session file、`switch_session`、`get_state`、`get_session_stats` | memory、cache、artifact、checkpoint、recovery | session file 索引、子 agent artifacts、统计用量 |
| ZCode | `session/create`、`session/resume`、projection、session events | memory、cache、projection、checkpoint、recovery | session 映射、projection 快照、子 agent 能力差异 |
| DSH | `session/list`、`session/new`、`session/resume`、`session/update` | memory、cache、event spool、checkpoint、recovery | session 索引、config catalog、原始 update 事件 |
| OpenAI Compatible | 通常没有稳定原生 session | daemon conversation memory、summary、artifact、cache、checkpoint | daemon 是主要事实来源 |
| Echo | 无外部持久 session | 完整接入公共 runtime | 作为公共契约和恢复测试 provider |

### Codex

现有 `capCache`、`mcpCache`、`usageCache` 应迁移到公共 `CacheManager` 的 Codex namespace。Codex 仍可实现原生 compaction，但 compaction 的 checkpoint、资源记账和 UI 投影必须由公共 runtime 管理。

### Pi

Pi 的 session file 是原生事实来源，daemon 维护索引、摘要、artifact 索引和恢复状态。`modelsCache`、session 映射和统计数据不能只存在 provider 的进程内字段。

### ZCode

`threadMap`、`projByThread` 和命令快照应拆为公共 `NativeSessionState`、`ProjectionCache` 和 `RuntimeCheckpoint`。子进程重启后必须从持久化引用重新挂载，而不是依赖旧内存 map。

### DSH

session 目录和 `session/list` 是原生事实来源；`session/update` 的原始数据进入统一 EventRouter 和 event spool。config catalog 使用公共缓存策略。

### OpenAI Compatible

由于通常没有可恢复的原生 session，daemon 必须保存 conversation memory，并在每次请求前按预算生成上下文：

```text
system context
→ summary
→ recent messages
→ relevant artifacts
→ current user input
```

不能在 `openai_compat.go` 内部继续维护一套不受 ResourceManager 约束的消息历史。

### Echo

Echo 也必须接入完整 runtime，不能因为只是测试 provider 就跳过 memory、cache、checkpoint 和 resource。它应作为所有 provider 的最低契约测试对象。

---

## 8. 状态、持久化和恢复

### 8.1 统一状态机

操作状态统一为：

```text
CREATED
→ ACCEPTED
→ RUNNING
→ WAITING_APPROVAL
→ RECOVERING
→ COMPLETED
→ FAILED
→ CANCELLED
```

设备连接状态：

```text
UNPAIRED
→ PAIRING
→ PAIRED
→ CONNECTING
→ READY
→ DEGRADED
→ RECONNECTING
→ REVOKED
```

协同成员状态：

```text
CREATED
→ ACCEPTED
→ RUNNING
→ WAITING
→ COMPLETED
→ FAILED
→ CANCELLED
```

UI 不得自行推断权威状态，只显示 daemon 返回的状态投影。

### 8.2 统一 Journal

权威状态和可恢复过程数据必须通过统一 Journal 写入：

```text
写临时文件
→ flush
→ fsync
→ atomic rename
→ 更新 journal
→ 周期性 checkpoint
```

启动恢复顺序：

```text
权威快照可读       → 使用快照
快照损坏、journal 完整 → 从 journal 恢复
两者均损坏         → 进入明确的 CORRUPTED 状态
```

设备表损坏时不能静默按空表启动并让用户误以为所有设备消失。应暂停远程认证并提供恢复、备份或重新配对路径。

### 8.3 任务和队列

当前仅存在于内存中的：

```text
queued turn
collab inflight
pending approval
resource reservation
```

都必须有持久化任务记录。重启后的任务必须进入：

```text
RUNNING
WAITING_APPROVAL
RECOVERY_REQUIRED
COMPLETED
FAILED
```

不能自动重新发送原始 prompt，避免重复执行副作用。

---

## 9. 统一事件和未知事件透析

所有 provider 事件经过：

```text
原生 agent
    ↓
adapter decode
    ↓
EventEnvelope
    ↓
EventRouter
    ├── 标准事件
    ├── 未知事件
    ├── 诊断事件
    ├── 持久化
    ├── UI 投影
    └── 恢复索引
```

标准分类只是投影，不构成封闭名单：

```text
TEXT
TOOL_CALL
APPROVAL_REQUEST
USER_INPUT_REQUEST
SUBAGENT_ACTIVITY
STATUS
COMPLETION
ERROR
UNKNOWN_NOTIFICATION
UNKNOWN_REQUEST
UNKNOWN_RESPONSE
```

处理规则：

### 未知 notification

```text
保留原始 payload
→ 分配 sequence
→ 写入 event store
→ 推送通用诊断事件
→ 不影响主 stream
```

### 未知 request

```text
保留原始 payload
→ 关联 thread / turn
→ 能安全拒绝则返回协议错误
→ 不能安全拒绝则标记 NEEDS_NATIVE_SUPPORT
→ 绝不默认批准
```

### 未知 response

```text
保留 request_id 和原始 payload
→ 关联发起操作
→ 写入诊断索引
→ 不让整个连接崩溃
```

Android 增加：

```text
Settings
└── Runtime Diagnostics
    ├── Unknown Events
    ├── Pending Native Requests
    ├── Capability Mismatch
    ├── Connection Recovery
    └── Resource Pressure
```

显示具体来源和 method，不显示没有信息量的“未知错误”。

---

## 10. Agent 能力统一

目标结构：

```go
type AgentDescriptor struct {
    ID               string
    DisplayName      string
    Version          string
    Capabilities     []Capability
    SessionModel     SessionModel
    ResumeModel      ResumeModel
    ApprovalModel    ApprovalModel
    SubagentModel    SubagentModel
    MemoryModel      MemoryModel
    CacheModel       CacheModel
    Limitations      []Limitation
}
```

能力示例：

```text
thread.create
thread.resume
turn.stream
turn.steer
approval.request
approval.resume
subagent.spawn
subagent.events
subagent.transcript
collab.delegate
session.export
runtime.capabilities
memory.native
memory.compaction
cache.native
```

UI 只能根据 capability 显示入口：

```text
has("subagent.transcript")
    → 显示完整子 agent 对话入口

没有该能力
    → 只显示工具参数和产出
```

禁止用 agent 名字决定能力，除非是已登记的协议兼容分支。

---

## 11. 多 agent 协同

协同分为三类：

### 原生子 agent

原生 agent 自己决定是否创建子 agent。PocketAgent 负责发现、记录、显示、恢复和资源预算。

### 显式委派

用户或父 agent 指定目标 agent，生成有父子关系的独立 thread。

### 并行协同扇出

用户选择多个 agent，对等执行同一指令：

```text
用户指令
    ├── Codex thread
    ├── Pi thread
    ├── ZCode thread
    └── DSH thread
```

每个成员拥有独立：

```text
AgentRuntime
Memory
Cache namespace
Resource lease
Checkpoint
Failure state
```

一个成员失败不能默认取消其他成员。

协同策略必须由 daemon 持久化：

```json
{
  "collaboration_enabled": true,
  "native_delegate_enabled": true,
  "fanout_enabled": true,
  "auto_summarize": false,
  "max_parallel_members": 3,
  "max_session_bytes": 134217728,
  "retain_artifacts": true
}
```

Android 的 `CollabPrefs` 只能保留页面显示偏好，不能作为真正的协同权限事实来源。

---

## 12. Android UI 改造

新增通用运行时界面：

```text
Settings
├── Connection
├── Pairing Devices
├── Agent Runtime
│   ├── Codex
│   ├── Pi
│   ├── ZCode
│   ├── DSH
│   ├── OpenAI Compatible
│   └── Echo
├── Memory and Cache
├── Collaboration
└── Runtime Diagnostics
```

每个 agent 至少显示：

```text
运行状态
当前 session
内存占用
会话记忆占用
缓存占用
事件缓冲占用
artifact 占用
是否支持 resume
是否支持 compaction
是否支持子 agent
最近一次恢复结果
```

统一组件：

```text
AgentMemoryCard
AgentCacheCard
AgentContextCard
AgentSessionCard
AgentResourceCard
```

Codex 可以额外显示原生 token usage 和 context compaction；其他 agent 也必须显示自己的等价能力或明确显示“该 agent 未报告此项”，不能整个 agent 没有内存和缓存面板。

---

## 13. 实施计划与优先级

### P0：用户运行边界

目标：

```text
用户电脑独立运行
安装包不包含开发者环境
Tailscale 只使用用户自己的网络
workspace、data、凭据完全按用户隔离
```

主要位置：

```text
dist/windows/
daemon/cmd/pocketagentd/
daemon/cmd/paguard/
```

### P0.5：公共 Agent Runtime 契约

目标：

```text
所有 agent 都必须声明 AgentDescriptor
所有 agent 都必须拥有 memory/cache/checkpoint/resource/recovery 入口
Echo 先作为最低契约测试 provider
```

主要位置：

```text
daemon/internal/runtime/
daemon/internal/agent/provider.go
daemon/internal/agent/capabilities_cache.go
```

### P1-A：统一身份、状态和恢复

目标：

```text
installation_id 持久化
host_id 持久化
设备配对持久化
任务 journal
断线恢复
daemon 重启恢复
协同 session 恢复
```

优先修复：

```text
server.New 中的随机 hostID
turnQueue 重启即丢
pending approval 无完整任务 journal
```

### P1-B：统一资源管理

目标：

```text
删除生产路径的 UNLIMITED buffer
所有 agent 有独立预算
事件和工具输出可 spill 到磁盘
缓存、artifact、终端和协同任务统一计费
```

### P2-A：统一事件和未知事件透析

目标：

```text
EventEnvelope
EventRouter
UnknownEvent Inspector
unknown request 安全处理
原始 payload 可检索
```

### P2-B：统一能力注册

目标：

```text
AgentDescriptor
CapabilityRegistry
能力驱动的 server 和 UI
删除新增的 agent 名称特判
```

### P2-C：统一协同

目标：

```text
daemon 权威协同策略
delegate / fanout / native subagent 统一模型
成员级资源预算
协同 checkpoint 和恢复
统一协同 UI
```

### P3：发布增强

```text
自动升级
跨平台 daemon
更完整的诊断导出
性能仪表盘
数据迁移工具
```

---

## 14. 代码落点与 file:line 锚点

### 目标目录（尚未创建）

```text
daemon/internal/runtime/
daemon/internal/runtime/cache/
daemon/internal/runtime/memory/
daemon/internal/runtime/recovery/
```

上面是实施落点，不是当前实现证据，因此暂时没有可引用的 `file:line`。实现对应工作包时，必须把目标文件加入本节并补上实际行号；在此之前不得把这些路径写成“已存在”。

### 当前实现锚点

| 责任 | 现状锚点 | 评审结论 |
|---|---|---|
| daemon 组装、数据目录、鉴权模式、TLS、gRPC keepalive | `daemon/cmd/pocketagentd/main.go:46-57`, `:104-135`, `:199-250` | 迁移入口；保留默认鉴权、TLS 和 keepalive 语义 |
| server 状态容器 | `daemon/internal/server/server.go:38-84` | 改为注入 `runtime.Kernel`，不再让 `Server` 独立拥有跨模块事实 |
| daemon 启动时生成 host ID | `daemon/internal/server/server.go:88-103`，尤其 `:99` | 必须改为持久化 `installation_id` / `host_id` |
| eventstore 初始化失败降级内存 | `daemon/internal/server/server.go:91-95` | 不能无提示继续提供“可恢复”语义；必须暴露 `PERSISTENCE_DEGRADED` |
| 运行中 turn 队列 | `daemon/internal/server/turn_queue.go:26-28`, `:43-60`, `:184-190` | 当前每线程上限 10，但重启丢失；迁移为 Durable Task Journal |
| gRPC 单写者和背压 | `daemon/internal/server/turn_transport.go:12-26`, `:41-71` | 保留单写者模型；ResourceManager 负责 `pending` 预算，不能绕过 writer |
| 设备配对持久化 | `daemon/internal/server/devices.go:33-95`, `:121-183`, `:273-283` | 保留每设备 token、挑战应答和撤销；补 schema、损坏恢复和撤销传播 |
| 会话落盘模型 | `daemon/internal/store/store.go:26-79`, `:81-197`, `:199-250`, `:278-300` | 现有字段作为 legacy v0 输入；新格式增加 envelope 和 schema version |
| 事件 JSONL 形状和轮转 | `daemon/internal/eventstore/eventstore.go:9-22`, `:37-73`, `:150-194`, `:196-223` | 现有 10 MiB + 1 代轮转是兼容基线；journal 另设，不覆盖 eventstore 语义 |
| Provider 基础接口 | `daemon/internal/agent/provider.go:21-69` | 扩展 `AgentDescriptor`、`AgentRuntime` 和 memory/cache contract |
| Codex 私有缓存 | `daemon/internal/agent/capabilities_cache.go:1-13`, `:22-55`, `:88-140` | 迁移为公共 CacheManager 的 Codex namespace |
| Codex 原生 session / compaction | `daemon/internal/agent/codex.go:803-900`，事件映射见同文件 `:1100-1250` | 作为 `NATIVE` session adapter，不再作为全局架构特例 |
| Pi session、模型缓存、统计 | `daemon/internal/agent/pi.go:65-97`, `:160-184`, `:546-620` | 迁移为 Pi AgentRuntime 的原生 session、cache 和 checkpoint |
| ZCode thread map、projection | `daemon/internal/agent/zcode.go:90-149`, `:497-532`, `:718-730` | 从 provider 私有 map 迁移为 NativeSessionState / ProjectionCache |
| DSH session map 和事件排空 | `daemon/internal/agent/dsh.go:123-150`, `:466-520`, `:618-767` | session index、event spool 和恢复纳入公共 runtime |
| Android 端点、token、TOFU 和 keepalive | `android/app/src/main/java/dev/pocketagent/app/AgentConnection.kt:158-166`, `:198-243`, `:322-442`, `:535-635`, `:1003-1027` | 保留手机侧密钥和指纹存储；连接事实与策略由 daemon 补全 |
| Android 证书指纹锁定 | `android/app/src/main/java/dev/pocketagent/app/PinTrustManager.kt:21-25`, `:71-97` | 保留 TOFU、指纹不匹配即阻断和清除指纹入口 |
| Android 设备密钥与挑战应答 | `android/app/src/main/java/dev/pocketagent/app/DeviceKey.kt:16-28`, `:65-105`, `:122-162` | 保留 Keystore P-256、公钥登记和挑战签名续期 |
| Android token 存储与迁移 | `android/app/src/main/java/dev/pocketagent/app/TokenStore.kt:21-29`, `:67-105`, `:120-147`, `:150-195` | 保留加密存储优先、旧明文迁移和失败降级语义 |
| Android 轮次状态与断点 | `android/app/src/main/java/dev/pocketagent/app/ui/TurnSession.kt:46-113`, `:451-506`, `:833-890` | 改为有界视图缓存；daemon checkpoint 和 event replay 是恢复事实 |
| Android 协同开关 | `android/app/src/main/java/dev/pocketagent/app/ui/CollabState.kt:146-162` | `CollabPrefs` 只能保留 UI 偏好；运行策略迁移到 daemon |

### 目标文件（尚未创建）

```text
daemon/internal/runtime/kernel.go
daemon/internal/runtime/identity.go
daemon/internal/runtime/operation.go
daemon/internal/runtime/state.go
daemon/internal/runtime/policy.go
daemon/internal/runtime/journal.go
daemon/internal/runtime/recovery.go
daemon/internal/runtime/resource.go
daemon/internal/runtime/event.go
daemon/internal/runtime/capability.go
daemon/internal/runtime/agent_runtime.go
daemon/internal/runtime/memory.go
daemon/internal/runtime/cache.go
daemon/internal/runtime/checkpoint.go
daemon/internal/runtime/collab.go
daemon/internal/runtime/cache/*
daemon/internal/runtime/memory/*
daemon/internal/runtime/recovery/*
```

这些条目只用于依赖和责任分配，不能用于丁类“锚点漂移”核验。文件创建后，提交必须同时更新本节为 `path:line`，并由 `AUDIT.md:26` 的文档审核复核。

### Android 现有迁移落点

```text
android/app/src/main/java/dev/pocketagent/app/AgentConnection.kt
android/app/src/main/java/dev/pocketagent/app/TokenStore.kt
android/app/src/main/java/dev/pocketagent/app/DeviceKey.kt
android/app/src/main/java/dev/pocketagent/app/ui/TurnSession.kt
android/app/src/main/java/dev/pocketagent/app/ui/TurnDelivery.kt
android/app/src/main/java/dev/pocketagent/app/ui/CollabState.kt
android/app/src/main/java/dev/pocketagent/app/ui/SettingsScreen.kt
android/app/src/main/java/dev/pocketagent/app/ui/component/PaCache.kt
```

这里列出的 Android 文件均已存在；对应的当前行号见上面的“当前实现锚点”表。只列文件名的路径表不是审核证据。

### 协议现状与目标

如果新增运行时诊断和策略 RPC，应新增或扩展：

```text
proto/v1/runtime.proto
android/app/src/main/proto/v1/runtime.proto
```

两份 proto 必须保持完全一致，生成代码不能手改。

当前协议真源仍是 `proto/v1/*.proto` 与 Android 镜像；例如通用 `Error` 的字段定义见
`proto/v1/common.proto:1-20`，主机能力探测与版本兼容入口见
`proto/v1/host.proto:1-42`。`runtime.proto` 尚未创建，本设计 §21 是其实现前的完整草案，不代表当前已有服务。

建议 RPC：

```text
GetRuntimeState
GetAgentRuntime
GetResourceUsage
SetRuntimePolicy
ListUnknownEvents
GetUnknownEvent
AcknowledgeUnknownEvent
RecoverTask
```

---

## 15. 审核执行方案（严格引用 `AUDIT.md`）

本节不定义新的 L0/L1/L2。变更分级和三层审核以 `AUDIT.md:17-51` 为唯一标准；本文只把本架构需要附加的检查对象、命令和证据写清楚。

### 15.1 甲/乙/丙/丁先定级

```powershell
Set-Location G:\项目1\pocketagent

git diff --check
python .\tools\audit\pa_audit.py `
  --repo . `
  --json
```

`pa_audit.py` 默认运行六条规则；`--only` 只用于局部排查，不能作为甲类、乙类或发版审核结论。本文涉及 `proto/**`、新增 RuntimeService、新增公共 agent contract，因此按 `AUDIT.md:23` 定为甲类；只改 daemon/Android 功能按 `AUDIT.md:24` 定为乙类；只改本文则按 `AUDIT.md:26` 只查真值锚点漂移。

### 15.2 AUDIT L0：产物自证

L0 只执行 `AUDIT.md:34-40` 的产物自证，不把“架构检查”改名为 L0：

```text
APK：按 AUDIT.md:38 用 apkanalyzer 检查本轮新增类和 dex 字面量；
daemon：按 AUDIT.md:39 确认运行中的 pid/7246 是本轮新进程；
甲类契约：补充 proto 双副本逐文件校验，但不替代产物自证。
```

### 15.3 AUDIT L1：自动化防假绿

```powershell
Set-Location G:\项目1\pocketagent

python tools\device\_pa_gradle.py test --rerun
Set-Location .\daemon
go test ./...
CGO_ENABLED=1 go test -race ./internal/server ./internal/agent ./internal/store ./internal/eventstore
```

另外运行本文 §15.1 的 `pa_audit.py` 全量扫描和现有 `AUDIT.md:117` 入口要求。Android 结果必须检查 XML 的 tests/failures、时间戳和 cleanapk 口径，不能用增量 APK 代替。

### 15.4 AUDIT L2：真机验收

按 `AUDIT.md:47-52` 执行受影响模块全量和其余冒烟；甲类里程碑执行全量三层。本文 §16 的网络、恢复、未知事件和多 agent 场景是 L2 的用例补充，不是新审核层级。每个 BLOCKED 必须写归因和解封条件，证据绑定 `AUDIT.md:97-103` 的五要素。

### 15.5 发布包补充检查（不是 AUDIT 层级）

发布包检查是本文的附加准出项，不称为 L3：

```text
proto 两副本不一致：阻塞甲类契约核验
新增硬编码 agent 名单：按 AUDIT.md:69-85 设计原则处理
新增无界 buffer：资源预算验收失败
未知事件被静默丢弃：设计原则和 L2 用例失败
未知请求被默认批准：AUDIT.md:59 红线
发布包包含开发机路径或凭据：发布包拒绝
某个 agent 没有公共 runtime 接口：统一 Runtime contract 不成立
```

### 15.6 本架构的自动化用例集合

Daemon：

```powershell
Set-Location G:\项目1\pocketagent\daemon

go build ./...
go vet ./...
go test -count=1 ./internal/...
go test -race ./internal/server ./internal/agent ./internal/store ./internal/eventstore
```

所有 agent 都必须通过同一套 runtime contract：

```text
MemoryAppendTest
MemoryLoadTest
MemoryCompactionTest
MemoryCheckpointTest
MemoryRestoreTest
CacheHitTest
CacheMissTest
CacheTTLTest
CacheEvictionTest
ResourceLimitTest
NativeSessionRecoveryTest
```

至少覆盖：

```text
Codex
Pi
ZCode
DSH
OpenAI Compatible
Echo
```

Android：

```powershell
Set-Location G:\项目1\pocketagent
pwsh -NoLogo -NoProfile -File .\android\build-ascii.ps1 test
```

新增测试方向：

```text
AgentRuntimeContractTest
AgentMemoryStateTest
AgentCacheBudgetTest
ConnectionRecoveryTest
UnknownEventRenderingTest
CollabPolicyAuthorityTest
MultiAgentResourceIsolationTest
```

### 15.7 本架构的真机用例集合

至少测试：

| 场景 | 验收结果 |
|---|---|
| 手机和电脑都在线 | 正常连接 |
| Wi-Fi 切移动网络 | 自动恢复，不重新配对 |
| Tailscale 短暂断开 | 进入 DEGRADED / RECONNECTING |
| daemon 重启 | token、installation、thread 保留 |
| 手机 App 被杀 | 重开后恢复 thread，不重发 prompt |
| turn 执行中断网 | 事件按 sequence 补齐 |
| 审批时断网 | 审批保留或明确过期 |
| 多 agent 一家失败 | 其他成员继续 |
| 缓存过期 | 旧值降级或重新拉取 |
| 内存达到硬上限 | 停止新增可选资源，不静默删用户数据 |
| 未知 event | 进入诊断，不崩溃 |
| 未知 request | 不自动批准 |
| 撤销设备 | 该设备不能继续连接 |

### 15.8 发布包补充核对

```powershell
Set-Location G:\项目1\pocketagent
pwsh -NoLogo -NoProfile -File .\dist\windows\build.ps1
```

发布包必须在没有项目源码、没有开发者环境、没有开发者凭据的干净 Windows 用户账户中启动。

必须核对：

```text
没有开发机路径
没有开发机地址
没有开发者 token
workspace 使用当前用户配置
data 使用当前用户目录
daemon 版本和 version.txt 一致
安装、升级、卸载行为可解释
```

---

## 16. 容量和行为审核

测试不只判断“有没有崩溃”，还必须判断：

```text
内存是否按 agent 和 thread 正确记账
缓存是否按 TTL 和容量淘汰
事件是否有序且可重放
任务是否重复执行
artifact 是否越过预算
协同成员是否互相污染
daemon 重启后是否恢复
未知事件是否保留原文
```

压力场景：

```text
1000 条连续事件
长上下文 thread
多个 agent 并行
多个 collab session
大工具输出
持续终端输出
子 agent artifact
断网期间持续产生事件
原生 agent 子进程重启
daemon 强制结束和重启
```

---

## 17. 能做到什么

实现完成后可以做到：

1. 每个用户只控制自己的电脑。
2. 手机通过用户自己的 Tailscale 访问用户自己的 daemon。
3. 配对、端点、thread、task 和协同 session 可以持久恢复。
4. Codex、Pi、ZCode、DSH、OpenAI Compatible 和 Echo 都拥有统一的内存与缓存机制。
5. agent 的原生 session 可以被利用，但不会成为唯一的运行时依赖。
6. 没有原生 session 的 agent 也能通过 daemon memory 继续工作。
7. 所有主要内存、缓存、事件、artifact 和并发任务都有预算。
8. 新事件不会直接导致 daemon 崩溃或被静默丢弃。
9. 新 agent 通过 capability 接入，而不是修改多处固定名单。
10. 多 agent 可以委派、扇出、恢复、限流并独立失败。
11. 手机 UI 可以看到每个 agent 的内存、缓存、session、资源和恢复状态。

---

## 18. 架构缺点和边界

### 复杂度增加

统一状态、journal、资源预算、事件路由和 agent runtime 会增加实现和测试量。这是获得持久性、一致性和可诊断性的必要成本。

### Tailscale 仍是外部依赖

PocketAgent 不能保证用户的 Tailscale 登录、网络策略、电脑在线或系统不休眠。它能保留配对、显示原因并自动恢复，但不能替用户修复网络。

### 不能完全控制外部进程 RSS

PocketAgent 可以精确控制自己管理的 RAM、缓存、队列、文件暂存和并发，但不能完全控制 Codex、Pi、ZCode、Node.js 或操作系统的全部 RSS。UI 必须区分：

```text
PocketAgent 管理范围
外部 agent 进程观测范围
```

### 原生协议仍会变化

统一 adapter 和未知事件透析能保证新事件“不崩溃、不丢失、可解释”，不能保证未声明的新能力立即自动可执行。

### 协同会增加资源和成本

多 agent 会增加 CPU、内存、token、文件竞争和结果汇总成本，因此必须有全局和成员级预算。

### 单台用户电脑仍是单点故障

电脑关机、磁盘损坏或系统损坏时，手机不能继续执行本机任务。多机容灾不属于当前范围。

---

## 19. 完成判定

本设计只有在以下条件全部满足时才算落地：

```text
[ ] 控制面明确部署在每个用户自己的电脑
[ ] 发布包不依赖开发者电脑
[ ] installation_id / host_id 持久化
[ ] 配对和端点持久化
[ ] task / approval / collab 状态可恢复
[ ] 所有 agent 通过统一 AgentRuntime
[ ] 所有 agent 都有 memory、cache、checkpoint、resource、recovery
[ ] 没有生产路径的无界缓冲
[ ] EventEnvelope / EventRouter 统一生效
[ ] 未知事件可保存、查询和展示
[ ] 未知 request 不会被默认批准
[ ] 协同策略由 daemon 权威保存
[ ] UI 按 capability 显示功能
[ ] Codex、Pi、ZCode、DSH、OpenAI Compatible、Echo 通过统一契约测试
[ ] 按 `AUDIT.md` 通过适用的 L0/L1/L2，并通过本文的发布包补充检查
```

在实现计划获批前，本文件只是目标架构规格；当前仓库中除本文件外，不应把上述目标当作已经完成的功能。

---

## 20. 现有项目文档与审核标准的对齐关系

本设计不另造一套审核流程。

| 本文内容 | `README.md` 对应 | `AUDIT.md` 对应 |
|---|---|---|
| 用户电脑 daemon、手机遥控器、无云中继 | `README.md:15-22`, `README.md:72` | `AUDIT.md:59-62` 的安全红线 |
| daemon、agent、store、eventstore、Android 连接层 | `README.md:94-124`, `README.md:138-162` | `AUDIT.md:23-26` 变更分级 |
| 事件落盘和断线重放 | `README.md:171-188` | `AUDIT.md:41-51` 自动化和真机审核 |
| agent 动态能力、未知事件、禁止新增名单 | `README.md:57`, `README.md:418` | `AUDIT.md:69-85` 设计原则合规 |
| 数据清理、缓存和用户数据边界 | `README.md:355-389` | `AUDIT.md:97-103` 证据链 |
| 发布准出 | `README.md:439-449` | `AUDIT.md:17-51`, `AUDIT.md:106-119` |

本文不替代项目审核等级。准确关系如下：

```text
甲/乙/丙/丁 = AUDIT.md §1 的变更分级
L0/L1/L2   = AUDIT.md §2 的正式审核层级
发布包补充检查 = 本设计的附加准出项，不是新的审核层级
```

其中：

- **甲类**（proto、Runtime Kernel、新 agent、新服务）必须按 `AUDIT.md:23` 执行 L0、L1、L2 和契约双向核验。
- **乙类**（daemon agent/server、Android UI、资源文件）必须按 `AUDIT.md:24` 执行 L1 和受影响 L2。
- **丙类**必须按 `AUDIT.md:25` 附带回归用例。
- **丁类**只检查真值锚点漂移，执行 `AUDIT.md:26`；本设计文档的 file:line 锚点就是丁级核验对象之一。

`pa_audit.py` 的六条规则不能由本文减少。本文上一版只列了其中四条，修订后明确：涉及代码或契约时必须运行六条规则；只运行四条的命令仅能作为局部排查，不能作为合入审核。

开发机路径的说明：

```text
文档中的 G:\项目1\pocketagent 是审核脚本在当前开发工作区运行的命令示例；
它不是发布包内路径，也不是用户 daemon 的默认路径。
```

发布包检查必须改用解压后的临时目录，并检查包内容中不存在开发机路径。

---

## 21. RuntimeService RPC 契约

本节是 P2 开工所需的 proto 契约草案。正式实现时新增：

```text
proto/v1/runtime.proto
android/app/src/main/proto/v1/runtime.proto
```

两份文件必须逐字同步；Go 生成代码放入现有 `daemon/internal/gen/pocketagent/v1/`，不能手改。

### 21.1 完整 proto 草案

```proto
syntax = "proto3";

package pocketagent.v1;

option java_multiple_files = true;
option go_package = "github.com/pocketagent/daemon/internal/gen/pocketagent/v1;pocketagentv1";

import "v1/common.proto";

service RuntimeService {
  rpc GetRuntimeState(GetRuntimeStateRequest) returns (RuntimeStateResponse);
  rpc GetAgentRuntime(GetAgentRuntimeRequest) returns (AgentRuntimeResponse);
  rpc GetResourceUsage(GetResourceUsageRequest) returns (ResourceUsageResponse);
  rpc SetRuntimePolicy(SetRuntimePolicyRequest) returns (RuntimePolicyResponse);
  rpc ListUnknownEvents(ListUnknownEventsRequest) returns (ListUnknownEventsResponse);
  rpc GetUnknownEvent(GetUnknownEventRequest) returns (UnknownEventResponse);
  rpc AcknowledgeUnknownEvent(AcknowledgeUnknownEventRequest)
      returns (AcknowledgeUnknownEventResponse);
  rpc RecoverTask(RecoverTaskRequest) returns (RecoverTaskResponse);
}

message GetRuntimeStateRequest {}

message RuntimeStateResponse {
  string installation_id = 1;
  string host_id = 2;
  string daemon_version = 3;
  string runtime_state = 4;
  uint32 schema_version = 5;
  uint64 policy_revision = 6;
  int64 last_checkpoint_unix_ms = 7;
  int64 sampled_at_unix_ms = 8;
  repeated string capabilities = 9;
  Error error = 10;
}

message GetAgentRuntimeRequest {
  string agent_id = 1;
  string thread_id = 2;
}

message AgentRuntimeResponse {
  string agent_id = 1;
  string thread_id = 2;
  string session_id = 3;
  string lifecycle_state = 4;
  string session_source = 5; // NATIVE / DAEMON / HYBRID / NONE
  repeated string capabilities = 6;
  MemoryUsage memory = 7;
  CacheUsage cache = 8;
  ResourceUsage resources = 9;
  RuntimeCheckpoint checkpoint = 10;
  Error error = 11;
}

message GetResourceUsageRequest {
  string scope = 1; // global / agent:<id> / thread:<id> / task:<id>
  bool include_children = 2;
}

message ResourceUsageResponse {
  string scope = 1;
  repeated ResourceMetric metrics = 2;
  int64 sampled_at_unix_ms = 3;
  Error error = 4;
}

message MemoryUsage {
  int64 working_bytes = 1;
  int64 conversation_bytes = 2;
  int64 summary_bytes = 3;
  int64 artifact_index_bytes = 4;
  int64 event_buffer_bytes = 5;
  int64 soft_limit_bytes = 6;
  int64 hard_limit_bytes = 7;
}

message CacheUsage {
  int64 bytes = 1;
  int64 entries = 2;
  int64 hit_count = 3;
  int64 miss_count = 4;
  int64 stale_hit_count = 5;
  int64 soft_limit_bytes = 6;
  int64 hard_limit_bytes = 7;
}

message ResourceUsage {
  int64 reserved_bytes = 1;
  int64 used_bytes = 2;
  int64 soft_limit_bytes = 3;
  int64 hard_limit_bytes = 4;
  int32 active_turns = 5;
  int32 active_subprocesses = 6;
  int32 active_collab_members = 7;
}

message ResourceMetric {
  string kind = 1;
  int64 reserved_bytes = 2;
  int64 used_bytes = 3;
  int64 soft_limit_bytes = 4;
  int64 hard_limit_bytes = 5;
  string state = 6; // NORMAL / SOFT_LIMIT / HARD_LIMIT / DEGRADED
}

message RuntimeCheckpoint {
  string checkpoint_id = 1;
  int64 sequence = 2;
  int64 created_at_unix_ms = 3;
  string state = 4; // VALID / STALE / RECOVERY_REQUIRED / CORRUPTED
  string detail = 5;
}

message RuntimePolicy {
  bool collaboration_enabled = 1;
  bool native_delegate_enabled = 2;
  bool fanout_enabled = 3;
  bool auto_summarize = 4;
  int32 max_parallel_members = 5;
  int64 max_session_bytes = 6;
  bool retain_artifacts = 7;
  int64 default_agent_memory_bytes = 8;
  int64 default_agent_cache_bytes = 9;
}

message SetRuntimePolicyRequest {
  RuntimePolicy policy = 1;
  uint64 expected_revision = 2; // 0 = no compare-and-swap
}

message RuntimePolicyResponse {
  RuntimePolicy policy = 1;
  uint64 revision = 2;
  Error error = 3;
}

message ListUnknownEventsRequest {
  string agent_id = 1;
  string thread_id = 2;
  string processing_status = 3;
  int32 page_size = 4;
  string page_token = 5;
}

message ListUnknownEventsResponse {
  repeated UnknownEventSummary items = 1;
  string next_page_token = 2;
  Error error = 3;
}

message UnknownEventSummary {
  string event_id = 1;
  int64 sequence = 2;
  string agent_id = 3;
  string thread_id = 4;
  string turn_id = 5;
  string source_method = 6;
  string normalized_kind = 7;
  string safety_class = 8;
  string processing_status = 9;
  int64 received_at_unix_ms = 10;
}

message GetUnknownEventRequest {
  string event_id = 1;
}

message UnknownEventResponse {
  UnknownEventSummary summary = 1;
  string raw_json = 2;
  string decoded_fields_json = 3;
  string handling_detail = 4;
  Error error = 5;
}

message AcknowledgeUnknownEventRequest {
  string event_id = 1;
  string action = 2; // INSPECTED / IGNORED / RETRY_LATER / ESCALATED
  string note = 3;
  uint64 expected_revision = 4;
}

message AcknowledgeUnknownEventResponse {
  string event_id = 1;
  string processing_status = 2;
  uint64 revision = 3;
  Error error = 4;
}

message RecoverTaskRequest {
  string task_id = 1;
  string action = 2; // RESUME / RETRY / CANCEL / MARK_FAILED
  uint64 expected_revision = 3;
}

message RecoverTaskResponse {
  string task_id = 1;
  string state = 2;
  bool accepted = 3;
  uint64 revision = 4;
  Error error = 5;
}
```

### 21.2 RPC 错误语义

业务错误写入响应内的 `Error`，保持现有 `proto/v1/common.proto:13-20` 的结构；认证、TLS、协议版本、传输失败使用 gRPC status。

稳定业务错误码：

```text
INVALID_ARGUMENT
NOT_FOUND
PERMISSION_DENIED
POLICY_CONFLICT
RESOURCE_LIMIT
PERSISTENCE_DEGRADED
RECOVERY_REQUIRED
CORRUPTED_STATE
UNSUPPORTED_CAPABILITY
STALE_CHECKPOINT
EVENT_NOT_RETAINED
TASK_NOT_RECOVERABLE
```

错误处理规则：

| 错误 | `retryable` | 客户端行为 |
|---|---:|---|
| `POLICY_CONFLICT` | true | 重新拉取 policy，不覆盖用户较新的设置 |
| `RESOURCE_LIMIT` | true | 显示占用者和释放方式，不自动重试 |
| `PERSISTENCE_DEGRADED` | true | 显示恢复能力已下降，禁止假报“可恢复” |
| `RECOVERY_REQUIRED` | true | 打开任务恢复页，由用户选择动作 |
| `CORRUPTED_STATE` | false | 进入数据恢复流程，不创建空状态覆盖原数据 |
| `UNSUPPORTED_CAPABILITY` | false | 隐藏或禁用对应入口，显示 agent 原始能力边界 |
| `EVENT_NOT_RETAINED` | true | 退回全量 history/snapshot |
| `PERMISSION_DENIED` | false | 不重试，不改变审批决定 |

### 21.3 兼容规则

新增 `RuntimeService` 是只增契约：

```text
旧 APK → 新 daemon：
  继续使用已有 Host/Thread/Model RPC；
  不调用 RuntimeService；
  旧功能保持可用。

新 APK → 旧 daemon：
  先检查 HostInfo.capabilities 是否包含 runtime.v1；
  不包含时隐藏 Runtime UI，不调用新 RPC；
  不因缺少新服务而断开旧连接。

新 APK → 新 daemon：
  使用 RuntimeService 全部能力。
```

`GetHostInfo` 当前字段和能力入口见 `proto/v1/host.proto:24-42`；新增 `runtime.v1` 必须走 capability，不得以 daemon 版本号硬编码判断。

---

## 22. 持久化 schema、目录、迁移和回滚

### 22.1 新数据根目录

Windows 发布版默认：

```text
%LOCALAPPDATA%\PocketAgent\data\
```

现有开发/旧版默认目录来自 `daemon/cmd/pocketagentd/main.go:37-47` 的 `defaultDataDir()` 和 `-data` 参数，实际迁移源目录以启动时解析到的旧 `dataDir` 为准，不能假设只有一个固定用户路径。

### 22.2 文件清单

| 文件 | schema | 类型 | 轮转/保留 |
|---|---:|---|---|
| `installation.json` | 1 | 权威身份 | 不轮转；保留 `.bak` |
| `devices.json` | 1 | 权威设备 | 不轮转；保留 `.bak` |
| `runtime-policy.json` | 1 | 权威策略 | 不轮转；保留 `.bak` |
| `threads.json` | 1 | 权威会话快照 | 保留 `.bak` 和 corrupt quarantine |
| `tasks.json` | 1 | 权威任务快照 | 保留 `.bak` |
| `journal/runtime.jsonl` | 1 | 状态变更 journal | 16 MiB/段，最多 4 段 |
| `events/events.jsonl` | legacy + envelope v1 | 事件重放 | 保持当前 10 MiB + `events-1.jsonl` 一代兼容 |
| `checkpoints/*.json` | 1 | thread/agent checkpoint | 每 thread 保留当前和上一版 |
| `memory/<agent>/<thread>/` | 1 | 记忆正文和摘要 | 按 thread/agent 预算淘汰 |
| `cache/<agent>/` | 1 | 可再生缓存 | TTL + LRU，不进入用户数据备份 |

当前 eventstore 的 10 MiB、一代轮转、`minSeq` 缺口语义见 `daemon/internal/eventstore/eventstore.go:37-73`、`:174-223`、`:286-304`，这是断线重放兼容基线，不因新增 journal 而改变。

### 22.2.1 旧文件到新目录的明确映射

迁移不是“把整个旧目录复制过去”，而是按文件逐项转换。默认旧目录由
`daemon/cmd/pocketagentd/main.go:37-47` 的 `defaultDataDir()` 和 `-data` 参数决定；以下
`<legacyDataDir>` 是启动时实际解析出的旧目录：

| 旧路径 | 新路径 | 读取方式 | 成功条件 |
|---|---|---|---|
| `<legacyDataDir>\threads.json` | `%LOCALAPPDATA%\PocketAgent\data\threads.json` | 解析 legacy v0 对象后包入 envelope v1 | thread 数量、ID、更新时间和 hash 核验一致 |
| `<legacyDataDir>\events.jsonl` | `%LOCALAPPDATA%\PocketAgent\data\events\events.jsonl` | 原始 JSONL 逐行校验后保留 legacy event 字段；新追加事件使用 envelope v1 | `maxSeq`、`minSeq` 和每行 checksum 可重建 |
| `<legacyDataDir>\events-1.jsonl` | `%LOCALAPPDATA%\PocketAgent\data\events\events-1.jsonl` | 原样保留一代轮转文件 | 轮转边界和 `minSeq` 不被改写 |
| `<legacyDataDir>\devices.json` | `%LOCALAPPDATA%\PocketAgent\data\devices.json` | legacy v0 设备记录映射为 schema v1，缺失 `key_generation` 取 1 | device ID、撤销状态和 token hash 不变 |
| `<legacyDataDir>\pair-code.txt` | `%LOCALAPPDATA%\PocketAgent\data\pairing\pair-code` | 只迁移当前有效 pairing secret，不把明文复制到日志 | 新 daemon 能完成一次已有设备续期 |

迁移成功前不删除旧文件；新目录写入失败、字段校验失败或 checksum 不一致时，daemon
继续使用旧目录并报告 `PERSISTENCE_DEGRADED`，不得创建一个看似空白的新 store 覆盖旧数据。

### 22.3 公共 envelope

新格式的顶层 envelope：

```json
{
  "schema_version": 1,
  "store": "threads",
  "updated_at_unix_ms": 0,
  "revision": 0,
  "data": {}
}
```

迁移期间读取规则：

```text
读到 envelope schema_version=1 → 正常读取
读到现有 legacy 直接对象 → 按 schema_version=0 读取并迁移
schema_version > 当前支持版本 → 拒绝写回，进入兼容错误
```

### 22.4 各文件字段

#### `installation.json`

```json
{
  "schema_version": 1,
  "installation_id": "inst-...",
  "host_id": "host-...",
  "key_ref": "keystore-or-file-reference",
  "created_at_unix_ms": 0,
  "last_migrated_at_unix_ms": 0,
  "generation": 1
}
```

`key_ref` 不能把私钥明文写入普通 JSON。Windows 版本优先使用 DPAPI/用户凭据保护；文件仅保存引用、算法和 key id。

#### `devices.json`

```json
{
  "schema_version": 1,
  "revision": 0,
  "devices": [
    {
      "device_id": "device-...",
      "name": "Pixel",
      "pub_key": "base64-spki",
      "token_hash": "sha256-hex",
      "scopes": ["thread.read", "thread.write", "approval.submit"],
      "created_at_unix_ms": 0,
      "last_seen_unix_ms": 0,
      "revoked_at_unix_ms": 0,
      "key_generation": 1
    }
  ]
}
```

#### `runtime-policy.json`

```json
{
  "schema_version": 1,
  "revision": 0,
  "collaboration_enabled": true,
  "native_delegate_enabled": true,
  "fanout_enabled": true,
  "auto_summarize": false,
  "max_parallel_members": 3,
  "max_session_bytes": 134217728,
  "retain_artifacts": true,
  "default_agent_memory_bytes": 33554432,
  "default_agent_cache_bytes": 16777216,
  "updated_at_unix_ms": 0
}
```

#### `tasks.json`

```json
{
  "schema_version": 1,
  "revision": 0,
  "tasks": {
    "task-...": {
      "task_id": "task-...",
      "thread_id": "thread-...",
      "agent_id": "pi",
      "turn_id": "turn-...",
      "state": "WAITING_APPROVAL",
      "operation_revision": 7,
      "checkpoint_id": "cp-...",
      "created_at_unix_ms": 0,
      "updated_at_unix_ms": 0,
      "recovery_reason": ""
    }
  }
}
```

#### `journal/runtime.jsonl`

一行一条状态变化：

```json
{
  "schema_version": 1,
  "journal_seq": 42,
  "timestamp_unix_ms": 0,
  "aggregate_type": "task",
  "aggregate_id": "task-...",
  "event_type": "task.transition",
  "from_state": "RUNNING",
  "to_state": "WAITING_APPROVAL",
  "correlation_id": "corr-...",
  "payload_json": "{\"request_id\":\"req-...\"}",
  "checksum_sha256": "..."
}
```

Journal 规则：

```text
每 16 MiB 轮转为 runtime-1.jsonl
最多保留 runtime.jsonl + runtime-1.jsonl + runtime-2.jsonl + runtime-3.jsonl
超过 4 段前必须生成 snapshot
snapshot 成功后才允许删除已覆盖的旧段
每次状态变更先 journal，再更新内存，再异步刷新 snapshot
```

### 22.5 迁移流程

启动时按以下顺序处理：

```text
1. 解析新 data root。
2. 如果新 root 没有 installation.json，扫描 legacy dataDir。
3. 生成 migration manifest，记录源路径、目标路径、文件 hash、schema。
4. 复制原始文件到 migration/source-backup，不直接删除源文件。
5. 将 legacy 文件读入 v0 adapter。
6. 写入 v1 envelope 和新目录。
7. 重新读取新文件并做 hash/字段核验。
8. 启动 runtime kernel。
9. 连续两次健康启动成功后，才允许用户选择清理 legacy 源目录。
```

迁移 manifest：

```json
{
  "migration_id": "mig-...",
  "from_root": "C:\\Users\\user\\.pocketagentd",
  "to_root": "C:\\Users\\user\\AppData\\Local\\PocketAgent\\data",
  "from_schema": 0,
  "to_schema": 1,
  "files": [
    {
      "source": "threads.json",
      "target": "threads.json",
      "source_sha256": "...",
      "target_sha256": "...",
      "status": "COPIED_AND_VERIFIED"
    }
  ],
  "status": "COMPLETED",
  "created_at_unix_ms": 0
}
```

### 22.6 回滚

回滚分为三种，不得混用：

| 操作 | 行为 |
|---|---|
| 升级失败回滚 | 保留新目录和 manifest，恢复上一版二进制；不删除新数据 |
| 重置配对 | 只撤销设备和重新生成配对凭据，不删除 threads/events |
| 卸载 | 默认删除程序和任务；用户数据保留，除非用户明确选择“删除全部数据” |

旧 daemon 读取旧目录时不保证识别 v1 新数据。因此正式支持的降级路径是：

```text
升级前保留 legacy backup
→ 回滚二进制
→ 从 legacy backup 恢复旧格式
```

不能直接让旧 daemon 写新 v1 文件。

---

## 23. 安全、授权和并发不变式

### 23.1 既有安全约束必须保留

现有安全事实：

- 默认配对码鉴权：`daemon/cmd/pocketagentd/main.go:46-53`, `:124-135`。
- `-insecure-no-auth` 只允许本机回环：`daemon/cmd/pocketagentd/main.go:114-121`，server 侧拦截见 `daemon/internal/server/server.go:209-242`。
- TLS 默认开启、明文仅应急：`daemon/cmd/pocketagentd/main.go:199-207`。
- 每设备独立 token、挑战应答和撤销：`daemon/internal/server/devices.go:158-183`, `:197-283`。
- Android token 敏感存储和证书 TOFU：`android/app/src/main/java/dev/pocketagent/app/AgentConnection.kt:198-225`, `:322-442`。
- Android 端证书指纹变化必须阻断，不能自动接受新指纹：`android/app/src/main/java/dev/pocketagent/app/AgentConnection.kt:135-155`。

RuntimeService 必须经过现有 unary/stream auth 拦截器，不能创建旁路服务。

### 23.2 scopes 枚举

`devices.json` 中的 `scopes` 使用稳定字符串，首批定义：

```text
host.read
runtime.read
runtime.policy.write
thread.read
thread.write
thread.cancel
approval.submit
fs.read
fs.write
terminal.open
terminal.write
collab.delegate
collab.fanout
diagnostics.read
```

默认手机配对 scopes：

```text
host.read
runtime.read
thread.read
thread.write
thread.cancel
approval.submit
fs.read
terminal.open
terminal.write
collab.delegate
collab.fanout
diagnostics.read
```

`runtime.policy.write`、`fs.write` 等高风险 scope 只能由用户在 daemon 本机或已授权设备上明确授予。

未知 scope：

```text
保存原文
不授予权限
不阻止读取其他已知权限
在诊断中显示未识别 scope
```

### 23.3 key rotation

`installation_key` 和设备密钥分开：

```text
installation_key：
  首次创建 data root 时由 CSPRNG 生成 32 字节；
  归属用户电脑，版本升级不轮换；
  私钥不写入普通 JSON，Windows 由 DPAPI/用户凭据保护；
  用途是保护 pairing secret、签署 migration manifest/checkpoint 的完整性，
  不是手机业务 RPC 的 bearer token；
  用户执行“重置安全状态”或检测到泄露时轮换；
  轮换先写新 key reference 和 generation，再使旧 pairing secret 失效；
  失败时保留旧 reference，禁止生成“无 key 但服务已启动”的半状态。

device_key：
  归属手机；
  Android Keystore 私钥不可导出；
  重新安装或用户主动轮换时生成新公钥；
  旧 device_id 进入 revoked。
```

轮换流程必须支持旧 token 在短窗口内只用于获取 challenge，不得继续调用业务 RPC。

### 23.4 撤销传播

设备撤销是 daemon 本地权威动作：

```text
撤销写入 devices.json 和 journal
→ 立即从认证缓存移除
→ 新 unary/stream RPC 拒绝
→ 已建立 stream 在下一次鉴权/心跳边界被关闭
→ 已在运行的 agent turn 不自动批准、不自动取消
→ 用户可通过 thread cancel 明确取消任务
```

目标传播延迟：

```text
新请求：< 100 ms（同一 daemon 进程）
已建立连接：≤ 35 s（当前 keepalive 25 s + timeout 10 s 基线）
```

### 23.5 连接重试策略

连接候选沿用现有注册表和顺序，现有 Android 端点注册见 `android/app/src/main/java/dev/pocketagent/app/AgentConnection.kt:72-107`。

每个 endpoint 使用指数退避：

```text
第 1 次：0.5 s
第 2 次：1 s
第 3 次：2 s
第 4 次：4 s
第 5 次及以后：8 s，上限 60 s
```

规则：

```text
TLS 指纹不匹配：立即停止该 endpoint，不重试
鉴权失败：先 challenge renewal 一次，失败后停止业务重试
连接拒绝/超时：按退避重试并尝试下一个 endpoint
网络恢复事件：立即取消当前退避，重新竞速候选
```

### 23.6 并发不变式

必须保留并明确以下现有不变式：

1. `activeTurns` 是每个 thread 的运行租约；同一 thread 同时只能有一个 active turn。当前容器位置：`daemon/internal/server/server.go:73-75`。
2. `turnQueue` 只负责不支持 native steer 的 provider 的排队；Codex 的 native steer 仍走 `Steerer`，不能把两种语义混在一起。接口位置：`daemon/internal/agent/provider.go:71-81`。
3. `turnTransport` 的 `pending` 只能由单一 gRPC writer 消费，事件必须先记录再发送。当前实现：`daemon/internal/server/turn_transport.go:12-26`, `:41-71`。
4. Runtime Kernel 不得让多个 goroutine 直接写 `threads.json`、`events.jsonl` 或同一个 checkpoint。
5. journal writer 是单写者；读者通过 snapshot + sequence 读取。
6. `ResourceLease` 的 `Reserve/Release` 必须成对出现；任务取消、panic、进程退出都必须进入最终释放路径。
7. 协同成员的资源 lease 独立；一个成员失败不能释放或覆盖其他成员的 lease。
8. 所有状态转换使用 `expected_revision` 或单写者队列，禁止旧请求覆盖新策略。

---

## 24. 量化验收目标与校准方法

### 24.1 初始验收目标

这些是第一轮实现的目标阈值；若真实测量证明不适合某个平台，必须在 `AUDIT.md` 或项目证据中记录修改原因，不能只在口头上调整。

| 指标 | 目标 |
|---|---:|
| daemon 空闲时 PocketAgent 自有 heap/RSS | ≤ 128 MiB；不含原生 agent 子进程 RSS |
| 3 个并行 agent、每个 1 个 active turn 时 daemon 自有 RSS | ≤ 256 MiB |
| 单 turn 内存事件缓冲硬上限 | 8 MiB |
| 单 agent cache 硬上限 | 16 MiB，用户可配置 |
| 单 thread working memory 硬上限 | 8 MiB |
| 单 thread conversation memory 默认磁盘上限 | 32 MiB |
| 单 collab session 默认资源上限 | 128 MiB |
| 单条 event payload 默认上限 | 4 MiB，与现有 eventstore scanner 上限对齐 |
| `GetRuntimeState` 本机 p95 | ≤ 250 ms |
| `GetAgentRuntime` 有缓存时本机 p95 | ≤ 500 ms |
| `SetRuntimePolicy` 提交到 journal p95 | ≤ 1 s |
| daemon clean restart 到 `READY` | ≤ 10 s |
| daemon restart 后 native session 恢复 | ≤ 30 s；不支持恢复则 ≤ 5 s 返回明确状态 |
| 网络恢复到重新建立鉴权连接 | ≤ 60 s |
| 断线事件补齐 | 网络恢复后 ≤ 5 s 开始，保留窗口内 100% 补齐 |
| 断线期间允许的事件丢失 | 0；若持久化失败必须显式进入 `PERSISTENCE_DEGRADED` |
| 已提交 journal 在进程崩溃后的恢复 | RPO ≤ 1 条 journal record |
| UI 重连重复展示 | 0 条重复 sequence |
| 未知事件原文保留率 | 100%（不超过配置的单事件硬上限） |
| 设备撤销对新 RPC 的生效 | < 100 ms |
| 设备撤销对已建立连接的生效 | ≤ 35 s |

### 24.2 校准方法

新增测试/探针：

```text
daemon/internal/runtime/*_benchmark_test.go
daemon/internal/runtime/*_load_test.go
daemon/cmd/runtimeprobe/
tools/audit/runtime_budget_probe.py
```

每轮记录：

```text
git commit
daemon version
OS / CPU / RAM
agent 版本
并发 thread 数
并发 agent 数
事件数和 payload 大小
峰值 RSS
PocketAgent 内部 heap
cache bytes
journal bytes
恢复耗时
事件缺口
重复 sequence 数
```

三组固定场景：

```text
S1：1 个 agent、1 个 thread、1000 条事件、无协同
S2：3 个 agent、3 个 thread、每个 1 个 turn、包含大工具输出
S3：3 个 agent 并行协同，执行中断网、杀 App、重启 daemon，再恢复
```

阈值判断：

```text
任一硬上限被绕过 = FAIL
超过目标 10% 以内 = WARN，必须登记原因
超过目标 10% 以上 = FAIL
事件 sequence 缺口或重复 = FAIL
恢复时间超过阈值 = FAIL
```

---

## 25. 未决问题与默认决策

以下问题曾经没有集中定义。为避免实现阶段各自解释，先给出默认决策；如果要改变，必须在实现计划批准前修改本文。

| 问题 | 默认决策 | 影响 |
|---|---|---|
| 卸载是否删除用户数据 | 默认保留 `data/`、用户项目和凭据；卸载器提供“删除全部数据”二次确认 | 防止卸载误删会话和配对 |
| 重置配对是否删除会话 | 不删除 threads、events、memory、cache；只撤销设备、轮换 pairing secret | 安全重置不等于数据清空 |
| 升级是否改变 installation_id | 不改变 | 保证用户手机和电脑身份连续 |
| `installation_key` 放在哪里 | Windows 用户保护存储；JSON 只存 key reference | 防止普通文件泄露私钥 |
| 设备 scopes 是否由手机自行申请 | 不能；手机只能请求，daemon 按用户策略批准 | 防止客户端自升权 |
| 是否支持自动多 agent 编排 | 默认关闭；第一版只实现用户显式 delegate/fanout | 避免 daemon 擅自猜测分工 |
| 协同开关事实来源 | daemon `runtime-policy.json` | 多手机连接同一电脑时保持一致 |
| eventstore 是否改成多代长日志 | 第一阶段不改；保持当前 10 MiB + 1 代 `events-1.jsonl`，新增 journal 独立轮转 | 降低断线恢复回归风险 |
| journal 是否替代 threads.json | 不立即替代；先 envelope + journal + snapshot 双轨，稳定后再评估 | 保持旧数据兼容 |
| agent 没有原生 session 怎么办 | 使用 daemon `ConversationMemory`，`SessionSource=DAEMON` | OpenAI Compatible/Echo 可完整工作 |
| agent 原生 session 与 daemon memory 冲突怎么办 | 原生正文优先，daemon 保存索引/摘要/状态；冲突进入 `RECOVERY_REQUIRED`，不静默覆盖 | 防止双写污染 |
| 撤销设备时是否终止在飞 turn | 不自动终止；拒绝后续 RPC，任务由用户另行取消 | 避免认证撤销被误当任务取消 |
| 是否嵌入 Tailscale | 不嵌入；继续使用用户安装的 Tailscale | 对齐 `README.md:215` 当前边界 |
| 是否在本阶段做 P2 内嵌 Tailscale、P3 GeckoView、自更新 | 不做，列为非目标 | 避免扩大当前迁移范围 |

---

## 26. 非目标清单

本阶段明确不实现：

1. PocketAgent 云端中继、跨用户控制面或集中式任务存储。
2. PocketAgent 内嵌 Tailscale、替用户登录 Tailscale 或管理 tailnet。
3. 多台用户电脑之间的自动容灾和任务迁移。
4. GeckoView 替换现有 Android WebView。
5. 自动更新器和静默升级。
6. 自动替用户决定 agent 分工、审批或工具授权。
7. 未知 agent 请求的自动批准。
8. 使用向量数据库实现没有明确需求的长期记忆。
9. 修改用户桌面 agent 的原生配置文件。
10. 把原生 agent 的所有 RSS 强行限制在 PocketAgent 进程预算内。

项目现有未完成项的对齐依据：

```text
README.md:215  Tailscale 为 PC 侧外部方案，内嵌 libipn 未做
README.md:472  P2 内嵌 Tailscale、P3 GeckoView / 自更新 / 实况通知未做
```

本文只把它们明确列为非目标，不把它们误写成已完成能力。

---

## 27. 依赖顺序、责任边界和估工

### 27.1 依赖顺序

```text
W0 规格、proto 和数据 schema 冻结
 ↓
W1 Runtime Kernel 骨架和 OperationContext
 ↓
W2 identity / pairing / endpoint / journal / migration
 ↓
W3 ResourceManager 和有界缓冲
 ↓
W4 AgentRuntime contract，先 Echo，再 API agent，再原生 agent
 ↓
W5 EventEnvelope / EventRouter / unknown diagnostics
 ↓
W6 capability registry
 ↓
W7 collaboration policy、checkpoint 和恢复
 ↓
W8 Android UI 和发布包
 ↓
W9 按 AUDIT.md 执行 L0/L1/L2，并做发布包补充检查
```

不能先做 W8 UI 再回头定义 W2/W3，否则 UI 会继续形成第二套状态事实。

### 27.2 责任边界

当前按仓库模块划分，不指定个人姓名：

| 工作包 | 责任目录 | 输出 |
|---|---|---|
| W0 契约 | `proto/v1/`, `codex-schema/`, `docs/superpowers/specs/` | proto、schema、兼容矩阵 |
| W1/W2 runtime | `daemon/internal/runtime/`, `server/`, `store/`, `eventstore/` | Kernel、journal、迁移、恢复 |
| W3 资源 | `daemon/internal/runtime/resource*`, `server/turn_transport.go`, `term/` | 预算、租约、有界队列 |
| W4 agent | `daemon/internal/agent/`, `codexapp/`, `piapp/`, `zcodeapp/`, `dshapp/` | 六类 provider contract |
| W5/W6 协议 | `proto/v1/runtime.proto`, `agent/`, `server/` | RuntimeService、事件路由、能力注册 |
| W7 协同 | `server/collab*`, `collabmcp/`, Android collab UI | 策略权威、恢复、限流 |
| W8 客户端/发布 | `android/...`, `dist/windows/`, `tools/device/` | UI、安装包、升级迁移 |
| W9 审核 | `AUDIT.md`, `tools/audit/`, `docs/verify/` | 证据、准出报告、真机结论 |

### 27.3 估工单位

估工以“工程日”表示，仅用于排序，不作为承诺工期：

| 工作包 | 估工 | 前置 |
|---|---:|---|
| W0 规格、契约、schema | 2-3 | 无 |
| W1 Kernel 骨架 | 2-4 | W0 |
| W2 迁移、journal、恢复 | 5-8 | W1 |
| W3 资源管理和有界流 | 4-7 | W1 |
| W4 六类 agent contract | 7-12 | W1、W3 |
| W5 未知事件和诊断 | 4-7 | W0、W4 |
| W6 capability registry | 3-5 | W4 |
| W7 协同策略和恢复 | 5-8 | W2、W3、W6 |
| W8 Android UI/发布包 | 6-10 | W2、W5、W7 |
| W9 全量审核和真机 | 4-7 | W8 |

W2、W3、W4 不能在同一个共享工作树里无约束并行修改相同的 store、proto 和 server 文件。可以在 W0 完成后分支并行，但合并必须经过一次完整的甲类审核。

---

## 28. 修订后的完成判定

在原 §19 清单之外，规格完成还必须满足：

```text
[ ] 每个当前实现判断都有 file:line 锚点
[ ] RuntimeService 有完整 proto message、字段、版本和错误语义
[ ] legacy dataDir → %LOCALAPPDATA%\PocketAgent\data 有迁移方案
[ ] 每个持久化文件有 schema_version、备份、轮转或保留策略
[ ] 有旧 APK ↔ 新 daemon、新 APK ↔ 旧 daemon 兼容矩阵
[ ] 有 installation_key 的生成、保存、轮换和回滚行为
[ ] 有 scopes 枚举和未知 scope 处理
[ ] 有设备撤销传播延迟和在飞任务处置
[ ] 有 endpoint 指纹、重试、退避和网络恢复规则
[ ] 有 Kernel 与 activeTurns/turnQueue/turnTransport 的并发不变式
[ ] 有可复现的内存、缓存、RTO、RPO 和事件恢复阈值
[ ] 有非目标清单，并与 README §15 对齐
[ ] 有集中未决问题及默认决策
[ ] 审核章节明确引用 AUDIT.md，不自造同名流程
[ ] 命令中的 G:\项目1 明确标注为开发机审核工作区，不进入发布包
```

本次评审涉及的是规格完整性，尚未代表任何上述实现已经完成。
