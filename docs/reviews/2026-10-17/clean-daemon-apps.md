# daemon 适配器层 + 小包 + cmd 入口 —— 代码整洁与测试面横切审查

- **日期**：2026-10-17
- **范围**：`daemon/cmd/**`（57 个子包）、`daemon/internal/{agency,codexapp,collabmcp,dshapp,hermesapp,piapp,zcodeapp,externalconv,panel,testagent,testkit,mdns,pathguard,jobobject,hidewin,stderrdrain,tlscert,term,version}`、**daemon 全域测试面横切**
- **零文件重叠确认**：不审 `server/` `agent/` `runtime/` `store/` `eventstore/` `rpcwire/` `selfguard/` `gen/` `hostenv/` `rotatelog/`（另两条线 / 另两线）
- **改动**：**无**。未修改任何代码/文件，未 git add/commit/push，未跑 `gofmt -w`（只读检查）。唯一的写动作是本报告本身。
- **执行环境**：ASCII junction `G:\pa_c1c` → `G:\项目1\pocketagent\daemon`（中文路径下 go 命令可用），量化脚本置于 `C:\Users\yanzi\AppData\Local\Temp\opencode\`，仓库内零脚本。

## 基线（只读，审查前实测）

| 命令 | 结果 |
|---|---|
| `go build ./...` | **绿**（exit 0，无输出） |
| `go vet ./cmd/... ./internal/...` | **绿**（exit 0，无告警） |
| `go test ./cmd/...` | **绿**（3 包有测试：collabrolecheck / paguard / pocketagentd；**54 包 "no test files"**） |
| `go test -race -count=1`（本域 20 包 + ./cmd/...） | **全绿**，20 包 ok，exit 0 |
| go 版本 | `go1.27.1 windows/amd64`（`go.mod: go 1.27.1`） |

规模：本域 **205 个 .go 文件 / 1275 个函数**（132 源 + 73 测试）。`cmd/` **651.1 KB / 79 文件 / 57 子包**；11 个适配器+小包合计 **411.7 KB 源 / 385.9 KB 测试**。**`cmd/` 源码量是整个适配器层的 1.6 倍。**

---

## 0. 结论摘要

| 维度 | 结论 | 级别 |
|---|---|---|
| **红线核查（P0-1）** | `hermesapp` 在 `HERMES_HOME` 未设时**写用户的 `~/.hermes/config.yaml`**，与 AGENTS.md 红线 2 直接冲突，且与本包自己的包注释、探针的强制沙箱要求**三处互证矛盾** | **P0** |
| 适配器同型重复 | 6 家 × 9 维度矩阵：5 格**不该收**（语义天然异构）、4 格**已收**（rpcwire/stderrdrain/hidewin/jobobject）、**3 格该收**（handler 快照发布四份逐字副本 / ACP 权限挑选器两份逐字副本 / 数据目录派生四份三种兜底） | **P1** ×3 |
| 适配器域漏收 | `hermesapp` 是 5 家里**唯一不走 `stderrdrain`** 的，子进程 stderr 全丢 | **P1** |
| 谎言注释 | 9 处高危逐句对账，**6 处证伪**（piapp 头注释 3 条 / main.go mDNS 1 条 / hermesapp 包注释 1 条 / 依赖借口 1 条） | **P1** ×2 / P2 ×4 |
| 测试面横切 | **仓库内无任何 CI 配置、无 git hook**：全部条件跳过只在单台开发机上求值；`mdns`（唯一的局域网暴露面）自述在 CI 必跳 | **P1** |
| 命名 / 死代码 / 接口 / 参数 | **干净**：噪声词近零、**0 处未引用私有符号**、全域仅 1 个 ≥3 方法接口、132 源文件中仅 15 个 ≥5 参函数 | — |
| 断言质量 | **异常好**：daemon 全域 **7420 处断言 100% 带消息，0 处裸 `t.Fatal()`/`t.Error()`** | 已查但干净 |
| 长测试函数 | 1663 个测试函数，仅 **25 个 >100 行、4 个 >200 行**，全是表驱动 | 已查但干净 |

**计数：P0 = 1 · P1 = 9 · P2 = 16**

---

## 1. 红线核查（专项）

### 1.1 `pathguard/` 是干什么的 —— 它**不**保护本域的写路径

`pathguard` 的职责边界（`internal/pathguard/pathguard.go:1-19` 包注释 + `:170-192` GuardDeny 文档）非常清楚：**只管「删除/移动」与「协同工作区根」这两类不可逆路径操作的七项清单判定**。

- 全域调用点仅 2 处（都在 `internal/server/`，另一条线）：
  - `server/fs.go:343` — 删除/移动护栏
  - `server/collab_workspace.go:909` — 协同工作区护栏
- 它**不覆盖**任何适配器的「写用户配置」路径。红线 2（`~/.codex/`、`~/.dsh/`、`~/.pi/` 等只读）在 daemon 里**没有任何机械化执行点**——它是纯注释纪律。

> **结论**：`pathguard` 名字容易被误读成「全局路径安全网」，实际是「不可逆操作清单」。命名不构成谎言（包注释第 17-19 行明写「只回答拒不拒不命中哪条」），但**读代码的人容易以为红线 2 有人守**。这是 P0-1 得以长期存在 5 个月的结构性原因。

### 1.2 各 `*app/` 的写面普查（全域穷举 `os.WriteFile|Create|Rename|Remove|MkdirAll|OpenFile`）

| 包 | 生产代码写点 | 落点 | 红线 2 判定 |
|---|---|---|---|
| `codexapp` | **0 处** | — | ✅ 完全只读 |
| `zcodeapp` | 3 处（`generate.go:442/452/456`） | `~/.pocketagentd/model-pref.json` | ✅ daemon 自有目录 |
| `piapp` | 4 处（`mcpsetup.go:195/268/283/333`、`WriteCollabMcpConfigWithEnv`） | `ResolveCollabAgentDir()` → 默认 `~/.pocketagentd/pi-agent` | ⚠️ 见 §1.4 |
| `dshapp` | 2 处（`collabpatch.go:70/73`） | 隔离 HOME `~/.pocketagentd/dsh-home/profiles/acp/cordis.patch.yml` | ✅ daemon 自有目录 |
| **`hermesapp`** | **2 处**（`config.go:74/77`） | **`~/.hermes/config.yaml`**（`HERMES_HOME` 未设时） | ❌ **违反红线 2** |

### 1.3 【P0-1】`hermesapp` 写用户桌面 agent 配置目录

**证据链（四段互相独立，逐段可核）**

1. **落点是用户真目录**
   `internal/hermesapp/config.go:25-33`
   ```go
   func ConfigPath() string {
       if home := strings.TrimSpace(os.Getenv("HERMES_HOME")); home != "" {
           return filepath.Join(home, "config.yaml")
       }
       if home, err := os.UserHomeDir(); err == nil && home != "" {
           return filepath.Join(home, ".hermes", "config.yaml")   // ← 用户的 hermes 家目录
       }
       return ""
   }
   ```
   `config.go:73-77` 随后 `os.WriteFile(tmp, …, 0o644)` + `os.Rename(tmp, path)` 原地覆盖。

2. **与本包自己的包注释直接矛盾（谎言注释）**
   `internal/hermesapp/locate.go:13-15`
   > 「daemon **只读不改**用户家配置(与 ~/.codex 同一红线)」

   同一个包的 `config.go` 在写。注释是假的。

3. **探针侧被逼着做强制沙箱 —— 反证生产侧没做**
   `cmd/hermeffortprobe/main.go:10-11`（探针自己的红线声明）
   > 「红线:HERMES_HOME 用 -home 指到沙箱(强度写入落沙箱 config.yaml,**绝不碰 hermes 家目录**(见 HERMES_HOME))」

   `cmd/hermeffortprobe/main.go:30/34-38`
   ```go
   home := flag.String("home", "", "HERMES_HOME 沙箱(必填;强度写入落这里)")
   if *home == "" { fmt.Println("!! -home 必填"); os.Exit(1) }
   os.Setenv("HERMES_HOME", *home) // SetReasoningEffort + 适配器同读此环境
   ```
   探针必须**强制**覆盖 `HERMES_HOME`，否则它自己就会写到用户家目录。这说明**写用户目录这件事是已知的**，处理只落在探针层，没落到生产层。

4. **生产调用点确实可达**
   `internal/agent/hermes.go:328` → `hermesapp.SetReasoningEffort(e)`（用户在手机端显式选思考强度时触发）。`agent/` 是另一条线，但**被调的函数在我域内、落点在用户家**。

**为什么这是 P0 而不是 P1**

- 违反的是 AGENTS.md §4 红线 2（明文：「`~/.codex/`、`~/.dsh/`、`~/.pi/` **等**一律只读」），`~/.hermes` 显然在「等」的范围内。
- 同域的 dshapp / piapp / zcodeapp **全部**把落点隔离到 `~/.pocketagentd/`。hermesapp 是唯一的例外，不是「另一种设计」。
- 写入的是**用户手工维护的 YAML 配置文件**，且是**无 YAML 库的行级文本改写**（`config.go:46-72`）：它按 `TrimSpace + HasPrefix("reasoning_effort:")` 找行然后整体替换为 `  reasoning_effort: <v>`。若该键原本嵌在 `agent:` 下更深一层（`agent:\n  x:\n    reasoning_effort: …`），这行会被**提升到 agent 直接子级并丢掉原缩进** → 结构语义改变。文件本身还被 0644 写出（`config.go:74`）。
- **没有任何测试覆盖这条分支**：`internal/hermesapp/config_test.go:11-16` 的 `setHermesHomeForTest` 每个用例都 `t.Setenv("HERMES_HOME", t.TempDir())`，`ConfigPath()` 的 `~/.hermes` 回落分支**一次都没被执行过**。犯事的那条路恰好是没测的那条。

**修复代价 / 风险**

- 最小修法（对齐 dshapp/piapp）：把 `SetReasoningEffort` 的落点改到 `<daemon dataDir>/hermes-effort.json`，启动时用 `StartOptions` 注入 hermes 子进程 env（hermes 支持 env-seeded 配置，见 `locate.go:12-14`）。改 2 个文件（`config.go` + `hermesapp/locate.go` 补 env 注入），删掉手写 YAML 改写。
- 代价：`hermes` 桌面端与 daemon 的强度设置会分叉（用户在 hermes CLI 改强度，daemon 不再跟随）。这是**有意的隔离代价**，需要在 SPEC 里写明。
- 风险：不碰甲级禁区（`proto/**`）、不碰 `selfguard` 0.4.0 发布产物、不碰红线 1（审批）。改动面在 `hermesapp` 内 + `agent/hermes.go` 的调用点（跨线，需协调）。
- 若产品上坚持要「跟随用户在 hermes CLI 的设置」，那就必须走 `hermes` 自己的配置面（若有 CLI/RPC），而不是改文件 —— 这需要先确认 hermes 是否有只读配置查询口。**不确定点：hermes 上游是否提供读 `config.yaml` 的官方接口，我未核实**（上游源码在 `runtime/hermes-agent/`，本次未审）。

### 1.4 ⚠️【疑似，不定级为 P0】`piapp` / `dshapp` 的「尊重用户 env」写入路径

`internal/piapp/mcpsetup.go:115-123`
```go
// ResolveCollabAgentDir 解析协作路径实际使用的 agent 目录:
// 用户 env 已设 PI_CODING_AGENT_DIR → 尊重(隔离原则:不覆写用户显式配置…)
func ResolveCollabAgentDir() string {
    if p := strings.TrimSpace(os.Getenv(agentDirEnv)); p != "" { return p }
    return DefaultCollabAgentDir()
}
```
`cmd/pocketagentd/main.go:255-274` 默认开启（`-pi-collab` 默认 `true`，`main.go:118`）下，会向 `ResolveCollabAgentDir()` 写 `mcp.json` 并安装 `pi-mcp-adapter`（后者会改 `settings.json` 的 packages 列表）。若用户的 `PI_CODING_AGENT_DIR` 指向其真实 `~/.pi`，daemon 就在写用户桌面目录。`dshapp` 的 `DSH_HOME` 是同款。

**为什么只标疑似、不定 P0**：

- 这是**用户显式设置的环境变量**，且代码注释明确解释了「尊重用户显式配置」的设计取舍 —— 与 §1.3 的「隐式默认回落」性质不同。
- 默认路径（未设 env）**完全合规**（`~/.pocketagentd/pi-agent`）。
- 两家口径一致（dsh / pi 都尊重 env），不是单点漂移。

**不确定点**：`PI_CODING_AGENT_DIR` 在真实用户环境里的常见取值我未取证。若 pi 官方文档推荐把它设成 `~/.pi`，则风险等级要提到 P0。建议由能查上游文档的线确认。

### 1.5 ✅ 红线 1（绝不替用户审批）—— 本域干净

逐包核查审批面：

| 包 | 审批面处理 | 判定 |
|---|---|---|
| `dshapp` | `protocol.go:540-575` 只**解析** `session/request_permission` 并暴露 `AllowOptionID/RejectOptionID`；决策在 `agent/` | ✅ 不代拟 |
| `hermesapp` | `acp.go:583-617` 同款 | ✅ 不代拟 |
| `codexapp` | 全包无任何 approval/confirm 字样，ReqHandler 交 `agent/` | ✅ 不代拟 |
| `zcodeapp` | `client.go:605-654 autoAnswer` 只应答 3 个**协议握手**类方法（`startup/storagePath` / `session/requestRuntimePreferences` / `interaction/requestProviderRuntimeHeaders`），**无一是审批** | ✅ 不代拟 |
| `piapp` | `guard.go:63-74` 渲染的 TS 扩展**强制** `await ctx.ui.confirm(...)`，拒绝即 `{block:true}`，无任何自动放行分支 | ✅ 不代拟 |
| `collabmcp` | 无审批面（`engine.go:85` 只是状态字段 `waiting_approval`） | ✅ |

补充确认：`dshapp/protocol.go:568-574` 的兜底注释「拒绝是失败安全方向」——选不到显式 reject 时宁可回空串让上层报错，也不误选 allow。方向正确。

---

## 2. 适配器同型重复矩阵（6 家 × 9 维度，逐格判定）

**参与矩阵的 6 家**：codexapp / dshapp / hermesapp / piapp / zcodeapp / collabmcp（collabmcp 无子进程，D1/D3 格为 N/A）。
「已收」= 本轮之前的 `rpcwire` / `stderrdrain` / `hidewin` / `jobobject` 波次已收。

| # | 维度 | codex | dsh | hermes | pi | zcode | collabmcp | 判定 | 依据 |
|---|---|---|---|---|---|---|---|---|---|
| D1 | 进程启动参数 | `Start()` 无参 | `Start(StartOptions)` | `Start(pythonExe, acpDir)` | `Start(StartOptions)` | `Start(StartOptions)` | N/A | **不收** | 选项集按 agent 天然异构（`codexapp/daemonScopeArgs:154`、`piapp/buildRPCArgs:344`、`zcodeapp/zcodeLaunchSpec:161`）。唯一漂移点是 hermes 的 2 个位置参（其余 4 家用 `StartOptions` 结构）→ 记 **P2-11** |
| D2 | 配置文件发现与解析 | 只读 `~/.codex/config.toml` | 隔离 HOME + 凭据 env | **写 `~/.hermes/config.yaml`** | 读写隔离 `mcp.json` | 只读 `~/.zcode/v2` | 只读 `~/.codex` | **不收** | 各家红线面不同，这是「适配器按 agent 分化」的正当理由。**但 hermesapp 一格已升级为 §1.3 的 P0** |
| D3 | stdout/stderr 流式事件解析 | `stderrdrain` ✓ | `stderrdrain` ✓ | **裸 Scanner 丢弃** ✗ | `stderrdrain` ✓ | `stderrdrain` ✓ | N/A | **已收 4/5，hermes 漏** → **P1-3** | 见 §2.1 |
| D4 | token/用量统计 | 无 | 无 | 16 处 token 字段 | 4 | 5 | 无 | **不收** | 只有 3 家有，且 payload 字段名逐家不同（实测统计见下），抽公共层要造 3 分支的适配器 = 更复杂 |
| D5 | 能力位映射 | 不在本层 | 不在本层 | 不在本层 | 不在本层 | 不在本层 | 不在本层 | **不收（硬禁区）** | 能力位生成与位族词汇表是甲级禁区，且根本不在适配器层，全域 grep 零命中 |
| D6 | 错误分类与退出码语义 | `giveUp`/`abandonedProbe` TTL | `withMethod`/`IsMissingCredential` | `rpcError`/`ErrSessionNotFound`/`giveUpTurn` | `ExitError`/`callWithReadyRetry` | `remember(ok, errMsg, state)` | **不收** | 分类轴逐家不同（超时 vs 凭据缺失 vs 会话丢失 vs 退出码 vs 排队重试），强行统一会造一层「按 agent 分派的错误码表」，比现状差 |
| D7 | 会话状态机 | `pending`+`abandoned` | `pending`+`callTimeout` | `abandonedTurn`+`budget` | `sessState`+`deathCh` | `exited`+`autoAnswer` | 记录表 | **不收** | 同 D6 |
| **D8** | **handler 快照发布** | ✓ | ✓ | ✗（无此机制） | ✓ | ✓ | N/A | **该收** → **P1-6** | 见 §2.2 |
| **D9** | **daemon 数据目录派生** | N/A | `client.go:57` + `credentials.go:51` **两份** | N/A | `mcpsetup.go:104` | `generate.go:399` | N/A | **该收** → **P1-7** | 见 §2.3 |

### 2.1 【P1-3】`hermesapp` 是 5 家里唯一丢弃子进程 stderr 的

`internal/hermesapp/client.go:327-333`
```go
func (c *Client) drainStderr(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		_ = sc.Text() // hermes 把 stderr 当纯调试日志;daemon 外丢弃(高频刷屏)…
	}
}
```

对照另外 4 家（均已收口）：
- `codexapp/client.go:267-274` → `stderrdrain.Drain(r, stderrdrain.Options{}, line)`
- `dshapp/client.go:347` → 同
- `piapp/client.go:424` → 同（闭包注入 `log.Printf`）
- `zcodeapp/client.go:348` → 同

更关键的是，**这个病已经被本域自己诊断过并写进注释**：`codexapp/client.go:194-197`
> 「stderr 单独收进 daemon 日志(绝不混进 stdout…;也绝不透传 os.Stderr:…codex 的死因输出全丢 —— **piapp/zcodeapp 当年同款病,2026-09-20 补齐 codexapp,范式与二者一致**)」

`piapp/client.go:420-421` / `zcodeapp/client.go:345-346` 的注释也各写「把 pi / zcode / dsh 三家的同款逻辑收成一份(此前各抄一遍,口径已漂移)」。**四家都承认这是同款病并修好了,hermesapp 落在最后一格没跟上。**

**分级依据（降级理由）**：hermes 的 provider 报错本身走 RPC 错误通道（`acp.go` 的 `rpcError` + `data.details`），用户可见的死因**不依赖** stderr。丢的是 daemon.log 里的诊断线索，不是用户面数据 → P1 而非 P0。

**修复代价 / 风险**：1 个函数替换为 `stderrdrain.Drain(r, stderrdrain.Options{}, func(l string){ log.Printf("[hermes-stderr] %s", l) })`。代价≈0。风险：日志量增加（受 `DefaultMaxTotal=256KB` 限流保护）；顺带暴露 hermes 可能刷屏 —— 但这正是另外 4 家已经接受的取舍。

**顺带**：`stderrdrain` 自己的包注释（`stderrdrain.go:1-8`）写「pi / zcode / dsh 三个 harness…保证三家的死因日志形态一致」「早先 codexapp 也是同款自由函数思路,现统一收口」—— 收口后**没回头更新注释**，现在是 4 家；且 hermes 不在其中 → **P2-7**。

### 2.2 【P1-6】handler 快照发布：4 处逐字副本，3 个包

规范化 body 后逐字节哈希，**4 个函数体在 3 个包里完全一致**（sha 前 10 位相同）：

| 函数 | codexapp | dshapp | piapp | zcodeapp | body（归一化后） |
|---|---|---|---|---|---|
| `publishHandlersLocked` | `client.go:89` | `client.go:242` | `client.go:290` | `client.go:100` | `c.hs.Store(&xxxHandlers{global:…, notif:…, req:…, unknownResponse:…})` |
| `SetGlobalHandler` | `client.go:96` | `client.go:249` | `client.go:297` | `client.go:107` | `c.handlerMu.Lock() defer c.handlerMu.Unlock() c.GlobalHandler = handler c.publishHandlersLocked()` |
| `SetUnknownResponseHandler` | `client.go:103` | `client.go:256` | `client.go:304` | `client.go:114` | 同上模式 |
| `SetTurnHandlers` | `client.go:111` | `client.go:264` | `client.go:318` | `client.go:121` | `c.NotifHandler, c.ReqHandler = notif, req` + publish |

**语义等价性已逐句核对**（不是靠哈希猜的）：
- 四处都是「`handlerMu` 加锁 → 写导出裸字段 → `publishHandlersLocked()` 把四元组打进 `atomic.Pointer` 快照」。锁语义、发布时机、字段映射四元组顺序在 4 处完全一致。
- **唯一差异是类型**：`SetUnknownResponseHandler` 的入参在 codexapp/piapp/zcodeapp 是 `func(json.RawMessage, json.RawMessage)`（RawMessage id），在 dshapp 是 `func(int64, json.RawMessage)`；`SetTurnHandlers` 的 req 同理。**但函数体本身一字不差**——差异只在字段的声明类型。

**收法**：`rpcwire`（或新建 `rpchandler`）加一个泛型 `HandlerBox[T]{mu sync.Mutex; cur atomic.Pointer[T]; publish func(T)}`，三家 Client 各自的 `xxxHandlers` 结构体作为 `T`，Setter 变成一行 `c.hb.Set(&xxxHandlers{...})`。
**影响面**：3 个包 × 4 个文件 × ~40 行，纯内部重构；**不改任何 wire 契约、不改 JSON-RPC id 类型、不碰 proto、不碰红线**。现有 `handler_concurrency_test.go`（codexapp / piapp / zcodeapp 各有 1 份）可直接作回归钉。

**为什么不整包收 Client 层**：见 §2.4。

### 2.3 【P1-7】daemon 数据目录派生：4 份手搓，3 种失败兜底

单一事实来源已经存在：`hostenv.UserDataDir()` / `LegacyDataDir()`（`pathguard` 就走它，`pathguard.go:300`；`hermesapp/locate.go:41` 也走 `hostenv.NpmGlobalRoots()`）。但适配器层手搓了 4 份：

| 位置 | 代码 | 取不到 home 时兜底 |
|---|---|---|
| `dshapp/client.go:57-66` `defaultDshHome` | `filepath.Join(home, ".pocketagentd", "dsh-home")` | **`TempDir()/pocketagentd/dsh-home`** |
| `dshapp/credentials.go:51-60` `defaultDataDir` | `filepath.Join(home, ".pocketagentd")` | **`TempDir()/pocketagentd`** |
| `piapp/mcpsetup.go:104-113` `DefaultCollabAgentDir` | `filepath.Join(home, ".pocketagentd", "pi-agent")` | **`""`（不注入/不写）** |
| `zcodeapp/generate.go:399-408` `defaultModelPrefPath` | `filepath.Join(home, ".pocketagentd", "model-pref.json")` | **`""`（读写都 no-op）** |

**语义等价性判定：形状等价、失败语义不等价** —— 这正是「必须先证明语义等价再收」的教科书案例。同一个「拿不到数据根」的事件，dshapp 静默落到**临时目录**（每次重启位置可能不同、临时目录会被清理 → 凭据与 profile 补丁可能丢），piapp/zcodeapp 则**什么都不做**。

**收法**：四份全部改为 `hostenv.UserDataDir()` + `filepath.Join(root, "<sub>")`，兜底统一走 `hostenv` 的既有语义。
**影响面**：4 处函数、`hostenv` 是另一条线的域（**需协调，但其 API 已存在、无需改签名**）；行为变化 = dshapp 在异常环境下不再落 TempDir（这是修正）。
**风险**：低。不碰红线（落点仍全在 `~/.pocketagentd/`）。

### 2.4 为什么 Client 层（readLoop/dispatch/pending）不收 —— 伪命题澄清

5 家的 `client.go` 结构上高度平行（`piapp/client.go` 877 行最大，`dshapp` 648 / `zcodeapp` 667 / `codexapp` 650 / `hermesapp` 547，合计 3389 行），成员清单几乎一一对应（`Start`/`CallCtx`/`Call`/`forget`/`writeLine`/`Respond`/`RespondError`/`readLoop`/`dispatch`/`PID`/`Alive`/`reapWithLog`/`Close` —— 13 个名字 4/5 家全有）。

**但归一化 body 比对后，收敛结论是：已经收的收了，剩下的不该收。**

- **已收**：`writeLine`（4 家全部是一行委托 `c.wire.WriteLine(c.stdin, raw, writeBudget)`）、`reapWithLog`（4 家全部委托 `c.wire.ReapWithLog`）、`drainStderr`（4/5 → 见 §2.1）、子进程 `Wait`（`rpcwire.Wire.ReapWithLog` 独占，`waitOnce` 保证一次）。
- **剩下最接近的 `forget`（4 家逐字相同）**：`c.mu.Lock(); delete(c.pending, id); c.mu.Unlock()` —— 3 行。抽公共层需要泛型化 `pending map[K]chan T` + 暴露锁，收益低于它引入的抽象成本。**不收。**
- **`readLoop`/`dispatch` 为什么不收**：逐句对比后差异是本质的 ——
  - ID 类型：`int64`（codex/dsh/zcode）vs `string`（pi）vs `json.RawMessage`（zcode 的 Respond 侧）
  - 分帧：`\n` 分隔 JSON 行 vs LSP `Content-Length` 分帧（hermes `writeFrame:359`）
  - 迟到应答处理：codex `abandonedProbe` + 10min TTL vs hermes `abandonedTurn` + budget vs dsh/pi/zcode 直接丢弃
  - 反向请求应答形状：dsh `Respond(id int64, …)` / zcode `Respond(rawID json.RawMessage, …)` / pi `RespondUI(id string, payload map)`
  
  把这些抽成一个泛型 `jsonrpcClient[K]` 会得到一个带 6 个类型参数 + 2 个策略接口的抽象体 —— 违反「能收才收，不收就写清理由」。

**结论**：适配器层的「同型重复」在**传输小件层已经收干净**；剩下的重复是**协议异质性**的表征，不是债务。D8/D9 是例外，因为它们是纯机制、不带协议语义。

### 2.5 ACP 权限挑选器：dshapp 与 hermesapp 两份逐字副本（判为**伪命题**，不进 P1）

规范化哈希命中 2 组：

| 函数 | dshapp | hermesapp | 判定 |
|---|---|---|---|
| `AllowOptionID` | `protocol.go:553-560` | `acp.go:595-602` | **语义等价**（都是 `HasPrefix(lower(Kind),"allow")`），但**不该收** |
| `RejectOptionID` | `protocol.go:562-575` | `acp.go:606-617` | **语义等价**（同前缀 + 同「最后一个非 allow」兜底） |
| `PermissionOutcomeSelected` | `protocol.go:583-587` | `acp.go:622-626` | **语义等价**（同一 JSON 形状） |

**为什么不收**：dsh 和 hermes 都是 ACP（Agent Client Protocol）说话方，但它们是**两套独立的用户安装**（`@deepseek-ai/dsh` 与 Nous Research 的 `hermes-agent` npm 包）。ACP 的 option 语义由各自版本定义；共享一份实现等于把两个供应商的协议演进绑死——任一方改 option 语义，另一方被连坐。这正是「adapter 按 agent 分化」的正当理由，与 D2/D6/D7 同性质。

**唯一记 P2 的**：drift。`dshapp/protocol.go:546` 用 `errors.New("session/request_permission 缺 options 数组")`，`hermesapp/acp.go:589` 用 `fmt.Errorf(同文案)` —— 同一条错误消息、同一个空参数分支，两种构造。见 §6 P2-6。

---

## 3. 命名

### 3.1 噪声词清点（仅源文件标识符，132 个文件）

| 噪声词 | 命中数 | 判定 |
|---|---|---|
| `Handler` | 77 | ✅ **不是噪声**。全部是 JSON-RPC handler（`NotifHandler`/`ReqHandler`/`SetTurnHandlers`），是该协议的固有词汇，不是「Handler 类」的兜底名 |
| `Builder` | 19 | ✅ 全部是 protobuf 生成的 `Builder` |
| `Manager` | 6 | ✅ `term.Manager`（终端会话管理器）、`NewManager`、Android 侧的 `NsdManager` |
| `Adapter` | 3 | ✅ `acpAdapterDir`（ACP 适配器**目录**，路径名词）、`AdapterInstalled`（谓词）、`piAdapterPackage`（包标识） |
| `Helper` | 3 | ✅ `cmd/hermapprovalprobe`、`cmd/imgprobe` 等测试 helper 进程函数 |
| `Util` / `Utils` / `Processor` / `Wrapper` / `Facade` | **0** | ✅ |

**判定：本域命名噪音近零。不列发现。**

### 3.2 `-app` 后缀一致性

`codexapp` / `dshapp` / `hermesapp` / `piapp` / `zcodeapp` —— **5/5 一致**，全部小写（符合 Go 包名约定）。`collabmcp` 无后缀 —— 但它不是 agent 适配器（是 daemon 内嵌的 MCP 服务器），不加 `-app` 反而更准。✅ 一致。

### 3.3 【P2-1】`agency.SourceFunc.Name_` 尾下划线

`internal/agency/agency.go:41-49`
```go
type SourceFunc struct {
	Name_   string                        // ← 尾下划线,只为避开 Name() 方法
	ProbeFn func(ctx context.Context) ([]Candidate, error)
}
```
Go 里尾下划线是非惯用记号。惯用解法是构造函数（`func NewSource(name string, fn ...) SourceFunc`，字段保持 `Name`/`Probe`），或把 `Name_` 改为嵌入的具名 struct。当前只有 1 个生产调用点（`agent/pi_models_store.go:35`）+ 1 个测试点 → 收进构造函数成本≈0。

### 3.4 同一概念多名

- `agent/provider/backend/vendor` 混用：本域**未发现**混用。适配器包统一称 agent（`agent.DetectProviders`、`agent_id`、`Agency`）。
- `Locate*` 前缀：6 家一致（`LocateCodex` / `LocateDsh` / `LocateHermes` / `LocatePi` / `LocateZCode`），piapp 内另有 `LocateNode`/`LocateCLI` 两个组成部分 —— 命名层次清楚。✅

---

## 4. 函数与结构

### 4.1 超长函数（>100 行）top —— 全域 49 个，其中生产 16 个

工具：先掩码字符串/字符/注释再配对花括号，`func x() expr`（无花括号体）按 +6 行封顶。**逐字核对 2 例**（见「已知工具局限」）。

**生产代码 >100 行（全域 top 8）**

| 行数 | 位置 | 函数 | 备注 |
|---|---|---|---|
| **487** | `cmd/pocketagentd/main.go:66-552` | `main()` | **god function** → **P2-9**。见 §4.2 |
| 213 | `cmd/piprobe/main.go:567-779` | `runner.runAll` | 探针 |
| 207 | `cmd/dshprobe/dshproto020.go:100-306` | `doProto020` | 探针 |
| 193 | `cmd/dshprobe/main.go:1328-1520` | `main` | 探针 |
| 178 | `cmd/piprobe/smode.go:22-203` | `runner.runSMode` | 探针 |
| 178 | `cmd/piprobe/forklive.go:24-201` | `runForkLiveMode` | 探针 |
| 119 | `internal/term/manager.go:234-352` | `Manager.spawnSession` | 生产 |

**判定**：>100 行的**生产**函数只有 7 个（1 个真 god function + 6 个探针）。探针长是合理的（顺序化实证脚本）。**不构成债。**

### 4.2 【P2-9】`cmd/pocketagentd/main()` 487 行 + 零测试 + 无包注释

- `main.go:66-552` 单函数 487 行：26 个 flag 定义 + 数据目录解析 + guard 子模式 + 鉴权模式 + pi/dsh 协作接线 + TLS + mDNS + panel + 信号处理 + 3 个 gRPC 注册 + 横幅打印。
- `cmd/pocketagentd/` 只有 2 个源文件（`main.go` 711 行 + `paircode.go`）和 1 个测试文件（`paircode_test.go`，只测 `paircode.go`）。**`main()` 本身 0 覆盖**（无 `TestMain*`、无子模式分发测试、无 flag 默认值回归钉）。
- 30/57 个 `cmd/*/main.go` 首行就是裸 `package main`，**无包注释**（`pocketagentd` 也在其中）。

**修复代价**：把「子模式分发」（`guard-child` / `collab-mcp` / 正常启动）抽成 `func run(submode) error` + `main()` 只做 flag.Parse + 分发；flag 默认值用表驱动测试钉住（`-pi-collab` 默认 `true` 这次就被头注释写成 `false`，说明默认值回归无钉是有代价的 —— 见 P1-1）。
**风险**：低。不碰红线；`main()` 重排不影响行为（需 review 确认）。

### 4.3 god object（>1000 行）top 10

**全域无 >1000 行的 `.go` 源文件**（最大的 `cmd/dshprobe/main.go` 1521 行是探针）。适配器层最大的生产文件是 `internal/piapp/client.go` 877 行 —— 低于阈值，且职责单一（pi stdio JSON-RPC 客户端）。**不列 god object。**

### 4.4 超大接口

**全域（132 源文件）只有 1 个 ≥3 方法的接口**：`panel.Controller`（5 方法，`panel.go:38-44`）。适配器层**全部用 struct + 方法，零接口**。

**判定：本域无超大接口。不列发现。** （这是刻意的设计选择，也是 §2.4「不抽泛型 Client 层」的正面论据。）

### 4.5 参数 >4 个 / 布尔 flag 参数

**≥5 参数的函数全域仅 15 个**，源文件里 8 个：

| 位置 | 参数 |
|---|---|
| `internal/term/manager.go:188` `Open` | `sessionID, cwd, shell string, cols, rows int, env map` — **6** |
| `internal/term/manager.go:234` `spawnSession` | 同上 6 项 |
| `internal/term/ctrl.go:75` `win32KeySeq` | `vk, sc, uc, kd, cs, rc int` — 6 个 Win32 键位标志位 |
| `internal/zcodeapp/accounts.go:197` `keyFor` | 5 |
| `internal/collabmcp/delegate.go:249` `remember` | `threadID, agentID string, ok bool, errMsg, state string` — 5 |
| `internal/testagent/fake_agent.go:767` `recordLedger` | 5 |
| `cmd/pocketagentd/main.go:561` `printConsoleBanner` | 5 |

**判定**：`term` 的 6 个参数全部是「一次终端会话的规格」（id/路径/shell/尺寸/env），内聚 → 可收成 `SessionSpec`，但不是债（**P2-15**）。`win32KeySeq` 的 6 个 bool 参数是 Win32 `KEYEVENT_RECORD` 的直译，抽成 struct 反而不如顺序位标志直观。

**布尔 flag 参数**：`codexapp.call(..., probe bool)`、`hermesapp.call(..., budget bool)` 是控制流开关而非「行为二选一」，可接受。

---

## 5. 注释

### 5.1 谎言注释 —— 高危 20 处抽查，**6 处证伪**

对账方法：注释断言 → 打开被指认的实现 → 逐句核对。

| # | 位置 | 注释断言 | 实现事实 | 判定 |
|---|---|---|---|---|
| 1 | `piapp/mcpsetup.go:16-19` | 「【总开关】本功能整体默认关闭」 | `cmd/pocketagentd/main.go:118` `-pi-collab` **默认 `true`**；`main.go:255-256` 无条件进分支 | ❌ **LIE** → **P1-1** |
| 2 | `piapp/mcpsetup.go:76-77` | 「SetCollabEnabled … 当前生产路径**无人调用**,默认关」 | `main.go:256` `piapp.SetCollabEnabled(true)` 就是唯一调用点 | ❌ **LIE** → **P1-1** |
| 3 | `piapp/mcpsetup.go:18-19` | 「collabmcp 服务器**尚未接线**,开着会让 pi 凭空带一个不可用的 MCP 服务器」 | `main.go:268-269` 把 `-collab-mcp` 子模式写进 `mcp.json`；`collabmcp` 包 3 文件 29.9 KB 已实现 | ❌ **LIE** → **P1-1** |
| 4 | `hermesapp/locate.go:13-14` | 「daemon **只读不改**用户家配置(与 ~/.codex 同一红线)」 | `config.go:74` 写 `~/.hermes/config.yaml` | ❌ **LIE + 红线违反** → **P0-1** |
| 5 | `cmd/pocketagentd/main.go:310` | 「mDNS 广告块仍读 *addr:它只取端口,且明文回环**本就不该向局域网宣告,零触碰**」 | `main.go:476-501` 无条件注册，`TLSEnabled: !*plaintext`（明文时 `tls=0` 照发） | ❌ **LIE** → **P1-2** |
| 6 | `dshapp/collabpatch.go:125` | 「插入排序:键极少,**不值得引 sort**」 | `sort` 是**标准库**，「引 sort」= 0 依赖；理由不成立 | ❌ **LIE（琐碎）** → **P2-8** |
| 7 | `stderrdrain.go:2,7` | 「pi / zcode / dsh **三个** harness」「早先 codexapp…现统一收口」 | 实际 4 家在用（codexapp/dshapp/piapp/zcodeapp），hermesapp 未用 | ⚠️ **DRIFT** → **P2-7** |
| 8 | `piapp/mcpsetup.go:10-14` | 「daemon 绝不动用户默认 pi 目录(~/.pi 红线)」 | 默认路径合规；但 `ResolveCollabAgentDir` 尊重 `PI_CODING_AGENT_DIR` env → 见 §1.4 | ⚠️ **有条件** |
| 9 | `dshapp/configoptions.go:13-14` | 「本文件全部只读解析,绝不写用户桌面 dsh 配置」 | ✅ 与实现一致（`FlattenOptionLeaves` 纯函数，无写） | ✅ |
| 10 | `codexapp/client.go:159-160` | 「红线同 daemonScopeArgs:覆盖只进本子进程内存,绝不写用户 ~/.codex/config.toml」 | ✅ 全包 0 写点（§1.2 实测） | ✅ |
| 11 | `codexapp/client.go:217-218` | 「收尸(cmd.Wait)由 reapWithLog 独占(waitOnce 保证只一次),这里绝不重复 Wait」 | ✅ `client.go:636` 委托 `c.wire.ReapWithLog`；`rpcwire/wire.go:128` `waitOnce.Do` | ✅ |
| 12 | `dshapp/collabpatch.go:12` | 「**归我们所有,不碰用户的 ~/.dsh**」 | ✅ 落点 `~/.pocketagentd/dsh-home/...`（`credentials.go:59` 实证） | ✅ |
| 13 | `piapp/mcpsetup.go:189-190` | 「凭据**不在本函数自动复制**…必须走用户引导确认后显式调用」 | ✅ `CopyUserCredentials` 无生产调用点（仅 `mcpsetup_test.go:339/356/363`） | ✅ |
| 14 | `term/ctrl.go:186-189` | 「超时后写可能仍在后台完成(goroutine 无从撤销…),经缓冲 channel 收尸,不泄漏」 | ✅ `errCh := make(chan error, 1)`（容量 1，无阻塞） | ✅ |
| 15 | `zcodeapp/generate.go:450` | 「临时文件 + 改名:半截文件不会出现」 | ⚠️ `tmp := modelPrefFile + ".tmp"` **固定名**（非 `CreateTemp`），且 `SetDataDir` 不持 `prefMu` → 见 **P1-4** | ⚠️ |
| 16 | `externalconv/externalconv.go:4-6` | 「本包对用户的 agent 目录**只读**…没有任何写入路径」 | ✅ 全包仅 `os.ReadFile`/`ReadDir`/`WalkDir` | ✅ |
| 17 | `collabmcp/collabmcp.go:4` | 「**四家** harness 唯一共用的工具语言」 | 5 家接入（hermes 2026-09/10 接入），未同步 | ⚠️ **DRIFT** → **P2-5** |
| 18 | `agency/agency.go:1,3` | 「**四家** agent 的…架构」「各家 agent(codex/pi/zcode/dsh)」 | 实际生产只实现 1 个 Source（`agent/pi_models_store.go:35`），`StoreFileSource` 仅探针用 | ⚠️ **覆盖面声明 vs 实现** → **P2-4** |
| 19 | `pathguard.go:60-62` | 「【平台】仅 Windows 可编译…daemon 全链本就 Windows-only」 | ✅ `jobobject_other.go` / `testkit/waitproc_other.go` 证明是全仓既定口径 | ✅ |
| 20 | `panel/panel.go:15-16` | 「Controller 接口根本没有取 token 的面」 | ✅ 5 个方法全是 policy/control-plane，无 token 面 | ✅ |

**统计：20 抽 6 假（30%）**，其中 2 条构成 P0/P1 级（4 号、5 号），1 组三条同源（1/2/3 号）。

### 5.2 【P1-1】`piapp/mcpsetup.go` 头注释三连假 —— 为什么这条值得单独列

三条假注释（§5.1 #1/#2/#3）都指向同一个已经翻掉的设计决策：2026-09-21 把 `-pi-collab` 默认从 `false` 改成 `true`（理由写在 `main.go:111-114`：用户报「派发子智能体直接失败」），**改了行为没回头改适配器的包注释**。

**危害**：下一个人（或下一轮审查）读 `piapp/mcpsetup.go` 会得出「这功能默认关、collabmcp 还没接线」的结论，可能据此做出错误决策（例如「先把 collabmcp 接线了再开这个开关」）。**这是注释滞后造成的事实性误导，不是措辞问题。**

**修复代价**：改 `mcpsetup.go:16-19` + `:76-77` 两段注释，约 8 行。**风险：0**（纯注释）。
**最佳修法**：把总开关的三条事实收敛成一处 —— 包注释写「生产默认开（`-pi-collab` 默认 true，见 cmd/pocketagentd/main.go）；`PA_PI_COLLAB` env 仅供手工调试；collabmcp 已接线」，`SetCollabEnabled` 的注释删掉「当前生产路径无人调用」这句。

### 5.3 【P1-2】mDNS：谎言注释 + 局域网信息暴露

**注释**（`main.go:310`）：「mDNS 广告块仍读 \*addr:它只取端口,且明文回环本就不该向局域网宣告,零触碰。」

**实现**（`main.go:475-501`）：无条件注册，`TLSEnabled: !*plaintext`。

**暴露面**（`internal/mdns/mdns.go:84-89`）：
```go
text := []string{
    "v=" + ProtocolVersion,
    "id=" + cfg.HostID,        // ← daemon 稳定主机标识,同网段任何设备可浏览得到
    "name=" + cfg.DisplayName, // ← 机器名
    "tls=" + tlsFlag,          // ← 明文模式下为 "0"
}
```

**实际影响分层**：
- 明文模式下 F2 裁决已把监听强制压回回环（`main.go:317-330`），所以**端口在 LAN 上不可达** —— 广告是误导性的，不会导致直接连接成功。
- 但**局域网内任何设备可通过一次 mDNS 浏览获得 `host_id` + 机器名 + 「有一个明文端口存在」这一事实**。`host_id` 的敏感度取决于 `server/` 侧的配对/设备注册语义（**另一条线的域，我未核实**，因此不确定它能否被用来冒充设备）。

**分级**：**P1**（不是 P0，因为可确认的泄露面止于「主机标识 + 机器名 + 端口存在性」，无凭据、无内容；且明文模式本身已是大声告警的应急路径）。
**修复代价 / 风险**：
- 最小修法（只改注释，0 风险）：把 `main.go:310` 改成如实描述「明文模式下仍会宣告，但 tls=0 让客户端自行判断；F2 已把监听压回回环，故 LAN 侧端口不可达」。
- 更彻底（可选）：`-plaintext` 时跳过 mDNS 注册，并给 `id=` 换成不稳定的会话级随机值（避免长期主机标识上广播）。后者**会破坏 mDNS 的核心用途**（手机靠它认机器），需要产品决策。
- 不碰红线 1/2，不碰甲级禁区。

### 5.4 注释掉的代码

在本域 132 个源文件中按「整行以 `//` 开头且含 `;`/`{`/`}`/赋值号且匹配 Go 语法」扫，**0 命中**。探针目录里的实验片段都是活代码。

### 5.5 TODO / FIXME 全清单

**本域 0 条。**（grep `TODO|FIXME|XXX|HACK|BUG` 在 `cmd/` + 19 个 internal 包上零命中；中文「待办/暂时/先凑合」等命中项经逐条核对，全部是「临时目录」「失败」「暂态」等业务词，非待办标记。）

**判定：TODO 纪律优秀。不列发现。**（`pathguard_test.go` / `mdns_test.go` / `server/security_regression_test.go` 的 `t.Skip` 里带 TODO 意的措辞属测试面，见 §7.3。）

### 5.6 变更日志式注释

有，但**质量高**：日期戳注释几乎全部附带「为什么」与「实证锚点」，例如
- `codexapp/client.go:236-248`（2026-09-20 引号剥离 bug 的活体日志证据）
- `codexapp/client.go:265-273`（stderrdrain 收口的两个缺口）
- `piapp/guard.go:42-46`（2026-09-19 EOF 崩溃的 TS ParseError 定位）
- `dshapp/configoptions.go:3-11`（两份探针证据的字段级结论）

这些不是「改了什么」的流水账，是「为什么必须这样」的知识沉淀。**不列发现。**

---

## 6. 错误处理

### 6.1 `fmt.Errorf` 缺 `%w`：全域 139 处，94 处有 `%w`，**45 处没有 —— 全部正确**

逐条核了 45 处无 `%w` 的：**没有一处包裹了 error 值**。它们分三类，全部合规：
1. `fmt.Errorf("未找到 codex(既没有安装也没有 CODEX_BIN 指向路径)")` — 纯新消息（Go 约定本可用 `errors.New`，但同包已用 `errors`，见 P2-6）
2. `fmt.Errorf("%s: 等待 codex 应答超时(%s)", method, callTimeout)` — 有格式参数，`fmt.Errorf` 正确
3. `fmt.Errorf("读取凭据源 %s 失败: %w", src, err)` — 这类**有** `%w`

**判定：`%w` 纪律干净。不列发现。**

### 6.2 【P2-6】`fmt.Errorf` 零格式参数（3 处）

`codexapp/client.go:470` / `:494`（`"响应里缺少可关联的请求 id,拒绝应答"`）+ `hermesapp/acp.go:589`（`"session/request_permission 缺 options 数组"`）—— 无格式指令，应为 `errors.New`。同包的 `dshapp/protocol.go:546` 对**完全相同**的分支已经用了 `errors.New`。纯风格，**P2**。

### 6.3 子进程 stderr 关键信息是否在错误分类里丢失

| 包 | stderr 去向 | 死因分类通道 | 判定 |
|---|---|---|---|
| `codexapp` | `stderrdrain` → `[codex-stderr]` | `giveUp` 超时 + `abandonedProbe` TTL | ✅ 完整 |
| `dshapp` | `stderrdrain` → `[dsh-stderr]` | `withMethod` + `IsMissingCredential`（按 code+message 归一） | ✅ 完整 |
| `piapp` | `stderrdrain` → `[pi-stderr]` | `callWithReadyRetry` + `ExitError` | ✅ 完整 |
| `zcodeapp` | `stderrdrain` → `[zcode-stderr]` | `autoAnswer` + `rpcwire.ReapWithLog`（落 exit code） | ✅ 完整 |
| **`hermesapp`** | **全丢** | `rpcError`（含 `data.details`）+ `ErrSessionNotFound` | ⚠️ **P1-3** |

### 6.4 panic 在非初始化路径

全域源文件 `panic(` 命中：**仅 `dshapp/client.go` 1 处**。逐行核对：`client.go` 的 `panic` 在 **`TestMain` 风格的 helper 进程分支**（`dshapp/client_test.go:28` 注释「TestFakeDshHelperProcess 是 go test helper 子进程」），生产路径不可达。

**判定：适配器解析进程输出时不会 panic 带崩会话。已查但干净。**

### 6.5 吞错误清单（全域 `_ =` / `_, _ =` 逐条过）

| 位置 | 吞的东西 | 判定 |
|---|---|---|
| `piapp/mcpsetup.go:379/383/387` | `os.Remove(tmpName)` × 3（tmp 清理失败路径） | ✅ 正确：已在 error 路径上，清理失败无法再上报 |
| `term/manager.go:345` | `cmd.Wait()` | ✅ 注释说明退出码另经 `exitCodeCh` 发布 |
| `term/manager.go:161` / `:397` | `psh` / `Process.Kill()` | ✅ 关闭路径 |
| `mdns.go:254` | `probe.Close()` | ✅ 有注释「探针独占 fd」 |
| `zcodeapp/generate_selected.go:28/76` | `session/close` / `cancelGenerateText` | ✅ 有意 best-effort，且用了独立 5s ctx（不继承已取消的 ctx） |
| `zcodeapp/generate.go:127` | `cancelGenerateText` | ✅ 同上 |
| `zcodeapp/client.go:612-651` | `autoAnswer` 的 6 处 `Respond*` | ⚠️ **P2-9b** |
| `dshapp/protocol.go:338` | `c.Notify("session/cancel", …)` | ✅ 注释自称 fire-and-forget |
| `collabmcp/collabmcp.go:198` | `NotifyProgress` | ✅ 进度通知 best-effort |
| `panel/panel.go:80` | `srv.Serve(lis)` | ⚠️ **P2-9c** |
| `panel/panel.go:282` | `renderIndex(w, cp, rp)` | ⚠️ **P2-9d** |
| `hermesapp/client.go:331` | `sc.Text()` | ⚠️ **P1-3 的同一处** |
| `hermesapp/acp.go:411/413` | `json.Unmarshal(ContentJSON)` | ✅ 失败即走「未知 content」降级 |
| `piapp/client.go:635/642` | `RespondUI` | ⚠️ 与 zcodeapp 同款，P2 |
| `term/ctrl.go:143/145`、`term/manager.go:239/316` | `freeConsole` / `os.Getwd` / resize err | ✅ 均有注释解释降级理由 |

**【P2-9c/d】`panel` 的两处吞**（合并为一条 P2）：`panel.go:80` `_ = srv.Serve(lis)` —— 面板启动成功后若 `Serve` 提前返回（fd 被关、panic 后 recover），**进程余生无人知晓**，调用方 `main.go:507` 只在启动时检查过一次错误。`panel.go:282` `_ = renderIndex(...)` —— 模板/渲染失败时用户看到半截页面且无任何提示。建议：各加一行 `log.Printf("[panel] ...")`。**代价 2 行，风险 0。**

### 6.6 errors.Is/As 未用

`errors.Is/As` 全域仅 12 处命中（`externalconv` 3 / `dshapp` 3 / `zcodeapp` 3 / `piapp` 2 / `collabmcp` 1）。逐条核对：这些是**跨包边界**的错误链判定（`fs.ErrNotExist`、`errClientClosed`、wrapper 链），场景正确。包内 sentinel 用 `==` 比较（如 `err == io.EOF`）也是 Go 惯例。**不列发现。**

### 6.7 defer 里 err 未处理

全域 `defer` 中的 `_ =` 只有 3 处（`term/ctrl.go:143/145` 的 `freeConsole`、`panel` 的 `stopPanel` 闭包内 `_ = srv.Close()`），均为关闭路径，语义正确。**不列发现。**

---

## 7. 并发与子进程生命周期

### 7.1 ✅ 子进程泄漏 —— 已查但干净（跨 `rpcwire` 收口）

5 家的 `cmd.Start()` 处都**看不到 `cmd.Wait()`**（`codexapp:202` / `dshapp:323` / `piapp:392` / `zcodeapp:306`），初看像经典泄漏。深挖后确认**全部委托给 `rpcwire.Wire.ReapWithLog`**（`rpcwire/wire.go:117-155`，另一条线的域）：

- 收尸有**两条触发路径**：`readLoop` EOF（`codexapp:563` / `dshapp:537` / `piapp:527` / `zcodeapp:525`）与 `Close()`（`codexapp:562` 注释「Close 不等收尸，写日志标准 goroutine」）→ **不会漏**。
- `waitOnce sync.Once` 保证 `cmd.Wait` 只执行一次（`rpcwire/wire.go:128`），避免「二次 Wait panic」。
- `hermesapp` 是唯一自带 `waitReap/reap`（`client.go:210/220`）的，因为它不走同一套 wire。
- `jobobject.Assign()`（`main.go:386`）+ `KILL_ON_JOB_CLOSE` 是内核级第二道防线。
- `codexapp/close_reap_test.go` / `piapp` / `zcodeapp` 各有专门的收尸回归测试。

**结论：无子进程泄漏。已查但干净。**

### 7.2 【P1-4】`zcodeapp` 的 `modelPrefFile` 锁纪律漏洞

`internal/zcodeapp/generate.go:384-396`
```go
var modelPrefFile = defaultModelPrefPath()          // ← 包级 var,init 时求值
var configuredDataDir atomic.Pointer[string]

func SetDataDir(dir string) {
	dir = strings.TrimSpace(dir)
	configuredDataDir.Store(&dir)
	modelPrefFile = defaultModelPrefPath()           // ← 写包级 var,**不持 prefMu**
}
```
vs. 所有读写方（`generate.go:412/419/440`）：
```go
var prefMu sync.Mutex        // 注释:"守卫显式加锁比依赖调用方纪律便宜"
func loadModelPref() { prefMu.Lock(); defer prefMu.Unlock(); ... 用 modelPrefFile ... }
func saveModelPref(...) { prefMu.Lock(); defer prefMu.Unlock(); ... 用 modelPrefFile ... }
```

**问题**：`SetDataDir` 是**唯一不持 `prefMu` 就写 `modelPrefFile` 的路径**。锁存在但不覆盖全部写入方 —— 这比没有锁更危险，因为它给人「已串行化」的错觉。`modelPrefFile` 注释自己写着「包级变量是为了让单测重定向到临时目录」。

**为什么是 P1 而不是 P2**：当前 `main.go:148` 在 `flag.Parse` 后、任何请求前调用 `SetDataDir`，**单线程下安全**。但这是靠调用顺序而非靠锁 —— 一旦 `SetDataDir` 被移到热路径（或未来加 `-data` 热重载），`loadModelPref`/`saveModelPref` 并发读 + `SetDataDir` 写 = **data race + 写到错误路径**（把用户偏好写到 `~/.pocketagentd` 而不是 `-data` 指定目录）。

**为何 race detector 没抓到**：`go test -race` 下没有并发 `SetDataDir` 调用（实测 20 包 race 全绿）。**这是「测试覆盖不到的真实竞态候选」，正是本轮要找的东西。**

**修复代价 / 风险**：改 `SetDataDir` 内 `prefMu.Lock()`；或更好——把 `modelPrefFile` 改成 `atomic.Pointer[string]` 并删掉 `prefMu` 的写侧负担。**代价 3 行，风险 0**（不碰禁区）。另建议：`defaultModelPrefPath()` 在 `SetDataDir` 里被重复求值——与 init 时的求值路径重复，可合并。

### 7.3 goroutine 无退出路径 / Ticker 不 Stop / ctx 未 cancel

| 检查项 | 结果 |
|---|---|
| `time.NewTicker` | **全域 0 命中**（本域无 Ticker）→ 无 Stop 缺失 |
| `time.After` | `codexapp:384`、`zcodeapp:411`、`term/ctrl.go:199` 等。**go.mod 是 `go 1.27.1`**，Go 1.23+ 起未引用的 `time.After` timer 会被 GC —— **无泄漏**。已查但干净 |
| `context.WithTimeout` 的 `defer cancel()` | `piapp/mcpsetup.go:208-209`、`dshapp/protocol.go:40`、`zcodeapp/generate_selected.go:14/26/74` —— 全部成对 `defer cancel()`。已查但干净 |
| 请求路径硬写 `context.Background()` | 21 处命中，**全部是 `ctx == nil` 的防御性兜底**（`codexapp:344`、`dshapp:410`、`piapp:447`、`zcodeapp:411`、`hermesapp:531/630`…）或启动期探测（`piapp/mcpsetup.go:208`、`zcodeapp/generate.go:94`）+ **有意为之的清理 ctx**（`generate_selected.go:26/74` 用 `context.Background()` 而非继承已取消的 ctx，**正确**）。已查但干净 |
| goroutine 退出路径 | `rpcwire.ReapWithLog` / `readLoop` / `drainStderr` 均以 EOF 或 ctx.Done 为终止；`zcodeapp:autoAnswer` 的 3 个 goroutine 是 fire-and-forget（**P2-9b**）；`term/ctrl.go:192` 的写 goroutine 经容量 1 channel 收尸（注释已论证） |

### 7.4 【P1-5 / 与 §2.3 同批】`hermesapp/config.go` 无锁 + 固定 tmp 名

`internal/hermesapp/config.go:37-78` 整段是 **读全文件 → 改行 → 写 `path+".tmp"` → `os.Rename`**：

```go
raw, _ := os.ReadFile(path)     // ← 整个函数无任何 mutex
lines := strings.Split(string(raw), "\n")
...  // 逐行改写
tmp := path + ".tmp"           // ← 固定名,非 os.CreateTemp
if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")), 0o644); err != nil { return err }
return os.Rename(tmp, path)
```

**两个问题**：
1. **无锁的 read-modify-write**：两个并发的强度选择（用户在两台设备上同时改）会互相覆盖，`.tmp` 名相同 → 后写者覆盖前写者的 tmp，再 rename 两次 → **丢一次修改**（不是文件损坏，因为 rename 是原子的，但内容会丢）。
2. **固定 tmp 名 + 与同域最佳实践不一致**：`piapp/mcpsetup.go:372` 用的是 `os.CreateTemp(agentDir, mcpConfigFile+"-*.tmp")` + 3 处 `_ = os.Remove(tmpName)` 清理。同域两种口径。
3. 失败时 `.tmp` 残片不清理（`piapp` 那三处清理就是为此）。

**分级 P1**：与 P0-1 同源（同一函数）。若按 §1.3 的建议把落点移到 daemon 数据目录 + 改用结构化持久化（不再手改 YAML），锁与 tmp 两个问题**一并消失**。若选择保留原地改写方案，则**必须**加锁 + 换 `CreateTemp`。
**风险**：不碰禁区/红线。

### 7.5 竞态候选（共享 map/slice 无锁）—— 逐处核过

| 位置 | 状态 |
|---|---|
| `codexapp/client.go:346-351/395-399/417-432/437-445` | `pending` / `abandoned` / `nextID` 全在 `c.mu` 下；`hs` 走 `atomic.Pointer` + `handlerMu` 双写单读 —— ✅ |
| `piapp/client.go:284-311` `syncHandlers/publishHandlersLocked` | 与 codexapp 同款，注释写明「dispatch 只读不可变快照且在锁外调用回调，回调可安全发请求或卸载自身」—— ✅ 设计正确 |
| `piapp/sessState`（`client.go:243`） | 需确认 `updateSession`（`client.go:267`）持锁 —— **未逐行核，标"疑似待核"** |
| `zcodeapp/client.go:368` `markExited` / `:358 Alive` | `atomic.Store/Load`（`c.exited.Load()`）—— ✅ |
| `collabmcp/delegate.go:110/124` | `defer release()` × 2，配 `agentLimiter` per-agent 信号量（包注释准则 7）—— ✅ |
| **`zcodeapp/generate.go:395`** | ❌ **P1-4**，见上 |
| `testagent/fake_agent.go` | 全 `a.mu` 保护，`recordLedger` 5 参数全是 string（非新发现）—— ✅ |
| `jobobject.Assign` 的 Job 句柄 | `main.go:386` `if _, err := jobobject.Assign();` —— 句柄被丢弃。**功能上成立**（Go 不自动关 Windows HANDLE，句柄随进程退出关闭），**但依赖的是「没人 close」这个隐式事实** → **P2-11b** |

---

## 8. daemon 全域测试面横切（本轮增值重点）

### 8.1 `testagent/`（31 KB）与 `testkit/`（6 KB）各覆盖什么

| 包 | 内容 | 导入方 |
|---|---|---|
| **`testagent`** | 完整的 `agent.Provider` **假实现**（`fake_agent.go` 810 行）+ 一套场景 DSL：`Scenario` / `Text` / `Thinking` / `ToolCall` / `ToolResult` / `Usage` / `CodexEventFrame` / `Unknown{Notification,Request,Response}` / `Pause` / `Late` / `Completed` / `Failed`，外加观测面（`Frames()` / `RunFrames(i)` / `Approvals()` / `LateDropped()` / `WaitIdle()`）。**关键设计：它是非测试包**（`fake_agent.go` 不带 `_test.go`），所以 `server` / 之外的包能 import。11 个 `AgentEvent` 实现 + 9 个场景测试 | **仅 3 个测试文件**：`server/collab_e2e_test.go:35`、`server/collab_recovery_e2e_test.go:31`、`server/unknown_event_e2e_test.go:50` |
| **`testkit`** | 4 个跨平台测试原语：`AssertNoDeveloperPaths(t)`（**开发者个人路径泄漏断言**）、`CaptureLog()`（日志捕获）、`WaitFile(t, path, d)`（轮询等文件）、`WaitProcessGone(pid, d)`（Windows/Unix 双实现） | **6 个测试文件 / 5 个包**：`codexapp/client_test.go`、`codexapp/close_reap_test.go`、`codexapp/start_args_test.go`、`codexapp/devpath_assert_test.go`、`term/manager_idle_test.go`、`eventstore/eventstore_degrade_test.go` |

`testagent` 是唯一能伪造**协议级未知事件**（未知通知/未知请求/未知应答）与**迟到应答**的装配 —— 这三样在 `AGENTS.md` 的「能透传就透传」原则下是最容易回归的面，所以它值得独立成包。✅ **设计合理。**

### 8.2 全仓第三、第四套 fake 装配的分布

统计方法：扫 daemon 全域 232 个 `_test.go`（排除 `gen/`）。

| 假装配形态 | 命中 | 分布 | 该不该进 testkit/testagent |
|---|---|---|---|
| **`agent.Provider` 的 Run 实现（fake agent）** | **8 个文件** | **全在 `internal/server/`**：`codex_subagent_history_test.go` / `codex_thread_delete_sync_test.go` / `control_plane_test.go` / `prefs_broadcast_test.go` / `thread_delete_dsh_test.go` / `thread_history_zcode_test.go` / `thread_provider_test.go` / `thread_section_pi_test.go` | ⚠️ **可收**：`server` 恰好是 `testagent` 的导入方（同包，不会循环依赖）。8 个文件各造一个 Provider，只为测 ListThreads/GetThreadHistory 这类「provider 被调用后返回固定历史」。**建议**：给 `testagent` 加一个 `StaticProvider{ID, Threads, History}` 最小件，8 处收敛。代价 ≈60 行新件 + 8 处替换；风险低。**P2**（重复小、收益中等） |
| **fake/Stub/Mock 类型声明** | **74 处，6 个包** | `agent/` 18 文件（`hermes_provider_test.go` ×4、`codex_read_test.go` ×3、`pi_selection_test.go` ×3…）、`server/` 若干 | ❌ **不该收**。`agent/` 与 `runtime/` 不能 import `testagent`（会成环 —— `testagent` import `agent` + `runtime`）。且这 74 处多数是「测某个 provider 的私有解析逻辑」的**局部桩**（如 `fakeZcodeSnapshot`），不是通用 Provider。**属 `agent/` 域，建议转交该线评估** |
| **fake 子进程（helper process）** | **9 处，4 个包** | `piapp/entries_test.go`(2) / `piapp/client_test.go` / `zcodeapp/client_test.go` / `codexapp/close_reap_test.go` / `agent/`×5 | ❌ **不该收**。Go 的 `TestXxxHelperProcess` + `os.Args[0] -test.run=` 是**语言机制**，每处必须有自己的 `TestMain`/环境变量分派，收进公共库反而绕。**但**：`piapp/entries_test.go` 与 `piapp/client_test.go` 造了**两个** helper 进程（`entries_test.go:168` 甚至要 `t.Skip("因为被 TestGetEntriesAgainstFakePi 抢先用了假 pi 子进程")` —— **两个测试抢同一个全局 helper 进程态**）。这是真实的脆弱点 → **P2-3** |
| **fake gRPC server（bufconn / grpc.NewServer）** | **4 处，2 个包** | `server/collab_delegate_rpc_test.go`(2) / `tlscert/tlscert_test.go`(2) | ❌ 不该收（各测各的协议面，无共享形状） |
| **fake exec.Command 接缝** | **0** | — | ✅ 域内**没有** exec seam 注入（都用真 `exec.Command` + helper 进程）—— 一致，无债 |
| **fake stdin/stdout pipe（io.Pipe / os.Pipe / net.Pipe）** | 19 处，6 包 | — | ❌ 不该收（局部） |

**小结（回答「第三、第四套 fake 装配在哪」）**：
- **第 1 套** = `testagent`（3 个 e2e 文件用）。
- **第 2 套** = `internal/server/` 里 **8 个各自造的 Provider 实现**（每个 20-60 行）。← **这是最值得收的「第三套」**，因为收件方与造件方同包，零依赖风险。
- **第 3/4 套** = `agent/` 的 74 处局部桩（域外，且有 import 环硬约束，**明确不收**）+ 9 处 helper process（语言机制，**明确不收**）。

### 8.3 【P1-6】条件性跳过

全域 **45 处 `t.Skip` / `45 处 `t.Skipf`，11 个包**。逐条分类：

| 类别 | 处数 | 判定 |
|---|---|---|
| **显式 opt-in 的真机测试**（`PA_LIVE_CODEX=1` / `PA_LIVE_ZCODE=1` / `PA_LIVE_HERMES=1` / `PA_LIVE_ADAPTER_VERIFY=1` / `PA_LIVE_PI_E2E=1`） | 14 | ✅ **正确设计**。需要真实登录的 agent 无法在 CI 跑；且都写了「怎么打开」的说明 |
| **平台门**（Windows-only / Unix-only / junction / ConPTY） | ~14 | ✅ daemon 全链 Windows-only，这些门是防御性的 |
| **环境能力门**（无 node / 无 git CLI / 端口 5353 被占 / root 目录只读 / 凭据存储不可用） | ~12 | ⚠️ **部分掩盖**：在**一台**跑测试的机器上，「无 git CLI」「无 node」会**恒定跳过**且不留痕 |
| **⚠️ 测试自身依赖失败即静默跳过** | **3** | ❌ **掩盖**（见下） |
| **⚠️ mDNS 广播在 CI 不可用** | 4 | ❌ **掩盖**（见下） |

**掩盖点 1（3 处）——「拉不起测试用子进程就跳过」**
`codexapp/close_reap_test.go:77`、`codexapp/client_test.go:340`、`dshapp/client_test.go:127` 都是
```go
t.Skipf("无法拉起测试用子进程: %v", err)
```
**这三个测试的全部价值就是「假子进程跑起来并以 exit 7 退出 → 断言收尸日志写了退出码」**。子进程拉不起来（AV 拦、路径含中文、句柄耗尽）时它们静默跳过 —— 而这正是最需要这个测试的环境。**建议**：区分「环境不支持」与「代码坏了」，前者可跳、后者必须 fail。**P1**（与 §8.5 的「无 CI」叠加：没人会看到它们在跳）。

**掩盖点 2（4 处）—— mdns 测试在 CI 全跳**
`internal/mdns/mdns_test.go` 的包注释自己写着：
> 「本机实测… **在 CI 广播不可用时,每次跑都 `t.Skip`(跑不掉就不该当 bug);只需要真机实跑,广播环境跑不通全跳过(单测/证书/TXT/去重)」

加上 `:33`「本机不支持 mDNS 广播(在 CI),跳过」、`:46`「0.0.0.0:5353 无法绑定(容器网络或被占),同组测试失败,跳过」、`:53`「short 模式无广播容器,跳过」。

**mdns 是本域唯一的「主动向局域网暴露服务」的面**（广播 `_pocketagent._tcp` + `host_id` + `name` + `tls`），而它的测试**在自动化环境里一次都跑不到**。加上 §5.3 的注释谎言与暴露面，mdns 是本域测试面最薄的一环。**P1**。

### 8.4 【P1-7】平台相关测试在 CI 真跑得到吗 —— **没有 CI**

**实测**：
- `Get-ChildItem -Force` 根目录 → 无 `.github/`
- 全仓 `*.yml`/`*.yaml` → **只有 2 个**：`flutter/analysis_options.yaml` 与 `flutter/pubspec.yaml`（都是 Flutter 工具链文件，**不是 CI**）
- 全仓无 `Jenkinsfile` / `.gitlab-ci.yml` / `azure-pipelines` / `.circleci` / `.buildkite`
- `.git/hooks/` 下**无任何非 `.sample` 钩子**（连 pre-commit 都没有）

**后果（这是本节最重要的一条）**：
1. AGENTS.md §5 的「测试水位不许倒退：合入前 `go test ./internal/...` 全绿 + Android `test` 分支 0 失败；数字变化要同步更新 DEVELOPING.md §0.3」—— **完全靠人工纪律**，无机器门禁。
2. §8.3 的 45 处条件跳过，只在一台开发机上求值。「本机跑绿」与「全绿」在本仓**是同义词**。
3. DEVELOPING.md 记载的「daemon 全量 30 个有测试包通过 / 12 包 race 通过」是**手工执行 + 人工转录**的数字，无 artifact 可复核（对照 AGENTS.md 红线 5「不留假绿：测试 `--rerun` 并核验结果 XML 时间戳」—— daemon 侧连 XML 都没有）。
4. **对本次审查的直接限制**：我无法验证「12 包 race 通过」是当前真实水位，只能自己重跑（本报告基线表里的 `-race` 结果是我实跑的，20 包全绿）。

**分级**：**P1**。这不是「代码脏」，是「整洁度没有机器保证」——而 AUDIT.md 的整套门禁（`tools/audit/pa_audit.py`、`pa_upgrade_gate.py`）也都是**手工调用**。

**修复代价 / 风险**：
- 最小可行：加一条 GitHub Actions workflow，`windows-latest` 上跑 `go build ./... && go vet ./... && go test ./... && go test -race ./internal/...`（约 20 行 YAML）。**代价极低、风险 0**（只读 job，不碰仓库代码）。
- 加 `-short` 约定：让 45 处 opt-in 测试统一在 `-short` 下跳过，让 CI 有明确的「这一轮跑了什么/没跑什么」清单（现在跳过理由散落在各文件里，无法汇总）。

**不确定点**：是否**故意**不设 CI（例如因为开发者只在 Windows 本机跑、且依赖真实 agent 登录）。我未查到任何说明。若是刻意的，应写成显式决策（如 `DEVELOPING.md` 记「daemon 无 CI，测试门禁为人工」），否则读者会误以为有 CI。

### 8.5 断言质量、magic number、超长测试函数

| 指标 | 实测 | 判定 |
|---|---|---|
| **无消息断言比例** | **0 / 7420 = 0%**（无一处裸 `t.Fatal()` / `t.Error()` / `t.Log()` / `t.Fail()`） | ✅ **异常好**。多数 Go 仓库这一项在 10-40%。`testify/require` 也零依赖 |
| `t.Fatalf` 带 `%v` | 5537 处 | ✅ |
| `t.Errorf` 带消息 | 471 处 | ✅ |
| `t.TempDir()` | 506 处，20 包 | ✅ 无手写临时目录 + 无清理遗漏 |
| `t.Parallel()` | **全域 0 处** | ⚠️ **P2-4**：1663 个测试函数全部串行，`go test` 墙钟时间 = 各包之和。加 `t.Parallel()` 是低风险提速（但需先确认无全局状态耦合 —— `modelPrefFile` 那类包级 var 就是障碍，见 P1-4） |
| **超长测试函数** | 1663 个中 **25 个 >100 行、11 个 >150、4 个 >200** | ✅ **1.5%**，且 top 4 全是表驱动：`piapp/client_test.go:47 TestRenderGuardExtension`(471) / `piapp/mcpsetup_test.go:119 TestAdapterInstalledDetection`(411) / `piapp/entries_test.go:25 TestParseEntriesPage`(369) / `store/collab_store_test.go:183`(384)。**长是因为表长，不是逻辑糊** |
| magic number | 未发现裸数字断言（如 `!= 1000`）| ✅ |
| 无信息量失败消息 | 抽 30 处失败消息检查，**全部含实际值 + 期望值**（如 `t.Fatalf("应为 agent 段含 reasoning_effort=high,得到:\n%s", got)`）| ✅ |

### 8.6 测试与源文件倒挂

| 包 | 源 | 测 | 比 | 判定 |
|---|---|---|---|---|
| `internal/hidewin` | 842 B | 2253 B | **2.68x** | ⚠️ 唯一倒挂超 2x。源是 24 行的 `Apply(cmd)`（3 行有效逻辑），测 2.2 KB 在验「幂等 + 不覆盖已有 SysProcAttr + HideWindow/CreationFlags 都置上」——**测试比实现详细是对的**（缺陷即测试）。**不是债**，但说明 `hidewin` 这个包**不值得单独成包**（见 §9.1） |
| `hostenv/goscan`（域外） | 3049 B | 5722 B | 1.88x | 域外 |
| `codexapp` | 29628 B | 39538 B | 1.33x | ✅ 合理（1 源文件对 9 测试，源高度集中） |
| `server` | 765851 B | 998849 B | 1.30x | 域外 |
| `piapp` | 74545 B | 93336 B | 1.25x | ✅ |
| `agent` | 957874 B | 1019542 B | 1.06x | 域外 |
| `piapp`/`zcodeapp`/`hermesapp`/`dshapp` | — | — | 0.78-1.25x | ✅ **六家适配器的测试量都在源的 0.8-1.3 倍，没有一家测试替实现兜底** |
| `internal/term` | 26964 B | 11941 B | **0.44x** | ⚠️ 最低的非零。`term` 含 ConPTY/键位模拟/win32 syscall，测试难写。`manager_spawn_test.go:9` 注释已诚实说明「…否则直接 hang,go test 超时收不到信号」→ **已知且有记录**，不算隐瞒 |
| `cmd/pocketagentd` | 41952 B | 3987 B | **0.10x** | ❌ 见 §4.2 / P2-9 |
| `cmd/collabrolecheck` | 9372 B | 673 B | 0.07x | ❌ 唯一有测试的探针，测试也只有 15 行 |
| **`cmd/` 合计** | **666696 B** | **0 B** | **0.00x** | ❌ 见下 |

**「测试替实现兜底」的判定：全域范围内 —— 未发现一例。** 六家适配器 + 8 个小包的测试/源比全部落在 0.44-1.33 的健康带内。R1 轮担心的「某包测试比源多 3 倍」在本域**不存在**（最高是 `hidewin` 2.68x，但那是 24 行实现配 2.2 KB 断言，正常）。

### 8.7 【P2-2】`cmd/` —— 57 个子包、651 KB、**零测试**

| 指标 | 值 |
|---|---|
| 子包数 | **57** |
| `.go` 文件 | 79 |
| 源码总量 | **666,696 B（651.1 KB）= 整个适配器层的 1.6 倍** |
| 有测试文件的子包 | **0**（`cmd/collabrolecheck` / `cmd/paguard` / `cmd/pocketagentd` 3 个有测试文件，但 57 个子包里 `go test` 报 "no test files" 的是 **54 个**） |
| 生产入口占比 | `cmd/pocketagentd` = 41,952 B，占 `cmd/` 的 **6.3%** |
| 最大家族 | `cmd/dshprobe`(91 KB/5) + `cmd/zcprobe`(83 KB/6) + `cmd/piprobe`(91 KB/7) = **265 KB** |
| 包注释缺失 | **30 / 57** 个 `main.go` 首行即裸 `package main`（含**唯一的生产入口** `pocketagentd`） |
| 过期标记 | **0 处** —— 无一个探针标注「已废弃/已完成/可删」 |

**判定（不是「删掉」的建议，是「无界累积」）**：
AGENTS.md §2 已经诚实披露了这一点：「`ls daemon/cmd/` —— chatprobe/modelprobe/smoketest 等**一次性探针**」。所以**不是隐瞒**。

真正的问题是**没有上界**：
1. 这些探针是**整个协议层的地面真值来源**（`docs/verify/evidence/dsh-p0-probe.md` 等），有长期考古价值 —— 所以不能简单删。
2. 但 `cmd/dshprobe/main.go` 1521 行 / `cmd/zcprobe/main.go` 996 行 **自己重跑了一遍生产协议解析**（`zcprobe/model.go`、`zcprobe/ctrl.go`…），与 `zcodeapp/` 的解析器**口径可能已经漂移**，而**零测试**意味着漂移不会被任何门禁发现。探针文档里写的 ground truth 有可能在测一个已经不存在的实现。
3. 没有归档目录（`docs/verify/` 有 evidence，但代码没有对应归档），没有过期标记 → `cmd/` 只会单调增长。

**建议（低成本、不删任何东西）**：加 `daemon/cmd/README.md`，列出三类——(1) 生产入口（pocketagentd、paguard、panel 相关）；(2) 仍在使用的活探针；(3) 已完成、结论已入 `docs/verify/evidence/` 的历史探针（可保留但标注证据文件路径 + 结论日期）。给 (3) 类的 `main.go` 加一行 `// 归档:证据见 docs/verify/evidence/<file>;<日期>结论:…`。
**代价**：1 个 README + N 行注释。**风险 0**。

---

## 9. 路径与安全面

### 9.1 小包职责边界

| 包 | 职责 | 边界判定 |
|---|---|---|
| `pathguard` | 七项不可逆位置清单 + 全锚点物理解析 | ✅ **无渗漏**。纯函数，`ResolvePhysical` 是唯一导出辅助。依赖面只有 `hostenv` + `x/sys/windows`。**不 import 任何适配器/传输/日志包** |
| `hidewin` | 给 `*exec.Cmd` 打 `CREATE_NO_WINDOW` | ✅ 无渗漏。24 行单一职责，无状态 |
| `stderrdrain` | 子进程 stderr 逐行脱敏 + 限流 | ✅ 无渗漏。依赖只有 `bufio/io/regexp/strconv` |
| `jobobject` | KILL_ON_JOB_CLOSE + UI 限制 | ⚠️ 见 §9.2 |
| `tlscert` | 自签证书生成/复用/损坏隔离 | ✅ 无渗漏。**不监听、不握手**（只出 `tls.Certificate`，TLS 配置在 `server/` 组装） |
| `mdns` | `_pocketagent._tcp` 注册/注销 | ✅ 无渗漏。**暴露面问题见 §5.3，不算职责渗漏** |
| `rotatelog` | 域外（另一条线） | — |
| `term` | PTY 会话管理 + Win32 键位 + 控制台广播 | ⚠️ `term` 实际做**三件事**（`manager.go` 会话池 / `ctrl.go` Win32 键位与控制台 ignore / 可能还有别的），是本域最大的「一包两事」候选 |

### 9.2 【P2-11b】`jobobject` 的两处脆弱

**(1) 签名随平台变**：`jobobject_windows.go:32` `func Assign() (windows.Handle, error)` vs `jobobject_other.go:9` `func Assign() (uintptr, error)`。两个文件靠 build tag 互斥，所以**各自平台都能编译**，但调用方无法写跨平台代码（写 `windows.Handle` 在 Unix 编译不过）。当前唯一调用方 `main.go:386` 用 `_, err :=` 丢弃返回值，**恰好绕开了这个不一致**。一旦将来有人要 `defer windows.CloseHandle(job)`，Unix 构建就炸。

**(2) 句柄被丢弃**：`main.go:386` `if _, err := jobobject.Assign(); err != nil {` —— Job 句柄直接丢在地上。**功能上正确**（Go 不自动关闭 Windows HANDLE，内核句柄随进程退出释放，KILL_ON_JOB_CLOSE 因此在整个进程生命期内生效），但**依赖的是「没人调 CloseHandle」这个隐式事实**，而这正是最容易被后人「顺手优化」掉的写法（比如加个 `defer job.Close()` 就把 Job 语义反过来了 —— 关句柄 = 杀全家）。

**建议**：签名统一为 `func Assign() (Handle, error)`，在包内定义 `type Handle = uintptr`（或 `windows.Handle` 的别名），非 Windows 返回 0；`main.go` 加一行注释说明「句柄必须持有到进程结束 —— 关闭它等于杀掉所有 agent 子进程」。**代价 5 行，风险 0。**

### 9.3 路径拼接穿越风险

逐包核了所有把外部输入拼进文件路径的位置：

| 位置 | 输入来源 | 穿越风险 |
|---|---|---|
| `piapp/mcpsetup.go:372` `os.CreateTemp(agentDir, mcpConfigFile+"-*.tmp")` | `agentDir` 来自 `ResolveCollabAgentDir()`（env 或 daemon dataDir） | ✅ `CreateTemp` 自身防穿越 |
| `piapp/mcpsetup.go:282` `filepath.Join(agentDir, name)` | `name` 来自 `CopyUserCredentials` 的 `sources` map key，**先过 `CredentialWhitelist` 白名单**（`:273`），白名单外直接 `errors` 拒绝并继续 | ✅ **有白名单，防任意文件写**（`mcpsetup_test.go:356` 有 `evil.txt` 的负向断言） |
| `hermesapp/config.go:73` `tmp := path + ".tmp"` | `path` 来自 `ConfigPath()` | 见 §7.4（不是穿越，是并发/固定名） |
| `externalconv` `filepath.Join(root, …)` | `ClaudeRoot(home)` 由 `home` 参数派生 | ✅ **全包只读**（`os.ReadFile`/`ReadDir`/`WalkDir`），且不导出任何写函数 |
| `dshapp/collabpatch.go:43` `filepath.Join(dshHome, "profiles","acp","cordis.patch.yml")` | `dshHome` 来自 `defaultDshHome()`（daemon 数据根） | ✅ 常量后缀，用户输入进不来 |
| `zcodeapp/accounts.go:92` `filepath.Join(desktopHome, ".zcode", "v2")` | `ZCODE_DESKTOP_HOME_DIR` env 或 home | ✅ 只读 |
| `codexapp` | 无路径写入 | ✅ |
| `pathguard` | 全是 `filepath.Clean` → `Abs` → 物理解析 → 锚点比对 | ✅ 这是**防穿越的本体**：`GuardDeny:194` `filepath.Abs(filepath.Clean(path))` 先归一，`isBareDriveRoot:198` 在归一前拦裸盘符，`extraAnchors:305` 加设备前缀与 `\\host\X$` 别名锚点，`denyAnchor:270` 数据目录用 `filepath.Rel` 判子树 |

**判定：本域无路径穿越风险。已查但干净。**（`pathguard` 的锚点纪律是本仓最扎实的一段代码，8 处注释解释了每个设计选择，测试 `pathguard_test.go:41-169` + `server/security_regression_test.go` 双侧钉住。）

### 9.4 mdns 意外暴露面

见 §5.3。要点重述：明文/无鉴权模式下仍向 LAN 广播 `host_id` + 机器名 + `tls=0`，但 F2 已把监听压回回环，**端口在 LAN 不可达**。真正的暴露是「主机标识 + 机器名」这两项元数据。**不确定点**：`host_id` 的安全语义（能否用于冒充设备）取决于 `server/` 的设备注册链 —— **另一条线**，我未核实，因此不定 P0。

---

## 10. 伪命题澄清

以下都「看起来像问题」，逐条证伪或明确不收：

### 10.1 「5 家适配器的 `Client` 结构是同型重复，应该抽公共基类」→ **不收**

§2.4 已用逐句对比证伪：`readLoop`/`dispatch` 的差异是**协议异质性**（ID 类型 int64/string/RawMessage、分帧 `\n`/Content-Length、迟到应答三套策略、反向应答形状三种），抽出来会得到带 6 个类型参数 + 2 个策略接口的抽象体。**传输小件层已被 `rpcwire` 收干净，剩下的是本质差异。**

### 10.2 「`AllowOptionID`/`RejectOptionID` 在 dshapp 和 hermesapp 重复」→ **不收**

§2.5。字面确实逐字相同（我的哈希工具先命中了），但两个 ACP 实现是**两套独立的用户安装**，共享实现会把两个供应商的协议演进绑死。**工具命中 ≠ 语义等价。**

### 10.3 「`piapp.DefaultCollabAgentDir` 与 `zcodeapp.defaultModelPrefPath` 重复」→ **部分收**

归一化 body 命中（形状相同）。逐行读：路径后缀不同、**失败兜底不同**（`""` vs `""`，但和 dshapp 的 TempDir 又不同）。收的理由是「都该走 `hostenv.UserDataDir()`」而不是「两个函数相同」→ 见 §2.3 **P1-7**。

### 10.4 「`piapp.GuardRules` 是写死的名单，违反红线 4」→ **不违反**

AGENTS.md 红线 4「不新增写死的名单/顺序/版本二分」的语境是**协议/能力位映射**（「未知输入走透传」）。`GuardRules` 是**危险 shell 命令的拒绝式护栏**（`rm -rf` / `git push --force` / 写系统目录 / 提权），方向与红线 4 相反（默认拒绝 vs 默认透传），且 `guard.go:6-8` 明确说明它是**唯一可阻塞点**（pi 无内建审批，事件流里没有执行前阻塞点）。**安全护栏的白名单/黑名单是正当的硬编码。**

（顺带：不硬编码不是万能 —— 正则护栏天然可被混淆绕过（`r''m -rf`、`rm --recursive --force` 的多空格形态等）。这属于设计权衡的讨论，不是本次整洁审查的发现。）

### 10.5 「`source` 字族被 protoc 生成污染」→ **误报**

`codexapp/client.go:275-285` 的 `wireMsg` 是**手写**的，且注释详述了「为何不用生成的 `CodexMessage`」（JSON-RPC 2.0 的 id 允许 number|string/null，生成类型用 `*int64` 接不住）。**这不是生成代码污染，是刻意手写并写清了理由。**

### 10.6 「`cmd/` 里 57 个探针是垃圾，应该删」→ **否**

§8.7。这些是协议层的地面真值来源，有考古价值（`docs/verify/evidence/` 大量引用）。建议是**加归档标注 + README**，不是删。删了会破坏「agent 升级时重跑探针对账」的工作流（AGENTS.md §2 明写这条）。

### 10.7 「`time.After` 在 select 里会泄漏 timer」→ **不成立**

`go.mod: go 1.27.1`。Go 1.23 起未引用的 `time.After` timer 会被 GC 回收。`codexapp:384`（120s）、`zcodeapp:411`、`term/ctrl.go:199`（5s）均无泄漏。

### 10.8 「`cmd/dshprobe` 有 `panic(` 在非初始化路径」→ **误报**

§6.4。唯一命中在 `dshapp/client.go` 的 helper 进程分支（配合 `dshapp/client_test.go:28` 的 `TestFakeDshHelperProcess`），生产不可达。

---

## 11. 已查但干净（清单，供后续审查免重复）

| 项 | 证据 |
|---|---|
| **子进程泄漏** | 5 家全部经 `rpcwire.Wire.ReapWithLog` 收尸，`waitOnce` 保证一次；`readLoop` EOF + `Close()` 双触发；`jobobject` 内核兜底。5 家各有收尸回归测试（`close_reap_test.go` 等）。**race 全绿** |
| **死代码** | 双向计数（全域符号出现次数 ≤ 1 = 只有定义处）→ **0 处**未引用的私有函数/类型/常量 |
| **`%w` 纪律** | 139 处 `fmt.Errorf`，45 处无 `%w` **全部无 error 值可包**，逐条核对无误 |
| **错误字符串约定** | 全部小写开头（中文消息无大小写约束），无句号结尾，无 `Err: error:` 前缀。合规 |
| **panic** | 适配器解析进程输出时**零 panic**；全域 1 处命中在测试 helper 分支 |
| **命名噪声** | `Util/Processor/Wrapper/Facade` = 0；`Handler` 77 处全是 JSON-RPC 术语；`Builder` 全是 protobuf 生成 |
| **超大接口** | 全域仅 1 个 ≥3 方法接口（`panel.Controller`，5 方法） |
| **参数爆炸** | 132 源文件中仅 15 个 ≥5 参函数，源文件里 8 个，参数全部内聚 |
| **断言消息** | **7420 处断言 100% 带消息，0 处裸 `t.Fatal()`/`t.Error()`** |
| **TODO/FIXME** | 本域 **0 条** |
| **注释掉的代码** | 本域 **0 处** |
| **子进程 stderr 脱敏** | 4/5 家走 `stderrdrain`（脱敏 + 256KB 限流 + 1MB 单行上限），凭据不落盘 |
| **`t.TempDir()`** | 506 处，20 包，无手写临时目录、无清理遗漏 |
| **路径穿越** | 8 处路径拼接全部有白名单（`CredentialWhitelist`）或常量后缀；`externalconv` 全包只读 |
| **`pathguard` 职责** | 纯函数、无 import 渗漏、双锚点 fail-closed、8 处设计注释 + 双侧测试钉 |
| **`hidewin` / `stderrdrain` / `tlscert` / `mdns`** | 单一职责，无渗漏 |
| **仓库内二进制/日志** | `git ls-files daemon/` 中 `.exe/.log/.bak-` **0 条**，`.gitignore` 覆盖 `*.exe`/`*.log`/`NUL`/`daemon/*.exe.bak-*` |
| **审批代办** | 6 家逐包核过，**零自动放行**；`piapp/guard.go` 强制 `ctx.ui.confirm`；`dshapp` 兜底方向是「拒绝」 |

---

## 12. 已知工具局限

1. **函数长度用花括号配对统计**（先掩码字符串/字符/注释）。两类已知失真：
   - `func x() expr` 无花括号体（本域 0 例命中，脚本按 +6 行封顶）
   - 裸 `}`（字符串/复合字面量里的右括号）—— 已通过掩码规避
   **抽样人工核对 2 例**：`cmd/pocketagentd/main.go main()` 报 66→552（487 行），实测第 66 行确为 `func main() {`、第 552 行确为其 `}`；`piapp/client_test.go TestRenderGuardExtension` 报 47→517（471 行），实测吻合。**未出现 android 域那轮的 1108 行虚报**。
2. **嵌套深度按花括号计数**，会把 `switch`/`select`/`case` 也算作一层，**比人读的缩进深 1-2**。故「嵌套 >3 层」在本报告只报 ≥5 的文件（43 个），且已逐个归因（生产 6 个、探针 37 个）。
3. **规范化 body 哈希把字符串字面量压成 `"S"`、数字压成 `"N"`** → 制造了 §2.5 那一组**假阳性**（`allow`/`reject`/`selected` 全变 `"S"`）。**这正是「先证明语义等价再列」的价值**：哈希命中只是线索，最终判定必须逐行读。
4. **`t.Parallel()` = 0 / `testify` = 0** 是全域精确值（不是抽样）。
5. **`cmd/` 有测试文件的子包数**：脚本按「目录下有 `_test.go`」统计得 **0**（`collabrolecheck`/`paguard`/`pocketagentd` 3 个有测试文件，但 57 个子包里 `go test` 报 "no test files" 的是 **54 个**）—— 两个数字口径不同，已在 §8.7 分列。
6. **未越界核实**：`host_id` 的安全语义（`server/`）、`SetReasoningEffort` 的调用点上下文（`agent/`）、`rpcwire.Wire` 的实现细节（`rpcwire/`）—— 涉及处一律标注「需另一条线确认」，未下结论。
7. **hermes 上游未审**：`runtime/hermes-agent/` 的 `resolve_reasoning_config` 是否提供只读配置查询口，本次未核实 —— 这决定 §1.3 修复方案是「重定向落点」还是「走官方读口 + 不写」。

---

## 13. P0/P1/P2 全清单（含修复代价与风险）

### P0（1）

| # | 发现 | 位置 | 代价 | 风险 |
|---|---|---|---|---|
| **P0-1** | `hermesapp` 在 `HERMES_HOME` 未设时**写用户桌面 agent 配置 `~/.hermes/config.yaml`**，违反 AGENTS.md 红线 2；与本包 `locate.go:13-14` 的「只读不改」注释矛盾；`cmd/hermeffortprobe` 被迫用 `-home` 强制沙箱反证；该分支零测试覆盖 | `internal/hermesapp/config.go:25-33,73-77` | 中（落点重定向 + 改 env 注入，或改走 hermes 官方读口 —— 后者需先核实上游能力） | 不碰甲级禁区/红线 1。改 `hermesapp` + `agent/hermes.go` 调用点（**跨线需协调**）。行为变化需在 SPEC 声明「daemon 不再跟随用户在 hermes CLI 的强度设置」 |

### P1（9）

| # | 发现 | 位置 | 代价 | 风险 |
|---|---|---|---|---|
| **P1-1** | `piapp` 头注释三连假：默认开关/无人调用/collabmcp 未接线，全部被 `main.go` 证伪 | `piapp/mcpsetup.go:16-19,76-77` | **极低**（改 8 行注释） | **0** |
| **P1-2** | mDNS 谎言注释 + 明文模式仍向 LAN 广播 `host_id`/机器名 | `cmd/pocketagentd/main.go:310,475-501` | 低（改注释即可消除误导；彻底修需产品决策） | **0**（`host_id` 语义依赖 `server/` 线，未定级更高） |
| **P1-3** | `hermesapp` 是 5 家里唯一不走 `stderrdrain`，子进程 stderr 全丢 | `hermesapp/client.go:327-333` | **极低**（1 函数替换） | **0**（日志量受 256KB 限流保护） |
| **P1-4** | `zcodeapp.SetDataDir` 不持 `prefMu` 就写包级 `modelPrefFile`，锁纪律漏一个写入方 | `zcodeapp/generate.go:384-396` | **极低**（3 行） | **0** |
| **P1-5** | `hermesapp.SetReasoningEffort` 无锁 read-modify-write + 固定 `.tmp` 名 | `hermesapp/config.go:37-78` | 低（加锁 + `CreateTemp`；或随 P0-1 一并消失） | **0** |
| **P1-6** | handler 快照发布 4 处逐字副本跨 3 个包 | `codexapp/dshapp/piapp/zcodeapp` 的 `client.go` | 中（泛型 `HandlerBox[T]`，4 文件 × ~40 行） | 不碰 wire 契约/proto/红线。现有 `handler_concurrency_test.go` ×3 作回归钉 |
| **P1-7** | daemon 数据目录派生手搓 4 份、失败兜底 3 种 | `dshapp/client.go:57`、`dshapp/credentials.go:51`、`piapp/mcpsetup.go:104`、`zcodeapp/generate.go:399` | 低（4 处改走 `hostenv.UserDataDir()`） | `hostenv` 是另一线域（**需协调**，但 API 已存在、签名不变）。行为变化 = dshapp 不再落 TempDir（是修正） |
| **P1-8** | 仓库内**无任何 CI 配置、无 git hook**：全部 45 处条件跳过只在单台开发机求值 | 全仓 | **极低**（1 个 20 行 workflow + `-short` 约定） | **0**（只读 job）。**不确定点**：是否刻意不设 CI —— 若刻意应写成显式决策 |
| **P1-9** | 掩盖型跳过：`拉不起测试用子进程` ×3（恰好掩盖最需要收尸测试的环境）+ `mdns` 在 CI 必跳 ×4（唯一的局域网暴露面） | `codexapp/close_reap_test.go:77`、`codexapp/client_test.go:340`、`dshapp/client_test.go:127`、`mdns/mdns_test.go:33/46/53` | 低（区分「环境不支持」与「代码坏了」；mdns 补纯逻辑测试） | **0** |

### P2（16）

| # | 发现 | 位置 |
|---|---|---|
| P2-1 | `agency.SourceFunc.Name_` 尾下划线 | `agency/agency.go:42` |
| P2-2 | `cmd/` 57 子包 651 KB 零测试、无归档标注、30/57 无包注释（建议 README + 归档标记） | `cmd/**` |
| P2-3 | `piapp` 两个测试抢同一个全局 helper 子进程态，需 `t.Skip` 互斥 | `piapp/entries_test.go:168` |
| P2-4 | 覆盖面声明 vs 实现：`agency` 头注释「四家」vs 只有 1 个生产 Source；`collabmcp:4` 「四家 harness」vs 已接入 5 家 | `agency/agency.go:1,3`、`collabmcp/collabmcp.go:4` |
| P2-5 | `agency.Compile` 用 `strings.Contains(From, from)` 做来源去重（子串匹配，非集合成员判定；`pi` 会误吞 `pip` 类来源名） | `agency/agency.go:98` |
| P2-6 | `fmt.Errorf` 零格式参数 3 处（应用 `errors.New`）；`dshapp:546` 与 `hermesapp:589` 同分支两种构造 = copy drift | `codexapp/client.go:470,494`、`hermesapp/acp.go:589` |
| P2-7 | `stderrdrain` 包注释停在「三个 harness」，实际 4 家在用、1 家未用 | `stderrdrain.go:1-8` |
| P2-8 | 「不值得引 sort」—— `sort` 是标准库，理由不成立（顺带：手写插入排序 6 行 vs `slices.Sort` 1 行） | `dshapp/collabpatch.go:125` |
| P2-9 | `cmd/pocketagentd/main()` 487 行 god function + `main()` 零覆盖 + 无包注释 | `cmd/pocketagentd/main.go:66-552` |
| P2-9b | `zcodeapp.autoAnswer` 3 个 goroutine 全吞 `Respond` 错误、无 ctx（其余 6 处 `autoAnswer` 同款） | `zcodeapp/client.go:612-651` |
| P2-9c | `panel` 吞 `srv.Serve(lis)` 与 `renderIndex(...)` 错误，面板中途失效/半截页面无人知晓 | `panel/panel.go:80,282` |
| P2-10 | `jobobject` 签名随平台变（`windows.Handle` vs `uintptr`）+ `main.go` 丢弃 Job 句柄（依赖「没人 CloseHandle」的隐式事实；误加 defer 会杀掉所有 agent 子进程） | `jobobject_other.go:9`、`jobobject_windows.go:32`、`main.go:386` |
| P2-11 | `Start` 签名 4 家用 `StartOptions` 结构、`hermesapp` 用 2 个位置参 | 各 `client.go` 的 `Start` |
| P2-12 | `agency.StoreFileSource.Probe(ctx)` 完全忽略 ctx（令 `Compile` 的「每源受 timeout 约束」契约对该来源失效） | `agency/store_source.go:18` |
| P2-13 | `StoreFileSource.Name()` = `"store:" + 绝对路径`，把用户完整文件路径写进 `Candidate.From` 诊断字段 | `agency/store_source.go:16` |
| P2-14 | 测试夹具寄居生产包：`stderrdrain.lineHasSecret`（生产文件里的测试专用私有函数）、`dshapp/probe_fixtures.go`（生产包里的导出探针快照）。**均已写明理由**，只记风格 | `stderrdrain.go:117`、`dshapp/probe_fixtures.go` |
| P2-15 | `term.Manager.Open/spawnSession` 6 位置参（内聚，可收 `SessionSpec`）；`term` 一个包做会话池 + Win32 键位两件事 | `term/manager.go:188,234`、`term/ctrl.go` |

---

## 14. 最值得先修的 3 条

1. **P0-1 · `hermesapp` 写 `~/.hermes/config.yaml`** —— 唯一的红线违反，且本包注释、探针红线声明、单测隔离三处都在替它撒谎。修它同时带走 P1-5（无锁 + 固定 tmp 名）。**先把 `cmd/hermeffortprobe` 那个 `-home` 强制沙箱的教训回灌到生产路径**。
2. **P1-8 · 没有 CI** —— 在没有机器门禁的前提下，上面 9 条 P1 的**任何一条**都只能靠下一个人重读代码发现。这一条的修复成本（约 20 行 workflow）是全部 P1 里最低的，收益是让其余 8 条不再复发。
3. **P1-6 · handler 快照发布四份副本** —— 本域唯一「明确该收、明确可收、收益确定、风险为零」的重复。3 个包 × 4 个文件 × ~40 行，且有 3 份 `handler_concurrency_test.go` 现成当回归钉。

（若只看红线维度，第 4 顺位是 **P1-2** mDNS 的 `host_id` 局域网广播 + 谎言注释 —— 改动只有注释，彻底修需要产品决策，但暴露面是真实的。）