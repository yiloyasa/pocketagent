# PocketAgent 全面升级计划总览

> **For agentic workers:** 本计划必须按子计划逐项执行。推荐使用 `superpowers:subagent-driven-development`：每个子任务由独立 agent 实现，并由独立 reviewer 验收；禁止多个 agent 同时修改 protobuf 双副本或同一个大文件。

**Goal:** 将 PocketAgent 从“功能丰富但跨域状态分散的模块化单体”升级为“可恢复、可解释、可操作、可扩展的本地 Agent 控制平台”，重点闭环 UI/交互、未知事件、历史查询、提示词优化、语音、模型选择和多 Agent 协同。

**Architecture:** 保留 Go daemon + Android Compose + gRPC/protobuf + 本地无云部署，不做微服务拆分和大规模重写。以 daemon 作为事实源，以能力描述和事件流驱动客户端；将线程、轮次、协同会话、未知事件和模型选择变成可恢复的领域状态。

**Tech Stack:** Go 1.27.1、gRPC/protobuf、Kotlin/Compose、JVM 单测、Windows daemon、Android minSdk 26；继续使用项目现有 `tools/audit/pa_audit.py`、ASCII 镜像构建和真机验收链。

**Spec:** `docs/superpowers/specs/2026-10-04-pocketagent-user-experience-and-reliability-upgrade.md`

## Global Constraints

- 不推倒重写；每个子计划必须保持旧客户端与旧数据的兼容路径。
- 不伪造 Agent 完成、审批、父子关系、模型能力或未知事件含义。
- 未知输入优先透传；无法安全处理时要明确说明，不能伪造 decision。
- `proto/v1/*.proto` 与 Android proto 副本必须同步；每次契约改动必须运行 MD5 校验。
- 修改 Agent/Server/UI 代码必须补自动化回归；Android 测试使用 `--rerun` 并核验 XML 新鲜度。
- 不允许新增无界队列、锁内磁盘 IO、不可取消 stdin/PTY 写入或静默吞错。
- 所有用户可见失败必须包含“发生了什么、影响是什么、下一步能做什么”。
- 重要状态不能只用 Toast；必须进入页面状态、任务中心、时间线卡片或可恢复操作区。
- 多 Agent 写入任务必须使用只读模式或隔离 worktree；禁止多个成员默认同时改同一工作区。
- 运行中切换模型/思考强度默认只影响下一轮，必须在 UI 明示。

## Review Focus

- 协同成员完成但 App 仍显示“执行中”：由协同会话事件和 `DONE/ERROR/CANCELLED` 回归测试覆盖。
- 未知反向请求被展示为普通错误或出现错误的“允许”按钮：由安全等级和无安全动作测试覆盖。
- 模型/思考强度 UI 与 daemon 实际生效值不一致：由 source、pending、turn snapshot 测试覆盖。
- 搜索结果因分页/回滚/加载更早而跳错消息：由稳定 `message_id` 和自动加载测试覆盖。
- 语音识别因页面切换、权限、死区或重组而丢文本：由 VoiceSession reducer 和真机脚本覆盖。

## 子计划与依赖

| 编号 | 子计划 | 交付重点 | 前置依赖 |
|---|---|---|---|
| P0 | `2026-10-04-baseline-and-gates.md` | 审核红线、测试基线、协议变更流程 | 无 |
| P1 | `2026-10-04-collab-session-closure.md` | 协同状态、进度、审批、持久化、隔离 | P0；协议 steward |
| P2 | `2026-10-04-unknown-event-action-center.md` | 未知事件人话解释、会话内卡片、可行动处理 | P0；协议 steward |
| P3 | `2026-10-04-effective-selection-and-conversation-ux.md` | 模型/思考强度真值、历史搜索、任务中心、UI 分层 | P0；可与 P1/P2 并行 |
| P4 | `2026-10-04-prompt-and-voice-input.md` | 提示词优化预览、多模式、语音控制器 | P0；P3 的 UI 状态规范 |
| P5 | `2026-10-04-release-and-end-to-end-validation.md` | E2E、混沌场景、发布和真机闭环 | P1-P4 |

## 推荐执行批次

### 批次 0：基线与门禁

- 修复当前 `pa_audit` redline：`android/.../ui/AgentScopeBadge.kt:42`。
- 处理两个 UI“未知”扫描项，确认是否用户可见；不能直接删掉诊断语义。
- 用 ASCII 镜像运行 Android 测试 `--rerun`，记录 1319 用例基线及 XML 时间。
- 运行 daemon `go build ./...`、`go vet ./...`、关键包 `go test -race`。
- 锁定 proto 字段号分配表；之后由一个 protocol steward 统一修改两份 proto 和生成代码。

### 批次 1：先修“真实不可用”闭环

并行执行 P1/P2，但 protobuf 只允许 steward 合并：

- P1：协同 DONE/ERROR/取消/审批/进度真实回填。
- P2：未知事件从 Runtime 设置页扩展到会话时间线和任务中心。

这批完成后，产品才不会出现“功能看似存在、状态永远不结束”的问题。

### 批次 2：统一会话真值和输入体验

- P3：模型/思考强度统一来源、历史搜索稳定定位、任务中心和 UI 状态治理。
- P4：提示词优化和语音输入从 ChatScreen 内部逻辑迁出，形成可测的输入服务。

### 批次 3：端到端验证和发布

- P5：真机断线、子进程退出、未知请求、协同写入隔离、模型切换、语音死区、安装升级完整验证。

## Agent 协作规则

### Protocol steward

唯一负责：

- 修改 `proto/v1/*.proto`；
- 同步 `android/app/src/main/proto/v1/*.proto`；
- 运行生成器；
- 维护字段号和兼容说明；
- 提交契约 golden test。

其他 agent 不得直接修改 proto；如需字段，先在自己的计划中写出 proposal，交 steward 合并。

### 子项目 agent

每个 agent 必须：

1. 先读根目录 `README.md`、`AUDIT.md`、本总览和自己的子计划；
2. 先写失败测试或回归探针，再改实现；
3. 只修改自己计划列出的文件；
4. 不把技术错误原文直接暴露给普通用户；原文放技术详情；
5. 运行计划中的最小测试，再运行项目级门禁；
6. 输出变更摘要、验证命令、未解决风险和需要其他 agent 接手的接口。

### Reviewer agent

审查重点：

- 是否出现伪造完成/批准/能力；
- 是否新增静态 agent/model 名单；
- 是否存在重复事实源；
- 是否有状态没有终局或无法恢复；
- 是否把失败静默成空列表/默认值；
- 是否破坏旧客户端和旧数据；
- 是否遗漏 Android proto 副本或测试 XML 新鲜度。

## 统一完成标准

一个子计划只有在以下条件同时满足时才算完成：

- 代码路径存在明确的状态机或事实源；
- 失败、取消、超时、重启和断线都有定义行为；
- 用户可见文案能用大白话说明原因和下一步；
- 单测覆盖正常、失败、未知、重复和旧数据场景；
- daemon build/vet/test 通过；
- Android 受影响测试使用 `--rerun` 通过；
- proto 双副本一致；
- `pa_audit.py --json` 无新增 redline；
- 文档更新包含接口、限制和证据路径。

## 禁止的捷径

- 不要把协同成员的“已受理”显示成“已完成”。
- 不要给未知请求增加一个泛化的“允许”按钮。
- 不要让搜索失败返回空结果而不说明范围。
- 不要让模型切换只改 SharedPreferences 而不更新 daemon 真值。
- 不要为了让协同成功而默认关闭审批或共享写工作区。
- 不要把所有新逻辑继续堆进 `ChatScreen.kt`、`SettingsScreen.kt` 或 `RuntimeSection.kt`。


## 可复制的 Agent 启动提示词模板

将下面模板和对应子计划一起发送给执行 agent：

```text
你是 PocketAgent 升级项目的子项目 agent。

项目目录：G:\项目1\pocketagent
当前日期：2026-10-04
你负责的子计划：<填写 P0/P1/P2/P3/P4/P5 及计划文件绝对路径>

先阅读：
1. README.md
2. AUDIT.md
3. docs/superpowers/specs/2026-10-04-pocketagent-user-experience-and-reliability-upgrade.md
4. docs/superpowers/plans/2026-10-04-pocketagent-upgrade-roadmap.md
5. 你的子计划文件

执行规则：
- 使用 PowerShell 7；先读代码和现有测试，再改代码。
- 只修改子计划 Files 中列出的文件；需要跨范围时先停下并报告。
- protobuf 只能由 protocol steward 修改；其他 agent 先提交字段 proposal。
- 先写失败测试或可复现探针，再写最小实现。
- 不伪造完成、审批、能力、未知事件含义或父子关系。
- 不把技术错误原文直接展示给普通用户；原文放技术详情，人话说明原因和下一步。
- 不新增静态 Agent/model 名单；能力使用 daemon 动态描述。
- 每个阻塞操作必须有取消、超时或明确的失败路径。
- 不默认共享多 Agent 可写工作区。

完成时必须输出：
1. 修改文件清单；
2. 新增/修改接口；
3. 测试命令及实际结果；
4. 证据路径；
5. 未解决问题、风险和下一步依赖；
6. 建议的 commit message。

完成前运行：
- git diff --check
- 对应定向测试
- daemon: C:\go\bin\go.exe build ./... 或计划中指定命令
- Android: python tools/device/_pa_gradle.py test，并确认使用 --rerun 和 XML mtime
- python tools/audit/pa_audit.py --json
```

## Agent 交接格式

```markdown
## 子项目交接

- 子计划：P?
- 状态：PASS / BLOCKED / NEEDS_REVIEW
- 修改：
- 接口：
- 测试：
- 证据：
- 未解决：
- 下一个 agent 必须先做：
```
