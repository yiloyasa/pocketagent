# PocketAgent 0.2.2 真机审核 —— 缺陷清单

> 轮次：2026-09-19-device-build（自动可执行子集）
> 候选：APK `554c87b1…aae5`（238.13 MB, v0.2.2 / vc5）｜Daemon `f220d385…9b4c`（22.94 MB）
> Git：`8cc0c5e`（工作树脏 334 文件，完整 diff 见 `00-environment/git-diff.txt`）
> 设备：Xiaomi 2211133C / fuxi / Android 16 / HyperOS OS3.0 / 12 GB
> 证据根目录：`docs/verify/evidence/2026-09-19-device-build/`

---

## [PA-REAL-0001] 候选 Daemon 以「无鉴权」模式对外监听，任何局域网主机可无凭据驱动 agent

- 严重级：**P0 阻断**
- 状态：Confirmed
- 发现用例：SEC-03 / SEC-04 / SEC-05 / SEC-11 / SEC-12（并因此使这 5 项全部不可测）
- 发现时间：2026-09-19 22:44
- APK SHA-256：`554c87b1359de44603bf04744a1bfd06a62ef1cb7ea0e3759c75a74bbd16aae5`
- Daemon SHA-256：`f220d385321a2ecf5702d6a6fb63c452abb2800d97f1e7dd9dae5c6624cf9b4c`
- Git commit：`8cc0c5e8f95a0da807b3d68823bc970b8c1e767e`
- 设备与系统：Xiaomi 2211133C (fuxi) / Android 16 / HyperOS OS3.0.308.0.WMCCNXM
- Agent 与版本：Codex 0.155.1 / Pi 0.85.1 / ZCode（BigModel Individual Coding Plan）
- 网络通道：局域网直连（PC 10.15.50.160:7246 ← 手机 10.15.8.248）
- 复现概率：必现

### 前置条件

Daemon 按当前生产方式启动（不带 `-pair-code`）。

### 复现步骤

1. 观察在跑 daemon 的命令行。
2. 调用 `bridgehealth` 读取 `auth_required`。
3. 列出 daemon 数据目录内容。

### 预期结果

按方案 §8.11，Daemon 应默认要求配对/令牌鉴权：错误配对码被拒（SEC-03）、每设备独立 token（SEC-04）、撤销后立即拒绝（SEC-05）、过期/跨线程审批被拒（SEC-11）、审批重放只接受一次（SEC-12）。

### 实际结果

```
进程命令行:  pocketagentd.exe -addr 0.0.0.0:7246 -workspace G:\项目1\pocketagent
             （无 -pair-code）
bridgehealth: HOST version=0.2.2 tls_pin=verified auth_required=false
数据目录 C:\Users\yanzi\.pocketagentd\: 仅 certs/ event.jsonl model-pref.json threads.json
             —— 无 devices.json（从未注册过任何设备）
```

`daemon/cmd/pocketagentd/main.go:40` 定义 `-pair-code` 默认空值，语义为「为空则不启用鉴权」；`internal/server/server.go:203` 的 `authorized()` 在 `cfg.PairCode == ""` 时**直接 `return nil`**；`internal/server/host.go:188` 的 `Authorize()` 同样直接返回空 token 并成功。同时监听地址为 `0.0.0.0:7246`（全接口）。

结论：当前候选配置下，**同一局域网内任意主机**无需任何凭据即可调用 Thread/Terminal/FileSystem/Git/Model 全部服务，等价于对用户电脑的远程代码执行面。

### 影响

- SEC-03 / SEC-04 / SEC-05 / SEC-11 / SEC-12 在本轮**无法按预期口径执行**（鉴权路径整体未被激活）。
- 直接命中方案 §3.2 P0 定义中的「凭据泄漏/权限越界」类别与 §15 第 10 条「不扩大宣传」。
- 注：TLS 本身正常（SEC-01 PASS），但 TLS 只保护传输，不提供客户端身份认证；TOFU 指纹固定保护的是**客户端**不被冒充，不能阻止**其他客户端**连入。

### 证据

- 截图/录屏：—
- App 日志：—
- Daemon 日志：`daemon/daemon.log`（启动行 `pocketagentd 已启动: 0.0.0.0:7246 | … | 鉴权: false`）
- 网络/性能记录：`03-network-recovery/probe-bridgehealth.txt`、`03-network-recovery/net-baseline-connections.txt`

### 初步定位

代码具备完整鉴权能力（`devices.go` 的 `RegisterByPairCode`、挑战应答、每设备 token、`auth_p0p2_test.go` 已有测试），**问题在部署配置**：默认值 `""` 即关闭鉴权，且缺少「未配置配对码时拒绝非回环来源」的兜底。

### 修复说明

建议二选一（需产品裁决）：

1. **改变默认**：`-pair-code` 未显式提供时，启动即生成随机配对码并打印到控制台/写入 `devices.json`，未配对来源一律拒绝；保留 `-insecure-no-auth` 作为显式降级开关并打印告警。
2. **限制来源**：鉴权关闭时仅允许回环/`adb reverse` 来源（即 `127.0.0.1`），对非回环连接一律拒绝并提示「请设置 -pair-code 后再从局域网连接」。

### 回归范围

SEC-01〜SEC-07、SEC-11〜SEC-12；NET-11、NET-12；以及所有依赖「已配对设备」的用例。

### 回归结果

待修复后重跑（当前 BLOCKED）。

---

## [PA-REAL-0002] 模型列表首次调用 30 秒超时

- 严重级：**P1 严重**
- 状态：Confirmed
- 发现用例：PERF-06（模型列表首次刷新）；关联 SET-01、AGT-13
- 发现时间：2026-09-19 22:44
- Daemon SHA-256：`f220d385…9b4c`
- 网络通道：本机 127.0.0.1（TLS）
- 复现概率：1 次中出现 1 次（首调用）；随后连续 3 次均 8 ms 返回

### 复现步骤

1. 确认 daemon 处于新启动、codex 侧尚未完成初始化的状态。
2. 立即执行 `modelprobe.exe`（内部 `ListModels`，超时 30 s）。

### 预期结果

PERF-06：模型列表首次刷新，无原生冷启动 ≤ 3 s；需启动原生进程时 ≤ 10 s **且必须有加载状态**。

### 实际结果

```
第 1 次: [1] ListModels -> (30000ms, err=rpc error: code=DeadlineExceeded, n=0)
         [4] ListCodexCapabilities -> err=rpc error: code=DeadlineExceeded
第 2 次: [1] ListModels -> (8ms, err=<nil>, n=40)
第 3 次: [1] ListModels -> (8ms, err=<nil>, n=40)
第 4 次: [1] ListModels -> (8ms, err=<nil>, n=40)
```

同批探针中 `verifyext` 耗时 52.47 s（其中 `GetCodexUsage` 报 `account/rateLimits/read: context deadline exceeded`），`piprobe` 耗时 97.17 s。

### 影响

冷启动窗口内进入设置页/模型选择，用户会看到 30 s 无响应；且失败被报成 `DeadlineExceeded` 而非「正在初始化，请稍候」的可重试态，与方案 §10.2「UI 显示成功但原生未生效」相邻的体验缺陷。

### 证据

- `02-routing-models/probe-modelprobe.txt`（第 1 次调用原始输出）
- `02-routing-models/probe-verifyext.txt`、`02-routing-models/probe-piprobe.txt`
- Daemon 日志：`daemon/daemon.log` 22:24 段

### 初步定位

与既有结论一致：`withClient` 在 codex 初始化期间持锁，只读调用落入 `ErrBusy`/超时窗口。已实现的 `withStatusClient(statusBudget=10s)` 覆盖了部分只读调用，但 `ListModels`/`ListCodexCapabilities` 的**首次冷调用**仍会吃满 30 s 客户端超时。

### 修复说明

待定。方向：把 `ListModels` 明确归入 `withStatusClient` 预算内，并在超时时返回「初始化中，可重试」的结构化诊断而非 `DeadlineExceeded`。

### 回归范围

PERF-06、SET-01、SET-02、SET-03、AGT-13，以及跨 Agent 能力探测（见 PA-REAL-0003）。

### 回归结果

待修复后重跑。

---

## [PA-REAL-0003] 跨 Agent 能力探测被 Codex 初始化阻塞

- 严重级：**P1 严重**
- 状态：Confirmed
- 发现用例：SET-02 / SET-03（Pi 档位）、AGT-14（ZCode 模型）、PERF-06
- 发现时间：2026-09-19 22:42
- Daemon SHA-256：`f220d385…9b4c`
- 复现概率：必现于「codex 初始化进行中」这一窗口

### 复现步骤

1. 触发 codex 侧初始化（重启 daemon 后首次调用能力类 RPC）。
2. 在同一窗口内调用 Pi 与 ZCode 的模型/档位探测。

### 预期结果

按方案 §2.2 与 §8.2，各 Agent 的能力读取应彼此独立；同 provider 串行排队时必须「状态真实、不丢失」。

### 实际结果

Daemon 日志逐字：

```
2026/09/19 22:42:42 [pi] get_available_models 失败: get_available_models: 等待 pi 应答超时(30s)
2026/09/19 22:42:48 [zcode] 模型列表探测失败: codex 正忙(另一个操作正在初始化/探测),请稍后重试
```

即 **ZCode 的模型列表因为 codex 忙而失败**——这是跨 Agent 的锁争用，不是同 provider 排队。

### 影响

用户在 Codex 冷启动期间打开 ZCode/Pi 的模型或档位选择，会看到与本 Agent 无关的错误原因（"codex 正忙"），误导排障；也直接造成上面 PA-REAL-0002 的表象。

### 证据

- Daemon 日志（`daemon/daemon.log`，2026/09/19 22:42:42 与 22:42:48 两行）
- `02-routing-models/probe-zcmodelprobe.txt`（同一窗口内该探针另做环境修补后成功，说明 ZCode 本身可用）

### 初步定位

各 provider 之外共享了一把「初始化/探测」互斥锁，锁的持有者未按 provider 区分。

### 修复说明

待定。方向：把该互斥锁按 provider 分片（或改为「仅阻塞同 provider 的初始化」），保证 codex 的冷启动不阻塞 Pi/ZCode 的能力读取。

### 回归范围

SET-02、SET-03、SET-04、AGT-14、AGT-15、PERF-06。

### 回归结果

待修复后重跑。

---

## [PA-REAL-0004] 聊天页发送/语音/附件按钮触控区域远小于 48dp

- 严重级：**P2 一般**
- 状态：Confirmed
- 发现用例：APP-07（字体/显示大小与触控区域）；关联 APP-04
- 发现时间：2026-09-19 23:02
- 设备：1080×2400，**440 dpi（换算比 2.75，dp = px / 2.75）**
- 复现概率：必现

### 复现步骤

1. 首页 →「新建任务」→ 选工作区 →「创建」，进入聊天页。
2. `uiautomator dump` 读取输入区节点 bounds。

### 预期结果

APP-07：按钮触控区域不小于 48dp。

### 实际结果

| 控件 | bounds (px) | 尺寸 (px) | 折算 (dp) |
|---|---|---|---|
| 发送 | [945,2254]–[995,2304] | 50×50 | **18.2 × 18.2** |
| 语音输入 | [824,2254]–[874,2304] | 50×50 | **18.2 × 18.2** |
| 添加附件 | [78,2252]–[133,2307] | 55×55 | **20.0 × 20.0** |
| 切换模型与思考强度（整行） | [33,2120]–[1047,2224] | 1014×104 | 368.7 × **37.8** |

四个控件的**高度**均低于 48dp；且输入 325 字符后 bounds 完全不变（尺寸稳定、未被文字挤压，这一点是好的）。

### 影响

手指点按容错低，误触率高；对大字体/无障碍用户更明显。有替代路径（可再试），故不升 P1。

### 证据

- `05-sessions/ui-405-app04-long-input.txt`（节点全量 dump 含 bounds）
- `05-sessions/app04-app05-final.json`（含 `send_dp` / `mic_dp` 折算）
- `07-performance/device-density.json`（`Physical density: 440`）
- 截图 `05-sessions/ui-405-app04-long-input.png`

### 初步定位

输入区图标按钮的箱子按图标尺寸定，未使用 Compose 的最小交互尺寸（`Modifier.minimumInteractiveComponentSize()` / `IconButton` 默认 48dp 箱）。

### 修复说明

待定。给发送/语音/附件三个按钮加最小 48dp 的触控箱（视觉图标可保持 18–20dp 不变）。

### 回归范围

APP-04、APP-07、COM-06。

### 回归结果

待修复后重跑。

---

## [PA-REAL-0005] 引导页仍提示「下载语音模型（约 42MB）」，与包内已内置 SenseVoice 不符

- 严重级：**P2 一般**
- 状态：Confirmed
- 发现用例：VOI-10（无对外下载文案）；关联 VOI-05
- 发现时间：2026-09-19 22:39
- 复现概率：必现

### 复现步骤

1. 清数据全新安装候选 APK，首启走引导流程。
2. 阅读「语音输入（可选）」一屏文案。
3. 对照 APK 内实际资产。

### 预期结果

VOI-10：全 App 不得出现旧 Vosk 下载入口、旧模型管理按钮或**对外下载文案**。

### 实际结果

引导页原文（首启实测 dump）：

```
语音输入(可选)
点麦克风开始听写
识别的文字先进输入框,你确认后再发送,绝不自动发出
首次使用需下载语音模型(约42MB)
端侧离线中文识别,一次下载永久可用;「设置 → 语音识别」可查看进度、失败重试、删除释放空间
部分安卓 ROM 没有系统识别服务:此时会自动改用端侧模型,无需任何配置。
```

而 APK 内已内置模型（VOI-05 PASS）：

```
assets/sensevoice/model.int8.onnx  239,233,841 bytes  sha256 c71f0ce0…cd51  (与冻结值一致)
assets/sensevoice/tokens.txt           315,894 bytes  sha256 f449eb28…a1dc  (与冻结值一致)
```

设备上也确有解压产物：`files/sensevoice-int8/`（21:44 创建）。

### 影响

文案与事实相反：①说「需下载」而实际是「已随包安装、首次使用时解压」；②42MB 与包内 239MB（压缩后 165MB）量级不符。会造成用户预期错误与客服成本。未发现任何外部下载行为（无网络请求），因此是**文案缺陷**而非功能缺陷。

### 证据

- `01-install-startup/02-seq2.png` / `ui-*` 引导屏 dump（`01-install-startup/APP-01-ui-dump.txt` 同批）
- `06-voice/VOI-05-apk-assets.txt`（包内哈希）
- `00-environment/sensevoice-assets.txt`

### 初步定位

SenseVoice 改为随包内置后，引导页文案未同步更新（旧 Vosk 时代措辞残留）。

### 修复说明

待定。改写为「语音模型已随 App 安装，首次使用时自动释放所需文件（约 239MB）；设置 → 语音识别 可查看状态、重试与释放空间」。

### 回归范围

VOI-10、VOI-01、SET-06。

### 回归结果

待修复后重跑。

---

## [PA-REAL-0006] NET-01 实测未满足「USB 优先」验收标准（实现为 lastGoodAddr 优先）

- 严重级：**P2 一般**（属验收标准与实现设计的差异，需产品裁决）
- 状态：Confirmed
- 发现用例：NET-01；关联 NET-04、§15 第 7 条
- 发现时间：2026-09-19 22:49
- 复现概率：必现（已保存过局域网地址时）

### 复现步骤

1. 建立 `adb reverse tcp:7246 tcp:7246`（PC 侧 `reverse --list` 确认 `UsbFfs tcp:7246 tcp:7246`）。
2. 手机侧 `am force-stop` 后冷启动 App。
3. 用 **PC 侧 `netstat -ano` 的对端地址**判定实际通道（不看 UI 标签）。

### 预期结果

NET-01：优先使用 `127.0.0.1:7246`；连接标签为 USB。

### 实际结果

```
adb reverse(在场): UsbFfs tcp:7246 tcp:7246
冷启动 TotalTime=848ms
PC 侧连接: 10.15.50.160:7246 <- 10.15.8.248:43558  ESTABLISHED   (局域网)
USB(127.0.0.1) 连接: 无
App 标签: 已连接 · 局域网直连
```

即 **USB 反向隧道在场时仍走局域网**。对照源码：

- `AgentConnection.kt:59` 注释：`顺序即优先级:USB(adb reverse 反向隧道)→ 局域网直连 → Tailscale → 用户手填(兜底)`
- `AgentConnection.kt:402` 注释：`最近一次成功连接的地址,持久化;冷启动时排在候选队列首位`

两条规则冲突：`lastGoodAddr`（此处为局域网地址）被插到候选队列首位，压过了 USB。App 侧持久化的 `shared_prefs/agent_conn.xml` 亦印证（`addr` / `lanAddr` / `lastGoodAddr` 均为 `10.15.50.160:7246`）。

**说明（取证边界）**：本轮曾尝试删除 `agent_conn.xml` 复测，但 `run-as … rm` 未生效（删除后 listing 中该文件仍在），故复测结论无效，已在证据中标注。因此本条判定为「实测与验收标准不符 + 代码依据充分」，未宣称「USB 候选完全未被尝试」。

### 影响

功能上不受损（局域网可用且通常更快，NET-02/NET-05 均 PASS）。但用户插着数据线时若期望走最稳的 USB 通道，实际不会；验收标准与实现需对齐，否则该类用例永远 FAIL。

### 证据

- `03-network-recovery/net01-usb.txt`（第一轮）
- `03-network-recovery/net05-priority.json`、`03-network-recovery/net-baseline-connections.txt`
- `03-network-recovery/app-agent-conn-prefs.txt`（脱敏后的 App 连接偏好）
- `03-network-recovery/net01-usb-retest.json`（标注为无效复测）

### 初步定位

`lastGoodAddr` 插队策略与 `METHODS` 声明优先级不一致。

### 修复说明

待产品裁决，二选一：

1. 调整实现：USB 反向隧道实际可达时（例如 `127.0.0.1:7246` 快速握手成功）优先于 `lastGoodAddr`。
2. 调整验收标准：明确「冷启动沿用上次成功地址（含局域网），USB 仅在前者不可达时使用」，并同步修订方案 §8.6 NET-01 与 §15 第 7 条措辞。

### 回归范围

NET-01、NET-02、NET-04、NET-05、NET-06、NET-15。

### 回归结果

待裁决/修复后重跑。

---

## 附：非缺陷观察项（不计入缺陷，供发布决策参考）

| 编号 | 观察 | 建议 |
|---|---|---|
| OBS-01 | `android:usesCleartextTraffic="true"` 且无 `networkSecurityConfig`（见 `09-security/manifest-print.txt`） | 建议用 `networkSecurityConfig` 把明文限定在私有网段（局域网 dev server 预览场景），公网域一律禁明文；gRPC 主链路走 TLS+指纹固定，不受此项影响（SEC-01/SEC-16 已 PASS） |
| OBS-02 | 候选产物为 **debug 包**（`android:debuggable="true"`） | 本轮 SEC 相关结论**不可外推到 release 包**；发布前须对 release 包重跑 §8.11 全部用例 |
| OBS-03 | APK 238.13 MB：含 `model.int8.onnx` 239 MB（未压缩）+ 4 套 ABI 原生库（`arm64-v8a`/`armeabi-v7a`/`x86`/`x86_64`/`mips`）+ `libvosk.so`（约 8–9 MB × 4） | 建议删 `mips`/`mips64` 与不需要的 ABI、评估 `libvosk.so` 是否仍需随包；安装耗时 11.9 s（含 `extractNativeLibs=true` 解压）可一并优化 |
| OBS-04 | mDNS 在本机不可用：`UDP 5353 已被其他程序独占`，注册失败 | Daemon 已正确告警并降级（`WARNING: mDNS 自动发现未启用(不影响手动填地址连接)`），NET-15 行为符合预期；属**环境问题**，不作为缺陷。`probe-mdnsprobe.txt` 探测 0 实例与之一致 |
| OBS-05 | `libvosk.so` / `VoskModelManager.kt` / `VoskVoiceRecognizer.kt` 仍存在于包与源码中 | 与 PA-REAL-0005 相关；需确认 Vosk 是否仍作为系统识别不可用时的**第二**回退路径。若已由 SenseVoice 取代，建议清理 |
| OBS-06 | 「多agent协同」在首页明确显示「开发中:多 agent 协同尚未开放」 | 符合方案 §1 第 8 条「对未支持能力明确显示真实限制」，记为正面证据 |
| OBS-07 | `disable_paste_burst` 回读为 `state=unsupported`，并给出原因「当前 codex 版本无 disable_paste_burst 这个配置键」 | 符合方案 §1 第 3 条「显示诊断而非伪造成功」，记为正面证据 |
| OBS-08 | `dao/daemon/cmd/zcprobe` 查找 builtin provider 配置的路径为 `resources\glm\provider\` 与 `AppData\config\provider\`，而实际位于 `resources\config\provider\`；**探针**因此报 `write |1: The pipe is being closed`（生产 daemon 已通过环境修补解决，21:34 之后 0 次该错误） | 属**验证工具缺陷**，非产品缺陷；建议修正 `cmd/zcprobe` 的路径解析，使探针与 `zcmodelprobe` 口径一致，避免下次误判 |
