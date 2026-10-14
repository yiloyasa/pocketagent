# daemon/internal/agent 代码收敛报告（2026-10-14）

> 任务：对 `daemon/internal/agent/` 做纯收敛重构 —— 消除重复与冗余，业务语义零改变。
> 约束：只改 `internal/agent/` 下 `*.go` 与 `*_test.go`；不动 `internal/` 其他包、`cmd/`、
> `flutter/`、`android/`、`proto/`；**不 commit / 不 add / 不 push**（主会话统一提交）。
> 等价优先：错误文案、日志字段、时序、位生成结果一律不改；拿不准的进 §3「已识别未动」。
> 测试零删除、零跳过、零弱化；不新增写死名单（红线 4）。

---

## 0. 基线与最终验证（测试证据原文）

**基线（动手前，绿）：**

```
C:\Go\bin\go.exe build ./...
    → exit 0（BUILD_OK）
C:\Go\bin\go.exe vet ./internal/agent/
    → 无输出（VET_OK）
C:\Go\bin\go.exe test -count=1 ./internal/agent/
    ok  github.com/pocketagent/daemon/internal/agent   17.705s
C:\Go\bin\go.exe test -v -count=1 ./internal/agent/    （盘点阶段）
    toplevelPASS=621  subPASS=74  FAIL=0
```

基线用例水位：**621 个顶层测试 PASS + 74 个子测试 PASS，0 FAIL**。全绿后才开始改动。

**逐项复跑（每完成一项就 build + vet + test 一次）：**

| 完成项 | `go build` | `go vet ./internal/agent/` | `go test -count=1 ./internal/agent/` |
|---|---|---|---|
| 基线 | exit 0 | 无输出 | ok 17.705s |
| 项 1 `lockLife` | exit 0 | 无输出 | ok 17.481s |
| 项 2 台账信封 | exit 0 | 无输出 | ok 16.949s |
| 项 3 死代码删除 | `build ./...` exit 0（跨包零引用的编译期证明） | 无输出 | ok 16.975s |
| 项 4+5 思考流/开关 | exit 0 | 无输出 | ok 16.792s |
| 项 7 测试 testkit | `./internal/...` exit 0（见下方异常说明） | 无输出 | ok 17.105s |
| 项 6 `ApplyPrefs` | `./internal/...` exit 0 | 无输出 | ok 17.198s |

> 上表按**落地先后**排列（项 7 的 testkit 先于项 6 的 `ApplyPrefs` 完成）；项号与 §1/§2 的
> 编号一致。

**最终（7 项全部完成，绿）：**

```
C:\Go\bin\go.exe build ./...
    → exit 0
C:\Go\bin\go.exe build ./internal/...
    → exit 0
C:\Go\bin\go.exe vet ./internal/agent/
    → 无输出（干净）
C:\Go\bin\go.exe test -count=1 ./internal/agent/
    ok  github.com/pocketagent/daemon/internal/agent   16.574s
C:\Go\bin\go.exe test -v -count=1 ./internal/agent/
    test_exit=0
    toplevelPASS=621 subPASS=74 RUNtotal=741 FAIL=0
    ok  github.com/pocketagent/daemon/internal/agent   16.574s
C:\Go\bin\go.exe test -race -count=1 ./internal/agent/
    ok  github.com/pocketagent/daemon/internal/agent   18.411s
```

**用例数与基线逐位一致（621 + 74，0 FAIL），另有 `-race` 全绿。**

**期间一次异常（非本任务引起，如实记录）**：测试 testkit（项 7）完成后的某次 `go build ./...` 报

```
cmd\capprobe\main.go:214:14: cannot use noteRec{…} (value of struct type noteRec) as rpcMsg value in argument to c.addNote
```

该文件属并行子代理正在改的 `daemon/cmd/`（几秒后行号自己从 214 跳到 219，可证在被实时编辑）。
当时 `go build ./internal/...` 与 `go vet ./internal/agent/` **均绿**，本包测试 ok —— 本任务判据
`go test ./internal/agent/` 全程未受污染；数分钟后重跑 `go build ./...` 已恢复 exit 0（上表最终行）。

**gofmt 状态**（`C:\Go\bin\gofmt.exe -l internal/agent`）：

```
internal\agent\codex_approval_wait_budget_20261009_test.go   ← 本轮未触碰
internal\agent\dsh.go                                         ← 本轮改过，偏差 hunk 不在改动行块（见下）
internal\agent\hermes.go                                      ← 同上
internal\agent\hermes_pump_badframe_20261009_test.go          ← 本轮未触碰
internal\agent\pi_events.go                                   ← 本轮未触碰
internal\agent\zcode.go                                       ← 本轮改过，偏差 hunk 不在改动行块（见下）
```

- 本轮**新增**的 4 个文件与改得最重的 `pi.go`、`codex.go`、3 个测试文件：**gofmt 干净**。
- `dsh.go` 的 3 个偏差 hunk 在 `@@ -760`（notif 泵 `select` 块整体多一层缩进）、`@@ -1050`
  （doc 注释块内插空 `//` 行）、`@@ -1452`（`dshToolCallUpdate` 结构体 tag 对齐）；
  `hermes.go` 在 `@@ -598`、`zcode.go` 在 `@@ -1031` —— 都是 doc 注释/结构体对齐类偏差，
  与本轮改动的行块（各文件的 `lockLife` / `hideReasoning` / `ApplyPrefs` / 台账信封 / `emitThinking`）
  **无交集**，判定为既有偏差。本轮**不顺手格式化**：那是独立的改动面，交给主会话决定。

---

## 1. 盘点清单（按「重复量 × 安全度」排序后的执行序）

盘点方法：
- `inv_funcs.py` → `inv_funcs.tsv`：1825 个顶层声明的全仓引用计数（含 server/cmd 跨包引用）；
- `dup_funcs.py` / `dup_body.py`：函数体归一化（去注释、压空白）后哈希找跨文件同体；
- `near_dup.py`：同名函数近似度（0.89–0.99）配对；
- `lit_dup.py`：字符串字面量重复统计；
- `dead_check.py` + 人工全仓 grep（`Ready` / `Account` / `applyDesktopDefaults` / `setThinkingLevel`）；
- 重建脚本 `restore_before.py`：把本轮 body 级改动逆向还原到临时目录，用**同一把尺**量收敛前后。

| # | 收敛项 | 站点数 | 重复正文 | 安全度 | 结论 |
|---|---|---|---|---|---|
| 1 | `lockLife` 有界排队取锁（dsh/hermes/zcode 三份逐字节相同） | 3 | 18 行 ×3 | 高（体逐字节相同，方法保留为一行委托） | ✅ 已做 |
| 2 | UNKNOWN_REQUEST 台账信封 + Note 字面量（codex/dsh/hermes/zcode 四份） | 4 | 7–10 行 ×4 + 同一长字面量 ×4 | 高（字段与文案逐字一致；`thread_id` 留空语义不变） | ✅ 已做 |
| 3 | 死代码：`CodexProvider.Ready` / `Account`、`PiProvider.applyDesktopDefaults` / `setThinkingLevel` | 4 函数 | — | 高（全仓 grep 零引用 + `build ./...` 编译期证明） | ✅ 已做 |
| 4 | `emitThinking` / `emitThinkingFor`（pi/zcode 四个方法同体） | 4 | 6 行 ×4 | 高（方法保留，签名不变，测试直接调用） | ✅ 已做 |
| 5 | `hideReasoning`（dsh/pi/zcode 同体 + hermes 仅锁名不同） | 4 | 5 行 ×4 | 中高（锁本体各留各的，只收"加锁→读字段→解锁"样板） | ✅ 已做 |
| 6 | `ApplyPrefs` 整快照覆盖（dsh/pi/zcode 同体 + hermes 仅锁名/参数名不同） | 4 | 5 行 ×4 | 中高（同上；codex 结构不同，不并） | ✅ 已做 |
| 7 | 测试台账 helper：`ledgerEntries` / `findLedgerEntry` / `findHermesLedgerEntry` | 3 | 7–10 行 ×3 | 高（Fatalf 文案逐字保留，函数名/签名不动） | ✅ 已做 |
| — | `threadRef` 读（codex/dsh/hermes/zcode）+ 写（codex/dsh/zcode）+ `stashGoal`（codex/zcode） | 4+3+2 | 5 行 ×9 | **中低**：helper 必须把 map 当值传入 → 埋下"字段被重赋值后写错 map"的隐性前提 | ⛔ 已识别未动（§3.1） |
| — | codex `HideReasoning` / `ApplyPrefs` | 2 | — | 结构本就不同（`setMu` + `hideAgentReasoning` 拆散字段，非整份快照） | ⛔ 已识别未动（§3.2） |
| — | 五家 provider 接口同名方法（`ID`/`DisplayName`/`Models`/`Checkpoint`/`Close`/`Source`/`NativeSessionFor`/`withClient`…） | ~20 | — | 接口面 + 各家语义差异，合并会抹平差异 | ⛔ 已识别未动（§3.3） |
| — | pi 两处「未知请求」台账 Note（`cancelled:true` 路径，文案与 -32601 路径不同） | 2 | — | 文案语义不同（协议无 -32601 通道），合并即改文案 | ⛔ 已识别未动（§3.4） |
| — | `desktopEffortFor` pi/zcode 回退实现不同（`desktopDefaults` vs `desktopEffort`） | 2 | — | 回退口径不同是既有声明，合并即改语义 | ⛔ 已识别未动（§3.5） |
| — | dsh `"session/update:"` 前缀字面量 | 15 | 15 字 ×15 | 中（跨 `dsh.go` 与 `native_notifications.go` 两文件，属另一轮接线面） | ⛔ 已识别未动（§3.6） |
| — | 测试侧 `UnknownEvents().List(...)` 过滤循环 | ~28 | — | 过滤维度（AgentID/ThreadID）与断言形状各异，只收敛了语义完全相同的 3 个 | ⛔ 已识别未动（§3.7） |
| — | gofmt 既有偏差 | 6 文件 | — | 独立改动面，不在"收敛"范围 | ⛔ 已识别未动（§3.8） |

**重复体量化（同一把尺，可复现）**：`dup_body.py`（body-only、去注释、压空白、跨文件、长度阈值 60）：

```
收敛前（restore_before.py 逆向重建）:  total funcs: 1500 ; body-only duplicate groups (len>=60): 10
收敛后（当前树）                      :  total funcs: 1500 ; body-only duplicate groups (len>=60): 6
```

| 收敛前 10 组 | 收敛后 6 组 |
|---|---|
| `lockLife` ×3（dsh/hermes/zcode，231B） | ✅ 已消除 |
| `emitThinkingFor` ×2（pi/zcode，155B） | ✅ 已消除 |
| `emitThinking` ×2（pi/zcode，151B） | ✅ 已消除 |
| `hideReasoning` ×3（dsh/pi/zcode，72B） | ✅ 已消除 |
| `ApplyPrefs` ×3（dsh/pi/zcode，56B，阈值 60 以下另计） | ✅ 已消除 |
| `codexThreadRef`/`dshThreadRefFor`/`threadRefFor` ×3（87B） | ⛔ 保留（§3.1） |
| `setCodexThreadRef`/`setDshThreadRef` ×2（67B） | ⛔ 保留（§3.1） |
| `stashGoal` codex/zcode ×2（73B） | ⛔ 保留（§3.1） |
| 测试：`runImageTurn` ×2（316B） | ⛔ 保留（§3.7） |
| 测试：fake `RespondError` ×2（121B） | ⛔ 保留（§3.7） |
| 测试：`textBlocks` ×2（88B） | ⛔ 保留（§3.7） |

> 说明：阈值放宽到 40 时，收敛后会**新出现**几组"同体"，那是**一行委托本身**
> （如 `return hideReasoningFrom(&p.prefsMu, &p.prefs)`）—— 收敛的目标是消掉多行样板，
> 同一 helper、同一实参的一行调用形态相同是设计结果，不是残留重复。

**字面量重复**（`lit_dup.py`）：Note 长字面量 `"已回协议错误 -32601(安全拒绝;未替用户做任何审批决定)"`
收敛前 ×4 → 收敛后 **×1**（仅 `unknown_request_ledger.go` 的 `unknownRequestRejectionNote` 定义）；
`"session/update:"` 在 `dsh.go` 仍 ×15（§3.6 未动）。

**函数数**：删 4 个死函数、增 7 个共享 helper，净 +3（`^func\s` 计数 1537 → 1540）。
**行数**：从既有文件删除 **165 行**（逐项计数见 §2），新增 **167 行**（4 个新文件），净 **+2 行** ——
收益不在行数，在"重复体 7 组生产侧 → 3 组"与"同一事实只写一处"。

---

## 2. 逐项记录（改动 + 等价论证 + 测试结果）

### 项 1：`lockLife` 三份同体 → 包级 `lockLifeMu`

**改动**：新增 `life_lock.go`（41 行）；`dsh.go:296` / `hermes.go:173` / `zcode.go:356` 的
`lockLife` 三份 18 行实现 → 各 3 行委托（现 `dsh.go:297` / `hermes.go:172` / `zcode.go:357`）：

```go
func (p *DshProvider) lockLife(wait time.Duration) bool {
	return lockLifeMu(&p.lifeMu, wait)
}
```

**等价论证**：
- 三份原体经 `dup_body.py` 归一化后哈希完全相同（bodylen=231、n=3），即逐字节同实现；
- 新实现是**同一段代码搬进一个函数**：`TryLock → wait<=0 早退 → deadline → 25ms 步进重试`，
  步进常量提为 `lifeLockRetryStep = 25 * time.Millisecond`，**值与原三处逐字一致，时序不变**；
- 锁对象仍是对各家 `p.lifeMu` 取地址（`sync.Mutex` 字段，地址稳定），锁的选型、粒度、
  归属一律不动；
- 方法**保留**：`zcode_lockwait_test.go` 直接调 `p.lockLife(...)` 断言"占锁→有界排队拿到 /
  窗口耗尽返回 false / wait<=0 立即 false"，签名与返回语义一字未改。
- 各家的忙哨兵文案（`dshBusy` / `errHermesBusy` / `ErrZcodeBusy`）与调用方日志**不在本次收敛范围**，全部留在原文件。

**验证**：`build` exit 0 / `vet` 无输出 / `test ok 17.481s`（zcode 锁等待用例全过）。

### 项 2：四家 UNKNOWN_REQUEST 台账信封 + 同一 Note 字面量

**改动**：新增 `unknown_request_ledger.go`（41 行），内含

```go
const unknownRequestRejectionNote = "已回协议错误 -32601(安全拒绝;未替用户做任何审批决定)"

func recordUnknownRequestRejection(threadID, agentID, method string, params json.RawMessage) {
	sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{ ... })
}
```

四处调用点（现 `codex.go:1626` / `dsh.go:1661` / `hermes.go:868` / `zcode.go:1930`）由手写信封改一行调用。

**等价论证**：
- 信封字段顺序与取值**逐字段一致**：`ThreadID / SourceAgent / SourceMethod / NormalizedKind /
  RawPayload / Note`；`RawPayload` 仍走 `redactNativePayload(params)`（脱敏不变）。
- `dsh` / `hermes` 原本**不写** `ThreadID` 字段（结构体零值 = `""`），helper 传空串 `""` ——
  与"不写该字段"在同一结构体上**取值完全相同**；它们"留空的原因"（一个调用点在握手路径，
  拿别的 id 冒充更糟）原封不动留在各自调用点注释里。
- `NormalizedKind` 恒为 `runtime.KindUnknownRequest`，四家原本各自写死同一常量。
- **Note 文案逐字一致**（原 4 处字面量经比对完全相同），现收敛为单一 const；
  断言侧 `dsh_unknown_request_ledger_test.go:56` 与 `hermes_unknown_request_ledger_test.go:52`
  只做 `strings.Contains(e.Note, "未替用户做任何审批决定")` —— 文案不变则断言语义不变。
- **调用结构不变**：codex 仍只在 `kind == reqUnknown` 时登记（`reqUnsupported` 不登记的口径保留）；
  zcode 仍带它"尚无 reqUnsupported 分类器"的原注释；日志行、`RespondError(-32601, msg)` 的
  顺序与文案都在调用点，helper 只管台账。
- pi 的两条 Note（`pi.go`、`unknown_passthrough.go`，走 `cancelled:true` 兜底）**刻意不并入**，
  文案与语义各自保留（§3.4）。

**验证**：`build` exit 0 / `vet` 无输出 / `test ok 16.949s`（四家台账用例全过）。

### 项 3：死代码删除（4 个函数）

| 删除对象 | 位置 | 死法证明 |
|---|---|---|
| `(*CodexProvider).Ready() error` | 原 codex.go:2130 | 全仓 `\bReady\(\)` / `Ready() error` grep **仅剩定义**；`server/model_ext.go:41` 接口只声明 `ReadyCtx`，`model_ext.go:154` 调的是 `probe.ReadyCtx(pctx)` |
| `(*CodexProvider).Account() string` | 原 codex.go:2152 | 全仓 `\bAccount\(\)` **仅剩定义**；`model_ext.go:163` 调 `probe.AccountCtx(pctx)` |
| `(*PiProvider).applyDesktopDefaults` | 原 pi.go:679 | 全仓 `applyDesktopDefaults` 仅剩**注释引用 + 定义**；活路径是 `pi_selection.go:114 configureSelection`（`pi.go:926` 在 `Run` 内每轮调用，`Run` 起于 `pi.go:855`）与 `applyModel` |
| `(*PiProvider).setThinkingLevel` | 原 pi.go:710 | **唯一调用点就是 `applyDesktopDefaults`**（pi.go:687/705），删掉它即成为新死代码，一并删；`applyModel` 自己内联了 `set_thinking_level` 调用（pi.go:569） |

**等价论证**：
- 死法用两道证明叠加：① 全仓（`daemon/` 含 server、cmd、gen、各 adapter 包 + 仓库根 `docs/`）
  正则 grep 零引用；② `C:\Go\bin\go.exe build ./...` **全模块编译通过** —— Go 的编译期未定义/未使用
  检查是比 grep 更硬的证据（接口方法若被需要，删除会直接编译失败）。
- 删的是**没有调用者的函数体**，运行路径、日志、时序、位生成一概不变。
- 三处指向已删符号的**注释**同步改写为活路径（否则留下悬空引用）：
  `pi.go` ApplyPrefs 注释 → 改指 `configureSelection`；`pi.go` `desktopEffortFor` 注释
  → 改指 `applyModel`（`configureSelection` 的 `t.Effort` 覆盖同理）；
  `pi_prefs_effect_test.go:7` 测试注释 → 改指「桌面默认模型只认 pi/ 前缀的依据」。
  **只改注释文字，不改任何判定/断言**。
- `codex.go` `ReadyCtx` 注释里的探测段名同步为 `ReadyCtx + AccountCtx + Requirements`
  （原文 `Ready + Account + Requirements` 指的就是这两个探测，非 ctx 版本已不存在）。

**验证**：`build ./...` exit 0；`vet` 无输出；`test ok 16.975s`。

### 项 4：`emitThinking` / `emitThinkingFor` 四个同体

**改动**：`pi.go:606`、`pi.go:640`、`zcode.go:945`、`zcode.go:952` 的 6 行体 → 一行委托
（共享实现 `emitThinkingDelta` 在 `prefs_access.go:46`）。

```go
func (p *PiProvider) emitThinking(delta string, sink Sink) {
	emitThinkingDelta(p.hideReasoning(), delta, sink)
}
```

**等价论证**：
- 闸门值在**调用点算**（`p.hideReasoning()` / `p.hideReasoningFor(t)`），共享函数只负责
  「hide=true 一帧不发，否则构造同一帧推给 sink」—— 判定时机与原实现相同（先取开关，再决定发不发）。
- 帧形状 `pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Delta: delta}}`
  与原四处逐字相同，收敛后**只有一处**，两家不可能再漂移。
- **方法全部保留且签名不变**：`pi_prefs_effect_test.go:40/47/57` 与
  `zcode_prefs_effect_test.go:24/31/41` 直接调 `p.emitThinking(...)` 断言
  「开关开时 ThinkingDelta 一帧不出 / 关时逐帧到达」。
- pi 的 `emitThinkingSnapshotFor`（`Snapshot` 字段，仅 pi 一家）**不并入**：它没有重复体，
  强行泛化只会把一个 5 行函数拆成两层。

**验证**：`build` exit 0 / `vet` 无输出 / `test ok 16.792s`。

### 项 5：`hideReasoning` 四份 → 共享读取 helper

**改动**：`dsh.go:1203` / `hermes.go:950` / `pi.go:591` / `zcode.go:907` → 一行委托
（`hideReasoningFrom`，`prefs_access.go:36`）。

**等价论证**：
- **锁本体各留各的**：dsh/pi/zcode 传 `&p.prefsMu`，hermes 传 `&p.mu`（其结构注释
  `hermes.go:58` 即 `mu sync.Mutex // prefs 锁`）—— **同一把锁、同一粒度**
  （`p.mu` 还同时守护 `turnActive`，见 §3.9，本轮不动），本轮**不改锁选型**；
- 读的都是 `*prefs` 的 `HideReasoning` 字段，同一字段、同一把锁、同样的
  `Lock → 读 → Unlock` 顺序，与原实现等价；
- `sync.Locker` 参数只是把"那四行样板"收走，不引入新锁、不改锁顺序、不跨锁嵌套；
- 方法保留：`pi_prefs_effect_test.go:51/74/84`、`zcode_prefs_effect_test.go:35/92/100`、
  `pi_zcode_global_override_semantics_test.go:34/38/43/72/75` 直接调 `p.hideReasoning()` /
  `p.hideReasoningFor(t)`，签名与语义不变；
- `hideReasoningFor` 的「全局优先、不读快照」口径（三家注释 + `TestHideReasoningIsGlobalAcrossSnapshots`）
  **一字未动**。

**验证**：`build` exit 0 / `vet` 无输出 / `test ok 16.792s`。

### 项 6：`ApplyPrefs` 整快照覆盖四份 → `applyPrefsSnapshot`

**改动**：`dsh.go:1199` / `hermes.go:148` / `pi.go:586` / `zcode.go:902` → 一行委托
（`applyPrefsSnapshot`，`prefs_access.go:27`）。

**等价论证**：
- 语义是"**整快照覆盖**（`*dst = src`，不逐字段合并）"，这是 PrefsAware 的既定纪律，
  收敛后这条纪律只有一处实现；写入仍在各家自己的锁内（`prefsMu` / hermes `mu`），锁不变；
- 各家 `ApplyPrefs` 的**文档注释原封不动**（它承载"消费哪些 / 不映射哪些、为什么"的边界说明，
  是这一轮真正有价值的部分）；
- codex 的 `ApplyPrefs` 写的是 `setMu` 保护的拆散字段（`hideAgentReasoning` 等），**不是**
  整份 `DesktopPrefs`，结构不同不并入（§3.2）；
- `provider.go:103` 的 `PrefsAware` 接口签名与 `provider.go` 的编译期断言未动。

**验证**：`build ./internal/...` exit 0 / `vet` 无输出 / `test ok 17.198s`。

### 项 7：测试台账 helper 收敛成 testkit

**改动**：新增 `ledger_query_testkit_test.go`（34 行）：

```go
func ledgerByMethod(agentID, method string) []runtime.EventEnvelope { ... }        // 全部
func firstLedgerByMethod(agentID, method string) (runtime.EventEnvelope, bool) ... // 第一条
```

三个既有 helper 保留**原名、原签名、原 `t.Helper()`、原 Fatalf 文案**，只把循环体换成调用：
`unknown_passthrough_test.go:35 ledgerEntries`、`four_provider_hooks_test.go:28 findLedgerEntry`、
`hermes_field_coverage_20261007_test.go:27 findHermesLedgerEntry`。

**等价论证**：
- 过滤条件逐字相同：`List(runtime.ListFilter{AgentID: agentID}, 0)` + `e.SourceMethod == method`
  （`findHermesLedgerEntry` 仍硬编码 `"hermes"`），返回顺序即 `List` 顺序；
- 找不到时的红字**逐字保留**：
  `t.Fatalf("agent %s 的钩子必须把 method=%s 记进诊断台账(钩子体→汇点断链)", agentID, method)`、
  `t.Fatalf("hermes 的钩子必须把 method=%s 记进诊断台账(投影兜底断链)", method)`；
- `ledgerEntries` 仍返回**全部**匹配（三个 `dsh` 台账用例靠它数条数），
  `find*` 仍返回**第一条** + 找不到即 Fatal —— 行为面没有并集化；
- 测试零删、零跳、零弱化，用例数 621+74 与基线一致。

**验证**：`vet` 无输出 / `test ok 17.105s`。

---

## 3. 已识别未动（含理由）

### 3.1 `threadRef` 读写 ×9 与 `stashGoal` ×2 —— 同体，但 helper 要付隐性契约

`codexThreadRef` / `dshThreadRefFor` / `threadRefFor`（+ `zcodeThreadRefFor`，仅局部变量名不同）、
`setCodexThreadRef` / `setDshThreadRef` / `setZcodeThreadRef`、`stashGoal` ×2 —— 语义完全同构，
`dup_body.py` 也把其中三组列为同体。**没做**的理由：这类 helper 必须把 `p.threadMap` /
`p.pendingGoals` **当值传进函数**，于是正确性依赖"这个字段永远不会被重新赋值"。
这不是空担心 —— `hermes.go:956-957` 就有 `if p.threadMap == nil { p.threadMap = ... }` 的重建先例；
今天 codex/dsh/zcode 全仓虽无重赋值（已 grep 确认），但只要将来有人加一行重赋值，
写入就会静默落到旧 map 上，且**没有任何测试会红**。4 行 ×9 的收益，不值这个隐性契约。
（对比项 5/6 传的是 `&p.prefs` —— 指向结构体字段的指针，不依赖"字段不重赋值"。）

### 3.2 codex 的开关结构不同，不硬并

`(*CodexProvider).HideReasoning()`（`setMu` + `hideAgentReasoning`）、`ApplyPrefs`（拆散字段赋值）
与另外四家的"整份 `DesktopPrefs` 快照"不是同一种东西，合并等于改数据结构与锁。

### 3.3 五家 provider 接口同名方法

`ID` / `DisplayName` / `Models` / `Checkpoint` / `Close` / `Source` / `NativeSessionFor` /
`withClient` / `InvalidateModels` 等在五家里同名不同体（`near_dup.py` 0.89–0.99 的配对基本都在这里）。
它们是**接口面 + 各家适配差异**，抽"通用体"必然把差异抹平，属行为级改变，红线挡住。

### 3.4 pi 的两条 Note 文案不同，不并入 -32601 常量

`pi.go:809`（确认在飞超限 → 协议原生兜底 `cancelled:true`）与 `unknown_passthrough.go:387`
（pi 的 `extension_ui_response` 无 -32601 通道）文案、协议路径都与四家 -32601 路径不同；
并入同一个 const 就是改文案（且会让 `pi_unknown_ui_inflight_20261009_test.go:102` 的
"非用户决定"断言失真）。

### 3.5 `desktopEffortFor` 的回退实现差异

pi 走 `desktopDefaults()`（返回 `(model, effort)` 二元组取 effort），zcode 走 `desktopEffort()`；
这是既有口径差异（pi 消费的偏好子集不同），注释已如实声明，合并即改语义。

### 3.6 dsh `"session/update:"` 前缀字面量 ×15

`lit_dup.py` 实测 `dsh.go` 内 ×15（另有 `native_notifications.go:236-245` 的
`strings.HasPrefix(method, "session/update:...")` ×9 同源）。可 const 化，但会同时动
`dsh.go` 与 `native_notifications.go` 两个文件的分档词表 —— 后者在 pi/zcode 接线轮有"互不踩文件"
的约定，留给单独一轮做（§4）。

### 3.7 测试侧 `UnknownEvents().List(...)` 循环 ~28 处

只有 3 处的过滤语义**完全相同**（agent+method），已收敛（项 7）；其余按 `ThreadID` 过滤、
或循环体内做各自断言（坏帧名、分档 kind、计数阈值…），强行抽一个大 helper 只会把断言塞进
参数列表，反而更难读。

### 3.8 gofmt 既有偏差 6 文件

见 §0 的 hunk 清单：偏差与本轮改动行块无交集，属独立改动面（结构体 tag 对齐、doc 注释重排、
`select` 块缩进），不顺手格式化以免把无关 diff 混进这次收敛。

### 3.9 锁名/锁选型：hermes `p.mu` vs 其余 `prefsMu`

`hermes.go:58` 的 `mu sync.Mutex // prefs 锁` 与 dsh/pi/zcode 的 `prefsMu` 角色相近但**不是同一把锁**：
`p.mu` 在 hermes 里同时守护 `prefs` **和** `turnActive`（`Run` 起止各一次，`hermes.go:238-245`），
粒度比别家的 `prefsMu` 粗。把它改名成 `prefsMu` 或拆成两把锁，都是锁结构/粒度的改动（行为面），
得先回答"`turnActive` 与 prefs 要不要互斥"这个问题 —— 不是本轮收敛该做的。
本轮只把"加锁→读/写→解锁"的样板收进 helper，**锁选型与粒度原样保留**（`hideReasoningFrom`/
`applyPrefsSnapshot` 传的就是各家原来那把锁）。

### 3.10 其他记录在案、明确不碰

- proto gen 符号、provider 接口方法：一律不删（红线）；
- 五家 `Run` / `Models` / `ID` 等的错误文案与日志字段：一律不改；
- `unknown_passthrough.go`（辅线 B）的既有骨架 `askUnknownApproval`：已经是收敛产物，不动。

---

## 4. 移交建议

1. **可选下一轮**：dsh `"session/update:"` 前缀 const 化（限定 `dsh.go` 内做，或与
   `native_notifications.go` 的 `HasPrefix` 词表一起做），配套 `dsh_020adapt_test.go` /
   `notification_kinds_test.go` 的字面量断言保持原样。
2. **可选下一轮**：gofmt 既有偏差（6 文件，hunk 见 §0）单独成一项，跑 `gofmt -w` 后
   逐 hunk 复核，与本轮收敛分开提交，便于 review。
3. **可选下一轮**：测试侧台账查询继续 testkit 化 —— 按"过滤维度（AgentID / ThreadID）×
   取全部/取第一条 × 找不到文案"做参数化 helper，一次覆盖 §3.7 的 ~28 处；
   文案必须留在各用例侧。
4. **给未来改 Pi 的人**：`applyDesktopDefaults` 已删（原注释声称"由 Run 每轮调用"早已不成立）。
   Pi 的桌面默认模型/思考强度的**活路径**是 `pi_selection.go configureSelection`
   （`pi.go` Run 内调用）+ `pi.go applyModel`；不要再按旧注释去找那个函数。
5. **本报告的度量可复跑**：
   `C:\Users\yanzi\AppData\Local\Temp\opencode\dup_body.py [pkg] [minlen]`（重复体）、
   `.../lit_dup.py`（字面量）、`.../near_dup.py`（同名近似）、
   `.../restore_before.py`（把本轮 body 级改动逆向重建到 `before_pkg/` 以量"收敛前"）。

---

## 5. 改动文件清单（13 个文件，均在 `daemon/internal/agent/` 内）

**新增（4，共 167 行）**

| 文件 | 行数 | 内容 |
|---|---|---|
| `life_lock.go` | 41 | `lockLifeMu` + `lifeLockRetryStep`（项 1） |
| `unknown_request_ledger.go` | 41 | `unknownRequestRejectionNote` + `recordUnknownRequestRejection`（项 2） |
| `prefs_access.go` | 51 | `applyPrefsSnapshot` / `hideReasoningFrom` / `emitThinkingDelta`（项 4–6） |
| `ledger_query_testkit_test.go` | 34 | `ledgerByMethod` / `firstLedgerByMethod`（项 7） |

**修改（9）**

| 文件 | 改了什么 |
|---|---|
| `codex.go` | 项 2 台账信封 → 1 行调用；项 3 删 `Ready()`/`Account()`，`ReadyCtx` 注释同步 |
| `dsh.go` | 项 1 `lockLife`、项 2 台账信封、项 5 `hideReasoning`、项 6 `ApplyPrefs` |
| `hermes.go` | 项 1 `lockLife`、项 2 台账信封、项 5 `hideReasoning`（锁仍 `p.mu`）、项 6 `ApplyPrefs` |
| `zcode.go` | 项 1 `lockLife`、项 2 台账信封、项 4 `emitThinking`/`emitThinkingFor`、项 5、项 6 |
| `pi.go` | 项 3 删 `applyDesktopDefaults`/`setThinkingLevel` + 2 处注释改指活路径；项 4 `emitThinking`/`emitThinkingFor`、项 5、项 6 |
| `pi_prefs_effect_test.go` | 仅 1 行注释改写（指向已删函数 → 指向活依据），断言未动 |
| `unknown_passthrough_test.go` | `ledgerEntries` 循环体 → `ledgerByMethod`（项 7） |
| `four_provider_hooks_test.go` | `findLedgerEntry` 循环体 → `firstLedgerByMethod`（项 7） |
| `hermes_field_coverage_20261007_test.go` | `findHermesLedgerEntry` 循环体 → `firstLedgerByMethod`（项 7） |

> 过程中曾短暂创建 `thinking_filter.go`，内容随即并入 `prefs_access.go` 并删除，最终状态不存在该文件。
> 未改动：`daemon/internal/` 其他包、`cmd/`、`flutter/`、`android/`、`proto/`、
> `docs/` 下任何既有文件（本报告为唯一新增文档）。**未执行任何 git 操作。**

---

## 6. 复跑命令（证据可复现，均在 `G:\项目1\pocketagent\daemon` 下）

```powershell
C:\Go\bin\go.exe build ./...                                  # 全模块（跨包零引用证明）
C:\Go\bin\go.exe vet ./internal/agent/                        # 无输出即过
C:\Go\bin\go.exe test -count=1 ./internal/agent/              # ok …s
C:\Go\bin\go.exe test -race -count=1 ./internal/agent/        # ok …s
# 用例数（应为 toplevelPASS=621 subPASS=74 FAIL=0）：
C:\Go\bin\go.exe test -v -count=1 ./internal/agent/ > out.txt 2>&1
Select-String '^\s*--- PASS' out.txt ; Select-String '--- FAIL' out.txt
# gofmt 与重复体度量：
C:\Go\bin\gofmt.exe -l internal/agent
python C:\Users\yanzi\AppData\Local\Temp\opencode\dup_body.py                       # 收敛后 6 组
python C:\Users\yanzi\AppData\Local\Temp\opencode\dup_body.py "C:\Users\yanzi\AppData\Local\Temp\opencode\before_pkg"   # 收敛前 10 组
```
