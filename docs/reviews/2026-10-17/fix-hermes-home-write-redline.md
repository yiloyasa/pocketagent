# fix-hermes-home-write-redline —— 根治 P0-1：`hermesapp` 写用户家 `~/.hermes/config.yaml`

- **日期**：2026-10-17
- **来源**：`docs/reviews/2026-10-17/clean-daemon-apps.md` §1.3【P0-1】
- **红线**：AGENTS.md §4 红线 2（绝不改写用户桌面配置：`~/.codex/`、`~/.dsh/`、`~/.pi/` **等**一律只读）+ §4 红线 5（不留假绿）
- **改动等级**：丙级（`internal/hermesapp` 整包）+ 乙级（`internal/agent` 的 `hermes.go` 调用点跨线，已按协调口径只做「诚实透出」+ 注释）
- **提交状态**：**未 commit / 未 add / 未 push**（按指令）

---

## 0. 一句话

`SetReasoningEffort` 原本在 `HERMES_HOME` 未设时**把用户的 `~/.hermes/config.yaml` 原���改写掉**，并且**静默报告成功**；现在写面只认 `%HERMES_HOME%`，未设即拒写、零字节落盘，并把「未固定 + 原因」如实透给手机端。读面（status/查询看用户真实配置）**保持家目录回落不变**。

---

## 1. 违规证据原文（改前代码 + 改后行为实证）

### 1.1 原代码（`daemon/internal/hermesapp/config.go`，改前）

```go
// ConfigPath 返回 hermes 家目录 config.yaml 的绝对路径。
func ConfigPath() string {
	if home := strings.TrimSpace(os.Getenv("HERMES_HOME")); home != "" {
		return filepath.Join(home, "config.yaml")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".hermes", "config.yaml")   // ← 用户家目录
	}
	return ""
}

func SetReasoningEffort(effort string) error {
	path := ConfigPath()          // ← 写入面直接复用读面(含家目录回落)
	if path == "" {
		return errNoHermesHome
	}
	...
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)   // ← 原地覆盖用户手写的 YAML
}
```

原注释（**撒谎**）：

```
// 会话创建那一刻读取。daemon 在用户于手机上显式选择强度时写该键 —— 与用户
// 手动编辑 config.yaml 等价(hermes 官方配置面,%HERMES_HOME% 是 hermes
// 自己的家,与 ~/.codex 等只读红线无关)。
```

原错误文案（**与事实不符**）：

```go
func (hermesHomeError) Error() string {
	return "hermes 家目录不可定位(HERMES_HOME 未设且无用户家)"   // 事实上"无用户家"从未被走到
}
```

### 1.2 现役性（不是潜伏分支）

- 生产 daemon `cmd/pocketagentd/main.go` **从不设置 `HERMES_HOME`**；全仓唯一 `os.Setenv("HERMES_HOME", …)` 在 `cmd/hermeffortprobe/main.go:38`（探针，且 `-home` 强制必填）。
- 调用点 `daemon/internal/agent/hermes.go:328` → 用户在手机上点强度就触发。

### 1.3 零覆盖（犯事的那条路恰好是没测的那条）

`config_test.go` 的 `setHermesHomeForTest` 每个用例都 `t.Setenv("HERMES_HOME", t.TempDir())`，家目录回落分支**一次都没被执行过**。

### 1.4 「改前」行为实证（变异探针：把 `path := WritableConfigPath()` 还原成 `path := ConfigPath()`）

用户家目录指向 `t.TempDir()`、预置一份用户自己写的 `~/.hermes/config.yaml`：

```
=== RUN   TestSetReasoningEffortRefusesToWriteUserHome
    config_test.go:135: 【自证】HERMES_HOME="" → 写面 WritableConfigPath()=""(空=无合法落点),读面 ConfigPath()="…\.hermes\config.yaml"
    config_test.go:137: 【自证】调用前用户家树: [.hermes/config.yaml]
    config_test.go:140: 【自证】调用后 err = <nil>            ← 静默成功
    config_test.go:141: 【自证】调用后用户家树: [.hermes/config.yaml]   ← 用户配置被原地改写
--- FAIL: TestSetReasoningEffortRefusesToWriteUserHome (0.01s)
```

**判决**：`err = <nil>` —— 改前不仅写了用户家目录，还**谎报成功**，这同时踩了红线 2 与红线 5。

---

## 2. 四项改动（改前 / 改后对比）

### 改动 1 — 新增「可写路径」解析，落点规则与读面解耦

| | 改前 | 改后 |
|---|---|---|
| 存在性 | 无（写面直接复用 `ConfigPath()`） | 新增 `WritableConfigPath()` |
| 规则 | `HERMES_HOME` → 否则 `~/.hermes` | `HERMES_HOME` 非空 → `<HERMES_HOME>/config.yaml`；**否则空串，绝不回落家目录** |

```go
// WritableConfigPath 返回 daemon **允许写**的 config.yaml 绝对路径:
// 只认 %HERMES_HOME%(适配器同款语义)。未设/空白 → 空串,调用方必须当失败
// 处理,严禁改回落用户家 ~/.hermes(红线 2:daemon 只读不改用户桌面配置)。
func WritableConfigPath() string {
	if home := strings.TrimSpace(os.Getenv("HERMES_HOME")); home != "" {
		return filepath.Join(home, "config.yaml")
	}
	return ""
}
```

**读面 `ConfigPath()` 一行未改**（家目录回落完整保留）—— 读是红线允许的，status/查询面需要看用户真实配置。

### 改动 2 — 写入面换用可写路径

| | 改前 | 改后 |
|---|---|---|
| `SetReasoningEffort` 取路径 | `path := ConfigPath()` | `path := WritableConfigPath()` |
| `HERMES_HOME` 未设 | 静默写 `~/.hermes/config.yaml` | `return errNoHermesHome`，**零字节落盘** |

### 改动 3 — 错误文案改成准确 + 诚实透出（`agent/hermes.go`）

| | 改前 | 改后 |
|---|---|---|
| 错误文案 | `hermes 家目录不可定位(HERMES_HOME 未设且无用户家)` | `HERMES_HOME 未设置,daemon 不改写用户家目录配置(~/.hermes),未固定强度(设 HERMES_HOME 指向 hermes 安装目录即可)` |
| 调用方行为 | `log.Printf` 后**继续走**（用户请求的 effort 被静默忽略，手机端零感知） | 日志留痕 + `sink(CodexEvent_STATUS)`，`Code = "reasoning_effort_not_pinned"`，`Message = "未固定为 <e>:<原因>(本轮沿用 hermes 现值)"` |

```go
log.Printf("[hermes] 强度 %s 未固定(写 config.yaml 失败,本轮沿用 hermes 现值): %v", e, werr)
sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
	Kind:    pb.CodexEvent_STATUS,
	Code:    "reasoning_effort_not_pinned",
	Title:   "hermes · 思考强度",
	Message: fmt.Sprintf("未固定为 %s:%v(本轮沿用 hermes 现值)", e, werr),
}}})
```

**说明**：`effortTiming` 那条成功分支的字段位置（`CodexEvent` 的 `Code/Title/Message`）经代码确认即手机端事件面板的渲染字段，未新增 proto 字段、未改 `proto/**`（避开甲级禁区）。新 `Code` 无需双端白名单——全仓 `reasoning_effort_applied` 只在 daemon 侧出现 2 次，Android/Flutter 按 `Kind=TITLE` + `Message` 通用渲染。

### 改动 4 — 注释改成真的

| 位置 | 改前（撒谎） | 改后 |
|---|---|---|
| `config.go:9-19` 文件头 | 「与 `~/.codex` 等只读红线**无关**」（而代码恰恰回落家目录） | 新增「读放宽 / 写收紧 / 诚实缺席」三段式说明，明确写只认 `%HERMES_HOME%` |
| `config.go:21-24` `ConfigPath` 文档 | 未区分读写面 | 标注「**只读面专用**……写入面用 `WritableConfigPath`,两者落点规则不同,不要混用」 |
| `config.go` `SetReasoningEffort` 文档 | 未提落点限制 | 标注「**只写 `%HERMES_HOME%` 下的 config.yaml**……错误不吞:调用方须如实透出」 |
| `locate.go:11-15` 包文档 | 「daemon **只读不改**用户家配置」—— 改前是假的 | 保留该行（改后为真），并追加「**"只读"的确切口径**」段落，把读/写两条规则分开说清 |
| `agent/hermes.go:308-318` 段注释 | 「与手动编辑 config.yaml 等价」隐含可写用户家 | 改为「只写 `%HERMES_HOME%` 下的 config.yaml(hermes 自己的家)……绝不回落改写用户家 `~/.hermes`」 |

同包其它注释一并核查：`config.go` 的 `validReasoningEfforts` / `IsValidReasoningEffort` 注释经复核**属实**（只讲白名单，不涉落点），未动。

---

## 3. 测试判决证据

### 3.1 新增用例（`daemon/internal/hermesapp/config_test.go`，6 个）

| 用例 | 钉住什么 |
|---|---|
| `TestSetReasoningEffortRefusesToWriteUserHome` | **红线判决**：家目录指临时目录 + `HERMES_HOME` 未设 → 必须报错；家目录树**前后逐条相同**；用户配置**内容逐字节未变**；错误文案须含 `HERMES_HOME` 与 `~/.hermes` |
| `TestSetReasoningEffortLeavesNoTmpWhenRefused`（2 子例） | **不残留 `.tmp`**：空家目录场景整树零条目（含目录也不许建）；已有 `~/.hermes/config.yaml` 场景除该文件外无增删、无 `config.yaml.tmp` |
| `TestConfigPathStillFallsBackToUserHomeForRead` | **读回落仍在**：钉住「读放宽」，同时断言同一时刻 `WritableConfigPath()==""` |
| `TestWritableConfigPathUsesHermesHome` | `HERMES_HOME` 已设时读写两面同址（写正常路径不回归） |
| `TestWritableConfigPathRejectsBlankHermesHome` | 空白串 = 未设，不得当成合法家目录 |
| `TestSetReasoningEffortLeavesNoTmpOnSuccess` | 成功路径 rename 消费掉临时文件，沙箱里只有 `config.yaml` |

跨平台：`sandboxUserHome` 同时设 `USERPROFILE`（Windows `os.UserHomeDir()`）与 `HOME`（类 Unix），保证判决力不只落在一种平台。

### 3.2 新增用例（`daemon/internal/agent/hermes_effort_status_test.go`，1 个）

`TestHermesEffortUnpinnedReasonReachesClient`：跑完整一轮带强度请求的 `Run`（假客户端），断言

- 必发一条 `CodexEvent_STATUS`（`Title = "hermes · 思考强度"`）；
- `Code == "reasoning_effort_not_pinned"`；
- `Message` 同时含 `未固定` / `HERMES_HOME` / `现值`，且**不含 `已设为`**（不许谎报成功）；
- 用户家目录（临时目录）**零条目**。

### 3.3 变异验证（证明断言不是摆设）

| 变异 | 结果 |
|---|---|
| `path := WritableConfigPath()` → `path := ConfigPath()`（还原违规） | `TestSetReasoningEffortRefusesToWriteUserHome` FAIL（`err = <nil>`）、`TestWritableConfigPathRejectsBlankHermesHome` FAIL（`open …\.hermes\config.yaml.tmp: The system cannot find the path specified`） |
| `agent/hermes.go` 去掉失败分支的 `sink(...)`（还原静默） | `TestHermesEffortUnpinnedReasonReachesClient` FAIL（`写不进去也必须发一条 STATUS 如实透出(不许静默),实际 1 帧`） |

两次变异均已回退，最终代码为修复态（`gofmt -d` 确认 `config.go` / `locate.go` 无格式偏差）。

### 3.4 用例数（不许跌）

用 `go test -json` 精确统计**顶层用例**（`-skip` 排除新增用例 = 改前口径，同一套代码、同一测量方式）：

| 包 | 改前 | 改后 | 失败 | 跳过 |
|---|---|---|---|---|
| `internal/hermesapp` | 39 | **45**（+6） | 0 → 0 | 0 → 0 |
| `internal/agent` | 708 | **709**（+1） | 0 → 0 | 9 → 9 |

既有 3 个写入用例（`CreatesAgentSection` / `ReplacesExistingKey` / `EmptyRemovesKey`）**未删未弱化**，改前改后全绿。

---

## 4. 验证输出

```
$ cd G:\项目1\pocketagent\daemon
$ go build ./...
BUILD_EXIT=0

$ go vet ./internal/hermesapp/... ./internal/agent/...
VET_EXIT=0

$ go test -count=1 ./internal/hermesapp/... ./internal/agent/...
ok  	github.com/pocketagent/daemon/internal/hermesapp	3.347s
ok  	github.com/pocketagent/daemon/internal/agent	17.639s
TEST_EXIT=0

$ go test -race -count=1 ./internal/hermesapp/...
ok  	github.com/pocketagent/daemon/internal/hermesapp	4.856s
RACE_EXIT=0

$ go test -count=1 ./internal/...          （daemon internal 全量）
ok_packages=27 FAIL_blocks=0
```

### 手工自证（`-v` 实测输出，判定用例自带 `t.Logf` 留痕）

```
$ go test -count=1 -v -run "TestSetReasoningEffortRefusesToWriteUserHome|TestSetReasoningEffortLeavesNoTmpWhenRefused" ./internal/hermesapp/
=== RUN   TestSetReasoningEffortRefusesToWriteUserHome
    config_test.go:135: 【自证】HERMES_HOME="" → 写面 WritableConfigPath()=""(空=无合法落点),读面 ConfigPath()="…\TestSetReasoningEffortRefusesToWriteUserHome2653027266\001\.hermes\config.yaml"
    config_test.go:137: 【自证】调用前用户家树: [.hermes/config.yaml]
    config_test.go:140: 【自证】调用后 err = HERMES_HOME 未设置,daemon 不改写用户家目录配置(~/.hermes),未固定强度(设 HERMES_HOME 指向 hermes 安装目录即可)
    config_test.go:141: 【自证】调用后用户家树: [.hermes/config.yaml]      ← 与调用前逐条相同
--- PASS: TestSetReasoningEffortRefusesToWriteUserHome (0.01s)
=== RUN   TestSetReasoningEffortLeavesNoTmpWhenRefused
--- PASS: TestSetReasoningEffortLeavesNoTmpWhenRefused (0.02s)
    --- PASS: TestSetReasoningEffortLeavesNoTmpWhenRefused/空家目录不留痕 (0.00s)
    --- PASS: TestSetReasoningEffortLeavesNoTmpWhenRefused/已有用户配置无残骸 (0.01s)
PASS
ok  	github.com/pocketagent/daemon/internal/hermesapp	1.796s
```

### `git status --short`（本波我改的文件，全部在许可清单内）

```
 M daemon/internal/agent/hermes.go
 M daemon/internal/agent/hermes_effort_status_test.go
 M daemon/internal/hermesapp/config.go
 M daemon/internal/hermesapp/config_test.go
 M daemon/internal/hermesapp/locate.go
```

> 全仓 `git status` 另有 `AGENTS.md` / `AUDIT.md` / `DEVELOPING.md` / `tools/**` / `daemon/internal/server/**` / `daemon/internal/agent/{codexsessions.go,pi_delete.go}` 等改动 —— 均为**其它并发子会话的在途工作**，非本波产物，本波未触碰。

---

## 5. 行为变更与影响面

### 5.1 谁会受影响

**只有 `HERMES_HOME` 未设的 hermes 部署**。生产 daemon 默认不设该变量（见 §1.2），因此这类部署**普遍**会命中新语义。`HERMES_HOME` 已设的部署（如 `cmd/hermeffortprobe` 沙箱、hermes-npm 安装时把该变量指向安装目录的形态）行为完全不变。

具体变化：
1. **不再改写** `~/.hermes/config.yaml`。用户手工维护的 `agent.reasoning_effort` 原样保留。
2. 手机端选择思考强度后，该轮**不再被静默忽略**——事件面板（`hermes · 思考强度`）会出现一条如实说明：**「未固定为 `<档位>`：HERMES_HOME 未设置,daemon 不改写用户家目录配置(~/.hermes),未固定强度(设 HERMES_HOME 指向 hermes 安装目录即可)(本轮沿用 hermes 现值)」**。该轮对话按 hermes 当前配置值正常跑完。
3. `HERMES_HOME` 未设时，daemon 与用户在 hermes CLI 侧设置的强度**不再联动**（这是隔离的代价，也是红线 2 要求的必然结果）。

### 5.2 如何恢复原能力

设置 `HERMES_HOME` 指向 hermes 安装目录（`%HERMES_HOME%\config.yaml` 即 daemon 唯一写入落点），此后强度固定恢复可用。`cmd/hermeffortprobe` 一直用 `-home` 走的就是这条路径，可作参照。

### 5.3 版本纪律

**未**擅自改 `CHANGELOG.md` / 版本号 / `DEVELOPING.md` 真值锚点 —— 按指令交主会话统一处理。本波属**用户可见行为变更**，发布说明应至少含：「daemon 不再改写 `~/.hermes/config.yaml`；未设 `HERMES_HOME` 时强度选择不再固定并在事件面板如实说明原因」。

---

## 6. 待决项（交主会话裁决）

1. **`agent/text_generator.go:107` 注释现在也是假的**（本波无修改权限，未动）：
   ```
   // 【effort 入参显式忽略】hermes 的思考强度没有运行时 RPC,唯一写入点是用户家
   // 目录 config.yaml(Run 的 SetReasoningEffort 段)—— 一次性优化轮绝不能顺手改
   // 用户桌面配置(红线 2),…
   ```
   修复后写入点已是 `%HERMES_HOME%\config.yaml`，不再是「用户家目录」。**建议**顺手改成 `唯一写入点是 %HERMES_HOME% 下的 config.yaml`（纯注释，零行为风险）。

2. **`cmd/hermeffortprobe/main.go:10-11` 的红线声明已过时**（同域 cmd 包，不在本波许可范围）：仍写「**绝不碰 hermes 家目录**(见 HERMES_HOME)」——现在 daemon 侧有同款机器判决，探针的 `-home` 强制沙箱可从「唯一防线」降级为「便利」。

3. **非法档位分支仍是纯 log**（`agent/hermes.go` 的 `IsValidReasoningEffort` 为 false 那一支）。本波只按指令处理了「写失败」的透出。但同属「用户请求被静默忽略」，按诚实缺席纪律应对称处理。**未自行扩范围**，请裁决是否补一条 `reasoning_effort_rejected` STATUS。实际触发概率低（手机端强度胶囊按 `AvailableEfforts` 渲染）。

4. **P1-5（无锁 read-modify-write + 固定 `.tmp` 名）未处理**：`config.go:99-103` 仍是 `path+".tmp"` 固定名 + 无锁。写面收窄到 `%HERMES_HOME%` 后危害面变小（不再覆盖用户手写文件），但同一 hermes 家目录下并发轮次仍可能互相踩。审查报告建议随 P0-1 一并根治（`CreateTemp` + 互斥），本波未做，留给主会话排期。

5. **上游读口未核实**：审查报告 §1.3 提到「若产品坚持跟随用户在 hermes CLI 的设置，应走 hermes 自己的配置面」。本波**未核实** hermes 上游是否提供读 `config.yaml` 的官方接口（上游源码在 `runtime/hermes-agent/`，本次未审）。当前读面 `ConfigPath()` 是「按路径猜 + 自己解析」，不是官方读口。

6. **无正当产品理由推翻设计的证据**：本轮在授权范围内检索了 `~/.hermes` 作为「daemon **应当**管理的目录」的产品理由 —— `locate.go` 包注释、`cmd/hermeffortprobe` 的强制沙箱、`config.go` 自述「`%HERMES_HOME%` 是 hermes 自己的家」三处独立自证均指向相反方向，**未发现支持回落的产品理由**，故按既定设计实施，未推翻。

---

## 7. 文件清单

| 文件 | 类型 | 说明 |
|---|---|---|
| `daemon/internal/hermesapp/config.go` | 改（代码 + 注释） | 新增 `WritableConfigPath()`；`SetReasoningEffort` 改用它；错误文案改准确；文件头/`ConfigPath`/`SetReasoningEffort` 注释改真 |
| `daemon/internal/hermesapp/config_test.go` | 改（测试） | 新增 6 个判决用例 + `sandboxUserHome`/`treeFiles` 助手；既有 3 个写入用例未动 |
| `daemon/internal/hermesapp/locate.go` | 改（**仅注释**） | 包文档追加「只读的确切口径」段落 |
| `daemon/internal/agent/hermes.go` | 改（代码 + 注释） | 写失败分支加 `sink` STATUS（`reasoning_effort_not_pinned`）；段注释改真 |
| `daemon/internal/agent/hermes_effort_status_test.go` | 改（测试） | 新增 1 个「未固定原因必须到达客户端」判决用例；文件头补红线说明 |

**未触碰**：`proto/**`、`android/**`、`flutter/**`、`tools/**`、`daemon/` 其余包、`docs/reviews/` 下既有报告、任何版本号 / CHANGELOG。未执行 `git add/commit/push`，未运行 `gofmt -w`。