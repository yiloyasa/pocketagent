# PocketAgent — 项目整体上下文

> 手机上的 AI 编程代理遥控器。**手机只做遥控，agent 跑在你自己的电脑上，全程零云服务器。**
>
> **本文件是项目唯一的上下文文档** —— 功能、架构、模块、数据流、入口、配置、依赖、现状、阻碍、坑，全部收敛在这里。
> **新会话 / 协作者只读 §0 速览即可开工**；其余章节按需查阅，不必通读。
> 过程性文档（调研/计划/评审）已舍弃，台账见 `_archives/_docs-discarded_20260920-manifest.json`、`_archives/_cleanup_20260922-manifest.json`。

---

## 0. 速览（新会话从这里开始）

### 0.1 一句话与架构

手机 App 经 WiFi / 4G / Tailscale 连到你自己电脑上的 `pocketagentd`（Go daemon）；daemon 桥接原生 AI 编码 agent（codex / pi / zcode / dsh），把它们的流式事件、工具调用、审批请求翻译给手机，把用户的决定再翻译回去。daemon **不重写**原生 agent 的循环 —— 它只是翻译官。

```
手机 (Kotlin/Compose) ──gRPC/TLS──► 本机 daemon (Go) ──stdio JSON-RPC──► 原生 agent ──► 大模型
   遥控 / 显示 / 审批              翻译事件·中继审批·托管子进程        codex/pi/zcode/dsh
                                                                    (自有循环/沙箱/审批/MCP)
```
连通三档降级：**USB `adb reverse` → 局域网（含 mDNS 自动发现）→ Tailscale**，无云中继。

### 0.2 技术与规模

- 手机：Kotlin + Jetpack Compose（minSdk 26 / compileSdk 35）
- daemon：Go **1.27.1**（无 ORM / 无框架，依赖极简）
- 契约：protobuf / gRPC `pocketagent.v1` —— **7 服务 / 85 RPC**，8 proto / 2182 行
  （`proto/v1/*` 与 `android/app/src/main/proto/v1/*` 是两副本，**MD5 必须一致**）
- 代码：android main **121** `.kt` / test **141** `.kt`；daemon Go **540** 文件（生成 pb.go **15**，手写 **525** 含测试）
- 版本：**0.2.2 / versionCode 5**

### 0.3 当前状态（真值锚点 · 2026-10-01 整理轮）

| 项 | 值 |
|---|---|
| Git | 分支 `main`，**代码基线** `73dbf5f`（2026-10-01：连接层四连修 + **全面优化三轮×4并发**（R1 首页/列表/设置/抽屉 → R2 聊天/审批/交互组件/文件Git → R3 主题动效根层/协同/任务离线/Markdown 渲染，调研对标 ChatGPT App/M3 Expressive/Linear/NN/g）+ **历史导航深优三轮**（R1 实时刷选 requestScrollToItem / R2 日期上下文三路数据链接真 / R3 面板搜索+最早最新快跳））；工作树保持干净，当前提交以 `git log` 为准 |
| 连接层（真机日志实证修复） | ① `connectWithFallback` 探测移出 connectionLock（原锁内串行探测实测挂起全部调用方 **28.3s**，ART Long monitor contention 实证）+ 并行竞速恢复（serial=true 是 TOFU 阶段遗留，重连被拖到 23.5s）② 探活连续失败闸（单次失败不再撕含 WatchEvents 的健康通道，"26s/轮连接断"风暴根因）③ 探活补 `withWaitForReady`（回退临时通道带、共享探活不带 → "回退必成功探活必失败"自持循环）④ Tailscale 候选独立 15s 预算（DERP 冷启动 8-20s，7s 竞速预算会错杀唯一活路）⑤ 探活异常落日志（此前两端日志都无迹可寻） |
| 测试基线 | Android **1233 用例 / 0 失败 / 0 错误**（基线从 712 → 1233，两轮优化 +521）；daemon 全量包绿；RPC **7 服务/85 RPC** |
| 真机 | 手机 8e0e7bbe 已装 2026-10-01 版（211.9MB）并经 **Tailscale** 连通（校园网 AP 隔离下 LAN 不通，VPN 兜底链路实测可用）；历史导航实时刷选/面板搜索/快跳/分组均已真机截图验证（`G:\pa\pa-live*.png` / `pa-panel.png` / `pa-search.png`，不入 git） |
| **下一刀（已明确）★** | ① daemon `GetHistory` 填充 `created_at_unix_ms`（手机侧日期分组已就绪，当前全部诚实归「早前」）② MIUI 边缘手势拦截非确定性（x<24dp 拖动偶发被系统吞，40dp 触控带+gestureExclusion 已缓解；通用用户引导"稍靠右按压"）③ FileKind 枚举快照 Mcp 色不随换盘（改组合期读取）④ SettingsScreen statusLabel 收口四词口径 ⑤ BrowserScreen 离线快照 toast 带"截至 X" |
| 遗留小项（已核销或拍板） | ~~attachmentEntryHiddenFor 遗留壳~~（实为能力门控活代码，挂账有误）~~unknown_events.go 陈旧注释~~（实为诚实的"预留档"说明）~~InvalidateVersionCache~~（零调用真死代码，2026-10-01 已删）~~dshModelsBudget const→var~~（拍板：维持 const，无覆写需求） |

### 0.4 现阶段阻碍（按优先级）★

1. **真机依赖**：UI 交互与 gRPC 联调依赖手机在线（USB / 局域网 / Tailscale 三档）；离线时只能跑 JVM / go 单测。当前校园网 AP 隔离使 LAN 候选不可达，Tailscale 是实际主通道（手机侧 VPN 开启）。
2. **MIUI 边缘手势拦截**：手势导航设备上 x<24dp 的触摸会被系统非确定性吞掉（同一位置有时可达有时被拦）；已用 40dp 触控带 + systemGestureExclusion（系统上限 200dp）+ 负 offset 贴边把影响压到最低，但极左缘无法 100% 根除（系统设计如此）。
3. **codex 实验特性警告**：为支持 Default 模式反问卡，daemon 强注 `default_mode_request_user_input`，而 codex 自报其 `stage=underDevelopment` → **开启即弹警告**；抑制需写用户 `~/.codex/config.toml`（**红线封死**）→ 只能诚实降级（不支持就不显示入口）。
4. **dsh 凭据注入链**：缺 key 时报 `no API key for provider route "deepseek-official"`；云 LLM 真实委派尚未端到端验证。
5. **mDNS 广播受抢占**：UDP 5353 被常驻浏览器独占时（daemon 日志有 WARNING），手机无法 mDNS 发现主机；局域网直连需手动地址或重启 daemon。
6. **演示视频工作区**：`video/pocket-agent-showcase/`（Remotion）去留待用户决定。

### 0.5 铁律（红线，违反即打回）

- **绝不替用户做审批决定**（伪造 decision）；没能力的原生请求回 error，**绝不伪造**。
- **绝不改写用户桌面配置**：`~/.codex/config.toml`、`~/.dsh/`、`~/.pi/`、`~/.codex/` 一律只读。
- **契约两副本必须同步**（改一份 proto 必须同步另一份）。
- **不新增写死的名单 / 顺序 / 版本二分**（模型白名单、agent 顺序、`isLegacy*`）；未知输入走「透传」，UI 禁显「未知 / 错误」。
- **不留假绿**：测试必须 `--rerun` 并核验结果 XML 时间戳；产物自证（apkanalyzer / 二进制 grep 新符号）。
- **「源码改对 ≠ 跑着的是新的」** —— 报「没生效」前先自证产物。

### 0.6 文档地图

- **本文件（README）** = 项目整体上下文（唯一）。
- **`AUDIT.md`** = 唯一审核标准（甲乙丙丁分级 / L0–L2 三层 / 红线一票否决）。
- 审核入口脚本 `tools/audit/pa_audit.py`（六条规则，`--json` 机器可读）；真机驱动 `tools/device/`。
- 工作区根 `_archives/` = 冷数据归档（只搬不删）；`apk-analysis/` = MobileCodex 生态逆向对比（项目前身调研）。

---

## 1. 系统定位与功能面

**手机 = 遥控器**（显示与输入）；**daemon = 桥**（翻译事件、中继审批、托管子进程会话）；**原生 agent = 干活者**（agent 循环跑在 codex/pi/zcode/dsh 自己的进程里，daemon 不重写）。

**功能面**：会话聊天（流式回复 / 工具卡片 / 审批弹卡 / 思考过程）、斜杠命令、语音输入（系统识别优先 → 端侧 SenseVoice 兜底）、真终端（xterm.js ↔ PTY）、文件浏览 / 编辑 / 预览（Office / PDF 内置解析）/ 下载、Git 面板（状态 / diff / 提交）、会话组织（分组 / 搜索 / 归档 / fork / 回滚 / 导入原生会话）、用量与上下文环、任务中心、内置浏览器、离线缓存与断线续传（事件重放 + 审批恢复）、mDNS 自动发现、多设备配对与 token 续期、**多智能体协同（委派 + 扇出 + 协同角色）**、**App 缓存管理**。

---

## 2. 技术栈与第三方依赖

| 层 | 技术 | 关键第三方 |
|---|---|---|
| Android | Kotlin + Jetpack Compose（BOM 2024.09 / M3），minSdk 26 / target·compileSdk 35 | `grpc-okhttp`+`grpc-protobuf-lite` 1.66；`security-crypto`（Keystore 加密 token）；`webkit`（xterm.js 容器）；`navigation-compose`；`kotlinx-coroutines`；**sherpa-onnx 1.13.8 AAR**（SenseVoice 端侧识别，构建时下载校验）。**vosk-android 已整体摘除（2026-09-20）**：引擎选择收敛为「系统识别 → 端侧 SenseVoice」两级，共享错误码迁至 `VoiceErrors.kt` |
| daemon | Go **1.27.1** | `grpc` 1.83.2、`protobuf` 1.36.12、`go-pty`、`zeroconf`。**依赖极简，无 ORM / 无框架** |
| 协议 | protobuf / gRPC `pocketagent.v1` | **7 服务 / 84 RPC**：Thread 35、Git 10、Fs 9、Runtime 13、Model 9、Host 5、Terminal 3（8 proto / 2146 行） |
| 终端 | xterm.js 6（WebView）+ JS bridge | ↔ daemon `go-pty`，字节流一致画面一致 |

> 图标全用 lucide 矢量（`ic_lucide_*`）+ 品牌 PNG，**故意不引** `material-icons-extended`（曾占 DEX 一半）。

---

## 3. 目录结构与模块职责

```
pocketagent/
├── proto/v1/                 # gRPC 契约（唯一真源；android/app/src/main/proto/v1/ 是同步副本，MD5 必须一致）
├── daemon/                   # Go 桌面宿主
│   ├── cmd/pocketagentd/     # 入口：flag → TLS → 组装 → gRPC server
│   ├── cmd/<probe>*          # 一次性探针/工具（modelprobe、chatprobe、paguard、smoketest、collabrolecheck…）
│   └── internal/
│       ├── gen/              # protoc 生成的 Go 代码（勿手改）
│       ├── server/           # ★ gRPC 服务实现层（含 codex/pi 子代理存根登记·历史回填 / 委派引擎 / 协同扇出 / 协同角色）
│       ├── agent/            # ★ Provider 抽象 + 6 实现（codex/pi/zcode/dsh/openai_compat/echo）+ 反向请求分派
│       ├── codexapp/         # codex app-server 子进程封装（JSON-RPC 2.0 over stdio）
│       ├── piapp/            # pi --mode rpc 子进程封装（1 子进程=1 会话）
│       ├── zcodeapp/         # zcode.cjs app-server 封装 + 账户桥接 + 子代理工具识别
│       ├── dshapp/           # dsh --profile acp 子进程封装（隔离 HOME + 凭据文件注入 + 部署级 MCP patch）
│       ├── collabmcp/        # 内嵌 MCP 服务器 pocketagent-collab（agent_delegate / agent_status）
│       ├── stderrdrain/      # 子进程 stderr 统一收口（脱敏 Redact + 限流 Drain）
│       ├── store/            # FileStore：threads.json 持久化（会话/设置/协同角色）
│       ├── eventstore/       # events.jsonl 全局单调 seq 事件日志（10MB 轮转，断线重放）
│       ├── term/             # PTY 会话管理 + 空闲回收 reaper
│       ├── mdns/             # _pocketagent._tcp 局域网宣告（失败仅警告，不崩）
│       ├── tlscert/          # 自签证书（ECDSA P-256，20 年，首启生成复用）
│       ├── hidewin/          # Windows 隐藏子进程窗口
│       └── version/          # 版本号（可 -ldflags -X 烙印）
├── android/app/
│   ├── src/main/java/dev/pocketagent/app/
│   │   ├── MainActivity.kt        # 入口 + NavHost：onboarding/home/chat/{id}/terminal/browser?url=/settings/collab…
│   │   ├── AgentConnection.kt     # ★ 单例连接管理：地址/token/配对/挑战续期/并行竞速候选
│   │   ├── TokenStore / DeviceKey / PinTrustManager / ConnectionDiagnostics
│   │   ├── TurnKeepAliveService.kt # dataSync 前台服务（超时停服务不崩溃）
│   │   ├── ui/                    # 各屏（Chat/Terminal/Files/Git/Settings/Home/Threads/Browser/TaskCenter/Collab…）
│   │   │   │                       # + 逻辑类（TurnSession 会话状态机、HistoryMessageMapper、TurnDelivery、
│   │   │   │                       #   SlashCommandsLogic、ThreadOrganize、SubAgentsLogic、CollabState…）
│   │   │   └── component/         # 组件库（SplitButton/FileKind/PaDownloader/OfflineStore/ThreadSnapshot/
│   │   │                           #   MdnsDiscovery/SenseVoice*/VoiceErrors…）
│   │   └── src/main/proto/v1/     # 契约副本（与 proto/v1/ 同步）
│   ├── src/main/assets/           # sensevoice 模型（229MB，构建时下载）+ terminal.html
│   ├── libs/（app/ 下，非 src/main）# sherpa-onnx AAR（构建时下载校验）
│   └── src/test/                  # JVM 单测 81 文件
├── codex-schema/             # codex app-server 官方 JSON Schema 快照（能力核对，勿手改）
├── tools/                    # make_icons 等资源生成 + device/ 真机工具 + ops/ 运维 + audit/ 审核
├── dist/windows/             # 桌面端分发包（build.ps1 → zip + install/uninstall）
├── docs/verify/              # 验收证据（evidence/ 结论入库、截图不入库）+ testdata
└── build/                    # APK 产物（gitignored）
```

### daemon 侧核心职责

| 模块 | 职责 | 实现要点 |
|---|---|---|
| `cmd/pocketagentd` | 启动组装 | flag：`-addr`(默认 :7246)、`-data`(默认 `~/.pocketagentd`)、`-pair-code`(**不传=自动生成 8 位随机码**并持久化 `pair-code.txt`；**鉴权默认开启**，PA-REAL-0001 已修)、`-insecure-no-auth`(显式关鉴权，**仅放行本机回环**)、`-workspace`、`-plaintext`(应急)、`-mdns`、`-term-idle`(默认 10m)、`-pi-collab`(默认开)、`-collab-max-parallel`(默认 3)。keepalive 25s/10s |
| `server` | 6 个 gRPC 服务 + 鉴权拦截器 | `server.go` `checkAccess()`：默认验 token（每设备 token / legacy = sha256("pair-code:"+code)）；PairCode 为空 fail closed；`-insecure-no-auth` 仅放行回环 peer。`thread.go` `StartTurn` 双向流是心脏；`turn_queue.go` 运行中插话（codex 支持 steer，其余入内存队列终局后续跑）；`turn_transport.go` 背压：**同步顺序落盘 eventstore + 单写者 gRPC writer**；`approval_resume.go` 断线后按 thread_id/request_id 补交审批（过期/重复/跨线程拒收，上限 2 分钟）；`collab_session.go` 协同扇出；`collab_roles.go` 协同角色；`thread_delete_tombstone` 墓碑防复活 |
| `agent` | Provider 抽象 | `provider.go` 定义 `Provider/Sinker/PrefsAware`；`DetectProviders()` 启动探测注册。5 实现：**codex**(主力)、**pi**、**zcode**、**dsh**、openai_compat(唯一直连自跑循环)、echo(冒烟)。`codex_requests.go` **按方法名精确匹配**分派反向请求（审批应答枚举分新旧协议：`approved/denied` vs `accept/decline`，**绝不伪造 decision**） |
| `codexapp` | codex 子进程 | `codex app-server` stdio JSON-RPC；自带 agent 循环/沙箱/审批/MCP/登录/会话——**我们只做翻译官** |
| `piapp` | pi 子进程 | `pi --mode rpc`；Thread ↔ 子进程 **1:1**；daemon 重启后按 SessionFile 重挂载（switch_session + get_state 校验握手不能省） |
| `zcodeapp` | zcode 子进程 + 账户 | 只读适配：读桌面宿主账户 overlay（family/profile→scoped key→权益→`provider/updateAccountConfig`），模型/档位全由 worker 返回**不造静态名单**；仅 Individual Coding Plan |
| `dshapp` | dsh 子进程 + MCP | 长驻 `dsh --profile acp` 承载多会话；隔离 HOME；`collabpatch.go` 读写隔离 HOME 的 `profiles/acp/cordis.patch.yml`（手写 yaml 生成，**不引 yaml 依赖**） |
| `collabmcp` | 内嵌 MCP | 工具层 → `GRPCEngine` → 回环 gRPC `DelegateSubTask` → `server.collabEngine`；`-collab-mcp` 是薄代理进程（不自持 store/引擎） |
| `store` | threads.json | 会话、默认工作区/模型/effort、协同角色（`CollabRoles`）；Thread 的 ExternalSession 落库供重启重挂载 |
| `eventstore` | events.jsonl | 每帧带全局单调 seq 落盘；手机重连 `ResumeFrom(seq)` 续传不丢不重 |

### android 侧核心职责

| 模块 | 职责 | 实现要点 |
|---|---|---|
| `AgentConnection` | 单例连接 | gRPC OkHttp channel；token 走 metadata；**并行竞速候选**（reverse→LAN→Tailscale 同时拨，先通先用）；配对携设备身份（UUID+P-256 公钥）领每设备独立 token；token 失效走**挑战应答续期**（Keystore 签 `device_id:challenge`） |
| `TokenStore`/`DeviceKey`/`PinTrustManager` | 安全 | token 经 EncryptedSharedPreferences（Keystore AES256-GCM）加密落盘，ROM 异常降级明文 prefs 不断连；证书指纹锁定，变化即阻断 |
| `TurnSession`/`TurnDelivery` | 会话状态机 | callbackFlow 收流（`.buffer(UNLIMITED)` 治丢帧）；流式已读文字定期落盘（文字+seq 原子保存）；恢复走**事件回放 + 原生历史兜底，绝不重发原 prompt** |
| `ChatScreen` 系 | 聊天 UI | 模型选择独立整行；语音引擎两级：系统识别优先 → SenseVoice 端侧兜底（模型打进 APK 首次解压校验） |
| `CollabState`/`Collab*Logic` | 多智能体协同 UI | 协同扇出（`DispatchCollab`）、协同角色（`CollabRoles`）、「多 agent 共同完成任务」专属页；无能力不显示假入口 |
| `SubAgentCard`/`SubAgentsLogic` | 子智能体呈现 | 内联子代理卡 / 组卡；`childrenOf` 按 `ParentThreadID` 分组 |
| `FileKind`/`DocFormatRegistry`/`XlsLegacy` | 文件预览 | 唯一判定源 `FileKind.preview`(IN_APP/SYSTEM/SAVE_ONLY)；自研 .xls OLE2+BIFF8 解析器；PDF 走系统 PdfRenderer |

---

## 4. 关键数据流

**① 一条消息的生命周期（正向）**
```
ChatScreen 输入 → AgentConnection stub(token metadata)
  → daemon gRPC [TLS + StreamAuth 拦截器]
  → server.StartTurn 双向流
      ├─ 轮次 running? → turn_queue: codex→turn/steer 原生插话; 其余→内存队列(Queued 回执)
      └─ providerForThread(agent_id) → agent.Provider.Run
            → codexapp/piapp/zcodeapp/dshapp 子进程 JSON-RPC
            → 原生 agent 执行（读文件/跑命令/改代码）
  ← 事件流沿 Sink 回传 → turn_transport（先同步写 eventstore 拿 seq，再单写者推 gRPC）
  ← 手机 callbackFlow → 逐字上屏 + 工具卡片 + 用量更新
```

**② 审批反向流**：原生 agent 发 `item/*/requestApproval` → `codex_requests.go` 精确匹配分派 → 转 `ToolCallBlock` 推给手机弹卡 → 用户决定经 `SubmitApproval`(thread_id+request_id) 回传 → daemon 翻译成对应协议的应答枚举。断网期间待审批保留，重连后可补交。

**③ 断线重放**：每帧事件带全局 seq 落 `events.jsonl` → 手机重连 `ResumeFrom(seq)` → daemon 从 eventstore 补发缺失帧 → 无丢无重。daemon 重启则从原生历史（session snapshot/resume）恢复终态。

**④ 会话挂载**：Thread.ExternalSession(SessionID/SessionFile) 落 threads.json → daemon 重启后按各 agent 方式重挂载（codex `thread/resume`；pi `switch_session`+`get_state`；zcode `session/resume`；dsh 隔离 HOME 内）——**不 resume 就静默吞请求**是三方共同教训。

**⑤ 多智能体协同（扇出）**：用户一条指令 → `DispatchCollab` → daemon 并行对等分发给 N 个 agent（**与委派的父子语义不同**）；RPC 只受理（快速返回 thread_id）不等结果；异步 goroutine 用**脱离取消链的新 context**（否则 RPC 一返回就杀掉刚起飞的轮次）；**不设 ParentThreadID**（协同 session id 非真实线程）；一家失败不拖垮别家。

---

## 5. 入口与配置

| 类别 | 位置 | 说明 |
|---|---|---|
| daemon 入口 | `daemon/cmd/pocketagentd/main.go` | 见 §3 flag 表；运行参数：`-addr 0.0.0.0:7246 -workspace G:\项目1\pocketagent`，env `CODEX_BIN=G:\Claude Code\codex.cmd`。⚠ **pi/dsh 的定位已改为按用户环境派生**（2026-09-23，`internal/hostenv`）：node 走 随包目录 → `%LOCALAPPDATA%`/`%ProgramFiles%` → PATH；pi/dsh 入口走 **npm 实际配置的全局根**（`.npmrc` 的 `prefix=` 或 `NPM_CONFIG_PREFIX`，其次才是 `%APPDATA%\npm` 等默认位置）→ PATH。本机 `.npmrc` 写着 `prefix=G:\Claude Code`，故 pi/dsh 均已实测命中、**无需重装**；仅在既无 env 覆盖、又装在别处时才须显式设 `PI_NODE`/`PI_CLI`（pi）或 `DSH_NODE`/`DSH_BIN`（dsh） |
| daemon 数据目录 | `~/.pocketagentd/` | `threads.json`(会话真值)、`events.jsonl`(+轮转)、`devices.json`(配对设备)、`pair-code.txt`(配对码，0600，重启复用不换码)、`certs/`(自签证书)、`dsh-home/`(dsh 隔离 HOME) |
| App 入口 | `MainActivity.kt` → NavHost | 冷启 Splash → onboarding(未配对)/home |
| App 构建 | `android/app/build.gradle.kts` | versionCode/versionName 与 daemon `internal/version` 同步（现 0.2.2/5）；sherpa AAR 由构建脚本下载+哈希校验 |
| 构建脚本 | `tools/device/_pa_gradle.py`、`android/build-ascii.ps1` | 中文路径 Gradle 处理不了 → robocopy 镜像到 `G:\pa\android` 再 gradlew |
| 看门狗 | `daemon/cmd/paguard` | 计划任务 PocketAgentDaemonGuard 每分钟**直跑 paguard.exe**（windowsgui 子系统物理无窗口）→ 探测 7246 死了才以同参数拉起 daemon（4s 内拉起）。旧链「计划任务 → wscript → watchdog_hidden.vbs → WMI → paguard」已于 2026-09-25 弃用（`6077295`：wscript 每分钟调度触发 VBScript 弃用弹窗，事件日志实锤），`watchdog.bat`/`watchdog_hidden.vbs` 两脚本已随之删除。⚠ daemon 启动参数由 paguard **硬编码**（`-addr`/`-workspace`/同目录 exe 名）→ 加 flag 必须重编译 paguard.exe。2026-09-23 起 paguard **不再注入 `CODEX_BIN` 默认值**（旧行为注入写死的开发机路径，会覆盖 daemon 自己的定位链，把「能找到 codex」变成「找不到」） |
| 分发 | `dist/windows/build.ps1` | 打包 daemon+paguard+version.txt+install/uninstall/使用说明 → `PocketAgent桌面端.zip`。2026-09-23 修：原脚本在 go build **之后**执行 `Remove-Item $StageDir -Recurse`，把刚构建的两个 exe 删掉，产出的 zip 缺 pocketagentd.exe/paguard.exe（提交 `42813da` 引入的回归）；现顺序为 清理 → 创建 → 构建 → 痕迹扫描 → 压缩，扫描命中开发机痕迹即中止不出包 |

---

## 6. 外部依赖与能力边界

| 依赖 | 用途 | 边界 |
|---|---|---|
| **codex CLI**（CODEX_BIN） | 主力原生 agent | 本机 token 403 时好时坏（地区限制）；模型窗口多数真未知（schema 无字段，UI 显「—」是如实）；**Default 模式反问卡**需 config 注入 `default_mode_request_user_input`，而 codex 自报该特性 `underDevelopment` → 开启弹警告，抑制红线封死 → 只按运行时宣告决定是否注入，未宣告则退回（**绝不写用户 config.toml**） |
| **pi CLI** | 原生 agent | 1 子进程=1 会话；同 provider 多会话 turn 锁排队，**不可宣称并行**；`-pi-collab` 默认开（自建 `~/.pocketagentd/pi-agent` 目录，**不碰用户 pi 配置**） |
| **ZCode 桌面端** | 原生 agent | 仅 Individual Coding Plan；Team/Start/Off-peak、OAuth 刷新未实现；**子代理协议层不透出内部对话**，只能展示工具卡参数与产出（见 §8）；生命周期锁已改有界排队（`zcodeLockWait` 15s） |
| **dsh（DeepSeek Harness）** | 原生 agent | `dsh --profile acp` 子进程，ACP/JSON-RPC；**隔离 HOME** `~/.pocketagentd/dsh-home`，凭据只透传 daemon env 的 `DEEPSEEK_API_KEY` 或 `<数据目录>/dsh-credentials.env`；**绝不碰用户 `~/.dsh`**；首跑握手可达分钟级（已用启动预热 + 有界排队吸收）；MCP 走**部署级** `cordis.patch.yml`（会话级 `mcpServers` 非空会被拒） |
| GLM / DeepSeek API | BYOK 直连（env key） | 唯一 daemon 自跑 agent 循环的路径 |
| Tailscale | 外网通道 | **PC 侧已装方案，非内嵌**（P2 内嵌 libipn 未做） |
| mDNS (`_pocketagent._tcp`) | 局域网自动发现 | 多播失败只丢便利性不崩 |
| 云服务器 | **无** | 零云是产品原则；`server/` 目录不存在 |

**红线**：绝不改写用户桌面 `~/.codex/config.toml`；粘贴长文本不中断是 stdio 通道既有事实（`turnInput()` 零裁剪，单测钉 24 万字）；没能力的原生请求回 error 不伪造 decision。

**子进程 stderr 统一收口**（`internal/stderrdrain`）：四家 harness 的子进程 stderr 落进 `daemon.log`（前缀 `[codex-stderr]`/`[pi-stderr]`/`[zcode-stderr]`/`[dsh-stderr]`）。**脱敏**：键值型（`api_key`/`token`/`password`/`authorization`/`bearer`/`x-api-key`/`cookie` 等键后的值）、JWT（`eyJ…`）、`sk-`/`pk-` 前缀长密钥串 → 打码；**刻意不碰**哈希/路径/UUID/错误原文（过度脱敏会把死因一起抹掉）。**限流**：单行落日志 ≤2000 字符、单子进程累计 ≤256KB（超出补一行限流说明后停止），避免一条巨行或刷屏淹没真正的死因。四家读取函数**签名不变**，函数体委托 `Drain`，调用方与既有单测零改动。

---

## 7. 设计原则：未来未知事件接口（能透传就透传）

**核心立场**：原生 agent（codex/pi/zcode/dsh）会升级、会改模型、会发新事件。daemon/App 不能假设「我认识全部」，更不能把「我不认识」当成「出错」一脚踩死。

### 三层含义
1. **识别不了 ≠ 出错** —— 原生发来没见过的通知/字段，默认定性为「数据」不是「异常」；能原样透传就透传，不丢弃、不报警、不伪造。
2. **接不了 ≠ 接死** —— 暂时没能力处理的反向请求，回 error（如实告知「不支持」），**绝不回伪造的 decision**。
3. **静态名单是债** —— 凡在代码里写死「模型白名单 / agent 名单 / 事件名单 / 版本号二分判断」，都是给未来埋雷。

### 已合规行为（保持）
| 位置 | 行为 |
|---|---|
| `server/model.go` ListModels | **无模型白名单**，原生返回什么透传什么 |
| `agent/codex_events.go` 默认分支 | 未识别事件**静默丢弃**（不伪造、不上报） |
| `agent/codex_requests.go` | 不支持的反向请求回 `reqUnsupported`（JSON-RPC error），**不伪造 decision** |
| `agent/zcode*.go` | 模型名/窗口/reasoning **全部运行时返回**，无静态 GLM 名单 |
| `agent/native_notifications.go` | 不认识的兄弟节点原样保留 |
| `turn_queue` | 中途插入降级为排队，不报错 |

### 已整改 / 待整改（技术债）
| 位置 | 问题 | 状态 |
|---|---|---|
| `server/thread.go` CreateThread | 未知 agent → 硬错误 | **已整改**：agent_id 在 `s.cfg.Providers` 注册表即接入，报错动态列举可用 agent |
| `server/model.go` order | agent 顺序写死 | **已整改**：从 `s.cfg.Providers` 动态派生（桌面桥接 PrefsAware 在前、echo 最后） |
| `server/model.go` defaultModelIDFor | 假设 `agent+"/default"` 存在 | **已整改**：default 哨兵 → 列表首项 → 明确报错，不伪造 ID |
| `server/agents_sessions.go` 导入名单 | 写死 `{pi,zcode}` | **已整改**：导入改能力接口 `AgentSessionImporter` 动态发现 |
| `agent/codex_requests.go` isLegacyApproval | 版本号二分判断 | **待协议演进时整改**（现存唯一挂牌硬编码） |

### 决策（2026-09-20 拍板）
- **主线 = A「能透传就透传」**：未知通知/字段原样包装（原文 + 原方法名）透传到 App 诊断视图；App 显示「原生发来了一个新事件」，不显示「错误/未知」。
- **辅线 = B「把决定权还给用户」**：未知反向请求透传请求体让 App 弹通用确认框（同意/拒绝/忽略）。**全局设置开关，默认关**。
- **不选 C「协议版本探测」**：维护成本高，仍是「假设我知道所有版本」的思路。

---

## 8. 子智能体：委派 / 协同 / 角色 / 可见性

> 手机端要能「同时派发多个子智能体，并行推进任务的不同部分」。先分清两条互不相同的链路，否则会把「原生不给数据」误判成「App 坏了」。

### 8.1 两条链路（务必区分）

| | **A. daemon 委派链（collab）** | **B. 原生子代理链** |
|---|---|---|
| 谁发起 | coordinator agent 调 MCP 工具 `agent_delegate`（daemon 内嵌 `pocketagent-collab`） | 原生 agent 自己的机制：codex `subAgentActivity`/`collabAgentToolCall`；ZCode `Agent`/`Task` 工具 |
| 是不是真线程 | **是**：daemon `CreateThread` 建真子线程，父链落 `ParentThreadID` | codex **是**（真线程，带 `parentThreadId`）；**ZCode 不是**（进程内机制，无父链可挂） |
| 手机端呈现 | 内联子代理卡（单卡/组卡）+ 可点进 `chat/{childThreadId}` 看完整子对话 | codex：同上（存根 + `thread/read` 回填）；ZCode：**仅工具卡**，默认展开显示任务与产出 |
| 对话正文可得性 | 完全可得 | codex：可得（`thread/read {includeTurns:true}` 是唯一通道）；**ZCode：协议层不透出**，硬边界 |

> **第三种形态——协同扇出（对等并行）**：与上述两条都不同。语义是「用户一条指令扇出给 N 个 agent 同时跑」（**并行对等**），而非「一个 agent 派子智能体」（父子委派）。见 §8.5。

### 8.2 组织：谁派给谁（委派链）

- `agent_delegate { agent_id, task, workspace?, model?, effort?, role_id? }` —— **`agent_id` 由调用方显式给出**，daemon 不猜、不路由；不在注册表的 id 走 `CreateThread` 的人话报错（动态列举当前可用 agent，无静态名单）。
- **子不生子**：`agent_delegate` 只挂到「用户主动接入的 coordinator」，委派产生的子线程**不挂载**该 MCP 服务器 —— 这是**结构性闸**（子线程工具列表里根本看不见 `agent_delegate`）。引擎侧另有一道 belt-and-braces：发起方线程若已在 `childThreads` 登记簿内 → 拒绝。

### 8.3 怎么把 MCP 挂到 agent（四家现状与边界）

一个 agent 只有挂上 `pocketagent-collab`，它的模型才看得见 `agent_delegate`。**没挂上 = 手机端根本派不出子智能体**（实测症状：会话里请求派发后消息数为 0，请求压根没发出去）。

| agent | 挂载方式 | 状态 |
|---|---|---|
| **pi** | `-pi-collab`（默认**开**）：初始化 daemon 自有 pi agent 目录（`~/.pocketagentd/pi-agent`，**不碰用户 pi 配置**）→ 装 pi-mcp-adapter → 写 `mcp.json` 注册本 daemon 的 `-collab-mcp` 子模式 → 注入 `PI_CODING_AGENT_DIR` 与 `POCKETAGENT_TOKEN` | **已可用** |
| **dsh** | 隔离 HOME 的部署级 profile（`~/.pocketagentd/dsh-home/profiles/acp/cordis.patch.yml`，归我们所有） | **已可用**（`collabpatch.go` 读-合并-写；非空且无我们 id 时不覆盖，回报降级） |
| **codex** | 协议层无注册入口，唯一方式是改 `~/.codex/config.toml` | **红线**：绝不代改 → 只能给用户可粘贴的配置文本 |
| **zcode** | 需调研其 MCP 配置机制（用户安装目录只读） | **待用户拍板落点**（工作区级 vs 用户级，见 §0.4 阻碍 6） |

> 开关可关：`-pi-collab=false`。写入失败一律只记日志，不阻塞 daemon 主功能。

**手机端怎么知道「谁能派发」**：daemon 在 `GetHostInfo` 的 capabilities 里报 **`collab:<agentId>`**（能力接口 `agent.CollabAware` 动态判定，**不枚举 agent 名字**）——判据是**可核验的接线事实**。手机端据此**点亮「多 agent 协同」入口**，而不是摆一个恒假的「开发中」开关（旧实现正是「开关可拨但功能未实现 = 假可用」，已整改）。

### 8.4 调度：有界并行，不伪造并行

- **跨 agent 可并行**；**同一 agent 的委派由 per-agent 信号量串行**（原生 turn 锁是事实约束 —— 排到队才发「排队中」progress，**绝不宣称并行**）。
- **有界并发**：全局与单设备默认上限均 **3**，`-collab-max-parallel N` 可调（`0`/负数回落默认）。超限**如实报错、不排队** —— 父轮次在工具调用上同步阻塞，排队会让父子互等成死锁。
- 每次委派有引擎侧时长兜底 `maxDelegateTurnDuration`(30m)，到点取消并如实回传。
- **子任务终稿摘要有界**（≤8192 字符，超出截断标注）—— 父持决策权，不让子代理灌爆父的上下文。

### 8.5 协同扇出（对等并行，2026-09-22 新增）

- 语义**明确区别于委派**：委派 = 父子（父 agent 决定派活）；**协同 = 并行对等**（用户一条指令扇出给 N 个 agent 同时跑）。
- 实现（`server/collab_session.go` + `DispatchCollab` RPC + `CollabMember`/`DispatchCollabRequest`/`DispatchCollabItem`/`DispatchCollabResponse`）：
  ① RPC 只**受理**（快速返回 thread_id），不等结果（4 个 agent 跑完太久）；
  ② 异步 goroutine 用**脱离取消链的新 context**（RPC 一返回 ctx 就被取消，沿用会把刚起飞的轮次整个杀掉，表现为「永远显示收到但什么都没跑」）；
  ③ **不设 ParentThreadID**（协同 session id 不是真实线程，挂上去就是一张永远点不亮的孤儿卡）；
  ④ 一家失败不影响别家（部分成功）。
- **刻意不做编排**（谁先谁后 / 谁汇总）—— 那是替用户猜分工，不可预测也不可控。
- 手机端 `CollabState.dispatch` 接真实 RPC，按**索引**对应 items；入口挂在 `multiAgent = collabIds.isNotEmpty()` 判定内（**无能力不显示假入口**）。

### 8.6 协同角色（可复用的「岗位」配置，2026-09-22 新增）

- **缺口**：委派原本只能**按次**传 `task/model/effort`，没有持久化「岗位」—— 父模型要让子代理反复扮演同一角色（如「审查者」）就得每次复述人设，改口径要改 N 处。
- **实现**：`proto/v1/thread.proto` 加 `CollabRole`（id/name/agent_id/description/instructions/model/effort/workspace）+ 3 RPC（List/Set/Delete CollabRole，均在 ThreadService）+ `DelegateSubTaskRequest.role_id`；`store.go` 并入既有 `threads.json`（`CollabRoles map[string]CollabRole`，复用原子写）；`server/collab_roles.go` 解析（**优先级：本次显式值 > 角色值**；未知 role_id → 报错并**列出可用 id**，绝不静默回落）；`agent_delegate` 加 `role_id` 入参（薄代理透传）。
- **刻意不做自动编排** —— 谁在什么时候派给谁，交给父模型 / 手机决定。

### 8.7 内容可见性（三路径）

- **codex 路径**：会话里出现**内联子代理卡**（标题/状态点/最后消息预览）；点卡进 `chat/{childThreadId}` 看完整子对话；子代理跑完自动回填对话内容。实现：`thread/started` 带 `parentThreadId` 时登记**轻量存根**，三处接线回填（打开子对话页懒拉 / 登记后异步首拉 / 转 `idle` 时全量重拉），合并规则**宁可少显示也不覆盖用户可见历史**。
- **ZCode 路径**：子代理工具卡（`Agent`/`Task`/`subagent`）**默认展开**，直接看「参数」里的任务与「输出」里的产出（结果放宽至约 1.6 万字符，普通工具仍 4000）。按**入参结构**识别子代理工具（不写死工具名，识别失败当普通工具）。
- **pi 路径**：子代理以**子线程存根**出现在会话列表（id = `pi-sub-<runId>`），点进去看到**完整对话**（transcript）+ **产出**（output）。数据来源：pi 留四件套 `<父会话文件同目录>/subagent-artifacts/<runId>_<agent>_{input.md, meta.json, output.md, transcript.jsonl}`；按**父会话文件同目录**定位（结构性事实），全部**有界**（消息 200 / 单条 4000 rune / 产出 20000 rune / 运行 50 条），坏条目跳过。**归属只在候选唯一时挂**（`subagent-artifacts` 是工作区级目录，`meta.json` 无父指针 → 同工作区 + run 时间落在会话活动期内 + 候选恰 1 个才挂；0 或多个一律不挂）。

### 8.8 子智能体界面设计规范（四条硬规范，所有 agent 一视同仁）

1. **接入口固定在会话页顶部**（不随消息滚动）：一行入口条 `<图标> N 个子智能体 · <状态摘要>` + 展开箭头；默认折叠；消息流里不再插子智能体卡片。
2. **子代理消息单独分组，不混入主对话**：主对话流只呈现主对话自身消息；子代理消息只在①顶部入口条展开后的迷你行预览②点进去的子代理会话页出现。
3. **子代理会话页顶部提供「返回主对话」入口**：来源行左侧显式返回控件；父会话不可达时置灰并给人话原因，不静默失效。
4. **统一设计**：图标一律走 `ui/AgentBrand.kt`（唯一映射）；组件只用 `SubAgentCard`/`SubAgentGroupCard`/`SubAgentSourceLine`/`SubAgentPeekSheet`（**不按 agent 分叉**）；数据模型归一成**子线程**（`ParentThreadID` → 手机只认一套分组逻辑 `SubAgentsLogic.childrenOf`）；状态统一 5 态（运行/等审批/完成/排队/失败）+ 统一配色；命名统一 `<agent 显示名> · <时间>`，不暴露内部 id。**唯一允许的差异是「能否看到内部对话」**（codex/pi 可看到完整子对话；ZCode 协议不透出，只能展示工具卡参数与产出）—— UI 骨架一致，差异只在内容区且**必须如实标注**（不得写成「未知/错误」）。**某 agent 无该能力时不显示假入口**。

---

## 9. 存储与内存管理

> 判定唯一标准 = 该文件是「用户数据」还是「可再生 / 可淘汰物」。

### 9.1 数据落点全表（2026-09-21 实测）

| 位置 | 内容 | 实测体量 | 性质 | 处置 |
|---|---|---|---|---|
| `~/.pocketagentd/threads.json` | **会话真值**（消息/标题/工作区/父子链/协同角色） | 1.5 MB | **用户数据** | 只随会话增删变化 |
| `~/.pocketagentd/threads.json.bak` | 上一次写入的备份 | 1.3 MB | 可再生（上一版快照） | 可删；**是删除会话后的最后退路**，谨慎 |
| `~/.pocketagentd/events.jsonl` | 事件日志（单调 seq，断线重放用） | 43 KB | 会话过程数据 | 轮转后旧段可清 |
| `~/.pocketagentd/events-*.jsonl` | 轮转出的历史事件段 | **10.5 MB** | 会话过程数据 | **可清理**（重放只需当前段 + 轮转窗口） |
| `~/.pocketagentd/devices.json` | 已配对设备 token | 421 B | **用户数据（安全凭据）** | 别删（删了要重新配对全部设备） |
| `~/.pocketagentd/pair-code.txt` | 配对码 | 9 B | **用户数据（安全凭据）** | 别删 |
| `~/.pocketagentd/certs/` | 自签证书 + 私钥 | 903 B | **用户数据（安全凭据）** | 别删（删了所有设备指纹失配） |
| `~/.pocketagentd/model-pref.json` | 桌面模型偏好快照 | 109 B | 用户偏好 | 可删（退默认） |
| `~/.pocketagentd/dsh-home/` | dsh 隔离 HOME（含 `profiles/node_modules`） | **421 MB** | 依赖 + 会话 | `sessions/` 属会话数据；`profiles/node_modules` 可删**但下次运行会重装** |
| `~/.pocketagentd/pi-agent/` | pi 协作 agent 目录（含 pi-mcp-adapter） | **83 MB** | 依赖 + 配置 | `mcp.json` 是接线事实（删了要重启 daemon 重建）；`npm/` 可删但会重装 |
| `~/.pocketagentd/pi-subvis-probe/` | **探针残留**（子代理调研探针的隔离 HOME） | **226 MB** | **纯垃圾** | **可删** |
| `<项目>/.pocketagent-uploads/` | 手机上传的文件（截图等） | 2.5 MB | 用户上传物 | 按会话清理（见 9.3） |
| `~/.pi/agent/sessions/**/subagent-artifacts/` | pi 子代理四件套 | 单个 124–308 KB | 会话过程数据 | **由用户决定**（见 9.3） |
| `~/.codex/sessions/**/rollout-*.jsonl` | codex 原生会话文件 | 取决于使用 | **原生数据（不属于我们）** | 删会话时联动删除（软失败不阻塞） |
| `<项目>/build/`、`dist/` | 构建产物 | 515 MB / 6.6 MB | 可再生 | 可删 |

### 9.2 App 侧（手机）

> **路径唯一真源 = `ui/component/PaPaths.kt`** —— 目录名与临时文件前缀集中于此，各模块统一引用；改名只改这里，且**必须配迁移**（否则旧目录变孤儿）。

| 位置（`PaPaths` 常量） | 内容 | 性质 | 处置 |
|---|---|---|---|
| `filesDir/pa_snap`（`DIR_SNAPSHOT`） | 会话离线快照（最近 N 轮，三层限额 + LRU） | **缓存（RISKY）** | 清了只能从电脑重拉、离线看不到；须二次确认 |
| `filesDir/turn-checkpoints`（`DIR_TURN_CHECKPOINTS`） | 轮次检查点 | 用户数据 | 随会话生命周期 |
| `filesDir/sensevoice-int8`（`DIR_SENSEVOICE`） | 端侧 SenseVoice 模型（首次解压校验） | 可再生成 | 删了下次用到会重解压 |
| `cacheDir/pa_offline/pa_url`（`DIR_OFFLINE`） | 网页快照（断网回看；30 天 TTL + 80MB LRU） | **缓存** | 可随时清 |
| `cacheDir/pa_preview`（`DIR_PREVIEW`） | 预览内容缓存（二次预览免传输，200MB 上限） | **缓存** | 可随时清 |
| `cacheDir/pa_pdf_*` / `pa_ziplist*`（`TMP_*_PREFIX`） | PDF 渲染 / zip 列表临时文件 | **临时** | 启动清扫（`PaCache.sweepTemp`） |
| 外部 `files/DOWNLOADS`（`downloadsDir`） | 用户显式下载内容 | **用户数据** | **清理不得删除** |
| `dataDir/app_webview`（`DIR_APP_WEBVIEW`） | 内置浏览器 WebView 数据（系统托管） | 缓存 | 用户主动清理（`PaCache.clearBrowserData`） |
| `EncryptedSharedPreferences` | 每设备 token（Keystore 加密） | **用户数据（安全凭据）** | 别删（删了要重新配对） |
| SP：`pa_onboarding` / `pa_thread_pins` / `pa_thread_usage` / `pa_usage_card_cache` | 引导态、置顶、用量、用量卡缓存 | 用户数据 | 随会话删除联动 |
| `SubAgentsState` / `ThreadUsageStore`（内存） | 子代理预览、用量 | 内存态 | 进程结束即释放；切页不串 |

> `cacheDir` 是共享目录（WebView、系统组件都在用）：`OfflineStore` 只动 `pa_offline/…`，`PaCache` 只动自己前缀 —— **不碰**别人的内容。
> App 缓存管理界面（2026-09-22）：快照标 RISKY、其它 SAFE，**凭据永不入清理目标**，带风险分级 + 二次确认。

### 9.3 「彻底删除」的现状与差距

**结论：目前不支持「彻底删除全部对话」** —— 删除会话只清理「会话本体 + 原生会话文件」，**不触碰过程产物**。

**删除交互**：列表左滑 / 菜单 → 删除 → **5 秒内可撤销**（纯本地还原，断网也必然成功）→ 过了撤销窗口才真正提交；失败会 toast 说明并在刷新时把乐观移除的行**复原**（不让会话「凭空消失」）。

**会**清理：`threads.json` 条目、codex rollout（软失败不阻塞）、pi session 文件、zcode 原生会话；并写**墓碑**（`thread_delete_tombstone`，记住「用户删过」，同步链遇墓碑跳过，防删除后复活）。

**不会**清理（残留）：`events.jsonl`/`events-*.jsonl`（seq 重放需要，**不能无脑删**）、`.pocketagent-uploads/`、pi 子代理四件套。

**分级设计建议**：**L1 删除会话**（现状，默认）；**L2 连带清过程产物**（显式开关，**默认关**）；**L3 清空全部数据**（删 threads/events/上传/隔离 HOME sessions，**保留** `devices.json`/`pair-code.txt`/`certs/`，**必须二次确认 + 写清保留项**）。**绝不自动删**：安全凭据三件 + `threads.json.bak`。

### 9.4 内存态（进程内）

- **daemon**：`activeTurns`（在飞轮次租约）、`turnQueue`、`collabEngine.childThreads/inflight`、各 provider 的 `lifeMu` + 模型缓存 —— **纯进程内**，重启即清；登记表按「量级 = 委派次数」设计，不清理也无泄漏。
- **子进程**：codex/pi/dsh 长驻（一号进程多会话；pi 例外：1 子进程 = 1 会话），由 reaper / Close 回收。
- **App**：会话列表、流式缓冲、子代理预览缓存随进程存活；前台服务 `TurnKeepAliveService` **超时停服务而非崩溃**，且电脑执行与结果保存**不依赖它**。

---

## 10. 文件 / 下载支持

判定唯一源 `ui/component/FileKind.kt` 的 `preview` 字段。

- **内置预览**：md / 文本 / json / html / csv / 图片 / **xlsx+xls（自研 OLE2/BIFF8）** / ods / docx / rtf / odt / pptx / odp / pdf（PdfRenderer）
- **只交系统**：doc / ppt / apk / 音视频
- **只保存**：压缩包 / exe
- **边界**：Office 只还原内容不还原版式；单读 64MB 上限；表格 800×40×3、PDF 12 页、PPTX 40 页。

---

## 11. UI 与设计约定

- 主题令牌 `Pa`；组件库 `ui/component/`；`ui/AgentBrand.kt` 是 agent 品牌图标**唯一映射**（未知 agent 用通用占位图标，**禁止**各屏各写一份）。
- 图标只用 lucide（`ic_lucide_*`）+ 品牌 PNG；**禁止 emoji 当图标**。
- 取向：统一、简洁、小而美；**少一层弹层**；能左滑就别开菜单。
- 「Agent 与模型」是**一个长页**（切换 Agent → 权限与安全 → 模型与思考 → 行为与降噪 → 用量与限流 → 能力/集成 → Codex 账户）。
- 设置项多选项一律 `SplitButton`；**不写解释性小字**（预览除外）。
- 子智能体界面另见 §8.8。
- **禁止「未知 / 错误」措辞** → 拿不到数据要给人话原因（AUDIT §3.3）；**无能力时不显示假入口**；**不新增写死名单**。
- 注释：**中文**，写「为什么」而非「做了什么」；**修复 / 新增不附用例 = 没做完**。

---

## 12. 关键坑（开发必读）

0. **动工前先 `go build ./...` 建基线**：工作树可能残留**上一轮未跟踪的半成品**（实例：`server/codex_subagent_history.go` 缺 import → 全仓编不过，还被并行子代理误判归属）。多子代理并行时：任务书必须显式写「不要碰哪些文件/包」，并额外做一次**合并后**整体编译校验。
1. **「源码改对 ≠ 跑着的是新的」**（头号假故障）：报「没生效」前先 `apkanalyzer dex packages` 查新类 + **dex/二进制里 grep 本轮新增的中文字面量**（Go 会把**注释**丢弃、不进二进制 → 自证要用**符号名或字符串常量**）。
2. **构建缓存三件套**：① 大批 `Unresolved reference` 且全指向**没改动**的文件 ⇒ Kotlin 增量缓存失配，物理删 `app/build` + `.kotlin` 重跑；② `:app:clean` 删不掉 = Gradle daemon 占锁 → `gradlew --stop`；③ `shutil.rmtree` 物理删 build 等价。**反例**：错误全在**刚改的文件**、首条是操作符 `Unresolved reference`（如 Flow `.buffer` 没 import）⇒ 补 import 即过，不是缓存问题。
3. **Kotlin 块注释可嵌套**（与 Java/C 不同）：注释里写 `video/*` 会吞到文件尾；修正时字面「星号+斜杠」会提前闭合，报错一样。自查 `tools/device/_pa_scan_nested_comment.py`（词法状态机）。
4. **StateFlow 按 equals 发射**：原地改字段永不刷 UI，要刷的用 `mutableStateOf` 委托。
5. **Composable 里 `return` 截断渲染**：底部弹层写在 `return` 之前。
6. **真机驱动三铁律**：每脚本开头 `ensure_reverse()`；关键点击 dump 校验 + 自愈重试；BACK 会打回桌面 → 导航一律 force-stop + 冷启动。脚本要前台跑完或收尾 kill（否则下一轮看到自己上一轮的操作，误判成并发驱动）。
7. **小米安装**：必须先 uninstall 再 install（`-r` 静默失败）。安装前先 copy 到 `C:\Users\yanzi\_pa_apk\app-debug.apk` 再装（忘 copy 就装旧包）。
8. **PowerShell 工具不回传输出** → 验证一律走 Python subprocess（`encoding='gbk'`）；Bash 禁调 `powershell.exe`；`schtasks`/`cscript`/`cmd` 被黑名单拦（**不要重试**）。
9. **杂项**：`Write` 会把源码 `\u0000` 写成真 NUL 字节（`read('rb').replace()` 修回）；Bash heredoc 写 Go 会把 `\n` 变成字面 `/n`；`f.delete()` 后 `f.length()` 恒 0；`taskkill //PID` 被路径转换搞坏 → 用 Python；Windows 删大目录加 `\\?\` 长路径前缀，批量删除先出 manifest。
10. **批量操作用户主目录前**，必须单独审「不以下划线开头」的命中项。

---

## 13. 审核（强制执行，→ `AUDIT.md`）

**任何更改合入前，必须严格按 `AUDIT.md` 执行，无例外。** 要点速记：

- **变更分级**（`git diff` 路径自动定级，取最高、不许降级）：**甲**=契约/架构（proto、新 agent）→ L0+L1+L2 全量 + 契约双向核验；**乙**=功能（daemon agent/server、android ui）→ L1 全量 + L2 受影响；**丙**=修复 → L1 + 单条复验 + **附回归用例**；**丁**=仅文档 → 只查锚点漂移。
- **三层**：L0 闸门（产物自证 apkanalyzer）→ L1 自动化（`--rerun` + XML 时间戳防假绿）→ L2 真机（BLOCKED 必带归因 + 解封条件）。
- **红线一票否决**：伪造 decision / 假绿 / 契约不同步 / 产物不对；**提示项不阻塞**：锚点漂移、证据缺项限期补录。
- **设计原则合规**：新增硬编码名单即报警（存量挂牌仅剩 1 处 `isLegacyApproval`；豁免登记项见 AUDIT 附录 A，只减不增）；UI 禁显「未知/错误」。
- **缺陷即测试**：修复不附回归用例 = 没修完；**审核资产只增不减**。
- 入口脚本 `tools/audit/pa_audit.py`（六条规则独立开关，`--json` 机器可读）；完整标准、checklist、linter 规则、准出 DoD、度量回流 → **`AUDIT.md`**。

---

## 14. 运维：日志与清理

- **日志 5 类**：A 运行(`daemon.log`/`watchdog.log`，轮转只留错误行)｜B 验收证据(`docs/verify/evidence/**`，磁盘**神圣不可清理**、**截图不入库**)｜C 构建一次性｜D 归档冷数据(`_archives/`，90 天)｜E 错误汇总(长期)
- **铁律**：成功日志不长期留盘；`*.log` 禁止入库；**归档只搬不删**；运行中的日志禁止直接删。
- **定期维护**：`python tools/ops/maintenance.py --apply`（建议每周一次；串联「日志轮转 → 垃圾清扫 → 冗余清理」三步，默认预演不改动）。计划任务注册示例见脚本头 docstring（**沙箱禁 `schtasks`**，注册需在能跑 schtasks 的外层 shell）。
- **工具**（`tools/ops/`）：`maintenance.py`（定期维护总入口） / `archive_logs.py` / `rotate_runtime_logs.py`(SHA256 校验后才截断) / `clean_junk.py` / `cleanup_project.py`(精确保护清单；**2026-09-22 扩展 P2「可再生/垃圾」层**：apk-analysis 反编译产物与工具、build 孤儿/快照、daemon 历史二进制与探针 exe、构建缓存、`_archives` 归档裁剪，白名单式 + 保护清单 + manifest) / `cleanup_cdrive.py`。⚠ `rotate_runtime_logs.py` 归档路径仍指向 `docs/ops/logs-archive/`（该目录由 cleanup 工具按需重建，属历史遗留落点，待统一）。
- **勿动**：运行中的 `pocketagentd.exe` + `watchdog.log`；两处 `.pocketagent-uploads/`（daemon 活数据）。
- **删除台账**：`_archives/_docs-discarded_20260920-manifest.json`(109 文档)、`_residual-cleanup_20260920-manifest.json`(97MB 残留)、`_cleanup_20260922-manifest.json`(过程文档 10 + 日志轮转 + 真删)、**`docs/ops/logs-archive/2026-09-22/project-cleanup-manifest.json`(595 项 / 1.81GB，工作区 2510→659MB)**。
- **可删再生**：`G:\pa\android`+`C:\pa\android` 镜像、`pocketagent/build/`、`android/.gradle`。

---

## 15. 已知边界与技术债

- **pi/dsh 定位已改为按用户环境派生**（2026-09-23）：`internal/{piapp,dshapp}` 不再硬编码开发机的 node / pi / dsh 绝对路径 —— 那些路径作为字符串常量经 `go build -trimpath` 后仍留在二进制字符串表里，会随 pocketagentd.exe 进发布包（规格 §2 禁止；旧发布包的 `paguard.exe` 解包实测确实命中 `Claude Code`）。改由 `internal/hostenv` 派生，顺序为 **env 覆盖 → 用户配置的事实 → 默认位置 → PATH**：env（`PI_NODE`/`PI_BIN`/`PI_CLI`、`DSH_NODE`/`DSH_BIN`）→ 随包分发目录 → `.npmrc` 的 `prefix=` / `NPM_CONFIG_PREFIX` → `%LOCALAPPDATA%`/`%ProgramFiles%` 下的 node 安装位置 → npm 默认全局根（`%APPDATA%\npm` 等）→ PATH；探测不到返回空串，绝不回落写死路径。**实测**：本机 `LocateCLI` / `LocateDsh` 均命中 `G:\Claude Code\node_modules\...`（err=nil），**无需重装 pi/dsh** —— 路径值与旧常量相同，但来历不同（跟着 `.npmrc` 走 vs 写死）。编译期回归钉子见 `{piapp,dshapp}/devpath_assert_test.go`（AST 只扫字符串字面量，**注释不算** —— 注释被编译器丢弃、不进二进制）。**仅在**既无 env 覆盖、又装在上述任何位置之外时，才须显式设 `PI_NODE`/`PI_CLI`（pi）或 `DSH_NODE`/`DSH_BIN`（dsh）。
- **build.ps1 打包链已修并首次实跑验证**（2026-09-23），两处缺陷：① 原脚本在 go build **之后**执行 `Remove-Item $StageDir -Recurse`，把刚构建的两个 exe 删掉，产出的 zip 缺 pocketagentd.exe 与 paguard.exe（提交 `42813da` 引入的回归；仓库 09-17 那份 zip 解包含两个 exe 可证）。② 脚本为 UTF-8 无 BOM，PowerShell 5.1（Windows 自带 5.1.26100）在 GBK 代码页下按 ANSI 读，中文注释乱码后吞掉字符串引号 → **解析即失败**；改动前的版本用同一解析探针同样报错，说明该脚本一直无法在当前环境被执行，09-17 那份 zip 的来历因此存疑。修法：清理上移到构建之前 + 脚本加 UTF-8 BOM + 压缩前插入开发机痕迹字节扫描（命中即中止不出包）。**实跑**：EXIT 0（7.6s），输出「痕迹扫描通过:6 个文件」，zip 含 pocketagentd.exe(19467264B) 与 paguard.exe(2567680B)；解包逐字节复核两个 exe 均无 `yanzi`/`Claude Code`/`G:\项目1`/`100.92.193.109` 命中（旧产物 paguard.exe 命中 `Claude Code`，对照即证）。

- daemon `go vet` 有 protobuf 值拷贝警告（存量噪音，非新问题）。
- ~~**daemon `internal/server` 有 2 例 flaky 测试**~~ → **已修**（2026-09-23，提交 `46d04c9`）。根因：协同扇出的轮次跑在**脱离取消链**的 goroutine 里（设计如此，避免 RPC 返回即被杀），而测试只断言 RPC 响应、不等轮次落定 —— 于是 Windows 上 `t.TempDir()` 的 `RemoveAll` 撞上仍在写文件的 goroutine，报 `The directory is not empty` 而间歇 FAIL（修复前实测 `-count=8` 挂 7 次）。修法取本条原列的方案①：`newCollabSessionServer` 注册 `t.Cleanup` 等待 `activeTurns` 清空（再留 20ms 沉降窗口 —— 租约释放与落库尾巴之间仍有间隔），`t.Cleanup` 的 LIFO 保证等待先于 `t.TempDir()` 清理。放在构造器而非各用例各自加：将来新增用例自动获得同一保证。修复后 `-count=8` 零失败，`go test -count=1 ./internal/...` **首次全绿**。
- 同 provider 多会话不并行（原生 turn 锁排队）。
- 上下文窗口多数模型真未知（`ListModels` 仅 `codex/default`/`echo-1` 带窗口；codex 不发该字段）。
- **Vosk 已摘除**（2026-09-20）：依赖 + `Vosk*` 两文件 + 引擎选择分支全部移除；共享错误码迁至 `VoiceErrors.kt`；行为零变化（摘除前引擎恒不可选）。
- `isLegacyApproval` 版本二分（唯一挂牌硬编码，待协议演进时整改）。
- **P2 内嵌 Tailscale、P3 GeckoView / 自更新 / 实况通知均未做**。
- 审核遗留：86 张 2026-09-16 证据截图已出库（2026-09-20，仅出 git 索引、磁盘保留）；`dist/windows` zip 版本未核（打包于 2026-09-17，内含 daemon 二进制无版本标识）。

---

## 16. 快速开始（命令速查）

### 16.0 构建前置（fresh clone → 可出完整产物）

- **daemon**：Go（版本对齐 `daemon/go.mod`，当前 `go 1.27.1`）。`cd daemon && go build ./...` 即可，无代码外资产。
- **Android APK**：需 Android SDK 与 JDK（`android/build-ascii.ps1` 的 `-SdkDir` / `-JbrDir` 参数可指定，默认值见脚本 param 块）。构建入口 `android/build-ascii.ps1`：先把源码镜像到 ASCII 暂存路径再跑 gradlew（中文路径 Gradle 处理不了），并**自动下载校验**两件不入库的语音资产 —— sherpa-onnx AAR（`android/app/libs/`，见该目录 README）与 SenseVoice 模型 `model.int8.onnx` + `tokens.txt`（`android/app/src/main/assets/sensevoice/`，见该目录 README）；SHA-256 不符即中止构建，无需手工放置。
- **桌面分发包**：`dist/windows/build.ps1` 构建 daemon + paguard 并打成 zip（含安装/卸载脚本）。

```bash
# ── daemon（电脑上）──
cd daemon
go run ./cmd/pocketagentd -addr 0.0.0.0:7246 -workspace "你的项目目录"
# 首启自动生成 8 位配对码：打印在启动日志，并持久化到 ~/.pocketagentd/pair-code.txt（重启复用不换码）
set CODEX_BIN=G:\Claude Code\codex.cmd                                    # 桥接 codex（推荐）

# ── APK（必须走镜像包装器，中文路径 Gradle 处理不了）──
python tools/device/_pa_gradle.py apk|cleanapk|test|compile
adb uninstall dev.pocketagent.app && adb install build/app-debug.apk     # 小米必须先 uninstall

# ── 契约(改 proto 后必做两件:双副本同步 + 生成 Go)──
python tools/proto/gen_proto.py            # 同步 android 副本 + 生成 Go；protoc 探测顺序/版本打印见脚本头
python tools/proto/gen_proto.py --check    # 只校验双副本 MD5 一致(AUDIT 红线 3)
# ⚠ 生成器版本会体现在 .pb.go 的版本注释行；diff 里冒出"没动过的 proto"对应的文件时先看这一行

# ── daemon 侧验证 ──
cd daemon && PATH="/c/go/bin:$PATH" go build ./... && go test ./internal/...
# Android 单测必须核验结果 XML 时间戳（Gradle 会 UP-TO-DATE 假绿）；test 分支已内置 --rerun
# 体积必须走 cleanapk（AGP 增量留孤儿 dex）；daemon 热部署 tools/device/_pa_rebuild_daemon.py
```

> 鉴权默认开启（PA-REAL-0001 已修）：无 `-pair-code` 时自动生成配对码；
> `-insecure-no-auth` 显式关鉴权但**仅允许本机回环连接**，局域网来源一律拒绝。
> 已配对存量手机凭旧 token 不受影响；升级重启复用 `pair-code.txt` 不换码。
