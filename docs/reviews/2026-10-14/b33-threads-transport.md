# B3-3 Flutter Threads 页接真通道（PaBridge `listThreads` + proto `Thread` 映射 + 诚实占位）

> 日期：2026-10-14 · 波次：B3-3（Threads 接真，transport 只增线程方法面）
> 范围：`flutter/lib/pages/threads*.dart`、`flutter/lib/transport/pa_bridge.dart`（只增）、
> `flutter/test/pages/`、`flutter/test/transport/` 新增测试；**未 git add/commit**。
> 验收：全量 `flutter test` **191 全过**（本轮净增 29 条用例：7+20 条在 2 个新测试文件、
> +2 条在 threads_page_test.dart；基线 124 + 并行设置波次 38 + 本轮 29 = 191）；
> junction 下 `flutter analyze` **零问题**。

---

## 一、一句话结论

Threads 页的数据源从 B3-2 的内存 fake 换成 **真通道默认**：`ThreadsPage` 不注入即构造
`BridgeThreadsDataSource`（真走 `dev.pocketagent/bridge` 的 `listThreads`），proto `Thread`
按「Android 同源口径」映射成六字段行；**宿主桥未注册（B4 前）/ 读失败走错误态
「桌面端会话通道未就绪（B4 波次）」，与空态「还没有会话」严格分家**——两条路径各有测试钉死，
绝无「假空态」。内存 fake 保留给测试注入。

---

## 二、方法面清单（transport 只增，不改既有 6 条）

### 2.1 `pa_bridge.dart` 新增第 7 条方法

| # | 方法名（契约字符串） | Dart 签名 | arguments | 返回 |
|---|---|---|---|---|
| 7 | `listThreads` | `Future<ListThreadsResponse> listThreads([ListThreadsRequest? request])` | `ListThreadsRequest` 的 wire 字节（无参 = 空请求 → **空字节数组**，编码总则 §2.1 同 `getHostInfo`） | `ListThreadsResponse` 的 wire 字节 → gen stub 反序列化 |

- **proto 同源**：`proto/v1/thread.proto:18` `rpc ListThreads(ListThreadsRequest) returns
  (ListThreadsResponse)`；生成的 Dart stub `flutter/lib/gen/pocketagent/v1/thread.pb.dart`
  直接 import（gen 目录只读，未触碰）。
- **错误语义**（Dart 侧照既有 `_invoke` 统一收敛）：
  - 宿主未注册 → `MissingPluginException` → `PaBridgeException(HOST_UNAVAILABLE)`（B4 前的常态）；
  - 宿主 `PlatformException` → `code/message/details` 原样透传（契约错误码表 §5 不改写）；
  - 返回值非 `Uint8List` / 坏字节 → `PaBridgeException(DECODE_FAILED)`（`_expectBytes` +
    `_parseListThreads`，与 `_parseHostInfo` 同款，不裸抛 protobuf 异常）。
- **【Kotlin 宿主桥 B4 波次实现】** 已按 B2 注释惯例在方法 dartdoc 标注（宿主侧清单见
  channel-contract.md §6）。
- **参数形态说明**：request 为可选。默认（无参）= 全量列表（`include_archived=false`
  等 proto 缺省语义）；带参时按 request 字段原样编码过桥——方法面不写死任何筛选默认值，
  页面本地三维筛选（`applyThreadFilter`）不变。
- **一行注释更新**：方法面区块头注释从「首批 6 条」改为「首批 6 条 + B3-3 追补 listThreads」
  （只改注释，不改任何既有方法本体）。

### 2.2 与 channel-contract.md 的对照（**发现文档滞后，未改文档**）

`flutter/docs/channel-contract.md` §3 方法清单仍写「首批 6 条」——本轮追补第 7 条后文档
滞后于代码。该文件不在本任务允许改动清单内，**如实标注为遗留项**：建议后续波次把 §3 表
与 §6 B4 清单同步补上 `listThreads`（错误码沿用 `NOT_CONNECTED`/`AUTH_REQUIRED`/`INTERNAL`
+ Dart 侧 `HOST_UNAVAILABLE`/`DECODE_FAILED`，无新码，无需扩 §5 表）。

---

## 三、字段映射表（proto `Thread` → `ThreadSummary` 六字段）

实现：`flutter/lib/pages/threads_bridge.dart`（纯函数，逐字段可单测）。口径来源全部对齐
Android 既有实现（双端同一会话长同一样，避免第二份自造口径）：

| ThreadSummary 字段 | 来源（proto 字段号） | 兜底/口径 | Android 同源 |
|---|---|---|---|
| `id` | `Thread.id`(1) | 透传 | — |
| `title` | `Thread.title`(2) | 空 →「(未命名)」 | `ThreadsScreen.kt:1988` |
| `agentId` | `Thread.agent_id`(16) | 空 → `"codex"`（proto:506 老记录兼容 / daemon `threadAgentID`） | `threadAgentIdOf` |
| `summary` | 失败档：`last_turn_error`(20) 首行摘要（40 字加 …）；否则 `message_count`(9) →「N 条消息」；都无 → 空串 | 不造占位文案 | `ThreadsScreen.kt:2007-2034` + `lastTurnErrorSummary` |
| `time` | `updated_at_unix_ms`(8) → 行标签「刚刚 / N 分钟前 / 今天 HH:mm / 昨天 HH:mm / MM-dd（跨年 yyyy-MM-dd）」 | `<=0` → 空串；时钟偏慢回落绝对日期 | `threadRowTime`（ThreadsRowLogic.kt:67） |
| `status` | `ThreadStatus.status`(6) + `pending_approvals`(21) 数 + `last_turn_error`(20) → 设计系统五态 | 优先级：**审批 > 运行 > 失败 > 完成**；UNSPECIFIED → failed 档（不冒充已完成） | `editorialStatusOf`（EditorialStatus.kt） |

状态映射细则（`threadStatusKindOf`）：

| 输入 | 输出 |
|---|---|
| `pending_approvals` 非空 或 `WAITING_APPROVAL` | `pending`（等人） |
| `RUNNING` | `running` |
| `IDLE` + `last_turn_error` 非空白 | `failed`（中断就是失败，不美化） |
| `UNSPECIFIED`（daemon 未上报；proto3 未知枚举值也被折回此档——已实测） | `failed` 档（保守，不画成成功） |
| 其余（`IDLE` 无错 / `ARCHIVED`） | `done` |

「摘要」为什么没有专字段：proto `Thread` 没有 last-message preview 字段（已全 proto grep
核对：`thread.proto` 的 `summary` 属别的消息），所以照 Android 行渲染的真实做法——失败给
错误首行，否则给消息数事实陈述。

---

## 四、诚实占位路径（红线：不留假绿）

链路：`ThreadsPage`（默认 `BridgeThreadsDataSource`）→ `PaBridge.listThreads` →
宿主缺席时 `HOST_UNAVAILABLE` → `BridgeThreadsDataSource` 把它们统一翻成
`ThreadsLoadFailure`（页面错误态唯一入口）。

| 情况 | 通道结果 | 页面表现 | 测试钉死 |
|---|---|---|---|
| **宿主桥未注册（B4 前）** | `MissingPluginException` → `HOST_UNAVAILABLE` | 错误态：「会话列表加载失败」+「**桌面端会话通道未就绪（B4 波次）**：Kotlin 宿主尚未注册 bridge 通道，会话列表暂不可用。」+ 重试按钮 | `threads_bridge_test.dart`「HOST_UNAVAILABLE → ThreadsLoadFailure，文案点名 B4 波次」；`threads_page_test.dart`「宿主未注册（B4 前）→ 错误态…绝不是空态」 |
| **宿主正常但空** | 空响应字节（0 字节，合法 proto） | 空态：「还没有会话」+「桌面端还没有会话记录，去电脑上开一个任务吧」 | `threads_bridge_test.dart`「空响应 0 字节 = 真空列表；与错误路径分家」；`threads_page_test.dart`「宿主正常回空响应 → 空态…」 |
| **宿主其他错误码**（NOT_CONNECTED 等） | `PlatformException` 透传 | 错误态展示宿主 message 原文（无 message 退 code，宁可难看也不吞） | `threads_bridge_test.dart` 两条 |
| **解码失败**（坏字节/类型不符） | `DECODE_FAILED` | 错误态展示定位信息 | `threads_bridge_test.dart` + `pa_bridge_threads_test.dart` |

**两条路径互斥断言**（防「假空态」的核心）：
- 错误态断言 `find.text('还没有会话') → findsNothing`（错误时不许出空态文案）；
- 空态断言 `find.textContaining('B4 波次') → findsNothing`、`find.textContaining('未就绪') → findsNothing`
  （空时不许出错误路径的词）。

**重试**：错误态「重试」按钮调 `_load()` 重发真调用；测试把宿主 mock 从「未注册」换成
「正常应答」后点重试，断言真转绿（重试不是装饰）。

---

## 五、测试（输出原文）

### 5.1 新增/改动文件与量级

| 文件 | 性质 | 测试数 |
|---|---|---|
| `flutter/test/transport/pa_bridge_threads_test.dart` | 新增（transport 层） | 7 |
| `flutter/test/pages/threads_bridge_test.dart` | 新增（映射纯函数 + 数据源层） | 20 |
| `flutter/test/pages/threads_page_test.dart` | 改（+3 默认接线组；删除 1 条旧「静态示例数据」用例，空态/三态旧断言更新为与新行为一致） | 10（净 +2） |
| `flutter/test/pages/threads_data_test.dart` | 未动 | 13 |

### 5.2 transport 层原文（7 过）

```
00:00 +0: loading G:/项目1/pocketagent/flutter/test/transport/pa_bridge_threads_test.dart
00:00 +0: listThreads：参数编码与返回解码 默认（无参）= 空 ListThreadsRequest 字节；响应字节 → ListThreadsResponse
00:00 +1: listThreads：参数编码与返回解码 带筛选参数：request 字段原样过桥，宿主可按 proto 回读
00:00 +2: listThreads：参数编码与返回解码 宿主回空响应字节（0 字节）= 合法空列表，不是错误
00:00 +3: listThreads：错误映射 宿主桥未实现（MissingPluginException）→ HOST_UNAVAILABLE 如实上抛
00:00 +4: listThreads：错误映射 宿主 PlatformException → code/message/details 原样透传
00:00 +5: listThreads：错误映射 返回类型不符（String 而非 wire 字节）→ DECODE_FAILED
00:00 +6: listThreads：错误映射 坏字节（非法 wire type）→ DECODE_FAILED，不裸抛 protobuf 异常
00:00 +7: All tests passed!
```

字节来源：响应 = python protoc 现生成的真 `Thread` 字节（`tools/protofixture/fixtures/thread.b64`，
与 `wire_fixture_test.dart` 同一套第三方编码器账本）包进 `ListThreadsResponse` repeated 字段
——通道层对账的就是那份真字节，不手搓常量自说自话。

### 5.3 映射 + 数据源层原文（20 过）

```
00:00 +0: loading G:/项目1/pocketagent/flutter/test/pages/threads_bridge_test.dart
00:00 +0: threadRowTimeLabel：行时间档位（口径 = Android threadRowTime） 缺席（<=0）返回空串：不显示假时间
00:00 +1: threadRowTimeLabel：行时间档位（口径 = Android threadRowTime） 一分钟内 = 刚刚；一小时内 = N 分钟前
00:00 +2: threadRowTimeLabel：行时间档位（口径 = Android threadRowTime） 今天超过一小时 = 今天 HH:mm；昨天 = 昨天 HH:mm
00:00 +3: threadRowTimeLabel：行时间档位（口径 = Android threadRowTime） 同一年更早 = MM-dd；跨年 = yyyy-MM-dd
00:00 +4: threadRowTimeLabel：行时间档位（口径 = Android threadRowTime） 客户端时钟偏慢（时间戳在未来）= 绝对日期档，不出现「负 N 分钟前」
00:00 +5: threadStatusKindOf：状态优先级（审批 > 运行 > 失败 > 完成） 挂审批（无论什么 status）= 等人（pending）——审批优先于运行
00:00 +6: threadStatusKindOf：状态优先级（审批 > 运行 > 失败 > 完成） RUNNING = running
00:00 +7: threadStatusKindOf：状态优先级（审批 > 运行 > 失败 > 完成） IDLE + last_turn_error = failed（中断就是失败，不美化）
00:00 +8: threadStatusKindOf：状态优先级（审批 > 运行 > 失败 > 完成） UNSPECIFIED（daemon 未上报/未知枚举）= failed 档，不冒充已完成
00:00 +9: threadStatusKindOf：状态优先级（审批 > 运行 > 失败 > 完成） IDLE 无错 / ARCHIVED = done
00:00 +10: lastTurnErrorSummary / threadRowSummary：摘要口径 取第一个非空行，裁到上限加省略号（Android 同款 40 字）
00:00 +11: lastTurnErrorSummary / threadRowSummary：摘要口径 失败档且错误非空 → 错误摘要；否则 N 条消息；都没有 → 空串
00:00 +12: threadRowTitle / threadAgentIdOf：兜底 空标题 → (未命名)；非空标题原样透传
00:00 +13: threadRowTitle / threadAgentIdOf：兜底 agent_id 空 → codex（proto/store 老记录兼容口径）；未知 id 不透明白名单
00:00 +14: BridgeThreadsDataSource：真通道行为 成功：python 真字节 → 六字段落位（含时间标签与状态）
00:00 +15: BridgeThreadsDataSource：真通道行为 成功（空响应 0 字节）= 真空列表；与错误路径分家
00:00 +16: BridgeThreadsDataSource：真通道行为 HOST_UNAVAILABLE → ThreadsLoadFailure，文案点名 B4 波次（不是空列表）
00:00 +17: BridgeThreadsDataSource：真通道行为 宿主其他错误码（NOT_CONNECTED）→ 透传宿主 message，不改写
00:00 +18: BridgeThreadsDataSource：真通道行为 宿主错误无 message → 退回 code（宁可难看也不吞）
00:00 +19: BridgeThreadsDataSource：真通道行为 DECODE_FAILED（坏字节）→ ThreadsLoadFailure 带定位信息
00:00 +20: All tests passed!
```

### 5.4 页面默认接线原文（10 过）

```
00:00 +0: loading G:/项目1/pocketagent/flutter/test/pages/threads_page_test.dart
00:00 +0: 三态 加载中 → 「正在加载会话…」，数据到了切换成列表
00:00 +1: 三态 加载失败 → 错误态带原因；重试后真拉到列表
00:00 +2: 三态 空列表 → 「还没有会话」（真真空态，不是通道错误的伪装）
00:00 +3: 行渲染 ThreadSummary 六字段落位：头像 agentId / 标题 / 摘要 / 时间 / 状态 chip
00:01 +4: 三维筛选交互 agent 维度：点 chip 只留该 agent 的行，再点取消
00:01 +5: 三维筛选交互 状态维度：点「已完成」只留 done 行
00:01 +6: 三维筛选交互 搜索维度：输关键词实时过滤；筛空给「清除筛选」出口
00:01 +7: 默认接线：真通道（不注入 source） 宿主未注册（B4 前）→ 错误态「未就绪（B4 波次）」，绝不是空态
00:01 +8: 默认接线：真通道（不注入 source） 宿主正常回空响应 → 空态「还没有会话」（与错误态分家的另一条）
00:01 +9: 默认接线：真通道（不注入 source） 宿主正常回列表 → 行渲染（标题/agent 头像/消息数摘要/状态）
00:01 +10: All tests passed!
```

「默认接线」组**不注入 source**：页面自己构造生产默认链（`BridgeThreadsDataSource` +
真 `dev.pocketagent/bridge`），宿主 wire 行为用 `setMockMethodCallHandler` 伪造（与
settings 页测试同款模式，PaBridge 全程真实执行）。

### 5.5 全量原文（191 过；本轮开工时实测基线 124）

```
00:05 +188: G:/项目1/pocketagent/flutter/test/widget_test.dart: 底部 4 Tab（会话/工具/文件/设置）默认暗色，深浅色可切
00:05 +189: G:/项目1/pocketagent/flutter/test/widget_test.dart: 底部 4 Tab（会话/工具/文件/设置）默认暗色，深浅色可切
00:05 +190: G:/项目1/pocketagent/flutter/test/widget_test.dart: 设置页可进入新手教程并横滑
00:06 +191: All tests passed!
```

> 数量说明：本轮开工时全量实测 **124 全过**（与任务书基线一致）；工作树里另有**并行的
> 设置波次**（`settings_agent/settings_capabilities/settings_page` 与其测试，非本任务产物）
> 合入后，加上本轮净增 29 条（7+20+2），总量到 **191**，全过。声明式自证：
> `Get-ChildItem test -Recurse -Filter '*_test.dart'` 逐文件计数合计 = 191 = 运行计数。

---

## 六、`flutter analyze`（junction 下跑，零问题）

跑法（中文路径直跑 analyze 会崩，用 junction）：

```powershell
New-Item -ItemType Junction -Path G:\pa_b33 -Target 'G:\项目1\pocketagent\flutter'
cd G:\pa_b33; C:\src\flutter\bin\flutter.bat analyze
```

结果原文：

```
Analyzing pa_b33...
No issues found! (ran in 4.4s)
```

（junction 已在使用后删除。）

---

## 七、改动文件清单（全部未 commit）

**本任务改动（9 个文件）：**

| 文件 | 改动 |
|---|---|
| `flutter/lib/transport/pa_bridge.dart` | **只增**：`listThreads` 方法 + `_decodeListThreads`/`_parseListThreads` + 区块头注释 1 行。既有 6 方法一字未动（`git diff` 已核：全部为新增行，无删除改动） |
| `flutter/lib/pages/threads_bridge.dart` | **新增**（228 行）：映射纯函数（时间/状态/摘要/title/agentId）+ `BridgeThreadsDataSource` + `threadsLoadErrorText` |
| `flutter/lib/pages/threads.dart` | 默认数据源 fake → 真通道；删除「静态示例数据」顶栏（真数据后该文案不再成立）；空态 hint 改为真真空态文案；文件头注释更新 |
| `flutter/lib/pages/threads_data.dart` | 仅注释更新（fake 保留为测试注入源；删除「后续波次接 ThreadService」等过期表述） |
| `flutter/test/transport/pa_bridge_threads_test.dart` | 新增（7 用例） |
| `flutter/test/pages/threads_bridge_test.dart` | 新增（20 用例） |
| `flutter/test/pages/threads_page_test.dart` | 改：+「默认接线：真通道」组 3 用例；旧「静态示例数据」与旧空态断言更新为与新行为一致 |
| `flutter/test/widget_test.dart` | **白名单外最小改动（必读披露，见 §八）**：+ setUp mock（22 行，含注释）；默认 Tab 断言改为「未就绪（B4 波次）」错误态 |
| `flutter/test/main_routing_test.dart` | **白名单外最小改动（必读披露，见 §八）**：+ setUp mock（19 行，含注释） |

**未触碰**（逐项核对）：`flutter/lib/gen/`（只 import）、`lib/theme/`、`lib/components/`、
`lib/pages/settings*.dart`、`lib/main.dart`（**默认注入点沿用 B3-2 的 `source` 构造注入模式，
根装配一行都不用动**）、`android/`、`daemon/`、`proto/`、`pubspec.yaml`、`tools/`。
**未 git add / commit / push**。

---

## 八、披露：两处白名单外改动（为什么必须做，做了什么）

### 8.1 背景事实（两个探针实测，非推测）

1. **`flutter_tester` 的未 mock 通道只在真实事件循环里送错**：`MethodChannel.invokeMethod`
   对无 handler 通道的 `MissingPluginException` 只在 `runAsync` 类的真实异步轮次里落地；
   fake-async 的 `pump`/`pumpAndSettle` 循环里永不送达（探针 D/E：`pumpAndSettle` 直接
   `timed out`，转圈永不停）。
2. **默认数据源接真后**，`widget_test.dart` / `main_routing_test.dart` 通过 `PocketAgentApp`
   → `HomeShell`（IndexedStack 立即构建 4 Tab）挂载**未注入的** `ThreadsPage`：
   - 页面的加载转圈会让这两个文件的 `pumpAndSettle` 全部超时（实测：4 个用例红）；
   - `widget_test.dart` 还断言了本轮删除的「静态示例数据」横幅（必删断言）。

### 8.2 处置（最小 diff + 生产路径复刻）

- 两个文件各加一个 `setUp`：mock `dev.pocketagent/bridge` —— `getConnectionState` 正常
  应答（避免设置页错误文案撑高首屏布局，挤掉惰性构建的「外观」组）；其余方法（含
  `listThreads`）抛 `MissingPluginException` = **复刻宿主未注册的生产路径**，让线程页走
  它自己的诚实错误态。这与项目内既有先例一致（settings 系测试全走同一模式）。
- `widget_test.dart` 的默认 Tab 断言改为：错误态可见 + 「未就绪（B4 波次）」可见 +
  「还没有会话」不可见——把「B4 前的默认首屏」也钉成诚实占位。

### 8.3 为什么不改 main.dart / 不另方案

- 「只许改 pages/threads*」与「全量绿」在这两处存在硬冲突（探针证明超时是通道时序的
  物理事实，不是可绕的）——选择：诚实占位不变 + 白名单外仅测试文件 + 完整披露。
- 未采用「main.dart 传 fake 兜底」（会破坏生产真通道默认，违反任务目标）、未采用
  「测试里跳过/忽略」（假绿红线）。

---

## 九、遗留项（如实登记，不在本轮范围）

1. `flutter/docs/channel-contract.md` §3/§6 未同步 `listThreads`（文档不在允许改动清单；
   代码侧已按 B4 惯例标注）。建议随 B4 宿主实现一并回填。
2. 本页只接列表首屏（`listThreads` 单源）；page-contracts §1.3 的其余三源
   （`listCodexThreads`/`listAgentThreads`/`listThreadSections`）、11 个写 RPC、离线快照
   （`ThreadSnapshot`）、`connectionEpoch` 刷新订阅均属后续波次（页面文件头已注明）。
3. 服务端过滤（`search_term`/`sort_key` 等）方法面已支持带参过桥，但页面当前仍走本地
   三维筛选（与 Android 现行为一致）；接服务端过滤属后续优化，不影响本波契约。
