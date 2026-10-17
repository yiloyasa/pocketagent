# fix-agent-small-debts —— agent 核三条小债收口（P1-1 / P1-2）

- 日期：2026-10-17
- 范围：`daemon/internal/agent/`（codex.go / zcode.go / 新增 1 个实现文件 + 2 个测试文件）
- 纪律：未 commit / 未 push；未碰 `server/`、`hermesapp/`、`proto/`、`android/`、`flutter/`、`tools/`；未跑任何格式化写操作（`gofmt -w` 一次都没执行，格式是手改的）
- 基线锚点：`main` @ `d740272`（与审查报告同一基线）

---

## 0. 三条债的复核结论（先看这里）

| # | 审查报告结论 | 复核结论 | 本次处置 |
|---|---|---|---|
| P1-2 | codex 看门狗注释写 90 秒、实值 150 秒；dsh/hermes/zcode 都写「对齐 codex」 | **成立**，且比报告更严重一点：被对齐的不止三家，**pi 也是**（`pi.go:77/79` 同值 150s/60min，`pi.go:42` 注释「对齐 codex 的同名纪律」） | 注释改成与实值一致 + 三个裸字面量提成具名常量（**实值一个字节没动**） |
| P1-1 | `ErrorInfoCarrier` 5 家里 3 家落面，zcode/hermes 缺席；建议「先加编译期断言」 | **成立**；但「先加断言」这条**不可行**（zcode 尚未实现，加断言立刻编译失败）→ 按指令改为**实现它**，实现后断言才有意义 | 为 zcode 实现 `ErrorInfoCarrier`（新增 `zcode_errors.go` + 接线 9 个失败出口 + 镜像断言） |
| ③ | 测试水位基线「顶层 667（PASS 657 / SKIP 10）」 | 成立（同口径），但该计数法**有编码粘连失真**，见 §4.1 | 新增 11 条顶层用例；既有断言一条未删未改（名字集合比对：缺失 = ∅） |

---

## 1. P1-2｜基准源自己写错了（codex 三闸裸字面量 + 谎言注释）

### 1.1 证据（复核原文）

**注释说 90 秒，代码与文案都是 150 秒**（`codex.go`，改动前行号）：

```
1737: // 4) 等结束。不能只干等:codex 对未知 threadId 会静默吞掉 turn/start
1738: // (无报错、无事件),90 秒收不到任何通知就判失败,让手机端看到明确报错。
1739: tick := time.NewTicker(10 * time.Second)
...
1763: if ms := lastNotif.Load(); ms >= 0 && time.Since(turnStartRef).Milliseconds()-ms > 150_000 {
1769/1771: fmt.Errorf("codex 150 秒无任何事件(任务可能未被接收,thread=%s)", codexTID)
1774: if time.Since(turnStart) > 60*time.Minute {
```

即：注释 90s ／ 判定 150_000ms ／ 用户可见文案 150 秒，**三份真值互相矛盾**。

**被对齐方（都写着「对齐 codex」）**：

| 位置 | 常量 | 值 | 注释原文 |
|---|---|---|---|
| `dsh.go:52-53` | `dshTurnCap` / `dshSilenceWatchdog` | 60min / 150s | 单轮上限(对齐 codex) / 无任何事件的看门狗(对齐 codex) |
| `hermes.go:603-605` | `hermesTurnCap` / `hermesSilenceWatchdog` / `hermesWatchdogTick` | 60min / 150s / 10s | (对齐 codex/dsh) |
| `pi.go:42,77,79` | `piSilenceWatchdog` / `piTurnCap` | 150s / 60min | 「对齐 codex 的同名纪律」/ 口径与 codex/dsh/zcode/hermes 一致 |
| `zcode.go:65` | `zcodeWatchdogSilence` | 150s | (与 codex 的 150s 同口径) |

结论：**「对齐 codex」共四家在指认一个连自己注释都是假的基准源**。下一个按注释把 codex 调到 90s，四家一起漂。

### 1.2 改动前 → 改动后

**（a）三个裸字面量 → 具名常量**（新增 `codex.go:394-418` 的 const 块，紧邻既有的 `clientInitBudget` / `statusBudget` 预算区，命名跟随同文件既有风格 `codex*` + 全家同名口径）：

```go
// 改动前（Run 内三处裸字面量）
tick := time.NewTicker(10 * time.Second)
if ms := lastNotif.Load(); ms >= 0 && time.Since(turnStartRef).Milliseconds()-ms > 150_000 { … }
if time.Since(turnStart) > 60*time.Minute { … }

// 改动后（真值只有这一份）
const (
	codexWatchdogTick      = 10 * time.Second
	codexSilenceWatchdog   = 150 * time.Second
	codexTurnCap           = 60 * time.Minute
)
tick := time.NewTicker(codexWatchdogTick)                                        // :1770
if ms := … > codexSilenceWatchdog.Milliseconds() { … }                            // :1794
if time.Since(turnStart) > codexTurnCap { … }                                    // :1805
```

**实值逐位等价（零行为变更，可自证）**：
- `150_000`（ms）≡ `codexSilenceWatchdog.Milliseconds()` = 150000 ms —— 单位换算由 `TestCodexWatchdogConstDefaults` 单独钉住；
- `60*time.Minute` ≡ `codexTurnCap`；`10*time.Second` ≡ `codexWatchdogTick`。

**（b）谎言注释改成与实值一致**（`codex.go:1764-1769`）：

```go
// 改动前:(无报错、无事件),90 秒收不到任何通知就判失败,让手机端看到明确报错。
// 改动后:(无报错、无事件),codexSilenceWatchdog(150 秒)收不到任何通知就判失败,
//         让手机端看到明确报错。【P1-2】此前此注释写的是"90 秒",实值一直是 150 秒
//         —— 注释与代码互相矛盾,而 dsh/hermes/pi/zcode 四家都拿这里当"对齐"基准。
```

**（c）用户可见文案一个字没动**：`"codex 150 秒无任何事件…"` / `"codex 超时(60 分钟)"` 保持字面量（**故意不用常量插值**：`fmt.Sprintf("%v", 150*time.Second)` 会渲染成 `2m30s`，那是改文案＝改用户可见行为，属另一次改动）。这条取舍由测试守（§4.3 ②）。

---

## 2. P1-1｜`ErrorInfoCarrier`：为 zcode 落面（hermes 单列判定见 §2.5）

### 2.1 复核：接口、消费点、三家现状

```go
// provider.go:35-39
type ErrorInfoCarrier interface { ErrorInfo() *pb.Error }

// server/thread.go:1518-1523（唯一消费点，类型断言）
failedError := &pb.Error{Code: "AGENT_ERROR", Message: runErr.Error()}
if carrier, ok := runErr.(agent.ErrorInfoCarrier); ok {
	if info := carrier.ErrorInfo(); info != nil { failedError = info }
}
```

| provider | 结构化错误类型 | 形态 | 镜像断言 |
|---|---|---|---|
| codex | `codexRunError`（`codex_events.go:25/36`） | 上游 error.codexErrorInfo → code/httpStatus/url/details | 无 |
| pi | `piError`（`pi.go:2047/2054`） | `{code,msg}` → `{Code,Message}` | 无 |
| dsh | `dshRunError`（`dsh.go:1766/1778`） | MISSING_CREDENTIAL / DSH_ERROR | `dsh.go:121` 有 |
| **zcode** | **本次新增 `zcodeRunError`（`zcode_errors.go`）** | 8 分类，见 §2.3 | `zcode_errors.go:114` |
| **hermes** | 无 | — | —（见 §2.5） |

断言失败的后果：手机端只看到 `AGENT_ERROR` + 一句裸字符串，且**编译期完全无感**。

### 2.2 改动前 → 改动后（zcode.go 接线，9 个失败出口）

```go
// 改动前（全部裸错误）                                    // 改动后（全部结构化）
if err := p.withClient(…); err != nil {                  if err := p.withClient(…); err != nil {
	return nil, nil, err                                     return nil, nil, zcodeRunErrorFrom(err)   // :1830 忙哨兵/子进程未启动/拉起失败
}                                                        }
sid, …, err := p.ensureSession(ctx, req)                 sid, …, err := p.ensureSession(ctx, req)
if err != nil { return nil, nil, err }                    if err != nil { return nil, nil, zcodeRunErrorFrom(err) }   // :1853
…applyZcodeNativeMode…  return nil, nil, err              …return nil, nil, zcodeRunErrorFrom(err)    // :1858
…applyModel…            return nil, nil, err              …return nil, nil, zcodeRunErrorFrom(err)    // :1873
…CallCtx("session/send") fmt.Errorf("session/send: %w")  …zcodeRunErrorFrom(fmt.Errorf("session/send: %w", err))  // :2125
err := fmt.Errorf("zcode %s 无任何事件…")                  zcodeErrorf(zcodeErrWatchdogSilent, true, …, "zcode %s 无任何事件…")  // :2163/:2166 各一份
return nil, nil, fmt.Errorf("zcode 超时(60 分钟)")         return nil, nil, zcodeErrorf(zcodeErrTurnTimeout, false, …, "zcode 超时(60 分钟)")  // :2173
if lastErr != "" { return …, fmt.Errorf("zcode: %s", lastErr) }   if lastErr != "" { return …, zcodeTurnFailedError(lastErr, lastErrDetail) }  // :2192
```

**唯一不包装的出口**：`ctx.Err()`（`:2145/:2147`）—— 手机端「停止任务」/断连走 ctx 取消，不是 provider 失败（codex/dsh/hermes 同款；server 依 ctx 判用户停止）。

**turn.failed 上游细节**（此前被 `mapZcodeTurnFailed` 丢掉）：

```go
// 改动前：只有人话
lastErr = msg                                   // msg = mapZcodeTurnFailed(payload)
// 改动后：细节原样带走（只进 Details，不进 Message）
lastErr = msg
lastErrDetail = zcodeTurnFailedDetail(ev.Payload)   // errorType=… errorCode=… turnPhase=…
```

`mapZcodeTurnFailed` 的**签名与行为一字未动**（既有 `TestMapZcodeTurnFailed` 不受影响，另加 `TestZcodeTurnFailedMappingUnchanged` 钉文案）。

### 2.3 字段映射表（逐形态；每条都有代码事实依据，不是猜）

| 形态 | 触发点 | `Code` | `Retryable` | `Message`（逐字节沿用旧文案） | `Details` |
|---|---|---|---|---|---|
| 忙哨兵 / 锁窗口耗尽 | `withClientCtx` TryLock+排队失败（`zcode.go:382`） | `ZCODE_BUSY` | **true** | `zcode 正忙(另一个操作正在初始化/探测),请稍后重试` | — |
| 子进程未启动 / 启动闪退重试后仍死 | `zcode.go:434-448`、`errNoZcodeClient` | `ZCODE_NOT_STARTED` | **true** | `ZCode 子进程未启动` | — |
| 看门狗静默 | `zcode.go:2163/2166` | `ZCODE_WATCHDOG_SILENCE` | **true** | `zcode 2m30s 无任何事件(任务可能未被接收,session=…)`（沿用旧 `%s` 渲染） | — |
| 单轮超限 | `zcode.go:2173` | `ZCODE_TURN_TIMEOUT` | false | `zcode 超时(60 分钟)` | — |
| turn.failed | 事件 `turn.failed` | `ZCODE_TURN_FAILED` | false | `zcode: <上游 message>` | `errorType=api_error errorCode=-32002 turnPhase=awaiting_model` |
| turn.completed 但非成功 | `zcodeTurnCompletedIsSuccess==false` | `ZCODE_TURN_FAILED` | false | `zcode: 轮次未成功结束(resultType=…)` | `resultType=…` |
| 上游 JSON-RPC 报错 | `session/send` 等 CallCtx | `ZCODE_RPC_ERROR` | false | `session/send: ZCode 返回错误 -32004: …` | `jsonRpcCode=-32004 data=…` |
| 桌面端账号同步失败 | `zcodeapp.AccountSyncError` | `ZCODE_ACCOUNT_SYNC` | false | `…ZCode desktop account sync: …` | 账号那句原文 |
| 兜底（认不出的形状） | 任意 | `ZCODE_ERROR` | false | 原错误文本 | — |

**Retryable 的依据（为什么只有前三个 true）**
- 忙：仓内既有的「可重试态」分级就是 ErrBusy（`codex.go:360 modelsListErr` → `ErrModelsInitializing`；`server/model_ext.go:253`），zcode 的忙哨兵 `Unwrap` 到同一 ErrBusy，语义同源。
- 未启动：下一次调用会重走 `withClientCtx` 的拉起/重启路径（`zcode.go:418/457`）。
- 看门狗静默：判定文案本身就写着「任务可能未被接收」，重发可再试。
- 其余一律 false：**没有实测依据就不承诺可重试**（宁可少承诺不说谎）。账号同步那条有硬依据——`zcodeapp/accounts.go:42-43` 明写 “callers must not restart an agent or create a replacement conversation”。

**Unwrap 保留分级链**：`errors.Is(err, ErrBusy)` / `errors.Is(err, errNoZcodeClient)` / `errors.As(*zcodeapp.RPCError)` 在包装后仍为真（server 与 codex 侧既有的 busy 分级不会因本次改动回归）。

### 2.4 消费点确认（已核对 `server/thread.go`，无需改动）

```go
failedError := &pb.Error{Code: "AGENT_ERROR", Message: runErr.Error()}
if carrier, ok := runErr.(agent.ErrorInfoCarrier); ok { if info := carrier.ErrorInfo(); info != nil { failedError = info } }
```
- 断言成功 → `failedError` 被**整体替换**为结构化错误；`info == nil` 时仍走原兜底（zcode 侧零值路径已按 codex/dsh 同款返回 nil，不会造出假结构）。
- `t.LastTurnError = runErr.Error()`（`thread.go:1509`）、主机事件标题 `truncateTitle("任务失败:"+runErr.Error(), 80)`（`:1570`）、collab 状态 detail（`collab_delegate.go:494-496`）全部吃**字符串**，而 `Error()` 逐字节沿用旧文案 ⇒ 这三处零变化。
- **未碰 proto**：`pb.Error` 既有字段（`code/message/retryable/http_status/url/details`，`common.pb.go:60-70`），未新增字段、未改双副本。

### 2.5 hermes：单独判定 —— **登记不做（证据不足）**

- 可判定的部分：hermes 有 `errHermesBusy`（`hermes.go:185`，同一 `busySentinelError` 载体）、有看门狗静默与单轮上限两条具名阈值（`:532/:536`）—— 这三类**可以**照 zcode 同款映射。
- 证据不足的部分：hermes 的**主要失败形态**是 stopReason，而仓内对它的判定被自己标注为「保守启发」（`hermesapp/acp.go:639-645`）：
  > `StopReasonIsError` 桌面适配器把 provider 失败也包装成 end_turn + 错误正文（实测 DeepSeek 400 时 `stopReason=end_turn`），判定「这轮真的成了」只能看正文内容。……refusal/cancelled 视为失败；end_turn 交由正文判定。
  也就是说 `end_turn` 里的上游失败**只存在于正文文案**（turn_failure_copy）。要把它分成「用户拒答 / 上游 4xx 可重试 / 配置缺失」必须写文案启发式 —— 那正是红线 4「不新增写死的名单/版本二分」和「未知输入走透传」要禁的东西。仓内**没有** stopReason→retryability 的实测对账材料。
- 处置：只登记，不实现。落地条件：先补一份 stopReason × 正文 的实测矩阵（探针 `cmd/` 级别），再照 zcode_errors.go 同款落面。

---

## 3. 行为变更与客户端可见影响（增量登记）

**性质：增量，不是等价重构。** zcode 轮次失败从「只有一句人话」变成「人话 + 结构化字段」。

| 项 | 改动前（手机端收到） | 改动后（手机端收到） |
|---|---|---|
| 忙 / 锁等待耗尽 | `code=AGENT_ERROR, message="zcode 正忙(…)"`，无 retryable | `code=ZCODE_BUSY, retryable=true`，message 不变 |
| 看门狗静默 | `AGENT_ERROR` + `zcode 2m30s 无任何事件…` | `ZCODE_WATCHDOG_SILENCE, retryable=true` |
| 单轮超限 | `AGENT_ERROR` + `zcode 超时(60 分钟)` | `ZCODE_TURN_TIMEOUT` |
| turn.failed（最常见） | `AGENT_ERROR` + `zcode: 上游超时` | `ZCODE_TURN_FAILED, details="errorType=api_error turnPhase=…"`（上游 type/code/turnPhase **首次透出**） |
| 上游 RPC 报错 | `AGENT_ERROR` + `ZCode 返回错误 -32004: …` | `ZCODE_RPC_ERROR, details="jsonRpcCode=-32004 data=…"` |
| 账号同步失败 | `AGENT_ERROR` + 账号文案 | `ZCODE_ACCOUNT_SYNC` |

- **文案零变化**：`Message` / `LastTurnError` / 通知标题 / collab detail 全部逐字节沿用（各有断言，见 §4.3）。
- **不需要客户端改代码即可正确处理**：Android 与 Flutter 均无任何 `AGENT_ERROR`/`MISSING_CREDENTIAL`/code 的硬编码分支（`android/app/src/main/java/**/*.kt` 命中 0；`flutter/lib` 只有生成的 `common.pb.dart` 字段），UI 现状是渲染 `message`。新增字段对既有渲染路径是纯增量；未知 code 不会被当「未知/错误」显示（客户端根本不读 code）。
- **若将来客户端要按 code 分档**：`Retryable=true` 的三类（忙 / 未启动 / 看门狗静默）是「该稍后再试」，`ZCODE_ACCOUNT_SYNC` 是「该去登录」，语义已在 §2.3 固定。

---

## 4. 测试与变异验证

### 4.1 计数（两种口径都给，含失真说明）

| 口径 | HEAD 基线 | 本次 | 差 |
|---|---|---|---|
| pwsh 重定向（任务给定口径，控制台会粘连若干 `--- PASS:` 行） | 667（PASS 657 / SKIP 10 / FAIL 0） | 677（PASS 667 / SKIP 10 / FAIL 0） | +10（实际 +11，其中 1 行因编码粘连与前一行粘连） |
| python 直接捕获 stdout（无编码粘连） | 723 PASS / 10 SKIP（= 本次 734 − 新增 11） | **734（PASS 724 / SKIP 10 / FAIL 0）** | +11 |

**未删改既有断言的硬证据**：把 HEAD 捕获到的 667 个顶层用例名与本次结果做集合比对 —— `missing = ∅`（一条都没少、一条都没改名），新增恰好 11 条（§4.2）。既有测试文件**一个都没碰**（本轮只新增文件，未编辑任何既有 `*_test.go`）。

### 4.2 新增用例清单（11 条顶层 + 12 条子用例）

`daemon/internal/agent/codex_watchdog_consts_test.go`
1. `TestCodexWatchdogConstDefaults` —— 三个常量默认值 + `Milliseconds()==150_000`（单位口径不许被改坏）。
2. `TestWatchdogThresholdFamilyStaysAligned` —— **五家阈值同值**（静默 150s：codex/dsh/hermes/pi/zcode；单轮 60min：codex/dsh/hermes/pi），并显式写死「巡检节奏不在对齐口径内」（pi 更短）。
3. `TestCodexWatchdogCommentMatchesConst` —— 注释/文案与常量一致（下面 ③ 的取舍说明）。

`daemon/internal/agent/zcode_errorinfo_test.go`
4. `TestZcodeRunErrorImplementsCarrier` —— 类型断言形状 + 零值/nil 接收者不 panic + `nil` 输入不造错误。
5. `TestZcodeRunErrorReachesServerConsumer` —— **镜像 `server/thread.go:1518-1523` 的消费逻辑**：断言成功 → 终局帧带 `ZCODE_BUSY`+retryable；未实现 carrier 的错误仍走 `AGENT_ERROR`（反证老形态）。
6. `TestZcodeRunFailureExitsAreStructured` —— **接线钉**：AST 扫 `(*ZcodeProvider).Run` 每个 3 返回值的 `return`，非结构化出口（且不是 `ctx.Err()`/`nil`）即红，并给出行号。
7. `TestZcodeRunErrorFromClassification` —— 6 个形态逐个断言 Code / Retryable / Message / Details / `errors.Is` / `errors.As`。
8. `TestZcodeWatchdogErrorsAreStructured` —— 两种看门狗形态：文案逐字节旧值 + code + retryable + `errors.Is(…, errZcode*哨兵)`。
9. `TestZcodeTurnFailedErrorCarriesUpstreamDetail` —— turn.failed / turn.completed 非成功两条：文案不变、Details 原样、Unwrap 为 nil（不编原因链）。
10. `TestZcodeTurnFailedDetailExtraction` —— 5 种 payload → Details 抽取（只拼在场字段；坏帧不编细节）。
11. `TestZcodeTurnFailedMappingUnchanged` —— 既有映射 `mapZcodeTurnFailed` 的文案/口径逐字不变。

### 4.3 ①号债（注释↔值）那条用例的取舍：锚点式，不是全文扫

任务建议「断言注释里的秒数与常量值一致」，我采纳方向、但**拒绝全文扫 `N 秒/N 分钟`**：

- 全文扫必然误伤三处 —— 同文件里 120 秒的 `callTimeout`、15 分钟的审批预算、8 秒的握手预算都是别的阈值，还有「P1-2 之前这里写的是 90 秒」这类**历史陈述**（必须留着，否则后人不知道这条债存在过）。这类用例会变成「大家都不喜欢但不敢删」的死钉。
- 落地形态：**锚点式**三面钉 —— ① 三个常量**各自的文档注释**（该出现的数必须在、且不许出现别的数）；② 两条会发到手机上的错误文案 + 一条日志（按 `无任何事件` / `codex 超时(` / `轮次超时(` 三个锚句定位；这里特意**不用**宽锚「`超时(`」—— 它会把 15 分钟的审批文案扫进来，第一次跑就红了，已实测并收窄）；③ 等待循环那段说明注释（锚句「收不到任何通知」）。
- **不脆弱的理由**：只在「相关位置出现**错的**数」或「该出现的数缺失」时红；措辞怎么改（换成「两分半」）、历史陈述怎么写都不误伤。若将来有人把阈值真改了而忘了改文案，这三条立刻红 —— 这正是要防的同类再犯。

### 4.4 变异验证（两次，含一次「第一次没抓住」）

**变异 A —— 改坏分类**：`zcode_errors.go` 忙哨兵分支条件改成 `errors.Is(err, ErrZcodeBusy) && false`

```
exit=1   FAIL_top: ['TestZcodeRunErrorReachesServerConsumer', 'TestZcodeRunErrorFromClassification']
PASS_top: 722   SKIP_top: 10        ← 其余全绿
msg: zcode_errorinfo_test.go:249: Code = "ZCODE_ERROR", want "ZCODE_BUSY"
msg: zcode_errorinfo_test.go:252: Retryable = false, want true
```
→ **恰好 2 条红**，且报的是「分类错了」而不是「用例跑不起来」。改回后 `zcode_errors.go` 的 SHA-256 与变异前快照**完全一致**。

**变异 B —— 改坏接线**：`zcode.go` 的 `ensureSession` 出口退回裸 `return nil, nil, err`

- **第一次（未抓住）**：`exit=0`，全绿。原因：接线钉当时按**变量名**做白名单，而看门狗分支里的局部 `err := zcodeErrorf(…)` 把同名遮蔽的 `err` 一起放行了。
- **据此加固**（两处）：① 接线钉判据收紧为「返回表达式里**就地**带结构化构造器 / `ctx.Err()` / `nil`」三条，去掉变量名白名单；② 生产侧把看门狗的两个 `return` 各自**就地构造**（与 `codex.go` 同处「同一句文案写两遍」的形态一致），并在代码注释里写明为什么。
- **第二次（抓住）**：
```
exit=1   FAIL_top: ['TestZcodeRunFailureExitsAreStructured']
PASS_top: 723   SKIP_top: 10
msg: zcode_errorinfo_test.go:151: 第 1853 行:Run 的失败出口没走结构化构造器(会静默退化成 AGENT_ERROR): err
```
→ **恰好 1 条红**，直接点名行号与表达式。

**改回后零残留的核验**：① 变异标记全仓 grep（`MUTATION`）命中 0（唯一命中是 `zcode_selection_test.go:52` 的既有无关用例）；② `zcode_errors.go` SHA-256 与快照一致；③ `zcode.go` 逐点核对 9 个接线 + 2 处 `ctx.Err()` 全部在位（grep 见 §2.2 行号）；④ `zcode_errorinfo_test.go` 与快照的差异仅为加固后的接线钉（有意改动）。

---

## 5. 验证（证据原文）

```
$ go build ./...                                    # C:\Go\bin\go.exe,in G:\项目1\pocketagent\daemon
exit=0   （无输出）

$ go test ./internal/agent/... -count=1 -v
exit=0
PASS_top 724   SKIP_top 10   FAIL_top []   total_top 734
ok  	github.com/pocketagent/daemon/internal/agent	17.613s

$ go test -race ./internal/agent/... -count=1
exit=0
ok  	github.com/pocketagent/daemon/internal/agent	20.138s

$ go vet ./internal/agent/...
exit=0   （无输出）

$ gofmt -l internal/agent/codex.go
CLEAN
$ gofmt -l internal/agent/zcode.go
CLEAN
$ gofmt -l internal/agent/zcode_errors.go
CLEAN
$ gofmt -l internal/agent/codex_watchdog_consts_test.go
CLEAN
$ gofmt -l internal/agent/zcode_errorinfo_test.go
CLEAN
（全程未执行 gofmt -w；格式按 gofmt 期望手改）

$ go test ./internal/... -count=1        # 兜底：确认 server 等邻包未回归
exit=0   ok_pkgs 27   bad_lines []
```

### 5.1 `git status --short` 的诚实说明

本会话**开始时工作树是干净的**（会话首次 `git status --short` 无输出）。当前状态：

```
 M daemon/internal/agent/codex.go          ← 本次
 M daemon/internal/agent/zcode.go          ← 本次
?? daemon/internal/agent/zcode_errors.go            ← 本次(新文件)
?? daemon/internal/agent/codex_watchdog_consts_test.go  ← 本次(新文件)
?? daemon/internal/agent/zcode_errorinfo_test.go      ← 本次(新文件)
?? docs/reviews/2026-10-17/fix-agent-small-debts.md   ← 本报告(新文件)

 M AGENTS.md / M AUDIT.md / tools/audit/pa_upgrade_gate.py / tools/audit/test_pa_upgrade_gate.py
 M tools/device/README.md / ?? .github/
?? docs/reviews/2026-10-17/add-ci.md / ?? docs/reviews/2026-10-17/fix-dist-freshness-hole.md
   ↑ 以上**不是本会话产生的**:本会话的全部写操作只落在
     daemon/internal/agent/ 的 5 个文件 + 本报告文件(mtime 14:09–14:23 区间内,
     与并发子会话的写入时间交错;git status 会话初始为空即是证据)。
```

因此「`git status --short` 只列 agent 下文件」这条**在本仓库当前并发状态下无法成立**；可核对的事实是：**本次改动的文件全部且仅在 `daemon/internal/agent/`（外加本报告这一份 .md 新文件）**，未触碰任何禁区路径。

---

## 6. 与审查报告的差异登记（复核后更正/补充）

| # | 报告原文 | 更正/补充 | 依据 |
|---|---|---|---|
| 1 | 「先加 `var _ ErrorInfoCarrier = (*ZcodeRunError)(nil)`，zcode 的缺口立刻在 CI 里变红」 | **该建议不可行**：断言要求 zcode 已实现接口，此时加断言 = 直接编译失败。本轮改为「先实现、再断言」，断言随实现一起落地 | 任务指令；`zcode_errors.go:114` |
| 2 | 类型名写作 `*ZcodeRunError` / `*HermesRunError` | 仓内三家均为**非导出小驼峰**（`codexRunError` / `dshRunError` / `piError`）；新类型随之取 `zcodeRunError`（同文件亦无 `HermesRunError` 先例） | `codex_events.go:25`、`dsh.go:1766`、`pi.go:2047` |
| 3 | 矩阵第 21 行：「单轮上限 …… **zcode 具名**；codex `60*time.Minute` 裸字面量」 | **zcode 的单轮上限不是具名的**：`zcode.go:2170` 仍是裸 `60*time.Minute`（zcode 只把**静默**看门狗具名成 `zcodeWatchdogSilence`）。故「裸字面量聚集」实为 codex 三处 + zcode 一处 | `zcode.go:2170`（本次未改，见 §7） |
| 4 | 「另外三家都拿 codex 当基准」 | 是**四家**：dsh / hermes / pi / zcode 都写着「对齐 codex」，pi 的同值来自 `pi.go:42` 的「对齐 codex 的同名纪律」。测试按四家钉 | `pi.go:42/77/79` |
| 5 | 表 20 行：「codex 全裸字面量 …… 五家里唯一一家没有具名看门狗常量」 | 精确化：codex 缺的是**三个**（tick/静默/上限）；zcode 缺一个（上限）。dsh 另有裸 `10 * time.Second` tick（`dsh.go:828`），pi 的 tick 是具名的但刻意 5 秒 | `zcode.go:2168`、`dsh.go:828`、`pi.go:83` |
| 6 | 「基线：顶层 667（PASS 657 / SKIP 10）」 | 该计数法会因控制台编码粘连**少记约 56 条**；无粘连口径下 HEAD 为 723 PASS / 10 SKIP。结论不受影响（本次 ≥ 基线，且既有用例名集合零缺失） | §4.1 |
| 7 | P1-1 风险行：「需同步确认 Android/Flutter 端对该字段是已有渲染路径还是会被当未知字段」 | 已核：两端**都没有**按 code 分支的渲染代码（Android Kotlin 命中 0；Flutter 只有生成的 pb 字段）。故是纯增量，不存在「被当未知字段」的风险 | `android/app/src/main/java/**` grep 0 命中；`flutter/lib/gen/**` 仅字段声明 |

---

## 7. 遗留登记（本轮不做，建议后续单独排）

1. **zcode 的单轮上限仍是裸字面量**：`zcode.go:2170` `60*time.Minute`。建议提 `zcodeTurnCap` 并把 `TestWatchdogThresholdFamilyStaysAligned` 的名单补上 zcode（改 3 行 + 测试 1 行）。本轮不动：P1-2 的指令限定在 codex 的三个字面量，且 zcode.go 的改动面已在 `ErrorInfoCarrier` 上用足。
2. **dsh 的巡检节奏是裸 `10 * time.Second`**（`dsh.go:828`），hermes/zcode 各有 `*WatchdogTick`（zcode 也是裸的，`zcode.go:2131`）。若要全对齐，同样是 3 行级别的小债。
3. **hermes 的 `ErrorInfoCarrier`**：待补 stopReason × 正文的实测矩阵后再做（§2.5）。**不要**先写文案启发式。
4. **本轮顺手手修的一条格式**：`zcode.go:1032` 插入一行 `//`（`mapZcodeToolCall` 文档注释分段）。该文件在 HEAD 上就被 `gofmt -l` 标脏（用 `git show HEAD:…` 取字节落盘后 `gofmt -l` 复现），属**既有偏差**，与三条债无关；为满足「改动文件 `gofmt -l` 无输出」的验收项手改一行注释分隔符。零行为影响，未用 `gofmt -w`。

---

## 8. 文件清单

**改动（2 改 + 3 增，全部在 `daemon/internal/agent/`）**

| 文件 | 性质 | 说明 |
|---|---|---|
| `daemon/internal/agent/codex.go` | 改 | +27 行注释与常量块（`codexWatchdogTick` / `codexSilenceWatchdog` / `codexTurnCap`）；等待循环注释纠错；3 处字面量换常量；**实值与文案零变化** |
| `daemon/internal/agent/zcode.go` | 改 | Run 的 9 个失败出口改走结构化构造器；新增 `lastErrDetail` 与 turn.failed 上游细节抽取；看门狗两个 return 就地构造；文档注释 1 行（§7.4） |
| `daemon/internal/agent/zcode_errors.go` | **新增** | `zcodeRunError`（Error/ErrorInfo/Unwrap）+ `var _ ErrorInfoCarrier` 镜像断言 + 8 个错误码常量 + 2 个看门狗哨兵 + `zcodeRunErrorFrom` / `zcodeErrorf` / `zcodeTurnFailedError` / `zcodeTurnFailedDetail` |
| `daemon/internal/agent/codex_watchdog_consts_test.go` | **新增** | 默认值 + 五家对齐 + 注释/文案↔值一致（3 条） |
| `daemon/internal/agent/zcode_errorinfo_test.go` | **新增** | 载体形状 + server 消费面镜像 + Run 接线钉 + 逐形态映射 + 细节抽取（8 条） |

**报告（本会话唯一的新增 .md）**：`docs/reviews/2026-10-17/fix-agent-small-debts.md`

**未触碰**：`daemon/internal/server/`、`daemon/internal/hermesapp/`、`proto/**`、`android/**`、`flutter/**`、`tools/**`、任何既有 `.md`（本报告是新增）、任何既有 `_test.go`。
