# B3 第 6 波 B1——Browser 页数据层（2026-10-15）

> 范围：`browser?url=` 分流 + 预览类型 + 导航裁决纯函数。不 commit。
> 关键前置：C2 子代理已先行交付 `browser.dart`（目标确认占位）+
> `browser_policy.dart`（WebNavPolicy 全量移植）+ 3 个测试文件（均未提交，
> 见 §6）；本波不覆盖、不复制，只补 C2 未覆盖的缺口。

## 1. 依赖判定：webview_flutter 可取，但本波不加

- 现状：`flutter/pubspec.yaml` 无 `webview_flutter`（已确认）。
- 探针（repo 外临时目录，repo 内零改动，最多 2 次内完成）：
  - 第 1 次：临时目录未先建，`pub add` 报 `Expected to find project root`
    （纯操作失误，不计网络问题）。
  - 第 2 次：`flutter pub add webview_flutter` 解析成功——
    `webview_flutter 4.14.1` + `webview_flutter_android 4.14.1` +
    `webview_flutter_wkwebview 3.27.0` + platform_interface 2.15.1，
    经国内镜像（`pub.flutter-io.cn`）`Changed 44 dependencies!`。
- 判定：**可取（4.14.1），但本波不加入 repo pubspec**，理由：
  1. 无消费者——本波页面（C2 占位 + 本波数据层）都不加载网络内容，
     加依赖 = 死依赖；真正的挂载点（下载桥门控 + 页内历史 + dev 面板）是 B5；
  2. 并行飞行中（C1/C2/T 在跑），pubspec.yaml/pubspec.lock  churn 会干扰
     各自的 test/analyze，且 add-to-app 模块加插件会影响主会话 APK 管线，
     应由主会话在 B5 统一决定；
  3. 诚实占位已有——C2 页文案逐字断言
     `网页渲染随 B5 依赖解冻接入…`（browser_page_test.dart），本波数据层注释
     同样点名 B5（browser_data.dart 头注释 B5 接线说明）。

## 2. 分流契约（`browserTargetOf`，新增）

`browser?url=` → 五态（`BrowserTargetKind`），分流顺序 = Kotlin 口径
（BrowserScreen.kt:359-365 先分发 `pc:` 再归一化）：

| 输入 | kind | address | 消费方（B5） |
|---|---|---|---|
| 空/空白 | `empty` | `''` | 起始页，不猜落点 |
| `pc:D:\a\b.txt`（大小写不敏感） | `pcFile` | `D:\a\b.txt`（原文） | readFile 真通道（§3） |
| `pc:` 空路径 | `blocked` | `''` + 人话 | 拦截页 |
| `about:`/`data:`/`file:`/`content://` | `localPreview` | 原文 | 本地渲染，不走网络 |
| 网址/搜索词（含 Bing 串） | `web` | `normalizeBrowserUrl` 结果 | WebView（B5 挂载） |
| 外部 scheme（tel:/weixin:/intent:…） | `blocked` | `''` + 点名 scheme | 拦截页（交对应应用） |
| 控制字符 | `blocked` | `''` + 人话 | 拦截页 |
| `javascript:`/`blob:` 当整页目标 | `blocked` | `''` + 人话 | 拦截页（页内子资源另算） |

## 3. 策略表

### 3.1 WebNavPolicy 复核（C2 交付，只读核验，未改一字）

逐条对过 `android/…/ui/component/WebNavPolicy.kt`（110 行）：

| Kotlin | Dart（browser_policy.dart） | 核验结论 |
|---|---|---|
| `isWebScheme`（:17-23） | `isBrowserWebScheme` | 等价（含 trim+小写；file:/content: 放行） |
| `isExternalScheme`（:32-42） | `isBrowserExternalScheme` | 等价（RFC 3986 形状；相对链接/空/`:` 开头 → false） |
| `trustedLocalHost`（:49-64） | `browserTrustedLocalHost` | 等价（回环+10/8+192.168/16+172.16/12+100.64/10；`:7246` 端口先剥） |
| `trustedBridgeSource`（:76-84） | `trustedBrowserBridgeSource` | 等价（空/about:blank/data:/file:/content:/私网 http(s)） |
| `fileNameFromDisposition`（:90-109） | `browserFileNameFromDisposition` | 等价（RFC 5987 优先 + `+` 字面量保护 + 去引号 + URL 末段兜底 + `下载文件`） |

信任门仍是 `trustedBrowserBridgeSource` 单一真值，本波不包第二层。

### 3.2 本波新增裁决层（browser_data.dart）

- `browserNavVerdictOf`：allow/block 二值。**单测抓到一个真 bug 并修复**——
  首版把 `javascript:`/`blob:` 检查放在归一化之后，无点号的
  `javascript:alert(1)` 会被 `normalizeBrowserUrl` 转成 Bing 搜索串而漏网
  （allow+web）；修复：移到归一化之前判原文（:195-198），回归 22 全绿。
  B5 接线顺序：先 `browserTargetOf` 分流，只把 `web` 的归一化地址喂 WebView。
- `pcPreviewKindOf`：后缀路由（口径 = PcFilePreview.kt render 路由 +
  isBinary 名单）：text（含 md/txt/json/xml/log/yaml + 19 个源码后缀）/
  table（xlsx/xlsm/xls/ods/csv/tsv）/ document（docx/doc/pptx/pdf…13 个）/
  image（png/jpg/gif/webp/bmp/ico/svg）/ web（html/htm）/ archive（zip/jar）/
  unsupported（二进制名单 + 无后缀 + 点文件，宁可严）。
- `pcPreviewRefusal`：类型门（可预览→`''`；unsupported→点名后缀的人话；
  大小门仍是 B3-4 `filePreviewRejection`，不重复）。
- 红线：全部 blocked/refusal 文案单测扫 `未知`/`错误` 子串（AGENTS.md §4.4），
  8 条全过。

### 3.3 PcFilePreview / readFile 接线（复用，不另起）

- `PaBridge.readFile`（`flutter/lib/transport/pa_bridge.dart:422`，只读核验）+
  `BridgeFilePreviewSource`（`files_bridge.dart:385-408`）已是完整文本预览链：
  readFile 首块 + `text` 标记判二进制 + 256KB 门 + 截断标注。
- 本波不包第二层 PreviewSource；B5 接线：pcFile+text → `readText`；
  pcFile+table/document/archive → readFile 字节喂 PcFilePreview 移植；
  image/web → WebView 原生。`stripPcMeta`（`?_pas=&_pam=`）是 B5 的，
  本文件只在 `pcPreviewKindOf` 顺手剥残留 `?`/`#` 尾巴。

## 4. 测试原文

- 基线（改前，后台全量）：`573 passed`（= C2 报告数，含 C2 的 34）。
- 本波单文件：`flutter test test/pages/browser_data_test.dart` →
  `+22: All tests passed!`（10 分流 + 5 后缀路由 + 2 类型门 + 4 裁决 + 1 红线横扫）。
- 改后全量：`+595: All tests passed!`（573 + 22，无非我所为的红）。
- 页面覆盖归属：页面三态/拦截/预览视图随 B5 真渲染实现；当前页行为覆盖在
  C2 的 `browser_page_test.dart`（4）+ `browser_policy_test.dart`（25），
  本波不再重复覆盖。

## 5. analyze

- junction（`G:\pa_b1`，用后已删，`Test-Path` = False）：`flutter analyze`
  连续 3 次 `analysis server exited with code 255`——服务端 LSP 在握手帧里
  被百分号编码的中文根路径（`%E9%A1%B9%E7%9B%AE1`）截断
  （`FormatException: Unexpected end of input`，chunk 切分 flake），
  与代码无关（C2 早前同手法曾成功，属环境 flake）。
- 替代证据（等价文件集，全 ASCII 临时拷贝 `b1-an`，repo 零污染，用后已删）：
  - 全量：`8 issues found`——**全部非我所为**：
    `chat_history.dart` unused_field ×1、`chat_history_data.dart`
    prefer_initializing_formals ×1、`chat_session.dart`
    unnecessary_import ×1 + unnecessary_cast ×3、
    `chat_turn_stream.dart` / `terminal_stream.dart` unnecessary_import ×2
   （均为并行 Chat/T 波次在建文件，如实登记，不碰）。
  - Browser 簇 6 文件
    （browser_data/policy/browser.dart + browser_data/policy/page_test.dart）：
    **`No issues found!`**——本波 2 文件零问题。

## 6. 文件清单（只增 2，不改 1 行旧码，无 commit/add/push）

- 新增：
  - `flutter/lib/pages/browser_data.dart`（分流/预览类型/裁决纯函数，~300 行）
  - `flutter/test/pages/browser_data_test.dart`（22 用例）
  - 本报告。
- **刻意未建**（如实登记，均非遗漏）：
  - `lib/pages/browser.dart`——C2 已交付目标确认占位（未提交，?? 状态），
    覆盖写入 =  parallel 碰撞；其 WebView/PcFilePreview/加载态/错误态属 B5。
  - `test/pages/browser_test.dart`——会与 C2 的 `browser_page_test.dart`
    断言同一 widget 同一文案，纯维护成本；页面真渲染覆盖待 B5。
- 未碰：`pa_bridge.dart`（只读复用 readFile）、`chat_*.dart`、`terminal*.dart`
 （含并行新建的 `terminal_data.dart`/`terminal_stream.dart`）、`main.dart`、
  `lib/gen/`、其他 pages（含 C2 的 browser_policy.dart/browser.dart）、
  `android/`、`daemon/`、`proto/`、`channel-contract.md`（只读）、pubspec
  （§1 探针在 repo 外）。`git status` 无 M 项属我（仅并行他人的 M/??）。
