# daemon agent 运行时核心域 整洁审查（clean-daemon-agent）

- **日期**：2026-10-17
- **范围**：`daemon/internal/agent/`（206 文件 / 1.9 MB，其中生产代码 61 文件 / 958 KB，测试 145 文件 / 1.0 MB）+ `daemon/internal/runtime/`（49 文件 / 423 KB，生产 27 文件）
- **明确不审**：`server/`、`cmd/`、各 `*app/`、`store/`、`selfguard/`、`gen/`、`eventstore/`、`hostenv/`、`rotatelog/`、小包、daemon 测试面（另两条 daemon 线）
- **审查人**：subagent（另一会话派生）
- **方法**：只读。`go vet` / `go test` 基线 + 自建 Go 词法分析脚本（`golex.py`，含注释/字符串/字符字面量剥离 + 花括号配对）落在 `C:\Users\yanzi\AppData\Local\Temp\opencode\`，**未在仓库留任何脚本/产物**
- **红线声明**：本报告**未改动任何代码/文件，未 git add/commit/push，未跑 `gofmt -w`**。涉及「能力位生成与其位族词汇表」（`codex_capabilities.go` / `capabilities_cache.go` / `runtime_descriptors.go` / `runtime/knowledge_*.go` / `event_translate.go` 词表）一律**只标注不建议改**。

---

## 0. 结论速览

| 面 | 状态 | 一句话 |
|---|---|---|
| **绝对红线 1：绝不替用户审批** | ✅ **干净（0 违反）** | 四条审批路径（codex/dsh/hermes/zcode）逐条对账：通道缺失→error、通道故障→error 非 denied、超预算→error 非 denied、选项表认不出→**显式报错拒绝代答**。无任何 auto-approve/default-allow 落面 |
| **绝对红线 2：绝不改用户桌面配置** | ⚠️ **P0 待裁决 ×1** | `config.toml` 一律不写（红线本体守住）；但 `DeleteCodexSession` 会 `os.Remove(~/.codex/sessions/rollout-*.jsonl)` —— **对红线目录的写操作**，见 §2 P0-1 |
| 适配器一致性（5 家 × 24 维矩阵） | ⚠️ **P1×1 + P2×2** | 共享骨架已收编得相当彻底；唯一实质缺口是 `ErrorInfoCarrier` 3/5 落面且无编译期钉子 |
| 命名 | ⚠️ P2×3 | 噪声词 7 处（轻）；`clip` 一名两义 + 参数遮蔽；无 `getX` 前缀、无谎言 `isX`/`newX` |
| 函数与结构 | ⚠️ **P1×1** + P2×3 | `CodexProvider` 42 字段 / 8 把互斥锁 / 2382 行；`handleCodexApproval` 11 参数；`Run` 708 行 |
| 注释 | ⚠️ **P1×4** | 谎言注释 3 处（其中 1 处量化值写错且被另外三家当对齐基准）、悬空文档引用 1 处。TODO/FIXME **生产 0 条** |
| 错误处理 | ⚠️ P1×1 + P2×3 | 哨兵错误用 `==` 而非 `errors.Is`（4 处，含「首次拉起」关键分支）；`fmt.Errorf` 建哨兵 4 处；error 首字母大写 23 处 |
| 并发与生命周期 | ✅ **干净** | goroutine 无泄漏路径、`time.Ticker` 4/4 有 `defer Stop()`、无 `exec.Cmd`、共享 map 访问 0 处越锁 |
| 死代码 | ✅ **干净（0）** | 双向计数（生产引用 ≤1 且包内引用 ≤1）：**agent 0 / runtime 0** |
| 测试整洁 | ⚠️ P1×1 + P2×2 | 已证等价的 responder 假件重复；断言无消息率仅 **1%**；条件跳过仅 4 处且全为具名 env 开关 |

**P0 = 1（红线 2 疑似违反，需人工裁决） / P1 = 8 / P2 = 13**

没有「已造成维护事故」的 P0。P0-1 是**语义边界问题**（红线原文是「目录一律只读」，代码做的是「用户主动删会话时删掉该会话的 rollout」），必须由人裁决，本审查不下结论。

---

## 1. 基线（只读，未修）

```
$ go vet ./internal/agent/... ./internal/runtime/...
（无输出 = 干净）

$ go test ./internal/agent/... ./internal/runtime/...
ok  	github.com/pocketagent/daemon/internal/agent   	18.055s
ok  	github.com/pocketagent/daemon/internal/runtime 	2.932s
```

**工具陷阱（记入「已知工具局限」§7）**：用 ASCII junction（`G:\pa_c1b` → `daemon`）跑同一命令会得到 **5 条 FAIL**，全部是相对路径 / 开发机绝对路径依赖：
`TestCapabilityBitVocabularyMatchesMatrix`（找 `../../../docs/2026-09-30-agent-capability-matrix.md`）、`TestCodexGoalSetShapeMatchesSchema`、`TestGoalNotificationShapeMatchesSchema`、`TestCodexReviewStartShapeMatchesSchema`、`TestCodexNoticeRegistryMatchesVendoredSchema`（后三个找 `G:\codex-schema\**`）。**这不是代码缺陷，是测试对工作目录的隐含假设**；报基线必须在真实中文路径下跑。

---

## 2. 分级债清单

### P0-1｜`DeleteCodexSession` 对用户桌面目录做删除（红线 2 疑似违反，**需人工裁决**）

**证据链**（完整可达，无名单分支）：

```
手机端 DeleteThread
  → server/thread.go:373-374   if d, ok := s.cfg.Providers[id].(nativeSessionDeleter); ok { d.DeleteNativeSession(t) }
  → agent/runtime_descriptors.go:259-271  (*CodexProvider).DeleteNativeSession
  → agent/runtime_descriptors.go:264      DeleteCodexSession(t.CodexThreadID)
  → agent/codexsessions.go:150            os.Remove(path)          // path ∈ ~/.codex/sessions/**/rollout-*.jsonl
  → agent/codexsessions.go:41-43          CodexSessionsDir() = os.UserHomeDir()/.codex/sessions
```

**两种读法，都成立**：

- 读法 A（**倾向不违规**）：红线原文「绝不改写用户桌面**配置**」，指的是 `config.toml` 之类会影响 agent 行为的配置。会话 rollout 是会话产物而非配置；删除由**用户主动点击**触发；代码对 `config.toml` 的封守非常彻底（`codex.go:116` / `codex.go:1152` / `codex.go:1909` / `codex.go:1918` 四处显式声明「红线永久封死」「绝不写」，并且真的一个字都没写）。
- 读法 B（**倾向违规**）：红线原文写的是「`~/.codex/`、`~/.dsh/`、`~/.pi/` 等**一律只读**」。按目录字面读，删除 rollout 就是对该目录的写。

**同时发现一个真实的、无论怎么裁决都该修的守卫不对称**（本条的硬证据）：

| | pi | codex |
|---|---|---|
| 路径 containment 校验 | ✅ `pi_delete.go:20-33` `EvalSymlinks` + `filepath.Rel` 前缀判定 | ❌ 无 |
| 扩展名白名单 | ✅ `pi_delete.go:35` `.jsonl` | ❌ 无（靠 `FindCodexSession` 的 `HasSuffix(name,"-"+id+".jsonl")` 隐式约束） |
| 会话身份回读校验 | ✅ `pi_delete.go:38-41` 文件头 id 必须等于传入 id | ❌ 无 |
| 匹配多重命中 | 单次精确 | ❌ `codexsessions.go:160-168` 取**第一个**命中即 `hit != ""` 短路 |
| 幂等处理 | ✅ | ✅ `codexsessions.go:150` `!os.IsNotExist` |

同类操作（删用户会话文件），pi 4 道闸、codex 1 道。

- **收法（守卫对齐，与裁决无关，建议先做）**：把 pi 的三道闸搬成共享 helper `safeNativeSessionDelete(root, path, expectIDFn)`，codex 侧在 `os.Remove` 前补 containment + 单一命中检查；`FindCodexSession` 改成收集全部命中，>1 时**报错而不是挑一个删**。
- **修复代价**：~30 行（helper + 两处调用点改写）。
- **修复风险**：**零**。纯 agent 域、不碰甲级禁区、不碰红线（只会更严）、不改协议、不改 proto。现有 `codex_approval_test.go`/`zcode_approval_test.go` 同级测试可照抄模式。
- **需人裁决的点**：「删用户自己刚在手机上删掉的会话」这条语义要不要继续存在。若裁决为「保留」，建议把红线原文从「目录一律只读」收敛为「配置只读；会话产物仅在用户显式删除该会话时可删」——**这是文档层裁决，不是代码层**。

---

### P1-1｜`ErrorInfoCarrier`：5 家里 3 家落面，且**没有编译期钉子**（适配器一致性 · 本轮头号发现）

**证据**：

| provider | 结构化错误类型 | `ErrorInfo() *pb.Error` |
|---|---|---|
| codex | `codexRunError` | ✅ `agent/codex_events.go:36` |
| pi | `piError` | ✅ `agent/pi.go:2054` |
| dsh | `dshRunError` | ✅ `agent/dsh.go:1778`（+ `:121` 有编译期断言 `_ ErrorInfoCarrier = (*dshRunError)(nil)`） |
| **zcode** | — | ❌ 无 |
| **hermes** | — | ❌ 无 |

消费点是**类型断言**，不是 Provider 接口方法：`server/thread.go:1511` `if carrier, ok := runErr.(agent.ErrorInfoCarrier); ok { ... }`。断言失败就静默降级成裸字符串。

**为什么这特别值得修**：zcode 恰恰是失败形态最结构化的一家 —— 有 `ErrZcodeBusy`（`zcode.go:383`）、有看门狗静默（`zcode.go:2140`）、有启动闪退重试（`zcode.go:428-439`）、有锁等待窗口（`zcode.go:376`）。这些正是「该重试 / 该稍后再试 / 该换配置」三类语义最需要结构化透出的场景，现在全被压成字符串。

**收法（照仓库已有先例，两步，都很小）**：
1. 在 `runtime_descriptors.go` 或 `provider.go` 加一条编译期断言块，把 `var _ ErrorInfoCarrier = (*ZcodeRunError)(nil)` / `(*HermesRunError)(nil)` 钉住 —— 形态完全照 `provider.go:112-118`（`PrefsAware` 五家名单）与 `image_input_capability.go:49-55`（`ImageInputter` 五家名单）。**这一步零行为变化，纯把「靠自觉」变成「靠编译期」**。
2. 各自补一个最小错误类型：把 zcode 的 `ErrZcodeBusy` / 看门狗静默 / 启动闪退三类失败包装进去，`ErrorInfo()` 填 code + retryable。dsh 的 `dshRunError`（`dsh.go:1771-1800`）是现成模板。

- **修复代价**：断言块 ~6 行；zcode 错误类型 ~40 行；hermes ~30 行。
- **修复风险**：**低**。不碰 proto（`pb.Error` 已存在）、不碰 `server/`、不碰能力位。第二步会让 zcode/hermes 的 `TurnFailed` 帧多出结构化字段 —— 需同步确认 Android/Flutter 端对该字段是**已有渲染路径**还是会被当未知字段（跨域线已确认 `pb.Error` 是既有契约，风险很小，但改前应与跨域结论对一次账）。

---

### P1-2｜量化谎言注释：codex 看门狗阈值注释写 90 秒，实到 150 秒 —— 而另外三家都拿 codex 当基准

- `agent/codex.go:1738`（注释）：`// (无报错、无事件),90 秒收不到任何通知就判失败,让手机端看到明确报错。`
- `agent/codex.go:1763`（实到）：`if ms := lastNotif.Load(); ms >= 0 && time.Since(turnStartRef).Milliseconds()-ms > 150_000 {`
- `agent/codex.go:1769/1771`（错误文案）：`"codex 150 秒无任何事件…"`

即：**注释说 90s，代码与文案都是 150s**。

放大伤害的是另外三家的注释：
- `agent/dsh.go:53` `dshSilenceWatchdog = 150 * time.Second // 无任何事件的看门狗(对齐 codex)`
- `agent/hermes.go:604` `hermesSilenceWatchdog = 150 * time.Second // …(对齐 codex/dsh)`
- `agent/zcode.go:65` `zcodeWatchdogSilence = 150 * time.Second`（同值）

**三家写「对齐 codex」，而 codex 自己注释说 90 秒** —— 基准源与对齐源互相矛盾。下一个人调 codex 阈值时按注释改成 90s，四家立刻漂移。

- **收法**（两选一，推荐 A）：
  - **A（顺手补上具名常量）**：提 `codexSilenceWatchdog = 150 * time.Second` / `codexWatchdogTick = 10 * time.Second` / `codexTurnCap = 60 * time.Minute` 三个常量（`codex.go:1739` `10 * time.Second`、`codex.go:1763` `150_000`、`codex.go:1774` `60*time.Minute` 现在全是裸字面量，是五家里唯一一家没有具名看门狗常量的），四处使用点替换，注释同步。这样 codex 才真正配得上被另外三家「对齐」。
  - **B（最小）**：只把 `codex.go:1738` 的「90 秒」改成「150 秒」。1 个字。
- **修复代价**：A ≈ 10 行；B = 1 行。
- **修复风险**：**零**。纯常量提取，行为逐字节不变；`go test` 全绿可自证。

---

### P1-3｜谎言注释（可证伪）：`nativeNoticeRecorder.record` 声称 pi/zcode/dsh 还在用，生产调用点 0

- `agent/native_notifications.go:395-397`：
  ```
  // record 是"未分类"入口:不关心分档的调用方(pi / zcode / dsh 等)继续用它,
  // 内部回落 UNKNOWN_NOTIFICATION —— 分类先从 codex item 路径落地,不让其余
  // runtime 的调用点为此被迫改动。
  ```
- 实到：生产代码里 `notices.record(` **0 命中**（全部 30 个生产调用点已迁到 `recordAs`，见 `dsh.go:793/1310/1332/…/1581`、`codex.go:891/1559`、`pi.go:1149`、`zcode.go:1951/1956/1987/2045/2084`、`hermes.go:781`）。
- **仓库自己已经知道这件事**：`agent/notification_kinds_test.go:139` 明写「(`grep -rn "notices\.record(" daemon/internal/agent/*.go` 应为 0 命中)」。而 `agent/native_notifications_test.go:180` 的用例名注释又重复了一遍错误说法（「钉住"既有调用方零成本":pi/zcode/dsh 等」）。

即：注释 + 一条测试注释 + 一条用例注释，三处一致地说谎，且测试注释自己就是反证。

- **收法**：两条路，推荐前者 ——
  - **A（删）**：`record` 生产无引用，是纯测试存活（见 §6）。删函数（6 行）+ 改 3 处注释；`unknown_events_index_test.go:22/51` 与 `native_notifications_test.go:51` 改调 `recordAs(kind, …)`。净减。
  - **B（留）**：改注释为「当前生产调用点已全部迁到 `recordAs`，此入口仅测试与将来不分档的新调用方使用」。
- **修复代价**：A ≈ 20 行（含 3 个测试点）。
- **修复风险**：**零**（纯注释 + 测试桩改名，`go test` 可自证）。

---

### P1-4｜陈旧占位注释：zcode「注册点占位、主模型合并时接入」，而它早已接入

`agent/provider.go:195-196`：

```
// 【zcode 注册点占位】zcode provider 由 T3 智能体在此处并列注册
// (internal/agent/zcode*.go),主模型合并时接入。
```

- 这两行挂在 **pi 注册块内部**（`:197-204` 是 pi 的 `LocatePi` 分支），位置本身就错。
- 实到：同文件 `:208` `RegisterZcode(m)` 已落面，`zcode.go` 早已是完整 provider（有 Run/Steer/会话/子代理/选择面全套）。
- 属「变更日志式注释」：记录了一个已经结束的历史状态，且没人清理。

- **收法**：删这 2 行。
- **修复代价**：2 行。**修复风险**：零。

---

### P1-5｜悬空设计文档引用：`dsh.go` 文件头「事实来源」指向全仓不存在的文件

- `agent/dsh.go:5`：`//   - 设计蓝本:2026-09-20-dsh-integration-plan.md §4/§5(写码时逐项对标 §10.A 参考实现对照表);`
- **全仓递归查（`docs/`、`daemon/`、`.aoci/` 全部）找不到 `2026-09-20-dsh-integration-plan.md`。** 现存的 dsh 相关文档是 `docs/verify/evidence/dsh-p0-probe.md`（同段落 `:7` 已经引了）、`docs/reviews/2026-10-14/dsh-hermes-capability-probe.md` 等。
- 该引用被文件头「本 Provider 明确不做的事（蓝本 §4.3）」等 **10 处**间接依赖（`dsh.go:10,13,874` + `dsh_plan.go:5` 等）。一份 1819 行文件的「事实来源」指向空气，是本域最容易被后来者误引的一处。

- **收法**：把 `:5` 改成现存文档路径，或整条删掉（`:7` 的 probe 文档已经承重了）。
- **修复代价**：1 行。**修复风险**：零。
- **同类（更轻）**：`agent/pi_fork_plan.md` 在包内（`pi_fork_plan.md` 15 KB），引用可达，无需处理。

---

### P1-6｜哨兵错误用 `==` 比较而不是 `errors.Is`（4 处，含「子进程首次拉起」关键分支）

- `agent/codex.go:503-504`：
  ```go
  if err == nil || err == errNoClient {
      if err == errNoClient {   // ← 首次拉起分支
  ```
- `agent/zcode.go:416-417`：同款，`err == errNoZcodeClient` 判定「首次拉起」。

为什么是问题：**仓库自己立过规矩**。`agent/codexsessions.go:20-24`：
```
// 【为什么要做成哨兵错误】…用裸 fmt.Errorf 的话调用方只能靠字符串匹配猜,一改文案就失效。
var ErrCodexSessionNotFound = errors.New(...)
```
而 `codexsessions.go:170` 确实用 `fmt.Errorf("%w: %s", ErrCodexSessionNotFound, id)` + `errors.Is` 判定。同一个文件里两种做法并存。

域内 `errors.Is/As` 已是主流（生产 40 处），这 4 处是残留。风险：一旦有人给 `errNoClient` 包一层（`fmt.Errorf("...: %w", errNoClient)`），`==` 立刻失效 → **首次拉起分支永不命中 → provider 永久判死**，而且没有任何编译期或测试期信号（这两条路径恰好在「子进程不存在」时被走，正常轮次测试打不到）。

- **收法**：4 处改 `errors.Is(err, errNoClient)` / `errors.Is(err, errNoZcodeClient)`。
- **修复代价**：4 处单行。
- **修复风险**：**零**（语义等价，且是向域内主流写法收敛）。

---

### P1-7｜测试假件重复（**已证语义等价**）：codex/zcode 两个 responder fake

- `agent/codex_approval_test.go:24-27` `fakeCodexResponder` 与 `agent/zcode_approval_test.go:24-27` `fakeZcodeResponder`：**结构体 4 个字段逐字相同**（`mu sync.Mutex` / `results []map[string]interface{}` / `errCodes []int` / `errMsgs []string`，含行尾注释对齐）。
- `RespondError`：codex 222 B / zcode 225 B，归一化接收者名与形参名后**逐字节相同**。
- `resultCount`：107 B / 107 B，**逐字节相同**。
- 唯一真差异：codex 的 `Respond` 多一段 `map[string]string` → `map[string]interface{}` 归一（`codex_approval_test.go:33-40`）；zcode 另有 `lastErr`/`lastResult`，codex 另有 `errCount`。

**语义等价性证明**（脚本 `a8_fakes.py` 归一化比对输出）：接收者类型名不同、形参名不同（`id` vs `rawID`）、字节长度差 3 B（形参名长度差）——除此之外方法体与字段完全同构。同理 `agent/pi_unknown_passthrough_test.go:27` 的 `fakePiUIResponder` 是 `map[string]any` 的窄接口变体，**不算重复**（语义不同的应答面）。

- **收法**：抽一个包内 `_test.go` 共享件 `responderRecorder`（含 4 字段 + `RespondError` + `resultCount`），两个 fake 各自 `embed` 后只写自己的 `Respond` 与专属断言器。净减约 25 行。
- **修复代价**：~25 行净减。
- **修复风险**：**零**（纯测试；`testagent/`、`testkit/` 均不涉及，不新建包）。

---

### P1-8｜God object：`CodexProvider` 42 字段 / 8 把互斥锁 / 单文件 2382 行

`agent/codex.go:36-…`，字段数 42（生产代码里最大），互斥锁/原子量 8+2：

| 职责簇 | 字段 | 锁 |
|---|---|---|
| 轮次串行化 | `turnMu` / `turnActive` | 1 |
| 子进程生命周期 | `lifeMu` / `client` / `inited` / `gen` | 1 |
| 桌面偏好 | `setMu` + 7 个字段 | 1 |
| 线程引用表 | `threadMu` / `threadMap` | 1 |
| 常驻目标 | `goalMu` / `pendingGoals` | 1 |
| guardian 审查 | `gMu` + 4 字段 | 1 |
| 压缩状态机 | `compactMu` / `compactState` | 1 |
| 未决反向请求 | `pendingReqMu` / `pendingResolved` | 1 |
| 运行中插话 | `steerMu` / `activeSteer` | 1 |
| 反问卡能力缓存 | `askFeatMu` + 3 字段 | 1 |
| TTL 缓存 | `capCache` / `mcpCache` / `usageCache` | 内部自带 |
| server 回调 | `TitleHandler` / `GoalHandler` / `ThreadEventHandler` | — |
| 测试注入 | `ProbeHook` / `sessionCall` / `modelsBudget` | — |

**为什么它是 P1 而不是 P2**：跨锁不变量目前**只由散文维护**。`codex.go:497-499`：
```
// 正在执行 turn 时，状态查询失败不能重启共享 app-server。
// Run() 持有的是 turnMu，不会持有 lifeMu；如果这里重启，正在跑的 turn 会被强行切断
```
「Run 不持 lifeMu」这条不变量没有任何类型或测试钉住 —— 8 把锁的获取顺序是隐式知识。

**收法（低成本版，够用）**：把**无锁归属争议的纯状态簇**按现有文件边界搬出去（god object 已经天然分好了文件：`codex_guardian.go` / `codex_compact.go` / `codex_steer.go` / `codex_goal.go` / `codexsessions.go`），具体做法是把这 5 个 `xxxMu + xxxState` 字段对换成 5 个小结构体（内含自己的 mutex），各文件的方法签名从 `p.xxxMu` 改成 `p.xxx.Mu`。**每簇 ≤30 行机械改动**，方法体逻辑一行不动。

不建议做的：把 5 家 provider 拆成 `internal/agent/codex` 等子包 —— 跨文件私有符号（`threadRef`、`errNoClient`、`clip`、`mustJSON`、`firstNonEmpty`、`sharedRuntimeBackend` …）密布，拆包会炸出上百个导出，改动量从「乙级」直接跳到「甲级」，且与另两条 daemon 线有文件耦合风险。

- **修复代价**：5 簇 × ~30 行机械改动 ≈ 150 行 diff。
- **修复风险**：**中低**。每簇独立、可分 5 个 commit；风险点是 mutex 从 provider 搬到子结构体后要确认**没有别处持 `p.xxxMu`** —— 已核：每个 mutex 的访问点全在同簇文件内（`compactMu` 2 处、`gMu` N 处均在 `codex_guardian.go`）。不碰甲级禁区、不碰红线、不碰 proto。

---

### P2 清单（13 条，均为风格/一致性偏好，不建议单独排期）

| # | 位置 | 问题 | 收法 | 代价/风险 |
|---|---|---|---|---|
| P2-1 | `agent/openai_compat.go:452` `clip()` 与 `agent/pi_events.go:909` `clipRunes()` | 同包两个近名截断器，语义不同（前者保字节预算回退 rune 边界，后者委托 `runtime.ClipRunesEllipsis`） | 前者改名 `clipUTF8` | 32 处调用点机械替换，~32 行；零风险 |
| P2-2 | `agent/pi_events.go:263` `MapToolExecUpdate(raw, clip int, elapsed)`、`:286` `MapToolExecEnd(raw, clip int, elapsed)` | **参数 `clip int` 遮蔽包级函数 `clip(s string, n int) string`** —— 函数内 `clip` 是 int，全包其他地方是函数 | 形参改名 `maxRunes` | 2 签名 + 2 调用点；零风险 |
| P2-3 | `agent/codex.go:187` | `fmt.Errorf("%s", out.Error)` —— 应 `errors.New`；`%s` 包字符串是 `perfsprint`/`S1028` 味 | 改 `errors.New(out.Error)` | 1 行；零风险 |
| P2-4 | `agent/codex.go:813/841`、`agent/zcode.go:316`、`agent/pi_delete.go:…` 等 4 处 | 用 `fmt.Errorf("常量串")` 造哨兵，应用 `errors.New` | 4 处改写 | 4 行；零风险 |
| P2-5 | 23 处（`pi_delete.go:33/41`、`pi.go:1637`、`pi_followup.go:72`、`pi_selection.go:64/83/106/109`、`zcode.go:240/316/664/684/694/715`、`zcode_selection.go:147/212/220/254/263/273/291`、`openai_compat.go:238`） | error 字符串首字母大写（违反 Go `ST1005`） | 若要改须与 UI 文案面同步（错误文案会上屏），**建议只立规矩不改存量** | 中；跨域耦合 |
| P2-6 | `agent/codex.go:1748-1755` / `1764-1772` / `1776-1784` | 同一函数内「interrupt → 快照 partial → 有 partial 返回 partial / 无 partial 返回 nil」的 8 行形状重复 3 次 | 提一个 `returnWithPartial(partial, err)` 局部闭包 | ~15 行净减；零风险 |
| P2-7 | `agent/codex.go:2372-2382`、`zcode.go:761-772`、`dsh.go:1184-1195`、`hermes.go:1053-1065`、`pi.go:2080-2095` | 5 份「锁包 map 的 get/set」访问器（`threadRef` 类型已共享、mutex 纪律已共享，只差最后一层包装） | 提 `threadRefStore{mu, m}` + `Get/Set/Del`，5 家 embed | ~40 行净减。**注意**：hermes 的 `dropThreadRef` 还删 `slashByThread`、pi 是 `cursors map[string]string` 语义不同 —— 只能收 get/set 对，drop/pi 不收 |
| P2-8 | `agent/pi.go:654` `threadModelExplicit` | 私有函数**生产 0 引用**，只被 `pi_prefs_effect_test.go:102` 用（双向计数确认） | 删函数 + 改测试直接调 `applyModel` 的判定，或把测试挪到断言 `applyModel` 行为 | 8 行；零风险 |
| P2-9 | `agent/zcode.go:1067` `mapZcodeToolResult` | 生产 0 引用（生产走 `mapZcodeToolResultClipped`，`zcode.go:2030`），只被 `zcode_test.go:74/89` 用 | 删，测试改调 `mapZcodeToolResultClipped(raw, zcodeToolClip)` | 5 行；零风险 |
| P2-10 | 45 个（agent）/ 60 个（runtime）导出生产符号**无跨包引用** | 多余导出。其中 `RegisterZcode`、`ConfigOverride`、`ConfigOverrideFor`、`ErrZcodeBusy`、`StdTools`、`ToolByName`、`Requirements`、`Map*Event` 族最明显（runtime 侧多数是 `contract_suite.go` 的接口方法，必须导出，不算） | 只收 agent 侧最明显的 8 个 | ~8 行；零风险。**不建议批量做** —— 逐个确认成本高于收益 |
| P2-11 | `agent/runtime/session.go:19` `NativeSessionAdapter`、`:67` `CheckpointManager`、`runtime/cache.go:145` `CacheManager`、`runtime/memory.go:171` `MemoryManager` | 噪声词（Manager/Adapter）4 处 | `CacheManager`→`Cache`、`MemoryManager`→`Memory`（后者已与 `Backend.Memory()` 撞名，需小心）；`CheckpointManager`→`Checkpoints` | 低；`MemoryManager` 改名有撞名风险，建议只改前两个 |
| P2-12 | `agent/codex.go:789`、`578`、`1280`、`1508`；`pi.go:474`、`1153`、`1196`；`zcode.go:2052`、`2159`、`2164`；`dsh.go:1376` | 11 处 `go <server 层回调>()` fire-and-forget，**无 ctx、无 recover、无生命周期绑定**。goroutine 内 panic 会带崩整个 daemon | agent 域内无法单方修（需 server 层配合加 recover）。**只标注**：建议 server 侧回调入口统一 `defer recover()` | 跨域；不在本域可改范围 |
| P2-13 | `agent/pi.go`、`agent/hermes.go`（无 `if ctx == nil` 守卫） vs `agent/codex.go:465`、`dsh.go:313`、`zcode.go:372`（三家 `withClientCtx` 首行都是 `if ctx == nil { ctx = context.Background() }`） | nil-ctx 防御 3/5 家有 2/5 家无。**疑似**：无法从本域判定 server 层是否可能传 nil ctx | 要么补齐 pi/hermes 的 `clientFor`/`ensureClient`，要么三家一起去掉（对齐到「Go 约定不传 nil ctx」） | 低；先定方向再动手 |

---

## 3. 适配器一致性矩阵（5 家 × 24 维）—— 本域头号重点

`N = {codex, pi, zcode, dsh, hermes}`。列含义：**收**=可收且应收 / **已收**=已抽共享骨架 / **不收**=语义不同，不该收 / **缺**=该维度有缺口。

| # | 维度 | 落面 | 判定 | 证据 / 不收理由 |
|---|---|---|---|---|
| 1 | 子进程生命周期骨架 | 各家一份 `withClientCtx`（codex/dsh/zcode）+ `clientFor`(pi) + `ensureClient`(hermes) | **不收** | 骨架同形但语义三分：codex `TryLock` 立即忙拒 + `turnActive` 守卫；dsh/zcode 有界排队 + dsh 另有 `awaitHandshake` 礼让；zcode 无 initialize 握手 + 独有闪退重试 + `errors.As` 排除 native 错误。收成一个泛型会得到一个 8 参数的万能函数 —— 这正是 R1「表面重复」的典型 |
| 2 | 生命周期锁 | ✅ **已收** `life_lock.go:24` `lockLifeMu(*sync.Mutex, wait)`，dsh:302 / hermes:193 / zcode:357 三家共用 | 已收 | codex 走 TryLock 不共用，是有意分歧 |
| 3 | 忙哨兵错误 | 各家一份 | **不收** | `ErrBusy`(codex) / `dshBusy`(dsh) / `ErrZcodeBusy`(zcode) 文案必须指名道姓（`zcode.go:363` 注释明说「不窜用 codex 的 ErrBusy 文案」）。语义正确 |
| 4 | 线程/会话引用登记 | 5 份 `map + mutex + get/set` | **可收（P2-7）** | `threadRef{id,gen}` 类型已共享、mutex 纪律已共享，只差最后一层包装。pi 的 `cursors` 是字符串游标、`drop` 各有额外语义 → 只收 get/set 对 |
| 5 | 引用读取 + gen 校验 | `loadedCodexThreadID`(codex_goal.go:31) / `loadedZcodeSessionID`(zcode.go:776) / dsh 侧内联 / hermes 侧内联 | **不收（收益低）** | 各 3-8 行，各家 gen 语义一致但取法不同；抽出来只省 12 行却多一个间接层 |
| 6 | 桌面偏好消费 | ✅ `provider.go:100` `PrefsAware` + `:112-118` **五家编译期断言名单** | 已收 | 五家消费子集不同（`provider.go:94-99` 如实文档化「pi/zcode 只消费说得通的子集」）—— **能力面抽取成功、业务语义保留差异**，是本域最漂亮的一处 |
| 7 | 原生设置目录透出 | ✅ `native_config_options.go:51` `NativeConfigOptions`（2 方法） | 已收（仅 dsh/pi 实现） | zcode/codex 不实现是如实表达（有各自设置面） |
| 8 | 图片输入能力表态 | ✅ `image_input_capability.go:42` `ImageInputter` + `:49-55` 五家编译期断言 | 已收 | dsh 恒 false / hermes 读协商事实，**声明的是已验证事实**（`:29-34` 如实文档化翻位条件） |
| 9 | 图片输入实际映射 | codex `turnInputWithImages` / pi `blocksToPrompt` / zcode `zcode_image_input.go` / hermes `hermes_image_input.go` | **不收** | 四家落地上游协议形状完全不同（v2 `LocalImage` / base64 `type:image` / 临时文件 attachments / ACP ImageContentBlock） |
| 10 | 反向请求审批处理 | 各家一份 `handleXxxApproval`/`handleRequestPermission`（codex 11 参数 / dsh 9 / hermes 8 / zcode 7） | **不收（但参数数要治，见 P2/§4）** | 协议形状、选项语义、超时口径全不同。dsh/hermes 的 allow/reject 分支结构确实同形（`dsh.go:1729-1735` vs `hermes.go:1011-1022` 逐行对仗），但那是 **ACP 协议同源**导致的必然重复，收了反而会挡住 dsh 后续 evolve。**可考虑**：只把 `reject/allow → respond(选项)` 的 6 行收成一个共享闭包工厂（`askACPAllowReject(perm, respond)`），dsh/hermes 各调一次 —— 收益 ~10 行，优先级低 |
| 11 | 未知反向请求透传 | ✅ `unknown_passthrough.go:162` `askUnknownApproval` **共享骨架**，四家薄适配（`:220`/`:233`/`:262`/`:356`） | 已收（本域最佳范式） | 骨架吃 `agentID/threadID/method/params/approve/waiting/budget/respondAllow` 8 个参数，薄适配只接各家应答形状。codex 的 `askWithResolvedRace`（`:267-292`）是专有出口差异，注释 `:159-161` 已说明 |
| 12 | 反应请求应答面 | `codexApprovalResponder`(2) / `zcodeResponder`(2) / `piUIClient`(1) | **不收** | 三个窄接口形状本来就不同（`json.RawMessage` vs `int64` vs `string`） |
| 13 | 会话发现/导入 | `codexsessions.go` / `pi_sessions.go` / `zcode_sessions.go` 三份 | **不收** | codex 扫 `~/.codex/sessions/**/rollout-*.jsonl` + 双格式解析；pi 扫 `~/.pi/agent/sessions/**/*.jsonl`；zcode **走协议不走文件**（`zcode_sessions.go:5` 明说「为什么走协议而不是读 ~/.zcode/cli/db/db.sqlite」）。文件格式与协议面都不同 |
| 14 | 会话删除 | ✅ `runtime_descriptors.go` **四家实现集中**（`:259`/`:276`/`:288`/`:300`）+ server 侧零名单接口断言 | 已收（但守卫不齐，见 P0-1） | 结构已收对，codex 的删除守卫没跟上 pi |
| 15 | 子代理/collaborator 读取 | `codex_subagents.go` / `pi_subagents.go` / `zcode_subagents.go` | **不收** | 三个上游协议面完全不同；dsh/hermes 无此能力（如实缺席） |
| 16 | 通知分类 | `codexGlobalNoticeKind`(codex.go:732) / `codexMethodNoticeKind`(:201) / `codexItemNoticeKind`(:158) / `piNoticeKind`(:283) / `piEventNoticeKind`(pi.go:678) / `zcodeNoticeKind`(:367) / `zcodeToolRestNoticeKind`(zcode.go:1195) / `dshNoticeKind`(:234) | **不收** | 都是「method 字符串 → normalized_kind」查表，但词表按 agent 分（`pi_events.go:663-670` 如实说明「为什么不并入 piNoticeKind」：文件归属按轮次切分） |
| 17 | 通知登记/限频 | ✅ `native_notifications.go:405` `recordAs` **共享**，30 个生产调用点全部走它 | 已收 | ⚠️ 配套的 `record`（`:398`）已成死代码且注释说谎 → P1-3 |
| 18 | 事件人话词表 | `runtime/knowledge_{codex,dsh,pi,zcode}.go` + `event_translate.go:69` 合并（**带 `panic` 重键守卫**） | **甲级禁区，只标注** | 四个 map 键是 `"<agent>\x1f<method>"` 字面量，`\x1f` 不可见；**没有任何测试校验键的 `\x1f` 前缀形状**（扫遍 `*_test.go` 无 x1f 校验）。写错前缀 = 该词条静默永不命中，且不会有任何信号。**建议（若日后开禁区）**：加一条 `TestKnowledgeKeysHaveAgentPrefix` 断言每键 `strings.HasPrefix(k, <对应 agent> + "\x1f")` —— 一条测试，零行为变化 |
| 19 | 事件前缀词表 | `runtime/knowledge_dsh.go:7` `knownEventPrefixes`（只 dsh 用） | **甲级禁区，只标注** | 结构体含 `agent` 字段但实际只有 dsh 一家填；`event_translate.go:84-86` 那段 `knownEventPrefixes` 的说明注释**与它的声明分离**（声明在 `knowledge_dsh.go:7`），在 `event_translate.go` 里读会看到一段找不到归属的注释 |
| 20 | 轮次看门狗阈值 | dsh `dshSilenceWatchdog` / hermes `hermesSilenceWatchdog` / zcode `zcodeWatchdogSilence` 具名；**codex 全裸字面量** | **该收（P1-2）** | 三家都写「对齐 codex」，codex 却是裸的 + 注释值还写错 |
| 21 | 轮次上限 | dsh `dshTurnCap` / hermes `hermesTurnCap` / pi `piTurnCap` / zcode 具名；**codex `60*time.Minute` 裸字面量**（`codex.go:1774`） | 该收（并入 P1-2） | 同上 |
| 22 | 模型/思考强度切换 | `codex ConfigOverride*` / `pi applyModel`+`pi_selection.go` / `zcode applyModel`+`zcode_selection.go` / dsh `applyConfigOptions` | **不收** | codex 走 `thread/start` 的 `config` 覆盖 map；pi 走 `piapp.SetModel` + `set_thinking_level`；zcode 走 `session/resume` 快照回读 + `set_config_option`；dsh 走 ACP `set_config_option`。四条完全不同的协议路径 |
| 23 | 错误分类（结构化诊断） | codex ✅ / pi ✅ / dsh ✅ / **zcode ❌ / hermes ❌** | **该收（P1-1，本轮头号）** | 消费点是类型断言 → 缺实现不报错、不编译红 |
| 24 | 轮次取消 | codex `turn/interrupt`×3 / zcode `session/stop`×2 / pi `abort` / hermes `session/cancel` / dsh `Cancel` | **不收** | 上游取消方法名各不相同，且各自对「取消后仍要交回已流出内容」的 partial 处理位置不同 |

**矩阵结论**：24 维里 **8 维已收（共享骨架到位）**、**1 维该收未收（P1-1 实质缺口）**、**3 维小收（P2-2/P2-7 + 并入 P1-2）**、**12 维不该收**。这与 R1 轮的结论一致且更强：**本域的复制粘贴债不是「同型不同名」，而是「该共享的最后一层包装没抽」+「一个能力维度悄悄缺两家」**。逐格判定依据都写在表里，复核成本 ≈ 读 3 个文件。

---

## 4. 命名 / 结构 / 函数

### 4.1 命名

- **`getX` 前缀：0 处**（Go 惯例守住）。
- **谎言 `isX`（非布尔）/ `newX`（不新建）：0 处。** `newCur`(pi.go:2138) / `newID`(codex_threads.go:330, runtime/identity.go:116) / `newInstallation`(runtime/identity.go:78) 均名副其实。`newID` 在两个包各有一份（互不相关，Go 包级作用域，不构成冲突）。
- **噪声词 7 处**（全生产）：`CacheManager`(runtime/cache.go:145) / `MemoryManager`(runtime/memory.go:171) / `CheckpointManager`(runtime/session.go:67) / `NativeSessionAdapter`(runtime/session.go:19) / `attachGlobalHandler`(codex.go:542, zcode.go:475) / `ThreadEventHandlerAsync`(codex.go:785) / `attachUnknownResponseHandler`(zcode.go:469)。量级轻 → P2-11。
- **同型不同名（已核实的真债）**：
  - `clip` / `clipRunes` 同包两名近义不同义 → P2-1
  - `clip` 形参遮蔽包级函数 → P2-2
  - `threadMap` 的 getter 五家五种叫法：`codexThreadRef` / `zcodeThreadRefFor` / `dshThreadRefFor` / `threadRefFor`（hermes）/ `cursorOf`（pi）。其中 **hermes 的 `threadRefFor` 最贴切**，其余三家带冗余前缀。收进 `threadRefStore` 后（P2-7）问题自动消失。
- **单字母非 lambda 作用域变量**：prod 共 174 处，top：`c`×41（多为 `*codexapp.Client` 短别名，可接受）/ `b`×25 / `m`×14 / `r`×11 / `p`×9。全部是「客户端/块/响应/请求」短别名，作用域都在单个函数内且注释交代。**不列债**。
- **缩写难读**：`dshapp` / `piapp` / `zcodeapp` / `hermesapp` 是包名统一风格，不算债。

### 4.2 函数与结构

**超长函数 Top 10（生产，花括号配对实测）**：

| 行数 | 位置 | 函数 | 备注 |
|---|---|---|---|
| **708** | `agent/codex.go:1104` | `(*CodexProvider).Run` | 全域最长 |
| **500** | `agent/pi.go:855` | `(*PiProvider).Run` | |
| **370** | `agent/zcode.go:1814` | `(*ZcodeProvider).Run` | |
| **305** | `agent/hermes.go:258` | `(*HermesProvider).Run` | |
| 276 | `agent/codex_events.go:174` | `codexNotificationEvent` | **纯映射表**，无副作用 |
| 232 | `agent/codex_events_ext.go:61` | `codexExtendedNotificationEvent` | 同上 |
| 231 | `agent/dsh.go:678` | `(*DshProvider).Run` | |
| 173 | `agent/pi_events.go:421` | `MapStatusEvent` | 同上 |
| 156 | `agent/codex_capabilities.go:241` | `fetchCapabilitiesDetailed` | **甲级禁区，只标注** |
| 138 | `agent/codex_requests.go:388` | `buildApprovalCall` | 审批卡构造，116 参数→4 |

`>100 行` 的生产函数共 **15 个**；5 个 `Run` 占 2114 行。

**`Run` 该不该拆？** 拆不动也不该拆 —— 它是「一轮 agent 执行」这个天然事务边界，内部按「取客户端 → 建/挂会话 → 收指令 → 起轮 → 泵事件 → 等结束 → 收尾」线性推进，拆开会把 `turnMu`/`lifeMu`/`turnActive` 三个生命周期状态机的持有关系打散，风险远大于收益。**真问题是 P1-8 的字段/锁聚堆，不是函数长度。**

**参数 >4 个：生产 20 个**

| 参数数 | 位置 |
|---|---|
| **11** | `agent/codex.go:988` `handleCodexApproval(rsp, id, kind, method, params, files, ctx, sink, approve, waiting, threadID)` |
| 9 | `agent/dsh.go:1661` `handleDshRequest` |
| 8 | `agent/hermes.go:961` `handleRequestPermission` / `agent/unknown_passthrough.go:233` `handleZcodeUnknownRequest` / `:262` `consumeCodexUnknownRequest` / `:162` `askUnknownApproval` |
| 7 | `agent/unknown_passthrough.go:220` `handleDshUnknownRequest` / `agent/zcode.go:1379` `handleZcodePermission` |
| 6 | `agent/hermes.go:671` `projectSessionUpdate` / `agent/openai_compat.go:348` `execWithApproval` |
| 5 | 6 个（`pi.go:831` / `codex.go:886` / `dsh.go:1531` / `hermes.go:882` / `openai_compat.go:215` / `pi_events.go:794` / `unknown_passthrough.go:356` / `zcode.go:1646/1674` / `zcode_session.go:96`） |

**收法**：`handleCodexApproval` 的 11 个参数里，`sink/approve/waiting/threadID/ctx` 五个是 provider 级上下文 → 收成一个 `approvalCtx` 结构体（`unknown_passthrough.go:162` 的 8 参数骨架已经证明这条路可行）；`rsp/id/kind/method/params/files` 六个是单次请求 → 收成一个 `codexServerRequest`。`handleDshRequest`/`handleRequestPermission` 同理。**这是纯签名重构 + 三处调用点改写**，约 60 行 diff，零行为变化。

**布尔 flag 参数：生产 20 个**（`includeLayers` / `allow` / `asc` / `archived` / `haveTitle *bool` / `hide` …）。绝大多数是「正向命名的自然布尔」（`includeArchived`、`allow`、`asc`），可读性尚可。唯一像 flag 参数的是 `readHeld bool` 风格的 `uiResponseFromDecision(approved bool)` / `piToolFooter(…, running bool)` —— 判定「能收」但优先级低，不列债。

**巨型 switch**：全生产 97 个 `switch`/`select`，**>12 case 的只有 3 个**：`codex.go:1231`（15 case，`Run` 内的通知处理）/ `codex_events.go:199`（15 case，method→事件映射）/ `codex_events_ext.go:62`（13 case）。后两个是**纯映射表**（无副作用、有 default、有 per-case 注释），是最健康的巨型 switch 形态。`codex.go:1231` 在 708 行函数里，是 P1-8 的直接症状。

**超大接口（>10 方法）：2 个**

- `agent/dsh.go:97` `dshClient`（15 方法）
- `agent/hermes.go:97` `hermesClient`（13 方法）

**判定：不收。** 两者都是「ACP/JSON-RPC 客户端协议面」的完整镜像，注释已说明设计意图（`hermes.go:105-110`「hermesapp 属 agent 之外的包…由 `*hermesapp.Client` 既有方法直接满足（零协议改动）」），且都有 `var _ dshClient = (*dshapp.Client)(nil)` 编译期钉死。它们的真正成本是让 `fakeDshClient`(dsh_test.go:57) 需要实现 15 个方法 —— 这是**刻意付出的可测性代价**，不是债。

**God object Top（含职责概述）**：

| 文件 | 行 | KB | 职责 |
|---|---|---|---|
| `agent/codex.go` | 2382 | 110.8 | codex provider 全部：`CodexProvider` 结构体 + 子进程生命周期 + `Run` + 反向请求路由 + 通知全局钩子 + 模型/用量/能力查询 + 线程读 + 图片输入 + 工具卡 + 桌面偏好映射 → **P1-8** |
| `agent/pi.go` | 2277 | 99.1 | pi provider 全部：结构体 + 子进程挂载 + `Run` + 扩展 UI 分发 + 断点续传 + 命名 + 停止 + fork + 模型缓存 |
| `agent/zcode.go` | 2189 | 92.2 | zcode provider 全部：结构体 + `Run` + 审批/反问/elicitation 三条反向通道 + 选择面 + 目标 + 快照 + 模型应用 |
| `agent/dsh.go` | 1819 | 79.4 | dsh provider 全部 + 握手礼让治理 + 工具卡 + 计划卡 + 会话重建 |
| `agent/hermes.go` | 1100 | 52.6 | hermes provider 全部 + 看门狗 + 取消 + 斜杠命令 |
| `agent/pi_events.go` | 915 | 35.5 | pi 事件 → TurnServerMessage 全量映射（`MapStatusEvent` 173 行等 5 个 `Map*` 函数） |
| `runtime/backend.go` | 1166 | 40.2 | `Backend` 18 字段 / 2 锁；记忆·缓存·资源·checkpoint·契约套件的门面 |

**206 文件的子域切分是否合理？** **合理，且比「拆包」更合适。** 判据：文件前缀统计显示 `codex*` 55 / `pi*` 41 / `zcode*` 32 / `hermes*` 18 / `dsh*` 14 = 160/206（78%）按 agent 前缀切；其余是 `native_*`(7) / `unknown_*`(7) / `collab_*`(3) / `echo_*`(3) / `openai_*`(3) / `image_*`(3) 等**横切面**。这是一套**按 agent 纵向 + 按能力横向**的双轴切分，靠文件命名的前缀约定维持，没有 Go 包边界强制。问题不在切分方式，而在**顶层 provider 文件（codex/pi/zcode/dsh/hermes.go）没有跟着横向切走** —— 它们的字段/锁/方法该落到 `codex_guardian.go`/`codex_compact.go` 这类已存在的横切文件里（正是 P1-8 的收法）。另注：`runtime/` 的 `knowledge_*` 11 文件是词表区，**甲级禁区**。

---

## 5. 注释审查

### 5.1 TODO / FIXME 全清单

**生产代码：0 条真实 TODO/FIXME/XXX/HACK。** 全域仅 1 处字符串命中 `runtime/agent_runtime.go:32`，且它是**反讽式说明**：

```
// 这不是"预留 TODO"，而是契约的一等状态：VerifyRuntime 会把 nil 与声明…
```

**这是本域最干净的注释面之一** —— 没有孤儿债、没有无归属的待办。

### 5.2 注释掉的代码

自动扫描出 12 处候选，**逐一复核后 11 处是误报**（散文注释里带括号，如 `pi_events.go:383` `// usage = 本轮增量(diff),total_usage = …`）。唯一像代码的是 `runtime/backend.go:194`：

```go
//	rt := backend.FullRuntime(descriptor, thread)
```

复核结论：**也是误报** —— 它在 `//` 之后的**缩进代码块**里，是 godoc 的用法示例（`backend.go:190-196` 的函数头注释），gofmt 会把它渲染成 `<pre>` 代码块，是**正确的文档写法**。

**净结果：注释掉的代码 0 处。**

### 5.3 谎言注释（20 处高危逐句对账，命中 4）

选取规则：含**可证伪的量化断言 / 存在性断言 / 调用点断言 / 「唯一调用方」**的注释，共筛出 526 条高断言注释，抽取 20 条逐句与实现对账：

| # | 注释 | 断言 | 实到 | 判定 |
|---|---|---|---|---|
| 1 | `codex.go:1738` | 90 秒判失败 | `codex.go:1763` `> 150_000` ms | ❌ **谎言（P1-2）** |
| 2 | `native_notifications.go:395-397` | pi/zcode/dsh 仍在用 `record` | 生产 `notices.record(` = 0 | ❌ **谎言（P1-3）** |
| 3 | `native_notifications_test.go:180` | 「既有调用方零成本:pi/zcode/dsh 等」 | 同上 | ❌ **谎言（随 P1-3 一并修）** |
| 4 | `provider.go:195-196` | zcode 待「主模型合并时接入」 | `provider.go:208` `RegisterZcode(m)` 已接入 | ❌ **谎言（P1-4）** |
| 5 | `dsh.go:5` | 蓝本 `2026-09-20-dsh-integration-plan.md` 存在 | 全仓不存在 | ❌ **悬空引用（P1-5）** |
| 6 | `codexsessions.go:139` | 「手机端删除会话**若要**要真正落到电脑上,需一并调用它」 | `runtime_descriptors.go:264` **已经调用** | ⚠️ 措辞像未来指令但已是现状，读者会误判为未接线 → 并入 P1-4 一类（弱） |
| 7 | `codex.go:139` | `fetchUsageUncached` 是「本轮唯一调用方」 | `codex.go:183` 单调用点（+2 测试点） | ✅ 属实 |
| 8 | `codex.go:743` | `account/usage/read`「本轮唯一调用方」 | 单调用点 | ✅ 属实 |
| 9 | `codex.go:585` | `codexGlobalNoticeKinds` 是「唯一一张全局登记表」 | 定义 `:594` + `:733` 查询 + 4 处测试遍历 | ✅ 属实 |
| 10 | `image_input_capability.go:12` | codex 按 `inputModalities` 门控 `codexSupportsImageInput` | `codex.go:1718` 单调用点 | ✅ 属实 |
| 11 | `codex.go:968` | 「改预算三处一起动：本变量、server approvalWaitBudget…」 | 三处 + 三份测试钉住（`codex/dsh/hermes_approval_wait_budget_20261009_test.go`） | ✅ 属实 |
| 12 | `zcode.go:759-760` | 「threadMap 的唯一读写入口…不允许任何路径直接摸 p.threadMap」 | 全量核对 `.threadMap` 11 处命中，**全部在访问器内部** | ✅ 属实（不变量目前成立） |
| 13 | `hermes.go:275-280` | turnMu「解锁权单一所有者…绝不双解锁」 | `:281-291` reaper 模式正确 | ✅ 属实 |
| 14 | `codex.go:456` | 「绝不"超时后用 go 继续等"」 | `:458-475` 全程同步 | ✅ 属实 |
| 15 | `dsh.go:398` | 凭据文件「只在 daemon 数据目录内(默认 ~/.pocketagentd),不碰用户 ~/.dsh」 | `resolvedDataDir`(`dsh.go:429`) 回落 dshapp 默认 | ✅ 属实 |
| 16 | `codex.go:1909` | 「这里只改内存,绝不写 ~/.codex/config.toml」 | 全域无任何 `~/.codex` 写调用（只有 `sessions` 读） | ✅ 属实（config.toml 部分） |
| 17 | `pi_config_options.go:90` | 「set_steering_mode 会把修改持久写入用户桌面全局配置(红线 1 明令不写 ~/.pi)」 | `pi_config_options.go` 无对应写调用 | ✅ 属实 |
| 18 | `codex.go:116/1152` | 「唯一官方途径是改写 ~/.codex/config.toml,红线永久封死」 | 事实陈述，无对应调用 | ✅ 属实 |
| 19 | `native_notifications.go:418-420` | 「sink 为 nil = 只留痕不出卡」 | `:420-422` `if sink == nil { return }` | ✅ 属实 |
| 20 | `dsh.go:10-18` | 「本 Provider 明确不做的事」5 条 | Steerer 未实现 / 无 sandbox 面 / 无 importer / 块级流 / 图片协商报错 —— 逐条核对全部属实 | ✅ 属实 |

**命中率：4/20（20%），全部集中在「历史变更残留」而非「逻辑说谎」** —— 注释的**内容质量**很高（20 条里 16 条逐字属实，且大量注释写的是「为什么」而非「是什么」），问题在**演进后的清理**。

### 5.4 变更日志式注释

- `provider.go:195-196`（P1-4）—— 典型形态：记录了一个未完成分支的意图，已完成但注释留着。
- `agent/codex.go:547` `// (旧实现此处恒记 0 占位)。`、`codex.go:1133` `// startCodexThread 唯一实现;"过渡期双路径"就此退役。` —— 「旧实现」「退役」是变更日志语汇，但它们同时标注了**当前唯一实现的位置**，有检索价值，**保留**。
- `knowledge_*.go` 的 `// ---- 0.156.1 收编批 ----` / `// ---- 0.159.2 收编批 ----` / `// ---- 2026-09-30 未知台账积压收编批 ----`（词表区）—— **甲级禁区，只标注**：这是「批次化追加记录」，长期会让词表变成编年史。**不建议改**（改动面大且碰禁区），但值得记录这个形态。

---

## 6. 错误处理 / 死代码 / 并发 / 测试

### 6.1 错误处理

| 项 | 数量 | 判定 |
|---|---|---|
| `_ = <call>` 吞错误 | **117** | **99% 无害，逐类核过**（见下） |
| `fmt.Errorf` 无 `%w` | 116 处中 **49 处** | 其中 **4 处**是「用 fmt.Errorf 造哨兵」（应 `errors.New`）→ P2-4；其余 45 处是新建错误（无被包装对象），本就不需要 `%w` |
| error 首字母大写 | **23** | → P2-5（建议只立规矩不改存量：文案会上屏） |
| `panic` / `log.Fatal` / `os.Exit` | **1**：`runtime/resource_disk.go:181` | **干净**：注释已给出不可达论证（`:179-180`「ResourceLease 全是可序列化字段，到这里失败只可能是未来加了不可序列化字段：那必须当场炸在测试里」）且紧邻的 `:191-194` 同一路径上另有返回错误的版本。**不列债** |
| `errors.Is/As` 使用 | 40 处 | 主流写法，但有 4 处残留 `==` → **P1-6** |
| `defer` 里未处理的 err | 0 | 25 处 `defer x.Close()` / `defer cancel()` / `defer tick.Stop()`，**全部无返回值**，无需处理 |

**117 处 `_ =` 吞错误的逐类核对**：

| 类 | 处数 | 为什么无害 |
|---|---|---|
| `_ = json.Unmarshal(raw, &n)` 解到 `map[string]interface{}` / 局部标量，失败即**如实留空** | ~60 | 显示投影路径。`dsh.go:1550` / `hermes.go:1042` 同款：`args["input"] = nil` 是「解析不出」的诚实表达，不是静默丢错 |
| `_ = rsp.RespondError(...)` / `_ = rsp.Respond(...)` 应答写回 | ~25 | **多数带 B4 留痕日志**（`codex.go:999-1003` 的 `respond` 闭包、`dsh.go:1708` / `hermes.go:998` 的「B4 应答必达性留痕」注释）。协议上应答写回失败无可补救手段（子进程已退出），日志是唯一出口 |
| `_ = os.Remove(tmp)` / `_ = tmp.Close()` 临时文件清理 | 14 | 幂等清理，失败无补救（`cache_disk.go` / `checkpoint.go` / `budget.go` / `identity.go` / `resource_disk.go` / `collab_policy.go`） |
| `_, _ = p.client.Call("turn/interrupt", ...)` 打断 | 8 | 打断是「尽力而为」，注释已说明（`codex.go:1775`「超时必须真的打断…」）；打断失败时已在返回错误路径上 |
| **`workspace, _ = os.Getwd()`** | 2（`hermes.go:305` / `hermes_compact.go:80`） | ⚠️ **唯一的可疑点**：Getwd 失败 → `workspace=""` 静默继续，作为 `cwd` 传给 `SessionNew`。应 `log` 一行。→ 并入 P2 |

**agent 进程非零退出的错误分类是否丢失 stderr？** —— **未丢失，且是本域亮点**。`agent/codex_events.go:47` `parseCodexErrorNotification` 把上游 `error.codexErrorInfo`（含 HTTP status / URL / 是否可重试）解析成 `pb.Error`；`codex_events.go:74-75` `codexErrorCode` / `codexHTTPStatus`；`provider.go:37` `ErrorInfoCarrier` 把结构化诊断交给 server 保真（`server/thread.go:1511`）。**唯一缺口是 zcode/hermes 两家没接这条通道（P1-1）**。dsh 侧另有 `dsh.go:1788` `dshRunErrorFrom` 做 ACP 错误 → 结构化的映射。

### 6.2 死代码（双向计数）

方法：对每个**私有**（小写开头）func/type/var 分别数 **(a) 生产代码包内引用数** 与 **(b) 含测试的包内引用数**，只在 `a ≤ 1 且 b ≤ 1`（即只有定义处一次）时判死。

| 包 | 私有声明总数 | 判死 |
|---|---|---|
| `internal/agent` | 603 个带接收者的 func + 全部私有 type/var/const | **0** |
| `internal/runtime` | 同上 | **0** |

**全域零死代码。** 三个「生产无引用、只被测试用」的私有函数（**都不是导出**，所以不是「为可测性而导出」，而是纯测试存活）：

| 函数 | 测试引用 | 性质 |
|---|---|---|
| `agent/native_notifications.go:398` `(*nativeNoticeRecorder).record` | 9（`notification_kinds_test.go:139` **已明写生产应为 0 命中**） | 收尾期残留 + 注释说谎 → **P1-3** |
| `agent/pi.go:654` `threadModelExplicit` | 2 | 纯测试存活 → **P2-8** |
| `agent/zcode.go:1067` `mapZcodeToolResult` | 2 | 纯测试存活（生产走 `mapZcodeToolResultClipped`）→ **P2-9** |

**导出符号面**（P2-10）：agent 有 45 个、runtime 有 60 个导出生产符号无跨包引用。**这不等于死代码** —— runtime 侧多数是 `contract_suite.go` 的接口方法（必须导出才能满足 `Backend` 实现）；agent 侧是 `Map*Event` 族（被本包调用，只是不需要导出）。判定为**多余导出**，非死代码，优先级最低。

### 6.3 并发与生命周期

| 检查项 | 结果 |
|---|---|
| **goroutine 无退出路径** | **0 处**。28 个 `go` 站点逐个核：`outCh := make(chan X, 1)` 带缓冲（`codex.go:1015` / `unknown_passthrough.go:274` / `dsh.go:819` / `hermes.go:499`）→ 迟到的应答由 GC 回收（注释已说明）；`codex.go:1577` 通知泵有 `notifStop` + `close(notifDone)`；`hermes.go:281-291` turnMu 抢锁带 reaper；`codex_compact.go:94` fireCompact 经 `withClient` 有 `clientInitBudget` 上界。**11 处 fire-and-forget server 回调无 recover** → P2-12（域内不可单方修） |
| **context 未透传 / 未 cancel** | **0 处真漏**。28 个 `go` 站点全部要么透传 `ctx`，要么经 `withClient(ctx)` 的有界兜底；所有 `context.WithTimeout` 都配了 `defer cancel()`（25 处 `defer cancel()`，0 处遗漏） |
| **`context.Background()` 在请求路径硬写** | 7 处，**全部有注释解释**：4 处是 `withClientCtx`/`withControlClient` 首行的 `if ctx == nil { ctx = context.Background() }` 防御（codex:465 / dsh:313 / zcode:372 / zcode_selection:35 / codex:385 / codex:287）；2 处是 `runtime/memory_inventory.go:234` / `recovery_events.go:77` 的无 ctx 后台维护操作（注释已说明锁外 IO）。→ pi/hermes 的 nil-ctx 缺口见 P2-13 |
| **竞态候选（共享 map 无锁）** | **0 处**。`.threadMap` 11 处命中全在访问器内（`zcode.go:759-760` 声明的不变量**成立**）；`.cursors` 6 处命中中 2 处直接 `delete`（`pi.go:490` / `pi.go:1888`）**都在 `p.mu.Lock()/Unlock()` 保护内**（已逐行核对）；`compactState` 全在 `compactMu` 内；`pendingResolved` 全在 `pendingReqMu` 内 |
| **channel 泄漏** | 0 处（带缓冲 + 关闭路径齐全） |
| **`time.Ticker` 不 Stop** | **0 处**。4 个 `NewTicker`（`codex.go:1739` / `hermes.go:507` / `openai_compat.go:254` / `zcode.go:2113`）**全部下一行就是 `defer tick.Stop()`** |
| **agent 子进程泄漏（没 Wait 就返回）** | **0 处**。`exec.Cmd` **完全不在本域**（在 `*app` 包，另一条线）。本域侧的对应物是 `p.gen`（`atomic.Int32` 代号）+ `p.gen.Add(1)` 记账，共 8 个 `Add` 点（codex 512/528、dsh 423、zcode 425/445/464、zcode_selection 54），每处紧跟 `Close()`；`DropThread`（`pi.go:486-497`）显式注释「进程+游标泄漏」并做 `c.Close()`；`runtime_descriptors.go:276-285` pi 的 `DeleteNativeSession` 内含 `DropThread` |

### 6.4 测试整洁

**testagent / testkit 覆盖了什么，agent 域有没有重复造第三套？**

| 件 | 大小 | 覆盖 | agent/runtime 域采用情况 |
|---|---|---|---|
| `internal/testagent/` | `fake_agent.go` 31 KB + `fake_agent_test.go` 21 KB | deterministic fake agent，**实现 `agent.TestFakeSurface`**（`provider.go:134`），并在 `fake_agent.go:337` 做镜像编译期断言 `_ agent.ErrorInfoCarrier = (*ExitError)(nil)` | ✅ 采用面是 server 层；**agent 包内测试不用它**（合理：agent 测试打的是单个 provider 的窄接口面，不是 server 的 provider 表） |
| `internal/testkit/` | 6 文件 / 5.8 KB | 轮询等子进程退出、轮询等 fake 产出文件、劫持 `log` 抓输出、`AssertNoDeveloperPaths`（查源码不含开发机路径） | ❌ **agent/runtime 域 0 引用**（全仓 6 个文件引用，都在 `*app` 与 eventstore） |

**结论：agent 域没有造第三套 testkit**，因为它的测试形态是**进程内 fake（`fakeDshClient` / `fakeHermesClient` / `fakeCodexCaller` …）+ 窄接口桩**，不需要「等子进程」「劫持 log」这类平台 helper。`AssertNoDeveloperPaths` 这一条**值得跟进**：全仓只有 6 个文件在用，而 agent 域测试里存在**硬编码开发机路径依赖**（`codex_goal_test.go:141` `G:\codex-schema\v2\ThreadGoalSetParams.json`、`codex_notice_schema_20261002_test.go:107` `G:\codex-schema\ServerNotification.json`）—— 这些测试在本机外必然失败。建议：`AssertNoDeveloperPaths` + 「schema 对照测试找不到快照则 `t.Skip` 并说明」两条一起补进 agent 域。（属 P2-12 同批）

**fake 桩清点**：agent+runtime 测试共 **37 个** fake/stub 类型。逐个分类：
- **协议面 fake（应留）**：`fakeDshClient`(15 方法)、`fakeHermesClient`(13)、`fakeZcodeSessionClient`、`fakeMountClient`、`fakeCodexCaller`… —— 每家协议面真不同
- **跨家重复（该收）**：`fakeCodexResponder` / `fakeZcodeResponder` → **P1-7**（唯一一组已证等价）
- **窄接口桩（该留）**：`piUIClient`/`fakePiUIResponder`（`map[string]any`，语义不同）、`selectionFake`/`piNotFoundFake`/`piReadbackFailFake`（`pi_selection_test.go` 三个变体，各测一个失败面，刻意分开可读）
- **命名 P2**：`sinkCollect` / `sinkSpy` / `sinkSpy`(`agent`) vs `sinkCollect`(`runtime`)? → 两包同名不同义（各包私有，不冲突），仅记录

**断言无消息比例：2996 个 assert/log 调用中仅 53 个（**1%**）无消息。** 最差文件 `current_selection_test.go` 2/3、`pi_sessions_test.go` 12/45。**这在本仓是优秀水平，不列债。**

**条件性跳过：全域仅 4 处**（`agent/native_validation_test.go:20/65/103/132`），全部是具名 env 开关（`PA_LIVE_ADAPTER_VERIFY=1` / `PA_LIVE_GENERATE_AGENT`+`PA_LIVE_GENERATE_MODEL`）且 `t.Skip` 消息写明如何开启。**零「条件性跳过」债。**

**magic number**：生产侧几乎全部具名（30 个 `*Budget/*Timeout/*Wait/*Cap/*Limit/*TTL` 常量，每个都带「为什么是这个值」的注释，如 `dsh.go:50-79` / `pi.go:46-79` / `zcode.go:58-69` / `hermes.go:603-612` / `capabilities_cache.go:28-30`）。**唯一的裸字面量聚集在 `codex.go` 的 Run 内**（`10*time.Second` tick / `150_000` ms / `60*time.Minute`）→ **P1-2**。测试侧有 `4_000` / `8_000` / `2000` 等截断长度散落，属可接受。

**超长测试函数**：仅 2 个 >150 行 —— `codex_effect_test.go:75` `runFakeCodexAppServer`（312 行，**假 app-server 装配器**）与 `capability_gate_test.go:158` `TestCapabilityStanceTableMatchesMatrix`（179 行，**矩阵对表**，甲级禁区）。两者都是「装置」而非「用例堆叠」，可接受。`dsh_test.go` 1072 行 / 25 个用例（单一 fixture `fakeDshClient`）—— 排版密集但职责单一，**不列债**。

---

## 7. 伪命题澄清（本轮最有价值的产出形态之一）

> 规则：R1 轮量化 4510 函数 / 28 组重复 body 并全判「不收」。本轮遇疑似重复**必须先证明语义等价**才列，否则归入本章。**以下 4 组经逐句对账后确认语义不等价，不收。**

### 伪命题 1｜「五家各写一遍 `withClientCtx`，是复制粘贴债」——**不成立**

表面证据极强：三份 `withClientCtx`（`codex.go:458` 74 行 / `dsh.go:312` 76 行 / `zcode.go:370` 98 行）结构逐段对仗：都做「取 nil ctx → 兜底 deadline → `try()` 闭包（初始化 + fn）→ 超时/取消直返 → `turnActive` 守卫 → 首次拉起 → 重启一次」。

**逐项对账后差异是语义级的，不是风格级的**：

| 维度 | codex | dsh | zcode |
|---|---|---|---|
| 锁获取 | `TryLock` 立即 `ErrBusy` | 有界排队 + **`awaitHandshake` 握手礼让**（第二次抢锁） | 有界排队，直接 `ErrZcodeBusy` |
| 初始化 | `CallCtx("initialize", …)` 显式 JSON-RPC | `client.Initialize(ctx)` + **回写 `p.caps`** | **无初始化握手**（靠 startup/storageState 通知自推进到 ready） |
| 错误分类 | 仅 `DeadlineExceeded/Canceled` 直返 | 同 codex | 同上 **+ `errors.As` 排除 `RPCError`/`AccountSyncError`**（native 错 ≠ 进程死） |
| 拉起后 | — | — | **闪退重试一次**（`zcode.go:428-439`，真机实证零 stderr 偶发） |
| 收尾 | — | — | `attachUnknownResponseHandler` + `attachGlobalHandler` |

抽成泛型骨架会得到一个 8 参数、6 个策略钩子的万能函数 —— **比三份各 80 行的专用实现更难读、更易错**。这是 R1 轮结论在本域的再次确认：**进程生命周期骨架的可复制性不等于可抽象性。**

### 伪命题 2｜「五家各写一遍模型/思考强度切换（`applyModel` 族），该收」——**不成立**

`pi.go:559 applyModel` / `zcode.go:644 applyModel` / `dsh.go:1224 applyConfigOptions` / `codex ConfigOverride` + `ConfigOverrideFor`。四份读起来都像「把 model/effort 灌给上游」。

**实到是四条完全不同的协议路径**：codex 把 `model_verbosity`/`model_reasoning_summary` 放进 `thread/start` 的 `config` 覆盖 map（**启动时给，不是运行期切换**）；pi 调 `piapp.SetModel` + `set_thinking_level` 两个 RPC；zcode 要先 `session/resume` 回读当前模型快照再 `set_config_option`（`zcode.go:653-664`）；dsh 走 ACP `set_config_option` 且**仅会话内生效不持久**（`native_config_options.go:17-21` 有实证记录）。连失败语义都不同：pi 失败只 log 沿用旧模型，zcode 失败返回错误并校验实际生效（`zcode_selection.go:212` 「ZCode 未确认实际 mode」）。

**收不了。** 这是「同一个业务动作，五种上游契约」的必然形态，不是复制粘贴。

### 伪命题 3｜「`dshRunError`/`codexRunError`/`piError` 三个 `ErrorInfoCarrier` 实现是重复，该收成一个」——**不成立**（但**缺口那一半是 P1-1**）

三个 `Error()`/`ErrorInfo()` 长得像，但承载的诊断维度完全不同：`dshRunError` 编码 ACP 的 `MISSING_CREDENTIAL`（`dsh.go:1788` `dshRunErrorFrom`，探针实证「缺 key 错误 → MISSING_CREDENTIAL」）；`codexRunError` 编码 `codexErrorCode`/`codexHTTPStatus`（`codex_events.go:74-75`）；`piError` 编码 pi 的 `PI_FORK_FAILED` 等（`pi.go:2052-2054`）。**强行统一成一个泛型错误会把三套不同错误分类糊在一起** —— 那正是本仓红线 4「不新增写死的名单」和「不伪造」要防的反面。

真正该收的是另一半：**zcode / hermes 两家根本没实现**（P1-1）。不是「三个实现重复」，是「五个 provider 里两个缺席，且缺席无声」。

### 伪命题 4｜「`clip` / `clipRunes` / `mustJSON` / `firstNonEmpty` 这些工具件应该统一到 runtime 或 testkit」——**不成立**

- `clip`（`openai_compat.go:452`，保字节预算回退 rune 边界，防 CJK 半字符）与 `clipRunes`（`pi_events.go:909`，委托 `runtime.ClipRunesEllipsis`）**同名不同义，但各自的注释都写明了为什么**（前者 `:453-455`「修复 CJK 半字符缺陷」，后者 `:911-912`「收敛见审查报告 util 节」）。真问题是**命名**（P2-1），不是**抽象**。
- `mustJSON` / `firstNonEmpty` 是 agent 包内的小件，提到 runtime 会让 runtime 反向依赖 pb/agent 的形状，方向错。
- 「测试基建该统一到 testkit」这个命题在本域**已被证伪**：agent 测试是进程内窄接口桩，testkit 是平台进程 helper（`testkit/doc.go` 写明「与被测包的协议毫无关系」）。**没有第三套可收。**

---

## 8. 已查但干净（本轮最有价值的产出形态之二）

### 干净 1｜两条绝对红线逐条对账 —— **零违反**（红线 1 完全干净，红线 2 见 P0-1 待裁决）

**红线 1「绝不替用户做审批决定」** —— 四条审批路径全查：

| 路径 | 通道缺失 | 通道故障/断流 | 超预算 | 选项表认不出 |
|---|---|---|---|---|
| codex `handleCodexApproval`(`codex.go:988`) | `:991-994` 回 `-32000`「审批通道不可用」 | `:1050-1055` 回 `-32000`「审批通道失败」，**注释明写「这不是用户拒绝…别把基础设施问题伪装成用户意志」** | `:1035-1046` 回 `-32000`，**注释明写「超时 ≠ 用户拒绝」** | n/a |
| dsh `handleRequestPermission`(`dsh.go:1661`) | `:1694` 回 `-32000` | `:1681` 回 `-32602` | `:1731/1735` **默认分支一律 reject** | `:1685-1688` **显式报错「拒绝代答」** |
| hermes `handleRequestPermission`(`hermes.go:961`) | `:986-987` 回 `-32000` | `:1013-1014` 按拒绝处理 | `:1017/1021` 默认分支一律 reject | `:978-981` **显式报错「拒绝代答」** |
| zcode `handleZcodePermission`(`zcode.go:1379`) | `:1391` 回 `-32000` | `:1425` 回 `-32000` | — | — |

- 全域 `autoApprove|defaultApprove|approve\s*=\s*true|代答|自动批准` 扫描：除上述两处**拒绝**代答的注释外，**0 命中**。
- 未知请求透传路径（`unknown_passthrough.go:179-195`）同样失败安全：`askErr != nil`（含超预算）→ `return true, false`（按拒绝）；`!allow` → 拒绝。
- `hermes_compact.go:24` 显式声明「审批通道:失败安全,绝不代答」。
- **`codex.go:984`「绝不替用户伪造 denied 决定」、`codex.go:1075`「这个应答没有 decision 字段:不同意的表达方式就是什么都不授予」** —— 这是对红线的**理解级落实**，不是口号。

**红线 2「绝不改写用户桌面配置」** —— `~/.codex` / `~/.pi` 的访问点全量清单：

| 路径 | 读 | 写 | 判定 |
|---|---|---|---|
| `~/.codex/config.toml` | 0 | **0** | ✅ 完全守住（4 处显式红线声明：`codex.go:116/1152/1909/1918`） |
| `~/.codex/sessions/**` | ✅ `codexsessions.go:42/43` 扫描、`current_selection.go:48` / `native_recovery.go:31` 定位、`codex.go:1824` | ⚠️ **`codexsessions.go:150` `os.Remove`** | **P0-1（待裁决）** |
| `~/.pi/agent/sessions/**` | ✅ `pi_sessions.go:53/54` | ⚠️ `pi_delete.go:43` `os.Remove` | 同 P0-1；守卫完整 |
| `~/.pi/agent/models-store.json` | ✅ `pi.go:393-397` | 0 | ✅ |
| `~/.pi` 全局配置 | 0 | 0 | ✅（`pi_config_options.go:90` / `pi_followup.go:11` 显式记录「实测那些调用会落盘用户 ~/.pi 全局配置」故不走） |
| `~/.dsh` | 0 | 0 | ✅（`dsh.go:398`「凭据文件只在 daemon 数据目录内(默认 ~/.pocketagentd),不碰用户 ~/.dsh」） |
| `~/.zcode/cli/db/db.sqlite` | **0** | 0 | ✅（`zcode_sessions.go:5` 如实说明「为什么走协议而不是读」） |
| `runtime` 域全部路径 | — | — | ✅ **0 处 home 引用**（`memory_inventory.go:34` 明确写「`~/.codex`、`~/.pi` 等原生侧数据从头到尾不进本文件的任何路径」，已核属实） |

**唯一需要裁决的就是 P0-1 那一条**，其余全部守住，且是**用注释固化住的**（不是靠自觉）。

### 干净 2｜零死代码（双向计数）+ 零 goroutine/子进程泄漏 + Ticker 4/4 有 Stop

三个「本仓最易藏债处」全部经量化核验为 0：

- **死代码：agent 0 / runtime 0**（双向计数，见 §6.2）。这在 206 文件 / 1.9 MB 的规模上是异常干净的。
- **goroutine 泄漏：0**（28 个 `go` 站点逐个核出口路径），**子进程泄漏：0**（本域无 `exec.Cmd`，`p.gen` 记账 8 点全有 `Close()` 配对）。
- **`time.Ticker` 无 Stop：0/4**；**context 硬写 `Background()`：7 处全部有注释解释**；**共享 map 无锁：0**（`zcode.go:759` 声明的「threadMap 唯一读写入口」不变量**经全量核对成立**）。

配套的「诚实但有代价」的设计也一并核过：`dshClient`(15 方法) / `hermesClient`(13 方法) 是刻意为可测性付出的接口面代价，有编译期钉死，**不是债**。

### 干净 3｜测试整洁度全项达标，TODO/注释掉的代码全域归零

- **TODO/FIXME：生产 0 条**（唯一命中是反讽说明）。
- **注释掉的代码：0 处**（12 个候选逐一复核，11 个是带括号的散文，1 个是 godoc 用法示例 `<pre>` 块）。
- **断言无消息率：1%**（53/2996），全域最差文件也只有 2/3。
- **条件性跳过：4 处，全部是具名 env 开关**且 `t.Skip` 消息写明如何开启 —— 零「条件性跳过」债。
- **超长测试函数：2 个 >150 行，均为「装置」**（假 app-server 装配器 / 矩阵对表），非用例堆叠。
- **magic number：生产侧 30 个预算/超时常量全部具名带注释**，唯一裸字面量聚集在 codex Run 内（已列 P1-2）。
- **TODO 归属**：无孤儿、无时间戳缺失问题（因为压根没有 TODO）。

---

## 9. 已知工具局限（写给下一个人，别重复踩）

1. **函数长度靠花括号配对统计，必须复核表达式体函数**。本域 Go 的 `func x() expr` 表达式体极少，但裸 `}`（字符串 / 复合字面量）会虚报行数。本轮**逐个人工复核了 Top 3**：`codex.go:1104 Run` 声明 708 行 → 实测 `:1104` 起、`:1811` 为 `}`（`return blocks…` 在 `:1810`）✅；`pi.go:855 Run` 声明 500 行 → `:1354` 为 `}` ✅；`zcode.go:1814 Run` 声明 370 行 ✅。**Top 20 内无虚报。**（android 域那轮因未复核虚报 1108 行 —— 本轮的对应风险已排除。）
2. **`_ = ` 吞错误扫描会命中所有 `_, _ =`**，其中 `_, _ = p.client.Call(...)` 是**有意忽略**不是漏处理。必须按调用语义分类，不能计数即列债（本轮 117 处 → 99% 无害，见 §6.1）。
3. **私有符号引用计数不能用 `(?<![\w.])`** —— 会把 `p.foo()` 这类带接收者的调用全排除，本轮初版因此虚报 **115 个「死代码」**，改成 `(?<!\w)` 后归零。**这是最容易致命的一个坑。**
4. **参数计数不能从第一个 `(` 开始切** —— 方法的**接收者**会抢走第一个括号组。初版把 `handleCodexApproval` 的 11 个参数算成 1 个。必须先跳过 receiver 再切。
5. **嵌套深度不能用「花括号跳到配对位置」的启发式** —— 本轮初版给出 `codex.go:1104 Run` depth=63 的荒谬值（`cur` 只增不减）。**在 gofmt 过的 Go 里，行首 tab 数 = 嵌套层级**，改用缩进后：prod 中 indent ≥7 tab 的只有 **4 个**函数，≥6 tab 的 93 个。**深嵌套在本域不是债。**
6. **注释掉的代码扫描假阳性率高**（散文带括号即命中）。本轮 12 候选 → 11 误报，且唯一那个（`backend.go:194` 的 `//\trt := backend.FullRuntime(...)`）是 **gofmt 的 godoc `<pre>` 代码块**，是**正确写法**。判据：缩进代码块 + 上方是函数头注释 = 示例，不是注释掉的代码。
7. **路径分隔符假设会让「跨包引用」统计全错**。本轮初版用 `f"\\internal\\{pkg}\\"` 判断包归属，在 Windows 原生路径下错配，报出 `DetectProviders` 零引用（实际 server/host.go:102 等 3 处）。**每个「无引用」结论都必须用独立的直接 grep 复核。**
8. **ASCII junction 会让测试假红**：`G:\pa_c1b` → `daemon` 下跑 `go test` 得 5 条 FAIL（`../../../docs/…` 与 `G:\codex-schema\**` 相对/绝对路径依赖），真实路径下 0 条。**报基线必须在真实中文路径跑**；junction 只用于需要 ASCII 路径的场合（如 Gradle），用完即删。
9. **per-agent 分支统计不要只 grep 字面量**。`Select-String -Pattern '"codex|pi|zcode|...'` 会把六家一起计入，得到「六家都 151 处」的荒谬一致数（本轮踩过一次）。**必须按 agent 逐个跑。**
10. **甲级禁区（能力位生成与位族词汇表）在本域的落点**：`agent/codex_capabilities.go`、`agent/capabilities_cache.go`、`agent/runtime_descriptors.go`、`agent/image_input_capability.go`、`runtime/knowledge_*.go`、`runtime/event_translate.go` 的词表、`agent/capability_gate_test.go`。本轮对这些只做只读观察，**未列任何「建议改」**（§3 矩阵第 18/19 行已标注）。
11. **并行线在本域文件上落笔 —— 本报告的 hermes.go 行号是「复核后的当前树」**。审查进行到 §6 时，`git status` 显示 `daemon/internal/agent/hermes.go` 已被**另一条 daemon 线**修改（+18/-5，与它自己拥有的 `hermesapp/config.go` + `hermesapp/locate.go` 配套：`SetReasoningEffort` 改为只写 `%HERMES_HOME%/config.yaml`、未设则拒写，并新增 `reasoning_effort_not_pinned` 的诚实缺席 STATUS 帧）。该改动落在 `Run` 内 `:312-340`，**导致 `:340` 之后所有行号整体下移 +13**。本报告已把全部 20 处 `hermes.go:*` 引用按**当前工作树重新核对并修正**（例：`handleRequestPermission` `:948`→`:961`；`hermesSilenceWatchdog` `:591`→`:604`；`setThreadRef` `:1040`→`:1053`）。**若该改动后续再变，本报告 hermes.go 侧的行号需要再核一次**；其余文件（codex/pi/zcode/dsh/runtime）本轮全程未被改动，引用稳定。
    - ⚠️ **范围重叠提示**：`daemon/internal/agent/hermes.go` 属本线声明的审查范围，但被 apps 线实改。建议主线在合并三条线的结论时，单独复核这一处 —— 本报告对 hermes 的判断（审批失败安全 ✅、`cancelTurn` ✅、`threadRef` 访问器形态 P2-7、无 `if ctx == nil` 守卫 P2-13）在改动后仍成立，但 P2-13 现在多了一条新线索：改动后的 `Run` 在写强度失败时走 `sink(STATUS)` 而非静默，说明作者已注意到「诚实缺席」纪律，该纪律与本报告 §8 干净 1 的审批口径一致。

---

## 9.1 自查：本报告自身的边界

- **未修改任何代码/文件**：全程只有 `read` / `grep` / `shell` 只读命令（`go vet`、`go test`、`Select-String`、`Get-Content`）。唯一写入是本报告 `docs/reviews/2026-10-17/clean-daemon-agent.md`。
- **未跑 `gofmt -w`**（一次都没跑过 `gofmt`，连只读 `-l` 也没跑，避免任何写入风险）。
- **未做任何 git 写操作**（无 `add` / `commit` / `push` / `checkout` / `stash`）。`git status --porcelain` 与 `git diff` 只用于**只读确认**我没有动过文件。
- **临时脚本全部在仓库外**：`C:\Users\yanzi\AppData\Local\Temp\opencode\{golex,a1_funcs,a2_naming,a3_comments,a4_errors,a5_params,a6_conc,a7_dead,a8_fakes,a9_iface,a10_tests,a11_switch,a12_nest,a13_codexwd,a14_claims,a15_verify,a16_deadexp,a17_deadexp2}.py`。
- **临时 junction 已删**：`G:\pa_c1b` 已 `.Delete()`，实测 `Test-Path` 返回 False。
- **测试基线为只读运行**，`GOCACHE` 指向仓库外临时目录，未污染仓库。
- **甲级禁区零建议改**：能力位生成面与其位族词表，本报告只标注（§3 矩阵第 18/19 行、§7 第 10 条）。

---

## 10. 建议处置顺序

| 序 | 条目 | 代价 | 风险 | 备注 |
|---|---|---|---|---|
| 1 | **P1-2** codex 看门狗 90/150 秒谎言注释 + 三个裸字面量提常量 | ~10 行 | 零 | 最便宜、影响最广（另外三家都拿它当基准） |
| 2 | **P0-1 的守卫对齐部分** `safeNativeSessionDelete` 共享 helper | ~30 行 | 零 | 不预设裁决结论先把闸补齐；红线的语义裁决并行推进 |
| 3 | **P1-1** `ErrorInfoCarrier` 编译期断言块（先只加断言，不加实现） | ~6 行 | 零 | 断言一加，zcode/hermes 的缺口立刻在 CI 里变红，**范围自然收敛**；第二步（补实现）再单独排 |
| 4 | P1-4 / P1-5 删陈旧占位注释 + 修悬空文档引用 | ~3 行 | 零 | 可与 1-3 同一 commit |
| 5 | P1-6 哨兵改 `errors.Is`（4 处） | 4 行 | 零 | |
| 6 | P1-3 `record` 死代码与谎言注释收尾 | ~20 行 | 零 | |
| 7 | P1-7 responder 假件收编 | ~25 行净减 | 零 | |
| 8 | P1-8 `CodexProvider` 5 簇状态拆出（分 5 个 commit） | ~150 行 | 中低 | 每簇独立可回滚 |
| 9 | P2-1/P2-2 `clip` 改名与遮蔽 | ~34 行 | 零 | |
| 10 | P2-3/P2-4/P2-6/P2-8/P2-9 杂项 | ~35 行 | 零 | |
| 11 | P2-5 / P2-10 / P2-11 / P2-13 风格项 | — | 低 | 建议合并成一次「命名一致性」commit，不单独排期 |
| 12 | P2-12 / testkit 跟进 | — | 跨域 | 需与 server 线、Android 线对一次账 |

**不建议做**：把 5 家 provider 拆成 `internal/agent/codex` 等子包（改动量从乙级跳甲级，且与另两条 daemon 线产生文件耦合）；改 `runtime/knowledge_*.go` 词表（甲级禁区）；改 `pb.Error` 字段（proto 甲级）。**本报告未运行任何 `gofmt -w`，未修改任何文件，未做任何 git 操作。**