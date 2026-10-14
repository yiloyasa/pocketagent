# d1 · hermes 斜杠命令位化接线报告

> 日期：2026-10-14 · 批次：hermes 斜杠命令位化（`slash:<agentId>` 能力位 + ListSlashCommands hermes 路由 + provider 命令账本）
> 范围：只改 `daemon/internal/agent/`（hermes*.go 相关 + 新增）、`daemon/internal/server/`（slash_commands.go / agent_commands.go / host.go / host_capability_bits_test.go + 新增测试）、`docs/2026-09-30-agent-capability-matrix.md`（slash 相关行）。
> 红线遵守：`daemon/internal/hermesapp/` 零改动（只读）；`daemon/cmd/`、`flutter/`、`android/`、`proto/` 零触碰；未 git commit/add/push。
> 上游依据：`docs/reviews/2026-10-14/dsh-hermes-capability-probe.md` §三 hermes slash 行 + §四可补位 #1。

---

## 一、实现要点（6 条，与任务书逐条对应）

### 1. hermes 命令账本（写入点两处，见 §二判据）

新增 `agent/hermes_slash.go`：

- `parseHermesAvailableCommands(raw json.RawMessage) (sid, cmds, ok)` —— 从 `session/update` 原始帧解出 `availableCommands[]{name,description,input.hint}` → `[]*pb.SlashCommand`；变体判别用 `update.sessionUpdate == "available_commands_update"`；空名条目过滤；**显式空数组得到非 nil 空清单**（显式"没有命令"，与 zcode 空数组同纪律），nil 只留给"从未收到广告"。
- `hermesSlashCapture` —— 会话建立窗口的捕获器（begin → bind(sid) → **commit**（成功）/ **close**（失败）→ 被 Run 的泵整面替换）。建立 RPC 在途时命令帧先缓冲，成功后按会话号对表入账（别家会话的帧丢弃）；失败闭窗丢弃缓冲 —— 半挂载会话不留账本。
- `setSlashCommands / SlashCommandsFor` —— 账本读写，与 `threadMap` 同 `threadMu` 锁纪律；`dropThreadRef` 连带清账本（ephemeral 优化轮不泄漏）。
- `noteCommandsFromEvent(threadID, expectSID, ev)` —— 轮次泵消费侧写入（含消费侧 sid 二道防线；压缩轮自装钩子路径没有泵过滤器兜底，靠它拦）。

`hermes.go` 接线：
- `createSession`：装捕获器 → `SessionNew` → `bind + commit`（成功路径**不卸**，留给泵整面替换）；失败 `close + SetTurnHandlers(nil,nil)`。
- `remountSession`：捕获器替换原弃帧计数器（`replayed` 计数语义保留，命令帧不混进回放数）；`LoadSession` 失败 `close + 卸`；成功 `commit` 且不卸。
- Run 泵消费者：`p.noteCommandsFromEvent(turnThreadID, sid, nm.ev)` 先于 `projectSessionUpdate`（**台账留痕照旧保留**，二者不互斥）。
- `projectSessionUpdate` 的 `ev.CommandsSeen > 0` 分支：注释更新为"留痕保留 + 账本另记"，行为不变。
- `hermes_compact.go`：压缩轮自装钩子也接命令帧入账（`noteCommandsFromEvent(t.ID, sid, ev)`），不混进压缩回执正文。

### 2. `SlashCommandsFor(threadID)`

与 pi/zcode 同签名同语义：账本命中返回命令；未命中返回 **nil**。未记账绝不冷启适配器（本家连可拉的协议方法都没有，见 §二）。编译期钉死 `var _ interface{ SlashCommandsFor(string) []*pb.SlashCommand } = (*HermesProvider)(nil)`。

### 3. server 路由

`slash_commands.go` 的 `ListSlashCommands` 加 `hermes` 分支：`s.cfg.Providers["hermes"].(interface{ SlashCommandsFor(string) []*pb.SlashCommand })` 类型断言（照 pi 分支模式，不写死 `*agent.HermesProvider`）。**无兜底拉取**（对本家不存在可拉的口子）；`req.Refresh` 不改变通路。文件头「面板现状」补 hermes 段（五家全量）；入口注释与 `agent_commands.go` 分工注释同步。

### 4. host.go 位生成

新增 `slash:<id>` 位，两形状任一在场即宣告（与 compact「多形状一位」同纪律）：

```go
if _, ok := provider.(threadShapedSlashSource); ok {   // SlashCommandsFor(threadID)
    caps = append(caps, "slash:"+id)
}
if _, ok := provider.(cwdShapedSlashSource); ok {      // SlashCommands(cwd)
    caps = append(caps, "slash:"+id)
}
```

窄接口在 host.go 独立命名（`threadShapedSlashSource` / `cwdShapedSlashSource`），不 import 路由文件符号（照该文件既有的独立命名纪律）。

### 5. 守护测试同步

- `server/host_capability_bits_test.go`：五家构造器对表补 slash 格 —— codex/pi/zcode/hermes 有位，**dsh 无位（断言不得虚报）**。文件头位清单补 slash 行。
- `agent/capability_gate_test.go`：`capabilityBitFamilies` 增 `slash` 族；stance 表新增 `slash` 行（四家：codex/pi/zcode=true、dsh=false；hermes 第五家由 server 侧端到端钉）。
- `agent/hermes_pump_wiring_test.go`：安装史口径从"首条非 nil → 卸载"扩成三段式「捕获器 `{notif,req=nil}` → 泵 `{notif,req}` → 卸载」，并**强断言首条是捕获器形状**（它是 slash 账本写入点 1，缺了首轮面板必空）——原 P0-2 泵回归判据（Prompt 进入时刻双钩子非 nil）原样保留。

### 6. 矩阵文档

见 §四「已改行清单」。

---

## 二、账本写入点判据（哪条路径，为什么）

### 判据 1：`available_commands_update` 的到达时机 —— live 实测

适配器源码（只读静读 `G:\Claude Code\node_modules\hermes-agent\runtime\hermes-agent\acp_adapter\`）：

- 触发点仅两处：`server.py:596`（`_session_response_fields` 收尾，session/new · load · resume 共用）与 `server.py:668`（fork 应答后）；**没有第三条**（prompt 轮次内不重发）。
- 发送机制：`_schedule_soon`（`server.py:275-280`）= `loop.call_soon(asyncio.create_task, …)`，注释原文 *"Run a notification coroutine right after the current response is queued"*。即广告排在**建立应答之后**、不与应答同帧。

2026-10-14 本机 live 抓帧（一次性探针，隔离 HERMES_HOME，真家 `G:\Hermes` config.yaml/.env 只读复制，凭据经 env 注入不落盘不打印）：

```
6.305   resp   {"jsonrpc":"2.0","id":1,...initialize 应答...}
13.169  resp   {"jsonrpc":"2.0","id":2,"result":{"models":...,"sessionId":"85cef105..."}}   ← session/new 应答
13.170  notif  {"params":{"sessionId":"85cef105...","update":{"availableCommands":[
                {"description":"List available commands","name":"help"},
                {"description":"Show current model and provider, or switch models",
                 "input":{"hint":"model name to switch to"},"name":"model"},
                ... 共 9 条 ...],"sessionUpdate":"available_commands_update"}}}
13.170  notif  {"params":{...,"update":{"size":1000000,"used":9261,"sessionUpdate":"usage_update"}}}
```

**结论：广告在 `session/new` 应答后 ~1ms 到达，远早于 daemon 的轮次泵安装**（泵装在 `hermes.go` 的 `ensureSession` 之后，中间还隔着一次 `set_model` 往返 + 事件泵 goroutine 装配）。旧接线在此窗口的帧落在 `hermesapp` readLoop 的 `h != nil && h.Notif != nil` 判空处 → **静默丢**，谁也没接住。

### 判据 2：为什么写入点必须是「会话建立窗口 + 轮次泵」双点

| 路径 | 为什么需要它 |
|---|---|
| **会话建立窗口**（`createSession` / `remountSession` 的捕获器） | 接住 live 实测"应答后 ~1ms"的那条 —— 它是常规到达形态，没有它首轮面板必空。成功路径不卸（否则与 1ms 竞速，输了丢命令面）；失败路径闭窗（不留半挂载账本）。 |
| **轮次事件泵消费侧**（`noteCommandsFromEvent`） | 兜住"轮内到达"的将来形态：适配器改时机、fork 会话重广告、共享适配器上其它路径。同时它与 `projectSessionUpdate` 的 KNOWN_PASSTHROUGH 留痕并存，不互斥。 |

### 判据 3：ACP v0.11.2 有没有主动拉取命令清单的方法 —— **没有**

只读实读 `venv/Lib/site-packages/acp/meta.py`：

```
AGENT_METHODS = {authenticate, initialize, session_cancel, session_close,
                 session_fork, session_list, session_load, session_new,
                 session_prompt, session_resume, session_set_config_option,
                 session_set_mode, session_set_model}          ← 13 个，无命令清单方法
CLIENT_METHODS = {fs_read_text_file, fs_write_text_file,
                  session_request_permission, session_update, terminal_*}  ← 通知面
```

`available_commands_update` 是 agent→client 的 **session/update 通知变体**（schema.py:2442 `AvailableCommandsUpdate`），**纯推送，无请求方法**。故：

- 账本空 = 空面板，**没有第二条数据源可问**，绝不冷启适配器"问一遍清单"（与 pi 的"不冷启"同哲学，只是本家连可拉的口子都不存在）；
- 与 pi / zcode 的语义差异如实注释在 `hermes_slash.go` 文件头。

### 判据 4：语义与 pi/zcode 的对照

| 家 | 数据源形状 | 补拉兜底 | 本家落地 |
|---|---|---|---|
| pi | `get_commands` RPC（活客户端） | 有（Refresh 一次性子进程） | 不在本批次 |
| zcode | 快照 `slashCommands[]`（3 路记账） | 有（一次性 resume + 10min 缓存） | 不在本批次 |
| **hermes** | `available_commands_update` **推送**（会话建立窗口 + 轮次泵） | **无**（协议没方法） | 本批次 |
| **语义对照** | 与 pi/zcode 一致：**先落地后有清单**（用户发过第一条消息 / 会话建立后才有内容） | — | — |

---

## 三、测试证据原文

### 3.1 基线（改动前）

```
C:\Go\bin\go.exe build ./...                        → BUILD_EXIT=0
C:\Go\bin\go.exe test -count=1 ./internal/agent/ ./internal/server/
  ok  github.com/pocketagent/daemon/internal/agent   21.860s
  ok  github.com/pocketagent/daemon/internal/server  36.657s
基线用例数（-json 统计）：PASS=1345 FAIL=0 SKIP=9
```

### 3.2 改动后（每条实现跑一次；最终全绿）

```
C:\Go\bin\go.exe build ./...                 → BUILD_EXIT=0
C:\Go\bin\go.exe vet ./internal/agent/ ./internal/server/  → VET_EXIT=0
C:\Go\bin\go.exe test -count=1 ./internal/agent/ ./internal/server/
  ok  github.com/pocketagent/daemon/internal/agent   17.881s
  ok  github.com/pocketagent/daemon/internal/server  29.124s
用例数：PASS=1364 FAIL=0 SKIP=9   （+19，全部为本批次新增/扩面）
```

race 复核：

```
C:\Go\bin\go.exe test -count=1 -race ./internal/agent/ ./internal/server/
  ok  github.com/pocketagent/daemon/internal/agent   21.807s
  ok  github.com/pocketagent/daemon/internal/server  32.780s
```

全量 internal（27 包）：

```
C:\Go\bin\go.exe test -count=1 ./internal/...
  ok  github.com/pocketagent/daemon/internal/agency      2.068s
  ok  github.com/pocketagent/daemon/internal/agent       20.781s
  ok  github.com/pocketagent/daemon/internal/codexapp     6.028s
  ok  github.com/pocketagent/daemon/internal/collabmcp    4.791s
  ok  github.com/pocketagent/daemon/internal/dshapp       6.843s
  ok  github.com/pocketagent/daemon/internal/eventstore   3.424s
  ok  github.com/pocketagent/daemon/internal/externalconv 3.238s
  ok  github.com/pocketagent/daemon/internal/hermesapp    6.993s
  ok  github.com/pocketagent/daemon/internal/hidewin      1.415s
  ok  github.com/pocketagent/daemon/internal/hostenv      2.709s
  ok  github.com/pocketagent/daemon/internal/hostenv/goscan 1.674s
  ok  github.com/pocketagent/daemon/internal/jobobject    2.565s
  ok  github.com/pocketagent/daemon/internal/mdns         2.940s
  ok  github.com/pocketagent/daemon/internal/panel        3.381s
  ok  github.com/pocketagent/daemon/internal/pathguard    5.302s
  ok  github.com/pocketagent/daemon/internal/piapp       13.672s
  ok  github.com/pocketagent/daemon/internal/rotatelog    2.630s
  ok  github.com/pocketagent/daemon/internal/rpcwire      3.926s
  ok  github.com/pocketagent/daemon/internal/runtime      5.224s
  ok  github.com/pocketagent/daemon/internal/selfguard    1.704s
  ok  github.com/pocketagent/daemon/internal/server      28.140s
  ok  github.com/pocketagent/daemon/internal/stderrdrain  1.011s
  ok  github.com/pocketagent/daemon/internal/store        3.391s
  ok  github.com/pocketagent/daemon/internal/term         3.912s
  ok  github.com/pocketagent/daemon/internal/testagent    4.787s
  ok  github.com/pocketagent/daemon/internal/tlscert      3.139s
  ok  github.com/pocketagent/daemon/internal/zcodeapp    13.148s
```

### 3.3 新增/扩面的用例（全绿）

agent 包（`hermes_slash_test.go` 新增 14 用例）：

```
TestParseHermesAvailableCommandsLiveShape
TestParseHermesAvailableCommandsRejectsNonCommandFrames
TestParseHermesAvailableCommandsExplicitEmptyIsEmptyNotNil
TestHermesSlashLedgerFilledFromSessionNewWindow     ← 写入点 1(缓冲路径,live 形状)
TestHermesSlashLedgerFilledAfterResponseBeforePump  ← 写入点 1(应答后泵前,直达)
TestHermesSlashLedgerIgnoresOtherSessions
TestHermesSlashLedgerSessionNewFailureClosesWindow
TestHermesSlashLedgerFilledFromTurnPump             ← 写入点 2 + 台账留痕并存
TestHermesSlashCommandsForNeverColdStarts
TestHermesSlashLedgerDroppedWithEphemeralRef
TestHermesSlashLedgerRemountWindow
TestHermesSlashLedgerRemountGhostKeepsNil
TestHermesSlashLedgerEmptyAdvertiseOverwrites
TestHermesSlashCaptureConcurrent                    ← -race 下跑
```

server 包（`hermes_slash_route_test.go` 新增 5 用例）：

```
TestListSlashCommandsHermesLedgerWins
TestListSlashCommandsHermesLedgerMissReturnsEmpty
TestListSlashCommandsHermesRouteIsolatedByAgentID
TestListSlashCommandsHermesProviderAbsentOrUnshaped
TestListSlashCommandsHermesRealProviderEmptyLedger
```

扩面（既有用例在新行为下照绿）：

```
TestCapabilityBitVocabularyMatchesMatrix          （vocabulary 增 slash 族）
TestCapabilityStanceTableMatchesMatrix            （stance 表增 slash 行）
TestGetHostInfoCapabilityBitsAlignedWithControllers（五家构造器端到端，含 slash 格）
TestHermesPumpWiringInstalledBeforePrompt          （安装史三段式收口）
```

### 3.4 live 实测（可选路径：本机 hermes 可用，已跑通）

```
$env:PA_LIVE_HERMES="1"
C:\Go\bin\go.exe test -tags pa_live -count=1 -run TestHermesLiveSlashLedgerFills -v ./internal/agent/

=== RUN   TestHermesLiveSlashLedgerFills
    hermes_slash_live_test.go:89: 隔离 HERMES_HOME 就绪(真实家 G:\Hermes 只读复制配置;凭据经 env 注入)
2026/10/14 23:19:52 [hermes] ACP 适配器已就绪 caps(loadSession=true list=true resume=true fork=true)
2026/10/14 23:20:00 [hermes] 新建会话 b5443ceb-2738-4877-9e7d-fc74dff390c8 cwd=...\002
2026/10/14 23:20:00 [hermes] session/prompt session=b5443ceb-... 输入=5字 图片0张
2026/10/14 23:20:00 [hermes] 一轮完成 session=b5443ceb-... 用时 13.337s 回复 511 字
    hermes_slash_live_test.go:132: live 账本 9 条(sink 事件 3):
      [name:"help" description:"List available commands"
       name:"model" description:"Show current model and provider, or switch models" input_hint:"model name to switch to"
       name:"tools" description:"List available tools with descriptions"
       name:"context" description:"Show conversation message counts by role"
       name:"reset" description:"Clear conversation history"
       name:"compress" description:"Compress conversation context"
       name:"steer" description:"Inject guidance into the currently running agent turn" input_hint:"guidance for the active turn"
       name:"queue" description:"Queue a prompt to run after the current turn finishes" input_hint:"prompt to run next"
       name:"version" description:"Show Hermes version"]
--- PASS: TestHermesLiveSlashLedgerFills (13.36s)
```

**live 结论**：真实适配器广告的 9 条命令完整入账本（name/description/input.hint 三字段逐条对上）；与探针报告 §三的 9 条严格一致（这是**验证**，不是硬编码 —— 代码里零名单字面量）。live 用例默认 skip（`PA_LIVE_HERMES` 未设），只在显式开启时才起真进程。

### 3.5 冻结契约对表（跨线契约复核）

| 冻结项 | 实际 | ✓ |
|---|---|---|
| 位名 `slash:<agentId>` | host.go `caps = append(caps, "slash:"+id)` | ✓ |
| 位生成判据两形状 | `threadShapedSlashSource` / `cwdShapedSlashSource` | ✓ |
| hermes 实现形状 `SlashCommandsFor(threadID string) []*pb.SlashCommand` | 方法签名 + 编译期钉死 | ✓ |
| 从账本按 threadID 返回；无账本返 nil | `SlashCommandsFor` 读 `slashByThread[threadID]`（未记账 nil） | ✓ |
| 保留台账留痕 | `projectSessionUpdate` 的 `recordLedgerOnly` 分支原样 | ✓ |

---

## 四、矩阵已改行清单（`docs/2026-09-30-agent-capability-matrix.md`）

python 脚本按字节改 + 读回验收（`OK` 全过；bytes 27151 → 30334）；只改 slash 相关：

| # | 位置 | 改动 |
|---|---|---|
| 1 | 行 8 文件头增补注 | 追加「**2026-10-14 批次（slash 位化）**」一行：新增 `slash:<agentId>` 位族（A 区 + §四同步）、hermes 面板接入（B 区 §三 ❌→✅）、位生成判据、Android 门控改位消费 |
| 2 | §二标题 | 「13 位族 + 6 宿主级」→「14 位族 + 6 宿主级」 |
| 3 | §二主矩阵 A | 在 `image_input` 行后新增 `slash:<id>` 行（五家格：codex/pi/zcode ✅、dsh ❌、hermes ✅；判据=两形状任一在场；出处逐家列出，含 hermes_slash.go） |
| 4 | §二表后增补注 | 追加第 5 条「**slash 位化（2026-10-14 批次）**」：四家有位、dsh 缺位；hermes 双写入点（live 帧次序 13.169 resp → 13.170 notif）；未记账返 nil（不冷启无兜底拉取）；台账留痕保留；指向本报告 + §四接线 |
| 5 | B 区 §三 `slash_commands` 行 | hermes 格 ❌（无 SlashCommands 方法）→ **✅ provider 命令账本**（`SlashCommandsFor(threadID)`，agent/hermes_slash.go）；事实与出处列重写（账本形、"无清单 RPC"、Android 门控 `slashPanelAgentOf` 位优先 + 旧三源名单垫） |
| 6 | §四接线表 | 新增 `slash:<id>` 行：`slashPanelAgentOf`（`agentCapableOf("slash", …)` ‖ `isSlashPanelAgent` 旧三源名单垫）→ / 开头候选条可见性；出处 `SlashCommandsLogic.kt:78-104`，注明「位优先、无位回落旧三源名单」（**A2 子代理不碰矩阵，本行统一写**） |
| 7 | §一行 12 机制句 | 位生成出处 `host.go:75-155` → `109-208` 的 provider 循环 + slash 段 `:178-190` 单列 |

**读回验收**（python 逐串 contains）：`slash:<agentId>` ✓ / `slash:<id>`（斜杠命令面板数据源）✓ / `slash:hermes` 位在场 ✓ / `slashPanelAgentOf` ✓ / 「14 位族 + 6 宿主级」✓ / 「slash 位化）**：新增」✓。

**未改**（超出本批范围，如实登记）：§三 B 区其它行、§五遗留清单（slash 位化不引入新遗留）、§六守护索引（新测试已在 A 区出处列引用）。

---

## 五、改动文件清单

### 生产代码（4 改 1 增）

| 文件 | 性质 |
|---|---|
| `daemon/internal/agent/hermes_slash.go` | **新增**：命令解析 + 建立窗口捕获器 + 账本读写 + `SlashCommandsFor` + 泵侧补记 |
| `daemon/internal/agent/hermes.go` | 改：`HermesProvider` 增 `slashByThread` 字段；`createSession`/`remountSession` 装捕获器；Run 泵消费侧调 `noteCommandsFromEvent`；`dropThreadRef` 连带清账本；投影注释与文件头更新 |
| `daemon/internal/agent/hermes_compact.go` | 改：压缩轮自装钩子接命令帧入账（不混进回执正文） |
| `daemon/internal/server/slash_commands.go` | 改：`ListSlashCommands` 加 hermes 分支（类型断言、无兜底拉取）；文件头「四家面板现状」扩五家 + 入口/失败哲学注释更新 |
| `daemon/internal/server/host.go` | 改：新增 `threadShapedSlashSource` / `cwdShapedSlashSource` 窄接口 + `slash:<id>` 位生成段 |
| `daemon/internal/server/agent_commands.go` | 改：文件头分工注释更新（pi/hermes 分支的去向） |

### 测试（2 改 3 增）

| 文件 | 性质 |
|---|---|
| `daemon/internal/agent/hermes_slash_test.go` | **新增**：14 用例（解析/两写入点/隔离/失败面/收尸/并发） |
| `daemon/internal/agent/hermes_slash_live_test.go` | **新增**（`//go:build pa_live`）：live 账本填充实测（`PA_LIVE_HERMES=1` 开启；隔离 HERMES_HOME） |
| `daemon/internal/agent/hermes_provider_test.go` | 改：`fakeHermesClient` 增 `newErr` 注入；Prompt 脚本增 `commands-round` 命令帧分支 |
| `daemon/internal/agent/hermes_pump_wiring_test.go` | 改：安装史口径三段式（捕获器 → 泵 → 卸载），强断言首条是捕获器形状 |
| `daemon/internal/agent/capability_gate_test.go` | 改：`capabilityBitFamilies` 增 `slash`；stance 表增 `slash` 行（四家；+pb import） |
| `daemon/internal/server/hermes_slash_route_test.go` | **新增**：5 用例（账本命中/未记账/隔离/provider 缺席/真 provider 空账本） |
| `daemon/internal/server/host_capability_bits_test.go` | 改：slash 位对表（codex/pi/zcode/hermes 在场、dsh 不得虚报）；文件头位清单补 slash 行 |

### 文档（1 改 1 增）

| 文件 | 性质 |
|---|---|
| `docs/2026-09-30-agent-capability-matrix.md` | 改：§四 7 处（slash 相关；见清单） |
| `docs/reviews/2026-10-14/d1-hermes-slash-wiring.md` | **本报告** |

### 域外文件误触与复位（如实登记）

实现中途误跑 `go fmt ./internal/agent/ ./internal/server/`（非 `gofmt -l` 只读检查），导致 7 个**域外文件**被 gofmt 规范化（纯格式/import 排序，无语义改动；逐文件与 `gofmt(HEAD)` 比对确认）：

```
GOFMT-ONLY  daemon/internal/agent/dsh.go
GOFMT-ONLY  daemon/internal/agent/zcode.go
GOFMT-ONLY  daemon/internal/agent/pi_events.go
GOFMT-ONLY  daemon/internal/agent/hermes_image_input.go
GOFMT-ONLY  daemon/internal/agent/hermes_image_input_test.go
GOFMT-ONLY  daemon/internal/agent/hermes_pump_badframe_20261009_test.go
GOFMT-ONLY  daemon/internal/agent/codex_approval_wait_budget_20261009_test.go
```

已全部 `git checkout --` 复位,复位后与 HEAD **逐字节一致**（7 文件 SHA-256 复核 IDENTICAL）。**未 commit、未 add**；`git diff` 里不含这 7 个文件。

另：`hermes_compact.go` 上一处误引入的 `//` 归一化行（同一次误跑）已撤回,格式状态与 HEAD 一致（HEAD 该文件本身有一处 gofmt 遗留,未代为修整 —— 域内不越权）。`hermes.go` 的 gofmt 遗留（410+ 行注释续行）系 HEAD 既有问题,本批次不代修。

`daemon/internal/hermesapp/` 零改动（构建脚本核对 acp.go/client.go/locate.go/config.go 与 HEAD 逐字节一致）；`daemon/cmd/`、`flutter/`、`android/`、`proto/` 零触碰（`git status` 核）；未 commit。

---

## 六、残留边界

1. **账本是进程内存，不落库**：daemon 重启后账本空，直到下一次会话建立（重挂载或用户下一条消息）才会重新收到广告。这与 pi 的 `aliveClient` 语义一致（daemon 重启后 pi 也没有活客户端）；用户重启后首次打开会看到空面板（比 pi 更"诚实"：pi 有 `Refresh` 兜底一次子进程拉取，hermes 协议上连这条路都不存在）。**是否给 hermes 加"重挂载拉取"兜底：不做**（ACP 方法面无请求方法，见 §二判据 3）。
2. **历史会话未打开时的面板**：daemon 重启后、用户没打开过该 hermes 会话（未 Run / 未重挂载）时，`ListSlashCommands` 返回空列表（nil）——面板安静收起，不报错（与 pi/zcode 同哲学，见 `slash_commands.go` 文件头）。用户发过第一条消息（或打开会话触发重挂载）后自然点亮。
3. **fork 会话窗口**：适配器在 `session/fork` 应答后也发广告（`server.py:668`）。daemon 的 `ForkSession`（acp.go:211）目前**没有**捕获窗口（本批次改动面只覆盖 create/remount；fork 的 daemon 侧调用链在 agent 层尚无重挂载入口，属"将来的第四家形状"）。此路径的广告目前不落账本 —— 如实登记，不假装覆盖。
4. **轮内重广告**：适配器当前只在建立时发一次；`noteCommandsFromEvent` 的泵侧接线是**预收编**（若上游将来改时机、或 fork 会话出现，自动生效），当前帧序不经过它。
5. **`available_commands_update` 解析在 agent 侧解原始字节**：`hermesapp.StreamEvent` 只暴露 `CommandsSeen int`（本批次不许改 hermesapp 解析层），故 `parseHermesAvailableCommands` 在 agent 包解 `ev.Raw`。若将来 want 收编进 hermesapp 导出面（如 `StreamEvent.Commands []AvailableCommand`），agent 侧可收敛为读字段 —— 这是"以后"的活，不阻塞本批次。
6. **命令提交语义（面板点选后）**：手机点选命令后回填 `/名 ` 直接作为 prompt 文本发（适配器 slash 分派自动触发，live 已实证 `/help` `/compress` `/context` 均被拦截）。daemon 不需要额外路由；`/compress` 已是 `CompactThread` 的既有通路（hermes_compact.go），面板里的 `compress` 条目与它共用同一适配器命令 —— 不冲突、不重复实现。
7. **矩阵文档的 Android 侧**：`slashPanelAgentOf` 已在 android 仓（另一 android 子代理负责），本批次只写文档行；daemon 侧的位生成/路由已就绪，端到端点亮需 android 侧合并后生效。
8. **live 用例默认 skip**：`TestHermesLiveSlashLedgerFills` 在 `PA_LIVE_HERMES` 未设时 skip（不进 CI 常规门禁；与本项目既有 `pa_live` 例同款）。本机已实跑通（见 §3.4）。
