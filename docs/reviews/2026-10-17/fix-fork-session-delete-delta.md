# fork 会话删除行为差 —— 判定与处置(2026-10-17)

承接 `fix-session-delete-guards.md` 第 9 条。**结论先行:行为差真实存在,且是本机多数形态(10/16),
不是边角**;已处置 —— 共享闸第 ④ 道从「文件头 id == 请求 id」改为「文件名 id 段与文件头自洽」,
fail-closed 不退让,collab/导入主路径不静默失效。

---

## 一、三项调查的实测数字

### 1. 精确的 fork 文件名形态(只读列举 `~/.codex/sessions`)

逐份读 16 份 rollout 的首行 `session_meta`,与文件名 id 段(去掉 `rollout-` 前缀与
`.jsonl` 后缀后跳过 20 字符时间戳)对账:

| 指标 | 数值 |
|---|---|
| rollout 总数 | **16** |
| 文件名 id 段 1 段(非 fork) | **6** |
| 文件名 id 段 2 段(fork) | **10** |
| 文件名 id 段 > 2 段 | **0** |
| 非 fork:文件名 id 段 == 文件头 id | **6 / 6** |
| fork:文件名**父段** == 文件头 id | **10 / 10** |
| fork:文件名父段 != 文件头 id | **0** |

真实形态(逐字取自本机):

```
rollout-2026-10-11T12-45-22-01a12946-6c10-72d3-a7df-6207720e57cb_01a12947-a240-71f1-80b5-248916ae60ff.jsonl
```

- 分隔符就是**单个下划线 `_`**,不是别的符号;fork 是整个 id 段变成 `<父uuid>_<子uuid>`。
- 该文件首行 `session_meta.payload` 里 `id` **与 `session_id` 同为父 uuid**
  `01a12946-…`,`payload` 里**完全没有子 id 字段**(逐字段看过:creator_*/session_id/id/
  timestamp/cwd/runtime_workspace_roots/originator/cli_version/source/thread_source/
  model_provider/base_instructions/history_mode/multi_agent_version/context_window)。
- `thread_source` 分布:`user` 14 份、`chatgpt_handoff` 2 份 —— `_` 形态**不限于**
  handoff,`user` 来源也会产生,故不是可忽略的导入残留。

**后缀匹配能否命中?能,但只有用复合 id 时能。**
`findCodexSessionFrom` 判的是 `strings.HasSuffix(name, "-"+id+".jsonl")`:

- 请求 `<父>_<子>` → 需要文件名以 `-<父>_<子>.jsonl` 结尾 → **命中**(实测 1 命中)。
- 请求 `<子>` → 需要以 `-<子>.jsonl` 结尾,但文件名里子段前面是 `_` → **不命中**。
- 请求 `<父>` → 命中父会话自己那份(`rollout-…-<父>.jsonl`),**不会**命中分叉那份。

### 2. PocketAgent 实际存的是什么 id(代码 + 本机 `threads.json` 只读)

**本机实际数据(`~/.pocketagentd/threads.json`)**:`threads` 是以 id 为键的 map,
共 **21** 条;带 `codex_thread_id` 的 **10** 条,值**全是裸 uuid**,复合串(含 `_`)**0** 条;
`parent_thread_id` 非空 **0** 条。

**但「现在库里没有复合 id」不等于「不会用复合 id 请求删除」—— 写入侧本来就会产出它:**

- `agent.ListCodexSessions`(`codexsessions.go`):`sess.ID = base[20:]`,即**文件名 id 段
  整段**。对这 10 份就是 `<父>_<子>` 复合串(本机逐份重算:10/16 为复合形态)。
- `server.ImportCodexThread`(`server/codexsessions.go:177`):`CodexThreadID: req.CodexThreadId`,
  **原样落库**,不归一。
- Android 端确实把列表 id 直接送来:`ThreadsScreen.kt:498 .setCodexThreadId(r.codexThreadId)`
  → `ThreadsScreen.kt:665-668 importCodexThread(... setCodexThreadId(c.id))`。

⇒ **「导入电脑上的会话」这条主路径必然把复合 id 存进 `CodexThreadID`。**
本机库里那 10 个裸 uuid 是历史数据(且都已不在当前 16 份 rollout 里),不能作为
「PocketAgent 从不用复合 id」的证据。

**collab 扇出子线程存的是子 uuid,不是复合串 —— 任务提示里「主路径」一词需要修正:**
`DispatchCollab` → codex `spawnAgent` 的子线程,由 `server/codex_subagent_sync.go`
从 `thread/started` 通知取 id 落库(`ID = CodexThreadID = codexChildTID`)。
证据文档 `docs/verify/evidence/codex-subagent-spawn-capture.md` 第 86 行实锤:
子线程 `"id":"01a0c3a9-d78a-76b0-ab14-1fd0ae7a1578"` 是**裸 uuid**,
同条通知 `"forkedFromId":null`。本机 `events.jsonl` 里 `parentThreadId` 出现 **0** 次
(那批子代理线程在 2026-09-21 之后已被清理),与文档证据一致。

⇒ collab 子线程删除走的是**裸 uuid + 非 fork 文件名 + 头 id == 请求 id**,
第 ④ 道本来就过得去 —— 它**不是**被回归的那条主路径。

### 3. 「后缀能匹配上、但文件头 id 与请求 id 不同」的文件真实存在吗

**存在,而且是多数。** 精确计数(用真实文件名 + 真实首行 id 模拟旧闸):

| 判定 | 通过 / 总数 |
|---|---|
| 旧第 ④ 道(`head == 请求 id`,请求 id = 列表给的 id) | **6 / 16** |
| 新第 ④ 道(文件名 id 段与头自洽) | **16 / 16** |
| 「命中但头不一致」——即旧代码会删、新闸会拒的文件 | **10 / 16** |

这 10 份就是 `docs/reviews` 登记表里担心的那类文件,真实存在于本机。

---

## 二、判定结论

**有行为差。** 旧 codex 删除只靠文件名后缀,这 10 份会被删;AG-1 的新闸会把它们判成
「不是它」而拒删。后果是**功能回归而非数据损失**:用户从「电脑上的会话」导入一条分叉
会话再删掉它,rollout 留在盘上、且因墓碑再也导不回来 —— 拿「防误删」换「删不掉」,
方向反了。这违反「主路径不许静默失效」。

AG-1 当时的保留意见(「没有查清 PocketAgent 实际会不会用复合 id 去请求删除」)到此**结清**:
会。写入路径 `ListCodexSessions → ImportCodexThread → CodexThreadID` 逐字产出复合 id。

---

## 三、代码改动

### 共享闸(`session_delete_guard.go`)

第 ④ 道由「注入**取 id 的读取器**」升级为「注入**身份判定**」,因为身份不是一个裸字符串:
codex 分叉 rollout 的「我是不是这条会话」要看文件名与文件头两个来源是否自洽,而 pi 没有
这个约定。

- `deleteNativeSessionFileWithIdentity(root, path, expectID, label, identityOK)` —— 四道闸的
  **唯一实现**,①②③ 逐字未动,顺序与「不过就不删」仍只存在于本文件一处。
- `deleteNativeSessionFile(..., readHeadID)` 保留为**薄适配器**(把读取器包成
  `head == expectID`)。`agent/pi_delete.go` 一行未改、pi 语义逐字不变 ——
  这也是本轮不能碰 `pi_delete.go` 的前提下能做到兼容的原因。

### codex 侧(`codexsessions.go`)

1. **`codexRolloutNameID(name)`** —— 文件名 id 段的**唯一实现**,`ListCodexSessions` 与
   删除闸第 ④ 道共用。原先 `ListCodexSessions` 内联写着切片,现抽成函数:「用户看到的 id」
   与「删除时校验的 id」必须是同一句话,否则改一处就会把两者悄悄掰开。
2. **`codexRolloutIdentityMatches(path, expectID)`** —— codex 第 ④ 道判定,两条证据同时成立:
   - (a) 文件名 id 段**全等**于请求 id(不是后缀:定位那步够「像」即可,这里要「是」);
   - (b) 文件头 id 与该 id 段自洽:相等(普通),**或** 名字恰好两段 `<父>_<子>` 且头里
     恰好是**父段**(分叉/续写约定)。
3. `deleteCodexSessionFrom` 改走 `deleteCodexRolloutFile` → 注入上述判定。
4. 订正 `codexRolloutSessionID` 的注释:原文写「闸 ④ 拒删是正确行为,不是误伤」,已按实测改写。

### 为什么这不放宽安全性(三条论证)

- **放行的只是「约定」,不是「信任」**:要求名字恰好两段、两段非空、且头 id 恰好等于父段。
  改名/复制残留(头里是第三个 id)、头读不到的坏文件、三段以上畸形名,全部落 `false`。
- **「删 A 绝不删到 B」仍由按名定位保证,与本判定无关**:请求 `<父>_<子>` 时父会话那份
  `rollout-…-<父>.jsonl` 的文件名不以 `-<父>_<子>.jsonl` 结尾(它以 `_<子>` 结尾),连候选
  都进不了;反向亦然。已固化为用例。
- **新判定不比定位松**:判定 (a) 是全等,比 `HasSuffix` 定位更严。只有「恰好两段且头等于父段」
  这一条比旧的「头 == 请求 id」宽松 —— 也就是本机 10/16 的实测主流形态。
  已用三段畸形名用例钉住:后缀能命中定位,但身份判定必须拒删。
- **`expectID == ""` 短路在判定之前**,一个「什么都返回 true」的判定也放不过空 id
  (已固化为用例)。fail-closed 无洞。

---

## 四、测试

既有 6 个 codex 删除判决 + 3 个共享闸/pi 判决用例**逐字未改、全绿**(pi 侧文案不许漂的钉子仍在)。
新增 5 条:

| 用例 | 钉的判决 |
|---|---|
| `TestDeleteCodexSessionDeletesForkShapedRollout` | **fork 形态可删**;且删 fork 时父会话那份必须活着(两份文件头 id 相同,只有文件名能区分 —— 这条就是本改动的全部风险面);反向请求父 id 也只删父那份 |
| `TestDeleteCodexSessionRejectsForkShapeWithoutMatchingHead` | **文件名父段与文件头不一致则拒删**:头里是第三个 id / 头读不到 / 头里是子段,三种都拒删且文件留在盘上 |
| `TestDeleteCodexSessionIdentityIsStricterThanNameLookup` | 后缀能命中定位但请求 id ≠ 文件名整段 ⇒ 拒删(第 ④ 道不比定位松) |
| `TestCodexRolloutNameIDIsTheSingleSourceOfTruth` | 名字 id 段解析的唯一真源:真实普通形态 + 真实 fork 复合形态 + 三种判不出的名字 |
| `TestDeleteNativeSessionFileReaderAdapterStaysStrict` | pi 走的读取器版入口仍是严格相等;空 `expectID` 时**根本不调用**注入的判定;判定 false ⇒ 拒删 |

## 五、验证证据

| 项 | 结果 |
|---|---|
| `go build ./internal/agent/...` | **exit 0** |
| `go test ./internal/agent/...` 改前 | 顶层 **662**(PASS 652 / SKIP 10 / FAIL 0) |
| `go test ./internal/agent/...` 改后 | 顶层 **667**(PASS **657** / SKIP 10 / FAIL 0)= 662 + 5 新增 |
| `go test -race ./internal/agent/...` | **exit 0** |
| `gofmt -l`(我改的 4 个文件) | **无输出(干净)**;未跑 `gofmt -w` |
| `git status --short` | 我的改动**仅**落在允许的 4 个文件内(见下) |

**并发线归属说明(未被我触碰)**:`git status` 里另有 `hermes.go` / `hermes_*_test.go` /
`server/*` / `tools/*` / `hermesapp/*` / 若干 `*.md` 的改动,均属另两条线或既有脏状态。
其中 `daemon/internal/agent/pi_delete.go` 显示为 modified —— 那是 **AG-1 抽取共享闸时**的改动
(`git diff` 显示它删掉了内联闸门、改为调用共享函数),**不是本轮**改的;本轮对 pi 侧零改动。

## 六、文件清单(仅这 4 个)

| 文件 | 改动 |
|---|---|
| `daemon/internal/agent/session_delete_guard.go` | 第 ④ 道改注入判定;新增唯一实现,读取器版降为适配器 |
| `daemon/internal/agent/codexsessions.go` | 新增 `codexRolloutNameID` / `codexRolloutIdentityMatches` / `deleteCodexRolloutFile`;`ListCodexSessions` 与删除闸共用名字解析;订正分叉注释 |
| `daemon/internal/agent/codexsessions_delete_test.go` | 新增 4 条判决用例 |
| `daemon/internal/agent/session_delete_guard_test.go` | 新增 1 条(适配器严格性 + 空 id 不调判定) |
| `docs/reviews/2026-10-17/fix-fork-session-delete-delta.md` | 本报告(新建) |

**未 commit / 未 add / 未 push。**

## 七、给 AG-1 登记表的回填建议

第 9 条「fork 会话删除行为差」由「待裁决」改为**已处置**,并请把它的结论句从
「闸 ④ 拒删是正确行为,不是误伤」改成:分叉形态下 PocketAgent 存的就是复合 id、
与文件头父 id 本来就不相等,故第 ④ 道必须按「文件名与文件头自洽」判定,
否则 10/16 的会话永远删不掉。