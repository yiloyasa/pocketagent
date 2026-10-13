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
- daemon：`go test ./...`；`CGO_ENABLED=1 go test -race`（server/agent/store/codexapp/piapp/zcodeapp/rpcwire/dshapp/hermesapp/eventstore/runtime/collabmcp）。2026-10-09 扩包：rpcwire/dshapp/hermesapp/eventstore/runtime/collabmcp 首次纳入（当轮实跑全绿），eventstore 与 rotatelog 含并发回归用例（Append×回放赛跑）。
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
3. **契约不同步**：proto 改一份未同步另一份；RPC 计数（现 7 服务 / 84 RPC，2026-09-26 按 proto/v1 实测校正）变化未在提交信息中说明。
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
- **升级门禁 `tools/audit/pa_upgrade_gate.py`**（批次 0 新增，2026-10-04）：`--stage baseline|contract|feature|release [--json]` 只读编排器——汇总 git 干净度/proto sync/daemon build+vet/Android 测试新鲜度/audit redline/dist 新鲜度七检查项，按阶段闸表输出 ALLOW/BLOCK；升级工作（roadmap P1~P5）每批次完成以此验收，非语义 go 退出码按环境异常提示不误报。
- **度量四数**（每轮记录）：BLOCKED 积压数、缺陷逃逸率（自动化未拦住、真机/用户才发现的）、每轮审核耗时、回归用例累积数。
- **框架自审**（每里程碑一次）：从没抓到过问题的检查降级；反复出现的问题加固化（变脚本/变用例）。自审结论追加到本文件附录。

---

## 附录 A · 当前挂牌技术债（只减不增）

| 位置 | 问题 | 状态 | 删除条件/版本 |
|---|---|---|---|
| `internal/server/thread.go` CreateThread（L99） | 未知 agent → 硬错误 | 已整改（2026-09-20, 批3）：在 `s.cfg.Providers` 注册表即接入，报错文案动态列举当前可用 agent | 已整改，无残留 |
| `internal/server/model.go` order（L73） | agent 顺序写死 | 已整改（2026-09-20, 批3）：顺序从 `s.cfg.Providers` 动态派生（桌面桥接 PrefsAware 在前、echo 最后），新 provider 自动入列 | 已整改，无残留 |
| `internal/server/model.go` defaultModelIDFor | 假设 `agent+"/default"` 存在 | 已整改（2026-09-20, 批3）：default 哨兵 → 模型列表首项 → 明确报错，不再伪造 ID | 已整改，无残留 |
| `internal/agent/codex_requests.go` isLegacyApproval | 版本号二分判断 | 待协议演进时整改 | 协议演进版（未定版本，演进即删） |
| `internal/server/agents_sessions.go` 导入名单 + `internal/server/thread.go` defaultProvider | 写死 `importAgentModels{pi,zcode}` 与 `[]string{"pi","zcode"}` 循环、`[]string{"codex","glm","deepseek"}` 优先级（体检 P1-4 / P1-5，原属未挂牌，登记在案） | 已整改（2026-09-20, 批3）：导入改为能力接口（`AgentSessionImporter`）动态发现、报错文案动态列举；defaultProvider 改为 store 默认模型前缀 → 字典序第一个非 echo → echo 兜底 | 已整改，无残留 |

### 豁免登记（非技术债，防 linter 误报；只增不减，每条附裁定）

| 位置 | 内容 | 豁免理由与裁定 | 删除条件/版本 |
|---|---|---|---|
| `android/.../ui/ChatScreen.kt` `systemVoiceRetireNeeded` | 系统 SR 错误码弃用名单（含增补 3=AUDIO_ERROR、9=INSUFFICIENT_PERMISSIONS，后者系 HyperOS fuxi 真机实证：App 权限已授时系统识别服务仍瞬间报 9 且永不可自愈） | Android SpeechRecognizer 公开错误码为封闭 SDK 契约（1..13 随 API 固定），非面向原生演化的开放名单；else 分支安全（未知码不弃用）。批1审核裁定不违规（2026-09-20）；9 的增补见 T-33（真机 L2 实证） | SDK 契约变更时复核（无删除计划） |
| `android/.../ui/HomeScreen.kt` `placeholderDisplayAgents` | opencode/claude-code/hermes/dsh 展示目录 | 仅决定灰显副文案，不参与能力判定（能力判定已全动态）；只随 agent 接入移除、不随新增增长。批3审核裁定合规（2026-09-20） | 展示目录随 agent 接入移除时同步删（无版本号，随接入） |
| `android/.../ui/SettingsScreen.kt` AgentDefaultsSection 按名特判 | `"pi","zcode" ->` 旧 daemon 兼容兜底 | else 分支的 `agent_settings:<id>` capability 动态判定已覆盖任意 agent；移除特判会让旧 daemon 用户丢失设置区。T-10 裁定维持现状（2026-09-20） | 已删除（0.3.1 最新-only，见附录 B 2026-10-13） |
| `android/.../ui/HomeScreen.kt` `buildAgents` 基线目录（7 项 AgentInfo） | 展示名/图标/连接方式基线表（pa_audit roster 首轮命中） | available 字段仅为基线缺省，能力判定走 `resolveAgentAvailability`（capabilities 动态+extra 追加，新 agent 零改动进网格）；与豁免 #2 同精神。pa_audit 首轮裁定登记（2026-09-21） | 展示基线变更时复核（无删除计划） |
| `android/.../ui/SettingsScreen.kt` optimizerAgents 回退 `setOf("codex","pi","zcode")` | 旧 daemon 无 text_generation capability 时的优化 agent 兜底 | 主路径 `"text_generation:$it" in capabilities` 动态判定在前，setOf 仅在 capabilities 全空时兜底（与豁免 #3 同款新旧兼容形态）。pa_audit 首轮裁定登记（2026-09-21） | 已删除（0.3.1 最新-only，见附录 B 2026-10-13） |
| `daemon/internal/agent/codex.go` `isOpaqueItemType` 排除五类 | item/started/completed 已专案处理类型（contextCompaction/commandExecution/agentMessage/subAgentActivity/collabAgentToolCall）的排除清单 | 黑名单形态：默认值=主线 A 透传，新类型自动进透传，漏向安全；专案分支由 T-L1 建。批 9 审核裁定不构成名单债（2026-09-21） | 专案分支变更时复核（无删除计划） |
| `android/.../ui/AgentScopeBadge.kt` `SESSION_DISPLAY_SCOPES`（声明现于 :47） | codex/pi/zcode/hermes 展示顺序名单（「会话与对话」组作用域徽章） | presentation-only：仅决定徽章展示顺序，不参与能力判定/会话路由；动态能力透传会把外部只读导入(opencode/claude-code)误纳入全局显示偏好作用域，故不能改动态源——与豁免 #2/#4 同精神。新增 agent 须同步本行+`AgentScopeBadgeTest` 防漂移用例。批次 0 裁定登记（2026-10-04） | 新增 agent 时同步（无删除计划） |

## 附录 B · 框架自审记录

| 日期 | 结论 | 动作 |
|---|---|---|
| 2026-09-20 | 规范初版（三轮讨论固化：闸门制 → 持续框架 → 人机分工优化） | 建立本文件 |
| 2026-09-23 | `pa_audit` 发现 `collabrolecheck` 用固定 `pi/codex` 数组探测模型，违反能力动态发现原则 | 已改为读取 `GetHostInfo.capabilities` 中的 `agent:*`，新增 `daemon/cmd/collabrolecheck/main_test.go`；本轮复跑红线 0 |
| 2026-09-20 | 批7 死代码清理（T-24/T-26）：dynamicEffortsFor+孤儿接口、PiProvider.ThinkingLevels 及其测试删除 | 回归用例净 -2/-1：被删用例钉的是已删死函数的行为，无独立存在意义；等价性由全绿测试套佐证；同批新增 AST 回归钉子 +1（codexapp client_debug_ast_test）对冲 |
| 2026-09-20 | UP-TO-DATE 假绿单轮三次被审/执两侧抓住（批2/批3/批5 审核） | 已修复（批10 T-D3，2026-09-21）：tools/device/_pa_gradle.py test 分支已统一补 --rerun 参数；实测连续两次强制真跑并刷新结果 XML（04:26:52→13:34:32→13:35:03），apk/compile 分支不受影响 |
| 2026-09-20 | 批5 Vosk 全摘除（T-22）：VoskModelManager 状态机/安装器/识别循环等 21 个纯 vosk 用例删除（清单见 T-22 交付报告）；SenseVoiceFallbackTest 24 例同数承接引擎回退钉子，全仓用例数 484 持平不减 | 被测对象已随引擎整体摘除，无行为可钉；端侧兜底/文件校验行为面由 SenseVoice 系测试覆盖；删除逐例清单与理由存档于 T-22 交付报告（2026-09-20 夜航档案） |
| 2026-09-24 | 收口轮死代码清理：`hostenv.DaemonDataDir` 及其 3 个策略用例（prefers-new/falls-back-legacy/fresh-install）删除，`WithoutLocalAppData` 改钉仍在线的 `UserDataDir`；净回归用例 **-3**。同轮新增：Android `TurnSession.release` 删除释放回归 +1、`unknown_response` 脱敏钉子 +1（daemon）、migration 4 例（2026-09-24 未提交工作流自带） | 被删用例钉的是「不自动搬移」策略下死函数的择路规则；该策略与函数已被 `ResolveDataDir`（校验+原子 rename+失败回滚）取代，择路语义由 `migration_test.go` 覆盖。理由同批7先例：钉死代码行为无独立存在意义。同轮校正 §3.1 RPC 计数漂移（6/69 → 7/79） |
| 2026-09-25 | 优化轮（E1/E2/E3 三执行器并行 + 主模型验收）：D7 收尾 cache/resource 文件后端 + recovery 产出方 **+15**（runtime 包）；Android 运行时详情四块上屏 **+9**；新消息适配回归加固 **+8**（未知 method 分类/新模型 ID 透传保真/四 client 未知 response 分发/dsh 台账）——净回归用例 **+32**。`go vet` copylocks 拦下 1 处 proto 值拷贝（已改为内联字面量） | 全量：daemon 18 包 0 失败 + 受影响包 race 通过、android 89 XML/679 用例 0 失败、pa_audit 红线 0。挂牌未完成项：B 开关 ON 态全域无实现（实现后补钉）；codex/zcode 反向请求端到端钉待 fake harness 扩展；recovery 为推导式产出（hydrate 埋钩子精确化另轮） |
| 2026-09-25 | UI 改版轮（地基 + EA/EB/EC 三线并行）：sheet 滚动保证钉子升级为 **PaSheet 两级钉法**（sheet 段必须用 PaSheet + PaSheet 本体必须含 heightIn/verticalScroll）——审核资产形态升级非删减，覆盖四处钉子；新增设计系统文件 PaGlass/PaMotion/PaSheet。**tween 残留挂牌（刻意保留，非遗漏）**：Splash 打字节拍、麦克风波形/涟漪、UndoBar 倒计时、PaSkeleton shimmer——均为循环/节拍器语义，spring 无时长语义不适用；新写状态过渡已全部 spring 化 | 全量 89 XML/679 用例 0 失败、pa_audit 红线 0；模拟器（Pixel 5 API 34）实机视觉验收通过：推背模糊/玻璃材质/spring 转场/键盘 spring 跟随生效，证据 `docs/verify/evidence/ui-revamp/`（不入 git）。挂牌：ApprovalSheet/ExpandedInputSheet 未收敛（PaSheet 缺参数）；ChatScreen 配对态与深色主题玻璃手感待真机 L2；浅色主题遮罩亮度复核 |
| 2026-09-25 | **统一卡片语言铁律固化（b172c43，用户两轮裁定驱动）**：①全 app 唯一卡型=16dp 圆角+Pa.Card+无边框/无发丝边/无阴影，卡内距 16、卡距 12、页边距 16；②卡内禁止带底色子容器，行首图标裸放；③玻璃只存在浮层（抽屉/Sheet/聊天编辑条）+推背模糊；④阴影白名单只留 FAB；⑤语义横幅（错误 8% 底）不算卡；⑥视觉验收必须含负向检查（多余层/装饰），不得只验正向功能。中途「零卡片」方向被用户纠正为「简约但不简单」——统一卡型而非删光卡片 | 模拟器实机复验通过（07-home-unified/08-settings-unified.png）；compile+test 679/0 绿；audit 红线 0。挂牌：UndoBar 浮条内一处旧发丝边（浮层范围待授权）；输入框/Button/chip 圆角属控件非卡型未强制归一；「Agents」标题措辞待用户定夺 |
| 2026-09-25 | **L2 真机回归通过（部署链+控制面+透析）**：热部署 PID 72172、D8 迁移真实生效（2463 文件/93.6MB，marker 校验和，旧目录保留）、L0 双向自证（dex 字面量 3/3+GetControlPlane 在包）、防火墙补规则后手机 Tailscale 连通、控制面八域真机渲染+协同开关就地切换（落盘 revision 29→30 双向确认）、透析弹窗人话+五处置键真机渲染、聊天新 UI 上屏。证据 docs/verify/evidence/l2-20260925/（不入 git，结论见 RESULT.md）。发现适配债挂牌：①dsh rc.3 旧会话 resume 失败后 UI 重试无出路（需产品决策：提示重建或自动降级）；②pi 0.87.1 rpc 模式拉起即退待专项 |
| 2026-09-25 | **弹窗修复补全（用户反馈不彻底）**：AP2 封掉 codex notify/apps 后仍有周期弹窗——事件日志实锤计划任务每分钟经 wscript 跑 watchdog_hidden.vbs（VBScriptDeprecationAlert 20:38/20:27/20:16 齿轮式命中）。修复：Set-ScheduledTask 把 action 改为直跑 paguard.exe（windowsgui 物理无窗口），绕开 wscript；验证 wscript 告警归零、看门狗 port up 每分钟正常、daemon 存活。教训：AP2 的「计划任务链干净」结论只核对了进程树没核对**事件日志的调度频率证据**——弹窗类排查必须看 Application/System 日志的周期性命中【口径勘误 2026-10-04（弹窗审计 G1）：当日「windowsgui 物理无窗口」仅属设计自述——build.ps1 的 paguard 构建行实际未带 -H=windowsgui；现已补上该 flag（tools/ops/_pa_deploy_night.py 同步），并以打包前 PE 子系统断言（subsystem==2，失败即中止打包）守护，该口径自此由产物自检断言守护而非自述】 |
| 2026-09-26 | **退出零残留内核级兜底（Job Object，3ab41a6）+ 弹窗监控复核**：internal/jobobject 启动早期绑定 Kill-on-Close Job + UI 限制（x/sys 类型安全封装）；实测 taskkill /F 强杀 daemon 后 node/cmd 子进程零残留。热部署后 9 分钟窗口监控（2s 快照对比）零新窗口、daemon 日志零 error/fail、系统日志无窗口异常——「电脑不弹窗+结束零残留」两条红线由 hidewin(进程级)+UILIMIT(内核级)+优雅退出三层共同保证 |
| 2026-10-07 | **弹窗红线监控工具固化**：一次性基线脚本（%TEMP%\pa_gate\winmon.py，EnumWindows+IsWindowVisible+PID+标题）升级为正式运维工具 `tools/ops/window_monitor.py`——纯 ctypes 无子进程（无 wmic 闪窗风险），pythonw 无控制台可跑、结论以退出码为准；快照 UTF-8 JSON（schema `window-monitor-snapshot/1`），diff 按 (PID,标题) 多重集相抵判新增（防同款第二只弹窗被基线首只抵消）；默认过滤空标题与系统壳窗口（Progman/WorkerW，`--all` 保留）。用例 `tools/ops/test_window_monitor.py`（注入窗口清单，不真 EnumWindows） | `--snapshot <file>` 写基线；`--diff <base> [--out <file>]` 报新增，退出码 0=无新增 / 1=有新增 / 2=参数或 IO 错误（判定看全量，不受 `--pid` 展示过滤影响）；`--pid` 仅按窗口属主 PID 过滤，不做子进程树归因（wmic 闪窗禁用、NtQueryInformationProcess 过深，退化路径=全量 diff+拿 PID 人工归因）；定位位=L2/日常「截图目检不弹窗」的机器辅助步骤——目检前后各一帧 snapshot/diff，新窗口 PID+标题给截图以机器证据锚点 |
| 2026-10-13 | **最新-only 落地（0.3.1，不兼容旧格式）**：删 App 侧五处旧 daemon/旧 App 兼容兜底——HostEventTracker sequence≤0 旧分支（`HostEventTrackerTest.legacyZeroSequence…` 改钉拒收，旧行为无独立存在意义，等价性由单调去重用例佐证）；AgentConnection 旧 pair_code 降级分支（主链设备注册不动；Keystore 不可用改明确失败不断旧链）；SettingsScreen optimizerAgents setOf 回退 + AgentDefaults 按名特判（改纯 capability 动态判定；两处旧豁免同步退出 pa_audit roster 基线）；ModelSelectionReducer 旧 daemon 缺席兜底（缺席改抛错不伪造，`ModelSelectionReducerTest` 两旧例改钉缺席抛错、`ConversationUpgradeIntegrationTest` 旧断言段删除）。daemon legacy 单 token 只加观测（Validate 命中记 WARNING + LegacyAuthCount 计数，逻辑暂留，`TestDevicesLegacyTokenFallback` 加计数断言 +1）。附录 A 全表加“删除条件/版本”TTL 列（两处已删标 0.3.1，其余给复核/删除条件）。pa_audit 自测 `test_kotlin_when_branch_baseline_is_warn` 随基线同步改钉未登记即红线 | 本轮删旧兜底用例 4 处（HostEventTracker 1 改钉、ModelSelectionReducer 2 改钉、ConversationUpgrade 1 段删）、新增观测钉子 2 处（reducer 缺席抛错、legacy 计数）；回归用例净 0（改钉对冲）。pa_audit roster 基线 -2（SettingsScreen #3/#5 退出） |
