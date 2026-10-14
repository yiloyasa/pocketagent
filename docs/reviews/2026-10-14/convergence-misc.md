# daemon/internal 非 agent/server/gen 包收敛改造(2026-10-14)

> 范围:`daemon/internal/` 下**除 `agent/`、`server/`、`gen/` 之外**的全部子包(26 个既有包,含本轮新建 `testkit/`)。
> `agent/`、`server/` 由另外两个并行子会话负责,`gen/` 为 protoc 产物;`cmd/`、`flutter/`、`android/`、`proto/` 一律未动。
> 约束:**等价优先**——纯收敛、零业务语义改变,任何(含仅诊断层面的)行为差异都在 §3 如实记录;
> 测试不删、不跳、不弱;**本报告与全部改动均未 commit**(主会话统一提交)。
> 红线复核:用户桌面配置(`~/.codex/` `~/.dsh/` `~/.pi/`)只读未触碰;`selfguard` 只做死代码排查、guard 逻辑未动(0.4.0 刚发布);
> store schema 未动;dshapp/hermesapp(codexapp/piapp)登录/启动/握手错误处理与超时语义未动。

---

## 0. 绿基线与收敛后验证(原样证据)

### 0.1 基线(改造前,先立绿再动手)

```
C:\Go\bin\go.exe build ./...                    → exit 0
C:\Go\bin\go.exe test  -count=1 ./internal/...  → 全部 ok,0 FAIL(逐包计数见下表 base 列)
C:\Go\bin\go.exe test  -count=1 -v ./internal/... → 保存为 baseline_v.txt(计数来源)
```

基线逐包 `ok` 行(原样摘录自 `baseline_v.txt`):

```
ok  	github.com/pocketagent/daemon/internal/agency	0.791s
ok  	github.com/pocketagent/daemon/internal/agent	17.632s
ok  	github.com/pocketagent/daemon/internal/codexapp	4.502s
ok  	github.com/pocketagent/daemon/internal/collabmcp	3.135s
ok  	github.com/pocketagent/daemon/internal/dshapp	4.653s
ok  	github.com/pocketagent/daemon/internal/eventstore	2.464s
ok  	github.com/pocketagent/daemon/internal/externalconv	1.328s
?   	github.com/pocketagent/daemon/internal/gen/pocketagent/v1	[no test files]
ok  	github.com/pocketagent/daemon/internal/hermesapp	4.495s
ok  	github.com/pocketagent/daemon/internal/hidewin	0.979s
ok  	github.com/pocketagent/daemon/internal/hostenv	1.762s
ok  	github.com/pocketagent/daemon/internal/hostenv/goscan	1.916s
ok  	github.com/pocketagent/daemon/internal/jobobject	1.894s
ok  	github.com/pocketagent/daemon/internal/mdns	2.155s
ok  	github.com/pocketagent/daemon/internal/panel	3.157s
ok  	github.com/pocketagent/daemon/internal/pathguard	3.881s
ok  	github.com/pocketagent/daemon/internal/piapp	13.561s
ok  	github.com/pocketagent/daemon/internal/rotatelog	0.821s
ok  	github.com/pocketagent/daemon/internal/rpcwire	2.752s
ok  	github.com/pocketagent/daemon/internal/runtime	4.251s
ok  	github.com/pocketagent/daemon/internal/selfguard	1.508s
ok  	github.com/pocketagent/daemon/internal/server	24.189s
ok  	github.com/pocketagent/daemon/internal/stderrdrain	1.363s
ok  	github.com/pocketagent/daemon/internal/store	2.829s
ok  	github.com/pocketagent/daemon/internal/term	3.166s
ok  	github.com/pocketagent/daemon/internal/testagent	4.177s
ok  	github.com/pocketagent/daemon/internal/tlscert	2.399s
?   	github.com/pocketagent/daemon/internal/version	[no test files]
ok  	github.com/pocketagent/daemon/internal/zcodeapp	13.265s
```

基线 `FAIL` 计数:`^--- FAIL` = **0**,`^FAIL` = **0**。

### 0.2 收敛后(最终态,含 §2 全部改动)

```
C:\Go\bin\go.exe build ./internal/...   → exit 0
C:\Go\bin\go.exe build ./...            → exit 0(全仓库,含 cmd/)
C:\Go\bin\go.exe vet   ./internal/...   → exit 0,无任何输出
C:\Go\bin\gofmt.exe -l <全部触碰目录>    → 无输出(gofmt 干净)
C:\Go\bin\go.exe test  -count=1 -v ./internal/... → exit 0,保存为 after2_v.txt,FAIL = 0
```

收敛后逐包 `ok` 行(原样摘录自 `after2_v.txt`):

```
ok  	github.com/pocketagent/daemon/internal/agency	1.962s
ok  	github.com/pocketagent/daemon/internal/agent	19.047s
ok  	github.com/pocketagent/daemon/internal/codexapp	6.061s
ok  	github.com/pocketagent/daemon/internal/collabmcp	3.773s
ok  	github.com/pocketagent/daemon/internal/dshapp	5.964s
ok  	github.com/pocketagent/daemon/internal/eventstore	2.672s
ok  	github.com/pocketagent/daemon/internal/externalconv	2.185s
?   	github.com/pocketagent/daemon/internal/gen/pocketagent/v1	[no test files]
ok  	github.com/pocketagent/daemon/internal/hermesapp	6.018s
ok  	github.com/pocketagent/daemon/internal/hidewin	0.945s
ok  	github.com/pocketagent/daemon/internal/hostenv	2.387s
ok  	github.com/pocketagent/daemon/internal/hostenv/goscan	2.196s
ok  	github.com/pocketagent/daemon/internal/jobobject	1.752s
ok  	github.com/pocketagent/daemon/internal/mdns	2.439s
ok  	github.com/pocketagent/daemon/internal/panel	3.944s
ok  	github.com/pocketagent/daemon/internal/pathguard	4.318s
ok  	github.com/pocketagent/daemon/internal/piapp	14.181s
ok  	github.com/pocketagent/daemon/internal/rotatelog	1.373s
ok  	github.com/pocketagent/daemon/internal/rpcwire	2.390s
ok  	github.com/pocketagent/daemon/internal/runtime	4.719s
ok  	github.com/pocketagent/daemon/internal/selfguard	2.049s
ok  	github.com/pocketagent/daemon/internal/server	25.432s
ok  	github.com/pocketagent/daemon/internal/stderrdrain	1.504s
ok  	github.com/pocketagent/daemon/internal/store	3.055s
ok  	github.com/pocketagent/daemon/internal/term	4.236s
ok  	github.com/pocketagent/daemon/internal/testagent	3.995s
?   	github.com/pocketagent/daemon/internal/testkit	[no test files]
ok  	github.com/pocketagent/daemon/internal/tlscert	2.646s
?   	github.com/pocketagent/daemon/internal/version	[no test files]
ok  	github.com/pocketagent/daemon/internal/zcodeapp	13.756s
```

**逐包顶层用例数(基线 vs 收敛后,统计口径:非缩进 `=== RUN` 行、无 `/` 的顶层测试名;脚本 `cnt.py` 读两份 `-v` 日志):**

```
agency                   base=3      after=3
agent                    base=671    after=671
codexapp                 base=31     after=31
collabmcp                base=18     after=18
dshapp                   base=38     after=38
eventstore               base=15     after=15
externalconv             base=5      after=5
gen/pocketagent/v1       base=0      after=0
hermesapp                base=39     after=39
hidewin                  base=1      after=1
hostenv                  base=16     after=16
hostenv/goscan           base=3      after=3
jobobject                base=1      after=1
mdns                     base=4      after=4
panel                    base=16     after=16
pathguard                base=6      after=6
piapp                    base=70     after=70
rotatelog                base=3      after=3
rpcwire                  base=5      after=5
runtime                  base=117    after=117
selfguard                base=9      after=9
server                   base=525    after=525
stderrdrain              base=6      after=6
store                    base=40     after=40
term                     base=11     after=11
testagent                base=9      after=9
testkit                  base=-      after=0        ← 新增包,0 用例(无 *_test.go)
tlscert                  base=4      after=4
version                  base=0      after=0
zcodeapp                 base=49     after=49
DIFFS: [('testkit', '-', 0)]
total base 1715  total after 1715
```

**结论:除新增 `testkit`(0 用例)外,逐包顶层用例数与基线逐一相等;总用例 1715 → 1715;`FAIL` 0 → 0。测试没有被删除、跳过或弱化**(§2.B 有死代码删除,已逐条证明被删符号 0 引用且不含任何 `func Test*`)。**

### 0.3 -race 复跑(全部触碰包)

```
C:\Go\bin\go.exe test -race -count=1 ./internal/testkit/ ./internal/codexapp/ ./internal/zcodeapp/ \
    ./internal/piapp/ ./internal/dshapp/ ./internal/eventstore/ ./internal/term/ \
    ./internal/agency/ ./internal/testagent/
```

```
?   	github.com/pocketagent/daemon/internal/testkit	[no test files]
ok  	github.com/pocketagent/daemon/internal/codexapp	3.685s
ok  	github.com/pocketagent/daemon/internal/zcodeapp	14.042s
ok  	github.com/pocketagent/daemon/internal/piapp	13.205s
ok  	github.com/pocketagent/daemon/internal/dshapp	3.407s
ok  	github.com/pocketagent/daemon/internal/eventstore	2.273s
ok  	github.com/pocketagent/daemon/internal/term	3.198s
ok  	github.com/pocketagent/daemon/internal/agency	1.748s
ok  	github.com/pocketagent/daemon/internal/testagent	4.984s
```

### 0.4 审核门禁(仓库六条规则)

```
python tools/audit/pa_audit.py --json   → exit 0
summary = {'redline': 0, 'warn': 2, 'pass': 5}
```

| 规则 | 结果 | 说明 |
|---|---|---|
| sheet_overflow | pass | — |
| roster | warn | 挂牌项 `daemon/internal/agent/codex_requests.go:107`(附录 A 既存挂牌,agent/ 属兄弟会话,非本轮引入) |
| unknown_ui | pass | — |
| fake_green | warn | 「最新结果 XML 距今 117 分钟」——Android 侧 XML 新鲜度,与本轮 Go 改动无关 |
| go_baseline | pass | `go build ./... 通过` / `go vet ./... 通过` |
| proto_sync | pass | 双副本一致(本轮未碰 proto) |

---

## 1. 清单(扫描产物 + 逐项判决)

### 1.1 范围内包清单(行数含测试文件,改造后口径)

| 包 | 文件数 | 行数 | 备注 |
|---|---:|---:|---|
| runtime | 49 | 9416 | |
| zcodeapp | 23 | 4248 | 本轮触碰(测试) |
| piapp | 23 | 4054 | 本轮触碰(测试) |
| store | 15 | 3634 | |
| dshapp | 16 | 2956 | 本轮触碰(测试) |
| hermesapp | 11 | 2530 | |
| codexapp | 10 | 1600 | 本轮触碰(测试) |
| collabmcp | 5 | 1256 | |
| testagent | 2 | 1233 | 本轮触碰(死代码) |
| eventstore | 4 | 1139 | 本轮触碰(测试) |
| term | 7 | 1134 | 本轮触碰(测试) |
| hostenv | 8 | 938 | |
| panel | 3 | 869 | |
| pathguard | 2 | 604 | |
| selfguard | 7 | 480 | 红线:只查死代码,未动 |
| agency | 4 | 414 | 本轮触碰(死代码) |
| mdns | 2 | 392 | |
| externalconv | 2 | 383 | |
| rpcwire | 2 | 350 | |
| tlscert | 2 | 334 | |
| hostenv/goscan | 3 | 221 | |
| rotatelog | 2 | 216 | |
| stderrdrain | 2 | 209 | |
| **testkit(本轮新建)** | 6 | 160 | 仅测试基建,生产代码禁 import |
| jobobject | 3 | 123 | |
| hidewin | 2 | 79 | |
| version | 1 | 29 | |
| *(排除)* agent | 197 | 41891 | 兄弟会话,未动 |
| *(排除)* server | 165 | 39023 | 兄弟会话,未动 |
| *(排除)* gen/pocketagent/v1 | 15 | 21192 | protoc 产物,未动 |

### 1.2 重复扫描(`dup.txt` / `near2.txt`,≥2 包逐字同款或同名近似)

| # | 重复点 | 出处(改造前) | 判决 |
|---|---|---|---|
| 1 | `TestSourceHasNoDeveloperPaths`(31 行 ×2,逐字) | dshapp / piapp `devpath_assert_test.go:29` | **✅ 执行 A4** |
| 2 | `waitProcessGone`(21 行 ×2,Windows,逐字) | codexapp / zcodeapp `close_wait_windows_test.go` | **✅ 执行 A2** |
| 3 | `waitProcessGone`(17 行 ×2,非 Windows,逐字) | codexapp / zcodeapp `close_wait_other_test.go` | **✅ 执行 A2** |
| 4 | `waitFile`(16 行 ×2,逐字) | codexapp:21 / piapp:32 `start_args_test.go` | **✅ 执行 A1** |
| 5 | `captureLog`(7 行 ×2,逐字) | eventstore:26 / term:33 | **✅ 执行 A3** |
| 6 | `(Candidate) reject/allow…`→`AllowOptionID`/`RejectOptionID` | dshapp/protocol.go:553,562 ↔ hermesapp/acp.go:595,606 | ⛔ 已识别未动(§4.3) |
| 7 | `forget`(8 行 ×4) | codex/dsh/hermes/zcode `client.go` | ⛔ 未动(§4.6) |
| 8 | `syncHandlers` / `writeLine`(6 行 ×4) | codex/dsh/pi/zcode `client.go` | ⛔ 未动(§4.6) |
| 9 | `SetGlobalHandler` / `SetUnknownResponseHandler` | codex/dsh/pi/zcode `client.go` | ⛔ 未动(§4.6) |
| 10 | `reapWithLog`(11 行 ×2)、`drainStderr`(9 行 ×2)、`Call`(4 行 ×3) | codex/dsh(/zcode) `client.go` | ⛔ 未动(§4.2/4.6) |
| 11 | `Alive`(9 行 ×2)、`readStderr`、`takeAbandoned`、`readLoop` | 跨 app 包 | ⛔ 未动(§4.6) |
| 12 | `SetTurnHandlers`(0.99)、handler 并发测试(0.98–0.99) | codexapp/zcodeapp | ⛔ 未动(§4.5) |
| 13 | `TestDispatchUnknownResponseFallsBackToHandler`(0.97,47 行) | codexapp/dshapp(/zcodeapp) | ⛔ 未动(§4.4) |
| 14 | `ParseRequestPermission`(0.98) | dshapp/hermesapp | ⛔ 未动(§4.3) |
| 15 | 8 行同款 run ×67 组(handler 快照、JSON-RPC 外壳、mutex 段等) | codex/dsh/pi/zcode `client.go` | ⛔ 未动(§4.6) |

### 1.3 死代码扫描(`dead.txt`:除声明文件外、剥注释后全仓库 0 文本引用)

| 候选 | 判决 |
|---|---|
| `agency.HasProvider`(agency.go:146) | **✅ 执行 B1 —— 已删** |
| `testagent.ToolCallCard`(fake_agent.go:133) | **✅ 执行 B2 —— 已删** |
| `testagent.CompletedBlocks`(fake_agent.go:235) | **✅ 执行 B3 —— 已删** |
| `testagent.CodexEventFrame`(fake_agent.go:160) | ⛔ 未动(§4.7):删除需连带删 `isAgentEvent` 类型与 emit `switch` 分支,构成行为链改动,且该包被 agent/、server/ 兄弟会话消费 |
| `store.MarshalJSON`(store.go:124) | ⛔ 误报:由 `encoding/json` 反射调用(`m := StoredMessage` 值接收者),删了会改变序列化行为 |

---

## 2. 逐项执行记录 + 等价性论证

### A. 新增共享测试基建包 `daemon/internal/testkit/`(6 文件 / 160 行)

`doc.go` 明确写入约束:**本包只应被 `*_test.go` 引用,生产代码不得 import**(否则 `testing` 被链进 daemon 二进制)。
验证:`go list -f '{{join .Imports " "}}' ./internal/testkit` → `bytes fmt hostenv/goscan log os strings syscall testing time`;
全仓库 import 本包的文件共 8 个,**全部为 `*_test.go`**(见 §6)。

#### A1 `waitfile.go` — `WaitFile(t, path, d)`(22 行)

- **收编自**:`codexapp/start_args_test.go:21 waitFile` 与 `piapp/start_args_test.go:32 waitFile`(扫描确认两份 16 行逐字同款)。
- **调用点改写**:codexapp 2 处(`start_args_test.go:42,117`)、piapp 1 处(`start_args_test.go:95`);本地 `waitFile` 定义删除。
- **等价性**:脚本 `verbatim2.py` 对剥注释后的语句逐行 diff —— `codexapp == piapp == testkit` **为真**;
  `t.Helper()` 原本就有;轮询 25ms、超时返回 `os.ErrNotExist`、签名 `(t, path, d) ([]byte, error)` 全部原样。
  调用方断言、超时值(5s)、错误文案一字未改。

#### A2 `waitproc_windows.go` / `waitproc_other.go` — `WaitProcessGone(pid, timeout)`(38 + 29 行,build tag `windows` / `!windows`)

- **收编自**:4 个被删文件 `codexapp|zcodeapp × close_wait_{windows,other}_test.go`(各仅含该 helper,**0 个 `func Test*`**,用 `git show HEAD:<file>` 逐个核验)。
- **调用点改写**:codexapp `close_reap_test.go:64`、zcodeapp `client_test.go:260`(各 1 处,参数 10s 不变)。
- **等价性**:脚本 `verbatim_check.py` 对剥注释后的函数体做逐行 diff,四个原始文件 vs 两份 testkit 实现 —— **全部 `identical: True`**;
  平台分支一一对应(Windows: `OpenProcess` + `WaitForSingleObject(100ms)`,打开失败视为已退出;非 Windows: `Signal(0)` 探测 + 50ms sleep);
  超时文案 `等待进程 %d 退出超时(%s),疑似只 Kill 未收尾` 与原件**逐字节相同**(见 §3.1 的过程记录)。
- **测试用例 0 删除**:被删的 4 个文件是 helper-only(`func Test` 命中数 = 0),用例本体在 `close_reap_test.go` / `client_test.go` 中保留。

#### A3 `logcap.go` — `CaptureLog()`(16 行)

- **收编自**:`eventstore/eventstore_degrade_test.go:26` 与 `term/manager_idle_test.go:33` 的 `captureLog`(7 行逐字同款)。
- **调用点改写**:eventstore 3 处(`:61,121,186`)、term 3 处(`:33,60,165`);本地定义删除,`bytes`/`log`/`os` 未再用的 import 剪除。
- **等价性**:`verbatim2.py` 判定 `eventstore == term == testkit` **为真**(剥注释语句级);
  `log.SetOutput(&buf)` + 恢复回 `os.Stderr` 原样,断言读取的 `read()` 语义不变。

#### A4 `devpath.go` — `AssertNoDeveloperPaths(t)`(40 行)

- **收编自**:`dshapp/devpath_assert_test.go:29` 与 `piapp/devpath_assert_test.go:29` 的 `TestSourceHasNoDeveloperPaths`(扫描确认两份 31 行**逐字相同**)。
- **调用点改写**:两个包的测试**函数名保留、用例保留**,函数体变成一行委托
  `testkit.AssertNoDeveloperPaths(t)`(`dshapp:29` / `piapp:29`),`strings`/`goscan` 本地 import 剪除。
- **等价性**:`verbatim3.py` 对剥注释语句 diff —— 唯一差异是 `+ t.Helper()`(见 §3.2);判定条件完全一致:
  `goscan.PackageGoFiles(".")` 枚举、空结果 `t.Fatal`(防"闸门静默空转")、逐文件 `goscan.DeveloperTracesInFile` AST 扫描、命中即 `t.Fatalf`。
  `"."` 仍解析为**被测包源码目录**(go test 在包目录启动测试二进制,helper 不改变进程 cwd),所以扫描对象与改造前同包同文件。
  5 处失败文案(`枚举包内 Go 文件失败` / `未扫到任何源文件 —— 闸门静默空转就是假绿` / `解析 %s 失败` / `f+" → "+…` / `…规格 §2 禁止`)已恢复为**与原件逐字相同**(见 §3.1)。

**A 小结**:4 组逐字重复 helper → 1 份共享实现;13 个调用点改写(`WaitFile` 3、`WaitProcessGone` 2、`CaptureLog` 6、`AssertNoDeveloperPaths` 2);
4 个 helper-only 测试文件删除(0 用例损失);**全部包顶层用例数不变(§0.2 表)。**

### B. 死代码删除(仅限 0 引用、且不需要连带链式修改的)

#### B1 `agency.HasProvider`(agency.go,删除 10 行含注释)

- 依据:`dead.txt` 命中;删除后全仓库复扫 `HasProvider|ToolCallCard|CompletedBlocks` —— Go 代码 **0 命中**
  (残留命中只有 Android Kotlin 的同名 UI 组件 `ToolCallCard`(与 Go 无关)与 `aoci.code.txt` 认知索引文本,见 §5)。
- 等价性:导出函数、无调用方 → 删除前后所有可达行为逐条相同;`agency` 包 3 个用例、`agent` 包 671 个用例均保持绿。
- **注意**:`agent/`、`server/` 属兄弟会话,删除前已确认它们不引用该符号(编译即证:`go build ./...` exit 0)。

#### B2/B3 `testagent.ToolCallCard` / `testagent.CompletedBlocks`(fake_agent.go,删除 6 行)

- 依据:`dead.txt` 命中 + 删除后全仓库 0 引用(同上复扫)。
- 等价性:两者是**纯构造器一行体**(`return toolCallEvent{...}`),不参与任何分发/类型判定;
  `testagent` 9 个用例、消费方 `agent`(671)、`server`(525)全绿,证明无隐藏引用。
- 对比项:`CodexEventFrame` 见 §4.7 —— 它不是纯构造器,删除会牵动事件分发 switch,故**不动**。

**B 小结:删 3 个符号共 16 行;0 用例删除;编译期全绿证明 0 引用。**

---

## 3. 差异记录(只记录,不改语义)

### 3.1 过程中发现并已消除的偏差(如实记录)

首轮收编时,testkit 里有 **2 处文案与原件不完全一致**(行为判定不受影响,但不满足"逐字"):

| 位置 | 首轮文案 | 原件文案 | 处理 |
|---|---|---|---|
| `waitproc_{windows,other}.go` 超时 Errorf | `…可能只 Kill 未收尸` | `…疑似只 Kill 未收尾` | ✅ 已改回原件(4 个原始文件同款) |
| `devpath.go` 4 处失败文案 | `未扫描到任何源文件 —— 门禁跳过默认转为静默` 等 | `未扫到任何源文件 —— 闸门静默空转就是假绿` 等 | ✅ 已改回原件(5 处逐字复原) |

复原后由 `verbatim_check.py` / `verbatim2.py` / `verbatim3.py` 重新跑逐行 diff:
`waitProcessGone` 四份原件 vs 两份 testkit = **identical**;`waitFile` / `captureLog` = **identical**;
`AssertNoDeveloperPaths` = 仅 `+ t.Helper()` 一行(见 3.2)。**当前树状态即为最终状态。**

### 3.2 保留的(诊断层面)差异

1. **`TestSourceHasNoDeveloperPaths` 失败行号指向**:新增的 `t.Helper()` 使失败位置从
   「`devpath_assert_test.go` 内 Fatalf 行」变为「同文件的一行委托调用行」。判定条件、扫描范围、文案均不变;
   只影响测试失败时打印的行号(属改进,保留并记录)。
2. **helper 定义所在文件变化**:`waitFile`/`waitProcessGone`/`captureLog` 从各包测试文件迁到 `testkit`,
   测试文件的 import 列表相应增减(`+testkit` / `-bytes,-log,-os,-strings,-goscan` 等未用 import)。运行时语义不变。
3. **包集合变化**:新增 `internal/testkit`(无 `*_test.go`,在 `-v` 日志中显示 `[no test files]`),逐包用例总数 1715 不变。
4. **注释重写**:testkit 内的函数注释是收编版说明(原注释在 `git show HEAD:<file>` 可查)。注释不参与行为。
5. **环境观察(非本轮改动)**:过程中一度出现 `go build ./...` 在 `cmd/capprobe` 报
   `cannot use m (rpcMsg) as noteRec …` —— 该目录当时是 `git status` 里的未跟踪新增目录(兄弟会话并行创建),
   错误为其文件内部自洽性问题,且与本轮符号(`HasProvider` 等)无关;**最终 `go build ./...` = exit 0**。

---

## 4. 已识别未动(候选收敛点,逐条说明为什么不动)

1. **`call`/pending/timeout 机器跨包复制**(codexapp/dshapp/zcodeapp `client.go`:`Call`、`readLoop`、`takeAbandoned`、`writeWithBudget` 等):
   相似度 0.95+,但三家超时/上下文/错误回退细节不逐条同源,合并会触碰**超时语义**——红线保护,不动。
2. **`drainStderr`/`reapWithLog`/`Alive`/`readStderr`**(codexapp ↔ dshapp ↔ piapp ↔ zcodeapp):
   同名近似但归属各自进程管理路径,合并需改跨包 API 面(乙级),风险大于收益。
3. **`AllowOptionID`/`RejectOptionID`/`ParseRequestPermission`/`RequestPermissionParams`**(dshapp/protocol.go ↔ hermesapp/acp.go):
   字段与 fallback 逻辑存在真实差异(`RejectOptionID` 虽逐字同款,但两侧协议族不同),且属 ACP **审批**路径——
   红线"绝不替用户做审批决定",不动。
4. **`TestDispatchUnknownResponseFallsBackToHandler`(4 份)与 handler 并发测试(2 份)**:
   文本相似度 0.97–0.99,但 id 类型(string vs int64 vs 缺席)、dispatch 签名、附带断言用例集不同,
   **不可证等价** → 不合并、不删。
5. **`SetGlobalHandler`/`SetTurnHandlers`/`syncHandlers`/`publishHandlersLocked` 快照机制**(4 家):
   形状相似但 dshapp 多 `UnknownResponseHandler` 分支等差异,合并需抽公共接口(乙级改动),不动。
6. **`forget`/`writeLine`/`SetUnknownResponseHandler`/`rpcResp`/`wireMsg` 等小件**:
   各家 id 形状不同(codex 有 `idPresent` 守卫、zcode 省略 `"jsonrpc":"2.0"`、dsh/hermes 用 int64),不动。
7. **`testagent.CodexEventFrame`**(dead 扫描命中):
   删除需连带删 `isAgentEvent` 判定与 emit `switch` 分支(行为链改动),且 `testagent` 被 agent/、server/ 兄弟会话消费 → 不动。
8. **`store.MarshalJSON`**(dead 扫描命中):`encoding/json` 反射调用的假阳性,删了会改变序列化行为 → 不动。
9. **store getter/setter 样板、`LoadOutcome` nil-receiver 方法**:embedding 会改变 nil-receiver 行为 → 不动。
10. **`selfguard` unix/windows `tryLock`**:0.4.0 刚发布的看门狗工件,只允许死代码级排查(本轮该包无死代码命中),guard 逻辑一行未动。
11. **`waitCond`(term)/`waitUntil`(rpcwire) 轮询间隔不同**(100ms vs 其他):合并会改节拍 → 不动。
12. **dshapp/hermesapp 权限参数结构体**(`PermissionToolCall`/`PermissionOption`):字段差异真实存在 → 不动。

---

## 5. 交接建议

1. **提交**:本轮全部改动未 commit。我的文件清单见 §6;`git status` 里 `agent/`、`server/`、`cmd/`、`flutter/`、`android/` 的改动**属兄弟会话,请分开提交/甄别**。
2. **AOCI 认知索引**:`aoci.code.txt` 的 `agency.go[DB7S]` 条目仍写着 `HasProvider(cs+provider)`;
   本会话无 AOCI MCP 工具(仅 `rea`),无法就地维护 → 建议主会话在提交后调用 `aoci_maintain` 刷新该条目(以及 testkit 新包是否纳管)。
3. **`testkit` 使用约定**:仅测试代码可 import;后续新增跨包同款 helper 优先收编到这里,
   但**只收编"与被测协议无关的测试基建"**,协议/超时/审批类逻辑不进 testkit(见 §4 红线理由)。
4. **合并前复跑**:兄弟会话仍在改 `agent/`/`server/`;主会话合并后请再跑一次
   `go build ./... && go vet ./internal/... && go test -count=1 ./internal/...`,以最终树为准更新 DEVELOPING.md §0.3 水位。
5. **`fake_green` warn**:Android 结果 XML 新鲜度与本轮无关,但按红线 5,Android 侧合入前需 `--rerun` 重跑核验时间戳。
6. **可复用脚本**(在 `C:\Users\yanzi\AppData\Local\Temp\opencode\`,建议需要时复制入库):
   `cnt.py`(逐包用例数对比)、`verbatim_check.py`/`verbatim2.py`/`verbatim3.py`(helper 逐字等价校验)、
   `go_dup.py`、`go_dead.py`、`go_near2.py`(重复/死代码扫描)。

---

## 6. 变更文件清单(本轮,未 commit)

**新增 `daemon/internal/testkit/`(6 文件 / 160 行,仅测试基建):**

| 文件 | 行数 | 内容 |
|---|---:|---|
| `doc.go` | 15 | 包定位与"生产禁 import"约束 |
| `waitfile.go` | 22 | `WaitFile` |
| `logcap.go` | 16 | `CaptureLog` |
| `devpath.go` | 40 | `AssertNoDeveloperPaths` |
| `waitproc_windows.go` | 38 | `WaitProcessGone`(windows) |
| `waitproc_other.go` | 29 | `WaitProcessGone`(!windows) |

**修改(12 个测试文件 + 2 个生产文件,`git diff` = +31 / −271):**

```
daemon/internal/agency/agency.go                        -10(删 HasProvider)
daemon/internal/testagent/fake_agent.go                  -6(删 ToolCallCard / CompletedBlocks)
daemon/internal/codexapp/close_reap_test.go              +3/-1(WaitProcessGone)
daemon/internal/codexapp/start_args_test.go              +4/-18(WaitFile ×2,删本地 waitFile)
daemon/internal/zcodeapp/client_test.go                  +3/-1(WaitProcessGone)
daemon/internal/piapp/start_args_test.go                 +3/-17(WaitFile,删本地 waitFile)
daemon/internal/piapp/devpath_assert_test.go             +4/-25(保留用例名,一行委托)
daemon/internal/dshapp/devpath_assert_test.go            +4/-25(同上)
daemon/internal/eventstore/eventstore_degrade_test.go    +5/-11(CaptureLog ×3,删本地 captureLog)
daemon/internal/term/manager_idle_test.go                +5/-13(CaptureLog ×3,删本地 captureLog)
```

**删除(4 个 helper-only 测试文件,合计 144 行,`func Test*` 命中数均为 0):**

```
daemon/internal/codexapp/close_wait_other_test.go        (32 行)
daemon/internal/codexapp/close_wait_windows_test.go      (41 行)
daemon/internal/zcodeapp/close_wait_other_test.go        (31 行)
daemon/internal/zcodeapp/close_wait_windows_test.go      (40 行)
```

**新增文档**:本文件 `docs/reviews/2026-10-14/convergence-misc.md`。

**净变化**:`-271 +31`(已跟踪)+ `+160`(testkit)= **净 −80 行**,重复 helper 由 4 组 13 处收敛为 1 份,测试水位 1715 → 1715 全绿。
