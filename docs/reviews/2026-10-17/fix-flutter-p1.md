# flutter 域两条 P1 修复（task_center 通知流生命周期 + collab_roles late 幽灵初始化）

- 日期：2026-10-17
- 承接：`docs/reviews/2026-10-17/clean-flutter.md` §3.4（P1-①）/ §3.5（P1-②）
- 风格与纪律参照：`docs/reviews/2026-10-16/convergence-flutter-pages2.md`（H5b）
- 边界：只改 `flutter/lib/pages/task_center_data.dart`、`flutter/lib/pages/collab_roles_state.dart`、
  `flutter/test/pages/task_center_data_test.dart`、`flutter/test/pages/collab_roles_state_test.dart`
  **共 4 个文件**。未碰其它任何 lib 文件、`lib/transport/`、`lib/gen/`、`main.dart`、`theme/`、
  `components/`、`android/`、`daemon/`、`tools/`。**全程未跑 `dart format`**，**未 git add/commit/push**。
- 测试水位：基线 **949** → 收尾 **954**（+5 条新增判决用例），`flutter analyze` **No issues found**。

---

## 0. 结论速览

| P1 | 审查结论 | 本次核实 | 处置 |
|---|---|---|---|
| ① `_controller` 无 close = 真泄漏 | 「B4 波把 store 提成单例后」 | **前提不成立**：当前树里 `TaskCenterStore` **不是** app 级单例，两个消费方各自 `widget.store ?? TaskCenterStore()`，`main.dart` 壳不持实例。**既无泄漏，也无可调用的 close**（Kotlin 对位 `MutableStateFlow` 根本没有 close） | 走任务书允许的「无需关闭」分支：**不加死 close 方法**；改正类头里那句错注释（「提 app 级方法面不动」），写清生命周期契约 + 唯一增长向量，并钉 3 条判决用例 |
| ② `late _loadFuture` 幽灵初始化 | 查询带副作用 | **成立**：`late` 初始化器首次读才求值 → 任何「只读探查」悄悄发一次 `listCollabRoles` | 改成**显式意图**惰性：字段零副作用（已完成占位 Future），发起加载只有 `load()` 一个动作；`loadCompleted` 查询语义与既有时序逐条不变。**新增 2 条判决用例**，其中核心断言=「只读查询零 RPC」 |

---

## 一、P1-① `TaskCenterStore._controller` 无 close —— 复核 + 裁决

### 1.1 审查给的前提（B4 已把 store 提成单例）**经核实不成立**

审查 §3.4 原文：「`TaskCenterStore` 是「app 级 store」…… **一旦 B4 把 store 提为 app 级单例**（注释明说届时这么干），`_controller` 永不 close 会变成真泄漏。」
任务书转述为「B4 波把 store 提成单例后」。**当前树上它还不是单例。** 机械证据（`Select-String` 全 `flutter/lib` 扫 `TaskCenterStore`，共 15 处命中，无 `static` 实例、无 provider/InheritedWidget）：

```
lib\pages\task_center.dart:61        final TaskCenterStore? store;          // 构造注入参数
lib\pages\task_center.dart:101       late final TaskCenterStore _store = widget.store ?? TaskCenterStore();
lib\pages\settings_downloads.dart:223 final TaskCenterStore? store;
lib\pages\settings_downloads.dart:236 late final TaskCenterStore _store = widget.store ?? TaskCenterStore();
lib\pages\settings.dart:67           final TaskCenterStore? store;         // 只透传给 downloads 子页
lib\pages\task_center_data.dart:560  class TaskCenterStore {               // 无 static 实例 / 无工厂单例
```

`main.dart` 的壳（`_HomeShellState`）只持 `_collabRolesStore`（协同角色，B3 产物），**没有**任务中心 store；
`_TaskCenterRoutePage`（main.dart:395）构造 `TaskCenterPage()` 不传 store → 每次进页面新建一个。
**结论：store 生命周期 = 页面生命周期，且今天就是**。

### 1.2 那「唯一的增长向量」是**页面不取消订阅**，今天无害、提单例后才致命

两个页面都在 `initState` 里 `listen` 且从不 `cancel`：

```
lib\pages\task_center.dart:121        _store.tasks.listen((List<TaskItem> next) { … });   // 无 StreamSubscription 字段、无 dispose
lib\pages\settings_downloads.dart:251 _store.tasks.listen((List<TaskItem> next) { … });   // 同上
```

- **今天**：store 随页面一起被回收，那个环 `State → store → controller → 订阅 → 闭包 → State`
  是**无外部根的引用环**，Dart 追踪 GC 整体回收 → **无泄漏**。
- **提单例后**：进程级 controller 上每开一次页面永久挂一个监听（还挂着整棵已 dispose 的 widget 子树）→ 真泄漏。
- 修法在**页面侧**（`listen` 返回值存字段、`dispose` 里 `cancel`，同
  `BridgeTerminalSession.close()` 的 owner-关流纪律）。**`task_center.dart` / `settings_downloads.dart`
  不在本轮授权清单内 → 登记移交（§五），不在 `task_center_data.dart` 里加一个没人调的 close 方法。**

### 1.3 为什么「不 close」在 Dart 侧也站得住（同仓既有范式 + 对位 Kotlin）

- **同仓既有范式是「流的所有者负责 close」**：`BridgeTerminalSession`（`terminal_data.dart:425-436`）
  是全仓唯一带 close 的流持有者，`_closed` 幂等闩 + `await _out.close(); await _exit.close();`，
  调用方是 `terminal.dart:112 dispose()` —— **有 owner 才 close**。
  `TaskCenterStore` 的 owner 只能是页面，而页面该做的动作是**退订**不是 close。
- **对位 Kotlin 侧结构上就没有 close**：`android/.../ui/component/TaskCenter.kt` 是
  `object TaskCenter`，通知面是 `MutableStateFlow`（第 82-88 行 `_tasks` / `_unreadCount`），
  **`StateFlow` 没有 close 方法**。凭空在 Dart 侧补一个 `dispose()` 会造出
  一个没有 Kotlin 对应物、又无调用方的假接口（违反「不留假绿 / 不自创」的纪律）。
- **未 close 的 broadcast controller 不按事件占内存**：没有监听者时 `add()` 的事件**直接丢弃**，
  不进任何队列。它只按**活跃订阅**占内存。这条是 Dart SDK 的 broadcast 语义，也是本仓既有测试
  「订阅前的变化不补」钉住的语义 → 升格为显式判决用例（§三）。

### 1.4 顺手改正了一句错注释（原类头）

原句：「测试各拿各的实例互不串味。**届时把实例提到 app 级即可，方法面不动**。」
——这句**是错的**（提到 app 级必然要改方法面/生命周期契约）。按「零谎言注释」纪律改正，
并把生命周期契约、close 决策、唯一增长向量写进类头（`task_center_data.dart:548-575`）。

### 1.5 改动前后对比

改动前（`task_center_data.dart` 类头 + 字段）：

```dart
/// 【为什么不是全局单例】…本波先给**可注入的类**——页面用构造注入…，
/// 测试各拿各的实例互不串味。届时把实例提到 app 级即可，方法面不动。
…
final StreamController<List<TaskItem>> _controller =
    StreamController<List<TaskItem>>.broadcast();
…
/// 变更通知流（broadcast）。
Stream<List<TaskItem>> get tasks => _controller.stream;
```

改动后（**零逻辑行变化**，只补契约与文档）：

```dart
/// 【生命周期 = 实例；刻意**不**提供 close】原注释此处写的是「届时把实例提到
/// app 级即可，方法面不动」——那句是错的，已改正：提到 app 级**必然要改方法面**
/// （见下面【唯一的增长向量】）。…当前树上**没有任何 app 级持有者**…今天
/// **既不存在泄漏、也不需要 close**。…Kotlin 那个 `object TaskCenter` 的通知面是
/// `MutableStateFlow`…**StateFlow 根本没有 close**…
///
/// 【唯一的增长向量：订阅者不取消】未 close 的 broadcast controller 只按**活跃
/// 订阅**占内存，**不按事件条数**占…B4 真把 store 提为 app 级单例的那天…那才是真泄漏
/// ——修法在页面侧…故登记移交、不在这里加没人调的 close 方法。

/// 【为什么没有 close】…未 close 的代价为零——无监听者时 `add` 不缓冲。
final StreamController<List<TaskItem>> _controller =
    StreamController<List<TaskItem>>.broadcast();

/// 【订阅者的义务】…**订了就要 `cancel`**…**退订由订阅方做，关闭由流的所有者做，
/// 而 store 不拥有页面**。
Stream<List<TaskItem>> get tasks => _controller.stream;
```

**为什么这次交付是「文档 + 判决用例」而不是「加 close」**：任务书明确禁止「硬加一个没人调的
close 方法」，而唯一合法调用方（页面 `dispose`）在授权清单外；且审查自己的 §3.4 也把它定性为
「P1 但**不是**内存泄漏，是『缺 dispose 契约』」。本次把那份契约补齐、写死依据、并让它可回归。

---

## 二、P1-② `collab_roles_state.dart` `late` 幽灵初始化 —— 修复

### 2.1 问题复核

```dart
late Future<void> _loadFuture = _doLoad();   // 旧
void load() { …; _loadFuture = _doLoad(); }
Future<void> get loadCompleted => _loadFuture;
```

Dart 的 `late` 字段初始化器在**首次读取**时才求值。旧实现下，只要有人**读一下** `loadCompleted`
（探查代码、断言、调试打印都算），就会**悄悄发起一次 `listCollabRoles` RPC**，
且在三态字段上表现为「`loadState` 自己从 loading 变成 ready」——典型幽灵行为。

### 2.2 全仓时序核对：既有读点**全部**是「先 load 后 await」，无一依赖旧行为

| 读点 | 形态 | 命中数 |
|---|---|---|
| `lib/pages/collab_roles.dart:101-102` | `_store.load(); await _store.loadCompleted;` | 1 |
| `lib/pages/collab_bridge.dart:456-457` | `store.load(); await store.loadCompleted;` | 1 |
| `test/pages/collab_roles_state_test.dart` | `store.load(); await store.loadCompleted;` | 13 |
| `test/pages/collab_role_editor_page_test.dart:85-86` | `store.load(); await store.loadCompleted;` | 1 |

⇒ 旧的那条 `late` 初始化器**在全仓从未被触发过**（它只在「忘了调 `load`」时才现身）。
**H3 波的 collab 共享 store（B3 波成果）时序因此逐条不变**：`BridgeCollabRolesSource`
（`collab_bridge.dart:452` 的 `static final CollabRolesStore store`）仍严格是
「`load()` 一次 → `await loadCompleted` 一次 → RPC 恰好一次」。

### 2.3 改动前后对比

改动前：

```dart
  /// 【为什么 fire-and-forget（返回 void 而不是 Future）】…测试要等它，直接 await 内部
  /// 的 [_load] 路径——不，测试要确定性：见 [_loadFuture]（进行中的加载只
  /// 保留一个，重复调用返回同一个 Future，测试可 await）。
  void load() {
    _loadState = CollabRolesLoadState.loading;
    _loadError = null;
    // 记录在飞加载：并发调用共享同一个 Future（Kotlin 侧由 LaunchedEffect 的
    // 单次性天然保证，Dart 侧显式记录）。
    _loadFuture = _doLoad();
  }

  late Future<void> _loadFuture = _doLoad();

  Future<void> get loadCompleted => _loadFuture;
```

改动后（**唯一逻辑行变化就是那一行字段声明**）：

```dart
  void load() {
    _loadState = CollabRolesLoadState.loading;
    _loadError = null;
    // 记录在飞加载（每次调用各起一个；谁最近发起，[loadCompleted] 就跟谁）。
    _loadFuture = _doLoad();
  }

  /// 【为什么初值是「已完成的空 Future」，而不是 late 字段 + 初始化器】
  /// `late` 字段的初始化器在**首次读取**时才求值，所以「late 声明 + 初始化器调
  /// `_doLoad()`」等于把一次 `listCollabRoles` RPC 藏在 getter 后面…
  /// 改成**显式意图**：字段本身零副作用（初值是一个已完成的占位 Future，读它什么
  /// 也不会发生），发起加载只有 [load] 这一个动作，要等收敛就 `await loadCompleted`。
  Future<void> _loadFuture = Future<void>.value();

  Future<void> get loadCompleted => _loadFuture;
```

**为什么选「已完成占位 Future」而不是审查建议的「构造即 load」**：
审查 §3.5 的第一方案（构造器里 `load()`）会**新增**一次所有构造点的 RPC——
`BridgeCollabRolesSource.store` 是 `static final`（类加载即构造），`collab_bridge.dart:456`
紧跟着又 `load()` 一次 ⇒ `collab.dart:315` 每次读角色都会变成**两次** `listCollabRoles`。
那是实打实的时序回归。零副作用的字段是唯一不动既有路径的方案。

**顺手改正的一句谎言注释**（属本条范围内、且与 H5b「零谎言注释」纪律一致）：
「进行中的加载只保留一个，**重复调用返回同一个 Future**」是假的——`load()` 每次都新建 Future
（既有 `重试：宿主从「未注册」换成「正常应答」后 load 转 READY` 用例连 await 两次两个不同 Future）。
已改成「每次调用各起一个 Future，`loadCompleted` 指向最近那次；不合并、不去重」，并新增判决用例钉住。
同段里指向不存在符号 `_load` 的 `[_load]` 悬空引用一并去掉。

---

## 三、判决用例（新增 5 条，全部是加强，零弱化）

### 3.1 `flutter/test/pages/task_center_data_test.dart` —— 新增 1 个 group / 3 条

| 用例 | 钉什么 |
|---|---|
| `无人监听时的连续变更不留存：新订阅者只收后续那一次，不补旧账` | 未 close 的 controller **不按事件条数**堆积：连打 50 次变更后再订阅，只收到**后续那 1 条**；同时 `snapshot` 仍是 51 条（真值一条不丢） |
| `退订后不再收通知；后续订阅者照常收（可释放的是订阅，不是历史）` | 可释放的是**订阅**不是历史：退订后不再收，新订阅者照常收且不补退订期间的变化 |
| `两个实例互不串味：通知流随实例走，不是隐式 app 级单例` | 钉住「生命周期 = 实例」这条前提；`reason` 里写明 B4 若提单例**这条必须先改**（届时必须同时补 owner 侧退订与 close 契约，不许悄悄改绿） |

无 `dart format`，沿用文件既有的 CRLF on-disk 形态与写法（`import 'dart:async';` 为 `StreamSubscription` 所需）。

### 3.2 `flutter/test/pages/collab_roles_state_test.dart` —— 新增 1 个 group / 2 条

| 用例 | 钉什么 |
|---|---|
| `只读 loadCompleted / 其余只读面 = 零 RPC；load() 才恰好发一次` | **本条核心断言**：构造后零 RPC；`await store.loadCompleted` 两次 + 读 `loadState`/`loadError`/`roles`/`findById` 全部**零 RPC** 且 `loadState` 仍停在构造初值 `loading`；随后 `load()` 才**恰好一次** `listCollabRoles` 并转 `ready` |
| `每次 load() 各起一次加载；loadCompleted 跟随最近那次（不去重）` | 时序护栏：两次 `load()` = 两次 RPC、两个不同 Future、第二次收敛后 roles 是 `r2`；重复 `await` 不产生新加载 |

**既有断言一条未动**：两个测试文件的 `test(` 声明数只增不减，无一条 `expect` 被改写或放松。

---

## 四、验证证据（原文）

### 4.1 基线（改动前，HEAD `ede6175` 的 flutter 树，`git status --short` 无 flutter 条目）

```
$ cd G:\项目1\pocketagent\flutter; flutter test --reporter expanded
00:15 +949: All tests passed!
```

### 4.2 改后全量

```
$ cd G:\项目1\pocketagent\flutter; flutter test
00:14 +953: ... 设置页可进入新手教程并横向切换
00:14 +954: All tests passed!
```

**949 → 954（+5，一条未跌）**。

### 4.3 `flutter analyze`（junction ASCII 路径 `G:\pa_flx1`，跑完已删）

```
$ cd G:\pa_flx1; flutter analyze
Analyzing pa_flx1...
No issues found! (ran in 3.9s)
```

（中途曾有 1 条 `unintended_html_in_doc_comment`——我新写的注释里 `Future<void>` 的尖括号；
已改为「late 字段 + 初始化器」措辞消除，复跑即零问题。中文路径 analyze 的坑按预案走 junction。）

### 4.4 变异验证（各一次；改回后 `git diff` 零残留）

**M1（P1-①）**：把 `_controller` 改成**进程级 static**（= B4「提 app 级单例」的形态）

```
$ flutter test test/pages/task_center_data_test.dart
00:00 +24: TaskCenterStore：通知流的生命周期契约（无 close 的依据） 两个实例互不串味：通知流随实例走，不是隐式 app 级单例 [E]
  Expected: empty
    Actual: [[TaskItem:TaskItem(msg:x, TaskKind.message, TaskStatus.running, read=false)]]
Failing tests:
  .../task_center_data_test.dart: ... 两个实例互不串味：通知流随实例走，不是隐式 app 级单例
```

⇒ **恰好 1 条红**，正是钉「生命周期 = 实例」那条；同文件另 24 条全绿。
（另试了一次「M1b：换成单订阅 controller」，红 5 条（含 2 条既有）——探针太粗、不可作
「恰好红」证据，已弃用并回退，只保留 M1 为正式证据。）

**M2（P1-②）**：把字段改回原状 `late Future<void> _loadFuture = _doLoad();`

```
$ flutter test test/pages/collab_roles_state_test.dart
00:00 +6 -1: loadCompleted：查询不带副作用 只读 loadCompleted / 其余只读面 = 零 RPC；load() 才恰好发一次 [E]
  Expected: empty
    Actual: ['listCollabRoles']
Failing tests:
  .../collab_roles_state_test.dart: loadCompleted：查询不带副作用 只读 loadCompleted / 其余只读面 = 零 RPC；load() 才恰好发一次
```

⇒ **恰好 1 条红**，红点正是核心断言「只读查询不得触发 RPC」。

**回退核对**：`git diff --stat -- flutter` 只剩 4 个授权文件的真实改动；
`Select-String 'MUTATION-PROBE' 两文件` = **0 命中**（探针标记零残留）。

### 4.5 换行符 / 噪声核对

`git ls-files --eol`：4 个文件 `i/lf` 形态未变；`task_center_data.dart` / `collab_roles_state.dart`
/ `collab_roles_state_test.dart` 保持 on-disk LF，`task_center_data_test.dart` 保持 on-disk CRLF
（编辑工具按文件既有形态落盘；`text=auto` 归一化后 git 判为无 EOL 噪声）。
`git diff -- flutter/lib/pages/` 逐行核对：**只有新增的注释块 + 新增测试，无任何全文件重排**。

---

## 五、行为变更登记（供主会话裁决）

| # | 事项 | 性质 | 我的处置 |
|---|---|---|---|
| B-1 | `CollabRolesStore.loadCompleted` 在「**从未调过 `load()`**」时，语义从「读它会触发一次加载、返回未完成的 Future」变成「返回**已完成**的 Future、零 RPC」。 | **行为变更**（有意，正是本条要修的幽灵副作用） | 已改实现 + 新增判决用例钉住新语义。**既有测试无一依赖旧行为**（全仓 16 个读点全是「先 load 后 await」，§2.2） |
| B-2 | `loadCompleted` 的**名字**未改（审查 §1.3 建议改 `whenLoadSettled`）——任务书要求「保持查询语义不变」 | 刻意不做 | 未动 |
| B-3 | 审查 §3.5 第一方案「构造即 `load()`」被**否决**：会让 `collab_bridge.dart:452` 的 `static final` store 多发一次 RPC，改变 collab 角色加载时序 | 设计裁决 | 已在本文件 §2.3 论证 |
| B-4 | `CollabRolesStore` 类头「生产由根装配持一个共享实例」与 `BridgeCollabRolesSource.store` 的 `static final` **确实是 app 级共享**（对比 ① 的任务中心 store 不是） | 观察项，非本条范围 | 未动 |
| H-1 | **移交（授权外）**：`task_center.dart:121` / `settings_downloads.dart:251` 的 `listen` 从不 `cancel`。今天无害（store 随页面回收）；**B4 提任务中心 store 为 app 级单例前必须先修**（存 `StreamSubscription` 字段 + `dispose` 里 `cancel`）。建议届时同批改这两个页面 + 重跑 `task_center_page_test`(19) / `settings_downloads_test`(14) | 移交 | 未动（非授权文件） |
| H-2 | 审查 §3.4 建议「现在给 `TaskCenterStore` 加 `dispose()`（即使暂无调用方）」 | **未采纳** | 任务书明令「不许硬加一个没人调的 close 方法」，且 Kotlin 对位无 close；改为补契约文档 + 判决用例（§一） |

---

## 六、文件清单

| 文件 | 变化 | 性质 |
|---|---|---|
| `flutter/lib/pages/task_center_data.dart` | +31 行 | **零逻辑行变化**：类头补【生命周期 = 实例；刻意不提供 close】+【唯一的增长向量】两段（并改正原「方法面不动」错注释）；`_controller` / `tasks` 补文档 |
| `flutter/lib/pages/collab_roles_state.dart` | +21 / −5 行 | **1 行逻辑变化**：`_loadFuture` 由 `late … = _doLoad()` 改 `Future<void>.value()`；其余为文档（新增「为什么初值是空 Future / 时序不变的依据」，改正 `load()` 上那句假注释） |
| `flutter/test/pages/task_center_data_test.dart` | +76 行 | 新增 1 group / 3 条判决用例 + `import 'dart:async';`；**既有断言零改动** |
| `flutter/test/pages/collab_roles_state_test.dart` | +72 行 | 新增 1 group / 2 条判决用例；**既有断言零改动** |

`git diff --stat -- flutter`：`task_center_data.dart` 37±、`collab_roles_state.dart` 29±、
`collab_roles_state_test.dart` 72+、`task_center_data_test.dart` 76+。

**并发提示**：`git status --short` 里 `flutter/docs/channel-contract.md` 也显示 ` M`，
但**不是本轮改的**（mtime 3:19:50，落在本会话两个编辑之间；内容是 B4 通道契约文档更新，
与本任务零重叠）。本轮改动仅上表 4 个文件。**未 git add / commit / push。**

---

## 七、红线自查

- [x] 未替用户做审批决定：P1-① 的任务书前提（已提单例）经核实不成立，如实报告并给出证据裁决，未硬造 close 方法凑数。
- [x] 未伪造数据面：`Object identity / repeated await / 50 次变更` 等都是实测断言，无假绿。
- [x] 测试水位 949 → 954，**未跌**；新增 5 条判决用例；**零弱化既有断言**。
- [x] 未跑 `dart format`；无格式化式重排（`git diff` 逐行核对）。
- [x] 换行符保持仓库既有形态。
- [x] analyze 零问题（junction 路径，跑完已删 `G:\pa_flx1`）。
- [x] 变异验证各一次且「恰好红」；回退后零残留。
- [x] **未 git commit / add / push**；4 个授权文件全部停留在工作区未暂存。

## 八、一句话结论

P1-② 已实修（`late` 幽灵初始化 → 零副作用显式意图，加载时序逐条不变，新增 2 条判决用例，
变异验证红点=「只读查询零 RPC」）；P1-① 的前提经核实不成立——任务中心 store 今天仍是页面级实例、
通知流按活跃订阅而非事件条数占内存、Kotlin 对位 `MutableStateFlow` 结构上无 close，
故按「无需关闭」分支交证据：改正了那句「提 app 级方法面不动」的错注释、把生命周期契约与唯一增长向量
写进类头，并新增 3 条判决用例把「生命周期 = 实例」钉成可回归；真要修的那一行（页面 `cancel` 订阅）
落在授权外，登记移交 H-1。**水位 949 → 954，analyze 零问题，未提交。**
