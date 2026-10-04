# PocketAgent 发布与端到端验证 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans`. 本计划在 P1-P4 功能实现后执行，不接受“单测通过就发布”。

**Goal:** 建立覆盖协同、未知事件、模型选择、历史、提示词优化、语音和分发链的端到端验证与发布门禁。

**Architecture:** 采用 deterministic fake agent + daemon integration + Android JVM + 真机 L2 四层验证。所有测试围绕用户可观察状态、恢复、取消、权限和安全边界设计，不只检查 RPC 返回码。

**Tech Stack:** Go test/race、Python audit tools、Gradle `--rerun`、ADB/真机驱动、Windows `build.ps1`、现有 evidence 目录。

**Spec:** `docs/superpowers/specs/2026-10-04-pocketagent-user-experience-and-reliability-upgrade.md`

## Global Constraints

- 所有结果必须带 commit、构建指纹、时间戳、结论和证据路径。
- 失败、BLOCKED 和未测必须带原因与解封条件。
- 任何 release 不能使用过期测试 XML、过期 zip 或旧 daemon 进程证明新代码有效。
- 真机测试不得自动替用户做危险审批；审批动作必须由测试脚本明确注入。
- release APK 和 daemon 必须通过个人路径/凭据/旧版本痕迹扫描。

## Review Focus

- 子进程卡死、stdin 阻塞、事件迟到和 daemon 重启后的状态恢复。
- Android 断线、切会话、进程被杀、旋转、后台恢复。
- 多 Agent 协同的部分成功、审批等待、取消、重试和工作区冲突。
- 未知事件的技术详情脱敏和普通用户文案。
- 构建产物、安装任务、token、TLS 和 zip 新鲜度。

### Task 1: 建立 deterministic fake agent

**Files:**
- Create: `daemon/internal/testagent/fake_agent.go`
- Create: `daemon/internal/testagent/fake_agent_test.go`
- Modify: `daemon/internal/agent/provider.go` only to expose test factory seam
- Test: `daemon/internal/server/*_test.go`

**Interfaces:**

```go
type Scenario struct {
    Name string
    Events []AgentEvent
    Delay time.Duration
    Approval *ApprovalScenario
    ExitAfter time.Duration
    IgnoreStdin bool
}

type FakeAgentFactory interface {
    Provider(id string, scenario Scenario) agent.Provider
}
```

必须覆盖：

- 正常文本和工具调用；
- 未知通知/请求/响应；
- 等待审批；
- 迟到响应；
- 子进程退出；
- 150 秒静默看门狗；
- 取消和重试。

### Task 2: daemon 协同 E2E

**Files:**
- Create: `daemon/internal/server/collab_e2e_test.go`
- Create: `daemon/internal/server/collab_recovery_e2e_test.go`
- Create: `docs/verify/evidence/collab-e2e.md`

测试场景：

- 两成员并行分析，均完成，App/daemon snapshot 终局一致；
- 一成员失败，另一成员完成；
- 一成员等待审批，另一成员继续；
- 取消单成员；
- 取消整个 session；
- daemon 重启后恢复 session；
- 事件序号从断点继续；
- 隔离 worktree 产生独立 diff；
- 共享工作区写任务被安全拒绝。

### Task 3: 未知事件 E2E

**Files:**
- Create: `daemon/internal/server/unknown_event_e2e_test.go`
- Create: `android/app/src/test/java/dev/pocketagent/app/ui/UnknownEventE2ETest.kt` if local fake transport exists
- Create: `docs/verify/evidence/unknown-event-e2e.md`

验证：

- notification 不结束轮次；
- unknown request 不伪造批准；
- safe action 显示真实动作并带 revision；
- 技术详情脱敏；
- 处置状态持久化；
- 旧 daemon 缺字段时新 App 人话降级；
- 详情 RPC 失败不把堆栈直接显示给用户。

### Task 4: 模型、历史、提示词和语音集成测试

**Files:**
- Create: `android/app/src/test/java/dev/pocketagent/app/ui/ConversationUpgradeIntegrationTest.kt`
- Create: `android/app/src/test/java/dev/pocketagent/app/ui/PromptVoiceIntegrationTest.kt`
- Create: `daemon/internal/server/model_history_prompt_e2e_test.go`

验证：

- 模型切换只影响下一轮；`TurnStarted` 与 UI 一致；
- effort 不可用时清理并显示原因；
- 历史搜索命中未加载消息时自动拉取并定位；
- 搜索失败和无结果区别显示；
- 优化期间输入变化不会被覆盖；
- 语音迟到回调不会进入新会话；
- offline snapshot 只读且显示时间。

### Task 5: Android 性能和生命周期回归

**Files:**
- Modify/Create: `android/app/src/androidTest/*` as existing test layout allows
- Create: `docs/verify/evidence/android-lifecycle-performance.md`

检查：

- 长消息流式渲染不做每 delta 全文 Markdown 重解析；
- Chat/History 使用稳定 key；
- 文件/PDF/Office 解析不在主线程；
- Activity STOP 后不继续收集无关流；
- 旋转/字号/主题切换不丢会话、语音和终端状态；
- 500/1000/5000 条历史消息滚动和搜索可用。

### Task 6: 安全和路径围栏回归

**Files:**
- Create: `daemon/internal/server/security_regression_test.go`
- Modify: `tools/audit/pa_audit.py` if rules need formalization
- Create: `docs/verify/evidence/security-regression.md`

验证：

- TLS 初始化失败 fail-closed；
- `-plaintext` 只作为显式应急选项并限制回环；
- 文件删除/移动/写入不能越过 workspace guard；
- junction/symlink/UNC/盘根被正确处理；
- token 不出现在计划任务命令行；
- 日志不含 token、完整语音文本、敏感路径；
- 未知 request 不存在通用允许捷径。

### Task 7: 发布链验证

**Files:**
- Modify: `dist/windows/build.ps1` only for missing gates
- Modify: `dist/windows/install.bat` only for missing health checks
- Modify: `android/build-hidden.vbs` / `android/build-ascii.ps1` if needed
- Test: `tools/audit/test_pa_audit.py`
- Create: `docs/verify/evidence/release-2026-10-04-upgrade.md`

门禁：

```powershell
python tools/audit/pa_upgrade_gate.py --stage release --json
python tools/device/_pa_gradle.py cleanapk
python tools/device/_pa_gradle.py test
pwsh -File dist/windows/build.ps1
```

- [ ] release/minify 构建通过。
- [ ] APK/daemon 版本与 commit 一致。
- [ ] zip 新鲜度不早于 build script。
- [ ] zip 内容包含正确 exe、安装脚本和版本信息。
- [ ] 安装后有界探活确认业务健康，不只确认端口打开。
- [ ] 安装/卸载/升级不会留下旧 token 或旧计划任务参数。

### Task 8: 真机 L2 用例矩阵

至少覆盖：

| 用例 | 期望 |
|---|---|
| USB → LAN → Tailscale 切换 | 连接状态和任务不丢 |
| 运行中切换模型/effort | 当前轮不变，下一轮明确生效 |
| 语音系统引擎死区 | 自动给出离线引擎路径 |
| 语音切换会话 | 旧文本不进入新会话 |
| 搜索更早消息 | 自动加载并准确定位 |
| 未知 request | 不自动批准，用户看到人话和技术详情 |
| 协同成员等待审批 | 只阻塞对应成员 |
| 协同成员完成/失败 | 协同页终局准确 |
| daemon 重启 | 线程、事件和协同状态可恢复 |
| offline snapshot | 明确只读和时间，不允许假发送 |

### Task 9: 发布准出

- [ ] P0/redline = 0。
- [ ] 受影响 JVM/Go/race 测试通过。
- [ ] 关键 L2 用例无 BLOCKED；BLOCKED 有归因和解封条件。
- [ ] evidence 文档完整。
- [ ] README/AUDIT 真值锚点更新。
- [ ] 提交：`test: validate end-to-end upgrade gates`。
