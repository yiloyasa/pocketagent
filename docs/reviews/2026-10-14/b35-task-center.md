# B3 第 3 波最后一件：TaskCenter 页（任务/未读中心）

> 日期：2026-10-15 · 波次：B3 第 3 波（TaskCenter）
> 范围：`flutter/lib/pages/task_center.dart`、`flutter/lib/pages/task_center_data.dart`（新增）、
> `flutter/test/pages/task_center_data_test.dart`、`flutter/test/pages/task_center_page_test.dart`（新增）。
> **`flutter/lib/transport/pa_bridge.dart` 一字未改**（数据源判定见下：无 RPC 可接）。
> 验收：全量 `flutter test` **341 全过**（本任务净增 **77** 条：51 + 26）；
> junction 下 `flutter analyze` **零问题**。**未 git add / commit / push**。

---

## 一、一句话结论

**TaskCenter 在 daemon 侧没有任何 RPC**——proto/v1 全 93 RPC 逐个核对（host/thread/fs/git/
model/runtime/terminal 六个 proto 全文 grep），唯一的 "Task" 字样是 thread.proto 的
`DelegateSubTask`（协同子任务派发，与任务中心无关）；page-contracts §1.7 也明说本页
「**零直连 RPC**」，Kotlin 侧是 app 级 object + SharedPreferences 的**手机本地聚合**。
→ 本页落**纯本地聚合骨架**：`TaskCenterStore`（内存 + 变更通知 Stream + 持久化委托）
+ 页面（两页签 / 四词分区 / 未读徽标 / 本地动作）。**不接 PaBridge、不显示任何远端数据、
不造假加载延迟**；`pa_bridge.dart` 未追加任何方法面（没有 `listDevServers` 可加的理由：
那是 Browser 页的 dev server 面板，§1.9，与本页无关）。

---

## 二、数据源判定（本任务的第一问，先答）

| 核查 | 结果 |
|---|---|
| proto/v1 全 93 RPC 里 task / notification / unread 相关 | **0 条**（`DelegateSubTask` 属协同派发，见 thread.proto:132） |
| page-contracts §1.7「RPC 依赖」 | 「**本页零直连 RPC**」；数据 = `TaskCenter.tasks`（本地 SharedPreferences，杀进程不丢） |
| §3.4 跨页状态枢纽 | `TaskCenter`（component/TaskCenter.kt:22）= `tasks` + `unreadCount`（StateFlow）+ SharedPreferences 持久化（300ms 节流） |
| Kotlin 写入方 | 消息执行进度 ← `TurnSession`；文件下载进度 ← `PaDownloader`；两者都在手机本地 |
| 重试的真实依赖 | 消息类 → `TurnSession.resyncFromDaemon`（B5 Chat 波次）；下载类 → `PaDownloader.downloadFromUrl/downloadFromPc`（B4 宿主桥 + 下载服务） |

**结论**：本地聚合，无真通道。落本地单例骨架，诚实标注 B4/B5 回填点。

---

## 三、TaskCenter 契约（Dart 形态 = Kotlin `object TaskCenter` 的可注入化）

### 3.1 `task_center_data.dart`（新增，~880 行）

| 层 | 内容 | Kotlin 同源 |
|---|---|---|
| 模型 | `TaskItem`（不可变 + `copyWith`，13 字段） | `TaskCenter.Task`（mutableStateOf 逐字段可变） |
| 类别 | `TaskKind` {message, download} + wire 名 | `Kind`（MESSAGE / DOWNLOAD，页签 = 类别） |
| 状态 | `TaskStatus` {running, done, failed, canceled, **interrupted**} | `Status` 五档，一个不少 |
| id 约定 | `messageTaskId('msg:$threadId')` / `downloadTaskId('dl:$fileKey')` | Kotlin 同款字符串前缀 |
| 持久化抽象 | `TaskCenterPrefs`（read/write）+ `InMemoryTaskCenterPrefs` | Kotlin `SharedPreferences("task_center")`；**不加 shared_preferences 依赖**（红线），照 onboarding_prefs.dart 接口先行先例，注释标「B4 后续接持久化」 |
| 磁盘格式 | `encodeTaskBlob` / `decodeTaskBlob`：13 字段 `\u0001` 分隔、`\n` 分行 | Kotlin `writeDisk` / `parseTasks` 逐字段同构（双端将来读同一个偏好文件） |
| 纯函数 | `unreadCountOf` / `taskStatsOf` / `taskPartitionRows` / `taskStatusKindOf` / `taskStatusLabel` / `taskRowTimeLabel` / `taskOfflineHint` / `redownloadPlanOf` / `redownloadNeedsPc` / `taskNextStepCopy` | `taskCenterStats` / `taskPartitionRows` / `editorialStatusOf(Status)` / `SimpleDateFormat("MM-dd HH:mm")` / `taskOfflineHint` / `redownloadPlanOf` / `failureNextStep` |
| 门闩 | `TaskRetryGate`（`tryBegin` / `inFlight`，cooldown 1500ms） | `RetryGate` + `TASK_RETRY_COOLDOWN_MS` |
| store | `TaskCenterStore`：trackMessage / finishMessage / trackDownload / updateDownload / finishDownload / annotateDownload / attachLocalUriIfEmpty / markRead / markAllRead / remove / removeByThread / clearFinished + `tasks` Stream + `snapshot` + `unreadCount` + `setViewing` | Kotlin object 同名方法面，逐条对齐 |

### 3.2 与 Kotlin 的**三处有意分歧**（都在代码注释里写明理由）

| # | 分歧 | 理由 |
|---|---|---|
| 1 | **已读策略**：Kotlin 停留页面 500ms 后 `markAllRead()` + `setViewing(true)`（停留期间新任务直接落已读）；Flutter 改为**显式动作**（顶部未读徽标点按 = 全部已读；点单行 = 该条已读） | 本波 store 没有 app 级共享消费者（AppDrawer 徽章 / Home 未接线），自动已读会把「未读」信号在无人看时悄悄清掉。store 的 `setViewing` 分支**已实现**（测试覆盖），B4 接线徽标消费方后可还原 Kotlin 全自动语义 |
| 2 | **indeterminate 进度条**：Kotlin RUNNING 且 progress<=0 是无限扫动动画；Flutter 改静态 40% 填充 | widget 测试的 `pumpAndSettle` 会被无限动画卡死；状态由 chip 承担，进度条只是密度提示，信息不丢 |
| 3 | **`taskNextStepCopy` 多一个 `hasRetryAction` 参数** | Kotlin 行尾必有重试按钮（TurnSession / PaDownloader 都在同一个 app 里）；Flutter 本波这两个依赖都未就绪，行尾按钮只在调用方注入回调时才出现。**没有按钮时不许写「点右侧重试」**——那是指向一个不存在的按钮（假出口） |

### 3.3 页面出口（可空回调 = 无位不显示假入口）

| 回调 | 注入时的行为 | 不注入时 |
|---|---|---|
| `onOpenThread(threadId)` | 点消息行 → 回那个会话（`chat/{id}`） | 点消息行只标已读，不跳转 |
| `onOpenUserData()` | 点下载行 → 用户数据管理（`settings?sub=downloads`） | 同上 |
| `onRetryMessage(threadId)` | 失败/中断消息行出行尾「重试」 | **不渲染按钮**，文案改「去会话里找回这次的结果」 |
| `onRetryDownload(task)` | 失败下载行（有可重放来源）出行尾「重新下载」 | **不渲染按钮**，文案改「再取一次 / 恢复连接后再试」 |
| `hostOffline`（bool 入参） | 断连：消息类运行中徽章隐去 + 「离线 · 恢复连接后继续」；落「离线待恢复」分区；消息类重试入口关闭 | 在线口径 |

---

## 四、页面行为（`task_center.dart`，~730 行）

- **无加载态**：本地 store 首帧即快照（`initState` 里 `_store.snapshot` 同步读）。
  **不造假加载延迟**——给一个瞬间完成的转圈等于骗一次动画（测试用
  `find.byType(CircularProgressIndicator) → findsNothing` 钉死）。
- **顶部四卡**：等待你处理（FAILED+INTERRUPTED，CANCELED 不算）/ 正在执行 / 已完成 /
  未读徽标（点按 = 全部已读）。四卡等高（`IntrinsicHeight` + stretch——ListView 给
  子节点 Infinite height 约束，stretch 会把无限高直接灌给卡片，RenderConstrainedBox
  断言炸；这是实现期实测抓到并修掉的）。
- **两页签**：消息任务 / 文件下载（Kotlin `kindTab`；默认停消息页签）。
- **列表按四词分区**（空组不出现，组序 = 优先级）：正在执行 / 已完成 / 未完成；
  断连时消息类运行中改落「离线待恢复」。分区头 = 状态词本身，与行内徽章同源。
- **单行**：图标块 + 标题 + 「N 轮」（rounds>1）+ 时间（MM-dd HH:mm）+ 状态徽章 +
  离线提示 / 失败下一步文案 + 来源会话 / detail / 大小 + 静态进度条 + 行尾动作
  （重试中占位 > 重试/重下 > 移除，每个按钮带 `task-row-<id>-{retry,remove}` 稳定键）。
- **底部「清理已完成的任务」**：只在有终局任务时出现（Kotlin `hasFinished` 同款）。
- **行点击**：先 `markRead` 再按类别分派出口（Kotlin 行 onClick 同款顺序）。

---

## 五、测试原文

### 5.1 新增/改动文件与量级

| 文件 | 性质 | 用例数 |
|---|---|---|
| `flutter/test/pages/task_center_data_test.dart` | 新增（store 行为 + 磁盘往返 + 纯函数 + 门闩） | **51**（10 组） |
| `flutter/test/pages/task_center_page_test.dart` | 新增（页面渲染 + 交互 + 离线 + 分区） | **26**（7 组） |
| `flutter/lib/pages/task_center_data.dart` | 新增（模型 + store + 持久化 + 纯函数） | — |
| `flutter/lib/pages/task_center.dart` | 新增（页面） | — |
| `flutter/lib/transport/pa_bridge.dart` | **未改** | — |

### 5.2 数据层（51 过）原文

```
00:00 +0: loading G:/pa_b35/test/pages/task_center_data_test.dart
00:00 +0: TaskCenterStore：消息任务生命周期 trackMessage → RUNNING + 未读 1；finishMessage(ok) → DONE + 仍算未读
00:00 +1: TaskCenterStore：消息任务生命周期 finishMessage(ok=false) → FAILED，进度不冒充满
00:00 +2: TaskCenterStore：消息任务生命周期 同一会话连发：RUNNING 中合并轮次（一条记录、轮次+1、重新计未读）
00:00 +3: TaskCenterStore：消息任务生命周期 同会话已终局后再 trackMessage：换新记录（旧终局被替换，不叠加）
00:00 +4: TaskCenterStore：消息任务生命周期 finishMessage 对不存在的会话 = no-op（不新建、不发通知）
00:00 +5: TaskCenterStore：下载任务生命周期 trackDownload → RUNNING；updateDownload 单调不减、非空 detail 覆盖
00:00 +6: TaskCenterStore：下载任务生命周期 updateDownload 空 detail 不覆盖（本次没有新话说）
00:00 +7: TaskCenterStore：下载任务生命周期 finishDownload 记录本地 uri 与大小；ok=false → FAILED
00:00 +8: TaskCenterStore：下载任务生命周期 同文件还在跑时重复 trackDownload = 原样返回（不重置进度）
00:00 +9: TaskCenterStore：下载任务生命周期 已终局任务再 trackDownload = 复活重跑（进度/详情/uri 全清）
00:00 +10: TaskCenterStore：下载任务生命周期 annotateDownload 补来源会话标题；空值不覆盖
00:00 +11: TaskCenterStore：下载任务生命周期 attachLocalUriIfEmpty 只填空不覆盖（老版本 uri 兜底）
00:00 +12: TaskCenterStore：已读 / 未读 / 移除 / 清理 markRead 单条已读；已读再点 = no-op
00:00 +13: TaskCenterStore：已读 / 未读 / 移除 / 清理 unreadCount 只数未读；markAllRead 后归零且幂等
00:00 +14: TaskCenterStore：已读 / 未读 / 移除 / 清理 remove 单条；不存在的 id = no-op
00:00 +15: TaskCenterStore：已读 / 未读 / 移除 / 清理 removeByThread 清该会话全部任务（消息 + 挂 threadId 的下载）
00:00 +16: TaskCenterStore：已读 / 未读 / 移除 / 清理 clearFinished 只清终局；RUNNING 一概不碰
00:00 +17: TaskCenterStore：已读 / 未读 / 移除 / 清理 CANCELED 算终局：清理已完成会带走它（Kotlin status != RUNNING 口径）
00:00 +18: TaskCenterStore：Stream 通知 add/update/remove 都发通知；快照内容逐条相等
00:00 +19: TaskCenterStore：Stream 通知 updateDownload 进度推进也发通知
00:00 +20: TaskCenterStore：Stream 通知 updateDownload 值没变（含乱序回退）= 不发通知
00:00 +21: TaskCenterStore：Stream 通知 订阅前的变化不补（broadcast 语义；快照同步读）
00:00 +22: 磁盘编解码往返（与 Kotlin writeDisk/parseTasks 同格式） encode → decode 逐字段还原（含中文/空串/进度/uri）
00:00 +23: 磁盘编解码往返（与 Kotlin writeDisk/parseTasks 同格式） 往返保序：磁盘顺序 = 写入顺序（人不重排，daemon/写入方也不重排）
00:00 +24: 磁盘编解码往返（与 Kotlin writeDisk/parseTasks 同格式） 恢复规则：落盘 RUNNING → INTERRUPTED（杀进程的任务不可能还活着）
00:00 +25: 磁盘编解码往返（与 Kotlin writeDisk/parseTasks 同格式） 坏行整行丢弃：字段不足/未知枚举/坏时间戳
00:00 +26: 磁盘编解码往返（与 Kotlin writeDisk/parseTasks 同格式） 老格式（10 字段，无 uri/sourceTitle/sizeText）向后兼容
00:00 +27: 磁盘编解码往返（与 Kotlin writeDisk/parseTasks 同格式） 空 blob → 空清单；空白行不炸
00:00 +28: 磁盘编解码往返（与 Kotlin writeDisk/parseTasks 同格式） store 构造即恢复；未读取 max(落盘值, 清单实数)
00:00 +29: 磁盘编解码往返（与 Kotlin writeDisk/parseTasks 同格式） 写路径落盘：本地操作后 prefs 里的 blob 与新快照一致
00:00 +30: 纯函数：未读 / 统计 / 分区 unreadCountOf 只数未读
00:00 +31: 纯函数：未读 / 统计 / 分区 taskStatsOf：等待你处理只数 FAILED+INTERRUPTED（CANCELED 不算）
00:00 +32: 纯函数：未读 / 统计 / 分区 taskPartitionRows（在线）：正在执行 / 已完成 / 未完成，空组不出现
00:00 +33: 纯函数：未读 / 统计 / 分区 taskPartitionRows（离线）：消息类运行中落「离线待恢复」，下载类照常执行
00:00 +34: 纯函数：未读 / 统计 / 分区 全空清单 → 无分区（页面空态的唯一来源）
00:00 +35: 纯函数：未读 / 统计 / 分区 分组只用当前页签的行：消息类与下载类互不串门
00:00 +36: 纯函数：行渲染口径 taskStatusKindOf：RUNNING/DONE 分档；FAILED/CANCELED/INTERRUPTED 同档
00:00 +37: 纯函数：行渲染口径 taskStatusLabel 用四词（与分区头同源）
00:00 +38: 纯函数：行渲染口径 taskRowTimeLabel：MM-dd HH:mm（本地时区）；<=0 返回空串
00:00 +39: 纯函数：行渲染口径 taskOfflineHint：只在「断连 + 消息类 + 运行中」给词
00:00 +40: 纯函数：行渲染口径 taskFileKey：下载类去 dl: 前缀；消息类恒空串
00:00 +41: 纯函数：redownloadPlanOf / redownloadNeedsPc URL 前缀 → 只要手机有网
00:00 +42: 纯函数：redownloadPlanOf / redownloadNeedsPc Windows 盘符/UNC/POSIX 绝对路径 → 要电脑在线
00:00 +43: 纯函数：redownloadPlanOf / redownloadNeedsPc blob / 文本另存 → 无可重放来源（needsPc = null）
00:00 +44: 纯函数：taskNextStepCopy（与行尾真实按钮对齐） 消息类：有重试按钮才说「点右侧重试」，否则说「去会话找回」
00:00 +45: 纯函数：taskNextStepCopy（与行尾真实按钮对齐） 下载类无可重放来源：有会话 → 回会话；无会话 → 只能移除
00:00 +46: 纯函数：taskNextStepCopy（与行尾真实按钮对齐） 下载类要电脑在线：断线时先说「现在取不了」
00:00 +47: 纯函数：taskNextStepCopy（与行尾真实按钮对齐） 下载类只靠手机有网（URL）：断连也照常说「点右侧重新下载」
00:00 +48: TaskRetryGate：重试冷却门闩 冷却内第二次点击被吞；窗口外放行
00:00 +49: TaskRetryGate：重试冷却门闩 首次点击（含 nowMs=0）必过
00:00 +50: TaskRetryGate：重试冷却门闩 standard() = Kotlin TASK_RETRY_COOLDOWN_MS
00:00 +51: All tests passed!
```

**测试抓到的真 bug**（已修）：初版 `trackMessage` 对「已终局后重新 track」是**追加**，
会出现同一会话两条记录；Kotlin 是 `filterNot { it.id == id } + Task(...)`（替换）。
该行为由「同会话已终局后再 trackMessage：换新记录」钉死并修复。

### 5.3 页面层（26 过）原文

```
00:00 +0: loading G:/pa_b35/test/pages/task_center_page_test.dart
00:00 +0: 空态 消息页签空 → 「还没有处理中的任务」+ 引导文案
00:00 +1: 空态 下载页签空 → 「还没有下载任务」+ 引导文案
00:00 +2: 空态 只有下载任务时：下载页签有列表，消息页签空态（两页签分家）
00:00 +3: 列表渲染 行六要素：标题 / 时间 / 状态徽章（四词）/ 来源会话 / 大小 / 轮次
00:00 +4: 列表渲染 首帧即列表（本地 store 无加载态，不造假转圈）
00:00 +5: 列表渲染 进行中任务显示静态进度条（无无限动画，pumpAndSettle 可收敛）
00:00 +6: 列表渲染 行内徽章与分区头同源：失败行进「未完成」分区
00:00 +7: 未读徽标 未读数 = 未读任务数；点按全部标为已读后归零
00:00 +8: 未读徽标 无未读时徽标不可点（没有假动作）
00:00 +9: 未读徽标 点单个任务行 → 该条标已读（未读 -1）
00:01 +10: 本地动作 行尾移除：从列表消失，未读数同步减
00:01 +11: 本地动作 「清理已完成的任务」只清终局，RUNNING 不动
00:01 +12: 本地动作 清理按钮只在有终局任务时出现
00:01 +13: 本地动作 点消息行 → 回来源会话回调（带 threadId）
00:01 +14: 本地动作 点下载行 → 用户数据管理回调（不注入则不跳转也不报错）
00:01 +15: 重试出口（无位不显示假入口） 不注入重试回调 → 没有重试按钮，文案不指向不存在的按钮
00:01 +16: 重试出口（无位不显示假入口） 注入 onRetryMessage → 失败消息行出重试按钮，点按回传 threadId
00:01 +17: 重试出口（无位不显示假入口） 注入 onRetryDownload → URL 来源失败下载行出「重新下载」
00:01 +18: 重试出口（无位不显示假入口） blob 来源（无可重放来源）→ 没有重下按钮，文案说清只能移除
00:01 +19: 重试出口（无位不显示假入口） 重试冷却门闩：冷却内连点被吞、冷却外放行
00:01 +20: 重试出口（无位不显示假入口） 冷却窗口内按钮被「重试中」占位替换（连点物理上点不到）
00:01 +21: 离线分支（hostOffline） 消息类运行中：徽章隐去 + 「离线 · 恢复连接后继续」
00:01 +22: 离线分支（hostOffline） 下载类运行中断连照常：徽章与分区头都是「正在执行」
00:01 +23: 离线分支（hostOffline） 断连 + 消息类失败：不给重试入口（即使注入了回调）
00:02 +24: 分区 五类任务按 正在执行/已完成/未完成 分区，空组不出现
00:02 +25: 分区 两页签各自分组：下载任务的失败行只在下载页签的「未完成」组
00:02 +26: All tests passed!
```

「重试冷却门闩」用例注入可控时钟（`nowMs`）：fake_async 接管 `Future.delayed`
但不接管系统时钟，门闩的 `tryBegin(now)` 必须由注入时钟钉死——生产走系统时钟
（与 Kotlin `System.currentTimeMillis()` 同源），页面因此开了 `nowMs` 注入点
（同 `threads_bridge.dart` 的 `nowMs` 先例）。

### 5.4 全量原文（341 过）

```
00:09 +340: G:/pa_b35/test/widget_test.dart: 底部 4 Tab（会话/工具/文件/设置）默认暗色，深浅色可切
00:09 +341: All tests passed!
```

> 数量说明：任务书基线 **191 全过**（开工时实测复核一致）；本任务净增 **77**
> （51 + 26）；工作树里另有**并行的 files/git 子代理**产物
> （`files.dart`/`files_bridge.dart`/`git.dart`/`git_bridge.dart`/`pa_page_kit.dart`
> 及其测试，非本任务产物）合入后，总量到 **341**，全过。
> 声明式自证：`Get-ChildItem test -Recurse -Filter '*_test.dart'` 逐文件计数；
> 本任务两个文件单跑 = **77 全过**（`flutter test test/pages/task_center_data_test.dart
> test/pages/task_center_page_test.dart` → `+77: All tests passed!`）。
> **并行子代理的文件本任务未触碰、未修改**（全量绿包含其产物；若其单独有红，
> 与本任务无关，已按纪律分开验证）。

---

## 六、`flutter analyze`（junction 下跑，零问题）

跑法（中文路径直跑 analyze 会崩，用 junction）：

```powershell
New-Item -ItemType Junction -Path G:\pa_b35 -Target 'G:\项目1\pocketagent\flutter'
cd G:\pa_b35; C:\src\flutter\bin\flutter.bat analyze
```

结果原文：

```
Analyzing pa_b35...
No issues found! (ran in 3.2s)
```

（junction 已在使用后删除：`Remove-Item G:\pa_b35 -Force` → `Test-Path` = False。）

---

## 七、改动文件清单（全部未 commit）

**本任务改动（4 个文件，全部新增）：**

| 文件 | 改动 |
|---|---|
| `flutter/lib/pages/task_center_data.dart` | **新增**（~880 行）：模型 + 持久化抽象（接口 + 内存实现）+ 磁盘编解码 + 10 个纯函数 + 重试门闩 + `TaskCenterStore` |
| `flutter/lib/pages/task_center.dart` | **新增**（~730 行）：页面（四卡 + 两页签 + 四词分区 + 行渲染 + 行尾动作 + 底部清理 + 离线分支） |
| `flutter/test/pages/task_center_data_test.dart` | 新增（51 用例） |
| `flutter/test/pages/task_center_page_test.dart` | 新增（26 用例） |

**未触碰**（逐项核对）：`flutter/lib/transport/pa_bridge.dart`（**无 TaskCenter 方法面可加**，
见 §二；文件 mtime 与内容双重核实：grep `TaskCenter|listDevServers` 零匹配）、
`lib/gen/`、`lib/theme/`、`lib/components/`、`lib/pages/files*.dart`、`lib/pages/git*.dart`
（并行子代理在建）、`lib/pages/settings*.dart`、`lib/pages/threads*.dart`、`lib/main.dart`
（**未接线**：页面是可注入组件，根装配接入属 B4 app 级单例波次，一行都不用改）、
`android/`、`daemon/`、`proto/`、`pubspec.yaml`、`tools/`。
**未 git add / commit / push。**

---

## 八、遗留项（如实登记，不在本轮范围）

1. **`setViewing` 的 Kotlin 自动已读语义未接页面**（见 §3.2 分歧 1）：store 已实现并测透，
   页面本波用显式按钮。B4 把 store 提为 app 级单例、AppDrawer 徽标接线后，
   在页面 `initState`/`dispose` 调 `setViewing(true/false)` 即可还原。
2. **持久化是内存实现**：`shared_preferences`（或经 B2 通道委托 Kotlin 宿主读写
   `task_center` 偏好文件）属 B4。接口与磁盘格式已定并与 Kotlin 同构，
   接入只需新增一个 `TaskCenterPrefs` 实现。
3. **`flutter/docs/channel-contract.md` 无需更新**：本页不新增任何方法面
   （与 B3-3 Threads 追补 `listThreads` 不同，那次有文档滞后，已登记在该轮报告）。
4. **消息任务/下载任务的真实写入方未接**：`trackMessage`/`trackDownload` 等由
   `TurnSession`（B5）与 `PaDownloader`（B4）调用；本波 store 的方法面与单测已就绪，
   接入是调用方加一行。
5. **下载类「已完成」行没有「打开文件」按钮**（Kotlin 的 `RowActionIcon(globe)`）：
   `PaDownloader.openLocalFile` 未接线，按「无位不显示假入口」纪律只留移除，
   点整行进用户数据管理。
