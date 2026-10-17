# 跨域契约面整洁审查（clean-cross-domain）

- **日期**：2026-10-17
- **范围**：Go daemon / Kotlin android / Dart flutter 三域交界的契约面（只读审查，未改任何代码/文件，未 git add/commit/push）
- **审查人**：subagent（另一会话派生）
- **方法**：`python tools/proto/gen_proto.py --check` 只读验证 + 跨文件 grep + Python 集合对账脚本（脚本与中间产物在 `C:\Users\yanzi\AppData\Local\Temp\opencode\`，未入仓库）
- **红线声明**：本报告只给「漂移表 + 建议清单 + 影响面」。**proto 改名 = 甲级改动；能力位增删 = 甲级改动；channel-contract 方法改名 = Dart+Kotlin 双端同步改动**。本报告不建议动 proto 字段/service 名，涉及处只标注「甲级、需双端同步」。

---

## 0. 结论速览

| 契约面 | 状态 | 一句话 |
|---|---|---|
| proto 双副本 | ✅ **一致** | `gen_proto.py --check` 通过，8 份 MD5 全同（另以 SHA256 复核） |
| proto 自身整洁 | ✅ 良好 | 字段命名 100% snake_case 无混用；`reserved` 有实例；注释密度高；`optional` 使用集中且有意图 |
| 通道名（4 条） | ✅ **逐字一致** | Dart 4 常量 ↔ Kotlin 4 常量全等 |
| 错误码（7 个） | ✅ **一致** | 文档/Dart/Kotlin 三方同集，无契约外码 |
| 方法面对账（35 行） | ⚠️ **P1×6** | Kotlin dispatch 缺 5 条 runtime 方法；文档 arguments 列 4 处与实现不符；2 条方法 Dart 实现不在 `transport/`（文档指向处） |
| 事件清单（3 type） | ✅ **一致** | conn_state/host_info/host_event 三方一致，五键平铺形状一致 |
| 能力位（14 族+6 宿主级） | ✅ **一致** | host.go 生成面 ↔ matrix 文档 ↔ Android/Flutter 消费面无「一端宣告另一端未知」（Flutter 用通用前缀分组兜底） |
| 跨域命名 | ⚠️ P2×4 | 同义不同名清单见 §7；proto 为权威，不建议动 |
| 跨域契约测试 | ⚠️ **P1×4** | 无「Dart↔Kotlin 通道/方法名字节级对账测试」；daemon 完全不测 channel-contract 方法名；startTurn 分支无具名测试 |

**没有 P0**：三端实际链路（方法名/通道名/错误码/事件 type/字节形状）在当前 APK 内是通的；文档漂移集中在「文档滞后于已落地的 B4 实现」。

---

## 1. proto 双副本与生成面

### 1.1 双副本校验（只读）

```
$ python tools/proto/gen_proto.py --check
[ok] 8 份 proto 双副本 MD5 一致
```

另以 SHA256 逐文件复核 8/8 相同（`proto/v1/*.proto` ↔ `android/app/src/main/proto/v1/*.proto`）。

`tools/proto/gen_proto.py` 设计良好：双副本同步 + 单版本 protoc 生成收口为唯一入口，注释固化了三个历史坑（protoc 探测、版本混用致 diff 噪音、Windows 盘符冒号）。`tools/audit/pa_audit.py` 的 `proto_sync` 规则是同一职责的 CI 化。**无建议。**

### 1.2 proto 自身整洁（8 份扫描）

| 检查项 | 结果 |
|---|---|
| 字段命名风格 | **100% snake_case，0 处 camelCase 混用**（此前粗略正则的「camelCase 413」是把单字小写字段误计；精确扫描确认无驼峰） |
| `syntax` | 8/8 `proto3`，统一 |
| `reserved` | 1 处实例：`host.proto:116-117` `reserved 2, 3, 4; reserved "workspace_changed", ...`（HostEvent 删字段的正确姿势，含字段号+名字双保留） |
| `deprecated` | **0 处使用**（全仓无废弃字段标记习惯——目前也没有需要标记的字段，属「无债」而非「坏习惯」；`model.proto` 有上游 deprecated 概念但未落到本 proto） |
| `optional` | model.proto 13 / runtime.proto 12 / thread.proto 2——集中在「补丁式 Set 请求」与「缺省=未报告」语义处，意图一致 |
| 字段号空洞 | 3 处：`HostEvent` 2/3/4（**已 reserved，合规**）；`ImportCodexThreadResponse` 缺 1、`ThreadMessageHit` 缺 5（**未见 reserved 声明**，P2） |
| service 组织 | thread.proto 42 RPC 按域分节注释（会话生命周期 / turn 双向流 / 事件续传 / codex 导入 / agent 导入 / 组织 / guardian / 导出 / 提示词 / slash / review / delegate / collab 角色 / collab 会话），注释解释「为什么」密度高 |
| 行号引用 | 文档与 Dart 注释里的 `xxx.proto:NNN` 引用 33 处全部解析到正确行（抽查脚本全过） |

**漂移表（proto 面）**

| # | 契约点 | 状态 | 证据 | 建议 | 风险 |
|---|---|---|---|---|---|
| P-1 | 双副本 MD5 | 一致 | gen_proto --check `[ok]` | 不动 | — |
| P-2 | `ImportCodexThreadResponse` 字段号缺 1 / `ThreadMessageHit` 缺 5 | 漂移(P2) | thread.proto 编号扫描 | 补 `reserved N;` 一句话声明（**不动字段本身**，纯注释级） | 未来误用该号 = wire 不兼容 |
| P-3 | `deprecated` 零使用 | 观察项(P2) | 8 份扫描 dep=0 | 立规矩：删字段一律 `reserved` 号+名，不删行；`host.proto:116` 即模板 | 无 |

---

## 2. channel-contract.md §3 方法表 ↔ Dart ↔ Kotlin（逐行对账，35 行全核）

### 2.1 三方集合差

| 来源 | 方法数 | 明细 |
|---|---|---|
| 文档 §3 表 | **35** | — |
| Kotlin `BridgeMethods.kt` dispatch 分支 | **30** | 缺 5 条 runtime |
| Dart 实现 | **33** | pa_bridge 25 + chat_turn_stream 4 + terminal_stream 4 |
| Dart 实现（补 pages/ 两个 transport） | **35** | + chat_history_transport / chat_input_transport |

差集：
- **文档有、Kotlin 无（5）**：`getRuntimeState` / `getRuntimePolicy` / `getControlPlane` / `listUnknownEvents` / `getUnknownEvent` → Dart 调这些会落 Kotlin `else -> HOST_UNAVAILABLE("未知方法…契约漂移")`。
- **文档有、Dart transport/ 无（2）**：`getThreadHistory` / `listSlashCommands` → 实现在 `flutter/lib/pages/chat_history_transport.dart` 与 `pages/chat_input_transport.dart`（与 pa_bridge 同通道、同 `bytesOrThrow`/`PaBridgeException` 纪律，注释自证「为什么不并入 pa_bridge」）。
- **Kotlin 有、文档无（0）**。
- **Dart 有、文档无（0）**。

### 2.2 逐行对账表（35 行 × 三方）

图例：✅ 一致｜⚠️ 文档滞后｜❌ 端缺失

| # | 方法 | 文档 args | Dart 实际 args | Kotlin | 判定 |
|---|---|---|---|---|---|
| 1 | getHostInfo | GetHostInfoRequest 字节(空) | 同 | parseProto | ✅ |
| 2 | connect | `{'addr','token'}` | 同 | stringMap | ✅ |
| 3 | disconnect | null | null | expectNull | ✅ |
| 4 | getConnectionState | null→Map 五键 | 同 | mapOf 五键 | ✅（键名逐字一致，见 §4） |
| 5 | pairCode | null→String | 同 | expectNull | ✅ |
| 6 | startTurn | TurnInput 字节 | 同 | **分支故意抛 HOST_UNAVAILABLE**（unary 已废，双流取代） | ⚠️ 文档仍列为在用方法；实为死入口（见 §2.4） |
| 7 | listThreads | ListThreadsRequest 字节(缺省空) | 同 | parseProto | ✅ |
| 8 | listDirectory | 同 | 同 | 同 | ✅ |
| 9 | readFile | 同（首块） | 同 | 同（流收首块，空流→INTERNAL） | ✅ |
| 10 | listBranches | 同 | 同 | 同 | ✅ |
| 11 | listCollabRoles | ListCollabRolesRequest 字节(空) | 同 | 同 | ✅ |
| 12 | setCollabRole | **`CollabRole` 字节** | **`SetCollabRoleRequest{role}` 字节** | parseProto(SetCollabRoleRequest) | ⚠️ 文档少写一层包装（行为兼容：宿主按 SetCollabRoleRequest 解析，Dart 发的就是它） |
| 13 | deleteCollabRole | DeleteCollabRoleRequest 字节(非 Map) | 同 | 同 | ✅ |
| 14 | dispatchCollab | 请求字节 | 同(可选参) | 同 | ✅ |
| 15 | getCollabSession | GetCollabSessionRequest 字节 | 同(可选参) | 同 | ✅ |
| 16 | listCollabSessions | **null** | **ListCollabSessionsRequest 字节** | parseProto(字节) | ⚠️ 文档 args 列错（§2.2 说无 proto 请求消息才传 null；此方法有 proto 消息） |
| 17 | cancelCollabSession | CancelCollabSessionRequest 字节 | 同 | 同 | ✅ |
| 18 | cancelCollabMember | 同 | 同 | 同 | ✅ |
| 19 | retryCollabMember | 同 | 同 | 同 | ✅ |
| 20 | getCollabPolicy | **null** | **GetCollabPolicyRequest 字节** | parseProto(字节) | ⚠️ 同 #16 |
| 21 | listTerminals | **null** | **ListTerminalsRequest 字节** | parseProto(字节) | ⚠️ 同 #16 |
| 22 | terminalAttach | `{'terminalId'}` | 同 | stringMap | ✅ |
| 23 | terminalWrite | TerminalClientMessage 字节 | 同 | parseProto | ✅ |
| 24 | terminalClose | `{'terminalId'}` | 同 | stringMap(+finally 取消流) | ✅ |
| 25 | turnStreamStart | `{'threadId'}` | 同 | stringMap | ✅ |
| 26 | turnStreamSend | TurnClientMessage 字节 | 同 | parseProto | ✅ |
| 27 | turnStreamCancel | null | null | expectNull | ✅ |
| 28 | turnResync | `{'threadId','afterSequence':int}` | 同 | argumentsMap(afterSequence as Number→toLong) | ✅ |
| 29 | getThreadHistory | 请求字节 | 同（pages/chat_history_transport） | parseProto | ✅（但 Dart 实现不在文档指向的 transport/，见 §2.3） |
| 30 | listSlashCommands | 请求字节 | 同（pages/chat_input_transport，带 refresh） | parseProto | ✅（同上） |
| 31 | getRuntimeState | GetRuntimeStateRequest 字节(空) | 同 | **无分支** | ❌ 端缺失 |
| 32 | getRuntimePolicy | 同 | 同 | **无分支** | ❌ 端缺失 |
| 33 | getControlPlane | 同 | 同 | **无分支** | ❌ 端缺失 |
| 34 | listUnknownEvents | ListUnknownEventsRequest 字节(缺省空) | 同 | **无分支** | ❌ 端缺失 |
| 35 | getUnknownEvent | 同 | 同 | **无分支** | ❌ 端缺失 |

### 2.3 文档结构性滞后（§1/§3 头/§6 全章）

| 位置 | 文档说 | 实际 | 判定 |
|---|---|---|---|
| §1 | 「Kotlin 宿主侧：**B4 波次待实现**（清单见 §6）」 | BridgeMethods.kt 30 分支已落地，installBridge 已挂载（docs/reviews/2026-10-17/b4g-c1-install-bridge.md） | ⚠️ P1 |
| §3 头 | 「宿主侧全部【B4 待实现】——`BridgeMethods.kt` 落盘中」 | 同上 | ⚠️ P1 |
| §6 | 9 个 `- [ ]` 未勾选清单 | 逐条对照全部已实现（含 EventChannel 注册、错误码对齐、readFile 首块、runtime 5 条**其中 Dart 侧已接**） | ⚠️ P1 |
| §1 表 | 只列 2 条通道（method + /events） | 实到 4 条（+ /turn_events、/terminal_events，Dart/Kotlin 双端常量在） | ⚠️ P1 文档缺两条通道行 |
| §3.16 | 「`getCollabPolicy`（§3.20 协同波次…）」 | **§3.20 不存在**（§3 只到 3.16） | ⚠️ P1 悬挂引用 |
| §3.8/§3.9 | 「proto `FileService.ListDirectory/ReadFile`」 | proto 服务名实为 **`FileSystemService`**（Dart 注释写对了，文档写错） | ⚠️ P2 |
| §3.6 | 「proto `TurnService.StartTurn`」 | 实为 **`ThreadService.StartTurn`**（proto 无 TurnService；Dart 注释同样写错，属双错） | ⚠️ P2 |
| §5 表 | 4 个码「产生方 = 宿主（B4）」 | 宿主侧 BridgeErrorCodes 已实现 | ⚠️ P2 |
| §7 表 | Dart 实现源只写 `transport/pa_bridge.dart` | getThreadHistory/listSlashCommands 在 `pages/` 两个 transport | ⚠️ P2 |

### 2.4 startTurn（#6）是死入口

- Dart `PaBridge.startTurn`（pa_bridge.dart:373）**零生产调用方**（grep 全 lib：唯一 `.startTurn(` 调用是 `chat_page.dart:143` 的 `TurnSession.startTurn` 页层类，内部走 `turnStreamStart+turnStreamSend` 双流；注释自证「旧 unary 保留兼容」）。
- Kotlin 分支**故意**抛 HOST_UNAVAILABLE（b4g-bidi-methods.md §C3 记录定案：unary 首帧与双流二选一，已选双流）。
- 文档 §3 仍把它列为 35 条之一、§3.6 花整节讲它。

**结论**：三端对它状态一致（都不通），**不是 P0**；但它是契约表里的「僵尸行」。建议二选一（都是文档级改动，**不动 proto**）：文档标注「已废：双流 turnStreamStart/Send 取代，宿主故意拒绝」，或从 §3 表移除并留一行沿革注。

---

## 3. §5 错误码 ↔ BridgeErrors.kt ↔ Dart PaBridgeErrorCodes

| 码 | 文档 §5 | Dart `PaBridgeErrorCodes` | Kotlin `BridgeErrorCodes` | Kotlin 使用点 |
|---|---|---|---|---|
| HOST_UNAVAILABLE | ✅ | ✅ | ✅ | BridgeMethods 6 处（streams 未装/未知方法/sink 缺） |
| DECODE_FAILED | ✅ | ✅ | ✅ | parseProto/stringMap/decodeKey/expectNull/argumentsMap |
| NOT_CONNECTED | ✅ | ✅ | ✅ | contractCodeOf(UNAVAILABLE) |
| AUTH_REQUIRED | ✅ | ✅ | ✅ | contractCodeOf(UNAUTHENTICATED) |
| INVALID_ADDR | ✅ | ✅ | ✅ | connect 空地址本地拒 |
| PAIR_DISABLED | ✅ | ✅ | ✅ | pairCodeContractCodeOf(PERMISSION_DENIED) + pairingEnabled 假/码空 |
| INTERNAL | ✅ | ✅ | ✅ | 兜底 + grpcFailure 非 gRPC 异常 |

- 三方集合**完全相同，无契约外码**（全 bridge 目录 grep `BridgeErrorCodes.*` 无第八个常量；全 Dart grep 无第八个）。
- `contractCodeOf` 是唯一映射表（gRPC Status.Code → 7 码），`pairCodeContractCodeOf` 是 pairCode 专属覆盖——BridgeErrorsTest 9 用例逐条钉死（含「PERMISSION_DENIED 通用表落 INTERNAL、pairCode 专属落 PAIR_DISABLED」这条易错点）。
- **无漂移。** 唯二 P2：§5「产生方」列写「宿主（B4）」滞后；「新增宿主错误码必须回填本表」这条纪律没有守护测试（第七个码若被私加，三份常量集就会出现 divergence 而测试只各自钉自己）。

---

## 4. §4 事件清单 ↔ Dart EventChannel ↔ Kotlin BridgeEventChannel.kt

### 4.1 通道名逐字对账

| 通道 | Dart 常量 | Kotlin 常量 | 值 | 判定 |
|---|---|---|---|---|
| 方法面 | `kPaBridgeMethodChannel` | `BRIDGE_CHANNEL_NAME` | `dev.pocketagent/bridge` | ✅ 逐字一致 |
| 事件面 1 | `kPaBridgeEventChannel` | `BRIDGE_EVENT_CHANNEL_NAME` | `dev.pocketagent/bridge/events` | ✅ |
| 事件面 2 | `kTurnStreamEventChannel` | `BRIDGE_TURN_EVENT_CHANNEL_NAME` | `dev.pocketagent/bridge/turn_events` | ✅ |
| 事件面 3 | `kTerminalEventChannel` | `BRIDGE_TERMINAL_EVENT_CHANNEL_NAME` | `dev.pocketagent/bridge/terminal_events` | ✅ |

**4/4 逐字一致，字节级无漂移。**

### 4.2 事件 type 与 payload 形状

| type | 文档 §4 | Kotlin 帧构造 | Dart 解析 | 判定 |
|---|---|---|---|---|
| conn_state | 五键与 type 平铺 | `connStateFrame()`: type+connState+connTransport+connAddress+connectionEpoch+hostCapabilities | `PaConnState.fromPlatform(raw)` 按同五键 strict 校验 | ✅（连「校验顺序=表序」都一致） |
| host_info | `info`: HostInfo 字节 | `mapOf("type","host_info","info",infoBytes)` | bytesOrThrow→HostInfo.fromBuffer | ✅ |
| host_event | `event`: HostEvent 字节 | 同形状 | 同 | ✅ |

- Kotlin 三帧各走独立 `launch` + `supervisorScope`（一源挂不杀全流）与文档 §4「坏事件不毒化流」语义一致。
- turn/terminal 事件通道帧是**裸 wire 字节**（非 Envelope Map），Dart `_decodeFrame` 直接 `bytesOrThrow`——与 BridgeEventChannels.kt:141-142 注释声明的形状一致，但**文档 §4 只描述了 /events 的 Envelope 协议，未描述另两条通道的裸字节协议**（P1 文档缺面，非实现漂移）。
- 未知 type → Dart 流错误 DECODE_FAILED；未知方法 → Kotlin HOST_UNAVAILABLE 且消息点名「契约漂移」——双边「未知显式失败」纪律对齐。

---

## 5. 能力位契约四端对账

### 5.1 位族全集

| 位族 | host.go 生成 | matrix 文档 §二 | Android 消费 | Flutter 消费 |
|---|---|---|---|---|
| `agent:<id>` | ✅ host.go:201-210 | ✅ | `resolveAgentAvailability` 前缀匹配 | 网格/点亮 |
| `agent_settings:<id>` | ✅ :112 | ✅ | `capabilitiesList.contains("agent_settings:$runningAgent")` | — |
| `agent_native_settings:<id>` | ✅ :118 | ✅ | `caps.contains(...)` | settings_agent 前缀分组（family 常量） |
| `text_generation:<id>` | ✅ :121 | ✅ | `"text_generation:$it" in capabilities` | chat_actions_data `agentCapableOf('text_generation')` |
| `collab:<id>` | ✅ :127 | ✅ | HomeScreen/CollabLogic 前缀过滤 | collab_session_data `family='collab:'` |
| `steer:<id>` | ✅ :136 | ✅ | agentCapableOf ×15 处 | —（无 steer 提交面） |
| `queue_clear:<id>` | ✅ :143 | ✅ | `queueClearVisible` 三门控 | — |
| `compact:<id>` | ✅ :155-161（三形状一位） | ✅ | agentCapableOf | agentCapableOf |
| `fork:<id>` | ✅ :164-167（双形状一位） | ✅ | agentCapableOf | agentCapableOf |
| `rollback:<id>` | ✅ :170 | ✅ | agentCapableOf | agentCapableOf |
| `review:<id>` | ✅ :173 | ✅ | agentCapableOf | agentCapableOf |
| `export_html:<id>` | ✅ :176 | ✅ | HtmlExportGate | agentCapableOf |
| `image_input:<id>` | ✅ :200（协商条件位） | ✅ | agentCapableOf | agentCapableOf |
| `slash:<id>` | ✅ :186-189（双形状） | ✅ | slashPanelAgentOf（位优先+旧三源回落） | slashPanelAgentOf（同构） |
| 宿主级 terminal/fs/git/agent/event_sequence | ✅ :204 | ✅ | — | settings_agent 分组注释列举 |
| `runtime.v1` | ✅ :208 | ✅（宿主级行） | runtimeControlCapable | kRuntimeV1Capability 门控 |

**没有「一端宣告、另一端当未知处理」的位**：
- Android 缺 direct `agentCapableOf` 的 6 族（agent_settings/agent_native_settings/collab/queue_clear/agent/text_generation）全部有等效消费（字符串 contains/前缀/专门 gate），非漏接。
- Flutter 侧更少（8 族 agentCapableOf + runtime.v1 + agent_native_settings family + collab: 前缀），**但 `settings_agent.dart` 的 `groupCapabilityBits` 是通用前缀分组：任何未知前缀按原文透传展示**（注释明确「未知前缀不允许写死名单」）——这是三域里最稳的未知位兜底，Android 无对应通用面（Android 是逐位硬编码门控，新位 = 新代码，符合「缺位=入口隐藏」语义，无错但无透传展示面）。
- 守护：`host_capability_bits_test.go`（五家真实构造器 × GetHostInfo 对表）+ `capability_gate_test.go`（**host.go 位族词汇表 ↔ matrix 文档机器对表**，位增删即红）——这是全仓最好的「文档↔代码」契约测试，位面四端对账有钉子。

### 5.2 能力位漂移表

| # | 契约点 | 状态 | 证据 | 建议 | 风险 |
|---|---|---|---|---|---|
| C-1 | 14 位族 × host.go/matrix/Android/Flutter | 一致 | §5.1 表；capability_gate_test.go 对表 | 不动 | — |
| C-2 | Flutter 无 Android 式「未知位透传展示」 | 漂移(P2) | settings_agent 有通用分组；Android ChatScreen 逐位硬编码 | 评估 Android 是否需要同类「能力位只读陈列」面（不必须：Android 语义是缺位即隐藏） | 低 |
| C-3 | matrix §二说「14 位族 + 6 宿主级」但宿主级行把 runtime.v1 与其余 5 个并列写在一条 | 文档(P2) | host.go:204 五个 + :208 runtime.v1 分两行 | 文档表格微调排版 | 无 |

---

## 6. 命名跨域一致性矩阵

proto 是 wire 权威；Kotlin/Dart 侧变量名是客户端内部名。评估「同义不同名」及对齐必要性：

| 概念 | proto（权威） | Kotlin | Dart | 一致性判定 |
|---|---|---|---|---|
| 会话 | `Thread` / `thread_id` | `Thread`/`threadId`（主）、`sessionId`（外部 agent 会话，proto `ExternalSessionRef.session_id` 语义正确区分） | 同 Kotlin | ✅ 语义分工清晰（thread=daemon 会话，session=外部 agent 会话），非同名异义 |
| 轮次 | `Turn` / `turn_id` | `Turn`/`turnId` | `Turn` | ✅ |
| 能力位 | `capabilities`（HostInfo 字段） | `hostCapabilities`（StateFlow）/ `capabilities`（参数） | `hostCapabilities`（Map 键）/ `capabilities` | ✅ |
| 审批 | `ApprovalRequest`/`SubmitApproval` | `Approval`* | `ApprovalRequest` | ✅ |
| 连接态 | **proto 无对应消息**（宿主进程内部态，文档 §7 明证「不造假同源」） | `connState`/`connTransport`/`connAddress`/`connectionEpoch`/`hostCapabilities` | 同五键 + Dart 字段名缩写（state/transport/address/epoch/capabilities） | ⚠️ P2：Dart 字段名是 Kotlin 的缩写版；**这是有意的**（PaConnState 是小类不是 proto），文档 §3.4 表格已列映射。不建议对齐——对齐反而要把 Kotlin 字段缩水 |
| 续传水位 | `sequence`/`after_sequence` | `connectionEpoch`（全局刷新）+ `afterSequence` | 同 | ✅ |
| 斜杠命令 | `SlashCommand`/`ListSlashCommands` | `SlashCommand`/`slashPanelAgentOf` | 同 | ✅ |
| 协同 | `Collab*` | `Collab*` | `Collab*` | ✅ |

**同义不同名清单（仅 2 项值得记录）**：

| # | 现象 | 权威端 | 建议 | 影响面 | 分级 |
|---|---|---|---|---|---|
| N-1 | 连接态：Kotlin `connState`（字段/流）vs Dart `PaConnState.state` | Kotlin 是 Map 键权威；Dart 字段缩写是刻意的小类投影 | **不建议对齐**。文档 §3.4 映射表即契约；改任一端都是双端同步 + 测试全改 | pa_bridge.dart PaConnState、BridgeMethods.kt 五键、BridgeEventChannels.kt connStateFrame、两份测试、文档 §3.4/§4 | 甲级（若改） |
| N-2 | `FileService`（文档）vs `FileSystemService`（proto） | proto | 改文档两处（§3.8/§3.9） | 仅 channel-contract.md | 丁级 |
| N-3 | `TurnService`（文档+Dart 注释）vs `ThreadService`（proto） | proto | 改文档 §3.6 + pa_bridge.dart:366 注释 | 2 文件 | 丁级 |
| N-4 | Flutter `agentCapableOf`（chat_composer_data.dart）与 Kotlin `agentCapableOf`（SlashCommandsLogic.kt）同名同语义 | 双端已是镜像实现 | 不改 | — | — |

---

## 7. 测试跨域契约面

### 7.1 三域各自钉的面

| 域 | 契约测试 | 钉什么 |
|---|---|---|
| daemon | 375 个测试文件；7 个 `*wire_test.go`（collab_session/host_recovery/model_selection/runtime_observation/thread_search/turn_source_selection/unknown_events）；`host_capability_bits_test.go`；`capability_gate_test.go` | gRPC 层字节级往返、能力位对表（含文档对表） |
| android | `BridgeMethodsTest` 68 用例（30 分支中 29 个被具名覆盖）；`BridgeErrorsTest`（码映射 + 通道名断言）；`BridgeEventChannelsTest`；`FlutterHostChannelShellTest`（3 条事件通道 handler 注册 + listen/cancel ACK）；`BridgeTestFixtures` | dispatch 逐方法、错误码、通道注册 |
| flutter | `test/transport/pa_bridge*_test.dart`（mock 宿主 wire；35 个文档方法在测试中全部有点涉及）；`test/gen/wire_fixture_test.dart`（**python protobuf 7.36.1 marshal 的第三方字节 ↔ Dart 解码**）；`fixture_support.dart`；`tools/protofixture/gen_fixtures.py`（protoc 现生成 + manifest 账本） | Dart 侧通道层行为 + Go/Dart wire 兼容 |

### 7.2 重叠与空洞

| # | 空洞 | 严重度 | 证据 | 建议 |
|---|---|---|---|---|
| T-1 | **无「Dart 通道名常量 == Kotlin 通道名常量」对账测试**。各自钉各自字面量（Dart pa_bridge_test 断言自己的 const；Kotlin BridgeErrorsTest 断言 `BRIDGE_CHANNEL_NAME`），没有第三方字节级/跨仓断言 | P1 | flutter/test 与 android/test 各自 grep 到的通道名断言互不引用 | 加一个极简对账：pa_audit 新规则或单测——读 `pa_bridge.dart`/`BridgeEventChannel.kt` 常量做集合相等（与 proto_sync 规则同形态） |
| T-2 | **无「Dart 方法集 == Kotlin dispatch 集 == 文档 §3 表」对账测试**。当前 35/30/33 的差集（§2.1）测试完全看不见 | P1 | 本次审查靠 Python 集合脚本才发现 | 同上：一个 `channel_contract_sync` 审计规则，三源集合求差即红 |
| T-3 | daemon 侧零 channel-contract 测试（grep `dev.pocketagent/bridge`/`turnStreamStart` 在 daemon/internal 零命中）——daemon 只测 gRPC 面，MethodChannel 面完全由 android/flutter 各自测 | P1（结构性） | daemon grep 0 命中 | 可接受的分层（daemon 不该知道 Flutter 通道），但 §2.1 的 5 条 runtime 漂移正因没有「Kotlin dispatch ⊇ 文档方法集」钉子而可复发。落 T-2 即可覆盖 |
| T-4 | `startTurn` Kotlin 分支无具名测试（BridgeMethodsTest 68 用例中唯一未被 `"startTurn"` 字面引用的分支） | P2 | 脚本比对 30 分支 × 测试文本 | 补一条「startTurn → HOST_UNAVAILABLE + 消息含双流指引」用例（行为已在 b4g 文档记录但无钉子） |
| T-5 | 第七个错误码「不许私加」无跨端钉子（三方各钉自己的 7 个常量） | P2 | BridgeErrorsTest:109-117 只钉 Kotlin 侧 | T-2 规则可顺带覆盖（错误码集合三方求差） |
| T-6 | wire fixture 只覆盖 host_info/host_event/thread/turn_server_* 6 类；runtime 域 5 方法无 fixture（用 Dart 自构消息） | P2 | fixtures 目录 6 个 b64 | 低优先；runtime 消息结构简单，Dart 自构+往返已够 |

---

## 8. 漂移表汇总（按严重度）

### P0（三端实际不通/字节形状不一致）

**无。** 通道名、错误码、事件 type、字节形状（Envelope Map / 裸 wire 字节）三端当前一致。

### P1（文档与实现不一致但行为兼容 / 端缺失但可观察）

| # | 漂移点 | 证据 | 修哪端 | 风险 |
|---|---|---|---|---|
| D-1 | Kotlin dispatch 缺 5 条 runtime 方法（getRuntimeState/getRuntimePolicy/getControlPlane/listUnknownEvents/getUnknownEvent），Dart 已接、Android 自己的 UI 走 `AgentConnection.runtimeService()` 直连（不经过桥） | BridgeMethods.kt 无分支；RuntimeStateLogic.kt:734-737 直连 gRPC stub | 若 Flutter console 页要在 Android 壳里活：补 Kotlin 5 分支（照 parseProto+rpc 同款，零设计）；若 Flutter UI 只是并存模块：文档 §6 勾注「Flutter 侧自接，宿主桥不转」 | 中：不补则 Flutter settings_console 在 Android 壳必现 HOST_UNAVAILABLE「未知方法」——而这条消息的含义被写成「契约漂移」，会误导排查 |
| D-2 | 文档 args 列 4 处错：listTerminals/getCollabPolicy/listCollabSessions 写 null（实传 proto 字节）；setCollabRole 写 CollabRole 字节（实传 SetCollabRoleRequest 包装） | §2.2 表 #12/16/20/21 | 只改 channel-contract.md | 低（行为兼容），但 §2.2「无 proto 请求消息=Map/null」规则被文档自己违反 |
| D-3 | 文档 §1 通道表缺 turn_events/terminal_events 两行；§4 未描述两通道的裸字节帧协议 | §4.1 双端常量在、文档无 | 只改文档 | 低（新读者不知两通道存在） |
| D-4 | §1/§3 头/§6 全章「B4 待实现」滞后（9 个未勾选清单其实已实现） | BridgeMethods.kt 30 分支 + installBridge 已挂载 | 只改文档 | 中：文档自称「代码即契约」，滞后=契约失真 |
| D-5 | §3.16 引用不存在的 §3.20 | 悬挂引用 | 只改文档 | 低 |
| D-6 | 无 Dart↔Kotlin 通道名/方法名/错误码三方集合对账测试（T-1/T-2/T-5） | §7.2 | 加一个 pa_audit 规则 `channel_contract_sync`（读三处源码求集合差） | 低（加测试），收益高：本次 5 处漂移全可被它抓住 |

### P2（命名/文档不齐）

| # | 漂移点 | 修哪端 | 风险 |
|---|---|---|---|
| D-7 | `FileService`→`FileSystemService`（文档 §3.8/§3.9） | 文档 | 无 |
| D-8 | `TurnService`→`ThreadService`（文档 §3.6 + pa_bridge.dart:366 注释同错） | 文档 + Dart 注释 | 无 |
| D-9 | startTurn 僵尸行：文档列为在用方法 #6，Kotlin 故意拒，Dart 零调用方 | 文档标注「已废」或移表留沿革 | 无 |
| D-10 | 文档 §1「Dart 侧实现：transport/pa_bridge.dart」不全（getThreadHistory/listSlashCommands 在 pages/ 两个 transport） | 文档 §1/§7 补一句 | 无 |
| D-11 | proto 字段号空洞 2 处未 reserved（ImportCodexThreadResponse:1、ThreadMessageHit:5） | proto 加 `reserved` 行（**纯增量、不动字段**；若从严视为甲级则只记录不动作） | 低 |
| D-12 | §5「产生方=宿主（B4）」措辞滞后；连接态 Dart 字段缩写（N-1） | 文档措辞；N-1 不动 | 无 |

---

## 9. 建议清单（按性价比排序；**全部为文档/测试级，零 proto 改动**）

1. **【最高】加 `channel_contract_sync` 审计规则**（D-6/T-1/T-2/T-5）：读 `channel-contract.md` §3 表、`BridgeMethods.kt` dispatch、Dart 四个 transport 文件、三处错误码常量、四个通道名常量，做集合求差，非空即红。形态照 `pa_audit.py` 既有 `ProtoSyncRule`。**这一条规则能把本次审查发现的全部 P1 变成 CI 可回归。**
2. **修文档 5 处事实错**（D-2/D-3/D-5/D-7/D-8）：args 列 4 处、通道表补 2 行、§3.20 悬挂引用、FileService/TurnService 服务名。
3. **§1/§3 头/§6「B4 待实现」批量更新为已落地**（D-4），startTurn 标注废弃（D-9），Dart 实现源补 pages/ 两个 transport（D-10）。
4. **定夺 runtime 5 条的去向**（D-1）：要么补 Kotlin 5 分支（~30 行，照 parseProto+rpc 同款），要么文档明说「Flutter 侧自接、宿主桥不转」。当前状态最坏：消息说「契约漂移」但实际是「有意未接」，排查误导。
5. **补 startTurn 分支具名测试**（T-4）：一条用例钉「HOST_UNAVAILABLE + 消息含双流指引」，把 b4g 文档定案变成钉子。
6. **proto 两处字段号空洞补 `reserved`**（D-11）：一行一条，纯增量；若审计口径下 `proto/**` 任何改动都甲级，则本条降级为「记录在案，下轮改 proto 时顺手补」。
7. **不做**：连接态 Dart 字段缩写对齐 Kotlin（N-1）——刻意的小类投影，对齐是双端同步甲级改动且无收益。

---

## 10. 红线标记（本次审查触碰的甲级面，均未动）

| 红线 | 本次是否触碰 | 说明 |
|---|---|---|
| proto 改名/改字段 = 甲级 | **未触碰**。D-7/D-8 建议只改文档/Dart 注释，不动 proto；D-11 若做也仅 `reserved` 增量 | proto 8 份文件零字节改动（MD5 校验通过即为证） |
| 能力位变更 = 甲级 | **未触碰**。§5 结论是「一致」，零位增删建议 | host.go / matrix / 两端消费零改动 |
| channel-contract 方法改名 = 双端同步 | **未触碰**。§2 无方法改名建议；startTurn 是「文档标注废弃」而非删方法 | 35 个方法名零改动建议 |
| 测试水位不许倒退 | 不适用（未改代码） | — |
| 不留假绿 | 遵守：本报告所有结论附可复现命令/脚本路径；「无 P0」的断言来自集合对账而非抽样 | 脚本在 `C:\Users\yanzi\AppData\Local\Temp\opencode\xdomain_*.py` |

---

## 附：可复现命令

```bash
# proto 双副本（只读）
python tools/proto/gen_proto.py --check

# 方法/错误码/通道/能力位集合对账（脚本在临时目录，未入仓库）
python C:\Users\yanzi\AppData\Local\Temp\opencode\xdomain_methods.py    # 35/30/33 集合差
python C:\Users\yanzi\AppData\Local\Temp\opencode\xdomain_errors.py     # 7 码三方 + 通道名 + 事件 type
python C:\Users\yanzi\AppData\Local\Temp\opencode\xdomain_table.py      # 35 行逐行三方表
python C:\Users\yanzi\AppData\Local\Temp\opencode\xdomain_caps.py       # 位族四端
```
