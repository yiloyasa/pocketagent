# PocketAgent 审核规范（AUDIT）

> **本文件是项目唯一的审核标准。任何更改（代码 / 契约 / 配置 / 文档）合入前，必须严格按本规范执行，无例外。**
> README 是项目整体上下文，本文件是审核执行标准；审核事项上两者表述不一致时，**以本文件为准**。

---

## 0. 总原则

1. **审核目的**：防假绿、防伪造、防静态名单埋雷、防上下文漂移。
2. **机器执行流水线，人只做机器做不了的裁决**——能自动化的检查全部进脚本；人只做真机用例与红线裁决。
3. **审核自身也遵守「能透传就透传」**：不一脚踩死——检查分**红线项**（一票否决）与**提示项**（限期补录，不阻塞）。
4. **框架自身有反馈环**：度量回流，每里程碑自审一次，流程允许演进，但演进必须改本文件，不许私下破例。

---

## 1. 变更分级（自动定级，人不许自行降级）

读 `git diff --name-only`，按路径信号定级；命中多个信号时**取最高级**：

| 级 | 信号（改动路径） | 审核深度 |
|---|---|---|
| **甲（契约/架构）** | `proto/**`、两份 proto 不同步、新增 agent/服务 | L0 闸门 + L1 全量 + L2 全量 + 契约双向核验 |
| **乙（功能）** | `daemon/internal/agent/**`、`daemon/internal/server/**`、`android/**/ui/**`、资源文件 | L1 全量 + L2 受影响模块全量 + 其余冒烟 |
| **丙（修复）** | 以上都不是的小改动 | L1 全量 + 该缺陷单条复验 + **附回归用例**（见 §4） |
| **丁（文档/配置）** | 仅 `*.md`、非代码配置 | 只查真值锚点漂移（README §10） |

> 分级字母刻意不用 A/B/C/D——避免与 README §7 的 A/B/C 方案（透传/决定权/版本探测）混淆。

---

## 2. 三层审核

### L0 闸门（甲类必过；不过则回修，不进 L1/L2）

- P0 清零（当前：PA-REAL-0001 鉴权未启用）。
- **产物自证**（防「源码改对 ≠ 跑着的是新的」）：
  - APK：`apkanalyzer dex packages` 查新增类在包里 + dex 内 grep 本轮新增中文字面量；
  - daemon：确认 pid/7246 为新进程（热部署走 `tools/device/_pa_rebuild_daemon.py`）。

### L1 自动化回归（每次代码变更必跑）

- JVM 448 用例：`python tools/device/_pa_gradle.py test`，**必须 `--rerun`**，并解析 `app/build/test-results/**/*.xml` 的 tests/failures + 时间戳是否本次（防 Gradle UP-TO-DATE 假绿）。
- daemon：`go test ./...`；`CGO_ENABLED=1 go test -race`（server/agent/store/codexapp/piapp/zcodeapp）。
- 体积结论必须走 `cleanapk`（AGP 增量会在 APK 留孤儿 dex，增量包结论不可信）。

### L2 真机验收

- 节奏：每轮只跑**受影响模块全量 + 其余冒烟**；里程碑（P0 修完 / P1 修完 / 发版候选）才 131 全量。
- **BLOCKED 必须带归因 + 解封条件**（归因三类：P0 依赖 / 环境依赖 / 用例缺陷），按类批量解封，不逐条硬啃；只写 blocked 不写原因的视为未测。
- 真机驱动三铁律（`tools/device/_pa_audit_drv.py` 已内建）：每脚本开头 `ensure_reverse()`；关键点击 dump 校验 + 自愈重试；BACK 会打回桌面 → 导航一律 force-stop + 冷启动。

---

## 3. 常驻审核维度（每次必过，与变更内容无关）

### 3.1 红线项（一票否决，直接打回）

1. **伪造 decision**：daemon 替用户做任何审批决定（README §7 核心红线）。
2. **假绿**：测试未 `--rerun`；结果 XML 时间戳非本次；拿增量 APK 当体积/功能结论。
3. **契约不同步**：proto 改一份未同步另一份；RPC 计数（现 7 服务 / 79 RPC，2026-09-24 按 proto/v1 实测校正）变化未在提交信息中说明。
4. **产物不对**：apkanalyzer 查无新类 / dex 无新字面量 / daemon 跑的是旧进程。

### 3.2 提示项（不阻塞合入，限期补录）

5. **真值锚点漂移**：README §10 的版本/体积/pid/基线数字与实测不符 → 同次或下次提交前补更新。
6. **证据链缺项**：五要素不全（见 §5）→ 24 小时内补齐。

### 3.3 设计原则合规（README §7 的可执行版）

每次更改回答三问：

1. **本次是否新增写死的名单/顺序/版本判断？**（模型白名单、agent 顺序、`isLegacy*` 版本二分）
   → 有则改为透传；实在改不了的，显式挂牌进 README §7「已踩死」表。**白名单只许减、不许增**。
2. **遇到不认识的输入走哪条路？** 透传 / 回 error / 伪造——伪造 = 红线 1，直接打回。
3. **UI 遇到未知值显示什么？** 必须显示原生原文（「原生发来了一个新事件」），**禁止显示「未知/错误」**。

机器可扫部分（linter，进 `pa_audit`）：

- 硬编码名单模式报警：agent 字面量数组（如 `[]string{"codex","pi","zcode"}`）、`isLegacy*` 版本二分函数——**新增即报警**；存量挂牌白名单仅剩 1 处（`codex_requests.go` isLegacyApproval，只减不增；CreateThread/order/defaultModelIDFor 三处已于批3整改，见附录 A），其余命中必须在附录 A 登记并说明为什么不能改为能力透传。
- UI 层新增 `"未知"` 字符串 → 报警。
- sheet 溢出巡检（批10 T-PA 固化）：android 全仓 `ModalBottomSheet` 调用点逐站列出处置（溢出风险/有滚动/Lazy 容器/heightIn 上限/静态内容），内容块含 forEach/items 列表渲染而同块无 `verticalScroll`/`LazyColumn`/`heightIn(max)` 上限 → 报警；T-NS/T-SS 已修处回归即现形。
- 假绿哨兵（批10 T-PA 固化）：解析 `testDebugUnitTest` 结果 XML 的 tests/failures 计数，failures/errors>0 → 红线；最新结果 XML mtime 距今 >10 分钟 → 提示疑似 Gradle UP-TO-DATE，需 `--rerun`（§2 L1 时间戳口径的规则化）。

常驻可测用例（回归资产）：mock 未知 method 通知、mock 新模型 ID、mock 未知反向请求（B 开关 ON/OFF 各一遍）、mock 第 4 个 agent 建会话（当前为挂牌技术债，只记录行为、不要求通过）。

---

## 4. 缺陷即测试

- 每个丙类修复**必须附带自动化用例**（JVM 或 go），否则不算修完。当前 P0/P1/P2 修复同样适用。
- 真机踩坑固化进 `tools/` 驱动脚本（如 `_pa_audit_drv.py` 三铁律）。
- **审核资产只增不减**：回归用例数、扫描规则数只许涨；要删必须在本文件记录理由。

---

## 5. 证据链五要素

每条用例结论必须绑定：**构建指纹（APK sha256 + daemon 版本/commit）+ 时间戳 + 结论 + 证据路径 + 归因**。

- 截图/录屏/dump 磁盘留存于 `docs/verify/evidence/`，**不入 git**；git 只入结论 md/json（证据目录 `.gitignore` 已配：`*` + `!*/` + `!*.md`/`!*.json`/`!*.txt`）。
- 审核记录（结论表）入库，历史可追溯。

---

## 6. 准出标准（发版 DoD）

- P0 = 0；P1 = 0（或有书面降级理由，写入本文件附录）。
- 131 用例：FAIL = 0、BLOCKED = 0。
- 已知真值不回退：冷启 ≤ 基线 700.8ms × 1.2；崩溃 / ANR / 数据丢失 = 0；TLS1.3；三 Agent 零交叉污染。
- **debug 包结论不外推 release**；上 release 前单独走一轮完整三层。

---

## 7. 执行入口与度量回流

- **入口脚本 `tools/audit/pa_audit.py`**（v1 已建，批10 T-PA，2026-09-21）：六条规则独立开关（`--only`/`--skip`，`--json` 机器可读）——名单 linter（对照附录 A，未登记命中即红线）、UI「未知」扫描（注释豁免）、sheet 溢出巡检（逐站处置清单）、假绿哨兵（XML 计数 + mtime）、daemon `go build`+`go vet` 基线、proto 两副本逐文件 MD5 核验；输出过/红线/提示三栏报告。定级、L1 全量、产物自证仍按 §1/§2 人工/既有脚本执行。人拿到报告只做：真机用例 + 红线裁决。
- **度量四数**（每轮记录）：BLOCKED 积压数、缺陷逃逸率（自动化未拦住、真机/用户才发现的）、每轮审核耗时、回归用例累积数。
- **框架自审**（每里程碑一次）：从没抓到过问题的检查降级；反复出现的问题加固化（变脚本/变用例）。自审结论追加到本文件附录。

---

## 附录 A · 当前挂牌技术债（只减不增）

| 位置 | 问题 | 状态 |
|---|---|---|
| `internal/server/thread.go` CreateThread（L99） | 未知 agent → 硬错误 | 已整改（2026-09-20, 批3）：在 `s.cfg.Providers` 注册表即接入，报错文案动态列举当前可用 agent |
| `internal/server/model.go` order（L73） | agent 顺序写死 | 已整改（2026-09-20, 批3）：顺序从 `s.cfg.Providers` 动态派生（桌面桥接 PrefsAware 在前、echo 最后），新 provider 自动入列 |
| `internal/server/model.go` defaultModelIDFor | 假设 `agent+"/default"` 存在 | 已整改（2026-09-20, 批3）：default 哨兵 → 模型列表首项 → 明确报错，不再伪造 ID |
| `internal/agent/codex_requests.go` isLegacyApproval | 版本号二分判断 | 待协议演进时整改 |
| `internal/server/agents_sessions.go` 导入名单 + `internal/server/thread.go` defaultProvider | 写死 `importAgentModels{pi,zcode}` 与 `[]string{"pi","zcode"}` 循环、`[]string{"codex","glm","deepseek"}` 优先级（体检 P1-4 / P1-5，原属未挂牌，登记在案） | 已整改（2026-09-20, 批3）：导入改为能力接口（`AgentSessionImporter`）动态发现、报错文案动态列举；defaultProvider 改为 store 默认模型前缀 → 字典序第一个非 echo → echo 兜底 |

### 豁免登记（非技术债，防 linter 误报；只增不减，每条附裁定）

| 位置 | 内容 | 豁免理由与裁定 |
|---|---|---|
| `android/.../ui/ChatScreen.kt` `systemVoiceRetireNeeded` | 系统 SR 错误码弃用名单（含增补 3=AUDIO_ERROR、9=INSUFFICIENT_PERMISSIONS，后者系 HyperOS fuxi 真机实证：App 权限已授时系统识别服务仍瞬间报 9 且永不可自愈） | Android SpeechRecognizer 公开错误码为封闭 SDK 契约（1..13 随 API 固定），非面向原生演化的开放名单；else 分支安全（未知码不弃用）。批1审核裁定不违规（2026-09-20）；9 的增补见 T-33（真机 L2 实证） |
| `android/.../ui/HomeScreen.kt` `placeholderDisplayAgents` | opencode/claude-code/hermes/dsh 展示目录 | 仅决定灰显副文案，不参与能力判定（能力判定已全动态）；只随 agent 接入移除、不随新增增长。批3审核裁定合规（2026-09-20） |
| `android/.../ui/SettingsScreen.kt` AgentDefaultsSection 按名特判 | `"pi","zcode" ->` 旧 daemon 兼容兜底 | else 分支的 `agent_settings:<id>` capability 动态判定已覆盖任意 agent；移除特判会让旧 daemon 用户丢失设置区。T-10 裁定维持现状（2026-09-20） |
| `android/.../ui/HomeScreen.kt` `buildAgents` 基线目录（7 项 AgentInfo） | 展示名/图标/连接方式基线表（pa_audit roster 首轮命中） | available 字段仅为基线缺省，能力判定走 `resolveAgentAvailability`（capabilities 动态+extra 追加，新 agent 零改动进网格）；与豁免 #2 同精神。pa_audit 首轮裁定登记（2026-09-21） |
| `android/.../ui/SettingsScreen.kt` optimizerAgents 回退 `setOf("codex","pi","zcode")` | 旧 daemon 无 text_generation capability 时的优化 agent 兜底 | 主路径 `"text_generation:$it" in capabilities` 动态判定在前，setOf 仅在 capabilities 全空时兜底（与豁免 #3 同款新旧兼容形态）。pa_audit 首轮裁定登记（2026-09-21） |
| `daemon/internal/agent/codex.go` `isOpaqueItemType` 排除五类 | item/started/completed 已专案处理类型（contextCompaction/commandExecution/agentMessage/subAgentActivity/collabAgentToolCall）的排除清单 | 黑名单形态：默认值=主线 A 透传，新类型自动进透传，漏向安全；专案分支由 T-L1 建。批 9 审核裁定不构成名单债（2026-09-21） |

## 附录 B · 框架自审记录

| 日期 | 结论 | 动作 |
|---|---|---|
| 2026-09-20 | 规范初版（三轮讨论固化：闸门制 → 持续框架 → 人机分工优化） | 建立本文件 |
| 2026-09-23 | `pa_audit` 发现 `collabrolecheck` 用固定 `pi/codex` 数组探测模型，违反能力动态发现原则 | 已改为读取 `GetHostInfo.capabilities` 中的 `agent:*`，新增 `daemon/cmd/collabrolecheck/main_test.go`；本轮复跑红线 0 |
| 2026-09-20 | 批7 死代码清理（T-24/T-26）：dynamicEffortsFor+孤儿接口、PiProvider.ThinkingLevels 及其测试删除 | 回归用例净 -2/-1：被删用例钉的是已删死函数的行为，无独立存在意义；等价性由全绿测试套佐证；同批新增 AST 回归钉子 +1（codexapp client_debug_ast_test）对冲 |
| 2026-09-20 | UP-TO-DATE 假绿单轮三次被审/执两侧抓住（批2/批3/批5 审核） | 已修复（批10 T-D3，2026-09-21）：tools/device/_pa_gradle.py test 分支已统一补 --rerun 参数；实测连续两次强制真跑并刷新结果 XML（04:26:52→13:34:32→13:35:03），apk/compile 分支不受影响 |
| 2026-09-20 | 批5 Vosk 全摘除（T-22）：VoskModelManager 状态机/安装器/识别循环等 21 个纯 vosk 用例删除（清单见 T-22 交付报告）；SenseVoiceFallbackTest 24 例同数承接引擎回退钉子，全仓用例数 484 持平不减 | 被测对象已随引擎整体摘除，无行为可钉；端侧兜底/文件校验行为面由 SenseVoice 系测试覆盖；删除逐例清单与理由存档于 T-22 交付报告（2026-09-20 夜航档案） |
| 2026-09-24 | 收口轮死代码清理：`hostenv.DaemonDataDir` 及其 3 个策略用例（prefers-new/falls-back-legacy/fresh-install）删除，`WithoutLocalAppData` 改钉仍在线的 `UserDataDir`；净回归用例 **-3**。同轮新增：Android `TurnSession.release` 删除释放回归 +1、`unknown_response` 脱敏钉子 +1（daemon）、migration 4 例（2026-09-24 未提交工作流自带） | 被删用例钉的是「不自动搬移」策略下死函数的择路规则；该策略与函数已被 `ResolveDataDir`（校验+原子 rename+失败回滚）取代，择路语义由 `migration_test.go` 覆盖。理由同批7先例：钉死代码行为无独立存在意义。同轮校正 §3.1 RPC 计数漂移（6/69 → 7/79） |
