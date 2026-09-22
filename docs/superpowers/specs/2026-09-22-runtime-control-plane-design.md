# PocketAgent 本机运行时内核与统一 Agent Runtime 设计

**日期**：2026-09-22
**状态**：架构设计规格，尚未实现
**适用范围**：Windows daemon、Android App、原生 agent 适配层、分发包、审核体系
**核心目标**：把当前已经存在的多个功能，收敛为一套由统一运行时约束的系统。

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

### P1：统一身份、状态和恢复

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

### P1：统一资源管理

目标：

```text
删除生产路径的 UNLIMITED buffer
所有 agent 有独立预算
事件和工具输出可 spill 到磁盘
缓存、artifact、终端和协同任务统一计费
```

### P2：统一事件和未知事件透析

目标：

```text
EventEnvelope
EventRouter
UnknownEvent Inspector
unknown request 安全处理
原始 payload 可检索
```

### P2：统一能力注册

目标：

```text
AgentDescriptor
CapabilityRegistry
能力驱动的 server 和 UI
删除新增的 agent 名称特判
```

### P2：统一协同

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

## 14. 代码落点

### 新增目录

```text
daemon/internal/runtime/
daemon/internal/runtime/cache/
daemon/internal/runtime/memory/
daemon/internal/runtime/recovery/
```

### 主要迁移文件

```text
daemon/internal/server/server.go
daemon/internal/server/thread.go
daemon/internal/server/turn_queue.go
daemon/internal/server/turn_transport.go
daemon/internal/server/collab_session.go
daemon/internal/server/collab_delegate.go
daemon/internal/server/devices.go
daemon/internal/store/store.go
daemon/internal/eventstore/eventstore.go
daemon/internal/agent/provider.go
daemon/internal/agent/codex.go
daemon/internal/agent/pi.go
daemon/internal/agent/zcode.go
daemon/internal/agent/dsh.go
daemon/internal/agent/openai_compat.go
daemon/internal/agent/echo.go
```

### Android 主要迁移文件

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

### 协议

如果新增运行时诊断和策略 RPC，应新增或扩展：

```text
proto/v1/runtime.proto
android/app/src/main/proto/v1/runtime.proto
```

两份 proto 必须保持完全一致，生成代码不能手改。

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

## 15. 审核方案

### L0：架构和契约闸门

```powershell
Set-Location G:\项目1\pocketagent

git diff --check
python .\tools\audit\pa_audit.py `
  --repo . `
  --only roster,unknown_ui,go_baseline,proto_sync `
  --json
```

额外检查：

```powershell
Get-ChildItem -Recurse -File |
  Select-String -Pattern 'G:\\项目1|yanzi|100\.92\.193\.109'
```

必须阻塞的问题：

```text
proto 两副本不一致
新的硬编码 agent 名单
新的无界 buffer
未知事件被静默丢弃
未知请求被默认批准
发布包包含开发机路径或凭据
某个 agent 没有公共 runtime 接口
```

### L1：自动化测试

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

### L2：真机网络和恢复

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

### L3：发布包审核

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
[ ] L0、L1、L2、L3 审核通过
```

在实现计划获批前，本文件只是目标架构规格；当前仓库中除本文件外，不应把上述目标当作已经完成的功能。
