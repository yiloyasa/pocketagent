# daemon host/RPC/store 核心域整洁审查（clean-daemon-server）

- **日期**：2026-10-17
- **范围**：`daemon/internal/server/`（167 文件 / 1.76 MB，prod 54 / test 113）、`rpcwire/`、`store/`、`eventstore/`、`hostenv/`、`rotatelog/`、`testkit/`（参考）。共 736 个生产函数、~39 383 行。
- **不审**：`agent/`、`runtime/`、`cmd/`、`*app/`、`selfguard/`、`gen/`、`panel/`、`collabmcp/`（属另两条线或既定红线）。
- **审查人**：subagent（主会话派生）
- **方法**：Go AST 无关的 Go-aware 词法遮罩扫描（自建 Python 脚本，落在 `C:\Users\yanzi\AppData\Local\Temp\opencode\`，**仓库内零脚本文件**）+ 逐条人工对账。中文路径下 `go vet` / `go test` 直接可跑（**未使用 junction**，见 §8 工具局限）。
- **只读声明**：本轮**未改任何代码/文件**，**未 git add/commit/push**，**未执行 gofmt -w**（gofmt 只做了「读」形态的存在性核对，见 §8）。

### 红线与甲级边界

本报告的每条建议都标注了「是否触碰甲级禁区」：

| 禁区 | 本轮涉及情况 |
|---|---|
| 能力位生成（`server/host.go` 位断言与位族词汇表） | **只标注不建议改**：`host.go` 的 `GetHostInfo`（142 行）本轮只做行数登记，未列任何建议；`host_capability_bits_test.go`（141 行，硬编码位族↔matrix 机器对表）只登记不评 |
| `capability_gate_test.go` 位族↔matrix 对表 | 未审（硬禁区） |
| selfguard（0.4.0 发布产物） | 不在范围 |
| proto 解析层 | 不在范围（仅在 §7 作为「不建议动」判定依据被引用） |
| `daemon/internal/gen/`（protoc 生成） | 扫描时显式排除 |

**全部 P0/P1 建议均不触碰能力位、RPC 契约、store schema** —— 已逐条核对，见各条「禁区」行。

---

## 0. 结论速览

| 面 | 状态 | 一句话 |
|---|---|---|
| `go vet ./internal/{server,rpcwire,store,eventstore,hostenv,rotatelog}/...` | ✅ **零告警** | 基线干净 |
| `go test`（同 6 包） | ✅ **全绿** | server 29.4 s / store 4.2 s / eventstore 2.6 s / hostenv 2.5 s / rpcwire 2.8 s / rotatelog 1.7 s |
| `go test -race`（同 6 包） | ✅ **全绿，0 DATA RACE** | 39.4 s / 4.2 s / 2.6 s / 2.5 s / 2.1 s / 2.8 s / 1.7 s |
| 命名 | ✅ **异常干净** | 万精油类名只有 1 处 `Handler`、3 处 `Manager`、`Util/Utils/Processor/Helper` **各 0 处**；无谎言名（0 例 `getX` 改状态 / 0 例 `isX` 非布尔 / 0 例 `newX` 不新建） |
| 错误处理 | ✅ **异常干净** | 103 个 RPC 入口**零**裸 `fmt.Errorf` 返回、**零** `return nil,nil`；103 处错误一律显式 gRPC status；全域仅 **1 处** `%v` 格式化 error 而未用 `%w` |
| panic | ✅ **零** | 生产代码 `panic(` 出现 **0 次** |
| 注释 | ✅ **异常干净** | 注释掉的代码块 **0 处**（≥3 行的连续代码式注释：0 组）；TODO/FIXME 仅 5 处且全部带归属；变更日志式注释仅 5 处且都承载真实设计信息 |
| 并发 | ⚠️ **P1×2** | goroutine 无退出路径 0 处、channel 泄漏 0 处、Ticker 不 Stop 0 处；但有 2 处**包级缓存无淘汰**（无界增长）与 1 处**未加锁的包级函数钩子**（未来接线即数据竞争） |
| 函数与结构 | ⚠️ **P1×3** | 无布尔 flag 参数滥用（生产仅 6 处且都可解释）；无 >10 方法接口（最大 7）；无 ≥20 case 的巨型 switch（**0 处**）；但有 3 个 god file + 1 个 god constructor |
| 测试整洁 | ⚠️ **P1×0 / P2×1** | 43 个 Server 装配 helper **零精确重复**（语义已证明不等价，见 §6）；断言 2724 处**全部带信息**；条件跳过 12 处全部是显式环境门 |
| 死代码 | ✅ **零** | 双向计数（全 daemon 模块 668 文件）后：无「仅定义处 1 次」的私有函数、无「生产零引用但测试引用」的导出符号、无空文件 |

### 分级计数

| 级别 | 条数 | 编号 |
|---|---|---|
| **P0** | **1** | P0-1 |
| **P1** | **5** | P1-1 … P1-5 |
| **P2** | **8** | P2-1 … P2-8 |

**没有「god object 且已造成维护事故」的 P0，也没有 goroutine 泄漏。** 本域的实际维护事故热点只有一个：PTY 错误静默吞掉（§1 P0-1）。

---

## 1. P0 —— 谎言名 / 吞错误 / goroutine 泄漏 / god object 且已造成或极可能造成事故

### P0-1 · `server/terminal.go:77,82` · 吞错误：PTY 写入/改窗错误被 `_ =` 丢弃，零留痕

**证据**（生产代码，非测试）：

```
server/terminal.go:62   _ = stream.Send(&pb.TerminalServerMessage{... TerminalServerMessage_Error ...})   // 紧接 return err，可接受
server/terminal.go:77   _ = session.Write(m.Input.Data)          // ← 手机敲的每一个键
server/terminal.go:82   _ = session.Resize(int(m.Resize.Cols), int(m.Resize.Rows))
```

**反模式归类**：吞错误（`_ = err`）+ 只 log 不传播**且连 log 都没有** + 调用方无感知面。

**为什么是 P0（而不是 P1）**：
1. **同一个函数内部自相矛盾**。`terminal.go:52 / 68 / 70` 的 `stream.Send` 全部检查错误并 `return err`；唯独对 PTY 侧的两个调用不检查。读者从同函数的对照里读得出「这里是有意放弃的」，但代码里没有任何注释说明为什么 —— 而本仓的规矩是**放弃也必须说理由**（见 `store_observability.go:9-12`：「【为什么只加日志不返回错误】……没有把错误回传给调用方的通道」—— 即便放弃也要写清理由）。
2. **违反本仓自己写下的留痕纪律**。`store_observability.go:5` 明写：「磁盘异常期间这些变化静默失效……**且日志里没有任何线索**」—— 这正是本仓认定的「不可接受」。`session.Write` 失败时 daemon.log 里**一个字都不会有**。
3. `term.Session.Write → writeInput` 与 `Resize → pty.Resize` 都**真的**会返回错误（PTY 层错误），不是 `always nil` 的桩。
4. **事故形态**：用户报「手机终端不响应了 / 敲键盘没反应」，daemon.log 零线索，`Exit` 帧也不会发（PTY 还活着，只是这次写失败）。排障成本极高。

**部分缓释（诚实标注）**：PTY 彻底死掉时，`pumpTerminalOutput`（`terminal.go:110`）会从 `sess.Done()` 侧发一帧 `Exit`，所以「完全失联」有间接感知面；但**部分/瞬时失败**（Windows conpty 写入 EIO、单次 resize 失败）没有任何感知面。这也是为什么定为 P0 而非「稳态 P1」：失败被吞 + 无痕 + 同函数内有对照反例 = 明确反模式，且修复代价 2 行。

**修复建议**（2 行，照抄 `logStoreUpdateErr` 的口径）：

```go
case *pb.TerminalClientMessage_Input:
    if session != nil {
        if err := session.Write(m.Input.Data); err != nil {
            log.Printf("[terminal] 写入 PTY 失败 session=%s bytes=%d: %v", session.ID, len(m.Input.Data), err)
        }
    }
case *pb.TerminalClientMessage_Resize:
    if session != nil {
        if err := session.Resize(int(m.Resize.Cols), int(m.Resize.Rows)); err != nil {
            log.Printf("[terminal] 改窗失败 session=%s cols=%d rows=%d: %v", session.ID, m.Resize.Cols, m.Resize.Rows, err)
        }
    }
```

- **修复代价**：**0.5 小时**（含 1 条回归测试：注入 write 失败的 fake session，断言日志出现）。
- **修复风险**：**不碰任何禁区**。`terminal.go` 不涉及能力位（那在 `host.go`）、不涉及 proto（用现成 `pb.Terminal*` 类型）、不涉及 store schema。纯日志行为，零协议面变更。
- **不确定点**：若仓外已有别的观测面（如 `term` 包内部已 log）则本条降为 P1。我核过 `term/session.go` 的 `Write`/`Resize` 实现，**未见内部 log**，故维持 P0。

---

## 2. P1 —— 明确反模式且有证据

### P1-1 · 两处包级缓存**只判 TTL 不淘汰** → 长跑 daemon 内存无界增长

#### (a) `server/slash_commands.go:289-314` · `slashCache`

```
289: var (
290: 	slashMu       sync.Mutex
291: 	slashCache    = map[string]slashCacheEntry{}
292: 	slashCacheTTL = 10 * time.Minute
...
300: func slashCacheGet(sessionID string) ([]*pb.SlashCommand, bool) {
303: 	e, ok := slashCache[sessionID]
304: 	if !ok || time.Since(e.at) > slashCacheTTL {
305: 		return nil, false          // ← 过期只「不返回」，条目仍留在 map 里
306: 	}
310: func slashCachePut(...) { slashCache[sessionID] = slashCacheEntry{...} }   // 只会增/覆盖
```

- 键 = zcode `sessionID`（「全局唯一」，见 `slash_commands.go:287` 注释）。
- 全仓 `slashCache` 的写入点只有 `slashCachePut` 与 `slashCacheFlush`；**`slashCacheFlush` 的 17 处调用点全部在 `_test.go`**。
- **后果**：用户开过的每一个 zcode 会话，其完整 `[]*pb.SlashCommand` 切片**永久驻留**。TTL 只让它「读起来过期」，不释放。

#### (b) `server/model_selection.go:409,439-440` · `lastTurnSelections`

```
409: var lastTurnSelections sync.Map // threadID -> *TurnSnapshot
...
439: 	prevAny, loaded := lastTurnSelections.Load(threadID)
440: 	lastTurnSelections.Store(threadID, snap)
```

- 键 = `threadID`，**没有 TTL**。
- 全仓唯一删除路径是 `resetSelectionTracking()`（`model_selection.go:425-430`），调用点只有 `model_selection_test.go:353,354` —— **生产零调用**。
- **额外加重**：`DeleteThread`（`thread.go:341-430`）**不清理**这个条目。会话被删除后，`lastTurnSelections` 里的快照仍然留着。跨域审查线记录的 `deletion_resurrection_test.go` 系列正是在钉「删除后不复生」，但这条缓存路径没被纳入该纪律。

**同仓反例证明团队知道正确写法**：`codex_subagent_history.go:103-106` 的 `subagentHistoryInflight` 用 `LoadOrStore` + `defer Delete` 精确配对；`host_preview.go:56-95` 的 `previewCache` 是**单槽**缓存（有界）。所以这是**两处漏网的同款**，不是团队约定。

- **修复代价**：**2-4 小时**。
  - (a) 在 `slashCacheGet` 的过期分支加 `delete(slashCache, sessionID)`（1 行）；或在 `slashCachePut` 里做一次 O(n) 老化扫描（n 很小）。
  - (b) 在 `DeleteThread` 里加 `lastTurnSelections.Delete(threadID)`；或给它加一个与 `slashCache` 同款的 TTL 老化。
- **修复风险**：**不碰禁区**。两者都是**进程内缓存**，不落盘、不进 proto、不改 store schema。唯一要确认的是 (b) 的 `DeleteThread` 属于 `thread.go`（大文件，见 P1-4），改动面小且独立。
- **影响量级（诚实标注）**：单条目 KB 级，实际占用在数月长跑后到 MB 级。定为 P1 是因为「可证明无界 + 与同仓正确写法不一致 + 修复 2 行」，**不是**因为已经造成过事故。

### P1-2 · `server/model_selection.go:415,450` · 未加锁的包级函数钩子 = 已埋好的数据竞争地雷

```
411: // selectionChangedHook selection_changed 的接线钩子(既有事件通路结论:
413: // proto 层暂无载体 —— steward 需在 host.proto 增配分支后,把真实广播实现挂进
414: // 本钩子;届时调用点一处不改)。生产为 nil = 只落日志留痕,不影响任何行为。
415: var selectionChangedHook func(threadID string, prev, cur *pb.EffectiveModelSelection)
...
417: func setSelectionChangedHook(h func(...)) func() { selectionChangedHook = h; ... }   // 无锁写
...
450: 	if selectionChangedHook != nil {          // 无锁读,在 StartTurn 的 goroutine 里跑
451: 		selectionChangedHook(threadID, ...)
```

- 写：`setSelectionChangedHook`（`model_selection.go:418-422`），今天只有测试调用。
- 读：`noteSelectionChanged`（`model_selection.go:435-455`），由 `StartTurn`（`thread.go:1111 / 1277`）在**每轮流式 goroutine** 里调。
- 两者之间**没有** mutex / atomic / 任何 happens-before。`go test -race` 现在不报，因为测试是单线程顺序执行的。
- 注释明确说：**未来 steward 会把真实广播实现挂进来**（需要改 `host.proto`，属甲级）。那一刻如果从非测试 goroutine 写这个全局变量，`-race` 会立刻炸，而调用点「一处不改」意味着**修复面会扩散到 proto 接线**。

**为什么不是纯理论**：`noteSelectionChanged` 的写入方 `lastTurnSelections.Store` 用了 `sync.Map`（正确），紧邻的同款全局变量却用了裸 `func` 变量 —— 同一函数内两种并发纪律。

- **修复代价**：**1-2 小时**（`atomic.Pointer[func(...)]`，或一个 1 行的 `sync.RWMutex`；加一条 `-race` 回归测试：并发 `noteSelectionChanged` + `setSelectionChangedHook`）。
- **修复风险**：**不碰禁区**。这是纯进程内实现细节。**但注意**：它不消除 P1-2 的隐患 —— 真正的隐患是「proto 接线时才发现要加锁」。建议在 `model_selection.go:411-414` 的注释里补一句「接线前必须先加锁」，成本 5 分钟。
- **不确定点**：若 steward 的接线计划里已包含加锁，则本条降为 P2（文档债）。我未见该计划文档，标注为**疑似未来债**。

### P1-3 · `server/server.go:148-302` · god constructor：155 行、12 项职责、含破坏性副作用

`New(cfg Config) *Server` 一个函数里同时做了 12 件事：

| 行 | 职责 | 副作用 |
|---|---|---|
| 151-155 | 打开 eventstore（失败退内存） | 开文件 |
| 158 | 构造 turnJournal | 可能开文件 |
| 161-171 | 组装 8 个字段 | — |
| 179-181 | 注入 summarizer | 改**进程共享**后端 |
| **186** | `reportPendingQueueInputs()` | **清空排队输入台账**（注释明说「然后清空台账」） |
| 190 | `buildRecoveryReport()` | 读 4 个数据源 |
| 193-195 | 登记一次性配对码（15 分钟 TTL） | 改设备表 |
| 199-200 | `verifyProviderDeclarations` | 打日志 |
| **203-231** | 存量自动改名迁移 | **改 store 并落盘**（遍历全库改 Title） |
| 233-266 | 接 codex 的 4 个回调 | 改 provider 字段 |
| 273 | `broadcastPrefs(...)` | 广播给全部 provider |
| 278-296 | 接 zcode / pi / dsh 的回调 | 改 provider 字段 |
| **301** | `backfillRestartedRunningStamp(...)` | **查原生 journal 并改 store** |

**为什么是反模式**：
1. 名字 `New` 承诺「构造」，实际是「构造 + 启动期迁移 + 台账清空 + 回调接线」。调用方（`cmd/`、`recover_report`、以及 **138 处测试装配**）无法从签名看出这一点。
2. 单元测试不可能只测构造：任何 `New(Config{...})` 都会跑一遍全库改名迁移 + 台账清空 + journal 查询。测试与「启动期迁移」被焊死了。
3. 里面已经出现了**规模信号**：`cfg.Store.List(true)` 全库遍历 + 逐条 `Update`（第 203/235/247 行三处重复的「遍历找 codex 线程」模式）。

**修复代价**：**1 日**（纯搬移，不改逻辑：把 186-231、301 抽成 `startupMigrations(srv)`，把 233-296 抽成 `wireProviderCallbacks(srv)`；`New` 留装配）。**这是纯搬移，测试全绿即可证等价**，风险低。
- **修复风险**：**不碰禁区**。`server.go` 的注释 `model_selection.go:406-408` 提到「Task 2 的可改文件圈定不含 server.go」—— 说明历史上有人刻意不动这个文件。**动之前应先确认那一圈定是否已解除**。这是本条唯一的真实风险点。
- **不确定点**：注释显示这是「圈定」而非「不能改」。若圈定仍在，本条应先做「解除圈定」的记录动作而非直接改。

### P1-4 · `server/thread.go` · god file：1931 行 / 1220 代码行 / 15 项职责

**职责清单**（按行号）：

| 行 | 职责 |
|---|---|
| 30-81 | `threadToPB` — pb 转换 |
| 83-98 | `guardianReasonLine` |
| 100-152 | `CreateThread` |
| 154-214 | `turnSettingsFromPrefs` / `defaultModelIDFor` / `availableAgentIDs` |
| 216-280 | `ListThreads` + `sortByCreated` |
| 282-339 | `GetThread` / `nativeSessionMissing` / `checkAndMarkStale` |
| 341-462 | `DeleteThread` + `deleteCollabSessionsForThread` |
| 464-514 | `tombstoneThread` / `deleteChildStubs` |
| 516-570 | `ArchiveThread` / `SetThreadGoal` + 7 个单方法接口 |
| 633-766 | `CompactThread` / `CancelTurn` / `ClearQueuedInputs` |
| 768-866 | `RenameThread` / `GetThreadHistory` |
| 868-946 | `threadMessageToPB` / `goalDetailFor` / `goalDetailFrom` |
| 951-966 | `tailByRounds` |
| 973-1048 | zcode 导入历史拉取（含 `zcodeHistoryMu` 全局） |
| **1061-1300** | **`StartTurn`（239 行）** — 轮次编排心脏 |
| 1316-1403 | `concatTextBlocks` / `outputStateOf` / `recoverEmptyTurnOutput` |
| **1434-1582** | **`runTurnOnce`（148 行）** |
| 1591-1660 | `wirePiTitleHandler` / `applyZcodeTitle` / `applyDshTitle` / `threadAgentID` |
| 1662-1750 | provider 选择（`providerForThread` / `providerFor` / `defaultProvider` / `agentUnavailableHint`） |
| 1751-1807 | 8 个零散小工具 |
| **1809-1882** | **`activeTurns` 租约表**（类型 + 6 方法） |
| 1884-1930 | `ExportThreadHtml` + `activeTurns.Has` |

**参照系**：Android 域 `ChatScreen` 6149 行（另一条线 P0）；这里 1931 行 ≈ 其 1/3，但**职责数（15）才是真信号** —— 一个文件里同时有「pb 转换层」「CRUD」「轮次编排」「provider 选择」「租约表」「工具函数」六种层次的东西。

**这个包自己已经证明了该怎么拆**：同目录里已经有 `thread_ext.go`(593) / `thread_search.go`(272) / `thread_thinking.go`(85) / `thread_memory_rpc.go`(109) —— **按关注点分文件是这个包的既有约定**，`thread.go` 只是没跟上。

**建议切分**（纯搬移）：
- `turn_start.go` — `StartTurn` + `runTurnOnce` + `concatTextBlocks` + `outputStateOf`（~560 行）
- `turn_lease.go` — `activeTurns` 全部（~90 行）
- `thread_pb.go` — `threadToPB` / `threadMessageToPB` / `goalDetailFrom` / `tailByRounds`（~150 行）
- `thread_provider.go` — provider 选择 + title handler（~170 行）
- `thread.go` 留 CRUD + 删除级联 + 历史（~900 行）

- **修复代价**：**1-2 日**（纯搬移 + 每步跑一次 `go test ./internal/server/`；start 循环会掉到 ~600 行）。
- **修复风险**：**不碰禁区**。`StartTurn` 内部注释密集（`thread.go:1052-1300` 有大段「讲解」与「红线」说明），**搬移必须连注释一起走**，否则会把 R10/红线语义与代码分离。`thread.go:1040-1050` 的 `approvalWaitBudget` 注释明确写了「收口纪律：改预算三处一起动」—— 搬移时保留该变量位置或同步更新互指注释。
- **降级理由（诚实标注）**：`StartTurn` 的 239 行里**约 130 行是注释**，实际代码 ~110 行，且每个分支都有设计理由说明。它**不是** android 那种「UI 巨物」，是「被文档包裹的心脏」。本条定为 P1 而非 P0：拆分的收益是可导航性，不是止血。

### P1-5 · `server/collab_session_state.go:1156-1236` · 第二个 store 实现混进了状态机文件

```
1156: type memoryCollabSessionStore struct {     // ← CollabSessionStore 的第二个实现
1163: var _ store.CollabSessionStore = (*memoryCollabSessionStore)(nil)
1165-1236: newMemoryCollabSessionStore / CreateSession / GetSession / ListSessions /
          UpdateSession / DeleteSession / AppendEvent / ReplayEvents / ...
```

- 文件总长 1236 行，其中**最后 ~85 行（7%）是另一个类型的完整实现**，与前 1155 行的 `collabCoordinator`（协同状态机权威，见文件头注释 `:4`）**零耦合**。
- 该类型有独立的编译期接口断言（`:1163`）、独立的一致性纪律（`:1211` 的排序口径注释）、独立的错误语义（`:1233/1236` 的 sentinel 包装）—— 是一等公民，被错放了位置。
- 对照：`store/collab_store.go` 是文件实现，它的内存版理应在 server 侧成对。

- **修复代价**：**1 小时**（把 1152-1236 整段搬进新文件 `collab_store_memory.go`，含 `var _` 断言与全部注释）。
- **修复风险**：**不碰禁区**。纯文件搬移，零逻辑改动，`go build` + `go test` 即证。

---

## 3. P2 —— 风格偏好（证据齐但不构成事故面）

| 编号 | 位置 | 现象 | 代价 / 风险 |
|---|---|---|---|
| **P2-1** | `server/terminal.go:19-26` | **纯 no-op 的 defer 块**：`var session *term.Session; defer func(){ if session != nil { /* 3 行解释性注释 */ _ = session } }()`。`_ = session` 是空语句，整个闭包可删。注释（解释「断开不杀会话，由 term.Manager 回收器处理」）信息量高，但被塞进了一个什么都不做的闭包里 —— 读者必须解析闭包才能学到「此处无需动作」。建议：注释上移到 `var session` 之上，删掉 defer。 | 0.5 h / 无风险 |
| **P2-2** | `server/unknown_actions.go:218` | `unknownActionResult(ok, conflict bool, detail string, summary *pb.…, technical *pb.…, newRevision uint64)` —— **6 参 + 两个相邻位置布尔**。4 处调用点写成 `unknownActionResult(true, false, …)` / `(false, true, …)`，**布尔含义完全靠注释与位置记忆**。这是本域唯一的「布尔 flag 参数 + 位置陷阱」组合。建议改 `unknownActionResult{ok, conflict, …}` 结构体字面量，或拆成 `okResult(...)` / `conflictResult(...)`。 | 1-2 h / 无风险（pb 形状不变） |
| **P2-3** | `server/fs.go:118`、`server/terminal.go:34` | 裸 sentinel 比较 `if err == io.EOF`（Go 约定应 `errors.Is`）。两处都成立（gRPC `Recv` 与 `bufio` 都直接返回 `io.EOF`），但同域 `errors.Is/As` 已被用了 14 次，口径不统一。 | 0.5 h / 无风险 |
| **P2-4** | `server/collab_workspace.go:338,343` | 全域**唯一**一处用 `%v` 而非 `%w` 格式化 error：`fmt.Errorf("…登记错误: %v;回滚输出: %s…", …, err, …)`。丢失 `errors.Is` 能力。注：此处是「登记失败 + 回滚也失败」的双错合并，用 `%w` 只能包一个，另一个仍需 `%v` —— 所以本条**部分属于合理取舍**，仅建议把主错改成 `%w`。 | 0.5 h / 无风险 |
| **P2-5** | `server/collab_session_state.go:109` + `server/collab_delegate.go:148` | 两个包级 `sync.Map` 以 `*Server` 指针为键（`collabCoordinators` / `collabEngines`），**无淘汰**。注释说「生产一个 Server 一个实例;测试各建各的,互不串扰」—— 生产侧成立（同寿命进程）；测试侧每个 `newXxxServer` 建的 Server 会被永久 pin 住（`collabCoordinator` 持 store + watchers + 有界 channel）。533 个 Test 函数累积后是个几 MB 级缓涨，不致命。 | 2-4 h / 无风险；**收益也小**，真要修建议加 `Server.Close()` 时机统一清 |
| **P2-6** | `server/approval_resume.go:19` | 注释说「非协同线程在此是 no-op（登记簿查不到即返回）」—— **这句对被调函数 `clearStalePendingApprovals` 是真的**（`:536-539` 确实首句就 `return`）。但调用点 `s.collabCoordinatorFor()` 会在**任何**线程的审批失败路径上**惰性构造整个协同协调器**，含 `store.NewFileCollabSessionStore(cfg.DataDir)`（开文件 + 解析 + 隔离检查 + 可能打 WARNING）。因 `LoadOrStore` 记忆化，只发生一次。注释只描述了 callee 的语义，没提调用点的惰性初始化副作用。 | 1 h（补注释）/ 无风险 |
| **P2-7** | `server/model_selection.go:242-354` | `classifySelection`：112 行、**控制流嵌套深度 6**（全域最深）。职责是「把一次会话选择解析成有效选择」，内部多级 `if/else` 回退链（:181-221 连续 5 个 `fmt.Errorf` 分支 + :294/:337/:393 多级降级）。属「错误分支淹没正常分支」。 | 0.5-1 日 / 无风险（纯函数，有测试覆盖） |
| **P2-8** | `server/thread.go:1061-1300` | `StartTurn` 239 行中**约 130 行是注释**，实际代码 ~110 行。作为对照，`thread.go` 的代码/注释比是 1220/711（≈ 63% 代码），`collab_delegate.go` 是 281/315（≈ 47%）。`StartTurn` 是全包注释密度最高的函数。**这不算缺陷**（注释内容经抽查全为真，见 §4），仅登记为「拆该函数时勿压缩注释」的施工约束。 | 不修 / 作为 P1-4 的施工约束 |

---

## 4. 注释专项：20 处高危声明逐句对账

扫描到 614 处含强断言词（绝不/永远/唯一/必须/不可能/没有任何/100%）的注释。抽查其中 **20 处可机械核验的**，逐句与实现对账：

| # | 声明 | 实现核验 | 判决 |
|---|---|---|---|
| 1 | `store_observability.go:6` 「err==nil(成功)不打日志,正常路径零开销」 | `:29-33` 首句 `if err == nil { return }` | ✅ 真 |
| 2 | `approval_resume.go:18` 「非协同线程在此是 no-op（登记簿查不到即返回）」 | `clearStalePendingApprovals:536-539` 首句即 `memberSessionOf` 失败 return | ✅ 真（但见 P2-6：调用点副作用未述） |
| 3 | `collab_roles.go:216-221` 「为什么返回**纯 error**而不是 status.Error…会带上 rpc error: code = … 前缀」 | `:222-230` 确实返回 `fmt.Errorf`，非 `status.Error` | ✅ 真，且理由（给父模型读、不要传输层噪音）成立 |
| 4 | `agent_commands.go:11-13` 「provider 不在场/不支持/读取失败,一律安静返回 nil…绝不报错打断输入」 | `codexSlashCommands:37-40` 首句 `if t == nil { return nil }`，返回类型 `[]*pb.SlashCommand` 无 error | ✅ 真 |
| 5 | `collab_policy.go:8-9` 「错误语义走 gRPC status:参数/校验 InvalidArgument,并发冲突 FailedPrecondition,落盘失败 Internal」 | `:37-40` 用 `errInvalidArg`；域内 errors.go 五helper均 `status.Error` | ✅ 真 |
| 6 | `thread.go:1040-1050` 「server 层永远 ≥ provider 预算才不失义…改预算三处一起动」 | `approvalWaitBudget = 15 * time.Minute`；注释点名 `agent/codex.go codexApprovalWaitBudget` 同为 15 分钟 | ✅ 真（agent 包不在本轮范围，未二次核对） |
| 7 | `turn_transport.go:74-75` 「Channels are never closed: a concurrent late/duplicate reply is simply ignored」 | `turnApprovals.pending map[string]chan *pb.ApprovalDecision`，全文无 `close(` | ✅ 真 |
| 8 | `thread.go:1075-1077` 「Cancellation is not completion: never overlap a replacement with an old provider…」 | `:1078-1086` `streamRunCtx` 独立于 `stream.Context()`，`activeTurns.acquire` 持整条流 | ✅ 真（R10 语义） |
| 9 | `collab_workspace.go:156` 「成功 = err == nil && rc == 0（真实 runner 里 rc!=0 必伴随 err!=nil）」 | `execGit:163-190` 返回 `(int, string, error)` | ✅ 口径自洽 |
| 10 | `codex_subagent_history.go:85` 「其余情况一律静默返回(存根是"有则显示"的增强,绝不打扰主流程)」 | `:92-134` 六个守卫全部 `return false`，无 error 返回 | ✅ 真 |
| 11 | `collab_session_events.go:9` 「绝不在回调里做文件 IO / 阻塞等待 —— 只更新内存里的有界…」 | `enqueueJob:846-872` 非阻塞 select + 定时放弃；落盘在 `runEventWorker` 单消费者 | ✅ 真 |
| 12 | `collab_session_events.go:241-246` 「ch 永不 close…出站通道由 Watch 的转发 goroutine 独占发送」 | `publish:279-293` 只 send 从不 close；`Watch:792-797` `defer close(out)` + `defer watchers.remove`（顺序正确：先摘订阅后关通道） | ✅ 真 |
| 13 | `collab_delegate.go:300` 「二者对"显式父链 / 无活跃轮次 / 唯一父候选"的语义完全一致」 | 两函数分支结构一致 | ✅ 真 |
| 14 | `memory_housekeeping.go:262-263` 「周期在启动那刻取原子快照…不重塑已存在的 ticker」 | `:264` `time.NewTicker(idleMemorySweepTick())` 在 goroutine 启动处取一次 | ✅ 真 |
| 15 | `store.go:304-305` 「loadOutcome…由 NewFileStore 一次性写入、之后只读,故读取无需持锁」 | `:337/365` 写入，`LoadOutcome():394-399` 无锁读 | ✅ 真（写入全在构造函数内，发布前） |
| 16 | `slash_commands.go:287` 「缓存键是 zcode sessionId(全局唯一),包级 map 语义等价」 | 键确为 zcode sessionId | ✅ 真 —— 但由此推出的「无界增长」正是 P1-1(a) |
| 17 | `thread.go:1206` 「【vet】call 为指针:pb 消息按值传会被 copylocks 拒绝」 | `approve := func(actx context.Context, call *pb.ToolCallBlock)` 确为指针 | ✅ 真 |
| 18 | `terminal.go:99-100` 「手机已断:别再往死流里塞,否则流控窗口一满,Send 会把整个 PTY 泵一起堵死」 | `:101-103` `if ctx.Err() != nil { return }` 在 Send 之前 | ✅ 真 |
| 19 | `rpcwire/wire.go:74-76` 「卡住期间每次写入会留一个阻塞在 Write/等锁上的 goroutine,持续到管道断开为止…有界」 | `:79` `done := make(chan writeOutcome, 1)` **带缓冲**（超时后仍能送出并放锁） | ✅ 真，goroutine 有界释放路径 |
| 20 | `collab_session_state.go:23,29` 「受理(ACCEPTED)永远不写成 DONE」「WAITING_APPROVAL 不是终局」 | `appendMember:304-321` 只写 `Status: store.CollabStatusAccepted`；`deriveCollabSessionStatus:1074` 含 WAITING_APPROVAL 分支 | ✅ 真 |

**20/20 相符。0 处谎言注释。**

**这在本仓是有据可查的纪律**，不是运气：大量注释在写**为什么这么选**而不是**这行做了什么**（例：`store.go:341-344` 解释「为什么 I/O 错误不做自愈」、`rpcwire/wire.go:70-76` 解释「为什么管道写无法撤销」、`eventstore.go:85-90` 解释「为什么需要 scanMu」）。这类注释**过期时会立刻误导**，但它们不撒谎。

**其余注释面统计**：
- 注释掉的代码块（≥3 行连续、形态为可编译代码）：**0 组**。
- TODO/FIXME/XXX/HACK：**5 处，全部在 prod**，全部带归属或被前文消化（`collab_roles.go:30` 是「XXX 引用」的正文用词；`thread.go:1441` 是「原 TODO 落地」的历史说明；`turn_memory_context.go:27/68`、`hostenv/locate.go:180` 是被括号夹在句中的正文）。**无「无归属、无时间戳」的悬空 TODO。**
- 变更日志式注释（`// 2026-10-xx`）：**5 处**（`agents_sessions.go:55`、`event_sequence.go:314`、`thread.go:1889`、`zcode_subagent_sync.go:131`、`rpcwire/wire.go:6`）。逐条看过，**全部同时承载了现存的设计信息**（改了之后为什么长这样），不是纯流水账。**不建议删**。

---

## 5. 伪命题澄清 —— 查了但判定「不该改」

> 本仓前三轮各产出 3 条；本轮产出 **5 条**。每条都给了「为什么看着像债但其实不是」。

### 5.1 `fmt.Errorf` 缺 `%w` → 「对 gRPC status 传播尤其致命」？—— **不成立（本域零欠账）**

审查框架点名这条要重点查。实测：

- `fmt.Errorf` 带 `%w`：**54 处**；不带：**54 处**。
- 但对 54 处「不带」的逐条分类：**53 处是「构造新错误」而非「包装已有错误」**（例 `store.go:510` `fmt.Errorf("thread %q no longer exists", t.ID)` —— 没有底层 err 可包）。
- **真正的「有 error 可包却用 `%v`」：全域仅 1 处**（P2-4，`collab_workspace.go:338`，且是双错合并场景）。
- **决定性证据**：脚本枚举 103 个 RPC 入口（`func (s *Server) X(ctx, *pb.XRequest)`），
  - 裸 `fmt.Errorf` / `errors.New` 直接返回的：**0 处**
  - `return nil, nil`：**0 处**
  - `errors.go:19-29` 五个 helper 全部 `status.Error(codes.X, msg)`。

**判定**：gRPC status **码**才是传播通道，且每个边界都显式构造。本域在这一点上无债。`errInternal(err)` 用 `err.Error()` 拍平字符串是**标准 gRPC 模式**（码已编码在 status 里，链不再需要）。

### 5.2 `server` 包 167 文件 = god package？—— **不成立**

`server` 内部**已经按关注点分文件**：`thread_*`(6) / `turn_*`(7) / `collab_*`(8) / `model_*`(4) / `host*`(2) / `codex_*`(2) / `zcode_*`/`pi_*` / `unknown_*`(2) / `store_observability` / `recovery_report` / `devices` / `control_plane` / `errors` / `identity` / `git` / `fs` / `terminal` / `slash_commands` / `prompt_opt` / `native_*`(3) / `agent_commands` / `agents_sessions` / `approval_*`(2) / `event_sequence` / `external_sessions` / `runtime_*`(2) / `memory_housekeeping` / `thread_review`。

命名分组清晰、无「misc.go / utils.go / helpers.go / common.go」黑洞文件。**真问题在单个文件（thread.go / collab_session_state.go），不在包。** 见 P1-4、P1-5。

### 5.3 43 个 Server 装配 helper 该收 testkit 吗？—— **不该（语义已证明不等价）**

审查框架要求「先证明语义等价再列」。逐条比对后：**0 组精确重复**（token 流哈希全不同），7 组「宽松相似」的逐行对账结果：

| 组 | 成员 | 为什么不等价 |
|---|---|---|
| 1（6 个） | `newAgentSessionsServer` / `newAuthTestServer` / `newDeleteSyncServer` / `newNativeOptionsServer` / `newDshDeleteServer` / `newDeleteTestServer` | **`newAuthTestServer`（`auth_p0p2_test.go:34`）完全绕过 `New()`**，直接 `&Server{cfg:…, devices:…, hostID:…, activeTurns:…}` 手工装 4 个字段 —— 与其余走 `New(Config{…})` 的是**两条不同的构造路径**；另两个是「拿 `newTestServer` 再改 `cfg.Providers`」的装饰模式 |
| 2（3 个） | `newCodexSlashServer` / `newSlashServer` / `newThreadSearchServer` | 参数面不同（thread 注入 / providers 注入），返回面不同 |
| 3（5 个） | `newProbeTestServer` / `newFsLockTestServer` / `newPrefsTestServer` / `newOptTestServer` / `newTestServer` | 各自要额外接 probe / prefs 录制器 / zcode 开关 |
| 4（2 个） | `newEventTestServer` / `newEmptyOutputServer` | 一个返回 `(*Server,*store.Thread)` 且注入 `seqProvider` |
| 5（2 个） | `newTurnMemoryServerWithPolicy` / `newSummarizerServer` | 都 23 行但一个注 `BudgetPolicy` 一个注 `runtime.Summarizer` |
| 6（2 个） | `newRuntimeStateTestServer` / `newUnknownTestServer` | 只差 `Providers` 字段 —— **这一对确实是唯一值得合并的**（各 9 行，可省 8 行）。收益极小，不值得为它扩 testkit |
| 7（3 个） | `newSteerTestServer` / `newThinkingTestServer` / `newTurnTestServer` | 返回三元组、注入的 probe 各不同 |

**且 `daemon/internal/testkit` 已存在但定位不同**：它只有 4 个导出符号（`AssertNoDeveloperPaths` / `CaptureLog` / `WaitFile` / `WaitProcessGone`）—— 是**环境断言/进程等待工具**，不是 Server fixture kit。给它塞 40 个装配 helper 会让「通用工具包」变成「server 测试专用包」，反而破坏它当前的定位。

**判定**：维持现状。只有第 6 组那对（8 行）值得顺手合并，属收益不足一单。

### 5.4 `rpcwire/wire.go:74-76` 的 goroutine 堆积 = goroutine 泄漏？—— **不成立**

注释自陈「卡住期间每次写入会留一个阻塞在 Write/等锁上的 goroutine,持续到管道断开为止…堆积有界;远好于旧实现」。核验：

- `:79` `done := make(chan writeOutcome, 1)` —— **带缓冲**。超时返回后 worker 仍能 `done <- …` 送出并 `defer w.mu.Unlock()` 放锁。**不会因无人接收而永久阻塞在 send 上**。
- 释放条件明确：`CloseStdin`（`:103-107`）与 `Close`（`:149-156`）持同一把 `w.mu` 关闭 stdin，卡住的 `stdin.Write` 随之返回。
- 有界性成立：调用方各有超时预算，**没有无界堆积路径**。

**判定**：这是**已量化的取舍**，不是泄漏。注释里已写明代价。**不建议改**。

### 5.5 `// _ = session`（`terminal.go:24`）是编译器需要 / 有语义？—— **不成立，但它也只是 P2**

先核一遍「是不是我看错了」：`var session *term.Session` 在函数体内被闭包捕获，`_ = session` **不会**影响逃逸分析、不影响变量存活、不触发 `unused` 检查。**确为空语句。**

但它只是 **P2-1**（风格），不是 P0：删掉它零行为变化，而它携带的注释（「断开不杀会话，由 term.Manager 回收器处理」）**必须保留**。所以「该不该删」的真答案是「该把注释挪上去、删掉闭包」—— 一行搬移，不构成债。

---

## 6. 已查但干净 —— 避免后续重复审

| 面 | 实测数字 | 判定 |
|---|---|---|
| `go vet`（6 包） | 0 告警 | ✅ |
| `go test`（6 包） | 全绿 | ✅ |
| `go test -race`（6 包） | 全绿，**0 DATA RACE** | ✅ |
| **生产 `panic(`** | **0 次** | ✅ HTTP/gRPC handler 里无 panic |
| 注释掉的代码块 | 0 组 | ✅ |
| 悬空 TODO/FIXME | 0 条（5 处命中全带归属） | ✅ |
| 纯变更日志注释 | 0 条（5 处命中均承载设计信息） | ✅ |
| **无退出路径的 goroutine** | **0 / 18** | ✅ 18 个 spawn 点逐个核过：`codexsessions.go:39`/`thread_ext.go:537`/`model.go:92`/`rpcwire/wire.go:79` 全部用**带缓冲 chan** 提交；`collab_session_state.go:299,695` 由 coordinator 的 session 生命周期兜底；`collab_session_state.go:792` 由 `defer close(out)` + ctx 取消驱动；`host.go:288`/`host_preview.go:112`/`memory_housekeeping.go:260`/`terminal.go:73`/`thread.go:1179`/`thread_ext.go:106,538`/`turn_transport.go:27`/`rpcwire/wire.go:155` 均有显式退出条件 |
| `time.Ticker` 不 Stop | 0 处（`host.go:457`、`memory_housekeeping.go:264` 均 `defer ticker.Stop()`；`collab_session_state.go:859`、`model.go:118`、`rpcwire/wire.go:86` 是 `NewTimer` + `defer Stop()`） | ✅ |
| channel 泄漏 / 「发已关通道」竞态 | 0 处 | ✅ `collabWatchers` 订阅通道**永不 close**（`collab_session_events.go:244` 明写设计），出站通道由单 goroutine 独占 close，defer 顺序正确（先 `remove` 后 `close`） |
| `defer` 在循环里（生产） | **0 处**（`host_preview.go:113,116` 在 goroutine 内，其外层 `for` 每次迭代新建 goroutine —— defer 在 goroutine 作用域，语义正确） | ✅ |
| 错误字符串首字母大写 / 句号 | **0 处** | ✅ |
| RPC 入口返回裸 error | **0 / 103** | ✅ |
| RPC 入口 `return nil, nil` | **0 / 103** | ✅ |
| 接口方法数 > 10 | **0**（最大 `CollabSessionStore` 7 方法） | ✅ |
| 巨型 switch（≥20 case） | **0** | ✅ |
| 布尔 flag 参数 | 生产仅 6 处：`authModeText(required)`（纯文案分支）、`totalIf(first,v)`（`fs.go:127`，是「首块」语义非开关）、`buildSettingStatus(…,reachable)`（文案）、`boolStr(b)`（文案）、`sortByCreated(…,asc)`（排序方向）、`unknownActionResult(ok,conflict,…)`（**唯一有问题的一处 → P2-2**） | ✅ 6/6 可解释 |
| 函数参数 > 4 个 | 生产仅 **1** 处：`assembleTurnContext`（`turn_memory_context.go:69`，5 参，含 `runtime` 侧的 ctx 与预算，字面量结构体更合适但不痛） | ✅ |
| 谎言名 `getX` 改状态 | **0**（启发式命中 2 处经核为误报：`fetchViaZcode`、`loadEventFile`，后者确会隔离留档但**已在文档注释里写明**「qPath 非空 = 文件损坏已隔离」） | ✅ |
| 谎言名 `isX`/`hasX` 非布尔 | **0**（命中 2 处是 `hashToken`/`hashFile`，正则误匹配） | ✅ |
| 谎言名 `newX` 不新建 | **0** | ✅ |
| 万精油类名 | `Util/Utils/Processor/Helper` **各 0**；`Handler` 1（`wirePiTitleHandler`，单一用途，可接受）；`Manager` 3（`CollabWorkspaceManager`/`FileCollabWorkspaceManager`/`New…`，是**真实有状态**的多资源工作区管理器，非误用）；`Provider` 5、`Registry` 4、`Agent` 17 —— 全部是领域词非噪声词 | ✅ |
| 同概念多名 | thread 118 / session 13（collab session 是**真概念**，与 thread 在文件头注释里明确区分）；turn/round 无混用（`Round` 概念仅存在于 pb 请求字段的 `rounds`）；cap/caps/capability 在本轮 6 包内无混用（能力位词汇表全在 `host.go`，**硬禁区**） | ✅ |
| 死代码（双向计数，全 daemon 668 文件） | 私有函数：定义处 1 次 + 引用 0 次 = **0**；生产零引用的导出符号（疑似「为可测性而导出」）= **0**；空文件 = **0** | ✅ |
| 断言无信息量 | `t.Fatalf` 2227 + `t.Errorf` 97 **全部带消息**；`t.Fatal` 393 中 365 为裸参，但其中 **118 是 `t.Fatal(err)`**（Go 惯例，err 自带信息），余下 ~50 全是带中文说明的 `t.Fatal("…")`。**无信息量断言 = 0** | ✅ |
| 条件性跳过 | 12 处，**全部**是显式环境门（Windows-only junction ×4、缺 git CLI ×2、`short` 模式 ×2、盘符/设备形态/管理共享 ×4），**无「出错就 skip」** | ✅ |
| 生产 magic number（≥3 位） | 45 处，绝大多数是 `1024`(8×,KiB 换算) / `devServerPorts`(11×,具名切片) / `4096`(buf) —— 已具名 | ✅ |
| 测试资源清理一致性 | **4 个**用真实 temp `DataDir` 构造 Server 的测试文件（`control_plane_test.go` / `runtime_policy_test.go` / `unknown_actions_test.go`）**全部**调用了 `srv.events.Close()` | ✅（曾一度怀疑不一致，逐文件核对后撤销） |
| FileStore 并发 | 全字段单一 `sync.Mutex`；所有读写路径锁序一致；`loadOutcome`/`quarantinePath` 的无锁读**已在字段注释里论证**为「构造期一次性写入后只读」 | ✅ |
| 超长测试函数（>100 行） | 5 个（`turn_concurrency_test.go` 150 / `host_capability_bits_test.go` 141〔**硬禁区，只登记**〕/ `collab_recovery_e2e_test.go` 109 / `collab_session_test.go` 102 / `unknown_event_e2e_test.go` 101）—— 全是 e2e 场景编排，长度来自场景本身而非样板 | ✅ |
| 控制流嵌套 > 3 层 | **39 / 736** 生产函数（5.3%），最深 `classifySelection` 深度 6（→ P2-7） | ✅ 可接受 |
| 超长函数 > 100 行（生产） | 12 个：StartTurn 239 / New 155 / ListModels 153 / runTurnOnce 148 / GetHostInfo 142〔**硬禁区，只登记**〕/ classifySelection 112 / runCollabMember 112 / delegate 110 / DeleteThread 89 / setAgentSettings 89 / SetCodexSettings 88 / prepareWorktree 87。**其中 10 个的注释占比 > 45%** | ⚠️ 见 P1-3/P1-4/P2-7 |

---

## 7. 已知工具局限（虚报剔除记录）

### 7.1 字符串遮罩破坏了错误扫描（**已发现并修正**）

首轮 `fmt.Errorf` 扫描用「遮罩后源码」逐行正则，**格式字符串本身是被遮罩的**（它就是 Go 字符串字面量），导致：

- 首轮报「`%w` 出现 **0** 次、`%v` 格式化 error 共 **111** 处」；
- 修正为**在原始源码上做「括号配平 + 字面量感知」的调用提取**后，真实值是 **`%w` 54 处、真正该包未包 1 处**；
- 首轮 111 条里 **110 条是假阳性**（遮罩后 `%s` 被抹成空格，`%\w` 匹配不到）。

**同类被污染的首轮指标**：「error 字符串首字母大写/句号 = 0 处」—— 这个 0 本身也是不可信的（因为字面量被遮罩了），修正后重扫仍是 0 处，但**理由不同**：重扫是逐条打开原始字符串字面量判定的。**结论未变，证据链已重建。**

### 7.2 花括号深度统计虚报（**已剔除 39→8 组量级**）

首轮嵌套统计把**复合字面量的花括号**也算成嵌套层。典型虚报：

| 函数 | 工具报深度 | 真实控制流深度 | 虚报来源 |
|---|---|---|---|
| `codex_subagent_history.go:201` `subagentSnapshotMessages` | **10** | **3** | `store.StoredMessage{` → `[]pb.ContentBlock{{` → `&pb.ContentBlock_Text{` → `&pb.TextBlock{` 共 4 层字面量括号 × 3 处出现 |
| `terminal.go:18` `Attach` | 8 | 3 | 同上 |
| `thread.go:1061` `StartTurn` | 7 | 3 | 同上 |

修正方式：按「花括号前是否紧跟**大写开头的类型标识符**（`&pb.X{` / `[]pb.X{{` / `map[string]X{`）」区分字面量括号与控制流括号。结果：**深度 ≥4 的函数从 293 个降到 39 个**（占 736 个生产函数的 5.3%），`StartTurn` 直接退出榜单。

### 7.3 `func x() expr` 表达式体（无花括号）

本轮扫描器对无花括号的表达式体函数（如 `func (e *turnRunError) Error() string { return … }` 有括号；`var _ X = (*Y)(nil)` 无 `func` 关键字）不产生误报 —— 因为只对 `^func ` 开头的行起算，且**要求签名区能找到 `{`**；找不到则跳过。**本轮 0 例该类虚报**（不同于 android 域那轮的 7 组）。

### 7.4 中文路径下的 `go test` 假失败（**非仓库缺陷，勿记入基线**）

按审查要求先建了 ASCII junction（`G:\pa_c1a` → `daemon`）跑只读命令。结果：

```
--- FAIL: TestThreadProtoTwoCopiesInSync (0.00s)
    agents_sessions_test.go:198: 读仓库根 proto 失败: open G:\proto\v1\thread.proto: The system cannot find the file specified.
```

**成因**：该测试从包目录向上回溯找仓库根（找 `proto/v1/thread.proto`）。经 junction 时路径是 `G:\pa_c1a\internal\server` → 回溯到 `G:\`（junction 的父是盘根而非真仓库根），于是找错地方。

**复核**：删掉 junction，在真实中文路径 `G:\项目1\pocketagent\daemon` 直接跑 —— **PASS**。**中文路径对 Go 工具链完全友好，不需要镜像**。junction 已删除（`Test-Path G:\pa_c1a` → False）。本报告所有基线数字均取自**中文路径直跑**。

### 7.5 「>100 行函数」与「god object」的口径

`StartTurn` 239 行中约 130 行是注释。本报告的「行数」一律是**物理行**，但分级时已按「有效代码行」降权（见 P1-4 的「降级理由」、P2-8）。若后续要按有效代码行排名，`StartTurn` ≈ 110 行、`New` ≈ 110 行、`classifySelection` ≈ 75 行 —— **均不进「超长函数」前 5**。

---

## 8. 修复优先级建议（不含禁区改动）

| 序 | 项 | 代价 | 禁区 |
|---|---|---|---|
| 1 | **P0-1** `terminal.go:77,82` 补 log | 0.5 h | 无 |
| 2 | **P1-1** 两处包级缓存加淘汰 | 2-4 h | 无 |
| 3 | **P2-1** `terminal.go:19-26` 注释上移、删空 defer | 0.5 h | 无 |
| 4 | **P1-5** `memoryCollabSessionStore` 独立成文件 | 1 h | 无 |
| 5 | **P2-7** `classifySelection` 拍平嵌套 | 0.5-1 日 | 无 |
| 6 | **P1-2** `selectionChangedHook` 加锁（**在 proto 接线之前**） | 1-2 h | 无 |
| 7 | **P2-2/P2-3/P2-4** 参数与 sentinel 口径 | 3 h | 无 |
| 8 | **P1-3** `New()` 拆分（**先确认 server.go 可改圈定是否解除**） | 1 日 | 无（但有历史圈定需确认） |
| 9 | **P1-4** `thread.go` 拆分（**纯搬移，注释必须随行**） | 1-2 日 | 无 |

**本报告 0 条建议触碰**：能力位生成、`capability_gate_test.go` 位族对表、selfguard、proto 解析层、`internal/gen/`、RPC 契约字段、store schema。

---

## 9. 方法与可复现性

- 量化脚本（`c1a_scan.py` / `c1a_metrics.py` / `c1a_err2.py` / `c1a_cmt.py` / `c1a_name.py` / `c1a_scaffold.py` / `c1a_names.py` / `c1a_rpc.py` / `c1a_nowrap.py` / `c1a_nest.py` / `c1a_nest2.py` / `c1a_test.py` / `c1a_evt.py`）全部位于 `C:\Users\yanzi\AppData\Local\Temp\opencode\`，**仓库内零新增文件**。
- 扫描器核心是一个 Go-aware 词法遮罩器：先剥离 `//` 注释、`/* */`、反引号原始串、双引号/单引号字面量（正确处理转义与跨行），再在**保行号**的前提下把注释与字符串替换为空格 —— 后续所有行号统计都基于它，因此**证据行号可直接跳转核对**。
- `gofmt` 只做了**只读存在性核对**（`gofmt -l` 等价于读），**未执行 `gofmt -w`**，未做任何格式化。
- 死代码判定采用**双向计数**：标识符在「全 daemon 模块生产源码（668 文件，去 `internal/gen`）」与「含测试的全体」两个语料上各计一次，只有「定义处 1 次且生产语料无其他出现」才判死 —— 避免了跨包误判。
- 重复体判定采用**两阶段**：先按遮罩后的 token 流 MD5 找精确重复（43 个 helper **0 组**），再按「调用目标名多重集」找宽松相似（7 组），**逐组人工对账后才下结论**（见 §5.3）—— 这是对 R1 轮「4510 函数 / 28 组重复 body 全部判定不收」教训的直接沿用。