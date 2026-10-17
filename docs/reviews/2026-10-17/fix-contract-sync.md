# 跨域通道契约漂移处置 + 对账同步器落地（fix-contract-sync）

- **日期**：2026-10-17
- **触发**：`docs/reviews/2026-10-17/clean-cross-domain.md` §8 漂移表 **D-1..D-6**（P1）
- **范围**：`flutter/docs/channel-contract.md`（契约文档）、`android/.../bridge/BridgeMethods.kt`（未知方法兜底文案）、`tools/audit/pa_audit.py` + `tools/audit/test_pa_audit.py`（新审计规则）、`AUDIT.md` / `AGENTS.md` / `DEVELOPING.md`（规则条数回流）
- **执行者**：subagent（另一会话派生，接管一条被限流中断的线）
- **方法**：逐条对账 diff 与文件现状 + 独立集合推导脚本（不复用规则自身的解析代码）+ 5 处变异验证 + 全量门禁
- **红线声明**：
  - **零 `git add` / `commit` / `push`**（全程只读 git + 只写工作树文件）。
  - **未触碰** `proto/**`、`android/app/src/main/proto/**`、能力位三件套（host.go / matrix 文档 / 守护测试）、`flutter/lib/gen/**`、任何方法**改名**、`daemon/**`。
  - **零格式化写操作**（未跑 `dart format` / `gofmt -w` / autopep8）。
  - **零 Dart 源码改动**（`flutter/lib/**` 与 `flutter/test/**` 的工作树变更全部来自并发的另一条线；本线唯一的 Dart 触碰是变异验证中的瞬时改写，已按字节还原并自证 sha256 一致）。
  - 未跑 `flutter test` / `flutter analyze`（理由见 §7.5）。

---

## 0. 结论速览

| # | 漂移点 | 审查判定 | 本轮结论 | 依据 |
|---|---|---|---|---|
| **D-1** | Kotlin dispatch 缺 runtime 等 5 条；未知方法文案谎称「契约漂移」 | 只改文案 + 文档标「有意未接」，不加方法 | **已完成（且范围比报告更大）**：兜底文案改为并列两种可能并指向文档；§3 表新增**末列「宿主桥」**逐行声明，§8 立「有意未接」清单——实为 **15 条**（不止报告说的 5 条）。**零方法新增** | §1.1 |
| **D-2** | 文档 arguments 列 4 处错 | 以 Dart wire 为权威改文档 | **已完成**：4 处逐字节核对后改正，机器检查（对账项 2）逐轮钉死 | §1.2 |
| **D-3** | §1 通道表缺 `turn_events` / `terminal_events`；§4 未描述裸字节帧 | 只改文档 | **已完成**：§1 补 2 行（含双端常量与行号），新增 §4.1 专述另两条通道的裸 wire 帧协议 | §1.3 |
| **D-4** | §1/§3 头/§6 全章「B4 待实现」滞后 | 按现状更新，真机项保持未完成 | **已完成**：§6 九项清单按现状逐条勾/改写，**「真机验证待设备」保持未勾选**；另补三处报告未点名的自相矛盾（§3.6 / §3.16 / §4.1 符号名） | §1.4 |
| **D-5** | §3.16 引用不存在的 §3.20 | 改成真实锚点 | **已完成**：改为「§3 表 #20」 | §1.5 |
| **D-6** | 无三方集合对账测试 | 加 `channel_contract_sync` 审计规则 | **已完成并通过变异验证**：7 条规则之一，六项对账、**零内嵌名单**、**全项只读**；5 处人为漂移全部精确报红，改回后字节零残留 | §1.6 / §2 / §3 |

**与审查报告的主要判定差异**：D-1 的「5 条」和「文档 35 条方法」都被事实推翻（真实为 45 条方法 / 15 条有意未接）。差异登记见 §4。

---

## 1. 逐条处置（含权威出处）

### 1.1 D-1 —— runtime 5 条 + 兜底文案（**首选路径，未加任何方法**）

**改动 A：`BridgeMethods.kt` 的 `else` 分支兜底文案（原谎称「契约漂移」）**

- 位置：`android/app/src/main/java/dev/pocketagent/app/bridge/BridgeMethods.kt:460-467`（注释 `:452-459`）
- 原文断言：「未知方法名**只能是**契约漂移（双端同 APK 同版本，未知方法名只能是 §3 与宿主实现未同步）」
- 改为：把两种可能**并列**写出并指向契约文档裁决——
  ① Android 桥**有意未接**（§3 表末列标「有意未接」的那批，理由见 §8）；
  ② **契约漂移**（§3 方法名与宿主 dispatch 未同步）。
- **为什么不替人下结论**：宿主侧要区分二者，唯一办法是把方法名写死成名单 —— 那是 AUDIT 红线 4「不新增写死的名单」，且名单本身立刻变成第二份真源。故交出「两种可能 + 定位锚点」，由排查者对照裁决。
- **既有断言未弱化**：文案仍保留 `未知方法` 与 `契约漂移` 两个可 grep 的定位词；Android 2002 例全绿（§7.4）证明 `BridgeMethodsTest` 对该分支的既有钉子未破。

**改动 B：文档把「有意未接」显式化**

- `flutter/docs/channel-contract.md` §3 表**新增末列「宿主桥」**（`已接` / `有意未接（见 §8）`），并在表后给出**口径段**：对账式为
  `{已接} == Kotlin dispatch 分支集` 且 `{已接 ∪ 有意未接} == Dart 通道面方法集` 且 `{已接 ∪ 有意未接} == 本表`。
- 新增 **§8「宿主桥『有意未接』清单」**，写明共同原因与三条处置纪律（不加分支 / 兜底文案不替人下结论 / Dart 面仍要真接）。

**权威出处（决定「15 条」而不是「5 条」的证据）**

| 断言 | 真源字节 |
|---|---|
| Kotlin dispatch 分支共 **30** 条 | `BridgeMethods.kt` `fun dispatch(` 到 `else ->` 之间 30 个 `"name" ->` 字面量（独立脚本复核：全文 `"x" ->` 也是 30，else 之后无遗漏） |
| §3 表 **45** 条方法 = 30 已接 + 15 有意未接 | §3 表逐行 + 上述 dispatch 集合，机器对账 `wired == dispatch` 差集双向为空 |
| runtime 5 条 Android 侧不过桥 | `AgentConnection.kt:1478` `fun runtimeService()`；`RuntimeStateLogic.kt:734-736` `runtimeService().getRuntimeState/getRuntimePolicy/getCollabPolicy`；`ControlPlaneLogic.kt:473` `runtimeService().getControlPlane` |
| Chat 周边 10 条 Android 侧不过桥 | `ChatViewModel.kt:669` `rpcResult { gateway.forkThread(threadId) }` 等同名 suspend 函数直打 gateway |
| Dart 侧这 15 条**确实在调**（不是僵尸行） | 规则推导的 Dart 方法面 45 条与 §3 表逐条相等（§2.2） |
| `getCollabPolicy` **已接**（不在 15 条内） | `BridgeMethods.kt:284` `"getCollabPolicy"` 分支 `rpc(method) { connection.runtimeService().getCollabPolicy(req) }` |

**判定**：D-1 采纳「首选路径」。**零方法新增**——加分支等于扩 RPC 契约面（甲级面 + 当前零消费者的假入口）。§8 处置纪律第 1 条明确：真要接就改 §3 表末列，**机器立刻红**（§3 变异 M4 已验证）。

### 1.2 D-2 —— arguments 列 4 处（以 Dart wire 为权威）

| §3 行 | 改前 | 改后 | 权威字节（Dart wire） |
|---|---|---|---|
| #12 `setCollabRole` | `CollabRole` 字节 | `SetCollabRoleRequest` 字节（外面包一层） | `pa_bridge.dart:768` `SetCollabRoleRequest(role: role).writeToBuffer()` |
| #16 `listCollabSessions` | `null` | `ListCollabSessionsRequest` 字节 | `pa_bridge.dart:519` `(request ?? ListCollabSessionsRequest()).writeToBuffer()` |
| #20 `getCollabPolicy` | `null` | `GetCollabPolicyRequest` 字节 | `pa_bridge.dart:590` `GetCollabPolicyRequest().writeToBuffer()` |
| #21 `listTerminals` | `null` | `ListTerminalsRequest` 字节 | `terminal_stream.dart:71` `(request ?? ListTerminalsRequest()).writeToBuffer()` |

顺带把 §3 表 #15/#17/#18/#19 的 Dart 签名订正为实际的可选参形态（`[XxxRequest?]`），依据同源。

**机器钉子**：对账项 2（§2.1）逐行比对「文档 arguments 列」与「Dart `_invoke` 实际实参表达式」——文档写 `null` 而 Dart 发字节、文档漏写请求消息包装，都判红。

### 1.3 D-3 —— §1 通道表补 2 行 + §4.1 裸字节协议

- §1 表补 `dev.pocketagent/bridge/turn_events` 与 `dev.pocketagent/bridge/terminal_events` 两行，各带 Dart/Kotlin 常量名**与行号**。
- 行号逐条复核为**真实字节**：`pa_bridge.dart:43/46`、`chat_turn_stream.dart:49`、`terminal_stream.dart:37`；`BridgeChannel.kt:60`、`BridgeEventChannel.kt:16/19/22`。
- 新增 **§4.1**：另两条事件通道每帧是**一条 proto 消息的裸 wire 字节**（无 `type` 判别、无 Map 包装）。权威出处：宿主 `BridgeEventChannels.kt:383` / `:433` 均是 `sink.success(value.toByteArray())`；Dart 侧 `chat_turn_stream.dart:202 _decodeFrame` / `terminal_stream.dart:196 _decodeEvent` 均走 `bytesOrThrow`。
- §1 末段补「方法面**一条通道**、六个 Dart transport 面共用」的事实（D-10 的文档半，顺手做完）。

### 1.4 D-4 —— 「B4 待实现」批量更新（**真机项保持未完成**）

- 文首 / §3 头：`B4 波次待实现` → `B4 波次已落地`，并写明装配点 `FlutterHostEngine.installChannelShell` 与 30 分支 / 15 未接。
- §6 由「9 项 `- [ ]` 未勾选清单 + 1 条本波不做」改为**接线状态**：**12 条 `- [x]` 已落地** + **1 条 `- [ ]` 真机待设备**。已落地的 12 条 = MethodChannel 注册与逐字对齐、proto 字节返回、五键 Map、三个列表转发、双向流六条、`turnResync` 收齐语义、`readFile` 首块与空流失败、三条 EventChannel、错误码对齐、`pairCode` 回环自检、方法面与事件面共享同一 `AgentConnection`、未知方法兜底。
  （原清单里的「runtime 域 5 条只读转发」一项**没有**照抄勾上——真源是那 5 条在 §3 表标 `有意未接`、dispatch 无分支；勾上就是假绿。改为在 §3 表末列 + §8 显式登记。）
- **§6 最后一项保持未勾选**：
  > `- [ ] **真机验证待设备**：… \`adb devices\` 当前**无设备**，真机冷启、事件面实推、双向流首帧落地均未在真机验过（2026-10-17）。`

  **没有**写成已完成。这条同时被机器哨兵反向保护（见 §2.1 对账项 6 只查「待实现 / 待落盘」，不查「待设备」，故不会被误判为漂移）。

**本轮另外补做的三处（报告未点名，属同一类文档滞后，均在 D-4 授权范围内）**

| 位置 | 问题 | 修正 | 权威出处 |
|---|---|---|---|
| §3.16 | 写「已接只读（6）」，与 §3 表末列把其中 5 条标「有意未接」**自相矛盾** | 改为「Dart 侧已实现（6）」+ 显式声明宿主桥接线状态看 §3 表末列 + 把下方 7 条改述为「**不列入 §3 契约表的 proto RPC**」（它们既无方法名也无 Dart 调用点，与 §8 不是一回事） | `BridgeMethods.kt` dispatch 无这 5 条分支 |
| §3.6 | §3 表末列写 `已接（故意拒，见 §3.6）`，但 §3.6 正文仍在描述 unary 已接的旧行为（「返回 null」「错误码 NOT_CONNECTED/AUTH_REQUIRED/INTERNAL」）——**指针指向的锚点没有兑现指针承诺** | 重写 §3.6：现状 = 有分支但故意抛 `HOST_UNAVAILABLE`；写清为什么不接（无消费面，假象比不接更糟）、Dart 侧零生产调用方的证据、以及当前实际错误码只有 `HOST_UNAVAILABLE` | `BridgeMethods.kt:438-449`；`pa_bridge.dart:373` `Future<void> startTurn` 定义；`flutter/lib` 内唯一同名调用点 `chat_page.dart:143` 是页层 `TurnSession.startTurn`（内部走双流） |
| §4.1 | terminal_events 行的 Dart 解码点写成 `_decodeFrame` | 订正为 `_decodeEvent`（并补两处行号 `:202` / `:196`） | `chat_turn_stream.dart:202` 是 `_decodeFrame`；`terminal_stream.dart:196` 是 `_decodeEvent` |

### 1.5 D-5 —— §3.16 悬挂引用

- `§3.20`（不存在，§3 只到 §3.16） → `§3 表 #20`（真实锚点，`getCollabPolicy` 在 §3 表第 20 行）。
- 机器钉子：对账项 5 收集文档内全部 `§X.Y` 引用，与「全部二/三级标题号 + §2 有序列表条目号」求差，差集非空即红。

### 1.6 D-6 —— 新审计规则 `channel_contract_sync`

见 §2（设计）、§3（变异验证）。

---

## 2. 同步器设计（`tools/audit/pa_audit.py` 规则 7，`class ChannelContractSyncRule` @ `:985`）

### 2.1 六项对账

| # | 对账项 | 判定逻辑 |
|---|---|---|
| 1 | **方法名三方** | `契约 §3 表 == Dart 通道面方法集`（逐向求差）；`§3 标「已接」== Kotlin dispatch 分支集`（逐向求差）。四类差集分别给出四句人话诊断（僵尸行 / 未登记 / 标已接无分支 / 有分支未登记） |
| 2 | **arguments 列 ↔ Dart wire** | 逐行比「文档该列」与「Dart `_invoke` 实参表达式」：文档写 `null` 而 Dart 发字节 → 红；文档漏写 Dart 实际封装的请求消息类型（`XxxRequest(` 形态）→ 红 |
| 3 | **通道名三方** | `§1 表 == Dart 常量 == Kotlin 常量`，三对逐向求差 |
| 4 | **错误码三方** | `§5 表 == Dart PaBridgeErrorCodes == Kotlin BridgeErrorCodes`，三对逐向求差 |
| 5 | **§X.Y 交叉引用** | 文档内全部 `§X.Y` 必须解析到真实锚点（标题号 ∪ §2 有序列表条目号） |
| 6 | **文档时效** | `installBridge(` 在 bridge 包**外**有真实调用点时，文档不得再写「待实现 / 待落盘」 |

### 2.2 推导方式（**零内嵌名单**，逐条自证）

| 数据 | 怎么来的 | 谁是唯一真源 |
|---|---|---|
| 契约方法名 / arguments 列 / 宿主桥列 | 解析 §3 表：第 2 列必须是整段反引号标识符；末列**由表头含「宿主桥」推出**，列缺失则单独报红并按「全部要求接」从严对账 | `channel-contract.md` |
| Dart 通道面**文件集** | 先从常量里认出「被 `MethodChannel(<标识符>)` 用的那个常量」，再取全 `flutter/lib` 树里用它的文件 —— 新增 transport 面自动进来 | Dart 源码 |
| Dart 方法名 | `_invoke / _invokeExpectNull / invokeMethod` 调用点；字面量直取，常量引用经常量表解引用（`kTurnStreamStartMethod` 这类）；形参声明被 `fullmatch` 排除 | Dart 源码 |
| Dart 通道名 / 错误码 | **按值的形状分流**：含 `/` = 通道名；`[A-Z][A-Z0-9_]*` 全匹配 = 错误码。不按所在类名 → 新增一条自动进对应集合 | Dart 源码 |
| Kotlin 通道名 / 错误码 | 同上，扫 bridge 包全部 `.kt` 的 `const val NAME = "..."` | Kotlin 源码 |
| Kotlin dispatch 分支 | 结构定位（`fun dispatch(` → `when (method)` → `else ->`）之间的 `"name" ->`，掩码只掩注释（方法名本身就是字符串） | Kotlin 源码 |
| §1 通道名列下标 | **由表头含「名称」推出**，不写死下标 —— 否则 §1 正文里的仓库路径（`pages/xxx_transport.dart`，也含 `/`）会被误当通道名 | 文档 |
| 「桥是否已装配」 | `installBridge(` 在 bridge 包外有调用点（唯一：`FlutterHostEngine.kt:212`）。**判据是调用点不是文档措辞** —— 桥摘掉装配后本项自动放行 | Kotlin 源码 |

**自证方法**（脚本在临时目录，未入仓库）：抽出规则 7 的 423 行源码块，穷举其全部字符串字面量，再与真源领域数据求交：

```
领域数据是否被内嵌 ====

 契约 §3 方法名 45 个
   出现在规则源码里的方法名: ['startTurn']
   出现在规则源码里的通道名: 无
   出现在规则源码里的错误码: 无
   规则里出现的 .dart 路径字面量: ['chat_turn_stream.dart', 'pages/xxx_transport.dart']
   规则里出现的 .kt 文件名字面量: ['BridgeMethods.kt']
=== 规则内写操作调用: 无（只读）===
=== 事件 type 内嵌: 无
```

三个 `startTurn` / `chat_turn_stream.dart` / `pages/xxx_transport.dart` 的命中全部落在**注释与 docstring**（`pa_audit.py:803` / `:916` / `:1053`），不在任何判定路径上。

规则源码里只剩两类字面量：① **定位词**（文件路径、Markdown 列位、表头字样、正则/结构锚点）；② **文档格式标记词**（`有意未接` / `已接` / `待实现` / `待落盘` / `null` / 章节号 `1 3 5`）。这两类都不是领域数据。本轮还顺手把规则头部注释里的硬编码计数（「方法名 45 个、通道名 4 个…」）改成不含数字的表述 —— **注释里的数字一样会过期成新的漂移源**。

### 2.3 只读性自证

- 源码层：规则 7 代码块内 `write_text / unlink / mkdir / rename / shutil.* / open(...,'w')` 命中数为 **0**，全部访问是 `read_text` / `read_bytes`。
- 行为层：单测 `test_rule_never_writes_checked_files` 对三个被检查文件（文档 / Dart / Kotlin）比对 `(bytes, mtime_ns)` 前后相等。

### 2.4 设计取舍与已知局限（诚实登记）

| 取舍 | 选择 | 理由 |
|---|---|---|
| 「有意未接」怎么表达 | 在 §3 表加**末列**（机器标记词），而不是新开一张名单表 | 接线状态与「这个方法是什么」在同一行，读者不用跨表对照；机器读表即得状态，不新增第二份真源 |
| 有意未接的成因 | **不**在代码里区分「有意未接 vs 漂移」 | 区分需写死方法名单 = 红线 4。改成文案并列两种可能 + 指向文档，把裁决权交回人 |
| Dart 侧文件集 | 按 `MethodChannel(<方法通道常量>)` 推导，不写死六个文件名 | 新增 transport 面自动进对账；否则规则自己就是漂移源 |
| Kotlin 侧 dispatch 文件 | **仍写死 `BridgeMethods.kt`**（唯一非推导的领域相关定位） | 需要一个确定性的入口；写成「扫 bridge 包找 `fun dispatch(`」会在多文件命中时产生歧义。单文件路径不是名单，且与 `proto_sync` 写死 `proto/v1` 同一形态。**登记为已知取舍** |
| 「文档时效」判据 | 看 `installBridge` 调用点，不看文档措辞 | 桥摘掉装配后文档再写「待实现」就不是漂移了；反之装配了还写「待实现」才是 |
| §X.Y 锚点集 | 标题号 ∪ §2 有序列表条目号 | §2 是 `1./2./3.` 列表不是子标题，但正文按 `§2.5` 引用；不收进去会全线误报 |

**已知局限（假绿通道，登记不改）**

1. **间接回调推不出方法名**：`settings_capabilities.dart:187` 有 `await invoke('listModels', …)`，但 `invoke` 是注入闭包，生产构造点 `:247` 传 `const BridgeModelsDataSource()`（`invoke == null`）→ 恒抛 `HOST_UNAVAILABLE`。它不是方法通道上的真实调用点，故不进 §3 表、规则也不报。**这是「未来形态」占位，不是漂移**；真接线那天会自然进规则视野（因为会变成 `_invoke` 调用点）。
2. **Dart 调不到的方法靠 Dart 自己暴露**：规则只看静态调用点。若某方法经运行时才拼出的方法名调用（当前仓库零例），静态推导看不见。
3. **arguments 列对账认「构造出的请求消息」**：`Map(` 这种全大写无泛型写法会被当成类型名而误报 —— 当前仓库用 `Map<String, Object?>{…}` 故不触发。这是**响亮失败（红）**不是静默放行，可接受。
4. **Kotlin 侧只认 `const val`**：若有人把通道名/错误码声明成普通 `val`，规则会看不见。当前 bridge 包 11 个 `const val` 全部命中（4 通道 + 7 码 + 1 `TAG`，`TAG` 值是驼峰不匹配错误码形状，正确排除）。
5. **daemon 侧零覆盖**（承接审查 T-3）：MethodChannel 面与 daemon 无关，daemon 不该知道通道名。这是有意分层，不是遗漏。

---

## 3. 变异验证（核心判决）

脚本在临时目录，**未入仓库**；每步 = 备份字节 → 变异 → 跑规则 → 还原 → sha256 比对。

| 变异 | 手法 | 规则判定 | 红项原文（节选） | 还原 |
|---|---|---|---|---|
| **M1** | 改**文档**方法名：§3 第 12 行 `setCollabRole` → `setCollabRoleX` | **exit 1 红，4 项** | `setCollabRoleX: 契约 §3 有此方法，Dart 通道面无任何调用点（僵尸行：要么删，要么接）` / `setCollabRole: Dart 通道面会调此方法，契约 §3 未登记` / `setCollabRoleX: 契约 §3 标「已接」但 Kotlin dispatch 无同名分支` / `setCollabRole: Kotlin dispatch 有分支但契约 §3 未登记` | sha256 一致 `43201964f17dc2cb` |
| **M2** | 改 **Kotlin dispatch 分支名**：`"turnResync" ->` → `"turnResyncX" ->` | **exit 1 红，2 项** | `turnResync: 契约 §3 标「已接」但 Kotlin dispatch 无同名分支` / `turnResyncX: Kotlin dispatch 有分支但契约 §3 未登记` | sha256 一致 `07a83fd670e59ff1` |
| **M3** | 改 **Dart 通道名常量**：`'dev.pocketagent/bridge/terminal_events'` → `…/term_events` | **exit 1 红，4 项** | `通道 …/term_events: Dart 有，契约 §1 表无` / `通道 …/terminal_events: 契约 §1 表有，Dart 无` + `Dart ↔ Kotlin` 两向 | sha256 一致 `a95c22db17eff266` |
| **M4** | 把 §3 表 `getRuntimeState` 的「有意未接」误标成「已接」 | **exit 1 红，1 项** | `getRuntimeState: 契约 §3 标「已接」但 Kotlin dispatch 无同名分支` | sha256 一致 `43201964f17dc2cb` |
| **M5** | 文档 §6 标题回退成「待实现」 | **exit 1 红，1 项** | `契约文档:369: installBridge 已有调用点（android\…\flutterhost\FlutterHostEngine.kt），文档仍写「待实现」` | sha256 一致 `43201964f17dc2cb` |
| **M6** | 全部改回后的最终态 | **exit 0 绿** | 方法名 45 条三方相等；通道名 4 条、错误码 7 个三方相等；§X.Y 全部解析 | 三个文件与基线**字节完全一致: True** |

M4 是「有意未接豁免不是后门」的判决：豁免只对**文档声明**生效，一旦把声明改成「已接」而代码无分支，立刻红。
M5 是 D-4 的回归钉子：文档任何一处退回「待实现」措辞即红。
三次还原后的 `git diff --stat` 中，`flutter/lib/transport/terminal_stream.dart` 与 `pa_bridge.dart` **均不出现** —— 变异零残留。

---

## 4. 与审查报告判定的差异登记

| # | 审查报告（clean-cross-domain） | 实测事实 | 差异性质 | 处置 |
|---|---|---|---|---|
| 1 | §3 表 **35** 条方法（§2.1） | **45** 条。审查时 `chat_actions_transport.dart`（B3 第 5 波④ Chat 动作面）的 10 个方法尚未进表 | 报告的基数已过期 | 补齐 10 行 + 把表头改成 45 条并标注来源波次 |
| 2 | D-1「Kotlin dispatch 缺 **5** 条 runtime 方法」 | 相对 §3 表缺 **15** 条（5 runtime + 10 Chat 动作） | 报告漏数（基数 35 时算对，但结论「5」在新基数下不成立） | §8 分两组如实写：runtime 域只读 5 条 + Chat 周边动作 10 条 |
| 3 | §2.1「文档有、**Dart transport/ 无（2）**：`getThreadHistory` / `listSlashCommands`」（并归为 §2.2 ✅ 无害） | 报告在同一段里既说这 2 条「与 pa_bridge 同通道、同纪律」又把它们算成差集 | 报告内部口径不一致（实为「实现文件不在文档指向的目录」，不是方法面缺失） | 文档 §1 末段改为「六个 transport 面共用一条方法通道」，把位置差异讲成位置差异，不讲成契约差异 |
| 4 | D-1 建议二选一：「补 Kotlin 5 分支」或「文档明说 Flutter 侧自接」 | 采纳第二项，但**发现只有一半是 Flutter 侧**：5 条 runtime 在 Flutter 壳里能走自有宿主，只有在 Android 壳里才拿 `HOST_UNAVAILABLE` | 报告的「Flutter 侧自接」措辞掩盖了「Android 壳里 Flutter UI 必现 HOST_UNAVAILABLE」这个可观察事实 | §8 处置纪律第 3 条显式写明「走 Android 壳时才拿 `HOST_UNAVAILABLE`——UI 必须有诚实占位」 |
| 5 | 报告未列 | §3.16 / §3.6 / §4.1 三处**新增内容自身引入或保留的自相矛盾** | 报告漏项（其中 §3.16/§3.6 是改完 §3 表后才成立的新矛盾） | 本轮补做，见 §1.4 |
| 6 | 报告未列 | `pa_bridge.dart:577` 注释写「arguments：null」但同函数 `:590` 实际发 `GetCollabPolicyRequest().writeToBuffer()`；`:626` 同款 | Dart 侧注释与自身代码矛盾（属报告 D-8 同类，P2） | **本轮不改**：`flutter/lib/transport/**` 属另一条并发线的工作面 + 硬禁区「dart 侧契约实现」。登记在案，下轮随 D-7/D-8 一起清 |
| 7 | D-7 / D-8（`FileService`→`FileSystemService`、`TurnService`→`ThreadService`）/ D-9 / D-10 / D-11 / D-12 | 仍成立（`§3.8`/`§3.9` 仍写 `FileService`，`§3.6` 标题仍写 `TurnService`） | **本轮范围外**（任务授权面是 D-1..D-6） | 不动，登记在案。D-10 的文档半已顺手做完（§1 末段） |

---

## 5. 伪命题澄清

审查报告与前序讨论里有几处看似成立、实则不成立的命题，逐条澄清：

| 伪命题 | 为什么假 | 证据 |
|---|---|---|
| ①「文档 §3 表 35 条 = 契约方法全集」 | 表**落后于代码**。`chat_actions_transport.dart` 的 10 个方法 Dart 侧在调、契约表无行 —— 这正是「未登记方法名 = 对账空洞」 | 规则推导 Dart 方法面 = 45 条，表 = 45 条（现已相等） |
| ②「未知方法只能是契约漂移」 | 宿主**拿不到**判定所需信息：区分「有意未接」与「漂移」需在代码里写死方法名单 = 红线 4。原文案是**用断言冒充事实** | 本轮文案改为并列两种可能 + 指向文档；M4 证明文档一旦误标立刻红 |
| ③「加 5 个 dispatch 分支风险低、照抄同款即可」 | 分支数从 30 变 45，等于**扩 RPC 契约面**（甲级），且这 10 条在 Android 壳里**零消费者**（Android 自有 UI 直连 gRPC）——无消费者的入口就是假入口 | `RuntimeStateLogic.kt:734` / `ChatViewModel.kt:669` 均不过桥 |
| ④「§3 表 arguments 列错了 4 处」的**根因**是文档粗心 | 根因是文档自己**违反**了它 §2.2 定的规则（「无 proto 请求消息才传 null」）——规则写在一处、执行在另一处，无机器检查 | 对账项 2 逐行机器比对，违反即红（M1/M4 同形态） |
| ⑤「补了同步器就不会再有漂移」 | 同步器只能覆盖**静态可推导**的面：运行时拼出的方法名、注入闭包里的间接调用（如 `listModels`）都看不见 | §2.4 已知局限 1/2 |
| ⑥「文档时效可以靠人定期复查」 | 复查是漂移的生产机制之一。本轮把判据从「文档措辞」换成「`installBridge` 有没有调用点」——桥摘掉后文档措辞自动免责 | M5 |
| ⑦「『真机验证待设备』也是滞后措辞，一起勾掉」 | **不是**。JVM 单测 + Dart mock 覆盖的是逻辑，不是链路；`adb devices` 无设备。勾掉它就是假绿 | §6 末项保持 `- [ ]`；本报告亦不声称真机已验 |

---

## 6. 收尾（被中断的执行者没来得及做的部分）

| 项 | 处置 |
|---|---|
| 规则条数回流 | `pa_audit` 由 6 条变 **7 条**。同步四处真值锚点：`AUDIT.md:117`（「六条」→「七条」+ 在枚举里补 `channel_contract_sync` 的职责一行）、`AGENTS.md:47`（§2 命令行注释「六条」→「七条」，**只改这一个数字，别处未碰**）、`DEVELOPING.md:72`、`DEVELOPING.md:455`（「六条」→「七条」）。后两处不在任务点名的两处之内，但留着就是本线刚消灭的那类新漂移，故一并同步并在此登记 |
| 单测用例数 | `test_pa_audit.py` 31 → 57（**净增 26**：本线新增 22 —— `ChannelContractSyncTest` 21 + `SelectRulesTest.test_new_rule_is_selectable` 1；另一条并发线新增 4）。`test_pa_upgrade_gate.py` 43。合计 **100**（基线 78 → 100，**≥ 基线**）。**删除 0 条、弱化 0 条**：逐条比对 HEAD 的 31 个测试方法名与当前 57 个，差异只有 2 个**改名**（`FakeGreenTest.test_stale_xml_is_warn → _redline`、`test_missing_results_is_warn → _redline`），二者属**另一条线**的 fake_green 严重级别裁决，不是本线，且改的是期望口径（WARN → RED）而非删除覆盖；另有 1 条既有断言随规则数变化（`SelectRulesTest.test_skip` 的 `len(ids)` 4 → 5） |

---

## 7. 验证证据（原文）

### 7.1 proto 双副本（未碰 proto 也自证）

```
$ python tools/proto/gen_proto.py --check
[ok] 8 份 proto 双副本 MD5 一致
gen_proto_exit=0
```

### 7.2 `pa_audit --json` 全量（七条规则逐条判定）

```
$ PA_MIRROR=G:\pa_ctx1 python tools/audit/pa_audit.py --json
  "total_rules": 7,
  "enabled": "sheet_overflow,roster,unknown_ui,fake_green,go_baseline,proto_sync,channel_contract_sync",
```

| 规则 | 判定 | 原文 |
|---|---|---|
| `sheet_overflow` | **pass** | 巡检 1 处 ModalBottomSheet 调用点：无溢出风险命中 |
| `roster` | **warn** | 名单模式命中但已在附录 A 登记（挂牌/豁免，只减不增）· 4 |
| `unknown_ui` | **warn** | 非注释区出现「未知」1 处 · `BridgeMethods.kt:462 · "未知方法 $method（dev.pocketagent/bridge 上没有宿主实现，两种可能请对照" +` |
| `fake_green` | **pass** | 206 份 XML / 2002 用例 / 0 失败 0 错误，最新结果距今 0.8 分钟（新鲜） |
| `go_baseline` | **pass** ×2 | `go build ./... 通过` / `go vet ./... 通过` |
| `proto_sync` | **pass** | 两副本逐文件 MD5 一致（8 个 proto） |
| `channel_contract_sync` | **pass** | 方法名 45 条三方相等（契约 §3 = Dart 通道面 = Kotlin dispatch 30 + 有意未接 15）；通道名 4 条、错误码 7 个三方相等；§X.Y 引用全部解析到真实锚点 |

`"summary": {"redline": 0, "warn": 2, "pass": 6}`，`audit_exit=0`。

> `unknown_ui` 的 warn 是**改动前就存在**的（旧文案同样含「未知方法」），非本轮新增；该 message 是宿主异常文案不是 UI 文案，机器无法判定可见性，按 AUDIT §3.3 交人裁决。

### 7.3 单测

```
$ python -m unittest tools.audit.test_pa_audit tools.audit.test_pa_upgrade_gate
Ran 100 tests in 0.912s
OK
```

（仓库既有跑法为 unittest —— 两份测试文件 docstring 自述；本机无 pytest 模块。）

### 7.4 Android 单测（`BridgeMethods.kt` 被改过 → 必跑）

```
$ PA_MIRROR=G:\pa_ctx1 python tools/device/_pa_gradle.py test
=== 1. robocopy 同步 ===
  robocopy rc = 3 (0/1/2/3 均为成功)
=== 1b. robocopy flutter module -> 隔离镜像 ===
  flutter robocopy rc = 1 (0/1/2/3 均为成功)
=== 2. gradle :app:testDebugUnitTest --rerun ===
  gradle rc = 0  elapsed = 68.9s
  log -> C:\Users\yanzi\_pa_gradle_test.log
--- 关键行 ---
  BUILD SUCCESSFUL in 1m 8s
```

**跑完立即核 XML**（`STALE_MINUTES=10`，顺序不可倒置）：

```
$ PA_MIRROR=G:\pa_ctx1 python tools/device/_pa_test_report.py 'G:\pa_ctx1\app\build\test-results\testDebugUnitTest'
XML 目录      : G:\pa_ctx1\app\build\test-results\testDebugUnitTest (来源: 位置参数)
XML 份数      : 206
累计 tests    : 2002
失败 failures : 0
错误 errors   : 0
跳过 skipped  : 0
最新 XML 时间 : 2026-10-17 12:31:25 (距今 0.2 分钟)
最旧 XML 时间 : 2026-10-17 12:31:25
新鲜度检查    : 全部 XML 距今 <10 分钟
水位判定      : 全部 XML 均来自 <600 秒窗口，0 个失败。共 2002 用例，全部通过
report_exit=0
```

**2002 = 基线 2002，用例数未跌**。

### 7.5 Dart —— **本轮跳过，未跑 `flutter test` / `flutter analyze`**

理由（三条，都是事实陈述不是借口）：

1. **本线零 Dart 源码改动**。`git status --short` 中 `flutter/lib/**`、`flutter/test/**` 的变更（`collab_roles_state.dart`、`task_center_data.dart`、`settings_downloads.dart`、`task_center.dart` 及对应测试）全部来自**并发运行的另一条线**；`flutter/docs/channel-contract.md` 是本线唯一的 Flutter 侧改动，它是 Markdown，不进 `flutter analyze` / `flutter test` 的编译面。
2. 变异验证对 `flutter/lib/transport/terminal_stream.dart` 的改写是**瞬时**的，已按字节还原并三重自证：sha256 与基线一致、`git diff --stat` 中该文件不出现、规则改回后 exit 0。
3. 此刻跑 `flutter test` 的数字会同时包含另一条线**进行中**的改动，既不是本线的证据也不能作为稳定水位，故不取。**交由主会话在 flutter 线收口后统一跑**。

---

## 8. 红线核对

| 红线 | 本轮 | 说明 |
|---|---|---|
| 绝不 `git add` / `commit` / `push` | **遵守** | 全程只 `git status` / `git diff` / `git show` |
| `proto/**` 与 `android/app/src/main/proto/**` 改动 | **未触碰** | `gen_proto.py --check` exit 0、8 份 MD5 一致 |
| 能力位（matrix + host.go + 守护测试） | **未触碰** | `go build` / `go vet` 通过 |
| `flutter/lib/gen/**` | **未触碰** | — |
| dart 侧契约实现与 `pa_bridge.dart` 公开 API | **未触碰** | 本线 Dart 侧只改了 `flutter/docs/channel-contract.md`（文档）；`pa_bridge.dart:577` 的注释矛盾**登记未改**（§4 差异 6） |
| 任何方法**改名** | **未触碰** | 45 个方法名逐字不变；M1/M2 的改名是瞬时变异，已还原 |
| `daemon/**` | **未触碰** | 本线零 Go 改动（`daemon/**` 的工作树变更来自并发线） |
| 不做格式化写操作 | **遵守** | 未跑 `dart format` / `gofmt -w` / autopep8 |
| 测试水位不许倒退 | **遵守** | Android 2002 = 基线 2002；unittest 100 ≥ 78 |
| 不留假绿 | **遵守** | 「真机验证待设备」保持未完成；本报告不声称真机已验；变异验证附字节级还原自证；`fake_green` 走的是本次实跑的 10 分钟内新 XML |
| 不新增写死的名单（红线 4） | **遵守** | 规则 7 零内嵌领域名单（§2.2 附穷举自证）；§8 的 15 条是**文档声明**（供人读），机器判定不依赖它——M4 证明改一行即红 |

---

## 附：可复现命令

```powershell
# 契约三方差（只读）
python tools/audit/pa_audit.py --only channel_contract_sync

# 变异验证（脚本在临时目录，未入仓库；含字节级还原自证）
python C:\Users\yanzi\AppData\Local\Temp\opencode\mutation.py
#   → C:\Users\yanzi\AppData\Local\Temp\opencode\mutation.out

# 零内嵌名单自证（穷举规则 7 全部字符串字面量 × 真源领域数据求交）
python C:\Users\yanzi\AppData\Local\Temp\opencode\chk_embedded.py
#   → …\chk_embedded.out

# 独立集合复核（不复用规则自身的解析代码）
python C:\Users\yanzi\AppData\Local\Temp\opencode\xcheck.py
#   → …\xcheck.out

# 全量门禁（结果 XML 须在 10 分钟窗口内）
$env:PA_MIRROR='G:\pa_ctx1'
python tools/device/_pa_gradle.py test
python tools/device/_pa_test_report.py 'G:\pa_ctx1\app\build\test-results\testDebugUnitTest'
python tools/audit/pa_audit.py --json
```