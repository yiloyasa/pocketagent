# daemon P1 补位轮 + slash 位化轮收敛报告（2026-10-14）

- **收敛范围**：`git diff --stat 369392e..HEAD -- daemon/` 的 13 个文件 + P1 轮（`d849494`）新增文件。共 4 个文件改动，全部落在 `daemon/internal/` 下 `*.go` / `*_test.go`。
- **基线**：`369392e`（P1 补位+B3-2+双域收敛轮锚点）→ HEAD `d92967c`。
- **纪律**：只做等价重构，业务语义零改变；不改 `daemon/internal/gen/`、`cmd/`、`flutter/`、`android/`、`proto/`；不 git commit/add/push。
- **踩坑遵守**：全程只用 `gofmt -l` 查看，**未跑过 `go fmt` / `gofmt -w`**（R1 教训）。gofmt 既有遗留文件（hermes.go、hermes_compact.go、hermes_image_input.go、hermes_image_input_test.go 等 11 个）原样不动，不代修。

---

## 0. 基线与复跑证据（原文含用例数）

### 收敛前基线（改动前，同一工作树先复跑确认）

```
ok  	github.com/pocketagent/daemon/internal/agency	2.211s
ok  	github.com/pocketagent/daemon/internal/agent	20.404s
ok  	github.com/pocketagent/daemon/internal/codexapp	4.861s
ok  	github.com/pocketagent/daemon/internal/collabmcp	4.281s
ok  	github.com/pocketagent/daemon/internal/dshapp	5.016s
ok  	github.com/pocketagent/daemon/internal/eventstore	2.479s
ok  	github.com/pocketagent/daemon/internal/externalconv	2.197s
?   	github.com/pocketagent/daemon/internal/gen/pocketagent/v1	[no test files]
ok  	github.com/pocketagent/daemon/internal/hermesapp	4.876s
ok  	github.com/pocketagent/daemon/internal/hidewin	0.963s
ok  	github.com/pocketagent/daemon/internal/hostenv	2.266s
ok  	github.com/pocketagent/daemon/internal/hostenv/goscan	1.708s
ok  	github.com/pocketagent/daemon/internal/jobobject	1.409s
ok  	github.com/pocketagent/daemon/internal/mdns	2.377s
ok  	github.com/pocketagent/daemon/internal/panel	3.882s
ok  	github.com/pocketagent/daemon/internal/pathguard	3.964s
ok  	github.com/pocketagent/daemon/internal/piapp	13.907s
ok  	github.com/pocketagent/daemon/internal/rotatelog	1.937s
ok  	github.com/pocketagent/daemon/internal/rpcwire	3.209s
ok  	github.com/pocketagent/daemon/internal/runtime	5.148s
ok  	github.com/pocketagent/daemon/internal/selfguard	1.924s
ok  	github.com/pocketagent/daemon/internal/server	26.597s
ok  	github.com/pocketagent/daemon/internal/stderrdrain	1.559s
ok  	github.com/pocketagent/daemon/internal/store	3.273s
ok  	github.com/pocketagent/daemon/internal/term	3.079s
ok  	github.com/pocketagent/daemon/internal/testagent	4.541s
?   	github.com/pocketagent/daemon/internal/testkit	[no test files]
ok  	github.com/pocketagent/daemon/internal/tlscert	2.486s
?   	github.com/pocketagent/daemon/internal/version	[no test files]
ok  	github.com/pocketagent/daemon/internal/zcodeapp	13.350s
```

用例数（`go test -v` 逐包统计）：

```
agent PASS=774 FAIL=0 SKIP=9
server PASS=590 FAIL=0 SKIP=0
合计 PASS=1364 FAIL=0 SKIP=9
```

（9 个 SKIP 全为 `pa_live` / 环境变量门控的 live 用例：TestLiveCodexReadOnlySurface、TestLiveCodexSectionLifecycle、TestLiveCodexThreadOrganization、TestLiveCodexGuardianApproveAccepted、TestLiveCodexGuardianAllActionVariantsAccepted、TestNativePiModelSelection、TestNativeSelectedTextGenerator、TestNativeAgentCatalogs、TestNativeCodexGoalProtocol。）

### 收敛后复跑（全部改动完成后）

```
=== FINAL: go build ===
build exit=0
=== FINAL: go vet agent+server ===
vet exit=0
=== FINAL: full test ===
ok  	github.com/pocketagent/daemon/internal/agency	2.526s
ok  	github.com/pocketagent/daemon/internal/agent	20.427s
ok  	github.com/pocketagent/daemon/internal/codexapp	4.874s
ok  	github.com/pocketagent/daemon/internal/collabmcp	3.377s
ok  	github.com/pocketagent/daemon/internal/dshapp	4.930s
ok  	github.com/pocketagent/daemon/internal/eventstore	1.795s
ok  	github.com/pocketagent/daemon/internal/externalconv	1.172s
?   	github.com/pocketagent/daemon/internal/gen/pocketagent/v1	[no test files]
ok  	github.com/pocketagent/daemon/internal/hermesapp	4.884s
ok  	github.com/pocketagent/daemon/internal/hidewin	1.098s
ok  	github.com/pocketagent/daemon/internal/hostenv	1.414s
ok  	github.com/pocketagent/daemon/internal/hostenv/goscan	1.197s
ok  	github.com/pocketagent/daemon/internal/jobobject	1.219s
ok  	github.com/pocketagent/daemon/internal/mdns	1.401s
ok  	github.com/pocketagent/daemon/internal/panel	2.863s
ok  	github.com/pocketagent/daemon/internal/pathguard	3.869s
ok  	github.com/pocketagent/daemon/internal/piapp	13.100s
ok  	github.com/pocketagent/daemon/internal/rotatelog	1.285s
ok  	github.com/pocketagent/daemon/internal/rpcwire	2.210s
ok  	github.com/pocketagent/daemon/internal/runtime	4.244s
ok  	github.com/pocketagent/daemon/internal/selfguard	1.101s
ok  	github.com/pocketagent/daemon/internal/server	30.413s
ok  	github.com/pocketagent/daemon/internal/stderrdrain	1.464s
ok  	github.com/pocketagent/daemon/internal/store	3.331s
ok  	github.com/pocketagent/daemon/internal/term	3.552s
ok  	github.com/pocketagent/daemon/internal/testagent	4.470s
?   	github.com/pocketagent/daemon/internal/testkit	[no test files]
ok  	github.com/pocketagent/daemon/internal/tlscert	2.447s
?   	github.com/pocketagent/daemon/internal/version	[no test files]
ok  	github.com/pocketagent/daemon/internal/zcodeapp	13.641s
```

用例数对照（收敛后，与基线逐字相同）：

```
agent PASS=774 FAIL=0 SKIP=9
server PASS=590 FAIL=0 SKIP=0
合计 PASS=1364 FAIL=0 SKIP=9
```

**零倒退**：27 个有测试包全 `ok`，用例数 1364 = 1364，FAIL 0 = 0，SKIP 9 = 9。测试未删、未跳过、未弱化。

### 额外证据：-race 复跑（slash 捕获器涉及 goroutine 锁序）

```
=== race: agent ===
ok  	github.com/pocketagent/daemon/internal/agent	20.306s
=== race: server ===
ok  	github.com/pocketagent/daemon/internal/server	33.850s
```

### gofmt 检查（只读，未写）

```
$ gofmt -l internal/
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

**与收敛前基线逐字相同**（11 个遗留文件，全为注释续行/对齐的历史遗留；本批 4 个改动文件 `hermes_slash.go`、`slash_commands.go`、`slash_commands_test.go`、`hermes_slash_route_test.go` 全部 gofmt 干净）。按纪律不代修遗留文件。

---

## 1. 盘点清单（全量盘点结果）

对 `369392e..HEAD` 的 13 个文件 + P1 轮新增文件逐文件过一遍，按「重复/可收敛」维度盘点如下。**已做** 3 项，**已识别未动** 8 项（含硬禁区与语义差异保留）。

| # | 位置 | 盘点结论 | 处置 |
|---|---|---|---|
| 1 | `server/slash_commands.go` hermes 分支 vs pi 分支 | 两处**逐字节相同**的内联匿名接口断言 + 取数（`s.cfg.Providers[X].(interface{SlashCommandsFor(string) []*pb.SlashCommand})` + `append(...)`） | **已做**（收敛项 1） |
| 2 | `server/hermes_slash_route_test.go` `hermesSlashFakeProvider` vs `server/slash_commands_test.go` `slashFakeProvider` | 同结构同方法，仅差一个 `queryCalls` 记录字段；同包同签名形状 | **已做**（收敛项 2） |
| 3 | `agent/hermes_slash.go:257` 编译期钉死注释 | 引用了**不存在的符号** `host.go 的 piShapedSlashSource`（真实名为 `threadShapedSlashSource` / `cwdShapedSlashSource`） | **已做**（收敛项 3，纯注释） |
| 4 | `agent/hermes_compact.go` 三道闸 vs pi `CompactThread` 三闸 | 闸的**顺序与判据同源**，但每闸的哨兵类型、锁对象（`turnMu` vs `p.mu`+`running` map）、文案都不同；zcode 又是第三套 | 不动（跨 provider 语义差异保留，见 §3.4） |
| 5 | `agent/text_generator.go` 五家 `GenerateText` | 共同骨架只有 `MkdirTemp + RemoveAll + generationThread + denyGenerationTool + textFromBlocks`，且这五件**已经是抽出来的共用 helper**；剩余部分各家语义差异（进程模型/ephemeral/effort 落点/收尾方式） | 不动（见 §3.5 逐家差异表） |
| 6 | `server/host.go` slash 两位断言 vs compact 三形状 | compact 用**三个具名接口类型**（`codexShapedCompactor` 等）各自一条 `if`；slash 用两个具名接口 + 两条 `if`。**已经是同款结构**，且位输出逐位不变 | 不动（见 §3.6） |
| 7 | `agent/capability_gate_test.go` slash probe vs fork probe | 两处都是「两形状任一」的内联匿名断言闭包；与 rollback/review/export_html 的「单形状」闭包同表驱动风格 | 不动（见 §3.7） |
| 8 | `agent/hermes_slash.go` 捕获器 vs `hermes_image_input.go` / Run 路径 | 三处**没有**重复的 try-lock/缓冲/commit 样板：image_input 无钩子无锁；Run 路径的钩子安装是泵而非捕获器 | 不动（见 §3.2） |
| 9 | `agent/hermes.go` 账本双写入点时序 | **硬禁区**（解 1ms 竞速的设计） | 禁区不动 |
| 10 | `agent/hermes_compact.go` 三道闸 / hermes 三道闸 | **硬禁区** | 禁区不动 |
| 11 | `agent/text_generator.go` hermes 零 config.yaml 写入 | **红线 2 纪律** | 红线不动 |
| 12 | `agent/text_generator_20261014_test.go` `hermesThreadRefs` / `dshThreadRefs` | 两个 4 行同构小 helper，但读的是**两个不同 provider 的不同锁字段**，且各只被本文件用一次 | 不动（见 §3.8） |
| 13 | `server/hermes_compact_route_test.go` / `hermes_slash_route_test.go` fake 基座 | 分别复用既有 `PiProviderStub` / `seqProvider`，无重复 | 不动 |
| 14 | `agent/capability_gate_test.go` 注释里 `host.go:<line>` 行号引用 | 多处行号已漂移（`host.go:121-129`、`host.go:150`、`host.go:152-157`、`host.go:93`、`host.go:75-155`） | **已识别未动**：经 `git show 369392e` 核对，这些引用**在本批之前就已漂移**（前几轮收敛遗留），不属本轮范围；记录移交（见 §4.1） |

---

## 2. 逐项收敛记录 + 等价论证

### 收敛项 1：`server/slash_commands.go` pi/hermes 两分支的同形断言收敛为 `providerSlashCommands`

**改动前**（两处逐字节相同的 5 行）：

```go
// pi 分支
if p, ok := s.cfg.Providers["pi"].(interface {
    SlashCommandsFor(string) []*pb.SlashCommand
}); ok {
    resp.Commands = append(resp.Commands, p.SlashCommandsFor(t.ID)...)
}

// hermes 分支
if p, ok := s.cfg.Providers["hermes"].(interface {
    SlashCommandsFor(string) []*pb.SlashCommand
}); ok {
    resp.Commands = append(resp.Commands, p.SlashCommandsFor(t.ID)...)
}
```

**改动后**：

```go
} else if threadAgentID(t) == "pi" {
    resp.Commands = append(resp.Commands, s.providerSlashCommands("pi", t.ID)...)
    if len(resp.Commands) == 0 && req.Refresh && t.Workspace != "" {
        // ... pi 的 Refresh 兜底拉取原样保留
    }
} else if threadAgentID(t) == "hermes" {
    // ... hermes 分支注释
    resp.Commands = append(resp.Commands, s.providerSlashCommands("hermes", t.ID)...)
}
```

新增 helper：

```go
func (s *Server) providerSlashCommands(agentID, threadID string) []*pb.SlashCommand {
    p, ok := s.cfg.Providers[agentID]
    if !ok {
        return nil
    }
    src, ok := p.(interface {
        SlashCommandsFor(string) []*pb.SlashCommand
    })
    if !ok {
        return nil
    }
    return src.SlashCommandsFor(threadID)
}
```

**等价论证（逐条）**：

1. **provider 查找等价**：原代码 `s.cfg.Providers["pi"]`（下标取值 + `, ok` 双返回值）在 key 缺失时 `ok=false` 走不进 if 体；helper 第一步 `p, ok := s.cfg.Providers[agentID]; if !ok { return nil }`。两者对「key 缺失」的产出都是**不 append 任何元素**，等价。
2. **形状断言等价**：匿名接口字面量 `interface{ SlashCommandsFor(string) []*pb.SlashCommand }` 在 helper 里与原两处**逐字节相同**（同一方法集、同一签名）。Go 的结构化类型判定对匿名接口是形状匹配，与原内联版本判定结果完全一致。`host.go` 的 `threadShapedSlashSource` 是同一方法集的具名版本，三者互相可满足 —— 这也正是 helper 注释里「host.go 的 threadShapedSlashSource 位断言同款最小形状」的依据。
3. **取数与 append 等价**：helper 返回 `src.SlashCommandsFor(threadID)`，调用方 `resp.Commands = append(resp.Commands, <nil 或切片>...)`。`append(x, nil...)` 是恒等操作（Go spec：`append(s, ...nil)` 不改变 s），与「ok=false 时根本不进 if 体」的产出逐字节相同。原代码在 ok=true 时 `append(..., p.SlashCommandsFor(t.ID)...)`，helper 路径同。
4. **agentID 参数化不改变语义**：两处调用点分别硬传 `"pi"` / `"hermes"`，与原先两处硬编码的 key 一致。helper 是纯函数式的查表，无副作用、无状态。
5. **pi 分支的 Refresh 兜底不受影响**：`if len(resp.Commands) == 0 && req.Refresh && t.Workspace != ""` 这段判定读的是 `resp.Commands` 的长度。改动前后 `resp.Commands` 的内容一致（见 1–3），故该判定的真值不变，piapp 冷启路径的触发条件逐字节不变。
6. **hermes 分支「没有兜底拉取」不受影响**：该分支原本就只有账本直透一条路径，改动后仍只有这一条（helper 不含任何拉取逻辑）。
7. **失败哲学不变**：helper 在 provider 不在场/形状不匹配时返回 nil，调用方安静收成空列表，与原两处「ok=false 就不 append」一致，仍然不报错、不 Toast。

**测试证据**：`go test -count=1 ./internal/server/` ok（30.413s），其中直接钉这两条分支的用例如下，全绿且断言未改：
- `TestListSlashCommandsHermesLedgerWins`（账本命中字段按序透出 + 查询用线程 id）
- `TestListSlashCommandsHermesLedgerMissReturnsEmpty`（refresh 两态都回空、Refresh 不改支路）
- `TestListSlashCommandsHermesRouteIsolatedByAgentID`（zcode 会话不碰 hermes 账本）
- `TestListSlashCommandsHermesProviderAbsentOrUnshaped`（provider 缺席 / 无账本形状 → 空列表不报错）
- `TestListSlashCommandsHermesRealProviderEmptyLedger`（真 `*agent.HermesProvider` 未记账 → 空列表）
- `TestListSlashCommandsNonZcodeReturnsEmpty`（pi 会话不碰 zcode 拉取，`calls == 0`）
- `TestListSlashCommandsProviderLedgerWins`（zcode 账本通路优先，零子进程）

**参照先例**：与 R2 轮收敛 `requireCodex`（server.go 3 文件 9 处）同款——照「同形断言抽最小 helper」先例评估后执行。此处比 `requireCodex` 更保守：`requireCodex` 收敛的是**错误返回**路径，本项只收敛**成功取数**路径，且两家的下游差异（pi 有 Refresh 兜底、hermes 没有）完整保留在各分支里，没有为了合并而把差异也揉进 helper。

**净效果**：删 10 行重复，增 19 行（含 12 行注释）。行数略增但重复归零；两处调用点从 5 行缩到 1 行，可读性提升。

---

### 收敛项 2：`hermes_slash_route_test.go` 的 `hermesSlashFakeProvider` 复用 `slash_commands_test.go` 的 `slashFakeProvider`

**改动前**：两个结构体同构，仅 `queryCalls` 字段之差：

```go
// slash_commands_test.go（既有）
type slashFakeProvider struct {
    seqProvider
    commands []*pb.SlashCommand
}
func (p *slashFakeProvider) SlashCommandsFor(threadID string) []*pb.SlashCommand {
    return p.commands
}

// hermes_slash_route_test.go（本批新增）
type hermesSlashFakeProvider struct {
    seqProvider
    commands   []*pb.SlashCommand
    queryCalls []string // SlashCommandsFor 的实参记录(线程 id 透传钉子)
}
func (p *hermesSlashFakeProvider) SlashCommandsFor(threadID string) []*pb.SlashCommand {
    p.queryCalls = append(p.queryCalls, threadID)
    return p.commands
}
```

**改动后**：删掉 `hermesSlashFakeProvider`，`slashFakeProvider` 增加 `queryCalls` 记录（zcode 分支原有 5 处用例不读该字段，行为不变）；`hermes_slash_route_test.go` 的 4 处构造点全部改用 `slashFakeProvider`。

**等价论证**：

1. **同包同层**：两个文件同在 `package server`，`slashFakeProvider` 对本文件可见，无需 export、无需新文件。
2. **接口满足面不变**：两个类型都只实现 `agent.Provider`（经嵌入 `seqProvider`）+ 账本形状 `SlashCommandsFor`。合并后仍是这一个类型满足同一组接口，`newSlashServer(t, map[string]agent.Provider{...})` 的入参类型不变。
3. **`queryCalls` 是纯追加记录，不影响返回值**：`p.queryCalls = append(...)` 后照旧 `return p.commands`。对**不读** `queryCalls` 的 zcode 分支 5 处用例（`TestListSlashCommandsProviderLedgerWins`、`TestListSlashCommandsNonZcodeReturnsEmpty`、`TestListSlashCommandsZcodeWithoutSessionReturnsEmpty`、`TestListSlashCommandsFetchSuccessAndCache`、`TestListSlashCommandsFetchFailureReturnsEmpty` 等）而言，`SlashCommandsFor` 的返回值与副作用都与改动前逐字节相同 —— 多写的那个 slice 字段无人读，等价于不存在。
4. **读 `queryCalls` 的 hermes 用例断言不变**：`TestListSlashCommandsHermesLedgerWins` 断言 `len(ledger.queryCalls) != 1 || ledger.queryCalls[0] != "th-h1"`；`TestListSlashCommandsHermesLedgerMissReturnsEmpty` 断言 `len(ledger.queryCalls) != 2`；`TestListSlashCommandsHermesRouteIsolatedByAgentID` 断言 `len(hermesLedger.queryCalls) != 0`。合并后这些字段名、语义、记录时机（每次 `SlashCommandsFor` 调用追加一次实参）与原 `hermesSlashFakeProvider` 完全一致。
5. **并发安全性不变**：两个 fake 都无锁，靠单 goroutine 测试驱动；合并不引入新的并发面。
6. **不弱化任何钉子**：`hermesSlashFakeProvider` 承担的「路由只认形状不认 agent 名字」钉法（`seqProvider{id: "hermes"}` + 只加账本方法）原样搬到 `slashFakeProvider` 上，构造点从 `&hermesSlashFakeProvider{...}` 改为 `&slashFakeProvider{...}`，其余字段初始化不变。

**为什么不进一步合并 `codexSlashFake`**：`codexSlashFake` 的方法是 `SlashCommands(cwd string)`（cwd 形状），与账本形状 `SlashCommandsFor(threadID string)` **方法名与签名都不同**，且多带 `gotCwd` / `errDesired` 两个字段。三者已由「最小形状」原则自然分开，强行合并反而要背上形状判定的分支，与 `host.go` 位生成「按形状不按 agent 名分支」的纪律相悖。故只合并同签名同语义的两个。

**测试证据**：`go test -count=1 ./internal/server/` ok；`TestListSlashCommands*` 与 `TestZcodeSupplement*` 共 13 个用例全绿（单独跑 `-run 'TestListSlashCommands|TestZcodeSupplement'` 亦 ok）。

**净效果**：删 11 行（类型 + 方法 + 注释），增 8 行（字段 + 一行记录 + 4 行说明注释）。fake 从 3 个同构降到 2 个形状（账本形 / cwd 形），与 host.go 的两个 slash 位形状一一对应。

---

### 收敛项 3：`agent/hermes_slash.go` 编译期钉死注释引用不存在的符号

**改动前**：

```go
// 编译期钉死:hermes 命令面与 pi/zcode 同形状(host.go 的 piShapedSlashSource
// 位断言与 server ListSlashCommands 路由都按形状走,不按 agent 名分支)。
```

**改动后**：

```go
// 编译期钉死:hermes 命令面与 pi/zcode 同形状(host.go 的
// threadShapedSlashSource 位断言与 server ListSlashCommands 路由都按形状走,
// 不按 agent 名分支)。
```

**等价论证**：纯注释文本修正，`var _ interface{...} = (*HermesProvider)(nil)` 这行代码一字未动，编译产物与运行行为零变化。修的依据是硬事实：全仓检索 `piShapedSlashSource` **只命中这一条注释本身**（无声明、无使用），而 `host.go:68` 的真实声明是 `threadShapedSlashSource`（账本形，pi/zcode/hermes 共用）、`host.go:72` 是 `cwdShapedSlashSource`（codex 技能形）。注释引用一个不存在的符号，会让后来按图索骥的人去搜一个凭空名字 —— 属于本轮新代码自己引入的文档债，按范围应当场清掉。

**测试证据**：`go build ./...` exit=0；`go test -count=1 ./internal/agent/` PASS=774 FAIL=0 SKIP=9（与基线一致）。

**净效果**：+2 行 −1 行，纯注释。

---

## 3. 已识别未动（含禁区与语义差异保留）

### 3.1 硬禁区（一字未动）

| 禁区 | 位置 | 未动理由 |
|---|---|---|
| 能力位生成语义 | `server/host.go:154-190` compact 三形状 + slash 两形状断言段 | 位输出逐位不变才允许动手；本次收敛项 1 未触及 host.go，slash 位仍由两个具名接口各自一条 `if` 生成，位序、位名、判定逻辑零变化。`host_capability_bits_test.go` 的 `TestGetHostInfoCapabilityBitsAlignedWithControllers` 全格绿证（含新增的 slash:codex/pi/zcode/hermes 在场 + slash:dsh 缺席）。 |
| 账本双写入点时序 | `agent/hermes.go` `createSession` 窗口捕获器（`beginSlashCapture`/`bind`/`commit`/`close` 时序）+ Run 泵消费侧 `noteCommandsFromEvent` | 这是解「适配器 session/new 应答后 ~1ms 发广告 vs Run 泵尚未安装」竞速的设计。收敛项 1、2 都不在 agent 包动 hermes.go；收敛项 3 只改注释。成功路径不卸载、失败路径立刻闭窗、commit 只走一次、`cap.mu → threadMu` 单向锁序 —— 全部原样。 |
| hermes 三道闸 | `agent/hermes_compact.go` 闸① turnActive 快判 + `turnMu.TryLock`；闸② 未落地；闸③ 预算 + `cancelTurn` | 顺序与判据是竞速/失败安全设计，未动。 |
| 红线 2（~/.dsh、G:\Hermes 只读） | `agent/text_generator.go` hermes `GenerateText` 恒传空 effort → Run 的 `Effort != ""` 强度写入段跳过 → 零 config.yaml 写入 | 由 `TestHermesGenerateTextReturnsReplyAndCleansUp` 钉死（`HERMES_HOME` 指 `t.TempDir()`，生成后断言 config.yaml 不含 `reasoning_effort`）。未动。 |

### 3.2 hermes 三处「窗口/钩子」结构评估：**不重复，不并**

任务书要求评估「会话建立窗口捕获器模式是否与 hermes_image_input.go 或 Run 路径里已有的窗口/缓冲结构重复」。逐处核对结论：

| 位置 | 有无钩子 | 有无锁 | 有无缓冲/commit | 判定 |
|---|---|---|---|---|
| `hermes_slash.go` `hermesSlashCapture` | 有（`SetTurnHandlers` 只认命令帧） | 有（`cap.mu`） | 有（`buffered` + `bind` + `commit` + `close` 四态生命周期） | 基线 |
| `hermes_image_input.go` | **无**（纯函数 `hermesInputBlocks` / `hermesPromptContent` / `sendHermesPrompt`，不碰 `SetTurnHandlers`） | **无** | **无** | 无重合 |
| `hermes.go` Run 泵 | 有（`notif` / `reqHandler` 双钩子 + `notifCh` 队列 + `stopPump` 收尸） | 有（`toolMu` + `enqueued/processed` 原子栅栏 + `sync.OnceFunc`） | 有（4096 容量 channel 缓冲） | 语义不同，见下 |

**不并的理由**：Run 的泵是「全量帧生产者-消费者队列 + 看门狗 + 排空栅栏」，捕获器是「单变体过滤器 + 三态窗口 + 按 sessionId 对表入账」。两者的缓冲目标不同（泵缓冲**所有**本会话帧保序投递，捕获器只缓冲**命令帧**且提交时按 sid 对表）、收场语义不同（泵 `stopPump` 幂等收尸等消费者退出，捕获器 `close` 丢缓冲闭窗）、锁序不同（泵不持 provider 锁做投递，捕获器刻意在 `cap.mu` 内做账本写入以让「缓冲提交」与「提交后直达」互斥）。硬并成一个 helper 会引入参数开关，反而把三处时序语义搅成一锅 —— 与「等价优先、语义不同不并」的纪律相悖。

### 3.3 hermes_compact.go 的 `noteCommandsFromEvent` 调用点：不抽

`hermes_compact.go:106` 的钩子里也调了 `p.noteCommandsFromEvent(t.ID, sid, ev)`。它与 `hermes.go` Run 泵消费侧的调用是同一个函数、同一签名，**已经是共用的**（这正是收敛项 1 之前就做对的事）。两处调用点的 `expectSID` 来源不同（Run 用本轮 `sid`，压缩轮用本地 `sid`），但都走同一函数，无需再抽。

### 3.4 hermes_compact 三道闸 vs pi `CompactThread`：不并

盘点发现两者闸的顺序与判据同源（快判 busy → TryLock → 未落地 → 预算 → 终局判据），但逐项都是差异：

| 维度 | hermes (`hermes_compact.go`) | pi (`pi.go:1775`) | zcode |
|---|---|---|---|
| busy 判据 | `p.turnActive` 快判 + `p.turnMu.TryLock()` | `p.running[t.ID]` map 查 | 另一套 |
| 锁对象 | `turnMu`（provider 级） | `p.mu` 短暂持锁读 map | — |
| 未落地判据 | `threadRefFor(t.ID)` 内存引用 **或** `t.ExternalSession.SessionID` | `t.ExternalSession.SessionFile == ""` | — |
| 哨兵类型 | `errHermesCompactBusy`（`busySentinelError` → Unwrap `ErrBusy`）+ `errHermesNotLanded`（裸 `fmt.Errorf`） | `piError{code:"PI_BUSY"/"PI_NOT_MOUNTED"}`（带错误码） | `ErrZcodeBusy` |
| 预算 | `hermesCompactBudget` 180s + `cancelTurn` | `piCompactBudget` | — |
| 终局判据 | `stopReason` 空/error + 正文空三重 | `res.TokensBefore` 日志 | — |

**不并的理由**：三家的「闸」是同源设计思想，但哨兵类型、错误码、锁对象、未落地判据、终局判据全都不同 —— server 层的 CompactThread 路由靠 `errors.Is`/错误码区分忙态与未落地，合并会把三家的错误面揉平。保留三份，符合前几轮收敛对 `busySentinelError` vs `piError` 差异的处理先例。

### 3.5 text_generator.go 五家 GenerateText：不并（共同骨架已是 helper，剩余全是语义差异）

盘点结论：**「已有共用部分」就是抽好的那五个 helper**，各家剩下的都是不可合并的差异。

已共用（无需再动）：
- `os.MkdirTemp("", "pocketagent-prompt-")` + `defer os.RemoveAll(dir)` —— 五家都有
- `generationThread(id, model, effort, workspace)` —— 五家共用（含 `SettingsSnapshot{Sandbox:"read-only", ApprovalPolicy:"untrusted", HideReasoning:true}` 一次性设置）
- `denyGenerationTool` —— 五家共用（红线 1：优化轮审批一律拒答）
- `textFromBlocks(blocks)` —— 五家共用收尾
- `[]pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: prompt}}}}` —— 五家共用输入形状

剩余差异（逐家，**不可合并**）：

| 家 | 进程模型 | 特有步骤 | effort 落点 | 收尾 |
|---|---|---|---|---|
| codex | 新起进程 | `NewCodexProvider()` + `lifeMu` 持锁关 client + `ApplyPrefs(DesktopPrefs{HideReasoning:true})` + `Ephemeral: true` | 随 thread 下传 | defer 关 client |
| pi | 新起进程 | `p.textOnly = true` + `defer p.DropThread(thread.ID)` | 随 thread 下传 | DropThread |
| zcode | **完全另一条路** | `zcodeapp.Start(...)` + `c.GenerateTextSelected(ctx, strings.TrimPrefix(model,"zcode/"), ...)`，不走 `Run` | 随 `GenerateTextSelected` 下传 | `defer c.Close()` |
| hermes | **共享进程**（复用本 provider） | `effort != ""` 时打日志显式忽略 + `generationThread("hermes", model, "", dir)` 恒传空 effort（红线 2：唯一入口是 config.yaml，一次性轮不改用户配置） | **显式丢弃** | `defer p.dropThreadRef(thread.ID)` |
| dsh | **共享进程**（复用本 provider） | defer 里 `CloseSession(sid)` 真删 + `dropDshThreadRef` | 随 `applyConfigOptions` → `session/set_config_option`（会话内生效不持久） | CloseSession + dropDshThreadRef |

**不并的理由**：hermes/dsh 复用共享实例（省一次 ACP 冷启 + 一份进程），codex/pi 新起实例，zcode 连 `Run` 都不走 —— 三种进程模型。「抽一个 `runEphemeralTextTurn` 大 helper」需要 6+ 个可选参数（是否新起 provider、是否 Ephemeral、是否 textOnly、effort 是否透传、收尾回调、是否 CloseSession），参数数量超过被省下的代码量，且每家注释里都挂着不可压缩的取证叙事（hermes 的红线 2 论证、dsh 的 set_config_option 实证、codex 的 ephemeral 形态）。判定：**共同骨架已经收敛到位，剩余部分是语义差异，保留**。

### 3.6 host.go slash 两位断言 vs compact 三形状：已是同款结构，不动

`compact` 用三个具名接口（`codexShapedCompactor` / `piShapedCompactor` / `zcodeShapedCompactor`）各自一条 `if`；`slash` 用两个具名接口（`threadShapedSlashSource` / `cwdShapedSlashSource`）各自一条 `if`。**结构已完全同款**（同款「多形状一位」先例：每家形状一条独立 `if`，append 同一位）。

评估「能否再收敛成一条」：若合成 `if _, ok := provider.(threadShapedSlashSource); ok { caps = append(caps, "slash:"+id) } else if _, ok := provider.(cwdShapedSlashSource); ok { ... }`，会改变位输出语义风险（虽然当前无 provider 同时满足两形状，但这是**巧合而非契约**：一个假 provider 同时实现两个方法时，现结构会 append 两次 `slash:<id>`（`sort.Strings` 后仍是两个重复串），合成结构只 append 一次）。现结构的多形状各自独立 `if` 与 compact 三形状**逐位同构**，`host_capability_bits_test.go` 的 `caps["slash:"+id]` map 查找对重复串不敏感，故当前测试绿。但为「位输出逐位不变」的硬纪律，**不动**：本项的收益（省 3 行）远低于改动能力位生成段的风险，且 compact 那边也是三行并列而非合并行 —— 保持并排结构本身就是可读性资产。

### 3.7 capability_gate_test.go slash probe vs fork probe：不并

`fork` 的 probe（两形状任一）与 `slash` 的 probe（两形状任一）是同形闭包，`rollback` / `review` / `export_html` 是单形状闭包，`compact` 是三形状闭包（已抽成 `compactShaped` 具名），`image_input` 是「断言 + 调方法」（已抽成 `imageInputShaped` 具名）。

评估「抽一个 `anyShaped(ifaces...)` 泛化 helper」：会让每个 probe 从「直白的匿名接口字面量」变成「接口切片参数」，而表驱动的可读性正来自「probe 的形状在表里一眼可读」。且 probe 数量只有 5 个内联 + 2 个具名，重复的是 4 行样板而非逻辑。判定：**同表驱动风格优先于消灭 4 行样板**，不并。（这也与前几轮收敛对 `four_provider_hooks_test.go` 同类探针的处理一致。）

### 3.8 text_generator_20261014_test.go 的 hermesThreadRefs / dshThreadRefs：不并

```go
func hermesThreadRefs(p *HermesProvider) int {
    p.threadMu.Lock()
    defer p.threadMu.Unlock()
    return len(p.threadMap)
}
func dshThreadRefs(p *DshProvider) int {
    p.threadMu.Lock()
    defer p.threadMu.Unlock()
    return len(p.threadMap)
}
```

**不并的理由**：两个函数体逐字相同，但 (a) 参数类型不同（`*HermesProvider` / `*DshProvider`），Go 无泛型情况下要么上 `any` + 反射/接口断言（更丑），要么定义一个小接口 `interface{ threadRefCount() int }` 并要求两个 provider 都实现（**为非测试代码添生产面**，违反「测试债不挪到生产」）；(b) 各只在本文件用 2 次（hermes 1 次 + dsh 1 次 + 各自前置）。4 行 × 2 的代价换任何方案都不划算。保留。

---

## 4. 移交建议

### 4.1 `capability_gate_test.go` 里 `host.go:<line>` 行号引用漂移（前轮遗留，非本批）

`git show 369392e:daemon/internal/agent/capability_gate_test.go` 核对确认，以下引用在本批之前就已漂移（host.go 现 452 行，实际位置见括号）：

- `host.go:75-155`（capabilities 段，实际 96-202）
- `host.go:121-129`（compact 三形状，实际 154-162）
- `host.go:150`（image_input，实际 199）
- `host.go:152-157`（fork，实际 163-169）
- `host.go:93`（collab，实际 126）
- `host.go:155 piShapedForker`（实际 163-168）

**未动理由**：不属「2026-10-14 两轮新增/修改」范围（本批只往该文件加了 `slash` 位族一行 + 一个 probe 块 + 改了两行注释，未新增行号引用）。建议下一轮（或 host.go 下次改动时）一并校准，或改成符号名引用（`host.go 的 codexShapedCompactor 段`）——行号引用在这类高频增删的文件里天生易腐，符号名才稳。

### 4.2 其余移交

1. **`providerSlashCommands` 的第三家潜在调用**：若将来 dsh 也落 `SlashCommandsFor` 账本形（当前 dsh 无命令面，`slash:dsh` 位缺席是诚实声明），直接 `s.providerSlashCommands("dsh", t.ID)` 一行接入，无需再写断言样板。
2. **`slashFakeProvider` 的 queryCalls 已成为公共字段**：将来任何路由分支要钉「查询按线程 id 透传」，直接读该字段，不必再养 fake。
3. **gofmt 遗留 11 文件**：按纪律未代修。若要清，建议单独一轮「纯格式化轮」并用 `gofmt -w` 明确列出文件清单，避免与业务 diff 相混（R1 教训：`go fmt` 是写操作）。

---

## 5. 改动文件清单（4 个，全在 daemon/internal/ 下）

| 文件 | 收敛项 | 行数变化 |
|---|---|---|
| `daemon/internal/server/slash_commands.go` | 项 1：pi/hermes 同形断言 → `providerSlashCommands` helper；hermes 分支注释同步 | +48 / −13（净 +35，含 19 行注释） |
| `daemon/internal/server/slash_commands_test.go` | 项 2：`slashFakeProvider` 增 `queryCalls` 记录（供 hermes 分支共用） | +8 / −3 |
| `daemon/internal/server/hermes_slash_route_test.go` | 项 2：删 `hermesSlashFakeProvider`，4 处构造点改用 `slashFakeProvider` | +4 / −21（净 −17） |
| `daemon/internal/agent/hermes_slash.go` | 项 3：编译期钉死注释修正不存在的符号引用 | +3 / −2 |

`git diff --stat` 原文：

```
 daemon/internal/agent/hermes_slash.go             |  5 ++-
 daemon/internal/server/hermes_slash_route_test.go | 21 +++-------
 daemon/internal/server/slash_commands.go          | 48 +++++++++++++++--------
 daemon/internal/server/slash_commands_test.go     |  8 +++-
 4 files changed, 47 insertions(+), 35 deletions(-)
```

**未触碰**：`daemon/internal/gen/`、`cmd/`、`flutter/`、`android/`、`proto/` 零改动。**未 git commit/add/push**（工作树仅这 4 个文件处于 modified）。

---

## 6. 结论

- 本批新代码整体质量高：双写入点时序、三道闸、能力位生成、红线 2 纪律全部落实，测试钉子密而准（hermes_slash_test.go 12 用例覆盖解析/两写入点/隔离/失败面/收尸/重挂载/并发；hermes_slash_route_test.go 5 用例覆盖路由五态）。
- 可收敛的重复只有 3 处，全部做掉：**一处生产代码同形断言**（pi/hermes 分支）、**一处测试 fake 同构重复**、**一处注释引用不存在的符号**。全部为等价重构，业务语义零改变，每条等价论证见 §2。
- 盘点中另外 8 处「看似重复」经逐项核对判定为**语义差异保留**（§3.2–3.8）或**硬禁区**（§3.1），未动。
- 测试证据：收敛前后 `agent PASS=774 FAIL=0 SKIP=9` / `server PASS=590 FAIL=0 SKIP=0`，合计 **1364 用例 0 失败**，逐字相同；`go build` / `go vet ./internal/agent/ ./internal/server/` 全 exit=0；`-race` 两个改动包 ok。gofmt 遗留清单与基线逐字相同，本批 4 个文件全部 gofmt 干净。
