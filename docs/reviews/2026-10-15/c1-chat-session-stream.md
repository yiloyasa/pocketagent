# B3 第 5 波① Chat 子步①——TurnSession + 双向 turn 流 + 时间线骨架

> 波次定位：Chat 压轴波第一步（最难的一块）。只新建 6 个文件（3 lib + 3 test）
> + 本文档；绝不碰 `pa_bridge.dart`（只读 import）、`main.dart`、`gen/`、
> 其他 pages、android、daemon、proto、pubspec、`channel-contract.md`；
> 不 commit。C2/T/B 三线并行，本波文件零重叠。

## 0. 基线与水位（诚实登记）

| 项 | 值 |
|---|---|
| 开工基线 | `flutter test` 全量 **573 全绿**（任务书写的 539 是旧数，开工实测 573，C2/T/B 已先加用例） |
| 完工本波 | 新 3 文件 **60 用例全绿**（turn_stream 16 + chat_session 24 + chat_timeline 20） |
| 完工全量 | **755 通过 / 3 失败**，3 失败全在 `terminal_*`（`terminal_data_test` / `terminal_page_test` / `terminal_test`，并行 T 线在途文件，其 analyze 亦有 10 项在途问题——非本波所为，未碰） |
| analyze（junction `G:\pa_c1`） | 本波 6 文件 **零问题**；全仓 12 项中其余 10 项归并行线（chat_history 1 + terminal 域 9），未碰 |
| junction | 已删（`G:\pa_c1` 用完即删） |

## 1. 流面契约（`lib/transport/chat_turn_stream.dart`，`TurnStreamBridge`）

方法面复用主通道 `dev.pocketagent/bridge`（与 PaBridge 同通道，宿主多注册 4 个方法名）：

| 方法（冻结名，前三不许改名） | arguments | 返回 | 宿主侧（B4） |
|---|---|---|---|
| `turnStreamStart` | `{'threadId': String}`（Map 形态，后续加参只增键） | null | 为该会话建一条 StartTurn 双向流 |
| `turnStreamSend` | `TurnClientMessage` wire 字节（input 首帧 / supplement 追问 / approval 应答 / cancel 取消统一入口，调用方按 proto 封帧） | null | 往流里发一帧 |
| `turnStreamCancel` | null | null | 拆本通道流资源 |
| `turnResync`（本流面 additive） | `{'threadId', 'afterSequence'}`（proto `GetThreadEventsSinceRequest` 口径） | `List<Uint8List>`（每元素一帧；空 = 无新帧） | 收齐 server-stream 后一次返回 |

事件面 `dev.pocketagent/bridge/turn_events`：宿主订阅 StartTurn 双向流，逐帧推
`TurnServerMessage` wire 字节。宿主缺席 = 流静默（F1b 定案：存活探针走方法面，
`turnStreamStart` 即报 HOST_UNAVAILABLE）。

- 错误模型沿用 `PaBridgeException` + `PaBridgeErrorCodes`（只读 import
  `pa_bridge.dart` 的 `bytesOrThrow` 收口 + 异常/错误码；`_invoke` 与
  `_invokeExpectNull` 按同一契约手写——pa_bridge 私有方法不可复用，注释写明）。
- 坏帧 → 流错误 `DECODE_FAILED`，流保持打开（坏一帧不杀整条流，主事件面同纪律）。
- `turnResync` 解码在 transport 层逐元素验形（坏元素定位到 `快照[i]`），调用方只做状态机应用。
- server-stream 收成一次性方法的理由写进文件头：Dart 侧 EventChannel 无"请求一次回一段流"形态。

## 2. 状态机（`lib/pages/chat_session.dart`，`TurnSession`）

`TurnPhase` 六态：`idle / running / queued / cancelling / failed / completed`
（对位 TurnSession.kt 的 running/recovering/queuedCount/终局收敛；用量卡/思考落卡/
检查点/看门狗留后续波次，不摆假字段）。

- **首帧走 stream 面统一**（定案）：`startTurn(TurnInput)` = `turnStreamStart` 建流
  + `turnStreamSend(input 帧)`；旧 unary `PaBridge.startTurn` 保留给兼容，本类不调用
  （注释写死，两条首帧路径必有一条是幽灵）。
- **sequence 水位**：`sequence>0 && <=lastSequence` 丢弃；新帧推进；0 不推进不拦截。
  实时流与回放共用 `applyFrame(frame, {replay})`（TurnSession.kt `applyServerFrame`
  的 `replay` 形参同源；回放不写本次提交的 Toast）。
- **`replayFrames(List<TurnServerMessage>)`**（C2 调，冻结名）：按序喂帧，幂等
  （同 sequence 不重复进；注释写明 C2 恢复回放调它）。
- **`steer(text)`**：运行中发 `TurnClientMessage{supplement}`；空闲调抛 StateError；
  发送成功不改本地态——Steered 还是 Queued 只看回执帧（hermes Steer 恒 false 落
  Queued，文案不按能力位预判，与 TurnDelivery.kt:39-46 同源，注释在案）。
- **`cancel()`**：流内 cancel 帧优先（reason `用户在手机端终止`），成功后
  `cancelling`，终局帧收尾；空闲/终局调是空操作。`closeStream()` 显式拆流
  （终局不自动调：QueueFlushed + Started 紧随终局，避免误杀——注释写死）。
- **`clearQueue()` 本地收敛**：`TurnClientMessage` 无清队帧，冻结方法面无
  `ClearQueuedInputs` 对应——只收敛本地计数与 phase，daemon 真清除随 B4 补齐
  （TODO 在文件头，不假装已清）。
- **`resyncFromDaemon()`**：`turnResync(threadId, lastSequence)` 快照 → `replayFrames`
  补帧；终局帧在其中结算；`ResyncRequired` → `needsFullHistory` 置位 + 水位作废
  （全量拉取留 C2）；宿主缺席如实抛 HOST_UNAVAILABLE。
- **`runningThreadIds`**（冻结名，静态只读快照）：running（含 queued/cancelling）
  才在集合；终局/起流失败即移除。mini 条与 Home 共用——本波先暴露，后续波次消费。
- 终局排队不断尾：`queuedCount>0` 时 COMPLETED/FAILED 不收尾（等 QueueFlushed +
  下一条）；`overflow`（pos=0）置位 + 不摆 Toast；`approvalResolved` 撤卡
  （幽灵审批卡治理）；codex `ERROR && !willRetry` 落 `errorMessage`。
- `ChangeNotifier`：任一变化即 notify；`takeDeliveryNotice()` 取后清 null；
  事件流坏帧记 `streamError` 诊断、不改 phase。

## 3. 时间线骨架（`lib/pages/chat_timeline.dart`）

- **TurnDelivery.kt 移植**（flutter/lib 无既有对位，grep 确认后移植，TurnSession 复用同一份）：
  `kDelivery{Pending,Steered,Queued,Overflow}` + `deliveryLabel`（pending 发送中…/
  steered 已插入运行中任务/queued 第N位·下一条/overflow 队列已满未送达）+
  `deliveryAckNotice`（Steered→`已插入运行中任务`、Queued→`已排队，当前轮结束后执行`、
  其余 null，hermes 修正注释同源）+ `queuedAckAccepted`（pos>0）。
- **`ChatTimelineAccumulator`**：同 sequence 去重（seq<=0 恒应用）+ 增量拼接
  （草稿在工具卡/审批卡/终局帧到达时落卡；thinking snapshot 整体替换）+ tile 投影。
- **五型 tile**：`text`（用户/助手正文，thinking 弱化样式）/ `toolCall`
  （STARTED 建卡、FINISHED/UPDATE 同 id 原地回填，一次调用一条；空名兜底`工具`）/
  `approval`（占位卡 + `ApprovalAction{requestId, approved}` 回调出口，真交互留 C4；
  resolved 撤卡 + 说明）/ `completed`（终局标记；无 delta 时 `final_blocks` 兜底正文）/
  `failed`（人话错误，空兜底`任务失败`；partial 落正文）。codex 事件/用量帧忽略
  （不摆假卡，注释写清留后续波次）。
- **`ChatTimeline` widget**：五型渲染 + 用户消息下 Steered/Queued 徽标 + 空态
  （`还没有消息，先发一句话开始吧`）+ 运行中尾巴 + 失败横幅 + 失败重试回调。
  宿主未就绪态：本骨架无通道直调（数据由 TurnSession 供给），`HOST_UNAVAILABLE`
  路径在 transport/session 层诚实上抛，页面错误态文案（`轮次通道未就绪`）留装配波次。

## 4. 字段映射（proto → Dart，gen 字段名逐字）

| 面 | proto（thread.proto） | Dart（gen stub） |
|---|---|---|
| 上行首帧 | `TurnClientMessage{input: TurnInput{thread_id, blocks[]}}` | `TurnClientMessage(input: TurnInput(threadId, blocks))`，`ContentBlock(text: TextBlock(text))` |
| 上行追问 | `TurnClientMessage{supplement: TurnSupplement{blocks}}` | `TurnClientMessage(supplement: TurnSupplement(blocks: …))` |
| 上行取消 | `TurnClientMessage{cancel: CancelSignal{reason}}` | `TurnClientMessage(cancel: CancelSignal(reason: '用户在手机端终止'))` |
| 下行增量 | `TurnServerMessage{text_delta.delta, thinking_delta.{delta,snapshot}, sequence(14)}` | `whichMsg()==textDelta/thinkingDelta`，`sequence.toInt()`（fixnum Int64） |
| 下行工具 | `ToolCallUpdate{phase(STARTED/FINISHED/UPDATE), call.{id,name}, result.{ok,output}}` | `ToolCallUpdate_Phase.STARTED`；回填键 `call.id`（空则末张 running） |
| 下行审批 | `ApprovalRequest{request_id, reason, details}` / `ApprovalResolved{request_id, note}` | `pendingApproval` 挂起 / 空 id 或同 id 撤卡 |
| 下行回执 | `Steered{turn_id}` / `Queued{position, depth}` / `QueueFlushed{remaining}` | pos>0 排队（phase=queued），pos=0 overflow；flushed 减计数 |
| 下行终局 | `TurnCompleted{turn_id, final_blocks[], output_state}` / `TurnFailed{turn_id, error.{message}, partial_text}` | 空草稿才用 final/partial；error 空兜底`任务失败` |
| 续传 | `GetThreadEventsSince{thread_id, after_sequence}` / `ResyncRequired{reason}` / `TurnServerMessage.sequence` | `turnResync(threadId, afterSequence)`；resyncRequired → 水位作废 + `needsFullHistory` |

## 5. 测试原文（三文件 60 用例，mock 的永远是宿主 wire 行为）

- `test/transport/turn_stream_test.dart`（16）：建流 Map 编码 + 非 null 回包 DECODE_FAILED；
  input/cancel 帧字节原样过桥（宿主侧解帧对账）；拆流无参；MissingPlugin→HOST_UNAVAILABLE；
  PlatformException 透传；turnResync 快照解码/空数组/非 List/坏元素定位`快照[1]`/缺席；
  事件好帧（含 sequence）/坏帧 DECODE_FAILED 且流继续/宿主 error 透传/缺席静默 + 方法面探针。
- `test/pages/chat_session_test.dart`（24）：起流 running + runningThreadIds + 首帧字节；
  运行中重开 StateError；缺席回 idle；steer 空闲 StateError/空文本 ArgumentError/
  supplement 帧 + 不预判；拼接 + snapshot 替换；水位去重 + seq0 恒应用；replay 幂等；
  Steered/Queued/Overflow/Flushed 四分支（含回放不写 Toast）；completed/final 兜底；
  failed 人话 + 空兜底；排队不断尾；审批挂起/撤卡；resyncRequired 置位；cancel 帧 +
  cancelling + 空闲空操作；clearQueue 本地收敛；resync 补帧结算 + 缺席；宿主推终局帧
  自动结算；坏帧记诊断不断流。
- `test/pages/chat_timeline_test.dart`（15 纯 + 5 widget）：徽标/Toast 文案逐字；
  拼接落卡；去重；thinking 落卡；final 兜底；failed；工具建卡/回填/兜底名/失败态；
  审批占位/撤卡；Steered/Queued/Overflow/Flushed 徽标；无 pending 空操作；
  widget 五型渲染/徽标/空态/运行中/横幅/审批回调/重试回调。

## 6. analyze（junction `G:\pa_c1`，用完已删）

- 本波 6 文件：`No issues found!`
- 全仓 12 项：本波初版 2 项（`chat_turn_stream.dart` 多余 `dart:typed_data` import、
  `chat_session_test.dart` 局部变量下划线前缀）已修；余 10 项归并行线在途文件
  （`chat_history*.dart` 2 + terminal 域 8，含 1 error），未碰、如实登记。

## 7. 文件清单（只新建，不碰旧文件）

| 文件 | 行数 | 说明 |
|---|---|---|
| `flutter/lib/transport/chat_turn_stream.dart` | ~230 | `TurnStreamBridge`：方法面 4 方法 + `turn_events` 事件流 + 解码收口 |
| `flutter/lib/pages/chat_session.dart` | ~400 | `TurnSession` + `TurnPhase` 六态：状态机/水位/起流/追问/取消/清队/续传/回放/`runningThreadIds` |
| `flutter/lib/pages/chat_timeline.dart` | ~560 | TurnDelivery 移植 + `ChatTile` 五型 + `ChatTimelineAccumulator` + `ChatTimeline` widget |
| `flutter/test/transport/turn_stream_test.dart` | ~330 | 16 用例 |
| `flutter/test/pages/chat_session_test.dart` | ~460 | 24 用例 |
| `flutter/test/pages/chat_timeline_test.dart` | ~360 | 20 用例（15 纯 + 5 widget） |
| 本文档 | — | 交付：契约 + 状态机 + 回放 API + 字段映射 + 测试 + analyze + 清单 |

## 8. 交接（后续波次）

1. B4 宿主实现：注册 `turn_events` StreamHandler + 4 方法（`turnStreamStart/Send/
   Cancel` + `turnResync` 收齐一次返回）；错误码对齐 §5（`channel-contract.md`
   本波未动，B4 需同步本文档的新增 4 方法 + `turn_events` 事件）。
2. C2 恢复回放调 `TurnSession.replayFrames`；`needsFullHistory` 置位后走全量回填；
   `runningThreadIds` 给 mini 条/Home 消费。
3. C4 审批真交互：`ApprovalAction` 回调出口已留，应答链路（`TurnClientMessage{
   approval}` 经 `turnStreamSend` + `SubmitApproval` 降级）待接。
4. `clearQueue` 的 daemon 真清除（`ClearQueuedInputs` 方法面）随 B4 补齐。
5. 用量卡/思考折叠卡/codex 事件卡/检查点落盘/看门狗：Android 侧能力，Flutter 后续波次。
