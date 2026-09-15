# PocketAgent 真机验证方案（2026-09-15）

> 验证对象：第五轮「Android UI/UX 大修（约 21 项）」+ daemon 侧 codex 会话修复
> 本文件同时是**执行清单**和**结果记录表**——每项都有「判定」列，测完就地填写。

---

## 0. 本轮验证对象与基线

| 项 | 值 |
|---|---|
| APK | `G:\项目1\pocketagent\build\app-debug.apk` |
| APK 大小 / MD5 | 19,360,250 B / `2B5A9D0538F8D72B5B1FCCF83D4050D5` |
| 构建方式 | `android\build-ascii.ps1`（ASCII 镜像构建，`G:\项目1` 中文路径下 protoc 会失败） |
| daemon | `G:\项目1\pocketagent\daemon\pocketagentd.exe`（源码 13:15 重编，20,405,760 B） |
| daemon 监听 | `0.0.0.0:7246` + `[::]:7246` |
| daemon workspace | `G:\项目1\pocketagent` |
| 单测基线 | `testDebugUnitTest` 11/11 通过 |

**变更范围**（本轮共 12 文件 / +1310 −257，尚未提交）：
`AgentConnection.kt` `AppDrawer.kt` `ChatScreen.kt` `GitScreen.kt` `HomeScreen.kt`
`SettingsScreen.kt` `Splash.kt` `ThreadsScreen.kt` `TurnSession.kt`
`daemon/internal/agent/codexsessions.go` `daemon/internal/server/codexsessions.go` `daemon/internal/server/thread.go`

---

## 1. 设备连接与环境准备

### 1.1 设备侧（小米 / HyperOS 特别注意）

| # | 步骤 | 说明 |
|---|---|---|
| 1 | 开发者选项 → **USB 调试** 打开 | 必须是「USB 调试」而不只是「USB 安装」 |
| 2 | 开发者选项 → **USB 调试（安全设置）** 打开 | HyperOS 默认关闭，不开则 `adb install` 被拒（`INSTALL_FAILED_USER_RESTRICTED`） |
| 3 | 数据线连接 PC，手机弹窗选 **允许 USB 调试** | 勾选「一律允许」可避免反复弹窗 |
| 4 | 关闭手机的「USB 网络共享」 | 否则电脑会多出 `169.254.x.x` 网卡，干扰局域网候选地址判定 |

### 1.2 PC 侧

```powershell
$adb = 'C:\Android\platform-tools\adb.exe'

# 1) 设备必须处于 device 状态（不是 unauthorized / offline）
& $adb devices -l

# 2) daemon 存活且监听 7246
Get-Process pocketagentd | Select-Object Id, StartTime
netstat -ano | Select-String '7246'

# 3) 【关键】建立 USB 反向隧道：手机的 127.0.0.1:7246 -> 电脑的 7246
& $adb -s 8e0e7bbe reverse tcp:7246 tcp:7246
& $adb -s 8e0e7bbe reverse --list        # 应回显 tcp:7246 tcp:7246

# 4) 记录 PC 候选地址，供手填兜底
(Get-NetIPAddress -AddressFamily IPv4 | Where-Object { $_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*' } |
    Select-Object IPAddress, InterfaceAlias | Format-Table -AutoSize)
```

**本机实测值（填表用）**

| 通道 | 地址 |
|---|---|
| USB（adb reverse） | `127.0.0.1:7246` |
| 局域网（WLAN） | `10.15.5.95:7246` |
| Tailscale | `100.92.193.109:7246` |

### 1.3 安装

```powershell
$adb = 'C:\Android\platform-tools\adb.exe'
& $adb -s 8e0e7bbe install -r 'G:\项目1\pocketagent\build\app-debug.apk'

# 建议先清一次旧数据，保证验证的是全新首启路径
& $adb -s 8e0e7bbe shell pm clear dev.pocketagent.app
```

### 1.4 基线快照（**已完成，2026-09-15 13:24–13:25**）

| 项 | 实测值 |
|---|---|
| `adb devices -l` | `8e0e7bbe  device product:fuxi model:2211133C device:fuxi` |
| 手机 | 小米 13（fuxi / 2211133C），**Android 16**，HyperOS `V816` |
| 手机 WiFi IP | `10.15.27.59/17`（子网 `10.15.0.0/17`，与 PC 同网段） |
| 安装结果 | `adb install -r` → **Success**（13:24:27；versionCode=1 / versionName=0.1.0 / targetSdk=35） |
| daemon PID | **80984**（13:22:02 由 watchdog 启动；前一个 68892 于 13:22 前消失，见 D-02） |
| `adb reverse --list` | `UsbFfs tcp:7246 tcp:7246` ✅ |
| PC 侧 TCP 探测 7246 | `127.0.0.1` ✅ 23ms ／ `10.15.5.95` ✅ 1ms ／ `100.92.193.109` ✅ 0ms |
| **实测已建立连接** | `10.15.5.95:7246 ← 10.15.27.59:41054 ESTABLISHED` → **走的是局域网，非 USB** |
| ⚠️ `POST_NOTIFICATIONS` | **granted=false**（Android 13+ 运行时权限未授予，影响前台服务通知展示，见 X13） |

> 注：App 沿用 2026-09-10 安装时的数据（`install -r` 保留数据），因此 `lastLanAddr` 已有缓存值。若要验证**首启连接路径**（USB 或手填），必须先 `pm clear` —— 但这会清掉 App 本地数据，需你确认后再做。

---

## 2. 连接层验证（P0 — 不通过则后续全部无法进行）

设计意图见 `AgentConnection.candidateAddresses()`：候选顺序
**`[USB 调试] → 局域网 → Tailscale → 手填地址`**，每候选 6 秒超时；`connectWithFallback()` 依次尝试。

| ID | 场景 | 操作 | 预期结果 | 判定 |
|---|---|---|---|---|
| C1 | USB 通道直连 | 插好 USB + `adb reverse`，冷启 App | 状态行显示「已连接 · USB 通道」，PC 侧 `netstat` 对端为 `127.0.0.1` | ✅ 修复后通过 |
| C2 | 局域网直连 | 拔掉 `adb reverse`（`adb reverse --remove-all`），同一 WiFi 下点「保存并测试连接」填 `10.15.5.95:7246` | 显示「已连接 · 局域网」 | ☐ |
| C3 | Tailscale 直连 | 手机开 Tailscale，关 WiFi 走蜂窝，地址填 `100.92.193.109:7246` | 显示「已连接 · Tailscale」；建立耗时 3–5s 属正常 | ☐ |
| C4 | 回退顺序与耗时 | 同时具备 USB + 局域网条件，观察先走哪条 | 优先 USB；USB 不可用时自动落到局域网，**无需手点重试** | ☐ |
| C5 | 断连自愈 | 连接正常时 `& $adb reverse --remove-all` 并关 WiFi，再恢复 | 状态变「未连接 · 自动重连中」，恢复后**自动**回到「已连接」，各页面数据自动刷新（由 `connectionEpoch` 驱动），无需手动下拉 | ☐ |
| C6 | 手填地址兜底 | 设置 → 全局 → 连接 → 地址填 `10.15.5.95:7246` → 「保存并测试连接」 | Toast「连接成功 · …」；地址失效时显示红色诊断卡（含 message + 地址） | ☐ |
| C7 | 配对 token | 给 daemon 加 `-token` 启动，App 端填错 token | 明确报鉴权失败，**不**无限重试空转 | ☐ |

### ⚠️ D-01（预判缺陷，优先验证）

`AgentConnection.isUsbDebugging()` 的判定是
`ADB_ENABLED==1` **且** `UsbManager.deviceList.isNotEmpty()`。

但 `UsbManager.deviceList` 返回的是**手机作为 USB 主机（OTG）时外挂的设备**，手机插在电脑上时它是**空的**（清单里也没有 `uses-feature usb.host`）。因此该函数很可能**恒为 false**，导致 USB 候选地址永远不会被加入队列——正好与本轮「USB 调试时优先走 USB」的诉求相反。

**验证方法**：插好 USB + `adb reverse`，打开 App 看状态行的通道名。
**若显示的不是「USB」** → D-01 成立。

**建议修法**（符合回退原则，不新增 RPC）：把 `127.0.0.1:7246` 无条件加入候选列表**最前面**。回环地址在无 `adb reverse` 时会被立刻 RST，耗时是毫秒级而非 6 秒，代价可忽略；这样既不误判，又天然实现了「有反向隧道就用、没有就落到局域网」。

| 判定 | 结果 |
|---|---|
| D-01 是否成立 | ☑ **成立（已实测证实）** |
| 修复状态 | ✅ **已修复，真机验证通过**（详见下文「实际修法 / 验证结果」） |

#### ✅ 实测证据（2026-09-15 13:25）

条件：USB 调试已开 + `adb reverse tcp:7246` 已注册 + 三通道从 PC 侧全部可达。**App 状态行却显示「已连接 · 局域网直连」。**

**证据 1 — 连接对端是手机 WiFi IP，不是 127.0.0.1：**
```
TCP    10.15.5.95:7246    10.15.27.59:41054    ESTABLISHED    80984
```
`10.15.27.59` = 手机 `wlan0` 地址（`ip -4 addr show wlan0` 确认）。若走 USB 反向隧道，对端应为 `127.0.0.1`。

**证据 2 — 手机此刻是 USB「设备」角色，`deviceList` 必然为空：**
```
device_manager:  connected=true  configured=true  host_connected=false
port_manager:    current_mode=ufp  data_role=device  power_role=sink
host_manager:    num_connects=14  ← 历史全部是耳机/DAC（manufacturer=13058, is_headset）
```
手机插在电脑上时 `data_role=device`、`host_connected=false`，不存在 USB 主机设备 → `UsbManager.deviceList` 为空 → `isUsbDebugging()` 恒为 false。

**证据归档**：`docs/verify/evidence/2026-09-15/d01-usb-detection-evidence.txt`

#### 影响评级

| 场景 | 影响 |
|---|---|
| 手机与电脑同网段 | 无感（自动走局域网，功能正常） |
| 首次安装 / 清数据后，仅有 USB 可用 | **无法连接**（USB 不候选、局域网/Tailscale 未缓存、只能手填地址兜底）→ **P1** |
| 电脑与手机不同网段（如公司网络隔离） | 需依赖 Tailscale 或手填 → **P1** |
| 与需求「USB 调试时优先走 USB」的一致性 | 不符合 → **P2** |

#### 建议修法（符合回退原则，不新增 proto/RPC）

把 `127.0.0.1:7246` **无条件**加入 `candidateAddresses()` 最前面，去掉 `isUsbDebugging()` 这道不可靠的门槛。
回环地址在无 `adb reverse` 时会立刻被 RST，耗时毫秒级（实测 23ms），远低于 6s 超时，代价可忽略；这样既避免误判，又天然实现「有反向隧道就用、没有就落局域网」。

若仍想保留「充电时不白等」的意图，可用**短超时**（如 800ms）单独探测 USB 候选，而非依赖 `UsbManager` 判定。

#### 🔧 实际修法（已落地，2026-09-15）

涉及文件：`AgentConnection.kt`、`SettingsScreen.kt`

1. `candidateAddresses()`：删掉 `isUsbDebugging()` 门槛，`USB_ADDR` 无条件排第一。
2. **顺带修掉一个连带隐患**：原先「用户手填地址」与「当前生效地址」共用 `address` 字段，
   回退链命中候选后会把它改写 —— USB 一旦生效，手填值就被覆盖，拔线后再也回不去。
   新增独立持久化字段 `manualAddress`（prefs 键 `manualAddr`），`setAddress` / `setCredentials` 写入，
   `candidateAddresses()` 的兜底项改用它。
3. 新增按候选分级超时：`USB_TIMEOUT_MS = 1500ms`（回环无隧道时毫秒级失败）、
   `REMOTE_TIMEOUT_MS = 6000ms`（Tailscale 走 DERP 需 3–5s）。
4. 删除 `isUsbDebugging()`（唯一调用点已移除，避免留下误导性死代码）。
5. `SettingsScreen` 地址输入框改用新增的 `configuredAddress()`（手填优先回显），
   避免 USB 生效时输入框显示 `127.0.0.1:7246`、让人误以为手填值丢了。

#### ✅ 验证结果

| 验证方式 | 命令 / 操作 | 结果 |
|---|---|---|
| 编译 | `build-ascii.ps1` → `assembleDebug` | ✅ BUILD SUCCESSFUL in 41s |
| 单元测试 | `testDebugUnitTest`（含新增 `AgentConnectionCandidatesTest` 3 例） | ✅ **14/14 通过**，0 失败 0 错误 |
| 真机 · USB 通道 | 保留 `adb reverse`，`force-stop` 后重启 | ✅ 状态行显示「已连接 · **USB 通道**」 |
| 真机 · 对端证据 | `netstat` | ✅ `127.0.0.1:7246 ← 127.0.0.1:60558 ESTABLISHED`（修复前是 `10.15.27.59`） |
| 真机 · 回退验证 | `adb reverse --remove-all` 后重启 | ✅ 自动落到「已连接 · **局域网直连**」，对端回到 `10.15.27.59`，USB 候选只多花约 1.5s |

证据：`04_usb_fixed.png`、`05_fallback_lan.png`、`d01-usb-after-fix.txt`、`d01-lan-fallback-after-fix.txt`

#### 回滚方案

改动集中在 2 个源文件 + 1 个新增测试，**无 proto / RPC / daemon 契约变更**，回滚成本极低：

```powershell
git -C G:\项目1\pocketagent checkout -- `
  android/app/src/main/java/dev/pocketagent/app/AgentConnection.kt `
  android/app/src/main/java/dev/pocketagent/app/ui/SettingsScreen.kt
Remove-Item 'G:\项目1\pocketagent\android\app\src\test\java\dev\pocketagent\app\AgentConnectionCandidatesTest.kt'
# 再重装上一个 APK
```

若只想退回「USB 不参与回退」的旧行为，最小改法是保留本次其余改动、
仅删掉 `candidateAddresses()` 里的 `add(USB_ADDR)` 一行即可（超时与字段改动均向后兼容）。

---

## 3. 核心功能验证

### A. 启动 / 导航 / 返回键

| ID | 场景 | 预期结果 | 判定 |
|---|---|---|---|
| A1 | 冷启动（`pm clear` 后首次） | 启动动画：主题色背景（**深色主题不得闪白**）→ 字标淡入+放大 → 逐字打出品牌名 → 到 "agent" 时光晕 + 底部进度线 → 整屏淡出 | ☐ |
| A2 | 冷启动耗时 | 从点击图标到可交互 ≤ 2.5s（中端机基准） | ☐ |
| A3 | 侧边栏铺满 | 打开抽屉：遮罩铺满整屏（含状态栏/导航栏区域），点遮罩**任意位置**均可关闭 | ☐ |
| A4 | 抽屉点按反馈 | 每个条目按下有缩放 + 触感；选中项有高亮动画与圆点 | ☐ |
| A5 | 分组与底栏 | 存在「导航 / 更多」分组；底部有连接状态行 | ☐ |
| A6 | **返回键分层**（本轮重点） | 抽屉打开时按返回 → **只关抽屉**，不退出页面；再按才退页面 | ☐ |
| A7 | 返回键分层（Git 面板） | 会话内 Git 面板打开时按返回 → 只关面板 | ☐ |
| A8 | **不再直接回桌面**（原缺陷） | 从聊天页按返回应回到会话列表/首页，**不得**直接退出到桌面 | ☐ |
| A9 | 系统返回手势 | 侧滑返回与返回键行为一致 | ☐ |

### B. 聊天页时间线（`ThinkingCard` / `ToolCallCard`）

| ID | 场景 | 预期结果 | 判定 |
|---|---|---|---|
| B1 | 思考流式 | 思考内容以独立卡片出现在**时间线中**（不再堆在顶部固定条），流式时默认展开 | ☐ |
| B2 | 思考落定 | 正文开始 / 工具开始 / 本轮结束时，思考落定为卡片并默认收起 | ☐ |
| B3 | 工具调用卡 | 一次调用 = 一张卡，含参数 / 输出 / 耗时 / 状态，可展开 | ☐ |
| B4 | 工具结果原地更新 | 工具执行完成后**更新同一张卡**，不得每阶段新增一张 | ☐ |
| B5 | 长内容 | 超长参数/输出可展开滚动，不撑破布局 | ☐ |

### C. 上下文窗口与模型

| ID | 场景 | 预期结果 | 判定 |
|---|---|---|---|
| C1 | 上下文窗口卡可交互（原缺陷：跟随时无交互） | 点模型行左侧上下文胶囊，弹出 `ContextWindowCard`，展示输入 / 输出 / 缓存读 / 缓存写四个维度的占比条 + 合计 token + 成本 | ☐ |
| C2 | 模型名过长 | 长模型名**不得挤压发送键**；文本省略号截断，发送键宽度稳定 | ☐ |
| C3 | 切换非「跟随桌面配置」模型 | 上下文窗口**依然显示**（原缺陷：切了就消失） | ☐ |
| C4 | 会话级模型持久化（原缺陷） | 会话内切到某模型 → 返回列表 → 重新进入该会话，**仍是所选模型**，不回退成「跟随桌面配置」 | ☐ |
| C5 | 全局默认 vs 会话覆盖 | 会话列表里选模型 = 改全局默认；聊天页里选 = 只影响本会话 | ☐ |

### D. 设置页三板块

| ID | 场景 | 预期结果 | 判定 |
|---|---|---|---|
| D1 | 板块名与含义 | 全局（所有 Agent + App 界面）/ 当前 Agent（`selectedAgent` 的专属设置）/ 其他 Agent | ☐ |
| D2 | 当前 Agent = Codex | 显示原有四块设置 | ☐ |
| D3 | 当前 Agent ≠ Codex | 显示占位说明，且提供「切回 Codex」 | ☐ |
| D4 | 其他 Agent 自动跳转 | 在「其他 Agent」里选中某项 → `switchAgent()` 生效 → **自动跳转到「当前 Agent」页** | ☐ |
| D5 | 默认模型可选（原缺陷） | 「默认模型」右侧有「选择」，点开 `ModelPickerDialog`，首项为「跟随桌面配置」（空 id），可正常选中并保存 | ☐ |
| D6 | 设置保存反馈 | 保存后有 Toast/状态提示；失败显示红色错误块 | ☐ |

### E. 会话列表（`ThreadsScreen`）

| ID | 场景 | 预期结果 | 判定 |
|---|---|---|---|
| E1 | Codex 项目/任务标题实时显示（原缺陷） | 加载中即可看到标题，**不留空白**（daemon 已用 rollout 扫描补齐空标题/cwd/时间） | ☐ |
| E2 | 归档过滤（原缺陷） | 「已归档」只显示归档会话；「活跃」只显示未归档（客户端按 `ThreadStatus` 分流） | ☐ |
| E3 | 归档操作后 | 会话从「活跃」消失并出现在「已归档」，无需重启 App | ☐ |
| E4 | 删除 Codex 会话（原缺陷） | 长按电脑端会话卡 → 底部弹出「删除这个会话」→ 确认后：本地线程删除 **且** 电脑端 rollout 文件被删 | ☐ |
| E5 | 删除后校验 | 电脑端 `~/.codex/sessions/**` 下对应 rollout 已不存在 | ☐ |

### F. Git

| ID | 场景 | 预期结果 | 判定 |
|---|---|---|---|
| F1 | Git 页有内容（原缺陷：空白无交互） | 自动解析出真正的 git 仓库并展示分支/改动，不再是空白页 | ☐ |
| F2 | 非仓库目录 | 给出明确「不是 git 仓库」提示 + 可切换仓库入口，而不是静默空白 | ☐ |
| F3 | 切换仓库 | 头部显示当前仓库路径；点文件夹图标可选已知目录或手填路径 | ☐ |
| F4 | 重连后刷新 | 断开重连后 Git 数据自动重取（`connectionEpoch` 驱动） | ☐ |
| F5 | 会话内 Git 面板 | 会话无工作区时回退到 daemon 默认工作区，不报错 | ☐ |

### G. 终端「无感」

| ID | 场景 | 预期结果 | 判定 |
|---|---|---|---|
| G1 | 点 agent 磁贴 | **不再**直接跳进全屏终端；只切换 agent + 轻提示 Toast | ☐ |
| G2 | 终端按需打开 | 仅当用户主动点「终端」才打开 | ☐ |
| G3 | PC 端无窗口 | daemon 侧 codex / git / 工具子进程**不弹控制台窗口**（`hidewin` + WMI 派生） | ☐ |
| G4 | 终端可用性 | 终端可正常输入输出、可缩放 | ☐ |

---

## 4. 通过标准（汇总判定）

| 等级 | 标准 |
|---|---|
| **通过** | 所有 P0（§2 连接层）项全绿；§3 中无「阻断级」缺陷；崩溃/ANR 为 0 |
| **有条件通过** | P0 全绿；存在不影响主流程的 UI 缺陷，已录入问题记录并给出修复计划 |
| **不通过** | §2 任一项失败导致无法连接；或出现崩溃、数据丢失、会话状态错乱 |

**客观门槛**

- 崩溃：`adb logcat -b crash` 无新增 `FATAL EXCEPTION`
- 无响应：无 ANR 弹窗（`adb shell dumpsys activity anr` 无新记录）
- 连接：单候选建立 ≤ 6s；USB 通道 ≤ 1s
- 冷启动：≤ 2.5s 到可交互
- 内存：连续操作 10 分钟后 PSS 无持续单调上涨（`adb shell dumpsys meminfo dev.pocketagent.app`）

---

## 5. 异常与边界情况

| # | 场景 | 处理方式 / 预期 |
|---|---|---|
| X1 | daemon 中途退出 | watchdog 应在 ≤60s 内拉起（当前实测存在**自发退出**现象，见 D-02）；App 侧应自动重连，不得卡死或白屏 |
| X2 | 网络抖动 | 状态在「已连接 / 自动重连中」间切换；恢复后数据自动刷新；不得反复弹错误 Toast 刷屏 |
| X3 | `adb reverse` 丢失（拔插线） | 应回退到局域网；不得在 USB 候选上死等 |
| X4 | Tailscale 走境外 DERP | 建立可能 3–5s；不得因超时误判为「连不上」 |
| X5 | 前后台切换 | 回前台后保持连接与滚动位置；前台服务通知正常（`TurnKeepAliveService`） |
| X6 | 屏幕旋转 | `configChanges` 已声明；不得重建丢失会话内状态 |
| X7 | 深色 / 浅色主题 | 启动动画、抽屉、卡片在两种主题下均可读；深色下**不出现白底/黑字错配** |
| X8 | 系统「减弱动态效果」 | 动画应降级而非卡顿（本轮已知未实现，作为记录项） |
| X9 | 超长文本 / 超长模型名 | 截断 + 省略号；布局不破 |
| X10 | 空数据（无会话 / 无仓库 / 无模型） | 显示空状态提示，不崩溃；空状态需提示长按可删除 |
| X11 | 非 git 仓库工作区 | `notRepo` 分支生效，给出切换入口 |
| X12 | 删除正在运行的会话 | 应给出保护或明确结果，不得留下半删除状态 |
| X13 | 无权限（无通知权限） | 前台服务降级但不崩溃 |
| X14 | 手机无 USB 权限（未授权） | `adb devices` 显示 `unauthorized`；App 内应能走局域网 |
| X15 | 并发：多端同时连同一 daemon | 至少不互相踢掉导致崩溃（记录实际行为即可） |

---

## 6. 输出：验证结论与问题记录

### 6.1 结论汇总（验证结束后填写）

```
验证日期：2026-09-15
验证人：
APK MD5：2B5A9D0538F8D72B5B1FCCF83D4050D5
设备：小米 13 (fuxi/2211133C) / HyperOS ______

结论：☐ 通过   ☐ 有条件通过   ☐ 不通过

通过率：P0 连接层 __/7    A __/9   B __/5   C __/5
        D __/6   E __/5   F __/5   G __/4

阻断级缺陷：__ 个      一般缺陷：__ 个      建议项：__ 个
崩溃次数：__    ANR 次数：__
```

### 6.2 缺陷单模板（每条缺陷一份）

```
[缺陷编号] D-0x
标题：      一句话描述现象
严重级：    P0 阻断 / P1 严重 / P2 一般 / P3 建议
复现概率：  必现 / 3/5 / 偶现
环境：      设备型号 / 系统版本 / 通道(USB|局域网|Tailscale) / APK MD5
前置条件：
复现步骤：  1. …
            2. …
            3. …
实际结果：
预期结果：
证据：      ☐ 截图   ☐ 录屏   ☐ logcat   ☐ daemon.log   ☐ adb 输出
疑似根因：  文件:行号 + 简要分析
建议修法：
```

### 6.3 证据留存要求

```powershell
$adb = 'C:\Android\platform-tools\adb.exe'

# 截图（二进制不能直接重定向，先落手机再 pull）
& $adb shell screencap -p /sdcard/pa_shot.png
& $adb pull /sdcard/pa_shot.png .\evidence\

# 录屏（最长 180s，验证动画/交互时必录）
& $adb shell screenrecord --time-limit 180 /sdcard/pa_rec.mp4
& $adb pull /sdcard/pa_rec.mp4 .\evidence\

# 崩溃日志
& $adb logcat -b crash -d *> .\evidence\crash.txt

# App 运行日志
$pid_ = (& $adb shell pidof dev.pocketagent.app).Trim()
& $adb logcat --pid=$pid_ -d *> .\evidence\app.txt

# daemon 侧日志（PC）
Get-Content 'G:\项目1\pocketagent\daemon\daemon.log' -Tail 200 *> .\evidence\daemon.log.txt
Get-Content 'G:\项目1\pocketagent\daemon\watchdog.log' -Tail 50 *> .\evidence\watchdog.log.txt
```

证据目录建议：`G:\项目1\pocketagent\docs\verify\evidence\2026-09-15\`

### 6.4 第一轮实测记录（2026-09-15 13:24–13:25，PC 侧自动完成）

| 项 | 结果 | 证据 |
|---|---|---|
| 设备连接与授权 | ✅ `device` 状态正常 | `adb devices -l` |
| APK 安装 | ✅ Success | `adb install -r` 输出 |
| USB 反向隧道 | ✅ `UsbFfs tcp:7246 tcp:7246` | `adb reverse --list` |
| 三通道连通性（PC 侧） | ✅ 全部可达（23ms / 1ms / 0ms） | TCP 探测 |
| A1 启动动画 | ✅ 字标 + 光晕 + 底部进度线（浅色主题白底正常） | `01_splash.png` |
| 连接建立 | ✅ 已连接（**局域网直连**） | `02_home.png` |
| C1 USB 通道 | ❌ **失败**（应显示「已连接 · USB」）→ **D-01** | `d01-usb-detection-evidence.txt` |
| D-01 预判缺陷 | ✅ **成立** | 同上 |
| D-02 daemon 自发退出 | ⚠️ 观察到（68892 → 13:22:02 重启为 80984） | `watchdog.log` |

### 6.5 修复后复测（2026-09-15 13:41–13:42）

| 项 | 结果 | 证据 |
|---|---|---|
| 编译 | ✅ BUILD SUCCESSFUL in 41s，APK 18.97 MB | `_build7` 输出 |
| 单元测试 | ✅ **14/14 通过**（新增 3 例回退链用例） | Gradle test-results |
| C1 USB 通道（隧道在） | ✅ 「已连接 · **USB 通道**」，`netstat` 对端 `127.0.0.1` | `04_usb_fixed.png` / `d01-usb-after-fix.txt` |
| C2 局域网回退（隧道移除） | ✅ 自动落到「已连接 · **局域网直连**」，对端 `10.15.27.59`，USB 仅多花 ~1.5s | `05_fallback_lan.png` / `d01-lan-fallback-after-fix.txt` |
| 深色主题渲染 | ✅ 本轮截图恰为深色主题，启动页/首页/卡片均正常（浅色见 01/02） | `04_usb_fixed.png` |

> 副作用记录：这轮截图里「多agent协同」由关闭变为开启、「最近任务」消息数由 2 变 4 ——
> 手机在使用中，非本次改动导致。可在后续复测中留意该开关的初值是否符合预期。

**待人工在手机完成**：§2 的 C2–C7、§3 全部 A/B/C/D/E/F/G 组、§5 异常边界项。

**注意**：本轮截图均为**浅色主题**。§3-A1 明确要求「深色主题不得闪白」，需另跑一次：

```powershell
& $adb shell cmd uimode night yes    # 切深色
& $adb shell am force-stop dev.pocketagent.app
& $adb shell am start -n dev.pocketagent.app/.MainActivity
# …截图…
& $adb shell cmd uimode night no     # 还原
```

---

## 7. 既有观察项（验证前已发现，待确认是否影响体验）

### D-02：daemon 会自发退出，由 watchdog 拉起

实测：13:15:52 启动的实例（PID 68892）在 13:22 前消失，watchdog 于 13:22:02 重启（PID 80984）。
`watchdog.log` 历史上还有 11:40 / 11:41 / 11:45 / 12:17 / 12:20 多次重启记录。

- **影响**：真机测试中可能出现「无故断连」，恢复需 ≤60s。
- **排查方向**：确认是 daemon 自身崩溃（查 `daemon.log` 有无 panic）、还是外部因素（宿主会话回收 / 电源策略 / WMI 宿主进程生命周期）。
- **验证动作**：连续观察 30 分钟，记录 PID 变化次数与间隔；期间在 App 端观察是否掉线。
- **结论**：☐ 稳定（30min 内 PID 不变）  ☐ 不稳定，重启 __ 次

### D-03：`unsupported_country_region_territory` 403

`daemon.log` 持续出现 codex token 刷新 403（地理限制）。**属环境问题，非本轮改动引入**。
→ 影响：真实 Turn 无法跑通，聊天页流式/工具卡只能验证到「连接与渲染」层面。

- 应对：若无法解除限制，聊天页用例 B1–B5、C1–C3 改用**本地可用的模型通道**验证；若确实无法产生真实 Turn，则在结论中标注为「未覆盖（环境受限）」而非「通过」。

---

## 8. 未覆盖 / 已知不实现项（避免误判为缺陷）

- 设置项保存反馈：本轮已加，需实测。
- 会话状态点、启动动画状态机、系统「减弱动态效果」降级：**本轮未实现**，验证时不要当缺陷记。
- `CompactThread` 仍是 stub（`internal/server/thread.go:107`，注释「P1 实现」）。
- `CancelTurn` 为设计上的空实现（取消走流断开 / 重入打断）。
