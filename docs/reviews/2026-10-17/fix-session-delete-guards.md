# fix-session-delete-guards — 会话删除守卫对齐（codex 提到 pi 同级，AG-1）

- 日期：2026-10-17
- 范围：`docs/reviews/2026-10-17/clean-daemon-agent.md` 的 **P0-1「守卫不对称」部分**（`cleanliness-rollup.md` 的 AG-1）
- 改动面：`daemon/internal/agent/` 共 5 个文件（2 改 3 新，**未 commit**）
- 纪律：未碰 `hermes.go` / `hermesapp/` / `server/` / `android/` / `flutter/` / `tools/` / 任何既有 `.md`；未 `git add/commit/push`；未跑任何格式化写操作（只用 `gofmt -l` 只读自检）；go 命令只跑 `./internal/agent/...`
- P0-1 的**语义裁决部分不在本线**：主会话已按 `DEVELOPING.md` §9.1/L362/L392/L396 裁决「删 codex 原生会话文件 = 既定设计、非红线违规」，本线只解决「守卫强度不对称」这一安全性问题，**没改任何可观察的删除语义**

---

## 0. 一句话结论

codex 的会话删除从「文件名后缀像就删」一道闸，提到与 pi 同级的**四道闸**：共用一个闸门函数（`session_delete_guard.go`），第 ④ 道（文件头 id）两家各自注入读取器；`FindCodexSession` 从「首个命中即返回」改为「多于一个命中即报错」。**判决点已实证**：两个同名尾巴的 rollout 同时存在时 `DeleteCodexSession` 报错且**两个文件都还在**；变异验证把共享闸门的包含性关掉后，恰好 4 个（1 新 + 1 既有 pi + 2 共享闸用例）转红，其余全绿。

---

## 1. 不对称证据（改动前，逐行核对）

| 闸 | pi（`pi_delete.go`） | codex（`codexsessions.go`，改前） |
|---|---|---|
| ① 解析（EvalSymlinks） | ✅ L20-30 | ❌ 无 |
| ② 包含性（真在 sessions 目录内） | ✅ L31-34（`rel=="."`/`..`/`..\`/`IsAbs` 全拒） | ❌ 无 |
| ③ 扩展名白名单 `.jsonl` | ✅ L35-37 | ❌ 无（只靠 `FindCodexSession` 的后缀匹配隐式约束） |
| ④ 身份回读（文件头 id == 请求 id） | ✅ L38-42 | ❌ 无 |
| 多命中 | 单次精确 | ❌ L159-172：首个命中即 `hit != ""` 短路，**其余命中根本不看** |
| 幂等 | ✅（文件已不在 ⇒ nil） | ✅ L150 `!os.IsNotExist` |

**可利用形状**：改前 `DeleteCodexSession(id)` 的唯一前提是「`~/.codex/sessions` 树下存在一个尾巴为 `-<id>.jsonl` 的文件」。只要磁盘上出现第二个同名尾巴的文件，删掉的就是遍历顺序里的那一个 —— 与用户手机上点的那条会话是不是同一个文件，毫无校验。

---

## 2. 闸门设计

### 2.1 共用边界：①②③ 共用，④ 注入（以及为什么这么切）

新增 `daemon/internal/agent/session_delete_guard.go`，一个函数 `deleteNativeSessionFile(root, path, expectID, label string, readHeadID func(path string) string) error`，顺序固定为 **解析 → 包含性 → 扩展名 → 身份 → 才 `os.Remove`**。

- **①②③ 放进共享函数**：它们只依赖「路径」，与文件格式无关，是「敢不敢删」这件事的纯路径纪律。
- **④ 由调用方注入读取器**：pi 的 id 在**顶层** `type=="session"` 行上（`pi_sessions.go:132-138`），codex 的 id 在 `type=="session_meta"` 行的 `payload` 里 —— 行类型判定与 JSON 路径都不同。硬凑成一个「万能取 id 函数」只会造出**第二份真值**：两个格式的知识混在一处，改一个格式的解析就得动同一个函数，还会互相牵连（pi 的 `type` 在顶层、codex 的在 `payload` 里，合并后必然出现「哪层取 id」的分支）。
- **但顺序与「任一闸不过就不删」的策略只存在于共享函数一处** —— 这才是共用的真正价值：某家想绕开闸门直接 `os.Remove`，代码上立刻可见，且有测试钉住顺序（§3.2）。
- **fail-closed**：请求 id 为空、或文件头读不到 id，一律拒删（无法证明 = 不删）。与 pi 既有语义一致。

### 2.2 第 ④ 道闸可行性判定：**可用**（不臆造校验）

先读懂了 rollout 的真实首行（2026-10-17 逐份核对本机 `~/.codex/sessions` 下 **16 份**真实 rollout，首行全部是 `type=session_meta`）：

```json
{"timestamp":"2026-10-11T04:44:05.178Z","ordinal":0,"type":"session_meta","payload":{
  "creator_user_id":"user-...","session_id":"01a12946-6c10-72d3-a7df-6207720e57cb",
  "id":"01a12946-6c10-72d3-a7df-6207720e57cb","timestamp":"...","cwd":"C:\\Users\\yanzi\\...",
  "cli_version":"0.162.0-alpha.17.2","source":"vscode", ...}}
```

- **头里确有 id**，且与文件名末段 uuid 同值（16 份里 6 份非分叉文件**全部**一致，0 例不符）→ 第 ④ 道闸是**真检查**，不需要登记为「不可用」。
- **只认 `type=="session_meta"` 行**：rollout 里别的行也带 `payload.id`（如 `response_item` 的 `msg_xxx`），不按行类型限定就会把消息 id 当会话 id，那道闸形同虚设。仓内 fixture 也是这个形状（`codexsessions_test.go:41`）。
- **解析复用**：不另写一套 rollout 解析 —— 逐行 jsonl 读法与同文件的 `scanMeta` 同款（同 Scanner 缓冲上限 64KB/4MB、同样坏行跳过、同样逐行 `json.Unmarshal` 失败即 `continue`）；行数上限 `codexRolloutHeadLineCap = 64`（头就在第 1 行，64 只为容忍将来版本挪后几行；读不到就拒删，绝不为一个 id 扫完几十 MB 的会话）。
- **pi 侧同样复用**：注入的是 `piSessionHeadID`，内部调既有 `scanPiSessionHead`（**未**另写 pi 的文件头解析）。

### 2.3 多命中报错（不猜）

`findCodexSessionFrom` 收集命中：`1` 个返回；`0` 个返回 `ErrCodexSessionNotFound` 的包装（**`errors.Is` 语义逐字未变**，`server/thread_delete_test.go:96` 那条用例不受影响）；**`≥2` 个返回明确错误，不返回任何路径**。

- 多命中**刻意不包** `ErrCodexSessionNotFound`：「有多个候选」与「没有这条会话」是相反的事实。`server/thread.go:308-315` 的 `nativeSessionMissing` 只在 `errors.Is` 命中哨兵时才提示用户「这个会话在电脑上的记录已经找不到了」，其余读故障一律按「还在」处理 —— 包上去就变成**谎报失联**。
- 上层软失败链未变：`CodexProvider.DeleteNativeSession`（`runtime_descriptors.go:264`）把错误原样上抛 → `server/thread.go:373-377` 只记日志、继续删本地线程。残留文件用户还能在「电脑上的 Codex 会话」里再处置（删除链既有语义，未改）。

### 2.4 调用方依赖核对（多命中/新闸是否破坏既有依赖）

`FindCodexSession` 的全部调用方（grep 全仓 21 处命中）：

| 调用点 | 用途 | 受影响？ |
|---|---|---|
| `agent/runtime_descriptors.go:264` → `DeleteCodexSession` | 删除（软失败） | 多命中/闸不过 ⇒ 只记日志，**不阻塞线程删除**，语义不变 |
| `agent/native_recovery.go:31` | 读原生终态 | 只在多命中时多一条错误返回；既有「找不到」路径不变 |
| `agent/current_selection.go:48` | 读当前选择 | 同上 |
| `server/thread.go:309` | 失联判定 | 多命中**不**报失联（按「还在」处理，保守方向正确） |
| `server/codexsessions.go:129` | 导入/fork 读 rollout | 同上 |

结论：**无需改动任何调用方**，也未发现「多命中报错会破坏既有依赖」的情况。

---

## 3. 测试判决（新增 9 条顶层用例 + 8 个子用例；既有 pi 用例一条未删未弱化）

### 3.1 codex 侧（`codexsessions_delete_test.go`，新建）

| 用例 | 钉的判决 | 结果 |
|---|---|---|
| **`TestDeleteCodexSessionRejectsAmbiguousHits`** | **最重要**：两个 `-<id>.jsonl`（分处 `2026/10/11`、`2026/10/12` 子目录）⇒ 报错、**两个文件都还在**、且**不**包 `ErrCodexSessionNotFound`；`findCodexSessionFrom` 同样报错不返回路径 | PASS |
| `TestDeleteCodexSessionDeletesOnlyTheExactRollout` | 正常路径不回归：唯一命中定位正确、只删目标、无关 rollout 不动、删完再删是幂等哨兵、空 id 被拒 | PASS |
| `TestDeleteCodexSessionRejectsOutsideSessionsDir` | 包含性闸：fixture **刻意做成其它三道闸全过**（`.jsonl` + 头 id 与请求一致），只有第 ② 道闸能挡住它 | PASS |
| `TestDeleteCodexSessionRejectsSymlinkedRolloutOutsideDir` | 穿越那一路：sessions 目录内放指向目录外的符号链接（名字照样 `-<id>.jsonl`，能过按名定位） | **SKIP**（本机无符号链接权限，见 §6 登记） |
| `TestDeleteCodexSessionRejectsNonJSONL` | 扩展名闸：`.txt` / `.log` / 同名目录一律拒删且不消失 | PASS |
| `TestDeleteCodexSessionRejectsHeadIDMismatch` | 第 ④ 道闸：头 id 不一致、头里读不到 id、请求 id 为空 ⇒ 三种都拒删 | PASS |
| `TestCodexRolloutSessionIDOnlyTrustsSessionMeta` | 头解析本身：只有 `session_meta` 行的 `payload.id` 算数；头挪到第 12 行仍读得到 | PASS |

### 3.2 共享闸门侧（`session_delete_guard_test.go`，新建）

| 用例 | 钉的判决 | 结果 |
|---|---|---|
| `TestNativeSessionDeleteGateOrder`（3 子用例） | **顺序**：同时破坏两道闸时报的是**靠前**那道（包含性先于扩展名、扩展名先于身份）；`rel=="."`（就是 sessions 目录本身）拒删 | PASS |
| `TestDeletePiSessionKeepsLegacyOutcome`（5 子用例） | 闸门抽成共享函数后 pi 侧**既有可观察结果逐字未变**：三条拒绝文案逐字相同、目标已不在仍是幂等成功、请求 id 为空仍拒删 | PASS |
| `TestDeletePiSessionChecksIdentityAndBoundary` | **既有 pi 用例，原样保留、未删未弱化** | PASS |

---

## 4. 变异验证（把共享闸门的包含性关掉）

改动：`session_delete_guard.go` 的包含性判定改为 `if false && (...)`（并加 `_ = rel` 保持可编译），其余一字未动。

**恰好 4 条转红，全部是包含性相关的（输出原文）：**

```
--- FAIL: TestDeleteCodexSessionRejectsOutsideSessionsDir (0.01s)      ← 新增的 codex 包含性用例
--- FAIL: TestDeletePiSessionChecksIdentityAndBoundary (0.01s)          ← 既有 pi 包含性用例
--- FAIL: TestNativeSessionDeleteGateOrder (0.01s)                      ← 新增顺序钉子
    --- FAIL: TestNativeSessionDeleteGateOrder/包含性先于扩展名 (0.00s)
--- FAIL: TestDeletePiSessionKeepsLegacyOutcome (0.02s)                 ← 新增 pi 遗留文案钉子
    --- FAIL: TestDeletePiSessionKeepsLegacyOutcome/目录外 (0.01s)
    --- FAIL: TestDeletePiSessionKeepsLegacyOutcome/头 id 不一致 (0.01s)   ← 级联：同一 fixture 在上一子用例已被删掉
    --- FAIL: TestDeletePiSessionKeepsLegacyOutcome/请求 id 为空 (0.00s)
FAIL	github.com/pocketagent/daemon/internal/agent	17.841s
```

**没转红的（含判决点在内的其余用例）**：`TestDeleteCodexSessionRejectsAmbiguousHits`（多命中与包含性无关，符合预期）。

**改回后零残留**：`git diff` 无残留、`session_delete_guard.go` SHA256 与变异前一致（`431C95A8FC3415A0D62419DA544EB0A1A765D3FE446E05B71DB0BE15DC2C623F`），文件内 `MUTATION|false &&` 命中 0 行；随后 `go test` / `-race` 全绿（§5）。

---

## 5. 验证（证据原文）

```
$ go build ./internal/agent/...
BUILD_EXIT=0

$ go test ./internal/agent/... -v -count=1        # 改前基线
PASS=644  SKIP=9    TOTAL_TOP_LEVEL=653
ok  	github.com/pocketagent/daemon/internal/agent	19.463s

$ go test ./internal/agent/... -v -count=1        # 改后
PASS=652  SKIP=10   TOTAL_TOP_LEVEL=662          （+9：8 PASS + 1 SKIP，无一条跌）
ok  	github.com/pocketagent/daemon/internal/agent	18.150s

$ go test -race ./internal/agent/... -count=1
ok  	github.com/pocketagent/daemon/internal/agent	25.671s      RACE_EXIT=0

$ go vet ./internal/agent/...
VET_EXIT=0

$ gofmt -l <本次 5 个文件>                          # 只读自检，未跑 -w
（无输出 = 全部符合 gofmt）
```

用例计数口径：**顶层 `--- PASS/FAIL/SKIP` 行**（子用例另计）。改前 653 → 改后 662，**只增不跌**；既有 9 条 SKIP 全部保留（live/真机类），新增的 1 条 SKIP 是下节登记的符号链接用例。

`git status --short -- daemon/internal/agent`（本次改动）：

```
 M daemon/internal/agent/codexsessions.go
 M daemon/internal/agent/pi_delete.go
?? daemon/internal/agent/session_delete_guard.go
?? daemon/internal/agent/session_delete_guard_test.go
?? daemon/internal/agent/codexsessions_delete_test.go
```

同目录另两条 ` M daemon/internal/agent/hermes.go` / ` M daemon/internal/agent/hermes_effort_status_test.go` 是**另一条线**的在途改动（我开工前的 `git status` 基线里已有，我未触碰）。

---

## 6. 登记（不在本线修，留给主会话裁决）

1. **分叉 rollout 现在不可被删（可观察行为增量，如实登记）**：codex 的分叉/续写文件名为 `rollout-<时间>-<父id>_<子id>.jsonl`，首行 `session_meta.id` 是**父**会话 id（本机 16 份里 10 份是这种形状）。这类文件只能经 `ListCodexSessions` 合成的复合 id（`<父>_<子>`）被按名命中，而它自己声明的身份是父会话 → 第 ④ 道闸拒删。**改前能被删，改后软失败**（本地线程照删、rollout 留在「电脑上的 Codex 会话」里）。这是「不许猜」的直接后果，不是误伤；是否要为分叉文件单开一条规则（如承认复合 id 的父段），属新语义设计，本线不自行发明。
2. **符号链接穿越用例在本机是 SKIP**：`os.Symlink` 报 `A required privilege is not held by the client`（Windows 需开发者模式/管理员）。该用例已写好并会在有权限的机器上执行；本机的包含性判决由「直接给目录外文件」那一路承担，且它**确实在变异验证里转红**。不做「假装跑过」的假绿。
3. **`~/.codex/sessions` 本身是符号链接时，`ListCodexSessions` 与 `FindCodexSession` 都会返回空**（`filepath.WalkDir` 不跟随符号链接目录）。属既有行为，本次未改、也不在允许改动范围内（属发现/导入面）。
4. **P1-3 未在本线处理**：任务简报把 `clean-daemon-agent.md` 的 **P1-3**（`native_notifications.go:398` `record` 死代码 + 谎言注释）并列进来，但该条目落在 `daemon/internal/agent/native_notifications.go` —— **不在本线允许改动的文件集**（`codexsessions.go` / `pi_delete.go` / 新增闸门 helper / 对应测试）内，故**只登记未动**。`cleanliness-rollup.md` 的 AG-1 行也只写了守卫对齐这一件事。

---

## 7. 文件清单

| 文件 | 状态 | 内容 |
|---|---|---|
| `daemon/internal/agent/session_delete_guard.go` | **新增**（75 行） | 共享闸门 `deleteNativeSessionFile`：解析 → 包含性 → 扩展名 → 注入的身份闸 → 才 `os.Remove`；含「为什么 ①②③ 共用 / ④ 注入」的注释 |
| `daemon/internal/agent/pi_delete.go` | 改（44 → 25 行，+11/−30） | `deletePiSessionFrom` 改走共享闸门；新增 `piSessionHeadID`（复用 `scanPiSessionHead`）。三条拒绝文案逐字不变 |
| `daemon/internal/agent/codexsessions.go` | 改（251 → 354 行，+93/−14） | `DeleteCodexSession` → `deleteCodexSessionFrom(root, id)` 走共享闸门；`FindCodexSession` → `findCodexSessionFrom(root, id)` 多命中报错（`filepath.SkipAll` 提前收工）；新增 `codexRolloutSessionID` + `codexRolloutHeadLineCap` |
| `daemon/internal/agent/codexsessions_delete_test.go` | **新增**（236 行） | codex 侧 7 条判决用例（含最重要的多命中） |
| `daemon/internal/agent/session_delete_guard_test.go` | **新增**（122 行） | 共享闸门顺序钉子 + pi 既有结果逐字钉子 |

**未 commit**（`git add/commit/push` 均未执行）。未新增任何写死的路径/名单常量：sessions 目录一律经 `PiSessionsDir()` / `CodexSessionsDir()`，且只出现在导出的那一层。
---

## 订正（2026-10-17，由 AG-2 复核后回填）

本报告第 9 条登记项的两处结论需更正，完整证据见
[fix-fork-session-delete-delta.md](fix-fork-session-delete-delta.md)：

1. **行为差不是伪命题，而是本机多数形态**：16 份真实
   rollout 中 **10 份是分叉形态**（`rollout-<时间>-<父>_<子>.jsonl`，
   **文件头存父 id、根本没有子 id 字段**，10/10 实测）。
   本报告只写了「登记待裁决」，未量化，容易被读成少数边缘事件。

2. **被回归的路径不是 collab，是「导入电脑上的会话」**：
   本报告推测「主路径是 collab 子线程删除」——**错的**。
   collab `spawnAgent` 子线程存的是**裸 uuid**（`forkedFromId:null` 实锤），
   闸④ 本就过得去；真正会被打废的是 `ListCodexSessions`
   取文件名 id 段整段（`base[20:]`）→ 复合 id 原样落 `CodexThreadID`
   → Android `ThreadsScreen` 直接送该 id。本机 `threads.json` 当前 21 条里
   复合 id 为 0，仅是**历史数据**，不能当「从不用」的证据。

3. **处置已落地**（AG-2）：闸④ 由「头 id == 请求 id」改为
   **「文件名 id 段与文件头自洽」**（全等，或恰两段且头 == 父段）；
   fail-closed 未退让（三段畸形名 / 改名残留 / 头读不到一律拒删）。
   用例 662 → **667**（PASS 657 / SKIP 10 / FAIL 0）。

引用：AG-2 报告《AG-1 的结论句「闸④ 拒删是正确行为，不是误伤」
需改为「分叉下两者本来就不相等，必须按自洽判定，否则 10/16 永远删不掉」。
