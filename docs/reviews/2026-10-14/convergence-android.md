# Android 代码收敛报告(2026-10-14)

> **范围**:`android/app/src/main/java/dev/pocketagent/app/**` 与 `android/app/src/test/java/**` 下的 .kt。
> **性质**:纯收敛重构(消重复 / 删死代码 / 抽共享),业务语义零改变;行为级差异一律不改,只记录。
> **提交边界**:本轮**不 commit**(主会话统一提交)。工作区内 `flutter/`、`daemon/` 的未提交改动属他人在途工作,本轮未触碰。

---

## 0. 结论摘要

| 项 | 值 |
|---|---|
| 改动文件 | **24 个**(19 改 / 5 新),全部落在允许的 android 源码目录内 |
| 行数 | 主+测试源 **95741 → 95431 行,净 -310 行**(tracked +69/-481,新增 +102) |
| 测试 | 基线 **1844 用例** → 收敛后 **1844 用例**,每阶段 0 失败 0 错误 0 跳过,196 份 XML 全新鲜 |
| 测试删改 | **零**:没有删除/跳过/弱化任何用例;测试侧改动仅为「重复 helper 定义删除 + 调用点等价改写」 |
| 审计门禁 | `pa_audit --json` 六规则:sheet_overflow / unknown_ui / **fake_green** / proto_sync 全 **pass**;roster warn 为附录A既有挂牌;go_baseline redline 来自工作区他人未提交的 daemon 改动(见 §3.3),与本轮无关 |
| 等价论证 | 每个收敛点见 §2;所有被合并副本在删除前做了**逐字节相同断言** |

---

## 1. 盘点清单(改前基线:354 个 .kt / 95741 实际换行)

扫描方法:自写 Python 脚本三类 —— ①精确重复块(滑窗哈希);②零引用符号(全仓 main+test grep 复核);③样板模式计数(正则)。

| # | 家族 | 规模(改前) | 处置 |
|---|---|---|---|
| 1 | 死代码(私有/零引用符号) | 7 个符号 / 163 行 | **A 删除** |
| 2 | ThreadsScreen `PcSessionCard`/`PcAgentSessionCard` 双份行卡 | 141 行区(两函数仅 4 行差异) | **B 合一** |
| 3 | 测试 fixture 重复:`locate`×3、`section`×2、`msg`×2、`msgFile`×3、`task`×2 | 12 处定义 | **C 收敛到 4 个 fixture 文件** |
| 4 | 源码定位器 walk-up(`user.dir` 向上 6 层) | 同构实现 **4 份**(SourceKit 之外 3 份) | **C+ 收敛到 `SourceKit.locate`** |
| 5 | Usage↔JSON 序列化双份(`ThreadUsageStore` vs 同包顶层 `usageToJson/usageFromJson`) | 各 1 份完整 builder 链 | **D 委托单点** |
| 6 | 新建会话请求构造+RPC 样板 | **3 份**(Home/Threads/Chat 三入口) | **E 收敛到 `createThreadRpc`** |
| 7 | `formatTime` 双份 | 2 处 / 2 文件 | 未动(§4-1:Locale 不同) |
| 8 | 相对时间族(`threadRowTime`/`relativeTime*` 等) | 5 处 / 3 文件 | 未动(§4-2:口径不同) |
| 9 | `fun init(ctx: Context)` 幂等样板 | 9 处 / 8 文件 | 未动(§4-3) |
| 10 | `"(未命名…"` 文案字面量 | 12 处 / 6 文件 | 未动(§4-4) |
| 11 | gRPC 样板:`withContext(Dispatchers.IO)` 123 处/19 文件、`service()` 调用 114 处/20 文件 | 横切面大 | 未动(仅 E 收敛「新建会话」一处,§4-5) |
| 12 | Compose 状态样板:`LaunchedEffect` 115 处、`collectAsState` 64 处 | 框架惯用式 | 未动(§4-6) |
| 13 | 测试 `fragment()` 与 `SourceKit.section()` 近重复 | 1 处 | 未动(§4-7:搜索起点/断言文案不同) |
| 14 | `ThreadUsageStore` ↔ `UsageCache` 存储层合并 | 已知技术债 | 未动(§4-8,D 仅收敛序列化) |
| 15 | 三入口 `NewThreadSheet` 行为差异(Threads 不套 `createThreadAgentId`) | 行为差异 | 不改(§4-9,E 用参数**显式化**差异) |

---

## 2. 逐项收敛记录(改动 → 等价论证 → 证据)

### A. 死代码删除(3 个主文件,-163 行)

| 符号 | 位置 | 行数 | 删除依据 |
|---|---|---|---|
| `ExportChip` | ChatScreen.kt | 13 | 全仓 main+test 零引用 |
| `internal fun relativeTime` | ChatScreen.kt | 18 | 按**包**判定零引用(其他包存在同名符号,已排除误伤) |
| `SubHeader` | RuntimeSection.kt | 11 | 零引用 |
| `sectionErrorPing` | RuntimeSection.kt | 5 | 零引用 |
| `effortLabel` | SettingsScreen.kt | (随分区) | 仅被已死的 Pi/ZCode 分区引用 |
| `PiStatusSection` / `ZCodeStatusSection` | SettingsScreen.kt | 116(合计) | 设置页实际渲染路径已改为 `AgentDefaultsSection`(按 capability 动态判定),旧「按名特判分区」零引用 |

**等价论证**:删除的是不可达代码(编译单元内无任何调用点),不改变任何可达路径的字节码语义。
**验收**:三文件 UTF-8 strict 解码 / 无 BOM / LF 保持;`git diff --stat` = 3 files, 163 deletions;测试 18:45:30 **1844/0/0/0**。

### B. ThreadsScreen 电脑行卡合一(`PcSessionRow`,-37 行)

- 改动:`PcSessionCard`(codex)+ `PcAgentSessionCard`(Pi/ZCode)两个 66 行函数体 → 一个共享 `PcSessionRow(badge, metaLabel, title, updatedAtUnixMs, workspace, importing)` + 两个 13 行薄委托。文件 2837 → 2800 行。
- **等价论证**:`difflib` 比对两函数体**仅 4 行差异**(且 1 行是注释措辞):①`agentBadgeRes("")` vs `agentBadgeRes(info.agentId)` → 参数 `badge`;②注释;③meta 首项 `"codex 会话"` vs `threadAgentIdOf(info.agentId) + " 会话"` → 参数 `metaLabel`;④字段来源 `info.title/updatedAtUnixMs/cwd` 在两类型上是同名同类型字段 → 同名参数。共享体由原体逐行推导,替换后其余行逐字节不变。
- **过程记录**:首次脚本 region 边界算错(截断函数中段)→ 编译红 → `git checkout -- ThreadsScreen.kt` **精确回滚该单项**(Phase A 成果不受影响)→ 修正版用 3 个锚点断言(签名行/结束行/空行)重做成功。证明单项回滚机制有效。
- 测试 18:58:08 **1844/0/0/0**。

### C. 测试 fixture 收敛(10 改 4 新,-54 行)

新建 4 个 fixture 文件,删除 12 处重复定义:

| helper | 原位置 | 新家 | 收敛前 |
|---|---|---|---|
| `locate(vararg)` | PaSheetConvergence / PaSheetDialogConvergence / SheetScrollOverflow | `ui/SourceKit.kt`(internal) | 3 份×13 行,**字节相同** |
| `section(text,from,to)` | PaSheetDialogConvergence / SheetScrollOverflow | `ui/SourceKit.kt` | 2 份×8 行,字节相同(KDoc 措辞略异,取一份) |
| `msg(id,role,text)` | ChatViewModelHistoryFirstOpenTest(顶层) / …LandingTest(成员) | `ui/chat/ChatHistoryFixtures.kt` | 2 份,body 去缩进后字节相同 |
| `msgFile(threadId)` | ThreadSnapshotTest / ThreadSnapshotO1Test / CachePartitionsTest | `ui/component/SnapshotMsgFileFixture.kt`,签名改为 `msgFile(dir, threadId)` | 3 份 body 同算法,目录字段不同(`dir`/`dir`/`snapDir`) |
| `task(id,kind,status)` | EditorialListWaveTest / TaskPartitionOfflineTest | `ui/TaskCenterFixtures.kt` | 2 份×7 行,字节相同 |

- **等价论证**:
  - 合并前脚本对每个「被合并副本组」执行 **`body`(sig..end)逐字节相同断言**(日志 `ASSERT all merged copies byte-identical`),任何一组不一致即中止不写盘;
  - `msgFile` 三份唯一差异是目录变量 → 参数化为 `dir`:调用点在 `@Test` 内、`dir`/`snapDir` 均由 `@Before` 初始化,取值与原成员内 `this.dir`/`this.snapDir` **同一对象**,12 个调用点机械改写(`msgFile("a")` → `msgFile(dir, "a")` 等),写文件路径逐字节不变;
  - 可见性 `private 成员` → `internal 顶层`(同包),调用点语句除上述参数外零改动;无断言/无用例被修改。
- 测试 19:28:01 **1844/0/0/0**。

### C+. 源码定位器 4 → 1(3 改,-39 行)

- 同族 `user.dir` 向上 6 层定位器还有三份:`ChatInputTouchTargetTest.locateChatScreen`(硬编码 rel)、`FirstTaskFunnelFixTest.locateSource(rel)`、`NewThreadSheetScrollRegressionTest.locateThreadsScreen`(硬编码 rel)。全部删除,5 个调用点改写为 `locate("…").readText()`(现 7 个测试文件共 18 个调用点单点走 `SourceKit.locate`)。
- **等价论证**:三份实现与 `SourceKit.locate` **算法逐行同构**——起点 `File(user.dir).absoluteFile`、同 6 层、每层同序检查 `[cur/rel, cur/android/app/rel]`、同 `dir = cur.parentFile` 递进、同 `?: return@repeat` 守卫。唯一差异是**全部未命中时**的 `error()` 文案(仍抛异常、测试同样红,仅失败消息措辞不同)。
- 测试 19:52:30 **1844/0/0/0**。

### D. 用量 JSON 序列化单点化(ThreadUsageStore,-14 行)

- 改动:删除 `ThreadUsageStore.usageToJson`(String 版)与 `jsonToUsage`(完整 builder 链),改为:
  - 写入:`usageToJson(turn)?.let { obj.put("turn", it.toString()) }` —— 复用同包顶层 `usageToJson(u): JSONObject?`;
  - 读取:新私有 `parseUsage(raw)` = 空判 + `try { usageFromJson(org.json.JSONObject(raw)) } catch { null }`,替代原 `jsonToUsage`。
- **等价论证**:
  - **字节格式不变**:顶层 `usageToJson` 与被删私有版键序(in/out/cr/cw/cost/reason)、取值完全一致,`toString()` 在同一构造上执行 → 落盘字符串逐字节相同;`put(key, String)` 类型不变(没有改成嵌套 JSONObject,规避 `optString` 语义变化);
  - **语义分层保持**:原 `jsonToUsage` 的内层 catch 使**坏片段只置空该字段、整份 load 仍返回 Snapshot**(total 不丢);若直接内联到 `load` 的外层 try,坏片段会拖垮整份快照 —— 故保留独立内层兜底,该差异点被显式守住;空/缺失串 → null 与原一致;
  - **防递归**:同名私有 `usageToJson` 已删除(否则成员遮蔽顶层导致自递归),经 grep 确认全仓无 `jsonToUsage` 残留引用。
- 测试 19:31:53 **1844/0/0/0**。

### E. 新建会话 RPC 三入口合一(`NewThreadRpc.kt`,样板 71 → 29 行,净 -3 行)

- 改动:三处 `NewThreadSheet.onCreate` 内的「构造 `CreateThreadRequest` + `withContext(IO) RPC`」样板(25/21/25 行)收敛为共享 `createThreadRpc(ws, applyAgentSelection)`(新文件 39 行,含 10 行 KDoc);入口各剩 9/12/8 行。
- **等价论证**(逐入口):
  - **构造段逐行搬移**:builder 初始 `setAgentId(selectedAgent)`、`selectedModel` 非空才 `setModelId`、`ws.isNotBlank()` 才 `setWorkspace` —— 条件与顺序原样;`createThreadAgentId` 映射块以 `applyAgentSelection` 开关:**Home/Chat 传 true(原样保留),Threads 传 false(原样跳过)**,把三入口既有的行为差异**参数显式化**,而不是抹平;
  - **返回值改形**:`val t = …createThread(…)` + `onOpenThread(t.id)` → `val t = createThreadRpc(…)`(helper 内取 `.id`)+ `onOpenThread(t)`,同一 Thread 对象的同一 `id` 字段;
  - **留在入口的东西逐字节未动**:停用闸 `newThreadAgentAllowed`(含 Home 用局部 `disabledAgents`、另两处用 `AgentConnection.disabledAgents` 的既有差异)、`scope.launch`/`try/catch`、失败 Toast(Threads 的 `?: "连接失败"` 回退文案、Home/Chat 的单行 Toast)、Home 独有的 `PaHaptics.confirm(view)` —— 脚本从原文件**逐字节提取复用**这些行,不做任何转写(防中文抄错);
  - helper 内的 P1 注释把原先 Home/Chat 两份不同措辞的注释统一为一份(注释非行为)。
- **扫描钉子规避核验**:改动前核对了所有源码扫描型测试的 marker——`FirstTaskFunnelFixTest` 扫 HomeScreen 的 `manualAdd`/`bottomNote` 区与 ThreadsScreen 的 `fun NewThreadSheet(` **声明体**、`SheetScrollOverflowRegressionTest` 的 section 边界(`organizeTarget?.let` 起 / `fun ManageAgentsSheet(`~`// ---- 新建会话的 agent 接线` 止)、`NewThreadSheetScrollRegressionTest` 扫声明体 —— 均在改动区之外,断言全部保留。
- 测试 19:45:44 **1844/0/0/0**。

---

## 3. 测试证据

### 3.1 全量测试链(每阶段后复跑,数字不缩水)

命令(均在 `G:\项目1\pocketagent` 下,中文路径自动镜像 `G:\pa`):

```text
python tools/device/_pa_gradle.py test      → gradle rc=0, BUILD SUCCESSFUL
python tools/device/_pa_test_report.py      → XML 份数 / tests / failures / errors / skipped / 新鲜度
```

| 时点 | 阶段 | gradle | XML 份数 | tests | failures | errors | skipped | XML 时间(最新=最旧,全量 --rerun) |
|---|---|---|---|---|---|---|---|---|
| 18:11:42 | 基线(改前) | — | 196 | **1844** | 0 | 0 | 0 | 2026-10-14 18:11:42 |
| 18:45:30 | A 死代码后 | rc=0 (93.6s) | 196 | **1844** | 0 | 0 | 0 | 2026-10-14 18:45:30 |
| 18:58:08 | B 行卡合一后 | rc=0 (74.8s) | 196 | **1844** | 0 | 0 | 0 | 2026-10-14 18:58:08 |
| 19:28:01 | C fixture 后 | rc=0 (44.8s) | 196 | **1844** | 0 | 0 | 0 | 2026-10-14 19:28:01 |
| 19:31:53 | D 序列化后 | rc=0 (45.5s) | 196 | **1844** | 0 | 0 | 0 | 2026-10-14 19:31:53 |
| 19:45:44 | E RPC 后 | rc=0 (75.4s) | 196 | **1844** | 0 | 0 | 0 | 2026-10-14 19:45:44 |
| 19:52:30 | C+ 定位器后(最终) | rc=0 (40.1s) | 196 | **1844** | 0 | 0 | 0 | 2026-10-14 19:52:30 |

`_pa_test_report.py` 最终输出原文:

```text
XML 份数      : 196
用例总数 tests: 1844
失败 failures : 0
错误 errors   : 0
跳过 skipped  : 0
最新 XML 时间 : 2026-10-14 19:52:30 (距今 0.3 分钟)
最旧 XML 时间 : 2026-10-14 19:52:30
新鲜度核验    : 全部 XML 均在 10 分钟内
```

> 中间有一次**预期内的红**:Phase B 首版脚本 region 边界算错,`compileDebugKotlin` 语法错误 → `git checkout -- ThreadsScreen.kt` 单项回滚 → 锚点断言修正版重做 → 转绿。除此之外全程绿。

### 3.2 字节级验收(24 个改动/新增文件)

UTF-8 strict 解码 ✓ / 无 BOM ✓ / 无裸 CR(换行风格按文件保留)✓ —— `files audited: 24, bad: 0`。

### 3.3 `pa_audit --json`(2026-10-14 19:54:34)

- **pass×4**:`sheet_overflow`、`unknown_ui`、`fake_green`(测试 XML 新鲜度与计数自证)、`proto_sync`(未碰 proto);
- **warn×2**:`roster`(附录A既有挂牌/豁免,只减不增)、`go_baseline` 的 vet 噪声(README §11 存量);
- **redline×1**:`go build ./...` 失败于 `daemon/internal/agent/text_generator.go:111 undefined: log` —— 来源是**工作区他人未提交的 daemon 在途改动**(`git diff --stat -- daemon/` 显示 `text_generator.go +63` 等 7 文件改动、4 个未跟踪新文件),本轮未触碰 daemon/,不属本报告范围。

---

## 4. 已识别未动(行为敏感 / 风险 > 收益)

1. **`formatTime` ×2 不合并**:`ThreadsRowLogic.kt`(顶层)用 `Locale.getDefault()`,`HistoryQuickNavLogic.kt`(类成员)用 `Locale.CHINESE` —— Locale 参数不同 = 设备语言相关格式化行为**不等价**;且一个是顶层私有、一个是类成员,合并需改可见性。非等价不动。
2. **相对时间族 5 处/3 文件**:`threadRowTime`(列表行口径)、`relativeTimeLabel`、抽屉/主页各自口径 —— 时间档位与空值策略不同,合并会改文案行为。
3. **`init(ctx: Context)` 幂等样板 9 处/8 文件**:存储对象各自持 prefs,样板本身 3 行,抽象收益低于可读性损失;合并涉及存储初始化顺序,超出纯收敛。
4. **`"(未命名…"` 字面量 12 处/6 文件**:散落在不同文案句式中(「(未命名会话)」「(未命名)」等),抽常量需先做文案口径核对,涉用户可见文案,留待文案轮。
5. **gRPC/IO 样板(123 处 `withContext(IO)`、114 处 `service()` 调用)**:横切 19/20 文件,泛化封装(如统一 `rpc { }`)会改异常传播/上下文语义,风险大;本轮只收敛语义完全同构的「新建会话」一处(E)。
6. **Compose 样板(`LaunchedEffect` 115、`collectAsState` 64)**:框架惯用式,收敛反而降低可读性。
7. **测试 `fragment()` vs `SourceKit.section()`**:近重复但**非等价** —— `fragment` 从 `start + from.length` 起找终点锚点(防止 from/to 前缀重叠)且断言文案不同;强并会改边界行为。保留。
8. **`ThreadUsageStore` ↔ `UsageCache` 存储合并**:`UsageCache.kt` 头注释明确标注「已知技术债,本轮不合并」(字段高度重叠但合并=共享存储重构,多人在改窗口风险大于收益)。D 阶段只把**序列化**收敛到单点,存储键(`pa_thread_usage` vs `pa_usage_card_cache`)与结构未动;后续方向按其头注释执行。
9. **三入口 `NewThreadSheet` 的 `createThreadAgentId` 行为差异**:ThreadsScreen 入口不套选中映射(Home/Chat 套),疑似历史遗留。属**行为差异**,本轮不改;E 用 `applyAgentSelection=false` 把差异**显式固定**并写进 KDoc,后续如裁定统一,只需改一个参数并补一轮行为核验。

---

## 5. 改动文件清单

| 文件 | 阶段 | + | - |
|---|---|---:|---:|
| main/ui/ChatScreen.kt | A, E | 2 | 50 |
| main/ui/HomeScreen.kt | E | 2 | 18 |
| main/ui/RuntimeSection.kt | A | 0 | 16 |
| main/ui/SettingsScreen.kt | A | 0 | 116 |
| main/ui/ThreadsScreen.kt | B, E | 37 | 83 |
| main/ui/component/ThreadUsageStore.kt | D | 11 | 25 |
| **main/ui/NewThreadRpc.kt(新)** | E | 39 | — |
| test/ui/PaSheetConvergenceTest.kt | C | 0 | 14 |
| test/ui/PaSheetDialogConvergenceTest.kt | C | 0 | 23 |
| test/ui/SheetScrollOverflowRegressionTest.kt | C | 0 | 23 |
| test/ui/EditorialListWaveTest.kt | C | 0 | 8 |
| test/ui/TaskPartitionOfflineTest.kt | C | 0 | 8 |
| test/ui/chat/ChatViewModelHistoryFirstOpenTest.kt | C | 0 | 9 |
| test/ui/chat/ChatViewModelHistoryLandingTest.kt | C | 0 | 8 |
| test/ui/component/ThreadSnapshotTest.kt | C | 2 | 10 |
| test/ui/component/ThreadSnapshotO1Test.kt | C | 8 | 16 |
| test/ui/component/CachePartitionsTest.kt | C | 2 | 10 |
| test/ui/ChatInputTouchTargetTest.kt | C+ | 1 | 16 |
| test/ui/FirstTaskFunnelFixTest.kt | C+ | 3 | 15 |
| test/ui/NewThreadSheetScrollRegressionTest.kt | C+ | 1 | 13 |
| **test/ui/SourceKit.kt(新)** | C | 27 | — |
| **test/ui/TaskCenterFixtures.kt(新)** | C | 11 | — |
| **test/ui/chat/ChatHistoryFixtures.kt(新)** | C | 14 | — |
| **test/ui/component/SnapshotMsgFileFixture.kt(新)** | C | 11 | — |
| **合计** | | **+69 (tracked) +102 (新)** | **-481** |

净变化:**-310 行**;分阶段:-163(A)、-37(B)、-54(C)、-39(C+)、-14(D)、-3(E,调用点样板 71→29 换 39 行共享 helper),合计恰为 -310。

---

## 6. 方法、工具与边界

- **扫描/编辑脚本**(均存于 `C:\Users\yanzi\AppData\Local\Temp\opencode\`):`dup_scan/dead_scan/pattern_scan`(盘点)、`cut.py`(字节级行删)、`phaseB.py/phaseB2.py`(区域重构)、`phaseC.py/phaseC2.py`(fixture 收敛,含副本逐字节断言)、`phaseE.py`(RPC 收敛,原行逐字节复用)。所有写盘均带:UTF-8 strict 回解、无 BOM 断言、换行风格保留、锚点唯一性断言、写后内容断言。
- **等价优先纪律**:只做「可论证逐字节/逐语义等价」的改动;凡涉及文案、Locale、边界条件、异常传播层级有差异的一律进 §4 不动清单。
- **git 边界**:`git diff --name-only HEAD` 中属于本轮的只有 android 下 19+5 个文件;`flutter/`(主题/页面在途改动)与 `daemon/`(hermes/dsh 等在途改动)保持原样;**未执行任何 `git add/commit/push`**。
- **源码钉子依赖**:改动前逐一核对源码扫描型测试的 marker(`fun NewThreadSheet(`、`manualAdd`/`bottomNote`、`organizeTarget?.let`、`fun ManageAgentsSheet(`、`// ---- 新建会话的 agent 接线` 等),删除/重构区域均避开;所有 marker 最终测试仍全绿。
- **不碰红线**:ui/theme 五套主题、AgentConnection/安全层、proto 双副本、`agentCapableOf` 语义,本轮零触碰。
