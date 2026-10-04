# PocketAgent 基线与门禁 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans`. This plan is a prerequisite for all feature work.

**Goal:** 建立升级前可重复、可审查、可回归的项目基线，并清掉当前审核门禁中的既有阻塞项。

**Architecture:** 不改变产品架构。先把审核、proto、Android 测试、daemon 构建和发布脚本变成可复用的门禁，避免后续多个 agent 在假绿或契约漂移的基础上并行开发。

**Tech Stack:** PowerShell 7、Python 3、Go 1.27.1、Gradle/Android、protobuf/gRPC。

**Spec:** `docs/superpowers/specs/2026-10-04-pocketagent-user-experience-and-reliability-upgrade.md`

## Global Constraints

- 当前工作目录是 `G:\项目1\pocketagent`；所有命令使用 PowerShell 7。
- 不修改用户桌面 Agent 配置，不使用个人绝对路径作为产品逻辑。
- Android 测试必须使用 ASCII 镜像脚本并带 `--rerun`。
- `proto/v1` 和 Android proto 副本必须逐文件 MD5 一致。

## Review Focus

- `pa_audit` 当前 redline 是否确实为展示名单误报，不能直接加豁免绕过。
- Android 测试 XML 是否为本轮生成，而不是 Gradle UP-TO-DATE 假绿。
- 审核脚本和构建脚本自身不能把个人路径、token 或旧 zip 当成产品输入。
- 任何基线修复不能改变现有功能语义。
- 基线完成后，后续 agent 能否只复制命令就复现结果。

### Task 1: 记录基线并锁定审查输出

**Files:**
- Create: `docs/verify/baselines/2026-10-04-upgrade-baseline.md`
- Modify: `docs/reviews/2026-10-04-full-code-review.md` only if current facts need correction
- Test: none

**Interfaces:**
- Produces: 基线 commit、测试用例数、审核 JSON、构建版本、已知 open 项清单。

- [ ] **Step 1: 运行只读基线命令**

```powershell
git status --short --branch
git log -1 --oneline
python tools/audit/pa_audit.py --json
```

同时记录 `C:\go\bin\go.exe` 的版本、Android 测试目录 mtime 和当前 APK/daemon 版本。

- [ ] **Step 2: 写入基线文档**

明确区分：

- 本轮实际通过的规则；
- 当前 redline；
- 当前 warn；
- 仅来自历史审查报告但尚未重新验证的结论。

- [ ] **Step 3: 验证文档无产品代码变更**

运行 `git diff --check`，确认只新增基线文档。

### Task 2: 清理当前 `pa_audit` redline

**Files:**
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/AgentScopeBadge.kt`
- Modify: `AUDIT.md` only if the static display roster is explicitly ruled as an allowed presentation-only list
- Test: `android/app/src/test/java/dev/pocketagent/app/ui/AgentScopeBadgeTest.kt`

**Interfaces:**
- Consumes: `resolveAgentAvailability` / capability-based agent data。
- Produces: `SESSION_DISPLAY_SCOPES` 不再触发未登记 roster redline，且不参与能力判定。

- [ ] **Step 1: 写回归测试**

验证展示标签可以保留基线项，但新增 Agent 不会因为能力判定缺失而被隐藏；同时验证未知 Agent 显示原生 id。

- [ ] **Step 2: 运行定向 JVM 测试，确认现状行为**

使用 Android 镜像脚本运行对应测试；如果是静态扫描问题，先记录预期失败输出。

- [ ] **Step 3: 选择最小修法**

优先把展示用集合改成明确的 `presentation-only` 数据源，或把真实能力判定完全移出该集合；禁止仅在 linter 中加无说明豁免。

- [ ] **Step 4: 复跑 `pa_audit.py --json`**

预期：roster redline = 0；若仍有命中，必须在审核文档中写出原因和行为测试。

### Task 3: 建立 Android fresh-test 门禁

**Files:**
- Inspect/Modify: `tools/device/_pa_gradle.py`
- Inspect/Modify: `tools/audit/pa_audit.py`
- Test: `tools/audit/test_pa_audit.py`

**Interfaces:**
- Produces: 可识别本轮测试结果 XML、tests/failures/errors、mtime 和 exit code 的机器输出。

- [ ] **Step 1: 为旧 XML、UP-TO-DATE 和失败 XML 写测试**
- [ ] **Step 2: 确认 `test` 分支强制 `--rerun`**
- [ ] **Step 3: 验证脚本在路径含中文时仍使用 ASCII 镜像**
- [ ] **Step 4: 运行脚本单测和一次真实测试**

命令：

```powershell
python tools/audit/test_pa_audit.py
python tools/device/_pa_gradle.py test
```

### Task 4: 生成升级批次的标准验收入口

**Files:**
- Create: `tools/audit/pa_upgrade_gate.py`
- Create: `tools/audit/test_pa_upgrade_gate.py`
- Modify: `AUDIT.md`

**Interfaces:**

```text
pa_upgrade_gate.py --stage baseline|contract|feature|release [--json]
```

输出必须包括：

- git commit；
- proto sync；
- daemon build/vet；
- Android 测试结果及 XML 时间；
- pa_audit redline/warn/pass；
- 当前阶段是否允许进入下一阶段。

- [ ] **Step 1: 为每个 gate 状态写 fixture 测试**
- [ ] **Step 2: 实现只读编排器**
- [ ] **Step 3: 在 PowerShell 7 下跑完整 gate**
- [ ] **Step 4: 写入使用说明和失败处理方式**

### Task 5: 完成基线验收

- [ ] 运行 `git diff --check`。
- [ ] 运行 `python tools/audit/pa_audit.py --json`，无新增 redline。
- [ ] 运行 daemon build/vet。
- [ ] 使用 `--rerun` 运行 Android 受影响测试。
- [ ] 将命令、时间、commit 和证据路径写入基线文档。
- [ ] 提交一个独立 commit：`chore: establish upgrade baseline and gates`。
