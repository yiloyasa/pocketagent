# 修复 server 核心域 3 条高优债（P0-1 / P1-1 / P1-2）

- **日期**：2026-10-17
- **上游报告**：`docs/reviews/2026-10-17/clean-daemon-server.md`（本轮只修其 P0-1 / P1-1 / P1-2）
- **范围**：`daemon/internal/server/` 的实现与测试。**未 commit / 未 git add / 未 push**，未执行 `gofmt -w`。
- **基线**：`main` @ `ede6175`（工作树中 `agent/`、`hermesapp/`、`tools/`、`docs/`、`AGENTS.md`、`DEVELOPING.md` 的改动属并发他线，本轮一行未碰）

---

## 0. 结论速览

| 债 | 上游判定 | 本轮判定 | 状态 |
|---|---|---|---|
| **P0-1** PTY 写/改窗错误被 `_ =` 吞 | P0，建议"补 log" | **属实**（逐行复核），但**修法判错**：补 log 不满足"客户端必须感知" | 已修：发 `TerminalError` 帧 + 终止流 + 用例钉死客户端可见 |
| **P1-1a** `slashCache` 只判 TTL 不淘汰 | P1，建议"读时 delete"或"put 时扫描" | **属实**，两处建议都采纳（读时删 + 写时扫） | 已修 + 2 条用例 |
| **P1-1b** `lastTurnSelections` 无 TTL、`DeleteThread` 不清 | P1 | **属实**（行号有微偏：清理点建议落在 `thread.go`，已落） | 已修 + 3 条用例 |
| **P1-2** `selectionChangedHook` 无锁裸 `func` | P1，"疑似未来债" | **属实** | 已修（`atomic.Pointer`）+ `-race` 并发用例 |

**测试水位**：533 → **542** 顶层用例（+9），子用例 43 → 43，0 失败 0 跳过。既有用例一条未删、未弱化。

---

## 1. P0-1 · `terminal.go:77,82` 吞 PTY 错误

### 1.1 复核：属实（且比报告描述的更严重一点）

改前原文（`git diff` 可见）：

```go
case *pb.TerminalClientMessage_Input:
    if session != nil {
        _ = session.Write(m.Input.Data)          // 报告 :77
    }
case *pb.TerminalClientMessage_Resize:
    if session != nil {
        _ = session.Resize(int(m.Resize.Cols), int(m.Resize.Rows))   // 报告 :82
    }
```

同函数内 `stream.Send` 的三处（L52 / L62-65 / L70）全部检查错误并 `return err` —— 同函数内两种纪律并存，报告的判读正确。

### 1.2 先查清错误语义（本轮自己做的功课，报告未做）

| 调用 | 链路 | 失败含义 | 依据 |
|---|---|---|---|
| `session.Write` | `writeInput` → `ptyWriteWithBudget`(5s 预算) / `sendInterrupt` | **致命**：会话已坏，不得复用 | `term/ctrl.go:186-189` 原文：「超时一律按会话已坏对待:返回人话错误,**由调用方终止/废弃该会话,不再复用**」 |
| `session.Resize` | `pty.Resize` → `windows.ResizePseudoConsole` | **伪控制台已死**（唯一失败形态） | go-pty `pty_windows.go:114-121` 只包 `ResizePseudoConsole`；`cols/rows<=0` 已在 `term/manager.go:451-453` 提前返回 nil |

PTY 挂掉后后续帧是否也会失败：
- 读侧**会**：term 的读泵遇任何读错即 `s.close(code)`（`term/manager.go:331-341`）→ reader channel 关闭 → `pumpTerminalOutput` 补发 `Exit` 帧。这是报告 §1「部分缓释」说的间接感知面，只覆盖**彻底死亡**。
- 写侧**会**：管道已关，`Write` 每次都返回 `os.ErrClosed`。
- **改窗失败不可自愈**：`term/manager.go:188-193` 的活会话快返**不重放 cols/rows**，重连也纠正不了尺寸。

### 1.3 与报告判定的差异（两处，均登记）

1. **报告给的修法不满足本轮的任务要求。** 报告 P0-1「修复建议」是"照抄 `logStoreUpdateErr` 的口径加 log"，验收标准是"断言日志出现"。这等于只解决 daemon 侧留痕，**客户端依然零信号** —— 用户视角的"终端不响应了、不知道发生了什么"原封不动。本轮按任务硬要求改成客户端可见。
2. **报告没查客户端怎么解释 error 帧，而它决定了修法。** `android/.../ui/TerminalScreen.kt:660-664`：`ERROR` 帧 → `state="error"` + `close()`。即 **`TerminalError` 在本协议里就是终态信号**，协议里根本没有"警告但继续"的载体。所以「发帧后继续」在本协议上**不存在**：服务端只会守一条客户端已放弃的流，下一个 `Recv` 错误反而盖掉真正的原因。纪律落成一句：**error 帧 ⟺ 本流结束**。

### 1.4 改动

`daemon/internal/server/terminal.go`（+47/-3）：

```go
case *pb.TerminalClientMessage_Input:
    if session != nil {
        if err := session.Write(m.Input.Data); err != nil {
            return failTerminalStream(stream, session.ID,
                fmt.Sprintf("终端输入写入失败(会话 %s): %v", session.ID, err), err)
        }
    }

case *pb.TerminalClientMessage_Resize:
    if session != nil {
        cols, rows := int(m.Resize.Cols), int(m.Resize.Rows)
        if err := session.Resize(cols, rows); err != nil {
            return failTerminalStream(stream, session.ID,
                fmt.Sprintf("终端改窗失败(会话 %s,%dx%d): %v", session.ID, cols, rows, err), err)
        }
    }
```

新增 `failTerminalStream`：发一帧 `TerminalError` 后返回底层 `cause`（状态文本里带得下 `file already closed` 这类原因）；连错误帧都送不出去时以传输错误为准。顺带补上 proto 里本就为"哪个会话坏了"准备的 `TerminalError.session_id`（`Open` 失败那条没有会话可填，保持原样，**未动**）。

**为什么两处都按终态处理（注释已写进代码）**：写失败是上游 `ctrl.go` 写死的契约；改窗失败按同一口径处理，是因为「发帧后继续」在本协议上等于「客户端已关流、服务端还在假装」，且改窗失败后尺寸**无法通过重连纠正**（复用快返不重放 cols/rows），继续跑等于让用户盯一块永久错位、永不解释的屏幕。同时注明这与 spawn 期 `term/manager.go:314` 那次「首次 resize 失败不影响会话」不矛盾 —— 那里还没有客户端可通知。

### 1.5 为什么没顺手做「PTY 写失败即 `Terminals.Close` 回收会话」

任务没要求，且它有真实副作用：`ptyWriteWithBudget` 的失败形态之一是输入管道被**大段粘贴**灌满导致 5s 超时，此时杀掉用户 shell 是过度反应。会话生命周期仍由 term 的读泵 + 回收器管（`Attach` 的 defer 明确"断开不杀会话"）。**作为遗留项登记在 §5，不在本轮偷偷扩范围。**

### 1.6 用例（判决钉在客户端可见性，不是日志）

新增 `daemon/internal/server/terminal_pty_error_test.go`（3 条）：

| 用例 | 判决 |
|---|---|
| `TestAttachSendsErrorFrameWhenPTYWriteFails` | 写失败 → 客户端收到 `TerminalError`（带 session_id + "写入失败"文案）**且** Attach 以错误收场 |
| `TestAttachSendsErrorFrameWhenPTYResizeFails` | 改窗失败 → 同上，文案含 `120x40` |
| `TestAttachNoErrorFrameWhenPTYIsHealthy` | **反向钉子**：PTY 活着时不得发 error 帧（防"修成无条件报错的终端"） |

时序确定性不靠 sleep：fake 流在"交付第 n 帧"这一精确时刻回调，先 `Terminals.Close` 再等**探针写失败**（`pty.Close` 内部先 `ClosePseudoConsole` 再关管道，故管道写失败时伪控制台必已关闭）——于是下一帧的 `Write`/`Resize` 必然失败。`Recv` 只由 Attach 的调用 goroutine 驱动（同步调用 Attach），回调里 `t.Fatalf` 合法且全无竞态。

---

## 2. P1-1 · 两个包级缓存永不淘汰

### 2.1 复核

| 报告项 | 复核结果 |
|---|---|
| (a) `slash_commands.go` TTL 只在读时判断、条目永久留存 | **属实**。并发安全**现状已有**（`slashMu sync.Mutex` 覆盖 Get/Put/Flush）—— 所以本轮**只加淘汰、不加锁**，两件事没有被混成一次大改 |
| (b) `lastTurnSelections` 无 TTL、`DeleteThread` 不清 | **属实**。行号微偏：值类型在 `model_selection.go:409`，写入/读取在 `:439-440`（报告写 :450），钩子在 `:415`/`:450`（修复前） |

### 2.2 学到的同仓范式

按任务要求先读了 `codex_subagent_history.go:103-106`：

```go
if _, busy := subagentHistoryInflight.LoadOrStore(t.ID, struct{}{}); busy { return false }
defer subagentHistoryInflight.Delete(t.ID)
```

要点不是"用 sync.Map"，而是**登记项的期限由持有者负责兑现**：在飞标记是"配对登记 + 配对删除"。缓存的对应物就是「条目可服务的期限到了就删掉」，而不是「读起来过期、内存里常驻」。下面两处都按这条落。

### 2.3 (a) `slashCache` 淘汰

`daemon/internal/server/slash_commands.go`（+27/-2）：读路径命中过期时**就地 `delete`**；写路径在既有 `slashMu` 下做一次 O(n) 老化扫描，清掉"到期且再也不会被读到"的条目。

为什么不加后台 ticker：那样要多养一条 goroutine 生命周期；表大小是"开过面板的 zcode 会话数"，量级很小，写时扫 O(n) 完全够用（与 `slashCachePut` 同款）。

### 2.4 (b) `lastTurnSelections` TTL + 连带清理

`daemon/internal/server/model_selection.go`（+81/-12）：

1. 值类型 `*TurnSnapshot` → `*turnSelectionEntry{snap, at}`：**只为多带一个落库时刻**（`sync.Map` 无法按值反查键，老化判定要靠它）。
2. `lastTurnSelectionTTL = 24h`。取 24h 的理由写进注释：这张表的唯一用途是"比较相邻两轮"，超长间隔后它承担不了这个语义 —— 与其拿一天前的快照判定"这轮算不算变化"，不如按「首次观察」处理（daemon 重启后本来就是这个行为，已有注释在案），免得在用户重开隔夜会话时补一条迟到一天多的"选择变了"通知。**这确实是一处行为收敛，已如实登记在 §3。**
3. `noteSelectionChanged` 里：读到的 `prev` 超 TTL → 不广播；全表扫描删到期条目。
4. **为什么全表扫描不会随历史变慢**：TTL 本身就是这张表的人口上限（留下的必然是 24h 内跑过轮的线程），所以扫描永远只在这批活会话上跑，不需要后台 ticker，也不需要写计数去摊薄。

`daemon/internal/server/thread.go`（+8）：`DeleteThread` 在 `deleteCollabSessionsForThread` 之后补 `lastTurnSelections.Delete(t.ID)`，与上面两处连带清理同口径，注释说明"不判错"的理由（无副作用）。

### 2.5 用例（4 条）

| 用例 | 判决 |
|---|---|
| `TestSlashCacheGetEvictsExpiredEntry` | 读过期 → 未命中**且包级 map 里那条消失**（判据是 map 大小，不是"读起来过期"） |
| `TestSlashCachePutSweepsExpiredEntries` | 写入时扫掉"到期且无人再读"的条目，未过期与新写入条目必须留下 |
| `TestNoteSelectionChangedIgnoresAndEvictsExpiredSnapshot` | 过期快照不当"上一次"（钩子 0 次）**且**被本轮新快照顶掉 |
| `TestNoteSelectionChangedSweepsExpiredEntries` | 别的线程写入时，到期条目被扫掉；未过期条目不得误删 |
| `TestDeleteThreadClearsLastTurnSelections` | 删会话 → 它的选择快照一并消失 |

既有的 `TestListSlashCommandsCacheExpires`（只断言"过期后重新拉取"）**未动、未弱化**，仍绿。

---

## 3. P1-2 · `selectionChangedHook` 无锁全局函数变量

### 3.1 复核：属实

修复前 `model_selection.go:415` 是裸 `var selectionChangedHook func(...)`，`:417-422` 无锁写，`:450-451` 在每轮流式 goroutine 里无锁读。报告指出"紧邻的 `lastTurnSelections` 用了 `sync.Map`、同款全局变量却用裸 func，同一函数内两种并发纪律"—— 属实。

### 3.2 原语选择：为什么是 `atomic.Pointer[selectionChangedHookFn]` 而不是锁

按访问形态选：**写极少、读极多**。写方只有 `setSelectionChangedHook` 一处（测试 + 未来的启动期接线，进程一生写不了几次），读方是每一轮流式 goroutine。原子指针让读侧一次 `Load()` 拿到**不可变的函数值快照**，此后无论谁换钩子，本轮执行的都还是同一个函数 —— 既不需要锁，也没有"判空与调用之间被人换掉"的时序缝。

`RWMutex` 也能用，但要把"取出来"和"调用"拆开才安全（钩子回调里若再写钩子，持读锁调用会自锁），指针没有这个坑。**钩子的语义一个字没改，也未把 proto 广播提前接进来**（仍是留后项），只换了拿它的方式。

`h == nil` 存的是 nil 指针而非"指向 nil 函数的指针"；`setSelectionChangedHook(nil)` 仍可用来摘钩子。

### 3.3 用例（1 条）

`TestSelectionChangedHookConcurrentReadWrite`（`model_selection_cache_test.go`）：8 个读 goroutine（各跑 200 轮 `noteSelectionChanged`）× 4 个写 goroutine（各 200 轮 `setSelectionChangedHook`/`restore`），收尾断言钩子仍能正常工作（真变化广播 1 次）。钩子体只用原子计数，**不在测试代码里自造竞态**。

**这条判决只在 `go test -race` 下有意义** —— 裸 `func` 变量版在本仓既有测试里永不报竞态（测试是单线程顺序执行的），这正是报告 P1-2「地雷在接线那天才炸」的实质：今天没有测试会撞上它，所以必须新加一条并发用例把它钉住。

---

## 4. 验证证据（原文）

### 4.1 build / vet

```
=== go build ./internal/server/... ===
build exit=0
=== go vet ./internal/server/... ===
vet exit=0
```

### 4.2 用例水位（`-json` 精确计数）

| | 顶层用例 | 子用例 | 失败 | 跳过 | 耗时 |
|---|---|---|---|---|---|
| **改前** | **533** | 43 | 0 | 0 | 27.785 s |
| **改后** | **542**（+9） | 43 | 0 | 0 | 29.04 s |

新增 9 条 = terminal 3 + slashCache 2 + model_selection 4，与文件清单逐条对账吻合。**水位只涨不跌：既有 533 条一条未删、未弱化。**

```
test:  top_pass=542 sub_pass=43 fail=0 skip=0 elapsed=29.04
```

### 4.3 race

```
race:  top_pass=542 fail=0 DATA_RACE=0 elapsed=31.238
```

③ 的并发判决单独点名跑（`-race` 门下）：

```
--- PASS: TestNoteSelectionChangedIgnoresAndEvictsExpiredSnapshot (0.00s)
--- PASS: TestNoteSelectionChangedSweepsExpiredEntries (0.00s)
--- PASS: TestDeleteThreadClearsLastTurnSelections (0.01s)
--- PASS: TestSelectionChangedHookConcurrentReadWrite (0.08s)
--- PASS: TestNoteSelectionChangedFiresOnlyOnChange (0.00s)
ok  	github.com/pocketagent/daemon/internal/server	4.244s
```

### 4.4 变异验证（5 次，逐条"恰好红"）

| # | 变异内容 | 红/绿判决 |
|---|---|---|
| **1** | `terminal.go` 退回 `_ = session.Write(...)` / `_ = session.Resize(...)` | **恰好红 2 条**（两条 error 帧用例），反向钉子 + 3 条既有 terminal 用例**仍绿** |
| **2** | `slash_commands.go` 退回「只判 TTL 不删」+「put 不扫」 | **恰好红 2 条**，11 条既有 slash 用例**全绿**（含 `TestListSlashCommandsCacheExpires`） |
| **3** | `selectionChangedHook` 退回裸 `func` 变量（`-race`） | `WARNING: DATA RACE` ×4 → `--- FAIL: TestSelectionChangedHookConcurrentReadWrite` |
| **4** | 撤掉 `noteSelectionChanged` 的 TTL 判定 + 全表扫描 | **恰好红 2 条**（TTL 用例 + 扫描用例）；钩子用例与 DeleteThread 用例**仍绿**（无假信号） |
| **5** | 撤掉 `DeleteThread` 里的 `lastTurnSelections.Delete` | **恰好红 1 条**（`TestDeleteThreadClearsLastTurnSelections`），13 条既有 DeleteThread 用例**全绿** |

变异 ① 的红输出原文（判决名即 P0-1 的症状本身 —— 客户端只收到 `opened`，没有任何错误帧）：

```
=== RUN   TestAttachSendsErrorFrameWhenPTYWriteFails
    terminal_pty_error_test.go:133: PTY 写失败时客户端必须收到 TerminalError 帧,实际帧:
      [opened:{session_id:"pty-write-fail-1"  shell:"C:\\Windows\\system32\\cmd.exe"  cwd:"..."}]
--- FAIL: TestAttachSendsErrorFrameWhenPTYWriteFails (0.02s)
=== RUN   TestAttachSendsErrorFrameWhenPTYResizeFails
    terminal_pty_error_test.go:164: PTY 改窗失败时客户端必须收到 TerminalError 帧,实际帧:
      [opened:{session_id:"pty-resize-fail-1"  shell:"C:\\Windows\\system32\\cmd.exe"  cwd:"..."}]
--- FAIL: TestAttachSendsErrorFrameWhenPTYResizeFails (0.02s)
=== RUN   TestAttachNoErrorFrameWhenPTYIsHealthy
--- PASS: TestAttachNoErrorFrameWhenPTYIsHealthy (0.03s)
```

变异 ③ 的红输出原文（`-race`）：

```
WARNING: DATA RACE
      G:/项目1/pocketagent/daemon/internal/server/model_selection.go:456 +0x12e
      G:/项目1/pocketagent/daemon/internal/server/model_selection_cache_test.go:156 +0xbc
      G:/项目1/pocketagent/daemon/internal/server/model_selection.go:457 +0x146
--- FAIL: TestSelectionChangedHookConcurrentReadWrite (0.04s)
    testing.go:1865: race detected during execution of test
FAIL	github.com/pocketagent/daemon/internal/server	3.136s
```

**变异方法的自我纠错（如实登记）**：第 4 次变异我最初用 `if true { return }` 桩来"撤掉 TTL 判定"，结果它把整条变化广播路径一起掐死了 —— 于是 TTL 用例**假绿**、钩子用例**假红**。该次变异作废（无判定力），改为**整段删除 TTL 判定**后重跑，才得到上表的干净信号（恰好红 2 条、无假信号）。记录在此以免后续复用这个反模式。

### 4.5 零残留

```
--- SHA256 恢复核对(应与变异前备份一致)---
MATCH  70CC1179DD44E858  terminal.go
MATCH  82A18CFF1B0F5785  slash_commands.go
MATCH  972D6FCD57DD1724  slash_commands_test.go
MATCH  A9E196D22993578D  model_selection.go
MATCH  9DB1E92DB552EEAD  thread.go
MATCH  1E587551F8979B7F  terminal_pty_error_test.go
MATCH  B86BD7CA9A151F6E  model_selection_cache_test.go
--- 变异残留扫描(MUTATION / 变异体 关键字)---
zero residue
```

`git diff --stat -- daemon/internal/server/`：

```
 daemon/internal/server/model_selection.go     | 81 +++++++++++++++++++++++----
 daemon/internal/server/slash_commands.go     | 27 ++++++++-
 daemon/internal/server/slash_commands_test.go | 62 ++++++++++++++++++++
 daemon/internal/server/terminal.go            | 47 +++++++++++++++-
 daemon/internal/server/thread.go              |  8 +++
 5 files changed, 209 insertions(+), 16 deletions(-)
```

`git status --short -- daemon/internal/server/`：5 个 `M` + 2 个 `??`（见 §6），**全部在 `daemon/internal/server/` 下，无一例外**。

`gofmt -l`（只读核对，**未执行 `-w`**）：7 个文件全部 clean（两个新测试文件初建时缺文件末换行，已手工补 LF 修好，未用 gofmt 写）。

---

## 5. 与报告判定的差异登记 / 遗留项

| # | 事项 | 性质 | 处置 |
|---|---|---|---|
| 1 | 报告 P0-1 的修法是"加 log" | **报告判错（修法层面）** | 按任务硬要求改成客户端可见；log 本身也不再是判据，改为"客户端收到错误帧" |
| 2 | 报告未核客户端如何解释 `TerminalError` | **报告漏查（决定修法）** | 本轮补核 `TerminalScreen.kt:660-664`，据此确立「error 帧 ⟺ 本流结束」纪律 |
| 3 | 报告 P1-1(b) 行号 `:439-440` → 实际 `:450`；清理点建议改 `thread.go` | 行号微偏 | 以代码为准；清理点确实落在 `thread.go:DeleteThread` |
| 4 | `lastTurnSelections` 加 24h TTL 后，"隔夜会话的首轮"从"变化"变为"首次观察" | **一处有意的行为收敛** | 已写进代码注释；与「daemon 重启后按首次观察处理」的既有语义一致 |
| 5 | PTY 写失败后，会话可能仍留在 `term.Manager` 里（`Alive()` 仍为真），重连会拿到同一具尸体并再次失败 | **本轮刻意不做（超范围）** | 会话生命周期归 term 的读泵 + 回收器；且 `ptyWriteWithBudget` 的失败形态之一是大段粘贴灌满管道导致超时，此时杀 shell 属过度反应。**建议 steward 单列一条** |
| 6 | 报告 P1-3/P1-4/P1-5、P2-x 共 10 条 | 本轮不修 | 任务只点 P0-1/P1-1/P1-2；其中 P1-3（`server.go` 圈定）与 P1-4（`thread.go` 拆分）本轮已碰 `thread.go` 8 行，但仍属独立决策，留给主线 |

---

## 6. 文件清单

**改实现（4）**

| 文件 | 改动 |
|---|---|
| `daemon/internal/server/terminal.go` | P0-1：`Input`/`Resize` 两处 PTY 失败 → 发 `TerminalError` 帧 + 终止流；新增 `failTerminalStream` 及其设计注释；新增 `fmt` 导入 |
| `daemon/internal/server/slash_commands.go` | P1-1a：读时删过期条目 + 写时全表老化扫描 |
| `daemon/internal/server/model_selection.go` | P1-1b：值类型加落库时刻、`lastTurnSelectionTTL`、过期不广播 + 全表老化；P1-2：钩子改 `atomic.Pointer[selectionChangedHookFn]`；新增 `sync/atomic` 导入 |
| `daemon/internal/server/thread.go` | P1-1b：`DeleteThread` 连带 `lastTurnSelections.Delete` |

**改测试（3）**

| 文件 | 改动 |
|---|---|
| `daemon/internal/server/terminal_pty_error_test.go` | **新增**：3 条（写失败/改窗失败 → 客户端收到错误帧；PTY 健康时不发错误帧） |
| `daemon/internal/server/model_selection_cache_test.go` | **新增**：4 条（TTL 忽略+淘汰、全表扫描、DeleteThread 清理、钩子并发） |
| `daemon/internal/server/slash_commands_test.go` | **追加**：2 条（读时淘汰、写时扫描） |

**红线核对**：0 处触碰 proto（`terminal.proto` 只读未改，双副本未动）、0 处触碰能力位（`host.go` 未动）、0 处触碰 store schema、0 处新增写死名单；未执行 `gofmt -w`；**未 git add / commit / push**。