# B3 第 5 波 Chat 子步②——历史加载 / 离线快照 / 恢复回放（C2）

> 日期 2026-10-15。只新建 7 个文件（4 库 + 3 测试），其余一律只读。
> 不 commit（`git status` 自证见 §7）。

## 1. 基线与结论

| 项 | 值 |
|---|---|
| 开工基线 | `flutter test` 全量 **573 全绿**（任务书 539 为昨日数，并行波次已增，照实登记） |
| 交付后自有文件 | `chat_history_data/snapshot_store/history` 三份测试 **43/43 全绿**（见 §5 测试原文） |
| 交付后全量 | **+740 −5**；5 红全在 `terminal*`（T 波次在建文件，见 §6 登记），无一与本波相关 |
| `flutter analyze`（junction `G:\pa_c2`，用完已删） | 本波 7 文件**零问题**；全仓余 6 条 info 全属 T 波次 `terminal*`（见 §6） |
| C1 冻结契约 | 开工时 `chat_session.dart` 不存在 → 按任务书「单独跑自身文件 + 如实登记」执行；收尾时 C1 已落盘，**只读核对** `TurnSession.replayFrames` 签名对齐（见 §4），未 import、未改 C1 一行 |

## 2. transport 面（`chat_history_transport.dart`）

- `ChatHistoryTransport.getThreadHistory(threadId, {limit = 0, rounds = 0})`：
  请求 `GetThreadHistoryRequest(threadId/limit/rounds).writeToBuffer()` →
  `invokeMethod('getThreadHistory', bytes)` → 响应经 `bytesOrThrow` 解码为
  `GetThreadHistoryResponse`（照 `pa_bridge.listThreads` 模式）。
- `pa_bridge.dart` **只读消费**三件套（未改一行）：`kPaBridgeMethodChannel`（同一通道，宿主 B4 按此名注册）、`bytesOrThrow`（解码唯一收口）、`PaBridgeErrorCodes`/`PaBridgeException`（错误形状）。
- `_invoke` 的 try/catch 在本文件抄了 10 行（`PlatformException` 透传 + `MissingPluginException → HOST_UNAVAILABLE`）：跨文件私有方法复用不了，抄的是错误语义逐字，不是新口径——注释已写明。
- 错误码：`NOT_CONNECTED`/`AUTH_REQUIRED`/`INTERNAL`（宿主透传）+ `HOST_UNAVAILABLE`/`DECODE_FAILED`（Dart）；B4 前宿主缺席 → `HOST_UNAVAILABLE` → 页错误态「通道未就绪（B4 波次）」。
- 方法名字符串 `getThreadHistory`：B4 Kotlin 宿主注册清单新增项（channel-contract.md §6 由宿主波次回填，本文档不改它）。
- `SearchThreadMessages` **本波不接**（裁决见 `chat_history_data.dart` 文件头三条理由：无搜索 UI=死契约；`scope`/`has_more` 需 UI 配合才诚实；只增不改，需要时按本文件同模式加方法）。

## 3. 映射表（`chat_history_data.dart`：`historyToFrames` / `historyFrameMetas`）

| 历史侧 `ThreadMessage` | 回放侧 `TurnServerMessage` | 备注 |
|---|---|---|
| `blocks[i].text.text` | `textDelta.delta` 原样 | 流式增量的终局形态即全文，一帧即全量 |
| `blocks[i].thinking.text` | `thinkingDelta.snapshot` | snapshot=权威全文整体替换；`signature` 展示无用，丢弃（注释） |
| `blocks[i].toolCall` | `toolCall=ToolCallUpdate(STARTED, call 原样)` | id/name/jsonArgs 逐字透传 |
| `blocks[i].toolResult` | `toolCall=ToolCallUpdate(FINISHED, result 原样)` | callId/ok/output/elapsedMs 透传；UPDATE 是 live 专属，历史只留终局 |
| `blocks[i].image` | 跳过（无载体帧） | 宁可少显示不错显示（`ThreadSnapshot.kt:29` 同纪律）；不产生元数据，对齐不断 |
| `role` / `created_at` / `message_id` / `turn_id` | 不进帧 → `HistoryFrameMeta` sidecar，下标与 frames 对齐 | 帧无这些字段；role 未知值原样透传 |
| `sequence` | 恒 0 | 0=无序号，按无续传能力处理（thread.proto 续传注释）；水位是 live 流的事，不冒充 |
| 空消息（blocks 空/全跳过） | 占位空 `TextDelta('')` | 对应 Android `historyMessageToChatItems` 空占位；顺序与轮数不塌 |

- 顺序：响应原序展开，不重排。`historyTextOf`（快照+行渲染共用）：只拼文本块，`\n` 连接。
- 分页口径（Android `HistoryRoundsLogic` 移植，同值）：`kHistoryRoundOptions=[10,20,30,50]`、`kDefaultHistoryRounds=20`（客户端显式传，服务端无默认）、`clampHistoryRounds`、`canLoadEarlier`（全量/给到头/首条非 user 都不显示入口）、`nextHistoryRounds`、`historyRoundCount`（user 条数估计轮数）。
- 数据源：`ChatHistorySource` 接口 + `BridgeChatHistorySource`（默认最近 20 轮，`PaBridgeException → ChatHistoryLoadFailure` 人话，HOST_UNAVAILABLE 点名 B4；**失败不回退空响应**，防假空态）+ `FakeChatHistorySource`（记录 threadId，failure 注入）。

## 4. 快照契约（`chat_snapshot_store.dart`）

- 模型：`ChatSnapshot{threadId, messages: List<ChatSnapshotMsg{role,text,savedAtMs}>, sequence, savedAtMs, version}`；短键 JSON（`r/t/at`，顶层 `v/thread/seq/at/msgs`）与 Kotlin `saveMessages` 同形（互读待 B4 验证，不许诺）。
- 只留 role+text（`ThreadSnapshot.Msg` 同口径）；`toHistoryResponse()` 重建文本-only 响应（`message_id`/`turn_id` 留空——快照不存稳定身份，注释写明定位靠顺序）。
- 上限（Android 同值）：单会话 60 条 / 400KB（UTF-16 `length*2` 粗估同算法），从最新往回取，超限丢最旧；整体覆盖写，不合并。
- 写回时机：历史页每次拉取成功后覆盖写；失败不写（旧快照继续兜底）。
- 有效性纯函数：`isSnapshotValid`（版本 `kChatSnapshotVersion=1` + threadId 非空 + 水位非负；损坏/旧版/空=无效→null，下次覆写自愈）；`isSnapshotStale(snapshot, live)`（水位比较；**水位 0=未知永不过期**——当前历史帧无序号，调用方传 0，等 `GetThreadEventsSince` 波次喂真水位，签名不动）。
- 持久化：`ChatSnapshotPrefs`（KV：read/write/remove）+ `InMemoryChatSnapshotPrefs`，**不加 shared_preferences 依赖**（onboarding_prefs/task_center_data 同一先例）；production 实现待 B2 通道/依赖波次挂载。
- `clearSnapshot` 幂等：会话删除流程必须调（否则删了的会话离线还能翻出，`removeMessages` 同款理由）。

## 5. 回放接线（`chat_history.dart`）

- 三态：`正在加载历史…` / `会话历史加载失败`（HOST_UNAVAILABLE 点名「通道未就绪（B4 波次）」）/ 就绪；空态 `还没有消息` 只属于成功路径零消息（无快照失败→错误态，**空态错误态严格分家**）。
- 失败 + 有有效非空快照 → 离线徽标（`离线快照 · <time>`，自报副本身份）+ 快照行 + 重试；失败 + 空快照/无快照 → 错误态（不把失败伪装成空）。
- 行渲染：简化行（角色中文标签 + 时间 + 纯文本），**不复用 C1 tile**（C4 统一 tile，注释即交接点）；时间标签复用 `threads_bridge.threadRowTimeLabel`（只读 import，未改该文件——`threads_data` 复用语义的落实）。
- 「恢复到会话」：`onReplay(List<TurnServerMessage>)`，按序原样转发（`List.unmodifiable`），回放中防重复提交，无回调/无帧时禁用。C1 落盘前注入 fake（测试钉行为）；C1 已落盘后的核对结论：
  - `TurnSession.replayFrames(List<TurnServerMessage> frames): void`（chat_session.dart:229，幂等，同 sequence 不重复进）——本页 `ChatReplayCallback = Future<void> Function(frames)` 对齐：调用方传 `(f) async { session.replayFrames(f); }`（同步包 async，文件头已写）。**未 import C1 文件**（并行仍在 churn，回调注入即契约），C1 文件零改动。
  - 顺带一致性：历史帧 sequence 恒 0 → C1 `applyFrame` 的 `0 不推进不拦截`分支直通，水位无干扰（两边注释互为回声）。
- 不造假两条：无快照=空态不冒充错误（反向亦然）；replay 幂等不断言重复帧（连点两次只断言两次都到且帧相同，不去重）。

### 测试原文（自有三文件单独跑，改后复跑一致）

```text
00:01 +39: .../chat_history_test.dart: 恢复到会话 点恢复 → onReplay 收到与 historyToFrames 一致的帧
00:01 +40: .../chat_history_test.dart: 恢复到会话 连点两次 → 两次都到且帧相同（本页不去重，幂等归 C1）
00:01 +41: .../chat_history_test.dart: 恢复到会话 快照兜底态点恢复 → 快照重建的帧同样可喂
00:01 +42: .../chat_history_test.dart: 恢复到会话 onReplay 为 null → 按钮禁用（不摆点了没反应的入口）
00:01 +43: All tests passed!
```

（`chat_history_data_test` 19 + `chat_snapshot_store_test` 13 + `chat_history_test` 11 = 43。）

### analyze 原文（junction `G:\pa_c2`，已删）

```text
Analyzing pa_c2...
   info - ... terminal.dart:217:21 - control_flow_in_finally
   info - ... terminal_data.dart:335:9 - prefer_initializing_formals
   info - ... terminal_stream.dart:21:8 - unnecessary_import
   info - ... terminal_data_test.dart:12:8 - unnecessary_import
   info - ... terminal_test.dart:175:39 - no_leading_underscores_for_local_identifiers
   info - ... terminal_stream_test.dart:12:8 - unnecessary_import
6 issues found. (ran in 3.1s)
```

修掉的自有两条（复跑前）：`chat_history.dart:78 unused_field _metas`（warning：state 不留没人读的副本，sidecar 由消费方用纯函数重算，注释交代）+ `chat_history_data.dart:262 prefer_initializing_formals`（改 `this._rounds` 初始化形参）。之后本波 7 文件零问题。

## 6. 非我所为的红（如实登记，主会话收口）

- 全量 `+740 −5`：5 红全部 `terminal*`——`terminal_page_test（占位页_标题）`、`terminal_data_test ×2`、`terminal_test（loading）`、`transport/terminal_stream_test`。`git status` 显示 `terminal.dart/terminal_data.dart/terminal_stream.dart` 等为 T 波次在建（与 worktree 无关的同仓并行修改）；失败用例无一 import 本波文件（`turn_stream_test` 亦 grep 确认零引用 `chat_history`）。本波三文件在全量跑中全部通过（计数器 `+111…+151` 段无 `-N` 增长）。
- analyze 余 6 条 info 全属 T 波次 `terminal*` 文件（见上原文），未碰（非我文件，改了即越界）。

## 7. 文件清单（只新建，不碰其他；不 commit）

```text
flutter/lib/pages/chat_history_transport.dart   # getThreadHistory 方法面（只读消费 pa_bridge 三件套）
flutter/lib/pages/chat_history_data.dart        # historyToFrames/metas/分页口径/数据源抽象+fake
flutter/lib/pages/chat_snapshot_store.dart      # 快照模型+编解码+有效性+KV接口+内存实现+store
flutter/lib/pages/chat_history.dart             # 三态+简化行+离线徽标+恢复按钮
flutter/test/pages/chat_history_data_test.dart  # transport mock+映射+分页+数据源（19）
flutter/test/pages/chat_snapshot_store_test.dart# 往返+损坏/版本+有效性/裁剪+重建（13）
flutter/test/pages/chat_history_test.dart       # 三态+快照兜底+恢复转发（11）
docs/reviews/2026-10-15/c2-chat-history-snapshot.md  # 本文档
```

只读消费（未改一行）：`lib/transport/pa_bridge.dart`（`bytesOrThrow`/错误码/通道名）、`lib/gen/*`（thread/common.pb）、`lib/pages/threads_data.dart`（字段语义）/`threads_bridge.dart`（时间标签）、`ThreadSnapshot.kt`、`thread.proto`、`channel-contract.md`、`HistoryMessageMapper.kt`、`HistoryRoundsLogic.kt`。
冻结遵守：`chat_session.dart`/`chat_timeline.dart`/`chat_turn_stream.dart`（C1 在建）零改动、零 import；`main.dart`、`pubspec.yaml`、`android/`、`daemon/`、`proto/` 未碰。
