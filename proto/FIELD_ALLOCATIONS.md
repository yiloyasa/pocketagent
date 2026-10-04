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
