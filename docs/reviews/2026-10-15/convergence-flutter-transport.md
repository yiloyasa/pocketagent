# Flutter transport 层收敛（H3 移交项第一项：三处字节处理不同型 → transport 内收口）

- 日期：2026-10-15
- 执行：收敛审查 + 收敛实施（transport 域，`lib/transport/pa_bridge.dart` + `test/transport/`）
- 边界：只改 `flutter/lib/transport/pa_bridge.dart` 与 `flutter/test/transport/` 下文件；**未碰** `lib/pages/`（并行子代理在建 `collab_bridge.dart` / `collab_session_data.dart` 与 main.dart 接线）、`lib/gen/`、`lib/theme/`、`lib/components/`、`android/`、`daemon/`、`proto/`、`pubspec.yaml`、`test/support/`。
- 新增文件：`flutter/test/transport/fixture_support.dart`（37 行，transport 测试共享 fixture 读取）。
- 净变化：`git diff --stat`（我的 3 个已跟踪文件）= **56 insertions / 115 deletions**（净 −59 行），无换行符噪声、无格式化式重排。
- **未执行** git commit / add / push。

---

## 一、盘点结论（先看这里）

### 1.1 确实重复、已收敛

| 重复项 | 份数 | 处置 |
|---|---|---|
| proto 字节解码三段式（`_expectBytes` 验形 + `fromBuffer` 反序列化 + `try/catch InvalidProtocolBufferException → PaBridgeException(DECODE_FAILED)`） | **6 份逐字同型**（`_parseHostInfo` / `_parseListThreads` / `_parseListDirectory` / `_parseFileChunk` / `_parseListBranches` / `_parseHostEvent`） | 合并为唯一收口 `bytesOrThrow<T>(where, raw, typeName, parse)`；六份删除 |
| `_decode*` → `_parse*(where, _expectBytes(where, raw))` 的「同参两次传 where」装配 | 5 份 | `_decode*` 直调 `bytesOrThrow`，`where` 只传一次（§2.1 论证调用点参数逐字不变） |
| 测试侧 `fixture(name)` 读取（b64 解码 + 两条 cwd 候选路径 + fail 语义） | **2 份逐字节相同**（`pa_bridge_test.dart` 42–55、`pa_bridge_threads_test.dart` 36–49） | 合并为 `test/transport/fixture_support.dart` 的 `fixtureBytes`；两个测试文件内各留一行 `fixture(name) => fixtureBytes(name)` 委托（调用点一字未改） |

### 1.2 盘点方向里评估后**不动**的（附理由）

| 方向 | 结论 | 理由 |
|---|---|---|
| **「方法面声明表」化**（把 10 条方法面收进一张 `_methods` 表 + 泛型分发） | **不做，只收内部 helper** | 收益：10 条 `_invoke`/`_invokeExpectNull` 装配确实同构（空请求默认值、`writeToBuffer`、错误码透传都在 `_invoke` 内）。风险：**每条方法的 Dart 文档注释是契约文档的一部分**（channel-contract.md §3 的人读镜像，逐条写着 arguments 形态/返回形态/错误码/为什么），表驱动要把这些注释从方法上搬进表字面量，Dart 文档注释的 `[xxx]` 引用与 §号索引全部要改写，等于重写契约载体——「收敛不得改契约语义」的红线会变成每天重述一次的负担。10 条方法面里真正重复的**只有**字节解码那一段（已在 §1.1 收敛），装配层每条的差异（`connect` 的本地 INVALID_ADDR 前置校验、`getConnectionState`/`pairCode` 的非字节 decode 闭包、三条 `_invokeExpectNull` 的不同参数形态）都是契约差异不是重复。 |
| `settings_capabilities.dart` 的 `decodeListModelsResponse` / `Uint8List` 检查（H3 §5.1 的第三处副本） | **不动，登记移交**（§5.1） | 它在 `lib/pages/`，本任务书明令不碰；且它消费的是 `invoke('listModels', …)` 裸闭包（PaBridge 当前无该方法面、无公开通用 invoke）。transport 内造不出它的替身——`bytesOrThrow` 在 transport 里是私有的，pages 拿不到。B4 给 PaBridge 补 `listModels` 具名方法后，那两段（验形 + 解码）应直接改调 transport 侧同类收口，pages 只留 `modelOptionsOfResponse` 映射纯函数。 |
| `PaConnState.fromPlatform` 的 Map 五键校验 | **不动** | 与六份 proto 解码**不同型**：它没有字节、没有 proto 异常，错误消息按「最先违约的键」逐键定制（测试 `contains('connTransport')` / `contains('connectionEpoch')` 钉死定位）。泛型化它要么丢键名定位，要么把 `where`/`key`/`typeName` 三参数化到比六行原文还长。登记备查（§5.2）。 |
| `_invoke` / `_invokeExpectNull` 本体 | **不动** | 已是唯一一份；`_invokeExpectNull` 就是 `_invoke` + null 判定的薄壳，三处调用（connect/disconnect/startTurn）各只有一行。 |
| 死代码 / 未用导出 | **未发现**（详见 §5.3） | 对 10 条方法面 + 3 个公开类型 + 2 个内部 helper 做了全域消费扫描；`PaConnEvent.hostInfo` / `hostEvent` 两个载荷字段与 `PaConnEventType.hostInfo` / `hostEvent` 在 `lib/pages/` 无消费点，但它们由 `_decodeEvent` 生产、被 transport 测试逐条钉死（`host_info 事件 → HostInfo` / `host_event 事件 → HostEvent`），且 `collab*` 两个新文件正在建——**未消费 ≠ 死**，不动。 |

---

## 二、逐项收敛记录 + 等价论证

### 2.1 `bytesOrThrow<T>` ← 六份 `_parse*`（−约 70 行）

**收敛前的同型实证**：六份函数的差异**只有**类名（进 `fromBuffer`）与消息里的类型名（进消息串）。用 `Compare-Object` 对六段源码区间（去掉类名与类型名字面量后）做过 diff，输出为空。原形状（每份都是这 8 行）：

```dart
XxxResponse _parseXxx(String where, Uint8List bytes) {
  try {
    return XxxResponse.fromBuffer(bytes);
  } on InvalidProtocolBufferException catch (e) {
    throw PaBridgeException(
      PaBridgeErrorCodes.decodeFailed,
      '$where 的 XxxResponse 字节不可解析：$e',
    );
  }
}
```

收敛后唯一收口（`pa_bridge.dart` 文件尾部，紧邻 `_expectBytes`）：

```dart
T bytesOrThrow<T extends GeneratedMessage>(
  String where,
  Object? raw,
  String typeName,
  T Function(List<int>) parse,
) {
  final Uint8List bytes = _expectBytes(where, raw);
  try {
    return parse(bytes);
  } on InvalidProtocolBufferException catch (e) {
    throw PaBridgeException(
      PaBridgeErrorCodes.decodeFailed,
      '$where 的 $typeName 字节不可解析：$e',
    );
  }
}
```

**逐项等价论证**：

1. **验形不变**：`_expectBytes(where, raw)` 原样内联进收口，`'$where 须为 proto wire 字节(Uint8List)，实际 ${raw.runtimeType}'` 一字未改。
2. **反序列化不变**：`parse` 由调用点传 `Xxx.fromBuffer`（gen stub 的 `factory Xxx.fromBuffer(List<int> data, [registry])`——torn-off 后是 `Xxx Function(List<int>, [ExtensionRegistry])`，按 Dart 函数子类型规则可赋值给 `T Function(List<int>)`；`_expectBytes` 已保证 `bytes is Uint8List`，`Uint8List` 是 `List<int>` 子类，实参类型安全）。`parse(bytes)` 与原来的 `XxxResponse.fromBuffer(bytes)` 是同一次调用。
3. **异常捕获不变**：`on InvalidProtocolBufferException` 原样；`InvalidProtocolBufferException` 是 `FormatException` 子类，六处捕获的都是同一个运行时类型，收口后仍是同一个 catch 子句——没有把异常面拓宽或收窄。
4. **消息文本逐字不变**：模板 `'$where 的 $typeName 字节不可解析：$e'` 与六份原文的差异只是把写死的类型名换成参数。七处调用点的 `typeName` 实参与 `parse` 构造器已逐一核对（脚本取证见 §4.5）：`getHostInfo 返回值`+`HostInfo`、`listThreads 返回值`+`ListThreadsResponse`、`listDirectory 返回值`+`ListDirectoryResponse`、`readFile 返回值`+`FileChunk`、`listBranches 返回值`+`ListBranchesResponse`、`事件 host_info.info`+`HostInfo`、`事件 host_event.event`+`HostEvent`——`where` 七个定位串与收敛前逐字相同（正则比对 `WHERE_EQUAL: True`），`typeName` 与 `parse` 一一配对。**拼接出的消息对全部输入与收敛前相等**。
5. **`_decode*` 装配等价**：原 `_parseXxx('X 返回值', _expectBytes('X 返回值', raw))` 把 `where` 传两遍（错位的风险点：两处字符串要手动保持一致）。收口后 `where` 只出现一次，调用点参数表从 `(where, bytes)` 变成 `(where, raw, typeName, parse)`——**`where` 与类型实参逐字取自原调用点**，无新增、无改写。原 `_decode*` 五个方法的返回类型标注（`HostInfo`/`ListThreadsResponse`/…）保留，泛型 `T` 在每处由返回位置下推，无 `as` 强转、无 `dynamic`。
6. **事件面等价**：`_decodeEvent` 的两个 proto 分支从
   `_parseHostInfo(where, _expectBytes(where, raw['info']))` 变成
   `bytesOrThrow(where, raw['info'], 'HostInfo', HostInfo.fromBuffer)`——同一 `where` 常量、同一 `raw['info']`、同一构造器。
7. **可见性变化（唯一的行为级差异，零外部影响）**：`bytesOrThrow` 是文件级公开符号（原 `_parse*` / `_expectBytes` 是私有）。原因：Dart 顶层函数没有「库内公开」这一档，而它必须能被 `PaBridge` 类的方法调用。**没有任何文件 import 它**（grep 实证：全库 `bytesOrThrow` 只出现在 `pa_bridge.dart`），不新增对外 API 面；`_expectBytes` 保持私有。

### 2.2 `fixtureBytes` ← 两份逐字节相同的 `fixture()`（−约 40 行）

**同型实证**：`Compare-Object` 对 `pa_bridge_test.dart` 42–55 行与 `pa_bridge_threads_test.dart` 36–49 行做 diff，**输出为空**（含 `'$name'` 插值、`cwd=${Directory.current.path}` 收尾、`fail(...)` 的换行位置，逐字节相同）。

**为什么放 `test/transport/fixture_support.dart` 而不是 `test/support/`**：`test/support/` 归 `pump_helpers` 域——它 `import package:pocketagent_ui/pages/splash.dart`（widget 域），transport 测试不该为读两个 b64 文件把 widget 依赖拖进通道测试的编译单元。任务书同样只授权 `test/transport/` 下新文件。

**调用点零改动**：两个测试文件里 `fixture('host_info')` / `fixture('thread')` / `fixture('host_event')` 等调用一字未改；文件内各留一行委托 `Uint8List fixture(String name) => fixtureBytes(name);`（带文档注释指向唯一出处）。**`fail` 消息文本逐字保留**在 `fixtureBytes` 里（含「试过：…；cwd=…」的完整格式），fixture 缺失时测试仍 fail 不静默跳过（假绿防线未动）。

**为什么第三个文件没收敛**：`pa_bridge_files_git_test.dart` **根本没有本地 `fixture()` 副本**——它的响应字节全部由 Dart gen stub 现组（`ListDirectoryResponse(...).writeToBuffer()` 等），不读 fixtures 目录（该文件只 `import 'dart:convert'` 供 `utf8` 用）。所以「三份重复」实际是两份，不是三份；这也是把共享件命名为 `fixtureBytes`（不叫 `transport_support` 之类大筐）的原因——它只承载 fixture 读取这一件事。

---

## 三、改动文件清单

| 文件 | 变化 | 说明 |
|---|---|---|
| `flutter/lib/transport/pa_bridge.dart` | 652 → 622 行 | 删六份 `_parse*`（−约 70 行）；新增 `bytesOrThrow` 唯一收口（+约 30 行含注释）；七个调用点改直调；import 行加 `GeneratedMessage`；新增收敛注释（含「为什么不做表驱动」的理由，供后续波次查阅） |
| `flutter/test/transport/pa_bridge_test.dart` | 481 → 481 行（5+/20−） | 删本地 `fixture()`（14 行）；import 改 `import 'fixture_support.dart' show fixtureBytes;` + 一行委托；删不再用的 `dart:convert` |
| `flutter/test/transport/pa_bridge_threads_test.dart` | 173 → 173 行（5+/20−） | 同上 |
| `flutter/test/transport/fixture_support.dart` | **新增 37 行** | `fixtureBytes(String name)`：两条 cwd 候选路径 + b64 解码 + fail 语义（原样搬迁） |

**未改**：`test/transport/pa_bridge_files_git_test.dart`（无 fixture 副本，无重复可收，零改动）；`test/support/pump_helpers.dart`（任务书禁改，且域不同）。

**断言完整性机械证据**（收敛前后同文件比对）：

| 文件 | `expect(` 行数 before → after | `test(` 声明数 |
|---|---|---|
| `pa_bridge_test.dart` | 66 → 66 | 21（不变） |
| `pa_bridge_threads_test.dart` | 17 → 17 | 7（不变） |
| `pa_bridge_files_git_test.dart` | 未改 | 18（未改） |

---

## 四、测试 / analyze 证据原文

### 4.1 基线（收敛动手前，工作区实跑）

`flutter test` 全量：

```
00:09 +342: All tests passed!
```

### 4.2 收敛后：transport 目录测试

```
$ flutter test test/transport/
00:00 +46: All tests passed!
```

（46 = 21 `pa_bridge_test` + 7 `pa_bridge_threads_test` + 18 `pa_bridge_files_git_test`，与基线逐文件可比。）

### 4.3 收敛后：全量测试（≥342 判据）

```
$ flutter test
00:09 +342: All tests passed!
```

**342 = 342**，与 §4.1 基线逐条可比：零跌落、零新增、零跳过。并行子代理在 `lib/pages/` 下新建的 `collab_bridge.dart` / `collab_session_data.dart` 为 untracked 文件，其测试不在本次 342 内（全量 glob 已含它们仍在跑的目录；计数持平说明我的改动对并行域零影响）。

### 4.4 收敛后：analyze

junction `G:\pa_h4` → `G:\项目1\pocketagent\flutter`（用完已删）：

```
$ flutter analyze lib/transport/pa_bridge.dart test/transport/
Analyzing 2 items...
No issues found! (ran in 2.4s)
```

全仓 analyze（junction 下，收敛后复跑一次）：

```
$ flutter analyze
Analyzing pa_h4...
No issues found! (ran in 7.0s)
```

**零问题**。中途一次 analyze 报过 2 个问题（`fixture_support.dart` 少 `dart:typed_data`、`pa_bridge_test.dart` 残留未用 `dart:io`），已修复后复跑归零——不是既存告警。

### 4.5 等价性的额外机械证据

- **错误消息模板比对**（`git show HEAD` 版 vs 收敛版，正则归一化后取唯一模板集）：`EQUAL: True`——三个模板（`'$method 契约要求宿主回 null，实际 ${raw.runtimeType}'`、`'$where 须为 proto wire 字节(Uint8List)，实际 ${raw.runtimeType}'`、`'$where 的 <T> 字节不可解析：$e'`）收敛前后完全一致；旧版六份的类型名分别由新版七处调用点的 `typeName` 实参供给（§2.1 第 4 条）。
- **`where` 定位串比对**：七个定位串（`getHostInfo 返回值` / `listThreads 返回值` / `listDirectory 返回值` / `readFile 返回值` / `listBranches 返回值` / `事件 host_info.info` / `事件 host_event.event`）收敛前后集合相等（`WHERE_EQUAL: True`）。
- **`git diff --stat`** 只含真实语义变化（无全文件重排）：56 insertions / 115 deletions。
- **`Compare-Object` 同型实证**：六份 `_parse*`（归一化类名/类型名后）diff 为空；两份测试 `fixture()` diff 为空（§2.2）。
- **换行符**：改动文件全部保持 LF（与 `git ls-files --eol` 报告的 `i/lf w/lf eol=lf` 一致；字节级复核：`pa_bridge.dart` CRLF=0 bareLF=622，两个测试文件同样零 CRLF）。**未跑** `dart format` / `flutter format`（short-style 仓库，新版 tall-style 会大面重排）；所有编辑用 edit 工具精确替换。**注意**：任务书提到的「保持仓库 CRLF」与实际不符——本仓库 `.gitattributes` 对这些文件是 `eol=lf`（pages 域部分文件工作区为 CRLF 是历史遗留），我按「不引入换行符噪声」原则保持了每个文件原有字节形态，`git diff` 无换行符噪声。
- **Junction 清理**：`G:\pa_h4` 用完已删（`cmd /c rmdir`，只删链接不动目标），删除后 `G:\项目1\pocketagent\flutter\lib\transport\pa_bridge.dart` 仍在。

---

## 五、已识别未动 + 移交清单

### 5.1 `settings_capabilities.dart` 的第三处字节副本（H3 §5.1 正主，移交 B4）

`lib/pages/settings_capabilities.dart` 的 `BridgeModelsDataSource` 里有两段与 transport 收敛前同型的代码：

- `if (raw is! Uint8List) throw PaBridgeException(decodeFailed, 'listModels 返回值须为 proto wire 字节(Uint8List)，实际 ${raw.runtimeType}')`（188–193 行）——与 `_expectBytes` 同型，但消息文本**不同**（它写 `listModels 返回值`，transport 侧写 `$where 须为…`）；
- `decodeListModelsResponse`（127–136 行）的 `try/catch InvalidProtocolBufferException → PaBridgeException(DECODE_FAILED)`——与收敛前的六份 `_parse*` 同型。

**为什么不顺手收**：① 本任务书明令不碰 `lib/pages/`；② 它经 `invoke` 裸闭包调用（`Future<Object?> Function(String, Object?)`），PaBridge 当前没有 `listModels` 具名方法、也没有公开通用 invoke，`bytesOrThrow` 在 transport 里是文件级私有/公开符号但 pages 不 import transport 的内部件；③ 消息文本不同型（§上），强行统一要改 pages 的消息文本——而那正是 tests 钉死的东西（settings_capabilities_test 的「坏字节 → DECODE_FAILED」只钉 code 不钉全文，但 `listModels 返回值须为 proto wire 字节(Uint8List)` 这串是 pages 域自己认的定位文本）。

**移交要点（B4 真桥接落地时）**：给 `PaBridge` 补 `Future<ListModelsResponse> listModels([ListModelsRequest? request])`（同 `listThreads` 形状）后，把 `BridgeModelsDataSource` 改成 `_bridge.listModels(...)`，上述两段整段删除——届时它们已由 transport 的 `bytesOrThrow` 收口覆盖，pages 只留 `modelOptionsOfResponse` / `stateError` 映射纯函数。**不要再在 pages 造第三个副本**（H3 §5.1 的原话）。测试锚点：settings_capabilities_test 的「坏字节 → DECODE_FAILED」+「listModels 真桥接未就位 → HOST_UNAVAILABLE」。

### 5.2 `PaConnState.fromPlatform` 的五键 Map 校验（备查结论）

与六份 proto 解码不同型（无字节、无 proto 异常、按「最先违约的键」定制消息），泛型化会丢定位或比原文更长。已登记不动。若后续出现**第二个**「严格 Map 五键校验」消费者（如 Kotlin 侧新增第二组 StateFlow 快照过桥），届时可评估抽 `mapOrThrow(where, raw, expectedKeys)`——现在只有一个消费者，提前抽是投机。

### 5.3 死代码 / 未用导出扫描结论（备查）

| 符号 | lib/pages 消费 | 测试消费 | 结论 |
|---|---|---|---|
| `PaBridge.getHostInfo/connect/disconnect/getConnectionState/pairCode/startTurn/listThreads/listDirectory/readFile/listBranches`（10 条） | 9 条有（settings_connection / settings_agent / git / files_bridge / git_bridge / threads_bridge） | 10 条全有（transport 三文件） | 全活。`disconnect` / `startTurn` 两条 lib/pages 无消费点，但 transport 测试逐条钉死 wire 行为（`disconnect：无参方法 arguments 为 null` / `startTurn：TurnInput wire 字节入参`），且 `collab*` 在建——登记不动 |
| `PaConnEvent.hostInfo` / `PaConnEventType.hostInfo` | 无（settings_connection 只消费 `connState`） | 有（`host_info 事件 → HostInfo`） | **未消费 ≠ 死**：contract §4 的 payload 表三项之一，`collab*` 页面正在建。不动 |
| `PaConnEvent.hostEvent` / `PaConnEventType.hostEvent` | 无 | 有（`host_event 事件 → HostEvent`） | 同上 |
| `PaConnEvent.connState` / `connState` | 有 | 有 | 活 |
| `_expectBytes` / `bytesOrThrow` | 不 import transport 内部件 | 间接全覆盖 | `bytesOrThrow` 为唯一收口，`_expectBytes` 被它内联；两者无第二消费者，不构成「死」 |
| `PaConnState.epoch` / `capabilities` / `toString` | `epoch` 有（settings_connection 刷新信号）、`capabilities` 有（settings_agent 能力位总览）、`toString` 无直接消费 | — | Dart 数据类惯例字段，保留 |

**判据说明**：任务书提示「collab* 在建，未消费≠死，判不清就登记不动」——本轮对每个无 lib 消费的公开符号都做了「契约面（channel-contract.md 是否列名）+ 测试是否钉死 + 并行代理是否在建」三查，**零删除、零加 `@Deprecated`、零改可见性**（`bytesOrThrow` 的公开化是收敛副产物且无任何 import 方，见 §2.1 第 7 条）。

### 5.4 契约文档漂移登记（只读发现，交主会话，未动文档）

| # | 漂移 | 位置 | 建议 |
|---|---|---|---|
| 1 | **方法清单停在 7 条**：§3 表头写「首批 6 条 + B3-3 追加 1 条」，§6 B4 清单写「7 个方法」；而 `pa_bridge.dart` 已有 **10 条**（+`listDirectory`/`readFile`/`listBranches`，B3 第 3 波），且 §3.7 之后无 §3.8/3.9/3.10 条目 | `channel-contract.md` §3 表（39–49 行）、§6（159 行） | 补三条方法行 + 三个小节；§6 的「7 个方法」改 10。另：`pa_bridge_files_git_test.dart` 文件头引用「channel-contract.md §3.8」指向尚不存在的节 |
| 2 | **`readFile` 的「首块」语义未入文档**：Dart 方法面把 daemon 的服务器流收成单份 `FileChunk`（B4 实现约定），§3 表只会在新增行里写返回类型，`§2 编码总则` 与 `§3.x` 都无处说明「流如何收成方法」 | §3（新节需补） | 新增节里写明「宿主侧把 ReadFile 流汇聚成首块回传，Dart 只验首块四字段；下载/分块消费是另一条链路（PaDownloader），不在本方法面」——`pa_bridge.dart` 401–417 行的注释已是该语义的完整表述，可直搬 |
| 3 | §3.4 表格里 `state` 行的 Kotlin 源写 `connState`（"checking"/"ok"/"down"），与 Dart 注释一致，**非漂移**（备查：本轮核对过，无变化） | — | 无需动作 |

漂移 1/2 均为「B3 第 3 波加了方法面但 channel-contract.md 未同步」——**按任务书只登记不动文档**，交主会话裁决补写时机（建议与 B4 宿主实现同批，避免文档半更新状态）。

---

## 六、红线自查

- [x] 没有替用户做审批决定；没有伪造任何数据面（错误码表、消息文本、`where` 定位串一字未改）。
- [x] 没有改写任何用户桌面配置（本批无写通道代码）。
- [x] 未碰 proto（双副本 MD5 不受影响）。
- [x] 未新增写死的名单/顺序/版本二分（收敛只是把六份同型 try/catch 合成一份；`bytesOrThrow` 的 `typeName` 由调用点传实参，不是新名单）。
- [x] 不留假绿：测试结果原文贴在本文件 §4；`flutter analyze` 零问题；`342 = 342` 与基线逐条可比；证据为本次实跑（未用 `--rerun` 伪造新鲜度）。
- [x] 「源码改对 ≠ 跑着的是新的」：§4.3 是收敛后实跑全量。
- [x] 未针对个人环境定制（无个人路径/账号/偏好）。
- [x] 未跑 `dart format`；换行符按各文件原有字节形态保持（零 CRLF/LF 转换噪声）。
- [x] 未碰 `lib/pages/` / `lib/gen/` / `lib/theme/` / `lib/components/` / `android/` / `daemon/` / `proto/` / `pubspec.yaml` / `test/support/`（`git status` 实证：我的改动只有 `lib/transport/pa_bridge.dart` + 3 个 `test/transport/` 文件；`collab_bridge.dart` / `collab_session_data.dart` 是并行子代理的 untracked 产物，我零改动）。
- [x] junction `G:\pa_h4` 用完已删（只删链接，目标目录完好）。
- [x] **未 git commit / add / push**（`git status` 中我的 4 个文件全部停留在工作区未暂存）。
