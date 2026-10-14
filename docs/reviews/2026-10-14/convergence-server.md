# daemon/internal/server 代码收敛报告（2026-10-14）

> 任务：对 `daemon/internal/server/` 做纯收敛重构 —— 消除重复与冗余，业务语义零改变。
> 约束：只改 `internal/server/` 下 `*.go` 与 `*_test.go`；不 commit（主会话统一提交）；
> 能力位生成（host.go）为硬契约，位输出必须逐位不变。

## 0. 基线与最终验证（测试证据原文）

**基线（动手前，绿）：**

```
C:\Go\bin\go.exe build ./...            → 无输出（成功）
C:\Go\bin\go.exe test -count=1 ./internal/server/
    ok  github.com/pocketagent/daemon/internal/server	25.845s
```

server 包用例数：**529 个 Test 函数**（`Select-String '^func Test'` 计数，含顶层测试函数；
子测试展开更多）。基线全绿后才开始改动。

**最终（全部 6 项收敛完成后，绿）：**

```
C:\Go\bin\go.exe vet ./internal/server/
    → 无输出（干净）
C:\Go\bin\go.exe test -count=1 ./internal/server/
    ok  github.com/pocketagent/daemon/internal/server	27.072s
C:\Go\bin\go.exe test -run 'TestCapabilityBitVocabularyMatchesMatrix|TestCapabilityStanceTableMatchesMatrix' -count=1 ./internal/agent/
    ok  github.com/pocketagent/daemon/internal/agent	2.821s
C:\Go\bin\go.exe test -run 'TestGetHostInfoCapabilityBitsAlignedWithControllers' -count=1 -v ./internal/server/
    --- PASS: TestGetHostInfoCapabilityBitsAlignedWithControllers (0.04s)
    PASS
    ok  github.com/pocketagent/daemon/internal/server	3.351s
```

期间每次收敛项完成后复跑 `go build ./...`（覆盖 cmd/、internal/agent 等全模块编译，
任何跨包引用断裂都会在此暴露），最终 `go vet` + 全量测试 + 能力位三道守护测试
全部通过。测试零删除、零跳过、零弱化。

**字节级验收**（脚本 `verify_bytes.py` 逐文件断言）：12 个改动文件全部为合法
UTF-8（无 BOM、无乱码引入）；旧样板计数归零/仅剩 helper 内唯一定义：

| 检查串 | 收敛后计数 | 说明 |
|---|---|---|
| `s.requireThread(` | 18 | 18 个调用点（12 处 RPC 入口 + 3 处变体 + StartTurn 2 处 + CancelTurn） |
| `errNotFound("会话不存在")` | 1(+1 注释引用) | 仅剩 server.go helper 内唯一定义 |
| `s.requireCodex(` | 9 | 9 个调用点 |
| `errUnavailable("电脑上没有可用的 codex")` | 1 | 仅剩 helper（model_ext.go 3 处响应字段填串是另一形状，未动） |
| `s.requireUnknownLog(` | 3 | 3 个调用点 |
| `errNotFound("未知事件诊断台账未启用")` | 1 | 仅剩 helper |
| `storedTextMessages(` | 3 | 3 个调用点（pi / codex rollout / claude 导入） |
| `CreatedAt: at,`（旧转换循环残迹） | 0 | 归零 |
| `appendGitPathArgs(` | 4 | 3 调用点 + 1 定义 |
| `s.defaultWorkspaceCwd(` | 2 | 2 个调用点 |

## 1. 盘点清单（按「行数收益 × 安全度」排序后的执行序）

| # | 收敛项 | 站点数 | 重复正文（约） | 安全度 | 结论 |
|---|---|---|---|---|---|
| 1 | `requireThread`：会话存在性检查样板（`Store.Get` + `errNotFound("会话不存在")`） | 18 | 18×4 行 | 高（纯机械替换，错误值逐字节同源） | ✅ 已做 |
| 2 | `requireCodex`：codex provider 缺席样板（`codexProvider()` + `errUnavailable`） | 9 | 9×4 行 | 高（同上） | ✅ 已做 |
| 3 | `storedTextMessages`：三家导入路径逐字相同的文本消息转换循环（含 `at<=0 → now` 兜底） | 3 | 3×12 行 | 中高（泛型收口，三源类型字段同形已逐一核对） | ✅ 已做 |
| 4 | `requireUnknownLog`：未知事件台账缺席守卫 | 3 | 3×4 行 | 高 | ✅ 已做 |
| 5 | `appendGitPathArgs`：git「路径参数或空路径兜底」拼装 | 3 | 3×7 行 | 高（纯函数） | ✅ 已做 |
| 6 | `defaultWorkspaceCwd`：导入路径工作区三层兜底 | 2 | 2×6 行 | 高 | ✅ 已做 |
| — | host.go 能力位生成表驱动化 | 13 位族 | — | **被契约挡住** | ⛔ 已识别未动（见 §3.1） |
| — | collab_session_state.go 成员卡迁移模板 | 4–5 | ~5×10 行 | 变体差异大 | ⚠️ 已识别未动（见 §3.2） |
| — | 死代码删除 | 8 候选 | — | 全部为 proto 接口方法 | ⛔ 已识别未动（见 §3.3） |
| — | 测试 helper 收敛到 testkit | — | — | 盘点后无真实重复 | ⛔ 已识别未动（见 §3.4） |

盘点方法：`scan_dead.py`（全仓含 cmd/agent 的零引用导出符号扫描）、`scan_dupblocks.py`
（非测试文件 6 行滑窗跨文件重复块检测，已过滤 import 噪声）、人工核对热点文件
（host.go / thread.go / thread_ext.go / turn_queue.go / event_sequence.go /
collab_session_state.go / agents_sessions.go / external_sessions.go / codexsessions.go /
git.go / fs.go / 三个 subagent sync 文件）。

## 2. 逐项记录（等价理由 + 测试结果）

### 收敛项 1：`requireThread`（server.go 新增；改 5 个文件 18 处）

**改动**：server.go 新增：

```go
func (s *Server) requireThread(id string) (*store.Thread, error) {
	t, ok := s.cfg.Store.Get(id)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	return t, nil
}
```

18 处入口统一为 `t, err := s.requireThread(...); if err != nil { return nil, err }`（StartTurn
两处为流式返回 `return err`）。

**等价理由**：
- 同一 `s.cfg.Store.Get(id)` 调用、同一 `errNotFound("会话不存在")`（`status.Error(codes.NotFound, ...)`）
  —— 返回给手机端的状态码与文案逐字节不变；
- 替换点在取到会话后立即返回，无中间副作用，控制流一一对应；
- 变量面：逐点核对无 `ok` 变量的后续依赖（编译期兜底验证，`go build ./...` 全绿；
  StartTurn 第二处 `t, err =` 复用既有 `err`，语义与原 `t, ok =` 对齐）。

**站点**：thread.go ×11（GetThread / ArchiveThread / SetThreadGoal / CompactThread /
CancelTurn / ClearQueuedInputs / RenameThread / GetThreadHistory / StartTurn ×2 /
ExportThreadHtml）、thread_ext.go ×4（MoveThreadToSection / ForkThread(src 变量) /
RollbackThread / ApproveGuardianDeniedAction）、thread_review.go ×1、
slash_commands.go ×1、thread_search.go ×1（其后 `t == nil` 防御检查原样保留）。

**测试**：全量 529 用例绿（`ok ... 27.072s`）。

### 收敛项 2：`requireCodex`（server.go 新增；改 3 个文件 9 处）

**改动**：`p, err := s.requireCodex(); if err != nil { return nil, err }` 取代
`p := s.codexProvider(); if p == nil { return nil, errUnavailable("电脑上没有可用的 codex") }`。

**等价理由**：同一 `codexProvider()` 调用、同一 `errUnavailable`（`codes.Unavailable`）与
逐字文案 —— 手机端"去电脑上配置"的展示语义不变。
**刻意排除**：model_ext.go 的3 处同串站点是**响应字段填串**（`out.Error = ...` /
`SettingStatus`），不是错误返回路径，形状不同，不并入（收敛 helper 注释里已写明）。

**站点**：thread_ext.go ×7（分组三件套 / Move / Fork / Rollback / 推翻驳回，其中 Move
处为 if 块内嵌套缩进单独替换）、thread_review.go ×1、thread.go ×1（CompactThread）。

**测试**：全量绿。

### 收敛项 3：`storedTextMessages` 泛型收口（agents_sessions.go；改 3 个文件）

**背景**：pi 导入（`agent.ReadPiSessionHistory`）、codex rollout（`agent.LoadCodexSessionMessages`）、
Claude 只读导入（`externalconv.ReadClaudeSession`）三条路径各有一段 12 行**逐字节相同**的
转换循环（角色/正文/时间戳 → `store.StoredMessage`，含"时间戳缺失用导入时刻兜底"）。

**等价理由**：
- 三家源消息类型字段逐一核对为同形（`Role string; Text string; At time.Time`：
  agent/pi_sessions.go:46、agent/codexsessions.go:35、externalconv/externalconv.go:60）；
  Go 无法跨包按形状断言结构体，故用泛型 + 三字段提取函数，**提取函数直接返回原字段**，
  零转换逻辑；
- helper 内逐行复刻原循环：同一容量预分配、同一 `at.UnixMilli()`、同一 `<=0 → time.Now().UnixMilli()`
  兜底、同一 `Blocks` 构造 —— 行为逐字等价，且兜底怪癖从 3 份变 1 份。

**测试**：`agents_sessions_test.go` / `external_sessions_test.go` / 分叉路径
（`forkSeedMessages` 消费 codexRolloutMessages）随全量测试绿。

### 收敛项 4：`requireUnknownLog`（unknown_events.go；改 2 个文件 3 处）

**等价理由**：同一 `unknownLog()`、同一 `errNotFound("未知事件诊断台账未启用")`。
**刻意排除**：ListUnknownEvents 的 nil 分支回空清单、controlDomainDialysis 的 nil 分支回
降级面板 —— 都不是错误路径，不并入（helper 注释已写明）。

### 收敛项 5：`appendGitPathArgs`（git.go；3 处）

**等价理由**：纯函数，参数拼装逐字等价 —— 带路径时先加 `--`（防止以 `-` 开头的路径被
git 当旗标解析）再追加路径；空路径走调用方兜底（`add -A` / `restore .`）。
**刻意排除**：Commit 的"有路径才先 add"是条件执行语义（空路径不跑命令），形状不同，
不并入（helper 注释已写明）。

### 收敛项 6：`defaultWorkspaceCwd`（agents_sessions.go；2 处）

**等价理由**：同一取值顺序（`Store.DefaultWorkspace()` 优先，空则 `cfg.Workspace`），
ImportCodexThread 与 ImportAgentThread 两处三层嵌套 if 收成一行。
**刻意排除**：host.go GetHostInfo 的同款两层取值 —— 探活路径保持零改动（见 §3.1 的
保守原则：不碰 host.go 任何一行）。

## 3. 已识别未动（含理由，交主会话/后续裁决）

### 3.1 host.go 能力位生成表驱动化 ⛔（硬契约挡住，且收益为负）

`daemon/internal/agent/capability_gate_test.go` 的 `TestCapabilityBitVocabularyMatchesMatrix`
**直接解析 host.go 源码文本**：逐行扫描 `caps = append(caps, ...)` 的字面量，按
`"xxx:"+id`（位族）/ `agentCapability(id)`（成员位）/ 裸字面量（宿主级位）三分类，
与测试内固化期望表双向对表。表驱动化（`for _, c := range capSpecs { caps = append(caps, c.prefix+id) }`）
会删除这些字面量行 → 词汇表解析得到空位族集合 → 该守护测试必然变红。这不是"位输出变化"
而是**源码形态契约**：该测试的设计初衷就是钉住字面量写法（其注释明言"位名是字面量、
无常量可用"）。故：
- host.go 全文件保持零改动（`git diff` 层面本任务未触碰该文件）；
- 位输出逐位不变由 `TestCapabilityBitVocabularyMatchesMatrix` +
  `TestGetHostInfoCapabilityBitsAlignedWithControllers` 双测绿证明（§0 原文）。
- 移交建议：若将来确要表驱动，需先在 agent 包裁决改守护测试的解析协议（属于 agent 包
  改动，超出本任务边界），矩阵文档与期望表同步动。

### 3.2 collab_session_state.go 成员卡迁移样板 ⚠️（变体差异大，收敛模板有行为风险）

`transitionRunning / finalizeMember / memberApprovalWaiting / clearStalePendingApprovals`
四处共享"retry 事务里定位成员 → 守卫 → 变更 → `deriveCollabSessionStatus`+`applySessionError`
→ 拷卡 → 发 important 事件帧"的骨架（各约 10 行重复），但变体差异实质性：
- 守卫不同（代际闸带日志 / 终局不复活带日志 / 无守卫）；
- finalizeMember 额外捕获 `prevStatus` 做会话级快照补帧；
- clearStalePendingApprovals **刻意不跑** `deriveCollabSessionStatus`/`applySessionError`
  （"状态未变，只摘审批"）—— 把这两行并进模板后，它的"幂等无新事实"语义要靠
  "重算必得同值"的间接论证支撑，该论证依赖 derive/apply 的实现细节，未来一改就静默漂移；
- kind 字段四家各不同（常量 / 状态参数 / waiting 派生 / card 现状）。

收敛成模板要么参数爆炸（4 个开关 + 2 个回调），要么把上面这条不变式变成隐式假设 ——
均不满足"等价优先"的验收标准。**建议**：保持现状；若后续真要动，先给
`deriveCollabSessionStatus`/`applySessionError` 补"重算幂等"的直接测试钉，再收敛。

### 3.3 死代码扫描结果 ⛔：8 个候选全部是对外接口，禁删

`scan_dead.py` 全仓（含 cmd/、internal/agent、internal/gen 之外全部包）扫描导出符号
零引用，得 8 候选：`CheckoutBranch / CreateBranch / DiscardChanges / ListBranches / Unstage`
（git.go）、`ListDirectory / ReadFileDiff`（fs.go）、`SetDefaultModel`（model.go）。
逐一对表 `internal/gen/pocketagent/v1/*_grpc.pb.go`：**全部是 gRPC 服务接口方法**
（如 `FileSystemServiceServer` 接口要求 `ListDirectory`），由 `Unimplemented*Server`
兜底实现 —— 删掉能编译但 RPC 运行时退化为 `Unimplemented`。属任务红线"proto gen 符号
与对外接口禁删"。**结论：server 包无死代码。**（扫描中 `StreamAuth`/`UnaryAuth`/`Commit`/
`CloseTerminal` 曾入候选，复核为 cmd/pocketagentd/main.go 方法值调用与接口实现，已排除；
两轮扫描的教训记入移交建议 —— 方法值引用 `srv.StreamAuth()` 不能用"去点前缀"的正则判死。）

### 3.4 测试 helper 收敛到 testkit ⛔：盘点后无真实重复

- 测试文件函数名两轮去重（顶层 + 方法）**零重名**：`newTestServer`（thread_ext_test.go）
  与 `mustStore`（thread_provider_test.go）各自唯一，是两个用途（起 Server 起步器 vs
  裸 store 起步器），非重复；
- 14 个 `fake*/stub*` 类型逐一过目：fakeGit / fakeDialer / fakeCodexReadFull / fakeZcodeHistory
  等各自钉不同接口形状（provider / dialer / git runner / 历史读取器），无同形可合并；
- sink 相关散布各测试的均为就地 `agent.Sink` 闭包（记录帧序列），行数 1–3 行，抽公共
  testkit 反而增加跳转成本。
**结论**：不存在"mock provider/sink/store 工厂"重复，无需建 testkit；强行收敛属为收敛而收敛。

### 3.5 其他已识别不动项

- **thread.go 1913 行的 StartTurn/runTurnOnce 巨型函数**：是复杂度/长度问题，不是重复问题，
  结构性拆分超出"纯收敛、语义零改变"的授权（拆分必然引入中间变量/回调边界的变化面）。
- **model.go / model_ext.go / model_selection.go 的 provider 遍历**：各处遍历后的产出
  （响应字段、设置状态、错误串）互不相同，无逐字重复块。
- **三个 subagent sync 文件（codex/pi/zcode）**：名字三胞胎但机制完全不同（通知驱动 /
  磁盘扫描 / 轮次终局清单），逐行核对后无共享骨架可收。
- **turn_queue.go 队列清理**：`next()`/`stop()` 的台账出账已带"锁外落盘"注释且刻意分立
  （临界区内禁磁盘 I/O 是 P0 纪律），不并。
- **event_sequence.go 断点续传**：WatchEvents（host.go）与 GetThreadEventsSince 的补发
  逻辑形似但消息类型不同（HostEvent vs TurnServerMessage）、保真策略不同（损坏行跳过 vs
  ResyncRequired 帧），泛型化要跨 pb 消息类型抽象，收益小于风险。

## 4. 改动文件清单（12 个，全部在 daemon/internal/server/ 内）

| 文件 | 行数 前→后 | 内容 |
|---|---|---|
| server.go | 459 → 488 | +`requireThread` / +`requireCodex`（含收敛理由注释） |
| thread.go | 1913 → 1913 | 11 处 requireThread + 1 处 requireCodex（行数中性替换） |
| thread_ext.go | 621 → 621 | 4 处 requireThread + 7 处 requireCodex |
| thread_review.go | 48 → 48 | 1 处 requireThread + 1 处 requireCodex |
| thread_search.go | 285 → 285 | 1 处 requireThread（`t==nil` 防御保留） |
| slash_commands.go | 279 → 279 | 1 处 requireThread |
| agents_sessions.go | 328 → 353 | +`storedTextMessages` / +`defaultWorkspaceCwd`；ReadImportHistory 换用 |
| codexsessions.go | 199 → 186 | codexRolloutMessages 换用转换 helper；ImportCodexThread 换用兜底 helper |
| external_sessions.go | 127 → 118 | claude ReadImportHistory 换用转换 helper |
| unknown_events.go | 221 → 234 | +`requireUnknownLog`；Get/Acknowledge 换用 |
| unknown_actions.go | 428 → 428 | ExecuteUnknownEventAction 换用 requireUnknownLog |
| git.go | 282 → 276 | +`appendGitPathArgs`；Stage/Unstage/DiscardChanges 换用 |

净行数 +39：新增 ~90 行带收敛理由注释的 helper，替换掉 ~150 行分散重复正文；
**收益不主要在行数，而在单源化** —— 6 类错误文案/取数样板从 35 个分散点收成 6 个定义点。

未触碰：host.go（零 diff）、全部 `*_test.go`（零 diff，无删除/跳过/弱化）、
proto/、cmd/、flutter/、android/、internal/ 其他包。

## 5. 移交建议

1. **提交**：12 个文件一次提交即可，建议信息方向 `refactor(server): RPC 入口样板单源化——requireThread/requireCodex/未知事件台账/导入消息转换/git 路径拼装/工作区兜底六类收口`。提交前复跑 §0 三条命令即可验收。
2. **并发注意**：本任务执行期间另一子代理在改 internal/agent 与 cmd/ —— server 包公共 API 未变（仅新增方法），但合并前建议在合并后的树上重跑 `go build ./... && go test ./internal/server/ ./internal/agent/`。
3. **判死代码的正则教训**：`(?<![\w.])` 会漏掉 `srv.StreamAuth()` 方法值引用；判"导出符号死活"必须把点前缀调用算作引用，否则会误删装配代码（本轮曾误报 11 个，其中 3 个即方法值/接口实现）。
4. **后续若做 host.go 表驱动**：先与 agent 包守护测试的"源码文本解析"契约一起设计（等价证明方式要从"位输出不变"升级为"词汇表解析结果不变"），并同步矩阵文档 —— 单动 server 侧必然红。
5. **collab 模板收敛的前置**：给 `deriveCollabSessionStatus`/`applySessionError` 补重算幂等钉后再动（§3.2）。
