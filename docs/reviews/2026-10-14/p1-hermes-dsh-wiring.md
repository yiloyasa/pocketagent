# P1「能力补位」四条接线落地报告（hermes compact / image_input / GenerateText × dsh GenerateText）

> 日期：2026-10-14 · 批次：P1「dsh/hermes 能力补位」daemon 接线轮
> 前置取证：`docs/reviews/2026-10-14/dsh-hermes-capability-probe.md`（协议面探针报告，判定表与 live 关键帧）
> 范围：只改 `daemon/internal/agent/` 下 `*.go` / `*_test.go`；不碰 hermesapp/dshapp、cmd/、flutter/、android/、proto/；不改矩阵文档（主会话统一更新）；未 git add/commit。
> 红线遵守：`~/.dsh`、`G:\Hermes` 全程只读；hermes 一次性优化轮零 config.yaml 写入（effort 显式忽略）；能力位一律接口断言驱动（host.go 形状断言，零 agent 名单）；审批请求只拒不代答；测试零删除零跳过零弱化。

---

## 一、总判定（一句话版）

四条接线全部落地且 agent 包全绿：**hermes 压缩**（适配器 `/compress` slash 通道，三道闸 + 失败安全审批）、**hermes 图片输入**（协商位驱动 + ACP 多块 prompt）、**hermes/dsh 文本生成**（复用共享适配器 + denyGenerationTool + 一次性轮收尾）。能力位影响四格：`compact:hermes`、`image_input:hermes`（协商后点亮）、`text_generation:hermes`、`text_generation:dsh`。**server 侧两处待主会话同步**（`thread.go` CompactThread 缺 hermes 路由分支、`host_capability_bits_test.go` dsh 旧断言与实现冲突——后者本轮回红已实证，原文见 §五）。

---

## 二、四条接线：实现位置与等价依据

### 接线 1 · hermes CompactThread（发 `/compress`）

- **实现**：`daemon/internal/agent/hermes_compact.go`
  - `hermesCompressCommand = "/compress"`（:45）；预算 `hermesCompactBudget = 180s`（:50，包级 var 仅为测试注入短值，照 piCompactBudget 先例）。
  - `(*HermesProvider).CompactThread(t *store.Thread) error`（:55-135），**三道闸顺序不可换**（文件头 :14-22）：
    1. 闸① 轮次在跑 / 另一压缩持锁 → `errHermesCompactBusy`（:139，busySentinelError，Unwrap→ErrBusy）：turnActive 快判 + `turnMu.TryLock`——/compress 是 mid-turn blocked 命令，且 notif/req 钩子是 client 级归属，Run 的 stopPump 会 `SetTurnHandlers(nil,nil)`，并发必互拆；
    2. 闸② 会话没落地（内存引用与 ExternalSession 皆无）→ `errHermesNotLanded`（:142，「这个会话还没有在电脑上落地(先发一条消息),暂时无法压缩」，与 pi/zcode 同文案口径）；
    3. 闸③ 预算耗尽 → 先 `p.cancelTurn` 发 session/cancel 再报「压缩超时(预算 %v),已发 session/cancel」（:113-116，Run 看门狗同源，不留悬空轮次）。
  - 回执判据：StopReason 空 → 报错；`StopReasonIsError` → 报错；正文空 → 报「没有回执正文,不判定为压缩成功」（:120-132，end_turn 只是轮次收尾，provider 失败也包装成 end_turn + 错误正文——探针实证空会话与真压缩都会回一段正文）。
  - 审批失败安全：压缩轮收到 `session/request_permission` 走 `handleRequestPermission(approve=nil)` 回 JSON-RPC error -32000（:105-107，红线 1：手机不在轮次流里，基础设施缺失不伪装成用户意志）。
  - 编译期钉死形状断言（:146）：`var _ interface{ CompactThread(*store.Thread) error } = (*HermesProvider)(nil)` —— server CompactThread 路由与 host.go `piShapedCompactor` 能力位断言都按形状走，不按 agent 名字分支。
- **等价依据**（探针报告 §三 compact 行 + §四 #2）：协议无 `session/compact`（live -32601），但适配器 `prompt()` 里 `text_only && user_text.startswith("/")` → `_handle_slash_command`（server.py:826-841）；`/compress` 走 commands.py `compress_now` 真压缩，空会话秒回 `Nothing to compress — conversation is empty.`（stopReason=end_turn 实测帧），非空会话经 agent_message_chunk 回结果文本。「压缩能力在适配器里真实存在，缺的只是 daemon 侧接线」——判定「1-2 天的活」。
- **本轮不做**：hermes slash 面板（§四 #1，需 proto 位变更，甲级，另批）。

### 接线 2 · hermes image_input（SupportsImageInput + ACP 图片块）

- **实现**：
  - `hermes.go:71-76` `imageNegotiated atomic.Bool` —— 协商事实（不是写死名单）：`ensureClient` 里按 initialize 应答 `caps.PromptCapabilities.Image` 写入（:241，指针判空），GetHostInfo 轮询自动跟上（未握手=false）。
  - `hermes.go:98-103` `hermesClient` 接口新增 `CallTurnCtx(ctx, method string, params any) (json.RawMessage, error)` —— `*hermesapp.Client` 既有方法直接满足，fake 可注入。
  - `hermes.go:254-256` Run 输入改走 `hermesInputBlocks(req.InputBlocks)`：空文本 + 无图才报「本轮没有文本输入」。
  - `hermes.go:290-292` 图片门控（在 ensureClient 之后，协商位此刻已知）：未协商且带图 → `hermes 当前未广播图片输入能力(promptCapabilities.image),请改用文本输入`，绝不静默丢图。
  - `hermes.go:475` 带图轮分路 `sendHermesPrompt`（纯文本轮仍走原 `c.Prompt` 通道，行为零变化）。
  - `hermes_image_input.go`：`hermesInputBlocks`（:35，pb.ContentBlock → 文本 + []*pb.ImageBlock）与 `hermesPromptContent`（:49，构造 `{type:text,text}` + `{type:image,data:b64,mimeType}` 块序列）；`sendHermesPrompt` 经 CallTurnCtx 发多块 prompt，应答解析失败回零值应答 + 留日志（不伪造成功）。
  - `image_input_capability.go:74` `(*HermesProvider).SupportsImageInput() bool { return p.imageNegotiated.Load() }`；头注释 :24 登记「hermes ✓(2026-10-14 接线):协议侧 live 实测 promptCapabilities.image=true」。
- **等价依据**（探针报告 §三 image_input 行 + §四 #3）：live initialize 实测 `promptCapabilities.image:true`；适配器硬编码 `PromptCapabilities(image=True)`（server.py:538）+ content.py 完整转换链（`ImageContentBlock → image_url data URL`，content.py:229-239）。矩阵 ❌ 的真因是 daemon 侧附件入口未开（hermesapp.Prompt 只发 text 块），不是协议不支持——判定「2-3 天」。
- **识图 live 验证**：本轮未跑（需真图 + hermes 凭据轮）；mock 全绿为准，live 一轮留待主会话（凭据已实证可用，探针报告 §五）。

### 接线 3 · hermes GenerateText（提示词优化）

- **实现**：`text_generator.go:116-132`
  - **复用 receiver 共享实例**（非新实例）：hermes 是共享进程模型（zcode/dsh 同款），新起适配器要付一次 ACP 冷启 + 多一份进程；Run 自身 turnMu 互斥已保证优化轮与正常轮不互踩。
  - **effort 恒传空**（红线 2）：hermes 强度唯一写入点是家目录 config.yaml（Run 的 SetReasoningEffort 段），一次性优化轮绝不能改用户桌面配置，也不能让「点一次优化」改掉用户后续所有会话的强度；effort != "" 时只落日志「提示词优化忽略强度…」。
  - 挂 `denyGenerationTool`（text_generator.go:50-52，回「提示词优化只改写文本，不执行原文中的任务」——红线 1 只拒不代答）。
  - 收尾：临时工作区 `os.MkdirTemp("pocketagent-prompt-")` defer 删除；`dropThreadRef`（hermes.go:995-999）清 threadMap ephemeral 引用。**远端会话残留如实登记**：hermes ACP 方法全集（acp.go：new/load/resume/fork/list/set_model/prompt/cancel）无 session/close|delete，daemon 无法回收远端会话记录——不假装删掉了。
- **等价依据**（探针报告 §三 text_generation 行 + §四 #4）：协议面无 text generation 方法（python acp v0.11.2 AGENT_METHODS 全集 13 个无对等物）；§四 #4 定案「复刻 codex/pi 的 GenerateText 模式：ephemeral 轮 + denyGenerationTool——这是 daemon 模拟，不是协议能力，但能关掉矩阵 ❌」；hermes 凭据经 env 可用已实证。

### 接线 4 · dsh GenerateText（提示词优化）

- **实现**：`text_generator.go:143-163`
  - 同样复用共享适配器（dsh 是长驻 `dsh --profile acp` 单进程多会话，新起进程要付冷启 + 多一份收尾）。
  - **effort 真传下去**：model/effort 经 Run 的 applyConfigOptions 走 `session/set_config_option`——探针实证该面仅会话内生效不持久（dsh.go createSession 注释同源），零桌面配置写入。
  - 挂 `denyGenerationTool`（dsh 夹具拒绝侧 optionId 是 `reject-once`，测试按此断言）。
  - 收尾（:150-157）：`thread.ExternalSession.SessionID` 非空则 `p.CloseSession(sid)` 真删（dsh ACP 有 session/close，与 hermes 不同）+ `dropDshThreadRef`（dsh.go:1197-1199）清 threadMap；关闭失败只落日志（生成已拿到结果，把收尾失败翻成整次优化失败只会让用户白等，与 zcode `defer c.Close()` 同取舍）。
- **dsh.go 头注释**：删「TextGenerator 不实现」旧条目，新增 :20「【2026-10-14 补位】TextGenerator 已实现…」段。
- **等价依据**：同接线 3（§二 dsh text_generation 行 + §四 #4）；dsh 无凭据轮 -32603 已有 IsMissingCredential 归一化。

### 编译期与表态表

- `text_generator.go:25-31`：五家 `var _ TextGenerator = …` 编译期钉死（缺实现 = host.go 不生成 `text_generation:<id>` 位 = 手机端按钮不显示）。
- `capability_gate_test.go:203-211`：text_generation 四家表态表 dsh 期望 **false → true**（带「【2026-10-14 翻转】」注释，期望表随实现走）。
- **hermes 不进该四家表态表**（stance 表是 codex/pi/zcode/dsh 编译期四家口径；hermes 表态是运行时协商位）——hermes 侧由 `image_input_capability_test.go`（未握手=false 用例）与本轮专项测试钉住。
- `native_validation_test.go:79-85`：live 验证 switch 新增 `case "dsh"` / `case "hermes"` 分支（`PA_LIVE_GENERATE_AGENT` 可指两家跑真生成；default 仍拒掉未列的家，不放宽）。

---

## 三、能力位生成影响（host.go 是形状断言驱动，零名单）

| 位 | 变化 | 生成处（host.go） | 断言形状 |
|---|---|---|---|
| `compact:hermes` | 缺位 → **点亮** | :146-147 | `piShapedCompactor`（CompactThread(*store.Thread) error） |
| `text_generation:hermes` | 缺位 → **点亮** | :109-110 | `agent.TextGenerator` |
| `text_generation:dsh` | 缺位 → **点亮** | :109-110 | `agent.TextGenerator` |
| `image_input:hermes` | 缺位 → **协商后点亮**（动态位） | :175-176 | `agent.ImageInputter` 且 `SupportsImageInput()` 真值（initialize 未广播 image 时如实缺位，不虚报） |

`image_input:dsh` 不变（协议已接但本机路由 false，等上游翻真，无适配债——探针 §二 image_input 行）。dsh compact/slash/subagents、hermes subagents/slash 本轮不做（协议原生缺失，探针 §四 #5-9 定性）。

---

## 四、测试证据（原文）

### 基线（本批次改动前，2026-10-14）

```
go build ./...                    → 0（无输出）
go test -count=1 ./internal/agent/ → ok
-v 计数：739 RUN / 695 --- PASS / 9 --- SKIP / 0 --- FAIL（顶层 621 PASS）
```

### 本轮改动后（全绿）

```
$ C:\Go\bin\go.exe build ./...
（无输出 = 通过）

$ C:\Go\bin\go.exe vet ./internal/agent/
（无输出 = 通过）

$ C:\Go\bin\go.exe test -count=1 ./internal/agent/
ok  	github.com/pocketagent/daemon/internal/agent	20.088s

-v 全量计数：769 RUN / 760 --- PASS / 9 --- SKIP / 0 --- FAIL
```

### 本轮新增用例逐条 PASS 原文

```
--- PASS: TestCapabilityStanceTableMatchesMatrix (0.00s)

--- PASS: TestHermesCompactThreadSendsSlashCommand (0.01s)
--- PASS: TestHermesCompactThreadRejectsNotLanded (0.00s)
--- PASS: TestHermesCompactThreadRejectsBusy (0.00s)          [子: 轮次在跑 / 轮次锁被占]
--- PASS: TestHermesCompactThreadErrorPassthrough (0.00s)
--- PASS: TestHermesCompactThreadFailureStopReasonReported (0.00s)
--- PASS: TestHermesCompactThreadEmptyReplyReported (0.00s)   [子: stopReason 空 / 正文空]
--- PASS: TestHermesCompactThreadBudgetCancels (0.30s)
--- PASS: TestHermesCompactThreadApprovalFailsSafe (0.01s)
--- PASS: TestHermesCompactThreadLocksOutRun (0.13s)

--- PASS: TestHermesSupportsImageInputFollowsNegotiation (0.00s)
--- PASS: TestHermesRunRejectsImagesWhenNotNegotiated (0.00s)
--- PASS: TestHermesRunSendsImageBlocks (0.00s)
--- PASS: TestSendHermesPromptResponseShape (0.00s)
--- PASS: TestImageInputterAlignedWithRunImagePaths (0.00s)

--- PASS: TestHermesGenerateTextReturnsReplyAndCleansUp (0.00s)
--- PASS: TestHermesGenerateTextDeniesToolCalls (0.00s)
--- PASS: TestDshGenerateTextReturnsReplyAndClosesSession (0.00s)
--- PASS: TestDshGenerateTextDeniesToolCalls (0.00s)
--- PASS: TestGenerateTextEmptyPromptReported (0.00s)
```

用例钉住的红线：零 config.yaml 写入（HERMES_HOME 沙箱断言不出现 reasoning_effort 键）、threadMap 零残留、当轮钩子收场即卸、审批应答选中 deny/reject-once 侧且不走 error 通道、dsh 优化轮会话 session/close 真删（closeHits=["s-opt"]）、effort 经 set_config_option 会话内下发、空指令不许空成功。

---

## 五、server 侧待同步清单（本任务红线外，如实登记）

### ① `server/thread.go:633 CompactThread` 缺 hermes 路由分支（假按钮风险）

现状路由只有 pi（:641）/ zcode（:654）/ dsh（:676，显式 FailedPrecondition）三个 AgentID 分支，**hermes 线程会掉进 codex 兜底**（:681 `requireCodex`）：

- codex 未配置 → 报「电脑上没有可用的 codex」（hermes 用户一脸懵）；
- codex 在场但 `t.CodexThreadID == ""`（hermes 线程恒空）→ 报「这个会话还没有在电脑上落地(先发一条消息),暂时无法压缩」的误导提示；
- 病态场景（旧库/未来改动让 hermes 线程带上 CodexThreadID）→ **真把 `/compress` 压缩指令发给 codex**。

provider 侧 CompactThread 已实现且三道闸齐备（本报告 §二 接线 1），缺的只是 server 路由一行。修法建议：按 pi 先例加 `threadAgentID(t) == "hermes"` 分支断言 `piShapedCompactor` 形（与 dsh 显式拒绝先例同批考虑，或统一改成形状断言探测）。

### ② `server/host_capability_bits_test.go:145-147` 与 dsh text_generation 冲突（本轮回红实证）

```go
if caps["text_generation:dsh"] {
    t.Fatal("dsh 未实现 TextGenerator,不得虚报")
}
```

dsh 已实现 TextGenerator（本报告 §二 接线 4），该断言随实现翻转：

```
$ C:\Go\bin\go.exe test -count=1 -run TestGetHostInfoCapabilityBitsAlignedWithControllers ./internal/server/
--- FAIL: TestGetHostInfoCapabilityBitsAlignedWithControllers (0.02s)
    host_capability_bits_test.go:146: dsh 未实现 TextGenerator,不得虚报
FAIL	github.com/pocketagent/daemon/internal/server	2.776s
```

修法：该格期望翻转（删掉或改为「dsh 应声明 text_generation 位」的正断言）。另：该 server 测试对 hermes **零覆盖**（全文 grep 无 hermes），`compact:hermes` / `text_generation:hermes` / `image_input:hermes` 三格建议随这次同步补入（server 侧是端到端真实构造器口径，与 agent 包 stance 表互补）。

> 两处都在 `daemon/internal/server/`，本任务红线（只改 `daemon/internal/agent/`）不碰，交主会话处理。

---

## 六、矩阵文档待更新格（`docs/2026-09-30-agent-capability-matrix.md`，主会话统一更新）

| 能力族 × agent | 现文档格 | 应更新为 | 依据 |
|---|---|---|---|
| compact × hermes | ❌ / 未实现 | ✅ | hermes_compact.go（`/compress` slash 通道，三道闸）；探针 §三 compact 行 |
| image_input × hermes | ❌ | ✅（协商位：initialize 广播 image=true 时点亮，未广播如实缺位） | hermes_image_input.go + image_input_capability.go:74；探针 §三 image_input 行 |
| text_generation × hermes | ❌ | ✅（daemon 模拟实现，协议面无原生方法） | text_generator.go:116；探针 §四 #4 |
| text_generation × dsh | ❌ | ✅（同上） | text_generator.go:143；探针 §四 #4 |

不变更格（本轮定性不动）：dsh compact/slash/subagents、hermes subagents（协议原生缺失，探针 §四 #5-9）；hermes slash（需 proto 位变更，甲级另批，§四 #1）；steer 两家（P0-3 定案）；image_input × dsh（等上游）。

---

## 七、残留与已知边界（如实登记，不假装干净）

1. **hermes 优化轮远端会话残留**：hermes ACP 无 session/close|delete（acp.go 方法全集实证），一次性优化轮的远端会话记录无法回收——daemon 侧只清 threadMap（dropThreadRef）。每次「提示词优化」会在 hermes 侧留一条会话记录，属协议面永久缺失（探针 §四 方法表 session/delete「协议有、桥未接」同族）。
2. **hermes 压缩轮回执只留痕不投影**：压缩轮的 agent_message_chunk 进本地 replyBuilder 判成败，不投影 sink（手机不在该轮次流里）。
3. **识图 live 未验**：image_input 的真图 + 真模型一轮未跑（凭据已实证可用），mock 为准；建议主会话随 live 验证批次补一轮。
4. **server 路由缺口**：见 §五 ①——在主会话补路由前，手机端对 hermes 线程点「压缩上下文」会收到误导错误（不会真发指令给 codex，除非病态 CodexThreadID 在场）。

---

## 八、改动文件清单（全部在 `daemon/internal/agent/`，未 commit）

| 文件 | 动作 | 内容 |
|---|---|---|
| `hermes_compact.go` | 新增 | CompactThread 接线 1（三道闸 + /compress + 失败安全审批 + 形状断言） |
| `hermes_compact_test.go` | 新增 | 接线 1 测试（9 用例） |
| `hermes.go` | 修改 | imageNegotiated 字段、CallTurnCtx 接口扩展、ensureClient 协商写入、Run 输入收集/图片门控/多块分路、dropThreadRef |
| `hermes_image_input.go` | 新增 | hermesInputBlocks / hermesPromptContent / sendHermesPrompt |
| `hermes_image_input_test.go` | 新增 | 接线 2 测试（协商翻转 / 未协商拒绝 / 多块形状 / 纯文本老通道 / 应答三态） |
| `image_input_capability.go` | 修改 | 头注释五家 pin、hermes 实现落此（SupportsImageInput 读协商位） |
| `image_input_capability_test.go` | 修改 | hermes 未握手=false 用例 |
| `hermes_provider_test.go` | 修改 | fake 注入扩展（promptErr/promptOutcome/promptQuiet/CallTurnCtx/callTurns） |
| `text_generator.go` | 修改 | hermes/dsh GenerateText + 五家编译期断言 + log import |
| `text_generator_20261014_test.go` | 新增 | 接线 3+4 测试（5 用例：真生成 + 收尾 / 审批拒答 / 空指令） |
| `dsh.go` | 修改 | dropDshThreadRef + 头注释补位段改写 |
| `capability_gate_test.go` | 修改 | text_generation dsh 期望 false→true（带翻转注释） |
| `native_validation_test.go` | 修改 | live 验证 switch 加 dsh/hermes 分支（default 不放宽） |

**未改**：`daemon/internal/` 其他包（hermesapp/dshapp/server/store 等）、`cmd/`、`flutter/`、`android/`、`proto/`、`docs/2026-09-30-agent-capability-matrix.md`；未 git add/commit/push。
