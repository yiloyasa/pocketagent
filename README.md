# PocketAgent — 项目整体上下文

> 手机上的 AI 编程代理遥控器。**手机只做遥控，agent 跑在你自己的电脑上，全程零云服务器。**
>
> **本文件是项目唯一的上下文文档** —— 功能、架构、模块、数据流、入口、配置、依赖、现状、坑，全部收敛在这里。
> 过程性文档（调研/计划/评审）已舍弃（台账 `_archives/_docs-discarded_20260920-manifest.json`）。

---

## 1. 系统是什么

```
手机 app ──( WiFi/4G )──► 家里电脑的 daemon ──► 原生 agent ──► 大模型 API
   │                          │                 (codex / pi / zcode)
   │  "看看日志,告诉我哪错了"     │ 桥接事件流/审批    │ 读你项目的文件
   │                          │                 │ 跑个命令试试
   │ ◄── 逐字回复 + "我要跑 rm 命令,允许吗?"
   │      "允许" ──►           │ 转发审批决定 ──►   │ 真的执行
   │ ◄── "修好了,已提交 commit"
```

- **手机 = 遥控器**（显示与输入）；**daemon = 桥**（翻译事件、中继审批、托管子进程会话）；
  **原生 agent = 干活者**（agent 循环跑在 codex/pi/zcode 自己的进程里，daemon **不重写**）
- 连通：**USB `adb reverse` → 局域网(含 mDNS 自动发现) → Tailscale** 三档降级，无云中继

### 功能面

会话聊天（流式回复/工具卡片/审批弹卡/思考过程）、斜杠命令、语音输入（系统识别优先→端侧 SenseVoice 兜底）、
真终端（xterm.js ↔ PTY）、文件浏览/编辑/预览（Office/PDF 内置解析）/下载、Git 面板（状态/diff/提交）、
会话组织（分组/搜索/归档/fork/回滚/导入原生会话）、用量与上下文环、任务中心、内置浏览器、
离线缓存与断线续传（事件重放 + 审批恢复）、mDNS 自动发现、多设备配对与 token 续期。

---

## 2. 技术栈与第三方依赖

| 层 | 技术 | 关键第三方 |
|---|---|---|
| Android | Kotlin + Jetpack Compose（BOM 2024.09 / M3），minSdk 26 / target·compileSdk 35 | `grpc-okhttp`+`grpc-protobuf-lite` 1.66（gRPC 客户端）；`security-crypto`（Keystore 加密 token）；`webkit`（xterm.js 容器）；`navigation-compose`；`kotlinx-coroutines`；**sherpa-onnx 1.13.8 AAR**（SenseVoice 端侧识别，构建时下载校验）；`vosk-android` **已整体摘除（2026-09-20）**：引擎选择收敛为"系统识别 → 端侧 SenseVoice"两级，共享错误码迁至 `VoiceErrors.kt` |
| daemon | Go **1.27.1** | `grpc` 1.83.2、`protobuf` 1.36.12、`go-pty`（PTY）、`zeroconf`（mDNS）。**依赖极简，无 ORM/无框架** |
| 协议 | protobuf / gRPC `pocketagent.v1` | **6 服务 / 64 RPC**：Thread 29、Git 10、Fs 9、Model 8、Host 5、Terminal 3（7 个 proto / 1353 行） |
| 终端 | xterm.js 6（WebView）+ JS bridge | ↔ daemon `go-pty`，字节流一致画面一致 |

> 图标全用 lucide 矢量（`ic_lucide_*`）+ 品牌 PNG，**故意不引** `material-icons-extended`（曾占 DEX 一半）。

---

## 3. 目录结构与模块职责

```
pocketagent/
├── proto/v1/                 # gRPC 契约（唯一真源；android/app/src/main/proto/v1/ 是同步副本，MD5 必须一致）
├── daemon/                   # Go 桌面宿主
│   ├── cmd/pocketagentd/     # 入口：flag → TLS → 组装 → gRPC server
│   ├── cmd/<probe>*37        # 一次性探针/工具（modelprobe、chatprobe、paguard、smoketest…）
│   └── internal/
│       ├── gen/              # protoc 生成的 Go 代码（勿手改）
│       ├── server/           # ★ gRPC 服务实现层（63 文件）
│       ├── agent/            # ★ Provider 抽象 + 5 实现 + 反向请求分派（78 文件）
│       ├── codexapp/         # codex app-server 子进程封装（JSON-RPC 2.0 over stdio）
│       ├── piapp/            # pi --mode rpc 子进程封装（1 子进程=1 会话）
│       ├── zcodeapp/         # zcode.cjs app-server 封装 + 账户桥接（只读适配）
│       ├── store/            # FileStore：threads.json 持久化（会话/设置）
│       ├── eventstore/       # events.jsonl 全局单调 seq 事件日志（10MB 轮转，断线重放）
│       ├── term/             # PTY 会话管理 + 空闲回收 reaper
│       ├── mdns/             # _pocketagent._tcp 局域网宣告（失败仅警告，不崩）
│       ├── tlscert/          # 自签证书（ECDSA P-256，20 年，首启生成复用）
│       ├── hidewin/          # Windows 隐藏子进程窗口
│       └── version/          # 版本号
├── android/app/
│   ├── src/main/java/dev/pocketagent/app/
│   │   ├── MainActivity.kt        # 入口 + NavHost：onboarding/home/chat/{id}/terminal/browser?url=/settings
│   │   ├── AgentConnection.kt     # ★ 单例连接管理：地址/token/配对/挑战续期/并行竞速候选
│   │   ├── TokenStore / DeviceKey / PinTrustManager / ConnectionDiagnostics
│   │   ├── TurnKeepAliveService.kt # dataSync 前台服务（超时停服务不崩溃）
│   │   ├── ui/                    # 各屏（Chat/Terminal/Files/Git/Settings/Home/Threads/Browser/TaskCenter…）
│   │   │   │                       # + 逻辑类（TurnSession 会话状态机、HistoryMessageMapper、
│   │   │   │                       #   TurnDelivery、SlashCommandsLogic、ThreadOrganize…）
│   │   │   └── component/         # 组件库（SplitButton/PaBadge/FileKind/PaDownloader/OfflineStore/
│   │   │                           #   MdnsDiscovery/SenseVoice*/VoiceErrors…）
│   │   └── src/main/proto/v1/     # 契约副本（与 proto/v1/ 同步）
│   ├── src/main/assets/           # sensevoice 模型(229MB，构建时下载)+terminal.html
│   ├── src/main/libs/             # sherpa-onnx AAR（构建时下载校验）
│   └── src/test/                  # JVM 单测 67 文件 / 7514 行
├── codex-schema/             # codex app-server 官方 JSON Schema 快照（能力核对，勿手改）
├── tools/                    # make_icons 等资源生成 + device/ 真机工具 + ops/ 运维工具
├── dist/windows/             # 桌面端分发包（build.ps1 → zip + install/uninstall）
├── docs/verify/              # 验收证据（evidence/ 结论入库、截图不入库）+ testdata
└── build/                    # APK 产物（gitignored）
```

### daemon 侧核心职责划分

| 模块 | 职责 | 实现要点 |
|---|---|---|
| `cmd/pocketagentd` | 启动组装 | flag：`-addr`(默认 :7246)、`-data`(默认 `~/.pocketagentd`)、`-pair-code`(**不传=自动生成 8 位随机码**并持久化到 `pair-code.txt`，显式指定则同步写回文件；**鉴权默认开启**，PA-REAL-0001 已修)、`-insecure-no-auth`(显式关闭鉴权，**仅放行本机回环**连接)、`-workspace`、`-plaintext`(应急)、`-mdns`、`-term-idle`(默认 10m 回收)。keepalive 25s/10s（移动网尽快检出假连接） |
| `server` | 6 个 gRPC 服务 + 鉴权拦截器 | `server.go` `checkAccess()`：默认验 token（每设备 token / legacy token = sha256("pair-code:"+code) 比对 metadata）；PairCode 为空 fail closed（不再有"空码=关鉴权"）；`-insecure-no-auth` 仅放行本机回环 peer。`thread.go` `StartTurn` 双向流是心脏；`turn_queue.go` 运行中插话（codex 支持 steer，其余入内存队列终局后续跑）；`turn_transport.go` 背压：**同步顺序落盘 eventstore + 单写者 gRPC writer**，慢手机不拖垮原生事件循环；`approval_resume.go` 断线后按 thread_id/request_id 补交审批（过期/重复/跨线程拒收，上限 2 分钟） |
| `agent` | Provider 抽象 | `provider.go` 定义 `Provider/Sinker/PrefsAware`；`DetectProviders()` 启动探测注册。5 实现：**codex**(主力)、**pi**、**zcode**、openai_compat(唯一直连自跑循环)、echo(冒烟)。`codex_requests.go` **按方法名精确匹配**分派 10 个反向请求（审批应答枚举分新旧协议：`approved/denied` vs `accept/decline`，混用判无效；没能力的回 JSON-RPC error，**绝不伪造 decision**） |
| `codexapp` | codex 子进程 | `codex app-server` stdio JSON-RPC；自带 agent 循环/沙箱/审批/MCP/登录/会话——**我们只做翻译官**（与原作 Mobile Codex 同款架构） |
| `piapp` | pi 子进程 | `pi --mode rpc`；Thread ↔ 子进程 **1:1**；daemon 重启后按 SessionFile 重挂载（switch_session + get_state 校验握手不能省） |
| `zcodeapp` | zcode 子进程 + 账户 | 只读适配：读桌面宿主账户 overlay（family/profile→scoped key→权益→`provider/updateAccountConfig`），模型/档位全由 worker 返回**不造静态名单**；仅 Individual Coding Plan |
| `store` | threads.json | 会话、默认工作区/模型/effort 等；Thread 的 ExternalSession 落库供重启重挂载 |
| `eventstore` | events.jsonl | 每帧带全局单调 seq 落盘；手机重连 `ResumeFrom(seq)` 续传不丢不重 |

### android 侧核心职责划分

| 模块 | 职责 | 实现要点 |
|---|---|---|
| `AgentConnection` | 单例连接 | gRPC OkHttp channel；token 走 metadata；**并行竞速候选**（reverse→LAN→Tailscale 同时拨，先通先用）；配对携设备身份（UUID+P-256 公钥）领每设备独立 token；token 失效走**挑战应答续期**（Keystore 签 `device_id:challenge`） |
| `TokenStore`/`DeviceKey`/`PinTrustManager` | 安全 | token 经 EncryptedSharedPreferences（Keystore AES256-GCM）加密落盘，ROM 异常降级明文 prefs 不断连；证书指纹锁定，变化即阻断 |
| `TurnSession`/`TurnDelivery` | 会话状态机 | callbackFlow 收流（`.buffer(UNLIMITED)` 治丢帧）；流式已读文字定期落盘（文字+seq 原子保存）；恢复走**事件回放 + 原生历史兜底，绝不重发原 prompt** |
| `ChatScreen` 系 | 聊天 UI | 模型选择独立整行；语音引擎选择两级：系统识别优先 → SenseVoice 端侧兜底（模型打进 APK 首次解压校验；旧 Vosk 引擎已整体摘除，2026-09-20） |
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
            → codexapp/piapp/zcodeapp 子进程 JSON-RPC
            → 原生 agent 执行（读文件/跑命令/改代码）
  ← 事件流沿 Sink 回传 → turn_transport（先同步写 eventstore 拿 seq，再单写者推 gRPC）
  ← 手机 callbackFlow → 逐字上屏 + 工具卡片 + 用量更新
```

**② 审批反向流**：原生 agent 发 `item/*/requestApproval`（codex 10 个反向请求之一）→ `codex_requests.go` 精确匹配分派 → 转 `ToolCallBlock` 推给手机弹卡 → 用户决定经 `SubmitApproval`(thread_id+request_id) 回传 → daemon 翻译成对应协议的应答枚举。断网期间待审批保留，重连后可补交。

**③ 断线重放**：每帧事件带全局 seq 落 `events.jsonl` → 手机重连 `ResumeFrom(seq)` → daemon 从 eventstore 补发缺失帧 → 无丢无重。daemon 重启则从原生历史(session snapshot/resume)恢复终态。

**④ 会话挂载**：Thread.ExternalSession(SessionID/SessionFile) 落 threads.json → daemon 重启后按 agent 各自方式重挂载（codex thread/resume；pi switch_session+get_state；zcode session/resume）——**不 resume 就静默吞请求**是三方共同教训。

---

## 5. 入口与配置

| 类别 | 位置 | 说明 |
|---|---|---|
| daemon 入口 | `daemon/cmd/pocketagentd/main.go` | 见 §3 flag 表；运行参数：`-addr 0.0.0.0:7246 -workspace G:\项目1\pocketagent`，env `CODEX_BIN=G:\Claude Code\codex.cmd` |
| daemon 数据目录 | `~/.pocketagentd/` | `threads.json`(会话真值)、`events.jsonl`(+轮转)、`devices.json`(配对设备)、`pair-code.txt`(配对码，0600，重启复用不换码)、`certs/`(自签证书) |
| App 入口 | `MainActivity.kt` → NavHost | 冷启 Splash → onboarding(未配对)/home |
| App 构建 | `android/build.gradle.kts` | versionCode/versionName 与 daemon `internal/version` 同步（现 0.2.2/5）；sherpa AAR 由构建脚本下载+哈希校验 |
| 构建脚本 | `tools/device/_pa_gradle.py`、`android/build-ascii.ps1` | 中文路径 Gradle 处理不了 → robocopy 镜像到 `G:\pa\android` 再 gradlew |
| 看门狗 | `daemon/cmd/paguard` + `watchdog_hidden.vbs` | 计划任务 PocketAgentDaemonGuard 每分钟 → wscript → WMI 拉起 paguard → 探测 7246 死了才拉 daemon（WMI 派生脱离 job object，实测 4s 内拉起）。paguard 运行中（2026-09-20 核对：watchdog.log 每分钟有探测记录；旧锚点"未运行"系漂移，T-30 纠正） |
| 分发 | `dist/windows/build.ps1` | 打包 daemon+paguard+install/uninstall → `PocketAgent桌面端.zip` |

---

## 6. 外部服务与能力边界

| 依赖 | 用途 | 边界 |
|---|---|---|
| **codex CLI**（CODEX_BIN） | 主力原生 agent | 本机 token 403 时好时坏（地区限制）；模型窗口多数真未知（schema 无字段，UI 显"—"是如实） |
| **pi CLI** | 原生 agent | 1 子进程=1 会话；同 provider 多会话 turn 锁排队，**不可宣称并行** |
| **ZCode 桌面端** | 原生 agent | 仅 Individual Coding Plan；Team/Start/Off-peak、OAuth 刷新未实现 |
| GLM / DeepSeek API | BYOK 直连（env key） | 唯一 daemon 自跑 agent 循环的路径 |
| Tailscale | 外网通道 | **PC 侧已装方案，非内嵌**（P2 内嵌 libipn 未做） |
| mDNS (`_pocketagent._tcp`) | 局域网自动发现 | 多播失败只丢便利性不崩 |
| 云服务器 | **无** | 零云是产品原则；`server/` 目录不存在 |

**红线**：绝不改写用户桌面 `~/.codex/config.toml`；粘贴长文本不中断是 stdio 通道既有事实（UI 写"已内置"，`turnInput()` 零裁剪，单测钉 24 万字）；没能力的原生请求回 error 不伪造 decision。

---

## 7. 设计原则：未来未知事件接口（能透传就透传）

**核心立场**：原生 agent（codex/pi/zcode）会升级、会改模型、会发新事件。daemon/App 不能假设「我认识全部」，更不能把「我不认识」当成「出错」一脚踩死。

### 三层含义

1. **识别不了 ≠ 出错** —— 原生发来 daemon 没见过的通知/字段，默认定性是「数据」不是「异常」；能原样透传就透传，不丢弃、不报警、不伪造。
2. **接不了 ≠ 接死** —— 暂时没能力处理的反向请求，回 error（如实告知「不支持」），**绝不回伪造的 decision**（伪造一个"同意"会让原生以为用户点了同意，后果不可逆）。
3. **静态名单是债** —— 凡在代码里写死「模型白名单 / agent 名单 / 事件名单 / 版本号二分判断」，都是给未来埋的雷。原生会新增模型、新增 agent、新增事件、升级到假设外的版本——写死的代码要么漏（新东西显示"未知"），要么错（旧判断踩到新场景）。

### 当前已合规的行为（保持）

| 位置 | 行为 |
|---|---|
| `internal/server/model.go` ListModels | **无模型白名单**，原生返回什么透传什么；新增模型/effort 自动出现在 App |
| `internal/agent/codex_events.go` 默认分支（L408） | 未识别事件**静默丢弃**（不伪造、不上报）——「伪造只会淹掉真正的事件」 |
| `internal/agent/codex_requests.go` | 不支持的反向请求回 `reqUnsupported`（JSON-RPC error），**不伪造 decision** |
| `internal/agent/zcode*.go` | 模型名/窗口/reasoning **全部运行时返回**，无静态 GLM 名单 |
| `internal/agent/native_notifications.go` | "Unknown siblings remain untouched" —— 不认识的兄弟节点原样保留 |
| `turn_queue` | 中途插入降级为排队，不报错 |

### 已踩死 / 待整改的点（技术债）

| 位置 | 问题 | 影响 |
|---|---|---|
| `internal/server/thread.go` CreateThread（L99） | 未知 agent → `errFailedPrecondition` **硬错误** | 新增第 4 个 agent 时 App 直接报错，不能「尽力接入」。**已整改（2026-09-20, 批3）**：agent_id 在 `s.cfg.Providers` 注册表即接入，报错文案动态列举当前可用 agent，不写死名单 |
| `internal/server/model.go` `order := []string{"codex","pi","zcode"}`（L73） | **agent 顺序写死** | 新 agent 排不进列表。**已整改（2026-09-20, 批3）**：顺序从 `s.cfg.Providers` 动态派生（桌面桥接 PrefsAware 在前、其余按字典序、echo 最后），新 provider 自动入列 |
| `internal/server/model.go` `defaultModelIDFor` | 假设 `agent+"/default"` 一定存在 | 原生删掉默认模型后返回一个不存在的 ID。**已整改（2026-09-20, 批3）**：default 哨兵 → 模型列表首项 → 明确报错（调用方给人话），不再伪造 ID |
| `internal/agent/codex_requests.go` `isLegacyApproval` | 按版本号**二分判断** | codex 升级到假设外的版本会踩错分支 |

### 决策（2026-09-20 拍板）

- **主线 = A「能透传就透传」**：未知通知/字段，daemon 不丢弃，**原样包装**（原文 + 原方法名）透传到 App 诊断视图；App 不显示「错误/未知」，显示「原生发来了一个新事件」。
- **辅线 = B「把决定权还给用户」**：未知反向请求（如新型审批），daemon 不伪造 decision，**透传请求体**让 App 弹通用确认框（同意/拒绝/忽略）。**做成全局设置开关，默认关**（避免普通用户被技术细节打扰；开发者/进阶用户可开）。
- **不选 C「协议版本探测」**：维护成本高，仍是「假设我知道所有版本」的思路，与「能透传就透传」相悖。

---

## 8. 审核（强制执行）

**任何更改合入前，必须严格按 `AUDIT.md` 执行，无例外。** `AUDIT.md` 是项目唯一审核标准，审核事项以它为准。要点速记：

- **变更分级**（`git diff` 路径自动定级，取最高、不许降级）：**甲**=契约/架构（proto、新 agent）→ L0+L1+L2 全量；**乙**=功能（daemon agent/server、android ui）→ L1 全量+L2 受影响；**丙**=修复 → L1+单条复验+附回归用例；**丁**=仅文档 → 只查锚点漂移
- **三层**：L0 闸门（产物自证 apkanalyzer）→ L1 自动化（`--rerun`+XML 时间戳防假绿）→ L2 真机（BLOCKED 必带归因+解封条件）
- **红线一票否决**：伪造 decision / 假绿 / 契约不同步 / 产物不对；**提示项不阻塞**：锚点漂移、证据缺项限期补录
- **设计原则合规**：新增硬编码名单即报警（存量挂牌仅剩 1 处 isLegacyApproval，其余三处已整改；豁免 3 项见 AUDIT 附录 A，只减不增）；UI 禁显「未知/错误」，必须显示原生原文
- **缺陷即测试**：修复不附回归用例 = 没修完；审核资产只增不减
- 完整标准、checklist、linter 规则、准出 DoD、度量回流 → **`AUDIT.md`**

---

## 9. 快速开始

```bash
# ── daemon（电脑上）──
cd daemon
go run ./cmd/pocketagentd -addr 0.0.0.0:7246 -workspace "你的项目目录"
# 首启自动生成 8 位配对码：打印在启动日志，并持久化到 ~/.pocketagentd/pair-code.txt（重启复用不换码）
# 手机首次连接时输入该配对码完成配对，之后凭每设备 token 直连，无需再输。
set CODEX_BIN=G:\Claude Code\codex.cmd                                    # 桥接 codex（推荐）

# ── APK（必须走镜像包装器，中文路径 Gradle 处理不了）──
python tools/device/_pa_gradle.py apk|cleanapk|test|compile
adb uninstall dev.pocketagent.app && adb install build/app-debug.apk     # 小米必须先 uninstall

# ── daemon 侧验证 ──
cd daemon && PATH="/c/go/bin:$PATH" go build ./... && go test ./internal/...
# Android 单测必须核验结果 XML 时间戳（Gradle 会 UP-TO-DATE 假绿）
```

> 鉴权默认开启（PA-REAL-0001 已修）：无 `-pair-code` 时自动生成配对码；
> `-insecure-no-auth` 显式关鉴权但**仅允许本机回环连接**，局域网来源一律拒绝。
> 已配对存量手机凭旧 token 不受影响；升级重启复用 `pair-code.txt` 不换码。

---

## 10. 当前状态（真值锚点）

| 项 | 值 |
|---|---|
| Git | 分支 `main`｜夜航批次提交链至 `b021fe6`（2026-09-21 晨）｜新功能批次（选择卡保真/历史按轮加载）审核后收口 |
| 版本 | **0.2.2 / versionCode 5**；daemon 启动日志含版本字段、支持 `-ldflags -X` 烙印（产物自证锚点，批7 T-27） |
| 契约 | 6 服务 / 64 RPC，7 proto / 1353 行 |
| 代码 | daemon 手写 Go 234 文件 / 48559 行（agent 78 + server 63 为主）；android main 75 `.kt` / 30965 行 |
| APK | debug **209.01 MB**（SenseVoice INT8 模型打包内；Vosk 已摘除 + ABI 收窄后较 09-19 基准 -29MB），`sha256=f34493a42174bd07885c4a37c0eef640f10898be73f21ecfe58371eb036b84e0` |
| daemon | pid 21440（旧无鉴权版），`0.0.0.0:7246` LISTENING；paguard 运行中（换血由夜航部署环节执行） |
| P0 | **PA-REAL-0001 代码已修**（daemon 默认配对码鉴权 + `-insecure-no-auth` 仅回环，回归用例已附）；SEC-03/04/05/11/12 真机重跑待手机插线 |
| 体积 | `pocketagent/` 642 MB（含 `.git` 28 MB）；工作区 `_archives/` 155 MB |

**测试基线**（2026-09-21 更新）：Android 70 套件 / 517 用例 / 0 失败；`go test ./...` PASS；`-race`(server/agent/store/codexapp/piapp/zcodeapp) PASS。

**发布审核：暂缓验收**（证据 `docs/verify/evidence/2026-09-19-device-build/`）：131 用例 = 30 PASS / 2 FAIL / 99 BLOCKED；
P0=鉴权未启用（**代码已修，见上表 P0 行**；真机复验待手机插线）；P1=模型列表首调 30s 超时、跨 Agent 探测被 Codex 初始化阻塞（**代码已修**，批4 T-14，待真机复验）；P2=按钮 <48dp（**代码已修**，批5 T-18，待真机 bounds 复验）、引导页"下载 42MB"文案失实（**已修**，批2）、reverse 在场仍走 LAN（**已修**，批2，待真机复验）。
已证真值：冷启 10/10 avg 700.8ms；三 Agent 零交叉污染；TLS1.3；崩溃/ANR/数据丢失 0。debug 包（debuggable）安全结论不可外推 release。

---

## 11. 已知边界与技术债

- daemon `go vet` 有 protobuf 值拷贝警告（存量噪音，非新问题）
- 同 provider 多会话不并行（原生 turn 锁排队）
- 上下文窗口多数模型真未知（`ListModels` 仅 `codex/default`/`echo-1` 带窗口；codex 不发该字段）
- **Vosk 已摘除**（2026-09-20，批5）：`vosk-android` 依赖 + `Vosk*` 两文件 + 引擎选择分支全部移除；102/103/MicUnavailableException 等共享定义迁至 `VoiceErrors.kt`；行为零变化（摘除前引擎恒不可选）
- P2 内嵌 Tailscale、P3 GeckoView/自更新/实况通知均未做
- 审核遗留：86 张 2026-09-16 证据截图已出库（2026-09-20，仅出 git 索引、磁盘保留，嵌套 .gitignore 防再入库）；`dist/windows` zip 版本未核（2026-09-20 核对：zip 打包于 2026-09-17，内含 daemon 二进制**无版本标识**——早于 `internal/version` 包引入，代码落后于仓库 0.2.2，随下次发版更新）

---

## 12. 关键坑（开发必读）

1. **"源码改对 ≠ 跑着的是新的"**（头号假故障）：报"没生效"前先 `apkanalyzer dex packages` 查新类 + dex 里 grep 新中文字面量。
2. **Kotlin 增量缓存失配**：大批 `Unresolved reference` 指向没改的文件 ⇒ 物理删 `app/build`+`.kotlin` 重跑；但错误集中在刚改的文件且首条是操作符 ⇒ 只是缺一行 import。
3. **Kotlin 块注释可嵌套**：注释里写 `video/*` 会吞到文件尾；修正时字面「星号+斜杠」会提前闭合，报错一样。用 `tools/device/_pa_scan_nested_comment.py`（词法状态机）自查。
4. **StateFlow 按 equals 发射**：原地改字段永不刷 UI，要刷的用 `mutableStateOf` 委托。
5. **Composable 里 `return` 截断渲染**：底部弹层写在 `return` 之前。
6. **真机驱动**：`adb reverse` 每条命令后可能被回收（脚本开头 `ensure_reverse()`）；`input tap` 丢事件（dump 校验+自愈重试）；BACK 键打回桌面（一律 force-stop+冷启动）；被滚动裁掉的节点 dump 里不存在（先滚再找）。
7. **小米安装**：必须先 uninstall 再 install（`-r` 静默失败）。
8. **PowerShell 工具不回传输出** → 验证一律走 Python subprocess（`encoding='gbk'`）；Bash 禁调 powershell.exe；`schtasks/cscript/cmd` 黑名单勿重试。
9. **Windows 大目录删除加 `\\?\` 长路径前缀**；批量删除先出 manifest。
10. Write 工具会把源码 `\u0000` 写成真 NUL 字节（`read('rb').replace()` 修回）。

---

## 13. 运维：日志与清理

- **日志 5 类**：A 运行(daemon.log/watchdog.log，轮转只留错误行)｜B 验收证据(`docs/verify/evidence/**`，磁盘不可清理、**截图不入库**)｜C 构建一次性｜D 归档冷数据(`_archives/`，90 天)｜E 错误汇总(长期)
- **铁律**：成功日志不长期留盘；`*.log` 禁止入库；归档只搬不删；运行中的日志禁止直接删
- **工具**（`tools/ops/`）：`archive_logs.py` / `rotate_runtime_logs.py`(SHA256 校验后才截断) / `clean_junk.py` / `cleanup_project.py`(精确保护清单) / `cleanup_cdrive.py`
- **勿动**：运行中的 `pocketagentd.exe`+`watchdog.log`；两处 `.pocketagent-uploads/`（daemon 活数据）
- **删除台账**：`_archives/_docs-discarded_20260920-manifest.json`(109 文档)、`_residual-cleanup_20260920-manifest.json`(97MB 残留)
- **可删再生**：`G:\pa\android`+`C:\pa\android` 镜像、`pocketagent/build/`、`android/.gradle`；`_archives/_trashbackup_20260919/`(154MB，`_backup-unique/` 建议留)

---

## 14. 文件/下载支持速查

判定唯一源 `ui/component/FileKind.kt` 的 `preview` 字段。内置预览：md/文本/json/html/csv/图片/**xlsx+xls(自研 OLE2/BIFF8)**/ods/docx/rtf/odt/pptx/odp/pdf(PdfRenderer)；
只交系统：doc/ppt/apk/音视频；只保存：压缩包/exe。边界：Office 只还原内容不还原版式；单读 64MB 上限；表格 800×40×3、PDF 12 页、PPTX 40 页。
