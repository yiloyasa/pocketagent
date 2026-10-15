# d4 · hermes fork 会话命令账本捕获窗口补位报告

> 日期：2026-10-15 · 任务：D1 报告 §六残留边界第 3 条（fork 会话窗口）。
> 上游：`docs/reviews/2026-10-14/d1-hermes-slash-wiring.md` §二判据 3（fork 广告点 server.py:668）+ §六.3。
> 范围纪律：只碰 `daemon/internal/agent/`（本次只新增 1 个测试文件，零生产改动）；
> `daemon/internal/hermesapp/` 只读；绝不碰 `daemon/internal/server/`、`gen/`、`cmd/`、
> `flutter/`、`android/`、`proto/`；未 git commit/add/push。

---

## 一、调用链核实结论：daemon 侧无 fork 调用链可挂窗口（走实施第 2 条）

核实手段均为只读（grep + 读文件），无改动：

1. **agent 包内零命中**：`grep ForkSession daemon/internal/agent/` = **No matches found**。
   `HermesProvider` 无任何 Fork 方法（`grep hermes.*fork` 仅命中注释与 caps 日志行
   `hermes.go:252`、测试 caps 构造行、collab 注释行——无方法、无调用）。
2. **`ForkSession` 只活在 hermesapp 层**：`daemon/internal/hermesapp/acp.go:211`
   `func (c *Client) ForkSession(ctx, cwd, sessionID) (*ForkSessionResult, error)`，
   经 `callCtx(session/fork)` 发请求，应答仅回新 `sessionId`（空串按失败处理，
   `acp.go:222-224`）；应答本身不带广告帧——广告是适配器在应答后经
   `loop.call_soon` 另发的 `available_commands_update` 通知（D1 §二判据 1，
   server.py:668）。daemon 侧调用方：除 `hermesapp/client_test.go` 与两个一次性
   探针（`cmd/capprobe`、`cmd/hermsessprobe`）外，**agent/server 生产代码无人调用**。
3. **server 侧 `ForkThread` 无 hermes 分支**：`daemon/internal/server/thread_ext.go:277-333`
   只分流 `pi`（`forkPiThread`）与 codex 默认路径；hermes 线程走 codex 路径会因
   `CodexThreadID == ""` 被拒——hermes 分叉在 daemon 侧语义缺席（D1 §六.3
   「将来的第四家形状」定性成立）。

**结论**：没有 fork 调用链，就没有可挂“与 create/remount 同款窗口捕获器”的位置。
按任务书实施第 2 条：**不造调用链**（不许为补窗口而新增 fork 功能），任务转为——
测试钉死「fork 会话跑第一轮时轮次泵 `noteCommandsFromEvent` 仍能入账」+ 如实登记。
生产代码（`hermes_slash.go` 时序语义、`hermes.go` 建立窗口）**一行未动**。

---

## 二、走哪条：实施第 2 条（测试钉泵路径，零生产改动）

- 未新增 `hermes_fork*.go` 生产文件；账本双写入点时序、三道闸、能力位生成——硬禁区零触碰。
- 新增测试文件 `daemon/internal/agent/hermes_fork_window_test.go`（3 用例），复用同一
  `noteCommandsFromEvent` 入口与既有假客户端脚本（`commands-round` 轮内广告帧），
  不另写时序：
  - `TestHermesForkAdvertiseFillsViaTurnPump`——fork 子会话（`ExternalSession=fork-sess-new`）
    首轮轮内广告经泵入账（`help`/`model` + `inputHint` 映射），父线程账本保持 nil，
    外来 sid 帧被消费侧二道防线拦下；
  - `TestHermesForkEmptyAdvertiseClearsViaPump`——fork 路径显式空广告经泵覆盖旧账本
    （非 nil 空清单）；非命令变体、外来 sid 空广告均不得误清；
  - `TestHermesForkHasNoDedicatedCaptureEntry`——反射守卫：`HermesProvider` 与
    `hermesClient` 依赖面不得出现含 Fork 的方法。将来真接通分叉而新增 provider 侧
    fork 方法时本用例变红，强制同步加窗口捕获器（复用 `beginSlashCapture` helper）。
- `slash:hermes` 位与 `ListSlashCommands` 行为不变：server 包零改动，位测试绿证（见 §三）。

---

## 三、测试证据原文

### 3.1 基线（改动前）

```
C:\Go\bin\go.exe build ./...                        → BUILD_EXIT=0
C:\Go\bin\go.exe test -count=1 ./internal/agent/    → ok 17.851s
基线用例数（-json 去重统计）：PASS=768 FAIL=0 SKIP=9
C:\Go\bin\go.exe vet ./internal/agent/              → VET_EXIT=0
gofmt -l internal/agent/（基线清单，9 项）：
  codex_approval_wait_budget_20261009_test.go / dsh.go / hermes.go /
  hermes_compact.go / hermes_image_input.go / hermes_image_input_test.go /
  hermes_pump_badframe_20261009_test.go / pi_events.go / zcode.go
```

### 3.2 新用例先行（只跑 fork 过滤）

```
C:\Go\bin\go.exe test -count=1 -run 'TestHermesFork' -v ./internal/agent/
=== RUN   TestHermesForkAdvertiseFillsViaTurnPump
[hermes] ACP 适配器已就绪 caps(loadSession=true list=true resume=true fork=true)
[hermes] 新建会话 fresh-sess cwd=C:\ws
[hermes] session/prompt session=fresh-sess 输入=2字 图片0张
[hermes] 一轮完成 session=fresh-sess 用时 16ms 回复 4 字
[hermes] 已重挂载会话 fork-sess-new(历史回放 1 帧,当前模型 "")
[hermes] session/prompt session=fork-sess-new 输入=23字 图片0张
[hermes] 一轮完成 session=fork-sess-new 用时 3ms 回复 4 字
--- PASS: TestHermesForkAdvertiseFillsViaTurnPump (0.02s)
=== RUN   TestHermesForkEmptyAdvertiseClearsViaPump
--- PASS: TestHermesForkEmptyAdvertiseClearsViaPump (0.00s)
=== RUN   TestHermesForkHasNoDedicatedCaptureEntry
--- PASS: TestHermesForkHasNoDedicatedCaptureEntry (0.00s)
PASS / ok 2.730s
```

### 3.3 改动后全量复跑

```
C:\Go\bin\go.exe build ./...                        → BUILD_EXIT=0
C:\Go\bin\go.exe vet ./internal/agent/              → VET_EXIT=0
C:\Go\bin\go.exe test -count=1 ./internal/agent/    → ok 17.115s
用例数（-json 去重统计）：PASS=771 FAIL=0 SKIP=9   （+3，全部为本批次新增）
C:\Go\bin\go.exe test -count=1 -race -run 'TestHermesFork|TestHermesSlash' ./internal/agent/
                                                    → ok 4.149s
C:\Go\bin\go.exe fmt 未跑（纪律）；gofmt -l internal/agent/ 清单与基线逐字相同
  （9 项同名同序；新增 hermes_fork_window_test.go 不在清单里 = gofmt 干净）
server 侧只读复核（零改动）：
C:\Go\bin\go.exe test -count=1 -run 'TestGetHostInfoCapabilityBitsAlignedWithControllers|TestCapabilityBitVocabularyMatchesMatrix|TestListSlashCommandsHermes' ./internal/server/
                                                    → ok 3.381s
  （slash:hermes 位与 ListSlashCommands hermes 路由行为不变）
```

### 3.4 live 验证

未跑。本机 hermes 真会话 fork 需真实 fork 语义介入（daemon 侧无 fork 入口，
live 亦无调用链可走），以 mock 为准并如实标注：mock 覆盖 fork 首轮泵路径的
到达形状（应答后通知 → 泵消费侧按 sid 对表），与 D1 live 实测的 create 路径
帧形同源（同一 `available_commands_update` 变体、同一 `noteCommandsFromEvent`
入口）。

---

## 四、改动文件清单

| 文件 | 性质 |
|---|---|
| `daemon/internal/agent/hermes_fork_window_test.go` | **新增**：3 用例（fork 首轮泵入账端到端 / 空广告覆盖 / fork 入口缺席反射守卫） |
| `docs/reviews/2026-10-15/d4-hermes-fork-window.md` | **本报告** |

生产代码零改动（`git status` 生产侧仅本新增测试文件为本人改动；flutter 等
其他未跟踪/已修改项为并行子代理所有，未触碰）。未 commit、未 add、未 push。

---

## 五、残留边界

1. **fork 窗口无调用链可挂，第一轮靠泵路径**：本次登记的正式结论。若适配器在
   fork 应答后 ~1ms 发广告，而 fork 会话的首个 daemon 轮次尚未 Run（泵未装），
   该帧仍无人承接——与 D1 前 create 窗口修复前的形态相同。根治需等“hermes 分叉”
   功能本身立项（server `ForkThread` hermes 分支 + agent 侧 fork 方法），届时
   `TestHermesForkHasNoDedicatedCaptureEntry` 会变红，强制在新链上复用
   `beginSlashCapture` 加窗口（届时 D1 §六.3「将来的第四家形状」落地）。
2. **泵路径覆盖的到达形态**：fork 应答后的广告若迟于首轮 Run 的泵安装到达，
   本批次即已覆盖（`TestHermesForkAdvertiseFillsViaTurnPump` 为证）；早于泵安装
   的形态仍是缺口（见上一条）。
3. **其余 D1 §六残留（1/2/4–8 项）**：本批次未动（进程内存账本、历史会话面板、
   轮内重广告预收编、agent 侧解 Raw、提交语义、Android 侧、live 用例默认 skip
   ——均维持 D1 口径）。
