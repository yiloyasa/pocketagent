# Flutter pages 域收敛（第二轮 / H5）

- 日期：2026-10-16
- 执行：收敛审查（pages 域，承接 H5 首轮「只落地 `finalBlocksText` 一处」的未竟盘点）
- 基线：`flutter test` 全量 **949 全绿**（HEAD `2cd0749`，收敛动手前实测复跑）；`flutter analyze` **No issues found**
- 边界：只改 `flutter/lib/pages/` 下 7 个授权文件 + `flutter/test/pages/` 下 3 个对应测试；**未碰**
  `lib/transport/*`（pa_bridge / chat_turn_stream / terminal_stream / chat_input_transport /
  chat_history_transport / chat_actions_transport）、`lib/gen/`、`lib/theme/`、`lib/components/`、
  `lib/main.dart`、`android/`、`daemon/`、`proto/`、`pubspec.yaml`、`tools/`、
  `flutter/docs/channel-contract.md`；`threads_bridge.dart` / `files_bridge.dart` / `git_bridge.dart`
  **不在本轮授权清单**（见 §5.1，是下一轮最明确的接续点）。
- 净变化：`git diff --stat` = **311 insertions / 184 deletions**（净 +127 行，全部是注释与共享件文档；
  逻辑代码净 **−87 行**），无格式化式重排、无换行符噪声。
- **未执行** git commit / add / push（`git status` 中我的 10 个文件全部停留在工作区未暂存）。

---

## 一、盘点结论（先看这里）

### 1.1 确实重复、已收敛

| # | 重复项 | 份数 / 位置 | 处置 |
|---|---|---|---|
| 1 | **通道错误文案的 if-else 链**（`HOST_UNAVAILABLE → 点名本通道未就绪（B4 波次）+ 收尾；其余码 message ?: code`） | 8 份 / 6 文件：threads/files×2/git/collab×3/terminal×2/chat_history | 新增 `paChannelErrorText(e, channel:, tail:)` 收口；**本轮改了在授权内的 4 份**（terminal×2、chat_history、collab 及其 2 个派生），另 4 份（threads/files×2/git）不在授权清单 → 登记移交（§5.1） |
| 2 | **settings_console 五处 `_loadX` 的「加载 → 落值 / 报错」骨架** | 4 份同型（`_loadState` / `_loadPolicy` / `_loadEvents` / `_openDetail`） | 抽私有泛型 `_loadBlock<T>`，四处调用点各传三闭包 + 两文案参数；`_loadCaps` 不同型**故意不收**（理由见 §2.4） |
| 3 | **collab 主面板的三态枚举**（`enum { loading, error, ready }` 同名同义同序） | 3 份（`_SessionsPhase` / `_PolicyPhase` / `_RolesPhase`） | 改 `typedef … = PaAsyncPhase`；`_DetailPhase` 多一个 `idle` 初态，**故意不并**（理由见 §2.3） |
| 4 | **chat_page 的「计划审查表」装配**（`showModalBottomSheet + ChatReviewSheet + onStart→pop+runGuarded`） | 2 份（slash `/计划` 分支、「更多」面板 review 项）逐字节相同 | 抽私有 `_openReviewSheet()` |
| 5 | **chat_page 的「清排队」装配**（`_runGuarded(async { notice → 判定失败/未就绪 → 收敛本地态 })`） | 2 份（`_dispatchAction` 的 clearQueue 项、`ChatRunningBar.onClearQueue`）逐字节相同 | 抽私有 `_clearQueueGuarded()` |

### 1.2 盘点方向里评估后**不动**的（附理由）

| 方向 | 结论 | 理由 |
|---|---|---|
| ① 三态装配整体抽公共 scaffold / `PaStateViews` | **不做** | 逐份比对后 pages 域实际有**四套不同型**的三态装配族，不是一套：<br>· **族 A**（threads/chat_history/terminal 内联 `Padding(top:60)+Center+Column[26×26 转圈+文案]`，错误态 `top:48+Icon(error_outline,30)+标题+原因+PAButton 重试`）——chat_history 与 terminal 两份**逐字节相同**（diff 仅 3 行：case 行、`key:` 行、两条文案），但 threads 的错误态没有 key；<br>· **族 B**（git/files 的 `_LoadingView`/`_ErrorView`/`_EmptyView` 私有类）——git 与 files 的 `_LoadingView` diff 仅 `top:48` vs `top:60`，`_ErrorView` diff 仅 `top` 与标题文案；<br>· **族 C**（collab 的 `_LoadingView`/`_ErrorView`/`_EmptyView`）——**Row + 20×20 转圈 + Container 卡片底**，与族 A/B 的居中 Column 完全不同型；<br>· **族 D**（settings 族的 `PaCardCaption`/`PaErrorRetry`，H3 已收）。<br>**本轮授权不含 threads.dart / files.dart / git.dart**，而族 A/B 的合并必须同时改这三个文件才等价（只改一边就改像素）。故登记为下一轮首选（§5.2），本轮不动。 |
| ② 错误文案映射重复 | **部分收敛**（8 份中的 4 份）+ 剩余登记 | 已证 `settings_connection.dart` 的七码正典表 `paBridgeErrorText` 确实**只有一份**，settings 四子页（agent/caps/console/downloads）已由 H3 的 `paPageErrorText` 委托，**没有**各写一份七码映射——任务书里的这个怀疑在本轮被证伪。真正的重复是通道页那一族 8 份「同型不同词」的 if-else（§2.1）。**两族不同型**：settings 族做完整码表（表外码走 `'$code：$message'` 拼装），通道族非 HOST_UNAVAILABLE 码一律原样透传 message（不加前缀）——硬并会改其中一族的文案。 |
| ③ 字节传参/解码样板 | **不动（无重复可收）** | pages 侧的 `writeToBuffer()/fromBuffer` 全部集中在三个 `*_transport.dart`（**不在授权内**，且已共用 transport 层的 `bytesOrThrow` 单点收口）；`terminal_data.dart` 的 4 个封帧函数是 proto 专属一行式，无同型副本；`settings_capabilities.dart` 的 `Uint8List` 类型检查即 H3 已登记的移交项（§5.3）。 |
| ④ Collab 主面板 vs Chat 的 reducer/accumulator 同构面 | **不同构，不动** | `ChatTimelineAccumulator` 是「帧 → tile」投影：sequence 水位去重 + draft/thinking 增量缓冲 + 五型 tile + 终局结算；`collab_session_data.dart` 是「成员快照 → 计数/状态派生」纯函数（无水位、无缓冲、无 tile）。两者共享的只有「列表遍历 + switch 枚举」这一层 Dart 惯用法，**无重复逻辑可收**。 |
| ⑤ 纯函数层重复 | **再扫一遍，只剩 2 处且已由 B4a 收敛** | 全 pages 域 138 个顶层函数 + 373 个私有方法做了两级机械扫描（规范化后全等 + Jaccard 词集 ≥0.6）：同名函数只有 `resolveCollabAgentIds` / `collabRoleDisplayName` 两组，**均已是 B4a 的委托壳**（`collab_roles_data.dart` 直接委托 `collab_session_data.dart` 的正典实现，调用方零改动）。其余高分对都是同一文件内的同族小函数（如 `isZcodeAgent`/`isCodexAgent`/`isPiAgent`、`runtimeStateLabel`/`unknownStatusLabel`），合并会把三份一目了然的判定变成一份带参数的间接层，**无收益**。 |
| ⑥ 死代码 | **未发现** | 对 24 个授权文件做了三级扫描：顶层函数全域引用计数（零引用的 0 个）、私有方法自有引用计数（0 个）、私有字段自有引用计数（0 个）。`CollabCopy` 的 11 个常量（`sectionMemberResults`/`compareResults`/`viewResult`/`actionOpenSession`…）虽无 lib 消费者，但被 `collab_session_data_test` 的禁词扫描组**逐条钉死**（P1 无比较/采用页，文案先备着不摆假入口，注释已交代）——**删了就是弱化测试，不动**。 |

---

## 二、逐项收敛记录 + 等价论证

### 2.1 `paChannelErrorText` ← 通道错误文案族 8 份中的 4 份（授权内）

**新增**（`pa_page_kit.dart`，与既有 7 个共享件同文件、同纪律）：

```dart
String paChannelErrorText(
  PaBridgeException e, {
  required String channel,
  required String tail,
}) {
  if (e.code == PaBridgeErrorCodes.hostUnavailable) {
    return '桌面端$channel通道未就绪（B4 波次）：$tail';
  }
  return e.message.isNotEmpty ? e.message : e.code;
}
```

**调用点改造**（原名、原签名、原默认值全部保留，调用方与既有单测零改动）：

| 函数 | channel | tail（原实现的冒号后全文） |
|---|---|---|
| `terminalLoadErrorText` | `终端` | `Kotlin 宿主尚未注册 bridge 通道，终端会话列表暂不可用。` |
| `terminalAttachErrorText` | `终端` | `终端连接暂不可用。`（**无中段**，见下） |
| `chatHistoryErrorText` | `会话历史` | `Kotlin 宿主尚未注册 bridge 通道，会话历史暂不可用。` |
| `collabLoadErrorText(e, {tail})` | `协同` | `Kotlin 宿主尚未注册 bridge 通道，$tail`（默认 tail `协同数据暂不可用。` 原值） |

**等价论证**：

1. **控制流**：8 份原实现都是同一形状——`if (e.code == hostUnavailable) return '<首段><尾段>'; return e.message.isNotEmpty ? e.message : e.code;`。收敛前对 6 个文件逐一取了函数体并规范化比对，**非 HOST_UNAVAILABLE 分支 8 份逐字节相同**（同一句三元表达式）。
2. **HOST_UNAVAILABLE 分支**：原实现的完整串一律是 `桌面端<channel>通道未就绪（B4 波次）：<冒号后全文>`。共享函数把「通道名」与「冒号后全文」提成两个参数，调用点各传原值——首段由共享函数拼（`'桌面端$channel通道未就绪（B4 波次）：'`），尾段整句由调用方给，故返回值与收敛前**逐字节相等**。
3. **本族有一处真实的「同型不同词」**：`terminalAttachErrorText` 与 `filePreviewErrorText` 的原实现**没有**「Kotlin 宿主尚未注册 bridge 通道，」中段（列表路径才有）。因此共享函数**不替调用方猜这句**，中段的有无由 `tail` 自带。H5 第一版曾把它硬编码进共享函数，正是被本轮收紧的全串 expect 抓到（见 §4.4 的「收敛过程实录」），随后改回「尾段整句交给调用方」。
4. **测试证据**：3 个测试文件的 5 条断言由 `contains(...)` 收紧为**全串 `expect`**（这是加强不是弱化：`contains` 只能钉片段，全串才钉得住「首段 + 中段有无 + 收尾句」三段拼接结果）。949 条全绿 = 四条路径的输出与收敛前逐字节相同。
5. **为什么放 `pa_page_kit.dart` 而不是 `settings_connection.dart`**：settings 族的 `paPageErrorText` 委托的是**七码正典表** `paBridgeErrorText`（完整 code→人话映射，表外码走 `'$code：$message'`）；本族不做码表（非 HOST_UNAVAILABLE 码一律原样透传 message，**不加** `'$code：'` 前缀，因为 host 的 message 可能已本地化）。两族语义不同型，混在一个文件里会让「到底哪份是错误码表」重新变糊。`pa_page_kit.dart` 是 pages 私域件的共享家，且已被 12 个 pages 文件 import，不新增跨域 API。

**为什么 `chat_history_data.dart` 的 import 从 `show PaBridgeErrorCodes, PaBridgeException` 收窄为 `show PaBridgeException`**：该文件不再直接引用 `PaBridgeErrorCodes`（判定搬进共享函数），收窄是 analyze 会盯着的最小 import 面，不是顺手改。

### 2.2 `_loadBlock<T>` ← settings_console 四处同型加载骨架

**收敛前**（4 份，`_loadState` / `_loadPolicy` / `_loadEvents` / `_openDetail`）：

```dart
setState(() { phase = loading; errorText = ''; });
try {
  final X v = await fetch();
  if (!mounted) return;
  setState(() { value = v; phase = ready; });
} on PaBridgeException catch (e) {
  if (!mounted) return;
  setState(() { value = null; phase = error; errorText = consolePageErrorText(e); });
} catch (e) {
  if (!mounted) return;
  setState(() { value = null; phase = error; errorText = '<本块兜底>：$e'; });
}
```

**收敛后**：私有泛型 `_loadBlock<T>({fetch, onValue, onLoading, onError, genericFailurePrefix})`，四处各传三闭包 + 一个文案参数。例：

```dart
Future<void> _loadState() => _loadBlock<RuntimeStateResponse>(
      fetch: _bridge.getRuntimeState,
      onValue: (RuntimeStateResponse v) { _runtimeState = v; _statePhase = PaAsyncPhase.ready; },
      onLoading: () { _statePhase = PaAsyncPhase.loading; _stateErrorText = ''; },
      onError: (String text) { _runtimeState = null; _statePhase = PaAsyncPhase.error; _stateErrorText = text; },
      genericFailurePrefix: '读取运行时状态失败',
    );
```

**等价论证**：

1. **异常分支顺序**：Dart 要求 `on PaBridgeException` 必须在 `catch` 之前（否则编译错，这是语言语义）。抽成函数后这个顺序原样保留，`PaBridgeException` 仍走 `consolePageErrorText(e)`、其余异常仍走 `'$prefix：$e'`。
2. **`if (!mounted) return` 的位置**：三处（成功路径落值前、`PaBridgeException` 分支报错前、`catch` 分支报错前）逐一对应原实现的三个位置，一个不少、一个不错位。
3. **setState 粒度**：原实现每处都是**一次** `setState` 包住多个字段赋值（相位 + 数据 + 文案一起进同一帧）。收敛后 `setState(() => onValue(v))` / `setState(() => onError(text))` 仍是「一次 setState 调一个闭包，闭包内改多个字段」——帧合并结果与原实现相同。
4. **`_openDetail` 的特殊处理**：它进函数前要先 `setState` 落 `_detailEventId`（页面自己的选中态，`_loadBlock` 不知道），所以它的 `onLoading` 传空闭包 `() {}`（首屏相位/文案已由前置 setState 置好），不重复置位。这一点在代码注释里点名，避免后人以为是漏写。
5. **`_loadCaps` 故意不收**：它成功路径后面多一段「位门控通过才拉三块」的连带副作用（`_loadState/_loadPolicy/_loadEvents`），且失败时清的是 `_capabilities = const <String>[]` 而不是 `null`——与其余四份不同型。收了就要把这两处差异藏进条件分支，反而不等价。保持原样（原文件 §注释已说明）。
6. **测试证据**：`settings_console_test` 的门控②组（「状态块读失败 → 策略与事件块照常就绪」）正是钉这条骨架的：mock 让 `getRuntimeState` 抛 `PlatformException(INTERNAL)` → 断言 `pa-console-state-error` 的文案 `contains('桌面端内部错误')`（走 `consolePageErrorText`），同时 `pa-console-policy-error` `findsNothing`、`pa-console-events-empty` 就绪。这条用例在收敛后仍绿 = 三块各自三态、互不拖垮的行为未变。

### 2.3 collab 三态枚举 → `PaAsyncPhase`（3 个 typedef）

**等价论证**：三个私有枚举 `enum { loading, error, ready }` 与 `pa_page_kit.dart` 的 `PaAsyncPhase` **同名同义同序**（H3 已把 caps/threads/collab_roles/terminal/chat_history 五处收成 typedef，本轮把 collab 主面板剩下的三处补上）。枚举本身**零渲染**（编译期常量），`typedef` 在运行时不产生任何包装——页面里所有 `_SessionsPhase.loading` 等比较表达式、`switch` 穷尽分支**一字未改**，`switch` 仍是穷尽匹配（analyze 的 `body_might_complete_normally` 守着，实测零问题）。

**`_DetailPhase` 故意不并**：它多一个 `idle` 初态（未选中会话时整个详情区收起，见 `_buildDetailSection` 首行的 `if (_detailPhase == _DetailPhase.idle) return const SizedBox.shrink();`）。四态与共享词汇的三态不同型：硬并要么给 `PaAsyncPhase` 加一个只有一页用的 `idle`（污染共享词汇），要么把 `idle` 塞进 `loading`（那会让「没选中」和「正在拉」在 UI 上分不开——前者是整个区块收起，后者是转圈）。保留独立枚举，注释写明理由。

### 2.4 chat_page 两处装配去重（`_openReviewSheet` / `_clearQueueGuarded`）

**等价论证**：

1. **`_openReviewSheet`**：收敛前 `_onSlashPick` 的 `review` 分支与 `_dispatchAction` 的 `ChatActionId.review` 分支各写一份 8 行 `showModalBottomSheet(isScrollControlled: true) + ChatReviewSheet(onStart: pop + _runGuarded(startReview))`，**逐字节相同**（唯一差异是 `_dispatchAction` 那份前面多 `if (!mounted) return;` 与 `await`）。抽成私有方法后 `mounted` 判定收在方法内（两处都等效：`_onSlashPick` 原本没有该判定，但它由 composer 的回调同步触发，此时 widget 必然 mounted；`_dispatchAction` 的判定前移进同一方法，净行为不变）；`await` 的移除不影响语义——原实现没有使用 `showModalBottomSheet` 的返回值（`Future<void>`），`break` 后的代码与等待无关。
2. **`_clearQueueGuarded`**：收敛前两处各写一份 `_runGuarded(() async { notice → 判失败/未就绪 → 收敛 })`，逐字节相同（仅 `_dispatchAction` 那份多一行注释，注释内容已搬进新方法的文档）。`ChatRunningBar.onClearQueue` 原本传 `() => _runGuarded(...)`（返回 `Future` 的闭包），现直接传方法引用 `_clearQueueGuarded`（`VoidCallback` 签名兼容：`Future<void> Function()` 可赋给 `void Function()`——Dart 允许返回 `Future<void>` 的函数赋给 `void` 回调槽，这是本仓库既有惯用法，见 `_runGuarded` 自身用法）。
3. **测试证据**：`chat_page_test` 的 clearQueue / review 相关用例在收敛后仍绿（含「失败串里带『失败』二字时不收敛本地态」这类诚实路径），949 条全绿。

---

## 三、改动文件清单

### 3.1 lib（7 个）

| 文件 | 变化 | 说明 |
|---|---|---|
| `flutter/lib/pages/pa_page_kit.dart` | +57 行 | 新增 `paChannelErrorText` + 族注释 + 等价论证文档（§2.1）。**未新增 widget**，只加一个纯函数。 |
| `flutter/lib/pages/terminal_data.dart` | 36 ± | `terminalLoadErrorText` / `terminalAttachErrorText` 改为委托 `paChannelErrorText`（原名原签名保留）；+import `pa_page_kit.dart`；两处函数的「收敛来历 / 中段有无」注释。 |
| `flutter/lib/pages/chat_history_data.dart` | 21 ± | `chatHistoryErrorText` 同上委托；import 从 `show PaBridgeErrorCodes, PaBridgeException` 收窄为 `show PaBridgeException`；+import `pa_page_kit.dart`。 |
| `flutter/lib/pages/collab_bridge.dart` | 18 ± | `collabLoadErrorText` 同上委托（`tail` 默认值 `'协同数据暂不可用。'` 原值，两个派生 `collabDispatchErrorText`/`collabActionErrorText` **一行未动**）；+import `pa_page_kit.dart`。 |
| `flutter/lib/pages/settings_console.dart` | 220 ±（净 −112 行逻辑） | 新增私有泛型 `_loadBlock<T>`；`_loadState`/`_loadPolicy`/`_loadEvents`/`_openDetail` 四份改为委托；`_loadCaps` 故意保持原样（理由见 §2.2-5）。 |
| `flutter/lib/pages/collab.dart` | 20 ± | `_SessionsPhase`/`_PolicyPhase`/`_RolesPhase` 三个枚举改 `typedef … = PaAsyncPhase`；`_DetailPhase` 保留独立枚举并补「为什么不并」注释；+import `pa_page_kit.dart`。 |
| `flutter/lib/pages/chat_page.dart` | 71 ±（净 −14 行逻辑） | 抽私有 `_openReviewSheet()`（slash review + 面板 review 共用）与 `_clearQueueGuarded()`（面板 clearQueue + 运行条 onClearQueue 共用）。 |

### 3.2 test（3 个）——**全部是加强，零弱化**

| 文件 | 变化 | 说明 |
|---|---|---|
| `flutter/test/pages/terminal_data_test.dart` | 24 ± | 错误文案组 2 条 `expect(... contains(...))` → **全串 `expect`**（含 attach 路径「无中段」的完整串）。 |
| `flutter/test/pages/collab_bridge_test.dart` | 22 ± | 同组 2 条 → 全串 `expect`（`collabLoadErrorText` 默认 tail + 派发/动作两个派生文案）。 |
| `flutter/test/pages/chat_history_data_test.dart` | 6 ± | `BridgeChatHistorySource` 的 HOST_UNAVAILABLE 用例由 `allOf(contains, contains)` → 全串 `expect`。 |

**没有删除任何用例、没有放松任何既有断言**：3 个文件改动前后用例声明数完全一致（`test(` 计数逐文件核对），只是把 `contains` 换成全串相等。收敛后全量 949 条 = 基线 949 条，**一条未跌**。

---

## 四、测试 / analyze 证据原文

### 4.1 基线（收敛动手前，HEAD `2cd0749`）

`flutter test` 全量（工作区 `G:\项目1\pocketagent\flutter`，收敛前）：

```
00:15 +949: All tests passed!
```

`flutter analyze`（junction `G:\pa_h5b` → `G:\项目1\pocketagent\flutter`，收敛前）：

```
Analyzing pa_h5b...
No issues found! (ran in 7.2s)
```

（中文路径 analyze 必崩的坑按任务书预案处理：junction 下跑，用完删。）

### 4.2 收敛后（最终态）

`flutter test` 全量：

```
00:15 +949: All tests passed!
```

`flutter analyze`（junction 下，全仓）：

```
Analyzing pa_h5b...
No issues found! (ran in 4.3s)
```

**949 = 949**：与基线逐条可比，零跌落、零新增、零跳过。

（收敛过程中每次中间态也都复跑过全量：`finalBlocksText` 之后的第一次 949、collab typedef 后 949、
chat_page 抽方法后 949、settings_console 泛型化后 949、测试收紧过程中 **945 −4**（见 §4.4），
修复后回到 949。最后一次全量 + analyze 是在所有编辑完成的最终态上复跑的，证据即上面两段原文。）

### 4.3 被保留 / 加强的关键断言（原样或加强后仍在）

| 测试文件 | 断言 | 状态 |
|---|---|---|
| `terminal_data_test.dart` | `HOST_UNAVAILABLE → 点名 B4 波次（列表与连接两条）` | **加强**：`contains` → 全串（列表带中段 / attach 不带中段，两条都钉） |
| `terminal_data_test.dart` | `其余码 message 原样透传；空 message 退 code` | 原样（`'还没连上主机'` / `'X'`） |
| `collab_bridge_test.dart` | `HOST_UNAVAILABLE → 点名「协同通道未就绪（B4 波次）」，不是空列表` | **加强**：`contains` → 全串 |
| `collab_bridge_test.dart` | `派发失败文案按派发语境收尾` | **加强**：`contains` → 全串（两个派生文案） |
| `collab_bridge_test.dart` | `宿主其他码 → 原样透传 message` / `无 message → 退 code` | 原样 |
| `chat_history_data_test.dart` | `Bridge 源默认请求最近 20 轮；HOST_UNAVAILABLE 翻成 B4 人话` | **加强**：`allOf(contains, contains)` → 全串 |
| `chat_history_data_test.dart` | `Bridge 源透传宿主 message；空 message 退 code` | 原样 |
| `settings_console_test.dart` | 门控②「三块各自三态，一块失败不拖垮其余」 | 原样（钉 `_loadBlock` 骨架的行为） |
| `settings_console_test.dart` | `pa-console-state/policy/events/detail-error` 各键 + `桌面端内部错误` 文案 | 原样 |
| `collab_page_test.dart` | 宿主未注册 → 错误态「未就绪（B4 波次）」，绝不是空态 | 原样（collab 文案收敛的旁证） |
| `terminal_test.dart` / `terminal_page_test.dart` | 三态互斥、`terminal-list-error` / `terminal-retry` / `terminal-empty` | 原样 |
| `chat_page_test.dart` | clearQueue / review / slash 面板相关用例 | 原样 |

### 4.4 收敛过程实录：收紧的测试抓出了一个真 bug（本轮的自我修正）

第一版 `paChannelErrorText` 把「Kotlin 宿主尚未注册 bridge 通道，」这句**硬编码进共享函数**（当时以为 8 份全有中段）。复跑全量得 **945 −4**，4 条失败全部落在我刚收紧的那 4 条全串 expect 上：

```
Expected: '桌面端终端通道未就绪（B4 波次）：终端连接暂不可用。'
  Actual: '桌面端终端通道未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，终端连接暂不可用。'
Differ at offset 18
```

（另 3 条是前缀被拼重了两次的另一版本，`Actual: '桌面端会话历史通道未就绪（B4 波次）：桌面端桥接未就绪（B4 波次）：…'`。）

**这正是本轮把 `contains` 收紧为全串 expect 的价值**：如果沿用 `contains('未就绪')` + `contains('B4 波次')`，这两处文案漂移（多一整句 / 前缀拼重）都会静默通过——而它们是**用户可见的文案错误**。修正方案：共享函数只拼「首段 + tail」，中段的有无由调用方的 `tail` 自带（`terminalAttachErrorText` 与 `files` 的 `filePreviewErrorText` 原实现就没有中段，这是本族一处真实的「同型不同词」）。修正后 949 全绿。

### 4.5 等价性的额外机械证据

- **机械扫描**：全 pages 域 138 个顶层函数 + 373 个私有方法，先做「规范化后全等」分组（0 组全等），再做 Jaccard 词集相似度（阈值 0.6）得 54 对，逐对人工判定后只有 §1.1 的 5 项是真重复；
- **死代码三级扫描**（顶层函数 / 私有方法 / 私有字段的全域引用计数）在 24 个授权文件上零命中；
- **逐行 diff 比对**：chat_history 与 terminal 的三态装配、git 与 files 的 `_LoadingView`/`_ErrorView`/`_EmptyView`、collab 三态视图，都用 `Compare-Object` 做过行级 diff（结论记在 §1.2 方向①）；
- **换行符**：`git ls-files --eol` 核对，7 个 lib 文件 + 3 个测试文件的 `i/`（index）与 `w/`（working tree）形态与改动前**完全一致**（LF 文件仍是 LF，`pa_page_kit.dart` 仍是 CRLF——它是 H3 新建时就是 CRLF 的文件）；
- **仓库风格**：short-style，**全程未跑** `dart format` / `flutter format`（H2 教训：新版 formatter 的 tall-style 会大面积重排混入噪声）。所有编辑用 edit 工具的精确替换，改完逐次 analyze；
- `git diff --stat` 只含真实语义变化（无全文件重排）：311 insertions / 184 deletions，其中新增行绝大多数是共享件与收敛注释。

---

## 五、已识别未动 + 移交清单

### 5.1 通道错误文案族的另外 4 份（**下一轮首选，授权外**）

本轮授权清单不含 `threads_bridge.dart` / `files_bridge.dart` / `git_bridge.dart`，因此这 4 份同型函数**未改**，但收敛路径已经完全铺平（共享函数 + 等价论证 + 全串测试三重就位）：

| 位置 | 函数 | channel | tail（原实现冒号后全文） |
|---|---|---|---|
| `threads_bridge.dart:189` | `threadsLoadErrorText` | `会话` | `Kotlin 宿主尚未注册 bridge 通道，会话列表暂不可用。` |
| `files_bridge.dart:288` | `filesLoadErrorText` | `文件` | `Kotlin 宿主尚未注册 bridge 通道，文件列表暂不可用。` |
| `files_bridge.dart:297` | `filePreviewErrorText` | `文件` | `文件预览暂不可用。`（**无中段**，与 `terminalAttachErrorText` 同型） |
| `git_bridge.dart:79` | `gitLoadErrorText` | `Git` | `Kotlin 宿主尚未注册 bridge 通道，分支列表暂不可用。` |

**移交要点**：改法与本轮 4 份完全同构（委托 `paChannelErrorText`，`+import 'pa_page_kit.dart'`，检查 `PaBridgeErrorCodes` 是否还有其它引用以决定 import 是否收窄）。改完请把 `files_page_test` / `git_page_test` / `threads_page_test` 里对应的 `contains` 断言一并收紧为全串（那三个文件当前只用 `find.textContaining('未就绪（B4 波次）')`，是页面级 widget 断言，收紧时注意找数据源层的直测文件）。

### 5.2 三态装配族 A / B 的合并（**下一轮次选，需三文件同批**）

族 A（threads / chat_history / terminal 内联）与族 B（git / files 私有类）**内部已经逐字节相同**（差异仅 `top:60` vs `top:48` 与文案/key）：

- 族 A 的 loading：`chat_history` 与 `terminal` 两份 diff 只有 3 行（case 行、`key:` 行、两条文案）——**可合**；
- 族 A 的 error：两份 diff 只有 case 行、key 行、标题文案、`_errorMessage` vs `_listError` 字段名——**可合**（key 需参数化：`chat-history-error` / `terminal-list-error` / `terminal-retry` / `chat-history-retry`）；
- 族 B 的 `_LoadingView`/`_ErrorView`/`_EmptyView`：git 与 files 两份 diff 只有 `top` 值与标题文案——**可合**（`top` 需参数化，且 `_ErrorView` 的 key 只有 terminal 有）。

**为什么本轮不做**：合并必须同时改 `threads.dart` / `files.dart` / `git.dart` 才逐像素等价（`top:60` vs `top:48` 是真实像素差异，只改一边就改渲染），而这三个文件**均不在本轮授权清单**。建议下一轮按「族 A 一页一页换 + 族 B 一页一页换」推进，每页换完单独复跑该页测试。

**族 C（collab）与族 D（settings）不与 A/B 合并**：族 C 是 Row + 20×20 转圈 + Container 卡片底，族 D 用 `PaCardCaption`/`PaErrorRetry`——不同型，合并要造条件分支把差异藏起来，反而不等价（H3 对 settings 三兄弟已得出同一结论）。

### 5.3 `settings_capabilities.dart` 的 `Uint8List` 类型检查（H3 已登记，本轮维持移交）

`decodeListModelsResponse` 自带 `if (raw is! Uint8List) throw DECODE_FAILED` 与 `pa_bridge.dart` 的 `_expectBytes` 同型。**处置不变**：待 B4 把 `listModels` 加成具名方法（同 `listThreads`）后，此处的类型检查与解码应下沉到 transport，pages 只留映射纯函数。**不在 pages 侧造第三个副本**。测试锚点：settings_capabilities_test 的「坏字节 → PaBridgeException(DECODE_FAILED)」。

### 5.4 `exportFailureMessage` / `_failureText`（chat_actions 族，不同型）

- `chat_actions_data.dart:183` `exportFailureMessage({code, message})`：入参是**两个 String** 不是 `PaBridgeException`，且兜底链是「message → `导出 HTML 失败（$code）` → `导出 HTML 失败`」三段——与 `paChannelErrorText` 的「message ?: code」两段不同型。不改。
- `chat_actions.dart:216` `_failureText(label, e)`：HOST_UNAVAILABLE 分支是 `'桌面端动作通道未就绪（B4 波次）：$label暂不可用。'`（**无中段**、把 label 插在中间），非 HOST_UNAVAILABLE 分支是 `'$label失败：${e.message}'` / `'$label失败（${e.code}）'`（**给 message 加了 `$label失败：` 前缀**）——与通道族的「原样透传」纪律不同型。不改。两者的文案都由 `chat_actions_test` 的全串断言钉死。

### 5.5 `collab.dart` 的 `_DetailPhase`（已识别，故意保留）

§2.3 已述：四态（多 `idle`）与 `PaAsyncPhase` 三态不同型，保留独立枚举。若后续 collab 详情区改成「未选中时也显示加载态」，届时可再评估并入。

### 5.6 纯函数层剩余同名对（已识别，已由 B4a 收敛）

`resolveCollabAgentIds` / `collabRoleDisplayName` 在 `collab_roles_data.dart` 与 `collab_session_data.dart` 各有一个同名函数——**均已是委托壳**（B4a 收敛产物，`collab_roles_data.dart` 直接调 `session_data.` 的正典实现）。本轮复核确认无第二份实现，不动。

---

## 六、红线自查

- [x] 没有替用户做审批决定；没有伪造任何数据面（错误文案、三态文案、诚实占位文案一字未改——§4.4 反而是收紧测试抓出了自己引入的文案 bug 并修回原值）。
- [x] 没有改写任何用户桌面配置（本批无写通道代码）。
- [x] 未碰 proto（双副本 MD5 不受影响）。
- [x] 未新增写死的名单/顺序/版本二分（新增的共享函数只做字符串拼接，未知输入透传路径未动；`CollabCopy` 禁词扫描组原样保留）。
- [x] 不留假绿：测试结果原文贴在本文件 §4；`flutter analyze` 零问题；未跑 `--rerun` 伪造新鲜度（证据为本次实跑输出，含中间态 945 −4 的实录）。
- [x] 「源码改对 ≠ 跑着的是新的」：§4.2 是收敛后实跑。
- [x] 测试不许弱化：3 个测试文件的改动全部是 `contains` → 全串 expect 的**加强**，用例数逐文件核对一条未变（949 = 949）。
- [x] 未针对个人环境定制（无个人路径/账号/偏好）。
- [x] 换行符保持仓库既有形态（`git ls-files --eol` 核对，§4.5）。
- [x] **未 git commit / add / push**（`git status` 中我的 10 个文件全部停留在工作区未暂存）。
- [x] 并发注意：两个 android 子代理在跑。`git status` 显示它们的改动落在
  `android/app/src/main/java/dev/pocketagent/app/flutterhost/FlutterHostEngine.kt`（M）与
  `android/app/src/test/java/dev/pocketagent/app/flutterhost/FlutterHostEventChannelsInstallTest.kt`（??），
  **与我的 10 个 flutter 文件零重叠**。全量 949 绿是在它们的工作树上跑出的，未出现非我所为的红。

---

## 七、一句话结论

本轮在 pages 域落地 5 处收敛（通道错误文案族 4/8 份 + settings_console 四处加载骨架 + collab 三个三态枚举 + chat_page 两处装配去重），净删逻辑代码 87 行；测试水位 **949 → 949**（3 个测试文件由 `contains` 收紧为全串 expect，正是它抓出并修掉了一处「中段硬编码导致文案多一整句」的回归）；`flutter analyze` 零问题。剩余最明确的接续点是 §5.1 的 4 份通道错误文案（授权外，路径已铺平）与 §5.2 的三态装配族 A/B（需 threads/files/git 三文件同批）。
