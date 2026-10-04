# PocketAgent 用户体验与可靠性升级规格

**日期：** 2026-10-04  
**状态：** 待用户审阅  
**适用版本：** 当前 `main` 基线 `fc45567`

## 1. 背景

PocketAgent 已具备 Android Compose 客户端、Go daemon、多个本地 Agent 适配器、gRPC 流式会话、断线事件回放、未知事件台账、模型能力探测、语音识别降级链和多 Agent 协同雏形。

当前主要风险不是缺少入口，而是跨域状态没有形成完整闭环：协同页无法可靠显示成员终局，未知事件存在但普通用户难以处理，模型和思考强度有多份真值，历史搜索按数组位置定位，提示词优化和语音逻辑集中在巨型会话页中。

## 2. 目标

升级后用户应能够：

1. 在会话页直接看懂 Agent 当前做什么、为什么停下、下一步能做什么。
2. 遇到未知事件时不被技术错误打断，并能在 App 内查看、延期、复制、转人工或执行明确安全动作。
3. 在多 Agent 协同中看到每个成员的真实进度、审批等待、结果、失败原因和可操作动作。
4. 清楚知道每一轮实际使用的 Agent、模型、思考强度和工作区。
5. 在长会话中稳定搜索、自动加载和定位历史消息。
6. 使用提示词优化时先预览、可撤销、不新增需求、不自动发送。
7. 使用语音输入时可开始、暂停、取消、编辑和切换识别引擎，不因页面重组或切换会话丢失文本。

## 3. 非目标

- 不拆成微服务。
- 不重写现有 Agent 原生循环。
- 不把所有业务改成完整事件溯源。
- 不默认替用户批准未知或危险操作。
- 不默认让多个 Agent 共享可写工作区。
- 不在这一轮引入不必要的动态 DLL/Go plugin 体系。

## 4. 统一领域模型

### 4.1 Thread / Turn

- `Thread`：可恢复的用户会话。
- `Turn`：Thread 中一次用户输入到 Agent 终局的执行单元。
- `TurnSnapshot`：Turn 启动时固定的 Agent、模型、effort、workspace、policy 快照。
- 运行中切换模型或 effort 默认只影响下一轮。

### 4.2 Unknown Event

未知事件不是一个笼统错误，而是三类：

- `UNKNOWN_NOTIFICATION`：电脑端传来无法投影但不阻塞任务的通知。
- `UNKNOWN_REQUEST`：电脑端请求客户端动作；只有安全且能准确解释的动作才显示可执行按钮。
- `UNKNOWN_RESPONSE`：收到无法匹配到在途请求的响应。

每条未知事件包含：技术原文、确定性人话解释、安全等级、当前影响、可执行动作、处理状态和审计信息。

### 4.3 Collab Session

协同会话是一条指令和多个成员执行单元的聚合对象，不等同于某个子线程：

- `CollabSession`：总指令、总体状态、成员、时间和策略。
- `CollabMemberRun`：成员 Agent、角色、线程、模型、effort、进度、审批、结果、错误和产物。
- `CollabEvent`：成员生命周期和结果变化。

### 4.4 Effective Selection

模型/effort 的有效配置必须带来源：

- `thread_override`
- `app_default`
- `desktop_default`
- `native_current`

客户端展示的值必须能够追溯到 daemon 的有效值，不允许只依赖 SharedPreferences。

## 5. 统一用户状态规范

所有长期操作都使用“状态 + 原因 + 动作”结构：

```text
状态：等待你确认
原因：电脑端想执行一条当前版本无法安全判断的操作
动作：查看详情 / 在电脑端处理 / 稍后再看
```

重要状态不能只通过 Toast 反馈。Toast 只能作为补充。

## 6. Unknown Event 处理规范

普通用户默认看到：

- 发生了什么；
- 是否影响当前任务；
- 可以做什么。

技术详情折叠显示：

- 原始 method；
- correlation id；
- protocol version；
- raw payload；
- decoded fields。

安全动作由 daemon/adapter 明确声明，不允许客户端根据未知 method 自己猜测动作。

## 7. 协同规范

第一阶段支持：

- 只读并行分析；
- 隔离 worktree 的并行修改；
- 用户逐成员审批；
- 用户逐成员重试、取消和查看结果。

不支持默认自动合并。任何汇总结果都必须标明是原始成员结果还是汇总 Agent 的二次判断。

## 8. 兼容性

- `pocketagent.v1` 只增加字段和 RPC，不修改既有字段语义。
- 老客户端继续使用 `DispatchCollabResponse`、旧 `UnknownEventSummary`、旧 `OptimizePromptRequest`。
- 新客户端通过 capability 判断新 RPC 和字段是否可用。
- 旧 JSON 数据加载时缺失的 message id、selection source、collab session 字段必须自动补齐或保持空值，不得阻止启动。

## 9. 验收指标

- 协同成员终局状态回填成功率 100%；重启后能恢复已持久化协同会话。
- 未知事件没有用户可见的裸 gRPC/JSON 错误；技术原文仍可追溯。
- 模型/effort 在 UI、daemon、TurnStarted 三处一致。
- 搜索结果可跨分页和加载更早定位，不依赖数组下标。
- 提示词优化成功后原文可恢复，优化失败不改动输入。
- 语音切换会话、旋转页面、系统回调迟到时不把文本写入错误会话。
- 多 Agent 可写任务不共享未隔离工作区。
- 所有子项目通过对应自动化测试和项目审核门禁。
