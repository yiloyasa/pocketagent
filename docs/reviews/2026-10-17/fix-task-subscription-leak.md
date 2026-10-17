# flutter 隐患修复：两处 `StreamSubscription` 从不 cancel（H-1 移交项收口）

- 日期：2026-10-17
- 上游：`docs/reviews/2026-10-17/clean-flutter.md` §3.4/§3.5（P1）派生移交项 **H-1**
  （登记于 `docs/reviews/2026-10-17/fix-flutter-p1.md:308`，理由「授权外」）
- 边界：只动 **4 个文件** —— `flutter/lib/pages/task_center.dart`、
  `flutter/lib/pages/settings_downloads.dart`、
  `flutter/test/pages/task_center_page_test.dart`、
  `flutter/test/pages/settings_downloads_test.dart`。
  **未**动其他 lib 文件 / `lib/transport/` / `lib/main.dart` / `flutter/docs/` /
  `android/` / `daemon/` / `tools/` / 任何既有 `.md`；**全程未 `dart format`**；
  **未 git add/commit/push**。
- 测试水位 **954 → 958**（+4 条判决用例）；`flutter analyze` **No issues found**。

---

## 0. 复核结论（含一处对上游说法的更正）

上游移交项 H-1 的事实陈述**全部复核为真**（两处 `listen` 确实从不 cancel、
构造器确实留了 store 注入口、`main.dart:395` 确实没注入），**但风险定性需要更正**：

| 上游说法 | 复核结果 |
|---|---|
| 「`dispose()` 之后、GC 之前若有任务事件到达即崩」 | **不成立**。两处 listener 首行都是 `if (!mounted) return;`；Flutter 的 `StatefulElement.unmount()` 在 `state.dispose()` **返回之后**才把 `state._element` 置 null，所以页面销毁一落地 `mounted` 就是 false，事件直接被降级成 no-op。实测见 §3.2：改前那 4 条判决用例里的 `expect(tester.takeException(), isNull)` **全部通过**。 |
| 「一旦 store 提成 app 级单例就必然崩」 | **同样不成立**（今天有 `mounted` 闸兜着）。但方向是对的：**真正会爆的不是 setState，是泄漏与无界累积**，见下。 |

**真正的隐患（可观测、可判决）**：store 的事件源是一条**不 close 的广播流**
（`task_center_data.dart:606`，设计如此：它默认与页面同生共死，但允许别的 owner 继续
`add`）。监听者注册进去之后**不会因为页面没了而自动摘掉**。一旦 store 的寿命长过页面
（B4 提 app 级单例正是这一步，且 `TaskCenterPage.store` / `SettingsDownloadsPage.store`
都是公开构造参数，装配点一行不改）：

- store 的监听者列表**只增不减** —— 每进出一次页面就多一个死 State；
- 每个死 State 都被监听闭包抓住 → 抓住 `widget` → **整棵已销毁的页面子树永不回收**；
- 每一条任务事件仍会打进这些死 State（做无用的字段改写）。

所以本条修复的价值是**把订阅纪律钉死在生命周期上**，而不是修一个今天就能崩的 bug。
`mounted` 闸与 `dispose` 退订不是二选一，是两道不同的闸：
前者决定「事件到了要不要动 setState」，后者决定「事件还要不要送过来」；
只留前者，**屏幕上确实看不出破绽**（这正是它容易被当成已修好的原因）。

---

## 一、隐患证据（改前）

### 1.1 两处原文

`flutter/lib/pages/task_center.dart:117-125`（改前）：

```dart
  @override
  void initState() {
    super.initState();
    // 订阅 store 变更（本地操作与后续真写入方都走这条通知）。
    _store.tasks.listen((List<TaskItem> next) {      // ← 返回值丢弃
      if (!mounted) return;
      setState(() => _tasks = next);
    });
  }
```

`flutter/lib/pages/settings_downloads.dart:247-256`（改前）：

```dart
  @override
  void initState() {
    super.initState();
    _tasks = _store.snapshot;
    _store.tasks.listen((List<TaskItem> next) {      // ← 返回值丢弃
      if (!mounted) return;
      setState(() => _tasks = next);
    });
    _loadHost();
  }
```

两个 `_…PageState` **原本都没有 `dispose()`**（`Select-String 'void dispose'` 在改前对这两个
文件零命中），订阅句柄无处安放，也无处退订。

### 1.2 事件源为什么不 close（所以不能指望「流结束自动摘人」）

`flutter/lib/pages/task_center_data.dart:604-607`（B4 波次定的口径，本条未改）：

```dart
  // 默认成对：不 close（没有 owner 时随页面一起回收；有 owner 时必须能继续
  // `add`，见 [tasks]）。
  final StreamController<List<TaskItem>> _controller =
      StreamController<List<TaskItem>>.broadcast();
```

广播流无 `onDone`、无自动清理；`cancel()` 的责任 100% 在订阅方。

### 1.3 今天为什么没炸（诚实记账）

- `main.dart:395` 的 `TaskCenterPage(...)` **不传 store** → 每次进页面新建一个 store，
  store 与页面同生共死，一起进 GC；
- `main.dart:251` 的 `SettingsPage(...)` 同样**不传 store** → `settings.dart:135` 的
  `store: widget.store` 透传下去的是 null，`SettingsDownloadsPage` 也自建 store。

也就是说：**当前生产装配下，泄漏的「受害者」和「持有者」一起死，看不见**。但注入口
是公开的（两个页面的 `store` 都是构造参数），装配点一行不用改 —— 这是典型的
「现在无害、接上就炸」的形状。

### 1.4 全量复核：还有没有第三处同形态 listener

`flutter/lib`（排除生成目录）全量 `.listen(` 命中 7 处，逐一核对：

| 位置 | 订阅句柄 | 拆卸点 | 结论 |
|---|---|---|---|
| `lib/pages/task_center.dart:139`（本次） | `_tasksSub` | `dispose`（本次新增） | 已修 |
| `lib/pages/settings_downloads.dart:266`（本次） | `_tasksSub` | `dispose`（本次新增） | 已修 |
| `lib/pages/settings_connection.dart:184` | `StreamSubscription<PaConnEvent>? _events` | `dispose(): _events?.cancel()` | 本仓既有正确范式 |
| `lib/pages/chat_session.dart:367` | `_sub` | `dispose(): unawaited(_unsubscribe())` | 正确 |
| `lib/pages/terminal_data.dart:382` | `_sub` | `close(): await _dropSubscription()` | 正确（`BridgeTerminalSession` 不是 widget State，见 §6-H2） |
| `lib/pages/terminal.dart:173 / :191` | `_subs[id]` | `dispose()` 逐个 cancel | 正确 |

**无第三处同形态缺陷**，无需越界修改。

---

## 二、改动（前后对比）

两处改法完全同构，沿用本仓 `settings_connection.dart` 的既有范式（可空句柄字段 +
`dispose` 里裸调 `?.cancel()`，不引 `unawaited`）。

### 2.1 `task_center.dart`（改后 `117-155`）

```dart
  /// store 广播流的订阅句柄：`initState` 建、`dispose` 拆。
  ///
  /// 【为什么必须拆】store 的事件源是一条**广播**流（`task_center_data.dart`
  /// 明确不 close：它默认与页面同生共死，但有别的 owner 时必须能继续 `add`），
  /// 监听者注册进去之后不会因为页面没了而自动摘掉。今天 store 是页面内建的，
  /// 两者一起进 GC，看不出问题；但 [TaskCenterPage.store] 是公开的构造注入口
  /// （B4 提 app 级单例后装配点一行不改）——**store 一旦比页面活得久，每一条
  /// 任务事件都会继续打进这个已销毁的 State**：监听闭包抓住 State → State 抓住
  /// widget → 整棵已销毁的页面子树永不回收，且 store 的监听者列表只增不减。
  ///
  /// 【与 listener 里那道 `mounted` 闸的关系】两者不是二选一，是两道不同的闸：
  /// `mounted` 闸决定「事件到了要不要动 setState」，退订决定「事件还要不要送过来」。
  /// 只留前者，屏幕上确实看不出破绽（这正是它容易被当成已修好的原因），但死 State
  /// 仍在被事件戳、仍在被 store 抓着不放。
  StreamSubscription<List<TaskItem>>? _tasksSub;

  @override
  void initState() {
    super.initState();
    // 订阅 store 变更（本地操作与后续真写入方都走这条通知）。
    _tasksSub = _store.tasks.listen((List<TaskItem> next) {
      if (!mounted) return;
      setState(() => _tasks = next);
    });
  }

  @override
  void dispose() {
    // 只退订本页自己那条订阅，**不碰 store 的任何内容**：store 可能是外部注入的
    // app 级单例，退订（收摊）与清空（毁数据）不是同一件事。
    // `cancel()` 是异步的，而 dispose 里不能 await（框架契约），调起即可 ——
    // 它对广播流做的就是「摘掉本监听者」；万一有一帧内已排队的事件穿过来，
    // listener 的 `mounted` 闸也会把它降级成一次 no-op，不会崩、也不会写脏 State。
    // 句柄用可空字段：initState 未跑完就 dispose 的极端路径下判空而不是崩。
    _tasksSub?.cancel();
    super.dispose();
  }
```

外加一行 import（`StreamSubscription` 需要 `dart:async`，放在 `package:` 之前、独立成段，
与 `settings_connection.dart:14-16` 同款）：

```dart
+import 'dart:async';
+
 import 'package:flutter/material.dart';
```

### 2.2 `settings_downloads.dart`（改后 `249-283`）

结构同上，注释按本页语境改写（强调它与任务中心**共享同一份 app 级真值**、`settings.dart`
把 store 往下透传这件事）。关键代码：

```dart
  StreamSubscription<List<TaskItem>>? _tasksSub;

  @override
  void initState() {
    super.initState();
    _tasks = _store.snapshot;
    _tasksSub = _store.tasks.listen((List<TaskItem> next) {
      if (!mounted) return;
      setState(() => _tasks = next);
    });
    _loadHost();
  }

  @override
  void dispose() {
    _tasksSub?.cancel();
    super.dispose();
  }
```

### 2.3 纪律自检

| 约束 | 落实 |
|---|---|
| `dispose` 里不 await | ✅ 裸调 `?.cancel()`，未引 `unawaited`（跟随 `settings_connection.dart` 既有风格） |
| late-init 竞态 | ✅ 字段**可空** + `?.`，不存在的分支路径判空不崩 |
| 不清空外部注入的 store | ✅ 只退订自己那条订阅，**未**加任何清 store / `markAllRead` 逻辑 |
| 不引新泄漏 | ✅ 字段是 State 的，会随 State 一起走；不存在静态持有 |
| 不引竞态 | ✅ `cancel()` 只摘监听者，queued 事件至多触发一次 `mounted` 闸后的 no-op |
| 不动 store 的 API 面（B4 成果） | ✅ `task_center_data.dart` 一字未动 |
| 不顺手重构 / 不改 UI | ✅ diff 仅 2 处 `listen` 赋值 + 2 个 `dispose` + 2 个字段 + 2 个 import |
| 换行符 | ✅ `task_center.dart` / `settings_downloads.dart` / `settings_downloads_test.dart` 保持 LF；`task_center_page_test.dart` 保持它原有的 **CRLF**（769/769，改前 693/693，未被工具改写） |

---

## 三、判决用例（必须新增，已新增 4 条，两处全覆盖）

### 3.1 观测量怎么选（这是本条的关键设计）

「dispose 时到底退订了没有」**从 UI 侧看不见**——`mounted` 闸挡住了 setState，退订与否
屏幕像素一模一样。所以判决不能只看「有没有抛异常」，只能数**事件到底送进了几个监听者**。

两个测试文件各加一只 `_CountingStore`（只覆写 `tasks` getter，在出口挂计数器）：

```dart
class _CountingStore extends TaskCenterStore {
  int deliveries = 0;

  @override
  Stream<List<TaskItem>> get tasks =>
      super.tasks.map((List<TaskItem> next) {
        deliveries += 1;
        return next;
      });
}
```

设计取舍（避免「假判决」）：

- **事件真源没换**：`super.tasks` 就是 store 自己那条私有广播流，`trackMessage` /
  `trackDownload` 照常打到真流上；覆写走的是公开可覆写面，**不是另造假数据源**；
- **计数只可能来自页面那条订阅**：`get tasks` 每次返回一条新的映射流，页面只在
  `initState` 订阅一次，测试自己不再订阅（否则计数会被探测订阅污染）；
- **每条用例都带对照组**：进页面时先推一条并断言「必达」，证明 `deliveries`
  不是恒 0 的摆设。

### 3.2 用例 A（两处各一条）：dispose 之后推事件

`task_center_page_test.dart:721-769`（`settings_downloads_test.dart` 同款）：

```dart
final _CountingStore store = _CountingStore();
await pumpPage(tester, store: store);          // store 由测试持有，活得比页面久
expect(store.deliveries, 0, reason: '首帧是同步读快照，没有任何事件可投');

await tester.pumpWidget(const SizedBox());     // 换掉整棵树 → 页面 dispose
final int afterDispose = store.deliveries;

store.trackMessage('th-after-dispose', '页面走了才来的任务');
store.trackDownload(r'C:\tmp\late.zip', 'late.zip', threadId: 'th-1');
await tester.pump();

expect(tester.takeException(), isNull, reason: '已销毁的页面不许再被事件驱动（setState after dispose）');
expect(store.deliveries, afterDispose, reason: 'dispose 必须 cancel store 订阅：…');
```

### 3.3 用例 B（两处各一条）：反复进出，监听者不跨轮累积

同文件内第二条：进页面 → 推 1 条（断言必达，**对照组**）→ `pumpWidget(SizedBox)`
→ 再进页面 → 推 1 条 → 断言 **`afterDispose + 1`**。残留订阅若在，计数会变成 `+2`。

### 3.4 既有断言一条未动

`git diff --numstat` 显示两个测试文件**只有新增**：

```
72	0	flutter/test/pages/settings_downloads_test.dart
76	0	flutter/test/pages/task_center_page_test.dart
```

（新增 0 删除，既有断言一条未删未改）；`task_center_page_test.dart` **26 → 28**、
`settings_downloads_test.dart` **14 → 16**。

### 3.5 改前跑同一批用例（红，且失败点正是计数断言）

```
$ flutter test test/pages/task_center_page_test.dart test/pages/settings_downloads_test.dart
Expected: <0>
  Actual: <2>
dispose 必须 cancel store 订阅：否则每条事件都继续打进死 State（广播流的监听闭包抓住 State
→ 页面连同子树永不回收，且 store 的监听者列表只增不减）
  The test description was: dispose 之后再推任务事件：页面已退订，不碰已销毁的 State
Expected: <2>
  Actual: <3>
第二轮只该收到 1 条：上一轮那条残留订阅若还在，会变成 2
  The test description was: 反复进出页面：每轮都退订，监听者不会跨轮累积（无残留）
…（两个文件各一组，签名完全相同）
00:04 +40 -4: Some tests failed.
```

**注意这条证据同时更正了上游的定性**：改前 `expect(tester.takeException(), isNull)`
这一行**是通过的**（失败发生在它之后的计数断言上），即**今天不会抛
`setState() called after dispose()`**；红的是「事件仍被投递给死 State」。

---

## 四、变异验证

把两个 `dispose()` 里的 `_tasksSub?.cancel();` **整行删掉**（其余一字不动），
重跑两文件：

```
$ flutter test test/pages/task_center_page_test.dart test/pages/settings_downloads_test.dart
Expected: <0>
  Actual: <2>
  The test description was: dispose 之后再推任务事件：页面已退订，不碰已销毁的 State
Expected: <2>
  Actual: <3>
  The test description was: 反复进出页面：每轮都退订，监听者不会跨轮累积（无残留）
Expected: <0>
  Actual: <2>
  The test description was: dispose 之后再推任务事件：页面已退订，不碰已销毁的 State
Expected: <2>
  Actual: <3>
  The test description was: 反复进出页面：每轮都退订，监听者不会跨轮累积（无残留）
00:04 +40 -4: Some tests failed.
```

红的是**恰好 4 条新判决用例**（两文件各 2 条），**既有 40 条一条没红**——
`+40 -4` 即 44 条里 40 过 4 挂，没有任何附带损伤。

把 `cancel()` 放回：

```
00:04 +44: All tests passed!
```

`git diff -- flutter/lib/pages/task_center.dart flutter/lib/pages/settings_downloads.dart`
复核为 `59 insertions(+), 2 deletions(-)`，两处 `_tasksSub?.cancel();` 均在位，
**变异残留为零**。

---

## 五、验证原文

### 5.1 基线（改前，`G:\pa_flx2` junction 下）

```
$ flutter test
00:42 +954: All tests passed!
```

### 5.2 改后

```
$ flutter test
00:16 +958: All tests passed!
```

### 5.3 analyze（中文路径必崩，走 `New-Item -ItemType Junction -Path G:\pa_flx2`，用完已删）

```
$ flutter analyze
Analyzing pa_flx2...
No issues found! (ran in 111.8s)
```

### 5.4 工作树范围

`git status --short` 中本条新增的改动**只有 4 个文件**：

```
 M flutter/lib/pages/settings_downloads.dart
 M flutter/lib/pages/task_center.dart
 M flutter/test/pages/settings_downloads_test.dart
 M flutter/test/pages/task_center_page_test.dart
?? docs/reviews/2026-10-17/fix-task-subscription-leak.md   （本报告，未提交）
```

（`flutter/docs/channel-contract.md`、`lib/pages/collab_roles_state.dart`、
`lib/pages/task_center_data.dart` 及对应测试是**另一条线**在本会话之前就有的改动，
本条未触碰。）

---

## 六、移交项

| 编号 | 内容 | 建议落点 |
|---|---|---|
| **H-1** | ✅ **本条已收口**：两处 `listen` 均改为存 `StreamSubscription` 句柄 + `dispose` 里 `cancel`，判决用例到位 | 可在 flutter 审查报告里划掉 |
| **H-2** | `terminal_data.dart` 的 `BridgeTerminalSession`（**不是 widget State**，是 `TerminalSession` 实现）把订阅拆卸放在 `close()`，靠 `terminal.dart:119-122` 的 `dispose()` 遍历 `session.close()` 间接触发。当前是对的，但它是**间接拆卸**：`close()` 里 `_dropSubscription()` 之前若抛异常就漏摘（`close()` 的 `try/finally` 只护 `_bridge.terminalClose`）。**本条不动**（授权外 + 终端会话的拆卸顺序属独立决策），建议终端线自查 | flutter 审查 / 终端波 |
| **H-3** | **B4「store 提成 app 级单例」落地时的验收清单**：`main.dart` 需要同时改 `:395` 的 `TaskCenterPage(store:)` 与 `:251` 的 `SettingsPage(store:)`，否则单例根本传不下去（现在两处都不传，页面各自新建）。改完后：① 两个页面读同一份真值（`settings_downloads.dart` 文件头已承诺这一语义）；② **本条的两条纪律用例（两文件各 2 条）就是那一步的回归闸**，必须重跑；③ `TaskCenterStore._controller` 按 B4 的设计**依然不 close**（单例的存活与流一致），别误加 | B4 提单例那条线 |
| **H-4** | 页面级 `mounted` 闸目前是这类 listener 的**唯一 setState 保险**，而它没有任何专门用例钉住（今天恰好安全纯属它没被重构掉）。若将来有人「清理冗余闸门」删掉 `if (!mounted) return;`，真实后果会从「泄漏」升级为「崩溃」，而现有 958 条用例**一条都不会红**。建议后续补一条「`mounted` 闸」的直接用例（可与 H-1 的计数用例同组） | flutter 审查 |

---

## 七、文件清单

### 改（4 个，全部在授权面内）

| 文件 | 改动 |
|---|---|
| `flutter/lib/pages/task_center.dart` | `+import 'dart:async'`；`+StreamSubscription<List<TaskItem>>? _tasksSub` 字段（含设计决策注释）；`listen` 返回值存入字段；`+dispose(){ _tasksSub?.cancel(); super.dispose(); }` |
| `flutter/lib/pages/settings_downloads.dart` | 同上四步（此前**没有** `dispose()`） |
| `flutter/test/pages/task_center_page_test.dart` | `+import 'dart:async'`；`+_CountingStore`；`+group('订阅生命周期（store 活得比页面久）')` 2 条用例。**纯新增 76 行，0 删除** |
| `flutter/test/pages/settings_downloads_test.dart` | `+_CountingStore`；同上 2 条用例。**纯新增 72 行，0 删除** |

### 新增文档（未提交）

- `docs/reviews/2026-10-17/fix-task-subscription-leak.md`（本文件）

### 未执行

- `dart format`：**未跑**
- `git add` / `commit` / `push`：**全未做**
- 改动面：**未越界**（未动其他 lib、`lib/transport/`、`lib/main.dart`、`flutter/docs/`、
  `android/`、`daemon/`、`tools/`、任何既有 `.md`）