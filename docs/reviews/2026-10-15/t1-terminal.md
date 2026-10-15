# B3 第 6 波——Terminal 页（终端 L）交付记录

- 日期：2026-10-15；轨道：T1（终端 L，独立双雄之一）
- 基线：进场 `flutter test` 全量 **573 通过**（任务书所记 539 为已合入基线；
  差值 = 同树并行波次（browser/collab/terminal 占位）的未合入新增用例）。
  完工复跑全量 **758 通过**（含本波新增 60 + 并行波次新增），0 失败。
- 红线：未碰 `pa_bridge.dart`（仅 import `bytesOrThrow`/`PaBridgeException`/
  `PaBridgeErrorCodes`/`kPaBridgeMethodChannel`）、`chat_*.dart`、`main.dart`、
  `gen/`、`proto/`、`pubspec.yaml`、`channel-contract.md`、其他 pages/；
  未执行任何 git commit/add/push。

## 1. 流面契约（`lib/transport/terminal_stream.dart`，`TerminalStreamBridge`）

与 C1 turn 流面同型、文件隔离（通道名/方法名/帧类型不同，共文件会串契约）。

| 面 | 通道 | 方法/事件 | 参数 | 返回 |
|---|---|---|---|---|
| 方法 | `dev.pocketagent/bridge`（与 PaBridge 同通道，方法名无冲突） | `listTerminals` | `ListTerminalsRequest` wire（缺省=空字节） | `ListTerminalsResponse` wire |
| 方法 | 同上 | `terminalAttach` | `{'terminalId': String}` | null（订阅受理；首帧走事件面） |
| 方法 | 同上 | `terminalWrite` | `TerminalClientMessage` wire（调用方封帧） | null（收下，不代表 daemon 已执行） |
| 方法 | 同上 | `terminalClose` | `{'terminalId': String}` | null |
| 事件 | `dev.pocketagent/bridge/terminal_events` | — | 宿主逐帧推 `TerminalServerMessage` wire | opened/output/exit/error/resumed |

- open 帧不走 `terminalAttach`：`TerminalClientMessage{open}`（session/cwd/
  shell/cols/rows）由调用方封帧后经 `terminalWrite` 送入流（封帧函数见 §2）；
  `terminalAttach` 只做订阅管理。复用存活会话时服务端回 `Resumed` 帧。
- 错误模型沿用 `PaBridgeException` + `PaBridgeErrorCodes`：宿主
  `PlatformException` 三件套透传；通道缺席 `MissingPluginException` →
  `HOST_UNAVAILABLE`；宿主回非约定类型/坏字节 → `DECODE_FAILED`（消息定位到
  方法名/`TerminalServerMessage`）；无返回语义的方法回非 null = 契约漂移。
- 事件面语义：broadcast 无缓冲（PTY 本来就无回放，terminal.proto:69）；
  坏帧 → 流错误 `DECODE_FAILED` 且流保持打开（后续帧照收）；
  宿主缺席流静默（握手错被框架吞掉，存活探针走方法面）。

## 2. 块渲染策略（`lib/pages/terminal_data.dart` 纯函数 + `terminal.dart`）

- 无 xterm 依赖，不自研终端仿真器；输出块按纯文本块渲染：可滚动、
  `PATypography.code` 等宽、`SelectableText` 原文透传（含 ANSI 转义字节，
  不解析、不着色）。页面底部诚实附注"ANSI 转义按原文显示，xterm 着色随后续
  波次接入"。无假光标、无假 shell 提示符（输入行是普通 TextField + "发送"）。
- 增量拼接 `appendTerminalOutput(current, chunk)`：按行掐头留尾，保证留下
  "完整的最后 N 行"；行数上限 `kTerminalMaxLines = 500`；溢出时
  `truncated=true`，页面闩锁后展示 `terminalTruncationNotice()` =
  "—— 输出过长，已截断，仅保留最后 500 行 ——"。
- 退出码 `terminalExitText(code)`：0="终端已正常退出（退出码 0）"，
  非 0 只陈述码值，不猜原因。
- 封帧：`terminalOpenFrame`（默认空 cwd/shell + 80×24，对位
  TerminalScreen.kt:475-476）、`terminalInputFrame`/`terminalInputFrameFromString`
 （原始字节不转义；行输入由页面补 `\n` 表回车）、`terminalResizeFrame`。
- 行模型 `TerminalSessionRow{id,shell,cwd,alive}` + 副文案"有什么说什么，
  存活态恒为最后一节（运行中/已退出）"；`TerminalOutput` 不带 session_id
  （proto 原样）→ 单活跃 attach 假设，输出帧恒归当前会话；切回列表不 close
  （PTY 服务端保持运行），dispose 全关；`TerminalError` 帧转 `output` 流错误
  `TerminalStreamFailure`（错误字样不冒充终端输出）。
- 页面 `terminal.dart`：会话列表三态（加载中/加载失败+重试/就绪；空态
  `terminal-empty` 与错误态分家）+ 会话视图（输出块/截断标注/退出码/
  错误横幅/输入行/发送/关闭/回列表）+ attach 时 open 自带当前尺寸、步进器
  改尺寸即发 resize（列 ±10、行 ±5，下限 1）。
- `TerminalSession` 抽象自 `terminal.dart` 迁入 `terminal_data.dart`
 （terminal.dart 按名重导出，旧 import 路径继续有效）；真实现
  `BridgeTerminalSession`（open=attach→订事件→写 open 帧；input/resize 经
  write；close 对位 closeTerminal，幂等）。

## 3. 测试原文

自身三文件（60 用例）+ 全量复跑：

```
flutter test test/transport/terminal_stream_test.dart \
  test/pages/terminal_data_test.dart test/pages/terminal_test.dart
# 00:02 +60: All tests passed!

flutter test
# 00:11 +758: All tests passed!
```

- `test/transport/terminal_stream_test.dart`（18）：listTerminals 编解码/
  空列表真空态/4 类错误映射；attach/write/close 参数与空返回约定、非 null
  回返漂移、三者 HOST_UNAVAILABLE、注入通道隔离；事件面 output 原文（含
  ANSI）/exit/error/opened/resumed 解码、坏帧不毒化流、坏字节、宿主
  EventSink.error 透传、无 handler 流静默+方法面探针。
- `test/pages/terminal_data_test.dart`（26）：拼接（累加/<=上限/掐头留尾/
  闩锁语义/空块）/标注逐字/退出码/四种封帧 wire 往返/行映射与副文案/
  错误文案两路/Bridge 列表源三路/会话 open-input-resize-close 帧形状与
  输出-退出-错误流向、未 open 调用 StateError。
- `test/pages/terminal_test.dart`（16）：列表三态（加载→列表/失败→重试→
  列表/真空态/返回键）；attach（open 带 80×24）→ 输出出现→增量累积→
  发送（原文+`\n`，清空输入框）→空输入不发帧→退出码→600 行截断标注→
  错误横幅→列+10 发 resize(90,24)→关闭回流→会话内返回不断流。

analyze（全树 `flutter analyze` 因 analysis_server LSP 通道崩溃无法执行，
系工具侧异常，与代码无关；改用 `dart analyze` 对本波六文件）：

```
dart analyze lib/transport/terminal_stream.dart lib/pages/terminal_data.dart \
  lib/pages/terminal.dart test/transport/terminal_stream_test.dart \
  test/pages/terminal_data_test.dart test/pages/terminal_test.dart
# No issues found!
```

junction 事项：曾按任务书建 `G:\pa_t1` junction 跑 analyze，analysis_server
在 junction 路径下同样崩溃（`FormatException: Unexpected end of input`，
`LspByteStreamServerChannel._readMessage`），直路亦崩；junction 已删。
全树 analyze 待工具侧恢复后补跑。

## 4. 文件清单（本波新建/改写 6+1）

- `flutter/lib/transport/terminal_stream.dart`（新建，~200 行）
- `flutter/lib/pages/terminal_data.dart`（新建，~430 行）
- `flutter/lib/pages/terminal.dart`（改写：占位 → 真列表+纯文本会话视图）
- `flutter/test/transport/terminal_stream_test.dart`（新建，18 用例）
- `flutter/test/pages/terminal_data_test.dart`（新建，26 用例）
- `flutter/test/pages/terminal_test.dart`（新建，16 用例）
- 本文档（产出，不 commit）

## 5. 并行与过程记录（诚实登记）

- `test/pages/terminal_page_test.dart`（未合入占位测试）首帧用例在本波中途
  由同树协作者同步改断言（"占位契约已退役…改断言首帧"，标题+`正在加载终端
  会话…`），与其余 4 用例（返回键 + TerminalSession 三形状）共 5 用例全绿；
  本波 `TerminalSession` 的 data 化+重导出保持了该文件编译通过。非本波所改，
  在此登记。
- 本波 `terminal_data.dart` 曾被同树协作者加注释/`ignore`（初始化形参说明），
  中间态一度使 scoped analyze 报 5 error（后证实为编辑中途态）；终态合并为
  行尾 `// ignore: prefer_initializing_formals`，scoped 零问题。
- widget 测试探针结论（已固化进实现注释）：testWidgets 的 FakeAsync 下
  broadcast 订阅 `cancel()` 的 future 不结束（最小复现已验证），页面取消
  订阅一律不 await（取消动作同步生效）。数据层（真 async）不受影响。
- 全量 analyze 崩溃见 §3；junction 已按纪律删除。
