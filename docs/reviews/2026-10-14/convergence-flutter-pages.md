# Flutter pages 收敛（B3-1 / B3-2 / F5 / B3-3 迁移波次新写大文件）

- 日期：2026-10-14
- 执行：收敛审查（pages 域，未过收敛审查的迁移波次产物）
- 边界：只改 `flutter/lib/pages/{settings,settings_connection,settings_agent,settings_capabilities,threads,threads_data,threads_bridge,onboarding,onboarding_steps}.dart` 与 `flutter/test/pages/` 下对应测试；**未碰** `files.dart` / `git*` / `task_center*`（并行子代理在建）、`lib/transport/`、`lib/gen/`、`lib/theme/`、`lib/components/`、`lib/main.dart`、`android/`、`daemon/`、`proto/`、`pubspec.yaml`。
- 新增文件：`flutter/lib/pages/pa_page_kit.dart`（313 行，迁移波次私有件的共享化；放在 pages/ 而非 components/，理由见 §2.0）。
- 净变化：`git diff --stat`（我的 5 个文件）= **123 insertions / 432 deletions**（净 −309 行），无换行符噪声、无格式化式重排。
- **未执行** git commit / add / push。

---

## 一、盘点结论（先看这里）

### 1.1 确实重复、已收敛

| 重复项 | 份数 | 处置 |
|---|---|---|
| 分组块（标题 + PACard + 58 缩进分隔线） | 3 份（settings `_Group`、agent `_Section`、connection/agent 内联字面量） | 合并为 `PaPageSection` |
| 32×32 圆角图标块 | 3 份（settings `_SettingsRow` 内联、connection `_IconBox`、agent `_IconBox`） | 合并为 `PaIconBox` |
| 左缩进分隔线（left 58 + Divider） | 3 份（settings/agent 内联、connection `_InsetDivider`） | 合并为 `PaInsetDivider` |
| 二级页头部（返回键 + 标题 + 可选刷新键） | 3 份（agent `_SubHeader` bottom 14、capabilities `_SubHeader` bottom 10 + 刷新、settings `_PendingSubPage` 头部 Row） | 合并为 `PaSubPageHeader`（bottomPadding 参数化，各自传原值） |
| 卡片内 caption 文案行 | 3 份（agent `_caption` 14/6/14/10、caps `_padded` symmetric(14,12) + Align、connection `_caption(danger:true)` 58/0/14/8） | 合并为 `PaCardCaption`（padding/danger 参数化） |
| 错误态装配（danger 文案 + 左对齐「重试」） | 2 份逐字节相同（agent 错误分支、caps 错误分支） | 合并为 `PaErrorRetry` |
| 「重试」TextButton（caption + brand + w600 + 左对齐） | 3 份（conn/agent/caps，键各不相同） | 合并为 `PaRetryButton` |
| 三态枚举（loading/error/ready 同名同义同序） | 2 份（caps `_ModelsPhase`、threads `_ThreadsPhase`） | 合并为 `PaAsyncPhase`，原处 typedef 保留名字 |
| 页面级错误文案的 HOST_UNAVAILABLE 委托 | 2 份（agent、caps 各写一遍 `if hostUnavailable … else paBridgeErrorText`） | 合并为 `paPageErrorText(e, hostUnavailableTail:)`，单点委托 `paBridgeErrorText` |

### 1.2 盘点方向里评估后**不动**的（附理由）

| 方向 | 结论 | 理由 |
|---|---|---|
| settings 三兄弟的「三态状态机 + 错误码文案 + 重试按钮 + Section 头」整体抽公共 scaffold | **部分做**（见 §2），不建 `SettingsSubPage` 大 scaffold | 三页差异真实存在：connection 是「一级页首组」不是二级页（无 SubHeader）；caps 有刷新键 + 前言文案 + 独立 PACard 包装；agent 的 ready 态是 chip/Wrap 流式布局。抽一个大 scaffold 会引入条件分支把差异藏起来，反而不等价。只合并**逐字节相同**的件，差异留给参数。 |
| `paBridgeErrorText`（七码映射）与 agent/caps 的错误处理是否各写一份 | **收敛** | agent/caps 并不是「各写一份七码映射」——它们本来就是委托 `paBridgeErrorText`，只重写 HOST_UNAVAILABLE 收尾。真正重复的是那 3 行委托样板，收进 `paPageErrorText`。**七码表本体仍然只有一份**（settings_connection.dart），映射结果逐条相等（测试照旧绿证，见 §4）。 |
| threads_bridge 的字节传参/解码/错误映射与 settings_* 同型代码共享 helper | **不动** | 三处形状**不同型**，不是重复：① threads_bridge 走 `PaBridge.listThreads()` 具名方法（字节校验已在 transport 层，见 pa_bridge.dart:371/464-465）；② caps 走未来 `invoke('listModels', …)` 闭包 + 自带 `Uint8List` 类型检查；③ settings_connection 完全不过 proto 字节（Map 五键）。强行统一要造抽象层把三种契约搅在一起，风险 > 收益。登记为移交建议（§5.1）。 |
| threads.dart 与 settings_capabilities.dart 的 ModelsState 四态「最像」 | **只合并枚举** | 两者只有**枚举同名同义**这一层相同；装配完全不同（threads 用 `List<Widget>` + `switch` + `Padding(top:60)` 居中转圈 + `PAButton` 重试；caps 用 PACard 内 `_buildBody` + caption 重试 + `PaCardCaption`）。合并 widget 装配会改像素，不做。枚举合并是零渲染的。 |
| onboarding `_OnboardPageView` 与 splash 动画小部件族重复 | **不存在重复** | splash 是 `AnimationController` 驱动的时间线（4 个 unbounded controller + 逐字打字机 + 光晕/进度线），onboarding 是 `PageController` 驱动的分页 + 指示点。两者共享的只有 `AnimatedSwitcher`/`FadeTransition` 的惯用法（splash 整屏淡出 vs onboarding `_PrevButton` 淡入淡出），时长/曲线/子内容全不同。**无重复可收**，登记在案备查。 |
| 死代码（无引用私有函数/参数） | **未发现** | 对 9 个在范围内的文件做了两级扫描（私有成员声明 + 全域引用计数），零死私有成员；公开符号均有 lib 内消费或测试钉死（`threadsLoadErrorText`/`threadSummaryOf`/`decodeListModelsResponse` 等虽无测试直调，但被生产路径 `BridgeThreadsDataSource` / `BridgeModelsDataSource` 消费，是契约面纯函数，保留）。 |

---

## 二、逐项收敛记录 + 等价论证

### 2.0 为什么共享件放在 `pages/pa_page_kit.dart` 而不是 `components/`

任务书只授权 pages/ 域；theme/components 已由 H2 单独收敛（`docs/reviews/2026-10-14/convergence-flutter-theme.md`）。本文件的每个件都是「迁移波次几个新文件各写一份的同款局部样式」的去重，**不是设计系统资产**（不参与 token/视觉语言决策），因此不放 components/。命名加 `Pa` 前缀只为与既有 `PACard`/`PAButton`/`PAStatusChip` 视觉命名对齐，**不新增公共 API 面**：没有文件在 pages/ 之外 import 它（grep 实证：全库 import `pa_page_kit.dart` 的只有 4 个 pages 文件）。

### 2.1 `PaPageSection` ← settings `_Group` + agent `_Section`（−约 45 行）

**等价论证**：两份实现在收敛前已逐行 diff，**逐字节相同**（仅类名与文档注释不同）。`Compare-Object` 对 `settings.dart` 227–265 行与 `settings_agent.dart` 419–457 行做 diff 输出为空。合并后结构、常量（bottom 22 / left 4 / bottom 8 / letterSpacing 0.4 / left 58）、`PACard(padding: EdgeInsets.zero)` 全部保持原值。settings 的 5 个 `_Group(` 调用点改名为 `PaPageSection(`，参数列表一字未改。

### 2.2 `PaInsetDivider` ← connection `_InsetDivider` + 2 处内联字面量

**等价论证**：connection 的 `_InsetDivider` 类与 `PaPageSection` 内原先内联的 `Padding(EdgeInsets.only(left: 58), child: Divider())` 字面量**是同一段代码**。收责任一后 `PaPageSection` 与 connection 的 build 都调共享件，构造仍为 `const`（`const PaInsetDivider()`），canonicalized 常量树与原来完全一致。

### 2.3 `PaIconBox` ← settings `_SettingsRow` 内联 + connection `_IconBox` + agent `_IconBox`（−约 40 行）

**等价论证**：三处源码收敛前已 diff：connection 与 agent 的两份 `_IconBox` 类体逐字节相同（仅文档注释一行不同）；settings `_SettingsRow` 内的 `Container(width: 32, height: 32, alignment: center, BoxDecoration(bgSurface2, circular(10)), Icon(icon, 17, fgSecondary))` 与 `_IconBox` 的 build 返回值逐字节相同。合并为同一 widget 后，三处的 32/32/center/bgSurface2/10/17/fgSecondary 全部原值。settings 行内那一份从「内联 Container」变成「`PaIconBox(icon:)`」——widget 类型从 `Container` 变成 `StatelessWidget` 包装的同一个 `Container`，子节点与约束完全相同；**测试不钉 Container 类型**（settings_page_test 只钉文本与 `SettingsAgentPage`/`SettingsCapabilitiesPage` 的存在性），已绿证。

### 2.4 `PaSubPageHeader` ← agent `_SubHeader` + caps `_SubHeader` + settings `_PendingSubPage` 头部（−约 60 行）

**等价论证**：agent 与 caps 的两份 `_SubHeader` 差异**只是** `EdgeInsets.only(bottom: 14)` vs `bottom: 10`，以及 caps 多一个刷新键 `IconButton(key: 'pa-models-refresh', Icons.refresh_rounded)`。共享件把 bottom 提为 `bottomPadding` 参数（默认 14）、刷新键提为 `onRefresh` 可选参数（非空即渲染，键名硬编码 `'pa-models-refresh'` 与收敛前一致）。调用点：agent 传 `PaSubPageHeader(title:…, backKey:'pa-agent-back', onBack:…)` 走默认 14；caps 显式 `bottomPadding: 10` + `onRefresh: _load`；settings 的 `_PendingSubPage` 头部 Row 原本就是 `Padding(bottom:14) + Row[IconButton(pa-pending-back) + SizedBox(2) + Expanded(Text(titleM))]`，与 agent 版逐字节相同，直接换成 `PaSubPageHeader(title:…, backKey:'pa-pending-back', onBack:…)`。**三个键名（pa-agent-back / pa-models-back / pa-pending-back）与 tooltip「返回」、图标 `Icons.arrow_back_rounded`、颜色 `fgSecondary` 全部原样透传**，settings_page_test 的返回键断言照旧绿。

### 2.5 `PaCardCaption` ← agent `_caption` + caps `_padded` + connection `_caption(danger: true)`（−约 25 行）

**等价论证**：三处的差异只有 padding 与颜色，都参数化且**各传原值**：
- agent：`fromLTRB(14, 6, 14, 10)` + fgTertiary（共享件默认值，调用点不传 padding）；
- caps：`symmetric(14, 12)` + fgTertiary；
- connection：`fromLTRB(58, 0, 14, 8)` + danger。

**唯一的「结构性删层」**：caps 的 `_padded` 原本是 `Padding(…) → Align(centerLeft) → child`，共享件去掉了 Align。为什么等价：`Padding` 传给子节点的是 **loose 约束**（宽度上限 = padding 内缘宽，高度不限）；`Align(alignment: centerLeft)` 在 `widthFactor`/`heightFactor` 均为 null 时自身撑满传入的 loose 约束，再把**同一组 loose 约束**透传给 `Text`。`Text` 在 loose 宽度下取自身行宽，左缘落在约束左端 —— 即 padding 内缘，与 `centerLeft` 要求的左端是同一点。故 `Text` 的最终位置、尺寸、样式逐像素相同。（`Align` 改的是它自己那个盒子，而那个盒子本来就撑满，不改变子节点布局结果。）
**无测试钉在 Align 上**：caps 测试钉的是文案内容与 key（`pa-models-loading` / `pa-models-empty` / `pa-models-error`），已绿证。

### 2.6 `PaErrorRetry` ← agent 错误分支 + caps 错误分支（−约 30 行）

**等价论证**：两份错误态分支收敛前已比对，**逐字节相同**：
`Padding(symmetric(h14, v12)) → Column(crossAxisAlignment: start) → [Text(_errorText, key, caption+danger), Align(centerLeft) → TextButton(key, onPressed:_load, Text('重试', caption+brand+w600))]`。
差异仅在 key 字符串与文案来源。共享件把 `errorText`/`errorKey`/`onRetry`/`retryKey` 四项参数化，两个调用点分别传 `('pa-agent-error','pa-agent-retry')` 与 `('pa-models-error','pa-models-retry')`。

**关于 Align 层数**：共享件 `PaErrorRetry` 的 Column 里只有一个子节点 `PaRetryButton`，而 `PaRetryButton` 内部自带那一个 `Align(centerLeft) → TextButton`。所以从 Column 往下数，层级是 `Column → PaRetryButton → Align → TextButton`，与原来的 `Column → Align → TextButton` **是同一条 Align**（只是中间多了个零渲染影响的 StatelessWidget 壳；`PaRetryButton` 自己不产盒子，build 直接返回那个 Align）。不是两层 Align 叠加。

### 2.7 `PaRetryButton` ← connection `pa-conn-retry` + agent + caps（3 处同款 TextButton）

**等价论证**：三处原本都是
```
Align(centerLeft) → TextButton(key: <各自键>, onPressed: <各自 load>, Text('重试', caption.copyWith(color: pa.brand, fontWeight: w600)))
```
（connection 那份原本把 Align 写在 `_buildStatusRow` 的 Column 里，agent/caps 写在错误块里 —— 同一枚按钮）。键名 `pa-conn-retry` / `pa-agent-retry` / `pa-models-retry` 由参数传入。threads.dart 错误态用的是 `PAButton(label:'重试')`（**另一套视觉**：filled button vs text button），**故意不并**，只在注释里点名。

### 2.8 `PaAsyncPhase` ← caps `_ModelsPhase` + threads `_ThreadsPhase`

**等价论证**：两个私有枚举 `enum { loading, error, ready }` 同名同义同序。枚举本身**零渲染**（只是编译期常量），合并后：
- caps：`typedef _ModelsPhase = PaAsyncPhase;` —— 页面里所有 `_ModelsPhase.loading` 等比较表达式一字不改，字段声明 `_ModelsPhase _phase` 也不改；
- threads：`typedef _ThreadsPhase = PaAsyncPhase;` —— 同理。

typedef 在运行时不产生任何包装，两页的 `switch`/`if` 仍是穷尽匹配（analyze 的 `body_might_complete_normally` 守着，实测零问题）。

### 2.9 `paPageErrorText` ← agent `agentPageErrorText` + caps `capabilitiesPageErrorText` 的委托样板（−约 12 行）

**等价论证**：原先两份各自写
```dart
if (e.code == hostUnavailable) return '桌面端桥接未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，<本页收尾>';
return paBridgeErrorText(e);
```
收进一个具名参数函数后，两个页面函数变成
```dart
String agentPageErrorText(PaBridgeException e) =>
    paPageErrorText(e, hostUnavailableTail: '能力位读取暂不可用。');
```
**拼接结果逐字节相同**：前缀串 `'桌面端桥接未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，'` 在 `paPageErrorText` 里只有一份，与原先两份里的前缀串字面相同；`hostUnavailableTail` 分别是 `'能力位读取暂不可用。'`（agent）与 `'模型列表暂不可用。'`（caps），与原先各自的收尾串字面相同。非 HOST_UNAVAILABLE 码仍走同一个 `paBridgeErrorText`（**七码表本体仍未动**），所以 `agentPageErrorText`/`capabilitiesPageErrorText` 的返回值对全部输入与收敛前相等 —— 测试照旧绿证（settings_agent_test 的「HOST_UNAVAILABLE 点名 B4，其余委托通用错误映射」+ 表外码透传；settings_capabilities_test 的「HOST_UNAVAILABLE 点名 B4；其余委托通用映射；表外码透传」）。

---

## 三、改动文件清单

| 文件 | 变化 | 说明 |
|---|---|---|
| `flutter/lib/pages/pa_page_kit.dart` | **新增 313 行** | 7 个共享件 + 1 个共享枚举（§2.1–2.8） |
| `flutter/lib/pages/settings.dart` | 384 → 315 | 删 `_Group` 类；5 个调用点改名；`_PendingSubPage` 头部换共享件；`_SettingsRow` 内联图标块换 `PaIconBox`；+import |
| `flutter/lib/pages/settings_agent.dart` | 479 → 331 | 删 `_SubHeader`/`_Section`/`_IconBox`/`_caption` 四个私有件与错误态内联块；错误文案委托 `paPageErrorText`；调用点全改共享件；+import |
| `flutter/lib/pages/settings_capabilities.dart` | 498 → 424 | 删 `_SubHeader`/`_padded`/错误态内联块；错误文案委托 `paPageErrorText`；三态枚举改 typedef；调用点改共享件；新增收敛注释 |
| `flutter/lib/pages/settings_connection.dart` | 546 → 522 | 删 `_InsetDivider`/`_IconBox`/`_caption` 三个私有件与状态条内联重试键；新增 `paPageErrorText`（七码表原样保留 + 委托函数与文档）；调用点改共享件；+import |
| `flutter/lib/pages/threads.dart` | 479 → 485 | 三态枚举改 typedef + 收敛注释（+6 行净增：全是注释，0 行逻辑变化） |

**测试文件：零改动**（§4 的绿证全部用原有断言跑出，没有弱化、没有新增、没有删除任何断言）。

未改：`threads_data.dart`、`threads_bridge.dart`、`onboarding.dart`、`onboarding_steps.dart`、`splash.dart`（盘点结论：无可收项，见 §1.2）。

---

## 四、测试 / analyze 证据原文

### 4.1 基线（收敛前）

`flutter test` 全量（工作区，收敛动手前）：

```
00:06 +191: All tests passed!
```

`flutter analyze`（junction `G:\pa_h3` → `G:\项目1\pocketagent\flutter`）：

```
Analyzing pa_h3...
No issues found! (ran in 56.9s)
```

### 4.2 收敛后：analyze（我的 10 个文件，junction 下）

```
Analyzing 10 items...
No issues found! (ran in 2.7s)
```

（10 items = 9 个在范围文件 + `pa_page_kit.dart`。全仓 analyze 因并行子代理的 `git_page_test.dart`/`task_center_data_test.dart` 有既存告警，不计入我的边界；我逐个文件 analyze 我的范围，零问题。）

### 4.3 收敛后：测试（与基线**同一批文件**，逐条可比）

命令 = `git ls-files flutter/test` 列出的 20 个基线测试 `.dart` 文件（第 21 个是 `test/support/pump_helpers.dart`，非测试文件；即基线 191 条对应的全部文件，一个不多一个不少；并行子代理新增的 untracked 测试文件**不在其列**）：

```
flutter test test/components/components_test.dart test/components/pa_deco_test.dart
            test/gen/wire_fixture_test.dart test/main_routing_test.dart
            test/pages/onboarding_page_test.dart test/pages/onboarding_prefs_test.dart
            test/pages/onboarding_steps_test.dart test/pages/settings_agent_test.dart
            test/pages/settings_capabilities_test.dart test/pages/settings_connection_test.dart
            test/pages/settings_page_test.dart test/pages/splash_test.dart
            test/pages/threads_bridge_test.dart test/pages/threads_data_test.dart
            test/pages/threads_page_test.dart test/theme/palette_test.dart
            test/theme/radii_test.dart test/transport/pa_bridge_test.dart
            test/transport/pa_bridge_threads_test.dart test/widget_test.dart
```

```
00:05 +191: All tests passed!
```

**191 = 191**：与 §4.1 的基线逐条可比，零跌落、零新增、零跳过。测试文件本身 `git status` 干净（我未改任何一个测试文件的一个字节），所以这 191 条断言与基线时**完全同一批**。

（① 另跑过一次「目录参数」版命令得 209–210 条全过 —— `flutter test test/components/ test/theme/ test/transport/ test/gen/` 的**目录参数会把并行子代理新建的 untracked 测试文件一起 glob 进来**（如 `test/transport/pa_bridge_files_git_test.dart`），所以那个数字既不是水位倒退也不是我的新测试，只旁证我的改动没弄坏并行代理的文件。

② 收敛后重跑同一份显式清单时得到 **192**（而非 191）：差值来自 `flutter/test/main_routing_test.dart` —— **并行子代理在我工作期间给它加了 1 条用例**（`git diff --numstat` = `26 0`，5 → 6 条 test 声明；该文件我零改动）。这与我的收敛无关：我改的 5 个 lib 文件的用例数逐文件核对**一条未变**（settings_agent 9 / settings_capabilities 17 / settings_connection 16 / settings_page 12 / threads_page 10 / threads_data 13 / threads_bridge 20 / onboarding_page 10 / onboarding_steps 17 / onboarding_prefs 4 / splash 5 / components 6+5 / theme 3+2 / gen 7 / widget 2 / transport 21+7）。

**判据**：以「我改的 pages 测试文件用例数逐文件不变 + 全量零失败」为准；并行代理对 test 树的增量不计入我的水位。）

### 4.4 三态 / 诚实占位 / 字段映射断言原样保留的实证

**逐文件用例数**（收敛后逐个文件实跑，合计数 = 191，与基线一一对应，无文件跌零、无文件暴涨）：

| 测试文件 | 用例数 |
|---|---|
| `test/transport/pa_bridge_test.dart` | 21 |
| `test/pages/threads_bridge_test.dart` | 20 |
| `test/pages/settings_capabilities_test.dart` | 17 |
| `test/pages/onboarding_steps_test.dart` | 17 |
| `test/pages/settings_connection_test.dart` | 16 |
| `test/pages/threads_data_test.dart` | 13 |
| `test/pages/settings_page_test.dart` | 12 |
| `test/pages/onboarding_page_test.dart` | 10 |
| `test/pages/threads_page_test.dart` | 10 |
| `test/pages/settings_agent_test.dart` | 9 |
| `test/gen/wire_fixture_test.dart` | 7 |
| `test/transport/pa_bridge_threads_test.dart` | 7 |
| `test/components/components_test.dart` | 6 |
| `test/main_routing_test.dart` | 5（基线）→ 6（收敛后，并行子代理加了 1 条，见 §4.3 注②） |
| `test/components/pa_deco_test.dart` | 5 |
| `test/pages/splash_test.dart` | 5 |
| `test/pages/onboarding_prefs_test.dart` | 4 |
| `test/theme/palette_test.dart` | 3 |
| `test/theme/radii_test.dart` | 2 |
| `test/widget_test.dart` | 2 |
| **合计** | **191（基线）/ 192（收敛后，+1 来自并行子代理）** |

被保留的关键断言（原文件 grep 命中，逐字未改）：
- settings_connection_test：`pa-conn-state-label` 三态（`已连接`/`检查中`/`已断开`）、`状态不可读`、`pa-conn-retry`、七码文案七条断言、表外码透传、`connStateLabelOf` 未知档透传；
- settings_agent_test：`pa-agent-loading`/`pa-agent-error`/`pa-agent-retry`/`pa-agent-empty`/`pa-agent-host-line`、`未报告任何能力位`、`桌面端桥接未就绪（B4 波次）`、原生设置门控两条；
- settings_capabilities_test：`pa-models-loading`/`pa-models-error`/`pa-models-retry`/`pa-models-empty`、`未报告任何模型`、`modelOptionsOfResponse` 字段映射（displayName 兜底 id / current 标记 / 未知上下文不造数字 / 坏字节 → DECODE_FAILED）、`ModelsUnavailableException` 原文；
- settings_page_test：`pa-pending-body` 诚实占位、`normalizeSettingsSub` 全集、sub 路由五组；
- threads_page_test：`正在加载会话…` / `会话列表加载失败` / `还没有会话` 三态互斥、`未就绪（B4 波次）` vs 空态两条路径分家、六字段行渲染、三维筛选交互。

全部随 §4.3 的 191 条一起绿。

### 4.5 等价性的额外机械证据

- `git diff --stat` 只含真实语义变化（无全文件重排）：123 insertions / 432 deletions；
- 三个「逐字节相同」的合并（`_Group`/`_Section`、两个 `_IconBox`、agent/caps 错误块）在合并前用 `Compare-Object` 对源行区间做过 diff，输出为空（仅注释行差异）；
- 仓库风格是 short-style，**全程未跑** `dart format` / `flutter format`（H2 教训：新版 formatter 的 tall-style 会大面积重排混入噪声）。所有编辑用 edit 工具的精确替换 + 少量受控 sed，改完逐文件 analyze；
- 换行符：改动文件统一回写为仓库原有的 CRLF（PowerShell 的 `Set-Content` 会把 CRLF 抹成 LF，已逐一归一化，`git diff` 无换行符噪声 —— 收敛前 `git status` 的 CRLF 警告在归一化后消失）。

---

## 五、已识别未动 + 移交建议

### 5.1 threads_bridge / settings_capabilities 的「字节传参 + 解码 + 错误映射」三处不同型（移交）

三处看起来同型、实际契约不同，本批**不合并**（理由见 §1.2）。建议后续在「B3-3 收尾 / B4 真桥接落地」时由 transport 主人一并裁决：

| 位置 | 形状 | 谁该拥有 |
|---|---|---|
| `threads_bridge.dart` `BridgeThreadsDataSource` | 调具名 `PaBridge.listThreads()`，字节校验在 transport 层 | transport |
| `settings_capabilities.dart` `BridgeModelsDataSource` + `decodeListModelsResponse` | 未来 `invoke('listModels', bytes)` 闭包 + 自带 `Uint8List` 类型检查 + `InvalidProtocolBufferException → DECODE_FAILED` | 待 B4 给 pa_bridge 补 `listModels` 后，此处的类型检查与解码应下沉到 transport，页面只留映射纯函数 |
| `settings_connection.dart` | 完全不过 proto 字节（Map 五键 wire） | 无 |

**移交要点**：若 B4 把 `listModels` 加成具名方法（同 `listThreads`），`decodeListModelsResponse` 的 `Uint8List` 检查与 `InvalidProtocolBufferException` 捕获就与 `pa_bridge.dart` 的 `_expectBytes`/`_parseHostInfo` 重复，届时应在 transport 内收敛，而不是在 pages 层造第三个副本。测试锚点：settings_capabilities_test 的「坏字节 → PaBridgeException(DECODE_FAILED)」。

### 5.2 onboarding / splash 无重复（备查结论）

§1.2 已详述。若后续波次新增第三个「时间线动画覆盖层」，再考虑抽 `PaMotion` 时长表（splash 的 `_Motion` 已是单一出处，不要提前抽）。

### 5.3 `FakeThreadsDataSource` / `kSampleThreads` 的定位（备查结论）

`threads_data.dart` 的 fake 与示例数据**只有测试与离线预览消费**（生产默认 `BridgeThreadsDataSource`）。本批不动：删了会让 threads_data_test 的 FakeThreadsDataSource 两组断言失去靶子（= 弱化测试），留着又不进生产路径。建议 B4 真桥接全面落地、线程页不再需要离线预览时，连同 `kSampleThreads` 一起评估退役，届时同步删 threads_data_test 的 `FakeThreadsDataSource` 组（**删测试必须同批报告登记**）。

### 5.4 未做「SettingsSubPage 大 scaffold」（备查结论）

§1.2 第一行已述。若后续 downloads/console 两个诚实占位页长成真页面，届时三个二级页（agent/caps/downloads|console）的骨架重复会再次浮现，**那才是**抽 `SettingsSubPage`（sub header + 三态 body 槽）的时机 —— 届时本批的 `PaSubPageHeader`/`PaCardCaption`/`PaErrorRetry` 就是它的零件，不必重做。

---

## 六、红线自查

- [x] 没有替用户做审批决定；没有伪造任何数据面（七码表、三态文案、诚实占位文案一字未改）。
- [x] 没有改写任何用户桌面配置（本批无写通道代码）。
- [x] 未碰 proto（双副本 MD5 不受影响）。
- [x] 未新增写死的名单/顺序/版本二分（新增的全是「同款样式的去重」，未知输入透传路径未动）。
- [x] 不留假绿：测试结果原文贴在本文件 §4；`flutter analyze` 零问题；未跑 `--rerun` 伪造新鲜度（证据为本次实跑输出）。
- [x] 「源码改对 ≠ 跑着的是新的」：§4.3 是收敛后实跑。
- [x] 未针对个人环境定制（无个人路径/账号/偏好）。
- [x] **未 git commit / add / push**（`git status` 中我的 6 个文件全部停留在工作区未暂存）。
