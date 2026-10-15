# B3 第 5 波 Chat 子步③——输入区（附件/斜杠/语音/模型选择）

> 范围：`chat_composer.dart`（UI）+ `chat_composer_data.dart`（纯函数）+
> `chat_input_transport.dart`（transport 面）+ 三份测试。
> 只读：`chat_session.dart`（C1）、`chat_turn_stream.dart`（C1）、`pa_bridge.dart`、
> `channel-contract.md`、`thread.pb.dart`/`common.pb.dart`、Android
> `ChatScreen.kt`/`chat/ChatComposer.kt`/`SlashCommandsLogic.kt`/
> `AttachmentPreview.kt`/`TurnSession.kt`。未碰冻结文件，未 commit。

## 1. transport 面：`listSlashCommands`

`flutter/lib/pages/chat_input_transport.dart`（`ChatInputTransport`，与
`chat_history_transport.dart` 同模式：`pa_bridge.dart` 只读消费
`kPaBridgeMethodChannel` + `bytesOrThrow` + `PaBridgeException`/
`PaBridgeErrorCodes`，`_invoke` 形状抄 10 行——跨文件私有方法复用不了，
抄的是错误语义逐字）：

| 项 | 值 |
|---|---|
| 方法名（契约字符串） | `listSlashCommands`（照 `listThreads` 模式） |
| arguments | `ListSlashCommandsRequest{thread_id, refresh}` wire 字节（`refresh` 缺省 false；`true` = 跳 daemon 10 分钟 TTL 缓存，进会话/下拉刷新语义） |
| 返回 | `ListSlashCommandsResponse` wire 字节 → `commands[]`（`SlashCommand{name, description, input_hint}`；空 = 非面板会话或快照暂不可用，不拿空冒充错误） |
| 错误码 | `NOT_CONNECTED`/`AUTH_REQUIRED`/`INTERNAL`（宿主）+ `HOST_UNAVAILABLE`/`DECODE_FAILED`（Dart；宿主 code 原样透传） |
| 宿主侧 | Kotlin 宿主桥 **B4 波次实现**（解析请求 → 按 `agent_id` 自动路由 daemon `ListSlashCommands` → 字节回传；B4 前如实抛 `HOST_UNAVAILABLE`，面板退回内置伪命令兜底） |

### 附件上传面判定口径（2026-10-15 全仓 grep 结论）

`proto/v1` 与 `android` 全仓**无** `UploadAttachment`/`AttachFile` 类 RPC——
附件**不走独立上传 RPC，不编新 RPC**。发送时走 `TurnInput` blocks 内联，
与 Android `TurnSession.buildTurnInput`（`TurnSession.kt:730-763`）同形：

- 文本块：`[附件已上传到电脑: <pcPath>]` 说明 + 正文（`attachNote + text`）；
- 图片块：`ImageBlock{data, media_type}`（仅 `attachmentKind == IMAGE` 且有字节，
  缺 `media_type` 兜底 `image/jpeg`；codex 运行中插话只取文本是 daemon 侧约定）；
- 随 `turnStreamSend` 首帧/追问帧发出。

`TurnInput` 组装是 **C4** 的事（composer 只交出原文 + 受控附件表，见 §4）。

## 2. 纯函数对位表：`chat_composer_data.dart` ↔ Android

| Dart（本文件） | Android 对位 | 语义（逐条同款） |
|---|---|---|
| `slashTokenOf` | `SlashCommandsLogic.kt:167` | 第一个空白前前缀；`/` 开头；裸 `/` 算词元；空/非斜杠 null |
| `filterSlashCandidates` | 同上 `:181` | 大小写不敏感前缀；词元后已敲空白 → 空；裸 `/` 全量；无命中退全量；空全集 → 空 |
| `slashCandidatesFor` | 同上 `:196` | 非面板会话直接空；内置置顶 + daemon 重名让位（`goal`/`compact` 原生别名） |
| `slashPanelVisible` | 同上 `:221` | 面板会话 + 非离线 + 词元 + 过滤非空 |
| `slashUnknownAgentHintVisible` + `kSlashUnknownAgentHint` | 同上 `:232,249` | `该助手暂无可用命令`（与面板互斥；加载中/离线不提示） |
| `applySlashSelection` | 同上 `:268` | `/词元` → `/命令 `（已有空白不重复补；保留后续；无词元 no-op） |
| `applySlashPick`（FILL 部分） | 同上 `:293` | 内置伪命令 no-op（调用方先分流）；codex 空 hint 技能填自然语言`请使用 <名称> 技能：`；其余模板回填 |
| `stripSlashToken` | 同上 `:306` | 只摘词元、保留后续、掐头去尾 |
| `builtinSlashCommands[For]` | 同上 `:136,150` | `/目标 /压缩 /计划`；pi 无目标；`/压缩`认 `compact:`、`/计划`认 `review:` |
| `agentCapableOf` / `slashPanelAgentOf` / `isSlashPanelAgentFallback` / `isZcode/Codex/PiAgent` | 同上 `:54-104,116` | 位优先 + 三源回落；大小写/空白不敏感；空 agent 恒 false；只认同 agent 位 |
| `attachmentKindOf`（三态） | `AttachmentPreview.kt:45` 二态扩展 | `image/*` 优先；图片扩展名；文本 mime/扩展名（含代码/配置后缀）；其余 `unsupported`（pdf/zip/未知——判出来为诚实提示，不摆能发假象） |
| `composerImageEntryVisible` | `ChatScreen.kt:2108` 取反肯定形态 | `image_input:<agentId>` 位；无位/空 agent 不显示图片入口（文本入口不受影响） |
| `validateComposerInput` + `kComposerMaxLength = 20000` | `TurnSession.send` 空口径（`isBlank` 拒） | 空/纯空白拒；超长给分段提示（proto 未声明上限，本值仅客户端前置提示，不静默截断）；边界值放行 |
| `SlashCommandInfo` + `slashInfoFromProto` | `SlashCommandInfo` 展平 | daemon 命令恒 FILL，三字段原样透传 |
| `SlashPickAction`（fill/goal/compact/review） | `SlashAction`（FILL/GOAL/COMPACT/REVIEW） | 点选分流形状；**动作执行留 C4**（本波只转发，见 §4） |
| `ChatPendingAttachment` | `Attachment`（`ChatScreen.kt:371`） | 受控附件元素（name + mediaType + kind 派生；上传/字节由 C4/B5 接线） |

## 3. 门控矩阵（能力位 × 入口）

| 入口 | 位/名单 | 无位表现 |
|---|---|---|
| 斜杠面板 | `slash:<agentId>` 位在 **或** 旧三源（zcode/codex/pi）回落 | 不开面板；已知 agent 敲 `/` 给人话空态`该助手暂无可用命令`（加载中/离线不提示） |
| daemon 候选 | `listSlashCommands` 成功（B4 前 HOST_UNAVAILABLE） | 退回内置伪命令兜底（codex/zcode 至少 `/目标`；空表不入缓存但记问过闸，同会话不再逐字刷 RPC） |
| `/压缩` | `compact:<agentId>` | 不进候选（点了必败的假入口不摆） |
| `/计划` | `review:<agentId>` | 同上（现状仅 codex 宣告） |
| `/目标` | 三源通用（pi 除外，无对应位，按 agent 判） | pi 不展示 |
| 图片附件入口 | `image_input:<agentId>`（codex/pi/zcode 带位、dsh 无位） | **不渲染**图片按钮；文本附件按钮不受影响 |
| 文本附件入口 | 无位要求（回调存在即渲染） | —（拾取实现由 C4/B5 接，无回调不渲染入口） |
| 语音 | 无依赖（pubspec 冻结） | 只渲染`语音输入随后续波次`（纯 Icon + 文案，**无可点录音键**） |
| 模型行 | 无位要求 | 空模型名展示`模型：跟随桌面`；`onModelTap` 为空则整行不可点、无 chevron（具体选择器留 C4/Settings②复用） |
| 发送键 | `validateComposerInput == null && enabled` | 空/纯空白/超长/离线禁用（`onPressed: null`）；提交交原文（首尾保留，代码缩进不丢）后本地清空 |

## 4. C4 接线契约（frozen，ChatPage 组装时读）

```dart
ChatComposer(
  threadId: '...',            // 斜杠拉取 thread_id；切换会话切缓存键
  agentId: ...,               // 会话归属 agent（可空；空按无位处理）
  capabilities: [...],        // 连接态 hostCapabilities 透传（slash/compact/review/image_input 四位消费）
  modelName: '...',           // 空 = 跟随桌面
  effortLabel: '...',         // 透传展示
  attachments: [...],         // ★受控：C4 持有；转 TurnInput blocks（§1 口径）随首帧发；onSubmit 里同步清空
  slashTransport: ...,         // null = 生产默认通道
  enabled: true,              // 离线快照态置 false（发送禁用 + 面板不弹）
  onSubmit: (String text) {}, // ★frozen：ValueChanged<String>；空闲调 TurnSession.startTurn(TurnInput)，运行中追问调 steer(text)
  onSlashPick: (SlashPick p) {}, // FILL 已回填（无需再动作）；GOAL→目标弹窗 / COMPACT→CompactThread / REVIEW→StartReview（review focus = 点选时 strip 后的输入框残留，composer 已本地摘词元）
  onModelTap: ...,            // 模型/强度切换入口（null = 不可点）
  onPickTextFile: ...,        // 文本拾取（null = 不渲染入口）
  onPickImage: ...,           // 图片拾取（null 或无位 = 不渲染入口）
  onRemoveAttachment: (int i) {}, // 受控移除（null = chips 无移除键）
)
```

- `onSubmit: ValueChanged<String>` 即 `void Function(String)`——**签名冻结**，
  C4 按此接 `TurnSession`（`startTurn(TurnInput)` / `steer(String)` 见
  `chat_session.dart:142,169`，只读）。
- composer 不 import session；`TurnInput` 组装（含附件 blocks 内联）归 C4。

## 5. 测试原文

新增 54 用例（data 35 + transport 6 + widget 13），三文件独立先绿；全量
`flutter test`：**812 全过**（基线 758 + 新增 54，无回归）；`flutter analyze`
（junction `G:\pa_c3`，用完已删）：**No issues found!**

```
00:01 +54: All tests passed!                     # 三文件独立跑
00:14 +812: All tests passed!                    # 全量（含基线 758）
Analyzing pa_c3...
No issues found! (ran in 4.0s)
```

三态覆盖（任务书点名项）：

- 空禁用：`composer-send` 初态 `onPressed == null` → 有字可用 → 提交交原文 +
  输入框清空；纯空白保持禁用且无回调。
- 附件门控两条：无位只有文本入口（图片键 `findsNothing`）；有
  `image_input:codex` 位双入口 + 图片拾取回调可达；chips 展示 + 移除带下标。
- slash 面板开合：非斜杠不开 → `/` 拉候选开（含 daemon `init` + 内置兜底）→
  点选回填 `/init ` + FILL 事件转发；`/目` 点 `/目标` 只转发
  `SlashPick(目标, goal)` 且词元摘除；未知助手（hermes 无位）不开空面板、
  给人话空态。
- 语音占位：`语音输入随后续波次`在位；`IconButton(Icons.mic_none_rounded)`
  `findsNothing`（无假录音键）。
- 模型行（附带）：空名`模型：跟随桌面`且无点击态；`codex/gpt-5.4 · 高` +
  点击走出口。

### 红档记录（本波内已清零，非他人之红）

1. `InkWell(tooltip:)`——该参数不存在，改 `Tooltip` 包 `InkWell`。
2. 宿主缺席用例：flutter_tester 对无 handler 调用不自发抛
   `MissingPluginException`（回 null），按 `pa_bridge_test` 范式改 mock 显式抛。
3. 置顶用例自摆乌龙：daemon 夹带 `goal`/`compact` 触发原生别名让位（实现正确），
   改用无碰撞 daemon 表；让位另有专条用例钉死。
4. `ListTile` 套 `Container+BoxDecoration` 触发 debug 断言（ink 遮挡）+ Chip
   删除钮无 key 位：面板容器改 `Card`（自带 Material 祖先），删除键钉在
   `deleteIcon` 上；codex 空 hint 技能走自然语言（实现正确），回填用例改带
   `inputHint` 的命令。

## 6. 文件清单（只新建，不碰他人领地）

| 文件 | 行数 | 说明 |
|---|---|---|
| `flutter/lib/pages/chat_composer_data.dart` | ~480 | 纯函数层（对位表见 §2） |
| `flutter/lib/pages/chat_input_transport.dart` | ~90 | transport 面（`listSlashCommands`；附件口径见 §1） |
| `flutter/lib/pages/chat_composer.dart` | ~520 | 输入区 UI（C4 契约见 §4） |
| `flutter/test/pages/chat_composer_data_test.dart` | 35 用例 | 纯函数 |
| `flutter/test/transport/chat_input_transport_test.dart` | 6 用例 | transport mock（含 HOST_UNAVAILABLE/DECODE_FAILED/code 透传） |
| `flutter/test/pages/chat_composer_test.dart` | 13 用例 | widget 三态 + 模型行 |

未改：`chat_session.dart`、`chat_timeline.dart`、`chat_history*.dart`、
`chat_snapshot_store.dart`、`chat_turn_stream.dart`、`chat_history_transport.dart`、
`terminal*.dart`、`browser*.dart`、`pa_bridge.dart`、`main.dart`、`gen/`、
其他 `pages/`、`android/`、`daemon/`、`proto/`、`pubspec.yaml`、
`channel-contract.md`。未 `git add/commit/push`。

后续交接：B4 宿主桥注册 `listSlashCommands`（`channel-contract.md` 方法清单
届时由契约owner 追行，不属本波）；`TurnInput` 组装 + 附件拾取实现 + 斜杠动作
执行 + 模型选择器归 C4；语音引擎随后续波次依赖解冻。
