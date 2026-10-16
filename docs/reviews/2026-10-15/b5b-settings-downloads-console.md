# B5-B Settings 第③子步 —— downloads / console 两个二级页（Settings 三子步收官）

> 日期：2026-10-15 ｜ 波次：B5-B（Settings 三子步最后一步）
> 范围：`flutter/lib/pages/settings_downloads.dart`（新建）、
> `flutter/lib/pages/settings_console.dart`（新建）、
> `flutter/lib/pages/settings.dart`（接线）、
> `flutter/lib/transport/pa_bridge.dart`（**只增** runtime 域 5 条只读方法面）、
> `flutter/docs/channel-contract.md`（append-only 同步）、
> `flutter/test/pages/settings_downloads_test.dart` / `settings_console_test.dart` /
> `settings_page_test.dart`（改）/ `flutter/test/transport/pa_bridge_runtime_test.dart`（新建）。
> **未 commit**（任务书纪律）。

---

## 0. 结论速览

| 项 | 结果 |
|---|---|
| 基线（开工前全量） | `flutter test` **898 全绿** |
| 收尾（全量复跑） | `flutter test` **949 全绿**（+51：新增 47 + settings_page_test 净 +4） |
| `flutter analyze`（junction `G:\pa_b5b`） | **No issues found!** |
| 本波 4 个测试文件单独跑 | **63 全过** |
| 写操作方法面 | **0 条**新增（runtime 域只接 5 条只读） |
| 假按钮 / 假开关 / 假列表 | **0**（逐条有用例钉） |
| 触碰禁区文件 | **0**（未碰 pages 其他文件 / main.dart / gen / theme / components / android / daemon / proto / pubspec / tools） |

---

## 1. 两个页面的行为

### 1.1 `settings_downloads.dart`（下载与存储，sub=downloads；旧深链 userdata 归一化到本页）

**三个分组**（`ListView` + `PaPageSection`，与其余 Settings 子页同款装配）：

1. **存储分区** —— 四行分区：下载内容 / 缓存 / 语音模型 / 其他。
   - 每行 = 分区名 + 占用 + 一行说明 + 「清理留后续波次」诚实文案。
   - 占用口径：`bytes == null` → 显示「未统计」；**绝不显示 0 B 冒充「没占用」**。
2. **下载内容 · {汇总}** —— 本地 `TaskCenterStore` 的下载类任务逐行渲染
   （标题 / 体积 / 来源会话 / 状态）。空清单 → 空态文案（Kotlin DownloadsPage 同款）。
3. **电脑端运行时** —— `getRuntimeState()` 真通道只读块（daemon 版本 / 运行时状态 /
   数据目录），三态齐全。

**纯函数（单测逐条钉）**：

| 函数 | 行为 |
|---|---|
| `parseHumanSize(text)` | `1.2MB`/`340KB`/`512B`/`2 GB`/`900` → 字节；空/不可解析 → **null**（未知 ≠ 0） |
| `downloadsTotalOf(tasks)` | 只数**已完成**下载；任一条体积未知 → `unknown=true` |
| `DownloadsTotal.label` | `N 个文件 · 共 X`；有未知条目 → `占用无法获取`；0 个 → `暂无下载内容`（不摆 0） |
| `storagePartitionsOf(tasks)` | 下载分区取本地真值；缓存/语音模型/其他 `bytes=null`；**零条目时下载分区也标未统计** |
| `downloadsPageErrorText(e)` | 委托 `paPageErrorText`，只改 HOST_UNAVAILABLE 收尾 |

**三态分家（空态 ≠ 错误态）**：

| 情形 | 渲染 |
|---|---|
| 本地清单空 | `pa-downloads-empty` 空态文案 + 分区「未统计」（**不是**错误态） |
| `getRuntimeState` 抛 HOST_UNAVAILABLE | `pa-downloads-host-error` 错误态 + 重试键（文案点名「桌面端桥接未就绪（B4 波次）」） |
| 读成功但 `data_dir` 空 | `pa-downloads-host-empty`「电脑端未报告数据目录位置」 |
| 读成功 | `pa-downloads-host-line`（daemon 版本 · 运行时 · 主机）+ `pa-downloads-host-datadir` |
| 宿主块失败 | **不拖垮本地块**：分区与空态照常渲染（Kotlin「离线可靠」同款） |

**只读纪律**：全页 0 个 `TextButton`（测试断言 `find.byType(TextButton), findsNothing`）；
清理/删除下载条目一律「留后续波次」文案，不摆假按钮。跨端调用只有 `getRuntimeState` 一次。

### 1.2 `settings_console.dart`（电脑控制台，sub=console）

**三门控**（本文件的纪律核心）：

| 门 | 触发条件 | 行为 |
|---|---|---|
| ① 位门控 | capabilities 无 `runtime.v1` | 三块内容**整块不渲染**，只给 `pa-console-no-capability`（文案点名缺的是 `runtime.v1`）；且**零 runtime 域调用**（不把 UNIMPLEMENTED 摆给用户看） |
| ①' 位读不到 | `getHostInfo` 抛 HOST_UNAVAILABLE | 能力位块错误态 + 重试（**不**渲染成「本机不支持」——读不到位 ≠ 没有位） |
| ② 逐块三态 | 三块各自 `getRuntimeState` / `getRuntimePolicy` / `listUnknownEvents` | **一块失败不拖垮其余两块**（Kotlin `RuntimeControlState` 五组 RPC 各拉各的同款）；每块独立 loading/error/ready |
| ③ 空态 ≠ 错误态 | 台账空列表 vs HOST_UNAVAILABLE | 空列表 → `pa-console-events-empty`「还没有未知事件」；读失败 → 错误态 + 重试。**空态文案绝不冒充错误** |

**三块内容（全部只读）**：

1. **运行时状态** —— `RuntimeStateResponse`：daemon 版本 / 运行时人话（ready→就绪、
   degraded→降级）/ 主机 + `contracts[]` 形态卡（agent 显示名、已申报/未申报、版本、
   六个 model 字段原文、issues/limitations 逐条）。零申报 → 「未申报任何 agent」。
2. **预算与资源** —— `RuntimePolicyResponse`：策略版本 + policy_path + `effective_global`
   额度行。**只渲染 `has*` 为真的项**（optional 语义：未设 ≠ 0，不把「没设」显示成
   「封禁」）；全未设 → 「未设置全局额度」。
3. **未知事件** —— `ListUnknownEventsResponse`：台账脚注（`累计 N 条，超出上限已丢弃
   M 条`；空台账/没丢 → null 不写噪音）+ 列表行（方法名原文 / agent / 归类 / 状态 /
   安全分级 / 「需要你处理」）。点行 → `getUnknownEvent` 详情（只读）：plain_title /
   translation / recommended_next_step / `digest[]` 透析表 / `raw_json` 脱敏原文 /
   收起详情。

**纯函数（单测逐条钉）**：`runtimeControlCapable`（大小写不敏感找 `runtime.v1`）、
`runtimeStateLabel` / `unknownStatusLabel` / `unknownKindLabel` / `safetyClassLabel`
（已知值译人话，**未知值原样透传**；UNKNOWN → 「尚未分类」而非「未知」）、
`unknownEventTitle`（方法名原文优先 → 归类 → event_id 三级兜底）、
`unknownLedgerNote`、`limitRowsOf`（只收 `has*` 项；并发轮次标 `isCount` 不带字节单位）。

**只读纪律**：全页 0 个 `Switch`/`Checkbox`/`Radio`；写操作四件套
（`setRuntimePolicy`/`setCollabPolicy`/`acknowledgeUnknownEvent`/
`executeUnknownEventAction`）**一个都不发**，也不摆灰按钮；唯一 TextButton 是
「收起详情」（纯本地 UI）。跨端调用集合 = `{getHostInfo, getRuntimeState,
getRuntimePolicy, listUnknownEvents, getUnknownEvent}`（测试用集合差断言钉死）。

### 1.3 `settings.dart` 接线

- `normalizeSettingsSub` 五值白名单**不动**（connection/downloads/agent/capabilities/
  console；userdata→downloads；未知→null）。
- `_PendingSubPage` 分支换成两个真页；`_PendingSubPage` **不退场**，改为
  「白名单加了新值却忘分派」的兜底分支（`sub != null && sub != 'connection'` 时渲染，
  静默落回一级页会让用户以为入口坏了）。
- 新增一级页「运行时」分组：`runtime.v1` 位门控的「电脑控制台」入口行
  （Kotlin SettingsScreen.kt:1648-1670 同款纪律）。能力位读自
  `getConnectionState()` 的 `hostCapabilities`（Kotlin `AgentConnection.hostCapabilities`
  的 Dart 形态，**零新方法面**）；读失败/宿主缺席 → 该组不出现，**不渲染错误横幅**
  （这一组现在是「不可判定」，不是「坏了」），一级页其余分组不依赖它。
- 新增 `store` 注入点（downloads 子页消费；与 TaskCenter 页共用同一个 app 级 store
  的等价物）。

---

## 2. 门控矩阵

| 页面 / 块 | 门控 | 位不在 | 位在读不到 | 位在 |
|---|---|---|---|---|
| 一级页「运行时」分组 | `runtime.v1` | 整组不出现 | 整组不出现（无错误横幅） | 「电脑控制台」行出现，可进真页 |
| console · 能力位块 | `runtime.v1` | —（位不在时由 `pa-console-no-capability` 承接） | 错误态 + 重试 | 通过，触发三块加载 |
| console · 运行时状态块 | 位①通过 | 不渲染 | 不渲染 | loading → error+retry / ready(+empty) |
| console · 预算与资源块 | 位①通过 | 不渲染 | 不渲染 | 同上 |
| console · 未知事件块 | 位①通过 | 不渲染 | 不渲染 | 同上；空列表 = 真空态 |
| downloads · 电脑端运行时块 | **无位门控**（Kotlin DownloadsPage 同款） | 照常尝试；失败即错误态 | 错误态 + 重试 | ready |
| downloads · 存储分区 / 下载内容 | 无（本地数据） | 照常 | 照常 | 照常 |

位门控的输入只有一处：`PaConnState.capabilities` / `HostInfo.capabilities`（同一份
宿主能力位真值，两个入口各自按自己页面已有的调用取）。

---

## 3. transport 方法面清单（`pa_bridge.dart` 只增 5 条）

| # | 方法 | proto | arguments | 返回 | 备注 |
|---|---|---|---|---|---|
| 31 | `getRuntimeState()` | `RuntimeService.GetRuntimeState`（runtime.proto:360） | `GetRuntimeStateRequest` 字节（空消息 → 空字节数组） | `RuntimeStateResponse` 字节 | 空字段 = 未报告 |
| 32 | `getRuntimePolicy()` | `RuntimeService.GetRuntimePolicy` | `GetRuntimePolicyRequest` 字节（空） | `RuntimePolicyResponse` 字节 | effective 与 policy 分开；`has*` 判定 optional |
| 33 | `getControlPlane()` | `RuntimeService.GetControlPlane`（:507） | `GetControlPlaneRequest` 字节（空） | `ControlPlane` 字节 | 人话全由 daemon 给；只给标签不给绝对路径 |
| 34 | `listUnknownEvents([request])` | `RuntimeService.ListUnknownEvents`（:171/:240） | `ListUnknownEventsRequest` 字节（缺省 = 空消息） | `ListUnknownEventsResponse` 字节 | 五筛选项原样过桥；total/dropped 必过桥 |
| 35 | `getUnknownEvent([request])` | `RuntimeService.GetUnknownEvent`（:249） | `GetUnknownEventRequest` 字节（缺省 = 空消息） | `UnknownEventResponse` 字节 | 原文可查；digest 旧 daemon 为空 |

**方法面记账（`pa_bridge.dart` 内注释 + channel-contract.md §3.16）**：

- 已接只读 6：上述 5 条 + `getCollabPolicy`（协同波次，同域只读面）。
- 未接 7（留后续波次，注释交接）：
  `SetRuntimePolicy` / `SetCollabPolicy` / `AcknowledgeUnknownEvent` /
  `ExecuteUnknownEventAction`（甲级写，完整策略 + CAS 语义）；
  `GetAgentRuntime` / `GetResourceUsage`（只读深查面）；
  `ListThreadMemories` / `ClearThreadMemory`（Kotlin 侧清单与清除同一弹层，拆开接
  只剩半个功能）。
- console 页因此**没有任何写通道**。

**契约同步（channel-contract.md，append-only）**：

- §3 标题计数：`B3 全波次后共 30 条` → `B3 全波次 + B5 Settings ③后共 35 条`；
  表内补 31–35 五行。
- 新增 §3.11–§3.15 五节（每条的错误码 / 空字段语义 / 只读边界 / 为什么这样接）。
- 新增 §3.16「runtime 域 RPC 全集与本波取舍」记账表（已接/未接/为什么不接）。
- §6 B4 清单：补 runtime 域 5 条只读的转发项 + 明说写操作四件套本波不注册；
  handler 注册条数 10 → 35。
- §7 类型来源总表：补 runtime 域 5 个返回/请求消息（gen stub，零小类包装）。

---

## 4. 测试原文

### 4.1 清单与计数

| 文件 | 用例数 | 覆盖 |
|---|---|---|
| `test/pages/settings_downloads_test.dart`（新建） | 14 | 纯函数 6 + 分区渲染 3 + 宿主块三态 4 + 路由出口 1 |
| `test/pages/settings_console_test.dart`（新建） | 16 | 纯函数 6 + 门控① 3 + 门控② 3 + 门控③ 4 |
| `test/transport/pa_bridge_runtime_test.dart`（新建） | 17 | getRuntimeState 2 + getRuntimePolicy 2 + getControlPlane 1 + list/getUnknownEvent 4 + 错误映射 8 |
| `test/pages/settings_page_test.dart`（改） | 16（原 12，净 +4） | downloads 真页 2 + console 真页 1 + 运行时门控 3（新增组）+ 原有用例 |

**新增合计 47，settings_page_test 净 +4 → 全量 898 → 949（+51）。**

### 4.2 诚实占位两条路径互斥（钉死）

- `settings_downloads_test.dart`「本地清单空 → 空态（不是错误态，也不是假列表）」：
  `pa-downloads-empty` 在场 **且** `pa-downloads-host-error` `findsNothing`；
  同时 `pa-storage-size-downloads` = 「未统计」（不是 `0 B`）。
- `settings_console_test.dart`「台账空列表 = 真空态；HOST_UNAVAILABLE = 错误态」：
  `pa-console-events-error` 在场 **且** `pa-console-events-empty` `findsNothing`
  （读失败不许渲染成「还没有未知事件」——那是真空态的文案）；
  台账脚注 `pa-console-events-ledger` 也 `findsNothing`（没有 total/dropped 可呈现）。
- `settings_console_test.dart`「位在读不到」：`pa-console-caps-error` 在场 **且**
  `pa-console-no-capability` `findsNothing`（读不到位 ≠ 没有位）。
- `settings_page_test.dart`「宿主缺席 → 运行时组不出现」：`连接与配对` 照常在场，
  `运行时` `findsNothing`，**无错误横幅**（不可判定 ≠ 坏了）。

### 4.3 位门控与零调用（钉死）

- console 位不在：`calls` 集合 == `{'getHostInfo'}`（一个 runtime 方法都不发）。
- console 位在读不到 → 重试恢复：位读到后三块标题出现，`pa-console-caps-error` 消失。
- 一级页位在：滚到「运行时」→「电脑控制台」行出现 → 点按进 `SettingsConsolePage`。
- 一级页位不在：滚到底 `运行时` `findsNothing`、`电脑控制台` `findsNothing`。

### 4.4 只读纪律（钉死）

- downloads：`find.byType(TextButton), findsNothing`（清理/删除一个按钮都不摆）；
  `calls` 方法列表 == `['getRuntimeState']`（本地块不走通道，宿主块只读一次）。
- console：`calls` 方法集合差（减去 5 个只读方法）为**空集**；
  `Switch`/`Checkbox`/`Radio` 全 `findsNothing`；TextButton 只有「收起详情」一个。
- console 收起详情：`calls.length` 不变（纯本地 UI 动作不过桥）。
- `getUnknownEvent` 请求字节回读 `eventId` == 点中的 id。

### 4.5 关键原文摘录（新增测试文件）

```dart
// settings_downloads_test.dart —— 未统计 ≠ 0
expect(
  tester
      .widget<Text>(find.byKey(const ValueKey('pa-storage-size-cache')))
      .data,
  '未统计',
  reason: '本地统计面未接入：如实标未统计，不用 0 冒充「没占用」',
);

// settings_downloads_test.dart —— 有条目体积未知 → 合计不可信
expect(total.label, '2 个文件 · 占用无法获取',
    reason: '有条目体积未知时如实说无法获取，不给偏小的数字');

// settings_console_test.dart —— 位不在时零 runtime 域调用
expect(calls.map((MethodCall c) => c.method).toSet(), <String>{'getHostInfo'},
    reason: '门控未通过时一个 runtime 方法都不发');

// settings_console_test.dart —— optional 语义
expect(limitRowsOf(RuntimeLimits()), isEmpty,
    reason: '全未设 = 一行都不渲染，不把 0 显示成「封禁」');

// settings_console_test.dart —— 安全分级不说「未知」
expect(safetyClassLabel('UNKNOWN'), '尚未分类',
    reason: '上游分类规则未定义：说「尚未分类」，不说「未知」');

// settings_console_test.dart —— 只读纪律
expect(
  methods.difference(<String>{
    'getHostInfo', 'getRuntimeState', 'getRuntimePolicy',
    'listUnknownEvents', 'getUnknownEvent',
  }),
  isEmpty,
  reason: '写操作四件套一个都不发',
);
```

---

## 5. 验证证据

### 5.1 基线（开工前）

```
$ flutter test --reporter compact
00:32 +898: All tests passed!
```

### 5.2 收尾（全量复跑，junction `G:\pa_b5b`）

```
$ flutter test --reporter compact
00:22 +949: All tests passed!
```

### 5.3 本波 4 个文件单独跑（如实登记：全量 949 里有 android 子代理并跑，
故单独复跑一遍本波文件确认无相互干扰）

```
$ flutter test test/pages/settings_downloads_test.dart test/pages/settings_console_test.dart \
    test/transport/pa_bridge_runtime_test.dart test/pages/settings_page_test.dart --reporter compact
00:11 +63: All tests passed!
```

### 5.4 `flutter analyze`（junction 下）

```
$ flutter analyze
Analyzing pa_b5b...
No issues found! (ran in 4.2s)
```

> 说明：在真实路径 `G:\项目1\pocketagent\flutter` 下 `flutter analyze` 的
> analysis server 连续三次以 code 255 退出（环境问题：与两个 android 子代理并行争
> 资源），**不是代码问题**——junction 路径下同一份代码零问题（见上）。
> 首轮 analyze 曾报 1 条 `unused_import`（`settings_downloads_test.dart` 的
> host.pb.dart），已删，复跑零问题。

---

## 6. 诚实记账与交接

### 6.1 没有做的事（以及为什么）

| 项 | 状态 | 原因 |
|---|---|---|
| 缓存/语音模型/其他分区的真实占用 | **未统计** | 需要「手机本地目录体积」平台面（path_provider + dart:io）或宿主方法面，两者都要动 pubspec 或宿主桥，超出本波授权 |
| 清理动作 | **不做** | 破坏性写（Kotlin 走 OfflineStore.clear / ThreadSnapshot.clearAll / PaCache，会话快照要二次确认） |
| 删除下载条目 | **不做** | 与「删手机文件本体」的确认流 + 文件面属同一组决策，留后续波次整组接 |
| 下载文件查看/分享 | **不做** | 需要平台文件面（content URI 打开 / 系统分享） |
| console 写操作四件套 | **不做** | 甲级写 + 完整策略 CAS 语义（只提交一个字段会把其余项重置回默认） |
| `GetAgentRuntime` / `GetResourceUsage` / `ListThreadMemories` | **不接** | 深查面 / 与写操作同弹层，拆开接只剩半个功能 |

### 6.2 踩到并钉住的坑（测试注释里留了来历）

1. **`trackDownload` 的入参是 fileKey 不是 id**：照 id 传会生成 `dl:dl:xxx`，
   一条都落不进清单（第一个用例当场踩到）。测试 helper 里留了注释。
2. **mock handler 抛任意异常会被 flutter_test 包成 `code='error'` 的
   PlatformException**：Dart 侧按契约只透传不改写，于是文案变成 `error：…`。
   测试改为按宿主形态直接抛 `PlatformException(code: HOST_UNAVAILABLE, …)`，
   并在注释里写清这个包装行为。
3. **键的落点有两种**：`PaCardCaption` 把 key 传给内部 Text（自身即命中），
   透析表的键挂在外层 Padding 上（命中容器）。测试 helper 两条路都走，
   不为布局细节写两份取法。
4. **零条目时下载分区不能显示 `0 B`**：「0 个文件」与「占了 0 字节」是两件事，
   前者该说没内容，后者是查过了。`storagePartitionsOf` 对 `count <= 0` 一律
   返回 `bytes = null`（未统计）。

### 6.3 交接注释位置

- `settings_downloads.dart` 文件末尾「交接（后续波次）」四条。
- `settings_console.dart` 文件头「三门控」+ 详情块「动作按钮本波不做」注释。
- `pa_bridge.dart` 「runtime 域 RPC 全集与本波取舍」段（已接 6 / 未接 7 逐条理由）。
- `channel-contract.md` §3.16 记账表。

---

## 7. 文件清单

### 7.1 新建

| 文件 | 行数 | MD5 |
|---|---|---|
| `flutter/lib/pages/settings_downloads.dart` | 488 | `8200A55839C60A0D90B9375716BAC78A` |
| `flutter/lib/pages/settings_console.dart` | 817 | `D2043D4F4070FD59777D968DF3577AE8` |
| `flutter/test/pages/settings_downloads_test.dart` | 475 | — |
| `flutter/test/pages/settings_console_test.dart` | 628 | — |
| `flutter/test/transport/pa_bridge_runtime_test.dart` | 450 | — |

### 7.2 修改

| 文件 | 改动 |
|---|---|
| `flutter/lib/pages/settings.dart` | imports +2/+1；类文档更新；`store` 注入点；`_loadCapabilities()`；一级页「运行时」门控组；`_PendingSubPage` 分支换真页 + 退化为兜底分支 |
| `flutter/lib/transport/pa_bridge.dart` | **只增**：runtime.pb.dart import 扩面、5 条只读方法（31–35）、6 个私有解码器、方法面头注释与「runtime 域取舍」段、`bytesOrThrow` 文档同步；**零删除零改写既有方法** |
| `flutter/docs/channel-contract.md` | append-only：§3 标题计数 30→35、表补 5 行、新增 §3.11–§3.16、§6 补 2 条 +  handler 计数 10→35、§7 补 runtime 类型行 |
| `flutter/test/pages/settings_page_test.dart` | downloads 两条旧占位用例改为真页断言；新增 console 真页用例 + 「运行时」门控组（3 例）；`scrollTo` 提升到文件级；`calls` 流水 |

### 7.3 明确未碰

`flutter/lib/pages/` 其他文件、`lib/main.dart`、`lib/gen/`、`lib/theme/`、
`lib/components/`、`android/`、`daemon/`、`proto/`、`pubspec.yaml`、`tools/`。
（git status 里的 android 改动与 `bridge/`、`flutterhost/` 未跟踪目录是两个
android 子代理的领土，与本波零文件重叠。）

---

## 8. 收尾状态

- [x] `flutter test` 全量 949 全绿（基线 898，+51）
- [x] `flutter analyze` 零问题（junction 路径）
- [x] 本波 4 个测试文件单独跑 63 全过
- [x] 写操作方法面 0 条；假按钮/假开关/假列表 0
- [x] 空态 ≠ 错误态两条路径互斥钉死
- [x] channel-contract.md 同步（§3 计数 + §3.11–§3.16 + §6 + §7）
- [x] **未 commit / add / push**
- [x] junction `G:\pa_b5b` 已删除
