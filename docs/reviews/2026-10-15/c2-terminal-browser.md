# B3 第 6 波——Terminal 页 + Browser 页（2026-10-15）

> 独立双雄最后一波。范围冻结：**页壳 + 安全策略 + 契约，不接真通道**。
> 不 commit（主会话收口统一接线 `main.dart`）。

## 1. 两页范围

| 页 | 路由（Kotlin 对位） | 本波落点 | 不做的（B5） |
|---|---|---|---|
| Terminal | `terminal` 全屏，不在 home tab 体系；入 = 各页抽屉「终端」；出仅 `onBack`。`TerminalScreen.kt` 758 行 + `TerminalBlocks` 578。状态：`bridge.state/error`（WebView+xterm 桥）、`recents`、`sessionId`、`cols/rows` | `terminal.dart`：`TerminalPage` 诚实占位（标题 + 状态说明，无列表——没有数据源就不摆列表）+ `TerminalSession` 抽象接口（open/sendInput/resize/output/exitCodes/close，对位 `Attach` 双向流帧） | xterm 桥（需 xterm 依赖，pubspec 冻结）+ `Attach` 双向流（B2 无流面）+ 工作区候选探测（`getHostInfo`+`listDirectory`）+ 目录切换重建 |
| Browser | `browser?url={url}`（url 默认空串）；入 = Home 抽屉/Files 预览/Chat 链接/TaskCenter 下载/Settings 下载页/Git；出仅 `onBack`（+ 页内历史后退，B5）。`BrowserScreen.kt` 1310 行。状态：`input/progress/pageTitle/canGoBack/canGoForward/desktopView/lastLocal/pcPath/localUri/bridgeRegistered/servers…` | `browser.dart`：`BrowserPage` 目标确认占位（归一化目标 url + 桥策略预告，不加载任何网络内容）+ `browser_policy.dart`：`WebNavPolicy` 全量纯函数移植 + `normalizeBrowserUrl` + `OpenBrowserUrl` 出口形状 | WebView（需 `webview_flutter`，冻结）+ `readFile` 流式渲染/PcFilePreview + dev server 面板（`listDevServers`）+ 页内历史后退 + `stripPcMeta`/`toPcPath`/`devServerPreviewUrl`/`isPcPreviewUrl` |

proto 核对（只记形状，**本波不接**，transport 零改动）：

- `proto/v1/terminal.proto:13-18` — `TerminalService.Attach(stream TerminalClientMessage) returns (stream TerminalServerMessage)`（双向流：`Open → Input*/Resize* → Output*/Exit`，`Close` 任意方向）+ `ListTerminals` + `CloseTerminal`。
- Browser 侧：`HostService.listDevServers`（`host.proto:33`）、`FileSystemService.readFile` 流式（Files 预览/`pc:` 渲染用）。其余全在手机本地 WebView，无新增 RPC。

## 2. 安全策略对位表（Kotlin → Dart，逐函数等价）

| Kotlin `WebNavPolicy.kt` | Dart `browser_policy.dart` | 口径 | 测试 |
|---|---|---|---|
| `isWebScheme`（:17） | `isBrowserWebScheme` | http/https/about:/data:/blob:/javascript:/file:/content: 放行；trim + 小写 | 大小写/首尾空白钉死 |
| `isExternalScheme`（:32） | `isBrowserExternalScheme` | RFC 3986 scheme 形状；相对链接/web scheme/空串/`:novalidscheme` → false | intent/weixin/tel/mailto 命中；相对路径不误杀 |
| `trustedLocalHost`（:49） | `browserTrustedLocalHost` | 回环 + 10/8 + 192.168/16 + 172.16/12 + 100.64/10（含带端口 `10.0.0.5:7246`、上下界 100.64/100.127） | 边界外（100.63/100.128/172.32/8.8.8.8/域名/空）全拒 |
| `trustedBridgeSource`（:76） | `trustedBrowserBridgeSource` | 空串/about:blank/data:/file:/content:/私网 http(s) 可信；公网 + blob:/外部 scheme 不可信 | 空起始页可信（与 Kotlin 初值 `!(http\|\|https)` 同口径） |
| `fileNameFromDisposition`（:90） | `browserFileNameFromDisposition` | RFC 5987 优先（含 `+` 字面量保护）→ 普通 filename → URL 末段 → `下载文件` | `%E6%8A%A5…`→`报告.pdf`；`a+b.txt` 不变空格 |
| `normalizeUrl`（BrowserScreen.kt:1251） | `normalizeBrowserUrl` | 空→`''`；可用 scheme 原样；像网址补 `https://`；搜索词走 Bing（`Uri.encodeQueryComponent`，空格编 `+`，与 `URLEncoder` 同口径）；`pc:`/`content://`/`file:` 原样保留供 B5 分发 | **唯一 deliberate 偏差**：控制字符 `[\x00-\x1F\x7F]` → `''` 拒绝（Kotlin 会送进 Bing 查询串；路由参数来自外部深链，拒绝更安全） |

红线复述：B5 挂载 `WebDownloadBridge` 必须以 `trustedBrowserBridgeSource` 为唯一门控（BrowserScreen.kt:142-150）；公网站点挂载 = 打回。

## 3. 出口回填清单（本波只定形状 `typedef OpenBrowserUrl = void Function(String url)`，调用方不动）

| 出口持有页（别人领地，本波未碰） | Kotlin 调用点 | B4/B5 回填动作 |
|---|---|---|
| Files 预览 | `browser?url=<编码后的 pc 路径>`（§1.4） | 预览行「用浏览器打开」→ `OpenBrowserUrl` 可空回调；null = 不渲染入口 |
| Chat 链接 | `browser?url=`（§1.2 出） | 消息内链接点按 → 同上 |
| TaskCenter 下载项 | `browser?url=`（§1.7 出） | 下载行打开 → 同上 |
| Settings 下载页 | `browser?url=`（§1.8 出） | 已下载网页/文本打开 → 同上 |
| Git | `browser?url=`（§1.5 出） | 预留 → 同上 |
| Home 抽屉 | 浏览器入口（§1.1 出） | 主会话 `main.dart` 接线时直连 `BrowserPage` 路由 |

纪律：可空回调，**不注入就不渲染对应按钮**（与 `TaskCenterPage.onRetryMessage/onRetryDownload` 同纪律）。

## 4. 测试原文（`flutter/test/pages/` 新增 3 文件，34 用例）

- `browser_policy_test.dart`（25）：6 组，对位 `WebNavPolicyWorkflowC.kt` 6 个 `@Test` + `normalizeBrowserUrl` 6 条（含控制字符拒绝、Bing 口径 `hello+world`）。
- `browser_page_test.dart`（4）：有目标（归一化 `example.com/a`→`https://example.com/a` + 外部站点桥策略）/ 空地址（起始页 + 可信来源）/ 私网目标（可信来源）/ 返回键。
- `terminal_page_test.dart`（2 widget + 3 契约）：占位标题 + 诚实文案逐字 + 返回键；`_EchoSession` 假实现钉 `TerminalSession` 形状（open 默认 80x24、sendInput→output 回显、resize、close）。
- 诚实文案（逐字断言，改一字就红）：
  - Terminal：`终端通道随 B5 双向流波次接入：路由已就绪，xterm 渲染与 PTY 会话待依赖解冻后接线。`
  - Browser：`网页渲染随 B5 依赖解冻接入：WebView（webview_flutter）未引入前，本页只做目标确认，不加载任何网络内容。`

## 5. analyze + 全量结果

- `flutter analyze`（junction `G:\pa_c2` 下，**No issues found!**，用后已删，`Test-Path` = False）。
- `flutter test` 全量：**573 / 573 全过**（基线 539 + 本波 34；无非我所为的红，并行 Chat/collab 波次未污染）。

## 6. 文件清单（只允许范围内的新增，无修改）

- 新增：`flutter/lib/pages/browser_policy.dart`、`flutter/lib/pages/browser.dart`、`flutter/lib/pages/terminal.dart`；
  `flutter/test/pages/browser_policy_test.dart`、`flutter/test/pages/browser_page_test.dart`、`flutter/test/pages/terminal_page_test.dart`；
  本报告。
- 未碰：`flutter/lib/pages/` 其他一切、`lib/transport/`、`lib/main.dart`、`lib/gen/`、`lib/theme/`、`lib/components/`、`pa_page_kit.dart`（只读复用 `PaSubPageHeader`）、`android/`、`daemon/`、`proto/`、`pubspec.yaml`。无 commit/add/push。

## 7. B5 交接点（主会话收口）

1. `main.dart` 接线：`terminal` → `TerminalPage(onBack: …)`；`browser?url=` → `BrowserPage(initialUrl:…, onBack: …)`（本波未改，由主会话做）。
2. 依赖解冻：`xterm` + `webview_flutter`（pubspec）。
3. Terminal 真实现 `TerminalSession`（构造注入，FilesPage source 模式）：`Attach` 双向流 + xterm 桥（bindWebView/onReady/sendInput/sendResize/start/close）+ 工作区候选探测 + 换 sessionKey 重建 + dispose 关流。
4. Browser 真实现：WebView 装配（`trustedBrowserBridgeSource` 门控挂载下载桥）+ `readFile` 流式 + PcFilePreview 移植 + dev server 面板 + 页内历史后退（`BackHandler(canGoBack)`）+ `stripPcMeta`/`toPcPath`/`devServerPreviewUrl`/`isPcPreviewUrl` 移植。
5. 五页出口回填（§3 表）：把打开动作收敛为 `OpenBrowserUrl` 可空回调。
