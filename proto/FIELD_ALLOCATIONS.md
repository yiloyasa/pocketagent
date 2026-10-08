# proto 字段号分配登记（FIELD_ALLOCATIONS）

## 规则（2026-10-04 起生效）

**后续任何 proto 变更（新增消息 / 追加字段 / 新增 RPC），必须先在本文件登记号段，由 protocol steward 统一分配后再动 `.proto` 文件。**

- 只增不改：已发布的字段号与类型永不复用、永不改型、永不挪动；废弃字段保留号位并注明废弃。
- 新消息字段号从 1 起；扩展既有消息时从该消息现有最大号 +1 续接，禁止插号。
- 每条登记包含：消息名 / 字段名 / 字段号 / 类型 / 分配日期 / 来源计划。
- 双副本同步与 Go 生成统一走 `tools/proto/gen_proto.py`（两副本 MD5 校验）。

## 登记表

### thread.proto —— 协同会话闭环（2026-10-04）

来源：`docs/superpowers/plans/2026-10-04-collab-session-closure.md` Task 1。
全部为新增消息，字段号从 1 起；不触碰任何既有消息。

| 消息 | 字段 | 号 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| CollabSessionSnapshot | session_id | 1 | string | 协同会话 id |
| CollabSessionSnapshot | instruction | 2 | string | 用户指令原文 |
| CollabSessionSnapshot | status | 3 | string | 会话状态（string 口径） |
| CollabSessionSnapshot | workspace | 4 | string | 协同工作区 |
| CollabSessionSnapshot | created_at_unix_ms | 5 | int64 | 创建时刻 |
| CollabSessionSnapshot | updated_at_unix_ms | 6 | int64 | 最近更新时刻 |
| CollabSessionSnapshot | members | 7 | repeated CollabMemberSnapshot | 成员卡片 |
| CollabSessionSnapshot | error | 8 | string | 会话级失败原因 |
| CollabMemberSnapshot | agent_id | 1 | string | 目标 agent |
| CollabMemberSnapshot | role_id | 2 | string | 协同角色 id |
| CollabMemberSnapshot | thread_id | 3 | string | 成员子线程 id |
| CollabMemberSnapshot | status | 4 | string | 成员状态（string 口径） |
| CollabMemberSnapshot | model_id | 5 | string | 实际模型 |
| CollabMemberSnapshot | effort | 6 | string | 思考强度 |
| CollabMemberSnapshot | progress | 7 | string | 有界进度摘要 |
| CollabMemberSnapshot | summary | 8 | string | 终稿摘要 |
| CollabMemberSnapshot | error | 9 | string | 失败/取消原因 |
| CollabMemberSnapshot | pending_approvals | 10 | repeated ApprovalRequest（既有消息） | 挂起审批 |
| CollabMemberSnapshot | workspace | 11 | string | 成员实际工作区 |
| WatchCollabSessionRequest | session_id | 1 | string | 会话 id |
| WatchCollabSessionRequest | after_sequence | 2 | int64 | 续传水位 |
| CollabSessionEvent | sequence | 1 | int64 | 全局单调序号 |
| CollabSessionEvent | session_id | 2 | string | 会话 id |
| CollabSessionEvent | member_thread_id | 3 | string | 涉及成员子线程 |
| CollabSessionEvent | kind | 4 | string | 事件种类（string 口径） |
| CollabSessionEvent | snapshot | 5 | CollabSessionSnapshot | 整屏快照兜底 |
| CollabSessionEvent | member | 6 | CollabMemberSnapshot | 单成员整卡 |
| GetCollabSessionRequest | session_id | 1 | string | 会话 id |
| ListCollabSessionsRequest | include_finished | 1 | bool | 是否含已结束 |
| ListCollabSessionsResponse | sessions | 1 | repeated CollabSessionSnapshot | 会话列表 |
| CancelCollabSessionRequest | session_id | 1 | string | 会话 id |
| CancelCollabMemberRequest | session_id | 1 | string | 会话 id |
| CancelCollabMemberRequest | thread_id | 2 | string | 成员子线程 id |
| RetryCollabMemberRequest | session_id | 1 | string | 会话 id |
| RetryCollabMemberRequest | thread_id | 2 | string | 成员子线程 id |

新增 RPC（ThreadService）：

| RPC | 形态 | 请求 → 响应 |
| --- | --- | --- |
| GetCollabSession | unary | GetCollabSessionRequest → CollabSessionSnapshot |
| ListCollabSessions | unary | ListCollabSessionsRequest → ListCollabSessionsResponse |
| WatchCollabSession | server-streaming | WatchCollabSessionRequest → stream CollabSessionEvent |
| CancelCollabSession | unary | CancelCollabSessionRequest → CollabSessionSnapshot |
| CancelCollabMember | unary | CancelCollabMemberRequest → CollabMemberSnapshot |
| RetryCollabMember | unary | RetryCollabMemberRequest → CollabMemberSnapshot |

### runtime.proto —— 未知事件用户处理中心（2026-10-04）

来源：`docs/superpowers/plans/2026-10-04-unknown-event-action-center.md` Task 1。

新增消息：

| 消息 | 字段 | 号 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| UserActionDescriptor | action_id | 1 | string | 动作稳定键 |
| UserActionDescriptor | label | 2 | string | 按钮文案 |
| UserActionDescriptor | description | 3 | string | 一句话说明 |
| UserActionDescriptor | safe | 4 | bool | 是否安全可渲染 |
| UserActionDescriptor | requires_confirmation | 5 | bool | 是否需二次确认 |
| UserActionDescriptor | destructive | 6 | bool | 是否破坏性 |

追加字段（既有消息，从现有最大号续接，**1–11 / 1–6 一字未动**）：

| 消息 | 字段 | 号 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| UnknownEventSummary | plain_title | 12 | string | 一行人话标题 |
| UnknownEventSummary | plain_explanation | 13 | string | 人话解释 |
| UnknownEventSummary | impact | 14 | string | 对任务的影响 |
| UnknownEventSummary | requires_user | 15 | bool | 是否需要用户处理 |
| UnknownEventSummary | can_open_native | 16 | bool | 电脑端是否有原生界面 |
| UnknownEventSummary | actions | 17 | repeated UserActionDescriptor | 安全可选动作 |
| UnknownEventResponse | recommended_next_step | 7 | string | 建议下一步 |
| UnknownEventResponse | support_bundle_hint | 8 | string | 支持包导出提示 |

### runtime.proto —— 未知事件用户动作执行（2026-10-04，P2-T6 接线）

来源：`docs/superpowers/plans/2026-10-04-unknown-event-action-center.md` Task 6（daemon 侧执行器已就绪，本次补 proto 接线）。

新增消息（字段号从 1 起，不碰任何既有消息）：

| 消息 | 字段 | 号 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| UnknownEventActionRequest | event_id | 1 | string | 目标事件 id |
| UnknownEventActionRequest | expected_revision | 2 | int64 | 乐观并发控制（0 = 不做 CAS） |
| UnknownEventActionRequest | action_id | 3 | string | 白名单动作 id |
| UnknownEventActionRequest | note | 4 | string | 可选备注 |
| UnknownEventActionResult | ok | 1 | bool | 执行成功（含幂等重复） |
| UnknownEventActionResult | conflict | 2 | bool | revision 冲突（结构化结果，非 error） |
| UnknownEventActionResult | detail | 3 | string | 人话说明 |
| UnknownEventActionResult | summary | 4 | UnknownEventSummary | 执行后/当前的台账真值 |
| UnknownEventActionResult | technical | 5 | UnknownEventResponse（复用既有详情消息） | 技术详情整包 |
| UnknownEventActionResult | new_revision | 6 | int64 | 执行后 revision |

新增 RPC（RuntimeService）：

| RPC | 形态 | 请求 → 响应 | 语义约定 |
| --- | --- | --- | --- |
| ExecuteUnknownEventAction | unary | UnknownEventActionRequest → UnknownEventActionResult | 幂等；冲突=结构化结果；白名单 daemon 侧硬编码：inspect/acknowledge/retry_later/open_native，绝不有 approve 类 |

### model.proto / thread.proto —— 有效模型选择与消息稳定身份（2026-10-04）

来源：`docs/superpowers/plans/2026-10-04-effective-selection-and-conversation-ux.md` Task 1 / 5 / 6。
全部为只增不改：既有消息从现有最大号 +1 续接，旧字段号与语义一字未动。

新增消息：

| 消息 | 字段 | 号 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| EffectiveModelSelection（model.proto） | agent_id | 1 | string | 归属 agent；空=默认 agent |
| EffectiveModelSelection | model_id | 2 | string | 生效模型；空=跟随默认（合法取值） |
| EffectiveModelSelection | effort | 3 | string | 生效思考强度；空=跟随默认（合法取值） |
| EffectiveModelSelection | source | 4 | string | 来源：thread_override/app_default/desktop_default/native_current |
| EffectiveModelSelection | applies_to_next_turn | 5 | bool | 将从下一轮生效 |
| EffectiveModelSelection | configuration_confirmed | 6 | bool | 用户已显式确认 |
| EffectiveModelSelection | explanation | 7 | string | 人话解释 |
| EffectiveModelSelection | available_efforts | 8 | repeated string | 当前模型可用强度档位 |
| MatchRange（thread.proto） | start | 1 | int32 | 命中起始码点（含，rune 口径） |
| MatchRange | end | 2 | int32 | 命中结束码点（不含） |

追加字段（既有消息，从现有最大号续接）：

| 消息 | 既有最大号 | 字段 | 号 | 类型 | 说明 |
| --- | --- | --- | --- | --- | --- |
| ListModelsResponse（model.proto） | 4 | effective_selection | 5 | EffectiveModelSelection | 解析后的选择真值；缺席=旧 daemon |
| Thread（thread.proto） | 23 | effective_selection | 24 | EffectiveModelSelection | 会话级选择真值；缺席=旧 daemon |
| ThreadMessage（thread.proto） | 3 | message_id | 4 | string | 稳定消息 id；新消息生成，旧数据启动迁移 |
| ThreadMessage | | turn_id | 5 | string | 所属轮次 id；空=未知 |
| ThreadMessageHit（thread.proto） | 4 | message_id | 6 | string | 命中定位主键（sequence 降级为兼容） |
| ThreadMessageHit | | turn_id | 7 | string | 命中所属轮次 |
| ThreadMessageHit | | ranges | 8 | repeated MatchRange | 消息内多处命中范围 |
| ThreadMessageHit | | from_loaded_history | 9 | bool | 命中是否来自已加载历史 |
| SearchThreadMessagesResponse（thread.proto） | 1 | has_more | 2 | bool | 还有未返回命中（分页位，FTS 迁移预留） |
| SearchThreadMessagesResponse | | searched_messages | 3 | int32 | 本次实扫消息条数 |
| SearchThreadMessagesResponse | | scope | 4 | string | 范围：full_thread/local_snapshot；空=旧 daemon |

**号位说明（ThreadMessageHit）**：spec 草案按"旧消息已占 1–5"分配 6–9，实测旧消息仅占
1–4；号位 6–9 与既有字段零冲突，按 spec 原样采用。**字段 5 冻结留空、永不启用**（防止
与任何按 spec 草稿实现的客户端撞号），已在 proto 注释与本登记表双处注明。

**跨文件引用**：`EffectiveModelSelection` 定义在 model.proto，thread.proto 新增
`import "v1/model.proto"`（无循环：model.proto 不引 thread.proto）。

### thread.proto —— TurnStarted 本轮生效配置来源（2026-10-04，P3-T2）

来源：模型选择四级来源收敛（daemon 侧快照真值已备好，startedFrame 一行接线）。
扩展既有消息，从该消息现有最大号 4 续接。

| 消息 | 既有最大号 | 字段 | 号 | 类型 | 说明 |
| --- | --- | --- | --- | --- | --- |
| TurnStarted（thread.proto） | 4 | source | 5 | string | 本轮实际生效配置的来源；词表与 EffectiveModelSelection.source 一致：thread_override / app_default / desktop_default / native_current；空串 = 旧 daemon 未带，客户端按"未知来源"如实处理 |

### host.proto —— 线程有效选择变化通知（2026-10-04，P3-T2）

来源：模型选择四级来源收敛（推送侧增量通知）。HostEvent 字段号现状：2-4 reserved
（已删旧事件，永不复用）、5 = turn_finished、6 = sequence；oneof 新分支从 7 续接。
host.proto 新增 `import "v1/model.proto"`（仅引用 EffectiveModelSelection，定义归
model.proto；同 package 非限定名引用，与 thread.proto 同款形态）。

| 消息 | 既有最大号 | 字段 | 号 | 类型 | 说明 |
| --- | --- | --- | --- | --- | --- |
| HostEvent（oneof event 新分支） | 6 | selection_changed | 7 | SelectionChangedNotification | 线程有效选择变化通知；daemon 重启后首轮不发，初始真值以 TurnStarted.source / 查询型 RPC 为准 |
| SelectionChangedNotification（host.proto，新增消息） | — | thread_id | 1 | string | 发生变化的线程 id（daemon 本地线程 id） |
| SelectionChangedNotification | | selection | 2 | EffectiveModelSelection（复用 model.proto 既有消息） | 变化后的有效选择整包（非 diff）；空 = 清回默认（合法状态） |

### thread.proto —— pi thinking 权威全文（2026-10-09，P1-3）

来源：交接档 `docs/verify/handoff-2026-10-08.md` P1-3（pi thinking 权威全文替换）。
扩展既有消息 ThinkingDelta，从现有最大号 1 续接。

| 消息 | 既有最大号 | 字段 | 号 | 类型 | 说明 |
| --- | --- | --- | --- | --- | --- |
| ThinkingDelta（thread.proto） | 1 | snapshot | 2 | string | 权威全文：pi thinking_end 终局事件/历史回放一次性携带；客户端收到即整体替换累积思考文本（丢帧/断连兜底）；旧客户端忽略未知字段，增量行为不变 |
