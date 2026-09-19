# PocketAgent

> 手机上的 AI 编程代理遥控器。**手机只做遥控，agent 跑在你自己的电脑上，全程零云服务器。**
>
> **本文件是项目唯一的上下文文档** —— 架构、现状、交付、运维全部收敛在这里。
> 其余过程性文档（调研 / 计划 / 评审 / 交接）已按"只留整体上下文"的决定舍弃。

---

## 1. 是什么

想象你在公司，家里电脑上有个项目跑挂了。你掏出手机：

```
手机 app ──( WiFi/4G )──► 家里电脑的 daemon ──► 原生 agent ──► 大模型 API
   │                          │                 (codex / pi / zcode)
   │  "看看日志,告诉我哪错了"     │ 桥接事件流/审批    │ 读你项目的文件
   │                          │                 │ 跑个命令试试
   │ ◄── 逐字回复 + "我要跑 rm 命令,允许吗?"
   │      "允许" ──►           │ 转发审批决定 ──►   │ 真的执行
   │ ◄── "修好了,已提交 commit"
```

- **手机只是遥控器**（显示和输入）
- **daemon 是桥**：把手机和电脑上真正的 agent 接起来
- **原生 agent 是干活的**：读文件、跑命令、改代码
- **大模型是大脑**：决定下一步干什么

> 关键：daemon **不自己重写 agent 循环**，只做三件事 ——
> ① 把原生 agent 的事件流翻译成 gRPC 消息；② 把手机上的审批决定转发回原生 agent；
> ③ 托管原生子进程的会话与重连。

---

## 2. 组成与布局

```
pocketagent/
├── proto/v1/      协议契约（gRPC，6 服务 / 64 RPC；daemon 与 Android 各一份副本）
├── daemon/        Go 桌面宿主：原生 agent 桥接 + PTY + 文件 + git
├── android/       手机 app（Kotlin + Compose + xterm.js）
├── codex-schema/  Codex app-server 官方 JSON Schema 快照（能力核对用）
├── tools/         资源生成 + device/（真机工具）+ ops/（运维工具）
├── dist/windows/  桌面端分发包（6.4 MB：daemon + paguard + install/uninstall）
├── docs/verify/   发布验收证据（截图/日志/冻结哈希/缺陷台账）—— 保留
└── build/app-debug.apk   APK 产物（gitignored）
```

> **没有 `server/` 目录**：本项目零云服务器。连通性靠 **USB `adb reverse` → 局域网 → Tailscale** 三档降级。

### 工作区（`G:\项目1`）

| 目录 | 用途 | 大小 |
|---|---|---|
| `pocketagent/` | 源码仓库（git） | **642 MB**（含 `.git` 28 MB） |
| `_archives/` | 清理隔离备份（见 §9.3） | 155 MB |
| `.pocketagent-uploads/` | 手机上传的截图/附件（daemon 接收，活数据） | 0.5 MB（根）/ 2.5 MB（`pocketagent/` 内） |
| `.workbuddy/` | 助手记忆（非项目文档） | 276 KB |

### 有意保留的"重复"（构建必需，不是垃圾）

| 文件 | 两份位置 | 原因 |
|---|---|---|
| 7 个 `*.proto` | `proto/v1/` 与 `android/app/src/main/proto/v1/` | daemon 与 Android 各需一份在自己的源码树里 |
| `Poppins-Bold.ttf` | `tools/fonts/` 与 `android/.../res/font/poppins_bold.ttf` | 工具脚本素材 + App 打包资源 |
| 整个 android 源码树 | 仓库 与 `G:\pa\android`（仓库外） | **构建镜像区**（见 §4.2） |

---

## 3. agent 怎么接

daemon 桥接电脑上现成的原生 agent（**不重写它们的循环**）：

| Agent | 桥接方式 | 说明 |
|---|---|---|
| **Codex** | `codex app-server`（stdio JSON-RPC） | 主力路径 |
| **Pi** | `pi --mode rpc` | 一个子进程 = 一个会话（Thread ↔ 子进程 1:1） |
| **ZCode** | `zcode.cjs app-server --stdio` | 账户桥接 + 会话映射 |
| OpenAI 方言 | `openai_compat.go` | 直连 GLM / DeepSeek（**唯一由 daemon 自己跑循环**） |
| echo | `echo.go` | 假回显，无 key 也能跑通全链路 |

`Provider` 接口（`daemon/internal/agent/provider.go`）是唯一抽象，上述 **5 个实现**；
后台按会话的 `agent_id` 选一个（**默认 `codex`**）。

**两条接入路径**（`DetectProviders()` 启动时"谁能用就注册谁"）：

| 条件 | 注册的 Provider | key 从哪来 |
|---|---|---|
| 装了 codex CLI | `codex` | **推荐**：用 Codex 自己的登录，daemon 不需要 key |
| 装了 node + pi CLI | `pi` | Pi 自己的账户 |
| 装了 node + ZCode | `zcode` | ZCode 账户桥接 |
| 设了 `GLM_API_KEY` | `glm` | 智谱 open.bigmodel.cn |
| 设了 `DEEPSEEK_API_KEY` | `deepseek` | DeepSeek platform.deepseek.com |
| 总是 | `echo` | 无 |

---

## 4. 快速开始

### 4.1 启动 daemon（在电脑上）

```bash
cd daemon

# 无任何 key 也能跑（echo 模式，全链路可联调）：
go run ./cmd/pocketagentd -addr 0.0.0.0:7246 -workspace "你的项目目录"

# 桥接桌面端 Codex（装了 codex CLI 且已登录即用）：
set CODEX_BIN=G:\Claude Code\codex.cmd
go run ./cmd/pocketagentd -addr 0.0.0.0:7246 -workspace "你的项目目录"
```

> ⚠ **当前候选 daemon 以无鉴权模式运行**（启动命令无 `-pair-code`，`daemon/internal/server/server.go:203` 的
> `authorized()` 返 nil）→ 同一局域网任意主机可无凭据驱动 agent。
> 这是未关闭的 **P0 缺陷 PA-REAL-0001**。**修它之前，别在不可信局域网里暴露 7246 端口。**

### 4.2 构建手机 app

`G:\项目1\…` 是**中文路径**，Gradle 的 `protoc` 与 javac 处理不了；junction **也没用**（Gradle 会把项目目录规范化回真实中文路径）。唯一可靠办法是**镜像到 ASCII 路径再构建**：

```bash
python tools/device/_pa_gradle.py apk        # assembleDebug
python tools/device/_pa_gradle.py cleanapk   # clean + assemble（体积数据必须走这条）
python tools/device/_pa_gradle.py test       # :app:testDebugUnitTest
python tools/device/_pa_gradle.py compile    # 只编译 Kotlin
```

- 镜像目标：**`G:\pa\android`**（`_pa_gradle.py` 与 `android/build-ascii.ps1:31` 一致）
- 产物：`build/app-debug.apk`（debug **238 MB**，含内置 SenseVoice INT8 模型 228 MB）
- 镜像目录**可删**，下次构建自动重建（首构慢 1–2 分钟）
- **体积必须走 `cleanapk`**：AGP 增量会在 APK 留孤儿 dex，只有清空 build 后重建才可信

### 4.3 安装到真机

```bash
adb uninstall dev.pocketagent.app      # 小米：先 uninstall 再 install（-r 会静默失败）
adb install build/app-debug.apk
```

### 4.4 玩什么

- 会话页 → 新建会话 → 发消息 → 看流式回复、工具调用过程、审批弹卡
- 顶部终端图标 → 真 PowerShell/bash 终端（xterm.js ↔ 电脑 PTY）

---

## 5. 环境与路径

| 项 | 值 |
|---|---|
| 仓库 | `G:\项目1\pocketagent`（git main）；**`C:\dev\pocketagent` 已不存在**（junction 早已解除） |
| daemon | pid **21440**，监听 `0.0.0.0:7246` 与 `[::]:7246`；`paguard` **当前未运行** |
| Go | `C:\go\bin`（`go.mod` 要求 **Go 1.27.1+**） |
| Gradle / protoc | `C:\gradle-8.9` / `C:\protoc\bin`（安装包已删，不必留） |
| JDK | `…\Android Studio\jbr`（JDK 17） |
| Android SDK | `C:\Android`（compileSdk 35 / minSdk 26 / targetSdk 35） |
| adb | `C:\Android\platform-tools\adb.exe` |
| 构建镜像 | `G:\pa\android`（另有遗留 `C:\pa\android`） |
| daemon 探针二进制 | `G:\pa\bin`（`modelprobe.exe` / `chatprobe.exe` / `wbfix.exe` …，源码在 `daemon/cmd/`） |
| store 真值 | `C:\Users\yanzi\.pocketagentd\threads.json` |
| 运行环境限制 | **PowerShell 工具不回传输出** → 看结果一律走 Python；Bash 里**禁止调 powershell.exe**（安全策略拦）；`schtasks`/`cscript`/`cmd` 在黑名单，**不要重试** |

---

## 6. 当前状态（2026-09-20 实测）

**真值锚点**

| 项 | 值 |
|---|---|
| Git | 分支 `main`；清理收口提交 `fc3b806`（2026-09-20）｜工作树**干净（0 未提交）** |
| 版本 | **0.2.2 / versionCode 5** |
| 契约 | **6 服务 / 64 RPC**（Thread 29、Git 10、Fs 9、Model 8、Host 5、Terminal 3），7 个 proto / 1347 行 |
| 代码 | daemon 手写 Go **234 文件 / 48559 行**（测试 112/17455、`cmd` 42/7991、生产 80/23113）；生成 13/15499；android main 74 `.kt` / 31323 行、test 62 / 7053 |
| APK | debug **238.13 MB**，`sha256=554c87b1359de446…` |

**已实现**：三 Agent 原生接入（codex/pi/zcode）、直连 BYOK、真 PTY、文件浏览/编辑、git 面板、流式聊天 + 审批、内置 SenseVoice 离线语音、断线续传、会话组织（分组/搜索/归档/fork/回滚）、斜杠命令。

**发布审核结论：暂缓验收**（证据 `docs/verify/evidence/2026-09-19-device-build/`）

- 131 用例 = **30 PASS / 2 FAIL / 99 BLOCKED**
- **P0 未关闭 PA-REAL-0001**：daemon 无 `-pair-code` → 局域网任意主机可无凭据驱动 agent
- P1：模型列表首调 30s 超时；跨 Agent 探测被 Codex 初始化阻塞
- P2：按钮 18~20dp（<48dp）；引导页"下载 42MB"文案与包内内置不符；NET-01（reverse 在场仍走局域网）
- 可复发真值：冷启 10/10 avg **700.8ms**；三 Agent 零交叉污染；TLS1.3 明文探测 EOF；包内 SenseVoice 哈希 MATCH；崩溃/ANR/数据丢失 **0**

---

## 7. 已知边界与技术债

- daemon `go vet` 有 protobuf 值拷贝警告（**存量**，无并发风险，非新问题）
- **同 provider 多会话不并行**（原生 adapter 的 turn 锁排队）→ 别宣称"并行生成"
- 上下文窗口对多数模型**真未知**：`ListModels` 里只有 `codex/default`(272k) 与 `echo/echo-1`(8k) 带窗口，
  其余 ~28 个端点模型全 `cw=0`；codex 也不发该字段，schema 里也没有 → UI 显示"—"是**如实**，不是 bug
- codex schema 里**不存在** `disable_paste_burst` / `hide_agent_reasoning` 键；**红线：绝不改写用户桌面 `~/.codex/config.toml`**
- 粘贴长文本**不中断**是本通道既有事实（stdio JSON-RPC，不经终端输入层）→ UI 写"已内置"，别改回置灰开关
- P2 **内嵌 Tailscale 未做**（靠 PC 侧 Tailscale + `adb reverse` / LAN 降级）
- P3 除语音外（GeckoView 可选模块、自更新、实况通知）**均未做**
- ZCode 仅验证 **Individual Coding Plan**；Team/Start/Off-peak、OAuth 自动刷新未实现
- 候选是 **debug APK（`debuggable=true`）** → 安全结论不可外推到 release
- codex 本机 `Failed to refresh token: 403` 时好时坏（地区限制）→ 探测态在 `applied`/`unreachable` 间跳

---

## 8. 关键坑（必读）

1. **"源码改对 ≠ 跑着的是新的"是本项目头号假故障来源**（APK 与 daemon 同坑）。报"没生效"前先自证产物：
   `apkanalyzer dex packages` 查新增类在不在包里 + **dex 里 grep 本轮新增的中文字面量**。
2. **构建缓存三件套**：① 大批 `Unresolved reference` 且全指向**没改动**的文件 ⇒ Kotlin 增量缓存失配，物理删 `app/build` + `.kotlin` 重跑，别逐条"修"；② `:app:clean` 删不掉 = 残留 Gradle daemon 占锁 → `gradlew --stop`；③ 不必依赖 gradle clean，`shutil.rmtree` 物理删 build 再 assemble 等价。
   **反例（别混）**：错误全在**你刚改的文件**、首条是某操作符 `Unresolved reference`（如 Flow `.buffer` 没 import）⇒ 单点失败拖垮类型推断，补那一行 import 即过，**不是缓存问题**。
3. **Kotlin 块注释可以嵌套**（与 Java/C 不同！）：注释正文写 `video/*` 会开新注释吞到文件尾；修正时写字面的「星号+斜杠」会**反向提前闭合**。注释里写通配一律用 `video/ 前缀` 措辞。自查用 `tools/device/_pa_scan_nested_comment.py`。
4. **StateFlow 按元素 equals 发射**：原地改对象字段永不刷 UI → 要刷的字段用 `mutableStateOf` 委托。
5. **Kotlin 函数类型形参不允许命名实参** → 参数一多就换成**带具名方法的类**。
6. **Composable 里的 `return` 会截断后续渲染**：底部弹层必须写在 `return` 之前。
7. **真机驱动别"后台启动就不管"**：脚本会活过发起它的那一轮，下一轮会看到"自己上一轮的操作"，误判成"有第三方并发驱动"。脚本应前台跑完或收尾显式 kill。
8. **小米安装**：先 uninstall 再 `adb install`（`-r` 会静默失败）；「USB安装提示」点「继续安装」。
9. **adb server 每条命令后都可能被回收**：`input tap` 会**丢事件** → 关键点击必须「dump 校验 + 自愈重试」；被滚动裁掉的节点 dump 里根本没有 → find 前先滚。
10. 杂项：`screencap` 偶发返回陈旧合成层（结构验证以 dump 为准）；`adb shell input text` 反斜杠写 4 个、中文路径不行；`f.delete()` 后 `f.length()` 恒 0；Write 工具会把源码 `\u0000` 写成真 NUL 字节。

---

## 9. 运维：日志与清理

### 9.1 日志分级（5 类）

| 类 | 内容 | 保留 |
|---|---|---|
| A 运行日志 | `daemon.log` / `watchdog.log` | 轮转，只留错误行 |
| B 验收证据 | `docs/verify/evidence/**` | **永久保留** |
| C 构建一次性 | `build/`、`*.log` | 随构建清理 |
| D 归档冷数据 | `_archives/` | 90 天后可删 |
| E 错误汇总 | 抽取的 ERROR/WARN 行 | 长期 |

**铁律**：成功日志不长期留盘；`*.log` 禁止入库；**归档只搬不删**；运行中的日志禁止直接删；
`docs/verify/evidence/**` **磁盘上不可清理**（但**不入库**：截图/录屏/UI dump 由
`docs/verify/evidence/.gitignore` 排除，仅 `*.md` 结论文档与 `*.json/*.txt` 冻结记录入库）。

### 9.2 清理工具（`tools/ops/`）

`archive_logs.py`（归档 + 错误抽取 + manifest）、`rotate_runtime_logs.py`（复制 + SHA256 校验通过才截断）、
`clean_junk.py`（可再生缓存）、`cleanup_project.py`（白名单删除 + 精确保护清单）、`cleanup_cdrive.py`（C 盘分流）。

### 9.3 归档

`_archives/` 现存三份：`_trashbackup_20260915`(445 KB)、`_trashbackup_20260918`(4 KB)、
**`_trashbackup_20260919`(154 MB)** —— 后者含 C 盘 535 项搬移、日志归档、以及
`_backup-unique/`（`_backup` 删除前抢救出的 **55 个 git 无法还原**的文件，31 MB）。

另有两份**删除台账**（逐条列出被删路径，可据此复核/找回）：
`_docs-discarded_20260920-manifest.json`（过程性文档 109 个）、
`_residual-cleanup_20260920-manifest.json`（残留 16 项 / 97 MB）。

> 已删除的 git 冗余备份 `_backup/pocketagent-pre-p0p1-20260918`（99.2 MB）：
> 其 `HEAD` 与主仓库同为 `8cc0c5e`、27 提交主仓库全有、工作树 713/810 逐字节一致、0 个"比现在新"的文件
> → 删除**零损失**（唯一 55 个例外已保留，见上）。

### 9.4 运行中，勿动

- `pocketagentd.exe`（pid 21440，端口 7246）；`daemon/daemon.log`、`watchdog.log` 停 daemon 前别删
- `paguard` 当前未运行 → daemon 掉了不会自动拉起
- `.pocketagent-uploads/`（两处）是 daemon 活数据，**不是垃圾**
- ⚠ 计划任务 `PocketAgentDaemonGuard` 状态本环境无法核实（`schtasks` 被拦）

### 9.5 空间紧张可删清单

| 项目 | 大小 | 再生方式 |
|---|---|---|
| `_archives/_trashbackup_20260919/` | 154 MB | 确认无异常后删（`_backup-unique/` 建议留一份） |
| `G:\pa\android` + `C:\pa\android` | 视缓存 | `_pa_gradle.py` 自动重建 |
| `pocketagent/build/` | ~250 MB | `_pa_gradle.py apk` |
| `android/.gradle`、`.kotlin` | ~7 MB | Gradle 自动重建 |

---

## 10. 文件/下载支持（App 内置预览）

唯一真值：`ui/component/FileKind.kt` 的 `preview` 字段（`IN_APP`/`SYSTEM`/`SAVE_ONLY`），
任务中心 / 文件浏览器 / 内置浏览器 / 下载分派全部读这一份。

- **内置可预览**：Markdown、纯文本/源码、JSON/XML、HTML、CSV/TSV、图片、`.xlsx`/`.xls`（含自研 OLE2+BIFF8 解析器 `XlsLegacy.kt`）、`.ods`、`.docx`、`.rtf`、`.odt`、`.pptx`、`.odp`、PDF（系统 `PdfRenderer` 逐页）
- **只交系统**：`.doc`/`.ppt`（OLE 二进制）、安装包、音视频、未知二进制
- **只保存不预览**：压缩包、可执行/库
- 已知边界（有意为之）：Office 只还原**内容与结构**，不还原版式；单次读取上限 64 MB；表格 800 行×40 列×3 表、PDF 前 12 页、PPTX 前 40 页

---

## 11. 真机 / daemon 工具

真机验证与 daemon 调试脚本在 `tools/device/`（50 个，含 `README.md`）：构建包装器 `_pa_gradle.py`、
小米安装 `_pa_install.py`、daemon 热部署 `_pa_rebuild_daemon.py`、共享驱动 `_pa_audit_drv.py`、
Kotlin 词法扫描 `_pa_scan_nested_comment.py`、发布审核流水线 `_pa_audit_*.py`、断言脚本 `_pa_verify_*.py`。
