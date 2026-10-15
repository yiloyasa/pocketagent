# daemon 全域「收敛债」总盘点（2026-10-15 增量轮）

- **任务性质**：原定对「2026-10-15 当天新增 daemon 代码」做收敛审查（D2 收敛轮之后的增量：hermes_slash 位化配套 + capability_gate_test.go 行号修正轮之后的新代码）。
- **范围核对结论**：`git diff --stat HEAD~2..HEAD -- daemon/` **输出为空** —— `HEAD~2..HEAD` 只覆盖 `6bea314`（B3 第 3 波 Files/Git/TaskCenter + pages 域收敛）与 `c0689f6`（AGENTS 基线锚点更新）两个提交，两者对 `daemon/` 零改动。daemon 侧本批实际增量**为零**：D2 收敛轮（`b7d6fb9`，5 文件 +53/−41）之后没有任何新的 daemon 代码落位，capability_gate 行号修正轮也已并入 `b7d6fb9` 自身（该轮修正的是 `b7d6fb9` 之前的注释漂移，见 `docs/reviews/2026-10-14/convergence-daemon-p1slash.md` §4.1 的移交记录，本轮核对确认**尚未有人做**）。
- **按任务书规定转项**：本批无增量可审 → 转为 **daemon 全域收敛债总盘点**。只盘点、不动手（除 §3 的一处低风险高收益测试 helper 收敛外，见下）。
- **纪律遵守**：只跑 `gofmt -l` 查看，**未跑过 `go fmt` / `gofmt -w`**；未 git commit/add/push；未触碰 `daemon/internal/gen/`、`cmd/`、`flutter/`、`android/`、`proto/`。

---

## 0. 基线与复跑证据（盘点前绿基线，原文含用例数）

```
C:\Go\bin\go.exe build ./...                → exit 0
C:\Go\bin\go.exe vet   ./...                → exit 0（无输出）
C:\Go\bin\go.exe test -count=1 ./internal/... → 27 包 ok / 0 FAIL
C:\Go\bin\gofmt.exe -l ./internal/           → 11 个文件（与前两轮基线逐字相同，见 §0.3）
```

用例数（`go test -v -count=1 ./internal/...` 逐行统计）：

```
=== RUN 总数        = 1909
--- PASS 总数       = 1936   （顶层 1555 + 子测试 191，顶层 PASS 里含 1757 个 Test* 列表项中的 1555 个直接 PASS）
--- FAIL 总数       = 0
--- SKIP 总数       = 13    （顶层 11 + 子测试 1；与 `go test -list` 口径差 1 是并发子测试的重复计数）
```

顶层 SKIP 全部为环境门控 live 用例（`pa_live` / 真机 / 缺 node 跳过），与基线一致：

```
TestLiveCodexReadOnlySurface, TestLiveCodexSectionLifecycle,
TestLiveCodexThreadOrganization, TestLiveCodexGuardianApproveAccepted,
TestLiveCodexGuardianAllActionVariantsAccepted, TestNativePiModelSelection,
TestNativeSelectedTextGenerator, TestNativeAgentCatalogs,
TestNativeCodexGoalProtocol, TestRegisterThenBrowse, TestE2ECollabAgentDirRealPi
```

子测试 SKIP 1 条：`TestLoadCredentialEnv/Unix权限过宽跳过注入`（dshapp，Windows 上不适用）。

### 0.1 与上一轮 daemon 收敛报告的水位对齐

上一轮（`docs/reviews/2026-10-14/convergence-daemon-p1slash.md` §0）记录的是 **agent + server 两包合计 1364 用例**。本轮做全域口径，故同时给出两套数字，避免「1364 → 1757」被误读成新增 393 个用例：

| 口径 | 用例数 | 说明 |
|---|---|---|
| agent + server 两包（上轮口径） | **1364**（agent 774 + server 590，SKIP 9） | 本轮复跑逐字相同，**证明 daemon 侧本批无新增/删除用例** |
| internal 全域 27 包（本轮口径） | **1909 RUN / 1936 PASS / 0 FAIL / 13 SKIP** | 盘点需要全域尺子，多出的是其余 25 个包的存量用例 |

### 0.2 盘点工具（照 R1 思路，递归化）

R1 的 `dup_body.py` 只扫单包（`os.listdir`），无法回答「全域还有没有未收敛的重复组」。本轮在 `C:\Users\yanzi\AppData\Local\Temp\opencode\` 新增两个同款尺子的递归版（**临时目录，不入库**）：

| 脚本 | 尺子 | 与 R1 的对应 |
|---|---|---|
| `dup_body_tree.py [root] [minlen]` | 函数体归一化（剥签名、去 `//` 注释、压空白）后 sha1 分组；`len>=60` 的重复组 | R1 `dup_body.py` 的 `os.walk` 版，**排除 `gen/`**，保留 test/src 分类与 `bodylen`/`n=份数` 输出 |
| `lit_dup_tree.py [mincount] [--cross]` | 非测试 `.go` 里长度 ≥6 的字符串字面量跨文件计数 | R1 `lit_dup.py` 的递归版，同样排除 `gen/` |

归一化规则与 R1 **逐字一致**（同一把尺，数据可比）。

### 0.3 gofmt 遗留清单（只读，未写）

```
internal\agent\codex_approval_wait_budget_20261009_test.go
internal\agent\dsh.go
internal\agent\hermes.go
internal\agent\hermes_compact.go
internal\agent\hermes_image_input.go
internal\agent\hermes_image_input_test.go
internal\agent\hermes_pump_badframe_20261009_test.go
internal\agent\pi_events.go
internal\agent\zcode.go
internal\hermesapp\abandoned_turn_test.go
internal\hermesapp\acp.go
```

**与 2026-10-14 两轮收敛报告的遗留清单逐字相同**（11 个文件）。本批 daemon 零改动，清单不变即守住纪律。

---

## 1. 全域重复体量化（核心数据）

### 1.1 总量

```
root = daemon/internal（递归，排除 gen/）  minlen = 60
total funcs: 4510 ; body-only duplicate groups (len>=60): 28
```

按包拆分（`dup_body_tree.py` 逐包复跑）：

| 包 | 函数数 | 重复组 | 组内位置 |
|---|---|---|---|
| agent | 1573 | **7** | 3 组生产（§1.3 A/B/C）+ 4 组测试 |
| server | 1499 | **4** | 1 组生产（§1.3 D）+ 3 组测试 |
| store | 115 | **1** | 1 组生产（§1.3 E） |
| codexapp / dshapp / piapp / zcodeapp / hermesapp | 68+132+112+151+93 | 0（包内） | 见 §1.4 跨包组 |
| 其余 17 包 | — | 0 | — |

> 单包尺子下五家 adapter 包均为 0；但**跨包**尺子（全域 4510 函数同池）下它们贡献了 8 组 —— 这正是「分域收敛」与「全域收敛」的视角差，见 §1.4。

### 1.2 与 agent 包历史基线对齐（同一把尺）

R1 agent 收敛后记录「收敛后 6 组」。本轮 agent 包实测 **7 组**，多出的 1 组是 `text_generator_20261014_test.go` 的 `hermesThreadRefs`/`dshThreadRefs`（P1 补位轮新增文件，R1 收敛发生在该文件之前）。其余 6 组与 R1 记录**逐组对应**：

| R1 记录的 agent 收敛后 6 组 | 本轮实测 | 变化 |
|---|---|---|
| `codexThreadRef`/`dshThreadRefFor`/`threadRefFor` ×3（87B） | 同（87B） | 无 |
| `setCodexThreadRef`/`setDshThreadRef` ×2（67B） | 同（67B） | 无 |
| `stashGoal` codex/zcode ×2（73B） | 同（73B） | 无 |
| 测试 `runImageTurn` ×2（316B） | 同（316B） | 无 |
| 测试 fake `RespondError` ×2（121B） | 同（121B） | 无 |
| 测试 `textBlocks` ×2（88B） | 同（88B，n=3 跨 agent/server） | 无 |
| — | `hermesThreadRefs`/`dshThreadRefs` ×2（69B） | **+1**（P1 轮新文件） |

**结论：agent 包收敛债自 R1 以来零新增**（除 P1 轮自带的一组），R1 的 6 组全部原样保留、无一被无意扩大。server 包 4 组与 store 1 组同理为存量。

### 1.3 生产代码重复组逐组明细（5 组，全部「已识别未动」类）

| # | 组 | 份数 | bodylen | 位置 | 判定 |
|---|---|---|---|---|---|
| A | `codexThreadRef` / `dshThreadRefFor` / `threadRefFor` | 3 | 87 | `agent/codex.go:2372`、`agent/dsh.go:1190`、`agent/hermes.go:1049` | **不并**（R1 §3.1 已论证：helper 要把 map 当值传入，埋「字段被重赋值后写错 map」的隐性契约；`hermes.go:1043` 的 nil 重建先例证明该风险真实存在） |
| B | `setCodexThreadRef` / `setDshThreadRef` | 2 | 67 | `agent/codex.go:2378`、`agent/dsh.go:1184` | 同 A |
| C | `stashGoal` | 2 | 73 | `agent/codex_goal.go:60`、`agent/zcode.go:817` | 同 A（`goalMu` + `pendingGoals` map） |
| D | `sortedProviderIDs` / `availableAgentIDs` | 2 | 135 | `server/control_plane.go:661`、`server/thread.go:200` | **本组有收敛空间但判定保留**，见 §2.1 详细论证（下游消费点语义不同、且 provider map 键集派生是「一处事实多处读取」，合并不改变正确性但会制造第三个名字） |
| E | `writeCollabFileAtomic` / `writeWorkspaceFileAtomic` | 2 | 315 | `store/collab_store.go:690`、`server/collab_workspace.go:702` | **跨包、刻意不并**：`collab_workspace.go:699-701` 的注释明写「与 store 包的 writeCollabFileAtomic 同一纪律，**独立实现以保持本文件自包含**」——server 自包含是该文件的设计声明（Task 5 规格），不是遗漏 |

同族的 `quarantineCorruptFile`（`store/store.go:421`）vs `quarantineWorkspaceFile`（`server/collab_workspace.go:723`）也是 146B 同体，理由同 E：一个在 store 包内被 collab_store 复用（**已经收敛过一次**），一个在 server 自包含。

### 1.4 跨包重复组（8 组，五家 adapter 包的「同源不同包」）

| # | 组 | 份数 | bodylen | 位置 |
|---|---|---|---|---|
| F | `SetGlobalHandler` | 4 | 99 | `codexapp/client.go:96`、`dshapp/client.go:249`、`piapp/client.go:297`、`zcodeapp/client.go:107` |
| G | `SetUnknownResponseHandler` | 4 | 108 | 同上四包 `:103/:256/:304/:114` |
| H | `SetTurnHandlers` | 3 | 115 | `codexapp:111`、`dshapp:264`、`zcodeapp:121` |
| I | `syncHandlers` | 4 | 73 | 同上四包 `:83/:236/:284/:94` |
| J | `Call`（无 ctx 便捷版） | 3 | 86 | `codexapp:391`、`dshapp:389`、`zcodeapp:461` |
| K | `takeAbandoned` | 2 | 107 | `codexapp:437`、`hermesapp:515` |
| L | `AllowOptionID` / `RejectOptionID` | 2 | 120/240 | `dshapp/protocol.go:553/562`、`hermesapp/acp.go:595/606` |
| M | `SetDataDir` | 2 | 60 | `dshapp/credentials.go:16`、`piapp/mcpsetup.go:71` |

**判定：全部「同源不同包」，不并。** 论证见 §2.2。这是五家 adapter 的既有架构形态（各包自包含、协议形状各异），与 D 轮「pages 域收编 pa_page_kit」不同——那里是同一个 Flutter 域内的 UI 重复，这里是**五份独立协议栈的同构纪律**。

### 1.5 测试侧重复组（15 组，逐字重复但低收益/有边界）

全域 28 组里 15 组在 `*_test.go`。逐组：

| 组 | 份数 | bodylen | 位置 | 判定 |
|---|---|---|---|---|
| `runTurnWithReader` / `runTurnWith` | 2 | 356 | `server/output_recovery_test.go:48`、`output_state_test.go:112` | **不并**：两个 provider 类型不同（`recoveringProvider` 带 `ReadNativeTurn` vs `emptyProvider`），Go 无泛型下要么 `any`+断言（更丑）要么接口（为测试添生产面） |
| `runImageTurn` / `runZcodeImageTurn` | 2 | 316 | `agent/codex_image_input_test.go:46`、`zcode_image_input_test.go:52` | 同上；且 codex fake 走 `TestMain` 分流、zcode fake 走 node 脚本，两条链路的装配语义不同 |
| `newFormatLine` / `newFormatRolloutLine` | 2 | 156 | `agent/codexsessions_test.go:34`、`server/codex_subagent_history_test.go:219` | **跨包**（agent vs server），server 侧注释明写「与 agent 包 fixture 同款」——fixture 跨包共享要新建共享包，超出收敛范围 |
| `piThreadWithSession` / `piExportThread` | 2 | 136 | `server/pi_compact_route_test.go:32`、`pi_export_route_test.go:39` | 两文件各只在本文件用；见 §3 收敛候选评估后保留 |
| `RespondError`（fake） | 2 | 121 | `agent/codex_approval_test.go:48`、`zcode_approval_test.go:40` | 两个不同 fake 类型（`fakeCodexResponder` / `fakeZcodeResponder`）的同名方法；R1 §3.7 已判定保留 |
| `piHead` / `piHeadForTest` | 2 | 119 | `agent/pi_sessions_test.go:16`、`server/agents_sessions_test.go:33` | 跨包 fixture，同 `newFormatLine` |
| `idsEqual` / `seqsEqual` | 2 | 116 | `agent/codex_approval_three_options_test.go:55`、`eventstore/eventstore_degrade_test.go:45` | 跨包；类型还不同（`[]string` vs `[]int64`） |
| `equalStrings` / `sameIDs` | 2 | 104 | `piapp/start_args_test.go:107`、`store/store_message_id_test.go:63` | 跨包，同上 |
| `textBlocks` / `zcodeTextBlocks` / server `textBlocks` | 3 | 88 | agent 两文件 + `server/thread_history_zcode_test.go:161` | 跨包 + 跨 provider；agent 侧 server 已有 `textBlocks` 同名，改名动 3 文件 |
| `hermesThreadRefs` / `dshThreadRefs` | 2 | 69 | `agent/text_generator_20261014_test.go:42/48` | R1 §3.8 已判定（不同 provider 类型、各只用 1 次） |
| `Send`（fake stream） | 2 | 65 | `server/terminal_lifecycle_test.go:50`、`thread_steer_queue_test.go:105` | 两个不同流类型（`fakeAttachStream` / `feedTurnStream`），帧类型也不同 |
| `LoadOutcome` | 2 | 65 | `store/collab_store.go:336`、`store/store.go:394` | 两个不同 store 类型的同语义 getter；nil  receiver 语义一致但属不同聚合根，合并不减概念数 |

**测试侧总结**：15 组全部满足「要么跨包、要么跨 provider 类型、要么各只用 1–2 次」。没有任何一组是「同包同类型、逐字重复 ≥3 处」的高收益形态。**测试侧收敛债已基本出清**——这正是前几轮（R1/R2/D2）反复收过的结果。

### 1.6 字面量重复量化（`lit_dup_tree.py`，非测试源码）

```
非测试字符串字面量（长度≥6）去重后 4096 个；出现 ≥8 次的Top：
```

| 次数 | 字面量 | 性质 |
|---|---|---|
| 121 | `"strings"` / 119 `"context"` / 92 `"encoding/json"` / 50 `"path/filepath"` / 50 `"errors"` | import 路径，**不是债** |
| 83 | `"github.com/pocketagent/daemon/internal/gen/pocketagent/v1"` 等内部 import | 同上 |
| 56 | `", fmt.Errorf("` 等代码片段（正则误命中） | 同上 |
| 47 | `"sessionId"` / 37 `"threadId"` / 32 `"message"` / 27 `"description"` / 22 `"options"` | JSON 字段名，**各协议的事实源** |
| **15** | **`"session/update:"`** | **dsh 台账登记名前缀**（`agent/dsh.go` 8 处 + `native_notifications.go` 7 处 HasPrefix） |
| 19 | `"hermes"` | agent id，跨文件事实 |
| 15 | `"toolCallId"` / 14 `"providerId"` / 13 `"toolName"` | JSON 字段名 |
| 12 | `"%s: %w"` 等 fmt 片段 | 同上 |
| 5 | `"codex/"` / 4 `"dsh/"` 等 | agent id 前缀 |

**唯一有收敛价值的字面量：`"session/update:"` ×15**。R1 §3.6 已判定「跨 `dsh.go` 与 `native_notifications.go` 两文件、属另一轮接线面」，本轮确认该结论仍成立且**范围已缩小**：15 处里 8 处是 `"session/update:"+kind` 的登记名拼接（dsh.go），7 处是 `strings.HasPrefix(method, "session/update:...")` 词表（native_notifications.go）。可 const 化，但会同时动两个文件，且 `dsh.go` 是 gofmt 遗留文件（顺带动它会混入格式化 diff）。**建议单独一轮做**（§4.1）。

---

## 2. 逐组「为什么不做」的完整论证（本轮新增部分）

前几轮报告已论证的组（A/B/C、测试侧大部分）不重复论证，只给索引。以下只写**本轮新查清的**。

### 2.1 D 组：`sortedProviderIDs` vs `availableAgentIDs`（135B 同体）

**同体实证**：两函数体归一化后哈希完全相同：

```go
ids := make([]string, 0, len(s.cfg.Providers))
for id := range s.cfg.Providers { ids = append(ids, id) }
sort.Strings(ids)
return ids
```

**消费点差异**（这是判定的关键，前几轮报告未展开）：

| 函数 | 调用点 | 用途 |
|---|---|---|
| `sortedProviderIDs` | `control_plane.go:429`（controlDomainMemory 遍历） | 总览页按 agent 列记忆条目，**条目次序是 UI 可见的** |
| `availableAgentIDs` | `thread.go:119`（StartThread 报错文案）、`model_selection.go:182`（ValidateSelection 报错文案） | **只进错误文案**（`strings.Join(..., ", ")`） |

**为什么不并**：

1. **两个名字承载两份不同的契约声明**。`control_plane.go:659-660` 的注释是「注册表 id 排序副本：map 遍历无序，不排序总览页每次刷新条目次序都会换位」——它钉的是「**排序是给 UI 稳定性用的**」；`thread.go:198-199` 的注释是「当前注册表里可用的 agent ID（字典序）。仅供报错文案动态列举 —— 任何『是否接入』的判断都直接查 s.cfg.Providers，不经过任何名单」——它钉的是「**这不是名单，是文案动态化手段**」（红线 4 的自辩）。合并成一个 `sortedProviderIDs` 后，错误文案的调用点会读到一个名字里带「sorted」却不提「文案」的 helper，两个契约声明必有一个被稀释。
2. **收益 6 行，风险是第三个名字**。合并要么让一方改名（触及 2 个报错文案调用点 + 1 个遍历调用点 + 3 处测试引用面），要么保留双名委托（省不下行数，只多一层间接）。两者都不划算。
3. **同包还有第三处同形**：`thread.go:1738`（`defaultProvider` 的字典序挑选，带 `!= "echo"` 过滤）与 `thread_memory_rpc.go:101`（`requireRegisteredAgent` 的排序名单）。四处的**循环体同形但判据不同**（两个纯排序、一个过滤 echo、一个顺便做存在性校验）——这四段共享的是「Go map 无序 → 要排序」的语言层事实，不是业务重复。
4. **`sortKeys`（thread.go:1761）是死代码边缘**：全仓 grep 仅定义无调用（`debug` 用途注释在，调用点已随调试代码消失）。它不在 28 组里（bodylen 不足 60），但顺手记录：**不建议删**——注释声明「调试用」，删它要动的是「调试便利设施」的判断，不属于收敛债。

### 2.2 F–M 组：五家 adapter 包的跨包同构

这 8 组的共同形态：**四个/五个包各有一份逐字节相同的方法体**。形态上比域内重复更「扎眼」，但判定一致不并，理由是架构层面的：

1. **各包是独立的协议栈封装**。`codexapp`（codex app-server，JSON-RPC 2.0 + initialize 握手）、`dshapp`（ACP v1）、`zcodeapp`（ZCode Protocol NDJSON，无 jsonrpc 字段）、`piapp`（pi `--mode rpc`，`Event` 类型而非 RawMessage）、`hermesapp`（ACP 0.9.0）。五家的 `Client` 结构体字段同名（`handlerMu` / `NotifHandler`）是**刻意对齐的纪律**（`codexapp/client.go:44-46` 注释：「wire 收编自本包的 stdio 传输小件……五家同款」），但 `piapp.SetGlobalHandler` 的签名是 `func(Event)`、另四家是 `func(string, json.RawMessage)`——**签名本身就不同**，F 组的「同体」是尺子把签名剥掉后的产物。
2. **收编要新建第六个共享包或塞进 `rpcwire`**。`rpcwire` 已被注释定位为「stdio 传输小件（写互斥 + 写预算 + 收尸 Once + Kill/关管道）」——把「handler 三 setter + syncHandlers + publishHandlersLocked」塞进去，会让一个**传输层**包去持有**五家各不相同的 handler 结构体**（`codexHandlers` / `dshHandlers` / `piHandlers` / `zcodeHandlers`，字段与语义都不同）。那是用「消灭 5×4 行」换「一个包反向依赖五个包的 handler 形状」，债务转移而非消除。
3. **L 组（AllowOptionID/RejectOptionID）是唯一「真同体」**：两家都是 ACP，`RequestPermissionParams` 结构体也同构（仅 dsh 多 `Kind`/`RequestKind` 两字段）。收编路径是 hermesapp 反向 import dshapp 的这两个方法——但 hermesapp 注释（`acp.go:605`）明写「与 dshapp 同口径」，且两家的**兜底语义依赖各自协议版本**（dsh ACP v1 vs hermes ACP 0.9.0，`PermissionOutcomeSelected` 的 result 形状注释就不同）。协议版本分叉的适配层强行共用一处实现，将来一家升级协议时改一处会静默改另一家——这是「多 provider 语义差异保留」原则在 adapter 层的同款应用。
4. **M 组（SetDataDir）最典型**：两行体，`atomic.Pointer[string]` 的 Trim+Store。收编它需要第五个「datadir」包或者让 piapp/dshapp 互相 import——为两行制造包依赖边，明显过度。

### 2.3 盘点中顺带核实的三个「疑似重复」（不在 28 组内，记录在案）

| 疑似 | 结论 |
|---|---|
| `agent` 包 8 处 clip/truncate 小函数（`echo.go:truncate`、`hermes.go:clipString`、`openai_compat.go:clip`、`pi_events.go:clipRunes`、`pi_sessions.go:clipTitleRunes`、`hermes.go:1097`）+ server 3 处（`clipRunesOr`/`clipRuneTail`/`truncateTitle`/`truncatePreview`）+ dshapp/hermesapp 各 1 | **已经收敛过了**：`runtime/clip.go` 的两个导出函数（`ClipRunes`/`ClipRunesEllipsis`）就是 2026-10-04 全审 util 节的收编成果，文件头注释逐条记录了「为什么放 runtime」。现存的小函数都是**带语义差异的薄壳**：`clipRunesOr` 持 `n<=0 不设限` 守卫（注释明写 helper 的空串语义不适用）、`clipRuneTail` 保留尾部（与头部截断互补）、`openai_compat.clip` 按字节预算回退 rune 边界 + `"\n...[截断]"` 后缀。**这些差异正是不能合并的原因**，且每个壳都不到 10 行。 |
| `mustJSON` / `firstNonEmpty` / `indexByte` | 已是唯一一份（`codex.go:2155` / `codex_events_ext.go:347` / `thread.go:1751`），21 处调用全部复用。**无重复**。 |
| `"session/update"` 前缀拼接 vs `zcodeLedgerMethod` / `hermesLedgerMethod` | 三家的「拆壳 → 复合键登记」是**同款模式不同数据源**：zcode 从 `zcodeEventEnvelope.Type` 取、hermes 从 `StreamEvent.Variant` 取、dsh 从 `dshUpdateKind(raw)` 的 `sessionUpdate`/`type` 双判名取。抽一个 `ledgerMethod(prefix, extractor)` 要把三种解析失败语义（拆不开 → UNKNOWN）各写一遍，比现在的三份 10 行函数更难读。**不并**。 |

---

## 3. 收敛候选的最终评估（一处做、一处登记）

### 3.1 已做：无

**本报告是纯盘点，未改任何 daemon 文件。** 理由：

- §1.3 的 5 组生产重复：A/B/C 是 R1 已论证的隐性契约组；D 是 §2.1 的契约声明组；E 是设计声明组。**没有一组满足「低风险 + 高收益」双条件**。
- §1.5 的 15 组测试重复：全部「跨包 / 跨类型 / 各用 1–2 次」。最接近可做的是 `piThreadWithSession`/`piExportThread`（同包、136B 逐字相同），但评估后**判定保留**：两个 helper 分属 compact 路由与 export 路由两组测试，`pi_compact_route_test.go` 的 helper 还被 `pi_queue_clear_route_test.go` 跨文件复用（3 处调用），而 `piExportThread` 只服务 export 一组 + `model_efforts_test.go` 1 处。合并要么让 compact 侧改名（动 2 文件），要么让 export 侧依赖 compact 文件的 helper（制造文件间隐性耦合，而两个文件的 fake 与断言语境完全独立）。**为 6 行制造耦合，不划算**——这与 R1/D2 对同类小 helper 的判定标准一致。

### 3.2 登记移交（下一轮候选，按收益排序）

| 优先 | 项 | 预估收益 | 前置条件 |
|---|---|---|---|
| 1 | `"session/update:"` const 化（15 处） | 字面量 ×15 → ×1，改协议前缀时只改一处 | 单独一轮；`dsh.go` 是 gofmt 遗留文件，须与该轮格式化解耦（或先做纯格式化轮） |
| 2 | `capability_gate_test.go` 的 `host.go:<line>` 行号引用校准（6 处漂移，D2 §4.1 移交未销案） | 消除「按图索骥扑空」 | 建议直接改成符号名引用（`codexShapedCompactor 段`），行号引用在这类高频增删文件里天生易腐 |
| 3 | gofmt 遗留 11 文件纯格式化轮 | 清零遗留清单 | 必须 `gofmt -w` 明确列文件清单后逐 hunk 复核，与业务 diff 分开（R1 教训） |
| 4 | D 组合并（若未来 `availableAgentIDs` 出现第三个调用点） | 6 行 | 出现第三点时「两份契约」的稀释成本会被规模摊薄，届时再评估 |

**明确不做**：A/B/C 三组（隐性契约）、F–M 八组（跨包同源）、§1.5 全部测试组（跨包/跨类型/低频）。这些不是「还没排到」，是**判定为保留**——与 R1 §3.1「已识别未动」同性质。

---

## 4. 硬禁区核对（本轮零改动，仍逐条确认未被波及）

| 禁区 | 位置 | 本轮状态 |
|---|---|---|
| 能力位生成语义 | `server/host.go` compact 三形状 + slash 两形状断言段 | 未动（daemon 全域本批零改动）；`host_capability_bits_test.go` 全绿证 |
| 账本双写入点时序 | `agent/hermes.go` `beginSlashCapture`/`bind`/`commit`/`close` + Run 泵 `noteCommandsFromEvent` | 未动；`hermes_slash_test.go` 12 用例全绿 |
| hermes 三道闸 | `agent/hermes_compact.go` 闸①/②/③ | 未动 |
| 红线 2（`~/.dsh`、`G:\Hermes` 只读） | `agent/text_generator.go` hermes `GenerateText` 恒传空 effort | 未动；`TestHermesGenerateTextReturnsReplyAndCleansUp` 绿 |

---

## 5. 改动文件清单

**本报告为纯盘点，daemon 侧零改动。**

| 类别 | 文件 | 说明 |
|---|---|---|
| 新增（仓库内） | `docs/reviews/2026-10-15/convergence-daemon-incremental.md` | 本报告 |
| 新增（仓库外，临时目录） | `C:\Users\yanzi\AppData\Local\Temp\opencode\dup_body_tree.py` | 递归版函数体去重尺子（照 R1 `dup_body.py`） |
| 新增（仓库外，临时目录） | `C:\Users\yanzi\AppData\Local\Temp\opencode\lit_dup_tree.py` | 递归版字面量重复尺子（照 R1 `lit_dup.py`） |
| 改动 | 无 | `git status` 干净度见下 |

**git 状态**：未执行 commit / add / push。盘点全程只读仓库（读文件 + `git log`/`git diff --stat`/`git show` + 跑测试与去重脚本）。

---

## 6. 结论

1. **本批 daemon 增量为零**：`git diff --stat HEAD~2..HEAD -- daemon/` 为空；agent+server 两包用例数 1364 = 1364 逐字相同，证明 D2 收敛轮之后 daemon 侧无新码。原定的「增量收敛审查」无对象，按任务书转为全域盘点。
2. **全域收敛债已高度出清**：4510 个函数里 28 组重复，其中 15 组在测试侧（全部跨包/跨类型/低频），5 组生产侧域内（3 组隐性契约 + 1 组契约声明 + 1 组设计声明），8 组跨包同源（五家 adapter 的架构形态）。**没有一组同时满足「低风险」与「高收益」**。
3. **值得下一轮动的只有四件小事**（§3.2）：`"session/update:"` const 化、`capability_gate_test.go` 行号引用改符号名、gofmt 遗留 11 文件纯格式化轮、以及条件性的 D 组合并。合计预估收益 < 40 行重复消灭 + 6 处注释漂移消除，**没有结构性债务**。
4. **度量可复跑**（均在 `G:\项目1\pocketagent\daemon` 下）：

```powershell
# 重复体（全域 / 逐包）
python C:\Users\yanzi\AppData\Local\Temp\opencode\dup_body_tree.py            # 28 组
python C:\Users\yanzi\AppData\Local\Temp\opencode\dup_body_tree.py "...\daemon\internal\agent" 60   # 7 组
# 字面量重复（非测试源码）
python C:\Users\yanzi\AppData\Local\Temp\opencode\lit_dup_tree.py 8
# 基线与水位
C:\Go\bin\go.exe build ./... ; C:\Go\bin\go.exe vet ./...
C:\Go\bin\go.exe test -count=1 ./internal/...        # 27 包 ok
C:\Go\bin\gofmt.exe -l ./internal/                   # 11 个遗留（与基线逐字相同）
```
